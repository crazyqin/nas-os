// Package s3 implements S3-compatible object storage for NAS-OS
// Compatible with MinIO S3 API
package s3

import (
	"time"
)

// Bucket represents an S3 bucket.
type Bucket struct {
	Name       string            `json:"name"`
	CreatedAt  time.Time         `json:"createdAt"`
	Versioning VersioningConfig  `json:"versioning"`
	Policy     *BucketPolicy     `json:"policy,omitempty"`
	Quota      *QuotaConfig      `json:"quota,omitempty"`
	Encryption *EncryptionConfig `json:"encryption,omitempty"`
	ObjectLock *ObjectLockConfig `json:"objectLock,omitempty"`
	Owner      string            `json:"owner"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// BucketInput for creating buckets.
type BucketInput struct {
	Name       string            `json:"name" binding:"required"`
	Versioning *VersioningConfig `json:"versioning"`
	Quota      *QuotaConfig      `json:"quota"`
	Encryption *EncryptionConfig `json:"encryption"`
	ObjectLock *ObjectLockConfig `json:"objectLock"`
	Tags       map[string]string `json:"tags"`
}

// Object represents an S3 object.
type Object struct {
	Key          string            `json:"key"`
	Bucket       string            `json:"bucket"`
	Size         int64             `json:"size"`
	ETag         string            `json:"etag"`
	ContentType  string            `json:"contentType"`
	LastModified time.Time         `json:"lastModified"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	VersionID    string            `json:"versionId,omitempty"`
	IsLatest     bool              `json:"isLatest"`
	StorageClass StorageClass      `json:"storageClass"`
}

// ObjectInfo contains object metadata without body.
type ObjectInfo struct {
	Key          string            `json:"key"`
	Bucket       string            `json:"bucket"`
	Size         int64             `json:"size"`
	ETag         string            `json:"etag"`
	ContentType  string            `json:"contentType"`
	LastModified time.Time         `json:"lastModified"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	VersionID    string            `json:"versionId,omitempty"`
	StorageClass StorageClass      `json:"storageClass"`
}

// ObjectList represents a list of objects.
type ObjectList struct {
	Bucket                string        `json:"bucket"`
	Prefix                string        `json:"prefix"`
	Delimiter             string        `json:"delimiter,omitempty"`
	IsTruncated           bool          `json:"isTruncated"`
	NextMarker            string        `json:"nextMarker,omitempty"`
	NextContinuationToken string        `json:"nextContinuationToken,omitempty"`
	MaxKeys               int           `json:"maxKeys"`
	CommonPrefixes        []string      `json:"commonPrefixes,omitempty"`
	Objects               []*ObjectInfo `json:"objects"`
}

// VersioningConfig for bucket versioning.
type VersioningConfig struct {
	Status    VersioningStatus `json:"status"`    // Enabled, Suspended
	MFADelete bool             `json:"mfaDelete"` // MFA delete enabled
}

// VersioningStatus represents versioning status.
type VersioningStatus string

// Versioning status constants.
const (
	// VersioningEnabled 表示版本控制已启用.
	VersioningEnabled   VersioningStatus = "Enabled"
	VersioningSuspended VersioningStatus = "Suspended"
)

// BucketPolicy represents bucket access policy (AWS IAM policy JSON).
type BucketPolicy struct {
	Version   string            `json:"version"`
	Statement []PolicyStatement `json:"statement"`
}

// PolicyStatement represents a policy statement.
type PolicyStatement struct {
	SID       string           `json:"sid,omitempty"`
	Effect    string           `json:"effect"` // Allow, Deny
	Principal interface{}      `json:"principal,omitempty"`
	Action    []string         `json:"action"`
	Resource  interface{}      `json:"resource"`
	Condition *PolicyCondition `json:"condition,omitempty"`
}

// PolicyCondition represents policy conditions.
type PolicyCondition struct {
	StringEquals    map[string]string `json:"StringEquals,omitempty"`
	StringNotEquals map[string]string `json:"StringNotEquals,omitempty"`
	StringLike      map[string]string `json:"StringLike,omitempty"`
	IPAddress       map[string]string `json:"IpAddress,omitempty"`
	NotIPAddress    map[string]string `json:"NotIpAddress,omitempty"`
}

// QuotaConfig for bucket quota.
type QuotaConfig struct {
	Enabled bool  `json:"enabled"`
	Size    int64 `json:"size"` // Max size in bytes
}

// EncryptionConfig for server-side encryption.
type EncryptionConfig struct {
	Algorithm string `json:"algorithm"` // AES256, aws:kms
	KMSKeyID  string `json:"kmsKeyId,omitempty"`
}

// ObjectLockConfig for object lock (WORM).
type ObjectLockConfig struct {
	Enabled          bool             `json:"enabled"`
	DefaultRetention *RetentionConfig `json:"defaultRetention,omitempty"`
}

// RetentionConfig for object retention.
type RetentionConfig struct {
	Mode  RetentionMode `json:"mode"`  // GOVERNANCE, COMPLIANCE
	Days  int           `json:"days"`  // Retention period in days
	Years int           `json:"years"` // Retention period in years
}

// RetentionMode represents retention mode.
type RetentionMode string

// Retention mode constants.
const (
	// RetentionGovernance 表示治理模式.
	RetentionGovernance RetentionMode = "GOVERNANCE"
	RetentionCompliance RetentionMode = "COMPLIANCE"
)

// StorageClass represents storage tier.
type StorageClass string

// Storage class constants.
const (
	// StorageClassStandard 表示标准存储.
	StorageClassStandard          StorageClass = "STANDARD"
	StorageClassReducedRedundancy StorageClass = "REDUCED_REDUNDANCY"
	StorageClassGlacier           StorageClass = "GLACIER"
	StorageClassDeepArchive       StorageClass = "DEEP_ARCHIVE"
)

// PresignedURL represents a presigned URL.
type PresignedURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
	Method    string    `json:"method"` // GET, PUT, DELETE
	ObjectKey string    `json:"objectKey"`
	Bucket    string    `json:"bucket"`
}

// PresignedURLRequest for generating presigned URLs.
type PresignedURLRequest struct {
	Bucket      string        `json:"bucket" binding:"required"`
	Key         string        `json:"key" binding:"required"`
	Method      string        `json:"method"`                // GET, PUT, DELETE (default: GET)
	Duration    time.Duration `json:"duration"`              // URL validity duration
	ContentType string        `json:"contentType,omitempty"` // For PUT
}

// UploadConfig for multipart uploads.
type UploadConfig struct {
	Bucket       string            `json:"bucket" binding:"required"`
	Key          string            `json:"key" binding:"required"`
	ContentType  string            `json:"contentType"`
	Metadata     map[string]string `json:"metadata"`
	StorageClass StorageClass      `json:"storageClass"`
}

// MultipartUpload represents a multipart upload.
type MultipartUpload struct {
	UploadID  string    `json:"uploadId"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
	Owner     string    `json:"owner"`
	Parts     []*Part   `json:"parts,omitempty"`
}

