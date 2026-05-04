package vpnserver

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Service manages all VPN server operations including WireGuard, OpenVPN,
// user authorization, and connection monitoring.
type Service struct {
	mu sync.RWMutex

	// WireGuard state
	wgInterfaces map[string]*WireGuardInterface

	// OpenVPN state
	openvpnConfig *OpenVPNConfig
	openvpnClients map[string]*OpenVPNClient

	// User and device authorization
	users   map[string]*VPNUser
	devices map[string]*VPNDevice

	// Active connections
	sessions map[string]*ConnectionSession

	// Fail2Ban 安全防护
	fail2ban *Fail2Ban

	// Network config
	dnsConfig *DNSConfig
	natConfig *NATConfig

	// Metadata
	startTime time.Time
	nextID    int
}

// NewService creates a new VPN server service.
func NewService() *Service {
	return &Service{
		wgInterfaces:   make(map[string]*WireGuardInterface),
		openvpnClients: make(map[string]*OpenVPNClient),
		users:          make(map[string]*VPNUser),
		devices:        make(map[string]*VPNDevice),
		sessions:       make(map[string]*ConnectionSession),
		fail2ban:       NewFail2Ban(),
		dnsConfig: &DNSConfig{
			PrimaryDNS:   "8.8.8.8",
			SecondaryDNS: "8.8.4.4",
		},
		natConfig: &NATConfig{
			Enabled:    false,
			Masquerade: true,
		},
		startTime: time.Now(),
		nextID:    1,
	}
}

// generateID generates a sequential unique ID with the given prefix.
func (s *Service) generateID(prefix string) string {
	id := fmt.Sprintf("%s_%d", prefix, s.nextID)
	s.nextID++
	return id
}

// ==================== WireGuard Management ====================

