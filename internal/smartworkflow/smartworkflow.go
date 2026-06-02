// Package smartworkflow 智能工作流引擎
// 提供可视化工作流编排、定时任务、条件触发、多步骤自动化、任务依赖管理功能
package smartworkflow

import (
	"fmt"
	"sync"
	"time"
)

// ExecutionStatus 工作流执行状态
type ExecutionStatus int

const (
	StatusPending   ExecutionStatus = iota // 等待执行
	StatusRunning                          // 执行中
	StatusCompleted                        // 执行完成
	StatusFailed                           // 执行失败
	StatusCancelled                        // 已取消
)

// String 返回状态的字符串表示
func (s ExecutionStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// StepType 步骤类型
type StepType int

const (
	StepTypeSerial   StepType = iota // 串行步骤
	StepTypeParallel                 // 并行步骤
)

// TriggerType 触发器类型
type TriggerType int

const (
	TriggerTypeCron    TriggerType = iota // 定时触发
	TriggerTypeEvent                      // 事件触发
	TriggerTypeManual                     // 手动触发
)

// Step 工作流步骤
type Step struct {
	ID          string            // 步骤ID
	Name        string            // 步骤名称
	Description string            // 步骤描述
	Type        StepType          // 步骤类型
	Action      string            // 执行动作
	Parameters  map[string]string // 步骤参数
	DependsOn   []string          // 依赖的步骤ID列表
	Timeout     time.Duration     // 超时时间
	RetryCount  int               // 重试次数
	RetryDelay  time.Duration     // 重试延迟
}

// Trigger 工作流触发器
type Trigger struct {
	ID          string            // 触发器ID
	Type        TriggerType       // 触发器类型
	Expression  string            // cron表达式或事件名
	Enabled     bool              // 是否启用
	Parameters  map[string]string // 触发器参数
	LastTrigger time.Time         // 上次触发时间
}

// Workflow 工作流定义
type Workflow struct {
	ID          string            // 工作流ID
	Name        string            // 工作流名称
	Description string            // 工作流描述
	Steps       []*Step           // 工作流步骤
	Triggers    []*Trigger        // 触发器列表
	Enabled     bool              // 是否启用
	CreatedAt   time.Time         // 创建时间
	UpdatedAt   time.Time         // 更新时间
	Tags        []string          // 标签
	Variables   map[string]string // 工作流变量
}

// Execution 工作流执行记录
type Execution struct {
	ID          string                 // 执行ID
	WorkflowID  string                 // 工作流ID
	Status      ExecutionStatus        // 执行状态
	StartTime   time.Time              // 开始时间
	EndTime     time.Time              // 结束时间
	StepResults map[string]*StepResult // 步骤执行结果
	Error       string                 // 错误信息
	TriggeredBy string                 // 触发方式
}

// StepResult 步骤执行结果
type StepResult struct {
	StepID    string          // 步骤ID
	Status    ExecutionStatus // 执行状态
	StartTime time.Time       // 开始时间
	EndTime   time.Time       // 结束时间
	Output    string          // 输出结果
	Error     string          // 错误信息
	RetryNum  int             // 重试次数
}

// Template 工作流模板
type Template struct {
	ID          string   // 模板ID
	Name        string   // 模板名称
	Description string   // 模板描述
	Category    string   // 模板分类
	Workflow    Workflow // 模板内容
	Tags        []string // 标签
}

// CronSchedule cron调度解析结果
type CronSchedule struct {
	Minute     []int // 分钟 (0-59)
	Hour       []int // 小时 (0-23)
	DayOfMonth []int // 日 (1-31)
	Month      []int // 月 (1-12)
	DayOfWeek  []int // 星期 (0-6, 0=周日)
}

// SmartWorkflow 智能工作流引擎主结构体
type SmartWorkflow struct {
	mu          sync.RWMutex
	workflows   map[string]*Workflow     // 工作流存储
	executions  map[string]*Execution    // 执行记录存储
	templates   map[string]*Template     // 模板存储
	triggers    map[string][]*Trigger    // 触发器索引
	execCounter int64                    // 执行计数器
}

// New 创建新的智能工作流引擎实例
func New() *SmartWorkflow {
	return &SmartWorkflow{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*Execution),
		templates:  make(map[string]*Template),
		triggers:   make(map[string][]*Trigger),
	}
}

