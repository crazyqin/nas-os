package trafficshaper

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Manager 流量整形管理器
type Manager struct {
	mu           sync.RWMutex
	config       *TrafficShaperConfig
	rules        map[string]*TrafficRule
	classes      map[string]*TrafficClass
	stats        map[string]*TrafficStats
	events       []*TrafficEvent
	ruleCounter  int
	classCounter int
	eventCounter int
}

// NewManager 创建流量整形管理器
func NewManager() *Manager {
	return &Manager{
		config:  DefaultTrafficShaperConfig(),
		rules:   make(map[string]*TrafficRule),
		classes: make(map[string]*TrafficClass),
		stats:   make(map[string]*TrafficStats),
		events:  make([]*TrafficEvent, 0),
	}
}

// CreateRule 创建流量规则
func (m *Manager) CreateRule(rule *TrafficRule) (*TrafficRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !IsValidDirection(rule.Direction) {
		return nil, fmt.Errorf("invalid direction: %s", rule.Direction)
	}
	if !IsValidPriority(rule.Priority) {
		return nil, fmt.Errorf("invalid priority: %d", rule.Priority)
	}
	if !IsValidAction(rule.Action) {
		return nil, fmt.Errorf("invalid action: %s", rule.Action)
	}

	// 设置默认协议
	if rule.Protocol == "" {
		rule.Protocol = ProtocolAny
	}
	if !IsValidProtocol(rule.Protocol) {
		return nil, fmt.Errorf("invalid protocol: %s", rule.Protocol)
	}

	m.ruleCounter++
	rule.ID = fmt.Sprintf("rule-%d", m.ruleCounter)
	rule.Enabled = true
	rule.CreatedAt = time.Now()

	m.rules[rule.ID] = rule

	// 初始化统计
	m.stats[rule.ID] = &TrafficStats{
		RuleID:    rule.ID,
		LastReset: time.Now(),
	}

	return rule, nil
}

// ListRules 列出所有流量规则
func (m *Manager) ListRules() []*TrafficRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*TrafficRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// UpdateRule 更新流量规则
func (m *Manager) UpdateRule(id string, rule *TrafficRule) (*TrafficRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	if rule.Name != "" {
		existing.Name = rule.Name
	}
	if rule.Direction != "" {
		if !IsValidDirection(rule.Direction) {
			return nil, fmt.Errorf("invalid direction: %s", rule.Direction)
		}
		existing.Direction = rule.Direction
	}
	if rule.Priority != 0 {
		if !IsValidPriority(rule.Priority) {
			return nil, fmt.Errorf("invalid priority: %d", rule.Priority)
		}
		existing.Priority = rule.Priority
	}
	if rule.Protocol != "" {
		if !IsValidProtocol(rule.Protocol) {
			return nil, fmt.Errorf("invalid protocol: %s", rule.Protocol)
		}
		existing.Protocol = rule.Protocol
	}
	if rule.SourceIP != "" {
		existing.SourceIP = rule.SourceIP
	}
	if rule.DestIP != "" {
		existing.DestIP = rule.DestIP
	}
	if rule.PortRange != "" {
		existing.PortRange = rule.PortRange
	}
	if rule.MaxBandwidth != 0 {
		existing.MaxBandwidth = rule.MaxBandwidth
	}
	if rule.GuaranteedBandwidth != 0 {
		existing.GuaranteedBandwidth = rule.GuaranteedBandwidth
	}
	if rule.BurstSize != 0 {
		existing.BurstSize = rule.BurstSize
	}
	if rule.Action != "" {
		if !IsValidAction(rule.Action) {
			return nil, fmt.Errorf("invalid action: %s", rule.Action)
		}
		existing.Action = rule.Action
	}

	return existing, nil
}

// DeleteRule 删除流量规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(m.rules, id)
	delete(m.stats, id)
	return nil
}

// ToggleRule 启用/禁用流量规则
func (m *Manager) ToggleRule(id string) (*TrafficRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	rule.Enabled = !rule.Enabled
	return rule, nil
}

// CreateClass 创建流量类别
func (m *Manager) CreateClass(class *TrafficClass) (*TrafficClass, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.classCounter++
	class.ID = fmt.Sprintf("class-%d", m.classCounter)
	class.CurrentUsage = 0
	class.RuleCount = 0

	m.classes[class.ID] = class
	return class, nil
}

