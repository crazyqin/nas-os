// Package locking 提供文件锁定协作功能
// 对标群晖 Drive 4.0 文件锁定机制，支持多协议协作锁定
package locking

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 锁类型和状态 ==========

// LockType 锁类型.
type LockType int

const (
	LockTypeShared    LockType = iota // 共享锁（读锁）- 多用户可同时持有
	LockTypeExclusive                 // 独占锁（写锁）- 只有一个用户可持有
)

func (lt LockType) String() string {
	switch lt {
	case LockTypeShared:
		return "shared"
	case LockTypeExclusive:
		return "exclusive"
	default:
		return "unknown"
	}
}

// LockStatus 锁状态.
type LockStatus int

const (
	LockStatusActive LockStatus = iota
	LockStatusExpired
	LockStatusReleased
	LockStatusWaiting // 等待获取锁
)

func (ls LockStatus) String() string {
	switch ls {
	case LockStatusActive:
		return "active"
	case LockStatusExpired:
		return "expired"
	case LockStatusReleased:
		return "released"
	case LockStatusWaiting:
		return "waiting"
	default:
		return "unknown"
	}
}

// ========== 锁定结构 ==========

// FileLock 文件锁.
type FileLock struct {
	ID            string            `json:"id"`
	FilePath      string            `json:"filePath"`
	LockType      LockType          `json:"lockType"`
	Status        LockStatus        `json:"status"`
	Owner         string            `json:"owner"`     // 用户ID
	OwnerName     string            `json:"ownerName"` // 用户显示名
	ClientID      string            `json:"clientId"`  // 客户端标识
	SessionID     string            `json:"sessionId"` // 会话ID（用于协作）
	Protocol      string            `json:"protocol"`  // SMB/NFS/WebDAV/Drive/API
	AppName       string            `json:"appName"`   // 应用名称（如Synology Drive）
	CreatedAt     time.Time         `json:"createdAt"`
	ExpiresAt     time.Time         `json:"expiresAt"`
	LastAccessed  time.Time         `json:"lastAccessed"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"` // 心跳时间
	Metadata      map[string]string `json:"metadata"`

	// 协作字段
	SharedWith   []string `json:"sharedWith"`   // 共享锁的用户列表
	Version      int64    `json:"version"`      // 锁版本号
	ParentLockID string   `json:"parentLockId"` // 父锁ID（嵌套锁定）

	mu sync.RWMutex
}

// IsExpired 检查是否过期.
func (fl *FileLock) IsExpired() bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return time.Now().After(fl.ExpiresAt)
}

// IsOwnedBy 检查是否由指定用户持有.
func (fl *FileLock) IsOwnedBy(owner string) bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.Owner == owner
}

// Refresh 刷新访问时间.
func (fl *FileLock) Refresh() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.LastAccessed = time.Now()
	fl.LastHeartbeat = time.Now()
}

// Extend 延长有效期.
func (fl *FileLock) Extend(duration time.Duration) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.ExpiresAt = time.Now().Add(duration)
	fl.LastAccessed = time.Now()
	fl.Version++
}

// Release 释放锁.
func (fl *FileLock) Release() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.Status = LockStatusReleased
}

// ========== 锁请求和响应 ==========

