package networkqos

import (
	"fmt"
	"sync"
	"time"
)

// QoSManager 网络QoS管理器
type QoSManager struct {
	mu     sync.RWMutex
	rules  map[string]*QoSRule
	stats  map[string]*BandwidthStats
	ifaces map[string]*InterfaceInfo
	config *QoSConfig
}

// QoSConfig QoS配置
type QoSConfig struct {
	DefaultPriority int    `json:"default_priority"`
	Enabled         bool   `json:"enabled"`
	Interface       string `json:"interface"`
	MaxBandwidth    int64  `json:"max_bandwidth_mbps"`
}

// QoSRule QoS规则
type QoSRule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Priority   int       `json:"priority"`
	Protocol   string    `json:"protocol"`
	SourceIP   string    `json:"source_ip"`
	DestIP     string    `json:"dest_ip"`
	SourcePort int       `json:"source_port"`
	DestPort   int       `json:"dest_port"`
	MinMbps    int64     `json:"min_mbps"`
	MaxMbps    int64     `json:"max_mbps"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// BandwidthStats 带宽统计
type BandwidthStats struct {
	RuleID      string    `json:"rule_id"`
	InMbps      float64   `json:"in_mbps"`
	OutMbps     float64   `json:"out_mbps"`
	TotalBytes  int64     `json:"total_bytes"`
	Packets     int64     `json:"packets"`
	LastUpdated time.Time `json:"last_updated"`
}

// InterfaceInfo 网络接口信息
type InterfaceInfo struct {
	Name        string  `json:"name"`
	Speed       int64   `json:"speed_mbps"`
	InUse       bool    `json:"in_use"`
	Utilization float64 `json:"utilization"`
}

// NewQoSManager 创建QoS管理器
func NewQoSManager(config *QoSConfig) *QoSManager {
	if config == nil {
		config = &QoSConfig{
			DefaultPriority: 5,
			Enabled:         true,
			MaxBandwidth:    1000,
		}
	}
	return &QoSManager{
		rules:  make(map[string]*QoSRule),
		stats:  make(map[string]*BandwidthStats),
		ifaces: make(map[string]*InterfaceInfo),
		config: config,
	}
}

// CreateRule 创建QoS规则
func (m *QoSManager) CreateRule(rule *QoSRule) (*QoSRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	if rule.Priority < 1 || rule.Priority > 10 {
		return nil, fmt.Errorf("优先级必须在1-10之间")
	}

	if rule.MaxMbps <= 0 {
		return nil, fmt.Errorf("最大带宽必须大于0")
	}

	if rule.MinMbps < 0 {
		return nil, fmt.Errorf("最小带宽不能为负数")
	}

	if rule.MinMbps > rule.MaxMbps {
		return nil, fmt.Errorf("最小带宽不能大于最大带宽")
	}

	rule.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true

	m.rules[rule.ID] = rule

	return rule, nil
}

// UpdateRule 更新QoS规则
func (m *QoSManager) UpdateRule(id string, update *QoSRule) (*QoSRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	if update.Name != "" {
		rule.Name = update.Name
	}
	if update.Priority > 0 && update.Priority <= 10 {
		rule.Priority = update.Priority
	}
	if update.MaxMbps > 0 {
		rule.MaxMbps = update.MaxMbps
	}
	if update.MinMbps >= 0 {
		rule.MinMbps = update.MinMbps
	}
	if update.Protocol != "" {
		rule.Protocol = update.Protocol
	}
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// DeleteRule 删除QoS规则
func (m *QoSManager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(m.rules, id)
	delete(m.stats, id)
	return nil
}

// GetRule 获取QoS规则
func (m *QoSManager) GetRule(id string) (*QoSRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// ListRules 列出所有QoS规则
func (m *QoSManager) ListRules() []*QoSRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*QoSRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetStats 获取带宽统计
func (m *QoSManager) GetStats(ruleID string) (*BandwidthStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[ruleID]
	if !exists {
		return nil, fmt.Errorf("统计不存在: %s", ruleID)
	}

	return stats, nil
}

// GetAllStats 获取所有带宽统计
func (m *QoSManager) GetAllStats() map[string]*BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*BandwidthStats)
	for k, v := range m.stats {
		result[k] = v
	}
	return result
}

// EnableRule 启用规则
func (m *QoSManager) EnableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableRule 禁用规则
func (m *QoSManager) DisableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	return nil
}
