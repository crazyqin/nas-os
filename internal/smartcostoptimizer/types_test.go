package smartcostoptimizer

import (
	"testing"
	"time"
)

// TestStorageType_Constants 测试存储类型常量定义
func TestStorageType_Constants(t *testing.T) {
	tests := []struct {
		name string
		st   StorageType
		want string
	}{
		{"ssd", StorageTypeSSD, "ssd"},
		{"hdd", StorageTypeHDD, "hdd"},
		{"nvme", StorageTypeNVMe, "nvme"},
		{"tape", StorageTypeTape, "tape"},
		{"cloud", StorageTypeCloud, "cloud"},
		{"unknown", StorageTypeUnknown, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.st) != tt.want {
				t.Errorf("StorageType = %v, want %v", tt.st, tt.want)
			}
		})
	}
}

// TestDataTemperature_Constants 测试数据温度常量定义
func TestDataTemperature_Constants(t *testing.T) {
	tests := []struct {
		name string
		temp DataTemperature
		want string
	}{
		{"hot", TempHot, "hot"},
		{"warm", TempWarm, "warm"},
		{"cold", TempCold, "cold"},
		{"frozen", TempFrozen, "frozen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.temp) != tt.want {
				t.Errorf("DataTemperature = %v, want %v", tt.temp, tt.want)
			}
		})
	}
}

// TestOptimizationStrategy_Constants 测试优化策略常量定义
func TestOptimizationStrategy_Constants(t *testing.T) {
	tests := []struct {
		name     string
		strategy OptimizationStrategy
		want     string
	}{
		{"cold_migration", StrategyColdMigration, "cold_migration"},
		{"deduplication", StrategyDeduplication, "deduplication"},
		{"compression", StrategyCompression, "compression"},
		{"tiering", StrategyTiering, "tiering"},
		{"cleanup", StrategyCleanup, "cleanup"},
		{"archive_policy", StrategyArchivePolicy, "archive_policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.strategy) != tt.want {
				t.Errorf("OptimizationStrategy = %v, want %v", tt.strategy, tt.want)
			}
		})
	}
}

// TestExportFormat_Constants 测试导出格式常量定义
func TestExportFormat_Constants(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   string
	}{
		{"json", ExportJSON, "json"},
		{"csv", ExportCSV, "csv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.want {
				t.Errorf("ExportFormat = %v, want %v", tt.format, tt.want)
			}
		})
	}
}

// TestTrendGranularity_Constants 测试趋势粒度常量定义
func TestTrendGranularity_Constants(t *testing.T) {
	tests := []struct {
		name string
		tg   TrendGranularity
		want string
	}{
		{"daily", TrendDaily, "daily"},
		{"weekly", TrendWeekly, "weekly"},
		{"monthly", TrendMonthly, "monthly"},
		{"yearly", TrendYearly, "yearly"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.tg) != tt.want {
				t.Errorf("TrendGranularity = %v, want %v", tt.tg, tt.want)
			}
		})
	}
}

