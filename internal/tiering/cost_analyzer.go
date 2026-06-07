// Package tiering 分层存储成本分析器 - 计算各存储层成本，预估分层后节省费用，以及 ROI 分析。
package tiering

import (
	"fmt"
	"time"
)

// ==================== 存储层成本配置 ====================

// TierCostConfig 存储层成本配置（每月每TB的价格）.
type TierCostConfig struct {
	TierType       TierType `json:"tierType"`
	CostPerTBMonth float64  `json:"costPerTbMonth"` // 月费（美元/TB）
	ReadIOPS       int      `json:"readIops"`       // 读IOPS
	WriteIOPS      int      `json:"writeIops"`      // 写IOPS
	ReadLatencyMs  float64  `json:"readLatencyMs"`  // 读延迟(ms)
	WriteLatencyMs float64  `json:"writeLatencyMs"` // 写延迟(ms)
	BandwidthMBps  float64  `json:"bandwidthMbps"`  // 带宽(MB/s)
}

// DefaultTierCosts 默认存储层成本配置.
var DefaultTierCosts = map[TierType]*TierCostConfig{
	TierTypeSSD: {
		TierType:       TierTypeSSD,
		CostPerTBMonth: 100.0, // SSD: ~$100/TB/月
		ReadIOPS:       100000,
		WriteIOPS:      80000,
		ReadLatencyMs:  0.1,
		WriteLatencyMs: 0.2,
		BandwidthMBps:  500,
	},
	TierTypeHDD: {
		TierType:       TierTypeHDD,
		CostPerTBMonth: 20.0, // HDD: ~$20/TB/月
		ReadIOPS:       200,
		WriteIOPS:      180,
		ReadLatencyMs:  10.0,
		WriteLatencyMs: 12.0,
		BandwidthMBps:  150,
	},
	TierTypeCloud: {
		TierType:       TierTypeCloud,
		CostPerTBMonth: 5.0, // 冷存储云: ~$5/TB/月
		ReadIOPS:       100,
		WriteIOPS:      80,
		ReadLatencyMs:  50.0,
		WriteLatencyMs: 100.0,
		BandwidthMBps:  50,
	},
}

// ==================== 分层效果报告 ====================

// TieringEffectReport 分层效果报告.
type TieringEffectReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Period      string    `json:"period"` // daily/weekly/monthly

	// 当前状态
	CurrentState *TieringStateSnapshot `json:"currentState"`

	// 分层前（模拟）vs 分层后
	BeforeTiering *PerformanceSnapshot `json:"beforeTiering"`
	AfterTiering  *PerformanceSnapshot `json:"afterTiering"`

	// 改进指标
	Improvements *PerformanceImprovement `json:"improvements"`

	// 成本对比
	CostComparison *CostComparison `json:"costComparison"`

	// 迁移统计
	MigrationStats *MigrationSummary `json:"migrationStats"`

	// 建议
	Recommendations []string `json:"recommendations"`
}

// TieringStateSnapshot 分层状态快照.
type TieringStateSnapshot struct {
	Tiers      map[TierType]*TierSnapshot `json:"tiers"`
	TotalFiles int64                      `json:"totalFiles"`
	TotalBytes int64                      `json:"totalBytes"`
}

// TierSnapshot 单个存储层快照.
type TierSnapshot struct {
	TierType    TierType `json:"tierType"`
	TotalFiles  int64    `json:"totalFiles"`
	TotalBytes  int64    `json:"totalBytes"`
	HotFiles    int64    `json:"hotFiles"`
	WarmFiles   int64    `json:"warmFiles"`
	ColdFiles   int64    `json:"coldFiles"`
	UsedPercent float64  `json:"usedPercent"`
}

// PerformanceSnapshot 性能快照.
type PerformanceSnapshot struct {
	AvgReadLatencyMs  float64 `json:"avgReadLatencyMs"`  // 平均读延迟
	AvgWriteLatencyMs float64 `json:"avgWriteLatencyMs"` // 平均写延迟
	TotalIOPS         int     `json:"totalIops"`         // 总IOPS
	BandwidthMBps     float64 `json:"bandwidthMbps"`     // 带宽
	SSDHitRate        float64 `json:"ssdHitRate"`        // SSD命中率
}

// PerformanceImprovement 性能改进指标.
type PerformanceImprovement struct {
	LatencyReduction float64 `json:"latencyReduction"` // 延迟降低百分比
	IOPSImprovement  float64 `json:"iopsImprovement"`  // IOPS提升百分比
	BandwidthChange  float64 `json:"bandwidthChange"`  // 带宽变化百分比
	SSDHitRateChange float64 `json:"ssdHitRateChange"` // SSD命中率变化
}

