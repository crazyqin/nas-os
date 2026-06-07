// Package drivesync 提供版本控制管理器
package drivesync

import (
	"fmt"
	"sync"
	"time"
)

// VersionManager 版本控制管理器.
type VersionManager struct {
	mu        sync.RWMutex
	config    VersionConfig
	versions  map[string][]*FileVersion // filePath -> []FileVersion
	totalSize int64                     // 当前版本存储总大小
}

// NewVersionManager 创建版本控制管理器.
func NewVersionManager(config VersionConfig) *VersionManager {
	// 设置默认值
	if config.RetentionDays <= 0 {
		config.RetentionDays = 30
	}
	if config.MaxVersions <= 0 {
		config.MaxVersions = 100
	}

	return &VersionManager{
		config:   config,
		versions: make(map[string][]*FileVersion),
	}
}

// CreateVersion 创建新版本.
func (vm *VersionManager) CreateVersion(filePath string, size int64, checksum string, createdBy string) (*FileVersion, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.config.Enabled {
		return nil, fmt.Errorf("版本控制未启用")
	}

	versions := vm.versions[filePath]
	versionNum := len(versions) + 1

	// 检查是否超过最大版本数
	if len(versions) >= vm.config.MaxVersions {
		// 删除最旧的版本
		oldest := versions[0]
		vm.totalSize -= oldest.Size
		versions = versions[1:]
	}

	// 检查存储大小限制
	if vm.config.MaxTotalSize > 0 && vm.totalSize+size > vm.config.MaxTotalSize {
		return nil, fmt.Errorf("版本存储空间不足，当前 %d 字节，需要 %d 字节", vm.totalSize, size)
	}

	expiresAt := time.Now().AddDate(0, 0, vm.config.RetentionDays)

	version := &FileVersion{
		ID:          generateID(),
		FilePath:    filePath,
		VersionNum:  versionNum,
		Size:        size,
		Checksum:    checksum,
		StoragePath: fmt.Sprintf(".versions/%s/v%d", filePath, versionNum),
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		ExpiresAt:   &expiresAt,
	}

	versions = append(versions, version)
	vm.versions[filePath] = versions
	vm.totalSize += size

	return version, nil
}

// GetVersions 获取文件的所有版本.
func (vm *VersionManager) GetVersions(filePath string) []*FileVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := vm.versions[filePath]
	if versions == nil {
		return make([]*FileVersion, 0)
	}
	return versions
}

// GetVersion 获取指定版本.
func (vm *VersionManager) GetVersion(filePath string, versionID string) (*FileVersion, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := vm.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			return v, nil
		}
	}

	return nil, ErrFileVersionNotFound
}

// CleanupExpired 清理过期版本.
func (vm *VersionManager) CleanupExpired() int {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for filePath, versions := range vm.versions {
		var valid []*FileVersion
		for _, v := range versions {
			if v.ExpiresAt != nil && v.ExpiresAt.Before(now) {
				vm.totalSize -= v.Size
				cleaned++
			} else {
				valid = append(valid, v)
			}
		}
		vm.versions[filePath] = valid
	}

	return cleaned
}

// GetTotalSize 获取版本存储总大小.
func (vm *VersionManager) GetTotalSize() int64 {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.totalSize
}

// GetConfig 获取版本控制配置.
func (vm *VersionManager) GetConfig() VersionConfig {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.config
}

// SetLabel 设置版本标签.
func (vm *VersionManager) SetLabel(filePath, versionID, label string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions := vm.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			v.Label = label
			return nil
		}
	}
	return ErrFileVersionNotFound
}

// SetComment 设置版本注释.
func (vm *VersionManager) SetComment(filePath, versionID, comment string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions := vm.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			v.Comment = comment
			return nil
		}
	}
	return ErrFileVersionNotFound
}
