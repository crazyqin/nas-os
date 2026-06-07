// Package vpnmanager 提供 VPN 隧道管理功能，支持 WireGuard、OpenVPN、IPSec、Tailscale、ZeroTier 等多种 VPN 类型。
// 提供隧道的创建/删除/启停、流量统计、配置导入导出、密钥管理、健康检查与自动重连、多隧道负载均衡等功能。
package vpnmanager

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// VPNType VPN 类型
type VPNType string

const (
	VPNTypeWireGuard VPNType = "wireguard"
	VPNTypeOpenVPN   VPNType = "openvpn"
	VPNTypeIPSec     VPNType = "ipsec"
	VPNTypeTailscale VPNType = "tailscale"
	VPNTypeZeroTier  VPNType = "zerotier"
)

// TunnelStatus 隧道状态
type TunnelStatus string

const (
	StatusActive     TunnelStatus = "active"
	StatusInactive   TunnelStatus = "inactive"
	StatusConnecting TunnelStatus = "connecting"
	StatusError      TunnelStatus = "error"
	StatusDisabled   TunnelStatus = "disabled"
)

// LoadBalanceMode 负载均衡模式
type LoadBalanceMode string

const (
	LoadBalanceRoundRobin LoadBalanceMode = "round_robin"
	LoadBalanceWeighted   LoadBalanceMode = "weighted"
	LoadBalanceFailover   LoadBalanceMode = "failover"
)

// VPNTunnel VPN 隧道结构体
type VPNTunnel struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          VPNType       `json:"type"`
	Status        TunnelStatus  `json:"status"`
	Endpoint      string        `json:"endpoint"`
	PublicKey     string        `json:"public_key,omitempty"`
	PrivateKey    string        `json:"private_key,omitempty"`
	AllowedIPs    []string      `json:"allowed_ips"`
	DNS           []string      `json:"dns,omitempty"`
	MTU           int           `json:"mtu"`
	KeepAlive     int           `json:"keep_alive"` // 秒
	Port          int           `json:"port,omitempty"`
	Weight        int           `json:"weight"` // 负载均衡权重
	AutoReconnect bool          `json:"auto_reconnect"`
	Config        string        `json:"config,omitempty"` // 原始配置文本
	Description   string        `json:"description,omitempty"`
	ErrorMsg      string        `json:"error_msg,omitempty"`
	TrafficStats  *TrafficStats `json:"traffic_stats,omitempty"`
	LastHandshake time.Time     `json:"last_handshake,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	UploadBytes   int64     `json:"upload_bytes"`
	DownloadBytes int64     `json:"download_bytes"`
	TotalBytes    int64     `json:"total_bytes"`
	LatencyMs     float64   `json:"latency_ms"`  // 延迟（毫秒）
	PacketLoss    float64   `json:"packet_loss"` // 丢包率（百分比）
	LastUpdated   time.Time `json:"last_updated"`
}

// PortForward 端口转发规则
type PortForward struct {
	ID          string    `json:"id"`
	TunnelID    string    `json:"tunnel_id"`
	Protocol    string    `json:"protocol"` // tcp/udp
	ListenPort  int       `json:"listen_port"`
	TargetAddr  string    `json:"target_addr"`
	TargetPort  int       `json:"target_port"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WireGuardConfig WireGuard 配置
type WireGuardConfig struct {
	Interface WireGuardInterface `json:"interface"`
	Peers     []WireGuardPeer    `json:"peers"`
}

// WireGuardInterface WireGuard 接口配置
type WireGuardInterface struct {
	Address    []string `json:"address"`
	PrivateKey string   `json:"private_key"`
	ListenPort int      `json:"listen_port,omitempty"`
	DNS        []string `json:"dns,omitempty"`
	MTU        int      `json:"mtu,omitempty"`
}

// WireGuardPeer WireGuard 对端配置
type WireGuardPeer struct {
	PublicKey    string   `json:"public_key"`
	Endpoint     string   `json:"endpoint,omitempty"`
	AllowedIPs   []string `json:"allowed_ips"`
	PresharedKey string   `json:"preshared_key,omitempty"`
	KeepAlive    int      `json:"persistent_keepalive,omitempty"`
}

