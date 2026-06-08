package storageanalytics

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// CostAnalyzer 存储成本分析器.
type CostAnalyzer struct {
	config     *Config
	tierCosts  []TierCostConfig
	cloudCosts []CloudProviderCost
}

// NewCostAnalyzer 创建成本分析器.
func NewCostAnalyzer(config *Config) *CostAnalyzer {
	if config == nil {
		config = DefaultConfig()
	}
	return &CostAnalyzer{
		config:     config,
		tierCosts:  DefaultTierConfigs(),
		cloudCosts: DefaultCloudCosts(),
	}
}

// SetTierCosts 设置自定义层级成本配置.
func (ca *CostAnalyzer) SetTierCosts(costs []TierCostConfig) {
	ca.tierCosts = costs
}

// AnalyzeCosts 分析存储成本，生成成本报告.
func (ca *CostAnalyzer) AnalyzeCosts(result *CollectResult, report *StorageReport) *StorageCostReport {
	costReport := &StorageCostReport{
		GeneratedAt: time.Now(),
	}

	// 按文件类型/年龄推断存储层级分布
	tierUsage := ca.estimateTierUsage(result)

	// 计算各层级成本
	costReport.TierBreakdown = ca.calculateTierBreakdown(tierUsage, result.TotalSize)

	// 汇总总成本
	for _, bd := range costReport.TierBreakdown {
		costReport.TotalMonthlyCost += bd.MonthlyCost
		costReport.TotalYearlyCost += bd.YearlyCost
	}

	// 平均每TB成本
	totalTB := float64(result.TotalSize) / float64(1024*1024*1024*1024)
	if totalTB > 0 {
		costReport.CostPerTBAvg = costReport.TotalMonthlyCost / totalTB
	}

	// 成本预测
	costReport.Forecast = ca.GenerateForecast(result, report, costReport.TotalMonthlyCost)

	// 优化建议
	costReport.Recommendations = ca.generateCostRecommendations(result, report, costReport)

	// 云存储对比
	costReport.ComparisonWithCloud = ca.CompareCloudCosts(result.TotalSize, costReport.TotalMonthlyCost)

	return costReport
}

// estimateTierUsage 估算各存储层级使用量.
// 基于文件访问频率和年龄推断数据适合的存储层级.
func (ca *CostAnalyzer) estimateTierUsage(result *CollectResult) map[StorageTier]int64 {
	tierUsage := map[StorageTier]int64{
		TierNVMe: 0,
		TierSSD:  0,
		TierHDD:  0,
		TierCold: 0,
		TierCloud: 0,
	}

	for _, f := range result.Files {
		tier := ca.classifyFileTier(f)
		tierUsage[tier] += f.Size
	}

	return tierUsage
}

// classifyFileTier 根据文件特征推荐存储层级.
func (ca *CostAnalyzer) classifyFileTier(f FileInfo) StorageTier {
	sinceAccess := time.Since(f.AccessTime)
	sinceModified := time.Since(f.ModTime)

	// 频繁访问（7天内）→ NVMe/SSD
	if sinceAccess < 7*24*time.Hour {
		if f.Size < 100*1024*1024 { // 小文件，频繁访问 → NVMe
			return TierNVMe
		}
		return TierSSD // 大文件频繁访问 → SSD
	}

	// 偶尔访问（30天内）→ SSD
	if sinceAccess < 30*24*time.Hour {
		return TierSSD
	}

	// 很少访问（90天内）→ HDD
	if sinceAccess < 90*24*time.Hour {
		return TierHDD
	}

	// 超过90天未访问 → 冷存储
	if sinceAccess < 365*24*time.Hour {
		return TierCold
	}

	// 超过1年 → 冷存储
	if sinceModified > 365*24*time.Hour {
		return TierCold
	}

	return TierHDD
}

