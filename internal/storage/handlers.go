// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 存储 API 处理器.
type Handlers struct {
	manager          *Manager
	immutableManager *ImmutableManager
	hotSpareManager  *HotSpareManager
	spaceAnalyzer    *SpaceAnalyzer
	fusionManager    *FusionPoolManager
	smartRAIDManager *SmartRAIDManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, immutableManager *ImmutableManager, hotSpareManager *HotSpareManager, spaceAnalyzer *SpaceAnalyzer) *Handlers {
	return &Handlers{
		manager:          manager,
		immutableManager: immutableManager,
		hotSpareManager:  hotSpareManager,
		spaceAnalyzer:    spaceAnalyzer,
	}
}

// NewHandlersWithFusion 创建带融合池支持的处理器.
func NewHandlersWithFusion(manager *Manager, immutableManager *ImmutableManager, hotSpareManager *HotSpareManager, spaceAnalyzer *SpaceAnalyzer, fusionManager *FusionPoolManager) *Handlers {
	return &Handlers{
		manager:          manager,
		immutableManager: immutableManager,
		hotSpareManager:  hotSpareManager,
		spaceAnalyzer:    spaceAnalyzer,
		fusionManager:    fusionManager,
	}
}

// NewHandlersWithSmartRAID 创建带智能 RAID 支持的处理器.
func NewHandlersWithSmartRAID(manager *Manager, immutableManager *ImmutableManager, hotSpareManager *HotSpareManager, spaceAnalyzer *SpaceAnalyzer, fusionManager *FusionPoolManager, smartRAIDManager *SmartRAIDManager) *Handlers {
	return &Handlers{
		manager:          manager,
		immutableManager: immutableManager,
		hotSpareManager:  hotSpareManager,
		spaceAnalyzer:    spaceAnalyzer,
		fusionManager:    fusionManager,
		smartRAIDManager: smartRAIDManager,
	}
}

