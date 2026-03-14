// Package main NAS-OS 主入口
package main

// @title NAS-OS API
// @version 2.41.0
// @description NAS-OS 是一个现代化的网络存储操作系统，提供卷管理、用户管理、共享管理、网络配置等功能。
// @description
// @description ## 功能模块
// @description - **卷管理**: Btrfs 卷创建、快照、RAID 配置
// @description - **用户管理**: 用户/组管理、认证授权
// @description - **共享管理**: SMB/NFS 共享配置
// @description - **网络管理**: 网络接口、DDNS、防火墙、端口转发
// @description - **Docker 管理**: 容器、镜像、应用商店
// @description - **插件系统**: 可扩展的插件架构
// @description - **配额管理**: 存储配额控制
// @description - **性能监控**: 系统性能监控与报告
// @termsOfService http://swagger.io/terms/

// @contact.name NAS-OS 团队
// @contact.url https://github.com/nas-os/nas-os
// @contact.email support@nas-os.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT 认证令牌，格式：Bearer {token}

// @securityDefinitions.apikey ApiKeyAuth
// @in query
// @name token
// @description API 密钥认证

// @tag.name volumes
// @tag.description 卷管理 API - 创建、管理 Btrfs 存储卷

// @tag.name snapshots
// @tag.description 快照管理 API - 创建、恢复、删除快照

// @tag.name users
// @tag.description 用户管理 API - 用户和用户组的 CRUD 操作

// @tag.name auth
// @tag.description 认证 API - 登录、登出、令牌刷新

// @tag.name shares
// @tag.description 共享管理 API - SMB 和 NFS 共享配置

// @tag.name network
// @tag.description 网络管理 API - 网络接口、DDNS、防火墙配置

// @tag.name docker
// @tag.description Docker 管理 API - 容器、镜像、应用管理

// @tag.name plugins
// @tag.description 插件系统 API - 插件安装、配置、管理

// @tag.name quota
// @tag.description 配额管理 API - 存储配额设置与查询

// @tag.name perf
// @tag.description 性能监控 API - 系统性能指标查询

// @tag.name system
// @tag.description 系统信息 API - 系统状态、健康检查

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"nas-os/internal/cluster"
	"nas-os/internal/downloader"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"
	"nas-os/internal/web"
)

func main() {
	log.Println("🚀 NAS-OS 启动中...")

	// 初始化日志
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	// 初始化用户管理
	userMgr, err := users.NewManager("/mnt")
	if err != nil {
		log.Fatalf("用户管理初始化失败：%v", err)
	}
	log.Println("✅ 用户管理模块就绪")

	// 初始化存储管理
	storMgr, err := storage.NewManager("/mnt")
	if err != nil {
		log.Fatalf("存储管理初始化失败：%v", err)
	}
	log.Println("✅ 存储管理模块就绪")

	// 初始化 SMB 共享
	smbMgr, err := smb.NewManagerWithUserMgr(userMgr, "/etc/samba/smb.conf")
	if err != nil {
		log.Fatalf("SMB 管理初始化失败：%v", err)
	}
	log.Println("✅ SMB 共享模块就绪")

	// 初始化 NFS 共享
	nfsMgr, err := nfs.NewManager("/etc/exports")
	if err != nil {
		log.Fatalf("NFS 管理初始化失败：%v", err)
	}
	log.Println("✅ NFS 共享模块就绪")

	// 初始化网络管理
	netMgr := network.NewManager("/etc/nas-os/network.json")
	if err := netMgr.Initialize(); err != nil {
		log.Printf("⚠️ 网络管理初始化警告：%v", err)
	}
	log.Println("✅ 网络管理模块就绪")

	// 启动 DDNS 后台任务
	netMgr.StartDDNSWorker()

	// 初始化集群服务（可选）
	hostname, _ := os.Hostname()
	clusterServices, err := cluster.InitializeCluster(cluster.ClusterRootConfig{
		NodeID:  hostname,
		DataDir: "/var/lib/nas-os",
	}, logger)
	if err != nil {
		log.Printf("⚠️ 集群服务初始化警告：%v", err)
	} else if clusterServices != nil {
		log.Println("✅ 集群服务就绪")
		defer func() {
			if err := cluster.ShutdownCluster(clusterServices); err != nil {
				logger.Error("failed to shutdown cluster", zap.Error(err))
			}
		}()
	}

	// 初始化下载管理器
	downloadMgr, err := downloader.NewManager("/var/lib/nas-os/downloads", logger)
	if err != nil {
		log.Printf("⚠️ 下载管理初始化警告：%v", err)
	} else {
		// 配置 Transmission/qBittorrent 地址（可选）
		downloadMgr.SetTransmissionURL("http://localhost:9091")
		log.Println("✅ 下载管理模块就绪")
		defer downloadMgr.Close()
	}

	// 初始化 Web 服务
	webServer := web.NewServer(storMgr, userMgr, smbMgr, nfsMgr, netMgr, downloadMgr, logger)

	// 启动 Web 服务
	go func() {
		if err := webServer.Start(":8080"); err != nil {
			log.Fatalf("Web 服务启动失败：%v", err)
		}
	}()

	log.Println("✅ NAS-OS 就绪 - Web 管理界面：http://localhost:8080")
	log.Println("📖 API 文档：http://localhost:8080/swagger/index.html")
	if clusterServices != nil {
		log.Printf("🔗 集群模式 - 节点 ID: %s, 角色：%s", hostname, getClusterRole(clusterServices))
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("👋 NAS-OS 正在关闭...")
	if err := webServer.Stop(); err != nil {
		logger.Error("failed to stop web server", zap.Error(err))
	}
}

func getClusterRole(services *cluster.ClusterServices) string {
	if services.HA.IsLeader() {
		return "leader"
	}
	return "follower"
}
