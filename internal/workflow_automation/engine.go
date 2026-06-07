// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Engine 工作流引擎.
type Engine struct {
	mu          sync.RWMutex
	workflows   map[string]*Workflow
	triggers    map[string]*Trigger
	handlers    map[ActionType]ActionHandler
	store       Store
	logger      *zap.Logger
	execLogger  ExecutionLogger
	condEval    *ConditionEvaluator
	triggerMgr  *TriggerManager
	executions  map[string]*Execution
	stopChan    chan struct{}
	running     bool
}

// NewEngine 创建工作流引擎.
func NewEngine(store Store, logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	execLogger := NewMemoryExecutionLogger(1000)
	condEval := NewConditionEvaluator()

	e := &Engine{
		workflows:  make(map[string]*Workflow),
		triggers:   make(map[string]*Trigger),
		handlers:   make(map[ActionType]ActionHandler),
		store:      store,
		logger:     logger,
		execLogger: execLogger,
		condEval:   condEval,
		executions: make(map[string]*Execution),
		stopChan:   make(chan struct{}),
	}

	e.triggerMgr = NewTriggerManager(e, logger)

	// 注册内置动作处理器
	e.registerBuiltinHandlers()

	return e
}

// registerBuiltinHandlers 注册内置动作处理器.
func (e *Engine) registerBuiltinHandlers() {
	e.handlers[ActionFileOps] = &FileOpsHandler{}
	e.handlers[ActionNotification] = &NotificationHandler{}
	e.handlers[ActionAPICall] = &APICallHandler{}
	e.handlers[ActionContainer] = &ContainerHandler{}
	e.handlers[ActionShell] = &ShellHandler{}
	e.handlers[ActionTransform] = &TransformHandler{}
}

// Start 启动引擎.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopChan = make(chan struct{})
	e.mu.Unlock()

	e.triggerMgr.Start()
	e.logger.Info("workflow engine started")
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopChan)
	e.triggerMgr.Stop()
	e.logger.Info("workflow engine stopped")
}

// IsRunning 检查引擎是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RegisterHandler 注册自定义动作处理器.
func (e *Engine) RegisterHandler(handler ActionHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[handler.Type()] = handler
	e.logger.Info("registered action handler",
		zap.String("type", string(handler.Type())),
		zap.String("name", handler.Name()),
	)
}

// GetHandler 获取动作处理器.
func (e *Engine) GetHandler(actionType ActionType) (ActionHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h, ok := e.handlers[actionType]
	return h, ok
}

// ========== 工作流 CRUD ==========

// CreateWorkflow 创建工作流.
func (e *Engine) CreateWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	wf.Version = 1
	wf.Status = StatusDraft
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()

	if wf.Nodes == nil {
		wf.Nodes = make(map[string]*Node)
	}
	if wf.Edges == nil {
		wf.Edges = make([]*Edge, 0)
	}

	// 添加默认 Start 和 End 节点
	wf.Nodes["start"] = &Node{
		ID:       "start",
		Type:     NodeTypeStart,
		Name:     "Start",
		Enabled:  true,
		Position: &Position{X: 100, Y: 200},
	}
	wf.Nodes["end"] = &Node{
		ID:       "end",
		Type:     NodeTypeEnd,
		Name:     "End",
		Enabled:  true,
		Position: &Position{X: 700, Y: 200},
	}

	e.workflows[wf.ID] = wf

	if e.store != nil {
		if err := e.store.SaveWorkflow(wf); err != nil {
			e.logger.Error("failed to save workflow", zap.Error(err))
			return fmt.Errorf("save workflow: %w", err)
		}
	}

	e.logger.Info("workflow created",
		zap.String("id", wf.ID),
		zap.String("name", wf.Name),
	)
	return nil
}

