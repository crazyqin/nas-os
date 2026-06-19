// Package storagecostanalyzer 存储成本分析器 - HTTP API 处理器
// 对标群晖存储分析器和 TrueNAS 智能数据服务
package storagecostanalyzer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准 API 响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 存储成本分析 API 处理器.
type Handlers struct {
	manager     *Manager
	capPlanner  *CapacityPlanner
	reportGen   *ReportGenerator
	tcoEngine   *TCOEngine
	budgetMgr   *BudgetManager
	trendEngine *TrendEngine
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager:     manager,
		capPlanner:  NewCapacityPlanner(manager),
		reportGen:   NewReportGenerator(manager),
		tcoEngine:   NewTCOEngine(manager),
		budgetMgr:   NewBudgetManager(manager),
		trendEngine: NewTrendEngine(manager),
	}
}

// RegisterRoutes 注册路由到 /api/v1/storage-cost 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cost := r.Group("/storage-cost")
	{
		// ========== 仪表板 ==========
		cost.GET("/dashboard", h.getDashboard)
		cost.GET("/summary", h.getSummary)

		// ========== 层级管理 ==========
		cost.POST("/tiers", h.registerTier)
		cost.GET("/tiers", h.listTiers)
		cost.GET("/tiers/:tier", h.getTierDetail)

		// ========== 成本记录 ==========
		cost.POST("/records", h.recordCost)
		cost.GET("/records", h.listRecords)

		// ========== 成本分析 ==========
		cost.GET("/analyze", h.analyzeCost)
		cost.GET("/cost-per-tb/:tier", h.getCostPerTB)

		// ========== 容量预测（AI 驱动）==========
		cost.GET("/forecast/capacity", h.forecastCapacity)
		cost.GET("/forecast/capacity/:tier", h.forecastTierCapacity)
		cost.GET("/forecast/cost", h.forecastCost)
		cost.GET("/forecast/enhanced", h.enhancedForecast)

		// ========== 优化建议 ==========
		cost.GET("/optimization", h.getOptimization)
		cost.POST("/optimization/apply/:id", h.applyOptimization)

		// ========== 报告生成 ==========
		cost.POST("/reports/generate", h.generateReport)
		cost.GET("/reports", h.listReports)
		cost.GET("/reports/multi-dimension", h.generateMultiDimensionReport)

		// ========== TCO 分析 ==========
		cost.GET("/tco/:tier", h.calculateTCO)

		// ========== ROI 分析 ==========
		cost.GET("/roi/:tier", h.calculateROI)

		// ========== 存储方案对比 ==========
		cost.POST("/compare", h.compareOptions)

		// ========== 数据优化估算 ==========
		cost.GET("/optimization/estimate/:tier", h.estimateOptimization)

		// ========== 能耗分析 ==========
		cost.GET("/energy/:tier", h.analyzeEnergy)
		cost.GET("/energy/forecast/:tier", h.forecastEnergy)

		// ========== 容量规划 ==========
		cost.GET("/capacity-plan/:tier", h.generateCapacityPlan)
		cost.GET("/capacity-plan/all", h.generateMultiTierPlan)

		// ========== 成本趋势分析 ==========
		cost.GET("/trend", h.getCostTrend)
		cost.GET("/trend/breakdown", h.getCostTrendBreakdown)
		cost.GET("/trend/anomaly", h.detectAnomalies)

		// ========== 预算管理 ==========
		cost.POST("/budget", h.setBudget)
		cost.GET("/budget", h.getBudget)
		cost.GET("/budget/alerts", h.getBudgetAlerts)
		cost.PUT("/budget/:id", h.updateBudget)
		cost.DELETE("/budget/:id", h.deleteBudget)

		// ========== 分析器控制 ==========
		cost.POST("/start", h.startAnalyzer)
		cost.POST("/stop", h.stopAnalyzer)
		cost.GET("/status", h.getStatus)
	}
}

// ========== 仪表板 Handlers ==========

func (h *Handlers) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	// 增加预算告警信息
	if h.budgetMgr != nil {
		budgetAlerts := h.budgetMgr.CheckBudgets()
		dashboard.Alerts = append(dashboard.Alerts, budgetAlerts...)
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: dashboard})
}

