package aiagentorchestrator

import (
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	o := NewOrchestrator()
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	defer o.Stop()

	if len(o.agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(o.agents))
	}
	if len(o.tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(o.tasks))
	}
}

func TestRegisterAgent(t *testing.T) {
	o := NewOrchestrator()
	defer o.Stop()

	agent := &Agent{
		Name:          "Test Agent",
		Type:          AgentTypeAssistant,
		Capabilities:  []string{"chat", "search"},
		Model:         "gpt-4",
		MaxConcurrent: 2,
	}

	err := o.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	if agent.ID == "" {
		t.Error("Agent ID not generated")
	}
	if agent.Status != AgentStatusIdle {
		t.Errorf("Expected status idle, got %s", agent.Status)
	}

	// Verify agent exists
	retrieved, err := o.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if retrieved.Name != "Test Agent" {
		t.Errorf("Expected name 'Test Agent', got '%s'", retrieved.Name)
	}
}

func TestCreateTask(t *testing.T) {
	o := NewOrchestrator()
	defer o.Stop()

	task := &Task{
		Name:        "Test Task",
		Description: "A test task",
		Priority:    5,
	}

	err := o.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID not generated")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}
}

func TestTaskExecution(t *testing.T) {
	o := NewOrchestrator()
	defer o.Stop()

	// Register agent
	agent := &Agent{
		Name:          "Worker Agent",
		Type:          AgentTypeExecutor,
		MaxConcurrent: 1,
	}
	o.RegisterAgent(agent)

	// Create and submit task
	task := &Task{
		Name:     "Execute Task",
		Priority: 5,
	}
	o.CreateTask(task)

	err := o.SubmitTask(task.ID)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// Wait for task completion
	time.Sleep(time.Second * 3)

	// Verify task completed
	completedTask, err := o.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if completedTask.Status != TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s", completedTask.Status)
	}
	if completedTask.Output == nil {
		t.Error("Task output is nil")
	}
}

func TestSendMessage(t *testing.T) {
	o := NewOrchestrator()
	defer o.Stop()

	// Register two agents
	agent1 := &Agent{Name: "Agent 1", Type: AgentTypeAssistant}
	agent2 := &Agent{Name: "Agent 2", Type: AgentTypeResearcher}
	o.RegisterAgent(agent1)
	o.RegisterAgent(agent2)

	// Send message
	msg := &Message{
		From:    agent1.ID,
		To:      agent2.ID,
		Content: "Hello from Agent 1",
		Type:    "notification",
	}

	err := o.SendMessage(msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify messages
	messages := o.GetMessages(agent2.ID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

func TestGetStats(t *testing.T) {
	o := NewOrchestrator()
	defer o.Stop()

	// Register agents
	o.RegisterAgent(&Agent{Name: "Agent 1", Type: AgentTypeAssistant})
	o.RegisterAgent(&Agent{Name: "Agent 2", Type: AgentTypeExecutor})

	// Create tasks
	o.CreateTask(&Task{Name: "Task 1", Priority: 5})
	o.CreateTask(&Task{Name: "Task 2", Priority: 3})

	stats := o.GetStats()

	if stats["total_agents"] != 2 {
		t.Errorf("Expected 2 agents, got %v", stats["total_agents"])
	}
	if stats["total_tasks"] != 2 {
		t.Errorf("Expected 2 tasks, got %v", stats["total_tasks"])
	}
}
