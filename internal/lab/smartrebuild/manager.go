// Package smartrebuild 智能RAID重建引擎 - 核心管理逻辑
package smartrebuild

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewManagerWithLogger 创建带日志的管理器.
func NewManagerWithLogger(cfg RebuildConfig, logger *zap.Logger) *Manager {
	mgr := NewManager(cfg)
	// logger 可以在后续扩展中存储
	return mgr
}

// ========== 任务管理 ==========

// CreateJob 创建重建任务.
func (m *Manager) CreateJob(ctx context.Context, poolName string, targetDisk DiskInfo, sourceDisks []DiskInfo) (*RebuildJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(sourceDisks) == 0 {
		return nil, fmt.Errorf("no source disks provided")
	}

	// 检查并行任务数限制
	activeJobs := m.countActiveJobs()
	if activeJobs >= m.config.MaxParallelJobs {
		return nil, fmt.Errorf("max parallel jobs (%d) reached, active: %d", m.config.MaxParallelJobs, activeJobs)
	}

	job := &RebuildJob{
		ID:          uuid.New().String(),
		PoolName:    poolName,
		SourceDisks: sourceDisks,
		TargetDisk:  targetDisk,
		State:       StatePending,
		TotalBytes:  targetDisk.SizeBytes,
		StartTime:   time.Now(),
		Segments:    m.generateSegments(targetDisk.SizeBytes),
	}

	m.jobs[job.ID] = job
	return job, nil
}

// StartJob 启动重建任务.
func (m *Manager) StartJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.State != StatePending && job.State != StatePaused {
		return fmt.Errorf("job %s cannot be started from state: %s", jobID, job.State)
	}

	// 检查并行限制
	activeJobs := m.countActiveJobs()
	if activeJobs >= m.config.MaxParallelJobs {
		return fmt.Errorf("max parallel jobs reached")
	}

	job.State = StateRunning
	return nil
}

// PauseJob 暂停重建任务.
func (m *Manager) PauseJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.State != StateRunning {
		return fmt.Errorf("job %s is not running", jobID)
	}

	job.State = StatePaused
	return nil
}

// CancelJob 取消重建任务.
func (m *Manager) CancelJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.State == StateCompleted || job.State == StateFailed {
		return fmt.Errorf("job %s already finished", jobID)
	}

	job.State = StateCancelled
	now := time.Now()
	job.EndTime = &now
	return nil
}

// GetJob 获取重建任务.
func (m *Manager) GetJob(ctx context.Context, jobID string) (*RebuildJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// ListJobs 列出所有重建任务.
func (m *Manager) ListJobs(ctx context.Context) []*RebuildJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*RebuildJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// ========== 智能优先级 ==========

// PrioritizeSegments 根据热度和重要性对数据段排序.
func (m *Manager) PrioritizeSegments(segments []DataSegment) []DataSegment {
	prioritized := make([]DataSegment, len(segments))
	copy(prioritized, segments)

	sort.Slice(prioritized, func(i, j int) bool {
		// 综合评分 = 热度 * 0.6 + 重要性 * 0.4
		scoreI := prioritized[i].HotScore*0.6 + prioritized[i].Importance*0.4
		scoreJ := prioritized[j].HotScore*0.6 + prioritized[j].Importance*0.4
		return scoreI > scoreJ
	})

	// 根据评分分配优先级
	for i := range prioritized {
		score := prioritized[i].HotScore*0.6 + prioritized[i].Importance*0.4
		switch {
		case score >= 0.8:
			prioritized[i].Priority = PriorityCritical
		case score >= 0.6:
			prioritized[i].Priority = PriorityHigh
		case score >= 0.3:
			prioritized[i].Priority = PriorityNormal
		default:
			prioritized[i].Priority = PriorityLow
		}
	}

	return prioritized
}

// UpdateHotScore 更新数据段热度.
func (m *Manager) UpdateHotScore(segmentID string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	m.hotDataMap[segmentID] = score
}

// ========== 并行调度 ==========

// ScheduleParallel 并行调度多个重建任务.
func (m *Manager) ScheduleParallel(ctx context.Context, jobs []*RebuildJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeJobs := m.countActiveJobs()
	available := m.config.MaxParallelJobs - activeJobs

	if available <= 0 {
		return fmt.Errorf("no available slots for parallel jobs")
	}

	// 按优先级排序
	sort.Slice(jobs, func(i, j int) bool {
		return m.jobPriority(jobs[i]) < m.jobPriority(jobs[j])
	})

	started := 0
	for _, job := range jobs {
		if started >= available {
			break
		}
		if job.State == StatePending {
			job.State = StateRunning
			started++
		}
	}

	return nil
}

// GetActiveJobCount 获取活跃任务数.
func (m *Manager) GetActiveJobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.countActiveJobs()
}

// ========== 进度预测 ==========

// EstimateETA 预估剩余时间.
func (m *Manager) EstimateETA(job *RebuildJob) time.Duration {
	if job.AvgSpeed <= 0 || job.TotalBytes <= 0 {
		return 0
	}

	remaining := job.TotalBytes - job.RebuiltBytes
	if remaining <= 0 {
		return 0
	}

	seconds := float64(remaining) / float64(job.AvgSpeed)
	return time.Duration(seconds * float64(time.Second))
}

