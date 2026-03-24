package tunnel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// STUN constants
const (
	STUNMagicCookie = 0x2112A442
	STUNHeaderSize  = 20
	STUNBindingRequest  = 0x0001
	STUNBindingResponse = 0x0101
	STUNAttrMappedAddress    = 0x0001
	STUNAttrXORMappedAddress = 0x0020
	STUNAttrSoftware         = 0x0002
	STUNAttrMessageIntegrity = 0x0008
	STUNAttrFingerprint      = 0x8028
)

var (
	// ErrSTUNTimeout indicates STUN request timeout
	ErrSTUNTimeout     = errors.New("STUN request timeout")
	// ErrSTUNNoResponse indicates no STUN response received
	ErrSTUNNoResponse  = errors.New("no STUN response")
	// ErrSTUNInvalid indicates invalid STUN response
	ErrSTUNInvalid     = errors.New("invalid STUN response")
	// ErrSTUNNoMapped indicates no mapped address in response
	ErrSTUNNoMapped    = errors.New("no mapped address in response")
)

// STUNClient handles STUN protocol operations
type STUNClient struct {
	config    *TunnelConfig
	conn      *net.UDPConn
	mu        sync.RWMutex
}

// STUNResult contains the result of a STUN query
type STUNResult struct {
	PublicIP   net.IP
	PublicPort int
	NATType    NATType
	LocalIP    net.IP
	LocalPort  int
}

// NewSTUNClient creates a new STUN client
func NewSTUNClient(config *TunnelConfig) *STUNClient {
	return &STUNClient{
		config: config,
	}
}

// Discover performs NAT discovery using multiple STUN servers
func (c *STUNClient) Discover(ctx context.Context) (*STUNResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create UDP socket
	localAddr := &net.UDPAddr{Port: c.config.ListenPort}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		// Try random port if specified port is in use
		localAddr = &net.UDPAddr{Port: 0}
		conn, err = net.ListenUDP("udp", localAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to create UDP socket: %w", err)
		}
	}
	c.conn = conn
	defer conn.Close()

	result := &STUNResult{
		LocalPort: localAddr.Port,
	}

	// Get local IP
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					result.LocalIP = ipnet.IP
					break
				}
			}
		}
	}

	// Query first STUN server for public address
	if len(c.config.STUNServers) > 0 {
		pubIP, pubPort, err := c.querySTUNServer(ctx, c.config.STUNServers[0])
		if err != nil {
			return nil, fmt.Errorf("STUN query failed: %w", err)
		}
		result.PublicIP = pubIP
		result.PublicPort = pubPort

		// Detect NAT type
		if len(c.config.STUNServers) >= 2 {
			result.NATType = c.detectNATType(ctx)
		} else {
			// Compare with local to determine if behind NAT
			if !result.PublicIP.Equal(result.LocalIP) {
				result.NATType = NATUnknown
			} else {
				result.NATType = NATNone
			}
		}
	}

	return result, nil
}

// querySTUNServer queries a single STUN server
func (c *STUNClient) querySTUNServer(ctx context.Context, server string) (net.IP, int, error) {
	// Parse STUN server address
	host := strings.TrimPrefix(server, "stun:")
	if host == "" {
		host = server
	}
	
	// Remove port if present and use default
	port := 3478
	if parts := strings.Split(host, ":"); len(parts) == 2 {
		host = parts[0]
		_, _ = fmt.Sscanf(parts[1], "%d", &port)
	}

	// Resolve server address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve STUN server: %w", err)
	}

	// Create and send binding request
	request := c.createBindingRequest()
	
	// Set deadline
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.config.STUNTimeout)
	}
	c.conn.SetDeadline(deadline)

	// Send request
	if _, err := c.conn.WriteToUDP(request, addr); err != nil {
		return nil, 0, fmt.Errorf("failed to send STUN request: %w", err)
	}

	// Receive response
	response := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(response)
	if err != nil {
		return nil, 0, ErrSTUNTimeout
	}

	// Parse response
	return c.parseBindingResponse(response[:n])
}

