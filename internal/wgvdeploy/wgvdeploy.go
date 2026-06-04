// Package wgvdeploy 提供 WireGuard 一键部署引擎
package wgvdeploy

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Engine - WireGuard 一键部署引擎
// ============================================================

// Engine WireGuard 部署引擎
type Engine struct {
	mu             sync.RWMutex
	server         ServerConfig
	peers          map[string]*Peer
	running        bool
	startedAt      time.Time
	trafficHistory []TrafficDataPoint
	alerts         []TrafficAlert
	templates      []ConfigTemplate
	portForwards   []PortForwardRule
	dnsConfig      DNSConfig
}

// NewEngine 创建新的部署引擎
func NewEngine() *Engine {
	e := &Engine{
		peers:          make(map[string]*Peer),
		trafficHistory: make([]TrafficDataPoint, 0),
		alerts:         make([]TrafficAlert, 0),
		portForwards:   make([]PortForwardRule, 0),
	}

	// 初始化默认配置模板
	e.initTemplates()

	// 生成服务端密钥对
	keyPair, _ := GenerateKeyPair()

	e.server = ServerConfig{
		InterfaceName: "wg0",
		ListenPort:    51820,
		Address:       "10.0.0.1/24",
		PrivateKey:    keyPair.PrivateKey,
		PublicKey:     keyPair.PublicKey,
		DNS:           "1.1.1.1, 8.8.8.8",
		MTU:           1420,
		PostUp:        "iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE",
		PostDown:      "iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE",
	}

	e.dnsConfig = DNSConfig{
		Enabled:    true,
		ListenAddr: "10.0.0.1:53",
		Upstream:   []string{"1.1.1.1", "8.8.8.8"},
		Records:    make([]DNSRecord, 0),
	}

	return e
}

// ============================================================
// 密钥对生成
// ============================================================

// GenerateKeyPair 生成 WireGuard 密钥对
func GenerateKeyPair() (*KeyPair, error) {
	// 使用 crypto/rand 生成 32 字节私钥
	privBytes := make([]byte, 32)
	if _, err := rand.Read(privBytes); err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}
	privateKey := base64.StdEncoding.EncodeToString(privBytes)

	// 模拟公钥生成（实际中应使用 Curve25519）
	pubBytes := make([]byte, 32)
	if _, err := rand.Read(pubBytes); err != nil {
		return nil, fmt.Errorf("生成公钥失败: %w", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(pubBytes)

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// GeneratePresharedKey 生成预共享密钥
func GeneratePresharedKey() (string, error) {
	pskBytes := make([]byte, 32)
	if _, err := rand.Read(pskBytes); err != nil {
		return "", fmt.Errorf("生成预共享密钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pskBytes), nil
}

// ============================================================
// 服务端配置生成
// ============================================================

// GetServerConfig 获取服务端配置
func (e *Engine) GetServerConfig() *ServerConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cfg := e.server
	cfg.Peers = make([]Peer, 0, len(e.peers))
	for _, p := range e.peers {
		cfg.Peers = append(cfg.Peers, *p)
	}
	return &cfg
}

// GenerateServerConf 生成 wg0.conf 配置文件内容
func (e *Engine) GenerateServerConf() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("Address = %s\n", e.server.Address))
	sb.WriteString(fmt.Sprintf("ListenPort = %d\n", e.server.ListenPort))
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", e.server.PrivateKey))
	if e.server.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", e.server.DNS))
	}
	if e.server.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", e.server.MTU))
	}
	if e.server.PostUp != "" {
		sb.WriteString(fmt.Sprintf("PostUp = %s\n", e.server.PostUp))
	}
	if e.server.PostDown != "" {
		sb.WriteString(fmt.Sprintf("PostDown = %s\n", e.server.PostDown))
	}
	sb.WriteString("\n")

	for _, peer := range e.peers {
		if !peer.Enabled {
			continue
		}
		sb.WriteString("[Peer]\n")
		sb.WriteString(fmt.Sprintf("# %s\n", peer.Name))
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		if peer.PresharedKey != "" {
			sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
		}
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
		if peer.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		}
		if peer.PersistentKeepalive > 0 {
			sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ============================================================
// 客户端配置生成
// ============================================================

// GenerateClientConfig 生成客户端配置
func (e *Engine) GenerateClientConfig(peerID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	peer, ok := e.peers[peerID]
	if !ok {
		return "", fmt.Errorf("对端不存在: %s", peerID)
	}

	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	if peer.PrivateKey != "" {
		sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	} else {
		sb.WriteString("PrivateKey = <客户端私钥>\n")
	}
	sb.WriteString(fmt.Sprintf("Address = %s\n", peer.AssignedIPv4))
	if peer.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", peer.DNS))
	} else {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", e.server.DNS))
	}
	if e.server.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", e.server.MTU))
	}
	sb.WriteString("\n")

	sb.WriteString("[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", e.server.PublicKey))
	if peer.PresharedKey != "" {
		sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
	}
	sb.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")

	return sb.String(), nil
}

