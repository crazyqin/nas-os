package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	stateFileName     = "sync_state.json"
	versionsDirName   = "versions"
	maxVersionDefault = 5
)

// StateStore 同步状态持久化存储.
type StateStore struct {
	mu      sync.RWMutex
	baseDir string
	states  map[string]*TaskState // taskID -> state
}

// TaskState 单个同步任务的持久化状态.
type TaskState struct {
	TaskID         string                `json:"taskId"`
	LastSyncRev    int64                 `json:"lastSyncRev"`
	LastSyncTime   time.Time             `json:"lastSyncTime"`
	LocalSnapshot  *Snapshot             `json:"localSnapshot,omitempty"`
	RemoteSnapshot *Snapshot             `json:"remoteSnapshot,omitempty"`
	FileStates     map[string]*FileState `json:"fileStates"` // relPath -> state
}

// FileState 文件级别的同步状态.
type FileState struct {
	RelPath     string    `json:"relPath"`
	LocalRev    int64     `json:"localRev"`
	RemoteRev   int64     `json:"remoteRev"`
	LastSyncRev int64     `json:"lastSyncRev"`
	LocalMtime  time.Time `json:"localMtime"`
	RemoteMtime time.Time `json:"remoteMtime"`
	LocalCS     string    `json:"localCs"`
	RemoteCS    string    `json:"remoteCs"`
	LocalSize   int64     `json:"localSize"`
	RemoteSize  int64     `json:"remoteSize"`
}

// NewStateStore 创建状态存储.
func NewStateStore(baseDir string) *StateStore {
	return &StateStore{
		baseDir: baseDir,
		states:  make(map[string]*TaskState),
	}
}

// Load 加载所有任务状态.
func (s *StateStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 首次运行，无状态文件
		}
		return fmt.Errorf("read state dir %s: %w", s.baseDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		statePath := filepath.Join(s.baseDir, e.Name(), stateFileName)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var ts TaskState
		if err := json.Unmarshal(data, &ts); err != nil {
			continue
		}
		s.states[ts.TaskID] = &ts
	}
	return nil
}

// Save 持久化所有任务状态.
func (s *StateStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ts := range s.states {
		if err := s.saveTaskState(ts); err != nil {
			return err
		}
	}
	return nil
}

// SaveTask 保存单个任务状态.
func (s *StateStore) SaveTask(taskID string) error {
	s.mu.RLock()
	ts, ok := s.states[taskID]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return s.saveTaskState(ts)
}

func (s *StateStore) saveTaskState(ts *TaskState) error {
	dir := filepath.Join(s.baseDir, ts.TaskID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	data, err := json.Marshal(ts)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stateFileName), data, 0600)
}

// GetTaskState 获取任务状态.
func (s *StateStore) GetTaskState(taskID string) *TaskState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[taskID]
}

// GetOrCreateTaskState 获取或创建任务状态.
func (s *StateStore) GetOrCreateTaskState(taskID string) *TaskState {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts, ok := s.states[taskID]
	if !ok {
		ts = &TaskState{
			TaskID:     taskID,
			FileStates: make(map[string]*FileState),
		}
		s.states[taskID] = ts
	}
	return ts
}

// UpdateFileState 更新文件级状态.
func (s *StateStore) UpdateFileState(taskID string, fs *FileState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.GetOrCreateTaskState(taskID)
	ts.FileStates[fs.RelPath] = fs
}

// GetFileState 获取文件级状态.
func (s *StateStore) GetFileState(taskID, relPath string) *FileState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ts := s.states[taskID]
	if ts == nil {
		return nil
	}
	return ts.FileStates[relPath]
}

// DeleteFileState 删除文件级状态（文件已被删除时调用）.
func (s *StateStore) DeleteFileState(taskID, relPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.states[taskID]
	if ts == nil {
		return
	}
	delete(ts.FileStates, relPath)
}

// UpdateSnapshots 更新快照.
func (s *StateStore) UpdateSnapshots(taskID string, local, remote *Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.GetOrCreateTaskState(taskID)
	ts.LocalSnapshot = local
	ts.RemoteSnapshot = remote
	ts.LastSyncRev++
	ts.LastSyncTime = time.Now()
}

