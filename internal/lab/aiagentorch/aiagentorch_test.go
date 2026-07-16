package aiagentorch

import (
	"testing"
)

// ==================== 辅助函数 ====================

func newTestManager() *Manager {
	return NewManager()
}

func newTestAgent(name string, agentType AgentType) *Agent {
	return &Agent{
		Name:        name,
		Description: "Test agent",
		Type:        agentType,
		Config: AgentConfig{
			ModelName:   "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
			Parameters:  map[string]string{},
		},
		Permission: AgentPermission{
			MaxConcurrent:   3,
			RateLimitPerMin: 60,
		},
		Tags:          []string{"test"},
		EventTriggers: []EventTrigger{},
	}
}

// ==================== Manager 测试 ====================

func TestNewManager(t *testing.T) {
	m := newTestManager()
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
}

func TestCreateAgent(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)

	if err := m.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent 失败: %v", err)
	}
	if agent.ID == "" {
		t.Fatal("代理 ID 未生成")
	}
	if agent.Status != AgentStatusInactive {
		t.Fatalf("期望状态 inactive，得到 %s", agent.Status)
	}
}

func TestCreateAgentDuplicateName(t *testing.T) {
	m := newTestManager()
	agent1 := newTestAgent("same-name", AgentTypeStorageOptim)
	agent2 := newTestAgent("same-name", AgentTypeBackup)

	m.CreateAgent(agent1)
	if err := m.CreateAgent(agent2); err != ErrAgentNameExists {
		t.Fatalf("期望 ErrAgentNameExists，得到 %v", err)
	}
}

func TestCreateAgentInvalidConfig(t *testing.T) {
	m := newTestManager()
	agent := &Agent{Name: "", Type: AgentTypeStorageOptim}

	if err := m.CreateAgent(agent); err != ErrInvalidConfig {
		t.Fatalf("期望 ErrInvalidConfig，得到 %v", err)
	}
}

func TestGetAgent(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	got, err := m.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent 失败: %v", err)
	}
	if got.Name != "test-agent" {
		t.Fatalf("期望名称 test-agent，得到 %s", got.Name)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Fatalf("期望 ErrAgentNotFound，得到 %v", err)
	}
}

func TestListAgents(t *testing.T) {
	m := newTestManager()
	m.CreateAgent(newTestAgent("agent1", AgentTypeStorageOptim))
	m.CreateAgent(newTestAgent("agent2", AgentTypeBackup))
	m.CreateAgent(newTestAgent("agent3", AgentTypeStorageOptim))

	agents, total := m.ListAgents(AgentTypeStorageOptim, "", 1, 10)
	if total != 2 {
		t.Fatalf("期望 2 个代理，得到 %d", total)
	}
	if len(agents) != 2 {
		t.Fatalf("期望返回 2 个代理，得到 %d", len(agents))
	}
}

func TestUpdateAgent(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	update := &Agent{
		Name:        "updated-agent",
		Description: "Updated description",
	}
	if err := m.UpdateAgent(agent.ID, update); err != nil {
		t.Fatalf("UpdateAgent 失败: %v", err)
	}

	got, _ := m.GetAgent(agent.ID)
	if got.Name != "updated-agent" {
		t.Fatalf("期望名称 updated-agent，得到 %s", got.Name)
	}
}

func TestDeleteAgent(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	if err := m.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent 失败: %v", err)
	}

	_, err := m.GetAgent(agent.ID)
	if err != ErrAgentNotFound {
		t.Fatalf("期望 ErrAgentNotFound，得到 %v", err)
	}
}

func TestEnableDisableAgent(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	if err := m.EnableAgent(agent.ID); err != nil {
		t.Fatalf("EnableAgent 失败: %v", err)
	}
	got, _ := m.GetAgent(agent.ID)
	if got.Status != AgentStatusActive {
		t.Fatalf("期望状态 active，得到 %s", got.Status)
	}

	if err := m.DisableAgent(agent.ID); err != nil {
		t.Fatalf("DisableAgent 失败: %v", err)
	}
	got, _ = m.GetAgent(agent.ID)
	if got.Status != AgentStatusInactive {
		t.Fatalf("期望状态 inactive，得到 %s", got.Status)
	}
}

