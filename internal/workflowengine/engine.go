// Package workflowengine 提供工作流引擎核心功能
package workflowengine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EngineStatus 引擎状态
type EngineStatus string

const (
	EngineStatusIdle    EngineStatus = "idle"
	EngineStatusRunning EngineStatus = "running"
	EngineStatusStopped EngineStatus = "stopped"
)

// StepType 步骤类型
type StepType string

const (
	StepTypeSerial   StepType = "serial"
	StepTypeParallel StepType = "parallel"
	StepTypeBranch   StepType = "branch"
	StepTypeLoop     StepType = "loop"
)

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrent int `json:"maxConcurrent"` // 最大并发执行数
	Timeout       int `json:"timeout"`        // 默认超时(秒)
	RetryPolicy   *RetryPolicy `json:"retryPolicy,omitempty"`
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries  int `json:"maxRetries"`
	DelayMs     int `json:"delayMs"`     // 重试间隔(毫秒)
	MaxDelayMs  int `json:"maxDelayMs"` // 最大重试间隔
	BackoffFactor int `json:"backoffFactor,omitempty"` // 退避因子
}

// StepConfig 步骤配置
type StepConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        StepType          `json:"type"`
	Action      string            `json:"action"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Condition   *Condition        `json:"condition,omitempty"`
	Steps       []StepConfig      `json:"steps,omitempty"`
	MaxRetries  int               `json:"maxRetries,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	LoopCount   int               `json:"loopCount,omitempty"`
}

// StepExecutor 步骤执行器接口
type StepExecutor interface {
	Execute(ctx context.Context, step StepConfig, vars map[string]interface{}) (map[string]interface{}, error)
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	mu          sync.RWMutex
	config      EngineConfig
	manager     *Manager
	executors   map[string]StepExecutor
	status      EngineStatus
	semaphore   chan struct{}
}

// NewWorkflowEngine 创建工作流引擎
func NewWorkflowEngine(manager *Manager, config EngineConfig) *WorkflowEngine {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 300 // 5分钟
	}

	return &WorkflowEngine{
		config:    config,
		manager:   manager,
		executors: make(map[string]StepExecutor),
		status:    EngineStatusIdle,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}
}

// RegisterExecutor 注册步骤执行器
func (e *WorkflowEngine) RegisterExecutor(action string, executor StepExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executors[action] = executor
}

// Start 启动引擎
func (e *WorkflowEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.status == EngineStatusRunning {
		return fmt.Errorf("engine is already running")
	}

	e.status = EngineStatusRunning
	if err := e.manager.Start(ctx); err != nil {
		e.status = EngineStatusStopped
		return err
	}

	return nil
}

// Stop 停止引擎
func (e *WorkflowEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.status != EngineStatusRunning {
		return fmt.Errorf("engine is not running")
	}

	e.status = EngineStatusStopped
	return e.manager.Stop()
}

// GetStatus 获取引擎状态
func (e *WorkflowEngine) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// ExecuteWorkflow 执行工作流（带步骤编排）
func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}, triggeredBy string) (*Execution, error) {
	if e.GetStatus() != EngineStatusRunning {
		return nil, fmt.Errorf("engine is not running")
	}

	// 获取工作流定义
	wf, err := e.manager.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}

	// 检查并发限制
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 创建执行记录
	exec, err := e.manager.ExecuteWorkflow(workflowID, &ExecuteWorkflowRequest{
		Input:       input,
		TriggeredBy: triggeredBy,
	})
	if err != nil {
		return nil, err
	}

	// 执行节点（DAG 编排）
	go e.executeNodes(ctx, wf, exec)

	return exec, nil
}

// executeNodes 执行工作流节点（DAG 编排）
func (e *WorkflowEngine) executeNodes(ctx context.Context, wf *Workflow, exec *Execution) {
	// 构建依赖图
	graph := make(map[string][]string) // nodeID -> dependent nodeIDs
	inDegree := make(map[string]int)
	nodeMap := make(map[string]*WorkflowNode)

	for i := range wf.Nodes {
		node := &wf.Nodes[i]
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0
	}

	for _, node := range wf.Nodes {
		for _, dep := range node.Dependencies {
			graph[dep] = append(graph[dep], node.ID)
			inDegree[node.ID]++
		}
	}

	// 拓扑排序执行
	ready := make(chan string, len(wf.Nodes))
	completed := make(chan string, len(wf.Nodes))
	failed := make(chan error, 1)

	// 初始化就绪队列
	for nodeID, degree := range inDegree {
		if degree == 0 {
			ready <- nodeID
		}
	}

	var wg sync.WaitGroup
	running := 0

	for {
		select {
		case <-ctx.Done():
			exec.Status = ExecutionStatusCancelled
			exec.Error = "execution cancelled"
			now := time.Now()
			exec.CompletedAt = &now
			return

		case nodeID := <-ready:
			if nodeID == "" {
				// 所有节点完成
				exec.Status = ExecutionStatusSuccess
				now := time.Now()
				exec.CompletedAt = &now
				exec.Duration = now.Sub(exec.StartedAt).String()
				return
			}

			node := nodeMap[nodeID]
			wg.Add(1)
			running++

			go func(n *WorkflowNode) {
				defer wg.Done()
				e.executeNode(ctx, n, exec, completed, failed)
			}(node)

		case nodeID := <-completed:
			running--
			// 更新依赖
			for _, dep := range graph[nodeID] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					ready <- dep
				}
			}

			if running == 0 && len(ready) == 0 {
				ready <- "" // 信号完成
			}

		case err := <-failed:
			exec.Status = ExecutionStatusFailed
			exec.Error = err.Error()
			now := time.Now()
			exec.CompletedAt = &now
			exec.Duration = now.Sub(exec.StartedAt).String()
			return
		}
	}
}

