package aichatbot

import (
	"testing"
	"time"
)

func TestNewAIChatbot(t *testing.T) {
	bot := NewAIChatbot()
	if bot == nil {
		t.Fatal("NewAIChatbot returned nil")
	}
	if bot.sessions == nil {
		t.Error("sessions map not initialized")
	}
	if bot.plugins == nil {
		t.Error("plugins map not initialized")
	}
	if bot.langTemplates == nil {
		t.Error("langTemplates map not initialized")
	}
}

func TestCreateSession(t *testing.T) {
	bot := NewAIChatbot()

	session, err := bot.CreateSession("session1", "user1", LangChinese)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID != "session1" {
		t.Errorf("expected session1, got %s", session.ID)
	}
	if session.UserID != "user1" {
		t.Errorf("expected user1, got %s", session.UserID)
	}
	if session.Language != LangChinese {
		t.Errorf("expected zh, got %s", session.Language)
	}
	if !session.IsActive {
		t.Error("session should be active")
	}

	// 测试重复创建
	_, err = bot.CreateSession("session1", "user1", LangChinese)
	if err == nil {
		t.Error("expected error for duplicate session")
	}
}

func TestGetSession(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	session, err := bot.GetSession("session1")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session.ID != "session1" {
		t.Errorf("expected session1, got %s", session.ID)
	}

	// 测试不存在的会话
	_, err = bot.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestCloseSession(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	err := bot.CloseSession("session1")
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	session, _ := bot.GetSession("session1")
	if session.IsActive {
		t.Error("session should be inactive")
	}

	// 测试不存在的会话
	err = bot.CloseSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)
	bot.CreateSession("session2", "user1", LangEnglish)
	bot.CreateSession("session3", "user2", LangChinese)
	bot.CloseSession("session2")

	// 列出 user1 的所有会话
	sessions := bot.ListSessions("user1", false)
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for user1, got %d", len(sessions))
	}

	// 列出 user1 的活跃会话
	sessions = bot.ListSessions("user1", true)
	if len(sessions) != 1 {
		t.Errorf("expected 1 active session for user1, got %d", len(sessions))
	}
}

func TestSendMessage(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	msg, err := bot.SendMessage("session1", "你好", MessageTypeUser)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if msg.Content != "你好" {
		t.Errorf("expected '你好', got %s", msg.Content)
	}
	if msg.Type != MessageTypeUser {
		t.Errorf("expected user type, got %s", msg.Type)
	}

	// 测试不存在的会话
	_, err = bot.SendMessage("nonexistent", "test", MessageTypeUser)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestGetMessages(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	bot.SendMessage("session1", "消息1", MessageTypeUser)
	bot.SendMessage("session1", "消息2", MessageTypeAssistant)
	bot.SendMessage("session1", "消息3", MessageTypeUser)

	// 获取所有消息
	msgs, err := bot.GetMessages("session1", 0)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}

	// 获取最近2条消息
	msgs, _ = bot.GetMessages("session1", 2)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "消息2" {
		t.Errorf("expected '消息2', got %s", msgs[0].Content)
	}
}

func TestRecognizeIntent(t *testing.T) {
	bot := NewAIChatbot()

	tests := []struct {
		input    string
		expected IntentType
	}{
		{"帮我创建一个共享文件夹", IntentShareFolderCreate},
		{"创建共享文件夹 test", IntentShareFolderCreate},
		{"删除共享文件夹", IntentShareFolderDelete},
		{"系统状态如何", IntentSystemStatus},
		{"查看磁盘使用情况", IntentDiskUsage},
		{"重启服务", IntentServiceRestart},
		{"创建定时任务", IntentScheduleTask},
		{"批量操作文件", IntentBatchOperation},
		{"随便聊聊", IntentUnknown},
	}

	for _, tt := range tests {
		intent := bot.RecognizeIntent(tt.input)
		if intent.Type != tt.expected {
			t.Errorf("input '%s': expected %s, got %s", tt.input, tt.expected, intent.Type)
		}
	}
}

