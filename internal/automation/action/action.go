package action

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ActionType 动作类型
type ActionType string

const (
	ActionTypeMove      ActionType = "move"
	ActionTypeCopy      ActionType = "copy"
	ActionTypeDelete    ActionType = "delete"
	ActionTypeRename    ActionType = "rename"
	ActionTypeConvert   ActionType = "convert"
	ActionTypeNotify    ActionType = "notify"
	ActionTypeCommand   ActionType = "command"
	ActionTypeWebhook   ActionType = "webhook"
	ActionTypeEmail     ActionType = "email"
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
	Type      ActionType `json:"type"`
	Path      string     `json:"path"`
	NewName   string     `json:"new_name"`
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
	Type        ActionType `json:"type"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	Format      string     `json:"format"` // e.g., "jpg", "png", "pdf", "mp4"
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
		// TODO: 实现 PDF 转换
		return fmt.Errorf("PDF conversion not implemented")
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

	fmt.Printf("Notification [%s]: %s - %s\n", a.Channel, title, message)
	// TODO: 实现实际的通知发送
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
	Type    ActionType `json:"type"`
	URL     string     `json:"url"`
	Method  string     `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string     `json:"body,omitempty"`
}

func (a *WebhookAction) GetType() ActionType {
	return ActionTypeWebhook
}

func (a *WebhookAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	// TODO: 实现 HTTP 请求
	fmt.Printf("Sending webhook to: %s\n", a.URL)
	return nil
}

// EmailAction 发送邮件
type EmailAction struct {
	Type      ActionType `json:"type"`
	To        string     `json:"to"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	HTML      bool       `json:"html,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}

func (a *EmailAction) GetType() ActionType {
	return ActionTypeEmail
}

func (a *EmailAction) Execute(ctx context.Context, contextData map[string]interface{}) error {
	// TODO: 实现邮件发送
	fmt.Printf("Sending email to: %s, subject: %s\n", a.To, a.Subject)
	return nil
}

// ActionConfig 动作配置（用于 JSON 序列化）
type ActionConfig struct {
	Type ActionType `json:"type"`
	
	// Common fields
	Source      string     `json:"source,omitempty"`
	Destination string     `json:"destination,omitempty"`
	Path        string     `json:"path,omitempty"`
	Overwrite   bool       `json:"overwrite,omitempty"`
	Recursive   bool       `json:"recursive,omitempty"`
	
	// Rename fields
	NewName   string     `json:"new_name,omitempty"`
	
	// Convert fields
	Format    string     `json:"format,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
	
	// Notify fields
	Channel   string     `json:"channel,omitempty"`
	Message   string     `json:"message,omitempty"`
	Title     string     `json:"title,omitempty"`
	To        string     `json:"to,omitempty"`
	
	// Command fields
	Command   string     `json:"command,omitempty"`
	Args      []string   `json:"args,omitempty"`
	WorkDir   string     `json:"work_dir,omitempty"`
	Env       []string   `json:"env,omitempty"`
	
	// Webhook fields
	URL       string     `json:"url,omitempty"`
	Method    string     `json:"method,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string     `json:"body,omitempty"`
	
	// Email fields
	Subject   string     `json:"subject,omitempty"`
	EmailBody string     `json:"email_body,omitempty"`
	HTML      bool       `json:"html,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
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
	default:
		return nil, fmt.Errorf("unknown action type: %s", config.Type)
	}
}

// 辅助函数
func replaceVariables(s string, contextData map[string]interface{}) string {
	// 简单的变量替换，支持 {{event.xxx}} 和 {{timestamp}} 等
	// TODO: 实现更完善的模板引擎
	return s
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
