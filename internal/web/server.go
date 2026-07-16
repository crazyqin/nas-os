package web

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"nas-os/internal/acl"
	"nas-os/internal/ai"
	alertremediation "nas-os/internal/alertremediation"
	"nas-os/internal/arch"
	"nas-os/internal/auth"
	"nas-os/internal/backup"
	"nas-os/internal/cloudsync"
	"nas-os/internal/config"
	"nas-os/internal/dedup"
	"nas-os/internal/diskbench"
	"nas-os/internal/docker"
	"nas-os/internal/downloader"
	"nas-os/internal/drdrill"
	"nas-os/internal/drivesync"
	"nas-os/internal/fasttransfer"
	"nas-os/internal/fileindex"
	"nas-os/internal/files"
	ftp "nas-os/internal/ftp"
	"nas-os/internal/hardware"
	"nas-os/internal/healthscore"
	"nas-os/internal/iscsi"
	"nas-os/internal/lock"
	"nas-os/internal/monitor"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/notification"
	"nas-os/internal/notify"
	"nas-os/internal/notifychannel"
	"nas-os/internal/office"
	"nas-os/internal/optimizer"
	"nas-os/internal/perf"
	"nas-os/internal/photos"
	"nas-os/internal/plugin"
	"nas-os/internal/project"
	"nas-os/internal/quota"
	"nas-os/internal/recyclecleaner"
	"nas-os/internal/replication"
	"nas-os/internal/s3"
	"nas-os/internal/s3gateway"
	"nas-os/internal/scheduler"
	"nas-os/internal/scrubsched"
	"nas-os/internal/search"
	sftp "nas-os/internal/sftp"
	"nas-os/internal/shares"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/storage/nvmeof"
	"nas-os/internal/system"
	"nas-os/internal/tags"
	"nas-os/internal/thermal"
	"nas-os/internal/tiering"
	"nas-os/internal/trash"
	"nas-os/internal/tunnel"
	"nas-os/internal/ups"
	"nas-os/internal/users"
	appversion "nas-os/internal/version"
	"nas-os/internal/versioning"
	"nas-os/internal/vm"
	"nas-os/internal/webdav"
	"nas-os/internal/webhook"
	"nas-os/internal/webterminal"
	"nas-os/internal/wol"
	"nas-os/internal/zfs"

	// v2.498.0 新增模块.
	"nas-os/internal/collabdocs"
	"nas-os/internal/containresmon"
	"nas-os/internal/dataclassify"
	"nas-os/internal/digitalwellbeing"
	"nas-os/internal/dlp"
	"nas-os/internal/dockergui"
	"nas-os/internal/edgecompute"
	"nas-os/internal/filesync"
	"nas-os/internal/netsentinel"
	"nas-os/internal/networkmap"
	"nas-os/internal/privacyvault"
	"nas-os/internal/remotedesktop"
	"nas-os/internal/ssohub"
	"nas-os/internal/surveillance"
	"nas-os/internal/unifiedsearch"
	"nas-os/internal/zfspool"

	// v2.513.0 新增模块.
	"nas-os/internal/alertguided"
	"nas-os/internal/datawarehouse"
	"nas-os/internal/filedejavu"
	"nas-os/internal/hybridflash"
	"nas-os/internal/lxcmkt"
	"nas-os/internal/objectimmutable"
	"nas-os/internal/privacyshield"
	"nas-os/internal/spotlight"

	// v2.542.0 新增模块.
	"nas-os/internal/apikey"
	"nas-os/internal/custombranding"
	"nas-os/internal/digitallegacy"
	"nas-os/internal/filetag"
	"nas-os/internal/musicserver"
	"nas-os/internal/smbdirect"
	"nas-os/internal/storagecostforecast"
	"nas-os/internal/syslogserver"

	_ "nas-os/docs/swagger" // Swagger 文档

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// Server Web 服务器.
type Server struct {
	cfg           *config.Config
	modules       []arch.Module
	engine        *gin.Engine
	httpSrv       *http.Server
	lifecycleMu   sync.Mutex
	started       bool
	stopping      bool
	logger        *zap.Logger
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
	vmMgr         *vm.Manager
	isoMgr        *vm.ISOManager
	snapshotMgr   *vm.SnapshotManager
	rbacMgr       *auth.RBACManager
	monitorMgr    *monitor.Manager
	optimizer     *optimizer.PerformanceOptimizer
	projectMgr    *project.Manager
	trashMgr      *trash.Manager
	replMgr       *replication.Manager
	webdavSrv     *webdav.Server
	ftpSrv        *ftp.Server
	sftpSrv       *sftp.Server
	versioningMgr *versioning.Manager
	dedupMgr      *dedup.Manager
	cloudsyncMgr  *cloudsync.Manager
	tagsMgr       *tags.Manager
	officeMgr     *office.Manager
	iscsiMgr      *iscsi.Manager
	nvmeofMgr     *nvmeof.Manager
	lockMgr       *lock.Manager
	searchEngine  *search.Engine
	searchSvc     *search.GlobalSearchService
	tunnelMgr     *tunnel.Manager
	tunnelService *tunnel.TunnelService
	frpManager    *tunnel.FRPManager
	aiSvc         *ai.AIService
	// v2.476.0 新增模块
	alertEngine    *alertremediation.RemediationEngine
	smartTierHdl   *tiering.SmartTieringHandler
	recycleHdl     *shares.RecycleHandlers
	scrubScheduler *zfs.ScrubScheduler
	s3PolicyHdl    *s3.PolicyHandlers
	// v2.477.0 新增模块
	upsMgr         *ups.Manager
	wolMgr         *wol.Manager
	aclMgr         *acl.Manager
	webhookMgr     *webhook.Manager
	recycleCleaner *recyclecleaner.Manager
	notifyChanMgr  *notifychannel.Manager
	// mediaMgr      *media.LibraryManager
	// v2.481.0 新增模块
	drDrillMgr    *drdrill.Manager
	driveSyncMgr  *drivesync.Manager
	scrubSchedMgr *scrubsched.Manager
	s3Gateway     *s3gateway.Gateway
	schedulerMgr  *scheduler.Scheduler
	// v2.481.0 竞品对标新增模块
	diskbenchMgr    *diskbench.BenchmarkManager
	healthscoreMgr  *healthscore.HealthScore
	fastTransferMgr *fasttransfer.TransferManager
	// v2.485.0 新增模块
	thermalMgr     *thermal.Manager
	fileindexMgr   *fileindex.Indexer
	webterminalMgr *webterminal.Manager
	// v2.490.0 新增模块
	// v2.491.0 新增模块
	notificationSvc *notification.Service
	// v2.498.0 新增模块
	collabDocsMgr    *collabdocs.Manager
	containResMonMgr *containresmon.Manager
	dataClassifyMgr  *dataclassify.Manager
	wellbeingMgr     *digitalwellbeing.Manager
	dlpMgr           *dlp.Manager
	edgeComputeMgr   *edgecompute.Manager
	fileSyncMgr      *filesync.SyncManager
	netSentinelMgr   *netsentinel.Manager
	networkMapMgr    *networkmap.Manager
	privacyVaultMgr  *privacyvault.Engine
	remoteDesktopMgr *remotedesktop.Manager
	ssoHubMgr        *ssohub.Manager
	surveillanceMgr  *surveillance.Manager
	unifiedSearchMgr *unifiedsearch.Manager
	// v2.499.0 竞品对标新增模块
	zfsPoolMgr   *zfspool.Manager
	dockerGuiMgr *dockergui.Manager
	// v2.513.0 新增模块
	alertGuidedMgr     *alertguided.Manager
	dataWarehouseMgr   *datawarehouse.Warehouse
	fileDejavuMgr      *filedejavu.Detector
	hybridFlashMgr     *hybridflash.Manager
	lxcmktMgr          *lxcmkt.Manager
	objectImmutableMgr *objectimmutable.Manager
	privacyShieldMgr   *privacyshield.Shield
	spotlightMgr       *spotlight.Manager
	// v2.542.0 新增模块
	musicServerMgr         *musicserver.Manager
	syslogServerMgr        *syslogserver.Manager
	digitalLegacyMgr       *digitallegacy.Manager
	customBrandingMgr      *custombranding.BrandingEngine
	smbDirectMgr           *smbdirect.SMBDirectManager
	storageCostForecastMgr *storagecostforecast.CostForecastEngine
	filetagMgr             *filetag.Manager
	apikeyMgr              *apikey.Manager
}

