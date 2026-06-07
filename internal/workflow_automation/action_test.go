// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== FileOpsHandler 测试 ==========

func TestFileOpsHandler(t *testing.T) {
	handler := &FileOpsHandler{}

	assert.Equal(t, ActionFileOps, handler.Type())
	assert.Equal(t, "File Operations", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestFileOpsHandlerValidate(t *testing.T) {
	handler := &FileOpsHandler{}

	// 有效配置
	err := handler.Validate(map[string]string{"operation": "copy"})
	assert.NoError(t, err)

	// 缺少 operation
	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

func TestFileOpsHandlerExecuteMkdir(t *testing.T) {
	handler := &FileOpsHandler{}
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "test-dir")

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "mkdir",
			"source":    target,
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证目录创建
	_, err = os.Stat(target)
	assert.NoError(t, err)
}

func TestFileOpsHandlerExecuteWriteRead(t *testing.T) {
	handler := &FileOpsHandler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// 写入
	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "write",
			"source":    filePath,
			"content":   "hello world",
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 读取
	ctx = &ActionContext{
		Config: map[string]string{
			"operation": "read",
			"source":    filePath,
		},
		Variables: make(map[string]interface{}),
	}

	result, err = handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "hello world", result.Output["content"])
}

func TestFileOpsHandlerExecuteCopy(t *testing.T) {
	handler := &FileOpsHandler{}
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	os.WriteFile(src, []byte("test content"), 0644)

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "copy",
			"source":    src,
			"dest":      dst,
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证复制
	data, _ := os.ReadFile(dst)
	assert.Equal(t, "test content", string(data))
}

func TestFileOpsHandlerExecuteDelete(t *testing.T) {
	handler := &FileOpsHandler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "to-delete.txt")

	os.WriteFile(filePath, []byte("delete me"), 0644)

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "delete",
			"source":    filePath,
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证删除
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestFileOpsHandlerExecuteUnknown(t *testing.T) {
	handler := &FileOpsHandler{}

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "unknown_op",
			"source":    "/tmp/test",
		},
		Variables: make(map[string]interface{}),
	}

	_, err := handler.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown file operation")
}

// ========== NotificationHandler 测试 ==========

func TestNotificationHandler(t *testing.T) {
	handler := &NotificationHandler{}

	assert.Equal(t, ActionNotification, handler.Type())
	assert.Equal(t, "Notification", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestNotificationHandlerValidate(t *testing.T) {
	handler := &NotificationHandler{}

	err := handler.Validate(map[string]string{"channel": "log"})
	assert.NoError(t, err)

	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

func TestNotificationHandlerExecuteLog(t *testing.T) {
	handler := &NotificationHandler{}

	ctx := &ActionContext{
		Config: map[string]string{
			"channel": "log",
			"title":   "Test Alert",
			"message": "Something happened",
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "Test Alert")
}

// ========== APICallHandler 测试 ==========

func TestAPICallHandler(t *testing.T) {
	handler := &APICallHandler{}

	assert.Equal(t, ActionAPICall, handler.Type())
	assert.Equal(t, "API Call", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestAPICallHandlerValidate(t *testing.T) {
	handler := &APICallHandler{}

	err := handler.Validate(map[string]string{"url": "http://example.com"})
	assert.NoError(t, err)

	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

// ========== ContainerHandler 测试 ==========

func TestContainerHandler(t *testing.T) {
	handler := &ContainerHandler{}

	assert.Equal(t, ActionContainer, handler.Type())
	assert.Equal(t, "Container Operations", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestContainerHandlerValidate(t *testing.T) {
	handler := &ContainerHandler{}

	err := handler.Validate(map[string]string{"operation": "start"})
	assert.NoError(t, err)

	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

func TestContainerHandlerExecuteUnknown(t *testing.T) {
	handler := &ContainerHandler{}

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "unknown",
			"container": "test",
		},
		Variables: make(map[string]interface{}),
	}

	_, err := handler.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown container operation")
}

// ========== ShellHandler 测试 ==========

func TestShellHandler(t *testing.T) {
	handler := &ShellHandler{}

	assert.Equal(t, ActionShell, handler.Type())
	assert.Equal(t, "Shell Command", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestShellHandlerValidate(t *testing.T) {
	handler := &ShellHandler{}

	err := handler.Validate(map[string]string{"command": "echo hello"})
	assert.NoError(t, err)

	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

func TestShellHandlerExecute(t *testing.T) {
	handler := &ShellHandler{}

	ctx := &ActionContext{
		Config:    map[string]string{"command": "echo hello"},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "hello", result.Output["stdout"])
}

func TestShellHandlerExecuteWithVariables(t *testing.T) {
	handler := &ShellHandler{}

	ctx := &ActionContext{
		Config:    map[string]string{"command": "echo {{name}}"},
		Variables: map[string]interface{}{"name": "world"},
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "world", result.Output["stdout"])
}

func TestShellHandlerExecuteError(t *testing.T) {
	handler := &ShellHandler{}

	ctx := &ActionContext{
		Config:    map[string]string{"command": "false"},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.False(t, result.Success)
}

// ========== TransformHandler 测试 ==========

func TestTransformHandler(t *testing.T) {
	handler := &TransformHandler{}

	assert.Equal(t, ActionTransform, handler.Type())
	assert.Equal(t, "Data Transform", handler.Name())
	assert.NotEmpty(t, handler.Description())
}

func TestTransformHandlerValidate(t *testing.T) {
	handler := &TransformHandler{}

	err := handler.Validate(map[string]string{"operation": "template"})
	assert.NoError(t, err)

	err = handler.Validate(map[string]string{})
	assert.Error(t, err)
}

func TestTransformHandlerExecuteTemplate(t *testing.T) {
	handler := &TransformHandler{}

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "template",
			"template":  "Hello {{name}}, you are {{age}} years old",
		},
		Variables: map[string]interface{}{
			"name": "Alice",
			"age":  30,
		},
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Hello Alice, you are 30 years old", result.Output["result"])
}

func TestTransformHandlerExecuteStringReplace(t *testing.T) {
	handler := &TransformHandler{}

	ctx := &ActionContext{
		Config: map[string]string{
			"operation": "string_replace",
			"input":     "hello world",
			"old":       "world",
			"new":       "golang",
		},
		Variables: make(map[string]interface{}),
	}

	result, err := handler.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "hello golang", result.Output["result"])
}

func TestTransformHandlerExecuteUnknown(t *testing.T) {
	handler := &TransformHandler{}

	ctx := &ActionContext{
		Config:    map[string]string{"operation": "unknown"},
		Variables: make(map[string]interface{}),
	}

	_, err := handler.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transform operation")
}

// ========== 辅助函数测试 ==========

func TestReplaceVariables(t *testing.T) {
	vars := map[string]interface{}{
		"name": "test",
		"age":  25,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"hello {{name}}", "hello test"},
		{"{{name}} is {{age}}", "test is 25"},
		{"no vars", "no vars"},
		{"{{missing}}", "{{missing}}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := replaceVariables(tt.input, vars)
			assert.Equal(t, tt.expected, result)
		})
	}
}
