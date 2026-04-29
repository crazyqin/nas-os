package costanalysis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestAnalyzer() *Analyzer {
	a := NewAnalyzer(nil)
	// 注册测试存储池
	a.RegisterPool(&StoragePool{
		ID:                    "pool-ssd-01",
		Name:                  "SSD热数据池",
		TierType:              TierSSD,
		TotalCapacity:         2 * 1024 * 1024 * 1024 * 1024, // 2TB
		UsedCapacity:          1 * 1024 * 1024 * 1024 * 1024, // 1TB
		HardwareCost:          8000,
		AnnualPowerCost:       600,
		AnnualMaintCost:       200,
		ExpectedLifespanYears: 5,
		CreatedAt:             time.Now(),
	})
	a.RegisterPool(&StoragePool{
		ID:                    "pool-hdd-01",
		Name:                  "HDD归档池",
		TierType:              TierHDD,
		TotalCapacity:         10 * 1024 * 1024 * 1024 * 1024, // 10TB
		UsedCapacity:          6 * 1024 * 1024 * 1024 * 1024,  // 6TB
		HardwareCost:          5000,
		AnnualPowerCost:       800,
		AnnualMaintCost:       150,
		ExpectedLifespanYears: 5,
		CreatedAt:             time.Now(),
	})
	// 添加增长历史数据
	baseTime := time.Now().AddDate(0, -6, 0)
	for i := 0; i < 6; i++ {
		usedBytes := int64(float64(1*1024*1024*1024*1024) * (1 + float64(i)*0.05))
		a.AddGrowthData("pool-ssd-01", GrowthDataPoint{
			Timestamp: baseTime.AddDate(0, i, 0),
			UsedBytes: usedBytes,
		})
	}
	return a
}

func TestDefaultAnalysisConfig(t *testing.T) {
	cfg := DefaultAnalysisConfig()
	assert.Greater(t, cfg.CloudStoragePricePerTBMonth, 0.0)
	assert.Greater(t, cfg.DefaultAnalysisPeriodYears, 0.0)
	assert.Greater(t, cfg.AlertUsageThreshold, 0.0)
}

func TestDefaultTierProfiles(t *testing.T) {
	profiles := DefaultTierProfiles()
	assert.Len(t, profiles, 3)
	tiers := make(map[StorageTierType]bool)
	for _, p := range profiles {
		tiers[p.TierType] = true
		assert.Greater(t, p.AvgCostPerTBYear, 0.0)
		assert.NotEmpty(t, p.RecommendedUse)
	}
	assert.True(t, tiers[TierNVMe])
	assert.True(t, tiers[TierSSD])
	assert.True(t, tiers[TierHDD])
}

func TestCalculateCostPerTB(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("正常计算", func(t *testing.T) {
		result, err := a.CalculateCostPerTB("pool-ssd-01")
		require.NoError(t, err)
		assert.Equal(t, "pool-ssd-01", result.PoolID)
		assert.Equal(t, TierSSD, result.TierType)
		assert.InDelta(t, 2.0, result.TotalCapacityTB, 0.01)
		assert.InDelta(t, 1.0, result.UsedCapacityTB, 0.01)
		assert.Greater(t, result.HardwareCostPerTB, 0.0)
		assert.Greater(t, result.TotalAnnualCostPerTB, 0.0)
		assert.InDelta(t, result.TotalAnnualCostPerTB/12, result.MonthlyCostPerTB, 0.01)
	})

	t.Run("池不存在", func(t *testing.T) {
		_, err := a.CalculateCostPerTB("nonexistent")
		assert.ErrorIs(t, err, ErrPoolNotFound)
	})
}

func TestCompareTiers(t *testing.T) {
	a := newTestAnalyzer()
	result := a.CompareTiers()

	assert.Len(t, result.Tiers, 3)
	assert.NotEmpty(t, result.BestValueTier)
	assert.NotEmpty(t, result.AnalysisNote)
	assert.False(t, result.ComparedAt.IsZero())

	// 确保HDD通常是最便宜的
	for _, tier := range result.Tiers {
		if tier.TierType == TierHDD {
			assert.Less(t, tier.AvgCostPerTBYear, 500.0)
		}
	}
}

func TestGenerateCapacityPlan(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("正常规划", func(t *testing.T) {
		plan, err := a.GenerateCapacityPlan("pool-ssd-01", 6)
		require.NoError(t, err)
		assert.Equal(t, "pool-ssd-01", plan.PoolID)
		assert.Len(t, plan.Predictions, 6)
		assert.NotEmpty(t, plan.UrgencyLevel)
		assert.False(t, plan.PlannedAt.IsZero())
	})

	t.Run("池不存在", func(t *testing.T) {
		_, err := a.GenerateCapacityPlan("nonexistent", 12)
		assert.ErrorIs(t, err, ErrPoolNotFound)
	})
}

