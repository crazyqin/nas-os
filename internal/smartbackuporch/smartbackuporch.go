// Package smartbackuporch 智能备份编排器
// 提供智能备份调度、依赖管理、多目标备份、备份链路优化、失败重试、备份验证功能
// 对标群晖 Active Backup for Business，支持跨平台备份编排
package smartbackuporch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull    BackupType = "full"    // 全量备份
	BackupTypeIncr    BackupType = "incr"    // 增量备份
	BackupTypeDiff    BackupType = "diff"    // 差异备份
	BackupTypeSynth   BackupType = "synth"   // 合成全量备份
	BackupTypeForever BackupType = "forever" // 永久增量备份
)

// BackupStatus 备份状态
type BackupStatus string

const (
	StatusPending    BackupStatus = "pending"    // 等待中
	StatusRunning    BackupStatus = "running"    // 执行中
	StatusCompleted  BackupStatus = "completed"  // 已完成
	StatusFailed     BackupStatus = "failed"     // 已失败
	StatusCancelled  BackupStatus = "cancelled"  // 已取消
	StatusValidating BackupStatus = "validating" // 验证中
	StatusRetrying   BackupStatus = "retrying"   // 重试中
)

// BackupTarget 备份目标类型
type BackupTarget string

const (
	TargetLocal   BackupTarget = "local"   // 本地存储
	TargetRemote  BackupTarget = "remote"  // 远程NAS
	TargetS3      BackupTarget = "s3"      // S3兼容存储
	TargetAzure   BackupTarget = "azure"   // Azure Blob
	TargetGCS     BackupTarget = "gcs"     // Google Cloud Storage
	TargetTape    BackupTarget = "tape"    // 磁带存储
)

// PriorityLevel 优先级
type PriorityLevel int

const (
	PriorityLow    PriorityLevel = 1
	PriorityNormal PriorityLevel = 5
	PriorityHigh   PriorityLevel = 8
	PriorityUrgent PriorityLevel = 10
)

// BackupJob 备份任务
type BackupJob struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Type          BackupType        `json:"type"`
	Source        *SourceConfig     `json:"source"`
	Targets       []*TargetConfig   `json:"targets"`
	Schedule      *ScheduleConfig   `json:"schedule"`
	RetryPolicy   *RetryPolicy      `json:"retry_policy"`
	DependsOn     []string          `json:"depends_on"`     // 依赖的任务ID
	Priority      PriorityLevel     `json:"priority"`
	Enabled       bool              `json:"enabled"`
	Tags          []string          `json:"tags"`
	Retention     *RetentionPolicy  `json:"retention"`
	Encryption    *EncryptionConfig `json:"encryption"`
	Compression   *CompressionConfig `json:"compression"`
	Status        BackupStatus      `json:"status"`
	LastRun       *time.Time        `json:"last_run,omitempty"`
	NextRun       *time.Time        `json:"next_run,omitempty"`
	RunCount      int64             `json:"run_count"`
	FailCount     int64             `json:"fail_count"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// SourceConfig 源配置
type SourceConfig struct {
	Type     string   `json:"type"`     // file, database, vm, container, cloud
	Path     string   `json:"path"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Database string   `json:"database,omitempty"`
	Tables   []string `json:"tables,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	Include  []string `json:"include,omitempty"`
}

// TargetConfig 目标配置
type TargetConfig struct {
	Type       BackupTarget `json:"type"`
	Name       string       `json:"name"`
	Endpoint   string       `json:"endpoint,omitempty"`
	Bucket     string       `json:"bucket,omitempty"`
	Path       string       `json:"path"`
	AccessKey  string       `json:"access_key,omitempty"`
	SecretKey  string       `json:"secret_key,omitempty"`
	Region     string       `json:"region,omitempty"`
	IsPrimary  bool         `json:"is_primary"`
	Weight     int          `json:"weight"` // 负载均衡权重
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Type       string    `json:"type"` // cron, interval, once, event
	Cron       string    `json:"cron,omitempty"`
	Interval   string    `json:"interval,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	TimeZone   string    `json:"timezone,omitempty"`
	MaxRunTime string    `json:"max_run_time,omitempty"`
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	RetryOn       []string      `json:"retry_on"` // 重试条件
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepLast    int       `json:"keep_last"`    // 保留最近N个备份
	KeepDaily   int       `json:"keep_daily"`   // 保留最近N天的每日备份
	KeepWeekly  int       `json:"keep_weekly"`  // 保留最近N周的每周备份
	KeepMonthly int       `json:"keep_monthly"` // 保留最近N月的每月备份
	KeepYearly  int       `json:"keep_yearly"`  // 保留最近N年的每年备份
	ExpireAt    time.Time `json:"expire_at,omitempty"`
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled    bool   `json:"enabled"`
	Algorithm  string `json:"algorithm"` // aes-256-gcm, chacha20-poly1305
	KeyID      string `json:"key_id,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled  bool    `json:"enabled"`
	Algorithm string `json:"algorithm"` // gzip, lz4, zstd, snappy
	Level    int     `json:"level"`     // 压缩级别 1-9
}

// BackupChain 备份链（全量+增量链）
type BackupChain struct {
	ID          string        `json:"id"`
	JobID       string        `json:"job_id"`
	FullBackup  *BackupRecord `json:"full_backup"`
	IncrBackups []*BackupRecord `json:"incr_backups"`
	TotalSize   int64         `json:"total_size"`
	ChainLength int           `json:"chain_length"`
	CreatedAt   time.Time     `json:"created_at"`
	IsValid     bool          `json:"is_valid"`
}

// BackupRecord 备份记录
type BackupRecord struct {
	ID           string       `json:"id"`
	JobID        string       `json:"job_id"`
	ChainID      string       `json:"chain_id"`
	Type         BackupType   `json:"type"`
	Status       BackupStatus `json:"status"`
	SourceSize   int64        `json:"source_size"`
	BackupSize   int64        `json:"backup_size"`
	DedupSize    int64        `json:"dedup_size"`
	CompSize     int64        `json:"comp_size"`
	Duration     time.Duration `json:"duration"`
	Target       string       `json:"target"`
	Checksum     string       `json:"checksum"`
	Verified     bool         `json:"verified"`
	VerifiedAt   *time.Time   `json:"verified_at,omitempty"`
	Error        string       `json:"error,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
}

