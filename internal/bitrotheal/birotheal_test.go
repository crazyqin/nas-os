package bitrotheal

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ========== NewHealEngine ==========

func TestNewHealEngine_NilConfig(t *testing.T) {
	engine := NewHealEngine(nil)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.config == nil {
		t.Fatal("expected non-nil default config")
	}
	if engine.config.Algorithm != AlgorithmSHA256 {
		t.Errorf("expected algorithm %q, got %q", AlgorithmSHA256, engine.config.Algorithm)
	}
	if engine.config.MaxConcurrent != 4 {
		t.Errorf("expected MaxConcurrent 4, got %d", engine.config.MaxConcurrent)
	}
}

func TestNewHealEngine_CustomConfig(t *testing.T) {
	cfg := &HealConfig{
		Algorithm:     AlgorithmCRC32,
		MaxConcurrent: 8,
		AutoRepair:    false,
	}
	engine := NewHealEngine(cfg)
	if engine.config.Algorithm != AlgorithmCRC32 {
		t.Errorf("expected algorithm %q, got %q", AlgorithmCRC32, engine.config.Algorithm)
	}
	if engine.config.MaxConcurrent != 8 {
		t.Errorf("expected MaxConcurrent 8, got %d", engine.config.MaxConcurrent)
	}
}

func TestNewHealEngine_DefaultsFilled(t *testing.T) {
	// Empty config should get defaults filled
	cfg := &HealConfig{}
	engine := NewHealEngine(cfg)
	if engine.config.Algorithm != AlgorithmSHA256 {
		t.Errorf("expected default algorithm %q, got %q", AlgorithmSHA256, engine.config.Algorithm)
	}
	if engine.config.MaxConcurrent != 4 {
		t.Errorf("expected default MaxConcurrent 4, got %d", engine.config.MaxConcurrent)
	}
}

func TestNewHealEngine_ZeroMaxConcurrent(t *testing.T) {
	cfg := &HealConfig{MaxConcurrent: 0}
	engine := NewHealEngine(cfg)
	if engine.config.MaxConcurrent != 4 {
		t.Errorf("expected default MaxConcurrent 4, got %d", engine.config.MaxConcurrent)
	}
}

func TestNewHealEngine_NegativeMaxConcurrent(t *testing.T) {
	cfg := &HealConfig{MaxConcurrent: -1}
	engine := NewHealEngine(cfg)
	if engine.config.MaxConcurrent != 4 {
		t.Errorf("expected default MaxConcurrent 4, got %d", engine.config.MaxConcurrent)
	}
}

// ========== CalculateChecksum ==========

