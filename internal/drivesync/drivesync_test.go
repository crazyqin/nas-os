package drivesync

import (
	"testing"
	"time"
)

func TestCreateFolder(t *testing.T) {
	m := NewDriveSyncManager()

	folder := &SyncFolder{
		ID:        "folder1",
		Name:      "Documents",
		LocalPath: "/data/documents",
		DeviceID:  "device1",
	}

	err := m.CreateFolder(folder)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// 重复创建应该失败
	err = m.CreateFolder(folder)
	if err == nil {
		t.Error("expected error for duplicate folder")
	}

	// ID为空应该失败
	err = m.CreateFolder(&SyncFolder{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestDeleteFolder(t *testing.T) {
	m := NewDriveSyncManager()

	m.CreateFolder(&SyncFolder{ID: "f1", Name: "test"})

	err := m.DeleteFolder("f1")
	if err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	// 删除不存在的文件夹
	err = m.DeleteFolder("f1")
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}

func TestUpdateFile(t *testing.T) {
	m := NewDriveSyncManager()
	m.CreateFolder(&SyncFolder{ID: "f1", Name: "test", ConflictPolicy: ConflictKeepBoth})

	file := &SyncFile{
		Path:    "doc.txt",
		Size:    1024,
		ModTime: time.Now(),
	}

	err := m.UpdateFile("f1", file)
	if err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}

	// 更新同名文件应增加版本
	file2 := &SyncFile{
		Path:    "doc.txt",
		Size:    2048,
		ModTime: time.Now(),
	}
	m.UpdateFile("f1", file2)

	history, _ := m.GetFileVersionHistory("f1", "doc.txt")
	if history.Version != 2 {
		t.Errorf("expected version 2, got %d", history.Version)
	}
}

func TestDeviceManagement(t *testing.T) {
	m := NewDriveSyncManager()

	device := &SyncDevice{
		ID:   "dev1",
		Name: "My PC",
		Type: "desktop",
		OS:   "Windows",
	}

	err := m.RegisterDevice(device)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	devices := m.ListDevices(false)
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}

	// 只看在线设备
	devices = m.ListDevices(true)
	if len(devices) != 1 {
		t.Errorf("expected 1 online device, got %d", len(devices))
	}
}

func TestSyncOperations(t *testing.T) {
	m := NewDriveSyncManager()
	m.CreateFolder(&SyncFolder{ID: "f1", Name: "test"})

	// 开始同步
	err := m.StartSync("f1")
	if err != nil {
		t.Fatalf("StartSync failed: %v", err)
	}

	folder, _ := m.GetFolder("f1")
	if folder.Status != SyncSyncing {
		t.Errorf("expected syncing status, got %s", folder.Status)
	}

	// 完成同步
	m.CompleteSync("f1")
	folder, _ = m.GetFolder("f1")
	if folder.Status != SyncIdle {
		t.Errorf("expected idle status, got %s", folder.Status)
	}
}

func TestSyncEvents(t *testing.T) {
	m := NewDriveSyncManager()
	m.CreateFolder(&SyncFolder{ID: "f1", Name: "test"})

	m.UpdateFile("f1", &SyncFile{Path: "a.txt", ModTime: time.Now()})
	m.UpdateFile("f1", &SyncFile{Path: "b.txt", ModTime: time.Now()})

	events := m.GetSyncEvents("f1", 0)
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}

func TestSyncStats(t *testing.T) {
	m := NewDriveSyncManager()
	m.CreateFolder(&SyncFolder{ID: "f1", Name: "test"})

	m.UpdateFile("f1", &SyncFile{Path: "a.txt", ModTime: time.Now()})
	m.UpdateFile("f1", &SyncFile{Path: "b.txt", ModTime: time.Now()})

	stats := m.GetSyncStats("")
	totalFiles := stats["total_files"].(int)
	if totalFiles != 2 {
		t.Errorf("expected 2 total files, got %d", totalFiles)
	}
}
