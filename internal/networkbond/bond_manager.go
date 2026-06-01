// Package networkbond 提供网卡绑定/链路聚合功能
// 本文件提供 QoS、VLAN、带宽聚合、链路健康监控等扩展管理功能
package networkbond

import (
	"fmt"
	"sync"
	"time"
)

// BondExtendedManager 扩展绑定管理器
type BondExtendedManager struct {
	mu       sync.RWMutex
	extended map[string]*BondExtended
	bondMgr  *BondManager
}

// NewBondExtendedManager 创建扩展绑定管理器
func NewBondExtendedManager(bondMgr *BondManager) *BondExtendedManager {
	return &BondExtendedManager{
		extended: make(map[string]*BondExtended),
		bondMgr:  bondMgr,
	}
}

// InitBondExtended 初始化绑定扩展配置
func (m *BondExtendedManager) InitBondExtended(bondName string) (*BondExtended, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证绑定存在
	_, err := m.bondMgr.GetBond(bondName)
	if err != nil {
		return nil, err
	}

	if _, exists := m.extended[bondName]; exists {
		return nil, fmt.Errorf("extended config for bond %s already exists", bondName)
	}

	ext := &BondExtended{
		BondName: bondName,
		Bandwidth: &BandwidthConfig{
			AggregateMode: AggregateBandwidth,
			HashPolicy:    HashLayer34,
			Resilience:    true,
		},
		Failover: &FailoverPolicy{
			Enabled:          true,
			Interval:         5 * time.Second,
			FailThreshold:    3,
			RecoverThreshold: 2,
			GracePeriod:      10 * time.Second,
		},
	}

	m.extended[bondName] = ext
	return ext, nil
}

// GetBondExtended 获取绑定扩展配置
func (m *BondExtendedManager) GetBondExtended(bondName string) (*BondExtended, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return nil, fmt.Errorf("extended config for bond %s not found", bondName)
	}
	return ext, nil
}

// AddQoSRule 添加QoS规则
func (m *BondExtendedManager) AddQoSRule(bondName string, rule *QoSRule) error {
	if rule == nil {
		return fmt.Errorf("QoS rule cannot be nil")
	}
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid QoS rule: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	// 检查名称重复
	for _, r := range ext.QoSRules {
		if r.Name == rule.Name {
			return fmt.Errorf("QoS rule %s already exists", rule.Name)
		}
	}

	ext.QoSRules = append(ext.QoSRules, rule)
	return nil
}

