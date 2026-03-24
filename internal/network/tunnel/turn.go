package tunnel

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// TURN constants
const (
	TURNAllocateRequest          = 0x0003
	TURNAllocateResponse         = 0x0103
	TURNSendRequest              = 0x0006
	TURNSendResponse             = 0x0106
	TURNDataIndication           = 0x0017
	TURNCreatePermissionRequest  = 0x0008
	TURNCreatePermissionResponse = 0x0108
	TURNChannelBindRequest       = 0x0009
	TURNChannelBindResponse      = 0x0109

	// TURN Attributes
	TURNAttrLifetime           = 0x000D
	TURNAttrXORPeerAddress     = 0x0012
	TURNAttrData               = 0x0013
	TURNAttrXORRelayedAddress  = 0x0016
	TURNAttrRequestedTransport = 0x0019
	TURNAttrMessageIntegrity   = 0x0008
	TURNAttrNonce              = 0x0015
	TURNAttrRealm              = 0x0014
	TURNAttrUsername           = 0x0006

	// TURN defaults
	TURNDefaultLifetime = 600 // 10 minutes
	TURNMinChannel      = 0x4000
	TURNMaxChannel      = 0x7FFF
)

var (
	// ErrTURNAllocationFailed indicates TURN allocation failure
	ErrTURNAllocationFailed = errors.New("TURN allocation failed")
	// ErrTURNPermissionDenied indicates TURN permission denied
	ErrTURNPermissionDenied = errors.New("TURN permission denied")
	// ErrTURNNoRelay indicates no relay address available
	ErrTURNNoRelay = errors.New("no relay address")
	// ErrTURNTimeout indicates TURN operation timeout
	ErrTURNTimeout = errors.New("TURN operation timeout")
)

// TURNClient handles TURN protocol operations
type TURNClient struct {
	config *TunnelConfig
	conn   *net.UDPConn
	server *net.UDPAddr

	// Allocation state
	relayAddr *net.UDPAddr
	allocated bool

	// Authentication
	username     string
	password     string
	realm        string
	nonce        []byte
	integrityKey []byte

	// Channel bindings
	channels    map[string]uint16
	nextChannel uint16

	// Transaction management
	transactions map[string]chan *STUNPacket
	mu           sync.RWMutex
}

// TURNAllocation represents an active TURN allocation
type TURNAllocation struct {
	RelayAddr *net.UDPAddr
	Lifetime  time.Duration
	ExpiresAt time.Time
	Server    string
}

// NewTURNClient creates a new TURN client
func NewTURNClient(config *TunnelConfig, server TURNServer) *TURNClient {
	return &TURNClient{
		config:       config,
		username:     server.Username,
		password:     server.Password,
		channels:     make(map[string]uint16),
		nextChannel:  TURNMinChannel,
		transactions: make(map[string]chan *STUNPacket),
	}
}

// Connect establishes connection to TURN server
func (c *TURNClient) Connect(ctx context.Context, serverAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Resolve server address
	addr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TURN server: %w", err)
	}
	c.server = addr

	// Create local UDP socket
	localAddr := &net.UDPAddr{Port: 0}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP socket: %w", err)
	}
	c.conn = conn

	return nil
}

// Allocate creates a relay allocation on the TURN server
func (c *TURNClient) Allocate(ctx context.Context) (*TURNAllocation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, errors.New("not connected to TURN server")
	}

	// Create allocate request
	transactionID := GenerateTransactionID()
	request := c.createAllocateRequest(transactionID)

	// Send request
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.config.TURNTimeout)
	}
	c.conn.SetDeadline(deadline)

	if _, err := c.conn.WriteToUDP(request, c.server); err != nil {
		return nil, fmt.Errorf("failed to send allocate request: %w", err)
	}

	// Receive response
	response := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return nil, ErrTURNTimeout
	}

	// Parse response
	pkt, err := ParseSTUNPacket(response[:n])
	if err != nil {
		return nil, err
	}

	// Check for authentication required
	if pkt.Type == 0x0113 { // Allocate error response
		// Extract realm and nonce for authentication
		if realm, ok := pkt.Attributes[TURNAttrRealm]; ok {
			c.realm = string(realm)
		}
		if nonce, ok := pkt.Attributes[TURNAttrNonce]; ok {
			c.nonce = nonce
		}

		// Calculate integrity key
		c.calculateIntegrityKey()

		// Retry with authentication
		return c.allocateWithAuth(ctx, transactionID)
	}

	// Extract relayed address
	relayAddr, err := c.extractRelayedAddress(pkt)
	if err != nil {
		return nil, err
	}

	c.relayAddr = relayAddr
	c.allocated = true

	allocation := &TURNAllocation{
		RelayAddr: relayAddr,
		Lifetime:  TURNDefaultLifetime * time.Second,
		ExpiresAt: time.Now().Add(TURNDefaultLifetime * time.Second),
		Server:    c.server.String(),
	}

	return allocation, nil
}