// GetDeltaFromLastSync 根据上次快照和当前快照计算本地 delta.
func (s *StateStore) GetDeltaFromLastSync(taskID string, currentLocal *Snapshot) *Delta {
	s.mu.RLock()
	ts := s.states[taskID]
	s.mu.RUnlock()

	if ts == nil || ts.LocalSnapshot == nil {
		// 首次同步：全部视为新增
		return &Delta{
			Adds: snapToDeltaAdds(currentLocal),
		}
	}
	return ComputeDelta(ts.LocalSnapshot, currentLocal)
}

// snapToDeltaAdds 将快照中所有条目转换为新增 DeltaItem.
func snapToDeltaAdds(snap *Snapshot) []*DeltaItem {
	if snap == nil || len(snap.Entries) == 0 {
		return nil
	}
	items := make([]*DeltaItem, 0, len(snap.Entries))
	for path, entry := range snap.Entries {
		e := entry // copy
		items = append(items, &DeltaItem{
			RelPath:    path,
			NewEntry:   e,
			ChangeType: ChangeCreate,
		})
	}
	return items
}

// VersionManager 版本管理器.
type VersionManager struct {
	mu      sync.Mutex
	baseDir string
	maxKeep int
}

// NewVersionManager 创建版本管理器.
func NewVersionManager(baseDir string, maxKeep int) *VersionManager {
	if maxKeep <= 0 {
		maxKeep = maxVersionDefault
	}
	return &VersionManager{
		baseDir: baseDir,
		maxKeep: maxKeep,
	}
}

// VersionDir 返回版本存储目录.
func versionDir(baseDir, taskID string) string {
	return filepath.Join(baseDir, versionsDirName, taskID)
}

// StoreVersion 保存一个文件版本.
// srcPath 是原始文件的绝对路径.
func (v *VersionManager) StoreVersion(taskID, relPath, srcPath string, rev int64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	vdir := versionDir(v.baseDir, taskID)
	destDir := filepath.Join(vdir, filepath.Dir(relPath))
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}

	// 版本文件名: <name>_<rev>_<timestamp>
	ext := filepath.Ext(relPath)
	base := relPath[:len(relPath)-len(ext)]
	ts := time.Now().Format("20060102_150405")
	versionPath := filepath.Join(vdir, fmt.Sprintf("%s_v%d_%s%s", base, rev, ts, ext))

	// 硬链接（同文件系统可节省空间），失败则回退复制
	if err := os.Link(srcPath, versionPath); err != nil {
		// 回退：复制文件
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read source: %w", err)
		}
		if err := os.WriteFile(versionPath, data, 0600); err != nil {
			return fmt.Errorf("write version: %w", err)
		}
	}

	return nil
}

// PruneVersions 清理旧版本，保留最近 N 个.
func (v *VersionManager) PruneVersions(taskID, relPath string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	vdir := versionDir(v.baseDir, taskID)
	ext := filepath.Ext(relPath)
	// 去掉末尾扩展名（含目录部分保留）
	nameWithoutExt := relPath[:len(relPath)-len(ext)]
	// 模式：匹配 vdir/name_v*.<ext>
	pattern := filepath.Join(vdir, nameWithoutExt+"_v*"+ext)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= v.maxKeep {
		return nil
	}

	// 按修改时间排序（旧的在前）
	sort.Slice(matches, func(i, j int) bool {
		iInfo, _ := os.Stat(matches[i])
		jInfo, _ := os.Stat(matches[j])
		if iInfo == nil || jInfo == nil {
			return false
		}
		return iInfo.ModTime().Before(jInfo.ModTime())
	})

	// 删除多余的旧版本
	for i := 0; i < len(matches)-v.maxKeep; i++ {
		_ = os.Remove(matches[i])
	}
	return nil
}

// ListVersions 列出文件的所有版本.
func (v *VersionManager) ListVersions(taskID, relPath string) ([]VersionEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	vdir := versionDir(v.baseDir, taskID)
	ext := filepath.Ext(relPath)
	base := relPath[:len(relPath)-len(ext)]
	pattern := filepath.Join(vdir, base+"_v*"+ext)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var versions []VersionEntry
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		versions = append(versions, VersionEntry{
			RelPath: relPath,
			AbsPath: m,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Created: info.ModTime(),
		})
	}

	// 按时间倒序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Created.After(versions[j].Created)
	})

	return versions, nil
}

// RestoreVersion 恢复到指定版本.
func (v *VersionManager) RestoreVersion(versionPath, targetPath string) error {
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}
	return os.WriteFile(targetPath, data, 0600)
}
