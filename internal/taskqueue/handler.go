package taskqueue

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 任务队列HTTP处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{manager: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	queue := rg.Group("/taskqueue")
	{
		// 管理
		queue.GET("/status", h.GetStatus)
		queue.GET("/stats", h.GetStats)
		queue.POST("/start", h.Start)
		queue.POST("/stop", h.Stop)

		// 任务 CRUD
		queue.POST("/tasks", h.SubmitTask)
		queue.GET("/tasks", h.ListTasks)
		queue.GET("/tasks/:id", h.GetTask)
		queue.POST("/tasks/:id/cancel", h.CancelTask)

		// 死信队列
		queue.GET("/dead-letter", h.GetDeadLetter)
		queue.POST("/dead-letter/:id/retry", h.RetryDeadLetter)
	}
}

// ========== 请求/响应类型 ==========

// SubmitTaskRequest 提交任务请求.
type SubmitTaskRequest struct {
	Name          string                 `json:"name" binding:"required"`
	Description   string                 `json:"description"`
	Priority      string                 `json:"priority"` // low/normal/high/urgent
	Payload       map[string]interface{} `json:"payload"`
	MaxRetries    int                    `json:"max_retries"`
	RetryDelay    string                 `json:"retry_delay"`    // duration string
	BackoffFactor float64                `json:"backoff_factor"`
	Timeout       string                 `json:"timeout"`        // duration string
	Dependencies  []string               `json:"dependencies"`
}

// TaskResponse 任务响应.
type TaskResponse struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Status        string                 `json:"status"`
	Priority      string                 `json:"priority"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
	MaxRetries    int                    `json:"max_retries"`
	RetryCount    int                    `json:"retry_count"`
	RetryDelay    string                 `json:"retry_delay"`
	BackoffFactor float64                `json:"backoff_factor"`
	Timeout       string                 `json:"timeout"`
	Dependencies  []string               `json:"dependencies,omitempty"`
	Progress      float64                `json:"progress"`
	Error         string                 `json:"error,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	StartedAt     *time.Time             `json:"started_at,omitempty"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
}

func toTaskResponse(task *Task) *TaskResponse {
	return &TaskResponse{
		ID:            task.ID,
		Name:          task.Name,
		Description:   task.Description,
		Status:        string(task.Status),
		Priority:      task.Priority.String(),
		Payload:       task.Payload,
		MaxRetries:    task.MaxRetries,
		RetryCount:    task.RetryCount,
		RetryDelay:    task.RetryDelay.String(),
		BackoffFactor: task.BackoffFactor,
		Timeout:       task.Timeout.String(),
		Dependencies:  task.Dependencies,
		Progress:      task.Progress,
		Error:         task.Error,
		CreatedAt:     task.CreatedAt,
		StartedAt:     task.StartedAt,
		CompletedAt:   task.CompletedAt,
	}
}

// ========== Handler 方法 ==========

// GetStatus 获取队列状态.
func (h *Handler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": h.manager.IsRunning(),
		"config":  h.manager.config,
	})
}

// GetStats 获取队列统计.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// Start 启动队列.
func (h *Handler) Start(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务队列已启动"})
}

// Stop 停止队列.
func (h *Handler) Stop(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务队列已停止"})
}

// SubmitTask 提交任务.
func (h *Handler) SubmitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解析优先级
	priority := ParsePriority(req.Priority)

	// 解析时间
	var retryDelay time.Duration
	if req.RetryDelay != "" {
		var err error
		retryDelay, err = time.ParseDuration(req.RetryDelay)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 retry_delay: " + err.Error()})
			return
		}
	}

	var timeout time.Duration
	if req.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 timeout: " + err.Error()})
			return
		}
	}

	opts := &TaskOptions{
		Name:          req.Name,
		Description:   req.Description,
		Priority:      priority,
		Payload:       req.Payload,
		MaxRetries:    req.MaxRetries,
		RetryDelay:    retryDelay,
		BackoffFactor: req.BackoffFactor,
		Timeout:       timeout,
		Dependencies:  req.Dependencies,
	}

	// 无操作的默认 handler（用于演示）
	handler := func(ctx *TaskContext) error {
		// 实际使用时应由调用方提供 handler
		return nil
	}

	task, err := h.manager.Submit(opts, handler)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toTaskResponse(task))
}

// ListTasks 列出任务.
func (h *Handler) ListTasks(c *gin.Context) {
	var filter TaskFilter

	// 解析查询参数
	if status := c.QueryArray("status"); len(status) > 0 {
		for _, s := range status {
			filter.Status = append(filter.Status, TaskStatus(s))
		}
	}
	if priority := c.QueryArray("priority"); len(priority) > 0 {
		for _, p := range priority {
			filter.Priority = append(filter.Priority, ParsePriority(p))
		}
	}
	if name := c.Query("name"); name != "" {
		filter.Name = name
	}

	// 解析分页
	if limit := c.Query("limit"); limit != "" {
		fmt.Sscanf(limit, "%d", &filter.Limit)
	}
	if offset := c.Query("offset"); offset != "" {
		fmt.Sscanf(offset, "%d", &filter.Offset)
	}

	tasks := h.manager.ListTasks(filter)
	result := make([]*TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = toTaskResponse(task)
	}

	c.JSON(http.StatusOK, result)
}

// GetTask 获取任务详情.
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toTaskResponse(task))
}

// CancelTask 取消任务.
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消", "task_id": id})
}

// GetDeadLetter 获取死信队列.
func (h *Handler) GetDeadLetter(c *gin.Context) {
	tasks := h.manager.GetDeadLetter()
	result := make([]*TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = toTaskResponse(task)
	}

	c.JSON(http.StatusOK, result)
}

// RetryDeadLetter 重试死信任务.
func (h *Handler) RetryDeadLetter(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RetryDeadLetter(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已重新入队", "task_id": id})
}
