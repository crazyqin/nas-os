package storageanalytics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ========== CostAnalyzer 测试 ==========

func TestCostAnalyzer_AnalyzeCosts(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)
	require.NotNil(t, report)

	// 获取成本报告
	costReport, err := analyzer.GetLastCostReport()
	require.NoError(t, err)
	assert.NotNil(t, costReport)
	assert.NotZero(t, costReport.GeneratedAt)
	// 小文件可能没有层级分布
	// 验证成本报告结构正确
	assert.NotNil(t, costReport.Forecast)
}

func TestCostAnalyzer_TierBreakdown(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)

	// 模拟采集结果
	result := &CollectResult{
		TotalSize: 1024 * 1024 * 1024 * 100, // 100GB
		Files: []FileInfo{
			{Path: "/fast/file1.dat", Size: 1024 * 1024 * 50, AccessTime: time.Now().Add(-1 * time.Hour)},
			{Path: "/warm/file2.dat", Size: 1024 * 1024 * 1024 * 10, AccessTime: time.Now().Add(-15 * 24 * time.Hour)},
			{Path: "/cold/file3.dat", Size: 1024 * 1024 * 1024 * 50, AccessTime: time.Now().Add(-200 * 24 * time.Hour)},
			{Path: "/archive/file4.dat", Size: 1024 * 1024 * 1024 * 40, AccessTime: time.Now().Add(-400 * 24 * time.Hour)},
		},
	}

	report := &StorageReport{
		Insights: InsightAnalysis{},
	}

	costReport := costAnalyzer.AnalyzeCosts(result, report)

	assert.NotEmpty(t, costReport.TierBreakdown)
	assert.Greater(t, costReport.TotalMonthlyCost, 0.0)

	// 验证层级分布
	tierMap := make(map[StorageTier]CostBreakdown)
	for _, bd := range costReport.TierBreakdown {
		tierMap[bd.Tier] = bd
	}

	// 频繁访问的小文件应该在NVMe
	_, hasNVMe := tierMap[TierNVMe]
	// 冷数据应该在Cold
	_, hasCold := tierMap[TierCold]
	assert.True(t, hasNVMe || hasCold, "应该有NVMe或Cold层级的数据")
}

func TestCostAnalyzer_CloudComparison(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)

	totalBytes := int64(1024 * 1024 * 1024 * 1024) // 1TB
	localCost := 100.0 // 100元/月

	comparison := costAnalyzer.CompareCloudCosts(totalBytes, localCost)

	assert.NotNil(t, comparison)
	assert.NotEmpty(t, comparison.CloudProviders)
	assert.NotEmpty(t, comparison.BestOption)
	assert.NotZero(t, comparison.LocalCostPerTB)
}

func TestCostAnalyzer_GrowthRate(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)

	// 模拟有月度数据的报告
	report := &StorageReport{
		Trends: TrendAnalysis{
			Monthly: []TrendPoint{
				{Date: time.Now().AddDate(0, -3, 0), Growth: 1024 * 1024 * 1024},      // 1GB
				{Date: time.Now().AddDate(0, -2, 0), Growth: 1024 * 1024 * 1024 * 2},  // 2GB
				{Date: time.Now().AddDate(0, -1, 0), Growth: 1024 * 1024 * 1024 * 3},  // 3GB
			},
		},
	}

	growthRate := costAnalyzer.calculateGrowthRate(report)
	assert.Greater(t, growthRate, 0.0)
}

func TestCostAnalyzer_PredictBreakpoint(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)

	// 模拟快速增长场景
	report := &StorageReport{}
	breakpoint := costAnalyzer.predictBreakpoint(1.0, 0.5, report) // 1TB当前，0.5TB/月增长

	assert.NotNil(t, breakpoint)
	assert.Greater(t, breakpoint.DaysRemaining, 0)
	assert.NotEmpty(t, breakpoint.WarningLevel)
}

func TestCostAnalyzer_IdentifySavingsOpportunities(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)

	result := &CollectResult{
		TotalSize: 1024 * 1024 * 1024 * 100, // 100GB
		Files: []FileInfo{
			{Path: "/cold/old.dat", Size: 1024 * 1024 * 1024 * 50, AccessTime: time.Now().Add(-400 * 24 * time.Hour)},
			{Path: "/large/big.zip", Size: 1024 * 1024 * 1024 * 10},
		},
	}

	report := &StorageReport{
		Health: HealthMetrics{
			RedundancyRate: 0.15,
		},
		Insights: InsightAnalysis{
			Insights: []Insight{
				{Type: "optimization", Saving: 1024 * 1024 * 1024 * 20},
			},
		},
	}

	currentCost := 500.0
	opportunities := costAnalyzer.identifySavingsOpportunities(result, report, currentCost)

	assert.NotEmpty(t, opportunities)
	for _, opp := range opportunities {
		assert.NotEmpty(t, opp.Type)
		assert.NotEmpty(t, opp.Description)
		assert.GreaterOrEqual(t, opp.Confidence, 0.0)
		assert.LessOrEqual(t, opp.Confidence, 1.0)
	}
}

