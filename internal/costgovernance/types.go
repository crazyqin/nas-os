// Package costgovernance 提供多云成本治理功能
package costgovernance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPolicyNotFound 成本策略不存在.
	ErrPolicyNotFound = errors.New("成本策略不存在")
	// ErrBudgetNotFound 预算不存在.
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrAlertNotFound 告警不存在.
	ErrAlertNotFound = errors.New("告警不存在")
	// ErrProviderUnsupported 不支持的云厂商.
	ErrProviderUnsupported = errors.New("不支持的云厂商")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
)

// ========== 云厂商类型 ==========

// CloudProvider 云厂商类型.
type CloudProvider string

const (
	// ProviderAWS 亚马逊云.
	ProviderAWS CloudProvider = "aws"
	// ProviderAzure 微软Azure.
	ProviderAzure CloudProvider = "azure"
	// ProviderGCP 谷歌云.
	ProviderGCP CloudProvider = "gcp"
	// ProviderAliyun 阿里云.
	ProviderAliyun CloudProvider = "aliyun"
)

// ========== 资源类型 ==========

// ResourceType 资源类型.
type ResourceType string

const (
	// ResourceCompute 计算资源.
	ResourceCompute ResourceType = "compute"
	// ResourceStorage 存储资源.
	ResourceStorage ResourceType = "storage"
	// ResourceNetwork 网络资源.
	ResourceNetwork ResourceType = "network"
	// ResourceDatabase 数据库资源.
	ResourceDatabase ResourceType = "database"
	// ResourceOther 其他资源.
	ResourceOther ResourceType = "other"
)

// ========== 告警级别 ==========

// AlertSeverity 告警级别.
type AlertSeverity string

const (
	// SeverityInfo 信息.
	SeverityInfo AlertSeverity = "info"
	// SeverityWarning 警告.
	SeverityWarning AlertSeverity = "warning"
	// SeverityCritical 严重.
	SeverityCritical AlertSeverity = "critical"
)

// ========== 预算周期 ==========

// BudgetPeriod 预算周期.
type BudgetPeriod string

const (
	// PeriodMonthly 月度.
	PeriodMonthly BudgetPeriod = "monthly"
	// PeriodQuarterly 季度.
	PeriodQuarterly BudgetPeriod = "quarterly"
	// PeriodYearly 年度.
	PeriodYearly BudgetPeriod = "yearly"
)

// ========== 核心数据结构 ==========

// CostPolicy 成本策略.
type CostPolicy struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Provider    CloudProvider `json:"provider"`
	// MaxMonthlySpend 月度最大支出（元）.
	MaxMonthlySpend float64 `json:"max_monthly_spend"`
	// MaxResourceCount 最大资源数量.
	MaxResourceCount int `json:"max_resource_count"`
	// AllowedRegions 允许的区域.
	AllowedRegions []string `json:"allowed_regions"`
	// TagRequirements 标签要求.
	TagRequirements map[string]string `json:"tag_requirements"`
	Enabled         bool              `json:"enabled"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Budget 预算.
type Budget struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Provider CloudProvider `json:"provider"`
	// Amount 预算金额（元）.
	Amount float64 `json:"amount"`
	// Spent 已花费金额（元）.
	Spent  float64      `json:"spent"`
	Period BudgetPeriod `json:"period"`
	// AlertThresholds 告警阈值百分比列表，如 [50, 80, 100].
	AlertThresholds []float64 `json:"alert_thresholds"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	CreatedAt       time.Time `json:"created_at"`
}

// CostAlert 成本告警.
type CostAlert struct {
	ID        string        `json:"id"`
	BudgetID  string        `json:"budget_id"`
	Provider  CloudProvider `json:"provider"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	Threshold float64       `json:"threshold"`
	Actual    float64       `json:"actual"`
	// Acknowledged 是否已确认.
	Acknowledged bool      `json:"acknowledged"`
	CreatedAt    time.Time `json:"created_at"`
}

// ResourceUsage 资源使用情况.
type ResourceUsage struct {
	ID           string        `json:"id"`
	ResourceID   string        `json:"resource_id"`
	ResourceName string        `json:"resource_name"`
	ResourceType ResourceType  `json:"resource_type"`
	Provider     CloudProvider `json:"provider"`
	Region       string        `json:"region"`
	// CPUPercent CPU使用率百分比.
	CPUPercent float64 `json:"cpu_percent"`
	// MemoryPercent 内存使用率百分比.
	MemoryPercent float64 `json:"memory_percent"`
	// StorageUsedGB 存储使用量（GB）.
	StorageUsedGB float64 `json:"storage_used_gb"`
	// StorageTotalGB 存储总量（GB）.
	StorageTotalGB float64 `json:"storage_total_gb"`
	// DailyCost 每日成本（元）.
	DailyCost float64           `json:"daily_cost"`
	Tags      map[string]string `json:"tags"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// CostReport 成本报表.
type CostReport struct {
	ID       string        `json:"id"`
	Provider CloudProvider `json:"provider"`
	// PeriodStart 报表周期开始.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 报表周期结束.
	PeriodEnd time.Time `json:"period_end"`
	// TotalCost 总成本（元）.
	TotalCost float64 `json:"total_cost"`
	// ByService 按服务分类成本.
	ByService map[string]float64 `json:"by_service"`
	// ByRegion 按区域分类成本.
	ByRegion map[string]float64 `json:"by_region"`
	// ByResourceType 按资源类型分类成本.
	ByResourceType map[ResourceType]float64 `json:"by_resource_type"`
	// OptimizationSavings 优化建议可节省金额（元）.
	OptimizationSavings float64   `json:"optimization_savings"`
	GeneratedAt         time.Time `json:"generated_at"`
}

// CostTrend 成本趋势数据点.
type CostTrend struct {
	Date     time.Time `json:"date"`
	Cost     float64   `json:"cost"`
	Provider string    `json:"provider"`
}

// AnomalyDetection 异常检测结果.
type AnomalyDetection struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	DetectedAt   time.Time `json:"detected_at"`
	ExpectedCost float64   `json:"expected_cost"`
	ActualCost   float64   `json:"actual_cost"`
	Deviation    float64   `json:"deviation"` // 偏差百分比
	Description  string    `json:"description"`
}
