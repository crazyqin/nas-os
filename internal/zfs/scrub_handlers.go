package zfs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ScrubHandler ZFS Scrub REST API handler.
type ScrubHandler struct {
	scheduler *ScrubScheduler
}

// NewScrubHandler 创建ScrubHandler.
func NewScrubHandler(scheduler *ScrubScheduler) *ScrubHandler {
	return &ScrubHandler{scheduler: scheduler}
}

// RegisterRoutes 注册scrub相关路由.
func (h *ScrubHandler) RegisterRoutes(rg *gin.RouterGroup) {
	scrub := rg.Group("/zfs/scrub")
	{
		scrub.GET("/status", h.GetStatus)
		scrub.POST("/start", h.StartScrub)
		scrub.POST("/pause", h.PauseScrub)
		scrub.POST("/resume", h.ResumeScrub)
		scrub.GET("/history", h.GetHistory)
		scrub.PUT("/schedule", h.UpdateSchedule)
		scrub.GET("/schedule", h.GetSchedule)
		scrub.GET("/io-stats", h.GetIOStats)
	}
}

// GetStatus 获取scrub状态
// @Summary 获取ZFS scrub实时状态
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} ScrubProgress
// @Router /api/v1/zfs/scrub/status [get].
func (h *ScrubHandler) GetStatus(c *gin.Context) {
	progress := h.scheduler.GetProgress()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    progress,
	})
}

// StartScrub 手动启动scrub
// @Summary 手动启动ZFS scrub
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/zfs/scrub/start [post].
func (h *ScrubHandler) StartScrub(c *gin.Context) {
	if err := h.scheduler.StartScrub(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "scrub started",
	})
}

// PauseScrub 暂停scrub
// @Summary 暂停ZFS scrub
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/zfs/scrub/pause [post].
func (h *ScrubHandler) PauseScrub(c *gin.Context) {
	if err := h.scheduler.PauseScrub(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "scrub paused",
	})
}

// ResumeScrub 恢复scrub
// @Summary 恢复ZFS scrub
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/zfs/scrub/resume [post].
func (h *ScrubHandler) ResumeScrub(c *gin.Context) {
	if err := h.scheduler.ResumeScrub(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "scrub resumed",
	})
}

// GetHistory 获取scrub历史
// @Summary 获取ZFS scrub历史记录
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {array} ScrubResult
// @Router /api/v1/zfs/scrub/history [get].
func (h *ScrubHandler) GetHistory(c *gin.Context) {
	history := h.scheduler.GetHistory()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    history,
	})
}

// UpdateSchedule 更新scrub调度配置
// @Summary 更新ZFS scrub调度配置
// @Tags ZFS Scrub
// @Accept json
// @Produce json
// @Param config body ScrubScheduleConfig true "调度配置"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/zfs/scrub/schedule [put].
func (h *ScrubHandler) UpdateSchedule(c *gin.Context) {
	var config ScrubScheduleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid config: " + err.Error(),
		})
		return
	}
	h.scheduler.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "schedule updated",
	})
}

// GetSchedule 获取scrub调度配置
// @Summary 获取ZFS scrub调度配置
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} ScrubScheduleConfig
// @Router /api/v1/zfs/scrub/schedule [get].
func (h *ScrubHandler) GetSchedule(c *gin.Context) {
	config := h.scheduler.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    config,
	})
}

// GetIOStats 获取磁盘IO统计
// @Summary 获取磁盘IO负载统计
// @Tags ZFS Scrub
// @Produce json
// @Success 200 {object} map[string]int
// @Router /api/v1/zfs/scrub/io-stats [get].
func (h *ScrubHandler) GetIOStats(c *gin.Context) {
	readIOPS, writeIOPS, err := h.scheduler.GetIOStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"read_iops":  readIOPS,
			"write_iops": writeIOPS,
		},
	})
}
