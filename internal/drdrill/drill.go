package drdrill

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─────────────────────── 执行接口（可 mock） ───────────────────────

// Snapshotter 快照接口，用于创建/恢复保护点.
type Snapshotter interface {
	CreateSnapshot(ctx context.Context, label string) (snapshotID string, err error)
	RestoreSnapshot(ctx context.Context, snapshotID string) error
}

// StepExecutor 步骤执行接口.
type StepExecutor interface {
	Execute(ctx context.Context, plan *DrillPlan, step StepDef) error
	Rollback(ctx context.Context, plan *DrillPlan, step StepDef) error
}

// ─────────────────────── Manager ───────────────────────

// Manager 容灾演练管理器.
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	plans       map[string]*DrillPlan
	executions  map[string]*DrillExecution
	snapshotter Snapshotter
	executor    StepExecutor
}

// NewManager 创建演练管理器.
func NewManager(logger *zap.Logger, snap Snapshotter, exec StepExecutor) *Manager {
	return &Manager{
		logger:      logger,
		plans:       make(map[string]*DrillPlan),
		executions:  make(map[string]*DrillExecution),
		snapshotter: snap,
		executor:    exec,
	}
}

// ─────────────────────── 计划 CRUD ───────────────────────

// CreatePlan 创建演练计划.
func (m *Manager) CreatePlan(req CreatePlanRequest) (*DrillPlan, error) {
	if err := validateType(req.Type); err != nil {
		return nil, err
	}
	if err := validateScope(req.Scope); err != nil {
		return nil, err
	}
	if err := validateMode(req.Mode); err != nil {
		return nil, err
	}

	now := time.Now()
	plan := &DrillPlan{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        DrillType(req.Type),
		Scope:       DrillScope(req.Scope),
		ScopeTarget: req.ScopeTarget,
		Mode:        DrillMode(req.Mode),
		Steps:       req.Steps,
		Schedule:    req.Schedule,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.mu.Lock()
	m.plans[plan.ID] = plan
	m.mu.Unlock()

	m.logger.Info("创建演练计划",
		zap.String("id", plan.ID),
		zap.String("name", plan.Name),
		zap.String("type", string(plan.Type)),
	)
	return plan, nil
}

// ListPlans 列出所有演练计划.
func (m *Manager) ListPlans() []*DrillPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DrillPlan, 0, len(m.plans))
	for _, p := range m.plans {
		result = append(result, p)
	}
	return result
}

// GetPlan 获取演练计划详情.
func (m *Manager) GetPlan(id string) (*DrillPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", id)
	}
	return plan, nil
}

// ─────────────────────── 执行引擎 ───────────────────────

// ExecutePlan 执行演练计划.
func (m *Manager) ExecutePlan(ctx context.Context, planID string) (*DrillExecution, error) {
	m.mu.RLock()
	plan, ok := m.plans[planID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	exec := &DrillExecution{
		ID:        uuid.New().String(),
		PlanID:    plan.ID,
		PlanName:  plan.Name,
		Mode:      plan.Mode,
		Status:    ExecRunning,
		StartTime: time.Now(),
		StepResults: make([]StepResult, len(plan.Steps)),
	}

	for i, step := range plan.Steps {
		exec.StepResults[i] = StepResult{
			Name:   step.Name,
			Status: StepPending,
		}
	}

	m.mu.Lock()
	m.executions[exec.ID] = exec
	m.mu.Unlock()

	// 异步执行
	go m.runExecution(ctx, exec, plan)

	return exec, nil
}

func (m *Manager) runExecution(ctx context.Context, exec *DrillExecution, plan *DrillPlan) {
	m.logger.Info("演练开始执行",
		zap.String("execution_id", exec.ID),
		zap.String("plan_id", plan.ID),
		zap.String("mode", string(plan.Mode)),
	)

	// 1. 执行前自动快照（保护点）
	if m.snapshotter != nil {
		snapID, err := m.snapshotter.CreateSnapshot(ctx, fmt.Sprintf("drdrill-protection-%s", exec.ID))
		if err != nil {
			m.logger.Error("创建保护点快照失败", zap.Error(err))
			exec.Status = ExecFailed
			exec.ErrorMessage = fmt.Sprintf("创建保护点失败: %v", err)
			exec.EndTime = time.Now()
			exec.TotalDuration = exec.EndTime.Sub(exec.StartTime)
			return
		}
		exec.SnapshotID = snapID
		m.logger.Info("保护点快照已创建", zap.String("snapshot_id", snapID))
	}

	// 2. 步骤化执行
	failed := false
	for i, step := range plan.Steps {
		if ctx.Err() != nil {
			exec.StepResults[i].Status = StepSkipped
			continue
		}

		m.executeStep(ctx, exec, plan, i, step)

		if exec.StepResults[i].Status == StepFailed {
			failed = true
			// 失败步骤自动回滚
			m.rollbackStep(ctx, exec, plan, i, step)
			break
		}
	}

	exec.EndTime = time.Now()
	exec.TotalDuration = exec.EndTime.Sub(exec.StartTime)

	if failed {
		exec.Status = ExecFailed
	} else if ctx.Err() != nil {
		exec.Status = ExecAborted
	} else {
		exec.Status = ExecSuccess
	}

	// 3. 执行后自动恢复到保护点
	if m.snapshotter != nil && exec.SnapshotID != "" {
		if err := m.snapshotter.RestoreSnapshot(ctx, exec.SnapshotID); err != nil {
			m.logger.Error("恢复保护点失败", zap.String("snapshot_id", exec.SnapshotID), zap.Error(err))
		} else {
			m.logger.Info("已恢复到保护点", zap.String("snapshot_id", exec.SnapshotID))
		}
	}

	m.logger.Info("演练执行完成",
		zap.String("execution_id", exec.ID),
		zap.String("status", string(exec.Status)),
		zap.Duration("duration", exec.TotalDuration),
	)
}

func (m *Manager) executeStep(ctx context.Context, exec *DrillExecution, plan *DrillPlan, idx int, step StepDef) {
	result := &exec.StepResults[idx]
	result.Status = StepRunning
	result.StartTime = time.Now()

	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	var lastErr error
	maxRetries := step.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			result.Retried = attempt
			m.logger.Info("步骤重试",
				zap.String("step", step.Name),
				zap.Int("attempt", attempt),
			)
		}

		if err := m.executor.Execute(stepCtx, plan, step); err != nil {
			lastErr = err
			if stepCtx.Err() != nil {
				break
			}
			continue
		}

		lastErr = nil
		break
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if lastErr != nil {
		result.Status = StepFailed
		result.Error = lastErr.Error()
	} else {
		result.Status = StepSuccess
	}
}

