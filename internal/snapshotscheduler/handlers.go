package snapshotscheduler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 快照调度 API 处理器
type Handlers struct {
	scheduler *Scheduler
}

// NewHandlers 创建处理器
func NewHandlers(scheduler *Scheduler) *Handlers {
	return &Handlers{scheduler: scheduler}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snaps := r.Group("/snapshots")
	{
		// 快照管理
		snaps.GET("", h.listSnapshots)
		snaps.GET("/:id", h.getSnapshot)
		snaps.POST("", h.createSnapshot)
		snaps.DELETE("/:id", h.deleteSnapshot)
		snaps.POST("/:id/clone", h.cloneSnapshot)
		snaps.POST("/rollback", h.rollback)

		// 调度管理
		snaps.GET("/schedules", h.listSchedules)
		snaps.GET("/schedules/:id", h.getSchedule)
		snaps.POST("/schedules", h.createSchedule)
		snaps.PUT("/schedules/:id", h.updateSchedule)
		snaps.DELETE("/schedules/:id", h.deleteSchedule)

		// 统计
		snaps.GET("/stats", h.getStats)
	}
}

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handlers) listSnapshots(c *gin.Context) {
	volume := c.Query("volume")
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	snapshots := h.scheduler.ListSnapshots(volume, limit)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: snapshots})
}

func (h *Handlers) getSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.scheduler.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: snap})
}

func (h *Handlers) createSnapshot(c *gin.Context) {
	var req struct {
		VolumePath string   `json:"volume_path" binding:"required"`
		Name       string   `json:"name" binding:"required"`
		Tags       []string `json:"tags,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	snap, err := h.scheduler.CreateSnapshot(req.VolumePath, req.Name, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "snapshot created", Data: snap})
}

func (h *Handlers) deleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.DeleteSnapshot(id); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "snapshot deleted"})
}

func (h *Handlers) cloneSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		TargetPath string `json:"target_path,omitempty"`
	}
	c.ShouldBindJSON(&req)

	result, err := h.scheduler.CloneSnapshot(id, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "snapshot cloned", Data: result})
}

func (h *Handlers) rollback(c *gin.Context) {
	var req RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.scheduler.Rollback(&req); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "rollback successful"})
}

func (h *Handlers) listSchedules(c *gin.Context) {
	enabledOnly := c.Query("enabled") == "true"
	schedules := h.scheduler.ListSchedules(enabledOnly)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: schedules})
}

func (h *Handlers) getSchedule(c *gin.Context) {
	id := c.Param("id")
	sched, err := h.scheduler.GetSchedule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: sched})
}

func (h *Handlers) createSchedule(c *gin.Context) {
	var sched Schedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.scheduler.CreateSchedule(&sched); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "schedule created", Data: sched})
}

func (h *Handlers) updateSchedule(c *gin.Context) {
	id := c.Param("id")
	var update Schedule
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.scheduler.UpdateSchedule(id, &update); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "schedule updated"})
}

func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.DeleteSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "schedule deleted"})
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.scheduler.GetStats()
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: stats})
}
