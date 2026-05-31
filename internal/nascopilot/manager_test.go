package nascopilot

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestManager() *Manager {
	return NewManager()
}

func setupTestRouter(m *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlers(m)
	h.RegisterRoutes(r.Group("/api"))
	return r
}

// ========== 对话测试 ==========

func TestCreateConversation(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "测试对话")

	assert.NotEmpty(t, conv.ID)
	assert.Equal(t, "user1", conv.UserID)
	assert.Equal(t, "测试对话", conv.Title)
	assert.Equal(t, ConverStatusActive, conv.Status)
}

func TestListConversations(t *testing.T) {
	m := setupTestManager()
	m.CreateConversation("user1", "对话1")
	m.CreateConversation("user1", "对话2")
	m.CreateConversation("user2", "对话3")

	// user1 的对话
	convList := m.ListConversations("user1")
	assert.Len(t, convList, 2)

	// 所有对话
	allConv := m.ListConversations("")
	assert.Len(t, allConv, 3)
}

func TestGetConversation(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "测试对话")

	gotConv, messages, err := m.GetConversation(conv.ID)
	assert.NoError(t, err)
	assert.Equal(t, conv.ID, gotConv.ID)
	assert.Empty(t, messages) // 初始无消息
}

func TestDeleteConversation(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "测试对话")

	err := m.DeleteConversation(conv.ID)
	assert.NoError(t, err)

	_, _, err = m.GetConversation(conv.ID)
	assert.Error(t, err)
}

func TestSendMessage(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "测试对话")

	resp, err := m.SendMessage(conv.ID, "查看磁盘存储空间", "user1")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, conv.ID, resp.ConversationID)
	assert.Equal(t, RoleAssistant, resp.Message.Role)
	assert.NotNil(t, resp.Intent)
}

func TestSendMessageToNonExistentConversation(t *testing.T) {
	m := setupTestManager()

	_, err := m.SendMessage("non-existent", "hello", "user1")
	assert.Error(t, err)
}

// ========== 意图解析测试 ==========

func TestParseIntentStorage(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("查看磁盘空间使用情况")
	assert.Equal(t, IntentStorage, intent.Type)
	assert.Greater(t, intent.Confidence, 0.0)
}

func TestParseIntentBackup(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("创建数据备份")
	assert.Equal(t, IntentBackup, intent.Type)
}

func TestParseIntentNetwork(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("修改网络配置")
	assert.Equal(t, IntentNetwork, intent.Type)
}

func TestParseIntentDocker(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("查看运行中的容器")
	assert.Equal(t, IntentDocker, intent.Type)
}

func TestParseIntentUser(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("修改用户密码")
	assert.Equal(t, IntentUser, intent.Type)
}

func TestParseIntentSystem(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("重启系统服务")
	assert.Equal(t, IntentSystem, intent.Type)
}

func TestParseIntentAction(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("执行清理任务")
	assert.Equal(t, IntentAction, intent.Type)
}

func TestParseIntentUnknown(t *testing.T) {
	m := setupTestManager()
	intent := m.ParseIntent("今天天气怎么样")
	assert.Equal(t, IntentQuery, intent.Type)
	assert.Equal(t, 0.5, intent.Confidence)
}

// ========== 命令执行测试 ==========

func TestExecuteCommand(t *testing.T) {
	m := setupTestManager()
	cmd := Command{
		Verb:         CommandCreate,
		ResourceType: "user",
		Parameters:   map[string]string{"name": "testuser"},
		Status:       CommandStatusPending,
	}

	result := m.ExecuteCommand(cmd)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.Message)
}

func TestExecuteCommandViaChat(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "命令测试")

	resp, err := m.SendMessage(conv.ID, "新建共享文件夹", "user1")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Message.CommandResult)
	assert.True(t, resp.Message.CommandResult.Success)
}

// ========== 知识库测试 ==========

func TestAddAndListKnowledge(t *testing.T) {
	m := setupTestManager()
	entry := m.AddKnowledge(KnowledgeEntry{
		Category: KnowledgeSecurity,
		Title:    "防火墙配置",
		Content:  "如何配置 NAS 防火墙",
		Tags:     []string{"安全", "防火墙"},
	})

	assert.NotEmpty(t, entry.ID)

	entries := m.ListKnowledge()
	assert.Len(t, entries, 1)
	assert.Equal(t, "防火墙配置", entries[0].Title)
}