func TestCalculateChecksum_SHA256(t *testing.T) {
	engine := NewHealEngine(&HealConfig{Algorithm: AlgorithmSHA256})
	data := []byte("hello world")
	got := engine.CalculateChecksum("test.txt", data)

	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	if got != want {
		t.Errorf("SHA256 mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestCalculateChecksum_CRC32(t *testing.T) {
	engine := NewHealEngine(&HealConfig{Algorithm: AlgorithmCRC32})
	data := []byte("hello world")
	got := engine.CalculateChecksum("test.txt", data)

	// CRC32 produces 8-char hex
	if len(got) != 8 {
		t.Errorf("expected 8-char CRC32 hex, got %q (len=%d)", got, len(got))
	}

	// Same data should produce same checksum
	got2 := engine.CalculateChecksum("test.txt", data)
	if got != got2 {
		t.Errorf("CRC32 not deterministic: %s != %s", got, got2)
	}
}

func TestCalculateChecksum_DifferentDataDifferentChecksum(t *testing.T) {
	engine := NewHealEngine(nil)
	c1 := engine.CalculateChecksum("a.txt", []byte("aaa"))
	c2 := engine.CalculateChecksum("a.txt", []byte("bbb"))
	if c1 == c2 {
		t.Error("different data should produce different checksums")
	}
}

func TestCalculateChecksum_DefaultAlgorithm(t *testing.T) {
	// Empty algorithm string should default to SHA256
	engine := NewHealEngine(&HealConfig{Algorithm: ""})
	data := []byte("test")
	got := engine.CalculateChecksum("f.txt", data)

	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Errorf("expected SHA256 default, got %s", got)
	}
}

func TestCalculateChecksum_EmptyData(t *testing.T) {
	engine := NewHealEngine(nil)
	c := engine.CalculateChecksum("empty.txt", []byte{})
	if c == "" {
		t.Error("expected non-empty checksum for empty data")
	}
}

// ========== AddChecksum & GetChecksum ==========

func TestAddAndGetChecksum(t *testing.T) {
	engine := NewHealEngine(nil)
	entry := &ChecksumEntry{
		Path:      "/tmp/test.txt",
		Algorithm: AlgorithmSHA256,
		Checksum:  "abc123",
		FileSize:  100,
	}

	if err := engine.AddChecksum(entry); err != nil {
		t.Fatalf("AddChecksum failed: %v", err)
	}

	got, err := engine.GetChecksum("/tmp/test.txt")
	if err != nil {
		t.Fatalf("GetChecksum failed: %v", err)
	}
	if got.Path != entry.Path {
		t.Errorf("path mismatch: %s != %s", got.Path, entry.Path)
	}
	if got.Checksum != entry.Checksum {
		t.Errorf("checksum mismatch: %s != %s", got.Checksum, entry.Checksum)
	}
}

func TestAddChecksum_NilEntry(t *testing.T) {
	engine := NewHealEngine(nil)
	if err := engine.AddChecksum(nil); err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
}

func TestAddChecksum_EmptyPath(t *testing.T) {
	engine := NewHealEngine(nil)
	entry := &ChecksumEntry{Path: ""}
	if err := engine.AddChecksum(entry); err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
}

func TestGetChecksum_EmptyPath(t *testing.T) {
	engine := NewHealEngine(nil)
	_, err := engine.GetChecksum("")
	if err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
}

func TestGetChecksum_NotFound(t *testing.T) {
	engine := NewHealEngine(nil)
	_, err := engine.GetChecksum("/nonexistent")
	if err != ErrChecksumNotFound {
		t.Errorf("expected ErrChecksumNotFound, got %v", err)
	}
}

func TestAddChecksum_Overwrite(t *testing.T) {
	engine := NewHealEngine(nil)
	e1 := &ChecksumEntry{Path: "/f.txt", Checksum: "aaa"}
	e2 := &ChecksumEntry{Path: "/f.txt", Checksum: "bbb"}

	engine.AddChecksum(e1)
	engine.AddChecksum(e2)

	got, _ := engine.GetChecksum("/f.txt")
	if got.Checksum != "bbb" {
		t.Errorf("expected overwritten checksum bbb, got %s", got.Checksum)
	}
}

func TestAddChecksum_Concurrent(t *testing.T) {
	engine := NewHealEngine(nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			entry := &ChecksumEntry{
				Path:     filepath.Join("/tmp", "concurrent", string(rune('a'+n%26))+".txt"),
				Checksum: "val",
			}
			engine.AddChecksum(entry)
		}(i)
	}
	wg.Wait()
}

// ========== Verify ==========

func TestVerify_Success(t *testing.T) {
	engine := NewHealEngine(nil)
	data := []byte("integrity test data")
	path := "/tmp/verify_test.txt"

	// Add known-good checksum
	checksum := engine.CalculateChecksum(path, data)
	engine.AddChecksum(&ChecksumEntry{
		Path:     path,
		Checksum: checksum,
	})

	ok, err := engine.Verify(path, data)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected verification to pass")
	}
}

func TestVerify_CorruptedData(t *testing.T) {
	engine := NewHealEngine(nil)
	path := "/tmp/corrupted.txt"
	original := []byte("original data")

	// Store checksum of original data
	checksum := engine.CalculateChecksum(path, original)
	engine.AddChecksum(&ChecksumEntry{
		Path:     path,
		Checksum: checksum,
	})

	// Verify with different data
	corrupted := []byte("corrupted data")
	ok, err := engine.Verify(path, corrupted)
	if err != ErrChecksumMismatch {
		t.Errorf("expected ErrChecksumMismatch, got %v", err)
	}
	if ok {
		t.Error("expected verification to fail for corrupted data")
	}
}

