// Package proactivemonitor - 主动监控模块
// 智能预警、异常检测、自动修复
package proactivemonitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Alert 告警.
type Alert struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`     // info, warning, error, critical
	Category   string            `json:"category"` // disk, cpu, memory, network, service, security
	Title      string            `json:"title"`
	Message    string            `json:"message"`
	Source     string            `json:"source"`
	Severity   int               `json:"severity"` // 1-10
	Status     string            `json:"status"`   // active, acknowledged, resolved, suppressed
	FirstSeen  time.Time         `json:"first_seen"`
	LastSeen   time.Time         `json:"last_seen"`
	Count      int               `json:"count"`
	Threshold  *Threshold        `json:"threshold,omitempty"`
	Value      float64           `json:"value,omitempty"`
	AutoAction *AutoAction       `json:"auto_action,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	ResolvedAt *time.Time        `json:"resolved_at,omitempty"`
	ResolvedBy string            `json:"resolved_by,omitempty"`
}

// Threshold 阈值.
type Threshold struct {
	Metric   string  `json:"metric"`
	Operator string  `json:"operator"` // gt, lt, eq, gte, lte
	Value    float64 `json:"value"`
	Duration int     `json:"duration"` // 秒
}

// AutoAction 自动动作.
type AutoAction struct {
	Type       string            `json:"type"` // restart, scale, notify, script, cleanup
	Params     map[string]string `json:"params,omitempty"`
	Executed   bool              `json:"executed"`
	ExecutedAt *time.Time        `json:"executed_at,omitempty"`
	Result     string            `json:"result,omitempty"`
}

// Rule 监控规则.
type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Enabled     bool        `json:"enabled"`
	Category    string      `json:"category"`
	Metric      string      `json:"metric"`
	Condition   Condition   `json:"condition"`
	Duration    int         `json:"duration"` // 秒
	Severity    int         `json:"severity"`
	AutoAction  *AutoAction `json:"auto_action,omitempty"`
	Channels    []string    `json:"channels,omitempty"` // email, webhook, sms
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Condition 条件.
type Condition struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

// Metric 指标.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// HealthCheck 健康检查.
type HealthCheck struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"` // http, tcp, process, script
	Target    string     `json:"target"`
	Interval  int        `json:"interval"` // 秒
	Timeout   int        `json:"timeout"`  // 秒
	Status    string     `json:"status"`   // healthy, unhealthy, unknown
	LastCheck *time.Time `json:"last_check,omitempty"`
	Latency   int        `json:"latency"` // ms
	Message   string     `json:"message,omitempty"`
}

// CreateRuleRequest 创建规则请求.
type CreateRuleRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Category    string      `json:"category"`
	Metric      string      `json:"metric"`
	Condition   Condition   `json:"condition"`
	Duration    int         `json:"duration"`
	Severity    int         `json:"severity"`
	AutoAction  *AutoAction `json:"auto_action,omitempty"`
	Channels    []string    `json:"channels,omitempty"`
}

// AcknowledgeAlertRequest 确认告警请求.
type AcknowledgeAlertRequest struct {
	AlertID string `json:"alert_id"`
	User    string `json:"user"`
	Comment string `json:"comment,omitempty"`
}

// Manager 管理器.
type Manager struct {
	mu           sync.RWMutex
	alerts       map[string]*Alert
	rules        map[string]*Rule
	healthChecks map[string]*HealthCheck
	metrics      []Metric
	config       *Config
	dataFile     string
}

// Config 配置.
type Config struct {
	MaxAlerts       int  `json:"max_alerts"`
	MaxRules        int  `json:"max_rules"`
	MetricRetention int  `json:"metric_retention"` // 天
	DefaultInterval int  `json:"default_interval"` // 秒
	AutoResolve     bool `json:"auto_resolve"`
	AlertCooldown   int  `json:"alert_cooldown"` // 秒
}

