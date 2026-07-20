package config

import (
	"slices"
	"strings"
	"testing"
)

func TestSystemPackageCatalogNonEmptyKinds(t *testing.T) {
	if len(SystemPackageCatalog) == 0 {
		t.Fatal("catalog must not be empty")
	}
	rec := RecommendedSystemPackageIDs()
	ext := HTTPExtensionPackageIDs()
	if len(rec) == 0 {
		t.Fatal("recommended products empty")
	}
	if len(ext) == 0 {
		t.Fatal("http extensions empty")
	}
	if len(rec)+len(ext) != len(SystemPackageCatalog) {
		t.Fatalf("kind partition %d+%d != catalog %d", len(rec), len(ext), len(SystemPackageCatalog))
	}
	// RecommendedSystemPackages must match catalog-derived IDs.
	if !slices.Equal(RecommendedSystemPackages, rec) {
		t.Fatalf("RecommendedSystemPackages=%v want %v", RecommendedSystemPackages, rec)
	}
	for _, id := range []string{"docker", "voicehub", "netdiag"} {
		if !IsCatalogedSystemPackage(id) {
			t.Fatalf("%s should be cataloged", id)
		}
	}
	if IsCatalogedSystemPackage("not-a-package") {
		t.Fatal("unknown must not be cataloged")
	}
}

func TestResolvePackages_DeprecationWhenModulesUsed(t *testing.T) {
	cfg := Default()
	cfg.Modules.Optional = true
	res := cfg.ResolvePackages()
	if !res.ModulesDeprecated {
		t.Fatal("modules.optional must mark ModulesDeprecated")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected deprecation warning")
	}
	found := false
	for _, w := range res.Warnings {
		if containsFold(w, "deprecated") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings=%v", res.Warnings)
	}
}

func TestResolvePackages_NoDeprecationWhenPackagesOnly(t *testing.T) {
	cfg := Default()
	cfg.Packages.RecommendedSystem = true
	cfg.Packages.Enabled = []string{"voicehub"}
	res := cfg.ResolvePackages()
	if res.ModulesDeprecated {
		t.Fatal("packages-only must not set ModulesDeprecated")
	}
	if !res.RecommendedSystem {
		t.Fatal("recommended expected")
	}
	if !slices.Contains(res.Enabled, "voicehub") || !slices.Contains(res.Enabled, "docker") {
		t.Fatalf("enabled=%v", res.Enabled)
	}
}

func TestResolvePackages_DefaultNoDeprecation(t *testing.T) {
	res := Default().ResolvePackages()
	if res.ModulesDeprecated || len(res.Warnings) != 0 {
		t.Fatalf("default must be clean: %+v", res)
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
