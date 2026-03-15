// Package reports 提供成本报告生成功能
package reports

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrBillingCostReportNotFound     = errors.New("成本报告不存在")
	ErrInvalidBillingReportType  = errors.New("无效的报告类型")
	ErrInvalidDateFormat  = errors.New("无效的日期格式")
	ErrCostExportFailed       = errors.New("导出失败")
)

// ========== 报告类型 ==========

// CostBillingReportType 成本报告类型
type CostBillingReportType string

const (
	CostBillingReportTypeDaily   CostBillingReportType = "daily"   // 日报
	CostBillingReportTypeWeekly  CostBillingReportType = "weekly"  // 周报
	CostBillingReportTypeMonthly CostBillingReportType = "monthly" // 月报
)

// CostBillingExportFormat 成本导出格式
type CostBillingExportFormat string

const (
	CostBillingExportFormatJSON CostBillingExportFormat = "json" // JSON格式
	CostBillingExportFormatCSV  CostBillingExportFormat = "csv"  // CSV格式
)

// ========== 成本报告定义 ==========

// BillingCostReport 成本报告
type BillingCostReport struct {
	// 基本信息
	ID           string      `json:"id"`
	BillingReportType   BillingReportType  `json:"report_type"`
	GeneratedAt  time.Time   `json:"generated_at"`
	PeriodStart  time.Time   `json:"period_start"`
	PeriodEnd    time.Time   `json:"period_end"`
	Currency     string      `json:"currency"`

	// 摘要
	Summary BillingReportSummary `json:"summary"`

	// 存储成本详情
	StorageCost StorageCostSection `json:"storage_cost"`

	// 带宽成本详情
	BandwidthCost BandwidthCostSection `json:"bandwidth_cost"`

	// 成本趋势
	Trends []BillingTrendItem `json:"trends"`

	// 按存储池分解
	PoolBreakdown []PoolCostItem `json:"pool_breakdown"`

	// 按用户分解
	UserBreakdown []UserCostItem `json:"user_breakdown"`

	// 预算对比
	BudgetComparison *BudgetComparison `json:"budget_comparison,omitempty"`

	// 优化建议
	Recommendations []RecommendationItem `json:"recommendations"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BillingReportSummary 成本报告摘要
type BillingReportSummary struct {
	// 总成本
	TotalCost         float64 `json:"total_cost"`
	StorageCost       float64 `json:"storage_cost"`
	BandwidthCost     float64 `json:"bandwidth_cost"`
	OtherCost         float64 `json:"other_cost"`

	// 变化
	CostChange         float64 `json:"cost_change"`          // 成本变化金额
	CostChangePercent  float64 `json:"cost_change_percent"`  // 成本变化百分比
	StorageChange      float64 `json:"storage_change"`
	BandwidthChange    float64 `json:"bandwidth_change"`

	// 资源使用
	TotalStorageGB     float64 `json:"total_storage_gb"`
	TotalTrafficGB     float64 `json:"total_traffic_gb"`
	StorageUtilization float64 `json:"storage_utilization"`

	// 预测
	ProjectedNextPeriod float64 `json:"projected_next_period"`
	BudgetStatus        string  `json:"budget_status"` // on_track, at_risk, over_budget

	// 健康评分
	HealthScore int `json:"health_score"` // 0-100
}

// StorageCostSection 存储成本部分
type StorageCostSection struct {
	// 总量
	TotalCapacityGB float64 `json:"total_capacity_gb"`
	UsedCapacityGB  float64 `json:"used_capacity_gb"`
	FreeCapacityGB  float64 `json:"free_capacity_gb"`
	UtilizationRate float64 `json:"utilization_rate"`

	// 成本
	MonthlyCost   float64 `json:"monthly_cost"`
	DailyCost     float64 `json:"daily_cost"`
	AveragePrice  float64 `json:"average_price"`

	// 按存储类型
	SSDCost      float64 `json:"ssd_cost"`
	SSDUsedGB    float64 `json:"ssd_used_gb"`
	HDDCost      float64 `json:"hdd_cost"`
	HDDUsedGB    float64 `json:"hdd_used_gb"`
	ArchiveCost  float64 `json:"archive_cost"`
	ArchiveUsedGB float64 `json:"archive_used_gb"`

	// 按访问频率
	HotDataGB   float64 `json:"hot_data_gb"`
	HotDataCost float64 `json:"hot_data_cost"`
	WarmDataGB  float64 `json:"warm_data_gb"`
	WarmDataCost float64 `json:"warm_data_cost"`
	ColdDataGB  float64 `json:"cold_data_gb"`
	ColdDataCost float64 `json:"cold_data_cost"`

	// 阶梯定价明细
	TierBreakdown []TierCostItem `json:"tier_breakdown"`
}

// BandwidthCostSection 带宽成本部分
type BandwidthCostSection struct {
	// 流量统计
	InboundTrafficGB  float64 `json:"inbound_traffic_gb"`
	OutboundTrafficGB float64 `json:"outbound_traffic_gb"`
	TotalTrafficGB    float64 `json:"total_traffic_gb"`

	// 带宽统计
	PeakMbps   float64 `json:"peak_mbps"`
	AverageMbps float64 `json:"average_mbps"`
	Peak95Mbps float64 `json:"peak_95_mbps"`

	// 成本
	TotalCost      float64 `json:"total_cost"`
	TrafficCost    float64 `json:"traffic_cost"`
	BandwidthCost  float64 `json:"bandwidth_cost"`
	OverageCost    float64 `json:"overage_cost"`

	// 计费模式
	BillingModel string `json:"billing_model"`

	// 免费额度
	FreeAllowanceGB float64 `json:"free_allowance_gb"`
	ChargedTrafficGB float64 `json:"charged_traffic_gb"`

	// 时间分布
	PeakHours   []int `json:"peak_hours"`   // 高峰时段
	OffPeakHours []int `json:"off_peak_hours"` // 低谷时段
}

// BillingTrendItem 成本趋势项
type BillingTrendItem struct {
	Date          time.Time `json:"date"`
	StorageCost   float64   `json:"storage_cost"`
	BandwidthCost float64   `json:"bandwidth_cost"`
	TotalCost     float64   `json:"total_cost"`
	StorageGB     float64   `json:"storage_gb"`
	TrafficGB     float64   `json:"traffic_gb"`
	CumulativeCost float64  `json:"cumulative_cost"`
}

// PoolCostItem 存储池成本项
type PoolCostItem struct {
	PoolID          string  `json:"pool_id"`
	PoolName        string  `json:"pool_name"`
	StorageType     string  `json:"storage_type"`
	TotalCapacityGB float64 `json:"total_capacity_gb"`
	UsedCapacityGB  float64 `json:"used_capacity_gb"`
	UsagePercent    float64 `json:"usage_percent"`
	PricePerGB      float64 `json:"price_per_gb"`
	MonthlyCost     float64 `json:"monthly_cost"`
	CostEfficiency  float64 `json:"cost_efficiency"`

	// 访问频率分布
	HotDataGB  float64 `json:"hot_data_gb"`
	WarmDataGB float64 `json:"warm_data_gb"`
	ColdDataGB float64 `json:"cold_data_gb"`

	// 趋势
	Trend string `json:"trend"` // up, down, stable
}

// UserCostItem 用户成本项
type UserCostItem struct {
	UserID      string             `json:"user_id"`
	UserName    string             `json:"user_name"`
	UsedGB      float64            `json:"used_gb"`
	MonthlyCost float64            `json:"monthly_cost"`
	CostPerGB   float64            `json:"cost_per_gb"`
	Tier        string             `json:"tier"`
	Trend       string             `json:"trend"`
	PoolUsage   map[string]float64 `json:"pool_usage"` // poolID -> GB
}

// TierCostItem 阶梯成本项
type TierCostItem struct {
	TierName   string  `json:"tier_name"`
	MinGB      float64 `json:"min_gb"`
	MaxGB      float64 `json:"max_gb"`
	UsedGB     float64 `json:"used_gb"`
	PricePerGB float64 `json:"price_per_gb"`
	Cost       float64 `json:"cost"`
}

// BudgetComparison 预算对比
type BudgetComparison struct {
	BudgetID     string  `json:"budget_id"`
	BudgetName   string  `json:"budget_name"`
	TotalBudget  float64 `json:"total_budget"`
	CurrentSpend float64 `json:"current_spend"`
	Remaining    float64 `json:"remaining"`
	Utilization  float64 `json:"utilization"`
	Status       string  `json:"status"`
	AlertLevel   string  `json:"alert_level"`

	// 分类预算
	Categories []BudgetCategoryItem `json:"categories"`
}

// BudgetCategoryItem 预算分类项
type BudgetCategoryItem struct {
	Name         string  `json:"name"`
	Budget       float64 `json:"budget"`
	CurrentSpend float64 `json:"current_spend"`
	Utilization  float64 `json:"utilization"`
	Trend        string  `json:"trend"`
}

// RecommendationItem 建议项
type RecommendationItem struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	Priority         string  `json:"priority"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	PotentialSavings float64 `json:"potential_savings"`
	CurrentCost      float64 `json:"current_cost"`
	OptimizedCost    float64 `json:"optimized_cost"`
	Action           string  `json:"action"`
	Impact           string  `json:"impact"`
}

