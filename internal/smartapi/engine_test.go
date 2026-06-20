package smartapi

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestCreateAPIKey(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	key := &APIKey{
		ID:      "key-1",
		Name:    "test-key",
		Key:     "sk-test-123",
		Scopes:  []string{"read", "write"},
		Enabled: true,
	}

	if err := engine.CreateAPIKey(key); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := engine.GetAPIKey("key-1")
	if !ok {
		t.Fatal("expected key to be registered")
	}
	if got.Name != "test-key" {
		t.Errorf("expected name 'test-key', got '%s'", got.Name)
	}
}

func TestValidateAPIKey(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.CreateAPIKey(&APIKey{
		ID:      "key-1",
		Key:     "sk-valid",
		Enabled: true,
	})

	key, err := engine.ValidateAPIKey("sk-valid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-1" {
		t.Errorf("expected key ID 'key-1', got '%s'", key.ID)
	}
}

func TestValidateAPIKeyExpired(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.CreateAPIKey(&APIKey{
		ID:        "key-1",
		Key:       "sk-expired",
		Enabled:   true,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	_, err := engine.ValidateAPIKey("sk-expired")
	if err != ErrKeyExpired {
		t.Errorf("expected ErrKeyExpired, got %v", err)
	}
}

func TestRevokeAPIKey(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.CreateAPIKey(&APIKey{
		ID:      "key-1",
		Key:     "sk-revoke",
		Enabled: true,
	})

	if err := engine.RevokeAPIKey("key-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key, _ := engine.GetAPIKey("key-1")
	if key.Enabled {
		t.Error("expected key to be disabled")
	}
}

func TestGetGatewayStats(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.CreateAPIKey(&APIKey{ID: "k1", Enabled: true})
	engine.CreateAPIKey(&APIKey{ID: "k2", Enabled: false})

	stats := engine.GetGatewayStats()

	if stats["total_keys"] != 2 {
		t.Errorf("expected 2 total keys, got %v", stats["total_keys"])
	}
	if stats["active_keys"] != 1 {
		t.Errorf("expected 1 active key, got %v", stats["active_keys"])
	}
}
