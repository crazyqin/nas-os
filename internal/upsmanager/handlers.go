package upsmanager

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers UPS 电源管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 UPS 处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册 UPS 路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	ups := apiGroup.Group("/ups")
	{
		// UPS 设备管理
		ups.GET("/devices", h.listDevices)
		ups.GET("/devices/:id", h.getDevice)
		ups.POST("/discover", h.discoverDevices)
		ups.POST("/connect", h.connectDevice)
		ups.DELETE("/devices/:id", h.disconnectDevice)

		// 电源状态
		ups.GET("/devices/:id/power", h.getPowerStatus)
		ups.GET("/power", h.getAllPowerStatus)
		ups.GET("/power/primary", h.getPrimaryPowerStatus)

		// 硬件健康
		ups.GET("/devices/:id/health", h.getHardwareHealth)

		// 关机策略
		ups.POST("/policies", h.createShutdownPolicy)
		ups.GET("/policies", h.listShutdownPolicies)
		ups.GET("/policies/:id", h.getShutdownPolicy)
		ups.PUT("/policies/:id", h.updateShutdownPolicy)
		ups.DELETE("/policies/:id", h.deleteShutdownPolicy)

		// 电源事件
		ups.GET("/events", h.getEvents)
		ups.GET("/events/count", h.getEventCount)

		// 电源统计
		ups.GET("/devices/:id/stats", h.getPowerStats)

		// 配置
		ups.GET("/config", h.getConfig)
		ups.PUT("/config", h.updateConfig)

		// 运行控制
		ups.POST("/start", h.start)
		ups.POST("/stop", h.stop)
		ups.GET("/status", h.getStatusSummary)

		// 状态设置（测试用）
		ups.PUT("/devices/:id/status", h.setDeviceStatus)
	}
}

// ========== UPS 设备管理 ==========

// listDevices 列出所有 UPS 设备
// @Summary 列出 UPS 设备
// @Description 获取所有已连接的 UPS 设备
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=[]UPSDevice}
// @Router /ups/devices [get].
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	api.OK(c, devices)
}

// getDevice 获取 UPS 设备详情
// @Summary 获取 UPS 设备
// @Description 根据 ID 获取 UPS 设备详情
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Success 200 {object} api.Response{data=UPSDevice}
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id} [get].
func (h *Handlers) getDevice(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	device, err := h.manager.GetDevice(upsID)
	if err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, device)
}

// discoverDevices 发现 UPS 设备
// @Summary 发现 UPS 设备
// @Description 扫描并发现新的 UPS 设备
// @Tags ups
// @Accept json
// @Produce json
// @Param request body DiscoverRequest true "发现请求"
// @Success 200 {object} api.Response{data=[]UPSDevice}
// @Failure 400 {object} api.Response
// @Router /ups/discover [post].
func (h *Handlers) discoverDevices(c *gin.Context) {
	var req DiscoverRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	devices, err := h.manager.Discover(req)
	if err != nil {
		if err == ErrProtocolNotSupported {
			api.BadRequest(c, "不支持的协议")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, devices)
}

// connectDevice 连接 UPS 设备
// @Summary 连接 UPS 设备
// @Description 连接并注册新的 UPS 设备
// @Tags ups
// @Accept json
// @Produce json
// @Param request body ConnectRequest true "连接请求"
// @Success 201 {object} api.Response{data=UPSDevice}
// @Failure 400 {object} api.Response
// @Failure 409 {object} api.Response
// @Router /ups/connect [post].
func (h *Handlers) connectDevice(c *gin.Context) {
	var req ConnectRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	device, err := h.manager.Connect(req)
	if err != nil {
		if err == ErrUPSAlreadyConnected {
			api.Conflict(c, "UPS 设备已连接")
			return
		}
		if err == ErrProtocolNotSupported {
			api.BadRequest(c, "不支持的协议")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, device)
}

// disconnectDevice 断开 UPS 设备
// @Summary 断开 UPS 设备
// @Description 断开并移除 UPS 设备
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id} [delete].
func (h *Handlers) disconnectDevice(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	if err := h.manager.Disconnect(upsID); err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "UPS 设备已断开", nil)
}