// ListClasses 列出所有流量类别
func (m *Manager) ListClasses() []*TrafficClass {
	m.mu.RLock()
	defer m.mu.RUnlock()

	classes := make([]*TrafficClass, 0, len(m.classes))
	for _, class := range m.classes {
		classes = append(classes, class)
	}
	return classes
}

// UpdateClass 更新流量类别
func (m *Manager) UpdateClass(id string, class *TrafficClass) (*TrafficClass, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.classes[id]
	if !exists {
		return nil, fmt.Errorf("class not found: %s", id)
	}

	if class.Name != "" {
		existing.Name = class.Name
	}
	if class.Priority != 0 {
		existing.Priority = class.Priority
	}
	if class.MaxBandwidth != 0 {
		existing.MaxBandwidth = class.MaxBandwidth
	}
	if class.GuaranteedBandwidth != 0 {
		existing.GuaranteedBandwidth = class.GuaranteedBandwidth
	}
	if class.Description != "" {
		existing.Description = class.Description
	}

	return existing, nil
}

// DeleteClass 删除流量类别
func (m *Manager) DeleteClass(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.classes[id]; !exists {
		return fmt.Errorf("class not found: %s", id)
	}

	delete(m.classes, id)
	return nil
}

// GetGlobalStats 获取全局流量统计
func (m *Manager) GetGlobalStats() *TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	global := &TrafficStats{
		RuleID:    "global",
		LastReset: time.Now(),
	}

	for _, stat := range m.stats {
		global.BytesIn += stat.BytesIn
		global.BytesOut += stat.BytesOut
		global.PacketsIn += stat.PacketsIn
		global.PacketsOut += stat.PacketsOut
		global.DropsIn += stat.DropsIn
		global.DropsOut += stat.DropsOut
		global.CurrentBpsIn += stat.CurrentBpsIn
		global.CurrentBpsOut += stat.CurrentBpsOut
		if stat.PeakBps > global.PeakBps {
			global.PeakBps = stat.PeakBps
		}
	}

	return global
}

// GetRuleStats 获取指定规则的流量统计
func (m *Manager) GetRuleStats(ruleID string) (*TrafficStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stat, exists := m.stats[ruleID]
	if !exists {
		return nil, fmt.Errorf("stats not found for rule: %s", ruleID)
	}

	return stat, nil
}

// GetAllocation 获取带宽分配情况
func (m *Manager) GetAllocation() *BandwidthAllocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allocation := &BandwidthAllocation{
		TotalBandwidth: m.config.TotalBandwidth,
		Classes:        make([]ClassAllocation, 0, len(m.classes)),
	}

	for _, class := range m.classes {
		allocated := class.MaxBandwidth
		if allocated == 0 {
			allocated = m.config.DefaultMaxBps
		}

		percentage := float64(0)
		if m.config.TotalBandwidth > 0 {
			percentage = float64(allocated) / float64(m.config.TotalBandwidth) * 100
		}

		allocation.Classes = append(allocation.Classes, ClassAllocation{
			ClassID:    class.ID,
			ClassName:  class.Name,
			Allocated:  allocated,
			Used:       class.CurrentUsage,
			Percentage: percentage,
		})

		allocation.AllocatedBandwidth += allocated
	}

	allocation.FreeBandwidth = allocation.TotalBandwidth - allocation.AllocatedBandwidth
	if allocation.FreeBandwidth < 0 {
		allocation.FreeBandwidth = 0
	}

	return allocation
}

// Rebalance 重新平衡带宽分配
func (m *Manager) Rebalance() *BandwidthAllocation {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.classes) == 0 {
		return &BandwidthAllocation{
			TotalBandwidth: m.config.TotalBandwidth,
			FreeBandwidth:  m.config.TotalBandwidth,
			Classes:        make([]ClassAllocation, 0),
		}
	}

	// 计算每个类别的权重（基于优先级）
	totalPriority := 0
	for _, class := range m.classes {
		totalPriority += class.Priority
	}

	if totalPriority == 0 {
		totalPriority = len(m.classes)
	}

	// 按优先级分配带宽
	for _, class := range m.classes {
		weight := float64(class.Priority) / float64(totalPriority)
		newBandwidth := int64(float64(m.config.TotalBandwidth) * weight)
		class.MaxBandwidth = newBandwidth
	}

	// 直接计算分配结果，避免死锁
	allocation := &BandwidthAllocation{
		TotalBandwidth: m.config.TotalBandwidth,
		Classes:        make([]ClassAllocation, 0, len(m.classes)),
	}

	for _, class := range m.classes {
		allocated := class.MaxBandwidth
		if allocated == 0 {
			allocated = m.config.DefaultMaxBps
		}

		percentage := float64(0)
		if m.config.TotalBandwidth > 0 {
			percentage = float64(allocated) / float64(m.config.TotalBandwidth) * 100
		}

		allocation.Classes = append(allocation.Classes, ClassAllocation{
			ClassID:    class.ID,
			ClassName:  class.Name,
			Allocated:  allocated,
			Used:       class.CurrentUsage,
			Percentage: percentage,
		})

		allocation.AllocatedBandwidth += allocated
	}

	allocation.FreeBandwidth = allocation.TotalBandwidth - allocation.AllocatedBandwidth
	if allocation.FreeBandwidth < 0 {
		allocation.FreeBandwidth = 0
	}

	return allocation
}

