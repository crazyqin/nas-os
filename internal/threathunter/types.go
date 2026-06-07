// Package threathunter 提供主动威胁猎手功能，包括威胁扫描、行为分析、威胁情报集成、事件响应和安全评分。
package threathunter

import "time"

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatCategory 威胁分类
type ThreatCategory string

const (
	CategoryMalware      ThreatCategory = "malware"
	CategoryIntrusion    ThreatCategory = "intrusion"
	CategoryDataLeak     ThreatCategory = "data_leak"
	CategoryBruteForce   ThreatCategory = "brute_force"
	CategoryPrivEsc      ThreatCategory = "privilege_escalation"
	CategorySuspicious   ThreatCategory = "suspicious_activity"
	CategoryConfigDrift  ThreatCategory = "config_drift"
	CategoryUnauthorized ThreatCategory = "unauthorized_access"
)

// ThreatStatus 威胁状态
type ThreatStatus string

const (
	StatusDetected      ThreatStatus = "detected"
	StatusInvestigating ThreatStatus = "investigating"
	StatusConfirmed     ThreatStatus = "confirmed"
	StatusMitigated     ThreatStatus = "mitigated"
	StatusResolved      ThreatStatus = "resolved"
	StatusFalsePositive ThreatStatus = "false_positive"
)

// IncidentSeverity 事件严重性
type IncidentSeverity string

const (
	SeverityInfo     IncidentSeverity = "info"
	SeverityWarning  IncidentSeverity = "warning"
	SeverityError    IncidentSeverity = "error"
	SeverityCritical IncidentSeverity = "critical"
)

// IncidentStatus 事件状态
type IncidentStatus string

const (
	IncidentStatusOpen       IncidentStatus = "open"
	IncidentStatusInProgress IncidentStatus = "in_progress"
	IncidentStatusContained  IncidentStatus = "contained"
	IncidentStatusEradicated IncidentStatus = "eradicated"
	IncidentStatusClosed     IncidentStatus = "closed"
)

// ResponseActionType 响应动作类型
type ResponseActionType string

const (
	ActionBlockIP     ResponseActionType = "block_ip"
	ActionDisableUser ResponseActionType = "disable_user"
	ActionQuarantine  ResponseActionType = "quarantine_file"
	ActionKillProcess ResponseActionType = "kill_process"
	ActionNotify      ResponseActionType = "notify"
	ActionLogAudit    ResponseActionType = "log_audit"
	ActionRateLimit   ResponseActionType = "rate_limit"
	ActionIsolateHost ResponseActionType = "isolate_host"
)

// Threat 威胁检测结果
type Threat struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Level       ThreatLevel    `json:"level"`
	Category    ThreatCategory `json:"category"`
	Status      ThreatStatus   `json:"status"`
	Source      string         `json:"source"`
	Target      string         `json:"target"`
	Indicators  []string       `json:"indicators,omitempty"`
	Evidence    []Evidence     `json:"evidence,omitempty"`
	Score       float64        `json:"score"`
	FirstSeen   time.Time      `json:"first_seen"`
	LastSeen    time.Time      `json:"last_seen"`
	DetectedAt  time.Time      `json:"detected_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// Evidence 威胁证据
type Evidence struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Context   string    `json:"context,omitempty"`
}

// BehaviorPattern 行为模式定义
type BehaviorPattern struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    ThreatCategory `json:"category"`
	Level       ThreatLevel    `json:"level"`
	Rules       []BehaviorRule `json:"rules"`
	Threshold   float64        `json:"threshold"`
	WindowSec   int            `json:"window_sec"`
	IsActive    bool           `json:"is_active"`
	HitCount    int64          `json:"hit_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// BehaviorRule 行为规则
type BehaviorRule struct {
	Field    string  `json:"field"`
	Operator string  `json:"operator"` // eq, ne, gt, lt, contains, regex
	Value    string  `json:"value"`
	Weight   float64 `json:"weight"`
}

