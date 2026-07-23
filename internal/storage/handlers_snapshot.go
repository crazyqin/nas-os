// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 快照管理 ==========

// SnapshotListResponse 快照列表响应.
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
// @Router /volumes/{name}/snapshots [get].
func (h *Handlers) listSnapshots(c *gin.Context) {
	volumeName := c.Param("name")
	subvolFilter := c.Query("subvol")

	snapshots, err := h.manager.ListSnapshots(volumeName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, buildSnapshotListResponses(volumeName, snapshots, subvolFilter))
}

// listAllSnapshots 列出所有快照（跨卷）
// @Summary 列出所有快照
// @Description 列出系统中所有卷的快照
// @Tags storage
// @Param volume query string false "过滤卷名称"
// @Success 200 {object} api.Response{data=[]SnapshotListResponse}
// @Router /snapshots [get].
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
// @Router /volumes/{name}/snapshots/{snap} [get].
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

// CreateSnapshotRequest 创建快照请求.
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
// @Router /volumes/{name}/snapshots [post].
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
// @Router /volumes/{name}/snapshots/{snap} [delete].
func (h *Handlers) deleteSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapName := c.Param("snap")

	if err := h.manager.DeleteSnapshot(volumeName, snapName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// RestoreSnapshotRequest 恢复快照请求.
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
// @Router /volumes/{name}/snapshots/{snap}/restore [post].
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

// RollbackSnapshotRequest 回滚快照请求.
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
// @Router /volumes/{name}/snapshots/{snap}/rollback [post].
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

