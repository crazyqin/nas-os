// Package smartcam 提供 REST API 处理器
package smartcam

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// response 标准响应结构
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler 摄像头 API 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	cam := r.Group("/surveillance/cameras")
	{
		// 摄像头管理
		cam.GET("", h.ListCameras)
		cam.POST("", h.AddCamera)
		cam.GET("/:id", h.GetCamera)
		cam.PUT("/:id", h.UpdateCamera)
		cam.DELETE("/:id", h.RemoveCamera)

		// 摄像头发现
		cam.POST("/discover", h.DiscoverCameras)

		// 摄像头控制
		cam.POST("/:id/enable", h.EnableCamera)
		cam.POST("/:id/disable", h.DisableCamera)
	}

	// 录像管理
	rec := r.Group("/surveillance/recordings")
	{
		rec.GET("", h.ListRecordings)
		rec.POST("/start", h.StartRecording)
		rec.POST("/stop", h.StopRecording)
		rec.DELETE("/:id", h.DeleteRecording)
	}

	// 移动侦测
	motion := r.Group("/surveillance/motion")
	{
		motion.GET("/events", h.GetMotionEvents)
		motion.POST("/trigger", h.TriggerMotion)
	}

	// 系统状态
	sys := r.Group("/surveillance/system")
	{
		sys.GET("/status", h.GetStatus)
		sys.GET("/storage", h.GetStorageStats)
		sys.GET("/config", h.GetConfig)
		sys.PUT("/config", h.UpdateConfig)
	}
}

// ========== 摄像头管理 API ==========

// ListCameras 列出所有摄像头
func (h *Handler) ListCameras(c *gin.Context) {
	cameras := h.manager.ListCameras()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(cameras),
			"cameras":  cameras,
		},
	})
}

// AddCamera 添加摄像头
func (h *Handler) AddCamera(c *gin.Context) {
	var req AddCameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	camera, err := h.manager.AddCamera(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "摄像头已添加", Data: camera})
}

// GetCamera 获取摄像头详情
func (h *Handler) GetCamera(c *gin.Context) {
	id := c.Param("id")
	camera, err := h.manager.GetCamera(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: camera})
}

// UpdateCamera 更新摄像头
func (h *Handler) UpdateCamera(c *gin.Context) {
	id := c.Param("id")
	var req UpdateCameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	camera, err := h.manager.UpdateCamera(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "摄像头已更新", Data: camera})
}

// RemoveCamera 移除摄像头
func (h *Handler) RemoveCamera(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveCamera(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "摄像头已移除"})
}

// DiscoverCameras 发现摄像头
func (h *Handler) DiscoverCameras(c *gin.Context) {
	subnet := c.Query("subnet")
	if subnet == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "请提供子网参数 (subnet)"})
		return
	}

	result, err := h.manager.DiscoverCameras(subnet)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "发现完成", Data: result})
}

// EnableCamera 启用摄像头
func (h *Handler) EnableCamera(c *gin.Context) {
	id := c.Param("id")
	enabled := true
	camera, err := h.manager.UpdateCamera(id, UpdateCameraRequest{Enabled: &enabled})
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "摄像头已启用", Data: camera})
}

// DisableCamera 禁用摄像头
func (h *Handler) DisableCamera(c *gin.Context) {
	id := c.Param("id")
	enabled := false
	camera, err := h.manager.UpdateCamera(id, UpdateCameraRequest{Enabled: &enabled})
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "摄像头已禁用", Data: camera})
}

// ========== 录像管理 API ==========

// ListRecordings 列出录像
func (h *Handler) ListRecordings(c *gin.Context) {
	cameraID := c.Query("cameraId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	recordings := h.manager.GetRecordings(cameraID, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(recordings),
			"recordings": recordings,
		},
	})
}

// StartRecording 开始录像
func (h *Handler) StartRecording(c *gin.Context) {
	var req StartRecordingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	recording, err := h.manager.StartRecording(req.CameraID, req.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "录像已开始", Data: recording})
}

// StopRecording 停止录像
func (h *Handler) StopRecording(c *gin.Context) {
	var req struct {
		CameraID string `json:"cameraId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	recording, err := h.manager.StopRecording(req.CameraID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "录像已停止", Data: recording})
}

// DeleteRecording 删除录像
func (h *Handler) DeleteRecording(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRecording(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "录像已删除"})
}

// ========== 移动侦测 API ==========

// GetMotionEvents 获取移动侦测事件
func (h *Handler) GetMotionEvents(c *gin.Context) {
	query := MotionEventQuery{
		CameraID:  c.Query("cameraId"),
		StartTime: c.Query("startTime"),
		EndTime:   c.Query("endTime"),
	}
	limitStr := c.DefaultQuery("limit", "50")
	if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
		query.Limit = limit
	}

	events := h.manager.GetMotionEvents(query)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// TriggerMotion 手动触发移动侦测
func (h *Handler) TriggerMotion(c *gin.Context) {
	var req struct {
		CameraID   string       `json:"cameraId" binding:"required"`
		ZoneID     string       `json:"zoneId"`
		Confidence float64      `json:"confidence"`
		BoundingBox *BoundingBox `json:"boundingBox"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}

	if req.Confidence == 0 {
		req.Confidence = 0.8
	}

	event, err := h.manager.TriggerMotionEvent(req.CameraID, req.ZoneID, req.Confidence, req.BoundingBox)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "移动侦测事件已触发", Data: event})
}

// ========== 系统 API ==========

// GetStatus 获取系统状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: status})
}

// GetStorageStats 获取存储统计
func (h *Handler) GetStorageStats(c *gin.Context) {
	stats := h.manager.GetStorageStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// GetConfig 获取系统配置
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// UpdateConfig 更新系统配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var cfg SystemConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "无效请求: " + err.Error()})
		return
	}
	h.manager.UpdateConfig(cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "配置已更新"})
}
