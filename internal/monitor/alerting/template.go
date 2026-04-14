// Package alerting 告警系统：模板、路由、聚合
package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"nas-os/internal/monitor"
)

// TemplateType 模板类型
type TemplateType string

const (
	TemplateTypeEmail     TemplateType = "email"
	TemplateTypeWebhook   TemplateType = "webhook"
	TemplateTypeTelegram  TemplateType = "telegram"
	TemplateTypeDingTalk   TemplateType = "dingtalk"
	TemplateTypeWeChat    TemplateType = "wechat"
	TemplateTypeSMS       TemplateType = "sms"
	TemplateTypeSlack     TemplateType = "slack"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelDebug    AlertLevel = "debug"
)

// AlertVars 告警变量，用于模板渲染
type AlertVars struct {
	AlertID      string                 `json:"alertId"`
	AlertName   string                 `json:"alertName"`
	HostName    string                 `json:"hostName"`
	HostIP      string                 `json:"hostIP"`
	Metric      string                 `json:"metric"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold"`
	Unit        string                 `json:"unit"`
	Level       AlertLevel             `json:"level"`
	Message     string                 `json:"message"`
	Source      string                 `json:"source"`
	ServiceType string                 `json:"serviceType"`
	Tags        map[string]string      `json:"tags"`
	Extra       map[string]interface{} `json:"extra"`
	Timestamp   time.Time              `json:"timestamp"`
}

// AlertTemplate 告警模板
type AlertTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        TemplateType `json:"type"`
	Level       AlertLevel   `json:"level"`
	Subject     string       `json:"subject"`     // 邮件/消息标题模板
	Body        string       `json:"body"`        // 正文模板
	IsHTML      bool         `json:"isHTML"`      // 是否为HTML格式
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// TemplateEngine 告警模板引擎
type TemplateEngine struct {
	mu       sync.RWMutex
	templates map[string]*AlertTemplate
	funcMap   template.FuncMap
}

// NewTemplateEngine 创建模板引擎
func NewTemplateEngine() *TemplateEngine {
	te := &TemplateEngine{
		templates: make(map[string]*AlertTemplate),
		funcMap:   make(template.FuncMap),
	}
	te.registerFuncs()
	te.registerDefaultTemplates()
	return te
}

// registerFuncs 注册自定义函数
func (te *TemplateEngine) registerFuncs() {
	te.funcMap["upper"] = strings.ToUpper
	te.funcMap["lower"] = strings.ToLower
	te.funcMap["title"] = strings.Title
	te.funcMap["trim"] = strings.TrimSpace
	te.funcMap["replace"] = strings.ReplaceAll
	te.funcMap["contains"] = strings.Contains
	te.funcMap["printf"] = fmt.Sprintf
	te.funcMap["json"] = te.jsonMarshal
	te.funcMap["time"] = te.formatTime
	te.funcMap["levelEmoji"] = te.levelEmoji
	te.funcMap["levelColor"] = te.levelColor
	te.funcMap["levelIcon"] = te.levelIcon
}

func (te *TemplateEngine) jsonMarshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (te *TemplateEngine) formatTime(t time.Time, layout string) string {
	if layout == "" {
		layout = "2006-01-02 15:04:05"
	}
	return t.Format(layout)
}

func (te *TemplateEngine) levelEmoji(level AlertLevel) string {
	switch level {
	case AlertLevelCritical:
		return "🚨"
	case AlertLevelWarning:
		return "⚠️"
	case AlertLevelInfo:
		return "ℹ️"
	case AlertLevelDebug:
		return "🔍"
	default:
		return "📢"
	}
}

func (te *TemplateEngine) levelColor(level AlertLevel) string {
	switch level {
	case AlertLevelCritical:
		return "#DC2626"
	case AlertLevelWarning:
		return "#F59E0B"
	case AlertLevelInfo:
		return "#3B82F6"
	case AlertLevelDebug:
		return "#6B7280"
	default:
		return "#6B7280"
	}
}

func (te *TemplateEngine) levelIcon(level AlertLevel) string {
	switch level {
	case AlertLevelCritical:
		return "🔴"
	case AlertLevelWarning:
		return "🟡"
	case AlertLevelInfo:
		return "🔵"
	case AlertLevelDebug:
		return "⚪"
	default:
		return "⚪"
	}
}