// ========== 报告生成器 ==========

// BillingBillingReportGenerator 成本报告生成器
type BillingBillingReportGenerator struct {
	mu        sync.RWMutex
	dataDir   string
	providers ReportDataProvider
	config    ReportConfig

	// 缓存
	reportCache map[string]*BillingCostReport
	cacheExpiry time.Duration
}

// ReportDataProvider 报告数据提供者接口
type ReportDataProvider interface {
	// 存储数据
	GetStorageData(ctx context.Context, start, end time.Time) (*StorageReportData, error)
	GetPoolData(ctx context.Context, start, end time.Time) ([]PoolReportData, error)
	GetUserData(ctx context.Context, start, end time.Time) ([]UserReportData, error)

	// 带宽数据
	GetBandwidthData(ctx context.Context, start, end time.Time) (*BandwidthReportData, error)

	// 趋势数据
	GetTrendData(ctx context.Context, start, end time.Time) ([]TrendReportData, error)

	// 预算数据
	GetBudgetData(ctx context.Context, budgetID string) (*BudgetReportData, error)

	// 成本建议
	GetRecommendations(ctx context.Context) ([]RecommendationItem, error)

	// 历史报告
	GetHistoricalReport(ctx context.Context, reportType BillingReportType, date time.Time) (*BillingCostReport, error)
}

