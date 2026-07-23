// Package storage 提供存储管理 API 处理器
package storage

import (

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 子卷管理 ==========

// SubvolumeListResponse 子卷列表响应.
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
// @Router /volumes/{name}/subvolumes [get].
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
// @Router /subvolumes [get].
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
// @Router /volumes/{name}/subvolumes/{subvol} [get].
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

// CreateSubvolumeRequest 创建子卷请求.
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
// @Router /volumes/{name}/subvolumes [post].
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
// @Router /volumes/{name}/subvolumes/{subvol} [delete].
func (h *Handlers) deleteSubvolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := h.manager.DeleteSubVolume(volumeName, subvolName); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.NoContent(c)
}

// MountSubvolumeRequest 挂载子卷请求.
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
// @Router /volumes/{name}/subvolumes/{subvol}/mount [post].
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

// SetReadOnlyRequest 设置只读请求.
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
// @Router /volumes/{name}/subvolumes/{subvol}/readonly [post].
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

