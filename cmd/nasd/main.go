// Package main NAS-OS 主入口
package main

// @title NAS-OS API
// @version 3.24.1
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
// @description
// @description ## 架构分层 (Core / Extension / Lab)
// @description 生产核心生命周期仅注册 identity / storage / network / sharing / system；
// @description 可选产品能力见 internal/extensions/*；实验与伪核心实现见 internal/lab/*。
// @termsOfService http://swagger.io/terms/

// @contact.name NAS-OS 团队
// @contact.url https://github.com/crazyqin/nas-os
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
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"nas-os/internal/application"
	"nas-os/internal/config"

	"go.uber.org/zap"
)

func main() {
	log.Println("🚀 NAS-OS 启动中...")

	var configPath string
	fs := flag.NewFlagSet("nasd", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "/etc/nas-os/config.yaml", "配置文件路径")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("参数解析失败：%v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("配置加载失败：%v", err)
	}
	log.Printf("✅ 配置就绪：config=%s mount=%s config_dir=%s data_dir=%s",
		configPath, cfg.Paths.MountBase, cfg.Paths.ConfigDir, cfg.Paths.DataDir)

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("日志初始化失败：%v", err)
	}
	defer func() { _ = logger.Sync() }()

	app, err := application.New(cfg, logger)
	if err != nil {
		logger.Fatal("application initialization failed", zap.Error(err))
	}

	ctx, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	runErr := app.Run(ctx)
	if ctx.Err() != nil {
		log.Println("👋 NAS-OS 正在关闭...")
	}
	stopErr := app.Stop()

	if runErr != nil {
		logger.Error("application exited with error", zap.Error(runErr))
	}
	if stopErr != nil {
		logger.Error("application shutdown failed", zap.Error(stopErr))
	}
	if runErr != nil || stopErr != nil {
		os.Exit(1)
	}
}
