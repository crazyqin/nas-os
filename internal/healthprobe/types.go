// Package healthprobe 提供系统健康探测功能
// 支持硬件健康监控、服务健康检查、自定义探针、健康评分和故障预测
package healthprobe

import (
	"context"
	"time"
)

// HealthLevel 健康级别
type HealthLevel string

const (
	// LevelHealthy 健康状态
	LevelHealthy HealthLevel = "healthy"
	// LevelDegraded 降级状态
	LevelDegraded HealthLevel = "degraded"
	// LevelCritical 严重状态
	LevelCritical HealthLevel = "critical"
	// LevelUnknown 未知状态
	LevelUnknown HealthLevel = "unknown"
)

// Severity 告警严重程度
type Severity string

const (
	// SeverityInfo 信息级别
	SeverityInfo Severity = "info"
	// SeverityWarning 警告级别
	SeverityWarning Severity = "warning"
	// SeverityCritical 严重级别
	SeverityCritical Severity = "critical"
)

// ProbeCategory 探针类别
type ProbeCategory string

const (
	// CategoryHardware 硬件探针
	CategoryHardware ProbeCategory = "hardware"
	// CategoryService 服务探针
	CategoryService ProbeCategory = "service"
	// CategoryCustom 自定义探针
	CategoryCustom ProbeCategory = "custom"
)

// MetricType 指标类型
type MetricType string

const (
	// MetricCPU 使用率指标
	MetricCPU MetricType = "cpu"
	// MetricMemory 内存指标
	MetricMemory MetricType = "memory"
	// MetricDisk 磁盘指标
	MetricDisk MetricType = "disk"
	// MetricNetwork 网络指标
	MetricNetwork MetricType = "network"
	// MetricTemp 温度指标
	MetricTemp MetricType = "temperature"
	// MetricSMART SMART 指标
	MetricSMART MetricType = "smart"
	// MetricECC ECC 指标
	MetricECC MetricType = "ecc"
	// MetricPower 电源指标
	MetricPower MetricType = "power"
	// MetricHTTP HTTP 服务指标
	MetricHTTP MetricType = "http"
	// MetricTCP TCP 服务指标
	MetricTCP MetricType = "tcp"
	// MetricProcess 进程指标
	MetricProcess MetricType = "process"
	// MetricCustom 自定义指标
	MetricCustom MetricType = "custom"
)

// ProbeResult 单项探针结果
type ProbeResult struct {
	Name       string                 `json:"name"`
	Category   ProbeCategory          `json:"category"`
	Type       MetricType             `json:"type"`
	Level      HealthLevel            `json:"level"`
	Value      float64                `json:"value"`
	Unit       string                 `json:"unit"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus 聚合健康状态
type HealthStatus struct {
	Level     HealthLevel             `json:"level"`
	Score     float64                 `json:"score"` // 0-100 健康评分
	Timestamp time.Time               `json:"timestamp"`
	Uptime    time.Duration           `json:"uptime"`
	Probes    map[string]*ProbeResult `json:"probes"`
	Summary   *StatusSummary          `json:"summary"`
	Trend     *TrendAnalysis          `json:"trend,omitempty"`
	Alerts    []*Alert                `json:"alerts,omitempty"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
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
	Direction  string           `json:"direction"` // improving, stable, degrading
	ScoreDelta float64          `json:"scoreDelta"`
	History    []*HistoryRecord `json:"history"`
	Prediction *TrendPrediction `json:"prediction,omitempty"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	EstimatedLevel HealthLevel `json:"estimatedLevel"`
	EstimatedTime  time.Time   `json:"estimatedTime"`
	Confidence     float64     `json:"confidence"`
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp time.Time   `json:"timestamp"`
	Level     HealthLevel `json:"level"`
	Score     float64     `json:"score"`
	Probes    int         `json:"probes"`
}

// Alert 健康告警
type Alert struct {
	ID        string      `json:"id"`
	Probe     string      `json:"probe"`
	Severity  Severity    `json:"severity"`
	Level     HealthLevel `json:"level"`
	Message   string      `json:"message"`
	Value     float64     `json:"value"`
	Threshold float64     `json:"threshold"`
	Timestamp time.Time   `json:"timestamp"`
	Resolved  bool        `json:"resolved"`
}

// Rule 健康检查规则
type Rule struct {
	Name      string        `json:"name"`
	Type      MetricType    `json:"type"`
	Category  ProbeCategory `json:"category"`
	Threshold float64       `json:"threshold"`
	Level     HealthLevel   `json:"level"`
	Operator  string        `json:"operator"` // gt, lt, gte, lte, eq
	Weight    float64       `json:"weight"`   // 权重 0-1
	Message   string        `json:"message"`
	Enabled   bool          `json:"enabled"`
}

// HealthReport 健康报告
type HealthReport struct {
	GeneratedAt time.Time              `json:"generatedAt"`
	Summary     *StatusSummary         `json:"summary"`
	Score       float64                `json:"score"`
	Level       HealthLevel            `json:"level"`
	Probes      map[string]*ProbeResult `json:"probes"`
	Alerts      []*Alert               `json:"alerts"`
	Trend       *TrendAnalysis         `json:"trend,omitempty"`
	TopIssues   []*ProbeResult         `json:"topIssues,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
}

