package unifiedhealthhub

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// HealthHub 统一健康中心
type HealthHub struct {
	mu         sync.RWMutex
	subsystems map[string]*Subsystem
	alerts     map[string]*HealthAlert
	incidents  map[string]*Incident
	rules      map[string]*HealthRule
	history    []*HealthSnapshot
	analyzer   *HealthAnalyzer
	metrics    *HealthMetrics
	config     *HubConfig
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// Subsystem 子系统
type Subsystem struct {
	ID        string
	Name      string
	Type      SubsystemType
	Status    HealthStatus
	Score     float64 // 0-100
	Checks    []*HealthCheck
	LastCheck time.Time
	Uptime    time.Duration
	Metadata  map[string]interface{}
}

// HealthCheck 健康检查
type HealthCheck struct {
	ID        string
	Name      string
	Type      CheckType
	Status    CheckStatus
	Message   string
	Value     interface{}
	Threshold interface{}
	Duration  time.Duration
	LastRun   time.Time
	NextRun   time.Time
}

// HealthAlert 健康告警
type HealthAlert struct {
	ID         string
	Subsystem  string
	Level      AlertLevel
	Title      string
	Message    string
	Source     string
	Acked      bool
	AckedBy    string
	AckedAt    time.Time
	Resolved   bool
	ResolvedAt time.Time
	CreatedAt  time.Time
}

// Incident 事件
type Incident struct {
	ID         string
	Title      string
	Severity   IncidentSeverity
	Status     IncidentStatus
	Subsystems []string
	Alerts     []string
	Timeline   []*IncidentEvent
	RootCause  string
	Resolution string
	CreatedAt  time.Time
	ResolvedAt time.Time
}

// IncidentEvent 事件时间线
type IncidentEvent struct {
	Timestamp time.Time
	Type      string
	Message   string
	Actor     string
}

// HealthRule 健康规则
type HealthRule struct {
	ID            string
	Name          string
	Condition     *RuleCondition
	Actions       []*RuleAction
	Priority      int
	Enabled       bool
	LastTriggered time.Time
}

// RuleCondition 规则条件
type RuleCondition struct {
	Metric    string
	Operator  string
	Threshold float64
	Duration  time.Duration
}

// RuleAction 规则动作
type RuleAction struct {
	Type       string
	Parameters map[string]interface{}
}

// HealthSnapshot 健康快照
type HealthSnapshot struct {
	Timestamp     time.Time
	Overall       float64
	Subsystems    map[string]float64
	AlertCount    int
	IncidentCount int
}

// HealthAnalyzer 健康分析器
type HealthAnalyzer struct {
	mu           sync.RWMutex
	patterns     map[string]*HealthPattern
	predictions  map[string]*HealthPrediction
	correlations []*Correlation
	accuracy     float64
}

// HealthPattern 健康模式
type HealthPattern struct {
	ID        string
	Subsystem string
	Pattern   string
	Frequency time.Duration
	Severity  AlertLevel
	LastSeen  time.Time
}

// HealthPrediction 健康预测
type HealthPrediction struct {
	Subsystem   string
	Predicted   HealthStatus
	Probability float64
	TimeWindow  time.Duration
	Factors     []string
	CreatedAt   time.Time
}

// Correlation 相关性
type Correlation struct {
	Source   string
	Target   string
	Strength float64
	Type     string
}

// HealthMetrics 健康指标
type HealthMetrics struct {
	OverallScore       float64
	Availability       float64
	MTBF               time.Duration // 平均故障间隔
	MTTR               time.Duration // 平均修复时间
	AlertCount         int
	IncidentCount      int
	ResolvedCount      int
	PredictionAccuracy float64
}

// HubConfig 中心配置
type HubConfig struct {
	CheckInterval     time.Duration
	AlertCooldown     time.Duration
	AutoIncident      bool
	PredictionEnabled bool
	RetentionDays     int
}

// 枚举类型
type SubsystemType int

const (
	SubsystemStorage SubsystemType = iota
	SubsystemNetwork
	SubsystemCompute
	SubsystemMemory
	SubsystemContainer
	SubsystemService
	SubsystemSecurity
)

type HealthStatus int

const (
	HealthHealthy HealthStatus = iota
	HealthDegraded
	HealthUnhealthy
	HealthCritical
	HealthUnknown
)

type CheckType int

const (
	CheckTypeHeartbeat CheckType = iota
	CheckTypeMetric
	CheckTypeLog
	CheckTypeProbe
	CheckTypeSynthetic
)

type CheckStatus int

const (
	CheckPass CheckStatus = iota
	CheckWarn
	CheckFail
	CheckSkip
)

type AlertLevel int

const (
	AlertInfo AlertLevel = iota
	AlertWarning
	AlertCritical
	AlertEmergency
)

type IncidentSeverity int

const (
	SeverityLow IncidentSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

type IncidentStatus int

const (
	IncidentOpen IncidentStatus = iota
	IncidentInvestigating
	IncidentMitigated
	IncidentResolved
	IncidentClosed
)

// ID generator helper
var idCounter uint64
var idMu sync.Mutex

func generateID(prefix string) string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), idCounter)
}

