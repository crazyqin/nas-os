// Package fileversion 文件版本控制 - 类似群晖 Drive 文件版本历史
// 支持文件版本管理、恢复、清理策略
package fileversion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// VersionState 版本状态
type VersionState string

const (
	VersionActive   VersionState = "active"
	VersionArchived VersionState = "archived"
	VersionDeleted  VersionState = "deleted"
)

// CleanupPolicy 清理策略
type CleanupPolicy string

const (
	CleanupByVersionCount CleanupPolicy = "version_count"
	CleanupByAge          CleanupPolicy = "age"
	CleanupBySize         CleanupPolicy = "size"
	CleanupSmart          CleanupPolicy = "smart" // 保留最近的、每天一个、每周一个
)

// FileVersion 文件版本
type FileVersion struct {
	ID          string       `json:"id"`
	FilePath    string       `json:"file_path"`
	Version     int          `json:"version"`
	Size        int64        `json:"size"`
	SHA256      string       `json:"sha256"`
	ModTime     time.Time    `json:"mod_time"`
	CreatedAt   time.Time    `json:"created_at"`
	State       VersionState `json:"state"`
	Comment     string       `json:"comment"`
	CreatedBy   string       `json:"created_by"`
	IsRestore   bool         `json:"is_restore"`
	RestoreFrom string       `json:"restore_from,omitempty"`
}

// FileHistory 文件历史
type FileHistory struct {
	FilePath    string        `json:"file_path"`
	CurrentSize int64         `json:"current_size"`
	Versions    []FileVersion `json:"versions"`
	TotalSize   int64         `json:"total_size"`
	MaxVersions int           `json:"max_versions"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// VersionConfig 版本配置
type VersionConfig struct {
	StoragePath     string        `json:"storage_path"`
	MaxVersions     int           `json:"max_versions"`
	MaxTotalSize    int64         `json:"max_total_size"` // per file
	MaxAge          time.Duration `json:"max_age"`
	CleanupPolicy   CleanupPolicy `json:"cleanup_policy"`
	AutoVersion     bool          `json:"auto_version"`  // auto-version on save
	ExcludePatterns []string      `json:"exclude_patterns"`
	IncludePatterns []string      `json:"include_patterns"`
}

// RestoreResult 恢复结果
type RestoreResult struct {
	FilePath    string    `json:"file_path"`
	Version     int       `json:"version"`
	RestoredAt  time.Time `json:"restored_at"`
	Size        int64     `json:"size"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// VersionStats 版本统计
type VersionStats struct {
	TotalFiles    int   `json:"total_files"`
	TotalVersions int   `json:"total_versions"`
	TotalSize     int64 `json:"total_size"`
	OldestVersion time.Time `json:"oldest_version"`
	NewestVersion time.Time `json:"newest_version"`
}

// Manager 版本管理器
type Manager struct {
	mu       sync.RWMutex
	config   VersionConfig
	histories map[string]*FileHistory
	storePath string
}

// NewManager 创建版本管理器
func NewManager(config VersionConfig) *Manager {
	if config.MaxVersions <= 0 {
		config.MaxVersions = 32
	}
	if config.MaxTotalSize <= 0 {
		config.MaxTotalSize = 10 * 1024 * 1024 * 1024 // 10GB per file
	}
	if config.CleanupPolicy == "" {
		config.CleanupPolicy = CleanupSmart
	}

	storePath := filepath.Join(config.StoragePath, ".versions")
	os.MkdirAll(storePath, 0755)

	return &Manager{
		config:    config,
		histories: make(map[string]*FileHistory),
		storePath: storePath,
	}
}

// CreateVersion 创建文件版本
func (m *Manager) CreateVersion(filePath string, reader io.Reader, comment, createdBy string) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isExcluded(filePath) {
		return nil, errors.New("file excluded from versioning")
	}

	// Read content and compute hash
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Get or create history
	history, exists := m.histories[filePath]
	if !exists {
		history = &FileHistory{
			FilePath:    filePath,
			MaxVersions: m.config.MaxVersions,
			CreatedAt:   time.Now(),
		}
		m.histories[filePath] = history
	}

	// Check if content is same as latest version
	if len(history.Versions) > 0 {
		latest := history.Versions[len(history.Versions)-1]
		if latest.SHA256 == hashStr {
			return &latest, nil // No change
		}
	}

	// Create version
	versionNum := 1
	if len(history.Versions) > 0 {
		versionNum = history.Versions[len(history.Versions)-1].Version + 1
	}

	version := FileVersion{
		ID:        fmt.Sprintf("%s-v%d", filepath.Base(filePath), versionNum),
		FilePath:  filePath,
		Version:   versionNum,
		Size:      int64(len(content)),
		SHA256:    hashStr,
		ModTime:   time.Now(),
		CreatedAt: time.Now(),
		State:     VersionActive,
		Comment:   comment,
		CreatedBy: createdBy,
	}

	// Store version file
	versionDir := filepath.Join(m.storePath, hashStr[:2])
	os.MkdirAll(versionDir, 0755)
	versionFile := filepath.Join(versionDir, hashStr)
	if err := os.WriteFile(versionFile, content, 0644); err != nil {
		return nil, fmt.Errorf("store version: %w", err)
	}

	history.Versions = append(history.Versions, version)
	history.CurrentSize = version.Size
	history.TotalSize += version.Size
	history.UpdatedAt = time.Now()

	// Cleanup old versions
	m.cleanup(history)

	return &version, nil
}

