package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"nas-os/internal/ai"
	"nas-os/internal/backup"
	"nas-os/internal/cloudsync"
	"nas-os/internal/cluster"
	"nas-os/internal/config"
	"nas-os/internal/docker"
	"nas-os/internal/downloader"
	"nas-os/internal/packageruntime"
	"nas-os/internal/photos"
	"nas-os/internal/vm"
	"nas-os/pkg/hostapi"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetClusterServices attaches process-level cluster services (from application).
func (s *Server) SetClusterServices(svc *cluster.Services) {
	if s == nil {
		return
	}
	s.clusterMu.Lock()
	defer s.clusterMu.Unlock()
	s.clusterServices = svc
}

// SetClusterBootstrap registers a factory to start cluster at runtime enable.
func (s *Server) SetClusterBootstrap(fn func() (*cluster.Services, error)) {
	if s == nil {
		return
	}
	s.clusterMu.Lock()
	defer s.clusterMu.Unlock()
	s.clusterBootstrap = fn
}

// ClusterRunning reports whether cluster services are live in this process.
func (s *Server) ClusterRunning() bool {
	if s == nil {
		return false
	}
	s.clusterMu.Lock()
	defer s.clusterMu.Unlock()
	return s.clusterServices != nil
}

// registerRecommendedProductCatalog registers catalog recommended_product IDs as
// TrustSystem packages operable via Application Center / packages API.
func (s *Server) registerRecommendedProductCatalog(rt *packageruntime.Runtime) error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	for _, e := range config.SystemPackageCatalog {
		if e.Kind != config.KindRecommendedProduct {
			continue
		}
		meta := hostapi.Meta{
			ID:          e.ID,
			Trust:       hostapi.TrustSystem,
			Description: e.Description + " (product surface)",
			Version:     "1",
		}
		md := meta
		srv := s
		if err := rt.Register(md, func(hostapi.Host) (hostapi.Package, error) {
			return &productGatePackage{meta: md, server: srv}, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// productGatePackage is a thin Runtime package for recommended product IDs.
type productGatePackage struct {
	meta   hostapi.Meta
	server *Server
}

func (p *productGatePackage) Meta() hostapi.Meta                       { return p.meta }
func (p *productGatePackage) Init(context.Context, hostapi.Host) error { return nil }
func (p *productGatePackage) Start(context.Context) error {
	if p.server != nil {
		p.server.ensureProductManager(p.meta.ID)
		p.server.registerProductRoutes(p.meta.ID)
	}
	return nil
}
func (p *productGatePackage) Stop(context.Context) error   { return nil }
func (p *productGatePackage) Health(context.Context) error { return nil }

// IsPackageActive reports whether id is loaded in the package runtime.
func (s *Server) IsPackageActive(id string) bool {
	if s == nil || s.pkgRuntime == nil {
		return false
	}
	return s.pkgRuntime.IsLoaded(id)
}

// requirePackageActive aborts with 503 when package is not loaded (disable semantics).
func (s *Server) requirePackageActive(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || !s.IsPackageActive(id) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    1,
				"message": "package disabled or not enabled: " + id,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// releaseProductManager drops product managers on disable (routes stay gated by IsPackageActive).
func (s *Server) releaseProductManager(id string) {
	if s == nil {
		return
	}
	switch id {
	case "docker":
		s.dockerMgr = nil
		s.appStore = nil
		log.Printf("ℹ️  product docker manager released")
	case "photos":
		if s.photosAIMgr != nil {
			s.photosAIMgr.Close()
			s.photosAIMgr = nil
		}
		s.photosMgr = nil
		log.Printf("ℹ️  product photos manager released")
	case "backup":
		s.backupMgr = nil
		s.syncMgr = nil
		log.Printf("ℹ️  product backup manager released")
	case "vm":
		s.vmMgr = nil
		s.isoMgr = nil
		s.snapshotMgr = nil
		log.Printf("ℹ️  product vm manager released")
	case "ai":
		s.aiSvc = nil
		log.Printf("ℹ️  product ai service released")
	case "cloudsync":
		s.cloudsyncMgr = nil
		log.Printf("ℹ️  product cloudsync manager released")
	case "downloader":
		if s.downloadMgr != nil {
			s.downloadMgr.Close()
		}
		s.downloadMgr = nil
		log.Printf("ℹ️  product downloader manager released")
	case "cluster":
		s.clusterMu.Lock()
		svc := s.clusterServices
		s.clusterServices = nil
		s.clusterMu.Unlock()
		if svc != nil {
			if err := cluster.ShutdownCluster(svc); err != nil {
				log.Printf("⚠️ cluster shutdown: %v", err)
			} else {
				log.Printf("✅ product cluster shut down in-process")
			}
		} else {
			log.Printf("ℹ️  product cluster disable persisted (was not running in this process)")
		}
	}
}

// ensureProductManager lazily constructs managers when a product is runtime-enabled.
func (s *Server) ensureProductManager(id string) {
	if s == nil || s.cfg == nil {
		return
	}
	switch id {
	case "docker":
		if s.dockerMgr != nil {
			return
		}
		mgr, err := docker.NewManager()
		if err != nil {
			log.Printf("⚠️ runtime enable docker: %v", err)
			return
		}
		s.dockerMgr = mgr
		if s.appStore == nil {
			store, err := docker.NewAppStore(mgr, "/opt/nas")
			if err == nil {
				s.appStore = store
			}
		}
		log.Printf("✅ product docker manager constructed")
	case "photos":
		if s.photosMgr != nil {
			return
		}
		s.photosMgr = photos.NewManager(s.cfg.DataPath("photos"))
		if s.photosMgr != nil {
			aiMgr, err := photos.NewAIManager(s.photosMgr, s.cfg.DataPath("photos", "models"))
			if err != nil {
				log.Printf("⚠️ photos AI manager: %v", err)
			} else {
				s.photosAIMgr = aiMgr
			}
		}
		log.Printf("✅ product photos manager constructed")
	case "backup":
		if s.backupMgr != nil {
			return
		}
		s.backupMgr = backup.NewManager(s.cfg.ConfigPath("backup-config.json"), s.cfg.MountPath("backups"))
		if err := s.backupMgr.Initialize(); err != nil {
			log.Printf("⚠️ backup init: %v", err)
		}
		if s.syncMgr == nil {
			s.syncMgr = backup.NewSyncManager(s.cfg.MountPath("backups"))
		}
		log.Printf("✅ product backup manager constructed")
	case "vm":
		if s.vmMgr != nil {
			return
		}
		vmStoragePath := s.cfg.MountPath("vms")
		vmLogger := zap.NewNop()
		mgr, err := vm.NewManager(vmStoragePath, vmLogger)
		if err != nil {
			log.Printf("⚠️ runtime enable vm: %v", err)
			return
		}
		s.vmMgr = mgr
		if s.isoMgr == nil {
			iso, err := vm.NewISOManager(s.cfg.MountPath("isos"), vmLogger)
			if err != nil {
				log.Printf("⚠️ iso manager: %v", err)
			} else {
				s.isoMgr = iso
			}
		}
		if s.snapshotMgr == nil && s.vmMgr != nil {
			snap, err := vm.NewSnapshotManager(vmStoragePath, s.vmMgr, vmLogger)
			if err != nil {
				log.Printf("⚠️ snapshot manager: %v", err)
			} else {
				s.snapshotMgr = snap
			}
		}
		log.Printf("✅ product vm manager constructed")
	case "ai":
		if s.aiSvc != nil {
			return
		}
		svc, err := ai.NewAIService(nil)
		if err != nil {
			log.Printf("⚠️ runtime enable ai: %v", err)
			return
		}
		s.aiSvc = svc
		log.Printf("✅ product ai service constructed")
	case "cloudsync":
		if s.cloudsyncMgr != nil {
			return
		}
		s.cloudsyncMgr = cloudsync.NewManager(s.cfg.ConfigPath("cloudsync-config.json"))
		if err := s.cloudsyncMgr.Initialize(); err != nil {
			log.Printf("⚠️ cloudsync init: %v", err)
		}
		log.Printf("✅ product cloudsync manager constructed")
	case "downloader":
		if s.downloadMgr != nil {
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
		s.downloadMgr = mgr
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
		if s.dockerMgr == nil {
			return
		}
		dg := api.Group("/")
		dg.Use(s.requirePackageActive("docker"))
		docker.NewHandlers(s.dockerMgr).RegisterRoutes(dg)
		if s.appStore != nil {
			ag := api.Group("/")
			ag.Use(s.requirePackageActive("docker"))
			docker.NewAppHandlers(s.appStore).RegisterRoutes(ag)
		}
	case "photos":
		if s.photosMgr == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("photos"))
		photos.NewHandler(s.photosMgr).RegisterRoutes(g)
	case "backup":
		if s.backupMgr == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("backup"))
		backup.NewHandlers(s.backupMgr, s.syncMgr).RegisterRoutes(g)
	case "vm":
		if s.vmMgr == nil || s.isoMgr == nil {
			return
		}
		vmHandler := vm.NewHandler(s.vmMgr, s.isoMgr, s.snapshotMgr, zap.NewNop())
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
		if s.aiSvc == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("ai"))
		if gateway := s.aiSvc.GetGateway(); gateway != nil {
			ai.NewGatewayHandlers(gateway, s.aiSvc.GetModelManager()).RegisterRoutes(g)
		} else {
			g.GET("/ai/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"active": true}})
			})
		}
	case "cloudsync":
		if s.cloudsyncMgr == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("cloudsync"))
		cloudsync.NewHandlers(s.cloudsyncMgr).RegisterRoutes(g)
	case "downloader":
		if s.downloadMgr == nil {
			return
		}
		g := api.Group("/")
		g.Use(s.requirePackageActive("downloader"))
		downloader.NewHandler(s.downloadMgr).RegisterRoutes(g)
	default:
		return
	}
	s.productRoutesRegistered[id] = struct{}{}
	log.Printf("✅ product routes registered: %s", id)
}

// bootWantProducts returns recommended product IDs to construct at NewServer time.
func bootWantProducts(cfg *config.Config) map[string]bool {
	want := make(map[string]bool)
	if cfg == nil {
		return want
	}
	for _, id := range cfg.BootProductIDs() {
		want[id] = true
	}
	return want
}

// productBulkSurface is the deprecated kitchen-sink optional surface.
// Only modules.optional (legacy) enables bulk companions (perf/quota/s3/…).
// packages.recommended_system enables the 8 catalog products only.
func productBulkSurface(cfg *config.Config) bool {
	return cfg != nil && cfg.Modules.Optional
}

