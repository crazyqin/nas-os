// Package projectboard 提供项目看板管理功能。
// workflow.go 实现工作流引擎，包括状态流转和自动化规则。
package projectboard

import "time"

// WorkflowEngine 工作流引擎。
type WorkflowEngine struct {
	engine *Engine
}

// NewWorkflowEngine 创建工作流引擎。
func NewWorkflowEngine(engine *Engine) *WorkflowEngine {
	return &WorkflowEngine{engine: engine}
}

// CreateWorkflow 创建工作流。
func (w *WorkflowEngine) CreateWorkflow(projectID, name string, transitions []Transition, rules []AutomationRule) (*Workflow, error) {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	if _, exists := w.engine.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	workflow := &Workflow{
		ID:          generateID(),
		ProjectID:   projectID,
		Name:        name,
		Transitions: transitions,
		Rules:       rules,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	w.engine.workflows[workflow.ID] = workflow
	return workflow, nil
}

// GetWorkflow 获取工作流。
func (w *WorkflowEngine) GetWorkflow(id string) (*Workflow, error) {
	w.engine.mu.RLock()
	defer w.engine.mu.RUnlock()

	workflow, exists := w.engine.workflows[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	return workflow, nil
}

// ListWorkflows 列出工作流。
func (w *WorkflowEngine) ListWorkflows(projectID string) []*Workflow {
	w.engine.mu.RLock()
	defer w.engine.mu.RUnlock()

	result := make([]*Workflow, 0)
	for _, workflow := range w.engine.workflows {
		if workflow.ProjectID == projectID {
			result = append(result, workflow)
		}
	}
	return result
}

// DeleteWorkflow 删除工作流。
func (w *WorkflowEngine) DeleteWorkflow(id string) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	if _, exists := w.engine.workflows[id]; !exists {
		return ErrWorkflowNotFound
	}

	delete(w.engine.workflows, id)
	return nil
}

// ValidateTransition 验证状态转换是否有效。
func (w *WorkflowEngine) ValidateTransition(workflowID string, fromStatus, toStatus CardStatus) (bool, error) {
	w.engine.mu.RLock()
	defer w.engine.mu.RUnlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return false, ErrWorkflowNotFound
	}

	for _, t := range workflow.Transitions {
		if t.FromStatus == fromStatus && t.ToStatus == toStatus {
			return true, nil
		}
	}

	return false, nil
}

// ExecuteTransition 执行状态转换。
func (w *WorkflowEngine) ExecuteTransition(cardID, workflowID string, toStatus CardStatus) (*Card, error) {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	card, exists := w.engine.cards[cardID]
	if !exists {
		return nil, ErrCardNotFound
	}

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return nil, ErrWorkflowNotFound
	}

	// 查找有效的转换
	var transition *Transition
	for _, t := range workflow.Transitions {
		if t.FromStatus == card.Status && t.ToStatus == toStatus {
			transition = &t
			break
		}
	}

	if transition == nil {
		return nil, ErrInvalidTransition
	}

	// 执行转换
	card.Status = toStatus
	card.UpdatedAt = time.Now()

	// 自动更新进度
	switch toStatus {
	case CardStatusDone:
		card.Progress = 100
	case CardStatusBacklog:
		card.Progress = 0
	case CardStatusTodo:
		if card.Progress > 0 && card.Progress < 100 {
			// 保持当前进度
		}
	}

	// 执行自动化规则
	w.executeRules(workflow, card, "card_moved")

	return card, nil
}

// executeRules 执行自动化规则。
func (w *WorkflowEngine) executeRules(workflow *Workflow, card *Card, trigger string) {
	for _, rule := range workflow.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.Trigger != trigger {
			continue
		}

		// 简化的条件检查
		if w.evaluateCondition(rule.Condition, card) {
			w.executeActions(rule.Actions, card)
		}
	}
}

// evaluateCondition 评估条件（简化实现）。
func (w *WorkflowEngine) evaluateCondition(condition string, card *Card) bool {
	if condition == "" {
		return true
	}

	// 简化的条件评估
	switch condition {
	case "has_assignee":
		return card.AssigneeID != ""
	case "high_priority":
		return card.Priority == PriorityHigh || card.Priority == PriorityUrgent
	case "has_labels":
		return len(card.Labels) > 0
	default:
		return true
	}
}

// executeActions 执行动作。
func (w *WorkflowEngine) executeActions(actions []string, card *Card) {
	for _, action := range actions {
		switch action {
		case "set_progress_100":
			card.Progress = 100
		case "clear_assignee":
			card.AssigneeID = ""
		// 更多动作可以扩展
		}
	}
}

// GetAvailableTransitions 获取卡片可用的状态转换。
func (w *WorkflowEngine) GetAvailableTransitions(cardID, workflowID string) ([]CardStatus, error) {
	w.engine.mu.RLock()
	defer w.engine.mu.RUnlock()

	card, exists := w.engine.cards[cardID]
	if !exists {
		return nil, ErrCardNotFound
	}

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return nil, ErrWorkflowNotFound
	}

	result := make([]CardStatus, 0)
	for _, t := range workflow.Transitions {
		if t.FromStatus == card.Status {
			result = append(result, t.ToStatus)
		}
	}

	return result, nil
}

// AddTransition 添加状态转换。
func (w *WorkflowEngine) AddTransition(workflowID string, transition Transition) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Transitions = append(workflow.Transitions, transition)
	workflow.UpdatedAt = time.Now()
	return nil
}

// RemoveTransition 移除状态转换。
func (w *WorkflowEngine) RemoveTransition(workflowID, transitionID string) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	for i, t := range workflow.Transitions {
		if t.ID == transitionID {
			workflow.Transitions = append(workflow.Transitions[:i], workflow.Transitions[i+1:]...)
			workflow.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWorkflowNotFound
}

// AddAutomationRule 添加自动化规则。
func (w *WorkflowEngine) AddAutomationRule(workflowID string, rule AutomationRule) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Rules = append(workflow.Rules, rule)
	workflow.UpdatedAt = time.Now()
	return nil
}

// RemoveAutomationRule 移除自动化规则。
func (w *WorkflowEngine) RemoveAutomationRule(workflowID, ruleID string) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	for i, r := range workflow.Rules {
		if r.ID == ruleID {
			workflow.Rules = append(workflow.Rules[:i], workflow.Rules[i+1:]...)
			workflow.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWorkflowNotFound
}

// ToggleAutomationRule 切换自动化规则启用状态。
func (w *WorkflowEngine) ToggleAutomationRule(workflowID, ruleID string) error {
	w.engine.mu.Lock()
	defer w.engine.mu.Unlock()

	workflow, exists := w.engine.workflows[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	for i, r := range workflow.Rules {
		if r.ID == ruleID {
			workflow.Rules[i].Enabled = !workflow.Rules[i].Enabled
			workflow.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrWorkflowNotFound
}
