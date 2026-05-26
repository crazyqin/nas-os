// Package backupconsole 集中备份管理控制台
// 灵感来源: 群晖 Active Backup for Business
package backupconsole

import (
	"fmt"
	"sync"
	"time"
)

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
	BackupTypeDifferential BackupType = "differential"
)

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusPending  BackupStatus = "pending"
	BackupStatusRunning  BackupStatus = "running"
	BackupStatusDone     BackupStatus = "done"
	BackupStatusFailed   BackupStatus = "failed"
	BackupStatusCanceled BackupStatus = "canceled"
)

// Platform 平台类型
type Platform string

const (
	PlatformWindows  Platform = "windows"
	PlatformLinux    Platform = "linux"
	PlatformMacOS    Platform = "macos"
	PlatformVMware   Platform = "vmware"
	PlatformHyperV   Platform = "hyperv"
	PlatformK8s      Platform = "kubernetes"
)

// BackupSource 备份源
type BackupSource struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Platform  Platform `json:"platform"`
	IP        string   `json:"ip"`
	Hostname  string   `json:"hostname"`
	Agent     string   `json:"agent_version"`
	LastSeen  time.Time `json:"last_seen"`
	Protected bool     `json:"protected"`
}

// BackupJob 备份任务
type BackupJob struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	SourceID    string       `json:"source_id"`
	SourceType  Platform     `json:"source_type"`
	TargetPool  string       `json:"target_pool"`
	BackupType  BackupType   `json:"backup_type"`
	Schedule    string       `json:"schedule"` // cron expression
	Retention   int          `json:"retention_days"`
	Enabled     bool         `json:"enabled"`
	LastRun     *time.Time   `json:"last_run,omitempty"`
	NextRun     *time.Time   `json:"next_run,omitempty"`
}

// BackupRecord 备份记录
type BackupRecord struct {
	ID           string        `json:"id"`
	JobID        string        `json:"job_id"`
	JobName      string        `json:"job_name"`
	SourceID     string        `json:"source_id"`
	BackupType   BackupType    `json:"backup_type"`
	Status       BackupStatus  `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Duration     time.Duration `json:"duration"`
	BytesTotal   int64         `json:"bytes_total"`
	BytesWritten int64         `json:"bytes_written"`
	DedupRatio   float64       `json:"dedup_ratio"`
	CompressRatio float64      `json:"compress_ratio"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// RestorePoint 恢复点
type RestorePoint struct {
	ID          string    `json:"id"`
	RecordID    string    `json:"record_id"`
	JobID       string    `json:"job_id"`
	SourceName  string    `json:"source_name"`
	Timestamp   time.Time `json:"timestamp"`
	SizeBytes   int64     `json:"size_bytes"`
	Type        BackupType `json:"type"`
	RetentionExpire *time.Time `json:"retention_expire,omitempty"`
}

// BackupConsole 集中备份管理器
type BackupConsole struct {
	mu          sync.RWMutex
	sources     map[string]*BackupSource
	jobs        map[string]*BackupJob
	records     []*BackupRecord
	restores    []*RestorePoint
}

// NewBackupConsole 创建备份管理器
func NewBackupConsole() *BackupConsole {
	return &BackupConsole{
		sources:  make(map[string]*BackupSource),
		jobs:     make(map[string]*BackupJob),
		records:  make([]*BackupRecord, 0),
		restores: make([]*RestorePoint, 0),
	}
}

// RegisterSource 注册备份源
func (bc *BackupConsole) RegisterSource(src *BackupSource) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	src.LastSeen = time.Now()
	bc.sources[src.ID] = src
}

// GetSource 获取备份源
func (bc *BackupConsole) GetSource(id string) (*BackupSource, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	s, ok := bc.sources[id]
	return s, ok
}

// ListSources 列出备份源
func (bc *BackupConsole) ListSources(platform Platform) []*BackupSource {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	srcs := make([]*BackupSource, 0)
	for _, s := range bc.sources {
		if platform == "" || s.Platform == platform {
			srcs = append(srcs, s)
		}
	}
	return srcs
}

// CreateJob 创建备份任务
func (bc *BackupConsole) CreateJob(job *BackupJob) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if _, exists := bc.sources[job.SourceID]; !exists {
		return fmt.Errorf("source %s not found", job.SourceID)
	}

	bc.jobs[job.ID] = job
	return nil
}

// GetJob 获取备份任务
func (bc *BackupConsole) GetJob(id string) (*BackupJob, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	j, ok := bc.jobs[id]
	return j, ok
}

