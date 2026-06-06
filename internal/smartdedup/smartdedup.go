package smartdedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Engine 智能去重引擎。
// 协调扫描器、哈希器和策略器完成文件去重工作。
type Engine struct {
	config   *Config
	scanner  *Scanner
	hasher   *Hasher
	strategy *Strategy
	stats    *DedupStats
	mu       sync.RWMutex
	scanning bool
}

// NewEngine 创建新的智能去重引擎。
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	algo := config.HashAlgorithm
	if !algo.IsValid() {
		algo = HashSHA256
	}
	return &Engine{
		config:   config,
		scanner:  NewScanner(config),
		hasher:   NewHasherWithAlgorithm(0, algo),
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

	scanResult, err := e.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	e.stats.AddScan(scanResult)

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
				dedupResult.Deleted++
				dedupResult.SavedBytes += fi.Size
				continue
			}

			// 尝试转为硬链接（节省空间且保留文件路径）
			if e.config.ConvertToHardLink && selection.Keep != nil {
				if err := e.convertToHardLink(selection.Keep.Path, fi.Path); err == nil {
					dedupResult.HardLinked++
					dedupResult.SavedBytes += fi.Size
					continue
				}
				// 硬链接失败，回退到删除
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

// convertToHardLink 将文件转换为硬链接。
// 先删除重复文件，再创建指向原始文件的硬链接。
func (e *Engine) convertToHardLink(originalPath, duplicatePath string) error {
	// 确保源文件存在
	if _, err := os.Stat(originalPath); err != nil {
		return fmt.Errorf("original file not found: %w", err)
	}

	// 删除重复文件
	if err := os.Remove(duplicatePath); err != nil {
		return fmt.Errorf("remove duplicate: %w", err)
	}

	// 创建硬链接
	if err := os.Link(originalPath, duplicatePath); err != nil {
		return fmt.Errorf("create hardlink: %w", err)
	}

	return nil
}

// removeFile 删除文件。
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

	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("create trash dir: %w", err)
	}

	baseName := filepath.Base(filePath)
	trashPath := filepath.Join(trashDir, baseName)

	if _, err := os.Stat(trashPath); err == nil {
		ext := filepath.Ext(baseName)
		name := baseName[:len(baseName)-len(ext)]
		trashPath = filepath.Join(trashDir, fmt.Sprintf("%s_%d%s", name, time.Now().UnixNano(), ext))
	}

	if err := os.Rename(filePath, trashPath); err != nil {
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
func (e *Engine) Stats() *DedupStats {
	stats := e.stats.Snapshot()
	return &stats
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
	algo := config.HashAlgorithm
	if !algo.IsValid() {
		algo = HashSHA256
	}
	e.hasher = NewHasherWithAlgorithm(0, algo)
}

// ScanSingle 扫描单个文件。
func (e *Engine) ScanSingle(filePath string) (*FileInfo, error) {
	fi, err := e.hasher.ComputeFileInfoWithLinks(filePath)
	if err != nil {
		return nil, err
	}
	fi.HashAlgorithm = string(e.hasher.Algorithm())
	return fi, nil
}

// GenerateReport 生成去重报告。
func (e *Engine) GenerateReport() *DedupReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := e.stats.Snapshot()
	report := &DedupReport{
		GeneratedAt:    time.Now(),
		TotalScans:     int(stats.TotalScans),
		TotalFiles:     int(stats.TotalFilesScanned),
		TotalSize:      stats.TotalSizeScanned,
		DuplicateCount: int(stats.TotalDuplicates),
		DuplicateSize:  stats.TotalSavedBytes,
		SavedBytes:     stats.TotalSavedBytes,
		TrashedCount:   int(stats.TotalTrashed),
		DeletedCount:   int(stats.TotalDeleted),
		HardLinkedCount: int(stats.TotalHardLinked),
		SpaceReclaimed: stats.TotalSavedBytes,
		RecoveryRatio:  stats.RecoveryRatio,
		GroupsByType:   make(map[string]int),
	}

	// 获取最新扫描结果以填充详细信息
	lastScan, err := e.scanner.Scan()
	if err == nil && lastScan != nil {
		// 按文件类型统计
		for _, dg := range lastScan.DuplicateGroups {
			if len(dg.Files) > 0 {
				ct := dg.Files[0].ContentType.String()
				report.GroupsByType[ct]++
			}
		}

		// 获取 Top N 重复组
		topN := e.config.ReportTopN
		if topN <= 0 {
			topN = 10
		}
		sorted := make([]*DuplicateGroup, len(lastScan.DuplicateGroups))
		copy(sorted, lastScan.DuplicateGroups)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].SavedSize > sorted[j].SavedSize
		})
		if len(sorted) > topN {
			sorted = sorted[:topN]
		}
		report.TopDuplicates = sorted
	}

	return report
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

// EstimateSaving 估算去重可节省的空间（不执行删除）。
func (e *Engine) EstimateSaving() (int64, error) {
	result, err := e.scanner.Scan()
	if err != nil {
		return 0, err
	}
	return e.strategy.EstimateSaving(result.DuplicateGroups), nil
}
