// Package disk 提供磁盘电源管理API处理器
// Version: v2.388.0 - 电源管理API + 能耗统计 + 智能调度
// 对标飞牛fnOS按需唤醒硬盘功能
package disk

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// PowerHandlers 电源管理处理器
type PowerHandlers struct {
	manager *PowerManager
}

// NewPowerHandlers 创建电源管理处理器
func NewPowerHandlers(manager *PowerManager) *PowerHandlers {
	return &PowerHandlers{
		manager: manager,
	}
}

// RegisterRoutes 注册电源管理路由
func (h *PowerHandlers) RegisterRoutes(r *gin.RouterGroup) {
	power := r.Group("/power")
	{
		// 电源状态
		power.GET("/status", h.getAllPowerStatus)
		power.GET("/status/:disk", h.getDiskPowerStatus)

		// 电源策略
		power.GET("/policies", h.getAllPolicies)
		power.GET("/policies/:id", h.getPolicy)
		power.POST("/policies", h.createPolicy)
		power.PUT("/policies/:id", h.updatePolicy)
		power.DELETE("/policies/:id", h.deletePolicy)

		// 电源控制
		power.POST("/disks/:disk/wake", h.wakeDisk)
		power.POST("/disks/:disk/sleep", h.sleepDisk)
		power.POST("/disks/:disk/standby", h.standbyDisk)

		// 按需唤醒队列
		power.GET("/wake-queue", h.getWakeQueue)
		power.POST("/wake-queue", h.addToWakeQueue)
		power.DELETE("/wake-queue/:disk", h.clearWakeQueue)

		// 能耗统计
		power.GET("/energy/report", h.getEnergyReport)
		power.GET("/energy/stats", h.getEnergyStats)
		power.GET("/energy/hourly", h.getHourlyEnergyStats)
		power.GET("/energy/disk/:disk", h.getDiskEnergyStat)

		// 业务时段配置
		power.GET("/business-hours", h.getBusinessHours)
		power.PUT("/business-hours", h.updateBusinessHours)

		// 智能调度配置
		power.GET("/smart-schedule/config", h.getSmartScheduleConfig)
		power.PUT("/smart-schedule/config", h.updateSmartScheduleConfig)

		// 电源管理配置
		power.GET("/config", h.getConfig)
		power.PUT("/config", h.updateConfig)
	}
}

// getAllPowerStatus 获取所有磁盘电源状态
// @Summary 获取所有磁盘电源状态
// @Description 获取所有注册磁盘的当前电源状态
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/status [get]
func (h *PowerHandlers) getAllPowerStatus(c *gin.Context) {
	statuses := h.manager.GetAllStatuses()

	// 生成摘要
	summary := struct {
		Total      int     `json:"total"`
		Active     int     `json:"active"`
		Idle       int     `json:"idle"`
		Standby    int     `json:"standby"`
		Sleep      int     `json:"sleep"`
		EnergySaved float64 `json:"energySaved"` // kWh
	}{}

	for _, status := range statuses {
		summary.Total++
		switch status.State {
		case PowerStateActive:
			summary.Active++
		case PowerStateIdle:
			summary.Idle++
		case PowerStateStandby:
			summary.Standby++
		case PowerStateSleep:
			summary.Sleep++
		}
		summary.EnergySaved += status.EnergySaved
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary": summary,
			"statuses": statuses,
		},
	})
}

// getDiskPowerStatus 获取单个磁盘电源状态
// @Summary 获取单个磁盘电源状态
// @Description 获取指定磁盘的详细电源状态
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/status/{disk} [get]
func (h *PowerHandlers) getDiskPowerStatus(c *gin.Context) {
	diskID := c.Param("disk")
	if diskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "磁盘ID不能为空",
		})
		return
	}

	status, err := h.manager.GetDiskStatus(diskID)
	if err != nil || status == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "磁盘未注册或不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// getAllPolicies 获取所有电源策略
// @Summary 获取所有电源策略
// @Description 获取所有已配置的电源管理策略
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/policies [get]
func (h *PowerHandlers) getAllPolicies(c *gin.Context) {
	// 获取所有策略
	policies := h.manager.GetAllPolicies()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// getPolicy 获取单个电源策略
// @Summary 获取单个电源策略
// @Description 获取指定策略的详细配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/policies/{id} [get]
func (h *PowerHandlers) getPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "策略ID不能为空",
		})
		return
	}

	policy, err := h.manager.GetPolicy(policyID)
	if err != nil || policy == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "策略不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policy,
	})
}

