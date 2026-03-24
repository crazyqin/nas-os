package lock

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 锁管理器
type Manager struct {
	config FileLockConfig
	logger *zap.Logger

	// locks 存储所有锁，key为文件路径
	locks sync.Map // map[string]*FileLock

	// locksByID 按ID索引的锁
	locksByID sync.Map // map[string]*FileLock

	// ownerLocks 按持有者索引的锁
	ownerLocks sync.Map // map[string]*sync.Map[lockID]*FileLock

	// waitQueues 等待队列，key为文件路径
	waitQueues sync.Map // map[string][]*LockWaitRequest

	// 审计日志记录器
	auditLogger LockAuditLogger

	// 统计信息
	stats struct {
		mu            sync.RWMutex
		totalLocks    int64
		activeLocks   int64
		expiredLocks  int64
		releasedLocks int64
		conflicts     int64
		preemptions   int64
	}

	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LockAuditLogger 锁审计日志接口
type LockAuditLogger interface {
	LogLockAudit(entry *LockAuditEntry)
}

// NewManager 创建锁管理器
func NewManager(config FileLockConfig, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动后台清理任务
	m.wg.Add(1)
	go m.cleanupLoop()

	// 启动自动续期任务
	if config.EnableAutoRenewal {
		m.wg.Add(1)
		go m.autoRenewalLoop()
	}

	// 启动等待队列处理
	if config.EnableWaitQueue {
		m.wg.Add(1)
		go m.waitQueueLoop()
	}

	return m
}

// SetAuditLogger 设置审计日志记录器
func (m *Manager) SetAuditLogger(logger LockAuditLogger) {
	m.auditLogger = logger
}

// ========== 核心锁操作 ==========

// Lock 尝试获取锁
func (m *Manager) Lock(req *LockRequest) (*FileLock, *LockConflict, error) {
	if req == nil {
		return nil, nil, ErrInvalidLockType
	}

	// 设置默认值
	if req.LockMode == 0 {
		req.LockMode = LockModeManual
	}
	if req.Priority == 0 {
		req.Priority = PriorityNormal
	}
	if req.ConflictStrategy == 0 {
		req.ConflictStrategy = m.config.DefaultConflictStrategy
	}

	// 检查是否超过最大锁数量
	if m.config.MaxTotalLocks > 0 {
		m.stats.mu.RLock()
		activeCount := m.stats.activeLocks
		m.stats.mu.RUnlock()
		if activeCount >= int64(m.config.MaxTotalLocks) {
			return nil, nil, ErrMaxLocksExceeded
		}
	}

	// 设置超时时间
	timeout := m.config.DefaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > m.config.MaxTimeout {
			timeout = m.config.MaxTimeout
		}
	}

	now := time.Now()
	lock := &FileLock{
		ID:              uuid.New().String(),
		FilePath:        req.FilePath,
		FileName:        filepath.Base(req.FilePath),
		LockType:        req.LockType,
		LockMode:        req.LockMode,
		Status:          LockStatusActive,
		Priority:        req.Priority,
		Owner:           req.Owner,
		OwnerName:       req.OwnerName,
		OwnerEmail:      req.OwnerEmail,
		ClientID:        req.ClientID,
		SessionID:       req.SessionID,
		Protocol:        req.Protocol,
		ShareName:       req.ShareName,
		CreatedAt:       now,
		ExpiresAt:       now.Add(timeout),
		LastAccessed:    now,
		ConflictStrategy: req.ConflictStrategy,
		Metadata:        req.Metadata,
		Reason:          req.Reason,
		AppName:         req.AppName,
		Version:         1,
	}

	// 处理共享锁持有者
	if req.LockType == LockTypeShared {
		lock.SharedOwners = []*SharedOwner{
			{
				Owner:      req.Owner,
				OwnerName:  req.OwnerName,
				ClientID:   req.ClientID,
				Protocol:   req.Protocol,
				AcquiredAt: now,
				ExpiresAt:  now.Add(timeout),
			},
		}
	}

	// 检查现有锁
	existingRaw, loaded := m.locks.Load(req.FilePath)
	if loaded {
		existing, ok := existingRaw.(*FileLock)
		if !ok {
			return nil, nil, ErrInvalidLockType
		}

		// 检查现有锁是否过期
		if existing.IsExpired() {
			m.releaseLockInternal(existing, "expired")
		} else {
			// 检查锁兼容性
			conflict := m.checkConflict(existing, req, lock)
			if conflict != nil {
				// 记录冲突审计
				m.logAudit(&LockAuditEntry{
					Event:     AuditEventLockConflict,
					FilePath:  req.FilePath,
					FileName:  lock.FileName,
					LockType:  req.LockType.String(),
					Owner:     req.Owner,
					OwnerName: req.OwnerName,
					ClientID:  req.ClientID,
					Protocol:  req.Protocol,
					ConflictWith: conflict.ExistingLock,
				})

				// 处理冲突策略
				return m.handleConflict(req, lock, existing, conflict)
			}

			// 如果现有锁是共享锁，且新请求也是共享锁，添加到现有锁
			if existing.LockType == LockTypeShared && req.LockType == LockTypeShared {
				existing.AddSharedOwner(&SharedOwner{
					Owner:      req.Owner,
					OwnerName:  req.OwnerName,
					ClientID:   req.ClientID,
					Protocol:   req.Protocol,
					AcquiredAt: now,
					ExpiresAt:  now.Add(timeout),
				})

				// 记录审计日志
				m.logAudit(&LockAuditEntry{
					Event:     AuditEventLockAcquired,
					LockID:    existing.ID,
					FilePath:  req.FilePath,
					FileName:  existing.FileName,
					LockType:  req.LockType.String(),
					Owner:     req.Owner,
					OwnerName: req.OwnerName,
					ClientID:  req.ClientID,
					Protocol:  req.Protocol,
				})

				return existing, nil, nil
			}
		}
	}

	// 存储锁
	m.storeLock(lock)

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockAcquired,
		LockID:    lock.ID,
		FilePath:  req.FilePath,
		FileName:  lock.FileName,
		LockType:  req.LockType.String(),
		Owner:     req.Owner,
		OwnerName: req.OwnerName,
		ClientID:  req.ClientID,
		Protocol:  req.Protocol,
	})

	m.logger.Debug("lock acquired",
		zap.String("id", lock.ID),
		zap.String("file", req.FilePath),
		zap.String("type", req.LockType.String()),
		zap.String("owner", req.Owner),
	)

	return lock, nil, nil
}

