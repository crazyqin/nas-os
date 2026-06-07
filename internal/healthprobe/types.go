// Package healthprobe 智能健康探测 - 系统健康检查与监控
package healthprobe

import (
	"context"
	"time"
)

// HealthLevel 健康级别
type HealthLevel string

const (
	LevelHealthy  HealthLevel = "healthy"  // 健康
	LevelDegraded HealthLevel = "degraded" // 降级
	LevelCritical HealthLevel = "critical" // 严重
	LevelUnknown  HealthLevel = "unknown"  // 未知
)

// ProbeCategory 探针类别
type ProbeCategory = string

const (
	CategoryHardware ProbeCategory = "hardware" // 硬件
	CategorySoftware ProbeCategory = "software" // 软件
	CategoryNetwork  ProbeCategory = "network"  // 网络
	CategoryService  ProbeCategory = "service"  // 服务
)

// MetricType 指标类型
type MetricType string

const (
	MetricCPU    MetricType = "cpu"     // CPU
	MetricMemory MetricType = "memory"  // 内存
	MetricDisk   MetricType = "disk"    // 磁盘
	MetricTemp   MetricType = "temp"    // 温度
	MetricSMART  MetricType = "smart"   // SMART
	MetricNet    MetricType = "network" // 网络
	MetricLoad   MetricType = "load"    // 负载
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Probe 探针接口
type Probe interface {
	Name() string
	Type() MetricType
	Category() string
	Collect(ctx context.Context) (*ProbeResult, error)
}

// ProbeResult 探针结果
type ProbeResult struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Category  string                 `json:"category"`
	Level     HealthLevel            `json:"level"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// StatusSummary 状态摘要
type StatusSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	Direction  string           `json:"direction"` // improving/stable/degrading
	ScoreDelta float64          `json:"score_delta"`
	Prediction *TrendPrediction `json:"prediction,omitempty"`
	History    []*HistoryRecord `json:"history"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	EstimatedLevel HealthLevel `json:"estimated_level"`
	EstimatedTime  time.Time   `json:"estimated_time"`
	Confidence     float64     `json:"confidence"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Timestamp time.Time               `json:"timestamp"`
	Uptime    time.Duration           `json:"uptime"`
	Level     HealthLevel             `json:"level"`
	Score     float64                 `json:"score"`
	Summary   *StatusSummary          `json:"summary"`
	Probes    map[string]*ProbeResult `json:"probes"`
	Trend     *TrendAnalysis          `json:"trend,omitempty"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
}

// HealthReport 健康报告
type HealthReport struct {
	GeneratedAt     time.Time               `json:"generated_at"`
	Summary         *StatusSummary          `json:"summary"`
	Score           float64                 `json:"score"`
	Level           HealthLevel             `json:"level"`
	Probes          map[string]*ProbeResult `json:"probes"`
	Alerts          []*Alert                `json:"alerts"`
	Trend           *TrendAnalysis          `json:"trend,omitempty"`
	TopIssues       []*ProbeResult          `json:"top_issues,omitempty"`
	Recommendations []string                `json:"recommendations"`
}

// Rule 检查规则
type Rule struct {
	Name      string        `json:"name"`
	Type      MetricType    `json:"type"`
	Category  ProbeCategory `json:"category,omitempty"`
	Operator  string        `json:"operator"` // gt/lt/gte/lte/eq
	Threshold float64       `json:"threshold"`
	Level     HealthLevel   `json:"level"`
	Message   string        `json:"message"`
	Weight    float64       `json:"weight"`
	Enabled   bool          `json:"enabled"`
}

// Alert 告警
type Alert struct {
	ID        string        `json:"id"`
	Probe     string        `json:"probe"`
	Severity  AlertSeverity `json:"severity"`
	Level     HealthLevel   `json:"level"`
	Message   string        `json:"message"`
	Value     float64       `json:"value"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
}

// Notifier 通知器接口
type Notifier interface {
	Notify(ctx context.Context, alert *Alert) error
}

// NotifierFunc 通知器函数类型
type NotifierFunc func(ctx context.Context, alert *Alert) error

// Notify 实现 Notifier 接口
func (f NotifierFunc) Notify(ctx context.Context, alert *Alert) error {
	return f(ctx, alert)
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp time.Time   `json:"timestamp"`
	Level     HealthLevel `json:"level"`
	Score     float64     `json:"score"`
	Probes    int         `json:"probes"`
}

// Config 配置
type Config struct {
	AutoStart     bool          `json:"auto_start"`     // 自动启动
	Interval      time.Duration `json:"interval"`       // 检测间隔
	Timeout       time.Duration `json:"timeout"`        // 单次探测超时
	HistorySize   int           `json:"history_size"`   // 历史记录大小
	TrendWindow   int           `json:"trend_window"`   // 趋势分析窗口
	EnableTrend   bool          `json:"enable_trend"`   // 启用趋势分析
	AlertCooldown time.Duration `json:"alert_cooldown"` // 告警冷却期
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		AutoStart:     false,
		Interval:      30 * time.Second,
		Timeout:       10 * time.Second,
		HistorySize:   1440,
		TrendWindow:   10,
		EnableTrend:   true,
		AlertCooldown: 5 * time.Minute,
	}
}

// probeFunc 探针函数实现
type probeFunc struct {
	name     string
	typ      MetricType
	category ProbeCategory
	fn       func(ctx context.Context) (*ProbeResult, error)
}

// NewProbeFunc 创建函数探针
func NewProbeFunc(name string, typ MetricType, category ProbeCategory, fn func(ctx context.Context) (*ProbeResult, error)) Probe {
	return &probeFunc{
		name:     name,
		typ:      typ,
		category: category,
		fn:       fn,
	}
}

func (p *probeFunc) Name() string     { return p.name }
func (p *probeFunc) Type() MetricType { return p.typ }
func (p *probeFunc) Category() string { return string(p.category) }
func (p *probeFunc) Collect(ctx context.Context) (*ProbeResult, error) {
	return p.fn(ctx)
}
