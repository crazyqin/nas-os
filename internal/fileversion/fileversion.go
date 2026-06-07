// Package fileversion 文件版本控制模块
// 提供文件版本快照、浏览、恢复、对比和存储优化功能
package fileversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 类型定义 ==========

// VersionConfig 版本控制配置
type VersionConfig struct {
	// StoragePath 版本存储根目录
	StoragePath string `json:"storage_path"`
	// MaxVersions 每个文件最大保留版本数
	MaxVersions int `json:"max_versions"`
	// RetentionDays 版本保留天数
	RetentionDays int `json:"retention_days"`
	// EnableIncremental 启用增量存储
	EnableIncremental bool `json:"enable_incremental"`
	// AutoCleanupInterval 自动清理间隔
	AutoCleanupInterval time.Duration `json:"auto_cleanup_interval"`
	// MaxStorageSize 最大存储大小（字节），0表示不限制
	MaxStorageSize int64 `json:"max_storage_size"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *VersionConfig {
	return &VersionConfig{
		StoragePath:         "/var/lib/nas-os/fileversions",
		MaxVersions:         50,
		RetentionDays:       90,
		EnableIncremental:   true,
		AutoCleanupInterval: 24 * time.Hour,
		MaxStorageSize:      0,
	}
}

// FileVersion 文件版本信息
type FileVersion struct {
	// ID 版本唯一标识
	ID string `json:"id"`
	// FilePath 原始文件路径
	FilePath string `json:"file_path"`
	// Version 版本号
	Version int `json:"version"`
	// Size 文件大小（字节）
	Size int64 `json:"size"`
	// Checksum 文件校验和（SHA256）
	Checksum string `json:"checksum"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// ModifiedAt 文件最后修改时间
	ModifiedAt time.Time `json:"modified_at"`
	// IsIncremental 是否为增量版本
	IsIncremental bool `json:"is_incremental"`
	// BaseVersionID 增量版本的基准版本ID
	BaseVersionID string `json:"base_version_id,omitempty"`
	// ChangeDescription 变更描述
	ChangeDescription string `json:"change_description,omitempty"`
	// StoragePath 版本存储路径
	StoragePath string `json:"storage_path"`
}

// VersionDiff 版本差异信息
type VersionDiff struct {
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// Version1 版本1信息
	Version1 *FileVersion `json:"version1"`
	// Version2 版本2信息
	Version2 *FileVersion `json:"version2"`
	// AddedLines 新增行数
	AddedLines int `json:"added_lines"`
	// RemovedLines 删除行数
	RemovedLines int `json:"removed_lines"`
	// ModifiedLines 修改行数
	ModifiedLines int `json:"modified_lines"`
	// Changes 变更详情
	Changes []DiffChange `json:"changes"`
}

// DiffChange 单行变更
type DiffChange struct {
	Type       string `json:"type"` // "add", "remove", "modify"
	LineNum    int    `json:"line_num"`
	Content    string `json:"content"`
	OldContent string `json:"old_content,omitempty"`
}

// VersionStats 版本统计信息
type VersionStats struct {
	TotalFiles    int       `json:"total_files"`
	TotalVersions int       `json:"total_versions"`
	TotalSize     int64     `json:"total_size"`
	OldestVersion time.Time `json:"oldest_version"`
	NewestVersion time.Time `json:"newest_version"`
}

// ========== 核心管理器 ==========

