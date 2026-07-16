package web

import (
	"log"
	"net/http"
	"strings"

	"nas-os/internal/extensions/activeprotect"
	"nas-os/internal/extensions/agentworkflow"
	"nas-os/internal/extensions/aiguardrails"
	"nas-os/internal/extensions/compliancescan"
	"nas-os/internal/extensions/deployorch"
	"nas-os/internal/extensions/netdiag"
	"nas-os/internal/extensions/voicehub"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// KnownExtensionNames are optional product extensions loadable when listed in
// config modules.extensions. Default boot loads none.
var KnownExtensionNames = []string{
	"activeprotect",
	"agentworkflow",
	"aiguardrails",
	"compliancescan",
	"deployorch",
	"netdiag",
	"voicehub",
}

// extensionHolders keeps live manager instances for enabled extensions so enable
// is not a construct-and-discard no-op.
type extensionHolders struct {
	activeProtect  *activeprotect.Manager
	complianceScan *compliancescan.Scanner
	deployOrch     *deployorch.Orchestrator
	netDiag        *netdiag.Diagnoser
	names          []string
}

// registerConfiguredExtensions mounts HTTP routes for extensions enabled in config.
// Unknown names are logged and skipped. Empty config means no extension routes.
func (s *Server) registerConfiguredExtensions(api *gin.RouterGroup) {
	if s == nil || s.cfg == nil || api == nil {
		return
	}
	enabled := s.cfg.Modules.Extensions
	if len(enabled) == 0 {
		return
	}

	if s.extHolders == nil {
		s.extHolders = &extensionHolders{}
	}

	seen := make(map[string]bool, len(enabled))
	for _, raw := range enabled {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if err := s.mountExtension(api, name); err != nil {
			log.Printf("⚠️ extension %q not mounted: %v", name, err)
			continue
		}
		s.extHolders.names = append(s.extHolders.names, name)
		log.Printf("✅ extension enabled: %s", name)
	}

	// Catalog of currently loaded extensions (always useful when any enabled).
	api.GET("/extensions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"enabled": s.EnabledExtensions(),
				"known":   KnownExtensionNames,
			},
		})
	})
}

func (s *Server) mountExtension(api *gin.RouterGroup, name string) error {
	switch name {
	case "agentworkflow":
		agentworkflow.NewHandler(agentworkflow.NewService()).RegisterRoutes(api)
		return nil
	case "aiguardrails":
		aiguardrails.NewHandlers(aiguardrails.NewService()).RegisterRoutes(api)
		return nil
	case "voicehub":
		var logger *zap.Logger
		if s.logger != nil {
			logger = s.logger
		} else {
			logger = zap.NewNop()
		}
		voicehub.NewHandlers(voicehub.NewManager(logger, nil)).RegisterRoutes(api)
		return nil
	case "activeprotect":
		m := activeprotect.NewManager()
		s.extHolders.activeProtect = m
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
		return nil
	case "compliancescan":
		sc := compliancescan.NewScanner()
		s.extHolders.complianceScan = sc
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
		return nil
	case "deployorch":
		o := deployorch.NewOrchestrator()
		s.extHolders.deployOrch = o
		g := api.Group("/deployorch")
		g.GET("/nodes", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.GetNodes()})
		})
		g.GET("/deployments", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": o.ListDeployments()})
		})
		return nil
	case "netdiag":
		d := netdiag.NewDiagnoser()
		s.extHolders.netDiag = d
		g := api.Group("/netdiag")
		g.POST("/full", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.RunFullDiagnosis()})
		})
		g.GET("/history", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": d.GetHistory(20)})
		})
		return nil
	default:
		return errUnknownExtension(name)
	}
}

type unknownExtensionError string

func (e unknownExtensionError) Error() string { return "unknown extension: " + string(e) }

func errUnknownExtension(name string) error { return unknownExtensionError(name) }

// EnabledExtensions returns the configured extension name list (copy).
func (s *Server) EnabledExtensions() []string {
	if s == nil || s.cfg == nil {
		return nil
	}
	out := make([]string, len(s.cfg.Modules.Extensions))
	copy(out, s.cfg.Modules.Extensions)
	return out
}

// LoadedExtensionNames returns names successfully mounted this process (may be empty).
func (s *Server) LoadedExtensionNames() []string {
	if s == nil || s.extHolders == nil {
		return nil
	}
	out := make([]string, len(s.extHolders.names))
	copy(out, s.extHolders.names)
	return out
}