// StorageReportData 存储报告数据
type StorageReportData struct {
	TotalCapacityGB   float64 `json:"total_capacity_gb"`
	UsedCapacityGB    float64 `json:"used_capacity_gb"`
	MonthlyCost       float64 `json:"monthly_cost"`
	DailyCost         float64 `json:"daily_cost"`
	AveragePrice      float64 `json:"average_price"`

	SSDCost          float64 `json:"ssd_cost"`
	SSDUsedGB        float64 `json:"ssd_used_gb"`
	HDDCost          float64 `json:"hdd_cost"`
	HDDUsedGB        float64 `json:"hdd_used_gb"`
	ArchiveCost      float64 `json:"archive_cost"`
	ArchiveUsedGB    float64 `json:"archive_used_gb"`

	HotDataGB        float64 `json:"hot_data_gb"`
	HotDataCost      float64 `json:"hot_data_cost"`
	WarmDataGB       float64 `json:"warm_data_gb"`
	WarmDataCost     float64 `json:"warm_data_cost"`
	ColdDataGB       float64 `json:"cold_data_gb"`
	ColdDataCost     float64 `json:"cold_data_cost"`

	UtilizationRate  float64 `json:"utilization_rate"`
	TierBreakdown    []TierCostItem `json:"tier_breakdown"`
}

// PoolReportData 存储池报告数据
type PoolReportData struct {
	PoolID          string  `json:"pool_id"`
	PoolName        string  `json:"pool_name"`
	StorageType     string  `json:"storage_type"`
	TotalCapacityGB float64 `json:"total_capacity_gb"`
	UsedCapacityGB  float64 `json:"used_capacity_gb"`
	PricePerGB      float64 `json:"price_per_gb"`
	MonthlyCost     float64 `json:"monthly_cost"`
	HotDataGB       float64 `json:"hot_data_gb"`
	WarmDataGB      float64 `json:"warm_data_gb"`
	ColdDataGB      float64 `json:"cold_data_gb"`
	Trend           string  `json:"trend"`
}

// UserReportData 用户报告数据
type UserReportData struct {
	UserID      string             `json:"user_id"`
	UserName    string             `json:"user_name"`
	UsedGB      float64            `json:"used_gb"`
	MonthlyCost float64            `json:"monthly_cost"`
	CostPerGB   float64            `json:"cost_per_gb"`
	Tier        string             `json:"tier"`
	Trend       string             `json:"trend"`
	PoolUsage   map[string]float64 `json:"pool_usage"`
}

// BandwidthReportData 带宽报告数据
type BandwidthReportData struct {
	InboundTrafficGB  float64 `json:"inbound_traffic_gb"`
	OutboundTrafficGB float64 `json:"outbound_traffic_gb"`
	TotalTrafficGB    float64 `json:"total_traffic_gb"`
	PeakMbps          float64 `json:"peak_mbps"`
	AverageMbps       float64 `json:"average_mbps"`
	Peak95Mbps        float64 `json:"peak_95_mbps"`
	TotalCost         float64 `json:"total_cost"`
	TrafficCost       float64 `json:"traffic_cost"`
	BandwidthCost     float64 `json:"bandwidth_cost"`
	BillingModel      string  `json:"billing_model"`
	FreeAllowanceGB   float64 `json:"free_allowance_gb"`
	PeakHours         []int   `json:"peak_hours"`
}

// TrendReportData 趋势报告数据
type TrendReportData struct {
	Date          time.Time `json:"date"`
	StorageCost   float64   `json:"storage_cost"`
	BandwidthCost float64   `json:"bandwidth_cost"`
	StorageGB     float64   `json:"storage_gb"`
	TrafficGB     float64   `json:"traffic_gb"`
}

// BudgetReportData 预算报告数据
type BudgetReportData struct {
	BudgetID     string  `json:"budget_id"`
	BudgetName   string  `json:"budget_name"`
	TotalBudget  float64 `json:"total_budget"`
	CurrentSpend float64 `json:"current_spend"`
	Status       string  `json:"status"`
	AlertLevel   string  `json:"alert_level"`
	Categories   []BudgetCategoryItem `json:"categories"`
}

