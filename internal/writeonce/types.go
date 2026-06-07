package writeonce

import (
	"time"
)

// RetentionMode defines how long files are protected
type RetentionMode string

const (
	RetentionModeFixed    RetentionMode = "fixed"    // Fixed retention period in days
	RetentionModeForever  RetentionMode = "forever"  // Permanent lock, never expires
	RetentionModeUnlocked RetentionMode = "unlocked" // Not yet locked
)

// PolicyMode defines the WORM policy enforcement level
type PolicyMode string

const (
	PolicyModeEnterprise PolicyMode = "enterprise" // Standard enterprise WORM
	PolicyModeCompliance PolicyMode = "compliance" // Regulatory compliance mode (stricter)
)

// FolderState represents the current state of a WriteOnce folder
type FolderState string

const (
	FolderStateOpen    FolderState = "open"    // Folder is open for writes
	FolderStateLocked  FolderState = "locked"  // Folder is locked, read-only
	FolderStateExpired FolderState = "expired" // Retention period expired
)

// WriteOnceConfig represents the configuration for WriteOnce feature
type WriteOnceConfig struct {
	Enabled              bool `json:"enabled"`
	DefaultRetentionDays int  `json:"default_retention_days"`
	MaxRetentionDays     int  `json:"max_retention_days"`
	AllowForeverLock     bool `json:"allow_forever_lock"`
	ComplianceMode       bool `json:"compliance_mode"`
	AdminBypassLock      bool `json:"admin_bypass_lock"`
	AuditLogEnabled      bool `json:"audit_log_enabled"`
}

// WriteOnceFolder represents a WriteOnce protected folder
type WriteOnceFolder struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Path           string        `json:"path"`
	State          FolderState   `json:"state"`
	RetentionMode  RetentionMode `json:"retention_mode"`
	RetentionDays  int           `json:"retention_days"`
	PolicyMode     PolicyMode    `json:"policy_mode"`
	LockedAt       *time.Time    `json:"locked_at,omitempty"`
	ExpiresAt      *time.Time    `json:"expires_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	CreatedBy      string        `json:"created_by"`
	FileCount      int64         `json:"file_count"`
	TotalSizeBytes int64         `json:"total_size_bytes"`
	Description    string        `json:"description,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
}

// WriteOnceFile represents a file inside a WriteOnce folder
type WriteOnceFile struct {
	ID         string    `json:"id"`
	FolderID   string    `json:"folder_id"`
	FilePath   string    `json:"file_path"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	FileHash   string    `json:"file_hash"`
	IsDeleted  bool      `json:"is_deleted"`
	CreatedAt  time.Time `json:"created_at"`
	UploadedBy string    `json:"uploaded_by"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID        string    `json:"id"`
	FolderID  string    `json:"folder_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// CreateFolderRequest represents a request to create a WriteOnce folder
type CreateFolderRequest struct {
	Name          string        `json:"name"`
	Path          string        `json:"path"`
	RetentionMode RetentionMode `json:"retention_mode"`
	RetentionDays int           `json:"retention_days"`
	PolicyMode    PolicyMode    `json:"policy_mode"`
	Description   string        `json:"description,omitempty"`
	Tags          []string      `json:"tags,omitempty"`
	CreatedBy     string        `json:"created_by"`
}

// LockFolderRequest represents a request to lock a folder
type LockFolderRequest struct {
	FolderID string `json:"folder_id"`
	UserID   string `json:"user_id"`
}

// AddFileRequest represents a request to add a file to a folder
type AddFileRequest struct {
	FolderID   string `json:"folder_id"`
	FilePath   string `json:"file_path"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileHash   string `json:"file_hash"`
	UploadedBy string `json:"uploaded_by"`
}
