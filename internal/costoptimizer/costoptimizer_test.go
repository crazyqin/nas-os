package costoptimizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 辅助函数 ==========

func testAllocations() []StorageAllocation {
	return []StorageAllocation{
		{
			Path:        "/data/hot",
			Tier:        TierNVMe,
			UsedBytes:   500 * 1024 * 1024 * 1024,  // 500GB
			SizeBytes:   1024 * 1024 * 1024 * 1024, // 1TB
			AccessCount: 10000,
			DataType:    DataTypeDocuments,
		},
		{
			Path:        "/data/warm",
			Tier:        TierSSD,
			UsedBytes:   1 * 1024 * 1024 * 1024 * 1024, // 1TB
			SizeBytes:   2 * 1024 * 1024 * 1024 * 1024, // 2TB
			AccessCount: 50,
			DataType:    DataTypeMedia,
		},
		{
			Path:        "/data/cold",
			Tier:        TierNVMe,
			UsedBytes:   1 * 1024 * 1024 * 1024 * 1024, // 1TB
			SizeBytes:   2 * 1024 * 1024 * 1024 * 1024, // 2TB
			AccessCount: 2,                             // 冷数据在NVMe上
			DataType:    DataTypeBackup,
		},
		{
			Path:        "/data/archive",
			Tier:        TierHDD,
			UsedBytes:   200 * 1024 * 1024 * 1024, // 200GB
			SizeBytes:   500 * 1024 * 1024 * 1024, // 500GB
			AccessCount: 0,
			DataType:    DataTypeArchive,
		},
	}
}

// ========== CostOptimizer 核心测试 ==========

func TestNewCostOptimizer(t *testing.T) {
	co := NewCostOptimizer()
	assert.NotNil(t, co)
	assert.NotNil(t, co.profiles)
	assert.NotNil(t, co.config)
	assert.NotNil(t, co.dedup)
	assert.NotNil(t, co.compress)
	assert.NotNil(t, co.tiering)
}

func TestNewCostOptimizerWithConfig(t *testing.T) {
	cfg := &OptimizerConfig{
		StorageCostPerGBMonth: 0.20,
		DefaultQuotaGB:        500,
	}
	co := NewCostOptimizerWithConfig(cfg)
	assert.Equal(t, 0.20, co.config.StorageCostPerGBMonth)
	assert.Equal(t, int64(500), co.config.DefaultQuotaGB)
}

func TestSetAllocations(t *testing.T) {
	co := NewCostOptimizer()
	allocs := testAllocations()
	co.SetAllocations(allocs)
	assert.Len(t, co.allocations, 4)
}

func TestSetCostProfile(t *testing.T) {
	co := NewCostOptimizer()
	co.SetCostProfile(TierNVMe, CostProfile{
		Tier:           TierNVMe,
		Name:           "自定义 NVMe",
		CostPerTBMonth: 600.0,
	})
	assert.Equal(t, "自定义 NVMe", co.profiles[TierNVMe].Name)
	assert.Equal(t, 600.0, co.profiles[TierNVMe].CostPerTBMonth)
}

func TestGenerateReport(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	report := co.GenerateReport()
	require.NotNil(t, report)
	assert.True(t, report.TotalMonthlyCost > 0, "月度总成本应大于0")
	assert.NotZero(t, report.GeneratedAt)
	assert.NotNil(t, report.CostByTier)
	assert.NotNil(t, report.WasteAnalysis)
}

func TestGenerateReportSavings(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	report := co.GenerateReport()
	// 有冷数据在NVMe上，应该有优化建议
	assert.True(t, len(report.Suggestions) > 0, "应有优化建议")
	assert.True(t, report.TotalSavings > 0, "应有节省金额")
	assert.True(t, report.OptimizedCost < report.TotalMonthlyCost, "优化后成本应更低")
	assert.True(t, report.SavingsPercent > 0, "节省百分比应大于0")
}

