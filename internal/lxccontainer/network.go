package lxccontainer

import (
	"fmt"
	"net"
)

// NetworkManager 容器网络管理.
type NetworkManager struct {
	bridges map[string]*Bridge
}

// Bridge 虚拟网桥.
type Bridge struct {
	Name    string   `json:"name"`
	Subnet  string   `json:"subnet"`
	Gateway string   `json:"gateway"`
	IPPool  []string `json:"ipPool"`
	Used    []string `json:"used"`
}

// NewNetworkManager 创建网络管理器.
func NewNetworkManager() *NetworkManager {
	return &NetworkManager{
		bridges: make(map[string]*Bridge),
	}
}

// CreateBridge 创建虚拟网桥.
func (nm *NetworkManager) CreateBridge(name, subnet, gateway string) (*Bridge, error) {
	if _, exists := nm.bridges[name]; exists {
		return nil, fmt.Errorf("网桥 %s 已存在", name)
	}

	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("无效子网 %s: %w", subnet, err)
	}

	ipPool := generateIPPool(ipNet)
	if len(ipPool) < 2 {
		return nil, fmt.Errorf("子网 %s 地址空间不足", subnet)
	}

	// 排除网关地址
	gwStr := gateway
	filtered := make([]string, 0, len(ipPool)-1)
	for _, ip := range ipPool {
		if ip != gwStr {
			filtered = append(filtered, ip)
		}
	}

	bridge := &Bridge{
		Name:    name,
		Subnet:  subnet,
		Gateway: gateway,
		IPPool:  filtered,
		Used:    make([]string, 0),
	}

	nm.bridges[name] = bridge
	return bridge, nil
}

// AllocateIP 从网桥分配 IP.
func (nm *NetworkManager) AllocateIP(bridgeName string) (string, error) {
	bridge, ok := nm.bridges[bridgeName]
	if !ok {
		return "", fmt.Errorf("网桥 %s 不存在", bridgeName)
	}

	if len(bridge.IPPool) == 0 {
		return "", fmt.Errorf("网桥 %s 无可用 IP", bridgeName)
	}

	ip := bridge.IPPool[0]
	bridge.IPPool = bridge.IPPool[1:]
	bridge.Used = append(bridge.Used, ip)
	return ip, nil
}

// ReleaseIP 释放 IP 回网桥.
func (nm *NetworkManager) ReleaseIP(bridgeName, ip string) error {
	bridge, ok := nm.bridges[bridgeName]
	if !ok {
		return fmt.Errorf("网桥 %s 不存在", bridgeName)
	}

	for i, used := range bridge.Used {
		if used == ip {
			bridge.Used = append(bridge.Used[:i], bridge.Used[i+1:]...)
			bridge.IPPool = append(bridge.IPPool, ip)
			return nil
		}
	}
	return fmt.Errorf("IP %s 不属于网桥 %s", ip, bridgeName)
}

// GetBridge 获取网桥.
func (nm *NetworkManager) GetBridge(name string) (*Bridge, error) {
	bridge, ok := nm.bridges[name]
	if !ok {
		return nil, fmt.Errorf("网桥 %s 不存在", name)
	}
	return bridge, nil
}

// ListBridges 列出所有网桥.
func (nm *NetworkManager) ListBridges() []*Bridge {
	result := make([]*Bridge, 0, len(nm.bridges))
	for _, b := range nm.bridges {
		result = append(result, b)
	}
	return result
}

// DeleteBridge 删除网桥.
func (nm *NetworkManager) DeleteBridge(name string) error {
	bridge, ok := nm.bridges[name]
	if !ok {
		return fmt.Errorf("网桥 %s 不存在", name)
	}
	if len(bridge.Used) > 0 {
		return fmt.Errorf("网桥 %s 仍有 %d 个 IP 在使用", name, len(bridge.Used))
	}
	delete(nm.bridges, name)
	return nil
}

// ValidateNetworkConfig 验证网络配置.
func ValidateNetworkConfig(cfg NetworkConfig) error {
	switch cfg.Mode {
	case NetworkModeBridge, NetworkModeNAT:
		if cfg.Bridge == "" {
			return fmt.Errorf("%s 模式需要指定网桥", cfg.Mode)
		}
	case NetworkModeStatic:
		if cfg.IPAddress == "" {
			return fmt.Errorf("静态模式需要指定 IP 地址")
		}
		if net.ParseIP(cfg.IPAddress) == nil {
			return fmt.Errorf("无效 IP 地址: %s", cfg.IPAddress)
		}
		if cfg.Gateway != "" && net.ParseIP(cfg.Gateway) == nil {
			return fmt.Errorf("无效网关地址: %s", cfg.Gateway)
		}
	case NetworkModeNone:
		// 无需验证
	default:
		return fmt.Errorf("不支持的网络模式: %s", cfg.Mode)
	}

	for _, dns := range cfg.DNS {
		if net.ParseIP(dns) == nil {
			return fmt.Errorf("无效 DNS 地址: %s", dns)
		}
	}

	return nil
}

// generateIPPool 从子网生成可用 IP 列表.
func generateIPPool(ipNet *net.IPNet) []string {
	var pool []string
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)

	for {
		inc(ip)
		if !ipNet.Contains(ip) {
			break
		}
		pool = append(pool, ip.String())
	}
	return pool
}

// inc IP 地址自增.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
