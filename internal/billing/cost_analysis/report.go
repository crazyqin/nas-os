// Package cost_analysis 提供成本分析报告功能
package cost_analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 成本分析报告类型 ==========

// CostReportType 成本报告类型
type CostReportType string

const (
	CostReportStorageTrend   CostReportType = "storage_trend"   // 存储成本趋势分析
	CostReportResourceUtil   CostReportType = "resource_util"   // 资源利用率报告
	CostReportOptimization   CostReportType = "optimization"    // 成本优化建议
	CostReportBudgetTracking CostReportType = "budget_tracking" // 预算跟踪报告
	CostReportComprehensive  CostReportType = "comprehensive"   // 综合成本分析报告
)

// CostReport 成本分析报告
type CostReport struct {
	ID              string                 `json:"id"`
	Type            CostReportType         `json:"type"`
	GeneratedAt     time.Time              `json:"generated_at"`
	PeriodStart     time.Time              `json:"period_start"`
	PeriodEnd       time.Time              `json:"period_end"`
	Summary         CostSummary            `json:"summary"`
	Details         interface{}            `json:"details,omitempty"`
	Trends          []CostTrend            `json:"trends,omitempty"`
	Recommendations []CostRecommendation   `json:"recommendations,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// CostSummary 成本摘要
type CostSummary struct {
	TotalCost            float64 `json:"total_cost"`             // 总成本（元）
	StorageCost          float64 `json:"storage_cost"`           // 存储成本
	BandwidthCost        float64 `json:"bandwidth_cost"`         // 带宽成本
	OtherCost            float64 `json:"other_cost"`             // 其他成本
	Currency             string  `json:"currency"`               // 货币单位
	CostChangePercent    float64 `json:"cost_change_percent"`    // 成本变化百分比
	AvgDailyCost         float64 `json:"avg_daily_cost"`         // 平均日成本
	ProjectedMonthlyCost float64 `json:"projected_monthly_cost"` // 预计月成本
	BudgetUtilization    float64 `json:"budget_utilization"`     // 预算使用率
}

// CostTrend 成本趋势
type CostTrend struct {
	Date          time.Time `json:"date"`
	StorageCost   float64   `json:"storage_cost"`
	BandwidthCost float64   `json:"bandwidth_cost"`
	TotalCost     float64   `json:"total_cost"`
	StorageUsedGB float64   `json:"storage_used_gb"`
	BandwidthGB   float64   `json:"bandwidth_gb"`
}

// CostRecommendation 成本优化建议
type CostRecommendation struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`     // storage, bandwidth, user, pool
	Priority         string  `json:"priority"` // high, medium, low
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	PotentialSavings float64 `json:"potential_savings"` // 预计节省金额（元/月）
	Impact           string  `json:"impact"`            // 影响范围
	Action           string  `json:"action"`            // 建议操作
	Implemented      bool    `json:"implemented"`       // 是否已实施
}

// ========== 存储成本趋势分析 ==========

// StorageTrendAnalysis 存储成本趋势分析
type StorageTrendAnalysis struct {
	AnalysisDate   time.Time            `json:"analysis_date"`
	PeriodDays     int                  `json:"period_days"`
	CurrentUsage   StorageUsageSnapshot `json:"current_usage"`
	TrendData      []StorageTrendPoint  `json:"trend_data"`
	GrowthRate     StorageGrowthRate    `json:"growth_rate"`
	CostProjection CostProjection       `json:"cost_projection"`
	PoolAnalysis   []PoolCostAnalysis   `json:"pool_analysis"`
	UserAnalysis   []UserCostAnalysis   `json:"user_analysis"`
	Alerts         []CostAlert          `json:"alerts,omitempty"`
}

// StorageUsageSnapshot 存储使用快照
type StorageUsageSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalBytes   uint64    `json:"total_bytes"`
	UsedBytes    uint64    `json:"used_bytes"`
	FreeBytes    uint64    `json:"free_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	CostPerGB    float64   `json:"cost_per_gb"`
	MonthlyCost  float64   `json:"monthly_cost"`
}

// StorageTrendPoint 存储趋势数据点
type StorageTrendPoint struct {
	Date         time.Time `json:"date"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsedGB       float64   `json:"used_gb"`
	UsagePercent float64   `json:"usage_percent"`
	DailyGrowth  float64   `json:"daily_growth"` // 日增长量（GB）
	CostPerGB    float64   `json:"cost_per_gb"`
	DailyCost    float64   `json:"daily_cost"`
}

// StorageGrowthRate 存储增长率
type StorageGrowthRate struct {
	DailyGrowthBytes   uint64  `json:"daily_growth_bytes"`
	DailyGrowthGB      float64 `json:"daily_growth_gb"`
	DailyGrowthPercent float64 `json:"daily_growth_percent"`
	WeeklyGrowthBytes  uint64  `json:"weekly_growth_bytes"`
	MonthlyGrowthBytes uint64  `json:"monthly_growth_bytes"`
	TrendDirection     string  `json:"trend_direction"` // increasing, decreasing, stable
}

// CostProjection 成本预测
type CostProjection struct {
	CurrentMonthlyCost   float64  `json:"current_monthly_cost"`
	ProjectedNextMonth   float64  `json:"projected_next_month"`
	ProjectedNextQuarter float64  `json:"projected_next_quarter"`
	ProjectedNextYear    float64  `json:"projected_next_year"`
	ConfidenceLevel      float64  `json:"confidence_level"` // 0-1
	Assumptions          []string `json:"assumptions"`
}

// PoolCostAnalysis 存储池成本分析
type PoolCostAnalysis struct {
	PoolID            string  `json:"pool_id"`
	PoolName          string  `json:"pool_name"`
	StorageType       string  `json:"storage_type"` // ssd, hdd, archive
	TotalBytes        uint64  `json:"total_bytes"`
	UsedBytes         uint64  `json:"used_bytes"`
	UsagePercent      float64 `json:"usage_percent"`
	PricePerGB        float64 `json:"price_per_gb"`
	MonthlyCost       float64 `json:"monthly_cost"`
	CostEfficiency    float64 `json:"cost_efficiency"` // 成本效率评分
	OptimizationScore float64 `json:"optimization_score"`
}

// UserCostAnalysis 用户成本分析
type UserCostAnalysis struct {
	UserID       string   `json:"user_id"`
	UserName     string   `json:"user_name"`
	TotalUsedGB  float64  `json:"total_used_gb"`
	MonthlyCost  float64  `json:"monthly_cost"`
	QuotaLimitGB float64  `json:"quota_limit_gb"`
	QuotaUsage   float64  `json:"quota_usage"`
	TopPools     []string `json:"top_pools"`
	GrowthRate   float64  `json:"growth_rate"`
}