// Unlock 释放锁
func (m *Manager) Unlock(lockID string, owner string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	// 验证持有者
	if lock.Owner != owner {
		return ErrNotLockOwner
	}

	// 计算锁持有时长
	duration := time.Since(lock.CreatedAt)

	m.releaseLockInternal(lock, "released")

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockReleased,
		LockID:    lock.ID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  lock.LockType.String(),
		Owner:     owner,
		OwnerName: lock.OwnerName,
		ClientID:  lock.ClientID,
		Protocol:  lock.Protocol,
		Duration:  duration.Milliseconds(),
	})

	m.logger.Debug("lock released",
		zap.String("id", lockID),
		zap.String("file", lock.FilePath),
		zap.String("owner", owner),
	)

	// 处理等待队列
	m.processWaitQueue(lock.FilePath)

	return nil
}

// UnlockByPath 通过路径释放锁
func (m *Manager) UnlockByPath(filePath string, owner string) error {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	// 验证持有者
	if lock.Owner != owner {
		return ErrNotLockOwner
	}

	return m.Unlock(lock.ID, owner)
}

// ForceUnlock 强制释放锁（管理员操作）
func (m *Manager) ForceUnlock(lockID string, reason string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	originalOwner := lock.Owner
	duration := time.Since(lock.CreatedAt)

	m.releaseLockInternal(lock, "force_released")

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockForceReleased,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  lock.LockType.String(),
		Owner:     originalOwner,
		OwnerName: lock.OwnerName,
		Duration:  duration.Milliseconds(),
		Reason:    reason,
	})

	m.logger.Info("lock force released",
		zap.String("id", lockID),
		zap.String("file", lock.FilePath),
		zap.String("owner", originalOwner),
		zap.String("reason", reason),
	)

	// 处理等待队列
	m.processWaitQueue(lock.FilePath)

	return nil
}

