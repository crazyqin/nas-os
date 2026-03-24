// Package storage 提供存储管理 API 处理器
package storage

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 存储 API 处理器
type Handlers struct {
	manager          *Manager
	immutableManager *ImmutableManager
	hotSpareManager  *HotSpareManager
	spaceAnalyzer    *SpaceAnalyzer
	fusionManager    *FusionPoolManager
	smartRAIDManager *SmartRAIDManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager, immutableManager *ImmutableManager, hotSpareManager *HotSpareManager, spaceAnalyzer *SpaceAnalyzer) *Handlers {
	return &Handlers{
		manager:          manager,
		immutableManager: immutableManager,
		hotSpareManager:  hotSpareManager,
		spaceAnalyzer:    spaceAnalyzer,
	}
}

// NewHandlersWithFusion 创建带融合池支持的处理器
func NewHandlersWithFusion(manager *Manager, immutableManager *ImmutableManager, hotSpareManager *HotSpareManager, spaceAnalyzer *SpaceAnalyzer, fusionManager *FusionPoolManager) *Handlers {
	return &Handlers{
		manager:          manager,
		immutableManager: immutableManager,
		hotSpareManager:  hotSpareManager,
		spaceAnalyzer:    spaceAnalyzer,
		fusionManager:    fusionManager,
	}
}

// NewHandlersWithSmartRAID 创建带智能 RAID 支持的处理器
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

// RegisterRoutes 注册路由
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

// VolumeListResponse 卷列表响应
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

// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags storage
// @Produce json
// @Success 200 {object} api.Response{data=[]VolumeListResponse}
// @Router /volumes [get]
func (h *Handlers) listVolumes(c *gin.Context) {
	volumes := h.manager.ListVolumes()

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

	api.OK(c, result)
}

// getVolume 获取卷详情
// @Summary 获取卷详情
// @Description 根据名称获取卷详细信息
// @Tags storage
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response{data=Volume}
// @Failure 404 {object} api.Response
// @Router /volumes/{name} [get]
func (h *Handlers) getVolume(c *gin.Context) {
	name := c.Param("name")

	vol := h.manager.GetVolume(name)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+name)
		return
	}

	api.OK(c, vol)
}