// GetVersion 获取特定版本
func (m *Manager) GetVersion(filePath string, version int) (*FileVersion, []byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.histories[filePath]
	if !exists {
		return nil, nil, fmt.Errorf("no history for %q", filePath)
	}

	for _, v := range history.Versions {
		if v.Version == version && v.State == VersionActive {
			content, err := m.readVersion(v.SHA256)
			if err != nil {
				return nil, nil, err
			}
			return &v, content, nil
		}
	}

	return nil, nil, fmt.Errorf("version %d not found", version)
}

// GetHistory 获取文件版本历史
func (m *Manager) GetHistory(filePath string) (*FileHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.histories[filePath]
	if !exists {
		return nil, fmt.Errorf("no history for %q", filePath)
	}
	return history, nil
}

// ListHistories 列出所有文件历史
func (m *Manager) ListHistories() []FileHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	histories := make([]FileHistory, 0, len(m.histories))
	for _, h := range m.histories {
		histories = append(histories, *h)
	}
	return histories
}

// RestoreVersion 恢复到特定版本
func (m *Manager) RestoreVersion(filePath string, version int, targetPath string) (*RestoreResult, error) {
	m.mu.RLock()
	history, exists := m.histories[filePath]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("no history for %q", filePath)
	}

	var targetVersion *FileVersion
	for _, v := range history.Versions {
		if v.Version == version && v.State == VersionActive {
			vCopy := v
			targetVersion = &vCopy
			break
		}
	}
	m.mu.RUnlock()

	if targetVersion == nil {
		return nil, fmt.Errorf("version %d not found", version)
	}

	content, err := m.readVersion(targetVersion.SHA256)
	if err != nil {
		return &RestoreResult{
			FilePath: filePath,
			Version:  version,
			Error:    err.Error(),
		}, err
	}

	restoreTo := targetPath
	if restoreTo == "" {
		restoreTo = filePath
	}

	if err := os.MkdirAll(filepath.Dir(restoreTo), 0755); err != nil {
		return &RestoreResult{Error: err.Error()}, err
	}

	if err := os.WriteFile(restoreTo, content, 0644); err != nil {
		return &RestoreResult{Error: err.Error()}, err
	}

	// Create a version marking the restore (skip nil reader)
	if history, exists := m.histories[filePath]; exists {
		history.UpdatedAt = time.Now()
	}

	return &RestoreResult{
		FilePath:   restoreTo,
		Version:    version,
		RestoredAt: time.Now(),
		Size:       targetVersion.Size,
		Success:    true,
	}, nil
}

// DeleteVersion 删除特定版本
func (m *Manager) DeleteVersion(filePath string, version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	history, exists := m.histories[filePath]
	if !exists {
		return fmt.Errorf("no history for %q", filePath)
	}

	for i := range history.Versions {
		if history.Versions[i].Version == version {
			history.Versions[i].State = VersionDeleted
			history.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("version %d not found", version)
}

// PurgeDeleted 清除已删除的版本
func (m *Manager) PurgeDeleted(filePath string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	history, exists := m.histories[filePath]
	if !exists {
		return 0, nil
	}

	count := 0
	active := make([]FileVersion, 0, len(history.Versions))
	for _, v := range history.Versions {
		if v.State == VersionDeleted {
			count++
			history.TotalSize -= v.Size
		} else {
			active = append(active, v)
		}
	}
	history.Versions = active
	history.UpdatedAt = time.Now()
	return count, nil
}

// DeleteHistory 删除文件历史
func (m *Manager) DeleteHistory(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.histories[filePath]; !exists {
		return fmt.Errorf("no history for %q", filePath)
	}

	delete(m.histories, filePath)
	return nil
}

// GetStats 获取版本统计
func (m *Manager) GetStats() VersionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := VersionStats{}
	var oldest, newest time.Time

	for _, h := range m.histories {
		stats.TotalFiles++
		stats.TotalSize += h.TotalSize
		for _, v := range h.Versions {
			if v.State == VersionActive {
				stats.TotalVersions++
				if oldest.IsZero() || v.CreatedAt.Before(oldest) {
					oldest = v.CreatedAt
				}
				if newest.IsZero() || v.CreatedAt.After(newest) {
					newest = v.CreatedAt
				}
			}
		}
	}

	stats.OldestVersion = oldest
	stats.NewestVersion = newest
	return stats
}

// SetConfig 更新配置
func (m *Manager) SetConfig(config VersionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *Manager) GetConfig() VersionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ExportMetadata 导出元数据
func (m *Manager) ExportMetadata() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := struct {
		Histories map[string]*FileHistory `json:"histories"`
		Config    VersionConfig            `json:"config"`
	}{
		Histories: m.histories,
		Config:    m.config,
	}
	return json.MarshalIndent(data, "", "  ")
}