// CostAlert 成本告警
type CostAlert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`     // budget_exceeded, growth_anomaly, inefficiency
	Severity     string    `json:"severity"` // info, warning, critical
	Message      string    `json:"message"`
	Value        float64   `json:"value"`
	Threshold    float64   `json:"threshold"`
	CreatedAt    time.Time `json:"created_at"`
	Acknowledged bool      `json:"acknowledged"`
}

// ========== 资源利用率报告 ==========

// ResourceUtilizationReport 资源利用率报告
type ResourceUtilizationReport struct {
	GeneratedAt     time.Time                   `json:"generated_at"`
	PeriodStart     time.Time                   `json:"period_start"`
	PeriodEnd       time.Time                   `json:"period_end"`
	OverallScore    float64                     `json:"overall_score"` // 整体利用率评分
	StorageUtil     StorageUtilization          `json:"storage_utilization"`
	BandwidthUtil   BandwidthUtilization        `json:"bandwidth_utilization"`
	UserUtilization []UserUtilStats             `json:"user_utilization"`
	PoolUtilization []PoolUtilStats             `json:"pool_utilization"`
	IdleResources   []IdleResource              `json:"idle_resources"`
	Underutilized   []UnderutilizedResource     `json:"underutilized"`
	Recommendations []UtilizationRecommendation `json:"recommendations"`
}

// StorageUtilization 存储利用率
type StorageUtilization struct {
	TotalCapacity   uint64  `json:"total_capacity"`
	UsedCapacity    uint64  `json:"used_capacity"`
	FreeCapacity    uint64  `json:"free_capacity"`
	UtilizationRate float64 `json:"utilization_rate"`
	AvgUtilization  float64 `json:"avg_utilization"`  // 平均利用率
	PeakUtilization float64 `json:"peak_utilization"` // 峰值利用率
	MinUtilization  float64 `json:"min_utilization"`  // 最低利用率
	UtilTrend       string  `json:"util_trend"`       // up, down, stable
	WastedCapacity  uint64  `json:"wasted_capacity"`  // 浪费容量（低利用率）
	WastedCost      float64 `json:"wasted_cost"`      // 浪费成本
}

// BandwidthUtilization 带宽利用率
type BandwidthUtilization struct {
	TotalAllocated  uint64  `json:"total_allocated"` // 分配带宽（Mbps）
	PeakUsage       uint64  `json:"peak_usage"`      // 峰值使用
	AvgUsage        uint64  `json:"avg_usage"`       // 平均使用
	UtilizationRate float64 `json:"utilization_rate"`
	InboundBytes    uint64  `json:"inbound_bytes"`
	OutboundBytes   uint64  `json:"outbound_bytes"`
	TotalBytes      uint64  `json:"total_bytes"`
	CostPerGB       float64 `json:"cost_per_gb"`
	MonthlyCost     float64 `json:"monthly_cost"`
}

// UserUtilStats 用户利用率统计
type UserUtilStats struct {
	UserID           string     `json:"user_id"`
	UserName         string     `json:"user_name"`
	QuotaAllocated   uint64     `json:"quota_allocated"`
	QuotaUsed        uint64     `json:"quota_used"`
	UtilizationRate  float64    `json:"utilization_rate"`
	Status           string     `json:"status"` // active, idle, underutilized
	LastActive       *time.Time `json:"last_active,omitempty"`
	SavingsPotential float64    `json:"savings_potential"` // 潜在节省
}

// PoolUtilStats 存储池利用率统计
type PoolUtilStats struct {
	PoolID          string  `json:"pool_id"`
	PoolName        string  `json:"pool_name"`
	StorageType     string  `json:"storage_type"`
	TotalCapacity   uint64  `json:"total_capacity"`
	UsedCapacity    uint64  `json:"used_capacity"`
	UtilizationRate float64 `json:"utilization_rate"`
	Performance     string  `json:"performance"` // good, fair, poor
	CostEfficiency  float64 `json:"cost_efficiency"`
}

// IdleResource 闲置资源
type IdleResource struct {
	ResourceType string     `json:"resource_type"` // quota, pool, directory
	ResourceID   string     `json:"resource_id"`
	ResourceName string     `json:"resource_name"`
	Capacity     uint64     `json:"capacity"`
	WastedCost   float64    `json:"wasted_cost"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	DaysIdle     int        `json:"days_idle"`
	Reason       string     `json:"reason"`
}

// UnderutilizedResource 低利用率资源
type UnderutilizedResource struct {
	ResourceID       string  `json:"resource_id"`
	ResourceName     string  `json:"resource_name"`
	ResourceType     string  `json:"resource_type"`
	Allocated        uint64  `json:"allocated"`
	Used             uint64  `json:"used"`
	UtilizationRate  float64 `json:"utilization_rate"`
	Threshold        float64 `json:"threshold"`
	SavingsPotential float64 `json:"savings_potential"`
}

// UtilizationRecommendation 利用率建议
type UtilizationRecommendation struct {
	Type        string  `json:"type"` // consolidate, deallocate, optimize
	Priority    string  `json:"priority"`
	Description string  `json:"description"`
	Savings     float64 `json:"savings"`
	Effort      string  `json:"effort"` // low, medium, high
	Impact      string  `json:"impact"`
}

// ========== 预算跟踪功能 ==========

