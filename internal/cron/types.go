package cron

import (
	"sync"
	"time"
)

// JobStatus 任务状态.
type JobStatus string

const (
	StatusEnabled  JobStatus = "enabled"
	StatusDisabled JobStatus = "disabled"
	StatusRunning  JobStatus = "running"
	StatusError    JobStatus = "error"
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	ScheduleCron    ScheduleType = "cron"    // cron 表达式
	ScheduleOnce    ScheduleType = "once"    // 一次性
	ScheduleEvery   ScheduleType = "every"   // 间隔执行
	ScheduleBoot    ScheduleType = "boot"    // 开机执行
	ScheduleDaily   ScheduleType = "daily"   // 每天
	ScheduleWeekly  ScheduleType = "weekly"  // 每周
	ScheduleMonthly ScheduleType = "monthly" // 每月
)

// CronJob 定时任务.
type CronJob struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      JobStatus `json:"status"`
	Schedule    Schedule  `json:"schedule"`
	// Command 要执行的命令或脚本.
	Command string `json:"command"`
	// WorkingDir 工作目录.
	WorkingDir string `json:"working_dir,omitempty"`
	// EnvVars 环境变量.
	EnvVars map[string]string `json:"env_vars,omitempty"`
	// RunAsUser 以指定用户运行.
	RunAsUser string `json:"run_as_user,omitempty"`
	// TimeoutS 超时秒数.
	TimeoutS int `json:"timeout_s"`
	// MaxRetries 最大重试次数.
	MaxRetries int `json:"max_retries"`
	// NotifyOnFailure 失败时通知.
	NotifyOnFailure bool       `json:"notify_on_failure"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	// RunCount 累计执行次数.
	RunCount int64 `json:"run_count"`
	// ErrorCount 累计错误次数.
	ErrorCount int64  `json:"error_count"`
	LastError  string `json:"last_error,omitempty"`
}

// Schedule 调度配置.
type Schedule struct {
	Type ScheduleType `json:"type"`
	// CronExpr cron 表达式（Type=Cron 时使用）.
	CronExpr string `json:"cron_expr,omitempty"`
	// IntervalS 间隔秒数（Type=Every 时使用）.
	IntervalS int64 `json:"interval_s,omitempty"`
	// Time 执行时间 "HH:MM"（Type=Daily/Weekly/Monthly 时使用）.
	Time string `json:"time,omitempty"`
	// DayOfWeek 周几（Type=Weekly 时使用，0=周日）.
	DayOfWeek int `json:"day_of_week,omitempty"`
	// DayOfMonth 几号（Type=Monthly 时使用）.
	DayOfMonth int `json:"day_of_month,omitempty"`
	// ExecAt 执行时间（Type=Once 时使用）.
	ExecAt *time.Time `json:"exec_at,omitempty"`
}

// JobRun 任务运行记录.
type JobRun struct {
	ID        string        `json:"id"`
	JobID     string        `json:"job_id"`
	JobName   string        `json:"job_name"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration"`
	ExitCode  int           `json:"exit_code"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Success   bool          `json:"success"`
}

// CronConfig 定时任务配置.
type CronConfig struct {
	MaxConcurrent    int  `json:"max_concurrent"`
	MaxHistoryPerJob int  `json:"max_history_per_job"`
	MaxHistoryTotal  int  `json:"max_history_total"`
	LogRetentionDays int  `json:"log_retention_days"`
	Enabled          bool `json:"enabled"`
}

// Manager 定时任务管理器.
type Manager struct {
	mu      sync.RWMutex
	config  *CronConfig
	jobs    map[string]*CronJob
	history []JobRun
	running int
}

// NewManager 创建定时任务管理器.
func NewManager() *Manager {
	return &Manager{
		config: &CronConfig{
			MaxConcurrent:    5,
			MaxHistoryPerJob: 100,
			MaxHistoryTotal:  10000,
			LogRetentionDays: 30,
			Enabled:          true,
		},
		jobs:    make(map[string]*CronJob),
		history: make([]JobRun, 0, 1000),
	}
}

// AddJob 添加任务.
func (m *Manager) AddJob(job *CronJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	if job.Status == "" {
		job.Status = StatusEnabled
	}
	m.jobs[job.ID] = job
}

// UpdateJob 更新任务.
func (m *Manager) UpdateJob(id string, update *CronJob) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.jobs[id]
	if !ok {
		return false
	}
	update.ID = id
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()
	update.RunCount = existing.RunCount
	update.ErrorCount = existing.ErrorCount
	update.LastRunAt = existing.LastRunAt
	m.jobs[id] = update
	return true
}

// DeleteJob 删除任务.
func (m *Manager) DeleteJob(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[id]; !ok {
		return false
	}
	delete(m.jobs, id)
	return true
}

// GetJob 获取任务.
func (m *Manager) GetJob(id string) (*CronJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// ListJobs 列出任务.
func (m *Manager) ListJobs(enabledOnly bool) []*CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*CronJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		if enabledOnly && j.Status != StatusEnabled {
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs
}

// EnableJob 启用任务.
func (m *Manager) EnableJob(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return false
	}
	job.Status = StatusEnabled
	job.UpdatedAt = time.Now()
	return true
}

// DisableJob 禁用任务.
func (m *Manager) DisableJob(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return false
	}
	job.Status = StatusDisabled
	job.UpdatedAt = time.Now()
	return true
}

// RunNow 立即执行任务.
func (m *Manager) RunNow(id string) (*JobRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running >= m.config.MaxConcurrent {
		return nil, ErrMaxConcurrent
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}

	now := time.Now()
	run := &JobRun{
		ID:        generateID(),
		JobID:     id,
		JobName:   job.Name,
		StartedAt: now,
		Success:   true, // 简化：实际需要执行命令
	}
	end := now.Add(time.Millisecond)
	run.EndedAt = &end
	run.Duration = time.Millisecond

	job.LastRunAt = &now
	job.RunCount++

	m.history = append(m.history, *run)
	if len(m.history) > m.config.MaxHistoryTotal {
		m.history = m.history[len(m.history)-m.config.MaxHistoryTotal:]
	}

	return run, nil
}

// GetHistory 获取执行历史.
func (m *Manager) GetHistory(jobID string, limit int) []JobRun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	result := make([]JobRun, 0)
	for i := len(m.history) - 1; i >= 0; i-- {
		if jobID != "" && m.history[i].JobID != jobID {
			continue
		}
		result = append(result, m.history[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *CronConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *CronConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetStats 获取统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	enabled := 0
	for _, j := range m.jobs {
		if j.Status == StatusEnabled {
			enabled++
		}
	}
	return map[string]interface{}{
		"total_jobs":   len(m.jobs),
		"enabled_jobs": enabled,
		"total_runs":   len(m.history),
		"running":      m.running,
	}
}

var idCounter int64

func generateID() string {
	idCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+idCounter%26))
}
