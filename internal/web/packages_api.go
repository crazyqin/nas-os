package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nas-os/internal/config"
	"nas-os/internal/packageruntime"
	"nas-os/pkg/hostapi"

	"github.com/gin-gonic/gin"
)

// packageItem is the Application Center list row shape.
type packageItem struct {
	ID          string `json:"id"`
	Trust       string `json:"trust"`
	Source      string `json:"source"` // system | community | product
	Kind        string `json:"kind"`   // http_extension | recommended_product | community
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Loaded      bool   `json:"loaded"`
	// Operable: can enable/disable through unified package runtime.
	Operable   bool `json:"operable"`
	CanEnable  bool `json:"can_enable"`
	CanDisable bool `json:"can_disable"`
	// RequiresRestart: enable persists but full process features need restart (e.g. cluster).
	RequiresRestart bool   `json:"requires_restart,omitempty"`
	Note            string `json:"note,omitempty"`
}

// runtimeEnabledMu guards s.runtimeEnabled (persisted user enable set).
// Stored on Server as map[string]struct{} in packages_state.go pattern — use Server fields.

func (s *Server) mountPackageStatusRoutes(api *gin.RouterGroup, rt *packageruntime.Runtime) {
	api.GET("/extensions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"enabled": s.EnabledExtensions(),
				"known":   KnownExtensionNames,
				"loaded":  rt.LoadedIDs(),
			},
		})
	})

	api.GET("/packages", s.handlePackagesList)
	api.POST("/packages/:id/enable", s.handlePackageEnable)
	api.POST("/packages/:id/disable", s.handlePackageDisable)
}

func (s *Server) handlePackagesList(c *gin.Context) {
	if s == nil || s.cfg == nil || s.pkgRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "package runtime unavailable"})
		return
	}
	rt := s.pkgRuntime
	res := s.cfg.ResolvePackages()
	items := s.buildPackageItems()
	var communityIDs []string
	for _, m := range s.communityDiscovered {
		communityIDs = append(communityIDs, strings.ToLower(strings.TrimSpace(m.ID)))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"host_api_version":     hostapi.APIVersion,
			"items":                items,
			"catalog":              rt.CatalogIDs(),
			"system_catalog":       config.SystemPackageCatalog,
			"http_extensions":      config.HTTPExtensionPackageIDs(),
			"recommended_products": config.RecommendedSystemPackageIDs(),
			"community_dir":        s.cfg.CommunityDir(),
			"community_discovered": communityIDs,
			"loaded":               rt.LoadedIDs(),
			"resolved":             res.Enabled,
			"runtime_enabled":      s.listRuntimeEnabled(),
			"recommended":          res.RecommendedSystem,
			"modules_deprecated":   res.ModulesDeprecated,
			"warnings":             res.Warnings,
			"statuses":             rt.Statuses(c.Request.Context()),
			// SSOT after first UI interaction: app-center-enabled.json;
			// packages.enabled is boot seed and in-memory mirror (synced on enable/disable).
			"enablement_source": "data_dir/app-center-enabled.json (SSOT) + packages.enabled boot seed → Runtime loaded",
			"persistence":       "app-center-enabled.json; cfg.Packages.Enabled mirrored in-process",
		},
	})
}

func (s *Server) buildPackageItems() []packageItem {
	rt := s.pkgRuntime
	if rt == nil {
		return nil
	}
	loadedSet := make(map[string]struct{})
	for _, id := range rt.LoadedIDs() {
		loadedSet[id] = struct{}{}
	}
	communitySet := make(map[string]packageruntime.DiskManifest)
	for _, m := range s.communityDiscovered {
		communitySet[strings.ToLower(strings.TrimSpace(m.ID))] = m
	}

	var items []packageItem
	// Official catalog: HTTP extensions + recommended products (all Runtime-operable).
	for _, e := range config.SystemPackageCatalog {
		_, loaded := loadedSet[e.ID]
		item := packageItem{
			ID:          e.ID,
			Trust:       string(hostapi.TrustSystem),
			Description: e.Description,
			Loaded:      loaded,
			Operable:    rt.Known(e.ID),
			CanEnable:   !loaded && rt.Known(e.ID),
			CanDisable:  loaded,
		}
		switch e.Kind {
		case config.KindHTTPExtension:
			item.Source = "system"
			item.Kind = string(config.KindHTTPExtension)
			item.Note = "HTTP extension — disable deactivates API routes until re-enabled"
		case config.KindRecommendedProduct:
			item.Source = "product"
			item.Kind = string(config.KindRecommendedProduct)
			item.Note = "Product surface — enable constructs/activates product managers"
			if e.ID == "cluster" {
				running := s.ClusterRunning()
				item.RequiresRestart = !running && !loaded
				if running {
					item.Note = "Cluster services running in this process; disable shuts them down and updates SSOT"
				} else if loaded {
					item.Note = "Cluster marked enabled but services not running — check logs or restart"
				} else {
					item.Note = "Enable starts cluster in-process when possible; disable shuts down. SSOT: app-center-enabled.json"
				}
			}
		default:
			continue
		}
		items = append(items, item)
	}
	// Community packages discovered on disk.
	for id, m := range communitySet {
		if config.IsCatalogedSystemPackage(id) {
			continue
		}
		_, loaded := loadedSet[id]
		trust := m.Trust
		if trust == "" {
			trust = string(hostapi.TrustLocal)
		}
		items = append(items, packageItem{
			ID:          id,
			Trust:       trust,
			Source:      "community",
			Kind:        "community",
			Description: m.Description,
			Version:     m.Version,
			Loaded:      loaded,
			Operable:    rt.Known(id),
			CanEnable:   !loaded && rt.Known(id),
			CanDisable:  loaded,
			Note:        "Third-party Host SDK package (community_dir)",
		})
	}
	return items
}