// calculateTierBreakdown 计算各层级成本分解.
func (ca *CostAnalyzer) calculateTierBreakdown(tierUsage map[StorageTier]int64, totalSize int64) []CostBreakdown {
	breakdown := make([]CostBreakdown, 0, len(tierUsage))

	for tier, usedBytes := range tierUsage {
		if usedBytes == 0 {
			continue
		}

		usedTB := float64(usedBytes) / float64(1024*1024*1024*1024)
		costPerTB := ca.getTierCost(tier)
		costPerGB := costPerTB / 1024
		monthlyCost := usedTB * costPerTB
		yearlyCost := monthlyCost * 12

		utilization := 0.0
		if totalSize > 0 {
			utilization = float64(usedBytes) / float64(totalSize)
		}

		breakdown = append(breakdown, CostBreakdown{
			Tier:        tier,
			TierName:    ca.getTierName(tier),
			CapacityTB:  usedTB,
			UsedTB:      usedTB,
			Utilization: round2(utilization),
			CostPerTB:   costPerTB,
			MonthlyCost: round2(monthlyCost),
			YearlyCost:  round2(yearlyCost),
			CostPerGB:   round2(costPerGB * 1000), // 转换为每GB
		})
	}

	// 按成本降序排序
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].MonthlyCost > breakdown[j].MonthlyCost
	})

	return breakdown
}

// getTierCost 获取指定层级的每TB月成本.
func (ca *CostAnalyzer) getTierCost(tier StorageTier) float64 {
	for _, tc := range ca.tierCosts {
		if tc.Tier == tier {
			return tc.CostPerTBMonth
		}
	}
	// 默认返回HDD成本
	return 80
}

// getTierName 获取层级中文名.
func (ca *CostAnalyzer) getTierName(tier StorageTier) string {
	for _, tc := range ca.tierCosts {
		if tc.Tier == tier {
			return tc.Name
		}
	}
	return string(tier)
}

// GenerateForecast 生成成本预测报告.
func (ca *CostAnalyzer) GenerateForecast(result *CollectResult, report *StorageReport, currentCost float64) *CostForecast {
	forecast := &CostForecast{
		GeneratedAt:   time.Now(),
		CurrentCost:   currentCost,
		CurrentSizeTB: float64(result.TotalSize) / float64(1024*1024*1024*1024),
	}

	// 计算月增长率
	growthRate := ca.calculateGrowthRate(report)
	forecast.GrowthRateTB = growthRate

	// 生成未来12个月预测
	forecast.Predictions = ca.predictFutureCosts(forecast.CurrentSizeTB, growthRate, currentCost, 12)

	// 瓶颈预测
	forecast.Breakpoint = ca.predictBreakpoint(forecast.CurrentSizeTB, growthRate, report)

	// 节省机会
	forecast.SavingsOpportunities = ca.identifySavingsOpportunities(result, report, currentCost)

	return forecast
}

// calculateGrowthRate 计算月增长率（TB/月）.
func (ca *CostAnalyzer) calculateGrowthRate(report *StorageReport) float64 {
	if report == nil || len(report.Trends.Monthly) < 2 {
		return 0
	}

	// 使用最近几个月的数据计算平均增长率
	monthly := report.Trends.Monthly
	if len(monthly) > 6 {
		monthly = monthly[len(monthly)-6:]
	}

	var totalGrowth int64
	for _, m := range monthly {
		totalGrowth += m.Growth
	}

	// 转换为TB/月
	if len(monthly) > 0 {
		avgGrowth := float64(totalGrowth) / float64(len(monthly))
		return avgGrowth / float64(1024*1024*1024*1024)
	}

	return 0
}

