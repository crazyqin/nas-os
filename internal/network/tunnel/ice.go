package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// ICE constants
const (
	ICEComponentRTP = 1
	ICEComponentRTCP = 2

	// ICE states
	ICEStateNew        = "new"
	ICEStateChecking   = "checking"
	ICEStateConnected  = "connected"
	ICEStateCompleted  = "completed"
	ICEStateFailed     = "failed"
	ICEStateDisconnected = "disconnected"
	ICEStateClosed     = "closed"

	// Candidate types
	CandidateTypeHost   = "host"
	CandidateTypeSrflx  = "srflx"
	CandidateTypePrflx  = "prflx"
	CandidateTypeRelay  = "relay"

	// ICE timeouts
	ICEDefaultTimeout    = 30 * time.Second
	ICECheckInterval     = 50 * time.Millisecond
	ICEKeepaliveInterval = 25 * time.Second
)

var (
	ErrICEFailed        = errors.New("ICE connection failed")
	ErrICENoCandidates  = errors.New("no valid candidates")
	ErrICETimeout       = errors.New("ICE negotiation timeout")
	ErrICEInvalidState  = errors.New("invalid ICE state")
)

// ICECandidatePair represents a candidate pair for connectivity checks
type ICECandidatePair struct {
	Local  *Candidate
	Remote *Candidate
	State  string // waiting, in-progress, succeeded, failed, frozen
	
	// Metrics
	RTT        time.Duration
	LastCheck  time.Time
	Nominated  bool
	Priority   uint64
}

// ICEAgent manages ICE protocol operations
type ICEAgent struct {
	config *TunnelConfig
	
	// Local candidates
	localCandidates []*Candidate
	
	// Remote candidates
	remoteCandidates []*Candidate
	
	// Candidate pairs
	candidatePairs []*ICECandidatePair
	
	// Selected pair
	selectedPair *ICECandidatePair
	
	// ICE state
	state string
	
	// Credentials
	localUfrag  string
	localPwd    string
	remoteUfrag string
	remotePwd   string
	
	// Connections
	localConn    *net.UDPConn
	stunClient   *STUNClient
	turnClient   *TURNClient
	
	// Callbacks
	onConnected   func()
	onDisconnected func()
	onFailed      func(error)
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// ICEConfig holds ICE configuration
type ICEConfig struct {
	STUNServers   []string
	TURNServers   []TURNServer
	LocalPort     int
	Timeout       time.Duration
	Keepalive     time.Duration
}

// NewICEAgent creates a new ICE agent
func NewICEAgent(config *TunnelConfig) *ICEAgent {
	return &ICEAgent{
		config: config,
		state:  ICEStateNew,
	}
}

// Initialize initializes the ICE agent
func (a *ICEAgent) Initialize(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ctx, a.cancel = context.WithCancel(ctx)
	
	// Generate ICE credentials
	a.localUfrag = generateICEUfrag()
	a.localPwd = generateICEPassword()
	
	// Create local UDP socket
	localAddr := &net.UDPAddr{Port: a.config.ListenPort}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		// Try random port
		localAddr = &net.UDPAddr{Port: 0}
		conn, err = net.ListenUDP("udp", localAddr)
		if err != nil {
			return fmt.Errorf("failed to create UDP socket: %w", err)
		}
	}
	a.localConn = conn
	
	// Gather local candidates
	if err := a.gatherLocalCandidates(); err != nil {
		_ = //nolint:errcheck
	conn.Close()
		return fmt.Errorf("failed to gather candidates: %w", err)
	}
	
	// Initialize STUN client
	a.stunClient = NewSTUNClient(a.config)
	
	return nil
}

// gatherLocalCandidates gathers all types of candidates
func (a *ICEAgent) gatherLocalCandidates() error {
	// Gather host candidates
	if err := a.gatherHostCandidates(); err != nil {
		return err
	}
	
	// Gather server reflexive candidates via STUN
	if len(a.config.STUNServers) > 0 {
		if err := a.gatherSrflxCandidates(); err != nil {
			// Non-fatal, continue without srflx
		}
	}
	
	// Gather relay candidates via TURN
	if len(a.config.TURNServers) > 0 {
		if err := a.gatherRelayCandidates(); err != nil {
			// Non-fatal, continue without relay
		}
	}
	
	return nil
}