// ==================== Task 测试 ====================

func TestCreateTask(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	task := &AgentTask{
		AgentID:     agent.ID,
		Name:        "test-task",
		Description: "Test task",
		Enabled:     true,
	}
	if err := m.CreateTask(task); err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if task.ID == "" {
		t.Fatal("任务 ID 未生成")
	}
}

func TestCreateTaskAgentNotFound(t *testing.T) {
	m := newTestManager()
	task := &AgentTask{
		AgentID: "nonexistent",
		Name:    "test-task",
	}
	if err := m.CreateTask(task); err != ErrAgentNotFound {
		t.Fatalf("期望 ErrAgentNotFound，得到 %v", err)
	}
}

func TestGetTask(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	task := &AgentTask{
		AgentID: agent.ID,
		Name:    "test-task",
		Enabled: true,
	}
	m.CreateTask(task)

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	if got.Name != "test-task" {
		t.Fatalf("期望名称 test-task，得到 %s", got.Name)
	}
}

func TestExecuteTask(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)
	m.EnableAgent(agent.ID)

	task := &AgentTask{
		AgentID: agent.ID,
		Name:    "test-task",
		Enabled: true,
	}
	m.CreateTask(task)

	log, err := m.ExecuteTask(task.ID)
	if err != nil {
		t.Fatalf("ExecuteTask 失败: %v", err)
	}
	if log == nil {
		t.Fatal("ExecuteTask 返回 nil log")
	}
}

// ==================== Message 测试 ====================

func TestSendMessage(t *testing.T) {
	m := newTestManager()
	agent1 := newTestAgent("agent1", AgentTypeStorageOptim)
	agent2 := newTestAgent("agent2", AgentTypeBackup)
	m.CreateAgent(agent1)
	m.CreateAgent(agent2)

	msg, err := m.SendMessage(agent1.ID, agent2.ID, "text", "Hello", PriorityNormal)
	if err != nil {
		t.Fatalf("SendMessage 失败: %v", err)
	}
	if msg == nil || msg.ID == "" {
		t.Fatal("消息 ID 未生成")
	}
}

func TestGetMessages(t *testing.T) {
	m := newTestManager()
	agent1 := newTestAgent("agent1", AgentTypeStorageOptim)
	agent2 := newTestAgent("agent2", AgentTypeBackup)
	m.CreateAgent(agent1)
	m.CreateAgent(agent2)

	m.SendMessage(agent1.ID, agent2.ID, "text", "Hello", PriorityNormal)
	m.SendMessage(agent2.ID, agent1.ID, "text", "Hi", PriorityNormal)

	msgs, total := m.GetMessages(agent1.ID, false, 1, 10)
	if total < 2 {
		t.Fatalf("期望至少 2 条消息，得到 %d", total)
	}
	if len(msgs) < 2 {
		t.Fatalf("期望返回至少 2 条消息，得到 %d", len(msgs))
	}
}

// ==================== Stats 测试 ====================

func TestGetStats(t *testing.T) {
	m := newTestManager()
	agent := newTestAgent("test-agent", AgentTypeStorageOptim)
	m.CreateAgent(agent)

	stats := m.GetStats()
	if stats.TotalAgents < 1 {
		t.Fatalf("期望至少 1 个代理，得到 %d", stats.TotalAgents)
	}
}

func TestGetAgentCount(t *testing.T) {
	m := newTestManager()
	m.CreateAgent(newTestAgent("agent1", AgentTypeStorageOptim))
	m.CreateAgent(newTestAgent("agent2", AgentTypeBackup))

	count := m.GetAgentCount()
	if count != 2 {
		t.Fatalf("期望 2 个代理，得到 %d", count)
	}
}
