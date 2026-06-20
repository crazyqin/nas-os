package zfsscrubber

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler ZFS 清洗器 HTTP 处理器
type Handler struct {
	scrubber *ZFSScrubber
}

// NewHandler 创建 HTTP 处理器
func NewHandler(scrubber *ZFSScrubber) *Handler {
	return &Handler{scrubber: scrubber}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	zfs := rg.Group("/zfs-scrubber")
	{
		// 调度管理
		zfs.POST("/schedules", h.createSchedule)
		zfs.GET("/schedules", h.listSchedules)
		zfs.GET("/schedules/:id", h.getSchedule)
		zfs.PUT("/schedules/:id", h.updateSchedule)
		zfs.DELETE("/schedules/:id", h.deleteSchedule)

		// 清洗任务
		zfs.POST("/scrub/:poolId", h.executeScrub)
		zfs.GET("/jobs", h.listJobs)
		zfs.GET("/jobs/:id", h.getJob)
		zfs.POST("/jobs/:id/cancel", h.cancelJob)

		// 清洗报告
		zfs.GET("/reports", h.listReports)
		zfs.GET("/reports/:id", h.getReport)

		// 健康监控
		zfs.GET("/health/pools", h.listPoolHealths)
		zfs.GET("/health/pools/:id", h.checkPoolHealth)
		zfs.PUT("/health/pools/:id", h.updatePoolHealth)
		zfs.GET("/health/disks/:path", h.checkDiskSMART)

		// 告警管理
		zfs.GET("/alerts", h.listAlerts)
		zfs.GET("/alerts/:id", h.getAlert)
		zfs.POST("/alerts/:id/ack", h.acknowledgeAlert)

		// 修复动作
		zfs.GET("/repairs", h.listRepairActions)
		zfs.GET("/repairs/:id", h.getRepairAction)

		// 配置
		zfs.GET("/config", h.getConfig)
		zfs.PUT("/config", h.updateConfig)

		// 状态
		zfs.GET("/status", h.getStatus)
	}
}

// createScheduleReq 创建调度请求
type createScheduleReq struct {
	ID         string         `json:"id" binding:"required"`
	PoolID     string         `json:"pool_id" binding:"required"`
	Frequency  ScrubFrequency `json:"frequency" binding:"required"`
	DayOfWeek  int            `json:"day_of_week"`
	DayOfMonth int            `json:"day_of_month"`
	Hour       int            `json:"hour" binding:"required"`
	Enabled    bool           `json:"enabled"`
}