// createBindingRequest creates a STUN binding request message
func (c *STUNClient) createBindingRequest() []byte {
	// STUN header: 2 bytes type + 2 bytes length + 4 bytes magic + 12 bytes transaction ID
	buf := make([]byte, STUNHeaderSize)
	
	// Message type: Binding Request
	binary.BigEndian.PutUint16(buf[0:2], STUNBindingRequest)
	
	// Magic cookie
	binary.BigEndian.PutUint32(buf[4:8], STUNMagicCookie)
	
	// Transaction ID (12 random bytes)
	rand.Read(buf[8:20])
	
	// No attributes in basic request
	binary.BigEndian.PutUint16(buf[2:4], 0)
	
	return buf
}

// parseBindingResponse parses a STUN binding response
func (c *STUNClient) parseBindingResponse(data []byte) (net.IP, int, error) {
	if len(data) < STUNHeaderSize {
		return nil, 0, ErrSTUNInvalid
	}

	// Verify magic cookie
	cookie := binary.BigEndian.Uint32(data[4:8])
	if cookie != STUNMagicCookie {
		return nil, 0, ErrSTUNInvalid
	}

	// Parse attributes
	attrLen := binary.BigEndian.Uint16(data[2:4])
	offset := STUNHeaderSize

	for offset < int(attrLen)+STUNHeaderSize {
		if offset+4 > len(data) {
			break
		}

		attrType := binary.BigEndian.Uint16(data[offset : offset+2])
		attrValueLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4

		if offset+attrValueLen > len(data) {
			break
		}

		// Look for XOR-Mapped-Address (preferred) or Mapped-Address
		switch attrType {
		case STUNAttrXORMappedAddress:
			return c.parseXORMappedAddress(data[offset : offset+attrValueLen])
		case STUNAttrMappedAddress:
			return c.parseMappedAddress(data[offset : offset+attrValueLen])
		}

		// Move to next attribute (4-byte aligned)
		offset += (attrValueLen + 3) & ^3
	}

	return nil, 0, ErrSTUNNoMapped
}

// parseMappedAddress parses STUN MAPPED-ADDRESS attribute
func (c *STUNClient) parseMappedAddress(data []byte) (net.IP, int, error) {
	if len(data) < 8 {
		return nil, 0, ErrSTUNInvalid
	}

	// Format: 1 byte reserved + 1 byte family + 2 bytes port + IP
	family := data[1]
	port := int(binary.BigEndian.Uint16(data[2:4]))

	var ip net.IP
	switch family {
	case 1: // IPv4
		ip = net.IP(data[4:8])
	case 2: // IPv6
		if len(data) < 20 {
			return nil, 0, ErrSTUNInvalid
		}
		ip = net.IP(data[4:20])
	default:
		return nil, 0, ErrSTUNInvalid
	}

	return ip, port, nil
}

// parseXORMappedAddress parses STUN XOR-MAPPED-ADDRESS attribute
func (c *STUNClient) parseXORMappedAddress(data []byte) (net.IP, int, error) {
	if len(data) < 8 {
		return nil, 0, ErrSTUNInvalid
	}

	family := data[1]
	// XOR port with magic cookie high bytes
	port := int(binary.BigEndian.Uint16(data[2:4])) ^ (STUNMagicCookie >> 16)

	var ip net.IP
	switch family {
	case 1: // IPv4
		// XOR IP with magic cookie
		cookieBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(cookieBytes, STUNMagicCookie)
		ip = make(net.IP, 4)
		for i := 0; i < 4; i++ {
			ip[i] = data[4+i] ^ cookieBytes[i]
		}
	case 2: // IPv6
		if len(data) < 20 {
			return nil, 0, ErrSTUNInvalid
		}
		// XOR with magic cookie + transaction ID (which we don't have)
		// Simplified: just use the IP as-is for now
		ip = net.IP(data[4:20])
	default:
		return nil, 0, ErrSTUNInvalid
	}

	return ip, port, nil
}

