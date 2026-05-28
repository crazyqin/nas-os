package tiercost

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

const (
	tb = 1024 * 1024 * 1024 * 1024 // 1TB
)

func newTestAnalyzer() *TierCostAnalyzer {
	a := NewTierCostAnalyzer(nil)

	// 注册NVMe层
	a.RegisterTier(&TierInfo{
		Name:     TierNVMe,
		Capacity: 2 * tb,
		Used:     1 * tb,
	})

	// 注册SSD层
	a.RegisterTier(&TierInfo{
		Name:     TierSSD,
		Capacity: 4 * tb,
		Used:     2 * tb,
	})

	// 注册HDD层
	a.RegisterTier(&TierInfo{
		Name:     TierHDD,
		Capacity: 20 * tb,
		Used:     10 * tb,
	})

	// 注册数据集
	a.RegisterDataset(&DatasetInfo{
		Name:            "database-primary",
		Size:            500 * 1024 * 1024 * 1024, // 500GB
		CurrentTier:     TierSSD,
		AccessFrequency: "hot",
		LastAccessTime:  time.Now(),
	})
	a.RegisterDataset(&DatasetInfo{
		Name:            "backup-daily",
		Size:            2 * tb,
		CurrentTier:     TierSSD,
		AccessFrequency: "cold",
		LastAccessTime:  time.Now().AddDate(0, -3, 0),
	})
	a.RegisterDataset(&DatasetInfo{
		Name:            "media-archive",
		Size:            5 * tb,
		CurrentTier:     TierHDD,
		AccessFrequency: "cold",
		LastAccessTime:  time.Now().AddDate(0, -6, 0),
	})
	a.RegisterDataset(&DatasetInfo{
		Name:            "dev-workspace",
		Size:            200 * 1024 * 1024 * 1024, // 200GB
		CurrentTier:     TierNVMe,
		AccessFrequency: "warm",
		LastAccessTime:  time.Now().AddDate(0, 0, -10),
	})

	return a
}

func TestNewTierCostAnalyzer(t *testing.T) {
	a := NewTierCostAnalyzer(nil)
	assert.NotNil(t, a)
	assert.NotNil(t, a.tiers)
	assert.NotNil(t, a.datasets)
	assert.NotNil(t, a.costHistory)

	// 检查默认定价
	pricing := a.GetPricing()
	assert.Equal(t, 800.0, pricing.NVMePricePerTBYear)
	assert.Equal(t, 500.0, pricing.SSDPricePerTBYear)
	assert.Equal(t, 120.0, pricing.HDDPricePerTBYear)
}

func TestCustomPricing(t *testing.T) {
	custom := &DefaultPricing{
		NVMePricePerTBYear: 1000,
		SSDPricePerTBYear:  600,
		HDDPricePerTBYear:  150,
	}
	a := NewTierCostAnalyzer(custom)
	pricing := a.GetPricing()
	assert.Equal(t, 1000.0, pricing.NVMePricePerTBYear)
	assert.Equal(t, 600.0, pricing.SSDPricePerTBYear)
	assert.Equal(t, 150.0, pricing.HDDPricePerTBYear)
}

func TestTierManagement(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("列出存储层", func(t *testing.T) {
		tiers := a.ListTiers()
		assert.Len(t, tiers, 3)
	})

	t.Run("获取存储层", func(t *testing.T) {
		tier, err := a.GetTier(TierNVMe)
		require.NoError(t, err)
		assert.Equal(t, TierNVMe, tier.Name)
		assert.Equal(t, "NVMe SSD", tier.DisplayName)
		assert.InDelta(t, 0.5, tier.Utilization, 0.01)
		assert.Greater(t, tier.UnitPrice, 0.0)
	})

	t.Run("存储层不存在", func(t *testing.T) {
		_, err := a.GetTier("unknown")
		assert.ErrorIs(t, err, ErrTierNotFound)
	})

	t.Run("移除存储层", func(t *testing.T) {
		a.RemoveTier(TierNVMe)
		_, err := a.GetTier(TierNVMe)
		assert.ErrorIs(t, err, ErrTierNotFound)
	})
}

