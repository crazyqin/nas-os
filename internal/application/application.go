// Package application 负责 NAS-OS 进程级依赖组装和生命周期管理。
package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"nas-os/internal/cluster"
	"nas-os/internal/config"
	"nas-os/internal/downloader"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"
	"nas-os/internal/web"

	"go.uber.org/zap"
)

// Application 是进程级组合根，持有需要显式关闭的服务。
type Application struct {
	cfg             *config.Config
	logger          *zap.Logger
	hostname        string
	clusterServices *cluster.Services
	downloadManager *downloader.Manager
	networkManager  *network.Manager
	webServer       *web.Server

	startOnce sync.Once
	stopOnce  sync.Once
	stopErr   error
}

// New 构造 NAS-OS 应用及其核心依赖。
// 必需模块初始化失败会终止启动；可选模块失败只记录警告并降级运行。
func New(cfg *config.Config, logger *zap.Logger) (*Application, error) {
	if cfg == nil {
		return nil, errors.New("application config is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid application config: %w", err)
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	userMgr, err := users.NewManager(cfg.Paths.MountBase)
	if err != nil {
		return nil, fmt.Errorf("initialize users: %w", err)
	}
	log.Println("✅ 用户管理模块就绪")

	storageMgr, err := storage.NewManager(cfg.Paths.MountBase)
	if err != nil {
		return nil, fmt.Errorf("initialize storage: %w", err)
	}
	log.Println("✅ 存储管理模块就绪")

	smbMgr, err := smb.NewManagerWithUserMgr(userMgr, cfg.Paths.SambaConfig)
	if err != nil {
		return nil, fmt.Errorf("initialize SMB: %w", err)
	}
	log.Println("✅ SMB 共享模块就绪")

	nfsMgr, err := nfs.NewManager(cfg.Paths.NFSExports)
	if err != nil {
		return nil, fmt.Errorf("initialize NFS: %w", err)
	}
	log.Println("✅ NFS 共享模块就绪")

	networkMgr := network.NewManager(cfg.ConfigPath("network.json"))
	if err := networkMgr.Initialize(); err != nil {
		log.Printf("⚠️ 网络管理初始化警告：%v", err)
	}
	log.Println("✅ 网络管理模块就绪")

	hostname, _ := os.Hostname()
	clusterServices, err := cluster.InitializeCluster(cluster.RootConfig{
		NodeID:  hostname,
		DataDir: cfg.Paths.DataDir,
	}, logger)
	if err != nil {
		log.Printf("⚠️ 集群服务初始化警告：%v", err)
		clusterServices = nil
	} else if clusterServices != nil {
		log.Println("✅ 集群服务就绪")
	}

	downloadMgr, err := downloader.NewManager(filepath.Join(cfg.Paths.DataDir, "downloads"), logger)
	if err != nil {
		log.Printf("⚠️ 下载管理初始化警告：%v", err)
		downloadMgr = nil
	} else {
		downloadMgr.SetTransmissionURL("http://localhost:9091")
		log.Println("✅ 下载管理模块就绪")
	}

	webServer := web.NewServer(cfg, storageMgr, userMgr, smbMgr, nfsMgr, networkMgr, downloadMgr, logger)

	return &Application{
		cfg:             cfg,
		logger:          logger,
		hostname:        hostname,
		clusterServices: clusterServices,
		downloadManager: downloadMgr,
		networkManager:  networkMgr,
		webServer:       webServer,
	}, nil
}

// Run 启动后台任务和 HTTP 服务，并阻塞到上下文取消或 HTTP 服务退出。
func (a *Application) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("run context is required")
	}

	a.startOnce.Do(func() {
		a.networkManager.StartDDNSWorker()
	})

	webErrCh := make(chan error, 1)
	go func() {
		webErrCh <- a.webServer.Start(a.cfg.Server.Addr())
	}()

	log.Printf("✅ NAS-OS 就绪 - Web 管理界面：http://%s", FriendlyAddr(a.cfg.Server))
	log.Printf("📖 API 文档：http://%s/swagger/index.html", FriendlyAddr(a.cfg.Server))
	if a.clusterServices != nil {
		log.Printf("🔗 集群模式 - 节点 ID: %s, 角色：%s", a.hostname, a.ClusterRole())
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-webErrCh:
		return err
	}
}

// Stop 按依赖逆序关闭应用；可安全重复调用。
func (a *Application) Stop() error {
	a.stopOnce.Do(func() {
		var errs []error
		if a.webServer != nil {
			if err := a.webServer.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("stop web server: %w", err))
			}
		}
		if a.downloadManager != nil {
			a.downloadManager.Close()
		}
		if a.networkManager != nil {
			a.networkManager.StopDDNSWorker()
		}
		if a.clusterServices != nil {
			if err := cluster.ShutdownCluster(a.clusterServices); err != nil {
				errs = append(errs, fmt.Errorf("stop cluster: %w", err))
			}
		}
		a.stopErr = errors.Join(errs...)
	})
	return a.stopErr
}

// ClusterRole 返回当前集群角色。
func (a *Application) ClusterRole() string {
	if a.clusterServices != nil && a.clusterServices.HA.IsLeader() {
		return "leader"
	}
	return "follower"
}

// FriendlyAddr 生成用户友好的访问地址（0.0.0.0 显示为 localhost）。
func FriendlyAddr(s config.ServerConfig) string {
	host := s.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	return fmt.Sprintf("%s:%d", host, s.Port)
}