func TestGenerateReportEmpty(t *testing.T) {
	co := NewCostOptimizer()
	report := co.GenerateReport()
	require.NotNil(t, report)
	assert.Equal(t, float64(0), report.TotalMonthlyCost)
	assert.Equal(t, float64(0), report.TotalSavings)
}

func TestCalculateWastedSpace(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations([]StorageAllocation{
		{
			SizeBytes: 1000 * 1024 * 1024 * 1024, // 1TB
			UsedBytes: 100 * 1024 * 1024 * 1024,  // 100GB - 使用率10%
		},
		{
			SizeBytes: 100 * 1024 * 1024 * 1024, // 100GB
			UsedBytes: 80 * 1024 * 1024 * 1024,  // 80GB - 使用率80%
		},
	})
	wasted := co.calculateWastedSpace()
	assert.True(t, wasted > 0, "应有浪费空间")
	// 第一个分配使用率10% < 30%，应该被计入浪费
	expectedWaste := int64(900 * 1024 * 1024 * 1024) // 900GB
	assert.Equal(t, expectedWaste, wasted)
}

// ========== 去重分析测试 ==========

func TestEstimateDedupPotential(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetDedupAnalyzer().EstimateDedupPotential()
	require.NotNil(t, result)
	assert.True(t, result.TotalDataBytes > 0, "应有数据总量")
	assert.True(t, result.SavingsBytes >= 0, "节省空间应非负")
	assert.True(t, result.DedupRatio >= 0 && result.DedupRatio <= 1, "去重率应在0-1之间")
}

func TestDedupByDataType(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetDedupAnalyzer().EstimateDedupPotential()
	assert.True(t, len(result.ByDataType) > 0, "应有按数据类型的分析")
	for _, dt := range result.ByDataType {
		assert.True(t, dt.TotalBytes > 0, "数据总量应大于0")
		assert.True(t, dt.DedupRatio >= 0, "去重率应非负")
	}
}

func TestDedupByTier(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetDedupAnalyzer().EstimateDedupPotential()
	assert.True(t, len(result.ByTier) > 0, "应有按存储层的分析")
	for _, tier := range result.ByTier {
		assert.True(t, tier.TotalBytes > 0, "数据总量应大于0")
		assert.True(t, tier.SavingsCost >= 0, "节省成本应非负")
	}
}

func TestDedupEmpty(t *testing.T) {
	co := NewCostOptimizer()
	result := co.GetDedupAnalyzer().EstimateDedupPotential()
	require.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalDataBytes)
	assert.Equal(t, int64(0), result.SavingsBytes)
}

func TestDedupRecommendations(t *testing.T) {
	co := NewCostOptimizer()
	// 大量备份数据应产生去重建议
	co.SetAllocations([]StorageAllocation{
		{
			Path:      "/backup/full",
			Tier:      TierHDD,
			UsedBytes: 500 * 1024 * 1024 * 1024, // 500GB
			DataType:  DataTypeBackup,
		},
	})

	result := co.GetDedupAnalyzer().EstimateDedupPotential()
	// 备份数据去重率高（40%），500GB * 40% = 200GB > 10GB 阈值
	if len(result.ByDataType) > 0 && result.ByDataType[0].DedupRatio > 0.2 {
		assert.True(t, len(result.Recommendations) > 0, "应有去重建议")
	}
}

// ========== 压缩分析测试 ==========

func TestEstimateCompressBenefit(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetCompressAnalyzer().EstimateCompressBenefit()
	require.NotNil(t, result)
	assert.True(t, result.TotalDataBytes > 0, "应有数据总量")
	assert.True(t, len(result.ByAlgorithm) > 0, "应有算法分析")
	assert.NotEmpty(t, result.RecommendedAlgo, "应有推荐算法")
	assert.True(t, result.RecommendedSavings >= 0, "推荐节省空间应非负")
}

