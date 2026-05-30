// Package backupverify provides backup verification and restore testing for NAS-OS
// Features: Integrity checks, restore testing, health scoring, auto-repair
package backupverify

import (
	"time"
)

// VerifyType represents the type of verification
type VerifyType string

const (
	VerifyIntegrity VerifyType = "integrity"
	VerifyRestore   VerifyType = "restore"
	VerifyFull      VerifyType = "full"
)

// TaskStatus represents the status of a verification task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// ResultStatus represents the result of a verification
type ResultStatus string

const (
	ResultPass ResultStatus = "pass"
	ResultWarn ResultStatus = "warn"
	ResultFail ResultStatus = "fail"
)

// FileStatus represents the verification status of a single file
type FileStatus string

const (
	FileStatusPass             FileStatus = "pass"
	FileStatusCorrupt          FileStatus = "corrupt"
	FileStatusMissing          FileStatus = "missing"
	FileStatusChecksumMismatch FileStatus = "checksum_mismatch"
)

// RestoreStatus represents the status of a restore test
type RestoreStatus string

const (
	RestoreStatusPending    RestoreStatus = "pending"
	RestoreStatusExtracting RestoreStatus = "extracting"
	RestoreStatusVerifying  RestoreStatus = "verifying"
	RestoreStatusCleanup    RestoreStatus = "cleanup"
	RestoreStatusCompleted  RestoreStatus = "completed"
	RestoreStatusFailed     RestoreStatus = "failed"
)

// RiskLevel represents the risk level of a backup
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// VerifyTask represents a backup verification task
type VerifyTask struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	BackupID    string     `json:"backup_id"`
	BackupPath  string     `json:"backup_path"`
	VerifyType  VerifyType `json:"verify_type"`
	Schedule    string     `json:"schedule"`
	Status      TaskStatus `json:"status"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
}

// VerifyResult represents the result of a verification run
type VerifyResult struct {
	ID             string           `json:"id"`
	TaskID         string           `json:"task_id"`
	BackupID       string           `json:"backup_id"`
	Status         ResultStatus     `json:"status"`
	FileCount      int              `json:"file_count"`
	VerifiedFiles  int              `json:"verified_files"`
	CorruptedFiles int              `json:"corrupted_files"`
	MissingFiles   int              `json:"missing_files"`
	TotalBytes     int64            `json:"total_bytes"`
	VerifiedBytes  int64            `json:"verified_bytes"`
	Duration       time.Duration    `json:"duration"`
	ErrorMessage   string           `json:"error_message,omitempty"`
	Details        []VerifyDetail   `json:"details,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// VerifyDetail represents the verification detail of a single file
type VerifyDetail struct {
	FilePath         string     `json:"file_path"`
	Status           FileStatus `json:"status"`
	ExpectedChecksum string     `json:"expected_checksum,omitempty"`
	ActualChecksum   string     `json:"actual_checksum,omitempty"`
	ExpectedSize     int64      `json:"expected_size"`
	ActualSize       int64      `json:"actual_size"`
}

// RestoreTest represents a restore test result
type RestoreTest struct {
	ID            string        `json:"id"`
	TaskID        string        `json:"task_id"`
	BackupID      string        `json:"backup_id"`
	TargetPath    string        `json:"target_path"`
	Status        RestoreStatus `json:"status"`
	RestoredFiles int           `json:"restored_files"`
	VerifiedFiles int           `json:"verified_files"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// BackupHealth represents the health status of a backup
type BackupHealth struct {
	BackupID        string   `json:"backup_id"`
	Source          string   `json:"source"`
	LastBackup      *time.Time `json:"last_backup,omitempty"`
	BackupSize      int64    `json:"backup_size"`
	VerifyStatus    string   `json:"verify_status"`
	RestoreStatus   string   `json:"restore_status"`
	IntegrityScore  float64  `json:"integrity_score"` // 0-100
	RiskLevel       RiskLevel `json:"risk_level"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// VerifyReport represents a verification report for all backups
type VerifyReport struct {
	GeneratedAt    time.Time      `json:"generated_at"`
	TotalBackups   int            `json:"total_backups"`
	HealthyBackups int            `json:"healthy_backups"`
	WarningBackups int            `json:"warning_backups"`
	FailedBackups  int            `json:"failed_backups"`
	Backups        []BackupHealth `json:"backups"`
}