// LockRequest 锁请求.
type LockRequest struct {
	FilePath    string            `json:"filePath" binding:"required"`
	LockType    LockType          `json:"lockType"`
	Owner       string            `json:"owner" binding:"required"`
	OwnerName   string            `json:"ownerName,omitempty"`
	ClientID    string            `json:"clientId,omitempty"`
	SessionID   string            `json:"sessionId,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	AppName     string            `json:"appName,omitempty"`
	Timeout     int               `json:"timeout"` // 超时秒数
	Metadata    map[string]string `json:"metadata,omitempty"`
	WaitForLock bool              `json:"waitForLock"` // 是否等待锁释放
	WaitTimeout int               `json:"waitTimeout"` // 等待超时秒数
}

// LockConflict 锁冲突信息.
type LockConflict struct {
	ExistingLock *LockInfo `json:"existingLock"`
	Message      string    `json:"message"`
	CanWait      bool      `json:"canWait"` // 是否可以等待
}

// LockInfo 锁信息（API响应）.
type LockInfo struct {
	ID            string            `json:"id"`
	FilePath      string            `json:"filePath"`
	LockType      string            `json:"lockType"`
	Status        string            `json:"status"`
	Owner         string            `json:"owner"`
	OwnerName     string            `json:"ownerName,omitempty"`
	ClientID      string            `json:"clientId,omitempty"`
	SessionID     string            `json:"sessionId,omitempty"`
	Protocol      string            `json:"protocol,omitempty"`
	AppName       string            `json:"appName,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	ExpiresAt     time.Time         `json:"expiresAt"`
	ExpiresIn     int64             `json:"expiresIn"` // 剩余秒数
	IsExpired     bool              `json:"isExpired"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	SharedWith    []string          `json:"sharedWith,omitempty"`
}

// ToInfo 转换为LockInfo.
func (fl *FileLock) ToInfo() *LockInfo {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	now := time.Now()
	expiresIn := int64(fl.ExpiresAt.Sub(now).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	return &LockInfo{
		ID:            fl.ID,
		FilePath:      fl.FilePath,
		LockType:      fl.LockType.String(),
		Status:        fl.Status.String(),
		Owner:         fl.Owner,
		OwnerName:     fl.OwnerName,
		ClientID:      fl.ClientID,
		SessionID:     fl.SessionID,
		Protocol:      fl.Protocol,
		AppName:       fl.AppName,
		CreatedAt:     fl.CreatedAt,
		ExpiresAt:     fl.ExpiresAt,
		ExpiresIn:     expiresIn,
		IsExpired:     now.After(fl.ExpiresAt),
		LastHeartbeat: fl.LastHeartbeat,
		Metadata:      fl.Metadata,
		SharedWith:    fl.SharedWith,
	}
}

// ========== 锁管理器配置 ==========

// LockConfig 锁配置.
type LockConfig struct {
	DefaultTimeout      time.Duration `json:"defaultTimeout"`
	MaxTimeout          time.Duration `json:"maxTimeout"`
	CleanupInterval     time.Duration `json:"cleanupInterval"`
	HeartbeatInterval   time.Duration `json:"heartbeatInterval"` // 心跳检查间隔
	HeartbeatTimeout    time.Duration `json:"heartbeatTimeout"`  // 心跳超时
	MaxLocksPerFile     int           `json:"maxLocksPerFile"`   // 每文件最大共享锁
	EnableAutoRenewal   bool          `json:"enableAutoRenewal"`
	AutoRenewalInterval time.Duration `json:"autoRenewalInterval"`
	EnableCollaboration bool          `json:"enableCollaboration"` // 启用协作锁定
	NotifyOnLockChange  bool          `json:"notifyOnLockChange"`  // 锁变更通知
}

// DefaultLockConfig 默认配置.
func DefaultLockConfig() LockConfig {
	return LockConfig{
		DefaultTimeout:      30 * time.Minute,
		MaxTimeout:          24 * time.Hour,
		CleanupInterval:     5 * time.Minute,
		HeartbeatInterval:   1 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		MaxLocksPerFile:     100,
		EnableAutoRenewal:   true,
		AutoRenewalInterval: 10 * time.Minute,
		EnableCollaboration: true,
		NotifyOnLockChange:  true,
	}
}

// ========== 锁管理器 ==========

// LockManager 锁管理器.
type LockManager struct {
	config LockConfig

	// 锁存储
	locks        sync.Map // map[string]*FileLock (filePath -> lock)
	locksByID    sync.Map // map[string]*FileLock (id -> lock)
	ownerLocks   sync.Map // map[string]sync.Map (owner -> locks)
	sessionLocks sync.Map // map[string]sync.Map (sessionID -> locks) 协作会话

	// 等待队列
	waitQueue sync.Map // map[string][]*LockWaiter (filePath -> waiters)

	// 事件通知
	notifiers   []LockNotifier
	notifiersMu sync.RWMutex

	// 统计
	stats struct {
		mu            sync.RWMutex
		totalLocks    int64
		activeLocks   int64
		expiredLocks  int64
		releasedLocks int64
		waiters       int64
	}

	// 后台任务
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LockWaiter 锁等待者.
type LockWaiter struct {
	Request    *LockRequest
	NotifyChan chan *FileLock
	StartTime  time.Time
	Timeout    time.Duration
}

// LockNotifier 锁事件通知器接口.
type LockNotifier interface {
	OnLockEvent(event LockEvent)
}

// LockEvent 锁事件.
type LockEvent struct {
	Type      EventType `json:"type"` // acquired/released/conflict/expired/heartbeat/wait
	LockID    string    `json:"lockId"`
	FilePath  string    `json:"filePath"`
	Owner     string    `json:"owner"`
	OwnerName string    `json:"ownerName"`
	SessionID string    `json:"sessionId"`
	Protocol  string    `json:"protocol"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType 事件类型.
type EventType int

const (
	EventLockAcquired EventType = iota
	EventLockReleased
	EventLockConflict
	EventLockExpired
	EventLockHeartbeat
	EventLockWait
)

// NewLockManager 创建锁管理器.
func NewLockManager(config LockConfig) *LockManager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &LockManager{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动后台任务
	m.wg.Add(1)
	go m.cleanupLoop()

	if config.EnableAutoRenewal {
		m.wg.Add(1)
		go m.autoRenewalLoop()
	}

	m.wg.Add(1)
	go m.heartbeatLoop()

	return m
}

// ========== 核心锁定操作 ==========

// Lock 获取锁.
func (m *LockManager) Lock(ctx context.Context, req *LockRequest) (*FileLock, *LockConflict, error) {
	if req == nil {
		return nil, nil, errors.New("invalid request")
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
		ID:            uuid.New().String(),
		FilePath:      req.FilePath,
		LockType:      req.LockType,
		Status:        LockStatusActive,
		Owner:         req.Owner,
		OwnerName:     req.OwnerName,
		ClientID:      req.ClientID,
		SessionID:     req.SessionID,
		Protocol:      req.Protocol,
		AppName:       req.AppName,
		CreatedAt:     now,
		ExpiresAt:     now.Add(timeout),
		LastAccessed:  now,
		LastHeartbeat: now,
		Metadata:      req.Metadata,
		Version:       1,
	}

	// 检查现有锁
	existingRaw, loaded := m.locks.Load(req.FilePath)
	if loaded {
		existing, ok := existingRaw.(*FileLock)
		if !ok {
			return nil, nil, errors.New("invalid lock type")
		}

		// 检查是否过期
		if existing.IsExpired() {
			m.releaseLockInternal(existing)
		} else {
			// 检查锁兼容性
			conflict := m.checkConflict(existing, req)
			if conflict != nil {
				// 如果请求等待锁
				if req.WaitForLock {
					return m.waitForLock(ctx, req, existing, conflict)
				}
				return nil, conflict, errors.New("lock conflict")
			}
		}
	}

	// 存储锁
	m.locks.Store(req.FilePath, lock)
	m.locksByID.Store(lock.ID, lock)
	m.addToOwnerIndex(lock)
	if req.SessionID != "" {
		m.addToSessionIndex(lock)
	}

	// 更新统计
	m.stats.mu.Lock()
	m.stats.totalLocks++
	m.stats.activeLocks++
	m.stats.mu.Unlock()

	// 发送事件
	m.emitEvent(LockEvent{
		Type:      EventLockAcquired,
		LockID:    lock.ID,
		FilePath:  req.FilePath,
		Owner:     req.Owner,
		OwnerName: req.OwnerName,
		SessionID: req.SessionID,
		Protocol:  req.Protocol,
		Timestamp: now,
	})

	return lock, nil, nil
}

// waitForLock 等待锁释放.
func (m *LockManager) waitForLock(ctx context.Context, req *LockRequest, existing *FileLock, conflict *LockConflict) (*FileLock, *LockConflict, error) {
	waitTimeout := time.Duration(req.WaitTimeout) * time.Second
	if waitTimeout <= 0 {
		waitTimeout = 30 * time.Second
	}

	waiter := &LockWaiter{
		Request:    req,
		NotifyChan: make(chan *FileLock, 1),
		StartTime:  time.Now(),
		Timeout:    waitTimeout,
	}

	// 加入等待队列
	m.addToWaitQueue(req.FilePath, waiter)
	m.stats.mu.Lock()
	m.stats.waiters++
	m.stats.mu.Unlock()

	// 发送等待事件
	m.emitEvent(LockEvent{
		Type:      EventLockWait,
		FilePath:  req.FilePath,
		Owner:     req.Owner,
		OwnerName: req.OwnerName,
		Message:   fmt.Sprintf("waiting for lock held by %s", existing.OwnerName),
		Timestamp: time.Now(),
	})

	// 等待锁释放或超时
	select {
	case <-ctx.Done():
		m.removeFromWaitQueue(req.FilePath, waiter)
		return nil, nil, ctx.Err()
	case <-time.After(waitTimeout):
		m.removeFromWaitQueue(req.FilePath, waiter)
		return nil, conflict, errors.New("wait timeout")
	case newLock := <-waiter.NotifyChan:
		m.removeFromWaitQueue(req.FilePath, waiter)
		m.stats.mu.Lock()
		m.stats.waiters--
		m.stats.mu.Unlock()
		return newLock, nil, nil
	}
}

// Unlock 释放锁.
func (m *LockManager) Unlock(lockID string, owner string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return errors.New("invalid lock type")
	}

	// 验证持有者
	if lock.Owner != owner {
		return errors.New("not lock owner")
	}

	m.releaseLockInternal(lock)

	// 通知等待者
	m.notifyWaiters(lock.FilePath)

	// 发送事件
	m.emitEvent(LockEvent{
		Type:      EventLockReleased,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		Owner:     owner,
		OwnerName: lock.OwnerName,
		SessionID: lock.SessionID,
		Protocol:  lock.Protocol,
		Timestamp: time.Now(),
	})

	return nil
}

