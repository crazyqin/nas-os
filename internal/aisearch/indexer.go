// Package aisearch 提供文件索引器功能
package aisearch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Indexer 文件索引器.
type Indexer struct {
	engine    *Engine
	config    *SearchConfig
	extractor ContentExtractor
	encoder   VectorEncoder
	workers   int
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	running   bool
	stats     *IndexerStats
}

// IndexerStats 索引器统计.
type IndexerStats struct {
	TotalFiles    int64         `json:"totalFiles"`
	IndexedFiles  int64         `json:"indexedFiles"`
	SkippedFiles  int64         `json:"skippedFiles"`
	FailedFiles   int64         `json:"failedFiles"`
	LastIndexTime time.Time     `json:"lastIndexTime"`
	Duration      time.Duration `json:"duration"`
}

// NewIndexer 创建索引器.
func NewIndexer(engine *Engine, config *SearchConfig, extractor ContentExtractor, encoder VectorEncoder) *Indexer {
	if config == nil {
		config = DefaultSearchConfig()
	}

	return &Indexer{
		engine:    engine,
		config:    config,
		extractor: extractor,
		encoder:   encoder,
		workers:   config.IndexWorkers,
		stopCh:    make(chan struct{}),
		stats:     &IndexerStats{},
	}
}

// IndexDirectory 索引目录.
func (idx *Indexer) IndexDirectory(rootPath string) error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return fmt.Errorf("索引器正在运行中")
	}
	idx.running = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.running = false
		idx.mu.Unlock()
	}()

	start := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-idx.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	fileCh := make(chan string, idx.workers*10)
	errCh := make(chan error, idx.workers)

	// 启动工作协程
	for i := 0; i < idx.workers; i++ {
		idx.wg.Add(1)
		go idx.worker(ctx, fileCh, errCh)
	}

	// 遍历目录
	go func() {
		defer close(fileCh)
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			// 检查文件大小
			if info.Size() > idx.config.MaxFileSize {
				idx.stats.SkippedFiles++
				return nil
			}

			// 检查文件类型
			ext := strings.ToLower(filepath.Ext(path))
			if !idx.isSupportedType(ext) {
				idx.stats.SkippedFiles++
				return nil
			}

			idx.stats.TotalFiles++
			fileCh <- path
			return nil
		})

		if err != nil && err != context.Canceled {
			log.Printf("遍历目录失败: %v", err)
		}
	}()

	// 等待所有工作协程完成
	go func() {
		idx.wg.Wait()
		close(errCh)
	}()

	// 收集错误
	var lastErr error
	for err := range errCh {
		if err != nil {
			lastErr = err
			idx.stats.FailedFiles++
		}
	}

	idx.stats.Duration = time.Since(start)
	idx.stats.LastIndexTime = time.Now()

	return lastErr
}

// IndexFile 索引单个文件.
func (idx *Indexer) IndexFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("不能索引目录")
	}

	if info.Size() > idx.config.MaxFileSize {
		return fmt.Errorf("文件大小超过限制")
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if !idx.isSupportedType(ext) {
		return fmt.Errorf("不支持的文件类型: %s", ext)
	}

	doc := idx.buildIndex(filePath, info)
	if err := idx.engine.IndexDocument(doc); err != nil {
		return fmt.Errorf("索引文件失败: %w", err)
	}

	idx.stats.IndexedFiles++
	return nil
}

// IncrementalIndex 增量索引.
func (idx *Indexer) IncrementalIndex(rootPath string) error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return fmt.Errorf("索引器正在运行中")
	}
	idx.running = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.running = false
		idx.mu.Unlock()
	}()

	start := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-idx.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	fileCh := make(chan string, idx.workers*10)
	errCh := make(chan error, idx.workers)

	// 启动工作协程
	for i := 0; i < idx.workers; i++ {
		idx.wg.Add(1)
		go idx.worker(ctx, fileCh, errCh)
	}

	// 遍历目录，检查是否需要更新
	go func() {
		defer close(fileCh)
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if info.IsDir() {
				return nil
			}

			// 检查是否已索引且未过期
			if idx.isUpToDate(path, info) {
				idx.stats.SkippedFiles++
				return nil
			}

			idx.stats.TotalFiles++
			fileCh <- path
			return nil
		})

		if err != nil && err != context.Canceled {
			log.Printf("遍历目录失败: %v", err)
		}
	}()

	// 等待所有工作协程完成
	go func() {
		idx.wg.Wait()
		close(errCh)
	}()

	// 收集错误
	var lastErr error
	for err := range errCh {
		if err != nil {
			lastErr = err
			idx.stats.FailedFiles++
		}
	}

	idx.stats.Duration = time.Since(start)
	idx.stats.LastIndexTime = time.Now()

	return lastErr
}

