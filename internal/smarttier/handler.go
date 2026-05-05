package smarttier

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 智能分层调度器HTTP处理器
type Handler struct {
	scheduler *SmartTierScheduler
}

// NewHandler 创建处理器
func NewHandler(scheduler *SmartTierScheduler) *Handler {
	return &Handler{scheduler: scheduler}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/smarttier")
	{
		group.GET("/stats", h.GetStats)
		group.GET("/patterns", h.GetPatterns)
		group.POST("/record", h.RecordIO)
		group.POST("/analyze", h.Analyze)
		group.GET("/config", h.GetConfig)
		group.POST("/config", h.UpdateConfig)
		group.POST("/start", h.Start)
		group.POST("/stop", h.Stop)
	}
}

// IOMetricsPayload I/O上报请求
type IOMetricsPayload struct {
	FilePath     string `json:"filePath" binding:"required"`
	BytesRead    int64  `json:"bytesRead"`
	BytesWritten int64  `json:"bytesWritten"`
}

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	HeatScores   map[string]float64 `json:"heatScores" binding:"required"`
	CurrentTiers map[string]string  `json:"currentTiers" binding:"required"`
}

// GetStats 获取调度器统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.scheduler.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   stats,
	})
}

// GetPatterns 获取I/O模式
func (h *Handler) GetPatterns(c *gin.Context) {
	patterns := h.scheduler.GetIOPatterns()
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   patterns,
		"total":  len(patterns),
	})
}

// RecordIO 记录I/O访问
func (h *Handler) RecordIO(c *gin.Context) {
	var payload IOMetricsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.scheduler.RecordIO(payload.FilePath, payload.BytesRead, payload.BytesWritten)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "I/O recorded",
	})
}

// Analyze 触发分析并返回决策
func (h *Handler) Analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	decisions := h.scheduler.AnalyzeAndDecide(req.HeatScores, req.CurrentTiers)
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"decisions": decisions,
		"count":     len(decisions),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetConfig 获取当前配置
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"config": h.scheduler.config,
		"currentThresholds": gin.H{
			"promote": h.scheduler.currentPromoteThreshold,
			"demote":  h.scheduler.currentDemoteThreshold,
		},
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	SSDUsageThreshold       *float64 `json:"ssdUsageThreshold"`
	EnableAdaptiveThreshold *bool    `json:"enableAdaptiveThreshold"`
	EnablePrefetch          *bool    `json:"enablePrefetch"`
	BasePromoteThreshold    *float64 `json:"basePromoteThreshold"`
	BaseDemoteThreshold     *float64 `json:"baseDemoteThreshold"`
	BatchSize               *int     `json:"batchSize"`
	MinConfidence           *float64 `json:"minConfidence"`
}

// UpdateConfig 更新配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.scheduler.mu.Lock()
	defer h.scheduler.mu.Unlock()

	if req.SSDUsageThreshold != nil {
		h.scheduler.config.SSDUsageThreshold = *req.SSDUsageThreshold
	}
	if req.EnableAdaptiveThreshold != nil {
		h.scheduler.config.EnableAdaptiveThreshold = *req.EnableAdaptiveThreshold
	}
	if req.EnablePrefetch != nil {
		h.scheduler.config.EnablePrefetch = *req.EnablePrefetch
	}
	if req.BasePromoteThreshold != nil {
		h.scheduler.config.BasePromoteThreshold = *req.BasePromoteThreshold
		h.scheduler.currentPromoteThreshold = *req.BasePromoteThreshold
	}
	if req.BaseDemoteThreshold != nil {
		h.scheduler.config.BaseDemoteThreshold = *req.BaseDemoteThreshold
		h.scheduler.currentDemoteThreshold = *req.BaseDemoteThreshold
	}
	if req.BatchSize != nil {
		h.scheduler.config.BatchSize = *req.BatchSize
	}
	if req.MinConfidence != nil {
		h.scheduler.config.MinConfidence = *req.MinConfidence
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "config updated",
	})
}

// Start 启动调度器
func (h *Handler) Start(c *gin.Context) {
	h.scheduler.Start()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "scheduler started"})
}

// Stop 停止调度器
func (h *Handler) Stop(c *gin.Context) {
	h.scheduler.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "scheduler stopped"})
}
