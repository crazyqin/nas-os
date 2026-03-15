// Package storage 提供存储管理 API 处理器
package storage

import (
	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// Handlers 存储 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
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