// createPolicyRequest 创建策略请求
type createPolicyRequest struct {
	ID               string        `json:"id" binding:"required"`
	Name             string        `json:"name" binding:"required"`
	IdleThreshold    int           `json:"idleThreshold"`    // 秒
	StandbyThreshold int           `json:"standbyThreshold"` // 秒
	SleepThreshold   int           `json:"sleepThreshold"`   // 秒
	Enabled          bool          `json:"enabled"`
	ExcludedDisks    []string      `json:"excludedDisks"`
	BusinessPeriods  []BusinessPeriod `json:"businessPeriods"`
	AllowSleepInPeak bool          `json:"allowSleepInPeak"`
	MaxWakePerHour   int           `json:"maxWakePerHour"`
}

// createPolicy 创建电源策略
// @Summary 创建电源策略
// @Description 创建新的电源管理策略
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body createPolicyRequest true "策略配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/policies [post]
func (h *PowerHandlers) createPolicy(c *gin.Context) {
	var req createPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	policy := &SleepPolicy{
		ID:               req.ID,
		Name:             req.Name,
		IdleThreshold:    time.Duration(req.IdleThreshold) * time.Second,
		StandbyThreshold: time.Duration(req.StandbyThreshold) * time.Second,
		SleepThreshold:   time.Duration(req.SleepThreshold) * time.Second,
		Enabled:          req.Enabled,
		ExcludedDisks:    req.ExcludedDisks,
		BusinessPeriods:  req.BusinessPeriods,
		AllowSleepInPeak: req.AllowSleepInPeak,
		MaxWakePerHour:   req.MaxWakePerHour,
	}

	if err := h.manager.AddPolicy(policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略创建成功",
		"data":    policy,
	})
}

// updatePolicy 更新电源策略
// @Summary 更新电源策略
// @Description 更新已存在的电源管理策略
// @Tags disk-power
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Param request body createPolicyRequest true "策略配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/policies/{id} [put]
func (h *PowerHandlers) updatePolicy(c *gin.Context) {
	policyID := c.Param("id")

	var req createPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	policy := &SleepPolicy{
		ID:               policyID,
		Name:             req.Name,
		IdleThreshold:    time.Duration(req.IdleThreshold) * time.Second,
		StandbyThreshold: time.Duration(req.StandbyThreshold) * time.Second,
		SleepThreshold:   time.Duration(req.SleepThreshold) * time.Second,
		Enabled:          req.Enabled,
		ExcludedDisks:    req.ExcludedDisks,
		BusinessPeriods:  req.BusinessPeriods,
		AllowSleepInPeak: req.AllowSleepInPeak,
		MaxWakePerHour:   req.MaxWakePerHour,
	}

	if err := h.manager.AddPolicy(policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略更新成功",
		"data":    policy,
	})
}

// deletePolicy 删除电源策略
// @Summary 删除电源策略
// @Description 删除指定的电源管理策略
// @Tags disk-power
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/policies/{id} [delete]
func (h *PowerHandlers) deletePolicy(c *gin.Context) {
	policyID := c.Param("id")

	if err := h.manager.DeletePolicy(policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略删除成功",
	})
}

// wakeDisk 唤醒磁盘
// @Summary 唤醒磁盘
// @Description 强制唤醒指定磁盘
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/disks/{disk}/wake [post]
func (h *PowerHandlers) wakeDisk(c *gin.Context) {
	diskID := c.Param("disk")
	if diskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "磁盘ID不能为空",
		})
		return
	}

	// 记录活动以唤醒磁盘
	if err := h.manager.RecordActivity(diskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.manager.GetDiskStatus(diskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "磁盘已唤醒",
		"data":    status,
	})
}

// sleepDisk 让磁盘休眠
// @Summary 让磁盘休眠
// @Description 强制让指定磁盘进入休眠状态
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/disks/{disk}/sleep [post]
func (h *PowerHandlers) sleepDisk(c *gin.Context) {
	diskID := c.Param("disk")
	if diskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "磁盘ID不能为空",
		})
		return
	}

	if err := h.manager.ForceSleep(diskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.manager.GetDiskStatus(diskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "磁盘已休眠",
		"data":    status,
	})
}