// predictFutureCosts 预测未来N个月的成本.
func (ca *CostAnalyzer) predictFutureCosts(currentSizeTB, growthRateTB, currentCost float64, months int) []CostPrediction {
	predictions := make([]CostPrediction, 0, months)

	// 使用指数平滑预测
	for i := 1; i <= months; i++ {
		predictedDate := time.Now().AddDate(0, i, 0)

		// 线性预测 + 季节性调整
		predictedSizeTB := currentSizeTB + growthRateTB*float64(i)

		// 确保不会为负
		if predictedSizeTB < 0 {
			predictedSizeTB = 0
		}

		// 成本预测（假设成本与容量成正比）
		costRatio := predictedSizeTB / currentSizeTB
		if currentSizeTB == 0 {
			costRatio = 1
		}
		predictedCost := currentCost * costRatio

		// 置信度随时间衰减
		confidence := math.Max(0.5, 1.0-float64(i)*0.05)

		// 预测方法
		method := "线性回归"
		if i > 6 {
			method = "指数平滑"
		}

		predictions = append(predictions, CostPrediction{
			PredictedDate:   predictedDate,
			PredictedSizeTB: round2(predictedSizeTB),
			PredictedCost:   round2(predictedCost),
			Confidence:      round2(confidence),
			Method:          method,
		})
	}

	return predictions
}

// predictBreakpoint 预测容量瓶颈.
func (ca *CostAnalyzer) predictBreakpoint(currentSizeTB, growthRateTB float64, report *StorageReport) *BreakpointInfo {
	if growthRateTB <= 0 {
		return nil
	}

	// 假设最大容量为当前使用的3倍（可根据实际配置调整）
	maxCapacityTB := currentSizeTB * 3
	if maxCapacityTB < 1 {
		maxCapacityTB = 1
	}

	// 计算何时达到80%使用率
	targetUsage := maxCapacityTB * 0.8
	remainingTB := targetUsage - currentSizeTB

	if remainingTB <= 0 {
		// 已经超过80%
		return &BreakpointInfo{
			EstimatedDate: time.Now(),
			DaysRemaining: 0,
			CurrentUsage:  1.0,
			WarningLevel:  "critical",
		}
	}

	// 计算需要多少个月
	monthsRemaining := remainingTB / growthRateTB
	daysRemaining := int(monthsRemaining * 30)

	estimatedDate := time.Now().AddDate(0, int(monthsRemaining), 0)

	// 确定警告级别
	currentUsage := currentSizeTB / maxCapacityTB
	warningLevel := "info"
	if currentUsage > 0.7 {
		warningLevel = "warning"
	}
	if currentUsage > 0.85 {
		warningLevel = "critical"
	}

	return &BreakpointInfo{
		EstimatedDate: estimatedDate,
		DaysRemaining: daysRemaining,
		CurrentUsage:  round2(currentUsage),
		WarningLevel:  warningLevel,
	}
}

