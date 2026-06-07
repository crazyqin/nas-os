// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ========== 文件操作处理器 ==========

// FileOpsHandler 文件操作动作处理器.
type FileOpsHandler struct{}

// Type 返回动作类型.
func (h *FileOpsHandler) Type() ActionType { return ActionFileOps }

// Name 返回动作名称.
func (h *FileOpsHandler) Name() string { return "File Operations" }

// Description 返回动作描述.
func (h *FileOpsHandler) Description() string {
	return "文件系统操作：复制、移动、删除、创建目录等"
}

// Validate 验证配置.
func (h *FileOpsHandler) Validate(config map[string]string) error {
	if _, ok := config["operation"]; !ok {
		return fmt.Errorf("file_ops requires 'operation' config")
	}
	return nil
}

// Execute 执行文件操作.
func (h *FileOpsHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	operation := ctx.Config["operation"]
	source := ctx.Config["source"]
	dest := ctx.Config["dest"]

	var err error
	var message string

	switch operation {
	case "copy":
		err = copyFile(source, dest)
		message = fmt.Sprintf("copied %s to %s", source, dest)
	case "move":
		err = os.Rename(source, dest)
		message = fmt.Sprintf("moved %s to %s", source, dest)
	case "delete":
		err = os.Remove(source)
		message = fmt.Sprintf("deleted %s", source)
	case "mkdir":
		err = os.MkdirAll(source, 0755)
		message = fmt.Sprintf("created directory %s", source)
	case "read":
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			err = readErr
		} else {
			return &ActionResult{
				Success: true,
				Output:  map[string]interface{}{"content": string(data)},
				Message: fmt.Sprintf("read %s", source),
			}, nil
		}
	case "write":
		content := ctx.Config["content"]
		err = os.WriteFile(source, []byte(content), 0644)
		message = fmt.Sprintf("wrote to %s", source)
	default:
		return nil, fmt.Errorf("unknown file operation: %s", operation)
	}

	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ActionResult{
		Success: true,
		Message: message,
	}, nil
}

// copyFile 复制文件.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// ========== 通知处理器 ==========

// NotificationHandler 通知动作处理器.
type NotificationHandler struct{}

// Type 返回动作类型.
func (h *NotificationHandler) Type() ActionType { return ActionNotification }

// Name 返回动作名称.
func (h *NotificationHandler) Name() string { return "Notification" }

// Description 返回动作描述.
func (h *NotificationHandler) Description() string {
	return "发送通知：邮件、Webhook、系统通知等"
}

// Validate 验证配置.
func (h *NotificationHandler) Validate(config map[string]string) error {
	if _, ok := config["channel"]; !ok {
		return fmt.Errorf("notification requires 'channel' config")
	}
	return nil
}

// Execute 执行通知.
func (h *NotificationHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	channel := ctx.Config["channel"]
	title := ctx.Config["title"]
	message := ctx.Config["message"]

	// 替换变量
	title = replaceVariables(title, ctx.Variables)
	message = replaceVariables(message, ctx.Variables)

	switch channel {
	case "webhook":
		return h.sendWebhook(ctx.Config, title, message)
	case "log":
		return &ActionResult{
			Success: true,
			Message: fmt.Sprintf("[%s] %s: %s", channel, title, message),
			Output: map[string]interface{}{
				"channel": channel,
				"title":   title,
				"message": message,
			},
		}, nil
	default:
		return &ActionResult{
			Success: true,
			Message: fmt.Sprintf("notification sent via %s", channel),
			Output: map[string]interface{}{
				"channel": channel,
				"title":   title,
				"message": message,
			},
		}, nil
	}
}

// sendWebhook 发送 Webhook 通知.
func (h *NotificationHandler) sendWebhook(config map[string]string, title, message string) (*ActionResult, error) {
	url := config["url"]
	if url == "" {
		return &ActionResult{
			Success: false,
			Error:   "webhook URL is required",
		}, nil
	}

	payload := map[string]string{
		"title":   title,
		"message": message,
	}
	jsonData, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	return &ActionResult{
		Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Message: fmt.Sprintf("webhook response: %d", resp.StatusCode),
		Output: map[string]interface{}{
			"status_code": resp.StatusCode,
		},
	}, nil
}

// ========== API 调用处理器 ==========

// APICallHandler API 调用动作处理器.
type APICallHandler struct{}

// Type 返回动作类型.
func (h *APICallHandler) Type() ActionType { return ActionAPICall }

// Name 返回动作名称.
func (h *APICallHandler) Name() string { return "API Call" }

// Description 返回动作描述.
func (h *APICallHandler) Description() string {
	return "调用外部 API：HTTP GET/POST/PUT/DELETE"
}

// Validate 验证配置.
func (h *APICallHandler) Validate(config map[string]string) error {
	if _, ok := config["url"]; !ok {
		return fmt.Errorf("api_call requires 'url' config")
	}
	return nil
}

// Execute 执行 API 调用.
func (h *APICallHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	url := ctx.Config["url"]
	method := ctx.Config["method"]
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	// 替换 URL 中的变量
	url = replaceVariables(url, ctx.Variables)

	var body io.Reader
	if bodyStr, ok := ctx.Config["body"]; ok {
		bodyStr = replaceVariables(bodyStr, ctx.Variables)
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 设置请求头
	if headers, ok := ctx.Config["headers"]; ok && headers != "" {
		var headerMap map[string]string
		if json.Unmarshal([]byte(headers), &headerMap) == nil {
			for k, v := range headerMap {
				req.Header.Set(k, v)
			}
		}
	}

	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	return &ActionResult{
		Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Message: fmt.Sprintf("API response: %d", resp.StatusCode),
		Output: map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(respBody),
		},
	}, nil
}

// ========== 容器操作处理器 ==========

