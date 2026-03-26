// Package usage provides AI service usage tracking and cost management
// types.go - Core data types for AI usage statistics
package usage

import (
	"time"
)

// ========== Token 计费模型 ==========

// TokenPricingModel Token 计费模式
type TokenPricingModel string

const (
	// TokenPricingModelFixed 固定单价
	TokenPricingModelFixed TokenPricingModel = "fixed"
	// TokenPricingModelTiered 阶梯定价
	TokenPricingModelTiered TokenPricingModel = "tiered"
	// TokenPricingModelDynamic 动态定价（按供需）
	TokenPricingModelDynamic TokenPricingModel = "dynamic"
)

// ModelPricing 模型定价配置
type ModelPricing struct {
	ModelID          string            `json:"model_id"`            // 模型ID
	ModelName        string            `json:"model_name"`          // 模型名称
	Provider         string            `json:"provider"`            // 提供商
	PricingModel     TokenPricingModel `json:"pricing_model"`       // 计费模式
	InputPricePer1K  float64           `json:"input_price_per_1k"`  // 输入token单价（元/千token）
	OutputPricePer1K float64           `json:"output_price_per_1k"` // 输出token单价（元/千token）
	Currency         string            `json:"currency"`            // 货币单位

	// 阶梯定价配置
	TieredPricing []TokenTier `json:"tiered_pricing,omitempty"`

	// 动态定价配置
	DynamicPricingConfig *DynamicPricingConfig `json:"dynamic_pricing_config,omitempty"`

	// 免费额度
	FreeInputTokens  int64 `json:"free_input_tokens"`  // 免费输入token数
	FreeOutputTokens int64 `json:"free_output_tokens"` // 免费输出token数

	// 折扣
	DiscountPercent float64 `json:"discount_percent"` // 折扣百分比

	Enabled     bool       `json:"enabled"`
	EffectiveAt time.Time  `json:"effective_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// TokenTier Token 阶梯定价
type TokenTier struct {
	MinTokens        int64   `json:"min_tokens"`          // 起始token数
	MaxTokens        int64   `json:"max_tokens"`          // 结束token数 (-1表示无限)
	InputPricePer1K  float64 `json:"input_price_per_1k"`  // 输入单价
	OutputPricePer1K float64 `json:"output_price_per_1k"` // 输出单价
}

// DynamicPricingConfig 动态定价配置
type DynamicPricingConfig struct {
	BaseInputPrice  float64 `json:"base_input_price"`  // 基础输入单价
	BaseOutputPrice float64 `json:"base_output_price"` // 基础输出单价
	PeakMultiplier  float64 `json:"peak_multiplier"`   // 高峰期倍率
	OffPeakDiscount float64 `json:"off_peak_discount"` // 低谷期折扣
	PeakHours       []int   `json:"peak_hours"`        // 高峰时段 (0-23)
	WeekendDiscount float64 `json:"weekend_discount"`  // 周末折扣
}

// ========== 使用量记录 ==========

// UsageRecord AI使用量记录
type UsageRecord struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	SessionID string `json:"session_id"` // 会话ID，关联多次请求

	// 请求信息
	RequestType RequestType `json:"request_type"` // chat, embed, completion
	ModelID     string      `json:"model_id"`     // 模型ID
	ModelName   string      `json:"model_name"`   // 模型名称
	Provider    string      `json:"provider"`     // 提供商
	BackendType string      `json:"backend_type"` // 后端类型 (ollama, openai, etc.)

	// Token 使用量
	InputTokens  int64 `json:"input_tokens"`  // 输入token数
	OutputTokens int64 `json:"output_tokens"` // 输出token数
	TotalTokens  int64 `json:"total_tokens"`  // 总token数

	// 计费信息
	InputCost    float64 `json:"input_cost"`    // 输入成本
	OutputCost   float64 `json:"output_cost"`   // 输出成本
	TotalCost    float64 `json:"total_cost"`    // 总成本
	Currency     string  `json:"currency"`      // 货币
	BilledAmount float64 `json:"billed_amount"` // 实际计费金额（扣除免费额度后）

	// 请求详情
	RequestDuration time.Duration `json:"request_duration"`        // 请求耗时
	Success         bool          `json:"success"`                 // 是否成功
	ErrorMessage    string        `json:"error_message,omitempty"` // 错误信息
	Streaming       bool          `json:"streaming"`               // 是否流式
	FinishReason    string        `json:"finish_reason"`           // 完成原因

	// 元数据
	PromptHash   string                 `json:"prompt_hash,omitempty"`   // Prompt哈希（用于去重）
	ResponseHash string                 `json:"response_hash,omitempty"` // 响应哈希
	Metadata     map[string]interface{} `json:"metadata,omitempty"`      // 扩展元数据
	Labels       map[string]string      `json:"labels,omitempty"`        // 标签

	// 时间信息
	Timestamp time.Time `json:"timestamp"`  // 请求时间
	CreatedAt time.Time `json:"created_at"` // 记录创建时间
}

// RequestType 请求类型
type RequestType string

const (
	RequestTypeChat       RequestType = "chat"
	RequestTypeCompletion RequestType = "completion"
	RequestTypeEmbed      RequestType = "embed"
	RequestTypeImage      RequestType = "image"      // 图像生成
	RequestTypeAudio      RequestType = "audio"      // 语音处理
	RequestTypeModeration RequestType = "moderation" // 内容审核
)

// ========== 用户配额 ==========

// UserQuota 用户AI服务配额
type UserQuota struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`

	// Token 配额
	TokenQuotaPerDay   int64 `json:"token_quota_per_day"`   // 每日token配额
	TokenQuotaPerWeek  int64 `json:"token_quota_per_week"`  // 每周token配额
	TokenQuotaPerMonth int64 `json:"token_quota_per_month"` // 每月token配额
	TokenQuotaTotal    int64 `json:"token_quota_total"`     // 总配额 (-1表示无限)

	// 成本配额
	CostQuotaPerDay   float64 `json:"cost_quota_per_day"`   // 每日成本配额
	CostQuotaPerWeek  float64 `json:"cost_quota_per_week"`  // 每周成本配额
	CostQuotaPerMonth float64 `json:"cost_quota_per_month"` // 每月成本配额
	CostQuotaTotal    float64 `json:"cost_quota_total"`     // 总成本配额

	// 请求配额
	RequestQuotaPerDay   int `json:"request_quota_per_day"`   // 每日请求配额
	RequestQuotaPerHour  int `json:"request_quota_per_hour"`  // 每小时请求配额
	RequestQuotaPerMonth int `json:"request_quota_per_month"` // 每月请求配额

	// 模型限制
	AllowedModels   []string `json:"allowed_models"`     // 允许使用的模型 (空表示全部)
	BlockedModels   []string `json:"blocked_models"`     // 禁止使用的模型
	MaxTokensPerReq int64    `json:"max_tokens_per_req"` // 单次请求最大token数

	// 当期使用量
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	TokensUsed         int64     `json:"tokens_used"`   // 当期已用token
	CostUsed           float64   `json:"cost_used"`     // 当期已用成本
	RequestsUsed       int       `json:"requests_used"` // 当期已用请求数

	// 告警阈值
	AlertThresholdPercent float64 `json:"alert_threshold_percent"` // 告警阈值百分比

	// 状态
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// QuotaUsage 配额使用情况
type QuotaUsage struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// Token使用
	TokenQuota        int64   `json:"token_quota"`
	TokensUsed        int64   `json:"tokens_used"`
	TokensRemaining   int64   `json:"tokens_remaining"`
	TokenUsagePercent float64 `json:"token_usage_percent"`

	// 成本使用
	CostQuota        float64 `json:"cost_quota"`
	CostUsed         float64 `json:"cost_used"`
	CostRemaining    float64 `json:"cost_remaining"`
	CostUsagePercent float64 `json:"cost_usage_percent"`

	// 请求使用
	RequestQuota        int     `json:"request_quota"`
	RequestsUsed        int     `json:"requests_used"`
	RequestsRemaining   int     `json:"requests_remaining"`
	RequestUsagePercent float64 `json:"request_usage_percent"`

	// 状态
	IsOverTokenLimit   bool `json:"is_over_token_limit"`
	IsOverCostLimit    bool `json:"is_over_cost_limit"`
	IsOverRequestLimit bool `json:"is_over_request_limit"`
	IsAlertTriggered   bool `json:"is_alert_triggered"`
}

