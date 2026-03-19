// Package budget 提供预算管理功能
package budget

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBudgetNotFound 预算不存在错误
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrBudgetExists 预算已存在错误
	ErrBudgetExists = errors.New("预算已存在")
	// ErrBudgetExceeded 超出预算限制错误
	ErrBudgetExceeded = errors.New("超出预算限制")
	// ErrInvalidAmount 无效的预算金额错误
	ErrInvalidAmount = errors.New("无效的预算金额")
	// ErrInvalidPeriod 无效的预算周期错误
	ErrInvalidPeriod = errors.New("无效的预算周期")
	// ErrAlertNotFound 预警规则不存在错误
	ErrAlertNotFound = errors.New("预警规则不存在")
	// ErrInvalidThreshold 无效的阈值错误
	ErrInvalidThreshold = errors.New("无效的阈值")
	// ErrNoPermission 无权限操作错误
	ErrNoPermission = errors.New("无权限操作")
)

// ========== 预算类型 ==========

// Type 预算类型
type Type string

// 预算类型常量，定义预算的资源类型。
const (
	// TypeStorage represents storage budget type
	TypeStorage Type = "storage" // 存储预算
	// TypeBandwidth represents bandwidth budget type
	TypeBandwidth Type = "bandwidth" // 带宽预算
	// TypeCompute represents compute budget type
	TypeCompute Type = "compute" // 计算预算
	// TypeOperations represents operations budget type
	TypeOperations Type = "operations" // 运维预算
	// TypeTotal represents total budget type
	TypeTotal Type = "total" // 总预算
)

// Period 预算周期
type Period string

// 预算周期常量，定义预算的时间周期。
const (
	// PeriodDaily represents daily budget period
	PeriodDaily Period = "daily" // 日预算
	// PeriodWeekly represents weekly budget period
	PeriodWeekly Period = "weekly" // 周预算
	// PeriodMonthly represents monthly budget period
	PeriodMonthly Period = "monthly" // 月预算
	// PeriodQuarter represents quarterly budget period
	PeriodQuarter Period = "quarter" // 季度预算
	// PeriodYearly represents yearly budget period
	PeriodYearly Period = "yearly" // 年预算
)

// Scope 预算范围
type Scope string

// 预算范围常量，定义预算的应用范围。
const (
	// ScopeGlobal represents global budget scope
	ScopeGlobal Scope = "global" // 全局预算
	// ScopeUser represents user budget scope
	ScopeUser Scope = "user" // 用户预算
	// ScopeGroup represents group budget scope
	ScopeGroup Scope = "group" // 用户组预算
	// ScopeVolume represents volume budget scope
	ScopeVolume Scope = "volume" // 卷预算
	// ScopeService represents service budget scope
	ScopeService Scope = "service" // 服务预算
	// ScopeDirectory represents directory budget scope
	ScopeDirectory Scope = "directory" // 目录预算
)

// Status 预算状态
type Status string

// 预算状态常量，定义预算的当前状态。
const (
	// StatusActive represents active budget status
	StatusActive Status = "active" // 活跃
	// StatusPaused represents paused budget status
	StatusPaused Status = "paused" // 暂停
	// StatusExceeded represents exceeded budget status
	StatusExceeded Status = "exceeded" // 超支
	// StatusExhausted represents exhausted budget status
	StatusExhausted Status = "exhausted" // 耗尽
	// StatusArchived represents archived budget status
	StatusArchived Status = "archived" // 归档
)

// ========== 预算定义 ==========

