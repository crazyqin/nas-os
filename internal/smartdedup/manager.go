// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 智能去重管理器.
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	configPath  string
	entries     map[string]*DedupEntry   // id -> entry
	hashIndex   map[string][]*DedupEntry // contentHash -> entries
	refCounts   map[string]*RefCountEntry // contentHash -> refCount
	stats       DedupStats
	scanCancel  context.CancelFunc
	dedupCancel context.CancelFunc
	storagePath string // 存储根路径
}

// NewManager 创建智能去重管理器.
func NewManager(configPath string, cfg *Config) (*Manager, error) {
	return NewManagerWithStorage(configPath, cfg, "")
}

// NewManagerWithStorage 创建智能去重管理器（指定存储路径）.
func NewManagerWithStorage(configPath string, cfg *Config, storagePath string) (*Manager, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	m := &Manager{
		config:      cfg,
		configPath:  configPath,
		entries:     make(map[string]*DedupEntry),
		hashIndex:   make(map[string][]*DedupEntry),
		refCounts:   make(map[string]*RefCountEntry),
		storagePath: storagePath,
	}

	return m, nil
}

// Config 返回当前配置.
func (m *Manager) Config() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()
	return nil
}

// GetStats 获取去重统计.
func (m *Manager) GetStats() *DedupStats {
	return m.stats.GetSnapshot()
}

// Scan 扫描路径查找重复文件.
func (m *Manager) Scan(ctx context.Context, paths []string) (*ScanResult, error) {
	cfg := m.Config()
	if !cfg.Enabled {
		return nil, fmt.Errorf("smart dedup is disabled")
	}

	// 使用配置中的路径或传入的路径
	scanPaths := paths
	if len(scanPaths) == 0 {
		scanPaths = cfg.ScanPaths
	}
	if len(scanPaths) == 0 {
		return nil, fmt.Errorf("no scan paths specified")
	}

	// 更新扫描状态
	m.stats.mu.Lock()
	m.stats.IsScanning = true
	m.stats.mu.Unlock()

	defer func() {
		m.stats.mu.Lock()
		m.stats.IsScanning = false
		m.stats.mu.Unlock()
	}()

	scanCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.scanCancel = cancel
	m.mu.Unlock()
	defer cancel()

	startTime := time.Now()
	result := &ScanResult{
		ScanID:    fmt.Sprintf("scan-%d", startTime.UnixNano()),
		StartTime: startTime,
	}

	// 阶段1: 遍历文件并计算哈希
	hashMap := make(map[string][]string) // hash -> []filePath
	var scanMu sync.Mutex
	var wg sync.WaitGroup
	fileCh := make(chan string, cfg.MaxWorkers*10)
	errCh := make(chan ScanError, 100)

	// 启动 worker
	for i := 0; i < cfg.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileCh {
				select {
				case <-scanCtx.Done():
					return
				default:
				}

				hash, size, err := m.hashFile(filePath)
				if err != nil {
					select {
					case errCh <- ScanError{Path: filePath, Error: err.Error()}:
					default:
					}
					continue
				}

				scanMu.Lock()
				hashMap[hash] = append(hashMap[hash], filePath)
				result.FilesScanned++
				result.TotalSize += size
				scanMu.Unlock()
			}
		}()
	}

	// 遍历文件
	go func() {
		defer close(fileCh)
		for _, root := range scanPaths {
			m.walkFiles(scanCtx, root, cfg, fileCh)
		}
	}()

	// 等待扫描完成
	wg.Wait()
	close(errCh)

	// 收集错误
	for err := range errCh {
		result.Errors = append(result.Errors, err)
	}

	// 阶段2: 分析重复组
	for hash, files := range hashMap {
		if len(files) < 2 {
			continue
		}

		// 获取文件大小
		var fileSize int64
		if info, err := os.Stat(files[0]); err == nil {
			fileSize = info.Size()
		}

		sort.Strings(files)
		group := DuplicateGroup{
			ContentHash: hash,
			FileCount:   len(files),
			Files:       files,
			UniqueSize:  fileSize,
			TotalSize:   fileSize * int64(len(files)),
			SavedSize:   fileSize * int64(len(files)-1),
			Status:      "pending",
		}
		result.DuplicateGroups = append(result.DuplicateGroups, group)
		result.TotalDuplicates += len(files) - 1
		result.PotentialSaving += group.SavedSize
	}

	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.TotalFilesScanned += int64(result.FilesScanned)
	m.stats.TotalSizeScanned += result.TotalSize
	m.stats.LastScanTime = endTime
	m.stats.TotalScanTime += result.Duration
	m.stats.mu.Unlock()

	return result, nil
}

