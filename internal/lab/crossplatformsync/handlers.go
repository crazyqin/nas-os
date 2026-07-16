package crossplatformsync

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 跨平台同步 HTTP handlers.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建跨平台同步 HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册跨平台同步 API 路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sync := rg.Group("/crossplatformsync")
	{
		// 设备管理
		sync.POST("/devices", h.CreateDevice)
		sync.GET("/devices", h.ListDevices)
		sync.GET("/devices/:id", h.GetDevice)
		sync.PUT("/devices/:id", h.UpdateDevice)
		sync.DELETE("/devices/:id", h.DeleteDevice)
		sync.POST("/devices/:id/test", h.TestDeviceConnection)

		// 同步任务管理
		sync.POST("/tasks", h.CreateSyncTask)
		sync.GET("/tasks", h.ListSyncTasks)
		sync.GET("/tasks/:id", h.GetSyncTask)
		sync.PUT("/tasks/:id", h.UpdateSyncTask)
		sync.DELETE("/tasks/:id", h.DeleteSyncTask)

		// 同步控制
		sync.POST("/tasks/:id/start", h.StartSync)
		sync.POST("/tasks/:id/pause", h.PauseSync)
		sync.POST("/tasks/:id/resume", h.ResumeSync)
		sync.POST("/tasks/:id/stop", h.StopSync)

		// 冲突管理
		sync.GET("/tasks/:id/conflicts", h.GetConflicts)
		sync.POST("/tasks/:id/conflicts/:conflictId/resolve", h.ResolveConflict)
		sync.POST("/tasks/:id/conflicts/resolve-all", h.ResolveAllConflicts)

		// 状态和统计
		sync.GET("/tasks/:id/status", h.GetSyncStatus)
		sync.GET("/statuses", h.GetAllStatuses)
		sync.GET("/stats", h.GetSyncStats)
		sync.GET("/logs", h.GetSyncLogs)

		// Mock数据
		sync.POST("/mock", h.LoadMockData)
	}
}

// ============================================================
// 设备管理 Handlers
// ============================================================

// CreateDevice 处理 POST /api/v1/crossplatformsync/devices.
func (h *Handler) CreateDevice(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	device, err := h.manager.CreateDevice(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": device})
}

// GetDevice 处理 GET /api/v1/crossplatformsync/devices/:id.
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": device})
}

// ListDevices 处理 GET /api/v1/crossplatformsync/devices.
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": devices, "total": len(devices)})
}

// UpdateDevice 处理 PUT /api/v1/crossplatformsync/devices/:id.
func (h *Handler) UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	device, err := h.manager.UpdateDevice(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": device})
}

// DeleteDevice 处理 DELETE /api/v1/crossplatformsync/devices/:id.
func (h *Handler) DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDevice(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// TestDeviceConnection 处理 POST /api/v1/crossplatformsync/devices/:id/test.
func (h *Handler) TestDeviceConnection(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.TestDeviceConnection(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// ============================================================
// 同步任务管理 Handlers
// ============================================================

// CreateSyncTask 处理 POST /api/v1/crossplatformsync/tasks.
func (h *Handler) CreateSyncTask(c *gin.Context) {
	var req CreateSyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	task, err := h.manager.CreateSyncTask(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// GetSyncTask 处理 GET /api/v1/crossplatformsync/tasks/:id.
func (h *Handler) GetSyncTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetSyncTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// ListSyncTasks 处理 GET /api/v1/crossplatformsync/tasks.
func (h *Handler) ListSyncTasks(c *gin.Context) {
	tasks := h.manager.ListSyncTasks()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks, "total": len(tasks)})
}

// UpdateSyncTask 处理 PUT /api/v1/crossplatformsync/tasks/:id.
func (h *Handler) UpdateSyncTask(c *gin.Context) {
	id := c.Param("id")
	var req UpdateSyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	task, err := h.manager.UpdateSyncTask(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// DeleteSyncTask 处理 DELETE /api/v1/crossplatformsync/tasks/:id.
func (h *Handler) DeleteSyncTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSyncTask(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// ============================================================
// 同步控制 Handlers
// ============================================================

// StartSync 处理 POST /api/v1/crossplatformsync/tasks/:id/start.
func (h *Handler) StartSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sync started"})
}

// PauseSync 处理 POST /api/v1/crossplatformsync/tasks/:id/pause.
func (h *Handler) PauseSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sync paused"})
}

// ResumeSync 处理 POST /api/v1/crossplatformsync/tasks/:id/resume.
func (h *Handler) ResumeSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResumeSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sync resumed"})
}

// StopSync 处理 POST /api/v1/crossplatformsync/tasks/:id/stop.
func (h *Handler) StopSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sync stopped"})
}

// ============================================================
// 冲突管理 Handlers
// ============================================================

// GetConflicts 处理 GET /api/v1/crossplatformsync/tasks/:id/conflicts.
func (h *Handler) GetConflicts(c *gin.Context) {
	id := c.Param("id")
	conflicts := h.manager.GetConflicts(id)
	if conflicts == nil {
		conflicts = make([]*FileConflict, 0)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": conflicts, "total": len(conflicts)})
}

// ResolveConflict 处理 POST /api/v1/crossplatformsync/tasks/:id/conflicts/:conflictId/resolve.
func (h *Handler) ResolveConflict(c *gin.Context) {
	taskID := c.Param("id")
	conflictID := c.Param("conflictId")
	var req ResolveConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	if err := h.manager.ResolveConflict(taskID, conflictID, req.Resolution); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "conflict resolved"})
}

// ResolveAllConflicts 处理 POST /api/v1/crossplatformsync/tasks/:id/conflicts/resolve-all.
func (h *Handler) ResolveAllConflicts(c *gin.Context) {
	taskID := c.Param("id")
	var req ResolveConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}
	if err := h.manager.ResolveAllConflicts(taskID, req.Resolution); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "all conflicts resolved"})
}

// ============================================================
// 状态和统计 Handlers
// ============================================================

// GetSyncStatus 处理 GET /api/v1/crossplatformsync/tasks/:id/status.
func (h *Handler) GetSyncStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.manager.GetSyncStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// GetAllStatuses 处理 GET /api/v1/crossplatformsync/statuses.
func (h *Handler) GetAllStatuses(c *gin.Context) {
	statuses := h.manager.GetAllSyncStatuses()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": statuses, "total": len(statuses)})
}

// GetSyncStats 处理 GET /api/v1/crossplatformsync/stats.
func (h *Handler) GetSyncStats(c *gin.Context) {
	stats := h.manager.GetSyncStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// GetSyncLogs 处理 GET /api/v1/crossplatformsync/logs.
func (h *Handler) GetSyncLogs(c *gin.Context) {
	taskID := c.Query("task_id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}
	logs := h.manager.GetSyncLogs(taskID, limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs, "total": len(logs)})
}

// LoadMockData 处理 POST /api/v1/crossplatformsync/mock.
func (h *Handler) LoadMockData(c *gin.Context) {
	h.manager.LoadMockData()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "mock data loaded"})
}
