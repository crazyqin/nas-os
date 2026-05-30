// Package storagecost - 存储成本分析 HTTP API 处理器
package storagecost

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 存储成本分析 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/storage-cost 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cost := r.Group("/storage-cost")
	{
		// 存储资产管理
		cost.POST("/assets", h.createAsset)
		cost.GET("/assets", h.listAssets)
		cost.GET("/assets/:id", h.getAsset)

		// TCO分析
		cost.POST("/tco/:assetId", h.calculateTCO)

		// 容量预测
		cost.POST("/capacity-samples", h.recordCapacitySample)
		cost.GET("/capacity-forecast", h.forecastCapacity)

		// 优化建议
		cost.GET("/optimization-report", h.getOptimizationReport)

		// 效率报告
		cost.GET("/efficiency-report", h.getEfficiencyReport)

		// 存储池
		cost.POST("/pools", h.createStoragePool)
		cost.GET("/pools", h.listStoragePools)

		// 预算管理
		cost.POST("/budgets", h.createBudgetPlan)
		cost.GET("/budgets", h.listBudgetPlans)
		cost.GET("/budgets/:id", h.getBudgetPlan)

		// 成本趋势
		cost.POST("/cost-trends", h.addCostTrend)
		cost.GET("/cost-trends", h.getCostTrends)

		// 多维对比
		cost.POST("/compare", h.compareStorageOptions)

		// 配置
		cost.GET("/config/tco", h.getTCOConfig)
		cost.PUT("/config/tco", h.updateTCOConfig)
		cost.GET("/config/forecast", h.getForecastConfig)
		cost.PUT("/config/forecast", h.updateForecastConfig)
		cost.POST("/config/alert-threshold", h.setAlertThreshold)
	}
}

// ========== 存储资产处理器 ==========

func (h *Handlers) createAsset(c *gin.Context) {
	var req StorageAsset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	asset, err := h.manager.CreateAsset(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, asset)
}

func (h *Handlers) listAssets(c *gin.Context) {
	assets := h.manager.ListAssets()
	c.JSON(http.StatusOK, gin.H{
		"assets": assets,
		"total":  len(assets),
	})
}

func (h *Handlers) getAsset(c *gin.Context) {
	id := c.Param("id")
	asset, err := h.manager.GetAsset(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, asset)
}

// ========== TCO分析处理器 ==========

func (h *Handlers) calculateTCO(c *gin.Context) {
	assetID := c.Param("assetId")
	result, err := h.manager.CalculateTCO(assetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ========== 容量预测处理器 ==========

func (h *Handlers) recordCapacitySample(c *gin.Context) {
	var req CapacitySample
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.RecordCapacitySample(req)
	c.JSON(http.StatusCreated, gin.H{"message": "容量采样已记录"})
}

func (h *Handlers) forecastCapacity(c *gin.Context) {
	forecast, err := h.manager.ForecastCapacity()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, forecast)
}

// ========== 优化建议处理器 ==========

func (h *Handlers) getOptimizationReport(c *gin.Context) {
	report := h.manager.GenerateOptimizationReport()
	c.JSON(http.StatusOK, report)
}

// ========== 效率报告处理器 ==========

func (h *Handlers) getEfficiencyReport(c *gin.Context) {
	report := h.manager.GenerateEfficiencyReport()
	c.JSON(http.StatusOK, report)
}

// ========== 存储池处理器 ==========

func (h *Handlers) createStoragePool(c *gin.Context) {
	var req StoragePool
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pool, err := h.manager.CreateStoragePool(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pool)
}

func (h *Handlers) listStoragePools(c *gin.Context) {
	pools := h.manager.ListStoragePools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// ========== 预算管理处理器 ==========

func (h *Handlers) createBudgetPlan(c *gin.Context) {
	var req BudgetPlan
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.manager.CreateBudgetPlan(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, plan)
}

func (h *Handlers) listBudgetPlans(c *gin.Context) {
	plans := h.manager.ListBudgetPlans()
	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
		"total": len(plans),
	})
}

func (h *Handlers) getBudgetPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetBudgetPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plan)
}

// ========== 成本趋势处理器 ==========