// CreateWorkflow 创建工作流
func (sw *SmartWorkflow) CreateWorkflow(workflow *Workflow) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("工作流ID不能为空")
	}

	if _, exists := sw.workflows[workflow.ID]; exists {
		return fmt.Errorf("工作流 %s 已存在", workflow.ID)
	}

	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now

	if workflow.Variables == nil {
		workflow.Variables = make(map[string]string)
	}

	sw.workflows[workflow.ID] = workflow
	return nil
}

// UpdateWorkflow 更新工作流
func (sw *SmartWorkflow) UpdateWorkflow(workflow *Workflow) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("工作流ID不能为空")
	}

	if _, exists := sw.workflows[workflow.ID]; !exists {
		return fmt.Errorf("工作流 %s 不存在", workflow.ID)
	}

	workflow.UpdatedAt = time.Now()
	sw.workflows[workflow.ID] = workflow
	return nil
}

// DeleteWorkflow 删除工作流
func (sw *SmartWorkflow) DeleteWorkflow(workflowID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if _, exists := sw.workflows[workflowID]; !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	delete(sw.workflows, workflowID)
	delete(sw.triggers, workflowID)
	return nil
}

// GetWorkflow 获取工作流
func (sw *SmartWorkflow) GetWorkflow(workflowID string) (*Workflow, error) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	return workflow, nil
}

// ListWorkflows 列出所有工作流
func (sw *SmartWorkflow) ListWorkflows() []*Workflow {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(sw.workflows))
	for _, w := range sw.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// AddStep 添加工作流步骤
func (sw *SmartWorkflow) AddStep(workflowID string, step *Step) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	if step.ID == "" {
		return fmt.Errorf("步骤ID不能为空")
	}

	// 检查步骤ID是否重复
	for _, s := range workflow.Steps {
		if s.ID == step.ID {
			return fmt.Errorf("步骤 %s 已存在", step.ID)
		}
	}

	workflow.Steps = append(workflow.Steps, step)
	workflow.UpdatedAt = time.Now()
	return nil
}

