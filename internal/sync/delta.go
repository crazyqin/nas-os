package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SnapshotScanner 扫描本地目录生成快照.
type SnapshotScanner struct {
	checksum bool
	skipHidden bool
	maxSize int64
	excludePatterns []string
}

// NewSnapshotScanner 创建快照扫描器.
func NewSnapshotScanner() *SnapshotScanner {
	return &SnapshotScanner{
		checksum:   true,
		skipHidden: true,
	}
}

// SetChecksum 设置是否计算校验和.
func (s *SnapshotScanner) SetChecksum(enable bool) *SnapshotScanner {
	s.checksum = enable
	return s
}

// SetMaxSize 设置最大文件大小（字节，0=不限制）.
func (s *SnapshotScanner) SetMaxSize(max int64) *SnapshotScanner {
	s.maxSize = max
	return s
}

// SetExcludePatterns 设置排除模式（glob）.
func (s *SnapshotScanner) SetExcludePatterns(patterns []string) *SnapshotScanner {
	s.excludePatterns = patterns
	return s
}

// Scan 扫描目录生成快照.
func (s *SnapshotScanner) Scan(rootPath string, rev int64) (*Snapshot, error) {
	ss := &Snapshot{
		Rev:     rev,
		RootPath: rootPath,
		Entries: make(map[string]*FileEntry),
		Mtime:   time.Now(),
	}

	err := filepath.Walk(rootPath, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}

		relPath, err := filepath.Rel(rootPath, fullPath)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		// 排除目录本身
		if info.IsDir() {
			return nil
		}

		// 排除隐藏文件
		if s.skipHidden && s.isHidden(relPath) {
			return nil
		}

		// 排除模式
		if s.matchExclude(relPath) {
			return nil
		}

		// 文件大小限制
		if s.maxSize > 0 && info.Size() > s.maxSize {
			return nil
		}

		entry := &FileEntry{
			Path:     relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			IsDir:    false,
			LastSyncedAt: time.Time{},
		}

		// 计算校验和
		if s.checksum && info.Size() > 0 {
			cs := NewChecksummer()
			h, err := cs.FileChecksum(fullPath)
			if err == nil {
				entry.Checksum = h
			}
		}

		ss.Entries[relPath] = entry
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scan root %s: %w", rootPath, err)
	}

	return ss, nil
}

// ScanRemote 扫描远程文件列表生成快照（由上层 Provider 注入）.
func (s *SnapshotScanner) ScanRemote(remoteFiles []FileEntry, rootPath string, rev int64) *Snapshot {
	ss := &Snapshot{
		Rev:      rev,
		RootPath: rootPath,
		Entries:  make(map[string]*FileEntry),
		Mtime:    time.Now(),
	}
	for i := range remoteFiles {
		f := &remoteFiles[i]
		if s.matchExclude(f.Path) {
			continue
		}
		if s.maxSize > 0 && f.Size > s.maxSize {
			continue
		}
		ss.Entries[f.Path] = f
	}
	return ss
}

func (s *SnapshotScanner) isHidden(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, p := range parts {
		if len(p) > 0 && p[0] == '.' {
			return true
		}
	}
	return false
}

func (s *SnapshotScanner) matchExclude(relPath string) bool {
	if len(s.excludePatterns) == 0 {
		return false
	}
	for _, pattern := range s.excludePatterns {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return true
		}
		// 目录匹配
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(relPath, dir+"/") || relPath == dir {
				return true
			}
		}
	}
	return false
}