// CreateVolumeRequest 创建卷请求
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
// @Router /volumes [post]
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
// @Description 删除指定卷（危险操作）
// @Tags storage
// @Param name path string true "卷名称"
// @Param force query bool false "强制删除（包含子卷）"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name} [delete]
func (h *Handlers) deleteVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.manager.DeleteVolume(name, force); err != nil {
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
// @Router /volumes/{name}/mount [post]
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
// @Router /volumes/{name}/unmount [post]
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
// @Router /volumes/{name}/scrub [post]
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
// @Router /volumes/{name}/scrub/status [get]
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
// @Router /volumes/{name}/balance [post]
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
// @Router /volumes/{name}/balance/status [get]
func (h *Handlers) getBalanceStatus(c *gin.Context) {
	name := c.Param("name")

	status, err := h.manager.GetBalanceStatus(name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

// ========== 子卷管理 ==========

// SubvolumeListResponse 子卷列表响应
type SubvolumeListResponse struct {
	Name          string `json:"name"`
	Volume        string `json:"volume"`
	Path          string `json:"path"`
	ID            uint64 `json:"id"`
	ParentID      uint64 `json:"parentId"`
	ReadOnly      bool   `json:"readOnly"`
	Size          uint64 `json:"size"`
	SnapshotCount int    `json:"snapshotCount"`
}

// listSubvolumes 列出卷的子卷
// @Summary 列出子卷
// @Description 列出指定卷的所有子卷
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response{data=[]SubvolumeListResponse}
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/subvolumes [get]
func (h *Handlers) listSubvolumes(c *gin.Context) {
	name := c.Param("name")

	subvols, err := h.manager.ListSubVolumes(name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result := make([]SubvolumeListResponse, 0, len(subvols))
	for _, sv := range subvols {
		result = append(result, SubvolumeListResponse{
			Name:          sv.Name,
			Volume:        name,
			Path:          sv.Path,
			ID:            sv.ID,
			ParentID:      sv.ParentID,
			ReadOnly:      sv.ReadOnly,
			Size:          sv.Size,
			SnapshotCount: len(sv.Snapshots),
		})
	}

	api.OK(c, result)
}

// listAllSubvolumes 列出所有子卷（跨卷）
// @Summary 列出所有子卷
// @Description 列出系统中所有卷的子卷
// @Tags storage
// @Param volume query string false "过滤卷名称"
// @Success 200 {object} api.Response{data=[]SubvolumeListResponse}
// @Router /subvolumes [get]
func (h *Handlers) listAllSubvolumes(c *gin.Context) {
	volumeFilter := c.Query("volume")

	volumes := h.manager.ListVolumes()
	var result []SubvolumeListResponse

	for _, vol := range volumes {
		if volumeFilter != "" && vol.Name != volumeFilter {
			continue
		}

		for _, sv := range vol.Subvolumes {
			result = append(result, SubvolumeListResponse{
				Name:          sv.Name,
				Volume:        vol.Name,
				Path:          sv.Path,
				ID:            sv.ID,
				ParentID:      sv.ParentID,
				ReadOnly:      sv.ReadOnly,
				Size:          sv.Size,
				SnapshotCount: len(sv.Snapshots),
			})
		}
	}

	if result == nil {
		result = []SubvolumeListResponse{}
	}

	api.OK(c, result)
}

// getSubvolume 获取子卷详情
// @Summary 获取子卷详情
// @Description 获取指定子卷的详细信息
// @Tags storage
// @Param name path string true "卷名称"
// @Param subvol path string true "子卷名称"
// @Success 200 {object} api.Response{data=SubVolume}
// @Failure 404 {object} api.Response
// @Router /volumes/{name}/subvolumes/{subvol} [get]
func (h *Handlers) getSubvolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	subvol, err := h.manager.GetSubVolume(volumeName, subvolName)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, subvol)
}

// CreateSubvolumeRequest 创建子卷请求
type CreateSubvolumeRequest struct {
	Name string `json:"name" binding:"required"`
	Path string `json:"path"` // 可选：自定义路径
}

// createSubvolume 创建子卷
// @Summary 创建子卷
// @Description 在指定卷中创建新的子卷
// @Tags storage
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body CreateSubvolumeRequest true "创建请求"
// @Success 201 {object} api.Response{data=SubVolume}
// @Failure 400 {object} api.Response
// @Router /volumes/{name}/subvolumes [post]
func (h *Handlers) createSubvolume(c *gin.Context) {
	volumeName := c.Param("name")

	var req CreateSubvolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	subvol, err := h.manager.CreateSubVolume(volumeName, req.Name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, subvol)
}

// deleteSubvolume 删除子卷
// @Summary 删除子卷
// @Description 删除指定子卷
// @Tags storage
// @Param name path string true "卷名称"
// @Param subvol path string true "子卷名称"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/subvolumes/{subvol} [delete]
func (h *Handlers) deleteSubvolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := h.manager.DeleteSubVolume(volumeName, subvolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// MountSubvolumeRequest 挂载子卷请求
type MountSubvolumeRequest struct {
	MountPath string `json:"mountPath" binding:"required"`
}

// mountSubvolume 挂载子卷
// @Summary 挂载子卷
// @Description 将子卷挂载到指定路径
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param subvol path string true "子卷名称"
// @Param request body MountSubvolumeRequest true "挂载请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/subvolumes/{subvol}/mount [post]
func (h *Handlers) mountSubvolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	var req MountSubvolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.MountSubVolume(volumeName, subvolName, req.MountPath); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "子卷已挂载", gin.H{"mountPath": req.MountPath})
}

// SetReadOnlyRequest 设置只读请求
type SetReadOnlyRequest struct {
	ReadOnly bool `json:"readOnly"`
}

// setSubvolumeReadOnly 设置子卷只读属性
// @Summary 设置子卷只读
// @Description 设置子卷的只读属性
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param subvol path string true "子卷名称"
// @Param request body SetReadOnlyRequest true "设置请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/subvolumes/{subvol}/readonly [post]
func (h *Handlers) setSubvolumeReadOnly(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	var req SetReadOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.SetSubVolumeReadOnly(volumeName, subvolName, req.ReadOnly); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "已更新只读属性", nil)
}

