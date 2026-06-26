// Package clientthumb - HTTP handlers
package clientthumb

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 客户端缩略图引擎 HTTP 处理器
type Handler struct {
	engine *Engine
}

// NewHandler 创建处理器
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	ct := api.Group("/clientthumb")
	{
		// 任务管理
		ct.POST("/submit", h.handleSubmit)
		ct.GET("/task/:id", h.handleGetTask)
		ct.POST("/result", h.handleReportResult)
		ct.POST("/failure", h.handleReportFailure)

		// 客户端管理
		ct.POST("/register", h.handleRegisterClient)
		ct.POST("/unregister", h.handleUnregisterClient)
		ct.POST("/heartbeat", h.handleHeartbeat)
		ct.GET("/clients", h.handleListClients)

		// 统计与监控
		ct.GET("/stats", h.handleStats)
		ct.POST("/prune", h.handlePruneClients)
	}
}

// handleSubmit 提交缩略图生成任务
func (h *Handler) handleSubmit(c *gin.Context) {
	var req struct {
		FileID   string `json:"fileId" binding:"required"`
		FilePath string `json:"filePath" binding:"required"`
		Format   string `json:"format"`
		Size     string `json:"size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.engine.SubmitTask(
		c.Request.Context(),
		req.FileID,
		req.FilePath,
		Format(req.Format),
		Size(req.Size),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"taskId":   task.ID,
		"status":   task.Status,
		"clientId": task.ClientID,
		"format":   task.Format,
		"size":     task.Size,
		"width":    task.Width,
		"height":   task.Height,
	})
}

// handleGetTask 获取任务状态
func (h *Handler) handleGetTask(c *gin.Context) {
	taskID := c.Param("id")
	task, ok := h.engine.GetTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	resp := gin.H{
		"taskId":    task.ID,
		"fileId":    task.FileID,
		"status":    task.Status,
		"clientId":  task.ClientID,
		"format":    task.Format,
		"size":      task.Size,
		"createdAt": task.CreatedAt,
	}
	if task.Result != nil {
		resp["result"] = gin.H{
			"thumbnailPath": task.Result.ThumbnailPath,
			"fileSize":      task.Result.FileSize,
			"width":         task.Result.Width,
			"height":        task.Result.Height,
			"checksum":      task.Result.Checksum,
		}
	}
	if task.Duration > 0 {
		resp["duration"] = task.Duration.String()
	}
	c.JSON(http.StatusOK, resp)
}

// handleReportResult 客户端上报生成结果
func (h *Handler) handleReportResult(c *gin.Context) {
	var req struct {
		TaskID        string `json:"taskId" binding:"required"`
		ThumbnailPath string `json:"thumbnailPath" binding:"required"`
		FileSize      int64  `json:"fileSize"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		Format        string `json:"format"`
		Checksum      string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := &TaskResult{
		ThumbnailPath: req.ThumbnailPath,
		FileSize:      req.FileSize,
		Width:         req.Width,
		Height:        req.Height,
		Format:        Format(req.Format),
		Checksum:      req.Checksum,
	}

	if err := h.engine.ReportResult(req.TaskID, result); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleReportFailure 客户端上报生成失败
func (h *Handler) handleReportFailure(c *gin.Context) {
	var req struct {
		TaskID  string `json:"taskId" binding:"required"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.engine.ReportFailure(req.TaskID, req.Message); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleRegisterClient 注册客户端
func (h *Handler) handleRegisterClient(c *gin.Context) {
	var req struct {
		ClientID  string   `json:"clientId" binding:"required"`
		Formats   []string `json:"formats"`
		MaxSize   int      `json:"maxSize"`
		Hardware  string   `json:"hardware"`
		WebPCodec bool     `json:"webpCodec"`
		AVIFCodec bool     `json:"avifCodec"`
		GPUMemMB  int      `json:"gpuMemMB"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	formats := make([]Format, 0, len(req.Formats))
	for _, f := range req.Formats {
		formats = append(formats, Format(f))
	}

	caps := ClientCaps{
		Formats:   formats,
		MaxSize:   req.MaxSize,
		Hardware:  req.Hardware,
		WebPCodec: req.WebPCodec,
		AVIFCodec: req.AVIFCodec,
		GPUMemMB:  req.GPUMemMB,
	}

	client := h.engine.RegisterClient(req.ClientID, caps)
	c.JSON(http.StatusOK, gin.H{
		"clientId": client.ID,
		"maxTasks": client.MaxTasks,
	})
}

// handleUnregisterClient 注销客户端
func (h *Handler) handleUnregisterClient(c *gin.Context) {
	var req struct {
		ClientID string `json:"clientId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.engine.UnregisterClient(req.ClientID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleHeartbeat 客户端心跳
func (h *Handler) handleHeartbeat(c *gin.Context) {
	var req struct {
		ClientID string `json:"clientId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.engine.Heartbeat(req.ClientID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleListClients 列出所有客户端
func (h *Handler) handleListClients(c *gin.Context) {
	clients := h.engine.ListClients()
	result := make([]gin.H, 0, len(clients))
	for _, cl := range clients {
		cl.mu.Lock()
		result = append(result, gin.H{
			"clientId":       cl.ID,
			"hardware":       cl.Capabilities.Hardware,
			"formats":        cl.Capabilities.Formats,
			"currentTasks":   cl.TaskCount,
			"maxTasks":       cl.MaxTasks,
			"totalGenerated": cl.TotalGenerated,
			"lastHeartbeat":  cl.LastHeartbeat,
		})
		cl.mu.Unlock()
	}
	c.JSON(http.StatusOK, gin.H{"clients": result})
}

// handleStats 获取性能统计
func (h *Handler) handleStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"totalTasks":       stats.TotalTasks,
		"completedTasks":   stats.CompletedTasks,
		"failedTasks":      stats.FailedTasks,
		"avgDuration":      stats.AvgDuration.String(),
		"p95Duration":      stats.P95Duration.String(),
		"totalBytesSaved":  stats.TotalBytesSaved,
		"throughputPerSec": stats.ThroughputPerSec,
		"speedupFactor":    stats.SpeedupFactor,
	})
}

// handlePruneClients 清理超时客户端
func (h *Handler) handlePruneClients(c *gin.Context) {
	pruned := h.engine.PruneStaleClients()
	c.JSON(http.StatusOK, gin.H{"pruned": pruned})
}
