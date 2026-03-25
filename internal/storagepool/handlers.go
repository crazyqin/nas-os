// Package storagepool 提供存储池管理功能
package storagepool

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 存储池 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	pools := r.Group("/pools")
	{
		// 存储池管理
		pools.GET("", h.listPools)
		pools.POST("", h.createPool)
		pools.GET("/:id", h.getPool)
		pools.DELETE("/:id", h.deletePool)

		// 设备管理
		pools.POST("/:id/devices", h.addDevice)
		pools.DELETE("/:id/devices", h.removeDevice)

		// 扩容/缩容
		pools.POST("/:id/resize", h.resizePool)

		// 统计信息
		pools.GET("/:id/stats", h.getPoolStats)
	}

	// 可用设备
	r.GET("/devices/available", h.listAvailableDevices)

	// RAID 配置
	r.GET("/raid-configs", h.getRAIDConfigs)
}

// ========== 存储池管理 ==========

// listPools 列出所有存储池
// @Summary 列出所有存储池
// @Description 获取系统中所有存储池的列表
// @Tags storagepool
// @Produce json
// @Success 200 {object} api.Response{data=[]Pool}
// @Router /pools [get].
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	api.OK(c, pools)
}

// getPool 获取单个存储池
// @Summary 获取存储池详情
// @Description 根据ID获取存储池详细信息
// @Tags storagepool
// @Produce json
// @Param id path string true "存储池ID"
// @Success 200 {object} api.Response{data=Pool}
// @Failure 404 {object} api.Response
// @Router /pools/{id} [get].
func (h *Handlers) getPool(c *gin.Context) {
	id := c.Param("id")

	pool, err := h.manager.GetPool(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// createPool 创建存储池
// @Summary 创建存储池
// @Description 使用指定设备和RAID级别创建新的存储池
// @Tags storagepool
// @Accept json
// @Produce json
// @Param request body CreatePoolRequest true "创建请求"
// @Success 201 {object} api.Response{data=Pool}
// @Failure 400 {object} api.Response
// @Router /pools [post].
func (h *Handlers) createPool(c *gin.Context) {
	var req CreatePoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.manager.CreatePool(&req)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, pool)
}

// deletePool 删除存储池
// @Summary 删除存储池
// @Description 删除指定存储池（需要先清空数据或强制删除）
// @Tags storagepool
// @Param id path string true "存储池ID"
// @Param force query bool false "强制删除"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /pools/{id} [delete].
func (h *Handlers) deletePool(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.DeletePool(id, force); err != nil {
		if err.Error() == "存储池不存在: "+id {
			api.NotFound(c, err.Error())
			return
		}
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// ========== 设备管理 ==========

// addDevice 添加设备到存储池
// @Summary 添加设备
// @Description 向存储池添加新设备或热备盘
// @Tags storagepool
// @Accept json
// @Produce json
// @Param id path string true "存储池ID"
// @Param request body AddDeviceRequest true "添加设备请求"
// @Success 200 {object} api.Response{data=Pool}
// @Failure 400,404 {object} api.Response
// @Router /pools/{id}/devices [post].
func (h *Handlers) addDevice(c *gin.Context) {
	poolID := c.Param("id")

	var req AddDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.manager.AddDevice(poolID, &req)
	if err != nil {
		if err.Error() == "存储池不存在: "+poolID {
			api.NotFound(c, err.Error())
			return
		}
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// removeDevice 从存储池移除设备
// @Summary 移除设备
// @Description 从存储池移除指定设备
// @Tags storagepool
// @Accept json
// @Produce json
// @Param id path string true "存储池ID"
// @Param request body RemoveDeviceRequest true "移除设备请求"
// @Success 200 {object} api.Response{data=Pool}
// @Failure 400,404 {object} api.Response
// @Router /pools/{id}/devices [delete].
func (h *Handlers) removeDevice(c *gin.Context) {
	poolID := c.Param("id")

	var req RemoveDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.manager.RemoveDevice(poolID, &req)
	if err != nil {
		if err.Error() == "存储池不存在: "+poolID {
			api.NotFound(c, err.Error())
			return
		}
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// ========== 扩容/缩容 ==========

// resizePool 扩容或缩容存储池
// @Summary 扩容/缩容存储池
// @Description 调整存储池大小，可添加/移除设备或更改RAID级别
// @Tags storagepool
// @Accept json
// @Produce json
// @Param id path string true "存储池ID"
// @Param request body ResizePoolRequest true "调整请求"
// @Success 200 {object} api.Response{data=Pool}
// @Failure 400,404 {object} api.Response
// @Router /pools/{id}/resize [post].
func (h *Handlers) resizePool(c *gin.Context) {
	poolID := c.Param("id")

	var req ResizePoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.manager.ResizePool(poolID, &req)
	if err != nil {
		if err.Error() == "存储池不存在: "+poolID {
			api.NotFound(c, err.Error())
			return
		}
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// ========== 统计信息 ==========

// getPoolStats 获取存储池统计信息
// @Summary 获取统计信息
// @Description 获取存储池的详细统计和健康状态
// @Tags storagepool
// @Produce json
// @Param id path string true "存储池ID"
// @Success 200 {object} api.Response{data=map[string]interface{}}
// @Failure 404 {object} api.Response
// @Router /pools/{id}/stats [get].
func (h *Handlers) getPoolStats(c *gin.Context) {
	poolID := c.Param("id")

	stats, err := h.manager.GetPoolStats(poolID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// ========== 可用设备 ==========

// listAvailableDevices 列出可用设备
// @Summary 列出可用设备
// @Description 获取系统中未被使用的可用设备列表
// @Tags storagepool
// @Produce json
// @Success 200 {object} api.Response{data=[]Device}
// @Router /devices/available [get].
func (h *Handlers) listAvailableDevices(c *gin.Context) {
	devices := h.manager.GetAvailableDevices()
	api.OK(c, devices)
}

// ========== RAID 配置 ==========

// getRAIDConfigs 获取 RAID 配置信息
// @Summary 获取 RAID 配置
// @Description 获取所有支持的 RAID 级别配置信息
// @Tags storagepool
// @Produce json
// @Success 200 {object} api.Response{data=map[string]RAIDConfig}
// @Router /raid-configs [get].
func (h *Handlers) getRAIDConfigs(c *gin.Context) {
	api.OK(c, RAIDConfigs)
}
