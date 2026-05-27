// Package workflowengine 提供工作流引擎核心管理器
package workflowengine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 工作流引擎管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	workflows map[string]*Workflow
	executions map[string]*Execution
	templates  map[string]*WorkflowTemplate
	auditLogs  []AuditLog

	// 事件通道
	eventCh chan Event

	// 取消函数
	cancelFuncs map[string]context.CancelFunc
}

// NewManager 创建工作流管理器
func NewManager(logger *zap.Logger) *Manager {
	m := &Manager{
		logger:      logger,
		workflows:   make(map[string]*Workflow),
		executions:  make(map[string]*Execution),
		templates:   make(map[string]*WorkflowTemplate),
		auditLogs:   make([]AuditLog, 0),
		eventCh:     make(chan Event, 100),
		cancelFuncs: make(map[string]context.CancelFunc),
	}

	// 初始化内置模板
	m.initBuiltinTemplates()

	return m
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("workflow engine started")
	go m.eventLoop(ctx)
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.logger.Info("workflow engine stopped")
	return nil
}

// ========== 工作流 CRUD ==========

// CreateWorkflow 创建工作流
func (m *Manager) CreateWorkflow(req *CreateWorkflowRequest, userID string) (*Workflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validateNodes(req.Nodes); err != nil {
		return nil, fmt.Errorf("invalid workflow nodes: %w", err)
	}

	workflow := &Workflow{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Status:      WorkflowStatusDraft,
		Version:     1,
		Nodes:       req.Nodes,
		Triggers:    req.Triggers,
		Variables:   req.Variables,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	m.workflows[workflow.ID] = workflow
	m.addAuditLog("workflow", workflow.ID, "create", userID, nil)

	m.logger.Info("workflow created", zap.String("id", workflow.ID), zap.String("name", workflow.Name))
	return workflow, nil
}

// GetWorkflow 获取工作流
func (m *Manager) GetWorkflow(id string) (*Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflow, ok := m.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", id)
	}
	return workflow, nil
}

// UpdateWorkflow 更新工作流
func (m *Manager) UpdateWorkflow(id string, req *UpdateWorkflowRequest, userID string) (*Workflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	workflow, ok := m.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", id)
	}

	if req.Name != nil {
		workflow.Name = *req.Name
	}
	if req.Description != nil {
		workflow.Description = *req.Description
	}
	if req.Nodes != nil {
		if err := m.validateNodes(req.Nodes); err != nil {
			return nil, fmt.Errorf("invalid workflow nodes: %w", err)
		}
		workflow.Nodes = req.Nodes
	}
	if req.Triggers != nil {
		workflow.Triggers = req.Triggers
	}
	if req.Variables != nil {
		workflow.Variables = req.Variables
	}
	if req.Tags != nil {
		workflow.Tags = req.Tags
	}

	workflow.Version++
	workflow.UpdatedAt = time.Now()
	workflow.UpdatedBy = userID

	m.addAuditLog("workflow", id, "update", userID, nil)
	m.logger.Info("workflow updated", zap.String("id", id))
	return workflow, nil
}

// DeleteWorkflow 删除工作流
func (m *Manager) DeleteWorkflow(id string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workflows[id]; !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	// 检查是否有正在运行的执行
	for _, exec := range m.executions {
		if exec.WorkflowID == id && exec.Status == ExecutionStatusRunning {
			return fmt.Errorf("cannot delete workflow with running executions")
		}
	}

	delete(m.workflows, id)
	m.addAuditLog("workflow", id, "delete", userID, nil)
	m.logger.Info("workflow deleted", zap.String("id", id))
	return nil
}

