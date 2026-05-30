package backupdedup

import (
	"fmt"
	"testing"
)

func TestProcessFile(t *testing.T) {
	config := DedupConfig{
		Enabled:         true,
		ChunkSize:       4,
		MinFileSize:     10,
		MaxFileSize:     1024,
		VerifyIntegrity: true,
	}

	m := NewManager(config)

	data := []byte("Hello, this is test data for deduplication!")
	info, err := m.ProcessFile("/test/file1.txt", int64(len(data)), data)
	if err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	if info.IsDuplicate {
		t.Error("First file should not be duplicate")
	}

	if info.RefCount != 1 {
		t.Errorf("Expected ref count 1, got %d", info.RefCount)
	}
}

func TestDuplicateDetection(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	data := []byte("Duplicate test data!")

	m.ProcessFile("/test/file1.txt", int64(len(data)), data)
	info2, err := m.ProcessFile("/test/file2.txt", int64(len(data)), data)
	if err != nil {
		t.Fatalf("Failed to process duplicate file: %v", err)
	}

	if !info2.IsDuplicate {
		t.Error("Second file should be duplicate")
	}

	stats := m.GetStats()
	if stats.DuplicateFiles != 1 {
		t.Errorf("Expected 1 duplicate file, got %d", stats.DuplicateFiles)
	}
}

func TestDisabledDedup(t *testing.T) {
	config := DedupConfig{
		Enabled: false,
	}

	m := NewManager(config)

	data := []byte("test")
	_, err := m.ProcessFile("/test/file.txt", int64(len(data)), data)
	if err == nil {
		t.Error("Expected error when dedup is disabled")
	}
}

func TestFileTooSmall(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		MinFileSize: 100,
	}

	m := NewManager(config)

	data := []byte("small")
	_, err := m.ProcessFile("/test/small.txt", int64(len(data)), data)
	if err == nil {
		t.Error("Expected error for file too small")
	}
}

func TestRunDedupJob(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	job, err := m.RunDedupJob("/data/backup")
	if err != nil {
		t.Fatalf("Failed to run dedup job: %v", err)
	}

	if job.Status != "completed" {
		t.Errorf("Expected status completed, got %s", job.Status)
	}
}

func TestGetStats(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	data1 := []byte("unique data 1")
	data2 := []byte("unique data 2")

	m.ProcessFile("/test/file1.txt", int64(len(data1)), data1)
	m.ProcessFile("/test/file2.txt", int64(len(data2)), data2)

	stats := m.GetStats()
	if stats.TotalFiles != 2 {
		t.Errorf("Expected 2 total files, got %d", stats.TotalFiles)
	}

	if stats.UniqueFiles != 2 {
		t.Errorf("Expected 2 unique files, got %d", stats.UniqueFiles)
	}
}

func TestChunkOperations(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	data := []byte("test data for chunk operations")
	m.ProcessFile("/test/file.txt", int64(len(data)), data)

	chunks := m.ListChunks()
	if len(chunks) == 0 {
		t.Error("Expected chunks to be created")
	}

	for _, chunk := range chunks {
		fetched, err := m.GetChunk(chunk.Hash)
		if err != nil {
			t.Errorf("Failed to get chunk %s: %v", chunk.Hash, err)
		}

		if fetched.Hash != chunk.Hash {
			t.Errorf("Expected hash %s, got %s", chunk.Hash, fetched.Hash)
		}
	}
}

func TestFileOperations(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	data := []byte("test data for file operations")
	m.ProcessFile("/test/file.txt", int64(len(data)), data)

	file, err := m.GetFile("/test/file.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}

	if file.FilePath != "/test/file.txt" {
		t.Errorf("Expected path /test/file.txt, got %s", file.FilePath)
	}

	files := m.ListFiles()
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

func TestJobOperations(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	job, _ := m.RunDedupJob("/data")

	fetched, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if fetched.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, fetched.ID)
	}

	jobs := m.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
}

func TestConfigOperations(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	current := m.GetConfig()
	if !current.Enabled {
		t.Error("Expected dedup to be enabled")
	}

	newConfig := DedupConfig{
		Enabled:     false,
		ChunkSize:   8,
		MinFileSize: 100,
	}

	m.UpdateConfig(newConfig)

	updated := m.GetConfig()
	if updated.Enabled {
		t.Error("Expected dedup to be disabled")
	}

	if updated.ChunkSize != 8 {
		t.Errorf("Expected chunk size 8, got %d", updated.ChunkSize)
	}
}

func TestDedupRatio(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	data := []byte("same data for ratio test")

	m.ProcessFile("/test/file1.txt", int64(len(data)), data)
	m.ProcessFile("/test/file2.txt", int64(len(data)), data)
	m.ProcessFile("/test/file3.txt", int64(len(data)), data)

	stats := m.GetStats()
	if stats.DedupRatio < 1.0 {
		t.Errorf("Expected dedup ratio >= 1.0, got %f", stats.DedupRatio)
	}

	if stats.SpaceSavedPct <= 0 {
		t.Errorf("Expected positive space saved percentage, got %f", stats.SpaceSavedPct)
	}
}

func TestMultipleChunkSizes(t *testing.T) {
	sizes := []int{1, 2, 4, 8, 16}

	for _, size := range sizes {
		config := DedupConfig{
			Enabled:     true,
			ChunkSize:   size,
			MinFileSize: 5,
		}

		m := NewManager(config)

		data := []byte("test data with enough bytes for multiple chunks")
		info, err := m.ProcessFile("/test/file.txt", int64(len(data)), data)
		if err != nil {
			t.Errorf("Failed with chunk size %d: %v", size, err)
			continue
		}

		if len(info.ChunkHashes) == 0 {
			t.Errorf("Expected chunks with chunk size %d", size)
		}
	}
}

func TestConcurrentProcessing(t *testing.T) {
	config := DedupConfig{
		Enabled:     true,
		ChunkSize:   4,
		MinFileSize: 5,
	}

	m := NewManager(config)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			data := []byte(fmt.Sprintf("concurrent test data %d", idx))
			m.ProcessFile("/test/file"+string(rune('0'+idx))+".txt", int64(len(data)), data)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := m.GetStats()
	if stats.TotalFiles != 10 {
		t.Errorf("Expected 10 total files, got %d", stats.TotalFiles)
	}
}
