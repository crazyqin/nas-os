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
		// 电源计划
		power.GET("/plans", h.getPlans)
		power.POST("/plan", h.setPlan)

		// 定时任务
		power.GET("/schedules", h.getSchedules)
		power.POST("/schedule", h.addSchedule)
		power.DELETE("/schedule/:id", h.removeSchedule)

		// UPS
		power.GET("/ups", h.getUPSStatus)

		// 功耗统计
		power.GET("/consumption", h.getConsumption)
		power.GET("/consumption/history", h.getConsumptionHistory)

		// 硬盘休眠管理
		power.GET("/disks", h.getDiskStatus)
		power.POST("/disk/hibernate/:device", h.hibernateDisk)
		power.POST("/disk/wake/:device", h.wakeDisk)

		// CPU 调频
		power.GET("/cpu", h.getCPUInfo)
		power.POST("/cpu/governor", h.setCPUGovernor)
		power.POST("/cpu/frequency", h.setCPUFrequency)

		// 网络唤醒
		power.POST("/wake", h.sendWoL)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 电源计划 ==========

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

// ========== 定时任务 ==========

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
		c.JSON(http.StatusBadRequest, response{
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

// ========== UPS ==========

// getUPSStatus 获取 UPS 状态.
func (h *Handlers) getUPSStatus(c *gin.Context) {
	ups := h.manager.GetUPSStatus()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    ups,
	})
}

// ========== 功耗统计 ==========

// getConsumption 获取功耗统计.
func (h *Handlers) getConsumption(c *gin.Context) {
	stats := h.manager.GetConsumptionStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getConsumptionHistory 获取功耗历史.
func (h *Handlers) getConsumptionHistory(c *gin.Context) {
	history := h.manager.GetConsumptionHistory()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// ========== 硬盘休眠管理 ==========

// getDiskStatus 获取硬盘状态.
func (h *Handlers) getDiskStatus(c *gin.Context) {
	disks := h.manager.GetDiskStatus()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(disks),
			"disks": disks,
		},
	})
}

// HibernateDiskRequest 休眠硬盘请求.
type HibernateDiskRequest struct {
	Device string `json:"device" binding:"required"`
}

// hibernateDisk 休眠硬盘.
func (h *Handlers) hibernateDisk(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "device is required",
		})
		return
	}

	// URL 中的设备名需要加上 /dev/ 前缀
	if len(device) > 0 && device[0] != '/' {
		device = "/dev/" + device
	}

	if err := h.manager.HibernateDisk(device); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "disk hibernated",
		Data: gin.H{
			"device": device,
		},
	})
}

// wakeDisk 唤醒硬盘.
func (h *Handlers) wakeDisk(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "device is required",
		})
		return
	}

	if len(device) > 0 && device[0] != '/' {
		device = "/dev/" + device
	}

	if err := h.manager.WakeDisk(device); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "disk woken up",
		Data: gin.H{
			"device": device,
		},
	})
}

// ========== CPU 调频 ==========

// getCPUInfo 获取 CPU 信息.
func (h *Handlers) getCPUInfo(c *gin.Context) {
	info := h.manager.GetCPUInfo()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// SetGovernorRequest 设置 CPU 调频策略请求.
type SetGovernorRequest struct {
	Governor string `json:"governor" binding:"required"`
}

// setCPUGovernor 设置 CPU 调频策略.
func (h *Handlers) setCPUGovernor(c *gin.Context) {
	var req SetGovernorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetCPUGovernor(req.Governor); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "CPU governor updated",
		Data:    h.manager.GetCPUInfo(),
	})
}

// SetFrequencyRequest 设置 CPU 频率请求.
type SetFrequencyRequest struct {
	Frequency int `json:"frequency" binding:"required"` // MHz
}

// setCPUFrequency 设置 CPU 频率.
func (h *Handlers) setCPUFrequency(c *gin.Context) {
	var req SetFrequencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetCPUFrequency(req.Frequency); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "CPU frequency updated",
		Data:    h.manager.GetCPUInfo(),
	})
}

// ========== Wake on LAN ==========

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