// RemoveQoSRule 移除QoS规则
func (m *BondExtendedManager) RemoveQoSRule(bondName, ruleName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	for i, r := range ext.QoSRules {
		if r.Name == ruleName {
			ext.QoSRules = append(ext.QoSRules[:i], ext.QoSRules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("QoS rule %s not found", ruleName)
}

// ListQoSRules 列出QoS规则
func (m *BondExtendedManager) ListQoSRules(bondName string) ([]*QoSRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return nil, fmt.Errorf("extended config for bond %s not found", bondName)
	}

	rules := make([]*QoSRule, len(ext.QoSRules))
	copy(rules, ext.QoSRules)
	return rules, nil
}

// AddVLAN 添加VLAN配置
func (m *BondExtendedManager) AddVLAN(bondName string, vlan *VLANConfig) error {
	if vlan == nil {
		return fmt.Errorf("VLAN config cannot be nil")
	}
	if err := vlan.Validate(); err != nil {
		return fmt.Errorf("invalid VLAN config: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	// 检查VLAN ID重复
	for _, v := range ext.VLANs {
		if v.ID == vlan.ID {
			return fmt.Errorf("VLAN %d already exists on bond %s", vlan.ID, bondName)
		}
	}

	// 设置默认MTU
	if vlan.MTU == 0 {
		vlan.MTU = 1500
	}

	ext.VLANs = append(ext.VLANs, vlan)
	return nil
}

// RemoveVLAN 移除VLAN
func (m *BondExtendedManager) RemoveVLAN(bondName string, vlanID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	for i, v := range ext.VLANs {
		if v.ID == vlanID {
			ext.VLANs = append(ext.VLANs[:i], ext.VLANs[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("VLAN %d not found on bond %s", vlanID, bondName)
}

// ListVLANs 列出VLAN
func (m *BondExtendedManager) ListVLANs(bondName string) ([]*VLANConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return nil, fmt.Errorf("extended config for bond %s not found", bondName)
	}

	vlans := make([]*VLANConfig, len(ext.VLANs))
	copy(vlans, ext.VLANs)
	return vlans, nil
}

// UpdateBandwidthConfig 更新带宽配置
func (m *BondExtendedManager) UpdateBandwidthConfig(bondName string, cfg *BandwidthConfig) error {
	if cfg == nil {
		return fmt.Errorf("bandwidth config cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	ext.Bandwidth = cfg
	return nil
}

// UpdateFailoverPolicy 更新故障切换策略
func (m *BondExtendedManager) UpdateFailoverPolicy(bondName string, policy *FailoverPolicy) error {
	if policy == nil {
		return fmt.Errorf("failover policy cannot be nil")
	}
	if policy.Interval <= 0 {
		return fmt.Errorf("failover interval must be positive")
	}
	if policy.FailThreshold < 1 {
		return fmt.Errorf("fail threshold must be at least 1")
	}
	if policy.RecoverThreshold < 1 {
		return fmt.Errorf("recover threshold must be at least 1")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	ext.Failover = policy
	return nil
}

// CalculateTotalBandwidth 计算绑定总带宽
func (m *BondExtendedManager) CalculateTotalBandwidth(bondName string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return 0, fmt.Errorf("extended config for bond %s not found", bondName)
	}

	bond, err := m.bondMgr.GetBond(bondName)
	if err != nil {
		return 0, err
	}

	var totalSpeed int
	for _, iface := range bond.Interfaces {
		if iface.State == StateUp {
			totalSpeed += iface.Speed
		}
	}

	// 冗余模式下只取最低带宽
	if ext.Bandwidth != nil && ext.Bandwidth.AggregateMode == AggregateRedundancy {
		minSpeed := 0
		for _, iface := range bond.Interfaces {
			if iface.State == StateUp {
				if minSpeed == 0 || iface.Speed < minSpeed {
					minSpeed = iface.Speed
				}
			}
		}
		return minSpeed, nil
	}

	return totalSpeed, nil
}

// SimulateFailover 模拟链路故障切换
func (m *BondExtendedManager) SimulateFailover(bondName, failedIface string) (*LinkHealthState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extended[bondName]
	if !exists {
		return nil, fmt.Errorf("extended config for bond %s not found", bondName)
	}

	if !ext.Failover.Enabled {
		return nil, fmt.Errorf("failover is disabled for bond %s", bondName)
	}

	// 触发底层故障切换
	err := m.bondMgr.FailoverTrigger(bondName, failedIface)
	if err != nil {
		return nil, err
	}

	health := &LinkHealthState{
		InterfaceName: failedIface,
		PacketLoss:    100.0,
		LastCheck:     time.Now(),
		Healthy:       false,
	}

	return health, nil
}

// GetLinkHealth 获取链路健康状态
func (m *BondExtendedManager) GetLinkHealth(bondName string) ([]*LinkHealthState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.extended[bondName]
	if !exists {
		return nil, fmt.Errorf("extended config for bond %s not found", bondName)
	}

	bond, err := m.bondMgr.GetBond(bondName)
	if err != nil {
		return nil, err
	}

	states := make([]*LinkHealthState, 0, len(bond.Interfaces))
	for _, iface := range bond.Interfaces {
		state := &LinkHealthState{
			InterfaceName: iface.Name,
			LastCheck:     time.Now(),
			Healthy:       iface.State == StateUp && iface.LinkState,
		}
		if !state.Healthy {
			state.PacketLoss = 100.0
		}
		states = append(states, state)
	}

	return states, nil
}

// DeleteBondExtended 删除绑定扩展配置
func (m *BondExtendedManager) DeleteBondExtended(bondName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.extended[bondName]; !exists {
		return fmt.Errorf("extended config for bond %s not found", bondName)
	}

	delete(m.extended, bondName)
	return nil
}