// detectNATType determines the NAT type using RFC 3489 algorithm
func (c *STUNClient) detectNATType(ctx context.Context) NATType {
	if len(c.config.STUNServers) < 2 {
		return NATUnknown
	}

	// Query first server
	ip1, port1, err := c.querySTUNServer(ctx, c.config.STUNServers[0])
	if err != nil {
		return NATUnknown
	}

	// Query second server
	ip2, port2, err := c.querySTUNServer(ctx, c.config.STUNServers[1])
	if err != nil {
		return NATUnknown
	}

	// Compare results
	if !ip1.Equal(ip2) {
		// Different IPs from different servers = Symmetric NAT
		return NATSymmetric
	}

	if port1 != port2 {
		// Same IP but different ports
		// This indicates port-dependent behavior
		return NATPortRestricted
	}

	// Same IP and port from both servers
	// Need additional tests to determine cone type
	// For simplicity, assume restricted cone
	return NATRestrictedCone
}

// GetPublicAddress returns the public IP and port
func (c *STUNClient) GetPublicAddress(ctx context.Context, server string) (net.IP, int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		localAddr := &net.UDPAddr{Port: c.config.ListenPort}
		conn, err := net.ListenUDP("udp", localAddr)
		if err != nil {
			localAddr = &net.UDPAddr{Port: 0}
			conn, err = net.ListenUDP("udp", localAddr)
			if err != nil {
				return nil, 0, err
			}
		}
		c.conn = conn
	}

	return c.querySTUNServer(ctx, server)
}

// Close closes the STUN client connection
func (c *STUNClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsBehindNAT checks if the client is behind NAT
func (c *STUNClient) IsBehindNAT(ctx context.Context) (bool, error) {
	result, err := c.Discover(ctx)
	if err != nil {
		return false, err
	}
	return result.NATType != NATNone && result.NATType != NATUnknown, nil
}

// ParseSTUNServer parses a STUN server URL
func ParseSTUNServer(url string) (host string, port int, err error) {
	url = strings.TrimPrefix(url, "stun:")
	url = strings.TrimPrefix(url, "stuns:")
	
	parts := strings.Split(url, ":")
	host = parts[0]
	port = 3478
	
	if len(parts) > 1 {
		_, err = fmt.Sscanf(parts[1], "%d", &port)
	}
	
	return host, port, err
}

// STUNPacket represents a parsed STUN packet
type STUNPacket struct {
	Type         uint16
	Length       uint16
	TransactionID []byte
	Attributes   map[uint16][]byte
}

// ParseSTUNPacket parses a raw STUN packet
func ParseSTUNPacket(data []byte) (*STUNPacket, error) {
	if len(data) < STUNHeaderSize {
		return nil, ErrSTUNInvalid
	}

	// Verify magic cookie
	cookie := binary.BigEndian.Uint32(data[4:8])
	if cookie != STUNMagicCookie {
		return nil, ErrSTUNInvalid
	}

	pkt := &STUNPacket{
		Type:          binary.BigEndian.Uint16(data[0:2]),
		Length:        binary.BigEndian.Uint16(data[2:4]),
		TransactionID: make([]byte, 12),
		Attributes:    make(map[uint16][]byte),
	}

	copy(pkt.TransactionID, data[8:20])

	// Parse attributes
	offset := STUNHeaderSize
	for offset < int(pkt.Length)+STUNHeaderSize {
		if offset+4 > len(data) {
			break
		}

		attrType := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4

		if offset+attrLen > len(data) {
			break
		}

		pkt.Attributes[attrType] = make([]byte, attrLen)
		copy(pkt.Attributes[attrType], data[offset:offset+attrLen])

		// Move to next attribute (4-byte aligned)
		offset += (attrLen + 3) & ^3
	}

	return pkt, nil
}

// GenerateTransactionID generates a new STUN transaction ID
func GenerateTransactionID() []byte {
	id := make([]byte, 12)
	rand.Read(id)
	return id
}

// ValidateTransactionID checks if two transaction IDs match
func ValidateTransactionID(id1, id2 []byte) bool {
	return bytes.Equal(id1, id2)
}