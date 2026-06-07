// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryExecutionLogger 内存执行日志器.
type MemoryExecutionLogger struct {
	mu      sync.RWMutex
	entries []*LogEntry
	maxSize int
}

// NewMemoryExecutionLogger 创建内存执行日志器.
func NewMemoryExecutionLogger(maxSize int) *MemoryExecutionLogger {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &MemoryExecutionLogger{
		entries: make([]*LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Log 记录日志.
func (l *MemoryExecutionLogger) Log(level LogLevel, executionID, nodeID, message string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := &LogEntry{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       level,
		Message:     message,
		Fields:      fields,
		Timestamp:   time.Now(),
	}

	// 容量管理
	if len(l.entries) >= l.maxSize {
		// 移除最旧的 10%
		removeCount := l.maxSize / 10
		if removeCount < 1 {
			removeCount = 1
		}
		l.entries = l.entries[removeCount:]
	}

	l.entries = append(l.entries, entry)
}

// LogStart 记录执行开始.
func (l *MemoryExecutionLogger) LogStart(executionID, workflowID string) {
	l.Log(LogInfo, executionID, "", "execution started", map[string]interface{}{
		"workflow_id": workflowID,
	})
}

// LogEnd 记录执行结束.
func (l *MemoryExecutionLogger) LogEnd(executionID string, status ExecutionStatus, err error) {
	fields := map[string]interface{}{
		"status": string(status),
	}
	if err != nil {
		fields["error"] = err.Error()
	}

	level := LogInfo
	if status == ExecFailed {
		level = LogError
	}

	l.Log(level, executionID, "", "execution finished", fields)
}

// LogStep 记录步骤执行.
func (l *MemoryExecutionLogger) LogStep(executionID, nodeID string, status ExecutionStatus, input, output map[string]interface{}, err error) {
	fields := map[string]interface{}{
		"status": string(status),
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	if input != nil {
		fields["input_keys"] = mapKeys(input)
	}
	if output != nil {
		fields["output_keys"] = mapKeys(output)
	}

	level := LogInfo
	if status == ExecFailed {
		level = LogError
	}

	l.Log(level, executionID, nodeID, "step executed", fields)
}

// GetLogs 获取日志.
func (l *MemoryExecutionLogger) GetLogs(executionID string, limit int) ([]*LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*LogEntry, 0)
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].ExecutionID == executionID {
			result = append(result, l.entries[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

// GetAllLogs 获取所有日志.
func (l *MemoryExecutionLogger) GetAllLogs(limit int) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}

	// 返回最新的日志
	start := len(l.entries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*LogEntry, limit)
	copy(result, l.entries[start:])
	return result
}

// GetLogsByLevel 按级别获取日志.
func (l *MemoryExecutionLogger) GetLogsByLevel(level LogLevel, limit int) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*LogEntry, 0)
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].Level == level {
			result = append(result, l.entries[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// GetLogsByNode 按节点获取日志.
func (l *MemoryExecutionLogger) GetLogsByNode(nodeID string, limit int) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*LogEntry, 0)
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].NodeID == nodeID {
			result = append(result, l.entries[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// Clear 清空日志.
func (l *MemoryExecutionLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*LogEntry, 0, l.maxSize)
}

// Count 获取日志数量.
func (l *MemoryExecutionLogger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// mapKeys 获取 map 的所有 key.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