// ContainerHandler 容器操作动作处理器.
type ContainerHandler struct{}

// Type 返回动作类型.
func (h *ContainerHandler) Type() ActionType { return ActionContainer }

// Name 返回动作名称.
func (h *ContainerHandler) Name() string { return "Container Operations" }

// Description 返回动作描述.
func (h *ContainerHandler) Description() string {
	return "容器操作：启动、停止、重启、查看状态等"
}

// Validate 验证配置.
func (h *ContainerHandler) Validate(config map[string]string) error {
	if _, ok := config["operation"]; !ok {
		return fmt.Errorf("container requires 'operation' config")
	}
	return nil
}

// Execute 执行容器操作.
func (h *ContainerHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	operation := ctx.Config["operation"]
	container := ctx.Config["container"]

	args := []string{}
	switch operation {
	case "start":
		args = []string{"start", container}
	case "stop":
		args = []string{"stop", container}
	case "restart":
		args = []string{"restart", container}
	case "status":
		args = []string{"inspect", "--format", "{{.State.Status}}", container}
	case "logs":
		args = []string{"logs", "--tail", "100", container}
	default:
		return nil, fmt.Errorf("unknown container operation: %s", operation)
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}, nil
	}

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("container %s: %s", container, operation),
		Output: map[string]interface{}{
			"output": strings.TrimSpace(string(output)),
		},
	}, nil
}

// ========== Shell 命令处理器 ==========

// ShellHandler Shell 命令动作处理器.
type ShellHandler struct{}

// Type 返回动作类型.
func (h *ShellHandler) Type() ActionType { return ActionShell }

// Name 返回动作名称.
func (h *ShellHandler) Name() string { return "Shell Command" }

// Description 返回动作描述.
func (h *ShellHandler) Description() string {
	return "执行 Shell 命令"
}

// Validate 验证配置.
func (h *ShellHandler) Validate(config map[string]string) error {
	if _, ok := config["command"]; !ok {
		return fmt.Errorf("shell requires 'command' config")
	}
	return nil
}

// Execute 执行 Shell 命令.
func (h *ShellHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	command := ctx.Config["command"]
	command = replaceVariables(command, ctx.Variables)

	workdir := ctx.Config["workdir"]
	if workdir != "" {
		workdir = replaceVariables(workdir, ctx.Variables)
	}

	cmd := exec.Command("sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}

	// 设置环境变量
	env := os.Environ()
	for k, v := range ctx.Variables {
		env = append(env, fmt.Sprintf("%s=%v", k, v))
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ActionResult{
			Success: false,
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
			Output: map[string]interface{}{
				"stdout": string(output),
			},
		}, nil
	}

	return &ActionResult{
		Success: true,
		Message: "command executed successfully",
		Output: map[string]interface{}{
			"stdout": strings.TrimSpace(string(output)),
		},
	}, nil
}

// ========== 数据转换处理器 ==========

// TransformHandler 数据转换动作处理器.
type TransformHandler struct{}

// Type 返回动作类型.
func (h *TransformHandler) Type() ActionType { return ActionTransform }

// Name 返回动作名称.
func (h *TransformHandler) Name() string { return "Data Transform" }

// Description 返回动作描述.
func (h *TransformHandler) Description() string {
	return "数据转换：JSON 处理、字符串操作、模板渲染等"
}

// Validate 验证配置.
func (h *TransformHandler) Validate(config map[string]string) error {
	if _, ok := config["operation"]; !ok {
		return fmt.Errorf("transform requires 'operation' config")
	}
	return nil
}

// Execute 执行数据转换.
func (h *TransformHandler) Execute(ctx *ActionContext) (*ActionResult, error) {
	operation := ctx.Config["operation"]

	switch operation {
	case "template":
		return h.executeTemplate(ctx)
	case "json_extract":
		return h.executeJSONExtract(ctx)
	case "string_replace":
		return h.executeStringReplace(ctx)
	default:
		return nil, fmt.Errorf("unknown transform operation: %s", operation)
	}
}

// executeTemplate 执行模板渲染.
func (h *TransformHandler) executeTemplate(ctx *ActionContext) (*ActionResult, error) {
	template := ctx.Config["template"]
	result := replaceVariables(template, ctx.Variables)

	return &ActionResult{
		Success: true,
		Output: map[string]interface{}{
			"result": result,
		},
	}, nil
}

// executeJSONExtract 执行 JSON 提取.
func (h *TransformHandler) executeJSONExtract(ctx *ActionContext) (*ActionResult, error) {
	source := ctx.Config["source"]
	path := ctx.Config["path"]

	// 简单实现：从输入中获取 JSON 并提取字段
	if inputVal, ok := ctx.Input[source]; ok {
		if jsonStr, ok := inputVal.(string); ok {
			var data map[string]interface{}
			if json.Unmarshal([]byte(jsonStr), &data) == nil {
				if val, exists := data[path]; exists {
					return &ActionResult{
						Success: true,
						Output: map[string]interface{}{
							"result": val,
						},
					}, nil
				}
			}
		}
	}

	return &ActionResult{
		Success: false,
		Error:   fmt.Sprintf("could not extract %s from %s", path, source),
	}, nil
}

// executeStringReplace 执行字符串替换.
func (h *TransformHandler) executeStringReplace(ctx *ActionContext) (*ActionResult, error) {
	input := ctx.Config["input"]
	old := ctx.Config["old"]
	new := ctx.Config["new"]

	result := strings.ReplaceAll(input, old, new)

	return &ActionResult{
		Success: true,
		Output: map[string]interface{}{
			"result": result,
		},
	}, nil
}

// replaceVariables 替换字符串中的变量引用.
func replaceVariables(s string, vars map[string]interface{}) string {
	result := s
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}