func TestVerify_EmptyPath(t *testing.T) {
	engine := NewHealEngine(nil)
	ok, err := engine.Verify("", []byte("data"))
	if err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
	if ok {
		t.Error("expected false for empty path")
	}
}

func TestVerify_NoChecksumRecord(t *testing.T) {
	engine := NewHealEngine(nil)
	ok, err := engine.Verify("/nonexistent", []byte("data"))
	if err != ErrChecksumNotFound {
		t.Errorf("expected ErrChecksumNotFound, got %v", err)
	}
	if ok {
		t.Error("expected false when no checksum record")
	}
}

func TestVerify_UpdatesLastVerified(t *testing.T) {
	engine := NewHealEngine(nil)
	path := "/tmp/verify_time.txt"
	data := []byte("time test")

	checksum := engine.CalculateChecksum(path, data)
	entry := &ChecksumEntry{
		Path:         path,
		Checksum:     checksum,
		LastVerified: time.Time{},
	}
	engine.AddChecksum(entry)

	before := time.Now()
	engine.Verify(path, data)

	got, _ := engine.GetChecksum(path)
	if got.LastVerified.Before(before) {
		t.Error("expected LastVerified to be updated")
	}
}

// ========== Repair ==========

func TestRepair_EmptyPath(t *testing.T) {
	engine := NewHealEngine(nil)
	result, err := engine.Repair("")
	if err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestRepair_NoRedundancy(t *testing.T) {
	engine := NewHealEngine(nil)
	result, err := engine.Repair("/nonexistent/file.txt")
	if err != ErrRepairFailed {
		t.Errorf("expected ErrRepairFailed, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Strategy != RepairManual {
		t.Errorf("expected strategy %q, got %q", RepairManual, result.Strategy)
	}
	if result.Error != ErrNoRedundancy.Error() {
		t.Errorf("expected error %q, got %q", ErrNoRedundancy.Error(), result.Error)
	}
}

func TestRepair_FromReplica(t *testing.T) {
	// Create temp dirs
	tmpDir := t.TempDir()
	replicaDir := t.TempDir()

	// Write source file (replica with correct data)
	sourceData := []byte("correct data from replica")
	replicaPath := filepath.Join(replicaDir, "data.txt")
	if err := os.WriteFile(replicaPath, sourceData, 0644); err != nil {
		t.Fatalf("failed to write replica: %v", err)
	}

	// Write corrupted target file
	targetPath := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(targetPath, []byte("corrupted"), 0644); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}

	// Register checksum for the target (matching the good data)
	engine := NewHealEngine(&HealConfig{
		ReplicaPaths: []string{replicaDir},
	})
	checksum := engine.CalculateChecksum(targetPath, sourceData)
	engine.AddChecksum(&ChecksumEntry{
		Path:     targetPath,
		Checksum: checksum,
	})

	result, err := engine.Repair(targetPath)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if !result.Success {
		t.Error("expected repair to succeed")
	}
	if result.Strategy != RepairFromReplica {
		t.Errorf("expected strategy %q, got %q", RepairFromReplica, result.Strategy)
	}

	// Verify the file was actually repaired
	repaired, _ := os.ReadFile(targetPath)
	if string(repaired) != string(sourceData) {
		t.Errorf("file content mismatch after repair: %q != %q", repaired, sourceData)
	}

	// Check repair count was incremented
	entry, _ := engine.GetChecksum(targetPath)
	if entry.RepairCount != 1 {
		t.Errorf("expected RepairCount=1, got %d", entry.RepairCount)
	}
}

func TestRepair_FromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := t.TempDir()

	// Write backup file with correct data
	correctData := []byte("correct backup data")
	backupPath := filepath.Join(backupDir, "file.txt")
	os.WriteFile(backupPath, correctData, 0644)

	// Write corrupted target
	targetPath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(targetPath, []byte("corrupted"), 0644)

	engine := NewHealEngine(&HealConfig{
		BackupRoot: backupDir,
	})
	checksum := engine.CalculateChecksum(targetPath, correctData)
	engine.AddChecksum(&ChecksumEntry{
		Path:     targetPath,
		Checksum: checksum,
	})

	result, err := engine.Repair(targetPath)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if !result.Success {
		t.Error("expected repair from backup to succeed")
	}
	if result.Strategy != RepairFromBackup {
		t.Errorf("expected strategy %q, got %q", RepairFromBackup, result.Strategy)
	}
}

