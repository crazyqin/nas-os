//go:build nasd_full

package web

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	"nas-os/internal/packageruntime"
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
	"nas-os/internal/versioning"
	"nas-os/internal/vm"
	"nas-os/internal/webdav"
	"nas-os/internal/webhook"
	"nas-os/internal/webterminal"
	"nas-os/internal/wol"
	"nas-os/internal/zfs"

	// v2.498.0 新增模块.
	"nas-os/internal/containresmon"
	"nas-os/internal/dataclassify"
	"nas-os/internal/dlp"
	"nas-os/internal/dockergui"
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
	"nas-os/internal/filetag"
	"nas-os/internal/musicserver"
	"nas-os/internal/smbdirect"
	"nas-os/internal/storagecostforecast"
	"nas-os/internal/syslogserver"

	_ "nas-os/docs/swagger" // Swagger 文档

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProductsLinked reports whether product managers are compiled in.
func ProductsLinked() bool { return true }

// ExtensionsLinked reports whether official HTTP extension mounts are compiled in.
func ExtensionsLinked() bool { return true }


// Server Web 服务器.
type Server struct {
	cfg           *config.Config
	modules       []arch.Module
	extHolders          *extensionHolders             // optional modules.extensions holders
	pkgRuntime          *packageruntime.Runtime       // ADR-0001 Stage 2 unified package runtime
	communityDiscovered []packageruntime.DiskManifest // third-party manifests from community_dir
	runtimeEnabledMu         sync.Mutex
	runtimeEnabled           map[string]struct{} // App Center click-enabled set (persisted)
	httpMountedMu           sync.Mutex
	httpMounted             map[string]struct{} // gin tree nodes registered once per package id
	packageMountMu          sync.RWMutex
	packageMounted          map[string]struct{} // true unload: requests 404 when absent
	productRoutesMu         sync.Mutex
	productRoutesRegistered map[string]struct{}
	adminAPI                *gin.RouterGroup // admin /api/v1 group for late product route register
	clusterMu               sync.Mutex
	clusterServices         any // *cluster.Services when set
	clusterBootstrap        func() (any, error)
	engine                  *gin.Engine
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
	downloadMgr   any
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
	containResMonMgr *containresmon.Manager
	dataClassifyMgr  *dataclassify.Manager
	dlpMgr           *dlp.Manager
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
	customBrandingMgr      *custombranding.BrandingEngine
	smbDirectMgr           *smbdirect.SMBDirectManager
	storageCostForecastMgr *storagecostforecast.CostForecastEngine
	filetagMgr             *filetag.Manager
	apikeyMgr              *apikey.Manager
}

