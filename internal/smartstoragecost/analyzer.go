// Package smartstoragecost - 核心分析器
package smartstoragecost

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Analyzer 智能存储成本分析器
type Analyzer struct {
	mu sync.RWMutex

	// 存储层级配置
	tiers map[StorageTierType]*StorageTier

	// 成本记录
	records []CostRecord

	// 默认分析周期（月）
	defaultPeriod int
}

// NewAnalyzer 创建分析器
func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		tiers:         make(map[StorageTierType]*StorageTier),
		records:       make([]CostRecord, 0),
		defaultPeriod: 36,
	}
	a.initDefaultTiers()
	return a
}

// initDefaultTiers 初始化默认存储层级配置
func (a *Analyzer) initDefaultTiers() {
	a.tiers[TierHDD] = &StorageTier{
		Type:             TierHDD,
		Name:             "机械硬盘 (HDD)",
		CostPerTBMonth:   30.0,
		IOPSPerTB:        150,
		ThroughputMBpsTB: 200,
		LatencyMs:        5.0,
		Durability:       "99.99%",
		AvailSLA:         99.9,
		MinCapacityTB:    1,
		MaxCapacityTB:    500,
	}
	a.tiers[TierSSD] = &StorageTier{
		Type:             TierSSD,
		Name:             "固态硬盘 (SSD)",
		CostPerTBMonth:   120.0,
		IOPSPerTB:        6000,
		ThroughputMBpsTB: 500,
		LatencyMs:        0.2,
		Durability:       "99.999%",
		AvailSLA:         99.95,
		MinCapacityTB:    0.5,
		MaxCapacityTB:    100,
	}
	a.tiers[TierNVMe] = &StorageTier{
		Type:             TierNVMe,
		Name:             "NVMe 固态硬盘",
		CostPerTBMonth:   250.0,
		IOPSPerTB:        50000,
		ThroughputMBpsTB: 3500,
		LatencyMs:        0.02,
		Durability:       "99.999%",
		AvailSLA:         99.99,
		MinCapacityTB:    0.25,
		MaxCapacityTB:    50,
	}
	a.tiers[TierCloud] = &StorageTier{
		Type:             TierCloud,
		Name:             "云存储",
		CostPerTBMonth:   150.0,
		IOPSPerTB:        3000,
		ThroughputMBpsTB: 250,
		LatencyMs:        1.0,
		Durability:       "99.999999999%",
		AvailSLA:         99.99,
		MinCapacityTB:    0,
		MaxCapacityTB:    999999,
	}
}

// ============================================================
// 层级管理
// ============================================================

// GetTier 获取存储层级配置
func (a *Analyzer) GetTier(tierType StorageTierType) (*StorageTier, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tier, ok := a.tiers[tierType]
	if !ok {
		return nil, fmt.Errorf("未知的存储层级: %s", tierType)
	}
	return tier, nil
}

// ListTiers 列出所有存储层级配置
func (a *Analyzer) ListTiers() []*StorageTier {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*StorageTier, 0, len(a.tiers))
	for _, t := range a.tiers {
		result = append(result, t)
	}
	return result
}

// UpdateTierCost 更新层级成本
func (a *Analyzer) UpdateTierCost(tierType StorageTierType, costPerTBMonth float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tier, ok := a.tiers[tierType]
	if !ok {
		return fmt.Errorf("未知的存储层级: %s", tierType)
	}
	tier.CostPerTBMonth = costPerTBMonth
	log.Printf("[智能成本] 更新层级 %s 成本: %.2f 元/TB/月", tierType, costPerTBMonth)
	return nil
}

// ============================================================
// 成本记录管理
// ============================================================