// RegisterRoutes 注册 domain 风格 /volumes 路由（非 nasd 主契约）。
//
// 生产 nasd 使用 internal/web.StorageHandlers 注册 /api/v1/storage/*（见 application
// 的 storage 模块与 web.registerCoreIdentityAndDocs）。本 Handler 的删卷同样经
// DeleteVolumeConfirmed 门闩，但路径不同，勿在 nasd 上重复挂载以免契约分叉。
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	// 卷管理
	volumes := r.Group("/volumes")
	{
		volumes.GET("", h.listVolumes)
		volumes.POST("", h.createVolume)
		volumes.GET("/:name", h.getVolume)
		volumes.DELETE("/:name", h.deleteVolume)

		// 卷操作
		volumes.POST("/:name/mount", h.mountVolume)
		volumes.POST("/:name/unmount", h.unmountVolume)
		volumes.POST("/:name/scrub", h.startScrub)
		volumes.GET("/:name/scrub/status", h.getScrubStatus)
		volumes.POST("/:name/balance", h.startBalance)
		volumes.GET("/:name/balance/status", h.getBalanceStatus)

		// 子卷管理
		volumes.GET("/:name/subvolumes", h.listSubvolumes)
		volumes.POST("/:name/subvolumes", h.createSubvolume)
		volumes.GET("/:name/subvolumes/:subvol", h.getSubvolume)
		volumes.DELETE("/:name/subvolumes/:subvol", h.deleteSubvolume)
		volumes.POST("/:name/subvolumes/:subvol/mount", h.mountSubvolume)
		volumes.POST("/:name/subvolumes/:subvol/readonly", h.setSubvolumeReadOnly)

		// 快照管理
		volumes.GET("/:name/snapshots", h.listSnapshots)
		volumes.POST("/:name/snapshots", h.createSnapshot)
		volumes.GET("/:name/snapshots/:snap", h.getSnapshot)
		volumes.DELETE("/:name/snapshots/:snap", h.deleteSnapshot)
		volumes.POST("/:name/snapshots/:snap/restore", h.restoreSnapshot)
		volumes.POST("/:name/snapshots/:snap/rollback", h.rollbackSnapshot)

		// 设备管理
		volumes.GET("/:name/devices", h.getDeviceStats)
		volumes.POST("/:name/devices", h.addDevice)
		volumes.DELETE("/:name/devices/:device", h.removeDevice)

		// RAID 转换
		volumes.POST("/:name/convert", h.convertRAID)
	}

	// 全局子卷列表（跨卷查询）
	r.GET("/subvolumes", h.listAllSubvolumes)

	// 全局快照列表（跨卷查询）
	r.GET("/snapshots", h.listAllSnapshots)

	// RAID 配置信息
	r.GET("/raid-configs", h.getRAIDConfigs)

	// Hot Spare (热备盘) 管理
	if h.hotSpareManager != nil {
		hotSpare := r.Group("/hot-spare")
		{
			hotSpare.GET("", h.listHotSpares)
			hotSpare.GET("/status", h.getHotSpareStatus)
			hotSpare.POST("", h.addHotSpare)
			hotSpare.DELETE("/:device", h.removeHotSpare)
			hotSpare.GET("/:device", h.getHotSpare)
			hotSpare.POST("/:device/activate", h.activateHotSpare)
			hotSpare.POST("/:device/cancel", h.cancelRebuild)
			hotSpare.GET("/:device/rebuild-status", h.getRebuildStatus)
			hotSpare.GET("/rebuilding", h.listRebuilding)
			hotSpare.GET("/config", h.getHotSpareConfig)
			hotSpare.PUT("/config", h.updateHotSpareConfig)
		}
	}

	// 空间分析
	if h.spaceAnalyzer != nil {
		space := r.Group("/space")
		{
			space.GET("/analyze/:volume", h.analyzeSpace)
			space.GET("/history/:volume", h.getSpaceHistory)
			space.GET("/trend/:volume", h.getSpaceTrend)
		}
	}

	// 不可变存储（WriteOnce）
	if h.immutableManager != nil {
		immutableHandlers := NewImmutableHandlers(h.immutableManager)
		immutableHandlers.RegisterRoutes(r)
	}

	// Fusion Pool（智能分层存储）
	if h.fusionManager != nil {
		fusion := r.Group("/fusion-pools")
		{
			fusion.GET("", h.listFusionPools)
			fusion.POST("", h.createFusionPool)
			fusion.GET("/:name", h.getFusionPool)
			fusion.DELETE("/:name", h.deleteFusionPool)

			// 子卷管理
			fusion.GET("/:name/subvolumes", h.listFusionSubvolumes)
			fusion.POST("/:name/subvolumes", h.createFusionSubvolume)
			fusion.GET("/:name/subvolumes/:subvol", h.getFusionSubvolume)
			fusion.DELETE("/:name/subvolumes/:subvol", h.deleteFusionSubvolume)

			// 设备管理
			fusion.POST("/:name/ssd-devices", h.addSSDDevice)
			fusion.POST("/:name/hdd-devices", h.addHDDDevice)

			// 分层操作
			fusion.POST("/:name/tiering", h.runTiering)
			fusion.POST("/:name/optimize", h.optimizeMetadataAccess)

			// 统计信息
			fusion.GET("/:name/stats", h.getFusionPoolStats)
		}
	}

	// SmartRAID（智能 RAID 管理，类似群晖 SHR）
	if h.smartRAIDManager != nil {
		smartPools := r.Group("/smart-pools")
		{
			smartPools.GET("", h.listSmartPools)
			smartPools.POST("", h.createSmartPool)
			smartPools.GET("/:name", h.getSmartPool)
			smartPools.DELETE("/:name", h.deleteSmartPool)

			// 子卷管理
			smartPools.GET("/:name/subvolumes", h.listSmartPoolSubvolumes)
			smartPools.POST("/:name/subvolumes", h.createSmartPoolSubvolume)
			smartPools.DELETE("/:name/subvolumes/:subvol", h.deleteSmartPoolSubvolume)

			// 设备管理
			smartPools.POST("/:name/devices", h.addDeviceToSmartPool)
			smartPools.POST("/:name/replace", h.replaceSmartPoolDevice)

			// 统计和规划
			smartPools.GET("/:name/stats", h.getSmartPoolStats)
			smartPools.GET("/:name/expansion-plan", h.getExpansionPlan)
		}
	}
}

// ========== 卷管理 ==========

// VolumeListResponse 卷列表响应.
type VolumeListResponse struct {
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Devices     []string `json:"devices"`
	Total       uint64   `json:"total"`
	Used        uint64   `json:"used"`
	Free        uint64   `json:"free"`
	Profile     string   `json:"profile"`
	MountPoint  string   `json:"mountPoint"`
	Healthy     bool     `json:"healthy"`
	SubvolCount int      `json:"subvolCount"`
}

func buildVolumeListResponses(volumes []*Volume) []VolumeListResponse {
	result := make([]VolumeListResponse, 0, len(volumes))
	for _, v := range volumes {
		result = append(result, VolumeListResponse{
			Name:        v.Name,
			UUID:        v.UUID,
			Devices:     v.Devices,
			Total:       v.Size,
			Used:        v.Used,
			Free:        v.Free,
			Profile:     v.DataProfile,
			MountPoint:  v.MountPoint,
			Healthy:     v.Status.Healthy,
			SubvolCount: len(v.Subvolumes),
		})
	}
	return result
}

