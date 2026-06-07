// Package filelock 文件锁定管理器核心逻辑
package filelock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 文件锁管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	policy    *LockPolicy
	locks     map[string]*FileLock // 锁ID -> 锁记录
	fileLocks map[string][]string  // 文件路径 -> 锁ID列表
	history   []*LockHistoryEntry
	stopChan  chan struct{}
	running   bool
}

// NewManager 创建文件锁管理器
func NewManager(logger *zap.Logger, policy *LockPolicy) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if policy == nil {
		policy = DefaultLockPolicy()
	}

	m := &Manager{
		logger:    logger,
		policy:    policy,
		locks:     make(map[string]*FileLock),
		fileLocks: make(map[string][]string),
		history:   make([]*LockHistoryEntry, 0),
		stopChan:  make(chan struct{}),
	}

	return m
}

// Start 启动管理器，开始自动清理过期锁
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.cleanupLoop()
	m.logger.Info("file lock manager started")
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.logger.Info("file lock manager stopped")
}

// cleanupLoop 定期清理过期锁
func (m *Manager) cleanupLoop() {
	interval := time.Duration(m.policy.CleanupInterval) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanupExpiredLocks()
		}
	}
}

// cleanupExpiredLocks 清理过期锁
func (m *Manager) cleanupExpiredLocks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expiredIDs := make([]string, 0)

	for id, lock := range m.locks {
		if lock.Status == LockStatusActive && now.After(lock.ExpiresAt) {
			lock.Status = LockStatusExpired
			lock.UpdatedAt = now
			expiredIDs = append(expiredIDs, id)

			m.addHistoryEntryLocked(lock.FilePath, "expired", lock.UserID, lock.UserName, lock.LockType, "锁已过期自动释放")
			m.logger.Info("lock expired",
				zap.String("lock_id", id),
				zap.String("file_path", lock.FilePath))
		}
	}

	if len(expiredIDs) > 0 {
		m.logger.Info("cleanup completed", zap.Int("expired_locks", len(expiredIDs)))
	}
}

