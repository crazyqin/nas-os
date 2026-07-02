// Package costpredict - 成本预测 HTTP API 处理器
// 使用 Gin 框架提供 REST API
package costpredict

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 成本预测 HTTP 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建新的 HTTP 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册 API 路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	costpredict := rg.Group("/costpredict")
	{
		costpredict.GET("/forecast", h.GetForecast)
		costpredict.GET("/growth", h.GetGrowthForecast)
		costpredict.POST("/alert", h.SetAlert)
		costpredict.GET("/report", h.GetReport)
	}
}

// GetForecast 获取成本预测
// GET /api/costpredict/forecast.
func (h *Handler) GetForecast(c *gin.Context) {
	h.logger.Info("Getting cost forecast")

	// 从查询参数获取
	method := PredictionMethod(c.DefaultQuery("method", "linear_regression"))
	horizon := ForecastHorizon(c.DefaultQuery("horizon", "1_year"))
	provider := c.Query("provider")

	result, err := h.manager.GetForecast(method, horizon, provider)
	if err != nil {
		h.logger.Error("Failed to get forecast", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get forecast: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetGrowthForecast 获取存储增长趋势
// GET /api/costpredict/growth.
func (h *Handler) GetGrowthForecast(c *gin.Context) {
	h.logger.Info("Getting growth forecast")

	provider := c.Query("provider")
	capacityGB := 10000.0 // 默认10TB
	if v := c.Query("capacity_gb"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &capacityGB); err != nil {
			capacityGB = 10000.0
		}
	}

	result, err := h.manager.GetGrowthForecast(provider, capacityGB)
	if err != nil {
		h.logger.Error("Failed to get growth forecast", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get growth forecast: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// SetAlert 设置预算告警
// POST /api/costpredict/alert.
func (h *Handler) SetAlert(c *gin.Context) {
	h.logger.Info("Setting budget alert")

	var req AlertConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.SetAlertConfig(req)
	if err != nil {
		h.logger.Error("Failed to set alert", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to set alert: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetReport 获取预测报告
// GET /api/costpredict/report.
func (h *Handler) GetReport(c *gin.Context) {
	h.logger.Info("Getting prediction report")

	provider := c.Query("provider")

	result, err := h.manager.GetReport(provider)
	if err != nil {
		h.logger.Error("Failed to get report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get report: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
