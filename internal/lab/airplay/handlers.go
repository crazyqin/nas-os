// Package airplay 提供 AirPlay 音视频投射服务功能
// HTTP API handlers
package airplay

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler AirPlay HTTP 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	airplay := rg.Group("/airplay")
	{
		// 服务状态
		airplay.GET("/status", h.GetStatus)
		airplay.POST("/start", h.StartService)
		airplay.POST("/stop", h.StopService)

		// 设备管理
		airplay.GET("/devices", h.ListDevices)
		airplay.POST("/devices/refresh", h.RefreshDevices)
		airplay.GET("/devices/:id", h.GetDevice)

		// 接收器
		airplay.GET("/receiver", h.GetReceiver)
		airplay.PUT("/receiver", h.UpdateReceiver)

		// 发送器
		airplay.GET("/sender", h.GetSender)
		airplay.POST("/sender/cast", h.Cast)
		airplay.POST("/sender/stop", h.StopCast)

		// 音频控制
		airplay.GET("/audio/queue", h.GetAudioQueue)
		airplay.POST("/audio/queue", h.AddToQueue)
		airplay.POST("/audio/play", h.PlayAudio)
		airplay.POST("/audio/pause", h.PauseAudio)
		airplay.POST("/audio/next", h.NextTrack)
		airplay.POST("/audio/prev", h.PrevTrack)
		airplay.PUT("/audio/volume", h.SetVolume)

		// 视频控制
		airplay.POST("/video/cast", h.CastVideo)
		airplay.POST("/video/stop", h.StopVideo)

		// 屏幕镜像
		airplay.POST("/mirror/start", h.StartMirror)
		airplay.POST("/mirror/stop", h.StopMirror)

		// 多房间音频
		airplay.GET("/groups", h.ListGroups)
		airplay.POST("/groups", h.CreateGroup)
		airplay.PUT("/groups/:id", h.UpdateGroup)
		airplay.DELETE("/groups/:id", h.DeleteGroup)

		// 设备配对
		airplay.GET("/pairing", h.ListPairings)
		airplay.POST("/pairing/:id/trust", h.TrustDevice)
		airplay.DELETE("/pairing/:id", h.UnpairDevice)

		// 统计
		airplay.GET("/stats", h.GetStats)
	}
}

// ========== 服务状态 ==========

// GetStatus handles GET /api/v1/airplay/status
// 获取 AirPlay 服务状态.
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// StartService handles POST /api/v1/airplay/start
// 启动 AirPlay 服务.
func (h *Handler) StartService(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 服务已启动")
	c.JSON(http.StatusOK, gin.H{"message": "AirPlay 服务已启动"})
}

// StopService handles POST /api/v1/airplay/stop
// 停止 AirPlay 服务.
func (h *Handler) StopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 服务已停止")
	c.JSON(http.StatusOK, gin.H{"message": "AirPlay 服务已停止"})
}

// ========== 设备管理 ==========

// ListDevices handles GET /api/v1/airplay/devices
// 列出所有 AirPlay 设备.
func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// RefreshDevices handles POST /api/v1/airplay/devices/refresh
// 刷新设备列表.
func (h *Handler) RefreshDevices(c *gin.Context) {
	devices := h.manager.RefreshDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// GetDevice handles GET /api/v1/airplay/devices/:id
// 获取设备详情.
func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, device)
}

// ========== 接收器 ==========

// GetReceiver handles GET /api/v1/airplay/receiver
// 获取接收器配置.
func (h *Handler) GetReceiver(c *gin.Context) {
	receiver := h.manager.GetReceiver()
	c.JSON(http.StatusOK, receiver)
}

// UpdateReceiverRequest 更新接收器配置请求.
type UpdateReceiverRequest struct {
	Name              string `json:"name"`
	Enabled           bool   `json:"enabled"`
	Port              int    `json:"port"`
	PasswordProtected bool   `json:"passwordProtected"`
	Password          string `json:"password"`
}

// UpdateReceiver handles PUT /api/v1/airplay/receiver
// 更新接收器配置.
func (h *Handler) UpdateReceiver(c *gin.Context) {
	var req UpdateReceiverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateReceiver(req.Name, req.Enabled, req.Port, req.PasswordProtected, req.Password)

	h.logger.Info("[AirPlay API] 接收器配置已更新")
	c.JSON(http.StatusOK, gin.H{"message": "接收器配置已更新"})
}