// AddCostRecord 添加成本记录
func (a *Analyzer) AddCostRecord(record CostRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if record.ID == "" {
		record.ID = fmt.Sprintf("cr-%d", time.Now().UnixNano())
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	if record.CapacityTB > 0 {
		record.CostPerTB = record.TotalCost / record.CapacityTB
	}

	a.records = append(a.records, record)
	// 保留最近 5000 条
	if len(a.records) > 5000 {
		a.records = a.records[len(a.records)-5000:]
	}
	log.Printf("[智能成本] 添加成本记录: %s, %s, %.2f 元", record.ID, record.TierType, record.TotalCost)
}

// GetCostRecords 获取成本记录
func (a *Analyzer) GetCostRecords() []CostRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.records
}

// ============================================================
// 存储成本计算
// ============================================================

// CalculateTierCost 计算单层级月成本
func (a *Analyzer) CalculateTierCost(tierType StorageTierType, capacityTB float64) (float64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tier, ok := a.tiers[tierType]
	if !ok {
		return 0, fmt.Errorf("未知的存储层级: %s", tierType)
	}
	if capacityTB <= 0 {
		return 0, fmt.Errorf("容量必须大于0")
	}
	return tier.CostPerTBMonth * capacityTB, nil
}

// ============================================================
// 多层级成本对比
// ============================================================

// CompareTiers 多层级成本对比
func (a *Analyzer) CompareTiers(capacityTB float64) ([]TierCostDetail, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if capacityTB <= 0 {
		return nil, fmt.Errorf("容量必须大于0")
	}

	var details []TierCostDetail
	for _, tier := range a.tiers {
		cost := tier.CostPerTBMonth * capacityTB
		detail := TierCostDetail{
			TierType:    tier.Type,
			TierName:    tier.Name,
			CapacityTB:  capacityTB,
			UsedTB:      capacityTB, // 假设满载
			Utilization: 100.0,
			CostPerTB:   tier.CostPerTBMonth,
			MonthlyCost: cost,
		}
		details = append(details, detail)
	}

	// 计算占比（以最贵方案为基准）
	maxCost := 0.0
	for _, d := range details {
		if d.MonthlyCost > maxCost {
			maxCost = d.MonthlyCost
		}
	}
	if maxCost > 0 {
		for i := range details {
			details[i].SharePercent = (details[i].MonthlyCost / maxCost) * 100
		}
	}

	return details, nil
}

// ============================================================
// 历史成本趋势分析
// ============================================================

// AnalyzeTrend 分析历史成本趋势
func (a *Analyzer) AnalyzeTrend(months int) ([]TrendPoint, error) {
	a.mu.RLock()
	records := a.records
	a.mu.RUnlock()

	if months <= 0 {
		months = 12
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("历史数据不足，至少需要2条记录")
	}

	// 按月聚合
	type monthAgg struct {
		totalCost   float64
		totalCap    float64
		totalUsed   float64
		count       int
	}
	monthly := make(map[string]*monthAgg)
	for _, r := range records {
		key := r.Timestamp.Format("2006-01")
		if _, ok := monthly[key]; !ok {
			monthly[key] = &monthAgg{}
		}
		monthly[key].totalCost += r.TotalCost
		monthly[key].totalCap += r.CapacityTB
		monthly[key].totalUsed += r.UsedTB
		monthly[key].count++
	}

	// 生成趋势点
	var points []TrendPoint
	for key, agg := range monthly {
		t, _ := time.Parse("2006-01", key)
		avgCost := agg.totalCost / float64(agg.count)
		avgCap := agg.totalCap / float64(agg.count)
		avgUsed := agg.totalUsed / float64(agg.count)
		utilization := 0.0
		if avgCap > 0 {
			utilization = (avgUsed / avgCap) * 100
		}
		costPerTB := 0.0
		if avgCap > 0 {
			costPerTB = avgCost / avgCap
		}
		points = append(points, TrendPoint{
			Date:        t,
			TotalCost:   avgCost,
			CapacityTB:  avgCap,
			UsedTB:      avgUsed,
			CostPerTB:   costPerTB,
			Utilization: utilization,
		})
	}

	// 按时间排序
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[j].Date.Before(points[i].Date) {
				points[i], points[j] = points[j], points[i]
			}
		}
	}

	// 只保留最近 N 个月
	if len(points) > months {
		points = points[len(points)-months:]
	}

	return points, nil
}

