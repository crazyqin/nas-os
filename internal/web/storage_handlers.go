// Package web 存储管理 API handlers
package web

import (
	"net/http"

	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

// StorageHandlers 存储管理 API handlers.
type StorageHandlers struct {
	storageMgr *storage.Manager
}

// NewStorageHandlers 创建存储管理 handlers.
func NewStorageHandlers(storageMgr *storage.Manager) *StorageHandlers {
	return &StorageHandlers{
		storageMgr: storageMgr,
	}
}

// RegisterRoutes 注册路由到 /api/storage 组.
func (h *StorageHandlers) RegisterRoutes(rg *gin.RouterGroup) {
	storage := rg.Group("/storage")
	{
		// 存储卷管理
		storage.GET("/volumes", h.ListVolumes)
		storage.POST("/volumes", h.CreateVolume)

		// 存储池管理
		storage.GET("/pools", h.ListPools)

		// 快照管理
		storage.GET("/snapshots", h.ListAllSnapshots)
	}
}

// ========== 存储卷 API ==========

// ListVolumes 列出所有存储卷
// @Summary 列出所有存储卷
// @Description 获取系统中所有 Btrfs 存储卷的列表
// @Tags storage
// @Accept json
// @Produce json
// @Success 200 {object} StorageResponse "成功"
// @Router /api/storage/volumes [get].
func (h *StorageHandlers) ListVolumes(c *gin.Context) {
	if h.storageMgr == nil {
		c.JSON(http.StatusOK, []VolumeResponse{})
		return
	}
	volumes := h.storageMgr.ListVolumes()

	// 转换为 API 响应格式
	result := make([]VolumeResponse, 0, len(volumes))
	for _, v := range volumes {
		result = append(result, VolumeResponse{
			Name:        v.Name,
			UUID:        v.UUID,
			Devices:     v.Devices,
			Size:        v.Size,
			Used:        v.Used,
			Free:        v.Free,
			DataProfile: v.DataProfile,
			MetaProfile: v.MetaProfile,
			MountPoint:  v.MountPoint,
			Status: VolumeStatusResponse{
				BalanceRunning:  v.Status.BalanceRunning,
				BalanceProgress: v.Status.BalanceProgress,
				ScrubRunning:    v.Status.ScrubRunning,
				ScrubProgress:   v.Status.ScrubProgress,
				ScrubErrors:     v.Status.ScrubErrors,
				Healthy:         v.Status.Healthy,
			},
			CreatedAt:  v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Subvolumes: convertSubvolumes(v.Subvolumes),
		})
	}

	c.JSON(http.StatusOK, StorageResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// CreateVolumeRequest 创建卷请求.
type CreateVolumeRequest struct {
	Name    string   `json:"name" binding:"required" example:"data-vol"`
	Devices []string `json:"devices" binding:"required" example:"/dev/sda,/dev/sdb"`
	Profile string   `json:"profile" example:"raid1"` // single, raid0, raid1, raid10, raid5, raid6
}

// CreateVolume 创建新卷
// @Summary 创建新卷
// @Description 使用指定设备和 RAID 配置创建新的 Btrfs 存储卷
// @Tags storage
// @Accept json
// @Produce json
// @Param request body CreateVolumeRequest true "卷创建参数"
// @Success 200 {object} StorageResponse "创建成功"
// @Failure 400 {object} StorageResponse "请求参数错误"
// @Failure 500 {object} StorageResponse "服务器内部错误"
// @Router /api/storage/volumes [post].
func (h *StorageHandlers) CreateVolume(c *gin.Context) {
	if h.storageMgr == nil {
		c.JSON(http.StatusInternalServerError, StorageResponse{
			Code:    500,
			Message: "storage manager not initialized",
		})
		return
	}
	var req CreateVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StorageResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	vol, err := h.storageMgr.CreateVolume(req.Name, req.Devices, req.Profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StorageResponse{
			Code:    500,
			Message: "创建卷失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StorageResponse{
		Code:    0,
		Message: "success",
		Data: VolumeResponse{
			Name:        vol.Name,
			UUID:        vol.UUID,
			Devices:     vol.Devices,
			Size:        vol.Size,
			Used:        vol.Used,
			Free:        vol.Free,
			DataProfile: vol.DataProfile,
			MetaProfile: vol.MetaProfile,
			MountPoint:  vol.MountPoint,
			Status: VolumeStatusResponse{
				Healthy: vol.Status.Healthy,
			},
			CreatedAt: vol.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// ========== 存储池 API ==========

// PoolResponse 存储池响应.
type PoolResponse struct {
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Devices     []string `json:"devices"`
	Size        uint64   `json:"size"`        // 总大小（字节）
	Used        uint64   `json:"used"`        // 已使用（字节）
	Free        uint64   `json:"free"`        // 可用空间（字节）
	DataProfile string   `json:"dataProfile"` // 数据配置
	MetaProfile string   `json:"metaProfile"` // 元数据配置
	MountPoint  string   `json:"mountPoint"`  // 挂载点
	Healthy     bool     `json:"healthy"`     // 健康状态
	VolumeCount int      `json:"volumeCount"` // 子卷数量
}

// ListPools 列出存储池
// @Summary 列出存储池
// @Description 获取所有存储池（Btrfs 卷）的列表
// @Tags storage
// @Accept json
// @Produce json
// @Success 200 {object} StorageResponse "成功"
// @Router /api/storage/pools [get].
func (h *StorageHandlers) ListPools(c *gin.Context) {
	if h.storageMgr == nil {
		c.JSON(http.StatusOK, []PoolResponse{})
		return
	}
	volumes := h.storageMgr.ListVolumes()

	// 将卷转换为存储池格式
	pools := make([]PoolResponse, 0, len(volumes))
	for _, v := range volumes {
		pools = append(pools, PoolResponse{
			Name:        v.Name,
			UUID:        v.UUID,
			Devices:     v.Devices,
			Size:        v.Size,
			Used:        v.Used,
			Free:        v.Free,
			DataProfile: v.DataProfile,
			MetaProfile: v.MetaProfile,
			MountPoint:  v.MountPoint,
			Healthy:     v.Status.Healthy,
			VolumeCount: len(v.Subvolumes),
		})
	}

	c.JSON(http.StatusOK, StorageResponse{
		Code:    0,
		Message: "success",
		Data:    pools,
	})
}

// ========== 快照 API ==========

// SnapshotResponse 快照响应.
type SnapshotResponse struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Source     string `json:"source"`     // 源子卷名
	SourceUUID string `json:"sourceUuid"` // 源子卷 UUID
	ReadOnly   bool   `json:"readOnly"`
	CreatedAt  string `json:"createdAt"`  // ISO 8601 格式
	Size       uint64 `json:"size"`       // 快照大小（估算）
	VolumeName string `json:"volumeName"` // 所属卷名
}

// ListAllSnapshots 列出所有快照
// @Summary 列出所有快照
// @Description 获取所有存储卷中的快照列表
// @Tags storage
// @Accept json
// @Produce json
// @Param volume query string false "卷名称过滤"
// @Success 200 {object} StorageResponse "成功"
// @Router /api/storage/snapshots [get].
func (h *StorageHandlers) ListAllSnapshots(c *gin.Context) {
	if h.storageMgr == nil {
		c.JSON(http.StatusOK, StorageResponse{
			Code:    0,
			Message: "success",
			Data:    []SnapshotResponse{},
		})
		return
	}
	volumeFilter := c.Query("volume")

	volumes := h.storageMgr.ListVolumes()
	snapshots := make([]SnapshotResponse, 0)

	for _, v := range volumes {
		// 如果指定了卷名过滤，跳过其他卷
		if volumeFilter != "" && v.Name != volumeFilter {
			continue
		}

		// 获取该卷的快照
		snaps, err := h.storageMgr.ListSnapshots(v.Name)
		if err != nil {
			continue
		}

		for _, snap := range snaps {
			snapshots = append(snapshots, SnapshotResponse{
				Name:       snap.Name,
				Path:       snap.Path,
				Source:     snap.Source,
				SourceUUID: snap.SourceUUID,
				ReadOnly:   snap.ReadOnly,
				CreatedAt:  snap.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				Size:       snap.Size,
				VolumeName: v.Name,
			})
		}
	}

	c.JSON(http.StatusOK, StorageResponse{
		Code:    0,
		Message: "success",
		Data:    snapshots,
	})
}

// ========== 辅助类型 ==========

// StorageResponse 存储 API 通用响应.
type StorageResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// VolumeResponse 卷响应.
type VolumeResponse struct {
	Name        string               `json:"name"`
	UUID        string               `json:"uuid"`
	Devices     []string             `json:"devices"`
	Size        uint64               `json:"size"`        // 总大小（字节）
	Used        uint64               `json:"used"`        // 已使用（字节）
	Free        uint64               `json:"free"`        // 可用空间（字节）
	DataProfile string               `json:"dataProfile"` // 数据配置
	MetaProfile string               `json:"metaProfile"` // 元数据配置
	MountPoint  string               `json:"mountPoint"`  // 挂载点
	Status      VolumeStatusResponse `json:"status"`
	CreatedAt   string               `json:"createdAt"` // ISO 8601 格式
	Subvolumes  []SubvolumeResponse  `json:"subvolumes,omitempty"`
}

// VolumeStatusResponse 卷状态响应.
type VolumeStatusResponse struct {
	BalanceRunning  bool    `json:"balanceRunning"`
	BalanceProgress float64 `json:"balanceProgress"`
	ScrubRunning    bool    `json:"scrubRunning"`
	ScrubProgress   float64 `json:"scrubProgress"`
	ScrubErrors     uint64  `json:"scrubErrors"`
	Healthy         bool    `json:"healthy"`
}

// SubvolumeResponse 子卷响应.
type SubvolumeResponse struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	ParentID  uint64 `json:"parentId"`
	ReadOnly  bool   `json:"readOnly"`
	UUID      string `json:"uuid"`
	Size      uint64 `json:"size"` // 估算大小
	SnapCount int    `json:"snapCount"`
}

// convertSubvolumes 转换子卷列表为响应格式.
func convertSubvolumes(subvolumes []*storage.SubVolume) []SubvolumeResponse {
	result := make([]SubvolumeResponse, 0, len(subvolumes))
	for _, sv := range subvolumes {
		result = append(result, SubvolumeResponse{
			ID:        sv.ID,
			Name:      sv.Name,
			Path:      sv.Path,
			ParentID:  sv.ParentID,
			ReadOnly:  sv.ReadOnly,
			UUID:      sv.UUID,
			Size:      sv.Size,
			SnapCount: len(sv.Snapshots),
		})
	}
	return result
}
