package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nas-os/internal/config"
)

func TestValidateBinaryCapabilities_CoreRejectsProducts(t *testing.T) {
	if ProductsLinked() {
		t.Skip("full build links products")
	}
	cfg := config.Default()
	cfg.Packages.RecommendedSystem = true
	err := ValidateBinaryCapabilities(cfg)
	if err == nil {
		t.Fatal("expected error when recommended_system on Core binary")
	}
	if !strings.Contains(err.Error(), "nasd_full") {
		t.Fatalf("error should mention nasd_full: %v", err)
	}
}

func TestValidateBinaryCapabilities_CoreRejectsNamedProduct(t *testing.T) {
	if ProductsLinked() {
		t.Skip("full build")
	}
	cfg := config.Default()
	cfg.Packages.Enabled = []string{"docker"}
	err := ValidateBinaryCapabilities(cfg)
	if err == nil {
		t.Fatal("expected error for packages.enabled=[docker] on Core")
	}
}

func TestValidateBinaryCapabilities_CoreAllowsEmpty(t *testing.T) {
	cfg := config.Default()
	if err := ValidateBinaryCapabilities(cfg); err != nil {
		t.Fatalf("default config must be valid on any build: %v", err)
	}
}

func TestValidateBinaryCapabilities_FullAllowsProducts(t *testing.T) {
	if !ProductsLinked() {
		t.Skip("core build")
	}
	cfg := config.Default()
	cfg.Packages.RecommendedSystem = true
	if err := ValidateBinaryCapabilities(cfg); err != nil {
		t.Fatalf("full build should accept recommended_system: %v", err)
	}
}

func TestCapabilityGaps_AppCenterExtension(t *testing.T) {
	if ExtensionsLinked() {
		t.Skip("full build links extensions")
	}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Paths.DataDir = dir
	path := filepath.Join(dir, "app-center-enabled.json")
	if err := os.WriteFile(path, []byte(`{"enabled":["netdiag"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	gaps := CapabilityGaps(cfg)
	found := false
	for _, g := range gaps {
		if g == "netdiag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected netdiag in gaps, got %v", gaps)
	}
}

func TestPackageLinked_CommunityAlways(t *testing.T) {
	if !packageLinked("com.example.hello") {
		t.Fatal("unknown community ids should be considered linkable at Host SDK layer")
	}
}
