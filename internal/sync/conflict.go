package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConflictDetector 冲突检测器.
// 判断本地和远程同时发生变化的文件，并记录冲突信息.
type ConflictDetector struct {
	strategy ConflictStrategy
	changes  map[string]*Conflict // relPath -> conflict
}

// NewConflictDetector 创建冲突检测器.
func NewConflictDetector(strategy ConflictStrategy) *ConflictDetector {
	return &ConflictDetector{
		strategy: strategy,
		changes:  make(map[string]*Conflict),
	}
}

// DetectAll 检测所有冲突.
// 输入：本地 delta、远程 delta、上次同步时的文件状态.
func (d *ConflictDetector) DetectAll(
	taskID string,
	localDelta, remoteDelta *Delta,
	fileStates map[string]*FileState,
) []*Conflict {
	var conflicts []*Conflict
	now := time.Now()

	// 收集两边同时修改的路径
	localChanged := deltaPaths(localDelta)
	remoteChanged := deltaPaths(remoteDelta)

	for path := range localChanged {
		if _, remoteAlso := remoteChanged[path]; remoteAlso {
			// 两端同时修改了同一文件 → 冲突
			conflict := &Conflict{
				ID:         fmt.Sprintf("conflict-%d-%s", now.UnixNano(), shortPath(path)),
				TaskID:     taskID,
				RelPath:    path,
				Strategy:   d.strategy,
				DetectedAt: now,
			}

			// 从 delta 中提取信息
			if item, ok := localChanged[path]; ok && item.NewEntry != nil {
				conflict.LocalModTime = item.NewEntry.ModTime
				conflict.LocalSize = item.NewEntry.Size
				conflict.LocalChecksum = item.NewEntry.Checksum
			}
			if item, ok := remoteChanged[path]; ok && item.NewEntry != nil {
				conflict.RemoteModTime = item.NewEntry.ModTime
				conflict.RemoteSize = item.NewEntry.Size
				conflict.RemoteChecksum = item.NewEntry.Checksum
			}

			d.changes[path] = conflict
			conflicts = append(conflicts, conflict)
		}
	}

	// 一端删除、另一端修改也是冲突
	for path := range localChanged {
		item := localChanged[path]
		if item.ChangeType == ChangeDelete {
			remoteMod, remoteExists := remoteChanged[path]
			if remoteExists && remoteMod != nil {
				c := &Conflict{
					ID:         fmt.Sprintf("conflict-del-%d-%s", now.UnixNano(), shortPath(path)),
					TaskID:     taskID,
					RelPath:    path,
					Strategy:   d.strategy,
					DetectedAt: now,
				}
				if remoteMod.NewEntry != nil {
					c.RemoteModTime = remoteMod.NewEntry.ModTime
					c.RemoteSize = remoteMod.NewEntry.Size
					c.RemoteChecksum = remoteMod.NewEntry.Checksum
				}
				if item.OldEntry != nil {
					c.LocalModTime = item.OldEntry.ModTime
					c.LocalSize = item.OldEntry.Size
					c.LocalChecksum = item.OldEntry.Checksum
				}
				d.changes[path] = c
				conflicts = append(conflicts, c)
			}
		}
	}
	for path := range remoteChanged {
		item := remoteChanged[path]
		if item.ChangeType == ChangeDelete {
			localMod, localExists := localChanged[path]
			if localExists && localMod != nil {
				if _, exists := d.changes[path]; !exists {
					c := &Conflict{
						ID:         fmt.Sprintf("conflict-del-%d-%s", now.UnixNano(), shortPath(path)),
						TaskID:     taskID,
						RelPath:    path,
						Strategy:   d.strategy,
						DetectedAt: now,
					}
					if localMod.NewEntry != nil {
						c.LocalModTime = localMod.NewEntry.ModTime
						c.LocalSize = localMod.NewEntry.Size
						c.LocalChecksum = localMod.NewEntry.Checksum
					}
					if item.OldEntry != nil {
						c.RemoteModTime = item.OldEntry.ModTime
						c.RemoteSize = item.OldEntry.Size
						c.RemoteChecksum = item.OldEntry.Checksum
					}
					d.changes[path] = c
					conflicts = append(conflicts, c)
				}
			}
		}
	}

	return conflicts
}