// Manager 版本控制管理器
type Manager struct {
	config   *VersionConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	versions map[string][]*FileVersion // 文件路径 -> 版本列表
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager 创建版本控制管理器
func NewManager(config *VersionConfig, logger *zap.Logger) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:   config,
		logger:   logger,
		versions: make(map[string][]*FileVersion),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动版本控制管理器
func (m *Manager) Start() error {
	m.logger.Info("启动文件版本控制管理器",
		zap.String("storage_path", m.config.StoragePath),
		zap.Int("max_versions", m.config.MaxVersions),
		zap.Int("retention_days", m.config.RetentionDays),
	)

	// 创建存储目录
	if err := os.MkdirAll(m.config.StoragePath, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 加载现有版本索引
	if err := m.loadIndex(); err != nil {
		m.logger.Warn("加载版本索引失败，将使用空索引", zap.Error(err))
	}

	// 启动自动清理
	go m.autoCleanup()

	return nil
}

// Stop 停止版本控制管理器
func (m *Manager) Stop() error {
	m.logger.Info("停止文件版本控制管理器")
	m.cancel()

	// 保存索引
	if err := m.saveIndex(); err != nil {
		m.logger.Error("保存版本索引失败", zap.Error(err))
	}

	return nil
}

// ========== 核心功能 ==========

// CreateVersion 创建文件版本快照
func (m *Manager) CreateVersion(ctx context.Context, filePath string, description string) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	// 计算文件校验和
	checksum, err := m.calculateChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	// 检查是否与最新版本相同
	versions := m.versions[filePath]
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		if latest.Checksum == checksum {
			m.logger.Debug("文件未变化，跳过版本创建", zap.String("file", filePath))
			return latest, nil
		}
	}

	// 生成版本ID
	versionID := m.generateVersionID(filePath, info.ModTime())

	// 确定版本号
	versionNum := 1
	if len(versions) > 0 {
		versionNum = versions[len(versions)-1].Version + 1
	}

	// 创建版本记录
	version := &FileVersion{
		ID:                versionID,
		FilePath:          filePath,
		Version:           versionNum,
		Size:              info.Size(),
		Checksum:          checksum,
		CreatedAt:         time.Now(),
		ModifiedAt:        info.ModTime(),
		IsIncremental:     false,
		ChangeDescription: description,
	}

	// 增量存储检查
	if m.config.EnableIncremental && len(versions) > 0 {
		baseVersion := versions[len(versions)-1]
		isIncremental, err := m.createIncrementalVersion(filePath, baseVersion)
		if err != nil {
			m.logger.Warn("创建增量版本失败，使用全量存储", zap.Error(err))
		} else if isIncremental {
			version.IsIncremental = true
			version.BaseVersionID = baseVersion.ID
		}
	}

	// 存储版本文件
	storagePath := m.getStoragePath(versionID)
	if err := m.storeVersion(filePath, storagePath, version.IsIncremental, version.BaseVersionID); err != nil {
		return nil, fmt.Errorf("存储版本失败: %w", err)
	}
	version.StoragePath = storagePath

	// 添加到版本列表
	m.versions[filePath] = append(m.versions[filePath], version)

	// 检查并清理旧版本
	m.cleanupOldVersions(filePath)

	m.logger.Info("创建文件版本成功",
		zap.String("file", filePath),
		zap.String("version_id", versionID),
		zap.Int("version_num", versionNum),
		zap.Int64("size", info.Size()),
	)

	return version, nil
}

// ListVersions 获取文件版本历史
func (m *Manager) ListVersions(filePath string) ([]*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return []*FileVersion{}, nil
	}

	// 返回副本
	result := make([]*FileVersion, len(versions))
	copy(result, versions)
	return result, nil
}

// GetVersion 获取指定版本信息
func (m *Manager) GetVersion(versionID string) (*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, versions := range m.versions {
		for _, v := range versions {
			if v.ID == versionID {
				return v, nil
			}
		}
	}

	return nil, fmt.Errorf("版本不存在: %s", versionID)
}

// RestoreVersion 恢复到指定版本
func (m *Manager) RestoreVersion(ctx context.Context, versionID string) error {
	m.mu.RLock()
	version, err := m.GetVersion(versionID)
	m.mu.RUnlock()

	if err != nil {
		return err
	}

	// 如果是增量版本，需要重建完整文件
	if version.IsIncremental {
		return m.restoreFromIncremental(version)
	}

	// 直接复制全量版本
	return m.copyVersionFile(version.StoragePath, version.FilePath)
}

// CompareVersions 对比两个版本的差异
func (m *Manager) CompareVersions(versionID1, versionID2 string) (*VersionDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取两个版本
	v1, err := m.getVersionByID(versionID1)
	if err != nil {
		return nil, fmt.Errorf("获取版本1失败: %w", err)
	}

	v2, err := m.getVersionByID(versionID2)
	if err != nil {
		return nil, fmt.Errorf("获取版本2失败: %w", err)
	}

	// 获取两个版本的内容
	content1, err := m.getVersionContent(v1)
	if err != nil {
		return nil, fmt.Errorf("读取版本1内容失败: %w", err)
	}

	content2, err := m.getVersionContent(v2)
	if err != nil {
		return nil, fmt.Errorf("读取版本2内容失败: %w", err)
	}

	// 执行差异对比
	return m.compareContents(v1, v2, content1, content2)
}

