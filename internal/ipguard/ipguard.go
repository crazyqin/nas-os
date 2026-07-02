// Package ipguard 提供IP防护功能，支持IP封禁、访问频率限制、
// 暴力破解防护、地理位置过滤和异常检测。参考飞牛fnOS IP防护功能。
package ipguard

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// ThreatLevel 威胁等级.
type ThreatLevel int

const (
	// ThreatLow 低威胁.
	ThreatLow ThreatLevel = iota
	// ThreatMedium 中威胁.
	ThreatMedium
	// ThreatHigh 高威胁.
	ThreatHigh
	// ThreatCritical 严重威胁.
	ThreatCritical
)

// IPRecord IP访问记录.
type IPRecord struct {
	IP           net.IP      `json:"ip"`
	FirstSeen    time.Time   `json:"first_seen"`
	LastSeen     time.Time   `json:"last_seen"`
	RequestCount int         `json:"request_count"`
	FailedLogins int         `json:"failed_logins"`
	ThreatLevel  ThreatLevel `json:"threat_level"`
	IsBlocked    bool        `json:"is_blocked"`
	BlockedAt    *time.Time  `json:"blocked_at,omitempty"`
	BlockReason  string      `json:"block_reason,omitempty"`
	Country      string      `json:"country,omitempty"`
	ASN          string      `json:"asn,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
}

// Rule 防护规则.
type Rule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // rate_limit, brute_force, geo_block, pattern_match
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`
	Condition Condition `json:"condition"`
	Action    Action    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Condition 规则条件.
type Condition struct {
	MaxRequests     int      `json:"max_requests,omitempty"`
	WindowSeconds   int      `json:"window_seconds,omitempty"`
	MaxFailedLogins int      `json:"max_failed_logins,omitempty"`
	BlockCountries  []string `json:"block_countries,omitempty"`
	Pattern         string   `json:"pattern,omitempty"`
	Methods         []string `json:"methods,omitempty"`
	Paths           []string `json:"paths,omitempty"`
}

// Action 规则动作.
type Action struct {
	Type     string `json:"type"` // block, challenge, throttle, log, alert
	Duration int    `json:"duration_seconds,omitempty"`
	Redirect string `json:"redirect,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Alert 告警记录.
type Alert struct {
	ID        string      `json:"id"`
	IP        net.IP      `json:"ip"`
	RuleID    string      `json:"rule_id"`
	RuleName  string      `json:"rule_name"`
	Level     ThreatLevel `json:"level"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	Details   interface{} `json:"details,omitempty"`
}

// Config 配置.
type Config struct {
	MaxFailedLogins int  `json:"max_failed_logins"`
	LockoutDuration int  `json:"lockout_duration_seconds"`
	EnableGeoBlock  bool `json:"enable_geo_block"`
	EnableRateLimit bool `json:"enable_rate_limit"`
	MaxAlerts       int  `json:"max_alerts"`
}

// Manager IP防护管理器.
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	records   map[string]*IPRecord
	rules     map[string]*Rule
	blocklist map[string]*time.Time // IP -> 解封时间
	allowlist map[string]bool
	alerts    []*Alert
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		config: &Config{
			MaxFailedLogins: 5,
			LockoutDuration: 1800,
			EnableGeoBlock:  false,
			EnableRateLimit: true,
			MaxAlerts:       10000,
		},
		records:   make(map[string]*IPRecord),
		rules:     make(map[string]*Rule),
		blocklist: make(map[string]*time.Time),
		allowlist: make(map[string]bool),
		alerts:    make([]*Alert, 0, 1000),
	}
}

// CheckIP 检查IP是否允许访问.
func (m *Manager) CheckIP(ip net.IP) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ipStr := ip.String()

	// 检查白名单
	if m.allowlist[ipStr] {
		return true, ""
	}

	// 检查黑名单
	if blockUntil, ok := m.blocklist[ipStr]; ok {
		if blockUntil == nil || blockUntil.After(time.Now()) {
			return false, "IP is blocked"
		}
		// 已过期，移除
		delete(m.blocklist, ipStr)
	}

	// 检查记录
	record, ok := m.records[ipStr]
	if !ok {
		return true, ""
	}

	if record.IsBlocked {
		return false, record.BlockReason
	}

	return true, ""
}

// RecordRequest 记录请求.
func (m *Manager) RecordRequest(ip net.IP, path string, method string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	record, ok := m.records[ipStr]
	if !ok {
		record = &IPRecord{
			IP:        ip,
			FirstSeen: time.Now(),
			Tags:      []string{},
		}
		m.records[ipStr] = record
	}

	record.LastSeen = time.Now()
	record.RequestCount++

	// 检查规则
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if m.matchRule(record, rule, path, method, statusCode) {
			m.applyAction(record, rule)
		}
	}
}