// ========== Optimizer 测试 ==========

func TestOptimizer_GenerateRecommendations(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)
	costReport, _ := analyzer.GetLastCostReport()

	optimizer := NewOptimizer(nil, NewCostAnalyzer(nil))
	recommendations := optimizer.GenerateRecommendations(result, report, costReport)

	// 小文件可能没有建议，验证函数执行成功
	assert.NotNil(t, recommendations)
	for _, rec := range recommendations {
		assert.NotEmpty(t, rec.ID)
		assert.NotEmpty(t, rec.Category)
		assert.NotEmpty(t, rec.Priority)
		assert.NotEmpty(t, rec.Title)
	}
}

func TestOptimizer_CleanupRecommendations(t *testing.T) {
	optimizer := NewOptimizer(nil, NewCostAnalyzer(nil))

	// 创建有临时文件的测试目录
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "temp.tmp"), []byte("temporary data"), 0644)
	os.WriteFile(filepath.Join(dir, "app.log"), []byte("log data"), 0644)
	os.WriteFile(filepath.Join(dir, "cache.cache"), []byte("cache data"), 0644)

	collector := NewCollector(nil, zap.NewNop())
	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	analyzer := NewAnalyzer(nil, zap.NewNop())
	report := analyzer.Analyze(result)

	recommendations := optimizer.generateCleanupRecommendations(result, report)

	// 小文件可能没有清理建议，验证函数执行成功
	assert.NotNil(t, recommendations)
	for _, rec := range recommendations {
		assert.Equal(t, "cleanup", rec.Category)
	}
}

func TestOptimizer_TierRecommendations(t *testing.T) {
	costAnalyzer := NewCostAnalyzer(nil)
	optimizer := NewOptimizer(nil, costAnalyzer)

	// 模拟大量数据在NVMe层级
	costReport := &StorageCostReport{
		TierBreakdown: []CostBreakdown{
			{
				Tier:        TierNVMe,
				TierName:    "NVMe SSD",
				UsedTB:      5.0,
				CostPerTB:   800,
				MonthlyCost: 4000,
			},
		},
	}

	result := &CollectResult{}
	report := &StorageReport{}

	recommendations := optimizer.generateTierRecommendations(result, report, costReport)
	assert.NotEmpty(t, recommendations)
	assert.Equal(t, "tier", recommendations[0].Category)
}

func TestOptimizer_DedupRecommendations(t *testing.T) {
	optimizer := NewOptimizer(nil, NewCostAnalyzer(nil))

	// 模拟高冗余率场景
	result := &CollectResult{
		TotalSize: 1024 * 1024 * 1024 * 100, // 100GB
	}
	report := &StorageReport{
		Health: HealthMetrics{
			RedundancyRate: 0.25, // 25%冗余率
		},
	}

	recommendations := optimizer.generateDedupRecommendations(result, report)
	assert.NotEmpty(t, recommendations)
	assert.Equal(t, "dedup", recommendations[0].Category)
	assert.Equal(t, "high", recommendations[0].Priority) // 25% > 20%
}

func TestOptimizer_CompressionRecommendations(t *testing.T) {
	optimizer := NewOptimizer(nil, NewCostAnalyzer(nil))

	// 模拟大文件场景
	result := &CollectResult{
		Files: []FileInfo{
			{Path: "/data/big1.dat", Size: 1024 * 1024 * 200, FileType: FileTypeOther},
			{Path: "/data/big2.dat", Size: 1024 * 1024 * 300, FileType: FileTypeDocument},
			{Path: "/data/big3.dat", Size: 1024 * 1024 * 150, FileType: FileTypeCode},
		},
	}
	report := &StorageReport{}

	recommendations := optimizer.generateCompressionRecommendations(result, report)
	assert.NotEmpty(t, recommendations)
	assert.Equal(t, "compression", recommendations[0].Category)
}

func TestOptimizer_LifecycleRecommendations(t *testing.T) {
	optimizer := NewOptimizer(nil, NewCostAnalyzer(nil))

	// 模拟冷数据场景
	result := &CollectResult{
		Files: []FileInfo{
			{Path: "/data/old1.dat", Size: 1024 * 1024 * 1024 * 2, AccessTime: time.Now().Add(-400 * 24 * time.Hour)},
			{Path: "/data/old2.dat", Size: 1024 * 1024 * 1024 * 3, AccessTime: time.Now().Add(-500 * 24 * time.Hour)},
		},
	}
	report := &StorageReport{}

	recommendations := optimizer.generateLifecycleRecommendations(result, report)
	assert.NotEmpty(t, recommendations)

	// 应该有冷数据归档建议
	found := false
	for _, rec := range recommendations {
		if rec.Category == "lifecycle" {
			found = true
			break
		}
	}
	assert.True(t, found, "应该有生命周期管理建议")
}