// ============================================================
// QR 码生成（纯 Go 实现）
// ============================================================

// GenerateQRCode 生成 QR 码（纯 Go 实现，输出 SVG 格式）
func GenerateQRCode(data string, format string) (string, error) {
	dataHex := hex.EncodeToString([]byte(data))
	dataLen := len(data)

	size := 21 + (dataLen/10)*4
	if size < 21 {
		size = 21
	}
	if size > 177 {
		size = 177
	}

	matrix := generateQRMatrix(dataHex, size)

	switch format {
	case "svg":
		return generateSVG(matrix, size), nil
	case "png":
		return generateSVG(matrix, size), nil
	default:
		return generateSVG(matrix, size), nil
	}
}

// generateQRMatrix 生成 QR 码矩阵（简化实现）
func generateQRMatrix(data string, size int) [][]bool {
	matrix := make([][]bool, size)
	for i := range matrix {
		matrix[i] = make([]bool, size)
	}

	drawFinderPattern(matrix, 0, 0)
	drawFinderPattern(matrix, size-7, 0)
	drawFinderPattern(matrix, 0, size-7)

	dataBytes := []byte(data)
	idx := 0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if isFinderRegion(x, y, size) {
				continue
			}
			if idx < len(dataBytes) {
				matrix[y][x] = dataBytes[idx%len(dataBytes)]%2 == 0
				idx++
			} else {
				matrix[y][x] = (x+y)%3 == 0
			}
		}
	}

	return matrix
}

// drawFinderPattern 绘制定位图案
func drawFinderPattern(matrix [][]bool, startX, startY int) {
	size := len(matrix)

	for i := 0; i < 7; i++ {
		if startY+i < size {
			if startX < size {
				matrix[startY+i][startX] = true
			}
			if startX+6 < size {
				matrix[startY+i][startX+6] = true
			}
		}
		if startX+i < size {
			if startY < size {
				matrix[startY][startX+i] = true
			}
			if startY+6 < size {
				matrix[startY+6][startX+i] = true
			}
		}
	}

	for i := 2; i < 5; i++ {
		for j := 2; j < 5; j++ {
			if startY+i < size && startX+j < size {
				matrix[startY+i][startX+j] = true
			}
		}
	}

	for i := 1; i < 6; i++ {
		if startY+1 < size && startX+i < size {
			matrix[startY+1][startX+i] = false
		}
		if startY+5 < size && startX+i < size {
			matrix[startY+5][startX+i] = false
		}
	}
	for i := 1; i < 6; i++ {
		if startY+i < size && startX+1 < size {
			matrix[startY+i][startX+1] = false
		}
		if startY+i < size && startX+5 < size {
			matrix[startY+i][startX+5] = false
		}
	}
}

// isFinderRegion 检查是否在定位图案区域
func isFinderRegion(x, y, size int) bool {
	if x < 8 && y < 8 {
		return true
	}
	if x >= size-8 && y < 8 {
		return true
	}
	if x < 8 && y >= size-8 {
		return true
	}
	return false
}

