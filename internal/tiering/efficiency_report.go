// Package tiering 存储分层效率报告
// 参考群晖 DSM Tiering 实现
package tiering

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// EfficiencyReport 分层效率报告
type EfficiencyReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Period      string    `json:"period"` // daily, weekly, monthly

	// 冷热数据分布可视化
	DataDistribution *DataDistributionReport `json:"dataDistribution"`

	// 迁移效率统计
	MigrationEfficiency *MigrationEfficiencyReport `json:"migrationEfficiency"`

	// 成本分析报告
	CostAnalysis *CostAnalysisReport `json:"costAnalysis"`

	// 容量趋势预测
	CapacityForecast *CapacityForecastReport `json:"capacityForecast"`

	// 分层健康评分
	HealthScore *TieringHealthScore `json:"healthScore"`

	// 建议
	Recommendations []TieringRecommendation `json:"recommendations"`
}

// DataDistributionReport 冷热数据分布可视化报告
type DataDistributionReport struct {
	// 总体分布
	TotalFiles int64 `json:"totalFiles"`
	TotalBytes int64 `json:"totalBytes"`

	// 冷热数据分布
	HotData  *DataSegment `json:"hotData"`
	WarmData *DataSegment `json:"warmData"`
	ColdData *DataSegment `json:"coldData"`

	// 按存储层分布
	ByTier map[TierType]*TierDataDistribution `json:"byTier"`

	// 分布图表数据（用于前端可视化）
	ChartData *DistributionChartData `json:"chartData"`

	// 访问模式分析
	AccessPatterns []AccessPattern `json:"accessPatterns"`
}

// DataSegment 数据分段统计
type DataSegment struct {
	Files      int64   `json:"files"`
	Bytes      int64   `json:"bytes"`
	Percentage float64 `json:"percentage"`
	AvgSize    int64   `json:"avgSize"`
}

// TierDataDistribution 存储层数据分布
type TierDataDistribution struct {
	TierType TierType `json:"tierType"`

	// 容量信息
	Capacity  int64 `json:"capacity"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`

	// 数据分布
	HotFiles  int64 `json:"hotFiles"`
	HotBytes  int64 `json:"hotBytes"`
	WarmFiles int64 `json:"warmFiles"`
	WarmBytes int64 `json:"warmBytes"`
	ColdFiles int64 `json:"coldFiles"`
	ColdBytes int64 `json:"coldBytes"`

	// 利用率
	UsagePercent      float64 `json:"usagePercent"`
	HotDataPercent    float64 `json:"hotDataPercent"`
	OptimizationScore float64 `json:"optimizationScore"` // 优化评分（热数据在高优先级层的比例）
}

// DistributionChartData 分布图表数据
type DistributionChartData struct {
	// 饼图数据（冷热分布）
	PieChart []PieChartItem `json:"pieChart"`

	// 柱状图数据（按存储层）
	BarChart []BarChartItem `json:"barChart"`

	// 热力图数据（访问频率热力图）
	Heatmap [][]float64 `json:"heatmap"`

	// 时间序列数据（访问趋势）
	TimeSeries []TimeSeriesPoint `json:"timeSeries"`
}

// PieChartItem 饼图数据项
type PieChartItem struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Color string  `json:"color"`
}

// BarChartItem 柱状图数据项
type BarChartItem struct {
	Label    string            `json:"label"`
	Values   map[string]int64  `json:"values"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	HotData   int64     `json:"hotData"`
	WarmData  int64     `json:"warmData"`
	ColdData  int64     `json:"coldData"`
}

// AccessPattern 访问模式
type AccessPattern struct {
	PatternType string  `json:"patternType"` // sequential, random, mixed
	Percentage  float64 `json:"percentage"`
	Description string  `json:"description"`
}

// MigrationEfficiencyReport 迁移效率统计报告
type MigrationEfficiencyReport struct {
	// 统计周期
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`

	// 迁移统计
	TotalMigrations      int64 `json:"totalMigrations"`
	SuccessfulMigrations int64 `json:"successfulMigrations"`
	FailedMigrations     int64 `json:"failedMigrations"`

	// 数据量统计
	TotalBytesMigrated  int64 `json:"totalBytesMigrated"`
	TotalFilesMigrated  int64 `json:"totalFilesMigrated"`
	AvgFileSizeMigrated int64 `json:"avgFileSizeMigrated"`

	// 性能指标
	AvgMigrationTimeMs int64   `json:"avgMigrationTimeMs"`
	AvgThroughputMBps  float64 `json:"avgThroughputMBps"`
	PeakThroughputMBps float64 `json:"peakThroughputMBps"`

	// 迁移成功率
	SuccessRate float64 `json:"successRate"`

	// 按存储层统计
	ByTier map[TierType]*TierMigrationStatsReport `json:"byTier"`

	// 按策略统计
	ByPolicy map[string]*PolicyMigrationStatsReport `json:"byPolicy"`

	// 迁移趋势
	MigrationTrend []MigrationTrendPoint `json:"migrationTrend"`

	// 效率评分
	EfficiencyScore float64 `json:"efficiencyScore"`
}

// TierMigrationStatsReport 存储层迁移统计报告
type TierMigrationStatsReport struct {
	TierType         TierType `json:"tierType"`
	FilesPromoted    int64    `json:"filesPromoted"` // 提升到该层的文件数
	BytesPromoted    int64    `json:"bytesPromoted"`
	FilesDemoted     int64    `json:"filesDemoted"` // 从该层降级的文件数
	BytesDemoted     int64    `json:"bytesDemoted"`
	FilesArchived    int64    `json:"filesArchived"` // 归档文件数
	BytesArchived    int64    `json:"bytesArchived"`
	AvgMigrationTime int64    `json:"avgMigrationTime"` // 平均迁移时间(ms)
	SuccessRate      float64  `json:"successRate"`
}

// PolicyMigrationStatsReport 策略迁移统计报告
type PolicyMigrationStatsReport struct {
	PolicyID       string    `json:"policyId"`
	PolicyName     string    `json:"policyName"`
	ExecutionCount int64     `json:"executionCount"`
	TotalBytes     int64     `json:"totalBytes"`
	TotalFiles     int64     `json:"totalFiles"`
	AvgDuration    int64     `json:"avgDuration"`
	SuccessRate    float64   `json:"successRate"`
	LastRun        time.Time `json:"lastRun"`
}