// ReportConfig 报告配置
type ReportConfig struct {
	DefaultCurrency string        `json:"default_currency"`
	DataRetentionDays int         `json:"data_retention_days"`
	EnableCache     bool          `json:"enable_cache"`
	CacheExpiry     time.Duration `json:"cache_expiry"`
	OutputDir       string        `json:"output_dir"`
}

// DefaultReportConfig 默认报告配置
func DefaultReportConfig() ReportConfig {
	return ReportConfig{
		DefaultCurrency:   "CNY",
		DataRetentionDays: 365,
		EnableCache:       true,
		CacheExpiry:       5 * time.Minute,
		OutputDir:         "./reports",
	}
}

// NewBillingBillingReportGenerator 创建成本报告生成器
func NewBillingBillingReportGenerator(dataDir string, providers ReportDataProvider, config ReportConfig) *BillingBillingReportGenerator {
	return &BillingBillingReportGenerator{
		dataDir:     dataDir,
		providers:   providers,
		config:      config,
		reportCache: make(map[string]*BillingCostReport),
		cacheExpiry: config.CacheExpiry,
	}
}

// ========== 报告生成方法 ==========

// GenerateDailyReport 生成日报
func (g *BillingBillingReportGenerator) GenerateDailyReport(ctx context.Context, date time.Time) (*BillingCostReport, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	report := &BillingCostReport{
		ID:          generateReportID(BillingReportTypeDaily, start),
		BillingReportType:  BillingReportTypeDaily,
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		Currency:    g.config.DefaultCurrency,
		Metadata:    make(map[string]interface{}),
	}

	// 收集数据
	if err := g.collectReportData(ctx, report, start, end); err != nil {
		return nil, fmt.Errorf("收集报告数据失败: %w", err)
	}

	// 保存报告
	if err := g.saveReport(report); err != nil {
		return nil, fmt.Errorf("保存报告失败: %w", err)
	}

	return report, nil
}

// GenerateWeeklyReport 生成周报
func (g *BillingBillingReportGenerator) GenerateWeeklyReport(ctx context.Context, weekStart time.Time) (*BillingCostReport, error) {
	// 计算周的开始（周一）和结束（周日）
	start := weekStart
	for start.Weekday() != time.Monday {
		start = start.AddDate(0, 0, -1)
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end := start.AddDate(0, 0, 7)

	report := &BillingCostReport{
		ID:          generateReportID(BillingReportTypeWeekly, start),
		BillingReportType:  BillingReportTypeWeekly,
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		Currency:    g.config.DefaultCurrency,
		Metadata:    make(map[string]interface{}),
	}

	// 收集数据
	if err := g.collectReportData(ctx, report, start, end); err != nil {
		return nil, fmt.Errorf("收集报告数据失败: %w", err)
	}

	// 添加周环比分析
	if err := g.addWeeklyAnalysis(ctx, report); err != nil {
		// 非致命错误，继续
		report.Metadata["weekly_analysis_error"] = err.Error()
	}

	// 保存报告
	if err := g.saveReport(report); err != nil {
		return nil, fmt.Errorf("保存报告失败: %w", err)
	}

	return report, nil
}

// GenerateMonthlyReport 生成月报
func (g *BillingBillingReportGenerator) GenerateMonthlyReport(ctx context.Context, month time.Time) (*BillingCostReport, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)

	report := &BillingCostReport{
		ID:          generateReportID(BillingReportTypeMonthly, start),
		BillingReportType:  BillingReportTypeMonthly,
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		Currency:    g.config.DefaultCurrency,
		Metadata:    make(map[string]interface{}),
	}

	// 收集数据
	if err := g.collectReportData(ctx, report, start, end); err != nil {
		return nil, fmt.Errorf("收集报告数据失败: %w", err)
	}

	// 添加月度分析
	if err := g.addMonthlyAnalysis(ctx, report); err != nil {
		report.Metadata["monthly_analysis_error"] = err.Error()
	}

	// 保存报告
	if err := g.saveReport(report); err != nil {
		return nil, fmt.Errorf("保存报告失败: %w", err)
	}

	return report, nil
}

// GenerateCustomReport 生成自定义时间范围报告
func (g *BillingBillingReportGenerator) GenerateCustomReport(ctx context.Context, start, end time.Time) (*BillingCostReport, error) {
	report := &BillingCostReport{
		ID:          fmt.Sprintf("custom-%d-%s", start.Unix(), randomString(6)),
		BillingReportType:  BillingReportTypeMonthly, // 使用月报类型
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		Currency:    g.config.DefaultCurrency,
		Metadata:    make(map[string]interface{}),
	}

	// 收集数据
	if err := g.collectReportData(ctx, report, start, end); err != nil {
		return nil, fmt.Errorf("收集报告数据失败: %w", err)
	}

	return report, nil
}

// ========== 数据收集 ==========

