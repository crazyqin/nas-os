// Package sysmigrate 提供系统迁移向导功能
// 引导用户完成 NAS 系统从源平台到当前平台的完整迁移流程
package sysmigrate

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
	// ErrTaskNotFound 迁移任务不存在.
	ErrTaskNotFound = errors.New("迁移任务不存在")
	// ErrAssessmentFailed 迁移评估失败.
	ErrAssessmentFailed = errors.New("迁移评估失败")
	// ErrPlanNotFound 迁移计划不存在.
	ErrPlanNotFound = errors.New("迁移计划不存在")
	// ErrStepFailed 步骤执行失败.
	ErrStepFailed = errors.New("步骤执行失败")
	// ErrRollbackFailed 回滚失败.
	ErrRollbackFailed = errors.New("回滚失败")
	// ErrInvalidPhase 非法阶段转换.
	ErrInvalidPhase = errors.New("非法迁移阶段转换")
)

// ========== Service 定义 ==========

// Service 系统迁移服务.
type Service struct {
	mu    sync.RWMutex
	tasks map[string]*migrationTask
}

// NewService 创建系统迁移服务.
func NewService() *Service {
	return &Service{
		tasks: make(map[string]*migrationTask),
	}
}

// Assess 执行迁移评估.
func (s *Service) Assess(ctx context.Context, req *AssessRequest) (*AssessResult, error) {
	// 验证请求
	if req.SourceHost == "" {
		return nil, fmt.Errorf("%w: 源主机地址不能为空", ErrAssessmentFailed)
	}
	if req.SourceUser == "" {
		return nil, fmt.Errorf("%w: 源系统用户名不能为空", ErrAssessmentFailed)
	}
	if req.TargetPath == "" {
		return nil, fmt.Errorf("%w: 目标路径不能为空", ErrAssessmentFailed)
	}

	// 创建任务
	taskID := uuid.New().String()
	now := time.Now()

	// 构造源系统信息（实际应通过 SSH/API 探测）
	sourceInfo := &SourceSystemInfo{
		Type:         req.SourceType,
		Version:      "未知",
		Hostname:     req.SourceHost,
		TotalStorage: 0,
		UsedStorage:  0,
		UserCount:    0,
		ShareCount:   0,
		ServiceCount: 0,
	}

	// 兼容性检查
	compatible := true
	var warnings, blockers []string

	switch req.SourceType {
	case SourceSynology, SourceQNAP, SourceTrueNAS, SourceUnraid:
		// 支持的源类型
	case SourceGeneric:
		warnings = append(warnings, "通用源系统类型，部分数据可能无法自动映射")
	default:
		compatible = false
		blockers = append(blockers, fmt.Sprintf("不支持的源系统类型: %s", req.SourceType))
	}

	// 存储兼容性检查（简化）
	if sourceInfo.UsedStorage > 0 {
		warnings = append(warnings, "请确保目标系统有足够的存储空间")
	}

	estimatedSize := sourceInfo.UsedStorage
	estimatedDuration := estimateDuration(estimatedSize, sourceInfo.UserCount)

	task := &migrationTask{
		id:         taskID,
		phase:      PhaseAssess,
		sourceInfo: sourceInfo,
		steps:      nil,
		currentIdx: -1,
		createdAt:  now,
		updatedAt:  now,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	return &AssessResult{
		TaskID:            taskID,
		Compatible:        compatible,
		SourceInfo:        sourceInfo,
		Warnings:          warnings,
		Blockers:          blockers,
		EstimatedDuration: estimatedDuration,
		EstimatedDataSize: estimatedSize,
		AssessedAt:        now,
	}, nil
}

// Plan 生成迁移计划.
func (s *Service) Plan(ctx context.Context, req *PlanRequest) (*PlanResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.phase != PhaseAssess && task.phase != PhasePlan {
		return nil, fmt.Errorf("%w: 当前阶段为 %s，无法生成计划", ErrInvalidPhase, task.phase)
	}

	// 按类别生成步骤
	var steps []MigrationStep
	order := 1

	// 第一步：创建备份快照
	steps = append(steps, MigrationStep{
		ID:       uuid.New().String(),
		Order:    order,
		Category: CategoryConfig,
		Name:     "创建系统备份快照",
		Status:   StepPending,
	})
	order++

	for _, cat := range req.Categories {
		var stepName string
		switch cat {
		case CategoryData:
			stepName = "迁移用户数据"
		case CategoryConfig:
			stepName = "迁移系统配置"
		case CategoryUsers:
			stepName = "迁移用户和权限"
		case CategoryServices:
			stepName = "迁移服务和应用"
		case CategoryNetwork:
			stepName = "迁移网络配置"
		case CategoryShared:
			stepName = "迁移共享文件夹"
		case CategoryCert:
			stepName = "迁移证书"
		case CategorySchedule:
			stepName = "迁移计划任务"
		default:
			stepName = fmt.Sprintf("迁移 %s", string(cat))
		}

		steps = append(steps, MigrationStep{
			ID:       uuid.New().String(),
			Order:    order,
			Category: cat,
			Name:     stepName,
			Status:   StepPending,
		})
		order++
	}

	// 最后一步：验证迁移结果
	steps = append(steps, MigrationStep{
		ID:       uuid.New().String(),
		Order:    order,
		Category: CategoryConfig,
		Name:     "验证迁移结果",
		Status:   StepPending,
	})

	task.steps = steps
	task.phase = PhasePlan
	task.updatedAt = time.Now()

	timeline := fmt.Sprintf("预计 %d 个步骤，耗时约 %s", len(steps), estimateDuration(task.sourceInfo.UsedStorage, task.sourceInfo.UserCount))

	return &PlanResult{
		TaskID:     req.TaskID,
		Steps:      steps,
		Timeline:   timeline,
		TotalSteps: len(steps),
		CreatedAt:  task.updatedAt,
	}, nil
}

// Execute 执行迁移.
func (s *Service) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.phase != PhasePlan && task.phase != PhaseExecute {
		return nil, fmt.Errorf("%w: 当前阶段为 %s，无法执行迁移", ErrInvalidPhase, task.phase)
	}

	if len(task.steps) == 0 {
		return nil, ErrPlanNotFound
	}

	// 构建跳过集合
	skipSet := make(map[string]bool)
	for _, id := range req.SkipSteps {
		skipSet[id] = true
	}

	task.phase = PhaseExecute
	startedAt := time.Now()

	completedCount := 0
	var execError string

	for i := range task.steps {
		step := &task.steps[i]

		// 跳过指定步骤
		if skipSet[step.ID] {
			step.Status = StepSkipped
			continue
		}

		// 已完成的步骤跳过
		if step.Status == StepCompleted {
			completedCount++
			continue
		}

		step.Status = StepRunning
		step.StartedAt = time.Now()
		task.currentIdx = i
		task.updatedAt = time.Now()

		// 模拟执行步骤（实际应调用对应的迁移逻辑）
		if req.DryRun {
			step.Status = StepCompleted
			step.FinishedAt = time.Now()
			completedCount++
			continue
		}

		// 实际迁移执行 - 此处为模拟成功
		step.Status = StepCompleted
		step.FinishedAt = time.Now()
		completedCount++
	}

	progress := float64(completedCount) / float64(len(task.steps)) * 100

	if execError != "" {
		task.phase = PhaseFailed
		task.error = execError
	} else {
		task.phase = PhaseDone
	}
	task.updatedAt = time.Now()

	return &ExecuteResult{
		TaskID:     req.TaskID,
		Phase:      task.phase,
		Progress:   progress,
		Steps:      task.steps,
		StartedAt:  startedAt,
		FinishedAt: task.updatedAt,
		Error:      execError,
	}, nil
}