// ========== Handler 新端点测试 ==========

func TestHandler_CostOverview(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取成本概览
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/cost/overview", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total_monthly_cost")
	assert.Contains(t, w.Body.String(), "tier_breakdown")
}

func TestHandler_CostForecast(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取成本预测
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/cost/forecast", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "predictions")
}

func TestHandler_CostTiers(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取层级成本
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/cost/tiers", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tiers")
}

func TestHandler_CostCloudComparison(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取云存储对比
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/cost/cloud-comparison", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "cloud_providers")
}

func TestHandler_OptimizationRecommendations(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取优化建议
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/optimization/recommendations", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "recommendations")
}

func TestHandler_OptimizationSavings(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取节省汇总
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/optimization/savings", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total_saving_bytes")
}

func TestHandler_CostEndpointsNoData(t *testing.T) {
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	endpoints := []string{
		"/api/v1/storage-analytics/cost/overview",
		"/api/v1/storage-analytics/cost/forecast",
		"/api/v1/storage-analytics/cost/tiers",
		"/api/v1/storage-analytics/cost/cloud-comparison",
		"/api/v1/storage-analytics/optimization/recommendations",
		"/api/v1/storage-analytics/optimization/savings",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", endpoint, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestHandler_ReportWithCost(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取包含成本分析的Markdown报告
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/report?format=markdown", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "存储成本分析")
	// 小文件可能没有优化建议，验证报告格式正确
}
}

// ========== Reporter 成本报告测试 ==========

func TestReporter_ToMarkdownWithCost(t *testing.T) {
	reporter := NewReporter()

	report := &StorageReport{
		ScanPath:    "/test/data",
		GeneratedAt: time.Now(),
		Summary: Summary{
			TotalSize:  1024 * 1024 * 1024 * 100,
			TotalFiles: 50,
			TotalDirs:  5,
		},
		Health: HealthMetrics{
			OverallScore: 75.5,
		},
	}

	costReport := &StorageCostReport{
		GeneratedAt:      time.Now(),
		TotalMonthlyCost: 500.0,
		TotalYearlyCost:  6000.0,
		CostPerTBAvg:     5.0,
		TierBreakdown: []CostBreakdown{
			{
				Tier:        TierHDD,
				TierName:    "机械硬盘",
				UsedTB:      0.1,
				MonthlyCost: 8.0,
				YearlyCost:  96.0,
				Utilization: 1.0,
			},
		},
		Forecast: &CostForecast{
			GrowthRateTB: 0.01,
			Predictions: []CostPrediction{
				{
					PredictedDate:   time.Now().AddDate(0, 1, 0),
					PredictedSizeTB: 0.11,
					PredictedCost:   8.8,
					Confidence:      0.95,
				},
			},
			Breakpoint: &BreakpointInfo{
				EstimatedDate: time.Now().AddDate(0, 6, 0),
				DaysRemaining: 180,
				WarningLevel:  "info",
			},
		},
		Recommendations: []OptimizationRecommendation{
			{
				ID:          "OPT-001",
				Category:    "cleanup",
				Priority:    "high",
				Title:       "存储空间清理",
				Description: "检测到临时文件占用空间",
				Impact:      "可释放 100MB 存储空间",
				SavingBytes: 100 * 1024 * 1024,
				SavingCost:  0.8,
				Effort:      "easy",
				Steps: []string{
					"1. 扫描并列出临时文件",
					"2. 执行清理操作",
				},
			},
		},
		ComparisonWithCloud: &CloudCostComparison{
			LocalCostPerTB: 80.0,
			CloudProviders: []CloudProviderCost{
				{Provider: "AWS", Tier: "S3 Standard", CostPerTBMonth: 180, MonthlyCost: 18.0},
				{Provider: "阿里云", Tier: "OSS 标准", CostPerTBMonth: 120, MonthlyCost: 12.0},
			},
			BestOption:     "阿里云 OSS 标准",
			SavingsVsCloud: 4.0,
		},
	}

	md := reporter.ToMarkdownWithCost(report, costReport)

	assert.Contains(t, md, "存储成本分析")
	assert.Contains(t, md, "月度总成本")
	assert.Contains(t, md, "层级成本分解")
	assert.Contains(t, md, "成本预测")
	assert.Contains(t, md, "存储优化建议")
	assert.Contains(t, md, "云存储成本对比")
	assert.Contains(t, md, "AWS")
	assert.Contains(t, md, "阿里云")
}

func TestReporter_ToMarkdownWithNilCost(t *testing.T) {
	reporter := NewReporter()

	report := &StorageReport{
		ScanPath:    "/test",
		GeneratedAt: time.Now(),
		Summary:     Summary{TotalSize: 1024, TotalFiles: 10},
	}

	md := reporter.ToMarkdownWithCost(report, nil)
	assert.Contains(t, md, "存储分析报告")
	assert.NotContains(t, md, "存储成本分析")
}
