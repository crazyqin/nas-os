package cloudsync

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 云同步 HTTP handlers
type Handler struct {
	manager *CloudSyncManager
	logger  *zap.Logger
}

// NewHandler 创建云同步 HTTP handler
func NewHandler(manager *CloudSyncManager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// NewHandlers 创建云同步 HTTP handler（兼容旧接口）
func NewHandlers(manager *Manager) *Handler {
	return NewHandler(manager, zap.NewNop())
}

// RegisterRoutes 注册云同步 API 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sync := rg.Group("/cloudsync")
	{
		// 提供商管理
		sync.POST("/providers", h.CreateProvider)
		sync.GET("/providers", h.ListProviders)
		sync.GET("/providers/:id", h.GetProvider)
		sync.PUT("/providers/:id", h.UpdateProvider)
		sync.DELETE("/providers/:id", h.DeleteProvider)
		sync.POST("/providers/:id/test", h.TestProvider)

		// 连接管理
		sync.POST("/connections", h.CreateConnection)
		sync.GET("/connections", h.ListConnections)
		sync.GET("/connections/:id", h.GetConnection)
		sync.PUT("/connections/:id", h.UpdateConnection)
		sync.DELETE("/connections/:id", h.DeleteConnection)

		// 同步任务管理
		sync.POST("/tasks", h.CreateTask)
		sync.GET("/tasks", h.ListTasks)
		sync.GET("/tasks/:id", h.GetTask)
		sync.PUT("/tasks/:id", h.UpdateTask)
		sync.DELETE("/tasks/:id", h.DeleteTask)

		// 同步控制
		sync.POST("/tasks/:id/start", h.StartSync)
		sync.POST("/tasks/:id/pause", h.PauseSync)
		sync.POST("/tasks/:id/resume", h.ResumeSync)
		sync.POST("/tasks/:id/stop", h.StopSync)

		// 状态和统计
		sync.GET("/tasks/:id/status", h.GetSyncStatus)
		sync.GET("/stats", h.GetSyncStats)
		sync.GET("/logs", h.GetSyncLogs)
		sync.GET("/connections/:id/usage", h.GetStorageUsage)

		// Mock数据
		sync.POST("/mock", h.LoadMockData)
	}
}

// ============================================================
// 提供商管理 Handlers
// ============================================================

// CreateProvider 处理 POST /api/v1/cloudsync/providers
func (h *Handler) CreateProvider(c *gin.Context) {
	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}

	// 如果没有设置 Type，使用 Provider 字段
	if config.Type == "" {
		config.Type = config.Provider
	}

	// 验证必需字段
	if config.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": "type is required"})
		return
	}

	provider, err := h.manager.CreateProvider(config)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": provider})
}

// GetProvider 处理 GET /api/v1/cloudsync/providers/:id
func (h *Handler) GetProvider(c *gin.Context) {
	id := c.Param("id")
	provider, err := h.manager.GetProvider(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": provider})
}

// ListProviders 处理 GET /api/v1/cloudsync/providers
func (h *Handler) ListProviders(c *gin.Context) {
	providers := h.manager.ListProviders()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": providers, "total": len(providers)})
}

// UpdateProvider 处理 PUT /api/v1/cloudsync/providers/:id
func (h *Handler) UpdateProvider(c *gin.Context) {
	id := c.Param("id")
	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "error": err.Error()})
		return
	}

	if err := h.manager.UpdateProvider(id, config); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}

	provider, _ := h.manager.GetProvider(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": provider})
}

// DeleteProvider 处理 DELETE /api/v1/cloudsync/providers/:id
func (h *Handler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteProvider(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// TestProvider 处理 POST /api/v1/cloudsync/providers/:id/test
func (h *Handler) TestProvider(c *gin.Context) {
	id := c.Param("id")
	_, err := h.manager.GetProvider(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "error": err.Error()})
		return
	}
	// 模拟测试成功
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "connection test successful"})
}

// ============================================================
// 连接管理 Handlers
// ============================================================

// CreateConnection 处理 POST /api/v1/cloudsync/connections
func (h *Handler) CreateConnection(c *gin.Context) {
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conn, err := h.manager.CreateConnection(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, conn)
}

