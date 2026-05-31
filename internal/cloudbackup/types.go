// Package cloudbackup 实现 M365/Google Workspace 云备份模块
// 支持 Microsoft 365（OneDrive/SharePoint/Exchange/Teams）和 Google Workspace（Drive/Gmail/Calendar）备份
package cloudbackup

import (
	"errors"
	"time"
)

var (
	ErrProviderNotFound   = errors.New("provider not found")
	ErrJobNotFound        = errors.New("job not found")
	ErrJobAlreadyRunning  = errors.New("job already running")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRestoreFailed      = errors.New("restore failed")
)

// CloudProvider 云服务提供商.
type CloudProvider string

const (
	ProviderM365 CloudProvider = "microsoft365"
	ProviderGWS  CloudProvider = "google_workspace"
)

// ServiceType 服务类型.
type ServiceType string

const (
	ServiceOneDrive   ServiceType = "onedrive"
	ServiceSharePoint ServiceType = "sharepoint"
	ServiceExchange   ServiceType = "exchange"
	ServiceTeams      ServiceType = "teams"
	ServiceGoogleDrive ServiceType = "google_drive"
	ServiceGmail      ServiceType = "gmail"
	ServiceCalendar   ServiceType = "calendar"
	ServiceContacts   ServiceType = "contacts"
)

// BackupJobStatus 备份任务状态.
type BackupJobStatus string

const (
	JobPending   BackupJobStatus = "pending"
	JobRunning   BackupJobStatus = "running"
	JobCompleted BackupJobStatus = "completed"
	JobFailed    BackupJobStatus = "failed"
	JobCancelled BackupJobStatus = "cancelled"
)

// CloudAccount 云账号配置.
type CloudAccount struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Provider     CloudProvider `json:"provider"`
	TenantID     string        `json:"tenant_id"`
	ClientID     string        `json:"client_id"`
	Domain       string        `json:"domain"`
	Status       string        `json:"status"` // active, expired, error
	LastSync     time.Time     `json:"last_sync"`
	CreatedAt    time.Time     `json:"created_at"`
	EnabledServices []ServiceType `json:"enabled_services"`
}

// BackupJob 备份任务.
type BackupJob struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id"`
	Provider    CloudProvider   `json:"provider"`
	Services    []ServiceType   `json:"services"`
	Status      BackupJobStatus `json:"status"`
	Progress    float64         `json:"progress"` // 0-100
	TotalItems  int64           `json:"total_items"`
	BackedItems int64           `json:"backed_items"`
	TotalBytes  int64           `json:"total_bytes"`
	BackedBytes int64           `json:"backed_bytes"`
	ErrorMsg    string          `json:"error_msg"`
	StartedAt   time.Time       `json:"started_at"`
	FinishedAt  time.Time       `json:"finished_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// BackupSchedule 备份计划.
type BackupSchedule struct {
	ID        string        `json:"id"`
	AccountID string        `json:"account_id"`
	Services  []ServiceType `json:"services"`
	CronExpr  string        `json:"cron_expr"`
	Enabled   bool          `json:"enabled"`
	LastRun   time.Time     `json:"last_run"`
	NextRun   time.Time     `json:"next_run"`
	Retention int           `json:"retention_days"`
}

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	ID        string      `json:"id"`
	JobID     string      `json:"job_id"`
	Service   ServiceType `json:"service"`
	ItemID    string      `json:"item_id"`
	Target    string      `json:"target"` // 恢复目标路径
	Status    string      `json:"status"`
	Progress  float64     `json:"progress"`
	ErrorMsg  string      `json:"error_msg"`
	CreatedAt time.Time   `json:"created_at"`
}

// BackupStats 备份统计.
type BackupStats struct {
	TotalAccounts int            `json:"total_accounts"`
	TotalJobs     int            `json:"total_jobs"`
	SuccessJobs   int            `json:"success_jobs"`
	FailedJobs    int            `json:"failed_jobs"`
	TotalItems    int64          `json:"total_items"`
	TotalBytes    int64          `json:"total_bytes"`
	ProviderStats map[string]int `json:"provider_stats"`
	ServiceStats  map[string]int `json:"service_stats"`
}

// BackupConfig 备份配置.
type BackupConfig struct {
	StoragePath    string        `json:"storage_path"`
	MaxConcurrent  int           `json:"max_concurrent"`
	ChunkSize      int64         `json:"chunk_size"`
	EnableEncrypt  bool          `json:"enable_encrypt"`
	EncryptKey     string        `json:"encrypt_key"`
	RetryCount     int           `json:"retry_count"`
	RetryDelay     time.Duration `json:"retry_delay"`
}