// CreateWGInterface creates a new WireGuard interface with the given configuration.
func (s *Service) CreateWGInterface(req CreateWGInterfaceRequest) (*WireGuardInterface, error) {
	if err := validateIPNetwork(req.Address); err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	if req.Name == "" {
		return nil, fmt.Errorf("interface name is required")
	}
	if req.ListenPort <= 0 || req.ListenPort > 65535 {
		return nil, fmt.Errorf("listen port must be between 1 and 65535")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.wgInterfaces[req.Name]; exists {
		return nil, fmt.Errorf("interface %s already exists", req.Name)
	}

	privateKey := generateKeyPlaceholder("wg_priv")
	publicKey := generateKeyPlaceholder("wg_pub")

	dns := req.DNS
	if len(dns) == 0 {
		dns = []string{"8.8.8.8", "8.8.4.4"}
	}

	iface := &WireGuardInterface{
		Name:       req.Name,
		ListenPort: req.ListenPort,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    req.Address,
		DNS:        dns,
		Status:     StatusRunning,
		Peers:      []WireGuardPeer{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.wgInterfaces[req.Name] = iface
	return copyWGInterface(iface), nil
}

// DeleteWGInterface deletes a WireGuard interface by name.
func (s *Service) DeleteWGInterface(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.wgInterfaces[name]; !exists {
		return fmt.Errorf("interface %s not found", name)
	}
	delete(s.wgInterfaces, name)
	return nil
}

// GetWGInterface returns a WireGuard interface by name.
func (s *Service) GetWGInterface(name string) (*WireGuardInterface, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	iface, exists := s.wgInterfaces[name]
	if !exists {
		return nil, fmt.Errorf("interface %s not found", name)
	}
	return copyWGInterface(iface), nil
}

// ListWGInterfaces returns all WireGuard interfaces.
func (s *Service) ListWGInterfaces() []*WireGuardInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WireGuardInterface, 0, len(s.wgInterfaces))
	for _, iface := range s.wgInterfaces {
		result = append(result, copyWGInterface(iface))
	}
	return result
}

// StartWGInterface starts a WireGuard interface.
func (s *Service) StartWGInterface(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	iface, exists := s.wgInterfaces[name]
	if !exists {
		return fmt.Errorf("interface %s not found", name)
	}
	iface.Status = StatusRunning
	iface.UpdatedAt = time.Now()
	return nil
}

// StopWGInterface stops a WireGuard interface.
func (s *Service) StopWGInterface(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	iface, exists := s.wgInterfaces[name]
	if !exists {
		return fmt.Errorf("interface %s not found", name)
	}
	iface.Status = StatusStopped
	iface.UpdatedAt = time.Now()
	return nil
}

// AddWGPeer adds a peer to a WireGuard interface.
func (s *Service) AddWGPeer(ifaceName string, req AddWGPeerRequest) (*WireGuardPeer, error) {
	if req.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}
	if len(req.AllowedIPs) == 0 {
		return nil, fmt.Errorf("at least one allowed IP is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	iface, exists := s.wgInterfaces[ifaceName]
	if !exists {
		return nil, fmt.Errorf("interface %s not found", ifaceName)
	}

	for _, p := range iface.Peers {
		if p.PublicKey == req.PublicKey {
			return nil, fmt.Errorf("peer with public key already exists")
		}
	}

	keepalive := req.PersistentKeepalive
	if keepalive == 0 {
		keepalive = 25
	}

	peer := WireGuardPeer{
		PublicKey:           req.PublicKey,
		AllowedIPs:          req.AllowedIPs,
		Name:                req.Name,
		Enabled:             true,
		PersistentKeepalive: keepalive,
	}
	iface.Peers = append(iface.Peers, peer)
	iface.UpdatedAt = time.Now()

	// Return a copy
	p := peer
	return &p, nil
}

// RemoveWGPeer removes a peer from a WireGuard interface by public key.
func (s *Service) RemoveWGPeer(ifaceName, publicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	iface, exists := s.wgInterfaces[ifaceName]
	if !exists {
		return fmt.Errorf("interface %s not found", ifaceName)
	}

	for i, p := range iface.Peers {
		if p.PublicKey == publicKey {
			iface.Peers = append(iface.Peers[:i], iface.Peers[i+1:]...)
			iface.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("peer not found")
}

// ListWGPeers returns all peers for a WireGuard interface.
func (s *Service) ListWGPeers(ifaceName string) ([]WireGuardPeer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	iface, exists := s.wgInterfaces[ifaceName]
	if !exists {
		return nil, fmt.Errorf("interface %s not found", ifaceName)
	}
	result := make([]WireGuardPeer, len(iface.Peers))
	copy(result, iface.Peers)
	return result, nil
}

// ==================== OpenVPN Management ====================

// GetOpenVPNConfig returns the current OpenVPN configuration.
func (s *Service) GetOpenVPNConfig() *OpenVPNConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.openvpnConfig == nil {
		return nil
	}
	cfg := *s.openvpnConfig
	return &cfg
}

// UpdateOpenVPNConfig updates the OpenVPN configuration.
func (s *Service) UpdateOpenVPNConfig(req UpdateOpenVPNRequest) *OpenVPNConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if s.openvpnConfig == nil {
		s.openvpnConfig = &OpenVPNConfig{
			CreatedAt: now,
		}
	}

	cfg := s.openvpnConfig
	cfg.Enabled = req.Enabled
	cfg.Port = req.Port
	cfg.Protocol = req.Protocol
	cfg.Subnet = req.Subnet
	cfg.Netmask = req.Netmask
	cfg.DNS = req.DNS
	cfg.MaxClients = req.MaxClients
	cfg.KeepAlive = req.KeepAlive
	cfg.Compression = req.Compression
	cfg.Cipher = req.Cipher
	cfg.AuthType = req.AuthType
	cfg.UpdatedAt = now

	if req.Enabled {
		cfg.Status = StatusRunning
	} else {
		cfg.Status = StatusStopped
	}

	result := *cfg
	return &result
}

// StartOpenVPN starts the OpenVPN server.
func (s *Service) StartOpenVPN() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.openvpnConfig == nil {
		return fmt.Errorf("OpenVPN not configured")
	}
	s.openvpnConfig.Enabled = true
	s.openvpnConfig.Status = StatusRunning
	s.openvpnConfig.UpdatedAt = time.Now()
	return nil
}

// StopOpenVPN stops the OpenVPN server.
func (s *Service) StopOpenVPN() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.openvpnConfig == nil {
		return fmt.Errorf("OpenVPN not configured")
	}
	s.openvpnConfig.Enabled = false
	s.openvpnConfig.Status = StatusStopped
	s.openvpnConfig.UpdatedAt = time.Now()
	return nil
}

