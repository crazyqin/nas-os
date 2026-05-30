// Package workflowengine HTTP API 处理器测试
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

// setupHandlerTest 创建测试用的 handler 和 manager
func setupHandlerTest(t *testing.T) (*WorkflowHandler, *Manager) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	engine := NewWorkflowEngine(manager, EngineConfig{
		MaxConcurrent: 5,
		Timeout:       60,
	})
	handler := NewWorkflowHandler(engine)
	return handler, manager
}

// TestHandlerListWorkflows 测试列出工作流
func TestHandlerListWorkflows(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 预先创建工作流
	for i := 0; i < 3; i++ {
		req := &CreateWorkflowRequest{
			Name: "Test Workflow",
			Nodes: []WorkflowNode{
				{ID: "node-1", Type: "task", TaskType: "test"},
			},
		}
		manager.CreateWorkflow(req, "test-user")
	}

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	w := httptest.NewRecorder()

	handler.listWorkflows(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if int(data["total"].(float64)) != 3 {
		t.Errorf("expected 3 workflows, got %v", data["total"])
	}
}

// TestHandlerCreateWorkflow 测试创建工作流
func TestHandlerCreateWorkflow(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `{
		"name": "New Workflow",
		"description": "Test description",
		"nodes": [{"id": "n1", "type": "task", "taskType": "shell"}],
		"triggers": [{"id": "t1", "type": "manual", "enabled": true}],
		"tags": ["test", "automation"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "test-user")
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

	workflow, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a workflow map")
	}
	if workflow["name"] != "New Workflow" {
		t.Errorf("expected name 'New Workflow', got '%v'", workflow["name"])
	}
}

// TestHandlerCreateWorkflowInvalidBody 测试创建无效请求体
func TestHandlerCreateWorkflowInvalidBody(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandlerGetWorkflow 测试获取单个工作流
func TestHandlerGetWorkflow(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建工作流
	createReq := &CreateWorkflowRequest{
		Name: "Get Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflow.ID, nil)
	w := httptest.NewRecorder()

	handler.getWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

// TestHandlerGetWorkflowNotFound 测试获取不存在的工作流
func TestHandlerGetWorkflowNotFound(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/non-existent", nil)
	w := httptest.NewRecorder()

	handler.getWorkflow(w, httpReq, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandlerUpdateWorkflow 测试更新工作流
func TestHandlerUpdateWorkflow(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建工作流
	createReq := &CreateWorkflowRequest{
		Name: "Original Name",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")

	// 更新
	newName := "Updated Name"
	body, _ := json.Marshal(UpdateWorkflowRequest{
		Name: &newName,
	})
	httpReq := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+workflow.ID, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", "test-user")
	w := httptest.NewRecorder()

	handler.updateWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

// TestHandlerDeleteWorkflow 测试删除工作流
func TestHandlerDeleteWorkflow(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建工作流
	createReq := &CreateWorkflowRequest{
		Name: "To Delete",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")

	httpReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/"+workflow.ID, nil)
	httpReq.Header.Set("X-User-ID", "test-user")
	w := httptest.NewRecorder()

	handler.deleteWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestHandlerDeleteWorkflowNotFound 测试删除不存在的工作流
func TestHandlerDeleteWorkflowNotFound(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/non-existent", nil)
	w := httptest.NewRecorder()

	handler.deleteWorkflow(w, httpReq, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandlerExecuteWorkflow 测试执行工作流
func TestHandlerExecuteWorkflow(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建并激活工作流
	createReq := &CreateWorkflowRequest{
		Name: "Executable",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")
	manager.ActivateWorkflow(workflow.ID, "test-user")

	// 启动引擎
	ctx := context.Background()
	handler.engine.Start(ctx)
	defer handler.engine.Stop()

	body := `{"input": {"key": "value"}, "triggeredBy": "test-user"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflow.ID+"/execute", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.executeWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
	}
}

// TestHandlerGetWorkflowHistory 测试获取执行历史
func TestHandlerGetWorkflowHistory(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建工作流
	createReq := &CreateWorkflowRequest{
		Name: "History Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflow.ID+"/history", nil)
	w := httptest.NewRecorder()

	handler.getWorkflowHistory(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

// TestHandlerListTemplates 测试列出模板
func TestHandlerListTemplates(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflow/templates", nil)
	w := httptest.NewRecorder()

	handler.handleTemplates(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	// 内置模板应该存在
	total := int(data["total"].(float64))
	if total < 2 {
		t.Errorf("expected at least 2 builtin templates, got %d", total)
	}
}

// TestHandlerGetTemplate 测试获取单个模板
func TestHandlerGetTemplate(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflow/templates/tpl-backup", nil)
	w := httptest.NewRecorder()

	handler.handleTemplateByID(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

// TestHandlerGetTemplateNotFound 测试获取不存在的模板
func TestHandlerGetTemplateNotFound(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflow/templates/non-existent", nil)
	w := httptest.NewRecorder()

	handler.handleTemplateByID(w, httpReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandlerApplyTemplate 测试应用模板
func TestHandlerApplyTemplate(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `{"name": "My Backup Workflow"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflow/templates/tpl-backup/apply", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-User-ID", "test-user")
	w := httptest.NewRecorder()

	handler.applyTemplate(w, httpReq, "tpl-backup")

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}

	workflow, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a workflow map")
	}
	if workflow["name"] != "My Backup Workflow" {
		t.Errorf("expected name 'My Backup Workflow', got '%v'", workflow["name"])
	}
}

// TestHandlerApplyTemplateMissingName 测试应用模板缺少名称
func TestHandlerApplyTemplateMissingName(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `{}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflow/templates/tpl-backup/apply", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.applyTemplate(w, httpReq, "tpl-backup")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandlerApplyTemplateNotFound 测试应用不存在的模板
func TestHandlerApplyTemplateNotFound(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `{"name": "Test"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflow/templates/non-existent/apply", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.applyTemplate(w, httpReq, "non-existent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandlerEngineStatus 测试引擎状态
func TestHandlerEngineStatus(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/engine/status", nil)
	w := httptest.NewRecorder()

	handler.handleEngineStatus(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

// TestHandlerEngineStatusMethodNotAllowed 测试引擎状态不允许的方法
func TestHandlerEngineStatusMethodNotAllowed(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/engine/status", nil)
	w := httptest.NewRecorder()

	handler.handleEngineStatus(w, httpReq)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestHandlerCancelWorkflow 测试取消工作流执行
func TestHandlerCancelWorkflow(t *testing.T) {
	handler, manager := setupHandlerTest(t)

	// 创建并激活工作流
	createReq := &CreateWorkflowRequest{
		Name: "Cancel Test",
		Nodes: []WorkflowNode{
			{ID: "node-1", Type: "task", TaskType: "test"},
		},
	}
	workflow, _ := manager.CreateWorkflow(createReq, "test-user")
	manager.ActivateWorkflow(workflow.ID, "test-user")

	// 启动执行
	exec, _ := manager.ExecuteWorkflow(workflow.ID, &ExecuteWorkflowRequest{
		TriggeredBy: "test-user",
	})

	body := `{"executionId": "` + exec.ID + `"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflow.ID+"/cancel", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.cancelWorkflow(w, httpReq, workflow.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestHandlerCancelWorkflowMissingExecutionID 测试取消执行缺少 executionId
func TestHandlerCancelWorkflowMissingExecutionID(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	body := `{}`
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/test/cancel", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.cancelWorkflow(w, httpReq, "test")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandlerMethodNotAllowed 测试不允许的 HTTP 方法
func TestHandlerMethodNotAllowed(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodPut, "/api/v1/workflows", nil)
	w := httptest.NewRecorder()

	handler.handleWorkflows(w, httpReq)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestHandlerTemplatesMethodNotAllowed 测试模板端点不允许的方法
func TestHandlerTemplatesMethodNotAllowed(t *testing.T) {
	handler, _ := setupHandlerTest(t)

	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflow/templates", nil)
	w := httptest.NewRecorder()

	handler.handleTemplates(w, httpReq)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