// ListJobs 列出备份任务
func (bc *BackupConsole) ListJobs(enabledOnly bool) []*BackupJob {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	jobs := make([]*BackupJob, 0)
	for _, j := range bc.jobs {
		if !enabledOnly || j.Enabled {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// RunBackup 执行备份
func (bc *BackupConsole) RunBackup(jobID string) (*BackupRecord, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	job, exists := bc.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	source, exists := bc.sources[job.SourceID]
	if !exists {
		return nil, fmt.Errorf("source %s not found", job.SourceID)
	}

	record := &BackupRecord{
		ID:         fmt.Sprintf("rec-%s-%d", jobID, time.Now().Unix()),
		JobID:      jobID,
		JobName:    job.Name,
		SourceID:   job.SourceID,
		BackupType: job.BackupType,
		Status:     BackupStatusRunning,
		StartedAt:  time.Now(),
	}

	bc.records = append(bc.records, record)

	// 创建恢复点
	rp := &RestorePoint{
		ID:         fmt.Sprintf("rp-%s-%d", jobID, time.Now().Unix()),
		RecordID:   record.ID,
		JobID:      jobID,
		SourceName: source.Name,
		Timestamp:  time.Now(),
		Type:       job.BackupType,
	}
	if job.Retention > 0 {
		expire := time.Now().AddDate(0, 0, job.Retention)
		rp.RetentionExpire = &expire
	}
	bc.restores = append(bc.restores, rp)

	now := time.Now()
	job.LastRun = &now

	return record, nil
}

// CompleteBackup 完成备份
func (bc *BackupConsole) CompleteBackup(recordID string, bytesTotal, bytesWritten int64, dedupRatio, compressRatio float64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for _, rec := range bc.records {
		if rec.ID == recordID {
			rec.Status = BackupStatusDone
			now := time.Now()
			rec.CompletedAt = &now
			rec.Duration = now.Sub(rec.StartedAt)
			rec.BytesTotal = bytesTotal
			rec.BytesWritten = bytesWritten
			rec.DedupRatio = dedupRatio
			rec.CompressRatio = compressRatio
			return nil
		}
	}
	return fmt.Errorf("record %s not found", recordID)
}

// FailBackup 标记备份失败
func (bc *BackupConsole) FailBackup(recordID, reason string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for _, rec := range bc.records {
		if rec.ID == recordID {
			rec.Status = BackupStatusFailed
			rec.ErrorMessage = reason
			now := time.Now()
			rec.CompletedAt = &now
			rec.Duration = now.Sub(rec.StartedAt)
			return nil
		}
	}
	return fmt.Errorf("record %s not found", recordID)
}

// GetRestorePoints 获取恢复点
func (bc *BackupConsole) GetRestorePoints(jobID string) []*RestorePoint {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	points := make([]*RestorePoint, 0)
	for _, rp := range bc.restores {
		if jobID == "" || rp.JobID == jobID {
			points = append(points, rp)
		}
	}
	return points
}

// GetRecords 获取备份记录
func (bc *BackupConsole) GetRecords(jobID string, limit int) []*BackupRecord {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	records := make([]*BackupRecord, 0)
	for i := len(bc.records) - 1; i >= 0; i-- {
		rec := bc.records[i]
		if jobID == "" || rec.JobID == jobID {
			records = append(records, rec)
			if limit > 0 && len(records) >= limit {
				break
			}
		}
	}
	return records
}

// Dashboard 备份仪表盘
type Dashboard struct {
	TotalSources   int `json:"total_sources"`
	ProtectedHosts int `json:"protected_hosts"`
	TotalJobs      int `json:"total_jobs"`
	ActiveJobs     int `json:"active_jobs"`
	TotalRecords   int `json:"total_records"`
	FailedBackups  int `json:"failed_backups"`
	RestorePoints  int `json:"restore_points"`
}

// GetDashboard 获取仪表盘
func (bc *BackupConsole) GetDashboard() *Dashboard {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	dash := &Dashboard{
		TotalSources:  len(bc.sources),
		TotalJobs:     len(bc.jobs),
		TotalRecords:  len(bc.records),
		RestorePoints: len(bc.restores),
	}

	for _, src := range bc.sources {
		if src.Protected {
			dash.ProtectedHosts++
		}
	}
	for _, job := range bc.jobs {
		if job.Enabled {
			dash.ActiveJobs++
		}
	}
	for _, rec := range bc.records {
		if rec.Status == BackupStatusFailed {
			dash.FailedBackups++
		}
	}

	return dash
}
