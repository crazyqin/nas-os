package application

import (
	"strings"
	"testing"

	"nas-os/internal/config"
	"nas-os/internal/web"
)

// TestNewRejectsUnlinkedProductsOnCore drives application.New with packages that
// require nasd_full. On Core builds this must fail closed.
func TestNewRejectsUnlinkedProductsOnCore(t *testing.T) {
	if web.ProductsLinked() {
		t.Skip("full build links products")
	}
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	cfg.Packages.Enabled = []string{"docker"}

	app, err := New(cfg, nil)
	if err == nil {
		if app != nil {
			_ = app.Stop()
		}
		t.Fatal("expected New to fail when docker requested on Core binary")
	}
	if !strings.Contains(err.Error(), "nasd_full") && !strings.Contains(err.Error(), "binary capability") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestNewAcceptsDefaultCoreConfig drives application.New with default Core-only config.
func TestNewAcceptsDefaultCoreConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	cfg.Paths.ConfigDir = t.TempDir()
	cfg.Paths.MountBase = t.TempDir()
	// Avoid absolute /etc paths in tests
	cfg.Paths.SambaConfig = cfg.Paths.ConfigDir + "/smb.conf"
	cfg.Paths.NFSExports = cfg.Paths.ConfigDir + "/exports"

	app, err := New(cfg, nil)
	if err != nil {
		// storage scan may fail without btrfs tools — still must not be capability error
		if strings.Contains(err.Error(), "binary capability") || strings.Contains(err.Error(), "nasd_full") {
			t.Fatalf("default config must not fail capability check: %v", err)
		}
		t.Logf("New failed for env reasons (ok if not capability): %v", err)
		return
	}
	defer app.Stop()
}