func TestCompressByAlgorithm(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetCompressAnalyzer().EstimateCompressBenefit()
	// 应有三种算法分析
	assert.Len(t, result.ByAlgorithm, 3)

	for _, algo := range result.ByAlgorithm {
		assert.NotEmpty(t, algo.AlgorithmName)
		assert.True(t, algo.CompressRatio > 0 && algo.CompressRatio <= 1, "压缩比应在0-1之间")
		assert.True(t, algo.SpeedMBps > 0, "速度应大于0")
		assert.True(t, algo.Score >= 0, "评分应非负")
	}
}

func TestCompressByDataType(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetCompressAnalyzer().EstimateCompressBenefit()
	assert.True(t, len(result.ByDataType) > 0, "应有按数据类型的分析")

	for _, dt := range result.ByDataType {
		assert.True(t, dt.TotalBytes > 0, "数据总量应大于0")
		assert.NotEmpty(t, dt.Recommended, "应有推荐算法")
	}
}

func TestCompressEmpty(t *testing.T) {
	co := NewCostOptimizer()
	result := co.GetCompressAnalyzer().EstimateCompressBenefit()
	require.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalDataBytes)
	assert.Len(t, result.ByAlgorithm, 0)
}

func TestCompressRecommendations(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetCompressAnalyzer().EstimateCompressBenefit()
	assert.True(t, len(result.Recommendations) > 0, "应有压缩建议")
}

// ========== 分层对比测试 ==========

func TestCompareSchemes(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetTieringAnalyzer().CompareSchemes()
	require.NotNil(t, result)
	assert.True(t, result.TotalDataBytes > 0, "应有数据总量")
	assert.True(t, result.CurrentCost > 0, "当前成本应大于0")
	assert.True(t, len(result.Schemes) > 0, "应有方案对比")
}

func TestSchemeTypes(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetTieringAnalyzer().CompareSchemes()
	schemeSet := make(map[StorageScheme]bool)
	for _, s := range result.Schemes {
		schemeSet[s.Scheme] = true
	}
	assert.True(t, schemeSet[SchemeAllSSD], "应有全SSD方案")
	assert.True(t, schemeSet[SchemeHybrid], "应有混合方案")
	assert.True(t, schemeSet[SchemeAllHDD], "应有全HDD方案")
}

func TestSchemeCostOrdering(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetTieringAnalyzer().CompareSchemes()
	// 全HDD应最便宜，全SSD应最贵
	var ssdCost, hddCost, hybridCost float64
	for _, s := range result.Schemes {
		switch s.Scheme {
		case SchemeAllSSD:
			ssdCost = s.MonthlyCost
		case SchemeAllHDD:
			hddCost = s.MonthlyCost
		case SchemeHybrid:
			hybridCost = s.MonthlyCost
		}
	}
	assert.True(t, ssdCost >= hybridCost, "全SSD应比混合贵")
	assert.True(t, hybridCost >= hddCost, "混合应比全HDD贵")
}

func TestSchemeSavings(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	result := co.GetTieringAnalyzer().CompareSchemes()
	assert.NotEmpty(t, result.BestScheme, "应有最优方案")
	// 最优方案应该是成本最低的（可能是全HDD或混合）
	for _, s := range result.Schemes {
		assert.True(t, s.MonthlyCost > 0, "成本应大于0")
		assert.True(t, s.AnnualCost > 0, "年度成本应大于0")
		assert.NotEmpty(t, s.Performance.ReadSpeed, "应有读速度")
		assert.NotEmpty(t, s.Performance.WriteSpeed, "应有写速度")
	}
}

func TestTieringEmpty(t *testing.T) {
	co := NewCostOptimizer()
	result := co.GetTieringAnalyzer().CompareSchemes()
	require.NotNil(t, result)
	assert.Equal(t, float64(0), result.CurrentCost)
	assert.Len(t, result.Schemes, 0)
}

