// Package ailoganalyzer 提供 AI 驱动的日志分析功能
package ailoganalyzer

import (
	"errors"
	"time"
)

// 日志级别常量.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

// 告警级别常量.
const (
	AlertLevelLow      = "low"
	AlertLevelMedium   = "medium"
	AlertLevelHigh     = "high"
	AlertLevelCritical = "critical"
)

// 告警状态常量.
const (
	AlertStatusActive    = "active"
	AlertStatusResolved  = "resolved"
	AlertStatusSilenced  = "silenced"
	AlertStatusEscalated = "escalated"
)

// 错误定义.
var (
	// ErrLogNotFound 日志不存在.
	ErrLogNotFound = errors.New("log not found")
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("rule not found")
	// ErrAlertNotFound 告警不存在.
	ErrAlertNotFound = errors.New("alert not found")
	// ErrStreamNotFound 流不存在.
	ErrStreamNotFound = errors.New("stream not found")
	// ErrInvalidQuery 无效查询.
	ErrInvalidQuery = errors.New("invalid query")
	// ErrStreamAlreadyRunning 流已在运行.
	ErrStreamAlreadyRunning = errors.New("stream already running")
)

// LogEntry 日志条目.
type LogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"` // 来源：system, app, docker 等
	Message   string                 `json:"message"`
	Labels    map[string]string      `json:"labels"`     // 标签
	Metadata  map[string]interface{} `json:"metadata"`   // 扩展元数据
	PatternID string                 `json:"pattern_id"` // 匹配的模式ID
	ClusterID string                 `json:"cluster_id"` // 所属聚类ID
	RuleID    string                 `json:"rule_id"`    // 触发的规则ID
}

// LogPattern 日志模式.
type LogPattern struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Regex       string    `json:"regex"`      // 正则表达式
	Keywords    []string  `json:"keywords"`   // 关键词列表
	Level       string    `json:"level"`      // 匹配的日志级别
	IsAnomaly   bool      `json:"is_anomaly"` // 是否标记为异常模式
	Severity    string    `json:"severity"`   // 严重程度
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AnomalyRule 异常检测规则.
type AnomalyRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        string        `json:"type"`       // frequency, pattern, time
	Threshold   int           `json:"threshold"`  // 阈值
	Window      time.Duration `json:"window"`     // 检测窗口
	Level       string        `json:"level"`      // 匹配的日志级别
	PatternID   string        `json:"pattern_id"` // 关联的模式ID
	TimeStart   string        `json:"time_start"` // 异常时间段开始 (HH:MM)
	TimeEnd     string        `json:"time_end"`   // 异常时间段结束 (HH:MM)
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Alert 告警.
type Alert struct {
	ID         string     `json:"id"`
	RuleID     string     `json:"rule_id"`
	RuleName   string     `json:"rule_name"`
	Level      string     `json:"level"`  // low, medium, high, critical
	Status     string     `json:"status"` // active, resolved, silenced, escalated
	Message    string     `json:"message"`
	LogIDs     []string   `json:"log_ids"` // 关联的日志ID
	Count      int        `json:"count"`   // 触发次数
	FirstSeen  time.Time  `json:"first_seen"`
	LastSeen   time.Time  `json:"last_seen"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Notes      string     `json:"notes,omitempty"`
}

// LogCluster 日志聚类.
type LogCluster struct {
	ID        string    `json:"id"`
	Pattern   string    `json:"pattern"` // 聚类模式（模板化消息）
	Count     int       `json:"count"`   // 日志数量
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	SampleIDs []string  `json:"sample_ids"` // 样本日志ID
	Level     string    `json:"level"`
	Source    string    `json:"source"`
}

// LogStream 日志流配置.
type LogStream struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"` // 文件路径或日志源
	Enabled   bool      `json:"enabled"`
	Running   bool      `json:"running"`
	CreatedAt time.Time `json:"created_at"`
}

// RootCauseAnalysis 根因分析结果.
type RootCauseAnalysis struct {
	ID          string          `json:"id"`
	AlertID     string          `json:"alert_id"`
	RootCause   string          `json:"root_cause"`
	Timeline    []TimelineEntry `json:"timeline"`     // 时间线
	RelatedLogs []string        `json:"related_logs"` // 关联日志ID
	Suggestions []string        `json:"suggestions"`  // 建议
	CreatedAt   time.Time       `json:"created_at"`
}

