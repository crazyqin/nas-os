// Package taskcenter 实现任务调度中心
// 学习群晖 Active Backup 高级调度功能，提供任务调度、依赖管理、执行监控
package taskcenter

import (
	"fmt"
	"sync"
	"time"
)

// TaskType 任务类型.
type TaskType string

const (
	// TaskTypeBackup 备份任务.
	TaskTypeBackup TaskType = "backup"
	// TaskTypeSync 同步任务.
	TaskTypeSync TaskType = "sync"
	// TaskTypeClean 清理任务.
	TaskTypeClean TaskType = "clean"
	// TaskTypeScan 扫描任务.
	TaskTypeScan TaskType = "scan"
	// TaskTypeReport 报告任务.
	TaskTypeReport TaskType = "report"
	// TaskTypeCustom 自定义任务.
	TaskTypeCustom TaskType = "custom"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 待执行.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 执行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusSkipped 已跳过.
	TaskStatusSkipped TaskStatus = "skipped"
	// TaskStatusWaiting 等待依赖.
	TaskStatusWaiting TaskStatus = "waiting"
)

// TaskPriority 任务优先级.
type TaskPriority int

const (
	// TaskPriorityLow 低优先级.
	TaskPriorityLow TaskPriority = 1
	// TaskPriorityNormal 普通优先级.
	TaskPriorityNormal TaskPriority = 5
	// TaskPriorityHigh 高优先级.
	TaskPriorityHigh TaskPriority = 10
	// TaskPriorityCritical 紧急优先级.
	TaskPriorityCritical TaskPriority = 15
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleTypeOnce 一次性.
	ScheduleTypeOnce ScheduleType = "once"
	// ScheduleTypeRecurring 重复执行.
	ScheduleTypeRecurring ScheduleType = "recurring"
	// ScheduleTypeCron Cron表达式.
	ScheduleTypeCron ScheduleType = "cron"
	// ScheduleTypeEvent 事件触发.
	ScheduleTypeEvent ScheduleType = "event"
)

// Task 任务.
type Task struct {
	// ID 任务ID
	ID string `json:"id"`
	// Name 名称
	Name string `json:"name"`
	// Description 描述
	Description string `json:"description"`
	// Type 类型
	Type TaskType `json:"type"`
	// Status 状态
	Status TaskStatus `json:"status"`
	// Priority 优先级
	Priority TaskPriority `json:"priority"`
	// Schedule 调度配置
	Schedule ScheduleConfig `json:"schedule"`
	// Dependencies 依赖任务ID列表
	Dependencies []string `json:"dependencies"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"maxRetries"`
	// RetryCount 当前重试次数
	RetryCount int `json:"retryCount"`
	// Timeout 超时时间
	Timeout time.Duration `json:"timeout"`
	// Parameters 参数
	Parameters map[string]interface{} `json:"parameters"`
	// Result 结果
	Result TaskResult `json:"result"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"startedAt,omitempty"`
	// CompletedAt 完成时间
	CompletedAt time.Time `json:"completedAt,omitempty"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// ScheduleConfig 调度配置.
type ScheduleConfig struct {
	// Type 调度类型
	Type ScheduleType `json:"type"`
	// CronExpr Cron表达式
	CronExpr string `json:"cronExpr,omitempty"`
	// Interval 间隔
	Interval time.Duration `json:"interval,omitempty"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime,omitempty"`
	// EndTime 结束时间
	EndTime time.Time `json:"endTime,omitempty"`
	// MaxRuns 最大执行次数
	MaxRuns int `json:"maxRuns"`
	// RunCount 已执行次数
	RunCount int `json:"runCount"`
	// EventType 事件类型
	EventType string `json:"eventType,omitempty"`
	// TimeZone 时区
	TimeZone string `json:"timeZone,omitempty"`
}