// ListWorkflows 列出工作流
func (m *Manager) ListWorkflows(filter *WorkflowFilter) []*Workflow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Workflow
	for _, w := range m.workflows {
		if !m.matchWorkflowFilter(w, filter) {
			continue
		}
		result = append(result, w)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	// 分页
	if filter != nil && filter.PageSize > 0 {
		page := filter.Page
		if page <= 0 {
			page = 1
		}
		start := (page - 1) * filter.PageSize
		if start >= len(result) {
			return []*Workflow{}
		}
		end := start + filter.PageSize
		if end > len(result) {
			end = len(result)
		}
		return result[start:end]
	}

	return result
}

// ActivateWorkflow 激活工作流
func (m *Manager) ActivateWorkflow(id string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	workflow, ok := m.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	if len(workflow.Nodes) == 0 {
		return fmt.Errorf("cannot activate workflow without nodes")
	}

	workflow.Status = WorkflowStatusActive
	workflow.UpdatedAt = time.Now()
	workflow.UpdatedBy = userID

	m.addAuditLog("workflow", id, "activate", userID, nil)
	m.logger.Info("workflow activated", zap.String("id", id))
	return nil
}

// DeactivateWorkflow 停用工作流
func (m *Manager) DeactivateWorkflow(id string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	workflow, ok := m.workflows[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}

	workflow.Status = WorkflowStatusDisabled
	workflow.UpdatedAt = time.Now()
	workflow.UpdatedBy = userID

	m.addAuditLog("workflow", id, "deactivate", userID, nil)
	m.logger.Info("workflow deactivated", zap.String("id", id))
	return nil
}

// ========== 工作流执行 ==========

// ExecuteWorkflow 执行工作流
func (m *Manager) ExecuteWorkflow(workflowID string, req *ExecuteWorkflowRequest) (*Execution, error) {
	m.mu.Lock()

	workflow, ok := m.workflows[workflowID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	if workflow.Status != WorkflowStatusActive {
		m.mu.Unlock()
		return nil, fmt.Errorf("workflow is not active")
	}

	execution := &Execution{
		ID:              uuid.New().String(),
		WorkflowID:      workflowID,
		Status:          ExecutionStatusRunning,
		TriggerType:     "manual",
		TriggeredBy:     req.TriggeredBy,
		Input:           req.Input,
		NodeStates:      make(map[string]NodeExecutionState),
		StartedAt:       time.Now(),
		WorkflowVersion: workflow.Version,
	}

	// 初始化所有节点状态
	for _, node := range workflow.Nodes {
		execution.NodeStates[node.ID] = NodeExecutionState{
			NodeID: node.ID,
			Status: NodeStatusPending,
		}
	}

	m.executions[execution.ID] = execution
	workflow.ExecutionCount++
	now := time.Now()
	workflow.LastExecutedAt = &now

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[execution.ID] = cancel

	m.mu.Unlock()

	// 异步执行
	go m.runExecution(ctx, execution, workflow)

	m.addAuditLog("execution", execution.ID, "start", req.TriggeredBy, map[string]interface{}{
		"workflowId": workflowID,
	})

	return execution, nil
}

// CancelExecution 取消执行
func (m *Manager) CancelExecution(executionID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	execution, ok := m.executions[executionID]
	if !ok {
		return fmt.Errorf("execution %s not found", executionID)
	}

	if execution.Status != ExecutionStatusRunning {
		return fmt.Errorf("execution is not running")
	}

	if cancel, ok := m.cancelFuncs[executionID]; ok {
		cancel()
		delete(m.cancelFuncs, executionID)
	}

	execution.Status = ExecutionStatusCancelled
	now := time.Now()
	execution.CompletedAt = &now
	execution.Duration = now.Sub(execution.StartedAt).String()

	m.addAuditLog("execution", executionID, "cancel", userID, nil)
	m.logger.Info("execution cancelled", zap.String("executionId", executionID))
	return nil
}

// GetExecution 获取执行记录
func (m *Manager) GetExecution(id string) (*Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	execution, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution %s not found", id)
	}
	return execution, nil
}

