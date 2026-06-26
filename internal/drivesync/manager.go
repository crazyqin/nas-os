// Package drivesync 文件同步服务模块
// 学习群晖 Drive 的多端同步功能
package drivesync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 同步管理器
type Manager struct {
	mu          sync.RWMutex
	files       map[string]*SyncFile
	tasks       map[string]*SyncTask
	conflicts   map[string]*FileConflict
	policy      SyncPolicy
	storagePath string
	syncChan    chan *SyncTask
	stopChan    chan struct{}
	activities  []*Activity
	locks       map[string]*FileLock               // filePath -> lock
	comments    map[string][]*Comment              // filePath -> comments
	wsListeners map[string][]chan WebSocketMessage // filePath -> listeners
}

// NewManager 创建同步管理器
func NewManager(storagePath string) *Manager {
	m := &Manager{
		files:       make(map[string]*SyncFile),
		tasks:       make(map[string]*SyncTask),
		conflicts:   make(map[string]*FileConflict),
		storagePath: storagePath,
		syncChan:    make(chan *SyncTask, 100),
		stopChan:    make(chan struct{}),
		locks:       make(map[string]*FileLock),
		comments:    make(map[string][]*Comment),
		wsListeners: make(map[string][]chan WebSocketMessage),
		policy: SyncPolicy{
			AutoSync:     true,
			SyncInterval: 5 * time.Minute,
			ConflictMode: "ask",
			MaxFileSize:  10 * 1024 * 1024 * 1024, // 10GB
		},
	}
	go m.syncWorker()
	return m
}

// AddSyncPath 添加同步目录
func (m *Manager) AddSyncPath(ctx context.Context, path string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证路径存在
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// 扫描目录
	return m.scanDirectory(path, userID)
}

// SyncFile 同步单个文件
func (m *Manager) SyncFile(ctx context.Context, filePath string, targetPath string) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// 检查文件大小限制
	if info.Size() > m.policy.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds limit: %d > %d", info.Size(), m.policy.MaxFileSize)
	}

	// 创建同步任务
	task := &SyncTask{
		ID:         generateID(),
		LocalPath:  filePath,
		RemotePath: targetPath,
		Direction:  SyncUploadOnly,
		Status:     TaskStatusIdle,
		FileCount:  1,
		CreatedAt:  time.Now(),
	}

	m.tasks[task.ID] = task
	m.syncChan <- task

	return task, nil
}

// SyncDirectory 同步整个目录
func (m *Manager) SyncDirectory(ctx context.Context, sourcePath string, targetPath string) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &SyncTask{
		ID:         generateID(),
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Direction:  SyncBidirectional,
		Status:     TaskStatusIdle,
		CreatedAt:  time.Now(),
	}

	// 统计文件数量
	fileCount := 0
	filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	task.TotalFiles = fileCount

	m.tasks[task.ID] = task
	m.syncChan <- task

	return task, nil
}

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus(ctx context.Context, taskID string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListConflicts 列出冲突
func (m *Manager) ListConflicts(ctx context.Context) ([]FileConflict, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conflicts := make([]FileConflict, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		if c.Resolution == "" {
			conflicts = append(conflicts, *c)
		}
	}

	return conflicts, nil
}

// ResolveConflict 解决冲突
func (m *Manager) ResolveConflict(ctx context.Context, conflictID string, resolution string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, exists := m.conflicts[conflictID]
	if !exists {
		return fmt.Errorf("conflict not found: %s", conflictID)
	}

	validResolutions := map[string]bool{
		"keep_local":  true,
		"keep_remote": true,
		"keep_both":   true,
		"skip":        true,
	}

	if !validResolutions[resolution] {
		return fmt.Errorf("invalid resolution: %s", resolution)
	}

	conflict.Resolution = ConflictResolution(resolution)
	now := time.Now()
	conflict.ResolvedAt = &now

	return nil
}

// GetFileVersions 获取文件版本历史
func (m *Manager) GetFileVersions(ctx context.Context, filePath string) ([]SyncFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := make([]SyncFile, 0)
	for _, file := range m.files {
		if file.FilePath == filePath || file.Path == filePath {
			versions = append(versions, *file)
		}
	}
	return versions, nil
}

// SetPolicy 设置同步策略
func (m *Manager) SetPolicy(ctx context.Context, policy SyncPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.policy = policy
	return nil
}

// GetStats 获取同步统计
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSize := int64(0)
	syncedCount := 0
	pendingCount := 0
	conflictCount := 0

	for _, file := range m.files {
		totalSize += file.Size
		switch file.Status {
		case "synced":
			syncedCount++
		case FileStatusPending:
			pendingCount++
		case "conflict":
			conflictCount++
		}
	}

	return map[string]interface{}{
		"total_files":    len(m.files),
		"synced_files":   syncedCount,
		"pending_files":  pendingCount,
		"conflict_files": conflictCount,
		"total_size":     totalSize,
		"active_tasks":   len(m.tasks),
	}, nil
}

// 内部方法

func (m *Manager) scanDirectory(path string, userID string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过隐藏文件
		if info.Name()[0] == '.' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查排除模式
		if m.isExcluded(filePath) {
			return nil
		}

		// 计算校验和
		checksum := ""
		if !info.IsDir() {
			checksum = m.calculateChecksum(filePath)
		}

		file := &SyncFile{
			ID:         generateID(),
			Path:       filePath,
			Name:       info.Name(),
			Size:       info.Size(),
			Checksum:   checksum,
			ModifiedAt: info.ModTime(),
			Status:     "synced",
			OwnerID:    userID,
			IsFolder:   info.IsDir(),
		}

		m.files[file.ID] = file
		return nil
	})
}

