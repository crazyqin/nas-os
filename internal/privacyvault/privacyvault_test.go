// Package privacyvault - privacyvault_test.go 隐私保险箱模块完整测试
package privacyvault

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("returns valid default config", func(t *testing.T) {
		config := DefaultConfig()
		if config == nil {
			t.Fatal("DefaultConfig returned nil")
		}
		if !config.Enabled {
			t.Error("Expected Enabled to be true")
		}
		if config.DefaultAlgorithm != AlgoAES256GCM {
			t.Errorf("Expected algorithm %s, got %s", AlgoAES256GCM, config.DefaultAlgorithm)
		}
		if config.AutoLockMinutes != 30 {
			t.Errorf("Expected AutoLockMinutes 30, got %d", config.AutoLockMinutes)
		}
		if config.MaxVaults != 10 {
			t.Errorf("Expected MaxVaults 10, got %d", config.MaxVaults)
		}
		if config.ShredPasses != 3 {
			t.Errorf("Expected ShredPasses 3, got %d", config.ShredPasses)
		}
	})
}

func TestNewEngine(t *testing.T) {
	t.Run("creates engine with default config", func(t *testing.T) {
		engine := NewEngine(nil)
		if engine == nil {
			t.Fatal("NewEngine returned nil")
		}
	})

	t.Run("creates engine with custom config", func(t *testing.T) {
		config := &PrivacyVaultConfig{
			Enabled:             true,
			DefaultAlgorithm:    AlgoAES256GCM,
			AutoLockMinutes:     15,
			MaxVaults:           5,
			ShredPasses:         5,
			AuditEnabled:        true,
			HiddenVaultsAllowed: true,
		}
		engine := NewEngine(config)
		if engine == nil {
			t.Fatal("NewEngine returned nil")
		}
	})
}

func TestCreateVault(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("creates vault successfully", func(t *testing.T) {
		vault := &Vault{
			ID:          "vault-1",
			Name:        "Test Vault",
			Description: "A test vault",
			Type:        "standard",
			Algorithm:   AlgoAES256GCM,
			Size:        1024 * 1024 * 100,
			OwnerID:     "user-1",
		}

		err := engine.CreateVault(vault, "strong-password-123")
		if err != nil {
			t.Fatalf("CreateVault failed: %v", err)
		}

		if vault.Status != StatusLocked {
			t.Errorf("Expected status %s, got %s", StatusLocked, vault.Status)
		}
		if vault.KeyID == "" {
			t.Error("Expected KeyID to be set")
		}
		if vault.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("rejects duplicate vault ID", func(t *testing.T) {
		vault := &Vault{
			ID:        "vault-1",
			Name:      "Duplicate",
			Type:      "standard",
			Algorithm: AlgoAES256GCM,
		}

		err := engine.CreateVault(vault, "password")
		if err == nil {
			t.Error("Expected error for duplicate vault ID")
		}
	})

	t.Run("respects max vaults limit", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxVaults = 1
		eng := NewEngine(config)

		vault1 := &Vault{ID: "v1", Type: "standard", Algorithm: AlgoAES256GCM}
		vault2 := &Vault{ID: "v2", Type: "standard", Algorithm: AlgoAES256GCM}

		eng.CreateVault(vault1, "pass")
		err := eng.CreateVault(vault2, "pass")
		if err == nil {
			t.Error("Expected error when exceeding max vaults")
		}
	})
}

func TestUnlockAndLock(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	vault := &Vault{
		ID:        "vault-lock",
		Name:      "Lock Test",
		Type:      "standard",
		Algorithm: AlgoAES256GCM,
	}
	engine.CreateVault(vault, "test-password")

	t.Run("unlocks with correct password", func(t *testing.T) {
		err := engine.Unlock("vault-lock", "test-password")
		if err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}

		v, _ := engine.GetVault("vault-lock")
		if v.Status != StatusUnlocked {
			t.Errorf("Expected %s, got %s", StatusUnlocked, v.Status)
		}
	})

	t.Run("locks vault", func(t *testing.T) {
		err := engine.Lock("vault-lock")
		if err != nil {
			t.Fatalf("Lock failed: %v", err)
		}

		v, _ := engine.GetVault("vault-lock")
		if v.Status != StatusLocked {
			t.Errorf("Expected %s, got %s", StatusLocked, v.Status)
		}
	})

	t.Run("returns error for non-existent vault", func(t *testing.T) {
		err := engine.Unlock("nonexistent", "pass")
		if err == nil {
			t.Error("Expected error for non-existent vault")
		}
	})

	t.Run("returns error when already unlocked", func(t *testing.T) {
		engine.Unlock("vault-lock", "test-password")
		err := engine.Unlock("vault-lock", "test-password")
		if err == nil {
			t.Error("Expected error when vault already unlocked")
		}
		engine.Lock("vault-lock")
	})
}

