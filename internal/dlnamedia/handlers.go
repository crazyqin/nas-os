package dlnamedia

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建新的处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	dlna := rg.Group("/dlna")
	{
		// 设备管理
		dlna.GET("/devices", h.ListDevices)
		dlna.GET("/devices/:id", h.GetDevice)
		dlna.POST("/devices/discover", h.DiscoverDevices)

		// 媒体库管理
		dlna.GET("/libraries", h.ListLibraries)
		dlna.POST("/libraries", h.CreateLibrary)
		dlna.GET("/libraries/:id", h.GetLibrary)
		dlna.PUT("/libraries/:id", h.UpdateLibrary)
		dlna.DELETE("/libraries/:id", h.DeleteLibrary)
		dlna.POST("/libraries/:id/scan", h.ScanLibrary)

		// 媒体搜索和浏览
		dlna.GET("/media/search", h.SearchMedia)
		dlna.GET("/media/:id", h.GetMediaItem)
		dlna.GET("/content-directory", h.GetContentDirectory)

		// 投屏和播放控制
		dlna.POST("/playback/push", h.PushMedia)
		dlna.GET("/playback/:sessionId", h.GetPlaybackStatus)
		dlna.POST("/playback/:sessionId/control", h.ControlPlayback)
		dlna.POST("/playback/:sessionId/volume", h.SetVolume)
		dlna.DELETE("/playback/:sessionId", h.StopSession)
		dlna.GET("/playback/sessions", h.ListSessions)

		// 播放队列
		dlna.GET("/queues/:deviceId", h.GetQueue)
		dlna.POST("/queues/:deviceId", h.ManageQueue)

		// 设备分组
		dlna.GET("/groups", h.ListGroups)
		dlna.POST("/groups", h.CreateGroup)
		dlna.GET("/groups/:id", h.GetGroup)
		dlna.PUT("/groups/:id", h.UpdateGroup)
		dlna.DELETE("/groups/:id", h.DeleteGroup)
	}
}

// ========== 设备管理 ==========

// ListDevices 列出所有设备.
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// GetDevice 获取设备详情.
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, device)
}

// DiscoverDevices 发现设备.
func (h *Handler) DiscoverDevices(c *gin.Context) {
	var req DiscoverDevicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认超时
		req.Timeout = 5
	}

	if req.Timeout <= 0 || req.Timeout > 30 {
		req.Timeout = 5
	}

	devices := h.manager.DiscoverDevices(req.Timeout)
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
		"timeout": req.Timeout,
	})
}

// ========== 媒体库管理 ==========

// ListLibraries 列出所有媒体库.
func (h *Handler) ListLibraries(c *gin.Context) {
	libraries := h.manager.ListLibraries()
	c.JSON(http.StatusOK, gin.H{
		"libraries": libraries,
		"total":     len(libraries),
	})
}

// CreateLibrary 创建媒体库.
func (h *Handler) CreateLibrary(c *gin.Context) {
	var req CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认值
	if req.MediaType == "" {
		req.MediaType = MediaTypeVideo
	}
	if req.ScanInterval <= 0 {
		req.ScanInterval = 60 // 默认 60 分钟
	}

	lib, err := h.manager.CreateLibrary(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, lib)
}

// GetLibrary 获取媒体库详情.
func (h *Handler) GetLibrary(c *gin.Context) {
	id := c.Param("id")
	lib, err := h.manager.GetLibrary(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lib)
}

// UpdateLibrary 更新媒体库.
func (h *Handler) UpdateLibrary(c *gin.Context) {
	id := c.Param("id")
	var req UpdateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lib, err := h.manager.UpdateLibrary(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, lib)
}

// DeleteLibrary 删除媒体库.
func (h *Handler) DeleteLibrary(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteLibrary(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Library deleted"})
}

// ScanLibrary 扫描媒体库.
func (h *Handler) ScanLibrary(c *gin.Context) {
	id := c.Param("id")
	var req ScanLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认不强制扫描
		req.Force = false
	}

	req.LibraryID = id
	if err := h.manager.ScanLibrary(id, req.Force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scan completed"})
}

// ========== 媒体搜索和浏览 ==========

// SearchMedia 搜索媒体.
func (h *Handler) SearchMedia(c *gin.Context) {
	var req SearchMediaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	items, total := h.manager.SearchMedia(req)
	c.JSON(http.StatusOK, gin.H{
		"items":      items,
		"total":      total,
		"page":       req.Page,
		"page_size":  req.PageSize,
	})
}

// GetMediaItem 获取媒体项.
func (h *Handler) GetMediaItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.manager.GetMediaItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// GetContentDirectory 获取内容目录.
func (h *Handler) GetContentDirectory(c *gin.Context) {
	parentID := c.Query("parent_id")
	items := h.manager.GetContentDirectory(parentID)
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": len(items),
	})
}

// ========== 投屏和播放控制 ==========

// PushMedia 推送媒体到设备.
func (h *Handler) PushMedia(c *gin.Context) {
	var req PushMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.PushMedia(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// GetPlaybackStatus 获取播放状态.
func (h *Handler) GetPlaybackStatus(c *gin.Context) {
	sessionID := c.Param("sessionId")
	session, err := h.manager.GetPlaybackStatus(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// ControlPlayback 控制播放.
func (h *Handler) ControlPlayback(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req ControlPlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.ControlPlayback(sessionID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// SetVolume 设置音量.
func (h *Handler) SetVolume(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req SetVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetVolume(sessionID, req.Level); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Volume set"})
}

// StopSession 停止播放会话.
func (h *Handler) StopSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := h.manager.StopSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Session stopped"})
}

// ListSessions 列出所有播放会话.
func (h *Handler) ListSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// ========== 播放队列 ==========

// GetQueue 获取播放队列.
func (h *Handler) GetQueue(c *gin.Context) {
	deviceID := c.Param("deviceId")
	queue, err := h.manager.GetQueue(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, queue)
}

// ManageQueue 管理播放队列.
func (h *Handler) ManageQueue(c *gin.Context) {
	deviceID := c.Param("deviceId")
	var req ManageQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	queue, err := h.manager.ManageQueue(deviceID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, queue)
}

// ========== 设备分组 ==========

// ListGroups 列出所有设备分组.
func (h *Handler) ListGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// CreateGroup 创建设备分组.
func (h *Handler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.manager.CreateGroup(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// GetGroup 获取设备分组.
func (h *Handler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	group, err := h.manager.GetGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// UpdateGroup 更新设备分组.
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.manager.UpdateGroup(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup 删除设备分组.
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Group deleted"})
}

// ========== 辅助函数 ==========

// parseIntParam 解析整数参数.
func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	valStr := c.Query(name)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
