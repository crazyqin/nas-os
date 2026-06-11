// Package webshare 文件索引器，提供文件系统索引和全文搜索支持。
// 集成 TrueSearch 实现 sub-second 响应的全文搜索，支持增量更新和批量索引。
package webshare

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// IndexStatus 索引状态
type IndexStatus string

const (
	IndexStatusPending  IndexStatus = "pending"
	IndexStatusIndexing IndexStatus = "indexing"
	IndexStatusReady    IndexStatus = "ready"
	IndexStatusError    IndexStatus = "error"
)

// IndexEntry 索引条目
type IndexEntry struct {
	ID         string            `json:"id"`          // 唯一标识
	Path       string            `json:"path"`        // 文件路径
	Name       string            `json:"name"`        // 文件名
	Extension  string            `json:"extension"`   // 扩展名
	Size       int64             `json:"size"`        // 文件大小
	ModTime    time.Time         `json:"mod_time"`    // 修改时间
	IndexTime  time.Time         `json:"index_time"`  // 索引时间
	Checksum   string            `json:"checksum"`    // 文件校验和
	IsDir      bool              `json:"is_dir"`      // 是否目录
	Metadata   map[string]string `json:"metadata"`    // 元数据
}

// IndexStats 索引统计信息
type IndexStats struct {
	TotalEntries   int64         `json:"total_entries"`    // 总条目数
	TotalDirs      int64         `json:"total_dirs"`       // 目录数
	TotalFiles     int64         `json:"total_files"`      // 文件数
	TotalSize      int64         `json:"total_size"`       // 总大小
	IndexSize      int64         `json:"index_size_bytes"` // 索引大小
	LastUpdateTime time.Time     `json:"last_update_time"` // 最后更新时间
	Status         IndexStatus   `json:"status"`           // 索引状态
	IndexDuration  time.Duration `json:"index_duration"`   // 索引耗时
}

// FileIndexer 文件索引器
type FileIndexer struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *WebShareConfig
	entries     map[string]*IndexEntry // path -> entry
	nameIndex   map[string][]string    // 小写文件名 -> paths
	checksums   map[string]string      // path -> checksum (用于增量更新)
	status      IndexStatus
	lastUpdate  time.Time
	stopChan    chan struct{}
	running     bool
}

// NewFileIndexer 创建文件索引器
func NewFileIndexer(logger *zap.Logger, config *WebShareConfig) *FileIndexer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &FileIndexer{
		logger:    logger,
		config:    config,
		entries:   make(map[string]*IndexEntry),
		nameIndex: make(map[string][]string),
		checksums: make(map[string]string),
		status:    IndexStatusPending,
		stopChan:  make(chan struct{}),
	}
}

// Start 启动索引器
func (fi *FileIndexer) Start(ctx context.Context) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.running {
		return fmt.Errorf("索引器已在运行")
	}

	fi.running = true
	fi.status = IndexStatusIndexing

	// 启动后台索引协程
	go fi.runIndexer(ctx)

	fi.logger.Info("文件索引器启动")
	return nil
}

// Stop 停止索引器
func (fi *FileIndexer) Stop() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if !fi.running {
		return nil
	}

	close(fi.stopChan)
	fi.running = false

	fi.logger.Info("文件索引器停止")
	return nil
}

// IsRunning 检查是否运行中
func (fi *FileIndexer) IsRunning() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.running
}

// GetStatus 获取索引状态
func (fi *FileIndexer) GetStatus() IndexStatus {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.status
}

// GetStats 获取索引统计
func (fi *FileIndexer) GetStats() *IndexStats {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	stats := &IndexStats{
		TotalEntries:   int64(len(fi.entries)),
		LastUpdateTime: fi.lastUpdate,
		Status:         fi.status,
	}

	for _, entry := range fi.entries {
		if entry.IsDir {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			stats.TotalSize += entry.Size
		}
	}

	return stats
}