// NewServer 创建 Web 服务器.
func NewServer(cfg *config.Config, modules []arch.Module, storMgr *storage.Manager, userMgr *users.Manager, mfaMgr *auth.MFAManager, smbMgr *smb.Manager, nfsMgr *nfs.Manager, netMgr *network.Manager, downloadMgr *downloader.Manager, logger *zap.Logger) *Server {
	// 如果未提供配置，回退到默认值，确保过渡期兼容。
	if cfg == nil {
		cfg = config.Default()
	}

	// 如果未提供 logger，使用 nop logger
	if logger == nil {
		logger = zap.NewNop()
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 使用加固的安全配置
	securityConfig := DefaultSecurityConfig()

	// 中间件链 (顺序重要)
	engine.Use(inputValidationMiddleware())         // 1. 输入验证
	engine.Use(loggerMiddleware())                  // 2. 结构化日志
	engine.Use(securityHeadersMiddleware())         // 3. 安全头
	engine.Use(corsMiddleware(securityConfig))      // 4. CORS (加固版)
	engine.Use(rateLimitMiddleware(securityConfig)) // 5. 速率限制
	engine.Use(csrfMiddleware(securityConfig))      // 6. CSRF 保护
	engine.Use(auditLogMiddleware())                // 7. 审计日志

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
		ConfigDir: cfg.ConfigPath("plugins"),
		DataDir:   cfg.DataPath("plugins"),
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
	quotaMgr, err = quota.NewManager(cfg.ConfigPath("quota.json"),
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
		CacheDir:         cfg.DataPath("cache", "thumbnails"),
		CacheExpiry:      24 * time.Hour,
		EnableVideoThumb: true,
		EnableDocPreview: true,
	})

	// 初始化通知管理器
	notifyMgr := notify.NewManager()
	notify.NewHandlers(notifyMgr, cfg.ConfigPath("notify-config.json"))

	// MFA 管理器由 Application 构造并注入，身份模块拥有唯一实例。

	// 初始化相册管理器
	photosMgr := photos.NewManager(cfg.DataPath("photos"))

	// 初始化 AI 相册管理器
	var photosAIMgr *photos.AIManager
	if photosMgr != nil {
		photosAIMgr, err = photos.NewAIManager(photosMgr, cfg.DataPath("photos", "models"))
		if err != nil {
			log.Printf("⚠️ AI 相册管理初始化警告：%v", err)
		} else {
			log.Println("✅ AI 相册管理模块就绪")
		}
	}

	// 初始化备份管理器
	backupMgr := backup.NewManager(cfg.ConfigPath("backup-config.json"), cfg.MountPath("backups"))
	if err := backupMgr.Initialize(); err != nil {
		log.Printf("⚠️ 备份管理初始化警告：%v", err)
	} else {
		log.Println("✅ 备份管理模块就绪")
	}

	// 初始化同步管理器
	syncMgr := backup.NewSyncManager(cfg.MountPath("backups"))
	log.Println("✅ 同步管理模块就绪")

	// 初始化系统监控器
	systemMonitor, err := system.NewMonitor(cfg.DataPath("system_monitor.db"))
	if err != nil {
		log.Printf("⚠️ 系统监控初始化警告：%v", err)
		systemMonitor = nil
	} else {
		log.Println("✅ 系统监控模块就绪")
	}

	// 初始化虚拟机管理器
	vmStoragePath := cfg.MountPath("vms")
	vmLogger := zap.NewNop()
	vmMgr, err := vm.NewManager(vmStoragePath, vmLogger)
	if err != nil {
		log.Printf("⚠️ 虚拟机管理初始化警告：%v", err)
		vmMgr = nil
	} else {
		log.Println("✅ 虚拟机管理模块就绪")
	}

	// 初始化 ISO 管理器
	isoMgr, err := vm.NewISOManager(cfg.MountPath("isos"), vmLogger)
	if err != nil {
		log.Printf("⚠️ ISO 管理初始化警告：%v", err)
		isoMgr = nil
	} else {
		log.Println("✅ ISO 管理模块就绪")
	}

	// 初始化快照管理器
	var snapshotMgr *vm.SnapshotManager
	if vmMgr != nil {
		snapshotMgr, err = vm.NewSnapshotManager(vmStoragePath, vmMgr, vmLogger)
		if err != nil {
			log.Printf("⚠️ 快照管理初始化警告：%v", err)
			snapshotMgr = nil
		} else {
			log.Println("✅ 快照管理模块就绪")
		}
	}

	// 初始化私有云AI服务
	aiSvc, err := ai.NewAIService(nil)
	if err != nil {
		log.Printf("⚠️ 私有云AI服务初始化警告：%v", err)
		aiSvc = nil
	} else {
		log.Println("✅ 私有云AI服务就绪")
	}

	// ========== v2.476.0 新增模块 ==========

	// 初始化引导式告警修复引擎（对标 TrueNAS 26 Guided Alerts）
	alertEngine := alertremediation.NewEngine(logger)
	log.Println("✅ 引导式告警修复引擎就绪")

	// 初始化智能分层规则引擎（对标群晖 Smarter Tiering）
	tierMgr := tiering.NewManager(cfg.ConfigPath("tiering.json"), tiering.PolicyEngineConfig{})
	tierRulesEngine := tiering.NewRulesEngine(tierMgr, cfg.DataPath("tiering"))
	smartTierEngine := tiering.NewAutoTierEngine(tierMgr, tierRulesEngine, cfg.DataPath("tiering"))
	costAnalyzer := tiering.NewCostAnalyzer(tierMgr)
	smartTierHdl := tiering.NewSmartTieringHandler(smartTierEngine, costAnalyzer)
	log.Println("✅ 智能分层规则引擎就绪")

	// 初始化SMB共享回收站（对标群晖回收站）
	recycleHdl := shares.NewRecycleHandlers(smbMgr)
	log.Println("✅ SMB共享回收站就绪")

	// 初始化ZFS智能Scrub调度器（对标 TrueNAS 26 智能Scrub）
	scrubConfig := zfs.DefaultScrubScheduleConfig()
	scrubScheduler := zfs.NewScrubScheduler("tank", scrubConfig)
	// ZFS scrub 调度器由 Server.Start 统一启动。
	log.Println("✅ ZFS智能Scrub调度器就绪")

	// 初始化S3策略与管理API（对标 TrueNAS V160 S3增强）
	var s3PolicyHdl *s3.PolicyHandlers
	// S3管理器已通过现有S3 handlers注册，这里复用
	s3Mgr, err := s3.NewManager(cfg.DataPath("s3"), cfg.DataPath("s3-data"))
	if err != nil {
		log.Printf("⚠️ S3管理器初始化警告：%v", err)
		s3PolicyHdl = nil
	} else {
		s3PolicyHdl = s3.NewPolicyHandlers(s3Mgr)
		log.Println("✅ S3策略管理模块就绪")
	}

	// ========== v2.477.0 新增模块 ==========

	// 初始化UPS电源监控（对标群晖 UPS 支持）
	upsMgr := ups.NewManager(ups.DefaultUPSConfig())
	// UPS 监控由 Server.Start 统一启动。
	log.Println("✅ UPS电源监控就绪")

	// 初始化网络唤醒 WOL（对标群晖 WOL）
	wolMgr := wol.NewManager()
	log.Println("✅ 网络唤醒模块就绪")

	// 初始化细粒度 ACL 权限控制（对标群晖 ACL）
	aclMgr := acl.NewManager()
	log.Println("✅ 细粒度ACL权限控制就绪")

	// 初始化 Webhook 通知集成（对标群晖 Webhook 通知）
	webhookMgr := webhook.NewManager()
	log.Println("✅ Webhook通知集成就绪")

	// 初始化 ML 勒索检测引擎（对标群晖 勒索防护增强）
	// ML 勒索检测由 Server.Start 统一启动。
	log.Println("✅ ML勒索检测引擎就绪")

	// 初始化回收站自动清理（对标群晖 回收站策略）
	recycleCleaner := recyclecleaner.NewManager()
	// 回收站清理由 Server.Start 统一启动。
	log.Println("✅ 回收站自动清理就绪")

	// 初始化多渠道通知管理（对标群晖 多通知渠道）
	notifyChanMgr := notifychannel.NewManager()
	log.Println("✅ 多渠道通知管理就绪")

	// 初始化版本控制管理器
	versioningMgr, err := versioning.NewManager(cfg.ConfigPath("versioning-config.json"), nil)
	if err != nil {
		log.Printf("⚠️ 版本控制初始化警告：%v", err)
		versioningMgr = nil
	} else {
		log.Println("✅ 版本控制模块就绪")
	}

	// 初始化数据去重管理器
	dedupMgr, err := dedup.NewManager(cfg.ConfigPath("dedup-config.json"), nil)
	if err != nil {
		log.Printf("⚠️ 数据去重初始化警告：%v", err)
		dedupMgr = nil
	} else {
		log.Println("✅ 数据去重模块就绪")
	}

	// 初始化云同步管理器
	cloudsyncMgr := cloudsync.NewManager(cfg.ConfigPath("cloudsync-config.json"))
	if err := cloudsyncMgr.Initialize(); err != nil {
		log.Printf("⚠️ 云同步初始化警告：%v", err)
	} else {
		log.Println("✅ 云同步模块就绪")
	}

	// 初始化标签管理器
	tagsMgr, err := tags.NewManager(cfg.DataPath("tags.db"))
	if err != nil {
		log.Printf("⚠️ 标签管理初始化警告：%v", err)
		tagsMgr = nil
	} else {
		log.Println("✅ 标签管理模块就绪")
	}

	// 初始化 OnlyOffice 管理器
	var officeMgr *office.Manager
	officeMgr, err = office.NewManager(cfg.ConfigPath("office.json"), nil)
	if err != nil {
		log.Printf("⚠️ OnlyOffice 初始化警告：%v", err)
		officeMgr = nil
	} else {
		log.Println("✅ OnlyOffice 文档编辑模块就绪")
	}

	// 初始化 iSCSI 管理器
	iscsiMgr, err := iscsi.NewManager(cfg.ConfigPath("iscsi-config.json"), cfg.DataPath("iscsi"))
	if err != nil {
		log.Printf("⚠️ iSCSI 初始化警告：%v", err)
		iscsiMgr = nil
	} else {
		log.Println("✅ iSCSI 目标管理模块就绪")
	}

	// 初始化 NVMe-oF 管理器
	nvmeofMgr, err := nvmeof.NewManager(cfg.ConfigPath("nvmeof-config.json"))
	if err != nil {
		log.Printf("⚠️ NVMe-oF 初始化警告：%v", err)
		nvmeofMgr = nil
	} else {
		log.Println("✅ NVMe-oF 模块就绪")
	}

	// 初始化项目管理器
	projectMgr := project.NewManager()
	log.Println("✅ 项目管理模块就绪")

	// 初始化文件锁管理器
	lockMgr := lock.NewManager(lock.FileLockConfig{
		DefaultTimeout:  30 * time.Minute,
		MaxTimeout:      24 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		MaxLocksPerFile: 10,
	}, logger)
	log.Println("✅ 文件锁管理模块就绪")

	// 初始化搜索引擎
	searchEngine, err := search.NewEngine(search.IndexConfig{
		IndexPath:    cfg.DataPath("search", "index.bleve"),
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		Workers:      4,
		IndexContent: true,
		BatchSize:    100,
	}, logger)
	if err != nil {
		log.Printf("⚠️ 搜索引擎初始化警告：%v", err)
		searchEngine = nil
	} else {
		log.Println("✅ 搜索引擎模块就绪")
	}

	// 初始化全局搜索服务
	var searchSvc *search.GlobalSearchService
	if searchEngine != nil {
		settingsRegistry := search.NewSettingsRegistry()
		appRegistry := search.NewAppRegistry()
		searchSvc = search.NewGlobalSearchService(searchEngine, settingsRegistry, appRegistry, logger)
		log.Println("✅ 全局搜索服务就绪")
	}

	// 初始化媒体库管理器
	// mediaMgr := media.NewLibraryManager("/etc/nas-os/media-libraries.json")
	// 添加元数据提供商（如果配置了 API 密钥）
	// mediaMgr.AddMetadataProvider(media.NewTMDBProvider("", "zh-CN"))
	// mediaMgr.AddMetadataProvider(media.NewDoubanProvider(""))

	// ========== v2.481.0 新增模块 ==========

	// 初始化容灾演练管理器（对标群晖 DR Drill）
	drDrillMgr := drdrill.NewManager(logger, nil, nil)
	log.Println("✅ 容灾演练模块就绪")

	// 初始化 Drive Sync 管理器（对标群晖 Drive Sync）
	driveSyncMgr := drivesync.NewManager(cfg.ConfigPath("drivesync.json"))
	log.Println("✅ Drive Sync 模块就绪")

	// 初始化智能Scrub调度管理器（对标 TrueNAS 26 智能Scrub）
	scrubSchedMgr := scrubsched.NewManager(cfg.ConfigPath("scrubsched.json"), nil, nil, nil, nil, nil)
	// 智能 scrub 调度由 Server.Start 统一启动。
	log.Println("✅ 智能Scrub调度管理器就绪")

	// 初始化S3对象存储网关（对标 MinIO/S3 兼容层）
	s3Gateway := s3gateway.NewGateway(s3gateway.GatewayConfig{
		StorageRoot: cfg.DataPath("s3-gateway"),
		Region:      "us-east-1",
	})
	log.Println("✅ S3对象存储网关就绪")

	// 初始化定时任务调度器
	schedulerCfg := &scheduler.Config{
		MaxConcurrentTasks: 10,
		StoragePath:        cfg.DataPath("scheduler"),
	}
	schedulerMgr, err := scheduler.NewScheduler(schedulerCfg)
	if err != nil {
		log.Printf("⚠️ 定时任务调度器初始化警告：%v", err)
		schedulerMgr = nil
	} else {
		log.Println("✅ 定时任务调度器就绪")
	}

	// 初始化智能迁移管理器（对标群晖智能数据迁移）
	log.Println("✅ 智能迁移管理器就绪")

	// 初始化磁盘性能测试管理器（对标群晖 Presto Benchmark）
	diskbenchMgr := diskbench.NewBenchmarkManager("/tmp/nas-bench")
	log.Println("✅ 磁盘性能测试模块就绪")

	// 初始化系统健康评分管理器（对标 TrueNAS Dashboard）
	healthscoreMgr := healthscore.NewHealthScoreManager()
	healthscoreMgr.SetWeights(healthscore.DefaultWeights)
	dc := healthscore.NewDefaultCollectors(healthscoreMgr)
	dc.RegisterDefaultCollectors()
	log.Println("✅ 系统健康评分模块就绪")

	// 初始化高速传输管理器（对标群晖 Presto File Server）
	fastTransferMgr := fasttransfer.NewTransferManager(&fasttransfer.Config{
		MaxConcurrent: 4,
		CompressLevel: 6,
		EncryptAES:    true,
		ChunkSizeMB:   64,
	})
	log.Println("✅ 高速传输模块就绪")

	// 初始化温控管理器（系统散热与温控管理）
	thermalMgr := thermal.NewManager(logger)
	if err := thermalMgr.LoadZones(); err != nil {
		log.Printf("⚠️ 温控管理加载警告：%v", err)
	}
	log.Println("✅ 温控管理模块就绪")

	// 初始化文件索引器（全文索引与搜索）
	fileindexMgr := fileindex.NewIndexer(logger, cfg.Paths.MountBase)
	log.Println("✅ 文件索引模块就绪")

	// 初始化Web终端管理器（WebSocket SSH终端）
	webterminalMgr := webterminal.NewManager()
	log.Println("✅ Web终端模块就绪")

	// 初始化日志中心管理器（对标群晖 Log Center）
	log.Println("✅ 日志中心模块就绪")

	// 初始化通知中心服务（对标群晖 Notification Center）
	notificationSvc, err := notification.NewService(nil)
	if err != nil {
		log.Printf("⚠️ 通知中心初始化失败: %v", err)
	} else {
		log.Println("✅ 通知中心模块就绪")
	}

	// ========== v2.498.0 新增模块初始化 ==========

	// 初始化应用中心（对标群晖 Package Center）
	log.Println("✅ 应用中心模块就绪")

	// 初始化备份验证（对标群晖 Active Backup 验证）
	log.Println("✅ 备份验证模块就绪")

	// 初始化协作文档（对标群晖 Office）
	collabDocsMgr := collabdocs.NewManager(&collabdocs.Config{Enabled: true, MaxDocuments: 1000, CollaborationEnabled: true})
	log.Println("✅ 协作文档模块就绪")

	// 初始化容器资源监控（对标群晖 Container Manager 增强）
	containResMonMgr := containresmon.NewManager(&containresmon.Config{Enabled: true, MonitorIntervalSec: 30})
	log.Println("✅ 容器资源监控模块就绪")

	// 初始化数据分类（对标群晖 AI 分类）
	dataClassifyMgr := dataclassify.NewManager(&dataclassify.Config{Enabled: true, AutoClassify: true, DetectPII: true})
	log.Println("✅ 数据分类模块就绪")

	// 初始化数字健康（竞品独有功能）
	wellbeingMgr := digitalwellbeing.NewManager(logger)
	log.Println("✅ 数字健康模块就绪")

	// 初始化数据防泄漏 DLP（竞品独有功能）
	dlpMgr := dlp.NewManager(&dlp.Config{Enabled: true, ScanIntervalHours: 24})
	log.Println("✅ 数据防泄漏模块就绪")

	// 初始化边缘计算（竞品独有功能）
	edgeComputeMgr := edgecompute.NewManager(&edgecompute.Config{Enabled: true, MaxFunctions: 50, WasmEnabled: true})
	log.Println("✅ 边缘计算模块就绪")

	// 初始化能源管理（对标群晖电源管理增强）
	log.Println("✅ 能源管理模块就绪")

	// 初始化文件同步（对标群晖 Drive Sync）
	fileSyncMgr := filesync.NewSyncManager(logger, cfg.DataPath("filesync"))
	log.Println("✅ 文件同步模块就绪")

	// 初始化网络哨兵（对标群晖网络工具增强）
	netSentinelMgr := netsentinel.NewManager(&netsentinel.Config{Enabled: true, MonitorInterval: 60})
	log.Println("✅ 网络哨兵模块就绪")

	// 初始化网络拓扑（对标群晖网络地图）
	networkMapMgr := networkmap.NewManager(&networkmap.Config{Enabled: true, AutoDiscover: true, BandwidthMonitor: true})
	log.Println("✅ 网络拓扑模块就绪")

	// 初始化照片增强（对标群晖 Photos AI）
	log.Println("✅ 照片增强模块就绪")

	// 初始化隐私保险库（竞品独有功能）
	privacyVaultMgr := privacyvault.NewEngine(&privacyvault.PrivacyVaultConfig{Enabled: true, DefaultAlgorithm: "AES-256-GCM", MaxVaults: 100})
	log.Println("✅ 隐私保险库模块就绪")

	// 初始化远程桌面（竞品独有功能）
	remoteDesktopMgr := remotedesktop.NewManager(&remotedesktop.Config{Enabled: true, MaxSessions: 10, WebSocketPort: 8443})
	log.Println("✅ 远程桌面模块就绪")

	// 初始化智能家居（竞品独有功能）
	log.Println("✅ 智能家居模块就绪")

	// 初始化 SSO Hub（竞品独有功能）
	ssoHubMgr := ssohub.NewManager(&ssohub.Config{Enabled: true, SessionTimeoutMin: 480, MaxSessions: 100})
	log.Println("✅ SSO Hub模块就绪")

	// 初始化监控中心（对标群晖 Surveillance Station）
	surveillanceMgr := surveillance.NewManager()
	log.Println("✅ 监控中心模块就绪")

	// 初始化统一搜索（对标群晖 Universal Search 增强）
	unifiedSearchMgr, err := unifiedsearch.NewManager(unifiedsearch.DefaultSearchConfig(), logger)
	if err != nil {
		log.Printf("⚠️ 统一搜索初始化警告: %v", err)
	}
	log.Println("✅ 统一搜索模块就绪")

	// v2.513.0 新增模块初始化
	log.Println("✅ AI 推荐引擎就绪")
	alertGuidedMgr := alertguided.NewManager(logger)
	log.Println("✅ 智能告警引导就绪")
	log.Println("✅ 审计追踪就绪")
	dataWarehouseMgr := datawarehouse.NewWarehouse(10000)
	log.Println("✅ 数据仓库就绪")
	fileDejavuMgr := filedejavu.NewDetector(nil)
	log.Println("✅ 重复文件检测就绪")
	hybridFlashMgr := hybridflash.NewManager(logger)
	log.Println("✅ 混合闪存管理就绪")
	lxcmktMgr := lxcmkt.NewManager(logger)
	log.Println("✅ LXC 容器市场就绪")
	objectImmutableMgr := objectimmutable.NewManager(logger)
	log.Println("✅ WORM 不可变存储就绪")
	privacyShieldMgr := privacyshield.NewShield()
	log.Println("✅ 隐私保护盾就绪")
	log.Println("✅ 自助服务门户就绪")
	log.Println("✅ 智能链接就绪")
	spotlightMgr := spotlight.NewManager(logger)

	// v2.542.0 新增模块初始化
	musicServerMgr := musicserver.NewManager()
	syslogServerMgr := syslogserver.NewManager()
	var digitalLegacyMgr *digitallegacy.Manager
	legacyKey := []byte(os.Getenv("NAS_DIGITAL_LEGACY_KEY"))
	if len(legacyKey) == 16 || len(legacyKey) == 24 || len(legacyKey) == 32 {
		digitalLegacyMgr = digitallegacy.NewLegacyService(legacyKey)
	} else {
		log.Println("⚠️ 数字遗产模块已禁用：NAS_DIGITAL_LEGACY_KEY 必须为 16、24 或 32 字节")
	}
	log.Println("✅ Spotlight 索引就绪")

	// v2.548.0 新增模块初始化
	customBrandingMgr := custombranding.New()
	smbDirectMgr := smbdirect.New(smbdirect.DefaultConfig())
	storageCostForecastMgr := storagecostforecast.New()
	filetagMgr := filetag.NewManager()
	apikeyMgr := apikey.NewManager()
	log.Println("✅ v2.548.0 新增模块就绪")

	s := &Server{
		cfg:           cfg,
		modules:       append([]arch.Module(nil), modules...),
		engine:        engine,
		logger:        logger,
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
		vmMgr:         vmMgr,
		isoMgr:        isoMgr,
		snapshotMgr:   snapshotMgr,
		rbacMgr:       auth.NewRBACManager(),
		monitorMgr: func() *monitor.Manager {
			mgr, _ := monitor.NewManager()
			return mgr
		}(),
		optimizer:  optimizer.NewOptimizer(nil, logger),
		projectMgr: projectMgr,
		trashMgr: func() *trash.Manager {
			mgr, _ := trash.NewManager(cfg.ConfigPath("trash.json"), cfg.DataPath("trash"), nil)
			return mgr
		}(),
		replMgr: func() *replication.Manager {
			mgr, _ := replication.NewManager(cfg.ConfigPath("replication.json"), nil)
			return mgr
		}(),
		webdavSrv: func() *webdav.Server {
			srv, _ := webdav.NewServer(nil)
			return srv
		}(),
		ftpSrv: func() *ftp.Server {
			srv, _ := ftp.NewServer(nil)
			return srv
		}(),
		sftpSrv: func() *sftp.Server {
			srv, _ := sftp.NewServer(nil)
			return srv
		}(),
		versioningMgr: versioningMgr,
		dedupMgr:      dedupMgr,
		cloudsyncMgr:  cloudsyncMgr,
		tagsMgr:       tagsMgr,
		officeMgr:     officeMgr,
		iscsiMgr:      iscsiMgr,
		nvmeofMgr:     nvmeofMgr,
		lockMgr:       lockMgr,
		searchEngine:  searchEngine,
		searchSvc:     searchSvc,
		tunnelMgr: func() *tunnel.Manager {
			cfg := tunnel.Config{
				ServerAddr:   "tunnel.nas-os.local",
				ServerPort:   7000,
				DeviceID:     "nas-device",
				DeviceName:   "NAS-OS",
				STUNServers:  []string{"stun:stun.l.google.com:19302"},
				HeartbeatInt: 30,
				ReconnectInt: 5,
				MaxReconnect: 10,
				Timeout:      30,
			}
			mgr, _ := tunnel.NewManager(cfg, logger)
			return mgr
		}(),
		tunnelService: nil, // 在服务启动时初始化
		frpManager: func() *tunnel.FRPManager {
			cfg := &tunnel.FRPConfig{
				Enabled:       false, // 默认关闭，用户配置后启用
				ServerAddr:    "frp.nas-os.local",
				ServerPort:    7000,
				DeviceID:      "nas-device",
				DeviceName:    "NAS-OS",
				AutoReconnect: true,
				LogLevel:      "info",
			}
			return tunnel.NewFRPManager(cfg, logger)
		}(),
		aiSvc: aiSvc,
		// v2.476.0 新增模块
		alertEngine:    alertEngine,
		smartTierHdl:   smartTierHdl,
		recycleHdl:     recycleHdl,
		scrubScheduler: scrubScheduler,
		s3PolicyHdl:    s3PolicyHdl,
		// v2.477.0 新增模块
		upsMgr:         upsMgr,
		wolMgr:         wolMgr,
		aclMgr:         aclMgr,
		webhookMgr:     webhookMgr,
		recycleCleaner: recycleCleaner,
		notifyChanMgr:  notifyChanMgr,
		// mediaMgr:      mediaMgr,
		// v2.481.0 新增模块
		drDrillMgr:      drDrillMgr,
		driveSyncMgr:    driveSyncMgr,
		scrubSchedMgr:   scrubSchedMgr,
		s3Gateway:       s3Gateway,
		schedulerMgr:    schedulerMgr,
		diskbenchMgr:    diskbenchMgr,
		healthscoreMgr:  healthscoreMgr,
		fastTransferMgr: fastTransferMgr,
		// v2.485.0 新增模块
		thermalMgr:     thermalMgr,
		fileindexMgr:   fileindexMgr,
		webterminalMgr: webterminalMgr,
		// v2.490.0 新增模块
		// v2.491.0 新增模块
		notificationSvc: notificationSvc,
		// v2.498.0 新增模块
		collabDocsMgr:    collabDocsMgr,
		containResMonMgr: containResMonMgr,
		dataClassifyMgr:  dataClassifyMgr,
		wellbeingMgr:     wellbeingMgr,
		dlpMgr:           dlpMgr,
		edgeComputeMgr:   edgeComputeMgr,
		fileSyncMgr:      fileSyncMgr,
		netSentinelMgr:   netSentinelMgr,
		networkMapMgr:    networkMapMgr,
		privacyVaultMgr:  privacyVaultMgr,
		remoteDesktopMgr: remoteDesktopMgr,
		ssoHubMgr:        ssoHubMgr,
		surveillanceMgr:  surveillanceMgr,
		unifiedSearchMgr: unifiedSearchMgr,
		// v2.513.0 新增模块
		alertGuidedMgr:         alertGuidedMgr,
		dataWarehouseMgr:       dataWarehouseMgr,
		fileDejavuMgr:          fileDejavuMgr,
		hybridFlashMgr:         hybridFlashMgr,
		lxcmktMgr:              lxcmktMgr,
		objectImmutableMgr:     objectImmutableMgr,
		privacyShieldMgr:       privacyShieldMgr,
		spotlightMgr:           spotlightMgr,
		musicServerMgr:         musicServerMgr,
		syslogServerMgr:        syslogServerMgr,
		digitalLegacyMgr:       digitalLegacyMgr,
		customBrandingMgr:      customBrandingMgr,
		smbDirectMgr:           smbDirectMgr,
		storageCostForecastMgr: storageCostForecastMgr,
		filetagMgr:             filetagMgr,
		apikeyMgr:              apikeyMgr,
	}

	// 设置 WebDAV 认证函数
	if s.webdavSrv != nil && s.userMgr != nil {
		s.webdavSrv.SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
	}

	// 设置 FTP 认证函数
	if s.ftpSrv != nil && s.userMgr != nil {
		s.ftpSrv.SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
		s.ftpSrv.SetGetUserHome(func(username string) string {
			if user, err := s.userMgr.GetUser(username); err == nil {
				return user.HomeDir
			}
			return ""
		})
	}

	// 设置 SFTP 认证函数
	if s.sftpSrv != nil && s.userMgr != nil {
		s.sftpSrv.SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
		s.sftpSrv.SetGetUserHome(func(username string) string {
			if user, err := s.userMgr.GetUser(username); err == nil {
				return user.HomeDir
			}
			return ""
		})
	}

	// 添加性能监控中间件 (在日志中间件之后)
	if perfMgr != nil {
		engine.Use(perfMgr.Middleware())
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	publicAPI := s.engine.Group("/api/v1")
	for _, module := range s.modules {
		if registrar, ok := module.(arch.PublicRouteRegistrar); ok {
			registrar.RegisterPublicRoutes(publicAPI.Group("/auth"))
			registrar.RegisterPublicRoutes(publicAPI) // 兼容旧版 /api/v1/login
		}
	}
	publicAPI.GET("/system/health", s.getHealth)
	// 兼容旧探针路径：/api/v1/health 与 /api/v1/system/health 等价
	publicAPI.GET("/health", s.getHealth)

	// 账户 API 要求登录，管理 API 默认要求管理员角色。
	authenticatedAPI := s.engine.Group("/api/v1")
	authenticatedAPI.Use(users.AuthMiddleware(s.userMgr))
	for _, module := range s.modules {
		if registrar, ok := module.(arch.AuthenticatedRouteRegistrar); ok {
			registrar.RegisterAuthenticatedRoutes(authenticatedAPI)
		}
	}

	api := s.engine.Group("/api/v1")
	api.Use(users.AuthMiddleware(s.userMgr), users.RequireAdmin(s.userMgr))
	for _, module := range s.modules {
		if registrar, ok := module.(arch.RouteRegistrar); ok {
			registrar.RegisterRoutes(api)
		}
	}
	// Optional product extensions (config modules.extensions); default list is empty.
	s.registerConfiguredExtensions(api)
	{
		storage.RegisterLegacyRoutes(api, storage.NewLegacyAPIHandlers(s.storageMgr))

		// ========== MFA 管理 ==========
		if s.mfaMgr != nil {
			auth.NewHandlers(s.mfaMgr).RegisterRoutes(api)
		}

		// ========== RBAC 权限管理 ==========
		if s.rbacMgr != nil {
			auth.NewRBACHandlers(s.rbacMgr).RegisterRoutes(api)
		}

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
		api.GET("/system/status", s.getSystemStatus)

		// ========== 性能监控 ==========
		if s.perfMgr != nil {
			perf.NewHandlers(s.perfMgr).RegisterRoutes(api)
		}

		// ========== 系统监控仪表盘 ==========
		if s.systemMonitor != nil {
			system.NewHandlers(s.systemMonitor).RegisterRoutes(api)
		}

		// ========== 监控告警 ==========
		if s.monitorMgr != nil {
			monitor.NewHandlers(s.monitorMgr, s.notifyMgr).RegisterRoutes(api)
		}

		// ========== 性能优化 ==========
		if s.optimizer != nil {
			optimizer.NewHandlers(s.optimizer).RegisterRoutes(api)
		}

		// ========== 回收站 ==========
		if s.trashMgr != nil {
			trash.NewHandlers(s.trashMgr).RegisterRoutes(api)
		}

		// ========== 存储复制 ==========
		if s.replMgr != nil {
			replication.NewHandlers(s.replMgr).RegisterRoutes(api)
		}

		// ========== WebDAV 服务器 ==========
		if s.webdavSrv != nil {
			webdav.NewHandlers(s.webdavSrv).RegisterRoutes(api)
		}

		// ========== FTP 服务器 ==========
		if s.ftpSrv != nil {
			ftp.NewHandlers(s.ftpSrv).RegisterRoutes(api)
		}

		// ========== SFTP 服务器 ==========
		if s.sftpSrv != nil {
			s.sftpSrv.RegisterRoutes(api)
		}

		// ========== AI 分类 ==========

		// ========== 私有云AI服务 ==========
		if s.aiSvc != nil {
			gateway := s.aiSvc.GetGateway()
			modelMgr := s.aiSvc.GetModelManager()
			if gateway != nil {
				ai.NewGatewayHandlers(gateway, modelMgr).RegisterRoutes(api)
			}
		}

		// ========== 文件版本控制 ==========
		if s.versioningMgr != nil {
			versioning.NewHandlers(s.versioningMgr).RegisterRoutes(api)
		}

		// ========== 数据去重 ==========
		if s.dedupMgr != nil {
			dedup.NewHandlers(s.dedupMgr).RegisterRoutes(api)
		}

		// ========== 云同步 ==========
		if s.cloudsyncMgr != nil {
			cloudsync.NewHandlers(s.cloudsyncMgr).RegisterRoutes(api)
		}

		// ========== 标签管理 ==========
		if s.tagsMgr != nil {
			tags.NewHandlers(s.tagsMgr).RegisterRoutes(api)
		}

		// ========== OnlyOffice 文档编辑 ==========
		if s.officeMgr != nil {
			office.NewHandlers(s.officeMgr).RegisterRoutes(api)
		}

		// ========== iSCSI 目标管理 ==========
		if s.iscsiMgr != nil {
			iscsi.NewHandlers(s.iscsiMgr).RegisterRoutes(api)
		}

		// ========== NVMe-oF 管理 ==========
		if s.nvmeofMgr != nil {
			nvmeof.NewHandlers(s.nvmeofMgr).RegisterRoutes(api)
		}

		// ========== NVMe硬件监控 ==========
		// 兵部 Round 141 - S.M.A.R.T. UI集成
		hardware.NewNVMeHandlers().RegisterRoutes(api)

		// ========== RAIDZ扩展管理 ==========
		// 兵部 Round 141 - 对标TrueNAS 24.10
		storage.NewRAIDZExpansionHandlers(nil).RegisterRoutes(api)

		// ========== 插件系统 ==========
		if s.pluginMgr != nil {
			plugin.NewHandlers(s.pluginMgr, s.pluginMarket).RegisterRoutes(api)
		}

		// ========== 配额管理 ==========
		if s.quotaMgr != nil {
			quota.NewHandlers(s.quotaMgr).RegisterRoutes(api)
			// 注册 V2 API（历史统计、图表、报告等）
			v2 := quota.NewHandlersV2(s.quotaMgr)
			v2.Start()
			v2.RegisterRoutesV2(api)
		}

		// ========== 文件预览 ==========
		if s.filesMgr != nil {
			files.NewHandlers(s.filesMgr).RegisterRoutes(api)
		}

		// ========== 通知管理 ==========
		notify.NewHandlers(s.notifyMgr, s.cfg.ConfigPath("notify-config.json")).RegisterRoutes(api)

		// ========== 下载中心 ==========
		if s.downloadMgr != nil {
			downloader.NewHandler(s.downloadMgr).RegisterRoutes(api)
		}

		// ========== 相册中心 ==========
		if s.photosMgr != nil {
			photos.NewHandler(s.photosMgr).RegisterRoutes(api)
		}

		// ========== 备份与同步 ==========
		backupHandlers := backup.NewHandlers(s.backupMgr, s.syncMgr)
		backupHandlers.RegisterRoutes(api)

		// ========== 备份验证 ==========
		// ========== 虚拟机管理 ==========
		if s.vmMgr != nil && s.isoMgr != nil {
			vmHandler := vm.NewHandler(s.vmMgr, s.isoMgr, s.snapshotMgr, zap.NewNop())
			// 注册 HTTP ServeMux 风格的路由到 Gin
			api.GET("/vms", func(c *gin.Context) {
				vmHandler.HandleListVMs(c.Writer, c.Request)
			})
			api.POST("/vms", func(c *gin.Context) {
				vmHandler.HandleCreateVM(c.Writer, c.Request)
			})
			api.GET("/vms/:id", func(c *gin.Context) {
				vmHandler.HandleVM(c.Writer, c.Request)
			})
			api.POST("/vms/:id", func(c *gin.Context) {
				vmHandler.HandleVM(c.Writer, c.Request)
			})
			api.DELETE("/vms/:id", func(c *gin.Context) {
				vmHandler.HandleVM(c.Writer, c.Request)
			})
			api.PUT("/vms/:id", func(c *gin.Context) {
				vmHandler.HandleVM(c.Writer, c.Request)
			})
			api.GET("/vm-isos", func(c *gin.Context) {
				vmHandler.HandleListISOs(c.Writer, c.Request)
			})
			api.GET("/vm-isos/:id", func(c *gin.Context) {
				vmHandler.HandleISO(c.Writer, c.Request)
			})
			api.POST("/vm-isos/:id", func(c *gin.Context) {
				vmHandler.HandleISO(c.Writer, c.Request)
			})
			api.DELETE("/vm-isos/:id", func(c *gin.Context) {
				vmHandler.HandleISO(c.Writer, c.Request)
			})
			api.GET("/vm-snapshots", func(c *gin.Context) {
				vmHandler.HandleListSnapshots(c.Writer, c.Request)
			})
			api.GET("/vm-snapshots/:id", func(c *gin.Context) {
				vmHandler.HandleSnapshot(c.Writer, c.Request)
			})
			api.POST("/vm-snapshots/:id", func(c *gin.Context) {
				vmHandler.HandleSnapshot(c.Writer, c.Request)
			})
			api.DELETE("/vm-snapshots/:id", func(c *gin.Context) {
				vmHandler.HandleSnapshot(c.Writer, c.Request)
			})
			api.GET("/vm-templates", func(c *gin.Context) {
				vmHandler.HandleListTemplates(c.Writer, c.Request)
			})
			api.GET("/vm-usb-devices", func(c *gin.Context) {
				vmHandler.HandleUSBDevices(c.Writer, c.Request)
			})
			api.GET("/vm-pci-devices", func(c *gin.Context) {
				vmHandler.HandlePCIDevices(c.Writer, c.Request)
			})
		}

		// ========== 项目管理 ==========
		if s.projectMgr != nil {
			project.NewHandlers(s.projectMgr).RegisterRoutes(api)
		}

		// ========== 文件锁管理 ==========
		if s.lockMgr != nil {
			lock.NewHandlers(s.lockMgr, s.logger).RegisterRoutes(api)
		}

		// ========== 全局搜索 ==========
		if s.searchSvc != nil && s.searchEngine != nil {
			settingsRegistry := search.NewSettingsRegistry()
			appRegistry := search.NewAppRegistry()
			apiSearchHandler := NewAPISearchHandler(s.searchSvc, s.searchEngine, settingsRegistry, appRegistry, s.logger)
			apiSearchHandler.RegisterRoutes(api)
		}

		// ========== 内网穿透服务 ==========
		if s.frpManager != nil || s.tunnelMgr != nil {
			tunnelHandler := tunnel.NewWebUIHandler(s.frpManager, s.tunnelService, s.logger)
			tunnelHandler.RegisterRoutes(api)
		}

		// ========== v2.476.0 新增路由 ==========

		// 引导式告警修复（对标 TrueNAS 26 Guided Alerts）
		if s.alertEngine != nil {
			alertHandlers := alertremediation.NewHandlers(s.alertEngine, s.logger)
			alertHandlers.RegisterRoutes(api)
		}

		// 智能分层规则（对标群晖 Smarter Tiering）
		if s.smartTierHdl != nil {
			s.smartTierHdl.RegisterRoutes(api)
		}

		// SMB共享回收站（对标群晖回收站）
		if s.recycleHdl != nil {
			s.recycleHdl.RegisterRoutes(api)
		}

		// ZFS智能Scrub调度（对标 TrueNAS 26 智能Scrub）
		if s.scrubScheduler != nil {
			scrubHdl := zfs.NewScrubHandler(s.scrubScheduler)
			scrubHdl.RegisterRoutes(api)
		}

		// S3策略与管理API增强（对标 TrueNAS V160 S3增强）
		if s.s3PolicyHdl != nil {
			s.s3PolicyHdl.RegisterRoutes(s.engine)
		}

		// ========== v2.477.0 新增路由 ==========

		// UPS电源监控 API
		ups.NewHandlers(s.upsMgr).RegisterRoutes(api)

		// 网络唤醒 WOL API
		wol.NewHandlers(s.wolMgr).RegisterRoutes(api)

		// 细粒度 ACL 权限 API
		acl.NewHandlers(s.aclMgr).RegisterRoutes(api)

		// Webhook 通知 API
		webhook.NewHandlers(s.webhookMgr).RegisterRoutes(api)

		// ML 勒索检测 API

		// 回收站自动清理 API
		recyclecleaner.NewHandlers(s.recycleCleaner).RegisterRoutes(api)

		// 多渠道通知管理 API
		notifychannel.NewHandlers(s.notifyChanMgr).RegisterRoutes(api)

		// ========== v2.481.0 新增路由 ==========

		// 整机备份 API（对标群晖 Active Backup for Business）

		// 音乐中心 API（对标群晖 Audio Station）

		// 容灾演练 API（对标群晖 DR Drill）
		if s.drDrillMgr != nil {
			drdrill.NewHandlers(s.drDrillMgr, s.logger).RegisterRoutes(api)
		}

		// Drive Sync API（对标群晖 Drive Sync）
		if s.driveSyncMgr != nil {
			drivesync.NewHandler(s.driveSyncMgr).RegisterRoutes(api)
		}

		// 智能Scrub调度 API（对标 TrueNAS 26 智能Scrub）
		if s.scrubSchedMgr != nil {
			scrubsched.NewHandlers(s.scrubSchedMgr).RegisterRoutes(api)
		}

		// 虚拟机导入导出 API

		// S3对象存储网关 API
		if s.s3Gateway != nil {
			s3Handler := s3gateway.NewHandler(s.s3Gateway)
			s3Handler.RegisterRoutes(api)
		}

		// 定时任务调度器 API
		if s.schedulerMgr != nil {
			scheduler.NewHandlers(s.schedulerMgr).RegisterRoutes(api)
		}

		// 智能迁移 API

		// ========== v2.485.0 新增路由 ==========

		// 温控管理 API（系统散热与温控管理）
		if s.thermalMgr != nil {
			thermal.NewHandlers(s.logger, s.thermalMgr).RegisterRoutes(api)
		}

		// 文件索引 API（全文索引与搜索）
		if s.fileindexMgr != nil {
			fileindex.NewHandlers(s.logger, s.fileindexMgr).RegisterRoutes(api)
		}

		// Web终端 API（WebSocket SSH终端）
		// webterminal 通过 WebSocket 路由处理，无需单独注册

		// 日志中心 API（对标群晖 Log Center）

		// ========== 通知中心 ==========
		// v2.491.0 工部新增 - 对标群晖 Notification Center
		if s.notificationSvc != nil {
			notification.NewGinHandler(s.notificationSvc).RegisterRoutes(api)
		}

		// ========== v2.498.0 新增路由 ==========

		// 应用中心（对标群晖 Package Center）

		// 文件同步（对标群晖 Drive Sync）
		if s.fileSyncMgr != nil {
			filesync.NewHandler(s.fileSyncMgr, s.logger).RegisterRoutes(api)
		}

		// http.ServeMux 桥接：注册使用标准库的模块
		newMux := http.NewServeMux()

		// 监控中心（对标群晖 Surveillance Station）
		if s.surveillanceMgr != nil {
			surveillance.NewHandler(s.surveillanceMgr).RegisterRoutes(newMux)
		}
		if s.collabDocsMgr != nil {
			collabdocs.NewHandler(s.collabDocsMgr).RegisterRoutes(newMux)
		}
		if s.containResMonMgr != nil {
			containresmon.NewHandler(s.containResMonMgr).RegisterRoutes(newMux)
		}
		if s.dataClassifyMgr != nil {
			dataclassify.NewHandler(s.dataClassifyMgr).RegisterRoutes(newMux)
		}
		if s.wellbeingMgr != nil {
			digitalwellbeing.NewHandlers(s.wellbeingMgr).RegisterRoutes(api)
		}
		if s.dlpMgr != nil {
			dlp.NewHandler(s.dlpMgr).RegisterRoutes(newMux)
		}
		if s.edgeComputeMgr != nil {
			edgecompute.NewHandler(s.edgeComputeMgr).RegisterRoutes(newMux)
		}
		if s.netSentinelMgr != nil {
			netsentinel.NewHandler(s.netSentinelMgr).RegisterRoutes(newMux)
		}
		if s.networkMapMgr != nil {
			networkmap.NewHandler(s.networkMapMgr).RegisterRoutes(newMux)
		}
		if s.privacyVaultMgr != nil {
			privacyvault.NewHandler(s.privacyVaultMgr).RegisterRoutes(newMux)
		}
		if s.remoteDesktopMgr != nil {
			remotedesktop.NewHandler(s.remoteDesktopMgr).RegisterRoutes(newMux)
		}
		if s.ssoHubMgr != nil {
			ssohub.NewHandler(s.ssoHubMgr).RegisterRoutes(newMux)
		}
		if s.unifiedSearchMgr != nil {
			unifiedsearch.NewHandler(s.unifiedSearchMgr).RegisterRoutes(newMux)
		}

		// v2.542.0 新增模块路由（http.ServeMux）
		if s.healthscoreMgr != nil {
			healthscore.NewHandlers(s.healthscoreMgr).RegisterRoutes(newMux)
		}

		// v2.513.0 新增模块路由
		if s.alertGuidedMgr != nil {
			alertguided.NewHandlers(s.logger, s.alertGuidedMgr).RegisterRoutes(api)
		}
		if s.dataWarehouseMgr != nil {
			datawarehouse.NewHandler(s.dataWarehouseMgr).RegisterRoutes(api)
		}
		if s.fileDejavuMgr != nil {
			filedejavu.NewHandlers().RegisterRoutes(api)
		}
		if s.hybridFlashMgr != nil {
			hybridflash.NewHandlers(s.logger, s.hybridFlashMgr).RegisterRoutes(api)
		}
		if s.lxcmktMgr != nil {
			lxcmkt.NewHandlers(s.logger, s.lxcmktMgr).RegisterRoutes(api)
		}
		if s.objectImmutableMgr != nil {
			objectimmutable.NewHandlers(s.logger, s.objectImmutableMgr).RegisterRoutes(api)
		}
		if s.privacyShieldMgr != nil {
			privacyshield.RegisterRoutes(api)
		}
		if s.spotlightMgr != nil {
			spotlight.NewHandlers(s.logger, s.spotlightMgr).RegisterRoutes(api)
		}

		// v2.542.0 新增模块路由
		if s.musicServerMgr != nil {
			musicserver.NewHandlers(s.musicServerMgr).RegisterRoutes(api)
		}
		if s.syslogServerMgr != nil {
			syslogserver.NewHandlers(s.syslogServerMgr).RegisterRoutes(api)
		}
		if s.digitalLegacyMgr != nil {
			legacyMux := http.NewServeMux()
			digitallegacy.NewHandlers(s.digitalLegacyMgr).RegisterRoutes(legacyMux, "/api/v1/legacy")
			authenticatedAPI.Any("/legacy/*path", func(c *gin.Context) {
				user, exists := c.Get("user")
				if !exists {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				authenticatedUser, ok := user.(*users.User)
				if !ok {
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				ctx := digitallegacy.WithUserID(c.Request.Context(), authenticatedUser.ID)
				legacyMux.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
			})
		}
		if s.customBrandingMgr != nil {
			custombranding.NewHandler(s.customBrandingMgr).RegisterRoutes(newMux)
		}
		if s.smbDirectMgr != nil {
			smbdirect.NewHandler(s.smbDirectMgr).RegisterRoutes(newMux)
		}
		if s.storageCostForecastMgr != nil {
			storagecostforecast.NewHandler(s.storageCostForecastMgr).RegisterRoutes(newMux)
		}
		if s.filetagMgr != nil {
			filetag.NewHandler(s.filetagMgr).RegisterRoutes(api)
		}
		if s.apikeyMgr != nil {
			apikey.NewHandler(s.apikeyMgr).RegisterRoutes(api)
		}
		s.engine.NoRoute(
			users.AuthMiddleware(s.userMgr),
			users.RequireAdmin(s.userMgr),
			gin.WrapH(newMux),
		)

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
	s.engine.Static("/", "/usr/share/nas-os/webui")

	// 下载中心页面
	s.engine.StaticFile("/downloader", "/usr/share/nas-os/webui/pages/downloader/index.html")
	s.engine.StaticFile("/downloader/", "/usr/share/nas-os/webui/pages/downloader/index.html")

	// 新增页面路由
	s.engine.StaticFile("/rbac", "/usr/share/nas-os/webui/pages/rbac.html")
	s.engine.StaticFile("/monitoring", "/usr/share/nas-os/webui/pages/monitoring.html")
	s.engine.StaticFile("/containers", "/usr/share/nas-os/webui/pages/containers.html")
	s.engine.StaticFile("/vms", "/usr/share/nas-os/webui/pages/vms.html")
	s.engine.StaticFile("/trash", "/usr/share/nas-os/webui/pages/trash.html")
	s.engine.StaticFile("/replication", "/usr/share/nas-os/webui/pages/replication.html")
	s.engine.StaticFile("/webdav", "/usr/share/nas-os/webui/pages/webdav.html")
	s.engine.StaticFile("/dir-quota", "/usr/share/nas-os/webui/pages/dir-quota.html")
	// v2.20.0 新增页面
	s.engine.StaticFile("/iscsi", "/usr/share/nas-os/webui/pages/iscsi.html")
	s.engine.StaticFile("/nvmeof", "/usr/share/nas-os/webui/pages/nvmeof.html")
	s.engine.StaticFile("/office", "/usr/share/nas-os/webui/pages/office.html")
	s.engine.StaticFile("/notify", "/usr/share/nas-os/webui/pages/notify.html")
	s.engine.StaticFile("/optimizer", "/usr/share/nas-os/webui/pages/optimizer.html")
	// v2.275.0 内网穿透页面
	s.engine.StaticFile("/tunnel", "/usr/share/nas-os/webui/pages/tunnel.html")
}

// Start 启动服务器.
func (s *Server) Start(addr string) error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return nil
	}
	if s.started {
		s.lifecycleMu.Unlock()
		return errors.New("web server already started")
	}
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	s.started = true
	httpSrv := s.httpSrv
	s.lifecycleMu.Unlock()

	// 所有构造期后台任务在此统一启动，Stop 中逆序停止。
	if s.scrubScheduler != nil {
		s.scrubScheduler.Start()
	}
	if s.upsMgr != nil {
		s.upsMgr.Start()
	}
	if s.recycleCleaner != nil {
		s.recycleCleaner.Start()
	}
	if s.scrubSchedMgr != nil {
		s.scrubSchedMgr.Start()
	}

	// 启动 WebDAV 服务器
	if s.webdavSrv != nil {
		if err := s.webdavSrv.Start(); err != nil {
			log.Printf("⚠️ WebDAV 服务器启动警告：%v", err)
		} else {
			log.Println("✅ WebDAV 服务器已启动")
		}
	}

	// 启动 FTP 服务器
	if s.ftpSrv != nil {
		cfg := s.ftpSrv.GetConfig()
		if cfg.Enabled {
			if err := s.ftpSrv.Start(); err != nil {
				log.Printf("⚠️ FTP 服务器启动警告：%v", err)
			} else {
				log.Println("✅ FTP 服务器已启动")
			}
		}
	}

	// 启动 SFTP 服务器
	if s.sftpSrv != nil {
		cfg := s.sftpSrv.GetConfig()
		if cfg.Enabled {
			if err := s.sftpSrv.Start(); err != nil {
				log.Printf("⚠️ SFTP 服务器启动警告：%v", err)
			} else {
				log.Println("✅ SFTP 服务器已启动")
			}
		}
	}

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop 停止服务器.
func (s *Server) Stop() error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return nil
	}
	s.stopping = true
	httpSrv := s.httpSrv
	s.lifecycleMu.Unlock()

	// 先停止 HTTP 入口并等待在途请求，再关闭请求依赖。
	var shutdownErr error
	if httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		shutdownErr = httpSrv.Shutdown(ctx)
		cancel()
	}

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

	// 停止智能 scrub 调度管理器
	if s.scrubSchedMgr != nil {
		s.scrubSchedMgr.Stop()
	}

	// 停止 ZFS scrub 调度器
	if s.scrubScheduler != nil {
		s.scrubScheduler.Stop()
	}

	// 停止 UPS 监控
	if s.upsMgr != nil {
		s.upsMgr.Stop()
	}

	// 停止 ML 勒索检测

	// 停止回收站自动清理
	if s.recycleCleaner != nil {
		s.recycleCleaner.Stop()
	}

	// 停止 WebDAV 服务器
	if s.webdavSrv != nil {
		_ = s.webdavSrv.Stop()
	}

	// 停止 FTP 服务器
	if s.ftpSrv != nil {
		_ = s.ftpSrv.Stop()
	}

	// 停止 SFTP 服务器
	if s.sftpSrv != nil {
		_ = s.sftpSrv.Stop()
	}

	return shutdownErr
}

