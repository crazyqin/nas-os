package web

import (
	"testing"
)

func TestDefaultSecurityConfigAllowsDevWithoutKey(t *testing.T) {
	t.Setenv("NAS_CSRF_KEY", "")
	t.Setenv("NAS_OS_ENV", "")
	t.Setenv("NAS_OS_REQUIRE_CSRF_KEY", "")
	cfg := DefaultSecurityConfig()
	if len(cfg.CSRFKey) == 0 {
		t.Fatal("expected ephemeral CSRF key in dev")
	}
}

func TestDefaultSecurityConfigPanicsInProductionWithoutKey(t *testing.T) {
	t.Setenv("NAS_CSRF_KEY", "")
	t.Setenv("NAS_OS_ENV", "production")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when NAS_OS_ENV=production without NAS_CSRF_KEY")
		}
	}()
	_ = DefaultSecurityConfig()
}
