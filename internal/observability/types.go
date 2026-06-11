// Package observability 提供企业级可观测性平台功能，
// 包括统一日志收集与聚合、指标采集与导出、分布式追踪支持、
// 告警规则引擎和可观测性数据导出。
package observability

import (
	"time"
)

// ========== 日志类型 ==========

// LogLevel 日志级别.
type LogLevel string

const (
	// LogLevelDebug 调试级别.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo 信息级别.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn 警告级别.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError 错误级别.
	LogLevelError LogLevel = "error"
	// LogLevelFatal 致命级别.
	LogLevelFatal LogLevel = "fatal"
)

// LogSource 日志来源.
type LogSource string

const (
	// LogSourceSystem 系统日志.
	LogSourceSystem LogSource = "system"
	// LogSourceApplication 应用日志.
	LogSourceApplication LogSource = "application"
	// LogSourceAudit 审计日志.
	LogSourceAudit LogSource = "audit"
)

// LogEntry 统一日志条目.
type LogEntry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Level     LogLevel          `json:"level"`
	Source    LogSource         `json:"source"`
	Service   string            `json:"service"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
	Fields    map[string]any    `json:"fields,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	SpanID    string            `json:"span_id,omitempty"`
	Host      string            `json:"host,omitempty"`
}

// LogQuery 日志查询请求.
type LogQuery struct {
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Level     LogLevel          `json:"level,omitempty"`
	Source    LogSource         `json:"source,omitempty"`
	Service   string            `json:"service,omitempty"`
	Keyword   string            `json:"keyword,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Limit     int               `json:"limit"`
	Offset    int               `json:"offset"`
}

// LogQueryResult 日志查询结果.
type LogQueryResult struct {
	Entries  []*LogEntry `json:"entries"`
	Total    int         `json:"total"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	HasMore  bool        `json:"has_more"`
	Duration string      `json:"duration"`
}

// ========== 指标类型 ==========

// MetricType 指标类型.
type MetricType string

const (
	// MetricTypeCounter 计数器（单调递增）.
	MetricTypeCounter MetricType = "counter"
	// MetricTypeGauge 仪表盘（可增可减）.
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeHistogram 直方图.
	MetricTypeHistogram MetricType = "histogram"
	// MetricTypeSummary 摘要.
	MetricTypeSummary MetricType = "summary"
)

// Metric 指标定义.
type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Help      string            `json:"help,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
}

// MetricFamily 指标家族（同名不同标签的指标集合）.
type MetricFamily struct {
	Name    string     `json:"name"`
	Type    MetricType `json:"type"`
	Help    string     `json:"help"`
	Metrics []*Metric  `json:"metrics"`
}

// MetricQuery 指标查询请求.
type MetricQuery struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Step      time.Duration     `json:"step"`
}

// MetricSample 指标采样点.
type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricQueryResult 指标查询结果.
type MetricQueryResult struct {
	Name    string          `json:"name"`
	Labels  map[string]string `json:"labels,omitempty"`
	Samples []*MetricSample `json:"samples"`
}

// ========== 追踪类型 ==========

// SpanStatus 追踪跨度状态.
type SpanStatus string

const (
	// SpanStatusOK 成功.
	SpanStatusOK SpanStatus = "ok"
	// SpanStatusError 错误.
	SpanStatusError SpanStatus = "error"
	// SpanStatusUnset 未设置.
	SpanStatusUnset SpanStatus = "unset"
)

// Span 追踪跨度.
type Span struct {
	TraceID       string            `json:"trace_id"`
	SpanID        string            `json:"span_id"`
	ParentSpanID  string            `json:"parent_span_id,omitempty"`
	Name          string            `json:"name"`
	Service       string            `json:"service"`
	Kind          string            `json:"kind"` // client, server, producer, consumer, internal
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Duration      time.Duration     `json:"duration"`
	Status        SpanStatus        `json:"status"`
	StatusMessage string            `json:"status_message,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Events        []SpanEvent       `json:"events,omitempty"`
}

// SpanEvent 跨度事件.
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Trace 完整追踪链.
type Trace struct {
	TraceID   string  `json:"trace_id"`
	RootSpan  *Span   `json:"root_span,omitempty"`
	Spans     []*Span `json:"spans"`
	Duration  time.Duration `json:"duration"`
	Services  []string `json:"services"`
	SpanCount int      `json:"span_count"`
}

// TraceQuery 追踪查询请求.
type TraceQuery struct {
	TraceID   string    `json:"trace_id,omitempty"`
	Service   string    `json:"service,omitempty"`
	Operation string    `json:"operation,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	MinDuration time.Duration `json:"min_duration,omitempty"`
	MaxDuration time.Duration `json:"max_duration,omitempty"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
}

// ========== 告警类型 ==========

// AlertSeverity 告警严重级别.
type AlertSeverity string

