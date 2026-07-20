package web

import (
	"context"
	"fmt"
	"net/http"

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

// registerSystemPackageCatalog registers official HTTP extensions as TrustSystem
// packages. api is closed over so MountHTTP can attach gin routes (Stage 2).
func (s *Server) registerSystemPackageCatalog(rt *packageruntime.Runtime, api *gin.RouterGroup) error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if api == nil {
		return fmt.Errorf("api group is nil")
	}

	type spec struct {
		id, desc string
		mount    func()
	}

	specs := []spec{
		{"agentworkflow", "Agent workflow automation", func() {
			agentworkflow.NewHandler(agentworkflow.NewService()).RegisterRoutes(api)
		}},
		{"aiguardrails", "AI guardrails", func() {
			aiguardrails.NewHandlers(aiguardrails.NewService()).RegisterRoutes(api)
		}},
		{"voicehub", "Voice hub", func() {
			var logger *zap.Logger
			if s.logger != nil {
				logger = s.logger
			} else {
				logger = zap.NewNop()
			}
			voicehub.NewHandlers(voicehub.NewManager(logger, nil)).RegisterRoutes(api)
		}},
		{"activeprotect", "Active protect", func() {
			m := activeprotect.NewManager()
			if s.extHolders != nil {
				s.extHolders.activeProtect = m
			}
			g := api.Group("/activeprotect")
			g.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.GetStatus()})
			})
			g.GET("/tasks", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.ListTasks("")})
			})
			g.GET("/templates", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": m.ListTemplates("")})
			})
		}},
		{"compliancescan", "Compliance scan", func() {
			sc := compliancescan.NewScanner()
			if s.extHolders != nil {
				s.extHolders.complianceScan = sc
			}
			g := api.Group("/compliancescan")
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
		}},
		{"deployorch", "Deploy orchestrator", func() {
			o := deployorch.NewOrchestrator()
			if s.extHolders != nil {
				s.extHolders.deployOrch = o
			}
			g := api.Group("/deployorch")
			g.GET("/nodes", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.GetNodes()})
			})
			g.GET("/deployments", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.ListDeployments()})
			})
		}},
		{"netdiag", "Network diagnostics", func() {
			d := netdiag.NewDiagnoser()
			if s.extHolders != nil {
				s.extHolders.netDiag = d
			}
			g := api.Group("/netdiag")
			g.POST("/full", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.RunFullDiagnosis()})
			})
			g.GET("/history", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.GetHistory(20)})
			})
		}},
	}

	for _, sp := range specs {
		// Capture loop variables.
		id, desc, mount := sp.id, sp.desc, sp.mount
		meta := hostapi.Meta{
			ID:          id,
			Trust:       hostapi.TrustSystem,
			Description: desc,
			Version:     "1",
		}
		err := rt.Register(meta, func(hostapi.Host) (hostapi.Package, error) {
			return &systemHTTPPackage{meta: meta, mount: mount}, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// systemHTTPPackage is a TrustSystem package that mounts gin routes on Start/MountHTTP.
type systemHTTPPackage struct {
	meta   hostapi.Meta
	mount  func()
	mounted bool
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
	if p.mounted {
		return nil
	}
	p.mount()
	p.mounted = true
	return nil
}