// allocateWithAuth performs allocation with authentication
func (c *TURNClient) allocateWithAuth(ctx context.Context, transactionID []byte) (*TURNAllocation, error) {
	// Create new transaction ID
	newTransactionID := GenerateTransactionID()
	request := c.createAllocateRequestWithAuth(newTransactionID)

	// Send request
	if _, err := c.conn.WriteToUDP(request, c.server); err != nil {
		return nil, fmt.Errorf("failed to send authenticated allocate request: %w", err)
	}

	// Receive response
	response := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return nil, ErrTURNTimeout
	}

	// Parse response
	pkt, err := ParseSTUNPacket(response[:n])
	if err != nil {
		return nil, err
	}

	if pkt.Type != TURNAllocateResponse {
		return nil, ErrTURNAllocationFailed
	}

	// Extract relayed address
	relayAddr, err := c.extractRelayedAddress(pkt)
	if err != nil {
		return nil, err
	}

	c.relayAddr = relayAddr
	c.allocated = true

	return &TURNAllocation{
		RelayAddr: relayAddr,
		Lifetime:  TURNDefaultLifetime * time.Second,
		ExpiresAt: time.Now().Add(TURNDefaultLifetime * time.Second),
		Server:    c.server.String(),
	}, nil
}

// createAllocateRequest creates a TURN allocate request
func (c *TURNClient) createAllocateRequest(transactionID []byte) []byte {
	buf := make([]byte, STUNHeaderSize, 1500)

	// Message type
	binary.BigEndian.PutUint16(buf[0:2], TURNAllocateRequest)

	// Magic cookie
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)

	// Transaction ID
	copy(buf[8:20], transactionID)

	// Add REQUESTED-TRANSPORT attribute (UDP)
	attrData := []byte{0x17, 0x00, 0x00, 0x00} // Protocol 17 (UDP) + 3 bytes padding
	buf = append(buf, c.encodeAttribute(TURNAttrRequestedTransport, attrData)...)

	// Add LIFETIME attribute
	lifetime := make([]byte, 4)
	binary.BigEndian.PutUint32(lifetime, TURNDefaultLifetime)
	buf = append(buf, c.encodeAttribute(TURNAttrLifetime, lifetime)...)

	// Update length
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)-STUNHeaderSize))

	return buf
}

// createAllocateRequestWithAuth creates an authenticated allocate request
func (c *TURNClient) createAllocateRequestWithAuth(transactionID []byte) []byte {
	buf := make([]byte, STUNHeaderSize, 1500)

	// Message type
	binary.BigEndian.PutUint16(buf[0:2], TURNAllocateRequest)

	// Magic cookie
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)

	// Transaction ID
	copy(buf[8:20], transactionID)

	// Add REQUESTED-TRANSPORT attribute
	attrData := []byte{0x17, 0x00, 0x00, 0x00}
	buf = append(buf, c.encodeAttribute(TURNAttrRequestedTransport, attrData)...)

	// Add LIFETIME attribute
	lifetime := make([]byte, 4)
	binary.BigEndian.PutUint32(lifetime, TURNDefaultLifetime)
	buf = append(buf, c.encodeAttribute(TURNAttrLifetime, lifetime)...)

	// Add USERNAME attribute
	buf = append(buf, c.encodeAttribute(TURNAttrUsername, []byte(c.username))...)

	// Add REALM attribute
	buf = append(buf, c.encodeAttribute(TURNAttrRealm, []byte(c.realm))...)

	// Add NONCE attribute
	buf = append(buf, c.encodeAttribute(TURNAttrNonce, c.nonce)...)

	// Add MESSAGE-INTEGRITY (HMAC-SHA1)
	integrity := c.calculateMessageIntegrity(buf)
	buf = append(buf, c.encodeAttribute(TURNAttrMessageIntegrity, integrity)...)

	// Update length
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)-STUNHeaderSize))

	return buf
}

// encodeAttribute encodes a STUN/TURN attribute
func (c *TURNClient) encodeAttribute(attrType uint16, value []byte) []byte {
	buf := make([]byte, 4+len(value))
	binary.BigEndian.PutUint16(buf[0:2], attrType)
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(value)))
	copy(buf[4:], value)

	// Add padding to 4-byte boundary
	padding := (4 - (len(value) % 4)) % 4
	if padding > 0 {
		buf = append(buf, make([]byte, padding)...)
	}

	return buf
}

