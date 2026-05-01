// Package drivesync 提供增强版文件同步功能
package drivesync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager Drive Sync 核心管理器.
type Manager struct {
	mu            sync.RWMutex
	tasks         map[string]*SyncTask       // taskID -> SyncTask
	files         map[string]*FileInfo        // filePath -> FileInfo
	versions      map[string][]*FileVersion   // filePath -> []FileVersion
	conflicts     map[string]*FileConflict    // conflictID -> FileConflict
	locks         map[string]*FileLock        // filePath -> FileLock
	comments      map[string][]*Comment       // filePath -> []Comment
	activities    []*Activity                 // 活动记录（最近N条）
	maxActivities int                         // 最大活动记录数
	configPath    string                      // 配置持久化路径
	versionConfig VersionConfig               // 版本控制配置
	startTime     time.Time                   // 启动时间
	wsClients     map[string]chan WebSocketMessage // WebSocket 客户端通道
}

// NewManager 创建 Drive Sync 管理器.
func NewManager(configPath string, versionCfg VersionConfig) *Manager {
	// 设置默认版本配置
	if versionCfg.RetentionDays == 0 {
		versionCfg.RetentionDays = 30
	}
	if versionCfg.MaxVersions == 0 {
		versionCfg.MaxVersions = 100
	}

	m := &Manager{
		tasks:         make(map[string]*SyncTask),
		files:         make(map[string]*FileInfo),
		versions:      make(map[string][]*FileVersion),
		conflicts:     make(map[string]*FileConflict),
		locks:         make(map[string]*FileLock),
		comments:      make(map[string][]*Comment),
		activities:    make([]*Activity, 0),
		maxActivities: 1000,
		configPath:    configPath,
		versionConfig: versionCfg,
		startTime:     time.Now(),
		wsClients:     make(map[string]chan WebSocketMessage),
	}

	// 加载持久化配置
	if configPath != "" {
		_ = m.loadConfig()
	}

	return m
}

// ========== 同步任务管理 ==========

// CreateTask 创建同步任务.
func (m *Manager) CreateTask(input SyncTaskInput) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if input.Direction == "" {
		input.Direction = SyncBidirectional
	}
	if input.ConflictPolicy == "" {
		input.ConflictPolicy = ConflictNewerWins
	}

	task := &SyncTask{
		ID:              generateID(),
		Name:            input.Name,
		LocalPath:       input.LocalPath,
		RemotePath:      input.RemotePath,
		DeviceID:        input.DeviceID,
		Direction:       input.Direction,
		ConflictPolicy:  input.ConflictPolicy,
		Status:          TaskStatusIdle,
		Enabled:         input.Enabled,
		Interval:        input.Interval,
		ExcludePatterns: input.ExcludePatterns,
		IncludePatterns: input.IncludePatterns,
		BandwidthLimit:  input.BandwidthLimit,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	m.tasks[task.ID] = task

	// 记录活动
	m.addActivity(ActivitySyncStarted, task.LocalPath, "", task.ID, fmt.Sprintf("创建同步任务: %s", task.Name))

	_ = m.saveConfig()
	return task, nil
}

// GetTask 获取同步任务.
func (m *Manager) GetTask(id string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrSyncTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有同步任务.
func (m *Manager) ListTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SyncTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// UpdateTask 更新同步任务.
func (m *Manager) UpdateTask(id string, input SyncTaskInput) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrSyncTaskNotFound
	}

	task.Name = input.Name
	task.LocalPath = input.LocalPath
	task.RemotePath = input.RemotePath
	task.DeviceID = input.DeviceID
	if input.Direction != "" {
		task.Direction = input.Direction
	}
	if input.ConflictPolicy != "" {
		task.ConflictPolicy = input.ConflictPolicy
	}
	task.Enabled = input.Enabled
	task.Interval = input.Interval
	task.ExcludePatterns = input.ExcludePatterns
	task.IncludePatterns = input.IncludePatterns
	task.BandwidthLimit = input.BandwidthLimit
	task.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return task, nil
}

