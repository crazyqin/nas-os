package surveillance

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 监控处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建监控处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	surveillance := r.Group("/surveillance")
	{
		// 摄像头管理
		surveillance.GET("/cameras", h.listCameras)
		surveillance.POST("/cameras", h.addCamera)
		surveillance.GET("/cameras/:id", h.getCamera)
		surveillance.PUT("/cameras/:id", h.updateCamera)
		surveillance.DELETE("/cameras/:id", h.deleteCamera)
		surveillance.GET("/cameras/discover", h.discoverCameras)

		// 录制管理
		surveillance.GET("/recordings", h.getRecordings)
		surveillance.POST("/recordings/start", h.startRecording)
		surveillance.POST("/recordings/:id/stop", h.stopRecording)

		// 事件管理
		surveillance.GET("/events", h.getEvents)
		surveillance.POST("/events", h.addEvent)

		// 流媒体
		surveillance.POST("/streams/start", h.startStream)
		surveillance.POST("/streams/:id/stop", h.stopStream)
		surveillance.GET("/streams/active", h.getActiveStreams)

		// 移动侦测
		surveillance.GET("/motion/:cameraId", h.getMotionDetection)
		surveillance.PUT("/motion/:cameraId", h.setMotionDetection)

		// 回放
		surveillance.POST("/playback/query", h.queryPlayback)

		// 导出
		surveillance.POST("/exports", h.createExport)
		surveillance.GET("/exports/:id", h.getExportJob)

		// 存储配额
		surveillance.GET("/storage/:cameraId", h.getStorageQuota)
		surveillance.PUT("/storage/:cameraId", h.setStorageQuota)

		// PTZ 控制
		surveillance.POST("/ptz", h.sendPTZCommand)

		// 录制计划
		surveillance.GET("/schedules", h.listSchedules)
		surveillance.POST("/schedules", h.addSchedule)
		surveillance.DELETE("/schedules/:id", h.deleteSchedule)

		// 统计
		surveillance.GET("/stats", h.getStats)
	}
}

// listCameras 列出摄像头.
func (h *Handlers) listCameras(c *gin.Context) {
	cameras := h.manager.ListCameras()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cameras,
	})
}

// addCamera 添加摄像头.
func (h *Handlers) addCamera(c *gin.Context) {
	var cam Camera
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddCamera(&cam); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    cam,
	})
}

// getCamera 获取摄像头.
func (h *Handlers) getCamera(c *gin.Context) {
	id := c.Param("id")
	cam, err := h.manager.GetCamera(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cam,
	})
}

// updateCamera 更新摄像头.
func (h *Handlers) updateCamera(c *gin.Context) {
	id := c.Param("id")
	var cam Camera
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	cam.ID = id
	if err := h.manager.UpdateCamera(&cam); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cam,
	})
}

// deleteCamera 删除摄像头.
func (h *Handlers) deleteCamera(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteCamera(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// discoverCameras 发现摄像头.
func (h *Handlers) discoverCameras(c *gin.Context) {
	results, err := h.manager.DiscoverCameras()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    results,
	})
}

// getRecordings 获取录制列表.
func (h *Handlers) getRecordings(c *gin.Context) {
	cameraID := c.Query("cameraId")
	recordings := h.manager.GetRecordings(cameraID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    recordings,
	})
}

// startRecording 开始录制.
func (h *Handlers) startRecording(c *gin.Context) {
	var req struct {
		CameraID string       `json:"cameraId"`
		Mode     RecordingMode `json:"mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	job, err := h.manager.StartRecording(req.CameraID, req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    job,
	})
}

// stopRecording 停止录制.
func (h *Handlers) stopRecording(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopRecording(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getEvents 获取事件列表.
func (h *Handlers) getEvents(c *gin.Context) {
	cameraID := c.Query("cameraId")
	limit := 100 // 默认限制

	events := h.manager.GetEvents(cameraID, limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    events,
	})
}

// addEvent 添加事件.
func (h *Handlers) addEvent(c *gin.Context) {
	var req struct {
		CameraID string `json:"cameraId"`
		Message  string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	h.manager.AddEvent(req.CameraID, req.Message)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// startStream 开始流媒体.
func (h *Handlers) startStream(c *gin.Context) {
	var req struct {
		CameraID string `json:"cameraId"`
		Protocol string `json:"protocol"`
		ClientID string `json:"clientId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	session, err := h.manager.StartStream(req.CameraID, req.Protocol, req.ClientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    session,
	})
}

// stopStream 停止流媒体.
func (h *Handlers) stopStream(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopStream(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getActiveStreams 获取活跃流.
func (h *Handlers) getActiveStreams(c *gin.Context) {
	streams := h.manager.GetActiveStreams()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    streams,
	})
}

// getMotionDetection 获取移动侦测配置.
func (h *Handlers) getMotionDetection(c *gin.Context) {
	cameraID := c.Param("cameraId")
	cfg, err := h.manager.GetMotionDetection(cameraID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cfg,
	})
}

// setMotionDetection 设置移动侦测配置.
func (h *Handlers) setMotionDetection(c *gin.Context) {
	cameraID := c.Param("cameraId")
	var cfg MotionDetectionConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	cfg.CameraID = cameraID
	if err := h.manager.SetMotionDetection(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// queryPlayback 查询回放.
func (h *Handlers) queryPlayback(c *gin.Context) {
	var req PlaybackQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	segments, err := h.manager.GetPlayback(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    segments,
	})
}

// createExport 创建导出任务.
func (h *Handlers) createExport(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	job, err := h.manager.CreateExport(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    job,
	})
}

// getExportJob 获取导出任务.
func (h *Handlers) getExportJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.manager.GetExportJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    job,
	})
}

// getStorageQuota 获取存储配额.
func (h *Handlers) getStorageQuota(c *gin.Context) {
	cameraID := c.Param("cameraId")
	quota, err := h.manager.GetStorageQuota(cameraID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    quota,
	})
}

// setStorageQuota 设置存储配额.
func (h *Handlers) setStorageQuota(c *gin.Context) {
	cameraID := c.Param("cameraId")
	var quota StorageQuota
	if err := c.ShouldBindJSON(&quota); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	quota.CameraID = cameraID
	if err := h.manager.SetStorageQuota(&quota); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// sendPTZCommand 发送 PTZ 命令.
func (h *Handlers) sendPTZCommand(c *gin.Context) {
	var cmd PTZCommand
	if err := c.ShouldBindJSON(&cmd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.SendPTZCommand(cmd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// listSchedules 列出录制计划.
func (h *Handlers) listSchedules(c *gin.Context) {
	cameraID := c.Query("cameraId")
	schedules := h.manager.ListSchedules(cameraID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    schedules,
	})
}

// addSchedule 添加录制计划.
func (h *Handlers) addSchedule(c *gin.Context) {
	var schedule RecordingSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddSchedule(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    schedule,
	})
}

// deleteSchedule 删除录制计划.
func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getStats 获取监控统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// HealthCheck 健康检查端点.
func (h *Handlers) HealthCheck(c *gin.Context) {
	stats := h.manager.GetStats()
	
	status := "healthy"
	if stats.OnlineCameras == 0 && stats.TotalCameras > 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
		"stats":     stats,
	})
}