// CreateOpenVPNClient creates a new OpenVPN client certificate.
func (s *Service) CreateOpenVPNClient(name string) (*OpenVPNClient, error) {
	if name == "" {
		return nil, fmt.Errorf("client name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.generateID("ovpn")
	client := &OpenVPNClient{
		ID:         id,
		Name:       name,
		CN:         name,
		Certificate: fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s_cert\n-----END CERTIFICATE-----", name),
		PrivateKey:  fmt.Sprintf("-----BEGIN PRIVATE KEY-----\n%s_key\n-----END PRIVATE KEY-----", name),
		Enabled:    true,
		CreatedAt:  time.Now(),
	}
	s.openvpnClients[id] = client
	result := *client
	return &result, nil
}

// DeleteOpenVPNClient deletes an OpenVPN client.
func (s *Service) DeleteOpenVPNClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openvpnClients[id]; !exists {
		return fmt.Errorf("client %s not found", id)
	}
	delete(s.openvpnClients, id)
	return nil
}

// ListOpenVPNClients returns all OpenVPN clients.
func (s *Service) ListOpenVPNClients() []*OpenVPNClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*OpenVPNClient, 0, len(s.openvpnClients))
	for _, c := range s.openvpnClients {
		cp := *c
		result = append(result, &cp)
	}
	return result
}

// GetOpenVPNClient returns an OpenVPN client by ID.
func (s *Service) GetOpenVPNClient(id string) (*OpenVPNClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, exists := s.openvpnClients[id]
	if !exists {
		return nil, fmt.Errorf("client %s not found", id)
	}
	result := *client
	return &result, nil
}

// ==================== User Authorization ====================

// CreateVPNUser creates a new VPN user with the given authorization.
func (s *Service) CreateVPNUser(req CreateVPNUserRequest) (*VPNUser, error) {
	if req.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if len(req.Protocols) == 0 {
		return nil, fmt.Errorf("at least one protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate username
	for _, u := range s.users {
		if u.Username == req.Username {
			return nil, fmt.Errorf("user %s already exists", req.Username)
		}
	}

	maxDevices := req.MaxDevices
	if maxDevices <= 0 {
		maxDevices = 5
	}

	id := s.generateID("user")
	user := &VPNUser{
		ID:           id,
		Username:     req.Username,
		Permission:   req.Permission,
		Protocols:    req.Protocols,
		MaxDevices:   maxDevices,
		Devices:      []VPNDevice{},
		TrafficLimit: req.TrafficLimit,
		ExpiresAt:    req.ExpiresAt,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Metadata:     req.Metadata,
	}
	if user.Permission == "" {
		user.Permission = PermissionAllow
	}
	s.users[id] = user
	result := *user
	return &result, nil
}

// GetVPNUser returns a VPN user by ID.
func (s *Service) GetVPNUser(id string) (*VPNUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user %s not found", id)
	}
	result := *user
	return &result, nil
}

// ListVPNUsers returns all VPN users.
func (s *Service) ListVPNUsers() []*VPNUser {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*VPNUser, 0, len(s.users))
	for _, u := range s.users {
		cp := *u
		result = append(result, &cp)
	}
	return result
}

// UpdateVPNUser updates a VPN user's authorization settings.
func (s *Service) UpdateVPNUser(id string, req CreateVPNUserRequest) (*VPNUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user %s not found", id)
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Permission != "" {
		user.Permission = req.Permission
	}
	if len(req.Protocols) > 0 {
		user.Protocols = req.Protocols
	}
	if req.MaxDevices > 0 {
		user.MaxDevices = req.MaxDevices
	}
	if req.TrafficLimit > 0 {
		user.TrafficLimit = req.TrafficLimit
	}
	if req.ExpiresAt != nil {
		user.ExpiresAt = req.ExpiresAt
	}
	if req.Metadata != nil {
		user.Metadata = req.Metadata
	}
	user.UpdatedAt = time.Now()

	result := *user
	return &result, nil
}

// DeleteVPNUser deletes a VPN user by ID.
func (s *Service) DeleteVPNUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[id]; !exists {
		return fmt.Errorf("user %s not found", id)
	}
	// Also remove user's devices
	for did, d := range s.devices {
		if d.UserID == id {
			delete(s.devices, did)
		}
	}
	delete(s.users, id)
	return nil
}

// ==================== Device Management ====================

