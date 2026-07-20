package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"nas-os/internal/config"
	"nas-os/internal/extensions/activeprotect"
	"nas-os/internal/extensions/agentworkflow"
	"nas-os/internal/extensions/aiguardrails"
	"nas-os/internal/extensions/compliancescan"
	"nas-os/internal/extensions/deployorch"
	"nas-os/internal/extensions/netdiag"
	"nas-os/internal/extensions/voicehub"
	"nas-os/internal/packageruntime"
	"nas-os/pkg/hostapi"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// registerSystemPackageCatalog registers every HTTP extension ID from
// config.SystemPackageCatalog into the Package Runtime. Mount implementations
// live in a table keyed by catalog ID; a catalog ID without a mount is an error
// (keeps catalog and runtime loadable set identical).
func (s *Server) registerSystemPackageCatalog(rt *packageruntime.Runtime, api *gin.RouterGroup) error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if api == nil {
		return fmt.Errorf("api group is nil")
	}

	mounts := s.httpExtensionMounts(api)
	catalogIDs := config.HTTPExtensionPackageIDs()
	if len(catalogIDs) == 0 {
		return fmt.Errorf("SystemPackageCatalog has no HTTP extensions")
	}

	// Fail fast if mount table drifts from catalog (either direction).
	for _, id := range catalogIDs {
		if _, ok := mounts[id]; !ok {
			return fmt.Errorf("catalog HTTP extension %q has no mount implementation", id)
		}
	}
	for id := range mounts {
		if !config.IsCatalogedSystemPackage(id) {
			return fmt.Errorf("mount implementation %q is not in SystemPackageCatalog", id)
		}
		entry, _ := config.LookupSystemPackage(id)
		if entry.Kind != config.KindHTTPExtension {
			return fmt.Errorf("mount implementation %q is not KindHTTPExtension", id)
		}
	}

	for _, id := range catalogIDs {
		entry, ok := config.LookupSystemPackage(id)
		if !ok {
			return fmt.Errorf("catalog lookup failed for %q", id)
		}
		mount := mounts[id]
		meta := hostapi.Meta{
			ID:          entry.ID,
			Trust:       hostapi.TrustSystem,
			Description: entry.Description,
			Version:     "1",
		}
		// Capture for factory closure.
		m := mount
		md := meta
		srv := s
		if err := rt.Register(md, func(hostapi.Host) (hostapi.Package, error) {
			// New instance per Enable; Server.httpMounted makes remounts no-ops.
			return &systemHTTPPackage{meta: md, mount: m, server: srv}, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// httpExtensionMounts returns mount callbacks for each official HTTP extension.
// Keys MUST stay aligned with config.HTTPExtensionPackageIDs().
func (s *Server) httpExtensionMounts(api *gin.RouterGroup) map[string]func() {
	// Each mount group uses requirePackageActive: unmounted → 404, disabled → 503.
	return map[string]func(){
		"agentworkflow": func() {
			g := api.Group("/")
			g.Use(s.requirePackageActive("agentworkflow"))
			agentworkflow.NewHandler(agentworkflow.NewService()).RegisterRoutes(g)
		},
		"aiguardrails": func() {
			g := api.Group("/")
			g.Use(s.requirePackageActive("aiguardrails"))
			aiguardrails.NewHandlers(aiguardrails.NewService()).RegisterRoutes(g)
		},
		"voicehub": func() {
			var logger *zap.Logger
			if s.logger != nil {
				logger = s.logger
			} else {
				logger = zap.NewNop()
			}
			g := api.Group("/")
			g.Use(s.requirePackageActive("voicehub"))
			voicehub.NewHandlers(voicehub.NewManager(logger, nil)).RegisterRoutes(g)
		},
		"activeprotect": func() {
			m := activeprotect.NewManager()
			if s.extHolders != nil {
				s.extHolders.activeProtect = m
			}
			g := api.Group("/activeprotect")
			g.Use(s.requirePackageActive("activeprotect"))
			g.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.GetStatus()})
			})
			g.GET("/tasks", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.ListTasks("")})
			})
			g.GET("/templates", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.ListTemplates("")})
			})
		},
		"compliancescan": func() {
			sc := compliancescan.NewScanner()
			if s.extHolders != nil {
				s.extHolders.complianceScan = sc
			}
			g := api.Group("/compliancescan")
			g.Use(s.requirePackageActive("compliancescan"))
			g.GET("/standards", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": sc.ListStandards()})
			})
			g.POST("/run/:standard", func(c *gin.Context) {
				std := compliancescan.Standard(c.Param("standard"))
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": sc.RunScan(std)})
			})
			g.POST("/run-all", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": sc.RunAllScans()})
			})
		},
		"deployorch": func() {
			o := deployorch.NewOrchestrator()
			if s.extHolders != nil {
				s.extHolders.deployOrch = o
			}
			g := api.Group("/deployorch")
			g.Use(s.requirePackageActive("deployorch"))
			g.GET("/nodes", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.GetNodes()})
			})
			g.GET("/deployments", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.ListDeployments()})
			})
		},
		"netdiag": func() {
			d := netdiag.NewDiagnoser()
			if s.extHolders != nil {
				s.extHolders.netDiag = d
			}
			g := api.Group("/netdiag")
			g.Use(s.requirePackageActive("netdiag"))
			g.POST("/full", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.RunFullDiagnosis()})
			})
			g.GET("/history", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.GetHistory(20)})
			})
		},
	}
}

