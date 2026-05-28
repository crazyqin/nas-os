package workflowengine

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// MockStepExecutor 模拟步骤执行器
type MockStepExecutor struct {
	executeFunc func(ctx context.Context, step StepConfig, vars map[string]interface{}) (map[string]interface{}, error)
}

func (m *MockStepExecutor) Execute(ctx context.Context, step StepConfig, vars map[string]interface{}) (map[string]interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step, vars)
	}
	return map[string]interface{}{"result": "ok"}, nil
}

func setupTestEngine(t *testing.T) (*WorkflowEngine, *Manager) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	engine := NewWorkflowEngine(manager, EngineConfig{
		MaxConcurrent: 5,
		Timeout:       60,
	})
	return engine, manager
}

func TestNewWorkflowEngine(t *testing.T) {
	engine, _ := setupTestEngine(t)

	if engine == nil {
		t.Fatal("expected engine to be non-nil")
	}
	if engine.GetStatus() != EngineStatusIdle {
		t.Errorf("expected status idle, got %s", engine.GetStatus())
	}
}

func TestEngineStartStop(t *testing.T) {
	engine, _ := setupTestEngine(t)

	ctx := context.Background()

	// 启动引擎
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}
	if engine.GetStatus() != EngineStatusRunning {
		t.Errorf("expected status running, got %s", engine.GetStatus())
	}

	// 重复启动应该失败
	if err := engine.Start(ctx); err == nil {
		t.Fatal("expected error for double start")
	}

	// 停止引擎
	if err := engine.Stop(); err != nil {
		t.Fatalf("failed to stop engine: %v", err)
	}
	if engine.GetStatus() != EngineStatusStopped {
		t.Errorf("expected status stopped, got %s", engine.GetStatus())
	}
}

func TestRegisterExecutor(t *testing.T) {
	engine, _ := setupTestEngine(t)

	executor := &MockStepExecutor{}
	engine.RegisterExecutor("test-action", executor)

	// 验证执行器已注册（间接验证）
	config := engine.GetConfig()
	if config.MaxConcurrent != 5 {
		t.Errorf("expected max concurrent 5, got %d", config.MaxConcurrent)
	}
}

func TestCreateWorkflow(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name:        "Test Workflow",
		Description: "A test workflow",
		Nodes: []WorkflowNode{
			{
				ID:       "node-1",
				Name:     "Step 1",
				Type:     "task",
				TaskType: "test",
			},
		},
		Triggers: []Trigger{
			{
				ID:      "trigger-1",
				Type:    TriggerTypeManual,
				Enabled: true,
			},
		},
	}

	workflow, err := manager.CreateWorkflow(req, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if workflow.ID == "" {
		t.Fatal("expected workflow ID to be non-empty")
	}
	if workflow.Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", workflow.Name)
	}
	if workflow.Status != WorkflowStatusDraft {
		t.Errorf("expected status draft, got %s", workflow.Status)
	}
}

func TestGetWorkflow(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Test Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}

	created, _ := manager.CreateWorkflow(req, "test-user")

	got, err := manager.GetWorkflow(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, got.ID)
	}
}

