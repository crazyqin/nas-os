package iscsiblockclone

import (
	"testing"
	"time"
)

func TestNewBlockCloneManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewBlockCloneManager(cfg)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestLUNCRUD(t *testing.T) {
	mgr := NewBlockCloneManager(DefaultManagerConfig())

	lun := &LUNInfo{
		ID:        "lun-001",
		Name:      "test-lun",
		SizeBytes: 1024 * 1024 * 1024,
		BlockSize: 4096,
		Protocol:  "iscsi",
		TargetIQN: "iqn.2026-01.test:target0",
		CreatedAt: time.Now(),
	}

	if err := mgr.RegisterLUN(lun); err != nil {
		t.Fatalf("register LUN failed: %v", err)
	}
	if err := mgr.RegisterLUN(lun); err != ErrLUNExists {
		t.Errorf("expected ErrLUNExists, got %v", err)
	}

	got, err := mgr.GetLUN("lun-001")
	if err != nil {
		t.Fatalf("get LUN failed: %v", err)
	}
	if got.Name != "test-lun" {
		t.Errorf("expected name test-lun, got %s", got.Name)
	}

	luns := mgr.ListLUNs()
	if len(luns) != 1 {
		t.Errorf("expected 1 LUN, got %d", len(luns))
	}

	if err := mgr.UnregisterLUN("lun-001"); err != nil {
		t.Fatalf("unregister LUN failed: %v", err)
	}
	if _, err := mgr.GetLUN("lun-001"); err != ErrLUNNotFound {
		t.Errorf("expected ErrLUNNotFound, got %v", err)
	}
}

func TestCloneLUN(t *testing.T) {
	mgr := NewBlockCloneManager(DefaultManagerConfig())

	lun := &LUNInfo{
		ID:        "lun-source",
		Name:      "source-lun",
		SizeBytes: 512 * 1024 * 1024,
		BlockSize: 4096,
		Protocol:  "iscsi",
		TargetIQN: "iqn.2026-01.test:source",
		CreatedAt: time.Now(),
	}
	mgr.RegisterLUN(lun)

	task, err := mgr.CloneLUN("lun-source", "clone-001", CloneLinked)
	if err != nil {
		t.Fatalf("clone LUN failed: %v", err)
	}
	if task == nil {
		t.Fatal("task should not be nil")
	}
	if task.SourceLUN != "lun-source" {
		t.Errorf("expected source lun-source, got %s", task.SourceLUN)
	}

	// 等待克隆完成
	time.Sleep(200 * time.Millisecond)

	task, err = mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if task.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestCloneNonexistentLUN(t *testing.T) {
	mgr := NewBlockCloneManager(DefaultManagerConfig())
	_, err := mgr.CloneLUN("nonexistent", "target", CloneFull)
	if err != ErrLUNNotFound {
		t.Errorf("expected ErrLUNNotFound, got %v", err)
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewBlockCloneManager(DefaultManagerConfig())
	stats := mgr.GetStats()
	if stats.TotalClones != 0 {
		t.Errorf("expected 0 total clones, got %d", stats.TotalClones)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultManagerConfig()
	if cfg.DefaultCloneType != CloneLinked {
		t.Errorf("expected linked clone, got %s", cfg.DefaultCloneType)
	}
	if cfg.MaxConcurrentClones != 4 {
		t.Errorf("expected 4 concurrent, got %d", cfg.MaxConcurrentClones)
	}
}
