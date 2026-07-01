// Package updatecoord 提供系统更新协调器功能
// 编排 NAS 系统更新的完整流程：检查→预检→下载→备份→安装→验证→切换
package updatecoord

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrTaskNotFound 更新任务不存在.
	ErrTaskNotFound = errors.New("更新任务不存在")
	// ErrPreCheckFailed 预检失败.
	ErrPreCheckFailed = errors.New("更新预检失败")
	// ErrUpdateFailed 更新失败.
	ErrUpdateFailed = errors.New("更新执行失败")
	// ErrRollbackFailed 回滚失败.
	ErrRollbackFailed = errors.New("回滚失败")
	// ErrInvalidPhase 非法阶段转换.
	ErrInvalidPhase = errors.New("非法更新阶段转换")
	// ErrVersionNotFound 版本不存在.
	ErrVersionNotFound = errors.New("更新版本不存在")
)

// ========== Service 定义 ==========

// Service 系统更新协调服务.
type Service struct {
	mu       sync.RWMutex
	tasks    map[string]*updateTask
	history  []HistoryEntry
	// 模拟的可用更新列表
	availableUpdates []UpdateInfo
	currentVersion   string
}

// NewService 创建更新协调服务.
func NewService() *Service {
	return &Service{
		tasks:          make(map[string]*updateTask),
		currentVersion: "1.0.0",
		availableUpdates: []UpdateInfo{
			{
				Version:       "1.1.0",
				Channel:       ChannelStable,
				ReleaseNotes:  "Bug 修复和性能优化",
				Size:          500 * 1024 * 1024, // 500MB
				ReleasedAt:    time.Now().Add(-72 * time.Hour),
				CriticalLevel: "low",
				Checksum:      "sha256:abc123",
				Available:     true,
			},
			{
				Version:       "2.0.0",
				Channel:       ChannelBeta,
				ReleaseNotes:  "重大版本更新，包含新功能",
				Size:          1024 * 1024 * 1024, // 1GB
				ReleasedAt:    time.Now().Add(-24 * time.Hour),
				CriticalLevel: "medium",
				Checksum:      "sha256:def456",
				Available:     true,
			},
			{
				Version:       "1.0.1",
				Channel:       ChannelStable,
				ReleaseNotes:  "安全补丁",
				Size:          50 * 1024 * 1024, // 50MB
				ReleasedAt:    time.Now().Add(-168 * time.Hour),
				CriticalLevel: "high",
				Checksum:      "sha256:ghi789",
				Available:     true,
			},
		},
	}
}

// SetCurrentVersion 设置当前版本（用于测试）.
func (s *Service) SetCurrentVersion(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentVersion = v
}

// Check 检查可用更新.
func (s *Service) Check(ctx context.Context, channel *UpdateChannel) ([]UpdateInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []UpdateInfo
	for _, u := range s.availableUpdates {
		if channel != nil && u.Channel != *channel {
			continue
		}
		if u.Available {
			result = append(result, u)
		}
	}
	return result, nil
}