// GetWorkflow 获取工作流.
func (e *Engine) GetWorkflow(id string) (*Workflow, error) {
	e.mu.RLock()
	wf, ok := e.workflows[id]
	e.mu.RUnlock()

	if !ok {
		if e.store != nil {
			stored, err := e.store.GetWorkflow(id)
			if err == nil && stored != nil {
				e.mu.Lock()
				e.workflows[id] = stored
				e.mu.Unlock()
				return stored, nil
			}
		}
		return nil, ErrWorkflowNotFound
	}
	return wf, nil
}

// ListWorkflows 列出所有工作流.
func (e *Engine) ListWorkflows() ([]*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wfs := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		wfs = append(wfs, wf)
	}
	return wfs, nil
}

// UpdateWorkflow 更新工作流.
func (e *Engine) UpdateWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.workflows[wf.ID]
	if !ok {
		return ErrWorkflowNotFound
	}

	wf.Version = existing.Version + 1
	wf.UpdatedAt = time.Now()
	wf.CreatedAt = existing.CreatedAt
	wf.CreatedBy = existing.CreatedBy

	// 保存版本快照
	if e.store != nil {
		version := &WorkflowVersion{
			WorkflowID: wf.ID,
			Version:    wf.Version,
			Snapshot:   existing,
			Comment:    fmt.Sprintf("auto-save before update to v%d", wf.Version),
			CreatedAt:  time.Now(),
		}
		if err := e.store.SaveVersion(version); err != nil {
			e.logger.Error("failed to save version snapshot", zap.Error(err))
		}
	}

	e.workflows[wf.ID] = wf

	if e.store != nil {
		if err := e.store.SaveWorkflow(wf); err != nil {
			e.logger.Error("failed to update workflow", zap.Error(err))
			return fmt.Errorf("update workflow: %w", err)
		}
	}

	e.logger.Info("workflow updated",
		zap.String("id", wf.ID),
		zap.Int("version", wf.Version),
	)
	return nil
}

// DeleteWorkflow 删除工作流.
func (e *Engine) DeleteWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.workflows[id]; !ok {
		return ErrWorkflowNotFound
	}

	// 先停用相关触发器
	e.triggerMgr.DisableTriggersByWorkflow(id)

	delete(e.workflows, id)

	if e.store != nil {
		if err := e.store.DeleteWorkflow(id); err != nil {
			e.logger.Error("failed to delete workflow from store", zap.Error(err))
			return fmt.Errorf("delete workflow: %w", err)
		}
	}

	e.logger.Info("workflow deleted", zap.String("id", id))
	return nil
}

// ========== 节点和边操作 ==========

// AddNode 添加节点.
func (e *Engine) AddNode(workflowID string, node *Node) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.Enabled == false && node.Type != NodeTypeStart && node.Type != NodeTypeEnd {
		node.Enabled = true
	}

	wf.Nodes[node.ID] = node
	wf.UpdatedAt = time.Now()
	return nil
}

// RemoveNode 移除节点.
func (e *Engine) RemoveNode(workflowID, nodeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	if _, ok := wf.Nodes[nodeID]; !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}

	// 不允许删除 start/end 节点
	if nodeID == "start" || nodeID == "end" {
		return fmt.Errorf("cannot delete %s node", nodeID)
	}

	// 移除相关边
	edges := make([]*Edge, 0, len(wf.Edges))
	for _, edge := range wf.Edges {
		if edge.From != nodeID && edge.To != nodeID {
			edges = append(edges, edge)
		}
	}
	wf.Edges = edges

	delete(wf.Nodes, nodeID)
	wf.UpdatedAt = time.Now()
	return nil
}

// AddEdge 添加边.
func (e *Engine) AddEdge(workflowID string, edge *Edge) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	if _, ok := wf.Nodes[edge.From]; !ok {
		return fmt.Errorf("source node %q not found", edge.From)
	}
	if _, ok := wf.Nodes[edge.To]; !ok {
		return fmt.Errorf("target node %q not found", edge.To)
	}

	if edge.ID == "" {
		edge.ID = uuid.New().String()
	}

	wf.Edges = append(wf.Edges, edge)
	wf.UpdatedAt = time.Now()
	return nil
}

