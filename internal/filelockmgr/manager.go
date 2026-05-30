package filelockmgr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 预定义错误
var (
	ErrLockNotFound      = errors.New("锁不存在")
	ErrLockExpired       = errors.New("锁已过期")
	ErrLockConflict      = errors.New("文件已被锁定，存在冲突")
	ErrUpgradeNotAllowed = errors.New("不允许升级锁")
	ErrLockNotShared     = errors.New("只能将共享锁升级为独占锁")
	ErrMaxLocksExceeded  = errors.New("用户锁数量已达上限")
	ErrInvalidDuration   = errors.New("锁持续时间无效")
	ErrUnauthorized      = errors.New("未授权操作")
	ErrDuplicateLock     = errors.New("重复的锁请求")
)

// Manager 文件锁管理器
type Manager struct {
	mu          sync.RWMutex
	locks       map[string]*FileLockEntry   // lockID -> entry
	fileLocks   map[string][]string         // filePath -> []lockID
	userLocks   map[string][]string         // userID -> []lockID
	conflicts   map[string]*LockConflict    // conflictID -> conflict
	policy      LockPolicy
	storagePath string
}

// NewManager 创建文件锁管理器
func NewManager(storagePath string) *Manager {
	m := &Manager{
		locks:       make(map[string]*FileLockEntry),
		fileLocks:   make(map[string][]string),
		userLocks:   make(map[string][]string),
		conflicts:   make(map[string]*LockConflict),
		storagePath: storagePath,
		policy: LockPolicy{
			MaxLockDuration:  24 * time.Hour,
			AutoExpire:       true,
			AllowUpgrade:     true,
			MaxLocksPerUser:  10,
			ConflictStrategy: StrategyReject,
		},
	}
	m.loadFromDisk()
	return m
}

// loadFromDisk 从磁盘加载锁数据
func (m *Manager) loadFromDisk() {
	if m.storagePath == "" {
		return
	}
	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return
	}
	var locks []*FileLockEntry
	if err := json.Unmarshal(data, &locks); err != nil {
		return
	}
	for _, lock := range locks {
		m.locks[lock.ID] = lock
		m.fileLocks[lock.FilePath] = append(m.fileLocks[lock.FilePath], lock.ID)
		m.userLocks[lock.LockedBy] = append(m.userLocks[lock.LockedBy], lock.ID)
	}
}

// saveToDisk 保存锁数据到磁盘
func (m *Manager) saveToDisk() error {
	if m.storagePath == "" {
		return nil
	}
	locks := make([]*FileLockEntry, 0, len(m.locks))
	for _, lock := range m.locks {
		locks = append(locks, lock)
	}
	data, err := json.MarshalIndent(locks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.storagePath, data, 0644)
}

// AcquireLock 获取锁
func (m *Manager) AcquireLock(ctx context.Context, req LockRequest) (*FileLockEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户锁数量限制
	userLocks := m.userLocks[req.LockedBy]
	if len(userLocks) >= m.policy.MaxLocksPerUser {
		return nil, ErrMaxLocksExceeded
	}

	// 设置锁持续时间
	duration := time.Duration(req.Duration) * time.Second
	if duration <= 0 {
		duration = 30 * time.Minute
	}
	if duration > m.policy.MaxLockDuration {
		duration = m.policy.MaxLockDuration
	}

	now := time.Now()
	entry := &FileLockEntry{
		ID:           uuid.New().String(),
		FilePath:     req.FilePath,
		LockType:     req.LockType,
		LockedBy:     req.LockedBy,
		LockedByName: req.LockedByName,
		Reason:       req.Reason,
		ExpiresAt:    now.Add(duration),
		CreatedAt:    now,
		Priority:     req.Priority,
	}

	// 检查冲突
	existingLocks := m.fileLocks[req.FilePath]
	for _, lockID := range existingLocks {
		existing := m.locks[lockID]
		if existing == nil || now.After(existing.ExpiresAt) {
			continue
		}

		// 同一用户重复锁定同一文件
		if existing.LockedBy == req.LockedBy {
			if req.UpgradeFrom != "" && existing.ID == req.UpgradeFrom {
				break
			}
			return nil, ErrDuplicateLock
		}

		// 独占锁冲突
		if existing.LockType == LockTypeExclusive || req.LockType == LockTypeExclusive {
			conflict := &LockConflict{
				ID:              uuid.New().String(),
				FilePath:        req.FilePath,
				RequesterID:     req.LockedBy,
				CurrentHolderID: existing.LockedBy,
				ConflictType:    "exclusive_conflict",
			}
			m.conflicts[conflict.ID] = conflict

			if m.policy.ConflictStrategy == StrategyReject {
				return nil, ErrLockConflict
			}
		}
	}

	// 添加锁
	m.locks[entry.ID] = entry
	m.fileLocks[req.FilePath] = append(m.fileLocks[req.FilePath], entry.ID)
	m.userLocks[req.LockedBy] = append(m.userLocks[req.LockedBy], entry.ID)

	if err := m.saveToDisk(); err != nil {
		// 回滚
		delete(m.locks, entry.ID)
		m.fileLocks[req.FilePath] = removeID(m.fileLocks[req.FilePath], entry.ID)
		m.userLocks[req.LockedBy] = removeID(m.userLocks[req.LockedBy], entry.ID)
		return nil, err
	}

	return entry, nil
}

// ReleaseLock 释放锁
func (m *Manager) ReleaseLock(ctx context.Context, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}

	delete(m.locks, lockID)
	m.fileLocks[lock.FilePath] = removeID(m.fileLocks[lock.FilePath], lockID)
	m.userLocks[lock.LockedBy] = removeID(m.userLocks[lock.LockedBy], lockID)

	return m.saveToDisk()
}

