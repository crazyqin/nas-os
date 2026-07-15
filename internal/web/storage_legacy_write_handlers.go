package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
func (s *Server) createVolume(c *gin.Context) {
	var req struct {
		Name    string   `json:"name" binding:"required"`
		Devices []string `json:"devices" binding:"required"`
		Profile string   `json:"profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	vol, err := s.storageMgr.CreateVolume(req.Name, req.Devices, req.Profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": vol})
}

func (s *Server) deleteVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := s.storageMgr.DeleteVolume(name, force); err != nil {
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

func (s *Server) mountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := s.storageMgr.MountVolume(name); err != nil {
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

func (s *Server) unmountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := s.storageMgr.UnmountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "卸载成功"})
}

func (s *Server) addDevice(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := s.storageMgr.AddDevice(volumeName, req.Device); err != nil {
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

func (s *Server) removeDevice(c *gin.Context) {
	volumeName := c.Param("name")
	device := c.Param("device")

	if err := s.storageMgr.RemoveDevice(volumeName, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备已移除"})
}

func (s *Server) getDeviceStats(c *gin.Context) {
	name := c.Param("name")
	stats, err := s.storageMgr.GetDeviceStats(name)
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

func (s *Server) createSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	subvol, err := s.storageMgr.CreateSubVolume(volumeName, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": subvol})
}

func (s *Server) deleteSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	if err := s.storageMgr.DeleteSubVolume(volumeName, subvolName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "子卷已删除"})
}

func (s *Server) setSubVolumeReadOnly(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	var req struct {
		ReadOnly bool `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := s.storageMgr.SetSubVolumeReadOnly(volumeName, subvolName, req.ReadOnly); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "属性已更新"})
}

// ========== 快照管理 API ==========

func (s *Server) createSnapshot(c *gin.Context) {
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

	snap, err := s.storageMgr.CreateSnapshot(volumeName, req.SubVolumeName, req.Name, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": snap})
}

func (s *Server) deleteSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapshotName := c.Param("snapshot")

	if err := s.storageMgr.DeleteSnapshot(volumeName, snapshotName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "快照已删除"})
}

func (s *Server) restoreSnapshot(c *gin.Context) {
	volumeName := c.Param("name")
	snapshotName := c.Param("snapshot")

	var req struct {
		TargetName string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := s.storageMgr.RestoreSnapshot(volumeName, snapshotName, req.TargetName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "快照已恢复"})
}

// ========== RAID 配置 API ==========

func (s *Server) convertRAID(c *gin.Context) {
	volumeName := c.Param("name")
	var req struct {
		DataProfile string `json:"dataProfile"`
		MetaProfile string `json:"metaProfile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := s.storageMgr.ConvertRAID(volumeName, req.DataProfile, req.MetaProfile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "RAID 配置转换已启动"})
}

// ========== 维护操作 API ==========

func (s *Server) startBalance(c *gin.Context) {
	volumeName := c.Param("name")
	if err := s.storageMgr.Balance(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "平衡已启动"})
}

func (s *Server) startScrub(c *gin.Context) {
	volumeName := c.Param("name")
	if err := s.storageMgr.Scrub(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "校验已启动"})
}
