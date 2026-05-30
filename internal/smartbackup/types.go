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

// BackupType 备份类型
// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull         BackupType = "full"         // 全量备份
	BackupTypeIncremental  BackupType = "incremental"  // 增量备份
	BackupTypeDifferential BackupType = "differential" // 差异备份
)

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusSuccess BackupStatus = "success"
	BackupStatusFailed  BackupStatus = "failed"
)

// BackupPolicy 备份策略
type BackupPolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	SourcePaths []string     `json:"source_paths"`
	TargetIDs   []string     `json:"target_ids"`
	BackupType  BackupType   `json:"backup_type"`
	Schedule    string       `json:"schedule"`
	RPO         *RPORequirements `json:"rpo,omitempty"`
	RTO         *RTORequirements `json:"rto,omitempty"`
	Status      BackupStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// BackupExecution 备份执行记录
type BackupExecution struct {
	ID         string       `json:"id"`
	PolicyID   string       `json:"policy_id"`
	BackupType BackupType   `json:"backup_type"`
	Status     BackupStatus `json:"status"`
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	SizeBytes  int64        `json:"size_bytes"`
	Error      string       `json:"error,omitempty"`
}

// RPORequirements RPO 要求
type RPORequirements struct {
	MaxDataLoss time.Duration `json:"max_data_loss"`
}

// RTORequirements RTO 要求
type RTORequirements struct {
	MaxRecoveryTime time.Duration `json:"max_recovery_time"`
}

// StrategyAnalysis 策略分析
type StrategyAnalysis struct {
	DataType        string            `json:"data_type"`
	DataSizeGB      float64           `json:"data_size_gb"`
	ChangeFrequency *ChangeFrequency  `json:"change_frequency,omitempty"`
	Requirements    *RPORequirements  `json:"requirements,omitempty"`
	RTORequirements *RTORequirements  `json:"rto_requirements,omitempty"`
}

// ChangeFrequency 数据变化频率
type ChangeFrequency struct {
	ChangeRate    float64 `json:"change_rate"`
	DailyChanges  int     `json:"daily_changes"`
}

// BackupStrategy 备份策略推荐
type BackupStrategy struct {
	RecommendedType BackupType `json:"recommended_type"`
	Reason          string     `json:"reason"`
	EstimatedSize   float64    `json:"estimated_size"`
	EstimatedTime   float64    `json:"estimated_time"`
	RPOFeasible     bool       `json:"rpo_feasible"`
	RTOFeasible     bool       `json:"rto_feasible"`
	Recommendations []string   `json:"recommendations"`
	Confidence      float64    `json:"confidence"`
}

// WindowOptimization 备份窗口优化
type WindowOptimization struct {
	RecommendedStart int      `json:"recommended_start"`
	RecommendedEnd   int      `json:"recommended_end"`
	PeakHours        []int    `json:"peak_hours"`
	OffPeakHours     []int    `json:"off_peak_hours"`
	Reason           string   `json:"reason"`
	Suggestions      []string `json:"suggestions"`
}

// PolicyEvaluation 策略评估
type PolicyEvaluation struct {
	PolicyID        string   `json:"policy_id"`
	TotalExecutions int      `json:"total_executions"`
	FailedExecutions int     `json:"failed_executions"`
	SuccessRate     float64  `json:"success_rate"`
	AvgDuration     string   `json:"avg_duration"`
	RPOCompliance   bool     `json:"rpo_compliance"`
	RTOCompliance   bool     `json:"rto_compliance"`
	Score           float64  `json:"score"`
	Recommendations []string `json:"recommendations"`
}
