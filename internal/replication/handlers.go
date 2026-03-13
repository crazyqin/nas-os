package replication

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 复制 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	repl := api.Group("/replications")
	{
		repl.GET("", h.list)
		repl.GET("/stats", h.getStats)
		repl.GET("/:id", h.get)
		repl.POST("", h.create)
		repl.PUT("/:id", h.update)
		repl.DELETE("/:id", h.delete)

		repl.POST("/:id/sync", h.startSync)
		repl.POST("/:id/pause", h.pause)
		repl.POST("/:id/resume", h.resume)
	}
}

// APIResponse 通用响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name             string `json:"name" binding:"required"`
	SourcePath       string `json:"source_path" binding:"required"`
	TargetPath       string `json:"target_path" binding:"required"`
	TargetHost       string `json:"target_host"`
	Type             string `json:"type" binding:"required"`
	Schedule         string `json:"schedule"`
	Enabled          bool   `json:"enabled"`
	Compress         bool   `json:"compress"`
	DeleteExtraneous bool   `json:"delete_extraneous"`
}

// create 创建复制任务
func (h *Handlers) create(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	task := &ReplicationTask{
		Name:             req.Name,
		SourcePath:       req.SourcePath,
		TargetPath:       req.TargetPath,
		TargetHost:       req.TargetHost,
		Type:             ReplicationType(req.Type),
		Schedule:         req.Schedule,
		Enabled:          req.Enabled,
		Compress:         req.Compress,
		DeleteExtraneous: req.DeleteExtraneous,
	}

	if err := h.manager.CreateTask(task); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, success(task))
}

// list 列出所有任务
func (h *Handlers) list(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, success(tasks))
}

// get 获取任务详情
func (h *Handlers) get(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiError(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(task))
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Name             *string `json:"name"`
	Schedule         *string `json:"schedule"`
	Enabled          *bool   `json:"enabled"`
	Compress         *bool   `json:"compress"`
	DeleteExtraneous *bool   `json:"delete_extraneous"`
}

// update 更新任务
func (h *Handlers) update(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Schedule != nil {
		updates["schedule"] = *req.Schedule
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Compress != nil {
		updates["compress"] = *req.Compress
	}
	if req.DeleteExtraneous != nil {
		updates["delete_extraneous"] = *req.DeleteExtraneous
	}

	if err := h.manager.UpdateTask(id, updates); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// delete 删除任务
func (h *Handlers) delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// startSync 手动触发同步
func (h *Handlers) startSync(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartSync(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(map[string]string{
		"status": "sync_started",
	}))
}

// pause 暂停任务
func (h *Handlers) pause(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseTask(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// resume 恢复任务
func (h *Handlers) resume(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeTask(id); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, success(stats))
}

// TaskResponse 任务响应
type TaskResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	SourcePath       string    `json:"source_path"`
	TargetPath       string    `json:"target_path"`
	TargetHost       string    `json:"target_host"`
	Type             string    `json:"type"`
	Status           string    `json:"status"`
	Schedule         string    `json:"schedule"`
	LastSyncAt       time.Time `json:"last_sync_at"`
	NextSyncAt       time.Time `json:"next_sync_at"`
	Progress         float64   `json:"progress"`
	BytesTransferred int64     `json:"bytes_transferred"`
	TotalBytes       int64     `json:"total_bytes"`
	FilesCount       int       `json:"files_count"`
	ErrorMessage     string    `json:"error_message"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
}
