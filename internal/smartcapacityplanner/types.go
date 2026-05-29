// Package smartcapacityplanner 提供智能容量规划功能
package smartcapacityplanner

import (
	"time"
)

// CapacitySnapshot 容量快照.
type CapacitySnapshot struct {
	ID          string    `json:"id"`
	TotalBytes  int64     `json:"total_bytes"`  // 总容量
	UsedBytes   int64     `json:"used_bytes"`   // 已使用容量
	FreeBytes   int64     `json:"free_bytes"`   // 剩余容量
	UsageRate   float64   `json:"usage_rate"`   // 使用率 (0-1)
	MountPoint  string    `json:"mount_point"`  // 挂载点
	FileSystem  string    `json:"file_system"`  // 文件系统类型
	Timestamp   time.Time `json:"timestamp"`
}

// ForecastResult 预测结果.
type ForecastResult struct {
	ID              string    `json:"id"`
	ModelType       string    `json:"model_type"`       // linear, exponential, seasonal
	PredictedUsage  float64   `json:"predicted_usage"`  // 预测使用率
	PredictedDate   time.Time `json:"predicted_date"`   // 预测日期
	Confidence      float64   `json:"confidence"`       // 置信度 (0-1)
	GrowthRate      float64   `json:"growth_rate"`      // 增长率
	Timestamp       time.Time `json:"timestamp"`
}

// GrowthTrend 增长趋势.
type GrowthTrend struct {
	ID            string    `json:"id"`
	Period        string    `json:"period"`         // daily, weekly, monthly
	GrowthBytes   int64     `json:"growth_bytes"`   // 增长字节数
	GrowthRate    float64   `json:"growth_rate"`    // 增长率
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	Timestamp     time.Time `json:"timestamp"`
}

// CapacityPlan 容量规划建议.
type CapacityPlan struct {
	ID               string    `json:"id"`
	CurrentUsage     float64   `json:"current_usage"`     // 当前使用率
	PredictedUsage   float64   `json:"predicted_usage"`   // 预测使用率
	DaysUntilFull    int       `json:"days_until_full"`   // 预计多少天后满
	RecommendedAction string  `json:"recommended_action"` // 建议操作
	RecommendedSize  int64     `json:"recommended_size"`  // 建议扩容大小
	Priority         string    `json:"priority"`          // high, medium, low
	Timestamp        time.Time `json:"timestamp"`
}

// Alert 告警信息.
type Alert struct {
	ID          string    `json:"id"`
	Level       string    `json:"level"`       // critical, warning, info
	Message     string    `json:"message"`
	Threshold   float64   `json:"threshold"`   // 触发阈值
	Current     float64   `json:"current"`     // 当前值
	IsRead      bool      `json:"is_read"`
	Timestamp   time.Time `json:"timestamp"`
}

// ForecastModel 预测模型类型.
type ForecastModel string

const (
	ModelLinear      ForecastModel = "linear"
	ModelExponential ForecastModel = "exponential"
	ModelSeasonal    ForecastModel = "seasonal"
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertCritical AlertLevel = "critical"
	AlertWarning  AlertLevel = "warning"
	AlertInfo     AlertLevel = "info"
)

// Priority 优先级.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// RecordUsageRequest 记录使用量请求.
type RecordUsageRequest struct {
	TotalBytes int64  `json:"total_bytes" binding:"required"`
	UsedBytes  int64  `json:"used_bytes" binding:"required"`
	MountPoint string `json:"mount_point" binding:"required"`
	FileSystem string `json:"file_system"`
}

// ForecastRequest 预测请求.
type ForecastRequest struct {
	ModelType  string `json:"model_type" binding:"required"` // linear, exponential, seasonal
	DaysAhead  int    `json:"days_ahead"`                    // 预测未来多少天
	MountPoint string `json:"mount_point"`
}

// PlanRequest 生成规划请求.
type PlanRequest struct {
	MountPoint string `json:"mount_point"`
}

// AlertConfigRequest 告警配置请求.
type AlertConfigRequest struct {
	WarningThreshold  float64 `json:"warning_threshold"`  // 告警阈值
	CriticalThreshold float64 `json:"critical_threshold"` // 严重告警阈值
}
