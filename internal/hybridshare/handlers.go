// Package hybridshare provides cloud-local hybrid storage management.
package hybridshare

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Handler HTTP 处理器
// ============================================================

// Handler 混合共享 HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建新的 Handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	share := r.Group("/hybridshare")
	{
		// 混合共享 CRUD
		share.POST("", h.CreateShare)
		share.GET("", h.ListShares)
		share.GET("/:id", h.GetShare)
		share.PUT("/:id", h.UpdateShare)
		share.DELETE("/:id", h.DeleteShare)

		// 文件操作
		share.GET("/:id/files", h.ListFiles)
		share.GET("/:id/files/*path", h.GetFileMetadata)

		// 缓存操作
		share.POST("/:id/cache", h.CacheFile)
		share.DELETE("/:id/cache/*path", h.EvictFromCache)
		share.POST("/:id/cache/pin", h.PinFile)
		share.DELETE("/:id/cache/pin/*path", h.UnpinFile)

		// 同步操作
		share.POST("/:id/sync", h.StartSync)
		share.GET("/:id/sync/tasks", h.ListSyncTasks)
		share.GET("/sync/tasks/:taskId", h.GetSyncTask)
		share.POST("/sync/tasks/:taskId/cancel", h.CancelSyncTask)

		// 统计
		share.GET("/:id/stats/capacity", h.GetCapacityStats)
		share.GET("/:id/stats/bandwidth", h.GetBandwidthStats)

		// 日志
		share.GET("/:id/logs/sync", h.GetSyncLogs)
		share.GET("/:id/logs/events", h.GetEventLogs)
	}
}

// ============================================================
// 混合共享 CRUD Handlers
// ============================================================

