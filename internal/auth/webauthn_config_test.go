package auth

import (
	"os"
	"testing"
)

func TestDefaultWebAuthnConfig_Env(t *testing.T) {
	t.Setenv("NAS_OS_WEBAUTHN_RPID", "https://nas.example.com/path")
	t.Setenv("NAS_OS_WEBAUTHN_ORIGINS", "https://nas.example.com, http://127.0.0.1:8080")
	t.Setenv("NAS_OS_WEBAUTHN_NAME", "Home NAS")
	cfg := DefaultWebAuthnConfig("ignored")
	if cfg.RPID != "nas.example.com" {
		t.Fatalf("RPID=%q", cfg.RPID)
	}
	if cfg.RPDisplayName != "Home NAS" {
		t.Fatalf("name=%q", cfg.RPDisplayName)
	}
	if len(cfg.RPOrigins) != 2 || cfg.RPOrigins[0] != "https://nas.example.com" {
		t.Fatalf("origins=%v", cfg.RPOrigins)
	}
	_ = os.Unsetenv // keep linter quiet if unused
}

func TestDefaultWebAuthnConfig_Defaults(t *testing.T) {
	t.Setenv("NAS_OS_WEBAUTHN_RPID", "")
	t.Setenv("NAS_OS_WEBAUTHN_ORIGINS", "")
	t.Setenv("NAS_OS_WEBAUTHN_NAME", "")
	cfg := DefaultWebAuthnConfig("NAS-OS")
	if cfg.RPID != "localhost" {
		t.Fatalf("default RPID=%q", cfg.RPID)
	}
	if len(cfg.RPOrigins) < 2 {
		t.Fatalf("default origins empty: %v", cfg.RPOrigins)
	}
}