// collectReportData 收集报告数据
func (g *BillingBillingReportGenerator) collectReportData(ctx context.Context, report *BillingCostReport, start, end time.Time) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 6)

	// 并发收集数据
	wg.Add(6)

	// 存储数据
	go func() {
		defer wg.Done()
		data, err := g.providers.GetStorageData(ctx, start, end)
		if err != nil {
			errChan <- fmt.Errorf("获取存储数据失败: %w", err)
			return
		}
		g.populateStorageSection(report, data)
	}()

	// 带宽数据
	go func() {
		defer wg.Done()
		data, err := g.providers.GetBandwidthData(ctx, start, end)
		if err != nil {
			errChan <- fmt.Errorf("获取带宽数据失败: %w", err)
			return
		}
		g.populateBandwidthSection(report, data)
	}()

	// 存储池数据
	go func() {
		defer wg.Done()
		data, err := g.providers.GetPoolData(ctx, start, end)
		if err != nil {
			errChan <- fmt.Errorf("获取存储池数据失败: %w", err)
			return
		}
		g.populatePoolBreakdown(report, data)
	}()

	// 用户数据
	go func() {
		defer wg.Done()
		data, err := g.providers.GetUserData(ctx, start, end)
		if err != nil {
			errChan <- fmt.Errorf("获取用户数据失败: %w", err)
			return
		}
		g.populateUserBreakdown(report, data)
	}()

	// 趋势数据
	go func() {
		defer wg.Done()
		data, err := g.providers.GetTrendData(ctx, start, end)
		if err != nil {
			errChan <- fmt.Errorf("获取趋势数据失败: %w", err)
			return
		}
		g.populateTrends(report, data)
	}()

	// 建议
	go func() {
		defer wg.Done()
		data, err := g.providers.GetRecommendations(ctx)
		if err != nil {
			errChan <- fmt.Errorf("获取建议失败: %w", err)
			return
		}
		report.Recommendations = data
	}()

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// 计算摘要
	g.calculateSummary(report)

	return nil
}

// populateStorageSection 填充存储部分
func (g *BillingBillingReportGenerator) populateStorageSection(report *BillingCostReport, data *StorageReportData) {
	report.StorageCost = StorageCostSection{
		TotalCapacityGB: data.TotalCapacityGB,
		UsedCapacityGB:  data.UsedCapacityGB,
		FreeCapacityGB:  data.TotalCapacityGB - data.UsedCapacityGB,
		UtilizationRate: data.UtilizationRate,
		MonthlyCost:     data.MonthlyCost,
		DailyCost:       data.DailyCost,
		AveragePrice:    data.AveragePrice,
		SSDCost:         data.SSDCost,
		SSDUsedGB:       data.SSDUsedGB,
		HDDCost:         data.HDDCost,
		HDDUsedGB:       data.HDDUsedGB,
		ArchiveCost:     data.ArchiveCost,
		ArchiveUsedGB:   data.ArchiveUsedGB,
		HotDataGB:       data.HotDataGB,
		HotDataCost:     data.HotDataCost,
		WarmDataGB:      data.WarmDataGB,
		WarmDataCost:    data.WarmDataCost,
		ColdDataGB:      data.ColdDataGB,
		ColdDataCost:    data.ColdDataCost,
		TierBreakdown:   data.TierBreakdown,
	}
}

// populateBandwidthSection 填充带宽部分
func (g *BillingBillingReportGenerator) populateBandwidthSection(report *BillingCostReport, data *BandwidthReportData) {
	report.BandwidthCost = BandwidthCostSection{
		InboundTrafficGB:  data.InboundTrafficGB,
		OutboundTrafficGB: data.OutboundTrafficGB,
		TotalTrafficGB:    data.TotalTrafficGB,
		PeakMbps:          data.PeakMbps,
		AverageMbps:       data.AverageMbps,
		Peak95Mbps:        data.Peak95Mbps,
		TotalCost:         data.TotalCost,
		TrafficCost:       data.TrafficCost,
		BandwidthCost:     data.BandwidthCost,
		BillingModel:      data.BillingModel,
		FreeAllowanceGB:   data.FreeAllowanceGB,
		PeakHours:         data.PeakHours,
	}
}

// populatePoolBreakdown 填充存储池分解
func (g *BillingBillingReportGenerator) populatePoolBreakdown(report *BillingCostReport, data []PoolReportData) {
	report.PoolBreakdown = make([]PoolCostItem, 0, len(data))
	for _, p := range data {
		usagePercent := 0.0
		if p.TotalCapacityGB > 0 {
			usagePercent = p.UsedCapacityGB / p.TotalCapacityGB * 100
		}
		costEfficiency := g.calculateCostEfficiency(usagePercent)

		report.PoolBreakdown = append(report.PoolBreakdown, PoolCostItem{
			PoolID:          p.PoolID,
			PoolName:        p.PoolName,
			StorageType:     p.StorageType,
			TotalCapacityGB: p.TotalCapacityGB,
			UsedCapacityGB:  p.UsedCapacityGB,
			UsagePercent:    usagePercent,
			PricePerGB:      p.PricePerGB,
			MonthlyCost:     p.MonthlyCost,
			CostEfficiency:  costEfficiency,
			HotDataGB:       p.HotDataGB,
			WarmDataGB:      p.WarmDataGB,
			ColdDataGB:      p.ColdDataGB,
			Trend:           p.Trend,
		})
	}
}

