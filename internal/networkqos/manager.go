package networkqos

import (
	"fmt"
	"time"
)

// GetRule 获取QoS规则
func (m *QoSManager) GetRuleByID(id string) (*QoSRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}
	return rule, nil
}

// UpdateStats 更新带宽统计
func (m *QoSManager) UpdateStats(ruleID string, inMbps, outMbps float64, bytes, packets int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[ruleID]; !exists {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}

	m.stats[ruleID] = &BandwidthStats{
		RuleID:      ruleID,
		InMbps:      inMbps,
		OutMbps:     outMbps,
		TotalBytes:  bytes,
		Packets:     packets,
		LastUpdated: time.Now(),
	}

	return nil
}

// GetInterfaceStats 获取接口统计
func (m *QoSManager) GetInterfaceStats(name string) (*InterfaceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, exists := m.ifaces[name]
	if !exists {
		return nil, fmt.Errorf("接口不存在: %s", name)
	}
	return iface, nil
}

// RegisterInterface 注册网络接口
func (m *QoSManager) RegisterInterface(name string, speedMbps int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ifaces[name] = &InterfaceInfo{
		Name:  name,
		Speed: speedMbps,
		InUse: true,
	}

	return nil
}

// GetTotalBandwidth 获取总带宽统计
func (m *QoSManager) GetTotalBandwidth() (inMbps, outMbps float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, stats := range m.stats {
		inMbps += stats.InMbps
		outMbps += stats.OutMbps
	}
	return
}

// GetRulesByPriority 按优先级获取规则
func (m *QoSManager) GetRulesByPriority(priority int) []*QoSRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*QoSRule
	for _, rule := range m.rules {
		if rule.Priority == priority {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetEnabledRules 获取启用的规则
func (m *QoSManager) GetEnabledRules() []*QoSRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*QoSRule
	for _, rule := range m.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return rules
}

// ResetStats 重置统计
func (m *QoSManager) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = make(map[string]*BandwidthStats)
}

// GetConfig 获取配置
func (m *QoSManager) GetConfig() *QoSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *QoSManager) UpdateConfig(config *QoSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	m.config = config
	return nil
}

// GetStatsSummary 获取统计摘要
func (m *QoSManager) GetStatsSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalIn, totalOut := 0.0, 0.0
	for _, stats := range m.stats {
		totalIn += stats.InMbps
		totalOut += stats.OutMbps
	}

	return map[string]interface{}{
		"total_rules":    len(m.rules),
		"enabled_rules":  len(m.GetEnabledRules()),
		"total_in_mbps":  totalIn,
		"total_out_mbps": totalOut,
		"interfaces":     len(m.ifaces),
	}
}