// TestDefaultSmartCostConfig 测试默认配置
func TestDefaultSmartCostConfig(t *testing.T) {
	cfg := DefaultSmartCostConfig()

	if !cfg.Enabled {
		t.Error("DefaultSmartCostConfig.Enabled should be true")
	}
	if cfg.DefaultCurrency != "CNY" {
		t.Errorf("DefaultSmartCostConfig.DefaultCurrency = %v, want CNY", cfg.DefaultCurrency)
	}
	if cfg.ColdThresholdDays != 90 {
		t.Errorf("DefaultSmartCostConfig.ColdThresholdDays = %v, want 90", cfg.ColdThresholdDays)
	}
	if cfg.UtilizationWarnPct != 20.0 {
		t.Errorf("DefaultSmartCostConfig.UtilizationWarnPct = %v, want 20.0", cfg.UtilizationWarnPct)
	}
	if cfg.DedupRatio != 0.30 {
		t.Errorf("DefaultSmartCostConfig.DedupRatio = %v, want 0.30", cfg.DedupRatio)
	}
	if cfg.CompressRatio != 0.40 {
		t.Errorf("DefaultSmartCostConfig.CompressRatio = %v, want 0.40", cfg.CompressRatio)
	}
	if cfg.ReportRetention != 365 {
		t.Errorf("DefaultSmartCostConfig.ReportRetention = %v, want 365", cfg.ReportRetention)
	}

	// 验证定价规则
	if len(cfg.PricingRules) != 5 {
		t.Errorf("len(PricingRules) = %v, want 5", len(cfg.PricingRules))
	}

	// 验证 SSD 定价
	ssdRule, ok := cfg.PricingRules[StorageTypeSSD]
	if !ok {
		t.Fatal("PricingRules should contain SSD")
	}
	if ssdRule.PricePerGBMonth != 0.50 {
		t.Errorf("SSD PricePerGBMonth = %v, want 0.50", ssdRule.PricePerGBMonth)
	}
}

// TestStorageAsset_Creation 测试存储资产创建
func TestStorageAsset_Creation(t *testing.T) {
	now := time.Now()
	asset := StorageAsset{
		ID:            "asset-001",
		Name:          "NVMe-DataPool",
		Type:          StorageTypeNVMe,
		CapacityBytes: 1024 * 1024 * 1024 * 1000, // 1TB
		UsedBytes:     1024 * 1024 * 1024 * 600,  // 600GB
		PurchaseCost:  2999.99,
		MonthlyOpex:   50.0,
		WarrantyYears: 5,
		PurchaseDate:  now.AddDate(-1, 0, 0),
		Pool:          "pool-main",
		Volume:        "vol-1",
		Provider:      "本地",
		Labels:        []string{"生产", "数据库"},
		CreatedAt:     now,
	}

	if asset.ID != "asset-001" {
		t.Errorf("StorageAsset.ID = %v, want asset-001", asset.ID)
	}
	if asset.Type != StorageTypeNVMe {
		t.Errorf("StorageAsset.Type = %v, want nvme", asset.Type)
	}
	if asset.CapacityBytes != 1024*1024*1024*1000 {
		t.Errorf("StorageAsset.CapacityBytes = %v, want %v", asset.CapacityBytes, 1024*1024*1024*1000)
	}
	if asset.PurchaseCost != 2999.99 {
		t.Errorf("StorageAsset.PurchaseCost = %v, want 2999.99", asset.PurchaseCost)
	}
	if len(asset.Labels) != 2 {
		t.Errorf("len(StorageAsset.Labels) = %v, want 2", len(asset.Labels))
	}
}

// TestPricingRule 测试定价规则
func TestPricingRule(t *testing.T) {
	rule := PricingRule{
		StorageType:     StorageTypeHDD,
		PricePerGBMonth: 0.20,
		TransferPerGB:   0.08,
		RetrievalPerGB:  0.10,
		RequestPer1K:    0.005,
	}

	if rule.StorageType != StorageTypeHDD {
		t.Errorf("PricingRule.StorageType = %v, want hdd", rule.StorageType)
	}
	if rule.PricePerGBMonth != 0.20 {
		t.Errorf("PricingRule.PricePerGBMonth = %v, want 0.20", rule.PricePerGBMonth)
	}
}

// TestCostEntry 测试成本记录
func TestCostEntry(t *testing.T) {
	now := time.Now()
	entry := CostEntry{
		ID:          "entry-001",
		AssetID:     "asset-001",
		AssetName:   "NVMe-DataPool",
		StorageType: StorageTypeNVMe,
		CapacityGB:  1000,
		UsedGB:      600,
		PricePerGB:  0.80,
		TotalCost:   480.0,
		PeriodStart: now.AddDate(0, -1, 0),
		PeriodEnd:   now,
		RecordedAt:  now,
	}

	if entry.ID != "entry-001" {
		t.Errorf("CostEntry.ID = %v, want entry-001", entry.ID)
	}
	if entry.TotalCost != 480.0 {
		t.Errorf("CostEntry.TotalCost = %v, want 480.0", entry.TotalCost)
	}
}

