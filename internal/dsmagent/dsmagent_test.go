package dsmagent

import (
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	config := AgentConfig{
		AgentID:         "agent-001",
		Role:            RoleSystemAdmin,
		Name:            "System Agent",
		Enabled:         true,
		ScanInterval:    5 * time.Minute,
		AutoRemediation: true,
		AlertThreshold:  80,
		MaxConcurrent:   4,
	}
	agent := NewAgent(config)
	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}
	status := agent.GetAgentStatus()
	if status["agent_id"] != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %v", status["agent_id"])
	}
	if status["name"] != "System Agent" {
		t.Errorf("expected name 'System Agent', got %v", status["name"])
	}
	if status["role"] != RoleSystemAdmin {
		t.Errorf("expected role %q, got %v", RoleSystemAdmin, status["role"])
	}
	if status["running"] != false {
		t.Errorf("expected running false, got %v", status["running"])
	}
	if status["task_count"] != 0 {
		t.Errorf("expected task_count 0, got %v", status["task_count"])
	}
}

func TestAgentStartStop(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-002",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	status := agent.GetAgentStatus()
	if status["running"] != true {
		t.Error("expected agent to be running after Start()")
	}

	err = agent.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	status = agent.GetAgentStatus()
	if status["running"] != false {
		t.Error("expected agent to be stopped after Stop()")
	}
}

func TestAgentDoubleStart(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-003",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	defer agent.Stop()

	err = agent.Start()
	if err == nil {
		t.Error("expected error on double Start, got nil")
	}
}

func TestAgentStopWhenNotRunning(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-004",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Stop()
	if err == nil {
		t.Error("expected error when stopping non-running agent, got nil")
	}
}

func TestSubmitTask(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-005",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer agent.Stop()

	task, err := agent.SubmitTask(WorkflowHealthCheck, "Check system health", 1)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}
	if task.Workflow != WorkflowHealthCheck {
		t.Errorf("expected workflow %q, got %q", WorkflowHealthCheck, task.Workflow)
	}
	if task.Description != "Check system health" {
		t.Errorf("expected description 'Check system health', got %q", task.Description)
	}
	if task.Priority != 1 {
		t.Errorf("expected priority 1, got %d", task.Priority)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
}

func TestSubmitTaskUnknownWorkflow(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-006",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	_, err := agent.SubmitTask(WorkflowType("nonexistent"), "Bad workflow", 1)
	if err == nil {
		t.Error("expected error for unknown workflow, got nil")
	}
}

func TestGetTask(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-007",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer agent.Stop()

	task, _ := agent.SubmitTask(WorkflowHealthCheck, "Health check task", 5)

	retrieved, err := agent.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if retrieved.ID != task.ID {
		t.Errorf("expected task ID %q, got %q", task.ID, retrieved.ID)
	}
	if retrieved.Description != "Health check task" {
		t.Errorf("expected description 'Health check task', got %q", retrieved.Description)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-008",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	_, err := agent.GetTask("nonexistent-task-id")
	if err == nil {
		t.Error("expected error for nonexistent task, got nil")
	}
}

func TestListTasks(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-009",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer agent.Stop()

	agent.SubmitTask(WorkflowHealthCheck, "Task 1", 1)
	agent.SubmitTask(WorkflowBackupVerify, "Task 2", 2)
	agent.SubmitTask(WorkflowSecurityScan, "Task 3", 3)

	// Give tasks time to be queued
	time.Sleep(100 * time.Millisecond)

	// List all tasks
	allTasks := agent.ListTasks(nil)
	if len(allTasks) < 3 {
		t.Errorf("expected at least 3 tasks, got %d", len(allTasks))
	}

	// List by status
	pending := TaskPending
	pendingTasks := agent.ListTasks(&pending)
	if len(pendingTasks) == 0 {
		t.Log("Note: tasks may have started running by now, this is OK")
	}
}

func TestGetHealth(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-010",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	health := agent.GetHealth()
	if health == nil {
		t.Fatal("GetHealth returned nil")
	}
}

func TestDefaultWorkflowsRegistered(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-011",
		Role:          RoleSystemAdmin,
		Name:          "Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	// Verify default workflows exist by trying to submit tasks for each
	expectedWorkflows := []WorkflowType{
		WorkflowHealthCheck,
		WorkflowBackupVerify,
		WorkflowSecurityScan,
		WorkflowStorageOpt,
	}

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer agent.Stop()

	for _, wf := range expectedWorkflows {
		task, err := agent.SubmitTask(wf, "test", 1)
		if err != nil {
			t.Errorf("expected workflow %q to be registered, got error: %v", wf, err)
		}
		if task == nil {
			t.Errorf("expected non-nil task for workflow %q", wf)
		}
	}
}

func TestTaskExecution(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:         "agent-012",
		Role:            RoleSystemAdmin,
		Name:            "Test Agent",
		ScanInterval:    1 * time.Hour,
		AutoRemediation: true,
		MaxConcurrent:   2,
	})

	err := agent.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer agent.Stop()

	task, _ := agent.SubmitTask(WorkflowHealthCheck, "Execute health check", 1)

	// Wait for task to complete
	time.Sleep(500 * time.Millisecond)

	updated, err := agent.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if updated.Status != TaskCompleted && updated.Status != TaskRunning && updated.Status != TaskPending {
		t.Errorf("unexpected task status: %s", updated.Status)
	}
	if updated.Status == TaskCompleted {
		if updated.Result == nil {
			t.Error("expected non-nil result for completed task")
		}
	}
}