// Config 健康探针配置
type Config struct {
	Interval      time.Duration `json:"interval"`      // 检测间隔
	Timeout       time.Duration `json:"timeout"`       // 单项检测超时
	HistorySize   int           `json:"historySize"`   // 历史记录大小
	AlertCooldown time.Duration `json:"alertCooldown"` // 告警冷却时间
	EnableTrend   bool          `json:"enableTrend"`   // 启用趋势分析
	TrendWindow   int           `json:"trendWindow"`   // 趋势分析窗口大小
	AutoStart     bool          `json:"autoStart"`     // 自动启动
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Interval:      30 * time.Second,
		Timeout:       10 * time.Second,
		HistorySize:   1440, // 24h @ 1min intervals
		AlertCooldown: 5 * time.Minute,
		EnableTrend:   true,
		TrendWindow:   10,
		AutoStart:     false,
	}
}

// Probe 探针接口
type Probe interface {
	// Name 返回探针名称
	Name() string
	// Type 返回指标类型
	Type() MetricType
	// Category 返回探针类别
	Category() ProbeCategory
	// Collect 收集探针数据
	Collect(ctx context.Context) (*ProbeResult, error)
}

// Notifier 告警通知接口
type Notifier interface {
	// Notify 发送告警通知
	Notify(ctx context.Context, alert *Alert) error
}

// ProbeFunc 函数式探针实现
type ProbeFunc struct {
	name     string
	mtype    MetricType
	category ProbeCategory
	collect  func(ctx context.Context) (*ProbeResult, error)
}

// NewProbeFunc 创建函数式探针
func NewProbeFunc(name string, mtype MetricType, category ProbeCategory, fn func(ctx context.Context) (*ProbeResult, error)) *ProbeFunc {
	return &ProbeFunc{
		name:     name,
		mtype:    mtype,
		category: category,
		collect:  fn,
	}
}

// Name 返回探针名称
func (p *ProbeFunc) Name() string { return p.name }

// Type 返回指标类型
func (p *ProbeFunc) Type() MetricType { return p.mtype }

// Category 返回探针类别
func (p *ProbeFunc) Category() ProbeCategory { return p.category }

// Collect 收集探针数据
func (p *ProbeFunc) Collect(ctx context.Context) (*ProbeResult, error) {
	return p.collect(ctx)
}

// NotifierFunc 函数式通知器
type NotifierFunc func(ctx context.Context, alert *Alert) error

// Notify 发送告警通知
func (f NotifierFunc) Notify(ctx context.Context, alert *Alert) error { return f(ctx, alert) }