// ========== 快照管理 ==========

// SnapshotListResponse 快照列表响应
type SnapshotListResponse struct {
	Name      string `json:"name"`
	Volume    string `json:"volume"`
	Subvolume string `json:"subvolume"`
	Path      string `json:"path"`
	ReadOnly  bool   `json:"readOnly"`
	Size      uint64 `json:"size"`
	CreatedAt string `json:"createdAt"`
	Type      string `json:"type"` // manual, scheduled
}

// listSnapshots 列出卷的快照
// @Summary 列出快照
// @Description 列出指定卷的所有快照
// @Tags storage
// @Param name path string true "卷名称"
// @Param subvol query string false "过滤子卷名称"
// @Success 200 {object} api.Response{data=[]SnapshotListResponse}
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/snapshots [get]
func (h *Handlers) listSnapshots(c *gin.Context) {
	volumeName := c.Param("name")
	subvolFilter := c.Query("subvol")

	snapshots, err := h.manager.ListSnapshots(volumeName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

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

	api.OK(c, result)
}

// listAllSnapshots 列出所有快照（跨卷）
// @Summary 列出所有快照
// @Description 列出系统中所有卷的快照
// @Tags storage
// @Param volume query string false "过滤卷名称"
// @Success 200 {object} api.Response{data=[]SnapshotListResponse}
// @Router /snapshots [get]
func (h *Handlers) listAllSnapshots(c *gin.Context) {
	volumeFilter := c.Query("volume")

	volumes := h.manager.ListVolumes()
	var result []SnapshotListResponse

	for _, vol := range volumes {
		if volumeFilter != "" && vol.Name != volumeFilter {
			continue
		}

		snapshots, err := h.manager.ListSnapshots(vol.Name)
		if err != nil {
			continue
		}

		for _, snap := range snapshots {
			snapType := "scheduled"
			if len(snap.Name) > 6 && snap.Name[:6] == "manual" {
				snapType = "manual"
			}

			result = append(result, SnapshotListResponse{
				Name:      snap.Name,
				Volume:    vol.Name,
				Subvolume: snap.Source,
				Path:      snap.Path,
				ReadOnly:  snap.ReadOnly,
				Size:      snap.Size,
				CreatedAt: snap.CreatedAt.Format("2006-01-02 15:04"),
				Type:      snapType,
			})
		}
	}

	if result == nil {
		result = []SnapshotListResponse{}
	}

	api.OK(c, result)
}

// getSnapshot 获取快照详情
// @Summary 获取快照详情
// @Description 获取指定快照的详细信息
// @Tags storage
// @Param name path string true "卷名称"
// @Param snap path string true "快照名称"
// @Success 200 {object} api.Response{data=Snapshot}
// @Failure 404 {object} api.Response
// @Router /volumes/{name}/snapshots/{snap} [get]
func (h *Handlers) getSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapName := c.Param("snap")

	snap, err := h.manager.GetSnapshot(volumeName, snapName)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, snap)
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Subvolume string `json:"subvolume" binding:"required"`
	Name      string `json:"name"`
	ReadOnly  bool   `json:"readOnly"`
}