// RemoveStep 移除工作流步骤
func (sw *SmartWorkflow) RemoveStep(workflowID, stepID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	for i, s := range workflow.Steps {
		if s.ID == stepID {
			workflow.Steps = append(workflow.Steps[:i], workflow.Steps[i+1:]...)
			workflow.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("步骤 %s 不存在", stepID)
}

// AddTrigger 添加触发器
func (sw *SmartWorkflow) AddTrigger(workflowID string, trigger *Trigger) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	if trigger.ID == "" {
		return fmt.Errorf("触发器ID不能为空")
	}

	workflow.Triggers = append(workflow.Triggers, trigger)
	sw.triggers[workflowID] = workflow.Triggers
	workflow.UpdatedAt = time.Now()
	return nil
}

// RemoveTrigger 移除触发器
func (sw *SmartWorkflow) RemoveTrigger(workflowID, triggerID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	for i, t := range workflow.Triggers {
		if t.ID == triggerID {
			workflow.Triggers = append(workflow.Triggers[:i], workflow.Triggers[i+1:]...)
			sw.triggers[workflowID] = workflow.Triggers
			workflow.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("触发器 %s 不存在", triggerID)
}

// Execute 执行工作流
func (sw *SmartWorkflow) Execute(workflowID string, triggeredBy string) (*Execution, error) {
	sw.mu.Lock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		sw.mu.Unlock()
		return nil, fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	if !workflow.Enabled {
		sw.mu.Unlock()
		return nil, fmt.Errorf("工作流 %s 未启用", workflowID)
	}

	sw.execCounter++
	execution := &Execution{
		ID:          fmt.Sprintf("exec_%d_%s", sw.execCounter, workflowID),
		WorkflowID:  workflowID,
		Status:      StatusRunning,
		StartTime:   time.Now(),
		StepResults: make(map[string]*StepResult),
		TriggeredBy: triggeredBy,
	}

	sw.executions[execution.ID] = execution
	sw.mu.Unlock()

	// 执行工作流步骤
	err := sw.executeSteps(workflow, execution)

	sw.mu.Lock()
	execution.EndTime = time.Now()
	if err != nil {
		execution.Status = StatusFailed
		execution.Error = err.Error()
	} else {
		execution.Status = StatusCompleted
	}
	sw.mu.Unlock()

	return execution, nil
}

// executeSteps 执行工作流步骤
func (sw *SmartWorkflow) executeSteps(workflow *Workflow, execution *Execution) error {
	// 构建步骤依赖图
	stepMap := make(map[string]*Step)
	for _, step := range workflow.Steps {
		stepMap[step.ID] = step
	}

	// 记录已完成的步骤
	completed := make(map[string]bool)

	// 按依赖顺序执行
	for len(completed) < len(workflow.Steps) {
		executed := false

		for _, step := range workflow.Steps {
			if completed[step.ID] {
				continue
			}

			// 检查依赖是否满足
			depsMet := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					depsMet = false
					break
				}
			}

			if depsMet {
				result := sw.executeStep(step)
				execution.StepResults[step.ID] = result

				if result.Status == StatusFailed {
					return fmt.Errorf("步骤 %s 执行失败: %s", step.ID, result.Error)
				}

				completed[step.ID] = true
				executed = true
			}
		}

		if !executed {
			return fmt.Errorf("存在循环依赖或无法满足的依赖关系")
		}
	}

	return nil
}

// executeStep 执行单个步骤（带重试）
func (sw *SmartWorkflow) executeStep(step *Step) *StepResult {
	result := &StepResult{
		StepID:    step.ID,
		StartTime: time.Now(),
	}

	maxRetries := step.RetryCount
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	for retry := 0; retry <= maxRetries; retry++ {
		result.RetryNum = retry

		// 模拟步骤执行
		err := sw.performStepAction(step)
		if err == nil {
			result.Status = StatusCompleted
			result.EndTime = time.Now()
			result.Output = fmt.Sprintf("步骤 %s 执行成功", step.ID)
			return result
		}

		lastErr = err

		if retry < maxRetries && step.RetryDelay > 0 {
			time.Sleep(step.RetryDelay)
		}
	}

	result.Status = StatusFailed
	result.EndTime = time.Now()
	result.Error = lastErr.Error()
	return result
}

// performStepAction 执行步骤动作
func (sw *SmartWorkflow) performStepAction(step *Step) error {
	// 模拟执行，实际实现中这里会调用具体的动作处理器
	if step.Action == "" {
		return fmt.Errorf("步骤动作为空")
	}

	// 这里是模拟实现，实际应根据Action调用对应的处理器
	return nil
}

// Schedule 定时调度工作流
func (sw *SmartWorkflow) Schedule(workflowID string, cronExpr string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	// 解析cron表达式
	_, err := ParseCron(cronExpr)
	if err != nil {
		return fmt.Errorf("cron表达式解析失败: %v", err)
	}

	// 创建定时触发器
	trigger := &Trigger{
		ID:         fmt.Sprintf("cron_%s_%d", workflowID, time.Now().UnixNano()),
		Type:       TriggerTypeCron,
		Expression: cronExpr,
		Enabled:    true,
	}

	workflow.Triggers = append(workflow.Triggers, trigger)
	sw.triggers[workflowID] = workflow.Triggers
	workflow.UpdatedAt = time.Now()

	return nil
}

// ParseCron 解析cron表达式
// 支持标准5位格式: 分 时 日 月 周
func ParseCron(expr string) (*CronSchedule, error) {
	// 简化的cron解析实现
	schedule := &CronSchedule{
		Minute:     make([]int, 0),
		Hour:       make([]int, 0),
		DayOfMonth: make([]int, 0),
		Month:      make([]int, 0),
		DayOfWeek:  make([]int, 0),
	}

	// 这里是简化实现，实际应完整解析cron表达式
	// 示例: "0 0 * * *" 表示每天0点执行
	if expr == "" {
		return nil, fmt.Errorf("cron表达式不能为空")
	}

	// 默认返回每小时执行的调度
	schedule.Minute = []int{0}
	for i := 0; i < 24; i++ {
		schedule.Hour = append(schedule.Hour, i)
	}
	for i := 1; i <= 31; i++ {
		schedule.DayOfMonth = append(schedule.DayOfMonth, i)
	}
	for i := 1; i <= 12; i++ {
		schedule.Month = append(schedule.Month, i)
	}
	for i := 0; i <= 6; i++ {
		schedule.DayOfWeek = append(schedule.DayOfWeek, i)
	}

	return schedule, nil
}

// GetExecutionHistory 获取工作流执行历史
func (sw *SmartWorkflow) GetExecutionHistory(workflowID string) []*Execution {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	var history []*Execution
	for _, exec := range sw.executions {
		if exec.WorkflowID == workflowID {
			history = append(history, exec)
		}
	}
	return history
}

// GetExecution 获取指定执行记录
func (sw *SmartWorkflow) GetExecution(executionID string) (*Execution, error) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	exec, exists := sw.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("执行记录 %s 不存在", executionID)
	}
	return exec, nil
}