// registerDefaultTemplates 注册默认模板
func (te *TemplateEngine) registerDefaultTemplates() {
	defaults := []*AlertTemplate{
		// Email 模板
		{
			ID:          "email_default",
			Name:        "默认邮件模板",
			Description: "通用邮件告警模板",
			Type:        TemplateTypeEmail,
			Level:       AlertLevelWarning,
			Subject:     "[{{.Level | upper}}] NAS-OS 告警 - {{.AlertName}}",
			Body: `告警详情：

告警名称：{{.AlertName}}
主机：{{.HostName}} ({{.HostIP}})
级别：{{.Level | upper}}
时间：{{.Timestamp | time}}
{{if .Metric}}指标：{{.Metric}}{{end}}
{{if .Value}}当前值：{{.Value}}{{.Unit}}{{end}}
{{if .Threshold}}阈值：{{.Threshold}}{{.Unit}}{{end}}
{{if .Message}}描述：{{.Message}}{{end}}
{{if .Source}}来源：{{.Source}}{{end}}

{{if .Tags}}标签：
{{range $k, $v := .Tags}}  {{$k}}: {{$v}}
{{end}}{{end}}

请及时处理。`,
			IsHTML:  false,
			Enabled: true,
		},
		{
			ID:          "email_html_default",
			Name:        "HTML邮件模板",
			Description: "HTML格式邮件告警模板",
			Type:        TemplateTypeEmail,
			Level:       AlertLevelWarning,
			Subject:     "[{{.Level | upper}}] NAS-OS 告警 - {{.AlertName}}",
			Body: `<!DOCTYPE html>
<html>
<head>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
.container { max-width: 600px; margin: 0 auto; padding: 20px; }
.header { background: {{.Level | levelColor}}; color: white; padding: 15px; border-radius: 8px 8px 0 0; text-align: center; }
.content { background: #f8f9fa; padding: 20px; border: 1px solid #e9ecef; border-top: none; }
.row { display: flex; padding: 8px 0; border-bottom: 1px solid #e9ecef; }
.label { font-weight: 600; width: 120px; color: #374151; }
.value { flex: 1; color: #6b7280; }
.footer { background: #f3f4f6; padding: 15px; border-radius: 0 0 8px 8px; text-align: center; font-size: 12px; color: #6b7280; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h2 style="margin:0;">{{.Level | levelEmoji}} NAS-OS 系统告警</h2>
  </div>
  <div class="content">
    <div class="row"><span class="label">告警名称</span><span class="value">{{.AlertName}}</span></div>
    <div class="row"><span class="label">主机</span><span class="value">{{.HostName}} ({{.HostIP}})</span></div>
    <div class="row"><span class="label">级别</span><span class="value">{{.Level | upper}}</span></div>
    <div class="row"><span class="label">时间</span><span class="value">{{.Timestamp | time}}</span></div>
    {{if .Metric}}<div class="row"><span class="label">指标</span><span class="value">{{.Metric}}</span></div>{{end}}
    {{if .Value}}<div class="row"><span class="label">当前值</span><span class="value">{{.Value}}{{.Unit}}</span></div>{{end}}
    {{if .Threshold}}<div class="row"><span class="label">阈值</span><span class="value">{{.Threshold}}{{.Unit}}</span></div>{{end}}
    {{if .Message}}<div class="row"><span class="label">描述</span><span class="value">{{.Message}}</span></div>{{end}}
    {{if .Source}}<div class="row"><span class="label">来源</span><span class="value">{{.Source}}</span></div></div>{{end}}
  </div>
  <div class="footer">NAS-OS 监控系统 | 请及时处理此告警</div>
</div>
</body>
</html>`,
			IsHTML:  true,
			Enabled: true,
		},
		// Webhook 模板
		{
			ID:          "webhook_default",
			Name:        "通用Webhook模板",
			Description: "通用JSON Webhook告警模板",
			Type:        TemplateTypeWebhook,
			Level:       AlertLevelWarning,
			Subject:     "",
			Body: `{
  "event": "nasos.alert",
  "alertId": "{{.AlertID}}",
  "alertName": "{{.AlertName}}",
  "level": "{{.Level}}",
  "host": {
    "name": "{{.HostName}}",
    "ip": "{{.HostIP}}"
  },
  "metric": {{.Metric | json}},
  "value": {{.Value}},
  "threshold": {{.Threshold}},
  "unit": "{{.Unit}}",
  "message": "{{.Message}}",
  "source": "{{.Source}}",
  "serviceType": "{{.ServiceType}}",
  "tags": {{.Tags | json}},
  "timestamp": "{{.Timestamp | time "2006-01-02T15:04:05Z07:00"}}"
}`,
			IsHTML:  false,
			Enabled: true,
		},
		// Telegram 模板
		{
			ID:          "telegram_default",
			Name:        "Telegram模板",
			Description: "Telegram Markdown告警模板",
			Type:        TemplateTypeTelegram,
			Level:       AlertLevelWarning,
			Subject:     "",
			Body: `{{.Level | levelEmoji}} *NAS-OS 告警*

*告警名称:* {{.AlertName}}
*主机:* {{.HostName}} ({{.HostIP}})
*级别:* {{.Level | upper}}
*时间:* {{.Timestamp | time}}

{{if .Metric}}📊 *指标:* {{.Metric}}{{end}}
{{if .Value}}📈 *当前值:* {{.Value}}{{.Unit}}{{end}}
{{if .Threshold}}⚠️ *阈值:* {{.Threshold}}{{.Unit}}{{end}}

{{if .Message}}📝 *描述:* {{.Message}}{{end}}
{{if .Source}}🔗 *来源:* {{.Source}}{{end}}`,
			IsHTML:  false,
			Enabled: true,
		},
		// 钉钉模板
		{
			ID:          "dingtalk_default",
			Name:        "钉钉模板",
			Description: "钉钉 Markdown 告警模板",
			Type:        TemplateTypeDingTalk,
			Level:       AlertLevelWarning,
			Subject:     "",
			Body: `## {{.Level | levelEmoji}} NAS-OS 告警通知

**告警名称:** {{.AlertName}}
**主机:** {{.HostName}} ({{.HostIP}})
**级别:** {{.Level | upper}}
**时间:** {{.Timestamp | time}}

{{if .Metric}}**指标:** {{.Metric}}{{end}}
{{if .Value}}**当前值:** {{.Value}}{{.Unit}}{{end}}
{{if .Threshold}}**阈值:** {{.Threshold}}{{.Unit}}{{end}}

{{if .Message}}**描述:** {{.Message}}{{end}}
{{if .Source}}**来源:** {{.Source}}{{end}}`,
			IsHTML:  false,
			Enabled: true,
		},
		// 企业微信模板
		{
			ID:          "wechat_default",
			Name:        "企业微信模板",
			Description: "企业微信 Markdown 告警模板",
			Type:        TemplateTypeWeChat,
			Level:       AlertLevelWarning,
			Subject:     "",
			Body: `{{.Level | levelEmoji}} **NAS-OS 告警**

> **告警名称:** {{.AlertName}}
> **主机:** {{.HostName}} ({{.HostIP}})
> **级别:** {{.Level | upper}}
> **时间:** {{.Timestamp | time}}

{{if .Metric}}> **指标:** {{.Metric}}{{end}}
{{if .Value}}> **当前值:** {{.Value}}{{.Unit}}{{end}}
{{if .Threshold}}> **阈值:** {{.Threshold}}{{.Unit}}{{end}}

{{if .Message}}> **描述:** {{.Message}}{{end}}
{{if .Source}}> **来源:** {{.Source}}{{end}}`,
			IsHTML:  false,
			Enabled: true,
		},
		// Slack 模板
		{
			ID:          "slack_default",
			Name:        "Slack模板",
			Description: "Slack Block Kit 告警模板",
			Type:        TemplateTypeSlack,
			Level:       AlertLevelWarning,
			Subject:     "",
			Body: `{
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "{{.Level | levelIcon}} NAS-OS Alert: {{.AlertName}}",
        "emoji": true
      }
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Level:*\\n{{.Level | upper}}"},
        {"type": "mrkdwn", "text": "*Host:*\\n{{.HostName}} ({{.HostIP}})"},
        {"type": "mrkdwn", "text": "*Time:*\\n{{.Timestamp | time}}"},
        {"type": "mrkdwn", "text": "*Source:*\\n{{.Source}}"}
      ]
    },
    {{if .Metric}}
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Metric:*\\n{{.Metric}}"},
        {"type": "mrkdwn", "text": "*Value:*\\n{{.Value}}{{.Unit}} (threshold: {{.Threshold}}{{.Unit}})"}
      ]
    },
    {{end}}
    {{if .Message}}
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*Message:*\\n{{.Message}}"
      }
    }
    {{end}}
  ]
}`,
			IsHTML:  false,
			Enabled: true,
		},
	}

	for _, t := range defaults {
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		te.templates[t.ID] = t
	}
}