// Budget 预算定义
type Budget struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        Type   `json:"type"`
	Period      Period `json:"period"`
	Scope       Scope  `json:"scope"`

	// 预算目标
	TargetID   string `json:"target_id"`   // 用户ID/组ID/卷名等
	TargetName string `json:"target_name"` // 显示名称

	// 预算金额（单位：元）
	Amount       float64 `json:"amount"`        // 预算总额
	UsedAmount   float64 `json:"used_amount"`   // 已使用金额
	Remaining    float64 `json:"remaining"`     // 剩余金额
	UsagePercent float64 `json:"usage_percent"` // 使用百分比

	// 时间范围
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	LastReset time.Time  `json:"last_reset"`
	NextReset *time.Time `json:"next_reset,omitempty"`

	// 状态和配置
	Status      Status      `json:"status"`
	AutoReset   bool        `json:"auto_reset"`   // 是否自动重置
	Rollover    bool        `json:"rollover"`     // 是否结转
	AlertConfig AlertConfig `json:"alert_config"` // 预警配置

	// 元数据
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
	Tags      []string  `json:"tags,omitempty"`
}

// Input 创建/更新预算输入
type Input struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Type        Type         `json:"type" binding:"required"`
	Period      Period       `json:"period" binding:"required"`
	Scope       Scope        `json:"scope" binding:"required"`
	TargetID    string       `json:"target_id"`
	TargetName  string       `json:"target_name"`
	Amount      float64      `json:"amount" binding:"required,gt=0"`
	StartDate   *time.Time   `json:"start_date"`
	EndDate     *time.Time   `json:"end_date"`
	AutoReset   bool         `json:"auto_reset"`
	Rollover    bool         `json:"rollover"`
	AlertConfig *AlertConfig `json:"alert_config"`
	Tags        []string     `json:"tags"`
}

// ========== 预算使用记录 ==========

