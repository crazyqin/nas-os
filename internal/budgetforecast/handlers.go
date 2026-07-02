package budgetforecast

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 存储预算预测 HTTP 处理器。
type Handlers struct {
	engine *ForecastEngine
	logger *zap.Logger
}

// NewHandlers 创建处理器。
func NewHandlers(engine *ForecastEngine, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{engine: engine, logger: logger}
}

// RegisterRoutes 注册路由。
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	budgetGroup := api.Group("/budget")
	{
		budgetGroup.GET("/forecast", h.getForecast)
		budgetGroup.POST("/snapshot", h.addSnapshot)
		budgetGroup.GET("/alerts", h.getAlerts)
	}
}

// getForecast 获取预测。
// GET /api/v1/budget/forecast?months=6.
func (h *Handlers) getForecast(c *gin.Context) {
	months := 6
	if m := c.Query("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}

	result := h.engine.Forecast(months)
	h.logger.Info("生成预算预测",
		zap.Int("months", months),
		zap.Float64("current_usage_gb", result.CurrentUsageGB),
		zap.Int("days_until_full", result.DaysUntilFull),
	)

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// addSnapshot 添加使用快照。
// POST /api/v1/budget/snapshot.
func (h *Handlers) addSnapshot(c *gin.Context) {
	var snapshot UsageSnapshot
	if err := c.ShouldBindJSON(&snapshot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if snapshot.Date.IsZero() {
		snapshot.Date = time.Now()
	}

	h.engine.AddSnapshot(snapshot)
	h.logger.Info("添加预算快照",
		zap.Time("date", snapshot.Date),
		zap.Int64("used_bytes", snapshot.UsedBytes),
		zap.Int64("total_bytes", snapshot.TotalBytes),
	)

	c.JSON(http.StatusCreated, gin.H{
		"message":  "快照添加成功",
		"snapshot": snapshot,
	})
}

// getAlerts 获取预算告警。
// GET /api/v1/budget/alerts.
func (h *Handlers) getAlerts(c *gin.Context) {
	result := h.engine.Forecast(6) // 默认预测6个月来生成告警
	h.logger.Info("获取预算告警",
		zap.Int("alert_count", len(result.Alerts)),
	)

	c.JSON(http.StatusOK, gin.H{
		"alerts": result.Alerts,
		"total":  len(result.Alerts),
	})
}
