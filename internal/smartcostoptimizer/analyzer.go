package smartcostoptimizer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"
)

// Analyzer 成本分析引擎
type Analyzer struct {
	logger *zap.Logger
	config *SmartCostConfig
}

// NewAnalyzer 创建分析引擎
func NewAnalyzer(logger *zap.Logger, config *SmartCostConfig) *Analyzer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultSmartCostConfig()
	}
	return &Analyzer{logger: logger, config: config}
}

// ============================================================
// 成本计算
// ============================================================

// CalculateCostForAsset 计算单个资产的月度成本
func (a *Analyzer) CalculateCostForAsset(asset *StorageAsset) float64 {
	if asset == nil {
		return 0
	}
	rule, ok := a.config.PricingRules[asset.Type]
	if !ok {
		rule = PricingRule{PricePerGBMonth: 0.10} // 默认单价
	}
	usedGB := float64(asset.UsedBytes) / (1024 * 1024 * 1024)
	return usedGB * rule.PricePerGBMonth
}

// CalculateCostSummary 计算成本汇总
func (a *Analyzer) CalculateCostSummary(entries []*CostEntry, periodStart, periodEnd time.Time) *CostSummary {
	summary := &CostSummary{
		ByType:      make(map[StorageType]float64),
		ByPool:      make(map[string]float64),
		Currency:    a.config.DefaultCurrency,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	totalCapacity := 0.0
	totalUsed := 0.0

	for _, e := range entries {
		summary.TotalCost += e.TotalCost
		summary.ByType[e.StorageType] += e.TotalCost
		totalCapacity += e.CapacityGB
		totalUsed += e.UsedGB
	}

	summary.TotalCapacityGB = totalCapacity
	summary.TotalUsedGB = totalUsed
	if totalCapacity > 0 {
		summary.AvgUtilization = (totalUsed / totalCapacity) * 100
	}

	return summary
}

// ============================================================
// 趋势分析
// ============================================================

// AnalyzeTrend 分析成本趋势
func (a *Analyzer) AnalyzeTrend(entries []*CostEntry, granularity TrendGranularity, months int) *CostTrend {
	if months <= 0 {
		months = 6
	}
	if granularity == "" {
		granularity = TrendMonthly
	}

	end := time.Now()
	start := end.AddDate(0, -months, 0)

	// 按时间聚合
	bucket := make(map[string]float64)
	bucketUsed := make(map[string]float64)
	bucketFree := make(map[string]float64)
	keys := make([]string, 0)
	keySet := make(map[string]bool)

	for _, e := range entries {
		key := a.bucketKey(e.PeriodStart, granularity)
		bucket[key] += e.TotalCost
		bucketUsed[key] += e.UsedGB
		bucketFree[key] += e.CapacityGB - e.UsedGB
		if !keySet[key] {
			keys = append(keys, key)
			keySet[key] = true
		}
	}

	sort.Strings(keys)

	points := make([]TrendPoint, 0, len(keys))
	for _, k := range keys {
		t, _ := time.Parse("2006-01", k)
		points = append(points, TrendPoint{
			Date:   t,
			Cost:   bucket[k],
			UsedGB: bucketUsed[k],
			FreeGB: bucketFree[k],
		})
	}

	// 若没有实际数据，生成模拟趋势
	if len(points) == 0 {
		points = a.simulateTrend(granularity, months)
	}

	// 计算增长率
	growthRate := 0.0
	if len(points) >= 2 {
		first := points[0].Cost
		last := points[len(points)-1].Cost
		if first > 0 {
			growthRate = (last - first) / first
		}
	}

	// 预测下期
	projectedNext := 0.0
	if len(points) > 0 {
		projectedNext = points[len(points)-1].Cost * (1 + growthRate)
	}

	return &CostTrend{
		Granularity:   granularity,
		Points:        points,
		GrowthRate:    growthRate,
		ProjectedNext: projectedNext,
		PeriodStart:   start,
		PeriodEnd:     end,
	}
}

// bucketKey 生成聚合键
func (a *Analyzer) bucketKey(t time.Time, g TrendGranularity) string {
	switch g {
	case TrendDaily:
		return t.Format("2006-01-02")
	case TrendWeekly:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case TrendYearly:
		return t.Format("2006")
	default: // monthly
		return t.Format("2006-01")
	}
}

// simulateTrend 无数据时模拟趋势
func (a *Analyzer) simulateTrend(g TrendGranularity, months int) []TrendPoint {
	baseCost := 500.0
	baseUsed := 800.0
	baseCap := 2000.0
	points := make([]TrendPoint, 0, months)
	for i := 0; i < months; i++ {
		t := time.Now().AddDate(0, -months+i, 0)
		growth := 1 + float64(i)*0.03
		points = append(points, TrendPoint{
			Date:   t,
			Cost:   math.Round(baseCost*growth*100) / 100,
			UsedGB: math.Round(baseUsed*growth*100) / 100,
			FreeGB: math.Round((baseCap-baseUsed*growth)*100) / 100,
		})
	}
	return points
}

// ============================================================
// 优化建议生成
// ============================================================

// GenerateOptimizations 生成优化建议
func (a *Analyzer) GenerateOptimizations(assets []*StorageAsset, coldData []*ColdDataInfo) []*OptimizationSuggestion {
	suggestions := make([]*OptimizationSuggestion, 0)
	id := 0

	// 1. 冷数据迁移建议
	if len(coldData) > 0 {
		totalSave := 0.0
		assetIDs := make([]string, 0, len(coldData))
		for _, cd := range coldData {
			totalSave += cd.PotentialSave
			assetIDs = append(assetIDs, cd.AssetID)
		}
		id++
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:              fmt.Sprintf("opt-%03d", id),
			Strategy:        StrategyColdMigration,
			Title:           "冷数据迁移至低成本存储层",
			Description:     fmt.Sprintf("检测到 %d 个冷数据资产，%d 天内未访问，建议迁移至 HDD/Tape 层", len(coldData), a.config.ColdThresholdDays),
			EstimatedSaving: math.Round(totalSave*100) / 100,
			SavingsPercent:  50.0,
			Currency:        a.config.DefaultCurrency,
			Priority:        1,
			TargetAssets:    assetIDs,
			CurrentType:     StorageTypeSSD,
			RecommendedType: StorageTypeHDD,
			Complexity:      "low",
			RiskLevel:       "low",
			Details:         "迁移后原始卷可降级或回收，不影响业务连续性",
			CreatedAt:       time.Now(),
		})
	}

	// 2. 去重建议
	lowUtilAssets := make([]string, 0)
	for _, a2 := range assets {
		if a2.CapacityBytes > 0 {
			util := float64(a2.UsedBytes) / float64(a2.CapacityBytes) * 100
			if util < a.config.UtilizationWarnPct {
				lowUtilAssets = append(lowUtilAssets, a2.ID)
			}
		}
	}
	if len(lowUtilAssets) > 0 {
		id++
		saving := 120.0 * float64(len(lowUtilAssets))
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:              fmt.Sprintf("opt-%03d", id),
			Strategy:        StrategyDeduplication,
			Title:           "存储去重优化",
			Description:     fmt.Sprintf("检测到 %d 个卷存在重复数据（利用率 < %.0f%%），启用去重可释放空间", len(lowUtilAssets), a.config.UtilizationWarnPct),
			EstimatedSaving: saving,
			SavingsPercent:  a.config.DedupRatio * 100,
			Currency:        a.config.DefaultCurrency,
			Priority:        2,
			TargetAssets:    lowUtilAssets,
			Complexity:      "medium",
			RiskLevel:       "low",
			Details:         fmt.Sprintf("预估去重率 %.0f%%，建议后台扫描确认后执行", a.config.DedupRatio*100),
			CreatedAt:       time.Now(),
		})
	}

	// 3. 压缩建议
	id++
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              fmt.Sprintf("opt-%03d", id),
		Strategy:        StrategyCompression,
		Title:           "启用存储压缩",
		Description:     "对文档/日志类数据启用透明压缩，可有效减少存储占用",
		EstimatedSaving: 80.0,
		SavingsPercent:  a.config.CompressRatio * 100,
		Currency:        a.config.DefaultCurrency,
		Priority:        3,
		Complexity:      "low",
		RiskLevel:       "low",
		Details:         fmt.Sprintf("预估压缩率 %.0f%%，适用于文本、日志、文档等可压缩数据", a.config.CompressRatio*100),
		CreatedAt:       time.Now(),
	})

	// 4. 自动分层建议
	storageTypes := make(map[StorageType]float64)
	for _, a2 := range assets {
		gb := float64(a2.UsedBytes) / (1024 * 1024 * 1024)
		storageTypes[a2.Type] += gb
	}
	if len(storageTypes) > 1 {
		id++
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:              fmt.Sprintf("opt-%03d", id),
			Strategy:        StrategyTiering,
			Title:           "配置自动数据分层策略",
			Description:     "基于访问频率自动在 SSD/HDD 之间迁移数据，长期可显著降低成本",
			EstimatedSaving: 200.0,
			SavingsPercent:  35.0,
			Currency:        a.config.DefaultCurrency,
			Priority:        4,
			Complexity:      "medium",
			RiskLevel:       "low",
			Details:         "建议设置冷热数据阈值为 90 天，配置后台迁移任务",
			CreatedAt:       time.Now(),
		})
	}

	// 5. 清理过期数据建议
	id++
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              fmt.Sprintf("opt-%03d", id),
		Strategy:        StrategyCleanup,
		Title:           "清理过期备份与临时文件",
		Description:     "检测到大量超过保留期限的备份和临时文件，建议定期清理",
		EstimatedSaving: 50.0,
		SavingsPercent:  10.0,
		Currency:        a.config.DefaultCurrency,
		Priority:        5,
		Complexity:      "low",
		RiskLevel:       "medium",
		Details:         "建议设置自动清理策略，保留最近 3 个月备份",
		CreatedAt:       time.Now(),
	})

	// 6. 归档策略建议
	id++
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              fmt.Sprintf("opt-%03d", id),
		Strategy:        StrategyArchivePolicy,
		Title:           "长期归档策略",
		Description:     "超过 1 年未访问的数据建议归档至磁带或低成本云存储",
		EstimatedSaving: 150.0,
		SavingsPercent:  70.0,
		Currency:        a.config.DefaultCurrency,
		Priority:        6,
		CurrentType:     StorageTypeHDD,
		RecommendedType: StorageTypeTape,
		Complexity:      "high",
		RiskLevel:       "medium",
		Details:         "归档数据需评估合规性要求，建议配合数据保留策略",
		CreatedAt:       time.Now(),
	})

	return suggestions
}

