package web

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"nas-os/internal/config"
	"nas-os/internal/docker"
	"nas-os/internal/packageruntime"
	"nas-os/pkg/hostapi"

	"github.com/gin-gonic/gin"
)

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

// ensureProductManager lazily constructs managers when a product is runtime-enabled.
func (s *Server) ensureProductManager(id string) {
	if s == nil {
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
		log.Printf("✅ product docker manager constructed (runtime enable)")
	default:
		log.Printf("ℹ️  product %s active via package runtime (manager from boot if constructed)", id)
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
	// Merge app-center persistence so restart restores product managers.
	for _, id := range loadEnabledIDsFromFile(cfg.DataPath("app-center-enabled.json")) {
		if e, ok := config.LookupSystemPackage(id); ok && e.Kind == config.KindRecommendedProduct {
			want[id] = true
		}
	}
	return want
}
