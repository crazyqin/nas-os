package storage

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type LegacyAPIHandlers struct {
	storageMgr *Manager
}

func NewLegacyAPIHandlers(storageMgr *Manager) *LegacyAPIHandlers {
	return &LegacyAPIHandlers{storageMgr: storageMgr}
}

// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags volumes
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /volumes [get].
func (h *LegacyAPIHandlers) listVolumes(c *gin.Context) {
	volumes := h.storageMgr.ListVolumes()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    volumes,
	})
}

// getVolume 获取卷详情
// @Summary 获取卷详情
// @Description 根据卷名称获取卷的详细信息
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 404 {object} GenericResponse "卷不存在"
// @Router /volumes/{name} [get].
func (h *LegacyAPIHandlers) getVolume(c *gin.Context) {
	name := c.Param("name")
	vol := h.storageMgr.GetVolume(name)
	if vol == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "卷不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": vol})
}

// getVolumeUsage 获取卷使用量
// @Summary 获取卷使用量
// @Description 获取指定卷的存储使用情况
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/usage [get].
func (h *LegacyAPIHandlers) getVolumeUsage(c *gin.Context) {
	name := c.Param("name")
	total, used, free, err := h.storageMgr.GetUsage(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": total,
			"used":  used,
			"free":  free,
		},
	})
}

// listSubVolumes 列出子卷
// @Summary 列出子卷
// @Description 获取指定卷的所有子卷列表
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/subvolumes [get].
func (h *LegacyAPIHandlers) listSubVolumes(c *gin.Context) {
	volumeName := c.Param("name")
	subvols, err := h.storageMgr.ListSubVolumes(volumeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    subvols,
	})
}

func (h *LegacyAPIHandlers) getSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	subvol, err := h.storageMgr.GetSubVolume(volumeName, subvolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": subvol})
}

// listSnapshots 列出快照
// @Summary 列出快照
// @Description 获取指定卷的所有快照列表
// @Tags snapshots
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/snapshots [get].
func (h *LegacyAPIHandlers) listSnapshots(c *gin.Context) {
	volumeName := c.Param("name")
	snapshots, err := h.storageMgr.ListSnapshots(volumeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    snapshots,
	})
}

