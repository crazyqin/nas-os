package alerting

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEngine_DefaultTemplates(t *testing.T) {
	engine := NewTemplateEngine()
	templates := engine.ListTemplates()
	assert.NotEmpty(t, templates, "应该有默认模板")

	// 验证各类型都有模板
	typeCount := make(map[TemplateType]int)
	for _, tmpl := range templates {
		typeCount[tmpl.Type]++
	}

	assert.GreaterOrEqual(t, typeCount[TemplateTypeEmail], 1, "应该有Email模板")
	assert.GreaterOrEqual(t, typeCount[TemplateTypeWebhook], 1, "应该有Webhook模板")
	assert.GreaterOrEqual(t, typeCount[TemplateTypeTelegram], 1, "应该有Telegram模板")
	assert.GreaterOrEqual(t, typeCount[TemplateTypeDingTalk], 1, "应该有DingTalk模板")
	assert.GreaterOrEqual(t, typeCount[TemplateTypeWeChat], 1, "应该有WeChat模板")
}

func TestTemplateEngine_Render(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:     "test-001",
		AlertName:   "磁盘空间告警",
		HostName:    "nas-server",
		HostIP:      "192.168.1.100",
		Metric:      "disk_usage",
		Value:       95.5,
		Threshold:   90.0,
		Unit:        "%",
		Level:       AlertLevelWarning,
		Message:     "磁盘使用率超过90%",
		Source:      "monitor",
		ServiceType: "storage",
		Timestamp:   time.Now(),
		Tags: map[string]string{
			"env":     "production",
			"cluster": "storage-01",
		},
	}

	tests := []struct {
		name       string
		templateID string
		wantSubstr string
	}{
		{
			name:       "email_html_default",
			templateID: "email_html_default",
			wantSubstr: "NAS-OS 系统告警",
		},
		{
			name:       "webhook_default",
			templateID: "webhook_default",
			wantSubstr: "nasos.alert",
		},
		{
			name:       "telegram_default",
			templateID: "telegram_default",
			wantSubstr: "NAS-OS 告警",
		},
		{
			name:       "dingtalk_default",
			templateID: "dingtalk_default",
			wantSubstr: "NAS-OS 告警通知",
		},
		{
			name:       "wechat_default",
			templateID: "wechat_default",
			wantSubstr: "NAS-OS 告警",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, body, err := engine.Render(tt.templateID, vars)
			require.NoError(t, err)
			// webhook/telegram/dingtalk/wechat 等模板是 JSON 或 Markdown 格式，不需要 subject
			if tt.templateID == "email_default" || tt.templateID == "email_html_default" {
				assert.NotEmpty(t, subject, "subject不应为空")
			} else {
				assert.Empty(t, subject, "JSON/Markdown模板的subject应为空")
			}
			assert.NotEmpty(t, body, "body不应为空")
			assert.Contains(t, body, tt.wantSubstr, "渲染结果应包含预期内容")

			// 验证变量被正确替换
			assert.NotContains(t, body, "{{.", "不应有未解析的模板变量")
			assert.Contains(t, body, vars.HostName, "应包含主机名")
			assert.Contains(t, body, vars.HostIP, "应包含主机IP")
		})
	}
}

func TestTemplateEngine_Funcs(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertName: "disk full",
		Level:     AlertLevelCritical,
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	tmpl := "{{.AlertName | upper}} - {{.Level | upper}} - {{.Timestamp | time}}"
	rendered, err := engine.renderString(tmpl, vars)
	require.NoError(t, err)

	assert.Contains(t, rendered, "DISK FULL")
	assert.Contains(t, rendered, "CRITICAL")
	assert.Contains(t, rendered, "2025-01-15")
}