// ComputeDelta 计算两个快照之间的差异.
// oldSnapshot 为上一次状态，newSnapshot 为当前状态.
func ComputeDelta(oldSnap, newSnap *Snapshot) *Delta {
	delta := &Delta{}

	if oldSnap == nil || oldSnap.Entries == nil {
		// 首次同步：所有文件视为新增
		return &Delta{Adds: snapToDeltaAdds(newSnap)}
	}

	// 遍历新快照中的所有文件
	for path, newEntry := range newSnap.Entries {
		oldEntry, existed := oldSnap.Entries[path]
		if !existed {
			// 新增
			delta.Adds = append(delta.Adds, &DeltaItem{
				RelPath:    path,
				NewEntry:   newEntry,
				ChangeType: ChangeCreate,
			})
		} else if entryChanged(oldEntry, newEntry) {
			// 修改
			delta.Mods = append(delta.Mods, &DeltaItem{
				RelPath:    path,
				OldEntry:   oldEntry,
				NewEntry:   newEntry,
				ChangeType: ChangeModify,
			})
		}
	}

	// 遍历旧快照中已删除的文件
	for path, oldEntry := range oldSnap.Entries {
		if _, stillExists := newSnap.Entries[path]; !stillExists {
			delta.Dels = append(delta.Dels, &DeltaItem{
				RelPath:    path,
				OldEntry:   oldEntry,
				ChangeType: ChangeDelete,
			})
		}
	}

	// 检测重命名（同名文件在不同路径，大小+校验和相同）——在删除+新增中寻找
	sort.Slice(delta.Dels, func(i, j int) bool {
		return delta.Dels[i].OldEntry.Size < delta.Dels[j].OldEntry.Size
	})
	sort.Slice(delta.Adds, func(i, j int) bool {
		return delta.Adds[i].NewEntry.Size < delta.Adds[j].NewEntry.Size
	})

	var remainingDels []*DeltaItem
	var remainingAdds []*DeltaItem

	for _, del := range delta.Dels {
		matched := false
		for i, add := range delta.Adds {
			if canBeRename(del.OldEntry, add.NewEntry) {
				delta.Renames = append(delta.Renames, &RenameItem{
					SrcPath:  del.RelPath,
					DstPath:  add.RelPath,
					SrcEntry: del.OldEntry,
					DstEntry: add.NewEntry,
					Score:    renameScore(del.OldEntry, add.NewEntry),
				})
				delta.Adds = append(delta.Adds[:i], delta.Adds[i+1:]...)
				matched = true
				break
			}
		}
		if !matched {
			remainingDels = append(remainingDels, del)
		}
	}
	delta.Dels = remainingDels

	_ = remainingAdds // adds not matched are already handled by removal above

	return delta
}

// entryChanged 判断两个条目是否有实质变化.
func entryChanged(a, b *FileEntry) bool {
	if a.Size != b.Size {
		return true
	}
	// mtime 有 1 秒容差
	if !sameSecond(a.ModTime, b.ModTime) {
		return true
	}
	// 如果都计算了校验和，比较校验和
	if a.Checksum != "" && b.Checksum != "" {
		return a.Checksum != b.Checksum
	}
	return false
}

// canBeRename 判断是否可能是重命名（同名 inode 或 内容相同）.
func canBeRename(old, new *FileEntry) bool {
	// 大小必须相同
	if old.Size != new.Size {
		return false
	}
	// 如果有校验和，必须相同
	if old.Checksum != "" && new.Checksum != "" {
		return old.Checksum == new.Checksum
	}
	// 大小为 0 的空文件，假设可重命名
	return old.Size == 0
}

// renameScore 计算重命名置信度（0-1）.
func renameScore(old, new *FileEntry) float64 {
	if old.Size != new.Size {
		return 0
	}
	if old.Checksum != "" && old.Checksum == new.Checksum {
		return 1.0
	}
	return 0.5 // 无法确认内容，仅凭大小
}

// sameSecond 判断两个时间是否在同一秒内.
func sameSecond(a, b time.Time) bool {
	return a.Unix() == b.Unix()
}

// DeltaStats delta 统计摘要.
func (d *Delta) Stats() (adds, mods, dels, renames int) {
	return len(d.Adds), len(d.Mods), len(d.Dels), len(d.Renames)
}

// TotalFilesDelta delta 涉及的文件总数.
func (d *Delta) TotalFilesDelta() int64 {
	return int64(len(d.Adds) + len(d.Mods) + len(d.Dels))
}

// TotalBytesDelta delta 涉及的总字节数（估算）.
func (d *Delta) TotalBytesDelta() int64 {
	var n int64
	for _, item := range d.Adds {
		n += item.NewEntry.Size
	}
	for _, item := range d.Mods {
		n += item.NewEntry.Size - item.OldEntry.Size
	}
	return n
}