// ExtendLock 延长锁有效期
func (m *Manager) ExtendLock(lockID string, owner string, duration time.Duration) error {
	if duration > m.config.MaxTimeout {
		duration = m.config.MaxTimeout
	}

	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	// 验证持有者
	if lock.Owner != owner {
		return ErrNotLockOwner
	}

	// 检查是否已过期
	if lock.IsExpired() {
		return ErrLockExpired
	}

	lock.Extend(duration)

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockExtended,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  lock.LockType.String(),
		Owner:     owner,
		OwnerName: lock.OwnerName,
		ClientID:  lock.ClientID,
		Protocol:  lock.Protocol,
	})

	m.logger.Debug("lock extended",
		zap.String("id", lockID),
		zap.Duration("duration", duration),
	)

	return nil
}

// ========== 锁升级/降级 ==========

// UpgradeLock 升级锁（共享锁 -> 独占锁）
func (m *Manager) UpgradeLock(lockID string, owner string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	// 验证持有者
	if lock.Owner != owner {
		return ErrNotLockOwner
	}

	// 检查是否已过期
	if lock.IsExpired() {
		return ErrLockExpired
	}

	// 检查是否有其他共享锁持有者（大于1表示有其他人）
	// 只有当前用户一个共享者时可以升级 - 在Upgrade方法中判断
	if len(lock.SharedOwners) > 1 {
		return ErrLockUpgradeFailed
	}

	// 如果只有一个共享者，检查是否是当前用户
	if len(lock.SharedOwners) == 1 {
		// 找到当前用户的共享记录
		found := false
		for _, so := range lock.SharedOwners {
			if so.Owner == owner {
				found = true
				break
			}
		}
		if !found {
			return ErrLockUpgradeFailed
		}
	}

	// 清空共享者列表（升级后变为独占锁）
	lock.SharedOwners = nil

	err := lock.Upgrade()
	if err != nil {
		return err
	}

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockUpgraded,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  "exclusive",
		Owner:     owner,
		OwnerName: lock.OwnerName,
		ClientID:  lock.ClientID,
		Protocol:  lock.Protocol,
	})

	return nil
}

// DowngradeLock 降级锁（独占锁 -> 共享锁）
func (m *Manager) DowngradeLock(lockID string, owner string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}

	// 验证持有者
	if lock.Owner != owner {
		return ErrNotLockOwner
	}

	// 检查是否已过期
	if lock.IsExpired() {
		return ErrLockExpired
	}

	lock.Downgrade()

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockDowngraded,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  "shared",
		Owner:     owner,
		OwnerName: lock.OwnerName,
		ClientID:  lock.ClientID,
		Protocol:  lock.Protocol,
	})

	return nil
}

// ========== 锁查询 ==========

// GetLock 获取锁信息
func (m *Manager) GetLock(lockID string) (*LockInfo, error) {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return nil, ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return nil, ErrInvalidLockType
	}

	// 检查是否过期
	if lock.IsExpired() {
		m.releaseLockInternal(lock, "expired")
		return nil, ErrLockExpired
	}

	return lock.ToInfo(), nil
}

// GetLockByPath 通过路径获取锁信息
func (m *Manager) GetLockByPath(filePath string) (*LockInfo, error) {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return nil, ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return nil, ErrInvalidLockType
	}

	// 检查是否过期
	if lock.IsExpired() {
		m.releaseLockInternal(lock, "expired")
		return nil, ErrLockExpired
	}

	return lock.ToInfo(), nil
}

// IsLocked 检查文件是否被锁定
func (m *Manager) IsLocked(filePath string) bool {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return false
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return false
	}

	// 检查是否过期
	if lock.IsExpired() {
		m.releaseLockInternal(lock, "expired")
		return false
	}

	return lock.Status == LockStatusActive
}

