// Package networkbond 提供网卡绑定/链路聚合功能
// 对标企业级 NAS 的 LACP/802.3ad / balance-rr / active-backup
// 支持多种绑定模式 / 健康检测 / 自动故障转移 / 带宽聚合
package networkbond

import (
	"fmt"
	"sync"
	"time"
)

// BondMode 绑定模式.
type BondMode int

const (
	BalanceRR    BondMode = 0 // 轮询负载均衡
	ActiveBackup BondMode = 1 // 主备模式
	BalanceXOR   BondMode = 2 // XOR负载均衡
	Broadcast    BondMode = 3 // 广播模式
	IEEE802_3ad  BondMode = 4 // LACP链路聚合
	BalanceTLB   BondMode = 5 // 自适应传输负载均衡
	BalanceALB   BondMode = 6 // 自适应负载均衡
)

// BondState 绑定状态.
type BondState string

const (
	StateUp       BondState = "up"
	StateDown     BondState = "down"
	StateDegraded BondState = "degraded"
	StateUnknown  BondState = "unknown"
)

// Bond 网卡绑定.
type Bond struct {
	mu           sync.RWMutex
	Name         string       `json:"name"`         // 绑定名称（如 bond0）
	Mode         BondMode     `json:"mode"`         // 绑定模式
	State        BondState    `json:"state"`        // 绑定状态
	MAC          string       `json:"mac"`          // 绑定MAC地址
	IP           string       `json:"ip"`           // 绑定IP
	MTU          int          `json:"mtu"`          // MTU
	Interfaces   []*Interface `json:"interfaces"`   // 成员接口
	Primary      string       `json:"primary"`      // 主接口
	ActiveSlave  string       `json:"activeSlave"`  // 当前活跃从接口
	TransmitHash string       `json:"transmitHash"` // 传输哈希策略
	MIIMonitor   int          `json:"miiMonitor"`   // MII监控间隔(ms)
	UpDelay      int          `json:"upDelay"`      // 上线延迟(ms)
	DownDelay    int          `json:"downDelay"`    // 下线延迟(ms)
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

// Interface 网卡接口.
type Interface struct {
	Name       string    `json:"name"`       // 接口名（如 eth0）
	MAC        string    `json:"mac"`        // MAC地址
	Speed      int       `json:"speed"`      // 速率(Mbps)
	Duplex     string    `json:"duplex"`     // 双工模式
	State      BondState `json:"state"`      // 接口状态
	IsPrimary  bool      `json:"isPrimary"`  // 是否主接口
	LinkState  bool      `json:"linkState"`  // 链路状态
	SlaveState string    `json:"slaveState"` // 从接口状态
	RxBytes    int64     `json:"rxBytes"`    // 接收字节
	TxBytes    int64     `json:"txBytes"`    // 发送字节
	RxPackets  int64     `json:"rxPackets"`  // 接收包数
	TxPackets  int64     `json:"txPackets"`  // 发送包数
	Errors     int64     `json:"errors"`     // 错误数
}

// BondStats 绑定统计.
type BondStats struct {
	BondName         string    `json:"bondName"`
	TotalRxBytes     int64     `json:"totalRxBytes"`
	TotalTxBytes     int64     `json:"totalTxBytes"`
	TotalRxPackets   int64     `json:"totalRxPackets"`
	TotalTxPackets   int64     `json:"totalTxPackets"`
	ActiveInterfaces int       `json:"activeInterfaces"`
	TotalBandwidth   int       `json:"totalBandwidth"` // 总带宽(Mbps)
	UpdatedAt        time.Time `json:"updatedAt"`
}

// BondManager 绑定管理器.
type BondManager struct {
	mu     sync.RWMutex
	bonds  map[string]*Bond
	config *BondManagerConfig
}

// BondManagerConfig 管理器配置.
type BondManagerConfig struct {
	DefaultMTU        int  `json:"defaultMTU"`        // 默认MTU
	DefaultMIIMonitor int  `json:"defaultMIIMonitor"` // 默认MII监控间隔
	DefaultUpDelay    int  `json:"defaultUpDelay"`    // 默认上线延迟
	DefaultDownDelay  int  `json:"defaultDownDelay"`  // 默认下线延迟
	AllowMixedSpeed   bool `json:"allowMixedSpeed"`   // 允许混合速率
}

// NewBondManager 创建绑定管理器.
func NewBondManager(config *BondManagerConfig) *BondManager {
	if config == nil {
		config = &BondManagerConfig{
			DefaultMTU:        1500,
			DefaultMIIMonitor: 100,
			DefaultUpDelay:    200,
			DefaultDownDelay:  200,
		}
	}
	return &BondManager{
		bonds:  make(map[string]*Bond),
		config: config,
	}
}

// CreateBond 创建网卡绑定.
func (m *BondManager) CreateBond(name string, mode BondMode, interfaces []string) (*Bond, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.bonds[name]; exists {
		return nil, fmt.Errorf("bond %s already exists", name)
	}

	if len(interfaces) < 2 && mode == IEEE802_3ad {
		return nil, fmt.Errorf("LACP mode requires at least 2 interfaces")
	}

	bond := &Bond{
		Name:       name,
		Mode:       mode,
		State:      StateDown,
		MTU:        m.config.DefaultMTU,
		MIIMonitor: m.config.DefaultMIIMonitor,
		UpDelay:    m.config.DefaultUpDelay,
		DownDelay:  m.config.DefaultDownDelay,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	for _, ifaceName := range interfaces {
		iface := &Interface{
			Name:  ifaceName,
			State: StateDown,
		}
		bond.Interfaces = append(bond.Interfaces, iface)
	}

	if len(interfaces) > 0 {
		bond.Primary = interfaces[0]
		bond.Interfaces[0].IsPrimary = true
	}

	m.bonds[name] = bond
	return bond, nil
}

// DeleteBond 删除绑定.
func (m *BondManager) DeleteBond(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %s not found", name)
	}

	if bond.State == StateUp {
		return fmt.Errorf("cannot delete active bond %s", name)
	}

	delete(m.bonds, name)
	return nil
}

// GetBond 获取绑定信息.
func (m *BondManager) GetBond(name string) (*Bond, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bond, exists := m.bonds[name]
	if !exists {
		return nil, fmt.Errorf("bond %s not found", name)
	}
	return bond, nil
}

// ListBonds 列出所有绑定.
func (m *BondManager) ListBonds() []*Bond {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bonds := make([]*Bond, 0, len(m.bonds))
	for _, bond := range m.bonds {
		bonds = append(bonds, bond)
	}
	return bonds
}

// ActivateBond 激活绑定.
func (m *BondManager) ActivateBond(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %s not found", name)
	}

	if len(bond.Interfaces) < 1 {
		return fmt.Errorf("bond %s has no interfaces", name)
	}

	bond.State = StateUp
	bond.UpdatedAt = time.Now()

	// 激活所有接口
	for _, iface := range bond.Interfaces {
		iface.State = StateUp
		iface.LinkState = true
		iface.SlaveState = "active"
	}

	// 设置活跃从接口
	bond.ActiveSlave = bond.Primary

	return nil
}

