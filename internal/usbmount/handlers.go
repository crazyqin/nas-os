// Package usbmount 提供 USB 设备自动挂载管理功能
package usbmount

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers API 处理器.
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
	usb := r.Group("/usb")
	{
		// 设备管理
		usb.GET("/devices", h.listDevices)
		usb.GET("/devices/:id", h.getDevice)
		usb.POST("/devices/:id/mount", h.mountDevice)
		usb.POST("/devices/:id/unmount", h.unmountDevice)
		usb.POST("/devices/mount-all", h.mountAllDevices)
		usb.POST("/devices/unmount-all", h.unmountAllDevices)

		// 规则管理
		usb.GET("/rules", h.listRules)
		usb.POST("/rules", h.createRule)
		usb.GET("/rules/:id", h.getRule)
		usb.PUT("/rules/:id", h.updateRule)
		usb.DELETE("/rules/:id", h.deleteRule)

		// 配置管理
		usb.GET("/config", h.getConfig)
		usb.PUT("/config", h.updateConfig)

		// 状态
		usb.GET("/status", h.getStatus)
	}
}

// ========== 设备管理 ==========

// listDevices 列出所有设备
// @Summary 列出 USB 设备
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices [get].
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    devices,
	})
}

// getDevice 获取设备详情
// @Summary 获取设备详情
// @Tags USB Mount
// @Produce json
// @Param id path string true "设备 ID"
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices/{id} [get].
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")

	device, err := h.manager.GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    device,
	})
}

// mountDevice 挂载设备
// @Summary 挂载 USB 设备
// @Tags USB Mount
// @Accept json
// @Produce json
// @Param id path string true "设备 ID"
// @Param body body MountRequest false "挂载选项"
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices/{id}/mount [post].
func (h *Handlers) mountDevice(c *gin.Context) {
	id := c.Param("id")

	var req MountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req = MountRequest{}
	}

	result, err := h.manager.Mount(id, req.MountPoint, req.Options)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "设备已挂载",
		"data":    result,
	})
}

// MountRequest 挂载请求.
type MountRequest struct {
	MountPoint string            `json:"mountPoint"`
	Options    map[string]string `json:"options"`
}

// unmountDevice 卸载设备
// @Summary 卸载 USB 设备
// @Tags USB Mount
// @Produce json
// @Param id path string true "设备 ID"
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices/{id}/unmount [post].
func (h *Handlers) unmountDevice(c *gin.Context) {
	id := c.Param("id")

	err := h.manager.Unmount(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "设备已卸载",
	})
}

// mountAllDevices 挂载所有设备
// @Summary 挂载所有 USB 设备
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices/mount-all [post].
func (h *Handlers) mountAllDevices(c *gin.Context) {
	results := h.manager.MountAll()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量挂载完成",
		"data":    results,
	})
}

// unmountAllDevices 卸载所有设备
// @Summary 卸载所有 USB 设备
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/devices/unmount-all [post].
func (h *Handlers) unmountAllDevices(c *gin.Context) {
	errors := h.manager.UnmountAll()

	if len(errors) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "部分设备卸载失败",
			"errors":  errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "所有设备已卸载",
	})
}

// ========== 规则管理 ==========

// listRules 列出所有规则
// @Summary 列出挂载规则
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/rules [get].
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// getRule 获取规则详情
// @Summary 获取规则详情
// @Tags USB Mount
// @Produce json
// @Param id path string true "规则 ID"
// @Success 200 {object} map[string]interface{}
// @Router /usb/rules/{id} [get].
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

// createRule 创建规则
// @Summary 创建挂载规则
// @Tags USB Mount
// @Accept json
// @Produce json
// @Param body body MountRule true "规则配置"
// @Success 200 {object} map[string]interface{}
// @Router /usb/rules [post].
func (h *Handlers) createRule(c *gin.Context) {
	var rule MountRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则创建成功",
		"data":    rule,
	})
}

// updateRule 更新规则
// @Summary 更新挂载规则
// @Tags USB Mount
// @Accept json
// @Produce json
// @Param id path string true "规则 ID"
// @Param body body MountRule true "规则配置"
// @Success 200 {object} map[string]interface{}
// @Router /usb/rules/{id} [put].
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")

	var rule MountRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateRule(id, &rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则更新成功",
		"data":    rule,
	})
}

// deleteRule 删除规则
// @Summary 删除挂载规则
// @Tags USB Mount
// @Produce json
// @Param id path string true "规则 ID"
// @Success 200 {object} map[string]interface{}
// @Router /usb/rules/{id} [delete].
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已删除",
	})
}

// ========== 配置管理 ==========

// getConfig 获取配置
// @Summary 获取 USB 挂载配置
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/config [get].
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// updateConfig 更新配置
// @Summary 更新 USB 挂载配置
// @Tags USB Mount
// @Accept json
// @Produce json
// @Param body body Config true "配置"
// @Success 200 {object} map[string]interface{}
// @Router /usb/config [put].
func (h *Handlers) updateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置更新成功",
		"data":    config,
	})
}

// ========== 状态 ==========

// getStatus 获取状态
// @Summary 获取 USB 挂载服务状态
// @Tags USB Mount
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /usb/status [get].
func (h *Handlers) getStatus(c *gin.Context) {
	devices := h.manager.ListDevices()

	var mountedCount, unmountedCount int
	for _, d := range devices {
		if d.Mounted {
			mountedCount++
		} else {
			unmountedCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"running":          h.manager.IsRunning(),
			"totalDevices":     len(devices),
			"mountedDevices":   mountedCount,
			"unmountedDevices": unmountedCount,
			"totalRules":       len(h.manager.ListRules()),
			"autoMount":        h.manager.GetConfig().AutoMount,
			"timestamp":        time.Now(),
		},
	})
}
