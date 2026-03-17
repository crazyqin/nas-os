package transfer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// ========== MergeChunks Tests ==========

func TestMergeChunks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test chunks
	testContent := []byte("hello world test for merge")
	chunk1 := testContent[:10]
	chunk2 := testContent[10:20]
	chunk3 := testContent[20:]

	baseName := "test.txt"
	if err := os.WriteFile(filepath.Join(tmpDir, baseName+".chunk.0"), chunk1, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, baseName+".chunk.1"), chunk2, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, baseName+".chunk.2"), chunk3, 0644); err != nil {
		t.Fatal(err)
	}

	uploader := NewChunkedUploader(10, 3)
	outputPath := filepath.Join(tmpDir, "merged.txt")
	err := uploader.MergeChunks(tmpDir, outputPath, 3)
	if err != nil {
		t.Fatalf("MergeChunks failed: %v", err)
	}

	// Verify merged content
	result, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, testContent) {
		t.Errorf("merged content mismatch: got %s, want %s", result, testContent)
	}
}

func TestMergeChunks_MissingChunk(t *testing.T) {
	tmpDir := t.TempDir()

	uploader := NewChunkedUploader(10, 3)
	outputPath := filepath.Join(tmpDir, "merged.txt")
	err := uploader.MergeChunks(tmpDir, outputPath, 3)
	if err == nil {
		t.Error("expected error for missing chunks")
	}
}

// ========== UploadChunk Tests ==========

func TestUploadChunk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test chunk file
	chunkPath := filepath.Join(tmpDir, "test.chunk.0")
	chunkData := []byte("test chunk data")
	if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
		t.Fatal(err)
	}

	uploader := NewChunkedUploader(10, 3)
	chunk := ChunkInfo{Index: 0, Size: int64(len(chunkData))}

	uploadFunc := func(data []byte, c ChunkInfo) error {
		return nil // success
	}

	err := uploader.UploadChunk(chunkPath, uploadFunc, chunk)
	if err != nil {
		t.Errorf("UploadChunk failed: %v", err)
	}
}

func TestUploadChunk_Retry(t *testing.T) {
	tmpDir := t.TempDir()

	chunkPath := filepath.Join(tmpDir, "test.chunk.0")
	chunkData := []byte("test chunk data")
	if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
		t.Fatal(err)
	}

	uploader := NewChunkedUploader(10, 1) // 1 retry
	chunk := ChunkInfo{Index: 0, Size: int64(len(chunkData))}

	attempts := 0
	uploadFunc := func(data []byte, c ChunkInfo) error {
		attempts++
		return io.EOF // simulate failure
	}

	err := uploader.UploadChunk(chunkPath, uploadFunc, chunk)
	if err == nil {
		t.Error("expected error after retries")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestUploadChunk_MissingFile(t *testing.T) {
	uploader := NewChunkedUploader(10, 3)
	uploadFunc := func(data []byte, c ChunkInfo) error { return nil }

	err := uploader.UploadChunk("/nonexistent/file", uploadFunc, ChunkInfo{})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ========== CompressReader/DecompressReader Tests ==========

func TestCompressReader(t *testing.T) {
	input := bytes.NewBufferString("hello world test data for compression")

	compressed, err := CompressReader(input)
	if err != nil {
		t.Fatalf("CompressReader failed: %v", err)
	}

	// Decompress and verify
	decompressed, err := DecompressReader(compressed)
	if err != nil {
		t.Fatalf("DecompressReader failed: %v", err)
	}

	result, err := io.ReadAll(decompressed)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if string(result) != "hello world test data for compression" {
		t.Errorf("content mismatch: got %s", result)
	}
}

func TestDecompressReader_InvalidData(t *testing.T) {
	input := bytes.NewBufferString("not valid gzip data")

	_, err := DecompressReader(input)
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}
}

// ========== ResumableUpload Tests ==========

func TestNewResumableUpload(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content for resumable upload")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 10)
	if err != nil {
		t.Fatalf("NewResumableUpload failed: %v", err)
	}
	if ru == nil {
		t.Fatal("NewResumableUpload returned nil")
	}
}

func TestNewResumableUpload_DefaultChunkSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ru.chunkSize != DefaultChunkSize {
		t.Errorf("expected default chunk size, got %d", ru.chunkSize)
	}
}

