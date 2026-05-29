// Package s3gateway 提供S3兼容对象存储网关功能
package s3gateway

import (
	"time"
)

// BucketPolicy 存储桶访问策略
type BucketPolicy string

const (
	PolicyPrivate BucketPolicy = "private"
	PolicyPublic  BucketPolicy = "public"
	PolicyCustom  BucketPolicy = "custom"
)

// StorageClass 存储类别
type StorageClass string

const (
	StorageClassStandard    StorageClass = "STANDARD"
	StorageClassInfrequent  StorageClass = "STANDARD_IA"
	StorageClassArchive     StorageClass = "GLACIER"
)

// Permission ACL权限类型
type Permission string

const (
	PermissionRead        Permission = "READ"
	PermissionWrite       Permission = "WRITE"
	PermissionReadACP     Permission = "READ_ACP"
	PermissionWriteACP    Permission = "WRITE_ACP"
	PermissionFullControl Permission = "FULL_CONTROL"
)

// GranteeType 被授权者类型
type GranteeType string

const (
	GranteeUser  GranteeType = "CanonicalUser"
	GranteeGroup GranteeType = "Group"
)

// VersioningStatus 版本控制状态
type VersioningStatus string

const (
	VersioningEnabled   VersioningStatus = "Enabled"
	VersioningSuspended VersioningStatus = "Suspended"
)

// LifecycleAction 生命周期动作类型
type LifecycleAction string

const (
	ActionExpiration  LifecycleAction = "Expiration"
	ActionTransition  LifecycleAction = "Transition"
)

// S3Config S3网关配置
type S3Config struct {
	StorageRoot   string        `json:"storageRoot"`
	DefaultPolicy BucketPolicy  `json:"defaultPolicy"`
	MaxBucketSize int64         `json:"maxBucketSize"`
	MaxObjectSize int64         `json:"maxObjectSize"`
	EnableLogging bool          `json:"enableLogging"`
	Region        string        `json:"region"`
}

// Bucket 存储桶
type Bucket struct {
	Name        string            `json:"name"`
	OwnerID     string            `json:"ownerId"`
	Policy      BucketPolicy      `json:"policy"`
	Quota       BucketQuota       `json:"quota"`
	Versioning  VersioningStatus  `json:"versioning"`
	ACL         []ACLGrant        `json:"acl,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	Region      string            `json:"region"`
}

// BucketQuota 存储桶配额
type BucketQuota struct {
	MaxSize    int64 `json:"maxSize"`
	MaxObjects int64 `json:"maxObjects"`
}

// ACLGrant ACL授权条目
type ACLGrant struct {
	GranteeID   string       `json:"granteeId"`
	GranteeType GranteeType  `json:"granteeType"`
	Permission  Permission   `json:"permission"`
}

// Object 存储对象
type Object struct {
	Key          string            `json:"key"`
	Bucket       string            `json:"bucket"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"contentType"`
	ETag         string            `json:"etag"`
	StorageClass StorageClass      `json:"storageClass"`
	VersionID    string            `json:"versionId,omitempty"`
	IsLatest     bool              `json:"isLatest"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	OwnerID      string            `json:"ownerId"`
	CreatedAt    time.Time         `json:"createdAt"`
	ExpiresAt    *time.Time        `json:"expiresAt,omitempty"`
	Data         []byte            `json:"-"`
}

// ObjectVersion 对象版本
type ObjectVersion struct {
	Key       string    `json:"key"`
	VersionID string    `json:"versionId"`
	Size      int64     `json:"size"`
	ETag      string    `json:"etag"`
	IsLatest  bool      `json:"isLatest"`
	CreatedAt time.Time `json:"createdAt"`
	OwnerID   string    `json:"ownerId"`
}

// MultipartUpload 多部分上传
type MultipartUpload struct {
	UploadID    string            `json:"uploadId"`
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	OwnerID     string            `json:"ownerId"`
	ContentType string            `json:"contentType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Parts       []UploadPart      `json:"parts,omitempty"`
	InitiatedAt time.Time         `json:"initiatedAt"`
}

// UploadPart 上传分片
type UploadPart struct {
	PartNumber int       `json:"partNumber"`
	Size       int64     `json:"size"`
	ETag       string    `json:"etag"`
	CreatedAt  time.Time `json:"createdAt"`
	Data       []byte    `json:"-"`
}

// AccessKey 访问密钥
type AccessKey struct {
	AccessKeyID     string    `json:"accessKeyId"`
	SecretAccessKey string    `json:"secretAccessKey"`
	OwnerID         string    `json:"ownerId"`
	Description     string    `json:"description,omitempty"`
	Status          string    `json:"status"` // Active, Inactive
	CreatedAt       time.Time `json:"createdAt"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
}

// LifecycleRule 生命周期规则
type LifecycleRule struct {
	ID              string          `json:"id"`
	Bucket          string          `json:"bucket"`
	Prefix          string          `json:"prefix"`
	Enabled         bool            `json:"enabled"`
	ExpirationDays  int             `json:"expirationDays"`
	TransitionDays  int             `json:"transitionDays"`
	TargetClass     StorageClass    `json:"targetClass"`
	Action          LifecycleAction `json:"action"`
}

// AccessLog 访问日志
type AccessLog struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"userId"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Operation string    `json:"operation"`
	Status    int       `json:"status"`
	Size      int64     `json:"size"`
	Duration  int64     `json:"durationMs"`
	UserAgent string    `json:"userAgent,omitempty"`
	IPAddress string    `json:"ipAddress,omitempty"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	TotalBuckets int64                 `json:"totalBuckets"`
	TotalObjects int64                 `json:"totalObjects"`
	TotalSize    int64                 `json:"totalSize"`
	ByUser       map[string]*UserStats  `json:"byUser,omitempty"`
	ByBucket     map[string]*BucketStats `json:"byBucket,omitempty"`
	ByOperation  map[string]int64       `json:"byOperation,omitempty"`
	ByClass      map[string]int64       `json:"byClass,omitempty"`
}

// UserStats 用户维度统计
type UserStats struct {
	Buckets   int64 `json:"buckets"`
	Objects   int64 `json:"objects"`
	TotalSize int64 `json:"totalSize"`
	Uploads   int64 `json:"uploads"`
	Downloads int64 `json:"downloads"`
}

// BucketStats 存储桶维度统计
type BucketStats struct {
	Objects   int64 `json:"objects"`
	TotalSize int64 `json:"totalSize"`
	Uploads   int64 `json:"uploads"`
	Downloads int64 `json:"downloads"`
}
