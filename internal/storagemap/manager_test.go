package storagemap

import (
	"testing"
)

func TestNewStorageMapManager(t *testing.T) {
	manager := NewStorageMapManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected default config to be set")
	}

	if manager.config.ScanInterval != 24*60*60*1000000000 { // 24 hours in nanoseconds
		t.Errorf("Expected scan interval to be 24 hours, got %v", manager.config.ScanInterval)
	}
}

func TestStartScan(t *testing.T) {
	manager := NewStorageMapManager(nil)

	job, err := manager.StartScan("/tmp")
	if err != nil {
		t.Fatalf("Failed to start scan: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be created")
	}

	if job.Status != "running" {
		t.Errorf("Expected status to be 'running', got '%s'", job.Status)
	}

	if job.Path != "/tmp" {
		t.Errorf("Expected path to be '/tmp', got '%s'", job.Path)
	}
}

func TestGetScanJob(t *testing.T) {
	manager := NewStorageMapManager(nil)

	// 创建一个扫描任务
	job, _ := manager.StartScan("/tmp")

	// 获取任务
	fetchedJob, err := manager.GetScanJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to get scan job: %v", err)
	}

	if fetchedJob.ID != job.ID {
		t.Errorf("Expected job ID '%s', got '%s'", job.ID, fetchedJob.ID)
	}

	// 测试不存在的任务
	_, err = manager.GetScanJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestGetFileType(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"photo.jpg", "image"},
		{"video.mp4", "video"},
		{"song.mp3", "audio"},
		{"doc.pdf", "document"},
		{"data.csv", "spreadsheet"},
		{"archive.zip", "archive"},
		{"script.go", "code"},
		{"file", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileType(tt.name)
			if result != tt.expected {
				t.Errorf("getFileType(%s) = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestUsageSummary(t *testing.T) {
	manager := NewStorageMapManager(nil)

	// 测试不存在的路径
	_, err := manager.GetUsageSummary("/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

func TestSnapshotDiff(t *testing.T) {
	manager := NewStorageMapManager(nil)

	// 测试不存在的快照
	_, err := manager.CompareSnapshots("/path1", "/path2")
	if err == nil {
		t.Error("Expected error for nonexistent snapshots")
	}
}
