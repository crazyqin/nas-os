package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMFAConfigRequiresManager(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mfa-config.json")

	if MFAConfigRequiresManager("") || MFAConfigRequiresManager(path) {
		t.Fatal("missing path must not require")
	}

	// empty map
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if MFAConfigRequiresManager(path) {
		t.Fatal("empty configs must not require")
	}

	// disabled user
	data, _ := json.Marshal(map[string]*MFAConfig{
		"u1": {UserID: "u1", Enabled: false, TOTPEnabled: false},
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if MFAConfigRequiresManager(path) {
		t.Fatal("disabled MFA must not require")
	}

	// enabled TOTP
	data, _ = json.Marshal(map[string]*MFAConfig{
		"u1": {UserID: "u1", TOTPEnabled: true},
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if !MFAConfigRequiresManager(path) {
		t.Fatal("TOTP enabled must require manager")
	}

	// corrupt
	if err := os.WriteFile(path, []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !MFAConfigRequiresManager(path) {
		t.Fatal("corrupt mfa-config must require (fail closed)")
	}
}
