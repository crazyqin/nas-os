package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if cfg.Server.Addr() != "127.0.0.1:8080" {
		t.Fatalf("unexpected default addr (must bind localhost by default): %s", cfg.Server.Addr())
	}
	if cfg.Modules.Optional {
		t.Fatal("default modules.optional must be false")
	}
}

func TestOptionalModulesDefaultOff(t *testing.T) {
	cfg := Default()
	if cfg.Modules.Optional || len(cfg.Modules.Extensions) != 0 {
		t.Fatalf("optional=%v extensions=%v want off", cfg.Modules.Optional, cfg.Modules.Extensions)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Paths.MountBase != "/mnt" {
		t.Fatalf("expected default mount_base, got %s", cfg.Paths.MountBase)
	}
}

func TestLoadYAMLOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	yaml := `server:
  host: 127.0.0.1
  port: 9090
paths:
  mount_base: /srv/nas
  config_dir: /srv/nas/etc
  data_dir: /srv/nas/var
  samba_config: /srv/nas/etc/smb.conf
  nfs_exports: /srv/nas/etc/exports
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr() != "127.0.0.1:9090" {
		t.Fatalf("addr override failed: %s", cfg.Server.Addr())
	}
	if cfg.Paths.MountBase != "/srv/nas" {
		t.Fatalf("mount_base override failed: %s", cfg.Paths.MountBase)
	}
	if cfg.ConfigPath("plugins") != "/srv/nas/etc/plugins" {
		t.Fatalf("ConfigPath helper failed: %s", cfg.ConfigPath("plugins"))
	}
	if cfg.DataPath("photos") != "/srv/nas/var/photos" {
		t.Fatalf("DataPath helper failed: %s", cfg.DataPath("photos"))
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("NAS_OS_LISTEN_PORT", "18080")
	t.Setenv("NAS_OS_MOUNT_BASE", "/mnt/custom")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 18080 {
		t.Fatalf("env port override failed: %d", cfg.Server.Port)
	}
	if cfg.Paths.MountBase != "/mnt/custom" {
		t.Fatalf("env mount_base override failed: %s", cfg.Paths.MountBase)
	}
}

func TestInvalidEnvPortReturnsError(t *testing.T) {
	t.Setenv("NAS_OS_LISTEN_PORT", "not-a-port")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid env port error")
	}
}

func TestValidateRejectsRelativePath(t *testing.T) {
	cfg := Default()
	cfg.Paths.MountBase = "relative/path"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for relative mount_base")
	}
}

func TestValidateRejectsBadPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero port")
	}
}

func TestBadYAMLReportsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: [not\n a number"), 0o600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected yaml parse error")
	}
}
