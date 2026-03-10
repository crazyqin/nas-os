package web

import (
	"context"
	"log"
	"net/http"
	"time"

	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/shares"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"

	"github.com/gin-gonic/gin"
)

// Server Web 服务器
type Server struct {
	engine     *gin.Engine
	httpSrv    *http.Server
	storageMgr *storage.Manager
	userMgr    *users.Manager
	smbMgr     *smb.Manager
	nfsMgr     *nfs.Manager
	networkMgr *network.Manager
}

// NewServer 创建 Web 服务器
func NewServer(storMgr *storage.Manager, userMgr *users.Manager, smbMgr *smb.Manager, nfsMgr *nfs.Manager, netMgr *network.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(loggerMiddleware())
	engine.Use(corsMiddleware())

	s := &Server{
		engine:     engine,
		storageMgr: storMgr,
		userMgr:    userMgr,
		smbMgr:     smbMgr,
		nfsMgr:     nfsMgr,
		networkMgr: netMgr,
	}
	s.setupRoutes()
	return s
}

func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[%d] %s %s (%v)", c.Writer.Status(), c.Request.Method, c.Request.URL.Path, time.Since(start))
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func (s *Server) setupRoutes() {
	// API 路由
	api := s.engine.Group("/api/v1")
	{
		// ========== 卷管理 ==========
		api.GET("/volumes", s.listVolumes)
		api.POST("/volumes", s.createVolume)
		api.GET("/volumes/:name", s.getVolume)
		api.DELETE("/volumes/:name", s.deleteVolume)
		api.POST("/volumes/:name/mount", s.mountVolume)
		api.POST("/volumes/:name/unmount", s.unmountVolume)
		api.GET("/volumes/:name/usage", s.getVolumeUsage)
		api.POST("/volumes/:name/devices", s.addDevice)
		api.DELETE("/volumes/:name/devices/:device", s.removeDevice)
		api.GET("/volumes/:name/devices", s.getDeviceStats)

		// ========== 子卷管理 ==========
		api.GET("/volumes/:name/subvolumes", s.listSubVolumes)
		api.POST("/volumes/:name/subvolumes", s.createSubVolume)
		api.GET("/volumes/:name/subvolumes/:subvol", s.getSubVolume)
		api.DELETE("/volumes/:name/subvolumes/:subvol", s.deleteSubVolume)
		api.PUT("/volumes/:name/subvolumes/:subvol/readonly", s.setSubVolumeReadOnly)

		// ========== 快照管理 ==========
		api.GET("/volumes/:name/snapshots", s.listSnapshots)
		api.POST("/volumes/:name/snapshots", s.createSnapshot)
		api.DELETE("/volumes/:name/snapshots/:snapshot", s.deleteSnapshot)
		api.POST("/volumes/:name/snapshots/:snapshot/restore", s.restoreSnapshot)

		// ========== RAID 配置 ==========
		api.GET("/raid-configs", s.getRAIDConfigs)
		api.POST("/volumes/:name/convert", s.convertRAID)

		// ========== 维护操作 ==========
		api.POST("/volumes/:name/balance", s.startBalance)
		api.GET("/volumes/:name/balance", s.getBalanceStatus)
		api.POST("/volumes/:name/scrub", s.startScrub)
		api.GET("/volumes/:name/scrub", s.getScrubStatus)

		// ========== 用户管理 ==========
		users.NewHandlers(s.userMgr).RegisterRoutes(api)

		// ========== 共享管理（SMB + NFS）==========
		shares.NewHandlers(s.smbMgr, s.nfsMgr).RegisterRoutes(api)

		// ========== 网络管理 ==========
		network.NewHandlers(s.networkMgr).RegisterRoutes(api)

		// ========== 系统信息 ==========
		api.GET("/system/info", s.getSystemInfo)
		api.GET("/system/health", s.getHealth)
	}

	// 静态文件（前端）
	s.engine.Static("/", "./webui/dist")
}

// Start 启动服务器
func (s *Server) Start(addr string) error {
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	return s.httpSrv.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

// ========== 卷管理 API ==========

func (s *Server) listVolumes(c *gin.Context) {
	volumes := s.storageMgr.ListVolumes()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    volumes,
	})
}

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

func (s *Server) getVolume(c *gin.Context) {
	name := c.Param("name")
	vol := s.storageMgr.GetVolume(name)
	if vol == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "卷不存在"})
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

func (s *Server) mountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := s.storageMgr.MountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "挂载成功"})
}

func (s *Server) unmountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := s.storageMgr.UnmountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "卸载成功"})
}

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

func (s *Server) getRAIDConfigs(c *gin.Context) {
	configs := s.storageMgr.GetRAIDConfigs()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

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

func (s *Server) startScrub(c *gin.Context) {
	volumeName := c.Param("name")
	if err := s.storageMgr.Scrub(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "校验已启动"})
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

// ========== 系统信息 API ==========

func (s *Server) getSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"hostname": "nas-os",
			"version":  "0.1.0",
		},
	})
}

func (s *Server) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "healthy",
	})
}