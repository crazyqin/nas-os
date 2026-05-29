// Package netbond 网络绑定 - 多网卡聚合/负载均衡
// 对标TrueNAS网络聚合，支持LACP/Active-Backup等模式
package netbond

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// BondMode 绑定模式
type BondMode string

const (
	BondModeRoundRobin    BondMode = "round_robin"
	BondModeActiveBackup  BondMode = "active_backup"
	BondModeXOR           BondMode = "xor"
	BondModeBroadcast     BondMode = "broadcast"
	BondMode8023AD        BondMode = "802.3ad" // LACP
	BondModeBalanceTLB    BondMode = "balance_tlb"
	BondModeBalanceALB    BondMode = "balance_alb"
)

// BondState 绑定状态
type BondState string

const (
	BondStateUp       BondState = "up"
	BondStateDown     BondState = "down"
	BondStateDegraded BondState = "degraded"
	BondStateError    BondState = "error"
)

// SlaveState 从接口状态
type SlaveState string

const (
	SlaveStateActive   SlaveState = "active"
	SlaveStateBackup   SlaveState = "backup"
	SlaveStateDown     SlaveState = "down"
	SlaveStateError    SlaveState = "error"
)

// IPConfig IP 配置
type IPConfig struct {
	IPv4      string `json:"ipv4"`
	Netmask   string `json:"netmask"`
	Gateway   string `json:"gateway"`
	DNS       []string `json:"dns"`
	IPv6      string `json:"ipv6"`
	IPv6Prefix int    `json:"ipv6_prefix"`
	DHCP      bool   `json:"dhcp"`
}

// SlaveInterface 从接口
type SlaveInterface struct {
	Name      string      `json:"name"`
	MAC       string      `json:"mac"`
	State     SlaveState  `json:"state"`
	Speed     int         `json:"speed"` // Mbps
	LinkUp    bool        `json:"link_up"`
	RxBytes   int64       `json:"rx_bytes"`
	TxBytes   int64       `json:"tx_bytes"`
	RxPackets int64       `json:"rx_packets"`
	TxPackets int64       `json:"tx_packets"`
	Errors    int64       `json:"errors"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// BondInterface 绑定接口
type BondInterface struct {
	Name       string           `json:"name"`
	Mode       BondMode         `json:"mode"`
	State      BondState        `json:"state"`
	Slaves     []SlaveInterface `json:"slaves"`
	IP         IPConfig         `json:"ip"`
	MTU        int              `json:"mtu"`
	MAC        string           `json:"mac"`
	ActiveSlave string          `json:"active_slave"`
	TransmitHash string         `json:"transmit_hash"` // for XOR/LACP
	MIIMonitor int              `json:"mii_monitor"`   // ms
	UpDelay    int              `json:"up_delay"`      // ms
	DownDelay  int              `json:"down_delay"`    // ms
	TotalRxBytes int64          `json:"total_rx_bytes"`
	TotalTxBytes int64          `json:"total_tx_bytes"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// VLANConfig VLAN 配置
type VLANConfig struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Parent    string `json:"parent"`
	IP        IPConfig `json:"ip"`
	Enabled   bool   `json:"enabled"`
}

