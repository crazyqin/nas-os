// Package containerautoscale 提供容器自动扩缩容功能，支持基于 CPU/内存/请求数/自定义指标的智能扩缩。
package containerautoscale

import "time"

// ScaleStrategy 扩缩策略
type ScaleStrategy string

const (
	StrategyThreshold ScaleStrategy = "threshold" // 阈值策略
	StrategyPredict   ScaleStrategy = "predict"   // 预测策略
	StrategyManual    ScaleStrategy = "manual"    // 手动策略
	StrategySchedule  ScaleStrategy = "schedule"  // 定时策略
)

// ScaleDirection 扩缩方向
type ScaleDirection string

const (
	ScaleUp   ScaleDirection = "up"   // 扩容
	ScaleDown ScaleDirection = "down" // 缩容
)

// MetricType 指标类型
type MetricType string

const (
	MetricCPU      MetricType = "cpu"      // CPU 使用率
	MetricMemory   MetricType = "memory"   // 内存使用率
	MetricRequests MetricType = "requests" // 请求数
	MetricCustom   MetricType = "custom"   // 自定义指标
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// Container 容器信息
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ServiceName string            `json:"service_name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	Replicas    int               `json:"replicas"`
	MinReplicas int               `json:"min_replicas"`
	MaxReplicas int               `json:"max_replicas"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ResourceQuota 资源配额
type ResourceQuota struct {
	ID            string  `json:"id"`
	ServiceName   string  `json:"service_name"`
	MaxCPU        float64 `json:"max_cpu"`          // CPU 核数上限
	MaxMemoryMB   int64   `json:"max_memory_mb"`    // 内存上限(MB)
	MaxReplicas   int     `json:"max_replicas"`     // 最大副本数
	MinReplicas   int     `json:"min_replicas"`     // 最小副本数
	MaxCostPerDay float64 `json:"max_cost_per_day"` // 每日成本上限(元)
	CurrentCost   float64 `json:"current_cost"`     // 当日已花费(元)
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp   time.Time  `json:"timestamp"`
	Type        MetricType `json:"type"`
	Value       float64    `json:"value"`
	ContainerID string     `json:"container_id,omitempty"`
	ServiceName string     `json:"service_name"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	ScaleUpThreshold   float64 `json:"scale_up_threshold"`   // 扩容阈值
	ScaleDownThreshold float64 `json:"scale_down_threshold"` // 缩容阈值
	EvaluationPeriods  int     `json:"evaluation_periods"`   // 评估周期数
	ScaleUpStep        int     `json:"scale_up_step"`        // 每次扩容步长
	ScaleDownStep      int     `json:"scale_down_step"`      // 每次缩容步长
}

// ScheduleRule 定时规则
type ScheduleRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CronExpr  string    `json:"cron_expr"` // cron 表达式
	Replicas  int       `json:"replicas"`  // 目标副本数
	Enabled   bool      `json:"enabled"`
	StartDate time.Time `json:"start_date,omitempty"`
	EndDate   time.Time `json:"end_date,omitempty"`
}

// ScalePolicy 扩缩策略配置
type ScalePolicy struct {
	ID              string           `json:"id"`
	ServiceName     string           `json:"service_name"`
	Strategy        ScaleStrategy    `json:"strategy"`
	Enabled         bool             `json:"enabled"`
	MetricType      MetricType       `json:"metric_type"`
	Threshold       *ThresholdConfig `json:"threshold,omitempty"`
	Schedules       []ScheduleRule   `json:"schedules,omitempty"`
	CooldownSec     int              `json:"cooldown_sec"`      // 冷却期(秒)
	CooldownUpSec   int              `json:"cooldown_up_sec"`   // 扩容冷却期
	CooldownDownSec int              `json:"cooldown_down_sec"` // 缩容冷却期
	LastScaleTime   time.Time        `json:"last_scale_time"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// ScaleEvent 扩缩事件
type ScaleEvent struct {
	ID            string         `json:"id"`
	ServiceName   string         `json:"service_name"`
	Direction     ScaleDirection `json:"direction"`
	FromReplicas  int            `json:"from_replicas"`
	ToReplicas    int            `json:"to_replicas"`
	Strategy      ScaleStrategy  `json:"strategy"`
	Reason        string         `json:"reason"`
	TriggerMetric string         `json:"trigger_metric,omitempty"`
	MetricValue   float64        `json:"metric_value,omitempty"`
	Success       bool           `json:"success"`
	Error         string         `json:"error,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// PredictResult 预测结果
type PredictResult struct {
	ServiceName     string    `json:"service_name"`
	PredictedValue  float64   `json:"predicted_value"`
	Confidence      float64   `json:"confidence"`
	RecommendedReps int       `json:"recommended_replicas"`
	Horizon         string    `json:"horizon"` // 预测时间跨度
	CreatedAt       time.Time `json:"created_at"`
}

// CostSuggestion 成本优化建议
type CostSuggestion struct {
	ID            string    `json:"id"`
	ServiceName   string    `json:"service_name"`
	Type          string    `json:"type"` // rightsize, schedule, spot
	Description   string    `json:"description"`
	CurrentCost   float64   `json:"current_cost"`
	EstimatedSave float64   `json:"estimated_save"`
	Priority      string    `json:"priority"` // low, medium, high
	CreatedAt     time.Time `json:"created_at"`
}

// Alert 告警信息
type Alert struct {
	ID          string     `json:"id"`
	ServiceName string     `json:"service_name"`
	Level       AlertLevel `json:"level"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	Resolved    bool       `json:"resolved"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AutoScaleConfig 自动扩缩容全局配置
type AutoScaleConfig struct {
	Enabled               bool `json:"enabled"`
	MetricsIntervalSec    int  `json:"metrics_interval_sec"` // 指标采集间隔
	HistoryRetentionDays  int  `json:"history_retention_days"`
	DefaultCooldownSec    int  `json:"default_cooldown_sec"`
	MaxScaleEventsPerHour int  `json:"max_scale_events_per_hour"`
}

// DefaultAutoScaleConfig 默认配置
func DefaultAutoScaleConfig() *AutoScaleConfig {
	return &AutoScaleConfig{
		Enabled:               true,
		MetricsIntervalSec:    30,
		HistoryRetentionDays:  30,
		DefaultCooldownSec:    300,
		MaxScaleEventsPerHour: 10,
	}
}

// ScaleRequest 扩缩请求
type ScaleRequest struct {
	ServiceName string `json:"service_name" binding:"required"`
	Replicas    int    `json:"replicas" binding:"required,min=0"`
	Reason      string `json:"reason,omitempty"`
}

// PolicyRequest 策略请求
type PolicyRequest struct {
	ServiceName     string           `json:"service_name" binding:"required"`
	Strategy        ScaleStrategy    `json:"strategy" binding:"required"`
	MetricType      MetricType       `json:"metric_type"`
	Threshold       *ThresholdConfig `json:"threshold,omitempty"`
	Schedules       []ScheduleRule   `json:"schedules,omitempty"`
	CooldownSec     int              `json:"cooldown_sec"`
	CooldownUpSec   int              `json:"cooldown_up_sec"`
	CooldownDownSec int              `json:"cooldown_down_sec"`
}

// QuotaRequest 配额请求
type QuotaRequest struct {
	ServiceName   string  `json:"service_name" binding:"required"`
	MaxCPU        float64 `json:"max_cpu"`
	MaxMemoryMB   int64   `json:"max_memory_mb"`
	MaxReplicas   int     `json:"max_replicas"`
	MinReplicas   int     `json:"min_replicas"`
	MaxCostPerDay float64 `json:"max_cost_per_day"`
}

// MetricsQuery 指标查询
type MetricsQuery struct {
	ServiceName string     `json:"service_name"`
	MetricType  MetricType `json:"metric_type"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	Granularity string     `json:"granularity,omitempty"` // 1m, 5m, 1h
}