// standbyDisk 让磁盘待机
// @Summary 让磁盘待机
// @Description 强制让指定磁盘进入待机状态
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/disks/{disk}/standby [post]
func (h *PowerHandlers) standbyDisk(c *gin.Context) {
	diskID := c.Param("disk")
	if diskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "磁盘ID不能为空",
		})
		return
	}

	if err := h.manager.ForceStandby(diskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.manager.GetDiskStatus(diskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "磁盘已待机",
		"data":    status,
	})
}

// wakeQueueRequest 按需唤醒请求
type wakeQueueRequest struct {
	DiskID      string `json:"diskId" binding:"required"`
	Reason      string `json:"reason"`
	Priority    int    `json:"priority"`
RequestedBy string `json:"requestedBy"`
}

// getWakeQueue 获取唤醒队列
// @Summary 获取唤醒队列
// @Description 获取所有待处理的唤醒请求队列
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/wake-queue [get]
func (h *PowerHandlers) getWakeQueue(c *gin.Context) {
	queue := h.manager.GetWakeQueue()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    queue,
	})
}

// addToWakeQueue 添加唤醒请求到队列
// @Summary 添加唤醒请求
// @Description 添加按需唤醒请求到队列
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body wakeQueueRequest true "唤醒请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/wake-queue [post]
func (h *PowerHandlers) addToWakeQueue(c *gin.Context) {
	var req wakeQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	wakeReq := WakeRequest{
		DiskID:      req.DiskID,
		Reason:      req.Reason,
		Priority:    req.Priority,
		Timestamp:   time.Now(),
		RequestedBy: req.RequestedBy,
	}

	if err := h.manager.AddWakeRequest(wakeReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "唤醒请求已添加",
		"data":    wakeReq,
	})
}

// clearWakeQueue 清除唤醒队列
// @Summary 清除唤醒队列
// @Description 清除指定磁盘的唤醒请求队列
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/wake-queue/{disk} [delete]
func (h *PowerHandlers) clearWakeQueue(c *gin.Context) {
	diskID := c.Param("disk")

	h.manager.ClearWakeQueue(diskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "唤醒队列已清除",
	})
}

// getEnergyReport 获取能耗报告
// @Summary 获取能耗报告
// @Description 获取整体能耗统计报告
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/energy/report [get]
func (h *PowerHandlers) getEnergyReport(c *gin.Context) {
	report := h.manager.GetEnergyReport()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// getEnergyStats 获取能耗统计
// @Summary 获取能耗统计
// @Description 获取详细的能耗统计数据
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/energy/stats [get]
func (h *PowerHandlers) getEnergyStats(c *gin.Context) {
	stats := h.manager.GetEnergyStatistics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getHourlyEnergyStats 获取小时级能耗统计
// @Summary 获取小时级能耗统计
// @Description 获取最近的小时级能耗统计数据
// @Tags disk-power
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(24)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/energy/hourly [get]
func (h *PowerHandlers) getHourlyEnergyStats(c *gin.Context) {
	limit := 24
	if l := c.Query("limit"); l != "" {
		// 解析limit参数
	}

	stats := h.manager.GetHourlyEnergyStats(limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getDiskEnergyStat 获取单磁盘能耗统计
// @Summary 获取单磁盘能耗统计
// @Description 获取指定磁盘的能耗统计数据
// @Tags disk-power
// @Accept json
// @Produce json
// @Param disk path string true "磁盘ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/energy/disk/{disk} [get]
func (h *PowerHandlers) getDiskEnergyStat(c *gin.Context) {
	diskID := c.Param("disk")

	stat := h.manager.GetDiskEnergyStat(diskID)
	if stat == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "磁盘无能耗数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stat,
	})
}

// businessHoursRequest 业务时段请求
type businessHoursRequest struct {
	Periods []BusinessPeriod `json:"periods" binding:"required"`
}

// getBusinessHours 获取业务时段配置
// @Summary 获取业务时段配置
// @Description 获取当前的业务高峰时段配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/business-hours [get]
func (h *PowerHandlers) getBusinessHours(c *gin.Context) {
	periods := h.manager.GetBusinessHours()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    periods,
	})
}

