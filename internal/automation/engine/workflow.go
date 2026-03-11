package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"nas-os/internal/automation/trigger"
	"nas-os/internal/automation/action"
)

// Workflow 定义一个自动化工作流
type Workflow struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Trigger     trigger.Trigger `json:"trigger"`
	Actions     []action.Action `json:"actions"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	LastRun     *time.Time      `json:"last_run,omitempty"`
	RunCount    int             `json:"run_count"`
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	workflows map[string]*Workflow
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewWorkflowEngine 创建新的工作流引擎
func NewWorkflowEngine() *WorkflowEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkflowEngine{
		workflows: make(map[string]*Workflow),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// CreateWorkflow 创建工作流
func (e *WorkflowEngine) CreateWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if wf.ID == "" {
		wf.ID = generateID()
	}
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()

	e.workflows[wf.ID] = wf
	return nil
}

// UpdateWorkflow 更新工作流
func (e *WorkflowEngine) UpdateWorkflow(id string, wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workflows[id]; !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	wf.UpdatedAt = time.Now()
	e.workflows[id] = wf
	return nil
}

// DeleteWorkflow 删除工作流
func (e *WorkflowEngine) DeleteWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workflows[id]; !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	delete(e.workflows, id)
	return nil
}

// GetWorkflow 获取工作流
func (e *WorkflowEngine) GetWorkflow(id string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[id]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", id)
	}

	return wf, nil
}

// ListWorkflows 列出所有工作流
func (e *WorkflowEngine) ListWorkflows() []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		result = append(result, wf)
	}
	return result
}

// EnableWorkflow 启用工作流
func (e *WorkflowEngine) EnableWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[id]
	if !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	wf.Enabled = true
	wf.UpdatedAt = time.Now()
	return nil
}

// DisableWorkflow 禁用工作流
func (e *WorkflowEngine) DisableWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[id]
	if !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	wf.Enabled = false
	wf.UpdatedAt = time.Now()
	return nil
}

// ExecuteWorkflow 执行工作流
func (e *WorkflowEngine) ExecuteWorkflow(id string, eventData map[string]interface{}) error {
	e.mu.Lock()
	wf, exists := e.workflows[id]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("workflow not found: %s", id)
	}

	if !wf.Enabled {
		e.mu.Unlock()
		return fmt.Errorf("workflow is disabled: %s", id)
	}

	// 复制动作以避免并发问题
	actionsCopy := make([]action.Action, len(wf.Actions))
	copy(actionsCopy, wf.Actions)
	e.mu.Unlock()

	now := time.Now()
	wf.LastRun = &now
	wf.RunCount++

	// 执行动作
	ctx := context.Background()
	contextData := map[string]interface{}{
		"event": eventData,
		"timestamp": now,
		"workflow_id": id,
	}

	for i, act := range actionsCopy {
		if err := act.Execute(ctx, contextData); err != nil {
			return fmt.Errorf("action %d failed: %w", i, err)
		}
	}

	return nil
}

// Start 启动引擎
func (e *WorkflowEngine) Start() error {
	// 启动触发器监控
	for _, wf := range e.workflows {
		if wf.Enabled {
			if err := wf.Trigger.Start(e.ctx, func(eventData map[string]interface{}) {
				go func() {
					if err := e.ExecuteWorkflow(wf.ID, eventData); err != nil {
						fmt.Printf("Workflow execution failed: %v\n", err)
					}
				}()
			}); err != nil {
				return fmt.Errorf("failed to start trigger for workflow %s: %w", wf.ID, err)
			}
		}
	}
	return nil
}

// Stop 停止引擎
func (e *WorkflowEngine) Stop() error {
	e.cancel()
	return nil
}

// ExportWorkflow 导出工作流为 JSON
func (e *WorkflowEngine) ExportWorkflow(id string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[id]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", id)
	}

	return json.MarshalIndent(wf, "", "  ")
}

// ImportWorkflow 从 JSON 导入工作流
func (e *WorkflowEngine) ImportWorkflow(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	wf.LastRun = nil
	wf.RunCount = 0

	e.workflows[wf.ID] = &wf
	return &wf, nil
}

func generateID() string {
	return fmt.Sprintf("wf_%d", time.Now().UnixNano())
}