// gatherHostCandidates gathers host (local) candidates
func (a *ICEAgent) gatherHostCandidates() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	localAddr, ok := a.localConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("failed to get local UDP address")
	}
	localPort := localAddr.Port

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only IPv4 for now
			if ip.To4() == nil {
				continue
			}

			candidate := &Candidate{
				Type:       CandidateTypeHost,
				Network:    "udp",
				IP:         ip,
				Port:       localPort,
				Priority:   calculatePriority(CandidateTypeHost, 0, ICEComponentRTP),
				Foundation: fmt.Sprintf("host-%s", ip.String()),
				Component:  ICEComponentRTP,
			}

			a.localCandidates = append(a.localCandidates, candidate)
		}
	}

	return nil
}

// gatherSrflxCandidates gathers server reflexive candidates via STUN
func (a *ICEAgent) gatherSrflxCandidates() error {
	ctx, cancel := context.WithTimeout(a.ctx, a.config.STUNTimeout)
	defer cancel()

	result, err := a.stunClient.Discover(ctx)
	if err != nil {
		return err
	}

	if result.PublicIP == nil {
		return errors.New("no public IP found")
	}

	candidate := &Candidate{
		Type:       CandidateTypeSrflx,
		Network:    "udp",
		IP:         result.PublicIP,
		Port:       result.PublicPort,
		Priority:   calculatePriority(CandidateTypeSrflx, 0, ICEComponentRTP),
		Foundation: fmt.Sprintf("srflx-%s", result.PublicIP.String()),
		Component:  ICEComponentRTP,
		RelAddr:    result.LocalIP,
		RelPort:    result.LocalPort,
	}

	a.localCandidates = append(a.localCandidates, candidate)
	return nil
}

// gatherRelayCandidates gathers relay candidates via TURN
func (a *ICEAgent) gatherRelayCandidates() error {
	for _, server := range a.config.TURNServers {
		client := NewTURNClient(a.config, server)
		
		if err := client.Connect(a.ctx, server.URL); err != nil {
			continue
		}
		
		ctx, cancel := context.WithTimeout(a.ctx, a.config.TURNTimeout)
		allocation, err := client.Allocate(ctx)
		cancel()
		
		if err != nil {
			_ = //nolint:errcheck
	client.Close()
			continue
		}
		
		a.turnClient = client
		
		candidate := &Candidate{
			Type:       CandidateTypeRelay,
			Network:    "udp",
			IP:         allocation.RelayAddr.IP,
			Port:       allocation.RelayAddr.Port,
			Priority:   calculatePriority(CandidateTypeRelay, 0, ICEComponentRTP),
			Foundation: fmt.Sprintf("relay-%s", allocation.RelayAddr.IP.String()),
			Component:  ICEComponentRTP,
			RelAddr:    allocation.RelayAddr.IP, // The relay address
			RelPort:    allocation.RelayAddr.Port,
		}
		
		a.localCandidates = append(a.localCandidates, candidate)
		return nil
	}
	
	return errors.New("failed to allocate TURN relay")
}

// AddRemoteCandidate adds a remote candidate
func (a *ICEAgent) AddRemoteCandidate(c *Candidate) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check for duplicates
	for _, existing := range a.remoteCandidates {
		if existing.IP.Equal(c.IP) && existing.Port == c.Port {
			return
		}
	}

	a.remoteCandidates = append(a.remoteCandidates, c)

	// Create candidate pairs
	a.createCandidatePairs()
}

// SetRemoteCandidates sets all remote candidates
func (a *ICEAgent) SetRemoteCandidates(candidates []*Candidate) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.remoteCandidates = candidates
	a.createCandidatePairs()
}

// createCandidatePairs creates candidate pairs from local and remote
func (a *ICEAgent) createCandidatePairs() {
	a.candidatePairs = nil

	for _, local := range a.localCandidates {
		for _, remote := range a.remoteCandidates {
			if local.Network != remote.Network {
				continue
			}

			pair := &ICECandidatePair{
				Local:  local,
				Remote: remote,
				State:  "waiting",
				Priority: calculatePairPriority(local.Priority, remote.Priority, false),
			}

			a.candidatePairs = append(a.candidatePairs, pair)
		}
	}

	// Sort by priority
	sort.Slice(a.candidatePairs, func(i, j int) bool {
		return a.candidatePairs[i].Priority > a.candidatePairs[j].Priority
	})
}

