package fileretriever

import (
	"fmt"
	"sync"
	"time"
)

// FileVersionManager 文件版本恢复管理器
type FileVersionManager struct {
	mu        sync.RWMutex
	versions  map[string][]*FileVersion
	recycles  map[string]*RecycleEntry
	config    *VersionConfig
}

type VersionConfig struct {
	MaxVersions     int  `json:"max_versions"`
	RetentionDays   int  `json:"retention_days"`
	RecycleBinDays  int  `json:"recycle_bin_days"`
	AutoCleanup     bool `json:"auto_cleanup"`
}

type FileVersion struct {
	ID         string    `json:"id"`
	FilePath   string    `json:"file_path"`
	Version    int       `json:"version"`
	Size       int64     `json:"size_bytes"`
	Checksum   string    `json:"checksum"`
	ModifiedBy string    `json:"modified_by"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
	IsCurrent  bool      `json:"is_current"`
}

type RecycleEntry struct {
	ID          string    `json:"id"`
	OriginalPath string   `json:"original_path"`
	DeletedPath  string   `json:"deleted_path"`
	Size        int64     `json:"size_bytes"`
	DeletedBy   string    `json:"deleted_by"`
	DeletedAt   time.Time `json:"deleted_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	FileType    string    `json:"file_type"`
}

type RetrievalStats struct {
	TotalVersions   int   `json:"total_versions"`
	TotalRecycle    int   `json:"total_recycle_entries"`
	TotalSize       int64 `json:"total_size_bytes"`
	RecycleSize     int64 `json:"recycle_size_bytes"`
	OldestVersion   *time.Time `json:"oldest_version,omitempty"`
	NewestVersion   *time.Time `json:"newest_version,omitempty"`
}

func NewFileVersionManager(config *VersionConfig) *FileVersionManager {
	if config == nil {
		config = &VersionConfig{
			MaxVersions:    50,
			RetentionDays:  90,
			RecycleBinDays: 30,
			AutoCleanup:    true,
		}
	}
	return &FileVersionManager{
		versions: make(map[string][]*FileVersion),
		recycles: make(map[string]*RecycleEntry),
		config:   config,
	}
}

func (m *FileVersionManager) SaveVersion(version *FileVersion) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if version.FilePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}

	versions := m.versions[version.FilePath]

	// 检查版本数量限制
	if len(versions) >= m.config.MaxVersions {
		// 移除最旧的版本
		versions = versions[1:]
	}

	// 标记所有版本为非当前
	for _, v := range versions {
		v.IsCurrent = false
	}

	version.ID = fmt.Sprintf("ver_%d", time.Now().UnixNano())
	version.Version = len(versions) + 1
	version.CreatedAt = time.Now()
	version.IsCurrent = true

	versions = append(versions, version)
	m.versions[version.FilePath] = versions

	return version, nil
}

func (m *FileVersionManager) GetVersions(filePath string) ([]*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("没有找到文件版本: %s", filePath)
	}

	return versions, nil
}

func (m *FileVersionManager) GetVersion(filePath string, versionNum int) (*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("没有找到文件版本: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == versionNum {
			return v, nil
		}
	}

	return nil, fmt.Errorf("版本不存在: %d", versionNum)
}

func (m *FileVersionManager) GetCurrentVersion(filePath string) (*FileVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[filePath]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("没有找到文件版本: %s", filePath)
	}

	for _, v := range versions {
		if v.IsCurrent {
			return v, nil
		}
	}

	return versions[len(versions)-1], nil
}

func (m *FileVersionManager) RestoreVersion(filePath string, versionNum int) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("没有找到文件版本: %s", filePath)
	}

	var target *FileVersion
	for _, v := range versions {
		if v.Version == versionNum {
			target = v
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("版本不存在: %d", versionNum)
	}

	// 标记所有版本为非当前
	for _, v := range versions {
		v.IsCurrent = false
	}

	// 创建新版本作为恢复
	newVersion := &FileVersion{
		ID:         fmt.Sprintf("ver_%d", time.Now().UnixNano()),
		FilePath:   filePath,
		Version:    len(versions) + 1,
		Size:       target.Size,
		Checksum:   target.Checksum,
		ModifiedBy: "system",
		Comment:    fmt.Sprintf("从版本 %d 恢复", versionNum),
		CreatedAt:  time.Now(),
		IsCurrent:  true,
	}

	versions = append(versions, newVersion)
	m.versions[filePath] = versions

	return newVersion, nil
}

func (m *FileVersionManager) DeleteVersion(filePath string, versionNum int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.versions[filePath]
	if !exists {
		return fmt.Errorf("没有找到文件版本: %s", filePath)
	}

	for i, v := range versions {
		if v.Version == versionNum {
			m.versions[filePath] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("版本不存在: %d", versionNum)
}

func (m *FileVersionManager) AddToRecycle(entry *RecycleEntry) (*RecycleEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.OriginalPath == "" {
		return nil, fmt.Errorf("原始路径不能为空")
	}

	entry.ID = fmt.Sprintf("rec_%d", time.Now().UnixNano())
	entry.DeletedAt = time.Now()
	entry.ExpiresAt = time.Now().AddDate(0, 0, m.config.RecycleBinDays)

	m.recycles[entry.ID] = entry
	return entry, nil
}

func (m *FileVersionManager) GetRecycleEntry(id string) (*RecycleEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.recycles[id]
	if !exists {
		return nil, fmt.Errorf("回收站条目不存在: %s", id)
	}

	return entry, nil
}

func (m *FileVersionManager) ListRecycle() []*RecycleEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*RecycleEntry, 0, len(m.recycles))
	for _, e := range m.recycles {
		if e.ExpiresAt.After(time.Now()) {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m *FileVersionManager) RestoreFromRecycle(id string) (*RecycleEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.recycles[id]
	if !exists {
		return nil, fmt.Errorf("回收站条目不存在: %s", id)
	}

	if entry.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("回收站条目已过期")
	}

	delete(m.recycles, id)
	return entry, nil
}

func (m *FileVersionManager) EmptyRecycle() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.recycles)
	m.recycles = make(map[string]*RecycleEntry)
	return count
}

func (m *FileVersionManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for id, entry := range m.recycles {
		if entry.ExpiresAt.Before(now) {
			delete(m.recycles, id)
			count++
		}
	}
	return count
}

func (m *FileVersionManager) GetStats() *RetrievalStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &RetrievalStats{
		TotalVersions: 0,
		TotalRecycle:  len(m.recycles),
	}

	for _, versions := range m.versions {
		stats.TotalVersions += len(versions)
		for _, v := range versions {
			stats.TotalSize += v.Size
			if stats.OldestVersion == nil || v.CreatedAt.Before(*stats.OldestVersion) {
				stats.OldestVersion = &v.CreatedAt
			}
			if stats.NewestVersion == nil || v.CreatedAt.After(*stats.NewestVersion) {
				stats.NewestVersion = &v.CreatedAt
			}
		}
	}

	for _, e := range m.recycles {
		stats.RecycleSize += e.Size
	}

	return stats
}