// CreateShare 创建混合共享
// @Summary 创建混合共享
// @Tags hybridshare
// @Accept json
// @Produce json
// @Param request body CreateShareRequest true "创建请求"
// @Success 201 {object} HybridShareConfig
// @Router /api/v1/hybridshare [post]
func (h *Handler) CreateShare(c *gin.Context) {
	var req CreateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.service.CreateShare(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// ListShares 列出所有混合共享
// @Summary 列出混合共享
// @Tags hybridshare
// @Produce json
// @Success 200 {array} ShareSummary
// @Router /api/v1/hybridshare [get]
func (h *Handler) ListShares(c *gin.Context) {
	summaries := h.service.ListShares()
	c.JSON(http.StatusOK, summaries)
}

// GetShare 获取混合共享详情
// @Summary 获取混合共享
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Success 200 {object} HybridShareConfig
// @Router /api/v1/hybridshare/{id} [get]
func (h *Handler) GetShare(c *gin.Context) {
	id := c.Param("id")

	config, err := h.service.GetShare(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateShare 更新混合共享
// @Summary 更新混合共享
// @Tags hybridshare
// @Accept json
// @Produce json
// @Param id path string true "共享ID"
// @Param request body UpdateShareRequest true "更新请求"
// @Success 200 {object} HybridShareConfig
// @Router /api/v1/hybridshare/{id} [put]
func (h *Handler) UpdateShare(c *gin.Context) {
	id := c.Param("id")

	var req UpdateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.service.UpdateShare(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteShare 删除混合共享
// @Summary 删除混合共享
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/{id} [delete]
func (h *Handler) DeleteShare(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteShare(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "share deleted successfully"})
}

// ============================================================
// 文件操作 Handlers
// ============================================================

// ListFiles 列出文件
// @Summary 列出文件
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param path query string false "路径前缀"
// @Success 200 {array} FileMetadata
// @Router /api/v1/hybridshare/{id}/files [get]
func (h *Handler) ListFiles(c *gin.Context) {
	shareID := c.Param("id")
	path := c.DefaultQuery("path", "")

	files, err := h.service.ListFiles(shareID, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, files)
}

// GetFileMetadata 获取文件元数据
// @Summary 获取文件元数据
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param path path string true "文件路径"
// @Success 200 {object} FileMetadata
// @Router /api/v1/hybridshare/{id}/files/{path} [get]
func (h *Handler) GetFileMetadata(c *gin.Context) {
	shareID := c.Param("id")
	filePath := c.Param("path")
	if filePath != "" && filePath[0] == '/' {
		filePath = filePath[1:]
	}

	meta, err := h.service.GetFileMetadata(shareID, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, meta)
}

// ============================================================
// 缓存操作 Handlers
// ============================================================

// CacheFileRequest 缓存文件请求
type CacheFileRequest struct {
	ShareID  string `json:"share_id" binding:"required"`
	FilePath string `json:"file_path" binding:"required"`
}

// CacheFile 缓存文件到本地
// @Summary 缓存文件
// @Tags hybridshare
// @Accept json
// @Produce json
// @Param id path string true "共享ID"
// @Param request body CacheFileRequest true "缓存请求"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/{id}/cache [post]
func (h *Handler) CacheFile(c *gin.Context) {
	shareID := c.Param("id")

	var req CacheFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 尝试从查询参数获取
		req.ShareID = shareID
		req.FilePath = c.Query("path")
		if req.FilePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file_path is required"})
			return
		}
	}

	if req.ShareID == "" {
		req.ShareID = shareID
	}

	if err := h.service.CacheFile(req.ShareID, req.FilePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file cached successfully"})
}

// EvictFromCache 从缓存驱逐文件
// @Summary 驱逐缓存
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param path path string true "文件路径"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/{id}/cache/{path} [delete]
func (h *Handler) EvictFromCache(c *gin.Context) {
	shareID := c.Param("id")
	filePath := c.Param("path")
	if filePath != "" && filePath[0] == '/' {
		filePath = filePath[1:]
	}

	if err := h.service.EvictFromCache(shareID, filePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file evicted from cache"})
}

// PinFileRequest 固定文件请求
type PinFileRequest struct {
	ShareID  string `json:"share_id" binding:"required"`
	FilePath string `json:"file_path" binding:"required"`
}

// PinFile 固定文件到缓存
// @Summary 固定文件
// @Tags hybridshare
// @Accept json
// @Produce json
// @Param id path string true "共享ID"
// @Param request body PinFileRequest true "固定请求"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/{id}/cache/pin [post]
func (h *Handler) PinFile(c *gin.Context) {
	shareID := c.Param("id")

	var req PinFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ShareID == "" {
		req.ShareID = shareID
	}

	if err := h.service.PinFile(req.ShareID, req.FilePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file pinned successfully"})
}

// UnpinFile 取消固定文件
// @Summary 取消固定
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param path path string true "文件路径"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/{id}/cache/pin/{path} [delete]
func (h *Handler) UnpinFile(c *gin.Context) {
	shareID := c.Param("id")
	filePath := c.Param("path")
	if filePath != "" && filePath[0] == '/' {
		filePath = filePath[1:]
	}

	if err := h.service.UnpinFile(shareID, filePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file unpinned successfully"})
}

// ============================================================
// 同步操作 Handlers
// ============================================================

// StartSync 启动同步
// @Summary 启动同步
// @Tags hybridshare
// @Accept json
// @Produce json
// @Param id path string true "共享ID"
// @Param request body SyncRequest true "同步请求"
// @Success 202 {object} SyncTask
// @Router /api/v1/hybridshare/{id}/sync [post]
func (h *Handler) StartSync(c *gin.Context) {
	shareID := c.Param("id")

	var req SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认同步全部
		req = SyncRequest{
			ShareID:   shareID,
			Direction: SyncDirectionUpload,
		}
	}

	if req.ShareID == "" {
		req.ShareID = shareID
	}

	task, err := h.service.StartSync(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, task)
}

// ListSyncTasks 列出同步任务
// @Summary 列出同步任务
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Success 200 {array} SyncTask
// @Router /api/v1/hybridshare/{id}/sync/tasks [get]
func (h *Handler) ListSyncTasks(c *gin.Context) {
	shareID := c.Param("id")

	tasks := h.service.ListSyncTasks(shareID)
	c.JSON(http.StatusOK, tasks)
}

// GetSyncTask 获取同步任务详情
// @Summary 获取同步任务
// @Tags hybridshare
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} SyncTask
// @Router /api/v1/hybridshare/sync/tasks/{taskId} [get]
func (h *Handler) GetSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")

	task, err := h.service.GetSyncTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// CancelSyncTask 取消同步任务
// @Summary 取消同步任务
// @Tags hybridshare
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/hybridshare/sync/tasks/{taskId}/cancel [post]
func (h *Handler) CancelSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")

	if err := h.service.CancelSyncTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sync task cancelled"})
}

// ============================================================
// 统计 Handlers
// ============================================================

// GetCapacityStats 获取容量统计
// @Summary 容量统计
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Success 200 {object} CapacityStats
// @Router /api/v1/hybridshare/{id}/stats/capacity [get]
func (h *Handler) GetCapacityStats(c *gin.Context) {
	shareID := c.Param("id")

	stats, err := h.service.GetCapacityStats(shareID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetBandwidthStats 获取带宽统计
// @Summary 带宽统计
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Success 200 {object} BandwidthStats
// @Router /api/v1/hybridshare/{id}/stats/bandwidth [get]
func (h *Handler) GetBandwidthStats(c *gin.Context) {
	shareID := c.Param("id")

	stats, err := h.service.GetBandwidthStats(shareID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ============================================================
// 日志 Handlers
// ============================================================

// GetSyncLogs 获取同步日志
// @Summary 同步日志
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param limit query int false "返回数量限制"
// @Success 200 {array} SyncLog
// @Router /api/v1/hybridshare/{id}/logs/sync [get]
func (h *Handler) GetSyncLogs(c *gin.Context) {
	shareID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	logs := h.service.GetSyncLogs(shareID, limit)
	c.JSON(http.StatusOK, logs)
}

// GetEventLogs 获取事件日志
// @Summary 事件日志
// @Tags hybridshare
// @Produce json
// @Param id path string true "共享ID"
// @Param limit query int false "返回数量限制"
// @Success 200 {array} EventLog
// @Router /api/v1/hybridshare/{id}/logs/events [get]
func (h *Handler) GetEventLogs(c *gin.Context) {
	shareID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	logs := h.service.GetEventLogs(shareID, limit)
	c.JSON(http.StatusOK, logs)
}