// ListExecutions 列出执行记录
func (m *Manager) ListExecutions(filter *ExecutionFilter) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Execution
	for _, e := range m.executions {
		if !m.matchExecutionFilter(e, filter) {
			continue
		}
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	if filter != nil && filter.PageSize > 0 {
		page := filter.Page
		if page <= 0 {
			page = 1
		}
		start := (page - 1) * filter.PageSize
		if start >= len(result) {
			return []*Execution{}
		}
		end := start + filter.PageSize
		if end > len(result) {
			end = len(result)
		}
		return result[start:end]
	}

	return result
}

// ========== 事件系统 ==========

// EmitEvent 发送事件
func (m *Manager) EmitEvent(event Event) {
	select {
	case m.eventCh <- event:
	default:
		m.logger.Warn("event channel full, dropping event", zap.String("type", event.Type))
	}
}

// eventLoop 事件处理循环
func (m *Manager) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-m.eventCh:
			m.handleEvent(event)
		}
	}
}

// handleEvent 处理事件
func (m *Manager) handleEvent(event Event) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, workflow := range m.workflows {
		if workflow.Status != WorkflowStatusActive {
			continue
		}

		for _, trigger := range workflow.Triggers {
			if !trigger.Enabled {
				continue
			}

			if trigger.Type == TriggerTypeEvent && trigger.Config.EventType == event.Type {
				m.logger.Info("event triggered workflow",
					zap.String("workflowId", workflow.ID),
					zap.String("eventType", event.Type),
				)

				go func(wf *Workflow) {
					req := &ExecuteWorkflowRequest{
						Input:       event.Data,
						TriggeredBy: "event:" + event.ID,
					}
					if _, err := m.ExecuteWorkflow(wf.ID, req); err != nil {
						m.logger.Error("failed to trigger workflow",
							zap.String("workflowId", wf.ID),
							zap.Error(err),
						)
					}
				}(workflow)
			}
		}
	}
}

// ========== 模板管理 ==========

// ListTemplates 列出模板
func (m *Manager) ListTemplates() []*WorkflowTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*WorkflowTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].CreatedAt.After(templates[j].CreatedAt)
	})

	return templates
}

// GetTemplate 获取模板
func (m *Manager) GetTemplate(id string) (*WorkflowTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpl, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %s not found", id)
	}
	return tmpl, nil
}

// CreateFromTemplate 从模板创建工作流
func (m *Manager) CreateFromTemplate(templateID string, name string, userID string) (*Workflow, error) {
	m.mu.RLock()
	tmpl, ok := m.templates[templateID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("template %s not found", templateID)
	}

	req := &CreateWorkflowRequest{
		Name:        name,
		Description: tmpl.Description,
		Nodes:       tmpl.Workflow.Nodes,
		Triggers:    tmpl.Workflow.Triggers,
		Variables:   tmpl.Workflow.Variables,
		Tags:        tmpl.Tags,
	}

	return m.CreateWorkflow(req, userID)
}

// ========== 审计日志 ==========

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs(filter *AuditLogFilter) []AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AuditLog
	for _, log := range m.auditLogs {
		if !m.matchAuditLogFilter(&log, filter) {
			continue
		}
		result = append(result, log)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if filter != nil && filter.PageSize > 0 {
		page := filter.Page
		if page <= 0 {
			page = 1
		}
		start := (page - 1) * filter.PageSize
		if start >= len(result) {
			return []AuditLog{}
		}
		end := start + filter.PageSize
		if end > len(result) {
			end = len(result)
		}
		return result[start:end]
	}

	return result
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() *WorkflowStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WorkflowStats{
		TotalWorkflows: len(m.workflows),
		TotalExecutions: len(m.executions),
	}

	for _, w := range m.workflows {
		if w.Status == WorkflowStatusActive {
			stats.ActiveWorkflows++
		}
	}

	var totalDuration time.Duration
	var durationCount int
	successCount := 0

	for _, e := range m.executions {
		switch e.Status {
		case ExecutionStatusRunning:
			stats.RunningExecs++
		case ExecutionStatusSuccess:
			stats.SuccessExecs++
			successCount++
		case ExecutionStatusFailed:
			stats.FailedExecs++
		}

		if e.CompletedAt != nil {
			d := e.CompletedAt.Sub(e.StartedAt)
			totalDuration += d
			durationCount++
		}
	}

	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalExecutions) * 100
	}

	if durationCount > 0 {
		avg := totalDuration / time.Duration(durationCount)
		stats.AvgExecutionTime = avg.String()
	}

	return stats
}

