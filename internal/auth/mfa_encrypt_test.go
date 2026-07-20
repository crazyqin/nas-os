package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMFAManager_TOTPSecretEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mfa-config.json")

	m, err := NewMFAManager(cfgPath, "NAS-OS-Test", nil)
	if err != nil {
		t.Fatalf("NewMFAManager: %v", err)
	}

	setup, err := m.SetupTOTP("user-1", "alice")
	if err != nil {
		t.Fatalf("SetupTOTP: %v", err)
	}
	if setup.Secret == "" {
		t.Fatal("expected plaintext secret returned to client")
	}

	// On-disk file must not contain the raw secret
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(raw), setup.Secret) {
		t.Fatalf("raw TOTP secret leaked to disk:\n%s", raw)
	}
	if !strings.Contains(string(raw), totpSecretEncPrefix) {
		t.Fatalf("expected encrypted prefix on disk, got:\n%s", raw)
	}

	// Master key file exists with restricted mode intent (0600)
	keyPath := filepath.Join(dir, "mfa-master.key")
	if st, err := os.Stat(keyPath); err != nil {
		t.Fatalf("master key missing: %v", err)
	} else if st.Mode().Perm()&0077 != 0 {
		t.Fatalf("master key too open: %v", st.Mode())
	}

	// Reload and verify secret is usable again
	m2, err := NewMFAManager(cfgPath, "NAS-OS-Test", nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cfg := m2.GetConfig("user-1")
	if cfg == nil || cfg.TOTPSecret == "" {
		t.Fatal("expected decrypted secret in memory after reload")
	}
	if cfg.TOTPSecret != setup.Secret {
		t.Fatalf("secret mismatch after reload")
	}
	// In-memory JSON of configs via save path still encrypts
	var disk map[string]map[string]any
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if sec, _ := disk["user-1"]["totp_secret"].(string); strings.HasPrefix(sec, totpSecretEncPrefix) == false {
		t.Fatalf("disk totp_secret not encrypted: %v", sec)
	}
}
