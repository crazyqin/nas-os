package opsrunbook

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Executor 步骤执行引擎.
type Executor struct {
	mu            sync.Mutex
	manager       *Manager
	logger        *zap.Logger
	commandFunc   CommandFunc
	scriptFunc    ScriptFunc
	checkFunc     CheckFunc
	notifyFunc    NotifyFunc
	maxConcurrent int
	semaphore     chan struct{}
}

// CommandFunc 命令执行函数.
type CommandFunc func(ctx context.Context, cmd string, vars map[string]string) (string, error)

// ScriptFunc 脚本执行函数.
type ScriptFunc func(ctx context.Context, script string, vars map[string]string) (string, error)

// CheckFunc 健康检查函数.
type CheckFunc func(ctx context.Context, check string, vars map[string]string) (bool, string, error)

// NotifyFunc 通知函数.
type NotifyFunc func(ctx context.Context, message string, severity Severity) error

// ExecutorConfig 执行器配置.
type ExecutorConfig struct {
	MaxConcurrent  int           `json:"max_concurrent"`
	CommandTimeout time.Duration `json:"command_timeout"`
}

// NewExecutor 创建执行器.
func NewExecutor(manager *Manager, logger *zap.Logger, config ExecutorConfig) *Executor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 5
	}
	if config.CommandTimeout <= 0 {
		config.CommandTimeout = 5 * time.Minute
	}

	e := &Executor{
		manager:       manager,
		logger:        logger,
		maxConcurrent: config.MaxConcurrent,
		semaphore:     make(chan struct{}, config.MaxConcurrent),
	}

	// 设置默认执行函数
	e.commandFunc = e.defaultCommandFunc
	e.scriptFunc = e.defaultScriptFunc
	e.checkFunc = e.defaultCheckFunc
	e.notifyFunc = e.defaultNotifyFunc

	return e
}

// SetCommandFunc 设置命令执行函数.
func (e *Executor) SetCommandFunc(fn CommandFunc) {
	e.commandFunc = fn
}

// SetScriptFunc 设置脚本执行函数.
func (e *Executor) SetScriptFunc(fn ScriptFunc) {
	e.scriptFunc = fn
}

// SetCheckFunc 设置健康检查函数.
func (e *Executor) SetCheckFunc(fn CheckFunc) {
	e.checkFunc = fn
}

// SetNotifyFunc 设置通知函数.
func (e *Executor) SetNotifyFunc(fn NotifyFunc) {
	e.notifyFunc = fn
}

