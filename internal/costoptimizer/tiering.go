package costoptimizer

import (
	"fmt"
	"sort"
)

// TieringAnalyzer 分层存储成本对比分析器
type TieringAnalyzer struct {
	optimizer *CostOptimizer
}

// NewTieringAnalyzer 创建分层分析器
func NewTieringAnalyzer(co *CostOptimizer) *TieringAnalyzer {
	return &TieringAnalyzer{optimizer: co}
}

// StorageScheme 存储方案
type StorageScheme string

const (
	SchemeAllSSD  StorageScheme = "all_ssd"   // 全 SSD
	SchemeHybrid  StorageScheme = "hybrid"    // 混合 SSD + HDD
	SchemeAllHDD  StorageScheme = "all_hdd"   // 全 HDD
)

// SchemeProfile 存储方案定义
type SchemeProfile struct {
	Scheme      StorageScheme `json:"scheme"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	// 数据分配比例
	HotDataTier  StorageTier `json:"hot_data_tier"`
	WarmDataTier StorageTier `json:"warm_data_tier"`
	ColdDataTier StorageTier `json:"cold_data_tier"`
	// 各层占比
	HotPercent  float64 `json:"hot_percent"`
	WarmPercent float64 `json:"warm_percent"`
	ColdPercent float64 `json:"cold_percent"`
	// 特性
	ReadPerformance  string `json:"read_performance"`
	WritePerformance string `json:"write_performance"`
	Reliability      string `json:"reliability"`
}

// DefaultSchemeProfiles 默认存储方案
var DefaultSchemeProfiles = map[StorageScheme]SchemeProfile{
	SchemeAllSSD: {
		Scheme:           SchemeAllSSD,
		Name:             "全 SSD 方案",
		Description:      "所有数据存储在 SSD 上，性能最佳，成本最高",
		HotDataTier:      TierSSD,
		WarmDataTier:     TierSSD,
		ColdDataTier:     TierSSD,
		HotPercent:       100,
		WarmPercent:      0,
		ColdPercent:      0,
		ReadPerformance:  "优秀",
		WritePerformance: "优秀",
		Reliability:      "99.999%",
	},
	SchemeHybrid: {
		Scheme:           SchemeHybrid,
		Name:             "混合方案（推荐）",
		Description:      "热数据 SSD + 温冷数据 HDD，性价比最优",
		HotDataTier:      TierSSD,
		WarmDataTier:     TierHDD,
		ColdDataTier:     TierHDD,
		HotPercent:       20,
		WarmPercent:      50,
		ColdPercent:      30,
		ReadPerformance:  "良好",
		WritePerformance: "良好",
		Reliability:      "99.999%",
	},
	SchemeAllHDD: {
		Scheme:           SchemeAllHDD,
		Name:             "全 HDD 方案",
		Description:      "所有数据存储在 HDD 上，成本最低，性能较差",
		HotDataTier:      TierHDD,
		WarmDataTier:     TierHDD,
		ColdDataTier:     TierHDD,
		HotPercent:       100,
		WarmPercent:      0,
		ColdPercent:      0,
		ReadPerformance:  "一般",
		WritePerformance: "一般",
		Reliability:      "99.99%",
	},
}

// TieringResult 分层对比结果
type TieringResult struct {
	TotalDataBytes  int64           `json:"total_data_bytes"`
	CurrentCost     float64         `json:"current_cost"`
	Schemes         []SchemeCompare `json:"schemes"`
	BestScheme      StorageScheme   `json:"best_scheme"`
	BestSavings     float64         `json:"best_savings"`
	Recommendations []TieringRecommend `json:"recommendations"`
}

// SchemeCompare 方案对比
type SchemeCompare struct {
	Scheme       StorageScheme      `json:"scheme"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	MonthlyCost  float64            `json:"monthly_cost"`
	AnnualCost   float64            `json:"annual_cost"`
	SavingsVsCurrent float64        `json:"savings_vs_current"`
	SavingsPercent   float64        `json:"savings_percent"`
	CostByTier   map[StorageTier]float64 `json:"cost_by_tier"`
	Performance  SchemePerformance  `json:"performance"`
	Allocation   []TierAllocation   `json:"allocation"`
}