// NewHealthHub 创建健康中心
func NewHealthHub(config *HubConfig, logger *slog.Logger) *HealthHub {
	if config == nil {
		config = &HubConfig{
			CheckInterval:     30 * time.Second,
			AlertCooldown:     5 * time.Minute,
			AutoIncident:      true,
			PredictionEnabled: true,
			RetentionDays:     30,
		}
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	hub := &HealthHub{
		subsystems: make(map[string]*Subsystem),
		alerts:     make(map[string]*HealthAlert),
		incidents:  make(map[string]*Incident),
		rules:      make(map[string]*HealthRule),
		history:    make([]*HealthSnapshot, 0),
		analyzer: &HealthAnalyzer{
			patterns:     make(map[string]*HealthPattern),
			predictions:  make(map[string]*HealthPrediction),
			correlations: make([]*Correlation, 0),
			accuracy:     0.0,
		},
		metrics: &HealthMetrics{},
		config:  config,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	logger.Info("HealthHub initialized",
		"checkInterval", config.CheckInterval,
		"predictionEnabled", config.PredictionEnabled)

	return hub
}

// RegisterSubsystem 注册子系统
func (h *HealthHub) RegisterSubsystem(subsystem *Subsystem) error {
	if subsystem == nil {
		return fmt.Errorf("subsystem cannot be nil")
	}
	if subsystem.ID == "" {
		subsystem.ID = generateID("sub")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.subsystems[subsystem.ID]; exists {
		return ErrSubsystemExists
	}

	if subsystem.Status == 0 && subsystem.Score == 0 {
		subsystem.Status = HealthUnknown
		subsystem.Score = 100.0
	}
	if subsystem.Metadata == nil {
		subsystem.Metadata = make(map[string]interface{})
	}

	h.subsystems[subsystem.ID] = subsystem

	h.logger.Info("Subsystem registered",
		"id", subsystem.ID,
		"name", subsystem.Name,
		"type", subsystem.Type)

	return nil
}

// RunHealthCheck 执行健康检查
func (h *HealthHub) RunHealthCheck(subsystemID string, check *HealthCheck) (*HealthCheck, error) {
	h.mu.Lock()
	subsystem, exists := h.subsystems[subsystemID]
	if !exists {
		h.mu.Unlock()
		return nil, ErrSubsystemNotFound
	}
	h.mu.Unlock()

	if check == nil {
		return nil, fmt.Errorf("health check cannot be nil")
	}

	if check.ID == "" {
		check.ID = generateID("chk")
	}

	start := time.Now()

	// Simulate running the check
	check.LastRun = start
	check.NextRun = start.Add(h.config.CheckInterval)
	check.Duration = time.Since(start)

	// Update subsystem
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find or add check to subsystem
	found := false
	for i, existing := range subsystem.Checks {
		if existing.ID == check.ID {
			subsystem.Checks[i] = check
			found = true
			break
		}
	}
	if !found {
		subsystem.Checks = append(subsystem.Checks, check)
	}

	subsystem.LastCheck = time.Now()

	// Recalculate subsystem score based on checks
	subsystem.Score, subsystem.Status = h.calculateSubsystemHealth(subsystem)

	h.logger.Debug("Health check completed",
		"subsystem", subsystemID,
		"check", check.ID,
		"status", check.Status,
		"duration", check.Duration)

	return check, nil
}

// calculateSubsystemHealth 计算子系统健康度
func (h *HealthHub) calculateSubsystemHealth(subsystem *Subsystem) (float64, HealthStatus) {
	if len(subsystem.Checks) == 0 {
		return 100.0, HealthUnknown
	}

	passCount := 0
	warnCount := 0
	failCount := 0

	for _, check := range subsystem.Checks {
		switch check.Status {
		case CheckPass:
			passCount++
		case CheckWarn:
			warnCount++
		case CheckFail:
			failCount++
		}
	}

	total := len(subsystem.Checks)
	score := float64(passCount)/float64(total)*100.0 - float64(warnCount)*10.0 - float64(failCount)*25.0
	score = math.Max(0, math.Min(100, score))

	var status HealthStatus
	switch {
	case score >= 90:
		status = HealthHealthy
	case score >= 70:
		status = HealthDegraded
	case score >= 40:
		status = HealthUnhealthy
	default:
		status = HealthCritical
	}

	return score, status
}

// RaiseAlert 发起告警
func (h *HealthHub) RaiseAlert(alert *HealthAlert) (*HealthAlert, error) {
	if alert == nil {
		return nil, fmt.Errorf("alert cannot be nil")
	}

	if alert.ID == "" {
		alert.ID = generateID("alert")
	}
	alert.CreatedAt = time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Verify subsystem exists if specified
	if alert.Subsystem != "" {
		if _, exists := h.subsystems[alert.Subsystem]; !exists {
			return nil, ErrSubsystemNotFound
		}
	}

	h.alerts[alert.ID] = alert

	h.logger.Warn("Alert raised",
		"id", alert.ID,
		"level", alert.Level,
		"title", alert.Title,
		"subsystem", alert.Subsystem)

	// Auto-create incident if enabled and alert is critical or emergency
	if h.config.AutoIncident && (alert.Level == AlertCritical || alert.Level == AlertEmergency) {
		incident := &Incident{
			ID:       generateID("inc"),
			Title:    fmt.Sprintf("Auto-incident for alert: %s", alert.Title),
			Severity: h.alertLevelToIncidentSeverity(alert.Level),
			Status:   IncidentOpen,
			Alerts:   []string{alert.ID},
			Timeline: []*IncidentEvent{
				{
					Timestamp: time.Now(),
					Type:      "created",
					Message:   fmt.Sprintf("Auto-created from alert %s", alert.ID),
					Actor:     "system",
				},
			},
			CreatedAt: time.Now(),
		}
		if alert.Subsystem != "" {
			incident.Subsystems = []string{alert.Subsystem}
		}
		h.incidents[incident.ID] = incident

		h.logger.Warn("Auto-incident created",
			"incidentID", incident.ID,
			"alertID", alert.ID)
	}

	return alert, nil
}

// alertLevelToIncidentSeverity converts alert level to incident severity
func (h *HealthHub) alertLevelToIncidentSeverity(level AlertLevel) IncidentSeverity {
	switch level {
	case AlertInfo:
		return SeverityLow
	case AlertWarning:
		return SeverityMedium
	case AlertCritical:
		return SeverityHigh
	case AlertEmergency:
		return SeverityCritical
	default:
		return SeverityLow
	}
}

// CreateIncident 创建事件
func (h *HealthHub) CreateIncident(incident *Incident) (*Incident, error) {
	if incident == nil {
		return nil, fmt.Errorf("incident cannot be nil")
	}

	if incident.ID == "" {
		incident.ID = generateID("inc")
	}
	incident.CreatedAt = time.Now()
	if incident.Status == 0 {
		incident.Status = IncidentOpen
	}
	if incident.Timeline == nil {
		incident.Timeline = make([]*IncidentEvent, 0)
	}

	// Add creation event
	incident.Timeline = append(incident.Timeline, &IncidentEvent{
		Timestamp: time.Now(),
		Type:      "created",
		Message:   "Incident created",
		Actor:     "user",
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	// Verify subsystems exist
	for _, subID := range incident.Subsystems {
		if _, exists := h.subsystems[subID]; !exists {
			return nil, ErrSubsystemNotFound
		}
	}

	// Verify alerts exist
	for _, alertID := range incident.Alerts {
		if _, exists := h.alerts[alertID]; !exists {
			return nil, ErrAlertNotFound
		}
	}

	h.incidents[incident.ID] = incident

	h.logger.Info("Incident created",
		"id", incident.ID,
		"title", incident.Title,
		"severity", incident.Severity)

	return incident, nil
}

// AcknowledgeAlert 确认告警
func (h *HealthHub) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	alert, exists := h.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	if alert.Acked {
		return ErrAlertAlreadyAcked
	}

	alert.Acked = true
	alert.AckedBy = acknowledgedBy
	alert.AckedAt = time.Now()

	h.logger.Info("Alert acknowledged",
		"id", alertID,
		"by", acknowledgedBy)

	return nil
}

// ResolveIncident 解决事件
func (h *HealthHub) ResolveIncident(incidentID, resolution string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	incident, exists := h.incidents[incidentID]
	if !exists {
		return ErrIncidentNotFound
	}

	if incident.Status == IncidentClosed {
		return ErrIncidentAlreadyClosed
	}

	incident.Status = IncidentResolved
	incident.Resolution = resolution
	incident.ResolvedAt = time.Now()

	// Add resolution event
	incident.Timeline = append(incident.Timeline, &IncidentEvent{
		Timestamp: time.Now(),
		Type:      "resolved",
		Message:   resolution,
		Actor:     "user",
	})

	h.logger.Info("Incident resolved",
		"id", incidentID,
		"resolution", resolution)

	return nil
}

// GetOverallHealth 获取整体健康状态
func (h *HealthHub) GetOverallHealth() (*HealthSnapshot, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.subsystems) == 0 {
		return &HealthSnapshot{
			Timestamp:     time.Now(),
			Overall:       100.0,
			Subsystems:    make(map[string]float64),
			AlertCount:    0,
			IncidentCount: 0,
		}, nil
	}

	totalScore := 0.0
	subsystemScores := make(map[string]float64)

	for id, sub := range h.subsystems {
		totalScore += sub.Score
		subsystemScores[id] = sub.Score
	}

	overallScore := totalScore / float64(len(h.subsystems))

	// Count active alerts and incidents
	activeAlerts := 0
	for _, alert := range h.alerts {
		if !alert.Resolved {
			activeAlerts++
		}
	}

	activeIncidents := 0
	for _, inc := range h.incidents {
		if inc.Status != IncidentResolved && inc.Status != IncidentClosed {
			activeIncidents++
		}
	}

	snapshot := &HealthSnapshot{
		Timestamp:     time.Now(),
		Overall:       overallScore,
		Subsystems:    subsystemScores,
		AlertCount:    activeAlerts,
		IncidentCount: activeIncidents,
	}

	// Store in history
	h.history = append(h.history, snapshot)

	// Update metrics
	h.metrics.OverallScore = overallScore
	h.metrics.AlertCount = activeAlerts
	h.metrics.IncidentCount = activeIncidents

	return snapshot, nil
}

// PredictHealth 预测健康趋势
func (h *HealthHub) PredictHealth(subsystemID string, window time.Duration) (*HealthPrediction, error) {
	if !h.config.PredictionEnabled {
		return nil, ErrPredictionDisabled
	}

	h.mu.RLock()
	subsystem, exists := h.subsystems[subsystemID]
	if !exists {
		h.mu.RUnlock()
		return nil, ErrSubsystemNotFound
	}
	h.mu.RUnlock()

	// Simple prediction based on current health and history
	prediction := &HealthPrediction{
		Subsystem:   subsystemID,
		Predicted:   subsystem.Status,
		Probability: 0.85, // Base probability
		TimeWindow:  window,
		Factors:     make([]string, 0),
		CreatedAt:   time.Now(),
	}

	// Adjust prediction based on check history
	if len(subsystem.Checks) > 0 {
		failCount := 0
		for _, check := range subsystem.Checks {
			if check.Status == CheckFail {
				failCount++
			}
		}
		failRate := float64(failCount) / float64(len(subsystem.Checks))

		if failRate > 0.3 {
			prediction.Predicted = HealthUnhealthy
			prediction.Probability = 0.7
			prediction.Factors = append(prediction.Factors, "high failure rate")
		} else if failRate > 0.1 {
			prediction.Predicted = HealthDegraded
			prediction.Probability = 0.6
			prediction.Factors = append(prediction.Factors, "moderate failure rate")
		}
	}

	// Check for patterns
	h.analyzer.mu.RLock()
	for _, pattern := range h.analyzer.patterns {
		if pattern.Subsystem == subsystemID {
			prediction.Factors = append(prediction.Factors, fmt.Sprintf("pattern: %s", pattern.Pattern))
		}
	}
	h.analyzer.mu.RUnlock()

	// Store prediction
	h.analyzer.mu.Lock()
	h.analyzer.predictions[subsystemID] = prediction
	h.analyzer.mu.Unlock()

	h.logger.Info("Health prediction generated",
		"subsystem", subsystemID,
		"predicted", prediction.Predicted,
		"probability", prediction.Probability)

	return prediction, nil
}

// GetMetrics 获取指标
func (h *HealthHub) GetMetrics() *HealthMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()

	metrics := *h.metrics

	// Calculate availability
	totalChecks := 0
	passedChecks := 0
	for _, sub := range h.subsystems {
		for _, check := range sub.Checks {
			totalChecks++
			if check.Status == CheckPass {
				passedChecks++
			}
		}
	}

	if totalChecks > 0 {
		metrics.Availability = float64(passedChecks) / float64(totalChecks) * 100.0
	} else {
		metrics.Availability = 100.0
	}

	// Count resolved incidents
	resolved := 0
	for _, inc := range h.incidents {
		if inc.Status == IncidentResolved || inc.Status == IncidentClosed {
			resolved++
		}
	}
	metrics.ResolvedCount = resolved

	// Prediction accuracy
	metrics.PredictionAccuracy = h.analyzer.accuracy

	return &metrics
}

// AddHealthRule 添加健康规则
func (h *HealthHub) AddHealthRule(rule *HealthRule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	if rule.ID == "" {
		rule.ID = generateID("rule")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.rules[rule.ID] = rule

	h.logger.Info("Health rule added",
		"id", rule.ID,
		"name", rule.Name)

	return nil
}

// Stop 停止健康中心
func (h *HealthHub) Stop() {
	h.cancel()
	h.logger.Info("HealthHub stopped")
}
