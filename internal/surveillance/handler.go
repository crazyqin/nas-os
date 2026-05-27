package surveillance

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 监控HTTP处理器
type Handler struct {
	manager *SurveillanceManager
	logger  *zap.Logger
}

// NewHandler 创建监控处理器
func NewHandler(manager *SurveillanceManager, logger *zap.Logger) *Handler {
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	surveillance := r.Group("/surveillance")
	{
		// 摄像头管理
		surveillance.GET("/cameras", h.ListCameras)
		surveillance.POST("/cameras", h.AddCamera)
		surveillance.GET("/cameras/:id", h.GetCamera)
		surveillance.PUT("/cameras/:id", h.UpdateCamera)
		surveillance.DELETE("/cameras/:id", h.RemoveCamera)

		// 录像控制
		surveillance.POST("/cameras/:id/record/start", h.StartRecording)
		surveillance.POST("/cameras/:id/record/stop", h.StopRecording)
		surveillance.GET("/cameras/:id/recordings", h.GetRecordings)

		// 移动侦测
		surveillance.GET("/cameras/:id/motions", h.GetMotionEvents)
		surveillance.POST("/cameras/:id/motions", h.ReportMotion)

		// 录像计划
		surveillance.POST("/cameras/:id/schedules", h.SetRecordingSchedule)

		// 时间线
		surveillance.GET("/cameras/:id/timeline", h.GetTimeline)

		// 存储配额
		surveillance.GET("/cameras/:id/quota", h.GetStorageQuota)

		// 系统状态
		surveillance.GET("/status", h.GetStatus)
	}
}

func (h *Handler) ListCameras(c *gin.Context) {
	cameras := h.manager.ListCameras(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"cameras": cameras, "total": len(cameras)})
}

func (h *Handler) AddCamera(c *gin.Context) {
	var cam Camera
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.AddCamera(c.Request.Context(), &cam); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cam)
}

func (h *Handler) GetCamera(c *gin.Context) {
	id := c.Param("id")
	cam, err := h.manager.GetCamera(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cam)
}

func (h *Handler) UpdateCamera(c *gin.Context) {
	id := c.Param("id")
	var cam Camera
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cam.ID = id
	if err := h.manager.UpdateCamera(c.Request.Context(), &cam); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cam)
}

func (h *Handler) RemoveCamera(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveCamera(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "camera removed"})
}

func (h *Handler) StartRecording(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartRecording(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "recording started"})
}

func (h *Handler) StopRecording(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopRecording(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "recording stopped"})
}

func (h *Handler) GetRecordings(c *gin.Context) {
	id := c.Param("id")
	startStr := c.DefaultQuery("start", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)

	recordings := h.manager.GetRecordings(c.Request.Context(), id, start, end)
	c.JSON(http.StatusOK, gin.H{"recordings": recordings, "total": len(recordings)})
}

func (h *Handler) GetMotionEvents(c *gin.Context) {
	id := c.Param("id")
	startStr := c.DefaultQuery("start", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)

	events := h.manager.GetMotionEvents(c.Request.Context(), id, start, end)
	c.JSON(http.StatusOK, gin.H{"events": events, "total": len(events)})
}

func (h *Handler) ReportMotion(c *gin.Context) {
	id := c.Param("id")
	var event MotionEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	event.CameraID = id
	if err := h.manager.ReportMotion(c.Request.Context(), &event); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, event)
}

func (h *Handler) SetRecordingSchedule(c *gin.Context) {
	id := c.Param("id")
	var schedule RecordingSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	schedule.CameraID = id
	if err := h.manager.SetRecordingSchedule(c.Request.Context(), &schedule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

func (h *Handler) GetTimeline(c *gin.Context) {
	id := c.Param("id")
	dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	date, _ := time.Parse("2006-01-02", dateStr)

	timeline := h.manager.GetTimeline(c.Request.Context(), id, date)
	c.JSON(http.StatusOK, timeline)
}

func (h *Handler) GetStorageQuota(c *gin.Context) {
	id := c.Param("id")
	quota, err := h.manager.GetStorageQuota(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quota)
}

func (h *Handler) GetStatus(c *gin.Context) {
	cameras := h.manager.ListCameras(c.Request.Context())
	online := 0
	recording := 0
	for _, cam := range cameras {
		if cam.Status == "online" || cam.Status == "recording" {
			online++
		}
		if cam.Status == "recording" {
			recording++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total_cameras":    len(cameras),
		"online_cameras":   online,
		"recording_cameras": recording,
		"storage_path":     h.manager.storagePath,
	})
}