func (h *Handlers) addCostTrend(c *gin.Context) {
	var req CostTrend
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.AddCostTrend(req)
	c.JSON(http.StatusCreated, gin.H{"message": "成本趋势已添加"})
}

func (h *Handlers) getCostTrends(c *gin.Context) {
	trends := h.manager.GetCostTrends()
	c.JSON(http.StatusOK, gin.H{
		"trends": trends,
		"total":  len(trends),
	})
}

// ========== 多维对比处理器 ==========

func (h *Handlers) compareStorageOptions(c *gin.Context) {
	var req struct {
		Options []StorageOption `json:"options" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.manager.CompareStorageOptions(req.Options)
	c.JSON(http.StatusOK, result)
}

// ========== 配置处理器 ==========

func (h *Handlers) getTCOConfig(c *gin.Context) {
	config := h.manager.GetTCOConfig()
	c.JSON(http.StatusOK, config)
}

func (h *Handlers) updateTCOConfig(c *gin.Context) {
	var config TCOConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateTCOConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "TCO配置已更新"})
}

func (h *Handlers) getForecastConfig(c *gin.Context) {
	config := h.manager.GetForecastConfig()
	c.JSON(http.StatusOK, config)
}

func (h *Handlers) updateForecastConfig(c *gin.Context) {
	var config ForecastConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateForecastConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "预测配置已更新"})
}

func (h *Handlers) setAlertThreshold(c *gin.Context) {
	var req struct {
		Key   string  `json:"key" binding:"required"`
		Value float64 `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.SetAlertThreshold(req.Key, req.Value)
	c.JSON(http.StatusOK, gin.H{"message": "告警阈值已设置"})
}

// ============================================================
// 成本分析模块新增处理器 (任务要求)
// ============================================================

// RegisterStorageCostRoutes 注册存储成本分析路由
func (h *Handlers) RegisterStorageCostRoutes(r *gin.RouterGroup) {
	cost := r.Group("/storage/cost")
	{
		cost.POST("/records", h.addCostRecord)
		cost.GET("/summary", h.getCostSummary)
		cost.GET("/trend", h.getCostTrend)
		cost.GET("/alerts", h.getCostAlerts)
		cost.GET("/suggestions", h.getOptimizationSuggestions)
		cost.GET("/estimate", h.estimateMonthlyCost)
		cost.POST("/budget-alert", h.setBudgetAlert)
		cost.GET("/export", h.exportCostReport)
	}
}

func (h *Handlers) addCostRecord(c *gin.Context) {
	var req CostRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddCostRecord(req); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "成本记录已添加", "id": req.ID})
}

func (h *Handlers) getCostSummary(c *gin.Context) {
	summary, err := h.manager.GetCostSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handlers) getCostTrend(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days := 30
	if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil || days <= 0 {
		days = 30
	}

	trend, err := h.manager.GetCostTrendByDays(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trend": trend, "days": days})
}

func (h *Handlers) getCostAlerts(c *gin.Context) {
	alerts, err := h.manager.GetCostAlerts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts, "total": len(alerts)})
}

func (h *Handlers) getOptimizationSuggestions(c *gin.Context) {
	suggestions, err := h.manager.GenerateOptimizationSuggestions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions, "total": len(suggestions)})
}

func (h *Handlers) estimateMonthlyCost(c *gin.Context) {
	storageType := c.DefaultQuery("type", "")
	sizeStr := c.DefaultQuery("size", "0")

	if storageType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少存储类型参数"})
		return
	}

	var size float64
	if _, err := fmt.Sscanf(sizeStr, "%f", &size); err != nil || size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的存储大小参数"})
		return
	}

	cost, err := h.manager.EstimateMonthlyCost(storageType, size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"storage_type": storageType,
		"size_gb":      size,
		"monthly_cost": cost,
	})
}

func (h *Handlers) setBudgetAlert(c *gin.Context) {
	var req struct {
		Threshold float64 `json:"threshold" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetBudgetAlert(req.Threshold); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "预算告警已设置", "threshold": req.Threshold})
}

func (h *Handlers) exportCostReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")

	data, err := h.manager.ExportCostReport(format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=cost_report.csv")
	case "json":
		c.Header("Content-Type", "application/json")
	}

	c.Data(http.StatusOK, c.GetHeader("Content-Type"), data)
}