// AddDevice adds a new device to a VPN user.
func (s *Service) AddDevice(userID string, req AddDeviceRequest) (*VPNDevice, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("device name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, fmt.Errorf("user %s not found", userID)
	}

	if len(user.Devices) >= user.MaxDevices {
		return nil, fmt.Errorf("device limit reached (%d/%d)", len(user.Devices), user.MaxDevices)
	}

	id := s.generateID("dev")
	publicKey := ""
	certificate := ""
	assignedIP := ""

	if req.Protocol == ProtocolWireGuard {
		publicKey = generateKeyPlaceholder("wg_peer")
		assignedIP = fmt.Sprintf("10.0.0.%d", s.nextID+10)
	} else {
		certificate = fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s_cert\n-----END CERTIFICATE-----", req.Name)
		assignedIP = fmt.Sprintf("10.8.0.%d", s.nextID+10)
	}

	device := &VPNDevice{
		ID:          id,
		Name:        req.Name,
		UserID:      userID,
		Protocol:    req.Protocol,
		PublicKey:   publicKey,
		Certificate: certificate,
		AssignedIP:  assignedIP,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	s.devices[id] = device

	// Add to user's device list
	user.Devices = append(user.Devices, *device)
	user.UpdatedAt = time.Now()

	result := *device
	return &result, nil
}

// DeleteDevice deletes a VPN device.
func (s *Service) DeleteDevice(userID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}
	if device.UserID != userID {
		return fmt.Errorf("device %s does not belong to user %s", deviceID, userID)
	}

	delete(s.devices, deviceID)

	// Remove from user's device list
	if user, ok := s.users[userID]; ok {
		for i, d := range user.Devices {
			if d.ID == deviceID {
				user.Devices = append(user.Devices[:i], user.Devices[i+1:]...)
				break
			}
		}
		user.UpdatedAt = time.Now()
	}
	return nil
}

// ListDevices returns all devices for a user.
func (s *Service) ListDevices(userID string) ([]VPNDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, fmt.Errorf("user %s not found", userID)
	}
	result := make([]VPNDevice, len(user.Devices))
	copy(result, user.Devices)
	return result, nil
}

// ==================== Connection Monitoring ====================

// GetActiveSessions returns all active VPN connection sessions.
func (s *Service) GetActiveSessions() []*ConnectionSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ConnectionSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cp := *sess
		cp.Duration = time.Since(sess.ConnectedAt)
		result = append(result, &cp)
	}
	return result
}

// GetServerStatus returns the overall VPN server status.
func (s *Service) GetServerStatus() *ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &ServerStatus{
		TotalUsers:  len(s.users),
		Uptime:      time.Since(s.startTime),
	}

	// Find first WG interface
	for _, iface := range s.wgInterfaces {
		cp := *iface
		status.WireGuard = &cp
		break
	}

	if s.openvpnConfig != nil {
		cfg := *s.openvpnConfig
		status.OpenVPN = &cfg
	}

	status.ActiveConns = len(s.sessions)

	var totalRx, totalTx int64
	for _, sess := range s.sessions {
		totalRx += sess.TrafficStats.RxBytes
		totalTx += sess.TrafficStats.TxBytes
	}
	status.TotalTraffic = TrafficStats{
		RxBytes:   totalRx,
		TxBytes:   totalTx,
		UpdatedAt: time.Now(),
	}

	return status
}

// ==================== Network Configuration ====================

// GetDNSConfig returns the current DNS configuration.
func (s *Service) GetDNSConfig() *DNSConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := *s.dnsConfig
	return &cfg
}

// UpdateDNSConfig updates the DNS configuration.
func (s *Service) UpdateDNSConfig(req UpdateDNSRequest) *DNSConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.PrimaryDNS != "" {
		s.dnsConfig.PrimaryDNS = req.PrimaryDNS
	}
	if req.SecondaryDNS != "" {
		s.dnsConfig.SecondaryDNS = req.SecondaryDNS
	}
	if req.Domains != nil {
		s.dnsConfig.Domains = req.Domains
	}

	cfg := *s.dnsConfig
	return &cfg
}

// GetNATConfig returns the current NAT configuration.
func (s *Service) GetNATConfig() *NATConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := *s.natConfig
	return &cfg
}

// UpdateNATConfig updates the NAT/masquerade configuration.
func (s *Service) UpdateNATConfig(req UpdateNATRequest) *NATConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.natConfig.Enabled = req.Enabled
	s.natConfig.Interface = req.Interface
	s.natConfig.Subnets = req.Subnets
	s.natConfig.Masquerade = req.Masquerade
	if req.PortForward != nil {
		s.natConfig.PortForward = req.PortForward
	}

	cfg := *s.natConfig
	return &cfg
}

// ==================== Config Generation ====================

