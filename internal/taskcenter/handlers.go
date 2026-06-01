// Package taskcenter HTTP API 处理器 (gin)
package taskcenter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	tc *TaskCenter
}

// NewHandler 创建处理器
func NewHandler(tc *TaskCenter) *Handler {
	return &Handler{tc: tc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/task-center")
	{
		group.GET("/tasks", h.ListTasks)
		group.POST("/tasks", h.CreateTask)
		group.GET("/tasks/:id", h.GetTask)
		group.PUT("/tasks/:id", h.UpdateTask)
		group.DELETE("/tasks/:id", h.DeleteTask)
		group.POST("/tasks/:id/start", h.StartTask)
		group.POST("/tasks/:id/pause", h.PauseTask)
		group.POST("/tasks/:id/cancel", h.CancelTask)
		group.GET("/tasks/:id/logs", h.GetTaskLogs)
		group.GET("/stats", h.GetStats)
	}
}

// ListTasks 获取任务列表
func (h *Handler) ListTasks(c *gin.Context) {
	taskType := TaskType(c.Query("type"))
	status := TaskStatus(c.Query("status"))
	tasks := h.tc.ListTasks(taskType, status)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks})
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.tc.CreateTask(task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": task})
}

// GetTask 获取任务详情
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.tc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// UpdateTask 更新任务
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	task.ID = id
	if err := h.tc.UpdateTask(task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// DeleteTask 删除任务
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.tc.DeleteTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "任务已删除"})
}

// StartTask 启动任务
func (h *Handler) StartTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.tc.ExecuteTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "任务已启动"})
}

// PauseTask 暂停任务
func (h *Handler) PauseTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.tc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	if task.Status != TaskStatusRunning {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "任务不在运行状态"})
		return
	}
	task.Status = TaskStatusWaiting
	task.UpdatedAt = time.Now()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "任务已暂停"})
}

// CancelTask 取消任务
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.tc.CancelTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "任务已取消"})
}

// GetTaskLogs 获取任务日志
func (h *Handler) GetTaskLogs(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	logs := h.tc.GetLogs(id, limit)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	tasks := h.tc.ListTasks("", "")
	stats := map[string]int{
		"total":     len(tasks),
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
	}
	for _, t := range tasks {
		switch t.Status {
		case TaskStatusPending:
			stats["pending"]++
		case TaskStatusRunning:
			stats["running"]++
		case TaskStatusCompleted:
			stats["completed"]++
		case TaskStatusFailed:
			stats["failed"]++
		case TaskStatusCancelled:
			stats["cancelled"]++
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}
