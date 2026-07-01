// Package agentworkflow 提供 AI 代理工作流功能
// 参考群晖 DSM Agent 2.0 的 agentic workflows，实现自然语言任务解析、
// 跨服务工作流编排、多步骤自动化、条件分支和任务状态管理
package agentworkflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("任务不存在")
	// ErrWorkflowNotFound 工作流不存在.
	ErrWorkflowNotFound = errors.New("工作流不存在")
	// ErrParseFailed 任务解析失败.
	ErrParseFailed = errors.New("任务解析失败")
	// ErrStepFailed 步骤执行失败.
	ErrStepFailed = errors.New("步骤执行失败")
	// ErrConditionNotMet 条件不满足.
	ErrConditionNotMet = errors.New("执行条件不满足")
	// ErrTaskAlreadyRunning 任务已在运行.
	ErrTaskAlreadyRunning = errors.New("任务已在运行")
	// ErrInvalidInput 输入无效.
	ErrInvalidInput = errors.New("输入无效")
)

// ========== Service 定义 ==========

// Service AI 代理工作流服务.
type Service struct {
	mu        sync.RWMutex
	tasks     map[string]*taskState
	templates map[string]*WorkflowTemplate
	rules     []IntentRule
}

// NewService 创建 AI 代理工作流服务.
func NewService() *Service {
	s := &Service{
		tasks:     make(map[string]*taskState),
		templates: make(map[string]*WorkflowTemplate),
	}
	s.initRules()
	s.initTemplates()
	return s
}

// initRules 初始化意图识别规则.
func (s *Service) initRules() {
	s.rules = []IntentRule{
		{
			Keywords: []string{"备份", "backup", "archive"},
			Intent:   "backup",
			WorkflowType: WorkflowSequential,
			Steps: []WorkflowStep{
				{
					ID:     "step-1",
					Order:  1,
					Name:   "扫描数据源",
					Service: "scanner",
					Action:  "scan",
					Status:  StepPending,
				},
				{
					ID:     "step-2",
					Order:  2,
					Name:   "创建备份任务",
					Service: "backup",
					Action:  "create",
					Status:  StepPending,
				},
				{
					ID:     "step-3",
					Order:  3,
					Name:   "执行备份",
					Service: "backup",
					Action:  "execute",
					Status:  StepPending,
				},
				{
					ID:     "step-4",
					Order:  4,
					Name:   "验证备份",
					Service: "backup",
					Action:  "verify",
					Status:  StepPending,
				},
			},
		},
		{
			Keywords: []string{"迁移", "migrate", "transfer"},
			Intent:   "migrate",
			WorkflowType: WorkflowConditional,
			Steps: []WorkflowStep{
				{
					ID:     "step-1",
					Order:  1,
					Name:   "评估迁移可行性",
					Service: "assessor",
					Action:  "assess",
					Status:  StepPending,
				},
				{
					ID:     "step-2",
					Order:  2,
					Name:   "生成迁移计划",
					Service: "planner",
					Action:  "plan",
					Status:  StepPending,
					Condition: &TaskCondition{
						Field:    "compatible",
						Operator: OpEquals,
						Value:    true,
					},
				},
				{
					ID:     "step-3",
					Order:  3,
					Name:   "执行迁移",
					Service: "migrator",
					Action:  "execute",
					Status:  StepPending,
					Condition: &TaskCondition{
						Field:    "planReady",
						Operator: OpEquals,
						Value:    true,
					},
				},
			},
		},
		{
			Keywords: []string{"监控", "monitor", "alert", "告警"},
			Intent:   "monitor",
			WorkflowType: WorkflowEvent,
			Steps: []WorkflowStep{
				{
					ID:     "step-1",
					Order:  1,
					Name:   "收集系统指标",
					Service: "metrics",
					Action:  "collect",
					Status:  StepPending,
				},
				{
					ID:     "step-2",
					Order:  2,
					Name:   "分析异常",
					Service: "analyzer",
					Action:  "analyze",
					Status:  StepPending,
				},
				{
					ID:     "step-3",
					Order:  3,
					Name:   "发送告警",
					Service: "notifier",
					Action:  "notify",
					Status:  StepPending,
					Condition: &TaskCondition{
						Field:    "hasAnomaly",
						Operator: OpEquals,
						Value:    true,
					},
				},
			},
		},
		{
			Keywords: []string{"快照", "snapshot", "snapshot"},
			Intent:   "snapshot",
			WorkflowType: WorkflowSequential,
			Steps: []WorkflowStep{
				{
					ID:     "step-1",
					Order:  1,
					Name:   "识别目标卷",
					Service: "storage",
					Action:  "identify",
					Status:  StepPending,
				},
				{
					ID:     "step-2",
					Order:  2,
					Name:   "创建快照",
					Service: "storage",
					Action:  "snapshot",
					Status:  StepPending,
				},
				{
					ID:     "step-3",
					Order:  3,
					Name:   "验证快照",
					Service: "storage",
					Action:  "verify",
					Status:  StepPending,
				},
			},
		},
		{
			Keywords: []string{"更新", "update", "upgrade", "升级"},
			Intent:   "update",
			WorkflowType: WorkflowConditional,
			Steps: []WorkflowStep{
				{
					ID:     "step-1",
					Order:  1,
					Name:   "检查可用更新",
					Service: "updater",
					Action:  "check",
					Status:  StepPending,
				},
				{
					ID:     "step-2",
					Order:  2,
					Name:   "预检",
					Service: "updater",
					Action:  "precheck",
					Status:  StepPending,
					Condition: &TaskCondition{
						Field:    "hasUpdate",
						Operator: OpEquals,
						Value:    true,
					},
				},
				{
					ID:     "step-3",
					Order:  3,
					Name:   "应用更新",
					Service: "updater",
					Action:  "apply",
					Status:  StepPending,
					Condition: &TaskCondition{
						Field:    "precheckPassed",
						Operator: OpEquals,
						Value:    true,
					},
				},
			},
		},
	}
}

