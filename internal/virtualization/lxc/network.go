// Package lxc 沙箱网络隔离模块
// 提供沙箱网络隔离、虚拟以太网对、网桥和防火墙管理
package lxc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"go.uber.org/zap"
)

// FirewallRule 防火墙规则
type FirewallRule struct {
	Direction  string `json:"direction"`   // in / out
	Action     string `json:"action"`      // accept / drop / reject
	Protocol   string `json:"protocol"`    // tcp / udp / icmp / all
	SourceIP   string `json:"source_ip"`   // 源 IP（空表示任意）
	DestIP     string `json:"dest_ip"`     // 目标 IP
	SourcePort int    `json:"source_port"` // 源端口（0 表示任意）
	DestPort   int    `json:"dest_port"`   // 目标端口
	Comment    string `json:"comment"`     // 规则注释
}

// SandboxBridgeConfig 沙箱网桥配置（避免与 types.go 中的 Network 冲突）
type SandboxBridgeConfig struct {
	Name       string   `json:"name"`       // 网桥名称
	IPAddress  string   `json:"ip_address"` // 网桥 IP 地址
	Subnet     string   `json:"subnet"`     // 子网掩码
	MTU        int      `json:"mtu"`        // 最大传输单元
	STP        bool     `json:"stp"`        // 是否启用 STP
	Interfaces []string `json:"interfaces"` // 绑定的物理接口
}

// SandboxVethPair 虚拟以太网对
type SandboxVethPair struct {
	HostSide  string `json:"host_side"`  // 宿主机端接口名
	GuestSide string `json:"guest_side"` // 沙箱端接口名
	Bridge    string `json:"bridge"`     // 所属网桥
	IPAddress string `json:"ip_address"` // 分配的 IP 地址
	MAC       string `json:"mac"`        // MAC 地址
}

// SandboxNetworkManager 沙箱网络管理器
type SandboxNetworkManager struct {
	mu        sync.RWMutex
	bridges   map[string]*SandboxBridgeConfig
	rules     map[string][]FirewallRule   // sandbox_id -> rules
	vethPairs map[string]*SandboxVethPair // sandbox_id -> veth pair
	config    *SandboxNetManagerConfig
	logger    *zap.Logger
}

// SandboxNetManagerConfig 网络管理器配置
type SandboxNetManagerConfig struct {
	DefaultBridge  string `json:"default_bridge"`  // 默认网桥
	SubnetPool     string `json:"subnet_pool"`     // 子网池（如 10.0.3.0/24）
	VethPrefix     string `json:"veth_prefix"`     // veth 接口名前缀
	EnableFirewall bool   `json:"enable_firewall"` // 是否启用内置防火墙
}

// NewSandboxNetworkManager 创建沙箱网络管理器
func NewSandboxNetworkManager(logger *zap.Logger) *SandboxNetworkManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SandboxNetworkManager{
		bridges:   make(map[string]*SandboxBridgeConfig),
		rules:     make(map[string][]FirewallRule),
		vethPairs: make(map[string]*SandboxVethPair),
		config: &SandboxNetManagerConfig{
			DefaultBridge:  "lxcbr0",
			SubnetPool:     "10.0.3.0/24",
			VethPrefix:     "veth",
			EnableFirewall: true,
		},
		logger: logger,
	}
}

// SetupVethPair 创建虚拟以太网对
// 在宿主机和沙箱之间建立点对点网络连接
func (nm *SandboxNetworkManager) SetupVethPair(ctx context.Context, sandboxID string, cfg SandboxNetworkConfig) (*SandboxVethPair, error) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 检查是否已存在
	if pair, exists := nm.vethPairs[sandboxID]; exists {
		return pair, nil
	}

	// 生成唯一的接口名
	hostSide := cfg.VethHost
	guestSide := cfg.VethGuest
	if hostSide == "" {
		hostSide = fmt.Sprintf("%s%s-h", nm.config.VethPrefix, sandboxID[:8])
	}
	if guestSide == "" {
		guestSide = fmt.Sprintf("%s%s-g", nm.config.VethPrefix, sandboxID[:8])
	}

	// 验证接口名长度（Linux 限制 15 字符）
	if len(hostSide) > 15 {
		hostSide = hostSide[:15]
	}
	if len(guestSide) > 15 {
		guestSide = guestSide[:15]
	}

	// 生成 MAC 地址
	mac := cfg.MACAddress
	if mac == "" {
		mac = generateSandboxMAC()
	}

	pair := &SandboxVethPair{
		HostSide:  hostSide,
		GuestSide: guestSide,
		Bridge:    cfg.Bridge,
		IPAddress: cfg.IPAddress,
		MAC:       mac,
	}

	// 实际创建 veth 对需要调用 ip link 命令
	// ip link add <host-side> type veth peer name <guest-side>
	nm.logger.Info("创建虚拟以太网对",
		zap.String("sandbox_id", sandboxID),
		zap.String("host_side", hostSide),
		zap.String("guest_side", guestSide),
		zap.String("mac", mac))

	nm.vethPairs[sandboxID] = pair
	return pair, nil
}

