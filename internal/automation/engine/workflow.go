package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/internal/automation/action"
	"nas-os/internal/automation/trigger"
)

// Workflow 定义一个自动化工作流
type Workflow struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Enabled      bool            `json:"enabled"`
	Trigger      trigger.Trigger `json:"trigger"`
	Actions      []action.Action `json:"actions"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	LastRun      *time.Time      `json:"last_run,omitempty"`
	RunCount     int             `json:"run_count"`
	SuccessCount int             `json:"success_count"`
	FailCount    int             `json:"fail_count"`
}

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusRunning ExecutionStatus = "running"
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusFailed  ExecutionStatus = "failed"
)

// ExecutionRecord 执行记录
type ExecutionRecord struct {
	WorkflowID  string                 `json:"workflow_id"`
	StartedAt   time.Time              `json:"started_at"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
	Status      ExecutionStatus        `json:"status"`
	Error       string                 `json:"error,omitempty"`
	EventData   map[string]interface{} `json:"event_data,omitempty"`
	ActionIndex int                    `json:"action_index"` // 失败时的动作索引
}

// ExecutionHistory 执行历史管理器
type ExecutionHistory struct {
	records map[string][]ExecutionRecord // workflowID -> records
	mu      sync.RWMutex
	maxSize int // 每个工作流最多保留的记录数
}

// NewExecutionHistory 创建执行历史管理器
func NewExecutionHistory(maxSize int) *ExecutionHistory {
	if maxSize <= 0 {
		maxSize = 100 // 默认保留 100 条
	}
	return &ExecutionHistory{
		records: make(map[string][]ExecutionRecord),
		maxSize: maxSize,
	}
}

// AddRecord 添加执行记录
func (h *ExecutionHistory) AddRecord(record ExecutionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	records := h.records[record.WorkflowID]
	records = append(records, record)

	// 限制记录数量
	if len(records) > h.maxSize {
		records = records[len(records)-h.maxSize:]
	}

	h.records[record.WorkflowID] = records
}

// GetRecords 获取指定工作流的执行记录
func (h *ExecutionHistory) GetRecords(workflowID string, limit int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	records := h.records[workflowID]
	if limit > 0 && len(records) > limit {
		return records[len(records)-limit:]
	}
	return records
}

// GetRecentRecords 获取最近的执行记录（所有工作流）
func (h *ExecutionHistory) GetRecentRecords(limit int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var allRecords []ExecutionRecord
	for _, records := range h.records {
		allRecords = append(allRecords, records...)
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(allRecords); i++ {
		for j := i + 1; j < len(allRecords); j++ {
			if allRecords[i].StartedAt.Before(allRecords[j].StartedAt) {
				allRecords[i], allRecords[j] = allRecords[j], allRecords[i]
			}
		}
	}

	if limit > 0 && len(allRecords) > limit {
		return allRecords[:limit]
	}
	return allRecords
}

// ClearRecords 清除指定工作流的执行记录
func (h *ExecutionHistory) ClearRecords(workflowID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.records, workflowID)
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	workflows   map[string]*Workflow
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	storagePath string                // 持久化存储路径
	history     *ExecutionHistory     // 执行历史
}

// NewWorkflowEngine 创建新的工作流引擎
func NewWorkflowEngine() *WorkflowEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkflowEngine{
		workflows: make(map[string]*Workflow),
		ctx:       ctx,
		cancel:    cancel,
		history:   NewExecutionHistory(100),
	}
}

// NewWorkflowEngineWithStorage 创建带持久化的工作流引擎
func NewWorkflowEngineWithStorage(storagePath string) (*WorkflowEngine, error) {
	engine := NewWorkflowEngine()
	engine.storagePath = storagePath

	// 确保存储目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// 加载已保存的工作流
	if err := engine.loadFromStorage(); err != nil {
		return nil, fmt.Errorf("failed to load workflows from storage: %w", err)
	}

	return engine, nil
}

// loadFromStorage 从存储加载工作流
func (e *WorkflowEngine) loadFromStorage() error {
	if e.storagePath == "" {
		return nil
	}

	entries, err := os.ReadDir(e.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(e.storagePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var wf Workflow
		if err := json.Unmarshal(data, &wf); err != nil {
			continue
		}

		e.workflows[wf.ID] = &wf
	}

	return nil
}

// saveToStorage 保存工作流到存储
func (e *WorkflowEngine) saveToStorage(wf *Workflow) error {
	if e.storagePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(e.storagePath, wf.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteFromStorage 从存储删除工作流
func (e *WorkflowEngine) deleteFromStorage(id string) error {
	if e.storagePath == "" {
		return nil
	}

	path := filepath.Join(e.storagePath, id+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
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

	// 持久化
	if err := e.saveToStorage(wf); err != nil {
		delete(e.workflows, wf.ID)
		return fmt.Errorf("failed to persist workflow: %w", err)
	}

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
	wf.ID = id // 确保 ID 不变
	e.workflows[id] = wf

	// 持久化
	if err := e.saveToStorage(wf); err != nil {
		return fmt.Errorf("failed to persist workflow: %w", err)
	}

	return nil
}

// DeleteWorkflow 删除工作流
func (e *WorkflowEngine) DeleteWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workflows[id]; !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	// 先删除存储文件
	if err := e.deleteFromStorage(id); err != nil {
		return fmt.Errorf("failed to delete workflow from storage: %w", err)
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

	// 创建执行记录
	record := ExecutionRecord{
		WorkflowID: id,
		StartedAt:  now,
		Status:     ExecutionStatusRunning,
		EventData:  eventData,
	}
	e.history.AddRecord(record)

	// 执行动作
	ctx := context.Background()
	contextData := map[string]interface{}{
		"event":       eventData,
		"timestamp":   now,
		"workflow_id": id,
	}

	execErr := func() error {
		for i, act := range actionsCopy {
			if err := act.Execute(ctx, contextData); err != nil {
				record.ActionIndex = i
				return fmt.Errorf("action %d failed: %w", i, err)
			}
		}
		return nil
	}()

	// 更新执行记录
	finishedAt := time.Now()
	record.FinishedAt = &finishedAt

	if execErr != nil {
		wf.FailCount++
		record.Status = ExecutionStatusFailed
		record.Error = execErr.Error()
	} else {
		wf.SuccessCount++
		record.Status = ExecutionStatusSuccess
	}

	// 更新历史记录
	e.history.AddRecord(record)

	// 持久化更新的统计信息
	e.mu.Lock()
	e.saveToStorage(wf)
	e.mu.Unlock()

	return execErr
}

// GetExecutionHistory 获取执行历史
func (e *WorkflowEngine) GetExecutionHistory(workflowID string, limit int) []ExecutionRecord {
	if workflowID != "" {
		return e.history.GetRecords(workflowID, limit)
	}
	return e.history.GetRecentRecords(limit)
}

// ClearExecutionHistory 清除执行历史
func (e *WorkflowEngine) ClearExecutionHistory(workflowID string) {
	e.history.ClearRecords(workflowID)
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
