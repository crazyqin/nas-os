package transfer

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultChunkSize 默认分块大小 4MB
	DefaultChunkSize = 4 * 1024 * 1024
	// MaxRetries 最大重试次数
	MaxRetries = 3
)

// ChunkInfo represents a file chunk
type ChunkInfo struct {
	Index      int       `json:"index"`
	Offset     int64     `json:"offset"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"` // 使用 SHA256 替代 MD5
	Uploaded   bool      `json:"uploaded"`
	UploadTime time.Time `json:"upload_time,omitempty"`
}

// UploadProgress tracks upload progress
type UploadProgress struct {
	TotalChunks    int     `json:"total_chunks"`
	UploadedChunks int     `json:"uploaded_chunks"`
	TotalBytes     int64   `json:"total_bytes"`
	UploadedBytes  int64   `json:"uploaded_bytes"`
	Progress       float64 `json:"progress"` // 0-100
	Speed          float64 `json:"speed"`    // bytes/s
	ETA            int64   `json:"eta"`      // seconds
}

// ChunkedUploader handles chunked file uploads
type ChunkedUploader struct {
	chunkSize  int64
	maxRetries int
}

// NewChunkedUploader creates a new chunked uploader
func NewChunkedUploader(chunkSize int64, maxRetries int) *ChunkedUploader {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if maxRetries <= 0 {
		maxRetries = MaxRetries
	}

	return &ChunkedUploader{
		chunkSize:  chunkSize,
		maxRetries: maxRetries,
	}
}

// SplitFile splits a file into chunks
func (u *ChunkedUploader) SplitFile(filePath string, outputDir string) ([]ChunkInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			_ = err // 忽略关闭错误
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := stat.Size()
	numChunks := (fileSize + u.chunkSize - 1) / u.chunkSize

	chunks := make([]ChunkInfo, numChunks)
	buf := make([]byte, u.chunkSize)

	for i := int64(0); i < numChunks; i++ {
		offset := i * u.chunkSize
		n, err := file.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		chunkData := buf[:n]
		chunkSHA256 := calculateSHA256(chunkData)

		chunks[i] = ChunkInfo{
			Index:  int(i),
			Offset: offset,
			Size:   int64(n),
			SHA256: chunkSHA256,
		}

		// Write chunk to file
		chunkPath := filepath.Join(outputDir, fmt.Sprintf("%s.chunk.%d", filepath.Base(filePath), i))
		if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write chunk %d: %w", i, err)
		}
	}

	return chunks, nil
}

// MergeChunks merges uploaded chunks into final file
func (u *ChunkedUploader) MergeChunks(chunkDir string, outputPath string, totalChunks int) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := outputFile.Close(); err != nil {
			_ = err // 忽略关闭错误
		}
	}()

	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("*.chunk.%d", i))
		matches, err := filepath.Glob(chunkPath)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("chunk %d not found", i)
		}

		chunkData, err := os.ReadFile(matches[0])
		if err != nil {
			return err
		}

		if _, err := outputFile.Write(chunkData); err != nil {
			return err
		}
	}

	return nil
}

// UploadChunk uploads a single chunk with retry
func (u *ChunkedUploader) UploadChunk(
	chunkPath string,
	uploadFunc func(data []byte, chunk ChunkInfo) error,
	chunk ChunkInfo,
) error {
	var lastErr error

	for attempt := 0; attempt < u.maxRetries; attempt++ {
		data, err := os.ReadFile(chunkPath)
		if err != nil {
			return err
		}

		if err := uploadFunc(data, chunk); err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		chunk.Uploaded = true
		chunk.UploadTime = time.Now()
		return nil
	}

	return fmt.Errorf("failed after %d attempts: %w", u.maxRetries, lastErr)
}

// CalculateFileSHA256 calculates SHA256 hash of a file
func CalculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			_ = err // 忽略关闭错误
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func calculateSHA256(data []byte) string {
	hash := sha256.New()
	_, _ = hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// CompressReader creates a gzip compression reader
func CompressReader(reader io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	gw := gzip.NewWriter(pw)

	go func() {
		// 关闭资源，忽略错误（通过 CloseWithError 传递主要错误）
		defer func() {
			if err := pw.Close(); err != nil {
				_ = err // 忽略关闭错误
			}
		}()
		defer func() {
			if err := gw.Close(); err != nil {
				_ = err // 忽略关闭错误
			}
		}()

		_, err := io.Copy(gw, reader)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, nil
}

// DecompressReader creates a gzip decompression reader
func DecompressReader(reader io.Reader) (io.Reader, error) {
	return gzip.NewReader(reader)
}

// CompressFile compresses a file
func CompressFile(inputPath, outputPath string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	gw := gzip.NewWriter(outputFile)
	defer gw.Close()

	_, err = io.Copy(gw, inputFile)
	return err
}

// DecompressFile decompresses a file
func DecompressFile(inputPath, outputPath string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	gr, err := gzip.NewReader(inputFile)
	if err != nil {
		return err
	}
	defer gr.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// 限制解压后大小（最大 10GB），防止解压缩炸弹攻击
	const maxDecompressSize = 10 << 30 // 10GB
	limitedReader := io.LimitReader(gr, maxDecompressSize)
	_, err = io.Copy(outputFile, limitedReader)
	return err
}

// ResumableUpload supports resumable uploads
type ResumableUpload struct {
	filePath     string
	fileSize     int64
	chunkSize    int64
	uploadedSize int64
	mu           sync.Mutex
}

// NewResumableUpload creates a new resumable upload session
func NewResumableUpload(filePath string, chunkSize int64) (*ResumableUpload, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	return &ResumableUpload{
		filePath:  filePath,
		fileSize:  stat.Size(),
		chunkSize: chunkSize,
	}, nil
}

// GetNextChunk returns the next chunk to upload
func (r *ResumableUpload) GetNextChunk() ([]byte, int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.uploadedSize >= r.fileSize {
		return nil, 0, 0, io.EOF
	}

	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// 忽略关闭错误
		}
	}()

	buf := make([]byte, r.chunkSize)
	n, err := file.ReadAt(buf, r.uploadedSize)
	if err != nil && err != io.EOF {
		return nil, 0, 0, err
	}

	offset := r.uploadedSize
	r.uploadedSize += int64(n)

	return buf[:n], offset, int64(n), nil
}

