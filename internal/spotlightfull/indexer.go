// Package spotlightfull - 文件索引器
package spotlightfull

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 文本内容提取最大字节数（避免索引大文件时内存溢出）.
const maxContentExtractSize = 1 << 20 // 1MB

// 可索引文本文件扩展名.
var indexableTextExts = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".rst": true,
	".csv": true, ".tsv": true, ".json": true, ".xml": true,
	".yaml": true, ".yml": true, ".toml": true, ".ini": true,
	".conf": true, ".cfg": true, ".log": true,
	".go": true, ".py": true, ".js": true, ".ts": true,
	".java": true, ".c": true, ".cpp": true, ".h": true,
	".rs": true, ".rb": true, ".php": true, ".sh": true,
	".html": true, ".htm": true, ".css": true, ".scss": true,
	".sql": true, ".r": true, ".lua": true, ".pl": true,
	".swift": true, ".kt": true, ".scala": true,
	".srt": true, ".sub": true, ".vtt": true,
	".tex": true, ".bib": true,
}

// NewFileIndexer 创建文件索引器.
func NewFileIndexer(engine *SearchEngine, config *IndexerConfig) *FileIndexer {
	if config == nil {
		config = DefaultIndexerConfig()
	}

	return &FileIndexer{
		engine: engine,
		config: config,
		stopCh: make(chan struct{}),
	}
}

// RunFullScan 执行全量扫描索引.
func (fi *FileIndexer) RunFullScan(ctx context.Context) error {
	fi.mu.Lock()
	if fi.isRunning {
		fi.mu.Unlock()
		return fmt.Errorf("索引器正在运行中")
	}
	fi.isRunning = true
	fi.mu.Unlock()

	defer func() {
		fi.mu.Lock()
		fi.isRunning = false
		fi.mu.Unlock()
	}()

	// 重建索引
	if err := fi.engine.RebuildIndex(); err != nil {
		return fmt.Errorf("重建索引失败: %w", err)
	}

	// 遍历所有扫描路径
	for _, scanPath := range fi.config.ScanPaths {
		if err := fi.scanDirectory(ctx, scanPath); err != nil {
			if ctx.Err() != nil {
				return ctx.Err() // 上下文取消
			}
			fmt.Printf("[spotlightfull] 扫描目录 %s 失败: %v\n", scanPath, err)
			continue
		}
	}

	// 持久化索引
	if err := fi.engine.SaveIndex(); err != nil {
		fmt.Printf("[spotlightfull] 保存索引失败: %v\n", err)
	}

	fi.mu.Lock()
	fi.lastIndexed = time.Now()
	fi.mu.Unlock()

	return nil
}

// RunIncrementalScan 执行增量扫描（仅处理新增和修改的文件）.
func (fi *FileIndexer) RunIncrementalScan(ctx context.Context) error {
	fi.mu.Lock()
	if fi.isRunning {
		fi.mu.Unlock()
		return fmt.Errorf("索引器正在运行中")
	}
	fi.isRunning = true
	fi.mu.Unlock()

	defer func() {
		fi.mu.Lock()
		fi.isRunning = false
		fi.mu.Unlock()
	}()

	for _, scanPath := range fi.config.ScanPaths {
		if err := fi.scanDirectoryIncremental(ctx, scanPath); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Printf("[spotlightfull] 增量扫描目录 %s 失败: %v\n", scanPath, err)
			continue
		}
	}

	// 持久化索引
	if err := fi.engine.SaveIndex(); err != nil {
		fmt.Printf("[spotlightfull] 保存索引失败: %v\n", err)
	}

	fi.mu.Lock()
	fi.lastIndexed = time.Now()
	fi.mu.Unlock()

	return nil
}

// Stop 停止索引器.
func (fi *FileIndexer) Stop() {
	close(fi.stopCh)
}

// IsRunning 检查索引器是否正在运行.
func (fi *FileIndexer) IsRunning() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.isRunning
}

// GetLastIndexedTime 获取最后索引时间.
func (fi *FileIndexer) GetLastIndexedTime() time.Time {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.lastIndexed
}