func (h *Handlers) getSummary(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	trend := h.trendEngine.AnalyzeTrend(6)

	summary := gin.H{
		"dashboard": dashboard,
		"trend":     trend,
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: summary})
}

// ========== 层级管理 Handlers ==========

func (h *Handlers) registerTier(c *gin.Context) {
	var cfg TierConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	if err := h.manager.RegisterTier(cfg.Tier, cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "层级已注册", Data: cfg})
}

func (h *Handlers) listTiers(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(dashboard.TierStats),
			"tiers": dashboard.TierStats,
		},
	})
}

func (h *Handlers) getTierDetail(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))

	// 获取容量预测
	forecastInput := DefaultCapacityPlanningInput(tier, 12)
	forecast, err := h.capPlanner.ForecastCapacity(forecastInput)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	// 获取容量规划
	planInput := DefaultCapacityPlanningInput(tier, 12)
	plan, err := h.capPlanner.GenerateCapacityPlan(planInput)
	if err != nil {
		plan = nil
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"forecast": forecast,
			"plan":     plan,
		},
	})
}

// ========== 成本记录 Handlers ==========

func (h *Handlers) recordCost(c *gin.Context) {
	var req struct {
		Tier     StorageTier  `json:"tier" binding:"required"`
		Category CostCategory `json:"category" binding:"required"`
		Amount   float64      `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	if err := h.manager.RecordCost(req.Tier, req.Category, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	// 检查预算告警
	alerts := h.budgetMgr.CheckBudgets()

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "成本已记录",
		Data: gin.H{
			"alerts": alerts,
		},
	})
}

func (h *Handlers) listRecords(c *gin.Context) {
	tier := c.Query("tier")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()

	var records []CostRecord
	if tier != "" {
		ts, ok := h.manager.tiers[StorageTier(tier)]
		if ok {
			start := len(ts.records) - limit
			if start < 0 {
				start = 0
			}
			records = ts.records[start:]
		}
	} else {
		start := len(h.manager.records) - limit
		if start < 0 {
			start = 0
		}
		records = h.manager.records[start:]
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(records),
			"records": records,
		},
	})
}

// ========== 成本分析 Handlers ==========

func (h *Handlers) analyzeCost(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")

	report, err := h.manager.GenerateReport(period)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

func (h *Handlers) getCostPerTB(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	cost, err := h.manager.CalculateCostPerTB(tier)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tier":         tier,
			"cost_per_tb":  cost,
			"currency":     h.manager.config.Currency,
		},
	})
}

// ========== 容量预测 Handlers ==========

func (h *Handlers) forecastCapacity(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	trend, err := h.manager.PredictCapacity(months)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: trend})
}

func (h *Handlers) forecastTierCapacity(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	model := c.DefaultQuery("model", "linear")

	input := CapacityPlanningInput{
		Tier:               tier,
		PlanningMonths:     months,
		GrowthModel:        model,
		TargetUtilization:  70.0,
		ExpansionCostPerTB: 500.0,
		IncludeBuffer:      true,
		BufferPercent:      20.0,
	}

	forecast, err := h.capPlanner.ForecastCapacity(input)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: forecast})
}

func (h *Handlers) forecastCost(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	forecast, err := h.manager.ForecastCost(months)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: forecast})
}

func (h *Handlers) enhancedForecast(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	// 增强预测：结合容量和成本
	capacityTrend, capErr := h.manager.PredictCapacity(months)
	costForecast, costErr := h.manager.ForecastCost(months)

	if capErr != nil && costErr != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "数据不足，无法生成预测"})
		return
	}

	// 获取优化建议
	suggestions := h.manager.GetOptimizationSuggestions()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"capacity_forecast": capacityTrend,
			"cost_forecast":     costForecast,
			"optimizations":     suggestions,
			"ai_insights":       generateAIInsights(capacityTrend, costForecast, suggestions),
		},
	})
}

// generateAIInsights 生成 AI 洞察.
func generateAIInsights(capTrend *CapacityTrend, costForecast *EnhancedCostForecast, suggestions []*OptimizationSuggestion) []string {
	var insights []string

	if capTrend != nil {
		if capTrend.GrowthRateTBPerMonth > 0 {
			insights = append(insights, "存储容量以稳定速率增长，建议提前规划扩容")
		}
		if len(capTrend.Suggestions) > 0 {
			insights = append(insights, capTrend.Suggestions...)
		}
	}

	if costForecast != nil {
		if costForecast.CostGrowthRate > 10 {
			insights = append(insights, "成本增长率超过10%，建议评估存储优化策略")
		}
		if costForecast.CostGrowthRate > 5 {
			insights = append(insights, "考虑实施数据分层策略以控制成本增长")
		}
	}

	totalSavings := 0.0
	for _, s := range suggestions {
		totalSavings += s.AnnualSavings
	}
	if totalSavings > 0 {
		insights = append(insights, "通过执行优化建议，年度可节省显著成本")
	}

	if len(insights) == 0 {
		insights = append(insights, "当前存储状态良好，建议保持定期监控")
	}

	return insights
}

// ========== 优化建议 Handlers ==========

func (h *Handlers) getOptimization(c *gin.Context) {
	suggestions := h.manager.GetOptimizationSuggestions()

	totalSavings := 0.0
	for _, s := range suggestions {
		totalSavings += s.AnnualSavings
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":             len(suggestions),
			"total_annual_savings": totalSavings,
			"suggestions":       suggestions,
		},
	})
}

func (h *Handlers) applyOptimization(c *gin.Context) {
	// 标记优化建议已应用（实际执行需要与存储后端集成）
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "优化建议已标记为待执行",
	})
}

// ========== 报告生成 Handlers ==========

func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Period string `json:"period" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	report, err := h.manager.GenerateReport(req.Period)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "报告已生成", Data: report})
}

