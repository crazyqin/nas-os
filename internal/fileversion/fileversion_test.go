// Package fileversion 文件版本控制 - 测试
package fileversion

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	config := m.GetConfig()
	if config.MaxVersions != 32 {
		t.Errorf("default max versions should be 32, got %d", config.MaxVersions)
	}
	if config.CleanupPolicy != CleanupSmart {
		t.Errorf("default cleanup should be smart, got %s", config.CleanupPolicy)
	}
}

func TestCreateVersion(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{
		StoragePath: tmpDir,
		MaxVersions: 10,
	})

	content := []byte("hello world v1")
	reader := bytes.NewReader(content)

	v, err := m.CreateVersion("/test/file.txt", reader, "initial", "user1")
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
	if v.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), v.Size)
	}
	if v.SHA256 == "" {
		t.Error("SHA256 should not be empty")
	}

	// Same content should return same version
	v2, err := m.CreateVersion("/test/file.txt", bytes.NewReader(content), "no change", "user1")
	if err != nil {
		t.Fatalf("CreateVersion (same) failed: %v", err)
	}
	if v2.Version != 1 {
		t.Errorf("same content should return version 1, got %d", v2.Version)
	}

	// Different content
	content2 := []byte("hello world v2")
	v3, err := m.CreateVersion("/test/file.txt", bytes.NewReader(content2), "updated", "user1")
	if err != nil {
		t.Fatalf("CreateVersion v2 failed: %v", err)
	}
	if v3.Version != 2 {
		t.Errorf("expected version 2, got %d", v3.Version)
	}
}

func TestGetVersion(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	content := []byte("test content")
	m.CreateVersion("/test.txt", bytes.NewReader(content), "v1", "user")

	v, data, err := m.GetVersion("/test.txt", 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}

	// Non-existent version
	_, _, err = m.GetVersion("/test.txt", 99)
	if err == nil {
		t.Error("expected error for non-existent version")
	}

	// Non-existent file
	_, _, err = m.GetVersion("/nonexistent.txt", 1)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestHistory(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/a.txt", bytes.NewReader([]byte("v1")), "v1", "user")
	m.CreateVersion("/a.txt", bytes.NewReader([]byte("v2")), "v2", "user")
	m.CreateVersion("/b.txt", bytes.NewReader([]byte("v1")), "v1", "user")

	h, err := m.GetHistory("/a.txt")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(h.Versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(h.Versions))
	}

	all := m.ListHistories()
	if len(all) != 2 {
		t.Errorf("expected 2 histories, got %d", len(all))
	}
}

func TestRestoreVersion(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/test.txt", bytes.NewReader([]byte("original")), "v1", "user")
	m.CreateVersion("/test.txt", bytes.NewReader([]byte("modified")), "v2", "user")

	restorePath := filepath.Join(tmpDir, "restored.txt")
	result, err := m.RestoreVersion("/test.txt", 1, restorePath)
	if err != nil {
		t.Fatalf("RestoreVersion failed: %v", err)
	}
	if !result.Success {
		t.Error("restore should succeed")
	}

	data, err := os.ReadFile(restorePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("expected 'original', got %q", data)
	}
}

func TestDeleteVersion(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/test.txt", bytes.NewReader([]byte("v1")), "v1", "user")
	m.CreateVersion("/test.txt", bytes.NewReader([]byte("v2")), "v2", "user")

	if err := m.DeleteVersion("/test.txt", 1); err != nil {
		t.Fatalf("DeleteVersion failed: %v", err)
	}

	// Should not be retrievable
	_, _, err := m.GetVersion("/test.txt", 1)
	if err == nil {
		t.Error("deleted version should not be retrievable")
	}

	// Purge
	count, err := m.PurgeDeleted("/test.txt")
	if err != nil {
		t.Fatalf("PurgeDeleted failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 purged, got %d", count)
	}
}

func TestDeleteHistory(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/test.txt", bytes.NewReader([]byte("v1")), "v1", "user")

	if err := m.DeleteHistory("/test.txt"); err != nil {
		t.Fatalf("DeleteHistory failed: %v", err)
	}

	if err := m.DeleteHistory("/nonexistent"); err == nil {
		t.Error("expected error on nonexistent history")
	}
}