func TestProcessMessage(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	// 安装一个测试插件
	installed := false
	bot.InstallPlugin(&Plugin{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Intents: []IntentType{IntentSystemStatus},
		Handler: func(ctx *PluginContext) (*PluginResponse, error) {
			installed = true
			return &PluginResponse{
				Content: "系统运行正常",
			}, nil
		},
	})

	// 处理消息
	msg, err := bot.ProcessMessage("session1", "系统状态如何")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if !installed {
		t.Error("plugin should have been called")
	}
	if msg.Content != "系统运行正常" {
		t.Errorf("expected '系统运行正常', got %s", msg.Content)
	}

	// 验证消息历史: user message + assistant response = 2
	msgs, _ := bot.GetMessages("session1", 0)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestPluginManagement(t *testing.T) {
	bot := NewAIChatbot()

	plugin := &Plugin{
		ID:          "plugin1",
		Name:        "Test Plugin",
		Description: "测试插件",
		Intents:     []IntentType{IntentSystemStatus},
		Handler: func(ctx *PluginContext) (*PluginResponse, error) {
			return &PluginResponse{Content: "ok"}, nil
		},
	}

	// 安装插件
	err := bot.InstallPlugin(plugin)
	if err != nil {
		t.Fatalf("InstallPlugin failed: %v", err)
	}

	// 测试重复安装
	err = bot.InstallPlugin(plugin)
	if err == nil {
		t.Error("expected error for duplicate plugin")
	}

	// 列出插件
	plugins := bot.ListPlugins(false)
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}

	// 禁用插件
	err = bot.DisablePlugin("plugin1")
	if err != nil {
		t.Fatalf("DisablePlugin failed: %v", err)
	}

	// 列出活跃插件
	plugins = bot.ListPlugins(true)
	if len(plugins) != 0 {
		t.Errorf("expected 0 active plugins, got %d", len(plugins))
	}

	// 启用插件
	err = bot.EnablePlugin("plugin1")
	if err != nil {
		t.Fatalf("EnablePlugin failed: %v", err)
	}

	// 卸载插件
	err = bot.UninstallPlugin("plugin1")
	if err != nil {
		t.Fatalf("UninstallPlugin failed: %v", err)
	}

	plugins = bot.ListPlugins(false)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins after uninstall, got %d", len(plugins))
	}

	// 测试操作不存在的插件
	err = bot.EnablePlugin("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestScheduledTasks(t *testing.T) {
	bot := NewAIChatbot()

	task := &ScheduledTask{
		ID:          "task1",
		UserID:      "user1",
		Name:        "每日备份",
		Description: "每天凌晨2点备份数据",
		Schedule:    "0 2 * * *",
		Action: &Action{
			Type:       "backup",
			Parameters: map[string]interface{}{"path": "/data"},
		},
	}

	// 创建定时任务
	err := bot.CreateScheduledTask(task)
	if err != nil {
		t.Fatalf("CreateScheduledTask failed: %v", err)
	}

	// 测试重复创建
	err = bot.CreateScheduledTask(task)
	if err == nil {
		t.Error("expected error for duplicate task")
	}

	// 获取定时任务
	fetched, err := bot.GetScheduledTask("task1")
	if err != nil {
		t.Fatalf("GetScheduledTask failed: %v", err)
	}
	if fetched.Name != "每日备份" {
		t.Errorf("expected '每日备份', got %s", fetched.Name)
	}

	// 列出定时任务
	tasks := bot.ListScheduledTasks("user1", false)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	// 删除定时任务
	err = bot.DeleteScheduledTask("task1")
	if err != nil {
		t.Fatalf("DeleteScheduledTask failed: %v", err)
	}

	_, err = bot.GetScheduledTask("task1")
	if err == nil {
		t.Error("expected error for deleted task")
	}
}

