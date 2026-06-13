package dsmagentorch

import (
	"testing"
)

func TestManagerStartStop(t *testing.T) {
	m := NewManager(nil)
	if m.IsRunning() {
		t.Fatal("新创建的管理器不应在运行")
	}
	if err := m.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("管理器应该在运行")
	}
	if err := m.Start(); err == nil {
		t.Fatal("重复启动应返回错误")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("管理器不应在运行")
	}
}

func TestRegisterTool(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	tool := &MCPTool{
		Name:        "disk_status",
		Description: "获取磁盘状态",
		Enabled:     true,
	}
	if err := m.RegisterTool(tool); err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}
	tools := m.ListTools()
	if len(tools) != 1 {
		t.Fatalf("期望1个工具，实际 %d", len(tools))
	}
}

func TestRegisterToolNotRunning(t *testing.T) {
	m := NewManager(nil)
	tool := &MCPTool{Name: "test", Description: "test"}
	if err := m.RegisterTool(tool); err == nil {
		t.Fatal("未运行时注册应返回错误")
	}
}

func TestUnregisterTool(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.RegisterTool(&MCPTool{Name: "test", Description: "test"})
	if err := m.UnregisterTool("test"); err != nil {
		t.Fatalf("注销工具失败: %v", err)
	}
	if len(m.ListTools()) != 0 {
		t.Fatal("工具应该已被注销")
	}
}

func TestUnregisterToolNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.UnregisterTool("nonexistent"); err == nil {
		t.Fatal("注销不存在的工具应返回错误")
	}
}

func TestCreateTask(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	task, err := m.CreateTask(WorkflowHealthCheck, "检查系统健康", 1)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if task.ID == "" {
		t.Fatal("任务ID不能为空")
	}
	if task.Status != TaskPending {
		t.Fatal("任务状态应为pending")
	}
}

func TestCreateTaskNotRunning(t *testing.T) {
	m := NewManager(nil)
	_, err := m.CreateTask(WorkflowHealthCheck, "test", 1)
	if err == nil {
		t.Fatal("未运行时创建任务应返回错误")
	}
}

func TestCompleteTask(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	task, _ := m.CreateTask(WorkflowHealthCheck, "test", 1)
	if err := m.CompleteTask(task.ID, true, "成功"); err != nil {
		t.Fatalf("完成任务失败: %v", err)
	}
	completed, _ := m.GetTask(task.ID)
	if completed.Status != TaskCompleted {
		t.Fatal("任务状态应为completed")
	}
	if !completed.Result.Success {
		t.Fatal("任务结果应为成功")
	}
}

func TestCompleteTaskFailed(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	task, _ := m.CreateTask(WorkflowSecurityScan, "scan", 1)
	m.CompleteTask(task.ID, false, "扫描失败")
	completed, _ := m.GetTask(task.ID)
	if completed.Status != TaskFailed {
		t.Fatal("任务状态应为failed")
	}
}

func TestCompleteTaskNotFound(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.CompleteTask("nonexistent", true, ""); err == nil {
		t.Fatal("完成不存在的任务应返回错误")
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateTask(WorkflowHealthCheck, "task1", 1)
	task2, _ := m.CreateTask(WorkflowSecurityScan, "task2", 2)
	m.CompleteTask(task2.ID, true, "done")

	pending := m.ListTasks(TaskPending)
	if len(pending) != 1 {
		t.Fatalf("期望1个pending任务，实际 %d", len(pending))
	}
	completed := m.ListTasks(TaskCompleted)
	if len(completed) != 1 {
		t.Fatalf("期望1个completed任务，实际 %d", len(completed))
	}
	all := m.ListTasks("")
	if len(all) != 2 {
		t.Fatalf("期望2个任务，实际 %d", len(all))
	}
}

func TestGuardrails(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	g := &Guardrail{
		Name:        "no_root_delete",
		Description: "禁止删除根目录",
		Rule:        "path != '/'",
		Enabled:     true,
		Action:      "block",
	}
	m.AddGuardrail(g)
	stats := m.GetStats()
	if stats["total_guardrails"] != 1 {
		t.Fatalf("期望1个护栏，实际 %v", stats["total_guardrails"])
	}
}

func TestRunDiagnostics(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	diags := m.RunDiagnostics()
	if len(diags) != 3 {
		t.Fatalf("期望3个诊断，实际 %d", len(diags))
	}
	for _, d := range diags {
		if d.Status != "ok" {
			t.Fatalf("诊断 %s 状态异常: %s", d.Name, d.Status)
		}
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.RegisterTool(&MCPTool{Name: "test", Description: "test"})
	task, _ := m.CreateTask(WorkflowHealthCheck, "test", 1)
	m.CompleteTask(task.ID, true, "done")

	stats := m.GetStats()
	if stats["running"] != true {
		t.Fatal("应该在运行")
	}
	if stats["total_tasks"] != 1 {
		t.Fatalf("期望1个任务，实际 %v", stats["total_tasks"])
	}
	if stats["total_tools"] != 1 {
		t.Fatalf("期望1个工具，实际 %v", stats["total_tools"])
	}
	if stats["completed"] != 1 {
		t.Fatalf("期望1个完成，实际 %v", stats["completed"])
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultAgentConfig()
	if config.AgentID == "" {
		t.Fatal("AgentID不能为空")
	}
	if config.Role != RoleSystemAdmin {
		t.Fatal("默认角色应为system_admin")
	}
	if !config.AutoRemediation {
		t.Fatal("默认应启用自动修复")
	}
}
