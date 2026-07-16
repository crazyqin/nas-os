// Package budgetplan 提供预算规划器功能。
// 支持预算编制、支出追踪、成本预测、预算对比和报表生成。
package budgetplan

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBudgetNotFound 预算不存在.
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrBudgetExists 预算已存在错误.
	ErrBudgetExists = errors.New("预算已存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrCategoryNotFound 支出分类不存在.
	ErrCategoryNotFound = errors.New("支出分类不存在")
	// ErrInsufficientData 历史数据不足.
	ErrInsufficientData = errors.New("历史数据不足")
	// ErrExpenseNotFound 支出记录不存在.
	ErrExpenseNotFound = errors.New("支出记录不存在")
)

// ========== 预算周期 ==========

// BudgetPeriod 预算周期类型.
type BudgetPeriod string

const (
	// PeriodMonthly 月度预算.
	PeriodMonthly BudgetPeriod = "monthly"
	// PeriodQuarterly 季度预算.
	PeriodQuarterly BudgetPeriod = "quarterly"
	// PeriodYearly 年度预算.
	PeriodYearly BudgetPeriod = "yearly"
)

// ========== 预算状态 ==========

// BudgetStatus 预算状态.
type BudgetStatus string

const (
	// StatusActive 活跃状态.
	StatusActive BudgetStatus = "active"
	// StatusPaused 暂停状态.
	StatusPaused BudgetStatus = "paused"
	// StatusCompleted 已完成.
	StatusCompleted BudgetStatus = "completed"
	// StatusExceeded 已超支.
	StatusExceeded BudgetStatus = "exceeded"
)

// ========== 支出分类 ==========

// ExpenseCategory 支出分类.
type ExpenseCategory string

const (
	// CategoryHardware 硬件支出.
	CategoryHardware ExpenseCategory = "hardware"
	// CategorySoftware 软件支出.
	CategorySoftware ExpenseCategory = "software"
	// CategoryService 服务支出.
	CategoryService ExpenseCategory = "service"
	// CategoryMaintenance 维护支出.
	CategoryMaintenance ExpenseCategory = "maintenance"
	// CategoryPower 电力支出.
	CategoryPower ExpenseCategory = "power"
	// CategoryBandwidth 带宽支出.
	CategoryBandwidth ExpenseCategory = "bandwidth"
	// CategoryStorage 存储支出.
	CategoryStorage ExpenseCategory = "storage"
	// CategoryOther 其他支出.
	CategoryOther ExpenseCategory = "other"
)

// ========== 核心数据结构 ==========