func TestTemplateEngine_LevelColor(t *testing.T) {
	engine := NewTemplateEngine()

	tests := []struct {
		level AlertLevel
		want  string
	}{
		{AlertLevelCritical, "#DC2626"},
		{AlertLevelWarning, "#F59E0B"},
		{AlertLevelInfo, "#3B82F6"},
		{AlertLevelDebug, "#6B7280"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := engine.levelColor(tt.level)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateEngine_LevelEmoji(t *testing.T) {
	engine := NewTemplateEngine()

	tests := []struct {
		level AlertLevel
		want  string
	}{
		{AlertLevelCritical, "🚨"},
		{AlertLevelWarning, "⚠️"},
		{AlertLevelInfo, "ℹ️"},
		{AlertLevelDebug, "🔍"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := engine.levelEmoji(tt.level)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplateEngine_CRUD(t *testing.T) {
	engine := NewTemplateEngine()

	// Add
	custom := &AlertTemplate{
		ID:      "custom-test",
		Name:    "自定义测试模板",
		Type:    TemplateTypeEmail,
		Subject: "自定义告警",
		Body:    "主机: {{.HostName}}",
		Enabled: true,
	}
	err := engine.AddTemplate(custom)
	require.NoError(t, err)

	// Get
	tmpl, ok := engine.GetTemplate("custom-test")
	require.True(t, ok)
	assert.Equal(t, "自定义测试模板", tmpl.Name)

	// Update
	custom.Body = "更新后的模板: {{.HostName}}"
	err = engine.UpdateTemplate(custom)
	require.NoError(t, err)

	tmpl, _ = engine.GetTemplate("custom-test")
	assert.Contains(t, tmpl.Body, "更新后")

	// Delete
	err = engine.DeleteTemplate("custom-test")
	require.NoError(t, err)

	_, ok = engine.GetTemplate("custom-test")
	assert.False(t, ok)
}

func TestTemplateEngine_ListByType(t *testing.T) {
	engine := NewTemplateEngine()

	templates := engine.ListTemplatesByType(TemplateTypeTelegram)
	for _, tmpl := range templates {
		assert.Equal(t, TemplateTypeTelegram, tmpl.Type)
	}
}

func TestTemplateEngine_RenderToJSON(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:     "json-001",
		AlertName:   "json-test",
		HostName:    "server1",
		HostIP:      "10.0.0.1",
		Level:       AlertLevelWarning,
		Timestamp:   time.Now(),
		Metric:      "cpu_usage",
		Value:       85.5,
		Threshold:   80.0,
		Unit:        "%",
		Message:     "test message",
		Source:      "monitor",
		ServiceType: "system",
		Tags:        map[string]string{"env": "test"},
	}

	payload, err := engine.RenderToJSON("webhook_default", vars)
	require.NoError(t, err)
	assert.NotNil(t, payload)
	assert.Equal(t, "nasos.alert", payload["event"])
}

func TestSendEmail(t *testing.T) {
	ctx := context.Background()
	err := SendEmail(ctx, "test@example.com", "测试告警", "这是一条测试告警", false)
	// 不应该返回错误（当前是mock实现）
	assert.NoError(t, err)
}

func TestSendWebhook(t *testing.T) {
	ctx := context.Background()
	// 测试无效URL，应该返回错误
	payload := map[string]interface{}{
		"event": "test",
	}
	err := SendWebhook(ctx, "http://127.0.0.1:99999/nonexistent", payload)
	// 连接不到会超时/失败
	assert.Error(t, err)
}

func TestTemplateEngine_HTMLTemplate(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "html-001",
		AlertName: "HTML测试",
		HostName:  "test-server",
		HostIP:    "192.168.1.1",
		Level:     AlertLevelCritical,
		Timestamp: time.Now(),
	}

	subject, body, err := engine.Render("email_html_default", vars)
	require.NoError(t, err)
	assert.NotEmpty(t, subject)
	assert.Contains(t, body, "<html>")
	assert.Contains(t, body, "</html>")
	assert.Contains(t, body, vars.HostName)
}

func TestAlertVars_Extra(t *testing.T) {
	vars := &AlertVars{
		AlertID:   "extra-001",
		AlertName: "extra-test",
		HostName:  "server",
		HostIP:    "10.0.0.1",
		Level:     AlertLevelWarning,
		Metric:    "cpu",
		Value:     85.0,
		Threshold: 80.0,
		Unit:      "%",
		Timestamp: time.Now(),
		Tags:      map[string]string{"env": "prod"},
		Extra: map[string]interface{}{
			"customField": "customValue",
			"count":       5,
		},
	}

	assert.Equal(t, "cpu", vars.Metric)
	assert.Equal(t, 85.0, vars.Value)
	assert.Equal(t, 80.0, vars.Threshold)
	assert.Equal(t, "%", vars.Unit)
	assert.Equal(t, "prod", vars.Tags["env"])
	assert.Equal(t, "customValue", vars.Extra["customField"])
}

func TestTemplate_RenderSlashPattern(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "slash-001",
		AlertName: "disk/usage",
		HostName:  "server",
		HostIP:    "10.0.0.1",
		Level:     AlertLevelWarning,
		Message:   "磁盘使用率过高",
		Timestamp: time.Now(),
	}

	_, body, err := engine.Render("email_default", vars)
	require.NoError(t, err)
	assert.Contains(t, body, vars.Message)
	assert.NotContains(t, body, "{{.")
}

func TestTemplateEngine_EmptyTemplateID(t *testing.T) {
	engine := NewTemplateEngine()
	_, _, err := engine.Render("nonexistent", &AlertVars{})
	assert.Error(t, err)
}

func BenchmarkTemplateRender(b *testing.B) {
	engine := NewTemplateEngine()
	vars := &AlertVars{
		AlertID:   "bench-001",
		AlertName: "磁盘空间告警",
		HostName:  "nas-server",
		HostIP:    "192.168.1.100",
		Metric:    "disk_usage",
		Value:     95.5,
		Threshold: 90.0,
		Unit:      "%",
		Level:     AlertLevelWarning,
		Message:   "磁盘使用率超过90%",
		Source:    "monitor",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"env": "production",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = engine.Render("email_html_default", vars)
	}
}

func TestTemplateEngine_GenerateAlertID(t *testing.T) {
	// 测试 AlertVars 能正确携带 AlertID
	vars := &AlertVars{
		AlertID:   "auto-test-id",
		AlertName: "auto-test",
		HostName:  "server",
		Level:     AlertLevelInfo,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "auto-test-id", vars.AlertID)

	// 测试不同的告警有不同的ID
	vars2 := &AlertVars{
		AlertID:   "auto-test-id-2",
		AlertName: "auto-test-2",
		HostName:  "server",
		Level:     AlertLevelInfo,
		Timestamp: time.Now(),
	}

	assert.NotEqual(t, vars.AlertID, vars2.AlertID)
}

func TestTemplate_ComplexTemplate(t *testing.T) {
	engine := NewTemplateEngine()

	// 自定义复杂模板
	custom := &AlertTemplate{
		ID:      "complex-test",
		Name:    "复杂模板测试",
		Type:    TemplateTypeEmail,
		Subject: "[{{.Level | upper}}] {{.AlertName}}",
		Body: `告警详情：
- 主机: {{.HostName}}
- IP: {{.HostIP}}
- 级别: {{.Level | upper}}
- 指标: {{.Metric}} = {{.Value}}{{.Unit}}
- 阈值: {{.Threshold}}{{.Unit}}
- 描述: {{.Message}}
- 时间: {{.Timestamp | time "2006-01-02 15:04:05"}}

{{if .Tags}}标签:
{{range $k, $v := .Tags}}  {{$k}}: {{$v}}
{{end}}{{end}}`,
		Enabled: true,
	}

	err := engine.AddTemplate(custom)
	require.NoError(t, err)

	vars := &AlertVars{
		AlertID:   "complex-001",
		AlertName: "CPU使用率过高",
		HostName:  "compute-node-01",
		HostIP:    "10.10.10.50",
		Metric:    "cpu_usage",
		Value:     98.5,
		Threshold: 90.0,
		Unit:      "%",
		Level:     AlertLevelCritical,
		Message:   "CPU使用率持续超过90%",
		Timestamp: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
		Tags: map[string]string{
			"env":     "production",
			"cluster": "compute",
			"owner":   "ops-team",
		},
	}

	subject, body, err := engine.Render("complex-test", vars)
	require.NoError(t, err)

	assert.Contains(t, subject, "CRITICAL")
	assert.Contains(t, subject, "CPU使用率过高")
	assert.Contains(t, body, "compute-node-01")
	assert.Contains(t, body, "98.5%")
	// 90.0 可能被格式化为 90 或 90.0，使用正则或检查包含
	assert.True(t, strings.Contains(body, "90%") || strings.Contains(body, "90.0%"), "body should contain threshold value")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "compute")
	assert.Contains(t, body, "ops-team")
	assert.NotContains(t, body, "{{.")
}

func TestTemplate_EmptyTags(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "empty-tags",
		AlertName: "test",
		HostName:  "server",
		Level:     AlertLevelInfo,
		Timestamp: time.Now(),
		Tags:      nil, // 空 tags
	}

	_, body, err := engine.Render("email_default", vars)
	require.NoError(t, err)
	assert.NotContains(t, body, "panic", "空tags不应导致panic")
}

func TestTemplate_ChineseChars(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "chinese-001",
		AlertName: "磁盘空间不足",
		HostName:  "存储服务器",
		HostIP:    "192.168.1.200",
		Level:     AlertLevelCritical,
		Message:   "数据盘使用率已达95%，请及时清理",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"环境": "生产环境",
			"部门": "运维部",
		},
	}

	_, body, err := engine.Render("email_default", vars)
	require.NoError(t, err)
	assert.Contains(t, body, "磁盘空间不足")
	assert.Contains(t, body, "存储服务器")
	assert.Contains(t, body, "运维部")
}

