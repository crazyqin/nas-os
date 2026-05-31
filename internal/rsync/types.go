package rsync

import "time"

// RsyncConfig Rsync 配置.
type RsyncConfig struct {
	MaxConcurrent  int           `json:"max_concurrent"`
	BandwidthLimit int           `json:"bandwidth_limit"` // KB/s, 0=无限制
	RetryCount     int           `json:"retry_count"`
	RetryDelay     time.Duration `json:"retry_delay"`
	HistoryLimit   int           `json:"history_limit"`
	DefaultFlags   []string      `json:"default_flags"`
	SSHKeyPath     string        `json:"ssh_key_path"`
	SSHPort        int           `json:"ssh_port"`
	Compress       bool          `json:"compress"`
	Archive        bool          `json:"archive"`
	DeleteExcluded bool          `json:"delete_excluded"`
	DryRun         bool          `json:"dry_run"`
}

// RsyncTarget 同步目标.
type RsyncTarget struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Source       string            `json:"source"`
	Destination  string            `json:"destination"`
	Type         TargetType        `json:"type"`
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Username     string            `json:"username"`
	SSHKey       string            `json:"ssh_key"`
	Options      []string          `json:"options"`
	Exclude      []string          `json:"exclude"`
	Include      []string          `json:"include"`
	Filter       []string          `json:"filter"`
	Attributes   map[string]string `json:"attributes"`
	CreatedAt    time.Time         `json:"created_at"`
}

// TargetType 目标类型.
type TargetType string

const (
	TargetTypeLocal  TargetType = "local"
	TargetTypeSSH    TargetType = "ssh"
	TargetTypeDaemon TargetType = "daemon"
)

// RsyncJob 同步任务.
type RsyncJob struct {
	ID          string     `json:"id"`
	TargetID    string     `json:"target_id"`
	Name        string     `json:"name"`
	Status      JobStatus  `json:"status"`
	Schedule    string     `json:"schedule"`
	NextRun     time.Time  `json:"next_run"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	ErrorMessage string    `json:"error_message,omitempty"`
	RetryCount  int        `json:"retry_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JobStatus 任务状态.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// RsyncResult 同步结果.
type RsyncResult struct {
	JobID            string        `json:"job_id"`
	Source           string        `json:"source"`
	Destination      string        `json:"destination"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	Duration         time.Duration `json:"duration"`
	FilesTransferred int           `json:"files_transferred"`
	FilesSkipped     int           `json:"files_skipped"`
	TotalSize        int64         `json:"total_size"`
	AverageSpeed     int64         `json:"average_speed"` // bytes/s
	ErrorMessage     string        `json:"error_message,omitempty"`
}

// RsyncHistory 同步历史.
type RsyncHistory struct {
	ID          string        `json:"id"`
	JobID       string        `json:"job_id"`
	TargetID    string        `json:"target_id"`
	Status      JobStatus     `json:"status"`
	FilesSynced int           `json:"files_synced"`
	BytesSynced int64         `json:"bytes_synced"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

// RsyncStats 统计信息.
type RsyncStats struct {
	TotalTargets  int   `json:"total_targets"`
	TotalJobs     int   `json:"total_jobs"`
	CompletedJobs int   `json:"completed_jobs"`
	FailedJobs    int   `json:"failed_jobs"`
	TotalFiles    int   `json:"total_files"`
	TotalBytes    int64 `json:"total_bytes"`
}
