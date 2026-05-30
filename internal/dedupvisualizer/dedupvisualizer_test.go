// Package dedupvisualizer 测试
package dedupvisualizer

import (
	"testing"
)

func TestNewDedupVisualizer(t *testing.T) {
	v := NewDedupVisualizer()
	if v == nil {
		t.Fatal("NewDedupVisualizer returned nil")
	}
}

func TestRegisterVolume(t *testing.T) {
	v := NewDedupVisualizer()
	v.RegisterVolume("vol1", "Data", 1000000000, true, "lz4")

	stats, err := v.GetVolumeStats("vol1")
	if err != nil {
		t.Fatalf("GetVolumeStats failed: %v", err)
	}
	if stats.VolumeName != "Data" {
		t.Fatalf("expected Data, got %s", stats.VolumeName)
	}
	if !stats.DedupEnabled {
		t.Fatal("expected dedup enabled")
	}
}

func TestGetVolumeStatsNotFound(t *testing.T) {
	v := NewDedupVisualizer()
	if _, err := v.GetVolumeStats("nonexistent"); err != ErrVolumeNotFound {
		t.Fatalf("expected ErrVolumeNotFound, got %v", err)
	}
}

func TestAddBlock(t *testing.T) {
	v := NewDedupVisualizer()

	block := &DataBlock{
		Hash: "abc123",
		Size: 4096,
		Type: BlockTypeFile,
	}
	v.AddBlock(block)

	if block.RefCount != 1 {
		t.Fatalf("expected ref count 1, got %d", block.RefCount)
	}
}

func TestAddDuplicateBlocks(t *testing.T) {
	v := NewDedupVisualizer()

	for i := 0; i < 5; i++ {
		v.AddBlock(&DataBlock{
			Hash:      "same-hash",
			Size:      4096,
			Type:      BlockTypeFile,
			FilePaths: []string{"/file" + string(rune('0'+i))},
		})
	}

	blocks := v.blocks["same-hash"]
	if len(blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(blocks))
	}
	for _, b := range blocks {
		if b.RefCount != 5 {
			t.Fatalf("expected ref count 5, got %d", b.RefCount)
		}
	}
}

func TestTakeSnapshot(t *testing.T) {
	v := NewDedupVisualizer()
	v.RegisterVolume("vol1", "Data", 1000000000, true, "lz4")

	// 添加唯一块
	v.AddBlock(&DataBlock{Hash: "h1", Size: 4096, Type: BlockTypeFile})
	v.AddBlock(&DataBlock{Hash: "h2", Size: 4096, Type: BlockTypeFile})
	// 添加重复块
	v.AddBlock(&DataBlock{Hash: "h1", Size: 4096, Type: BlockTypeFile, FilePaths: []string{"/dup1"}})
	v.AddBlock(&DataBlock{Hash: "h1", Size: 4096, Type: BlockTypeFile, FilePaths: []string{"/dup2"}})

	snap, err := v.TakeSnapshot("vol1")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snap.UniqueBlocks != 2 {
		t.Fatalf("expected 2 unique blocks, got %d", snap.UniqueBlocks)
	}
	if snap.DedupRatio <= 1.0 {
		t.Fatalf("expected dedup ratio > 1.0, got %f", snap.DedupRatio)
	}
	if snap.SavingsBytes <= 0 {
		t.Fatal("expected positive savings")
	}
}

func TestTakeSnapshotNotFound(t *testing.T) {
	v := NewDedupVisualizer()
	if _, err := v.TakeSnapshot("nonexistent"); err != ErrVolumeNotFound {
		t.Fatalf("expected ErrVolumeNotFound, got %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	v := NewDedupVisualizer()
	v.RegisterVolume("vol1", "Data", 1000000000, true, "lz4")
	v.AddBlock(&DataBlock{Hash: "h1", Size: 4096, Type: BlockTypeFile})
	v.AddBlock(&DataBlock{Hash: "h1", Size: 4096, Type: BlockTypeFile})
	v.TakeSnapshot("vol1")

	report := v.GenerateReport()
	if report == nil {
		t.Fatal("report is nil")
	}
	if len(report.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(report.Volumes))
	}
	if report.TotalSaved <= 0 {
		t.Fatal("expected positive total saved")
	}
}

func TestExportJSON(t *testing.T) {
	v := NewDedupVisualizer()
	v.RegisterVolume("vol1", "Test", 1000, false, "none")

	data, err := v.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}
}
