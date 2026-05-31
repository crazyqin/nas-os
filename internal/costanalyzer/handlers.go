// Package costanalyzer 存储成本分析器 HTTP handlers
package costanalyzer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 成本分析HTTP处理器
type Handlers struct {
	mgr      *Manager
	analyzer *CostAnalyzer
}

// NewHandlers 创建HTTP处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		mgr:      mgr,
		analyzer: NewCostAnalyzer(mgr),
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/cost-analyzer")
	{
		// 成本分析
		g.POST("/analyze", h.Analyze)
		g.GET("/report/latest", h.GetLatestReport)
		g.GET("/stats", h.GetStats)

		// 存储池管理
		g.GET("/pools", h.GetPools)
		g.POST("/pools", h.AddPool)
		g.DELETE("/pools/:id", h.RemovePool)
		g.PUT("/pools/:id/usage", h.UpdatePoolUsage)

		// 趋势与预测
		g.GET("/trends", h.GetTrends)
		g.GET("/forecast/:months", h.ForecastCost)
		g.GET("/growth/:months", h.ForecastGrowth)

		// 分析工具
		g.GET("/efficiency", h.GetEfficiency)
		g.GET("/breakdown", h.GetBreakdown)
		g.GET("/compare", h.ComparePeriods)
		g.GET("/recommendations", h.GetRecommendations)
		g.GET("/savings", h.EstimateSavings)
		g.GET("/roi", h.GetROI)
		g.GET("/cloud-compare", h.CompareCloud)
	}
}

// Analyze 执行成本分析
func (h *Handlers) Analyze(c *gin.Context) {
	report := h.mgr.AnalyzeCost()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

// GetLatestReport 获取最新报告
func (h *Handlers) GetLatestReport(c *gin.Context) {
	report, err := h.mgr.GetLatestReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

// GetStats 获取统计概览
func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}

// GetPools 获取所有存储池
func (h *Handlers) GetPools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetPools()})
}

// AddPool 添加存储池
func (h *Handlers) AddPool(c *gin.Context) {
	var pool StoragePool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.AddPool(&pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "pool added"})
}

// RemovePool 移除存储池
func (h *Handlers) RemovePool(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.RemovePool(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "pool removed"})
}

// UpdatePoolUsage 更新存储池使用量
func (h *Handlers) UpdatePoolUsage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		UsedTB float64 `json:"used_tb"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.UpdatePoolUsage(id, req.UsedTB); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "usage updated"})
}

// GetTrends 获取成本趋势
func (h *Handlers) GetTrends(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetTrends(period)})
}

// ForecastCost 成本预测
func (h *Handlers) ForecastCost(c *gin.Context) {
	months, err := strconv.Atoi(c.Param("months"))
	if err != nil || months <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid months"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.analyzer.ForecastCost(months)})
}

// ForecastGrowth 增长预测
func (h *Handlers) ForecastGrowth(c *gin.Context) {
	months, err := strconv.Atoi(c.Param("months"))
	if err != nil || months <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid months"})
		return
	}
	forecast := h.analyzer.ForecastGrowth(months)
	if forecast == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "insufficient data"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": forecast})
}

// GetEfficiency 获取存储池效率
func (h *Handlers) GetEfficiency(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.analyzer.AnalyzePoolEfficiency()})
}

// GetBreakdown 获取成本明细
func (h *Handlers) GetBreakdown(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.analyzer.GetCostBreakdown()})
}

// ComparePeriods 对比时期
func (h *Handlers) ComparePeriods(c *gin.Context) {
	p1 := c.Query("period1")
	p2 := c.Query("period2")
	if p1 == "" || p2 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "period1 and period2 required"})
		return
	}
	result, err := h.analyzer.ComparePeriods(p1, p2)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// GetRecommendations 获取优化建议
func (h *Handlers) GetRecommendations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.analyzer.GetRecommendations()})
}

// EstimateSavings 估算节省
func (h *Handlers) EstimateSavings(c *gin.Context) {
	dedup := c.Query("dedup") == "true"
	compress := c.Query("compress") == "true"
	tiering := c.Query("tiering") == "true"
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.analyzer.EstimateSavings(dedup, compress, tiering)})
}

// GetROI 获取ROI
func (h *Handlers) GetROI(c *gin.Context) {
	investmentStr := c.DefaultQuery("investment", "0")
	investment, err := strconv.ParseFloat(investmentStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid investment amount"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetROI(investment)})
}

// CompareCloud 云存储对比
func (h *Handlers) CompareCloud(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.CompareCloud()})
}
