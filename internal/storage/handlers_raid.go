// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== RAID 配置 ==========

// ConvertRAIDRequest RAID 转换请求.
type ConvertRAIDRequest struct {
	DataProfile string `json:"dataProfile"`
	MetaProfile string `json:"metaProfile"`
}

// convertRAID 转换 RAID 配置
// @Summary 转换 RAID 配置
// @Description 转换卷的 RAID 配置
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param request body ConvertRAIDRequest true "转换请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/convert [post].
func (h *Handlers) convertRAID(c *gin.Context) {
	volumeName := c.Param("name")

	var req ConvertRAIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.ConvertRAID(volumeName, req.DataProfile, req.MetaProfile); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RAID 配置转换已启动", nil)
}

// getRAIDConfigs 获取 RAID 配置信息
// @Summary 获取 RAID 配置
// @Description 获取所有支持的 RAID 级别配置
// @Tags storage
// @Produce json
// @Success 200 {object} api.Response
// @Router /raid-configs [get].
func (h *Handlers) getRAIDConfigs(c *gin.Context) {
	api.OK(c, RAIDConfigs)
}

