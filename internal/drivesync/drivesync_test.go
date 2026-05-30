// Package drivesync 测试文件
package drivesync

import (
	"context"
	"testing"
)

func TestManager_GetStats(t *testing.T) {
	m := NewManager("/tmp/test-drive")
	ctx := context.Background()

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["total_files"] != 0 {
		t.Errorf("Expected 0 files, got %v", stats["total_files"])
	}
}

func TestManager_ListConflicts(t *testing.T) {
	m := NewManager("/tmp/test-drive")
	ctx := context.Background()

	conflicts, err := m.ListConflicts(ctx)
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(conflicts))
	}
}
