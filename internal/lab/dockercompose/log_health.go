package dockercompose

import (
	"fmt"
	"sync"
	"time"
)

// LogManager 日志收集和查看管理器.
type LogManager struct {
	mu      sync.RWMutex
	logs    map[string][]LogEntry // key: projectName/serviceName
	maxSize int
}

// NewLogManager 创建日志管理器.
func NewLogManager(maxEntriesPerService int) *LogManager {
	if maxEntriesPerService <= 0 {
		maxEntriesPerService = 1000
	}
	return &LogManager{
		logs:    make(map[string][]LogEntry),
		maxSize: maxEntriesPerService,
	}
}

// AddLog 添加日志.
func (lm *LogManager) AddLog(projectName, serviceName, message string, level LogLevel) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	key := projectName + "/" + serviceName
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Service:   serviceName,
		Project:   projectName,
		Message:   message,
	}

	lm.logs[key] = append(lm.logs[key], entry)

	// 超过上限时丢弃旧日志
	if len(lm.logs[key]) > lm.maxSize {
		lm.logs[key] = lm.logs[key][len(lm.logs[key])-lm.maxSize:]
	}
}

// GetLogs 获取服务日志.
func (lm *LogManager) GetLogs(projectName, serviceName string, opts LogOptions) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	key := projectName + "/" + serviceName
	entries := lm.logs[key]

	// 时间过滤
	if !opts.Since.IsZero() {
		filtered := make([]LogEntry, 0)
		for _, e := range entries {
			if e.Timestamp.After(opts.Since) || e.Timestamp.Equal(opts.Since) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if !opts.Until.IsZero() {
		filtered := make([]LogEntry, 0)
		for _, e := range entries {
			if e.Timestamp.Before(opts.Until) || e.Timestamp.Equal(opts.Until) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// 级别过滤
	if opts.Level != "" {
		filtered := make([]LogEntry, 0)
		for _, e := range entries {
			if e.Level == opts.Level {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// 尾部限制
	if opts.Tail > 0 && opts.Tail < len(entries) {
		entries = entries[len(entries)-opts.Tail:]
	}

	return entries
}

// GetProjectLogs 获取项目所有服务日志.
func (lm *LogManager) GetProjectLogs(projectName string, opts LogOptions) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []LogEntry
	prefix := projectName + "/"
	for key, entries := range lm.logs {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, entries...)
		}
	}

	// 时间过滤
	if !opts.Since.IsZero() {
		filtered := make([]LogEntry, 0)
		for _, e := range result {
			if e.Timestamp.After(opts.Since) || e.Timestamp.Equal(opts.Since) {
				filtered = append(filtered, e)
			}
		}
		result = filtered
	}

	// 尾部限制
	if opts.Tail > 0 && opts.Tail < len(result) {
		result = result[len(result)-opts.Tail:]
	}

	return result
}

// ClearLogs 清除服务日志.
func (lm *LogManager) ClearLogs(projectName, serviceName string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.logs, projectName+"/"+serviceName)
}

// HealthManager 健康检查管理器.
type HealthManager struct {
	mu      sync.RWMutex
	status  map[string]*HealthServiceStatus // key: project/service
	checks  map[string]*HealthCheck         // key: project/service
	results map[string][]HealthCheckResult  // key: project/service
}

// NewHealthManager 创建健康检查管理器.
func NewHealthManager() *HealthManager {
	return &HealthManager{
		status:  make(map[string]*HealthServiceStatus),
		checks:  make(map[string]*HealthCheck),
		results: make(map[string][]HealthCheckResult),
	}
}

// RegisterCheck 注册健康检查.
func (hm *HealthManager) RegisterCheck(projectName, serviceName string, check *HealthCheck) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	key := projectName + "/" + serviceName
	hm.checks[key] = check
	hm.status[key] = &HealthServiceStatus{
		Project: projectName,
		Service: serviceName,
		Status:  HealthUnknown,
	}
}

// RunCheck 执行一次健康检查.
func (hm *HealthManager) RunCheck(projectName, serviceName string, healthy bool, message string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	key := projectName + "/" + serviceName
	now := time.Now()

	result := HealthCheckResult{
		Timestamp: now,
		Healthy:   healthy,
		Message:   message,
	}

	hm.results[key] = append(hm.results[key], result)
	// 保留最近 100 条
	if len(hm.results[key]) > 100 {
		hm.results[key] = hm.results[key][len(hm.results[key])-100:]
	}

	status, ok := hm.status[key]
	if !ok {
		status = &HealthServiceStatus{
			Project: projectName,
			Service: serviceName,
		}
		hm.status[key] = status
	}

	if healthy {
		status.Status = HealthHealthy
		status.FailCount = 0
	} else {
		status.FailCount++
		check := hm.checks[key]
		retries := 3
		if check != nil && check.Retries > 0 {
			retries = check.Retries
		}
		if status.FailCount >= retries {
			status.Status = HealthUnhealthy
		} else {
			status.Status = HealthStarting
		}
	}
	status.LastCheck = now
	status.Message = message
}

// GetStatus 获取服务健康状态.
func (hm *HealthManager) GetStatus(projectName, serviceName string) (*HealthServiceStatus, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	key := projectName + "/" + serviceName
	status, ok := hm.status[key]
	if !ok {
		return nil, fmt.Errorf("服务 %s/%s 未注册健康检查", projectName, serviceName)
	}
	return status, nil
}

// GetProjectHealth 获取项目所有服务健康状态.
func (hm *HealthManager) GetProjectHealth(projectName string) []*HealthServiceStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var result []*HealthServiceStatus
	prefix := projectName + "/"
	for key, status := range hm.status {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, status)
		}
	}
	return result
}

// GetResults 获取健康检查历史结果.
func (hm *HealthManager) GetResults(projectName, serviceName string, limit int) []HealthCheckResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	key := projectName + "/" + serviceName
	results := hm.results[key]
	if limit > 0 && limit < len(results) {
		results = results[len(results)-limit:]
	}
	return results
}

// UnregisterCheck 注销健康检查.
func (hm *HealthManager) UnregisterCheck(projectName, serviceName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	key := projectName + "/" + serviceName
	delete(hm.checks, key)
	delete(hm.status, key)
	delete(hm.results, key)
}
