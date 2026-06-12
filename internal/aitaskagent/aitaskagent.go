package aitaskagent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents an AI-managed task
type Task struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"` // backup, cleanup, monitor, optimize, custom
	Status      string            `json:"status"` // pending, scheduled, running, completed, failed
	Priority    int               `json:"priority"` // 1-10, 10 highest
	Schedule    string            `json:"schedule"` // cron expression
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	NextRun     *time.Time        `json:"next_run,omitempty"`
	RunCount    int               `json:"run_count"`
	FailCount   int               `json:"fail_count"`
	LastError   string            `json:"last_error,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// TaskExecution represents a single task execution
type TaskExecution struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"` // running, completed, failed
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
}

// AgentWorkflow represents an AI agent workflow
type AgentWorkflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"created_at"`
	RunCount    int            `json:"run_count"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"` // task, condition, wait, notify
	Config     map[string]string `json:"config"`
	NextStep   string            `json:"next_step,omitempty"`
	OnFailure  string            `json:"on_failure,omitempty"`
}

// AgentCapability represents what the AI agent can do
type AgentCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Parameters  []string `json:"parameters"`
}

// AgentStats aggregates agent statistics
type AgentStats struct {
	TotalTasks      int     `json:"total_tasks"`
	ActiveTasks     int     `json:"active_tasks"`
	CompletedRuns   int     `json:"completed_runs"`
	FailedRuns      int     `json:"failed_runs"`
	AvgRunDuration  float64 `json:"avg_run_duration_ms"`
	UptimePercent   float64 `json:"uptime_percent"`
	WorkflowsActive int     `json:"workflows_active"`
}

// AITaskAgent manages automated tasks and workflows
type AITaskAgent struct {
	mu           sync.RWMutex
	tasks        map[string]*Task
	executions   []TaskExecution
	workflows    map[string]*AgentWorkflow
	capabilities []AgentCapability
}

// NewAITaskAgent creates a new AI task agent
func NewAITaskAgent() *AITaskAgent {
	agent := &AITaskAgent{
		tasks:      make(map[string]*Task),
		executions: make([]TaskExecution, 0),
		workflows:  make(map[string]*AgentWorkflow),
		capabilities: []AgentCapability{
			{Name: "backup", Description: "Automated backup management", Category: "storage", Parameters: []string{"path", "schedule", "retention"}},
			{Name: "cleanup", Description: "Disk cleanup and optimization", Category: "storage", Parameters: []string{"path", "older_than", "pattern"}},
			{Name: "monitor", Description: "System health monitoring", Category: "system", Parameters: []string{"threshold", "interval"}},
			{Name: "optimize", Description: "Performance optimization", Category: "system", Parameters: []string{"target", "strategy"}},
			{Name: "notify", Description: "Send notifications", Category: "communication", Parameters: []string{"channel", "message", "priority"}},
		},
	}
	return agent
}

// CreateTask creates a new task
func (agent *AITaskAgent) CreateTask(ctx context.Context, task *Task) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	task.Status = "pending"
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.Enabled = true

	agent.tasks[task.ID] = task
	return nil
}

// UpdateTask updates an existing task
func (agent *AITaskAgent) UpdateTask(ctx context.Context, taskID string, updates map[string]interface{}) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	task, ok := agent.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if name, ok := updates["name"].(string); ok {
		task.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := updates["priority"].(int); ok {
		task.Priority = priority
	}
	if schedule, ok := updates["schedule"].(string); ok {
		task.Schedule = schedule
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		task.Enabled = enabled
	}

	task.UpdatedAt = time.Now()
	return nil
}

// DeleteTask deletes a task
func (agent *AITaskAgent) DeleteTask(ctx context.Context, taskID string) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if _, ok := agent.tasks[taskID]; !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	delete(agent.tasks, taskID)
	return nil
}

// RunTask executes a task
func (agent *AITaskAgent) RunTask(ctx context.Context, taskID string) (*TaskExecution, error) {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	task, ok := agent.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	if !task.Enabled {
		return nil, fmt.Errorf("task %s is disabled", taskID)
	}

	execution := &TaskExecution{
		ID:        fmt.Sprintf("exec-%s-%d", taskID, time.Now().Unix()),
		TaskID:    taskID,
		Status:    "running",
		StartTime: time.Now(),
	}

	task.Status = "running"
	task.UpdatedAt = time.Now()

	agent.executions = append(agent.executions, *execution)
	return execution, nil
}

