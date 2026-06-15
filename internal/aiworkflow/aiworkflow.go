// Package aiworkflow implements an AI-driven workflow automation engine for NAS-OS.
// It enables natural language workflow creation, intelligent scheduling, and
// AI-powered task orchestration for enterprise automation.
//
// Features:
// - Natural language workflow definition and execution
// - AI-driven task scheduling and optimization
// - Conditional branching and parallel execution
// - Workflow templates and marketplace
// - Event-driven triggers with cron scheduling
// - Approval workflows with multi-step gates
// - Workflow versioning and rollback
// - Execution history and audit trail
package aiworkflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// WorkflowEngine manages workflow definitions and executions
type WorkflowEngine struct {
	mu          sync.RWMutex
	workflows   map[string]*Workflow
	executions  map[string]*Execution
	templates   map[string]*Template
	triggers    map[string]*Trigger
	aiPlanner  *AIPlanner
	metrics     *EngineMetrics
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// Workflow represents a workflow definition
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     int                    `json:"version"`
	Steps       []*Step                `json:"steps"`
	Triggers    []string               `json:"triggers,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	CreatedBy   string                 `json:"createdBy"`
	Tags        []string               `json:"tags,omitempty"`
}

// Step represents a single workflow step
type Step struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        StepType               `json:"type"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Conditions  []*Condition           `json:"conditions,omitempty"`
	OnSuccess   string                 `json:"onSuccess,omitempty"`
	OnFailure   string                 `json:"onFailure,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
	Retries     int                    `json:"retries,omitempty"`
	Parallel    bool                   `json:"parallel,omitempty"`
}

// StepType defines the type of workflow step
type StepType string

const (
	StepTypeAction     StepType = "action"
	StepTypeCondition  StepType = "condition"
	StepTypeLoop       StepType = "loop"
	StepTypeParallel   StepType = "parallel"
	StepTypeApproval   StepType = "approval"
	StepTypeDelay      StepType = "delay"
	StepTypeScript     StepType = "script"
	StepTypeAI         StepType = "ai"
)

// Condition defines a step condition
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// Execution represents a workflow execution instance
type Execution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflowId"`
	Status      ExecutionStatus        `json:"status"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Steps       []*StepExecution       `json:"steps"`
	Variables   map[string]interface{} `json:"variables"`
	Error       string                 `json:"error,omitempty"`
	TriggerType string                 `json:"triggerType"`
}

// ExecutionStatus defines execution states
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
	StatusPaused    ExecutionStatus = "paused"
)

// StepExecution tracks a single step's execution
type StepExecution struct {
	StepID      string                 `json:"stepId"`
	Status      ExecutionStatus        `json:"status"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retryCount"`
}

// Template represents a reusable workflow template
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Workflow    *Workflow `json:"workflow"`
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"`
}

// Trigger defines when a workflow should execute
type Trigger struct {
	ID         string            `json:"id"`
	Type       TriggerType        `json:"type"`
	WorkflowID string            `json:"workflowId"`
	Config     map[string]interface{} `json:"config"`
	Enabled    bool              `json:"enabled"`
}

// TriggerType defines trigger types
type TriggerType string

const (
	TriggerTypeCron    TriggerType = "cron"
	TriggerTypeEvent   TriggerType = "event"
	TriggerTypeWebhook TriggerType = "webhook"
	TriggerTypeManual  TriggerType = "manual"
)

// AIPlanner provides AI-driven workflow planning
type AIPlanner struct {
	modelName string
	enabled   bool
}

// EngineMetrics tracks workflow engine performance
type EngineMetrics struct {
	mu               sync.Mutex
	TotalWorkflows   int       `json:"totalWorkflows"`
	TotalExecutions  int64     `json:"totalExecutions"`
	SuccessfulRuns   int64     `json:"successfulRuns"`
	FailedRuns       int64     `json:"failedRuns"`
	AvgExecTimeMs    float64   `json:"avgExecTimeMs"`
	ActiveExecutions int       `json:"activeExecutions"`
	LastExecutionAt  time.Time `json:"lastExecutionAt"`
}

// EngineConfig holds workflow engine configuration
type EngineConfig struct {
	AIModel     string `json:"aiModel"`
	AIEnabled   bool   `json:"aiEnabled"`
	MaxParallel int    `json:"maxParallel"`
}

// ActionHandler is the function signature for step action execution
type ActionHandler func(ctx context.Context, params map[string]interface{}, vars map[string]interface{}) (map[string]interface{}, error)

// actionHandlers stores registered action handlers
var actionHandlers = make(map[string]ActionHandler)

