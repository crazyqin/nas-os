package webdav

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"sync"
	"time"
)

// Lock WebDAV 锁
type Lock struct {
	Token     string    `json:"token"`
	Path      string    `json:"path"`
	Owner     string    `json:"owner"`
	Depth     int       `json:"depth"`
	Scope     string    `json:"scope"` // exclusive 或 shared
	Type      string    `json:"type"`  // write
	Timeout   time.Time `json:"timeout"`
	CreatedAt time.Time `json:"created_at"`
}

// LockManager 锁管理器
type LockManager struct {
	mu    sync.RWMutex
	locks map[string]*Lock  // token -> Lock
	paths map[string]string // path -> token
}

// NewLockManager 创建锁管理器
func NewLockManager() *LockManager {
	return &LockManager{
		locks: make(map[string]*Lock),
		paths: make(map[string]string),
	}
}

// CreateLock 创建锁
func (lm *LockManager) CreateLock(path, owner string, depth int, scope string, timeoutSeconds int) (*Lock, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 检查是否已存在锁
	// 简化实现：任何已存在的锁都会阻止新锁创建
	if _, exists := lm.paths[path]; exists {
		return nil, ErrLocked
	}

	// 生成锁令牌
	token, err := generateLockToken()
	if err != nil {
		return nil, err
	}

	// 计算超时时间
	timeout := time.Time{}
	if timeoutSeconds > 0 {
		timeout = time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	}

	lock := &Lock{
		Token:     token,
		Path:      path,
		Owner:     owner,
		Depth:     depth,
		Scope:     scope,
		Type:      "write",
		Timeout:   timeout,
		CreatedAt: time.Now(),
	}

	lm.locks[token] = lock
	lm.paths[path] = token

	return lock, nil
}

// GetLock 获取锁
func (lm *LockManager) GetLock(token string) (*Lock, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lock, exists := lm.locks[token]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if !lock.Timeout.IsZero() && time.Now().After(lock.Timeout) {
		return nil, false
	}

	return lock, true
}

// GetLockByPath 通过路径获取锁
func (lm *LockManager) GetLockByPath(path string) (*Lock, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	token, exists := lm.paths[path]
	if !exists {
		return nil, false
	}

	lock, exists := lm.locks[token]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if !lock.Timeout.IsZero() && time.Now().After(lock.Timeout) {
		return nil, false
	}

	return lock, true
}

// RefreshLock 刷新锁
func (lm *LockManager) RefreshLock(token string, timeoutSeconds int) (*Lock, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[token]
	if !exists {
		return nil, ErrLockNotFound
	}

	// 检查是否过期
	if !lock.Timeout.IsZero() && time.Now().After(lock.Timeout) {
		delete(lm.locks, token)
		delete(lm.paths, lock.Path)
		return nil, ErrLockNotFound
	}

	// 刷新超时时间
	if timeoutSeconds > 0 {
		lock.Timeout = time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	}

	return lock, nil
}

// RemoveLock 移除锁
func (lm *LockManager) RemoveLock(token string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[token]
	if !exists {
		return ErrLockNotFound
	}

	delete(lm.locks, token)
	delete(lm.paths, lock.Path)

	return nil
}

// IsLocked 检查路径是否被锁定
func (lm *LockManager) IsLocked(path string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	token, exists := lm.paths[path]
	if !exists {
		return false
	}

	lock, exists := lm.locks[token]
	if !exists {
		return false
	}

	// 检查是否过期
	if !lock.Timeout.IsZero() && time.Now().After(lock.Timeout) {
		return false
	}

	return true
}

// ValidateToken 验证锁令牌
func (lm *LockManager) ValidateToken(path, token string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lock, exists := lm.locks[token]
	if !exists {
		return false
	}

	// 检查是否过期
	if !lock.Timeout.IsZero() && time.Now().After(lock.Timeout) {
		return false
	}

	// 检查路径匹配
	return lock.Path == path
}

// CleanupExpired 清理过期锁
func (lm *LockManager) CleanupExpired() int {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	count := 0
	now := time.Now()

	for token, lock := range lm.locks {
		if !lock.Timeout.IsZero() && now.After(lock.Timeout) {
			delete(lm.locks, token)
			delete(lm.paths, lock.Path)
			count++
		}
	}

	return count
}

// generateLockToken 生成锁令牌
func generateLockToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return "urn:uuid:" + hex.EncodeToString(b), nil
}

// ========== XML 类型定义 ==========

// LockDiscovery 锁发现 XML
type LockDiscovery struct {
	XMLName    xml.Name    `xml:"D:lockdiscovery"`
	ActiveLock *ActiveLock `xml:"D:activelock,omitempty"`
}

// ActiveLock 活动锁 XML
type ActiveLock struct {
	XMLName   xml.Name   `xml:"D:activelock"`
	LockType  *LockType  `xml:"D:locktype"`
	LockScope *LockScope `xml:"D:lockscope"`
	Depth     int        `xml:"D:depth"`
	Owner     *Owner     `xml:"D:owner,omitempty"`
	Timeout   string     `xml:"D:timeout,omitempty"`
	LockToken *LockToken `xml:"D:locktoken"`
	LockRoot  *LockRoot  `xml:"D:lockroot"`
}

// LockType 锁类型
type LockType struct {
	XMLName xml.Name `xml:"D:locktype"`
	Write   struct{} `xml:"D:write"`
}

// LockScope 锁范围
type LockScope struct {
	XMLName   xml.Name  `xml:"D:lockscope"`
	Exclusive *struct{} `xml:"D:exclusive,omitempty"`
	Shared    *struct{} `xml:"D:shared,omitempty"`
}

// Owner 所有者
type Owner struct {
	XMLName xml.Name `xml:"D:owner"`
	Href    string   `xml:"D:href,omitempty"`
}

// LockToken 锁令牌
type LockToken struct {
	XMLName xml.Name `xml:"D:locktoken"`
	Href    string   `xml:"D:href"`
}

// LockRoot 锁根
type LockRoot struct {
	XMLName xml.Name `xml:"D:lockroot"`
	Href    string   `xml:"D:href"`
}
