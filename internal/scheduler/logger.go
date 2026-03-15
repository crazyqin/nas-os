package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// LogManager 日志管理器
type LogManager struct {
	executions map[string]*TaskExecution
	logs       map[string][]*TaskLog // executionID -> logs
	mu         sync.RWMutex
	storePath  string
	maxLogs    int
	retention  time.Duration
}

// NewLogManager 创建日志管理器
func NewLogManager(storePath string, maxLogs int, retention time.Duration) (*LogManager, error) {
	if maxLogs <= 0 {
		maxLogs = 10000
	}
	if retention <= 0 {
		retention = 7 * 24 * time.Hour // 默认保留 7 天
	}

	lm := &LogManager{
		executions: make(map[string]*TaskExecution),
		logs:       make(map[string][]*TaskLog),
		storePath:  storePath,
		maxLogs:    maxLogs,
		retention:  retention,
	}

	if err := lm.load(); err != nil {
		return nil, fmt.Errorf("加载日志失败: %w", err)
	}

	// 启动定期清理
	go lm.periodicCleanup()

	return lm, nil
}

// load 加载日志
func (lm *LogManager) load() error {
	if lm.storePath == "" {
		return nil
	}

	// 加载执行记录
	execData, err := os.ReadFile(lm.storePath + "/executions.json")
	if err == nil {
		var executions []*TaskExecution
		if err := json.Unmarshal(execData, &executions); err == nil {
			for _, exec := range executions {
				lm.executions[exec.ID] = exec
			}
		}
	}

	// 加载日志
	logData, err := os.ReadFile(lm.storePath + "/logs.json")
	if err == nil {
		var logs map[string][]*TaskLog
		if err := json.Unmarshal(logData, &logs); err == nil {
			lm.logs = logs
		}
	}

	return nil
}

// save 保存日志
func (lm *LogManager) save() error {
	if lm.storePath == "" {
		return nil
	}

	// 创建目录
	if err := os.MkdirAll(lm.storePath, 0755); err != nil {
		return err
	}

	// 保存执行记录
	executions := make([]*TaskExecution, 0, len(lm.executions))
	for _, exec := range lm.executions {
		executions = append(executions, exec)
	}
	execData, _ := json.MarshalIndent(executions, "", "  ")
	if err := os.WriteFile(lm.storePath+"/executions.json", execData, 0644); err != nil {
		return err
	}

	// 保存日志
	logData, _ := json.MarshalIndent(lm.logs, "", "  ")
	return os.WriteFile(lm.storePath+"/logs.json", logData, 0644)
}

// RecordExecution 记录执行
func (lm *LogManager) RecordExecution(execution *TaskExecution) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.executions[execution.ID] = execution
	return lm.save()
}

// UpdateExecution 更新执行记录
func (lm *LogManager) UpdateExecution(execution *TaskExecution) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.executions[execution.ID] = execution
	return lm.save()
}

// GetExecution 获取执行记录
func (lm *LogManager) GetExecution(id string) (*TaskExecution, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	exec, exists := lm.executions[id]
	if !exists {
		return nil, fmt.Errorf("执行记录不存在: %s", id)
	}
	return exec, nil
}

// GetExecutionsByTaskID 获取任务的所有执行记录
func (lm *LogManager) GetExecutionsByTaskID(taskID string) []*TaskExecution {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]*TaskExecution, 0)
	for _, exec := range lm.executions {
		if exec.TaskID == taskID {
			result = append(result, exec)
		}
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	return result
}

// QueryExecutions 查询执行记录
func (lm *LogManager) QueryExecutions(filter *ExecutionFilter) []*TaskExecution {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]*TaskExecution, 0)
	for _, exec := range lm.executions {
		if !lm.matchExecFilter(exec, filter) {
			continue
		}
		result = append(result, exec)
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	// 分页
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(result) {
		return []*TaskExecution{}
	}
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

func (lm *LogManager) matchExecFilter(exec *TaskExecution, filter *ExecutionFilter) bool {
	if filter == nil {
		return true
	}

	if filter.TaskID != "" && exec.TaskID != filter.TaskID {
		return false
	}
	if filter.Status != "" && exec.Status != filter.Status {
		return false
	}
	if filter.StartTime != nil && exec.StartedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && exec.StartedAt.After(*filter.EndTime) {
		return false
	}

	return true
}

// AddLog 添加日志
func (lm *LogManager) AddLog(log *TaskLog) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if log.ID == "" {
		log.ID = fmt.Sprintf("log_%d", time.Now().UnixNano())
	}
	log.Timestamp = time.Now()

	lm.logs[log.ExecutionID] = append(lm.logs[log.ExecutionID], log)
	return lm.save()
}

// GetLogs 获取执行日志
func (lm *LogManager) GetLogs(executionID string) []*TaskLog {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	logs := lm.logs[executionID]
	if logs == nil {
		return []*TaskLog{}
	}

	result := make([]*TaskLog, len(logs))
	copy(result, logs)
	return result
}

// GetTaskLogs 获取任务的所有日志
func (lm *LogManager) GetTaskLogs(taskID string) []*TaskLog {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]*TaskLog, 0)
	for execID, logs := range lm.logs {
		if exec, exists := lm.executions[execID]; exists && exec.TaskID == taskID {
			result = append(result, logs...)
		}
	}

	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// Log 记录日志
func (lm *LogManager) Log(executionID, taskID, level, message string, data map[string]interface{}) error {
	return lm.AddLog(&TaskLog{
		ExecutionID: executionID,
		TaskID:      taskID,
		Level:       level,
		Message:     message,
		Data:        data,
	})
}

// Debug 记录调试日志
func (lm *LogManager) Debug(executionID, taskID, message string, data ...map[string]interface{}) error {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	return lm.Log(executionID, taskID, "debug", message, d)
}

