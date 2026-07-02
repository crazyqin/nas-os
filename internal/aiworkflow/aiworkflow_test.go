package aiworkflow

import (
	"context"
	"testing"
	"time"
)

func TestNewWorkflowEngine(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	if engine == nil {
		t.Fatal("NewWorkflowEngine returned nil")
	}
	defer engine.Stop()
}

func TestCreateWorkflow(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	wf := &Workflow{
		Name:        "test-workflow",
		Description: "A test workflow",
		Steps: []*Step{
			{
				ID:     "step1",
				Name:   "First Step",
				Type:   StepTypeAction,
				Action: "system.info",
			},
		},
		Enabled: true,
	}

	if err := engine.CreateWorkflow(wf); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	if wf.ID == "" {
		t.Error("workflow ID should be auto-generated")
	}
	if wf.Version != 1 {
		t.Errorf("expected version 1, got %d", wf.Version)
	}
}

func TestCreateWorkflowValidation(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	// Empty name
	err := engine.CreateWorkflow(&Workflow{
		Steps: []*Step{{ID: "s1", Type: StepTypeAction, Action: "test"}},
	})
	if err == nil {
		t.Error("should reject empty name")
	}

	// No steps
	err = engine.CreateWorkflow(&Workflow{
		Name: "no-steps",
	})
	if err == nil {
		t.Error("should reject workflow without steps")
	}
}

func TestExecuteWorkflow(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	RegisterActionHandler("test.action", func(ctx context.Context, params map[string]interface{}, vars map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "success"}, nil
	})

	wf := &Workflow{
		Name:    "exec-test",
		Enabled: true,
		Steps: []*Step{
			{
				ID:     "step1",
				Name:   "Test Step",
				Type:   StepTypeAction,
				Action: "test.action",
			},
		},
	}

	engine.CreateWorkflow(wf)

	exec, err := engine.Execute(context.Background(), wf.ID, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	exec, _ = engine.GetExecution(exec.ID)
	if exec.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
}

func TestExecuteDisabledWorkflow(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	wf := &Workflow{
		Name:    "disabled",
		Enabled: false,
		Steps: []*Step{
			{ID: "s1", Type: StepTypeAction, Action: "test"},
		},
	}

	engine.CreateWorkflow(wf)

	_, err := engine.Execute(context.Background(), wf.ID, nil)
	if err == nil {
		t.Error("should reject disabled workflow")
	}
}

func TestCancelExecution(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	wf := &Workflow{
		Name:    "cancel-test",
		Enabled: true,
		Steps: []*Step{
			{
				ID:         "slow",
				Type:       StepTypeDelay,
				Action:     "delay",
				Parameters: map[string]interface{}{"seconds": float64(10)},
			},
		},
	}

	engine.CreateWorkflow(wf)

	exec, _ := engine.Execute(context.Background(), wf.ID, nil)
	time.Sleep(50 * time.Millisecond)

	if err := engine.CancelExecution(exec.ID); err != nil {
		t.Fatalf("CancelExecution failed: %v", err)
	}

	exec, _ = engine.GetExecution(exec.ID)
	if exec.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", exec.Status)
	}
}

func TestConditionEvaluation(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	conditions := []*Condition{
		{Field: "env", Operator: "eq", Value: "production"},
	}

	vars := map[string]interface{}{
		"env": "production",
	}

	if !engine.evaluateConditions(conditions, vars) {
		t.Error("condition should pass")
	}

	vars["env"] = "staging"
	if engine.evaluateConditions(conditions, vars) {
		t.Error("condition should fail")
	}
}

func TestListWorkflows(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	for i := 0; i < 3; i++ {
		engine.CreateWorkflow(&Workflow{
			Name:    "wf",
			Enabled: true,
			Steps:   []*Step{{ID: "s1", Type: StepTypeAction, Action: "test"}},
		})
	}

	workflows := engine.ListWorkflows()
	if len(workflows) != 3 {
		t.Errorf("expected 3 workflows, got %d", len(workflows))
	}
}

func TestMetrics(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil)
	defer engine.Stop()

	RegisterActionHandler("metric.test", func(ctx context.Context, params map[string]interface{}, vars map[string]interface{}) (map[string]interface{}, error) {
		return nil, nil
	})

	wf := &Workflow{
		Name:    "metrics-test",
		Enabled: true,
		Steps: []*Step{
			{ID: "s1", Type: StepTypeAction, Action: "metric.test"},
		},
	}

	engine.CreateWorkflow(wf)
	engine.Execute(context.Background(), wf.ID, nil)
	time.Sleep(200 * time.Millisecond)

	metrics := engine.GetMetrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", metrics.TotalExecutions)
	}
}