// initTemplates 初始化预设工作流模板.
func (s *Service) initTemplates() {
	now := time.Now()

	s.templates["full-backup"] = &WorkflowTemplate{
		ID:          "full-backup",
		Name:        "完整备份工作流",
		Description:  "扫描→创建→执行→验证的完整备份流程",
		Type:        WorkflowSequential,
		Steps:       s.rules[0].Steps,
		CreatedAt:   now,
	}

	s.templates["system-migrate"] = &WorkflowTemplate{
		ID:          "system-migrate",
		Name:        "系统迁移工作流",
		Description:  "评估→计划→执行的条件分支迁移流程",
		Type:        WorkflowConditional,
		Steps:       s.rules[1].Steps,
		CreatedAt:   now,
	}
}

// ParseTask 解析自然语言任务.
func (s *Service) ParseTask(ctx context.Context, req *ParseTaskRequest) (*ParseTaskResult, error) {
	if strings.TrimSpace(req.Input) == "" {
		return nil, fmt.Errorf("%w: 输入不能为空", ErrInvalidInput)
	}

	taskID := uuid.New().String()
	now := time.Now()

	// 匹配意图规则
	intent, workflow, confidence := s.matchIntent(req.Input, taskID, now)

	if intent == "" {
		intent = "unknown"
		confidence = 0
	}

	task := &taskState{
		id:           taskID,
		nlInput:      req.Input,
		parsedIntent: intent,
		status:       TaskPending,
		progress:     0,
		priority:     req.Priority,
		createdAt:    now,
		tags:         req.Tags,
	}

	if workflow != nil {
		task.workflow = workflow
		task.workflowID = workflow.ID
		workflow.TaskID = taskID
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	var warnings []string
	if confidence < 0.5 && intent != "unknown" {
		warnings = append(warnings, "意图识别置信度较低，建议确认")
	}
	if intent == "unknown" {
		warnings = append(warnings, "无法识别任务意图，请提供更详细的描述")
	}

	return &ParseTaskResult{
		TaskID:       taskID,
		NLInput:      req.Input,
		ParsedIntent: intent,
		Workflow:     workflow,
		Confidence:   confidence,
		Warnings:     warnings,
	}, nil
}

// matchIntent 意图匹配（关键字匹配，实际可接入 NLP 模型）.
func (s *Service) matchIntent(input string, taskID string, now time.Time) (string, *Workflow, float64) {
	lowerInput := strings.ToLower(input)

	for _, rule := range s.rules {
		for _, kw := range rule.Keywords {
			if strings.Contains(lowerInput, strings.ToLower(kw)) {
				// 匹配成功，构建工作流
				steps := make([]WorkflowStep, len(rule.Steps))
				for i, step := range rule.Steps {
					steps[i] = WorkflowStep{
						ID:          uuid.New().String(),
						Order:       step.Order,
						Name:        step.Name,
						Description: step.Description,
						Service:     step.Service,
						Action:      step.Action,
						Parameters:  step.Parameters,
						Condition:   step.Condition,
						OnSuccess:   step.OnSuccess,
						OnFailure:   step.OnFailure,
						Status:      StepPending,
					}
				}

				workflow := &Workflow{
					ID:        uuid.New().String(),
					Name:      fmt.Sprintf("%s 工作流", rule.Intent),
					Type:      rule.WorkflowType,
					Steps:     steps,
					TaskID:    taskID,
					Status:    TaskPending,
					CreatedAt: now,
					UpdatedAt: now,
				}

				return rule.Intent, workflow, 0.85
			}
		}
	}

	return "", nil, 0
}

// ExecuteWorkflow 执行工作流.
func (s *Service) ExecuteWorkflow(ctx context.Context, req *ExecuteWorkflowRequest) (*ExecuteWorkflowResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.workflow == nil {
		return nil, fmt.Errorf("%w: 任务 %s 没有关联的工作流", ErrWorkflowNotFound, req.TaskID)
	}

	if task.status == TaskRunning {
		return nil, fmt.Errorf("%w: 任务 %s 已在运行", ErrTaskAlreadyRunning, req.TaskID)
	}

	now := time.Now()
	task.status = TaskRunning
	task.startedAt = now

	// 初始化执行上下文
	execCtx := &ExecutionContext{
		TaskID:      req.TaskID,
		WorkflowID:  task.workflow.ID,
		Variables:   req.Variables,
		StepOutput:  make(map[string]map[string]any),
		StartedAt:    now,
		CurrentStep: 0,
	}
	if execCtx.Variables == nil {
		execCtx.Variables = make(map[string]any)
	}
	task.context = execCtx

	wf := task.workflow
	wf.Status = TaskRunning

	completedCount := 0
	totalSteps := len(wf.Steps)
	var execError string

	for i := range wf.Steps {
		step := &wf.Steps[i]
		execCtx.CurrentStep = i

		// 检查条件
		if step.Condition != nil {
			ok, err := s.evaluateCondition(step.Condition, execCtx)
			if err != nil {
				step.Status = StepFailed
				step.Error = err.Error()
				execError = fmt.Sprintf("步骤 %s 条件评估失败: %v", step.Name, err)
				break
			}
			if !ok {
				step.Status = StepSkipped
				step.FinishedAt = time.Now()
				completedCount++
				continue
			}
		}

		step.Status = StepRunning
		step.StartedAt = time.Now()

		if req.DryRun {
			// 模拟执行
			step.Status = StepCompleted
			step.FinishedAt = time.Now()
			step.Output = map[string]any{
				"simulated": true,
				"service":   step.Service,
				"action":    step.Action,
			}
			execCtx.StepOutput[step.ID] = step.Output
			completedCount++
			continue
		}

		// 模拟实际执行（实际应调用对应服务的 API）
		step.Status = StepCompleted
		step.FinishedAt = time.Now()
		step.Output = map[string]any{
			"executed": true,
			"service":  step.Service,
			"action":   step.Action,
		}
		execCtx.StepOutput[step.ID] = step.Output
		completedCount++
	}

	progress := float64(completedCount) / float64(totalSteps) * 100
	if totalSteps == 0 {
		progress = 0
	}

	if execError != "" {
		task.status = TaskFailed
		task.error = execError
		wf.Status = TaskFailed
	} else {
		task.status = TaskCompleted
		wf.Status = TaskCompleted
	}

	task.progress = progress
	task.finishedAt = time.Now()
	wf.UpdatedAt = time.Now()

	return &ExecuteWorkflowResult{
		TaskID:     req.TaskID,
		WorkflowID: wf.ID,
		Status:     task.status,
		Progress:   progress,
		Steps:      wf.Steps,
		StartedAt:  task.startedAt,
		FinishedAt: task.finishedAt,
		Error:      execError,
	}, nil
}

// CancelTask 取消任务.
func (s *Service) CancelTask(ctx context.Context, req *CancelTaskRequest) (*TaskStatusResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.status == TaskCompleted {
		return nil, fmt.Errorf("任务已完成，无法取消")
	}
	if task.status == TaskCancelled {
		return nil, fmt.Errorf("任务已取消")
	}

	task.status = TaskCancelled
	if req.Reason != "" {
		task.error = req.Reason
	}

	if task.workflow != nil {
		// 将运行中的步骤标记为失败
		for i := range task.workflow.Steps {
			step := &task.workflow.Steps[i]
			if step.Status == StepRunning || step.Status == StepPending {
				step.Status = StepFailed
				step.Error = "任务已取消"
			}
		}
		task.workflow.Status = TaskCancelled
		task.workflow.UpdatedAt = time.Now()
	}

	return s.toStatusResponse(task), nil
}

// GetTaskStatus 获取任务状态.
func (s *Service) GetTaskStatus(taskID string) (*TaskStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return s.toStatusResponse(task), nil
}

// ListTasks 列出所有任务.
func (s *Service) ListTasks() ([]AgentTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AgentTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, s.toAgentTask(task))
	}
	return result, nil
}

