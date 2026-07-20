package web

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"nas-os/internal/config"
	"nas-os/internal/packageruntime"
	"nas-os/pkg/hostapi"

	"github.com/gin-gonic/gin"
)

// SetClusterServices attaches process-level cluster services (from application).
// Typed as any so Core builds do not link the cluster package.
func (s *Server) SetClusterServices(svc any) {
	if s == nil {
		return
	}
	s.clusterMu.Lock()
	defer s.clusterMu.Unlock()
	s.clusterServices = svc
}

// SetClusterBootstrap registers a factory to start cluster at runtime enable.
// Factory returns any (typically *cluster.Services in full builds).
func (s *Server) SetClusterBootstrap(fn func() (any, error)) {
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
// Core builds (!ProductsLinked) skip registration so Known/Operable stay false.
func (s *Server) registerRecommendedProductCatalog(rt *packageruntime.Runtime) error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if !ProductsLinked() {
		log.Println("ℹ️  core build: recommended products not registered in Runtime (non-operable; need -tags nasd_full)")
		return nil
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
		p.server.mountPackageRoutes(p.meta.ID)
	}
	return nil
}
func (p *productGatePackage) Stop(context.Context) error {
	if p.server != nil {
		p.server.unmountPackageRoutes(p.meta.ID)
		p.server.releaseProductManager(p.meta.ID)
	}
	return nil
}
func (p *productGatePackage) Health(context.Context) error { return nil }

// IsPackageActive reports whether id is loaded in the package runtime.
func (s *Server) IsPackageActive(id string) bool {
	if s == nil || s.pkgRuntime == nil {
		return false
	}
	return s.pkgRuntime.IsLoaded(id)
}

// mountPackageRoutes marks package HTTP routes as mounted (requests served).
func (s *Server) mountPackageRoutes(id string) {
	if s == nil || id == "" {
		return
	}
	s.packageMountMu.Lock()
	defer s.packageMountMu.Unlock()
	if s.packageMounted == nil {
		s.packageMounted = make(map[string]struct{})
	}
	s.packageMounted[id] = struct{}{}
	log.Printf("✅ package routes mounted: %s", id)
}

// unmountPackageRoutes marks package HTTP routes as unmounted (requests → 404).
func (s *Server) unmountPackageRoutes(id string) {
	if s == nil || id == "" {
		return
	}
	s.packageMountMu.Lock()
	defer s.packageMountMu.Unlock()
	delete(s.packageMounted, id)
	log.Printf("ℹ️  package routes unmounted: %s", id)
}

// isPackageMounted reports whether package routes are currently mounted.
func (s *Server) isPackageMounted(id string) bool {
	if s == nil {
		return false
	}
	s.packageMountMu.RLock()
	defer s.packageMountMu.RUnlock()
	_, ok := s.packageMounted[id]
	return ok
}

// requirePackageActive enforces true unload: unmounted → 404; mounted but not
// loaded → 503; mounted+loaded → handler.
func (s *Server) requirePackageActive(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || !s.isPackageMounted(id) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "package routes unmounted: " + id,
			})
			c.Abort()
			return
		}
		if !s.IsPackageActive(id) {
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