// AddTemplate 添加模板
func (te *TemplateEngine) AddTemplate(tmpl *AlertTemplate) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if tmpl.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}
	if tmpl.Body == "" {
		return fmt.Errorf("模板内容不能为空")
	}
	if tmpl.Type == "" {
		tmpl.Type = TemplateTypeEmail
	}

	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()
	te.templates[tmpl.ID] = tmpl
	return nil
}

// UpdateTemplate 更新模板
func (te *TemplateEngine) UpdateTemplate(tmpl *AlertTemplate) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if _, exists := te.templates[tmpl.ID]; !exists {
		return fmt.Errorf("模板不存在: %s", tmpl.ID)
	}

	tmpl.UpdatedAt = time.Now()
	te.templates[tmpl.ID] = tmpl
	return nil
}

// DeleteTemplate 删除模板
func (te *TemplateEngine) DeleteTemplate(id string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if _, exists := te.templates[id]; !exists {
		return fmt.Errorf("模板不存在: %s", id)
	}

	delete(te.templates, id)
	return nil
}

// GetTemplate 获取模板
func (te *TemplateEngine) GetTemplate(id string) (*AlertTemplate, bool) {
	te.mu.RLock()
	defer te.mu.RUnlock()

	tmpl, ok := te.templates[id]
	return tmpl, ok
}

