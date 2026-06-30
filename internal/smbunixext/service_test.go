package smbunixext

import (
	"testing"
	"time"
)

func TestSetAndGetExtension(t *testing.T) {
	m := NewExtensionManager()
	req := &SetExtensionRequest{
		ShareName: "share1",
		Enabled:   true,
	}
	cfg, err := m.SetExtension(req)
	if err != nil {
		t.Fatalf("SetExtension failed: %v", err)
	}
	if cfg.ShareName != "share1" {
		t.Errorf("expected share name 'share1', got %q", cfg.ShareName)
	}
	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if !cfg.IsMultiProtocol {
		t.Error("expected is_multi_protocol to be true when enabled")
	}
	if cfg.Protocol != ProtocolMulti {
		t.Errorf("expected protocol 'multi', got %q", cfg.Protocol)
	}
	if len(cfg.Capabilities) == 0 {
		t.Error("expected capabilities to be set")
	}
	if cfg.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}

	got, err := m.GetExtension("share1")
	if err != nil {
		t.Fatalf("GetExtension failed: %v", err)
	}
	if got.ShareName != "share1" {
		t.Errorf("expected share name 'share1', got %q", got.ShareName)
	}
}

func TestSetExtensionNilRequest(t *testing.T) {
	m := NewExtensionManager()
	if _, err := m.SetExtension(nil); err == nil {
		t.Error("expected error for nil request")
	}
}

func TestSetExtensionEmptyShareName(t *testing.T) {
	m := NewExtensionManager()
	req := &SetExtensionRequest{Enabled: true}
	if _, err := m.SetExtension(req); err == nil {
		t.Error("expected error for empty share name")
	}
}

func TestSetExtensionDisabled(t *testing.T) {
	m := NewExtensionManager()
	req := &SetExtensionRequest{
		ShareName: "share2",
		Enabled:   false,
	}
	cfg, err := m.SetExtension(req)
	if err != nil {
		t.Fatalf("SetExtension failed: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected enabled to be false")
	}
	if cfg.IsMultiProtocol {
		t.Error("expected is_multi_protocol to be false when disabled")
	}
}

func TestGetExtensionNotFound(t *testing.T) {
	m := NewExtensionManager()
	if _, err := m.GetExtension("nonexistent"); err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestGetExtensionStatus(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})

	status, err := m.GetExtensionStatus("share1")
	if err != nil {
		t.Fatalf("GetExtensionStatus failed: %v", err)
	}
	if status.Status != ExtensionStatusEnabled {
		t.Errorf("expected status 'enabled', got %q", status.Status)
	}
	if !status.IsMultiProtocol {
		t.Error("expected is_multi_protocol to be true")
	}
}

func TestGetExtensionStatusNotFound(t *testing.T) {
	m := NewExtensionManager()
	if _, err := m.GetExtensionStatus("nonexistent"); err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestListExtensions(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	m.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})

	configs := m.ListExtensions()
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
}

func TestListExtensionsEmpty(t *testing.T) {
	m := NewExtensionManager()
	configs := m.ListExtensions()
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
}

func TestRemoveExtension(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	m.RemoveExtension("s1")
	if _, err := m.GetExtension("s1"); err == nil {
		t.Error("expected error after remove")
	}
}

func TestIsMultiProtocol(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	if !m.IsMultiProtocol("s1") {
		t.Error("expected IsMultiProtocol to be true for enabled share")
	}

	m.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})
	if m.IsMultiProtocol("s2") {
		t.Error("expected IsMultiProtocol to be false for disabled share")
	}

	if m.IsMultiProtocol("nonexistent") {
		t.Error("expected IsMultiProtocol to be false for nonexistent share")
	}
}

func TestCanEnableUnixExtensions(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})

	can, err := m.CanEnableUnixExtensions("s1")
	if err != nil {
		t.Fatalf("CanEnableUnixExtensions failed: %v", err)
	}
	if !can {
		t.Error("expected CanEnableUnixExtensions to be true for multi-protocol share")
	}
}

func TestCanEnableUnixExtensionsNotFound(t *testing.T) {
	m := NewExtensionManager()
	if _, err := m.CanEnableUnixExtensions("nonexistent"); err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestSaveAndLoadFromDB(t *testing.T) {
	m := NewExtensionManager()
	m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})

	if err := m.SaveToDB("s1"); err != nil {
		t.Fatalf("SaveToDB failed: %v", err)
	}

	cfg, err := m.LoadFromDB("s1")
	if err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}
	if cfg.ShareName != "s1" {
		t.Errorf("expected share name 's1', got %q", cfg.ShareName)
	}
}

func TestSaveToDBNotFound(t *testing.T) {
	m := NewExtensionManager()
	if err := m.SaveToDB("nonexistent"); err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestLoadFromDBNotFound(t *testing.T) {
	m := NewExtensionManager()
	if _, err := m.LoadFromDB("nonexistent"); err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestExtensionStatusValues(t *testing.T) {
	if ExtensionStatusEnabled != "enabled" {
		t.Errorf("expected 'enabled', got %q", ExtensionStatusEnabled)
	}
	if ExtensionStatusDisabled != "disabled" {
		t.Errorf("expected 'disabled', got %q", ExtensionStatusDisabled)
	}
}

func TestUpdatedAtSet(t *testing.T) {
	m := NewExtensionManager()
	before := time.Now()
	cfg, _ := m.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	if !cfg.UpdatedAt.After(before.Add(-1 * time.Second)) {
		t.Error("expected updated_at to be recent")
	}
}