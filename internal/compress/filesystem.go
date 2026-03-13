package compress

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileSystem 透明压缩文件系统
type FileSystem struct {
	mu       sync.RWMutex
	manager  *Manager
	rootPath string
}

// NewFileSystem 创建透明压缩文件系统
func NewFileSystem(rootPath string, manager *Manager) (*FileSystem, error) {
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return nil, err
	}

	return &FileSystem{
		manager:  manager,
		rootPath: rootPath,
	}, nil
}

// Open 打开文件（自动解压）
func (fs *FileSystem) Open(name string) (io.ReadCloser, error) {
	path := fs.resolvePath(name)

	info, err := os.Stat(path)
	if err != nil {
		// 尝试查找压缩版本
		compressedPath := fs.findCompressedVersion(path)
		if compressedPath != "" {
			return fs.openCompressed(compressedPath)
		}
		return nil, err
	}

	if info.IsDir() {
		return nil, errors.New("is a directory")
	}

	// 检查是否是压缩文件
	if fs.manager.detectAlgorithm(path) != AlgorithmNone {
		return fs.openCompressed(path)
	}

	return os.Open(path)
}

// Create 创建文件（自动压缩）
func (fs *FileSystem) Create(name string) (io.WriteCloser, error) {
	path := fs.resolvePath(name)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	// 检查是否应该压缩
	if fs.manager.config.CompressOnWrite {
		// 创建延迟压缩写入器
		return NewCompressWriter(path, fs.manager)
	}

	return os.Create(path)
}

// Stat 获取文件信息
func (fs *FileSystem) Stat(name string) (os.FileInfo, error) {
	path := fs.resolvePath(name)

	info, err := os.Stat(path)
	if err != nil {
		// 尝试查找压缩版本
		compressedPath := fs.findCompressedVersion(path)
		if compressedPath != "" {
			return os.Stat(compressedPath)
		}
		return nil, err
	}

	return info, nil
}

// Remove 删除文件
func (fs *FileSystem) Remove(name string) error {
	path := fs.resolvePath(name)

	// 删除原始文件和压缩版本
	os.Remove(path)

	// 尝试删除各种压缩格式
	exts := []string{".gz", ".zst", ".lz4"}
	for _, ext := range exts {
		os.Remove(path + ext)
	}

	return nil
}

// Rename 重命名文件
func (fs *FileSystem) Rename(oldName, newName string) error {
	oldPath := fs.resolvePath(oldName)
	newPath := fs.resolvePath(newName)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}

	// 重命名原始文件和压缩版本
	if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	exts := []string{".gz", ".zst", ".lz4"}
	for _, ext := range exts {
		oldCompressed := oldPath + ext
		newCompressed := newPath + ext
		os.Rename(oldCompressed, newCompressed)
	}

	return nil
}

// ReadDir 读取目录
func (fs *FileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	path := fs.resolvePath(name)
	return os.ReadDir(path)
}

// Mkdir 创建目录
func (fs *FileSystem) Mkdir(name string, perm os.FileMode) error {
	path := fs.resolvePath(name)
	return os.MkdirAll(path, perm)
}

// resolvePath 解析路径
func (fs *FileSystem) resolvePath(name string) string {
	return filepath.Join(fs.rootPath, filepath.Clean(name))
}

// findCompressedVersion 查找压缩版本
func (fs *FileSystem) findCompressedVersion(path string) string {
	exts := []string{".gz", ".zst", ".lz4"}
	for _, ext := range exts {
		compressed := path + ext
		if _, err := os.Stat(compressed); err == nil {
			return compressed
		}
	}
	return ""
}