// Execute 执行运维手册.
func (e *Executor) Execute(ctx context.Context, runbookID string, trigger TriggerType, triggerRef string, vars map[string]string, operator string) (*Execution, error) {
	e.manager.mu.Lock()
	rb, ok := e.manager.runbooks[runbookID]
	if !ok {
		e.manager.mu.Unlock()
		return nil, fmt.Errorf("runbook %s not found", runbookID)
	}

	if rb.Status != StatusActive {
		e.manager.mu.Unlock()
		return nil, fmt.Errorf("runbook %s is not active (status: %s)", runbookID, rb.Status)
	}
	e.manager.mu.Unlock()

	// 并发控制
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("execution cancelled: %w", ctx.Err())
	}

	execID := fmt.Sprintf("exec_%s_%d", runbookID, time.Now().UnixNano())
	execution := &Execution{
		ID:          execID,
		RunbookID:   runbookID,
		RunbookName: rb.Name,
		Status:      StepRunning,
		Trigger:     trigger,
		TriggerRef:  triggerRef,
		Variables:   vars,
		Steps:       make([]*StepResult, 0, len(rb.Steps)),
		StartedAt:   time.Now(),
		Operator:    operator,
	}

	// 保存执行记录
	e.manager.mu.Lock()
	e.manager.executions[execID] = execution
	e.manager.mu.Unlock()

	e.emitEvent("started", execID, "", StepRunning, fmt.Sprintf("开始执行运维手册: %s", rb.Name))

	// 设置超时
	if rb.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rb.Timeout)
		defer cancel()
	}

	// 执行步骤
	success := true
	for _, step := range rb.Steps {
		result := e.executeStep(ctx, step, execution.Variables)
		execution.Steps = append(execution.Steps, result)

		if result.Status == StepFailed {
			success = false
			e.logger.Error("step failed",
				zap.String("execution", execID),
				zap.String("step", step.Name),
				zap.String("error", result.Error),
			)

			// 判断是否需要回滚
			shouldRollback := rb.RollbackOn == "always" ||
				(rb.RollbackOn == "failure" && !success)

			if shouldRollback {
				e.rollback(ctx, execution, rb)
			}

			if step.ContinueOn != "failure" && step.ContinueOn != "always" {
				break
			}
		}
	}

	// 更新执行结果
	now := time.Now()
	execution.FinishedAt = &now
	execution.Duration = now.Sub(execution.StartedAt)

	if success {
		execution.Status = StepSuccess
		e.emitEvent("completed", execID, "", StepSuccess, "运维手册执行成功")
	} else {
		execution.Status = StepFailed
		execution.Error = "一个或多个步骤执行失败"
		e.emitEvent("failed", execID, "", StepFailed, execution.Error)
	}

	// 更新统计
	e.manager.mu.Lock()
	if stats, ok := e.manager.execStats[runbookID]; ok {
		stats.TotalRuns++
		if success {
			stats.SuccessRuns++
		} else {
			stats.FailedRuns++
		}
		if execution.Rollbacked {
			stats.RollbackRuns++
		}
		stats.LastRunAt = &now
		stats.SuccessRate = float64(stats.SuccessRuns) / float64(stats.TotalRuns)
		if stats.AvgDuration == 0 {
			stats.AvgDuration = execution.Duration
		} else {
			stats.AvgDuration = (stats.AvgDuration + execution.Duration) / 2
		}
	}
	rb.RunCount++
	if rb.RunCount > 0 {
		stats := e.manager.execStats[runbookID]
		if stats != nil {
			rb.SuccessRate = stats.SuccessRate
		}
	}
	e.manager.mu.Unlock()

	// 持久化
	if e.manager.store != nil {
		if err := e.manager.store.SaveExecution(execution); err != nil {
			e.logger.Error("failed to save execution", zap.String("id", execID), zap.Error(err))
		}
	}

	return execution, nil
}

// executeStep 执行单个步骤.
func (e *Executor) executeStep(ctx context.Context, step *Step, vars map[string]string) *StepResult {
	result := &StepResult{
		StepID:    step.ID,
		StepName:  step.Name,
		Status:    StepRunning,
		StartedAt: time.Now(),
	}

	e.emitEvent("step_started", "", step.ID, StepRunning, fmt.Sprintf("执行步骤: %s", step.Name))

	// 合并变量
	stepVars := make(map[string]string)
	for k, v := range vars {
		stepVars[k] = v
	}
	for k, v := range step.Variables {
		stepVars[k] = v
	}

	// 检查依赖
	if len(step.DependsOn) > 0 {
		// 依赖检查在调用方已通过步骤顺序保证
	}

	// 设置步骤超时
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// 执行（支持重试）
	var lastErr error
	maxRetries := step.RetryCount
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			result.Retries = retry
			if step.RetryDelay > 0 {
				select {
				case <-time.After(step.RetryDelay):
				case <-stepCtx.Done():
					result.Status = StepFailed
					result.Error = "step cancelled during retry wait"
					result.Duration = time.Since(result.StartedAt)
					return result
				}
			}
		}

		var output string
		var err error

		switch step.Type {
		case StepTypeCommand:
			output, err = e.executeCommand(stepCtx, step.Command, stepVars)
		case StepTypeScript:
			output, err = e.executeScript(stepCtx, step.Script, stepVars)
		case StepTypeCheck:
			ok, checkOutput, checkErr := e.executeCheck(stepCtx, step.Command, stepVars)
			if checkErr != nil {
				err = checkErr
			} else if !ok {
				err = fmt.Errorf("check failed: %s", checkOutput)
			}
			output = checkOutput
		case StepTypeWait:
			output, err = e.executeWait(stepCtx, step.Command, stepVars)
		case StepTypeApproval:
			output, err = e.executeApproval(stepCtx, step, stepVars)
		case StepTypeNotify:
			err = e.notifyFunc(stepCtx, step.Command, Severity(stepVars["severity"]))
			output = "notification sent"
		case StepTypeCondition:
			output, err = e.executeCondition(stepCtx, step.Condition, stepVars)
		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err == nil {
			result.Output = output
			result.Status = StepSuccess
			result.Duration = time.Since(result.StartedAt)
			e.emitEvent("step_completed", "", step.ID, StepSuccess, fmt.Sprintf("步骤完成: %s", step.Name))
			return result
		}

		lastErr = err
	}

	result.Status = StepFailed
	result.Error = lastErr.Error()
	result.Duration = time.Since(result.StartedAt)
	return result
}