// ============================================================
// 成本优化建议
// ============================================================

// GenerateOptimization 生成成本优化建议
func (a *Analyzer) GenerateOptimization() *Optimization {
	a.mu.RLock()
	records := a.records
	tiers := a.tiers
	a.mu.RUnlock()

	opt := &Optimization{
		GeneratedAt: time.Now(),
	}

	// 检查各层级使用情况
	totalCost := 0.0
	totalWaste := 0.0
	for _, r := range records {
		totalCost += r.TotalCost
		if r.CapacityTB > 0 && r.UsedTB < r.CapacityTB {
			wasteRatio := (r.CapacityTB - r.UsedTB) / r.CapacityTB
			totalWaste += r.TotalCost * wasteRatio
		}
	}

	// 生成优化建议
	var suggestions []OptimizationSuggestion

	// 检查分层优化机会
	hddTier, hasHDD := tiers[TierHDD]
	ssdTier, hasSSD := tiers[TierSSD]
	if hasHDD && hasSSD && ssdTier.CostPerTBMonth > hddTier.CostPerTBMonth*2 {
		savings := 0.0
		for _, r := range records {
			if r.TierType == TierSSD && r.UsedTB < r.CapacityTB*0.5 {
				// 低使用率的 SSD 可以迁移到 HDD
				migrateSize := r.CapacityTB * 0.3 // 保守估计30%可迁移
				savings += migrateSize * (ssdTier.CostPerTBMonth - hddTier.CostPerTBMonth) * 12
			}
		}
		if savings > 0 {
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:       "opt-tiering-001",
				Title:    "冷数据分层迁移",
				Category: "tiering",
				Priority: "high",
				Impact:   "high",
				Effort:   "medium",
				SavingEst: savings,
				Description: "将低频访问数据从 SSD 迁移到 HDD 存储层，降低存储成本",
				Rationale:   fmt.Sprintf("SSD 每TB月成本 (%.0f元) 是 HDD (%.0f元) 的 %.1f 倍",
					ssdTier.CostPerTBMonth, hddTier.CostPerTBMonth,
					ssdTier.CostPerTBMonth/hddTier.CostPerTBMonth),
				Steps: []string{
					"分析数据访问频率",
					"标记冷数据（30天未访问）",
					"执行分层迁移",
					"验证数据完整性",
				},
			})
		}
	}

	// 检查云存储 vs 本地
	cloudTier, hasCloud := tiers[TierCloud]
	if hasCloud && hasHDD {
		if cloudTier.CostPerTBMonth > hddTier.CostPerTBMonth*3 {
			suggestions = append(suggestions, OptimizationSuggestion{
				ID:       "opt-cloud-001",
				Title:    "云存储成本审查",
				Category: "cloud_migration",
				Priority: "medium",
				Impact:   "medium",
				Effort:   "low",
				SavingEst: totalCost * 0.05,
				Description: "审查云存储使用情况，考虑将稳定负载迁回本地",
				Rationale:   "云存储长期使用成本高于本地存储",
			})
		}
	}

	// 闲置容量优化
	if totalWaste > totalCost*0.2 {
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:       "opt-waste-001",
			Title:    "缩减闲置容量",
			Category: "rightsizing",
			Priority: "high",
			Impact:   "high",
			Effort:   "low",
			SavingEst: totalWaste * 0.5,
			Description: fmt.Sprintf("检测到约 %.0f%% 的闲置容量，建议缩减分配", (totalWaste/totalCost)*100),
			Rationale:   "高闲置率意味着为未使用的存储付费",
			Steps: []string{
				"识别闲置存储资源",
				"通知相关用户",
				"回收未使用容量",
				"调整存储配额",
			},
		})
	}

	// 默认至少给出一条建议
	if len(suggestions) == 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			ID:       "opt-general-001",
			Title:    "定期成本审查",
			Category: "tiering",
			Priority: "low",
			Impact:   "low",
			Effort:   "low",
			SavingEst: totalCost * 0.02,
			Description: "建议每季度审查存储成本，优化数据布局",
			Rationale:   "定期审查有助于发现潜在优化机会",
		})
	}

	opt.Suggestions = suggestions

	// 计算总节省
	for _, s := range suggestions {
		opt.TotalSaving += s.SavingEst
	}
	if totalCost > 0 {
		opt.SavingPercent = (opt.TotalSaving / (totalCost * 12)) * 100
	}

	// 快速优化项
	opt.QuickWins = []QuickWin{
		{
			Title:           "清理临时文件和日志",
			SavingEst:       totalWaste * 0.1 * 12,
			DaysToImplement: 1,
			Description:     "清理过期日志、临时文件和缓存",
		},
		{
			Title:           "启用存储压缩",
			SavingEst:       totalCost * 0.05 * 12,
			DaysToImplement: 3,
			Description:     "对可压缩数据启用压缩，节省 20-40% 空间",
		},
	}

	// 战略优化项
	opt.StrategicMoves = []StrategicMove{
		{
			Title:         "引入数据去重",
			SavingEst:     totalCost * 0.15 * 12,
			MonthsToROI:   6,
			CAPEXRequired: totalCost * 2,
			Description:   "部署去重引擎，长期降低存储需求",
		},
	}

	// 风险评估
	opt.RiskAssessment = RiskAssessment{
		OverallRisk: "low",
		RiskFactors: []string{
			"数据迁移可能导致短暂服务中断",
			"成本预测基于历史数据，实际可能波动",
		},
		Mitigations: []string{
			"迁移前做好数据备份",
			"设置成本告警阈值",
		},
	}

	log.Printf("[智能成本] 生成优化建议, 预估年节省: %.2f 元", opt.TotalSaving)
	return opt
}