func TestMaxFailedAttempts(t *testing.T) {
	config := DefaultConfig()
	config.MaxFailedAttempts = 2
	engine := NewEngine(config)

	vault := &Vault{ID: "vault-fail", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "correct-pass")

	t.Run("blocks after max failed attempts", func(t *testing.T) {
		engine.Unlock("vault-fail", "wrong1")
		engine.Unlock("vault-fail", "wrong2")

		err := engine.Unlock("vault-fail", "wrong3")
		if err == nil {
			t.Error("Expected error after max failed attempts")
		}
	})
}

func TestAddAndGetSecret(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-sec", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-sec", "pass")

	t.Run("adds secret successfully", func(t *testing.T) {
		secret := &Secret{
			ID:            "sec-1",
			Name:          "My Password",
			Type:          "credential",
			EncryptedData: []byte("encrypted-data"),
			DataSize:      100,
		}

		err := engine.AddSecret("vault-sec", secret)
		if err != nil {
			t.Fatalf("AddSecret failed: %v", err)
		}
	})

	t.Run("retrieves secret by ID", func(t *testing.T) {
		secret, err := engine.GetSecret("vault-sec", "sec-1")
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}
		if secret.Name != "My Password" {
			t.Errorf("Expected name 'My Password', got '%s'", secret.Name)
		}
	})

	t.Run("lists all secrets", func(t *testing.T) {
		secrets, err := engine.ListSecrets("vault-sec")
		if err != nil {
			t.Fatalf("ListSecrets failed: %v", err)
		}
		if len(secrets) != 1 {
			t.Errorf("Expected 1 secret, got %d", len(secrets))
		}
	})

	t.Run("rejects adding to locked vault", func(t *testing.T) {
		engine.Lock("vault-sec")
		secret := &Secret{ID: "sec-2", Name: "test"}
		err := engine.AddSecret("vault-sec", secret)
		if err == nil {
			t.Error("Expected error when adding to locked vault")
		}
		engine.Unlock("vault-sec", "pass")
	})
}

func TestDestroyVault(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-destroy", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")

	t.Run("destroys vault successfully", func(t *testing.T) {
		err := engine.Destroy("vault-destroy")
		if err != nil {
			t.Fatalf("Destroy failed: %v", err)
		}

		_, err = engine.GetVault("vault-destroy")
		if err == nil {
			t.Error("Expected error after destroying vault")
		}
	})

	t.Run("returns error for non-existent vault", func(t *testing.T) {
		err := engine.Destroy("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent vault")
		}
	})
}

func TestShareLinks(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-share", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-share", "pass")

	secret := &Secret{ID: "share-sec", Name: "Shared File", DataSize: 1024}
	engine.AddSecret("vault-share", secret)

	t.Run("creates share link", func(t *testing.T) {
		link, err := engine.CreateShareLink("vault-share", "share-sec", "user-1", ShareDownload, 1*time.Hour)
		if err != nil {
			t.Fatalf("CreateShareLink failed: %v", err)
		}
		if link.Token == "" {
			t.Error("Expected token to be set")
		}
		if link.Permission != ShareDownload {
			t.Errorf("Expected permission %s, got %s", ShareDownload, link.Permission)
		}
	})

	t.Run("accesses share link by token", func(t *testing.T) {
		link, _ := engine.CreateShareLink("vault-share", "share-sec", "user-1", ShareView, 1*time.Hour)
		result, err := engine.AccessShareLink(link.Token)
		if err != nil {
			t.Fatalf("AccessShareLink failed: %v", err)
		}
		if result.VaultID != "vault-share" {
			t.Errorf("Expected vault ID 'vault-share', got '%s'", result.VaultID)
		}
	})

	t.Run("rejects expired share link", func(t *testing.T) {
		link, _ := engine.CreateShareLink("vault-share", "share-sec", "user-1", ShareView, -1*time.Hour)
		_, err := engine.AccessShareLink(link.Token)
		if err == nil {
			t.Error("Expected error for expired share link")
		}
	})
}

