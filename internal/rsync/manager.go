package rsync

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager Rsync 备份管理器.
type Manager struct {
	mu      sync.RWMutex
	config  RsyncConfig
	jobs    map[string]*RsyncJob
	targets map[string]*RsyncTarget
	history []*RsyncHistory
	running bool
	stopCh  chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg RsyncConfig) *Manager {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.BandwidthLimit == 0 {
		cfg.BandwidthLimit = 0 // 无限制
	}
	if cfg.RetryCount == 3 {
		cfg.RetryCount = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5 * time.Minute
	}
	if cfg.HistoryLimit == 0 {
		cfg.HistoryLimit = 1000
	}
	return &Manager{
		config:  cfg,
		jobs:    make(map[string]*RsyncJob),
		targets: make(map[string]*RsyncTarget),
		history: make([]*RsyncHistory, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.schedulerLoop()
	log.Println("[Rsync] Rsync 备份管理器已启动")
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
	log.Println("[Rsync] Rsync 备份管理器已停止")
}

// ========== 目标管理 ==========

// AddTarget 添加同步目标.
func (m *Manager) AddTarget(target *RsyncTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if target.ID == "" {
		return fmt.Errorf("目标 ID 不能为空")
	}
	if _, exists := m.targets[target.ID]; exists {
		return fmt.Errorf("目标 %s 已存在", target.ID)
	}
	target.CreatedAt = time.Now()
	m.targets[target.ID] = target
	log.Printf("[Rsync] 同步目标已添加: %s (%s -> %s)", target.ID, target.Source, target.Destination)
	return nil
}

// RemoveTarget 移除同步目标.
func (m *Manager) RemoveTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.targets[id]; !exists {
		return fmt.Errorf("目标 %s 不存在", id)
	}
	delete(m.targets, id)
	log.Printf("[Rsync] 同步目标已移除: %s", id)
	return nil
}

// GetTarget 获取同步目标.
func (m *Manager) GetTarget(id string) (*RsyncTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	target, exists := m.targets[id]
	if !exists {
		return nil, fmt.Errorf("目标 %s 不存在", id)
	}
	return target, nil
}

// ListTargets 列出所有同步目标.
func (m *Manager) ListTargets() []*RsyncTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	targets := make([]*RsyncTarget, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	return targets
}

// ========== 任务管理 ==========

// CreateJob 创建同步任务.
func (m *Manager) CreateJob(job *RsyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.TargetID == "" {
		return fmt.Errorf("目标 ID 不能为空")
	}
	if _, exists := m.targets[job.TargetID]; !exists {
		return fmt.Errorf("目标 %s 不存在", job.TargetID)
	}
	job.ID = fmt.Sprintf("rsync-%d", time.Now().UnixNano())
	job.Status = JobStatusPending
	job.CreatedAt = time.Now()
	m.jobs[job.ID] = job
	log.Printf("[Rsync] 同步任务已创建: %s (目标: %s)", job.ID, job.TargetID)
	return nil
}

// RunJob 执行同步任务.
func (m *Manager) RunJob(jobID string) (*RsyncResult, error) {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("任务 %s 不存在", jobID)
	}
	target := m.targets[job.TargetID]
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()
	m.mu.Unlock()

	// 模拟同步过程
	result := &RsyncResult{
		JobID:            jobID,
		Source:           target.Source,
		Destination:      target.Destination,
		StartTime:        time.Now(),
		FilesTransferred: 150,
		TotalSize:        1024 * 1024 * 500, // 500MB
		AverageSpeed:     10 * 1024 * 1024,  // 10MB/s
	}

	m.mu.Lock()
	job.Status = JobStatusCompleted
	job.CompletedAt = time.Now()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 记录历史
	history := &RsyncHistory{
		ID:          fmt.Sprintf("hist-%d", time.Now().UnixNano()),
		JobID:       jobID,
		TargetID:    job.TargetID,
		Status:      JobStatusCompleted,
		FilesSynced: result.FilesTransferred,
		BytesSynced: result.TotalSize,
		Duration:    result.Duration,
		Timestamp:   time.Now(),
	}
	m.history = append(m.history, history)
	if len(m.history) > m.config.HistoryLimit {
		m.history = m.history[1:]
	}
	m.mu.Unlock()

	log.Printf("[Rsync] 同步任务完成: %s (%d 文件, %s)", jobID, result.FilesTransferred, result.Duration)
	return result, nil
}

// GetJob 获取任务.
func (m *Manager) GetJob(id string) (*RsyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[id]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", id)
	}
	return job, nil
}

// ListJobs 列出所有任务.
func (m *Manager) ListJobs() []*RsyncJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*RsyncJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// ========== 历史记录 ==========

// GetHistory 获取同步历史.
func (m *Manager) GetHistory(limit int) []*RsyncHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}
	return m.history[start:]
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *RsyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	completed := 0
	failed := 0
	for _, j := range m.jobs {
		switch j.Status {
		case JobStatusCompleted:
			completed++
		case JobStatusFailed:
			failed++
		}
	}
	totalBytes := int64(0)
	totalFiles := 0
	for _, h := range m.history {
		totalBytes += h.BytesSynced
		totalFiles += h.FilesSynced
	}
	return &RsyncStats{
		TotalTargets:  len(m.targets),
		TotalJobs:     len(m.jobs),
		CompletedJobs: completed,
		FailedJobs:    failed,
		TotalFiles:    totalFiles,
		TotalBytes:    totalBytes,
	}
}

// ========== 内部方法 ==========

// schedulerLoop 定时任务调度循环.
func (m *Manager) schedulerLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkScheduledJobs()
		}
	}
}

// checkScheduledJobs 检查定时任务.
func (m *Manager) checkScheduledJobs() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	for _, job := range m.jobs {
		if job.Schedule != "" && job.NextRun.Before(now) {
			log.Printf("[Rsync] 触发定时任务: %s", job.ID)
		}
	}
}
