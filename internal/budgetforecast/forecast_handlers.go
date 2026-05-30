// Package budgetforecast - 预算预测 REST API 处理器
package budgetforecast

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ForecastHandlers 预算预测 API 处理器
type ForecastHandlers struct {
	manager *ForecastManager
	logger  *zap.Logger
}

// NewForecastHandlers 创建预算预测处理器
func NewForecastHandlers(manager *ForecastManager, logger *zap.Logger) *ForecastHandlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ForecastHandlers{
		manager: manager,
		logger:  logger,
	}
}

// RegisterForecastRoutes 注册预测路由
func (h *ForecastHandlers) RegisterForecastRoutes(rg *gin.RouterGroup) {
	forecast := rg.Group("/forecast")
	{
		// 生成预测
		forecast.GET("/generate", h.generateForecast)

		// 预算告警
		forecast.GET("/alerts", h.getAlerts)
		forecast.POST("/alerts/config", h.setAlerts)

		// 成本趋势
		forecast.GET("/trends", h.getTrends)

		// 导出报告
		forecast.POST("/export", h.exportReport)
		forecast.GET("/export/:exportID", h.getExport)

		// 预测模型
		forecast.GET("/models", h.getModels)

		// 预算配置
		forecast.GET("/configs", h.getConfigs)
		forecast.PUT("/configs/:configID", h.updateConfig)
	}
}

// generateForecast 生成预测
// GET /api/v1/budget/forecast/generate?months=6&model=linear
func (h *ForecastHandlers) generateForecast(c *gin.Context) {
	months := 6
	if m := c.Query("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}

	modelType := c.DefaultQuery("model", "linear")

	result, err := h.manager.GenerateForecast(months, modelType)
	if err != nil {
		h.logger.Error("Failed to generate forecast", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "生成预测失败: " + err.Error(),
		})
		return
	}

	h.logger.Info("Forecast generated",
		zap.Int("months", months),
		zap.String("model", modelType),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// getAlerts 获取预算告警
// GET /api/v1/budget/forecast/alerts
func (h *ForecastHandlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"alerts": alerts,
			"total":  len(alerts),
		},
	})
}

// setAlerts 设置预算告警
// POST /api/v1/budget/forecast/alerts/config
func (h *ForecastHandlers) setAlerts(c *gin.Context) {
	var req struct {
		ConfigID   string           `json:"config_id" binding:"required"`
		Thresholds []AlertThreshold `json:"thresholds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetAlerts(req.ConfigID, req.Thresholds); err != nil {
		h.logger.Error("Failed to set alerts", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "设置告警失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "告警配置已更新",
	})
}

// getTrends 获取成本趋势
// GET /api/v1/budget/forecast/trends?resource_type=storage&period=monthly&start_date=2024-01-01&end_date=2024-12-31
func (h *ForecastHandlers) getTrends(c *gin.Context) {
	resourceType := c.DefaultQuery("resource_type", "storage")
	period := c.DefaultQuery("period", "monthly")

	var startDate, endDate time.Time
	if v := c.Query("start_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			startDate = t
		}
	}
	if v := c.Query("end_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			endDate = t
		}
	}

	// 默认最近12个月
	if startDate.IsZero() {
		startDate = time.Now().AddDate(-1, 0, 0)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	trend, err := h.manager.GetTrends(resourceType, period, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get trends", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取趋势失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trend,
	})
}

// exportReport 导出报告
// POST /api/v1/budget/forecast/export
func (h *ForecastHandlers) exportReport(c *gin.Context) {
	var req ExportRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	// 默认最近12个月
	if req.StartDate.IsZero() {
		req.StartDate = time.Now().AddDate(-1, 0, 0)
	}
	if req.EndDate.IsZero() {
		req.EndDate = time.Now()
	}

	export, err := h.manager.ExportReport(req)
	if err != nil {
		h.logger.Error("Failed to export report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "导出报告失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    export,
	})
}

// getExport 获取导出结果
// GET /api/v1/budget/forecast/export/:exportID
func (h *ForecastHandlers) getExport(c *gin.Context) {
	exportID := c.Param("exportID")

	export, err := h.manager.GetExport(exportID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "导出结果未找到: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    export,
	})
}

// getModels 获取预测模型列表
// GET /api/v1/budget/forecast/models
func (h *ForecastHandlers) getModels(c *gin.Context) {
	models := h.manager.GetModels()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"models": models,
			"total":  len(models),
		},
	})
}

// getConfigs 获取预算配置列表
// GET /api/v1/budget/forecast/configs
func (h *ForecastHandlers) getConfigs(c *gin.Context) {
	configs := h.manager.GetConfigs()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configs": configs,
			"total":   len(configs),
		},
	})
}

// updateConfig 更新预算配置
// PUT /api/v1/budget/forecast/configs/:configID
func (h *ForecastHandlers) updateConfig(c *gin.Context) {
	configID := c.Param("configID")

	var config BudgetConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(configID, &config); err != nil {
		h.logger.Error("Failed to update config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "更新配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已更新",
	})
}
