// Package fileversioncontrol 文件版本控制模块
// 提供文件快照、版本管理、回滚、差异比较等功能
package fileversioncontrol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// VersionStatus 版本状态
type VersionStatus string

const (
	StatusCurrent  VersionStatus = "current"  // 当前版本
	StatusPrevious VersionStatus = "previous" // 历史版本
	StatusDeleted  VersionStatus = "deleted"  // 已删除版本
	StatusArchived VersionStatus = "archived" // 已归档版本
)

// SnapshotType 快照类型
type SnapshotType string

const (
	SnapshotManual    SnapshotType = "manual"     // 手动快照
	SnapshotAuto      SnapshotType = "auto"       // 自动快照
	SnapshotScheduled SnapshotType = "scheduled"  // 定时快照
	SnapshotBeforeMod SnapshotType = "before_mod" // 修改前快照
)

// FileVersion 文件版本
type FileVersion struct {
	ID             string            `json:"id"`
	FilePath       string            `json:"file_path"`
	Version        int               `json:"version"`
	Size           int64             `json:"size"`
	Checksum       string            `json:"checksum"`
	Status         VersionStatus     `json:"status"`
	SnapshotType   SnapshotType      `json:"snapshot_type"`
	SnapshotID     string            `json:"snapshot_id"`
	Comment        string            `json:"comment"`
	CreatedBy      string            `json:"created_by"`
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	IsCompressed   bool              `json:"is_compressed"`
	CompressedSize int64             `json:"compressed_size,omitempty"`
}