// BudgetConfig 预算配置
type BudgetConfig struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	TotalBudget     float64          `json:"total_budget"` // 总预算（元）
	Period          string           `json:"period"`       // monthly, quarterly, yearly
	StartDate       time.Time        `json:"start_date"`
	EndDate         time.Time        `json:"end_date"`
	AlertThresholds []float64        `json:"alert_thresholds"` // 告警阈值（百分比）
	Categories      []BudgetCategory `json:"categories"`
	NotifyEmails    []string         `json:"notify_emails"`
	NotifyWebhooks  []string         `json:"notify_webhooks"`
	Enabled         bool             `json:"enabled"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// BudgetCategory 预算分类
type BudgetCategory struct {
	Name         string  `json:"name"`          // storage, bandwidth, etc.
	Budget       float64 `json:"budget"`        // 该分类预算
	CurrentSpend float64 `json:"current_spend"` // 当前支出
	Percentage   float64 `json:"percentage"`    // 占总预算百分比
}

// BudgetTrackingReport 预算跟踪报告
type BudgetTrackingReport struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	BudgetID         string             `json:"budget_id"`
	BudgetName       string             `json:"budget_name"`
	Period           string             `json:"period"`
	PeriodStart      time.Time          `json:"period_start"`
	PeriodEnd        time.Time          `json:"period_end"`
	DaysRemaining    int                `json:"days_remaining"`
	TotalBudget      float64            `json:"total_budget"`
	CurrentSpend     float64            `json:"current_spend"`
	Remaining        float64            `json:"remaining"`
	Utilization      float64            `json:"utilization"`       // 预算使用率
	ProjectedSpend   float64            `json:"projected_spend"`   // 预计期末支出
	ProjectedOverrun float64            `json:"projected_overrun"` // 预计超支
	Status           string             `json:"status"`            // on_track, at_risk, over_budget
	Categories       []CategorySpend    `json:"categories"`
	Trend            []BudgetTrendPoint `json:"trend"`
	Alerts           []BudgetAlert      `json:"alerts,omitempty"`
	Recommendations  []string           `json:"recommendations"`
}

// CategorySpend 分类支出
type CategorySpend struct {
	Name         string  `json:"name"`
	Budget       float64 `json:"budget"`
	CurrentSpend float64 `json:"current_spend"`
	Percentage   float64 `json:"percentage"`
	Trend        string  `json:"trend"` // up, down, stable
}

// BudgetTrendPoint 预算趋势数据点
type BudgetTrendPoint struct {
	Date       time.Time `json:"date"`
	Spend      float64   `json:"spend"`
	Cumulative float64   `json:"cumulative"`
	Percentage float64   `json:"percentage"`
}

// BudgetAlert 预算告警
type BudgetAlert struct {
	ID           string    `json:"id"`
	BudgetID     string    `json:"budget_id"`
	Type         string    `json:"type"` // threshold_exceeded, projected_overrun
	Severity     string    `json:"severity"`
	Message      string    `json:"message"`
	Threshold    float64   `json:"threshold"`
	Actual       float64   `json:"actual"`
	CreatedAt    time.Time `json:"created_at"`
	Acknowledged bool      `json:"acknowledged"`
}

// ========== 成本分析引擎 ==========

// CostAnalysisEngine 成本分析引擎
// 提供成本分析报告生成、预算管理、趋势分析等功能
// 支持多种报告类型：存储趋势、资源利用率、优化建议、预算跟踪等
type CostAnalysisEngine struct {
	mu            sync.RWMutex
	dataDir       string
	billingClient BillingDataProvider
	quotaClient   QuotaDataProvider
	config        AnalysisConfig
	budgets       map[string]*BudgetConfig
	trendData     []CostTrend
	alerts        []*CostAlert

	// 缓存：提高频繁查询的性能
	summaryCache     *CostSummary
	summaryCacheTime time.Time
	cacheTTL         time.Duration
}

// BillingDataProvider 计费数据提供者接口
type BillingDataProvider interface {
	GetUsageRecords(userID, poolID string, start, end time.Time) ([]*UsageRecord, error)
	GetUserUsageSummary(userID string, start, end time.Time) (*UsageSummary, error)
	GetBillingStats(start, end time.Time) (*BillingStats, error)
	GetStoragePrice(poolID string) float64
	GetBandwidthPrice() float64
}

// QuotaDataProvider 配额数据提供者接口
type QuotaDataProvider interface {
	GetAllUsage() ([]*QuotaUsageInfo, error)
	GetUserUsage(username string) ([]*QuotaUsageInfo, error)
	GetPoolUsage(poolID string) (*QuotaUsageInfo, error)
}

// UsageRecord 用量记录
type UsageRecord struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	PoolID        string    `json:"pool_id"`
	StorageUsedGB float64   `json:"storage_used_gb"`
	BandwidthGB   float64   `json:"bandwidth_gb"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// UsageSummary 用量汇总
type UsageSummary struct {
	UserID             string  `json:"user_id"`
	TotalStorageUsedGB float64 `json:"total_storage_used_gb"`
	TotalBandwidthGB   float64 `json:"total_bandwidth_gb"`
}

// BillingStats 计费统计
type BillingStats struct {
	TotalStorageUsedGB float64 `json:"total_storage_used_gb"`
	TotalBandwidthGB   float64 `json:"total_bandwidth_gb"`
	TotalRevenue       float64 `json:"total_revenue"`
	StorageRevenue     float64 `json:"storage_revenue"`
	BandwidthRevenue   float64 `json:"bandwidth_revenue"`
}