// Resolve 解决冲突，返回应采取的操作.
func (d *ConflictDetector) Resolve(c *Conflict) SyncOpType {
	now := time.Now()
	c.ResolvedAt = &now

	switch c.Strategy {
	case ConflictLocal:
		c.ResolvedPath = c.RelPath
		return SyncOpUpload

	case ConflictRemote:
		c.ResolvedPath = c.RelPath
		return SyncOpDownload

	case ConflictNewer:
		if c.LocalModTime.After(c.RemoteModTime) {
			c.ResolvedPath = c.RelPath
			return SyncOpUpload
		} else if c.RemoteModTime.After(c.LocalModTime) {
			c.ResolvedPath = c.RelPath
			return SyncOpDownload
		}
		// 时间相同，保留较大文件
		if c.LocalSize >= c.RemoteSize {
			c.ResolvedPath = c.RelPath
			return SyncOpUpload
		}
		c.ResolvedPath = c.RelPath
		return SyncOpDownload

	case ConflictRename:
		ext := filepath.Ext(c.RelPath)
		base := strings.TrimSuffix(c.RelPath, ext)
		c.ResolvedPath = fmt.Sprintf("%s_conflict_%s%s", base, now.Format("20060102_150405"), ext)
		return SyncOpRename

	case ConflictSkip:
		return SyncOpSkip

	case ConflictAsk:
		return SyncOpConflict

	default:
		return SyncOpSkip
	}
}

// ResolveRename 执行 rename 策略：将较旧版本移到冲突路径.
// 返回 (newLocalPath, newRemotePath, error).
func (d *ConflictDetector) ResolveRename(
	c *Conflict,
	localRoot, remoteRoot string,
	provider Provider,
) (string, string, error) {
	if c.Strategy != ConflictRename {
		return "", "", fmt.Errorf("conflict strategy is not rename")
	}

	ext := filepath.Ext(c.RelPath)
	base := strings.TrimSuffix(c.RelPath, ext)
	ts := time.Now().Format("20060102_150405")

	localConflicted := fmt.Sprintf("%s_local_%s%s", base, ts, ext)
	remoteConflicted := fmt.Sprintf("%s_remote_%s%s", base, ts, ext)

	// 在本地创建冲突副本：将当前本地版本重命名
	localFull := filepath.Join(localRoot, c.RelPath)
	localConflictFull := filepath.Join(localRoot, localConflicted)
	if err := os.Rename(localFull, localConflictFull); err != nil {
		return "", "", fmt.Errorf("rename local conflict: %w", err)
	}

	// 将远程版本下载到原始路径
	if err := provider.Download(context.Background(), c.RelPath, localFull); err != nil {
		// 回滚
		_ = os.Rename(localConflictFull, localFull)
		return "", "", fmt.Errorf("download remote version: %w", err)
	}

	// 在远程也上传冲突副本（本地版本）
	if err := provider.Upload(context.Background(), localConflictFull, remoteConflicted); err != nil {
		// 不回滚，本地已有两个版本
		return localConflicted, remoteConflicted, fmt.Errorf("upload local conflict copy: %w", err)
	}

	return localConflicted, remoteConflicted, nil
}

// GetConflicts 获取所有已检测冲突.
func (d *ConflictDetector) GetConflicts() []*Conflict {
	var result []*Conflict
	for _, c := range d.changes {
		result = append(result, c)
	}
	return result
}

// SyncOpType 同步操作类型.
type SyncOpType string

const (
	SyncOpUpload   SyncOpType = "upload"
	SyncOpDownload SyncOpType = "download"
	SyncOpDelete   SyncOpType = "delete"
	SyncOpRename   SyncOpType = "rename"
	SyncOpSkip     SyncOpType = "skip"
	SyncOpConflict SyncOpType = "conflict" // 需要用户介入
)

// deltaPaths 提取 delta 中所有涉及文件的路径及其对应的 DeltaItem.
func deltaPaths(d *Delta) map[string]*DeltaItem {
	m := make(map[string]*DeltaItem, len(d.Adds)+len(d.Mods)+len(d.Dels))
	for _, item := range d.Adds {
		m[item.RelPath] = item
	}
	for _, item := range d.Mods {
		m[item.RelPath] = item
	}
	for _, item := range d.Dels {
		m[item.RelPath] = item
	}
	return m
}

// shortPath 取路径最后部分用于 ID 生成.
func shortPath(p string) string {
	if len(p) > 32 {
		return p[len(p)-32:]
	}
	return p
}