// updateBusinessHours 更新业务时段配置
// @Summary 更新业务时段配置
// @Description 更新业务高峰时段配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body businessHoursRequest true "时段配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/business-hours [put]
func (h *PowerHandlers) updateBusinessHours(c *gin.Context) {
	var req businessHoursRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.manager.SetBusinessHours(req.Periods)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "业务时段配置已更新",
		"data":    req.Periods,
	})
}

// smartScheduleConfigRequest 智能调度配置请求
type smartScheduleConfigRequest struct {
	EnableWakeOnDemand     bool    `json:"enableWakeOnDemand"`
	EnableSmartScheduling  bool    `json:"enableSmartScheduling"`
	DefaultDiskPowerWatts  float64 `json:"defaultDiskPowerWatts"`
	WakePowerSpikeWatts    float64 `json:"wakePowerSpikeWatts"`
	WakeDurationSeconds    float64 `json:"wakeDurationSeconds"`
}

// getSmartScheduleConfig 获取智能调度配置
// @Summary 获取智能调度配置
// @Description 获取当前智能调度配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/smart-schedule/config [get]
func (h *PowerHandlers) getSmartScheduleConfig(c *gin.Context) {
	config := h.manager.GetSmartScheduleConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// updateSmartScheduleConfig 更新智能调度配置
// @Summary 更新智能调度配置
// @Description 更新智能调度参数配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body smartScheduleConfigRequest true "配置参数"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/smart-schedule/config [put]
func (h *PowerHandlers) updateSmartScheduleConfig(c *gin.Context) {
	var req smartScheduleConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.manager.UpdateSmartScheduleConfig(req.EnableWakeOnDemand, req.EnableSmartScheduling,
		req.DefaultDiskPowerWatts, req.WakePowerSpikeWatts, req.WakeDurationSeconds)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "智能调度配置已更新",
	})
}

// getConfig 获取电源管理配置
// @Summary 获取电源管理配置
// @Description 获取当前电源管理器配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/config [get]
func (h *PowerHandlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// powerConfigRequest 电源配置请求
type powerConfigRequest struct {
	CheckInterval          int     `json:"checkInterval"`          // 秒
	DefaultPolicy          string  `json:"defaultPolicy"`
	EnableMonitoring       bool    `json:"enableMonitoring"`
	EnableWakeOnDemand     bool    `json:"enableWakeOnDemand"`
	EnableSmartScheduling  bool    `json:"enableSmartScheduling"`
	DefaultDiskPowerWatts  float64 `json:"defaultDiskPowerWatts"`
	WakePowerSpikeWatts    float64 `json:"wakePowerSpikeWatts"`
	WakeDurationSeconds    float64 `json:"wakeDurationSeconds"`
}

// updateConfig 更新电源管理配置
// @Summary 更新电源管理配置
// @Description 更新电源管理器全局配置
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body powerConfigRequest true "配置参数"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/config [put]
func (h *PowerHandlers) updateConfig(c *gin.Context) {
	var req powerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&PowerConfig{
		CheckInterval:          time.Duration(req.CheckInterval) * time.Second,
		DefaultPolicy:          req.DefaultPolicy,
		EnableMonitoring:       req.EnableMonitoring,
		EnableWakeOnDemand:     req.EnableWakeOnDemand,
		EnableSmartScheduling:  req.EnableSmartScheduling,
		DefaultDiskPowerWatts:  req.DefaultDiskPowerWatts,
		WakePowerSpikeWatts:    req.WakePowerSpikeWatts,
		WakeDurationSeconds:    req.WakeDurationSeconds,
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
	})
}

// registerDiskRequest 注册磁盘请求
type registerDiskRequest struct {
	DiskID  string `json:"diskId" binding:"required"`
	PolicyID string `json:"policyId"`
}

// RegisterDisk 注册磁盘到电源管理
// @Summary 注册磁盘到电源管理
// @Description 将磁盘添加到电源管理系统
// @Tags disk-power
// @Accept json
// @Produce json
// @Param request body registerDiskRequest true "注册请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /disk/power/register [post]
func (h *PowerHandlers) RegisterDisk(c *gin.Context) {
	var req registerDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.RegisterDisk(req.DiskID, req.PolicyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.manager.GetDiskStatus(req.DiskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "磁盘已注册",
		"data":    status,
	})
}