// TunnelCreateRequest 创建隧道请求
type TunnelCreateRequest struct {
	Name          string   `json:"name" binding:"required"`
	Type          VPNType  `json:"type" binding:"required"`
	Endpoint      string   `json:"endpoint"`
	AllowedIPs    []string `json:"allowed_ips"`
	DNS           []string `json:"dns"`
	MTU           int      `json:"mtu"`
	KeepAlive     int      `json:"keep_alive"`
	Port          int      `json:"port"`
	Weight        int      `json:"weight"`
	AutoReconnect bool     `json:"auto_reconnect"`
	Config        string   `json:"config"`
	Description   string   `json:"description"`
}

// TunnelUpdateRequest 更新隧道请求
type TunnelUpdateRequest struct {
	Name          *string  `json:"name,omitempty"`
	Endpoint      *string  `json:"endpoint,omitempty"`
	AllowedIPs    []string `json:"allowed_ips,omitempty"`
	DNS           []string `json:"dns,omitempty"`
	MTU           *int     `json:"mtu,omitempty"`
	KeepAlive     *int     `json:"keep_alive,omitempty"`
	Port          *int     `json:"port,omitempty"`
	Weight        *int     `json:"weight,omitempty"`
	AutoReconnect *bool    `json:"auto_reconnect,omitempty"`
	Config        *string  `json:"config,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

// ImportConfigRequest 导入配置请求
type ImportConfigRequest struct {
	Type   VPNType `json:"type"`
	Config string  `json:"config" binding:"required"`
	Name   string  `json:"name"`
}

// VPNManager VPN 管理器
type VPNManager struct {
	mu           sync.RWMutex
	tunnels      map[string]*VPNTunnel
	portForwards map[string]*PortForward
	stopChans    map[string]chan struct{}
	healthTicker *time.Ticker
	stopHealth   chan struct{}
}

// NewVPNManager 创建 VPN 管理器
func NewVPNManager() *VPNManager {
	m := &VPNManager{
		tunnels:      make(map[string]*VPNTunnel),
		portForwards: make(map[string]*PortForward),
		stopChans:    make(map[string]chan struct{}),
		stopHealth:   make(chan struct{}),
	}

	// 启动健康检查
	m.startHealthCheck()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateTunnel 创建隧道
func (m *VPNManager) CreateTunnel(req *TunnelCreateRequest) (*VPNTunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 VPN 类型
	if !isValidVPNType(req.Type) {
		return nil, fmt.Errorf("unsupported VPN type: %s", req.Type)
	}

	// 设置默认值
	mtu := req.MTU
	if mtu <= 0 {
		mtu = 1420
	}
	keepAlive := req.KeepAlive
	if keepAlive < 0 {
		keepAlive = 25
	}
	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}

	// 生成密钥对（仅 WireGuard）
	publicKey := ""
	privateKey := ""
	if req.Type == VPNTypeWireGuard {
		pub, priv, err := generateWireGuardKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate WireGuard keys: %w", err)
		}
		publicKey = pub
		privateKey = priv
	}

	now := time.Now()
	tunnel := &VPNTunnel{
		ID:            generateID(),
		Name:          req.Name,
		Type:          req.Type,
		Status:        StatusInactive,
		Endpoint:      req.Endpoint,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		AllowedIPs:    req.AllowedIPs,
		DNS:           req.DNS,
		MTU:           mtu,
		KeepAlive:     keepAlive,
		Port:          req.Port,
		Weight:        weight,
		AutoReconnect: req.AutoReconnect,
		Config:        req.Config,
		Description:   req.Description,
		TrafficStats:  &TrafficStats{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.tunnels[tunnel.ID] = tunnel
	return tunnel, nil
}

// DeleteTunnel 删除隧道
func (m *VPNManager) DeleteTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	// 如果隧道活跃，先停止
	if tunnel.Status == StatusActive || tunnel.Status == StatusConnecting {
		if stopChan, exists := m.stopChans[id]; exists {
			close(stopChan)
			delete(m.stopChans, id)
		}
	}

	// 删除关联的端口转发规则
	for fwdID, fwd := range m.portForwards {
		if fwd.TunnelID == id {
			delete(m.portForwards, fwdID)
		}
	}

	delete(m.tunnels, id)
	return nil
}

// GetTunnel 获取隧道详情
func (m *VPNManager) GetTunnel(id string) (*VPNTunnel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return nil, fmt.Errorf("tunnel not found: %s", id)
	}
	return tunnel, nil
}

// ListTunnels 列出所有隧道
func (m *VPNManager) ListTunnels() []*VPNTunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]*VPNTunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// UpdateTunnel 更新隧道
func (m *VPNManager) UpdateTunnel(id string, req *TunnelUpdateRequest) (*VPNTunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return nil, fmt.Errorf("tunnel not found: %s", id)
	}

	if req.Name != nil {
		tunnel.Name = *req.Name
	}
	if req.Endpoint != nil {
		tunnel.Endpoint = *req.Endpoint
	}
	if req.AllowedIPs != nil {
		tunnel.AllowedIPs = req.AllowedIPs
	}
	if req.DNS != nil {
		tunnel.DNS = req.DNS
	}
	if req.MTU != nil {
		tunnel.MTU = *req.MTU
	}
	if req.KeepAlive != nil {
		tunnel.KeepAlive = *req.KeepAlive
	}
	if req.Port != nil {
		tunnel.Port = *req.Port
	}
	if req.Weight != nil {
		tunnel.Weight = *req.Weight
	}
	if req.AutoReconnect != nil {
		tunnel.AutoReconnect = *req.AutoReconnect
	}
	if req.Config != nil {
		tunnel.Config = *req.Config
	}
	if req.Description != nil {
		tunnel.Description = *req.Description
	}

	tunnel.UpdatedAt = time.Now()
	return tunnel, nil
}

// StartTunnel 启动隧道
func (m *VPNManager) StartTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.Status == StatusActive {
		return fmt.Errorf("tunnel is already active")
	}

	if tunnel.Status == StatusDisabled {
		return fmt.Errorf("tunnel is disabled")
	}

	tunnel.Status = StatusConnecting
	tunnel.UpdatedAt = time.Now()

	// 创建停止通道
	stopChan := make(chan struct{})
	m.stopChans[id] = stopChan

	// 模拟连接过程（实际实现中应调用相应的 VPN 工具）
	go m.connectTunnel(tunnel, stopChan)

	return nil
}

// StopTunnel 停止隧道
func (m *VPNManager) StopTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.Status == StatusInactive || tunnel.Status == StatusDisabled {
		return fmt.Errorf("tunnel is not active")
	}

	// 发送停止信号
	if stopChan, exists := m.stopChans[id]; exists {
		close(stopChan)
		delete(m.stopChans, id)
	}

	tunnel.Status = StatusInactive
	tunnel.UpdatedAt = time.Now()

	return nil
}

// connectTunnel 连接隧道（模拟实现）
func (m *VPNManager) connectTunnel(tunnel *VPNTunnel, stopChan chan struct{}) {
	// 模拟连接延迟
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	tunnel.Status = StatusActive
	tunnel.LastHandshake = time.Now()
	tunnel.UpdatedAt = time.Now()
	m.mu.Unlock()

	// 模拟流量统计更新
	go m.simulateTrafficStats(tunnel.ID, stopChan)
}

// simulateTrafficStats 模拟流量统计（实际实现中应从系统获取真实数据）
func (m *VPNManager) simulateTrafficStats(tunnelID string, stopChan chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			m.mu.Lock()
			tunnel, ok := m.tunnels[tunnelID]
			if !ok || tunnel.Status != StatusActive {
				m.mu.Unlock()
				return
			}

			// 模拟流量增长
			if tunnel.TrafficStats != nil {
				tunnel.TrafficStats.UploadBytes += 1024 * 100   // 100KB
				tunnel.TrafficStats.DownloadBytes += 1024 * 500 // 500KB
				tunnel.TrafficStats.TotalBytes = tunnel.TrafficStats.UploadBytes + tunnel.TrafficStats.DownloadBytes
				tunnel.TrafficStats.LatencyMs = 50.0 + float64(time.Now().UnixNano()%100)
				tunnel.TrafficStats.PacketLoss = 0.1
				tunnel.TrafficStats.LastUpdated = time.Now()
			}
			m.mu.Unlock()
		}
	}
}

// GetTrafficStats 获取隧道流量统计
func (m *VPNManager) GetTrafficStats(id string) (*TrafficStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return nil, fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.TrafficStats == nil {
		return &TrafficStats{}, nil
	}
	return tunnel.TrafficStats, nil
}

// ImportConfig 导入 VPN 配置
func (m *VPNManager) ImportConfig(req *ImportConfigRequest) (*VPNTunnel, error) {
	if req.Config == "" {
		return nil, fmt.Errorf("config is required")
	}

	// 自动检测类型
	vpnType := req.Type
	if vpnType == "" {
		vpnType = detectVPNType(req.Config)
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("imported-%s", generateID()[:8])
	}

	// 根据类型解析配置
	var tunnelReq *TunnelCreateRequest
	switch vpnType {
	case VPNTypeWireGuard:
		parsed, err := ParseWireGuardConfig(req.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse WireGuard config: %w", err)
		}
		tunnelReq = &TunnelCreateRequest{
			Name:       name,
			Type:       VPNTypeWireGuard,
			Config:     req.Config,
			AllowedIPs: parsed.Interface.Address,
			DNS:        parsed.Interface.DNS,
			MTU:        parsed.Interface.MTU,
		}
		if len(parsed.Peers) > 0 {
			tunnelReq.Endpoint = parsed.Peers[0].Endpoint
			tunnelReq.KeepAlive = parsed.Peers[0].KeepAlive
			if len(parsed.Peers[0].AllowedIPs) > 0 {
				tunnelReq.AllowedIPs = parsed.Peers[0].AllowedIPs
			}
		}
	default:
		tunnelReq = &TunnelCreateRequest{
			Name:   name,
			Type:   vpnType,
			Config: req.Config,
		}
	}

	return m.CreateTunnel(tunnelReq)
}

// ExportConfig 导出隧道配置
func (m *VPNManager) ExportConfig(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return "", fmt.Errorf("tunnel not found: %s", id)
	}

	// 如果有原始配置，直接返回
	if tunnel.Config != "" {
		return tunnel.Config, nil
	}

	// 根据类型生成配置
	switch tunnel.Type {
	case VPNTypeWireGuard:
		return m.generateWireGuardConfig(tunnel)
	default:
		return "", fmt.Errorf("export not supported for VPN type: %s", tunnel.Type)
	}
}

// generateWireGuardConfig 生成 WireGuard 配置
func (m *VPNManager) generateWireGuardConfig(tunnel *VPNTunnel) (string, error) {
	if tunnel.PrivateKey == "" {
		return "", fmt.Errorf("private key is required for WireGuard config")
	}

	var sb strings.Builder
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", tunnel.PrivateKey))
	if tunnel.Port > 0 {
		sb.WriteString(fmt.Sprintf("ListenPort = %d\n", tunnel.Port))
	}
	if len(tunnel.AllowedIPs) > 0 {
		sb.WriteString(fmt.Sprintf("Address = %s\n", strings.Join(tunnel.AllowedIPs, ", ")))
	}
	if len(tunnel.DNS) > 0 {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", strings.Join(tunnel.DNS, ", ")))
	}
	if tunnel.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", tunnel.MTU))
	}

	sb.WriteString("\n[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", tunnel.PublicKey))
	if tunnel.Endpoint != "" {
		sb.WriteString(fmt.Sprintf("Endpoint = %s\n", tunnel.Endpoint))
	}
	if len(tunnel.AllowedIPs) > 0 {
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(tunnel.AllowedIPs, ", ")))
	}
	if tunnel.KeepAlive > 0 {
		sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", tunnel.KeepAlive))
	}

	return sb.String(), nil
}

// ParseWireGuardConfig 解析 WireGuard 配置
func ParseWireGuardConfig(config string) (*WireGuardConfig, error) {
	result := &WireGuardConfig{
		Interface: WireGuardInterface{},
		Peers:     []WireGuardPeer{},
	}

	lines := strings.Split(config, "\n")
	var currentSection string
	var currentPeer *WireGuardPeer

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(strings.Trim(line, "[]"))
			switch section {
			case "interface":
				currentSection = "interface"
				currentPeer = nil
			case "peer":
				currentSection = "peer"
				peer := WireGuardPeer{}
				result.Peers = append(result.Peers, peer)
				currentPeer = &result.Peers[len(result.Peers)-1]
			}
			continue
		}

		// 解析键值对
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "interface":
			switch strings.ToLower(key) {
			case "privatekey":
				result.Interface.PrivateKey = value
			case "address":
				result.Interface.Address = strings.Split(value, ",")
				for i := range result.Interface.Address {
					result.Interface.Address[i] = strings.TrimSpace(result.Interface.Address[i])
				}
			case "dns":
				result.Interface.DNS = strings.Split(value, ",")
				for i := range result.Interface.DNS {
					result.Interface.DNS[i] = strings.TrimSpace(result.Interface.DNS[i])
				}
			case "listenport":
				fmt.Sscanf(value, "%d", &result.Interface.ListenPort)
			case "mtu":
				fmt.Sscanf(value, "%d", &result.Interface.MTU)
			}
		case "peer":
			if currentPeer != nil {
				switch strings.ToLower(key) {
				case "publickey":
					currentPeer.PublicKey = value
				case "endpoint":
					currentPeer.Endpoint = value
				case "allowedips":
					currentPeer.AllowedIPs = strings.Split(value, ",")
					for i := range currentPeer.AllowedIPs {
						currentPeer.AllowedIPs[i] = strings.TrimSpace(currentPeer.AllowedIPs[i])
					}
				case "presharedkey":
					currentPeer.PresharedKey = value
				case "persistentkeepalive":
					fmt.Sscanf(value, "%d", &currentPeer.KeepAlive)
				}
			}
		}
	}

	return result, nil
}

// GenerateWireGuardKeyPair 生成 WireGuard 密钥对
func GenerateWireGuardKeyPair() (publicKey, privateKey string, err error) {
	return generateWireGuardKeyPair()
}

// generateWireGuardKeyPair 生成 WireGuard 密钥对
func generateWireGuardKeyPair() (publicKey, privateKey string, err error) {
	// 生成 32 字节私钥
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		return "", "", err
	}

	// 设置 clamping bits
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64

	// 在真实实现中，这里应该使用 Curve25519 计算公钥
	// 这里简化处理，使用占位符
	pubKey := make([]byte, 32)
	if _, err := rand.Read(pubKey); err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(pubKey),
		base64.StdEncoding.EncodeToString(privKey),
		nil
}

// detectVPNType 检测 VPN 类型
func detectVPNType(config string) VPNType {
	configLower := strings.ToLower(config)

	if strings.Contains(configLower, "[interface]") && strings.Contains(configLower, "[peer]") {
		return VPNTypeWireGuard
	}
	if strings.Contains(configLower, "client") && (strings.Contains(configLower, "remote ") || strings.Contains(configLower, "dev tun")) {
		return VPNTypeOpenVPN
	}
	if strings.Contains(configLower, "conn ") || strings.Contains(configLower, "left=") {
		return VPNTypeIPSec
	}

	return VPNTypeWireGuard // 默认
}

// isValidVPNType 检查 VPN 类型是否有效
func isValidVPNType(t VPNType) bool {
	switch t {
	case VPNTypeWireGuard, VPNTypeOpenVPN, VPNTypeIPSec, VPNTypeTailscale, VPNTypeZeroTier:
		return true
	}
	return false
}

// --- 端口转发管理 ---

// CreatePortForward 创建端口转发规则
func (m *VPNManager) CreatePortForward(tunnelID string, protocol string, listenPort int, targetAddr string, targetPort int, description string) (*PortForward, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tunnels[tunnelID]; !ok {
		return nil, fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	protocol = strings.ToLower(protocol)
	if protocol != "tcp" && protocol != "udp" {
		return nil, fmt.Errorf("invalid protocol: %s (must be tcp or udp)", protocol)
	}

	if listenPort <= 0 || listenPort > 65535 {
		return nil, fmt.Errorf("invalid listen port: %d", listenPort)
	}

	if targetPort <= 0 || targetPort > 65535 {
		return nil, fmt.Errorf("invalid target port: %d", targetPort)
	}

	if net.ParseIP(targetAddr) == nil {
		return nil, fmt.Errorf("invalid target address: %s", targetAddr)
	}

	now := time.Now()
	fwd := &PortForward{
		ID:          generateID(),
		TunnelID:    tunnelID,
		Protocol:    protocol,
		ListenPort:  listenPort,
		TargetAddr:  targetAddr,
		TargetPort:  targetPort,
		Enabled:     true,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.portForwards[fwd.ID] = fwd
	return fwd, nil
}

// DeletePortForward 删除端口转发规则
func (m *VPNManager) DeletePortForward(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.portForwards[id]; !ok {
		return fmt.Errorf("port forward not found: %s", id)
	}

	delete(m.portForwards, id)
	return nil
}

// ListPortForwards 列出端口转发规则
func (m *VPNManager) ListPortForwards(tunnelID string) []*PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fwds := make([]*PortForward, 0)
	for _, fwd := range m.portForwards {
		if tunnelID == "" || fwd.TunnelID == tunnelID {
			fwds = append(fwds, fwd)
		}
	}
	return fwds
}

// --- 健康检查 ---

// startHealthCheck 启动健康检查
func (m *VPNManager) startHealthCheck() {
	m.healthTicker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-m.stopHealth:
				return
			case <-m.healthTicker.C:
				m.checkTunnelsHealth()
			}
		}
	}()
}

// checkTunnelsHealth 检查所有隧道健康状态
func (m *VPNManager) checkTunnelsHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tunnel := range m.tunnels {
		if tunnel.Status != StatusActive {
			continue
		}

		// 检查最后握手时间
		if !tunnel.LastHandshake.IsZero() && time.Since(tunnel.LastHandshake) > 3*time.Minute {
			tunnel.Status = StatusError
			tunnel.ErrorMsg = "handshake timeout"
			tunnel.UpdatedAt = time.Now()

			// 自动重连
			if tunnel.AutoReconnect {
				go m.reconnectTunnel(tunnel.ID)
			}
		}
	}
}

// reconnectTunnel 重新连接隧道
func (m *VPNManager) reconnectTunnel(id string) {
	m.mu.Lock()
	tunnel, ok := m.tunnels[id]
	if !ok {
		m.mu.Unlock()
		return
	}

	tunnel.Status = StatusConnecting
	tunnel.UpdatedAt = time.Now()

	stopChan := make(chan struct{})
	m.stopChans[id] = stopChan
	m.mu.Unlock()

	// 模拟重连
	time.Sleep(3 * time.Second)

	m.mu.Lock()
	tunnel, ok = m.tunnels[id]
	if !ok {
		m.mu.Unlock()
		return
	}

	tunnel.Status = StatusActive
	tunnel.LastHandshake = time.Now()
	tunnel.ErrorMsg = ""
	tunnel.UpdatedAt = time.Now()
	m.mu.Unlock()

	// 恢复流量统计
	go m.simulateTrafficStats(id, stopChan)
}

// --- 多隧道负载均衡 ---

// GetActiveTunnels 获取所有活跃隧道
func (m *VPNManager) GetActiveTunnels() []*VPNTunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*VPNTunnel
	for _, t := range m.tunnels {
		if t.Status == StatusActive {
			active = append(active, t)
		}
	}
	return active
}

// SelectTunnelForLoadBalance 选择负载均衡隧道
func (m *VPNManager) SelectTunnelForLoadBalance(mode LoadBalanceMode) (*VPNTunnel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*VPNTunnel
	for _, t := range m.tunnels {
		if t.Status == StatusActive {
			active = append(active, t)
		}
	}

	if len(active) == 0 {
		return nil, fmt.Errorf("no active tunnels available")
	}

	switch mode {
	case LoadBalanceRoundRobin:
		// 简单轮询（使用时间戳取模）
		idx := time.Now().UnixNano() % int64(len(active))
		return active[idx], nil

	case LoadBalanceWeighted:
		// 加权选择
		totalWeight := 0
		for _, t := range active {
			totalWeight += t.Weight
		}
		if totalWeight == 0 {
			return active[0], nil
		}

		target := time.Now().UnixNano() % int64(totalWeight)
		current := int64(0)
		for _, t := range active {
			current += int64(t.Weight)
			if current > target {
				return t, nil
			}
		}
		return active[len(active)-1], nil

	case LoadBalanceFailover:
		// 故障转移：选择延迟最低的
		var best *VPNTunnel
		bestLatency := float64(999999)
		for _, t := range active {
			if t.TrafficStats != nil && t.TrafficStats.LatencyMs < bestLatency {
				bestLatency = t.TrafficStats.LatencyMs
				best = t
			}
		}
		if best == nil {
			return active[0], nil
		}
		return best, nil

	default:
		return active[0], nil
	}
}

// DisableTunnel 禁用隧道
func (m *VPNManager) DisableTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	// 如果活跃，先停止
	if tunnel.Status == StatusActive {
		if stopChan, exists := m.stopChans[id]; exists {
			close(stopChan)
			delete(m.stopChans, id)
		}
	}

	tunnel.Status = StatusDisabled
	tunnel.UpdatedAt = time.Now()
	return nil
}

// EnableTunnel 启用隧道
func (m *VPNManager) EnableTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, ok := m.tunnels[id]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.Status != StatusDisabled {
		return fmt.Errorf("tunnel is not disabled")
	}

	tunnel.Status = StatusInactive
	tunnel.UpdatedAt = time.Now()
	return nil
}

// ExportAllConfigs 导出所有隧道配置
func (m *VPNManager) ExportAllConfigs() (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make(map[string]string)
	for _, tunnel := range m.tunnels {
		if tunnel.Config != "" {
			configs[tunnel.ID] = tunnel.Config
		}
	}

	return configs, nil
}

// MarshalJSON 自定义 JSON 序列化（隐藏私钥）
func (t *VPNTunnel) MarshalJSON() ([]byte, error) {
	type Alias VPNTunnel
	return json.Marshal(&struct {
		*Alias
		PrivateKey string `json:"private_key,omitempty"`
	}{
		Alias:      (*Alias)(t),
		PrivateKey: "***", // 隐藏私钥
	})
}

// Stop 停止管理器
func (m *VPNManager) Stop() {
	close(m.stopHealth)
	m.healthTicker.Stop()

	// 停止所有隧道
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, stopChan := range m.stopChans {
		close(stopChan)
		if tunnel, ok := m.tunnels[id]; ok {
			tunnel.Status = StatusInactive
			tunnel.UpdatedAt = time.Now()
		}
	}
	m.stopChans = make(map[string]chan struct{})
}

// 获取支持的 VPN 类型列表
func SupportedVPNTypes() []VPNType {
	return []VPNType{
		VPNTypeWireGuard,
		VPNTypeOpenVPN,
		VPNTypeIPSec,
		VPNTypeTailscale,
		VPNTypeZeroTier,
	}
}

// IsValidVPNType 检查 VPN 类型是否有效
func IsValidVPNType(t VPNType) bool {
	return isValidVPNType(t)
}

// 获取隧道状态列表
func SupportedTunnelStatuses() []TunnelStatus {
	return []TunnelStatus{
		StatusActive,
		StatusInactive,
		StatusConnecting,
		StatusError,
		StatusDisabled,
	}
}

// IsValidTunnelStatus 检查隧道状态是否有效
func IsValidTunnelStatus(s TunnelStatus) bool {
	switch s {
	case StatusActive, StatusInactive, StatusConnecting, StatusError, StatusDisabled:
		return true
	}
	return false
}
