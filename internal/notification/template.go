package notification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
)

// TemplateManager 模板管理器
type TemplateManager struct {
	templates map[string]*Template
	mu        sync.RWMutex
	storePath string
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(storePath string) (*TemplateManager, error) {
	tm := &TemplateManager{
		templates: make(map[string]*Template),
		storePath: storePath,
	}

	if err := tm.load(); err != nil {
		return nil, fmt.Errorf("加载模板失败: %w", err)
	}

	// 加载内置模板
	tm.loadBuiltinTemplates()

	return tm, nil
}

// load 加载模板
func (tm *TemplateManager) load() error {
	if tm.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(tm.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var templates []*Template
	if err := json.Unmarshal(data, &templates); err != nil {
		return err
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range templates {
		tm.templates[t.ID] = t
	}

	return nil
}

// save 保存模板
func (tm *TemplateManager) save() error {
	if tm.storePath == "" {
		return nil
	}

	tm.mu.RLock()
	templates := make([]*Template, 0, len(tm.templates))
	for _, t := range tm.templates {
		templates = append(templates, t)
	}
	tm.mu.RUnlock()

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(tm.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(tm.storePath, data, 0640)
}

// loadBuiltinTemplates 加载内置模板
func (tm *TemplateManager) loadBuiltinTemplates() {
	builtinTemplates := []*Template{
		{
			ID:          "system-alert",
			Name:        "系统告警",
			Description: "系统告警通知模板",
			Category:    "system",
			Subject:     "[{{.level}}] NAS-OS 系统告警：{{.title}}",
			Body: `告警标题：{{.title}}
告警级别：{{.level}}
告警时间：{{.timestamp}}
告警来源：{{.source}}

详细信息：
{{.message}}

---
此消息由 NAS-OS 系统自动发送`,
			Variables: []TemplateVariable{
				{Name: "title", Description: "告警标题", Required: true},
				{Name: "level", Description: "告警级别", Required: true},
				{Name: "message", Description: "告警详情", Required: true},
				{Name: "source", Description: "告警来源"},
				{Name: "timestamp", Description: "告警时间"},
			},
			Channels: []ChannelType{ChannelEmail, ChannelWebhook, ChannelWebSocket},
		},
		{
			ID:          "storage-alert",
			Name:        "存储告警",
			Description: "存储空间告警通知模板",
			Category:    "storage",
			Subject:     "存储空间告警：{{.poolName}}",
			Body: `存储池：{{.poolName}}
使用率：{{.usagePercent}}%
剩余空间：{{.availableSpace}}

{{if .threshold}}阈值：{{.threshold}}%{{end}}

建议：请及时清理或扩容存储空间。

告警时间：{{.timestamp}}`,
			Variables: []TemplateVariable{
				{Name: "poolName", Description: "存储池名称", Required: true},
				{Name: "usagePercent", Description: "使用百分比", Required: true},
				{Name: "availableSpace", Description: "剩余空间"},
				{Name: "threshold", Description: "告警阈值"},
				{Name: "timestamp", Description: "告警时间"},
			},
			Channels: []ChannelType{ChannelEmail, ChannelWebhook},
		},
		{
			ID:          "backup-complete",
			Name:        "备份完成",
			Description: "备份任务完成通知模板",
			Category:    "backup",
			Subject:     "备份任务完成：{{.taskName}}",
			Body: `任务名称：{{.taskName}}
任务状态：{{.status}}
开始时间：{{.startTime}}
结束时间：{{.endTime}}
耗时：{{.duration}}

{{if .size}}数据量：{{.size}}{{end}}
{{if .errorCount}}错误数：{{.errorCount}}{{end}}`,
			Variables: []TemplateVariable{
				{Name: "taskName", Description: "任务名称", Required: true},
				{Name: "status", Description: "任务状态", Required: true},
				{Name: "startTime", Description: "开始时间"},
				{Name: "endTime", Description: "结束时间"},
				{Name: "duration", Description: "耗时"},
				{Name: "size", Description: "数据量"},
				{Name: "errorCount", Description: "错误数"},
			},
			Channels: []ChannelType{ChannelEmail, ChannelWebhook, ChannelWebSocket},
		},
		{
			ID:          "user-activity",
			Name:        "用户活动",
			Description: "用户活动通知模板",
			Category:    "security",
			Subject:     "用户活动通知：{{.activityType}}",
			Body: `用户：{{.username}}
活动：{{.activityType}}
IP 地址：{{.ipAddress}}
时间：{{.timestamp}}

{{if .details}}详情：{{.details}}{{end}}`,
			Variables: []TemplateVariable{
				{Name: "username", Description: "用户名", Required: true},
				{Name: "activityType", Description: "活动类型", Required: true},
				{Name: "ipAddress", Description: "IP 地址"},
				{Name: "timestamp", Description: "时间"},
				{Name: "details", Description: "详情"},
			},
			Channels: []ChannelType{ChannelEmail, ChannelWebSocket},
		},
		{
			ID:          "service-status",
			Name:        "服务状态",
			Description: "服务状态变更通知模板",
			Category:    "service",
			Subject:     "服务状态变更：{{.serviceName}}",
			Body: `服务名称：{{.serviceName}}
新状态：{{.newStatus}}
{{if .oldStatus}}原状态：{{.oldStatus}}{{end}}
时间：{{.timestamp}}

{{if .message}}消息：{{.message}}{{end}}`,
			Variables: []TemplateVariable{
				{Name: "serviceName", Description: "服务名称", Required: true},
				{Name: "newStatus", Description: "新状态", Required: true},
				{Name: "oldStatus", Description: "原状态"},
				{Name: "timestamp", Description: "时间"},
				{Name: "message", Description: "消息"},
			},
			Channels: []ChannelType{ChannelEmail, ChannelWebhook, ChannelWebSocket},
		},
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range builtinTemplates {
		if _, exists := tm.templates[t.ID]; !exists {
			t.CreatedAt = time.Now()
			t.UpdatedAt = time.Now()
			tm.templates[t.ID] = t
		}
	}
}

// Create 创建模板
func (tm *TemplateManager) Create(template *Template) error {
	if template.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}

	if template.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.templates[template.ID]; exists {
		return fmt.Errorf("模板已存在: %s", template.ID)
	}

	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	tm.templates[template.ID] = template

	return tm.save()
}

// Update 更新模板
func (tm *TemplateManager) Update(template *Template) error {
	if template.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, exists := tm.templates[template.ID]
	if !exists {
		return fmt.Errorf("模板不存在: %s", template.ID)
	}

	// 保留创建时间
	template.CreatedAt = existing.CreatedAt
	template.UpdatedAt = time.Now()
	tm.templates[template.ID] = template

	return tm.save()
}

// Delete 删除模板
func (tm *TemplateManager) Delete(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.templates[id]; !exists {
		return fmt.Errorf("模板不存在: %s", id)
	}

	delete(tm.templates, id)
	return tm.save()
}

// Get 获取模板
func (tm *TemplateManager) Get(id string) (*Template, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	template, exists := tm.templates[id]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", id)
	}

	return template, nil
}

// List 列出模板
func (tm *TemplateManager) List(category string) []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Template, 0)
	for _, t := range tm.templates {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}

	return result
}

// Render 渲染模板
func (tm *TemplateManager) Render(templateID string, variables map[string]interface{}) (*RenderedTemplate, error) {
	tmpl, err := tm.Get(templateID)
	if err != nil {
		return nil, err
	}

	// 合并默认变量
	mergedVars := make(map[string]interface{})
	for _, v := range tmpl.Variables {
		if v.Default != "" {
			mergedVars[v.Name] = v.Default
		}
	}
	for k, v := range variables {
		mergedVars[k] = v
	}

	// 添加内置变量
	mergedVars["timestamp"] = time.Now().Format("2006-01-02 15:04:05")
	mergedVars["date"] = time.Now().Format("2006-01-02")
	mergedVars["time"] = time.Now().Format("15:04:05")

	// 渲染标题
	subject, err := renderTemplate(tmpl.Subject, mergedVars)
	if err != nil {
		return nil, fmt.Errorf("渲染标题失败: %w", err)
	}

	// 渲染正文
	body, err := renderTemplate(tmpl.Body, mergedVars)
	if err != nil {
		return nil, fmt.Errorf("渲染正文失败: %w", err)
	}

	return &RenderedTemplate{
		TemplateID: templateID,
		Subject:    subject,
		Body:       body,
		Variables:  mergedVars,
	}, nil
}

// RenderedTemplate 渲染后的模板
type RenderedTemplate struct {
	TemplateID string
	Subject    string
	Body       string
	Variables  map[string]interface{}
}

// renderTemplate 渲染文本模板
func renderTemplate(text string, variables map[string]interface{}) (string, error) {
	// 方法1：使用 Go text/template
	tmpl, err := template.New("notification").Parse(text)
	if err != nil {
		// 如果 Go 模板解析失败，使用简单变量替换
		return simpleVariableReplace(text, variables), nil
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, variables); err != nil {
		return simpleVariableReplace(text, variables), nil
	}

	return buf.String(), nil
}

// simpleVariableReplace 简单变量替换
func simpleVariableReplace(text string, variables map[string]interface{}) string {
	result := text

	// 替换 {{.var}} 格式
	re := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		varName := re.FindStringSubmatch(match)[1]
		if val, ok := variables[varName]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})

	// 替换 {{.var}} 格式（带默认值）
	reWithDefault := regexp.MustCompile(`\{\{\.(\w+)\s+\|\s+default\s+"([^"]*)"\}\}`)
	result = reWithDefault.ReplaceAllStringFunc(result, func(match string) string {
		matches := reWithDefault.FindStringSubmatch(match)
		varName := matches[1]
		defaultVal := matches[2]
		if val, ok := variables[varName]; ok && val != nil && val != "" {
			return fmt.Sprintf("%v", val)
		}
		return defaultVal
	})

	return result
}