// GetTemplates 获取工作流模板列表.
func (s *Service) GetTemplates() ([]WorkflowTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]WorkflowTemplate, 0, len(s.templates))
	for _, tpl := range s.templates {
		result = append(result, *tpl)
	}
	return result, nil
}

// CreateWorkflowFromTemplate 从模板创建工作流.
func (s *Service) CreateWorkflowFromTemplate(ctx context.Context, templateID string, taskID string) (*Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tpl, ok := s.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	now := time.Now()
	steps := make([]WorkflowStep, len(tpl.Steps))
	for i, step := range tpl.Steps {
		steps[i] = WorkflowStep{
			ID:          uuid.New().String(),
			Order:       step.Order,
			Name:        step.Name,
			Description: step.Description,
			Service:     step.Service,
			Action:      step.Action,
			Parameters:  step.Parameters,
			Condition:   step.Condition,
			Status:      StepPending,
		}
	}

	workflow := &Workflow{
		ID:        uuid.New().String(),
		Name:      tpl.Name,
		Description: tpl.Description,
		Type:      tpl.Type,
		Steps:     steps,
		TaskID:     taskID,
		Status:    TaskPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	task.workflow = workflow
	task.workflowID = workflow.ID

	return workflow, nil
}

// evaluateCondition 评估步骤执行条件.
func (s *Service) evaluateCondition(cond *TaskCondition, execCtx *ExecutionContext) (bool, error) {
	if cond == nil {
		return true, nil
	}

	// 从上下文变量或步骤输出中获取字段值
	fieldValue, exists := execCtx.Variables[cond.Field]
	if !exists {
		// 尝试从步骤输出中查找
		for _, output := range execCtx.StepOutput {
			if v, ok := output[cond.Field]; ok {
				fieldValue = v
				exists = true
				break
			}
		}
	}

	if !exists {
		// 字段不存在，条件不满足（不报错，视为 false）
		return false, nil
	}

	return compareValues(fieldValue, cond.Operator, cond.Value)
}

// compareValues 比较两个值.
func compareValues(actual any, op ConditionOperator, expected any) (bool, error) {
	switch op {
	case OpEquals:
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected), nil
	case OpNotEquals:
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected), nil
	case OpGreaterThan:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a > b })
	case OpLessThan:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a < b })
	case OpGreaterEq:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a >= b })
	case OpLessEq:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a <= b })
	case OpContains:
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected)), nil
	case OpIn:
		// expected 应为列表
		actualStr := fmt.Sprintf("%v", actual)
		switch expList := expected.(type) {
		case []any:
			for _, item := range expList {
				if fmt.Sprintf("%v", item) == actualStr {
					return true, nil
				}
			}
			return false, nil
		case []string:
			for _, item := range expList {
				if item == actualStr {
					return true, nil
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("OpIn 期望列表类型值")
		}
	case OpNotIn:
		result, err := compareValues(actual, OpIn, expected)
		return !result, err
	default:
		return false, fmt.Errorf("不支持的操作符: %s", op)
	}
}

