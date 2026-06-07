// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryExecutionLogger(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)
	assert.NotNil(t, logger)
	assert.Equal(t, 100, logger.maxSize)
	assert.Equal(t, 0, logger.Count())
}

func TestNewMemoryExecutionLoggerDefaultSize(t *testing.T) {
	logger := NewMemoryExecutionLogger(0)
	assert.Equal(t, 10000, logger.maxSize)
}

func TestMemoryExecutionLoggerLog(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.Log(LogInfo, "exec-1", "node-1", "test message", map[string]interface{}{
		"key": "value",
	})

	assert.Equal(t, 1, logger.Count())

	logs, err := logger.GetLogs("exec-1", 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)

	assert.Equal(t, "exec-1", logs[0].ExecutionID)
	assert.Equal(t, "node-1", logs[0].NodeID)
	assert.Equal(t, LogInfo, logs[0].Level)
	assert.Equal(t, "test message", logs[0].Message)
	assert.Equal(t, "value", logs[0].Fields["key"])
	assert.False(t, logs[0].Timestamp.IsZero())
	assert.NotEmpty(t, logs[0].ID)
}

func TestMemoryExecutionLoggerLogStart(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.LogStart("exec-1", "wf-1")

	logs, _ := logger.GetLogs("exec-1", 10)
	require.Len(t, logs, 1)
	assert.Equal(t, LogInfo, logs[0].Level)
	assert.Equal(t, "execution started", logs[0].Message)
	assert.Equal(t, "wf-1", logs[0].Fields["workflow_id"])
}

func TestMemoryExecutionLoggerLogEnd(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.LogEnd("exec-1", ExecSuccess, nil)

	logs, _ := logger.GetLogs("exec-1", 10)
	require.Len(t, logs, 1)
	assert.Equal(t, LogInfo, logs[0].Level)
	assert.Equal(t, "execution finished", logs[0].Message)
	assert.Equal(t, "success", logs[0].Fields["status"])
}

func TestMemoryExecutionLoggerLogEndWithError(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.LogEnd("exec-1", ExecFailed, assert.AnError)

	logs, _ := logger.GetLogs("exec-1", 10)
	require.Len(t, logs, 1)
	assert.Equal(t, LogError, logs[0].Level)
	assert.Equal(t, "failed", logs[0].Fields["status"])
	assert.NotNil(t, logs[0].Fields["error"])
}

func TestMemoryExecutionLoggerLogStep(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	input := map[string]interface{}{"key": "value"}
	output := map[string]interface{}{"result": "ok"}

	logger.LogStep("exec-1", "node-1", ExecSuccess, input, output, nil)

	logs, _ := logger.GetLogs("exec-1", 10)
	require.Len(t, logs, 1)
	assert.Equal(t, "step executed", logs[0].Message)
	assert.Equal(t, "success", logs[0].Fields["status"])
}

func TestMemoryExecutionLoggerLogStepFailed(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.LogStep("exec-1", "node-1", ExecFailed, nil, nil, assert.AnError)

	logs, _ := logger.GetLogs("exec-1", 10)
	require.Len(t, logs, 1)
	assert.Equal(t, LogError, logs[0].Level)
	assert.Equal(t, "failed", logs[0].Fields["status"])
}

func TestMemoryExecutionLoggerGetLogsEmpty(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logs, err := logger.GetLogs("nonexistent", 10)
	require.NoError(t, err)
	assert.Len(t, logs, 0)
}

func TestMemoryExecutionLoggerGetLogsMultiple(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	for i := 0; i < 5; i++ {
		logger.Log(LogInfo, "exec-1", "", "message", nil)
	}
	for i := 0; i < 3; i++ {
		logger.Log(LogInfo, "exec-2", "", "message", nil)
	}

	// 获取 exec-1 的日志
	logs, _ := logger.GetLogs("exec-1", 10)
	assert.Len(t, logs, 5)

	// 获取 exec-2 的日志
	logs, _ = logger.GetLogs("exec-2", 10)
	assert.Len(t, logs, 3)

	// 限制数量
	logs, _ = logger.GetLogs("exec-1", 2)
	assert.Len(t, logs, 2)
}

func TestMemoryExecutionLoggerGetAllLogs(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	for i := 0; i < 10; i++ {
		logger.Log(LogInfo, "exec-1", "", "message", nil)
	}

	all := logger.GetAllLogs(0)
	assert.Len(t, all, 10)

	limited := logger.GetAllLogs(5)
	assert.Len(t, limited, 5)
}

func TestMemoryExecutionLoggerGetLogsByLevel(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.Log(LogInfo, "exec-1", "", "info", nil)
	logger.Log(LogError, "exec-1", "", "error", nil)
	logger.Log(LogDebug, "exec-1", "", "debug", nil)
	logger.Log(LogError, "exec-1", "", "error2", nil)

	errors := logger.GetLogsByLevel(LogError, 0)
	assert.Len(t, errors, 2)

	infos := logger.GetLogsByLevel(LogInfo, 0)
	assert.Len(t, infos, 1)

	debugs := logger.GetLogsByLevel(LogDebug, 0)
	assert.Len(t, debugs, 1)
}

func TestMemoryExecutionLoggerGetLogsByNode(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	logger.Log(LogInfo, "exec-1", "node-1", "msg", nil)
	logger.Log(LogInfo, "exec-1", "node-2", "msg", nil)
	logger.Log(LogInfo, "exec-1", "node-1", "msg", nil)

	node1Logs := logger.GetLogsByNode("node-1", 0)
	assert.Len(t, node1Logs, 2)

	node2Logs := logger.GetLogsByNode("node-2", 0)
	assert.Len(t, node2Logs, 1)
}

func TestMemoryExecutionLoggerClear(t *testing.T) {
	logger := NewMemoryExecutionLogger(100)

	for i := 0; i < 10; i++ {
		logger.Log(LogInfo, "exec-1", "", "msg", nil)
	}

	assert.Equal(t, 10, logger.Count())

	logger.Clear()
	assert.Equal(t, 0, logger.Count())
}

func TestMemoryExecutionLoggerMaxSize(t *testing.T) {
	// 设置小容量
	logger := NewMemoryExecutionLogger(5)

	// 写入超过容量
	for i := 0; i < 10; i++ {
		logger.Log(LogInfo, "exec-1", "", "msg", nil)
	}

	// 应该进行清理（保留 90%）
	count := logger.Count()
	assert.LessOrEqual(t, count, 5)
	assert.Greater(t, count, 0)
}

func TestMapKeys(t *testing.T) {
	m := map[string]interface{}{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	keys := mapKeys(m)
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")
	assert.Contains(t, keys, "c")
}

func TestMapKeysEmpty(t *testing.T) {
	m := map[string]interface{}{}
	keys := mapKeys(m)
	assert.Len(t, keys, 0)
}