// NewServer 创建 Web 服务器.
func NewServer(cfg *config.Config, modules []arch.Module, storMgr *storage.Manager, userMgr *users.Manager, mfaMgr *auth.MFAManager, smbMgr *smb.Manager, nfsMgr *nfs.Manager, netMgr *network.Manager, downloadMgr any, logger *zap.Logger) *Server {
	// 如果未提供配置，回退到默认值，确保过渡期兼容。
	if cfg == nil {
		cfg = config.Default()
	}

	// 如果未提供 logger，使用 nop logger
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := newEngineWithSecurity()

	var err error
	var aclMgr *acl.Manager
	var aiSvc *ai.AIService
	var alertEngine *alertremediation.RemediationEngine
	var alertGuidedMgr *alertguided.Manager
	var apikeyMgr *apikey.Manager
	var appStore *docker.AppStore
	var backupMgr *backup.Manager
	var cloudsyncMgr *cloudsync.Manager
	var containResMonMgr *containresmon.Manager
	var customBrandingMgr *custombranding.BrandingEngine
	var dataClassifyMgr *dataclassify.Manager
	var dataWarehouseMgr *datawarehouse.Warehouse
	var dedupMgr *dedup.Manager
	var diskbenchMgr *diskbench.BenchmarkManager
	var dlpMgr *dlp.Manager
	var dockerMgr *docker.Manager
	var drDrillMgr *drdrill.Manager
	var driveSyncMgr *drivesync.Manager
	var fastTransferMgr *fasttransfer.TransferManager
	var fileDejavuMgr *filedejavu.Detector
	var fileSyncMgr *filesync.SyncManager
	var fileindexMgr *fileindex.Indexer
	var filesMgr *files.Manager
	var filetagMgr *filetag.Manager
	var healthscoreMgr *healthscore.HealthScore
	var hybridFlashMgr *hybridflash.Manager
	var iscsiMgr *iscsi.Manager
	var isoMgr *vm.ISOManager
	var lockMgr *lock.Manager
	var lxcmktMgr *lxcmkt.Manager
	var musicServerMgr *musicserver.Manager
	var netSentinelMgr *netsentinel.Manager
	var networkMapMgr *networkmap.Manager
	var notificationSvc *notification.Service
	var notifyChanMgr *notifychannel.Manager
	var notifyMgr *notify.Manager
	var nvmeofMgr *nvmeof.Manager
	var objectImmutableMgr *objectimmutable.Manager
	var officeMgr *office.Manager
	var perfMgr *perf.Manager
	var photosAIMgr *photos.AIManager
	var photosMgr *photos.Manager
	var pluginMarket *plugin.Market
	var pluginMgr *plugin.Manager
	var privacyShieldMgr *privacyshield.Shield
	var privacyVaultMgr *privacyvault.Engine
	var projectMgr *project.Manager
	var quotaMgr *quota.Manager
	var recycleCleaner *recyclecleaner.Manager
	var recycleHdl *shares.RecycleHandlers
	var remoteDesktopMgr *remotedesktop.Manager
	var s3Gateway *s3gateway.Gateway
	var s3PolicyHdl *s3.PolicyHandlers
	var schedulerMgr *scheduler.Scheduler
	var scrubSchedMgr *scrubsched.Manager
	var scrubScheduler *zfs.ScrubScheduler
	var searchEngine *search.Engine
	var searchSvc *search.GlobalSearchService
	var smartTierHdl *tiering.SmartTieringHandler
	var smbDirectMgr *smbdirect.SMBDirectManager
	var snapshotMgr *vm.SnapshotManager
	var spotlightMgr *spotlight.Manager
	var ssoHubMgr *ssohub.Manager
	var storageCostForecastMgr *storagecostforecast.CostForecastEngine
	var surveillanceMgr *surveillance.Manager
	var syncMgr *backup.SyncManager
	var syslogServerMgr *syslogserver.Manager
	var systemMonitor *system.Monitor
	var tagsMgr *tags.Manager
	var thermalMgr *thermal.Manager
	var unifiedSearchMgr *unifiedsearch.Manager
	var upsMgr *ups.Manager
	var versioningMgr *versioning.Manager
	var vmMgr *vm.Manager
	var webhookMgr *webhook.Manager
	var webterminalMgr *webterminal.Manager
	var wolMgr *wol.Manager
	// Product surface: per-product managers only when that product is wanted.
	// bulk = deprecated modules.optional kitchen-sink only.
	// packages.recommended_system → BootProductIDs() = 8 catalog products, not bulk.
	wantProducts := bootWantProducts(cfg)
	bulk := productBulkSurface(cfg)
	if bulk || len(wantProducts) > 0 {
		// 初始化 Docker 管理器（仅当 docker product 启用）
		if bulk || wantProducts["docker"] {
			dockerMgr, err = docker.NewManager()
			if err != nil {
				dockerMgr = nil
			}
			if dockerMgr != nil {
				appStore, err = docker.NewAppStore(dockerMgr, "/opt/nas")
				if err != nil {
					appStore = nil
				}
			}
		}

		// Shared non-catalog managers: only for full recommended_system bulk surface.
		if bulk {
			// 初始化性能监控
			perfMgr, err = perf.NewManager(nil)
			if err != nil {
				perfMgr = nil
			}

			// Deprecated Go .so plugin host — opt-in packages.legacy_so_plugins only.
			if cfg.LegacySOPluginHostEnabled() {
				pluginMgr, err = plugin.NewManager(plugin.ManagerConfig{
					PluginDir: "/opt/nas/plugins",
					ConfigDir: cfg.ConfigPath("plugins"),
					DataDir:   cfg.DataPath("plugins"),
				})
				if err != nil {
					pluginMgr = nil
				}
				if pluginMgr != nil {
					pluginMarket = plugin.NewMarket(plugin.MarketConfig{
						BaseURL: "",
					})
				}
			}

			// 初始化配额管理器
			quotaMgr, err = quota.NewManager(cfg.ConfigPath("quota.json"),
				quota.NewStorageAdapter(storMgr),
				quota.NewUserAdapter(userMgr))
			if err != nil {
				quotaMgr = nil
			}

			// 初始化文件预览管理器
			filesMgr = files.NewManager(files.PreviewConfig{
				ThumbnailSize:    256,
				MaxPreviewSize:   50 * 1024 * 1024,
				CacheDir:         cfg.DataPath("cache", "thumbnails"),
				CacheExpiry:      24 * time.Hour,
				EnableVideoThumb: true,
				EnableDocPreview: true,
			})

			// 初始化通知管理器
			notifyMgr = notify.NewManager()
			notify.NewHandlers(notifyMgr, cfg.ConfigPath("notify-config.json"))

			// 系统监控（会注册 /system/*；勿与下方 Core 的 /system/info 双注册）
			systemMonitor, err = system.NewMonitor(cfg.DataPath("system_monitor.db"))
			if err != nil {
				log.Printf("⚠️ 系统监控初始化警告：%v", err)
				systemMonitor = nil
			} else {
				log.Println("✅ 系统监控模块就绪")
			}

			// 引导式告警修复引擎
			alertEngine = alertremediation.NewEngine(logger)
			log.Println("✅ 引导式告警修复引擎就绪")
		}

		// MFA 管理器由 Application 构造并注入，身份模块拥有唯一实例。

		// 相册
		if bulk || wantProducts["photos"] {
			photosMgr = photos.NewManager(cfg.DataPath("photos"))
			if photosMgr != nil {
				photosAIMgr, err = photos.NewAIManager(photosMgr, cfg.DataPath("photos", "models"))
				if err != nil {
					log.Printf("⚠️ AI 相册管理初始化警告：%v", err)
				} else {
					log.Println("✅ AI 相册管理模块就绪")
				}
			}
		}

		// 备份
		if bulk || wantProducts["backup"] {
			backupMgr = backup.NewManager(cfg.ConfigPath("backup-config.json"), cfg.MountPath("backups"))
			if err := backupMgr.Initialize(); err != nil {
				log.Printf("⚠️ 备份管理初始化警告：%v", err)
			} else {
				log.Println("✅ 备份管理模块就绪")
			}
			syncMgr = backup.NewSyncManager(cfg.MountPath("backups"))
			log.Println("✅ 同步管理模块就绪")
		}

		// 虚拟机
		if bulk || wantProducts["vm"] {
			vmStoragePath := cfg.MountPath("vms")
			vmLogger := zap.NewNop()
			vmMgr, err = vm.NewManager(vmStoragePath, vmLogger)
			if err != nil {
				log.Printf("⚠️ 虚拟机管理初始化警告：%v", err)
				vmMgr = nil
			} else {
				log.Println("✅ 虚拟机管理模块就绪")
			}
			isoMgr, err = vm.NewISOManager(cfg.MountPath("isos"), vmLogger)
			if err != nil {
				log.Printf("⚠️ ISO 管理初始化警告：%v", err)
				isoMgr = nil
			} else {
				log.Println("✅ ISO 管理模块就绪")
			}
			if vmMgr != nil {
				snapshotMgr, err = vm.NewSnapshotManager(vmStoragePath, vmMgr, vmLogger)
				if err != nil {
					log.Printf("⚠️ 快照管理初始化警告：%v", err)
					snapshotMgr = nil
				} else {
					log.Println("✅ 快照管理模块就绪")
				}
			}
		}

		// AI
		if bulk || wantProducts["ai"] {
			aiSvc, err = ai.NewAIService(nil)
			if err != nil {
				log.Printf("⚠️ 私有云AI服务初始化警告：%v", err)
				aiSvc = nil
			} else {
				log.Println("✅ 私有云AI服务就绪")
			}
		}

		// ========== bulk-only optional managers (not individual product packages) ==========
		if bulk {
		// 初始化智能分层规则引擎（对标群晖 Smarter Tiering）
		tierMgr := tiering.NewManager(cfg.ConfigPath("tiering.json"), tiering.PolicyEngineConfig{})
		tierRulesEngine := tiering.NewRulesEngine(tierMgr, cfg.DataPath("tiering"))
		smartTierEngine := tiering.NewAutoTierEngine(tierMgr, tierRulesEngine, cfg.DataPath("tiering"))
		costAnalyzer := tiering.NewCostAnalyzer(tierMgr)
		smartTierHdl = tiering.NewSmartTieringHandler(smartTierEngine, costAnalyzer)
		log.Println("✅ 智能分层规则引擎就绪")

		// 初始化SMB共享回收站（对标群晖回收站）
		recycleHdl = shares.NewRecycleHandlers(smbMgr)
		log.Println("✅ SMB共享回收站就绪")

		// 初始化ZFS智能Scrub调度器（对标 TrueNAS 26 智能Scrub）
		scrubConfig := zfs.DefaultScrubScheduleConfig()
		scrubScheduler = zfs.NewScrubScheduler("tank", scrubConfig)
		// ZFS scrub 调度器由 Server.Start 统一启动。
		log.Println("✅ ZFS智能Scrub调度器就绪")

		// 初始化S3策略与管理API（对标 TrueNAS V160 S3增强）
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
		upsMgr = ups.NewManager(ups.DefaultUPSConfig())
		// UPS 监控由 Server.Start 统一启动。
		log.Println("✅ UPS电源监控就绪")

		// 初始化网络唤醒 WOL（对标群晖 WOL）
		wolMgr = wol.NewManager()
		log.Println("✅ 网络唤醒模块就绪")

		// 初始化细粒度 ACL 权限控制（对标群晖 ACL）
		aclMgr = acl.NewManager()
		log.Println("✅ 细粒度ACL权限控制就绪")

		// 初始化 Webhook 通知集成（对标群晖 Webhook 通知）
		webhookMgr = webhook.NewManager()
		log.Println("✅ Webhook通知集成就绪")

		// 初始化回收站自动清理（对标群晖 回收站策略）
		recycleCleaner = recyclecleaner.NewManager()
		// 回收站清理由 Server.Start 统一启动。
		log.Println("✅ 回收站自动清理就绪")

		// 初始化多渠道通知管理（对标群晖 多通知渠道）
		notifyChanMgr = notifychannel.NewManager()
		log.Println("✅ 多渠道通知管理就绪")

		// 初始化版本控制管理器
		versioningMgr, err = versioning.NewManager(cfg.ConfigPath("versioning-config.json"), nil)
		if err != nil {
			log.Printf("⚠️ 版本控制初始化警告：%v", err)
			versioningMgr = nil
		} else {
			log.Println("✅ 版本控制模块就绪")
		}

		// 初始化数据去重管理器
		dedupMgr, err = dedup.NewManager(cfg.ConfigPath("dedup-config.json"), nil)
		if err != nil {
			log.Printf("⚠️ 数据去重初始化警告：%v", err)
			dedupMgr = nil
		} else {
			log.Println("✅ 数据去重模块就绪")
		}

		// 初始化标签管理器
		tagsMgr, err = tags.NewManager(cfg.DataPath("tags.db"))
		if err != nil {
			log.Printf("⚠️ 标签管理初始化警告：%v", err)
			tagsMgr = nil
		} else {
			log.Println("✅ 标签管理模块就绪")
		}

		// 初始化 OnlyOffice 管理器
		officeMgr, err = office.NewManager(cfg.ConfigPath("office.json"), nil)
		if err != nil {
			log.Printf("⚠️ OnlyOffice 初始化警告：%v", err)
			officeMgr = nil
		} else {
			log.Println("✅ OnlyOffice 文档编辑模块就绪")
		}

		// 初始化 iSCSI 管理器
		iscsiMgr, err = iscsi.NewManager(cfg.ConfigPath("iscsi-config.json"), cfg.DataPath("iscsi"))
		if err != nil {
			log.Printf("⚠️ iSCSI 初始化警告：%v", err)
			iscsiMgr = nil
		} else {
			log.Println("✅ iSCSI 目标管理模块就绪")
		}

		// 初始化 NVMe-oF 管理器
		nvmeofMgr, err = nvmeof.NewManager(cfg.ConfigPath("nvmeof-config.json"))
		if err != nil {
			log.Printf("⚠️ NVMe-oF 初始化警告：%v", err)
			nvmeofMgr = nil
		} else {
			log.Println("✅ NVMe-oF 模块就绪")
		}

		// 初始化项目管理器
		projectMgr = project.NewManager()
		log.Println("✅ 项目管理模块就绪")

		// 初始化文件锁管理器
		lockMgr = lock.NewManager(lock.FileLockConfig{
			DefaultTimeout:  30 * time.Minute,
			MaxTimeout:      24 * time.Hour,
			CleanupInterval: 5 * time.Minute,
			MaxLocksPerFile: 10,
		}, logger)
		log.Println("✅ 文件锁管理模块就绪")

		// 初始化搜索引擎
		searchEngine, err = search.NewEngine(search.IndexConfig{
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
		drDrillMgr = drdrill.NewManager(logger, nil, nil)
		log.Println("✅ 容灾演练模块就绪")

		// 初始化 Drive Sync 管理器（对标群晖 Drive Sync）
		driveSyncMgr = drivesync.NewManager(cfg.ConfigPath("drivesync.json"))
		log.Println("✅ Drive Sync 模块就绪")

		// 初始化智能Scrub调度管理器（对标 TrueNAS 26 智能Scrub）
		scrubSchedMgr = scrubsched.NewManager(cfg.ConfigPath("scrubsched.json"), nil, nil, nil, nil, nil)
		// 智能 scrub 调度由 Server.Start 统一启动。
		log.Println("✅ 智能Scrub调度管理器就绪")

		// 初始化S3对象存储网关（对标 MinIO/S3 兼容层）
		s3Gateway = s3gateway.NewGateway(s3gateway.GatewayConfig{
			StorageRoot: cfg.DataPath("s3-gateway"),
			Region:      "us-east-1",
		})
		log.Println("✅ S3对象存储网关就绪")

		// 初始化定时任务调度器
		schedulerCfg := &scheduler.Config{
			MaxConcurrentTasks: 10,
			StoragePath:        cfg.DataPath("scheduler"),
		}
		schedulerMgr, err = scheduler.NewScheduler(schedulerCfg)
		if err != nil {
			log.Printf("⚠️ 定时任务调度器初始化警告：%v", err)
			schedulerMgr = nil
		} else {
			log.Println("✅ 定时任务调度器就绪")
		}

		// 初始化磁盘性能测试管理器（对标群晖 Presto Benchmark）
		diskbenchMgr = diskbench.NewBenchmarkManager("/tmp/nas-bench")
		log.Println("✅ 磁盘性能测试模块就绪")

		// 初始化系统健康评分管理器（对标 TrueNAS Dashboard）
		healthscoreMgr = healthscore.NewHealthScoreManager()
		healthscoreMgr.SetWeights(healthscore.DefaultWeights)
		dc := healthscore.NewDefaultCollectors(healthscoreMgr)
		dc.RegisterDefaultCollectors()
		log.Println("✅ 系统健康评分模块就绪")

		// 初始化高速传输管理器（对标群晖 Presto File Server）
		fastTransferMgr = fasttransfer.NewTransferManager(&fasttransfer.Config{
			MaxConcurrent: 4,
			CompressLevel: 6,
			EncryptAES:    true,
			ChunkSizeMB:   64,
		})
		log.Println("✅ 高速传输模块就绪")

		// 初始化温控管理器（系统散热与温控管理）
		thermalMgr = thermal.NewManager(logger)
		if err := thermalMgr.LoadZones(); err != nil {
			log.Printf("⚠️ 温控管理加载警告：%v", err)
		}
		log.Println("✅ 温控管理模块就绪")

		// 初始化文件索引器（全文索引与搜索）
		fileindexMgr = fileindex.NewIndexer(logger, cfg.Paths.MountBase)
		log.Println("✅ 文件索引模块就绪")

		// 初始化Web终端管理器（WebSocket SSH终端）
		webterminalMgr = webterminal.NewManager()
		log.Println("✅ Web终端模块就绪")

		// 初始化通知中心服务（对标群晖 Notification Center）
		notificationSvc, err = notification.NewService(nil)
		if err != nil {
			log.Printf("⚠️ 通知中心初始化失败: %v", err)
		} else {
			log.Println("✅ 通知中心模块就绪")
		}

		// ========== v2.498.0 新增模块初始化 ==========

		// 初始化容器资源监控（对标群晖 Container Manager 增强）
		containResMonMgr = containresmon.NewManager(&containresmon.Config{Enabled: true, MonitorIntervalSec: 30})
		log.Println("✅ 容器资源监控模块就绪")

		// 初始化数据分类（对标群晖 AI 分类）
		dataClassifyMgr = dataclassify.NewManager(&dataclassify.Config{Enabled: true, AutoClassify: true, DetectPII: true})
		log.Println("✅ 数据分类模块就绪")

		// 初始化数据防泄漏 DLP（竞品独有功能）
		dlpMgr = dlp.NewManager(&dlp.Config{Enabled: true, ScanIntervalHours: 24})
		log.Println("✅ 数据防泄漏模块就绪")

		// 初始化文件同步（对标群晖 Drive Sync）
		fileSyncMgr = filesync.NewSyncManager(logger, cfg.DataPath("filesync"))
		log.Println("✅ 文件同步模块就绪")

		// 初始化网络哨兵（对标群晖网络工具增强）
		netSentinelMgr = netsentinel.NewManager(&netsentinel.Config{Enabled: true, MonitorInterval: 60})
		log.Println("✅ 网络哨兵模块就绪")

		// 初始化网络拓扑（对标群晖网络地图）
		networkMapMgr = networkmap.NewManager(&networkmap.Config{Enabled: true, AutoDiscover: true, BandwidthMonitor: true})
		log.Println("✅ 网络拓扑模块就绪")

		// 初始化隐私保险库（竞品独有功能）
		privacyVaultMgr = privacyvault.NewEngine(&privacyvault.PrivacyVaultConfig{Enabled: true, DefaultAlgorithm: "AES-256-GCM", MaxVaults: 100})
		log.Println("✅ 隐私保险库模块就绪")

		// 初始化远程桌面（竞品独有功能）
		remoteDesktopMgr = remotedesktop.NewManager(&remotedesktop.Config{Enabled: true, MaxSessions: 10, WebSocketPort: 8443})
		log.Println("✅ 远程桌面模块就绪")

		// 初始化 SSO Hub（竞品独有功能）
		ssoHubMgr = ssohub.NewManager(&ssohub.Config{Enabled: true, SessionTimeoutMin: 480, MaxSessions: 100})
		log.Println("✅ SSO Hub模块就绪")

		// 初始化监控中心（对标群晖 Surveillance Station）
		surveillanceMgr = surveillance.NewManager()
		log.Println("✅ 监控中心模块就绪")

		// 初始化统一搜索（对标群晖 Universal Search 增强）
		unifiedSearchMgr, err = unifiedsearch.NewManager(unifiedsearch.DefaultSearchConfig(), logger)
		if err != nil {
			log.Printf("⚠️ 统一搜索初始化警告: %v", err)
		}
		log.Println("✅ 统一搜索模块就绪")

		// v2.513.0 新增模块初始化
		alertGuidedMgr = alertguided.NewManager(logger)
		log.Println("✅ 智能告警引导就绪")
		dataWarehouseMgr = datawarehouse.NewWarehouse(10000)
		log.Println("✅ 数据仓库就绪")
		fileDejavuMgr = filedejavu.NewDetector(nil)
		log.Println("✅ 重复文件检测就绪")
		hybridFlashMgr = hybridflash.NewManager(logger)
		log.Println("✅ 混合闪存管理就绪")
		lxcmktMgr = lxcmkt.NewManager(logger)
		log.Println("✅ LXC 容器市场就绪")
		objectImmutableMgr = objectimmutable.NewManager(logger)
		log.Println("✅ WORM 不可变存储就绪")
		privacyShieldMgr = privacyshield.NewShield()
		log.Println("✅ 隐私保护盾就绪")
		spotlightMgr = spotlight.NewManager(logger)

		// v2.542.0 新增模块初始化
		musicServerMgr = musicserver.NewManager()
		syslogServerMgr = syslogserver.NewManager()
		log.Println("✅ Spotlight 索引就绪")

		// v2.548.0 新增模块初始化
		customBrandingMgr = custombranding.New()
		smbDirectMgr = smbdirect.New(smbdirect.DefaultConfig())
		storageCostForecastMgr = storagecostforecast.New()
		filetagMgr = filetag.NewManager()
		apikeyMgr = apikey.NewManager()
		log.Println("✅ v2.548.0 新增模块就绪")
		} // end bulk-only managers

		// 云同步（独立 product 包，可单独启用）
		if bulk || wantProducts["cloudsync"] {
			cloudsyncMgr = cloudsync.NewManager(cfg.ConfigPath("cloudsync-config.json"))
			if err := cloudsyncMgr.Initialize(); err != nil {
				log.Printf("⚠️ 云同步初始化警告：%v", err)
			} else {
				log.Println("✅ 云同步模块就绪")
			}
		}

	} else {
		log.Println("ℹ️  packages/modules optional off: non-Core product managers not constructed")
	}
	if res := cfg.ResolvePackages(); len(res.Warnings) > 0 || res.RecommendedSystem || len(res.Enabled) > 0 {
		for _, w := range res.Warnings {
			log.Printf("⚠️  %s", w)
		}
		if res.RecommendedSystem || len(res.Enabled) > 0 {
			log.Printf("ℹ️  packages resolved: recommended_system=%v enabled=%v (prefer packages.* over deprecated modules.*)",
				res.RecommendedSystem, res.Enabled)
		}
	}

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
		rbacMgr: auth.NewRBACManager(),
		// Non-catalog companions: ONLY deprecated modules.optional (bulk kitchen-sink).
		// Enabling a single product (e.g. docker) must NOT pull tunnel/trash/ftp/….
		// Catalog products are constructed in the block above via wantProducts.
		monitorMgr: func() *monitor.Manager {
			if !bulk {
				return nil
			}
			mgr, _ := monitor.NewManager()
			return mgr
		}(),
		optimizer: func() *optimizer.PerformanceOptimizer {
			if !bulk {
				return nil
			}
			return optimizer.NewOptimizer(nil, logger)
		}(),
		projectMgr: projectMgr,
		trashMgr: func() *trash.Manager {
			if !bulk {
				return nil
			}
			mgr, _ := trash.NewManager(cfg.ConfigPath("trash.json"), cfg.DataPath("trash"), nil)
			return mgr
		}(),
		replMgr: func() *replication.Manager {
			if !bulk {
				return nil
			}
			mgr, _ := replication.NewManager(cfg.ConfigPath("replication.json"), nil)
			return mgr
		}(),
		webdavSrv: func() *webdav.Server {
			if !bulk {
				return nil
			}
			srv, _ := webdav.NewServer(nil)
			return srv
		}(),
		ftpSrv: func() *ftp.Server {
			if !bulk {
				return nil
			}
			srv, _ := ftp.NewServer(nil)
			return srv
		}(),
		sftpSrv: func() *sftp.Server {
			if !bulk {
				return nil
			}
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
			if !bulk {
				return nil
			}
			tcfg := tunnel.Config{
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
			mgr, _ := tunnel.NewManager(tcfg, logger)
			return mgr
		}(),
		tunnelService: nil, // 在服务启动时初始化
		frpManager: func() *tunnel.FRPManager {
			if !bulk {
				return nil
			}
			fcfg := &tunnel.FRPConfig{
				Enabled:       false, // 默认关闭，用户配置后启用
				ServerAddr:    "frp.nas-os.local",
				ServerPort:    7000,
				DeviceID:      "nas-device",
				DeviceName:    "NAS-OS",
				AutoReconnect: true,
				LogLevel:      "info",
			}
			return tunnel.NewFRPManager(fcfg, logger)
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
		containResMonMgr: containResMonMgr,
		dataClassifyMgr:  dataClassifyMgr,
		dlpMgr:           dlpMgr,
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
	// Shared Core route tree (modules, public health, admin auth).
	api := s.registerCorePublicAndAdminGroups()
	s.registerConfiguredExtensions(api)
	// Product routes for managers already constructed at boot.
	for id := range bootWantProducts(s.cfg) {
		s.registerProductRoutes(id)
	}
	// Bulk system monitor (if any) before shared Core identity/docs so /system/info is not dual-registered.
	if s.systemMonitor != nil {
		system.NewHandlers(s.systemMonitor).RegisterRoutes(api)
	}
	{
		// Full product-only routes (not MFA/RBAC/storage/swagger — those use registerCoreIdentityAndDocs).

		// ========== 性能监控 ==========
		if s.perfMgr != nil {
			perf.NewHandlers(s.perfMgr).RegisterRoutes(api)
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

		// ========== AI / 云同步 → registerProductRoutes ==========

		// ========== 文件版本控制 ==========
		if s.versioningMgr != nil {
			versioning.NewHandlers(s.versioningMgr).RegisterRoutes(api)
		}

		// ========== 数据去重 ==========
		if s.dedupMgr != nil {
			dedup.NewHandlers(s.dedupMgr).RegisterRoutes(api)
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

		// ========== NVMe硬件监控 / RAIDZ 扩展 ==========
		// Bulk (modules.optional) or storage-related product surface only — not bare Core.
		if productBulkSurface(s.cfg) || s.cfg.OptionalProductsEnabled() {
			hardware.NewNVMeHandlers().RegisterRoutes(api)
			storage.NewRAIDZExpansionHandlers(nil).RegisterRoutes(api)
		}

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
		if s.notifyMgr != nil {
			notify.NewHandlers(s.notifyMgr, s.cfg.ConfigPath("notify-config.json")).RegisterRoutes(api)
		}

		// download/photos/backup/vm/cloudsync/ai/docker product routes: registerProductRoutes.

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

		// UPS / WOL / ACL / webhook / recycle / notifychannel — bulk-only managers
		if s.upsMgr != nil {
			ups.NewHandlers(s.upsMgr).RegisterRoutes(api)
		}
		if s.wolMgr != nil {
			wol.NewHandlers(s.wolMgr).RegisterRoutes(api)
		}
		if s.aclMgr != nil {
			acl.NewHandlers(s.aclMgr).RegisterRoutes(api)
		}
		if s.webhookMgr != nil {
			webhook.NewHandlers(s.webhookMgr).RegisterRoutes(api)
		}
		if s.recycleCleaner != nil {
			recyclecleaner.NewHandlers(s.recycleCleaner).RegisterRoutes(api)
		}
		if s.notifyChanMgr != nil {
			notifychannel.NewHandlers(s.notifyChanMgr).RegisterRoutes(api)
		}

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
		if s.containResMonMgr != nil {
			containresmon.NewHandler(s.containResMonMgr).RegisterRoutes(newMux)
		}
		if s.dataClassifyMgr != nil {
			dataclassify.NewHandler(s.dataClassifyMgr).RegisterRoutes(newMux)
		}
		if s.dlpMgr != nil {
			dlp.NewHandler(s.dlpMgr).RegisterRoutes(newMux)
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

	// Shared MFA/RBAC/system-info-or-skip/storage/swagger/WebUI (same as Core).
	s.registerCoreIdentityAndDocs(api)
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