// DeleteTask 删除同步任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrSyncTaskNotFound
	}

	// 检查是否正在同步
	if task.Status == TaskStatusSyncing {
		return ErrSyncTaskRunning
	}

	delete(m.tasks, id)
	_ = m.saveConfig()
	return nil
}

// PauseTask 暂停同步任务.
func (m *Manager) PauseTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrSyncTaskNotFound
	}

	if task.Status != TaskStatusSyncing && task.Status != TaskStatusIdle {
		return ErrSyncTaskNotRunning
	}

	task.Status = TaskStatusPaused
	task.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return nil
}

// ResumeTask 恢复同步任务.
func (m *Manager) ResumeTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrSyncTaskNotFound
	}

	if task.Status != TaskStatusPaused {
		return fmt.Errorf("任务未暂停，当前状态: %s", task.Status)
	}

	task.Status = TaskStatusIdle
	task.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return nil
}

// ========== 版本管理 ==========

// GetFileVersions 获取文件版本历史.
func (m *Manager) GetFileVersions(filePath string) []*FileVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[filePath]
	if versions == nil {
		return make([]*FileVersion, 0)
	}
	return versions
}

// CreateFileVersion 创建文件版本.
func (m *Manager) CreateFileVersion(filePath string, size int64, checksum string, createdBy string) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[filePath]
	versionNum := len(versions) + 1

	expiresAt := time.Now().AddDate(0, 0, m.versionConfig.RetentionDays)

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

	m.versions[filePath] = append(m.versions[filePath], version)

	// 清理过期版本
	m.cleanupVersions(filePath)

	// 记录活动
	m.addActivity(ActivityVersionCreated, filePath, createdBy, "", fmt.Sprintf("创建版本 v%d", versionNum))

	_ = m.saveConfig()
	return version, nil
}

// RestoreVersion 恢复文件到指定版本.
func (m *Manager) RestoreVersion(filePath string, versionID string) (*FileVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			// 记录活动
			m.addActivity(ActivityFileRestored, filePath, "", "", fmt.Sprintf("恢复到版本 v%d", v.VersionNum))
			_ = m.saveConfig()
			return v, nil
		}
	}

	return nil, ErrFileVersionNotFound
}

// DiffVersions 对比两个版本的差异.
func (m *Manager) DiffVersions(filePath string, v1ID string, v2ID string) (*VersionDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[filePath]

	var ver1, ver2 *FileVersion
	for _, v := range versions {
		if v.ID == v1ID {
			ver1 = v
		}
		if v.ID == v2ID {
			ver2 = v
		}
	}

	if ver1 == nil || ver2 == nil {
		return nil, ErrFileVersionNotFound
	}

	// 简化版diff：比较校验和和大小
	diff := &VersionDiff{
		FromVersion:  v1ID,
		ToVersion:    v2ID,
		FromChecksum: ver1.Checksum,
		ToChecksum:   ver2.Checksum,
	}

	if ver1.Checksum == ver2.Checksum {
		diff.Similarity = 1.0
	} else {
		// 基于大小差异估算相似度
		sizeDiff := ver1.Size - ver2.Size
		if sizeDiff < 0 {
			sizeDiff = -sizeDiff
		}
		maxSize := ver1.Size
		if ver2.Size > maxSize {
			maxSize = ver2.Size
		}
		if maxSize > 0 {
			diff.Similarity = 1.0 - float64(sizeDiff)/float64(maxSize)
		}
		diff.Modified = 1
	}

	return diff, nil
}