// BackupMetrics 备份指标
type BackupMetrics struct {
	TotalJobs       int           `json:"total_jobs"`
	ActiveJobs      int           `json:"active_jobs"`
	TotalBackups    int64         `json:"total_backups"`
	TotalSize       int64         `json:"total_size"`
	SuccessRate     float64       `json:"success_rate"`
	AvgDuration     time.Duration `json:"avg_duration"`
	LastBackupTime  *time.Time    `json:"last_backup_time"`
	NextBackupTime  *time.Time    `json:"next_backup_time"`
	DailyChangeRate float64       `json:"daily_change_rate"`
}

// Orchestrator 智能备份编排器
type Orchestrator struct {
	mu          sync.RWMutex
	config      *Config
	jobs        map[string]*BackupJob
	chains      map[string]*BackupChain
	records     []*BackupRecord
	metrics     *BackupMetrics
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// Config 编排器配置
type Config struct {
	MaxConcurrent    int           `json:"max_concurrent"`     // 最大并发备份数
	DefaultRetry     *RetryPolicy  `json:"default_retry"`      // 默认重试策略
	VerifyAfterBackup bool         `json:"verify_after_backup"` // 备份后自动验证
	AutoOptimize     bool          `json:"auto_optimize"`      // 自动优化备份链
	AlertOnFailure   bool          `json:"alert_on_failure"`   // 失败时告警
	RetentionCheck   time.Duration `json:"retention_check"`    // 保留策略检查间隔
}

// NewOrchestrator 创建新的备份编排器
func NewOrchestrator(config *Config) *Orchestrator {
	if config == nil {
		config = &Config{
			MaxConcurrent:     3,
			VerifyAfterBackup: true,
			AutoOptimize:      true,
			AlertOnFailure:    true,
			RetentionCheck:    time.Hour,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Orchestrator{
		config:  config,
		jobs:    make(map[string]*BackupJob),
		chains:  make(map[string]*BackupChain),
		records: make([]*BackupRecord, 0),
		metrics: &BackupMetrics{},
		ctx:     ctx,
		cancel:  cancel,
	}
}

// RegisterJob 注册备份任务
func (o *Orchestrator) RegisterJob(job *BackupJob) error {
	if job == nil {
		return errors.New("backup job cannot be nil")
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	if job.Status == "" {
		job.Status = StatusPending
	}
	o.jobs[job.ID] = job
	return nil
}

// GetJob 获取备份任务
func (o *Orchestrator) GetJob(jobID string) (*BackupJob, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	job, exists := o.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	return job, nil
}

// ListJobs 列出所有备份任务
func (o *Orchestrator) ListJobs() []*BackupJob {
	o.mu.RLock()
	defer o.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(o.jobs))
	for _, job := range o.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// Start 启动编排器
func (o *Orchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running {
		return errors.New("orchestrator is already running")
	}
	o.running = true
	return nil
}

// Stop 停止编排器
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.running {
		return errors.New("orchestrator is not running")
	}
	o.running = false
	o.cancel()
	return nil
}

// GetMetrics 获取备份指标
func (o *Orchestrator) GetMetrics() *BackupMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.metrics
}

// ValidateBackupChain 验证备份链完整性
func (o *Orchestrator) ValidateBackupChain(chainID string) (bool, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	chain, exists := o.chains[chainID]
	if !exists {
		return false, fmt.Errorf("chain %s not found", chainID)
	}

	if chain.FullBackup == nil {
		return false, errors.New("chain has no full backup")
	}

	// 检查全量备份状态
	if chain.FullBackup.Status != StatusCompleted {
		return false, errors.New("full backup is not completed")
	}

	// 检查增量备份链
	for i, incr := range chain.IncrBackups {
		if incr.Status != StatusCompleted {
			return false, fmt.Errorf("incremental backup %d is not completed", i)
		}
	}

	chain.IsValid = true
	return true, nil
}
