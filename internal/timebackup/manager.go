// Package timebackup 提供文件/目录版本备份与恢复功能
package timebackup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Manager 版本备份管理器.
type Manager struct {
	tasks     map[string]*BackupTask
	snapshots map[string]*Snapshot
	baseDir   string           // 快照存储根目录
	cron      *cron.Cron
	logger    *zap.Logger
	mu        sync.RWMutex
	store     *Store
}

// NewManager 创建备份管理器.
// baseDir: 快照存储根目录.
func NewManager(baseDir string, store *Store, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		tasks:     make(map[string]*BackupTask),
		snapshots: make(map[string]*Snapshot),
		baseDir:   baseDir,
		cron:      cron.New(),
		logger:    logger,
		store:     store,
	}
}

// Start 启动调度器.
func (m *Manager) Start() error {
	if m.store != nil {
		// 从持久化存储加载任务
		tasks, err := m.store.LoadTasks()
		if err != nil {
			m.logger.Error("failed to load tasks from store", zap.Error(err))
		} else {
			m.mu.Lock()
			for _, t := range tasks {
				m.tasks[t.ID] = t
			}
			m.mu.Unlock()
		}

		// 加载快照
		snapshots, err := m.store.LoadSnapshots()
		if err != nil {
			m.logger.Error("failed to load snapshots from store", zap.Error(err))
		} else {
			m.mu.Lock()
			for _, s := range snapshots {
				m.snapshots[s.ID] = s
			}
			m.mu.Unlock()
		}
	}

	// 注册已启用的定时任务
	m.mu.RLock()
	for _, t := range m.tasks {
		if t.Enabled && t.Schedule != "" {
			m.registerCron(t)
		}
	}
	m.mu.RUnlock()

	m.cron.Start()
	m.logger.Info("timebackup manager started")
	return nil
}

// Stop 停止调度器.
func (m *Manager) Stop() {
	ctx := m.cron.Stop()
	<-ctx.Done()
	m.logger.Info("timebackup manager stopped")
}

