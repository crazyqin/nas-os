package aitaskagent

import (
	"context"
	"testing"
)

func TestCreateTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	task := &Task{
		ID:          "task-1",
		Name:        "Daily Backup",
		Description: "Backup important files",
		Type:        "backup",
		Priority:    8,
		Schedule:    "0 2 * * *",
	}

	err := agent.CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Status != "pending" {
		t.Errorf("expected status pending, got %s", task.Status)
	}
	if !task.Enabled {
		t.Error("task should be enabled")
	}
}

func TestCreateTaskNoID(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	task := &Task{
		Name: "No ID",
	}

	err := agent.CreateTask(ctx, task)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestUpdateTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{
		ID:       "task-1",
		Name:     "Original Name",
		Priority: 5,
	})

	err := agent.UpdateTask(ctx, "task-1", map[string]interface{}{
		"name":     "Updated Name",
		"priority": 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, _ := agent.GetTask(ctx, "task-1")
	if task.Name != "Updated Name" {
		t.Errorf("expected Updated Name, got %s", task.Name)
	}
	if task.Priority != 10 {
		t.Errorf("expected priority 10, got %d", task.Priority)
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	err := agent.UpdateTask(ctx, "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Test"})

	err := agent.DeleteTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.GetTask(ctx, "task-1")
	if err == nil {
		t.Fatal("task should be deleted")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	err := agent.DeleteTask(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Test Task"})

	exec, err := agent.RunTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exec.Status != "running" {
		t.Errorf("expected status running, got %s", exec.Status)
	}
}

func TestRunTaskDisabled(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	task := &Task{ID: "task-1", Name: "Disabled Task", Enabled: false}
	agent.CreateTask(ctx, task)
	agent.mu.Lock()
	agent.tasks["task-1"].Enabled = false
	agent.mu.Unlock()

	_, err := agent.RunTask(ctx, "task-1")
	if err == nil {
		t.Fatal("expected error for disabled task")
	}
}

func TestRunTaskNotFound(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	_, err := agent.RunTask(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompleteTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Test Task"})
	exec, _ := agent.RunTask(ctx, "task-1")

	err := agent.CompleteTask(ctx, exec.ID, "Task completed successfully")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, _ := agent.GetTask(ctx, "task-1")
	if task.Status != "completed" {
		t.Errorf("expected status completed, got %s", task.Status)
	}
	if task.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", task.RunCount)
	}
}

func TestFailTask(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Test Task"})
	exec, _ := agent.RunTask(ctx, "task-1")

	err := agent.FailTask(ctx, exec.ID, "disk full")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, _ := agent.GetTask(ctx, "task-1")
	if task.Status != "failed" {
		t.Errorf("expected status failed, got %s", task.Status)
	}
	if task.FailCount != 1 {
		t.Errorf("expected fail count 1, got %d", task.FailCount)
	}
}

func TestCreateWorkflow(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	workflow := &AgentWorkflow{
		ID:          "wf-1",
		Name:        "Backup and Verify",
		Description: "Backup data and verify integrity",
		Steps: []WorkflowStep{
			{Name: "backup", Type: "task", Config: map[string]string{"task_id": "task-1"}},
			{Name: "verify", Type: "task", Config: map[string]string{"task_id": "task-2"}},
		},
	}

	err := agent.CreateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	workflows := agent.GetWorkflows(ctx)
	if len(workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(workflows))
	}
}

func TestCreateWorkflowNoID(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	workflow := &AgentWorkflow{
		Name: "No ID",
	}

	err := agent.CreateWorkflow(ctx, workflow)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestListTasks(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Backup", Type: "backup"})
	agent.CreateTask(ctx, &Task{ID: "task-2", Name: "Cleanup", Type: "cleanup"})
	agent.CreateTask(ctx, &Task{ID: "task-3", Name: "Backup2", Type: "backup"})

	// All tasks
	tasks := agent.ListTasks(ctx, "")
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	// Filter by type
	tasks = agent.ListTasks(ctx, "backup")
	if len(tasks) != 2 {
		t.Errorf("expected 2 backup tasks, got %d", len(tasks))
	}
}

func TestGetExecutions(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "Test"})
	exec1, _ := agent.RunTask(ctx, "task-1")
	agent.CompleteTask(ctx, exec1.ID, "done")

	exec2, _ := agent.RunTask(ctx, "task-1")
	agent.FailTask(ctx, exec2.ID, "error")

	execs := agent.GetExecutions(ctx, "task-1")
	if len(execs) != 2 {
		t.Errorf("expected 2 executions, got %d", len(execs))
	}

	// Empty filter returns all
	execs = agent.GetExecutions(ctx, "")
	if len(execs) != 2 {
		t.Errorf("expected 2 executions, got %d", len(execs))
	}
}

func TestGetCapabilities(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	caps := agent.GetCapabilities(ctx)
	if len(caps) == 0 {
		t.Error("expected some capabilities")
	}
}

func TestGetStats(t *testing.T) {
	agent := NewAITaskAgent()
	ctx := context.Background()

	agent.CreateTask(ctx, &Task{ID: "task-1", Name: "T1", Enabled: true})
	task := &Task{ID: "task-2", Name: "T2"}
	agent.CreateTask(ctx, task)
	agent.mu.Lock()
	agent.tasks["task-2"].Enabled = false
	agent.mu.Unlock()

	exec, _ := agent.RunTask(ctx, "task-1")
	agent.CompleteTask(ctx, exec.ID, "done")

	stats := agent.GetStats(ctx)
	if stats.TotalTasks != 2 {
		t.Errorf("expected 2 total tasks, got %d", stats.TotalTasks)
	}
	if stats.ActiveTasks != 1 {
		t.Errorf("expected 1 active task, got %d", stats.ActiveTasks)
	}
	if stats.CompletedRuns != 1 {
		t.Errorf("expected 1 completed run, got %d", stats.CompletedRuns)
	}
}