// GetEvents 获取流量事件日志
func (m *Manager) GetEvents() []*TrafficEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*TrafficEvent, len(m.events))
	copy(events, m.events)
	return events
}

// SimulateTraffic 模拟流量数据
func (m *Manager) SimulateTraffic() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for ruleID, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		stat, exists := m.stats[ruleID]
		if !exists {
			continue
		}

		// 生成随机流量数据
		bytesIn := int64(rand.Intn(10000000)) // 0-10MB
		bytesOut := int64(rand.Intn(10000000))
		packetsIn := int64(rand.Intn(100000))
		packetsOut := int64(rand.Intn(100000))

		// 计算当前速率 (bytes/sec)
		currentBpsIn := bytesIn
		currentBpsOut := bytesOut

		// 更新统计
		stat.BytesIn += bytesIn
		stat.BytesOut += bytesOut
		stat.PacketsIn += packetsIn
		stat.PacketsOut += packetsOut
		stat.CurrentBpsIn = currentBpsIn
		stat.CurrentBpsOut = currentBpsOut

		if currentBpsIn+currentBpsOut > stat.PeakBps {
			stat.PeakBps = currentBpsIn + currentBpsOut
		}

		// 检查是否超过带宽限制
		if rule.MaxBandwidth > 0 && currentBpsIn+currentBpsOut > rule.MaxBandwidth {
			// 生成溢出事件
			m.eventCounter++
			event := &TrafficEvent{
				ID:            fmt.Sprintf("event-%d", m.eventCounter),
				RuleID:        ruleID,
				EventType:     EventOverflow,
				BytesAffected: (currentBpsIn + currentBpsOut) - rule.MaxBandwidth,
				Timestamp:     time.Now(),
				Details:       fmt.Sprintf("Traffic exceeded max bandwidth: %d > %d", currentBpsIn+currentBpsOut, rule.MaxBandwidth),
			}
			m.events = append(m.events, event)

			// 根据规则动作生成事件
			switch rule.Action {
			case ActionBlock:
				m.eventCounter++
				m.events = append(m.events, &TrafficEvent{
					ID:            fmt.Sprintf("event-%d", m.eventCounter),
					RuleID:        ruleID,
					EventType:     EventBlock,
					BytesAffected: bytesIn + bytesOut,
					Timestamp:     time.Now(),
					Details:       "Traffic blocked due to bandwidth limit",
				})
				stat.DropsIn += bytesIn
				stat.DropsOut += bytesOut
			case ActionShape:
				m.eventCounter++
				m.events = append(m.events, &TrafficEvent{
					ID:            fmt.Sprintf("event-%d", m.eventCounter),
					RuleID:        ruleID,
					EventType:     EventThrottle,
					BytesAffected: (currentBpsIn + currentBpsOut) - rule.MaxBandwidth,
					Timestamp:     time.Now(),
					Details:       "Traffic throttled to max bandwidth",
				})
			}
		} else {
			// 流量在限制内
			m.eventCounter++
			m.events = append(m.events, &TrafficEvent{
				ID:            fmt.Sprintf("event-%d", m.eventCounter),
				RuleID:        ruleID,
				EventType:     EventAllow,
				BytesAffected: bytesIn + bytesOut,
				Timestamp:     time.Now(),
				Details:       "Traffic allowed within limits",
			})
		}

		// 限制事件数量
		if m.config.MaxEvents > 0 && len(m.events) > m.config.MaxEvents {
			m.events = m.events[len(m.events)-m.config.MaxEvents:]
		}
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *TrafficShaperConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *TrafficShaperConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
}
