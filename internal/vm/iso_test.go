package vm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestISOManager_NewISOManager(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("ISOManager should not be nil")
	}
	if mgr.isoPath == "" {
		t.Error("isoPath should not be empty")
	}
	if mgr.isos == nil {
		t.Error("isos map should be initialized")
	}
}

func TestISOManager_NewISOManager_EmptyPath(t *testing.T) {
	// Should use default path when empty
	mgr, err := NewISOManager("", nil)
	if err != nil {
		// May fail if default path doesn't exist, that's OK
		t.Logf("NewISOManager with empty path: %v", err)
		return
	}
	if mgr.isoPath != DefaultISOStoragePath {
		t.Errorf("isoPath should be default, got: %s", mgr.isoPath)
	}
}

func TestISOManager_ListISOs(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	isos := mgr.ListISOs()
	if isos == nil {
		t.Error("ListISOs should not return nil")
	}

	// Should have built-in ISOs
	if len(isos) == 0 {
		t.Error("ListISOs should return built-in ISOs")
	}
}

func TestISOManager_GetISO(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	// Get built-in ISO
	iso, err := mgr.GetISO("ubuntu-2204-lts")
	if err != nil {
		t.Errorf("GetISO should find built-in ISO: %v", err)
	}
	if iso == nil {
		t.Fatal("ISO should not be nil")
	}
	if iso.Name != "Ubuntu 22.04 LTS" {
		t.Errorf("ISO name mismatch: %s", iso.Name)
	}
}

func TestISOManager_GetISO_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	_, err = mgr.GetISO("nonexistent")
	if err == nil {
		t.Error("GetISO should return error for non-existent ISO")
	}
}

func TestISOManager_UploadISO(t *testing.T) {
	tmpDir := t.TempDir()

	logger := zap.NewNop()
	mgr, err := NewISOManager(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	// Create test content
	content := []byte("test iso content")
	reader := bytes.NewReader(content)

	iso, err := mgr.UploadISO(context.Background(), "test-iso", reader)
	if err != nil {
		t.Fatalf("UploadISO failed: %v", err)
	}
	if iso == nil {
		t.Fatal("ISO should not be nil")
	}
	if iso.Name == "" {
		t.Error("ISO name should not be empty")
	}
	if iso.Size != uint64(len(content)) {
		t.Errorf("ISO size mismatch: %d", iso.Size)
	}
	if !iso.IsUploaded {
		t.Error("ISO should be marked as uploaded")
	}
}

func TestISOManager_DeleteISO(t *testing.T) {
	tmpDir := t.TempDir()

	logger := zap.NewNop()
	mgr, err := NewISOManager(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	// Upload an ISO first
	content := []byte("test iso content")
	reader := bytes.NewReader(content)
	iso, err := mgr.UploadISO(context.Background(), "test-iso", reader)
	if err != nil {
		t.Fatalf("UploadISO failed: %v", err)
	}

	// Delete it
	err = mgr.DeleteISO(iso.ID)
	if err != nil {
		t.Fatalf("DeleteISO failed: %v", err)
	}

	// Verify it's deleted
	_, err = mgr.GetISO(iso.ID)
	if err == nil {
		t.Error("ISO should be deleted")
	}
}

func TestISOManager_DeleteISO_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	err = mgr.DeleteISO("nonexistent")
	if err == nil {
		t.Error("DeleteISO should return error for non-existent ISO")
	}
}

func TestISOManager_DeleteISO_BuiltIn(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	// Try to delete built-in ISO (should fail)
	err = mgr.DeleteISO("ubuntu-2204-lts")
	if err == nil {
		t.Error("DeleteISO should fail for built-in ISOs")
	}
}

func TestISOManager_LoadISOs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an ISO file first
	isoFile := filepath.Join(tmpDir, "test.iso")
	err := os.WriteFile(isoFile, []byte("test iso content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test ISO: %v", err)
	}

	// Create manager - it should load existing ISOs
	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	// Find the uploaded ISO
	isos := mgr.ListISOs()
	found := false
	for _, iso := range isos {
		if iso.Name == "test.iso" && iso.IsUploaded {
			found = true
			break
		}
	}

	if !found {
		t.Error("ListISOs should include the existing ISO file")
	}
}

func TestISOManager_ISOFields(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewISOManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewISOManager failed: %v", err)
	}

	iso, err := mgr.GetISO("ubuntu-2204-lts")
	if err != nil {
		t.Fatalf("GetISO failed: %v", err)
	}

	// Check all fields are set
	if iso.ID == "" {
		t.Error("ISO ID should not be empty")
	}
	if iso.Name == "" {
		t.Error("ISO Name should not be empty")
	}
	if iso.OS == "" {
		t.Error("ISO OS should not be empty for built-in ISOs")
	}
}