// Part represents an uploaded part.
type Part struct {
	PartNumber   int       `json:"partNumber"`
	ETag         string    `json:"etag"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
}

// CompletedPart for completing multipart upload.
type CompletedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

// CompleteMultipartUploadInput for completing upload.
type CompleteMultipartUploadInput struct {
	UploadID string           `json:"uploadId"`
	Parts    []*CompletedPart `json:"parts"`
}

// Config for S3 service.
type Config struct {
	Enabled       bool   `json:"enabled"`
	BindAddress   string `json:"bindAddress"`
	Port          int    `json:"port"`
	Domain        string `json:"domain"`
	Region        string `json:"region"`
	AccessKey     string `json:"accessKey"`
	SecretKey     string `json:"secretKey"`
	MaxUploadSize int64  `json:"maxUploadSize"` // Max object size
}

// BucketStats represents bucket statistics.
type BucketStats struct {
	Name         string    `json:"name"`
	ObjectCount  int64     `json:"objectCount"`
	Size         int64     `json:"size"`
	VersionCount int64     `json:"versionCount"`
	LastModified time.Time `json:"lastModified"`
}

// Errors.
var (
	ErrBucketNotFound      = &S3Error{Code: 404, CodeStr: "NoSuchBucket", Message: "The specified bucket does not exist"}
	ErrBucketExists        = &S3Error{Code: 409, CodeStr: "BucketAlreadyExists", Message: "The requested bucket name is not available"}
	ErrBucketNotEmpty      = &S3Error{Code: 409, CodeStr: "BucketNotEmpty", Message: "The bucket you tried to delete is not empty"}
	ErrObjectNotFound      = &S3Error{Code: 404, CodeStr: "NoSuchKey", Message: "The specified key does not exist"}
	ErrInvalidBucketName   = &S3Error{Code: 400, CodeStr: "InvalidBucketName", Message: "The specified bucket is not valid"}
	ErrInvalidKey          = &S3Error{Code: 400, CodeStr: "InvalidKey", Message: "The specified key is not valid"}
	ErrAccessDenied        = &S3Error{Code: 403, CodeStr: "AccessDenied", Message: "Access Denied"}
	ErrQuotaExceeded       = &S3Error{Code: 403, CodeStr: "QuotaExceeded", Message: "Quota exceeded for this bucket"}
	ErrInvalidUploadID     = &S3Error{Code: 404, CodeStr: "NoSuchUpload", Message: "The specified multipart upload does not exist"}
	ErrEntityTooLarge      = &S3Error{Code: 400, CodeStr: "EntityTooLarge", Message: "Your proposed upload exceeds the maximum allowed size"}
	ErrInvalidPart         = &S3Error{Code: 400, CodeStr: "InvalidPart", Message: "One or more of the specified parts could not be found"}
	ErrInvalidPartOrder    = &S3Error{Code: 400, CodeStr: "InvalidPartOrder", Message: "The list of parts was not in ascending order"}
	ErrInvalidStorageClass = &S3Error{Code: 400, CodeStr: "InvalidStorageClass", Message: "The storage class you specified is not valid"}
)

// S3Error represents an S3 error.
type S3Error struct {
	Code     int
	CodeStr  string
	Message  string
	Resource string
}

func (e *S3Error) Error() string {
	return e.Message
}

// ToXML returns XML error response.
func (e *S3Error) ToXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>` + e.CodeStr + `</Code>
  <Message>` + e.Message + `</Message>
  <Resource>` + e.Resource + `</Resource>
</Error>`
}