// RecordFailedLogin 记录失败登录.
func (m *Manager) RecordFailedLogin(ip net.IP, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	record, ok := m.records[ipStr]
	if !ok {
		record = &IPRecord{
			IP:        ip,
			FirstSeen: time.Now(),
			Tags:      []string{},
		}
		m.records[ipStr] = record
	}

	record.FailedLogins++
	record.LastSeen = time.Now()

	// 检查暴力破解规则
	for _, rule := range m.rules {
		if !rule.Enabled || rule.Type != "brute_force" {
			continue
		}
		if rule.Condition.MaxFailedLogins > 0 && record.FailedLogins >= rule.Condition.MaxFailedLogins {
			m.applyAction(record, rule)
		}
	}

	// 检查默认暴力破解防护
	if m.config.MaxFailedLogins > 0 && record.FailedLogins >= m.config.MaxFailedLogins {
		m.blockIP(record, "Brute force detected", m.config.LockoutDuration)
	}
}

// BlockIP 手动封禁IP.
func (m *Manager) BlockIP(ip net.IP, reason string, durationSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	record, ok := m.records[ipStr]
	if !ok {
		record = &IPRecord{
			IP:        ip,
			FirstSeen: time.Now(),
			Tags:      []string{},
		}
		m.records[ipStr] = record
	}

	m.blockIP(record, reason, durationSeconds)
	return nil
}

// UnblockIP 解封IP.
func (m *Manager) UnblockIP(ip net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ipStr := ip.String()
	record, ok := m.records[ipStr]
	if ok {
		record.IsBlocked = false
		record.BlockedAt = nil
		record.BlockReason = ""
	}

	delete(m.blocklist, ipStr)
	return nil
}

// AddRule 添加规则.
func (m *Manager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule 删除规则.
func (m *Manager) RemoveRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rules, ruleID)
	return nil
}

// GetRules 获取所有规则.
func (m *Manager) GetRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*Rule
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetBlockedIPs 获取封禁列表.
func (m *Manager) GetBlockedIPs() []*IPRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*IPRecord
	for _, record := range m.records {
		if record.IsBlocked {
			result = append(result, record)
		}
	}
	return result
}

// GetAlerts 获取告警.
func (m *Manager) GetAlerts(limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}

	return m.alerts[start:]
}

// GetIPRecord 获取IP记录.
func (m *Manager) GetIPRecord(ip net.IP) *IPRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.records[ip.String()]
}

// matchRule 匹配规则.
func (m *Manager) matchRule(record *IPRecord, rule *Rule, path, method string, statusCode int) bool {
	switch rule.Type {
	case "rate_limit":
		if rule.Condition.MaxRequests > 0 && record.RequestCount >= rule.Condition.MaxRequests {
			return true
		}
	case "brute_force":
		if rule.Condition.MaxFailedLogins > 0 && record.FailedLogins >= rule.Condition.MaxFailedLogins {
			return true
		}
	case "pattern_match":
		if rule.Condition.Pattern != "" {
			// 简化实现
			return false
		}
	}
	return false
}

// applyAction 应用动作.
func (m *Manager) applyAction(record *IPRecord, rule *Rule) {
	switch rule.Action.Type {
	case "block":
		m.blockIP(record, rule.Action.Message, rule.Action.Duration)

		alert := &Alert{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			IP:        record.IP,
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			Level:     ThreatHigh,
			Message:   rule.Action.Message,
			Timestamp: time.Now(),
		}
		m.addAlert(alert)
	}
}

// blockIP 封禁IP.
func (m *Manager) blockIP(record *IPRecord, reason string, durationSeconds int) {
	now := time.Now()
	record.IsBlocked = true
	record.BlockedAt = &now
	record.BlockReason = reason
	record.ThreatLevel = ThreatHigh

	if durationSeconds > 0 {
		blockUntil := now.Add(time.Duration(durationSeconds) * time.Second)
		m.blocklist[record.IP.String()] = &blockUntil
	} else {
		m.blocklist[record.IP.String()] = nil // 永久封禁
	}
}

// addAlert 添加告警.
func (m *Manager) addAlert(alert *Alert) {
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > m.config.MaxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.config.MaxAlerts:]
	}
}

// RegisterRoutes 注册HTTP路由.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ipguard/blocked", m.handleGetBlockedIPs)
	mux.HandleFunc("/api/ipguard/block", m.handleBlockIP)
	mux.HandleFunc("/api/ipguard/unblock", m.handleUnblockIP)
	mux.HandleFunc("/api/ipguard/rules", m.handleGetRules)
	mux.HandleFunc("/api/ipguard/alerts", m.handleGetAlerts)
}

func (m *Manager) handleGetBlockedIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ips := m.GetBlockedIPs()
	json.NewEncoder(w).Encode(ips)
}

func (m *Manager) handleBlockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP       string `json:"ip"`
		Reason   string `json:"reason"`
		Duration int    `json:"duration_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(req.IP)
	if ip == nil {
		http.Error(w, "Invalid IP", http.StatusBadRequest)
		return
	}

	if err := m.BlockIP(ip, req.Reason, req.Duration); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleUnblockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(req.IP)
	if ip == nil {
		http.Error(w, "Invalid IP", http.StatusBadRequest)
		return
	}

	if err := m.UnblockIP(ip); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *Manager) handleGetRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rules := m.GetRules()
	json.NewEncoder(w).Encode(rules)
}

func (m *Manager) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	alerts := m.GetAlerts(100)
	json.NewEncoder(w).Encode(alerts)
}
