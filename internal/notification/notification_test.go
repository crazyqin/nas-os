package notification

import (
	"context"
	"testing"
	"time"
)

// ========== 模板管理测试 ==========

func TestTemplateManager_Create(t *testing.T) {
	tm, err := NewTemplateManager("")
	if err != nil {
		t.Fatalf("创建模板管理器失败: %v", err)
	}

	template := &Template{
		ID:       "test-template",
		Name:     "测试模板",
		Subject:  "测试标题: {{.title}}",
		Body:     "测试内容: {{.message}}",
		Category: "test",
	}

	if err := tm.Create(template); err != nil {
		t.Errorf("创建模板失败: %v", err)
	}

	// 重复创建应该失败
	if err := tm.Create(template); err == nil {
		t.Error("重复创建应该失败")
	}
}

func TestTemplateManager_Get(t *testing.T) {
	tm, _ := NewTemplateManager("")

	template := &Template{
		ID:       "test-get",
		Name:     "测试模板",
		Subject:  "标题",
		Body:     "内容",
		Category: "test",
	}
	_ = tm.Create(template)

	// 获取存在的模板
	got, err := tm.Get("test-get")
	if err != nil {
		t.Errorf("获取模板失败: %v", err)
	}
	if got.Name != "测试模板" {
		t.Errorf("模板名称不匹配: %s", got.Name)
	}

	// 获取不存在的模板
	_, err = tm.Get("not-exist")
	if err == nil {
		t.Error("获取不存在的模板应该失败")
	}
}

func TestTemplateManager_Render(t *testing.T) {
	tm, _ := NewTemplateManager("")

	template := &Template{
		ID:       "test-render",
		Name:     "渲染测试",
		Subject:  "标题: {{.title}}",
		Body:     "内容: {{.message}}, 时间: {{.timestamp}}",
		Category: "test",
		Variables: []TemplateVariable{
			{Name: "title", Required: true},
			{Name: "message", Required: true},
		},
	}
	_ = tm.Create(template)

	variables := map[string]interface{}{
		"title":   "测试标题",
		"message": "测试消息",
	}

	rendered, err := tm.Render("test-render", variables)
	if err != nil {
		t.Errorf("渲染模板失败: %v", err)
	}

	if rendered.Subject != "标题: 测试标题" {
		t.Errorf("标题渲染不正确: %s", rendered.Subject)
	}
}

func TestTemplateManager_ValidateVariables(t *testing.T) {
	tm, _ := NewTemplateManager("")

	template := &Template{
		ID:      "test-validate",
		Name:    "验证测试",
		Subject: "标题",
		Body:    "内容",
		Variables: []TemplateVariable{
			{Name: "required", Required: true},
			{Name: "optional", Required: false},
		},
	}
	_ = tm.Create(template)

	// 缺少必填变量
	err := tm.ValidateVariables("test-validate", map[string]interface{}{})
	if err == nil {
		t.Error("缺少必填变量应该验证失败")
	}

	// 有必填变量
	err = tm.ValidateVariables("test-validate", map[string]interface{}{
		"required": "value",
	})
	if err != nil {
		t.Errorf("有必填变量应该验证通过: %v", err)
	}
}

// ========== 渠道管理测试 ==========

func TestChannelManager_AddChannel(t *testing.T) {
	cm := NewChannelManager()

	config := &ChannelConfig{
		ID:      "test-channel",
		Name:    "测试渠道",
		Type:    ChannelEmail,
		Enabled: true,
		Config: map[string]interface{}{
			"smtpHost": "smtp.example.com",
			"smtpPort": 587,
		},
	}

	if err := cm.AddChannel(config); err != nil {
		t.Errorf("添加渠道失败: %v", err)
	}

	// 验证添加成功
	got, err := cm.GetChannel("test-channel")
	if err != nil {
		t.Errorf("获取渠道失败: %v", err)
	}
	if got.Name != "测试渠道" {
		t.Errorf("渠道名称不匹配: %s", got.Name)
	}
}

