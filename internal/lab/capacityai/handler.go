package capacityai

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler AI容量规划HTTP处理器.
type Handler struct {
	ai *CapacityAI
}

// NewHandler 创建处理器.
func NewHandler(ai *CapacityAI) *Handler {
	return &Handler{ai: ai}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/capacityai")
	{
		group.GET("/pools", h.GetPools)
		group.GET("/forecasts", h.GetForecasts)
		group.GET("/optimizations", h.GetOptimizations)
		group.POST("/pool", h.RegisterPool)
		group.POST("/record", h.RecordUsage)
		group.POST("/start", h.Start)
		group.POST("/stop", h.Stop)
	}
}

// GetPools 获取存储池列表.
func (h *Handler) GetPools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ai.GetPools()})
}

// GetForecasts 获取容量预测.
func (h *Handler) GetForecasts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ai.GetForecasts()})
}

// GetOptimizations 获取优化建议.
func (h *Handler) GetOptimizations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": h.ai.GetOptimizations()})
}

// RegisterPool 注册存储池.
func (h *Handler) RegisterPool(c *gin.Context) {
	var pool StoragePool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.ai.RegisterPool(pool)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// RecordUsage 记录使用量.
func (h *Handler) RecordUsage(c *gin.Context) {
	var req struct {
		PoolID    string `json:"poolId" binding:"required"`
		UsedBytes int64  `json:"usedBytes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.ai.RecordUsage(req.PoolID, req.UsedBytes)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Start 启动.
func (h *Handler) Start(c *gin.Context) {
	h.ai.Start()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Stop 停止.
func (h *Handler) Stop(c *gin.Context) {
	h.ai.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