// MigrationTrendPoint 迁移趋势数据点
type MigrationTrendPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	BytesMigrated int64     `json:"bytesMigrated"`
	FilesMigrated int64     `json:"filesMigrated"`
	AvgSpeed      float64   `json:"avgSpeed"`
}

// CostAnalysisReport 成本分析报告
type CostAnalysisReport struct {
	// 成本概览
	TotalMonthlyCost float64 `json:"totalMonthlyCost"`
	TotalYearlyCost  float64 `json:"totalYearlyCost"`
	CostPerGB        float64 `json:"costPerGB"`

	// 按存储层成本
	TierCosts map[TierType]*TierCostInfo `json:"tierCosts"`

	// 成本节省分析
	CostSavings *CostSavingsInfo `json:"costSavings"`

	// 成本趋势预测
	CostForecast []CostForecastPoint `json:"costForecast"`

	// 优化建议
	OptimizationOpportunities []CostOptimizationOpportunity `json:"optimizationOpportunities"`

	// ROI 分析
	ROI *ROIAnalysis `json:"roi"`
}

// TierCostInfo 存储层成本信息
type TierCostInfo struct {
	TierType       TierType `json:"tierType"`
	CostPerGBMonth float64  `json:"costPerGBMonth"` // 每GB每月成本
	MonthlyCost    float64  `json:"monthlyCost"`
	YearlyCost     float64  `json:"yearlyCost"`
	UsedGB         float64  `json:"usedGB"`
	CapacityGB     float64  `json:"capacityGB"`
	Utilization    float64  `json:"utilization"`
}

// CostSavingsInfo 成本节省信息
type CostSavingsInfo struct {
	// 通过分层实现的节省
	MonthlySavings float64 `json:"monthlySavings"`
	YearlySavings  float64 `json:"yearlySavings"`
	SavingsPercent float64 `json:"savingsPercent"`

	// 潜在节省（如果进一步优化）
	PotentialMonthlySavings float64 `json:"potentialMonthlySavings"`
	PotentialYearlySavings  float64 `json:"potentialYearlySavings"`

	// 节省来源分析
	SavingsByTier map[TierType]float64 `json:"savingsByTier"`

	// 节省趋势
	SavingsTrend []SavingsTrendPoint `json:"savingsTrend"`
}

// SavingsTrendPoint 节省趋势数据点
type SavingsTrendPoint struct {
	Month      time.Time `json:"month"`
	Savings    float64   `json:"savings"`
	Cumulative float64   `json:"cumulative"`
}

// CostForecastPoint 成本预测数据点
type CostForecastPoint struct {
	Month          time.Time `json:"month"`
	ProjectedCost  float64   `json:"projectedCost"`
	ProjectedUsage float64   `json:"projectedUsage"`
	Confidence     float64   `json:"confidence"` // 预测置信度
}

// CostOptimizationOpportunity 成本优化机会
type CostOptimizationOpportunity struct {
	Type             string                 `json:"type"` // archive_cold_data, resize_tier, change_policy
	Description      string                 `json:"description"`
	PotentialSavings float64                `json:"potentialSavings"`
	Effort           string                 `json:"effort"` // low, medium, high
	Priority         int                    `json:"priority"`
	Details          map[string]interface{} `json:"details"`
}

// ROIAnalysis ROI 分析
type ROIAnalysis struct {
	// 分层系统投资成本
	InitialInvestment float64 `json:"initialInvestment"`
	MonthlyOpCost     float64 `json:"monthlyOpCost"`

	// 收益
	MonthlyBenefit float64 `json:"monthlyBenefit"`
	YearlyBenefit  float64 `json:"yearlyBenefit"`

	// ROI 指标
	ROI           float64 `json:"roi"`           // 投资回报率
	PaybackPeriod float64 `json:"paybackPeriod"` // 回收期(月)
	NPV           float64 `json:"npv"`           // 净现值
}

// CapacityForecastReport 容量趋势预测报告
type CapacityForecastReport struct {
	// 预测时间范围
	ForecastDays int `json:"forecastDays"`

	// 总体容量预测
	TotalCapacity *CapacityPredict `json:"totalCapacity"`

	// 按存储层预测
	ByTier map[TierType]*CapacityPredict `json:"byTier"`

	// 容量警告
	Warnings []CapacityWarning `json:"warnings"`

	// 预测数据点
	ForecastPoints []CapacityForecastPoint `json:"forecastPoints"`

	// 预测模型信息
	ModelInfo *ForecastModelInfo `json:"modelInfo"`
}

// CapacityPredict 容量预测
type CapacityPredict struct {
	TierType        TierType `json:"tierType"`
	CurrentUsed     int64    `json:"currentUsed"`
	CurrentCapacity int64    `json:"currentCapacity"`
	GrowthRateDaily float64  `json:"growthRateDaily"` // 每日增长率(GB)

	// 预测时间点
	DaysToFull       int   `json:"daysToFull"`       // 预计多少天填满
	PredictedUsed7D  int64 `json:"predictedUsed7D"`  // 7天后预测
	PredictedUsed30D int64 `json:"predictedUsed30D"` // 30天后预测
	PredictedUsed90D int64 `json:"predictedUsed90D"` // 90天后预测

	// 置信度
	Confidence float64 `json:"confidence"`
}

// CapacityWarning 容量警告
type CapacityWarning struct {
	TierType      TierType `json:"tierType"`
	WarningType   string   `json:"warningType"` // approaching_full, growth_accelerating
	Severity      string   `json:"severity"`    // info, warning, critical
	Message       string   `json:"message"`
	DaysRemaining int      `json:"daysRemaining"`
	Suggestion    string   `json:"suggestion"`
}

// CapacityForecastPoint 容量预测数据点
type CapacityForecastPoint struct {
	Date          time.Time `json:"date"`
	TierType      TierType  `json:"tierType"`
	PredictedUsed int64     `json:"predictedUsed"`
	UpperBound    int64     `json:"upperBound"` // 上界
	LowerBound    int64     `json:"lowerBound"` // 下界
	Confidence    float64   `json:"confidence"`
}