// BehaviorEvent 行为事件
type BehaviorEvent struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id,omitempty"`
	HostIP    string                 `json:"host_ip,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RiskScore float64                `json:"risk_score"`
}

// BehaviorAnalysisResult 行为分析结果
type BehaviorAnalysisResult struct {
	Events       []*BehaviorEvent `json:"events"`
	MatchedRules []MatchedRule    `json:"matched_rules"`
	TotalScore   float64          `json:"total_score"`
	Anomalies    []Anomaly        `json:"anomalies"`
	AnalyzedAt   time.Time        `json:"analyzed_at"`
}

// MatchedRule 匹配的规则
type MatchedRule struct {
	PatternID   string  `json:"pattern_id"`
	PatternName string  `json:"pattern_name"`
	RuleIndex   int     `json:"rule_index"`
	Score       float64 `json:"score"`
	EventID     string  `json:"event_id"`
}

// Anomaly 异常行为
type Anomaly struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Severity    ThreatLevel `json:"severity"`
	Score       float64     `json:"score"`
	Timestamp   time.Time   `json:"timestamp"`
}

// ThreatIntel 威胁情报条目
type ThreatIntel struct {
	ID          string      `json:"id"`
	IOCType     string      `json:"ioc_type"` // ip, domain, hash, url, email
	IOCValue    string      `json:"ioc_value"`
	ThreatType  string      `json:"threat_type"`
	Severity    ThreatLevel `json:"severity"`
	Source      string      `json:"source"`
	Description string      `json:"description"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	ExpiryDate  *time.Time  `json:"expiry_date,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	IsActive    bool        `json:"is_active"`
}

// IntelFeed 威胁情报源
type IntelFeed struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	FeedType   string    `json:"feed_type"` // ip_list, domain_list, hash_list, mixed
	Enabled    bool      `json:"enabled"`
	LastSync   time.Time `json:"last_sync"`
	EntryCount int       `json:"entry_count"`
	Interval   int       `json:"interval_min"` // 同步间隔（分钟）
}

// Incident 安全事件
type Incident struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Severity    IncidentSeverity `json:"severity"`
	Status      IncidentStatus   `json:"status"`
	Threats     []string         `json:"threats,omitempty"` // 关联的威胁 ID
	Assignee    string           `json:"assignee,omitempty"`
	Actions     []ResponseAction `json:"actions,omitempty"`
	Timeline    []IncidentEvent  `json:"timeline,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	ResolvedAt  *time.Time       `json:"resolved_at,omitempty"`
}

// ResponseAction 响应动作
type ResponseAction struct {
	ID         string                 `json:"id"`
	Type       ResponseActionType     `json:"type"`
	Target     string                 `json:"target"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Status     string                 `json:"status"`
	Result     string                 `json:"result,omitempty"`
	ExecutedAt *time.Time             `json:"executed_at,omitempty"`
}

// IncidentEvent 事件时间线
type IncidentEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	Actor       string    `json:"actor"`
	EventType   string    `json:"event_type"`
}

// SecurityScore 安全评分
type SecurityScore struct {
	Overall         float64            `json:"overall"`   // 0-100
	Grade           string             `json:"grade"`     // A, B, C, D, F
	Breakdown       map[string]float64 `json:"breakdown"` // 各维度评分
	Trends          []ScoreTrend       `json:"trends,omitempty"`
	Recommendations []Recommendation   `json:"recommendations,omitempty"`
	ScoredAt        time.Time          `json:"scored_at"`
}

// ScoreTrend 评分趋势
type ScoreTrend struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
}

// Recommendation 安全建议
type Recommendation struct {
	ID          string      `json:"id"`
	Category    string      `json:"category"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Priority    ThreatLevel `json:"priority"`
	Impact      float64     `json:"impact"` // 对评分的潜在提升
}

// ScanRequest 扫描请求
type ScanRequest struct {
	ScanType   string           `json:"scan_type"` // quick, full, targeted
	Targets    []string         `json:"targets,omitempty"`
	Categories []ThreatCategory `json:"categories,omitempty"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID           string    `json:"id"`
	ScanType     string    `json:"scan_type"`
	Threats      []*Threat `json:"threats"`
	TotalScanned int       `json:"total_scanned"`
	ThreatCount  int       `json:"threat_count"`
	Duration     string    `json:"duration"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}

// IncidentRequest 创建事件请求
type IncidentRequest struct {
	Title       string           `json:"title" binding:"required"`
	Description string           `json:"description"`
	Severity    IncidentSeverity `json:"severity" binding:"required"`
	Threats     []string         `json:"threats,omitempty"`
	Assignee    string           `json:"assignee,omitempty"`
}

// ThreatHunterConfig 威胁猎手配置
type ThreatHunterConfig struct {
	Enabled            bool               `json:"enabled"`
	ScanIntervalMin    int                `json:"scan_interval_min"`
	AutoResponse       bool               `json:"auto_response"`
	ScoreWeights       map[string]float64 `json:"score_weights"`
	MaxThreatHistory   int                `json:"max_threat_history"`
	MaxIncidentHistory int                `json:"max_incident_history"`
	AlertThreshold     float64            `json:"alert_threshold"`
}

// DefaultThreatHunterConfig 默认配置
func DefaultThreatHunterConfig() *ThreatHunterConfig {
	return &ThreatHunterConfig{
		Enabled:         true,
		ScanIntervalMin: 30,
		AutoResponse:    true,
		ScoreWeights: map[string]float64{
			"threat_count":   0.3,
			"threat_level":   0.25,
			"behavior_score": 0.2,
			"intel_coverage": 0.15,
			"response_time":  0.1,
		},
		MaxThreatHistory:   10000,
		MaxIncidentHistory: 5000,
		AlertThreshold:     0.7,
	}
}

// String 等级转字符串
func (l ThreatLevel) String() string {
	return string(l)
}

// IsValidThreatLevel 校验威胁等级
func IsValidThreatLevel(level ThreatLevel) bool {
	switch level {
	case ThreatLevelLow, ThreatLevelMedium, ThreatLevelHigh, ThreatLevelCritical:
		return true
	}
	return false
}
