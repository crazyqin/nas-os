package encryptionvault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultLockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	config := VaultConfig{VaultPath: filepath.Join(tmpDir, "vault")}
	vault := NewEncryptionVault(config)

	if vault.GetState() != VaultLocked {
		t.Fatal("vault should start locked")
	}

	err := vault.Unlock("testpassword123")
	if err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	if vault.GetState() != VaultUnlocked {
		t.Fatal("vault should be unlocked")
	}

	vault.Lock()
	if vault.GetState() != VaultLocked {
		t.Fatal("vault should be locked after lock")
	}
}

func TestVaultMaxAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	config := VaultConfig{VaultPath: filepath.Join(tmpDir, "vault"), MaxAttempts: 3}
	vault := NewEncryptionVault(config)

	vault.Unlock("correct")
	vault.Lock()

	for i := 0; i < 3; i++ {
		vault.Unlock("wrong")
	}

	err := vault.Unlock("wrong")
	if err == nil {
		t.Fatal("should fail after max attempts")
	}
}

func TestVaultEncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	config := VaultConfig{VaultPath: filepath.Join(tmpDir, "vault")}
	vault := NewEncryptionVault(config)
	vault.Unlock("testpass")

	content := []byte("secret data for testing")
	srcPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(srcPath, content, 0644)

	entry, err := vault.EncryptFile(srcPath, "test.txt")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if entry.Name != "test.txt" {
		t.Fatal("entry name mismatch")
	}

	destPath := filepath.Join(tmpDir, "decrypted.txt")
	err = vault.DecryptFile(entry.ID, destPath)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	decrypted, _ := os.ReadFile(destPath)
	if string(decrypted) != string(content) {
		t.Fatal("decrypted content mismatch")
	}
}

func TestVaultManager(t *testing.T) {
	mgr := NewVaultManager()
	tmpDir := t.TempDir()

	v1 := mgr.CreateVault("vault1", VaultConfig{VaultPath: filepath.Join(tmpDir, "v1")})
	if v1 == nil {
		t.Fatal("create vault failed")
	}

	v2, ok := mgr.GetVault("vault1")
	if !ok || v2 != v1 {
		t.Fatal("get vault failed")
	}

	names := mgr.ListVaults()
	if len(names) != 1 {
		t.Fatalf("expected 1 vault, got %d", len(names))
	}
}