// Heartbeat 锁心跳（保持活跃状态）.
func (m *LockManager) Heartbeat(lockID string, owner string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return errors.New("invalid lock type")
	}

	if lock.Owner != owner {
		return errors.New("not lock owner")
	}

	lock.Refresh()

	// 发送心跳事件
	m.emitEvent(LockEvent{
		Type:      EventLockHeartbeat,
		LockID:    lockID,
		FilePath:  lock.FilePath,
		Owner:     owner,
		Timestamp: time.Now(),
	})

	return nil
}

// ========== 查询操作 ==========

// GetLock 获取锁信息.
func (m *LockManager) GetLock(lockID string) (*LockInfo, error) {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return nil, errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return nil, errors.New("invalid lock type")
	}

	if lock.IsExpired() {
		m.releaseLockInternal(lock)
		return nil, errors.New("lock expired")
	}

	return lock.ToInfo(), nil
}

// GetLockByPath 通过路径获取锁.
func (m *LockManager) GetLockByPath(filePath string) (*LockInfo, error) {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return nil, errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return nil, errors.New("invalid lock type")
	}

	if lock.IsExpired() {
		m.releaseLockInternal(lock)
		return nil, errors.New("lock expired")
	}

	return lock.ToInfo(), nil
}

// IsLocked 检查是否锁定.
func (m *LockManager) IsLocked(filePath string) bool {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return false
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return false
	}

	if lock.IsExpired() {
		m.releaseLockInternal(lock)
		return false
	}

	return lock.Status == LockStatusActive
}