// CanAcquire 检查是否可以获取锁
func (m *Manager) CanAcquire(filePath string, lockType LockType, owner string) (*LockConflict, bool) {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return nil, true
	}

	existing, ok := raw.(*FileLock)
	if !ok {
		return nil, true
	}

	// 检查是否过期
	if existing.IsExpired() {
		m.releaseLockInternal(existing, "expired")
		return nil, true
	}

	// 同一用户可以升级锁
	if existing.Owner == owner {
		return nil, true
	}

	// 检查锁兼容性
	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		Owner:    owner,
	}

	conflict := m.checkConflictSimple(existing, req)
	return conflict, conflict == nil
}

// ListLocks 列出所有锁
func (m *Manager) ListLocks(filter *LockFilter) []*LockInfo {
	var result []*LockInfo

	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}

		// 跳过过期锁
		if lock.IsExpired() {
			return true
		}

		// 应用过滤器
		if filter != nil {
			if filter.Owner != "" && lock.Owner != filter.Owner {
				return true
			}
			if filter.LockType != 0 && lock.LockType != filter.LockType {
				return true
			}
			if filter.Status != 0 && lock.Status != filter.Status {
				return true
			}
			if filter.Protocol != "" && lock.Protocol != filter.Protocol {
				return true
			}
			if filter.LockMode != 0 && lock.LockMode != filter.LockMode {
				return true
			}
		}

		result = append(result, lock.ToInfo())
		return true
	})

	return result
}

// ListLocksByOwner 列出指定用户的所有锁
func (m *Manager) ListLocksByOwner(owner string) []*LockInfo {
	var result []*LockInfo

	raw, ok := m.ownerLocks.Load(owner)
	if !ok {
		return result
	}

	ownerLocks, ok := raw.(*sync.Map)
	if !ok {
		return result
	}

	ownerLocks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}
		if !lock.IsExpired() && lock.Status == LockStatusActive {
			result = append(result, lock.ToInfo())
		}
		return true
	})

	return result
}

// ========== 统计信息 ==========

// Stats 获取统计信息
func (m *Manager) Stats() ManagerStats {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	var activeCount int64
	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}
		if !lock.IsExpired() && lock.Status == LockStatusActive {
			activeCount++
		}
		return true
	})

	return ManagerStats{
		TotalLocks:    m.stats.totalLocks,
		ActiveLocks:   activeCount,
		ExpiredLocks:  m.stats.expiredLocks,
		ReleasedLocks: m.stats.releasedLocks,
		Conflicts:     m.stats.conflicts,
		Preemptions:   m.stats.preemptions,
	}
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalLocks    int64 `json:"totalLocks"`
	ActiveLocks   int64 `json:"activeLocks"`
	ExpiredLocks  int64 `json:"expiredLocks"`
	ReleasedLocks int64 `json:"releasedLocks"`
	Conflicts     int64 `json:"conflicts"`
	Preemptions   int64 `json:"preemptions"`
}

// ========== 内部方法 ==========

// storeLock 存储锁
func (m *Manager) storeLock(lock *FileLock) {
	m.locks.Store(lock.FilePath, lock)
	m.locksByID.Store(lock.ID, lock)
	m.addToOwnerIndex(lock)

	m.stats.mu.Lock()
	m.stats.totalLocks++
	m.stats.activeLocks++
	m.stats.mu.Unlock()
}

