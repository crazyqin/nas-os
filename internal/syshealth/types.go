// Package syshealth 提供统一系统健康仪表盘功能，
// 整合所有子系统健康状态，提供一目了然的系统概览。
package syshealth

import (
	"time"
)

// ========== 系统状态 ==========

// SystemStatus 系统整体状态。
type SystemStatus string

const (
	StatusHealthy  SystemStatus = "healthy"
	StatusWarning  SystemStatus = "warning"
	StatusCritical SystemStatus = "critical"
	StatusUnknown  SystemStatus = "unknown"
)

// ========== 健康等级 ==========

// HealthLevel 健康等级。
type HealthLevel string

const (
	LevelExcellent HealthLevel = "excellent" // 优秀 90-100
	LevelGood      HealthLevel = "good"      // 良好 70-89
	LevelFair      HealthLevel = "fair"      // 一般 50-69
	LevelPoor      HealthLevel = "poor"      // 差 30-49
	LevelCritical  HealthLevel = "critical"  // 严重 0-29
)

// ClassifyLevel 根据分数返回等级。
func ClassifyLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return LevelExcellent
	case score >= 70:
		return LevelGood
	case score >= 50:
		return LevelFair
	case score >= 30:
		return LevelPoor
	default:
		return LevelCritical
	}
}

// ClassifyStatus 根据分数返回系统状态。
func ClassifyStatus(score float64) SystemStatus {
	switch {
	case score >= 70:
		return StatusHealthy
	case score >= 50:
		return StatusWarning
	default:
		return StatusCritical
	}
}

// ========== 总览面板 ==========

