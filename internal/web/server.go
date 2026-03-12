package web

import (
	"context"
	"log"
	"net/http"
	"time"

	"nas-os/internal/auth"
	"nas-os/internal/backup"
	"nas-os/internal/docker"
	"nas-os/internal/downloader"
	"nas-os/internal/files"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/notify"
	"nas-os/internal/perf"
	"nas-os/internal/photos"
	"nas-os/internal/plugin"
	"nas-os/internal/quota"
	"nas-os/internal/shares"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/system"
	"nas-os/internal/users"

	_ "nas-os/docs/swagger" // Swagger 文档

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server Web 服务器
type Server struct {
	engine        *gin.Engine
	httpSrv       *http.Server
	storageMgr    *storage.Manager
	userMgr       *users.Manager
	mfaMgr        *auth.MFAManager
	smbMgr        *smb.Manager
	nfsMgr        *nfs.Manager
	networkMgr    *network.Manager
	dockerMgr     *docker.Manager
	appStore      *docker.AppStore
	perfMgr       *perf.Manager
	pluginMgr     *plugin.Manager
	pluginMarket  *plugin.Market
	quotaMgr      *quota.Manager
	filesMgr      *files.Manager
	notifyMgr     *notify.Manager
	downloadMgr   *downloader.Manager
	photosMgr     *photos.Manager
	photosAIMgr   *photos.AIManager
	backupMgr     *backup.Manager
	syncMgr       *backup.SyncManager
	systemMonitor *system.Monitor
	// mediaMgr      *media.LibraryManager
}

// NewServer 创建 Web 服务器
func NewServer(storMgr *storage.Manager, userMgr *users.Manager, smbMgr *smb.Manager, nfsMgr *nfs.Manager, netMgr *network.Manager, downloadMgr *downloader.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 使用加固的安全配置
	securityConfig := DefaultSecurityConfig()

	// 中间件链 (顺序重要)
	engine.Use(inputValidationMiddleware())     // 1. 输入验证
	engine.Use(loggerMiddleware())              // 2. 结构化日志
	engine.Use(securityHeadersMiddleware())     // 3. 安全头
	engine.Use(corsMiddleware(securityConfig))  // 4. CORS (加固版)
	engine.Use(rateLimitMiddleware(securityConfig)) // 5. 速率限制
	engine.Use(csrfMiddleware(securityConfig))  // 6. CSRF 保护
	engine.Use(auditLogMiddleware())            // 7. 审计日志

	// 初始化 Docker 管理器
	dockerMgr, err := docker.NewManager()
	if err != nil {
		// Docker 不可用时继续运行
		dockerMgr = nil
	}

	// 初始化应用商店
	var appStore *docker.AppStore
	if dockerMgr != nil {
		appStore, err = docker.NewAppStore(dockerMgr, "/opt/nas")
		if err != nil {
			appStore = nil
		}
	}

	// 初始化性能监控
	perfMgr, err := perf.NewManager(nil)
	if err != nil {
		// 性能监控不可用时继续运行
		perfMgr = nil
	}

	// 初始化插件管理器
	pluginMgr, err := plugin.NewManager(plugin.ManagerConfig{
		PluginDir: "/opt/nas/plugins",
		ConfigDir: "/etc/nas-os/plugins",
		DataDir:   "/var/lib/nas-os/plugins",
	})
	if err != nil {
		// 插件系统不可用时继续运行
		pluginMgr = nil
	}

	// 初始化插件市场
	pluginMarket := plugin.NewMarket(plugin.MarketConfig{
		BaseURL: "", // 使用内置模拟数据，可配置为实际市场地址
	})

	// 初始化配额管理器
	var quotaMgr *quota.Manager
	quotaMgr, err = quota.NewManager("/etc/nas-os/quota.json", 
		quota.NewStorageAdapter(storMgr), 
		quota.NewUserAdapter(userMgr))
	if err != nil {
		// 配额管理不可用时继续运行
		quotaMgr = nil
	}

	// 初始化文件预览管理器
	filesMgr := files.NewManager(files.PreviewConfig{
		ThumbnailSize:    256,
		MaxPreviewSize:   50 * 1024 * 1024, // 50MB
		CacheDir:         "/var/cache/nas-os/thumbnails",
		CacheExpiry:      24 * time.Hour,
		EnableVideoThumb: true,
		EnableDocPreview: true,
	})

	// 初始化通知管理器
	notifyMgr := notify.NewManager()
	notify.NewHandlers(notifyMgr, "/etc/nas-os/notify-config.json")

	// 初始化 MFA 管理器
	mfaMgr, err := auth.NewMFAManager(
		"/etc/nas-os/mfa-config.json",
		"NAS-OS",
		nil, // 短信提供商，生产环境配置为 AliyunSMSProvider 或 TencentSMSProvider
	)
	if err != nil {
		// MFA 不可用时继续运行（记录日志）
		mfaMgr = nil
	}

	// 初始化相册管理器
	photosMgr, err := photos.NewManager("/var/lib/nas-os/photos")
	if err != nil {
		// 相册管理不可用时继续运行（记录日志）
		photosMgr = nil
	}

	// 初始化 AI 相册管理器
	var photosAIMgr *photos.AIManager
	if photosMgr != nil {
		photosAIMgr, err = photos.NewAIManager(photosMgr, "/var/lib/nas-os/photos/models")
		if err != nil {
			log.Printf("⚠️ AI 相册管理初始化警告：%v", err)
		} else {
			log.Println("✅ AI 相册管理模块就绪")
		}
	}

	// 初始化备份管理器
	backupMgr := backup.NewManager("/etc/nas-os/backup-config.json", "/mnt/backups")
	if err := backupMgr.Initialize(); err != nil {
		log.Printf("⚠️ 备份管理初始化警告：%v", err)
	} else {
		log.Println("✅ 备份管理模块就绪")
	}

	// 初始化同步管理器
	syncMgr := backup.NewSyncManager("/mnt/backups")
	log.Println("✅ 同步管理模块就绪")

	// 初始化系统监控器
	systemMonitor, err := system.NewMonitor("/var/lib/nas-os/system_monitor.db")
	if err != nil {
		log.Printf("⚠️ 系统监控初始化警告：%v", err)
		systemMonitor = nil
	} else {
		log.Println("✅ 系统监控模块就绪")
	}

	// 初始化媒体库管理器
	// mediaMgr := media.NewLibraryManager("/etc/nas-os/media-libraries.json")
	// 添加元数据提供商（如果配置了 API 密钥）
	// mediaMgr.AddMetadataProvider(media.NewTMDBProvider("", "zh-CN"))
	// mediaMgr.AddMetadataProvider(media.NewDoubanProvider(""))

	s := &Server{
		engine:        engine,
		storageMgr:    storMgr,
		userMgr:       userMgr,
		mfaMgr:        mfaMgr,
		smbMgr:        smbMgr,
		nfsMgr:        nfsMgr,
		networkMgr:    netMgr,
		dockerMgr:     dockerMgr,
		appStore:      appStore,
		perfMgr:       perfMgr,
		pluginMgr:     pluginMgr,
		pluginMarket:  pluginMarket,
		quotaMgr:      quotaMgr,
		filesMgr:      filesMgr,
		notifyMgr:     notifyMgr,
		downloadMgr:   downloadMgr,
		photosMgr:     photosMgr,
		photosAIMgr:   photosAIMgr,
		backupMgr:     backupMgr,
		syncMgr:       syncMgr,
		systemMonitor: systemMonitor,
		// mediaMgr:      mediaMgr,
	}

	// 添加性能监控中间件 (在日志中间件之后)
	if perfMgr != nil {
		engine.Use(perfMgr.Middleware())
	}

	s.setupRoutes()
	return s
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
		users.NewHandlers(s.userMgr, s.mfaMgr).RegisterRoutes(api)

		// ========== MFA 管理 ==========
		if s.mfaMgr != nil {
			auth.NewHandlers(s.mfaMgr).RegisterRoutes(api)
		}

		// ========== 共享管理（SMB + NFS）==========
		shares.NewHandlers(s.smbMgr, s.nfsMgr).RegisterRoutes(api)

		// ========== 网络管理 ==========
		network.NewHandlers(s.networkMgr).RegisterRoutes(api)

		// ========== Docker 管理 ==========
		if s.dockerMgr != nil {
			docker.NewHandlers(s.dockerMgr).RegisterRoutes(api)
		}

		// ========== 应用商店 ==========
		if s.appStore != nil {
			docker.NewAppHandlers(s.appStore).RegisterRoutes(api)
		}

		// ========== 系统信息 ==========
		api.GET("/system/info", s.getSystemInfo)
		api.GET("/system/health", s.getHealth)

		// ========== 性能监控 ==========
		if s.perfMgr != nil {
			perf.NewHandlers(s.perfMgr).RegisterRoutes(api)
		}

		// ========== 系统监控仪表盘 ==========
		if s.systemMonitor != nil {
			system.NewHandlers(s.systemMonitor).RegisterRoutes(api)
		}

		// ========== 插件系统 ==========
		if s.pluginMgr != nil {
			plugin.NewHandlers(s.pluginMgr, s.pluginMarket).RegisterRoutes(api)
		}

		// ========== 配额管理 ==========
		if s.quotaMgr != nil {
			quota.NewHandlers(s.quotaMgr).RegisterRoutes(api)
		}

		// ========== 文件预览 ==========
		if s.filesMgr != nil {
			files.NewHandlers(s.filesMgr).RegisterRoutes(api)
		}

		// ========== 通知管理 ==========
		notify.NewHandlers(s.notifyMgr, "/etc/nas-os/notify-config.json").RegisterRoutes(api)

		// ========== 下载中心 ==========
		if s.downloadMgr != nil {
			downloader.NewHandler(s.downloadMgr).RegisterRoutes(api)
		}

		// ========== 相册中心 ==========
		if s.photosMgr != nil {
			photos.NewHandlers(s.photosMgr, s.photosAIMgr).RegisterRoutes(api)
		}

		// ========== 备份与同步 ==========
		backupHandlers := backup.NewHandlers(s.backupMgr, s.syncMgr)
		backupHandlers.RegisterRoutes(api)

		// ========== 媒体中心 ==========
		// if s.mediaMgr != nil {
		// 	media.NewHandlers(s.mediaMgr).RegisterRoutes(api)
		// }
	}

	// Swagger API 文档
	// 访问地址: http://localhost:8080/swagger/index.html
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.URL("/swagger/doc.json"),
		ginSwagger.DefaultModelsExpandDepth(-1),
	))

	// OpenAPI JSON 规范
	s.engine.GET("/openapi.json", func(c *gin.Context) {
		c.File("./docs/swagger/swagger.json")
	})

	// OpenAPI YAML 规范
	s.engine.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("./docs/swagger/swagger.yaml")
	})

	// 静态文件（前端）
	s.engine.Static("/", "./webui/dist")
	
	// 下载中心页面
	s.engine.StaticFile("/downloader", "./webui/pages/downloader/index.html")
	s.engine.StaticFile("/downloader/", "./webui/pages/downloader/index.html")
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
	// 停止性能监控
	if s.perfMgr != nil {
		s.perfMgr.Stop()
	}
	
	// 停止配额管理
	if s.quotaMgr != nil {
		s.quotaMgr.Stop()
	}
	
	// 停止 AI 相册管理
	if s.photosAIMgr != nil {
		s.photosAIMgr.Close()
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

// ========== 卷管理 API ==========

// GenericResponse 通用 API 响应
type GenericResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags volumes
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /volumes [get]
func (s *Server) listVolumes(c *gin.Context) {
	volumes := s.storageMgr.ListVolumes()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    volumes,
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
// @Router /volumes [post]
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

// getVolume 获取卷详情
// @Summary 获取卷详情
// @Description 根据卷名称获取卷的详细信息
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 404 {object} GenericResponse "卷不存在"
// @Router /volumes/{name} [get]
func (s *Server) getVolume(c *gin.Context) {
	name := c.Param("name")
	vol := s.storageMgr.GetVolume(name)
	if vol == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "卷不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": vol})
}