// TaskResult 任务结果.
type TaskResult struct {
	// Success 是否成功
	Success bool `json:"success"`
	// Message 消息
	Message string `json:"message"`
	// Output 输出
	Output string `json:"output"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Duration 持续时间
	Duration time.Duration `json:"duration"`
	// Artifacts 产物
	Artifacts []string `json:"artifacts,omitempty"`
	// Metrics 指标
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// TaskLog 任务日志.
type TaskLog struct {
	// ID 日志ID
	ID string `json:"id"`
	// TaskID 任务ID
	TaskID string `json:"taskId"`
	// Level 级别
	Level string `json:"level"`
	// Message 消息
	Message string `json:"message"`
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// Details 详情
	Details map[string]interface{} `json:"details,omitempty"`
}

// TaskExecution 任务执行记录.
type TaskExecution struct {
	// ID 执行ID
	ID string `json:"id"`
	// TaskID 任务ID
	TaskID string `json:"taskId"`
	// Status 状态
	Status TaskStatus `json:"status"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间
	EndTime time.Time `json:"endTime"`
	// Duration 持续时间
	Duration time.Duration `json:"duration"`
	// Result 结果
	Result TaskResult `json:"result"`
	// Logs 日志
	Logs []TaskLog `json:"logs"`
	// RetryCount 重试次数
	RetryCount int `json:"retryCount"`
}