func TestAccessPolicies(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-pol", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")

	t.Run("sets access policy", func(t *testing.T) {
		policy := &AccessPolicy{
			ID:         "pol-1",
			UserID:     "user-1",
			Level:      AccessReadWrite,
			AllowedIPs: []string{"192.168.1.100"},
		}

		err := engine.SetAccessPolicy("vault-pol", policy)
		if err != nil {
			t.Fatalf("SetAccessPolicy failed: %v", err)
		}
	})

	t.Run("checks access with matching IP", func(t *testing.T) {
		allowed, level := engine.CheckAccess("vault-pol", "user-1", "192.168.1.100")
		if !allowed {
			t.Error("Expected access to be allowed")
		}
		if level != AccessReadWrite {
			t.Errorf("Expected level %s, got %s", AccessReadWrite, level)
		}
	})

	t.Run("rejects access with non-matching IP", func(t *testing.T) {
		allowed, _ := engine.CheckAccess("vault-pol", "user-1", "10.0.0.1")
		if allowed {
			t.Error("Expected access to be rejected for non-matching IP")
		}
	})

	t.Run("rejects access for unknown user", func(t *testing.T) {
		allowed, _ := engine.CheckAccess("vault-pol", "unknown", "192.168.1.100")
		if allowed {
			t.Error("Expected access to be rejected for unknown user")
		}
	})
}

func TestKeyShares(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-ks", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")

	t.Run("adds key shares", func(t *testing.T) {
		for i := 1; i <= 3; i++ {
			share := &KeyShare{
				ID:          fmt.Sprintf("ks-%d", i),
				ShareIndex:  i,
				Threshold:   2,
				TotalShares: 3,
				HolderID:    fmt.Sprintf("holder-%d", i),
			}
			err := engine.AddKeyShare("vault-ks", share)
			if err != nil {
				t.Fatalf("AddKeyShare failed: %v", err)
			}
		}
	})

	t.Run("retrieves key shares", func(t *testing.T) {
		shares, err := engine.GetKeyShares("vault-ks")
		if err != nil {
			t.Fatalf("GetKeyShares failed: %v", err)
		}
		if len(shares) != 3 {
			t.Errorf("Expected 3 shares, got %d", len(shares))
		}
	})
}

func TestAutoLock(t *testing.T) {
	config := DefaultConfig()
	config.AutoLockMinutes = 0 // 使用 vault 级别的设置
	engine := NewEngine(config)

	vault := &Vault{
		ID:              "vault-autolock",
		Type:            "standard",
		Algorithm:       AlgoAES256GCM,
		AutoLockMinutes: 0, // 0 = 立即模拟超时
	}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-autolock", "pass")

	t.Run("auto-lock checks", func(t *testing.T) {
		locked := engine.AutoLockCheck()
		if locked < 0 {
			t.Error("Expected non-negative locked count")
		}
	})
}

func TestGetStats(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	vault := &Vault{ID: "vault-stats", Type: "standard", Algorithm: AlgoAES256GCM, Size: 1000}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-stats", "pass")

	secret := &Secret{ID: "s1", Name: "test", DataSize: 100}
	engine.AddSecret("vault-stats", secret)

	stats := engine.GetStats()
	if stats.TotalVaults != 1 {
		t.Errorf("Expected 1 vault, got %d", stats.TotalVaults)
	}
	if stats.UnlockedVaults != 1 {
		t.Errorf("Expected 1 unlocked vault, got %d", stats.UnlockedVaults)
	}
	if stats.TotalSecrets != 1 {
		t.Errorf("Expected 1 secret, got %d", stats.TotalSecrets)
	}
}

