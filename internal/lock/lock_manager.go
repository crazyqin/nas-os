package lock

import (
	"context"
	"fmt"
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
	ownerLocks sync.Map // map[string]map[string]*FileLock

	// 统计信息
	stats struct {
		mu            sync.RWMutex
		totalLocks    int64
		activeLocks   int64
		expiredLocks  int64
		releasedLocks int64
	}

	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
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

	return m
}

// Lock 尝试获取锁
func (m *Manager) Lock(req *LockRequest) (*FileLock, *LockConflict, error) {
	if req == nil {
		return nil, nil, ErrInvalidLockType
	}

	// 设置默认超时
	timeout := m.config.DefaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > m.config.MaxTimeout {
			timeout = m.config.MaxTimeout
		}
	}

	now := time.Now()
	lock := &FileLock{
		ID:           uuid.New().String(),
		FilePath:     req.FilePath,
		LockType:     req.LockType,
		Status:       LockStatusActive,
		Owner:        req.Owner,
		OwnerName:    req.OwnerName,
		ClientID:     req.ClientID,
		Protocol:     req.Protocol,
		CreatedAt:    now,
		ExpiresAt:    now.Add(timeout),
		LastAccessed: now,
		Metadata:     req.Metadata,
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
			// 过期锁自动释放
			m.releaseLockInternal(existing)
		} else {
			// 检查锁兼容性
			if conflict := m.checkConflict(existing, req); conflict != nil {
				return nil, conflict, ErrLockConflict
			}
		}
	}

	// 对于独占锁，直接替换
	if req.LockType == LockTypeExclusive {
		// 如果存在共享锁，需要检查是否都是同一用户
		if existingRaw != nil {
			existing, ok := existingRaw.(*FileLock)
			if !ok {
				return nil, nil, ErrInvalidLockType
			}
			if !existing.IsExpired() && existing.Owner != req.Owner {
				return nil, &LockConflict{
					ExistingLock: existing.ToInfo(),
					Message:      "file is locked by another user",
				}, ErrLockConflict
			}
			m.releaseLockInternal(existing)
		}
	}

	// 存储锁
	m.locks.Store(req.FilePath, lock)
	m.locksByID.Store(lock.ID, lock)

	// 更新持有者索引
	m.addToOwnerIndex(lock)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.totalLocks++
	m.stats.activeLocks++
	m.stats.mu.Unlock()

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

	m.releaseLockInternal(lock)

	m.logger.Debug("lock released",
		zap.String("id", lockID),
		zap.String("file", lock.FilePath),
		zap.String("owner", owner),
	)

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

	m.releaseLockInternal(lock)

	return nil
}

// ForceUnlock 强制释放锁（管理员操作）
func (m *Manager) ForceUnlock(lockID string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return ErrLockNotFound
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return ErrInvalidLockType
	}
	m.releaseLockInternal(lock)

	m.logger.Info("lock force released",
		zap.String("id", lockID),
		zap.String("file", lock.FilePath),
		zap.String("owner", lock.Owner),
	)

	return nil
}

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
		m.releaseLockInternal(lock)
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
		m.releaseLockInternal(lock)
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
		m.releaseLockInternal(lock)
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
		m.releaseLockInternal(existing)
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

	conflict := m.checkConflict(existing, req)
	return conflict, conflict == nil
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

	m.logger.Debug("lock extended",
		zap.String("id", lockID),
		zap.Duration("duration", duration),
	)

	return nil
}

// ListLocks 列出所有锁
func (m *Manager) ListLocks(filter *LockFilter) []*LockInfo {
	var result []*LockInfo

	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
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
		result = append(result, lock.ToInfo())
		return true
	})

	return result
}

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
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.cancel()
	m.wg.Wait()

	m.logger.Info("lock manager closed")
}

// LockFilter 锁过滤器
type LockFilter struct {
	Owner    string
	LockType LockType
	Status   LockStatus
	Protocol string
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalLocks    int64 `json:"totalLocks"`
	ActiveLocks   int64 `json:"activeLocks"`
	ExpiredLocks  int64 `json:"expiredLocks"`
	ReleasedLocks int64 `json:"releasedLocks"`
}

// 内部方法

func (m *Manager) checkConflict(existing *FileLock, req *LockRequest) *LockConflict {
	// 同一用户不冲突
	if existing.Owner == req.Owner {
		return nil
	}

	// 独占锁与任何锁都冲突
	if existing.LockType == LockTypeExclusive || req.LockType == LockTypeExclusive {
		return &LockConflict{
			ExistingLock: existing.ToInfo(),
			Message:      fmt.Sprintf("file is exclusively locked by %s", existing.OwnerName),
		}
	}

	// 共享锁之间不冲突（除非超过限制）
	return nil
}

func (m *Manager) releaseLockInternal(lock *FileLock) {
	// 更新状态
	lock.Release()

	// 从索引中删除
	m.locks.Delete(lock.FilePath)
	m.locksByID.Delete(lock.ID)
	m.removeFromOwnerIndex(lock)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.activeLocks--
	m.stats.releasedLocks++
	m.stats.mu.Unlock()
}

func (m *Manager) addToOwnerIndex(lock *FileLock) {
	raw, _ := m.ownerLocks.LoadOrStore(lock.Owner, &sync.Map{})
	ownerLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	ownerLocks.Store(lock.ID, lock)
}

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
		lock.Status = LockStatusExpired
		m.releaseLockInternal(lock)

		m.logger.Debug("lock expired",
			zap.String("id", lock.ID),
			zap.String("file", lock.FilePath),
			zap.String("owner", lock.Owner),
		)

		m.stats.mu.Lock()
		m.stats.expiredLocks++
		m.stats.mu.Unlock()
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
			// 续期
			lock.Extend(m.config.DefaultTimeout)

			m.logger.Debug("lock auto-renewed",
				zap.String("id", lock.ID),
				zap.String("file", lock.FilePath),
			)
		}

		return true
	})
}

// SMB/NFS 集成适配器

// SMBLockAdapter SMB 锁适配器
type SMBLockAdapter struct {
	manager *Manager
}

// NewSMBLockAdapter 创建 SMB 锁适配器
func NewSMBLockAdapter(manager *Manager) *SMBLockAdapter {
	return &SMBLockAdapter{manager: manager}
}

// Lock 锁定文件
func (a *SMBLockAdapter) Lock(filePath string, owner string, exclusive bool) error {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		Owner:    owner,
		Protocol: "SMB",
	}

	_, _, err := a.manager.Lock(req)
	return err
}

// Unlock 解锁文件
func (a *SMBLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定
func (a *SMBLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者
func (a *SMBLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// NFSLockAdapter NFS 锁适配器
type NFSLockAdapter struct {
	manager *Manager
}

// NewNFSLockAdapter 创建 NFS 锁适配器
func NewNFSLockAdapter(manager *Manager) *NFSLockAdapter {
	return &NFSLockAdapter{manager: manager}
}

// Lock 锁定文件
func (a *NFSLockAdapter) Lock(filePath string, owner string, exclusive bool) error {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		Owner:    owner,
		Protocol: "NFS",
	}

	_, _, err := a.manager.Lock(req)
	return err
}

// Unlock 解锁文件
func (a *NFSLockAdapter) Unlock(filePath string, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// IsLocked 检查文件是否被锁定
func (a *NFSLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

// GetLockOwner 获取锁持有者
func (a *NFSLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}