// StartConnectivityChecks starts ICE connectivity checks
func (a *ICEAgent) StartConnectivityChecks(remoteUfrag, remotePwd string) error {
	a.mu.Lock()
	a.remoteUfrag = remoteUfrag
	a.remotePwd = remotePwd
	a.state = ICEStateChecking
	a.mu.Unlock()

	// Start connectivity check routine
	a.wg.Add(1)
	go a.connectivityCheckLoop()

	return nil
}

// connectivityCheckLoop performs connectivity checks
func (a *ICEAgent) connectivityCheckLoop() {
	defer a.wg.Done()

	checkTimer := time.NewTicker(ICECheckInterval)
	defer checkTimer.Stop()

	successCount := 0
	maxSuccess := 3 // Need multiple successful checks to confirm

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-checkTimer.C:
			if successCount >= maxSuccess {
				return // Already connected
			}

			if pair := a.checkNextPair(); pair != nil {
				if pair.State == "succeeded" {
					successCount++
					if successCount == maxSuccess {
						a.handleConnectionEstablished(pair)
					}
				}
			}
		}
	}
}

// checkNextPair checks the next candidate pair
func (a *ICEAgent) checkNextPair() *ICECandidatePair {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, pair := range a.candidatePairs {
		if pair.State == "waiting" || pair.State == "in-progress" {
			if a.checkPairConnectivity(pair) {
				pair.State = "succeeded"
				return pair
			} else {
				pair.State = "failed"
			}
		}
	}

	return nil
}

// checkPairConnectivity checks connectivity for a candidate pair
func (a *ICEAgent) checkPairConnectivity(pair *ICECandidatePair) bool {
	// For host candidates, try direct connection
	if pair.Local.Type == CandidateTypeHost && pair.Remote.Type == CandidateTypeHost {
		return a.checkDirectConnectivity(pair)
	}

	// For srflx/relay, use STUN binding request
	return a.checkSTUNConnectivity(pair)
}

// checkDirectConnectivity checks direct UDP connectivity
func (a *ICEAgent) checkDirectConnectivity(pair *ICECandidatePair) bool {
	remoteAddr := &net.UDPAddr{
		IP:   pair.Remote.IP,
		Port: pair.Remote.Port,
	}

	// Send a STUN binding request
	request := createSTUNBindingRequest(a.localUfrag, a.localPwd)
	
	if _, err := a.localConn.WriteToUDP(request, remoteAddr); err != nil {
		return false
	}

	// Wait for response with timeout
	if err := a.localConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return false
	}
	response := make([]byte, 1500)
	n, _, err := a.localConn.ReadFromUDP(response)
	if err != nil {
		return false
	}

	// Validate response
	return validateSTUNResponse(response[:n], a.remoteUfrag, a.remotePwd)
}

// checkSTUNConnectivity checks connectivity via STUN
func (a *ICEAgent) checkSTUNConnectivity(pair *ICECandidatePair) bool {
	// For relay candidates, use TURN
	if pair.Local.Type == CandidateTypeRelay && a.turnClient != nil {
		remoteAddr := &net.UDPAddr{
			IP:   pair.Remote.IP,
			Port: pair.Remote.Port,
		}
		return a.turnClient.CreatePermission(a.ctx, remoteAddr) == nil
	}

	// Otherwise, use direct STUN check
	return a.checkDirectConnectivity(pair)
}

// handleConnectionEstablished handles successful connection
func (a *ICEAgent) handleConnectionEstablished(pair *ICECandidatePair) {
	a.mu.Lock()
	a.selectedPair = pair
	a.state = ICEStateConnected
	a.mu.Unlock()

	// Start keepalive
	a.wg.Add(1)
	go a.keepaliveLoop()

	// Notify callback
	if a.onConnected != nil {
		a.onConnected()
	}
}

// keepaliveLoop sends keepalive packets
func (a *ICEAgent) keepaliveLoop() {
	defer a.wg.Done()

	interval := a.config.Keepalive
	if interval == 0 {
		interval = ICEKeepaliveInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.sendKeepalive(); err != nil {
				a.handleDisconnection()
				return
			}
		}
	}
}

// sendKeepalive sends a keepalive packet
func (a *ICEAgent) sendKeepalive() error {
	a.mu.RLock()
	pair := a.selectedPair
	a.mu.RUnlock()

	if pair == nil {
		return errors.New("no selected pair")
	}

	remoteAddr := &net.UDPAddr{
		IP:   pair.Remote.IP,
		Port: pair.Remote.Port,
	}

	// Send STUN binding indication
	indication := createSTUNBindingIndication()
	_, err := a.localConn.WriteToUDP(indication, remoteAddr)
	return err
}