// Usage 预算使用记录
type Usage struct {
	ID         string    `json:"id"`
	BudgetID   string    `json:"budget_id"`
	RecordedAt time.Time `json:"recorded_at"`

	// 使用详情
	Amount     float64 `json:"amount"`     // 本次使用金额
	Cumulative float64 `json:"cumulative"` // 累计使用
	Remaining  float64 `json:"remaining"`  // 剩余金额

	// 使用来源
	SourceType  string `json:"source_type"` // storage, bandwidth, compute, etc.
	SourceID    string `json:"source_id"`   // 来源ID
	Description string `json:"description"` // 描述

	// 资源详情
	ResourceType string  `json:"resource_type"` // 资源类型
	ResourceID   string  `json:"resource_id"`   // 资源ID
	UnitCost     float64 `json:"unit_cost"`     // 单价
	Quantity     float64 `json:"quantity"`      // 数量

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UsageInput 预算使用输入
type UsageInput struct {
	BudgetID     string                 `json:"budget_id" binding:"required"`
	Amount       float64                `json:"amount" binding:"required,gt=0"`
	SourceType   string                 `json:"source_type" binding:"required"`
	SourceID     string                 `json:"source_id"`
	Description  string                 `json:"description"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	UnitCost     float64                `json:"unit_cost"`
	Quantity     float64                `json:"quantity"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ========== 预算预警 ==========

// AlertConfig 预警配置
type AlertConfig struct {
	Enabled           bool             `json:"enabled"`
	Thresholds        []AlertThreshold `json:"thresholds"`
	NotifyEmail       bool             `json:"notify_email"`
	NotifyWebhook     bool             `json:"notify_webhook"`
	WebhookURL        string           `json:"webhook_url,omitempty"`
	NotifyChannels    []string         `json:"notify_channels,omitempty"`
	CooldownMinutes   int              `json:"cooldown_minutes"`   // 冷却时间
	EscalationEnabled bool             `json:"escalation_enabled"` // 升级预警
	EscalationRules   []EscalationRule `json:"escalation_rules"`
}

// AlertThreshold 预警阈值
type AlertThreshold struct {
	Percent     float64  `json:"percent"`      // 触发百分比
	Level       Level    `json:"level"`        // 预警级别
	Message     string   `json:"message"`      // 自定义消息
	Actions     []string `json:"actions"`      // 触发动作
	NotifyUsers []string `json:"notify_users"` // 通知用户
}

// AlertLevel, BudgetAlert, AlertStatus 定义在 alert.go 中

// EscalationRule 升级规则
type EscalationRule struct {
	AfterMinutes int      `json:"after_minutes"` // 多少分钟后升级
	ToLevel      Level    `json:"to_level"`      // 升级到级别
	NotifyUsers  []string `json:"notify_users"`  // 通知用户
}

// ========== 预算报告 ==========

// Report 预算报告
type Report struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	GeneratedAt     time.Time         `json:"generated_at"`
	Period          ReportPeriod      `json:"period"`
	Summary         ReportSummary     `json:"summary"`
	BudgetDetails   []Detail          `json:"budget_details"`
	UsageTrend      []UsageTrendPoint `json:"usage_trend"`
	TopConsumers    []TopConsumer     `json:"top_consumers"`
	Alerts          []Alert           `json:"alerts"`
	Recommendations []Recommendation  `json:"recommendations"`
}

// ReportPeriod 报告时间范围
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ReportSummary 预算报告摘要
type ReportSummary struct {
	TotalBudgets      int     `json:"total_budgets"`
	ActiveBudgets     int     `json:"active_budgets"`
	TotalBudgetAmount float64 `json:"total_budget_amount"`
	TotalUsedAmount   float64 `json:"total_used_amount"`
	TotalRemaining    float64 `json:"total_remaining"`
	AvgUsagePercent   float64 `json:"avg_usage_percent"`
	ExceededBudgets   int     `json:"exceeded_budgets"`
	NearLimitBudgets  int     `json:"near_limit_budgets"` // 接近上限的预算数
	ActiveAlerts      int     `json:"active_alerts"`
	HealthScore       int     `json:"health_score"` // 健康评分 0-100
}

// Detail 预算详情
type Detail struct {
	BudgetID       string `json:"budget_id"`
	BudgetName     string `json:"budget_name"`
	Type           Type   `json:"type"`
	Scope          Scope  `json:"scope"`
	TargetName     string `json:"target_name"`
	Amount         float64 `json:"amount"`
	UsedAmount     float64 `json:"used_amount"`
	Remaining      float64 `json:"remaining"`
	UsagePercent   float64 `json:"usage_percent"`
	Status         Status `json:"status"`
	Trend          string `json:"trend"` // up, down, stable
	DailyAvgUsage  float64 `json:"daily_avg_usage"`
	ProjectedUsage float64 `json:"projected_usage"` // 预计期末使用量
	DaysRemaining  int    `json:"days_remaining"`
	Alerts         []Alert `json:"alerts"`
}

// UsageTrendPoint 使用趋势数据点
type UsageTrendPoint struct {
	Date       time.Time `json:"date"`
	UsedAmount float64   `json:"used_amount"`
	Remaining  float64   `json:"remaining"`
	Percent    float64   `json:"percent"`
}

// TopConsumer 消费排行
type TopConsumer struct {
	Rank       int     `json:"rank"`
	BudgetID   string  `json:"budget_id"`
	BudgetName string  `json:"budget_name"`
	Scope      Scope   `json:"scope"`
	TargetName string  `json:"target_name"`
	UsedAmount float64 `json:"used_amount"`
	Percent    float64 `json:"percent"`
	Trend      string  `json:"trend"`
}

// Recommendation 预算建议
type Recommendation struct {
	Type        string  `json:"type"`     // increase, decrease, optimize, alert
	Priority    string  `json:"priority"` // high, medium, low
	BudgetID    string  `json:"budget_id"`
	BudgetName  string  `json:"budget_name"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Current     float64 `json:"current"`
	Suggested   float64 `json:"suggested"`
	Savings     float64 `json:"savings,omitempty"`
	Action      string  `json:"action"`
}

// ReportRequest 报告请求
type ReportRequest struct {
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
	BudgetIDs    []string   `json:"budget_ids,omitempty"`
	Types        []Type     `json:"types,omitempty"`
	Scopes       []Scope    `json:"scopes,omitempty"`
	IncludeUsage bool       `json:"include_usage"`
	IncludeTrend bool       `json:"include_trend"`
}

// ========== 查询参数 ==========

// Query 预算查询参数
type Query struct {
	IDs       []string  `json:"ids,omitempty"`
	Types     []Type    `json:"types,omitempty"`
	Scopes    []Scope   `json:"scopes,omitempty"`
	Statuses  []Status  `json:"statuses,omitempty"`
	TargetIDs []string  `json:"target_ids,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	MinAmount *float64  `json:"min_amount,omitempty"`
	MaxAmount *float64  `json:"max_amount,omitempty"`
	MinUsage  *float64  `json:"min_usage,omitempty"`
	MaxUsage  *float64  `json:"max_usage,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Page      int       `json:"page"`
	PageSize  int       `json:"page_size"`
	SortBy    string    `json:"sort_by"`    // name, amount, used_amount, usage_percent, created_at
	SortOrder string    `json:"sort_order"` // asc, desc
}

// UsageQuery 使用记录查询参数
type UsageQuery struct {
	BudgetID    string     `json:"budget_id,omitempty"`
	SourceTypes []string   `json:"source_types,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	MinAmount   *float64   `json:"min_amount,omitempty"`
	MaxAmount   *float64   `json:"max_amount,omitempty"`
	Page        int        `json:"page"`
	PageSize    int        `json:"page_size"`
}

// AlertQuery 预警查询参数
type AlertQuery struct {
	BudgetIDs []string      `json:"budget_ids,omitempty"`
	Levels    []Level       `json:"levels,omitempty"`
	Statuses  []AlertStatus `json:"statuses,omitempty"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
	Page      int           `json:"page"`
	PageSize  int           `json:"page_size"`
}

// ========== 统计数据 ==========

// Stats 预算统计
type Stats struct {
	TotalBudgets      int                `json:"total_budgets"`
	ActiveBudgets     int                `json:"active_budgets"`
	TotalAmount       float64            `json:"total_amount"`
	TotalUsed         float64            `json:"total_used"`
	TotalRemaining    float64            `json:"total_remaining"`
	ByType            map[Type]TypeStats `json:"by_type"`
	ByScope           map[Scope]TypeStats `json:"by_scope"`
	ExceededCount     int                `json:"exceeded_count"`
	NearLimitCount    int                `json:"near_limit_count"`
	ActiveAlertCount  int                `json:"active_alert_count"`
	HealthScore       int                `json:"health_score"`
	ProjectedMonthEnd float64            `json:"projected_month_end"`
}

// TypeStats 类型统计
type TypeStats struct {
	Count     int     `json:"count"`
	Amount    float64 `json:"amount"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
}

// ========== 默认配置 ==========

// DefaultAlertThresholds 默认预警阈值
var DefaultAlertThresholds = []AlertThreshold{
	{Percent: 50, Level: LevelInfo, Message: "预算已使用 50%"},
	{Percent: 70, Level: LevelWarning, Message: "预算已使用 70%，请注意"},
	{Percent: 85, Level: LevelCritical, Message: "预算已使用 85%，请及时处理"},
	{Percent: 95, Level: LevelEmergency, Message: "预算即将耗尽，请立即处理"},
}

// DefaultAlertConfig 默认预警配置
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		Enabled:           true,
		Thresholds:        DefaultAlertThresholds,
		NotifyEmail:       true,
		NotifyWebhook:     false,
		CooldownMinutes:   60,
		EscalationEnabled: false,
	}
}

// ========== 类型别名（向后兼容） ==========

// BudgetQuery 是 Query 的别名
type BudgetQuery = Query

// BudgetDetail 是 Detail 的别名
type BudgetDetail = Detail

// BudgetRecommendation 是 Recommendation 的别名
type BudgetRecommendation = Recommendation

// BudgetStats 是 Stats 的别名
type BudgetStats = Stats

// BudgetStatus 是 Status 的别名
type BudgetStatus = Status