// rollback 回滚已执行的步骤.
func (e *Executor) rollback(ctx context.Context, exec *Execution, rb *Runbook) {
	e.logger.Info("starting rollback", zap.String("execution", exec.ID))
	exec.Rollbacked = true
	e.emitEvent("rollback", exec.ID, "", StepRollback, "开始回滚操作")

	// 逆序执行回滚步骤
	for i := len(exec.Steps) - 1; i >= 0; i-- {
		stepResult := exec.Steps[i]
		if stepResult.Status != StepSuccess {
			continue
		}

		// 查找对应的回滚步骤
		for _, step := range rb.Steps {
			if step.ID == stepResult.StepID && step.Rollback != nil {
				e.logger.Info("rolling back step", zap.String("step", step.Name))
				rollbackResult := e.executeStep(ctx, step.Rollback, exec.Variables)
				exec.Steps = append(exec.Steps, &StepResult{
					StepID:    step.Rollback.ID,
					StepName:  fmt.Sprintf("[回滚] %s", step.Rollback.Name),
					Status:    rollbackResult.Status,
					Output:    rollbackResult.Output,
					Error:     rollbackResult.Error,
					StartedAt: rollbackResult.StartedAt,
					Duration:  rollbackResult.Duration,
				})
			}
		}
	}
}

// executeCommand 执行命令.
func (e *Executor) executeCommand(ctx context.Context, cmd string, vars map[string]string) (string, error) {
	if e.commandFunc != nil {
		return e.commandFunc(ctx, cmd, vars)
	}
	return e.defaultCommandFunc(ctx, cmd, vars)
}