// TestCostSummary 测试成本汇总
func TestCostSummary(t *testing.T) {
	now := time.Now()
	summary := CostSummary{
		TotalCost:       1500.0,
		TotalCapacityGB: 5000.0,
		TotalUsedGB:     3000.0,
		AvgUtilization:  60.0,
		ByType: map[StorageType]float64{
			StorageTypeSSD: 800.0,
			StorageTypeHDD: 700.0,
		},
		ByPool: map[string]float64{
			"pool-main": 900.0,
			"pool-back": 600.0,
		},
		Currency:    "CNY",
		PeriodStart: now.AddDate(0, -1, 0),
		PeriodEnd:   now,
	}

	if summary.TotalCost != 1500.0 {
		t.Errorf("CostSummary.TotalCost = %v, want 1500.0", summary.TotalCost)
	}
	if summary.AvgUtilization != 60.0 {
		t.Errorf("CostSummary.AvgUtilization = %v, want 60.0", summary.AvgUtilization)
	}
	if len(summary.ByType) != 2 {
		t.Errorf("len(ByType) = %v, want 2", len(summary.ByType))
	}
	if len(summary.ByPool) != 2 {
		t.Errorf("len(ByPool) = %v, want 2", len(summary.ByPool))
	}
}

// TestOptimizationSuggestion 测试优化建议
func TestOptimizationSuggestion(t *testing.T) {
	suggestion := OptimizationSuggestion{
		ID:              "opt-001",
		Strategy:        StrategyColdMigration,
		Title:           "冷数据迁移至低成本存储层",
		Description:     "检测到 5 个冷数据资产，建议迁移至 HDD/Tape 层",
		EstimatedSaving: 500.0,
		SavingsPercent:  50.0,
		Currency:        "CNY",
		Priority:        1,
		TargetAssets:    []string{"asset-001", "asset-002"},
		CurrentType:     StorageTypeSSD,
		RecommendedType: StorageTypeHDD,
		Complexity:      "low",
		RiskLevel:       "low",
		CreatedAt:       time.Now(),
	}

	if suggestion.ID != "opt-001" {
		t.Errorf("OptimizationSuggestion.ID = %v, want opt-001", suggestion.ID)
	}
	if suggestion.Strategy != StrategyColdMigration {
		t.Errorf("OptimizationSuggestion.Strategy = %v, want cold_migration", suggestion.Strategy)
	}
	if suggestion.Priority != 1 {
		t.Errorf("OptimizationSuggestion.Priority = %v, want 1", suggestion.Priority)
	}
	if len(suggestion.TargetAssets) != 2 {
		t.Errorf("len(TargetAssets) = %v, want 2", len(suggestion.TargetAssets))
	}
}

// TestROIInput 测试 ROI 输入
func TestROIInput(t *testing.T) {
	input := ROIInput{
		InvestmentCost: 10000.0,
		AnnualSaving:   5000.0,
		AnnualOpex:     1000.0,
		ProjectYears:   3,
		DiscountRate:   0.08,
	}

	if input.InvestmentCost != 10000.0 {
		t.Errorf("ROIInput.InvestmentCost = %v, want 10000.0", input.InvestmentCost)
	}
	if input.ProjectYears != 3 {
		t.Errorf("ROIInput.ProjectYears = %v, want 3", input.ProjectYears)
	}
}

