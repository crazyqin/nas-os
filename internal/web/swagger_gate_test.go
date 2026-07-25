package web

import "testing"

func TestSwaggerEnabled_DefaultAndEnv(t *testing.T) {
	t.Setenv("NAS_OS_SWAGGER", "")
	t.Setenv("NAS_OS_ENV", "")
	if !swaggerEnabled() {
		t.Fatal("default non-prod should enable swagger")
	}
	t.Setenv("NAS_OS_ENV", "production")
	if swaggerEnabled() {
		t.Fatal("production should disable swagger by default")
	}
	t.Setenv("NAS_OS_SWAGGER", "1")
	if !swaggerEnabled() {
		t.Fatal("NAS_OS_SWAGGER=1 should force on")
	}
	t.Setenv("NAS_OS_ENV", "dev")
	t.Setenv("NAS_OS_SWAGGER", "0")
	if swaggerEnabled() {
		t.Fatal("NAS_OS_SWAGGER=0 should force off")
	}
}
