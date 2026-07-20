package web

import (
	"slices"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

// TestHTTPCatalogMatchesMountTableAndRuntime drives real catalog helpers and
// registerSystemPackageCatalog — fails if SystemPackageCatalog HTTP IDs diverge
// from the mount table or Package Runtime registration.
func TestHTTPCatalogMatchesMountTableAndRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	catalog := config.HTTPExtensionPackageIDs()
	if len(catalog) == 0 {
		t.Fatal("HTTPExtensionPackageIDs empty")
	}
	mountIDs := MountTableHTTPExtensionIDs()
	if !sameStringSet(catalog, mountIDs) {
		t.Fatalf("catalog HTTP IDs %v != mount table %v", catalog, mountIDs)
	}
	if !sameStringSet(catalog, KnownExtensionNames) {
		t.Fatalf("KnownExtensionNames %v != catalog %v", KnownExtensionNames, catalog)
	}

	cfg := config.Default()
	s := &Server{cfg: cfg}
	r := gin.New()
	api := r.Group("/api/v1")
	// Real production path: registerConfiguredExtensions always installs catalog.
	s.registerConfiguredExtensions(api)
	if s.pkgRuntime == nil {
		t.Fatal("pkgRuntime not set")
	}
	runtimeIDs := s.pkgRuntime.CatalogIDs()
	// Runtime catalog includes HTTP extensions + recommended products + community.
	for _, id := range catalog {
		if !slices.Contains(runtimeIDs, id) {
			t.Fatalf("HTTP extension %q missing from runtime catalog %v", id, runtimeIDs)
		}
	}
	for _, id := range config.RecommendedSystemPackageIDs() {
		if !slices.Contains(runtimeIDs, id) {
			t.Fatalf("recommended product %q missing from runtime catalog", id)
		}
	}
	// Default enablement: nothing loaded.
	if loaded := s.pkgRuntime.LoadedIDs(); len(loaded) != 0 {
		t.Fatalf("default boot must load no packages, got %v", loaded)
	}
}

// TestDefaultResolutionCoreOnly drives real Default() + ResolvePackages.
func TestDefaultResolutionCoreOnly(t *testing.T) {
	cfg := config.Default()
	res := cfg.ResolvePackages()
	if res.RecommendedSystem {
		t.Fatal("default recommended_system must be false")
	}
	if len(res.Enabled) != 0 {
		t.Fatalf("default enabled must be empty, got %v", res.Enabled)
	}
	if cfg.OptionalProductsEnabled() {
		t.Fatal("OptionalProductsEnabled must be false by default")
	}
	if res.ModulesDeprecated {
		t.Fatal("default must not mark modules deprecated")
	}
}

// TestRecommendedProductIDsNonEmptyAndDisjointFromHTTP locks catalog kinds.
func TestRecommendedProductIDsNonEmptyAndDisjointFromHTTP(t *testing.T) {
	rec := config.RecommendedSystemPackageIDs()
	httpIDs := config.HTTPExtensionPackageIDs()
	if len(rec) == 0 || len(httpIDs) == 0 {
		t.Fatal("both catalog kinds must be non-empty")
	}
	for _, id := range rec {
		if slices.Contains(httpIDs, id) {
			t.Fatalf("%q is both recommended_product and http_extension", id)
		}
		entry, ok := config.LookupSystemPackage(id)
		if !ok || entry.Kind != config.KindRecommendedProduct {
			t.Fatalf("%q kind mismatch", id)
		}
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}