// defaultCommandFunc 默认命令执行.
func (e *Executor) defaultCommandFunc(ctx context.Context, cmd string, vars map[string]string) (string, error) {
	// 替换变量
	for k, v := range vars {
		cmd = strings.ReplaceAll(cmd, fmt.Sprintf("${%s}", k), v)
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// executeScript 执行脚本.
func (e *Executor) executeScript(ctx context.Context, script string, vars map[string]string) (string, error) {
	if e.scriptFunc != nil {
		return e.scriptFunc(ctx, script, vars)
	}
	return e.defaultScriptFunc(ctx, script, vars)
}

// defaultScriptFunc 默认脚本执行.
func (e *Executor) defaultScriptFunc(ctx context.Context, script string, vars map[string]string) (string, error) {
	// 替换变量
	for k, v := range vars {
		script = strings.ReplaceAll(script, fmt.Sprintf("${%s}", k), v)
	}

	c := exec.CommandContext(ctx, "sh", "-c", script)
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("script failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// executeCheck 执行健康检查.
func (e *Executor) executeCheck(ctx context.Context, check string, vars map[string]string) (bool, string, error) {
	if e.checkFunc != nil {
		return e.checkFunc(ctx, check, vars)
	}
	return e.defaultCheckFunc(ctx, check, vars)
}

// defaultCheckFunc 默认健康检查.
func (e *Executor) defaultCheckFunc(ctx context.Context, check string, vars map[string]string) (bool, string, error) {
	for k, v := range vars {
		check = strings.ReplaceAll(check, fmt.Sprintf("${%s}", k), v)
	}

	parts := strings.Fields(check)
	if len(parts) == 0 {
		return false, "", fmt.Errorf("empty check command")
	}

	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := c.CombinedOutput()
	if err != nil {
		return false, string(output), nil
	}
	return true, string(output), nil
}

// defaultNotifyFunc 默认通知函数.
func (e *Executor) defaultNotifyFunc(_ context.Context, message string, _ Severity) error {
	e.logger.Info("notification", zap.String("message", message))
	return nil
}

// executeWait 执行等待.
func (e *Executor) executeWait(ctx context.Context, condition string, vars map[string]string) (string, error) {
	for k, v := range vars {
		condition = strings.ReplaceAll(condition, fmt.Sprintf("${%s}", k), v)
	}

	// 格式: duration:5s 或 until:command
	parts := strings.SplitN(condition, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid wait condition: %s", condition)
	}

	switch parts[0] {
	case "duration":
		d, err := time.ParseDuration(parts[1])
		if err != nil {
			return "", fmt.Errorf("invalid duration: %w", err)
		}
		select {
		case <-time.After(d):
			return fmt.Sprintf("waited %s", d), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	case "until":
		// 轮询检查直到成功
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		timeout := 5 * time.Minute
		deadline := time.Now().Add(timeout)

		for {
			select {
			case <-ticker.C:
				if time.Now().After(deadline) {
					return "", fmt.Errorf("wait timeout after %s", timeout)
				}
				ok, output, err := e.defaultCheckFunc(ctx, parts[1], vars)
				if err != nil {
					continue
				}
				if ok {
					return fmt.Sprintf("condition met: %s", output), nil
				}
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	default:
		return "", fmt.Errorf("unknown wait type: %s", parts[0])
	}
}

// executeApproval 执行审批.
func (e *Executor) executeApproval(ctx context.Context, step *Step, vars map[string]string) (string, error) {
	if step.AutoApprove {
		return "auto-approved", nil
	}

	approvalID := fmt.Sprintf("approval_%s_%d", step.ID, time.Now().UnixNano())
	req := &ApprovalRequest{
		ID:          approvalID,
		StepID:      step.ID,
		StepName:    step.Name,
		Description: step.Description,
		RequestedAt: time.Now(),
	}

	e.manager.mu.Lock()
	e.manager.approvals[approvalID] = req
	e.manager.mu.Unlock()

	e.emitEvent("approval_requested", "", step.ID, StepWaiting, fmt.Sprintf("需要审批: %s", step.Name))

	// 等待审批
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.manager.mu.RLock()
			updated := e.manager.approvals[approvalID]
			e.manager.mu.RUnlock()

			if updated.ApprovedAt != nil {
				if updated.Rejected {
					return "", fmt.Errorf("approval rejected by %s: %s", updated.ApprovedBy, updated.Reason)
				}
				return fmt.Sprintf("approved by %s", updated.ApprovedBy), nil
			}
		case <-ctx.Done():
			return "", fmt.Errorf("approval timeout: %w", ctx.Err())
		}
	}
}

// executeCondition 执行条件分支.
func (e *Executor) executeCondition(ctx context.Context, condition string, vars map[string]string) (string, error) {
	for k, v := range vars {
		condition = strings.ReplaceAll(condition, fmt.Sprintf("${%s}", k), v)
	}

	// 简单条件求值: check_command && echo "true" || echo "false"
	c := exec.CommandContext(ctx, "sh", "-c", condition)
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("condition evaluation failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// emitEvent 发送执行事件.
func (e *Executor) emitEvent(eventType, execID, stepID string, status StepStatus, message string) {
	event := &ExecutionEvent{
		Type:        eventType,
		ExecutionID: execID,
		StepID:      stepID,
		Status:      status,
		Message:     message,
		Timestamp:   time.Now(),
	}

	select {
	case e.manager.eventChan <- event:
	default:
		e.logger.Warn("event channel full, dropping event", zap.String("type", eventType))
	}
}
