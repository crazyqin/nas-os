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

// ============================================================
// 新增模块测试
// ============================================================

func TestWorkflowEngine(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-wf-001",
		Role:          RoleSystemAdmin,
		Name:          "Workflow Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	engine := agent.GetWorkflowEngine()
	if engine == nil {
		t.Fatal("GetWorkflowEngine returned nil")
	}

	// 测试模板列表
	templates := engine.ListTemplates(nil)
	if len(templates) == 0 {
		t.Error("expected default workflow templates to be registered")
	}

	// 测试按分类查询
	cat := CategoryMonitor
	monitorTemplates := engine.ListTemplates(&cat)
	if len(monitorTemplates) == 0 {
		t.Error("expected monitor category templates")
	}

	// 测试获取模板
	tmpl, err := engine.GetTemplate("tpl_health_patrol")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tmpl.Name != "系统健康巡检" {
		t.Errorf("expected template name '系统健康巡检', got %q", tmpl.Name)
	}

	// 测试执行工作流
	instance, err := engine.Execute("tpl_health_patrol", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if instance == nil {
		t.Fatal("expected non-nil instance")
	}
	if instance.Status != WfStatusRunning && instance.Status != WfStatusCompleted {
		t.Errorf("unexpected instance status: %s", instance.Status)
	}

	// 等待执行完成
	time.Sleep(1 * time.Second)

	// 获取实例状态
	retrieved, err := engine.GetInstance(instance.ID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected non-nil retrieved instance")
	}
}

func TestWorkflowEngineCancelInstance(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-wf-002",
		Role:          RoleSystemAdmin,
		Name:          "Cancel Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	engine := agent.GetWorkflowEngine()

	instance, err := engine.Execute("tpl_health_patrol", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	err = engine.CancelInstance(instance.ID)
	if err != nil {
		t.Fatalf("CancelInstance failed: %v", err)
	}

	retrieved, _ := engine.GetInstance(instance.ID)
	if retrieved.Status != WfStatusCancelled {
		t.Errorf("expected cancelled status, got %s", retrieved.Status)
	}
}

func TestWorkflowEngineRegisterTemplate(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-wf-003",
		Role:          RoleSystemAdmin,
		Name:          "Register Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	engine := agent.GetWorkflowEngine()

	// 注册自定义模板
	customTmpl := &WorkflowTemplate{
		ID:          "tpl_custom_test",
		Name:        "自定义测试模板",
		Description: "用于测试的自定义模板",
		Category:    CategoryMaintenance,
		Steps: []StepDefinition{
			{ID: "step1", Name: "步骤1", Action: "test_action", Timeout: 10 * time.Second},
		},
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	err := engine.RegisterTemplate(customTmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// 验证注册成功
	tmpl, err := engine.GetTemplate("tpl_custom_test")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tmpl.Name != "自定义测试模板" {
		t.Errorf("expected name '自定义测试模板', got %q", tmpl.Name)
	}

	// 测试重复注册
	err = engine.RegisterTemplate(customTmpl)
	if err == nil {
		t.Error("expected error for duplicate template registration")
	}

	// 测试注销
	err = engine.UnregisterTemplate("tpl_custom_test")
	if err != nil {
		t.Fatalf("UnregisterTemplate failed: %v", err)
	}

	_, err = engine.GetTemplate("tpl_custom_test")
	if err == nil {
		t.Error("expected error for unregistered template")
	}
}

func TestToolRegistry(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-tool-001",
		Role:          RoleSystemAdmin,
		Name:          "Tool Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	registry := agent.GetToolRegistry()
	if registry == nil {
		t.Fatal("GetToolRegistry returned nil")
	}

	// 测试工具列表
	tools := registry.ListTools(nil, false)
	if len(tools) == 0 {
		t.Error("expected default tools to be registered")
	}

	// 测试按分类查询
	catID := "cat_system"
	sysTools := registry.ListTools(&catID, false)
	if len(sysTools) == 0 {
		t.Error("expected system category tools")
	}

	// 测试获取工具
	tool, err := registry.GetTool("tool_cpu_check")
	if err != nil {
		t.Fatalf("GetTool failed: %v", err)
	}
	if tool.Name != "CPU检查" {
		t.Errorf("expected tool name 'CPU检查', got %q", tool.Name)
	}

	// 测试执行动作
	err = registry.ExecuteAction("check_cpu", nil)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}

	// 测试按动作查找工具
	found, err := registry.FindToolByAction("check_memory")
	if err != nil {
		t.Fatalf("FindToolByAction failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected non-nil tool")
	}

	// 测试统计信息
	stats := registry.GetStats()
	if stats["total_tools"] == 0 {
		t.Error("expected non-zero total tools in stats")
	}
}

func TestToolRegistryRegister(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-tool-002",
		Role:          RoleSystemAdmin,
		Name:          "Tool Register Test",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	registry := agent.GetToolRegistry()

	// 注册自定义工具
	customTool := &RegisteredTool{
		ID:          "tool_custom_test",
		Name:        "自定义测试工具",
		Description: "用于测试的自定义工具",
		Category:    "cat_system",
		Version:     "1.0",
		Enabled:     true,
		Timeout:     10 * time.Second,
		Actions: []ToolAction{
			{Name: "custom_action", Description: "自定义动作"},
		},
		handler: func(action string, params map[string]interface{}) error {
			return nil
		},
	}

	err := registry.Register(customTool)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 验证注册成功
	tool, err := registry.GetTool("tool_custom_test")
	if err != nil {
		t.Fatalf("GetTool failed: %v", err)
	}
	if tool.Name != "自定义测试工具" {
		t.Errorf("expected name '自定义测试工具', got %q", tool.Name)
	}

	// 测试执行自定义动作
	err = registry.ExecuteAction("custom_action", nil)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}

	// 测试禁用工具
	err = registry.DisableTool("tool_custom_test")
	if err != nil {
		t.Fatalf("DisableTool failed: %v", err)
	}

	_, err = registry.GetTool("tool_custom_test")
	if err != nil {
		t.Fatalf("GetTool should still work when disabled: %v", err)
	}

	// 测试启用工具
	err = registry.EnableTool("tool_custom_test")
	if err != nil {
		t.Fatalf("EnableTool failed: %v", err)
	}

	// 测试注销
	err = registry.Unregister("tool_custom_test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}
}

func TestGuardrails(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-guard-001",
		Role:          RoleSystemAdmin,
		Name:          "Guardrails Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	guard := agent.GetGuardrails()
	if guard == nil {
		t.Fatal("GetGuardrails returned nil")
	}

	// 测试正常操作
	err := guard.CheckOperation("admin", "check_cpu", "system")
	if err != nil {
		t.Errorf("expected normal operation to be allowed: %v", err)
	}

	// 测试被禁止的操作
	err = guard.CheckOperation("admin", "rm -rf /", "system")
	if err == nil {
		t.Error("expected blocked operation to be rejected")
	}

	// 测试资源限制检查
	health := &SystemHealth{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
	}
	warnings := guard.CheckResourceLimits(health)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for normal usage, got %d", len(warnings))
	}

	// 测试超限
	health.CPUUsage = 98.0
	health.MemoryUsage = 96.0
	warnings = guard.CheckResourceLimits(health)
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings for high usage, got %d", len(warnings))
	}

	// 测试审计日志
	auditLog := guard.GetAuditLog(10)
	if len(auditLog) == 0 {
		t.Error("expected audit log entries")
	}

	// 测试配置获取
	config := guard.GetConfig()
	if !config.AuditEnabled {
		t.Error("expected audit to be enabled")
	}
}

func TestGuardrailsApproval(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-guard-002",
		Role:          RoleSystemAdmin,
		Name:          "Approval Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	guard := agent.GetGuardrails()

	// 请求审批
	request, err := guard.RequestApproval("admin", "dangerous_action", "system", "需要执行危险操作")
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if request.Status != ApprovalPending {
		t.Errorf("expected pending status, got %s", request.Status)
	}

	// 列出待审批
	pending := guard.ListPendingApprovals()
	if len(pending) == 0 {
		t.Error("expected pending approvals")
	}

	// 批准请求
	err = guard.ApproveRequest(request.ID, "superadmin", "已审核通过")
	if err != nil {
		t.Fatalf("ApproveRequest failed: %v", err)
	}

	retrieved, _ := guard.GetApprovalRequest(request.ID)
	if retrieved.Status != ApprovalApproved {
		t.Errorf("expected approved status, got %s", retrieved.Status)
	}
}

func TestGuardrailsRejectApproval(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-guard-003",
		Role:          RoleSystemAdmin,
		Name:          "Reject Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	guard := agent.GetGuardrails()

	request, _ := guard.RequestApproval("user1", "test_action", "resource", "测试")

	err := guard.RejectRequest(request.ID, "admin", "不批准")
	if err != nil {
		t.Fatalf("RejectRequest failed: %v", err)
	}

	retrieved, _ := guard.GetApprovalRequest(request.ID)
	if retrieved.Status != ApprovalRejected {
		t.Errorf("expected rejected status, got %s", retrieved.Status)
	}
}

func TestGuidedWizard(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-wizard-001",
		Role:          RoleSystemAdmin,
		Name:          "Wizard Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	wizard := agent.GetWizard()
	if wizard == nil {
		t.Fatal("GetWizard returned nil")
	}

	// 测试模板列表
	templates := wizard.ListTemplates(nil)
	if len(templates) == 0 {
		t.Error("expected default wizard templates")
	}

	// 启动会话
	session, err := wizard.StartSession("wizard_create_storage_pool", "test_user")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if session.Status != WizardStatusActive {
		t.Errorf("expected active status, got %s", session.Status)
	}

	// 获取当前步骤
	stepInfo, err := wizard.GetCurrentStep(session.ID)
	if err != nil {
		t.Fatalf("GetCurrentStep failed: %v", err)
	}
	if stepInfo.StepIndex != 0 {
		t.Errorf("expected step index 0, got %d", stepInfo.StepIndex)
	}
	if stepInfo.TotalSteps == 0 {
		t.Error("expected non-zero total steps")
	}

	// 提交响应
	err = wizard.SubmitStepResponse(session.ID, map[string]interface{}{
		"pool_name": "TestVolume",
	})
	if err != nil {
		t.Fatalf("SubmitStepResponse failed: %v", err)
	}

	// 验证前进到下一步
	stepInfo, _ = wizard.GetCurrentStep(session.ID)
	if stepInfo.StepIndex != 1 {
		t.Errorf("expected step index 1, got %d", stepInfo.StepIndex)
	}

	// 测试后退
	err = wizard.GoBack(session.ID)
	if err != nil {
		t.Fatalf("GoBack failed: %v", err)
	}

	stepInfo, _ = wizard.GetCurrentStep(session.ID)
	if stepInfo.StepIndex != 0 {
		t.Errorf("expected step index 0 after GoBack, got %d", stepInfo.StepIndex)
	}

	// 测试取消
	err = wizard.CancelSession(session.ID)
	if err != nil {
		t.Fatalf("CancelSession failed: %v", err)
	}

	retrieved, _ := wizard.GetSession(session.ID)
	if retrieved.Status != WizardStatusCancelled {
		t.Errorf("expected cancelled status, got %s", retrieved.Status)
	}
}

func TestGuidedWizardValidation(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-wizard-002",
		Role:          RoleSystemAdmin,
		Name:          "Validation Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	wizard := agent.GetWizard()

	session, _ := wizard.StartSession("wizard_create_storage_pool", "test_user")

	// 测试必填字段验证（缺少pool_name）
	err := wizard.SubmitStepResponse(session.ID, map[string]interface{}{})
	if err == nil {
		t.Error("expected validation error for missing required field")
	}
}

func TestDiagnosticAgent(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-diag-001",
		Role:          RoleSystemAdmin,
		Name:          "Diagnostic Test Agent",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	diag := agent.GetDiagnostic()
	if diag == nil {
		t.Fatal("GetDiagnostic returned nil")
	}

	// 测试正常状态诊断
	health := &SystemHealth{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
		Temperature: 45.0,
	}

	summary := diag.RunDiagnosis(health)
	if summary == nil {
		t.Fatal("RunDiagnosis returned nil")
	}
	if summary.OverallHealth != "good" {
		t.Errorf("expected overall health 'good', got %q", summary.OverallHealth)
	}
	if summary.Score < 80 {
		t.Errorf("expected score >= 80 for healthy system, got %d", summary.Score)
	}
}

func TestDiagnosticAgentWithIssues(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-diag-002",
		Role:          RoleSystemAdmin,
		Name:          "Issue Diagnostic Test",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	diag := agent.GetDiagnostic()

	// 测试有问题的状态
	health := &SystemHealth{
		CPUUsage:    95.0,
		MemoryUsage: 92.0,
		DiskUsage:   96.0,
		Temperature: 85.0,
	}

	summary := diag.RunDiagnosis(health)
	if summary.OverallHealth == "good" {
		t.Error("expected non-good health for critical usage")
	}
	if summary.Issues == 0 {
		t.Error("expected issues to be detected")
	}
	if summary.Score >= 80 {
		t.Errorf("expected score < 80 for critical system, got %d", summary.Score)
	}

	// 检查诊断历史
	diagnoses := diag.GetDiagnoses(10)
	if len(diagnoses) == 0 {
		t.Error("expected diagnosis results in history")
	}
}

func TestDiagnosticAgentAlertRules(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:       "agent-diag-003",
		Role:          RoleSystemAdmin,
		Name:          "Alert Rule Test",
		ScanInterval:  1 * time.Hour,
		MaxConcurrent: 2,
	})

	diag := agent.GetDiagnostic()

	// 测试默认告警规则
	rules := diag.ListAlertRules()
	if len(rules) == 0 {
		t.Error("expected default alert rules")
	}

	// 添加自定义规则
	customRule := &AlertRule{
		ID:        "rule_custom_test",
		Name:      "自定义测试规则",
		Enabled:   true,
		Metric:    "custom_metric",
		Condition: "gt",
		Threshold: 100.0,
		Severity:  SeverityWarning,
		Message:   "自定义指标超限",
	}
	diag.AddAlertRule(customRule)

	rules = diag.ListAlertRules()
	found := false
	for _, r := range rules {
		if r.ID == "rule_custom_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom rule to be in list")
	}
}