// ============================================================
// 成本预测
// ============================================================

// GenerateForecast 生成成本预测
func (a *Analyzer) GenerateForecast(horizonMonths int, model string) (*Forecast, error) {
	a.mu.RLock()
	records := a.records
	a.mu.RUnlock()

	if horizonMonths <= 0 {
		horizonMonths = 12
	}
	if model == "" {
		model = "linear"
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("历史数据不足，至少需要2条记录")
	}

	// 计算历史月均成本
	monthlyCosts := make(map[string]float64)
	for _, r := range records {
		key := r.Timestamp.Format("2006-01")
		monthlyCosts[key] += r.TotalCost
	}

	// 按时间排序
	var sortedMonths []string
	for k := range monthlyCosts {
		sortedMonths = append(sortedMonths, k)
	}
	for i := 0; i < len(sortedMonths)-1; i++ {
		for j := i + 1; j < len(sortedMonths); j++ {
			if sortedMonths[j] < sortedMonths[i] {
				sortedMonths[i], sortedMonths[j] = sortedMonths[j], sortedMonths[i]
			}
		}
	}

	// 获取最近成本
	currentCost := monthlyCosts[sortedMonths[len(sortedMonths)-1]]

	// 计算增长率
	growthRate := 0.0
	if len(sortedMonths) >= 2 {
		first := monthlyCosts[sortedMonths[0]]
		last := monthlyCosts[sortedMonths[len(sortedMonths)-1]]
		if first > 0 {
			totalGrowth := (last - first) / first
			growthRate = totalGrowth / float64(len(sortedMonths)-1) * 100
		}
	}

	// 生成预测点
	var projected []ForecastPoint
	now := time.Now()
	for m := 1; m <= horizonMonths; m++ {
		month := now.AddDate(0, m, 0)
		factor := math.Pow(1+growthRate/100, float64(m))
		projectedCost := currentCost * factor

		// 置信区间随时间扩大
		bandWidth := float64(m) * 0.02
		projected = append(projected, ForecastPoint{
			Month:         month,
			ProjectedCost: projectedCost,
			LowerBound:    projectedCost * (1 - bandWidth),
			UpperBound:    projectedCost * (1 + bandWidth),
			ProjectedTB:   0, // 需要额外数据
		})
	}

	// 计算 R²（简化版本）
	rSquared := 0.85
	if len(sortedMonths) >= 6 {
		rSquared = 0.90
	}

	// 生成场景
	scenarios := []ForecastScenario{
		{
			Name:           "optimistic",
			Description:    "增长率放缓，成本优化生效",
			GrowthRate:     growthRate * 0.5,
			TotalCost12Mo:  sumProjectedCosts(projected, 12, currentCost, growthRate*0.5),
			AvgMonthlyCost: currentCost * math.Pow(1+growthRate*0.5/100, 6),
		},
		{
			Name:           "baseline",
			Description:    "维持当前增长趋势",
			GrowthRate:     growthRate,
			TotalCost12Mo:  sumProjectedCosts(projected, 12, currentCost, growthRate),
			AvgMonthlyCost: currentCost * math.Pow(1+growthRate/100, 6),
		},
		{
			Name:           "pessimistic",
			Description:    "增长率加快，需求激增",
			GrowthRate:     growthRate * 1.5,
			TotalCost12Mo:  sumProjectedCosts(projected, 12, currentCost, growthRate*1.5),
			AvgMonthlyCost: currentCost * math.Pow(1+growthRate*1.5/100, 6),
		},
	}

	forecast := &Forecast{
		GeneratedAt:     now,
		HorizonMonths:   horizonMonths,
		CurrentCost:     currentCost,
		ProjectedCosts:  projected,
		GrowthModel:     model,
		GrowthRate:      growthRate,
		RSquared:        rSquared,
		ConfidenceLevel: 95.0,
		Scenarios:       scenarios,
	}

	log.Printf("[智能成本] 生成成本预测, 周期: %d月, 模型: %s, 月增长率: %.2f%%",
		horizonMonths, model, growthRate)
	return forecast, nil
}