// ========== 使用量汇总 ==========

// UsageSummary 使用量汇总
type UsageSummary struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// Token 汇总
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	AvgTokensPerReq   float64 `json:"avg_tokens_per_req"`
	MaxTokensPerReq   int64   `json:"max_tokens_per_req"`
	MinTokensPerReq   int64   `json:"min_tokens_per_req"`

	// 成本汇总
	TotalInputCost  float64 `json:"total_input_cost"`
	TotalOutputCost float64 `json:"total_output_cost"`
	TotalCost       float64 `json:"total_cost"`
	AvgCostPerReq   float64 `json:"avg_cost_per_req"`
	AvgCostPerToken float64 `json:"avg_cost_per_token"`

	// 请求汇总
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	StreamingRequests int64   `json:"streaming_requests"`

	// 请求类型分布
	RequestsByType map[RequestType]int64 `json:"requests_by_type"`

	// 模型使用分布
	ModelsUsed []ModelUsage `json:"models_used"`

	// 时段分布
	HourlyDistribution map[int]int64 `json:"hourly_distribution"` // 按小时分布

	// 性能指标
	AvgLatencyMs int64 `json:"avg_latency_ms"`
	MaxLatencyMs int64 `json:"max_latency_ms"`
	MinLatencyMs int64 `json:"min_latency_ms"`
	P95LatencyMs int64 `json:"p95_latency_ms"`
}