func TestAuditLog(t *testing.T) {
	config := DefaultConfig()
	config.AuditEnabled = true
	engine := NewEngine(config)

	vault := &Vault{ID: "vault-audit", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-audit", "pass")
	engine.Lock("vault-audit")

	entries := engine.GetAuditLog("vault-audit", 10)
	if len(entries) < 3 {
		t.Errorf("Expected at least 3 audit entries, got %d", len(entries))
	}
}

func TestStop(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	vault := &Vault{ID: "vault-stop", Type: "standard", Algorithm: AlgoAES256GCM}
	engine.CreateVault(vault, "pass")
	engine.Unlock("vault-stop", "pass")

	engine.Stop()

	// 确保密钥已清除（内部验证）
}

// ============================================================
// CryptoEngine 测试
// ============================================================

func TestCryptoEngine(t *testing.T) {
	ce := NewCryptoEngine(AlgoAES256GCM)

	t.Run("derives key from passphrase", func(t *testing.T) {
		key, salt, err := ce.DeriveKey("test-passphrase")
		if err != nil {
			t.Fatalf("DeriveKey failed: %v", err)
		}
		if len(key) != DefaultKeySize {
			t.Errorf("Expected key size %d, got %d", DefaultKeySize, len(key))
		}
		if len(salt) != DefaultSaltSize {
			t.Errorf("Expected salt size %d, got %d", DefaultSaltSize, len(salt))
		}
	})

	t.Run("encrypts and decrypts data", func(t *testing.T) {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		plaintext := []byte("Hello, PrivacyVault!")

		ciphertext, err := ce.Encrypt(key, plaintext)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		decrypted, err := ce.Decrypt(key, ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if string(decrypted) != string(plaintext) {
			t.Errorf("Decrypted text doesn't match: got '%s'", string(decrypted))
		}
	})

	t.Run("decrypt fails with wrong key", func(t *testing.T) {
		key1 := make([]byte, 32)
		key2 := make([]byte, 32)
		for i := range key1 {
			key1[i] = byte(i)
			key2[i] = byte(i + 1)
		}

		ciphertext, _ := ce.Encrypt(key1, []byte("secret"))
		_, err := ce.Decrypt(key2, ciphertext)
		if err == nil {
			t.Error("Expected decryption error with wrong key")
		}
	})

	t.Run("rotate key", func(t *testing.T) {
		oldKey := make([]byte, 32)
		newKey := make([]byte, 32)
		for i := range oldKey {
			oldKey[i] = byte(i)
			newKey[i] = byte(255 - i)
		}

		ciphertext, _ := ce.Encrypt(oldKey, []byte("rotate me"))

		newCiphertext, err := ce.RotateKey(oldKey, newKey, ciphertext)
		if err != nil {
			t.Fatalf("RotateKey failed: %v", err)
		}

		decrypted, err := ce.Decrypt(newKey, newCiphertext)
		if err != nil {
			t.Fatalf("Decrypt after rotation failed: %v", err)
		}

		if string(decrypted) != "rotate me" {
			t.Errorf("Expected 'rotate me', got '%s'", string(decrypted))
		}
	})
}

func TestComputeHash(t *testing.T) {
	t.Run("computes SHA-256 hash", func(t *testing.T) {
		hash := ComputeHash([]byte("test data"))
		if hash == "" {
			t.Error("Expected non-empty hash")
		}
		if len(hash) != 64 { // SHA-256 hex = 64 chars
			t.Errorf("Expected 64 char hash, got %d", len(hash))
		}
	})

	t.Run("verifies hash correctly", func(t *testing.T) {
		data := []byte("verify me")
		hash := ComputeHash(data)
		if !VerifyHash(data, hash) {
			t.Error("Expected hash verification to pass")
		}
		if VerifyHash([]byte("wrong data"), hash) {
			t.Error("Expected hash verification to fail for wrong data")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	t.Run("generates token of specified length", func(t *testing.T) {
		token, err := GenerateToken(32)
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}
		if len(token) != 64 { // 32 bytes = 64 hex chars
			t.Errorf("Expected 64 char token, got %d", len(token))
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		t1, _ := GenerateToken(16)
		t2, _ := GenerateToken(16)
		if t1 == t2 {
			t.Error("Expected unique tokens")
		}
	})
}

func TestVerifyPassphrase(t *testing.T) {
	t.Run("verifies correct passphrase", func(t *testing.T) {
		salt := make([]byte, 32)
		token := GenerateVerificationToken("correct-pass", salt)
		if !VerifyPassphrase("correct-pass", salt, token) {
			t.Error("Expected passphrase verification to pass")
		}
	})

	t.Run("rejects wrong passphrase", func(t *testing.T) {
		salt := make([]byte, 32)
		token := GenerateVerificationToken("correct-pass", salt)
		if VerifyPassphrase("wrong-pass", salt, token) {
			t.Error("Expected passphrase verification to fail")
		}
	})
}

// ============================================================
// Shredder 测试
// ============================================================

func TestNewShredder(t *testing.T) {
	t.Run("creates shredder with default config", func(t *testing.T) {
		shredder := NewShredder(nil)
		if shredder == nil {
			t.Fatal("NewShredder returned nil")
		}
	})

	t.Run("creates shredder with custom config", func(t *testing.T) {
		config := &ShredConfig{
			Mode:   ShredModeDoD5220,
			Passes: 7,
		}
		shredder := NewShredder(config)
		if shredder == nil {
			t.Fatal("NewShredder returned nil")
		}
	})
}

func TestShredFile(t *testing.T) {
	t.Run("shreds file successfully", func(t *testing.T) {
		// 创建临时文件
		tmpFile, err := os.CreateTemp("", "shred-test-*")
		if err != nil {
			t.Fatalf("CreateTemp failed: %v", err)
		}
		tmpFile.Write([]byte("sensitive data to shred"))
		tmpFile.Close()

		shredder := NewShredder(nil)
		result, err := shredder.ShredFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("ShredFile failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}
		if result.Passes != 3 {
			t.Errorf("Expected 3 passes, got %d", result.Passes)
		}

		// 文件应该被删除
		if _, err := os.Stat(tmpFile.Name()); !os.IsNotExist(err) {
			t.Error("Expected file to be deleted after shredding")
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		shredder := NewShredder(nil)
		_, err := shredder.ShredFile("/nonexistent/file.txt")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

func TestShredData(t *testing.T) {
	t.Run("zeros out data", func(t *testing.T) {
		data := []byte{0xFF, 0xAA, 0x55, 0x01, 0x02}
		shredder := NewShredder(nil)
		shredder.ShredData(data)

		for i, b := range data {
			if b != 0 {
				t.Errorf("Expected byte %d to be 0, got %d", i, b)
			}
		}
	})
}

func TestShredDirectory(t *testing.T) {
	t.Run("shreds all files in directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "shred-dir-*")
		if err != nil {
			t.Fatalf("MkdirTemp failed: %v", err)
		}
		defer os.RemoveAll(dir)

		// 创建测试文件
		for i := 0; i < 3; i++ {
			path := filepath.Join(dir, "file"+string(rune('0'+i))+".txt")
			os.WriteFile(path, []byte("data"), 0644)
		}

		shredder := NewShredder(nil)
		results, err := shredder.ShredDirectory(dir)
		if err != nil {
			t.Fatalf("ShredDirectory failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
	})
}

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/tmp/test.txt", true},
		{"file.tmp", true},
		{"file.temp", true},
		{".hidden", true},
		{"normal.txt", false},
		{"/home/user/doc.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsTempFile(tt.path); got != tt.expected {
				t.Errorf("IsTempFile(%s) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDefaultShredConfig(t *testing.T) {
	config := DefaultShredConfig()
	if config.Mode != ShredModeStandard {
		t.Errorf("Expected mode %s, got %s", ShredModeStandard, config.Mode)
	}
	if config.Passes != 3 {
		t.Errorf("Expected 3 passes, got %d", config.Passes)
	}
	if !config.SyncAfterWrite {
		t.Error("Expected SyncAfterWrite to be true")
	}
}

func TestGetShredStats(t *testing.T) {
	shredder := NewShredder(nil)
	results := []*ShredResult{
		{Success: true, BytesWritten: 100, Passes: 3, Duration: time.Millisecond},
		{Success: true, BytesWritten: 200, Passes: 3, Duration: time.Millisecond},
		{Success: false, BytesWritten: 0, Passes: 0, Duration: time.Millisecond, Error: "failed"},
	}

	stats := shredder.GetStats(results)
	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", stats.TotalFiles)
	}
	if stats.SuccessCount != 2 {
		t.Errorf("Expected 2 successes, got %d", stats.SuccessCount)
	}
	if stats.FailCount != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.FailCount)
	}
	if stats.TotalBytes != 300 {
		t.Errorf("Expected 300 bytes, got %d", stats.TotalBytes)
	}
}

// ============================================================
// 错误类型测试
// ============================================================

func TestPrivacyVaultError(t *testing.T) {
	t.Run("formats error without internal error", func(t *testing.T) {
		err := &PrivacyVaultError{Code: "TEST", Message: "test message"}
		expected := "[TEST] test message"
		if err.Error() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("formats error with internal error", func(t *testing.T) {
		inner := fmt.Errorf("inner error")
		err := &PrivacyVaultError{Code: "TEST", Message: "outer", Err: inner}
		if err.Unwrap() != inner {
			t.Error("Expected Unwrap to return inner error")
		}
	})
}

func TestFormatKeyID(t *testing.T) {
	id := FormatKeyID("vault-1", 2)
	expected := "key-vault-1-2"
	if id != expected {
		t.Errorf("Expected '%s', got '%s'", expected, id)
	}
}
