// Package websharepro - 文件版本历史模块
// 提供文件版本管理、快照、回滚功能
// 支持增量版本、自动清理、版本标签
package websharepro

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// VersionStatus 版本状态.
type VersionStatus string

const (
	VersionActive   VersionStatus = "active"
	VersionArchived VersionStatus = "archived"
	VersionDeleted  VersionStatus = "deleted"
	VersionLocked   VersionStatus = "locked"
)

// VersionPolicy 版本保留策略.
type VersionPolicy struct {
	MaxVersions     int           `json:"maxVersions"`     // 最大版本数
	MaxAge          time.Duration `json:"maxAge"`          // 最大保留时长
	KeepMinVersions int           `json:"keepMinVersions"` // 最少保留版本数
	AutoCleanup     bool          `json:"autoCleanup"`     // 自动清理
	CleanupInterval time.Duration `json:"cleanupInterval"` // 清理间隔
	CompressOld     bool          `json:"compressOld"`     // 压缩旧版本
}

// FileVersion 文件版本.
type FileVersion struct {
	ID          string        `json:"id"`
	FilePath    string        `json:"filePath"`
	Version     int           `json:"version"`
	SHA256      string        `json:"sha256"`
	Size        int64         `json:"size"`
	Status      VersionStatus `json:"status"`
	Message     string        `json:"message,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Author      string        `json:"author"`
	CreatedAt   time.Time     `json:"createdAt"`
	StoragePath string        `json:"storagePath"`        // 版本数据存储位置
	IsSnapshot  bool          `json:"isSnapshot"`         // 是否为完整快照
	ParentID    string        `json:"parentId,omitempty"` // 增量版本的父版本
}

// VersionDiff 版本差异.
type VersionDiff struct {
	FromVersion   int      `json:"fromVersion"`
	ToVersion     int      `json:"toVersion"`
	Additions     int      `json:"additions"`
	Deletions     int      `json:"deletions"`
	Modifications int      `json:"modifications"`
	ChangedFiles  []string `json:"changedFiles,omitempty"`
	Summary       string   `json:"summary"`
}

// VersionStats 版本统计.
type VersionStats struct {
	TotalVersions    int64      `json:"totalVersions"`
	TotalSize        int64      `json:"totalSize"`
	ActiveVersions   int64      `json:"activeVersions"`
	ArchivedVersions int64      `json:"archivedVersions"`
	OldestVersion    *time.Time `json:"oldestVersion,omitempty"`
	NewestVersion    *time.Time `json:"newestVersion,omitempty"`
	AvgVersionSize   int64      `json:"avgVersionSize"`
}

// VersionManager 版本管理器.
type VersionManager struct {
	mu       sync.RWMutex
	versions map[string][]*FileVersion // filePath -> versions
	policy   *VersionPolicy
	storage  VersionStorage
	stats    *VersionStats
}

// VersionStorage 版本存储接口.
type VersionStorage interface {
	Save(versionID string, data []byte) error
	Load(versionID string) ([]byte, error)
	Delete(versionID string) error
	List(prefix string) ([]string, error)
}

// NewVersionManager 创建版本管理器.
func NewVersionManager(storage VersionStorage, policy *VersionPolicy) *VersionManager {
	if policy == nil {
		policy = &VersionPolicy{
			MaxVersions:     50,
			MaxAge:          90 * 24 * time.Hour, // 90 天
			KeepMinVersions: 5,
			AutoCleanup:     true,
			CleanupInterval: 24 * time.Hour,
			CompressOld:     true,
		}
	}

	m := &VersionManager{
		versions: make(map[string][]*FileVersion),
		policy:   policy,
		storage:  storage,
		stats:    &VersionStats{},
	}

	if policy.AutoCleanup {
		go m.cleanupWorker()
	}

	return m
}

// CreateVersion 创建新版本.
func (m *VersionManager) CreateVersion(filePath, author, message string, data []byte) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算哈希
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// 检查是否与最新版本相同
	versions := m.versions[filePath]
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		if latest.SHA256 == hashStr {
			return nil, errors.New("content unchanged, skipping version")
		}
	}

	// 确定版本号
	versionNum := 1
	if len(versions) > 0 {
		versionNum = versions[len(versions)-1].Version + 1
	}

	// 决定是否创建快照
	isSnapshot := versionNum == 1 || versionNum%m.policy.KeepMinVersions == 0

	versionID := fmt.Sprintf("%s-v%d-%d", filePath, versionNum, time.Now().UnixNano())

	version := &FileVersion{
		ID:          versionID,
		FilePath:    filePath,
		Version:     versionNum,
		SHA256:      hashStr,
		Size:        int64(len(data)),
		Status:      VersionActive,
		Message:     message,
		Author:      author,
		CreatedAt:   time.Now(),
		StoragePath: versionID,
		IsSnapshot:  isSnapshot,
	}

	// 存储版本数据
	if m.storage != nil {
		if err := m.storage.Save(versionID, data); err != nil {
			return nil, fmt.Errorf("save version data: %w", err)
		}
	}

	m.versions[filePath] = append(m.versions[filePath], version)

	// 更新统计
	m.stats.TotalVersions++
	m.stats.TotalSize += version.Size
	m.stats.ActiveVersions++
	if m.stats.OldestVersion == nil || version.CreatedAt.Before(*m.stats.OldestVersion) {
		m.stats.OldestVersion = &version.CreatedAt
	}
	m.stats.NewestVersion = &version.CreatedAt

	// 应用保留策略
	m.applyPolicy(filePath)

	return version, nil
}

// GetVersion 获取特定版本.
func (m *VersionManager) GetVersion(filePath string, versionNum int) (*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("no versions for file: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == versionNum {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version %d not found for file: %s", versionNum, filePath)
}

// GetLatestVersion 获取最新版本.
func (m *VersionManager) GetLatestVersion(filePath string) (*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("no versions for file: %s", filePath)
	}

	return versions[len(versions)-1], nil
}

// ListVersions 列出文件的所有版本.
func (m *VersionManager) ListVersions(filePath string) []*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[filePath]
	result := make([]*FileVersion, len(versions))
	copy(result, versions)
	return result
}

// LoadVersionData 加载版本数据.
func (m *VersionManager) LoadVersionData(versionID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.storage == nil {
		return nil, errors.New("storage not configured")
	}

	return m.storage.Load(versionID)
}

// Rollback 回滚到指定版本.
func (m *VersionManager) Rollback(filePath string, versionNum int, author string) (*FileVersion, error) {
	m.mu.RLock()
	versions, exists := m.versions[filePath]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("no versions for file: %s", filePath)
	}

	var target *FileVersion
	for _, v := range versions {
		if v.Version == versionNum {
			target = v
			break
		}
	}
	m.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("version %d not found", versionNum)
	}

	// 加载目标版本数据
	if m.storage == nil {
		return nil, errors.New("storage not configured")
	}

	data, err := m.storage.Load(target.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("load version data: %w", err)
	}

	// 创建新版本（基于回滚）
	return m.CreateVersion(filePath, author, fmt.Sprintf("rollback to v%d", versionNum), data)
}

// TagVersion 为版本添加标签.
func (m *VersionManager) TagVersion(filePath string, versionNum int, tag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions for file: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == versionNum {
			// 检查标签是否已存在
			for _, t := range v.Tags {
				if t == tag {
					return nil // 标签已存在
				}
			}
			v.Tags = append(v.Tags, tag)
			return nil
		}
	}

	return fmt.Errorf("version %d not found", versionNum)
}

// LockVersion 锁定版本（防止清理）.
func (m *VersionManager) LockVersion(filePath string, versionNum int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions for file: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == versionNum {
			v.Status = VersionLocked
			return nil
		}
	}

	return fmt.Errorf("version %d not found", versionNum)
}

// UnlockVersion 解锁版本.
func (m *VersionManager) UnlockVersion(filePath string, versionNum int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions for file: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == versionNum {
			v.Status = VersionActive
			return nil
		}
	}

	return fmt.Errorf("version %d not found", versionNum)
}

// DiffVersions 计算两个版本之间的差异.
func (m *VersionManager) DiffVersions(filePath string, fromVersion, toVersion int) (*VersionDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("no versions for file: %s", filePath)
	}

	var from, to *FileVersion
	for _, v := range versions {
		if v.Version == fromVersion {
			from = v
		}
		if v.Version == toVersion {
			to = v
		}
	}

	if from == nil || to == nil {
		return nil, errors.New("one or both versions not found")
	}

	// 基于哈希和大小估算差异
	diff := &VersionDiff{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	if from.SHA256 == to.SHA256 {
		diff.Summary = "no changes"
	} else {
		diff.Modifications = 1
		sizeDiff := to.Size - from.Size
		if sizeDiff > 0 {
			diff.Additions = 1
			diff.Summary = fmt.Sprintf("file grew by %d bytes", sizeDiff)
		} else if sizeDiff < 0 {
			diff.Deletions = 1
			diff.Summary = fmt.Sprintf("file shrunk by %d bytes", -sizeDiff)
		} else {
			diff.Summary = "content changed, same size"
		}
	}

	return diff, nil
}

// DeleteVersion 删除版本.
func (m *VersionManager) DeleteVersion(filePath string, versionNum int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions for file: %s", filePath)
	}

	for i, v := range versions {
		if v.Version == versionNum {
			if v.Status == VersionLocked {
				return errors.New("cannot delete locked version")
			}

			// 删除存储数据
			if m.storage != nil {
				m.storage.Delete(v.StoragePath)
			}

			// 更新状态
			v.Status = VersionDeleted
			m.stats.ActiveVersions--
			m.stats.TotalSize -= v.Size

			// 从列表移除
			m.versions[filePath] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("version %d not found", versionNum)
}

// GetStats 获取版本统计.
func (m *VersionManager) GetStats() *VersionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	if stats.TotalVersions > 0 {
		stats.AvgVersionSize = stats.TotalSize / stats.TotalVersions
	}
	return &stats
}

// ListAllVersions 列出所有文件的版本.
func (m *VersionManager) ListAllVersions() map[string][]*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*FileVersion)
	for path, versions := range m.versions {
		copied := make([]*FileVersion, len(versions))
		copy(copied, versions)
		result[path] = copied
	}
	return result
}

// SearchVersions 搜索版本.
func (m *VersionManager) SearchVersions(query string) []*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*FileVersion
	for _, versions := range m.versions {
		for _, v := range versions {
			if v.Status == VersionDeleted {
				continue
			}
			// 搜索消息、标签、作者
			if containsIgnoreCase(v.Message, query) ||
				containsIgnoreCase(v.Author, query) ||
				containsTag(v.Tags, query) {
				results = append(results, v)
			}
		}
	}

	// 按创建时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

// applyPolicy 应用保留策略.
func (m *VersionManager) applyPolicy(filePath string) {
	versions := m.versions[filePath]
	if len(versions) == 0 {
		return
	}

	// 计算需要保留的版本数
	toRemove := len(versions) - m.policy.MaxVersions
	if toRemove <= 0 {
		return
	}

	// 保留最少版本数
	if len(versions)-toRemove < m.policy.KeepMinVersions {
		toRemove = len(versions) - m.policy.KeepMinVersions
	}
	if toRemove <= 0 {
		return
	}

	// 从最旧的版本开始清理
	for i := 0; i < toRemove; i++ {
		v := versions[i]
		if v.Status == VersionLocked {
			continue
		}

		// 检查是否有标签
		if len(v.Tags) > 0 {
			continue
		}

		v.Status = VersionArchived
		m.stats.ActiveVersions--
		m.stats.ArchivedVersions++
	}
}

// cleanupWorker 后台清理工作协程.
func (m *VersionManager) cleanupWorker() {
	ticker := time.NewTicker(m.policy.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 执行清理.
func (m *VersionManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for filePath, versions := range m.versions {
		var cleaned []*FileVersion
		for _, v := range versions {
			if v.Status == VersionLocked {
				cleaned = append(cleaned, v)
				continue
			}

			age := now.Sub(v.CreatedAt)
			if age > m.policy.MaxAge && v.Status != VersionActive {
				// 删除过期版本
				if m.storage != nil {
					m.storage.Delete(v.StoragePath)
				}
				m.stats.TotalVersions--
				m.stats.TotalSize -= v.Size
				if v.Status == VersionArchived {
					m.stats.ArchivedVersions--
				}
			} else {
				cleaned = append(cleaned, v)
			}
		}
		m.versions[filePath] = cleaned
	}
}

// GetFileVersionCount 获取文件版本数量.
func (m *VersionManager) GetFileVersionCount(filePath string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.versions[filePath])
}

// HasVersions 检查文件是否有版本.
func (m *VersionManager) HasVersions(filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	versions, exists := m.versions[filePath]
	return exists && len(versions) > 0
}

// 辅助函数

func containsIgnoreCase(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && containsFold(s, substr))
}

func containsFold(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func containsTag(tags []string, query string) bool {
	for _, tag := range tags {
		if equalFold(tag, query) {
			return true
		}
	}
	return false
}