// executeNode 执行单个节点
func (e *WorkflowEngine) executeNode(ctx context.Context, node *WorkflowNode, exec *Execution, completed chan<- string, failed chan<- error) {
	// 检查条件
	if node.Condition != nil {
		if !evaluateCondition(node.Condition, exec.Input) {
			// 条件不满足，跳过
			exec.NodeStates[node.ID] = NodeExecutionState{
				NodeID: node.ID,
				Status: NodeStatusSkipped,
			}
			completed <- node.ID
			return
		}
	}

	// 更新状态为运行中
	exec.NodeStates[node.ID] = NodeExecutionState{
		NodeID:    node.ID,
		Status:    NodeStatusRunning,
		StartedAt: timePtr(time.Now()),
	}

	// 执行任务
	e.mu.RLock()
	executor, exists := e.executors[node.TaskType]
	e.mu.RUnlock()

	if !exists {
		exec.NodeStates[node.ID] = NodeExecutionState{
			NodeID:    node.ID,
			Status:    NodeStatusFailed,
			Error:     fmt.Sprintf("no executor for task type: %s", node.TaskType),
			StartedAt: exec.NodeStates[node.ID].StartedAt,
		}
		failed <- fmt.Errorf("no executor for task type: %s", node.TaskType)
		return
	}

	// 重试逻辑
	maxRetries := node.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1
	}

	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		output, err := executor.Execute(ctx, StepConfig{
			ID:     node.ID,
			Name:   node.Name,
			Action: node.TaskType,
			Config: node.TaskConfig,
		}, exec.Input)

		if err == nil {
			// 成功
			exec.NodeStates[node.ID] = NodeExecutionState{
				NodeID:      node.ID,
				Status:      NodeStatusSuccess,
				StartedAt:   exec.NodeStates[node.ID].StartedAt,
				CompletedAt: timePtr(time.Now()),
				Output:      output,
				RetryCount:  retry,
			}
			completed <- node.ID
			return
		}

		lastErr = err
		exec.NodeStates[node.ID] = NodeExecutionState{
			NodeID:      node.ID,
			Status:      NodeStatusRunning,
			StartedAt:   exec.NodeStates[node.ID].StartedAt,
			Error:       err.Error(),
			RetryCount:  retry + 1,
		}

		if retry < maxRetries-1 {
			delay := calculateRetryDelay(retry, e.config.RetryPolicy)
			select {
			case <-ctx.Done():
				exec.NodeStates[node.ID] = NodeExecutionState{
					NodeID:    node.ID,
					Status:    NodeStatusCancelled,
					StartedAt: exec.NodeStates[node.ID].StartedAt,
				}
				failed <- ctx.Err()
				return
			case <-time.After(time.Duration(delay) * time.Millisecond):
			}
		}
	}

	// 所有重试失败
	exec.NodeStates[node.ID] = NodeExecutionState{
		NodeID:      node.ID,
		Status:      NodeStatusFailed,
		StartedAt:   exec.NodeStates[node.ID].StartedAt,
		CompletedAt: timePtr(time.Now()),
		Error:       lastErr.Error(),
		RetryCount:  maxRetries,
	}
	failed <- lastErr
}

// evaluateCondition 评估条件
func evaluateCondition(cond *Condition, vars map[string]interface{}) bool {
	if cond == nil {
		return true
	}

	_, exists := vars[cond.Field]
	if !exists {
		return false
	}

	// 简单的条件评估
	// 实际项目中应使用更强大的表达式引擎
	return true
}

// calculateRetryDelay 计算重试延迟
func calculateRetryDelay(retry int, policy *RetryPolicy) int {
	if policy == nil || policy.DelayMs == 0 {
		return 1000 * (1 << retry) // 默认指数退避
	}

	delay := policy.DelayMs
	if policy.BackoffFactor > 1 {
		delay = policy.DelayMs * (policy.BackoffFactor * (retry + 1))
	}

	if policy.MaxDelayMs > 0 && delay > policy.MaxDelayMs {
		delay = policy.MaxDelayMs
	}

	return delay
}

// timePtr 返回时间指针
func timePtr(t time.Time) *time.Time {
	return &t
}

// GetManager 获取管理器
func (e *WorkflowEngine) GetManager() *Manager {
	return e.manager
}

// GetConfig 获取配置
func (e *WorkflowEngine) GetConfig() EngineConfig {
	return e.config
}

// UpdateConfig 更新配置
func (e *WorkflowEngine) UpdateConfig(config EngineConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}