// GetWorkflowDAG 获取工作流 DAG 结构
func (m *Manager) GetWorkflowDAG(workflowID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflow, ok := m.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	nodes := make([]map[string]interface{}, 0, len(workflow.Nodes))
	edges := make([]map[string]string, 0)

	for _, node := range workflow.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"id":   node.ID,
			"name": node.Name,
			"type": node.Type,
		})

		for _, dep := range node.Dependencies {
			edges = append(edges, map[string]string{
				"from": dep,
				"to":   node.ID,
			})
		}
	}

	return map[string]interface{}{
		"workflowId": workflowID,
		"nodes":      nodes,
		"edges":      edges,
	}, nil
}

// ========== 内部方法 ==========

// runExecution 执行工作流
func (m *Manager) runExecution(ctx context.Context, execution *Execution, workflow *Workflow) {
	m.logger.Info("execution started", zap.String("executionId", execution.ID))

	// 按拓扑序执行节点
	execOrder, err := m.topologicalSort(workflow.Nodes)
	if err != nil {
		m.completeExecution(execution, ExecutionStatusFailed, fmt.Sprintf("topological sort failed: %v", err))
		return
	}

	for _, nodeID := range execOrder {
		select {
		case <-ctx.Done():
			m.completeExecution(execution, ExecutionStatusCancelled, "execution cancelled")
			return
		default:
		}

		node := m.findNode(workflow.Nodes, nodeID)
		if node == nil {
			continue
		}

		// 检查依赖是否完成
		if !m.checkDependencies(execution, node) {
			continue
		}

		// 检查条件
		if node.Condition != nil && !m.evaluateCondition(node.Condition, execution) {
			m.updateNodeState(execution, nodeID, NodeStatusSkipped, nil, "")
			continue
		}

		// 执行节点
		m.executeNode(ctx, execution, node)
	}

	// 判断最终状态
	if execution.Status == ExecutionStatusRunning {
		allSuccess := true
		for _, state := range execution.NodeStates {
			if state.Status == NodeStatusFailed {
				m.completeExecution(execution, ExecutionStatusFailed, "node execution failed")
				return
			}
			if state.Status != NodeStatusSuccess && state.Status != NodeStatusSkipped {
				allSuccess = false
			}
		}

		if allSuccess {
			m.completeExecution(execution, ExecutionStatusSuccess, "")
		}
	}
}

// executeNode 执行单个节点
func (m *Manager) executeNode(ctx context.Context, execution *Execution, node *WorkflowNode) {
	m.updateNodeState(execution, node.ID, NodeStatusRunning, nil, "")
	m.logger.Info("executing node", zap.String("nodeId", node.ID), zap.String("type", node.Type))

	// 模拟节点执行（实际实现应根据 node.Type 调用不同的执行器）
	output := map[string]interface{}{
		"nodeId":    node.ID,
		"nodeType":  node.Type,
		"taskType":  node.TaskType,
		"timestamp": time.Now().Unix(),
	}

	// 模拟执行时间
	select {
	case <-ctx.Done():
		m.updateNodeState(execution, node.ID, NodeStatusCancelled, nil, "cancelled")
		return
	case <-time.After(100 * time.Millisecond):
	}

	m.updateNodeState(execution, node.ID, NodeStatusSuccess, output, "")
	m.logger.Info("node executed successfully", zap.String("nodeId", node.ID))
}