// CostComparison 成本对比.
type CostComparison struct {
	BeforeMonthlyCost float64 `json:"beforeMonthlyCost"` // 分层前月成本
	AfterMonthlyCost  float64 `json:"afterMonthlyCost"`  // 分层后月成本
	MonthlySavings    float64 `json:"monthlySavings"`    // 月节省
	AnnualSavings     float64 `json:"annualSavings"`     // 年节省
	SavingsPercent    float64 `json:"savingsPercent"`    // 节省比例
}

// MigrationSummary 迁移摘要.
type MigrationSummary struct {
	TotalMigrations  int   `json:"totalMigrations"`  // 总迁移次数
	FilesMigrated    int   `json:"filesMigrated"`    // 已迁移文件数
	BytesMigrated    int64 `json:"bytesMigrated"`    // 已迁移字节数
	FailedMigrations int   `json:"failedMigrations"` // 失败迁移数
	ActiveTasks      int   `json:"activeTasks"`      // 活跃任务数
}

// ROIReport ROI（投资回报率）报告.
type ROIReport struct {
	GeneratedAt time.Time `json:"generatedAt"`

	// 投资成本（管理、监控、迁移开销）
	ManagementCostMonth float64 `json:"managementCostMonth"` // 月管理成本
	MigrationCostTotal  float64 `json:"migrationCostTotal"`  // 迁移总成本（一次性）
	MonitoringCostMonth float64 `json:"monitoringCostMonth"` // 月监控成本

	// 收益
	MonthlySavings       float64 `json:"monthlySavings"`       // 月节省
	PerformanceGainValue float64 `json:"performanceGainValue"` // 性能提升价值估算

	// ROI 指标
	TotalInvestment     float64 `json:"totalInvestment"`     // 总投入（12个月）
	TotalReturn         float64 `json:"totalReturn"`         // 总回报（12个月）
	ROIPercent          float64 `json:"roiPercent"`          // ROI百分比
	PaybackPeriodMonths float64 `json:"paybackPeriodMonths"` // 回本周期（月）
}

// ==================== CostAnalyzer ====================

// CostAnalyzer 成本分析器.
type CostAnalyzer struct {
	manager *Manager
	costs   map[TierType]*TierCostConfig
}

// NewCostAnalyzer 创建成本分析器.
func NewCostAnalyzer(manager *Manager) *CostAnalyzer {
	// 复制默认配置
	costs := make(map[TierType]*TierCostConfig)
	for k, v := range DefaultTierCosts {
		costs[k] = v
	}

	return &CostAnalyzer{
		manager: manager,
		costs:   costs,
	}
}

// SetTierCost 设置存储层成本配置.
func (a *CostAnalyzer) SetTierCost(tierType TierType, config *TierCostConfig) {
	a.costs[tierType] = config
}

// CalculateTierCost 计算指定存储层的月成本.
func (a *CostAnalyzer) CalculateTierCost(bytes int64, tierType TierType) float64 {
	config, ok := a.costs[tierType]
	if !ok {
		return 0
	}

	tbSize := float64(bytes) / (1024 * 1024 * 1024 * 1024) // 字节转TB
	return tbSize * config.CostPerTBMonth
}

// EstimateTierCostDifference 预估将数据从源层迁移到目标层的成本差异（月费变化，负数表示节省）.
func (a *CostAnalyzer) EstimateTierCostDifference(bytes int64, sourceTier, targetTier TierType) float64 {
	sourceCost := a.CalculateTierCost(bytes, sourceTier)
	targetCost := a.CalculateTierCost(bytes, targetTier)
	return sourceCost - targetCost // 正数表示节省
}

// EstimateMigrationCost 预估迁移成本（包含时间和带宽信息）.
func (a *CostAnalyzer) EstimateMigrationCost(totalBytes int64, estimatedTime time.Duration) *MigrationCost {
	cost := &MigrationCost{}

	// 迁移时间和带宽
	cost.TransferTimeHours = estimatedTime.Hours()
	if estimatedTime.Seconds() > 0 {
		cost.TransferBandwidth = float64(totalBytes) / (1024 * 1024) / estimatedTime.Seconds()
	}

	// 当前存储层成本（假设按当前分布）
	currentCost := a.estimateCurrentMonthlyCost()
	cost.CurrentMonthlyCost = currentCost

	// 预估分层后的成本（简化：假设总数据中按比例分布）
	projectedCost := a.estimateProjectedMonthlyCost()
	cost.ProjectedMonthlyCost = projectedCost

	cost.MonthlySavings = currentCost - projectedCost
	cost.AnnualSavings = cost.MonthlySavings * 12

	if currentCost > 0 {
		cost.SavingsPercent = (cost.MonthlySavings / currentCost) * 100
	}

	// 性能影响估算
	cost.EstimatedIOPSChange = 15.0 // 分层后IOPS通常提升15%左右
	cost.EstimatedLatencyMs = -2.5  // 延迟通常降低

	return cost
}

