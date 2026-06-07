// Package storagecost 提供 REST API 处理器
package storagecost

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 存储成本 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cost := r.Group("/cost")
	{
		cost.POST("/report", h.generateReport)
		cost.GET("/report", h.listReports)
		cost.GET("/report/:id", h.getReport)
		cost.GET("/breakdown", h.getBreakdown)
		cost.POST("/forecast", h.forecastCost)
		cost.GET("/forecast", h.listForecasts)
		cost.GET("/optimize", h.getOptimizations)
		cost.GET("/config", h.getConfig)
		cost.PUT("/config", h.updateConfig)
	}

	// 注册存储成本相关路由
	h.RegisterStorageCostRoutes(r)
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// generateReport 生成成本报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Pool        string `json:"pool"`
		Volume      string `json:"volume"`
		Directory   string `json:"directory"`
		PeriodStart string `json:"period_start" binding:"required"`
		PeriodEnd   string `json:"period_end" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid period_start format, use YYYY-MM-DD",
		})
		return
	}

	periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid period_end format, use YYYY-MM-DD",
		})
		return
	}

	report, err := h.manager.GenerateReport(req.Pool, req.Volume, req.Directory, periodStart, periodEnd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "report generated",
		Data:    report,
	})
}

// listReports 列出报告
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    reports,
	})
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// getBreakdown 获取成本明细
func (h *Handlers) getBreakdown(c *gin.Context) {
	pool := c.Query("pool")
	volume := c.Query("volume")
	directory := c.Query("directory")
	tier := StorageTier(c.Query("tier"))
	category := CostCategory(c.Query("category"))

	breakdowns, err := h.manager.GetBreakdown(pool, volume, directory, tier, category)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    breakdowns,
	})
}

// forecastCost 预测成本
func (h *Handlers) forecastCost(c *gin.Context) {
	var req struct {
		Months     int     `json:"months"`
		GrowthRate float64 `json:"growth_rate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	forecast, err := h.manager.ForecastCost(req.Months, req.GrowthRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "forecast generated",
		Data:    forecast,
	})
}

// listForecasts 列出预测
func (h *Handlers) listForecasts(c *gin.Context) {
	forecasts := h.manager.ListForecasts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    forecasts,
	})
}

// getOptimizations 获取优化建议
func (h *Handlers) getOptimizations(c *gin.Context) {
	suggestions, err := h.manager.GetOptimizations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    suggestions,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg StorageCostConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}
