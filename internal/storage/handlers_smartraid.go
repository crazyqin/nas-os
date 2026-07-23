// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 智能 RAID (SmartRAID) ==========

// listSmartPools 列出所有智能池
// @Summary 列出所有智能池
// @Description 获取所有智能存储池列表
// @Tags storage
// @Success 200 {object} api.Response{data=[]SmartPool}
// @Router /smart-pools [get].
func (h *Handlers) listSmartPools(c *gin.Context) {
	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	pools := h.smartRAIDManager.ListPools()
	api.OK(c, pools)
}

// getSmartPool 获取智能池详情
// @Summary 获取智能池详情
// @Description 获取指定智能池的详细信息
// @Tags storage
// @Param name path string true "智能池名称"
// @Success 200 {object} api.Response{data=SmartPool}
// @Failure 404 {object} api.Response
// @Router /smart-pools/{name} [get].
func (h *Handlers) getSmartPool(c *gin.Context) {
	name := c.Param("name")

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	pool := h.smartRAIDManager.GetPool(name)
	if pool == nil {
		api.NotFound(c, "智能池不存在: "+name)
		return
	}

	api.OK(c, pool)
}

// createSmartPoolRequest 创建智能池请求.
type createSmartPoolRequest struct {
	Name            string      `json:"name" binding:"required"`
	Description     string      `json:"description"`
	Devices         []string    `json:"devices" binding:"required,min=1"`
	RAIDPolicy      *RAIDPolicy `json:"raidPolicy"`
	RedundancyLevel int         `json:"redundancyLevel"`
}

// createSmartPool 创建智能池
// @Summary 创建智能池
// @Description 创建新的智能存储池，支持不同容量硬盘混用
// @Tags storage
// @Accept json
// @Produce json
// @Param request body createSmartPoolRequest true "创建请求"
// @Success 201 {object} api.Response{data=SmartPool}
// @Failure 400 {object} api.Response
// @Router /smart-pools [post].
func (h *Handlers) createSmartPool(c *gin.Context) {
	var req createSmartPoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	pool, err := h.smartRAIDManager.CreateSmartPool(&CreateSmartPoolRequest{
		Name:            req.Name,
		Description:     req.Description,
		Devices:         req.Devices,
		RAIDPolicy:      req.RAIDPolicy,
		RedundancyLevel: req.RedundancyLevel,
	})
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, pool)
}

// deleteSmartPool 删除智能池
// @Summary 删除智能池
// @Description 删除指定智能池（危险操作）
// @Tags storage
// @Param name path string true "智能池名称"
// @Param force query bool false "强制删除（包含子卷）"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name} [delete].
func (h *Handlers) deleteSmartPool(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	if err := h.smartRAIDManager.DeletePool(name, force); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// addDeviceToSmartPoolRequest 添加设备请求.
type addDeviceToSmartPoolRequest struct {
	Device string `json:"device" binding:"required"`
}

// addDeviceToSmartPool 添加设备到智能池
// @Summary 添加设备到智能池
// @Description 向智能池添加新设备，支持在线扩容
// @Tags storage
// @Accept json
// @Param name path string true "智能池名称"
// @Param request body addDeviceToSmartPoolRequest true "设备信息"
// @Success 200 {object} api.Response{data=SmartPool}
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/devices [post].
func (h *Handlers) addDeviceToSmartPool(c *gin.Context) {
	poolName := c.Param("name")

	var req addDeviceToSmartPoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	pool, err := h.smartRAIDManager.AddDevice(poolName, req.Device)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, pool)
}

// replaceSmartPoolDeviceRequest 替换设备请求.
type replaceSmartPoolDeviceRequest struct {
	OldDevice string `json:"oldDevice" binding:"required"`
	NewDevice string `json:"newDevice" binding:"required"`
}

// replaceSmartPoolDevice 替换智能池设备
// @Summary 替换智能池设备
// @Description 用新设备替换智能池中的设备，支持用更大容量设备替换以扩展存储
// @Tags storage
// @Accept json
// @Param name path string true "智能池名称"
// @Param request body replaceSmartPoolDeviceRequest true "设备信息"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/replace [post].
func (h *Handlers) replaceSmartPoolDevice(c *gin.Context) {
	poolName := c.Param("name")

	var req replaceSmartPoolDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	if err := h.smartRAIDManager.ReplaceDevice(poolName, req.OldDevice, req.NewDevice); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备替换已启动", nil)
}

// getSmartPoolStats 获取智能池统计信息
// @Summary 获取智能池统计信息
// @Description 获取智能池的详细统计信息，包括容量、层级、设备类型等
// @Tags storage
// @Param name path string true "智能池名称"
// @Success 200 {object} api.Response{data=SmartPoolStats}
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/stats [get].
func (h *Handlers) getSmartPoolStats(c *gin.Context) {
	poolName := c.Param("name")

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	stats, err := h.smartRAIDManager.GetPoolStats(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// getExpansionPlan 获取扩容计划
// @Summary 获取扩容计划
// @Description 分析当前池状态，提供扩容建议
// @Tags storage
// @Param name path string true "智能池名称"
// @Success 200 {object} api.Response{data=ExpansionPlan}
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/expansion-plan [get].
func (h *Handlers) getExpansionPlan(c *gin.Context) {
	poolName := c.Param("name")

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	plan, err := h.smartRAIDManager.GetExpansionPlan(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, plan)
}

// listSmartPoolSubvolumes 列出智能池子卷
// @Summary 列出智能池子卷
// @Description 列出指定智能池的所有子卷
// @Tags storage
// @Param name path string true "智能池名称"
// @Success 200 {object} api.Response{data=[]SmartSubvolume}
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/subvolumes [get].
func (h *Handlers) listSmartPoolSubvolumes(c *gin.Context) {
	name := c.Param("name")

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	pool := h.smartRAIDManager.GetPool(name)
	if pool == nil {
		api.NotFound(c, "智能池不存在: "+name)
		return
	}

	api.OK(c, pool.Subvolumes)
}

// createSmartSubvolumeRequest 创建子卷请求.
type createSmartSubvolumeRequest struct {
	Name string `json:"name" binding:"required"`
}

// createSmartPoolSubvolume 创建智能池子卷
// @Summary 创建智能池子卷
// @Description 在指定智能池中创建新的子卷
// @Tags storage
// @Accept json
// @Param name path string true "智能池名称"
// @Param request body createSmartSubvolumeRequest true "创建请求"
// @Success 201 {object} api.Response{data=SmartSubvolume}
// @Failure 400 {object} api.Response
// @Router /smart-pools/{name}/subvolumes [post].
func (h *Handlers) createSmartPoolSubvolume(c *gin.Context) {
	poolName := c.Param("name")

	var req createSmartSubvolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	subvol, err := h.smartRAIDManager.CreateSubvolume(poolName, req.Name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, subvol)
}

// deleteSmartPoolSubvolume 删除智能池子卷
// @Summary 删除智能池子卷
// @Description 删除指定子卷
// @Tags storage
// @Param name path string true "智能池名称"
// @Param subvol path string true "子卷名称"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /smart-pools/{name}/subvolumes/{subvol} [delete].
func (h *Handlers) deleteSmartPoolSubvolume(c *gin.Context) {
	poolName := c.Param("name")
	subvolName := c.Param("subvol")

	if h.smartRAIDManager == nil {
		api.BadRequest(c, "智能 RAID 管理器未初始化")
		return
	}

	if err := h.smartRAIDManager.DeleteSubvolume(poolName, subvolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}