// ListTemplates 列出所有模板
func (te *TemplateEngine) ListTemplates() []*AlertTemplate {
	te.mu.RLock()
	defer te.mu.RUnlock()

	result := make([]*AlertTemplate, 0, len(te.templates))
	for _, tmpl := range te.templates {
		result = append(result, tmpl)
	}
	return result
}

// ListTemplatesByType 按类型列出模板
func (te *TemplateEngine) ListTemplatesByType(typ TemplateType) []*AlertTemplate {
	te.mu.RLock()
	defer te.mu.RUnlock()

	result := make([]*AlertTemplate, 0)
	for _, tmpl := range te.templates {
		if tmpl.Type == typ {
			result = append(result, tmpl)
		}
	}
	return result
}

// Render 渲染模板
func (te *TemplateEngine) Render(tmplID string, vars *AlertVars) (subject string, body string, err error) {
	te.mu.RLock()
	tmpl, ok := te.templates[tmplID]
	te.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("模板不存在: %s", tmplID)
	}

	if tmpl.Subject != "" {
		subject, err = te.renderString(tmpl.Subject, vars)
		if err != nil {
			return "", "", fmt.Errorf("渲染标题失败: %w", err)
		}
	}

	body, err = te.renderString(tmpl.Body, vars)
	if err != nil {
		return "", "", fmt.Errorf("渲染内容失败: %w", err)
	}

	return subject, body, nil
}

// RenderToBytes 渲染模板为字节
func (te *TemplateEngine) RenderToBytes(tmplID string, vars *AlertVars) ([]byte, error) {
	_, body, err := te.Render(tmplID, vars)
	if err != nil {
		return nil, err
	}
	return []byte(body), nil
}

// renderString 渲染单个字符串模板
func (te *TemplateEngine) renderString(tmplStr string, vars *AlertVars) (string, error) {
	tmpl, err := template.New("alert").Funcs(te.funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}

	return buf.String(), nil
}

// RenderAlertVarsFromMonitor 从监控Alert转换为渲染变量
func RenderAlertVarsFromMonitor(alert *monitor.Alert, hostName, hostIP string, extra map[string]interface{}) *AlertVars {
	vars := &AlertVars{
		AlertID:    alert.ID,
		AlertName:  alert.Type,
		HostName:   hostName,
		HostIP:     hostIP,
		Level:      AlertLevel(alert.Level),
		Message:    alert.Message,
		Source:     alert.Source,
		Timestamp:  alert.Timestamp,
		Tags:       make(map[string]string),
		Extra:     extra,
	}

	if extra != nil {
		if metric, ok := extra["metric"].(string); ok {
			vars.Metric = metric
		}
		if value, ok := extra["value"].(float64); ok {
			vars.Value = value
		}
		if threshold, ok := extra["threshold"].(float64); ok {
			vars.Threshold = threshold
		}
		if unit, ok := extra["unit"].(string); ok {
			vars.Unit = unit
		}
		if serviceType, ok := extra["serviceType"].(string); ok {
			vars.ServiceType = serviceType
		}
	}

	return vars
}

// RenderToJSON 将渲染结果转为JSON（用于Webhook等）
func (te *TemplateEngine) RenderToJSON(tmplID string, vars *AlertVars) (map[string]interface{}, error) {
	te.mu.RLock()
	tmpl, ok := te.templates[tmplID]
	te.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", tmplID)
	}

	// 如果模板是JSON格式，尝试解析
	body, err := te.renderString(tmpl.Body, vars)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		// 如果不是有效JSON，作为普通文本返回
		return map[string]interface{}{
			"content": body,
		}, nil
	}

	return result, nil
}

// SendEmail 发送邮件（模拟实现）
func SendEmail(ctx context.Context, to, subject, body string, isHTML bool) error {
	// 这里应该接入实际的邮件发送服务
	// 暂时打印到日志
	fmt.Printf("[Email] To: %s, Subject: %s, HTML: %v\n", to, subject, isHTML)
	return nil
}

// SendWebhook 发送Webhook
func SendWebhook(ctx context.Context, url string, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化payload失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}
