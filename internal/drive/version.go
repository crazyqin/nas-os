package drive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// VersionManager 版本管理器
// 实现 Intelliversioning 算法 - 智能保留重要版本
type VersionManager struct {
	mu         sync.RWMutex
	maxVersions int
	versionDir string
}

// FileVersion 文件版本
type FileVersion struct {
	Version     int       `json:"version"`
	Path        string    `json:"path"`
	Hash        string    `json:"hash"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	CreatedAt   time.Time `json:"createdAt"`
	Importance  float64   `json:"importance"` // 版本重要性评分
	ChangeType  string    `json:"changeType"` // major, minor, patch
}

// NewVersionManager 创建版本管理器
func NewVersionManager(maxVersions int) *VersionManager {
	return &VersionManager{
		maxVersions: maxVersions,
		versionDir:  ".versions",
	}
}

// CreateVersion 创建新版本
func (v *VersionManager) CreateVersion(path, hash string, size int64, modTime time.Time) (*FileVersion, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 获取现有版本
	versions, err := v.ListVersions(path)
	if err != nil {
		versions = []FileVersion{}
	}

	// 计算新版本号
	newVersion := 1
	if len(versions) > 0 {
		newVersion = versions[0].Version + 1
	}

	// 检测变更类型
	changeType := v.detectChangeType(versions, hash, size)

	// 计算重要性评分
	importance := v.calculateImportance(versions, changeType, modTime)

	version := &FileVersion{
		Version:    newVersion,
		Path:       path,
		Hash:       hash,
		Size:       size,
		ModTime:    modTime,
		CreatedAt:  time.Now(),
		Importance: importance,
		ChangeType: changeType,
	}

	// 保存版本元数据
	if err := v.saveVersionMeta(version); err != nil {
		return nil, err
	}

	return version, nil
}

// GetVersion 获取指定版本
func (v *VersionManager) GetVersion(path string, versionNum int) ([]byte, error) {
	versionPath := v.getVersionPath(path, versionNum)
	return os.ReadFile(versionPath)
}

// ListVersions 列出所有版本
func (v *VersionManager) ListVersions(path string) ([]FileVersion, error) {
	metaPath := v.getMetaPath(path)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileVersion{}, nil
		}
		return nil, err
	}

	var versions []FileVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, err
	}

	// 按版本号降序排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// CleanupOldVersions 清理旧版本 (Intelliversioning 算法)
func (v *VersionManager) CleanupOldVersions() error {
	// 遍历所有版本目录
	return filepath.Walk(".versions", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// 读取版本列表
		versions, err := v.ListVersions(path)
		if err != nil {
			return nil
		}

		// 如果超过最大版本数，删除不重要的版本
		if len(versions) > v.maxVersions {
			v.pruneVersions(path, versions)
		}

		return nil
	})
}

// pruneVersion 按重要性修剪版本
func (v *VersionManager) pruneVersions(path string, versions []FileVersion) {
	// 按重要性排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Importance > versions[j].Importance
	})

	// 保留重要的版本
	keep := versions[:v.maxVersions]
	delete := versions[v.maxVersions:]

	// 删除不重要的版本
	for _, ver := range delete {
		versionPath := v.getVersionPath(ver.Path, ver.Version)
		os.Remove(versionPath)
	}

	// 更新元数据
	if len(keep) > 0 {
		v.saveVersionsMeta(keep[0].Path, keep)
	}
}

// detectChangeType 检测变更类型
func (v *VersionManager) detectChangeType(versions []FileVersion, newHash string, newSize int64) string {
	if len(versions) == 0 {
		return "major" // 新文件
	}

	lastVer := versions[0]

	// 大小变化超过 50% 视为 major
	sizeChange := float64(newSize-lastVer.Size) / float64(lastVer.Size)
	if sizeChange > 0.5 || sizeChange < -0.5 {
		return "major"
	}

	// 哈希完全不同视为 major
	if lastVer.Hash != newHash {
		return "minor"
	}

	return "patch"
}

// calculateImportance 计算版本重要性
func (v *VersionManager) calculateImportance(versions []FileVersion, changeType string, modTime time.Time) float64 {
	base := 1.0

	// 变更类型权重
	switch changeType {
	case "major":
		base *= 3.0
	case "minor":
		base *= 2.0
	case "patch":
		base *= 1.0
	}

	// 时间衰减 - 越近的版本越重要
	hoursSinceMod := time.Since(modTime).Hours()
	if hoursSinceMod < 24 {
		base *= 2.0 // 24小时内权重加倍
	} else if hoursSinceMod < 168 {
		base *= 1.5 // 一周内权重 1.5 倍
	}

	// 版本间隔 - 如果距离上一个版本时间很长，重要性更高
	if len(versions) > 0 {
		lastMod := versions[0].ModTime
		interval := modTime.Sub(lastMod).Hours()
		if interval > 168 { // 超过一周
			base *= 1.5
		}
	}

	return base
}

// getVersionPath 获取版本文件路径
func (v *VersionManager) getVersionPath(path string, version int) string {
	return filepath.Join(v.versionDir, path, fmt.Sprintf("v%d", version))
}

// getMetaPath 获取元数据路径
func (v *VersionManager) getMetaPath(path string) string {
	return filepath.Join(v.versionDir, path, "meta.json")
}

// saveVersionMeta 保存版本元数据
func (v *VersionManager) saveVersionMeta(version *FileVersion) error {
	versions, _ := v.ListVersions(version.Path)
	versions = append([]FileVersion{*version}, versions...)
	return v.saveVersionsMeta(version.Path, versions)
}

// saveVersionsMeta 保存版本列表元数据
func (v *VersionManager) saveVersionsMeta(path string, versions []FileVersion) error {
	metaPath := v.getMetaPath(path)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}