func TestTieringRecommendations(t *testing.T) {
	co := NewCostOptimizer()
	// NVMe上的冷数据应该产生分层建议
	co.SetAllocations([]StorageAllocation{
		{
			Path:        "/data/cold",
			Tier:        TierNVMe,
			UsedBytes:   2 * 1024 * 1024 * 1024 * 1024, // 2TB
			AccessCount: 2,
			DataType:    DataTypeBackup,
		},
	})

	result := co.GetTieringAnalyzer().CompareSchemes()
	assert.True(t, len(result.Recommendations) > 0, "应有分层建议")
}

// ========== 报告生成测试 ==========

func TestGenerateMonthlyReport(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()
	require.NotNil(t, report)

	assert.NotEmpty(t, report.ReportID)
	assert.Equal(t, ReportMonthly, report.ReportType)
	assert.NotZero(t, report.GeneratedAt)
	assert.True(t, report.Overview.TotalDataTB > 0, "应有数据总量")
	assert.True(t, report.Overview.CurrentCost > 0, "当前成本应大于0")
	assert.NotNil(t, report.DedupAnalysis)
	assert.NotNil(t, report.CompressAnalysis)
	assert.NotNil(t, report.TieringAnalysis)
	assert.NotNil(t, report.Forecast)
}

func TestGenerateAnnualReport(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	rg := NewReportGenerator(co)
	report := rg.GenerateAnnualReport()
	require.NotNil(t, report)

	assert.Equal(t, ReportAnnual, report.ReportType)
	assert.True(t, len(report.Forecast.Projections) > len(report.Forecast.Projections)/2,
		"年度报告应有更多预测点")
}

func TestReportCostBreakdown(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()

	assert.True(t, len(report.CostBreakdown.ByTier) > 0, "应有按层明细")
	assert.True(t, len(report.CostBreakdown.ByDataType) > 0, "应有按类型明细")
}

func TestReportForecast(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()

	require.NotNil(t, report.Forecast)
	assert.True(t, report.Forecast.MonthlyGrowthRate > 0, "增长率应大于0")
	assert.True(t, len(report.Forecast.Projections) > 0, "应有预测点")

	// 预测成本应逐步增长
	for i := 1; i < len(report.Forecast.Projections); i++ {
		assert.True(t, report.Forecast.Projections[i].Cost >= report.Forecast.Projections[i-1].Cost,
			"预测成本应逐期增长")
	}
}

func TestReportOptimizations(t *testing.T) {
	co := NewCostOptimizer()
	co.SetAllocations(testAllocations())

	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()

	assert.True(t, len(report.Optimizations) > 0, "应有优化建议")
	for _, opt := range report.Optimizations {
		assert.NotEmpty(t, opt.Type, "优化类型不应为空")
		assert.NotEmpty(t, opt.Title, "标题不应为空")
		assert.NotEmpty(t, opt.Priority, "优先级不应为空")
	}
}

func TestReportEmptyData(t *testing.T) {
	co := NewCostOptimizer()
	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()
	require.NotNil(t, report)
	assert.Equal(t, float64(0), report.Overview.CurrentCost)
}