func (m *Manager) rollbackStep(ctx context.Context, exec *DrillExecution, plan *DrillPlan, idx int, step StepDef) {
	if step.Rollback == "" {
		return
	}

	result := &exec.StepResults[idx]
	m.logger.Info("回滚步骤", zap.String("step", step.Name))

	if err := m.executor.Rollback(ctx, plan, step); err != nil {
		m.logger.Error("步骤回滚失败", zap.String("step", step.Name), zap.Error(err))
	} else {
		result.RolledBack = true
		result.Status = StepRolledBack
	}
}

// ─────────────────────── 查询 ───────────────────────

// ListExecutions 列出所有执行记录.
func (m *Manager) ListExecutions() []*DrillExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DrillExecution, 0, len(m.executions))
	for _, e := range m.executions {
		result = append(result, e)
	}
	return result
}

// GetExecution 获取执行详情.
func (m *Manager) GetExecution(id string) (*DrillExecution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", id)
	}
	return exec, nil
}

// GetReport 获取演练报告.
func (m *Manager) GetReport(execID string) (*DrillReport, error) {
	exec, err := m.GetExecution(execID)
	if err != nil {
		return nil, err
	}

	report := &DrillReport{
		ExecutionID:   exec.ID,
		PlanID:        exec.PlanID,
		PlanName:      exec.PlanName,
		Mode:          exec.Mode,
		Status:        exec.Status,
		StartTime:     exec.StartTime,
		EndTime:       exec.EndTime,
		TotalDuration: exec.TotalDuration,
		StepResults:   exec.StepResults,
		RTOActual:     exec.TotalDuration,
		RPOActual:     m.calculateRPO(exec),
		Issues:        m.analyzeIssues(exec),
		Suggestions:   m.generateSuggestions(exec),
		Trend:         m.calculateTrend(exec.PlanID),
	}

	return report, nil
}

// GetMetrics 获取 RTO/RPO 指标统计.
func (m *Manager) GetMetrics() *DRMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &DRMetrics{
		TotalPlans: len(m.plans),
		TotalExecs: len(m.executions),
	}

	if len(m.executions) == 0 {
		return metrics
	}

	var totalRTO, totalRPO time.Duration
	var bestRTO, worstRTO, bestRPO, worstRPO time.Duration
	var successCount int
	var lastDrill time.Time

	first := true
	for _, exec := range m.executions {
		rto := exec.TotalDuration
		rpo := m.calculateRPO(exec)

		totalRTO += rto
		totalRPO += rpo

		if exec.Status == ExecSuccess {
			successCount++
		}

		if first {
			bestRTO, worstRTO = rto, rto
			bestRPO, worstRPO = rpo, rpo
			first = false
		} else {
			if rto < bestRTO {
				bestRTO = rto
			}
			if rto > worstRTO {
				worstRTO = rto
			}
			if rpo < bestRPO {
				bestRPO = rpo
			}
			if rpo > worstRPO {
				worstRPO = rpo
			}
		}

		if exec.EndTime.After(lastDrill) {
			lastDrill = exec.EndTime
		}
	}

	n := time.Duration(len(m.executions))
	metrics.SuccessRate = float64(successCount) / float64(len(m.executions)) * 100
	metrics.AvgRTO = totalRTO / n
	metrics.AvgRPO = totalRPO / n
	metrics.BestRTO = bestRTO
	metrics.WorstRTO = worstRTO
	metrics.BestRPO = bestRPO
	metrics.WorstRPO = worstRPO
	metrics.LastDrillTime = lastDrill

	return metrics
}