// Budget 预算定义.
type Budget struct {
	// ID 预算ID.
	ID string `json:"id"`
	// Name 预算名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// Period 预算周期.
	Period BudgetPeriod `json:"period"`
	// TotalAmount 预算总额.
	TotalAmount float64 `json:"total_amount"`
	// UsedAmount 已使用金额.
	UsedAmount float64 `json:"used_amount"`
	// RemainingAmount 剩余金额.
	RemainingAmount float64 `json:"remaining_amount"`
	// UsagePercent 使用百分比.
	UsagePercent float64 `json:"usage_percent"`
	// Status 预算状态.
	Status BudgetStatus `json:"status"`
	// StartDate 周期开始日期.
	StartDate time.Time `json:"start_date"`
	// EndDate 周期结束日期.
	EndDate time.Time `json:"end_date"`
	// Categories 各分类预算分配.
	Categories map[ExpenseCategory]float64 `json:"categories"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
	// CreatedBy 创建人.
	CreatedBy string `json:"created_by"`
	// Tags 标签.
	Tags []string `json:"tags,omitempty"`
}

// BudgetCreateInput 创建预算输入.
type BudgetCreateInput struct {
	// Name 预算名称.
	Name string `json:"name" binding:"required"`
	// Description 描述.
	Description string `json:"description"`
	// Period 预算周期.
	Period BudgetPeriod `json:"period" binding:"required"`
	// TotalAmount 预算总额.
	TotalAmount float64 `json:"total_amount" binding:"required,gt=0"`
	// StartDate 开始日期.
	StartDate *time.Time `json:"start_date"`
	// EndDate 结束日期.
	EndDate *time.Time `json:"end_date"`
	// Categories 各分类预算分配.
	Categories map[ExpenseCategory]float64 `json:"categories"`
	// Tags 标签.
	Tags []string `json:"tags"`
}

// ========== 支出记录 ==========

// Expense 支出记录.
type Expense struct {
	// ID 支出ID.
	ID string `json:"id"`
	// BudgetID 关联预算ID.
	BudgetID string `json:"budget_id"`
	// Amount 支出金额.
	Amount float64 `json:"amount"`
	// Category 支出分类.
	Category ExpenseCategory `json:"category"`
	// Description 支出描述.
	Description string `json:"description"`
	// Vendor 供应商.
	Vendor string `json:"vendor"`
	// InvoiceNumber 发票号.
	InvoiceNumber string `json:"invoice_number"`
	// OccurredAt 支出发生时间.
	OccurredAt time.Time `json:"occurred_at"`
	// CreatedAt 记录创建时间.
	CreatedAt time.Time `json:"created_at"`
	// CreatedBy 记录人.
	CreatedBy string `json:"created_by"`
	// Metadata 附加元数据.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ExpenseInput 支出记录输入.
type ExpenseInput struct {
	// BudgetID 关联预算ID.
	BudgetID string `json:"budget_id" binding:"required"`
	// Amount 支出金额.
	Amount float64 `json:"amount" binding:"required,gt=0"`
	// Category 支出分类.
	Category ExpenseCategory `json:"category" binding:"required"`
	// Description 支出描述.
	Description string `json:"description"`
	// Vendor 供应商.
	Vendor string `json:"vendor"`
	// InvoiceNumber 发票号.
	InvoiceNumber string `json:"invoice_number"`
	// OccurredAt 支出发生时间.
	OccurredAt *time.Time `json:"occurred_at"`
	// Metadata 附加元数据.
	Metadata map[string]interface{} `json:"metadata"`
}

// ========== 成本预测 ==========

// ForecastResult 成本预测结果.
type ForecastResult struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// BudgetID 关联预算ID.
	BudgetID string `json:"budget_id"`
	// BudgetName 预算名称.
	BudgetName string `json:"budget_name"`
	// CurrentUsage 当前使用量.
	CurrentUsage float64 `json:"current_usage"`
	// PredictedUsage 预测使用量.
	PredictedUsage float64 `json:"predicted_usage"`
	// Confidence 置信度 0-1.
	Confidence float64 `json:"confidence"`
	// MonthlyGrowthRate 月增长率.
	MonthlyGrowthRate float64 `json:"monthly_growth_rate"`
	// ProjectedEndDate 预计耗尽日期.
	ProjectedEndDate *time.Time `json:"projected_end_date,omitempty"`
	// ForecastPoints 预测数据点.
	ForecastPoints []ForecastPoint `json:"forecast_points"`
	// Recommendations 建议.
	Recommendations []string `json:"recommendations"`
	// Trend 趋势: increasing/decreasing/stable.
	Trend string `json:"trend"`
}

// ForecastPoint 预测数据点.
type ForecastPoint struct {
	// Date 预测日期.
	Date time.Time `json:"date"`
	// PredictedAmount 预测金额.
	PredictedAmount float64 `json:"predicted_amount"`
	// LowerBound 下限.
	LowerBound float64 `json:"lower_bound"`
	// UpperBound 上限.
	UpperBound float64 `json:"upper_bound"`
}

// HistoricalDataPoint 历史数据点.
type HistoricalDataPoint struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// Amount 金额.
	Amount float64 `json:"amount"`
}

// ========== 预算对比 ==========

// BudgetComparison 预算对比结果.
type BudgetComparison struct {
	// BudgetID 预算ID.
	BudgetID string `json:"budget_id"`
	// BudgetName 预算名称.
	BudgetName string `json:"budget_name"`
	// Period 对比周期.
	Period string `json:"period"`
	// BudgetedAmount 预算金额.
	BudgetedAmount float64 `json:"budgeted_amount"`
	// ActualAmount 实际金额.
	ActualAmount float64 `json:"actual_amount"`
	// Variance 差异金额.
	Variance float64 `json:"variance"`
	// VariancePercent 差异百分比.
	VariancePercent float64 `json:"variance_percent"`
	// Status 状态: under_budget/on_budget/over_budget.
	Status string `json:"status"`
	// CategoryBreakdown 分类明细.
	CategoryBreakdown []CategoryComparison `json:"category_breakdown"`
}

// CategoryComparison 分类对比.
type CategoryComparison struct {
	// Category 分类.
	Category ExpenseCategory `json:"category"`
	// BudgetedAmount 预算金额.
	BudgetedAmount float64 `json:"budgeted_amount"`
	// ActualAmount 实际金额.
	ActualAmount float64 `json:"actual_amount"`
	// Variance 差异.
	Variance float64 `json:"variance"`
	// VariancePercent 差异百分比.
	VariancePercent float64 `json:"variance_percent"`
}

// CompareResult 多预算对比结果.
type CompareResult struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// Comparisons 各预算对比.
	Comparisons []BudgetComparison `json:"comparisons"`
	// TotalBudgeted 总预算金额.
	TotalBudgeted float64 `json:"total_budgeted"`
	// TotalActual 总实际金额.
	TotalActual float64 `json:"total_actual"`
	// TotalVariance 总差异.
	TotalVariance float64 `json:"total_variance"`
	// OverallStatus 整体状态.
	OverallStatus string `json:"overall_status"`
}

// ========== 报表 ==========

// Report 财务报表.
type Report struct {
	// ID 报表ID.
	ID string `json:"id"`
	// Name 报表名称.
	Name string `json:"name"`
	// Type 报表类型.
	Type ReportType `json:"type"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// PeriodStart 周期开始.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 周期结束.
	PeriodEnd time.Time `json:"period_end"`
	// Summary 摘要.
	Summary ReportSummary `json:"summary"`
	// BudgetDetails 预算详情.
	BudgetDetails []BudgetReportDetail `json:"budget_details"`
	// ExpenseSummary 支出摘要.
	ExpenseSummary []ExpenseSummaryItem `json:"expense_summary"`
	// TrendData 趋势数据.
	TrendData []TrendDataPoint `json:"trend_data"`
	// TopExpenses 最大支出.
	TopExpenses []Expense `json:"top_expenses"`
	// GeneratedBy 生成人.
	GeneratedBy string `json:"generated_by"`
}