// deleteVolume 删除卷
// @Summary 删除卷
// @Description 删除指定的 Btrfs 卷
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param force query bool false "强制删除"
// @Success 200 {object} GenericResponse "删除成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name} [delete]
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
// @Router /volumes/{name}/mount [post]
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
// @Router /volumes/{name}/unmount [post]
func (s *Server) unmountVolume(c *gin.Context) {
	name := c.Param("name")

	if err := s.storageMgr.UnmountVolume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "卸载成功"})
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
// @Router /volumes/{name}/usage [get]
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

// addDevice 添加设备到卷
// @Summary 添加设备到卷
// @Description 向指定卷添加存储设备
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body DeviceAddRequest true "设备参数"
// @Success 200 {object} GenericResponse "添加成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// addDevice 添加设备到卷
// @Summary 添加设备到卷
// @Description 向指定卷添加存储设备
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body DeviceAddRequest true "设备参数"
// @Success 200 {object} GenericResponse "添加成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/devices [post]
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
// @Router /volumes/{name}/devices/{device} [delete]
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

// listSubVolumes 列出子卷
// @Summary 列出子卷
// @Description 获取指定卷的所有子卷列表
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/subvolumes [get]
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

// createSubVolume 创建子卷
// @Summary 创建子卷
// @Description 在指定卷中创建新的子卷
// @Tags volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body SubVolumeCreateRequest true "子卷参数"
// @Success 200 {object} GenericResponse "创建成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/subvolumes [post]
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

// listSnapshots 列出快照
// @Summary 列出快照
// @Description 获取指定卷的所有快照列表
// @Tags snapshots
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} GenericResponse "成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /volumes/{name}/snapshots [get]
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

// getSystemInfo 获取系统信息
// @Summary 获取系统信息
// @Description 获取 NAS-OS 系统的基本信息
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /system/info [get]
func (s *Server) getSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"hostname": "nas-os",
			"version":  "0.1.0",
		},
	})
}

// getHealth 健康检查
// @Summary 健康检查
// @Description 检查系统是否正常运行
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "系统健康"
// @Router /system/health [get]
func (s *Server) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "healthy",
	})
}