func TestSearchKnowledge(t *testing.T) {
	m := setupTestManager()
	m.AddKnowledge(KnowledgeEntry{
		Category: KnowledgeSecurity,
		Title:    "防火墙配置指南",
		Content:  "NAS 防火墙安全设置",
		Tags:     []string{"安全"},
	})
	m.AddKnowledge(KnowledgeEntry{
		Category: KnowledgeBackup,
		Title:    "备份策略",
		Content:  "定期备份数据的最佳实践",
		Tags:     []string{"备份"},
	})

	results := m.SearchKnowledge("防火墙")
	assert.Len(t, results, 1)
	assert.Equal(t, "防火墙配置指南", results[0].Title)

	results = m.SearchKnowledge("备份")
	assert.Len(t, results, 1)
}

// ========== 定时任务测试 ==========

func TestCreateAndListTasks(t *testing.T) {
	m := setupTestManager()
	task := m.CreateScheduledTask("每日备份", "0 2 * * *", "backup --all", true)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "每日备份", task.Description)
	assert.True(t, task.Enabled)

	tasks := m.ListScheduledTasks()
	assert.Len(t, tasks, 1)
}

func TestUpdateScheduledTask(t *testing.T) {
	m := setupTestManager()
	task := m.CreateScheduledTask("每日备份", "0 2 * * *", "backup --all", true)

	enabled := false
	updated, err := m.UpdateScheduledTask(task.ID, UpdateTaskRequest{
		Description: "每周备份",
		Enabled:     &enabled,
	})

	assert.NoError(t, err)
	assert.Equal(t, "每周备份", updated.Description)
	assert.False(t, updated.Enabled)
}

func TestDeleteScheduledTask(t *testing.T) {
	m := setupTestManager()
	task := m.CreateScheduledTask("测试任务", "* * * * *", "echo test", true)

	err := m.DeleteScheduledTask(task.ID)
	assert.NoError(t, err)

	tasks := m.ListScheduledTasks()
	assert.Len(t, tasks, 0)
}

func TestUpdateNonExistentTask(t *testing.T) {
	m := setupTestManager()
	_, err := m.UpdateScheduledTask("non-existent", UpdateTaskRequest{Description: "test"})
	assert.Error(t, err)
}

func TestDeleteNonExistentTask(t *testing.T) {
	m := setupTestManager()
	err := m.DeleteScheduledTask("non-existent")
	assert.Error(t, err)
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	m := setupTestManager()
	m.CreateConversation("user1", "对话1")
	m.CreateConversation("user2", "对话2")

	stats := m.GetStats()
	assert.Equal(t, 2, stats.TotalConversations)
	assert.Equal(t, 0, stats.TotalMessages)
}

func TestGetStatsAfterActivity(t *testing.T) {
	m := setupTestManager()
	conv := m.CreateConversation("user1", "测试")
	m.SendMessage(conv.ID, "测试消息", "user1")

	stats := m.GetStats()
	assert.Equal(t, 1, stats.TotalConversations)
	assert.Greater(t, stats.TotalMessages, 0)
}

// ========== 审计日志测试 ==========

func TestAuditLog(t *testing.T) {
	m := setupTestManager()
	m.AddAuditEntry("user1", "create_user", "user add testuser", "success", "192.168.1.100")

	entries := m.ListAuditEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, "user1", entries[0].UserID)
	assert.Equal(t, "create_user", entries[0].Operation)
}

// ========== 用户偏好测试 ==========

func TestUserPreference(t *testing.T) {
	m := setupTestManager()

	// 默认偏好
	pref := m.GetUserPreference("user1")
	assert.Equal(t, "zh-CN", pref.Language)
	assert.Equal(t, ConfirmDangerous, pref.ConfirmLevel)

	// 更新偏好
	updated := m.UpdateUserPreference("user1", UserPreference{
		Language:     "en",
		ConfirmLevel: ConfirmAlways,
		OutputFormat: "json",
	})
	assert.Equal(t, "en", updated.Language)

	// 获取更新后的偏好
	got := m.GetUserPreference("user1")
	assert.Equal(t, "en", got.Language)
	assert.Equal(t, ConfirmAlways, got.ConfirmLevel)
}

// ========== HTTP API 测试 ==========

func TestHTTPChat(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	body, _ := json.Marshal(ChatRequest{
		Message: "你好",
		UserID:  "user1",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/copilot/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPListConversations(t *testing.T) {
	m := setupTestManager()
	m.CreateConversation("user1", "test")
	r := setupTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/copilot/conversations?userId=user1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPGetStats(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/copilot/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats CopilotStats
	json.Unmarshal(w.Body.Bytes(), &stats)
	assert.Equal(t, 0, stats.TotalConversations)
}

func TestHTTPGetAudit(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/copilot/audit", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPSearchKnowledge(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/copilot/knowledge/search?q=test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPSearchKnowledgeNoQuery(t *testing.T) {
	m := setupTestManager()
	r := setupTestRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/copilot/knowledge/search", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