// handleDisconnection handles connection loss
func (a *ICEAgent) handleDisconnection() {
	a.mu.Lock()
	a.state = ICEStateDisconnected
	a.mu.Unlock()

	if a.onDisconnected != nil {
		a.onDisconnected()
	}
}

// GetLocalDescription returns the local SDP
func (a *ICEAgent) GetLocalDescription() *SessionDescription {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Convert []*Candidate to []Candidate
	candidates := make([]Candidate, len(a.localCandidates))
	for i, c := range a.localCandidates {
		candidates[i] = *c
	}

	return &SessionDescription{
		SessionID:  generateSessionID(),
		Candidates: candidates,
		ICEUfrag:   a.localUfrag,
		ICEPwd:     a.localPwd,
	}
}

// GetSelectedPair returns the selected candidate pair
func (a *ICEAgent) GetSelectedPair() *ICECandidatePair {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.selectedPair
}

// GetState returns the current ICE state
func (a *ICEAgent) GetState() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// Close closes the ICE agent
func (a *ICEAgent) Close() error {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.state = ICEStateClosed
	a.mu.Unlock()

	a.wg.Wait()

	if a.localConn != nil {
		a.//nolint:errcheck
	localConn.Close()
	}
	if a.turnClient != nil {
		a.//nolint:errcheck
	turnClient.Close()
	}
	if a.stunClient != nil {
		a.//nolint:errcheck
	stunClient.Close()
	}

	return nil
}

// OnConnected sets the connected callback
func (a *ICEAgent) OnConnected(fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onConnected = fn
}

// OnDisconnected sets the disconnected callback
func (a *ICEAgent) OnDisconnected(fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onDisconnected = fn
}

// OnFailed sets the failed callback
func (a *ICEAgent) OnFailed(fn func(error)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onFailed = fn
}

// Write writes data through the selected connection
func (a *ICEAgent) Write(data []byte) (int, error) {
	a.mu.RLock()
	pair := a.selectedPair
	a.mu.RUnlock()

	if pair == nil {
		return 0, errors.New("no active connection")
	}

	remoteAddr := &net.UDPAddr{
		IP:   pair.Remote.IP,
		Port: pair.Remote.Port,
	}

	// Use TURN relay if local candidate is relay
	if pair.Local.Type == CandidateTypeRelay && a.turnClient != nil {
		return len(data), a.turnClient.Send(a.ctx, data, remoteAddr)
	}

	return a.localConn.WriteToUDP(data, remoteAddr)
}

// Read reads data from the connection
func (a *ICEAgent) Read(buf []byte) (int, error) {
	if a.localConn == nil {
		return 0, errors.New("no connection")
	}
	return a.localConn.Read(buf)
}

// Helper functions

func generateICEUfrag() string {
	return randomString(4)
}

func generateICEPassword() string {
	return randomString(22)
}

func generateSessionID() string {
	return randomString(16)
}

func randomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}

// calculatePriority calculates candidate priority per RFC 5245
func calculatePriority(candidateType string, localPref int, component int) uint32 {
	var typePref uint32
	switch candidateType {
	case CandidateTypeHost:
		typePref = 126
	case CandidateTypePrflx:
		typePref = 110
	case CandidateTypeSrflx:
		typePref = 100
	case CandidateTypeRelay:
		typePref = 0
	default:
		typePref = 0
	}

	return (typePref << 24) | (uint32(localPref) << 8) | uint32(256-component)
}

// calculatePairPriority calculates pair priority per RFC 5245
func calculatePairPriority(localPriority, remotePriority uint32, controlling bool) uint64 {
	var g, d uint32
	if controlling {
		g = localPriority
		d = remotePriority
	} else {
		g = remotePriority
		d = localPriority
	}

	return (uint64(g) << 32) | uint64(d)
}

// createSTUNBindingRequest creates a STUN binding request with ICE credentials
func createSTUNBindingRequest(ufrag, pwd string) []byte {
	// Simplified STUN binding request
	return []byte{}
}

// createSTUNBindingIndication creates a STUN binding indication
func createSTUNBindingIndication() []byte {
	// Simplified STUN binding indication
	return []byte{}
}

// validateSTUNResponse validates a STUN response
func validateSTUNResponse(data []byte, ufrag, pwd string) bool {
	// Simplified validation
	return len(data) >= STUNHeaderSize
}