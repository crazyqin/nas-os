package transfer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewChunkedUploader(t *testing.T) {
	uploader := NewChunkedUploader(0, 0)
	if uploader == nil {
		t.Fatal("NewChunkedUploader returned nil")
	}
	if uploader.chunkSize != DefaultChunkSize {
		t.Errorf("expected default chunk size %d, got %d", DefaultChunkSize, uploader.chunkSize)
	}
	if uploader.maxRetries != MaxRetries {
		t.Errorf("expected default max retries %d, got %d", MaxRetries, uploader.maxRetries)
	}
}

func TestNewChunkedUploader_CustomValues(t *testing.T) {
	uploader := NewChunkedUploader(1024, 5)
	if uploader.chunkSize != 1024 {
		t.Errorf("expected chunk size 1024, got %d", uploader.chunkSize)
	}
	if uploader.maxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", uploader.maxRetries)
	}
}

func TestChunkInfo_Fields(t *testing.T) {
	chunk := ChunkInfo{
		Index:  0,
		Offset: 0,
		Size:   1024,
		SHA256: "abc123",
	}

	if chunk.Index != 0 {
		t.Error("Index mismatch")
	}
	if chunk.Size != 1024 {
		t.Error("Size mismatch")
	}
	if chunk.SHA256 != "abc123" {
		t.Error("SHA256 mismatch")
	}
}

func TestUploadProgress_Fields(t *testing.T) {
	progress := UploadProgress{
		TotalChunks:    10,
		UploadedChunks: 5,
		TotalBytes:     10000,
		UploadedBytes:  5000,
		Progress:       50.0,
		Speed:          1000.0,
		ETA:            5,
	}

	if progress.TotalChunks != 10 {
		t.Error("TotalChunks mismatch")
	}
	if progress.Progress != 50.0 {
		t.Error("Progress mismatch")
	}
}

func TestChunkedUploader_SplitFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world, this is a test file for chunking")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	uploader := NewChunkedUploader(10, 3) // small chunks for testing
	chunks, err := uploader.SplitFile(testFile, tmpDir)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	// Verify chunk info
	for _, chunk := range chunks {
		t.Logf("Chunk %d: offset=%d, size=%d, sha256=%s", chunk.Index, chunk.Offset, chunk.Size, chunk.SHA256)
	}
}

func TestCalculateSHA256(t *testing.T) {
	data := []byte("hello world")
	sha256Hash := calculateSHA256(data)

	if sha256Hash == "" {
		t.Error("calculateSHA256 should not return empty")
	}
	if len(sha256Hash) != 64 {
		t.Errorf("SHA256 hash should be 64 chars, got %d", len(sha256Hash))
	}
}

func TestCalculateFileSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world for file md5")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := CalculateFileSHA256(testFile)
	if err != nil {
		t.Fatalf("CalculateFileSHA256 failed: %v", err)
	}
	if hash == "" {
		t.Error("CalculateFileSHA256 should not return empty")
	}
	if len(hash) != 64 {
		t.Errorf("SHA256 hash should be 64 chars, got %d", len(hash))
	}
}

func TestCompressFile(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.txt")
	outputPath := filepath.Join(tmpDir, "test.txt.gz")

	testData := []byte("test data to compress")
	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	err := CompressFile(inputPath, outputPath)
	if err != nil {
		t.Errorf("CompressFile failed: %v", err)
	}

	// Check compressed file exists
	info, err := os.Stat(outputPath)
	if os.IsNotExist(err) {
		t.Error("compressed file should exist")
		return
	}
	if info.Size() == 0 {
		t.Error("compressed file should not be empty")
	}
}

func TestDecompressFile(t *testing.T) {
	tmpDir := t.TempDir()

	// First compress, then decompress
	originalPath := filepath.Join(tmpDir, "original.txt")
	testData := []byte("test data to compress and decompress")
	if err := os.WriteFile(originalPath, testData, 0644); err != nil {
		t.Fatalf("failed to create original file: %v", err)
	}

	compressedPath := filepath.Join(tmpDir, "compressed.txt.gz")
	if err := CompressFile(originalPath, compressedPath); err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	decompressedPath := filepath.Join(tmpDir, "decompressed.txt")
	err := DecompressFile(compressedPath, decompressedPath)
	if err != nil {
		t.Errorf("DecompressFile failed: %v", err)
	}

	// Verify content matches
	result, err := os.ReadFile(decompressedPath)
	if err != nil {
		t.Fatalf("failed to read decompressed file: %v", err)
	}
	if string(result) != string(testData) {
		t.Error("decompressed content should match original")
	}
}
