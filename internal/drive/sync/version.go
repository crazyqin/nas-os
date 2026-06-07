package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// VersionManager 管理系统文件版本历史.
// 参考 Synology Drive Intelliversioning 算法.
type VersionManager struct {
	mu          sync.RWMutex
	config      VersionConfig
	maxVersions int
	versions    map[string][]*FileVersion // path -> versions
}

// FileVersion 单个文件版本.
type FileVersion struct {
	Version    int       `json:"version"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	ModifiedBy string    `json:"modified_by"`
	Label      string    `json:"label,omitempty"`
	LocalPath  string    `json:"local_path,omitempty"`  // 本地路径（本地版本）
	RemotePath string    `json:"remote_path,omitempty"` // 远端路径（远端版本）
}

// NewVersionManager 创建版本管理器.
func NewVersionManager(config VersionConfig) *VersionManager {
	maxVersions := config.MaxVersions
	if maxVersions <= 0 {
		maxVersions = 32 // 默认值，参考 Synology
	}
	return &VersionManager{
		config:      config,
		maxVersions: maxVersions,
		versions:    make(map[string][]*FileVersion),
	}
}

// SaveRemoteVersion 保存远端文件的版本.
func (vm *VersionManager) SaveRemoteVersion(ctx context.Context, remote RemoteStorage, entry *FileEntry) error {
	if !vm.config.Enabled {
		return nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions := vm.versions[entry.Path]
	newVersion := &FileVersion{
		Version:    len(versions) + 1,
		Hash:       entry.Checksum,
		Size:       entry.Size,
		ModifiedAt: entry.ModTime,
		RemotePath: entry.Path,
	}

	// 保存版本到远端存储
	versionPath := vm.remoteVersionPath(entry.Path, newVersion.Version)
	if err := remote.Put(ctx, entry.Path, versionPath); err != nil {
		slog.Warn("failed to save remote version", "path", entry.Path, "error", err)
		return fmt.Errorf("save version: %w", err)
	}

	versions = append(versions, newVersion)

	// 强制最大版本数
	if len(versions) > vm.maxVersions {
		versions = vm.intelliversion(versions)
	}

	vm.versions[entry.Path] = versions
	slog.Debug("saved remote version", "path", entry.Path, "version", newVersion.Version)
	return nil
}

// SaveLocalVersion 保存本地文件的版本.
func (vm *VersionManager) SaveLocalVersion(entry *FileEntry, localPath string) error {
	if !vm.config.Enabled {
		return nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions := vm.versions[entry.Path]
	newVersion := &FileVersion{
		Version:    len(versions) + 1,
		Hash:       entry.Checksum,
		Size:       entry.Size,
		ModifiedAt: entry.ModTime,
		LocalPath:  entry.Path,
	}

	// 保存到本地版本目录
	srcPath := filepath.Join(localPath, entry.Path)
	versionDir := filepath.Join(localPath, ".sync_versions", entry.Path)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}

	versionFile := filepath.Join(versionDir, fmt.Sprintf("v%d", newVersion.Version))
	if err := copyFile(srcPath, versionFile); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}

	versions = append(versions, newVersion)

	if len(versions) > vm.maxVersions {
		versions = vm.intelliversion(versions)
	}

	vm.versions[entry.Path] = versions
	slog.Debug("saved local version", "path", entry.Path, "version", newVersion.Version)
	return nil
}

// GetVersions 返回文件所有版本.
func (vm *VersionManager) GetVersions(path string) []*FileVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions, ok := vm.versions[path]
	if !ok {
		return nil
	}

	// 返回副本，最新版本在前
	result := make([]*FileVersion, len(versions))
	copy(result, versions)
	return result
}

// GetLatestVersion 返回文件的最新版本.
func (vm *VersionManager) GetLatestVersion(path string) *FileVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions, ok := vm.versions[path]
	if !ok || len(versions) == 0 {
		return nil
	}
	return versions[len(versions)-1]
}

// RestoreVersion 恢复文件到指定版本.
func (vm *VersionManager) RestoreVersion(path string, version int) (*FileVersion, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions, ok := vm.versions[path]
	if !ok {
		return nil, fmt.Errorf("no versions found for %s", path)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version %d not found for %s", version, path)
}

// GetVersionCount 返回文件版本数量.
func (vm *VersionManager) GetVersionCount(path string) int {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return len(vm.versions[path])
}

// remoteVersionPath 生成远端版本路径.
func (vm *VersionManager) remoteVersionPath(path string, version int) string {
	ext := filepath.Ext(path)
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	return filepath.Join(dir, fmt.Sprintf(".sync_versions/%s_v%d%s", base, version, ext))
}

// intelliversion 实现类 Synology Intelliversioning 算法.
// 保留时间跨度最大的版本（代表最大变更）。
func (vm *VersionManager) intelliversion(versions []*FileVersion) []*FileVersion {
	if len(versions) <= vm.maxVersions {
		return versions
	}

	// 始终保留最新版本
	kept := []*FileVersion{versions[len(versions)-1]}

	// 计算相邻版本间的时间间隔
	type gapInfo struct {
		index int
		gap   time.Duration
	}
	gaps := make([]gapInfo, 0, len(versions)-1)
	for i := 0; i < len(versions)-1; i++ {
		gap := versions[i+1].ModifiedAt.Sub(versions[i].ModifiedAt)
		gaps = append(gaps, gapInfo{index: i, gap: gap})
	}

	// 冒泡排序：最大间隔优先
	for i := 0; i < len(gaps)-1; i++ {
		for j := i + 1; j < len(gaps); j++ {
			if gaps[j].gap > gaps[i].gap {
				gaps[i], gaps[j] = gaps[j], gaps[i]
			}
		}
	}

	// 保留最大间隔边界的版本
	keepSet := make(map[int]bool)
	for i := 0; i < len(gaps) && len(keepSet) < vm.maxVersions-1; i++ {
		keepSet[gaps[i].index] = true
	}

	for idx := range keepSet {
		kept = append(kept, versions[idx])
	}

	return kept
}

// copyFile 复制文件（与sync.go保持一致，避免循环import）.
func copyFile(src, dst string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer to.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := from.Read(buf)
		if n > 0 {
			if _, werr := to.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			break
		}
	}
	return to.Sync()
}