// checkConflict 检查锁冲突
func (m *Manager) checkConflict(existing *FileLock, req *LockRequest, newLock *FileLock) *LockConflict {
	// 同一用户（不同客户端）特殊处理
	if existing.Owner == req.Owner {
		// 同一客户端，可以重新获取
		if existing.ClientID == req.ClientID && existing.ClientID != "" {
			return nil
		}
		// 不同客户端，根据策略处理
		if req.LockType == LockTypeExclusive {
			return &LockConflict{
				ConflictType: ConflictTypeOwner,
				ExistingLock: existing.ToInfo(),
				Message:      fmt.Sprintf("file is locked by another session of user %s", existing.OwnerName),
				Resolution:   "close the file in other sessions or contact the other session",
				CanWait:      false,
				CanPreempt:   false,
			}
		}
		return nil // 共享锁允许同一用户多客户端
	}

	// 独占锁与任何锁都冲突
	if existing.LockType == LockTypeExclusive {
		return &LockConflict{
			ConflictType: ConflictTypeExclusive,
			ExistingLock: existing.ToInfo(),
			Message:      fmt.Sprintf("file is exclusively locked by %s", existing.OwnerName),
			Resolution:   "wait for the lock to be released or contact the lock owner",
			CanWait:      m.config.EnableWaitQueue,
			CanPreempt:   m.config.EnablePreemption && req.Priority > existing.Priority,
			EstimatedWait: int64(time.Until(existing.ExpiresAt).Seconds()),
		}
	}

	// 新请求是独占锁，与现有共享锁冲突
	if req.LockType == LockTypeExclusive {
		// 检查共享锁数量是否超过限制
		if len(existing.SharedOwners) == 1 {
			return &LockConflict{
				ConflictType: ConflictTypeShared,
				ExistingLock: existing.ToInfo(),
				Message:      fmt.Sprintf("file is shared by %s, cannot acquire exclusive lock", existing.OwnerName),
				Resolution:   "wait for shared locks to be released",
				CanWait:      m.config.EnableWaitQueue,
				CanPreempt:   m.config.EnablePreemption && req.Priority > existing.Priority,
			}
		}
		return &LockConflict{
			ConflictType: ConflictTypeShared,
			ExistingLock: existing.ToInfo(),
			Message:      fmt.Sprintf("file is shared by %d users, cannot acquire exclusive lock", len(existing.SharedOwners)+1),
			Resolution:   "wait for all shared locks to be released",
			CanWait:      m.config.EnableWaitQueue,
			CanPreempt:   false, // 多人共享时不能抢占
		}
	}

	// 检查共享锁数量限制
	if len(existing.SharedOwners) >= m.config.MaxLocksPerFile {
		return &LockConflict{
			ConflictType: ConflictTypeShared,
			ExistingLock: existing.ToInfo(),
			Message:      fmt.Sprintf("maximum shared locks (%d) reached for this file", m.config.MaxLocksPerFile),
			Resolution:   "wait for existing shared locks to be released",
			CanWait:      true,
			CanPreempt:   false,
		}
	}

	return nil // 共享锁之间兼容
}

// checkConflictSimple 简单冲突检查
func (m *Manager) checkConflictSimple(existing *FileLock, req *LockRequest) *LockConflict {
	return m.checkConflict(existing, req, nil)
}

// handleConflict 处理锁冲突
func (m *Manager) handleConflict(req *LockRequest, newLock *FileLock, existing *FileLock, conflict *LockConflict) (*FileLock, *LockConflict, error) {
	m.stats.mu.Lock()
	m.stats.conflicts++
	m.stats.mu.Unlock()

	// 强制获取锁（管理员操作）
	if req.Force && m.config.EnablePreemption {
		m.preemptLock(existing, req.Owner)
		m.storeLock(newLock)
		return newLock, nil, nil
	}

	// 抢占模式
	if conflict.CanPreempt && req.ConflictStrategy == ConflictStrategyPreempt {
		m.preemptLock(existing, req.Owner)
		m.storeLock(newLock)
		return newLock, nil, nil
	}

	// 等待模式
	if conflict.CanWait && req.ConflictStrategy == ConflictStrategyWait && req.WaitTimeout > 0 {
		return m.waitForLock(req, newLock)
	}

	// 通知模式
	if req.ConflictStrategy == ConflictStrategyNotify {
		m.notifyLockOwner(existing, req)
	}

	// 默认拒绝
	return nil, conflict, ErrLockConflict
}

// preemptLock 抢占锁
func (m *Manager) preemptLock(lock *FileLock, preemptor string) {
	m.releaseLockInternal(lock, "preempted")

	m.stats.mu.Lock()
	m.stats.preemptions++
	m.stats.mu.Unlock()

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventLockPreempted,
		LockID:    lock.ID,
		FilePath:  lock.FilePath,
		FileName:  lock.FileName,
		LockType:  lock.LockType.String(),
		Owner:     lock.Owner,
		OwnerName: lock.OwnerName,
		Reason:    fmt.Sprintf("preempted by %s", preemptor),
	})

	m.logger.Info("lock preempted",
		zap.String("id", lock.ID),
		zap.String("file", lock.FilePath),
		zap.String("originalOwner", lock.Owner),
		zap.String("preemptor", preemptor),
	)
}