// ========== 电源状态 ==========

// getPowerStatus 获取 UPS 电源状态
// @Summary 获取电源状态
// @Description 获取指定 UPS 的电源状态
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Success 200 {object} api.Response{data=PowerStatus}
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id}/power [get].
func (h *Handlers) getPowerStatus(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	status, err := h.manager.GetPowerStatus(upsID)
	if err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, status)
}

// getAllPowerStatus 获取所有 UPS 电源状态
// @Summary 获取所有电源状态
// @Description 获取所有 UPS 的电源状态
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=map[string]PowerStatus}
// @Router /ups/power [get].
func (h *Handlers) getAllPowerStatus(c *gin.Context) {
	status := h.manager.GetAllPowerStatus()
	api.OK(c, status)
}

// getPrimaryPowerStatus 获取主 UPS 电源状态
// @Summary 获取主 UPS 电源状态
// @Description 获取主 UPS 的电源状态
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=PowerStatus}
// @Failure 404 {object} api.Response
// @Router /ups/power/primary [get].
func (h *Handlers) getPrimaryPowerStatus(c *gin.Context) {
	status, err := h.manager.GetPrimaryPowerStatus()
	if err != nil {
		if err == ErrNoPrimaryUPS {
			api.NotFound(c, "没有主 UPS 设备")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, status)
}

// ========== 硬件健康 ==========

// getHardwareHealth 获取硬件健康信息
// @Summary 获取硬件健康
// @Description 获取 UPS 关联的硬件健康信息
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Success 200 {object} api.Response{data=HardwareHealth}
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id}/health [get].
func (h *Handlers) getHardwareHealth(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	health, err := h.manager.GetHardwareHealth(upsID)
	if err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, health)
}

// ========== 关机策略 ==========

// createShutdownPolicy 创建关机策略
// @Summary 创建关机策略
// @Description 创建新的优雅关机策略
// @Tags ups
// @Accept json
// @Produce json
// @Param request body SetShutdownPolicyRequest true "策略信息"
// @Success 201 {object} api.Response{data=ShutdownPolicy}
// @Failure 400 {object} api.Response
// @Router /ups/policies [post].
func (h *Handlers) createShutdownPolicy(c *gin.Context) {
	var req SetShutdownPolicyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	policy, err := h.manager.CreateShutdownPolicy(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, policy)
}

// listShutdownPolicies 列出所有关机策略
// @Summary 列出关机策略
// @Description 获取所有关机策略
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=[]ShutdownPolicy}
// @Router /ups/policies [get].
func (h *Handlers) listShutdownPolicies(c *gin.Context) {
	policies := h.manager.ListShutdownPolicies()
	api.OK(c, policies)
}

