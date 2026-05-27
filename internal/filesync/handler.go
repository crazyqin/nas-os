package filesync

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 文件同步HTTP处理器
type Handler struct {
	manager *SyncManager
	logger  *zap.Logger
}

// NewHandler 创建同步处理器
func NewHandler(manager *SyncManager, logger *zap.Logger) *Handler {
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	sync := r.Group("/filesync")
	{
		// 同步任务
		sync.GET("/tasks", h.ListTasks)
		sync.POST("/tasks", h.CreateTask)
		sync.GET("/tasks/:id", h.GetTask)
		sync.PUT("/tasks/:id", h.UpdateTask)
		sync.DELETE("/tasks/:id", h.DeleteTask)

		// 同步控制
		sync.POST("/tasks/:id/start", h.StartSync)
		sync.POST("/tasks/:id/stop", h.StopSync)

		// 文件管理
		sync.GET("/tasks/:id/files", h.GetSyncFiles)
		sync.POST("/tasks/:id/files", h.RecordSyncFile)

		// 冲突管理
		sync.GET("/tasks/:id/conflicts", h.GetConflicts)
		sync.POST("/tasks/:id/conflicts", h.ReportConflict)
		sync.PUT("/conflicts/:id/resolve", h.ResolveConflict)

		// 版本管理
		sync.GET("/files/:id/versions", h.GetVersions)
		sync.POST("/files/:id/versions", h.AddVersion)

		// 设备管理
		sync.GET("/devices", h.ListDevices)
		sync.POST("/devices", h.RegisterDevice)

		// 统计
		sync.GET("/tasks/:id/stats", h.GetSyncStats)
	}
}

func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": len(tasks)})
}

func (h *Handler) CreateTask(c *gin.Context) {
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreateTask(c.Request.Context(), &task); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task.ID = id
	if err := h.manager.UpdateTask(c.Request.Context(), &task); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}

func (h *Handler) StartSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartSync(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync started"})
}

func (h *Handler) StopSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopSync(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync paused"})
}

func (h *Handler) GetSyncFiles(c *gin.Context) {
	id := c.Param("id")
	files := h.manager.GetSyncFiles(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handler) RecordSyncFile(c *gin.Context) {
	taskID := c.Param("id")
	var file SyncFile
	if err := c.ShouldBindJSON(&file); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	file.TaskID = taskID
	if err := h.manager.RecordSyncFile(c.Request.Context(), &file); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, file)
}

func (h *Handler) GetConflicts(c *gin.Context) {
	id := c.Param("id")
	conflicts := h.manager.GetConflicts(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts, "total": len(conflicts)})
}

func (h *Handler) ReportConflict(c *gin.Context) {
	taskID := c.Param("id")
	var conflict SyncConflict
	if err := c.ShouldBindJSON(&conflict); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	conflict.TaskID = taskID
	if err := h.manager.ReportConflict(c.Request.Context(), &conflict); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conflict)
}

func (h *Handler) ResolveConflict(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.ResolveConflict(c.Request.Context(), id, req.Resolution); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "conflict resolved"})
}

func (h *Handler) GetVersions(c *gin.Context) {
	fileID := c.Param("id")
	versions := h.manager.GetVersions(c.Request.Context(), fileID)
	c.JSON(http.StatusOK, gin.H{"versions": versions, "total": len(versions)})
}

func (h *Handler) AddVersion(c *gin.Context) {
	fileID := c.Param("id")
	var version SyncVersion
	if err := c.ShouldBindJSON(&version); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version.FileID = fileID
	if err := h.manager.AddVersion(c.Request.Context(), &version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, version)
}

func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"devices": devices, "total": len(devices)})
}

func (h *Handler) RegisterDevice(c *gin.Context) {
	var device DeviceInfo
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.RegisterDevice(c.Request.Context(), &device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, device)
}

func (h *Handler) GetSyncStats(c *gin.Context) {
	id := c.Param("id")
	stats := h.manager.GetSyncStats(c.Request.Context(), id)
	c.JSON(http.StatusOK, stats)
}