// MountTableHTTPExtensionIDs returns sorted IDs present in the mount table
// (without registering). Used by structure consistency tests.
func MountTableHTTPExtensionIDs() []string {
	// Build mounts with a throwaway group is not needed — keys only.
	s := &Server{}
	// Use a dummy group: mounts close over api but we only need map keys.
	// Creating nil group is unsafe if called; only read keys from a local map builder.
	// Replicate key set via empty gin engine for safety.
	r := gin.New()
	api := r.Group("/")
	m := s.httpExtensionMounts(api)
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// systemHTTPPackage is a TrustSystem package that mounts gin routes via MountHTTP.
// Gin route registration is process-global per Server; Disable cannot unregister
// routes, so remount after re-Enable must be skipped via Server.httpMounted.
type systemHTTPPackage struct {
	meta   hostapi.Meta
	mount  func()
	server *Server
}

func (p *systemHTTPPackage) Meta() hostapi.Meta                       { return p.meta }
func (p *systemHTTPPackage) Init(context.Context, hostapi.Host) error { return nil }
func (p *systemHTTPPackage) Start(context.Context) error              { return nil }
func (p *systemHTTPPackage) Stop(context.Context) error               { return nil }
func (p *systemHTTPPackage) Health(context.Context) error             { return nil }

func (p *systemHTTPPackage) MountHTTP(_ func(method, path string, h http.Handler)) error {
	if p.mount == nil {
		return fmt.Errorf("package %s: nil mount", p.meta.ID)
	}
	if p.server != nil && p.server.httpRoutesMounted(p.meta.ID) {
		// Tree nodes already registered — only remount (serve) the package.
		p.server.mountPackageRoutes(p.meta.ID)
		return nil
	}
	p.mount()
	if p.server != nil {
		p.server.markHTTPRoutesMounted(p.meta.ID)
		p.server.mountPackageRoutes(p.meta.ID)
	}
	return nil
}

func (s *Server) httpRoutesMounted(id string) bool {
	if s == nil {
		return false
	}
	s.httpMountedMu.Lock()
	defer s.httpMountedMu.Unlock()
	_, ok := s.httpMounted[id]
	return ok
}

func (s *Server) markHTTPRoutesMounted(id string) {
	if s == nil {
		return
	}
	s.httpMountedMu.Lock()
	defer s.httpMountedMu.Unlock()
	if s.httpMounted == nil {
		s.httpMounted = make(map[string]struct{})
	}
	s.httpMounted[id] = struct{}{}
}