// GetStats 获取版本统计信息
func (m *Manager) GetStats() *VersionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &VersionStats{}
	var oldest, newest time.Time

	for _, versions := range m.versions {
		stats.TotalFiles++
		stats.TotalVersions += len(versions)

		for _, v := range versions {
			stats.TotalSize += v.Size

			if oldest.IsZero() || v.CreatedAt.Before(oldest) {
				oldest = v.CreatedAt
			}
			if newest.IsZero() || v.CreatedAt.After(newest) {
				newest = v.CreatedAt
			}
		}
	}

	stats.OldestVersion = oldest
	stats.NewestVersion = newest

	return stats
}

// ========== 内部方法 ==========

// calculateChecksum 计算文件校验和
func (m *Manager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateVersionID 生成版本ID
func (m *Manager) generateVersionID(filePath string, modTime time.Time) string {
	data := fmt.Sprintf("%s:%d:%d", filePath, modTime.UnixNano(), time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// getStoragePath 获取版本存储路径
func (m *Manager) getStoragePath(versionID string) string {
	// 使用两级目录结构避免单目录文件过多
	return filepath.Join(m.config.StoragePath, versionID[:2], versionID)
}

// storeVersion 存储版本文件
func (m *Manager) storeVersion(srcPath, dstPath string, incremental bool, baseVersionID string) error {
	// 创建目录
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	if incremental && baseVersionID != "" {
		// 增量存储：只存储差异
		return m.storeIncremental(srcPath, dstPath, baseVersionID)
	}

	// 全量存储
	return m.copyFile(srcPath, dstPath)
}

// storeIncremental 存储增量差异
func (m *Manager) storeIncremental(srcPath, dstPath, baseVersionID string) error {
	// 简化实现：存储完整文件，但标记为增量
	// 实际生产环境应使用rsync或二进制diff
	return m.copyFile(srcPath, dstPath)
}

// createIncrementalVersion 创建增量版本
func (m *Manager) createIncrementalVersion(filePath string, baseVersion *FileVersion) (bool, error) {
	// 获取当前文件大小
	currentInfo, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	// 如果文件较小（<1MB），不使用增量存储
	if currentInfo.Size() < 1024*1024 {
		return false, nil
	}

	// 检查基准版本是否存在
	if _, err := os.Stat(baseVersion.StoragePath); err != nil {
		return false, nil
	}

	return true, nil
}

// restoreFromIncremental 从增量版本恢复
func (m *Manager) restoreFromIncremental(version *FileVersion) error {
	// 获取基准版本
	baseVersion, err := m.getVersionByID(version.BaseVersionID)
	if err != nil {
		return fmt.Errorf("获取基准版本失败: %w", err)
	}

	// 恢复基准版本
	if err := m.copyVersionFile(baseVersion.StoragePath, version.FilePath); err != nil {
		return fmt.Errorf("恢复基准版本失败: %w", err)
	}

	// 应用增量（简化实现：直接覆盖）
	return m.copyVersionFile(version.StoragePath, version.FilePath)
}

// copyVersionFile 复制版本文件到目标路径
func (m *Manager) copyVersionFile(src, dst string) error {
	return m.copyFile(src, dst)
}

// copyFile 复制文件
func (m *Manager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// getVersionByID 根据ID获取版本
func (m *Manager) getVersionByID(versionID string) (*FileVersion, error) {
	for _, versions := range m.versions {
		for _, v := range versions {
			if v.ID == versionID {
				return v, nil
			}
		}
	}
	return nil, fmt.Errorf("版本不存在: %s", versionID)
}

// getVersionContent 获取版本内容
func (m *Manager) getVersionContent(version *FileVersion) ([]byte, error) {
	return os.ReadFile(version.StoragePath)
}

// compareContents 对比两个版本内容
func (m *Manager) compareContents(v1, v2 *FileVersion, content1, content2 []byte) (*VersionDiff, error) {
	diff := &VersionDiff{
		FilePath: v1.FilePath,
		Version1: v1,
		Version2: v2,
	}

	lines1 := splitLines(string(content1))
	lines2 := splitLines(string(content2))

	// 简单的逐行对比
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}

	for i := 0; i < maxLen; i++ {
		var line1, line2 string
		if i < len(lines1) {
			line1 = lines1[i]
		}
		if i < len(lines2) {
			line2 = lines2[i]
		}

		switch {
		case i >= len(lines1):
			diff.AddedLines++
			diff.Changes = append(diff.Changes, DiffChange{
				Type:    "add",
				LineNum: i + 1,
				Content: line2,
			})
		case i >= len(lines2):
			diff.RemovedLines++
			diff.Changes = append(diff.Changes, DiffChange{
				Type:    "remove",
				LineNum: i + 1,
				Content: line1,
			})
		case line1 != line2:
			diff.ModifiedLines++
			diff.Changes = append(diff.Changes, DiffChange{
				Type:       "modify",
				LineNum:    i + 1,
				Content:    line2,
				OldContent: line1,
			})
		}
	}

	return diff, nil
}

// splitLines 按行分割文本
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	lines := []string{}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// cleanupOldVersions 清理旧版本
func (m *Manager) cleanupOldVersions(filePath string) {
	versions := m.versions[filePath]
	if len(versions) <= m.config.MaxVersions {
		return
	}

	// 按版本号排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})

	// 删除超出限制的旧版本
	toDelete := versions[:len(versions)-m.config.MaxVersions]
	for _, v := range toDelete {
		if err := os.Remove(v.StoragePath); err != nil && !os.IsNotExist(err) {
			m.logger.Error("删除旧版本文件失败",
				zap.String("path", v.StoragePath),
				zap.Error(err),
			)
		}
	}

	m.versions[filePath] = versions[len(toDelete):]
}

// autoCleanup 自动清理过期版本
func (m *Manager) autoCleanup() {
	if m.config.AutoCleanupInterval <= 0 {
		m.logger.Debug("自动清理间隔未设置，跳过自动清理")
		return
	}

	ticker := time.NewTicker(m.config.AutoCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpiredVersions()
		}
	}
}

// cleanupExpiredVersions 清理过期版本
func (m *Manager) cleanupExpiredVersions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)

	for filePath, versions := range m.versions {
		var valid []*FileVersion
		for _, v := range versions {
			if v.CreatedAt.Before(cutoff) {
				// 删除过期版本文件
				if err := os.Remove(v.StoragePath); err != nil && !os.IsNotExist(err) {
					m.logger.Error("删除过期版本失败",
						zap.String("path", v.StoragePath),
						zap.Error(err),
					)
				}
			} else {
				valid = append(valid, v)
			}
		}
		m.versions[filePath] = valid
	}
}

// ========== 持久化 ==========

// VersionIndex 版本索引
type VersionIndex struct {
	Versions map[string][]*FileVersion `json:"versions"`
}

// loadIndex 加载版本索引
func (m *Manager) loadIndex() error {
	indexPath := filepath.Join(m.config.StoragePath, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var index VersionIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	m.versions = index.Versions
	return nil
}

// saveIndex 保存版本索引
func (m *Manager) saveIndex() error {
	indexPath := filepath.Join(m.config.StoragePath, "index.json")

	index := VersionIndex{
		Versions: m.versions,
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}

// ListAllVersions 列出所有文件的版本（用于API）
func (m *Manager) ListAllVersions() map[string][]*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*FileVersion)
	for k, v := range m.versions {
		copied := make([]*FileVersion, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result
}

// DeleteVersion 删除指定版本
func (m *Manager) DeleteVersion(versionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for filePath, versions := range m.versions {
		for i, v := range versions {
			if v.ID == versionID {
				// 删除版本文件
				if err := os.Remove(v.StoragePath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("删除版本文件失败: %w", err)
				}

				// 从列表中移除
				m.versions[filePath] = append(versions[:i], versions[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("版本不存在: %s", versionID)
}
