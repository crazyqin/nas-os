package smartbackup

import (
	"fmt"
	"time"
)

// BackupJob represents a backup job configuration
type BackupJob struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	SourcePath      string    `json:"source_path"`
	DestinationPath string    `json:"destination_path"`
	Schedule        string    `json:"schedule"`
	RetentionPolicy string    `json:"retention_policy"`
	Compression     bool      `json:"compression"`
	Encryption      bool      `json:"encryption"`
	Incremental     bool      `json:"incremental"`
	Status          string    `json:"status"`
	LastRun         *time.Time `json:"last_run,omitempty"`
	NextRun         *time.Time `json:"next_run,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// BackupHistory represents a backup execution history
type BackupHistory struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Duration    int       `json:"duration_seconds"`
	FilesTotal  int       `json:"files_total"`
	FilesCopied int       `json:"files_copied"`
	FilesFailed int       `json:"files_failed"`
	SizeTotal   int64     `json:"size_total"`
	SizeCopied  int64     `json:"size_copied"`
	Error       string    `json:"error,omitempty"`
}

// RestoreRequest represents a restore request
type RestoreRequest struct {
	BackupID    string `json:"backup_id"`
	Destination string `json:"destination"`
	Overwrite   bool   `json:"overwrite"`
	Selective   bool   `json:"selective"`
	FilePattern string `json:"file_pattern"`
}

// RestoreStatus represents restore operation status
type RestoreStatus struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	FilesTotal  int       `json:"files_total"`
	FilesRestored int     `json:"files_restored"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// BackupStats represents backup statistics
type BackupStats struct {
	TotalJobs      int           `json:"total_jobs"`
	ActiveJobs     int           `json:"active_jobs"`
	TotalBackups   int           `json:"total_backups"`
	TotalSize      int64         `json:"total_size"`
	LastBackup     *time.Time    `json:"last_backup,omitempty"`
	NextBackup     *time.Time    `json:"next_backup,omitempty"`
	SuccessRate    float64       `json:"success_rate"`
	RecentBackups  []BackupHistory `json:"recent_backups"`
}

// NewBackupJob creates a new backup job
func NewBackupJob(name, source, destination, schedule string) *BackupJob {
	now := time.Now()
	return &BackupJob{
		ID:              fmt.Sprintf("job-%d", now.UnixNano()),
		Name:            name,
		SourcePath:      source,
		DestinationPath: destination,
		Schedule:        schedule,
		RetentionPolicy: "30d",
		Compression:     true,
		Incremental:     true,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}
