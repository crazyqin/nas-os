package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags volumes
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /volumes [get].
func (s *Server) listVolumes(c *gin.Context) {
	volumes := s.storageMgr.ListVolumes()
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
func (s *Server) getVolume(c *gin.Context) {
	name := c.Param("name")
	vol := s.storageMgr.GetVolume(name)
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
func (s *Server) getVolumeUsage(c *gin.Context) {
	name := c.Param("name")
	total, used, free, err := s.storageMgr.GetUsage(name)
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
func (s *Server) listSubVolumes(c *gin.Context) {
	volumeName := c.Param("name")
	subvols, err := s.storageMgr.ListSubVolumes(volumeName)
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

func (s *Server) getSubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	subvolName := c.Param("subvol")

	subvol, err := s.storageMgr.GetSubVolume(volumeName, subvolName)
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
func (s *Server) listSnapshots(c *gin.Context) {
	volumeName := c.Param("name")
	snapshots, err := s.storageMgr.ListSnapshots(volumeName)
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

func (s *Server) getRAIDConfigs(c *gin.Context) {
	configs := s.storageMgr.GetRAIDConfigs()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

func (s *Server) getBalanceStatus(c *gin.Context) {
	volumeName := c.Param("name")
	status, err := s.storageMgr.GetBalanceStatus(volumeName)
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

func (s *Server) getScrubStatus(c *gin.Context) {
	volumeName := c.Param("name")
	status, err := s.storageMgr.GetScrubStatus(volumeName)
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