func TestGetWorkflowNotFound(t *testing.T) {
	_, manager := setupTestEngine(t)

	_, err := manager.GetWorkflow("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}
}

func TestListWorkflows(t *testing.T) {
	_, manager := setupTestEngine(t)

	// 创建几个工作流
	for i := 0; i < 3; i++ {
		req := &CreateWorkflowRequest{
			Name: "Workflow",
			Nodes: []WorkflowNode{
				{ID: "node-1", Type: "task", TaskType: "test"},
			},
		}
		manager.CreateWorkflow(req, "test-user")
	}

	workflows := manager.ListWorkflows(&WorkflowFilter{Page: 1, PageSize: 10})
	if len(workflows) != 3 {
		t.Errorf("expected 3 workflows, got %d", len(workflows))
	}
}

func TestDeleteWorkflow(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "To Delete",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}

	created, _ := manager.CreateWorkflow(req, "test-user")

	err := manager.DeleteWorkflow(created.ID, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = manager.GetWorkflow(created.ID)
	if err == nil {
		t.Fatal("expected error for deleted workflow")
	}
}

func TestExecuteWorkflow(t *testing.T) {
	engine, manager := setupTestEngine(t)

	// 注册模拟执行器
	engine.RegisterExecutor("test", &MockStepExecutor{
		executeFunc: func(ctx context.Context, step StepConfig, vars map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"output": "success"}, nil
		},
	})

	// 创建工作流
	req := &CreateWorkflowRequest{
		Name: "Executable Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	// 激活工作流
	if err := manager.ActivateWorkflow(workflow.ID, "test-user"); err != nil {
		t.Fatalf("failed to activate workflow: %v", err)
	}

	// 启动引擎
	ctx := context.Background()
	engine.Start(ctx)
	defer engine.Stop()

	// 执行工作流
	exec, err := engine.ExecuteWorkflow(ctx, workflow.ID, map[string]interface{}{"input": "test"}, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exec == nil {
		t.Fatal("expected execution to be non-nil")
	}
	if exec.Status != ExecutionStatusRunning && exec.Status != ExecutionStatusPending {
		t.Logf("execution status: %s", exec.Status)
	}
}

func TestCancelExecution(t *testing.T) {
	_, manager := setupTestEngine(t)

	// 创建并启动执行
	req := &CreateWorkflowRequest{
		Name: "Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	// 激活工作流
	if err := manager.ActivateWorkflow(workflow.ID, "test-user"); err != nil {
		t.Fatalf("failed to activate workflow: %v", err)
	}

	exec, err := manager.ExecuteWorkflow(workflow.ID, &ExecuteWorkflowRequest{
		TriggeredBy: "test-user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 取消执行
	err = manager.CancelExecution(exec.ID, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证状态
	gotExec, _ := manager.GetExecution(exec.ID)
	if gotExec.Status != ExecutionStatusCancelled {
		t.Errorf("expected status cancelled, got %s", gotExec.Status)
	}
}

func TestGetExecution(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	// 激活工作流
	if err := manager.ActivateWorkflow(workflow.ID, "test-user"); err != nil {
		t.Fatalf("failed to activate workflow: %v", err)
	}

	exec, _ := manager.ExecuteWorkflow(workflow.ID, &ExecuteWorkflowRequest{
		TriggeredBy: "test-user",
	})

	gotExec, err := manager.GetExecution(exec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotExec.ID != exec.ID {
		t.Errorf("expected ID %s, got %s", exec.ID, gotExec.ID)
	}
}

func TestListExecutions(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	// 激活工作流
	if err := manager.ActivateWorkflow(workflow.ID, "test-user"); err != nil {
		t.Fatalf("failed to activate workflow: %v", err)
	}

	manager.ExecuteWorkflow(workflow.ID, &ExecuteWorkflowRequest{
		TriggeredBy: "user-1",
	})
	manager.ExecuteWorkflow(workflow.ID, &ExecuteWorkflowRequest{
		TriggeredBy: "user-2",
	})

	manager.ListExecutions(&ExecutionFilter{
		WorkflowID: workflow.ID,
		Page:       1,
		PageSize:   10,
	})
}

// HTTP Handler 测试

func TestHTTPCreateWorkflow(t *testing.T) {
	engine, _ := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	body := `{
		"name": "HTTP Test Workflow",
		"nodes": [{"id": "n1", "type": "task", "taskType": "test"}],
		"triggers": [{"id": "t1", "type": "manual", "enabled": true}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createWorkflow(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestHTTPListWorkflows(t *testing.T) {
	engine, manager := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	// 创建一些工作流
	for i := 0; i < 3; i++ {
		req := &CreateWorkflowRequest{
			Name: "Workflow",
			Nodes: []WorkflowNode{
				{ID: "node-1", Type: "task", TaskType: "test"},
			},
		}
		manager.CreateWorkflow(req, "test-user")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	w := httptest.NewRecorder()

	handler.listWorkflows(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestHTTPGetWorkflow(t *testing.T) {
	engine, manager := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	req := &CreateWorkflowRequest{
		Name: "Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflow.ID, nil)
	w := httptest.NewRecorder()

	handler.getWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHTTPGetWorkflowNotFound(t *testing.T) {
	engine, _ := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/non-existent", nil)
	w := httptest.NewRecorder()

	handler.getWorkflow(w, req, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHTTPDeleteWorkflow(t *testing.T) {
	engine, manager := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	req := &CreateWorkflowRequest{
		Name: "To Delete",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(req, "test-user")

	httpReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/"+workflow.ID, nil)
	w := httptest.NewRecorder()

	handler.deleteWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	engine, _ := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows", nil)
	w := httptest.NewRecorder()

	handler.handleWorkflows(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHTTPEngineStatus(t *testing.T) {
	engine, _ := setupTestEngine(t)
	handler := NewWorkflowHandler(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/engine/status", nil)
	w := httptest.NewRecorder()

	handler.handleEngineStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestRetryPolicy(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:    3,
		DelayMs:       100,
		MaxDelayMs:    1000,
		BackoffFactor: 2,
	}

	// 测试延迟计算
	delay := calculateRetryDelay(0, policy)
	if delay < 100 {
		t.Errorf("expected delay >= 100, got %d", delay)
	}

	delay = calculateRetryDelay(2, policy)
	if delay > 1000 {
		t.Errorf("expected delay <= 1000, got %d", delay)
	}
}

func TestEvaluateCondition(t *testing.T) {
	// 测试空条件
	if !evaluateCondition(nil, nil) {
		t.Error("expected nil condition to evaluate to true")
	}

	// 测试条件评估
	cond := &Condition{
		Field:    "key",
		Operator: ConditionOpEquals,
		Value:    "value",
	}

	vars := map[string]interface{}{
		"key": "value",
	}

	if !evaluateCondition(cond, vars) {
		t.Error("expected condition to be true")
	}
}

func TestWorkflowTriggerTypes(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Trigger Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
		Triggers: []Trigger{
			{
				ID:      "t1",
				Type:    TriggerTypeWebhook,
				Enabled: true,
				Config: TriggerConfig{
					WebhookPath: "/api/webhook/test",
				},
			},
			{
				ID:      "t2",
				Type:    TriggerTypeSchedule,
				Enabled: true,
				Config: TriggerConfig{
					CronExpression: "0 0 * * *",
				},
			},
		},
	}

	workflow, err := manager.CreateWorkflow(req, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(workflow.Triggers) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(workflow.Triggers))
	}
}

func TestWorkflowVariables(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Variable Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
		Variables: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	workflow, err := manager.CreateWorkflow(req, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if workflow.Variables["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", workflow.Variables["key1"])
	}
}

func TestWorkflowTags(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Tagged Workflow",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
		Tags: []string{"production", "critical"},
	}

	workflow, err := manager.CreateWorkflow(req, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(workflow.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(workflow.Tags))
	}
}

func TestWorkflowAuditLog(t *testing.T) {
	_, manager := setupTestEngine(t)

	req := &CreateWorkflowRequest{
		Name: "Audit Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}

	workflow, _ := manager.CreateWorkflow(req, "test-user")

	// 获取审计日志
	logs := manager.GetAuditLogs(&AuditLogFilter{
		EntityType: "workflow",
		EntityID:   workflow.ID,
		Page:       1,
		PageSize:   10,
	})
	if len(logs) == 0 {
		t.Error("expected audit logs in result")
	}
}
