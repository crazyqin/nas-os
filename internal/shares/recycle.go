// Package shares 提供 SMB 共享回收站功能，参考群晖 DSM 7.3 的回收站机制。
// 当用户通过 SMB 删除文件时，文件会被移动到 .recycle/ 目录而不是直接删除，
// 支持文件恢复、自动清理过期文件、容量限制等功能。
package shares

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// recycleDirName 回收站目录名。
const recycleDirName = ".recycle"

// recycleMetaDir 回收站元数据目录名。
const recycleMetaDir = ".meta"

// RecycleBinConfig 回收站配置。
type RecycleBinConfig struct {
	// Enabled 是否启用回收站。
	Enabled bool `json:"enabled"`
	// RetentionDays 文件保留天数，0 表示永久保留。
	RetentionDays int `json:"retention_days"`
	// MaxSizeGB 回收站最大容量（GB），0 表示不限制。
	MaxSizeGB int64 `json:"max_size_gb"`
}

// RecycleEntry 回收站条目。
type RecycleEntry struct {
	// ID 条目唯一标识。
	ID string `json:"id"`
	// OriginalPath 文件原始路径（相对于共享根目录）。
	OriginalPath string `json:"original_path"`
	// RecyclePath 文件在回收站中的实际路径。
	RecyclePath string `json:"recycle_path"`
	// Name 文件名。
	Name string `json:"name"`
	// IsDir 是否为目录。
	IsDir bool `json:"is_dir"`
	// Size 文件大小（字节），目录为 0。
	Size int64 `json:"size"`
	// DeletedAt 删除时间。
	DeletedAt time.Time `json:"deleted_at"`
	// DeletedBy 删除者用户名。
	DeletedBy string `json:"deleted_by"`
}

// RecycleStats 回收站统计信息。
type RecycleStats struct {
	// TotalItems 总条目数。
	TotalItems int `json:"total_items"`
	// TotalSize 总大小（字节）。
	TotalSize int64 `json:"total_size"`
	// FolderCount 目录数量。
	FolderCount int `json:"folder_count"`
	// FileCount 文件数量。
	FileCount int `json:"file_count"`
}

// RecycleBin SMB 共享回收站。
type RecycleBin struct {
	shareName string
	sharePath string
	config    RecycleBinConfig
	recyclePath string // 回收站根目录
	metaPath    string // 元数据目录
	mu          sync.RWMutex
}

// NewRecycleBin 创建回收站实例。
// shareName 为共享名称，sharePath 为共享根目录路径。
func NewRecycleBin(shareName, sharePath string, config RecycleBinConfig) *RecycleBin {
	return &RecycleBin{
		shareName:   shareName,
		sharePath:   sharePath,
		config:      config,
		recyclePath: filepath.Join(sharePath, recycleDirName),
		metaPath:    filepath.Join(sharePath, recycleDirName, recycleMetaDir),
	}
}

// ensureDirs 确保回收站目录存在。
func (rb *RecycleBin) ensureDirs() error {
	if err := os.MkdirAll(rb.metaPath, 0o755); err != nil {
		return fmt.Errorf("创建回收站目录失败: %w", err)
	}
	return nil
}

// getEntryPath 获取条目在回收站中的存储路径。
func (rb *RecycleBin) getEntryPath(id string) string {
	return filepath.Join(rb.recyclePath, id)
}

// getMetaPath 获取条目元数据文件路径。
func (rb *RecycleBin) getMetaPath(id string) string {
	return filepath.Join(rb.metaPath, id+".json")
}

// loadEntry 从元数据文件加载条目。
func (rb *RecycleBin) loadEntry(id string) (*RecycleEntry, error) {
	metaPath := rb.getMetaPath(id)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("读取元数据失败: %w", err)
	}

	var entry RecycleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("解析元数据失败: %w", err)
	}

	return &entry, nil
}

// saveEntry 保存条目元数据到文件。
func (rb *RecycleBin) saveEntry(entry *RecycleEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	metaPath := rb.getMetaPath(entry.ID)
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return fmt.Errorf("写入元数据失败: %w", err)
	}

	return nil
}

// deleteEntryMeta 删除条目元数据文件。
func (rb *RecycleBin) deleteEntryMeta(id string) error {
	metaPath := rb.getMetaPath(id)
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除元数据失败: %w", err)
	}
	return nil
}

