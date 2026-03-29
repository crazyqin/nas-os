package filelock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LockService 文件锁定服务
// 对标: 群晖Drive 4.0文件锁定机制

// LockState 锁定状态
type LockState struct {
	Path       string    // 文件路径
	LockedBy   string    // 锁定者用户
	SessionID  string    // 会话ID
	LockedAt   time.Time // 锁定时间
	ExpiresAt  time.Time // 过期时间
	LockType   LockType  // 锁类型
}

// LockType 锁类型
type LockType int

const (
	LockTypeRead  LockType = 1 // 读锁 (共享锁)
	LockTypeWrite LockType = 2 // 写锁 (排他锁)
)

// LockConflict 锁冲突信息
type LockConflict struct {
	Path          string
	RequestBy     string
	CurrentLock   LockState
	ConflictType  string
}

// LockBackend 锁后端接口
type LockBackend interface {
	Acquire(ctx context.Context, path, user, session string, lockType LockType, ttl time.Duration) (*LockState, error)
	Release(ctx context.Context, path, session string) error
	Get(ctx context.Context, path string) (*LockState, error)
	Check(ctx context.Context, path string, user string) (*LockConflict, error)
	Extend(ctx context.Context, path, session string, ttl time.Duration) error
	List(ctx context.Context, user string) ([]LockState, error)
}

// MemoryLockBackend 内存锁后端 (开发测试用)
type MemoryLockBackend struct {
	locks map[string]*LockState
	mu    sync.RWMutex
}

func NewMemoryLockBackend() *MemoryLockBackend {
	return &MemoryLockBackend{
		locks: make(map[string]*LockState),
	}
}

func (b *MemoryLockBackend) Acquire(ctx context.Context, path, user, session string, lockType LockType, ttl time.Duration) (*LockState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 检查是否已有锁
	existing := b.locks[path]
	if existing != nil {
		// 检查锁是否过期
		if existing.ExpiresAt.After(time.Now()) {
			// 如果是同用户同会话，允许重入
			if existing.LockedBy == user && existing.SessionID == session {
				// 延长锁
				existing.ExpiresAt = time.Now().Add(ttl)
				return existing, nil
			}
			// 读锁可以共享
			if existing.LockType == LockTypeRead && lockType == LockTypeRead {
				// 共享读锁
				newLock := &LockState{
					Path:      path,
					LockedBy:  user,
					SessionID: session,
					LockedAt:  time.Now(),
					ExpiresAt: time.Now().Add(ttl),
					LockType:  LockTypeRead,
				}
				// 使用复合key存储多个读锁
				b.locks[path+"_"+session] = newLock
				return newLock, nil
			}
			return nil, fmt.Errorf("file locked by %s", existing.LockedBy)
		}
		// 过期锁，清除
		delete(b.locks, path)
	}
	
	// 创建新锁
	lock := &LockState{
		Path:      path,
		LockedBy:  user,
		SessionID: session,
		LockedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		LockType:  lockType,
	}
	b.locks[path] = lock
	
	return lock, nil
}

func (b *MemoryLockBackend) Release(ctx context.Context, path, session string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	lock := b.locks[path]
	if lock != nil && lock.SessionID == session {
		delete(b.locks, path)
		return nil
	}
	
	// 检查共享读锁
	if _, ok := b.locks[path+"_"+session]; ok {
		delete(b.locks, path+"_"+session)
		return nil
	}
	
	return fmt.Errorf("lock not found or not owned by session")
}

func (b *MemoryLockBackend) Get(ctx context.Context, path string) (*LockState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	lock := b.locks[path]
	if lock == nil {
		return nil, nil
	}
	
	// 检查过期
	if lock.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	
	return lock, nil
}

func (b *MemoryLockBackend) Check(ctx context.Context, path, user string) (*LockConflict, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	lock := b.locks[path]
	if lock == nil || lock.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	
	if lock.LockedBy != user && lock.LockType == LockTypeWrite {
		return &LockConflict{
			Path:         path,
			RequestBy:    user,
			CurrentLock:  *lock,
			ConflictType: "write_exclusive",
		}, nil
	}
	
	return nil, nil
}

func (b *MemoryLockBackend) Extend(ctx context.Context, path, session string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	lock := b.locks[path]
	if lock == nil || lock.SessionID != session {
		return fmt.Errorf("lock not found")
	}
	
	lock.ExpiresAt = time.Now().Add(ttl)
	return nil
}

