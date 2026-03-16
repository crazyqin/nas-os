package vm

import (
	"context"
	"testing"
)

func TestIsSafeString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"safe_name-123", true},
		{"Safe Name", true},
		{"unsafe;name", false},
		{"unsafe|name", false},
		{"unsafe&name", false},
		{"unsafe$name", false},
		{"unsafe`name", false},
		{"unsafe(name)", false},
		{"unsafe<name>", false},
		{"", true}, // Empty string is safe
		{"test123", true},
		{"TEST_123-ABC", true},
	}

	for _, tt := range tests {
		result := isSafeString(tt.input)
		if result != tt.expected {
			t.Errorf("isSafeString(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"safe description", "safe description"},
		{"with;semicolon", "withsemicolon"},
		{"with|pipe", "withpipe"},
		{"with&ampersand", "withampersand"},
		{"with$dollar", "withdollar"},
		{"with`backtick", "withbacktick"},
		{"with(parens)", "withparens"},
		{"with<angle>brackets", "withanglebrackets"},
		{"multi\nline", "multi line"},
		{"carriage\rreturn", "carriagereturn"},
	}

	for _, tt := range tests {
		result := sanitizeDescription(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeDescription(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSnapshotManager_NewSnapshotManager(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a VM manager first
	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}
	if snapMgr == nil {
		t.Fatal("SnapshotManager should not be nil")
	}
	if snapMgr.storagePath == "" {
		t.Error("storagePath should not be empty")
	}
	if snapMgr.snapshots == nil {
		t.Error("snapshots map should be initialized")
	}
}

func TestSnapshotManager_ListSnapshots(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	// List snapshots for non-existent VM
	snapshots := snapMgr.ListSnapshots("nonexistent")
	if snapshots == nil {
		t.Error("ListSnapshots should not return nil")
	}
	if len(snapshots) != 0 {
		t.Error("ListSnapshots should return empty list for non-existent VM")
	}
}

func TestSnapshotManager_GetSnapshot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	_, err = snapMgr.GetSnapshot("nonexistent")
	if err == nil {
		t.Error("GetSnapshot should return error for non-existent snapshot")
	}
}

func TestSnapshotManager_CreateSnapshot_InvalidName(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	// Try to create snapshot with unsafe name
	_, err = snapMgr.CreateSnapshot(context.Background(), "nonexistent", "invalid;name", "description")
	if err == nil {
		t.Error("CreateSnapshot should fail with unsafe name")
	}
}

func TestSnapshotManager_DeleteSnapshot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	err = snapMgr.DeleteSnapshot(context.Background(), "nonexistent")
	if err == nil {
		t.Error("DeleteSnapshot should return error for non-existent snapshot")
	}
}

func TestSnapshotManager_RestoreSnapshot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	err = snapMgr.RestoreSnapshot(context.Background(), "nonexistent")
	if err == nil {
		t.Error("RestoreSnapshot should return error for non-existent snapshot")
	}
}

func TestSnapshotManager_CreateSnapshot_VMNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	vmMgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	snapMgr, err := NewSnapshotManager(tmpDir, vmMgr, nil)
	if err != nil {
		t.Fatalf("NewSnapshotManager failed: %v", err)
	}

	_, err = snapMgr.CreateSnapshot(context.Background(), "nonexistent", "safe_name", "description")
	if err == nil {
		t.Error("CreateSnapshot should fail for non-existent VM")
	}
}