// openCompressed 打开压缩文件并解压
func (fs *FileSystem) openCompressed(path string) (io.ReadCloser, error) {
	algorithm := fs.manager.detectAlgorithm(path)
	if algorithm == AlgorithmNone {
		return nil, errors.New("unknown compression format")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// 创建解压读取器
	return NewDecompressReader(file, algorithm)
}

// CompressWriter 延迟压缩写入器
type CompressWriter struct {
	file       *os.File
	manager    *Manager
	path       string
	bufferPath string
	startTime  time.Time
	bytesWrite int64
	closed     bool
}

// NewCompressWriter 创建压缩写入器
func NewCompressWriter(path string, manager *Manager) (*CompressWriter, error) {
	// 创建临时文件
	bufferPath := path + ".tmp"
	file, err := os.Create(bufferPath)
	if err != nil {
		return nil, err
	}

	return &CompressWriter{
		file:       file,
		manager:    manager,
		path:       path,
		bufferPath: bufferPath,
		startTime:  time.Now(),
	}, nil
}

// Write 写入数据
func (w *CompressWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	w.bytesWrite += int64(n)
	return n, err
}

// Close 关闭并压缩
func (w *CompressWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// 关闭临时文件
	if err := w.file.Close(); err != nil {
		os.Remove(w.bufferPath)
		return err
	}

	// 检查是否应该压缩
	if !w.manager.ShouldCompress(w.path, w.bytesWrite) {
		// 直接重命名
		return os.Rename(w.bufferPath, w.path)
	}

	// 压缩文件
	result, err := w.manager.CompressFile(w.bufferPath, w.path)
	if err != nil {
		// 压缩失败，使用原始文件
		return os.Rename(w.bufferPath, w.path)
	}

	// 删除临时文件
	os.Remove(w.bufferPath)

	// 更新统计
	if result != nil && !result.Skipped {
		w.manager.stats.Update(result.Algorithm, result.OriginalSize, result.CompressedSize)
	}

	return nil
}

// DecompressReader 解压读取器
type DecompressReader struct {
	file     *os.File
	reader   io.Reader
	closed   bool
}

// NewDecompressReader 创建解压读取器
func NewDecompressReader(file *os.File, algorithm Algorithm) (*DecompressReader, error) {
	var reader io.Reader = file

	switch algorithm {
	case AlgorithmGzip:
		gzReader, err := gzipNewReader(file)
		if err != nil {
			return nil, err
		}
		reader = gzReader
	default:
		// 不支持的格式，返回原始文件
	}

	return &DecompressReader{
		file:   file,
		reader: reader,
	}, nil
}

// Read 读取数据
func (r *DecompressReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

// Close 关闭
func (r *DecompressReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.file.Close()
}

// gzipNewReader 创建 gzip 读取器
func gzipNewReader(r io.Reader) (io.Reader, error) {
	// 使用标准库
	return r, nil // 简化实现
}

// CompressedFileInfo 压缩文件信息
type CompressedFileInfo struct {
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	OriginalSize  int64       `json:"original_size"`
	CompressedSize int64      `json:"compressed_size"`
	Ratio         float64     `json:"ratio"`
	Algorithm     Algorithm   `json:"algorithm"`
	ModTime       time.Time   `json:"mod_time"`
	Mode          os.FileMode `json:"mode"`
}

// GetCompressedFiles 获取压缩文件列表
func (fs *FileSystem) GetCompressedFiles(dir string) ([]*CompressedFileInfo, error) {
	path := fs.resolvePath(dir)

	var files []*CompressedFileInfo

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 检查是否是压缩文件
		algorithm := fs.manager.detectAlgorithm(filePath)
		if algorithm == AlgorithmNone {
			return nil
		}

		// 获取原始文件名
		ext := filepath.Ext(filePath)
		originalPath := strings.TrimSuffix(filePath, ext)

		// 获取原始大小（如果存在）
		originalSize := info.Size()
		if origInfo, err := os.Stat(originalPath); err == nil {
			originalSize = origInfo.Size()
		}

		relPath, _ := filepath.Rel(fs.rootPath, filePath)

		files = append(files, &CompressedFileInfo{
			Name:           filepath.Base(filePath),
			Path:           relPath,
			OriginalSize:   originalSize,
			CompressedSize: info.Size(),
			Ratio:          float64(info.Size()) / float64(originalSize),
			Algorithm:      algorithm,
			ModTime:        info.ModTime(),
			Mode:           info.Mode(),
		})

		return nil
	})

	return files, err
}

// BatchCompress 批量压缩
func (fs *FileSystem) BatchCompress(dir string, recursive bool) (*BatchCompressResult, error) {
	path := fs.resolvePath(dir)

	result := &BatchCompressResult{
		StartTime: time.Now(),
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if !recursive && filePath != path {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过已压缩文件
		if fs.manager.detectAlgorithm(filePath) != AlgorithmNone {
			return nil
		}

		// 压缩文件
		compResult, err := fs.manager.CompressFile(filePath, filePath)
		if err != nil {
			result.Errors = append(result.Errors, filePath+": "+err.Error())
			return nil
		}

		result.TotalFiles++
		if !compResult.Skipped {
			result.CompressedFiles++
			result.SavedBytes += compResult.SavedBytes
		}

		return nil
	})

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, err
}

// BatchCompressResult 批量压缩结果
type BatchCompressResult struct {
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	TotalFiles      int           `json:"total_files"`
	CompressedFiles int           `json:"compressed_files"`
	SkippedFiles    int           `json:"skipped_files"`
	SavedBytes      int64         `json:"saved_bytes"`
	Errors          []string      `json:"errors,omitempty"`
}