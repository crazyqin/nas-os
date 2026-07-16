package web

import (
	"log"
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

	seen := make(map[string]bool, len(enabled))
	for _, raw := range enabled {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if err := s.mountExtension(api, name); err != nil {
			log.Printf("⚠️ extension %q not mounted: %v", name, err)
		} else {
			log.Printf("✅ extension enabled: %s", name)
		}
	}
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
		// Manager-only extension: expose readiness via side registry for ops/tests.
		_ = activeprotect.NewManager()
		return nil
	case "compliancescan":
		_ = compliancescan.NewScanner()
		return nil
	case "deployorch":
		_ = deployorch.NewOrchestrator()
		return nil
	case "netdiag":
		_ = netdiag.NewDiagnoser()
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