// DeactivateBond 停用绑定.
func (m *BondManager) DeactivateBond(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[name]
	if !exists {
		return fmt.Errorf("bond %s not found", name)
	}

	bond.State = StateDown
	bond.UpdatedAt = time.Now()

	for _, iface := range bond.Interfaces {
		iface.State = StateDown
		iface.LinkState = false
		iface.SlaveState = "inactive"
	}

	return nil
}

// AddInterface 添加接口到绑定.
func (m *BondManager) AddInterface(bondName, ifaceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[bondName]
	if !exists {
		return fmt.Errorf("bond %s not found", bondName)
	}

	// 检查是否已存在
	for _, iface := range bond.Interfaces {
		if iface.Name == ifaceName {
			return fmt.Errorf("interface %s already in bond %s", ifaceName, bondName)
		}
	}

	iface := &Interface{
		Name:  ifaceName,
		State: StateDown,
	}
	bond.Interfaces = append(bond.Interfaces, iface)
	bond.UpdatedAt = time.Now()

	return nil
}

// RemoveInterface 从绑定移除接口.
func (m *BondManager) RemoveInterface(bondName, ifaceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[bondName]
	if !exists {
		return fmt.Errorf("bond %s not found", bondName)
	}

	for i, iface := range bond.Interfaces {
		if iface.Name == ifaceName {
			bond.Interfaces = append(bond.Interfaces[:i], bond.Interfaces[i+1:]...)
			bond.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("interface %s not found in bond %s", ifaceName, bondName)
}

// GetBondStats 获取绑定统计.
func (m *BondManager) GetBondStats(name string) (*BondStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bond, exists := m.bonds[name]
	if !exists {
		return nil, fmt.Errorf("bond %s not found", name)
	}

	stats := &BondStats{
		BondName:  name,
		UpdatedAt: time.Now(),
	}

	for _, iface := range bond.Interfaces {
		if iface.State == StateUp {
			stats.ActiveInterfaces++
			stats.TotalBandwidth += iface.Speed
		}
		stats.TotalRxBytes += iface.RxBytes
		stats.TotalTxBytes += iface.TxBytes
		stats.TotalRxPackets += iface.RxPackets
		stats.TotalTxPackets += iface.TxPackets
	}

	return stats, nil
}

// FailoverTrigger 触发故障转移.
func (m *BondManager) FailoverTrigger(bondName, failedInterface string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, exists := m.bonds[bondName]
	if !exists {
		return fmt.Errorf("bond %s not found", bondName)
	}

	if bond.Mode == ActiveBackup {
		// 找到故障接口并切换
		for _, iface := range bond.Interfaces {
			if iface.Name == failedInterface {
				iface.State = StateDown
				iface.LinkState = false
				iface.SlaveState = "failed"
				break
			}
		}

		// 切换到备用接口
		for _, iface := range bond.Interfaces {
			if iface.Name != failedInterface && iface.State == StateUp {
				bond.ActiveSlave = iface.Name
				break
			}
		}

		bond.UpdatedAt = time.Now()
	}

	return nil
}

// GetBondModeName 获取绑定模式名称.
func GetBondModeName(mode BondMode) string {
	switch mode {
	case BalanceRR:
		return "balance-rr"
	case ActiveBackup:
		return "active-backup"
	case BalanceXOR:
		return "balance-xor"
	case Broadcast:
		return "broadcast"
	case IEEE802_3ad:
		return "802.3ad"
	case BalanceTLB:
		return "balance-tlb"
	case BalanceALB:
		return "balance-alb"
	default:
		return "unknown"
	}
}