// ReportType 报表类型.
type ReportType string

const (
	// ReportTypeMonthly 月度报表.
	ReportTypeMonthly ReportType = "monthly"
	// ReportTypeQuarterly 季度报表.
	ReportTypeQuarterly ReportType = "quarterly"
	// ReportTypeYearly 年度报表.
	ReportTypeYearly ReportType = "yearly"
	// ReportTypeCustom 自定义报表.
	ReportTypeCustom ReportType = "custom"
)

// ReportSummary 报表摘要.
type ReportSummary struct {
	// TotalBudgets 预算总数.
	TotalBudgets int `json:"total_budgets"`
	// ActiveBudgets 活跃预算数.
	ActiveBudgets int `json:"active_budgets"`
	// TotalBudgetAmount 总预算金额.
	TotalBudgetAmount float64 `json:"total_budget_amount"`
	// TotalExpenses 总支出金额.
	TotalExpenses float64 `json:"total_expenses"`
	// TotalRemaining 总剩余金额.
	TotalRemaining float64 `json:"total_remaining"`
	// AvgUsagePercent 平均使用率.
	AvgUsagePercent float64 `json:"avg_usage_percent"`
	// ExceededBudgets 超支预算数.
	ExceededBudgets int `json:"exceeded_budgets"`
	// HealthScore 健康评分 0-100.
	HealthScore int `json:"health_score"`
}

// BudgetReportDetail 预算报表详情.
type BudgetReportDetail struct {
	// BudgetID 预算ID.
	BudgetID string `json:"budget_id"`
	// BudgetName 预算名称.
	BudgetName string `json:"budget_name"`
	// Period 周期.
	Period BudgetPeriod `json:"period"`
	// TotalAmount 预算总额.
	TotalAmount float64 `json:"total_amount"`
	// UsedAmount 已使用金额.
	UsedAmount float64 `json:"used_amount"`
	// RemainingAmount 剩余金额.
	RemainingAmount float64 `json:"remaining_amount"`
	// UsagePercent 使用百分比.
	UsagePercent float64 `json:"usage_percent"`
	// Status 状态.
	Status BudgetStatus `json:"status"`
	// ExpenseCount 支出笔数.
	ExpenseCount int `json:"expense_count"`
}

// ExpenseSummaryItem 支出摘要条目.
type ExpenseSummaryItem struct {
	// Category 分类.
	Category ExpenseCategory `json:"category"`
	// TotalAmount 总金额.
	TotalAmount float64 `json:"total_amount"`
	// Count 笔数.
	Count int `json:"count"`
	// Percent 占总支出百分比.
	Percent float64 `json:"percent"`
}

// TrendDataPoint 趋势数据点.
type TrendDataPoint struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// BudgetAmount 预算金额.
	BudgetAmount float64 `json:"budget_amount"`
	// ExpenseAmount 支出金额.
	ExpenseAmount float64 `json:"expense_amount"`
	// Remaining 剩余.
	Remaining float64 `json:"remaining"`
}

// ReportRequest 报表请求.
type ReportRequest struct {
	// Type 报表类型.
	Type ReportType `json:"type"`
	// Name 报表名称.
	Name string `json:"name"`
	// StartDate 开始日期.
	StartDate *time.Time `json:"start_date"`
	// EndDate 结束日期.
	EndDate *time.Time `json:"end_date"`
	// BudgetIDs 指定预算ID（可选，为空则包含所有）.
	BudgetIDs []string `json:"budget_ids,omitempty"`
	// IncludeTopExpenses 是否包含最大支出明细.
	IncludeTopExpenses bool `json:"include_top_expenses"`
	// TopExpensesCount 最大支出条数.
	TopExpensesCount int `json:"top_expenses_count"`
}

// ========== 查询参数 ==========

// ExpenseQuery 支出查询参数.
type ExpenseQuery struct {
	// BudgetID 预算ID过滤.
	BudgetID string `json:"budget_id,omitempty"`
	// Categories 分类过滤.
	Categories []ExpenseCategory `json:"categories,omitempty"`
	// StartTime 开始时间.
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间.
	EndTime *time.Time `json:"end_time,omitempty"`
	// MinAmount 最小金额.
	MinAmount *float64 `json:"min_amount,omitempty"`
	// MaxAmount 最大金额.
	MaxAmount *float64 `json:"max_amount,omitempty"`
	// Vendor 供应商过滤.
	Vendor string `json:"vendor,omitempty"`
	// Page 页码.
	Page int `json:"page"`
	// PageSize 每页条数.
	PageSize int `json:"page_size"`
}
