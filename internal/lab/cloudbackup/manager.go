package cloudbackup

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Manager 云备份管理器.
type Manager struct {
	mu        sync.RWMutex
	config    BackupConfig
	accounts  map[string]*CloudAccount
	jobs      map[string]*BackupJob
	schedules map[string]*BackupSchedule
	restores  map[string]*RestoreRequest
	running   bool
	stopCh    chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg BackupConfig) *Manager {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 10 * 1024 * 1024 // 10MB
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 30 * time.Second
	}
	return &Manager{
		config:    cfg,
		accounts:  make(map[string]*CloudAccount),
		jobs:      make(map[string]*BackupJob),
		schedules: make(map[string]*BackupSchedule),
		restores:  make(map[string]*RestoreRequest),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动备份管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.scheduleLoop()
	log.Println("[CloudBackup] 云备份管理器已启动")
	return nil
}

// Stop 停止.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[CloudBackup] 云备份管理器已停止")
}

// ========== 账号管理 ==========

// AddAccount 添加云账号.
func (m *Manager) AddAccount(acc *CloudAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acc.Status = "active"
	acc.CreatedAt = time.Now()
	m.accounts[acc.ID] = acc
	return nil
}

// RemoveAccount 删除云账号.
func (m *Manager) RemoveAccount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.accounts[id]; !ok {
		return ErrProviderNotFound
	}
	delete(m.accounts, id)
	return nil
}

// GetAccount 获取账号.
func (m *Manager) GetAccount(id string) (*CloudAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	acc, ok := m.accounts[id]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return acc, nil
}

// ListAccounts 列出账号.
func (m *Manager) ListAccounts(provider CloudProvider) []*CloudAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*CloudAccount
	for _, a := range m.accounts {
		if provider != "" && a.Provider != provider {
			continue
		}
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ========== 备份任务 ==========

// CreateJob 创建备份任务.
func (m *Manager) CreateJob(job *BackupJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.Status == "" {
		job.Status = JobPending
	}
	job.CreatedAt = time.Now()
	m.jobs[job.ID] = job
	return nil
}

// RunJob 运行备份任务.
func (m *Manager) RunJob(id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}
	if job.Status == JobRunning {
		m.mu.Unlock()
		return ErrJobAlreadyRunning
	}
	job.Status = JobRunning
	job.StartedAt = time.Now()
	job.Progress = 0
	m.mu.Unlock()

	go m.executeJob(id)
	return nil
}

// CancelJob 取消任务.
func (m *Manager) CancelJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = JobCancelled
	job.FinishedAt = time.Now()
	return nil
}

// GetJob 获取任务.
func (m *Manager) GetJob(id string) (*BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs 列出任务.
func (m *Manager) ListJobs(accountID string, status BackupJobStatus) []*BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*BackupJob
	for _, j := range m.jobs {
		if accountID != "" && j.AccountID != accountID {
			continue
		}
		if status != "" && j.Status != status {
			continue
		}
		result = append(result, j)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ========== 备份计划 ==========

// CreateSchedule 创建备份计划.
func (m *Manager) CreateSchedule(sched *BackupSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sched.Enabled = true
	m.schedules[sched.ID] = sched
	return nil
}

// DeleteSchedule 删除计划.
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.schedules, id)
	return nil
}

// ListSchedules 列出计划.
func (m *Manager) ListSchedules() []*BackupSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*BackupSchedule
	for _, s := range m.schedules {
		result = append(result, s)
	}
	return result
}

// ========== 恢复 ==========

// CreateRestore 创建恢复请求.
func (m *Manager) CreateRestore(req *RestoreRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req.Status = "pending"
	req.CreatedAt = time.Now()
	m.restores[req.ID] = req
	return nil
}

// GetRestore 获取恢复请求.
func (m *Manager) GetRestore(id string) (*RestoreRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	req, ok := m.restores[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return req, nil
}

// ========== 统计 ==========

// GetStats 获取统计.
func (m *Manager) GetStats() BackupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := BackupStats{
		TotalAccounts: len(m.accounts),
		ProviderStats: make(map[string]int),
		ServiceStats:  make(map[string]int),
	}
	for _, j := range m.jobs {
		stats.TotalJobs++
		switch j.Status {
		case JobCompleted:
			stats.SuccessJobs++
		case JobFailed:
			stats.FailedJobs++
		}
		stats.TotalItems += j.BackedItems
		stats.TotalBytes += j.BackedBytes
		stats.ProviderStats[string(j.Provider)]++
	}
	return stats
}

// ========== 内部 ==========

func (m *Manager) executeJob(jobID string) {
	m.mu.Lock()
	job := m.jobs[jobID]
	m.mu.Unlock()

	// 模拟备份执行
	for i := 0; i <= 100; i += 10 {
		m.mu.Lock()
		if job.Status == JobCancelled {
			m.mu.Unlock()
			return
		}
		job.Progress = float64(i)
		job.BackedItems = int64(float64(job.TotalItems) * float64(i) / 100)
		job.BackedBytes = int64(float64(job.TotalBytes) * float64(i) / 100)
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	job.Status = JobCompleted
	job.Progress = 100
	job.FinishedAt = time.Now()
	m.mu.Unlock()
}

func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkSchedules()
		}
	}
}

func (m *Manager) checkSchedules() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	for _, sched := range m.schedules {
		if !sched.Enabled {
			continue
		}
		if !sched.NextRun.IsZero() && now.After(sched.NextRun) {
			job := &BackupJob{
				ID:        fmt.Sprintf("auto-%s-%d", sched.ID, now.Unix()),
				AccountID: sched.AccountID,
				Services:  sched.Services,
			}
			m.mu.RUnlock()
			_ = m.CreateJob(job)
			_ = m.RunJob(job.ID)
			m.mu.RLock()
			sched.LastRun = now
			sched.NextRun = now.Add(24 * time.Hour)
		}
	}
}
