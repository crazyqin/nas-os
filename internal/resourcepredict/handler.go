package resourcepredict

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 资源预测HTTP处理器.
type Handler struct {
	predictor *ResourcePredictor
}

// NewHandler 创建处理器.
func NewHandler(predictor *ResourcePredictor) *Handler {
	return &Handler{predictor: predictor}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/resourcepredict")
	{
		group.GET("/predictions", h.GetPredictions)
		group.GET("/metrics", h.GetMetrics)
		group.POST("/record", h.RecordValue)
		group.POST("/predict", h.PredictNow)
		group.GET("/config", h.GetConfig)
		group.POST("/thresholds", h.UpdateThresholds)
	}
}

// RecordRequest 记录请求.
type RecordRequest struct {
	ResourceType string  `json:"resourceType" binding:"required"`
	Value        float64 `json:"value" binding:"required"`
}

// ThresholdUpdate 阈值更新请求.
type ThresholdUpdate struct {
	WarningDays  *int     `json:"warningDays"`
	CriticalDays *int     `json:"criticalDays"`
	UrgentDays   *int     `json:"urgentDays"`
	MinR2        *float64 `json:"minR2"`
}

// GetPredictions 获取预测结果.
func (h *Handler) GetPredictions(c *gin.Context) {
	predictions := h.predictor.GetLatest()
	if len(predictions) == 0 {
		// 没有缓存的预测，执行一次
		report := h.predictor.PredictNow()
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"data":   report,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"predictions": predictions,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// GetMetrics 获取资源指标.
func (h *Handler) GetMetrics(c *gin.Context) {
	metrics := h.predictor.GetMetrics()
	result := make(map[string]interface{})
	for k, v := range metrics {
		result[string(k)] = v
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   result,
	})
}

// RecordValue 记录资源值.
func (h *Handler) RecordValue(c *gin.Context) {
	var req RecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.predictor.RecordValue(ResourceType(req.ResourceType), req.Value)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "value recorded",
	})
}

// PredictNow 立即预测.
func (h *Handler) PredictNow(c *gin.Context) {
	report := h.predictor.PredictNow()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   report,
	})
}

// GetConfig 获取配置.
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"thresholds": h.predictor.config.Thresholds,
		"retention":  h.predictor.config.RetentionDays,
		"interval":   h.predictor.config.SamplingInterval.String(),
	})
}

// UpdateThresholds 更新阈值.
func (h *Handler) UpdateThresholds(c *gin.Context) {
	var req ThresholdUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.predictor.mu.Lock()
	defer h.predictor.mu.Unlock()

	if req.WarningDays != nil {
		h.predictor.config.Thresholds.WarningDays = *req.WarningDays
	}
	if req.CriticalDays != nil {
		h.predictor.config.Thresholds.CriticalDays = *req.CriticalDays
	}
	if req.UrgentDays != nil {
		h.predictor.config.Thresholds.UrgentDays = *req.UrgentDays
	}
	if req.MinR2 != nil {
		h.predictor.config.Thresholds.MinR2 = *req.MinR2
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "thresholds updated",
	})
}