// GetProgress returns upload progress
func (r *ResumableUpload) GetProgress() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return float64(r.uploadedSize) / float64(r.fileSize) * 100
}

// SetUploadedSize sets the already uploaded size (for resuming)
func (r *ResumableUpload) SetUploadedSize(size int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uploadedSize = size
}

// GetUploadedSize returns the uploaded size
func (r *ResumableUpload) GetUploadedSize() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.uploadedSize
}

// IsComplete checks if upload is complete
func (r *ResumableUpload) IsComplete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.uploadedSize >= r.fileSize
}

// ChunkedReader reads a file in chunks
type ChunkedReader struct {
	file      *os.File
	chunkSize int
	buf       []byte
	offset    int64
	eof       bool
}

// NewChunkedReader creates a new chunked reader
func NewChunkedReader(filePath string, chunkSize int) (*ChunkedReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	return &ChunkedReader{
		file:      file,
		chunkSize: chunkSize,
		buf:       make([]byte, chunkSize),
	}, nil
}

// ReadNextChunk reads the next chunk
func (r *ChunkedReader) ReadNextChunk() ([]byte, error) {
	if r.eof {
		return nil, io.EOF
	}

	n, err := r.file.Read(r.buf)
	if err != nil {
		if err == io.EOF {
			r.eof = true
			if n > 0 {
				return r.buf[:n], nil
			}
			return nil, io.EOF
		}
		return nil, err
	}

	r.offset += int64(n)
	return r.buf[:n], nil
}

// GetOffset returns current read offset
func (r *ChunkedReader) GetOffset() int64 {
	return r.offset
}

// Close closes the reader
func (r *ChunkedReader) Close() error {
	return r.file.Close()
}

// ChunkedWriter writes data in chunks
type ChunkedWriter struct {
	file      *os.File
	chunkSize int
	buf       []byte
	offset    int64
	mu        sync.Mutex
}

// NewChunkedWriter creates a new chunked writer
func NewChunkedWriter(filePath string, chunkSize int) (*ChunkedWriter, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	return &ChunkedWriter{
		file:      file,
		chunkSize: chunkSize,
		buf:       make([]byte, chunkSize),
	}, nil
}

// WriteChunk writes a chunk
func (w *ChunkedWriter) WriteChunk(data []byte) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.file.Write(data)
	if err != nil {
		return 0, err
	}

	w.offset += int64(n)
	return int64(n), nil
}

// WriteAt writes data at specific offset
func (w *ChunkedWriter) WriteAt(data []byte, offset int64) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.file.WriteAt(data, offset)
	if err != nil {
		return 0, err
	}

	return int64(n), nil
}

// GetOffset returns current write offset
func (w *ChunkedWriter) GetOffset() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.offset
}

// Close closes the writer
func (w *ChunkedWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// BufferPool provides reusable buffers
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(bufferSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() *[]byte {
	buf, ok := p.pool.Get().(*[]byte)
	if !ok {
		// 类型断言失败，返回新分配的 buffer
		newBuf := make([]byte, 0)
		return &newBuf
	}
	return buf
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf *[]byte) {
	p.pool.Put(buf)
}

// CopyWithProgress copies data with progress tracking
func CopyWithProgress(dst io.Writer, src io.Reader, totalSize int64, progressFunc func(int64)) (int64, error) {
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := src.Read(buf)
		if n > 0 {
			nw, ew := dst.Write(buf[:n])
			written += int64(nw)

			if progressFunc != nil {
				progressFunc(written)
			}

			if ew != nil {
				return written, ew
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return written, err
		}
	}

	return written, nil
}

// StreamCopy copies data with buffering
func StreamCopy(dst io.Writer, src io.Reader, bufferSize int) (int64, error) {
	if bufferSize <= 0 {
		bufferSize = 32 * 1024
	}

	buf := bufio.NewReaderSize(src, bufferSize)
	return io.Copy(dst, buf)
}