// identifySavingsOpportunities 识别成本节省机会.
func (ca *CostAnalyzer) identifySavingsOpportunities(result *CollectResult, report *StorageReport, currentCost float64) []SavingOpportunity {
	var opportunities []SavingOpportunity

	// 1. 层级迁移机会
	if report != nil {
		for _, insight := range report.Insights.Insights {
			if insight.Type == "optimization" && insight.Saving > 0 {
				// 估算迁移节省
				savingTB := float64(insight.Saving) / float64(1024*1024*1024*1024)
				savingPerMonth := savingTB * (ca.getTierCost(TierHDD) - ca.getTierCost(TierCold))

				if savingPerMonth > 0 {
					opportunities = append(opportunities, SavingOpportunity{
						Type:           "tier_migration",
						Description:    "将冷数据迁移到低成本存储层",
						SavingPerMonth: round2(savingPerMonth),
						SavingPerYear:  round2(savingPerMonth * 12),
						Confidence:     0.8,
						Difficulty:     "easy",
					})
				}
			}
		}
	}

	// 2. 去重机会
	if report != nil && report.Health.RedundancyRate > 0.1 {
		redundantBytes := int64(float64(result.TotalSize) * report.Health.RedundancyRate)
		savingTB := float64(redundantBytes) / float64(1024*1024*1024*1024)
		savingPerMonth := savingTB * ca.getTierCost(TierHDD)

		opportunities = append(opportunities, SavingOpportunity{
			Type:           "dedup",
			Description:    "检测到冗余数据，启用去重可节省空间",
			SavingPerMonth: round2(savingPerMonth),
			SavingPerYear:  round2(savingPerMonth * 12),
			Confidence:     0.6,
			Difficulty:     "medium",
		})
	}

	// 3. 压缩机会
	largeFileCount := 0
	var largeFileSize int64
	for _, f := range result.Files {
		if f.Size > 100*1024*1024 { // > 100MB
			largeFileCount++
			largeFileSize += f.Size
		}
	}

	if largeFileCount > 0 {
		compressionSaving := float64(largeFileSize) * 0.3 // 假设30%压缩率
		savingTB := compressionSaving / float64(1024*1024*1024*1024)
		savingPerMonth := savingTB * ca.getTierCost(TierHDD)

		opportunities = append(opportunities, SavingOpportunity{
			Type:           "compression",
			Description:    "对大文件启用压缩存储",
			SavingPerMonth: round2(savingPerMonth),
			SavingPerYear:  round2(savingPerMonth * 12),
			Confidence:     0.7,
			Difficulty:     "easy",
		})
	}

	// 4. 冷数据归档
	var coldDataSize int64
	for _, f := range result.Files {
		if time.Since(f.AccessTime) > 365*24*time.Hour {
			coldDataSize += f.Size
		}
	}

	if coldDataSize > 1024*1024*1024 { // > 1GB
		coldTB := float64(coldDataSize) / float64(1024*1024*1024*1024)
		// 从HDD迁移到Cold存储的节省
		savingPerMonth := coldTB * (ca.getTierCost(TierHDD) - ca.getTierCost(TierCold))

		opportunities = append(opportunities, SavingOpportunity{
			Type:           "cold_archive",
			Description:    "超过1年未访问的数据可归档到冷存储",
			SavingPerMonth: round2(savingPerMonth),
			SavingPerYear:  round2(savingPerMonth * 12),
			Confidence:     0.9,
			Difficulty:     "easy",
		})
	}

	// 按节省金额降序排序
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].SavingPerMonth > opportunities[j].SavingPerMonth
	})

	return opportunities
}

// CompareCloudCosts 对比云存储成本.
func (ca *CostAnalyzer) CompareCloudCosts(totalBytes int64, localCost float64) *CloudCostComparison {
	totalTB := float64(totalBytes) / float64(1024*1024*1024*1024)

	comparison := &CloudCostComparison{
		LocalCostPerTB: localCost / totalTB,
	}

	var bestCost float64
	var bestProvider string

	for _, cloud := range ca.cloudCosts {
		monthlyCost := totalTB * cloud.CostPerTBMonth
		comparison.CloudProviders = append(comparison.CloudProviders, CloudProviderCost{
			Provider:       cloud.Provider,
			Tier:           cloud.Tier,
			CostPerTBMonth: cloud.CostPerTBMonth,
			MonthlyCost:    round2(monthlyCost),
			LatencyMs:      cloud.LatencyMs,
		})

		if bestCost == 0 || monthlyCost < bestCost {
			bestCost = monthlyCost
			bestProvider = cloud.Provider + " " + cloud.Tier
		}
	}

	comparison.BestOption = bestProvider
	comparison.SavingsVsCloud = round2(bestCost - localCost)

	return comparison
}

// DefaultCloudCosts 返回默认的云存储成本配置.
func DefaultCloudCosts() []CloudProviderCost {
	return []CloudProviderCost{
		{Provider: "AWS", Tier: "S3 Standard", CostPerTBMonth: 180, LatencyMs: 50},
		{Provider: "AWS", Tier: "S3 Glacier", CostPerTBMonth: 30, LatencyMs: 5000},
		{Provider: "Azure", Tier: "Blob Hot", CostPerTBMonth: 150, LatencyMs: 45},
		{Provider: "Azure", Tier: "Blob Cool", CostPerTBMonth: 80, LatencyMs: 100},
		{Provider: "阿里云", Tier: "OSS 标准", CostPerTBMonth: 120, LatencyMs: 30},
		{Provider: "阿里云", Tier: "OSS 归档", CostPerTBMonth: 15, LatencyMs: 3000},
		{Provider: "腾讯云", Tier: "COS 标准", CostPerTBMonth: 100, LatencyMs: 35},
		{Provider: "腾讯云", Tier: "COS 归档", CostPerTBMonth: 12, LatencyMs: 3000},
	}
}