// NetworkStats 网络统计
type NetworkStats struct {
	Interface  string    `json:"interface"`
	RxBytes    int64     `json:"rx_bytes"`
	TxBytes    int64     `json:"tx_bytes"`
	RxPackets  int64     `json:"rx_packets"`
	TxPackets  int64     `json:"tx_packets"`
	RxErrors   int64     `json:"rx_errors"`
	TxErrors   int64     `json:"tx_errors"`
	RxDropped  int64     `json:"rx_dropped"`
	TxDropped  int64     `json:"tx_dropped"`
	Speed      int       `json:"speed"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Manager 网络管理器
type Manager struct {
	mu       sync.RWMutex
	bonds    map[string]*BondInterface
	vlans    map[string]*VLANConfig
	stats    map[string]*NetworkStats
	onChange func(string, string) // interface, event
}

// NewManager 创建网络管理器
func NewManager() *Manager {
	return &Manager{
		bonds: make(map[string]*BondInterface),
		vlans: make(map[string]*VLANConfig),
		stats: make(map[string]*NetworkStats),
	}
}

// CreateBond 创建绑定接口
func (m *Manager) CreateBond(name string, mode BondMode, slaves []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.bonds[name]; exists {
		return fmt.Errorf("bond %q already exists", name)
	}

	if len(slaves) < 2 {
		return errors.New("at least 2 slave interfaces required")
	}

	bond := &BondInterface{
		Name:       name,
		Mode:       mode,
		State:      BondStateDown,
		MTU:        1500,
		MIIMonitor: 100,
		UpDelay:    0,
		DownDelay:  0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	for _, slave := range slaves {
		bond.Slaves = append(bond.Slaves, SlaveInterface{
			Name:  slave,
			State: SlaveStateDown,
		})
	}

	m.bonds[name] = bond
	return nil
}

// DeleteBond 删除绑定接口
func (m *Manager) DeleteBond(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %q not found", name)
	}
	if bond.State == BondStateUp {
		return errors.New("cannot delete active bond, bring it down first")
	}

	delete(m.bonds, name)
	return nil
}

// GetBond 获取绑定接口
func (m *Manager) GetBond(name string) (*BondInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bond, exists := m.bonds[name]
	if !exists {
		return nil, fmt.Errorf("bond %q not found", name)
	}
	return bond, nil
}

// ListBonds 列出所有绑定接口
func (m *Manager) ListBonds() []BondInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bonds := make([]BondInterface, 0, len(m.bonds))
	for _, b := range m.bonds {
		bonds = append(bonds, *b)
	}
	return bonds
}

// UpBond 启动绑定接口
func (m *Manager) UpBond(name string, ip IPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %q not found", name)
	}

	bond.IP = ip
	bond.State = BondStateUp
	bond.UpdatedAt = time.Now()

	for i := range bond.Slaves {
		bond.Slaves[i].State = SlaveStateActive
		bond.Slaves[i].LinkUp = true
		bond.Slaves[i].UpdatedAt = time.Now()
	}
	if len(bond.Slaves) > 0 {
		bond.ActiveSlave = bond.Slaves[0].Name
	}

	if m.onChange != nil {
		go m.onChange(name, "up")
	}
	return nil
}

// DownBond 关闭绑定接口
func (m *Manager) DownBond(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %q not found", name)
	}

	bond.State = BondStateDown
	bond.ActiveSlave = ""
	bond.UpdatedAt = time.Now()

	for i := range bond.Slaves {
		bond.Slaves[i].State = SlaveStateDown
		bond.Slaves[i].LinkUp = false
		bond.Slaves[i].UpdatedAt = time.Now()
	}

	if m.onChange != nil {
		go m.onChange(name, "down")
	}
	return nil
}

// AddSlave 添加从接口
func (m *Manager) AddSlave(bondName, slaveName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[bondName]
	if !exists {
		return fmt.Errorf("bond %q not found", bondName)
	}

	for _, s := range bond.Slaves {
		if s.Name == slaveName {
			return errors.New("slave already in bond")
		}
	}

	state := SlaveStateDown
	if bond.State == BondStateUp {
		state = SlaveStateActive
	}

	bond.Slaves = append(bond.Slaves, SlaveInterface{
		Name:      slaveName,
		State:     state,
		UpdatedAt: time.Now(),
	})
	bond.UpdatedAt = time.Now()
	return nil
}

// RemoveSlave 移除从接口
func (m *Manager) RemoveSlave(bondName, slaveName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[bondName]
	if !exists {
		return fmt.Errorf("bond %q not found", bondName)
	}

	if len(bond.Slaves) <= 2 {
		return errors.New("bond must have at least 2 slaves")
	}

	for i, s := range bond.Slaves {
		if s.Name == slaveName {
			bond.Slaves = append(bond.Slaves[:i], bond.Slaves[i+1:]...)
			bond.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("slave %q not found in bond", slaveName)
}

// SetMTU 设置 MTU
func (m *Manager) SetMTU(name string, mtu int) error {
	if mtu < 68 || mtu > 9216 {
		return errors.New("MTU must be between 68 and 9216")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %q not found", name)
	}

	bond.MTU = mtu
	bond.UpdatedAt = time.Now()
	return nil
}

// CreateVLAN 创建 VLAN
func (m *Manager) CreateVLAN(config VLANConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := fmt.Sprintf("%s.%d", config.Parent, config.ID)
	if _, exists := m.vlans[name]; exists {
		return fmt.Errorf("VLAN %q already exists", name)
	}

	config.Name = name
	m.vlans[name] = &config
	return nil
}

// DeleteVLAN 删除 VLAN
func (m *Manager) DeleteVLAN(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.vlans[name]; !exists {
		return fmt.Errorf("VLAN %q not found", name)
	}

	delete(m.vlans, name)
	return nil
}

// ListVLANs 列出 VLAN
func (m *Manager) ListVLANs() []VLANConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vlans := make([]VLANConfig, 0, len(m.vlans))
	for _, v := range m.vlans {
		vlans = append(vlans, *v)
	}
	return vlans
}

// UpdateStats 更新接口统计
func (m *Manager) UpdateStats(iface string, stats NetworkStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stats.UpdatedAt = time.Now()
	m.stats[iface] = &stats
}

// GetStats 获取接口统计
func (m *Manager) GetStats(iface string) (*NetworkStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[iface]
	if !exists {
		return nil, fmt.Errorf("stats for %q not found", iface)
	}
	return stats, nil
}

// GetAllStats 获取所有统计
func (m *Manager) GetAllStats() map[string]NetworkStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]NetworkStats, len(m.stats))
	for k, v := range m.stats {
		result[k] = *v
	}
	return result
}

// SetOnChangeCallback 设置变更回调
func (m *Manager) SetOnChangeCallback(cb func(string, string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = cb
}

// GetSystemInterfaces 获取系统网络接口
func (m *Manager) GetSystemInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

// ExportConfig 导出配置
func (m *Manager) ExportConfig() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := struct {
		Bonds map[string]*BondInterface `json:"bonds"`
		VLANs map[string]*VLANConfig    `json:"vlans"`
	}{
		Bonds: m.bonds,
		VLANs: m.vlans,
	}
	return json.MarshalIndent(config, "", "  ")
}

// ImportConfig 导入配置
func (m *Manager) ImportConfig(data []byte) error {
	var config struct {
		Bonds map[string]*BondInterface `json:"bonds"`
		VLANs map[string]*VLANConfig    `json:"vlans"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Bonds != nil {
		m.bonds = config.Bonds
	}
	if config.VLANs != nil {
		m.vlans = config.VLANs
	}
	return nil
}
