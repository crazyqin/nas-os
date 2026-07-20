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
	Source      string `json:"source"` // system | community
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Loaded      bool   `json:"loaded"`
	CanEnable   bool   `json:"can_enable"`
	CanDisable  bool   `json:"can_disable"`
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
			"persistence":          "in-process + data_dir/app-center-enabled.json (survives restart when data dir writable)",
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
	// Official HTTP extensions from system catalog.
	for _, e := range config.SystemPackageCatalog {
		if e.Kind != config.KindHTTPExtension {
			continue
		}
		_, loaded := loadedSet[e.ID]
		items = append(items, packageItem{
			ID:          e.ID,
			Trust:       string(hostapi.TrustSystem),
			Source:      "system",
			Description: e.Description,
			Loaded:      loaded,
			CanEnable:   !loaded && rt.Known(e.ID),
			CanDisable:  loaded,
		})
	}
	// Community packages discovered on disk.
	for id, m := range communitySet {
		if config.IsCatalogedSystemPackage(id) {
			continue // already listed as system if any clash
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
			Description: m.Description,
			Version:     m.Version,
			Loaded:      loaded,
			CanEnable:   !loaded && rt.Known(id),
			CanDisable:  loaded,
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
	_ = s.persistRuntimeEnabled()
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
	s.removeRuntimeEnabled(id)
	_ = s.persistRuntimeEnabled()
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
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Enabled []string `json:"enabled"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	s.initRuntimeEnabled()
	s.runtimeEnabledMu.Lock()
	defer s.runtimeEnabledMu.Unlock()
	for _, id := range doc.Enabled {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" {
			s.runtimeEnabled[id] = struct{}{}
		}
	}
	return doc.Enabled
}

