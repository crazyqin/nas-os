// Package zerotrust 提供零信任网络架构实现
package zerotrust

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// WireGuardPeer WireGuard 对等端.
type WireGuardPeer struct {
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	AllowedIPs  []string  `json:"allowedIPs"`
	Endpoint    string    `json:"endpoint,omitempty"`
	TrustLevel  TrustLevel `json:"trustLevel"`
	IsActive    bool      `json:"isActive"`
	LastHandshake time.Time `json:"lastHandshake,omitempty"`
	BytesSent   int64     `json:"bytesSent"`
	BytesRecv   int64     `json:"bytesRecv"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// WireGuardStats WireGuard 统计信息.
type WireGuardStats struct {
	Tunnels     int   `json:"tunnels"`
	Peers       int   `json:"peers"`
	ActivePeers int   `json:"activePeers"`
	BytesSent   int64 `json:"bytesSent"`
	BytesRecv   int64 `json:"bytesRecv"`
}

// WireGuardStatus WireGuard 状态.
type WireGuardStatus struct {
	Interface string          `json:"interface"`
	PublicKey string          `json:"publicKey"`
	ListenPort int            `json:"listenPort"`
	Peers     []*WireGuardPeer `json:"peers"`
	Stats     WireGuardStats  `json:"stats"`
}

// WireGuardManager WireGuard 管理器.
type WireGuardManager struct {
	mu           sync.RWMutex
	interfaceName string
	listenPort    int
	privateKey    string
	publicKey     string
	peers         map[string]*WireGuardPeer // publicKey -> peer
	running       bool
}

// NewWireGuardManager 创建 WireGuard 管理器.
func NewWireGuardManager(interfaceName string, listenPort int) (*WireGuardManager, error) {
	if interfaceName == "" {
		interfaceName = "wg0"
	}
	if listenPort == 0 {
		listenPort = 51820
	}

	mgr := &WireGuardManager{
		interfaceName: interfaceName,
		listenPort:    listenPort,
		peers:         make(map[string]*WireGuardPeer),
	}

	// 生成密钥对
	if err := mgr.generateKeys(); err != nil {
		return nil, fmt.Errorf("failed to generate wireguard keys: %w", err)
	}

	return mgr, nil
}

// generateKeys 生成 WireGuard 密钥对.
func (m *WireGuardManager) generateKeys() error {
	// 生成私钥
	privateKey, err := m.runWGCommand("genkey")
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}
	m.privateKey = strings.TrimSpace(privateKey)

	// 从私钥生成公钥
	publicKey, err := m.runWGCommand("pubkey", strings.NewReader(m.privateKey))
	if err != nil {
		return fmt.Errorf("failed to generate public key: %w", err)
	}
	m.publicKey = strings.TrimSpace(publicKey)

	log.Printf("[WireGuard] Generated key pair, public key: %s", m.publicKey[:16]+"...")
	return nil
}

// runWGCommand 执行 WireGuard 命令.
func (m *WireGuardManager) runWGCommand(args ...interface{}) (string, error) {
	var cmdArgs []string
	var stdin *strings.Reader

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			cmdArgs = append(cmdArgs, v)
		case *strings.Reader:
			stdin = v
		}
	}

	cmd := exec.Command("wg", cmdArgs...)
	if stdin != nil {
		cmd.Stdin = stdin
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wg command failed: %s, output: %s", err, string(output))
	}

	return string(output), nil
}

// Setup 初始化 WireGuard 接口.
func (m *WireGuardManager) Setup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("wireguard interface %s already running", m.interfaceName)
	}

	// 创建 WireGuard 接口
	if err := m.createInterface(); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// 配置接口
	if err := m.configureInterface(); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	m.running = true
	log.Printf("[WireGuard] Interface %s is up, listening on port %d", m.interfaceName, m.listenPort)
	return nil
}

// createInterface 创建 WireGuard 网络接口.
func (m *WireGuardManager) createInterface() error {
	// 使用 ip link 创建接口
	cmd := exec.Command("ip", "link", "add", m.interfaceName, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		// 如果接口已存在，先删除再重建
		if strings.Contains(string(output), "exists") {
			m.deleteInterface()
			cmd = exec.Command("ip", "link", "add", m.interfaceName, "type", "wireguard")
			if output, err = cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to create interface: %s, output: %s", err, string(output))
			}
		} else {
			return fmt.Errorf("failed to create interface: %s, output: %s", err, string(output))
		}
	}

	return nil
}

// configureInterface 配置 WireGuard 接口.
func (m *WireGuardManager) configureInterface() error {
	// 配置私钥和监听端口
	configCmd := fmt.Sprintf("echo '%s' | wg set %s private-key /dev/stdin listen-port %d",
		m.privateKey, m.interfaceName, m.listenPort)
	cmd := exec.Command("sh", "-c", configCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure interface: %s, output: %s", err, string(output))
	}

	// 设置接口 IP（使用 10.0.0.1/24 作为示例）
	cmd = exec.Command("ip", "addr", "add", "10.0.0.1/24", "dev", m.interfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		// 忽略 "already assigned" 错误
		if !strings.Contains(string(output), "RTNETLINK answers: File exists") {
			return fmt.Errorf("failed to set IP: %s, output: %s", err, string(output))
		}
	}

	// 启动接口
	cmd = exec.Command("ip", "link", "set", m.interfaceName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up interface: %s, output: %s", err, string(output))
	}

	return nil
}

// deleteInterface 删除 WireGuard 接口.
func (m *WireGuardManager) deleteInterface() {
	cmd := exec.Command("ip", "link", "delete", m.interfaceName)
	cmd.Run()
}

// AddPeer 添加对等端.
func (m *WireGuardManager) AddPeer(peer *WireGuardPeer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peer.PublicKey == "" {
		return fmt.Errorf("public key is required")
	}

	if _, exists := m.peers[peer.PublicKey]; exists {
		return fmt.Errorf("peer with public key %s already exists", peer.PublicKey[:16]+"...")
	}

	// 配置 WireGuard 对等端
	if err := m.configurePeer(peer); err != nil {
		return fmt.Errorf("failed to configure peer: %w", err)
	}

	peer.CreatedAt = time.Now()
	peer.UpdatedAt = time.Now()
	peer.IsActive = true

	m.peers[peer.PublicKey] = peer
	log.Printf("[WireGuard] Added peer: %s (%s)", peer.Name, peer.PublicKey[:16]+"...")
	return nil
}

// configurePeer 配置 WireGuard 对等端.
func (m *WireGuardManager) configurePeer(peer *WireGuardPeer) error {
	if !m.running {
		return fmt.Errorf("wireguard interface not running")
	}

	allowedIPs := strings.Join(peer.AllowedIPs, ",")

	args := []string{"set", m.interfaceName, "peer", peer.PublicKey}
	if allowedIPs != "" {
		args = append(args, "allowed-ips", allowedIPs)
	}
	if peer.Endpoint != "" {
		args = append(args, "endpoint", peer.Endpoint)
	}

	cmd := exec.Command("wg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add peer: %s, output: %s", err, string(output))
	}

	return nil
}

// RemovePeer 移除对等端.
func (m *WireGuardManager) RemovePeer(publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.peers[publicKey]
	if !exists {
		return fmt.Errorf("peer not found: %s", publicKey[:16]+"...")
	}

	// 从 WireGuard 移除对等端
	if m.running {
		cmd := exec.Command("wg", "set", m.interfaceName, "peer", publicKey, "remove")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to remove peer: %s, output: %s", err, string(output))
		}
	}

	delete(m.peers, publicKey)
	log.Printf("[WireGuard] Removed peer: %s (%s)", peer.Name, publicKey[:16]+"...")
	return nil
}

// RestartPeer 重启对等端连接.
func (m *WireGuardManager) RestartPeer(publicKey string) error {
	m.mu.RLock()
	peer, exists := m.peers[publicKey]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("peer not found: %s", publicKey[:16]+"...")
	}

	// 先移除再添加
	if err := m.RemovePeer(publicKey); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	if err := m.AddPeer(peer); err != nil {
		return fmt.Errorf("failed to re-add peer: %w", err)
	}

	log.Printf("[WireGuard] Restarted peer: %s (%s)", peer.Name, publicKey[:16]+"...")
	return nil
}

// GetPeer 获取对等端.
func (m *WireGuardManager) GetPeer(publicKey string) (*WireGuardPeer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, ok := m.peers[publicKey]
	return peer, ok
}

// ListPeers 列出所有对等端.
func (m *WireGuardManager) ListPeers() []*WireGuardPeer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*WireGuardPeer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers
}

// GetStats 获取统计信息.
func (m *WireGuardManager) GetStats() WireGuardStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := WireGuardStats{
		Peers: len(m.peers),
	}

	if m.running {
		stats.Tunnels = 1
	}

	// 从 WireGuard 获取流量统计
	if m.running {
		wgStats, err := m.getWGStats()
		if err == nil {
			stats.BytesSent = wgStats.BytesSent
			stats.BytesRecv = wgStats.BytesRecv
		}

		// 更新对等端状态
		for _, peer := range m.peers {
			if peer.IsActive && time.Since(peer.LastHandshake) < 3*time.Minute {
				stats.ActivePeers++
			}
		}
	}

	return stats
}

// getWGStats 从 WireGuard 获取统计信息.
func (m *WireGuardManager) getWGStats() (*WireGuardStats, error) {
	cmd := exec.Command("wg", "show", m.interfaceName, "transfer")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get wg stats: %s, output: %s", err, string(output))
	}

	stats := &WireGuardStats{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			publicKey := parts[0]
			if peer, exists := m.peers[publicKey]; exists {
				var recv, sent int64
				fmt.Sscanf(parts[1], "%d", &recv)
				fmt.Sscanf(parts[2], "%d", &sent)
				peer.BytesRecv = recv
				peer.BytesSent = sent
				stats.BytesRecv += recv
				stats.BytesSent += sent
			}
		}
	}

	return stats, nil
}

// GetStatus 获取 WireGuard 状态.
func (m *WireGuardManager) GetStatus() (*WireGuardStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return &WireGuardStatus{
			Interface: m.interfaceName,
			PublicKey: m.publicKey,
		}, nil
	}

	// 更新对等端握手状态
	m.updatePeerHandshake()

	status := &WireGuardStatus{
		Interface:  m.interfaceName,
		PublicKey:  m.publicKey,
		ListenPort: m.listenPort,
		Peers:      m.ListPeers(),
		Stats:      m.GetStats(),
	}

	return status, nil
}

// updatePeerHandshake 更新对等端握手状态.
func (m *WireGuardManager) updatePeerHandshake() {
	cmd := exec.Command("wg", "show", m.interfaceName, "latest-handshakes")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			publicKey := parts[0]
			var handshakeTime int64
			fmt.Sscanf(parts[1], "%d", &handshakeTime)

			if peer, exists := m.peers[publicKey]; exists {
				if handshakeTime > 0 {
					peer.LastHandshake = time.Unix(handshakeTime, 0)
					peer.IsActive = time.Since(peer.LastHandshake) < 3*time.Minute
				} else {
					peer.IsActive = false
				}
			}
		}
	}
}

// Close 关闭 WireGuard 管理器.
func (m *WireGuardManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	log.Printf("[WireGuard] Shutting down interface %s...", m.interfaceName)

	// 删除接口
	m.deleteInterface()
	m.running = false

	return nil
}

// GetPublicKey 获取公钥.
func (m *WireGuardManager) GetPublicKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicKey
}

// GetPrivateKey 获取私钥（仅内部使用）.
func (m *WireGuardManager) GetPrivateKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.privateKey
}

// GeneratePeerKeyPair 生成新的对等端密钥对.
func (m *WireGuardManager) GeneratePeerKeyPair() (publicKey, privateKey string, err error) {
	// 生成私钥
	privateKeyOutput, err := m.runWGCommand("genkey")
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}
	privateKey = strings.TrimSpace(privateKeyOutput)

	// 生成公钥
	publicKeyOutput, err := m.runWGCommand("pubkey", strings.NewReader(privateKey))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKey = strings.TrimSpace(publicKeyOutput)

	return publicKey, privateKey, nil
}

// GeneratePsk 生成预共享密钥.
func (m *WireGuardManager) GeneratePsk() (string, error) {
	psk, err := m.runWGCommand("genpsk")
	if err != nil {
		return "", fmt.Errorf("failed to generate psk: %w", err)
	}
	return strings.TrimSpace(psk), nil
}

// ParseAllowedIPs 解析 AllowedIPs 字符串.
func ParseAllowedIPs(allowedIPs string) ([]string, error) {
	if allowedIPs == "" {
		return nil, nil
	}

	parts := strings.Split(allowedIPs, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 验证 CIDR 格式
		if _, _, err := net.ParseCIDR(part); err != nil {
			// 尝试作为单个 IP 解析
			if ip := net.ParseIP(part); ip == nil {
				return nil, fmt.Errorf("invalid IP or CIDR: %s", part)
			}
			// 单个 IP，添加 /32 或 /128 后缀
			if strings.Contains(part, ":") {
				part += "/128"
			} else {
				part += "/32"
			}
		}

		result = append(result, part)
	}

	return result, nil
}
