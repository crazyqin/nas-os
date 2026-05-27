package zfspool

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreatePool(t *testing.T) {
	m := NewManager()
	pool, err := m.CreatePool("tank", RaidTypeRaidz2, []string{"sda", "sdb", "sdc"})
	if err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}
	if pool.Name != "tank" {
		t.Errorf("expected name 'tank', got '%s'", pool.Name)
	}
	if pool.RaidType != RaidTypeRaidz2 {
		t.Errorf("expected raidz2, got '%s'", pool.RaidType)
	}
	if pool.Disks != 3 {
		t.Errorf("expected 3 disks, got %d", pool.Disks)
	}
}

func TestCreatePoolDuplicate(t *testing.T) {
	m := NewManager()
	m.CreatePool("tank", RaidTypeMirror, []string{"sda", "sdb"})
	_, err := m.CreatePool("tank", RaidTypeRaidz1, []string{"sdc", "sdd", "sde"})
	if err == nil {
		t.Fatal("expected error for duplicate pool")
	}
}

func TestGetPool(t *testing.T) {
	m := NewManager()
	m.CreatePool("tank", RaidTypeMirror, []string{"sda", "sdb"})
	pool, err := m.GetPool("tank")
	if err != nil {
		t.Fatalf("GetPool failed: %v", err)
	}
	if pool.Name != "tank" {
		t.Errorf("expected 'tank', got '%s'", pool.Name)
	}
}

func TestGetPoolNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetPool("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pool")
	}
}

func TestDeletePool(t *testing.T) {
	m := NewManager()
	m.CreatePool("tank", RaidTypeMirror, []string{"sda", "sdb"})
	err := m.DeletePool("tank")
	if err != nil {
		t.Fatalf("DeletePool failed: %v", err)
	}
	_, err = m.GetPool("tank")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestStartScrub(t *testing.T) {
	m := NewManager()
	m.CreatePool("tank", RaidTypeRaidz2, []string{"sda", "sdb", "sdc"})
	err := m.StartScrub("tank")
	if err != nil {
		t.Fatalf("StartScrub failed: %v", err)
	}
	pool, _ := m.GetPool("tank")
	if pool.ScrubStatus != "scrubbing" {
		t.Errorf("expected scrubbing, got '%s'", pool.ScrubStatus)
	}
}

func TestExpandPool(t *testing.T) {
	m := NewManager()
	m.CreatePool("tank", RaidTypeRaidz1, []string{"sda", "sdb", "sdc"})
	err := m.ExpandPool("tank", "sdd")
	if err != nil {
		t.Fatalf("ExpandPool failed: %v", err)
	}
	pool, _ := m.GetPool("tank")
	if pool.Disks != 4 {
		t.Errorf("expected 4 disks, got %d", pool.Disks)
	}
}

func TestCreateSnapshot(t *testing.T) {
	m := NewManager()
	snap, err := m.CreateSnapshot("tank/data", "backup-2026")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap.Dataset != "tank/data" {
		t.Errorf("expected 'tank/data', got '%s'", snap.Dataset)
	}
}

func TestGetPools(t *testing.T) {
	m := NewManager()
	m.CreatePool("pool1", RaidTypeMirror, []string{"sda", "sdb"})
	m.CreatePool("pool2", RaidTypeRaidz1, []string{"sdc", "sdd", "sde"})
	pools := m.GetPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}