// generateCostRecommendations 生成基于成本分析的优化建议.
func (ca *CostAnalyzer) generateCostRecommendations(result *CollectResult, report *StorageReport, costReport *StorageCostReport) []OptimizationRecommendation {
	var recommendations []OptimizationRecommendation

	if costReport == nil {
		return recommendations
	}

	// 1. 层级迁移建议
	for _, bd := range costReport.TierBreakdown {
		if bd.Tier == TierNVMe && bd.UsedTB > 2 {
			migrateTB := bd.UsedTB * 0.5
			savingPerMonth := migrateTB * (bd.CostPerTB - ca.getTierCost(TierSSD))
			if savingPerMonth > 10 {
				recommendations = append(recommendations, OptimizationRecommendation{
					Category:    "tier",
					Priority:    "high",
					Title:       "NVMe层级数据迁移建议",
					Description: fmt.Sprintf("NVMe层级有 %.1fTB 数据，建议将非频繁访问数据迁移到SSD层级", bd.UsedTB),
					Impact:      fmt.Sprintf("每月可节省 %.0f 元", savingPerMonth),
					SavingCost:  round2(savingPerMonth),
					Effort:      "medium",
					Steps: []string{
						"1. 分析NVMe层级中访问频率较低的文件",
						"2. 创建数据迁移计划",
						"3. 使用存储分层策略自动迁移",
						"4. 监控迁移后的性能影响",
					},
				})
			}
		}
	}

	// 2. 冷数据归档建议
	if result != nil {
		var coldDataSize int64
		for _, f := range result.Files {
			if time.Since(f.AccessTime) > 365*24*time.Hour {
				coldDataSize += f.Size
			}
		}
		if coldDataSize > 1024*1024*1024 {
			coldTB := float64(coldDataSize) / float64(1024*1024*1024*1024)
			savingPerMonth := coldTB * (ca.getTierCost(TierHDD) - ca.getTierCost(TierCold))
			recommendations = append(recommendations, OptimizationRecommendation{
				Category:    "lifecycle",
				Priority:    "high",
				Title:       "冷数据归档策略",
				Description: fmt.Sprintf("超过1年未访问的数据有 %s，建议归档到冷存储", formatBytes(coldDataSize)),
				Impact:      fmt.Sprintf("每月可节省 %.0f 元存储成本", savingPerMonth),
				SavingBytes: coldDataSize,
				SavingCost:  round2(savingPerMonth),
				Effort:      "easy",
				Steps: []string{
					"1. 配置自动归档策略（访问超过365天）",
					"2. 设置归档目标为冷存储层",
					"3. 配置归档前的数据完整性校验",
				},
			})
		}
	}

	// 3. 浪费空间清理
	if report != nil && report.Insights.WastedSpace > 100*1024*1024 {
		savingTB := float64(report.Insights.WastedSpace) / float64(1024*1024*1024*1024)
		savingPerMonth := savingTB * ca.getTierCost(TierHDD)
		recommendations = append(recommendations, OptimizationRecommendation{
			Category:    "cleanup",
			Priority:    "medium",
			Title:       "存储空间清理",
			Description: fmt.Sprintf("检测到 %s 临时文件和缓存占用空间", formatBytes(report.Insights.WastedSpace)),
			Impact:      fmt.Sprintf("可释放 %s 存储空间", formatBytes(report.Insights.WastedSpace)),
			SavingBytes: report.Insights.WastedSpace,
			SavingCost:  round2(savingPerMonth),
			Effort:      "easy",
			Steps: []string{
				"1. 扫描并列出临时文件",
				"2. 审核清理候选列表",
				"3. 执行清理操作",
				"4. 配置自动清理策略",
			},
		})
	}

	return recommendations
}