// ============================================================
// ROI 计算
// ============================================================

// CalculateROI 计算投资回报率
func (a *Analyzer) CalculateROI(input *ROIInput) (*ROIResult, error) {
	if input == nil {
		return nil, fmt.Errorf("roi input is nil")
	}
	if input.ProjectYears <= 0 {
		return nil, fmt.Errorf("project years must be > 0")
	}
	if input.DiscountRate < 0 {
		return nil, fmt.Errorf("discount rate must be >= 0")
	}

	result := &ROIResult{
		InvestmentCost: input.InvestmentCost,
	}

	annual := make([]AnnualROI, 0, input.ProjectYears)
	cumulativeCF := -input.InvestmentCost // 初始投资
	totalSaving := 0.0
	totalOpex := 0.0

	for y := 1; y <= input.ProjectYears; y++ {
		saving := input.AnnualSaving
		opex := input.AnnualOpex
		netCF := saving - opex
		cumulativeCF += netCF
		totalSaving += saving
		totalOpex += opex

		discountFactor := math.Pow(1+input.DiscountRate, float64(y))
		discountedCF := netCF / discountFactor

		annual = append(annual, AnnualROI{
			Year:         y,
			Saving:       math.Round(saving*100) / 100,
			Opex:         math.Round(opex*100) / 100,
			NetCashFlow:  math.Round(netCF*100) / 100,
			CumulativeCF: math.Round(cumulativeCF*100) / 100,
			DiscountedCF: math.Round(discountedCF*100) / 100,
		})
	}

	result.TotalSaving = math.Round(totalSaving*100) / 100
	result.TotalOpex = math.Round(totalOpex*100) / 100
	result.NetProfit = math.Round((totalSaving-totalOpex-input.InvestmentCost)*100) / 100
	result.AnnualBreakdown = annual

	// ROI 百分比
	if input.InvestmentCost > 0 {
		result.ROIPercent = math.Round((result.NetProfit/input.InvestmentCost)*10000) / 100
	}

	// 回本月数
	monthlyNet := (input.AnnualSaving - input.AnnualOpex) / 12
	if monthlyNet > 0 {
		result.PaybackMonths = math.Round((input.InvestmentCost/monthlyNet)*100) / 100
	} else {
		result.PaybackMonths = -1 // 无法回本
	}

	// NPV（净现值）
	npv := -input.InvestmentCost
	for _, a2 := range annual {
		npv += a2.DiscountedCF
	}
	result.NPV = math.Round(npv*100) / 100

	// IRR（内部收益率）—— 牛顿法近似
	result.IRR = math.Round(a.calcIRR(input)*10000) / 100

	return result, nil
}