// getShutdownPolicy 获取关机策略详情
// @Summary 获取关机策略
// @Description 根据 ID 获取关机策略详情
// @Tags ups
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response{data=ShutdownPolicy}
// @Failure 404 {object} api.Response
// @Router /ups/policies/{id} [get].
func (h *Handlers) getShutdownPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	policy, err := h.manager.GetShutdownPolicy(policyID)
	if err != nil {
		if err == ErrShutdownPolicyNotFound {
			api.NotFound(c, "关机策略不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// updateShutdownPolicy 更新关机策略
// @Summary 更新关机策略
// @Description 更新指定的关机策略
// @Tags ups
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Param request body SetShutdownPolicyRequest true "策略信息"
// @Success 200 {object} api.Response{data=ShutdownPolicy}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ups/policies/{id} [put].
func (h *Handlers) updateShutdownPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	var req SetShutdownPolicyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	policy, err := h.manager.UpdateShutdownPolicy(policyID, req)
	if err != nil {
		if err == ErrShutdownPolicyNotFound {
			api.NotFound(c, "关机策略不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// deleteShutdownPolicy 删除关机策略
// @Summary 删除关机策略
// @Description 删除指定的关机策略
// @Tags ups
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ups/policies/{id} [delete].
func (h *Handlers) deleteShutdownPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	if err := h.manager.DeleteShutdownPolicy(policyID); err != nil {
		if err == ErrShutdownPolicyNotFound {
			api.NotFound(c, "关机策略不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "关机策略已删除", nil)
}

// ========== 电源事件 ==========

// getEvents 获取电源事件
// @Summary 获取电源事件
// @Description 获取电源事件列表，支持过滤和分页
// @Tags ups
// @Produce json
// @Param upsId query string false "UPS ID 过滤"
// @Param type query string false "事件类型过滤"
// @Param severity query string false "严重级别过滤"
// @Param limit query int false "每页数量" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} api.Response{data=[]PowerEvent}
// @Router /ups/events [get].
func (h *Handlers) getEvents(c *gin.Context) {
	var params EventQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		api.BadRequest(c, "参数错误")
		return
	}

	events := h.manager.GetEvents(params)
	api.OK(c, events)
}

// getEventCount 获取事件总数
// @Summary 获取事件总数
// @Description 获取电源事件总数
// @Tags ups
// @Produce json
// @Param upsId query string false "UPS ID 过滤"
// @Success 200 {object} api.Response{data=int}
// @Router /ups/events/count [get].
func (h *Handlers) getEventCount(c *gin.Context) {
	upsID := c.Query("upsId")
	count := h.manager.GetEventCount(upsID)
	api.OK(c, count)
}

// ========== 电源统计 ==========

// getPowerStats 获取电源统计
// @Summary 获取电源统计
// @Description 获取指定 UPS 的电源统计数据
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Success 200 {object} api.Response{data=PowerStats}
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id}/stats [get].
func (h *Handlers) getPowerStats(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	stats, err := h.manager.GetPowerStats(upsID)
	if err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// ========== 配置管理 ==========

// getConfig 获取配置
// @Summary 获取配置
// @Description 获取 UPS 管理器配置
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=Config}
// @Router /ups/config [get].
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	api.OK(c, config)
}

// updateConfig 更新配置
// @Summary 更新配置
// @Description 更新 UPS 管理器配置
// @Tags ups
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置信息"
// @Success 200 {object} api.Response{data=Config}
// @Failure 400 {object} api.Response
// @Router /ups/config [put].
func (h *Handlers) updateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	config := h.manager.UpdateConfig(req)
	api.OK(c, config)
}

// ========== 运行控制 ==========

// start 启动 UPS 管理器
// @Summary 启动管理器
// @Description 启动 UPS 管理器定时轮询
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response
// @Router /ups/start [post].
func (h *Handlers) start(c *gin.Context) {
	h.manager.Start()
	api.OKWithMessage(c, "UPS 管理器已启动", nil)
}

// stop 停止 UPS 管理器
// @Summary 停止管理器
// @Description 停止 UPS 管理器定时轮询
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response
// @Router /ups/stop [post].
func (h *Handlers) stop(c *gin.Context) {
	h.manager.Stop()
	api.OKWithMessage(c, "UPS 管理器已停止", nil)
}

// getStatusSummary 获取状态摘要
// @Summary 获取状态摘要
// @Description 获取 UPS 管理器状态摘要
// @Tags ups
// @Produce json
// @Success 200 {object} api.Response{data=map[string]interface{}}
// @Router /ups/status [get].
func (h *Handlers) getStatusSummary(c *gin.Context) {
	summary := h.manager.GetStatusSummary()
	api.OK(c, summary)
}

// setDeviceStatus 设置设备状态（测试用）
// @Summary 设置设备状态
// @Description 手动设置 UPS 设备状态
// @Tags ups
// @Produce json
// @Param id path string true "UPS ID"
// @Param status query string true "状态" Enums(online, on_battery, low_battery, charging, fault, disconnected)
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ups/devices/{id}/status [put].
func (h *Handlers) setDeviceStatus(c *gin.Context) {
	upsID := c.Param("id")
	if upsID == "" {
		api.BadRequest(c, "UPS ID 不能为空")
		return
	}

	status := c.Query("status")
	if status == "" {
		api.BadRequest(c, "状态不能为空")
		return
	}

	if err := h.manager.SetUPSStatus(upsID, UPSStatus(status)); err != nil {
		if err == ErrUPSNotFound {
			api.NotFound(c, "UPS 设备不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备状态已更新", nil)
}