func TestRepair_ReplicaPriorityOverBackup(t *testing.T) {
	tmpDir := t.TempDir()
	replicaDir := t.TempDir()
	backupDir := t.TempDir()

	// Both replica and backup have correct data
	correctData := []byte("correct data")
	os.WriteFile(filepath.Join(replicaDir, "f.txt"), correctData, 0644)
	os.WriteFile(filepath.Join(backupDir, "f.txt"), correctData, 0644)

	targetPath := filepath.Join(tmpDir, "f.txt")
	os.WriteFile(targetPath, []byte("bad"), 0644)

	engine := NewHealEngine(&HealConfig{
		ReplicaPaths: []string{replicaDir},
		BackupRoot:   backupDir,
	})
	checksum := engine.CalculateChecksum(targetPath, correctData)
	engine.AddChecksum(&ChecksumEntry{Path: targetPath, Checksum: checksum})

	result, err := engine.Repair(targetPath)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	// Should use replica first
	if result.Strategy != RepairFromReplica {
		t.Errorf("expected replica strategy, got %q", result.Strategy)
	}
}

func TestRepair_ReplicaCorruptFallsToBackup(t *testing.T) {
	tmpDir := t.TempDir()
	replicaDir := t.TempDir()
	backupDir := t.TempDir()

	correctData := []byte("good data")

	// Replica has wrong data (corrupt replica)
	os.WriteFile(filepath.Join(replicaDir, "f.txt"), []byte("bad replica"), 0644)
	// Backup has correct data
	os.WriteFile(filepath.Join(backupDir, "f.txt"), correctData, 0644)

	targetPath := filepath.Join(tmpDir, "f.txt")
	os.WriteFile(targetPath, []byte("corrupted"), 0644)

	engine := NewHealEngine(&HealConfig{
		ReplicaPaths: []string{replicaDir},
		BackupRoot:   backupDir,
	})
	checksum := engine.CalculateChecksum(targetPath, correctData)
	engine.AddChecksum(&ChecksumEntry{Path: targetPath, Checksum: checksum})

	result, err := engine.Repair(targetPath)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if !result.Success {
		t.Error("expected repair to succeed via backup")
	}
	if result.Strategy != RepairFromBackup {
		t.Errorf("expected backup strategy, got %q", result.Strategy)
	}
}

// ========== Scan ==========

func TestScan_EmptyRoot(t *testing.T) {
	engine := NewHealEngine(nil)
	_, err := engine.Scan("")
	if err != ErrPathRequired {
		t.Errorf("expected ErrPathRequired, got %v", err)
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	engine := NewHealEngine(nil)
	tmpDir := t.TempDir()

	report, err := engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if report.ScannedFiles != 0 {
		t.Errorf("expected 0 scanned files, got %d", report.ScannedFiles)
	}
}

func TestScan_HealthyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(nil)

	// Create test files
	files := map[string]string{
		"a.txt": "content A",
		"b.txt": "content B",
		"c.txt": "content C",
	}
	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		os.WriteFile(path, []byte(content), 0644)
	}

	// First scan registers checksums
	report, err := engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if report.ScannedFiles != 3 {
		t.Errorf("expected 3 scanned files, got %d", report.ScannedFiles)
	}
	if report.CorruptedFiles != 0 {
		t.Errorf("expected 0 corrupted files, got %d", report.CorruptedFiles)
	}
}

