package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager 网络管理器
type Manager struct {
	mu sync.RWMutex

	// 网络接口状态缓存
	interfaces []*Interface

	// DDNS 配置
	ddnsConfigs map[string]*DDNSConfig

	// 端口转发规则
	portForwards map[string]*PortForward

	// 防火墙规则
	firewallRules map[string]*FirewallRule

	// 配置文件路径
	configPath string
}

// Interface 网络接口
type Interface struct {
	Name    string `json:"name"`
	MAC     string `json:"mac,omitempty"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	DNS     string `json:"dns,omitempty"`
	State   string `json:"state"` // up, down
	Type    string `json:"type"`  // ethernet, wifi, bridge
	Speed   string `json:"speed,omitempty"`
	RxBytes int64  `json:"rxBytes"`
	TxBytes int64  `json:"txBytes"`
	Mtu     int    `json:"mtu"`
}

// InterfaceConfig 接口配置
type InterfaceConfig struct {
	IP      string `json:"ip"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
	DNS     string `json:"dns"`
	DHCP    bool   `json:"dhcp"`
}

// DDNSConfig DDNS 配置
type DDNSConfig struct {
	Provider   string `json:"provider"` // alidns, cloudflare, duckdns, noip
	Domain     string `json:"domain"`
	Token      string `json:"token"`     // API Token
	Secret     string `json:"secret"`    // API Secret (某些服务商需要)
	Interface  string `json:"interface"` // 使用的网络接口，auto 为自动
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status"`     // active, error, pending
	LastIP     string `json:"lastIp"`     // 上次更新的 IP
	LastUpdate string `json:"lastUpdate"` // 上次更新时间
	Interval   int    `json:"interval"`   // 更新间隔（秒）
}

// PortForward 端口转发规则
type PortForward struct {
	Name         string `json:"name"`
	ExternalPort int    `json:"externalPort"`
	Protocol     string `json:"protocol"` // tcp, udp
	InternalIP   string `json:"internalIp"`
	InternalPort int    `json:"internalPort"`
	Enabled      bool   `json:"enabled"`
	Comment      string `json:"comment,omitempty"`
}

// FirewallRule 防火墙规则
type FirewallRule struct {
	Name      string `json:"name"`
	Action    string `json:"action"`    // accept, drop, reject
	Direction string `json:"direction"` // in, out, forward
	Protocol  string `json:"protocol"`  // tcp, udp, icmp, all
	SourceIP  string `json:"sourceIp"`  // 源 IP，空为任意
	DestIP    string `json:"destIp"`    // 目标 IP，空为任意
	DestPort  string `json:"destPort"`  // 目标端口，空为任意
	Enabled   bool   `json:"enabled"`
	Comment   string `json:"comment,omitempty"`
}

// Stats 网络统计
type Stats struct {
	Interfaces   []InterfaceStats `json:"interfaces"`
	TotalRxBytes int64            `json:"totalRxBytes"`
	TotalTxBytes int64            `json:"totalTxBytes"`
}

// InterfaceStats 接口统计
type InterfaceStats struct {
	Name      string `json:"name"`
	RxBytes   int64  `json:"rxBytes"`
	TxBytes   int64  `json:"txBytes"`
	RxPackets int64  `json:"rxPackets"`
	TxPackets int64  `json:"txPackets"`
}

// NewManager 创建网络管理器
func NewManager(configPath string) *Manager {
	return &Manager{
		ddnsConfigs:   make(map[string]*DDNSConfig),
		portForwards:  make(map[string]*PortForward),
		firewallRules: make(map[string]*FirewallRule),
		configPath:    configPath,
	}
}

// Initialize 初始化网络管理器
func (m *Manager) Initialize() error {
	// 加载配置
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在是正常的，忽略
		_ = err // 明确忽略错误，避免 staticcheck 警告
	}

	// 初始化网络接口列表
	ifaces, err := m.ListInterfaces()
	if err == nil {
		m.interfaces = ifaces
	}

	return nil
}

// loadConfig 加载配置文件
func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 配置文件不存在是正常的
		}
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	var config struct {
		DDNSConfigs   map[string]*DDNSConfig   `json:"ddnsConfigs"`
		PortForwards  map[string]*PortForward  `json:"portForwards"`
		FirewallRules map[string]*FirewallRule `json:"firewallRules"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if config.DDNSConfigs != nil {
		m.ddnsConfigs = config.DDNSConfigs
	}
	if config.PortForwards != nil {
		m.portForwards = config.PortForwards
	}
	if config.FirewallRules != nil {
		m.firewallRules = config.FirewallRules
	}

	return nil
}

// saveConfig 保存配置文件
// 注意：调用者必须持有锁（读锁或写锁）
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	config := struct {
		DDNSConfigs   map[string]*DDNSConfig   `json:"ddnsConfigs"`
		PortForwards  map[string]*PortForward  `json:"portForwards"`
		FirewallRules map[string]*FirewallRule `json:"firewallRules"`
	}{
		DDNSConfigs:   m.ddnsConfigs,
		PortForwards:  m.portForwards,
		FirewallRules: m.firewallRules,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败：%w", err)
	}

	return nil
}
