// Package cost 提供成本计算和管理功能 (v2.90.0 户部)
package cost

import (
	"time"
)

// ========== 成本类型定义 ==========

// CostType 成本类型.
type CostType string

const (
	// CostTypeCPU represents CPU cost type.
	CostTypeCPU CostType = "cpu"
	// CostTypeMemory represents memory cost type.
	CostTypeMemory CostType = "memory"
	// CostTypeStorage represents storage cost type.
	CostTypeStorage CostType = "storage"
	// CostTypeNetwork represents network cost type.
	CostTypeNetwork CostType = "network"
	// CostTypeElectricity represents electricity cost type.
	CostTypeElectricity CostType = "electricity"
	// CostTypeHardware represents hardware cost type.
	CostTypeHardware CostType = "hardware"
	// CostTypeLicense represents license cost type.
	CostTypeLicense CostType = "license"
)

// CostRecord 成本记录.
type CostRecord struct {
	// 记录ID
	ID string `json:"id"`

	// 关联对象ID（应用/用户/系统）
	TargetID string `json:"target_id"`

	// 关联对象类型
	TargetType string `json:"target_type"` // app, user, system

	// 成本类型
	CostType CostType `json:"cost_type"`

	// 成本金额（元）
	Amount float64 `json:"amount"`

	// 货币
	Currency string `json:"currency"`

	// 计量值
	Measurement float64 `json:"measurement"`

	// 计量单位
	Unit string `json:"unit"` // core_hours, gb_hours, gb, gb_month

	// 单价
	UnitPrice float64 `json:"unit_price"`

	// 计费周期开始
	PeriodStart time.Time `json:"period_start"`

	// 计费周期结束
	PeriodEnd time.Time `json:"period_end"`

	// 记录时间
	RecordedAt time.Time `json:"recorded_at"`

	// 定价模型
	PricingModel string `json:"pricing_model"`

	// 标签
	Labels map[string]string `json:"labels,omitempty"`
}

// CostSummary 成本汇总.
type CostSummary struct {
	// 对象ID
	TargetID string `json:"target_id"`

	// 对象名称
	TargetName string `json:"target_name"`

	// 对象类型
	TargetType string `json:"target_type"`

	// 统计周期开始
	PeriodStart time.Time `json:"period_start"`

	// 统计周期结束
	PeriodEnd time.Time `json:"period_end"`

	// 成本明细
	CostBreakdown map[CostType]float64 `json:"cost_breakdown"`

	// 总成本
	TotalCost float64 `json:"total_cost"`

	// 货币
	Currency string `json:"currency"`

	// 预算限制
	BudgetLimit float64 `json:"budget_limit,omitempty"`

	// 预算使用率
	BudgetUsagePercent float64 `json:"budget_usage_percent,omitempty"`

	// 与上期对比
	ChangePercent float64 `json:"change_percent,omitempty"`

	// 成本效率
	EfficiencyScore float64 `json:"efficiency_score"`

	// 预测下期成本
	ForecastNextPeriod float64 `json:"forecast_next_period,omitempty"`
}

// PricingTier 定价阶梯.
type PricingTier struct {
	// 阶梯名称
	Name string `json:"name"`

	// 起始值
	StartValue float64 `json:"start_value"`

	// 结束值（null表示无上限）
	EndValue *float64 `json:"end_value,omitempty"`

	// 单价
	UnitPrice float64 `json:"unit_price"`

	// 折扣比例
	DiscountPercent float64 `json:"discount_percent"`
}

// PricingModel 定价模型.
type PricingModel struct {
	// 模型ID
	ID string `json:"id"`

	// 模型名称
	Name string `json:"name"`

	// 成本类型
	CostType CostType `json:"cost_type"`

	// 计费周期
	BillingPeriod string `json:"billing_period"` // hourly, daily, monthly

	// 基础单价
	BasePrice float64 `json:"base_price"`

	// 阶梯定价
	Tiers []PricingTier `json:"tiers,omitempty"`

	// 预留折扣
	ReservedDiscount float64 `json:"reserved_discount,omitempty"`

	// 货币
	Currency string `json:"currency"`

	// 是否启用
	Enabled bool `json:"enabled"`

	// 生效时间
	EffectiveFrom time.Time `json:"effective_from"`

	// 过期时间
	EffectiveUntil *time.Time `json:"effective_until,omitempty"`
}