// compareNumeric 数值比较辅助函数.
func compareNumeric(actual, expected any, cmp func(a, b float64) bool) (bool, error) {
	a, ok := toFloat64(actual)
	if !ok {
		return false, fmt.Errorf("无法将实际值 %v 转为数值", actual)
	}
	b, ok := toFloat64(expected)
	if !ok {
		return false, fmt.Errorf("无法将期望值 %v 转为数值", expected)
	}
	return cmp(a, b), nil
}

// toFloat64 将任意值转为 float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// toAgentTask 转为对外 AgentTask.
func (s *Service) toAgentTask(task *taskState) AgentTask {
	return AgentTask{
		ID:           task.id,
		NLInput:      task.nlInput,
		ParsedIntent: task.parsedIntent,
		WorkflowID:   task.workflowID,
		Status:       task.status,
		Progress:     task.progress,
		Priority:     task.priority,
		CreatedAt:    task.createdAt,
		StartedAt:    task.startedAt,
		FinishedAt:   task.finishedAt,
		Error:        task.error,
		Result:       task.result,
		Tags:         task.tags,
	}
}

// toStatusResponse 转为状态响应.
func (s *Service) toStatusResponse(task *taskState) *TaskStatusResponse {
	updated := task.createdAt
	if !task.finishedAt.IsZero() {
		updated = task.finishedAt
	} else if !task.startedAt.IsZero() {
		updated = task.startedAt
	}

	return &TaskStatusResponse{
		TaskID:    task.id,
		Status:    task.status,
		Progress:  task.progress,
		Workflow:  task.workflow,
		Error:     task.error,
		CreatedAt: task.createdAt,
		UpdatedAt: updated,
	}
}