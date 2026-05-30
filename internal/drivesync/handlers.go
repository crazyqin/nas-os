// Package drivesync 文件同步服务 - HTTP handlers
package drivesync

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	drive := api.Group("/drive")
	{
		drive.POST("/sync", h.handleSync)
		drive.GET("/status", h.handleStatus)
		drive.GET("/conflicts", h.handleConflicts)
		drive.POST("/conflicts/resolve", h.handleResolveConflict)
		drive.GET("/stats", h.handleStats)
		drive.GET("/policy", h.handleGetPolicy)
		drive.PUT("/policy", h.handleSetPolicy)
	}
}

func (h *Handler) handleSync(c *gin.Context) {
	var req struct {
		SourcePath string `json:"source_path"`
		TargetPath string `json:"target_path"`
		IsDir      bool   `json:"is_dir"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var task *SyncTask
	var err error

	if req.IsDir {
		task, err = h.manager.SyncDirectory(c.Request.Context(), req.SourcePath, req.TargetPath)
	} else {
		task, err = h.manager.SyncFile(c.Request.Context(), req.SourcePath, req.TargetPath)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *Handler) handleStatus(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id required"})
		return
	}

	status, err := h.manager.GetSyncStatus(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

func (h *Handler) handleConflicts(c *gin.Context) {
	conflicts, err := h.manager.ListConflicts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts})
}

func (h *Handler) handleResolveConflict(c *gin.Context) {
	var req struct {
		ConflictID string `json:"conflict_id"`
		Resolution string `json:"resolution"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.manager.ResolveConflict(c.Request.Context(), req.ConflictID, req.Resolution); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

func (h *Handler) handleStats(c *gin.Context) {
	stats, err := h.manager.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) handleGetPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) handleSetPolicy(c *gin.Context) {
	var policy SyncPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.manager.SetPolicy(c.Request.Context(), policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