// completeExecution 完成执行
func (m *Manager) completeExecution(execution *Execution, status ExecutionStatus, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	execution.Status = status
	now := time.Now()
	execution.CompletedAt = &now
	execution.Duration = now.Sub(execution.StartedAt).String()

	if errMsg != "" {
		execution.Error = errMsg
	}

	delete(m.cancelFuncs, execution.ID)

	// 更新工作流统计
	if workflow, ok := m.workflows[execution.WorkflowID]; ok {
		successCount := 0
		totalCount := 0
		for _, e := range m.executions {
			if e.WorkflowID == execution.WorkflowID {
				totalCount++
				if e.Status == ExecutionStatusSuccess {
					successCount++
				}
			}
		}
		if totalCount > 0 {
			workflow.SuccessRate = float64(successCount) / float64(totalCount) * 100
		}
	}

	m.logger.Info("execution completed",
		zap.String("executionId", execution.ID),
		zap.String("status", string(status)),
		zap.String("duration", execution.Duration),
	)
}

// updateNodeState 更新节点状态
func (m *Manager) updateNodeState(execution *Execution, nodeID string, status NodeStatus, output map[string]interface{}, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := execution.NodeStates[nodeID]
	state.Status = status

	if status == NodeStatusRunning {
		now := time.Now()
		state.StartedAt = &now
	}

	if status == NodeStatusSuccess || status == NodeStatusFailed || status == NodeStatusSkipped || status == NodeStatusCancelled {
		now := time.Now()
		state.CompletedAt = &now
		if state.StartedAt != nil {
			state.Duration = now.Sub(*state.StartedAt).String()
		}
	}

	if output != nil {
		state.Output = output
	}
	if errMsg != "" {
		state.Error = errMsg
	}

	execution.NodeStates[nodeID] = state
}

// topologicalSort 拓扑排序
func (m *Manager) topologicalSort(nodes []WorkflowNode) ([]string, error) {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)
	nodeIDs := make(map[string]bool)

	for _, node := range nodes {
		nodeIDs[node.ID] = true
		if _, ok := inDegree[node.ID]; !ok {
			inDegree[node.ID] = 0
		}
		for _, dep := range node.Dependencies {
			graph[dep] = append(graph[dep], node.ID)
			inDegree[node.ID]++
		}
	}

	// BFS 拓扑排序
	var queue []string
	for id := range nodeIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, next := range graph[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(nodeIDs) {
		return nil, fmt.Errorf("cycle detected in workflow graph")
	}

	return order, nil
}