const (
	// AlertSeverityInfo 信息.
	AlertSeverityInfo AlertSeverity = "info"
	// AlertSeverityWarning 警告.
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical 严重.
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertRuleType 告警规则类型.
type AlertRuleType string

const (
	// AlertRuleTypeThreshold 阈值告警.
	AlertRuleTypeThreshold AlertRuleType = "threshold"
	// AlertRuleTypeAnomaly 异常检测告警.
	AlertRuleTypeAnomaly AlertRuleType = "anomaly"
	// AlertRuleTypeCorrelation 关联分析告警.
	AlertRuleTypeCorrelation AlertRuleType = "correlation"
)

// ThresholdOperator 阈值比较操作符.
type ThresholdOperator string

const (
	// ThresholdOpGT 大于.
	ThresholdOpGT ThresholdOperator = "gt"
	// ThresholdOpGTE 大于等于.
	ThresholdOpGTE ThresholdOperator = "gte"
	// ThresholdOpLT 小于.
	ThresholdOpLT ThresholdOperator = "lt"
	// ThresholdOpLTE 小于等于.
	ThresholdOpLTE ThresholdOperator = "lte"
	// ThresholdOpEQ 等于.
	ThresholdOpEQ ThresholdOperator = "eq"
	// ThresholdOpNEQ 不等于.
	ThresholdOpNEQ ThresholdOperator = "neq"
)

// AlertRule 告警规则.
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Enabled     bool              `json:"enabled"`
	Type        AlertRuleType     `json:"type"`
	Severity    AlertSeverity     `json:"severity"`
	Labels      map[string]string `json:"labels,omitempty"`
	// 阈值告警配置
	ThresholdConfig *ThresholdConfig `json:"threshold_config,omitempty"`
	// 异常检测配置
	AnomalyConfig *AnomalyConfig `json:"anomaly_config,omitempty"`
	// 关联分析配置
	CorrelationConfig *CorrelationConfig `json:"correlation_config,omitempty"`
	// 通知配置
	NotifyChannels []NotifyChannel `json:"notify_channels,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ThresholdConfig 阈值告警配置.
type ThresholdConfig struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	Operator   ThresholdOperator `json:"operator"`
	Value      float64           `json:"value"`
	Duration   time.Duration     `json:"duration"` // 持续时间
}

// AnomalyConfig 异常检测配置.
type AnomalyConfig struct {
	MetricName  string            `json:"metric_name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Sensitivity float64           `json:"sensitivity"` // 灵敏度 (0-1)
	WindowSize  time.Duration     `json:"window_size"` // 检测窗口
}

// CorrelationConfig 关联分析配置.
type CorrelationConfig struct {
	Rules []CorrelationRule `json:"rules"`
}

// CorrelationRule 关联规则.
type CorrelationRule struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	Operator   ThresholdOperator `json:"operator"`
	Value      float64           `json:"value"`
}

// NotifyChannel 通知渠道.
type NotifyChannel struct {
	Type    string `json:"type"` // email, webhook, sms
	Address string `json:"address"`
	Enabled bool   `json:"enabled"`
}