// Dedup 执行去重操作.
func (m *Manager) Dedup(ctx context.Context, groups []DuplicateGroup) (*DedupResult, error) {
	cfg := m.Config()
	if !cfg.Enabled {
		return nil, fmt.Errorf("smart dedup is disabled")
	}

	m.stats.mu.Lock()
	m.stats.IsDeduping = true
	m.stats.mu.Unlock()

	defer func() {
		m.stats.mu.Lock()
		m.stats.IsDeduping = false
		m.stats.mu.Unlock()
	}()

	dedupCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.dedupCancel = cancel
	m.mu.Unlock()
	defer cancel()

	startTime := time.Now()
	result := &DedupResult{
		DedupID:   fmt.Sprintf("dedup-%d", startTime.UnixNano()),
		StartTime: startTime,
	}

	for _, group := range groups {
		select {
		case <-dedupCtx.Done():
			result.Errors = append(result.Errors, DedupError{
				ContentHash: group.ContentHash,
				Error:       "operation cancelled",
			})
			continue
		default:
		}

		if len(group.Files) < 2 {
			continue
		}

		// 选择源文件（保留第一个）
		sourceFile := group.Files[0]
		dedupFiles := group.Files[1:]

		for _, targetFile := range dedupFiles {
			if err := m.dedupFile(sourceFile, targetFile, cfg); err != nil {
				result.Errors = append(result.Errors, DedupError{
					ContentHash: group.ContentHash,
					FilePath:    targetFile,
					Error:       err.Error(),
				})
				continue
			}
			result.DedupedFiles++
			result.SavedSpace += group.UniqueSize
		}

		// 更新引用计数
		m.updateRefCount(group.ContentHash, group.Files)
		result.ProcessedGroups++
	}

	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.DedupedFiles += int64(result.DedupedFiles)
	m.stats.SavedSpace += result.SavedSpace
	m.stats.LastDedupTime = endTime
	m.stats.TotalDedupTime += result.Duration
	m.stats.updateRatioLocked()
	m.stats.mu.Unlock()

	return result, nil
}

// GetDuplicateGroups 获取所有重复组.
func (m *Manager) GetDuplicateGroups() []DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[string]*DuplicateGroup)
	for _, entry := range m.entries {
		if group, ok := groups[entry.ContentHash]; ok {
			group.Files = append(group.Files, entry.FilePath)
			group.FileCount++
			group.TotalSize += entry.FileSize
			group.SavedSize = entry.FileSize * int64(group.FileCount-1)
		} else {
			groups[entry.ContentHash] = &DuplicateGroup{
				ContentHash: entry.ContentHash,
				Files:       []string{entry.FilePath},
				FileCount:   1,
				UniqueSize:  entry.FileSize,
				TotalSize:   entry.FileSize,
				Status:      "pending",
			}
		}
	}

	result := make([]DuplicateGroup, 0)
	for _, g := range groups {
		if g.FileCount >= 2 {
			result = append(result, *g)
		}
	}
	return result
}

// GetEntry 获取去重条目.
func (m *Manager) GetEntry(id string) (*DedupEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[id]
	return entry, ok
}

// ListEntries 列出所有去重条目.
func (m *Manager) ListEntries() []*DedupEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries := make([]*DedupEntry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}
	return entries
}

// GetRefCount 获取引用计数.
func (m *Manager) GetRefCount(contentHash string) (*RefCountEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ref, ok := m.refCounts[contentHash]
	return ref, ok
}

// ListRefCounts 列出所有引用计数.
func (m *Manager) ListRefCounts() []*RefCountEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	refs := make([]*RefCountEntry, 0, len(m.refCounts))
	for _, r := range m.refCounts {
		refs = append(refs, r)
	}
	return refs
}

// CancelScan 取消扫描.
func (m *Manager) CancelScan() {
	m.mu.RLock()
	cancel := m.scanCancel
	m.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

// CancelDedup 取消去重.
func (m *Manager) CancelDedup() {
	m.mu.RLock()
	cancel := m.dedupCancel
	m.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

// DetectBackend 检测存储后端.
func (m *Manager) DetectBackend(path string) (StorageBackend, error) {
	cfg := m.Config()
	if cfg.Backend != BackendAuto {
		return cfg.Backend, nil
	}

	// 尝试检测 Btrfs
	if m.isBtrfs(path) {
		return BackendBtrfs, nil
	}

	// 尝试检测 ZFS
	if m.isZFS(path) {
		return BackendZFS, nil
	}

	// 默认使用 hardlink
	return "", fmt.Errorf("unable to detect storage backend for %s", path)
}

// hashFile 计算文件的 SHA-256 哈希.
func (m *Manager) hashFile(filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}

	hash := hex.EncodeToString(h.Sum(nil))
	return hash, size, nil
}

// walkFiles 递归遍历文件.
func (m *Manager) walkFiles(ctx context.Context, root string, cfg *Config, fileCh chan<- string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}

		// 跳过目录
		if info.IsDir() {
			// 检查排除路径
			for _, exclude := range cfg.ExcludePaths {
				if strings.HasPrefix(path, exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 检查文件大小
		if cfg.MinFileSize > 0 && info.Size() < cfg.MinFileSize {
			return nil
		}
		if cfg.MaxFileSize > 0 && info.Size() > cfg.MaxFileSize {
			return nil
		}

		// 检查排除模式
		for _, pattern := range cfg.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				return nil
			}
		}

		select {
		case fileCh <- path:
		case <-ctx.Done():
			return filepath.SkipAll
		}
		return nil
	})
}