// ListLocks 列出所有锁.
func (m *LockManager) ListLocks(filter *LockFilter) []*LockInfo {
	var result []*LockInfo

	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}

		if filter != nil {
			if filter.Owner != "" && lock.Owner != filter.Owner {
				return true
			}
			if filter.LockType != 0 && lock.LockType != filter.LockType {
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

// ListLocksBySession 列出会话的所有锁（协作功能）.
func (m *LockManager) ListLocksBySession(sessionID string) []*LockInfo {
	raw, ok := m.sessionLocks.Load(sessionID)
	if !ok {
		return nil
	}

	sessionLocks, ok := raw.(*sync.Map)
	if !ok {
		return nil
	}

	var result []*LockInfo
	sessionLocks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}
		result = append(result, lock.ToInfo())
		return true
	})

	return result
}

// LockFilter 锁过滤器.
type LockFilter struct {
	Owner    string
	LockType LockType
	Protocol string
}

// ========== 协作功能 ==========

// ShareLock 共享锁给其他用户.
func (m *LockManager) ShareLock(lockID string, owner string, users []string) error {
	raw, ok := m.locksByID.Load(lockID)
	if !ok {
		return errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return errors.New("invalid lock type")
	}

	if lock.Owner != owner {
		return errors.New("not lock owner")
	}

	if lock.LockType != LockTypeShared {
		return errors.New("only shared locks can be shared")
	}

	lock.mu.Lock()
	lock.SharedWith = append(lock.SharedWith, users...)
	lock.Version++
	lock.mu.Unlock()

	return nil
}

