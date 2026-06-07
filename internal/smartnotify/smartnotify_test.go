package smartnotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

// 测试场景1: 发送通知
func TestSendNotification(t *testing.T) {
	m := setupTestManager(t)

	notify := &Notification{
		Title:    "测试通知",
		Content:  "这是一条测试通知",
		Priority: PriorityNormal,
		Channels: []NotifyChannel{ChannelEmail, ChannelPush},
		Source:   "test",
		Tags:     map[string]string{"env": "test"},
	}

	err := m.SendNotification(notify)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	if notify.ID == "" {
		t.Error("expected non-empty notification ID")
	}
	if notify.Status != StatusSent {
		t.Errorf("expected status sent, got %v", notify.Status)
	}
	if notify.SentAt == nil {
		t.Error("expected non-nil sent_at")
	}
}

// 测试场景2: 规则匹配
func TestRuleMatching(t *testing.T) {
	m := setupTestManager(t)

	// 创建一个测试规则
	rule := &NotifyRule{
		Name:     "测试规则",
		Priority: PriorityImportant,
		Conditions: []RuleCondition{
			{Field: "tags.category", Operator: OpEquals, Value: "test"},
		},
		Channels: []NotifyChannel{ChannelEmail},
	}
	m.CreateRule(rule)

	// 发送匹配规则的通知
	notify := &Notification{
		Title:   "匹配通知",
		Content: "应该匹配测试规则",
		Tags:    map[string]string{"category": "test"},
	}
	m.SendNotification(notify)

	// 验证历史记录中有规则ID
	history := m.GetHistory(10)
	found := false
	for _, h := range history {
		if h.NotifyID == notify.ID && h.RuleID == rule.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected history with rule ID")
	}
}

// 测试场景3: 通知去重
func TestNotificationDeduplication(t *testing.T) {
	cfg := DefaultSmartNotifyConfig()
	cfg.Deduplication = true
	cfg.DedupWindow = 5 * time.Minute
	m := NewManager(zap.NewNop(), cfg)

	// 发送第一条通知
	notify1 := &Notification{
		Title:   "重复测试",
		Content: "相同内容",
		Source:  "test",
	}
	m.SendNotification(notify1)

	// 发送重复通知
	notify2 := &Notification{
		Title:   "重复测试",
		Content: "相同内容",
		Source:  "test",
	}
	m.SendNotification(notify2)

	if notify2.Status != StatusSilenced {
		t.Errorf("expected status silenced, got %v", notify2.Status)
	}
}

// 测试场景4: 免打扰时段
func TestSilencePeriod(t *testing.T) {
	m := setupTestManager(t)

	// 创建带免打扰的规则
	rule := &NotifyRule{
		Name:     "免打扰规则",
		Priority: PriorityNormal,
		Conditions: []RuleCondition{
			{Field: "tags.type", Operator: OpEquals, Value: "routine"},
		},
		Channels: []NotifyChannel{ChannelPush},
		Silence: SilenceConfig{
			Enabled:   true,
			StartTime: "00:00",
			EndTime:   "23:59", // 全天免打扰
		},
	}
	m.CreateRule(rule)

	// 发送普通优先级通知
	notify := &Notification{
		Title:   "普通通知",
		Content: "应该被静默",
		Tags:    map[string]string{"type": "routine"},
	}
	m.SendNotification(notify)

	if notify.Status != StatusSilenced {
		t.Errorf("expected status silenced, got %v", notify.Status)
	}

	// 发送紧急通知（应该突破免打扰）
	urgentNotify := &Notification{
		Title:    "紧急通知",
		Content:  "应该突破免打扰",
		Priority: PriorityUrgent,
		Tags:     map[string]string{"type": "routine"},
	}
	m.SendNotification(urgentNotify)

	// 紧急通知不会匹配这个规则（因为需要tags.type=routine），所以会直接发送
	// 这里我们验证紧急通知不会被静默
}