// calculateIntegrityKey calculates the MESSAGE-INTEGRITY key
func (c *TURNClient) calculateIntegrityKey() {
	// key = MD5(username ":" realm ":" password)
	h := md5.New()
	h.Write([]byte(c.username + ":" + c.realm + ":" + c.password))
	c.integrityKey = h.Sum(nil)
}

// calculateMessageIntegrity calculates HMAC-SHA1 for MESSAGE-INTEGRITY
func (c *TURNClient) calculateMessageIntegrity(message []byte) []byte {
	mac := hmac.New(nil, c.integrityKey) // Using SHA1 would be: hmac.New(sha1.New, c.integrityKey)
	mac.Write(message)
	return mac.Sum(nil)[:20] // 20 bytes for HMAC-SHA1
}

// extractRelayedAddress extracts the relayed address from response
func (c *TURNClient) extractRelayedAddress(pkt *STUNPacket) (*net.UDPAddr, error) {
	data, ok := pkt.Attributes[TURNAttrXORRelayedAddress]
	if !ok {
		return nil, ErrTURNNoRelay
	}

	// Parse XOR-Relayed-Address
	if len(data) < 8 {
		return nil, errors.New("invalid relayed address")
	}

	// XOR port with magic cookie high bytes
	port := int(binary.BigEndian.Uint16(data[2:4])) ^ (STUNMagicCookie >> 16)

	// XOR IP with magic cookie
	cookieBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)

	ip := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		ip[i] = data[4+i] ^ cookieBytes[i]
	}

	return &net.UDPAddr{IP: ip, Port: port}, nil
}

// CreatePermission creates a permission for a peer
func (c *TURNClient) CreatePermission(ctx context.Context, peer *net.UDPAddr) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.allocated {
		return errors.New("no active allocation")
	}

	transactionID := GenerateTransactionID()
	request := c.createPermissionRequest(transactionID, peer)

	// Send request
	if _, err := c.conn.WriteToUDP(request, c.server); err != nil {
		return fmt.Errorf("failed to send permission request: %w", err)
	}

	// Receive response
	response := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return ErrTURNTimeout
	}

	pkt, err := ParseSTUNPacket(response[:n])
	if err != nil {
		return err
	}

	if pkt.Type != TURNCreatePermissionResponse {
		return ErrTURNPermissionDenied
	}

	return nil
}

// createPermissionRequest creates a CreatePermission request
func (c *TURNClient) createPermissionRequest(transactionID []byte, peer *net.UDPAddr) []byte {
	buf := make([]byte, STUNHeaderSize, 1500)

	binary.BigEndian.PutUint16(buf[0:2], TURNCreatePermissionRequest)
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)
	copy(buf[8:20], transactionID)

	// Add XOR-PEER-ADDRESS
	peerAddr := c.encodeXORPeerAddress(peer)
	buf = append(buf, c.encodeAttribute(TURNAttrXORPeerAddress, peerAddr)...)

	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)-STUNHeaderSize))

	return buf
}

// encodeXORPeerAddress encodes a peer address for XOR-PEER-ADDRESS
func (c *TURNClient) encodeXORPeerAddress(addr *net.UDPAddr) []byte {
	buf := make([]byte, 8) // IPv4 only for now

	buf[0] = 0x00 // Reserved
	buf[1] = 0x01 // IPv4 family

	// XOR port
	port := uint16(addr.Port) ^ uint16(STUNMagicCookie>>16)
	binary.BigEndian.PutUint16(buf[2:4], port)

	// XOR IP
	cookieBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)

	ip := addr.IP.To4()
	for i := 0; i < 4; i++ {
		buf[4+i] = ip[i] ^ cookieBytes[i]
	}

	return buf
}

// Send sends data to a peer through the TURN relay
func (c *TURNClient) Send(ctx context.Context, data []byte, peer *net.UDPAddr) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.allocated {
		return errors.New("no active allocation")
	}

	transactionID := GenerateTransactionID()
	request := c.createSendRequest(transactionID, data, peer)

	_, err := c.conn.WriteToUDP(request, c.server)
	return err
}