// ModelUsage 模型使用情况
type ModelUsage struct {
	ModelID      string  `json:"model_id"`
	ModelName    string  `json:"model_name"`
	Provider     string  `json:"provider"`
	RequestCount int64   `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
}

// ========== 成本分摊 ==========

// CostAllocation 成本分摊配置
type CostAllocation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`

	// 分摊模式
	AllocationType AllocationType `json:"allocation_type"`

	// 按用户分摊
	UserAllocations []UserAllocation `json:"user_allocations,omitempty"`

	// 按部门分摊
	DepartmentAllocations []DepartmentAllocation `json:"department_allocations,omitempty"`

	// 按项目分摊
	ProjectAllocations []ProjectAllocation `json:"project_allocations,omitempty"`

	// 共享成本
	SharedCostRatio float64 `json:"shared_cost_ratio"` // 共享成本比例

	// 预算
	BudgetAmount   float64 `json:"budget_amount"`   // 预算总额
	AlertThreshold float64 `json:"alert_threshold"` // 预算告警阈值
}

// AllocationType 分摊类型
type AllocationType string

const (
	AllocationTypeUser       AllocationType = "user"       // 按用户分摊
	AllocationTypeDepartment AllocationType = "department" // 按部门分摊
	AllocationTypeProject    AllocationType = "project"    // 按项目分摊
	AllocationTypeUsage      AllocationType = "usage"      // 按使用量分摊
)

// UserAllocation 用户成本分摊
type UserAllocation struct {
	UserID      string  `json:"user_id"`
	UserName    string  `json:"user_name"`
	Ratio       float64 `json:"ratio"`        // 分摊比例
	FixedAmount float64 `json:"fixed_amount"` // 固定金额
}

// DepartmentAllocation 部门成本分摊
type DepartmentAllocation struct {
	DepartmentID   string   `json:"department_id"`
	DepartmentName string   `json:"department_name"`
	Ratio          float64  `json:"ratio"`
	FixedAmount    float64  `json:"fixed_amount"`
	Users          []string `json:"users"` // 部门成员
}

// ProjectAllocation 项目成本分摊
type ProjectAllocation struct {
	ProjectID   string            `json:"project_id"`
	ProjectName string            `json:"project_name"`
	Ratio       float64           `json:"ratio"`
	FixedAmount float64           `json:"fixed_amount"`
	Users       []string          `json:"users"`  // 项目成员
	Labels      map[string]string `json:"labels"` // 项目标签
}