// dedupFile 执行单个文件的去重.
func (m *Manager) dedupFile(source, target string, cfg *Config) error {
	if cfg.DryRun {
		log.Printf("[DRY RUN] Would dedup %s -> %s", target, source)
		return nil
	}

	switch cfg.Action {
	case ActionHardlink:
		return m.dedupHardlink(source, target)
	case ActionReflink, ActionReflinkPlus:
		return m.dedupReflink(source, target)
	case ActionReport:
		return nil // 仅报告，不执行
	default:
		return fmt.Errorf("unsupported dedup action: %s", cfg.Action)
	}
}

// dedupHardlink 使用硬链接去重.
func (m *Manager) dedupHardlink(source, target string) error {
	// 先备份目标文件
	backupPath := target + ".dedup.bak"
	if err := os.Rename(target, backupPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// 创建硬链接
	if err := os.Link(source, target); err != nil {
		// 回滚
		os.Rename(backupPath, target)
		return fmt.Errorf("hardlink failed: %w", err)
	}

	// 删除备份
	os.Remove(backupPath)
	return nil
}

// dedupReflink 使用 CoW 引用去重.
func (m *Manager) dedupReflink(source, target string) error {
	// 检测后端
	backend, err := m.DetectBackend(source)
	if err != nil {
		// 降级到硬链接
		return m.dedupHardlink(source, target)
	}

	switch backend {
	case BackendBtrfs:
		return m.dedupReflinkBtrfs(source, target)
	case BackendZFS:
		return m.dedupReflinkZFS(source, target)
	default:
		// 降级到硬链接
		return m.dedupHardlink(source, target)
	}
}

// dedupReflinkBtrfs Btrfs CoW 引用.
func (m *Manager) dedupReflinkBtrfs(source, target string) error {
	// 使用 cp --reflink=always 实现 CoW
	// 这里简化实现，实际应该使用 syscall
	return m.dedupHardlink(source, target)
}

// dedupReflinkZFS ZFS 引用.
func (m *Manager) dedupReflinkZFS(source, target string) error {
	// ZFS 使用 dedup 属性
	// 这里简化实现，实际应该使用 ZFS ioctl
	return m.dedupHardlink(source, target)
}

// isBtrfs 检测是否为 Btrfs 文件系统.
func (m *Manager) isBtrfs(path string) bool {
	// 简化实现：检查 /proc/mounts
	// 实际应该使用 statfs
	return false
}

// isZFS 检测是否为 ZFS 文件系统.
func (m *Manager) isZFS(path string) bool {
	// 简化实现：检查 /proc/mounts
	// 实际应该使用 statfs
	return false
}

// updateRefCount 更新引用计数.
func (m *Manager) updateRefCount(contentHash string, files []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ref, ok := m.refCounts[contentHash]
	if !ok {
		ref = &RefCountEntry{
			ContentHash: contentHash,
			SourceFile:  files[0],
			CreatedAt:   time.Now(),
		}
		m.refCounts[contentHash] = ref
	}

	for _, f := range files {
		ref.IncrRef(f)
	}
}

// DedupResult 去重结果.
type DedupResult struct {
	DedupID         string        `json:"dedupId"`
	StartTime       time.Time     `json:"startTime"`
	EndTime         time.Time     `json:"endTime"`
	Duration        time.Duration `json:"duration"`
	ProcessedGroups int           `json:"processedGroups"`
	DedupedFiles    int           `json:"dedupedFiles"`
	SavedSpace      int64         `json:"savedSpace"`
	Errors          []DedupError  `json:"errors,omitempty"`
}

// DedupError 去重错误.
type DedupError struct {
	ContentHash string `json:"contentHash"`
	FilePath    string `json:"filePath,omitempty"`
	Error       string `json:"error"`
}