// ImportMetadata 导入元数据
func (m *Manager) ImportMetadata(data []byte) error {
	var imported struct {
		Histories map[string]*FileHistory `json:"histories"`
	}
	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for path, history := range imported.Histories {
		m.histories[path] = history
	}
	return nil
}

// DiffVersions 比较两个版本
func (m *Manager) DiffVersions(filePath string, v1, v2 int) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.histories[filePath]
	if !exists {
		return false, fmt.Errorf("no history for %q", filePath)
	}

	var hash1, hash2 string
	for _, v := range history.Versions {
		if v.Version == v1 {
			hash1 = v.SHA256
		}
		if v.Version == v2 {
			hash2 = v.SHA256
		}
	}

	if hash1 == "" || hash2 == "" {
		return false, errors.New("version not found")
	}

	return hash1 == hash2, nil
}

// CleanupVersions 清理版本
func (m *Manager) CleanupVersions() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalCleaned := 0
	for _, history := range m.histories {
		before := len(history.Versions)
		m.cleanup(history)
		totalCleaned += before - len(history.Versions)
	}
	return totalCleaned, nil
}

func (m *Manager) cleanup(history *FileHistory) {
	switch m.config.CleanupPolicy {
	case CleanupByVersionCount:
		if len(history.Versions) > m.config.MaxVersions {
			excess := len(history.Versions) - m.config.MaxVersions
			for i := 0; i < excess; i++ {
				history.TotalSize -= history.Versions[i].Size
			}
			history.Versions = history.Versions[excess:]
		}

	case CleanupByAge:
		cutoff := time.Now().Add(-m.config.MaxAge)
		active := make([]FileVersion, 0)
		for _, v := range history.Versions {
			if v.CreatedAt.After(cutoff) {
				active = append(active, v)
			} else {
				history.TotalSize -= v.Size
			}
		}
		history.Versions = active

	case CleanupBySize:
		for history.TotalSize > m.config.MaxTotalSize && len(history.Versions) > 1 {
			history.TotalSize -= history.Versions[0].Size
			history.Versions = history.Versions[1:]
		}

	case CleanupSmart:
		m.smartCleanup(history)
	}

	history.UpdatedAt = time.Now()
}

func (m *Manager) smartCleanup(history *FileHistory) {
	if len(history.Versions) <= m.config.MaxVersions {
		return
	}

	// Keep: all from last 24h, one per day for last week, one per week before that
	now := time.Now()
	var kept []FileVersion

	// Sort by time (should already be sorted)
	sort.Slice(history.Versions, func(i, j int) bool {
		return history.Versions[i].CreatedAt.Before(history.Versions[j].CreatedAt)
	})

	dayBuckets := make(map[string]bool)
	weekBuckets := make(map[string]bool)

	for _, v := range history.Versions {
		age := now.Sub(v.CreatedAt)

		if age < 24*time.Hour {
			// Keep all from last 24h
			kept = append(kept, v)
			continue
		}

		if age < 7*24*time.Hour {
			// One per day
			day := v.CreatedAt.Format("2006-01-02")
			if !dayBuckets[day] {
				dayBuckets[day] = true
				kept = append(kept, v)
			}
			continue
		}

		// One per week
		year, week := v.CreatedAt.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%d", year, week)
		if !weekBuckets[weekKey] {
			weekBuckets[weekKey] = true
			kept = append(kept, v)
		}
	}

	// Ensure we keep at least MaxVersions
	if len(kept) > m.config.MaxVersions {
		kept = kept[len(kept)-m.config.MaxVersions:]
	}

	var totalSize int64
	for _, v := range kept {
		totalSize += v.Size
	}
	history.Versions = kept
	history.TotalSize = totalSize
}

func (m *Manager) readVersion(hash string) ([]byte, error) {
	versionFile := filepath.Join(m.storePath, hash[:2], hash)
	return os.ReadFile(versionFile)
}

func (m *Manager) isExcluded(filePath string) bool {
	base := filepath.Base(filePath)
	for _, pattern := range m.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
