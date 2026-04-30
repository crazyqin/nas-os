package hybridpool

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 混合池HTTP处理器
type Handler struct {
	pool *HybridPool
}

// NewHandler 创建处理器
func NewHandler(pool *HybridPool) *Handler {
	return &Handler{pool: pool}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	hybrid := rg.Group("/hybridpool")
	{
		hybrid.GET("/status", h.GetStatus)
		hybrid.GET("/stats", h.GetStats)
		hybrid.GET("/heatmap", h.GetHeatMap)
		hybrid.POST("/config", h.UpdateConfig)
		hybrid.POST("/start", h.Start)
		hybrid.POST("/stop", h.Stop)
		hybrid.POST("/record-access", h.RecordAccess)
		hybrid.GET("/tiers", h.GetTiers)
	}
}

// GetStatus 获取混合池状态
func (h *Handler) GetStatus(c *gin.Context) {
	h.pool.mu.RLock()
	defer h.pool.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"name":    h.pool.config.Name,
		"running": h.pool.running,
		"tiers":   len(h.pool.tiers),
		"config":  h.pool.config,
	})
}

// GetStats 获取分层统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.pool.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetHeatMap 获取文件热度图
func (h *Handler) GetHeatMap(c *gin.Context) {
	heatmap := h.pool.GetHeatMap()
	// 只返回top 100热点文件
	result := make([]*FileHeatMap, 0, 100)
	for _, heat := range heatmap {
		result = append(result, heat)
		if len(result) >= 100 {
			break
		}
	}
	c.JSON(http.StatusOK, result)
}

// UpdateConfig 更新配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var config HybridPoolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.pool.mu.Lock()
	h.pool.config = config
	h.pool.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// Start 启动混合池
func (h *Handler) Start(c *gin.Context) {
	if err := h.pool.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "hybrid pool started"})
}

// Stop 停止混合池
func (h *Handler) Stop(c *gin.Context) {
	h.pool.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "hybrid pool stopped"})
}

// RecordAccess 记录文件访问
func (h *Handler) RecordAccess(c *gin.Context) {
	var req struct {
		FilePath   string `json:"filePath" binding:"required"`
		ReadBytes  int64  `json:"readBytes"`
		WriteBytes int64  `json:"writeBytes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.pool.RecordAccess(req.FilePath, req.ReadBytes, req.WriteBytes)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("access recorded for %s", req.FilePath)})
}

// GetTiers 获取层级信息
func (h *Handler) GetTiers(c *gin.Context) {
	tiers := h.pool.GetTierStatus()
	c.JSON(http.StatusOK, tiers)
}