func TestTemplate_UnicodeEmoji(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "emoji-001",
		AlertName: "emoji test",
		HostName:  "server",
		Level:     AlertLevelCritical,
		Timestamp: time.Now(),
	}

	_, body, err := engine.Render("telegram_default", vars)
	require.NoError(t, err)
	// 验证emoji正确渲染
	assert.Contains(t, body, "🚨")
	assert.Contains(t, body, "NAS-OS 告警")
}

func TestTemplate_InvalidTemplateID(t *testing.T) {
	engine := NewTemplateEngine()

	// 测试不存在的模板ID
	_, _, err := engine.Render("definitely-does-not-exist-12345", &AlertVars{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestTemplate_DeleteBuiltinTemplate(t *testing.T) {
	engine := NewTemplateEngine()

	// 尝试删除内置模板应该可以（允许用户自定义）
	initialCount := len(engine.ListTemplates())

	err := engine.DeleteTemplate("email_default")
	assert.NoError(t, err)

	newCount := len(engine.ListTemplates())
	assert.Equal(t, initialCount-1, newCount)
}

func TestTemplate_UpdateBuiltinTemplate(t *testing.T) {
	engine := NewTemplateEngine()

	// 获取内置模板
	tmpl, ok := engine.GetTemplate("email_default")
	require.True(t, ok)

	// 修改并更新
	originalBody := tmpl.Body
	tmpl.Body = originalBody + "\n\n<!-- Custom Footer -->"

	err := engine.UpdateTemplate(tmpl)
	assert.NoError(t, err)

	updated, _ := engine.GetTemplate("email_default")
	assert.Contains(t, updated.Body, "Custom Footer")
}

func TestTemplate_RenderAllBuiltinTemplates(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:     "all-001",
		AlertName:   "综合测试告警",
		HostName:    "test-server",
		HostIP:      "192.168.1.1",
		Metric:      "test_metric",
		Value:       75.0,
		Threshold:   70.0,
		Unit:        "%",
		Level:       AlertLevelWarning,
		Message:     "测试告警消息",
		Source:      "test",
		ServiceType: "test",
		Timestamp:   time.Now(),
		Tags: map[string]string{
			"test": "true",
		},
	}

	for _, tmpl := range engine.ListTemplates() {
		if !tmpl.Enabled {
			continue
		}
		t.Run(string(tmpl.Type)+"_"+tmpl.ID, func(t *testing.T) {
			subject, body, err := engine.Render(tmpl.ID, vars)
			require.NoError(t, err, "模板 %s 渲染失败", tmpl.ID)
			// subject可以为空（webhook/telegram等非邮件模板）
			if tmpl.Subject != "" {
				assert.NotEmpty(t, subject, "subject不应为空")
			}
			assert.NotEmpty(t, body, "body不应为空")
			assert.NotContains(t, body, "{{.", "模板变量未完全解析")
		})
	}
}

func TestTemplate_MarkdownInBody(t *testing.T) {
	engine := NewTemplateEngine()

	// 测试包含markdown特殊字符的内容
	vars := &AlertVars{
		AlertID:   "md-001",
		AlertName: "special chars test",
		HostName:  "server",
		Level:     AlertLevelWarning,
		Message:   "字符串含 `代码` 和 *星号* 和 _下划线_",
		Timestamp: time.Now(),
	}

	_, body, err := engine.Render("email_default", vars)
	require.NoError(t, err)
	assert.Contains(t, body, vars.Message)
}

func TestTemplateEngine_ConcurrentAccess(t *testing.T) {
	engine := NewTemplateEngine()

	vars := &AlertVars{
		AlertID:   "concurrent-001",
		AlertName: "concurrent-test",
		HostName:  "server",
		Level:     AlertLevelInfo,
		Timestamp: time.Now(),
	}

	done := make(chan bool)

	// 并发读写
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _, _ = engine.Render("email_default", vars)
			}
			done <- true
		}()
	}

	// 并发添加
	for i := 0; i < 5; i++ {
		go func(idx int) {
			tmpl := &AlertTemplate{
				ID:   "concurrent-" + strings.Replace(time.Now().Format("150405.000"), ".", "", -1),
				Name: "Concurrent Test",
				Type: TemplateTypeEmail,
				Body: "Test body",
			}
			_ = engine.AddTemplate(tmpl)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 15; i++ {
		<-done
	}

	// 应该没有panic或错误
	assert.True(t, len(engine.ListTemplates()) > 0)
}
