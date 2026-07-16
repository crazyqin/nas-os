// Package saasbackup 提供 SaaS 应用数据备份与恢复功能，
// 支持 Microsoft 365 和 Google Workspace 等主流 SaaS 平台的数据保护。
package saasbackup

import "time"

// ========== SaaS 提供商类型 ==========

// SaaSProvider SaaS 提供商类型.
type SaaSProvider string

const (
	// ProviderMicrosoft365 Microsoft 365.
	ProviderMicrosoft365 SaaSProvider = "microsoft365"
	// ProviderGoogleWorkspace Google Workspace.
	ProviderGoogleWorkspace SaaSProvider = "google_workspace"
)

// ========== 租户类型 ==========

// SaaSTenant SaaS 租户连接信息.
type SaaSTenant struct {
	ID          string       `json:"id"`
	Provider    SaaSProvider `json:"provider"`
	Domain      string       `json:"domain"`
	AdminEmail  string       `json:"admin_email"`
	ConnectedAt time.Time    `json:"connected_at"`
	Status      TenantStatus `json:"status"`
}

// TenantStatus 租户连接状态.
type TenantStatus string

const (
	// TenantStatusConnected 已连接.
	TenantStatusConnected TenantStatus = "connected"
	// TenantStatusDisconnected 已断开.
	TenantStatusDisconnected TenantStatus = "disconnected"
	// TenantStatusError 连接错误.
	TenantStatusError TenantStatus = "error"
)

// ========== 备份任务类型 ==========

// BackupJob 备份任务.
type BackupJob struct {
	ID            string       `json:"id"`
	Provider      SaaSProvider `json:"provider"`
	TenantID      string       `json:"tenant_id"`
	UserID        string       `json:"user_id"`
	ResourceType  ResourceType `json:"resource_type"`
	Schedule      string       `json:"schedule,omitempty"` // cron 表达式
	LastRun       *time.Time   `json:"last_run,omitempty"`
	NextRun       *time.Time   `json:"next_run,omitempty"`
	Status        JobStatus    `json:"status"`
	RetentionDays int          `json:"retention_days"`
	ItemCount     int          `json:"item_count"`
	SizeBytes     int64        `json:"size_bytes"`
}

// ResourceType 备份资源类型.
type ResourceType string

const (
	// ResourceMail 邮件.
	ResourceMail ResourceType = "mail"
	// ResourceDrive 云盘/网盘.
	ResourceDrive ResourceType = "drive"
	// ResourceContacts 联系人.
	ResourceContacts ResourceType = "contacts"
	// ResourceCalendar 日历.
	ResourceCalendar ResourceType = "calendar"
)

// JobStatus 任务状态.
type JobStatus string

const (
	// JobStatusIdle 空闲.
	JobStatusIdle JobStatus = "idle"
	// JobStatusRunning 运行中.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted 已完成.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed 失败.
	JobStatusFailed JobStatus = "failed"
)

// ========== 备份项类型 ==========

// BackupItem 单个备份项.
type BackupItem struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	SourcePath string    `json:"source_path"`
	BackupPath string    `json:"backup_path"`
	SizeBytes  int64     `json:"size_bytes"`
	Checksum   string    `json:"checksum"`
	CreatedAt  time.Time `json:"created_at"`
	ItemType   string    `json:"item_type"`
}

// ========== 恢复请求类型 ==========

// RestoreMode 恢复模式.
type RestoreMode string

const (
	// RestoreModeOriginal 恢复到原始位置.
	RestoreModeOriginal RestoreMode = "original"
	// RestoreModeCrossUser 跨用户恢复.
	RestoreModeCrossUser RestoreMode = "cross_user"
)

// RestoreRequest 数据恢复请求.
type RestoreRequest struct {
	JobID        string      `json:"job_id" binding:"required"`
	ItemIDs      []string    `json:"item_ids" binding:"required,min=1"`
	TargetUserID string      `json:"target_user_id,omitempty"`
	RestoreMode  RestoreMode `json:"restore_mode" binding:"required"`
}

// ========== 统计类型 ==========

// BackupStats 备份统计信息.
type BackupStats struct {
	TotalJobs      int        `json:"total_jobs"`
	TotalItems     int        `json:"total_items"`
	TotalSize      int64      `json:"total_size"`
	SuccessRate    float64    `json:"success_rate"`
	LastBackupTime *time.Time `json:"last_backup_time,omitempty"`
}

// ========== 请求类型 ==========

// ConnectTenantRequest 连接 SaaS 租户请求.
type ConnectTenantRequest struct {
	Provider   SaaSProvider `json:"provider" binding:"required,oneof=microsoft365 google_workspace"`
	Domain     string       `json:"domain" binding:"required"`
	AdminEmail string       `json:"admin_email" binding:"required,email"`
}

// CreateJobRequest 创建备份任务请求.
type CreateJobRequest struct {
	TenantID      string       `json:"tenant_id" binding:"required"`
	UserID        string       `json:"user_id" binding:"required"`
	ResourceType  ResourceType `json:"resource_type" binding:"required,oneof=mail drive contacts calendar"`
	Schedule      string       `json:"schedule"`
	RetentionDays int          `json:"retention_days"`
}
