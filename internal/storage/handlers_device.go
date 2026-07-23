// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 设备管理 ==========

// getDeviceStats 获取设备统计
// @Summary 获取设备统计
// @Description 获取卷中各设备的统计信息
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/devices [get].
func (h *Handlers) getDeviceStats(c *gin.Context) {
	volumeName := c.Param("name")

	stats, err := h.manager.GetDeviceStats(volumeName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// AddDeviceRequest 添加设备请求.
type AddDeviceRequest struct {
	Device string `json:"device" binding:"required"`
}

// addDevice 添加设备
// @Summary 添加设备
// @Description 向卷添加新设备
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param request body AddDeviceRequest true "添加请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/devices [post].
func (h *Handlers) addDevice(c *gin.Context) {
	volumeName := c.Param("name")

	var req AddDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.AddDevice(volumeName, req.Device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备已添加", nil)
}

// removeDevice 移除设备
// @Summary 移除设备
// @Description 从卷中移除设备
// @Tags storage
// @Param name path string true "卷名称"
// @Param device path string true "设备路径"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/devices/{device} [delete].
func (h *Handlers) removeDevice(c *gin.Context) {
	volumeName := c.Param("name")
	device := c.Param("device")

	if err := h.manager.RemoveDevice(volumeName, device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备已移除", nil)
}