// cleanupVersions 清理过期和超限版本.
func (m *Manager) cleanupVersions(filePath string) {
	versions := m.versions[filePath]
	if len(versions) == 0 {
		return
	}

	now := time.Now()
	var valid []*FileVersion

	for _, v := range versions {
		if v.ExpiresAt != nil && v.ExpiresAt.Before(now) {
			continue // 已过期，跳过
		}
		valid = append(valid, v)
	}

	// 超过最大版本数，保留最新的
	if len(valid) > m.versionConfig.MaxVersions {
		valid = valid[len(valid)-m.versionConfig.MaxVersions:]
	}

	m.versions[filePath] = valid
}

// SetVersionLabel 设置版本标签.
func (m *Manager) SetVersionLabel(filePath, versionID, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			v.Label = label
			_ = m.saveConfig()
			return nil
		}
	}
	return ErrFileVersionNotFound
}

// SetVersionComment 设置版本注释.
func (m *Manager) SetVersionComment(filePath, versionID, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[filePath]
	for _, v := range versions {
		if v.ID == versionID {
			v.Comment = comment
			_ = m.saveConfig()
			return nil
		}
	}
	return ErrFileVersionNotFound
}

// ========== 冲突管理 ==========

// CreateConflict 创建冲突记录.
func (m *Manager) CreateConflict(taskID, filePath string, localChecksum, remoteChecksum string, localModTime, remoteModTime time.Time, localSize, remoteSize int64, localDeviceID, remoteDeviceID string) *FileConflict {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict := &FileConflict{
		ID:             generateID(),
		TaskID:         taskID,
		FilePath:       filePath,
		LocalChecksum:  localChecksum,
		RemoteChecksum: remoteChecksum,
		LocalModTime:   localModTime,
		RemoteModTime:  remoteModTime,
		LocalSize:      localSize,
		RemoteSize:     remoteSize,
		LocalDeviceID:  localDeviceID,
		RemoteDeviceID: remoteDeviceID,
		Status:         ConflictStatusPending,
		CreatedAt:      time.Now(),
	}

	m.conflicts[conflict.ID] = conflict

	// 记录活动
	m.addActivity(ActivityConflictDetected, filePath, "", taskID, "检测到文件冲突")

	// 通知 WebSocket 客户端
	m.broadcastWS(WebSocketMessage{
		Type:    "conflict",
		Payload: conflict,
		Time:    time.Now(),
	})

	_ = m.saveConfig()
	return conflict
}

// ListConflicts 列出所有冲突.
func (m *Manager) ListConflicts() []*FileConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FileConflict, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		result = append(result, c)
	}
	return result
}

// ResolveConflict 解决冲突.
func (m *Manager) ResolveConflict(conflictID string, resolution ConflictResolution, resolvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, exists := m.conflicts[conflictID]
	if !exists {
		return ErrConflictNotFound
	}

	now := time.Now()
	conflict.Resolution = resolution
	conflict.Status = ConflictStatusResolved
	conflict.ResolvedBy = resolvedBy
	conflict.ResolvedAt = &now

	// 如果保留双方，生成重命名路径
	if resolution == ConflictKeepBoth {
		ext := filepath.Ext(conflict.FilePath)
		base := conflict.FilePath[:len(conflict.FilePath)-len(ext)]
		conflict.RenamedPath = fmt.Sprintf("%s (conflict %s)%s", base, now.Format("20060102-150405"), ext)
	}

	// 记录活动
	m.addActivity(ActivityConflictResolved, conflict.FilePath, resolvedBy, conflict.TaskID,
		fmt.Sprintf("冲突已解决: %s", resolution))

	// 通知 WebSocket 客户端
	m.broadcastWS(WebSocketMessage{
		Type:    "conflict_resolved",
		Payload: conflict,
		Time:    time.Now(),
	})

	_ = m.saveConfig()
	return nil
}

// ========== 文件锁管理 ==========

