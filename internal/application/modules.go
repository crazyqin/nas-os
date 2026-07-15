package application

import (
	"context"
	"fmt"

	"nas-os/internal/arch"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"

	"go.uber.org/zap"
)

const (
	moduleIdentity = "identity"
	moduleStorage  = "storage"
	moduleNetwork  = "network"
	moduleSharing  = "sharing"
	moduleSystem   = "system"
)

// registerCoreModules 将核心领域纳入统一生命周期和依赖图。
// 当前使用适配器渐进迁移，避免要求既有 Manager 一次性实现 arch.Module。
func registerCoreModules(
	container *arch.Container,
	userMgr *users.Manager,
	storageMgr *storage.Manager,
	networkMgr *network.Manager,
	smbMgr *smb.Manager,
	nfsMgr *nfs.Manager,
	logger *zap.Logger,
) error {
	modules := []arch.Module{
		arch.NewModuleAdapter(moduleIdentity, nil, logger).
			WithInit(requireService("users manager", userMgr)),
		arch.NewModuleAdapter(moduleStorage, nil, logger).
			WithInit(requireService("storage manager", storageMgr)),
		arch.NewModuleAdapter(moduleNetwork, nil, logger).
			WithInit(requireService("network manager", networkMgr)).
			WithStart(func(context.Context) error {
				networkMgr.StartDDNSWorker()
				return nil
			}).
			WithStop(func(context.Context) error {
				networkMgr.StopDDNSWorker()
				return nil
			}),
		arch.NewModuleAdapter(moduleSharing, []string{moduleIdentity, moduleStorage, moduleNetwork}, logger).
			WithInit(func(context.Context) error {
				if smbMgr == nil || nfsMgr == nil {
					return fmt.Errorf("SMB and NFS managers are required")
				}
				return nil
			}),
		arch.NewModuleAdapter(moduleSystem, []string{moduleIdentity, moduleStorage, moduleNetwork, moduleSharing}, logger),
	}

	for _, module := range modules {
		if err := container.RegisterModule(module); err != nil {
			return err
		}
	}

	container.Register(moduleIdentity, userMgr)
	container.Register(moduleStorage, storageMgr)
	container.Register(moduleNetwork, networkMgr)
	container.Register("sharing.smb", smbMgr)
	container.Register("sharing.nfs", nfsMgr)
	return nil
}

func requireService(name string, service interface{}) func(context.Context) error {
	return func(context.Context) error {
		if service == nil {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
}
