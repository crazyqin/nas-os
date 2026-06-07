package smartcompressengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	manager, err := NewManager(nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if manager == nil {
		t.Fatal("Manager is nil")
	}
}

func TestAnalyzeFile(t *testing.T) {
	manager, _ := NewManager(nil)
	manager.Start()

	// 创建临时文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello World! This is a test file."), 0644)

	ctx := context.Background()
	analysis, err := manager.AnalyzeFile(ctx, tmpFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if analysis.FileType != FileTypeText {
		t.Errorf("Expected FileTypeText, got %s", analysis.FileType)
	}
	if !analysis.Compressible {
		t.Error("Expected file to be compressible")
	}
}

func TestCompressFile(t *testing.T) {
	manager, _ := NewManager(nil)
	manager.Start()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello World! This is a test file."), 0644)

	ctx := context.Background()
	task, err := manager.CompressFile(ctx, tmpFile, tmpFile+".gz", AlgorithmGzip, LevelBalanced)
	if err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	if task == nil {
		t.Fatal("Task is nil")
	}

	// 等待任务完成
	// 在实际场景中应该有更好的同步机制
}

func TestGetStats(t *testing.T) {
	manager, _ := NewManager(nil)
	stats := manager.GetStats()
	if stats.TotalTasks != 0 {
		t.Errorf("Expected 0 tasks, got %d", stats.TotalTasks)
	}
}

func TestListTasks(t *testing.T) {
	manager, _ := NewManager(nil)
	tasks := manager.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}
}
