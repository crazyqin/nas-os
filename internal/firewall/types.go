package firewall

import (
	"net"
	"sync"
	"time"
)

// RuleAction 规则动作.
type RuleAction string

const (
	ActionAllow RuleAction = "allow"
	ActionDeny  RuleAction = "deny"
	ActionLog   RuleAction = "log"
)

// Protocol 网络协议.
type Protocol string

const (
	ProtoTCP  Protocol = "tcp"
	ProtoUDP  Protocol = "udp"
	ProtoICMP Protocol = "icmp"
	ProtoAny  Protocol = "any"
)

// Rule 防火墙规则.
type Rule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `json:"enabled"`
	Priority    int        `json:"priority"`
	Action      RuleAction `json:"action"`
	Protocol    Protocol   `json:"protocol"`
	// Source 源地址（CIDR 或 IP）.
	Source string `json:"source,omitempty"`
	// Dest 目标地址.
	Dest string `json:"dest,omitempty"`
	// SrcPort 源端口范围 "80" 或 "8000-9000".
	SrcPort string `json:"src_port,omitempty"`
	// DstPort 目标端口范围.
	DstPort   string    `json:"dst_port,omitempty"`
	Direction Direction `json:"direction"`
	// LogEnabled 是否记录匹配日志.
	LogEnabled bool      `json:"log_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// HitCount 规则命中次数.
	HitCount int64 `json:"hit_count"`
}

// Direction 流量方向.
type Direction string

const (
	DirInbound  Direction = "inbound"
	DirOutbound Direction = "outbound"
	DirBoth     Direction = "both"
)

// Zone 防火墙区域.
type Zone struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Interfaces  []string `json:"interfaces"`
	DefaultAction RuleAction `json:"default_action"`
	Rules       []string `json:"rules"` // Rule IDs
}

// FirewallConfig 防火墙全局配置.
type FirewallConfig struct {
	Enabled       bool   `json:"enabled"`
	DefaultIn     RuleAction `json:"default_in"`
	DefaultOut    RuleAction `json:"default_out"`
	LogDropped    bool   `json:"log_dropped"`
	MaxRules      int    `json:"max_rules"`
	SyncIntervalS int    `json:"sync_interval_s"`
}

// TrafficLog 流量日志.
type TrafficLog struct {
	Timestamp time.Time `json:"timestamp"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Action    RuleAction `json:"action"`
	Protocol  Protocol  `json:"protocol"`
	SrcIP     net.IP    `json:"src_ip"`
	DstIP     net.IP    `json:"dst_ip"`
	SrcPort   int       `json:"src_port"`
	DstPort   int       `json:"dst_port"`
	Bytes     int64     `json:"bytes"`
}

// FirewallStats 防火墙统计.
type FirewallStats struct {
	TotalRules    int   `json:"total_rules"`
	EnabledRules  int   `json:"enabled_rules"`
	TotalHits     int64 `json:"total_hits"`
	DroppedCount  int64 `json:"dropped_count"`
	AllowedCount  int64 `json:"allowed_count"`
	LogCount      int64 `json:"log_count"`
	LastSyncAt    time.Time `json:"last_sync_at"`
}

// Manager 防火墙管理器.
type Manager struct {
	mu          sync.RWMutex
	config      *FirewallConfig
	rules       map[string]*Rule
	zones       map[string]*Zone
	trafficLog  []TrafficLog
	stats       FirewallStats
	maxLogSize  int
}

// NewManager 创建防火墙管理器.
func NewManager() *Manager {
	return &Manager{
		config: &FirewallConfig{
			Enabled:       true,
			DefaultIn:     ActionDeny,
			DefaultOut:    ActionAllow,
			LogDropped:    true,
			MaxRules:      1000,
			SyncIntervalS: 60,
		},
		rules:      make(map[string]*Rule),
		zones:      make(map[string]*Zone),
		trafficLog: make([]TrafficLog, 0, 1000),
		maxLogSize: 10000,
	}
}

// AddRule 添加规则.
func (m *Manager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.rules) >= m.config.MaxRules {
		return ErrMaxRulesReached
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	m.updateStatsLocked()
	return nil
}

// UpdateRule 更新规则.
func (m *Manager) UpdateRule(id string, update *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}

	update.ID = id
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()
	update.HitCount = existing.HitCount
	m.rules[id] = update
	return nil
}

// DeleteRule 删除规则.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return ErrRuleNotFound
	}
	delete(m.rules, id)
	m.updateStatsLocked()
	return nil
}

// GetRule 获取规则.
func (m *Manager) GetRule(id string) (*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有规则.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// EnableRule 启用规则.
func (m *Manager) EnableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}
	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableRule 禁用规则.
func (m *Manager) DisableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}
	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	return nil
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *FirewallConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *FirewallConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetStats 获取统计.
func (m *Manager) GetStats() FirewallStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetTrafficLog 获取流量日志.
func (m *Manager) GetTrafficLog(limit int) []TrafficLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.trafficLog) {
		limit = len(m.trafficLog)
	}
	start := len(m.trafficLog) - limit
	if start < 0 {
		start = 0
	}
	result := make([]TrafficLog, limit)
	copy(result, m.trafficLog[start:])
	return result
}

// AddZone 添加区域.
func (m *Manager) AddZone(zone *Zone) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zones[zone.Name] = zone
}

// GetZone 获取区域.
func (m *Manager) GetZone(name string) (*Zone, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	z, ok := m.zones[name]
	return z, ok
}

// ListZones 列出所有区域.
func (m *Manager) ListZones() []*Zone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zones := make([]*Zone, 0, len(m.zones))
	for _, z := range m.zones {
		zones = append(zones, z)
	}
	return zones
}

func (m *Manager) updateStatsLocked() {
	m.stats.TotalRules = len(m.rules)
	enabled := 0
	for _, r := range m.rules {
		if r.Enabled {
			enabled++
		}
	}
	m.stats.EnabledRules = enabled
}
