package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ActionType 动作类型
type ActionType string

const (
	ActionTypeMove        ActionType = "move"
	ActionTypeCopy        ActionType = "copy"
	ActionTypeDelete      ActionType = "delete"
	ActionTypeRename      ActionType = "rename"
	ActionTypeConvert     ActionType = "convert"
	ActionTypeNotify      ActionType = "notify"
	ActionTypeCommand     ActionType = "command"
	ActionTypeWebhook     ActionType = "webhook"
	ActionTypeEmail       ActionType = "email"
	ActionTypeConditional ActionType = "conditional"
)

// Action 动作接口
type Action interface {
	GetType() ActionType
	Execute(ctx context.Context, contextData map[string]interface{}) error
}

// MoveAction 移动文件/文件夹
type MoveAction struct {
	Type        ActionType `json:"type"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	Overwrite   bool       `json:"overwrite"`
}

func (a *MoveAction) GetType() ActionType {
	return ActionTypeMove
}

func (a *MoveAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	source := a.Source
	dest := a.Destination

	// 支持变量替换
	source = replaceVariables(source, contextData)
	dest = replaceVariables(dest, contextData)

	if err := os.Rename(source, dest); err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", source, dest, err)
	}

	return nil
}

// CopyAction 复制文件/文件夹
type CopyAction struct {
	Type        ActionType `json:"type"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	Overwrite   bool       `json:"overwrite"`
	Recursive   bool       `json:"recursive"`
}

func (a *CopyAction) GetType() ActionType {
	return ActionTypeCopy
}

func (a *CopyAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	source := replaceVariables(a.Source, contextData)
	dest := replaceVariables(a.Destination, contextData)

	if err := copyFileOrDir(source, dest, a.Recursive); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", source, dest, err)
	}

	return nil
}

// DeleteAction 删除文件/文件夹
type DeleteAction struct {
	Type      ActionType `json:"type"`
	Path      string     `json:"path"`
	Recursive bool       `json:"recursive"`
}

func (a *DeleteAction) GetType() ActionType {
	return ActionTypeDelete
}

func (a *DeleteAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	path := replaceVariables(a.Path, contextData)

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete %s: %w", path, err)
	}

	return nil
}

// RenameAction 重命名文件/文件夹
type RenameAction struct {
	Type    ActionType `json:"type"`
	Path    string     `json:"path"`
	NewName string     `json:"new_name"`
}

func (a *RenameAction) GetType() ActionType {
	return ActionTypeRename
}

func (a *RenameAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	path := replaceVariables(a.Path, contextData)
	newName := replaceVariables(a.NewName, contextData)

	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, newName)

	if err := os.Rename(path, newPath); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", path, newPath, err)
	}

	return nil
}

// ConvertAction 转换文件格式
type ConvertAction struct {
	Type        ActionType             `json:"type"`
	Source      string                 `json:"source"`
	Destination string                 `json:"destination"`
	Format      string                 `json:"format"` // e.g., "jpg", "png", "pdf", "mp4"
	Options     map[string]interface{} `json:"options,omitempty"`
}

func (a *ConvertAction) GetType() ActionType {
	return ActionTypeConvert
}