// TaskGroup 任务组.
type TaskGroup struct {
	// ID 组ID
	ID string `json:"id"`
	// Name 名称
	Name string `json:"name"`
	// Description 描述
	Description string `json:"description"`
	// Tasks 任务ID列表
	Tasks []string `json:"tasks"`
	// Parallel 是否并行执行
	Parallel bool `json:"parallel"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
}

// TaskTemplate 任务模板.
type TaskTemplate struct {
	// ID 模板ID
	ID string `json:"id"`
	// Name 名称
	Name string `json:"name"`
	// Description 描述
	Description string `json:"description"`
	// Type 类型
	Type TaskType `json:"type"`
	// DefaultParameters 默认参数
	DefaultParameters map[string]interface{} `json:"defaultParameters"`
	// DefaultSchedule 默认调度
	DefaultSchedule ScheduleConfig `json:"defaultSchedule"`
	// Category 分类
	Category string `json:"category"`
}

// TaskCenter 任务中心.
type TaskCenter struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
	executions map[string]*TaskExecution
	groups     map[string]*TaskGroup
	templates  map[string]*TaskTemplate
	logs       []TaskLog
}

// NewTaskCenter 创建任务中心.
func NewTaskCenter() *TaskCenter {
	return &TaskCenter{
		tasks:      make(map[string]*Task),
		executions: make(map[string]*TaskExecution),
		groups:     make(map[string]*TaskGroup),
		templates:  make(map[string]*TaskTemplate),
	}
}

// CreateTask 创建任务.
func (tc *TaskCenter) CreateTask(task Task) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	task.Status = TaskStatusPending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	tc.tasks[task.ID] = &task
	return nil
}

// UpdateTask 更新任务.
func (tc *TaskCenter) UpdateTask(task Task) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	existing, ok := tc.tasks[task.ID]
	if !ok {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now()
	tc.tasks[task.ID] = &task
	return nil
}

// DeleteTask 删除任务.
func (tc *TaskCenter) DeleteTask(taskID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	delete(tc.tasks, taskID)
	return nil
}

// GetTask 获取任务.
func (tc *TaskCenter) GetTask(taskID string) (*Task, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	task, ok := tc.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks 列出任务.
func (tc *TaskCenter) ListTasks(taskType TaskType, status TaskStatus) []*Task {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range tc.tasks {
		if (taskType == "" || task.Type == taskType) &&
			(status == "" || task.Status == status) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// ExecuteTask 执行任务.
func (tc *TaskCenter) ExecuteTask(taskID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	task, ok := tc.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// 检查依赖
	for _, depID := range task.Dependencies {
		dep, ok := tc.tasks[depID]
		if !ok {
			return fmt.Errorf("dependency not found: %s", depID)
		}
		if dep.Status != TaskStatusCompleted {
			return fmt.Errorf("dependency not completed: %s", depID)
		}
	}

	task.Status = TaskStatusRunning
	task.StartedAt = time.Now()
	task.UpdatedAt = time.Now()

	// 创建执行记录
	execution := &TaskExecution{
		ID:        fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		TaskID:    taskID,
		Status:    TaskStatusRunning,
		StartTime: time.Now(),
	}
	tc.executions[execution.ID] = execution

	return nil
}

// CompleteTask 完成任务.
func (tc *TaskCenter) CompleteTask(taskID string, result TaskResult) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	task, ok := tc.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if result.Success {
		task.Status = TaskStatusCompleted
	} else {
		task.Status = TaskStatusFailed
	}
	task.Result = result
	task.CompletedAt = time.Now()
	task.UpdatedAt = time.Now()

	// 更新执行记录
	for _, exec := range tc.executions {
		if exec.TaskID == taskID && exec.Status == TaskStatusRunning {
			exec.Status = task.Status
			exec.EndTime = time.Now()
			exec.Duration = exec.EndTime.Sub(exec.StartTime)
			exec.Result = result
			break
		}
	}

	return nil
}

// CancelTask 取消任务.
func (tc *TaskCenter) CancelTask(taskID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	task, ok := tc.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = TaskStatusCancelled
	task.UpdatedAt = time.Now()
	return nil
}

// RetryTask 重试任务.
func (tc *TaskCenter) RetryTask(taskID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	task, ok := tc.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.RetryCount >= task.MaxRetries {
		return fmt.Errorf("max retries exceeded: %d/%d", task.RetryCount, task.MaxRetries)
	}

	task.RetryCount++
	task.Status = TaskStatusPending
	task.UpdatedAt = time.Now()
	return nil
}

// GetExecution 获取执行记录.
func (tc *TaskCenter) GetExecution(execID string) (*TaskExecution, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	exec, ok := tc.executions[execID]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", execID)
	}

	return exec, nil
}

// ListExecutions 列出执行记录.
func (tc *TaskCenter) ListExecutions(taskID string) []*TaskExecution {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	executions := make([]*TaskExecution, 0)
	for _, exec := range tc.executions {
		if taskID == "" || exec.TaskID == taskID {
			executions = append(executions, exec)
		}
	}
	return executions
}

// CreateGroup 创建任务组.
func (tc *TaskCenter) CreateGroup(group TaskGroup) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	group.CreatedAt = time.Now()
	tc.groups[group.ID] = &group
	return nil
}

// GetGroup 获取任务组.
func (tc *TaskCenter) GetGroup(groupID string) (*TaskGroup, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	group, ok := tc.groups[groupID]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	return group, nil
}

// ListGroups 列出任务组.
func (tc *TaskCenter) ListGroups() []*TaskGroup {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	groups := make([]*TaskGroup, 0, len(tc.groups))
	for _, group := range tc.groups {
		groups = append(groups, group)
	}
	return groups
}

// AddTemplate 添加模板.
func (tc *TaskCenter) AddTemplate(template TaskTemplate) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.templates[template.ID] = &template
	return nil
}

// GetTemplate 获取模板.
func (tc *TaskCenter) GetTemplate(templateID string) (*TaskTemplate, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	template, ok := tc.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	return template, nil
}

// ListTemplates 列出模板.
func (tc *TaskCenter) ListTemplates(taskType TaskType) []*TaskTemplate {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	templates := make([]*TaskTemplate, 0)
	for _, template := range tc.templates {
		if taskType == "" || template.Type == taskType {
			templates = append(templates, template)
		}
	}
	return templates
}

// AddLog 添加日志.
func (tc *TaskCenter) AddLog(log TaskLog) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	log.Timestamp = time.Now()
	tc.logs = append(tc.logs, log)

	// 限制日志数量
	if len(tc.logs) > 10000 {
		tc.logs = tc.logs[len(tc.logs)-5000:]
	}
}

// GetLogs 获取日志.
func (tc *TaskCenter) GetLogs(taskID string, limit int) []TaskLog {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	logs := make([]TaskLog, 0)
	for _, log := range tc.logs {
		if taskID == "" || log.TaskID == taskID {
			logs = append(logs, log)
		}
	}

	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	return logs
}
