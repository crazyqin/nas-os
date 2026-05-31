// Package quickrestore HTTP API 处理器
package quickrestore

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
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/quick-restore")
	{
		group.GET("/points", h.ListPoints)
		group.POST("/points", h.CreatePoint)
		group.DELETE("/points/:id", h.DeletePoint)
		group.POST("/preview", h.PreviewRestore)
		group.POST("/execute", h.ExecuteRestore)
		group.GET("/history", h.GetHistory)
		group.POST("/batch", h.BatchRestore)
		group.GET("/status/:id", h.GetTaskStatus)
	}
}

// ListPoints 获取恢复点列表
func (h *Handler) ListPoints(c *gin.Context) {
	points := h.manager.ListPoints()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": points})
}

// CreatePoint 创建恢复点
func (h *Handler) CreatePoint(c *gin.Context) {
	var req CreatePointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.Source == "" {
		req.Source = SourceSnapshot
	}

	point, err := h.manager.CreatePoint(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": point})
}

// DeletePoint 删除恢复点
func (h *Handler) DeletePoint(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeletePoint(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "恢复点不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PreviewRestore 恢复预览
func (h *Handler) PreviewRestore(c *gin.Context) {
	var req struct {
		PointID    string   `json:"point_id" binding:"required"`
		TargetPath string   `json:"target_path" binding:"required"`
		Files      []string `json:"files,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	preview, err := h.manager.PreviewRestore(req.PointID, req.TargetPath, req.Files)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": preview})
}

// ExecuteRestore 执行恢复
func (h *Handler) ExecuteRestore(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	task, err := h.manager.ExecuteRestore(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": task})
}

// GetHistory 获取恢复历史
func (h *Handler) GetHistory(c *gin.Context) {
	history := h.manager.GetHistory()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": history})
}

// BatchRestore 批量恢复
func (h *Handler) BatchRestore(c *gin.Context) {
	var req BatchRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	tasks, err := h.manager.BatchRestore(req.Requests)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": tasks})
}

// GetTaskStatus 获取恢复任务状态
func (h *Handler) GetTaskStatus(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}