func (h *LegacyAPIHandlers) getRAIDConfigs(c *gin.Context) {
	configs := h.storageMgr.GetRAIDConfigs()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

func (h *LegacyAPIHandlers) getBalanceStatus(c *gin.Context) {
	volumeName := c.Param("name")
	status, err := h.storageMgr.GetBalanceStatus(volumeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

func (h *LegacyAPIHandlers) getScrubStatus(c *gin.Context) {
	volumeName := c.Param("name")
	status, err := h.storageMgr.GetScrubStatus(volumeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// createVolume 创建卷
// @Summary 创建新卷
// @Description 使用指定设备和配置创建新的 Btrfs 卷
// @Tags volumes
// @Accept json
// @Produce json
// @Param request body VolumeCreateRequest true "卷创建参数"
// @Success 200 {object} GenericResponse "创建成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes [post].
func (h *LegacyAPIHandlers) createVolume(c *gin.Context) {
	var req struct {
		Name    string   `json:"name" binding:"required"`
		Devices []string `json:"devices" binding:"required"`
		Profile string   `json:"profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	vol, err := h.storageMgr.CreateVolume(req.Name, req.Devices, req.Profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": vol})
}

func (h *LegacyAPIHandlers) deleteVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.storageMgr.DeleteVolume(name, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "卷已删除"})
}

// mountVolume 挂载卷
// @Summary 挂载卷
// @Description 挂载指定的 Btrfs 卷
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "挂载成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/mount [post].

func (h *LegacyAPIHandlers) mountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := h.storageMgr.MountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "挂载成功"})
}

// unmountVolume 卸载卷
// @Summary 卸载卷
// @Description 卸载指定的 Btrfs 卷
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "卸载成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/unmount [post].

func (h *LegacyAPIHandlers) unmountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := h.storageMgr.UnmountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "卸载成功"})
}

func (h *LegacyAPIHandlers) addDevice(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.storageMgr.AddDevice(volumeName, req.Device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备已添加"})
}

// removeDevice 从卷移除设备
// @Summary 从卷移除设备
// @Description 从指定卷移除存储设备
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param device path string true "设备路径"
// @Success 200 {object} GenericResponse "移除成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/devices/{device} [delete].

func (h *LegacyAPIHandlers) removeDevice(c *gin.Context) {
	volumeName := c.Param("name")
	device := c.Param("device")

	if err := h.storageMgr.RemoveDevice(volumeName, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备已移除"})
}

func (h *LegacyAPIHandlers) getDeviceStats(c *gin.Context) {
	name := c.Param("name")
	stats, err := h.storageMgr.GetDeviceStats(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== 子卷管理 API ==========

func (h *LegacyAPIHandlers) createSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	subvol, err := h.storageMgr.CreateSubVolume(volumeName, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": subvol})
}

func (h *LegacyAPIHandlers) deleteSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := h.storageMgr.DeleteSubVolume(volumeName, subvolName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "子卷已删除"})
}

func (h *LegacyAPIHandlers) setSubVolumeReadOnly(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	var req struct {
		ReadOnly bool `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.storageMgr.SetSubVolumeReadOnly(volumeName, subvolName, req.ReadOnly); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "属性已更新"})
}

// ========== 快照管理 API ==========

func (h *LegacyAPIHandlers) createSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		SubVolumeName string `json:"subvolume" binding:"required"`
		Name          string `json:"name" binding:"required"`
		ReadOnly      bool   `json:"readonly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	snap, err := h.storageMgr.CreateSnapshot(volumeName, req.SubVolumeName, req.Name, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": snap})
}

func (h *LegacyAPIHandlers) deleteSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapshotName := c.Param("snapshot")

	if err := h.storageMgr.DeleteSnapshot(volumeName, snapshotName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "快照已删除"})
}

func (h *LegacyAPIHandlers) restoreSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapshotName := c.Param("snapshot")

	var req struct {
		TargetName string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.storageMgr.RestoreSnapshot(volumeName, snapshotName, req.TargetName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "快照已恢复"})
}

// ========== RAID 配置 API ==========

func (h *LegacyAPIHandlers) convertRAID(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		DataProfile string `json:"dataProfile"`
		MetaProfile string `json:"metaProfile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.storageMgr.ConvertRAID(volumeName, req.DataProfile, req.MetaProfile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "RAID 配置转换已启动"})
}

// ========== 维护操作 API ==========

func (h *LegacyAPIHandlers) startBalance(c *gin.Context) {
	volumeName := c.Param("name")
	if err := h.storageMgr.Balance(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "平衡已启动"})
}

func (h *LegacyAPIHandlers) startScrub(c *gin.Context) {
	volumeName := c.Param("name")
	if err := h.storageMgr.Scrub(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "校验已启动"})
}

// RegisterLegacyRoutes 注册历史存储 API 路由。
func RegisterLegacyRoutes(api *gin.RouterGroup, h *LegacyAPIHandlers) {
	api.GET("/volumes", h.listVolumes)
	api.POST("/volumes", h.createVolume)
	api.GET("/volumes/:name", h.getVolume)
	api.DELETE("/volumes/:name", h.deleteVolume)
	api.POST("/volumes/:name/mount", h.mountVolume)
	api.POST("/volumes/:name/unmount", h.unmountVolume)
	api.GET("/volumes/:name/usage", h.getVolumeUsage)
	api.POST("/volumes/:name/devices", h.addDevice)
	api.DELETE("/volumes/:name/devices/:device", h.removeDevice)
	api.GET("/volumes/:name/devices", h.getDeviceStats)
	api.GET("/volumes/:name/subvolumes", h.listSubVolumes)
	api.POST("/volumes/:name/subvolumes", h.createSubVolume)
	api.GET("/volumes/:name/subvolumes/:subvol", h.getSubVolume)
	api.DELETE("/volumes/:name/subvolumes/:subvol", h.deleteSubVolume)
	api.PUT("/volumes/:name/subvolumes/:subvol/readonly", h.setSubVolumeReadOnly)
	api.GET("/volumes/:name/snapshots", h.listSnapshots)
	api.POST("/volumes/:name/snapshots", h.createSnapshot)
	api.DELETE("/volumes/:name/snapshots/:snapshot", h.deleteSnapshot)
	api.POST("/volumes/:name/snapshots/:snapshot/restore", h.restoreSnapshot)
	api.GET("/raid-configs", h.getRAIDConfigs)
	api.POST("/volumes/:name/convert", h.convertRAID)
	api.POST("/volumes/:name/balance", h.startBalance)
	api.GET("/volumes/:name/balance", h.getBalanceStatus)
	api.POST("/volumes/:name/scrub", h.startScrub)
	api.GET("/volumes/:name/scrub", h.getScrubStatus)
}