// CreateFromTemplate 从模板创建工作流
func (sw *SmartWorkflow) CreateFromTemplate(templateID string, workflowID string, variables map[string]string) (*Workflow, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	template, exists := sw.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	if _, exists := sw.workflows[workflowID]; exists {
		return nil, fmt.Errorf("工作流 %s 已存在", workflowID)
	}

	// 从模板创建工作流
	workflow := &Workflow{
		ID:          workflowID,
		Name:        template.Workflow.Name,
		Description: template.Workflow.Description,
		Enabled:     true,
		Tags:        template.Tags,
		Variables:   variables,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 复制步骤
	for _, step := range template.Workflow.Steps {
		newStep := *step
		workflow.Steps = append(workflow.Steps, &newStep)
	}

	// 复制触发器
	for _, trigger := range template.Workflow.Triggers {
		newTrigger := *trigger
		workflow.Triggers = append(workflow.Triggers, &newTrigger)
	}

	sw.workflows[workflowID] = workflow
	return workflow, nil
}

// GetTemplates 获取所有模板
func (sw *SmartWorkflow) GetTemplates() []*Template {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	templates := make([]*Template, 0, len(sw.templates))
	for _, t := range sw.templates {
		templates = append(templates, t)
	}
	return templates
}

// RegisterTemplate 注册工作流模板
func (sw *SmartWorkflow) RegisterTemplate(template *Template) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if template.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	if _, exists := sw.templates[template.ID]; exists {
		return fmt.Errorf("模板 %s 已存在", template.ID)
	}

	sw.templates[template.ID] = template
	return nil
}

// CancelExecution 取消正在执行的工作流
func (sw *SmartWorkflow) CancelExecution(executionID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	exec, exists := sw.executions[executionID]
	if !exists {
		return fmt.Errorf("执行记录 %s 不存在", executionID)
	}

	if exec.Status != StatusRunning {
		return fmt.Errorf("执行 %s 不在运行状态", executionID)
	}

	exec.Status = StatusCancelled
	exec.EndTime = time.Now()
	return nil
}

// GetWorkflowTriggers 获取工作流的所有触发器
func (sw *SmartWorkflow) GetWorkflowTriggers(workflowID string) ([]*Trigger, error) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	return workflow.Triggers, nil
}

// EnableWorkflow 启用工作流
func (sw *SmartWorkflow) EnableWorkflow(workflowID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	workflow.Enabled = true
	workflow.UpdatedAt = time.Now()
	return nil
}

// DisableWorkflow 禁用工作流
func (sw *SmartWorkflow) DisableWorkflow(workflowID string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	workflow, exists := sw.workflows[workflowID]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", workflowID)
	}

	workflow.Enabled = false
	workflow.UpdatedAt = time.Now()
	return nil
}