func (h *Handlers) listReports(c *gin.Context) {
	h.manager.mu.RLock()
	reports := make([]*CostReport, len(h.manager.reports))
	copy(reports, h.manager.reports)
	h.manager.mu.RUnlock()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

func (h *Handlers) generateMultiDimensionReport(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")

	report, err := h.reportGen.GenerateMultiDimensionReport(period)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// ========== TCO 分析 Handlers ==========

func (h *Handlers) calculateTCO(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	monthsStr := c.DefaultQuery("months", "36")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 36
	}

	input := DefaultTCOInput(tier, months)
	result, err := h.tcoEngine.CalculateTCO(input)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// ========== ROI 分析 Handlers ==========

func (h *Handlers) calculateROI(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	investmentStr := c.DefaultQuery("investment", "10000")
	investment, _ := strconv.ParseFloat(investmentStr, 64)
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}
	if investment <= 0 {
		investment = 10000
	}

	result, err := h.manager.CalculateROI(tier, investment, months)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// ========== 存储方案对比 Handlers ==========

func (h *Handlers) compareOptions(c *gin.Context) {
	var req struct {
		RequiredTB float64         `json:"required_tb" binding:"required"`
		Options    []StorageOption `json:"options" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	result, err := h.manager.CompareStorageOptions(req.RequiredTB, req.Options)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// ========== 数据优化估算 Handlers ==========

func (h *Handlers) estimateOptimization(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	dedupStr := c.DefaultQuery("dedup_ratio", "0.15")
	dedupRatio, _ := strconv.ParseFloat(dedupStr, 64)
	compressStr := c.DefaultQuery("compression_ratio", "0.30")
	compressionRatio, _ := strconv.ParseFloat(compressStr, 64)

	result, err := h.manager.EstimateDataOptimization(tier, dedupRatio, compressionRatio)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// ========== 能耗分析 Handlers ==========

func (h *Handlers) analyzeEnergy(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	// 能耗分析需要额外配置，此处返回基本分析
	h.manager.mu.RLock()
	ts, ok := h.manager.tiers[tier]
	h.manager.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: "层级不存在"})
		return
	}

	// 基本能耗估算
	capacityTB := ts.config.CapacityTB
	monthlyPowerCost := capacityTB * 8.0 * 0.8 * 720 / 1000 // 8W/TB, 0.8元/kWh, 720h/月

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tier":              tier,
			"capacity_tb":       capacityTB,
			"estimated_watts":   capacityTB * 8.0,
			"monthly_kwh":       capacityTB * 8.0 * 720 / 1000,
			"monthly_cost":      monthlyPowerCost,
			"annual_cost":       monthlyPowerCost * 12,
		},
	})
}

func (h *Handlers) forecastEnergy(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	h.manager.mu.RLock()
	ts, ok := h.manager.tiers[tier]
	h.manager.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: "层级不存在"})
		return
	}

	// 简化能耗预测
	capacityTB := ts.config.CapacityTB
	growthRate := capacityTB * 0.05 // 假设5%月增长
	totalCost := 0.0
	var forecasts []gin.H

	for i := 1; i <= months; i++ {
		projectedCapacity := capacityTB + growthRate*float64(i)
		monthlyCost := projectedCapacity * 8.0 * 0.8 * 720 / 1000
		totalCost += monthlyCost
		forecasts = append(forecasts, gin.H{
			"month":            i,
			"projected_tb":     projectedCapacity,
			"monthly_cost":     monthlyCost,
			"cumulative_cost":  totalCost,
		})
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"tier":            tier,
			"forecast_months": months,
			"total_cost":      totalCost,
			"forecasts":       forecasts,
		},
	})
}

// ========== 容量规划 Handlers ==========

func (h *Handlers) generateCapacityPlan(c *gin.Context) {
	tier := StorageTier(c.Param("tier"))
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	input := DefaultCapacityPlanningInput(tier, months)
	plan, err := h.capPlanner.GenerateCapacityPlan(input)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: plan})
}

func (h *Handlers) generateMultiTierPlan(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "12")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 12
	}

	targetStr := c.DefaultQuery("target_utilization", "70")
	target, _ := strconv.ParseFloat(targetStr, 64)
	if target <= 0 || target > 100 {
		target = 70.0
	}

	plan, err := h.capPlanner.GenerateMultiTierPlan(months, target)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: plan})
}

// ========== 成本趋势分析 Handlers ==========

func (h *Handlers) getCostTrend(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "6")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 6
	}

	trend := h.trendEngine.AnalyzeTrend(months)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: trend})
}

func (h *Handlers) getCostTrendBreakdown(c *gin.Context) {
	monthsStr := c.DefaultQuery("months", "6")
	months, _ := strconv.Atoi(monthsStr)
	if months <= 0 {
		months = 6
	}

	breakdown := h.trendEngine.AnalyzeTrendByTier(months)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: breakdown})
}

func (h *Handlers) detectAnomalies(c *gin.Context) {
	anomalies := h.trendEngine.DetectAnomalies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(anomalies),
			"anomalies": anomalies,
		},
	})
}

// ========== 预算管理 Handlers ==========

func (h *Handlers) setBudget(c *gin.Context) {
	var req BudgetConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	budget, err := h.budgetMgr.SetBudget(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "预算已设置", Data: budget})
}

func (h *Handlers) getBudget(c *gin.Context) {
	budgets := h.budgetMgr.ListBudgets()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(budgets),
			"budgets":  budgets,
		},
	})
}

func (h *Handlers) getBudgetAlerts(c *gin.Context) {
	alerts := h.budgetMgr.CheckBudgets()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

func (h *Handlers) updateBudget(c *gin.Context) {
	id := c.Param("id")
	var req BudgetConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	budget, err := h.budgetMgr.UpdateBudget(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "预算已更新", Data: budget})
}

func (h *Handlers) deleteBudget(c *gin.Context) {
	id := c.Param("id")
	if err := h.budgetMgr.DeleteBudget(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "预算已删除"})
}

// ========== 分析器控制 Handlers ==========

func (h *Handlers) startAnalyzer(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "分析器已启动"})
}

func (h *Handlers) stopAnalyzer(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "分析器已停止"})
}

func (h *Handlers) getStatus(c *gin.Context) {
	running := h.manager.IsRunning()
	config := h.manager.config

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"running":              running,
			"currency":             config.Currency,
			"forecast_months":      config.ForecastMonths,
			"alert_threshold":      config.AlertThreshold,
			"auto_analyze":         config.AutoAnalyze,
			"analyze_interval":     config.AnalyzeIntervalHours,
			"report_retention_days": config.ReportRetentionDays,
		},
	})
}
