package apikeymgr

import (
	"testing"
)

func TestNewAPIKeyManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewAPIKeyManager(cfg)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestCreateKey(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())

	req := CreateKeyRequest{
		UserID:      "user-001",
		Name:        "test-key",
		Permissions: []Permission{PermRead, PermWrite},
		ExpiresIn:   30,
		RateLimit:   100,
	}

	key, rawKey, err := mgr.CreateKey(req)
	if err != nil {
		t.Fatalf("create key failed: %v", err)
	}
	if key == nil || rawKey == "" {
		t.Fatal("key and rawKey should not be nil/empty")
	}
	if key.UserID != "user-001" {
		t.Errorf("expected user-001, got %s", key.UserID)
	}
	if key.Status != StatusActive {
		t.Errorf("expected active, got %s", key.Status)
	}
	if key.ExpiresAt == nil {
		t.Error("expires_at should be set")
	}
}

func TestValidateKey(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())

	_, rawKey, _ := mgr.CreateKey(CreateKeyRequest{
		UserID:      "user-001",
		Name:        "valid-key",
		Permissions: []Permission{PermRead},
	})

	validated, err := mgr.ValidateKey(rawKey)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if validated.Name != "valid-key" {
		t.Errorf("expected valid-key, got %s", validated.Name)
	}
	if validated.UsageCount != 1 {
		t.Errorf("expected usage count 1, got %d", validated.UsageCount)
	}
}

func TestValidateInvalidKey(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())
	_, err := mgr.ValidateKey("invalid-key")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestRevokeKey(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())

	key, rawKey, _ := mgr.CreateKey(CreateKeyRequest{
		UserID: "user-001",
		Name:   "revoke-test",
	})

	if err := mgr.RevokeKey(key.ID); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	_, err := mgr.ValidateKey(rawKey)
	if err != ErrKeyRevoked {
		t.Errorf("expected ErrKeyRevoked, got %v", err)
	}
}

func TestMaxKeysPerUser(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxKeysPerUser = 2
	mgr := NewAPIKeyManager(cfg)

	for i := 0; i < 2; i++ {
		_, _, err := mgr.CreateKey(CreateKeyRequest{
			UserID: "user-001",
			Name:   "key",
		})
		if err != nil {
			t.Fatalf("create key %d failed: %v", i, err)
		}
	}

	_, _, err := mgr.CreateKey(CreateKeyRequest{
		UserID: "user-001",
		Name:   "extra-key",
	})
	if err != ErrMaxKeysReached {
		t.Errorf("expected ErrMaxKeysReached, got %v", err)
	}
}

func TestRotateKey(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())

	key, rawKey1, _ := mgr.CreateKey(CreateKeyRequest{
		UserID: "user-001",
		Name:   "rotate-test",
	})

	newKey, rawKey2, err := mgr.RotateKey(key.ID)
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if rawKey1 == rawKey2 {
		t.Error("new key should be different from old key")
	}
	if newKey.Status != StatusActive {
		t.Errorf("new key should be active, got %s", newKey.Status)
	}

	// 旧密钥应该被吊销
	_, err = mgr.ValidateKey(rawKey1)
	if err != ErrKeyRevoked {
		t.Errorf("old key should be revoked, got %v", err)
	}
}

func TestListUserKeys(t *testing.T) {
	mgr := NewAPIKeyManager(DefaultManagerConfig())

	mgr.CreateKey(CreateKeyRequest{UserID: "user-001", Name: "key1"})
	mgr.CreateKey(CreateKeyRequest{UserID: "user-001", Name: "key2"})
	mgr.CreateKey(CreateKeyRequest{UserID: "user-002", Name: "key3"})

	keys := mgr.ListUserKeys("user-001")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestCleanupExpired(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewAPIKeyManager(cfg)

	// 创建一个立即过期的密钥
	_, _, err := mgr.CreateKey(CreateKeyRequest{
		UserID:    "user-001",
		Name:      "expired-key",
		ExpiresIn: -1, // 已过期
	})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	count := mgr.CleanupExpired()
	if count != 1 {
		t.Errorf("expected 1 expired, got %d", count)
	}
}