// MoveToRecycle 将文件移动到回收站。
// originalPath 是相对于共享根目录的文件路径，deletedBy 是删除者用户名。
// 返回回收站条目信息。
func (rb *RecycleBin) MoveToRecycle(originalPath string, deletedBy string) (*RecycleEntry, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.config.Enabled {
		return nil, fmt.Errorf("回收站未启用")
	}

	if err := rb.ensureDirs(); err != nil {
		return nil, err
	}

	// 构造完整路径并验证文件存在
	fullPath := filepath.Join(rb.sharePath, originalPath)

	// 防止路径遍历攻击
	relPath, err := filepath.Rel(rb.sharePath, fullPath)
	if err != nil || len(relPath) > 0 && relPath[0] == '.' {
		return nil, fmt.Errorf("无效的文件路径")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	// 生成唯一 ID
	id := uuid.New().String()

	// 计算文件大小
	var size int64
	if !info.IsDir() {
		size = info.Size()
	}

	// 创建条目
	entry := &RecycleEntry{
		ID:           id,
		OriginalPath: originalPath,
		RecyclePath:  rb.getEntryPath(id),
		Name:         info.Name(),
		IsDir:        info.IsDir(),
		Size:         size,
		DeletedAt:    time.Now(),
		DeletedBy:    deletedBy,
	}

	// 移动文件到回收站
	destPath := rb.getEntryPath(id)
	if err := os.Rename(fullPath, destPath); err != nil {
		return nil, fmt.Errorf("移动文件到回收站失败: %w", err)
	}

	// 保存元数据
	if err := rb.saveEntry(entry); err != nil {
		// 尝试回滚
		//nolint:errcheck
		os.Rename(destPath, fullPath)
		return nil, err
	}

	return entry, nil
}

// Restore 从回收站恢复文件到原始位置。
// id 为回收站条目 ID。
func (rb *RecycleBin) Restore(id string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	entry, err := rb.loadEntry(id)
	if err != nil {
		return fmt.Errorf("条目不存在: %w", err)
	}

	// 构造恢复目标路径
	restorePath := filepath.Join(rb.sharePath, entry.OriginalPath)

	// 检查目标位置是否已存在同名文件
	if _, err := os.Stat(restorePath); err == nil {
		return fmt.Errorf("目标位置已存在同名文件: %s", entry.OriginalPath)
	}

	// 确保目标父目录存在
	parentDir := filepath.Dir(restorePath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 移动文件回原位
	recycleFilePath := rb.getEntryPath(id)
	if err := os.Rename(recycleFilePath, restorePath); err != nil {
		return fmt.Errorf("恢复文件失败: %w", err)
	}

	// 删除元数据
	if err := rb.deleteEntryMeta(id); err != nil {
		return err
	}

	return nil
}

// Purge 永久删除回收站中的指定条目。
// id 为回收站条目 ID。
func (rb *RecycleBin) Purge(id string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	entry, err := rb.loadEntry(id)
	if err != nil {
		return fmt.Errorf("条目不存在: %w", err)
	}

	// 删除文件
	recycleFilePath := rb.getEntryPath(id)
	if err := os.RemoveAll(recycleFilePath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	// 删除元数据
	if err := rb.deleteEntryMeta(entry.ID); err != nil {
		return err
	}

	return nil
}

// PurgeAll 清空回收站，永久删除所有文件。
func (rb *RecycleBin) PurgeAll() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	entries, err := rb.listEntries()
	if err != nil {
		return fmt.Errorf("列出回收站条目失败: %w", err)
	}

	var lastErr error
	for _, entry := range entries {
		recycleFilePath := rb.getEntryPath(entry.ID)
		if err := os.RemoveAll(recycleFilePath); err != nil {
			lastErr = fmt.Errorf("删除文件 %s 失败: %w", entry.Name, err)
			continue
		}
		if err := rb.deleteEntryMeta(entry.ID); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// AutoClean 自动清理超过保留期的文件。
// 返回被清理的条目数量。
func (rb *RecycleBin) AutoClean() (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.config.RetentionDays <= 0 {
		return 0, nil
	}

	entries, err := rb.listEntries()
	if err != nil {
		return 0, fmt.Errorf("列出回收站条目失败: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -rb.config.RetentionDays)
	cleaned := 0

	for _, entry := range entries {
		if entry.DeletedAt.Before(cutoff) {
			recycleFilePath := rb.getEntryPath(entry.ID)
			if err := os.RemoveAll(recycleFilePath); err != nil {
				continue
			}
			//nolint:errcheck
			rb.deleteEntryMeta(entry.ID)
			cleaned++
		}
	}

	return cleaned, nil
}

// List 列出回收站中的所有条目，按删除时间降序排列。
func (rb *RecycleBin) List() ([]RecycleEntry, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.listEntries()
}

// listEntries 内部列出方法（调用者需持锁）。
func (rb *RecycleBin) listEntries() ([]RecycleEntry, error) {
	if err := rb.ensureDirs(); err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(rb.metaPath)
	if err != nil {
		return nil, fmt.Errorf("读取元数据目录失败: %w", err)
	}

	var entries []RecycleEntry
	for _, de := range dirEntries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}

		id := de.Name()[:len(de.Name())-5] // 去掉 .json 后缀
		entry, err := rb.loadEntry(id)
		if err != nil {
			continue
		}

		entries = append(entries, *entry)
	}

	// 按删除时间降序排列
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DeletedAt.After(entries[j].DeletedAt)
	})

	return entries, nil
}

// GetStats 获取回收站统计信息。
func (rb *RecycleBin) GetStats() (*RecycleStats, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	entries, err := rb.listEntries()
	if err != nil {
		return nil, err
	}

	stats := &RecycleStats{
		TotalItems: len(entries),
	}

	for _, entry := range entries {
		stats.TotalSize += entry.Size
		if entry.IsDir {
			stats.FolderCount++
		} else {
			stats.FileCount++
		}
	}

	return stats, nil
}

// UpdateConfig 更新回收站配置。
func (rb *RecycleBin) UpdateConfig(config RecycleBinConfig) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.config = config
}

// GetConfig 获取当前回收站配置。
func (rb *RecycleBin) GetConfig() RecycleBinConfig {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.config
}

// GetID 获取条目。
func (rb *RecycleBin) GetID(id string) (*RecycleEntry, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.loadEntry(id)
}
