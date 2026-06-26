package snapshotmgr

import (
	"testing"

	"go.uber.org/zap"
)

func TestZFSSnapshotConfigNaming(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	config := &ZFSSnapshotConfig{
		Pool:         "tank",
		Dataset:      "tank/data",
		Recursive:    true,
		NamingFormat: "snap-20060102-150405",
	}

	name := zfs.GenerateSnapshotName(config)
	if name == "" {
		t.Error("expected non-empty snapshot name")
	}
	// Name should start with "snap-"
	if len(name) < 5 || name[:5] != "snap-" {
		t.Errorf("expected name starting with 'snap-', got %q", name)
	}
}

func TestZFSDefaultNamingFormat(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	config := &ZFSSnapshotConfig{
		Pool:    "tank",
		Dataset: "",
	}
	name := zfs.GenerateSnapshotName(config)
	if name == "" {
		t.Error("expected non-empty snapshot name with default format")
	}
}

func TestZFSCreateSnapshotCommand(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	// Non-recursive
	config := &ZFSSnapshotConfig{
		Pool:      "tank",
		Dataset:   "tank/data",
		Recursive: false,
	}
	args := zfs.CreateSnapshotCommand(config, "snap-20250615")
	expected := []string{"snapshot", "tank/data@snap-20250615"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i, v := range args {
		if v != expected[i] {
			t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], v)
		}
	}

	// Recursive
	config.Recursive = true
	args = zfs.CreateSnapshotCommand(config, "snap-20250615")
	if args[1] != "-r" {
		t.Errorf("expected -r flag for recursive, got %v", args)
	}

	// Pool-only (no dataset)
	config.Dataset = ""
	args = zfs.CreateSnapshotCommand(config, "snap-20250615")
	if args[len(args)-1] != "tank@snap-20250615" {
		t.Errorf("expected 'tank@snap-20250615', got %q", args[len(args)-1])
	}
}

func TestZFSCreateBookmark(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	bm, err := zfs.CreateBookmark("tank", "tank/data", "snap-20250615", "bm-daily")
	if err != nil {
		t.Fatalf("CreateBookmark failed: %v", err)
	}

	if bm.Name != "bm-daily" {
		t.Errorf("expected name 'bm-daily', got %q", bm.Name)
	}
	if bm.SnapName != "snap-20250615" {
		t.Errorf("expected snap_name 'snap-20250615', got %q", bm.SnapName)
	}
	if bm.Pool != "tank" {
		t.Errorf("expected pool 'tank', got %q", bm.Pool)
	}
	if bm.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestZFSDuplicateBookmark(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	_, _ = zfs.CreateBookmark("tank", "tank/data", "snap-1", "bm-test")

	_, err := zfs.CreateBookmark("tank", "tank/data", "snap-1", "bm-test")
	if err == nil {
		t.Error("expected error for duplicate bookmark")
	}
}

func TestZFSListBookmarks(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	zfs.CreateBookmark("tank", "tank/data", "snap-1", "bm-1")
	zfs.CreateBookmark("tank", "tank/backup", "snap-2", "bm-2")
	zfs.CreateBookmark("other", "other/data", "snap-3", "bm-3")

	// List all for pool "tank"
	bookmarks := zfs.ListBookmarks("tank", "")
	if len(bookmarks) != 2 {
		t.Errorf("expected 2 bookmarks for pool 'tank', got %d", len(bookmarks))
	}

	// List for specific dataset
	bookmarks = zfs.ListBookmarks("tank", "tank/data")
	if len(bookmarks) != 1 {
		t.Errorf("expected 1 bookmark for tank/data, got %d", len(bookmarks))
	}
}

func TestZFSDeleteBookmark(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	zfs.CreateBookmark("tank", "tank/data", "snap-1", "bm-test")

	err := zfs.DeleteBookmark("tank", "tank/data", "bm-test")
	if err != nil {
		t.Fatalf("DeleteBookmark failed: %v", err)
	}

	bookmarks := zfs.ListBookmarks("tank", "")
	if len(bookmarks) != 0 {
		t.Errorf("expected 0 bookmarks after delete, got %d", len(bookmarks))
	}

	// Delete nonexistent
	err = zfs.DeleteBookmark("tank", "tank/data", "bm-nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent bookmark")
	}
}