// ========== 内部方法 ==========

func (m *LockManager) checkConflict(existing *FileLock, req *LockRequest) *LockConflict {
	// 同一用户可升级锁
	if existing.Owner == req.Owner {
		return nil
	}

	// 独占锁与任何锁冲突
	if existing.LockType == LockTypeExclusive || req.LockType == LockTypeExclusive {
		return &LockConflict{
			ExistingLock: existing.ToInfo(),
			Message:      fmt.Sprintf("file is exclusively locked by %s", existing.OwnerName),
			CanWait:      true,
		}
	}

	// 共享锁之间不冲突（但有限制数量）
	return nil
}

func (m *LockManager) releaseLockInternal(lock *FileLock) {
	lock.Release()
	m.locks.Delete(lock.FilePath)
	m.locksByID.Delete(lock.ID)
	m.removeFromOwnerIndex(lock)
	m.removeFromSessionIndex(lock)

	m.stats.mu.Lock()
	m.stats.activeLocks--
	m.stats.releasedLocks++
	m.stats.mu.Unlock()
}

func (m *LockManager) addToOwnerIndex(lock *FileLock) {
	raw, _ := m.ownerLocks.LoadOrStore(lock.Owner, &sync.Map{})
	ownerLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	ownerLocks.Store(lock.ID, lock)
}

