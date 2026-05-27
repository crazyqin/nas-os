package privacyvault

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:             true,
		DefaultAlgorithm:    "aes-256-gcm",
		AutoLockMinutes:     30,
		MaxVaults:           10,
		ShredPasses:         3,
		AuditEnabled:        true,
		HiddenVaultsAllowed: true,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateAndUnlockVault(t *testing.T) {
	config := &Config{
		Enabled:          true,
		MaxVaults:        10,
		ShredPasses:      3,
		AuditEnabled:     true,
		AutoLockMinutes:  30,
	}
	manager := NewManager(config)

	vault := &Vault{
		ID:          "test-vault-1",
		Name:        "My Secrets",
		Description: "Personal encrypted vault",
		Type:        VaultStandard,
		Algorithm:   AlgoAES256GCM,
		Size:        1024 * 1024 * 100, // 100MB
	}

	if err := manager.CreateVault(vault, "strong-password-123"); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}

	// Should be locked
	got, _ := manager.GetVault("test-vault-1")
	if got.Status != StatusLocked {
		t.Errorf("Expected locked, got %s", got.Status)
	}

	// Unlock with wrong password
	err := manager.Unlock("test-vault-1", "wrong-password")
	if err == nil {
		t.Error("Expected error for wrong password")
	}

	// Unlock with correct password
	if err := manager.Unlock("test-vault-1", "strong-password-123"); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	got, _ = manager.GetVault("test-vault-1")
	if got.Status != StatusUnlocked {
		t.Errorf("Expected unlocked, got %s", got.Status)
	}
}

func TestHiddenVault(t *testing.T) {
	config := &Config{
		Enabled:             true,
		MaxVaults:           10,
		AuditEnabled:        true,
		HiddenVaultsAllowed: true,
	}
	manager := NewManager(config)

	vault := &Vault{
		ID:         "hidden-1",
		Name:       "Hidden Vault",
		Type:       VaultHidden,
		Algorithm:  AlgoAES256GCM,
		DenyExists: true,
	}

	if err := manager.CreateVault(vault, "hidden-pass"); err != nil {
		t.Fatalf("CreateVault hidden failed: %v", err)
	}

	stats := manager.GetStats()
	if stats.HiddenVaults != 1 {
		t.Errorf("Expected 1 hidden vault, got %d", stats.HiddenVaults)
	}
}

func TestSecureDelete(t *testing.T) {
	config := &Config{
		Enabled:      true,
		MaxVaults:    10,
		AuditEnabled: true,
		ShredPasses:  3,
	}
	manager := NewManager(config)

	vault := &Vault{
		ID:        "vault-files",
		Name:      "File Vault",
		Type:      VaultStandard,
		Algorithm: AlgoAES256GCM,
	}
	manager.CreateVault(vault, "pass123")
	manager.Unlock("vault-files", "pass123")

	file := &SecureFile{
		ID:   "file-1",
		Name: "secret.txt",
		Path: "/secret.txt",
		Size: 1024,
	}
	manager.AddFile("vault-files", file)

	if err := manager.SecureDelete("vault-files", "file-1"); err != nil {
		t.Fatalf("SecureDelete failed: %v", err)
	}

	stats := manager.GetStats()
	if stats.TotalFiles != 0 {
		t.Errorf("Expected 0 files after delete, got %d", stats.TotalFiles)
	}
}

func TestEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, NAS-OS Privacy Vault!")

	ciphertext, err := EncryptData(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}

	decrypted, err := DecryptData(key, ciphertext)
	if err != nil {
		t.Fatalf("DecryptData failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s", string(decrypted))
	}
}

func TestAuditLog(t *testing.T) {
	config := &Config{
		Enabled:      true,
		MaxVaults:    10,
		AuditEnabled: true,
	}
	manager := NewManager(config)

	vault := &Vault{
		ID:        "audit-vault",
		Name:      "Audit Test",
		Type:      VaultStandard,
		Algorithm: AlgoAES256GCM,
	}
	manager.CreateVault(vault, "pass")
	manager.Unlock("audit-vault", "pass")
	manager.Lock("audit-vault")

	entries := manager.GetAuditLog("audit-vault", 10)
	if len(entries) < 3 {
		t.Errorf("Expected at least 3 audit entries, got %d", len(entries))
	}
}