// LockFile 锁定文件.
func (m *Manager) LockFile(filePath string, input FileLockInput) (*FileLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已被锁定
	if existingLock, exists := m.locks[filePath]; exists {
		// 检查锁是否过期
		if existingLock.ExpiresAt.After(time.Now()) {
			return nil, ErrFileLocked
		}
		// 锁已过期，自动清理
		delete(m.locks, filePath)
	}

	// 设置默认值
	lockType := input.LockType
	if lockType == "" {
		lockType = "exclusive"
	}
	duration := input.Duration
	if duration <= 0 {
		duration = 30 // 默认30分钟
	}

	lock := &FileLock{
		ID:        generateID(),
		FilePath:  filePath,
		LockedBy:  input.LockedBy,
		LockType:  lockType,
		Reason:    input.Reason,
		ExpiresAt: time.Now().Add(time.Duration(duration) * time.Minute),
		CreatedAt: time.Now(),
	}

	m.locks[filePath] = lock

	// 记录活动
	m.addActivity(ActivityLockAcquired, filePath, input.LockedBy, "", fmt.Sprintf("文件已锁定 (%s)", lockType))

	// 通知 WebSocket 客户端
	m.broadcastWS(WebSocketMessage{
		Type:    "lock_change",
		Payload: lock,
		Time:    time.Now(),
	})

	_ = m.saveConfig()
	return lock, nil
}

// UnlockFile 解锁文件.
func (m *Manager) UnlockFile(filePath string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[filePath]
	if !exists {
		return ErrFileNotLocked
	}

	// 验证锁定者（只能由锁定者解锁）
	if lock.LockedBy != userID {
		return fmt.Errorf("只有锁定者 %s 可以解锁", lock.LockedBy)
	}

	delete(m.locks, filePath)

	// 记录活动
	m.addActivity(ActivityLockReleased, filePath, userID, "", "文件已解锁")

	// 通知 WebSocket 客户端
	m.broadcastWS(WebSocketMessage{
		Type:    "lock_change",
		Payload: map[string]string{"file_path": filePath, "action": "unlocked"},
		Time:    time.Now(),
	})

	_ = m.saveConfig()
	return nil
}

// ListLocks 列出所有文件锁.
func (m *Manager) ListLocks() []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FileLock, 0, len(m.locks))
	for _, l := range m.locks {
		// 只返回未过期的锁
		if l.ExpiresAt.After(time.Now()) {
			result = append(result, l)
		}
	}
	return result
}

// GetFileLock 获取文件的锁信息.
func (m *Manager) GetFileLock(filePath string) *FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, exists := m.locks[filePath]
	if !exists || lock.ExpiresAt.Before(time.Now()) {
		return nil
	}
	return lock
}

// ========== 协作（评论和活动） ==========

// AddComment 添加文件评论.
func (m *Manager) AddComment(filePath string, input CommentInput) *Comment {
	m.mu.Lock()
	defer m.mu.Unlock()

	comment := &Comment{
		ID:        generateID(),
		FilePath:  filePath,
		UserID:    input.UserID,
		UserName:  input.UserName,
		Content:   input.Content,
		Mentions:  input.Mentions,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.comments[filePath] = append(m.comments[filePath], comment)

	// 记录活动
	m.addActivity(ActivityCommentAdded, filePath, input.UserID, "", fmt.Sprintf("添加评论: %s", truncate(input.Content, 50)))

	// 如果有@提及，记录提及活动
	if len(input.Mentions) > 0 {
		m.addActivity(ActivityMentionAdded, filePath, input.UserID, "", fmt.Sprintf("@提及了 %d 人", len(input.Mentions)))
	}

	// 通知 WebSocket 客户端
	m.broadcastWS(WebSocketMessage{
		Type:    "comment_added",
		Payload: comment,
		Time:    time.Now(),
	})

	_ = m.saveConfig()
	return comment
}

// GetComments 获取文件评论列表.
func (m *Manager) GetComments(filePath string) []*Comment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	comments := m.comments[filePath]
	if comments == nil {
		return make([]*Comment, 0)
	}
	return comments
}

