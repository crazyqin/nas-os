// Package esignature 提供电子签名功能
package esignature

import (
	"errors"
	"sync"
	"time"
)

// WorkflowEngine 工作流引擎.
type WorkflowEngine struct {
	mu        sync.RWMutex
	engine    *Engine
	workflows map[string]*Workflow
	instances map[string]*WorkflowInstance
	idCounter int64
}

// WorkflowInstance 工作流实例.
type WorkflowInstance struct {
	ID          string        `json:"id"`
	WorkflowID  string        `json:"workflow_id"`
	DocumentID  string        `json:"document_id"`
	Status      string        `json:"status"` // running, completed, failed, cancelled
	CurrentStep int           `json:"current_step"`
	Steps       []StepInstance `json:"steps"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	CreatedBy   string        `json:"created_by"`
}

// StepInstance 步骤实例.
type StepInstance struct {
	StepID      string     `json:"step_id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Status      string     `json:"status"` // pending, in_progress, completed, failed, skipped
	Assignees   []string   `json:"assignees"`
	Results     []StepResult `json:"results,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// StepResult 步骤结果.
type StepResult struct {
	Assignee  string    `json:"assignee"`
	Action    string    `json:"action"` // approve, reject, sign
	Comment   string    `json:"comment,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// WorkflowRequest 工作流请求.
type WorkflowRequest struct {
	WorkflowID string `json:"workflow_id" binding:"required"`
	DocumentID string `json:"document_id" binding:"required"`
	CreatedBy  string `json:"created_by"`
}

// StepActionRequest 步骤操作请求.
type StepActionRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	StepID     string `json:"step_id" binding:"required"`
	Assignee   string `json:"assignee" binding:"required"`
	Action     string `json:"action" binding:"required"` // approve, reject, sign
	Comment    string `json:"comment,omitempty"`
}

// NewWorkflowEngine 创建工作流引擎.
func NewWorkflowEngine(engine *Engine) *WorkflowEngine {
	return &WorkflowEngine{
		engine:    engine,
		workflows: make(map[string]*Workflow),
		instances: make(map[string]*WorkflowInstance),
	}
}

// generateID 生成唯一ID.
func (we *WorkflowEngine) generateID(prefix string) string {
	we.idCounter++
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + string(rune('A'+we.idCounter%26))
}

// CreateWorkflow 创建工作流.
func (we *WorkflowEngine) CreateWorkflow(req CreateWorkflowRequest) (*Workflow, error) {
	if req.Name == "" {
		return nil, errors.New("名称不能为空")
	}

	we.mu.Lock()
	defer we.mu.Unlock()

	workflow := &Workflow{
		ID:          we.generateID("wf"),
		Name:        req.Name,
		Description: req.Description,
		Steps:       req.Steps,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置步骤ID
	for i := range workflow.Steps {
		if workflow.Steps[i].ID == "" {
			workflow.Steps[i].ID = we.generateID("step")
		}
	}

	we.workflows[workflow.ID] = workflow
	return workflow, nil
}

// GetWorkflow 获取工作流.
func (we *WorkflowEngine) GetWorkflow(id string) (*Workflow, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	wf, ok := we.workflows[id]
	if !ok {
		return nil, errors.New("工作流不存在")
	}
	return wf, nil
}

// UpdateWorkflow 更新工作流.
func (we *WorkflowEngine) UpdateWorkflow(id string, req UpdateWorkflowRequest) (*Workflow, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	wf, ok := we.workflows[id]
	if !ok {
		return nil, errors.New("工作流不存在")
	}

	if req.Name != nil {
		wf.Name = *req.Name
	}
	if req.Description != nil {
		wf.Description = *req.Description
	}
	if req.Status != nil {
		wf.Status = *req.Status
	}
	wf.UpdatedAt = time.Now()

	return wf, nil
}

// DeleteWorkflow 删除工作流.
func (we *WorkflowEngine) DeleteWorkflow(id string) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	if _, ok := we.workflows[id]; !ok {
		return errors.New("工作流不存在")
	}

	delete(we.workflows, id)
	return nil
}

