// Package systemclone - HTTP API 处理器
package systemclone

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 系统克隆 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	clone := rg.Group("/system-clone")
	{
		// 克隆任务
		clone.GET("/tasks", h.listTasks)
		clone.POST("/tasks", h.createTask)
		clone.GET("/tasks/:id", h.getTask)
		clone.POST("/tasks/:id/start", h.startTask)

		// 镜像管理
		clone.GET("/images", h.listImages)
		clone.POST("/images", h.createImage)

		// 恢复
		clone.POST("/restore", h.restore)

		// PXE 部署
		clone.POST("/pxe", h.configurePXE)

		// 统计
		clone.GET("/stats", h.getStats)
	}
}

func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListCloneTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handlers) createTask(c *gin.Context) {
	var task DiskCloneTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateCloneTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) getTask(c *gin.Context) {
	task, err := h.manager.GetCloneTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) startTask(c *gin.Context) {
	if err := h.manager.StartClone(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (h *Handlers) listImages(c *gin.Context) {
	images := h.manager.ListImages()
	c.JSON(http.StatusOK, gin.H{"images": images})
}

func (h *Handlers) createImage(c *gin.Context) {
	var image BackupImage
	if err := c.ShouldBindJSON(&image); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateImage(&image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, image)
}

func (h *Handlers) restore(c *gin.Context) {
	var req struct {
		ImageID    string `json:"imageId"`
		TargetDisk string `json:"targetDisk"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.manager.RestoreFromImage(req.ImageID, req.TargetDisk)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) configurePXE(c *gin.Context) {
	var config PXEDeployConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.ConfigurePXE(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "configured"})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}
