// Package gpupassthrough GPU直通管理API处理器
package gpupassthrough

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers GPU直通管理API处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由到 /gpupassthrough 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	gp := r.Group("/gpupassthrough")
	{
		// GPU设备管理
		gp.GET("/devices", h.listDevices)
		gp.GET("/devices/:id", h.getDevice)
		gp.GET("/devices/:id/stats", h.getDeviceStats)
		gp.POST("/devices/:id/assign", h.assignGPU)
		gp.DELETE("/devices/:id/assign/:vmid", h.unassignGPU)
		gp.POST("/devices/:id/bind", h.bindVFIO)
		gp.POST("/devices/:id/unbind", h.unbindVFIO)

		// 分配查询
		gp.GET("/assignments", h.listAssignments)

		// 告警查询
		gp.GET("/alerts", h.listAlerts)
	}
}

// listDevices 列出所有GPU设备
func (h *Handlers) listDevices(c *gin.Context) {
	// 先刷新设备列表
	if err := h.manager.DiscoverDevices(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: "发现设备失败: " + err.Error(),
		})
		return
	}

	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    devices,
	})
}

// getDevice 获取GPU设备详情
func (h *Handlers) getDevice(c *gin.Context) {
	pciAddr := c.Param("id")

	device, err := h.manager.GetDevice(pciAddr)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    device,
	})
}

// getDeviceStats 获取GPU设备统计
func (h *Handlers) getDeviceStats(c *gin.Context) {
	pciAddr := c.Param("id")

	stats, err := h.manager.GetDeviceStats(pciAddr)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    stats,
	})
}

// assignGPU 分配GPU
func (h *Handlers) assignGPU(c *gin.Context) {
	pciAddr := c.Param("id")

	var req AssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.AssignGPU(pciAddr, &req); err != nil {
		c.JSON(http.StatusConflict, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "分配成功",
		Data:    nil,
	})
}

// unassignGPU 取消分配
func (h *Handlers) unassignGPU(c *gin.Context) {
	pciAddr := c.Param("id")
	vmid := c.Param("vmid")

	if err := h.manager.UnassignGPU(pciAddr, vmid); err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "取消分配成功",
		Data:    nil,
	})
}

// bindVFIO 绑定VFIO驱动
func (h *Handlers) bindVFIO(c *gin.Context) {
	pciAddr := c.Param("id")

	if err := h.manager.BindVFIO(pciAddr); err != nil {
		c.JSON(http.StatusConflict, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "绑定VFIO驱动成功",
		Data:    nil,
	})
}

// unbindVFIO 解绑VFIO驱动
func (h *Handlers) unbindVFIO(c *gin.Context) {
	pciAddr := c.Param("id")

	if err := h.manager.UnbindVFIO(pciAddr); err != nil {
		c.JSON(http.StatusConflict, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "解绑驱动成功",
		Data:    nil,
	})
}

// listAssignments 列出所有分配
func (h *Handlers) listAssignments(c *gin.Context) {
	vmAssigns, containerAssigns := h.manager.GetAllAssignments()

	data := map[string]interface{}{
		"vmAssignments":       vmAssigns,
		"containerAssignments": containerAssigns,
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// listAlerts 列出告警
func (h *Handlers) listAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    alerts,
	})
}