func TestMaxVersions(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{
		StoragePath:     tmpDir,
		MaxVersions:     3,
		CleanupPolicy:   CleanupByVersionCount,
	})

	for i := 0; i < 5; i++ {
		content := []byte("version " + string(rune('A'+i)))
		m.CreateVersion("/test.txt", bytes.NewReader(content), "", "user")
	}

	h, _ := m.GetHistory("/test.txt")
	if len(h.Versions) > 3 {
		t.Errorf("expected max 3 versions, got %d", len(h.Versions))
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/a.txt", bytes.NewReader([]byte("hello")), "v1", "user")
	m.CreateVersion("/b.txt", bytes.NewReader([]byte("world")), "v1", "user")
	m.CreateVersion("/a.txt", bytes.NewReader([]byte("hello2")), "v2", "user")

	stats := m.GetStats()
	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}
	if stats.TotalVersions != 3 {
		t.Errorf("expected 3 versions, got %d", stats.TotalVersions)
	}
	if stats.TotalSize == 0 {
		t.Error("total size should not be 0")
	}
}

func TestDiffVersions(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/test.txt", bytes.NewReader([]byte("same")), "v1", "user")
	m.CreateVersion("/test.txt", bytes.NewReader([]byte("diff")), "v2", "user")
	m.CreateVersion("/test.txt", bytes.NewReader([]byte("other")), "v3", "user")

	same, err := m.DiffVersions("/test.txt", 1, 2)
	if err != nil {
		t.Fatalf("DiffVersions failed: %v", err)
	}
	if same {
		t.Error("v1 and v2 should be different")
	}

	same, err = m.DiffVersions("/test.txt", 2, 3)
	if err != nil {
		t.Fatalf("DiffVersions failed: %v", err)
	}
	if same {
		t.Error("v2 and v3 should be different")
	}
}

func TestExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{StoragePath: tmpDir})

	m.CreateVersion("/test.txt", bytes.NewReader([]byte("hello")), "v1", "user")

	data, err := m.ExportMetadata()
	if err != nil {
		t.Fatalf("ExportMetadata failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty export")
	}

	m2 := NewManager(VersionConfig{StoragePath: tmpDir})
	if err := m2.ImportMetadata(data); err != nil {
		t.Fatalf("ImportMetadata failed: %v", err)
	}

	h, err := m2.GetHistory("/test.txt")
	if err != nil {
		t.Fatalf("GetHistory after import failed: %v", err)
	}
	if len(h.Versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(h.Versions))
	}
}

func TestExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{
		StoragePath:     tmpDir,
		ExcludePatterns: []string{"*.tmp", "*.bak"},
	})

	_, err := m.CreateVersion("/test.tmp", bytes.NewReader([]byte("temp")), "v1", "user")
	if err == nil {
		t.Error("excluded file should not be versioned")
	}

	v, err := m.CreateVersion("/test.txt", bytes.NewReader([]byte("ok")), "v1", "user")
	if err != nil {
		t.Fatalf("non-excluded file should be versioned: %v", err)
	}
	if v == nil {
		t.Error("expected version")
	}
}

func TestCleanupVersions(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(VersionConfig{
		StoragePath:   tmpDir,
		MaxVersions:   2,
		CleanupPolicy: CleanupByVersionCount,
	})

	for i := 0; i < 5; i++ {
		content := []byte("version " + string(rune('A'+i)))
		m.CreateVersion("/a.txt", bytes.NewReader(content), "", "user")
	}

	h, _ := m.GetHistory("/a.txt")
	if len(h.Versions) > 2 {
		t.Errorf("cleanup should keep max 2 versions, got %d", len(h.Versions))
	}
}

func TestVersionStates(t *testing.T) {
	states := map[VersionState]string{
		VersionActive:   "active",
		VersionArchived: "archived",
		VersionDeleted:  "deleted",
	}
	for state, expected := range states {
		if string(state) != expected {
			t.Errorf("state %v != %q", state, expected)
		}
	}
}

func TestCleanupPolicies(t *testing.T) {
	policies := map[CleanupPolicy]string{
		CleanupByVersionCount: "version_count",
		CleanupByAge:          "age",
		CleanupBySize:         "size",
		CleanupSmart:          "smart",
	}
	for policy, expected := range policies {
		if string(policy) != expected {
			t.Errorf("policy %v != %q", policy, expected)
		}
	}
}

func TestGetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	config := VersionConfig{
		StoragePath:   tmpDir,
		MaxVersions:   50,
		MaxTotalSize:  1024 * 1024,
		MaxAge:        30 * 24 * time.Hour,
		CleanupPolicy: CleanupSmart,
	}
	m := NewManager(config)

	got := m.GetConfig()
	if got.MaxVersions != 50 {
		t.Errorf("expected 50, got %d", got.MaxVersions)
	}
	if got.MaxTotalSize != 1024*1024 {
		t.Errorf("expected 1048576, got %d", got.MaxTotalSize)
	}
}