// ForecastModelInfo 预测模型信息
type ForecastModelInfo struct {
	ModelType   string    `json:"modelType"`  // linear, exponential, arima
	Accuracy    float64   `json:"accuracy"`   // 模型准确度
	DataPoints  int       `json:"dataPoints"` // 使用的数据点数
	LastTrained time.Time `json:"lastTrained"`
	Features    []string  `json:"features"` // 使用的特征
}

// TieringHealthScore 分层健康评分
type TieringHealthScore struct {
	OverallScore float64 `json:"overallScore"` // 0-100
	Grade        string  `json:"grade"`        // A, B, C, D, F

	// 子评分
	DistributionScore float64 `json:"distributionScore"` // 数据分布合理性
	EfficiencyScore   float64 `json:"efficiencyScore"`   // 迁移效率
	CostScore         float64 `json:"costScore"`         // 成本优化程度
	CapacityScore     float64 `json:"capacityScore"`     // 容量健康度
	PolicyScore       float64 `json:"policyScore"`       // 策略配置合理性

	// 评分详情
	ScoreBreakdown []ScoreItem `json:"scoreBreakdown"`
}

// ScoreItem 评分项
type ScoreItem struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	MaxScore    float64 `json:"maxScore"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// TieringRecommendation 分层建议
type TieringRecommendation struct {
	Type        string                 `json:"type"`     // migrate, archive, resize, policy_change
	Priority    int                    `json:"priority"` // 1-5
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Impact      string                 `json:"impact"` // 高/中/低影响
	Effort      string                 `json:"effort"` // low, medium, high
	Details     map[string]interface{} `json:"details"`
}

// EfficiencyReportGenerator 效率报告生成器
type EfficiencyReportGenerator struct {
	manager *Manager
	metrics *Metrics

	// 历史数据缓存
	historyMu  sync.RWMutex
	dailyStats []DailyStats
	// weeklyStats 和 monthlyStats 保留供将来扩展
	// weeklyStats  []WeeklyStats
	// monthlyStats []MonthlyStats

	// 成本配置
	costConfig *CostConfig
}

// DailyStats 每日统计
type DailyStats struct {
	Date            time.Time `json:"date"`
	TotalBytes      int64     `json:"totalBytes"`
	HotBytes        int64     `json:"hotBytes"`
	WarmBytes       int64     `json:"warmBytes"`
	ColdBytes       int64     `json:"coldBytes"`
	MigrationsCount int64     `json:"migrationsCount"`
	BytesMigrated   int64     `json:"bytesMigrated"`
}

// WeeklyStats 每周统计
type WeeklyStats struct {
	WeekStart       time.Time `json:"weekStart"`
	TotalBytes      int64     `json:"totalBytes"`
	HotBytes        int64     `json:"hotBytes"`
	WarmBytes       int64     `json:"warmBytes"`
	ColdBytes       int64     `json:"coldBytes"`
	MigrationsCount int64     `json:"migrationsCount"`
	BytesMigrated   int64     `json:"bytesMigrated"`
	AvgThroughput   float64   `json:"avgThroughput"`
}

// MonthlyStats 每月统计
type MonthlyStats struct {
	MonthStart      time.Time `json:"monthStart"`
	TotalBytes      int64     `json:"totalBytes"`
	HotBytes        int64     `json:"hotBytes"`
	WarmBytes       int64     `json:"warmBytes"`
	ColdBytes       int64     `json:"coldBytes"`
	MigrationsCount int64     `json:"migrationsCount"`
	BytesMigrated   int64     `json:"bytesMigrated"`
	MonthlyCost     float64   `json:"monthlyCost"`
	CostSavings     float64   `json:"costSavings"`
}

// CostConfig 成本配置
type CostConfig struct {
	// 各存储层成本（每GB每月）
	SSDCostPerGBMonth   float64 `json:"ssdCostPerGBMonth"`
	HDDCostPerGBMonth   float64 `json:"hddCostPerGBMonth"`
	CloudCostPerGBMonth float64 `json:"cloudCostPerGBMonth"`

	// 云存储额外成本
	CloudEgressCostPerGB float64 `json:"cloudEgressCostPerGB"` // 出口流量成本
	CloudRequestCost     float64 `json:"cloudRequestCost"`     // 请求成本

	// 分层系统运维成本
	MonthlyOpCost float64 `json:"monthlyOpCost"`
}

// DefaultCostConfig 默认成本配置
func DefaultCostConfig() *CostConfig {
	return &CostConfig{
		SSDCostPerGBMonth:    0.10, // SSD 较贵
		HDDCostPerGBMonth:    0.03, // HDD 便宜
		CloudCostPerGBMonth:  0.01, // 云存储归档最便宜
		CloudEgressCostPerGB: 0.05, // 出口流量
		CloudRequestCost:     0.0001,
		MonthlyOpCost:        50.0,
	}
}

// NewEfficiencyReportGenerator 创建效率报告生成器
func NewEfficiencyReportGenerator(manager *Manager, metrics *Metrics, costConfig *CostConfig) *EfficiencyReportGenerator {
	if costConfig == nil {
		costConfig = DefaultCostConfig()
	}
	return &EfficiencyReportGenerator{
		manager:    manager,
		metrics:    metrics,
		costConfig: costConfig,
	}
}

// GenerateReport 生成分层效率报告
func (g *EfficiencyReportGenerator) GenerateReport(period string) (*EfficiencyReport, error) {
	report := &EfficiencyReport{
		GeneratedAt:         time.Now(),
		Period:              period,
		DataDistribution:    g.generateDataDistribution(),
		MigrationEfficiency: g.generateMigrationEfficiency(period),
		CostAnalysis:        g.generateCostAnalysis(),
		CapacityForecast:    g.generateCapacityForecast(90),
		HealthScore:         g.calculateHealthScore(),
		Recommendations:     g.generateRecommendations(),
	}

	return report, nil
}

// generateDataDistribution 生成数据分布报告
func (g *EfficiencyReportGenerator) generateDataDistribution() *DataDistributionReport {
	report := &DataDistributionReport{
		ByTier:         make(map[TierType]*TierDataDistribution),
		AccessPatterns: make([]AccessPattern, 0),
	}

	// 获取访问统计
	accessStats := g.manager.GetAccessStats()
	report.TotalFiles = accessStats.TotalFiles
	report.TotalBytes = accessStats.TotalReadBytes + accessStats.TotalWriteBytes

	// 计算冷热数据分布
	totalFiles := accessStats.TotalFiles
	if totalFiles == 0 {
		totalFiles = 1 // 避免除零
	}

	report.HotData = &DataSegment{
		Files:      accessStats.HotFiles,
		Percentage: float64(accessStats.HotFiles) / float64(totalFiles) * 100,
	}

	report.WarmData = &DataSegment{
		Files:      accessStats.WarmFiles,
		Percentage: float64(accessStats.WarmFiles) / float64(totalFiles) * 100,
	}

	report.ColdData = &DataSegment{
		Files:      accessStats.ColdFiles,
		Percentage: float64(accessStats.ColdFiles) / float64(totalFiles) * 100,
	}

	// 按存储层统计
	tiers := g.manager.ListTiers()
	for _, tier := range tiers {
		tierStats, err := g.manager.GetTierStats(tier.Type)
		if err != nil {
			continue
		}

		dist := &TierDataDistribution{
			TierType:          tier.Type,
			Capacity:          tier.Capacity,
			Used:              tier.Used,
			Available:         tier.Capacity - tier.Used,
			HotFiles:          tierStats.HotFiles,
			HotBytes:          tierStats.HotBytes,
			WarmFiles:         tierStats.WarmFiles,
			WarmBytes:         tierStats.WarmBytes,
			ColdFiles:         tierStats.ColdFiles,
			ColdBytes:         tierStats.ColdBytes,
			UsagePercent:      tierStats.UsagePercent,
			HotDataPercent:    float64(tierStats.HotBytes) / float64(tierStats.TotalBytes+1) * 100,
			OptimizationScore: g.calculateTierOptimizationScore(tier.Type, tierStats),
		}

		report.ByTier[tier.Type] = dist
	}

	// 生成图表数据
	report.ChartData = g.generateChartData(report)

	// 分析访问模式
	report.AccessPatterns = g.analyzeAccessPatterns(accessStats)

	return report
}

// generateMigrationEfficiency 生成迁移效率报告
func (g *EfficiencyReportGenerator) generateMigrationEfficiency(period string) *MigrationEfficiencyReport {
	report := &MigrationEfficiencyReport{
		PeriodStart: time.Now().AddDate(0, 0, -30),
		PeriodEnd:   time.Now(),
		ByTier:      make(map[TierType]*TierMigrationStatsReport),
		ByPolicy:    make(map[string]*PolicyMigrationStatsReport),
	}

	// 获取迁移指标
	metrics := g.metrics.GetMigrationMetrics()
	if metrics != nil {
		report.TotalMigrations = metrics.TotalTasks
		report.SuccessfulMigrations = metrics.CompletedTasks
		report.FailedMigrations = metrics.FailedTasks
		report.TotalBytesMigrated = metrics.TotalBytesMigrated
		report.TotalFilesMigrated = metrics.TotalFilesMigrated
		report.AvgMigrationTimeMs = metrics.AverageMigrationTimeMs

		if metrics.TotalTasks > 0 {
			report.SuccessRate = float64(metrics.CompletedTasks) / float64(metrics.TotalTasks) * 100
		}

		// 计算平均吞吐量
		if metrics.TotalMigrationTimeMs > 0 {
			report.AvgThroughputMBps = float64(metrics.TotalBytesMigrated) / 1024 / 1024 / (float64(metrics.TotalMigrationTimeMs) / 1000)
		}
	}

	// 按存储层统计
	for _, tier := range g.manager.ListTiers() {
		tierMetrics := g.metrics.GetTierMetrics(tier.Type)
		if tierMetrics == nil {
			continue
		}

		stats := &TierMigrationStatsReport{
			TierType:      tier.Type,
			FilesPromoted: tierMetrics.FilesMigratedIn,
			BytesPromoted: tierMetrics.BytesMigratedIn,
			FilesDemoted:  tierMetrics.FilesMigratedOut,
			BytesDemoted:  tierMetrics.BytesMigratedOut,
		}

		totalFiles := stats.FilesPromoted + stats.FilesDemoted
		if totalFiles > 0 {
			stats.SuccessRate = float64(totalFiles) / float64(totalFiles) * 100 // 简化计算
		}

		report.ByTier[tier.Type] = stats
	}

	// 按策略统计
	for policyID, policyMetrics := range metrics.ByPolicy {
		policy, err := g.manager.GetPolicy(policyID)
		policyName := policyID
		if err == nil && policy != nil {
			policyName = policy.Name
		}

		stats := &PolicyMigrationStatsReport{
			PolicyID:       policyID,
			PolicyName:     policyName,
			ExecutionCount: policyMetrics.ExecutionCount,
			TotalBytes:     policyMetrics.TotalBytes,
			TotalFiles:     policyMetrics.TotalFiles,
			AvgDuration:    policyMetrics.AverageDurationMs,
			LastRun:        policyMetrics.LastRunTime,
		}

		if policyMetrics.ExecutionCount > 0 {
			stats.SuccessRate = float64(policyMetrics.SuccessCount) / float64(policyMetrics.ExecutionCount) * 100
		}

		report.ByPolicy[policyID] = stats
	}

	// 计算效率评分
	report.EfficiencyScore = g.calculateMigrationEfficiencyScore(report)

	return report
}

// generateCostAnalysis 生成成本分析报告
func (g *EfficiencyReportGenerator) generateCostAnalysis() *CostAnalysisReport {
	report := &CostAnalysisReport{
		TierCosts:                 make(map[TierType]*TierCostInfo),
		OptimizationOpportunities: make([]CostOptimizationOpportunity, 0),
	}

	// 计算各存储层成本
	totalMonthlyCost := 0.0
	totalGB := 0.0

	for _, tier := range g.manager.ListTiers() {
		usedGB := float64(tier.Used) / 1024 / 1024 / 1024
		capacityGB := float64(tier.Capacity) / 1024 / 1024 / 1024

		var costPerGB float64
		switch tier.Type {
		case TierTypeSSD:
			costPerGB = g.costConfig.SSDCostPerGBMonth
		case TierTypeHDD:
			costPerGB = g.costConfig.HDDCostPerGBMonth
		case TierTypeCloud:
			costPerGB = g.costConfig.CloudCostPerGBMonth
		}

		monthlyCost := usedGB * costPerGB
		totalMonthlyCost += monthlyCost
		totalGB += usedGB

		report.TierCosts[tier.Type] = &TierCostInfo{
			TierType:       tier.Type,
			CostPerGBMonth: costPerGB,
			MonthlyCost:    monthlyCost,
			YearlyCost:     monthlyCost * 12,
			UsedGB:         usedGB,
			CapacityGB:     capacityGB,
			Utilization:    float64(tier.Used) / float64(tier.Capacity+1) * 100,
		}
	}

	report.TotalMonthlyCost = totalMonthlyCost
	report.TotalYearlyCost = totalMonthlyCost * 12
	if totalGB > 0 {
		report.CostPerGB = totalMonthlyCost / totalGB
	}

	// 计算成本节省
	report.CostSavings = g.calculateCostSavings(report)

	// 生成成本预测
	report.CostForecast = g.generateCostForecast()

	// ROI 分析
	report.ROI = g.calculateROI(report)

	// 优化机会
	report.OptimizationOpportunities = g.identifyCostOptimizations(report)

	return report
}

// generateCapacityForecast 生成容量趋势预测
func (g *EfficiencyReportGenerator) generateCapacityForecast(days int) *CapacityForecastReport {
	report := &CapacityForecastReport{
		ForecastDays:   days,
		ByTier:         make(map[TierType]*CapacityPredict),
		ForecastPoints: make([]CapacityForecastPoint, 0),
		Warnings:       make([]CapacityWarning, 0),
	}

	// 获取历史数据计算增长率
	g.historyMu.RLock()
	dailyStats := g.dailyStats
	g.historyMu.RUnlock()

	// 计算总体增长率
	growthRate := g.calculateGrowthRate(dailyStats)

	// 按存储层预测
	for _, tier := range g.manager.ListTiers() {
		predict := &CapacityPredict{
			TierType:        tier.Type,
			CurrentUsed:     tier.Used,
			CurrentCapacity: tier.Capacity,
			GrowthRateDaily: growthRate,
		}

		// 预测未来使用量
		if growthRate > 0 {
			predict.PredictedUsed7D = int64(float64(tier.Used) + growthRate*7*1024*1024*1024)
			predict.PredictedUsed30D = int64(float64(tier.Used) + growthRate*30*1024*1024*1024)
			predict.PredictedUsed90D = int64(float64(tier.Used) + growthRate*90*1024*1024*1024)

			remainingCapacity := float64(tier.Capacity - tier.Used)
			predict.DaysToFull = int(remainingCapacity / (growthRate * 1024 * 1024 * 1024))
		}

		report.ByTier[tier.Type] = predict

		// 生成预测数据点
		for i := 0; i <= days; i += 7 {
			predictedUsed := tier.Used + int64(growthRate*float64(i)*1024*1024*1024)
			report.ForecastPoints = append(report.ForecastPoints, CapacityForecastPoint{
				Date:          time.Now().AddDate(0, 0, i),
				TierType:      tier.Type,
				PredictedUsed: predictedUsed,
				UpperBound:    int64(float64(predictedUsed) * 1.1),
				LowerBound:    int64(float64(predictedUsed) * 0.9),
				Confidence:    0.85 - float64(i)*0.001, // 置信度随时间降低
			})
		}

		// 检查是否需要警告
		if predict.DaysToFull > 0 && predict.DaysToFull < 30 {
			report.Warnings = append(report.Warnings, CapacityWarning{
				TierType:      tier.Type,
				WarningType:   "approaching_full",
				Severity:      "warning",
				Message:       fmt.Sprintf("存储层 %s 预计 %d 天后填满", tier.Name, predict.DaysToFull),
				DaysRemaining: predict.DaysToFull,
				Suggestion:    "考虑扩容或迁移冷数据到归档层",
			})
		}
	}

	// 模型信息
	report.ModelInfo = &ForecastModelInfo{
		ModelType:   "linear",
		Accuracy:    0.85,
		DataPoints:  len(dailyStats),
		LastTrained: time.Now(),
		Features:    []string{"daily_growth", "capacity", "usage_pattern"},
	}

	return report
}

// calculateHealthScore 计算健康评分
func (g *EfficiencyReportGenerator) calculateHealthScore() *TieringHealthScore {
	score := &TieringHealthScore{
		ScoreBreakdown: make([]ScoreItem, 0),
	}

	// 数据分布评分（热数据在高优先级层）
	distributionScore := g.calculateDistributionScore()
	score.DistributionScore = distributionScore
	score.ScoreBreakdown = append(score.ScoreBreakdown, ScoreItem{
		Name:        "数据分布",
		Score:       distributionScore,
		MaxScore:    100,
		Weight:      0.3,
		Description: "热数据在高优先级层的比例",
	})

	// 迁移效率评分
	efficiencyScore := g.calculateEfficiencyScore()
	score.EfficiencyScore = efficiencyScore
	score.ScoreBreakdown = append(score.ScoreBreakdown, ScoreItem{
		Name:        "迁移效率",
		Score:       efficiencyScore,
		MaxScore:    100,
		Weight:      0.25,
		Description: "迁移成功率和吞吐量",
	})

	// 成本优化评分
	costScore := g.calculateCostOptimizationScore()
	score.CostScore = costScore
	score.ScoreBreakdown = append(score.ScoreBreakdown, ScoreItem{
		Name:        "成本优化",
		Score:       costScore,
		MaxScore:    100,
		Weight:      0.2,
		Description: "存储成本优化程度",
	})

	// 容量健康评分
	capacityScore := g.calculateCapacityHealthScore()
	score.CapacityScore = capacityScore
	score.ScoreBreakdown = append(score.ScoreBreakdown, ScoreItem{
		Name:        "容量健康",
		Score:       capacityScore,
		MaxScore:    100,
		Weight:      0.15,
		Description: "存储层容量使用健康状况",
	})

	// 策略配置评分
	policyScore := g.calculatePolicyScore()
	score.PolicyScore = policyScore
	score.ScoreBreakdown = append(score.ScoreBreakdown, ScoreItem{
		Name:        "策略配置",
		Score:       policyScore,
		MaxScore:    100,
		Weight:      0.1,
		Description: "分层策略配置合理性",
	})

	// 计算总分
	score.OverallScore = distributionScore*0.3 + efficiencyScore*0.25 + costScore*0.2 + capacityScore*0.15 + policyScore*0.1

	// 计算等级
	switch {
	case score.OverallScore >= 90:
		score.Grade = "A"
	case score.OverallScore >= 80:
		score.Grade = "B"
	case score.OverallScore >= 70:
		score.Grade = "C"
	case score.OverallScore >= 60:
		score.Grade = "D"
	default:
		score.Grade = "F"
	}

	return score
}

// generateRecommendations 生成优化建议
func (g *EfficiencyReportGenerator) generateRecommendations() []TieringRecommendation {
	recommendations := make([]TieringRecommendation, 0)

	// 检查数据分布
	accessStats := g.manager.GetAccessStats()
	if accessStats != nil && accessStats.HotFiles > 0 {
		// 检查是否有热数据在低优先级层
		hddTier, _ := g.manager.GetTier(TierTypeHDD)
		if hddTier != nil {
			hddStats, err := g.manager.GetTierStats(TierTypeHDD)
			if err == nil && hddStats.HotFiles > 0 {
				recommendations = append(recommendations, TieringRecommendation{
					Type:        "migrate",
					Priority:    1,
					Title:       "提升热数据到SSD",
					Description: fmt.Sprintf("发现 %d 个热数据文件在HDD层，建议迁移到SSD提升访问性能", hddStats.HotFiles),
					Impact:      "高",
					Effort:      "low",
					Details: map[string]interface{}{
						"source_tier": "hdd",
						"target_tier": "ssd",
						"files_count": hddStats.HotFiles,
					},
				})
			}
		}
	}

	// 检查冷数据
	ssdTier, _ := g.manager.GetTier(TierTypeSSD)
	if ssdTier != nil {
		ssdStats, err := g.manager.GetTierStats(TierTypeSSD)
		if err == nil && ssdStats.ColdFiles > 0 {
			recommendations = append(recommendations, TieringRecommendation{
				Type:        "archive",
				Priority:    2,
				Title:       "归档冷数据释放SSD空间",
				Description: fmt.Sprintf("发现 %d 个冷数据文件占用SSD空间，建议迁移到HDD或云存储", ssdStats.ColdFiles),
				Impact:      "中",
				Effort:      "low",
				Details: map[string]interface{}{
					"source_tier":       "ssd",
					"files_count":       ssdStats.ColdFiles,
					"potential_savings": float64(ssdStats.ColdBytes) / 1024 / 1024 / 1024 * g.costConfig.SSDCostPerGBMonth,
				},
			})
		}
	}

	// 检查容量预警
	for _, tier := range g.manager.ListTiers() {
		usagePercent := float64(tier.Used) / float64(tier.Capacity+1) * 100
		if usagePercent > 80 {
			recommendations = append(recommendations, TieringRecommendation{
				Type:        "resize",
				Priority:    1,
				Title:       fmt.Sprintf("扩展 %s 存储层容量", tier.Name),
				Description: fmt.Sprintf("%s 使用率达到 %.1f%%，建议扩容或清理数据", tier.Name, usagePercent),
				Impact:      "高",
				Effort:      "medium",
				Details: map[string]interface{}{
					"tier_type":     tier.Type,
					"usage_percent": usagePercent,
				},
			})
		}
	}

	// 检查策略配置
	policies := g.manager.ListPolicies()
	if len(policies) == 0 {
		recommendations = append(recommendations, TieringRecommendation{
			Type:        "policy_change",
			Priority:    3,
			Title:       "创建自动分层策略",
			Description: "未配置自动分层策略，建议创建策略实现自动化数据迁移",
			Impact:      "中",
			Effort:      "low",
			Details: map[string]interface{}{
				"suggestion": "创建基于访问频率的自动分层策略",
			},
		})
	}

	return recommendations
}

// 辅助计算函数

func (g *EfficiencyReportGenerator) calculateTierOptimizationScore(tierType TierType, stats *TierStats) float64 {
	if stats == nil || stats.TotalBytes == 0 {
		return 0
	}

	// 高优先级层应该有更多热数据
	hotRatio := float64(stats.HotBytes) / float64(stats.TotalBytes)

	switch tierType {
	case TierTypeSSD:
		// SSD 应该有高热数据比例
		return math.Min(hotRatio*150, 100) // 放大热数据占比
	case TierTypeHDD:
		// HDD 应该主要是温数据
		warmRatio := float64(stats.WarmBytes) / float64(stats.TotalBytes)
		return math.Min(warmRatio*120, 100)
	case TierTypeCloud:
		// 云存储应该是冷数据
		coldRatio := float64(stats.ColdBytes) / float64(stats.TotalBytes)
		return math.Min(coldRatio*120, 100)
	}

	return 50
}

func (g *EfficiencyReportGenerator) generateChartData(report *DataDistributionReport) *DistributionChartData {
	data := &DistributionChartData{
		PieChart:   make([]PieChartItem, 0),
		BarChart:   make([]BarChartItem, 0),
		TimeSeries: make([]TimeSeriesPoint, 0),
	}

	// 饼图数据
	data.PieChart = append(data.PieChart,
		PieChartItem{Label: "热数据", Value: float64(report.HotData.Files), Color: "#ff6b6b"},
		PieChartItem{Label: "温数据", Value: float64(report.WarmData.Files), Color: "#ffd93d"},
		PieChartItem{Label: "冷数据", Value: float64(report.ColdData.Files), Color: "#6bcb77"},
	)

	// 柱状图数据
	for tierType, dist := range report.ByTier {
		data.BarChart = append(data.BarChart, BarChartItem{
			Label: string(tierType),
			Values: map[string]int64{
				"hot":  dist.HotBytes,
				"warm": dist.WarmBytes,
				"cold": dist.ColdBytes,
			},
		})
	}

	return data
}

func (g *EfficiencyReportGenerator) analyzeAccessPatterns(stats *AccessStats) []AccessPattern {
	patterns := make([]AccessPattern, 0)

	// 简化分析：根据热/温/冷数据比例推断访问模式
	total := stats.HotFiles + stats.WarmFiles + stats.ColdFiles
	if total == 0 {
		return patterns
	}

	hotRatio := float64(stats.HotFiles) / float64(total)
	warmRatio := float64(stats.WarmFiles) / float64(total)

	// 分析访问模式
	if hotRatio > 0.5 {
		patterns = append(patterns, AccessPattern{
			PatternType: "sequential",
			Percentage:  hotRatio * 100,
			Description: "大量热数据表示顺序访问模式，适合预取和缓存优化",
		})
	}

	if warmRatio > 0.3 {
		patterns = append(patterns, AccessPattern{
			PatternType: "random",
			Percentage:  warmRatio * 100,
			Description: "温数据表示随机访问模式，需要良好的分层策略",
		})
	}

	patterns = append(patterns, AccessPattern{
		PatternType: "mixed",
		Percentage:  (1 - hotRatio - warmRatio) * 100,
		Description: "混合访问模式，需要综合优化策略",
	})

	return patterns
}

func (g *EfficiencyReportGenerator) calculateGrowthRate(stats []DailyStats) float64 {
	if len(stats) < 2 {
		return 0.1 // 默认每天0.1GB增长
	}

	// 计算平均日增长率
	var totalGrowth float64
	for i := 1; i < len(stats); i++ {
		growth := float64(stats[i].TotalBytes-stats[i-1].TotalBytes) / 1024 / 1024 / 1024
		totalGrowth += growth
	}

	return totalGrowth / float64(len(stats)-1)
}

func (g *EfficiencyReportGenerator) calculateMigrationEfficiencyScore(report *MigrationEfficiencyReport) float64 {
	if report.TotalMigrations == 0 {
		return 50 // 没有迁移任务，返回中等分数
	}

	// 基于成功率和吞吐量计算
	successScore := report.SuccessRate

	// 吞吐量评分（假设 10MB/s 是良好）
	throughputScore := math.Min(report.AvgThroughputMBps/10*100, 100)

	return (successScore*0.7 + throughputScore*0.3)
}

func (g *EfficiencyReportGenerator) calculateCostSavings(report *CostAnalysisReport) *CostSavingsInfo {
	savings := &CostSavingsInfo{
		SavingsByTier: make(map[TierType]float64),
		SavingsTrend:  make([]SavingsTrendPoint, 0),
	}

	// 假设没有分层的情况：所有数据都在最贵的存储层
	var costWithoutTiering float64
	for _, tier := range g.manager.ListTiers() {
		usedGB := float64(tier.Used) / 1024 / 1024 / 1024
		costWithoutTiering += usedGB * g.costConfig.SSDCostPerGBMonth
	}

	savings.MonthlySavings = costWithoutTiering - report.TotalMonthlyCost
	savings.YearlySavings = savings.MonthlySavings * 12

	if costWithoutTiering > 0 {
		savings.SavingsPercent = savings.MonthlySavings / costWithoutTiering * 100
	}

	// 潜在节省（进一步优化）
	// 检查是否还有优化空间
	potentialSavings := 0.0
	for _, tier := range g.manager.ListTiers() {
		stats, _ := g.manager.GetTierStats(tier.Type)
		if stats == nil {
			continue
		}

		// SSD上的冷数据如果移到HDD可以节省
		if tier.Type == TierTypeSSD && stats.ColdBytes > 0 {
			coldGB := float64(stats.ColdBytes) / 1024 / 1024 / 1024
			potentialSavings += coldGB * (g.costConfig.SSDCostPerGBMonth - g.costConfig.HDDCostPerGBMonth)
		}
	}

	savings.PotentialMonthlySavings = potentialSavings
	savings.PotentialYearlySavings = potentialSavings * 12

	return savings
}

func (g *EfficiencyReportGenerator) generateCostForecast() []CostForecastPoint {
	forecast := make([]CostForecastPoint, 0)

	// 计算当前成本（直接计算，避免递归）
	currentCost := 0.0
	for _, tier := range g.manager.ListTiers() {
		usedGB := float64(tier.Used) / 1024 / 1024 / 1024
		var costPerGB float64
		switch tier.Type {
		case TierTypeSSD:
			costPerGB = g.costConfig.SSDCostPerGBMonth
		case TierTypeHDD:
			costPerGB = g.costConfig.HDDCostPerGBMonth
		case TierTypeCloud:
			costPerGB = g.costConfig.CloudCostPerGBMonth
		}
		currentCost += usedGB * costPerGB
	}

	// 简单的线性预测
	growthRate := 0.1 // 假设每月10%增长

	for i := 0; i < 12; i++ {
		projectedCost := currentCost * math.Pow(1+growthRate, float64(i))
		forecast = append(forecast, CostForecastPoint{
			Month:          time.Now().AddDate(0, i, 0),
			ProjectedCost:  projectedCost,
			ProjectedUsage: projectedCost / (g.costConfig.SSDCostPerGBMonth + g.costConfig.HDDCostPerGBMonth) / 2,
			Confidence:     math.Max(0.9-float64(i)*0.05, 0.5),
		})
	}

	return forecast
}

func (g *EfficiencyReportGenerator) calculateROI(report *CostAnalysisReport) *ROIAnalysis {
	roi := &ROIAnalysis{
		InitialInvestment: 1000.0, // 假设初始投资
		MonthlyOpCost:     g.costConfig.MonthlyOpCost,
	}

	roi.MonthlyBenefit = report.CostSavings.MonthlySavings
	roi.YearlyBenefit = report.CostSavings.YearlySavings

	// 计算ROI
	netBenefit := roi.YearlyBenefit - roi.MonthlyOpCost*12
	if roi.InitialInvestment > 0 {
		roi.ROI = netBenefit / roi.InitialInvestment * 100
	}

	// 计算回收期
	if roi.MonthlyBenefit > 0 {
		roi.PaybackPeriod = roi.InitialInvestment / (roi.MonthlyBenefit - roi.MonthlyOpCost)
	}

	// 计算NPV（简化）
	roi.NPV = roi.YearlyBenefit*3 - roi.InitialInvestment

	return roi
}

func (g *EfficiencyReportGenerator) identifyCostOptimizations(report *CostAnalysisReport) []CostOptimizationOpportunity {
	opportunities := make([]CostOptimizationOpportunity, 0)

	// 检查SSD上的冷数据
	ssdTier, _ := g.manager.GetTier(TierTypeSSD)
	if ssdTier != nil {
		ssdStats, _ := g.manager.GetTierStats(TierTypeSSD)
		if ssdStats != nil && ssdStats.ColdBytes > 0 {
			coldGB := float64(ssdStats.ColdBytes) / 1024 / 1024 / 1024
			opportunities = append(opportunities, CostOptimizationOpportunity{
				Type:             "archive_cold_data",
				Description:      fmt.Sprintf("将SSD上的 %.1fGB 冷数据迁移到HDD或云存储", coldGB),
				PotentialSavings: coldGB * (g.costConfig.SSDCostPerGBMonth - g.costConfig.HDDCostPerGBMonth),
				Effort:           "low",
				Priority:         1,
			})
		}
	}

	// 检查未充分利用的存储层
	for _, tier := range g.manager.ListTiers() {
		utilization := float64(tier.Used) / float64(tier.Capacity+1)
		if utilization < 0.3 && tier.Enabled {
			capacityGB := float64(tier.Capacity) / 1024 / 1024 / 1024
			opportunities = append(opportunities, CostOptimizationOpportunity{
				Type:             "resize_tier",
				Description:      fmt.Sprintf("%s 利用率仅 %.1f%%，可考虑缩容", tier.Name, utilization*100),
				PotentialSavings: capacityGB * 0.5 * g.getTierCost(tier.Type),
				Effort:           "medium",
				Priority:         3,
			})
		}
	}

	return opportunities
}

func (g *EfficiencyReportGenerator) getTierCost(tierType TierType) float64 {
	switch tierType {
	case TierTypeSSD:
		return g.costConfig.SSDCostPerGBMonth
	case TierTypeHDD:
		return g.costConfig.HDDCostPerGBMonth
	case TierTypeCloud:
		return g.costConfig.CloudCostPerGBMonth
	}
	return 0
}

func (g *EfficiencyReportGenerator) calculateDistributionScore() float64 {
	// 检查数据分布合理性
	// 热数据应该在SSD，冷数据应该在HDD或云存储

	ssdStats, _ := g.manager.GetTierStats(TierTypeSSD)
	if ssdStats == nil {
		return 50
	}

	// SSD上的热数据比例
	ssdHotRatio := float64(ssdStats.HotBytes) / float64(ssdStats.TotalBytes+1)

	// HDD上的冷数据比例
	hddStats, _ := g.manager.GetTierStats(TierTypeHDD)
	hddColdRatio := 0.5
	if hddStats != nil && hddStats.TotalBytes > 0 {
		hddColdRatio = float64(hddStats.ColdBytes) / float64(hddStats.TotalBytes)
	}

	// 综合评分 (0-100)
	return ssdHotRatio*60 + hddColdRatio*40
}

func (g *EfficiencyReportGenerator) calculateEfficiencyScore() float64 {
	metrics := g.metrics.GetMigrationMetrics()
	if metrics == nil || metrics.TotalTasks == 0 {
		return 50
	}

	successRate := float64(metrics.CompletedTasks) / float64(metrics.TotalTasks) * 100
	return successRate
}

func (g *EfficiencyReportGenerator) calculateCostOptimizationScore() float64 {
	// 基于成本节省比例评分
	costReport := g.generateCostAnalysis()
	if costReport.CostSavings == nil {
		return 50
	}

	return math.Min(costReport.CostSavings.SavingsPercent*2, 100)
}

func (g *EfficiencyReportGenerator) calculateCapacityHealthScore() float64 {
	// 基于存储层利用率评分
	var totalScore float64
	count := 0

	for _, tier := range g.manager.ListTiers() {
		if !tier.Enabled {
			continue
		}

		usagePercent := float64(tier.Used) / float64(tier.Capacity+1) * 100

		// 理想利用率是 50-80%
		var tierScore float64
		switch {
		case usagePercent >= 50 && usagePercent <= 80:
			tierScore = 100
		case usagePercent < 50:
			tierScore = usagePercent * 2 // 低利用率扣分
		case usagePercent > 80:
			tierScore = 100 - (usagePercent-80)*5 // 高利用率扣分更多
		}

		totalScore += tierScore
		count++
	}

	if count == 0 {
		return 50
	}

	return totalScore / float64(count)
}

func (g *EfficiencyReportGenerator) calculatePolicyScore() float64 {
	policies := g.manager.ListPolicies()

	if len(policies) == 0 {
		return 30 // 没有策略，低分
	}

	enabledCount := 0
	for _, p := range policies {
		if p.Enabled {
			enabledCount++
		}
	}

	// 基于启用策略比例评分
	enabledRatio := float64(enabledCount) / float64(len(policies))

	// 有策略且启用率高的分数更高
	return 50 + enabledRatio*50
}

// RecordDailyStats 记录每日统计
func (g *EfficiencyReportGenerator) RecordDailyStats() {
	g.historyMu.Lock()
	defer g.historyMu.Unlock()

	stats := DailyStats{
		Date: time.Now(),
	}

	accessStats := g.manager.GetAccessStats()
	if accessStats != nil {
		stats.HotBytes = accessStats.TotalReadBytes
		stats.WarmBytes = accessStats.TotalWriteBytes
	}

	metrics := g.metrics.GetMigrationMetrics()
	if metrics != nil {
		stats.MigrationsCount = metrics.TotalTasks
		stats.BytesMigrated = metrics.TotalBytesMigrated
	}

	g.dailyStats = append(g.dailyStats, stats)

	// 保留最近365天的数据
	if len(g.dailyStats) > 365 {
		g.dailyStats = g.dailyStats[len(g.dailyStats)-365:]
	}
}