func (m *LockManager) removeFromOwnerIndex(lock *FileLock) {
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

func (m *LockManager) addToSessionIndex(lock *FileLock) {
	if lock.SessionID == "" {
		return
	}
	raw, _ := m.sessionLocks.LoadOrStore(lock.SessionID, &sync.Map{})
	sessionLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	sessionLocks.Store(lock.ID, lock)
}

func (m *LockManager) removeFromSessionIndex(lock *FileLock) {
	if lock.SessionID == "" {
		return
	}
	raw, ok := m.sessionLocks.Load(lock.SessionID)
	if !ok {
		return
	}
	sessionLocks, ok := raw.(*sync.Map)
	if !ok {
		return
	}
	sessionLocks.Delete(lock.ID)
}

func (m *LockManager) addToWaitQueue(filePath string, waiter *LockWaiter) {
	raw, _ := m.waitQueue.LoadOrStore(filePath, &[]*LockWaiter{})
	waiters, ok := raw.(*[]*LockWaiter)
	if !ok {
		return
	}
	*waiters = append(*waiters, waiter)
}

func (m *LockManager) removeFromWaitQueue(filePath string, waiter *LockWaiter) {
	raw, ok := m.waitQueue.Load(filePath)
	if !ok {
		return
	}
	waiters, ok := raw.(*[]*LockWaiter)
	if !ok {
		return
	}
	for i, w := range *waiters {
		if w == waiter {
			*waiters = append((*waiters)[:i], (*waiters)[i+1:]...)
			break
		}
	}
}

func (m *LockManager) notifyWaiters(filePath string) {
	raw, ok := m.waitQueue.Load(filePath)
	if !ok {
		return
	}

	waiters, ok := raw.(*[]*LockWaiter)
	if !ok || len(*waiters) == 0 {
		return
	}

	// 取第一个等待者
	if len(*waiters) > 0 {
		first := (*waiters)[0]
		// 尝试获取锁
		newLock, _, err := m.Lock(context.Background(), first.Request)
		if err == nil {
			first.NotifyChan <- newLock
		}
		*waiters = (*waiters)[1:]
	}
}

func (m *LockManager) emitEvent(event LockEvent) {
	m.notifiersMu.RLock()
	notifiers := make([]LockNotifier, len(m.notifiers))
	copy(notifiers, m.notifiers)
	m.notifiersMu.RUnlock()

	for _, n := range notifiers {
		go n.OnLockEvent(event)
	}
}

// AddNotifier 添加通知器.
func (m *LockManager) AddNotifier(notifier LockNotifier) {
	m.notifiersMu.Lock()
	m.notifiers = append(m.notifiers, notifier)
	m.notifiersMu.Unlock()
}

// ========== 后台任务 ==========

func (m *LockManager) cleanupLoop() {
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

func (m *LockManager) cleanupExpiredLocks() {
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

		m.emitEvent(LockEvent{
			Type:      EventLockExpired,
			LockID:    lock.ID,
			FilePath:  lock.FilePath,
			Owner:     lock.Owner,
			Timestamp: time.Now(),
		})

		// 通知等待者
		m.notifyWaiters(lock.FilePath)

		m.stats.mu.Lock()
		m.stats.expiredLocks++
		m.stats.mu.Unlock()
	}
}

func (m *LockManager) autoRenewalLoop() {
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

func (m *LockManager) renewActiveLocks() {
	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}

		if lock.Status != LockStatusActive {
			return true
		}

		remaining := time.Until(lock.ExpiresAt)
		if remaining < m.config.AutoRenewalInterval {
			lock.Extend(m.config.DefaultTimeout)
		}

		return true
	})
}

