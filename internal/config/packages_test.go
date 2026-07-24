package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestResolvePackages_DefaultCoreOnly(t *testing.T) {
	cfg := Default()
	res := cfg.ResolvePackages()
	if res.RecommendedSystem {
		t.Fatal("default recommended_system must be false")
	}
	if len(res.Enabled) != 0 {
		t.Fatalf("default enabled must be empty, got %v", res.Enabled)
	}
	if res.DualSource {
		t.Fatal("default must not be dual-source")
	}
	if cfg.OptionalProductsEnabled() {
		t.Fatal("OptionalProductsEnabled must be false by default")
	}
	if names := cfg.EnabledPackageNames(); len(names) != 0 {
		t.Fatalf("EnabledPackageNames default: %v", names)
	}
}

func TestResolvePackages_ModulesOnlyOptional(t *testing.T) {
	cfg := Default()
	cfg.Modules.Optional = true
	res := cfg.ResolvePackages()
	if !res.RecommendedSystem {
		t.Fatal("modules.optional=true must set RecommendedSystem")
	}
	if len(res.Enabled) == 0 {
		t.Fatal("optional must expand to non-empty recommended set")
	}
	for _, want := range RecommendedSystemPackages {
		if !slices.Contains(res.Enabled, want) {
			t.Fatalf("missing recommended package %q in %v", want, res.Enabled)
		}
	}
	if res.DualSource {
		t.Fatal("modules-only must not be dual-source")
	}
}

func TestResolvePackages_ModulesOnlyExtensions(t *testing.T) {
	cfg := Default()
	cfg.Modules.Extensions = []string{"VoiceHub", " netdiag ", "voicehub"}
	res := cfg.ResolvePackages()
	if res.RecommendedSystem {
		t.Fatal("extensions alone must not enable recommended_system")
	}
	if !slices.Equal(res.Enabled, []string{"netdiag", "voicehub"}) {
		t.Fatalf("enabled=%v", res.Enabled)
	}
}

func TestResolvePackages_PackagesOnly(t *testing.T) {
	cfg := Default()
	cfg.Packages.RecommendedSystem = true
	cfg.Packages.Enabled = []string{"voicehub"}
	res := cfg.ResolvePackages()
	if !res.RecommendedSystem {
		t.Fatal("packages.recommended_system must enable RecommendedSystem")
	}
	if !slices.Contains(res.Enabled, "voicehub") {
		t.Fatalf("named package missing: %v", res.Enabled)
	}
	if !slices.Contains(res.Enabled, "docker") {
		t.Fatalf("recommended set missing docker: %v", res.Enabled)
	}
	if res.DualSource {
		t.Fatal("packages-only must not be dual-source")
	}
}

func TestResolvePackages_UnionBothSources(t *testing.T) {
	cfg := Default()
	cfg.Modules.Optional = true
	cfg.Modules.Extensions = []string{"agentworkflow"}
	cfg.Packages.RecommendedSystem = false
	cfg.Packages.Enabled = []string{"voicehub", "netdiag"}
	res := cfg.ResolvePackages()
	if !res.RecommendedSystem {
		t.Fatal("union must keep optional→RecommendedSystem")
	}
	if !res.DualSource {
		t.Fatal("both modules and packages contributed: DualSource expected")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("dual-source should emit warning")
	}
	for _, want := range []string{"agentworkflow", "voicehub", "netdiag", "docker"} {
		if !slices.Contains(res.Enabled, want) {
			t.Fatalf("union missing %q: %v", want, res.Enabled)
		}
	}
}

func TestResolvePackages_PackagesRecommendedOnlyExpands(t *testing.T) {
	cfg := Default()
	cfg.Packages.RecommendedSystem = true
	res := cfg.ResolvePackages()
	if !res.RecommendedSystem || len(res.Enabled) == 0 {
		t.Fatalf("recommended_system alone must expand: rec=%v enabled=%v",
			res.RecommendedSystem, res.Enabled)
	}
	if len(RecommendedSystemPackages) == 0 {
		t.Fatal("RecommendedSystemPackages must be non-empty")
	}
}

func TestLoadYAML_PackagesSurface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	yaml := `server:
  host: 127.0.0.1
  port: 8080
paths:
  mount_base: /mnt
  config_dir: /etc/nas-os
  data_dir: /var/lib/nas-os
  samba_config: /etc/samba/smb.conf
  nfs_exports: /etc/exports
packages:
  recommended_system: true
  enabled:
    - voicehub
    - NetDiag
modules:
  optional: false
  extensions:
    - agentworkflow
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Drive real Load → ResolvePackages path.
	res := cfg.ResolvePackages()
	if !res.RecommendedSystem {
		t.Fatal("packages.recommended_system from YAML must resolve true")
	}
	if !res.DualSource {
		t.Fatal("modules.extensions + packages.* must be dual-source")
	}
	for _, want := range []string{"voicehub", "netdiag", "agentworkflow", "docker"} {
		if !slices.Contains(res.Enabled, want) {
			t.Fatalf("Load+Resolve missing %q: %v", want, res.Enabled)
		}
	}
	if !cfg.OptionalProductsEnabled() {
		t.Fatal("OptionalProductsEnabled after Load")
	}
}

func TestLoadYAML_DefaultFileCoreOnly(t *testing.T) {
	// Real default.yaml path if present; otherwise Load("") defaults.
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	res := cfg.ResolvePackages()
	if res.RecommendedSystem || len(res.Enabled) != 0 {
		t.Fatalf("empty Load must be Core-only: %+v", res)
	}
}

func TestEnabledNamedPackages_FiltersKnown(t *testing.T) {
	cfg := Default()
	cfg.Modules.Optional = true
	cfg.Packages.Enabled = []string{"voicehub", "not-a-real-ext"}
	known := []string{"voicehub", "netdiag"}
	got := cfg.EnabledNamedPackages(known)
	if !slices.Equal(got, []string{"voicehub"}) {
		t.Fatalf("got %v", got)
	}
}

func TestOptionalProductsEnabled_PackagesFlag(t *testing.T) {
	cfg := Default()
	cfg.Packages.RecommendedSystem = true
	if !cfg.OptionalProductsEnabled() {
		t.Fatal("packages.recommended_system alone must enable optional products")
	}
}

func TestResolvePackages_StrictIgnoresModules(t *testing.T) {
	t.Setenv("NAS_OS_STRICT_PACKAGES", "1")
	cfg := Default()
	cfg.Modules.Optional = true
	cfg.Modules.Extensions = []string{"voicehub"}
	cfg.Packages.RecommendedSystem = false
	cfg.Packages.Enabled = nil
	res := cfg.ResolvePackages()
	if res.RecommendedSystem {
		t.Fatal("strict must ignore modules.optional")
	}
	if len(res.Enabled) != 0 {
		t.Fatalf("strict enabled=%v want empty", res.Enabled)
	}
	if !res.ModulesDeprecated {
		t.Fatal("still mark ModulesDeprecated for migration signal")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("want strict ignore warning")
	}
}

func TestValidateDeprecatedModulesStrict(t *testing.T) {
	t.Setenv("NAS_OS_REJECT_MODULES", "1")
	cfg := Default()
	cfg.Modules.Optional = true
	if err := cfg.ValidateDeprecatedModulesStrict(); err == nil {
		t.Fatal("want reject error")
	}
	cfg.Modules.Optional = false
	if err := cfg.ValidateDeprecatedModulesStrict(); err != nil {
		t.Fatal(err)
	}
}