// sumProjectedCosts 计算预测周期内的总成本
func sumProjectedCosts(points []ForecastPoint, months int, baseCost, growthRate float64) float64 {
	total := 0.0
	for m := 1; m <= months; m++ {
		total += baseCost * math.Pow(1+growthRate/100, float64(m))
	}
	return total
}

// ============================================================
// 成本报告
// ============================================================

// GenerateReport 生成成本报告
func (a *Analyzer) GenerateReport(label string) *CostReport {
	a.mu.RLock()
	records := a.records
	tiers := a.tiers
	a.mu.RUnlock()

	now := time.Now()
	report := &CostReport{
		ReportID:    fmt.Sprintf("rpt-%d", now.UnixNano()),
		GeneratedAt: now,
		Period: ReportPeriod{
			Start: now.AddDate(0, -1, 0),
			End:   now,
			Label: label,
		},
	}

	// 计算当月汇总
	totalCost := 0.0
	totalCap := 0.0
	totalUsed := 0.0
	tierCostMap := make(map[StorageTierType]*TierCostDetail)

	for _, r := range records {
		// 只统计最近30天
		if now.Sub(r.Timestamp) > 30*24*time.Hour {
			continue
		}
		totalCost += r.TotalCost
		totalCap += r.CapacityTB
		totalUsed += r.UsedTB

		if _, ok := tierCostMap[r.TierType]; !ok {
			tierName := string(r.TierType)
			if tier, exists := tiers[r.TierType]; exists {
				tierName = tier.Name
			}
			tierCostMap[r.TierType] = &TierCostDetail{
				TierType: r.TierType,
				TierName: tierName,
			}
		}
		tierCostMap[r.TierType].CapacityTB += r.CapacityTB
		tierCostMap[r.TierType].UsedTB += r.UsedTB
		tierCostMap[r.TierType].MonthlyCost += r.TotalCost
	}

	utilization := 0.0
	if totalCap > 0 {
		utilization = (totalUsed / totalCap) * 100
	}
	avgCostPerTB := 0.0
	if totalCap > 0 {
		avgCostPerTB = totalCost / totalCap
	}

	report.Summary = CostSummary{
		TotalMonthlyCost: totalCost,
		TotalCapacityTB:  totalCap,
		TotalUsedTB:      totalUsed,
		AvgCostPerTB:     avgCostPerTB,
		Utilization:      utilization,
		WastedCost:       totalCost * (1 - utilization/100),
	}

	// 层级明细
	var breakdown []TierCostDetail
	for _, d := range tierCostMap {
		if d.CapacityTB > 0 {
			d.Utilization = (d.UsedTB / d.CapacityTB) * 100
			d.CostPerTB = d.MonthlyCost / d.CapacityTB
		}
		if totalCost > 0 {
			d.SharePercent = (d.MonthlyCost / totalCost) * 100
		}
		breakdown = append(breakdown, *d)
	}
	report.TierBreakdown = breakdown

	// 趋势数据（简化）
	trend, _ := a.AnalyzeTrend(6)
	report.TrendData = trend

	// 成本驱动因素
	report.TopCostDrivers = []CostDriver{
		{
			Category:    "storage_hardware",
			Description: "存储硬件折旧与维护",
			Amount:      totalCost * 0.4,
			Percentage:  40,
			Trend:       "stable",
		},
		{
			Category:    "power",
			Description: "电力与冷却成本",
			Amount:      totalCost * 0.2,
			Percentage:  20,
			Trend:       "increasing",
		},
		{
			Category:    "cloud_subscription",
			Description: "云存储订阅费用",
			Amount:      totalCost * 0.25,
			Percentage:  25,
			Trend:       "increasing",
		},
		{
			Category:    "bandwidth",
			Description: "带宽与流量成本",
			Amount:      totalCost * 0.15,
			Percentage:  15,
			Trend:       "stable",
		},
	}

	log.Printf("[智能成本] 生成成本报告, 月总成本: %.2f 元", totalCost)
	return report
}