// waitForLock 等待锁
func (m *Manager) waitForLock(req *LockRequest, newLock *FileLock) (*FileLock, *LockConflict, error) {
	waitReq := &LockWaitRequest{
		ID:          uuid.New().String(),
		FilePath:    req.FilePath,
		LockType:    req.LockType,
		Owner:       req.Owner,
		OwnerName:   req.OwnerName,
		ClientID:    req.ClientID,
		Protocol:    req.Protocol,
		RequestedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(req.WaitTimeout) * time.Second),
		Priority:    req.Priority,
		NotifyChan:  make(chan *FileLock, 1),
		CancelChan:  make(chan struct{}),
	}

	// 添加到等待队列
	raw, _ := m.waitQueues.LoadOrStore(req.FilePath, &[]*LockWaitRequest{})
	queue := raw.(*[]*LockWaitRequest)
	*queue = append(*queue, waitReq)

	// 记录审计日志
	m.logAudit(&LockAuditEntry{
		Event:     AuditEventWaitQueued,
		FilePath:  req.FilePath,
		FileName:  newLock.FileName,
		LockType:  req.LockType.String(),
		Owner:     req.Owner,
		OwnerName: req.OwnerName,
		ClientID:  req.ClientID,
		Protocol:  req.Protocol,
	})

	// 等待通知或超时
	timeout := time.Duration(req.WaitTimeout) * time.Second
	select {
	case acquiredLock := <-waitReq.NotifyChan:
		return acquiredLock, nil, nil
	case <-time.After(timeout):
		// 从等待队列移除
		m.removeFromWaitQueue(req.FilePath, waitReq.ID)
		// 记录超时审计
		m.logAudit(&LockAuditEntry{
			Event:     AuditEventWaitTimeout,
			FilePath:  req.FilePath,
			FileName:  newLock.FileName,
			LockType:  req.LockType.String(),
			Owner:     req.Owner,
			OwnerName: req.OwnerName,
		})
		return nil, &LockConflict{
			ConflictType: ConflictTypeTimeout,
			Message:      "wait for lock timed out",
		}, ErrLockTimeout
	case <-waitReq.CancelChan:
		return nil, nil, ErrInvalidOperation
	case <-m.ctx.Done():
		return nil, nil, m.ctx.Err()
	}
}

// notifyLockOwner 通知锁持有者
func (m *Manager) notifyLockOwner(lock *FileLock, req *LockRequest) {
	// TODO: 实现通知机制（通过WebSocket、消息队列等）
	m.logger.Info("notifying lock owner",
		zap.String("lockId", lock.ID),
		zap.String("owner", lock.Owner),
		zap.String("requester", req.Owner),
	)
}

// releaseLockInternal 内部释放锁
func (m *Manager) releaseLockInternal(lock *FileLock, reason string) {
	// 更新状态
	switch reason {
	case "expired":
		lock.Status = LockStatusExpired
	case "force_released", "preempted":
		lock.Status = LockStatusReleased
	default:
		lock.Status = LockStatusReleased
	}

	// 从索引中删除
	m.locks.Delete(lock.FilePath)
	m.locksByID.Delete(lock.ID)
	m.removeFromOwnerIndex(lock)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.activeLocks--
	if reason == "expired" {
		m.stats.expiredLocks++
	} else {
		m.stats.releasedLocks++
	}
	m.stats.mu.Unlock()
}

// addToOwnerIndex 添加到持有者索引
func (m *Manager) addToOwnerIndex(lock *FileLock) {
	raw, _ := m.ownerLocks.LoadOrStore(lock.Owner, &sync.Map{})
	ownerLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	ownerLocks.Store(lock.ID, lock)
}

// removeFromOwnerIndex 从持有者索引移除
func (m *Manager) removeFromOwnerIndex(lock *FileLock) {
	raw, ok := m.ownerLocks.Load(lock.Owner)
	if !ok {
		return
	}
	ownerLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	ownerLocks.Delete(lock.ID)
}