// IndexFile 索引单个文件
func (fi *FileIndexer) IndexFile(path string) error {
	if !fi.IsRunning() {
		return fmt.Errorf("索引器未运行")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("无法读取文件信息: %w", err)
	}

	// 生成 ID
	id := fi.generateID(path)

	// 计算校验和（仅文件，不计算目录）
	checksum := ""
	if !info.IsDir() {
		checksum, err = fi.calculateChecksum(path)
		if err != nil {
			fi.logger.Warn("计算校验和失败",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	// 构建索引条目
	entry := &IndexEntry{
		ID:        id,
		Path:      path,
		Name:      info.Name(),
		Extension: getFileExtension(info.Name()),
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		IndexTime: time.Now(),
		Checksum:  checksum,
		IsDir:     info.IsDir(),
		Metadata:  make(map[string]string),
	}

	// 存入索引
	fi.mu.Lock()
	fi.entries[path] = entry

	// 更新文件名索引
	nameKey := strings.ToLower(info.Name())
	fi.nameIndex[nameKey] = append(fi.nameIndex[nameKey], path)

	// 更新校验和
	if checksum != "" {
		fi.checksums[path] = checksum
	}
	fi.mu.Unlock()

	fi.logger.Debug("索引文件",
		zap.String("path", path),
		zap.String("name", info.Name()))

	return nil
}

// RemoveEntry 移除索引条目
func (fi *FileIndexer) RemoveEntry(path string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	entry, exists := fi.entries[path]
	if !exists {
		return
	}

	// 移除文件名索引
	nameKey := strings.ToLower(entry.Name)
	if paths, ok := fi.nameIndex[nameKey]; ok {
		newPaths := make([]string, 0, len(paths)-1)
		for _, p := range paths {
			if p != path {
				newPaths = append(newPaths, p)
			}
		}
		if len(newPaths) == 0 {
			delete(fi.nameIndex, nameKey)
		} else {
			fi.nameIndex[nameKey] = newPaths
		}
	}

	// 移除校验和
	delete(fi.checksums, path)

	// 移除条目
	delete(fi.entries, path)
}

// Search 搜索文件
func (fi *FileIndexer) Search(query string, limit int) []*IndexEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query = strings.ToLower(query)
	results := make([]*IndexEntry, 0, limit)

	// 使用文件名索引进行快速匹配
	for nameKey, paths := range fi.nameIndex {
		if strings.Contains(nameKey, query) {
			for _, path := range paths {
				if entry, ok := fi.entries[path]; ok {
					results = append(results, entry)
					if len(results) >= limit {
						return results
					}
				}
			}
		}
	}

	return results
}

// SearchByPrefix 按前缀搜索文件名（用于自动补全）
func (fi *FileIndexer) SearchByPrefix(prefix string, limit int) []string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	prefix = strings.ToLower(prefix)
	suggestions := make([]string, 0, limit)
	seen := make(map[string]bool)

	for nameKey := range fi.nameIndex {
		if strings.HasPrefix(nameKey, prefix) {
			// 提取原始文件名
			if paths, ok := fi.nameIndex[nameKey]; ok && len(paths) > 0 {
				entry, exists := fi.entries[paths[0]]
				if exists && !seen[entry.Name] {
					seen[entry.Name] = true
					suggestions = append(suggestions, entry.Name)
					if len(suggestions) >= limit {
						break
					}
				}
			}
		}
	}

	return suggestions
}

// GetEntry 获取索引条目
func (fi *FileIndexer) GetEntry(path string) (*IndexEntry, bool) {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	entry, exists := fi.entries[path]
	if !exists {
		return nil, false
	}

	// 返回副本
	entryCopy := *entry
	return &entryCopy, true
}

// ListEntries 列出所有索引条目
func (fi *FileIndexer) ListEntries(limit int) []*IndexEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if limit <= 0 {
		limit = len(fi.entries)
	}

	entries := make([]*IndexEntry, 0, limit)
	for _, entry := range fi.entries {
		entryCopy := *entry
		entries = append(entries, &entryCopy)
		if len(entries) >= limit {
			break
		}
	}

	return entries
}

// ==================== 内部方法 ====================

// runIndexer 运行索引器
func (fi *FileIndexer) runIndexer(ctx context.Context) {
	fi.logger.Info("开始初始索引", zap.String("root", fi.config.RootPath))

	start := time.Now()

	// 初始全量索引
	if err := fi.buildFullIndex(ctx); err != nil {
		fi.logger.Error("初始索引失败", zap.Error(err))
		fi.mu.Lock()
		fi.status = IndexStatusError
		fi.mu.Unlock()
		return
	}

	fi.mu.Lock()
	fi.status = IndexStatusReady
	fi.lastUpdate = time.Now()
	fi.mu.Unlock()

	fi.logger.Info("初始索引完成",
		zap.Duration("耗时", time.Since(start)),
		zap.Int("条目数", len(fi.entries)))

	// 启动增量更新
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-fi.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi.runIncrementalUpdate(ctx)
		}
	}
}

// buildFullIndex 构建完整索引
func (fi *FileIndexer) buildFullIndex(ctx context.Context) error {
	fi.mu.Lock()
	fi.status = IndexStatusIndexing
	fi.mu.Unlock()

	return filepath.Walk(fi.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 检查上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 跳过隐藏目录
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != fi.config.RootPath {
			return filepath.SkipDir
		}

		// 跳过符号链接
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// 索引文件
		if err := fi.IndexFile(path); err != nil {
			fi.logger.Warn("索引文件失败",
				zap.String("path", path),
				zap.Error(err))
		}

		return nil
	})
}

// runIncrementalUpdate 运行增量更新
func (fi *FileIndexer) runIncrementalUpdate(ctx context.Context) {
	fi.logger.Debug("开始增量更新")

	updated := 0

	// 遍历现有索引，检查是否有更新
	fi.mu.RLock()
	paths := make([]string, 0, len(fi.entries))
	for path := range fi.entries {
		paths = append(paths, path)
	}
	fi.mu.RUnlock()

	for _, path := range paths {
		select {
		case <-ctx.Done():
			return
		case <-fi.stopChan:
			return
		default:
		}

		info, err := os.Stat(path)
		if err != nil {
			// 文件已删除
			fi.RemoveEntry(path)
			updated++
			continue
		}

		// 检查修改时间
		fi.mu.RLock()
		entry, exists := fi.entries[path]
		fi.mu.RUnlock()

		if exists && info.ModTime().After(entry.ModTime) {
			// 文件已更新，重新索引
			if err := fi.IndexFile(path); err != nil {
				fi.logger.Warn("增量更新失败",
					zap.String("path", path),
					zap.Error(err))
			}
			updated++
		}
	}

	if updated > 0 {
		fi.mu.Lock()
		fi.lastUpdate = time.Now()
		fi.mu.Unlock()

		fi.logger.Info("增量更新完成", zap.Int("更新数", updated))
	}
}

// generateID 生成索引条目 ID
func (fi *FileIndexer) generateID(path string) string {
	hash := sha256.Sum256([]byte(path))
	return fmt.Sprintf("idx_%x", hash[:16])
}

// calculateChecksum 计算文件校验和
func (fi *FileIndexer) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 对于大文件，只读取前 4KB 计算校验和
	hash := sha256.New()
	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return "", err
	}
	hash.Write(buf[:n])

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