// createSnapshot 创建快照
// @Summary 创建快照
// @Description 为指定子卷创建快照
// @Tags storage
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body CreateSnapshotRequest true "创建请求"
// @Success 201 {object} api.Response{data=Snapshot}
// @Failure 400 {object} api.Response
// @Router /volumes/{name}/snapshots [post]
func (h *Handlers) createSnapshot(c *gin.Context) {
	volumeName := c.Param("name")

	var req CreateSnapshotRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	snap, err := h.manager.CreateSnapshot(volumeName, req.Subvolume, req.Name, req.ReadOnly)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, snap)
}

// deleteSnapshot 删除快照
// @Summary 删除快照
// @Description 删除指定快照
// @Tags storage
// @Param name path string true "卷名称"
// @Param snap path string true "快照名称"
// @Success 204 "No Content"
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/snapshots/{snap} [delete]
func (h *Handlers) deleteSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapName := c.Param("snap")

	if err := h.manager.DeleteSnapshot(volumeName, snapName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// RestoreSnapshotRequest 恢复快照请求
type RestoreSnapshotRequest struct {
	TargetName string `json:"targetName"` // 恢复后的名称
}

// restoreSnapshot 恢复快照
// @Summary 恢复快照
// @Description 从快照创建可写副本
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param snap path string true "快照名称"
// @Param request body RestoreSnapshotRequest true "恢复请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/snapshots/{snap}/restore [post]
func (h *Handlers) restoreSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapName := c.Param("snap")

	var req RestoreSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	targetName := req.TargetName
	if targetName == "" {
		targetName = snapName + "-restored"
	}

	if err := h.manager.RestoreSnapshot(volumeName, snapName, targetName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "快照已恢复", gin.H{"targetName": targetName})
}

// RollbackSnapshotRequest 回滚快照请求
type RollbackSnapshotRequest struct {
	Subvolume string `json:"subvolume" binding:"required"`
}

// rollbackSnapshot 回滚快照
// @Summary 回滚快照
// @Description 将子卷回滚到快照状态（危险操作）
// @Tags storage
// @Accept json
// @Param name path string true "卷名称"
// @Param snap path string true "快照名称"
// @Param request body RollbackSnapshotRequest true "回滚请求"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/snapshots/{snap}/rollback [post]
func (h *Handlers) rollbackSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapName := c.Param("snap")

	var req RollbackSnapshotRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.RollbackSnapshot(volumeName, req.Subvolume, snapName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "已回滚到快照", nil)
}

// ========== 设备管理 ==========

// getDeviceStats 获取设备统计
// @Summary 获取设备统计
// @Description 获取卷中各设备的统计信息
// @Tags storage
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response
// @Failure 400,404 {object} api.Response
// @Router /volumes/{name}/devices [get]
func (h *Handlers) getDeviceStats(c *gin.Context) {
	volumeName := c.Param("name")

	stats, err := h.manager.GetDeviceStats(volumeName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// AddDeviceRequest 添加设备请求
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
// @Router /volumes/{name}/devices [post]
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
// @Router /volumes/{name}/devices/{device} [delete]
func (h *Handlers) removeDevice(c *gin.Context) {
	volumeName := c.Param("name")
	device := c.Param("device")

	if err := h.manager.RemoveDevice(volumeName, device); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "设备已移除", nil)
}

// ========== RAID 配置 ==========

// ConvertRAIDRequest RAID 转换请求
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
// @Router /volumes/{name}/convert [post]
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
// @Router /raid-configs [get]
func (h *Handlers) getRAIDConfigs(c *gin.Context) {
	api.OK(c, RAIDConfigs)
}

// ========== Hot Spare (热备盘) 管理 ==========

// listHotSpares 列出热备盘
// @Summary 列出热备盘
// @Description 列出所有或指定卷的热备盘
// @Tags storage
// @Param volume query string false "卷名称过滤"
// @Success 200 {object} api.Response{data=[]HotSpare}
// @Router /hot-spare [get]
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
// @Router /hot-spare/status [get]
func (h *Handlers) getHotSpareStatus(c *gin.Context) {
	status := h.hotSpareManager.GetStatus()
	api.OK(c, status)
}

// AddHotSpareRequest 添加热备盘请求
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
// @Router /hot-spare [post]
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
// @Router /hot-spare/{device} [delete]
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
// @Router /hot-spare/{device} [get]
func (h *Handlers) getHotSpare(c *gin.Context) {
	device := c.Param("device")

	hs, err := h.hotSpareManager.GetHotSpare(device)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, hs)
}

