// Package netdataex 提供 Netdata 高级系统监控功能
package netdataex

import "time"

// MetricType 指标类型
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// PanelType 面板类型
type PanelType string

const (
	PanelTypeLine    PanelType = "line"
	PanelTypeGauge   PanelType = "gauge"
	PanelTypeBar     PanelType = "bar"
	PanelTypeHeatmap PanelType = "heatmap"
	PanelTypeStat    PanelType = "stat"
	PanelTypeTable   PanelType = "table"
)

// AlertCondition 告警条件
type AlertCondition string

const (
	ConditionGT         AlertCondition = "gt"
	ConditionLT         AlertCondition = "lt"
	ConditionEQ         AlertCondition = "eq"
	ConditionChangeRate AlertCondition = "change_rate"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// MetricSeries 指标序列
type MetricSeries struct {
	Name        string        `json:"name"`
	Unit        string        `json:"unit"`
	Type        MetricType    `json:"type"`
	Points      []MetricPoint `json:"points"`
	Aggregation string        `json:"aggregation,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Metric         string         `json:"metric"`
	Condition      AlertCondition `json:"condition"`
	Threshold      float64        `json:"threshold"`
	Duration       time.Duration  `json:"duration"`
	Severity       AlertSeverity  `json:"severity"`
	NotifyChannels []string       `json:"notify_channels,omitempty"`
	Enabled        bool           `json:"enabled"`
	LastFired      *time.Time     `json:"last_fired,omitempty"`
}

// AlertEvent 告警事件
type AlertEvent struct {
	ID           string        `json:"id"`
	RuleID       string        `json:"rule_id"`
	Metric       string        `json:"metric"`
	Value        float64       `json:"value"`
	Threshold    float64       `json:"threshold"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	Acknowledged bool          `json:"acknowledged"`
	AckedBy      string        `json:"acked_by,omitempty"`
	ResolvedAt   *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// Position 面板位置
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Panel 面板
type Panel struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Type     PanelType `json:"type"`
	Metrics  []string  `json:"metrics"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Position Position  `json:"position"`
}

// Dashboard 仪表板
type Dashboard struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Layout          string    `json:"layout,omitempty"`
	Panels          []Panel   `json:"panels"`
	RefreshInterval int       `json:"refresh_interval"`
	IsDefault       bool      `json:"is_default"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Status  string `json:"status"`
	Value   string `json:"value"`
	Message string `json:"message,omitempty"`
}

// HealthReport 健康报告
type HealthReport struct {
	Score           int             `json:"score"`
	CPU             ComponentHealth `json:"cpu"`
	Memory          ComponentHealth `json:"memory"`
	Disk            ComponentHealth `json:"disk"`
	Network         ComponentHealth `json:"network"`
	Temperature     ComponentHealth `json:"temperature"`
	Power           ComponentHealth `json:"power"`
	Uptime          int64           `json:"uptime"`
	Recommendations []string        `json:"recommendations,omitempty"`
}

// NetdataConfig Netdata 配置
type NetdataConfig struct {
	NetdataURL       string `json:"netdata_url"`
	RetentionDays    int    `json:"retention_days"`
	AlertWebhook     string `json:"alert_webhook,omitempty"`
	DefaultDashboard string `json:"default_dashboard,omitempty"`
	SamplingInterval int    `json:"sampling_interval"`
	ExportEnabled    bool   `json:"export_enabled"`
}