// ValidateVariables 验证模板变量
func (tm *TemplateManager) ValidateVariables(templateID string, variables map[string]interface{}) error {
	tmpl, err := tm.Get(templateID)
	if err != nil {
		return err
	}

	for _, v := range tmpl.Variables {
		if v.Required {
			val, exists := variables[v.Name]
			if !exists || val == nil || val == "" {
				return fmt.Errorf("必填变量缺失: %s", v.Name)
			}
		}
	}

	return nil
}

// Clone 克隆模板
func (tm *TemplateManager) Clone(sourceID, targetID string) (*Template, error) {
	source, err := tm.Get(sourceID)
	if err != nil {
		return nil, err
	}

	clone := *source
	clone.ID = targetID
	clone.Name = source.Name + " (副本)"
	clone.CreatedAt = time.Now()
	clone.UpdatedAt = time.Now()

	if err := tm.Create(&clone); err != nil {
		return nil, err
	}

	return &clone, nil
}

// Export 导出模板
func (tm *TemplateManager) Export(ids []string) ([]*Template, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Template, 0, len(ids))
	for _, id := range ids {
		if t, exists := tm.templates[id]; exists {
			result = append(result, t)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到要导出的模板")
	}

	return result, nil
}

// Import 导入模板
func (tm *TemplateManager) Import(templates []*Template, overwrite bool) error {
	for _, t := range templates {
		tm.mu.Lock()
		_, exists := tm.templates[t.ID]
		tm.mu.Unlock()

		if exists && !overwrite {
			continue
		}

		if exists {
			if err := tm.Update(t); err != nil {
				return err
			}
		} else {
			if err := tm.Create(t); err != nil {
				return err
			}
		}
	}

	return nil
}
