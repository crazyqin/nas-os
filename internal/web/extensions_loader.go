package web

import (
	"context"
	"log"
	"net/http"
	"strings"

	"nas-os/internal/config"
	"nas-os/internal/extensions/activeprotect"
	"nas-os/internal/extensions/compliancescan"
	"nas-os/internal/extensions/deployorch"
	"nas-os/internal/extensions/netdiag"
	"nas-os/internal/packageruntime"

	"github.com/gin-gonic/gin"
)

// KnownExtensionNames are official HTTP extensions (packages.enabled).
// Derived from config.SystemPackageCatalog (ADR-0001 Stage 3).
// Default boot loads none. Recommended-system expansion does not auto-mount these.
var KnownExtensionNames = config.HTTPExtensionPackageIDs()

// extensionHolders keeps live manager instances for enabled extensions so enable
// is not a construct-and-discard no-op.
type extensionHolders struct {
	activeProtect  *activeprotect.Manager
	complianceScan *compliancescan.Scanner
	deployOrch     *deployorch.Orchestrator
	netDiag        *netdiag.Diagnoser
	names          []string
}

// registerConfiguredExtensions mounts HTTP routes for system extensions and
// discovers/loads third-party packages via the unified Package Runtime.
func (s *Server) registerConfiguredExtensions(api *gin.RouterGroup) {
	if s == nil || s.cfg == nil || api == nil {
		return
	}

	host := newConfigHost(s.cfg)
	rt := packageruntime.New(host, func(method, path string, h http.Handler) {
		api.Handle(method, path, gin.WrapH(h))
	})
	s.pkgRuntime = rt

	if s.extHolders == nil {
		s.extHolders = &extensionHolders{}
	}

	if err := s.registerSystemPackageCatalog(rt, api); err != nil {
		log.Printf("⚠️ system package catalog: %v", err)
		return
	}
	if err := s.registerRecommendedProductCatalog(rt); err != nil {
		log.Printf("⚠️ recommended product catalog: %v", err)
		return
	}

	// Third-party discovery (community/local): on-disk only; not SystemPackageCatalog.
	var discovered []packageruntime.DiskManifest
	if dir := s.cfg.CommunityDir(); dir != "" {
		var err error
		discovered, err = packageruntime.DiscoverDir(dir)
		if err != nil {
			log.Printf("⚠️ community discovery: %v", err)
		} else if len(discovered) > 0 {
			ids, err := rt.RegisterDiscovered(discovered)
			if err != nil {
				log.Printf("⚠️ community register: %v", err)
			} else {
				log.Printf("ℹ️  community packages discovered: %v", ids)
			}
		}
	}
	s.communityDiscovered = discovered

	// Boot enablement SSOT: app-center-enabled.json if present; else packages.enabled seed.
	// Default both empty → nothing loaded (Core-only package surface).
	if s.appCenterSSOTExists() {
		_ = s.loadPersistedRuntimeEnabled()
	} else {
		for _, id := range s.cfg.EnabledPackageNames() {
			s.addRuntimeEnabled(id)
		}
	}
	toEnable := intersectIDs(s.listRuntimeEnabled(), rt.CatalogIDs())
	if len(toEnable) > 0 {
		loaded, unknown, err := rt.Enable(context.Background(), toEnable)
		if err != nil {
			log.Printf("⚠️ package runtime enable: %v", err)
		}
		for _, u := range unknown {
			log.Printf("⚠️ package %q not in catalog", u)
		}
		s.extHolders.names = append([]string(nil), loaded...)
		for _, name := range loaded {
			log.Printf("✅ package enabled: %s", name)
		}
	}
	// Keep cfg.Packages.Enabled mirrored to SSOT set (may create file on first boot with seed).
	s.syncEnablementSSOT()

	s.mountPackageStatusRoutes(api, rt)
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, list := range [][]string{a, b} {
		for _, raw := range list {
			id := strings.ToLower(strings.TrimSpace(raw))
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// EnabledExtensions returns resolved known extension names (copy).
func (s *Server) EnabledExtensions() []string {
	if s == nil || s.cfg == nil {
		return nil
	}
	return s.cfg.EnabledNamedPackages(KnownExtensionNames)
}

// LoadedExtensionNames returns names successfully mounted this process.
func (s *Server) LoadedExtensionNames() []string {
	if s == nil {
		return nil
	}
	if s.pkgRuntime != nil {
		return s.pkgRuntime.LoadedIDs()
	}
	if s.extHolders == nil {
		return nil
	}
	out := make([]string, len(s.extHolders.names))
	copy(out, s.extHolders.names)
	return out
}

func intersectIDs(want, catalog []string) []string {
	if len(want) == 0 || len(catalog) == 0 {
		return nil
	}
	allow := make(map[string]struct{}, len(catalog))
	for _, id := range catalog {
		allow[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	var out []string
	seen := make(map[string]struct{})
	for _, raw := range want {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "" {
			continue
		}
		if _, ok := allow[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