// ListWorkflows 列出工作流.
func (we *WorkflowEngine) ListWorkflows() []*Workflow {
	we.mu.RLock()
	defer we.mu.RUnlock()

	result := make([]*Workflow, 0)
	for _, wf := range we.workflows {
		result = append(result, wf)
	}
	return result
}

// StartWorkflow 启动工作流.
func (we *WorkflowEngine) StartWorkflow(req WorkflowRequest) (*WorkflowInstance, error) {
	we.mu.RLock()
	wf, ok := we.workflows[req.WorkflowID]
	we.mu.RUnlock()

	if !ok {
		return nil, errors.New("工作流不存在")
	}

	// 验证文档存在
	if _, err := we.engine.GetDocument(req.DocumentID); err != nil {
		return nil, err
	}

	we.mu.Lock()
	defer we.mu.Unlock()

	// 创建步骤实例
	stepInstances := make([]StepInstance, 0)
	for _, step := range wf.Steps {
		stepInst := StepInstance{
			StepID:    step.ID,
			Name:      step.Name,
			Type:      step.Type,
			Status:    "pending",
			Assignees: step.Assignees,
			Results:   make([]StepResult, 0),
		}
		stepInstances = append(stepInstances, stepInst)
	}

	// 如果有步骤，第一个步骤设为进行中
	if len(stepInstances) > 0 {
		now := time.Now()
		stepInstances[0].Status = "in_progress"
		stepInstances[0].StartedAt = &now
	}

	instance := &WorkflowInstance{
		ID:          we.generateID("wfi"),
		WorkflowID:  req.WorkflowID,
		DocumentID:  req.DocumentID,
		Status:      "running",
		CurrentStep: 0,
		Steps:       stepInstances,
		StartedAt:   time.Now(),
		CreatedBy:   req.CreatedBy,
	}

	we.instances[instance.ID] = instance

	// 添加审计记录
	we.engine.addAudit(req.DocumentID, req.CreatedBy, "workflow_start", "启动工作流: "+wf.Name, "", "")

	return instance, nil
}

// GetInstance 获取工作流实例.
func (we *WorkflowEngine) GetInstance(id string) (*WorkflowInstance, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	inst, ok := we.instances[id]
	if !ok {
		return nil, errors.New("工作流实例不存在")
	}
	return inst, nil
}

