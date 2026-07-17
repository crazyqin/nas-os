package scrubscheduler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 是 scrub 调度器的 HTTP API 处理器.
type Handlers struct {
	scheduler *ScrubScheduler
}

// NewHandlers 创建处理器.
func NewHandlers(scheduler *ScrubScheduler) *Handlers {
	return &Handlers{scheduler: scheduler}
}

// RegisterRoutes 注册路由到指定的路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	scrub := r.Group("/scrub")
	{
		// 调度管理
		scrub.GET("/schedules", h.listSchedules)
		scrub.POST("/schedules", h.createSchedule)
		scrub.PUT("/schedules/:id", h.updateSchedule)
		scrub.DELETE("/schedules/:id", h.deleteSchedule)

		// 手动触发与状态查询
		scrub.POST("/pools/:pool/start", h.startScrub)
		scrub.GET("/pools/:pool/status", h.getPoolStatus)

		// 历史记录
		scrub.GET("/history", h.getHistory)
	}
}

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listSchedules 获取所有调度.
func (h *Handlers) listSchedules(c *gin.Context) {
	schedules := h.scheduler.ListSchedules()
	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    schedules,
	})
}

// createSchedule 创建调度.
func (h *Handlers) createSchedule(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	sch := &ScrubSchedule{
		PoolName:          req.PoolName,
		Schedule:          req.Schedule,
		MaintenanceWindow: req.MaintenanceWindow,
		MaxDuration:       req.MaxDuration,
		RetryCount:        req.RetryCount,
		Enabled:           enabled,
	}

	// 应用默认值
	if sch.MaintenanceWindow.Start == "" && sch.MaintenanceWindow.End == "" {
		sch.MaintenanceWindow = h.scheduler.config.DefaultMaintenanceWindow
	}
	if sch.MaxDuration == 0 {
		sch.MaxDuration = h.scheduler.config.DefaultMaxDuration
	}
	if sch.RetryCount == 0 {
		sch.RetryCount = h.scheduler.config.DefaultRetryCount
	}

	if err := h.scheduler.AddSchedule(sch); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, apiResponse{
		Code:    0,
		Message: "schedule created",
		Data:    sch,
	})
}

// updateSchedule 更新调度.
func (h *Handlers) updateSchedule(c *gin.Context) {
	id := c.Param("id")

	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	sch, err := h.scheduler.UpdateSchedule(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "schedule updated",
		Data:    sch,
	})
}

// deleteSchedule 删除调度.
func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")

	if err := h.scheduler.DeleteSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "schedule deleted",
	})
}

// startScrub 手动触发 scrub.
func (h *Handlers) startScrub(c *gin.Context) {
	pool := c.Param("pool")

	if err := h.scheduler.StartScrub(c.Request.Context(), pool); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "scrub started",
		Data: map[string]string{
			"pool": pool,
		},
	})
}

// getPoolStatus 获取池的 scrub 状态.
func (h *Handlers) getPoolStatus(c *gin.Context) {
	pool := c.Param("pool")

	status, err := h.scheduler.GetPoolStatus(pool)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// getHistory 获取历史记录.
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.scheduler.GetHistory(limit)
	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}