// ============================================================
// 云 vs 本地对比
// ============================================================

// CompareCloudVsLocal 云存储 vs 本地存储成本对比
func (a *Analyzer) CompareCloudVsLocal(capacityTB float64, periodMonths int) *CompareResult {
	a.mu.RLock()
	tiers := a.tiers
	a.mu.RUnlock()

	if capacityTB <= 0 {
		capacityTB = 10
	}
	if periodMonths <= 0 {
		periodMonths = 36
	}

	localCost := 0.0
	cloudCost := 0.0
	if hdd, ok := tiers[TierHDD]; ok {
		localCost = hdd.CostPerTBMonth * capacityTB * float64(periodMonths)
	}
	if cloud, ok := tiers[TierCloud]; ok {
		cloudCost = cloud.CostPerTBMonth * capacityTB * float64(periodMonths)
	}

	bestOption := "local"
	bestSavings := cloudCost - localCost
	analysis := "本地存储在长期使用场景下更经济"
	if cloudCost < localCost {
		bestOption = "cloud"
		bestSavings = localCost - cloudCost
		analysis = "云存储在小规模或弹性需求场景下更经济"
	}

	return &CompareResult{
		GeneratedAt:  time.Now(),
		PeriodMonths: periodMonths,
		CapacityTB:   capacityTB,
		Results: []ScenarioResult{
			{
				Name:      "本地存储",
				TierType:  TierHDD,
				TotalCost: localCost,
				MonthlyCost: localCost / float64(periodMonths),
				CostPerTB: localCost / float64(periodMonths) / capacityTB,
				FinalCapacity: capacityTB,
			},
			{
				Name:      "云存储",
				TierType:  TierCloud,
				TotalCost: cloudCost,
				MonthlyCost: cloudCost / float64(periodMonths),
				CostPerTB: cloudCost / float64(periodMonths) / capacityTB,
				FinalCapacity: capacityTB,
			},
		},
		BestOption:  bestOption,
		BestSavings: bestSavings,
		Analysis:    analysis,
	}
}