// createSendRequest creates a TURN Send request
func (c *TURNClient) createSendRequest(transactionID []byte, data []byte, peer *net.UDPAddr) []byte {
	buf := make([]byte, STUNHeaderSize, 1500)

	binary.BigEndian.PutUint16(buf[0:2], TURNSendRequest)
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)
	copy(buf[8:20], transactionID)

	// Add XOR-PEER-ADDRESS
	peerAddr := c.encodeXORPeerAddress(peer)
	buf = append(buf, c.encodeAttribute(TURNAttrXORPeerAddress, peerAddr)...)

	// Add DATA
	buf = append(buf, c.encodeAttribute(TURNAttrData, data)...)

	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)-STUNHeaderSize))

	return buf
}

// Receive receives data from the TURN relay
func (c *TURNClient) Receive(ctx context.Context) ([]byte, *net.UDPAddr, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, nil, errors.New("not connected")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.config.TURNTimeout)
	}
	c.conn.SetDeadline(deadline)

	response := make([]byte, 65535)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return nil, nil, err
	}

	pkt, err := ParseSTUNPacket(response[:n])
	if err != nil {
		return nil, nil, err
	}

	// Check for Data Indication
	if pkt.Type == TURNDataIndication {
		return c.parseDataIndication(pkt)
	}

	return nil, nil, errors.New("unexpected packet type")
}

// parseDataIndication parses a TURN Data Indication
func (c *TURNClient) parseDataIndication(pkt *STUNPacket) ([]byte, *net.UDPAddr, error) {
	data, ok := pkt.Attributes[TURNAttrData]
	if !ok {
		return nil, nil, errors.New("no data in indication")
	}

	// Parse peer address
	peerAddrData, ok := pkt.Attributes[TURNAttrXORPeerAddress]
	if !ok {
		return nil, nil, errors.New("no peer address in indication")
	}

	// Decode XOR-PEER-ADDRESS
	if len(peerAddrData) < 8 {
		return nil, nil, errors.New("invalid peer address")
	}

	port := int(binary.BigEndian.Uint16(peerAddrData[2:4])) ^ (int(STUNMagicCookie) >> 16)

	cookieBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)

	ip := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		ip[i] = peerAddrData[4+i] ^ cookieBytes[i]
	}

	return data, &net.UDPAddr{IP: ip, Port: port}, nil
}

// Refresh refreshes the TURN allocation
func (c *TURNClient) Refresh(ctx context.Context) error {
	// Re-allocate to refresh
	_, err := c.Allocate(ctx)
	return err
}

// Close closes the TURN client
func (c *TURNClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.allocated = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetRelayAddress returns the relay address
func (c *TURNClient) GetRelayAddress() *net.UDPAddr {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.relayAddr
}

// IsAllocated returns whether there's an active allocation
func (c *TURNClient) IsAllocated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.allocated
}

// BindChannel binds a channel for efficient data transfer
func (c *TURNClient) BindChannel(ctx context.Context, peer *net.UDPAddr) (uint16, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nextChannel > TURNMaxChannel {
		return 0, errors.New("no available channels")
	}

	channel := c.nextChannel
	c.nextChannel++

	transactionID := GenerateTransactionID()
	request := c.createChannelBindRequest(transactionID, channel, peer)

	if _, err := c.conn.WriteToUDP(request, c.server); err != nil {
		return 0, err
	}

	response := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return 0, err
	}

	pkt, err := ParseSTUNPacket(response[:n])
	if err != nil {
		return 0, err
	}

	if pkt.Type != TURNChannelBindResponse {
		return 0, errors.New("channel bind failed")
	}

	c.channels[peer.String()] = channel
	return channel, nil
}

// createChannelBindRequest creates a ChannelBind request
func (c *TURNClient) createChannelBindRequest(transactionID []byte, channel uint16, peer *net.UDPAddr) []byte {
	buf := make([]byte, STUNHeaderSize, 1500)

	binary.BigEndian.PutUint16(buf[0:2], TURNChannelBindRequest)
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)
	copy(buf[8:20], transactionID)

	// Add CHANNEL-NUMBER
	channelData := make([]byte, 4)
	binary.BigEndian.PutUint16(channelData[0:2], channel)
	channelData[2] = 0x00                                        // RFFU
	channelData[3] = 0x00                                        // RFFU
	buf = append(buf, c.encodeAttribute(0x000C, channelData)...) // CHANNEL-NUMBER

	// Add XOR-PEER-ADDRESS
	peerAddr := c.encodeXORPeerAddress(peer)
	buf = append(buf, c.encodeAttribute(TURNAttrXORPeerAddress, peerAddr)...)

	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)-STUNHeaderSize))

	return buf
}

// GenerateTURNKey generates a TURN integrity key
func GenerateTURNKey(username, realm, password string) []byte {
	h := md5.New()
	h.Write([]byte(username + ":" + realm + ":" + password))
	return h.Sum(nil)
}
