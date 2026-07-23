//go:build nasd_full

package web

import (
	"log"
	"net/http"
	"path/filepath"

	"nas-os/internal/ai"
	"nas-os/internal/backup"
	"nas-os/internal/cloudsync"
	"nas-os/internal/cluster"
	"nas-os/internal/docker"
	"nas-os/internal/downloader"
	"nas-os/internal/photos"
	"nas-os/internal/vm"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// releaseProductManager drops product managers on disable.
// True unload path: unmount routes (404) → Runtime.Disable → here (free managers).
// Gin tree nodes remain (framework limit); managers are nil'd so memory can be reclaimed.
func (s *Server) releaseProductManager(id string) {
	if s == nil {
		return
	}
	// Drop registry slot first (teardown is owned by the switch below — avoid double Close).
	if s.productReg != nil {
		s.productReg.drop(id)
	}
	switch id {
	case "docker":
		s.setHolder("dockerMgr", nil)
		s.setHolder("appStore", nil)
		log.Printf("ℹ️  product docker manager released (memory reclaim)")
	case "photos":
		if s.hasHolder("photosAIMgr") {
			holderAs[*photos.AIManager](s, "photosAIMgr").Close()
			s.setHolder("photosAIMgr", nil)
		}
		s.setHolder("photosMgr", nil)
		log.Printf("ℹ️  product photos manager released (memory reclaim)")
	case "backup":
		s.setHolder("backupMgr", nil)
		s.setHolder("syncMgr", nil)
		log.Printf("ℹ️  product backup manager released (memory reclaim)")
	case "vm":
		s.setHolder("vmMgr", nil)
		s.setHolder("isoMgr", nil)
		s.setHolder("snapshotMgr", nil)
		log.Printf("ℹ️  product vm manager released (memory reclaim)")
	case "ai":
		s.setHolder("aiSvc", nil)
		log.Printf("ℹ️  product ai service released (memory reclaim)")
	case "cloudsync":
		s.setHolder("cloudsyncMgr", nil)
		log.Printf("ℹ️  product cloudsync manager released (memory reclaim)")
	case "downloader":
		if m, ok := holderAs[any](s, "downloadMgr").(*downloader.Manager); ok && m != nil {
			m.Close()
		}
		s.setHolder("downloadMgr", nil)
		log.Printf("ℹ️  product downloader manager released (memory reclaim)")
	case "cluster":
		s.clusterMu.Lock()
		svc := s.clusterServices
		s.clusterServices = nil
		s.clusterMu.Unlock()
		if svc != nil {
			if cs, ok := svc.(*cluster.Services); ok {
				if err := cluster.ShutdownCluster(cs); err != nil {
					log.Printf("⚠️ cluster shutdown: %v", err)
				} else {
					log.Printf("✅ product cluster shut down in-process")
				}
			}
		} else {
			log.Printf("ℹ️  product cluster disable persisted (was not running in this process)")
		}
	}
	// Allow re-register of HTTP routes on next enable (handlers re-bind to new managers).
	s.productRoutesMu.Lock()
	delete(s.productRoutesRegistered, id)
	s.productRoutesMu.Unlock()
}

// trackProduct registers a live product manager in the lifecycle registry.
func (s *Server) trackProduct(id string, holder any, stop func()) {
	if s == nil || holder == nil {
		return
	}
	if s.productReg == nil {
		s.productReg = newProductRegistry()
	}
	s.productReg.put(id, holder, stop)
}

// seedProductRegistry registers managers constructed at NewServer boot into the registry.
func (s *Server) seedProductRegistry() {
	if s == nil {
		return
	}
	if s.productReg == nil {
		s.productReg = newProductRegistry()
	}
	if s.hasHolder("dockerMgr") {
		s.trackProduct("docker", holderAs[*docker.Manager](s, "dockerMgr"), nil)
	}
	if s.hasHolder("photosMgr") {
		s.trackProduct("photos", holderAs[*photos.Manager](s, "photosMgr"), func() {
			if s.hasHolder("photosAIMgr") {
				holderAs[*photos.AIManager](s, "photosAIMgr").Close()
			}
		})
	}
	if s.hasHolder("backupMgr") {
		s.trackProduct("backup", holderAs[*backup.Manager](s, "backupMgr"), nil)
	}
	if s.hasHolder("vmMgr") {
		s.trackProduct("vm", holderAs[*vm.Manager](s, "vmMgr"), nil)
	}
	if s.hasHolder("aiSvc") {
		s.trackProduct("ai", holderAs[*ai.AIService](s, "aiSvc"), nil)
	}
	if s.hasHolder("cloudsyncMgr") {
		s.trackProduct("cloudsync", holderAs[*cloudsync.Manager](s, "cloudsyncMgr"), nil)
	}
	if s.hasHolder("downloadMgr") {
		s.trackProduct("downloader", holderAs[any](s, "downloadMgr"), func() {
			if m, ok := holderAs[any](s, "downloadMgr").(*downloader.Manager); ok && m != nil {
				m.Close()
			}
		})
	}
	s.clusterMu.Lock()
	cs := s.clusterServices
	s.clusterMu.Unlock()
	if cs != nil {
		s.trackProduct("cluster", cs, nil)
	}
}

// ensureProductManager lazily constructs managers when a product is runtime-enabled.
func (s *Server) ensureProductManager(id string) {
	if s == nil || s.cfg == nil {
		return
	}
	switch id {
	case "docker":
		if s.hasHolder("dockerMgr") {
			return
		}
		mgr, err := docker.NewManager()
		if err != nil {
			log.Printf("⚠️ runtime enable docker: %v", err)
			return
		}
		s.setHolder("dockerMgr", mgr)
		if !s.hasHolder("appStore") {
			store, err := docker.NewAppStore(mgr, "/opt/nas")
			if err == nil {
				s.setHolder("appStore", store)
			}
		}
		s.trackProduct("docker", mgr, nil)
		log.Printf("✅ product docker manager constructed")
	case "photos":
		if s.hasHolder("photosMgr") {
			return
		}
		s.setHolder("photosMgr", photos.NewManager(s.cfg.DataPath("photos")))
		if s.hasHolder("photosMgr") {
			aiMgr, err := photos.NewAIManager(holderAs[*photos.Manager](s, "photosMgr"), s.cfg.DataPath("photos", "models"))
			if err != nil {
				log.Printf("⚠️ photos AI manager: %v", err)
			} else {
				s.setHolder("photosAIMgr", aiMgr)
			}
		}
		s.trackProduct("photos", holderAs[*photos.Manager](s, "photosMgr"), func() {
			if s.hasHolder("photosAIMgr") {
				holderAs[*photos.AIManager](s, "photosAIMgr").Close()
			}
		})
		log.Printf("✅ product photos manager constructed")
	case "backup":
		if s.hasHolder("backupMgr") {
			return
		}
		s.setHolder("backupMgr", backup.NewManager(s.cfg.ConfigPath("backup-config.json"), s.cfg.MountPath("backups")))
		if err := holderAs[*backup.Manager](s, "backupMgr").Initialize(); err != nil {
			log.Printf("⚠️ backup init: %v", err)
		}
		if !s.hasHolder("syncMgr") {
			s.setHolder("syncMgr", backup.NewSyncManager(s.cfg.MountPath("backups")))
		}
		s.trackProduct("backup", holderAs[*backup.Manager](s, "backupMgr"), nil)
		log.Printf("✅ product backup manager constructed")
	case "vm":
		if s.hasHolder("vmMgr") {
			return
		}
		vmStoragePath := s.cfg.MountPath("vms")
		vmLogger := zap.NewNop()
		mgr, err := vm.NewManager(vmStoragePath, vmLogger)
		if err != nil {
			log.Printf("⚠️ runtime enable vm: %v", err)
			return
		}
		s.setHolder("vmMgr", mgr)
		if !s.hasHolder("isoMgr") {
			iso, err := vm.NewISOManager(s.cfg.MountPath("isos"), vmLogger)
			if err != nil {
				log.Printf("⚠️ iso manager: %v", err)
			} else {
				s.setHolder("isoMgr", iso)
			}
		}
		if !s.hasHolder("snapshotMgr") && s.hasHolder("vmMgr") {
			snap, err := vm.NewSnapshotManager(vmStoragePath, holderAs[*vm.Manager](s, "vmMgr"), vmLogger)
			if err != nil {
				log.Printf("⚠️ snapshot manager: %v", err)
			} else {
				s.setHolder("snapshotMgr", snap)
			}
		}
		s.trackProduct("vm", holderAs[*vm.Manager](s, "vmMgr"), nil)
		log.Printf("✅ product vm manager constructed")
	case "ai":
		if s.hasHolder("aiSvc") {
			return
		}
		svc, err := ai.NewAIService(nil)
		if err != nil {
			log.Printf("⚠️ runtime enable ai: %v", err)
			return
		}
		s.setHolder("aiSvc", svc)
		s.trackProduct("ai", svc, nil)
		log.Printf("✅ product ai service constructed")
	case "cloudsync":
		if s.hasHolder("cloudsyncMgr") {
			return
		}
		s.setHolder("cloudsyncMgr", cloudsync.NewManager(s.cfg.ConfigPath("cloudsync-config.json")))
		if err := holderAs[*cloudsync.Manager](s, "cloudsyncMgr").Initialize(); err != nil {
			log.Printf("⚠️ cloudsync init: %v", err)
		}
		s.trackProduct("cloudsync", holderAs[*cloudsync.Manager](s, "cloudsyncMgr"), nil)
		log.Printf("✅ product cloudsync manager constructed")
	case "downloader":
		if s.hasHolder("downloadMgr") {
			return
		}
		logger := s.logger
		if logger == nil {
			logger = zap.NewNop()
		}
		mgr, err := downloader.NewManager(filepath.Join(s.cfg.Paths.DataDir, "downloads"), logger)
		if err != nil {
			log.Printf("⚠️ runtime enable downloader: %v", err)
			return
		}
		s.setHolder("downloadMgr", mgr)
		s.trackProduct("downloader", mgr, func() {
			if m, ok := holderAs[any](s, "downloadMgr").(*downloader.Manager); ok && m != nil {
				m.Close()
			}
		})
		log.Printf("✅ product downloader manager constructed")
	case "cluster":
		s.clusterMu.Lock()
		if s.clusterServices != nil {
			s.clusterMu.Unlock()
			log.Printf("✅ product cluster already running")
			return
		}
		boot := s.clusterBootstrap
		s.clusterMu.Unlock()
		if boot == nil {
			log.Printf("⚠️  cluster bootstrap not wired; enable persisted — restart process to start cluster")
			return
		}
		svc, err := boot()
		if err != nil {
			log.Printf("⚠️ runtime enable cluster: %v", err)
			return
		}
		if svc == nil {
			log.Printf("⚠️ cluster bootstrap returned nil services")
			return
		}
		s.clusterMu.Lock()
		s.clusterServices = svc
		s.clusterMu.Unlock()
		s.trackProduct("cluster", svc, nil)
		log.Printf("✅ product cluster started in-process (runtime enable)")
	default:
		log.Printf("ℹ️  product %s active via package runtime", id)
	}
}

// registerProductRoutes registers HTTP routes for a product once (safe after runtime enable).
func (s *Server) registerProductRoutes(id string) {
	if s == nil || s.adminAPI == nil {
		return
	}
	s.productRoutesMu.Lock()
	defer s.productRoutesMu.Unlock()
	if s.productRoutesRegistered == nil {
		s.productRoutesRegistered = make(map[string]struct{})
	}
	if _, ok := s.productRoutesRegistered[id]; ok {
		return
	}

	api := s.adminAPI
	switch id {
	case "docker":
		if !s.hasHolder("dockerMgr") {
			return
		}
		dg := api.Group("/")
		dg.Use(s.requirePackageActive("docker"))
		docker.NewHandlers(holderAs[*docker.Manager](s, "dockerMgr")).RegisterRoutes(dg)
		if s.hasHolder("appStore") {
			ag := api.Group("/")
			ag.Use(s.requirePackageActive("docker"))
			docker.NewAppHandlers(holderAs[*docker.AppStore](s, "appStore")).RegisterRoutes(ag)
		}
	case "photos":
		if !s.hasHolder("photosMgr") {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("photos"))
		photos.NewHandler(holderAs[*photos.Manager](s, "photosMgr")).RegisterRoutes(g)
	case "backup":
		if !s.hasHolder("backupMgr") {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("backup"))
		backup.NewHandlers(holderAs[*backup.Manager](s, "backupMgr"), holderAs[*backup.SyncManager](s, "syncMgr")).RegisterRoutes(g)
	case "vm":
		if !s.hasHolder("vmMgr") || !s.hasHolder("isoMgr") {
			return
		}
		vmHandler := vm.NewHandler(holderAs[*vm.Manager](s, "vmMgr"), holderAs[*vm.ISOManager](s, "isoMgr"), holderAs[*vm.SnapshotManager](s, "snapshotMgr"), zap.NewNop())
		g := api.Group("/")
		g.Use(s.requirePackageActive("vm"))
		g.GET("/vms", func(c *gin.Context) { vmHandler.HandleListVMs(c.Writer, c.Request) })
		g.POST("/vms", func(c *gin.Context) { vmHandler.HandleCreateVM(c.Writer, c.Request) })
		g.GET("/vms/:id", func(c *gin.Context) { vmHandler.HandleVM(c.Writer, c.Request) })
		g.POST("/vms/:id", func(c *gin.Context) { vmHandler.HandleVM(c.Writer, c.Request) })
		g.DELETE("/vms/:id", func(c *gin.Context) { vmHandler.HandleVM(c.Writer, c.Request) })
		g.PUT("/vms/:id", func(c *gin.Context) { vmHandler.HandleVM(c.Writer, c.Request) })
		g.GET("/vm-isos", func(c *gin.Context) { vmHandler.HandleListISOs(c.Writer, c.Request) })
		g.GET("/vm-isos/:id", func(c *gin.Context) { vmHandler.HandleISO(c.Writer, c.Request) })
		g.POST("/vm-isos/:id", func(c *gin.Context) { vmHandler.HandleISO(c.Writer, c.Request) })
		g.DELETE("/vm-isos/:id", func(c *gin.Context) { vmHandler.HandleISO(c.Writer, c.Request) })
		g.GET("/vm-snapshots", func(c *gin.Context) { vmHandler.HandleListSnapshots(c.Writer, c.Request) })
		g.GET("/vm-snapshots/:id", func(c *gin.Context) { vmHandler.HandleSnapshot(c.Writer, c.Request) })
		g.POST("/vm-snapshots/:id", func(c *gin.Context) { vmHandler.HandleSnapshot(c.Writer, c.Request) })
		g.DELETE("/vm-snapshots/:id", func(c *gin.Context) { vmHandler.HandleSnapshot(c.Writer, c.Request) })
		g.GET("/vm-templates", func(c *gin.Context) { vmHandler.HandleListTemplates(c.Writer, c.Request) })
		g.GET("/vm-usb-devices", func(c *gin.Context) { vmHandler.HandleUSBDevices(c.Writer, c.Request) })
		g.GET("/vm-pci-devices", func(c *gin.Context) { vmHandler.HandlePCIDevices(c.Writer, c.Request) })
	case "ai":
		if !s.hasHolder("aiSvc") {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("ai"))
		if gateway := holderAs[*ai.AIService](s, "aiSvc").GetGateway(); gateway != nil {
			ai.NewGatewayHandlers(gateway, holderAs[*ai.AIService](s, "aiSvc").GetModelManager()).RegisterRoutes(g)
		} else {
			g.GET("/ai/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"active": true}})
			})
		}
	case "cloudsync":
		if !s.hasHolder("cloudsyncMgr") {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("cloudsync"))
		cloudsync.NewHandlers(holderAs[*cloudsync.Manager](s, "cloudsyncMgr")).RegisterRoutes(g)
	case "downloader":
		mgr, ok := holderAs[any](s, "downloadMgr").(*downloader.Manager)
		if !ok || mgr == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("downloader"))
		downloader.NewHandler(mgr).RegisterRoutes(g)
	default:
		return
	}
	s.productRoutesRegistered[id] = struct{}{}
	log.Printf("✅ product routes registered: %s", id)
}