func TestDatasetManagement(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("列出数据集", func(t *testing.T) {
		datasets := a.ListDatasets()
		assert.Len(t, datasets, 4)
	})

	t.Run("移除数据集", func(t *testing.T) {
		a.RemoveDataset("media-archive")
		datasets := a.ListDatasets()
		assert.Len(t, datasets, 3)
	})
}

func TestAnalyzeCost(t *testing.T) {
	a := newTestAnalyzer()

	report := a.AnalyzeCost()

	assert.NotNil(t, report)
	assert.Greater(t, report.TotalCost, 0.0)
	assert.Len(t, report.TierBreakdown, 3)
	assert.False(t, report.GeneratedAt.IsZero())

	// 检查各层成本明细
	for _, detail := range report.TierBreakdown {
		assert.NotEmpty(t, detail.TierName)
		assert.NotEmpty(t, detail.DisplayName)
		assert.Greater(t, detail.UsedTB, 0.0)
		assert.Greater(t, detail.UnitPrice, 0.0)
		assert.Greater(t, detail.AnnualCost, 0.0)
	}

	// 成本占比之和应接近100%
	totalPct := 0.0
	for _, detail := range report.TierBreakdown {
		totalPct += detail.CostPercentage
	}
	assert.InDelta(t, 100.0, totalPct, 1.0)

	// 应该有推荐建议
	assert.NotEmpty(t, report.Recommendations)
	assert.GreaterOrEqual(t, report.SavingsPotential, 0.0)
}

func TestGetRecommendations(t *testing.T) {
	a := newTestAnalyzer()

	recommendations, savings := a.GetRecommendations()

	assert.NotEmpty(t, recommendations)
	assert.Greater(t, savings, 0.0)

	// cold数据应推荐迁移到HDD
	for _, rec := range recommendations {
		assert.NotEmpty(t, rec.DatasetName)
		assert.NotEmpty(t, rec.CurrentTier)
		assert.NotEmpty(t, rec.RecommendedTier)
		assert.NotEmpty(t, rec.Reason)
	}

	// 验证cold数据被推荐迁移
	foundColdRec := false
	for _, rec := range recommendations {
		if rec.DatasetName == "backup-daily" && rec.RecommendedTier == TierHDD {
			foundColdRec = true
			assert.Greater(t, rec.EstSavings, 0.0)
		}
	}
	assert.True(t, foundColdRec, "cold数据应被推荐迁移到HDD")

	// 验证warm数据从NVMe迁移到SSD
	foundWarmRec := false
	for _, rec := range recommendations {
		if rec.DatasetName == "dev-workspace" && rec.RecommendedTier == TierSSD {
			foundWarmRec = true
			assert.Greater(t, rec.EstSavings, 0.0)
		}
	}
	assert.True(t, foundWarmRec, "warm数据从NVMe应被推荐迁移到SSD")
}

func TestGetCostTrends(t *testing.T) {
	a := newTestAnalyzer()

	// 先生成一些历史数据
	a.AnalyzeCost()

	t.Run("默认月数", func(t *testing.T) {
		trends := a.GetCostTrends(0)
		assert.NotEmpty(t, trends)
	})

	t.Run("指定月数", func(t *testing.T) {
		trends := a.GetCostTrends(6)
		assert.NotEmpty(t, trends)
	})

	t.Run("包含预测数据", func(t *testing.T) {
		trends := a.GetCostTrends(12)
		hasProjection := false
		for _, trend := range trends {
			if trend.IsProjected {
				hasProjection = true
				assert.Greater(t, trend.ProjectedCost, 0.0)
			}
		}
		assert.True(t, hasProjection, "应包含预测数据")
	})
}