func (m *Manager) isExcluded(path string) bool {
	for _, pattern := range m.policy.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

func (m *Manager) calculateChecksum(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (m *Manager) syncWorker() {
	for {
		select {
		case task := <-m.syncChan:
			m.processSyncTask(task)
		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) processSyncTask(task *SyncTask) {
	m.mu.Lock()
	task.Status = "running"
	m.mu.Unlock()

	log.Printf("Starting sync task: %s", task.ID)

	files := m.collectTaskFiles(task)
	if task.TotalFiles == 0 {
		task.TotalFiles = len(files)
	}
	if task.TotalFiles == 0 {
		task.TotalFiles = 1
	}
	for i, filePath := range files {
		if task.TargetPath != "" {
			_ = copySyncFile(filePath, filepath.Join(task.TargetPath, filepath.Base(filePath)))
		} else if task.RemotePath != "" {
			_ = copySyncFile(filePath, task.RemotePath)
		}
		m.mu.Lock()
		task.SyncedFiles = i + 1
		task.Progress = float64(task.SyncedFiles) / float64(task.TotalFiles)
		m.mu.Unlock()
	}
	if len(files) == 0 {
		m.mu.Lock()
		task.Progress = 1
		task.SyncedFiles = task.TotalFiles
		m.mu.Unlock()
	}

	m.mu.Lock()
	task.Status = "completed"
	task.Progress = 1.0
	now := time.Now()
	task.CompletedAt = &now
	m.mu.Unlock()

	log.Printf("Sync task completed: %s", task.ID)
}

func (m *Manager) collectTaskFiles(task *SyncTask) []string {
	root := task.SourcePath
	if root == "" {
		root = task.LocalPath
	}
	if root == "" {
		return nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{root}
	}
	files := make([]string, 0)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func copySyncFile(src, dst string) error {
	if src == "" || dst == "" || src == dst {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
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
	_, err = io.Copy(out, in)
	return err
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Export 导出同步文件列表
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]SyncFile, 0, len(m.files))
	for _, f := range m.files {
		files = append(files, *f)
	}

	return json.MarshalIndent(files, "", "  ")
}

// GetActivities 获取活动记录列表
func (m *Manager) GetActivities(limit int) []*Activity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.activities) {
		limit = len(m.activities)
	}
	// 返回最新的记录
	start := len(m.activities) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*Activity, limit)
	copy(result, m.activities[start:])
	return result
}

// addActivity 添加活动记录
func (m *Manager) addActivity(actType ActivityType, filePath, userID, userName, details string) {
	activity := &Activity{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:      actType,
		FilePath:  filePath,
		UserID:    userID,
		UserName:  userName,
		Details:   details,
		CreatedAt: time.Now(),
	}
	m.activities = append(m.activities, activity)
	// 限制活动记录数量
	if len(m.activities) > 10000 {
		m.activities = m.activities[len(m.activities)-10000:]
	}
}

// LockFile 锁定文件
func (m *Manager) LockFile(filePath string, input FileLockInput) (*FileLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否已被锁定
	if existingLock, exists := m.locks[filePath]; exists {
		if time.Now().Before(existingLock.ExpiresAt) {
			return nil, fmt.Errorf("%w: %s", ErrFileLocked, filePath)
		}
	}

	duration := input.Duration
	if duration <= 0 {
		duration = 30
	}

	lockType := input.LockType
	if lockType == "" {
		lockType = "exclusive"
	}

	lock := &FileLock{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		FilePath:  filePath,
		LockedBy:  input.LockedBy,
		LockType:  lockType,
		Reason:    input.Reason,
		ExpiresAt: time.Now().Add(time.Duration(duration) * time.Minute),
		CreatedAt: time.Now(),
	}

	m.locks[filePath] = lock
	m.addActivity(ActivityLockAcquired, filePath, input.LockedBy, "", "文件已锁定")

	return lock, nil
}

// UnlockFile 解锁文件
func (m *Manager) UnlockFile(filePath, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[filePath]
	if !exists {
		return fmt.Errorf("%w: %s", ErrFileNotLocked, filePath)
	}

	if lock.LockedBy != userID {
		return fmt.Errorf("文件被其他用户锁定: %s", lock.LockedBy)
	}

	delete(m.locks, filePath)
	m.addActivity(ActivityLockReleased, filePath, userID, "", "文件已解锁")

	return nil
}

// AddComment 添加评论
func (m *Manager) AddComment(filePath string, input CommentInput) *Comment {
	m.mu.Lock()
	defer m.mu.Unlock()

	comment := &Comment{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		FilePath:  filePath,
		UserID:    input.UserID,
		UserName:  input.UserName,
		Content:   input.Content,
		Mentions:  input.Mentions,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.comments[filePath] = append(m.comments[filePath], comment)
	m.addActivity(ActivityCommentAdded, filePath, input.UserID, input.UserName, input.Content)

	return comment
}

// broadcastWS 广播 WebSocket 消息
func (m *Manager) broadcastWS(msg WebSocketMessage) {
	// 不锁 mu，调用方需保证线程安全
	filePath, _ := msg.Payload.(map[string]interface{})["file_path"].(string)
	if filePath == "" {
		return
	}
	if listeners, exists := m.wsListeners[filePath]; exists {
		for _, ch := range listeners {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}
