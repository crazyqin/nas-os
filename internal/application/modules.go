package application

import (
	"context"
	"fmt"

	"nas-os/internal/arch"
	"nas-os/internal/auth"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/shares"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"
	"nas-os/internal/web"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	moduleIdentity = "identity"
	moduleStorage  = "storage"
	moduleNetwork  = "network"
	moduleSharing  = "sharing"
	moduleSystem   = "system"
)

// coreModule 是无后台任务领域的原生 Module 实现。
type coreModule struct {
	name     string
	tier     arch.ModuleTier
	deps     []string
	initFn   func(context.Context) error
	healthFn func(context.Context) error
	logger   *zap.Logger
}

func (m *coreModule) Name() string { return m.name }
func (m *coreModule) Tier() arch.ModuleTier {
	if m.tier != "" {
		return m.tier
	}
	return ModuleTierFor(m.name)
}
func (m *coreModule) Dependencies() []string      { return append([]string(nil), m.deps...) }
func (m *coreModule) Start(context.Context) error { return nil }
func (m *coreModule) Stop(context.Context) error  { return nil }
func (m *coreModule) Init(ctx context.Context) error {
	if m.initFn != nil {
		return m.initFn(ctx)
	}
	return nil
}
func (m *coreModule) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

// identityModule 原生身份模块，同时拥有公开和已认证账户路由。
type identityModule struct {
	coreModule
	handlers *users.Handlers
}

func (m *identityModule) RegisterPublicRoutes(rg *gin.RouterGroup) {
	m.handlers.RegisterPublicRoutes(rg)
}
func (m *identityModule) RegisterAuthenticatedRoutes(rg *gin.RouterGroup) {
	m.handlers.RegisterProtectedRoutes(rg)
}

// storageModule 原生存储模块；兼容路由处理器将在后续迁入 storage 包。
type storageModule struct {
	coreModule
	compatRoutes arch.RouteRegistrar
}

func (m *storageModule) RegisterRoutes(rg *gin.RouterGroup) {
	m.compatRoutes.RegisterRoutes(rg)
}

// networkModule 原生网络模块，拥有 DDNS worker 和管理员路由。
type networkModule struct {
	coreModule
	manager  *network.Manager
	handlers *network.Handlers
}

func (m *networkModule) Start(context.Context) error {
	m.manager.StartDDNSWorker()
	return nil
}
func (m *networkModule) Stop(context.Context) error {
	m.manager.StopDDNSWorker()
	return nil
}
func (m *networkModule) RegisterRoutes(rg *gin.RouterGroup) {
	m.handlers.RegisterRoutes(rg)
}

// sharingModule 原生共享模块，拥有 SMB/NFS 聚合路由。
type sharingModule struct {
	coreModule
	handlers *shares.Handlers
}

func (m *sharingModule) RegisterRoutes(rg *gin.RouterGroup) {
	m.handlers.RegisterRoutes(rg)
}

// systemModule 暴露统一模块健康状态。
type systemModule struct {
	coreModule
	container *arch.Container
}

func (m *systemModule) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/system/modules", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    0,
			"message": "success",
			"data":    m.container.GetModulesStatus(c.Request.Context()),
		})
	})
}

// registerCoreModules 注册核心领域并返回同一批实例供 Web 层发现路由契约。
func registerCoreModules(
	container *arch.Container,
	userMgr *users.Manager,
	mfaMgr *auth.MFAManager,
	storageMgr *storage.Manager,
	networkMgr *network.Manager,
	smbMgr *smb.Manager,
	nfsMgr *nfs.Manager,
	logger *zap.Logger,
) ([]arch.Module, error) {
	identity := &identityModule{
		coreModule: coreModule{
			name:   moduleIdentity,
			initFn: requireService("users manager", userMgr),
			healthFn: func(context.Context) error {
				if userMgr == nil {
					return fmt.Errorf("users manager unavailable")
				}
				return nil
			},
			logger: logger,
		},
		handlers: users.NewHandlers(userMgr, mfaMgr),
	}
	storageMod := &storageModule{
		coreModule: coreModule{
			name:   moduleStorage,
			initFn: requireService("storage manager", storageMgr),
			healthFn: func(context.Context) error {
				if storageMgr == nil {
					return fmt.Errorf("storage manager unavailable")
				}
				return nil
			},
			logger: logger,
		},
		compatRoutes: web.NewStorageHandlers(storageMgr),
	}
	networkMod := &networkModule{
		coreModule: coreModule{
			name:   moduleNetwork,
			initFn: requireService("network manager", networkMgr),
			healthFn: func(context.Context) error {
				if networkMgr == nil {
					return fmt.Errorf("network manager unavailable")
				}
				return nil
			},
			logger: logger,
		},
		manager:  networkMgr,
		handlers: network.NewHandlers(networkMgr),
	}
	sharing := &sharingModule{
		coreModule: coreModule{
			name: moduleSharing,
			deps: []string{moduleIdentity, moduleStorage, moduleNetwork},
			initFn: func(context.Context) error {
				if smbMgr == nil || nfsMgr == nil {
					return fmt.Errorf("SMB and NFS managers are required")
				}
				return nil
			},
			healthFn: func(context.Context) error {
				if smbMgr == nil || nfsMgr == nil {
					return fmt.Errorf("sharing managers unavailable")
				}
				return nil
			},
			logger: logger,
		},
		handlers: shares.NewHandlers(smbMgr, nfsMgr),
	}
	systemMod := &systemModule{
		coreModule: coreModule{
			name:   moduleSystem,
			deps:   []string{moduleIdentity, moduleStorage, moduleNetwork, moduleSharing},
			logger: logger,
		},
		container: container,
	}

	modules := []arch.Module{identity, storageMod, networkMod, sharing, systemMod}
	for _, module := range modules {
		if err := container.RegisterModule(module); err != nil {
			return nil, err
		}
	}

	container.Register(moduleIdentity, userMgr)
	container.Register(moduleStorage, storageMgr)
	container.Register(moduleNetwork, networkMgr)
	container.Register("sharing.smb", smbMgr)
	container.Register("sharing.nfs", nfsMgr)
	return modules, nil
}

func requireService(name string, service interface{}) func(context.Context) error {
	return func(context.Context) error {
		if service == nil {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
}

var (
	_ arch.Module                      = (*identityModule)(nil)
	_ arch.PublicRouteRegistrar        = (*identityModule)(nil)
	_ arch.AuthenticatedRouteRegistrar = (*identityModule)(nil)
	_ arch.Module                      = (*storageModule)(nil)
	_ arch.RouteRegistrar              = (*storageModule)(nil)
	_ arch.Module                      = (*networkModule)(nil)
	_ arch.RouteRegistrar              = (*networkModule)(nil)
	_ arch.Module                      = (*sharingModule)(nil)
	_ arch.RouteRegistrar              = (*sharingModule)(nil)
	_ arch.Module                      = (*systemModule)(nil)
	_ arch.RouteRegistrar              = (*systemModule)(nil)
)
