//go:build nasd_full

package web

import (
	"net/http"

	"nas-os/internal/acl"
	"nas-os/internal/alertguided"
	alertremediation "nas-os/internal/alertremediation"
	"nas-os/internal/apikey"
	"nas-os/internal/containresmon"
	"nas-os/internal/custombranding"
	"nas-os/internal/dataclassify"
	"nas-os/internal/datawarehouse"
	"nas-os/internal/dedup"
	"nas-os/internal/dlp"
	"nas-os/internal/drdrill"
	"nas-os/internal/drivesync"
	"nas-os/internal/filedejavu"
	"nas-os/internal/fileindex"
	"nas-os/internal/files"
	"nas-os/internal/filesync"
	"nas-os/internal/filetag"
	ftp "nas-os/internal/ftp"
	"nas-os/internal/hardware"
	"nas-os/internal/healthscore"
	"nas-os/internal/hybridflash"
	"nas-os/internal/iscsi"
	"nas-os/internal/lock"
	"nas-os/internal/lxcmkt"
	"nas-os/internal/monitor"
	"nas-os/internal/musicserver"
	"nas-os/internal/netsentinel"
	"nas-os/internal/networkmap"
	"nas-os/internal/notification"
	"nas-os/internal/notify"
	"nas-os/internal/notifychannel"
	"nas-os/internal/objectimmutable"
	"nas-os/internal/office"
	"nas-os/internal/optimizer"
	"nas-os/internal/perf"
	"nas-os/internal/plugin"
	"nas-os/internal/privacyvault"
	"nas-os/internal/privacyshield"
	"nas-os/internal/project"
	"nas-os/internal/quota"
	"nas-os/internal/recyclecleaner"
	"nas-os/internal/remotedesktop"
	"nas-os/internal/replication"
	"nas-os/internal/s3gateway"
	"nas-os/internal/scheduler"
	"nas-os/internal/scrubsched"
	"nas-os/internal/search"
	"nas-os/internal/smbdirect"
	"nas-os/internal/spotlight"
	"nas-os/internal/ssohub"
	"nas-os/internal/storage"
	"nas-os/internal/storage/nvmeof"
	"nas-os/internal/storagecostforecast"
	"nas-os/internal/surveillance"
	"nas-os/internal/syslogserver"
	"nas-os/internal/tags"
	"nas-os/internal/thermal"
	"nas-os/internal/trash"
	"nas-os/internal/tunnel"
	"nas-os/internal/unifiedsearch"
	"nas-os/internal/ups"
	"nas-os/internal/users"
	"nas-os/internal/versioning"
	"nas-os/internal/webhook"
	"nas-os/internal/webdav"
	"nas-os/internal/wol"
	"nas-os/internal/zfs"

	"github.com/gin-gonic/gin"
	sftp "nas-os/internal/sftp"
	"nas-os/internal/tiering"
	"nas-os/internal/shares"
	"nas-os/internal/s3"
)