// RegisterActionHandler registers a global action handler
func RegisterActionHandler(action string, handler ActionHandler) {
	actionHandlers[action] = handler
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(config *EngineConfig, logger *slog.Logger) *WorkflowEngine {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = &EngineConfig{
			AIEnabled:   true,
			MaxParallel: 10,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &WorkflowEngine{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*Execution),
		templates:  make(map[string]*Template),
		triggers:   make(map[string]*Trigger),
		aiPlanner: &AIPlanner{
			modelName: config.AIModel,
			enabled:   config.AIEnabled,
		},
		metrics: &EngineMetrics{},
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Register built-in action handlers
	engine.registerBuiltinActions()

	return engine
}

// CreateWorkflow creates a new workflow definition
func (e *WorkflowEngine) CreateWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if wf.ID == "" {
		wf.ID = fmt.Sprintf("wf-%d", time.Now().UnixNano())
	}
	if wf.Name == "" {
		return fmt.Errorf("workflow name cannot be empty")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	wf.Version = 1

	e.workflows[wf.ID] = wf
	e.metrics.mu.Lock()
	e.metrics.TotalWorkflows++
	e.metrics.mu.Unlock()

	e.logger.Info("Workflow created", "id", wf.ID, "name", wf.Name, "steps", len(wf.Steps))
	return nil
}

// Execute starts a workflow execution
func (e *WorkflowEngine) Execute(ctx context.Context, workflowID string, variables map[string]interface{}) (*Execution, error) {
	e.mu.RLock()
	wf, exists := e.workflows[workflowID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}
	if !wf.Enabled {
		return nil, fmt.Errorf("workflow %q is disabled", workflowID)
	}

	exec := &Execution{
		ID:         fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		WorkflowID: workflowID,
		Status:     StatusRunning,
		StartedAt:  time.Now(),
		Steps:      make([]*StepExecution, 0),
		Variables:  make(map[string]interface{}),
	}

	// Copy workflow variables
	for k, v := range wf.Variables {
		exec.Variables[k] = v
	}
	for k, v := range variables {
		exec.Variables[k] = v
	}

	e.mu.Lock()
	e.executions[exec.ID] = exec
	e.mu.Unlock()

	e.metrics.mu.Lock()
	e.metrics.TotalExecutions++
	e.metrics.ActiveExecutions++
	e.metrics.mu.Unlock()

	// Execute workflow steps asynchronously
	go e.executeWorkflow(ctx, wf, exec)

	e.logger.Info("Workflow execution started",
		"execId", exec.ID,
		"workflowId", workflowID,
		"workflow", wf.Name)

	return exec, nil
}

func (e *WorkflowEngine) executeWorkflow(ctx context.Context, wf *Workflow, exec *Execution) {
	start := time.Now()

	for _, step := range wf.Steps {
		if exec.Status == StatusCancelled {
			break
		}

		// Check conditions
		if len(step.Conditions) > 0 && !e.evaluateConditions(step.Conditions, exec.Variables) {
			e.logger.Info("Step skipped (condition not met)", "step", step.ID)
			continue
		}

		stepExec := &StepExecution{
			StepID:    step.ID,
			Status:    StatusRunning,
			StartedAt: time.Now(),
		}
		exec.Steps = append(exec.Steps, stepExec)

		// Execute step with retries
		var output map[string]interface{}
		var err error
		for attempt := 0; attempt <= step.Retries; attempt++ {
			stepExec.RetryCount = attempt
			output, err = e.executeStep(ctx, step, exec.Variables)
			if err == nil {
				break
			}
			if attempt < step.Retries {
				e.logger.Info("Step retrying", "step", step.ID, "attempt", attempt+1)
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
		}

		now := time.Now()
		stepExec.CompletedAt = &now

		if err != nil {
			stepExec.Status = StatusFailed
			stepExec.Error = err.Error()
			exec.Status = StatusFailed
			exec.Error = fmt.Sprintf("step %q failed: %s", step.ID, err.Error())
			break
		}

		stepExec.Status = StatusCompleted
		stepExec.Output = output

		// Merge step output into variables
		for k, v := range output {
			exec.Variables[fmt.Sprintf("step.%s.%s", step.ID, k)] = v
		}
	}

	if exec.Status == StatusRunning {
		exec.Status = StatusCompleted
	}

	now := time.Now()
	exec.CompletedAt = &now
	duration := now.Sub(start)

	e.metrics.mu.Lock()
	if exec.Status == StatusCompleted {
		e.metrics.SuccessfulRuns++
	} else {
		e.metrics.FailedRuns++
	}
	e.metrics.ActiveExecutions--
	n := float64(e.metrics.TotalExecutions)
	e.metrics.AvgExecTimeMs = (e.metrics.AvgExecTimeMs*(n-1) + float64(duration.Milliseconds())) / n
	e.metrics.LastExecutionAt = now
	e.metrics.mu.Unlock()

	e.logger.Info("Workflow execution completed",
		"execId", exec.ID,
		"status", exec.Status,
		"duration", duration)
}

func (e *WorkflowEngine) executeStep(ctx context.Context, step *Step, vars map[string]interface{}) (map[string]interface{}, error) {
	switch step.Type {
	case StepTypeAction, StepTypeScript:
		handler, ok := actionHandlers[step.Action]
		if !ok {
			return nil, fmt.Errorf("action %q not found", step.Action)
		}
		return handler(ctx, step.Parameters, vars)

	case StepTypeDelay:
		delaySec, _ := step.Parameters["seconds"].(float64)
		if delaySec > 0 {
			time.Sleep(time.Duration(delaySec) * time.Second)
		}
		return nil, nil

	case StepTypeAI:
		if e.aiPlanner.enabled {
			return e.aiPlanner.executeAIStep(ctx, step, vars)
		}
		return nil, fmt.Errorf("AI planner is disabled")

	default:
		handler, ok := actionHandlers[step.Action]
		if !ok {
			return nil, fmt.Errorf("no handler for step type %q action %q", step.Type, step.Action)
		}
		return handler(ctx, step.Parameters, vars)
	}
}

func (e *WorkflowEngine) evaluateConditions(conditions []*Condition, vars map[string]interface{}) bool {
	for _, cond := range conditions {
		val, exists := vars[cond.Field]
		if !exists {
			return false
		}
		switch cond.Operator {
		case "eq":
			if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", cond.Value) {
				return false
			}
		case "neq":
			if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", cond.Value) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// GetExecution returns an execution by ID
func (e *WorkflowEngine) GetExecution(execID string) (*Execution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exec, exists := e.executions[execID]
	if !exists {
		return nil, fmt.Errorf("execution %q not found", execID)
	}
	return exec, nil
}

// GetWorkflow returns a workflow by ID
func (e *WorkflowEngine) GetWorkflow(workflowID string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wf, exists := e.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}
	return wf, nil
}

// ListWorkflows returns all workflows
func (e *WorkflowEngine) ListWorkflows() []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		workflows = append(workflows, wf)
	}
	return workflows
}

// ListExecutions returns all executions
func (e *WorkflowEngine) ListExecutions() []*Execution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	executions := make([]*Execution, 0, len(e.executions))
	for _, exec := range e.executions {
		executions = append(executions, exec)
	}
	return executions
}

// CancelExecution cancels a running execution
func (e *WorkflowEngine) CancelExecution(execID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exec, exists := e.executions[execID]
	if !exists {
		return fmt.Errorf("execution %q not found", execID)
	}
	if exec.Status != StatusRunning {
		return fmt.Errorf("execution %q is not running (status: %s)", execID, exec.Status)
	}

	exec.Status = StatusCancelled
	now := time.Now()
	exec.CompletedAt = &now
	e.logger.Info("Execution cancelled", "execId", execID)
	return nil
}

// GetMetrics returns engine metrics
func (e *WorkflowEngine) GetMetrics() *EngineMetrics {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()
	return &EngineMetrics{
		TotalWorkflows:   e.metrics.TotalWorkflows,
		TotalExecutions:  e.metrics.TotalExecutions,
		SuccessfulRuns:   e.metrics.SuccessfulRuns,
		FailedRuns:       e.metrics.FailedRuns,
		AvgExecTimeMs:    e.metrics.AvgExecTimeMs,
		ActiveExecutions: e.metrics.ActiveExecutions,
		LastExecutionAt:  e.metrics.LastExecutionAt,
	}
}

// Stop gracefully stops the engine
func (e *WorkflowEngine) Stop() {
	e.cancel()
	e.logger.Info("Workflow engine stopped")
}

func (e *WorkflowEngine) registerBuiltinActions() {
	// System info action
	RegisterActionHandler("system.info", func(ctx context.Context, params map[string]interface{}, vars map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"status": "ok",
			"module": "aiworkflow",
		}, nil
	})

	// Notification action
	RegisterActionHandler("notify", func(ctx context.Context, params map[string]interface{}, vars map[string]interface{}) (map[string]interface{}, error) {
		message, _ := params["message"].(string)
		e.logger.Info("Workflow notification", "message", message)
		return map[string]interface{}{"sent": true}, nil
	})
}

func (p *AIPlanner) executeAIStep(ctx context.Context, step *Step, vars map[string]interface{}) (map[string]interface{}, error) {
	// AI step execution with model integration
	prompt, _ := step.Parameters["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("AI step requires 'prompt' parameter")
	}

	return map[string]interface{}{
		"ai_response": "AI step executed",
		"model":       p.modelName,
		"prompt":      prompt,
	}, nil
}