// scanDirectory 全量扫描目录.
func (fi *FileIndexer) scanDirectory(ctx context.Context, root string) error {
	// 收集所有文件
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 检查上下文
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 跳过目录和排除的路径
		if info.IsDir() {
			if fi.isExcluded(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if fi.isExcluded(path) {
			return nil
		}

		// 检查文件大小
		if info.Size() > fi.config.MaxFileSize {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return err
	}

	// 并发索引文件
	return fi.indexFilesConcurrently(ctx, files)
}

// scanDirectoryIncremental 增量扫描目录.
func (fi *FileIndexer) scanDirectoryIncremental(ctx context.Context, root string) error {
	fi.mu.RLock()
	lastIndexed := fi.lastIndexed
	fi.mu.RUnlock()

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			if fi.isExcluded(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.isExcluded(path) {
			return nil
		}
		if info.Size() > fi.config.MaxFileSize {
			return nil
		}
		// 仅索引修改时间晚于上次索引时间的文件
		if info.ModTime().After(lastIndexed) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	if len(files) > 0 {
		return fi.indexFilesConcurrently(ctx, files)
	}
	return nil
}

// indexFilesConcurrently 并发索引文件.
func (fi *FileIndexer) indexFilesConcurrently(ctx context.Context, files []string) error {
	workerCount := fi.config.WorkerCount
	if workerCount < 1 {
		workerCount = 1
	}

	fileCh := make(chan string, workerCount*2)
	errCh := make(chan error, workerCount)
	var wg sync.WaitGroup

	// 启动工作线程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileCh {
				if ctx.Err() != nil {
					return
				}
				if err := fi.indexFile(path); err != nil {
					errCh <- fmt.Errorf("索引文件 %s 失败: %w", path, err)
				}
			}
		}()
	}

	// 分发文件
	go func() {
		defer close(fileCh)
		for _, path := range files {
			select {
			case fileCh <- path:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待完成
	wg.Wait()
	close(errCh)

	// 检查错误
	for err := range errCh {
		fmt.Printf("[spotlightfull] %v\n", err)
	}

	fi.mu.Lock()
	fi.indexedCount += int64(len(files))
	fi.mu.Unlock()

	return nil
}

// indexFile 索引单个文件.
func (fi *FileIndexer) indexFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	entry := &IndexEntry{
		Path:       path,
		Name:       info.Name(),
		Extension:  strings.ToLower(filepath.Ext(path)),
		Size:       info.Size(),
		FileType:   classifyFileType(path),
		MimeType:   detectMimeType(path),
		Tags:       []string{},
		Metadata:   make(map[string]string),
		ModifiedAt: info.ModTime(),
	}

	// 提取文本内容（仅限文本文件）
	if isTextFile(entry.Extension) && info.Size() > 0 {
		content, err := extractTextContent(path)
		if err == nil {
			entry.Content = content
		}
	}

	// 添加元数据
	entry.Metadata["ext"] = entry.Extension
	entry.Metadata["size_group"] = sizeGroup(entry.Size)
	entry.Metadata["dir"] = filepath.Dir(path)

	return fi.engine.AddDocument(entry)
}

// isExcluded 检查路径是否被排除.
func (fi *FileIndexer) isExcluded(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range fi.config.ExcludePatterns {
		if strings.Contains(base, pattern) || strings.HasPrefix(base, ".") {
			return true
		}
	}
	return false
}

// classifyFileType 根据扩展名分类文件类型.
func classifyFileType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".md", ".rtf", ".odt", ".xls", ".xlsx",
		".ppt", ".pptx", ".csv", ".json", ".xml", ".yaml", ".yml", ".epub":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".heic", ".heif", ".raw", ".cr2", ".nef":
		return FileTypeImage
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".vob":
		return FileTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a", ".opus", ".aiff":
		return FileTypeAudio
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz", ".zst":
		return FileTypeArchive
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb",
		".php", ".sh", ".html", ".css", ".scss", ".sql", ".swift", ".kt", ".lua", ".pl":
		return FileTypeCode
	default:
		return FileTypeOther
	}
}

// detectMimeType 检测文件MIME类型.
func detectMimeType(path string) string {
	ext := filepath.Ext(path)
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

// isTextFile 检查是否为可索引的文本文件.
func isTextFile(ext string) bool {
	return indexableTextExts[ext]
}

// extractTextContent 提取文件文本内容.
func extractTextContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// 限制读取大小
	reader := io.LimitReader(f, maxContentExtractSize)
	scanner := bufio.NewScanner(reader)

	var content strings.Builder
	lineCount := 0
	maxLines := 10000 // 最多读取10000行

	for scanner.Scan() {
		line := scanner.Text()
		// 跳过二进制内容检测
		if !utf8.ValidString(line) && lineCount > 10 {
			return content.String(), nil
		}
		content.WriteString(line)
		content.WriteString(" ")
		lineCount++
		if lineCount >= maxLines {
			break
		}
	}

	return content.String(), scanner.Err()
}

// sizeGroup 文件大小分组（用于元数据索引）.
func sizeGroup(size int64) string {
	switch {
	case size == 0:
		return "empty"
	case size < 1024:
		return "tiny"
	case size < 1024*1024:
		return "small"
	case size < 100*1024*1024:
		return "medium"
	case size < 1024*1024*1024:
		return "large"
	default:
		return "huge"
	}
}
