// Package wifimgr 提供 WiFi 网络管理功能
// 网络扫描、连接管理、热点控制、信号监控、安全审计
package wifimgr

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// ========== 类型定义 ==========

// AuthType 认证类型
type AuthType string

const (
	AuthWPA2PSK        AuthType = "WPA2-PSK"
	AuthWPA3PSK        AuthType = "WPA3-PSK"
	AuthWPA2Enterprise AuthType = "WPA2-Enterprise"
	AuthOpen           AuthType = "Open"
)

// BandType 频段类型
type BandType string

const (
	Band24GHz BandType = "2.4GHz"
	Band5GHz  BandType = "5GHz"
	BandAuto  BandType = "Auto"
)

// WiFiNetwork WiFi 网络
type WiFiNetwork struct {
	SSID      string    `json:"ssid"`
	BSSID     string    `json:"bssid"`
	Signal    int       `json:"signal"`    // dBm
	Frequency float64   `json:"frequency"` // MHz
	AuthType  AuthType  `json:"authType"`
	IsSaved   bool      `json:"isSaved"`
	Channel   int       `json:"channel"`
	Band      BandType  `json:"band"`
	LastSeen  time.Time `json:"lastSeen"`
}

// WiFiProfile WiFi 配置
type WiFiProfile struct {
	ID          string    `json:"id"`
	SSID        string    `json:"ssid"`
	Password    string    `json:"password,omitempty"`
	AuthType    AuthType  `json:"authType"`
	AutoConnect bool      `json:"autoConnect"`
	Priority    int       `json:"priority"` // 越高越优先
	Band        BandType  `json:"band"`
	CreatedAt   time.Time `json:"createdAt"`
}

// WiFiStatus WiFi 状态
type WiFiStatus struct {
	Connected   bool      `json:"connected"`
	SSID        string    `json:"ssid,omitempty"`
	IP          string    `json:"ip,omitempty"`
	Signal      int       `json:"signal,omitempty"`
	LinkSpeed   float64   `json:"linkSpeed,omitempty"` // Mbps
	Frequency   float64   `json:"frequency,omitempty"`
	Band        BandType  `json:"band,omitempty"`
	Gateway     string    `json:"gateway,omitempty"`
	DNS         []string  `json:"dns,omitempty"`
	ConnectedAt time.Time `json:"connectedAt,omitempty"`
}

// HotspotConfig 热点配置
type HotspotConfig struct {
	SSID       string   `json:"ssid"`
	Password   string   `json:"password,omitempty"`
	Band       BandType `json:"band"`
	MaxClients int      `json:"maxClients"`
	Enabled    bool     `json:"enabled"`
	Channel    int      `json:"channel"`
	Hidden     bool     `json:"hidden"`
}

// SignalHistory 信号历史
type SignalHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Signal    int       `json:"signal"` // dBm
	Noise     int       `json:"noise"`  // dBm
}

// ========== Manager ==========

// Manager WiFi 管理器
type Manager struct {
	mu                sync.RWMutex
	networks          []WiFiNetwork
	profiles          map[string]*WiFiProfile
	status            WiFiStatus
	hotspot           HotspotConfig
	hotspotEnabled    bool
	connectedClients  int
	signalHistory     []SignalHistory
	autoReconnect     bool
	reconnectStrategy string
	scanInterval      time.Duration
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		profiles:          make(map[string]*WiFiProfile),
		scanInterval:      30 * time.Second,
		reconnectStrategy: "exponential",
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 默认状态：未连接
	m.status = WiFiStatus{
		Connected: false,
	}

	// 默认热点配置
	m.hotspot = HotspotConfig{
		SSID:       "NAS-Hotspot",
		Band:       Band24GHz,
		MaxClients: 10,
		Channel:    6,
	}
}

// ========== 扫描 ==========

// Scan 扫描 WiFi 网络
func (m *Manager) Scan() ([]WiFiNetwork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟扫描结果
	m.networks = m.simulateScan()

	log.Printf("[WiFi] 扫描完成，发现 %d 个网络", len(m.networks))
	return m.networks, nil
}

// simulateScan 模拟扫描
func (m *Manager) simulateScan() []WiFiNetwork {
	now := time.Now()
	return []WiFiNetwork{
		{SSID: "Home-5G", BSSID: "AA:BB:CC:DD:EE:01", Signal: -45, Frequency: 5180, AuthType: AuthWPA3PSK, Band: Band5GHz, Channel: 36, LastSeen: now},
		{SSID: "Home-2.4G", BSSID: "AA:BB:CC:DD:EE:02", Signal: -55, Frequency: 2437, AuthType: AuthWPA2PSK, Band: Band24GHz, Channel: 6, LastSeen: now},
		{SSID: "Office-WiFi", BSSID: "AA:BB:CC:DD:EE:03", Signal: -65, Frequency: 5240, AuthType: AuthWPA2Enterprise, Band: Band5GHz, Channel: 48, LastSeen: now},
		{SSID: "Cafe-Free", BSSID: "AA:BB:CC:DD:EE:04", Signal: -75, Frequency: 2412, AuthType: AuthOpen, Band: Band24GHz, Channel: 1, LastSeen: now},
		{SSID: "Neighbor-WiFi", BSSID: "AA:BB:CC:DD:EE:05", Signal: -80, Frequency: 2462, AuthType: AuthWPA2PSK, Band: Band24GHz, Channel: 11, LastSeen: now},
	}
}