func TestAnalyzeROI(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("SSD池ROI", func(t *testing.T) {
		result, err := a.AnalyzeROI("pool-ssd-01", 3)
		require.NoError(t, err)
		assert.Greater(t, result.LocalStorage.TotalPerYear, 0.0)
		assert.Greater(t, result.CloudStorage.TotalPerYear, 0.0)
		assert.NotEmpty(t, result.RecommendedOption)
		assert.Len(t, result.Assumptions, 5)
	})

	t.Run("默认周期", func(t *testing.T) {
		result, err := a.AnalyzeROI("pool-ssd-01", 0)
		require.NoError(t, err)
		assert.Equal(t, float64(3), result.AnalysisPeriodYears) // 默认3年
	})

	t.Run("池不存在", func(t *testing.T) {
		_, err := a.AnalyzeROI("nonexistent", 3)
		assert.ErrorIs(t, err, ErrPoolNotFound)
	})
}

func TestGenerateOptimizationReport(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("正常生成", func(t *testing.T) {
		report, err := a.GenerateOptimizationReport("pool-ssd-01")
		require.NoError(t, err)
		assert.NotEmpty(t, report.Suggestions)
		assert.GreaterOrEqual(t, report.TotalPotentialSaving, 0.0)
		assert.False(t, report.GeneratedAt.IsZero())

		// 检查建议都有必要字段
		for _, s := range report.Suggestions {
			assert.NotEmpty(t, s.ID)
			assert.NotEmpty(t, s.Category)
			assert.NotEmpty(t, s.Title)
			assert.GreaterOrEqual(t, s.Priority, 1)
			assert.LessOrEqual(t, s.Priority, 3)
		}
	})

	t.Run("池不存在", func(t *testing.T) {
		_, err := a.GenerateOptimizationReport("nonexistent")
		assert.ErrorIs(t, err, ErrPoolNotFound)
	})
}

func TestPoolLifecycle(t *testing.T) {
	a := NewAnalyzer(nil)

	// 注册
	pool := &StoragePool{
		ID:                    "test-pool",
		Name:                  "测试池",
		TierType:              TierHDD,
		TotalCapacity:         1024 * 1024 * 1024 * 1024,
		UsedCapacity:          512 * 1024 * 1024 * 1024,
		HardwareCost:          3000,
		AnnualPowerCost:       300,
		AnnualMaintCost:       100,
		ExpectedLifespanYears: 5,
	}
	a.RegisterPool(pool)

	// 列出
	pools := a.ListPools()
	assert.Len(t, pools, 1)

	// 获取
	got, err := a.GetPool("test-pool")
	require.NoError(t, err)
	assert.Equal(t, "测试池", got.Name)

	// 添加增长数据
	a.AddGrowthData("test-pool", GrowthDataPoint{
		Timestamp: time.Now(),
		UsedBytes: 512 * 1024 * 1024 * 1024,
	})

	// 移除
	a.RemovePool("test-pool")
	_, err = a.GetPool("test-pool")
	assert.ErrorIs(t, err, ErrPoolNotFound)
	pools = a.ListPools()
	assert.Len(t, pools, 0)
}

func TestRound2(t *testing.T) {
	assert.Equal(t, 3.14, round2(3.14159))
	assert.Equal(t, 10.0, round2(10.0))
	assert.Equal(t, 0.01, round2(0.005))
}

func TestHandlers(t *testing.T) {
	a := newTestAnalyzer()
	h := NewHandlers(a)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("列出存储池", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "pool-ssd-01")
	})

	t.Run("注册存储池", func(t *testing.T) {
		body := `{
			"id": "pool-new",
			"name": "新池",
			"tier_type": "nvme",
			"total_capacity": 1099511627776,
			"used_capacity": 549755813888,
			"hardware_cost": 12000,
			"annual_power_cost": 400,
			"annual_maint_cost": 200,
			"expected_lifespan_years": 5
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/cost-analysis/pools", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("获取每TB成本", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools/pool-ssd-01/cost-per-tb", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "cost_per_tb")
	})

	t.Run("获取层级对比", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/tier-comparison", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "tiers")
	})

	t.Run("获取容量规划", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools/pool-ssd-01/capacity-plan?months=6", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "predictions")
	})

	t.Run("获取ROI分析", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools/pool-ssd-01/roi?years=3", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "local_storage")
	})

	t.Run("获取优化建议", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools/pool-ssd-01/optimization", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "suggestions")
	})

	t.Run("添加增长数据", func(t *testing.T) {
		body := `{"timestamp":"2026-04-01T00:00:00Z","used_bytes":1073741824000}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/cost-analysis/pools/pool-ssd-01/growth-data", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("池不存在", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/cost-analysis/pools/nonexistent/cost-per-tb", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCalculateGrowthRate(t *testing.T) {
	t.Run("正常增长", func(t *testing.T) {
		base := time.Now().AddDate(0, -3, 0)
		history := []GrowthDataPoint{
			{Timestamp: base, UsedBytes: 100 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 1, 0), UsedBytes: 110 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 2, 0), UsedBytes: 120 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 3, 0), UsedBytes: 130 * 1024 * 1024 * 1024},
		}
		gb, pct := calculateGrowthRate(history)
		assert.Greater(t, gb, 0.0)
		assert.Greater(t, pct, 0.0)
	})

	t.Run("数据不足", func(t *testing.T) {
		history := []GrowthDataPoint{
			{Timestamp: time.Now(), UsedBytes: 100 * 1024 * 1024 * 1024},
		}
		gb, pct := calculateGrowthRate(history)
		assert.Equal(t, 0.0, gb)
		assert.Equal(t, 0.0, pct)
	})
}