// CreateTask 创建备份任务.
func (m *Manager) CreateTask(req *CreateTaskRequest) (*BackupTask, error) {
	// 检查源路径
	if _, err := os.Stat(req.SourcePath); err != nil {
		return nil, fmt.Errorf("源路径不存在: %w", err)
	}

	strategy := req.Strategy
	if strategy == "" {
		strategy = StrategyCopy
	}

	retention := RetentionPolicy{
		Mode: RetentionByCount,
		MaxCount: 10,
	}
	if req.Retention != nil {
		retention = *req.Retention
	}

	now := time.Now()
	task := &BackupTask{
		ID:        uuid.New().String(),
		Name:      req.Name,
		SourcePath: req.SourcePath,
		Strategy:  strategy,
		Schedule:  req.Schedule,
		Retention: retention,
		Status:    TaskStatusIdle,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 注册 cron
	if task.Schedule != "" {
		m.registerCron(task)
	}

	// 持久化
	if m.store != nil {
		if err := m.store.SaveTask(task); err != nil {
			m.logger.Error("failed to persist task", zap.Error(err))
		}
	}

	m.logger.Info("backup task created",
		zap.String("task_id", task.ID),
		zap.String("name", task.Name),
		zap.String("source", task.SourcePath),
	)
	return task, nil
}

// DeleteTask 删除备份任务及其快照.
func (m *Manager) DeleteTask(taskID string) error {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// 删除关联快照
	var snapshotIDs []string
	for id, snap := range m.snapshots {
		if snap.TaskID == taskID {
			snapshotIDs = append(snapshotIDs, id)
		}
	}
	for _, id := range snapshotIDs {
		snap := m.snapshots[id]
		_ = os.RemoveAll(snap.SnapshotDir)
		delete(m.snapshots, id)
	}

	delete(m.tasks, taskID)
	m.mu.Unlock()

	// 清理快照文件目录
	taskDir := filepath.Join(m.baseDir, taskID)
	_ = os.RemoveAll(taskDir)

	// 持久化删除
	if m.store != nil {
		if err := m.store.DeleteTask(taskID); err != nil {
			m.logger.Error("failed to delete task from store", zap.Error(err))
		}
		if err := m.store.DeleteSnapshotsByTask(taskID); err != nil {
			m.logger.Error("failed to delete snapshots from store", zap.Error(err))
		}
	}

	m.logger.Info("backup task deleted",
		zap.String("task_id", taskID),
		zap.String("name", task.Name),
	)
	return nil
}

// GetTask 获取备份任务详情.
func (m *Manager) GetTask(taskID string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出所有备份任务.
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BackupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// CreateSnapshot 为指定任务创建快照.
func (m *Manager) CreateSnapshot(taskID string) (*Snapshot, error) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 更新任务状态
	m.updateTaskStatus(taskID, TaskStatusRunning, "")

	snapID := uuid.New().String()
	now := time.Now()
	snapDir := filepath.Join(m.baseDir, taskID, snapID+"_"+now.Format("20060102_150405"))

	if err := os.MkdirAll(snapDir, 0755); err != nil {
		m.updateTaskStatus(taskID, TaskStatusFailed, err.Error())
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 执行复制或快照
	var size int64
	var fileCount int
	var err error

	switch task.Strategy {
	case StrategyBtrfs:
		size, fileCount, err = m.createBtrfsSnapshot(task.SourcePath, snapDir)
		if err != nil {
			// fallback to copy
			m.logger.Warn("btrfs snapshot failed, falling back to copy", zap.Error(err))
			size, fileCount, err = m.copyDirectory(task.SourcePath, snapDir)
		}
	default:
		size, fileCount, err = m.copyDirectory(task.SourcePath, snapDir)
	}

	if err != nil {
		_ = os.RemoveAll(snapDir)
		m.updateTaskStatus(taskID, TaskStatusFailed, err.Error())
		return nil, fmt.Errorf("创建快照失败: %w", err)
	}

	snapshot := &Snapshot{
		ID:          snapID,
		TaskID:      taskID,
		SourcePath:  task.SourcePath,
		SnapshotDir: snapDir,
		Size:        size,
		FileCount:   fileCount,
		Strategy:    task.Strategy,
		Metadata: map[string]string{
			"task_name": task.Name,
		},
		CreatedAt: now,
	}

	m.mu.Lock()
	m.snapshots[snapID] = snapshot
	task.SnapshotCount++
	m.mu.Unlock()

	// 更新任务状态
	m.updateTaskStatus(taskID, TaskStatusSuccess, "")

	// 持久化
	if m.store != nil {
		if err := m.store.SaveSnapshot(snapshot); err != nil {
			m.logger.Error("failed to persist snapshot", zap.Error(err))
		}
	}

	// 自动清理旧快照
	go m.autoCleanup(taskID)

	m.logger.Info("snapshot created",
		zap.String("task_id", taskID),
		zap.String("snapshot_id", snapID),
		zap.Int64("size", size),
		zap.Int("file_count", fileCount),
	)
	return snapshot, nil
}

// ListVersions 列出某个任务或路径的所有版本.
func (m *Manager) ListVersions(taskID, path string, limit int) []*Version {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var versions []*Version
	for _, snap := range m.snapshots {
		if taskID != "" && snap.TaskID != taskID {
			continue
		}
		if path != "" && !strings.HasPrefix(snap.SourcePath, path) {
			continue
		}
		versions = append(versions, &Version{
			SnapshotID: snap.ID,
			TaskID:     snap.TaskID,
			Path:       snap.SourcePath,
			Size:       snap.Size,
			FileCount:  snap.FileCount,
			CreatedAt:  snap.CreatedAt,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	if limit > 0 && len(versions) > limit {
		versions = versions[:limit]
	}
	return versions
}

// RestoreSnapshot 恢复快照到目标路径.
func (m *Manager) RestoreSnapshot(snapshotID, targetPath string, overwrite bool) error {
	m.mu.RLock()
	snap, ok := m.snapshots[snapshotID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("快照不存在: %s", snapshotID)
	}

	// 检查目标路径
	if _, err := os.Stat(targetPath); err == nil && !overwrite {
		return fmt.Errorf("目标路径已存在且未设置覆盖: %s", targetPath)
	}

	// 确保源快照目录存在
	if _, err := os.Stat(snap.SnapshotDir); err != nil {
		return fmt.Errorf("快照数据不存在: %w", err)
	}

	// 恢复（复制快照内容到目标路径）
	if _, err := os.Stat(targetPath); err == nil && overwrite {
		if err := os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("清理目标路径失败: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	_, _, err := m.copyDirectory(snap.SnapshotDir, targetPath)
	if err != nil {
		return fmt.Errorf("恢复快照失败: %w", err)
	}

	m.logger.Info("snapshot restored",
		zap.String("snapshot_id", snapshotID),
		zap.String("target", targetPath),
	)
	return nil
}

// DiffSnapshots 对比两个快照.
func (m *Manager) DiffSnapshots(snapshotOld, snapshotNew string) (*DiffResult, error) {
	m.mu.RLock()
	snapOld, ok1 := m.snapshots[snapshotOld]
	snapNew, ok2 := m.snapshots[snapshotNew]
	m.mu.RUnlock()

	if !ok1 {
		return nil, fmt.Errorf("快照不存在: %s", snapshotOld)
	}
	if !ok2 {
		return nil, fmt.Errorf("快照不存在: %s", snapshotNew)
	}

	// 收集两个快照的文件信息
	oldFiles, err := collectFileInfo(snapOld.SnapshotDir)
	if err != nil {
		return nil, fmt.Errorf("读取旧快照失败: %w", err)
	}

	newFiles, err := collectFileInfo(snapNew.SnapshotDir)
	if err != nil {
		return nil, fmt.Errorf("读取新快照失败: %w", err)
	}

	var entries []*DiffEntry

	// 检查新增和修改
	for path, newInfo := range newFiles {
		oldInfo, exists := oldFiles[path]
		if !exists {
			entries = append(entries, &DiffEntry{
				Path:   path,
				Change: "added",
				SizeNew: newInfo.size,
				ModTimeNew: newInfo.modTime.Format(time.RFC3339),
			})
		} else if oldInfo.size != newInfo.size || oldInfo.modTime != newInfo.modTime {
			entries = append(entries, &DiffEntry{
				Path:   path,
				Change: "modified",
				SizeOld: oldInfo.size,
				SizeNew: newInfo.size,
				ModTimeOld: oldInfo.modTime.Format(time.RFC3339),
				ModTimeNew: newInfo.modTime.Format(time.RFC3339),
			})
		}
	}

	// 检查删除
	for path, oldInfo := range oldFiles {
		if _, exists := newFiles[path]; !exists {
			entries = append(entries, &DiffEntry{
				Path:   path,
				Change: "removed",
				SizeOld: oldInfo.size,
				ModTimeOld: oldInfo.modTime.Format(time.RFC3339),
			})
		}
	}

	summary := DiffSummary{Total: len(entries)}
	for _, e := range entries {
		switch e.Change {
		case "added":
			summary.Added++
		case "removed":
			summary.Removed++
		case "modified":
			summary.Modified++
		}
	}

	return &DiffResult{
		SnapshotOld: snapshotOld,
		SnapshotNew: snapshotNew,
		Entries:     entries,
		Summary:     summary,
	}, nil
}

// DeleteSnapshot 删除单个快照.
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	m.mu.Lock()
	snap, ok := m.snapshots[snapshotID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("快照不存在: %s", snapshotID)
	}
	delete(m.snapshots, snapshotID)
	taskID := snap.TaskID
	m.mu.Unlock()

	// 删除快照文件
	if err := os.RemoveAll(snap.SnapshotDir); err != nil {
		m.logger.Error("failed to remove snapshot dir", zap.Error(err))
	}

	// 更新任务快照计数
	m.mu.Lock()
	if task, ok := m.tasks[taskID]; ok {
		task.SnapshotCount--
		if task.SnapshotCount < 0 {
			task.SnapshotCount = 0
		}
	}
	m.mu.Unlock()

	// 持久化
	if m.store != nil {
		if err := m.store.DeleteSnapshot(snapshotID); err != nil {
			m.logger.Error("failed to delete snapshot from store", zap.Error(err))
		}
	}

	m.logger.Info("snapshot deleted", zap.String("snapshot_id", snapshotID))
	return nil
}

// GetSnapshotsByTask 获取任务的所有快照（按时间排序）.
func (m *Manager) GetSnapshotsByTask(taskID string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	for _, snap := range m.snapshots {
		if snap.TaskID == taskID {
			result = append(result, snap)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// autoCleanup 根据保留策略自动清理过期快照.
func (m *Manager) autoCleanup(taskID string) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	snapshots := m.GetSnapshotsByTask(taskID)
	var toDelete []*Snapshot

	switch task.Retention.Mode {
	case RetentionByCount:
		if task.Retention.MaxCount > 0 && len(snapshots) > task.Retention.MaxCount {
			// 保留最近 N 个，删除多余的
			toDelete = snapshots[task.Retention.MaxCount:]
		}
	case RetentionByTime:
		if task.Retention.MaxAgeDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -task.Retention.MaxAgeDays)
			for _, snap := range snapshots {
				if snap.CreatedAt.Before(cutoff) {
					toDelete = append(toDelete, snap)
				}
			}
		}
	case RetentionBySpace:
		if task.Retention.MaxSizeGB > 0 {
			maxBytes := int64(task.Retention.MaxSizeGB * 1024 * 1024 * 1024)
			var totalSize int64
			for _, snap := range snapshots {
				totalSize += snap.Size
				if totalSize > maxBytes {
					toDelete = append(toDelete, snap)
				}
			}
		}
	}

	for _, snap := range toDelete {
		if err := m.DeleteSnapshot(snap.ID); err != nil {
			m.logger.Error("auto-cleanup failed",
				zap.String("snapshot_id", snap.ID),
				zap.Error(err),
			)
		} else {
			m.logger.Info("auto-cleanup: deleted old snapshot",
				zap.String("snapshot_id", snap.ID),
				zap.String("task_id", taskID),
			)
		}
	}
}

// registerCron 注册定时任务.
func (m *Manager) registerCron(task *BackupTask) {
	_, err := m.cron.AddFunc(task.Schedule, func() {
		m.logger.Info("cron triggered snapshot", zap.String("task_id", task.ID))
		if _, err := m.CreateSnapshot(task.ID); err != nil {
			m.logger.Error("cron snapshot failed",
				zap.String("task_id", task.ID),
				zap.Error(err),
			)
		}
	})
	if err != nil {
		m.logger.Error("failed to register cron",
			zap.String("task_id", task.ID),
			zap.String("schedule", task.Schedule),
			zap.Error(err),
		)
	}
}

// updateTaskStatus 更新任务状态.
func (m *Manager) updateTaskStatus(taskID string, status TaskStatus, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	task.Status = status
	task.LastError = errMsg
	now := time.Now()
	task.LastRun = &now
	task.UpdatedAt = now

	if m.store != nil {
		if err := m.store.SaveTask(task); err != nil {
			m.logger.Error("failed to update task status", zap.Error(err))
		}
	}
}

// copyDirectory 递归复制目录，返回总大小和文件数.
func (m *Manager) copyDirectory(src, dst string) (int64, int, error) {
	var totalSize int64
	var fileCount int

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		// 复制文件
		if err := copyFile(path, target); err != nil {
			return err
		}
		totalSize += info.Size()
		fileCount++
		return nil
	})

	return totalSize, fileCount, err
}

// createBtrfsSnapshot 创建 btrfs 快照（需内核支持）.
func (m *Manager) createBtrfsSnapshot(src, dst string) (int64, int, error) {
	// btrfs 快照需要 root 权限和 btrfs 文件系统
	// 这里提供框架，实际执行需要 btrfs 工具
	m.logger.Info("btrfs snapshot not implemented, using copy fallback",
		zap.String("src", src),
		zap.String("dst", dst),
	)
	return m.copyDirectory(src, dst)
}

// fileInfo 文件元信息.
type fileInfo struct {
	size    int64
	modTime time.Time
	hash    string
}

// collectFileInfo 收集目录下所有文件信息.
func collectFileInfo(dir string) (map[string]*fileInfo, error) {
	files := make(map[string]*fileInfo)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		files[relPath] = &fileInfo{
			size:    info.Size(),
			modTime: info.ModTime(),
		}
		return nil
	})

	return files, err
}

// copyFile 复制单个文件.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	// 保留文件权限
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// fileHash 计算文件 SHA256.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// taskToJSON 序列化任务（用于持久化）.
func taskToJSON(task *BackupTask) (string, error) {
	b, err := json.Marshal(task)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// snapshotToJSON 序列化快照（用于持久化）.
func snapshotToJSON(snap *Snapshot) (string, error) {
	b, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
