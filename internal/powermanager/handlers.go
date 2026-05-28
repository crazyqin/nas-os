// Package powermanager 提供电源管理 HTTP API
package powermanager

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 电源管理 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	power := r.Group("/power")
	{
		power.GET("/plans", h.getPlans)
		power.POST("/plan", h.setPlan)
		power.GET("/schedules", h.getSchedules)
		power.POST("/schedule", h.addSchedule)
		power.DELETE("/schedule/:id", h.removeSchedule)
		power.GET("/ups", h.getUPSStatus)
		power.GET("/consumption", h.getConsumption)
		power.POST("/wake", h.sendWoL)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getPlans 获取电源计划列表.
func (h *Handlers) getPlans(c *gin.Context) {
	plans := h.manager.GetPlans()
	current := h.manager.GetCurrentPlan()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"plans":   plans,
			"current": current,
		},
	})
}

// SetPlanRequest 设置电源计划请求.
type SetPlanRequest struct {
	Plan PowerPlan `json:"plan" binding:"required"`
}

// setPlan 设置电源计划.
func (h *Handlers) setPlan(c *gin.Context) {
	var req SetPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetPlan(req.Plan); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "power plan updated",
		Data:    h.manager.GetCurrentPlan(),
	})
}

// getSchedules 获取定时任务列表.
func (h *Handlers) getSchedules(c *gin.Context) {
	schedules := h.manager.GetSchedules()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(schedules),
			"schedules": schedules,
		},
	})
}

// addSchedule 添加定时任务.
func (h *Handlers) addSchedule(c *gin.Context) {
	var schedule PowerSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddSchedule(&schedule); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "schedule added",
		Data:    schedule,
	})
}

// removeSchedule 删除定时任务.
func (h *Handlers) removeSchedule(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "schedule id is required",
		})
		return
	}

	if err := h.manager.RemoveSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "schedule removed",
	})
}

// getUPSStatus 获取 UPS 状态.
func (h *Handlers) getUPSStatus(c *gin.Context) {
	ups := h.manager.GetUPSStatus()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    ups,
	})
}

// getConsumption 获取功耗统计.
func (h *Handlers) getConsumption(c *gin.Context) {
	stats := h.manager.GetConsumptionStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// sendWoL 发送网络唤醒.
func (h *Handlers) sendWoL(c *gin.Context) {
	var req WoLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.SendWakeOnLAN(&req); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "WoL packet sent",
	})
}
