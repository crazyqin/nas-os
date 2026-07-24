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
	cfg                     *config.Config
	modules                 []arch.Module
	extHolders              *extensionHolders
	pkgRuntime              *packageruntime.Runtime
	communityDiscovered     []packageruntime.DiskManifest
	runtimeEnabledMu        sync.Mutex
	runtimeEnabled          map[string]struct{}
	httpMountedMu           sync.Mutex
	httpMounted             map[string]struct{}
	packageMountMu          sync.RWMutex
	packageMounted          map[string]struct{}
	productRoutesMu         sync.Mutex
	productRoutesRegistered map[string]struct{}
	productReg              *productRegistry
	adminAPI                *gin.RouterGroup
	clusterMu               sync.Mutex
	clusterServices         any
	clusterBootstrap        func() (any, error)
	engine                  *gin.Engine
	httpSrv                 *http.Server
	lifecycleMu             sync.Mutex
	started                 bool
	stopping                bool
	logger                  *zap.Logger
	// Core managers (always typed)
	storageMgr *storage.Manager
	userMgr    *users.Manager
	mfaMgr     *auth.MFAManager
	smbMgr     *smb.Manager
	nfsMgr     *nfs.Manager
	networkMgr *network.Manager
	rbacMgr    *auth.RBACManager
	// Optional/product/bulk managers live in h (see holders.go) — not 90+ fields.
	h *holderBag
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
	var zfsPoolMgr *zfspool.Manager
	var dockerGuiMgr *dockergui.Manager
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
		cfg:        cfg,
		modules:    append([]arch.Module(nil), modules...),
		engine:     engine,
		logger:     logger,
		productReg: newProductRegistry(),
		h:          newHolderBag(),
		storageMgr: storMgr,
		userMgr:    userMgr,
		mfaMgr:     mfaMgr,
		smbMgr:     smbMgr,
		nfsMgr:     nfsMgr,
		networkMgr: netMgr,
		rbacMgr:    auth.NewRBACManager(),
	}

	// Wire optional/product/bulk managers into holder bag (not Server fields).
	s.setHolder("dockerMgr", dockerMgr)
	s.setHolder("appStore", appStore)
	s.setHolder("perfMgr", perfMgr)
	s.setHolder("pluginMgr", pluginMgr)
	s.setHolder("pluginMarket", pluginMarket)
	s.setHolder("quotaMgr", quotaMgr)
	s.setHolder("filesMgr", filesMgr)
	s.setHolder("notifyMgr", notifyMgr)
	s.setHolder("downloadMgr", downloadMgr)
	s.setHolder("photosMgr", photosMgr)
	s.setHolder("photosAIMgr", photosAIMgr)
	s.setHolder("backupMgr", backupMgr)
	s.setHolder("syncMgr", syncMgr)
	s.setHolder("systemMonitor", systemMonitor)
	s.setHolder("vmMgr", vmMgr)
	s.setHolder("isoMgr", isoMgr)
	s.setHolder("snapshotMgr", snapshotMgr)
	if bulk {
		if mgr, _ := monitor.NewManager(); mgr != nil {
			s.setHolder("monitorMgr", mgr)
		}
		s.setHolder("optimizer", optimizer.NewOptimizer(nil, logger))
		if mgr, _ := trash.NewManager(cfg.ConfigPath("trash.json"), cfg.DataPath("trash"), nil); mgr != nil {
			s.setHolder("trashMgr", mgr)
		}
		if mgr, _ := replication.NewManager(cfg.ConfigPath("replication.json"), nil); mgr != nil {
			s.setHolder("replMgr", mgr)
		}
		if srv, _ := webdav.NewServer(nil); srv != nil {
			s.setHolder("webdavSrv", srv)
		}
		if srv, _ := ftp.NewServer(nil); srv != nil {
			s.setHolder("ftpSrv", srv)
		}
		if srv, _ := sftp.NewServer(nil); srv != nil {
			s.setHolder("sftpSrv", srv)
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
		if mgr, _ := tunnel.NewManager(tcfg, logger); mgr != nil {
			s.setHolder("tunnelMgr", mgr)
		}
	}
	s.setHolder("projectMgr", projectMgr)
	s.setHolder("versioningMgr", versioningMgr)
	s.setHolder("dedupMgr", dedupMgr)
	s.setHolder("cloudsyncMgr", cloudsyncMgr)
	s.setHolder("tagsMgr", tagsMgr)
	s.setHolder("officeMgr", officeMgr)
	s.setHolder("iscsiMgr", iscsiMgr)
	s.setHolder("nvmeofMgr", nvmeofMgr)
	s.setHolder("lockMgr", lockMgr)
	s.setHolder("searchEngine", searchEngine)
	s.setHolder("searchSvc", searchSvc)
	s.setHolder("aiSvc", aiSvc)
	s.setHolder("alertEngine", alertEngine)
	s.setHolder("smartTierHdl", smartTierHdl)
	s.setHolder("recycleHdl", recycleHdl)
	s.setHolder("scrubScheduler", scrubScheduler)
	s.setHolder("s3PolicyHdl", s3PolicyHdl)
	s.setHolder("upsMgr", upsMgr)
	s.setHolder("wolMgr", wolMgr)
	s.setHolder("aclMgr", aclMgr)
	s.setHolder("webhookMgr", webhookMgr)
	s.setHolder("recycleCleaner", recycleCleaner)
	s.setHolder("notifyChanMgr", notifyChanMgr)
	s.setHolder("drDrillMgr", drDrillMgr)
	s.setHolder("driveSyncMgr", driveSyncMgr)
	s.setHolder("scrubSchedMgr", scrubSchedMgr)
	s.setHolder("s3Gateway", s3Gateway)
	s.setHolder("schedulerMgr", schedulerMgr)
	s.setHolder("diskbenchMgr", diskbenchMgr)
	s.setHolder("healthscoreMgr", healthscoreMgr)
	s.setHolder("fastTransferMgr", fastTransferMgr)
	s.setHolder("thermalMgr", thermalMgr)
	s.setHolder("fileindexMgr", fileindexMgr)
	s.setHolder("webterminalMgr", webterminalMgr)
	s.setHolder("notificationSvc", notificationSvc)
	s.setHolder("containResMonMgr", containResMonMgr)
	s.setHolder("dataClassifyMgr", dataClassifyMgr)
	s.setHolder("dlpMgr", dlpMgr)
	s.setHolder("fileSyncMgr", fileSyncMgr)
	s.setHolder("netSentinelMgr", netSentinelMgr)
	s.setHolder("networkMapMgr", networkMapMgr)
	s.setHolder("privacyVaultMgr", privacyVaultMgr)
	s.setHolder("remoteDesktopMgr", remoteDesktopMgr)
	s.setHolder("ssoHubMgr", ssoHubMgr)
	s.setHolder("surveillanceMgr", surveillanceMgr)
	s.setHolder("unifiedSearchMgr", unifiedSearchMgr)
	s.setHolder("zfsPoolMgr", zfsPoolMgr)
	s.setHolder("dockerGuiMgr", dockerGuiMgr)
	s.setHolder("alertGuidedMgr", alertGuidedMgr)
	s.setHolder("dataWarehouseMgr", dataWarehouseMgr)
	s.setHolder("fileDejavuMgr", fileDejavuMgr)
	s.setHolder("hybridFlashMgr", hybridFlashMgr)
	s.setHolder("lxcmktMgr", lxcmktMgr)
	s.setHolder("objectImmutableMgr", objectImmutableMgr)
	s.setHolder("privacyShieldMgr", privacyShieldMgr)
	s.setHolder("spotlightMgr", spotlightMgr)
	s.setHolder("musicServerMgr", musicServerMgr)
	s.setHolder("syslogServerMgr", syslogServerMgr)
	s.setHolder("customBrandingMgr", customBrandingMgr)
	s.setHolder("smbDirectMgr", smbDirectMgr)
	s.setHolder("storageCostForecastMgr", storageCostForecastMgr)
	s.setHolder("filetagMgr", filetagMgr)
	s.setHolder("apikeyMgr", apikeyMgr)


	// 设置 WebDAV 认证函数
	if s.hasHolder("webdavSrv") && s.userMgr != nil {
		holderAs[*webdav.Server](s, "webdavSrv").SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
	}

	// 设置 FTP 认证函数
	if s.hasHolder("ftpSrv") && s.userMgr != nil {
		holderAs[*ftp.Server](s, "ftpSrv").SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
		holderAs[*ftp.Server](s, "ftpSrv").SetGetUserHome(func(username string) string {
			if user, err := s.userMgr.GetUser(username); err == nil {
				return user.HomeDir
			}
			return ""
		})
	}

	// 设置 SFTP 认证函数
	if s.hasHolder("sftpSrv") && s.userMgr != nil {
		holderAs[*sftp.Server](s, "sftpSrv").SetAuthFunc(func(username, password string) bool {
			_, err := s.userMgr.Authenticate(username, password)
			return err == nil
		})
		holderAs[*sftp.Server](s, "sftpSrv").SetGetUserHome(func(username string) string {
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

	s.seedProductRegistry()
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
	if s.hasHolder("systemMonitor") {
		system.NewHandlers(holderAs[*system.Monitor](s, "systemMonitor")).RegisterRoutes(api)
	}
	s.registerBulkOptionalRoutes(api)

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
	// WriteTimeout 0 = unlimited body write (large downloads/uploads).
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}
	s.started = true
	httpSrv := s.httpSrv
	s.lifecycleMu.Unlock()

	// 所有构造期后台任务在此统一启动，Stop 中逆序停止。
	if s.hasHolder("scrubScheduler") {
		holderAs[*zfs.ScrubScheduler](s, "scrubScheduler").Start()
	}
	if s.hasHolder("upsMgr") {
		holderAs[*ups.Manager](s, "upsMgr").Start()
	}
	if s.hasHolder("recycleCleaner") {
		holderAs[*recyclecleaner.Manager](s, "recycleCleaner").Start()
	}
	if s.hasHolder("scrubSchedMgr") {
		holderAs[*scrubsched.Manager](s, "scrubSchedMgr").Start()
	}

	// 启动 WebDAV 服务器
	if s.hasHolder("webdavSrv") {
		if err := holderAs[*webdav.Server](s, "webdavSrv").Start(); err != nil {
			log.Printf("⚠️ WebDAV 服务器启动警告：%v", err)
		} else {
			log.Println("✅ WebDAV 服务器已启动")
		}
	}

	// 启动 FTP 服务器
	if s.hasHolder("ftpSrv") {
		cfg := holderAs[*ftp.Server](s, "ftpSrv").GetConfig()
		if cfg.Enabled {
			if err := holderAs[*ftp.Server](s, "ftpSrv").Start(); err != nil {
				log.Printf("⚠️ FTP 服务器启动警告：%v", err)
			} else {
				log.Println("✅ FTP 服务器已启动")
			}
		}
	}

	// 启动 SFTP 服务器
	if s.hasHolder("sftpSrv") {
		cfg := holderAs[*sftp.Server](s, "sftpSrv").GetConfig()
		if cfg.Enabled {
			if err := holderAs[*sftp.Server](s, "sftpSrv").Start(); err != nil {
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
	if s.hasHolder("perfMgr") {
		holderAs[*perf.Manager](s, "perfMgr").Stop()
	}

	// 停止配额管理
	if s.hasHolder("quotaMgr") {
		holderAs[*quota.Manager](s, "quotaMgr").Stop()
	}

	// 停止 AI 相册管理
	if s.hasHolder("photosAIMgr") {
		holderAs[*photos.AIManager](s, "photosAIMgr").Close()
	}

	// 停止智能 scrub 调度管理器
	if s.hasHolder("scrubSchedMgr") {
		holderAs[*scrubsched.Manager](s, "scrubSchedMgr").Stop()
	}

	// 停止 ZFS scrub 调度器
	if s.hasHolder("scrubScheduler") {
		holderAs[*zfs.ScrubScheduler](s, "scrubScheduler").Stop()
	}

	// 停止 UPS 监控
	if s.hasHolder("upsMgr") {
		holderAs[*ups.Manager](s, "upsMgr").Stop()
	}

	// 停止回收站自动清理
	if s.hasHolder("recycleCleaner") {
		holderAs[*recyclecleaner.Manager](s, "recycleCleaner").Stop()
	}

	// 停止 WebDAV 服务器
	if s.hasHolder("webdavSrv") {
		_ = holderAs[*webdav.Server](s, "webdavSrv").Stop()
	}

	// 停止 FTP 服务器
	if s.hasHolder("ftpSrv") {
		_ = holderAs[*ftp.Server](s, "ftpSrv").Stop()
	}

	// 停止 SFTP 服务器
	if s.hasHolder("sftpSrv") {
		_ = holderAs[*sftp.Server](s, "sftpSrv").Stop()
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