// populateUserBreakdown 填充用户分解
func (g *BillingBillingReportGenerator) populateUserBreakdown(report *BillingCostReport, data []UserReportData) {
	report.UserBreakdown = make([]UserCostItem, 0, len(data))
	for _, u := range data {
		report.UserBreakdown = append(report.UserBreakdown, UserCostItem{
			UserID:      u.UserID,
			UserName:    u.UserName,
			UsedGB:      u.UsedGB,
			MonthlyCost: u.MonthlyCost,
			CostPerGB:   u.CostPerGB,
			Tier:        u.Tier,
			Trend:       u.Trend,
			PoolUsage:   u.PoolUsage,
		})
	}
}

// populateTrends 填充趋势数据
func (g *BillingBillingReportGenerator) populateTrends(report *BillingCostReport, data []TrendReportData) {
	report.Trends = make([]BillingTrendItem, 0, len(data))
	var cumulative float64

	for _, t := range data {
		totalCost := t.StorageCost + t.BandwidthCost
		cumulative += totalCost

		report.Trends = append(report.Trends, BillingTrendItem{
			Date:           t.Date,
			StorageCost:    t.StorageCost,
			BandwidthCost:  t.BandwidthCost,
			TotalCost:      totalCost,
			StorageGB:      t.StorageGB,
			TrafficGB:      t.TrafficGB,
			CumulativeCost: cumulative,
		})
	}
}

// calculateSummary 计算摘要
func (g *BillingBillingReportGenerator) calculateSummary(report *BillingCostReport) {
	summary := BillingReportSummary{
		TotalCost:         report.StorageCost.MonthlyCost + report.BandwidthCost.TotalCost,
		StorageCost:       report.StorageCost.MonthlyCost,
		BandwidthCost:     report.BandwidthCost.TotalCost,
		TotalStorageGB:    report.StorageCost.UsedCapacityGB,
		TotalTrafficGB:    report.BandwidthCost.TotalTrafficGB,
		StorageUtilization: report.StorageCost.UtilizationRate,
	}

	// 计算健康评分
	summary.HealthScore = g.calculateHealthScore(report, &summary)

	// 设置预算状态
	summary.BudgetStatus = "on_track"
	if report.BudgetComparison != nil {
		summary.BudgetStatus = report.BudgetComparison.Status
	}

	report.Summary = summary
}