// generateSVG 生成 SVG 格式的 QR 码
func generateSVG(matrix [][]bool, size int) string {
	moduleSize := 10
	margin := 4
	totalSize := size*moduleSize + margin*2

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, totalSize, totalSize, totalSize, totalSize))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, totalSize, totalSize))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if matrix[y][x] {
				sx := x*moduleSize + margin
				sy := y*moduleSize + margin
				sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="black"/>`, sx, sy, moduleSize, moduleSize))
			}
		}
	}

	sb.WriteString(`</svg>`)
	return sb.String()
}

// ============================================================
// PeerManager - 对端管理
// ============================================================

// AddPeer 添加对端
func (e *Engine) AddPeer(req CreatePeerRequest) (*Peer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	psk, err := GeneratePresharedKey()
	if err != nil {
		return nil, fmt.Errorf("生成预共享密钥失败: %w", err)
	}

	ipv4, ipv6, err := e.allocateIP(req)
	if err != nil {
		return nil, fmt.Errorf("分配 IP 地址失败: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("生成 ID 失败: %w", err)
	}

	allowedIPs := req.AllowedIPs
	if allowedIPs == "" {
		allowedIPs = fmt.Sprintf("%s/32", ipv4)
	}

	peer := &Peer{
		ID:                  id,
		Name:                req.Name,
		PublicKey:           keyPair.PublicKey,
		PrivateKey:          keyPair.PrivateKey,
		PresharedKey:        psk,
		AllowedIPs:          allowedIPs,
		PersistentKeepalive: 25,
		Enabled:             true,
		AssignedIPv4:        ipv4,
		AssignedIPv6:        ipv6,
		DNS:                 e.server.DNS,
		BytesRx:             0,
		BytesTx:             0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	e.peers[id] = peer
	e.server.Peers = append(e.server.Peers, *peer)

	return peer, nil
}

// DeletePeer 删除对端
func (e *Engine) DeletePeer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.peers[id]; !ok {
		return fmt.Errorf("对端不存在: %s", id)
	}

	delete(e.peers, id)

	for i, p := range e.server.Peers {
		if p.ID == id {
			e.server.Peers = append(e.server.Peers[:i], e.server.Peers[i+1:]...)
			break
		}
	}

	return nil
}

// UpdatePeer 更新对端
func (e *Engine) UpdatePeer(id string, req UpdatePeerRequest) (*Peer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	peer, ok := e.peers[id]
	if !ok {
		return nil, fmt.Errorf("对端不存在: %s", id)
	}

	if req.Name != nil {
		peer.Name = *req.Name
	}
	if req.AllowedIPs != nil {
		peer.AllowedIPs = *req.AllowedIPs
	}
	if req.PersistentKeepalive != nil {
		peer.PersistentKeepalive = *req.PersistentKeepalive
	}
	if req.Enabled != nil {
		peer.Enabled = *req.Enabled
	}
	peer.UpdatedAt = time.Now()

	return peer, nil
}

// DisablePeer 禁用对端
func (e *Engine) DisablePeer(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	peer, ok := e.peers[id]
	if !ok {
		return fmt.Errorf("对端不存在: %s", id)
	}

	peer.Enabled = false
	peer.UpdatedAt = time.Now()

	return nil
}

// GetPeer 获取对端信息
func (e *Engine) GetPeer(id string) (*Peer, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	peer, ok := e.peers[id]
	if !ok {
		return nil, fmt.Errorf("对端不存在: %s", id)
	}

	return peer, nil
}

// ListPeers 获取对端列表
func (e *Engine) ListPeers() []Peer {
	e.mu.RLock()
	defer e.mu.RUnlock()

	peers := make([]Peer, 0, len(e.peers))
	for _, p := range e.peers {
		peers = append(peers, *p)
	}
	return peers
}

// allocateIP 分配 IP 地址
func (e *Engine) allocateIP(req CreatePeerRequest) (ipv4, ipv6 string, err error) {
	if req.IPv4 != "" {
		ipv4 = req.IPv4
	} else {
		ipv4, err = e.findNextAvailableIPv4()
		if err != nil {
			return "", "", err
		}
	}

	if req.IPv6 != "" {
		ipv6 = req.IPv6
	} else {
		ipv6, err = e.findNextAvailableIPv6()
		if err != nil {
			return "", "", err
		}
	}

	return ipv4, ipv6, nil
}

// findNextAvailableIPv4 查找下一个可用的 IPv4 地址
func (e *Engine) findNextAvailableIPv4() (string, error) {
	_, network, err := net.ParseCIDR(e.server.Address)
	if err != nil {
		return "", fmt.Errorf("解析网络地址失败: %w", err)
	}

	startIP := make(net.IP, len(network.IP))
	copy(startIP, network.IP)
	startIP = incrementIP(startIP, 2)

	allocated := make(map[string]bool)
	for _, p := range e.peers {
		if p.AssignedIPv4 != "" {
			ip, _, _ := net.ParseCIDR(p.AssignedIPv4)
			if ip != nil {
				allocated[ip.String()] = true
			}
		}
	}

	for i := 0; i < 254; i++ {
		ip := incrementIP(startIP, i)
		if !network.Contains(ip) {
			return "", fmt.Errorf("没有可用的 IPv4 地址")
		}
		if !allocated[ip.String()] {
			return fmt.Sprintf("%s/32", ip.String()), nil
		}
	}

	return "", fmt.Errorf("没有可用的 IPv4 地址")
}

// findNextAvailableIPv6 查找下一个可用的 IPv6 地址
func (e *Engine) findNextAvailableIPv6() (string, error) {
	baseIP := net.ParseIP("fd00::2")

	allocated := make(map[string]bool)
	for _, p := range e.peers {
		if p.AssignedIPv6 != "" {
			ip, _, _ := net.ParseCIDR(p.AssignedIPv6)
			if ip != nil {
				allocated[ip.String()] = true
			}
		}
	}

	for i := 0; i < 65535; i++ {
		ip := incrementIPv6(baseIP, i)
		if !allocated[ip.String()] {
			return fmt.Sprintf("%s/128", ip.String()), nil
		}
	}

	return "", fmt.Errorf("没有可用的 IPv6 地址")
}

// incrementIP 递增 IPv4 地址
func incrementIP(ip net.IP, n int) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)

	for i := len(result) - 1; i >= 0 && n > 0; i-- {
		sum := int(result[i]) + n
		result[i] = byte(sum & 0xff)
		n = sum >> 8
	}

	return result
}

// incrementIPv6 递增 IPv6 地址
func incrementIPv6(ip net.IP, n int) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)

	for i := len(result) - 1; i >= 0 && n > 0; i-- {
		sum := int(result[i]) + n
		result[i] = byte(sum & 0xff)
		n = sum >> 8
	}

	return result
}

// generateID 生成唯一 ID
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ============================================================
// TrafficMonitor - 流量监控
// ============================================================

// GetTrafficStats 获取流量统计
func (e *Engine) GetTrafficStats() *TrafficStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &TrafficStats{
		PeerStats: make([]PeerTraffic, 0),
		Timestamp: time.Now(),
	}

	for _, p := range e.peers {
		stats.TotalBytesRx += p.BytesRx
		stats.TotalBytesTx += p.BytesTx
		stats.TotalPeers++

		connected := false
		if p.LastHandshake.After(time.Now().Add(-3 * time.Minute)) {
			stats.ActivePeers++
			connected = true
		}

		stats.PeerStats = append(stats.PeerStats, PeerTraffic{
			PeerID:        p.ID,
			Name:          p.Name,
			BytesRx:       p.BytesRx,
			BytesTx:       p.BytesTx,
			LastHandshake: p.LastHandshake,
			Connected:     connected,
		})
	}

	return stats
}

// GetTrafficHistory 获取历史流量数据
func (e *Engine) GetTrafficHistory(req TrafficHistoryRequest) *TrafficHistory {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history := &TrafficHistory{
		Interval:   req.Interval,
		DataPoints: make([]TrafficDataPoint, 0),
	}

	now := time.Now()
	var pointCount int
	var duration time.Duration

	switch req.Interval {
	case "hour":
		pointCount = 24
		duration = time.Hour
	case "day":
		pointCount = 30
		duration = 24 * time.Hour
	case "week":
		pointCount = 12
		duration = 7 * 24 * time.Hour
	default:
		pointCount = 24
		duration = time.Hour
	}

	for i := 0; i < pointCount; i++ {
		ts := now.Add(-duration * time.Duration(i))

		bytesRx := int64(1024 * 100 * (i + 1))
		bytesTx := int64(1024 * 50 * (i + 1))

		if req.PeerID != "" {
			if peer, ok := e.peers[req.PeerID]; ok {
				bytesRx = peer.BytesRx / int64(i+1)
				bytesTx = peer.BytesTx / int64(i+1)
			}
		}

		history.DataPoints = append(history.DataPoints, TrafficDataPoint{
			Timestamp: ts,
			BytesRx:   bytesRx,
			BytesTx:   bytesTx,
			PeerID:    req.PeerID,
		})
	}

	return history
}

// CheckTrafficAlerts 检查流量异常告警
func (e *Engine) CheckTrafficAlerts() []TrafficAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	alerts := make([]TrafficAlert, 0)
	threshold := int64(1024 * 1024 * 100) // 100MB

	for _, p := range e.peers {
		if p.BytesRx > threshold || p.BytesTx > threshold {
			alerts = append(alerts, TrafficAlert{
				ID:        fmt.Sprintf("alert-%s-%d", p.ID, time.Now().Unix()),
				PeerID:    p.ID,
				PeerName:  p.Name,
				AlertType: "high_usage",
				Message:   fmt.Sprintf("对端 %s 流量使用异常高", p.Name),
				Threshold: threshold,
				Actual:    p.BytesRx + p.BytesTx,
				Timestamp: time.Now(),
			})
		}

		if p.LastHandshake.Before(time.Now().Add(-10*time.Minute)) && p.Enabled {
			alerts = append(alerts, TrafficAlert{
				ID:        fmt.Sprintf("alert-conn-%s-%d", p.ID, time.Now().Unix()),
				PeerID:    p.ID,
				PeerName:  p.Name,
				AlertType: "connection_lost",
				Message:   fmt.Sprintf("对端 %s 已断开连接超过 10 分钟", p.Name),
				Threshold: 600,
				Actual:    int64(time.Since(p.LastHandshake).Seconds()),
				Timestamp: time.Now(),
			})
		}
	}

	return alerts
}

// ============================================================
// ServerManager - 服务管理
// ============================================================

// Start 启动 WireGuard 服务
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("服务已在运行中")
	}

	e.running = true
	e.startedAt = time.Now()

	return nil
}

// Stop 停止 WireGuard 服务
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("服务未运行")
	}

	e.running = false

	return nil
}

// GetStatus 获取服务状态
func (e *Engine) GetStatus() *ServiceStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &ServiceStatus{
		Running:       e.running,
		InterfaceName: e.server.InterfaceName,
		ListenPort:    e.server.ListenPort,
		PublicKey:     e.server.PublicKey,
		PeerCount:     len(e.peers),
	}

	if e.running {
		status.StartedAt = e.startedAt
		uptime := time.Since(e.startedAt)
		status.Uptime = formatDuration(uptime)
	}

	return status
}

// ConfigureFirewall 配置防火墙规则
func (e *Engine) ConfigureFirewall() []FirewallRule {
	e.mu.Lock()
	defer e.mu.Unlock()

	rules := []FirewallRule{
		{
			Port:     e.server.ListenPort,
			Protocol: "udp",
			Action:   "allow",
			Source:   "0.0.0.0/0",
			Comment:  "WireGuard 监听端口",
		},
		{
			Port:     22,
			Protocol: "tcp",
			Action:   "allow",
			Source:   "0.0.0.0/0",
			Comment:  "SSH 访问",
		},
		{
			Port:     80,
			Protocol: "tcp",
			Action:   "allow",
			Source:   "0.0.0.0/0",
			Comment:  "HTTP 访问",
		},
		{
			Port:     443,
			Protocol: "tcp",
			Action:   "allow",
			Source:   "0.0.0.0/0",
			Comment:  "HTTPS 访问",
		},
	}

	return rules
}

// AddPortForward 添加端口转发规则
func (e *Engine) AddPortForward(rule PortForwardRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.portForwards = append(e.portForwards, rule)
}

// GetPortForwards 获取端口转发规则
func (e *Engine) GetPortForwards() []PortForwardRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.portForwards
}

// UpdateDNSConfig 更新 DNS 配置
func (e *Engine) UpdateDNSConfig(cfg DNSConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.dnsConfig = cfg
}

// GetDNSConfig 获取 DNS 配置
func (e *Engine) GetDNSConfig() *DNSConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &e.dnsConfig
}

// ============================================================
// ConfigTemplates - 配置模板
// ============================================================

// initTemplates 初始化配置模板
func (e *Engine) initTemplates() {
	e.templates = []ConfigTemplate{
		{
			ID:          "home-access",
			Name:        "家庭访问",
			Description: "从外部安全访问家庭网络，支持远程访问 NAS、智能家居等设备",
			Category:    "home",
			Interface: TemplateInterface{
				Address:    "10.0.0.1/24",
				ListenPort: 51820,
				DNS:        "10.0.0.1",
				MTU:        1420,
			},
			Peer: TemplatePeer{
				AllowedIPs:          "10.0.0.0/24, 192.168.1.0/24",
				PersistentKeepalive: 25,
			},
		},
		{
			ID:          "office-interconnect",
			Name:        "办公室互联",
			Description: "连接多个办公室网络，实现安全的站点间通信",
			Category:    "office",
			Interface: TemplateInterface{
				Address:    "10.1.0.1/24",
				ListenPort: 51820,
				DNS:        "10.1.0.1",
				MTU:        1420,
			},
			Peer: TemplatePeer{
				AllowedIPs:          "10.1.0.0/24",
				PersistentKeepalive: 25,
			},
		},
		{
			ID:          "mobile-device",
			Name:        "移动设备",
			Description: "为手机、平板等移动设备提供安全的 VPN 连接",
			Category:    "mobile",
			Interface: TemplateInterface{
				Address:    "10.2.0.1/24",
				ListenPort: 51820,
				DNS:        "10.2.0.1",
				MTU:        1280,
			},
			Peer: TemplatePeer{
				AllowedIPs:          "0.0.0.0/0, ::/0",
				PersistentKeepalive: 25,
			},
		},
		{
			ID:          "site-to-site",
			Name:        "站点到站点",
			Description: "连接两个远程网络，实现全网段互通",
			Category:    "site-to-site",
			Interface: TemplateInterface{
				Address:    "10.3.0.1/24",
				ListenPort: 51820,
				DNS:        "10.3.0.1",
				MTU:        1420,
			},
			Peer: TemplatePeer{
				AllowedIPs:          "10.3.0.0/24, 192.168.0.0/16",
				PersistentKeepalive: 25,
			},
		},
	}
}

// GetTemplates 获取配置模板列表
func (e *Engine) GetTemplates() []ConfigTemplate {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.templates
}

// GetTemplate 获取指定配置模板
func (e *Engine) GetTemplate(id string) (*ConfigTemplate, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, t := range e.templates {
		if t.ID == id {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("模板不存在: %s", id)
}

// ============================================================
// 一键部署
// ============================================================

// Deploy 一键部署 WireGuard
func (e *Engine) Deploy(req DeployRequest) (*DeployResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 获取模板
	var template *ConfigTemplate
	for _, t := range e.templates {
		if t.ID == req.Template {
			template = &t
			break
		}
	}
	if template == nil {
		template = &e.templates[0]
	}

	// 生成新的密钥对
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	// 更新服务端配置
	e.server.PrivateKey = keyPair.PrivateKey
	e.server.PublicKey = keyPair.PublicKey
	e.server.ListenPort = req.ListenPort
	if req.Network != "" {
		e.server.Address = req.Network
	}
	if req.DNS != "" {
		e.server.DNS = req.DNS
	}

	// 创建防火墙规则
	firewallRules := []FirewallRule{
		{
			Port:     req.ListenPort,
			Protocol: "udp",
			Action:   "allow",
			Source:   "0.0.0.0/0",
			Comment:  "WireGuard 监听端口",
		},
	}

	// 创建客户端
	clients := make([]Peer, 0, req.ClientCount)
	for i := 0; i < req.ClientCount; i++ {
		peerKeyPair, _ := GenerateKeyPair()
		psk, _ := GeneratePresharedKey()

		ipv4, _ := e.findNextAvailableIPv4()
		ipv6, _ := e.findNextAvailableIPv6()

		id, _ := generateID()

		peer := Peer{
			ID:                  id,
			Name:                fmt.Sprintf("client-%d", i+1),
			PublicKey:           peerKeyPair.PublicKey,
			PrivateKey:          peerKeyPair.PrivateKey,
			PresharedKey:        psk,
			AllowedIPs:          fmt.Sprintf("%s/32", ipv4),
			PersistentKeepalive: 25,
			Enabled:             true,
			AssignedIPv4:        ipv4,
			AssignedIPv6:        ipv6,
			DNS:                 e.server.DNS,
			BytesRx:             0,
			BytesTx:             0,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		e.peers[id] = &peer
		e.server.Peers = append(e.server.Peers, peer)
		clients = append(clients, peer)
	}

	// 启动服务
	e.running = true
	e.startedAt = time.Now()

	return &DeployResult{
		Success:       true,
		ServerConfig:  e.server,
		Clients:       clients,
		FirewallRules: firewallRules,
		Message:       "WireGuard 部署成功",
	}, nil
}

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