// Rollback 回滚迁移.
func (s *Service) Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.phase != PhaseDone && task.phase != PhaseFailed && task.phase != PhaseExecute {
		return nil, fmt.Errorf("%w: 当前阶段为 %s，无法回滚", ErrInvalidPhase, task.phase)
	}

	task.phase = PhaseRollback
	task.updatedAt = time.Now()

	// 回滚策略：从当前步骤倒序回滚
	rolledBack := 0
	rollbackStart := len(task.steps) - 1

	// 如果指定了步骤ID，回滚到该步骤之前
	if req.StepID != "" {
		for i, step := range task.steps {
			if step.ID == req.StepID {
				rollbackStart = i
				break
			}
		}
	}

	for i := rollbackStart; i >= 0; i-- {
		step := &task.steps[i]
		if step.Status == StepCompleted {
			// 模拟回滚（实际应调用对应的回滚逻辑）
			step.Status = StepPending
			step.StartedAt = time.Time{}
			step.FinishedAt = time.Time{}
			step.Error = ""
			rolledBack++
		}
	}

	task.phase = PhaseDone
	task.updatedAt = time.Now()

	return &RollbackResult{
		TaskID:   req.TaskID,
		Success:  true,
		Steps:    task.steps,
		Message:  fmt.Sprintf("成功回滚 %d 个步骤", rolledBack),
		RolledAt: task.updatedAt,
	}, nil
}

// GetStatus 获取迁移状态.
func (s *Service) GetStatus(taskID string) (*MigrationStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	completedCount := 0
	for _, step := range task.steps {
		if step.Status == StepCompleted {
			completedCount++
		}
	}

	progress := 0.0
	if len(task.steps) > 0 {
		progress = float64(completedCount) / float64(len(task.steps)) * 100
	}

	var currentStep *MigrationStep
	if task.currentIdx >= 0 && task.currentIdx < len(task.steps) {
		currentStep = &task.steps[task.currentIdx]
	}

	return &MigrationStatus{
		TaskID:      taskID,
		Phase:       task.phase,
		Progress:    progress,
		CurrentStep: currentStep,
		Steps:       task.steps,
		CreatedAt:   task.createdAt,
		UpdatedAt:   task.updatedAt,
		Error:       task.error,
	}, nil
}

// estimateDuration 估算迁移耗时.
func estimateDuration(dataSize int64, userCount int) string {
	if dataSize <= 0 {
		return "1-2 小时"
	}
	// 粗略估算：每 GB 约 1 分钟
	gb := dataSize / (1024 * 1024 * 1024)
	if gb < 10 {
		return "1-2 小时"
	} else if gb < 100 {
		return "2-4 小时"
	} else if gb < 500 {
		return "4-12 小时"
	}
	return "12+ 小时"
}
