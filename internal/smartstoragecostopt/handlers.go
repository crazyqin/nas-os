// Package smartstoragecostopt 提供智能存储成本优化的HTTP处理器
package smartstoragecostopt

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能存储成本优化HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	costGroup := api.Group("/storage-cost")
	{
		// 存储池管理
		costGroup.POST("/pools", h.createPool)
		costGroup.GET("/pools", h.listPools)
		costGroup.GET("/pools/:id", h.getPool)
		costGroup.PUT("/pools/:id/usage", h.updatePoolUsage)

		// 成本分析
		costGroup.GET("/total", h.getTotalCost)
		costGroup.GET("/breakdown", h.getCostBreakdown)
		costGroup.GET("/forecast/:poolId", h.forecastCost)
		costGroup.GET("/trend", h.getCostTrend)
		costGroup.GET("/compare", h.compareTiers)

		// 成本记录
		costGroup.POST("/records", h.recordCost)
		costGroup.GET("/records", h.getCostHistory)

		// 优化建议
		costGroup.POST("/suggestions/generate", h.generateSuggestions)
		costGroup.GET("/suggestions", h.getSuggestions)

		// ROI分析
		costGroup.POST("/roi", h.calculateROI)

		// 概览
		costGroup.GET("/overview", h.getOverview)
	}
}

// createPool 创建存储池
func (h *Handlers) createPool(c *gin.Context) {
	var pool StoragePool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.CreatePool(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "存储池创建成功",
		"pool":    pool,
	})
}

// listPools 列出存储池
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// getPool 获取存储池
func (h *Handlers) getPool(c *gin.Context) {
	id := c.Param("id")
	pool, err := h.manager.GetPool(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// updatePoolUsage 更新使用量
func (h *Handlers) updatePoolUsage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		UsedGB float64 `json:"used_gb" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdatePoolUsage(id, req.UsedGB); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "使用量更新成功"})
}

// getTotalCost 获取总成本
func (h *Handlers) getTotalCost(c *gin.Context) {
	total := h.manager.CalculateTotalCost()
	c.JSON(http.StatusOK, gin.H{
		"total_cost_usd": total,
		"period":         "monthly",
	})
}

// getCostBreakdown 获取成本分解
func (h *Handlers) getCostBreakdown(c *gin.Context) {
	breakdown := h.manager.GetCostBreakdown()
	c.JSON(http.StatusOK, gin.H{
		"breakdown": breakdown,
	})
}

// forecastCost 预测成本
func (h *Handlers) forecastCost(c *gin.Context) {
	poolID := c.Param("poolId")
	months := 12
	forecast, err := h.manager.ForecastCost(poolID, months)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, forecast)
}

// getCostTrend 获取成本趋势
func (h *Handlers) getCostTrend(c *gin.Context) {
	trend := h.manager.GetCostTrend(12)
	c.JSON(http.StatusOK, gin.H{
		"trend": trend,
	})
}

// compareTiers 比较存储层级
func (h *Handlers) compareTiers(c *gin.Context) {
	var req struct {
		UsedGB float64 `json:"used_gb" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	costs := h.manager.CompareTiers(req.UsedGB)
	c.JSON(http.StatusOK, gin.H{
		"used_gb": req.UsedGB,
		"costs":   costs,
	})
}

// recordCost 记录成本
func (h *Handlers) recordCost(c *gin.Context) {
	var record CostRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.RecordCost(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "成本记录成功",
		"record":  record,
	})
}

// getCostHistory 获取成本历史
func (h *Handlers) getCostHistory(c *gin.Context) {
	poolID := c.Query("pool_id")
	records := h.manager.GetCostHistory(poolID, 12)
	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// generateSuggestions 生成优化建议
func (h *Handlers) generateSuggestions(c *gin.Context) {
	suggestions := h.manager.GenerateOptimizationSuggestions()
	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// getSuggestions 获取优化建议
func (h *Handlers) getSuggestions(c *gin.Context) {
	suggestions := h.manager.GetSuggestions()
	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// calculateROI 计算ROI
func (h *Handlers) calculateROI(c *gin.Context) {
	var req struct {
		InvestmentUSD    float64 `json:"investment_usd" binding:"required"`
		AnnualSavingsUSD float64 `json:"annual_savings_usd" binding:"required"`
		Years            int     `json:"years"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.Years <= 0 {
		req.Years = 3
	}

	roi := h.manager.CalculateROI(req.InvestmentUSD, req.AnnualSavingsUSD, req.Years)
	c.JSON(http.StatusOK, roi)
}

// getOverview 获取概览
func (h *Handlers) getOverview(c *gin.Context) {
	totalStorage := h.manager.GetTotalStorage()
	totalCost := h.manager.CalculateTotalCost()
	breakdown := h.manager.GetCostBreakdown()
	suggestions := h.manager.GetSuggestions()

	c.JSON(http.StatusOK, gin.H{
		"storage":     totalStorage,
		"total_cost":  totalCost,
		"breakdown":   breakdown,
		"suggestions": len(suggestions),
	})
}
