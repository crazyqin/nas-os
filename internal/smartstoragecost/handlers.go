// Package smartstoragecost - HTTP API 处理器
package smartstoragecost

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 智能存储成本分析 HTTP 处理器
type Handlers struct {
	analyzer *Analyzer
}

// NewHandlers 创建处理器
func NewHandlers(analyzer *Analyzer) *Handlers {
	return &Handlers{analyzer: analyzer}
}

// RegisterRoutes 注册路由到 /api/smartstoragecost
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ssc := r.Group("/smartstoragecost")
	{
		ssc.GET("/overview", h.overview)
		ssc.GET("/report", h.report)
		ssc.GET("/forecast", h.forecast)
		ssc.GET("/optimization", h.optimization)
		ssc.POST("/compare", h.compare)
	}
}

// overview GET /api/smartstoragecost/overview - 成本概览
func (h *Handlers) overview(c *gin.Context) {
	tiers := h.analyzer.ListTiers()
	records := h.analyzer.GetCostRecords()

	// 计算当月总成本
	totalCost := 0.0
	totalCap := 0.0
	totalUsed := 0.0
	for _, r := range records {
		totalCost += r.TotalCost
		totalCap += r.CapacityTB
		totalUsed += r.UsedTB
	}

	utilization := 0.0
	if totalCap > 0 {
		utilization = (totalUsed / totalCap) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"tiers":              tiers,
		"total_monthly_cost": totalCost,
		"total_capacity_tb":  totalCap,
		"total_used_tb":      totalUsed,
		"utilization":        utilization,
		"record_count":       len(records),
	})
}

// report GET /api/smartstoragecost/report - 生成成本报告
func (h *Handlers) report(c *gin.Context) {
	label := c.DefaultQuery("label", "当前周期")
	report := h.analyzer.GenerateReport(label)
	c.JSON(http.StatusOK, report)
}

// forecast GET /api/smartstoragecost/forecast - 成本预测
func (h *Handlers) forecast(c *gin.Context) {
	months, _ := strconv.Atoi(c.DefaultQuery("months", "12"))
	model := c.DefaultQuery("model", "linear")

	forecast, err := h.analyzer.GenerateForecast(months, model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, forecast)
}

// optimization GET /api/smartstoragecost/optimization - 优化建议
func (h *Handlers) optimization(c *gin.Context) {
	opt := h.analyzer.GenerateOptimization()
	c.JSON(http.StatusOK, opt)
}

// compare POST /api/smartstoragecost/compare - 多方案对比
func (h *Handlers) compare(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.PeriodMonths <= 0 {
		req.PeriodMonths = 36
	}
	if req.CapacityTB <= 0 {
		req.CapacityTB = 10
	}

	// 构建对比方案
	var results []ScenarioResult
	for _, scenario := range req.Scenarios {
		totalCost := scenario.InitialCost
		capacity := req.CapacityTB

		for m := 1; m <= req.PeriodMonths; m++ {
			totalCost += scenario.MonthlyPerTB * capacity
			capacity *= (1 + req.GrowthRate/100)
		}

		avgMonthly := totalCost / float64(req.PeriodMonths)
		costPerTB := 0.0
		if req.CapacityTB > 0 {
			costPerTB = avgMonthly / req.CapacityTB
		}

		results = append(results, ScenarioResult{
			Name:          scenario.Name,
			TierType:      scenario.TierType,
			TotalCost:     totalCost,
			MonthlyCost:   avgMonthly,
			CostPerTB:     costPerTB,
			FinalCapacity: capacity,
		})
	}

	// 排序找最优
	bestIdx := 0
	for i := 1; i < len(results); i++ {
		if results[i].TotalCost < results[bestIdx].TotalCost {
			bestIdx = i
		}
	}

	// 计算排名和节省
	maxCost := 0.0
	for _, r := range results {
		if r.TotalCost > maxCost {
			maxCost = r.TotalCost
		}
	}
	for i := range results {
		results[i].TotalSavings = maxCost - results[i].TotalCost
		if maxCost > 0 {
			results[i].SavingsPercent = (results[i].TotalSavings / maxCost) * 100
		}
		results[i].Rank = i + 1
	}

	// 简单排名
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].TotalCost < results[i].TotalCost {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	for i := range results {
		results[i].Rank = i + 1
	}

	bestOption := results[0].Name
	bestSavings := results[len(results)-1].TotalCost - results[0].TotalCost

	c.JSON(http.StatusOK, CompareResult{
		GeneratedAt:  time.Now(),
		PeriodMonths: req.PeriodMonths,
		CapacityTB:   req.CapacityTB,
		Results:      results,
		BestOption:   bestOption,
		BestSavings:  bestSavings,
		Analysis:     "基于 " + strconv.Itoa(req.PeriodMonths) + " 个月周期的成本对比分析",
	})
}