// TimelineEntry 时间线条目.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	LogID     string    `json:"log_id,omitempty"`
	Level     string    `json:"level"`
}

// RetentionPolicy 日志保留策略.
type RetentionPolicy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Level       string    `json:"level"`        // 应用的日志级别，空表示全部
	Source      string    `json:"source"`       // 应用的日志源，空表示全部
	MaxAge      int       `json:"max_age_days"` // 最大保留天数
	MaxCount    int       `json:"max_count"`    // 最大保留数量
	ArchivePath string    `json:"archive_path"` // 归档路径
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// LogStats 日志统计.
type LogStats struct {
	TotalLogs    int            `json:"total_logs"`
	LogsByLevel  map[string]int `json:"logs_by_level"`
	LogsBySource map[string]int `json:"logs_by_source"`
	ErrorRate    float64        `json:"error_rate"`
	TopErrors    []PatternStat  `json:"top_errors"`
	TrendData    []TrendPoint   `json:"trend_data"`
}

// PatternStat 模式统计.
type PatternStat struct {
	PatternID   string    `json:"pattern_id"`
	PatternName string    `json:"pattern_name"`
	Count       int       `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
	Errors    int       `json:"errors"`
}

// ========== 请求/响应类型 ==========

// QueryLogsRequest 查询日志请求.
type QueryLogsRequest struct {
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
	Level     string     `form:"level"`
	Source    string     `form:"source"`
	Keyword   string     `form:"keyword"`
	Regex     string     `form:"regex"`
	PatternID string     `form:"pattern_id"`
	ClusterID string     `form:"cluster_id"`
	Page      int        `form:"page"`
	PageSize  int        `form:"page_size"`
}

// CreatePatternRequest 创建模式请求.
type CreatePatternRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description,omitempty"`
	Regex       string   `json:"regex"`
	Keywords    []string `json:"keywords"`
	Level       string   `json:"level"`
	IsAnomaly   bool     `json:"is_anomaly"`
	Severity    string   `json:"severity"`
}

// UpdatePatternRequest 更新模式请求.
type UpdatePatternRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Regex       *string  `json:"regex,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Level       *string  `json:"level,omitempty"`
	IsAnomaly   *bool    `json:"is_anomaly,omitempty"`
	Severity    *string  `json:"severity,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
}

// CreateRuleRequest 创建异常检测规则请求.
type CreateRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type" binding:"required"`
	Threshold   int    `json:"threshold"`
	Window      int    `json:"window_seconds"`
	Level       string `json:"level"`
	PatternID   string `json:"pattern_id"`
	TimeStart   string `json:"time_start"`
	TimeEnd     string `json:"time_end"`
}

// UpdateRuleRequest 更新异常检测规则请求.
type UpdateRuleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Threshold   *int    `json:"threshold,omitempty"`
	Window      *int    `json:"window_seconds,omitempty"`
	Level       *string `json:"level,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// UpdateAlertRequest 更新告警请求.
type UpdateAlertRequest struct {
	Status *string `json:"status,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}

// CreateStreamRequest 创建日志流请求.
type CreateStreamRequest struct {
	Name   string `json:"name" binding:"required"`
	Source string `json:"source" binding:"required"`
}

// CreateRetentionPolicyRequest 创建保留策略请求.
type CreateRetentionPolicyRequest struct {
	Name        string `json:"name" binding:"required"`
	Level       string `json:"level"`
	Source      string `json:"source"`
	MaxAgeDays  int    `json:"max_age_days"`
	MaxCount    int    `json:"max_count"`
	ArchivePath string `json:"archive_path"`
}

// AnalysisResult 分析结果.
type AnalysisResult struct {
	TotalLogs int           `json:"total_logs"`
	Anomalies int           `json:"anomalies"`
	Patterns  []PatternStat `json:"patterns"`
	Clusters  []LogCluster  `json:"clusters"`
	Summary   string        `json:"summary"`
}

// StatsQueryRequest 统计查询请求.
type StatsQueryRequest struct {
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
	Source    string     `form:"source"`
	Interval  string     `form:"interval"` // minute, hour, day
}