// UpdateProgress 更新重建进度.
func (m *Manager) UpdateProgress(jobID string, rebuiltBytes int64, currentSpeed int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.RebuiltBytes = rebuiltBytes
	job.CurrentSpeed = currentSpeed

	if job.TotalBytes > 0 {
		job.Progress = float64(rebuiltBytes) / float64(job.TotalBytes) * 100
	}

	// 计算平均速度
	elapsed := time.Since(job.StartTime).Seconds()
	if elapsed > 0 {
		job.AvgSpeed = int64(float64(rebuiltBytes) / elapsed)
	}

	// 计算ETA
	job.ETA = m.EstimateETA(job)

	// 完成检查
	if job.Progress >= 100 {
		job.Progress = 100
		job.State = StateCompleted
		now := time.Now()
		job.EndTime = &now
	}

	return nil
}

// GetProgressSnapshot 获取进度快照.
func (m *Manager) GetProgressSnapshot() ProgressSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSpeed int64
	activeJobs := 0
	var totalProgress float64

	for _, job := range m.jobs {
		if job.State == StateRunning {
			activeJobs++
			totalSpeed += job.CurrentSpeed
			totalProgress += job.Progress
		}
	}

	avgProgress := 0.0
	if activeJobs > 0 {
		avgProgress = totalProgress / float64(activeJobs)
	}

	return ProgressSnapshot{
		Timestamp:     time.Now(),
		Progress:      avgProgress,
		Speed:         totalSpeed,
		ActiveJobs:    activeJobs,
		IOUtil:        m.calculateIoutil(),
		RebuildIOMBps: totalSpeed / (1024 * 1024),
	}
}

// ========== 性能保护 ==========

// ThrottleRebuild 限速重建，保证业务IO.
func (m *Manager) ThrottleRebuild(jobID string, businessIOPS int64) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return 0, fmt.Errorf("job not found: %s", jobID)
	}

	if job.State != StateRunning {
		return 0, fmt.Errorf("job is not running")
	}

	// 计算可用带宽
	maxSpeed := int64(m.config.MaxDiskSpeedMBps) * 1024 * 1024
	rebuildBudget := int64(float64(maxSpeed) * m.config.RebuildIOWeight)

	// 如果业务IO高，进一步降低重建速度
	if businessIOPS > 1000 {
		// 高业务负载时，重建速度降低50%
		rebuildBudget = rebuildBudget / 2
	} else if businessIOPS > 500 {
		// 中等业务负载，降低30%
		rebuildBudget = rebuildBudget * 70 / 100
	}

	// 温度保护
	if m.isOverheated(job.TargetDisk.TempC) {
		// 温度过高，降低到最低速度
		rebuildBudget = maxSpeed / 10
	}

	return rebuildBudget, nil
}

// SetIOMetrics 更新IO指标.
func (m *Manager) SetIOMetrics(metrics IOMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ioMetrics = &metrics
}

// ========== 调度管理 ==========

// CreateSchedule 创建重建调度计划.
func (m *Manager) CreateSchedule(ctx context.Context, schedule *RebuildSchedule) (*RebuildSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()

	if schedule.MaxParallel <= 0 {
		schedule.MaxParallel = m.config.MaxParallelJobs
	}
	if schedule.ThrottleMBps <= 0 {
		schedule.ThrottleMBps = m.config.MaxDiskSpeedMBps
	}

	m.schedules[schedule.ID] = schedule
	return schedule, nil
}

// ListSchedules 列出调度计划.
func (m *Manager) ListSchedules(ctx context.Context) []*RebuildSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*RebuildSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetSchedule 获取调度计划.
func (m *Manager) GetSchedule(ctx context.Context, scheduleID string) (*RebuildSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.schedules[scheduleID]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", scheduleID)
	}
	return s, nil
}

// DeleteSchedule 删除调度计划.
func (m *Manager) DeleteSchedule(ctx context.Context, scheduleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[scheduleID]; !ok {
		return fmt.Errorf("schedule not found: %s", scheduleID)
	}
	delete(m.schedules, scheduleID)
	return nil
}

// ========== 内部方法 ==========

func (m *Manager) countActiveJobs() int {
	count := 0
	for _, job := range m.jobs {
		if job.State == StateRunning {
			count++
		}
	}
	return count
}

func (m *Manager) generateSegments(totalBytes int64) []DataSegment {
	segmentSize := m.config.SegmentSizeBytes
	if segmentSize <= 0 {
		segmentSize = 4 * 1024 * 1024
	}

	numSegments := int(math.Ceil(float64(totalBytes) / float64(segmentSize)))
	segments := make([]DataSegment, numSegments)

	for i := 0; i < numSegments; i++ {
		offset := int64(i) * segmentSize
		size := segmentSize
		if offset+size > totalBytes {
			size = totalBytes - offset
		}

		segments[i] = DataSegment{
			ID:         fmt.Sprintf("seg-%d", i),
			Offset:     offset,
			SizeBytes:  size,
			HotScore:   0.5, // 默认中等热度
			Importance: 0.5, // 默认中等重要性
			Priority:   PriorityNormal,
		}
	}

	return segments
}

func (m *Manager) jobPriority(job *RebuildJob) int {
	// 根据磁盘状态计算优先级
	switch job.TargetDisk.Status {
	case DiskStatusFaulted:
		return 1
	case DiskStatusDegraded:
		return 2
	default:
		return 3
	}
}

func (m *Manager) calculateIoutil() float64 {
	if m.ioMetrics == nil {
		return 0
	}
	return m.ioMetrics.Ioutil
}

func (m *Manager) isOverheated(tempC int) bool {
	return tempC > m.config.TempThreshold
}