// ProcessStep 处理步骤.
func (we *WorkflowEngine) ProcessStep(req StepActionRequest) (*WorkflowInstance, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	inst, ok := we.instances[req.InstanceID]
	if !ok {
		return nil, errors.New("工作流实例不存在")
	}

	if inst.Status != "running" {
		return nil, errors.New("工作流不在运行状态")
	}

	// 查找步骤
	stepIndex := -1
	for i, step := range inst.Steps {
		if step.StepID == req.StepID {
			stepIndex = i
			break
		}
	}

	if stepIndex == -1 {
		return nil, errors.New("步骤不存在")
	}

	step := &inst.Steps[stepIndex]
	if step.Status != "in_progress" {
		return nil, errors.New("步骤不在进行中状态")
	}

	// 验证操作人是否是被分配人
	isAssignee := false
	for _, assignee := range step.Assignees {
		if assignee == req.Assignee {
			isAssignee = true
			break
		}
	}
	if !isAssignee {
		return nil, errors.New("无权操作此步骤")
	}

	// 记录结果
	result := StepResult{
		Assignee:  req.Assignee,
		Action:    req.Action,
		Comment:   req.Comment,
		Timestamp: time.Now(),
	}
	step.Results = append(step.Results, result)

	// 添加审计记录
	we.engine.addAudit(inst.DocumentID, req.Assignee, "step_action",
		req.Action+" - 步骤: "+step.Name, "", "")

	// 检查是否所有被分配人都完成了
	allDone := true
	for _, assignee := range step.Assignees {
		done := false
		for _, r := range step.Results {
			if r.Assignee == assignee {
				done = true
				break
			}
		}
		if !done {
			allDone = false
			break
		}
	}

	// 获取工作流配置（已经持有写锁，直接访问）
	wf, _ := we.workflows[inst.WorkflowID]

	// 检查是否需要并行等待
	if wf != nil && stepIndex < len(wf.Steps) {
		parallel := wf.Steps[stepIndex].Parallel
		if parallel && !allDone {
			return inst, nil
		}
	}

	// 标记步骤完成
	now := time.Now()
	step.Status = "completed"
	step.CompletedAt = &now

	// 检查是否有拒绝
	hasReject := false
	for _, r := range step.Results {
		if r.Action == "reject" {
			hasReject = true
			break
		}
	}

	if hasReject {
		// 工作流失败
		inst.Status = "failed"
		inst.CompletedAt = &now
		we.engine.addAudit(inst.DocumentID, "", "workflow_fail", "工作流失败: 步骤被拒绝", "", "")
		return inst, nil
	}

	// 推进到下一步
	if stepIndex+1 < len(inst.Steps) {
		inst.CurrentStep = stepIndex + 1
		nextStep := &inst.Steps[stepIndex+1]
		nextStep.Status = "in_progress"
		nextStep.StartedAt = &now
	} else {
		// 所有步骤完成
		inst.Status = "completed"
		inst.CompletedAt = &now
		we.engine.addAudit(inst.DocumentID, "", "workflow_complete", "工作流完成", "", "")
	}

	return inst, nil
}

// CancelInstance 取消工作流实例.
func (we *WorkflowEngine) CancelInstance(id, userID string) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	inst, ok := we.instances[id]
	if !ok {
		return errors.New("工作流实例不存在")
	}

	if inst.Status != "running" {
		return errors.New("只能取消运行中的工作流")
	}

	now := time.Now()
	inst.Status = "cancelled"
	inst.CompletedAt = &now

	we.engine.addAudit(inst.DocumentID, userID, "workflow_cancel", "取消工作流", "", "")

	return nil
}

// ListInstances 列出工作流实例.
func (we *WorkflowEngine) ListInstances(workflowID, documentID string) []*WorkflowInstance {
	we.mu.RLock()
	defer we.mu.RUnlock()

	result := make([]*WorkflowInstance, 0)
	for _, inst := range we.instances {
		if workflowID != "" && inst.WorkflowID != workflowID {
			continue
		}
		if documentID != "" && inst.DocumentID != documentID {
			continue
		}
		result = append(result, inst)
	}
	return result
}

// GetRunningInstances 获取运行中的实例.
func (we *WorkflowEngine) GetRunningInstances() []*WorkflowInstance {
	we.mu.RLock()
	defer we.mu.RUnlock()

	result := make([]*WorkflowInstance, 0)
	for _, inst := range we.instances {
		if inst.Status == "running" {
			result = append(result, inst)
		}
	}
	return result
}

// SkipStep 跳过步骤.
func (we *WorkflowEngine) SkipStep(instanceID, stepID, userID string) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	inst, ok := we.instances[instanceID]
	if !ok {
		return errors.New("工作流实例不存在")
	}

	if inst.Status != "running" {
		return errors.New("工作流不在运行状态")
	}

	for i, step := range inst.Steps {
		if step.StepID == stepID {
			if step.Status != "in_progress" && step.Status != "pending" {
				return errors.New("步骤状态不允许跳过")
			}

			now := time.Now()
			inst.Steps[i].Status = "skipped"
			inst.Steps[i].CompletedAt = &now

			// 推进到下一步
			if i+1 < len(inst.Steps) {
				inst.CurrentStep = i + 1
				inst.Steps[i+1].Status = "in_progress"
				inst.Steps[i+1].StartedAt = &now
			} else {
				inst.Status = "completed"
				inst.CompletedAt = &now
			}

			we.engine.addAudit(inst.DocumentID, userID, "step_skip", "跳过步骤: "+step.Name, "", "")

			return nil
		}
	}

	return errors.New("步骤不存在")
}