// GenerateWGConfig generates a WireGuard client configuration file content.
func (s *Service) GenerateWGConfig(ifaceName, peerPublicKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	iface, exists := s.wgInterfaces[ifaceName]
	if !exists {
		return "", fmt.Errorf("interface %s not found", ifaceName)
	}

	var peer *WireGuardPeer
	for i, p := range iface.Peers {
		if p.PublicKey == peerPublicKey {
			peer = &iface.Peers[i]
			break
		}
	}
	if peer == nil {
		return "", fmt.Errorf("peer not found")
	}

	dns := "8.8.8.8, 8.8.4.4"
	if len(iface.DNS) > 0 {
		dns = ""
		for i, d := range iface.DNS {
			if i > 0 {
				dns += ", "
			}
			dns += d
		}
	}

	allowedIPs := "0.0.0.0/0"
	if len(peer.AllowedIPs) > 0 {
		allowedIPs = ""
		for i, ip := range peer.AllowedIPs {
			if i > 0 {
				allowedIPs += ", "
			}
			allowedIPs += ip
		}
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = %s
DNS = %s

[Peer]
PublicKey = %s
Endpoint = <SERVER_ADDRESS>:%d
AllowedIPs = %s
PersistentKeepalive = %d
`, peer.AllowedIPs[0], dns, iface.PublicKey, iface.ListenPort, allowedIPs, peer.PersistentKeepalive)

	return config, nil
}

// GenerateOpenVPNConfig generates an OpenVPN client configuration file content.
func (s *Service) GenerateOpenVPNConfig(clientID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.openvpnConfig == nil {
		return "", fmt.Errorf("OpenVPN not configured")
	}

	client, exists := s.openvpnClients[clientID]
	if !exists {
		return "", fmt.Errorf("client %s not found", clientID)
	}

	proto := s.openvpnConfig.Protocol
	if proto == "" {
		proto = "udp"
	}

	cipher := s.openvpnConfig.Cipher
	if cipher == "" {
		cipher = "AES-256-GCM"
	}

	config := fmt.Sprintf(`client
dev tun
proto %s
remote <SERVER_ADDRESS> %d
resolv-retry infinite
nobind
persist-key
persist-tun
cipher %s
auth SHA256
verb 3

<cert>
%s
</cert>

<key>
%s
</key>

<ca>
<CA_CERTIFICATE>
</ca>
`, proto, s.openvpnConfig.Port, cipher, client.Certificate, client.PrivateKey)

	return config, nil
}

// MarshalJSON helper for testing.
func (s *Service) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.Marshal(struct {
		WgInterfaces  map[string]*WireGuardInterface `json:"wg_interfaces"`
		OpenVPNConfig *OpenVPNConfig                  `json:"openvpn_config"`
		Users         map[string]*VPNUser             `json:"users"`
		Devices       map[string]*VPNDevice            `json:"devices"`
	}{
		WgInterfaces:  s.wgInterfaces,
		OpenVPNConfig: s.openvpnConfig,
		Users:         s.users,
		Devices:       s.devices,
	})
}

// ==================== Fail2Ban 管理 ====================

// GetFail2BanStatus 获取 Fail2Ban 状态
func (s *Service) GetFail2BanStatus() Fail2BanStatus {
	return s.fail2ban.GetStatus()
}

// Fail2BanUnblock 手动解封指定IP
func (s *Service) Fail2BanUnblock(ip string) error {
	return s.fail2ban.Unblock(ip)
}

// RecordLoginFail 记录登录失败（供其他模块调用）
func (s *Service) RecordLoginFail(ip, username string) {
	s.fail2ban.RecordFailAttempt(ip, username)
}

// IsIPBanned 检查IP是否被封禁
func (s *Service) IsIPBanned(ip string) bool {
	return s.fail2ban.IsBanned(ip)
}

// AddFail2BanWhiteList 将IP加入白名单
func (s *Service) AddFail2BanWhiteList(ip string) {
	s.fail2ban.AddToWhiteList(ip)
}

// RemoveFail2BanWhiteList 将IP从白名单移除
func (s *Service) RemoveFail2BanWhiteList(ip string) error {
	return s.fail2ban.RemoveFromWhiteList(ip)
}

// ==================== Deep Copy Helpers ====================

func copyWGInterface(src *WireGuardInterface) *WireGuardInterface {
	if src == nil {
		return nil
	}
	dst := *src
	dst.DNS = make([]string, len(src.DNS))
	copy(dst.DNS, src.DNS)
	dst.Peers = make([]WireGuardPeer, len(src.Peers))
	copy(dst.Peers, src.Peers)
	return &dst
}