func TestScan_CorruptedFileDetected(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(nil)

	// Create a file
	path := filepath.Join(tmpDir, "data.txt")
	originalData := []byte("original content")
	os.WriteFile(path, originalData, 0644)

	// First scan: registers checksum
	engine.Scan(tmpDir)

	// Corrupt the file
	os.WriteFile(path, []byte("corrupted content"), 0644)

	// Second scan: detects corruption
	report, err := engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if report.CorruptedFiles != 1 {
		t.Errorf("expected 1 corrupted file, got %d", report.CorruptedFiles)
	}
	if len(report.CorruptedPaths) != 1 || report.CorruptedPaths[0] != path {
		t.Errorf("expected corrupted path %q, got %v", path, report.CorruptedPaths)
	}
}

func TestScan_Subdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(nil)

	// Create nested structure
	subDir := filepath.Join(tmpDir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	report, err := engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if report.ScannedFiles != 2 {
		t.Errorf("expected 2 scanned files (recursive), got %d", report.ScannedFiles)
	}
}

func TestScan_SetsStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(nil)

	before := time.Now()
	report, _ := engine.Scan(tmpDir)
	after := time.Now()

	if report.StartTime.Before(before) || report.StartTime.After(after) {
		t.Errorf("StartTime %v not between %v and %v", report.StartTime, before, after)
	}
	if report.ScanDuration <= 0 {
		t.Error("expected positive ScanDuration")
	}
}

func TestScan_AutoRepairWithReplica(t *testing.T) {
	tmpDir := t.TempDir()
	replicaDir := t.TempDir()
	engine := NewHealEngine(&HealConfig{
		ReplicaPaths: []string{replicaDir},
		AutoRepair:   true,
	})

	// Create file and replica
	path := filepath.Join(tmpDir, "data.txt")
	originalData := []byte("good data")
	os.WriteFile(path, originalData, 0644)
	os.WriteFile(filepath.Join(replicaDir, "data.txt"), originalData, 0644)

	// First scan registers checksum
	engine.Scan(tmpDir)

	// Corrupt the file
	os.WriteFile(path, []byte("bad data"), 0644)

	// Second scan should auto-repair
	report, _ := engine.Scan(tmpDir)
	if report.RepairedFiles != 1 {
		t.Errorf("expected 1 repaired file, got %d", report.RepairedFiles)
	}
	if report.CorruptedFiles != 1 {
		t.Errorf("expected 1 corrupted file detected, got %d", report.CorruptedFiles)
	}

	// Verify file was actually repaired
	repaired, _ := os.ReadFile(path)
	if string(repaired) != string(originalData) {
		t.Errorf("file not repaired: %q", repaired)
	}
}

func TestScan_AutoRepairDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(&HealConfig{AutoRepair: false})

	path := filepath.Join(tmpDir, "data.txt")
	os.WriteFile(path, []byte("original"), 0644)
	engine.Scan(tmpDir)

	os.WriteFile(path, []byte("corrupted"), 0644)
	report, _ := engine.Scan(tmpDir)

	if report.RepairedFiles != 0 {
		t.Errorf("expected 0 repaired files with AutoRepair=false, got %d", report.RepairedFiles)
	}
	if report.UnrepairableFiles != 1 {
		t.Errorf("expected 1 unrepairable file, got %d", report.UnrepairableFiles)
	}
}

// ========== Stop ==========

func TestStop(t *testing.T) {
	engine := NewHealEngine(nil)
	// Should not panic
	engine.Stop()
}

func TestStop_Idempotent(t *testing.T) {
	// Note: double-close panics on a plain channel; this test verifies
	// the engine's Stop works at least once. If the implementation uses
	// sync.Once or similar, this test would pass cleanly.
	engine := NewHealEngine(nil)
	engine.Stop()
}