// SystemOverview 系统总览。
type SystemOverview struct {
	// OverallScore 综合健康评分 0-100。
	OverallScore float64 `json:"overall_score"`
	// Level 健康等级。
	Level HealthLevel `json:"level"`
	// Status 系统状态：healthy / warning / critical。
	Status SystemStatus `json:"status"`
	// Subsystems 子系统状态列表。
	Subsystems []SubsystemStatus `json:"subsystems"`
	// Metrics 核心指标。
	Metrics CoreMetrics `json:"metrics"`
	// ActiveAlerts 活跃告警数量。
	ActiveAlerts int `json:"active_alerts"`
	// Recommendations 健康建议。
	Recommendations []Recommendation `json:"recommendations,omitempty"`
	// EvaluatedAt 评估时间。
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// CoreMetrics 核心系统指标。
type CoreMetrics struct {
	// CPU CPU 使用率 0-1。
	CPU float64 `json:"cpu"`
	// Memory 内存使用率 0-1。
	Memory float64 `json:"memory"`
	// Disk 磁盘使用率 0-1。
	Disk float64 `json:"disk"`
	// Network 网络吞吐量 (bytes/s)。
	Network int64 `json:"network"`
	// Temperature 温度 (℃)。
	Temperature float64 `json:"temperature"`
	// Uptime 运行时间（秒）。
	Uptime int64 `json:"uptime"`
	// LoadAverage 系统负载（1分钟）。
	LoadAverage float64 `json:"load_average"`
}

// ========== 子系统状态 ==========

// SubsystemStatus 子系统状态。
type SubsystemStatus struct {
	// Name 子系统名称。
	Name string `json:"name"`
	// Type 子系统类型：storage / raid / smart / service / container / vm / network / security。
	Type string `json:"type"`
	// Status 状态：healthy / warning / critical / unknown。
	Status SystemStatus `json:"status"`
	// Score 评分 0-100。
	Score float64 `json:"score"`
	// Message 状态描述。
	Message string `json:"message"`
	// Details 详细信息。
	Details map[string]interface{} `json:"details,omitempty"`
	// LastCheck 最后检查时间。
	LastCheck time.Time `json:"last_check"`
}

// ========== 趋势分析 ==========

// HealthTrend 健康趋势数据。
type HealthTrend struct {
	// Timestamp 时间点。
	Timestamp time.Time `json:"timestamp"`
	// Score 健康评分。
	Score float64 `json:"score"`
	// Level 健康等级。
	Level HealthLevel `json:"level"`
	// Status 系统状态。
	Status SystemStatus `json:"status"`
	// SubsystemScores 各子系统评分。
	SubsystemScores map[string]float64 `json:"subsystem_scores"`
}

// TrendAnalysis 趋势分析结果。
type TrendAnalysis struct {
	// Period 分析周期（天）。
	Period int `json:"period"`
	// Trends 趋势数据点。
	Trends []HealthTrend `json:"trends"`
	// AverageScore 平均评分。
	AverageScore float64 `json:"average_score"`
	// MinScore 最低评分。
	MinScore float64 `json:"min_score"`
	// MaxScore 最高评分。
	MaxScore float64 `json:"max_score"`
	// Trend 趋势方向：rising / falling / stable。
	Trend string `json:"trend"`
	// Prediction 预测结果。
	Prediction *Prediction `json:"prediction,omitempty"`
}

// Prediction 预测结果。
type Prediction struct {
	// PredictedScore 预测评分。
	PredictedScore float64 `json:"predicted_score"`
	// Confidence 置信度 0-1。
	Confidence float64 `json:"confidence"`
	// PredictedAt 预测时间。
	PredictedAt time.Time `json:"predicted_at"`
	// RiskLevel 风险等级：low / medium / high / critical。
	RiskLevel string `json:"risk_level"`
}

// ========== 告警 ==========

// Alert 系统告警。
type Alert struct {
	// ID 告警 ID。
	ID string `json:"id"`
	// Level 告警级别：info / warning / critical。
	Level string `json:"level"`
	// Source 来源子系统。
	Source string `json:"source"`
	// Message 告警消息。
	Message string `json:"message"`
	// Details 详细信息。
	Details map[string]interface{} `json:"details,omitempty"`
	// Time 告警时间。
	Time time.Time `json:"time"`
	// Resolved 是否已解决。
	Resolved bool `json:"resolved"`
	// ResolvedAt 解决时间。
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ========== 快速修复 ==========

// FixAction 修复动作。
type FixAction struct {
	// ID 动作 ID。
	ID string `json:"id"`
	// Name 动作名称。
	Name string `json:"name"`
	// Description 动作描述。
	Description string `json:"description"`
	// Category 类别：storage / service / cache / network。
	Category string `json:"category"`
	// Risk 风险等级：low / medium / high。
	Risk string `json:"risk"`
	// RequiresConfirm 是否需要确认。
	RequiresConfirm bool `json:"requires_confirm"`
}

// FixResult 修复结果。
type FixResult struct {
	// ActionID 执行的动作 ID。
	ActionID string `json:"action_id"`
	// Success 是否成功。
	Success bool `json:"success"`
	// Message 结果消息。
	Message string `json:"message"`
	// Details 详细信息。
	Details map[string]interface{} `json:"details,omitempty"`
	// ExecutedAt 执行时间。
	ExecutedAt time.Time `json:"executed_at"`
}

// ========== 健康建议 ==========

// Recommendation 健康建议。
type Recommendation struct {
	// ID 建议 ID。
	ID string `json:"id"`
	// Category 类别：storage / service / performance / security。
	Category string `json:"category"`
	// Severity 严重程度：high / medium / low。
	Severity string `json:"severity"`
	// Title 建议标题。
	Title string `json:"title"`
	// Description 详细描述。
	Description string `json:"description"`
	// Action 建议操作。
	Action string `json:"action"`
	// RelatedSubsystem 相关子系统。
	RelatedSubsystem string `json:"related_subsystem,omitempty"`
}

// ========== 子系统检查器接口 ==========

// SubsystemChecker 子系统健康检查器接口。
type SubsystemChecker interface {
	// Name 返回子系统名称。
	Name() string
	// Type 返回子系统类型。
	Type() string
	// Check 执行健康检查。
	Check() SubsystemStatus
}

// ========== 数据提供函数类型 ==========

// MetricsProvider 核心指标数据提供函数类型。
type MetricsProvider func() (CoreMetrics, error)

// ========== 历史记录 ==========

// HealthRecord 健康历史记录。
type HealthRecord struct {
	// Timestamp 时间戳。
	Timestamp time.Time `json:"timestamp"`
	// OverallScore 综合评分。
	OverallScore float64 `json:"overall_score"`
	// Level 健康等级。
	Level HealthLevel `json:"level"`
	// Status 系统状态。
	Status SystemStatus `json:"status"`
	// Subsystems 子系统状态快照。
	Subsystems []SubsystemStatus `json:"subsystems"`
}

// ========== API 响应 ==========

// OverviewResponse 总览响应。
type OverviewResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    SystemOverview `json:"data"`
}

// TrendsResponse 趋势响应。
type TrendsResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    TrendAnalysis `json:"data"`
}

// FixRequest 修复请求。
type FixRequest struct {
	// Issue 问题类型。
	Issue string `json:"issue"`
	// Confirm 是否确认执行。
	Confirm bool `json:"confirm"`
	// Params 额外参数。
	Params map[string]interface{} `json:"params,omitempty"`
}

// FixResponse 修复响应。
type FixResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    FixResult `json:"data"`
}

// AvailableFixesResponse 可用修复列表响应。
type AvailableFixesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []FixAction `json:"data"`
}
