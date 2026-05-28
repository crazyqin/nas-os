package wanrepl

import (
	"fmt"
	"time"
)

// NewReplicationEngine 创建复制引擎
func NewReplicationEngine(config *ReplConfig) *ReplicationEngine {
	cfg := DefaultReplConfig()
	if config != nil {
		cfg = *config
	}

	return &ReplicationEngine{
		config:  cfg,
		sites:   make(map[string]*RemoteSite),
		jobs:    make(map[string]*ReplicationJob),
		states:  make(map[string]*replicationJobState),
		running: false,
	}
}

// AddSite 添加远程站点
func (r *ReplicationEngine) AddSite(site *RemoteSite) error {
	if site == nil {
		return fmt.Errorf("wanrepl: site cannot be nil")
	}
	if site.ID == "" {
		return fmt.Errorf("wanrepl: site ID is required")
	}
	if site.Endpoint == "" {
		return fmt.Errorf("wanrepl: site endpoint is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sites[site.ID]; exists {
		return fmt.Errorf("wanrepl: site %s already exists", site.ID)
	}

	now := time.Now()
	site.Status = SiteStatusUnknown
	site.CreatedAt = now
	site.UpdatedAt = now

	r.sites[site.ID] = site
	return nil
}

// RemoveSite 移除远程站点
func (r *ReplicationEngine) RemoveSite(siteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sites[siteID]; !exists {
		return fmt.Errorf("wanrepl: site %s not found", siteID)
	}

	// 检查是否有运行中的任务依赖此站点
	for _, job := range r.jobs {
		if job.TargetSiteID == siteID {
			if state, ok := r.states[job.ID]; ok {
				state.mu.Lock()
				running := state.state.Progress > 0 && state.state.Progress < 1.0
				state.mu.Unlock()
				if running {
					return fmt.Errorf("wanrepl: site %s has active replication jobs", siteID)
				}
			}
		}
	}

	delete(r.sites, siteID)
	return nil
}

// ListSites 列出所有远程站点
func (r *ReplicationEngine) ListSites() []*RemoteSite {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sites := make([]*RemoteSite, 0, len(r.sites))
	for _, site := range r.sites {
		sites = append(sites, site)
	}
	return sites
}

// GetSite 获取指定站点
func (r *ReplicationEngine) GetSite(siteID string) (*RemoteSite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	site, exists := r.sites[siteID]
	if !exists {
		return nil, fmt.Errorf("wanrepl: site %s not found", siteID)
	}
	return site, nil
}

// CreateJob 创建复制任务
func (r *ReplicationEngine) CreateJob(job *ReplicationJob) error {
	if job == nil {
		return fmt.Errorf("wanrepl: job cannot be nil")
	}
	if job.ID == "" {
		return fmt.Errorf("wanrepl: job ID is required")
	}
	if job.Source == "" {
		return fmt.Errorf("wanrepl: job source is required")
	}
	if job.Destination == "" {
		return fmt.Errorf("wanrepl: job destination is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[job.ID]; exists {
		return fmt.Errorf("wanrepl: job %s already exists", job.ID)
	}

	// 验证目标站点存在
	if job.TargetSiteID != "" {
		if _, exists := r.sites[job.TargetSiteID]; !exists {
			return fmt.Errorf("wanrepl: target site %s not found", job.TargetSiteID)
		}
	}

	// 设置默认值
	now := time.Now()
	if job.Strategy == "" {
		job.Strategy = StrategyIncremental
	}
	if job.Compression == "" {
		job.Compression = CompressionType(r.config.DefaultCompress)
	}
	if job.Encryption == "" {
		job.Encryption = EncryptionTLS
	}
	job.Status = JobStatusPending
	job.CreatedAt = now
	job.UpdatedAt = now

	r.jobs[job.ID] = job
	r.states[job.ID] = &replicationJobState{
		state: SyncState{
			JobID:       job.ID,
			StartTime:   now,
			LastUpdated: now,
		},
		cancel: make(chan struct{}),
		stats: TransferStats{
			JobID:     job.ID,
			UpdatedAt: now,
		},
	}

	return nil
}

// DeleteJob 删除复制任务
func (r *ReplicationEngine) DeleteJob(jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}

	if job.Status == JobStatusRunning {
		return fmt.Errorf("wanrepl: cannot delete running job %s", jobID)
	}

	delete(r.jobs, jobID)
	delete(r.states, jobID)
	return nil
}

// StartSync 启动同步任务
func (r *ReplicationEngine) StartSync(jobID string) error {
	r.mu.Lock()
	job, exists := r.jobs[jobID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}

	if job.Status == JobStatusRunning {
		r.mu.Unlock()
		return fmt.Errorf("wanrepl: job %s is already running", jobID)
	}

	state, ok := r.states[jobID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("wanrepl: state for job %s not found", jobID)
	}

	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	job.UpdatedAt = now

	state.mu.Lock()
	state.state.StartTime = now
	state.state.LastUpdated = now
	state.state.Progress = 0
	state.paused = false
	state.mu.Unlock()

	r.mu.Unlock()

	// 启动异步复制
	go r.runReplication(jobID)

	return nil
}

// StopSync 停止同步任务
func (r *ReplicationEngine) StopSync(jobID string) error {
	r.mu.RLock()
	job, exists := r.jobs[jobID]
	if !exists {
		r.mu.RUnlock()
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}
	state, ok := r.states[jobID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("wanrepl: state for job %s not found", jobID)
	}

	if job.Status != JobStatusRunning {
		return fmt.Errorf("wanrepl: job %s is not running", jobID)
	}

	state.mu.Lock()
	select {
	case <-state.cancel:
		// already closed
	default:
		close(state.cancel)
	}
	state.mu.Unlock()

	r.mu.Lock()
	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()
	r.mu.Unlock()

	return nil
}

// PauseSync 暂停同步任务
func (r *ReplicationEngine) PauseSync(jobID string) error {
	r.mu.RLock()
	job, exists := r.jobs[jobID]
	if !exists {
		r.mu.RUnlock()
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}
	state, ok := r.states[jobID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("wanrepl: state for job %s not found", jobID)
	}

	if job.Status != JobStatusRunning {
		return fmt.Errorf("wanrepl: job %s is not running", jobID)
	}

	state.mu.Lock()
	state.paused = true
	state.mu.Unlock()

	r.mu.Lock()
	job.Status = JobStatusPaused
	job.UpdatedAt = time.Now()
	r.mu.Unlock()

	return nil
}

// ResumeSync 恢复同步任务
func (r *ReplicationEngine) ResumeSync(jobID string) error {
	r.mu.RLock()
	job, exists := r.jobs[jobID]
	if !exists {
		r.mu.RUnlock()
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}
	state, ok := r.states[jobID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("wanrepl: state for job %s not found", jobID)
	}

	if job.Status != JobStatusPaused {
		return fmt.Errorf("wanrepl: job %s is not paused", jobID)
	}

	state.mu.Lock()
	state.paused = false
	state.mu.Unlock()

	r.mu.Lock()
	job.Status = JobStatusRunning
	job.UpdatedAt = time.Now()
	r.mu.Unlock()

	return nil
}

// GetSyncState 获取同步状态
func (r *ReplicationEngine) GetSyncState(jobID string) (*SyncState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[jobID]
	if !exists {
		return nil, fmt.Errorf("wanrepl: job %s not found", jobID)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	result := state.state
	return &result, nil
}

// GetTransferStats 获取传输统计
func (r *ReplicationEngine) GetTransferStats(jobID string) *TransferStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[jobID]
	if !exists {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	result := state.stats
	return &result
}

// GetConflicts 获取冲突记录
func (r *ReplicationEngine) GetConflicts(jobID string) []ConflictRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[jobID]
	if !exists {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	result := make([]ConflictRecord, len(state.conflicts))
	copy(result, state.conflicts)
	return result
}

// ResolveConflict 解决冲突
func (r *ReplicationEngine) ResolveConflict(conflict *ConflictRecord, resolution string) error {
	if conflict == nil {
		return fmt.Errorf("wanrepl: conflict record cannot be nil")
	}

	r.mu.RLock()
	state, exists := r.states[conflict.JobID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("wanrepl: job %s not found", conflict.JobID)
	}

	// 验证解决策略
	res := ConflictResolution(resolution)
	switch res {
	case ConflictLocalWins, ConflictRemoteWins, ConflictNewest, ConflictManual, ConflictRename:
		// valid
	default:
		return fmt.Errorf("wanrepl: invalid conflict resolution: %s", resolution)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// 查找并更新冲突记录
	found := false
	for i, c := range state.conflicts {
		if c.ID == conflict.ID && !c.Resolved {
			now := time.Now()
			state.conflicts[i].Resolution = res
			state.conflicts[i].Resolved = true
			state.conflicts[i].ResolvedAt = &now
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("wanrepl: conflict %s not found or already resolved", conflict.ID)
	}

	return nil
}

// ListJobs 列出所有任务
func (r *ReplicationEngine) ListJobs() []*ReplicationJob {
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobs := make([]*ReplicationJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetJob 获取指定任务
func (r *ReplicationEngine) GetJob(jobID string) (*ReplicationJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("wanrepl: job %s not found", jobID)
	}
	return job, nil
}

// IsRunning 引擎是否运行中
func (r *ReplicationEngine) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// SetBandwidthLimit 设置任务带宽限制
func (r *ReplicationEngine) SetBandwidthLimit(jobID string, limitBps int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("wanrepl: job %s not found", jobID)
	}

	job.BandwidthLimit = limitBps
	job.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// 内部实现
// ============================================================

// runReplication 执行复制任务（异步）
func (r *ReplicationEngine) runReplication(jobID string) {
	r.mu.RLock()
	job := r.jobs[jobID]
	state := r.states[jobID]
	r.mu.RUnlock()

	if job == nil || state == nil {
		return
	}

	defer func() {
		r.mu.Lock()
		if job, ok := r.jobs[jobID]; ok {
			now := time.Now()
			job.UpdatedAt = now
			completedAt := now
			if job.Status == JobStatusRunning {
				job.Status = JobStatusCompleted
				job.CompletedAt = &completedAt
			}
		}
		r.mu.Unlock()
	}()

	// 模拟复制过程
	var totalBytes int64 = 1024 * 1024 * 100 // 100MB 示例
	var transferred int64
	chunkSize := int64(r.config.TransferBufSize)
	if chunkSize == 0 {
		chunkSize = 4 * 1024 * 1024
	}

	state.mu.Lock()
	state.state.TotalBytes = totalBytes
	state.state.BytesRemaining = totalBytes
	state.mu.Unlock()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for transferred < totalBytes {
		select {
		case <-state.cancel:
			return
		case <-ticker.C:
			state.mu.Lock()
			if state.paused {
				state.mu.Unlock()
				continue
			}

			// 应用带宽限制
			actualChunk := chunkSize
			if job.BandwidthLimit > 0 {
				limitChunk := job.BandwidthLimit / 2 // 每500ms的量
				if limitChunk < actualChunk {
					actualChunk = limitChunk
				}
			}

			transferred += actualChunk
			if transferred > totalBytes {
				transferred = totalBytes
			}

			now := time.Now()
			elapsed := now.Sub(state.state.StartTime).Seconds()
			state.state.BytesTransferred = transferred
			state.state.BytesRemaining = totalBytes - transferred
			state.state.Progress = float64(transferred) / float64(totalBytes)
			state.state.Speed = actualChunk * 2 // bytes/s (500ms间隔)
			if elapsed > 0 {
				state.state.AvgSpeed = int64(float64(transferred) / elapsed)
			}
			if state.state.Speed > 0 {
				state.state.ETA = time.Duration(float64(totalBytes-transferred)/float64(state.state.Speed)) * time.Second
			}
			state.state.LastUpdated = now

			// 更新统计
			state.stats.TotalBytes = transferred
			state.stats.AvgSpeedBps = state.state.AvgSpeed
			if state.state.Speed > state.stats.PeakSpeedBps {
				state.stats.PeakSpeedBps = state.state.Speed
			}
			state.stats.UpdatedAt = now

			state.mu.Unlock()
		}
	}

	// 完成
	state.mu.Lock()
	state.state.Progress = 1.0
	state.state.BytesRemaining = 0
	state.state.Speed = 0
	state.state.ETA = 0
	state.state.LastUpdated = time.Now()
	state.stats.TotalDuration = time.Since(state.state.StartTime)
	state.mu.Unlock()
}

// updateSiteStatus 更新站点状态
func (r *ReplicationEngine) updateSiteStatus(siteID string, status SiteStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if site, ok := r.sites[siteID]; ok {
		site.Status = status
		site.UpdatedAt = time.Now()
	}
}
