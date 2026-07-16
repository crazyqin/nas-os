// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"sync"
	"time"
)

// RestoreManager 恢复管理器.
type RestoreManager struct {
	mu      sync.RWMutex
	manager *Manager
	jobs    map[string]*RestoreJobState // jobID -> state
	quit    chan struct{}
	running bool
}

// RestoreJobState 恢复任务状态.
type RestoreJobState struct {
	Job         *RestoreJob `json:"job"`          // 恢复任务
	Stage       string      `json:"stage"`        // 当前阶段
	Message     string      `json:"message"`      // 状态消息
	StartedAt   time.Time   `json:"started_at"`   // 开始时间
	UpdatedAt   time.Time   `json:"updated_at"`   // 更新时间
	CompletedAt *time.Time  `json:"completed_at"` // 完成时间
}

// RestoreStage 恢复阶段常量.
const (
	// RestoreStageInit 初始化.
	RestoreStageInit = "init"
	// RestoreStagePreparing 准备中.
	RestoreStagePreparing = "preparing"
	// RestoreStageDownloading 下载数据.
	RestoreStageDownloading = "downloading"
	// RestoreStageWriting 写入数据.
	RestoreStageWriting = "writing"
	// RestoreStageVerifying 校验数据.
	RestoreStageVerifying = "verifying"
	// RestoreStageBootloader 引导修复.
	RestoreStageBootloader = "bootloader"
	// RestoreStageComplete 完成.
	RestoreStageComplete = "complete"
	// RestoreStageFailed 失败.
	RestoreStageFailed = "failed"
)

// NewRestoreManager 创建恢复管理器.
func NewRestoreManager(mgr *Manager) *RestoreManager {
	return &RestoreManager{
		manager: mgr,
		jobs:    make(map[string]*RestoreJobState),
		quit:    make(chan struct{}),
	}
}

// Start 启动恢复管理器.
func (rm *RestoreManager) Start() {
	rm.mu.Lock()
	if rm.running {
		rm.mu.Unlock()
		return
	}
	rm.running = true
	rm.mu.Unlock()

	go rm.monitorLoop()
}

// Stop 停止恢复管理器.
func (rm *RestoreManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.running {
		return
	}

	rm.running = false
	close(rm.quit)
	rm.quit = make(chan struct{})
}

// IsRunning 是否运行中.
func (rm *RestoreManager) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// monitorLoop 监控循环.
func (rm *RestoreManager) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.quit:
			return
		case <-ticker.C:
			rm.checkJobs()
		}
	}
}

// checkJobs 检查恢复任务状态.
func (rm *RestoreManager) checkJobs() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, state := range rm.jobs {
		if state.Job.Status == TaskStatusRunning {
			state.UpdatedAt = time.Now()
		}
	}
}

// StartRestore 启动恢复任务.
func (rm *RestoreManager) StartRestore(job *RestoreJob) *RestoreJobState {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state := &RestoreJobState{
		Job:       job,
		Stage:     RestoreStageInit,
		Message:   "恢复任务已创建",
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	rm.jobs[job.ID] = state
	return state
}

// UpdateStage 更新恢复阶段.
func (rm *RestoreManager) UpdateStage(jobID string, stage string, message string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.jobs[jobID]
	if !exists {
		return
	}

	state.Stage = stage
	state.Message = message
	state.UpdatedAt = time.Now()

	if stage == RestoreStageComplete || stage == RestoreStageFailed {
		now := time.Now()
		state.CompletedAt = &now
	}
}

// GetJobState 获取恢复任务状态.
func (rm *RestoreManager) GetJobState(jobID string) *RestoreJobState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.jobs[jobID]
}

// ListJobStates 列出所有恢复任务状态.
func (rm *RestoreManager) ListJobStates() []*RestoreJobState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*RestoreJobState, 0, len(rm.jobs))
	for _, s := range rm.jobs {
		result = append(result, s)
	}
	return result
}

// GetActiveRestoreCount 获取活跃恢复任务数.
func (rm *RestoreManager) GetActiveRestoreCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	count := 0
	for _, s := range rm.jobs {
		if s.Job.Status == TaskStatusRunning {
			count++
		}
	}
	return count
}

// ValidateRestoreRequest 验证恢复请求.
func (rm *RestoreManager) ValidateRestoreRequest(req RestoreRequest) error {
	// 验证恢复点存在
	if _, err := rm.manager.GetRestorePoint(req.RestorePointID); err != nil {
		return err
	}

	// 文件恢复需要指定文件列表
	if req.RestoreType == RestoryTypeFiles && len(req.Files) == 0 {
		return ErrRestorePointNotFound // 复用错误，实际可自定义更精确的错误
	}

	return nil
}

// GetRestoreChainSize 计算恢复链大小.
func (rm *RestoreManager) GetRestoreChainSize(pointID string) uint64 {
	chain := rm.manager.GetRestorePointChain(pointID)
	var total uint64
	for _, p := range chain {
		total += p.Size
	}
	return total
}

// GetEstimatedTime 估算恢复时间.
func (rm *RestoreManager) GetEstimatedTime(pointID string, speedBytesPerSec uint64) time.Duration {
	if speedBytesPerSec == 0 {
		speedBytesPerSec = 100 * 1024 * 1024 // 默认 100MB/s
	}
	totalSize := rm.GetRestoreChainSize(pointID)
	if totalSize == 0 {
		return 0
	}
	seconds := totalSize / speedBytesPerSec
	return time.Duration(seconds) * time.Second
}