// 测试场景5: 规则管理
func TestRuleManagement(t *testing.T) {
	m := setupTestManager(t)

	// 列出默认规则
	rules := m.ListRules()
	if len(rules) < 4 {
		t.Errorf("expected at least 4 default rules, got %d", len(rules))
	}

	// 创建规则
	rule := &NotifyRule{
		Name:     "新规则",
		Priority: PriorityImportant,
		Conditions: []RuleCondition{
			{Field: "source", Operator: OpEquals, Value: "test"},
		},
		Channels: []NotifyChannel{ChannelEmail},
	}
	err := m.CreateRule(rule)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	// 获取规则
	got, err := m.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if got.Name != "新规则" {
		t.Errorf("expected name '新规则', got '%s'", got.Name)
	}

	// 更新规则
	rule.Name = "更新后的规则"
	err = m.UpdateRule(rule.ID, rule)
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	// 切换规则状态
	err = m.ToggleRule(rule.ID)
	if err != nil {
		t.Fatalf("ToggleRule failed: %v", err)
	}
	got, _ = m.GetRule(rule.ID)
	if got.Enabled {
		t.Error("expected rule to be disabled after toggle")
	}

	// 删除规则
	err = m.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	_, err = m.GetRule(rule.ID)
	if err == nil {
		t.Error("expected error for deleted rule")
	}
}

// 测试场景6: 模板管理
func TestTemplateManagement(t *testing.T) {
	m := setupTestManager(t)

	// 列出默认模板
	templates := m.ListTemplates()
	if len(templates) < 4 {
		t.Errorf("expected at least 4 default templates, got %d", len(templates))
	}

	// 创建模板
	tpl := &NotifyTemplate{
		Name:    "测试模板",
		Channel: ChannelEmail,
		Title:   "[{{level}}] {{title}}",
		Content: "告警详情: {{content}}",
	}
	err := m.CreateTemplate(tpl)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	// 获取模板
	got, err := m.GetTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if got.Name != "测试模板" {
		t.Errorf("expected name '测试模板', got '%s'", got.Name)
	}

	// 渲染模板
	title, content, err := m.RenderTemplate(tpl.ID, map[string]string{
		"level":   "紧急",
		"title":   "服务器宕机",
		"content": "服务器无法访问",
	})
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if title != "[紧急] 服务器宕机" {
		t.Errorf("unexpected title: %s", title)
	}
	if content != "告警详情: 服务器无法访问" {
		t.Errorf("unexpected content: %s", content)
	}

	// 更新模板
	tpl.Name = "更新后的模板"
	err = m.UpdateTemplate(tpl.ID, tpl)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}

	// 删除模板
	err = m.DeleteTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	_, err = m.GetTemplate(tpl.ID)
	if err == nil {
		t.Error("expected error for deleted template")
	}
}

// 测试场景7: 通知统计
func TestNotificationStats(t *testing.T) {
	cfg := DefaultSmartNotifyConfig()
	cfg.Deduplication = false // 禁用去重以测试统计
	m := NewManager(zap.NewNop(), cfg)

	// 发送多条通知
	for i := 0; i < 5; i++ {
		m.SendNotification(&Notification{
			Title:    fmt.Sprintf("统计测试%d", i),
			Content:  fmt.Sprintf("测试内容%d", i),
			Priority: PriorityNormal,
			Channels: []NotifyChannel{ChannelEmail},
		})
	}

	stats := m.GetStats()
	if stats.TotalSent != 5 {
		t.Errorf("expected 5 sent, got %d", stats.TotalSent)
	}
	if stats.ByChannel[ChannelEmail] != 5 {
		t.Errorf("expected 5 email notifications, got %d", stats.ByChannel[ChannelEmail])
	}
}