// GenerateTieringEffectReport 生成分层效果报告.
func (a *CostAnalyzer) GenerateTieringEffectReport() *TieringEffectReport {
	report := &TieringEffectReport{
		GeneratedAt: time.Now(),
		Period:      "monthly",
	}

	// 当前状态
	report.CurrentState = a.getCurrentState()

	// 性能对比（模拟分层前）
	report.BeforeTiering = a.estimateBeforeTieringPerformance()
	report.AfterTiering = a.estimateAfterTieringPerformance()

	// 计算改进
	report.Improvements = a.calculateImprovements(report.BeforeTiering, report.AfterTiering)

	// 成本对比
	report.CostComparison = a.generateCostComparison()

	// 迁移统计
	report.MigrationStats = a.getMigrationSummary()

	// 生成建议
	report.Recommendations = a.generateEffectRecommendations(report)

	return report
}

// GenerateROIReport 生成ROI报告.
func (a *CostAnalyzer) GenerateROIReport() *ROIReport {
	report := &ROIReport{
		GeneratedAt: time.Now(),
	}

	// 投资成本
	report.ManagementCostMonth = 5.0 // 管理开销估计
	report.MigrationCostTotal = 10.0 // 迁移一次性开销
	report.MonitoringCostMonth = 2.0 // 监控开销

	// 收益
	costComp := a.generateCostComparison()
	report.MonthlySavings = costComp.MonthlySavings
	report.PerformanceGainValue = 20.0 // 性能提升价值估算

	// ROI 计算
	totalInvestmentYear := (report.ManagementCostMonth+report.MonitoringCostMonth)*12 + report.MigrationCostTotal
	totalReturnYear := (report.MonthlySavings + report.PerformanceGainValue) * 12

	report.TotalInvestment = totalInvestmentYear
	report.TotalReturn = totalReturnYear

	if totalInvestmentYear > 0 {
		report.ROIPercent = ((totalReturnYear - totalInvestmentYear) / totalInvestmentYear) * 100
	}

	if report.MonthlySavings > 0 {
		monthlyInvestment := report.ManagementCostMonth + report.MonitoringCostMonth
		report.PaybackPeriodMonths = report.MigrationCostTotal / report.MonthlySavings
		_ = monthlyInvestment // 用于未来扩展
	}

	return report
}

// ==================== 内部辅助方法 ====================

// estimateCurrentMonthlyCost 估算当前月成本.
func (a *CostAnalyzer) estimateCurrentMonthlyCost() float64 {
	var totalCost float64

	for _, tierType := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		records := a.manager.tracker.GetRecordsByTier(tierType)
		var totalBytes int64
		for _, r := range records {
			totalBytes += r.Size
		}
		totalCost += a.CalculateTierCost(totalBytes, tierType)
	}

	return totalCost
}

// estimateProjectedMonthlyCost 估算分层后的月成本.
func (a *CostAnalyzer) estimateProjectedMonthlyCost() float64 {
	// 基于当前数据分布，考虑优化后的理想分布
	var totalCost float64

	for _, tierType := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		records := a.manager.tracker.GetRecordsByTier(tierType)
		var totalBytes int64
		for _, r := range records {
			totalBytes += r.Size
		}
		totalCost += a.CalculateTierCost(totalBytes, tierType)
	}

	// 估计优化后可节省15-30%
	return totalCost * 0.75
}

// getCurrentState 获取当前分层状态.
func (a *CostAnalyzer) getCurrentState() *TieringStateSnapshot {
	state := &TieringStateSnapshot{
		Tiers: make(map[TierType]*TierSnapshot),
	}

	for _, tierType := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		records := a.manager.tracker.GetRecordsByTier(tierType)

		snap := &TierSnapshot{
			TierType: tierType,
		}

		for _, r := range records {
			snap.TotalFiles++
			snap.TotalBytes += r.Size

			switch r.Frequency {
			case AccessFrequencyHot:
				snap.HotFiles++
			case AccessFrequencyWarm:
				snap.WarmFiles++
			case AccessFrequencyCold:
				snap.ColdFiles++
			}
		}

		state.Tiers[tierType] = snap
		state.TotalFiles += snap.TotalFiles
		state.TotalBytes += snap.TotalBytes
	}

	return state
}

// estimateBeforeTieringPerformance 估算分层前的性能（假设所有数据在HDD）.
func (a *CostAnalyzer) estimateBeforeTieringPerformance() *PerformanceSnapshot {
	hddConfig := a.costs[TierTypeHDD]
	return &PerformanceSnapshot{
		AvgReadLatencyMs:  hddConfig.ReadLatencyMs,
		AvgWriteLatencyMs: hddConfig.WriteLatencyMs,
		TotalIOPS:         hddConfig.ReadIOPS + hddConfig.WriteIOPS,
		BandwidthMBps:     hddConfig.BandwidthMBps,
		SSDHitRate:        0,
	}
}

