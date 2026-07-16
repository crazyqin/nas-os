// Package monitoring 提供系统监控告警功能
// 对标群晖DSM的资源监控
// 支持CPU/内存/磁盘/网络监控、阈值告警、趋势图表
package monitoring

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrMetricNotFound 指标未找到.
	ErrMetricNotFound = errors.New("指标未找到")
	// ErrAlertNotFound 告警未找到.
	ErrAlertNotFound = errors.New("告警未找到")
	// ErrInvalidThreshold 无效阈值.
	ErrInvalidThreshold = errors.New("无效阈值")
)

// ========== 指标类型 ==========

// MetricType 指标类型.
type MetricType string

const (
	MetricCPU         MetricType = "cpu"
	MetricMemory      MetricType = "memory"
	MetricDisk        MetricType = "disk"
	MetricNetwork     MetricType = "network"
	MetricTemperature MetricType = "temperature"
	MetricLoad        MetricType = "load"
)

// ========== 监控数据 ==========

// SystemMetrics 系统指标.
type SystemMetrics struct {
	Timestamp   time.Time     `json:"timestamp"`
	CPU         CPUMetrics    `json:"cpu"`
	Memory      MemoryMetrics `json:"memory"`
	Disks       []DiskMetrics `json:"disks"`
	Network     []NetMetrics  `json:"network"`
	Temperature float64       `json:"temperature"`
	LoadAverage LoadAverage   `json:"load_average"`
	Uptime      int64         `json:"uptime"` // seconds
}

// CPUMetrics CPU指标.
type CPUMetrics struct {
	Usage       float64   `json:"usage"` // 0-100
	User        float64   `json:"user"`
	System      float64   `json:"system"`
	Idle        float64   `json:"idle"`
	IOWait      float64   `json:"iowait"`
	CoreUsages  []float64 `json:"core_usages"`
	Frequency   float64   `json:"frequency"` // MHz
	Temperature float64   `json:"temperature"`
}

// MemoryMetrics 内存指标.
type MemoryMetrics struct {
	Total     int64   `json:"total"` // bytes
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Available int64   `json:"available"`
	Buffers   int64   `json:"buffers"`
	Cached    int64   `json:"cached"`
	SwapTotal int64   `json:"swap_total"`
	SwapUsed  int64   `json:"swap_used"`
	Usage     float64 `json:"usage"` // 0-100
}

// DiskMetrics 磁盘指标.
type DiskMetrics struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mount_point"`
	FSType      string  `json:"fs_type"`
	Total       int64   `json:"total"`
	Used        int64   `json:"used"`
	Free        int64   `json:"free"`
	Usage       float64 `json:"usage"` // 0-100
	ReadBytes   int64   `json:"read_bytes"`
	WriteBytes  int64   `json:"write_bytes"`
	ReadOps     int64   `json:"read_ops"`
	WriteOps    int64   `json:"write_ops"`
	IOTime      int64   `json:"io_time"` // ms
	Temperature float64 `json:"temperature"`
	Health      string  `json:"health"` // good, warning, critical
}

// NetMetrics 网络指标.
type NetMetrics struct {
	Interface   string `json:"interface"`
	BytesSent   int64  `json:"bytes_sent"`
	BytesRecv   int64  `json:"bytes_recv"`
	PacketsSent int64  `json:"packets_sent"`
	PacketsRecv int64  `json:"packets_recv"`
	ErrorsIn    int64  `json:"errors_in"`
	ErrorsOut   int64  `json:"errors_out"`
	DropsIn     int64  `json:"drops_in"`
	DropsOut    int64  `json:"drops_out"`
	Speed       int64  `json:"speed"` // bps
	IsUp        bool   `json:"is_up"`
}

// LoadAverage 负载平均值.
type LoadAverage struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// ========== 告警配置 ==========

// AlertSeverity 告警严重程度.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertRule 告警规则.
type AlertRule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	MetricType    MetricType    `json:"metric_type"`
	MetricName    string        `json:"metric_name"` // e.g., "usage", "temperature"
	Condition     string        `json:"condition"`   // gt, lt, eq, gte, lte
	Threshold     float64       `json:"threshold"`
	Severity      AlertSeverity `json:"severity"`
	Duration      int           `json:"duration"` // seconds, 持续时间
	Enabled       bool          `json:"enabled"`
	NotifyEmail   bool          `json:"notify_email"`
	NotifyWebhook bool          `json:"notify_webhook"`
	WebhookURL    string        `json:"webhook_url,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// AlertRuleCreateRequest 创建告警规则请求.
type AlertRuleCreateRequest struct {
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	MetricType    MetricType    `json:"metric_type"`
	MetricName    string        `json:"metric_name"`
	Condition     string        `json:"condition"`
	Threshold     float64       `json:"threshold"`
	Severity      AlertSeverity `json:"severity"`
	Duration      int           `json:"duration"`
	NotifyEmail   bool          `json:"notify_email"`
	NotifyWebhook bool          `json:"notify_webhook"`
	WebhookURL    string        `json:"webhook_url,omitempty"`
}

// ========== 告警事件 ==========

// Alert 告警事件.
type Alert struct {
	ID         string        `json:"id"`
	RuleID     string        `json:"rule_id"`
	RuleName   string        `json:"rule_name"`
	Severity   AlertSeverity `json:"severity"`
	Message    string        `json:"message"`
	Value      float64       `json:"value"`
	Threshold  float64       `json:"threshold"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	Status     AlertStatus   `json:"status"`
	Notified   bool          `json:"notified"`
	NotifiedAt *time.Time    `json:"notified_at,omitempty"`
}

// AlertStatus 告警状态.
type AlertStatus string

const (
	AlertActive   AlertStatus = "active"
	AlertResolved AlertStatus = "resolved"
	AlertSilenced AlertStatus = "silenced"
)

// ========== 历史数据 ==========

// MetricsHistory 指标历史.
type MetricsHistory struct {
	MetricType MetricType  `json:"metric_type"`
	MetricName string      `json:"metric_name"`
	Start      time.Time   `json:"start"`
	End        time.Time   `json:"end"`
	Interval   int         `json:"interval"` // seconds
	DataPoints []DataPoint `json:"data_points"`
}

// DataPoint 数据点.
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Min       float64   `json:"min,omitempty"`
	Max       float64   `json:"max,omitempty"`
	Avg       float64   `json:"avg,omitempty"`
}

// ========== 统计 ==========

// MonitoringStats 监控统计.
type MonitoringStats struct {
	TotalAlerts      int `json:"total_alerts"`
	ActiveAlerts     int `json:"active_alerts"`
	ResolvedAlerts   int `json:"resolved_alerts"`
	TotalRules       int `json:"total_rules"`
	EnabledRules     int `json:"enabled_rules"`
	MetricsCollected int `json:"metrics_collected"`
}

// ========== 配置 ==========

// MonitorConfig 监控配置.
type MonitorConfig struct {
	CollectInterval    int  `json:"collect_interval"`     // seconds
	HistoryRetention   int  `json:"history_retention"`    // days
	AlertCheckInterval int  `json:"alert_check_interval"` // seconds
	EmailEnabled       bool `json:"email_enabled"`
	WebhookEnabled     bool `json:"webhook_enabled"`
}

// DefaultMonitorConfig 默认配置.
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		CollectInterval:    10,
		HistoryRetention:   30,
		AlertCheckInterval: 5,
		EmailEnabled:       false,
		WebhookEnabled:     false,
	}
}
