package smartdedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Engine 智能去重引擎。
// 协调扫描器、哈希器和策略器完成文件去重工作。
type Engine struct {
	config    *Config
	scanner   *Scanner
	hasher    *Hasher
	strategy  *Strategy
	stats     *DedupStats
	mu        sync.RWMutex
	scanning  bool
}

// NewEngine 创建新的智能去重引擎。
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	return &Engine{
		config:   config,
		scanner:  NewScanner(config),
		hasher:   NewHasher(0),
		strategy: NewStrategy(config.RetentionPolicy),
		stats:    &DedupStats{},
	}
}

// Scan 执行扫描，返回扫描结果。
func (e *Engine) Scan() (*ScanResult, error) {
	e.mu.Lock()
	if e.scanning {
		e.mu.Unlock()
		return nil, fmt.Errorf("scan already in progress")
	}
	e.scanning = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.scanning = false
		e.mu.Unlock()
	}()

	if !e.config.Enabled {
		return nil, fmt.Errorf("smartdedup is disabled")
	}

	result, err := e.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	e.stats.AddScan(result)
	return result, nil
}

// Dedup 执行去重。
// 先扫描，再按策略选择保留文件，最后删除重复文件。
func (e *Engine) Dedup() (*DedupResult, error) {
	e.mu.Lock()
	if e.scanning {
		e.mu.Unlock()
		return nil, fmt.Errorf("scan/dedup already in progress")
	}
	e.scanning = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.scanning = false
		e.mu.Unlock()
	}()

	if !e.config.Enabled {
		return nil, fmt.Errorf("smartdedup is disabled")
	}

	startTime := time.Now()

	// 扫描
	scanResult, err := e.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	e.stats.AddScan(scanResult)

	// 选择并删除
	dedupResult := &DedupResult{
		StartTime: startTime,
	}

	for _, group := range scanResult.DuplicateGroups {
		selection := e.strategy.Select(group)
		if selection == nil {
			continue
		}

		for _, fi := range selection.Remove {
			dedupResult.Processed++
			if e.config.DryRun {
				// DryRun 模式只计数不删除
				dedupResult.Deleted++
				dedupResult.SavedBytes += fi.Size
				continue
			}

			if err := e.removeFile(fi); err != nil {
				dedupResult.Failed++
				dedupResult.Errors = append(dedupResult.Errors, DedupError{
					Path:  fi.Path,
					Error: err.Error(),
				})
				continue
			}

			if e.config.SafeDelete {
				dedupResult.Trashed++
			} else {
				dedupResult.Deleted++
			}
			dedupResult.SavedBytes += fi.Size
		}
	}

	dedupResult.EndTime = time.Now()
	dedupResult.Duration = dedupResult.EndTime.Sub(dedupResult.StartTime)

	e.stats.AddDedup(dedupResult)
	return dedupResult, nil
}

// removeFile 删除文件。
// 如果启用了安全删除，先移到回收站。
func (e *Engine) removeFile(fi *FileInfo) error {
	if e.config.SafeDelete {
		return e.moveToTrash(fi.Path)
	}
	return os.Remove(fi.Path)
}

// moveToTrash 将文件移到回收站。
func (e *Engine) moveToTrash(filePath string) error {
	trashDir := e.config.TrashPath
	if trashDir == "" {
		trashDir = "/tmp/smartdedup-trash"
	}

	// 确保回收站目录存在
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("create trash dir: %w", err)
	}

	// 生成回收站中的文件名（避免冲突）
	baseName := filepath.Base(filePath)
	trashPath := filepath.Join(trashDir, baseName)

	// 如果已存在同名文件，添加时间戳
	if _, err := os.Stat(trashPath); err == nil {
		ext := filepath.Ext(baseName)
		name := baseName[:len(baseName)-len(ext)]
		trashPath = filepath.Join(trashDir, fmt.Sprintf("%s_%d%s", name, time.Now().UnixNano(), ext))
	}

	// 尝试 rename（同文件系统），否则 copy + delete
	if err := os.Rename(filePath, trashPath); err != nil {
		// 跨文件系统，使用 copy + delete
		return e.copyAndDelete(filePath, trashPath)
	}
	return nil
}

// copyAndDelete 复制文件到目标位置后删除源文件。
func (e *Engine) copyAndDelete(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	buf := make([]byte, 32*1024)
	if _, err := copyBuffer(dstFile, srcFile, buf); err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}

	if err := dstFile.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("close destination: %w", err)
	}

	return os.Remove(src)
}

// copyBuffer 使用缓冲区复制数据。
func copyBuffer(dst *os.File, src *os.File, buf []byte) (int64, error) {
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
		}
		if er != nil {
			if er.Error() == "EOF" {
				break
			}
			return written, er
		}
	}
	return written, nil
}

// IsScanning 是否正在扫描。
func (e *Engine) IsScanning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.scanning
}

// Stats 返回统计信息。
func (e *Engine) Stats() DedupStats {
	return e.stats.Snapshot()
}

// Config 返回当前配置。
func (e *Engine) Config() *Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// UpdateConfig 更新配置。
func (e *Engine) UpdateConfig(config *Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
	e.scanner = NewScanner(config)
	e.strategy = NewStrategy(config.RetentionPolicy)
}

// ScanSingle 扫描单个文件。
func (e *Engine) ScanSingle(filePath string) (*FileInfo, error) {
	return e.hasher.ComputeFileInfo(filePath)
}

// CleanTrash 清理回收站中超过指定天数的文件。
func (e *Engine) CleanTrash(olderThanDays int) (int, error) {
	trashDir := e.config.TrashPath
	if trashDir == "" {
		trashDir = "/tmp/smartdedup-trash"
	}

	if _, err := os.Stat(trashDir); os.IsNotExist(err) {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	removed := 0

	err := filepath.Walk(trashDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return nil
	})

	return removed, err
}
