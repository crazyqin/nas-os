// Package drivemigration 测试
package drivemigration

import (
	"testing"
)

func TestNewMigrationManager(t *testing.T) {
	m := NewMigrationManager()
	if m == nil {
		t.Fatal("NewMigrationManager returned nil")
	}
}

func TestRegisterDisk(t *testing.T) {
	m := NewMigrationManager()
	disk := &Disk{
		Device:    "/dev/sda",
		Model:     "WD Red 8TB",
		Serial:    "WD123456",
		Size:      8000000000000,
		Interface: "SATA",
		IsSSD:     false,
	}
	if err := m.RegisterDisk(disk); err != nil {
		t.Fatalf("RegisterDisk failed: %v", err)
	}
	if disk.ID == "" {
		t.Fatal("disk ID not generated")
	}

	disks := m.ListDisks()
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
}

func TestCreatePool(t *testing.T) {
	m := NewMigrationManager()
	pool := &StoragePool{
		Name:      "volume1",
		RAIDType:  RAIDTypeRAID5,
		TotalSize: 24000000000000,
	}
	if err := m.CreatePool(pool); err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}
	if pool.ID == "" {
		t.Fatal("pool ID not generated")
	}
	if pool.Status != "healthy" {
		t.Fatalf("expected healthy status, got %s", pool.Status)
	}
}

func TestStartMigration(t *testing.T) {
	m := NewMigrationManager()

	m.RegisterDisk(&Disk{ID: "disk-old", Device: "/dev/sda", Serial: "OLD001", Size: 4000000000000})
	m.RegisterDisk(&Disk{ID: "disk-new", Device: "/dev/sdb", Serial: "NEW001", Size: 8000000000000})
	m.CreatePool(&StoragePool{ID: "pool1", Name: "volume1", RAIDType: RAIDTypeRAID1})

	task := &MigrationTask{
		Type:         MigrationTypeReplace,
		SourcePoolID: "pool1",
		SourceDiskID: "disk-old",
		TargetDiskID: "disk-new",
		BytesTotal:   4000000000000,
	}

	result, err := m.StartMigration(task)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}
	if result.ID == "" {
		t.Fatal("task ID not generated")
	}
	if result.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", result.Status)
	}
}

func TestStartMigrationDiskNotFound(t *testing.T) {
	m := NewMigrationManager()
	m.CreatePool(&StoragePool{ID: "pool1", Name: "volume1"})

	task := &MigrationTask{
		Type:         MigrationTypeReplace,
		SourcePoolID: "pool1",
		SourceDiskID: "nonexistent",
		TargetDiskID: "also-nonexistent",
	}

	if _, err := m.StartMigration(task); err != ErrDiskNotFound {
		t.Fatalf("expected ErrDiskNotFound, got %v", err)
	}
}

func TestUpdateProgress(t *testing.T) {
	m := NewMigrationManager()
	m.RegisterDisk(&Disk{ID: "disk-old", Serial: "OLD001"})
	m.RegisterDisk(&Disk{ID: "disk-new", Serial: "NEW001"})
	m.CreatePool(&StoragePool{ID: "pool1", Name: "vol1"})

	task, _ := m.StartMigration(&MigrationTask{
		Type:         MigrationTypeReplace,
		SourcePoolID: "pool1",
		SourceDiskID: "disk-old",
		TargetDiskID: "disk-new",
		BytesTotal:   1000000000,
	})

	if err := m.UpdateProgress(task.ID, 50.0, 500000000, 100.0); err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}

	updated, _ := m.GetMigration(task.ID)
	if updated.Progress != 50.0 {
		t.Fatalf("expected 50%% progress, got %f", updated.Progress)
	}
	if updated.Status != StatusSyncing {
		t.Fatalf("expected syncing status, got %s", updated.Status)
	}
}

func TestUpdateProgressComplete(t *testing.T) {
	m := NewMigrationManager()
	m.RegisterDisk(&Disk{ID: "d1", Serial: "S1"})
	m.RegisterDisk(&Disk{ID: "d2", Serial: "S2"})
	m.CreatePool(&StoragePool{ID: "p1", Name: "v1"})

	task, _ := m.StartMigration(&MigrationTask{
		Type: MigrationTypeReplace, SourcePoolID: "p1",
		SourceDiskID: "d1", TargetDiskID: "d2", BytesTotal: 1000,
	})

	m.UpdateProgress(task.ID, 100.0, 1000, 50.0)
	updated, _ := m.GetMigration(task.ID)
	if updated.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", updated.Status)
	}
	if updated.CompletedAt.IsZero() {
		t.Fatal("CompletedAt not set")
	}
}

func TestListMigrations(t *testing.T) {
	m := NewMigrationManager()
	m.RegisterDisk(&Disk{ID: "d1", Serial: "S1"})
	m.RegisterDisk(&Disk{ID: "d2", Serial: "S2"})
	m.CreatePool(&StoragePool{ID: "p1", Name: "v1"})
	m.CreatePool(&StoragePool{ID: "p2", Name: "v2"})

	m.StartMigration(&MigrationTask{
		Type: MigrationTypeReplace, SourcePoolID: "p1",
		SourceDiskID: "d1", TargetDiskID: "d2", BytesTotal: 1000,
	})
	m.StartMigration(&MigrationTask{
		Type: MigrationTypeExpand, SourcePoolID: "p2",
		SourceDiskID: "d2", TargetDiskID: "d1", BytesTotal: 500,
	})

	list := m.ListMigrations()
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}
}

func TestExportReport(t *testing.T) {
	m := NewMigrationManager()
	m.RegisterDisk(&Disk{ID: "d1", Serial: "S1", Size: 1000})
	m.CreatePool(&StoragePool{ID: "p1", Name: "v1", RAIDType: RAIDTypeRAID5})

	data, err := m.ExportReport()
	if err != nil {
		t.Fatalf("ExportReport failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("report is empty")
	}
}
