package lock

import (
	"errors"
	"sync"
	"time"
)

// ========== 锁类型定义 ==========

// LockType 锁类型
type LockType int

const (
	// LockTypeShared 共享锁（读锁）- 多个用户可同时持有
	// 适用场景：多人同时阅读文档
	LockTypeShared LockType = iota
	// LockTypeExclusive 独占锁（写锁）- 只有一个用户可持有
	// 适用场景：编辑文档时防止冲突
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

// ParseLockType 解析锁类型字符串
func ParseLockType(s string) LockType {
	switch s {
	case "shared", "Shared", "r", "read":
		return LockTypeShared
	case "exclusive", "Exclusive", "w", "write":
		return LockTypeExclusive
	default:
		return LockTypeShared
	}
}

// ========== 锁状态定义 ==========

// LockStatus 锁状态
type LockStatus int

const (
	// LockStatusActive 锁活跃状态
	LockStatusActive LockStatus = iota
	// LockStatusExpired 锁已过期
	LockStatusExpired
	// LockStatusReleased 锁已释放
	LockStatusReleased
	// LockStatusPending 等待中（锁升级/降级）
	LockStatusPending
	// LockStatusConflict 冲突状态
	LockStatusConflict
)

func (ls LockStatus) String() string {
	switch ls {
	case LockStatusActive:
		return "active"
	case LockStatusExpired:
		return "expired"
	case LockStatusReleased:
		return "released"
	case LockStatusPending:
		return "pending"
	case LockStatusConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// ========== 锁模式定义（参考群晖 Drive）==========

// LockMode 锁模式
type LockMode int

const (
	// LockModeManual 手动锁 - 用户显式锁定
	LockModeManual LockMode = iota
	// LockModeAuto 自动锁 - 打开文件时自动锁定
	LockModeAuto
	// LockModeAdvisory 建议锁 - 软锁定，仅提示
	LockModeAdvisory
	// LockModeMandatory 强制锁 - 系统强制锁定
	LockModeMandatory
)

func (lm LockMode) String() string {
	switch lm {
	case LockModeManual:
		return "manual"
	case LockModeAuto:
		return "auto"
	case LockModeAdvisory:
		return "advisory"
	case LockModeMandatory:
		return "mandatory"
	default:
		return "unknown"
	}
}

// ========== 锁冲突策略 ==========

// ConflictStrategy 冲突解决策略
type ConflictStrategy int

const (
	// ConflictStrategyReject 拒绝新锁请求
	ConflictStrategyReject ConflictStrategy = iota
	// ConflictStrategyWait 等待现有锁释放
	ConflictStrategyWait
	// ConflictStrategyPreempt 抢占（强制释放现有锁）
	ConflictStrategyPreempt
	// ConflictStrategyDowngrade 降级（独占锁降为共享锁）
	ConflictStrategyDowngrade
	// ConflictStrategyNotify 通知现有锁持有者
	ConflictStrategyNotify
)

func (cs ConflictStrategy) String() string {
	switch cs {
	case ConflictStrategyReject:
		return "reject"
	case ConflictStrategyWait:
		return "wait"
	case ConflictStrategyPreempt:
		return "preempt"
	case ConflictStrategyDowngrade:
		return "downgrade"
	case ConflictStrategyNotify:
		return "notify"
	default:
		return "unknown"
	}
}

// ========== 锁优先级 ==========

// LockPriority 锁优先级
type LockPriority int

const (
	// PriorityLow 低优先级 - 可被抢占
	PriorityLow LockPriority = iota
	// PriorityNormal 正常优先级
	PriorityNormal
	// PriorityHigh 高优先级 - 管理员操作
	PriorityHigh
	// PriorityCritical 关键优先级 - 系统操作
	PriorityCritical
)

// ========== 错误定义 ==========

var (
	// ErrLockNotFound 锁不存在
	ErrLockNotFound = errors.New("lock not found")
	// ErrLockConflict 锁冲突
	ErrLockConflict = errors.New("lock conflict")
	// ErrLockExpired 锁已过期
	ErrLockExpired = errors.New("lock expired")
	// ErrNotLockOwner 不是锁的持有者
	ErrNotLockOwner = errors.New("not lock owner")
	// ErrInvalidLockType 无效的锁类型
	ErrInvalidLockType = errors.New("invalid lock type")
	// ErrFileAlreadyLocked 文件已被锁定
	ErrFileAlreadyLocked = errors.New("file already locked")
	// ErrLockUpgradeFailed 锁升级失败
	ErrLockUpgradeFailed = errors.New("lock upgrade failed")
	// ErrLockDowngradeFailed 锁降级失败
	ErrLockDowngradeFailed = errors.New("lock downgrade failed")
	// ErrMaxLocksExceeded 超过最大锁数量
	ErrMaxLocksExceeded = errors.New("max locks exceeded")
	// ErrLockTimeout 锁等待超时
	ErrLockTimeout = errors.New("lock wait timeout")
	// ErrInvalidOperation 无效操作
	ErrInvalidOperation = errors.New("invalid operation")
)

// ========== 文件锁模型 ==========

// FileLock 文件锁（参考群晖 DSM Drive 设计）
type FileLock struct {
	// ========== 基本信息 ==========
	// ID 锁的唯一标识（UUID）
	ID string `json:"id"`
	// FilePath 文件路径（绝对路径）
	FilePath string `json:"filePath"`
	// FileName 文件名
	FileName string `json:"fileName"`

	// ========== 锁类型与状态 ==========
	// LockType 锁类型（共享/独占）
	LockType LockType `json:"lockType"`
	// LockMode 锁模式（手动/自动/建议/强制）
	LockMode LockMode `json:"lockMode"`
	// Status 锁状态
	Status LockStatus `json:"status"`
	// Priority 锁优先级
	Priority LockPriority `json:"priority"`

	// ========== 持有者信息 ==========
	// Owner 锁持有者ID（用户ID或客户端标识）
	Owner string `json:"owner"`
	// OwnerName 锁持有者名称（用于显示）
	OwnerName string `json:"ownerName,omitempty"`
	// OwnerEmail 持有者邮箱
	OwnerEmail string `json:"ownerEmail,omitempty"`
	// ClientID 客户端标识（用于区分同一用户的不同客户端）
	ClientID string `json:"clientId,omitempty"`
	// SessionID 会话ID
	SessionID string `json:"sessionId,omitempty"`

	// ========== 协议信息 ==========
	// Protocol 协议来源（SMB/NFS/WebDAV/API/Drive）
	Protocol string `json:"protocol,omitempty"`
	// ShareName 共享名称
	ShareName string `json:"shareName,omitempty"`

	// ========== 时间信息 ==========
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt 过期时间
	ExpiresAt time.Time `json:"expiresAt"`
	// LastAccessed 最后访问时间（用于自动续期判断）
	LastAccessed time.Time `json:"lastAccessed"`
	// LastRenewedAt 最后续期时间
	LastRenewedAt time.Time `json:"lastRenewedAt,omitempty"`

	// ========== 冲突信息 ==========
	// ConflictStrategy 冲突解决策略
	ConflictStrategy ConflictStrategy `json:"conflictStrategy,omitempty"`
	// WaitQueue 等待队列（其他等待获取锁的请求）
	WaitQueue []*LockWaitRequest `json:"waitQueue,omitempty"`

	// ========== 协作信息 ==========
	// SharedOwners 共享锁持有者列表（仅用于共享锁）
	SharedOwners []*SharedOwner `json:"sharedOwners,omitempty"`
	// Version 锁版本（用于乐观锁）
	Version int64 `json:"version"`

	// ========== 元数据 ==========
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`
	// Reason 锁定原因
	Reason string `json:"reason,omitempty"`
	// AppName 应用名称
	AppName string `json:"appName,omitempty"`

	// 内部字段
	mu sync.RWMutex
}

// IsExpired 检查锁是否已过期
func (fl *FileLock) IsExpired() bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return time.Now().After(fl.ExpiresAt)
}

// IsOwnedBy 检查是否由指定用户持有
func (fl *FileLock) IsOwnedBy(owner string) bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.Owner == owner
}

// IsOwnedByClient 检查是否由指定客户端持有
func (fl *FileLock) IsOwnedByClient(owner, clientID string) bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.Owner == owner && fl.ClientID == clientID
}

// Refresh 刷新锁的访问时间
func (fl *FileLock) Refresh() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.LastAccessed = time.Now()
}

// Extend 延长锁的有效期
func (fl *FileLock) Extend(duration time.Duration) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	now := time.Now()
	fl.ExpiresAt = now.Add(duration)
	fl.LastAccessed = now
	fl.LastRenewedAt = now
	fl.Version++ // 版本增加
}

// Release 释放锁
func (fl *FileLock) Release() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.Status = LockStatusReleased
	fl.Version++
}

// Upgrade 升级锁（共享锁 -> 独占锁）
func (fl *FileLock) Upgrade() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.LockType == LockTypeExclusive {
		return nil // 已经是独占锁
	}

	// 检查是否可以升级（没有其他共享锁持有者）
	// SharedOwners列表中的其他用户会导致升级失败
	if len(fl.SharedOwners) > 0 {
		// 只有一个共享者且是当前用户时，可以升级
		// 但这个方法无法知道当前用户是谁，所以保守地拒绝
		return ErrLockUpgradeFailed
	}

	fl.LockType = LockTypeExclusive
	fl.SharedOwners = nil // 独占锁没有共享者列表
	fl.Version++
	return nil
}

// Downgrade 降级锁（独占锁 -> 共享锁）
func (fl *FileLock) Downgrade() {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.LockType == LockTypeShared {
		return // 已经是共享锁
	}

	fl.LockType = LockTypeShared
	fl.Version++
}

// AddSharedOwner 添加共享锁持有者
func (fl *FileLock) AddSharedOwner(owner *SharedOwner) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.SharedOwners == nil {
		fl.SharedOwners = make([]*SharedOwner, 0)
	}

	// 检查是否已存在
	for _, o := range fl.SharedOwners {
		if o.Owner == owner.Owner {
			return
		}
	}

	fl.SharedOwners = append(fl.SharedOwners, owner)
	fl.Version++
}

// RemoveSharedOwner 移除共享锁持有者
func (fl *FileLock) RemoveSharedOwner(ownerID string) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	for i, o := range fl.SharedOwners {
		if o.Owner == ownerID {
			fl.SharedOwners = append(fl.SharedOwners[:i], fl.SharedOwners[i+1:]...)
			fl.Version--
			return
		}
	}
}

// ========== 共享锁持有者 ==========

// SharedOwner 共享锁持有者信息
type SharedOwner struct {
	Owner      string    `json:"owner"`
	OwnerName  string    `json:"ownerName,omitempty"`
	ClientID   string    `json:"clientId,omitempty"`
	Protocol   string    `json:"protocol,omitempty"`
	AcquiredAt time.Time `json:"acquiredAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// ========== 锁等待请求 ==========

// LockWaitRequest 锁等待请求
type LockWaitRequest struct {
	ID          string         `json:"id"`
	FilePath    string         `json:"filePath"`
	LockType    LockType       `json:"lockType"`
	Owner       string         `json:"owner"`
	OwnerName   string         `json:"ownerName,omitempty"`
	ClientID    string         `json:"clientId,omitempty"`
	Protocol    string         `json:"protocol,omitempty"`
	RequestedAt time.Time      `json:"requestedAt"`
	ExpiresAt   time.Time      `json:"expiresAt"`
	Priority    LockPriority   `json:"priority"`
	NotifyChan  chan *FileLock `json:"-"`
	CancelChan  chan struct{}  `json:"-"`
}

// ========== 锁信息（API响应）==========

// LockInfo 锁信息（用于API响应）
type LockInfo struct {
	ID            string            `json:"id"`
	FilePath      string            `json:"filePath"`
	FileName      string            `json:"fileName"`
	LockType      string            `json:"lockType"`
	LockMode      string            `json:"lockMode"`
	Status        string            `json:"status"`
	Priority      string            `json:"priority"`
	Owner         string            `json:"owner"`
	OwnerName     string            `json:"ownerName,omitempty"`
	OwnerEmail    string            `json:"ownerEmail,omitempty"`
	ClientID      string            `json:"clientId,omitempty"`
	SessionID     string            `json:"sessionId,omitempty"`
	Protocol      string            `json:"protocol,omitempty"`
	ShareName     string            `json:"shareName,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	ExpiresAt     time.Time         `json:"expiresAt"`
	ExpiresIn     int64             `json:"expiresIn"` // 剩余秒数
	IsExpired     bool              `json:"isExpired"`
	LastAccessed  time.Time         `json:"lastAccessed"`
	LastRenewedAt *time.Time        `json:"lastRenewedAt,omitempty"`
	SharedOwners  []*SharedOwner    `json:"sharedOwners,omitempty"`
	SharedCount   int               `json:"sharedCount"`
	WaitQueueSize int               `json:"waitQueueSize"`
	Version       int64             `json:"version"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	AppName       string            `json:"appName,omitempty"`
}

// ToInfo 转换为LockInfo
func (fl *FileLock) ToInfo() *LockInfo {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	now := time.Now()
	expiresIn := int64(fl.ExpiresAt.Sub(now).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	var lastRenewedAt *time.Time
	if !fl.LastRenewedAt.IsZero() {
		lastRenewedAt = &fl.LastRenewedAt
	}

	return &LockInfo{
		ID:            fl.ID,
		FilePath:      fl.FilePath,
		FileName:      fl.FileName,
		LockType:      fl.LockType.String(),
		LockMode:      fl.LockMode.String(),
		Status:        fl.Status.String(),
		Priority:      priorityToString(fl.Priority),
		Owner:         fl.Owner,
		OwnerName:     fl.OwnerName,
		OwnerEmail:    fl.OwnerEmail,
		ClientID:      fl.ClientID,
		SessionID:     fl.SessionID,
		Protocol:      fl.Protocol,
		ShareName:     fl.ShareName,
		CreatedAt:     fl.CreatedAt,
		ExpiresAt:     fl.ExpiresAt,
		ExpiresIn:     expiresIn,
		IsExpired:     now.After(fl.ExpiresAt),
		LastAccessed:  fl.LastAccessed,
		LastRenewedAt: lastRenewedAt,
		SharedOwners:  fl.SharedOwners,
		SharedCount:   len(fl.SharedOwners),
		WaitQueueSize: len(fl.WaitQueue),
		Version:       fl.Version,
		Metadata:      fl.Metadata,
		Reason:        fl.Reason,
		AppName:       fl.AppName,
	}
}

func priorityToString(p LockPriority) string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}

// ========== 锁请求 ==========

// LockRequest 锁请求
type LockRequest struct {
	// FilePath 文件路径
	FilePath string `json:"filePath" binding:"required"`
	// LockType 锁类型
	LockType LockType `json:"lockType"`
	// LockMode 锁模式
	LockMode LockMode `json:"lockMode"`
	// Owner 锁持有者
	Owner string `json:"owner" binding:"required"`
	// OwnerName 锁持有者名称
	OwnerName string `json:"ownerName,omitempty"`
	// OwnerEmail 持有者邮箱
	OwnerEmail string `json:"ownerEmail,omitempty"`
	// ClientID 客户端标识
	ClientID string `json:"clientId,omitempty"`
	// SessionID 会话ID
	SessionID string `json:"sessionId,omitempty"`
	// Protocol 协议来源
	Protocol string `json:"protocol,omitempty"`
	// ShareName 共享名称
	ShareName string `json:"shareName,omitempty"`
	// Timeout 锁超时时间（秒）
	Timeout int `json:"timeout"`
	// WaitTimeout 等待锁的最大时间（秒），0表示不等待
	WaitTimeout int `json:"waitTimeout"`
	// Priority 锁优先级
	Priority LockPriority `json:"priority"`
	// ConflictStrategy 冲突解决策略
	ConflictStrategy ConflictStrategy `json:"conflictStrategy"`
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`
	// Reason 锁定原因
	Reason string `json:"reason,omitempty"`
	// AppName 应用名称
	AppName string `json:"appName,omitempty"`
	// Force 是否强制获取锁（管理员操作）
	Force bool `json:"force"`
}

// ========== 锁冲突信息 ==========

// LockConflict 锁冲突信息
type LockConflict struct {
	// ConflictType 冲突类型
	ConflictType ConflictType `json:"conflictType"`
	// ExistingLock 现有的锁
	ExistingLock *LockInfo `json:"existingLock"`
	// Message 冲突消息
	Message string `json:"message"`
	// Resolution 建议的解决方案
	Resolution string `json:"resolution,omitempty"`
	// CanWait 是否可以等待
	CanWait bool `json:"canWait"`
	// CanPreempt 是否可以抢占
	CanPreempt bool `json:"canPreempt"`
	// EstimatedWait 预计等待时间（秒）
	EstimatedWait int64 `json:"estimatedWait,omitempty"`
}

// ConflictType 冲突类型
type ConflictType string

const (
	// ConflictTypeExclusive 独占锁冲突
	ConflictTypeExclusive ConflictType = "exclusive"
	// ConflictTypeShared 共享锁冲突
	ConflictTypeShared ConflictType = "shared"
	// ConflictTypeOwner 同一用户冲突
	ConflictTypeOwner ConflictType = "owner"
	// ConflictTypeTimeout 超时冲突
	ConflictTypeTimeout ConflictType = "timeout"
	// ConflictTypePermission 权限冲突
	ConflictTypePermission ConflictType = "permission"
)

// ========== 锁配置 ==========

// FileLockConfig 锁配置
type FileLockConfig struct {
	// DefaultTimeout 默认锁超时时间
	DefaultTimeout time.Duration `json:"defaultTimeout"`
	// MaxTimeout 最大锁超时时间
	MaxTimeout time.Duration `json:"maxTimeout"`
	// CleanupInterval 清理过期锁的间隔
	CleanupInterval time.Duration `json:"cleanupInterval"`
	// MaxLocksPerFile 每个文件最大共享锁数量
	MaxLocksPerFile int `json:"maxLocksPerFile"`
	// MaxTotalLocks 系统最大锁数量
	MaxTotalLocks int `json:"maxTotalLocks"`
	// EnableAutoRenewal 是否启用自动续期
	EnableAutoRenewal bool `json:"enableAutoRenewal"`
	// AutoRenewalInterval 自动续期间隔
	AutoRenewalInterval time.Duration `json:"autoRenewalInterval"`
	// EnableAudit 是否启用审计日志
	EnableAudit bool `json:"enableAudit"`
	// EnableWaitQueue 是否启用等待队列
	EnableWaitQueue bool `json:"enableWaitQueue"`
	// MaxWaitQueueSize 等待队列最大长度
	MaxWaitQueueSize int `json:"maxWaitQueueSize"`
	// DefaultConflictStrategy 默认冲突策略
	DefaultConflictStrategy ConflictStrategy `json:"defaultConflictStrategy"`
	// EnablePreemption 是否允许抢占
	EnablePreemption bool `json:"enablePreemption"`
	// PreemptionTimeout 抢占超时时间
	PreemptionTimeout time.Duration `json:"preemptionTimeout"`
}

// DefaultConfig 默认配置
func DefaultConfig() FileLockConfig {
	return FileLockConfig{
		DefaultTimeout:          30 * time.Minute,
		MaxTimeout:              24 * time.Hour,
		CleanupInterval:         5 * time.Minute,
		MaxLocksPerFile:         100,
		MaxTotalLocks:           10000,
		EnableAutoRenewal:       true,
		AutoRenewalInterval:     10 * time.Minute,
		EnableAudit:             true,
		EnableWaitQueue:         true,
		MaxWaitQueueSize:        50,
		DefaultConflictStrategy: ConflictStrategyReject,
		EnablePreemption:        true,
		PreemptionTimeout:       5 * time.Minute,
	}
}

// ========== 审计事件类型 ==========

// LockAuditEvent 锁审计事件类型
type LockAuditEvent string

const (
	// AuditEventLockAcquired 锁获取成功
	AuditEventLockAcquired LockAuditEvent = "lock_acquired"
	// AuditEventLockReleased 锁释放
	AuditEventLockReleased LockAuditEvent = "lock_released"
	// AuditEventLockExpired 锁过期
	AuditEventLockExpired LockAuditEvent = "lock_expired"
	// AuditEventLockExtended 锁续期
	AuditEventLockExtended LockAuditEvent = "lock_extended"
	// AuditEventLockUpgraded 锁升级
	AuditEventLockUpgraded LockAuditEvent = "lock_upgraded"
	// AuditEventLockDowngraded 锁降级
	AuditEventLockDowngraded LockAuditEvent = "lock_downgraded"
	// AuditEventLockConflict 锁冲突
	AuditEventLockConflict LockAuditEvent = "lock_conflict"
	// AuditEventLockPreempted 锁被抢占
	AuditEventLockPreempted LockAuditEvent = "lock_preempted"
	// AuditEventLockForceReleased 强制释放
	AuditEventLockForceReleased LockAuditEvent = "lock_force_released"
	// AuditEventWaitQueued 加入等待队列
	AuditEventWaitQueued LockAuditEvent = "wait_queued"
	// AuditEventWaitTimeout 等待超时
	AuditEventWaitTimeout LockAuditEvent = "wait_timeout"
)

// ========== 锁审计日志条目 ==========

// LockAuditEntry 锁审计日志条目
type LockAuditEntry struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Event        LockAuditEvent         `json:"event"`
	LockID       string                 `json:"lockId,omitempty"`
	FilePath     string                 `json:"filePath"`
	FileName     string                 `json:"fileName"`
	LockType     string                 `json:"lockType"`
	Owner        string                 `json:"owner"`
	OwnerName    string                 `json:"ownerName,omitempty"`
	ClientID     string                 `json:"clientId,omitempty"`
	Protocol     string                 `json:"protocol,omitempty"`
	Duration     int64                  `json:"duration,omitempty"` // 锁持有时长(ms)
	Reason       string                 `json:"reason,omitempty"`
	ConflictWith *LockInfo              `json:"conflictWith,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// ========== 协议锁适配器接口 ==========

// ProtocolLockAdapter 协议锁适配器接口（用于与 SMB/NFS 集成）
type ProtocolLockAdapter interface {
	// Lock 锁定文件
	Lock(filePath string, owner string, exclusive bool) error
	// Unlock 解锁文件
	Unlock(filePath string, owner string) error
	// IsLocked 检查文件是否被锁定
	IsLocked(filePath string) bool
	// GetLockOwner 获取锁持有者
	GetLockOwner(filePath string) (string, error)
	// GetLockInfo 获取锁详情
	GetLockInfo(filePath string) (*LockInfo, error)
}