// ========== 发送器 ==========

// GetSender handles GET /api/v1/airplay/sender
// 获取发送器状态.
func (h *Handler) GetSender(c *gin.Context) {
	sender := h.manager.GetSender()
	c.JSON(http.StatusOK, sender)
}

// CastRequest 投射请求.
type CastRequest struct {
	TargetID string     `json:"targetId" binding:"required"`
	Media    *MediaInfo `json:"media"`
}

// Cast handles POST /api/v1/airplay/sender/cast
// 发起投射.
func (h *Handler) Cast(c *gin.Context) {
	var req CastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.Cast(req.TargetID, req.Media); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 投射已开始", zap.String("target", req.TargetID))
	c.JSON(http.StatusOK, gin.H{"message": "投射已开始"})
}

// StopCast handles POST /api/v1/airplay/sender/stop
// 停止投射.
func (h *Handler) StopCast(c *gin.Context) {
	if err := h.manager.StopCast(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 投射已停止")
	c.JSON(http.StatusOK, gin.H{"message": "投射已停止"})
}

// ========== 音频控制 ==========

// GetAudioQueue handles GET /api/v1/airplay/audio/queue
// 获取播放队列.
func (h *Handler) GetAudioQueue(c *gin.Context) {
	streamID := c.Query("streamId")
	if streamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 streamId 参数"})
		return
	}

	queue, err := h.manager.GetAudioQueue(streamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queue": queue,
		"total": len(queue),
	})
}

// AddToQueueRequest 添加到队列请求.
type AddToQueueRequest struct {
	StreamID string    `json:"streamId" binding:"required"`
	Media    MediaInfo `json:"media" binding:"required"`
}

// AddToQueue handles POST /api/v1/airplay/audio/queue
// 添加到播放队列.
func (h *Handler) AddToQueue(c *gin.Context) {
	var req AddToQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddToQueue(req.StreamID, req.Media); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 已添加到队列", zap.String("title", req.Media.Title))
	c.JSON(http.StatusOK, gin.H{"message": "已添加到队列"})
}

// AudioControlRequest 音频控制请求.
type AudioControlRequest struct {
	StreamID string `json:"streamId" binding:"required"`
}

// PlayAudio handles POST /api/v1/airplay/audio/play
// 播放音频.
func (h *Handler) PlayAudio(c *gin.Context) {
	var req AudioControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.PlayAudio(req.StreamID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "播放"})
}

// PauseAudio handles POST /api/v1/airplay/audio/pause
// 暂停音频.
func (h *Handler) PauseAudio(c *gin.Context) {
	var req AudioControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.PauseAudio(req.StreamID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已暂停"})
}

// NextTrack handles POST /api/v1/airplay/audio/next
// 下一曲.
func (h *Handler) NextTrack(c *gin.Context) {
	var req AudioControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.NextTrack(req.StreamID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "下一曲"})
}

// PrevTrack handles POST /api/v1/airplay/audio/prev
// 上一曲.
func (h *Handler) PrevTrack(c *gin.Context) {
	var req AudioControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.PrevTrack(req.StreamID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "上一曲"})
}

// SetVolumeRequest 设置音量请求.
type SetVolumeRequest struct {
	StreamID string `json:"streamId" binding:"required"`
	Volume   int    `json:"volume" binding:"required"`
}

// SetVolume handles PUT /api/v1/airplay/audio/volume
// 设置音量.
func (h *Handler) SetVolume(c *gin.Context) {
	var req SetVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SetVolume(req.StreamID, req.Volume); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "音量已设置"})
}

// ========== 视频控制 ==========

// CastVideoRequest 视频投射请求.
type CastVideoRequest struct {
	TargetID   string     `json:"targetId" binding:"required"`
	Media      *MediaInfo `json:"media"`
	Resolution string     `json:"resolution"`
	Bitrate    int        `json:"bitrate"`
}

// CastVideo handles POST /api/v1/airplay/video/cast
// 视频投射.
func (h *Handler) CastVideo(c *gin.Context) {
	var req CastVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CastVideo(req.TargetID, req.Media, req.Resolution, req.Bitrate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 视频投射已开始", zap.String("target", req.TargetID))
	c.JSON(http.StatusOK, gin.H{"message": "视频投射已开始"})
}

// StopVideoRequest 停止视频请求.
type StopVideoRequest struct {
	StreamID string `json:"streamId" binding:"required"`
}

// StopVideo handles POST /api/v1/airplay/video/stop
// 停止视频投射.
func (h *Handler) StopVideo(c *gin.Context) {
	var req StopVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.StopVideo(req.StreamID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "视频投射已停止"})
}

