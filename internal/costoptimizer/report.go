package costoptimizer

import (
	"fmt"
	"time"
)

// ReportGenerator 成本报告生成器
type ReportGenerator struct {
	optimizer *CostOptimizer
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(co *CostOptimizer) *ReportGenerator {
	return &ReportGenerator{optimizer: co}
}

// ReportType 报告类型
type ReportType string

const (
	ReportMonthly ReportType = "monthly"
	ReportAnnual  ReportType = "annual"
)

// StorageCostReport 完整存储成本报告
type StorageCostReport struct {
	ReportID    string    `json:"report_id"`
	ReportType  ReportType `json:"report_type"`
	GeneratedAt time.Time `json:"generated_at"`
	Period      ReportPeriodInfo `json:"period"`

	// 总览
	Overview CostOverview `json:"overview"`

	// 成本明细
	CostBreakdown CostBreakdownDetail `json:"cost_breakdown"`

	// 去重分析
	DedupAnalysis *DedupResult `json:"dedup_analysis,omitempty"`

	// 压缩分析
	CompressAnalysis *CompressResult `json:"compress_analysis,omitempty"`

	// 分层对比
	TieringAnalysis *TieringResult `json:"tiering_analysis,omitempty"`

	// 优化建议
	Optimizations []OptimizationSummary `json:"optimizations"`

	// 趋势预测
	Forecast *CostForecast `json:"forecast,omitempty"`
}

// ReportPeriodInfo 报告周期信息
type ReportPeriodInfo struct {
	Type      string    `json:"type"` // monthly/annual
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// CostOverview 成本总览
type CostOverview struct {
	TotalDataTB     float64 `json:"total_data_tb"`
	CurrentCost     float64 `json:"current_cost"`
	OptimizedCost   float64 `json:"optimized_cost"`
	PotentialSaving float64 `json:"potential_saving"`
	SavingPercent   float64 `json:"saving_percent"`
	CostPerTB       float64 `json:"cost_per_tb"`
}

// CostBreakdownDetail 成本明细
type CostBreakdownDetail struct {
	ByTier     []TierCostDetail  `json:"by_tier"`
	ByDataType []TypeCostDetail  `json:"by_data_type"`
}

// TierCostDetail 各层成本明细
type TierCostDetail struct {
	Tier       StorageTier `json:"tier"`
	TierName   string      `json:"tier_name"`
	DataTB     float64     `json:"data_tb"`
	Cost       float64     `json:"cost"`
	Percent    float64     `json:"percent"`
}

// TypeCostDetail 各类型成本明细
type TypeCostDetail struct {
	DataType  DataType `json:"data_type"`
	DataTB    float64  `json:"data_tb"`
	Cost      float64  `json:"cost"`
	Percent   float64  `json:"percent"`
}

// OptimizationSummary 优化建议汇总
type OptimizationSummary struct {
	Type        string  `json:"type"` // dedup/compress/tiering/archive
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingGB    float64 `json:"saving_gb"`
	SavingCost  float64 `json:"saving_cost"`
	Priority    string  `json:"priority"`
}

// CostForecast 成本预测
type CostForecast struct {
	MonthlyGrowthRate float64       `json:"monthly_growth_rate"` // 月增长率
	Projections       []CostPoint   `json:"projections"`         // 预测点
}

// CostPoint 成本预测点
type CostPoint struct {
	Date      time.Time `json:"date"`
	Cost      float64   `json:"cost"`
	DataTB    float64   `json:"data_tb"`
}

// GenerateMonthlyReport 生成月度报告
func (rg *ReportGenerator) GenerateMonthlyReport() *StorageCostReport {
	return rg.generateReport(ReportMonthly)
}

// GenerateAnnualReport 生成年度报告
func (rg *ReportGenerator) GenerateAnnualReport() *StorageCostReport {
	return rg.generateReport(ReportAnnual)
}

// generateReport 生成报告
func (rg *ReportGenerator) generateReport(reportType ReportType) *StorageCostReport {
	now := time.Now()
	report := &StorageCostReport{
		ReportID:    fmt.Sprintf("RPT-%s-%s", reportType, now.Format("20060102-150405")),
		ReportType:  reportType,
		GeneratedAt: now,
		Period: ReportPeriodInfo{
			Type:      string(reportType),
			EndDate:   now,
		},
	}

	if reportType == ReportMonthly {
		report.Period.StartDate = now.AddDate(0, -1, 0)
	} else {
		report.Period.StartDate = now.AddDate(-1, 0, 0)
	}

	allocs := rg.optimizer.allocations
	profiles := rg.optimizer.profiles

	// 计算总览
	var totalBytes int64
	var totalCost float64
	costByTier := make(map[StorageTier]float64)
	costByType := make(map[DataType]float64)

	for _, alloc := range allocs {
		totalBytes += alloc.UsedBytes
		if profile, ok := profiles[alloc.Tier]; ok {
			cost := bytesToTB(alloc.UsedBytes) * profile.CostPerTBMonth
			totalCost += cost
			costByTier[alloc.Tier] += cost
		}
	}

	// 按数据类型统计
	typeBytesMap := make(map[DataType]int64)
	for _, alloc := range allocs {
		typeBytesMap[alloc.DataType] += alloc.UsedBytes
	}
	for dt, bytes := range typeBytesMap {
		// 使用平均成本
		avgCostPerTB := totalCost / max(bytesToTB(totalBytes), 0.001)
		costByType[dt] = bytesToTB(bytes) * avgCostPerTB
	}

	totalTB := bytesToTB(totalBytes)

	// 去重分析
	dedupResult := rg.optimizer.dedup.EstimateDedupPotential()

	// 压缩分析
	compressResult := rg.optimizer.compress.EstimateCompressBenefit()

	// 分层对比
	tieringResult := rg.optimizer.tiering.CompareSchemes()

	// 去重后剩余数据量
	remainingBytes := totalBytes
	if dedupResult.SavingsBytes > 0 {
		remainingBytes = totalBytes - dedupResult.SavingsBytes
	}
	// 压缩后剩余数据量
	remainingAfterCompress := remainingBytes
	if compressResult.RecommendedSavings > 0 {
		compressSavingsOnRemaining := int64(float64(compressResult.RecommendedSavings) * float64(remainingBytes) / float64(max(totalBytes, 1)))
		remainingAfterCompress = remainingBytes - compressSavingsOnRemaining
	}
	// 分层节省基于去重+压缩后的数据量
	tieringSaving := 0.0
	if len(tieringResult.Schemes) > 0 && tieringResult.Schemes[0].SavingsVsCurrent > 0 {
		// 按比例缩放分层节省
		scale := float64(remainingAfterCompress) / float64(max(totalBytes, 1))
		tieringSaving = tieringResult.Schemes[0].SavingsVsCurrent * scale
	}

	// 计算优化后成本（分步应用）
	// 1. 去重节省成本
	dedupCostSaving := bytesToTB(dedupResult.SavingsBytes) * (totalCost / max(bytesToTB(totalBytes), 0.001))
	// 2. 压缩节省成本（基于剩余数据）
	compressCostSaving := bytesToTB(totalBytes-remainingAfterCompress) * (totalCost / max(bytesToTB(totalBytes), 0.001))
	// 3. 分层节省
	potentialSaving := dedupCostSaving + compressCostSaving + tieringSaving

	// 确保节省不超过当前成本
	if potentialSaving > totalCost {
		potentialSaving = totalCost * 0.8 // 最多节省80%
	}

	optimizedCost := totalCost - potentialSaving
	if optimizedCost < 0 {
		optimizedCost = 0
	}

	report.Overview = CostOverview{
		TotalDataTB:     totalTB,
		CurrentCost:     totalCost,
		OptimizedCost:   optimizedCost,
		PotentialSaving: potentialSaving,
		SavingPercent:   safePercent(potentialSaving, totalCost),
		CostPerTB:       safePercent(totalCost, totalTB),
	}

	// 成本明细
	tierDetails := make([]TierCostDetail, 0)
	for tier, cost := range costByTier {
		profile, _ := profiles[tier]
		var tierBytes int64
		for _, alloc := range allocs {
			if alloc.Tier == tier {
				tierBytes += alloc.UsedBytes
			}
		}
		tierDetails = append(tierDetails, TierCostDetail{
			Tier:     tier,
			TierName: profile.Name,
			DataTB:   bytesToTB(tierBytes),
			Cost:     cost,
			Percent:  safePercent(cost, totalCost),
		})
	}

	typeDetails := make([]TypeCostDetail, 0)
	for dt, cost := range costByType {
		typeDetails = append(typeDetails, TypeCostDetail{
			DataType: dt,
			DataTB:   bytesToTB(typeBytesMap[dt]),
			Cost:     cost,
			Percent:  safePercent(cost, totalCost),
		})
	}

	report.CostBreakdown = CostBreakdownDetail{
		ByTier:     tierDetails,
		ByDataType: typeDetails,
	}

	report.DedupAnalysis = dedupResult
	report.CompressAnalysis = compressResult
	report.TieringAnalysis = tieringResult

	// 优化建议汇总
	report.Optimizations = rg.summarizeOptimizations(dedupResult, compressResult, tieringResult)

	// 成本预测
	report.Forecast = rg.generateForecast(totalBytes, totalCost, reportType)

	return report
}

// summarizeOptimizations 汇总优化建议
func (rg *ReportGenerator) summarizeOptimizations(
	dedup *DedupResult,
	compress *CompressResult,
	tiering *TieringResult,
) []OptimizationSummary {
	var summaries []OptimizationSummary

	// 去重建议
	if dedup.SavingsBytes > 0 {
		summaries = append(summaries, OptimizationSummary{
			Type:        "dedup",
			Title:       "数据去重",
			Description: fmt.Sprintf("预计去重率 %.1f%%，可节省 %s 空间", dedup.DedupRatio*100, FormatBytes(dedup.SavingsBytes)),
			SavingGB:    float64(dedup.SavingsBytes) / (1024 * 1024 * 1024),
			SavingCost:  dedup.SavingsCost,
			Priority:    priorityBySaving(dedup.SavingsCost),
		})
	}

	// 压缩建议
	if compress.RecommendedSavings > 0 {
		algoProfile := DefaultCompressProfiles[compress.RecommendedAlgo]
		summaries = append(summaries, OptimizationSummary{
			Type:        "compress",
			Title:       fmt.Sprintf("数据压缩（%s）", algoProfile.Name),
			Description: algoProfile.Description,
			SavingGB:    float64(compress.RecommendedSavings) / (1024 * 1024 * 1024),
			SavingCost:  compress.RecommendedCostSave,
			Priority:    priorityBySaving(compress.RecommendedCostSave),
		})
	}

	// 分层建议
	if len(tiering.Schemes) > 0 && tiering.Schemes[0].SavingsVsCurrent > 0 {
		best := tiering.Schemes[0]
		summaries = append(summaries, OptimizationSummary{
			Type:        "tiering",
			Title:       fmt.Sprintf("存储分层（%s）", best.Name),
			Description: best.Description,
			SavingCost:  best.SavingsVsCurrent,
			Priority:    priorityBySaving(best.SavingsVsCurrent),
		})
	}

	return summaries
}

// generateForecast 生成成本预测
func (rg *ReportGenerator) generateForecast(totalBytes int64, totalCost float64, reportType ReportType) *CostForecast {
	config := rg.optimizer.config
	// 基于配置的增长率
	growthRate := config.GrowthAlertThresholdGB / 1000 // 简化增长率
	if growthRate <= 0 {
		growthRate = 0.05 // 默认月增长5%
	}

	forecast := &CostForecast{
		MonthlyGrowthRate: growthRate,
		Projections:       make([]CostPoint, 0),
	}

	// 生成预测点
	periods := 12
	if reportType == ReportAnnual {
		periods = 24
	}

	currentBytes := totalBytes
	currentCost := totalCost
	for i := 1; i <= periods; i++ {
		currentBytes = int64(float64(currentBytes) * (1 + growthRate))
		currentCost = currentCost * (1 + growthRate)
		forecast.Projections = append(forecast.Projections, CostPoint{
			Date:   time.Now().AddDate(0, i, 0),
			Cost:   currentCost,
			DataTB: bytesToTB(currentBytes),
		})
	}

	return forecast
}

// priorityBySaving 根据节省金额决定优先级
func priorityBySaving(saving float64) string {
	switch {
	case saving > 100:
		return "high"
	case saving > 20:
		return "medium"
	default:
		return "low"
	}
}
