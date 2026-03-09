package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nas-os/internal/nfs"
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
}

// NewServer 创建 Web 服务器
func NewServer(storMgr *storage.Manager, userMgr *users.Manager, smbMgr *smb.Manager, nfsMgr *nfs.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(loggerMiddleware())

	s := &Server{
		engine:     engine,
		storageMgr: storMgr,
		userMgr:    userMgr,
		smbMgr:     smbMgr,
		nfsMgr:     nfsMgr,
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

func (s *Server) setupRoutes() {
	// API 路由
	api := s.engine.Group("/api/v1")
	{
		// 存储管理
		api.GET("/volumes", s.listVolumes)
		api.POST("/volumes", s.createVolume)
		api.GET("/volumes/:name", s.getVolume)
		api.POST("/volumes/:name/subvolumes", s.createSubVolume)
		api.POST("/volumes/:name/snapshots", s.createSnapshot)
		api.POST("/volumes/:name/balance", s.balanceVolume)
		api.POST("/volumes/:name/scrub", s.scrubVolume)

		// 用户管理
		users.NewHandlers(s.userMgr).RegisterRoutes(api)

		// SMB 共享
		smb.NewHandlers(s.smbMgr).RegisterRoutes(api)

		// NFS 共享
		nfs.NewHandlers(s.nfsMgr).RegisterRoutes(api)

		// 系统信息
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

// API Handlers

func (s *Server) listVolumes(c *gin.Context) {
	volumes := s.storageMgr.GetVolumes()
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

func (s *Server) balanceVolume(c *gin.Context) {
	volumeName := c.Param("name")
	if err := s.storageMgr.Balance(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "平衡已启动"})
}

func (s *Server) scrubVolume(c *gin.Context) {
	volumeName := c.Param("name")
	if err := s.storageMgr.Scrub(volumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "校验已启动"})
}

func (s *Server) getSystemInfo(c *gin.Context) {
	// TODO: 获取系统信息
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

// 通用响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

var _ = json.Marshal // 避免未使用导入
