package cloudsyncmgr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 多云同步 HTTP API 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册 API 路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sync := rg.Group("/cloudsyncmgr")
	{
		// 任务管理
		sync.POST("/tasks", h.CreateTask)
		sync.GET("/tasks", h.ListTasks)
		sync.GET("/tasks/:id", h.GetTask)
		sync.PUT("/tasks/:id", h.UpdateTask)
		sync.DELETE("/tasks/:id", h.DeleteTask)

		// 同步控制
		sync.POST("/tasks/:id/sync", h.StartSync)
		sync.POST("/tasks/:id/stop", h.StopSync)
		sync.POST("/tasks/:id/pause", h.PauseTask)
		sync.POST("/tasks/:id/resume", h.ResumeTask)

		// 状态查询
		sync.GET("/status", h.GetAllStatus)
		sync.GET("/tasks/:id/status", h.GetStatus)

		// 元信息
		sync.GET("/providers", h.ListProviders)
	}
}

// createTaskReq 创建任务请求.
type createTaskReq struct {
	Name             string            `json:"name" binding:"required"`
	Provider         ProviderType      `json:"provider" binding:"required"`
	ProviderConfig   map[string]string `json:"provider_config" binding:"required"`
	LocalPath        string            `json:"local_path" binding:"required"`
	RemotePath       string            `json:"remote_path" binding:"required"`
	Direction        SyncDirection     `json:"direction" binding:"required"`
	ConflictPolicy   ConflictPolicy    `json:"conflict_policy"`
	ScheduleMode     ScheduleMode      `json:"schedule_mode"`
	ScheduleInterval int64             `json:"schedule_interval_sec,omitempty"` // 秒
	ScheduleCron     string            `json:"schedule_cron,omitempty"`
	BandwidthLimit   int64             `json:"bandwidth_limit"`
	EncryptInTransit bool              `json:"encrypt_in_transit"`
	FilterPatterns   []string          `json:"filter_patterns,omitempty"`
	MaxRetries       int               `json:"max_retries"`
}

// CreateTask handles POST /api/v1/cloudsyncmgr/tasks.
func (h *Handler) CreateTask(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := SyncConfig{
		Name:             req.Name,
		Enabled:          true,
		Provider:         req.Provider,
		ProviderConfig:   req.ProviderConfig,
		LocalPath:        req.LocalPath,
		RemotePath:       req.RemotePath,
		Direction:        req.Direction,
		ConflictPolicy:   req.ConflictPolicy,
		ScheduleMode:     req.ScheduleMode,
		ScheduleCron:     req.ScheduleCron,
		BandwidthLimit:   req.BandwidthLimit,
		EncryptInTransit: req.EncryptInTransit,
		FilterPatterns:   req.FilterPatterns,
		MaxRetries:       req.MaxRetries,
	}

	// 设置默认值
	if config.ConflictPolicy == "" {
		config.ConflictPolicy = ConflictNewest
	}
	if config.ScheduleMode == "" {
		config.ScheduleMode = ScheduleManual
	}
	if req.ScheduleInterval > 0 {
		config.ScheduleInterval = time.Duration(req.ScheduleInterval) * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	task, err := h.manager.CreateTask(config)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /api/v1/cloudsyncmgr/tasks.
func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// GetTask handles GET /api/v1/cloudsyncmgr/tasks/:id.
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// UpdateTask handles PUT /api/v1/cloudsyncmgr/tasks/:id.
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")

	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := SyncConfig{
		Name:             req.Name,
		Enabled:          true,
		Provider:         req.Provider,
		ProviderConfig:   req.ProviderConfig,
		LocalPath:        req.LocalPath,
		RemotePath:       req.RemotePath,
		Direction:        req.Direction,
		ConflictPolicy:   req.ConflictPolicy,
		ScheduleMode:     req.ScheduleMode,
		ScheduleCron:     req.ScheduleCron,
		BandwidthLimit:   req.BandwidthLimit,
		EncryptInTransit: req.EncryptInTransit,
		FilterPatterns:   req.FilterPatterns,
		MaxRetries:       req.MaxRetries,
	}

	if config.ConflictPolicy == "" {
		config.ConflictPolicy = ConflictNewest
	}
	if config.ScheduleMode == "" {
		config.ScheduleMode = ScheduleManual
	}
	if req.ScheduleInterval > 0 {
		config.ScheduleInterval = time.Duration(req.ScheduleInterval) * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	task, err := h.manager.UpdateTask(id, config)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DeleteTask handles DELETE /api/v1/cloudsyncmgr/tasks/:id.
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除"})
}

// StartSync handles POST /api/v1/cloudsyncmgr/tasks/:id/sync.
func (h *Handler) StartSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartSync(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步已启动"})
}

// StopSync handles POST /api/v1/cloudsyncmgr/tasks/:id/stop.
func (h *Handler) StopSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步已停止"})
}

// PauseTask handles POST /api/v1/cloudsyncmgr/tasks/:id/pause.
func (h *Handler) PauseTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseTask(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已暂停"})
}

// ResumeTask handles POST /api/v1/cloudsyncmgr/tasks/:id/resume.
func (h *Handler) ResumeTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResumeTask(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已恢复"})
}

// GetAllStatus handles GET /api/v1/cloudsyncmgr/status.
func (h *Handler) GetAllStatus(c *gin.Context) {
	statuses := h.manager.GetAllStatus()
	c.JSON(http.StatusOK, gin.H{
		"statuses": statuses,
		"total":    len(statuses),
	})
}

// GetStatus handles GET /api/v1/cloudsyncmgr/tasks/:id/status.
func (h *Handler) GetStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.manager.GetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// ListProviders handles GET /api/v1/cloudsyncmgr/providers.
func (h *Handler) ListProviders(c *gin.Context) {
	providers := SupportedProviders()
	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
		"total":     len(providers),
	})
}
