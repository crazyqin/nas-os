// Package objectimmutable 提供 S3 兼容的不可变对象存储功能
// Version: v2.480.0 - Object Immutable WORM 模块
package objectimmutable

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBucketNotFound 桶不存在.
	ErrBucketNotFound = errors.New("桶不存在")
	// ErrBucketExists 桶已存在.
	ErrBucketExists = errors.New("桶已存在")
	// ErrObjectNotFound 对象不存在.
	ErrObjectNotFound = errors.New("对象不存在")
	// ErrObjectLocked 对象已被锁定，无法删除或修改.
	ErrObjectLocked = errors.New("对象已被锁定")
	// ErrObjectImmutable 对象处于不可变模式.
	ErrObjectImmutable = errors.New("对象处于不可变模式")
	// ErrInvalidRetention 无效的保留期限配置.
	ErrInvalidRetention = errors.New("无效的保留期限配置")
	// ErrRetentionExpired 保留期已过期.
	ErrRetentionExpired = errors.New("保留期已过期")
	// ErrLegalHoldActive 法律保留生效中，无法删除.
	ErrLegalHoldActive = errors.New("法律保留生效中，无法删除")
	// ErrInvalidLockMode 无效的锁定模式.
	ErrInvalidLockMode = errors.New("无效的锁定模式")
	// ErrWORMViolation WORM 违规：尝试修改不可变数据.
	ErrWORMViolation = errors.New("WORM 违规：尝试修改不可变数据")
)

// ========== 锁定模式 ==========

// LockMode 对象锁定模式.
type LockMode string

const (
	// LockModeGovernance 治理模式 - 允许特权用户覆盖.
	LockModeGovernance LockMode = "GOVERNANCE"
	// LockModeCompliance 合规模式 - 不允许任何覆盖.
	LockModeCompliance LockMode = "COMPLIANCE"
)

// ========== 保留状态 ==========

// RetentionStatus 保留状态.
type RetentionStatus string

const (
	// RetentionStatusLocked 已锁定.
	RetentionStatusLocked RetentionStatus = "LOCKED"
	// RetentionStatusUnlocked 未锁定.
	RetentionStatusUnlocked RetentionStatus = "UNLOCKED"
	// RetentionStatusExpired 已过期.
	RetentionStatusExpired RetentionStatus = "EXPIRED"
)

// ========== Object Lock 配置 ==========

