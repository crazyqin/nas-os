// Package aiagentorchestrator provides AI agent orchestration engine for NAS-OS
// aiagentorchestrator.go - Multi-agent collaboration and task orchestration
package aiagentorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AgentType represents the type of AI agent
type AgentType string

const (
	AgentTypeAssistant  AgentType = "assistant"
	AgentTypeResearcher AgentType = "researcher"
	AgentTypeCoder      AgentType = "coder"
	AgentTypeAnalyst    AgentType = "analyst"
	AgentTypePlanner    AgentType = "planner"
	AgentTypeExecutor   AgentType = "executor"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusIdle     AgentStatus = "idle"
	AgentStatusBusy     AgentStatus = "busy"
	AgentStatusWaiting  AgentStatus = "waiting"
	AgentStatusError    AgentStatus = "error"
	AgentStatusDisabled AgentStatus = "disabled"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Agent represents an AI agent in the orchestration system
type Agent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        AgentType         `json:"type"`
	Status      AgentStatus       `json:"status"`
	Capabilities []string         `json:"capabilities"`
	Model       string            `json:"model"`
	SystemPrompt string           `json:"system_prompt"`
	MaxConcurrent int             `json:"max_concurrent"`
	CurrentTasks  int             `json:"current_tasks"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastActiveAt *time.Time       `json:"last_active_at,omitempty"`
}

// Task represents a task to be executed by agents
type Task struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      TaskStatus        `json:"status"`
	Priority    int               `json:"priority"` // 1-10, higher is more important
	AssignedTo  string            `json:"assigned_to,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Dependencies []string         `json:"dependencies,omitempty"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	RetryCount  int               `json:"retry_count"`
	MaxRetries  int               `json:"max_retries"`
	Timeout     time.Duration     `json:"timeout"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// Workflow represents a workflow of tasks
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      TaskStatus        `json:"status"`
	Tasks       []string          `json:"tasks"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// Message represents a message between agents
type Message struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // request, response, notification
	TaskID    string    `json:"task_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Orchestrator manages AI agents and tasks
type Orchestrator struct {
	agents     map[string]*Agent
	tasks      map[string]*Task
	workflows  map[string]*Workflow
	messages   []Message
	taskQueue  chan *Task
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator() *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	o := &Orchestrator{
		agents:    make(map[string]*Agent),
		tasks:     make(map[string]*Task),
		workflows: make(map[string]*Workflow),
		messages:  make([]Message, 0),
		taskQueue: make(chan *Task, 100),
		ctx:       ctx,
		cancel:    cancel,
	}
	go o.processTaskQueue()
	return o
}

// RegisterAgent registers a new agent
func (o *Orchestrator) RegisterAgent(agent *Agent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	agent.Status = AgentStatusIdle
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	o.agents[agent.ID] = agent
	log.Printf("Agent registered: %s (%s)", agent.Name, agent.ID)
	return nil
}

// UnregisterAgent removes an agent
func (o *Orchestrator) UnregisterAgent(agentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	agent, exists := o.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	if agent.Status == AgentStatusBusy {
		return fmt.Errorf("cannot remove busy agent: %s", agentID)
	}

	delete(o.agents, agentID)
	log.Printf("Agent unregistered: %s", agentID)
	return nil
}

// GetAgent returns an agent by ID
func (o *Orchestrator) GetAgent(agentID string) (*Agent, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agent, exists := o.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return agent, nil
}

// ListAgents returns all agents
func (o *Orchestrator) ListAgents() []*Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agents := make([]*Agent, 0, len(o.agents))
	for _, agent := range o.agents {
		agents = append(agents, agent)
	}
	return agents
}

// CreateTask creates a new task
func (o *Orchestrator) CreateTask(task *Task) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Status = TaskStatusPending
	task.CreatedAt = time.Now()

	o.tasks[task.ID] = task
	log.Printf("Task created: %s (%s)", task.Name, task.ID)
	return nil
}

// SubmitTask submits a task for execution
func (o *Orchestrator) SubmitTask(taskID string) error {
	o.mu.RLock()
	task, exists := o.tasks[taskID]
	o.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	o.taskQueue <- task
	return nil
}

// processTaskQueue processes tasks from the queue
func (o *Orchestrator) processTaskQueue() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case task := <-o.taskQueue:
			o.assignAndExecuteTask(task)
		}
	}
}

// assignAndExecuteTask assigns a task to an available agent and executes it
func (o *Orchestrator) assignAndExecuteTask(task *Task) {
	o.mu.Lock()

	// Find available agent
	var selectedAgent *Agent
	for _, agent := range o.agents {
		if agent.Status == AgentStatusIdle && agent.CurrentTasks < agent.MaxConcurrent {
			selectedAgent = agent
			break
		}
	}

	if selectedAgent == nil {
		// No agent available, requeue
		task.Status = TaskStatusPending
		o.mu.Unlock()
		go func() {
			time.Sleep(time.Second)
			o.taskQueue <- task
		}()
		return
	}

	// Assign task to agent
	task.AssignedTo = selectedAgent.ID
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	selectedAgent.Status = AgentStatusBusy
	selectedAgent.CurrentTasks++
	selectedAgent.UpdatedAt = time.Now()

	o.mu.Unlock()

	// Execute task (simulated)
	log.Printf("Executing task %s with agent %s", task.ID, selectedAgent.ID)
	o.executeTask(task, selectedAgent)
}

// executeTask simulates task execution
func (o *Orchestrator) executeTask(task *Task, agent *Agent) {
	// Simulate work
	time.Sleep(time.Second * 2)

	o.mu.Lock()
	defer o.mu.Unlock()

	// Update task status
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Output = map[string]interface{}{
		"result": fmt.Sprintf("Task completed by agent %s", agent.ID),
	}

	// Update agent status
	agent.CurrentTasks--
	if agent.CurrentTasks == 0 {
		agent.Status = AgentStatusIdle
	}
	agent.UpdatedAt = time.Now()
	lastActive := time.Now()
	agent.LastActiveAt = &lastActive

	log.Printf("Task completed: %s", task.ID)
}

// GetTask returns a task by ID
func (o *Orchestrator) GetTask(taskID string) (*Task, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	task, exists := o.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// ListTasks returns all tasks
func (o *Orchestrator) ListTasks() []*Task {
	o.mu.RLock()
	defer o.mu.RUnlock()

	tasks := make([]*Task, 0, len(o.tasks))
	for _, task := range o.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CreateWorkflow creates a new workflow
func (o *Orchestrator) CreateWorkflow(workflow *Workflow) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if workflow.ID == "" {
		workflow.ID = uuid.New().String()
	}
	workflow.Status = TaskStatusPending
	workflow.CreatedAt = time.Now()

	o.workflows[workflow.ID] = workflow
	log.Printf("Workflow created: %s (%s)", workflow.Name, workflow.ID)
	return nil
}

// StartWorkflow starts a workflow execution
func (o *Orchestrator) StartWorkflow(workflowID string) error {
	o.mu.Lock()
	workflow, exists := o.workflows[workflowID]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	workflow.Status = TaskStatusRunning
	now := time.Now()
	workflow.StartedAt = &now
	o.mu.Unlock()

	// Execute workflow tasks
	go o.executeWorkflow(workflow)
	return nil
}

// executeWorkflow executes all tasks in a workflow
func (o *Orchestrator) executeWorkflow(workflow *Workflow) {
	for _, taskID := range workflow.Tasks {
		task, err := o.GetTask(taskID)
		if err != nil {
			log.Printf("Workflow task not found: %s", taskID)
			continue
		}

		o.SubmitTask(task.ID)

		// Wait for task completion
		for {
			time.Sleep(time.Second)
			task, _ = o.GetTask(task.ID)
			if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
				break
			}
		}
	}

	o.mu.Lock()
	workflow.Status = TaskStatusCompleted
	now := time.Now()
	workflow.CompletedAt = &now
	o.mu.Unlock()

	log.Printf("Workflow completed: %s", workflow.ID)
}

// SendMessage sends a message between agents
func (o *Orchestrator) SendMessage(msg *Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.Timestamp = time.Now()

	o.messages = append(o.messages, *msg)
	log.Printf("Message sent: %s -> %s", msg.From, msg.To)
	return nil
}

// GetMessages returns messages for an agent
func (o *Orchestrator) GetMessages(agentID string) []Message {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var messages []Message
	for _, msg := range o.messages {
		if msg.To == agentID || msg.From == agentID {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetStats returns orchestrator statistics
func (o *Orchestrator) GetStats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agentsByStatus := make(map[AgentStatus]int)
	for _, agent := range o.agents {
		agentsByStatus[agent.Status]++
	}

	tasksByStatus := make(map[TaskStatus]int)
	for _, task := range o.tasks {
		tasksByStatus[task.Status]++
	}

	return map[string]interface{}{
		"total_agents":     len(o.agents),
		"total_tasks":      len(o.tasks),
		"total_workflows":  len(o.workflows),
		"total_messages":   len(o.messages),
		"agents_by_status": agentsByStatus,
		"tasks_by_status":  tasksByStatus,
	}
}

// Stop stops the orchestrator
func (o *Orchestrator) Stop() {
	o.cancel()
}

// RegisterRoutes registers HTTP routes for the orchestrator API
func RegisterRoutes(mux *http.ServeMux, orchestrator *Orchestrator) {
	mux.HandleFunc("/api/aiagent/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			agents := orchestrator.ListAgents()
			json.NewEncoder(w).Encode(agents)
		case http.MethodPost:
			var agent Agent
			if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := orchestrator.RegisterAgent(&agent); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(agent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/aiagent/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			tasks := orchestrator.ListTasks()
			json.NewEncoder(w).Encode(tasks)
		case http.MethodPost:
			var task Task
			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := orchestrator.CreateTask(&task); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(task)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/aiagent/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := orchestrator.GetStats()
		json.NewEncoder(w).Encode(stats)
	})
}
