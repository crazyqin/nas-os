// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== Fusion Pool 智能分层存储 ==========

// listFusionPools 列出所有融合池
// @Summary 列出所有融合池
// @Description 获取系统中所有 Fusion Pool 的列表
// @Tags storage
// @Produce json
// @Success 200 {object} api.Response{data=[]FusionPool}
// @Router /fusion-pools [get].
func (h *Handlers) listFusionPools(c *gin.Context) {
	pools := h.fusionManager.ListPools()
	api.OK(c, pools)
}

// getFusionPool 获取融合池详情
// @Summary 获取融合池详情
// @Description 根据名称获取融合池详细信息
// @Tags storage
// @Produce json
// @Param name path string true "融合池名称"
// @Success 200 {object} api.Response{data=FusionPool}
// @Failure 404 {object} api.Response
// @Router /fusion-pools/{name} [get].
func (h *Handlers) getFusionPool(c *gin.Context) {
	name := c.Param("name")

	pool := h.fusionManager.GetPool(name)
	if pool == nil {
		api.NotFound(c, "融合池不存在: "+name)
		return
	}

	api.OK(c, pool)
}

// createFusionPool 创建融合池
// @Summary 创建融合池
// @Description 创建新的智能分层存储池
// @Tags storage
// @Accept json
// @Produce json
// @Param request body CreateFusionPoolRequest true "创建请求"
// @Success 201 {object} api.Response{data=FusionPool}
// @Failure 400 {object} api.Response
// @Router /fusion-pools [post].
func (h *Handlers) createFusionPool(c *gin.Context) {
	var req CreateFusionPoolRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	pool, err := h.fusionManager.CreateFusionPool(&req)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, pool)
}

// deleteFusionPool 删除融合池
// @Summary 删除融合池
// @Description 删除指定融合池（危险操作）
// @Tags storage
// @Param name path string true "融合池名称"
// @Param force query bool false "强制删除（包含子卷）"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name} [delete].
func (h *Handlers) deleteFusionPool(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.fusionManager.DeletePool(name, force); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// listFusionSubvolumes 列出融合池的子卷
// @Summary 列出融合池子卷
// @Description 列出指定融合池的所有子卷
// @Tags storage
// @Param name path string true "融合池名称"
// @Success 200 {object} api.Response{data=[]FusionSubvolume}
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/subvolumes [get].
func (h *Handlers) listFusionSubvolumes(c *gin.Context) {
	name := c.Param("name")

	pool := h.fusionManager.GetPool(name)
	if pool == nil {
		api.NotFound(c, "融合池不存在: "+name)
		return
	}

	api.OK(c, pool.Subvolumes)
}

// createFusionSubvolume 创建融合池子卷
// @Summary 创建融合池子卷
// @Description 在指定融合池中创建新的子卷
// @Tags storage
// @Accept json
// @Produce json
// @Param name path string true "融合池名称"
// @Param request body map[string]string true "创建请求 {name: \"子卷名称\"}"
// @Success 201 {object} api.Response{data=FusionSubvolume}
// @Failure 400 {object} api.Response
// @Router /fusion-pools/{name}/subvolumes [post].
func (h *Handlers) createFusionSubvolume(c *gin.Context) {
	poolName := c.Param("name")

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	subvol, err := h.fusionManager.CreateSubvolume(poolName, req.Name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, subvol)
}

// getFusionSubvolume 获取融合池子卷详情
// @Summary 获取融合池子卷详情
// @Description 获取指定子卷的详细信息
// @Tags storage
// @Param name path string true "融合池名称"
// @Param subvol path string true "子卷名称"
// @Success 200 {object} api.Response{data=FusionSubvolume}
// @Failure 404 {object} api.Response
// @Router /fusion-pools/{name}/subvolumes/{subvol} [get].
func (h *Handlers) getFusionSubvolume(c *gin.Context) {
	poolName := c.Param("name")
	subvolName := c.Param("subvol")

	subvol, err := h.fusionManager.GetSubvolume(poolName, subvolName)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, subvol)
}

// deleteFusionSubvolume 删除融合池子卷
// @Summary 删除融合池子卷
// @Description 删除指定子卷
// @Tags storage
// @Param name path string true "融合池名称"
// @Param subvol path string true "子卷名称"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/subvolumes/{subvol} [delete].
func (h *Handlers) deleteFusionSubvolume(c *gin.Context) {
	poolName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := h.fusionManager.DeleteSubvolume(poolName, subvolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// addSSDDeviceRequest 添加 SSD 设备请求.
type addSSDDeviceRequest struct {
	Device string `json:"device" binding:"required"`
}

// addSSDDevice 添加 SSD 设备到融合池
// @Summary 添加 SSD 设备
// @Description 向融合池添加 SSD 设备以扩展元数据存储
// @Tags storage
// @Accept json
// @Param name path string true "融合池名称"
// @Param request body addSSDDeviceRequest true "设备信息"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/ssd-devices [post].
func (h *Handlers) addSSDDevice(c *gin.Context) {
	poolName := c.Param("name")

	var req addSSDDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.fusionManager.AddSSDDevice(poolName, req.Device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "SSD 设备已添加", nil)
}

// addHDDDevice 添加 HDD 设备到融合池
// @Summary 添加 HDD 设备
// @Description 向融合池添加 HDD 设备以扩展数据存储
// @Tags storage
// @Accept json
// @Param name path string true "融合池名称"
// @Param request body addSSDDeviceRequest true "设备信息"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/hdd-devices [post].
func (h *Handlers) addHDDDevice(c *gin.Context) {
	poolName := c.Param("name")

	var req addSSDDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.fusionManager.AddHDDDevice(poolName, req.Device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "HDD 设备已添加", nil)
}

// runTiering 执行分层任务
// @Summary 执行分层任务
// @Description 手动触发数据分层任务
// @Tags storage
// @Param name path string true "融合池名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/tiering [post].
func (h *Handlers) runTiering(c *gin.Context) {
	poolName := c.Param("name")

	if err := h.fusionManager.RunTiering(poolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "分层任务已启动", nil)
}

// optimizeMetadataAccess 优化元数据访问
// @Summary 优化元数据访问
// @Description 预热元数据缓存以加速访问
// @Tags storage
// @Param name path string true "融合池名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/optimize [post].
func (h *Handlers) optimizeMetadataAccess(c *gin.Context) {
	poolName := c.Param("name")

	if err := h.fusionManager.OptimizeMetadataAccess(poolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "元数据缓存已预热", nil)
}

// getFusionPoolStats 获取融合池统计信息
// @Summary 获取融合池统计信息
// @Description 获取融合池的详细统计信息
// @Tags storage
// @Param name path string true "融合池名称"
// @Success 200 {object} api.Response{data=FusionPoolStats}
// @Failure 400,404 {object} api.Response
// @Router /fusion-pools/{name}/stats [get].
func (h *Handlers) getFusionPoolStats(c *gin.Context) {
	poolName := c.Param("name")

	stats, err := h.fusionManager.GetPoolStats(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, stats)
}