// createSchedule 创建调度
func (h *Handler) createSchedule(c *gin.Context) {
	var req createScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	schedule := &ScrubSchedule{
		ID:         req.ID,
		PoolID:     req.PoolID,
		Frequency:  req.Frequency,
		DayOfWeek:  req.DayOfWeek,
		DayOfMonth: req.DayOfMonth,
		Hour:       req.Hour,
		Enabled:    req.Enabled,
	}

	if err := h.scrubber.CreateSchedule(schedule); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

// listSchedules 列出调度
func (h *Handler) listSchedules(c *gin.Context) {
	schedules := h.scrubber.ListSchedules()
	c.JSON(http.StatusOK, gin.H{"schedules": schedules, "total": len(schedules)})
}

// getSchedule 获取调度
func (h *Handler) getSchedule(c *gin.Context) {
	schedule, err := h.scrubber.GetSchedule(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// updateScheduleReq 更新调度请求
type updateScheduleReq struct {
	Frequency  ScrubFrequency `json:"frequency"`
	DayOfWeek  int            `json:"day_of_week"`
	DayOfMonth int            `json:"day_of_month"`
	Hour       int            `json:"hour"`
	Enabled    *bool          `json:"enabled"`
}

// updateSchedule 更新调度
func (h *Handler) updateSchedule(c *gin.Context) {
	var req updateScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	schedule, err := h.scrubber.GetSchedule(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if req.Frequency != "" {
		schedule.Frequency = req.Frequency
	}
	if req.DayOfWeek != 0 {
		schedule.DayOfWeek = req.DayOfWeek
	}
	if req.DayOfMonth != 0 {
		schedule.DayOfMonth = req.DayOfMonth
	}
	if req.Hour != 0 {
		schedule.Hour = req.Hour
	}
	if req.Enabled != nil {
		schedule.Enabled = *req.Enabled
	}

	if err := h.scrubber.UpdateSchedule(schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// deleteSchedule 删除调度
func (h *Handler) deleteSchedule(c *gin.Context) {
	if err := h.scrubber.DeleteSchedule(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "调度已删除"})
}

// executeScrub 执行清洗
func (h *Handler) executeScrub(c *gin.Context) {
	job, err := h.scrubber.ExecuteScrub(c.Param("poolId"))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// listJobs 列出任务
func (h *Handler) listJobs(c *gin.Context) {
	poolID := c.Query("pool_id")
	jobs := h.scrubber.ListJobs(poolID)
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// getJob 获取任务
func (h *Handler) getJob(c *gin.Context) {
	job, err := h.scrubber.GetJob(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// cancelJob 取消任务
func (h *Handler) cancelJob(c *gin.Context) {
	if err := h.scrubber.CancelJob(c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// listReports 列出报告
func (h *Handler) listReports(c *gin.Context) {
	poolID := c.Query("pool_id")
	reports := h.scrubber.ListReports(poolID)
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": len(reports)})
}

// getReport 获取报告
func (h *Handler) getReport(c *gin.Context) {
	report, err := h.scrubber.GetReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// listPoolHealths 列出池健康状态
func (h *Handler) listPoolHealths(c *gin.Context) {
	healths := h.scrubber.ListPoolHealths()
	c.JSON(http.StatusOK, gin.H{"pools": healths, "total": len(healths)})
}

// checkPoolHealth 检查池健康
func (h *Handler) checkPoolHealth(c *gin.Context) {
	health, err := h.scrubber.CheckPoolHealth(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, health)
}

// updatePoolHealthReq 更新池健康请求
type updatePoolHealthReq struct {
	PoolName        string       `json:"pool_name"`
	OverallHealth   HealthLevel  `json:"overall_health"`
	Status          string       `json:"status"`
	TotalSize       int64        `json:"total_size"`
	UsedSize        int64        `json:"used_size"`
	FreeSize        int64        `json:"free_size"`
	Fragmentation   float64      `json:"fragmentation"`
	ScrubErrors     int          `json:"scrub_errors"`
	ChecksumErrors  int          `json:"checksum_errors"`
	DegradedDevices int          `json:"degraded_devices"`
	FailedDevices   int          `json:"failed_devices"`
	Disks           []*DiskSMART `json:"disks"`
}

// updatePoolHealth 更新池健康
func (h *Handler) updatePoolHealth(c *gin.Context) {
	var req updatePoolHealthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	health := &PoolHealth{
		PoolID:          c.Param("id"),
		PoolName:        req.PoolName,
		OverallHealth:   req.OverallHealth,
		Status:          req.Status,
		TotalSize:       req.TotalSize,
		UsedSize:        req.UsedSize,
		FreeSize:        req.FreeSize,
		Fragmentation:   req.Fragmentation,
		ScrubErrors:     req.ScrubErrors,
		ChecksumErrors:  req.ChecksumErrors,
		DegradedDevices: req.DegradedDevices,
		FailedDevices:   req.FailedDevices,
		Disks:           req.Disks,
	}

	h.scrubber.UpdatePoolHealth(health)
	c.JSON(http.StatusOK, health)
}

// checkDiskSMART 检查磁盘 SMART
func (h *Handler) checkDiskSMART(c *gin.Context) {
	smart, err := h.scrubber.CheckDiskSMART(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, smart)
}

// listAlerts 列出告警
func (h *Handler) listAlerts(c *gin.Context) {
	poolID := c.Query("pool_id")
	unackedOnly := c.Query("unacked") == "true"
	alerts := h.scrubber.ListAlerts(poolID, unackedOnly)
	c.JSON(http.StatusOK, gin.H{"alerts": alerts, "total": len(alerts)})
}

// getAlert 获取告警
func (h *Handler) getAlert(c *gin.Context) {
	alert, err := h.scrubber.GetAlert(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alert)
}

// acknowledgeAlert 确认告警
func (h *Handler) acknowledgeAlert(c *gin.Context) {
	if err := h.scrubber.AcknowledgeAlert(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "告警已确认"})
}

// listRepairActions 列出修复动作
func (h *Handler) listRepairActions(c *gin.Context) {
	poolID := c.Query("pool_id")
	actions := h.scrubber.ListRepairActions(poolID)
	c.JSON(http.StatusOK, gin.H{"actions": actions, "total": len(actions)})
}

// getRepairAction 获取修复动作
func (h *Handler) getRepairAction(c *gin.Context) {
	action, err := h.scrubber.GetRepairAction(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, action)
}

// getConfig 获取配置
func (h *Handler) getConfig(c *gin.Context) {
	config := h.scrubber.GetConfig()
	c.JSON(http.StatusOK, config)
}

// updateConfig 更新配置
func (h *Handler) updateConfig(c *gin.Context) {
	var config ScrubberConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.scrubber.UpdateConfig(&config)
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// getStatus 获取状态
func (h *Handler) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running":         h.scrubber.IsRunning(),
		"schedules_count": len(h.scrubber.ListSchedules()),
		"jobs_count":      len(h.scrubber.ListJobs("")),
		"reports_count":   len(h.scrubber.ListReports("")),
		"alerts_count":    len(h.scrubber.ListAlerts("", true)),
		"timestamp":       time.Now(),
	})
}
