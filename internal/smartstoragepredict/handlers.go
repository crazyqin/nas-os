package smartstoragepredict

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP处理器
type Handler struct {
	predictor *StoragePredictor
}

// NewHandler 创建处理器
func NewHandler(predictor *StoragePredictor) *Handler {
	return &Handler{predictor: predictor}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	predict := router.Group("/storage-predict")
	{
		predict.GET("/pools", h.ListPools)
		predict.POST("/pools", h.RegisterPool)
		predict.GET("/pools/:id", h.GetPool)
		predict.POST("/pools/:id/record", h.RecordUsage)
		predict.GET("/pools/:id/predict", h.Predict)
		predict.GET("/pools/:id/history", h.GetHistory)
		predict.GET("/stats", h.GetStats)
	}
}

// RegisterPoolRequest 注册存储池请求
type RegisterPoolRequest struct {
	ID         string `json:"id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	TotalBytes int64  `json:"total_bytes" binding:"required"`
}

// RegisterPool 注册存储池
func (h *Handler) RegisterPool(c *gin.Context) {
	var req RegisterPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pool := &StoragePool{
		ID:         req.ID,
		Name:       req.Name,
		TotalBytes: req.TotalBytes,
		FreeBytes:  req.TotalBytes,
		CreatedAt:  time.Now(),
	}

	if err := h.predictor.RegisterPool(pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pool)
}

// GetPool 获取存储池
func (h *Handler) GetPool(c *gin.Context) {
	poolID := c.Param("id")

	pool, err := h.predictor.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pool)
}

// ListPools 列出存储池
func (h *Handler) ListPools(c *gin.Context) {
	pools := h.predictor.ListPools()
	c.JSON(http.StatusOK, pools)
}

// RecordUsageRequest 记录使用量请求
type RecordUsageRequest struct {
	UsedBytes int64 `json:"used_bytes" binding:"required"`
}

// RecordUsage 记录使用量
func (h *Handler) RecordUsage(c *gin.Context) {
	poolID := c.Param("id")

	var req RecordUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.predictor.RecordUsage(poolID, req.UsedBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pool, _ := h.predictor.GetPool(poolID)
	c.JSON(http.StatusOK, pool)
}

// Predict 预测存储容量
func (h *Handler) Predict(c *gin.Context) {
	poolID := c.Param("id")

	// 解析预测时间范围
	horizonStr := c.DefaultQuery("horizon", "30d")
	horizon, err := time.ParseDuration(horizonStr)
	if err != nil {
		horizon = 30 * 24 * time.Hour // 默认30天
	}

	result, err := h.predictor.Predict(poolID, horizon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetHistory 获取历史数据
func (h *Handler) GetHistory(c *gin.Context) {
	poolID := c.Param("id")

	// 解析时间范围
	durationStr := c.DefaultQuery("duration", "7d")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 7 * 24 * time.Hour // 默认7天
	}

	history := h.predictor.GetHistory(poolID, duration)
	c.JSON(http.StatusOK, history)
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.predictor.GetStats()
	c.JSON(http.StatusOK, stats)
}

// PredictRequest 预测请求（用于批量预测）
type PredictRequest struct {
	PoolIDs []string `json:"pool_ids"`
	Horizon string   `json:"horizon"`
}

// BatchPredict 批量预测
func (h *Handler) BatchPredict(c *gin.Context) {
	var req PredictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	horizon, err := time.ParseDuration(req.Horizon)
	if err != nil {
		horizon = 30 * 24 * time.Hour
	}

	results := make(map[string]*PredictionResult)
	for _, poolID := range req.PoolIDs {
		result, err := h.predictor.Predict(poolID, horizon)
		if err != nil {
			results[poolID] = &PredictionResult{
				AlertLevel: AlertNormal,
			}
			continue
		}
		results[poolID] = result
	}

	c.JSON(http.StatusOK, results)
}

// ExportData 导出数据
func (h *Handler) ExportData(c *gin.Context) {
	poolID := c.Param("id")

	pool, err := h.predictor.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	history := h.predictor.GetHistory(poolID, 90*24*time.Hour)
	prediction, _ := h.predictor.GetPrediction(poolID)

	export := struct {
		Pool      *StoragePool       `json:"pool"`
		History   []DataPoint        `json:"history"`
		Prediction *PredictionResult `json:"prediction,omitempty"`
	}{
		Pool:       pool,
		History:    history,
		Prediction: prediction,
	}

	c.Header("Content-Disposition", "attachment; filename=storage-predict-"+poolID+".json")
	c.Header("Content-Type", "application/json")
	json.NewEncoder(c.Writer).Encode(export)
}