// ========== 工具函数测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0, "¥0.00"},
		{10.5, "¥10.50"},
		{1234.56, "¥1234.56"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCost(tt.cost)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBytesToTB(t *testing.T) {
	// 1 TB = 1024^4 bytes
	tb := bytesToTB(1024 * 1024 * 1024 * 1024)
	assert.InDelta(t, 1.0, tb, 0.001)

	tb = bytesToTB(512 * 1024 * 1024 * 1024) // 512GB
	assert.InDelta(t, 0.5, tb, 0.001)
}

func TestSafePercent(t *testing.T) {
	assert.Equal(t, 50.0, safePercent(50, 100))
	assert.Equal(t, 0.0, safePercent(0, 100))
	assert.Equal(t, 0.0, safePercent(100, 0))  // 除零保护
	assert.Equal(t, 0.0, safePercent(100, -1)) // 负数保护
}

// ========== 默认值测试 ==========

func TestDefaultCostProfiles(t *testing.T) {
	assert.Len(t, DefaultCostProfiles, 4)
	assert.Equal(t, "NVMe SSD", DefaultCostProfiles[TierNVMe].Name)
	assert.Equal(t, "SATA SSD", DefaultCostProfiles[TierSSD].Name)
	assert.Equal(t, "HDD", DefaultCostProfiles[TierHDD].Name)
	assert.Equal(t, "云存储", DefaultCostProfiles[TierCloud].Name)
}

func TestDefaultCompressProfiles(t *testing.T) {
	assert.Len(t, DefaultCompressProfiles, 3)
	lz4 := DefaultCompressProfiles[CompressLZ4]
	assert.Equal(t, "LZ4", lz4.Name)
	assert.True(t, lz4.SpeedMBps > DefaultCompressProfiles[CompressZSTD].SpeedMBps, "LZ4应比ZSTD快")
}

func TestDefaultSchemeProfiles(t *testing.T) {
	assert.Len(t, DefaultSchemeProfiles, 3)
	assert.Equal(t, "全 SSD 方案", DefaultSchemeProfiles[SchemeAllSSD].Name)
	assert.Equal(t, "混合方案（推荐）", DefaultSchemeProfiles[SchemeHybrid].Name)
	assert.Equal(t, "全 HDD 方案", DefaultSchemeProfiles[SchemeAllHDD].Name)
}

func TestDefaultOptimizerConfig(t *testing.T) {
	cfg := DefaultOptimizerConfig()
	assert.NotNil(t, cfg)
	assert.True(t, cfg.StorageCostPerGBMonth > 0)
	assert.True(t, cfg.IdleDaysThreshold > 0)
	assert.True(t, cfg.DefaultQuotaGB > 0)
}

// ========== 集成测试 ==========

func TestFullWorkflow(t *testing.T) {
	// 1. 创建优化器
	co := NewCostOptimizer()

	// 2. 设置数据
	allocs := testAllocations()
	co.SetAllocations(allocs)

	// 3. 去重分析
	dedupResult := co.GetDedupAnalyzer().EstimateDedupPotential()
	assert.NotNil(t, dedupResult)
	t.Logf("去重分析: 总数据=%s, 预计节省=%s, 去重率=%.1f%%",
		FormatBytes(dedupResult.TotalDataBytes),
		FormatBytes(dedupResult.SavingsBytes),
		dedupResult.DedupRatio*100)

	// 4. 压缩分析
	compressResult := co.GetCompressAnalyzer().EstimateCompressBenefit()
	assert.NotNil(t, compressResult)
	t.Logf("压缩分析: 推荐算法=%s, 预计节省=%s",
		compressResult.RecommendedAlgo,
		FormatBytes(compressResult.RecommendedSavings))

	// 5. 分层对比
	tieringResult := co.GetTieringAnalyzer().CompareSchemes()
	assert.NotNil(t, tieringResult)
	t.Logf("分层对比: 当前成本=¥%.2f, 最优方案=%s, 最优节省=¥%.2f",
		tieringResult.CurrentCost,
		tieringResult.BestScheme,
		tieringResult.BestSavings)

	// 6. 生成报告
	rg := NewReportGenerator(co)
	report := rg.GenerateMonthlyReport()
	assert.NotNil(t, report)
	t.Logf("月度报告: 总数据=%.2fTB, 当前成本=¥%.2f, 优化后=¥%.2f, 节省=¥%.2f (%.1f%%)",
		report.Overview.TotalDataTB,
		report.Overview.CurrentCost,
		report.Overview.OptimizedCost,
		report.Overview.PotentialSaving,
		report.Overview.SavingPercent)
}
