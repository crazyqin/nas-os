// Package lock 提供文件锁定机制，防止并发编辑冲突
// 支持独占锁和共享锁，参考群晖 Drive 文件锁定实现
package lock

import (
	"errors"
	"sync"
	"time"
)

// LockType defines the type of file lock.
type LockType int

const (
	// LockTypeShared 共享锁（读锁）- 多个用户可同时持有.
	LockTypeShared LockType = iota
	// LockTypeExclusive 独占锁（写锁）- 只有一个用户可持有.
	LockTypeExclusive
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
	// LockStatusActive 锁活跃状态.
	LockStatusActive LockStatus = iota
	// LockStatusExpired 锁已过期.
	LockStatusExpired
	// LockStatusReleased 锁已释放.
	LockStatusReleased
)

func (ls LockStatus) String() string {
	switch ls {
	case LockStatusActive:
		return "active"
	case LockStatusExpired:
		return "expired"
	case LockStatusReleased:
		return "released"
	default:
		return "unknown"
	}
}

// 定义错误.
var (
	// ErrLockNotFound 锁不存在.
	ErrLockNotFound = errors.New("lock not found")
	// ErrLockConflict 锁冲突.
	ErrLockConflict = errors.New("lock conflict")
	// ErrLockExpired 锁已过期.
	ErrLockExpired = errors.New("lock expired")
	// ErrNotLockOwner 不是锁的持有者.
	ErrNotLockOwner = errors.New("not lock owner")
	// ErrInvalidLockType 无效的锁类型.
	ErrInvalidLockType = errors.New("invalid lock type")
	// ErrFileAlreadyLocked 文件已被锁定.
	ErrFileAlreadyLocked = errors.New("file already locked")
)

// FileLock 文件锁.
type FileLock struct {
	// ID 锁的唯一标识
	ID string `json:"id"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// LockType 锁类型
	LockType LockType `json:"lockType"`
	// Status 锁状态
	Status LockStatus `json:"status"`
	// Owner 锁持有者（用户ID或客户端标识）
	Owner string `json:"owner"`
	// OwnerName 锁持有者名称（用于显示）
	OwnerName string `json:"ownerName,omitempty"`
	// ClientID 客户端标识（用于区分同一用户的不同客户端）
	ClientID string `json:"clientId,omitempty"`
	// Protocol 协议来源（SMB/NFS/WebDAV/API）
	Protocol string `json:"protocol,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt 过期时间
	ExpiresAt time.Time `json:"expiresAt"`
	// LastAccessed 最后访问时间
	LastAccessed time.Time `json:"lastAccessed"`
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`

	// 内部字段
	mu sync.RWMutex
}

// IsExpired 检查锁是否已过期.
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

// Refresh 刷新锁的访问时间.
func (fl *FileLock) Refresh() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.LastAccessed = time.Now()
}

// Extend 延长锁的有效期.
func (fl *FileLock) Extend(duration time.Duration) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.ExpiresAt = time.Now().Add(duration)
	fl.LastAccessed = time.Now()
}

// Release 释放锁.
func (fl *FileLock) Release() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.Status = LockStatusReleased
}

// LockInfo 锁信息（用于API响应）.
type LockInfo struct {
	ID        string            `json:"id"`
	FilePath  string            `json:"filePath"`
	LockType  string            `json:"lockType"`
	Status    string            `json:"status"`
	Owner     string            `json:"owner"`
	OwnerName string            `json:"ownerName,omitempty"`
	ClientID  string            `json:"clientId,omitempty"`
	Protocol  string            `json:"protocol,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	ExpiresAt time.Time         `json:"expiresAt"`
	ExpiresIn int64             `json:"expiresIn"` // 剩余秒数
	IsExpired bool              `json:"isExpired"`
	Metadata  map[string]string `json:"metadata,omitempty"`
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
		ID:        fl.ID,
		FilePath:  fl.FilePath,
		LockType:  fl.LockType.String(),
		Status:    fl.Status.String(),
		Owner:     fl.Owner,
		OwnerName: fl.OwnerName,
		ClientID:  fl.ClientID,
		Protocol:  fl.Protocol,
		CreatedAt: fl.CreatedAt,
		ExpiresAt: fl.ExpiresAt,
		ExpiresIn: expiresIn,
		IsExpired: now.After(fl.ExpiresAt),
		Metadata:  fl.Metadata,
	}
}

// LockRequest 锁请求.
type LockRequest struct {
	// FilePath 文件路径
	FilePath string `json:"filePath" binding:"required"`
	// LockType 锁类型
	LockType LockType `json:"lockType"`
	// Owner 锁持有者
	Owner string `json:"owner" binding:"required"`
	// OwnerName 锁持有者名称
	OwnerName string `json:"ownerName,omitempty"`
	// ClientID 客户端标识
	ClientID string `json:"clientId,omitempty"`
	// Protocol 协议来源
	Protocol string `json:"protocol,omitempty"`
	// Timeout 锁超时时间（秒）
	Timeout int `json:"timeout"`
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// LockConflict 锁冲突信息.
type LockConflict struct {
	// ExistingLock 现有的锁
	ExistingLock *LockInfo `json:"existingLock"`
	// Message 冲突消息
	Message string `json:"message"`
}

// FileLockConfig 锁配置.
type FileLockConfig struct {
	// DefaultTimeout 默认锁超时时间
	DefaultTimeout time.Duration
	// MaxTimeout 最大锁超时时间
	MaxTimeout time.Duration
	// CleanupInterval 清理过期锁的间隔
	CleanupInterval time.Duration
	// MaxLocksPerFile 每个文件最大共享锁数量
	MaxLocksPerFile int
	// EnableAutoRenewal 是否启用自动续期
	EnableAutoRenewal bool
	// AutoRenewalInterval 自动续期间隔
	AutoRenewalInterval time.Duration
}

// DefaultConfig 默认配置.
func DefaultConfig() FileLockConfig {
	return FileLockConfig{
		DefaultTimeout:      30 * time.Minute,
		MaxTimeout:          24 * time.Hour,
		CleanupInterval:     5 * time.Minute,
		MaxLocksPerFile:     100,
		EnableAutoRenewal:   true,
		AutoRenewalInterval: 10 * time.Minute,
	}
}

// ProtocolLockAdapter 协议锁适配器接口（用于与 SMB/NFS 集成）.
type ProtocolLockAdapter interface {
	// Lock 锁定文件
	Lock(filePath string, owner string, exclusive bool) error
	// Unlock 解锁文件
	Unlock(filePath string, owner string) error
	// IsLocked 检查文件是否被锁定
	IsLocked(filePath string) bool
	// GetLockOwner 获取锁持有者
	GetLockOwner(filePath string) (string, error)
}