// ActivateHotSpareRequest 激活热备盘请求
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
// @Router /hot-spare/{device}/activate [post]
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
// @Router /hot-spare/{device}/cancel [post]
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
// @Router /hot-spare/{device}/rebuild-status [get]
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
// @Router /hot-spare/rebuilding [get]
func (h *Handlers) listRebuilding(c *gin.Context) {
	result := h.hotSpareManager.ListRebuilding()
	api.OK(c, result)
}

// getHotSpareConfig 获取热备盘配置
// @Summary 获取热备盘配置
// @Description 获取热备盘系统的配置
// @Tags storage
// @Success 200 {object} api.Response{data=HotSpareConfig}
// @Router /hot-spare/config [get]
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
// @Router /hot-spare/config [put]
func (h *Handlers) updateHotSpareConfig(c *gin.Context) {
	var config HotSpareConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.hotSpareManager.SetConfig(config)
	api.OKWithMessage(c, "配置已更新", nil)
}

// ========== Fusion Pool 智能分层存储 ==========

// listFusionPools 列出所有融合池
// @Summary 列出所有融合池
// @Description 获取系统中所有 Fusion Pool 的列表
// @Tags storage
// @Produce json
// @Success 200 {object} api.Response{data=[]FusionPool}
// @Router /fusion-pools [get]
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
// @Router /fusion-pools/{name} [get]
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
// @Router /fusion-pools [post]
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
// @Router /fusion-pools/{name} [delete]
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
// @Router /fusion-pools/{name}/subvolumes [get]
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
// @Router /fusion-pools/{name}/subvolumes [post]
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
// @Router /fusion-pools/{name}/subvolumes/{subvol} [get]
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
// @Router /fusion-pools/{name}/subvolumes/{subvol} [delete]
func (h *Handlers) deleteFusionSubvolume(c *gin.Context) {
	poolName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := h.fusionManager.DeleteSubvolume(poolName, subvolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// addSSDDeviceRequest 添加 SSD 设备请求
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
// @Router /fusion-pools/{name}/ssd-devices [post]
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
// @Router /fusion-pools/{name}/hdd-devices [post]
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
// @Router /fusion-pools/{name}/tiering [post]
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
// @Router /fusion-pools/{name}/optimize [post]
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
// @Router /fusion-pools/{name}/stats [get]
func (h *Handlers) getFusionPoolStats(c *gin.Context) {
	poolName := c.Param("name")

	stats, err := h.fusionManager.GetPoolStats(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// ========== 空间分析 ==========

// AnalyzeSpaceRequest 空间分析请求
type AnalyzeSpaceRequest struct {
	Path               string `json:"path"`               // 分析路径（可选）
	IncludeHidden      bool   `json:"includeHidden"`      // 包含隐藏文件
	LargeFileThreshold uint64 `json:"largeFileThreshold"` // 大文件阈值（字节）
	TopDirCount        int    `json:"topDirCount"`        // 返回前N个目录
	TopFileTypes       int    `json:"topFileTypes"`       // 返回前N个文件类型
	AnalyzeDepth       int    `json:"analyzeDepth"`       // 分析深度
	EnableTrend        bool   `json:"enableTrend"`        // 启用趋势预测
}

// analyzeSpace 执行空间分析
// @Summary 执行空间分析
// @Description 对指定卷执行全面的存储空间分析
// @Tags storage
// @Accept json
// @Param volume path string true "卷名称"
// @Param request body AnalyzeSpaceRequest false "分析选项"
// @Success 200 {object} api.Response{data=AnalyzeResult}
// @Failure 400,404 {object} api.Response
// @Router /space/analyze/{volume} [get]
func (h *Handlers) analyzeSpace(c *gin.Context) {
	volumeName := c.Param("volume")

	// 从查询参数或请求体获取选项
	opts := DefaultAnalyzeOptions

	// 尝试从查询参数解析
	if path := c.Query("path"); path != "" {
		opts.Path = path
	}
	if c.Query("includeHidden") == "true" {
		opts.IncludeHidden = true
	}
	if threshold := c.Query("largeFileThreshold"); threshold != "" {
		_, _ = fmt.Sscanf(threshold, "%d", &opts.LargeFileThreshold)
	}
	if topDir := c.Query("topDirCount"); topDir != "" {
		_, _ = fmt.Sscanf(topDir, "%d", &opts.TopDirCount)
	}
	if topTypes := c.Query("topFileTypes"); topTypes != "" {
		_, _ = fmt.Sscanf(topTypes, "%d", &opts.TopFileTypes)
	}
	if depth := c.Query("analyzeDepth"); depth != "" {
		_, _ = fmt.Sscanf(depth, "%d", &opts.AnalyzeDepth)
	}
	if c.Query("enableTrend") == "false" {
		opts.EnableTrend = false
	}

	result, err := h.spaceAnalyzer.Analyze(volumeName, opts)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// getSpaceHistory 获取空间使用历史
// @Summary 获取空间使用历史
// @Description 获取指定卷的空间使用历史记录
// @Tags storage
// @Param volume path string true "卷名称"
// @Param days query int false "查询天数" default(30)
// @Success 200 {object} api.Response{data=[]SpaceRecord}
// @Failure 400,404 {object} api.Response
// @Router /space/history/{volume} [get]
func (h *Handlers) getSpaceHistory(c *gin.Context) {
	volumeName := c.Param("volume")

	days := 30
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	records, err := h.spaceAnalyzer.GetHistory(volumeName, days)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, records)
}

// getSpaceTrend 获取空间趋势预测
// @Summary 获取空间趋势预测
// @Description 获取指定卷的空间使用趋势和预测
// @Tags storage
// @Param volume path string true "卷名称"
// @Success 200 {object} api.Response{data=SpaceTrend}
// @Failure 400,404 {object} api.Response
// @Router /space/trend/{volume} [get]
func (h *Handlers) getSpaceTrend(c *gin.Context) {
	volumeName := c.Param("volume")

	// 获取卷信息
	vol := h.manager.GetVolume(volumeName)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+volumeName)
		return
	}

	trend := h.spaceAnalyzer.predictTrend(volumeName, vol)
	api.OK(c, trend)
}

// ========== 智能 RAID (SmartRAID) ==========

// listSmartPools 列出所有智能池
// @Summary 列出所有智能池
// @Description 获取所有智能存储池列表
// @Tags storage
// @Success 200 {object} api.Response{data=[]SmartPool}
// @Router /smart-pools [get]
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
// @Router /smart-pools/{name} [get]
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

// createSmartPoolRequest 创建智能池请求
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
// @Router /smart-pools [post]
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
// @Router /smart-pools/{name} [delete]
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

// addDeviceToSmartPoolRequest 添加设备请求
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
// @Router /smart-pools/{name}/devices [post]
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

// replaceSmartPoolDeviceRequest 替换设备请求
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
// @Router /smart-pools/{name}/replace [post]
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
// @Router /smart-pools/{name}/stats [get]
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
// @Router /smart-pools/{name}/expansion-plan [get]
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
// @Router /smart-pools/{name}/subvolumes [get]
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

// createSmartSubvolumeRequest 创建子卷请求
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
// @Router /smart-pools/{name}/subvolumes [post]
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
// @Router /smart-pools/{name}/subvolumes/{subvol} [delete]
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