// removeFromWaitQueue 从等待队列移除
func (m *Manager) removeFromWaitQueue(filePath string, waitID string) {
	raw, ok := m.waitQueues.Load(filePath)
	if !ok {
		return
	}
	queue := raw.(*[]*LockWaitRequest)
	for i, req := range *queue {
		if req.ID == waitID {
			*queue = append((*queue)[:i], (*queue)[i+1:]...)
			return
		}
	}
}

// processWaitQueue 处理等待队列
func (m *Manager) processWaitQueue(filePath string) {
	if !m.config.EnableWaitQueue {
		return
	}

	raw, ok := m.waitQueues.Load(filePath)
	if !ok {
		return
	}
	queue := raw.(*[]*LockWaitRequest)
	if len(*queue) == 0 {
		return
	}

	// 检查是否可以获取锁
	next := (*queue)[0]
	conflict, canAcquire := m.CanAcquire(filePath, next.LockType, next.Owner)
	if canAcquire {
		// 创建锁
		lock, _, _ := m.Lock(&LockRequest{
			FilePath:  filePath,
			LockType:  next.LockType,
			Owner:     next.Owner,
			OwnerName: next.OwnerName,
			ClientID:  next.ClientID,
			Protocol:  next.Protocol,
		})
		if lock != nil {
			// 通知等待者
			select {
			case next.NotifyChan <- lock:
			default:
			}
			// 从队列移除
			*queue = (*queue)[1:]
		}
	} else if conflict != nil {
		// 检查等待是否超时
		if time.Now().After(next.ExpiresAt) {
			*queue = (*queue)[1:]
		}
	}
}

// logAudit 记录审计日志
func (m *Manager) logAudit(entry *LockAuditEntry) {
	if !m.config.EnableAudit {
		return
	}

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 调用审计日志记录器
	if m.auditLogger != nil {
		m.auditLogger.LogLockAudit(entry)
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.cancel()
	m.wg.Wait()
	m.logger.Info("lock manager closed")
}

// ========== 后台任务 ==========

func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpiredLocks()
		}
	}
}

func (m *Manager) cleanupExpiredLocks() {
	var expiredLocks []*FileLock

	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}
		if lock.IsExpired() && lock.Status == LockStatusActive {
			expiredLocks = append(expiredLocks, lock)
		}
		return true
	})

	for _, lock := range expiredLocks {
		m.releaseLockInternal(lock, "expired")

		// 记录审计日志
		m.logAudit(&LockAuditEntry{
			Event:     AuditEventLockExpired,
			LockID:    lock.ID,
			FilePath:  lock.FilePath,
			FileName:  lock.FileName,
			LockType:  lock.LockType.String(),
			Owner:     lock.Owner,
			OwnerName: lock.OwnerName,
			Duration:  time.Since(lock.CreatedAt).Milliseconds(),
		})

		m.logger.Debug("lock expired",
			zap.String("id", lock.ID),
			zap.String("file", lock.FilePath),
			zap.String("owner", lock.Owner),
		)

		// 处理等待队列
		m.processWaitQueue(lock.FilePath)
	}
}

func (m *Manager) autoRenewalLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.AutoRenewalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.renewActiveLocks()
		}
	}
}

func (m *Manager) renewActiveLocks() {
	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}

		// 只续期活跃的锁
		if lock.Status != LockStatusActive {
			return true
		}

		// 如果锁即将过期（剩余时间小于配置的续期间隔）
		remaining := time.Until(lock.ExpiresAt)
		if remaining < m.config.AutoRenewalInterval {
			lock.Extend(m.config.DefaultTimeout)

			m.logger.Debug("lock auto-renewed",
				zap.String("id", lock.ID),
				zap.String("file", lock.FilePath),
			)
		}

		return true
	})
}

func (m *Manager) waitQueueLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.processAllWaitQueues()
		}
	}
}

func (m *Manager) processAllWaitQueues() {
	m.waitQueues.Range(func(key, value interface{}) bool {
		filePath, ok := key.(string)
		if !ok {
			return true
		}
		m.processWaitQueue(filePath)
		return true
	})
}

// LockFilter 锁过滤器
type LockFilter struct {
	Owner    string
	LockType LockType
	Status   LockStatus
	Protocol string
	LockMode LockMode
}