func TestAgentModulesIntegration(t *testing.T) {
	agent := NewAgent(AgentConfig{
		AgentID:         "agent-integration-001",
		Role:            RoleSystemAdmin,
		Name:            "Integration Test Agent",
		ScanInterval:    1 * time.Hour,
		AutoRemediation: true,
		MaxConcurrent:   4,
	})

	// 验证所有模块都已初始化
	if agent.GetWorkflowEngine() == nil {
		t.Error("WorkflowEngine not initialized")
	}
	if agent.GetToolRegistry() == nil {
		t.Error("ToolRegistry not initialized")
	}
	if agent.GetGuardrails() == nil {
		t.Error("Guardrails not initialized")
	}
	if agent.GetWizard() == nil {
		t.Error("GuidedWizard not initialized")
	}
	if agent.GetDiagnostic() == nil {
		t.Error("DiagnosticAgent not initialized")
	}

	// 验证状态包含模块信息
	status := agent.GetAgentStatus()
	modules, ok := status["modules"]
	if !ok {
		t.Error("expected modules in agent status")
	}
	if modules == nil {
		t.Error("expected non-nil modules")
	}

	// 测试系统诊断
	agent.health = &SystemHealth{
		CPUUsage:    45.0,
		MemoryUsage: 55.0,
		DiskUsage:   65.0,
		Temperature: 40.0,
	}
	summary := agent.RunDiagnostic()
	if summary == nil {
		t.Fatal("RunDiagnostic returned nil")
	}
	if summary.OverallHealth != "good" {
		t.Errorf("expected good health, got %q", summary.OverallHealth)
	}
}