// RemoveEdge 移除边.
func (e *Engine) RemoveEdge(workflowID, edgeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, ok := e.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	for i, edge := range wf.Edges {
		if edge.ID == edgeID {
			wf.Edges = append(wf.Edges[:i], wf.Edges[i+1:]...)
			wf.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("edge %q not found", edgeID)
}

// ========== 工作流执行 ==========

// Execute 执行工作流.
func (e *Engine) Execute(ctx context.Context, workflowID string, trigger *TriggerEvent) (*Execution, error) {
	wf, err := e.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}

	if wf.Status != StatusActive {
		return nil, fmt.Errorf("workflow %q is not active (status: %s)", workflowID, wf.Status)
	}

	exec := &Execution{
		ID:         uuid.New().String(),
		WorkflowID: wf.ID,
		Version:    wf.Version,
		Status:     ExecRunning,
		Context:    make(map[string]interface{}),
		Steps:      make([]*StepExecution, 0),
		StartedAt:  time.Now(),
	}

	if trigger != nil {
		exec.TriggerID = trigger.TriggerID
		exec.TriggeredBy = string(trigger.Type)
		if trigger.Payload != nil {
			for k, v := range trigger.Payload {
				exec.Context[k] = v
			}
		}
	}

	// 合并工作流变量
	if wf.Variables != nil {
		for k, v := range wf.Variables {
			if _, exists := exec.Context[k]; !exists {
				exec.Context[k] = v
			}
		}
	}

	e.mu.Lock()
	e.executions[exec.ID] = exec
	e.mu.Unlock()

	e.execLogger.LogStart(exec.ID, wf.ID)
	e.logger.Info("execution started",
		zap.String("execution_id", exec.ID),
		zap.String("workflow_id", wf.ID),
	)

	// 异步执行
	go e.runExecution(ctx, wf, exec)

	return exec, nil
}

// runExecution 运行工作流执行.
func (e *Engine) runExecution(ctx context.Context, wf *Workflow, exec *Execution) {
	startNode := wf.Nodes["start"]
	if startNode == nil {
		e.failExecution(exec, fmt.Errorf("start node not found"))
		return
	}

	// 从 start 节点开始，按边遍历执行
	err := e.executeFromNode(ctx, wf, exec, "start")
	if err != nil {
		e.failExecution(exec, err)
		return
	}

	// 检查是否到达 end 节点
	exec.Status = ExecSuccess
	now := time.Now()
	exec.FinishedAt = &now
	exec.Duration = now.Sub(exec.StartedAt)

	e.execLogger.LogEnd(exec.ID, ExecSuccess, nil)

	if e.store != nil {
		_ = e.store.SaveExecution(exec)
	}

	e.logger.Info("execution completed",
		zap.String("execution_id", exec.ID),
		zap.Duration("duration", exec.Duration),
	)
}

// executeFromNode 从指定节点开始执行.
func (e *Engine) executeFromNode(ctx context.Context, wf *Workflow, exec *Execution, nodeID string) error {
	// 检查上下文取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	node, ok := wf.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %q not found", nodeID)
	}

	if !node.Enabled {
		e.execLogger.Log(LogInfo, exec.ID, nodeID, "node disabled, skipping", nil)
		return e.followEdges(ctx, wf, exec, nodeID)
	}

	switch node.Type {
	case NodeTypeEnd:
		return nil // 到达结束节点

	case NodeTypeStart:
		return e.followEdges(ctx, wf, exec, nodeID)

	case NodeTypeCondition:
		return e.executeConditionNode(ctx, wf, exec, node)

	case NodeTypeLoop:
		return e.executeLoopNode(ctx, wf, exec, node)

	case NodeTypeAction, NodeTypeTrigger:
		return e.executeActionNode(ctx, wf, exec, node)

	default:
		return fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// executeActionNode 执行动作节点.
func (e *Engine) executeActionNode(ctx context.Context, wf *Workflow, exec *Execution, node *Node) error {
	handler, ok := e.handlers[node.ActionType]
	if !ok {
		handler, ok = e.handlers[ActionShell] // 默认 fallback
		if !ok {
			return fmt.Errorf("no handler for action type: %s", node.ActionType)
		}
	}

	// 准备输入
	input := make(map[string]interface{})
	for k, v := range exec.Context {
		input[k] = v
	}

	actionCtx := &ActionContext{
		Config:    node.Config,
		Input:     input,
		Variables: exec.Context,
		Logger:    e.execLogger,
		Timeout:   node.Timeout,
	}
	if actionCtx.Timeout == 0 {
		actionCtx.Timeout = 60 * time.Second
	}

	step := &StepExecution{
		NodeID:    node.ID,
		Status:    ExecRunning,
		Input:     input,
		StartedAt: time.Now(),
	}
	exec.Steps = append(exec.Steps, step)

	// 执行动作（含重试）
	var result *ActionResult
	var err error

	maxRetries := 0
	if node.RetryPolicy != nil {
		maxRetries = node.RetryPolicy.MaxRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		step.RetryCount = attempt
		if attempt > 0 {
			e.logger.Info("retrying action",
				zap.String("node_id", node.ID),
				zap.Int("attempt", attempt),
			)
			if node.RetryPolicy != nil {
				time.Sleep(node.RetryPolicy.Interval)
			}
		}

		result, err = handler.Execute(actionCtx)
		if err == nil && result != nil && result.Success {
			break
		}
	}

	now := time.Now()
	step.FinishedAt = &now
	step.Duration = now.Sub(step.StartedAt)

	if err != nil {
		step.Status = ExecFailed
		step.Error = err.Error()
		e.execLogger.LogStep(exec.ID, node.ID, ExecFailed, input, nil, err)
		return fmt.Errorf("action %q failed: %w", node.ID, err)
	}
	if result == nil || !result.Success {
		step.Status = ExecFailed
		errMsg := "action returned unsuccessful result"
		if result != nil && result.Error != "" {
			errMsg = result.Error
		}
		step.Error = errMsg
		e.execLogger.LogStep(exec.ID, node.ID, ExecFailed, input, nil, fmt.Errorf("%s", errMsg))
		return fmt.Errorf("action %q: %s", node.ID, errMsg)
	}

	step.Status = ExecSuccess
	step.Output = result.Output
	e.execLogger.LogStep(exec.ID, node.ID, ExecSuccess, input, result.Output, nil)

	// 将输出合并到执行上下文
	if result.Output != nil {
		for k, v := range result.Output {
			exec.Context[fmt.Sprintf("%s.%s", node.ID, k)] = v
		}
	}

	return e.followEdges(ctx, wf, exec, node.ID)
}

// executeConditionNode 执行条件节点.
func (e *Engine) executeConditionNode(ctx context.Context, wf *Workflow, exec *Execution, node *Node) error {
	if node.Condition == nil {
		return e.followEdges(ctx, wf, exec, node.ID)
	}

	result := e.condEval.Evaluate(node.Condition, exec.Context)

	e.logger.Debug("condition evaluated",
		zap.String("node_id", node.ID),
		zap.Bool("matched", result.Matched),
		zap.String("detail", result.Detail),
	)

	// 根据条件结果选择路径
	for _, edge := range wf.Edges {
		if edge.From != node.ID {
			continue
		}

		// 无条件边或条件匹配的边
		if edge.Condition == "" || (result.Matched && edge.Condition == "true") || (!result.Matched && edge.Condition == "false") {
			if err := e.executeFromNode(ctx, wf, exec, edge.To); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

// executeLoopNode 执行循环节点.
func (e *Engine) executeLoopNode(ctx context.Context, wf *Workflow, exec *Execution, node *Node) error {
	if node.LoopConfig == nil {
		return e.followEdges(ctx, wf, exec, node.ID)
	}

	cfg := node.LoopConfig
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 100 // 默认上限
	}

	// 获取循环目标节点
	targets := e.getLoopTargets(wf, node.ID)
	if len(targets) == 0 {
		return fmt.Errorf("loop node %q has no targets", node.ID)
	}

	for i := 0; i < maxIter; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 检查中断条件
		if cfg.BreakCondition != "" {
			cond := &ConditionExpr{
				Op:    OpEquals,
				Field: cfg.BreakCondition,
				Value: "true",
			}
			if e.condEval.Evaluate(cond, exec.Context).Matched {
				break
			}
		}

		// 设置循环变量
		exec.Context["_loop_index"] = i

		// 执行循环体
		for _, target := range targets {
			if err := e.executeFromNode(ctx, wf, exec, target); err != nil {
				return err
			}
		}
	}

	// 跳过循环体，继续后续流程
	return e.followEdgesExcluding(ctx, wf, exec, node.ID, e.getLoopTargets(wf, node.ID))
}

// followEdges 沿边继续执行.
func (e *Engine) followEdges(ctx context.Context, wf *Workflow, exec *Execution, nodeID string) error {
	for _, edge := range wf.Edges {
		if edge.From == nodeID {
			if err := e.executeFromNode(ctx, wf, exec, edge.To); err != nil {
				return err
			}
			return nil // 默认只走第一条边
		}
	}
	return nil
}

// followEdgesExcluding 沿边继续执行（排除指定目标）.
func (e *Engine) followEdgesExcluding(ctx context.Context, wf *Workflow, exec *Execution, nodeID string, excludeTargets []string) error {
	excludeMap := make(map[string]bool, len(excludeTargets))
	for _, t := range excludeTargets {
		excludeMap[t] = true
	}
	for _, edge := range wf.Edges {
		if edge.From == nodeID && !excludeMap[edge.To] {
			return e.executeFromNode(ctx, wf, exec, edge.To)
		}
	}
	return nil
}

// getLoopTargets 获取循环节点的目标节点.
func (e *Engine) getLoopTargets(wf *Workflow, loopNodeID string) []string {
	targets := make([]string, 0)
	for _, edge := range wf.Edges {
		if edge.From == loopNodeID && edge.Condition == "loop_body" {
			targets = append(targets, edge.To)
		}
	}
	return targets
}

// failExecution 标记执行失败.
func (e *Engine) failExecution(exec *Execution, err error) {
	exec.Status = ExecFailed
	exec.Error = err.Error()
	now := time.Now()
	exec.FinishedAt = &now
	exec.Duration = now.Sub(exec.StartedAt)

	e.execLogger.LogEnd(exec.ID, ExecFailed, err)

	if e.store != nil {
		_ = e.store.SaveExecution(exec)
	}

	e.logger.Error("execution failed",
		zap.String("execution_id", exec.ID),
		zap.Error(err),
	)
}

// ========== 执行查询 ==========

// GetExecution 获取执行记录.
func (e *Engine) GetExecution(id string) (*Execution, error) {
	e.mu.RLock()
	exec, ok := e.executions[id]
	e.mu.RUnlock()

	if !ok {
		if e.store != nil {
			return e.store.GetExecution(id)
		}
		return nil, fmt.Errorf("execution %q not found", id)
	}
	return exec, nil
}

// ListExecutions 列出工作流执行记录.
func (e *Engine) ListExecutions(workflowID string, limit int) ([]*Execution, error) {
	if e.store != nil {
		return e.store.ListExecutions(workflowID, limit)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	execs := make([]*Execution, 0)
	for _, exec := range e.executions {
		if workflowID == "" || exec.WorkflowID == workflowID {
			execs = append(execs, exec)
		}
	}
	return execs, nil
}

// GetExecLogger 获取执行日志器.
func (e *Engine) GetExecLogger() ExecutionLogger {
	return e.execLogger
}

// GetTriggerManager 获取触发器管理器.
func (e *Engine) GetTriggerManager() *TriggerManager {
	return e.triggerMgr
}