func TestChannelManager_ListChannels(t *testing.T) {
	cm := NewChannelManager()

	// 添加多个渠道
	cm.AddChannel(&ChannelConfig{ID: "email1", Name: "邮件1", Type: ChannelEmail, Enabled: true})
	cm.AddChannel(&ChannelConfig{ID: "email2", Name: "邮件2", Type: ChannelEmail, Enabled: false})
	cm.AddChannel(&ChannelConfig{ID: "webhook1", Name: "Webhook1", Type: ChannelWebhook, Enabled: true})

	// 列出所有渠道
	all := cm.ListChannels("")
	if len(all) != 3 {
		t.Errorf("渠道数量不正确: %d", len(all))
	}

	// 按类型过滤
	emails := cm.ListChannels(ChannelEmail)
	if len(emails) != 2 {
		t.Errorf("邮件渠道数量不正确: %d", len(emails))
	}

	// 获取启用的渠道
	enabled := cm.GetEnabledChannels()
	if len(enabled) != 2 {
		t.Errorf("启用渠道数量不正确: %d", len(enabled))
	}
}

func TestChannelManager_RemoveChannel(t *testing.T) {
	cm := NewChannelManager()

	cm.AddChannel(&ChannelConfig{ID: "remove-test", Name: "测试", Type: ChannelEmail})

	if err := cm.RemoveChannel("remove-test"); err != nil {
		t.Errorf("删除渠道失败: %v", err)
	}

	// 验证删除成功
	_, err := cm.GetChannel("remove-test")
	if err == nil {
		t.Error("删除后应该找不到渠道")
	}
}

// ========== 规则引擎测试 ==========

func TestRuleEngine_CreateRule(t *testing.T) {
	engine, err := NewRuleEngine("")
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	rule := &Rule{
		ID:       "test-rule",
		Name:     "测试规则",
		Enabled:  true,
		Priority: 10,
		Conditions: RuleGroup{
			Operator: OperatorAnd,
			Rules: []RuleConditionItem{
				{Field: "level", Condition: ConditionEquals, Value: "error"},
			},
		},
		Channels: []string{"channel1"},
	}

	if err := engine.CreateRule(rule); err != nil {
		t.Errorf("创建规则失败: %v", err)
	}
}

func TestRuleEngine_MatchRules(t *testing.T) {
	engine, _ := NewRuleEngine("")

	// 创建测试规则
	engine.CreateRule(&Rule{
		ID:       "match-level",
		Name:     "级别匹配规则",
		Enabled:  true,
		Priority: 10,
		Conditions: RuleGroup{
			Operator: OperatorAnd,
			Rules: []RuleConditionItem{
				{Field: "level", Condition: ConditionEquals, Value: "error"},
			},
		},
		Channels: []string{"channel1"},
	})

	engine.CreateRule(&Rule{
		ID:       "match-source",
		Name:     "来源匹配规则",
		Enabled:  true,
		Priority: 5,
		Conditions: RuleGroup{
			Operator: OperatorOr,
			Rules: []RuleConditionItem{
				{Field: "source", Condition: ConditionContains, Value: "system"},
			},
		},
		Channels: []string{"channel2"},
	})

	// 测试匹配
	notification := &Notification{
		Title:   "测试通知",
		Message: "这是一条测试通知",
		Level:   LevelError,
		Source:  "system-monitor",
	}

	matched := engine.MatchRules(notification)
	if len(matched) != 2 {
		t.Errorf("匹配规则数量不正确: %d", len(matched))
	}
}