// NewManager 创建管理器.
func NewManager(dataFile string) *Manager {
	return &Manager{
		alerts:       make(map[string]*Alert),
		rules:        make(map[string]*Rule),
		healthChecks: make(map[string]*HealthCheck),
		metrics:      make([]Metric, 0),
		config: &Config{
			MaxAlerts:       1000,
			MaxRules:        100,
			MetricRetention: 30,
			DefaultInterval: 60,
			AutoResolve:     true,
			AlertCooldown:   300,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化.
func (m *Manager) Initialize() error {
	m.loadDefaultRules()
	return m.load()
}

func (m *Manager) loadDefaultRules() {
	defaults := []CreateRuleRequest{
		{Name: "磁盘空间警告", Category: "disk", Metric: "disk_usage_percent", Condition: Condition{Operator: "gte", Value: 80}, Severity: 7, Duration: 300},
		{Name: "磁盘空间严重", Category: "disk", Metric: "disk_usage_percent", Condition: Condition{Operator: "gte", Value: 95}, Severity: 10, Duration: 60},
		{Name: "CPU使用率高", Category: "cpu", Metric: "cpu_usage_percent", Condition: Condition{Operator: "gte", Value: 90}, Severity: 6, Duration: 300},
		{Name: "内存使用率高", Category: "memory", Metric: "memory_usage_percent", Condition: Condition{Operator: "gte", Value: 90}, Severity: 7, Duration: 300},
		{Name: "磁盘温度过高", Category: "disk", Metric: "disk_temperature", Condition: Condition{Operator: "gte", Value: 55}, Severity: 8, Duration: 120},
		{Name: "RAID降级", Category: "disk", Metric: "raid_status", Condition: Condition{Operator: "eq", Value: 0}, Severity: 9, Duration: 0},
		{Name: "服务停止", Category: "service", Metric: "service_status", Condition: Condition{Operator: "eq", Value: 0}, Severity: 8, Duration: 60},
	}

	for _, req := range defaults {
		m.CreateRule(req)
	}
}

// CreateRule 创建规则.
func (m *Manager) CreateRule(req CreateRuleRequest) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.rules) >= m.config.MaxRules {
		return nil, fmt.Errorf("已达到最大规则数限制 (%d)", m.config.MaxRules)
	}

	id := fmt.Sprintf("rule_%d", time.Now().UnixNano())
	rule := &Rule{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Category:    req.Category,
		Metric:      req.Metric,
		Condition:   req.Condition,
		Duration:    req.Duration,
		Severity:    req.Severity,
		AutoAction:  req.AutoAction,
		Channels:    req.Channels,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.rules[id] = rule
	return rule, m.save()
}

// GetRule 获取规则.
func (m *Manager) GetRule(id string) (*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("规则 '%s' 不存在", id)
	}
	return rule, nil
}

// ListRules 列出规则.
func (m *Manager) ListRules(category string) []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Rule
	for _, r := range m.rules {
		if category == "" || r.Category == category {
			result = append(result, r)
		}
	}
	return result
}

// DeleteRule 删除规则.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("规则 '%s' 不存在", id)
	}

	delete(m.rules, id)
	return m.save()
}

// ReportMetric 上报指标.
func (m *Manager) ReportMetric(metric Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = append(m.metrics, metric)
	m.evaluateRules(metric)
}

// evaluateRules 评估规则.
func (m *Manager) evaluateRules(metric Metric) {
	for _, rule := range m.rules {
		if !rule.Enabled || rule.Metric != metric.Name {
			continue
		}

		triggered := false
		switch rule.Condition.Operator {
		case "gt":
			triggered = metric.Value > rule.Condition.Value
		case "gte":
			triggered = metric.Value >= rule.Condition.Value
		case "lt":
			triggered = metric.Value < rule.Condition.Value
		case "lte":
			triggered = metric.Value <= rule.Condition.Value
		case "eq":
			triggered = metric.Value == rule.Condition.Value
		}

		if triggered {
			m.createAlert(rule, metric)
		}
	}
}

// createAlert 创建告警.
func (m *Manager) createAlert(rule *Rule, metric Metric) {
	alertID := fmt.Sprintf("alert_%s_%d", rule.ID, time.Now().UnixNano())

	// 检查是否已有相同规则的活跃告警
	for _, alert := range m.alerts {
		if alert.Source == rule.ID && alert.Status == "active" {
			alert.LastSeen = time.Now()
			alert.Count++
			alert.Value = metric.Value
			return
		}
	}

	if len(m.alerts) >= m.config.MaxAlerts {
		return
	}

	alert := &Alert{
		ID:        alertID,
		Type:      severityToType(rule.Severity),
		Category:  rule.Category,
		Title:     rule.Name,
		Message:   fmt.Sprintf("%s: %.2f (阈值: %.2f)", rule.Name, metric.Value, rule.Condition.Value),
		Source:    rule.ID,
		Severity:  rule.Severity,
		Status:    "active",
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
		Count:     1,
		Threshold: &Threshold{
			Metric:   rule.Metric,
			Operator: rule.Condition.Operator,
			Value:    rule.Condition.Value,
			Duration: rule.Duration,
		},
		Value:      metric.Value,
		AutoAction: rule.AutoAction,
		Tags:       metric.Labels,
	}

	m.alerts[alertID] = alert
}