// CompleteTask marks a task execution as complete
func (agent *AITaskAgent) CompleteTask(ctx context.Context, executionID string, output string) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	for i, exec := range agent.executions {
		if exec.ID == executionID {
			now := time.Now()
			agent.executions[i].Status = "completed"
			agent.executions[i].EndTime = &now
			agent.executions[i].Duration = now.Sub(exec.StartTime)
			agent.executions[i].Output = output

			task := agent.tasks[exec.TaskID]
			if task != nil {
				task.Status = "completed"
				task.RunCount++
				task.LastRun = &now
				task.UpdatedAt = now
			}
			return nil
		}
	}

	return fmt.Errorf("execution %s not found", executionID)
}

// FailTask marks a task execution as failed
func (agent *AITaskAgent) FailTask(ctx context.Context, executionID string, errMsg string) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	for i, exec := range agent.executions {
		if exec.ID == executionID {
			now := time.Now()
			agent.executions[i].Status = "failed"
			agent.executions[i].EndTime = &now
			agent.executions[i].Duration = now.Sub(exec.StartTime)
			agent.executions[i].Error = errMsg

			task := agent.tasks[exec.TaskID]
			if task != nil {
				task.Status = "failed"
				task.FailCount++
				task.LastError = errMsg
				task.UpdatedAt = now
			}
			return nil
		}
	}

	return fmt.Errorf("execution %s not found", executionID)
}

// CreateWorkflow creates a new agent workflow
func (agent *AITaskAgent) CreateWorkflow(ctx context.Context, workflow *AgentWorkflow) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}

	workflow.CreatedAt = time.Now()
	workflow.Enabled = true
	agent.workflows[workflow.ID] = workflow
	return nil
}

// GetTask returns a task by ID
func (agent *AITaskAgent) GetTask(ctx context.Context, taskID string) (*Task, error) {
	agent.mu.RLock()
	defer agent.mu.RUnlock()

	task, ok := agent.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// ListTasks returns all tasks
func (agent *AITaskAgent) ListTasks(ctx context.Context, taskType string) []*Task {
	agent.mu.RLock()
	defer agent.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, t := range agent.tasks {
		if taskType == "" || t.Type == taskType {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// GetExecutions returns execution history
func (agent *AITaskAgent) GetExecutions(ctx context.Context, taskID string) []TaskExecution {
	agent.mu.RLock()
	defer agent.mu.RUnlock()

	if taskID == "" {
		return agent.executions
	}

	var result []TaskExecution
	for _, e := range agent.executions {
		if e.TaskID == taskID {
			result = append(result, e)
		}
	}
	return result
}

// GetWorkflows returns all workflows
func (agent *AITaskAgent) GetWorkflows(ctx context.Context) []*AgentWorkflow {
	agent.mu.RLock()
	defer agent.mu.RUnlock()

	workflows := make([]*AgentWorkflow, 0, len(agent.workflows))
	for _, w := range agent.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// GetCapabilities returns agent capabilities
func (agent *AITaskAgent) GetCapabilities(ctx context.Context) []AgentCapability {
	return agent.capabilities
}

// GetStats returns agent statistics
func (agent *AITaskAgent) GetStats(ctx context.Context) *AgentStats {
	agent.mu.RLock()
	defer agent.mu.RUnlock()

	stats := &AgentStats{
		TotalTasks: len(agent.tasks),
	}

	for _, t := range agent.tasks {
		if t.Enabled {
			stats.ActiveTasks++
		}
	}

	totalDuration := 0.0
	for _, e := range agent.executions {
		switch e.Status {
		case "completed":
			stats.CompletedRuns++
		case "failed":
			stats.FailedRuns++
		}
		totalDuration += float64(e.Duration.Milliseconds())
	}

	if len(agent.executions) > 0 {
		stats.AvgRunDuration = totalDuration / float64(len(agent.executions))
	}

	stats.WorkflowsActive = len(agent.workflows)

	return stats
}