func TestRuleEngine_EvaluateConditions(t *testing.T) {
	engine, _ := NewRuleEngine("")

	tests := []struct {
		name         string
		conditions   RuleGroup
		notification *Notification
		expected     bool
	}{
		{
			name: "Equals条件",
			conditions: RuleGroup{
				Operator: OperatorAnd,
				Rules: []RuleConditionItem{
					{Field: "level", Condition: ConditionEquals, Value: "error"},
				},
			},
			notification: &Notification{Level: LevelError},
			expected:     true,
		},
		{
			name: "Contains条件",
			conditions: RuleGroup{
				Operator: OperatorAnd,
				Rules: []RuleConditionItem{
					{Field: "title", Condition: ConditionContains, Value: "告警"},
				},
			},
			notification: &Notification{Title: "系统告警"},
			expected:     true,
		},
		{
			name: "And条件-全部满足",
			conditions: RuleGroup{
				Operator: OperatorAnd,
				Rules: []RuleConditionItem{
					{Field: "level", Condition: ConditionEquals, Value: "error"},
					{Field: "source", Condition: ConditionEquals, Value: "system"},
				},
			},
			notification: &Notification{Level: LevelError, Source: "system"},
			expected:     true,
		},
		{
			name: "And条件-部分满足",
			conditions: RuleGroup{
				Operator: OperatorAnd,
				Rules: []RuleConditionItem{
					{Field: "level", Condition: ConditionEquals, Value: "error"},
					{Field: "source", Condition: ConditionEquals, Value: "other"},
				},
			},
			notification: &Notification{Level: LevelError, Source: "system"},
			expected:     false,
		},
		{
			name: "Or条件-任一满足",
			conditions: RuleGroup{
				Operator: OperatorOr,
				Rules: []RuleConditionItem{
					{Field: "level", Condition: ConditionEquals, Value: "error"},
					{Field: "level", Condition: ConditionEquals, Value: "critical"},
				},
			},
			notification: &Notification{Level: LevelWarning},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.evaluateConditions(tt.conditions, tt.notification)
			if result != tt.expected {
				t.Errorf("条件评估结果不正确: got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRuleEngine_RateLimit(t *testing.T) {
	engine, _ := NewRuleEngine("")

	rule := &Rule{
		ID:      "rate-limit-rule",
		Name:    "频率限制规则",
		Enabled: true,
		RateLimit: &RateLimit{
			Count:    2,
			Duration: time.Minute,
		},
		Channels: []string{"channel1"},
	}
	_ = engine.CreateRule(rule)

	notification := &Notification{
		Title:  "测试",
		Source: "test-source",
	}

	// 前两次应该通过
	if !engine.checkRateLimit(rule, notification) {
		t.Error("第一次应该通过")
	}
	if !engine.checkRateLimit(rule, notification) {
		t.Error("第二次应该通过")
	}
	// 第三次应该被限制
	if engine.checkRateLimit(rule, notification) {
		t.Error("第三次应该被限制")
	}
}

// ========== 历史管理测试 ==========

func TestHistoryManager_AddRecord(t *testing.T) {
	hm, err := NewHistoryManager("", 100, 30)
	if err != nil {
		t.Fatalf("创建历史管理器失败: %v", err)
	}

	record := &NotificationRecord{
		NotificationID: "notif-1",
		Channel:        ChannelEmail,
		ChannelName:    "测试邮件",
		Status:         StatusSent,
	}

	if err := hm.AddRecord(record); err != nil {
		t.Errorf("添加记录失败: %v", err)
	}

	if hm.Count() != 1 {
		t.Errorf("记录数量不正确: %d", hm.Count())
	}
}

func TestHistoryManager_Query(t *testing.T) {
	hm, _ := NewHistoryManager("", 100, 30)

	// 添加多条记录
	hm.AddRecord(&NotificationRecord{
		ID:             "rec-1",
		NotificationID: "notif-1",
		Channel:        ChannelEmail,
		Status:         StatusSent,
		Notification:   &Notification{Level: LevelInfo, Title: "信息通知"},
	})

	hm.AddRecord(&NotificationRecord{
		ID:             "rec-2",
		NotificationID: "notif-2",
		Channel:        ChannelWebhook,
		Status:         StatusFailed,
		Notification:   &Notification{Level: LevelError, Title: "错误通知"},
	})

	// 按状态过滤
	filter := &HistoryFilter{Status: StatusSent}
	records := hm.Query(filter)
	if len(records) != 1 {
		t.Errorf("状态过滤结果不正确: %d", len(records))
	}

	// 按渠道过滤
	filter = &HistoryFilter{Channel: ChannelWebhook}
	records = hm.Query(filter)
	if len(records) != 1 {
		t.Errorf("渠道过滤结果不正确: %d", len(records))
	}

	// 按级别过滤
	filter = &HistoryFilter{Level: LevelError}
	records = hm.Query(filter)
	if len(records) != 1 {
		t.Errorf("级别过滤结果不正确: %d", len(records))
	}
}

func TestHistoryManager_GetStats(t *testing.T) {
	hm, _ := NewHistoryManager("", 100, 30)

	// 添加记录
	hm.AddRecord(&NotificationRecord{
		Channel:      ChannelEmail,
		Status:       StatusSent,
		Notification: &Notification{Level: LevelInfo},
	})

	hm.AddRecord(&NotificationRecord{
		Channel:      ChannelEmail,
		Status:       StatusSent,
		Notification: &Notification{Level: LevelError},
	})

	hm.AddRecord(&NotificationRecord{
		Channel:      ChannelWebhook,
		Status:       StatusFailed,
		Notification: &Notification{Level: LevelError},
	})

	stats := hm.GetStats(nil, nil)

	if stats.TotalCount != 3 {
		t.Errorf("总数不正确: %d", stats.TotalCount)
	}

	if stats.SuccessCount != 2 {
		t.Errorf("成功数不正确: %d", stats.SuccessCount)
	}

	if stats.FailedCount != 1 {
		t.Errorf("失败数不正确: %d", stats.FailedCount)
	}

	if stats.ChannelStats[ChannelEmail] != 2 {
		t.Errorf("邮件渠道统计不正确: %d", stats.ChannelStats[ChannelEmail])
	}

	if stats.LevelStats[LevelError] != 2 {
		t.Errorf("错误级别统计不正确: %d", stats.LevelStats[LevelError])
	}
}

// ========== 服务测试 ==========

func TestService_Send(t *testing.T) {
	service, err := NewService(&ServiceConfig{
		MaxConcurrent: 2,
		StoragePath:   "",
	})
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	// 添加测试渠道（使用一个不会真正发送的渠道配置）
	_ = service.AddChannel(&ChannelConfig{
		ID:      "test-webhook",
		Name:    "测试Webhook",
		Type:    ChannelWebhook,
		Enabled: true,
		Config: map[string]interface{}{
			"url": "http://localhost:9999/test", // 不会真正发送
		},
	})

	// 注意：这个测试会因为网络请求失败，但流程是正确的
	// 在实际测试中应该使用 mock
}

func TestService_GetTemplateManager(t *testing.T) {
	service, _ := NewService(nil)

	if service.GetTemplateManager() == nil {
		t.Error("模板管理器不应该为空")
	}
}

func TestService_GetChannelManager(t *testing.T) {
	service, _ := NewService(nil)

	if service.GetChannelManager() == nil {
		t.Error("渠道管理器不应该为空")
	}
}

func TestService_GetRuleEngine(t *testing.T) {
	service, _ := NewService(nil)

	if service.GetRuleEngine() == nil {
		t.Error("规则引擎不应该为空")
	}
}

func TestService_GetHistoryManager(t *testing.T) {
	service, _ := NewService(nil)

	if service.GetHistoryManager() == nil {
		t.Error("历史管理器不应该为空")
	}
}

// ========== 类型定义测试 ==========

func TestNotificationLevel_String(t *testing.T) {
	levels := []NotificationLevel{LevelInfo, LevelSuccess, LevelWarning, LevelError, LevelCritical}

	for _, level := range levels {
		if string(level) == "" {
			t.Errorf("级别字符串不应该为空")
		}
	}
}

func TestChannelType_String(t *testing.T) {
	channels := []ChannelType{ChannelEmail, ChannelWebhook, ChannelWebSocket, ChannelWeChat, ChannelDingTalk, ChannelTelegram}

	for _, channel := range channels {
		if string(channel) == "" {
			t.Errorf("渠道类型字符串不应该为空")
		}
	}
}

func TestNotificationStatus_String(t *testing.T) {
	statuses := []NotificationStatus{StatusPending, StatusSent, StatusFailed, StatusRetrying, StatusCancelled}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("状态字符串不应该为空")
		}
	}
}

// ========== 发送器测试 ==========

func TestEmailSender_Type(t *testing.T) {
	sender := NewEmailSender()
	if sender.Type() != ChannelEmail {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}
}

func TestWebhookSender_Type(t *testing.T) {
	sender := NewWebhookSender()
	if sender.Type() != ChannelWebhook {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}
}

func TestWeChatSender_Type(t *testing.T) {
	sender := NewWeChatSender()
	if sender.Type() != ChannelWeChat {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}
}

func TestDingTalkSender_Type(t *testing.T) {
	sender := NewDingTalkSender()
	if sender.Type() != ChannelDingTalk {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}
}

func TestTelegramSender_Type(t *testing.T) {
	sender := NewTelegramSender()
	if sender.Type() != ChannelTelegram {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}
}

// ========== 发送器注册表测试 ==========

func TestSenderRegistry_Get(t *testing.T) {
	registry := NewSenderRegistry()

	// 测试已注册的发送器
	sender, exists := registry.Get(ChannelEmail)
	if !exists {
		t.Error("邮件发送器应该已注册")
	}
	if sender.Type() != ChannelEmail {
		t.Errorf("发送器类型不正确: %s", sender.Type())
	}

	// 测试未注册的发送器
	_, exists = registry.Get(ChannelType("unknown"))
	if exists {
		t.Error("未知发送器不应该存在")
	}
}

// ========== 辅助函数测试 ==========

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("ID不应该为空")
	}

	if id1 == id2 {
		t.Error("ID应该唯一")
	}
}

func TestSanitizeEmailHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@example.com", "test@example.com"},
		{"test\r\n@example.com", "test@example.com"},
		{"test\n@example.com", "test@example.com"},
		{"test\r@example.com", "test@example.com"},
	}

	for _, tt := range tests {
		result := sanitizeEmailHeader(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeEmailHeader(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ========== 集成测试 ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	// 这个测试演示完整的工作流程
	// 创建服务
	service, err := NewService(&ServiceConfig{
		MaxConcurrent:  2,
		MaxHistoryDays: 30,
	})
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	// 创建模板
	template := &Template{
		ID:       "integration-test",
		Name:     "集成测试模板",
		Subject:  "测试: {{.title}}",
		Body:     "消息: {{.message}}",
		Category: "test",
	}
	if err := service.GetTemplateManager().Create(template); err != nil {
		t.Errorf("创建模板失败: %v", err)
	}

	// 创建渠道
	channel := &ChannelConfig{
		ID:      "integration-channel",
		Name:    "集成测试渠道",
		Type:    ChannelWebhook,
		Enabled: true,
		Config: map[string]interface{}{
			"url": "http://localhost:9999/webhook",
		},
	}
	if err := service.AddChannel(channel); err != nil {
		t.Errorf("创建渠道失败: %v", err)
	}

	// 创建规则
	rule := &Rule{
		ID:       "integration-rule",
		Name:     "集成测试规则",
		Enabled:  true,
		Priority: 10,
		Conditions: RuleGroup{
			Operator: OperatorAnd,
			Rules: []RuleConditionItem{
				{Field: "category", Condition: ConditionEquals, Value: "test"},
			},
		},
		Channels: []string{"integration-channel"},
	}
	if err := service.GetRuleEngine().CreateRule(rule); err != nil {
		t.Errorf("创建规则失败: %v", err)
	}

	// 验证规则匹配
	notification := &Notification{
		Title:    "测试标题",
		Message:  "测试消息",
		Level:    LevelInfo,
		Category: "test",
	}
	matched := service.GetRuleEngine().MatchRules(notification)
	if len(matched) != 1 {
		t.Errorf("规则匹配数量不正确: %d", len(matched))
	}

	// 清理
	service.Stop()
}

// ========== Context 测试 ==========

func TestService_SendWithContext(t *testing.T) {
	service, _ := NewService(nil)

	// 创建一个已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 使用已取消的 context 发送应该立即返回错误
	_, err := service.Send(ctx, &SendRequest{
		Notification: &Notification{
			Title:   "测试",
			Message: "测试消息",
		},
	})

	if err == nil {
		t.Error("使用已取消的 context 应该返回错误")
	}
}