// GetActivities 获取活动流.
func (m *Manager) GetActivities(limit int) []*Activity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > len(m.activities) {
		limit = len(m.activities)
	}

	result := make([]*Activity, limit)
	copy(result, m.activities[len(m.activities)-limit:])

	// 反转顺序（最新的在前）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// addActivity 添加活动记录（需要持有写锁）.
func (m *Manager) addActivity(actType ActivityType, filePath, userID, taskID, details string) {
	activity := &Activity{
		ID:        generateID(),
		Type:      actType,
		FilePath:  filePath,
		UserID:    userID,
		TaskID:    taskID,
		Details:   details,
		CreatedAt: time.Now(),
	}

	m.activities = append(m.activities, activity)

	// 限制活动记录数
	if len(m.activities) > m.maxActivities {
		m.activities = m.activities[len(m.activities)-m.maxActivities:]
	}
}

// ========== 同步统计 ==========

// GetStats 获取同步统计信息.
func (m *Manager) GetStats() *SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SyncStats{
		TotalTasks: len(m.tasks),
		Uptime:     time.Since(m.startTime),
	}

	for _, t := range m.tasks {
		switch t.Status {
		case TaskStatusSyncing:
			stats.ActiveTasks++
		case TaskStatusPaused:
			stats.PausedTasks++
		case TaskStatusError:
			stats.ErrorTasks++
		}
		stats.TotalFiles += t.FileCount
		stats.SyncedBytes += t.SyncedBytes

		if t.LastSyncAt != nil {
			if stats.LastSyncAt == nil || t.LastSyncAt.After(*stats.LastSyncAt) {
				stats.LastSyncAt = t.LastSyncAt
			}
		}
	}

	// 统计版本数
	for _, versions := range m.versions {
		stats.TotalVersions += len(versions)
	}

	// 统计活跃锁数
	for _, l := range m.locks {
		if l.ExpiresAt.After(time.Now()) {
			stats.ActiveLocks++
		}
	}

	// 统计待解决冲突
	for _, c := range m.conflicts {
		if c.Status == ConflictStatusPending {
			stats.PendingConflicts++
		}
	}

	return stats
}

// ========== WebSocket ==========

// RegisterWSClient 注册 WebSocket 客户端.
func (m *Manager) RegisterWSClient(clientID string) chan WebSocketMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan WebSocketMessage, 100)
	m.wsClients[clientID] = ch
	return ch
}

// UnregisterWSClient 注销 WebSocket 客户端.
func (m *Manager) UnregisterWSClient(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.wsClients[clientID]; exists {
		close(ch)
		delete(m.wsClients, clientID)
	}
}

// broadcastWS 广播 WebSocket 消息（需要持有写锁）.
func (m *Manager) broadcastWS(msg WebSocketMessage) {
	for _, ch := range m.wsClients {
		select {
		case ch <- msg:
		default:
			// 客户端缓冲区满，跳过
		}
	}
}

// ========== 持久化 ==========

type persistentConfig struct {
	Tasks     map[string]*SyncTask        `json:"tasks"`
	Versions  map[string][]*FileVersion   `json:"versions"`
	Conflicts map[string]*FileConflict    `json:"conflicts"`
	Locks     map[string]*FileLock        `json:"locks"`
	Comments  map[string][]*Comment       `json:"comments"`
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	if pc.Tasks != nil {
		m.tasks = pc.Tasks
	}
	if pc.Versions != nil {
		m.versions = pc.Versions
	}
	if pc.Conflicts != nil {
		m.conflicts = pc.Conflicts
	}
	if pc.Locks != nil {
		m.locks = pc.Locks
	}
	if pc.Comments != nil {
		m.comments = pc.Comments
	}

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentConfig{
		Tasks:     m.tasks,
		Versions:  m.versions,
		Conflicts: m.conflicts,
		Locks:     m.locks,
		Comments:  m.comments,
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 工具函数 ==========

// generateID 生成唯一ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// truncate 截断字符串.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