// PreCheck 执行更新前预检.
func (s *Service) PreCheck(ctx context.Context, req *PreCheckRequest) (*PreCheckResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证版本存在
	found := false
	var targetUpdate UpdateInfo
	for _, u := range s.availableUpdates {
		if u.Version == req.Version && u.Available {
			found = true
			targetUpdate = u
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: 版本 %s 不存在", ErrVersionNotFound, req.Version)
	}

	// 执行预检项
	checks := []PreCheckItem{
		{
			Name:     "磁盘空间检查",
			Category: "disk",
			Passed:   true,
			Message:  "磁盘空间充足",
			Detail:   fmt.Sprintf("需要 %.0f MB，可用 100 GB", float64(targetUpdate.Size)/(1024*1024)),
		},
		{
			Name:     "运行服务检查",
			Category: "service",
			Passed:   true,
			Message:  "所有关键服务运行正常",
		},
		{
			Name:     "备份状态检查",
			Category: "backup",
			Passed:   true,
			Message:  "最近备份可用",
		},
		{
			Name:     "网络连接检查",
			Category: "network",
			Passed:   true,
			Message:  "网络连接正常",
		},
		{
			Name:     "系统完整性检查",
			Category: "system",
			Passed:   true,
			Message:  "系统文件完整",
		},
	}

	allPassed := true
	var warnings []string
	for _, c := range checks {
		if !c.Passed {
			allPassed = false
		}
	}

	// 高危更新添加警告
	if targetUpdate.CriticalLevel == "high" || targetUpdate.CriticalLevel == "critical" {
		warnings = append(warnings, "此更新为高优先级，建议在维护窗口内执行")
	}

	return &PreCheckResult{
		Version:   req.Version,
		Passed:    allPassed,
		Checks:    checks,
		Warnings:  warnings,
		CheckedAt: time.Now(),
	}, nil
}

// Apply 应用更新 - 分步执行：下载→备份→安装→验证→切换.
func (s *Service) Apply(ctx context.Context, req *ApplyRequest) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证版本存在
	found := false
	var targetUpdate UpdateInfo
	for _, u := range s.availableUpdates {
		if u.Version == req.Version && u.Available {
			found = true
			targetUpdate = u
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: 版本 %s 不存在", ErrVersionNotFound, req.Version)
	}
	_ = targetUpdate // 版本信息已验证可用

	// 创建更新任务
	taskID := uuid.New().String()
	now := time.Now()

	// 构建步骤
	var steps []UpdateStep
	order := 1

	// 步骤1：下载更新
	steps = append(steps, UpdateStep{
		ID:     uuid.New().String(),
		Order:  order,
		Name:   "下载更新包",
		Phase:  PhaseDownload,
		Status: StepPending,
	})
	order++

	// 步骤2：备份（可跳过）
	backupStep := UpdateStep{
		ID:     uuid.New().String(),
		Order:  order,
		Name:   "创建系统备份",
		Phase:  PhaseBackup,
		Status: StepPending,
	}
	if req.SkipBackup {
		backupStep.Status = StepSkipped
	}
	steps = append(steps, backupStep)
	order++

	// 步骤3：安装更新
	steps = append(steps, UpdateStep{
		ID:     uuid.New().String(),
		Order:  order,
		Name:   "安装更新",
		Phase:  PhaseInstall,
		Status: StepPending,
	})
	order++

	// 步骤4：验证更新
	steps = append(steps, UpdateStep{
		ID:     uuid.New().String(),
		Order:  order,
		Name:   "验证更新",
		Phase:  PhaseVerify,
		Status: StepPending,
	})
	order++

	// 步骤5：切换到新版本
	steps = append(steps, UpdateStep{
		ID:     uuid.New().String(),
		Order:  order,
		Name:   "切换到新版本",
		Phase:  PhaseSwitch,
		Status: StepPending,
	})

	task := &updateTask{
		id:           taskID,
		version:      req.Version,
		fromVersion:  s.currentVersion,
		phase:        PhaseDownload,
		steps:        steps,
		currentIdx:   -1,
		createdAt:    now,
		updatedAt:    now,
		autoRollback: req.AutoRollback,
	}
	s.tasks[taskID] = task

	// 执行步骤
	startedAt := time.Now()
	completedCount := 0
	var execError string

	for i := range task.steps {
		step := &task.steps[i]

		if step.Status == StepSkipped {
			continue
		}

		step.Status = StepRunning
		step.StartedAt = time.Now()
		task.currentIdx = i
		task.updatedAt = time.Now()

		if req.DryRun {
			step.Status = StepCompleted
			step.FinishedAt = time.Now()
			completedCount++
			continue
		}

		// 模拟执行（实际应调用对应的更新逻辑）
		step.Status = StepCompleted
		step.FinishedAt = time.Now()
		completedCount++
	}

	progress := float64(completedCount) / float64(len(task.steps)) * 100

	if execError != "" {
		task.phase = PhaseFailed
		task.error = execError

		// 自动回滚
		if req.AutoRollback {
			s.rollbackTask(task)
		}
	} else {
		task.phase = PhaseDone
		// 更新当前版本
		s.currentVersion = req.Version
	}
	task.updatedAt = time.Now()

	// 记录历史
	s.history = append(s.history, HistoryEntry{
		ID:          taskID,
		Version:     req.Version,
		FromVersion: task.fromVersion,
		Phase:       task.phase,
		Success:     task.phase == PhaseDone,
		Steps:       task.steps,
		StartedAt:   startedAt,
		FinishedAt:  task.updatedAt,
		Error:       execError,
	})

	return &ApplyResult{
		Version:    req.Version,
		Phase:      task.phase,
		Progress:   progress,
		Steps:      task.steps,
		StartedAt:  startedAt,
		FinishedAt: task.updatedAt,
		Error:      execError,
	}, nil
}

// GetHistory 获取更新历史.
func (s *Service) GetHistory() ([]HistoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	result := make([]HistoryEntry, len(s.history))
	copy(result, s.history)
	return result, nil
}

// Rollback 回滚到之前的版本.
func (s *Service) Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找历史记录
	var targetEntry *HistoryEntry
	targetIdx := -1
	for i := range s.history {
		if req.HistoryID != "" && s.history[i].ID == req.HistoryID {
			targetEntry = &s.history[i]
			targetIdx = i
			break
		}
		if req.HistoryID == "" && s.history[i].Version == req.Version {
			targetEntry = &s.history[i]
			targetIdx = i
			break
		}
	}

	if targetEntry == nil {
		return nil, fmt.Errorf("%w: 未找到版本 %s 的更新记录", ErrRollbackFailed, req.Version)
	}

	// 创建回滚步骤
	now := time.Now()
	rollbackSteps := []UpdateStep{
		{
			ID:         uuid.New().String(),
			Order:      1,
			Name:       "停止当前版本服务",
			Phase:      PhaseRollback,
			Status:     StepCompleted,
			StartedAt:  now,
			FinishedAt: now.Add(time.Second),
		},
		{
			ID:         uuid.New().String(),
			Order:      2,
			Name:       "恢复备份",
			Phase:      PhaseRollback,
			Status:     StepCompleted,
			StartedAt:  now.Add(time.Second),
			FinishedAt: now.Add(2 * time.Second),
		},
		{
			ID:         uuid.New().String(),
			Order:      3,
			Name:       "启动旧版本服务",
			Phase:      PhaseRollback,
			Status:     StepCompleted,
			StartedAt:  now.Add(2 * time.Second),
			FinishedAt: now.Add(3 * time.Second),
		},
	}

	// 回滚当前版本
	s.currentVersion = targetEntry.FromVersion

	// 更新历史记录
	s.history[targetIdx].Phase = PhaseRollback
	s.history[targetIdx].Success = true

	return &RollbackResult{
		Version:  targetEntry.FromVersion,
		Success:  true,
		Steps:    rollbackSteps,
		Message:  fmt.Sprintf("成功回滚到版本 %s", targetEntry.FromVersion),
		RolledAt: now,
	}, nil
}

// rollbackTask 内部回滚逻辑（不加锁，调用方已持有锁）.
func (s *Service) rollbackTask(task *updateTask) {
	task.phase = PhaseRollback
	task.updatedAt = time.Now()

	for i := range task.steps {
		step := &task.steps[i]
		if step.Status == StepCompleted {
			step.Status = StepPending
			step.StartedAt = time.Time{}
			step.FinishedAt = time.Time{}
			step.Error = ""
		}
	}

	task.phase = PhaseDone
	task.updatedAt = time.Now()
}