// CostAllocationResult 成本分摊结果
type CostAllocationResult struct {
	AllocationID string    `json:"allocation_id"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	TotalCost    float64   `json:"total_cost"`
	Currency     string    `json:"currency"`

	// 分摊明细
	UserAllocations       []UserCostAllocation       `json:"user_allocations"`
	DepartmentAllocations []DepartmentCostAllocation `json:"department_allocations,omitempty"`
	ProjectAllocations    []ProjectCostAllocation    `json:"project_allocations,omitempty"`

	// 汇总
	SharedCost    float64   `json:"shared_cost"`
	AllocatedCost float64   `json:"allocated_cost"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// UserCostAllocation 用户成本分摊明细
type UserCostAllocation struct {
	UserID        string  `json:"user_id"`
	UserName      string  `json:"user_name"`
	UsageCost     float64 `json:"usage_cost"`     // 使用成本
	AllocatedCost float64 `json:"allocated_cost"` // 分摊成本
	TotalCost     float64 `json:"total_cost"`     // 总成本
	Ratio         float64 `json:"ratio"`          // 分摊比例
}

// DepartmentCostAllocation 部门成本分摊明细
type DepartmentCostAllocation struct {
	DepartmentID   string  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	UsageCost      float64 `json:"usage_cost"`
	AllocatedCost  float64 `json:"allocated_cost"`
	TotalCost      float64 `json:"total_cost"`
	Ratio          float64 `json:"ratio"`
	UserCount      int     `json:"user_count"`
}

// ProjectCostAllocation 项目成本分摊明细
type ProjectCostAllocation struct {
	ProjectID     string  `json:"project_id"`
	ProjectName   string  `json:"project_name"`
	UsageCost     float64 `json:"usage_cost"`
	AllocatedCost float64 `json:"allocated_cost"`
	TotalCost     float64 `json:"total_cost"`
	Ratio         float64 `json:"ratio"`
	UserCount     int     `json:"user_count"`
}

// ========== 使用报告 ==========

// UsageReport 使用报告
type UsageReport struct {
	ID          string       `json:"id"`
	Type        ReportType   `json:"type"`
	Format      ReportFormat `json:"format"`
	GeneratedAt time.Time    `json:"generated_at"`
	Period      ReportPeriod `json:"period"`

	// 报告内容
	Summary UsageReportSummary `json:"summary"`
	Details interface{}        `json:"details,omitempty"`
	Charts  []ReportChart      `json:"charts,omitempty"`

	// 导出信息
	ExportURL string `json:"export_url,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
}

// ReportType 报告类型
type ReportType string

const (
	ReportTypeUser    ReportType = "user"    // 用户报告
	ReportTypeModel   ReportType = "model"   // 模型报告
	ReportTypeCost    ReportType = "cost"    // 成本报告
	ReportTypeTrend   ReportType = "trend"   // 趋势报告
	ReportTypeSummary ReportType = "summary" // 汇总报告
	ReportTypeAudit   ReportType = "audit"   // 审计报告
)

// ReportFormat 报告格式
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatCSV  ReportFormat = "csv"
	ReportFormatHTML ReportFormat = "html"
	ReportFormatPDF  ReportFormat = "pdf"
)

// ReportPeriod 报告周期
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// UsageReportSummary 报告摘要
type UsageReportSummary struct {
	TotalUsers       int              `json:"total_users"`
	TotalModels      int              `json:"total_models"`
	TotalRequests    int64            `json:"total_requests"`
	TotalTokens      int64            `json:"total_tokens"`
	TotalCost        float64          `json:"total_cost"`
	AvgCostPerUser   float64          `json:"avg_cost_per_user"`
	AvgTokensPerUser int64            `json:"avg_tokens_per_user"`
	TopUsers         []UserUsageRank  `json:"top_users"`
	TopModels        []ModelUsageRank `json:"top_models"`
}

// UserUsageRank 用户使用排名
type UserUsageRank struct {
	Rank       int     `json:"rank"`
	UserID     string  `json:"user_id"`
	UserName   string  `json:"user_name"`
	TokensUsed int64   `json:"tokens_used"`
	Cost       float64 `json:"cost"`
	Requests   int64   `json:"requests"`
}

// ModelUsageRank 模型使用排名
type ModelUsageRank struct {
	Rank       int     `json:"rank"`
	ModelID    string  `json:"model_id"`
	ModelName  string  `json:"model_name"`
	TokensUsed int64   `json:"tokens_used"`
	Cost       float64 `json:"cost"`
	Requests   int64   `json:"requests"`
}

// ReportChart 报告图表
type ReportChart struct {
	Type        ChartType   `json:"type"`
	Title       string      `json:"title"`
	Data        interface{} `json:"data"`
	Description string      `json:"description,omitempty"`
}

// ChartType 图表类型
type ChartType string

const (
	ChartTypeLine    ChartType = "line"
	ChartTypeBar     ChartType = "bar"
	ChartTypePie     ChartType = "pie"
	ChartTypeArea    ChartType = "area"
	ChartTypeHeatmap ChartType = "heatmap"
)

// ========== 告警 ==========

// UsageAlert 使用量告警
type UsageAlert struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	UserName   string      `json:"user_name"`
	AlertType  AlertType   `json:"alert_type"`
	AlertLevel AlertLevel  `json:"alert_level"`
	Status     AlertStatus `json:"status"`

	// 告警详情
	ThresholdType  ThresholdType `json:"threshold_type"`
	ThresholdValue float64       `json:"threshold_value"`
	CurrentValue   float64       `json:"current_value"`
	UsagePercent   float64       `json:"usage_percent"`

	// 时间
	TriggeredAt    time.Time  `json:"triggered_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`

	// 通知
	Notified       bool     `json:"notified"`
	NotifyChannels []string `json:"notify_channels"`

	// 消息
	Message string `json:"message"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeTokenQuota   AlertType = "token_quota"   // Token配额告警
	AlertTypeCostQuota    AlertType = "cost_quota"    // 成本配额告警
	AlertTypeRequestQuota AlertType = "request_quota" // 请求配额告警
	AlertTypeBudget       AlertType = "budget"        // 预算告警
	AlertTypeAnomaly      AlertType = "anomaly"       // 异常使用告警
	AlertTypeFailure      AlertType = "failure"       // 失败率告警
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo      AlertLevel = "info"
	AlertLevelWarning   AlertLevel = "warning"
	AlertLevelCritical  AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusSilenced     AlertStatus = "silenced"
)

// ThresholdType 阈值类型
type ThresholdType string

const (
	ThresholdTypePercent  ThresholdType = "percent"  // 百分比
	ThresholdTypeAbsolute ThresholdType = "absolute" // 绝对值
)

// ========== 配置 ==========

// UsageConfig 使用量统计配置
type UsageConfig struct {
	Enabled             bool          `json:"enabled"`
	DataDir             string        `json:"data_dir"`             // 数据存储目录
	RetentionDays       int           `json:"retention_days"`       // 数据保留天数
	AggregationInterval time.Duration `json:"aggregation_interval"` // 聚合间隔

	// 计费配置
	DefaultCurrency string `json:"default_currency"`

	// 配额配置
	DefaultUserQuota *UserQuotaConfig `json:"default_user_quota"`

	// 告警配置
	AlertConfig *UsageAlertConfig `json:"alert_config"`

	// 报告配置
	ReportConfig *UsageReportConfig `json:"report_config"`
}

// UserQuotaConfig 用户配额默认配置
type UserQuotaConfig struct {
	TokenQuotaPerDay      int64   `json:"token_quota_per_day"`
	TokenQuotaPerMonth    int64   `json:"token_quota_per_month"`
	CostQuotaPerDay       float64 `json:"cost_quota_per_day"`
	CostQuotaPerMonth     float64 `json:"cost_quota_per_month"`
	RequestQuotaPerDay    int     `json:"request_quota_per_day"`
	RequestQuotaPerMonth  int     `json:"request_quota_per_month"`
	AlertThresholdPercent float64 `json:"alert_threshold_percent"`
}

// UsageAlertConfig 使用量告警配置
type UsageAlertConfig struct {
	Enabled            bool          `json:"enabled"`
	CheckInterval      time.Duration `json:"check_interval"`
	WarningThreshold   float64       `json:"warning_threshold"`   // 70%
	CriticalThreshold  float64       `json:"critical_threshold"`  // 85%
	EmergencyThreshold float64       `json:"emergency_threshold"` // 95%
	NotifyChannels     []string      `json:"notify_channels"`
	SilenceDuration    time.Duration `json:"silence_duration"`
}

// UsageReportConfig 报告配置
type UsageReportConfig struct {
	Enabled          bool          `json:"enabled"`
	AutoGenerate     bool          `json:"auto_generate"`
	GenerateInterval time.Duration `json:"generate_interval"` // 自动生成间隔
	DefaultFormat    ReportFormat  `json:"default_format"`
	ReportTypes      []ReportType  `json:"report_types"`
}

// DefaultUsageConfig 默认配置
func DefaultUsageConfig() *UsageConfig {
	return &UsageConfig{
		Enabled:             true,
		DataDir:             "/var/lib/nas-os/ai/usage",
		RetentionDays:       365,
		AggregationInterval: time.Hour,
		DefaultCurrency:     "CNY",
		DefaultUserQuota: &UserQuotaConfig{
			TokenQuotaPerDay:      100000,  // 10万token/天
			TokenQuotaPerMonth:    2000000, // 200万token/月
			CostQuotaPerDay:       10.0,    // 10元/天
			CostQuotaPerMonth:     200.0,   // 200元/月
			RequestQuotaPerDay:    1000,    // 1000次/天
			RequestQuotaPerMonth:  20000,   // 2万次/月
			AlertThresholdPercent: 80.0,    // 80%告警
		},
		AlertConfig: &UsageAlertConfig{
			Enabled:            true,
			CheckInterval:      5 * time.Minute,
			WarningThreshold:   70.0,
			CriticalThreshold:  85.0,
			EmergencyThreshold: 95.0,
			NotifyChannels:     []string{"email", "webhook"},
			SilenceDuration:    30 * time.Minute,
		},
		ReportConfig: &UsageReportConfig{
			Enabled:          true,
			AutoGenerate:     true,
			GenerateInterval: 24 * time.Hour,
			DefaultFormat:    ReportFormatJSON,
			ReportTypes:      []ReportType{ReportTypeSummary, ReportTypeCost, ReportTypeTrend},
		},
	}
}