// Budget 预算配置.
type Budget struct {
	// 预算ID
	ID string `json:"id"`

	// 预算名称
	Name string `json:"name"`

	// 关联对象ID
	TargetID string `json:"target_id"`

	// 关联对象类型
	TargetType string `json:"target_type"`

	// 预算金额
	Limit float64 `json:"limit"`

	// 货币
	Currency string `json:"currency"`

	// 预算周期
	Period string `json:"period"` // daily, weekly, monthly

	// 告警阈值1（百分比）
	AlertThreshold1 float64 `json:"alert_threshold_1"` // 如70%

	// 告警阈值2（百分比）
	AlertThreshold2 float64 `json:"alert_threshold_2"` // 如90%

	// 告警阈值3（百分比）
	AlertThreshold3 float64 `json:"alert_threshold_3"` // 如100%

	// 是否启用
	Enabled bool `json:"enabled"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`

	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 预算ID
	BudgetID string `json:"budget_id"`

	// 告警级别
	Level string `json:"level"` // warning, critical

	// 当前使用率
	UsagePercent float64 `json:"usage_percent"`

	// 预算限制
	BudgetLimit float64 `json:"budget_limit"`

	// 当前成本
	CurrentCost float64 `json:"current_cost"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 消息
	Message string `json:"message"`

	// 是否已处理
	Resolved bool `json:"resolved"`

	// 处理时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// CostOptimizationSuggestion 成本优化建议.
type CostOptimizationSuggestion struct {
	// 建议ID
	ID string `json:"id"`

	// 目标对象ID
	TargetID string `json:"target_id"`

	// 建议类型
	Type string `json:"type"` // scale_down, scale_up, optimize, migrate, terminate

	// 优先级
	Priority int `json:"priority"` // 1-5

	// 建议标题
	Title string `json:"title"`

	// 建议描述
	Description string `json:"description"`

	// 预计节省金额
	EstimatedSavings float64 `json:"estimated_savings"`

	// 实施难度
	ImplementationComplexity string `json:"implementation_complexity"` // easy, medium, hard

	// 影响评估
	ImpactAssessment string `json:"impact_assessment"`

	// 实施步骤
	Steps []string `json:"steps,omitempty"`

	// 置信度
	Confidence float64 `json:"confidence"` // 0-1

	// 数据来源
	DataSources []string `json:"data_sources"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`

	// 状态
	Status string `json:"status"` // pending, accepted, rejected, implemented
}

// CostReportRequest 成本报告请求.
type CostReportRequest struct {
	// 报告类型
	Type string `json:"type"` // summary, detail, trend, comparison

	// 目标对象ID（可选）
	TargetID string `json:"target_id,omitempty"`

	// 目标类型（可选）
	TargetType string `json:"target_type,omitempty"`

	// 周期开始
	PeriodStart time.Time `json:"period_start"`

	// 周期结束
	PeriodEnd time.Time `json:"period_end"`

	// 是否包含预测
	IncludeForecast bool `json:"include_forecast"`

	// 是否包含建议
	IncludeSuggestions bool `json:"include_suggestions"`

	// 是否包含明细
	IncludeDetails bool `json:"include_details"`

	// 输出格式
	Format string `json:"format"` // json, html, csv, xlsx
}

// CostReportResponse 成本报告响应.
type CostReportResponse struct {
	// 报告ID
	ID string `json:"id"`

	// 报告类型
	Type string `json:"type"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 报告周期
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 成本汇总
	Summary CostSummary `json:"summary"`

	// 成本明细（可选）
	Details []CostRecord `json:"details,omitempty"`

	// 成本趋势（可选）
	Trend []TrendPoint `json:"trend,omitempty"`

	// 预测数据（可选）
	Forecast []ForecastPoint `json:"forecast,omitempty"`

	// 优化建议（可选）
	Suggestions []CostOptimizationSuggestion `json:"suggestions,omitempty"`

	// 对比数据（可选）
	Comparison *CostComparison `json:"comparison,omitempty"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	// 时间点
	Timestamp time.Time `json:"timestamp"`

	// 成本值
	Cost float64 `json:"cost"`

	// 成本类型细分
	CostBreakdown map[CostType]float64 `json:"cost_breakdown,omitempty"`
}

// ForecastPoint 预测数据点.
type ForecastPoint struct {
	// 时间点
	Timestamp time.Time `json:"timestamp"`

	// 预测成本
	PredictedCost float64 `json:"predicted_cost"`

	// 置信区间下限
	ConfidenceLow float64 `json:"confidence_low"`

	// 置信区间上限
	ConfidenceHigh float64 `json:"confidence_high"`

	// 置信度
	Confidence float64 `json:"confidence"`
}

// CostComparison 成本对比.
type CostComparison struct {
	// 对比周期
	ComparisonPeriod string `json:"comparison_period"` // previous_period, same_period_last_month, same_period_last_year

	// 对比周期开始
	PeriodStart time.Time `json:"period_start"`

	// 对比周期结束
	PeriodEnd time.Time `json:"period_end"`

	// 对比成本
	ComparisonCost float64 `json:"comparison_cost"`

	// 变化金额
	ChangeAmount float64 `json:"change_amount"`

	// 变化比例
	ChangePercent float64 `json:"change_percent"`

	// 变化原因分析
	ChangeReasons []CostChangeReason `json:"change_reasons,omitempty"`
}

// CostChangeReason 成本变化原因.
type CostChangeReason struct {
	// 成本类型
	CostType CostType `json:"cost_type"`

	// 变化金额
	ChangeAmount float64 `json:"change_amount"`

	// 变化比例
	ChangePercent float64 `json:"change_percent"`

	// 原因描述
	Description string `json:"description"`
}