func TestNewResumableUpload_MissingFile(t *testing.T) {
	_, err := NewResumableUpload("/nonexistent/file", 10)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResumableUpload_GetNextChunk(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world test")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Get first chunk
	data, offset, size, err := ru.GetNextChunk()
	if err != nil {
		t.Fatalf("GetNextChunk failed: %v", err)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
	if size != 5 {
		t.Errorf("expected size 5, got %d", size)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %s", data)
	}

	// Get remaining chunks until EOF
	for {
		_, _, _, err := ru.GetNextChunk()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestResumableUpload_GetProgress(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 5)
	if err != nil {
		t.Fatal(err)
	}

	progress := ru.GetProgress()
	if progress != 0 {
		t.Errorf("expected initial progress 0, got %f", progress)
	}

	ru.SetUploadedSize(6)
	progress = ru.GetProgress()
	if progress <= 0 {
		t.Errorf("expected progress > 0 after SetUploadedSize, got %f", progress)
	}
}

func TestResumableUpload_SetAndGetUploadedSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 10)
	if err != nil {
		t.Fatal(err)
	}

	ru.SetUploadedSize(100)
	if ru.GetUploadedSize() != 100 {
		t.Errorf("expected uploaded size 100, got %d", ru.GetUploadedSize())
	}
}

func TestResumableUpload_IsComplete(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	ru, err := NewResumableUpload(testFile, 10)
	if err != nil {
		t.Fatal(err)
	}

	if ru.IsComplete() {
		t.Error("should not be complete initially")
	}

	ru.SetUploadedSize(1000) // more than file size
	if !ru.IsComplete() {
		t.Error("should be complete after setting uploaded size >= file size")
	}
}

// ========== ChunkedReader Tests ==========

func TestNewChunkedReader(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	cr, err := NewChunkedReader(testFile, 5)
	if err != nil {
		t.Fatalf("NewChunkedReader failed: %v", err)
	}
	defer cr.Close()

	if cr == nil {
		t.Fatal("NewChunkedReader returned nil")
	}
}

func TestNewChunkedReader_DefaultChunkSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	cr, err := NewChunkedReader(testFile, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer cr.Close()

	if cr.chunkSize != DefaultChunkSize {
		t.Errorf("expected default chunk size, got %d", cr.chunkSize)
	}
}

func TestChunkedReader_ReadNextChunk(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world test")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	cr, err := NewChunkedReader(testFile, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer cr.Close()

	// Read first chunk
	chunk, err := cr.ReadNextChunk()
	if err != nil {
		t.Fatalf("ReadNextChunk failed: %v", err)
	}
	if string(chunk) != "hello" {
		t.Errorf("expected 'hello', got %s", chunk)
	}

	// Read until EOF
	for {
		_, err := cr.ReadNextChunk()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestChunkedReader_GetOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	cr, err := NewChunkedReader(testFile, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer cr.Close()

	if cr.GetOffset() != 0 {
		t.Error("initial offset should be 0")
	}

	cr.ReadNextChunk()
	if cr.GetOffset() != 5 {
		t.Errorf("offset should be 5 after reading, got %d", cr.GetOffset())
	}
}

func TestChunkedReader_MissingFile(t *testing.T) {
	_, err := NewChunkedReader("/nonexistent/file", 10)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ========== ChunkedWriter Tests ==========

func TestNewChunkedWriter(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	cw, err := NewChunkedWriter(outputFile, 10)
	if err != nil {
		t.Fatalf("NewChunkedWriter failed: %v", err)
	}
	defer cw.Close()

	if cw == nil {
		t.Fatal("NewChunkedWriter returned nil")
	}
}

func TestChunkedWriter_WriteChunk(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	cw, err := NewChunkedWriter(outputFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer cw.Close()

	n, err := cw.WriteChunk([]byte("hello"))
	if err != nil {
		t.Fatalf("WriteChunk failed: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
}

func TestChunkedWriter_WriteAt(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	cw, err := NewChunkedWriter(outputFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer cw.Close()

	n, err := cw.WriteAt([]byte("world"), 0)
	if err != nil {
		t.Fatalf("WriteAt failed: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
}

func TestChunkedWriter_GetOffset(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	cw, err := NewChunkedWriter(outputFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer cw.Close()

	if cw.GetOffset() != 0 {
		t.Error("initial offset should be 0")
	}

	cw.WriteChunk([]byte("hello"))
	if cw.GetOffset() != 5 {
		t.Errorf("offset should be 5 after write, got %d", cw.GetOffset())
	}
}

// ========== BufferPool Tests ==========

func TestNewBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)
	if pool == nil {
		t.Fatal("NewBufferPool returned nil")
	}
}

func TestBufferPool_GetPut(t *testing.T) {
	pool := NewBufferPool(1024)

	buf := pool.Get()
	if buf == nil {
		t.Fatal("Get returned nil")
	}

	pool.Put(buf)
}

func TestBufferPool_MultipleGetPut(t *testing.T) {
	pool := NewBufferPool(1024)

	buf1 := pool.Get()
	buf2 := pool.Get()

	if buf1 == nil || buf2 == nil {
		t.Fatal("Get returned nil")
	}

	pool.Put(buf1)
	pool.Put(buf2)
}

// ========== CopyWithProgress Tests ==========

func TestCopyWithProgress(t *testing.T) {
	src := bytes.NewBufferString("hello world test data")
	dst := &bytes.Buffer{}
	totalSize := int64(src.Len())

	var lastProgress int64
	n, err := CopyWithProgress(dst, src, totalSize, func(written int64) {
		lastProgress = written
	})

	if err != nil {
		t.Fatalf("CopyWithProgress failed: %v", err)
	}
	if n != totalSize {
		t.Errorf("expected %d bytes copied, got %d", totalSize, n)
	}
	if lastProgress != totalSize {
		t.Errorf("expected final progress %d, got %d", totalSize, lastProgress)
	}
}

func TestCopyWithProgress_NilProgressFunc(t *testing.T) {
	src := bytes.NewBufferString("test")
	dst := &bytes.Buffer{}

	n, err := CopyWithProgress(dst, src, int64(src.Len()), nil)

	if err != nil {
		t.Fatalf("CopyWithProgress failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes, got %d", n)
	}
}

// ========== StreamCopy Tests ==========

func TestStreamCopy(t *testing.T) {
	src := bytes.NewBufferString("hello world test data for stream copy")
	dst := &bytes.Buffer{}

	n, err := StreamCopy(dst, src, 1024)

	if err != nil {
		t.Fatalf("StreamCopy failed: %v", err)
	}
	if n == 0 {
		t.Error("expected non-zero bytes copied")
	}
}

func TestStreamCopy_DefaultBufferSize(t *testing.T) {
	src := bytes.NewBufferString("test")
	dst := &bytes.Buffer{}

	n, err := StreamCopy(dst, src, 0)

	if err != nil {
		t.Fatalf("StreamCopy failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes, got %d", n)
	}
}

// ========== Edge Cases ==========

func TestCalculateFileSHA256_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := CalculateFileSHA256(emptyFile)
	if err != nil {
		t.Fatalf("CalculateFileSHA256 failed: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty for empty file")
	}
}

func TestDecompressFile_InvalidInput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "invalid.gz")
	outputPath := filepath.Join(tmpDir, "output.txt")

	// Write invalid gzip data
	if err := os.WriteFile(inputPath, []byte("not gzip data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := DecompressFile(inputPath, outputPath)
	if err == nil {
		t.Error("expected error for invalid gzip input")
	}
}