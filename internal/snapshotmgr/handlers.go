package snapshotmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 快照 HTTP API 处理器
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	snap := rg.Group("/snapshots")
	{
		snap.POST("", h.CreateSnapshot)
		snap.GET("", h.ListSnapshots)
		snap.GET("/stats", h.GetStats)
		snap.GET("/:id", h.GetSnapshot)
		snap.DELETE("/:id", h.DeleteSnapshot)
		snap.POST("/:id/restore", h.RestoreSnapshot)
	}
}

// createSnapshotReq 创建快照请求
type createSnapshotReq struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Source      string         `json:"source"`
	Items       []SnapshotItem `json:"items"`
}

// CreateSnapshot POST /api/v1/snapshots
func (h *Handlers) CreateSnapshot(c *gin.Context) {
	var req createSnapshotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	snap, err := h.manager.CreateSnapshot(req.Name, req.Description, source, req.Items)
	if err != nil {
		h.logger.Error("failed to create snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, snap)
}

// ListSnapshots GET /api/v1/snapshots
func (h *Handlers) ListSnapshots(c *gin.Context) {
	snapshots := h.manager.ListSnapshots()
	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}

// GetSnapshot GET /api/v1/snapshots/:id
func (h *Handlers) GetSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

// DeleteSnapshot DELETE /api/v1/snapshots/:id
func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSnapshot(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "snapshot deleted"})
}

// RestoreSnapshot POST /api/v1/snapshots/:id/restore
func (h *Handlers) RestoreSnapshot(c *gin.Context) {
	id := c.Param("id")
	items, err := h.manager.RestoreSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "restore initiated",
		"items":   items,
	})
}

// GetStats GET /api/v1/snapshots/stats
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
