package ipv6net

import (
	"fmt"
	"net"
	"sync"
)

// IPv6Config IPv6配置
type IPv6Config struct {
	Enabled        bool
	AutoConfig     bool
	DHCPv6         bool
	PrivacyExt     bool
	LinkLocal      bool
	GlobalAddrs    []string
	Gateway        string
	DNS            []string
}

// IPv6Manager IPv6管理器
type IPv6Manager struct {
	config    IPv6Config
	interfaces map[string]*InterfaceInfo
	mu         sync.RWMutex
}

// InterfaceInfo 网络接口信息
type InterfaceInfo struct {
	Name         string
	MTU          int
	LinkLocal    string
	GlobalAddrs  []string
	Status       string
	RxBytes      int64
	TxBytes      int64
}

// NewIPv6Manager 创建IPv6管理器
func NewIPv6Manager(config IPv6Config) *IPv6Manager {
	return &IPv6Manager{
		config:     config,
		interfaces: make(map[string]*InterfaceInfo),
	}
}

// Init 初始化IPv6网络
func (m *IPv6Manager) Init() error {
	if !m.config.Enabled {
		return nil
	}

	// 启用IPv6转发
	if err := m.enableForwarding(); err != nil {
		return fmt.Errorf("failed to enable IPv6 forwarding: %w", err)
	}

	// 配置隐私扩展
	if m.config.PrivacyExt {
		if err := m.enablePrivacyExtensions(); err != nil {
			return fmt.Errorf("failed to enable privacy extensions: %w", err)
		}
	}

	// 检测网络接口
	if err := m.detectInterfaces(); err != nil {
		return fmt.Errorf("failed to detect interfaces: %w", err)
	}

	return nil
}

// enableForwarding 启用IPv6转发
func (m *IPv6Manager) enableForwarding() error {
	// 写入/proc/sys/net/ipv6/conf/all/forwarding
	return nil // 简化实现
}

// enablePrivacyExtensions 启用隐私扩展
func (m *IPv6Manager) enablePrivacyExtensions() error {
	// 写入/proc/sys/net/ipv6/conf/all/use_tempaddr
	return nil // 简化实现
}

// detectInterfaces 检测网络接口
func (m *IPv6Manager) detectInterfaces() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		info := &InterfaceInfo{
			Name:   iface.Name,
			MTU:    iface.MTU,
			Status: "up",
		}

		// 获取IPv6地址
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if ipNet.IP.To4() != nil {
				continue // 跳过IPv4
			}

			if ipNet.IP.IsLinkLocalUnicast() {
				info.LinkLocal = ipNet.IP.String()
			} else {
				info.GlobalAddrs = append(info.GlobalAddrs, ipNet.IP.String())
			}
		}

		m.interfaces[iface.Name] = info
	}

	return nil
}

// GetInterfaceInfo 获取接口信息
func (m *IPv6Manager) GetInterfaceInfo(name string) (*InterfaceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.interfaces[name]
	if !exists {
		return nil, fmt.Errorf("interface not found: %s", name)
	}

	return info, nil
}

// GetAllInterfaces 获取所有接口
func (m *IPv6Manager) GetAllInterfaces() map[string]*InterfaceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*InterfaceInfo)
	for k, v := range m.interfaces {
		result[k] = v
	}

	return result
}

// SetAddress 设置IPv6地址
func (m *IPv6Manager) SetAddress(iface, addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.interfaces[iface]
	if !exists {
		return fmt.Errorf("interface not found: %s", iface)
	}

	// 验证地址格式
	ip := net.ParseIP(addr)
	if ip == nil || ip.To4() != nil {
		return fmt.Errorf("invalid IPv6 address: %s", addr)
	}

	info.GlobalAddrs = append(info.GlobalAddrs, addr)
	return nil
}

// RemoveAddress 移除IPv6地址
func (m *IPv6Manager) RemoveAddress(iface, addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.interfaces[iface]
	if !exists {
		return fmt.Errorf("interface not found: %s", iface)
	}

	for i, a := range info.GlobalAddrs {
		if a == addr {
			info.GlobalAddrs = append(info.GlobalAddrs[:i], info.GlobalAddrs[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("address not found: %s", addr)
}

// EnableDHCPv6 启用DHCPv6
func (m *IPv6Manager) EnableDHCPv6(iface string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.interfaces[iface]; !exists {
		return fmt.Errorf("interface not found: %s", iface)
	}

	// 启动DHCPv6客户端
	return nil // 简化实现
}

// GetDNSConfig 获取DNS配置
func (m *IPv6Manager) GetDNSConfig() []string {
	return m.config.DNS
}

// SetDNSConfig 设置DNS配置
func (m *IPv6Manager) SetDNSConfig(dns []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.DNS = dns
}

// IsIPv6Enabled 检查IPv6是否启用
func (m *IPv6Manager) IsIPv6Enabled() bool {
	return m.config.Enabled
}

// GetStatus 获取IPv6状态
func (m *IPv6Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        m.config.Enabled,
		"auto_config":    m.config.AutoConfig,
		"dhcpv6":         m.config.DHCPv6,
		"privacy_ext":    m.config.PrivacyExt,
		"interface_count": len(m.interfaces),
	}
}