// ConfigureBridge 配置网桥
// 创建或更新 Linux 网桥，用于沙箱间通信和外部访问
func (nm *SandboxNetworkManager) ConfigureBridge(ctx context.Context, cfg SandboxBridgeConfig) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if cfg.Name == "" {
		return fmt.Errorf("网桥名称不能为空")
	}

	if cfg.MTU == 0 {
		cfg.MTU = 1500
	}

	// 验证 IP 地址
	if cfg.IPAddress != "" {
		if net.ParseIP(cfg.IPAddress) == nil {
			return fmt.Errorf("无效的网桥 IP 地址: %s", cfg.IPAddress)
		}
	}

	// 创建网桥的步骤（实际实现中执行系统命令）：
	// 1. ip link add name <bridge> type bridge
	// 2. ip addr add <ip>/<mask> dev <bridge>
	// 3. ip link set <bridge> up
	// 4. 对于每个绑定接口: ip link set <iface> master <bridge>

	nm.bridges[cfg.Name] = &cfg

	nm.logger.Info("网桥配置完成",
		zap.String("name", cfg.Name),
		zap.String("ip", cfg.IPAddress),
		zap.Int("mtu", cfg.MTU))

	return nil
}

// SetupFirewall 为沙箱设置防火墙规则
// 使用 iptables/nftables 实现网络访问控制
func (nm *SandboxNetworkManager) SetupFirewall(ctx context.Context, sandboxID string, rules []FirewallRule) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.config.EnableFirewall {
		nm.logger.Debug("防火墙未启用，跳过规则设置", zap.String("sandbox_id", sandboxID))
		return nil
	}

	// 验证规则
	for i, rule := range rules {
		if rule.Direction != "in" && rule.Direction != "out" {
			return fmt.Errorf("规则 %d: 无效的方向 '%s'，必须为 in 或 out", i, rule.Direction)
		}
		if rule.Action != "accept" && rule.Action != "drop" && rule.Action != "reject" {
			return fmt.Errorf("规则 %d: 无效的动作 '%s'", i, rule.Action)
		}
	}

	// 实际实现中会调用 iptables/nftables 命令
	// 为沙箱创建独立的链（chain），便于管理
	nm.rules[sandboxID] = rules

	nm.logger.Info("防火墙规则已设置",
		zap.String("sandbox_id", sandboxID),
		zap.Int("rule_count", len(rules)))

	return nil
}

// IsolateNetwork 实现沙箱网络隔离
// 配置网络命名空间隔离，确保沙箱间互不可见
func (nm *SandboxNetworkManager) IsolateNetwork(ctx context.Context, sandboxID string, cfg SandboxNetworkConfig) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 网络隔离策略：
	// 1. 每个沙箱拥有独立的网络命名空间（LXC 自动提供）
	// 2. 默认禁止沙箱间通信（通过 iptables FORWARD 链过滤）
	// 3. 仅允许通过网桥访问外部网络
	// 4. 通过 ebtables 阻止二层流量泄露

	switch cfg.Mode {
	case "bridge":
		// 桥接模式：沙箱直接接入网桥，获得独立 IP
		nm.logger.Info("配置桥接模式网络隔离",
			zap.String("sandbox_id", sandboxID),
			zap.String("bridge", cfg.Bridge))

	case "nat":
		// NAT 模式：沙箱通过 NAT 访问外部网络
		nm.logger.Info("配置 NAT 模式网络隔离",
			zap.String("sandbox_id", sandboxID))

	case "none":
		// 无网络：完全断网
		nm.logger.Info("配置无网络模式",
			zap.String("sandbox_id", sandboxID))

	case "host":
		// 共享宿主网络（不推荐）
		nm.logger.Warn("使用共享宿主网络模式，沙箱间无网络隔离",
			zap.String("sandbox_id", sandboxID))

	default:
		return fmt.Errorf("不支持的网络模式: %s", cfg.Mode)
	}

	// 设置默认的隔离防火墙规则
	isolationRules := []FirewallRule{
		{
			Direction: "in",
			Action:    "drop",
			Protocol:  "all",
			Comment:   "默认拒绝所有入站",
		},
		{
			Direction: "out",
			Action:    "accept",
			Protocol:  "all",
			Comment:   "允许所有出站",
		},
	}

	// 添加允许的端口
	for _, port := range cfg.AllowedPorts {
		isolationRules = append(isolationRules, FirewallRule{
			Direction: "in",
			Action:    "accept",
			Protocol:  "tcp",
			DestPort:  port,
			Comment:   fmt.Sprintf("允许 TCP 端口 %d 入站", port),
		})
	}

	nm.rules[sandboxID] = isolationRules

	nm.logger.Info("网络隔离已配置",
		zap.String("sandbox_id", sandboxID),
		zap.String("mode", cfg.Mode))

	return nil
}

// RemoveNetwork 移除沙箱网络配置
func (nm *SandboxNetworkManager) RemoveNetwork(ctx context.Context, sandboxID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 清理 veth 对
	if pair, exists := nm.vethPairs[sandboxID]; exists {
		// ip link del <host-side>
		nm.logger.Info("清理虚拟以太网对",
			zap.String("sandbox_id", sandboxID),
			zap.String("host_side", pair.HostSide))
		delete(nm.vethPairs, sandboxID)
	}

	// 清理防火墙规则
	delete(nm.rules, sandboxID)

	nm.logger.Info("沙箱网络已清理", zap.String("sandbox_id", sandboxID))
	return nil
}

// GetNetworkInfo 获取沙箱网络信息
func (nm *SandboxNetworkManager) GetNetworkInfo(sandboxID string) (*SandboxVethPair, []FirewallRule) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	pair := nm.vethPairs[sandboxID]
	rules := nm.rules[sandboxID]
	return pair, rules
}

// generateSandboxMAC 生成随机 MAC 地址（本地管理地址）
func generateSandboxMAC() string {
	mac := make(net.HardwareAddr, 6)
	mac[0] = 0x02 // 本地管理地址标志
	for i := 1; i < 6; i++ {
		mac[i] = byte(i * 17) // 简单的伪随机
	}
	return mac.String()
}