// QuotaUsageInfo 配额使用信息
type QuotaUsageInfo struct {
	QuotaID      string  `json:"quota_id"`
	TargetID     string  `json:"target_id"`
	TargetName   string  `json:"target_name"`
	VolumeName   string  `json:"volume_name"`
	HardLimit    uint64  `json:"hard_limit"`
	UsedBytes    uint64  `json:"used_bytes"`
	Available    uint64  `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}

// AnalysisConfig 分析配置
type AnalysisConfig struct {
	DataRetentionDays     int       `json:"data_retention_days"`
	TrendAnalysisDays     int       `json:"trend_analysis_days"`
	IdleThresholdDays     int       `json:"idle_threshold_days"`
	UnderutilThreshold    float64   `json:"underutil_threshold"`
	DefaultCurrency       string    `json:"default_currency"`
	BudgetAlertThresholds []float64 `json:"budget_alert_thresholds"`
}

// DefaultAnalysisConfig 默认分析配置
func DefaultAnalysisConfig() AnalysisConfig {
	return AnalysisConfig{
		DataRetentionDays:     365,
		TrendAnalysisDays:     30,
		IdleThresholdDays:     30,
		UnderutilThreshold:    0.3, // 30% 以下为低利用率
		DefaultCurrency:       "CNY",
		BudgetAlertThresholds: []float64{50, 75, 90, 100},
	}
}

// NewCostAnalysisEngine 创建成本分析引擎
func NewCostAnalysisEngine(dataDir string, billing BillingDataProvider, quota QuotaDataProvider, config AnalysisConfig) *CostAnalysisEngine {
	engine := &CostAnalysisEngine{
		dataDir:       dataDir,
		billingClient: billing,
		quotaClient:   quota,
		config:        config,
		budgets:       make(map[string]*BudgetConfig),
		trendData:     make([]CostTrend, 0),
		alerts:        make([]*CostAlert, 0),
		cacheTTL:      5 * time.Minute, // 缓存有效期5分钟
	}

	// 加载已有数据（忽略错误，使用默认值）
	_ = engine.load()

	return engine
}

// load 加载数据
func (e *CostAnalysisEngine) load() error {
	// 加载预算配置
	budgetPath := filepath.Join(e.dataDir, "budgets.json")
	if data, err := os.ReadFile(budgetPath); err == nil {
		var budgets []*BudgetConfig
		if err := json.Unmarshal(data, &budgets); err == nil {
			for _, b := range budgets {
				e.budgets[b.ID] = b
			}
		}
	}

	// 加载趋势数据
	trendPath := filepath.Join(e.dataDir, "trend_data.json")
	if data, err := os.ReadFile(trendPath); err == nil {
		json.Unmarshal(data, &e.trendData)
	}

	return nil
}

// save 保存数据
func (e *CostAnalysisEngine) save() error {
	if err := os.MkdirAll(e.dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 保存预算配置
	budgets := make([]*BudgetConfig, 0, len(e.budgets))
	for _, b := range e.budgets {
		budgets = append(budgets, b)
	}
	if data, err := json.MarshalIndent(budgets, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(e.dataDir, "budgets.json"), data, 0644); err != nil {
			return fmt.Errorf("保存预算配置失败: %w", err)
		}
	}

	// 保存趋势数据
	if data, err := json.MarshalIndent(e.trendData, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(e.dataDir, "trend_data.json"), data, 0644); err != nil {
			return fmt.Errorf("保存趋势数据失败: %w", err)
		}
	}

	return nil
}

// ========== 成本分析报告生成 ==========

// GenerateStorageTrendReport 生成存储成本趋势报告
func (e *CostAnalysisEngine) GenerateStorageTrendReport(days int) (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	report := &CostReport{
		ID:          generateReportID(),
		Type:        CostReportStorageTrend,
		GeneratedAt: now,
		PeriodStart: startDate,
		PeriodEnd:   now,
	}

	// 获取分析数据
	analysis := e.analyzeStorageTrend(startDate, now)
	report.Details = analysis

	// 生成摘要
	report.Summary = CostSummary{
		TotalCost:         analysis.CurrentUsage.MonthlyCost,
		StorageCost:       analysis.CurrentUsage.MonthlyCost,
		Currency:          e.config.DefaultCurrency,
		CostChangePercent: e.calculateCostChange(days),
		AvgDailyCost:      analysis.CurrentUsage.MonthlyCost / 30,
	}

	// 生成趋势数据
	report.Trends = e.generateTrendData(startDate, now)

	// 生成建议
	report.Recommendations = e.generateStorageRecommendations(analysis)

	return report, nil
}

// GenerateResourceUtilizationReport 生成资源利用率报告
func (e *CostAnalysisEngine) GenerateResourceUtilizationReport() (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	startDate := now.AddDate(0, 0, -7) // 最近一周

	report := &CostReport{
		ID:          generateReportID(),
		Type:        CostReportResourceUtil,
		GeneratedAt: now,
		PeriodStart: startDate,
		PeriodEnd:   now,
	}

	// 获取利用率数据
	utilReport := e.analyzeResourceUtilization(startDate, now)
	report.Details = utilReport

	// 计算整体利用率评分
	report.Summary = CostSummary{
		Currency: e.config.DefaultCurrency,
	}

	// 生成建议
	report.Recommendations = e.generateUtilizationRecommendations(utilReport)

	return report, nil
}

// GenerateOptimizationReport 生成成本优化建议报告
func (e *CostAnalysisEngine) GenerateOptimizationReport() (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	startDate := now.AddDate(0, 0, -30) // 最近30天

	report := &CostReport{
		ID:          generateReportID(),
		Type:        CostReportOptimization,
		GeneratedAt: now,
		PeriodStart: startDate,
		PeriodEnd:   now,
	}

	// 分析优化机会
	recommendations := e.analyzeOptimizationOpportunities(startDate, now)
	report.Recommendations = recommendations

	// 计算潜在节省
	var totalSavings float64
	for _, rec := range recommendations {
		totalSavings += rec.PotentialSavings
	}

	report.Summary = CostSummary{
		Currency: e.config.DefaultCurrency,
	}
	report.Metadata = map[string]interface{}{
		"total_potential_savings": totalSavings,
		"recommendation_count":    len(recommendations),
	}

	return report, nil
}

// GenerateBudgetTrackingReport 生成预算跟踪报告
func (e *CostAnalysisEngine) GenerateBudgetTrackingReport(budgetID string) (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	budget, exists := e.budgets[budgetID]
	if !exists {
		return nil, fmt.Errorf("预算不存在: %s", budgetID)
	}

	now := time.Now()
	report := &CostReport{
		ID:          generateReportID(),
		Type:        CostReportBudgetTracking,
		GeneratedAt: now,
		PeriodStart: budget.StartDate,
		PeriodEnd:   budget.EndDate,
	}

	// 生成预算跟踪详情
	tracking := e.generateBudgetTracking(budget)
	report.Details = tracking

	// 摘要
	report.Summary = CostSummary{
		TotalCost:            tracking.CurrentSpend,
		Currency:             e.config.DefaultCurrency,
		BudgetUtilization:    tracking.Utilization,
		ProjectedMonthlyCost: tracking.ProjectedSpend,
	}

	// 建议
	report.Recommendations = e.generateBudgetRecommendations(tracking)

	return report, nil
}

// GenerateComprehensiveReport 生成综合成本分析报告
func (e *CostAnalysisEngine) GenerateComprehensiveReport() (*CostReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	startDate := now.AddDate(0, -1, 0) // 最近一个月

	report := &CostReport{
		ID:          generateReportID(),
		Type:        CostReportComprehensive,
		GeneratedAt: now,
		PeriodStart: startDate,
		PeriodEnd:   now,
	}

	// 组合多种分析
	trendAnalysis := e.analyzeStorageTrend(startDate, now)
	utilReport := e.analyzeResourceUtilization(startDate, now)
	optimizations := e.analyzeOptimizationOpportunities(startDate, now)

	// 预算跟踪
	var budgetTrackings []BudgetTrackingReport
	for _, budget := range e.budgets {
		if budget.Enabled {
			budgetTrackings = append(budgetTrackings, *e.generateBudgetTracking(budget))
		}
	}

	report.Details = map[string]interface{}{
		"storage_trend":   trendAnalysis,
		"resource_util":   utilReport,
		"budget_tracking": budgetTrackings,
	}

	// 综合摘要
	var totalSavings float64
	for _, opt := range optimizations {
		totalSavings += opt.PotentialSavings
	}

	report.Summary = CostSummary{
		TotalCost:         trendAnalysis.CurrentUsage.MonthlyCost,
		StorageCost:       trendAnalysis.CurrentUsage.MonthlyCost,
		Currency:          e.config.DefaultCurrency,
		CostChangePercent: e.calculateCostChange(30),
	}

	report.Recommendations = optimizations

	return report, nil
}

// ========== 分析方法 ==========

// analyzeStorageTrend 分析存储趋势
func (e *CostAnalysisEngine) analyzeStorageTrend(start, end time.Time) *StorageTrendAnalysis {
	analysis := &StorageTrendAnalysis{
		AnalysisDate: time.Now(),
		PeriodDays:   int(end.Sub(start).Hours() / 24),
		TrendData:    make([]StorageTrendPoint, 0),
		PoolAnalysis: make([]PoolCostAnalysis, 0),
		UserAnalysis: make([]UserCostAnalysis, 0),
		Alerts:       make([]CostAlert, 0),
	}

	// 获取当前使用情况
	if quotaUsages, err := e.quotaClient.GetAllUsage(); err == nil {
		var totalUsed uint64
		for _, u := range quotaUsages {
			totalUsed += u.UsedBytes
		}

		pricePerGB := e.billingClient.GetStoragePrice("")
		monthlyCost := float64(totalUsed) / (1024 * 1024 * 1024) * pricePerGB * 30

		analysis.CurrentUsage = StorageUsageSnapshot{
			Timestamp:    time.Now(),
			UsedBytes:    totalUsed,
			UsagePercent: 0, // 需要总容量来计算
			CostPerGB:    pricePerGB,
			MonthlyCost:  monthlyCost,
		}

		// 分析各存储池
		poolMap := make(map[string]*PoolCostAnalysis)
		for _, u := range quotaUsages {
			if _, exists := poolMap[u.VolumeName]; !exists {
				poolMap[u.VolumeName] = &PoolCostAnalysis{
					PoolID:     u.VolumeName,
					PoolName:   u.VolumeName,
					UsedBytes:  0,
					PricePerGB: e.billingClient.GetStoragePrice(u.VolumeName),
				}
			}
			poolMap[u.VolumeName].UsedBytes += u.UsedBytes
			poolMap[u.VolumeName].TotalBytes += u.HardLimit
		}

		for _, pool := range poolMap {
			if pool.TotalBytes > 0 {
				pool.UsagePercent = float64(pool.UsedBytes) / float64(pool.TotalBytes) * 100
			}
			pool.MonthlyCost = float64(pool.UsedBytes) / (1024 * 1024 * 1024) * pool.PricePerGB * 30
			pool.CostEfficiency = e.calculateCostEfficiency(pool.UsagePercent)
			analysis.PoolAnalysis = append(analysis.PoolAnalysis, *pool)
		}

		// 分析用户
		userMap := make(map[string]*UserCostAnalysis)
		for _, u := range quotaUsages {
			if _, exists := userMap[u.TargetID]; !exists {
				userMap[u.TargetID] = &UserCostAnalysis{
					UserID:       u.TargetID,
					UserName:     u.TargetName,
					TotalUsedGB:  0,
					QuotaLimitGB: float64(u.HardLimit) / (1024 * 1024 * 1024),
				}
			}
			userMap[u.TargetID].TotalUsedGB += float64(u.UsedBytes) / (1024 * 1024 * 1024)
		}

		for _, user := range userMap {
			if user.QuotaLimitGB > 0 {
				user.QuotaUsage = user.TotalUsedGB / user.QuotaLimitGB * 100
			}
			analysis.UserAnalysis = append(analysis.UserAnalysis, *user)
		}
	}

	// 计算增长率
	analysis.GrowthRate = e.calculateGrowthRate(start, end)

	// 成本预测
	analysis.CostProjection = e.projectCost(analysis)

	// 生成告警
	analysis.Alerts = e.generateCostAlerts(analysis)

	return analysis
}

// analyzeResourceUtilization 分析资源利用率
func (e *CostAnalysisEngine) analyzeResourceUtilization(start, end time.Time) *ResourceUtilizationReport {
	report := &ResourceUtilizationReport{
		GeneratedAt:     time.Now(),
		PeriodStart:     start,
		PeriodEnd:       end,
		UserUtilization: make([]UserUtilStats, 0),
		PoolUtilization: make([]PoolUtilStats, 0),
		IdleResources:   make([]IdleResource, 0),
		Underutilized:   make([]UnderutilizedResource, 0),
		Recommendations: make([]UtilizationRecommendation, 0),
	}

	// 获取配额使用情况
	quotaUsages, err := e.quotaClient.GetAllUsage()
	if err != nil {
		return report
	}

	var totalAllocated, totalUsed uint64
	userStats := make(map[string]*UserUtilStats)
	poolStats := make(map[string]*PoolUtilStats)

	for _, u := range quotaUsages {
		totalAllocated += u.HardLimit
		totalUsed += u.UsedBytes

		// 用户统计
		if _, exists := userStats[u.TargetID]; !exists {
			userStats[u.TargetID] = &UserUtilStats{
				UserID:         u.TargetID,
				UserName:       u.TargetName,
				QuotaAllocated: 0,
				QuotaUsed:      0,
			}
		}
		userStats[u.TargetID].QuotaAllocated += u.HardLimit
		userStats[u.TargetID].QuotaUsed += u.UsedBytes

		// 存储池统计
		if _, exists := poolStats[u.VolumeName]; !exists {
			poolStats[u.VolumeName] = &PoolUtilStats{
				PoolID:        u.VolumeName,
				PoolName:      u.VolumeName,
				TotalCapacity: 0,
				UsedCapacity:  0,
			}
		}
		poolStats[u.VolumeName].TotalCapacity += u.HardLimit
		poolStats[u.VolumeName].UsedCapacity += u.UsedBytes
	}

	// 计算整体利用率
	if totalAllocated > 0 {
		report.StorageUtil = StorageUtilization{
			TotalCapacity:   totalAllocated,
			UsedCapacity:    totalUsed,
			FreeCapacity:    totalAllocated - totalUsed,
			UtilizationRate: float64(totalUsed) / float64(totalAllocated) * 100,
		}
	}

	// 计算用户利用率
	for _, stats := range userStats {
		if stats.QuotaAllocated > 0 {
			stats.UtilizationRate = float64(stats.QuotaUsed) / float64(stats.QuotaAllocated) * 100
		}

		// 判断状态
		if stats.UtilizationRate < 10 {
			stats.Status = "idle"
			stats.SavingsPotential = e.calculateSavings(stats.QuotaAllocated, stats.QuotaUsed)
			report.IdleResources = append(report.IdleResources, IdleResource{
				ResourceType: "user_quota",
				ResourceID:   stats.UserID,
				ResourceName: stats.UserName,
				Capacity:     stats.QuotaAllocated,
				WastedCost:   stats.SavingsPotential,
			})
		} else if stats.UtilizationRate < e.config.UnderutilThreshold*100 {
			stats.Status = "underutilized"
			report.Underutilized = append(report.Underutilized, UnderutilizedResource{
				ResourceID:      stats.UserID,
				ResourceName:    stats.UserName,
				ResourceType:    "user_quota",
				Allocated:       stats.QuotaAllocated,
				Used:            stats.QuotaUsed,
				UtilizationRate: stats.UtilizationRate,
				Threshold:       e.config.UnderutilThreshold * 100,
			})
		} else {
			stats.Status = "active"
		}

		report.UserUtilization = append(report.UserUtilization, *stats)
	}

	// 计算存储池利用率
	for _, stats := range poolStats {
		if stats.TotalCapacity > 0 {
			stats.UtilizationRate = float64(stats.UsedCapacity) / float64(stats.TotalCapacity) * 100
		}

		// 性能评估
		if stats.UtilizationRate > 80 {
			stats.Performance = "poor"
		} else if stats.UtilizationRate > 60 {
			stats.Performance = "fair"
		} else {
			stats.Performance = "good"
		}

		stats.CostEfficiency = e.calculateCostEfficiency(stats.UtilizationRate)
		report.PoolUtilization = append(report.PoolUtilization, *stats)
	}

	// 计算整体评分
	report.OverallScore = e.calculateOverallScore(report)

	// 生成建议
	report.Recommendations = e.generateUtilRecommendations(report)

	return report
}

// analyzeOptimizationOpportunities 分析优化机会
func (e *CostAnalysisEngine) analyzeOptimizationOpportunities(start, end time.Time) []CostRecommendation {
	recommendations := make([]CostRecommendation, 0)

	// 获取利用率报告
	utilReport := e.analyzeResourceUtilization(start, end)

	// 闲置资源建议
	for _, idle := range utilReport.IdleResources {
		recommendations = append(recommendations, CostRecommendation{
			ID:               generateReportID(),
			Type:             idle.ResourceType,
			Priority:         "high",
			Title:            fmt.Sprintf("回收闲置配额: %s", idle.ResourceName),
			Description:      fmt.Sprintf("用户 %s 的配额利用率低于 10%%，建议回收或调整", idle.ResourceName),
			PotentialSavings: idle.WastedCost,
			Impact:           "用户存储空间",
			Action:           "联系用户确认后回收配额或降低配额限制",
			Implemented:      false,
		})
	}

	// 低利用率资源建议
	for _, under := range utilReport.Underutilized {
		recommendations = append(recommendations, CostRecommendation{
			ID:               generateReportID(),
			Type:             under.ResourceType,
			Priority:         "medium",
			Title:            fmt.Sprintf("优化低利用率配额: %s", under.ResourceName),
			Description:      fmt.Sprintf("用户 %s 的配额利用率仅 %.1f%%，建议优化", under.ResourceName, under.UtilizationRate),
			PotentialSavings: under.SavingsPotential,
			Impact:           "用户存储空间",
			Action:           "建议降低配额限制或优化存储使用",
			Implemented:      false,
		})
	}

	// 存储池优化建议
	for _, pool := range utilReport.PoolUtilization {
		if pool.UtilizationRate > 80 {
			recommendations = append(recommendations, CostRecommendation{
				ID:               generateReportID(),
				Type:             "pool",
				Priority:         "high",
				Title:            fmt.Sprintf("存储池 %s 容量紧张", pool.PoolName),
				Description:      fmt.Sprintf("存储池使用率已达 %.1f%%，建议扩容或清理", pool.UtilizationRate),
				PotentialSavings: 0, // 扩容是成本增加
				Impact:           "整个存储池",
				Action:           "评估扩容需求或清理无用数据",
				Implemented:      false,
			})
		} else if pool.UtilizationRate < 30 && pool.TotalCapacity > 100*1024*1024*1024 { // 大于100GB
			recommendations = append(recommendations, CostRecommendation{
				ID:               generateReportID(),
				Type:             "pool",
				Priority:         "low",
				Title:            fmt.Sprintf("存储池 %s 利用率偏低", pool.PoolName),
				Description:      fmt.Sprintf("存储池使用率仅 %.1f%%，资源可能过剩", pool.UtilizationRate),
				PotentialSavings: e.calculatePoolSavings(pool.TotalCapacity, pool.UsedCapacity, pool.PoolID),
				Impact:           "存储资源效率",
				Action:           "考虑整合存储资源或调整存储类型",
				Implemented:      false,
			})
		}
	}

	return recommendations
}

// ========== 预算管理 ==========

// CreateBudget 创建预算
func (e *CostAnalysisEngine) CreateBudget(config BudgetConfig) (*BudgetConfig, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if config.ID == "" {
		config.ID = generateReportID()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	e.budgets[config.ID] = &config
	e.save()

	return &config, nil
}

// UpdateBudget 更新预算
func (e *CostAnalysisEngine) UpdateBudget(id string, config BudgetConfig) (*BudgetConfig, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.budgets[id]
	if !exists {
		return nil, fmt.Errorf("预算不存在: %s", id)
	}

	config.ID = id
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = time.Now()

	e.budgets[id] = &config
	e.save()

	return &config, nil
}

// DeleteBudget 删除预算
func (e *CostAnalysisEngine) DeleteBudget(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.budgets[id]; !exists {
		return fmt.Errorf("预算不存在: %s", id)
	}

	delete(e.budgets, id)
	e.save()

	return nil
}

// GetBudget 获取预算
func (e *CostAnalysisEngine) GetBudget(id string) (*BudgetConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	budget, exists := e.budgets[id]
	if !exists {
		return nil, fmt.Errorf("预算不存在: %s", id)
	}

	return budget, nil
}

// ListBudgets 列出所有预算
func (e *CostAnalysisEngine) ListBudgets() []*BudgetConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*BudgetConfig, 0, len(e.budgets))
	for _, b := range e.budgets {
		result = append(result, b)
	}
	return result
}

// generateBudgetTracking 生成预算跟踪
func (e *CostAnalysisEngine) generateBudgetTracking(budget *BudgetConfig) *BudgetTrackingReport {
	now := time.Now()
	tracking := &BudgetTrackingReport{
		GeneratedAt:     now,
		BudgetID:        budget.ID,
		BudgetName:      budget.Name,
		Period:          budget.Period,
		PeriodStart:     budget.StartDate,
		PeriodEnd:       budget.EndDate,
		TotalBudget:     budget.TotalBudget,
		Categories:      make([]CategorySpend, 0),
		Trend:           make([]BudgetTrendPoint, 0),
		Alerts:          make([]BudgetAlert, 0),
		Recommendations: make([]string, 0),
	}

	// 计算剩余天数
	if budget.EndDate.After(now) {
		tracking.DaysRemaining = int(budget.EndDate.Sub(now).Hours() / 24)
	} else {
		tracking.DaysRemaining = 0
	}

	// 获取当前支出
	stats, err := e.billingClient.GetBillingStats(budget.StartDate, now)
	if err == nil {
		tracking.CurrentSpend = stats.StorageRevenue + stats.BandwidthRevenue
	} else {
		tracking.CurrentSpend = 0
	}

	tracking.Remaining = budget.TotalBudget - tracking.CurrentSpend
	if budget.TotalBudget > 0 {
		tracking.Utilization = tracking.CurrentSpend / budget.TotalBudget * 100
	}

	// 预测期末支出
	if tracking.DaysRemaining > 0 {
		elapsedDays := int(now.Sub(budget.StartDate).Hours() / 24)
		if elapsedDays > 0 {
			dailyRate := tracking.CurrentSpend / float64(elapsedDays)
			totalDays := int(budget.EndDate.Sub(budget.StartDate).Hours() / 24)
			tracking.ProjectedSpend = dailyRate * float64(totalDays)
		}
	}

	// 计算预计超支
	if tracking.ProjectedSpend > budget.TotalBudget {
		tracking.ProjectedOverrun = tracking.ProjectedSpend - budget.TotalBudget
	}

	// 判断状态
	if tracking.Utilization >= 100 {
		tracking.Status = "over_budget"
	} else if tracking.ProjectedOverrun > 0 {
		tracking.Status = "at_risk"
	} else {
		tracking.Status = "on_track"
	}

	// 生成告警
	for _, threshold := range budget.AlertThresholds {
		if tracking.Utilization >= threshold {
			tracking.Alerts = append(tracking.Alerts, BudgetAlert{
				ID:        generateReportID(),
				BudgetID:  budget.ID,
				Type:      "threshold_exceeded",
				Severity:  e.getAlertSeverity(threshold),
				Message:   fmt.Sprintf("预算使用已达 %.0f%%", threshold),
				Threshold: threshold,
				Actual:    tracking.Utilization,
				CreatedAt: now,
			})
		}
	}

	// 分类支出
	for _, cat := range budget.Categories {
		// 根据分类名称获取实际支出
		var spend float64
		if stats != nil {
			if cat.Name == "storage" {
				spend = stats.StorageRevenue
			} else if cat.Name == "bandwidth" {
				spend = stats.BandwidthRevenue
			}
		}

		percentage := 0.0
		if cat.Budget > 0 {
			percentage = spend / cat.Budget * 100
		}

		tracking.Categories = append(tracking.Categories, CategorySpend{
			Name:         cat.Name,
			Budget:       cat.Budget,
			CurrentSpend: spend,
			Percentage:   percentage,
			Trend:        "stable",
		})
	}

	return tracking
}

// ========== 辅助方法 ==========

// calculateCostChange 计算成本变化
func (e *CostAnalysisEngine) calculateCostChange(days int) float64 {
	if len(e.trendData) < 2 {
		return 0
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -days*2)

	var oldTotal, newTotal float64
	var oldCount, newCount int

	for _, t := range e.trendData {
		if t.Date.After(startDate) && t.Date.Before(now.AddDate(0, 0, -days)) {
			oldTotal += t.TotalCost
			oldCount++
		} else if t.Date.After(now.AddDate(0, 0, -days)) {
			newTotal += t.TotalCost
			newCount++
		}
	}

	if oldCount > 0 && newCount > 0 {
		oldAvg := oldTotal / float64(oldCount)
		newAvg := newTotal / float64(newCount)
		if oldAvg > 0 {
			return (newAvg - oldAvg) / oldAvg * 100
		}
	}

	return 0
}

// calculateGrowthRate 计算增长率
func (e *CostAnalysisEngine) calculateGrowthRate(start, end time.Time) StorageGrowthRate {
	// 简化实现，实际应从历史数据计算
	return StorageGrowthRate{
		TrendDirection: "stable",
	}
}

// projectCost 预测成本
func (e *CostAnalysisEngine) projectCost(analysis *StorageTrendAnalysis) CostProjection {
	projection := CostProjection{
		CurrentMonthlyCost: analysis.CurrentUsage.MonthlyCost,
		Assumptions:        []string{"基于当前使用趋势", "价格保持不变"},
		ConfidenceLevel:    0.7,
	}

	// 基于增长率预测
	if analysis.GrowthRate.DailyGrowthGB > 0 {
		dailyGrowthCost := analysis.GrowthRate.DailyGrowthGB * analysis.CurrentUsage.CostPerGB
		projection.ProjectedNextMonth = projection.CurrentMonthlyCost + dailyGrowthCost*30
		projection.ProjectedNextQuarter = projection.CurrentMonthlyCost + dailyGrowthCost*90
		projection.ProjectedNextYear = projection.CurrentMonthlyCost + dailyGrowthCost*365
	} else {
		projection.ProjectedNextMonth = projection.CurrentMonthlyCost
		projection.ProjectedNextQuarter = projection.CurrentMonthlyCost * 3
		projection.ProjectedNextYear = projection.CurrentMonthlyCost * 12
	}

	return projection
}

// generateCostAlerts 生成成本告警
func (e *CostAnalysisEngine) generateCostAlerts(analysis *StorageTrendAnalysis) []CostAlert {
	alerts := make([]CostAlert, 0)

	// 检查增长率异常
	if analysis.GrowthRate.DailyGrowthPercent > 5 {
		alerts = append(alerts, CostAlert{
			ID:        generateReportID(),
			Type:      "growth_anomaly",
			Severity:  "warning",
			Message:   fmt.Sprintf("存储日增长率异常: %.1f%%", analysis.GrowthRate.DailyGrowthPercent),
			Value:     analysis.GrowthRate.DailyGrowthPercent,
			Threshold: 5,
			CreatedAt: time.Now(),
		})
	}

	// 检查成本预测超预算
	for _, budget := range e.budgets {
		if budget.Enabled && analysis.CostProjection.ProjectedNextMonth > budget.TotalBudget {
			alerts = append(alerts, CostAlert{
				ID:        generateReportID(),
				Type:      "budget_exceeded",
				Severity:  "critical",
				Message:   fmt.Sprintf("预计下月成本 %.2f 元将超出预算 %.2f 元", analysis.CostProjection.ProjectedNextMonth, budget.TotalBudget),
				Value:     analysis.CostProjection.ProjectedNextMonth,
				Threshold: budget.TotalBudget,
				CreatedAt: time.Now(),
			})
		}
	}

	return alerts
}

// generateStorageRecommendations 生成存储建议
func (e *CostAnalysisEngine) generateStorageRecommendations(analysis *StorageTrendAnalysis) []CostRecommendation {
	return e.analyzeOptimizationOpportunities(time.Now().AddDate(0, -1, 0), time.Now())
}

// generateTrendData 生成趋势数据
func (e *CostAnalysisEngine) generateTrendData(start, end time.Time) []CostTrend {
	trends := make([]CostTrend, 0)

	// 使用已有的趋势数据
	for _, t := range e.trendData {
		if t.Date.After(start) && t.Date.Before(end) {
			trends = append(trends, t)
		}
	}

	return trends
}

// calculateCostEfficiency 计算成本效率
func (e *CostAnalysisEngine) calculateCostEfficiency(usagePercent float64) float64 {
	// 理想利用率在 60-80% 之间
	if usagePercent >= 60 && usagePercent <= 80 {
		return 1.0
	} else if usagePercent > 80 {
		return 0.8 - (usagePercent-80)*0.02 // 超过80%效率下降
	} else {
		return usagePercent / 60 // 低于60%效率较低
	}
}

// calculateSavings 计算节省
func (e *CostAnalysisEngine) calculateSavings(allocated, used uint64) float64 {
	wasted := allocated - used
	wastedGB := float64(wasted) / (1024 * 1024 * 1024)
	pricePerGB := e.billingClient.GetStoragePrice("")
	return wastedGB * pricePerGB
}

// calculatePoolSavings 计算存储池节省
func (e *CostAnalysisEngine) calculatePoolSavings(total, used uint64, poolID string) float64 {
	return e.calculateSavings(total, used)
}

// calculateOverallScore 计算整体评分
func (e *CostAnalysisEngine) calculateOverallScore(report *ResourceUtilizationReport) float64 {
	if report.StorageUtil.TotalCapacity == 0 {
		return 0
	}

	utilRate := report.StorageUtil.UtilizationRate

	// 评分逻辑：利用率越高越好，但过高也不好
	var score float64
	if utilRate >= 60 && utilRate <= 80 {
		score = 100
	} else if utilRate > 80 {
		score = 100 - (utilRate-80)*2
	} else {
		score = utilRate / 60 * 100
	}

	// 扣除闲置资源影响
	idleRatio := float64(len(report.IdleResources)) / float64(len(report.UserUtilization)+1)
	score -= idleRatio * 10

	if score < 0 {
		score = 0
	}

	return score
}

// generateUtilRecommendations 生成利用率建议
func (e *CostAnalysisEngine) generateUtilRecommendations(report *ResourceUtilizationReport) []UtilizationRecommendation {
	recs := make([]UtilizationRecommendation, 0)

	// 闲置资源建议
	if len(report.IdleResources) > 0 {
		recs = append(recs, UtilizationRecommendation{
			Type:        "deallocate",
			Priority:    "high",
			Description: fmt.Sprintf("发现 %d 个闲置资源，建议回收", len(report.IdleResources)),
			Savings:     e.sumIdleSavings(report.IdleResources),
			Effort:      "low",
			Impact:      "降低成本，释放资源",
		})
	}

	// 低利用率建议
	if len(report.Underutilized) > 0 {
		recs = append(recs, UtilizationRecommendation{
			Type:        "optimize",
			Priority:    "medium",
			Description: fmt.Sprintf("发现 %d 个低利用率资源，建议优化", len(report.Underutilized)),
			Savings:     e.sumUnderutilSavings(report.Underutilized),
			Effort:      "medium",
			Impact:      "提高资源效率",
		})
	}

	return recs
}

// generateUtilizationRecommendations 生成利用率建议（兼容方法）
func (e *CostAnalysisEngine) generateUtilizationRecommendations(report *ResourceUtilizationReport) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	for _, util := range report.Recommendations {
		recs = append(recs, CostRecommendation{
			ID:               generateReportID(),
			Type:             "utilization",
			Priority:         util.Priority,
			Title:            util.Description,
			Description:      util.Description,
			PotentialSavings: util.Savings,
			Impact:           util.Impact,
			Action:           util.Description,
		})
	}

	return recs
}

// generateBudgetRecommendations 生成预算建议
func (e *CostAnalysisEngine) generateBudgetRecommendations(tracking *BudgetTrackingReport) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	if tracking.Status == "over_budget" {
		recs = append(recs, CostRecommendation{
			ID:          generateReportID(),
			Type:        "budget",
			Priority:    "critical",
			Title:       "预算已超支",
			Description: fmt.Sprintf("当前支出 %.2f 元已超过预算 %.2f 元", tracking.CurrentSpend, tracking.TotalBudget),
			Impact:      "财务",
			Action:      "立即审核支出并采取控制措施",
		})
	} else if tracking.Status == "at_risk" {
		recs = append(recs, CostRecommendation{
			ID:          generateReportID(),
			Type:        "budget",
			Priority:    "high",
			Title:       "预算存在超支风险",
			Description: fmt.Sprintf("预计期末支出 %.2f 元，可能超出预算", tracking.ProjectedSpend),
			Impact:      "财务",
			Action:      "评估是否需要调整预算或控制支出",
		})
	}

	return recs
}

// getAlertSeverity 获取告警严重级别
func (e *CostAnalysisEngine) getAlertSeverity(threshold float64) string {
	if threshold >= 100 {
		return "critical"
	} else if threshold >= 90 {
		return "warning"
	} else if threshold >= 75 {
		return "info"
	}
	return "info"
}

// sumIdleSavings 计算闲置资源总节省
func (e *CostAnalysisEngine) sumIdleSavings(idle []IdleResource) float64 {
	var total float64
	for _, i := range idle {
		total += i.WastedCost
	}
	return total
}

// sumUnderutilSavings 计算低利用率资源总节省
func (e *CostAnalysisEngine) sumUnderutilSavings(under []UnderutilizedResource) float64 {
	var total float64
	for _, u := range under {
		total += u.SavingsPotential
	}
	return total
}

// RecordTrendData 记录趋势数据
func (e *CostAnalysisEngine) RecordTrendData(data CostTrend) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.trendData = append(e.trendData, data)
	e.save()
}

// GetAlerts 获取成本告警
func (e *CostAnalysisEngine) GetAlerts() []*CostAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*CostAlert, len(e.alerts))
	copy(result, e.alerts)
	return result
}

// AcknowledgeAlert 确认告警
func (e *CostAnalysisEngine) AcknowledgeAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, alert := range e.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			return nil
		}
	}

	return fmt.Errorf("告警不存在: %s", alertID)
}

// ClearCache 清除缓存
// 用于在数据更新后强制刷新缓存
func (e *CostAnalysisEngine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.summaryCache = nil
	e.summaryCacheTime = time.Time{}
}

// GetCacheStatus 获取缓存状态
// 返回缓存是否有效和缓存时间
func (e *CostAnalysisEngine) GetCacheStatus() (valid bool, cacheTime time.Time) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.summaryCache == nil {
		return false, time.Time{}
	}

	valid = time.Since(e.summaryCacheTime) < e.cacheTTL
	return valid, e.summaryCacheTime
}

// generateReportID 生成报告ID
func generateReportID() string {
	return fmt.Sprintf("rpt-%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString 生成随机字符串
// 注意：此函数用于生成非安全敏感的ID，如报告ID
// 对于安全敏感的场景，应使用 crypto/rand
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	// 使用当前时间的纳秒作为种子，结合循环索引确保随机性
	// 对于报告ID生成，这种简单实现足够
	now := time.Now().UnixNano()
	for i := range b {
		// 简单的伪随机：交替使用时间戳的不同部分
		seed := now + int64(i)*12345
		b[i] = letters[(seed+int64(i)*7919)%int64(len(letters))]
	}
	return string(b)
}