func (s *Server) handlePackageEnable(c *gin.Context) {
	if s == nil || s.pkgRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "package runtime unavailable"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(c.Param("id")))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "package id required"})
		return
	}
	if !s.pkgRuntime.Known(id) {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "package not in catalog (install/discover first)"})
		return
	}
	loaded, unknown, err := s.pkgRuntime.Enable(context.Background(), []string{id})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error(), "data": gin.H{"id": id, "loaded": s.pkgRuntime.LoadedIDs()}})
		return
	}
	if len(unknown) > 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "unknown package", "data": gin.H{"unknown": unknown}})
		return
	}
	s.addRuntimeEnabled(id)
	s.syncEnablementSSOT()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "enabled",
		"data": gin.H{
			"id":     id,
			"loaded": loaded,
			"items":  s.buildPackageItems(),
		},
	})
}

func (s *Server) handlePackageDisable(c *gin.Context) {
	if s == nil || s.pkgRuntime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "package runtime unavailable"})
		return
	}
	id := strings.ToLower(strings.TrimSpace(c.Param("id")))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "package id required"})
		return
	}
	if err := s.pkgRuntime.Disable(context.Background(), id); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	s.releaseProductManager(id)
	s.removeRuntimeEnabled(id)
	s.syncEnablementSSOT()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "disabled",
		"data": gin.H{
			"id":     id,
			"loaded": s.pkgRuntime.LoadedIDs(),
			"items":  s.buildPackageItems(),
		},
	})
}

// syncEnablementSSOT makes app-center-enabled.json + in-memory cfg.Packages.Enabled match
// the UI enable set (single source of truth for optional packages).
func (s *Server) syncEnablementSSOT() {
	if s == nil || s.cfg == nil {
		return
	}
	ids := s.listRuntimeEnabled()
	// Mirror into typed config so ResolvePackages sees the same set.
	s.cfg.Packages.Enabled = append([]string(nil), ids...)
	_ = s.persistRuntimeEnabled()
}

// --- runtime-enabled persistence (user click path) ---

func (s *Server) initRuntimeEnabled() {
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	if s.runtimeEnabled == nil {
		s.runtimeEnabled = make(map[string]struct{})
	}
}

func (s *Server) addRuntimeEnabled(id string) {
	s.initRuntimeEnabled()
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	s.runtimeEnabled[id] = struct{}{}
}

func (s *Server) removeRuntimeEnabled(id string) {
	s.initRuntimeEnabled()
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	delete(s.runtimeEnabled, id)
}

func (s *Server) listRuntimeEnabled() []string {
	s.initRuntimeEnabled()
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	out := make([]string, 0, len(s.runtimeEnabled))
	for id := range s.runtimeEnabled {
		out = append(out, id)
	}
	return out
}

func (s *Server) runtimeEnabledPath() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return s.cfg.DataPath("app-center-enabled.json")
}

// appCenterSSOTExists reports whether the enablement SSOT file is already on disk.
func (s *Server) appCenterSSOTExists() bool {
	path := s.runtimeEnabledPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (s *Server) persistRuntimeEnabled() error {
	path := s.runtimeEnabledPath()
	if path == "" {
		return nil
	}
	ids := s.listRuntimeEnabled()
	data, err := json.MarshalIndent(map[string]interface{}{"enabled": ids}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

func (s *Server) loadPersistedRuntimeEnabled() []string {
	path := s.runtimeEnabledPath()
	ids := loadEnabledIDsFromFile(path)
	s.initRuntimeEnabled()
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	for _, id := range ids {
		s.runtimeEnabled[id] = struct{}{}
	}
	return ids
}

// loadEnabledIDsFromFile reads app-center-enabled.json (shared with config.BootProductIDs).
func loadEnabledIDsFromFile(path string) []string {
	return config.LoadAppCenterEnabledIDs(path)
}