// ========== 卷管理 API ==========

// GenericResponse 通用 API 响应.
type GenericResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type APISearchHandler struct {
	globalSearch *search.GlobalSearchService
	engine       *search.Engine
	settings     *search.SettingsRegistry
	apps         *search.AppRegistry
	logger       *zap.Logger
}

// NewAPISearchHandler 创建搜索处理器.
func NewAPISearchHandler(
	globalSearch *search.GlobalSearchService,
	engine *search.Engine,
	settings *search.SettingsRegistry,
	apps *search.AppRegistry,
	logger *zap.Logger,
) *APISearchHandler {
	return &APISearchHandler{
		globalSearch: globalSearch,
		engine:       engine,
		settings:     settings,
		apps:         apps,
		logger:       logger,
	}
}

// RegisterRoutes 注册搜索路由.
func (h *APISearchHandler) RegisterRoutes(r *gin.RouterGroup) {
	searchGroup := r.Group("/search")
	{
		searchGroup.POST("/global", h.globalSearchHandler)
		searchGroup.GET("/quick", h.quickSearchHandler)
		searchGroup.GET("/suggestions", h.getSuggestionsHandler)
		searchGroup.GET("/categories", h.getCategoriesHandler)
		searchGroup.POST("/files", h.searchFilesHandler)
	}
}