func TestSetSessionLanguage(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	err := bot.SetSessionLanguage("session1", LangEnglish)
	if err != nil {
		t.Fatalf("SetSessionLanguage failed: %v", err)
	}

	session, _ := bot.GetSession("session1")
	if session.Language != LangEnglish {
		t.Errorf("expected en, got %s", session.Language)
	}

	// 测试不存在的会话
	err = bot.SetSessionLanguage("nonexistent", LangChinese)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestMultiLanguage(t *testing.T) {
	bot := NewAIChatbot()

	// 测试中文
	bot.CreateSession("zh_session", "user1", LangChinese)
	_, err := bot.SendMessage("zh_session", "你好", MessageTypeUser)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 测试英文
	bot.CreateSession("en_session", "user1", LangEnglish)
	_, err = bot.SendMessage("en_session", "hello", MessageTypeUser)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 测试日文
	bot.CreateSession("ja_session", "user1", LangJapanese)
	_, err = bot.SendMessage("ja_session", "こんにちは", MessageTypeUser)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)
	bot.CreateSession("session2", "user1", LangEnglish)
	bot.CreateSession("session3", "user2", LangChinese)
	bot.CloseSession("session2")

	bot.SendMessage("session1", "msg1", MessageTypeUser)
	bot.SendMessage("session1", "msg2", MessageTypeAssistant)

	bot.CreateScheduledTask(&ScheduledTask{
		ID:     "task1",
		UserID: "user1",
		Name:   "Task 1",
		Action: &Action{Type: "test"},
	})

	bot.InstallPlugin(&Plugin{
		ID:      "plugin1",
		Name:    "Plugin 1",
		Intents: []IntentType{IntentSystemStatus},
		Handler: func(ctx *PluginContext) (*PluginResponse, error) {
			return &PluginResponse{Content: "ok"}, nil
		},
	})

	stats := bot.GetStats()

	if stats.TotalSessions != 3 {
		t.Errorf("expected 3 total sessions, got %d", stats.TotalSessions)
	}
	if stats.TotalMessages != 2 {
		t.Errorf("expected 2 total messages, got %d", stats.TotalMessages)
	}
	if stats.ActiveSessions != 2 {
		t.Errorf("expected 2 active sessions, got %d", stats.ActiveSessions)
	}
	if stats.SessionsByUser["user1"] != 2 {
		t.Errorf("expected 2 sessions for user1, got %d", stats.SessionsByUser["user1"])
	}
	if stats.ScheduledTasks != 1 {
		t.Errorf("expected 1 scheduled task, got %d", stats.ScheduledTasks)
	}
	if stats.InstalledPlugins != 1 {
		t.Errorf("expected 1 installed plugin, got %d", stats.InstalledPlugins)
	}
}

func TestPluginExecution(t *testing.T) {
	bot := NewAIChatbot()

	executed := false
	bot.InstallPlugin(&Plugin{
		ID:      "disk-plugin",
		Name:    "Disk Plugin",
		Intents: []IntentType{IntentDiskUsage},
		Handler: func(ctx *PluginContext) (*PluginResponse, error) {
			executed = true
			return &PluginResponse{
				Content: "磁盘使用率 50%",
				Actions: []*Action{
					{
						Type:   "query",
						Status: "success",
					},
				},
			}, nil
		},
	})

	bot.CreateSession("session1", "user1", LangChinese)
	bot.ProcessMessage("session1", "查看磁盘使用情况")

	if !executed {
		t.Error("disk plugin should have been executed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	done := make(chan bool, 10)

	// 并发发送消息
	for i := 0; i < 10; i++ {
		go func(i int) {
			bot.SendMessage("session1", "msg", MessageTypeUser)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	msgs, _ := bot.GetMessages("session1", 0)
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages, got %d", len(msgs))
	}
}

func TestSessionContext(t *testing.T) {
	bot := NewAIChatbot()

	session, _ := bot.CreateSession("session1", "user1", LangChinese)

	// 设置上下文
	bot.mu.Lock()
	session.Context["last_topic"] = "磁盘管理"
	session.Context["user_preference"] = "中文"
	bot.mu.Unlock()

	// 获取会话并检查上下文
	session, _ = bot.GetSession("session1")
	if session.Context["last_topic"] != "磁盘管理" {
		t.Errorf("expected '磁盘管理', got %v", session.Context["last_topic"])
	}
}

func TestMessageTimestamps(t *testing.T) {
	bot := NewAIChatbot()

	bot.CreateSession("session1", "user1", LangChinese)

	before := time.Now()
	msg, _ := bot.SendMessage("session1", "test", MessageTypeUser)
	after := time.Now()

	if msg.CreatedAt.Before(before) || msg.CreatedAt.After(after) {
		t.Error("message timestamp out of range")
	}
}