func buildSnapshotListResponses(volumeName string, snapshots []*Snapshot, subvolFilter string) []SnapshotListResponse {
	result := make([]SnapshotListResponse, 0, len(snapshots))
	for _, snap := range snapshots {
		if subvolFilter != "" && snap.Source != subvolFilter {
			continue
		}
		snapType := "scheduled"
		if len(snap.Name) > 6 && snap.Name[:6] == "manual" {
			snapType = "manual"
		}
		result = append(result, SnapshotListResponse{
			Name:      snap.Name,
			Volume:    volumeName,
			Subvolume: snap.Source,
			Path:      snap.Path,
			ReadOnly:  snap.ReadOnly,
			Size:      snap.Size,
			CreatedAt: snap.CreatedAt.Format("2006-01-02 15:04"),
			Type:      snapType,
		})
	}
	return result
}

// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags storage
// @Produce json
// @Success 200 {object} api.Response{data=[]VolumeListResponse}
// @Router /volumes [get].
func (h *Handlers) listVolumes(c *gin.Context) {
	volumes := h.manager.ListVolumes()

	api.OK(c, buildVolumeListResponses(volumes))
}

// getVolume 获取卷详情
// @Summary 获取卷详情
// @Description 根据名称获取卷详细信息
// @Tags storage
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response{data=Volume}
// @Failure 404 {object} api.Response
// @Router /volumes/{name} [get].
func (h *Handlers) getVolume(c *gin.Context) {
	name := c.Param("name")

	vol := h.manager.GetVolume(name)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+name)
		return
	}

	api.OK(c, vol)
}

// CreateVolumeRequest 创建卷请求.
type CreateVolumeRequest struct {
	Name    string   `json:"name" binding:"required"`
	Devices []string `json:"devices" binding:"required,min=1"`
	Profile string   `json:"profile"` // single, raid0, raid1, raid5, raid6, raid10
}

// createVolume 创建卷
// @Summary 创建卷
// @Description 使用指定设备和 RAID 配置创建新的 Btrfs 卷
// @Tags storage
// @Accept json
// @Produce json
// @Param request body CreateVolumeRequest true "创建请求"
// @Success 201 {object} api.Response{data=Volume}
// @Failure 400 {object} api.Response
// @Router /volumes [post].
func (h *Handlers) createVolume(c *gin.Context) {
	var req CreateVolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if req.Profile == "" {
		req.Profile = "single"
	}

	vol, err := h.manager.CreateVolume(req.Name, req.Devices, req.Profile)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, vol)
}

// deleteVolume 删除卷
// @Summary 删除卷
// @Description 删除指定卷（危险操作）。必须提供 confirm_name（与卷名一致）与 allow_wipe=true。
// @Tags storage
// @Param name path string true "卷名称"
// @Param force query bool false "强制删除（包含子卷）"
// @Param body body DeleteVolumeOptions true "确认载荷：confirm_name + allow_wipe"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name} [delete].
func (h *Handlers) deleteVolume(c *gin.Context) {
	name := c.Param("name")
	var opts DeleteVolumeOptions
	_ = c.ShouldBindJSON(&opts)
	if c.Query("force") == "true" {
		opts.Force = true
	}

	if err := h.manager.DeleteVolumeConfirmed(name, opts); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// mountVolume 挂载卷
// @Summary 挂载卷
// @Description 挂载指定卷
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/mount [post].
func (h *Handlers) mountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.MountVolume(name); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "卷已挂载", nil)
}

// unmountVolume 卸载卷
// @Summary 卸载卷
// @Description 卸载指定卷
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/unmount [post].
func (h *Handlers) unmountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.UnmountVolume(name); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "卷已卸载", nil)
}

// ========== 维护操作 ==========

// startScrub 启动数据校验
// @Summary 启动数据校验
// @Description 启动卷的数据校验任务
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/scrub [post].
func (h *Handlers) startScrub(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.Scrub(name); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "数据校验已启动", nil)
}

// getScrubStatus 获取校验状态
// @Summary 获取校验状态
// @Description 获取卷的数据校验任务状态
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/scrub/status [get].
func (h *Handlers) getScrubStatus(c *gin.Context) {
	name := c.Param("name")

	status, err := h.manager.GetScrubStatus(name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

// startBalance 启动数据平衡
// @Summary 启动数据平衡
// @Description 启动卷的数据平衡任务
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/balance [post].
func (h *Handlers) startBalance(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.Balance(name); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "数据平衡已启动", nil)
}

// getBalanceStatus 获取平衡状态
// @Summary 获取平衡状态
// @Description 获取卷的数据平衡任务状态
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/balance/status [get].
func (h *Handlers) getBalanceStatus(c *gin.Context) {
	name := c.Param("name")

	status, err := h.manager.GetBalanceStatus(name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

