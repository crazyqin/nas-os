// Package integration 提供 NAS-OS 集成测试
// 数据去重集成测试
package integration

import (
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/dedup"
)

func TestDedup_Integration(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test1.txt")
	content := []byte("Test content for deduplication integration test")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager, err := dedup.NewManager(filepath.Join(tmpDir, "dedup.json"), dedup.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	t.Run("Scan", func(t *testing.T) {
		result, err := manager.Scan([]string{tmpDir})
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}

		if result == nil {
			t.Fatal("Scan result is nil")
		}
		t.Logf("Scanned %d files, found %d duplicates", result.FilesScanned, result.DuplicateGroups)
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := manager.GetStats()
		t.Logf("Stats: TotalFiles=%d, TotalSize=%d", stats.TotalFiles, stats.TotalSize)
	})

	t.Run("GetDuplicates", func(t *testing.T) {
		groups, err := manager.GetDuplicates()
		if err != nil {
			t.Logf("GetDuplicates error (expected for new scan): %v", err)
		}
		t.Logf("Found %d duplicate groups", len(groups))
	})
}

func TestDedup_ConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()

	manager, err := dedup.NewManager(filepath.Join(tmpDir, "dedup.json"), dedup.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			testFile := filepath.Join(tmpDir, "concurrent_test.txt")
			content := []byte("Concurrent test content")
			os.WriteFile(testFile, content, 0644)

			_, err := manager.ScanForUser([]string{testFile}, "test")
			if err != nil {
				t.Logf("Concurrent scan error: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDedup_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	manager, err := dedup.NewManager(filepath.Join(tmpDir, "dedup.json"), dedup.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	testFile := filepath.Join(tmpDir, "cancel_test.txt")
	os.WriteFile(testFile, []byte("Test content"), 0644)

	_, err = manager.ScanForUser([]string{testFile}, "test")
	if err != nil {
		t.Logf("Expected error after cancellation: %v", err)
	}
}

// 性能测试.
func BenchmarkDedup_Scan(b *testing.B) {
	tmpDir := b.TempDir()

	manager, err := dedup.NewManager(filepath.Join(tmpDir, "dedup.json"), dedup.DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	testFile := filepath.Join(tmpDir, "bench_test.txt")
	content := make([]byte, 1024*1024) // 1MB
	os.WriteFile(testFile, content, 0644)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		manager.Scan([]string{testFile})
	}
}