// FileSnapshot 文件快照
type FileSnapshot struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Type          SnapshotType   `json:"type"`
	FilePaths     []string       `json:"file_paths"`
	Versions      map[string]int `json:"versions"` // filePath -> version
	TotalSize     int64          `json:"total_size"`
	CreatedBy     string         `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	IsLocked      bool           `json:"is_locked"`
	RetentionDays int            `json:"retention_days"`
}

// VersionDiff 版本差异
type VersionDiff struct {
	FilePath      string    `json:"file_path"`
	Version1      int       `json:"version1"`
	Version2      int       `json:"version2"`
	SizeDiff      int64     `json:"size_diff"`
	ChecksumMatch bool      `json:"checksum_match"`
	Changes       []Change  `json:"changes"`
	ComparedAt    time.Time `json:"compared_at"`
}

// Change 变更记录
type Change struct {
	Type     string `json:"type"` // add, modify, delete
	Line     int    `json:"line,omitempty"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	MaxVersions       int       `json:"max_versions"`
	MaxAgeDays        int       `json:"max_age_days"`
	KeepDaily         int       `json:"keep_daily"`
	KeepWeekly        int       `json:"keep_weekly"`
	KeepMonthly       int       `json:"keep_monthly"`
	KeepYearly        int       `json:"keep_yearly"`
	MinVersionsToKeep int       `json:"min_versions_to_keep"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// VersionStats 版本统计
type VersionStats struct {
	TotalFiles       int                   `json:"total_files"`
	TotalVersions    int                   `json:"total_versions"`
	TotalSnapshots   int                   `json:"total_snapshots"`
	TotalSize        int64                 `json:"total_size_bytes"`
	CompressedSize   int64                 `json:"compressed_size_bytes"`
	SpaceSaved       int64                 `json:"space_saved_bytes"`
	VersionsByStatus map[VersionStatus]int `json:"versions_by_status"`
	OldestVersion    *time.Time            `json:"oldest_version,omitempty"`
	NewestVersion    *time.Time            `json:"newest_version,omitempty"`
	AverageVersions  float64               `json:"average_versions_per_file"`
}

// FileVersionControl 文件版本控制器
type FileVersionControl struct {
	mu                sync.RWMutex
	versions          map[string][]FileVersion // filePath -> versions
	snapshots         map[string]*FileSnapshot
	retentionPolicies map[string]*RetentionPolicy
	config            *VersionControlConfig
}

// VersionControlConfig 版本控制配置
type VersionControlConfig struct {
	DefaultRetentionPolicyID string `json:"default_retention_policy_id"`
	AutoSnapshotOnModify     bool   `json:"auto_snapshot_on_modify"`
	MaxVersionsPerFile       int    `json:"max_versions_per_file"`
	CompressionEnabled       bool   `json:"compression_enabled"`
	DeduplicationEnabled     bool   `json:"deduplication_enabled"`
	EncryptionEnabled        bool   `json:"encryption_enabled"`
	ChecksumAlgorithm        string `json:"checksum_algorithm"`
	StoragePath              string `json:"storage_path"`
}

// NewFileVersionControl 创建文件版本控制器
func NewFileVersionControl(config *VersionControlConfig) *FileVersionControl {
	if config == nil {
		config = &VersionControlConfig{
			AutoSnapshotOnModify: true,
			MaxVersionsPerFile:   100,
			CompressionEnabled:   true,
			DeduplicationEnabled: true,
			EncryptionEnabled:    false,
			ChecksumAlgorithm:    "sha256",
			StoragePath:          "/data/.versions",
		}
	}

	return &FileVersionControl{
		versions:          make(map[string][]FileVersion),
		snapshots:         make(map[string]*FileSnapshot),
		retentionPolicies: make(map[string]*RetentionPolicy),
		config:            config,
	}
}

// CreateVersion 创建文件版本
func (fvc *FileVersionControl) CreateVersion(filePath string, size int64, content []byte, comment string, createdBy string) (*FileVersion, error) {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	// 计算校验和
	checksum := fvc.calculateChecksum(content)

	// 检查是否与上一版本相同
	versions := fvc.versions[filePath]
	if len(versions) > 0 {
		lastVersion := versions[len(versions)-1]
		if lastVersion.Checksum == checksum {
			// 内容未变化，不创建新版本
			return &lastVersion, nil
		}
	}

	// 确定版本号
	versionNum := 1
	if len(versions) > 0 {
		versionNum = versions[len(versions)-1].Version + 1
	}

	// 创建版本记录
	version := FileVersion{
		ID:           fmt.Sprintf("ver_%s_%d_%d", filePath, versionNum, time.Now().UnixNano()),
		FilePath:     filePath,
		Version:      versionNum,
		Size:         size,
		Checksum:     checksum,
		Status:       StatusCurrent,
		SnapshotType: SnapshotManual,
		Comment:      comment,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
		IsCompressed: fvc.config.CompressionEnabled,
	}

	// 如果启用压缩，计算压缩后大小
	if fvc.config.CompressionEnabled {
		version.CompressedSize = int64(float64(size) * 0.7) // 假设30%压缩率
	}

	// 将之前的版本标记为历史
	for i := range versions {
		versions[i].Status = StatusPrevious
	}

	// 添加新版本
	fvc.versions[filePath] = append(versions, version)

	// 应用保留策略
	fvc.applyRetentionPolicy(filePath)

	return &version, nil
}

// GetVersion 获取指定版本
func (fvc *FileVersionControl) GetVersion(filePath string, version int) (*FileVersion, error) {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("no versions found for file: %s", filePath)
	}

	for _, v := range versions {
		if v.Version == version {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("version %d not found for file: %s", version, filePath)
}

// GetLatestVersion 获取最新版本
func (fvc *FileVersionControl) GetLatestVersion(filePath string) (*FileVersion, error) {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	versions, exists := fvc.versions[filePath]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for file: %s", filePath)
	}

	return &versions[len(versions)-1], nil
}

// GetVersionHistory 获取版本历史
func (fvc *FileVersionControl) GetVersionHistory(filePath string, limit int) []FileVersion {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return nil
	}

	// 按版本号降序排序
	sortedVersions := make([]FileVersion, len(versions))
	copy(sortedVersions, versions)
	sort.Slice(sortedVersions, func(i, j int) bool {
		return sortedVersions[i].Version > sortedVersions[j].Version
	})

	if limit > 0 && limit < len(sortedVersions) {
		sortedVersions = sortedVersions[:limit]
	}

	return sortedVersions
}

// CreateSnapshot 创建快照
func (fvc *FileVersionControl) CreateSnapshot(name string, description string, filePaths []string, createdBy string) (*FileSnapshot, error) {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	snapshot := &FileSnapshot{
		ID:            fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		Name:          name,
		Description:   description,
		Type:          SnapshotManual,
		FilePaths:     filePaths,
		Versions:      make(map[string]int),
		CreatedBy:     createdBy,
		CreatedAt:     time.Now(),
		RetentionDays: 30,
	}

	// 记录每个文件的当前版本
	totalSize := int64(0)
	for _, filePath := range filePaths {
		versions, exists := fvc.versions[filePath]
		if exists && len(versions) > 0 {
			latest := versions[len(versions)-1]
			snapshot.Versions[filePath] = latest.Version
			totalSize += latest.Size

			// 标记版本为快照版本
			versions[len(versions)-1].SnapshotID = snapshot.ID
			versions[len(versions)-1].SnapshotType = SnapshotManual
		}
	}

	snapshot.TotalSize = totalSize
	fvc.snapshots[snapshot.ID] = snapshot

	return snapshot, nil
}

// RollbackToVersion 回滚到指定版本
func (fvc *FileVersionControl) RollbackToVersion(filePath string, version int) (*FileVersion, error) {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("no versions found for file: %s", filePath)
	}

	// 查找目标版本
	var targetVersion *FileVersion
	for i, v := range versions {
		if v.Version == version {
			targetVersion = &versions[i]
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version %d not found for file: %s", version, filePath)
	}

	// 创建新版本（基于回滚版本）
	newVersion := FileVersion{
		ID:             fmt.Sprintf("ver_%s_%d_%d", filePath, len(versions)+1, time.Now().UnixNano()),
		FilePath:       filePath,
		Version:        len(versions) + 1,
		Size:           targetVersion.Size,
		Checksum:       targetVersion.Checksum,
		Status:         StatusCurrent,
		SnapshotType:   SnapshotManual,
		Comment:        fmt.Sprintf("回滚到版本 %d", version),
		CreatedBy:      "system",
		CreatedAt:      time.Now(),
		IsCompressed:   targetVersion.IsCompressed,
		CompressedSize: targetVersion.CompressedSize,
	}

	// 将当前版本标记为历史
	for i := range versions {
		versions[i].Status = StatusPrevious
	}

	// 添加新版本
	fvc.versions[filePath] = append(versions, newVersion)

	return &newVersion, nil
}

// CompareVersions 比较两个版本
func (fvc *FileVersionControl) CompareVersions(filePath string, version1, version2 int) (*VersionDiff, error) {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return nil, fmt.Errorf("no versions found for file: %s", filePath)
	}

	var v1, v2 *FileVersion
	for i, v := range versions {
		if v.Version == version1 {
			v1 = &versions[i]
		}
		if v.Version == version2 {
			v2 = &versions[i]
		}
	}

	if v1 == nil {
		return nil, fmt.Errorf("version %d not found", version1)
	}
	if v2 == nil {
		return nil, fmt.Errorf("version %d not found", version2)
	}

	diff := &VersionDiff{
		FilePath:      filePath,
		Version1:      version1,
		Version2:      version2,
		SizeDiff:      v2.Size - v1.Size,
		ChecksumMatch: v1.Checksum == v2.Checksum,
		ComparedAt:    time.Now(),
	}

	// 如果校验和不同，生成变更记录
	if !diff.ChecksumMatch {
		diff.Changes = []Change{
			{Type: "modify", OldValue: v1.Checksum, NewValue: v2.Checksum},
		}
	}

	return diff, nil
}

// DeleteVersion 删除版本
func (fvc *FileVersionControl) DeleteVersion(filePath string, version int) error {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions found for file: %s", filePath)
	}

	for i, v := range versions {
		if v.Version == version {
			// 标记为已删除
			fvc.versions[filePath][i].Status = StatusDeleted
			return nil
		}
	}

	return fmt.Errorf("version %d not found for file: %s", version, filePath)
}

// RestoreDeletedVersion 恢复已删除版本
func (fvc *FileVersionControl) RestoreDeletedVersion(filePath string, version int) error {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return fmt.Errorf("no versions found for file: %s", filePath)
	}

	for i, v := range versions {
		if v.Version == version && v.Status == StatusDeleted {
			fvc.versions[filePath][i].Status = StatusPrevious
			return nil
		}
	}

	return fmt.Errorf("deleted version %d not found for file: %s", version, filePath)
}

// PurgeOldVersions 清理旧版本
func (fvc *FileVersionControl) PurgeOldVersions(filePath string, keepCount int) (int, error) {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	versions, exists := fvc.versions[filePath]
	if !exists {
		return 0, nil
	}

	if len(versions) <= keepCount {
		return 0, nil
	}

	// 按版本号排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})

	// 删除多余的版本
	deleteCount := len(versions) - keepCount
	fvc.versions[filePath] = versions[deleteCount:]

	return deleteCount, nil
}

// GetStats 获取统计信息
func (fvc *FileVersionControl) GetStats() *VersionStats {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	stats := &VersionStats{
		VersionsByStatus: make(map[VersionStatus]int),
	}

	totalVersions := 0
	totalSize := int64(0)
	compressedSize := int64(0)

	for _, versions := range fvc.versions {
		stats.TotalFiles++
		totalVersions += len(versions)

		for _, v := range versions {
			totalSize += v.Size
			compressedSize += v.CompressedSize
			stats.VersionsByStatus[v.Status]++

			if stats.OldestVersion == nil || v.CreatedAt.Before(*stats.OldestVersion) {
				oldest := v.CreatedAt
				stats.OldestVersion = &oldest
			}
			if stats.NewestVersion == nil || v.CreatedAt.After(*stats.NewestVersion) {
				newest := v.CreatedAt
				stats.NewestVersion = &newest
			}
		}
	}

	stats.TotalVersions = totalVersions
	stats.TotalSnapshots = len(fvc.snapshots)
	stats.TotalSize = totalSize
	stats.CompressedSize = compressedSize
	stats.SpaceSaved = totalSize - compressedSize

	if stats.TotalFiles > 0 {
		stats.AverageVersions = float64(totalVersions) / float64(stats.TotalFiles)
	}

	return stats
}

// SetRetentionPolicy 设置保留策略
func (fvc *FileVersionControl) SetRetentionPolicy(policy *RetentionPolicy) error {
	fvc.mu.Lock()
	defer fvc.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("retention policy ID is required")
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	fvc.retentionPolicies[policy.ID] = policy
	return nil
}

// MarshalJSON 序列化
func (fvc *FileVersionControl) MarshalJSON() ([]byte, error) {
	fvc.mu.RLock()
	defer fvc.mu.RUnlock()

	return json.Marshal(struct {
		Versions          map[string][]FileVersion    `json:"versions"`
		Snapshots         map[string]*FileSnapshot    `json:"snapshots"`
		RetentionPolicies map[string]*RetentionPolicy `json:"retention_policies"`
		Config            *VersionControlConfig       `json:"config"`
	}{
		Versions:          fvc.versions,
		Snapshots:         fvc.snapshots,
		RetentionPolicies: fvc.retentionPolicies,
		Config:            fvc.config,
	})
}

// 内部方法

func (fvc *FileVersionControl) calculateChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func (fvc *FileVersionControl) applyRetentionPolicy(filePath string) {
	versions := fvc.versions[filePath]
	if len(versions) == 0 {
		return
	}

	maxVersions := fvc.config.MaxVersionsPerFile
	if maxVersions > 0 && len(versions) > maxVersions {
		// 保留最新版本，删除最旧的
		excess := len(versions) - maxVersions
		fvc.versions[filePath] = versions[excess:]
	}
}

// GenerateDefaultRetentionPolicy 生成默认保留策略
func GenerateDefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		ID:                "default",
		Name:              "默认保留策略",
		Description:       "保留最近100个版本或30天内的版本",
		MaxVersions:       100,
		MaxAgeDays:        30,
		KeepDaily:         7,
		KeepWeekly:        4,
		KeepMonthly:       12,
		KeepYearly:        3,
		MinVersionsToKeep: 5,
		Enabled:           true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}