func severityToType(severity int) string {
	switch {
	case severity >= 9:
		return "critical"
	case severity >= 7:
		return "error"
	case severity >= 4:
		return "warning"
	default:
		return "info"
	}
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(req AcknowledgeAlertRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[req.AlertID]
	if !ok {
		return fmt.Errorf("告警 '%s' 不存在", req.AlertID)
	}

	alert.Status = "acknowledged"
	return m.save()
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(alertID, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("告警 '%s' 不存在", alertID)
	}

	now := time.Now()
	alert.Status = "resolved"
	alert.ResolvedAt = &now
	alert.ResolvedBy = user

	return m.save()
}

// ListAlerts 列出告警.
func (m *Manager) ListAlerts(status, category string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert
	for _, a := range m.alerts {
		if (status == "" || a.Status == status) &&
			(category == "" || a.Category == category) {
			result = append(result, a)
		}
	}
	return result
}

// GetAlert 获取告警.
func (m *Manager) GetAlert(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, fmt.Errorf("告警 '%s' 不存在", id)
	}
	return alert, nil
}

// AddHealthCheck 添加健康检查.
func (m *Manager) AddHealthCheck(check HealthCheck) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if check.Interval <= 0 {
		check.Interval = m.config.DefaultInterval
	}
	if check.Timeout <= 0 {
		check.Timeout = 10
	}
	check.Status = "unknown"

	m.healthChecks[check.ID] = &check
	return m.save()
}

// UpdateHealthCheckStatus 更新健康检查状态.
func (m *Manager) UpdateHealthCheckStatus(id, status string, latency int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	check, ok := m.healthChecks[id]
	if !ok {
		return
	}

	now := time.Now()
	check.Status = status
	check.LastCheck = &now
	check.Latency = latency
	check.Message = message
}

// GetHealthChecks 获取健康检查列表.
func (m *Manager) GetHealthChecks() []*HealthCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*HealthCheck
	for _, c := range m.healthChecks {
		result = append(result, c)
	}
	return result
}

// GetAlertStats 获取告警统计.
func (m *Manager) GetAlertStats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]int{
		"total":        0,
		"active":       0,
		"acknowledged": 0,
		"resolved":     0,
		"critical":     0,
		"error":        0,
		"warning":      0,
		"info":         0,
	}

	for _, a := range m.alerts {
		stats["total"]++
		stats[a.Status]++
		stats[a.Type]++
	}

	return stats
}

func (m *Manager) load() error {
	return nil
}

func (m *Manager) save() error {
	return nil
}

// RegisterHandlers 注册HTTP处理器.
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/monitor/alerts", m.handleAlerts)
	mux.HandleFunc("/api/v1/monitor/alerts/", m.handleAlertByID)
	mux.HandleFunc("/api/v1/monitor/rules", m.handleRules)
	mux.HandleFunc("/api/v1/monitor/rules/", m.handleRuleByID)
	mux.HandleFunc("/api/v1/monitor/health", m.handleHealth)
	mux.HandleFunc("/api/v1/monitor/metrics", m.handleMetrics)
	mux.HandleFunc("/api/v1/monitor/stats", m.handleStats)
}

func (m *Manager) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := r.URL.Query().Get("status")
		category := r.URL.Query().Get("category")
		alerts := m.ListAlerts(status, category)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/monitor/alerts/"):]
	if id == "" {
		http.Error(w, "Missing alert ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		alert, err := m.GetAlert(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alert)
	case http.MethodPut:
		action := r.URL.Query().Get("action")
		switch action {
		case "acknowledge":
			var req AcknowledgeAlertRequest
			json.NewDecoder(r.Body).Decode(&req)
			req.AlertID = id
			m.AcknowledgeAlert(req)
		case "resolve":
			user := r.URL.Query().Get("user")
			m.ResolveAlert(id, user)
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		rules := m.ListRules(category)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	case http.MethodPost:
		var req CreateRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		rule, err := m.CreateRule(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/monitor/rules/"):]
	if id == "" {
		http.Error(w, "Missing rule ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rule, err := m.GetRule(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	case http.MethodDelete:
		if err := m.DeleteRule(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	checks := m.GetHealthChecks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(checks)
}

func (m *Manager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metric Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	m.ReportMetric(metric)
	w.WriteHeader(http.StatusAccepted)
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := m.GetAlertStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