// ListConnections 处理 GET /api/v1/cloudsync/connections
func (h *Handler) ListConnections(c *gin.Context) {
	conns := h.manager.ListConnections()
	c.JSON(http.StatusOK, gin.H{"connections": conns, "total": len(conns)})
}

// GetConnection 处理 GET /api/v1/cloudsync/connections/:id
func (h *Handler) GetConnection(c *gin.Context) {
	id := c.Param("id")
	conn, err := h.manager.GetConnection(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// UpdateConnection 处理 PUT /api/v1/cloudsync/connections/:id
func (h *Handler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")
	var req UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conn, err := h.manager.UpdateConnection(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conn)
}

// DeleteConnection 处理 DELETE /api/v1/cloudsync/connections/:id
func (h *Handler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteConnection(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ============================================================
// 同步任务管理 Handlers
// ============================================================

// CreateTask 处理 POST /api/v1/cloudsync/tasks
func (h *Handler) CreateTask(c *gin.Context) {
	// 先读取 body 为 map 以支持 camelCase
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 支持 camelCase 和 snake_case
	req := CreateTaskRequest{}
	if name, ok := body["name"].(string); ok {
		req.Name = name
	}
	if id, ok := body["providerId"].(string); ok {
		req.ConnectionID = id
	} else if id, ok := body["connection_id"].(string); ok {
		req.ConnectionID = id
	}
	if path, ok := body["localPath"].(string); ok {
		req.LocalPath = path
	} else if path, ok := body["local_path"].(string); ok {
		req.LocalPath = path
	}
	if path, ok := body["remotePath"].(string); ok {
		req.RemotePath = path
	} else if path, ok := body["remote_path"].(string); ok {
		req.RemotePath = path
	}
	if mode, ok := body["direction"].(string); ok {
		req.Mode = SyncMode(mode)
	} else if mode, ok := body["mode"].(string); ok {
		req.Mode = SyncMode(mode)
	}

	if req.Name == "" || req.ConnectionID == "" || req.LocalPath == "" || req.RemotePath == "" || req.Mode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
		return
	}

	task, err := h.manager.CreateTask(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListTasks 处理 GET /api/v1/cloudsync/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": len(tasks)})
}

// GetTask 处理 GET /api/v1/cloudsync/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// UpdateTask 处理 PUT /api/v1/cloudsync/tasks/:id
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.UpdateTask(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DeleteTask 处理 DELETE /api/v1/cloudsync/tasks/:id
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ============================================================
// 同步控制 Handlers
// ============================================================

// StartSync 处理 POST /api/v1/cloudsync/tasks/:id/start
func (h *Handler) StartSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync started"})
}

// PauseSync 处理 POST /api/v1/cloudsync/tasks/:id/pause
func (h *Handler) PauseSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.PauseSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync paused"})
}

// ResumeSync 处理 POST /api/v1/cloudsync/tasks/:id/resume
func (h *Handler) ResumeSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResumeSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync resumed"})
}

// StopSync 处理 POST /api/v1/cloudsync/tasks/:id/stop
func (h *Handler) StopSync(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopSync(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync stopped"})
}

// ============================================================
// 状态和统计 Handlers
// ============================================================

// GetSyncStatus 处理 GET /api/v1/cloudsync/tasks/:id/status
func (h *Handler) GetSyncStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.manager.GetSyncStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// GetSyncStats 处理 GET /api/v1/cloudsync/stats
func (h *Handler) GetSyncStats(c *gin.Context) {
	stats := h.manager.GetSyncStats()
	c.JSON(http.StatusOK, stats)
}

// GetSyncLogs 处理 GET /api/v1/cloudsync/logs
func (h *Handler) GetSyncLogs(c *gin.Context) {
	taskID := c.Query("task_id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	logs := h.manager.GetSyncLogs(taskID, limit)
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": len(logs)})
}

// GetStorageUsage 处理 GET /api/v1/cloudsync/connections/:id/usage
func (h *Handler) GetStorageUsage(c *gin.Context) {
	id := c.Param("id")
	usage, err := h.manager.GetStorageUsage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, usage)
}

// LoadMockData 处理 POST /api/v1/cloudsync/mock
func (h *Handler) LoadMockData(c *gin.Context) {
	h.manager.LoadMockData()
	c.JSON(http.StatusOK, gin.H{"message": "mock data loaded"})
}
