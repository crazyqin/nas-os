// Package drivesync 单元测试
package drivesync

import (
	"context"
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager("/tmp/drivesync-test")
}

// ========== 同步任务测试 ==========

func TestNewManager(t *testing.T) {
	m := newTestManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestSetPolicy(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	policy := SyncPolicy{
		AutoSync:     true,
		SyncInterval: 5 * time.Minute,
		ConflictMode: "ask",
		MaxFileSize:  10 * 1024 * 1024 * 1024,
	}

	err := m.SetPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("SetPolicy failed: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
}

func TestListConflicts(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	conflicts, err := m.ListConflicts(ctx)
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if conflicts == nil {
		t.Fatal("ListConflicts returned nil")
	}
}

func TestExport(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	data, err := m.Export(ctx)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if data == nil {
		t.Fatal("Export returned nil")
	}
}