// ObjectLockConfiguration 对象锁定配置.
type ObjectLockConfiguration struct {
	// Enabled 是否启用对象锁定.
	Enabled bool `json:"enabled"`
	// DefaultRetention 默认保留配置.
	DefaultRetention *DefaultRetention `json:"default_retention,omitempty"`
	// CreatedAt 配置创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 配置更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultRetention 默认保留配置.
type DefaultRetention struct {
	// Mode 锁定模式.
	Mode LockMode `json:"mode"`
	// Days 保留天数.
	Days int `json:"days,omitempty"`
	// Years 保留年数.
	Years int `json:"years,omitempty"`
}

// ========== 对象保留 ==========

// ObjectRetention 对象保留信息.
type ObjectRetention struct {
	// Mode 锁定模式.
	Mode LockMode `json:"mode"`
	// RetainUntilDate 保留截止日期.
	RetainUntilDate time.Time `json:"retain_until_date"`
	// Status 保留状态.
	Status RetentionStatus `json:"status"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 法律保留 ==========

// LegalHold 法律保留信息.
type LegalHold struct {
	// Enabled 是否启用法律保留.
	Enabled bool `json:"enabled"`
	// Reason 保留原因.
	Reason string `json:"reason,omitempty"`
	// SetBy 设置者.
	SetBy string `json:"set_by,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 不可变对象 ==========

// ImmutableObject 不可变对象.
type ImmutableObject struct {
	// ObjectKey 对象键.
	ObjectKey string `json:"object_key"`
	// BucketName 桶名称.
	BucketName string `json:"bucket_name"`
	// Size 对象大小（字节）.
	Size int64 `json:"size"`
	// ETag 对象 ETag.
	ETag string `json:"etag"`
	// ContentType 内容类型.
	ContentType string `json:"content_type"`
	// Data 对象数据（内存存储模式）.
	Data []byte `json:"-"`
	// Retention 保留信息.
	Retention *ObjectRetention `json:"retention,omitempty"`
	// LegalHold 法律保留信息.
	LegalHold *LegalHold `json:"legal_hold,omitempty"`
	// WORMProtected WORM 保护状态.
	WORMProtected bool `json:"worm_protected"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 不可变桶 ==========

// ImmutableBucket 不可变桶.
type ImmutableBucket struct {
	// Name 桶名称.
	Name string `json:"name"`
	// ObjectLockConfig 对象锁定配置.
	ObjectLockConfig *ObjectLockConfiguration `json:"object_lock_config,omitempty"`
	// DefaultImmutable 默认不可变模式.
	DefaultImmutable bool `json:"default_immutable"`
	// ObjectCount 对象数量.
	ObjectCount int `json:"object_count"`
	// TotalSize 总大小（字节）.
	TotalSize int64 `json:"total_size"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 审计日志 ==========

// AuditAction 审计操作类型.
type AuditAction string

const (
	// AuditActionRetentionSet 设置保留期.
	AuditActionRetentionSet AuditAction = "RETENTION_SET"
	// AuditActionRetentionExtend 延长保留期.
	AuditActionRetentionExtend AuditAction = "RETENTION_EXTEND"
	// AuditActionRetentionRelease 释放保留.
	AuditActionRetentionRelease AuditAction = "RETENTION_RELEASE"
	// AuditActionLegalHoldSet 设置法律保留.
	AuditActionLegalHoldSet AuditAction = "LEGAL_HOLD_SET"
	// AuditActionLegalHoldRelease 释放法律保留.
	AuditActionLegalHoldRelease AuditAction = "LEGAL_HOLD_RELEASE"
	// AuditActionDeleteAttempt 尝试删除.
	AuditActionDeleteAttempt AuditAction = "DELETE_ATTEMPT"
	// AuditActionDeleteBlocked 删除被阻止.
	AuditActionDeleteBlocked AuditAction = "DELETE_BLOCKED"
	// AuditActionWORMViolation WORM 违规.
	AuditActionWORMViolation AuditAction = "WORM_VIOLATION"
)

// RetentionAuditEntry 保留期审计日志条目.
type RetentionAuditEntry struct {
	// ID 审计条目 ID.
	ID string `json:"id"`
	// ObjectKey 对象键.
	ObjectKey string `json:"object_key"`
	// BucketName 桶名称.
	BucketName string `json:"bucket_name"`
	// Action 操作类型.
	Action AuditAction `json:"action"`
	// OldRetention 旧保留信息.
	OldRetention *ObjectRetention `json:"old_retention,omitempty"`
	// NewRetention 新保留信息.
	NewRetention *ObjectRetention `json:"new_retention,omitempty"`
	// Reason 操作原因.
	Reason string `json:"reason,omitempty"`
	// Operator 操作者.
	Operator string `json:"operator"`
	// Timestamp 操作时间.
	Timestamp time.Time `json:"timestamp"`
	// IPAddress 操作者 IP.
	IPAddress string `json:"ip_address,omitempty"`
	// Success 操作是否成功.
	Success bool `json:"success"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
}

// ========== S3 兼容请求/响应类型 ==========

// PutObjectRetentionRequest 设置对象保留请求.
type PutObjectRetentionRequest struct {
	// Mode 锁定模式.
	Mode LockMode `json:"mode"`
	// RetainUntilDate 保留截止日期.
	RetainUntilDate time.Time `json:"retain_until_date"`
	// BypassGovernance 是否绕过治理锁定.
	BypassGovernance bool `json:"bypass_governance,omitempty"`
}

// PutObjectRetentionResponse 设置对象保留响应.
type PutObjectRetentionResponse struct {
	// Retention 返回的保留信息.
	Retention *ObjectRetention `json:"retention"`
}

// GetObjectRetentionResponse 获取对象保留响应.
type GetObjectRetentionResponse struct {
	// Retention 保留信息.
	Retention *ObjectRetention `json:"retention"`
}

// PutObjectLegalHoldRequest 设置对象法律保留请求.
type PutObjectLegalHoldRequest struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Reason 保留原因.
	Reason string `json:"reason,omitempty"`
	// SetBy 设置者.
	SetBy string `json:"set_by,omitempty"`
}

// PutObjectLegalHoldResponse 设置对象法律保留响应.
type PutObjectLegalHoldResponse struct {
	// LegalHold 法律保留信息.
	LegalHold *LegalHold `json:"legal_hold"`
}

// GetObjectLegalHoldResponse 获取对象法律保留响应.
type GetObjectLegalHoldResponse struct {
	// LegalHold 法律保留信息.
	LegalHold *LegalHold `json:"legal_hold"`
}

// PutBucketObjectLockRequest 设置桶对象锁定配置请求.
type PutBucketObjectLockRequest struct {
	// ObjectLockConfig 对象锁定配置.
	ObjectLockConfig ObjectLockConfiguration `json:"object_lock_config"`
}

// PutBucketObjectLockResponse 设置桶对象锁定配置响应.
type PutBucketObjectLockResponse struct {
	// Bucket 桶信息.
	Bucket *ImmutableBucket `json:"bucket"`
}

// GetBucketObjectLockResponse 获取桶对象锁定配置响应.
type GetBucketObjectLockResponse struct {
	// ObjectLockConfig 对象锁定配置.
	ObjectLockConfig *ObjectLockConfiguration `json:"object_lock_config"`
}

// ========== 查询类型 ==========

// ListObjectsRequest 列出对象请求.
type ListObjectsRequest struct {
	// BucketName 桶名称.
	BucketName string `json:"bucket_name"`
	// Prefix 前缀过滤.
	Prefix string `json:"prefix,omitempty"`
	// MaxKeys 最大返回数量.
	MaxKeys int `json:"max_keys,omitempty"`
	// ContinuationToken 分页令牌.
	ContinuationToken string `json:"continuation_token,omitempty"`
}

// ListObjectsResponse 列出对象响应.
type ListObjectsResponse struct {
	// Objects 对象列表.
	Objects []*ImmutableObject `json:"objects"`
	// IsTruncated 是否有更多结果.
	IsTruncated bool `json:"is_truncated"`
	// NextContinuationToken 下一页令牌.
	NextContinuationToken string `json:"next_continuation_token,omitempty"`
}

// ListAuditLogsRequest 列出审计日志请求.
type ListAuditLogsRequest struct {
	// ObjectKey 对象键过滤.
	ObjectKey string `json:"object_key,omitempty"`
	// BucketName 桶名称过滤.
	BucketName string `json:"bucket_name,omitempty"`
	// Action 操作类型过滤.
	Action AuditAction `json:"action,omitempty"`
	// StartTime 开始时间.
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间.
	EndTime *time.Time `json:"end_time,omitempty"`
	// MaxResults 最大返回数量.
	MaxResults int `json:"max_results,omitempty"`
}

// ListAuditLogsResponse 列出审计日志响应.
type ListAuditLogsResponse struct {
	// Logs 审计日志列表.
	Logs []*RetentionAuditEntry `json:"logs"`
	// Total 总数.
	Total int `json:"total"`
}

// ========== 统计类型 ==========

// WORMStats WORM 统计信息.
type WORMStats struct {
	// TotalBuckets 桶总数.
	TotalBuckets int `json:"total_buckets"`
	// TotalObjects 对象总数.
	TotalObjects int `json:"total_objects"`
	// ProtectedObjects WORM 保护对象数.
	ProtectedObjects int `json:"protected_objects"`
	// LegalHoldObjects 法律保留对象数.
	LegalHoldObjects int `json:"legal_hold_objects"`
	// TotalSize 总大小（字节）.
	TotalSize int64 `json:"total_size"`
	// AuditLogCount 审计日志数量.
	AuditLogCount int `json:"audit_log_count"`
}