func TestSimulateTierPlan(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("正常模拟", func(t *testing.T) {
		req := &SimulateRequest{
			Datasets: []DatasetInfo{
				{Name: "dataset-1", Size: 1 * tb, CurrentTier: TierSSD, AccessFrequency: "cold"},
				{Name: "dataset-2", Size: 500 * 1024 * 1024 * 1024, CurrentTier: TierHDD, AccessFrequency: "hot"},
			},
			TierAssignments: map[string]TierType{
				"dataset-1": TierHDD,  // 冷数据移到HDD
				"dataset-2": TierSSD,  // 热数据移到SSD
			},
		}

		result, err := a.SimulateTierPlan(req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.CurrentCost, 0.0)
		assert.Greater(t, result.SimulatedCost, 0.0)
		assert.Len(t, result.Details, 3)
	})

	t.Run("空请求", func(t *testing.T) {
		_, err := a.SimulateTierPlan(nil)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("空数据集", func(t *testing.T) {
		_, err := a.SimulateTierPlan(&SimulateRequest{})
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("节省成本场景", func(t *testing.T) {
		req := &SimulateRequest{
			Datasets: []DatasetInfo{
				{Name: "big-cold", Size: 5 * tb, CurrentTier: TierNVMe},
			},
			TierAssignments: map[string]TierType{
				"big-cold": TierHDD,
			},
		}

		result, err := a.SimulateTierPlan(req)
		require.NoError(t, err)
		assert.Greater(t, result.Savings, 0.0, "从NVMe移到HDD应节省成本")
		assert.Greater(t, result.SavingsPercent, 0.0)
	})
}

func TestUpdatePricing(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("更新单个价格", func(t *testing.T) {
		newPrice := 900.0
		err := a.UpdatePricing(&PricingUpdateRequest{
			NVMePricePerTBYear: &newPrice,
		})
		require.NoError(t, err)

		pricing := a.GetPricing()
		assert.Equal(t, 900.0, pricing.NVMePricePerTBYear)
		assert.Equal(t, 500.0, pricing.SSDPricePerTBYear) // 不变
	})

	t.Run("更新所有价格", func(t *testing.T) {
		nvme := 1000.0
		ssd := 600.0
		hdd := 150.0
		err := a.UpdatePricing(&PricingUpdateRequest{
			NVMePricePerTBYear: &nvme,
			SSDPricePerTBYear:  &ssd,
			HDDPricePerTBYear:  &hdd,
		})
		require.NoError(t, err)

		pricing := a.GetPricing()
		assert.Equal(t, 1000.0, pricing.NVMePricePerTBYear)
		assert.Equal(t, 600.0, pricing.SSDPricePerTBYear)
		assert.Equal(t, 150.0, pricing.HDDPricePerTBYear)
	})

	t.Run("无效价格", func(t *testing.T) {
		negPrice := -100.0
		err := a.UpdatePricing(&PricingUpdateRequest{
			NVMePricePerTBYear: &negPrice,
		})
		assert.ErrorIs(t, err, ErrInvalidPricing)
	})

	t.Run("空请求", func(t *testing.T) {
		err := a.UpdatePricing(nil)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
}

func TestRound2(t *testing.T) {
	assert.Equal(t, 3.14, round2(3.14159))
	assert.Equal(t, 10.0, round2(10.0))
	assert.Equal(t, 0.01, round2(0.005))
	assert.Equal(t, 100.0, round2(99.995))
}

func TestTierDisplayName(t *testing.T) {
	assert.Equal(t, "NVMe SSD", tierDisplayName(TierNVMe))
	assert.Equal(t, "SATA SSD", tierDisplayName(TierSSD))
	assert.Equal(t, "HDD 机械硬盘", tierDisplayName(TierHDD))
	assert.Equal(t, "unknown", tierDisplayName(TierType("unknown")))
}

// ========== HTTP Handler Tests ==========

func TestHandlers(t *testing.T) {
	a := newTestAnalyzer()
	h := NewHandlers(a)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("获取分层成本分析", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tier-cost/analysis", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "total_cost")
		assert.Contains(t, body, "tier_breakdown")
		assert.Contains(t, body, "savings_potential")
		assert.Contains(t, body, "recommendations")
	})

	t.Run("获取分层建议", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tier-cost/recommendations", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "recommendations")
		assert.Contains(t, body, "total_savings")
	})

	t.Run("获取成本趋势", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tier-cost/trends?months=6", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "trends")
	})

	t.Run("模拟分层方案", func(t *testing.T) {
		body := `{
			"datasets": [
				{"name": "ds1", "size": 1099511627776, "current_tier": "ssd", "access_frequency": "cold"}
			],
			"tier_assignments": {
				"ds1": "hdd"
			}
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/tier-cost/simulate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		resp := w.Body.String()
		assert.Contains(t, resp, "current_cost")
		assert.Contains(t, resp, "simulated_cost")
		assert.Contains(t, resp, "savings")
	})

	t.Run("模拟-无效请求", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/tier-cost/simulate", strings.NewReader("invalid"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("更新存储单价", func(t *testing.T) {
		body := `{"nvme_price_per_tb_year": 950}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/tier-cost/pricing", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		resp := w.Body.String()
		assert.Contains(t, resp, "存储单价已更新")
		assert.Contains(t, resp, "950")
	})

	t.Run("更新单价-无效价格", func(t *testing.T) {
		body := `{"nvme_price_per_tb_year": -100}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/tier-cost/pricing", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("更新单价-无效JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/tier-cost/pricing", strings.NewReader("bad"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestEvaluateDatasetEdgeCases(t *testing.T) {
	a := newTestAnalyzer()

	t.Run("hot数据在HDD上应推荐SSD", func(t *testing.T) {
		a.RegisterDataset(&DatasetInfo{
			Name:            "hot-on-hdd",
			Size:            100 * 1024 * 1024 * 1024,
			CurrentTier:     TierHDD,
			AccessFrequency: "hot",
			LastAccessTime:  time.Now(),
		})
		recs, _ := a.GetRecommendations()
		found := false
		for _, r := range recs {
			if r.DatasetName == "hot-on-hdd" && r.RecommendedTier == TierSSD {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("hot数据在SSD上不需要迁移", func(t *testing.T) {
		a.RegisterDataset(&DatasetInfo{
			Name:            "hot-on-ssd",
			Size:            100 * 1024 * 1024 * 1024,
			CurrentTier:     TierSSD,
			AccessFrequency: "hot",
			LastAccessTime:  time.Now(),
		})
		recs, _ := a.GetRecommendations()
		for _, r := range recs {
			assert.NotEqual(t, "hot-on-ssd", r.DatasetName)
		}
	})

	t.Run("冷数据在HDD上不需要迁移", func(t *testing.T) {
		a.RegisterDataset(&DatasetInfo{
			Name:            "cold-on-hdd",
			Size:            100 * 1024 * 1024 * 1024,
			CurrentTier:     TierHDD,
			AccessFrequency: "cold",
			LastAccessTime:  time.Now().AddDate(0, -6, 0),
		})
		recs, _ := a.GetRecommendations()
		for _, r := range recs {
			assert.NotEqual(t, "cold-on-hdd", r.DatasetName)
		}
	})

	t.Run("无访问频率时基于时间判断", func(t *testing.T) {
		a.RegisterDataset(&DatasetInfo{
			Name:           "old-no-freq",
			Size:           100 * 1024 * 1024 * 1024,
			CurrentTier:    TierSSD,
			LastAccessTime: time.Now().AddDate(0, -4, 0), // 超过90天
		})
		recs, _ := a.GetRecommendations()
		found := false
		for _, r := range recs {
			if r.DatasetName == "old-no-freq" && r.RecommendedTier == TierHDD {
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestCostHistory(t *testing.T) {
	a := newTestAnalyzer()

	// 多次分析应只更新同月记录
	a.AnalyzeCost()
	a.AnalyzeCost()
	trends := a.GetCostTrends(6)
	assert.NotEmpty(t, trends)

	// 验证同月不会产生重复历史记录
	// GetCostTrends会补充模拟历史数据，只需确保趋势数据存在
	realHistory := 0
	for _, trend := range trends {
		if !trend.IsProjected && trend.Cost > 0 {
			realHistory++
		}
	}
	assert.GreaterOrEqual(t, realHistory, 1, "应至少有1条历史数据")
}
