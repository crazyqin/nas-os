// Package smartbudget 提供智能预算管理功能
package smartbudget

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPlanNotFound 预算计划不存在错误.
	ErrPlanNotFound = errors.New("预算计划不存在")
	// ErrPlanExists 预算计划已存在错误.
	ErrPlanExists = errors.New("预算计划已存在")
	// ErrInvalidInput 无效输入错误.
	ErrInvalidInput = errors.New("无效输入")
	// ErrProviderNotSupported 不支持的云提供商错误.
	ErrProviderNotSupported = errors.New("不支持的云提供商")
	// ErrAnalysisFailed 分析失败错误.
	ErrAnalysisFailed = errors.New("分析失败")
)

// ========== 云提供商类型 ==========

// CloudProvider 云提供商类型.
type CloudProvider string

const (
	// ProviderAWS Amazon Web Services.
	ProviderAWS CloudProvider = "aws"
	// ProviderAzure Microsoft Azure.
	ProviderAzure CloudProvider = "azure"
	// ProviderGCP Google Cloud Platform.
	ProviderGCP CloudProvider = "gcp"
	// ProviderLocal 本地存储.
	ProviderLocal CloudProvider = "local"
)

// ========== 预算周期 ==========

// Period 预算周期.
type Period string

const (
	// PeriodMonthly 月度周期.
	PeriodMonthly Period = "monthly"
	// PeriodQuarterly 季度周期.
	PeriodQuarterly Period = "quarterly"
	// PeriodYearly 年度周期.
	PeriodYearly Period = "yearly"
)

// ========== 趋势类型 ==========

// Trend 趋势方向.
type Trend string

const (
	// TrendUp 上升趋势.
	TrendUp Trend = "up"
	// TrendDown 下降趋势.
	TrendDown Trend = "down"
	// TrendStable 稳定趋势.
	TrendStable Trend = "stable"
)

// ========== 优化类型 ==========

// OptimizationType 优化类型.
type OptimizationType string

const (
	// OptTypeColdMigration 冷数据迁移.
	OptTypeColdMigration OptimizationType = "cold_migration"
	// OptTypeDedup 去重.
	OptTypeDedup OptimizationType = "dedup"
	// OptTypeCompress 压缩.
	OptTypeCompress OptimizationType = "compress"
	// OptTypeArchive 归档.
	OptTypeArchive OptimizationType = "archive"
	// OptTypeDelete 删除冗余.
	OptTypeDelete OptimizationType = "delete"
)

// ========== 优先级 ==========

// Priority 优先级.
type Priority string

const (
	// PriorityHigh 高优先级.
	PriorityHigh Priority = "high"
	// PriorityMedium 中优先级.
	PriorityMedium Priority = "medium"
	// PriorityLow 低优先级.
	PriorityLow Priority = "low"
)

// ========== 告警级别 ==========

// AlertLevel 告警级别.
type AlertLevel string

const (
	// AlertLevelInfo 信息.
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告.
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重.
	AlertLevelCritical AlertLevel = "critical"
)

// ========== 核心数据结构 ==========

// BudgetPlan 预算计划.
type BudgetPlan struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Department  string    `json:"department"`
	Project     string    `json:"project,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	MonthlyCap  float64   `json:"monthly_cap"`
	CurrentUse  float64   `json:"current_use"`
	Currency    string    `json:"currency"`
	Period      Period    `json:"period"`
	Provider    CloudProvider `json:"provider,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CostBreakdown 成本明细.
type CostBreakdown struct {
	Category    string  `json:"category"` // storage, compute, network, backup
	Amount      float64 `json:"amount"`
	Percentage  float64 `json:"percentage"`
	Trend       Trend   `json:"trend"`
	Department  string  `json:"department,omitempty"`
	Provider    string  `json:"provider,omitempty"`
}

// CostOptimization 成本优化建议.
type CostOptimization struct {
	ID          string          `json:"id"`
	Type        OptimizationType `json:"type"`
	Description string          `json:"description"`
	SavingEst   float64         `json:"saving_estimate"`
	Priority    Priority        `json:"priority"`
	Resource    string          `json:"resource,omitempty"`
	Department  string          `json:"department,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// CostTrend 成本趋势数据点.
type CostTrend struct {
	Date        time.Time `json:"date"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category,omitempty"`
	Department  string    `json:"department,omitempty"`
}

// CostForecast 成本预测.
type CostForecast struct {
	Date            time.Time `json:"date"`
	PredictedAmount float64   `json:"predicted_amount"`
	Confidence      float64   `json:"confidence"` // 0-100
	LowerBound      float64   `json:"lower_bound"`
	UpperBound      float64   `json:"upper_bound"`
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	PlanName    string    `json:"plan_name"`
	Level       AlertLevel `json:"level"`
	Message     string    `json:"message"`
	Threshold   float64   `json:"threshold"`
	CurrentUse  float64   `json:"current_use"`
	BudgetCap   float64   `json:"budget_cap"`
	CreatedAt   time.Time `json:"created_at"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
}

// MonthlyReport 月度报告.
type MonthlyReport struct {
	Month       string           `json:"month"` // YYYY-MM
	TotalCost   float64          `json:"total_cost"`
	BudgetCap   float64          `json:"budget_cap"`
	Usage       float64          `json:"usage"` // percentage
	Breakdown   []CostBreakdown  `json:"breakdown"`
	Trends      []CostTrend      `json:"trends"`
	Alerts      []BudgetAlert    `json:"alerts"`
	Optimizations []CostOptimization `json:"optimizations"`
}

// ========== 请求/响应结构 ==========

// CreatePlanRequest 创建预算计划请求.
type CreatePlanRequest struct {
	Name       string        `json:"name" binding:"required"`
	Department string        `json:"department" binding:"required"`
	Project    string        `json:"project,omitempty"`
	Owner      string        `json:"owner,omitempty"`
	MonthlyCap float64       `json:"monthly_cap" binding:"required,gt=0"`
	Currency   string        `json:"currency"`
	Period     Period        `json:"period"`
	Provider   CloudProvider `json:"provider,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
}

// CostQueryRequest 成本查询请求.
type CostQueryRequest struct {
	Department string    `form:"department,omitempty"`
	Project    string    `form:"project,omitempty"`
	Provider   string    `form:"provider,omitempty"`
	Category   string    `form:"category,omitempty"`
	StartDate  string    `form:"start_date,omitempty"`
	EndDate    string    `form:"end_date,omitempty"`
}

// TrendQueryRequest 趋势查询请求.
type TrendQueryRequest struct {
	Department string `form:"department,omitempty"`
	Category   string `form:"category,omitempty"`
	Months     int    `form:"months,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse 成功响应.
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