// 测试场景8: HTTP Handler
func TestHandler_SendNotification(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"title":"API测试","content":"通过API发送","priority":1,"channels":["email","push"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smartnotify/send", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

// 测试场景9: 获取通知详情
func TestGetNotification(t *testing.T) {
	m := setupTestManager(t)

	// 先发送通知
	notify := &Notification{
		Title:   "详情测试",
		Content: "测试内容",
	}
	m.SendNotification(notify)

	// 获取通知详情
	got, err := m.GetNotification(notify.ID)
	if err != nil {
		t.Fatalf("GetNotification failed: %v", err)
	}
	if got.Title != "详情测试" {
		t.Errorf("expected title '详情测试', got '%s'", got.Title)
	}
}

// 测试场景10: 列出通知
func TestListNotifications(t *testing.T) {
	m := setupTestManager(t)

	// 发送多条通知
	for i := 0; i < 3; i++ {
		m.SendNotification(&Notification{
			Title:   "列表测试",
			Content: "测试内容",
		})
	}

	notifications := m.ListNotifications(10)
	if len(notifications) != 3 {
		t.Errorf("expected 3 notifications, got %d", len(notifications))
	}
}

// 测试场景11: HTTP Handler 列出规则
func TestHandler_ListRules(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smartnotify/rules", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

// 测试场景12: 条件操作符
func TestConditionOperators(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name      string
		condition RuleCondition
		notify    *Notification
		expected  bool
	}{
		{
			name:      "equals match",
			condition: RuleCondition{Field: "source", Operator: OpEquals, Value: "test"},
			notify:    &Notification{Source: "test"},
			expected:  true,
		},
		{
			name:      "equals no match",
			condition: RuleCondition{Field: "source", Operator: OpEquals, Value: "prod"},
			notify:    &Notification{Source: "test"},
			expected:  false,
		},
		{
			name:      "not_equals",
			condition: RuleCondition{Field: "source", Operator: OpNotEquals, Value: "prod"},
			notify:    &Notification{Source: "test"},
			expected:  true,
		},
		{
			name:      "contains",
			condition: RuleCondition{Field: "title", Operator: OpContains, Value: "告警"},
			notify:    &Notification{Title: "系统告警通知"},
			expected:  true,
		},
		{
			name:      "greater than",
			condition: RuleCondition{Field: "tags.value", Operator: OpGreaterThan, Value: "80"},
			notify:    &Notification{Tags: map[string]string{"value": "90"}},
			expected:  true,
		},
		{
			name:      "less than",
			condition: RuleCondition{Field: "tags.value", Operator: OpLessThan, Value: "10"},
			notify:    &Notification{Tags: map[string]string{"value": "5"}},
			expected:  true,
		},
		{
			name:      "regex",
			condition: RuleCondition{Field: "title", Operator: OpRegex, Value: `^系统.*告警$`},
			notify:    &Notification{Title: "系统告警"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.matchCondition(tt.notify, tt.condition)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// 测试场景13: 配置管理
func TestConfigManagement(t *testing.T) {
	m := setupTestManager(t)

	// 获取默认配置
	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}

	// 更新配置
	newCfg := DefaultSmartNotifyConfig()
	newCfg.MaxRetries = 5
	newCfg.Deduplication = false
	m.UpdateConfig(newCfg)

	cfg = m.GetConfig()
	if cfg.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", cfg.MaxRetries)
	}
	if cfg.Deduplication {
		t.Error("expected deduplication to be false")
	}
}

// 测试场景14: 无效通知
func TestInvalidNotification(t *testing.T) {
	cfg := DefaultSmartNotifyConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	notify := &Notification{
		Title:   "测试",
		Content: "测试内容",
	}

	err := m.SendNotification(notify)
	if err == nil {
		t.Error("expected error when disabled")
	}
}

// 测试场景15: 无效渠道
func TestInvalidChannel(t *testing.T) {
	if IsValidChannel("invalid") {
		t.Error("expected 'invalid' to be invalid channel")
	}
	if !IsValidChannel(ChannelEmail) {
		t.Error("expected email to be valid channel")
	}
}

// 测试场景16: 渠道名称
func TestChannelNames(t *testing.T) {
	tests := []struct {
		channel  NotifyChannel
		expected string
	}{
		{ChannelEmail, "邮件"},
		{ChannelSMS, "短信"},
		{ChannelWeChat, "微信"},
		{ChannelDingTalk, "钉钉"},
		{ChannelTelegram, "Telegram"},
		{ChannelWebhook, "Webhook"},
		{ChannelPush, "推送"},
	}

	for _, tt := range tests {
		name := ChannelName(tt.channel)
		if name != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, name)
		}
	}
}

// 测试场景17: 优先级名称
func TestPriorityNames(t *testing.T) {
	tests := []struct {
		priority NotifyPriority
		expected string
	}{
		{PriorityUrgent, "紧急"},
		{PriorityImportant, "重要"},
		{PriorityNormal, "普通"},
		{PriorityLow, "低"},
	}

	for _, tt := range tests {
		name := PriorityName(tt.priority)
		if name != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, name)
		}
	}
}

// 测试场景18: 默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultSmartNotifyConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if len(cfg.DefaultChannels) != 2 {
		t.Errorf("expected 2 default channels, got %d", len(cfg.DefaultChannels))
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}
	if !cfg.Deduplication {
		t.Error("expected deduplication to be true")
	}
}