// checkDependencies 检查依赖是否完成
func (m *Manager) checkDependencies(execution *Execution, node *WorkflowNode) bool {
	for _, dep := range node.Dependencies {
		state, ok := execution.NodeStates[dep]
		if !ok || (state.Status != NodeStatusSuccess && state.Status != NodeStatusSkipped) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件
func (m *Manager) evaluateCondition(condition *Condition, execution *Execution) bool {
	// 简化实现：始终返回 true
	// 实际实现应根据 condition.Field, condition.Operator, condition.Value 进行评估
	return true
}

// findNode 查找节点
func (m *Manager) findNode(nodes []WorkflowNode, nodeID string) *WorkflowNode {
	for i := range nodes {
		if nodes[i].ID == nodeID {
			return &nodes[i]
		}
	}
	return nil
}

// validateNodes 验证节点
func (m *Manager) validateNodes(nodes []WorkflowNode) error {
	if len(nodes) == 0 {
		return fmt.Errorf("workflow must have at least one node")
	}

	ids := make(map[string]bool)
	for _, node := range nodes {
		if node.ID == "" {
			return fmt.Errorf("node ID cannot be empty")
		}
		if ids[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		ids[node.ID] = true
	}

	// 验证依赖引用
	for _, node := range nodes {
		for _, dep := range node.Dependencies {
			if !ids[dep] {
				return fmt.Errorf("node %s depends on non-existent node %s", node.ID, dep)
			}
		}
	}

	return nil
}

// matchWorkflowFilter 匹配工作流过滤条件
func (m *Manager) matchWorkflowFilter(w *Workflow, filter *WorkflowFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Status != "" && w.Status != filter.Status {
		return false
	}

	if filter.Keyword != "" {
		found := false
		if contains(w.Name, filter.Keyword) || contains(w.Description, filter.Keyword) {
			found = true
		}
		if !found {
			return false
		}
	}

	if len(filter.Tags) > 0 {
		hasTag := false
		for _, ft := range filter.Tags {
			for _, wt := range w.Tags {
				if ft == wt {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	return true
}

// matchExecutionFilter 匹配执行过滤条件
func (m *Manager) matchExecutionFilter(e *Execution, filter *ExecutionFilter) bool {
	if filter == nil {
		return true
	}

	if filter.WorkflowID != "" && e.WorkflowID != filter.WorkflowID {
		return false
	}

	if filter.Status != "" && e.Status != filter.Status {
		return false
	}

	if filter.StartTime != nil && e.StartedAt.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && e.StartedAt.After(*filter.EndTime) {
		return false
	}

	return true
}

// matchAuditLogFilter 匹配审计日志过滤条件
func (m *Manager) matchAuditLogFilter(log *AuditLog, filter *AuditLogFilter) bool {
	if filter == nil {
		return true
	}

	if filter.EntityType != "" && log.EntityType != filter.EntityType {
		return false
	}

	if filter.EntityID != "" && log.EntityID != filter.EntityID {
		return false
	}

	if filter.Action != "" && log.Action != filter.Action {
		return false
	}

	if filter.StartTime != nil && log.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && log.Timestamp.After(*filter.EndTime) {
		return false
	}

	return true
}

// addAuditLog 添加审计日志
func (m *Manager) addAuditLog(entityType, entityID, action, userID string, details map[string]interface{}) {
	log := AuditLog{
		ID:         uuid.New().String(),
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		UserID:     userID,
		Details:    details,
		Timestamp:  time.Now(),
	}
	m.auditLogs = append(m.auditLogs, log)
}

// initBuiltinTemplates 初始化内置模板
func (m *Manager) initBuiltinTemplates() {
	templates := []*WorkflowTemplate{
		{
			ID:          "tpl-backup",
			Name:        "数据备份",
			Description: "自动备份数据并发送通知",
			Category:    "backup",
			IsBuiltin:   true,
			Workflow: Workflow{
				Nodes: []WorkflowNode{
					{ID: "start", Name: "开始", Type: "task", TaskType: "builtin"},
					{ID: "backup", Name: "执行备份", Type: "task", TaskType: "shell", Dependencies: []string{"start"}},
					{ID: "notify", Name: "发送通知", Type: "task", TaskType: "builtin", Dependencies: []string{"backup"}},
				},
			},
			Tags:      []string{"backup", "builtin"},
			CreatedAt: time.Now(),
		},
		{
			ID:          "tpl-monitor",
			Name:        "系统监控",
			Description: "监控系统指标并在异常时告警",
			Category:    "monitoring",
			IsBuiltin:   true,
			Workflow: Workflow{
				Nodes: []WorkflowNode{
					{ID: "check", Name: "检查指标", Type: "task", TaskType: "builtin"},
					{ID: "evaluate", Name: "评估阈值", Type: "condition", Dependencies: []string{"check"}},
					{ID: "alert", Name: "发送告警", Type: "task", TaskType: "builtin", Dependencies: []string{"evaluate"}},
				},
			},
			Tags:      []string{"monitoring", "builtin"},
			CreatedAt: time.Now(),
		},
	}

	for _, t := range templates {
		m.templates[t.ID] = t
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