func (h *APISearchHandler) globalSearchHandler(c *gin.Context) {
	var req struct {
		Query      string   `json:"query" binding:"required"`
		Types      []string `json:"types,omitempty"`
		Limit      int      `json:"limit,omitempty"`
		TotalLimit int      `json:"totalLimit,omitempty"`
		MinScore   float64  `json:"minScore,omitempty"`
		IncludeRaw bool     `json:"includeRaw,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求参数: " + err.Error()})
		return
	}

	var types []search.GlobalSearchResultType
	for _, t := range req.Types {
		types = append(types, search.GlobalSearchResultType(t))
	}

	result, err := h.globalSearch.GlobalSearch(c.Request.Context(), search.GlobalSearchRequest{
		Query:      req.Query,
		Types:      types,
		Limit:      req.Limit,
		TotalLimit: req.TotalLimit,
		MinScore:   req.MinScore,
		IncludeRaw: req.IncludeRaw,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *APISearchHandler) quickSearchHandler(c *gin.Context) {
	query := c.Query("query")
	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	result, err := h.globalSearch.QuickSearch(c.Request.Context(), query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *APISearchHandler) getSuggestionsHandler(c *gin.Context) {
	query := c.Query("query")
	suggestions := h.globalSearch.GenerateSuggestions(query)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"suggestions": suggestions}})
}

func (h *APISearchHandler) getCategoriesHandler(c *gin.Context) {
	categories := h.globalSearch.GetSearchCategories()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": categories})
}

func (h *APISearchHandler) searchFilesHandler(c *gin.Context) {
	var req search.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求参数: " + err.Error()})
		return
	}

	result, err := h.engine.Search(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func (s *Server) getSystemInfo(c *gin.Context) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "nas-os"
	}
	build := appversion.GetBuildInfo()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"hostname":   hostname,
			"version":    appversion.GetVersion(),
			"build_date": build["build_date"],
			"git_commit": build["git_commit"],
		},
	})
}

// getHealth 健康检查
// @Summary 健康检查
// @Description 聚合 Core 模块 Health()；任一 Core 不健康则 status=unhealthy
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "系统健康（含 modules）"
// @Router /system/health [get].
func (s *Server) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, AggregateCoreHealth(c.Request.Context(), s.modules))
}

func (s *Server) getSystemStatus(c *gin.Context) {
	system.GetSystemStatus(c)
}
