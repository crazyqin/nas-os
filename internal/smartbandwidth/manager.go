package smartbandwidth

import (
	"fmt"
	"time"
)

// Manager 带宽管理器封装
type Manager struct {
	*SmartBandwidthManager
}

// NewManager 创建带宽管理器实例
func NewManager(config *SmartBandwidthConfig) *Manager {
	return &Manager{
		SmartBandwidthManager: NewSmartBandwidthManager(config),
	}
}

// StartDynamicAdjustment 启动动态调整
func (m *Manager) StartDynamicAdjustment(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(time.Duration(m.config.AdjustInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				if err := m.AdjustDynamic(); err != nil {
					fmt.Printf("动态调整失败: %v\n", err)
				}
			}
		}
	}()
}

// GetClassSummary 获取流量类型汇总
func (m *Manager) GetClassSummary() map[TrafficClass]*ClassSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := make(map[TrafficClass]*ClassSummary)

	for _, stats := range m.stats {
		if stats.TrafficClass == "" {
			continue
		}

		s, exists := summary[stats.TrafficClass]
		if !exists {
			s = &ClassSummary{
				TrafficClass: stats.TrafficClass,
				RuleCount:    0,
				TotalMbps:    0,
				TotalBytes:   0,
				TotalPackets: 0,
			}
			summary[stats.TrafficClass] = s
		}

		s.RuleCount++
		s.TotalMbps += stats.CurrentMbps
		s.TotalBytes += stats.TotalBytes
		s.TotalPackets += stats.Packets
	}

	return summary
}

// ClassSummary 流量类型汇总
type ClassSummary struct {
	TrafficClass TrafficClass `json:"traffic_class"`
	RuleCount    int          `json:"rule_count"`
	TotalMbps    float64      `json:"total_mbps"`
	TotalBytes   int64        `json:"total_bytes"`
	TotalPackets int64        `json:"total_packets"`
}

// GetBandwidthUsage 获取带宽使用情况
func (m *Manager) GetBandwidthUsage() *BandwidthUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usage := &BandwidthUsage{
		TotalMbps:    float64(m.config.TotalBandwidthMbps),
		UsedMbps:     0,
		FreeMbps:     float64(m.config.TotalBandwidthMbps),
		Utilization:  0,
		RuleCount:    len(m.rules),
		EnabledRules: 0,
		UpdatedAt:    time.Now(),
	}

	for _, rule := range m.rules {
		if rule.Enabled {
			usage.EnabledRules++
		}
	}

	for _, stats := range m.stats {
		usage.UsedMbps += stats.CurrentMbps
	}

	if usage.TotalMbps > 0 {
		usage.Utilization = usage.UsedMbps / usage.TotalMbps * 100
	}
	usage.FreeMbps = usage.TotalMbps - usage.UsedMbps
	if usage.FreeMbps < 0 {
		usage.FreeMbps = 0
	}

	return usage
}

// BandwidthUsage 带宽使用情况
type BandwidthUsage struct {
	TotalMbps    float64   `json:"total_mbps"`
	UsedMbps     float64   `json:"used_mbps"`
	FreeMbps     float64   `json:"free_mbps"`
	Utilization  float64   `json:"utilization_percent"`
	RuleCount    int       `json:"rule_count"`
	EnabledRules int       `json:"enabled_rules"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ResetStats 重置所有统计
func (m *Manager) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stats := range m.stats {
		stats.CurrentMbps = 0
		stats.TotalBytes = 0
		stats.Packets = 0
		stats.Utilization = 0
		stats.LastUpdated = time.Now()
	}
}

// ResetRuleStats 重置指定规则统计
func (m *Manager) ResetRuleStats(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, exists := m.stats[ruleID]
	if !exists {
		return fmt.Errorf("统计不存在: %s", ruleID)
	}

	stats.CurrentMbps = 0
	stats.TotalBytes = 0
	stats.Packets = 0
	stats.Utilization = 0
	stats.LastUpdated = time.Now()

	return nil
}

// UpdateStats 更新统计数据
func (m *Manager) UpdateStats(ruleID string, currentMbps float64, bytes int64, packets int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, exists := m.stats[ruleID]
	if !exists {
		return fmt.Errorf("统计不存在: %s", ruleID)
	}

	stats.CurrentMbps = currentMbps
	stats.TotalBytes += bytes
	stats.Packets += packets
	stats.LastUpdated = time.Now()

	// 计算利用率
	rule, exists := m.rules[ruleID]
	if exists && rule.MaxMbps > 0 {
		stats.Utilization = currentMbps / float64(rule.MaxMbps) * 100
	}

	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *SmartBandwidthConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *SmartBandwidthConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.TotalBandwidthMbps > 0 {
		m.config.TotalBandwidthMbps = config.TotalBandwidthMbps
	}
	if config.Interface != "" {
		m.config.Interface = config.Interface
	}
	if config.AdjustInterval > 0 {
		m.config.AdjustInterval = config.AdjustInterval
	}
	if config.Enabled != m.config.Enabled {
		m.config.Enabled = config.Enabled
	}
}