func (m *LockManager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

func (m *LockManager) checkHeartbeats() {
	var staleLocks []*FileLock

	m.locks.Range(func(key, value interface{}) bool {
		lock, ok := value.(*FileLock)
		if !ok {
			return true
		}

		if lock.Status != LockStatusActive {
			return true
		}

		// 检查心跳超时
		if time.Since(lock.LastHeartbeat) > m.config.HeartbeatTimeout {
			staleLocks = append(staleLocks, lock)
		}

		return true
	})

	for _, lock := range staleLocks {
		// 心跳超时，视为客户端失联，释放锁
		m.releaseLockInternal(lock)

		m.emitEvent(LockEvent{
			Type:      EventLockExpired,
			LockID:    lock.ID,
			FilePath:  lock.FilePath,
			Owner:     lock.Owner,
			Message:   "heartbeat timeout",
			Timestamp: time.Now(),
		})

		m.notifyWaiters(lock.FilePath)
	}
}

// Close 关闭管理器.
func (m *LockManager) Close() {
	m.cancel()
	m.wg.Wait()
}

// Stats 获取统计.
func (m *LockManager) Stats() LockStats {
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

	return LockStats{
		TotalLocks:    m.stats.totalLocks,
		ActiveLocks:   activeCount,
		ExpiredLocks:  m.stats.expiredLocks,
		ReleasedLocks: m.stats.releasedLocks,
		Waiters:       m.stats.waiters,
	}
}

// LockStats 锁统计.
type LockStats struct {
	TotalLocks    int64 `json:"totalLocks"`
	ActiveLocks   int64 `json:"activeLocks"`
	ExpiredLocks  int64 `json:"expiredLocks"`
	ReleasedLocks int64 `json:"releasedLocks"`
	Waiters       int64 `json:"waiters"`
}

// ========== 协议适配器 ==========

// SMBLockAdapter SMB协议锁适配器.
type SMBLockAdapter struct {
	manager *LockManager
}

func NewSMBLockAdapter(manager *LockManager) *SMBLockAdapter {
	return &SMBLockAdapter{manager: manager}
}

func (a *SMBLockAdapter) Lock(filePath, owner string, exclusive bool) error {
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

	_, _, err := a.manager.Lock(context.Background(), req)
	return err
}

func (a *SMBLockAdapter) Unlock(filePath, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

func (a *SMBLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

func (a *SMBLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// NFSLockAdapter NFS协议锁适配器.
type NFSLockAdapter struct {
	manager *LockManager
}

func NewNFSLockAdapter(manager *LockManager) *NFSLockAdapter {
	return &NFSLockAdapter{manager: manager}
}

func (a *NFSLockAdapter) Lock(filePath, owner string, exclusive bool) error {
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

	_, _, err := a.manager.Lock(context.Background(), req)
	return err
}

func (a *NFSLockAdapter) Unlock(filePath, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

func (a *NFSLockAdapter) IsLocked(filePath string) bool {
	return a.manager.IsLocked(filePath)
}

func (a *NFSLockAdapter) GetLockOwner(filePath string) (string, error) {
	info, err := a.manager.GetLockByPath(filePath)
	if err != nil {
		return "", err
	}
	return info.Owner, nil
}

// DriveLockAdapter 群晖Drive协议锁适配器.
type DriveLockAdapter struct {
	manager *LockManager
}

func NewDriveLockAdapter(manager *LockManager) *DriveLockAdapter {
	return &DriveLockAdapter{manager: manager}
}

func (a *DriveLockAdapter) Lock(filePath, owner, sessionID string, exclusive bool) (*FileLock, error) {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath:  filePath,
		LockType:  lockType,
		Owner:     owner,
		SessionID: sessionID,
		Protocol:  "Drive",
		AppName:   "Synology Drive",
	}

	lock, _, err := a.manager.Lock(context.Background(), req)
	return lock, err
}

func (a *DriveLockAdapter) Unlock(lockID, owner string) error {
	return a.manager.Unlock(lockID, owner)
}

func (a *DriveLockAdapter) Heartbeat(lockID, owner string) error {
	return a.manager.Heartbeat(lockID, owner)
}

func (a *DriveLockAdapter) GetLock(filePath string) (*LockInfo, error) {
	return a.manager.GetLockByPath(filePath)
}

func (a *DriveLockAdapter) ListSessionLocks(sessionID string) []*LockInfo {
	return a.manager.ListLocksBySession(sessionID)
}

func (a *DriveLockAdapter) ShareLock(lockID, owner string, users []string) error {
	return a.manager.ShareLock(lockID, owner, users)
}

// WebDAVLockAdapter WebDAV协议锁适配器.
type WebDAVLockAdapter struct {
	manager *LockManager
}

func NewWebDAVLockAdapter(manager *LockManager) *WebDAVLockAdapter {
	return &WebDAVLockAdapter{manager: manager}
}

func (a *WebDAVLockAdapter) Lock(filePath, owner string, exclusive bool) (*FileLock, error) {
	lockType := LockTypeShared
	if exclusive {
		lockType = LockTypeExclusive
	}

	req := &LockRequest{
		FilePath: filePath,
		LockType: lockType,
		Owner:    owner,
		Protocol: "WebDAV",
	}

	lock, _, err := a.manager.Lock(context.Background(), req)
	return lock, err
}

func (a *WebDAVLockAdapter) Unlock(filePath, owner string) error {
	return a.manager.UnlockByPath(filePath, owner)
}

// UnlockByPath 通过路径释放锁.
func (m *LockManager) UnlockByPath(filePath string, owner string) error {
	raw, ok := m.locks.Load(filePath)
	if !ok {
		return errors.New("lock not found")
	}

	lock, ok := raw.(*FileLock)
	if !ok {
		return errors.New("invalid lock type")
	}

	return m.Unlock(lock.ID, owner)
}