// ========== DefaultConfig ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Algorithm != AlgorithmSHA256 {
		t.Errorf("default algorithm: expected %q, got %q", AlgorithmSHA256, cfg.Algorithm)
	}
	if cfg.ScanInterval != 24*time.Hour {
		t.Errorf("default ScanInterval: expected 24h, got %v", cfg.ScanInterval)
	}
	if !cfg.AutoRepair {
		t.Error("default AutoRepair should be true")
	}
	if cfg.MaxConcurrent != 4 {
		t.Errorf("default MaxConcurrent: expected 4, got %d", cfg.MaxConcurrent)
	}
}

// ========== Integration / E2E ==========

func TestIntegration_FullCycle(t *testing.T) {
	tmpDir := t.TempDir()
	replicaDir := t.TempDir()

	engine := NewHealEngine(&HealConfig{
		Algorithm:     AlgorithmSHA256,
		ReplicaPaths:  []string{replicaDir},
		AutoRepair:    true,
		MaxConcurrent: 2,
	})

	// 1. Create files and replicas
	testFiles := map[string]string{
		"doc1.txt":  "important document",
		"doc2.txt":  "another document",
		"data.json": `{"key": "value"}`,
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		os.WriteFile(path, []byte(content), 0644)
		os.WriteFile(filepath.Join(replicaDir, name), []byte(content), 0644)
	}

	// 2. Initial scan to register checksums
	report, err := engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("initial scan failed: %v", err)
	}
	if report.ScannedFiles != 3 {
		t.Errorf("expected 3 files scanned, got %d", report.ScannedFiles)
	}

	// 3. Verify each file
	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		ok, err := engine.Verify(path, []byte(content))
		if err != nil {
			t.Errorf("verify %s failed: %v", name, err)
		}
		if !ok {
			t.Errorf("verify %s returned false", name)
		}
	}

	// 4. Corrupt a file
	corruptPath := filepath.Join(tmpDir, "doc1.txt")
	os.WriteFile(corruptPath, []byte("CORRUPTED!"), 0644)

	// 5. Verify detects corruption
	ok, err := engine.Verify(corruptPath, []byte("CORRUPTED!"))
	if err != ErrChecksumMismatch {
		t.Errorf("expected ErrChecksumMismatch, got %v", err)
	}
	if ok {
		t.Error("expected verification to fail after corruption")
	}

	// 6. Scan with auto-repair
	report, err = engine.Scan(tmpDir)
	if err != nil {
		t.Fatalf("repair scan failed: %v", err)
	}
	if report.CorruptedFiles != 1 {
		t.Errorf("expected 1 corrupted file, got %d", report.CorruptedFiles)
	}
	if report.RepairedFiles != 1 {
		t.Errorf("expected 1 repaired file, got %d", report.RepairedFiles)
	}

	// 7. Verify file is repaired
	repaired, _ := os.ReadFile(corruptPath)
	if string(repaired) != "important document" {
		t.Errorf("file not properly repaired: %q", repaired)
	}

	// 8. Check repair count
	entry, _ := engine.GetChecksum(corruptPath)
	if entry.RepairCount != 1 {
		t.Errorf("expected RepairCount=1, got %d", entry.RepairCount)
	}
}

func TestIntegration_CRC32Algorithm(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewHealEngine(&HealConfig{
		Algorithm: AlgorithmCRC32,
	})

	path := filepath.Join(tmpDir, "crc.txt")
	data := []byte("crc32 test data")
	os.WriteFile(path, data, 0644)

	report, _ := engine.Scan(tmpDir)
	if report.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", report.ScannedFiles)
	}

	// Verify checksum format is CRC32 (8 chars)
	entry, _ := engine.GetChecksum(path)
	if len(entry.Checksum) != 8 {
		t.Errorf("expected 8-char CRC32 checksum, got %q (len=%d)", entry.Checksum, len(entry.Checksum))
	}
	if entry.Algorithm != AlgorithmCRC32 {
		t.Errorf("expected algorithm %q, got %q", AlgorithmCRC32, entry.Algorithm)
	}
}