// calculateHealthScore 计算健康评分
func (g *BillingBillingReportGenerator) calculateHealthScore(report *BillingCostReport, summary *BillingReportSummary) int {
	score := 100

	// 利用率评分
	util := report.StorageCost.UtilizationRate
	if util < 30 {
		score -= int(30 - util) // 低利用率扣分
	} else if util > 90 {
		score -= int(util - 90) // 高利用率扣分
	}

	// 预算状态评分
	if report.BudgetComparison != nil {
		switch report.BudgetComparison.AlertLevel {
		case "critical":
			score -= 30
		case "warning":
			score -= 15
		}
	}

	// 冷数据过多扣分
	coldPercent := 0.0
	if report.StorageCost.UsedCapacityGB > 0 {
		coldPercent = report.StorageCost.ColdDataGB / report.StorageCost.UsedCapacityGB * 100
	}
	if coldPercent > 50 {
		score -= int((coldPercent - 50) / 2)
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// calculateCostEfficiency 计算成本效率
func (g *BillingBillingReportGenerator) calculateCostEfficiency(usagePercent float64) float64 {
	if usagePercent >= 60 && usagePercent <= 80 {
		return 1.0
	} else if usagePercent > 80 {
		return 1 - (usagePercent-80)/100
	} else {
		return usagePercent / 60
	}
}

// addWeeklyAnalysis 添加周环比分析
func (g *BillingBillingReportGenerator) addWeeklyAnalysis(ctx context.Context, report *BillingCostReport) error {
	// 获取上周报告进行对比
	prevWeekStart := report.PeriodStart.AddDate(0, 0, -7)
	prevReport, err := g.providers.GetHistoricalReport(ctx, BillingReportTypeWeekly, prevWeekStart)
	if err != nil {
		return err
	}

	// 计算环比变化
	if prevReport.Summary.TotalCost > 0 {
		report.Summary.CostChange = report.Summary.TotalCost - prevReport.Summary.TotalCost
		report.Summary.CostChangePercent = (report.Summary.TotalCost - prevReport.Summary.TotalCost) / prevReport.Summary.TotalCost * 100
	}
	report.Summary.StorageChange = report.Summary.StorageCost - prevReport.Summary.StorageCost
	report.Summary.BandwidthChange = report.Summary.BandwidthCost - prevReport.Summary.BandwidthCost

	return nil
}

// addMonthlyAnalysis 添加月度分析
func (g *BillingBillingReportGenerator) addMonthlyAnalysis(ctx context.Context, report *BillingCostReport) error {
	// 获取上月报告进行对比
	prevMonthStart := report.PeriodStart.AddDate(0, -1, 0)
	prevReport, err := g.providers.GetHistoricalReport(ctx, BillingReportTypeMonthly, prevMonthStart)
	if err != nil {
		return err
	}

	// 计算环比变化
	if prevReport.Summary.TotalCost > 0 {
		report.Summary.CostChange = report.Summary.TotalCost - prevReport.Summary.TotalCost
		report.Summary.CostChangePercent = (report.Summary.TotalCost - prevReport.Summary.TotalCost) / prevReport.Summary.TotalCost * 100
	}
	report.Summary.StorageChange = report.Summary.StorageCost - prevReport.Summary.StorageCost
	report.Summary.BandwidthChange = report.Summary.BandwidthCost - prevReport.Summary.BandwidthCost

	// 预测下月成本
	if len(report.Trends) > 0 {
		var totalTrendCost float64
		for _, t := range report.Trends {
			totalTrendCost += t.TotalCost
		}
		avgDailyCost := totalTrendCost / float64(len(report.Trends))
		report.Summary.ProjectedNextPeriod = avgDailyCost * 30
	}

	return nil
}

// ========== 导出方法 ==========

// ExportReport 导出报告
func (g *BillingBillingReportGenerator) ExportReport(report *BillingCostReport, format BillingExportFormat, outputPath string) error {
	switch format {
	case BillingExportFormatJSON:
		return g.exportJSON(report, outputPath)
	case BillingExportFormatCSV:
		return g.exportCSV(report, outputPath)
	default:
		return ErrInvalidBillingReportType
	}
}

// exportJSON 导出JSON格式
func (g *BillingBillingReportGenerator) exportJSON(report *BillingCostReport, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化报告失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// exportCSV 导出CSV格式
func (g *BillingBillingReportGenerator) exportCSV(report *BillingCostReport, outputPath string) error {
	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入报告头
	headers := []string{
		"Report ID", "Report Type", "Generated At", "Period Start", "Period End", "Currency",
		"Total Cost", "Storage Cost", "Bandwidth Cost",
		"Total Storage GB", "Total Traffic GB", "Health Score",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	// 写入报告摘要
	row := []string{
		report.ID,
		string(report.BillingReportType),
		report.GeneratedAt.Format(time.RFC3339),
		report.PeriodStart.Format(time.RFC3339),
		report.PeriodEnd.Format(time.RFC3339),
		report.Currency,
		fmt.Sprintf("%.2f", report.Summary.TotalCost),
		fmt.Sprintf("%.2f", report.Summary.StorageCost),
		fmt.Sprintf("%.2f", report.Summary.BandwidthCost),
		fmt.Sprintf("%.2f", report.Summary.TotalStorageGB),
		fmt.Sprintf("%.2f", report.Summary.TotalTrafficGB),
		fmt.Sprintf("%d", report.Summary.HealthScore),
	}
	if err := writer.Write(row); err != nil {
		return err
	}

	// 写入空行
	writer.Write([]string{})

	// 写入存储池明细
	writer.Write([]string{"Pool Breakdown"})
	poolHeaders := []string{"Pool ID", "Pool Name", "Storage Type", "Used GB", "Usage %", "Monthly Cost", "Cost Efficiency"}
	writer.Write(poolHeaders)

	for _, p := range report.PoolBreakdown {
		row := []string{
			p.PoolID,
			p.PoolName,
			p.StorageType,
			fmt.Sprintf("%.2f", p.UsedCapacityGB),
			fmt.Sprintf("%.1f", p.UsagePercent),
			fmt.Sprintf("%.2f", p.MonthlyCost),
			fmt.Sprintf("%.2f", p.CostEfficiency),
		}
		writer.Write(row)
	}

	// 写入空行
	writer.Write([]string{})

	// 写入用户明细
	writer.Write([]string{"User Breakdown"})
	userHeaders := []string{"User ID", "User Name", "Used GB", "Monthly Cost", "Cost Per GB", "Tier"}
	writer.Write(userHeaders)

	for _, u := range report.UserBreakdown {
		row := []string{
			u.UserID,
			u.UserName,
			fmt.Sprintf("%.2f", u.UsedGB),
			fmt.Sprintf("%.2f", u.MonthlyCost),
			fmt.Sprintf("%.4f", u.CostPerGB),
			u.Tier,
		}
		writer.Write(row)
	}

	// 写入空行
	writer.Write([]string{})

	// 写入趋势数据
	writer.Write([]string{"Trend Data"})
	trendHeaders := []string{"Date", "Storage Cost", "Bandwidth Cost", "Total Cost", "Storage GB", "Traffic GB"}
	writer.Write(trendHeaders)

	for _, t := range report.Trends {
		row := []string{
			t.Date.Format("2006-01-02"),
			fmt.Sprintf("%.2f", t.StorageCost),
			fmt.Sprintf("%.2f", t.BandwidthCost),
			fmt.Sprintf("%.2f", t.TotalCost),
			fmt.Sprintf("%.2f", t.StorageGB),
			fmt.Sprintf("%.2f", t.TrafficGB),
		}
		writer.Write(row)
	}

	return nil
}

// ExportToJSON 导出为JSON字符串
func (g *BillingBillingReportGenerator) ExportToJSON(report *BillingCostReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化报告失败: %w", err)
	}
	return string(data), nil
}

// ExportToCSV 导出为CSV字符串
func (g *BillingBillingReportGenerator) ExportToCSV(report *BillingCostReport) (string, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	defer writer.Flush()

	// 简化的CSV输出
	headers := []string{"Category", "Metric", "Value"}
	writer.Write(headers)

	// 摘要数据
	writer.Write([]string{"Summary", "Total Cost", fmt.Sprintf("%.2f", report.Summary.TotalCost)})
	writer.Write([]string{"Summary", "Storage Cost", fmt.Sprintf("%.2f", report.Summary.StorageCost)})
	writer.Write([]string{"Summary", "Bandwidth Cost", fmt.Sprintf("%.2f", report.Summary.BandwidthCost)})
	writer.Write([]string{"Summary", "Health Score", fmt.Sprintf("%d", report.Summary.HealthScore)})

	// 存储池数据
	for _, p := range report.PoolBreakdown {
		writer.Write([]string{"Pool", p.PoolName, fmt.Sprintf("%.2f GB / %.2f 元", p.UsedCapacityGB, p.MonthlyCost)})
	}

	return builder.String(), nil
}

// ========== 报告管理 ==========

// GetReport 获取报告
func (g *BillingBillingReportGenerator) GetReport(id string) (*BillingCostReport, error) {
	// 先查缓存
	if g.config.EnableCache {
		g.mu.RLock()
		if report, ok := g.reportCache[id]; ok {
			g.mu.RUnlock()
			return report, nil
		}
		g.mu.RUnlock()
	}

	// 从文件加载
	reportPath := filepath.Join(g.dataDir, "reports", id+".json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, ErrReportNotFound
	}

	var report BillingCostReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("解析报告失败: %w", err)
	}

	// 缓存
	if g.config.EnableCache {
		g.mu.Lock()
		g.reportCache[id] = &report
		g.mu.Unlock()
	}

	return &report, nil
}

// ListReports 列出报告
func (g *BillingBillingReportGenerator) ListReports(reportType BillingReportType, limit int) ([]*BillingCostReport, error) {
	reportsDir := filepath.Join(g.dataDir, "reports")
	files, err := os.ReadDir(reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*BillingCostReport{}, nil
		}
		return nil, err
	}

	var reports []*BillingCostReport
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(f.Name(), ".json")
		if reportType != "" && !strings.HasPrefix(id, string(reportType)) {
			continue
		}

		report, err := g.GetReport(id)
		if err != nil {
			continue
		}

		reports = append(reports, report)
		if limit > 0 && len(reports) >= limit {
			break
		}
	}

	return reports, nil
}

