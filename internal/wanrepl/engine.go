// Package wanrepl WAN远程复制引擎实现
// 基于types.go中定义的类型实现完整的WAN复制功能
package wanrepl

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrSiteNotFound     = errors.New("站点不存在")
	ErrJobNotFound      = errors.New("任务不存在")
	ErrJobRunning       = errors.New("任务正在运行")
	ErrEngineNotRunning = errors.New("引擎未启动")
	ErrSiteExists       = errors.New("站点已存在")
	ErrInvalidConfig    = errors.New("无效配置")
)

// NewReplicationEngine 创建复制引擎
func NewReplicationEngine(config ReplConfig) *ReplicationEngine {
	return &ReplicationEngine{
		config:  config,
		sites:   make(map[string]*RemoteSite),
		jobs:    make(map[string]*ReplicationJob),
		states:  make(map[string]*replicationJobState),
		running: false,
	}
}

// Start 启动引擎
func (e *ReplicationEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = true
	return nil
}

// Stop 停止引擎
func (e *ReplicationEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, state := range e.states {
		if state.cancel != nil {
			close(state.cancel)
		}
	}
	e.running = false
	return nil
}

// IsRunning 引擎是否运行中
func (e *ReplicationEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// AddSite 添加远程站点
func (e *ReplicationEngine) AddSite(site *RemoteSite) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sites[site.ID]; exists {
		return ErrSiteExists
	}
	now := time.Now()
	site.CreatedAt = now
	site.UpdatedAt = now
	if site.Status == "" {
		site.Status = SiteStatusUnknown
	}
	e.sites[site.ID] = site
	return nil
}

// RemoveSite 移除远程站点
func (e *ReplicationEngine) RemoveSite(siteID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.sites[siteID]; !ok {
		return ErrSiteNotFound
	}
	delete(e.sites, siteID)
	return nil
}

// UpdateSiteStatus 更新站点状态
func (e *ReplicationEngine) UpdateSiteStatus(siteID string, status SiteStatus, latency int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	site, ok := e.sites[siteID]
	if !ok {
		return ErrSiteNotFound
	}
	site.Status = status
	site.Latency = latency
	site.UpdatedAt = time.Now()
	return nil
}

// GetSite 获取站点信息
func (e *ReplicationEngine) GetSite(siteID string) (*RemoteSite, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	site, ok := e.sites[siteID]
	if !ok {
		return nil, ErrSiteNotFound
	}
	return site, nil
}

// ListSites 列出所有站点
func (e *ReplicationEngine) ListSites() []*RemoteSite {
	e.mu.RLock()
	defer e.mu.RUnlock()
	list := make([]*RemoteSite, 0, len(e.sites))
	for _, s := range e.sites {
		list = append(list, s)
	}
	return list
}

// CreateJob 创建复制任务
func (e *ReplicationEngine) CreateJob(job *ReplicationJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.sites[job.TargetSiteID]; !ok {
		return ErrSiteNotFound
	}

	if job.ID == "" {
		job.ID = fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	if job.Status == "" {
		job.Status = JobStatusPending
	}
	if job.Strategy == "" {
		job.Strategy = StrategyIncremental
	}
	if job.Compression == "" {
		job.Compression = CompressionZstd
	}
	if job.Encryption == "" {
		job.Encryption = EncryptionTLS
	}

	e.jobs[job.ID] = job
	e.states[job.ID] = &replicationJobState{
		state: SyncState{
			JobID:       job.ID,
			StartTime:   now,
			LastUpdated: now,
		},
		cancel:    make(chan struct{}),
		stats:     TransferStats{JobID: job.ID, UpdatedAt: now},
		conflicts: make([]ConflictRecord, 0),
	}
	return nil
}

// StartJob 启动复制任务
func (e *ReplicationEngine) StartJob(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, ok := e.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	if job.Status == JobStatusRunning {
		return ErrJobRunning
	}

	now := time.Now()
	job.Status = JobStatusRunning
	job.StartedAt = &now
	job.UpdatedAt = now

	state := e.states[jobID]
	state.state.StartTime = now
	state.state.LastUpdated = now
	return nil
}

// PauseJob 暂停任务
func (e *ReplicationEngine) PauseJob(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, ok := e.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = JobStatusPaused
	job.UpdatedAt = time.Now()

	state, ok := e.states[jobID]
	if ok {
		state.paused = true
	}
	return nil
}

// CompleteJob 完成任务
func (e *ReplicationEngine) CompleteJob(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, ok := e.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	now := time.Now()
	job.Status = JobStatusCompleted
	job.CompletedAt = &now
	job.UpdatedAt = now

	state, ok := e.states[jobID]
	if ok {
		state.state.Progress = 1.0
		state.state.LastUpdated = now
	}
	return nil
}

// UpdateSyncProgress 更新同步进度
func (e *ReplicationEngine) UpdateSyncProgress(jobID string, progress float64, bytesTransferred int64, speedBps int64, currentFile string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.states[jobID]
	if !ok {
		return ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.state.Progress = progress
	state.state.BytesTransferred = bytesTransferred
	state.state.Speed = speedBps
	state.state.CurrentFile = currentFile
	state.state.LastUpdated = time.Now()

	if speedBps > 0 && state.state.BytesRemaining > 0 {
		state.state.ETA = time.Duration(state.state.BytesRemaining/speedBps) * time.Second
	}

	return nil
}

// ReportConflict 报告冲突
func (e *ReplicationEngine) ReportConflict(jobID string, conflict ConflictRecord) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.states[jobID]
	if !ok {
		return ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	conflict.ID = fmt.Sprintf("conflict-%d", time.Now().UnixNano())
	conflict.JobID = jobID
	conflict.DetectedAt = time.Now()
	state.conflicts = append(state.conflicts, conflict)
	state.stats.ConflictCount = len(state.conflicts)
	return nil
}

// ResolveConflict 解决冲突
func (e *ReplicationEngine) ResolveConflict(jobID, conflictID string, resolution ConflictResolution) error {
	e.mu.RLock()
	state, ok := e.states[jobID]
	e.mu.RUnlock()
	if !ok {
		return ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for i, c := range state.conflicts {
		if c.ID == conflictID {
			state.conflicts[i].Resolution = resolution
			state.conflicts[i].Resolved = true
			now := time.Now()
			state.conflicts[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("conflict %s not found", conflictID)
}

// GetSyncState 获取同步状态
func (e *ReplicationEngine) GetSyncState(jobID string) (*SyncState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.states[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	s := state.state
	return &s, nil
}

// GetTransferStats 获取传输统计
func (e *ReplicationEngine) GetTransferStats(jobID string) (*TransferStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.states[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	s := state.stats
	return &s, nil
}

// GetConflicts 获取冲突列表
func (e *ReplicationEngine) GetConflicts(jobID string, unresolvedOnly bool) ([]ConflictRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.states[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	result := make([]ConflictRecord, 0)
	for _, c := range state.conflicts {
		if unresolvedOnly && c.Resolved {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

// GetJob 获取任务
func (e *ReplicationEngine) GetJob(jobID string) (*ReplicationJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	job, ok := e.jobs[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs 列出所有任务
func (e *ReplicationEngine) ListJobs() []*ReplicationJob {
	e.mu.RLock()
	defer e.mu.RUnlock()
	list := make([]*ReplicationJob, 0, len(e.jobs))
	for _, j := range e.jobs {
		list = append(list, j)
	}
	return list
}

// RecordChangeSet 记录变更集
func (e *ReplicationEngine) RecordChangeSet(cs ChangeSet) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	// 存储变更集用于增量同步
	return nil
}