// Info 记录信息日志
func (lm *LogManager) Info(executionID, taskID, message string, data ...map[string]interface{}) error {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	return lm.Log(executionID, taskID, "info", message, d)
}

// Warn 记录警告日志
func (lm *LogManager) Warn(executionID, taskID, message string, data ...map[string]interface{}) error {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	return lm.Log(executionID, taskID, "warn", message, d)
}

// Error 记录错误日志
func (lm *LogManager) Error(executionID, taskID, message string, data ...map[string]interface{}) error {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	return lm.Log(executionID, taskID, "error", message, d)
}

// DeleteExecution 删除执行记录
func (lm *LogManager) DeleteExecution(id string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	delete(lm.executions, id)
	delete(lm.logs, id)

	return lm.save()
}

// Clear 清空所有日志
func (lm *LogManager) Clear() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.executions = make(map[string]*TaskExecution)
	lm.logs = make(map[string][]*TaskLog)

	return lm.save()
}

// cleanup 清理过期日志
func (lm *LogManager) cleanup() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	cutoff := time.Now().Add(-lm.retention)

	// 清理过期的执行记录
	for id, exec := range lm.executions {
		if exec.StartedAt.Before(cutoff) {
			delete(lm.executions, id)
			delete(lm.logs, id)
		}
	}

	// 如果日志数量超限，删除最旧的
	if len(lm.executions) > lm.maxLogs {
		// 收集并排序
		execList := make([]*TaskExecution, 0, len(lm.executions))
		for _, exec := range lm.executions {
			execList = append(execList, exec)
		}
		sort.Slice(execList, func(i, j int) bool {
			return execList[i].StartedAt.After(execList[j].StartedAt)
		})

		// 删除多余的
		for i := lm.maxLogs; i < len(execList); i++ {
			delete(lm.executions, execList[i].ID)
			delete(lm.logs, execList[i].ID)
		}
	}
}

// periodicCleanup 定期清理
func (lm *LogManager) periodicCleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		lm.cleanup()
		lm.save()
	}
}

// Stats 获取日志统计
func (lm *LogManager) Stats() *LogStats {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats := &LogStats{
		TotalExecutions: len(lm.executions),
		TotalLogs:       0,
		StatusCounts:    make(map[ExecutionStatus]int),
	}

	for _, exec := range lm.executions {
		stats.StatusCounts[exec.Status]++
	}

	for _, logs := range lm.logs {
		stats.TotalLogs += len(logs)
	}

	return stats
}

// LogStats 日志统计
type LogStats struct {
	TotalExecutions int                     `json:"totalExecutions"`
	TotalLogs       int                     `json:"totalLogs"`
	StatusCounts    map[ExecutionStatus]int `json:"statusCounts"`
}

// ExportLogs 导出日志
func (lm *LogManager) ExportLogs(executionID string, format string) ([]byte, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	exec, exists := lm.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("执行记录不存在")
	}

	logs := lm.logs[executionID]

	export := map[string]interface{}{
		"execution": exec,
		"logs":      logs,
	}

	switch format {
	case "json":
		return json.MarshalIndent(export, "", "  ")
	default:
		return json.MarshalIndent(export, "", "  ")
	}
}

// GetRecentExecutions 获取最近的执行记录
func (lm *LogManager) GetRecentExecutions(limit int) []*TaskExecution {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// 收集并排序
	execList := make([]*TaskExecution, 0, len(lm.executions))
	for _, exec := range lm.executions {
		execList = append(execList, exec)
	}
	sort.Slice(execList, func(i, j int) bool {
		return execList[i].StartedAt.After(execList[j].StartedAt)
	})

	if limit > 0 && len(execList) > limit {
		execList = execList[:limit]
	}

	return execList
}

// GetTodayExecutions 获取今日执行记录
func (lm *LogManager) GetTodayExecutions() []*TaskExecution {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	today := time.Now().Truncate(24 * time.Hour)
	result := make([]*TaskExecution, 0)

	for _, exec := range lm.executions {
		if exec.StartedAt.After(today) || exec.StartedAt.Equal(today) {
			result = append(result, exec)
		}
	}

	return result
}

// WriteExecutionLogFile 写入执行日志文件
func (lm *LogManager) WriteExecutionLogFile(executionID string) (string, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	exec, exists := lm.executions[executionID]
	if !exists {
		return "", fmt.Errorf("执行记录不存在")
	}

	logs := lm.logs[executionID]

	// 创建日志文件目录
	logDir := filepath.Join(lm.storePath, "files")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}

	// 生成文件名
	filename := fmt.Sprintf("%s_%s.log", exec.TaskID, executionID)
	filepath := filepath.Join(logDir, filename)

	// 构建日志内容
	var content string
	content += fmt.Sprintf("任务: %s (%s)\n", exec.TaskName, exec.TaskID)
	content += fmt.Sprintf("执行ID: %s\n", executionID)
	content += fmt.Sprintf("开始时间: %s\n", exec.StartedAt.Format("2006-01-02 15:04:05"))
	if exec.CompletedAt != nil {
		content += fmt.Sprintf("结束时间: %s\n", exec.CompletedAt.Format("2006-01-02 15:04:05"))
		content += fmt.Sprintf("耗时: %s\n", exec.Duration)
	}
	content += fmt.Sprintf("状态: %s\n", exec.Status)
	if exec.Error != "" {
		content += fmt.Sprintf("错误: %s\n", exec.Error)
	}
	content += "\n--- 日志 ---\n\n"

	for _, log := range logs {
		content += fmt.Sprintf("[%s] %s %s\n", log.Timestamp.Format("15:04:05"), log.Level, log.Message)
	}

	// 写入文件
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", err
	}

	return filepath, nil
}
