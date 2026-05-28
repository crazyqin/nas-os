// Package energydashboard 提供能源看板 HTTP API 处理器
package energydashboard

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 能源看板 HTTP 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	energy := r.Group("/energy")
	{
		// 仪表盘数据
		energy.GET("/dashboard", h.GetDashboard)
		energy.GET("/power/current", h.GetCurrentPower)

		// 能耗记录
		energy.GET("/records", h.GetRecords)
		energy.GET("/trend", h.GetTrend)

		// 预算管理
		energy.GET("/budgets", h.GetBudgets)
		energy.POST("/budgets", h.SetBudget)
		energy.DELETE("/budgets/:id", h.DeleteBudget)

		// 告警管理
		energy.GET("/alerts", h.GetAlerts)
		energy.POST("/alerts/:id/ack", h.AcknowledgeAlert)

		// 地区配置
		energy.GET("/regions/:code", h.GetRegionConfig)
		energy.PUT("/regions", h.UpdateRegionConfig)

		// 设备管理
		energy.GET("/devices", h.GetDevices)
		energy.POST("/devices", h.AddDevice)
		energy.DELETE("/devices/:id", h.RemoveDevice)

		// 报告
		energy.POST("/reports", h.GenerateReport)

		// 配置
		energy.GET("/config", h.GetConfig)
		energy.PUT("/config", h.UpdateConfig)
	}
}

// GetDashboard 获取仪表盘数据
func (h *Handler) GetDashboard(c *gin.Context) {
	data := h.manager.GetDashboardData()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetCurrentPower 获取当前功耗
func (h *Handler) GetCurrentPower(c *gin.Context) {
	power := h.manager.GetCurrentPower()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    power,
	})
}

// GetRecordsRequest 获取记录请求
type GetRecordsRequest struct {
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Limit     int    `form:"limit"`
}

// GetRecords 获取能耗记录
func (h *Handler) GetRecords(c *gin.Context) {
	var req GetRecordsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []interface{}{},
	})
}

// GetTrend 获取趋势数据
func (h *Handler) GetTrend(c *gin.Context) {
	hours := 24
	data := h.manager.GetDashboardData()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hours": hours,
			"trend": data.Trend,
		},
	})
}

// GetBudgets 获取所有预算
func (h *Handler) GetBudgets(c *gin.Context) {
	budgets := h.manager.GetBudgets()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    budgets,
	})
}

// SetBudget 设置预算
func (h *Handler) SetBudget(c *gin.Context) {
	var req SetBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	budget := h.manager.SetBudget(&req)
	h.logger.Info("budget created", zap.String("id", budget.ID), zap.String("name", budget.Name))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    budget,
	})
}

// DeleteBudget 删除预算
func (h *Handler) DeleteBudget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBudget(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.logger.Info("budget deleted", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "预算已删除",
	})
}

// GetAlerts 获取告警
func (h *Handler) GetAlerts(c *gin.Context) {
	data := h.manager.GetDashboardData()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data.Alerts,
	})
}

// AcknowledgeAlert 确认告警
func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.AcknowledgeAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.logger.Info("alert acknowledged", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "告警已确认",
	})
}

// GetRegionConfig 获取地区配置
func (h *Handler) GetRegionConfig(c *gin.Context) {
	code := c.Param("code")
	region, err := h.manager.GetRegionConfig(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    region,
	})
}

// UpdateRegionConfig 更新地区配置
func (h *Handler) UpdateRegionConfig(c *gin.Context) {
	var req UpdateRegionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.manager.UpdateRegionConfig(&req)
	h.logger.Info("region config updated", zap.String("code", req.Code))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "地区配置已更新",
	})
}

// GetDevices 获取设备列表
func (h *Handler) GetDevices(c *gin.Context) {
	devices := h.manager.GetDevices()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
	})
}

// AddDevice 添加设备
func (h *Handler) AddDevice(c *gin.Context) {
	var dev DeviceConfig
	if err := c.ShouldBindJSON(&dev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if dev.ID == "" {
		dev.ID = generateID()
	}

	h.manager.AddDevice(&dev)
	h.logger.Info("device added", zap.String("id", dev.ID), zap.String("name", dev.Name))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    dev,
	})
}

// RemoveDevice 移除设备
func (h *Handler) RemoveDevice(c *gin.Context) {
	id := c.Param("id")
	h.manager.RemoveDevice(id)
	h.logger.Info("device removed", zap.String("id", id))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备已移除",
	})
}

// GenerateReportRequest 生成报告请求
type GenerateReportRequest struct {
	ReportType ReportType `json:"report_type" binding:"required"`
	StartDate  string     `json:"start_date" binding:"required"`
	EndDate    string     `json:"end_date" binding:"required"`
}

// GenerateReport 生成能源报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid start_date format, use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid end_date format, use YYYY-MM-DD",
		})
		return
	}

	report, err := h.manager.GenerateReport(c.Request.Context(), req.ReportType, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.logger.Info("report generated", zap.String("id", report.ID), zap.String("type", string(report.ReportType)))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    report,
	})
}

// GetConfig 获取配置
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateConfig 更新配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var config EnergyDashboardConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&config)
	h.logger.Info("config updated")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已更新",
	})
}