// ========== 连接 ==========

// Connect 连接到网络
func (m *Manager) Connect(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[profileID]
	if !ok {
		return fmt.Errorf("profile %s not found", profileID)
	}

	// 模拟连接
	m.status = WiFiStatus{
		Connected:   true,
		SSID:        profile.SSID,
		IP:          fmt.Sprintf("192.168.1.%d", 100+rand.Intn(50)),
		Signal:      -50 + rand.Intn(20),
		LinkSpeed:   866.7,
		Frequency:   5180,
		Band:        profile.Band,
		Gateway:     "192.168.1.1",
		DNS:         []string{"8.8.8.8", "8.8.4.4"},
		ConnectedAt: time.Now(),
	}

	// 记录信号历史
	m.signalHistory = append(m.signalHistory, SignalHistory{
		Timestamp: time.Now(),
		Signal:    m.status.Signal,
		Noise:     -90,
	})

	log.Printf("[WiFi] 已连接到 %s (%s)", profile.SSID, profile.ID)
	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.status.Connected {
		return fmt.Errorf("not connected")
	}

	ssid := m.status.SSID
	m.status = WiFiStatus{Connected: false}

	log.Printf("[WiFi] 已断开 %s", ssid)
	return nil
}

// GetStatus 获取 WiFi 状态
func (m *Manager) GetStatus() (*WiFiStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := m.status
	return &status, nil
}

// ========== 配置管理 ==========

// SaveProfile 保存 WiFi 配置
func (m *Manager) SaveProfile(profile *WiFiProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if profile.ID == "" {
		return fmt.Errorf("profile ID cannot be empty")
	}
	if profile.SSID == "" {
		return fmt.Errorf("SSID cannot be empty")
	}

	profile.CreatedAt = time.Now()
	m.profiles[profile.ID] = profile

	log.Printf("[WiFi] 保存配置: %s (%s)", profile.SSID, profile.ID)
	return nil
}

// DeleteProfile 删除 WiFi 配置
func (m *Manager) DeleteProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.profiles[id]; !ok {
		return fmt.Errorf("profile %s not found", id)
	}

	// 如果正在连接，先断开
	if m.status.Connected {
		m.status = WiFiStatus{Connected: false}
	}

	delete(m.profiles, id)
	log.Printf("[WiFi] 删除配置: %s", id)
	return nil
}

// ListProfiles 列出所有配置
func (m *Manager) ListProfiles() []WiFiProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]WiFiProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, *p)
	}
	return profiles
}

// ========== 热点 ==========

// EnableHotspot 启用热点
func (m *Manager) EnableHotspot(config *HotspotConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.SSID == "" {
		return fmt.Errorf("SSID cannot be empty")
	}
	if len(config.Password) < 8 && config.Password != "" {
		return fmt.Errorf("password must be at least 8 characters")
	}

	m.hotspot = *config
	m.hotspot.Enabled = true
	m.hotspotEnabled = true
	m.connectedClients = 0

	log.Printf("[WiFi] 热点已启用: %s", config.SSID)
	return nil
}

// DisableHotspot 禁用热点
func (m *Manager) DisableHotspot() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hotspotEnabled {
		return fmt.Errorf("hotspot is not enabled")
	}

	m.hotspot.Enabled = false
	m.hotspotEnabled = false
	m.connectedClients = 0

	log.Printf("[WiFi] 热点已禁用")
	return nil
}

// GetHotspotStatus 获取热点状态
func (m *Manager) GetHotspotStatus() (*HotspotConfig, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.hotspotEnabled {
		return nil, 0, fmt.Errorf("hotspot is not enabled")
	}

	config := m.hotspot
	return &config, m.connectedClients, nil
}

// ========== 信号监控 ==========

// GetSignalHistory 获取信号历史
func (m *Manager) GetSignalHistory(duration time.Duration) []SignalHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	result := make([]SignalHistory, 0)
	for _, h := range m.signalHistory {
		if h.Timestamp.After(cutoff) {
			result = append(result, h)
		}
	}
	return result
}

// ========== 自动重连 ==========

// SetAutoReconnect 设置自动重连
func (m *Manager) SetAutoReconnect(enable bool, strategy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strategy != "" && strategy != "immediate" && strategy != "exponential" && strategy != "linear" {
		return fmt.Errorf("invalid strategy: %s (must be immediate, exponential, or linear)", strategy)
	}

	m.autoReconnect = enable
	if strategy != "" {
		m.reconnectStrategy = strategy
	}

	log.Printf("[WiFi] 自动重连: %v, 策略: %s", enable, m.reconnectStrategy)
	return nil
}

// ========== 安全审计 ==========

// ScanDiagnostics 安全诊断扫描
func (m *Manager) ScanDiagnostics() ([]WiFiNetwork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 扫描并添加安全信息
	networks := m.simulateScan()

	// 标记已保存网络
	for i := range networks {
		for _, p := range m.profiles {
			if networks[i].SSID == p.SSID {
				networks[i].IsSaved = true
				break
			}
		}
	}

	m.networks = networks
	log.Printf("[WiFi] 安全诊断扫描完成，%d 个网络", len(networks))
	return networks, nil
}