func (b *MemoryLockBackend) List(ctx context.Context, user string) ([]LockState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	locks := make([]LockState, 0)
	for _, lock := range b.locks {
		if lock.ExpiresAt.After(time.Now()) && (user == "" || lock.LockedBy == user) {
			locks = append(locks, *lock)
		}
	}
	return locks, nil
}

// FileLockService 文件锁定服务
type FileLockService struct {
	backend    LockBackend
	defaultTTL time.Duration
	mu         sync.RWMutex
}

// NewFileLockService 创建文件锁定服务
func NewFileLockService(backend LockBackend) *FileLockService {
	return &FileLockService{
		backend:    backend,
		defaultTTL: 30 * time.Minute, // 默认30分钟超时
	}
}

// LockFile 锁定文件
func (s *FileLockService) LockFile(ctx context.Context, path, user, session string) (*LockState, error) {
	return s.backend.Acquire(ctx, path, user, session, LockTypeWrite, s.defaultTTL)
}

// UnlockFile 解锁文件
func (s *FileLockService) UnlockFile(ctx context.Context, path, session string) error {
	return s.backend.Release(ctx, path, session)
}

// GetLock 获取文件锁状态
func (s *FileLockService) GetLock(ctx context.Context, path string) (*LockState, error) {
	return s.backend.Get(ctx, path)
}

// CheckLock 检查锁冲突
func (s *FileLockService) CheckLock(ctx context.Context, path, user string) (*LockConflict, error) {
	return s.backend.Check(ctx, path, user)
}

// ExtendLock 延长锁时间
func (s *FileLockService) ExtendLock(ctx context.Context, path, session string) error {
	return s.backend.Extend(ctx, path, session, s.defaultTTL)
}

// ListUserLocks 获取用户所有锁
func (s *FileLockService) ListUserLocks(ctx context.Context, user string) ([]LockState, error) {
	return s.backend.List(ctx, user)
}

// TryLock 尝试锁定 (不阻塞)
func (s *FileLockService) TryLock(ctx context.Context, path, user, session string) (*LockState, bool, error) {
	// 先检查冲突
	conflict, err := s.CheckLock(ctx, path, user)
	if err != nil {
		return nil, false, err
	}
	
	if conflict != nil {
		return nil, false, nil // 有冲突，无法锁定
	}
	
	lock, err := s.LockFile(ctx, path, user, session)
	if err != nil {
		return nil, false, err
	}
	
	return lock, true, nil
}

// WaitForLock 等待锁释放
func (s *FileLockService) WaitForLock(ctx context.Context, path, user, session string, timeout time.Duration) (*LockState, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		lock, acquired, err := s.TryLock(ctx, path, user, session)
		if err != nil {
			return nil, err
		}
		if acquired {
			return lock, nil
		}
		
		// 等待一小段时间再尝试
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil, fmt.Errorf("lock wait timeout")
}

// AutoUnlock 自动解锁过期锁
func (s *FileLockService) AutoUnlock(ctx context.Context) int {
	locks, err := s.backend.List(ctx, "")
	if err != nil {
		return 0
	}
	
	count := 0
	for _, lock := range locks {
		if lock.ExpiresAt.Before(time.Now()) {
			// 过期锁
			_ = s.backend.Release(ctx, lock.Path, lock.SessionID)
			count++
		}
	}
	
	return count
}

// StartAutoUnlock 启动自动解锁后台任务
func (s *FileLockService) StartAutoUnlock(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.AutoUnlock(ctx)
		}
	}
}

// DistributedLockBackend 分布式锁后端 (Redis/etcd)
// TODO: 实现Redis分布式锁后端用于集群场景
type DistributedLockBackend struct {
	// Redis或etcd客户端
}

// NewRedisLockBackend 创建Redis锁后端
func NewRedisLockBackend(redisAddr string) *DistributedLockBackend {
	// TODO: 实现Redis连接和锁机制
	return &DistributedLockBackend{}
}

// NotifyLockEvent 锁事件通知
type LockEvent struct {
	Type     string // acquired/released/conflict/expired
	Path     string
	User     string
	Session  string
	Timestamp time.Time
}

// LockNotifier 锁事件通知器接口
type LockNotifier interface {
	Notify(event LockEvent)
}

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	URL string
}

func (n *WebhookNotifier) Notify(event LockEvent) {
	// TODO: 发送webhook通知
}