// RetryStep 重试步骤.
func (we *WorkflowEngine) RetryStep(instanceID, stepID, userID string) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	inst, ok := we.instances[instanceID]
	if !ok {
		return errors.New("工作流实例不存在")
	}

	if inst.Status != "failed" {
		return errors.New("只能重试失败的工作流")
	}

	for i, step := range inst.Steps {
		if step.StepID == stepID {
			if step.Status != "completed" {
				return errors.New("只能重试已完成的步骤")
			}

			// 检查是否有拒绝
			hasReject := false
			for _, r := range step.Results {
				if r.Action == "reject" {
					hasReject = true
					break
				}
			}

			if !hasReject {
				return errors.New("步骤没有被拒绝")
			}

			// 重置步骤
			now := time.Now()
			inst.Steps[i].Status = "in_progress"
			inst.Steps[i].StartedAt = &now
			inst.Steps[i].CompletedAt = nil
			inst.Steps[i].Results = make([]StepResult, 0)
			inst.Status = "running"
			inst.CompletedAt = nil
			inst.CurrentStep = i

			we.engine.addAudit(inst.DocumentID, userID, "step_retry", "重试步骤: "+step.Name, "", "")

			return nil
		}
	}

	return errors.New("步骤不存在")
}

// GetStepProgress 获取步骤进度.
func (we *WorkflowEngine) GetStepProgress(instanceID string) (int, int, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	inst, ok := we.instances[instanceID]
	if !ok {
		return 0, 0, errors.New("工作流实例不存在")
	}

	completed := 0
	for _, step := range inst.Steps {
		if step.Status == "completed" || step.Status == "skipped" {
			completed++
		}
	}

	return completed, len(inst.Steps), nil
}

// CheckTimeout 检查超时.
func (we *WorkflowEngine) CheckTimeout(instanceID string) (bool, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	inst, ok := we.instances[instanceID]
	if !ok {
		return false, errors.New("工作流实例不存在")
	}

	if inst.Status != "running" {
		return false, nil
	}

	// 获取工作流配置
	wf, ok := we.workflows[inst.WorkflowID]
	if !ok {
		return false, nil
	}

	// 检查当前步骤是否超时
	if inst.CurrentStep < len(wf.Steps) {
		step := wf.Steps[inst.CurrentStep]
		if step.Timeout > 0 && inst.Steps[inst.CurrentStep].StartedAt != nil {
			elapsed := time.Since(*inst.Steps[inst.CurrentStep].StartedAt)
			if elapsed.Hours() > float64(step.Timeout) {
				return true, nil
			}
		}
	}

	return false, nil
}

// AutoExpire 自动过期.
func (we *WorkflowEngine) AutoExpire() int {
	we.mu.Lock()
	defer we.mu.Unlock()

	expired := 0
	now := time.Now()

	for _, inst := range we.instances {
		if inst.Status != "running" {
			continue
		}

		// 检查工作流配置
		wf, ok := we.workflows[inst.WorkflowID]
		if !ok {
			continue
		}

		if inst.CurrentStep < len(wf.Steps) {
			step := wf.Steps[inst.CurrentStep]
			if step.Timeout > 0 && inst.Steps[inst.CurrentStep].StartedAt != nil {
				elapsed := now.Sub(*inst.Steps[inst.CurrentStep].StartedAt)
				if elapsed.Hours() > float64(step.Timeout) {
					inst.Status = "failed"
					inst.CompletedAt = &now
					expired++

					we.engine.addAudit(inst.DocumentID, "", "workflow_timeout", "工作流超时", "", "")
				}
			}
		}
	}

	return expired
}