// estimateAfterTieringPerformance 估算分层后的性能.
func (a *CostAnalyzer) estimateAfterTieringPerformance() *PerformanceSnapshot {
	ssdConfig := a.costs[TierTypeSSD]
	hddConfig := a.costs[TierTypeHDD]

	// 热数据在SSD（约30%），冷数据在HDD
	hotRatio := 0.3
	coldRatio := 0.7

	return &PerformanceSnapshot{
		AvgReadLatencyMs:  ssdConfig.ReadLatencyMs*hotRatio + hddConfig.ReadLatencyMs*coldRatio,
		AvgWriteLatencyMs: ssdConfig.WriteLatencyMs*hotRatio + hddConfig.WriteLatencyMs*coldRatio,
		TotalIOPS:         int(float64(ssdConfig.ReadIOPS+ssdConfig.WriteIOPS)*hotRatio + float64(hddConfig.ReadIOPS+hddConfig.WriteIOPS)*coldRatio),
		BandwidthMBps:     ssdConfig.BandwidthMBps*hotRatio + hddConfig.BandwidthMBps*coldRatio,
		SSDHitRate:        hotRatio * 100,
	}
}

// calculateImprovements 计算性能改进.
func (a *CostAnalyzer) calculateImprovements(before, after *PerformanceSnapshot) *PerformanceImprovement {
	imp := &PerformanceImprovement{}

	if before.AvgReadLatencyMs > 0 {
		imp.LatencyReduction = ((before.AvgReadLatencyMs - after.AvgReadLatencyMs) / before.AvgReadLatencyMs) * 100
	}
	if before.TotalIOPS > 0 {
		imp.IOPSImprovement = ((float64(after.TotalIOPS) - float64(before.TotalIOPS)) / float64(before.TotalIOPS)) * 100
	}
	if before.BandwidthMBps > 0 {
		imp.BandwidthChange = ((after.BandwidthMBps - before.BandwidthMBps) / before.BandwidthMBps) * 100
	}
	imp.SSDHitRateChange = after.SSDHitRate - before.SSDHitRate

	return imp
}

// generateCostComparison 生成成本对比.
func (a *CostAnalyzer) generateCostComparison() *CostComparison {
	before := a.estimateCurrentMonthlyCost()
	after := a.estimateProjectedMonthlyCost()

	comp := &CostComparison{
		BeforeMonthlyCost: before,
		AfterMonthlyCost:  after,
		MonthlySavings:    before - after,
		AnnualSavings:     (before - after) * 12,
	}

	if before > 0 {
		comp.SavingsPercent = (comp.MonthlySavings / before) * 100
	}

	return comp
}

// getMigrationSummary 获取迁移摘要.
func (a *CostAnalyzer) getMigrationSummary() *MigrationSummary {
	summary := &MigrationSummary{}

	tasks := a.manager.ListTasks(1000)
	summary.TotalMigrations = len(tasks)

	for _, task := range tasks {
		summary.FilesMigrated += int(task.ProcessedFiles)
		summary.BytesMigrated += task.ProcessedBytes
		summary.FailedMigrations += int(task.FailedFiles)

		switch task.Status {
		case MigrateStatusRunning, MigrateStatusPending:
			summary.ActiveTasks++
		}
	}

	return summary
}

// generateEffectRecommendations 生成效果报告建议.
func (a *CostAnalyzer) generateEffectRecommendations(report *TieringEffectReport) []string {
	var recs []string

	if report.Improvements.LatencyReduction < 5 {
		recs = append(recs, "当前分层策略对延迟改善有限，建议增加SSD缓存层容量或调整分层阈值")
	}

	if report.Improvements.SSDHitRateChange < 20 {
		recs = append(recs, "SSD命中率偏低，建议将更频繁访问的文件提升到SSD层")
	}

	if report.CostComparison.SavingsPercent < 10 {
		recs = append(recs, "成本节省率较低，建议审查是否有更多冷数据可下沉到低成本存储层")
	}

	if report.MigrationStats.FailedMigrations > 0 {
		recs = append(recs, fmt.Sprintf("存在 %d 次失败迁移，建议检查存储层连接状态和权限配置", report.MigrationStats.FailedMigrations))
	}

	if report.CurrentState != nil {
		if ssdSnap, ok := report.CurrentState.Tiers[TierTypeSSD]; ok {
			if ssdSnap.UsedPercent > 90 {
				recs = append(recs, "SSD层使用率超过90%，建议及时扩容或加速冷数据下沉")
			}
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "当前分层策略运行良好，无需调整")
	}

	return recs
}