// ─────────────────────── 辅助方法 ───────────────────────

func (m *Manager) calculateRPO(exec *DrillExecution) time.Duration {
	// 简化计算：如果有快照，RPO ≈ 快照到执行开始的时间差
	if exec.SnapshotID != "" && !exec.StartTime.IsZero() {
		return time.Since(exec.StartTime) // 保护点到现在的差（简化）
	}
	return exec.TotalDuration // 无快照时取执行时长作为估算
}

func (m *Manager) analyzeIssues(exec *DrillExecution) []string {
	var issues []string
	for _, step := range exec.StepResults {
		if step.Status == StepFailed {
			issues = append(issues, fmt.Sprintf("步骤 [%s] 执行失败: %s", step.Name, step.Error))
		}
		if step.Retried > 0 {
			issues = append(issues, fmt.Sprintf("步骤 [%s] 重试 %d 次后才成功", step.Name, step.Retried))
		}
		if step.RolledBack {
			issues = append(issues, fmt.Sprintf("步骤 [%s] 触发了回滚", step.Name))
		}
	}
	if exec.Status == ExecFailed {
		issues = append(issues, "演练整体执行失败，容灾能力可能存在风险")
	}
	return issues
}

func (m *Manager) generateSuggestions(exec *DrillExecution) []string {
	var suggestions []string
	if exec.Status == ExecFailed {
		suggestions = append(suggestions, "建议检查失败步骤的前置依赖和环境配置")
		suggestions = append(suggestions, "建议增加失败步骤的重试次数或超时时间")
	}
	retried := 0
	for _, step := range exec.StepResults {
		retried += step.Retried
	}
	if retried > 0 {
		suggestions = append(suggestions, "存在步骤需要重试，建议优化执行脚本稳定性")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "演练执行顺利，建议保持当前演练节奏")
	}
	return suggestions
}

func (m *Manager) calculateTrend(planID string) *TrendData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var execs []*DrillExecution
	for _, e := range m.executions {
		if e.PlanID == planID {
			execs = append(execs, e)
		}
	}

	if len(execs) == 0 {
		return nil
	}

	trend := &TrendData{
		TotalDrills: len(execs),
	}

	var totalRTO, totalRPO time.Duration
	var successCount int
	first := true

	for _, e := range execs {
		rto := e.TotalDuration
		rpo := m.calculateRPO(e)
		totalRTO += rto
		totalRPO += rpo

		if e.Status == ExecSuccess {
			successCount++
		}

		if first {
			trend.BestRTO, trend.WorstRTO = rto, rto
			trend.BestRPO, trend.WorstRPO = rpo, rpo
			first = false
		} else {
			if rto < trend.BestRTO {
				trend.BestRTO = rto
			}
			if rto > trend.WorstRTO {
				trend.WorstRTO = rto
			}
			if rpo < trend.BestRPO {
				trend.BestRPO = rpo
			}
			if rpo > trend.WorstRPO {
				trend.WorstRPO = rpo
			}
		}
	}

	n := time.Duration(len(execs))
	trend.SuccessRate = float64(successCount) / float64(len(execs)) * 100
	trend.AvgRTO = totalRTO / n
	trend.AvgRPO = totalRPO / n

	// 计算改善率（最近一次 vs 第一次）
	if len(execs) >= 2 {
		firstRTO := execs[0].TotalDuration
		lastRTO := execs[len(execs)-1].TotalDuration
		if firstRTO > 0 {
			trend.ImprovementRate = float64(firstRTO-lastRTO) / float64(firstRTO) * 100
		}
	}

	return trend
}

// ─────────────────────── 校验 ───────────────────────

func validateType(t string) error {
	switch DrillType(t) {
	case DrillTypeDiskFault, DrillTypeNetworkDown, DrillTypePoolDegrade,
		DrillTypeServiceDown, DrillTypeDataRecovery:
		return nil
	default:
		return fmt.Errorf("invalid drill type: %s", t)
	}
}

func validateScope(s string) error {
	switch DrillScope(s) {
	case ScopeSystem, ScopePool, ScopeService:
		return nil
	default:
		return fmt.Errorf("invalid drill scope: %s", s)
	}
}

func validateMode(m string) error {
	switch DrillMode(m) {
	case ModeDryRun, ModeReal:
		return nil
	default:
		return fmt.Errorf("invalid drill mode: %s", m)
	}
}
