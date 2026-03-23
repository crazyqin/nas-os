package drive

import (
	"fmt"
	"sync"
	"time"
)

// FileLocker 文件锁定器
// 实现协作编辑时的文件锁定，防止冲突
type FileLocker struct {
	mu      sync.RWMutex
	locks   map[string]*FileLock
	timeout time.Duration
}

// FileLock 文件锁
type FileLock struct {
	Path       string    `json:"path"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName"`
	LockedAt   time.Time `json:"lockedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	ClientInfo string    `json:"clientInfo"` // 客户端信息 (IP, 设备等)
}

// NewFileLocker 创建文件锁定器
func NewFileLocker(timeout time.Duration) *FileLocker {
	return &FileLocker{
		locks:   make(map[string]*FileLock),
		timeout: timeout,
	}
}

// Lock 锁定文件
func (l *FileLocker) Lock(path, userID string) error {
	return l.LockWithClient(path, userID, "")
}

// LockWithClient 锁定文件 (带客户端信息)
func (l *FileLocker) LockWithClient(path, userID, clientInfo string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否已被锁定
	if lock, exists := l.locks[path]; exists {
		// 检查是否过期
		if time.Now().Before(lock.ExpiresAt) {
			// 同一用户可以重新锁定
			if lock.UserID == userID {
				lock.LockedAt = time.Now()
				lock.ExpiresAt = time.Now().Add(l.timeout)
				lock.ClientInfo = clientInfo
				return nil
			}
			return fmt.Errorf("文件已被 %s 锁定", lock.UserName)
		}
		// 锁已过期，可以重新锁定
	}

	// 创建新锁
	l.locks[path] = &FileLock{
		Path:       path,
		UserID:     userID,
		UserName:   userID, // TODO: 从用户服务获取用户名
		LockedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(l.timeout),
		ClientInfo: clientInfo,
	}

	return nil
}

// Unlock 解锁文件
func (l *FileLocker) Unlock(path, userID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock, exists := l.locks[path]
	if !exists {
		return nil // 未锁定，无需解锁
	}

	// 只有锁定者可以解锁
	if lock.UserID != userID {
		return fmt.Errorf("只能由锁定者解锁")
	}

	delete(l.locks, path)
	return nil
}

// ForceUnlock 强制解锁 (管理员权限)
func (l *FileLocker) ForceUnlock(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.locks, path)
	return nil
}

// IsLocked 检查文件是否被锁定
func (l *FileLocker) IsLocked(path string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	lock, exists := l.locks[path]
	if !exists {
		return false
	}

	// 检查是否过期
	return time.Now().Before(lock.ExpiresAt)
}

// GetLock 获取文件锁信息
func (l *FileLocker) GetLock(path string) (*FileLock, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	lock, exists := l.locks[path]
	if !exists {
		return nil, nil
	}

	// 检查是否过期
	if time.Now().After(lock.ExpiresAt) {
		return nil, nil
	}

	return lock, nil
}

// GetLocksByUser 获取用户锁定的所有文件
func (l *FileLocker) GetLocksByUser(userID string) []*FileLock {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var locks []*FileLock
	for _, lock := range l.locks {
		if lock.UserID == userID && time.Now().Before(lock.ExpiresAt) {
			locks = append(locks, lock)
		}
	}
	return locks
}

// GetAllLocks 获取所有活动锁
func (l *FileLocker) GetAllLocks() []*FileLock {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var locks []*FileLock
	now := time.Now()
	for _, lock := range l.locks {
		if now.Before(lock.ExpiresAt) {
			locks = append(locks, lock)
		}
	}
	return locks
}

// CleanupExpired 清理过期锁
func (l *FileLocker) CleanupExpired() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := 0
	now := time.Now()
	for path, lock := range l.locks {
		if now.After(lock.ExpiresAt) {
			delete(l.locks, path)
			count++
		}
	}
	return count
}

// ExtendLock 延长锁定期
func (l *FileLocker) ExtendLock(path, userID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock, exists := l.locks[path]
	if !exists {
		return fmt.Errorf("文件未锁定")
	}

	if lock.UserID != userID {
		return fmt.Errorf("只能延长自己的锁")
	}

	lock.ExpiresAt = time.Now().Add(l.timeout)
	return nil
}