// DeleteReport 删除报告
func (g *BillingBillingReportGenerator) DeleteReport(id string) error {
	reportPath := filepath.Join(g.dataDir, "reports", id+".json")

	// 删除文件
	if err := os.Remove(reportPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 清除缓存
	g.mu.Lock()
	delete(g.reportCache, id)
	g.mu.Unlock()

	return nil
}

// saveReport 保存报告
func (g *BillingBillingReportGenerator) saveReport(report *BillingCostReport) error {
	reportsDir := filepath.Join(g.dataDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return err
	}

	reportPath := filepath.Join(reportsDir, report.ID+".json")
	return g.exportJSON(report, reportPath)
}

// ========== 辅助函数 ==========

// generateReportID 生成报告ID
func generateReportID(reportType BillingReportType, date time.Time) string {
	return fmt.Sprintf("%s-%s", reportType, date.Format("20060102"))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}

// CleanupOldReports 清理过期报告
func (g *BillingBillingReportGenerator) CleanupOldReports() error {
	cutoff := time.Now().AddDate(0, 0, -g.config.DataRetentionDays)

	reports, err := g.ListReports("", 0)
	if err != nil {
		return err
	}

	for _, report := range reports {
		if report.GeneratedAt.Before(cutoff) {
			if err := g.DeleteReport(report.ID); err != nil {
				// 记录错误但继续清理
				continue
			}
		}
	}

	return nil
}