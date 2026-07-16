// Package costdashboard 提供云存储成本分析功能，帮助用户分析各云存储的费用和使用情况
package costdashboard

import (
	"time"
)

// CloudProviderType 云提供商类型.
type CloudProviderType string

const (
	ProviderAliyun   CloudProviderType = "aliyun"
	ProviderTencent  CloudProviderType = "tencent"
	ProviderAWS      CloudProviderType = "aws"
	ProviderGDrive   CloudProviderType = "gdrive"
	ProviderOneDrive CloudProviderType = "onedrive"
)

// AlertSeverity 告警严重级别.
type AlertSeverity string

const (
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// ReportPeriod 报告周期.
type ReportPeriod string

const (
	PeriodMonthly ReportPeriod = "monthly"
	PeriodWeekly  ReportPeriod = "weekly"
	PeriodDaily   ReportPeriod = "daily"
)

// OptimizationType 优化类型.
type OptimizationType string

const (
	OptOversized  OptimizationType = "oversized"
	OptInfrequent OptimizationType = "infrequent"
	OptDuplicate  OptimizationType = "duplicate"
)

// CloudProvider 云提供商.
type CloudProvider struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      CloudProviderType `json:"type"`
	APIKey    string            `json:"api_key,omitempty"`
	Region    string            `json:"region,omitempty"`
	Status    string            `json:"status"` // active/inactive/error
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// StorageMetrics 存储指标.
type StorageMetrics struct {
	ProviderID   string    `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	UsedBytes    int64     `json:"used_bytes"`
	TotalBytes   int64     `json:"total_bytes"`
	CostPerGB    float64   `json:"cost_per_gb"`
	MonthlyCost  float64   `json:"monthly_cost"`
	TransferCost float64   `json:"transfer_cost"`
	SyncedAt     time.Time `json:"synced_at"`
}

// CostReport 成本报告.
type CostReport struct {
	ID          string           `json:"id"`
	Period      ReportPeriod     `json:"period"`
	Providers   []StorageMetrics `json:"providers"`
	TotalCost   float64          `json:"total_cost"`
	Trend       string           `json:"trend"` // up/down/stable
	GeneratedAt time.Time        `json:"generated_at"`
}

// CostAlert 成本告警.
type CostAlert struct {
	ID          string        `json:"id"`
	ProviderID  string        `json:"provider_id"`
	Threshold   float64       `json:"threshold"`
	CurrentCost float64       `json:"current_cost"`
	Severity    AlertSeverity `json:"severity"`
	TriggeredAt time.Time     `json:"triggered_at"`
	Acked       bool          `json:"acked"`
}

// CostOptimization 成本优化建议.
type CostOptimization struct {
	ID                string           `json:"id"`
	Type              OptimizationType `json:"type"`
	Description       string           `json:"description"`
	PotentialSaving   float64          `json:"potential_saving"`
	RecommendedAction string           `json:"recommended_action"`
	ProviderID        string           `json:"provider_id"`
	GeneratedAt       time.Time        `json:"generated_at"`
}

// AddProviderRequest 添加云提供商请求.
type AddProviderRequest struct {
	Name   string            `json:"name" binding:"required"`
	Type   CloudProviderType `json:"type" binding:"required"`
	APIKey string            `json:"api_key"`
	Region string            `json:"region"`
}

// UpdateProviderRequest 更新云提供商请求.
type UpdateProviderRequest struct {
	Name   *string `json:"name,omitempty"`
	APIKey *string `json:"api_key,omitempty"`
	Region *string `json:"region,omitempty"`
	Status *string `json:"status,omitempty"`
}

// GenerateReportRequest 生成报告请求.
type GenerateReportRequest struct {
	Period ReportPeriod `json:"period" binding:"required"`
}

// SetAlertRequest 设置告警请求.
type SetAlertRequest struct {
	ProviderID string        `json:"provider_id" binding:"required"`
	Threshold  float64       `json:"threshold" binding:"required"`
	Severity   AlertSeverity `json:"severity" binding:"required"`
}

// AcknowledgeAlertRequest 确认告警请求.
type AcknowledgeAlertRequest struct {
	AlertID string `json:"alert_id" binding:"required"`
}

// CompareRequest 对比请求.
type CompareRequest struct {
	ProviderIDs []string `form:"provider_ids"`
}

// TrendRequest 趋势请求.
type TrendRequest struct {
	ProviderID string `form:"provider_id"`
	Period     string `form:"period"`
}