func (a *ConvertAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	source := replaceVariables(a.Source, contextData)
	dest := replaceVariables(a.Destination, contextData)

	// 使用 ffmpeg 或其他工具进行转换
	var cmd *exec.Cmd
	switch a.Format {
	case "jpg", "png", "gif", "webp":
		cmd = exec.CommandContext(ctx, "convert", source, dest)
	case "mp4", "avi", "mkv":
		cmd = exec.CommandContext(ctx, "ffmpeg", "-i", source, dest)
	case "pdf":
		// PDF 转换使用 wkhtmltopdf 或类似工具
		cmd = exec.CommandContext(ctx, "wkhtmltopdf", source, dest)
		if _, err := exec.LookPath("wkhtmltopdf"); err != nil {
			// 回退到其他 PDF 工具
			if _, err := exec.LookPath("pandoc"); err == nil {
				cmd = exec.CommandContext(ctx, "pandoc", source, "-o", dest)
			} else {
				return fmt.Errorf("PDF conversion requires wkhtmltopdf or pandoc")
			}
		}
	default:
		return fmt.Errorf("unsupported format: %s", a.Format)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	return nil
}

// NotifyAction 发送通知
type NotifyAction struct {
	Type    ActionType `json:"type"`
	Channel string     `json:"channel"` // discord, email, sms, webhook
	Message string     `json:"message"`
	Title   string     `json:"title,omitempty"`
	To      string     `json:"to,omitempty"`
}

func (a *NotifyAction) GetType() ActionType {
	return ActionTypeNotify
}

func (a *NotifyAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	message := replaceVariables(a.Message, contextData)
	title := replaceVariables(a.Title, contextData)
	to := replaceVariables(a.To, contextData)

	switch a.Channel {
	case "email":
		return sendEmailNotification(to, title, message, false)
	case "webhook":
		return sendWebhookNotification(to, title, message)
	case "discord":
		return sendDiscordNotification(to, title, message)
	default:
		fmt.Printf("Notification [%s]: %s - %s\n", a.Channel, title, message)
	}
	return nil
}

// sendEmailNotification 发送邮件通知
func sendEmailNotification(to, subject, body string, html bool) error {
	// 从环境变量获取 SMTP 配置
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")

	if smtpHost == "" {
		return fmt.Errorf("SMTP 未配置")
	}

	if smtpPort == "" {
		smtpPort = "587"
	}

	// 构建邮件
	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n", from, to, subject)
	if html {
		msg += "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	} else {
		msg += "Content-Type: text/plain; charset=\"UTF-8\";\n\n"
	}
	msg += body

	// 发送邮件
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// sendWebhookNotification 发送 Webhook 通知
func sendWebhookNotification(url, title, message string) error {
	payload := map[string]interface{}{
		"title":     title,
		"message":   message,
		"timestamp": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

// sendDiscordNotification 发送 Discord 通知
func sendDiscordNotification(webhookURL, title, message string) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": message,
				"color":       5814783, // 蓝色
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

// CommandAction 执行系统命令
type CommandAction struct {
	Type    ActionType `json:"type"`
	Command string     `json:"command"`
	Args    []string   `json:"args,omitempty"`
	WorkDir string     `json:"work_dir,omitempty"`
	Env     []string   `json:"env,omitempty"`
}

func (a *CommandAction) GetType() ActionType {
	return ActionTypeCommand
}

func (a *CommandAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	cmd := exec.CommandContext(ctx, a.Command, a.Args...)

	if a.WorkDir != "" {
		cmd.Dir = replaceVariables(a.WorkDir, contextData)
	}

	if len(a.Env) > 0 {
		cmd.Env = append(os.Environ(), a.Env...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return nil
}

// WebhookAction 发送 Webhook 请求
type WebhookAction struct {
	Type    ActionType        `json:"type"`
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

func (a *WebhookAction) GetType() ActionType {
	return ActionTypeWebhook
}

func (a *WebhookAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	url := replaceVariables(a.URL, contextData)
	body := replaceVariables(a.Body, contextData)

	method := a.Method
	if method == "" {
		method = "POST"
	}

	var reqBody *bytes.Buffer
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// 设置 headers
	for key, value := range a.Headers {
		req.Header.Set(key, replaceVariables(value, contextData))
	}

	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

// EmailAction 发送邮件
type EmailAction struct {
	Type        ActionType `json:"type"`
	To          string     `json:"to"`
	Subject     string     `json:"subject"`
	Body        string     `json:"body"`
	HTML        bool       `json:"html,omitempty"`
	Attachments []string   `json:"attachments,omitempty"`
}

func (a *EmailAction) GetType() ActionType {
	return ActionTypeEmail
}

func (a *EmailAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	to := replaceVariables(a.To, contextData)
	subject := replaceVariables(a.Subject, contextData)
	body := replaceVariables(a.Body, contextData)

	return sendEmailNotification(to, subject, body, a.HTML)
}

// ConditionOperator 条件运算符
type ConditionOperator string

const (
	OperatorEquals      ConditionOperator = "equals"
	OperatorNotEquals   ConditionOperator = "not_equals"
	OperatorContains    ConditionOperator = "contains"
	OperatorNotContains ConditionOperator = "not_contains"
	OperatorGreaterThan ConditionOperator = "greater_than"
	OperatorLessThan    ConditionOperator = "less_than"
	OperatorExists      ConditionOperator = "exists"
	OperatorNotExists   ConditionOperator = "not_exists"
	OperatorMatches     ConditionOperator = "matches" // 正则匹配
)

// Condition 条件
type Condition struct {
	Field    string            `json:"field"`           // 字段路径，如 "event.type" 或 "event.data.size"
	Operator ConditionOperator `json:"operator"`        // 运算符
	Value    interface{}       `json:"value,omitempty"` // 比较值
}

// ConditionalAction 条件动作 - 根据条件决定是否执行
type ConditionalAction struct {
	Type       ActionType `json:"type"`
	Condition  Condition  `json:"condition"`             // 条件
	ThenAction Action     `json:"then_action"`           // 条件为真时执行
	ElseAction Action     `json:"else_action,omitempty"` // 条件为假时执行（可选）
}

func (a *ConditionalAction) GetType() ActionType {
	return ActionTypeConditional
}

func (a *ConditionalAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	// 评估条件
	result, err := a.evaluateCondition(contextData)
	if err != nil {
		return fmt.Errorf("condition evaluation failed: %w", err)
	}

	// 根据条件执行相应动作
	if result {
		if a.ThenAction != nil {
			return a.ThenAction.Execute(ctx, contextData)
		}
	} else {
		if a.ElseAction != nil {
			return a.ElseAction.Execute(ctx, contextData)
		}
	}

	return nil
}

// evaluateCondition 评估条件
func (a *ConditionalAction) evaluateCondition(contextData map[string]interface{}) (bool, error) {
	// 获取字段值
	fieldValue := getNestedValue(contextData, a.Condition.Field)

	switch a.Condition.Operator {
	case OperatorEquals:
		return compareEquals(fieldValue, a.Condition.Value), nil
	case OperatorNotEquals:
		return !compareEquals(fieldValue, a.Condition.Value), nil
	case OperatorContains:
		return checkContains(fieldValue, a.Condition.Value)
	case OperatorNotContains:
		result, err := checkContains(fieldValue, a.Condition.Value)
		return !result, err
	case OperatorGreaterThan:
		return compareNumbers(fieldValue, a.Condition.Value, ">")
	case OperatorLessThan:
		return compareNumbers(fieldValue, a.Condition.Value, "<")
	case OperatorExists:
		return fieldValue != nil, nil
	case OperatorNotExists:
		return fieldValue == nil, nil
	case OperatorMatches:
		return checkRegex(fieldValue, a.Condition.Value)
	default:
		return false, fmt.Errorf("unknown operator: %s", a.Condition.Operator)
	}
}

// compareEquals 比较是否相等
func compareEquals(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// checkContains 检查是否包含
func checkContains(field, value interface{}) (bool, error) {
	if field == nil {
		return false, nil
	}

	fieldStr := fmt.Sprintf("%v", field)
	valueStr := fmt.Sprintf("%v", value)

	return strings.Contains(fieldStr, valueStr), nil
}

// compareNumbers 比较数字
func compareNumbers(a, b interface{}, op string) (bool, error) {
	aFloat, err := toFloat64(a)
	if err != nil {
		return false, err
	}

	bFloat, err := toFloat64(b)
	if err != nil {
		return false, err
	}

	switch op {
	case ">":
		return aFloat > bFloat, nil
	case "<":
		return aFloat < bFloat, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %s", op)
	}
}

// toFloat64 将值转换为 float64
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// checkRegex 正则匹配检查
func checkRegex(field, pattern interface{}) (bool, error) {
	if field == nil {
		return false, nil
	}

	fieldStr := fmt.Sprintf("%v", field)
	patternStr := fmt.Sprintf("%v", pattern)

	matched, err := regexp.MatchString(patternStr, fieldStr)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return matched, nil
}

// ActionConfig 动作配置（用于 JSON 序列化）
type ActionConfig struct {
	Type ActionType `json:"type"`

	// Common fields
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Path        string `json:"path,omitempty"`
	Overwrite   bool   `json:"overwrite,omitempty"`
	Recursive   bool   `json:"recursive,omitempty"`

	// Rename fields
	NewName string `json:"new_name,omitempty"`

	// Convert fields
	Format  string                 `json:"format,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`

	// Notify fields
	Channel string `json:"channel,omitempty"`
	Message string `json:"message,omitempty"`
	Title   string `json:"title,omitempty"`
	To      string `json:"to,omitempty"`

	// Command fields
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"work_dir,omitempty"`
	Env     []string `json:"env,omitempty"`

	// Webhook fields
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`

	// Email fields
	Subject     string   `json:"subject,omitempty"`
	EmailBody   string   `json:"email_body,omitempty"`
	HTML        bool     `json:"html,omitempty"`
	Attachments []string `json:"attachments,omitempty"`

	// Conditional fields
	Condition  Condition     `json:"condition,omitempty"`
	ThenAction *ActionConfig `json:"then_action,omitempty"`
	ElseAction *ActionConfig `json:"else_action,omitempty"`
}

// NewActionFromConfig 从配置创建动作
func NewActionFromConfig(config ActionConfig) (Action, error) {
	switch config.Type {
	case ActionTypeMove:
		return &MoveAction{
			Type:        config.Type,
			Source:      config.Source,
			Destination: config.Destination,
			Overwrite:   config.Overwrite,
		}, nil
	case ActionTypeCopy:
		return &CopyAction{
			Type:        config.Type,
			Source:      config.Source,
			Destination: config.Destination,
			Overwrite:   config.Overwrite,
			Recursive:   config.Recursive,
		}, nil
	case ActionTypeDelete:
		return &DeleteAction{
			Type:      config.Type,
			Path:      config.Path,
			Recursive: config.Recursive,
		}, nil
	case ActionTypeRename:
		return &RenameAction{
			Type:    config.Type,
			Path:    config.Path,
			NewName: config.NewName,
		}, nil
	case ActionTypeConvert:
		return &ConvertAction{
			Type:        config.Type,
			Source:      config.Source,
			Destination: config.Destination,
			Format:      config.Format,
			Options:     config.Options,
		}, nil
	case ActionTypeNotify:
		return &NotifyAction{
			Type:    config.Type,
			Channel: config.Channel,
			Message: config.Message,
			Title:   config.Title,
			To:      config.To,
		}, nil
	case ActionTypeCommand:
		return &CommandAction{
			Type:    config.Type,
			Command: config.Command,
			Args:    config.Args,
			WorkDir: config.WorkDir,
			Env:     config.Env,
		}, nil
	case ActionTypeWebhook:
		return &WebhookAction{
			Type:    config.Type,
			URL:     config.URL,
			Method:  config.Method,
			Headers: config.Headers,
			Body:    config.Body,
		}, nil
	case ActionTypeEmail:
		return &EmailAction{
			Type:        config.Type,
			To:          config.To,
			Subject:     config.Subject,
			Body:        config.Body,
			HTML:        config.HTML,
			Attachments: config.Attachments,
		}, nil
	case ActionTypeConditional:
		// 解析嵌套的 then_action 和 else_action
		var thenAction, elseAction Action
		var err error

		if config.ThenAction != nil {
			thenAction, err = NewActionFromConfig(*config.ThenAction)
			if err != nil {
				return nil, fmt.Errorf("failed to create then_action: %w", err)
			}
		}

		if config.ElseAction != nil {
			elseAction, err = NewActionFromConfig(*config.ElseAction)
			if err != nil {
				return nil, fmt.Errorf("failed to create else_action: %w", err)
			}
		}

		return &ConditionalAction{
			Type:       config.Type,
			Condition:  config.Condition,
			ThenAction: thenAction,
			ElseAction: elseAction,
		}, nil
	default:
		return nil, fmt.Errorf("unknown action type: %s", config.Type)
	}
}

// 辅助函数
func replaceVariables(s string, contextData map[string]interface{}) string {
	if contextData == nil {
		return s
	}

	result := s

	// 支持 {{variable}} 和 {{nested.path}} 格式
	for i := 0; i < len(result); i++ {
		if result[i] == '{' && i+1 < len(result) && result[i+1] == '{' {
			// 找到结束的 }}
			end := strings.Index(result[i:], "}}")
			if end == -1 {
				continue
			}
			end += i

			// 提取变量名
			varName := strings.TrimSpace(result[i+2 : end])

			// 解析嵌套路径
			value := getNestedValue(contextData, varName)

			// 替换变量
			if value != nil {
				result = result[:i] + fmt.Sprintf("%v", value) + result[end+2:]
				i-- // 重新检查当前位置
			} else {
				// 内置变量
				switch varName {
				case "timestamp":
					result = result[:i] + time.Now().Format(time.RFC3339) + result[end+2:]
					i--
				case "date":
					result = result[:i] + time.Now().Format("2006-01-02") + result[end+2:]
					i--
				case "time":
					result = result[:i] + time.Now().Format("15:04:05") + result[end+2:]
					i--
				case "datetime":
					result = result[:i] + time.Now().Format("2006-01-02 15:04:05") + result[end+2:]
					i--
				case "unix":
					result = result[:i] + fmt.Sprintf("%d", time.Now().Unix()) + result[end+2:]
					i--
				}
			}
		}
	}

	return result
}

// getNestedValue 从嵌套 map 中获取值
func getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil
			}
		case map[string]string:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

func copyFileOrDir(source, dest string, recursive bool) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return copyFile(source, dest)
	}

	if !recursive {
		return fmt.Errorf("cannot copy directory without recursive flag")
	}

	return copyDir(source, dest)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