// TestROIResult 测试 ROI 结果
func TestROIResult(t *testing.T) {
	result := ROIResult{
		InvestmentCost: 10000.0,
		TotalSaving:    15000.0,
		TotalOpex:      3000.0,
		NetProfit:      2000.0,
		ROIPercent:     20.0,
		PaybackMonths:  30.0,
		NPV:            1500.0,
		IRR:            12.5,
		AnnualBreakdown: []AnnualROI{
			{Year: 1, Saving: 5000, Opex: 1000, NetCashFlow: 4000, CumulativeCF: -6000},
			{Year: 2, Saving: 5000, Opex: 1000, NetCashFlow: 4000, CumulativeCF: -2000},
			{Year: 3, Saving: 5000, Opex: 1000, NetCashFlow: 4000, CumulativeCF: 2000},
		},
	}

	if result.ROIPercent != 20.0 {
		t.Errorf("ROIResult.ROIPercent = %v, want 20.0", result.ROIPercent)
	}
	if len(result.AnnualBreakdown) != 3 {
		t.Errorf("len(AnnualBreakdown) = %v, want 3", len(result.AnnualBreakdown))
	}
}

// TestColdDataInfo 测试冷数据信息
func TestColdDataInfo(t *testing.T) {
	cold := ColdDataInfo{
		AssetID:       "asset-002",
		AssetName:     "HDD-Archive",
		Volume:        "vol-archive",
		Directory:     "/data/old-backups",
		SizeBytes:     1024 * 1024 * 1024 * 500, // 500GB
		LastAccess:    time.Now().AddDate(0, -6, 0),
		DaysSince:     180,
		CurrentType:   StorageTypeHDD,
		Temperature:   TempCold,
		SuggestedType: StorageTypeTape,
		PotentialSave: 75.0,
	}

	if cold.AssetID != "asset-002" {
		t.Errorf("ColdDataInfo.AssetID = %v, want asset-002", cold.AssetID)
	}
	if cold.Temperature != TempCold {
		t.Errorf("ColdDataInfo.Temperature = %v, want cold", cold.Temperature)
	}
	if cold.SuggestedType != StorageTypeTape {
		t.Errorf("ColdDataInfo.SuggestedType = %v, want tape", cold.SuggestedType)
	}
}

// TestCostReport 测试成本报告
func TestCostReport(t *testing.T) {
	report := CostReport{
		ID:         "report-001",
		ReportName: "2024年Q1存储成本报告",
		Summary: &CostSummary{
			TotalCost: 2000.0,
			Currency:  "CNY",
		},
		Suggestions: []*OptimizationSuggestion{
			{ID: "opt-1", Title: "优化建议1"},
			{ID: "opt-2", Title: "优化建议2"},
		},
		ColdData: []*ColdDataInfo{
			{AssetID: "cd-1", PotentialSave: 100.0},
		},
		GeneratedAt: time.Now(),
		Format:      ExportCSV,
	}

	if report.ID != "report-001" {
		t.Errorf("CostReport.ID = %v, want report-001", report.ID)
	}
	if len(report.Suggestions) != 2 {
		t.Errorf("len(Suggestions) = %v, want 2", len(report.Suggestions))
	}
	if len(report.ColdData) != 1 {
		t.Errorf("len(ColdData) = %v, want 1", len(report.ColdData))
	}
}

// TestTrendPoint 测试趋势数据点
func TestTrendPoint(t *testing.T) {
	now := time.Now()
	point := TrendPoint{
		Date:   now,
		Cost:   500.0,
		UsedGB: 800.0,
		FreeGB: 200.0,
	}

	if point.Cost != 500.0 {
		t.Errorf("TrendPoint.Cost = %v, want 500.0", point.Cost)
	}
	if point.UsedGB+point.FreeGB != 1000.0 {
		t.Error("UsedGB + FreeGB should equal 1000.0")
	}
}

// TestCostTrend 测试成本趋势
func TestCostTrend(t *testing.T) {
	trend := CostTrend{
		Granularity:   TrendMonthly,
		GrowthRate:    0.15,
		ProjectedNext: 575.0,
	}

	if trend.Granularity != TrendMonthly {
		t.Errorf("CostTrend.Granularity = %v, want monthly", trend.Granularity)
	}
	if trend.GrowthRate != 0.15 {
		t.Errorf("CostTrend.GrowthRate = %v, want 0.15", trend.GrowthRate)
	}
}