func TestZFSAddRemoveHold(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	snapID := "tank/data@snap-1"

	err := zfs.AddHold(snapID, "replication", "Prevent deletion during replication", "job-123")
	if err != nil {
		t.Fatalf("AddHold failed: %v", err)
	}

	holds := zfs.ListHolds(snapID)
	if len(holds) != 1 {
		t.Fatalf("expected 1 hold, got %d", len(holds))
	}
	if holds[0].Tag != "replication" {
		t.Errorf("expected tag 'replication', got %q", holds[0].Tag)
	}

	// Duplicate tag
	err = zfs.AddHold(snapID, "replication", "another reason", "job-456")
	if err == nil {
		t.Error("expected error for duplicate hold tag")
	}

	// Remove hold
	err = zfs.RemoveHold(snapID, "replication")
	if err != nil {
		t.Fatalf("RemoveHold failed: %v", err)
	}

	holds = zfs.ListHolds(snapID)
	if len(holds) != 0 {
		t.Errorf("expected 0 holds after remove, got %d", len(holds))
	}
}

func TestZFSIsHeld(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	snapID := "tank/data@snap-1"

	if zfs.IsHeld(snapID) {
		t.Error("expected not held initially")
	}

	zfs.AddHold(snapID, "keep", "important", "")

	if !zfs.IsHeld(snapID) {
		t.Error("expected held after adding hold")
	}
}

func TestZFSCanDelete(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	snapID := "tank/data@snap-1"

	canDelete, holds := zfs.CanDelete(snapID)
	if !canDelete {
		t.Error("expected can delete with no holds")
	}
	if len(holds) != 0 {
		t.Errorf("expected no holds, got %d", len(holds))
	}

	zfs.AddHold(snapID, "protect", "do not delete", "")

	canDelete, holds = zfs.CanDelete(snapID)
	if canDelete {
		t.Error("expected cannot delete with active hold")
	}
	if len(holds) != 1 {
		t.Errorf("expected 1 hold, got %d", len(holds))
	}
}

func TestZFSDiff(t *testing.T) {
	zfs := NewZFSSnapshotManager(zap.NewNop())

	result, err := zfs.Diff("tank", "tank/data", "snap-A", "snap-B")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if result.SnapshotA != "tank/data@snap-A" {
		t.Errorf("expected snapshot_a 'tank/data@snap-A', got %q", result.SnapshotA)
	}
	if result.SnapshotB != "tank/data@snap-B" {
		t.Errorf("expected snapshot_b 'tank/data@snap-B', got %q", result.SnapshotB)
	}
}

func TestParseZFSPath(t *testing.T) {
	tests := []struct {
		input      string
		expPool    string
		expDataset string
		expSuffix  string
		expType    string
	}{
		{"tank/data@snap-1", "tank", "data", "snap-1", "snapshot"},
		{"tank@snap-1", "tank", "", "snap-1", "snapshot"},
		{"tank/data#bm-1", "tank", "data", "bm-1", "bookmark"},
		{"tank", "tank", "", "", ""},
	}

	for _, tt := range tests {
		pool, dataset, suffix, suffixType := ParseZFSPath(tt.input)
		if pool != tt.expPool {
			t.Errorf("ParseZFSPath(%q): pool = %q, want %q", tt.input, pool, tt.expPool)
		}
		if dataset != tt.expDataset {
			t.Errorf("ParseZFSPath(%q): dataset = %q, want %q", tt.input, dataset, tt.expDataset)
		}
		if suffix != tt.expSuffix {
			t.Errorf("ParseZFSPath(%q): suffix = %q, want %q", tt.input, suffix, tt.expSuffix)
		}
		if suffixType != tt.expType {
			t.Errorf("ParseZFSPath(%q): suffixType = %q, want %q", tt.input, suffixType, tt.expType)
		}
	}
}
