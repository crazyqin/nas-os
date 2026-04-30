// Package active Active Backup 调度引擎
// 负责备份任务的生命周期管理、并发控制和任务编排
package active

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EngineState 引擎状态
type EngineState string

const (
	EngineStateIdle     EngineState = "idle"
	EngineStateRunning  EngineState = "running"
	EngineStateStopping EngineState = "stopping"
)

// TaskRun 单次任务执行记录
type TaskRun struct {
	ID         string       `json:"id"`
	JobID      string       `json:"job_id"`
	BackupType BackupType   `json:"backup_type"`
	Status     BackupStatus `json:"status"`
	Progress   float64      `json:"progress"` // 0.0 ~ 1.0
	BytesDone  int64        `json:"bytes_done"`
	BytesTotal int64        `json:"bytes_total"`
	FilesDone  int          `json:"files_done"`
	FilesTotal int          `json:"files_total"`
	Error      string       `json:"error,omitempty"`
	StartedAt  time.Time    `json:"started_at"`
	EndedAt    *time.Time   `json:"ended_at,omitempty"`
}

// EngineEvent 引擎事件（供 dashboard 订阅）
type EngineEvent struct {
	Type      string     `json:"type"` // "task_start", "task_progress", "task_complete", "task_fail"
	Timestamp time.Time  `json:"timestamp"`
	JobID     string     `json:"job_id"`
	TaskRunID string     `json:"task_run_id,omitempty"`
	Progress  float64    `json:"progress,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// EngineEventCallback 引擎事件回调函数
type EngineEventCallback func(event EngineEvent)

// Engine 备份调度引擎
type Engine struct {
	mu            sync.RWMutex
	state         EngineState
	manager       *BackupManager
	dedupEngine   *CDCEngine        // CDC 全局去重
	scheduler     *ScheduleManager   // 调度管理器
	agentRegistry *AgentRegistry     // 远程代理注册表
	logger        *zap.Logger
	taskRuns      map[string]*TaskRun   // taskRunID -> run
	running       map[string]context.CancelFunc // jobID -> cancel
	maxConcurrent int
	sem           chan struct{}
	eventCallback EngineEventCallback
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrent   int    `json:"max_concurrent"`    // 最大并发任务数
	DedupBlockSize  int    `json:"dedup_block_size"`   // 去重块大小（字节）
	DedupMinSize    int    `json:"dedup_min_size"`     // CDC 最小块
	DedupMaxSize    int    `json:"dedup_max_size"`     // CDC 最大块
	AgentListenAddr string `json:"agent_listen_addr"`  // 代理监听地址
	StoragePath     string `json:"storage_path"`       // 存储路径
}

// DefaultEngineConfig 返回默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		MaxConcurrent:   4,
		DedupBlockSize:  0,
		DedupMinSize:    64 * 1024,    // 64KB
		DedupMaxSize:    8 * 1024 * 1024, // 8MB
		AgentListenAddr: ":9843",
		StoragePath:     "/var/lib/nas-os/backup/active",
	}
}

// NewEngine 创建备份调度引擎
func NewEngine(manager *BackupManager, config *EngineConfig, logger *zap.Logger) (*Engine, error) {
	if config == nil {
		config = DefaultEngineConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if manager == nil {
		return nil, fmt.Errorf("BackupManager 不能为空")
	}

	// 创建 CDC 去重引擎
	cdcEngine := NewCDCEngine(config.DedupMinSize, config.DedupMaxSize, logger)

	// 创建调度管理器
	scheduleMgr, err := NewScheduleManager(logger)
	if err != nil {
		return nil, fmt.Errorf("创建调度管理器失败: %w", err)
	}

	// 创建代理注册表
	agentRegistry := NewAgentRegistry(logger)

	e := &Engine{
		state:         EngineStateIdle,
		manager:       manager,
		dedupEngine:   cdcEngine,
		scheduler:     scheduleMgr,
		agentRegistry: agentRegistry,
		logger:        logger,
		taskRuns:      make(map[string]*TaskRun),
		running:       make(map[string]context.CancelFunc),
		maxConcurrent: config.MaxConcurrent,
		sem:           make(chan struct{}, config.MaxConcurrent),
		stopCh:        make(chan struct{}),
	}

	return e, nil
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.state == EngineStateRunning {
		e.mu.Unlock()
		return fmt.Errorf("引擎已在运行中")
	}
	e.state = EngineStateRunning
	e.mu.Unlock()

	e.wg.Add(1)
	go e.scheduleLoop(ctx)

	e.logger.Info("Active Backup 引擎已启动",
		zap.Int("max_concurrent", e.maxConcurrent))

	e.emitEvent(EngineEvent{
		Type:      "engine_start",
		Timestamp: time.Now(),
		Message:   "引擎启动",
	})

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.state != EngineStateRunning {
		e.mu.Unlock()
		return nil
	}
	e.state = EngineStateStopping
	e.mu.Unlock()

	close(e.stopCh)

	// 取消所有运行中的任务
	e.mu.Lock()
	for jobID, cancel := range e.running {
		e.logger.Info("取消运行中的任务", zap.String("job_id", jobID))
		cancel()
	}
	e.mu.Unlock()

	e.wg.Wait()

	e.mu.Lock()
	e.state = EngineStateIdle
	e.mu.Unlock()

	e.logger.Info("Active Backup 引擎已停止")
	return nil
}

// GetState 获取引擎状态
func (e *Engine) GetState() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// SetEventCallback 设置事件回调
func (e *Engine) SetEventCallback(cb EngineEventCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventCallback = cb
}

// SubmitTask 提交备份任务执行
func (e *Engine) SubmitTask(ctx context.Context, jobID string, backupType BackupType) (*TaskRun, error) {
	e.mu.RLock()
	if e.state != EngineStateRunning {
		e.mu.RUnlock()
		return nil, fmt.Errorf("引擎未运行")
	}
	e.mu.RUnlock()

	// 检查任务是否已在运行
	e.mu.RLock()
	if _, running := e.running[jobID]; running {
		e.mu.RUnlock()
		return nil, fmt.Errorf("任务 %s 已在执行中", jobID)
	}
	e.mu.RUnlock()

	// 获取信号量
	select {
	case e.sem <- struct{}{}:
	default:
		return nil, fmt.Errorf("已达最大并发数 %d", e.maxConcurrent)
	}

	job, err := e.manager.GetJob(jobID)
	if err != nil {
		<-e.sem
		return nil, err
	}

	taskRun := &TaskRun{
		ID:         uuid.New().String(),
		JobID:      jobID,
		BackupType: backupType,
		Status:     BackupStatusRunning,
		StartedAt:  time.Now(),
	}

	e.mu.Lock()
	e.taskRuns[taskRun.ID] = taskRun
	runCtx, cancel := context.WithCancel(ctx)
	e.running[jobID] = cancel
	e.mu.Unlock()

	e.emitEvent(EngineEvent{
		Type:      "task_start",
		Timestamp: time.Now(),
		JobID:     jobID,
		TaskRunID: taskRun.ID,
		Message:   fmt.Sprintf("开始 %s 备份", backupType),
	})

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() { <-e.sem }()
		defer func() {
			e.mu.Lock()
			delete(e.running, jobID)
			e.mu.Unlock()
		}()

		e.executeTask(runCtx, job, taskRun)
	}()

	return taskRun, nil
}

// GetTaskRun 获取任务执行记录
func (e *Engine) GetTaskRun(taskRunID string) (*TaskRun, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	run, exists := e.taskRuns[taskRunID]
	if !exists {
		return nil, fmt.Errorf("任务执行记录 %s 不存在", taskRunID)
	}
	return run, nil
}

// ListTaskRuns 列出任务执行记录
func (e *Engine) ListTaskRuns(jobID string) []*TaskRun {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*TaskRun, 0)
	for _, run := range e.taskRuns {
		if jobID == "" || run.JobID == jobID {
			result = append(result, run)
		}
	}
	return result
}

// CancelTask 取消运行中的任务
func (e *Engine) CancelTask(jobID string) error {
	e.mu.RLock()
	cancel, exists := e.running[jobID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %s 未在运行中", jobID)
	}

	cancel()
	e.logger.Info("任务取消请求已发送", zap.String("job_id", jobID))
	return nil
}

// GetDedupEngine 获取去重引擎
func (e *Engine) GetDedupEngine() *CDCEngine {
	return e.dedupEngine
}

// GetScheduler 获取调度管理器
func (e *Engine) GetScheduler() *ScheduleManager {
	return e.scheduler
}

// GetAgentRegistry 获取代理注册表
func (e *Engine) GetAgentRegistry() *AgentRegistry {
	return e.agentRegistry
}

// GetStats 获取引擎统计信息
func (e *Engine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := EngineStats{
		State:         string(e.state),
		MaxConcurrent: e.maxConcurrent,
		ActiveTasks:   len(e.running),
		TotalRuns:     len(e.taskRuns),
		Agents:        e.agentRegistry.Count(),
	}

	for _, run := range e.taskRuns {
		stats.TotalBytesProcessed += run.BytesDone
		stats.TotalFilesProcessed += run.FilesDone
		if run.Status == BackupStatusCompleted {
			stats.CompletedRuns++
		} else if run.Status == BackupStatusFailed {
			stats.FailedRuns++
		}
	}

	return stats
}

// EngineStats 引擎统计信息
type EngineStats struct {
	State                string `json:"state"`
	MaxConcurrent        int    `json:"max_concurrent"`
	ActiveTasks          int    `json:"active_tasks"`
	TotalRuns            int    `json:"total_runs"`
	CompletedRuns        int    `json:"completed_runs"`
	FailedRuns           int    `json:"failed_runs"`
	TotalBytesProcessed  int64  `json:"total_bytes_processed"`
	TotalFilesProcessed  int    `json:"total_files_processed"`
	Agents               int    `json:"agents"`
}

// executeTask 执行备份任务核心逻辑
func (e *Engine) executeTask(ctx context.Context, job *BackupJob, run *TaskRun) {
	result, err := e.manager.RunBackup(ctx, run.JobID)

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	run.EndedAt = &now

	if err != nil {
		run.Status = BackupStatusFailed
		run.Error = err.Error()
		e.logger.Error("备份任务失败",
			zap.String("job_id", run.JobID),
			zap.Error(err))
		e.emitEvent(EngineEvent{
			Type:      "task_fail",
			Timestamp: now,
			JobID:     run.JobID,
			TaskRunID: run.ID,
			Message:   err.Error(),
		})
		return
	}

	run.Status = BackupStatusCompleted
	run.Progress = 1.0
	if result != nil {
		run.BytesTotal = result.TotalSize
		run.BytesDone = result.TotalSize
		run.FilesTotal = result.TotalFiles
		run.FilesDone = result.TotalFiles
	}

	e.emitEvent(EngineEvent{
		Type:      "task_complete",
		Timestamp: now,
		JobID:     run.JobID,
		TaskRunID: run.ID,
		Progress:  1.0,
		Message:   "备份完成",
	})
}

// scheduleLoop 定时调度循环
func (e *Engine) scheduleLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkScheduledJobs(ctx)
		}
	}
}

// checkScheduledJobs 检查需要执行的定时任务
func (e *Engine) checkScheduledJobs(ctx context.Context) {
	jobs := e.manager.ListJobs()
	now := time.Now()

	for _, job := range jobs {
		if !job.Schedule.Enabled {
			continue
		}
		if job.NextRun == nil || job.NextRun.After(now) {
			continue
		}
		if job.Status == BackupStatusRunning {
			continue
		}

		// 检查时间窗口
		if !e.scheduler.IsWithinTimeWindow(job.Schedule, now) {
			e.logger.Debug("跳过任务：不在时间窗口内",
				zap.String("job_id", job.ID))
			continue
		}

		e.logger.Info("触发定时备份任务",
			zap.String("job_id", job.ID),
			zap.String("name", job.Name))

		_, err := e.SubmitTask(ctx, job.ID, job.Policy.Type)
		if err != nil {
			e.logger.Error("提交定时任务失败",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}

		// 更新下次执行时间
		e.mu.Lock()
		job.NextRun = nil
		nextRun := e.scheduler.CalculateNextRun(job.Schedule)
		job.NextRun = &nextRun
		e.mu.Unlock()
	}
}

// emitEvent 发送引擎事件
func (e *Engine) emitEvent(event EngineEvent) {
	if e.eventCallback != nil {
		go e.eventCallback(event)
	}
}