// Stop 停止索引器.
func (idx *Indexer) Stop() {
	close(idx.stopCh)
	idx.wg.Wait()
}

// GetStats 获取索引统计.
func (idx *Indexer) GetStats() *IndexerStats {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	stats := *idx.stats
	return &stats
}

// worker 工作协程.
func (idx *Indexer) worker(ctx context.Context, fileCh <-chan string, errCh chan<- error) {
	defer idx.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-fileCh:
			if !ok {
				return
			}

			if err := idx.IndexFile(path); err != nil {
				errCh <- fmt.Errorf("索引 %s: %w", path, err)
			} else {
				errCh <- nil
			}
		}
	}
}

// buildIndex 构建索引条目.
func (idx *Indexer) buildIndex(filePath string, info os.FileInfo) *SearchIndex {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := getMimeType(ext)

	doc := &SearchIndex{
		ID:         uuid.New().String(),
		FilePath:   filePath,
		FileName:   info.Name(),
		FileType:   getFileType(ext),
		FileSize:   info.Size(),
		ModifiedAt: info.ModTime(),
		CreatedAt:  info.ModTime(), // 简化处理
		Tags:       make([]string, 0),
		Metadata:   make(map[string]string),
		Status:     IndexStatusPending,
	}

	// 计算内容哈希
	hash, err := hashFile(filePath)
	if err == nil {
		doc.ContentHash = hash
	}

	// 提取内容
	if idx.extractor != nil && idx.extractor.CanExtract(doc.FileType, mimeType) {
		content, err := idx.extractor.Extract(filePath)
		if err == nil {
			doc.Content = content
		}
	}

	// 生成向量
	if idx.config.EnableSemantic && idx.encoder != nil && doc.Content != "" {
		vector, err := idx.encoder.Encode(doc.Content)
		if err == nil {
			doc.Vector = vector
		}
	}

	return doc
}

// isUpToDate 检查文件是否已索引且未过期.
func (idx *Indexer) isUpToDate(filePath string, info os.FileInfo) bool {
	idx.engine.mu.RLock()
	defer idx.engine.mu.RUnlock()

	for _, doc := range idx.engine.indexes {
		if doc.FilePath == filePath {
			// 检查修改时间
			if doc.ModifiedAt.Equal(info.ModTime()) {
				return true
			}
			// 检查内容哈希
			hash, err := hashFile(filePath)
			if err == nil && hash == doc.ContentHash {
				return true
			}
			return false
		}
	}

	return false
}

// isSupportedType 检查是否支持的文件类型.
func (idx *Indexer) isSupportedType(ext string) bool {
	fileType := getFileType(ext)
	for _, t := range idx.config.SupportedTypes {
		if fileType == t {
			return true
		}
	}
	return false
}

// getFileType 根据扩展名获取文件类型.
func getFileType(ext string) FileType {
	switch ext {
	case ".txt", ".md", ".doc", ".docx", ".pdf", ".xls", ".xlsx", ".ppt", ".pptx", ".csv", ".json", ".xml", ".yaml", ".yml":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff":
		return FileTypeImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return FileTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a":
		return FileTypeAudio
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return FileTypeArchive
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".html", ".css", ".sql", ".sh":
		return FileTypeCode
	default:
		return FileTypeOther
	}
}

// getMimeType 根据扩展名获取 MIME 类型.
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".csv":  "text/csv",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".jpg":  "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// hashFile 计算文件哈希.
func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