// SchemePerformance 方案性能
type SchemePerformance struct {
	ReadSpeed  string `json:"read_speed"`
	WriteSpeed string `json:"write_speed"`
	Latency    string `json:"latency"`
	Reliability string `json:"reliability"`
}

// TierAllocation 层级分配
type TierAllocation struct {
	Tier      StorageTier `json:"tier"`
	Name      string      `json:"name"`
	SizeBytes int64       `json:"size_bytes"`
	Percent   float64     `json:"percent"`
	Cost      float64     `json:"cost"`
}

// TieringRecommend 分层建议
type TieringRecommend struct {
	Priority    string  `json:"priority"`
	Scheme      string  `json:"scheme"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingsPerMonth float64 `json:"savings_per_month"`
	SavingsPerYear  float64 `json:"savings_per_year"`
}

// CompareSchemes 对比存储方案
func (ta *TieringAnalyzer) CompareSchemes() *TieringResult {
	allocs := ta.optimizer.allocations
	profiles := ta.optimizer.profiles

	result := &TieringResult{
		Schemes:         make([]SchemeCompare, 0),
		Recommendations: make([]TieringRecommend, 0),
	}

	if len(allocs) == 0 {
		return result
	}

	// 计算当前成本
	for _, alloc := range allocs {
		result.TotalDataBytes += alloc.UsedBytes
		if profile, ok := profiles[alloc.Tier]; ok {
			result.CurrentCost += bytesToTB(alloc.UsedBytes) * profile.CostPerTBMonth
		}
	}

	// 分析数据热度分布
	hotBytes, warmBytes, coldBytes := ta.analyzeDataHeat(allocs)
	totalBytes := hotBytes + warmBytes + coldBytes
	if totalBytes == 0 {
		totalBytes = result.TotalDataBytes
	}

	// 对比各方案
	for scheme, sp := range DefaultSchemeProfiles {
		compare := ta.evaluateScheme(scheme, sp, hotBytes, warmBytes, coldBytes, totalBytes, profiles)
		compare.SavingsVsCurrent = result.CurrentCost - compare.MonthlyCost
		if result.CurrentCost > 0 {
			compare.SavingsPercent = (compare.SavingsVsCurrent / result.CurrentCost) * 100
		}
		result.Schemes = append(result.Schemes, compare)
	}

	// 按月度成本排序
	sort.Slice(result.Schemes, func(i, j int) bool {
		return result.Schemes[i].MonthlyCost < result.Schemes[j].MonthlyCost
	})

	// 找到最优方案
	if len(result.Schemes) > 0 {
		result.BestScheme = result.Schemes[0].Scheme
		result.BestSavings = result.Schemes[0].SavingsVsCurrent
	}

	// 生成建议
	result.Recommendations = ta.generateTieringRecommendations(result)

	return result
}

// analyzeDataHeat 分析数据热度
func (ta *TieringAnalyzer) analyzeDataHeat(allocs []StorageAllocation) (hotBytes, warmBytes, coldBytes int64) {
	for _, alloc := range allocs {
		switch {
		case alloc.AccessCount > 100:
			hotBytes += alloc.UsedBytes
		case alloc.AccessCount > 10:
			warmBytes += alloc.UsedBytes
		default:
			coldBytes += alloc.UsedBytes
		}
	}
	return
}

// evaluateScheme 评估一个存储方案
func (ta *TieringAnalyzer) evaluateScheme(
	scheme StorageScheme,
	sp SchemeProfile,
	hotBytes, warmBytes, coldBytes, totalBytes int64,
	profiles map[StorageTier]CostProfile,
) SchemeCompare {
	compare := SchemeCompare{
		Scheme:      scheme,
		Name:        sp.Name,
		Description: sp.Description,
		CostByTier:  make(map[StorageTier]float64),
		Allocation:  make([]TierAllocation, 0),
	}

	// 计算各层分配
	allocs := []struct {
		bytes int64
		tier  StorageTier
		label string
	}{
		{hotBytes, sp.HotDataTier, "热数据"},
		{warmBytes, sp.WarmDataTier, "温数据"},
		{coldBytes, sp.ColdDataTier, "冷数据"},
	}

	tierSizeMap := make(map[StorageTier]int64)
	for _, a := range allocs {
		if a.bytes > 0 {
			tierSizeMap[a.tier] += a.bytes
		}
	}

	for tier, size := range tierSizeMap {
		profile, ok := profiles[tier]
		if !ok {
			continue
		}
		cost := bytesToTB(size) * profile.CostPerTBMonth
		compare.MonthlyCost += cost
		compare.CostByTier[tier] += cost
		percent := float64(size) / float64(max(totalBytes, 1)) * 100
		compare.Allocation = append(compare.Allocation, TierAllocation{
			Tier:      tier,
			Name:      profile.Name,
			SizeBytes: size,
			Percent:   percent,
			Cost:      cost,
		})
	}

	compare.AnnualCost = compare.MonthlyCost * 12

	// 性能评估
	ssdProfile := profiles[TierSSD]
	hddProfile := profiles[TierHDD]
	switch scheme {
	case SchemeAllSSD:
		compare.Performance = SchemePerformance{
			ReadSpeed:   fmt.Sprintf("%d MB/s", ssdProfile.ReadSpeedMBps),
			WriteSpeed:  fmt.Sprintf("%d MB/s", ssdProfile.WriteSpeedMBps),
			Latency:     fmt.Sprintf("%.1f ms", ssdProfile.LatencyMs),
			Reliability: ssdProfile.Reliability,
		}
	case SchemeHybrid:
		compare.Performance = SchemePerformance{
			ReadSpeed:   fmt.Sprintf("SSD: %d / HDD: %d MB/s", ssdProfile.ReadSpeedMBps, hddProfile.ReadSpeedMBps),
			WriteSpeed:  fmt.Sprintf("SSD: %d / HDD: %d MB/s", ssdProfile.WriteSpeedMBps, hddProfile.WriteSpeedMBps),
			Latency:     fmt.Sprintf("SSD: %.1f / HDD: %.1f ms", ssdProfile.LatencyMs, hddProfile.LatencyMs),
			Reliability: "99.999%",
		}
	case SchemeAllHDD:
		compare.Performance = SchemePerformance{
			ReadSpeed:   fmt.Sprintf("%d MB/s", hddProfile.ReadSpeedMBps),
			WriteSpeed:  fmt.Sprintf("%d MB/s", hddProfile.WriteSpeedMBps),
			Latency:     fmt.Sprintf("%.1f ms", hddProfile.LatencyMs),
			Reliability: hddProfile.Reliability,
		}
	}

	return compare
}

// generateTieringRecommendations 生成分层建议
func (ta *TieringAnalyzer) generateTieringRecommendations(result *TieringResult) []TieringRecommend {
	var recs []TieringRecommend

	// 找最优方案
	for _, scheme := range result.Schemes {
		if scheme.SavingsVsCurrent > 0 {
			priority := "medium"
			if scheme.SavingsPercent > 30 {
				priority = "high"
			}
			recs = append(recs, TieringRecommend{
				Priority:        priority,
				Scheme:          string(scheme.Scheme),
				Title:           fmt.Sprintf("切换到 %s", scheme.Name),
				Description:     scheme.Description,
				SavingsPerMonth: scheme.SavingsVsCurrent,
				SavingsPerYear:  scheme.SavingsVsCurrent * 12,
			})
		}
	}

	// 按节省金额排序
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].SavingsPerMonth > recs[j].SavingsPerMonth
	})

	return recs
}