// UpgradeLock 升级锁（共享锁升级为独占锁）
func (m *Manager) UpgradeLock(ctx context.Context, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.policy.AllowUpgrade {
		return ErrUpgradeNotAllowed
	}

	lock, exists := m.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}

	if lock.LockType != LockTypeShared {
		return ErrLockNotShared
	}

	// 检查是否有其他共享锁
	fileLocks := m.fileLocks[lock.FilePath]
	for _, otherID := range fileLocks {
		if otherID == lockID {
			continue
		}
		other := m.locks[otherID]
		if other == nil || time.Now().After(other.ExpiresAt) {
			continue
		}
		// 有其他活跃锁，不能升级
		return ErrLockConflict
	}

	lock.LockType = LockTypeExclusive
	return m.saveToDisk()
}

// RefreshLock 续期锁
func (m *Manager) RefreshLock(ctx context.Context, lockID string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}

	if time.Now().After(lock.ExpiresAt) {
		return ErrLockExpired
	}

	if duration <= 0 {
		return ErrInvalidDuration
	}
	if duration > m.policy.MaxLockDuration {
		duration = m.policy.MaxLockDuration
	}

	lock.ExpiresAt = time.Now().Add(duration)
	return m.saveToDisk()
}

// GetLocksByFile 获取文件的所有锁
func (m *Manager) GetLocksByFile(ctx context.Context, filePath string) []FileLockEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []FileLockEntry
	lockIDs := m.fileLocks[filePath]
	now := time.Now()

	for _, lockID := range lockIDs {
		lock := m.locks[lockID]
		if lock != nil && now.Before(lock.ExpiresAt) {
			result = append(result, *lock)
		}
	}
	return result
}

// GetLocksByUser 获取用户的所有锁
func (m *Manager) GetLocksByUser(ctx context.Context, userID string) []FileLockEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []FileLockEntry
	lockIDs := m.userLocks[userID]
	now := time.Now()

	for _, lockID := range lockIDs {
		lock := m.locks[lockID]
		if lock != nil && now.Before(lock.ExpiresAt) {
			result = append(result, *lock)
		}
	}
	return result
}

// DetectConflict 冲突检测
func (m *Manager) DetectConflict(ctx context.Context, filePath string, userID string) (*LockConflict, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lockIDs := m.fileLocks[filePath]
	now := time.Now()

	for _, lockID := range lockIDs {
		lock := m.locks[lockID]
		if lock == nil || now.After(lock.ExpiresAt) {
			continue
		}
		if lock.LockedBy != userID && lock.LockType == LockTypeExclusive {
			return &LockConflict{
				ID:              uuid.New().String(),
				FilePath:        filePath,
				RequesterID:     userID,
				CurrentHolderID: lock.LockedBy,
				ConflictType:    "exclusive_conflict",
			}, nil
		}
	}
	return nil, nil
}

// ForceRelease 管理员强制释放锁
func (m *Manager) ForceRelease(ctx context.Context, lockID string, adminID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}

	// 记录强制释放冲突
	conflict := &LockConflict{
		ID:              uuid.New().String(),
		FilePath:        lock.FilePath,
		RequesterID:     adminID,
		CurrentHolderID: lock.LockedBy,
		ConflictType:    "force_release",
		ResolvedAt:      time.Now(),
		Resolution:      fmt.Sprintf("管理员 %s 强制释放", adminID),
	}
	m.conflicts[conflict.ID] = conflict

	delete(m.locks, lockID)
	m.fileLocks[lock.FilePath] = removeID(m.fileLocks[lock.FilePath], lockID)
	m.userLocks[lock.LockedBy] = removeID(m.userLocks[lock.LockedBy], lockID)

	return m.saveToDisk()
}

// GetStats 获取锁统计
func (m *Manager) GetStats(ctx context.Context) LockStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := LockStats{}
	now := time.Now()
	var totalDuration time.Duration
	activeCount := 0

	for _, lock := range m.locks {
		if now.After(lock.ExpiresAt) {
			continue
		}
		activeCount++
		stats.TotalLocks++
		if lock.LockType == LockTypeExclusive {
			stats.ExclusiveLocks++
		} else {
			stats.SharedLocks++
		}
		totalDuration += now.Sub(lock.CreatedAt)
	}

	if activeCount > 0 {
		stats.AvgLockDuration = totalDuration.Seconds() / float64(activeCount)
	}

	// 统计待处理冲突
	for _, conflict := range m.conflicts {
		if conflict.ResolvedAt.IsZero() {
			stats.PendingConflicts++
		}
	}

	return stats
}

// CleanupExpired 清理过期锁，返回清理数量
func (m *Manager) CleanupExpired(ctx context.Context) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expired []string

	for lockID, lock := range m.locks {
		if now.After(lock.ExpiresAt) {
			expired = append(expired, lockID)
		}
	}

	for _, lockID := range expired {
		lock := m.locks[lockID]
		delete(m.locks, lockID)
		m.fileLocks[lock.FilePath] = removeID(m.fileLocks[lock.FilePath], lockID)
		m.userLocks[lock.LockedBy] = removeID(m.userLocks[lock.LockedBy], lockID)
	}

	if len(expired) > 0 {
		m.saveToDisk()
	}

	return len(expired)
}

// removeID 从切片中移除指定ID
func removeID(ids []string, target string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != target {
			result = append(result, id)
		}
	}
	return result
}
