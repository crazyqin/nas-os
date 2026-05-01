// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers Active Backup HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	ab := api.Group("/activebackup")
	{
		// Agent 管理
		ab.POST("/agents", h.registerAgent)
		ab.GET("/agents", h.listAgents)
		ab.GET("/agents/:id", h.getAgent)
		ab.PUT("/agents/:id", h.updateAgent)
		ab.DELETE("/agents/:id", h.deleteAgent)

		// 备份任务管理
		ab.POST("/tasks", h.createTask)
		ab.GET("/tasks", h.listTasks)
		ab.GET("/tasks/:id", h.getTask)
		ab.PUT("/tasks/:id", h.updateTask)
		ab.DELETE("/tasks/:id", h.deleteTask)
		ab.POST("/tasks/:id/run", h.runTask)
		ab.POST("/tasks/:id/cancel", h.cancelTask)

		// 恢复管理
		ab.GET("/restore/points", h.listRestorePoints)
		ab.POST("/restore/full", h.restoreFull)
		ab.POST("/restore/files", h.restoreFiles)
		ab.GET("/restore/browse/:pointId", h.browseRestorePoint)

		// 统计和存储
		ab.GET("/stats", h.getStats)
		ab.GET("/storage", h.getStorageUsage)
	}
}

// ========== 通用响应 ==========

// Response 通用 API 响应结构.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应.
func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// Error 返回错误响应.
func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== Agent API ==========

// registerAgent 注册 Agent.
func (h *Handlers) registerAgent(c *gin.Context) {
	var req AgentRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	agent, err := h.manager.RegisterAgent(req)
	if err != nil {
		if err == ErrAgentExists {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(agent))
}

// listAgents 列出 Agent.
func (h *Handlers) listAgents(c *gin.Context) {
	agents := h.manager.ListAgents()
	c.JSON(http.StatusOK, Success(agents))
}

// getAgent 获取 Agent 详情.
func (h *Handlers) getAgent(c *gin.Context) {
	id := c.Param("id")
	agent, err := h.manager.GetAgent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(agent))
}

// updateAgent 更新 Agent.
func (h *Handlers) updateAgent(c *gin.Context) {
	id := c.Param("id")
	var req AgentRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	agent, err := h.manager.UpdateAgent(id, req)
	if err != nil {
		if err == ErrAgentNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(agent))
}

// deleteAgent 删除 Agent.
func (h *Handlers) deleteAgent(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAgent(id); err != nil {
		if err == ErrAgentNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// ========== 备份任务 API ==========

// createTask 创建备份任务.
func (h *Handlers) createTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	task, err := h.manager.CreateTask(req)
	if err != nil {
		if err == ErrAgentNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(task))
}

// listTasks 列出备份任务.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, Success(tasks))
}

// getTask 获取任务详情.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(task))
}

// updateTask 更新备份任务.
func (h *Handlers) updateTask(c *gin.Context) {
	id := c.Param("id")
	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	task, err := h.manager.UpdateTask(id, req)
	if err != nil {
		if err == ErrTaskNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(task))
}

// deleteTask 删除备份任务.
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		if err == ErrTaskNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		if err == ErrTaskRunning {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// runTask 手动执行任务.
func (h *Handlers) runTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.RunTask(id)
	if err != nil {
		switch err {
		case ErrTaskNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrTaskRunning:
			c.JSON(http.StatusConflict, Error(409, err.Error()))
		case ErrAgentNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrAgentOffline:
			c.JSON(http.StatusServiceUnavailable, Error(503, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, Success(task))
}

// cancelTask 取消执行.
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CancelTask(id); err != nil {
		switch err {
		case ErrTaskNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrTaskNotRunning:
			c.JSON(http.StatusConflict, Error(409, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// ========== 恢复 API ==========

// listRestorePoints 列出恢复点.
func (h *Handlers) listRestorePoints(c *gin.Context) {
	taskID := c.Query("task_id")
	points := h.manager.ListRestorePoints(taskID)
	c.JSON(http.StatusOK, Success(points))
}

// restoreFull 整机恢复.
func (h *Handlers) restoreFull(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	req.RestoreType = RestoreTypeFull

	job, err := h.manager.CreateRestoreJob(req)
	if err != nil {
		switch err {
		case ErrRestorePointNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrAgentNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrAgentOffline:
			c.JSON(http.StatusServiceUnavailable, Error(503, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusCreated, Success(job))
}

// restoreFiles 文件恢复.
func (h *Handlers) restoreFiles(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	req.RestoreType = RestoryTypeFiles

	job, err := h.manager.CreateRestoreJob(req)
	if err != nil {
		switch err {
		case ErrRestorePointNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrAgentNotFound:
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
		case ErrAgentOffline:
			c.JSON(http.StatusServiceUnavailable, Error(503, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusCreated, Success(job))
}

// browseRestorePoint 浏览恢复点.
func (h *Handlers) browseRestorePoint(c *gin.Context) {
	pointID := c.Param("pointId")
	path := c.Query("path")

	items, err := h.manager.BrowseRestorePoint(pointID, path)
	if err != nil {
		if err == ErrRestorePointNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(items))
}

// ========== 统计 API ==========

// getStats 获取备份统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, Success(stats))
}

// getStorageUsage 获取存储使用情况.
func (h *Handlers) getStorageUsage(c *gin.Context) {
	usage := h.manager.GetStorageUsage()
	c.JSON(http.StatusOK, Success(usage))
}

// ========== Agent 心跳处理 ==========

// HandleHeartbeat 处理 Agent 心跳（供 Agent 调用）.
func (h *Handlers) HandleHeartbeat(c *gin.Context) {
	var req AgentHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.ProcessHeartbeat(req); err != nil {
		if err == ErrAgentNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	// 返回 Agent 配置
	config := AgentConfig{
		HeartbeatInterval: 60,
		BandwidthLimit:    0,
		CompressionLevel:  3,
		EncryptionEnabled: false,
		RetryCount:        3,
		RetryInterval:     30,
	}

	c.JSON(http.StatusOK, Success(config))
}

// HandleTaskComplete 处理任务完成（供 Agent 调用）.
func (h *Handlers) HandleTaskComplete(c *gin.Context) {
	var req struct {
		TaskID         string `json:"task_id" binding:"required"`
		Success        bool   `json:"success"`
		RestorePointID string `json:"restore_point_id"`
		ErrorMsg       string `json:"error_msg"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	h.manager.CompleteTask(req.TaskID, req.Success, req.RestorePointID, req.ErrorMsg)
	c.JSON(http.StatusOK, Success(nil))
}

// HandleProgress 处理进度上报（供 Agent 调用）.
func (h *Handlers) HandleProgress(c *gin.Context) {
	var req struct {
		TaskID     string  `json:"task_id" binding:"required"`
		Progress   float64 `json:"progress"`
		SpeedBytes uint64  `json:"speed_bytes"`
		BytesDone  uint64  `json:"bytes_done"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	h.manager.mu.Lock()
	if task, exists := h.manager.tasks[req.TaskID]; exists {
		task.Progress = req.Progress
		task.SpeedBytes = req.SpeedBytes
		task.Transferred = req.BytesDone
		task.UpdatedAt = time.Now()
	}
	h.manager.mu.Unlock()

	c.JSON(http.StatusOK, Success(nil))
}