// AcquireLock 获取文件锁
func (m *Manager) AcquireLock(req *AcquireRequest) (*FileLock, error) {
	if !m.policy.Enabled {
		return nil, fmt.Errorf("文件锁定功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认锁类型
	lockType := req.LockType
	if lockType == "" {
		lockType = m.policy.DefaultLockType
	}

	// 设置默认持续时间
	duration := req.Duration
	if duration <= 0 {
		switch lockType {
		case LockTypeExclusive:
			duration = m.policy.ExclusiveLockMaxDuration
		case LockTypeShared:
			duration = m.policy.SharedLockMaxDuration
		default:
			duration = m.policy.ExclusiveLockMaxDuration
		}
	}

	// 验证持续时间
	maxDuration := m.policy.ExclusiveLockMaxDuration
	if lockType == LockTypeShared {
		maxDuration = m.policy.SharedLockMaxDuration
	}
	if duration > maxDuration {
		return nil, fmt.Errorf("锁定时长超过最大限制: %d分钟", maxDuration)
	}

	// 检查用户锁数量限制
	userLockCount := 0
	for _, lock := range m.locks {
		if lock.UserID == req.UserID && lock.Status == LockStatusActive {
			userLockCount++
		}
	}
	if userLockCount >= m.policy.MaxLocksPerUser {
		return nil, fmt.Errorf("用户锁数量已达上限: %d", m.policy.MaxLocksPerUser)
	}

	// 检查文件锁冲突
	existingLocks := m.getFileActiveLocks(req.FilePath)
	if len(existingLocks) > 0 {
		// 检查是否有独占锁
		for _, existing := range existingLocks {
			if existing.LockType == LockTypeExclusive {
				if existing.UserID == req.UserID {
					return nil, fmt.Errorf("文件已被您锁定（独占锁）")
				}
				return nil, fmt.Errorf("文件已被用户 %s 独占锁定", existing.UserName)
			}
		}

		// 如果请求独占锁，但有共享锁存在
		if lockType == LockTypeExclusive {
			return nil, fmt.Errorf("文件存在共享锁定，无法获取独占锁")
		}

		// 共享锁：检查同一用户是否已持有共享锁
		for _, existing := range existingLocks {
			if existing.UserID == req.UserID {
				return nil, fmt.Errorf("您已持有该文件的共享锁")
			}
		}
	}

	now := time.Now()
	lockID := uuid.New().String()

	lock := &FileLock{
		ID:            lockID,
		FilePath:      req.FilePath,
		LockType:      lockType,
		Status:        LockStatusActive,
		UserID:        req.UserID,
		UserName:      req.UserName,
		AcquiredAt:    now,
		LastRenewedAt: now,
		ExpiresAt:     now.Add(time.Duration(duration) * time.Minute),
		Comment:       req.Comment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.locks[lockID] = lock
	m.addFileLock(req.FilePath, lockID)
	m.addHistoryEntryLocked(req.FilePath, "acquired", req.UserID, req.UserName, lockType, req.Comment)

	m.logger.Info("lock acquired",
		zap.String("lock_id", lockID),
		zap.String("file_path", req.FilePath),
		zap.String("user_id", req.UserID),
		zap.String("lock_type", string(lockType)))

	return lock, nil
}

// ReleaseLock 释放文件锁
func (m *Manager) ReleaseLock(req *ReleaseRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.UserID != req.UserID {
		return fmt.Errorf("无权释放此锁：锁属于用户 %s", lock.UserName)
	}

	if lock.Status != LockStatusActive {
		return fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	now := time.Now()
	lock.Status = LockStatusReleased
	lock.ReleasedAt = &now
	lock.UpdatedAt = now

	m.removeFileLock(lock.FilePath, req.LockID)
	m.addHistoryEntryLocked(lock.FilePath, "released", req.UserID, lock.UserName, lock.LockType, "用户主动释放")

	m.logger.Info("lock released",
		zap.String("lock_id", req.LockID),
		zap.String("file_path", lock.FilePath),
		zap.String("user_id", req.UserID))

	return nil
}

// RenewLock 续期文件锁
func (m *Manager) RenewLock(req *RenewRequest) (*FileLock, error) {
	if !m.policy.AllowRenewal {
		return nil, fmt.Errorf("续期功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.UserID != req.UserID {
		return nil, fmt.Errorf("无权续期此锁：锁属于用户 %s", lock.UserName)
	}

	if lock.Status != LockStatusActive {
		return nil, fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	// 设置续期时长
	duration := req.Duration
	if duration <= 0 {
		switch lock.LockType {
		case LockTypeExclusive:
			duration = m.policy.ExclusiveLockMaxDuration
		case LockTypeShared:
			duration = m.policy.SharedLockMaxDuration
		default:
			duration = m.policy.ExclusiveLockMaxDuration
		}
	}

	// 验证续期时长
	maxDuration := m.policy.ExclusiveLockMaxDuration
	if lock.LockType == LockTypeShared {
		maxDuration = m.policy.SharedLockMaxDuration
	}
	if duration > maxDuration {
		return nil, fmt.Errorf("续期时长超过最大限制: %d分钟", maxDuration)
	}

	now := time.Now()
	lock.LastRenewedAt = now
	lock.ExpiresAt = now.Add(time.Duration(duration) * time.Minute)
	lock.UpdatedAt = now

	m.addHistoryEntryLocked(lock.FilePath, "renewed", req.UserID, lock.UserName, lock.LockType, fmt.Sprintf("续期%d分钟", duration))

	m.logger.Info("lock renewed",
		zap.String("lock_id", req.LockID),
		zap.String("file_path", lock.FilePath),
		zap.Int("duration", duration))

	return lock, nil
}

// ForceReleaseLock 管理员强制释放锁
func (m *Manager) ForceReleaseLock(req *ForceReleaseRequest) error {
	if !m.policy.AllowForceRelease {
		return fmt.Errorf("强制释放功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[req.LockID]
	if !ok {
		return fmt.Errorf("锁不存在: %s", req.LockID)
	}

	if lock.Status != LockStatusActive {
		return fmt.Errorf("锁已非活跃状态: %s", lock.Status)
	}

	now := time.Now()
	lock.Status = LockStatusForceReleased
	lock.ReleasedAt = &now
	lock.ReleasedBy = req.AdminID
	lock.UpdatedAt = now

	m.removeFileLock(lock.FilePath, req.LockID)
	m.addHistoryEntryLocked(lock.FilePath, "force_released", req.AdminID, "管理员", lock.LockType, req.Reason)

	m.logger.Info("lock force released",
		zap.String("lock_id", req.LockID),
		zap.String("file_path", lock.FilePath),
		zap.String("admin_id", req.AdminID),
		zap.String("reason", req.Reason))

	return nil
}

// GetLock 获取锁详情
func (m *Manager) GetLock(lockID string) (*FileLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, ok := m.locks[lockID]
	if !ok {
		return nil, fmt.Errorf("锁不存在: %s", lockID)
	}
	return lock, nil
}

// ListLocks 列出锁
func (m *Manager) ListLocks(req *ListLocksRequest) ([]*FileLock, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FileLock, 0)

	for _, lock := range m.locks {
		// 按文件路径过滤
		if req.FilePath != "" && !strings.HasPrefix(lock.FilePath, req.FilePath) {
			continue
		}
		// 按用户ID过滤
		if req.UserID != "" && lock.UserID != req.UserID {
			continue
		}
		// 按状态过滤
		if req.Status != "" && lock.Status != req.Status {
			continue
		}
		// 按锁类型过滤
		if req.LockType != "" && lock.LockType != req.LockType {
			continue
		}
		result = append(result, lock)
	}

	total := len(result)

	// 分页
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start >= total {
		return []*FileLock{}, total
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return result[start:end], total
}

// GetStats 获取锁定统计信息
func (m *Manager) GetStats() *LockStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LockStats{}
	today := time.Now().Truncate(24 * time.Hour)
	activeUsers := make(map[string]bool)

	for _, lock := range m.locks {
		switch lock.Status {
		case LockStatusActive:
			stats.ActiveLocks++
			activeUsers[lock.UserID] = true
			if lock.LockType == LockTypeExclusive {
				stats.ExclusiveLocks++
			} else {
				stats.SharedLocks++
			}
		case LockStatusExpired:
			stats.ExpiredLocks++
		}
	}

	// 统计今日操作
	for _, entry := range m.history {
		if entry.Timestamp.Before(today) {
			continue
		}
		switch entry.Action {
		case "acquired":
			stats.TodayAcquisitions++
		case "released":
			stats.TodayReleases++
		case "force_released":
			stats.TodayForceReleases++
		case "conflict":
			stats.TodayConflicts++
		}
	}

	stats.ActiveUsers = len(activeUsers)
	return stats
}

// GetHistory 获取锁定历史
func (m *Manager) GetHistory(limit int) []*LockHistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*LockHistoryEntry, limit)
	copy(result, m.history[start:])
	return result
}

// GetPolicy 获取锁定策略
func (m *Manager) GetPolicy() *LockPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy := *m.policy
	return &policy
}

// UpdatePolicy 更新锁定策略
func (m *Manager) UpdatePolicy(policy *LockPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.logger.Info("lock policy updated")
}

// IsFileLocked 检查文件是否被锁定
func (m *Manager) IsFileLocked(filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locks := m.getFileActiveLocks(filePath)
	return len(locks) > 0
}

// GetFileLocks 获取文件的所有活跃锁
func (m *Manager) GetFileLocks(filePath string) []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getFileActiveLocks(filePath)
}

// getUserLocks 获取用户的所有活跃锁
func (m *Manager) getUserLocks(userID string) []*FileLock {
	result := make([]*FileLock, 0)
	for _, lock := range m.locks {
		if lock.UserID == userID && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// getFileActiveLocks 获取文件的所有活跃锁（内部方法，调用者需持有锁）
func (m *Manager) getFileActiveLocks(filePath string) []*FileLock {
	lockIDs, ok := m.fileLocks[filePath]
	if !ok {
		return nil
	}

	result := make([]*FileLock, 0)
	for _, id := range lockIDs {
		if lock, exists := m.locks[id]; exists && lock.Status == LockStatusActive {
			result = append(result, lock)
		}
	}
	return result
}

// addFileLock 添加文件锁映射
func (m *Manager) addFileLock(filePath, lockID string) {
	m.fileLocks[filePath] = append(m.fileLocks[filePath], lockID)
}

// removeFileLock 移除文件锁映射
func (m *Manager) removeFileLock(filePath, lockID string) {
	locks := m.fileLocks[filePath]
	for i, id := range locks {
		if id == lockID {
			m.fileLocks[filePath] = append(locks[:i], locks[i+1:]...)
			break
		}
	}
	if len(m.fileLocks[filePath]) == 0 {
		delete(m.fileLocks, filePath)
	}
}

// addHistoryEntryLocked 添加历史记录（调用者需持有锁）
func (m *Manager) addHistoryEntryLocked(filePath, action, userID, userName string, lockType LockType, detail string) {
	entry := &LockHistoryEntry{
		ID:        uuid.New().String(),
		FilePath:  filePath,
		Action:    action,
		UserID:    userID,
		UserName:  userName,
		LockType:  lockType,
		Detail:    detail,
		Timestamp: time.Now(),
	}
	m.history = append(m.history, entry)
}