// registerBulkOptionalRoutes mounts Full-only optional/bulk product routes.
// Extracted from setupRoutes to keep server_full.go focused on lifecycle.
func (s *Server) registerBulkOptionalRoutes(api *gin.RouterGroup) {
		// Full product-only routes (not MFA/RBAC/storage/swagger — those use registerCoreIdentityAndDocs).

		// ========== 性能监控 ==========
		if s.hasHolder("perfMgr") {
			perf.NewHandlers(holderAs[*perf.Manager](s, "perfMgr")).RegisterRoutes(api)
		}

		// ========== 监控告警 ==========
		if s.hasHolder("monitorMgr") {
			monitor.NewHandlers(holderAs[*monitor.Manager](s, "monitorMgr"), holderAs[*notify.Manager](s, "notifyMgr")).RegisterRoutes(api)
		}

		// ========== 性能优化 ==========
		if s.hasHolder("optimizer") {
			optimizer.NewHandlers(holderAs[*optimizer.PerformanceOptimizer](s, "optimizer")).RegisterRoutes(api)
		}

		// ========== 回收站 ==========
		if s.hasHolder("trashMgr") {
			trash.NewHandlers(holderAs[*trash.Manager](s, "trashMgr")).RegisterRoutes(api)
		}

		// ========== 存储复制 ==========
		if s.hasHolder("replMgr") {
			replication.NewHandlers(holderAs[*replication.Manager](s, "replMgr")).RegisterRoutes(api)
		}

		// ========== WebDAV 服务器 ==========
		if s.hasHolder("webdavSrv") {
			webdav.NewHandlers(holderAs[*webdav.Server](s, "webdavSrv")).RegisterRoutes(api)
		}

		// ========== FTP 服务器 ==========
		if s.hasHolder("ftpSrv") {
			ftp.NewHandlers(holderAs[*ftp.Server](s, "ftpSrv")).RegisterRoutes(api)
		}

		// ========== SFTP 服务器 ==========
		if s.hasHolder("sftpSrv") {
			holderAs[*sftp.Server](s, "sftpSrv").RegisterRoutes(api)
		}

		// ========== AI / 云同步 → registerProductRoutes ==========

		// ========== 文件版本控制 ==========
		if s.hasHolder("versioningMgr") {
			versioning.NewHandlers(holderAs[*versioning.Manager](s, "versioningMgr")).RegisterRoutes(api)
		}

		// ========== 数据去重 ==========
		if s.hasHolder("dedupMgr") {
			dedup.NewHandlers(holderAs[*dedup.Manager](s, "dedupMgr")).RegisterRoutes(api)
		}

		// ========== 标签管理 ==========
		if s.hasHolder("tagsMgr") {
			tags.NewHandlers(holderAs[*tags.Manager](s, "tagsMgr")).RegisterRoutes(api)
		}

		// ========== OnlyOffice 文档编辑 ==========
		if s.hasHolder("officeMgr") {
			office.NewHandlers(holderAs[*office.Manager](s, "officeMgr")).RegisterRoutes(api)
		}

		// ========== iSCSI 目标管理 ==========
		if s.hasHolder("iscsiMgr") {
			iscsi.NewHandlers(holderAs[*iscsi.Manager](s, "iscsiMgr")).RegisterRoutes(api)
		}

		// ========== NVMe-oF 管理 ==========
		if s.hasHolder("nvmeofMgr") {
			nvmeof.NewHandlers(holderAs[*nvmeof.Manager](s, "nvmeofMgr")).RegisterRoutes(api)
		}

		// ========== NVMe硬件监控 / RAIDZ 扩展 ==========
		// Bulk (modules.optional) or storage-related product surface only — not bare Core.
		if productBulkSurface(s.cfg) || s.cfg.OptionalProductsEnabled() {
			hardware.NewNVMeHandlers().RegisterRoutes(api)
			storage.NewRAIDZExpansionHandlers(nil).RegisterRoutes(api)
		}

		// ========== 插件系统 ==========
		if s.hasHolder("pluginMgr") {
			plugin.NewHandlers(holderAs[*plugin.Manager](s, "pluginMgr"), holderAs[*plugin.Market](s, "pluginMarket")).RegisterRoutes(api)
		}

		// ========== 配额管理 ==========
		if s.hasHolder("quotaMgr") {
			quota.NewHandlers(holderAs[*quota.Manager](s, "quotaMgr")).RegisterRoutes(api)
			// 注册 V2 API（历史统计、图表、报告等）
			v2 := quota.NewHandlersV2(holderAs[*quota.Manager](s, "quotaMgr"))
			v2.Start()
			v2.RegisterRoutesV2(api)
		}

		// ========== 文件预览 ==========
		if s.hasHolder("filesMgr") {
			files.NewHandlers(holderAs[*files.Manager](s, "filesMgr")).RegisterRoutes(api)
		}

		// ========== 通知管理 ==========
		if s.hasHolder("notifyMgr") {
			notify.NewHandlers(holderAs[*notify.Manager](s, "notifyMgr"), s.cfg.ConfigPath("notify-config.json")).RegisterRoutes(api)
		}

		// download/photos/backup/vm/cloudsync/ai/docker product routes: registerProductRoutes.

		// ========== 项目管理 ==========
		if s.hasHolder("projectMgr") {
			project.NewHandlers(holderAs[*project.Manager](s, "projectMgr")).RegisterRoutes(api)
		}

		// ========== 文件锁管理 ==========
		if s.hasHolder("lockMgr") {
			lock.NewHandlers(holderAs[*lock.Manager](s, "lockMgr"), s.logger).RegisterRoutes(api)
		}

		// ========== 全局搜索 ==========
		if s.hasHolder("searchSvc") && s.hasHolder("searchEngine") {
			settingsRegistry := search.NewSettingsRegistry()
			appRegistry := search.NewAppRegistry()
			apiSearchHandler := NewAPISearchHandler(holderAs[*search.GlobalSearchService](s, "searchSvc"), holderAs[*search.Engine](s, "searchEngine"), settingsRegistry, appRegistry, s.logger)
			apiSearchHandler.RegisterRoutes(api)
		}

		// ========== 内网穿透服务 ==========
		if s.hasHolder("frpManager") || s.hasHolder("tunnelMgr") {
			tunnelHandler := tunnel.NewWebUIHandler(holderAs[*tunnel.FRPManager](s, "frpManager"), holderAs[*tunnel.TunnelService](s, "tunnelService"), s.logger)
			tunnelHandler.RegisterRoutes(api)
		}

		// ========== v2.476.0 新增路由 ==========

		// 引导式告警修复（对标 TrueNAS 26 Guided Alerts）
		if s.hasHolder("alertEngine") {
			alertHandlers := alertremediation.NewHandlers(holderAs[*alertremediation.RemediationEngine](s, "alertEngine"), s.logger)
			alertHandlers.RegisterRoutes(api)
		}

		// 智能分层规则（对标群晖 Smarter Tiering）
		if s.hasHolder("smartTierHdl") {
			holderAs[*tiering.SmartTieringHandler](s, "smartTierHdl").RegisterRoutes(api)
		}

		// SMB共享回收站（对标群晖回收站）
		if s.hasHolder("recycleHdl") {
			holderAs[*shares.RecycleHandlers](s, "recycleHdl").RegisterRoutes(api)
		}

		// ZFS智能Scrub调度（对标 TrueNAS 26 智能Scrub）
		if s.hasHolder("scrubScheduler") {
			scrubHdl := zfs.NewScrubHandler(holderAs[*zfs.ScrubScheduler](s, "scrubScheduler"))
			scrubHdl.RegisterRoutes(api)
		}

		// S3策略与管理API增强（对标 TrueNAS V160 S3增强）
		if s.hasHolder("s3PolicyHdl") {
			holderAs[*s3.PolicyHandlers](s, "s3PolicyHdl").RegisterRoutes(s.engine)
		}

		// ========== v2.477.0 新增路由 ==========

		// UPS / WOL / ACL / webhook / recycle / notifychannel — bulk-only managers
		if s.hasHolder("upsMgr") {
			ups.NewHandlers(holderAs[*ups.Manager](s, "upsMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("wolMgr") {
			wol.NewHandlers(holderAs[*wol.Manager](s, "wolMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("aclMgr") {
			acl.NewHandlers(holderAs[*acl.Manager](s, "aclMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("webhookMgr") {
			webhook.NewHandlers(holderAs[*webhook.Manager](s, "webhookMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("recycleCleaner") {
			recyclecleaner.NewHandlers(holderAs[*recyclecleaner.Manager](s, "recycleCleaner")).RegisterRoutes(api)
		}
		if s.hasHolder("notifyChanMgr") {
			notifychannel.NewHandlers(holderAs[*notifychannel.Manager](s, "notifyChanMgr")).RegisterRoutes(api)
		}

		// ========== v2.481.0 新增路由 ==========

		// 整机备份 API（对标群晖 Active Backup for Business）

		// 音乐中心 API（对标群晖 Audio Station）

		// 容灾演练 API（对标群晖 DR Drill）
		if s.hasHolder("drDrillMgr") {
			drdrill.NewHandlers(holderAs[*drdrill.Manager](s, "drDrillMgr"), s.logger).RegisterRoutes(api)
		}

		// Drive Sync API（对标群晖 Drive Sync）
		if s.hasHolder("driveSyncMgr") {
			drivesync.NewHandler(holderAs[*drivesync.Manager](s, "driveSyncMgr")).RegisterRoutes(api)
		}

		// 智能Scrub调度 API（对标 TrueNAS 26 智能Scrub）
		if s.hasHolder("scrubSchedMgr") {
			scrubsched.NewHandlers(holderAs[*scrubsched.Manager](s, "scrubSchedMgr")).RegisterRoutes(api)
		}

		// 虚拟机导入导出 API

		// S3对象存储网关 API
		if s.hasHolder("s3Gateway") {
			s3Handler := s3gateway.NewHandler(holderAs[*s3gateway.Gateway](s, "s3Gateway"))
			s3Handler.RegisterRoutes(api)
		}

		// 定时任务调度器 API
		if s.hasHolder("schedulerMgr") {
			scheduler.NewHandlers(holderAs[*scheduler.Scheduler](s, "schedulerMgr")).RegisterRoutes(api)
		}

		// 智能迁移 API

		// ========== v2.485.0 新增路由 ==========

		// 温控管理 API（系统散热与温控管理）
		if s.hasHolder("thermalMgr") {
			thermal.NewHandlers(s.logger, holderAs[*thermal.Manager](s, "thermalMgr")).RegisterRoutes(api)
		}

		// 文件索引 API（全文索引与搜索）
		if s.hasHolder("fileindexMgr") {
			fileindex.NewHandlers(s.logger, holderAs[*fileindex.Indexer](s, "fileindexMgr")).RegisterRoutes(api)
		}

		// Web终端 API（WebSocket SSH终端）
		// webterminal 通过 WebSocket 路由处理，无需单独注册

		// 日志中心 API（对标群晖 Log Center）

		// ========== 通知中心 ==========
		// v2.491.0 工部新增 - 对标群晖 Notification Center
		if s.hasHolder("notificationSvc") {
			notification.NewGinHandler(holderAs[*notification.Service](s, "notificationSvc")).RegisterRoutes(api)
		}

		// ========== v2.498.0 新增路由 ==========

		// 应用中心（对标群晖 Package Center）

		// 文件同步（对标群晖 Drive Sync）
		if s.hasHolder("fileSyncMgr") {
			filesync.NewHandler(holderAs[*filesync.SyncManager](s, "fileSyncMgr"), s.logger).RegisterRoutes(api)
		}

		// http.ServeMux 桥接：注册使用标准库的模块
		newMux := http.NewServeMux()

		// 监控中心（对标群晖 Surveillance Station）
		if s.hasHolder("surveillanceMgr") {
			surveillance.NewHandler(holderAs[*surveillance.Manager](s, "surveillanceMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("containResMonMgr") {
			containresmon.NewHandler(holderAs[*containresmon.Manager](s, "containResMonMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("dataClassifyMgr") {
			dataclassify.NewHandler(holderAs[*dataclassify.Manager](s, "dataClassifyMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("dlpMgr") {
			dlp.NewHandler(holderAs[*dlp.Manager](s, "dlpMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("netSentinelMgr") {
			netsentinel.NewHandler(holderAs[*netsentinel.Manager](s, "netSentinelMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("networkMapMgr") {
			networkmap.NewHandler(holderAs[*networkmap.Manager](s, "networkMapMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("privacyVaultMgr") {
			privacyvault.NewHandler(holderAs[*privacyvault.Engine](s, "privacyVaultMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("remoteDesktopMgr") {
			remotedesktop.NewHandler(holderAs[*remotedesktop.Manager](s, "remoteDesktopMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("ssoHubMgr") {
			ssohub.NewHandler(holderAs[*ssohub.Manager](s, "ssoHubMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("unifiedSearchMgr") {
			unifiedsearch.NewHandler(holderAs[*unifiedsearch.Manager](s, "unifiedSearchMgr")).RegisterRoutes(newMux)
		}

		// v2.542.0 新增模块路由（http.ServeMux）
		if s.hasHolder("healthscoreMgr") {
			healthscore.NewHandlers(holderAs[*healthscore.HealthScore](s, "healthscoreMgr")).RegisterRoutes(newMux)
		}

		// v2.513.0 新增模块路由
		if s.hasHolder("alertGuidedMgr") {
			alertguided.NewHandlers(s.logger, holderAs[*alertguided.Manager](s, "alertGuidedMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("dataWarehouseMgr") {
			datawarehouse.NewHandler(holderAs[*datawarehouse.Warehouse](s, "dataWarehouseMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("fileDejavuMgr") {
			filedejavu.NewHandlers().RegisterRoutes(api)
		}
		if s.hasHolder("hybridFlashMgr") {
			hybridflash.NewHandlers(s.logger, holderAs[*hybridflash.Manager](s, "hybridFlashMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("lxcmktMgr") {
			lxcmkt.NewHandlers(s.logger, holderAs[*lxcmkt.Manager](s, "lxcmktMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("objectImmutableMgr") {
			objectimmutable.NewHandlers(s.logger, holderAs[*objectimmutable.Manager](s, "objectImmutableMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("privacyShieldMgr") {
			privacyshield.RegisterRoutes(api)
		}
		if s.hasHolder("spotlightMgr") {
			spotlight.NewHandlers(s.logger, holderAs[*spotlight.Manager](s, "spotlightMgr")).RegisterRoutes(api)
		}

		// v2.542.0 新增模块路由
		if s.hasHolder("musicServerMgr") {
			musicserver.NewHandlers(holderAs[*musicserver.Manager](s, "musicServerMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("syslogServerMgr") {
			syslogserver.NewHandlers(holderAs[*syslogserver.Manager](s, "syslogServerMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("customBrandingMgr") {
			custombranding.NewHandler(holderAs[*custombranding.BrandingEngine](s, "customBrandingMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("smbDirectMgr") {
			smbdirect.NewHandler(holderAs[*smbdirect.SMBDirectManager](s, "smbDirectMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("storageCostForecastMgr") {
			storagecostforecast.NewHandler(holderAs[*storagecostforecast.CostForecastEngine](s, "storageCostForecastMgr")).RegisterRoutes(newMux)
		}
		if s.hasHolder("filetagMgr") {
			filetag.NewHandler(holderAs[*filetag.Manager](s, "filetagMgr")).RegisterRoutes(api)
		}
		if s.hasHolder("apikeyMgr") {
			apikey.NewHandler(holderAs[*apikey.Manager](s, "apikeyMgr")).RegisterRoutes(api)
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