// AlertInstance 告警实例（已触发的告警）.
type AlertInstance struct {
	ID          string        `json:"id"`
	RuleID      string        `json:"rule_id"`
	RuleName    string        `json:"rule_name"`
	Severity    AlertSeverity `json:"severity"`
	Message     string        `json:"message"`
	Labels      map[string]string `json:"labels,omitempty"`
	Value       float64       `json:"value"`
	Threshold   float64       `json:"threshold,omitempty"`
	Status      AlertStatus   `json:"status"`
	StartsAt    time.Time     `json:"starts_at"`
	EndsAt      *time.Time    `json:"ends_at,omitempty"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AlertStatus 告警状态.
type AlertStatus string

const (
	// AlertStatusFiring 触发中.
	AlertStatusFiring AlertStatus = "firing"
	// AlertStatusResolved 已解决.
	AlertStatusResolved AlertStatus = "resolved"
	// AlertStatusSilenced 已静默.
	AlertStatusSilenced AlertStatus = "silenced"
)

// ========== 导出类型 ==========

// ExportTarget 导出目标类型.
type ExportTarget string

const (
	// ExportTargetGrafana Grafana 导出.
	ExportTargetGrafana ExportTarget = "grafana"
	// ExportTargetLoki Loki 日志导出.
	ExportTargetLoki ExportTarget = "loki"
	// ExportTargetElasticsearch Elasticsearch 导出.
	ExportTargetElasticsearch ExportTarget = "elasticsearch"
	// ExportTargetPrometheus Prometheus 指标导出.
	ExportTargetPrometheus ExportTarget = "prometheus"
)

// ExportConfig 导出配置.
type ExportConfig struct {
	ID        string       `json:"id"`
	Target    ExportTarget `json:"target"`
	Endpoint  string       `json:"endpoint"`
	Enabled   bool         `json:"enabled"`
	AuthType  string       `json:"auth_type,omitempty"` // basic, bearer, api_key
	AuthToken string       `json:"auth_token,omitempty"`
	BatchSize int          `json:"batch_size"`
	FlushInterval time.Duration `json:"flush_interval"`
}

// ========== 配置类型 ==========

// Config 可观测性平台配置.
type Config struct {
	// 日志配置
	LogConfig LogConfig `json:"log_config"`
	// 指标配置
	MetricConfig MetricConfig `json:"metric_config"`
	// 追踪配置
	TraceConfig TraceConfig `json:"trace_config"`
	// 告警配置
	AlertConfig AlertConfig `json:"alert_config"`
	// 导出配置
	ExportConfigs []ExportConfig `json:"export_configs"`
}

// LogConfig 日志配置.
type LogConfig struct {
	RetentionDays int    `json:"retention_days"`
	MaxEntries    int    `json:"max_entries"`
	BatchSize     int    `json:"batch_size"`
	FlushInterval time.Duration `json:"flush_interval"`
}

// MetricConfig 指标配置.
type MetricConfig struct {
	ScrapeInterval time.Duration `json:"scrape_interval"`
	RetentionDays  int           `json:"retention_days"`
	MaxMetrics     int           `json:"max_metrics"`
}

// TraceConfig 追踪配置.
type TraceConfig struct {
	SampleRate   float64       `json:"sample_rate"`   // 采样率 (0-1)
	MaxSpans     int           `json:"max_spans"`
	RetentionDays int          `json:"retention_days"`
}

// AlertConfig 告警配置.
type AlertConfig struct {
	EvaluationInterval time.Duration `json:"evaluation_interval"`
	MaxAlerts          int           `json:"max_alerts"`
	ResolveTimeout     time.Duration `json:"resolve_timeout"`
}

// DefaultConfig 默认配置.
var DefaultConfig = &Config{
	LogConfig: LogConfig{
		RetentionDays: 30,
		MaxEntries:    100000,
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
	},
	MetricConfig: MetricConfig{
		ScrapeInterval: 15 * time.Second,
		RetentionDays:  90,
		MaxMetrics:     50000,
	},
	TraceConfig: TraceConfig{
		SampleRate:    1.0,
		MaxSpans:      100000,
		RetentionDays: 7,
	},
	AlertConfig: AlertConfig{
		EvaluationInterval: 30 * time.Second,
		MaxAlerts:          1000,
		ResolveTimeout:     5 * time.Minute,
	},
}

// ========== 请求/响应类型 ==========

// IngestLogsRequest 日志收集请求.
type IngestLogsRequest struct {
	Entries []*LogEntry `json:"entries" binding:"required,min=1"`
}

// IngestMetricsRequest 指标收集请求.
type IngestMetricsRequest struct {
	Metrics []*Metric `json:"metrics" binding:"required,min=1"`
}

// IngestTracesRequest 追踪收集请求.
type IngestTracesRequest struct {
	Spans []*Span `json:"spans" binding:"required,min=1"`
}

// CreateAlertRuleRequest 创建告警规则请求.
type CreateAlertRuleRequest struct {
	Name              string             `json:"name" binding:"required"`
	Description       string             `json:"description"`
	Type              AlertRuleType      `json:"type" binding:"required"`
	Severity          AlertSeverity      `json:"severity" binding:"required"`
	Labels            map[string]string  `json:"labels"`
	ThresholdConfig   *ThresholdConfig   `json:"threshold_config"`
	AnomalyConfig     *AnomalyConfig     `json:"anomaly_config"`
	CorrelationConfig *CorrelationConfig `json:"correlation_config"`
	NotifyChannels    []NotifyChannel    `json:"notify_channels"`
}

// UpdateAlertRuleRequest 更新告警规则请求.
type UpdateAlertRuleRequest struct {
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	Enabled           *bool              `json:"enabled"`
	Severity          AlertSeverity      `json:"severity"`
	Labels            map[string]string  `json:"labels"`
	ThresholdConfig   *ThresholdConfig   `json:"threshold_config"`
	AnomalyConfig     *AnomalyConfig     `json:"anomaly_config"`
	CorrelationConfig *CorrelationConfig `json:"correlation_config"`
	NotifyChannels    []NotifyChannel    `json:"notify_channels"`
}

// CreateExportConfigRequest 创建导出配置请求.
type CreateExportConfigRequest struct {
	Target        ExportTarget  `json:"target" binding:"required"`
	Endpoint      string        `json:"endpoint" binding:"required"`
	AuthType      string        `json:"auth_type"`
	AuthToken     string        `json:"auth_token"`
	BatchSize     int           `json:"batch_size"`
	FlushInterval time.Duration `json:"flush_interval"`
}

// ========== 状态/统计类型 ==========

// PlatformStats 平台统计信息.
type PlatformStats struct {
	TotalLogs       int64         `json:"total_logs"`
	TotalMetrics    int64         `json:"total_metrics"`
	TotalTraces     int64         `json:"total_traces"`
	TotalAlertRules int           `json:"total_alert_rules"`
	ActiveAlerts    int           `json:"active_alerts"`
	ExportTargets   int           `json:"export_targets"`
	Uptime          time.Duration `json:"uptime"`
}

// HealthStatus 健康状态.
type HealthStatus struct {
	Status     string            `json:"status"` // ok, degraded, unhealthy
	Components map[string]string `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}