// ========== 屏幕镜像 ==========

// StartMirrorRequest 开始镜像请求.
type StartMirrorRequest struct {
	SourceID   string `json:"sourceId" binding:"required"`
	TargetID   string `json:"targetId" binding:"required"`
	Resolution string `json:"resolution"`
	FrameRate  int    `json:"frameRate"`
}

// StartMirror handles POST /api/v1/airplay/mirror/start
// 开始屏幕镜像.
func (h *Handler) StartMirror(c *gin.Context) {
	var req StartMirrorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.StartMirror(req.SourceID, req.TargetID, req.Resolution, req.FrameRate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 屏幕镜像已开始")
	c.JSON(http.StatusOK, gin.H{"message": "屏幕镜像已开始"})
}

// StopMirrorRequest 停止镜像请求.
type StopMirrorRequest struct {
	MirrorID string `json:"mirrorId" binding:"required"`
}

// StopMirror handles POST /api/v1/airplay/mirror/stop
// 停止屏幕镜像.
func (h *Handler) StopMirror(c *gin.Context) {
	var req StopMirrorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.StopMirror(req.MirrorID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "屏幕镜像已停止"})
}

// ========== 多房间音频 ==========

// ListGroups handles GET /api/v1/airplay/groups
// 列出多房间组.
func (h *Handler) ListGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// CreateGroupRequest 创建组请求.
type CreateGroupRequest struct {
	Name     string   `json:"name" binding:"required"`
	MasterID string   `json:"masterId" binding:"required"`
	SlaveIDs []string `json:"slaveIds"`
}

// CreateGroup handles POST /api/v1/airplay/groups
// 创建多房间组.
func (h *Handler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.manager.CreateGroup(req.Name, req.MasterID, req.SlaveIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 多房间组已创建", zap.String("name", req.Name))
	c.JSON(http.StatusCreated, group)
}

// UpdateGroupRequest 更新组请求.
type UpdateGroupRequest struct {
	Name     string   `json:"name"`
	SlaveIDs []string `json:"slaveIds"`
}

// UpdateGroup handles PUT /api/v1/airplay/groups/:id
// 更新多房间组.
func (h *Handler) UpdateGroup(c *gin.Context) {
	groupID := c.Param("id")

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateGroup(groupID, req.Name, req.SlaveIDs); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 多房间组已更新", zap.String("id", groupID))
	c.JSON(http.StatusOK, gin.H{"message": "多房间组已更新"})
}

// DeleteGroup handles DELETE /api/v1/airplay/groups/:id
// 删除多房间组.
func (h *Handler) DeleteGroup(c *gin.Context) {
	groupID := c.Param("id")

	if err := h.manager.DeleteGroup(groupID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 多房间组已删除", zap.String("id", groupID))
	c.JSON(http.StatusOK, gin.H{"message": "多房间组已删除"})
}

// ========== 设备配对 ==========

// ListPairings handles GET /api/v1/airplay/pairing
// 列出配对设备.
func (h *Handler) ListPairings(c *gin.Context) {
	pairings := h.manager.ListPairings()
	c.JSON(http.StatusOK, gin.H{
		"pairings": pairings,
		"total":    len(pairings),
	})
}

// TrustDevice handles POST /api/v1/airplay/pairing/:id/trust
// 信任设备.
func (h *Handler) TrustDevice(c *gin.Context) {
	deviceID := c.Param("id")

	if err := h.manager.TrustDevice(deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 设备已信任", zap.String("deviceID", deviceID))
	c.JSON(http.StatusOK, gin.H{"message": "设备已信任"})
}

// UnpairDevice handles DELETE /api/v1/airplay/pairing/:id
// 取消配对.
func (h *Handler) UnpairDevice(c *gin.Context) {
	deviceID := c.Param("id")

	if err := h.manager.UnpairDevice(deviceID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[AirPlay API] 设备已取消配对", zap.String("deviceID", deviceID))
	c.JSON(http.StatusOK, gin.H{"message": "设备已取消配对"})
}

// ========== 统计 ==========

// GetStats handles GET /api/v1/airplay/stats
// 获取统计信息.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
