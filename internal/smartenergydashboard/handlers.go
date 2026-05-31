// Package smartenergydashboard 提供 REST API 处理器
package smartenergydashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 能源仪表盘 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	energy := r.Group("/energy")
	{
		// 当前功耗
		energy.GET("/current", h.getCurrentPower)

		// 历史记录
		energy.GET("/history", h.getHistory)

		// 设备功耗
		energy.GET("/devices", h.getDevicePower)

		// 预算管理
		energy.POST("/budget", h.setBudget)
		energy.GET("/budget", h.getBudget)

		// 能源报告
		energy.GET("/report", h.getReport)

		// 成本预测
		energy.GET("/forecast", h.getForecast)

		// 设置
		energy.POST("/settings", h.updateSettings)
		energy.GET("/settings", h.getSettings)

		// 节能建议
		energy.GET("/tips", h.getTips)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getCurrentPower 获取当前功耗
func (h *Handlers) getCurrentPower(c *gin.Context) {
	reading := h.manager.GetCurrentPower()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    reading,
	})
}

// getHistory 获取历史记录
func (h *Handlers) getHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")

	records := h.manager.GetHistory(period)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    records,
	})
}

// getDevicePower 获取设备功耗
func (h *Handlers) getDevicePower(c *gin.Context) {
	devices := h.manager.GetDevicePower()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// budgetRequest 预算请求
type budgetRequest struct {
	MonthlyLimitKWh  float64 `json:"monthly_limit_kwh" binding:"required"`
	MonthlyLimitCost float64 `json:"monthly_limit_cost" binding:"required"`
	AlertThreshold   float64 `json:"alert_threshold"`
}

// setBudget 设置预算
func (h *Handlers) setBudget(c *gin.Context) {
	var req budgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 默认阈值 80%
	if req.AlertThreshold <= 0 || req.AlertThreshold > 100 {
		req.AlertThreshold = 80
	}

	budget := h.manager.SetBudget(req.MonthlyLimitKWh, req.MonthlyLimitCost, req.AlertThreshold)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "budget set",
		Data:    budget,
	})
}

// getBudget 获取当前预算
func (h *Handlers) getBudget(c *gin.Context) {
	budget := h.manager.GetBudget()
	if budget == nil {
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "no budget configured",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    budget,
	})
}

// getReport 获取能源报告
func (h *Handlers) getReport(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")

	report := h.manager.GenerateReport(period)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// getForecast 获取成本预测
func (h *Handlers) getForecast(c *gin.Context) {
	forecasts := h.manager.ForecastCost()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    forecasts,
	})
}

// settingsRequest 设置请求
type settingsRequest struct {
	ElectricityRate   *float64 `json:"electricity_rate"`
	CarbonFactor      *float64 `json:"carbon_factor"`
	Currency          string   `json:"currency"`
	MonitoringEnabled *bool    `json:"monitoring_enabled"`
	AlertEnabled      *bool    `json:"alert_enabled"`
}

// updateSettings 更新设置
func (h *Handlers) updateSettings(c *gin.Context) {
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	settings := h.manager.GetSettings()

	if req.ElectricityRate != nil {
		settings.ElectricityRate = *req.ElectricityRate
	}
	if req.CarbonFactor != nil {
		settings.CarbonFactor = *req.CarbonFactor
	}
	if req.Currency != "" {
		settings.Currency = req.Currency
	}
	if req.MonitoringEnabled != nil {
		settings.MonitoringEnabled = *req.MonitoringEnabled
	}
	if req.AlertEnabled != nil {
		settings.AlertEnabled = *req.AlertEnabled
	}

	h.manager.UpdateSettings(settings)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "settings updated",
		Data:    settings,
	})
}

// getSettings 获取设置
func (h *Handlers) getSettings(c *gin.Context) {
	settings := h.manager.GetSettings()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    settings,
	})
}

// getTips 获取节能建议
func (h *Handlers) getTips(c *gin.Context) {
	tips := h.manager.GetTips()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tips,
	})
}