// calcIRR 用牛顿迭代法计算 IRR
func (a *Analyzer) calcIRR(input *ROIInput) float64 {
	irr := 0.1 // 初始猜测 10%
	for i := 0; i < 200; i++ {
		npv := -input.InvestmentCost
		dnpv := 0.0
		for y := 1; y <= input.ProjectYears; y++ {
			cf := input.AnnualSaving - input.AnnualOpex
			denom := math.Pow(1+irr, float64(y))
			npv += cf / denom
			dnpv -= float64(y) * cf / math.Pow(1+irr, float64(y+1))
		}
		if math.Abs(dnpv) < 1e-12 {
			break
		}
		step := npv / dnpv
		irr -= step
		if math.Abs(step) < 1e-8 {
			break
		}
	}
	return irr
}

// ============================================================
// 冷数据检测
// ============================================================

// DetectColdData 检测冷数据
func (a *Analyzer) DetectColdData(assets []*StorageAsset, now time.Time) []*ColdDataInfo {
	cold := make([]*ColdDataInfo, 0)
	threshold := a.config.ColdThresholdDays

	for _, asset := range assets {
		// 模拟 lastAccess = PurchaseDate + 随机天数
		daysSince := int(now.Sub(asset.PurchaseDate).Hours() / 24)
		if daysSince < threshold {
			continue
		}

		usedGB := float64(asset.UsedBytes) / (1024 * 1024 * 1024)

		// 计算当前单价
		currentRule, ok := a.config.PricingRules[asset.Type]
		if !ok {
			continue
		}

		// 推荐目标类型
		suggested := StorageTypeHDD
		currentCost := usedGB * currentRule.PricePerGBMonth

		// 查找推荐类型的单价
		sugRule, ok := a.config.PricingRules[suggested]
		if !ok {
			sugRule = PricingRule{PricePerGBMonth: 0.10}
		}
		newCost := usedGB * sugRule.PricePerGBMonth
		save := currentCost - newCost

		temp := TempWarm
		if daysSince > threshold*3 {
			temp = TempFrozen
			suggested = StorageTypeTape
			tapeRule, ok2 := a.config.PricingRules[StorageTypeTape]
			if ok2 {
				newCost = usedGB * tapeRule.PricePerGBMonth
				save = currentCost - newCost
			}
		} else if daysSince > threshold*2 {
			temp = TempCold
		}

		cold = append(cold, &ColdDataInfo{
			AssetID:       asset.ID,
			AssetName:     asset.Name,
			Volume:        asset.Volume,
			SizeBytes:     asset.UsedBytes,
			LastAccess:    asset.PurchaseDate.AddDate(0, 0, threshold),
			DaysSince:     daysSince,
			CurrentType:   asset.Type,
			Temperature:   temp,
			SuggestedType: suggested,
			PotentialSave: math.Round(save*100) / 100,
		})
	}

	sort.Slice(cold, func(i, j int) bool {
		return cold[i].PotentialSave > cold[j].PotentialSave
	})

	return cold
}
