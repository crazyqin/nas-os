// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== Hot Spare (热备盘) 管理 ==========

// listHotSpares 列出热备盘
// @Summary 列出热备盘
// @Description 列出所有或指定卷的热备盘
// @Tags storage
// @Param volume query string false "卷名称过滤"
// @Success 200 {object} api.Response{data=[]HotSpare}
// @Router /hot-spare [get].
func (h *Handlers) listHotSpares(c *gin.Context) {
	volumeName := c.Query("volume")
	result := h.hotSpareManager.ListHotSpares(volumeName)
	api.OK(c, result)
}

// getHotSpareStatus 获取热备盘系统状态
// @Summary 获取热备盘系统状态
// @Description 获取热备盘系统的整体状态
// @Tags storage
// @Success 200 {object} api.Response{data=HotSpareStatus}
// @Router /hot-spare/status [get].
func (h *Handlers) getHotSpareStatus(c *gin.Context) {
	status := h.hotSpareManager.GetStatus()
	api.OK(c, status)
}

// AddHotSpareRequest 添加热备盘请求.
type AddHotSpareRequest struct {
	Device     string `json:"device" binding:"required"`
	VolumeName string `json:"volumeName"` // 可选：指定关联的卷
}

// addHotSpare 添加热备盘
// @Summary 添加热备盘
// @Description 添加设备作为热备盘
// @Tags storage
// @Accept json
// @Param request body AddHotSpareRequest true "添加请求"
// @Success 201 {object} api.Response{data=HotSpare}
// @Failure 400 {object} api.Response
// @Router /hot-spare [post].
func (h *Handlers) addHotSpare(c *gin.Context) {
	var req AddHotSpareRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	hs, err := h.hotSpareManager.AddHotSpare(req.Device, req.VolumeName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, hs)
}

// removeHotSpare 移除热备盘
// @Summary 移除热备盘
// @Description 移除指定的热备盘
// @Tags storage
// @Param device path string true "设备路径"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /hot-spare/{device} [delete].
func (h *Handlers) removeHotSpare(c *gin.Context) {
	device := c.Param("device")

	if err := h.hotSpareManager.RemoveHotSpare(device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "热备盘已移除", nil)
}

// getHotSpare 获取热备盘详情
// @Summary 获取热备盘详情
// @Description 获取指定热备盘的详细信息
// @Tags storage
// @Param device path string true "设备路径"
// @Success 200 {object} api.Response{data=HotSpare}
// @Failure 404 {object} api.Response
// @Router /hot-spare/{device} [get].
func (h *Handlers) getHotSpare(c *gin.Context) {
	device := c.Param("device")

	hs, err := h.hotSpareManager.GetHotSpare(device)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, hs)
}

// ActivateHotSpareRequest 激活热备盘请求.
type ActivateHotSpareRequest struct {
	VolumeName   string `json:"volumeName" binding:"required"`
	FailedDevice string `json:"failedDevice" binding:"required"`
}

// activateHotSpare 激活热备盘
// @Summary 激活热备盘
// @Description 手动激活热备盘进行重建
// @Tags storage
// @Accept json
// @Param device path string true "设备路径"
// @Param request body ActivateHotSpareRequest true "激活请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /hot-spare/{device}/activate [post].
func (h *Handlers) activateHotSpare(c *gin.Context) {
	device := c.Param("device")

	var req ActivateHotSpareRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.hotSpareManager.ActivateHotSpare(device, req.VolumeName, req.FailedDevice); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "热备盘已激活，正在开始重建", nil)
}

// cancelRebuild 取消重建
// @Summary 取消重建
// @Description 取消正在进行的重建任务
// @Tags storage
// @Param device path string true "设备路径"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /hot-spare/{device}/cancel [post].
func (h *Handlers) cancelRebuild(c *gin.Context) {
	device := c.Param("device")

	if err := h.hotSpareManager.CancelRebuild(device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "重建已取消", nil)
}

// getRebuildStatus 获取重建状态
// @Summary 获取重建状态
// @Description 获取指定热备盘的重建状态
// @Tags storage
// @Param device path string true "设备路径"
// @Success 200 {object} api.Response{data=RebuildStatus}
// @Failure 404 {object} api.Response
// @Router /hot-spare/{device}/rebuild-status [get].
func (h *Handlers) getRebuildStatus(c *gin.Context) {
	device := c.Param("device")

	status, err := h.hotSpareManager.GetRebuildStatus(device)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, status)
}

// listRebuilding 列出正在重建的热备盘
// @Summary 列出正在重建的热备盘
// @Description 列出所有正在重建的热备盘
// @Tags storage
// @Success 200 {object} api.Response{data=[]RebuildStatus}
// @Router /hot-spare/rebuilding [get].
func (h *Handlers) listRebuilding(c *gin.Context) {
	result := h.hotSpareManager.ListRebuilding()
	api.OK(c, result)
}

// getHotSpareConfig 获取热备盘配置
// @Summary 获取热备盘配置
// @Description 获取热备盘系统的配置
// @Tags storage
// @Success 200 {object} api.Response{data=HotSpareConfig}
// @Router /hot-spare/config [get].
func (h *Handlers) getHotSpareConfig(c *gin.Context) {
	config := h.hotSpareManager.GetConfig()
	api.OK(c, config)
}

// updateHotSpareConfig 更新热备盘配置
// @Summary 更新热备盘配置
// @Description 更新热备盘系统的配置
// @Tags storage
// @Accept json
// @Param request body HotSpareConfig true "配置请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /hot-spare/config [put].
func (h *Handlers) updateHotSpareConfig(c *gin.Context) {
	var config HotSpareConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.hotSpareManager.SetConfig(config)
	api.OKWithMessage(c, "配置已更新", nil)
}

