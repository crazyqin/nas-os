package downloader

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/downloader/tasks", h.ListTasks)
	r.POST("/downloader/tasks", h.CreateTask)
	r.GET("/downloader/tasks/:id", h.GetTask)
	r.PUT("/downloader/tasks/:id", h.UpdateTask)
	r.DELETE("/downloader/tasks/:id", h.DeleteTask)
	r.POST("/downloader/tasks/:id/start", h.StartTask)
	r.POST("/downloader/tasks/:id/pause", h.PauseTask)
	r.POST("/downloader/tasks/:id/resume", h.ResumeTask)
	r.GET("/downloader/stats", h.GetStats)
}

// APIResponse API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// writeError 写入错误响应
func writeError(c *gin.Context, status int, message string) {
	writeJSON(c, status, APIResponse{
		Success: false,
		Error:   message,
	})
}

// writeSuccess 写入成功响应
func writeSuccess(c *gin.Context, data interface{}) {
	writeJSON(c, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// ListTasks 列出任务
func (h *Handler) ListTasks(c *gin.Context) {
	status := DownloadStatus(c.Query("status"))
	tasks := h.manager.ListTasks(status)
	writeSuccess(c, tasks)
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.URL == "" {
		writeError(c, http.StatusBadRequest, "URL 不能为空")
		return
	}

	task, err := h.manager.CreateTask(req)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, task)
}

// GetTask 获取任务
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, exists := h.manager.GetTask(id)
	if !exists {
		writeError(c, http.StatusNotFound, "任务不存在")
		return
	}

	writeSuccess(c, task)
}

// UpdateTask 更新任务
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "无效的请求体")
		return
	}

	task, err := h.manager.UpdateTask(id, req)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, task)
}

// DeleteTask 删除任务
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	deleteFiles := strings.ToLower(c.Query("delete_files")) == "true"

	if err := h.manager.DeleteTask(id, deleteFiles); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, nil)
}

// StartTask 启动任务
func (h *Handler) StartTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartTask(id); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, nil)
}

// PauseTask 暂停任务
func (h *Handler) PauseTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseTask(id); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, nil)
}

// ResumeTask 恢复任务
func (h *Handler) ResumeTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeTask(id); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(c, nil)
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	writeSuccess(c, stats)
}
