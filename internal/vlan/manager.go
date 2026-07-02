// Package vlan 提供 VLAN 网络管理功能
// 支持 IEEE 802.1Q VLAN 创建、配置和管理
package vlan

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// VLAN 表示一个 VLAN 接口.
type VLAN struct {
	ID          int      `json:"id"`           // VLAN ID (1-4094)
	Name        string   `json:"name"`         // 接口名称 (如 eth0.100)
	ParentIface string   `json:"parent_iface"` // 父接口 (如 eth0)
	IPAddr      string   `json:"ip_addr"`      // IP 地址
	Netmask     string   `json:"netmask"`      // 子网掩码
	Gateway     string   `json:"gateway"`      // 网关
	MTU         int      `json:"mtu"`          // MTU (默认 1500)
	Status      string   `json:"status"`       // up/down
	Tags        []string `json:"tags"`         // 自定义标签
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
}

// VLANStats 表示 VLAN 接口统计信息.
type VLANStats struct {
	VLANID     int   `json:"vlan_id"`
	RxBytes    int64 `json:"rx_bytes"`
	TxBytes    int64 `json:"tx_bytes"`
	RxPackets  int64 `json:"rx_packets"`
	TxPackets  int64 `json:"tx_packets"`
	RxErrors   int64 `json:"rx_errors"`
	TxErrors   int64 `json:"tx_errors"`
	RxDropped  int64 `json:"rx_dropped"`
	TxDropped  int64 `json:"tx_dropped"`
	Collisions int64 `json:"collisions"`
}

// Manager 管理所有 VLAN 接口.
type Manager struct {
	mu     sync.RWMutex
	vlans  map[int]*VLAN  // VLAN ID -> VLAN
	ifaces map[string]int // 接口名 -> VLAN ID
}

// NewManager 创建 VLAN 管理器.
func NewManager() *Manager {
	return &Manager{
		vlans:  make(map[int]*VLAN),
		ifaces: make(map[string]int),
	}
}

// Create 创建 VLAN 接口.
func (m *Manager) Create(parentIface string, vlanID int, ipAddr, netmask, gateway string, mtu int) (*VLAN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证参数
	if vlanID < 1 || vlanID > 4094 {
		return nil, fmt.Errorf("VLAN ID 必须在 1-4094 之间，当前: %d", vlanID)
	}

	if _, exists := m.vlans[vlanID]; exists {
		return nil, fmt.Errorf("VLAN %d 已存在", vlanID)
	}

	// 验证父接口
	if parentIface == "" {
		return nil, fmt.Errorf("父接口不能为空")
	}

	// 验证 IP 地址
	if ipAddr != "" {
		if net.ParseIP(ipAddr) == nil {
			return nil, fmt.Errorf("无效的 IP 地址: %s", ipAddr)
		}
	}

	// 设置默认 MTU
	if mtu <= 0 {
		mtu = 1500
	}

	// 设置默认子网掩码
	if netmask == "" {
		netmask = "255.255.255.0"
	}

	name := fmt.Sprintf("%s.%d", parentIface, vlanID)

	vlan := &VLAN{
		ID:          vlanID,
		Name:        name,
		ParentIface: parentIface,
		IPAddr:      ipAddr,
		Netmask:     netmask,
		Gateway:     gateway,
		MTU:         mtu,
		Status:      "down",
		Tags:        []string{},
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	m.vlans[vlanID] = vlan
	m.ifaces[name] = vlanID

	return vlan, nil
}

// Delete 删除 VLAN 接口.
func (m *Manager) Delete(vlanID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vlan, exists := m.vlans[vlanID]
	if !exists {
		return fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	delete(m.ifaces, vlan.Name)
	delete(m.vlans, vlanID)

	return nil
}

// Get 获取 VLAN 信息.
func (m *Manager) Get(vlanID int) (*VLAN, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vlan, exists := m.vlans[vlanID]
	if !exists {
		return nil, fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	return vlan, nil
}

// List 列出所有 VLAN.
func (m *Manager) List() []*VLAN {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*VLAN, 0, len(m.vlans))
	for _, vlan := range m.vlans {
		result = append(result, vlan)
	}
	return result
}

// Update 更新 VLAN 配置.
func (m *Manager) Update(vlanID int, ipAddr, netmask, gateway string, mtu int, tags []string) (*VLAN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vlan, exists := m.vlans[vlanID]
	if !exists {
		return nil, fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	if ipAddr != "" {
		if net.ParseIP(ipAddr) == nil {
			return nil, fmt.Errorf("无效的 IP 地址: %s", ipAddr)
		}
		vlan.IPAddr = ipAddr
	}

	if netmask != "" {
		vlan.Netmask = netmask
	}

	if gateway != "" {
		vlan.Gateway = gateway
	}

	if mtu > 0 {
		vlan.MTU = mtu
	}

	if tags != nil {
		vlan.Tags = tags
	}

	vlan.UpdatedAt = time.Now().Unix()

	return vlan, nil
}

// Enable 启用 VLAN 接口.
func (m *Manager) Enable(vlanID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vlan, exists := m.vlans[vlanID]
	if !exists {
		return fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	vlan.Status = "up"
	vlan.UpdatedAt = time.Now().Unix()

	return nil
}

// Disable 禁用 VLAN 接口.
func (m *Manager) Disable(vlanID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vlan, exists := m.vlans[vlanID]
	if !exists {
		return fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	vlan.Status = "down"
	vlan.UpdatedAt = time.Now().Unix()

	return nil
}

// GetStats 获取 VLAN 统计信息.
func (m *Manager) GetStats(vlanID int) (*VLANStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.vlans[vlanID]; !exists {
		return nil, fmt.Errorf("VLAN %d 不存在", vlanID)
	}

	// 实际实现中应从系统读取统计信息
	return &VLANStats{
		VLANID: vlanID,
	}, nil
}

// GetByParent 获取指定父接口的所有 VLAN.
func (m *Manager) GetByParent(parentIface string) []*VLAN {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*VLAN
	for _, vlan := range m.vlans {
		if vlan.ParentIface == parentIface {
			result = append(result, vlan)
		}
	}
	return result
}
