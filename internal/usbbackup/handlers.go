// Package usbbackup - HTTP API 处理器
package usbbackup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers USB 备份 HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	usb := api.Group("/usbbackup")
	{
		// 任务
		usb.POST("/tasks", h.CreateTask)
		usb.GET("/tasks", h.ListTasks)
		usb.GET("/tasks/:id", h.GetTask)
		usb.PUT("/tasks/:id", h.UpdateTask)
		usb.DELETE("/tasks/:id", h.DeleteTask)
		usb.POST("/tasks/:id/trigger", h.TriggerTask)
		usb.POST("/tasks/:id/pause", h.PauseTask)
		usb.POST("/tasks/:id/resume", h.ResumeTask)
		usb.GET("/tasks/:id/progress", h.GetProgress)
		usb.GET("/progress", h.GetAllProgress)
	}
}

func (h *Handlers) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.mgr.CreateTask(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) ListTasks(c *gin.Context) {
	tasks := h.mgr.ListTasks()
	c.JSON(http.StatusOK, tasks)
}

func (h *Handlers) GetTask(c *gin.Context) {
	task, err := h.mgr.GetTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) UpdateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := h.mgr.UpdateTask(c.Param("id"), &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) DeleteTask(c *gin.Context) {
	if err := h.mgr.DeleteTask(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handlers) TriggerTask(c *gin.Context) {
	progress, err := h.mgr.TriggerTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, progress)
}

func (h *Handlers) PauseTask(c *gin.Context) {
	if err := h.mgr.PauseTask(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已暂停"})
}

func (h *Handlers) ResumeTask(c *gin.Context) {
	if err := h.mgr.ResumeTask(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func (h *Handlers) GetProgress(c *gin.Context) {
	progress := h.mgr.GetProgress(c.Param("id"))
	if progress == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "进度不存在"})
		return
	}
	c.JSON(http.StatusOK, progress)
}

func (h *Handlers) GetAllProgress(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetAllProgress())
}
