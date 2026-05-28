// Package serverless 提供边缘 Serverless 函数引擎功能.
package serverless

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Engine Serverless 函数引擎.
type Engine struct {
	functions   map[string]*Function
	invocations map[string]*Invocation
	versions    map[string][]*FunctionVersion
	logs        []*FunctionLog
	config      EngineConfig
	mu          sync.RWMutex
	running     bool
	semaphore   chan struct{}
}

// NewEngine 创建函数引擎.
func NewEngine(config *EngineConfig) *Engine {
	cfg := DefaultEngineConfig()
	if config != nil {
		cfg = *config
	}

	return &Engine{
		functions:   make(map[string]*Function),
		invocations: make(map[string]*Invocation),
		versions:    make(map[string][]*FunctionVersion),
		logs:        make([]*FunctionLog, 0),
		config:      cfg,
		semaphore:   make(chan struct{}, cfg.MaxConcurrentInvocations),
	}
}

// Start 启动引擎.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("引擎已在运行")
	}

	e.running = true
	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.running = false
}

// IsRunning 检查引擎是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.running
}

// ========== 函数管理 ==========

// CreateFunction 创建函数.
func (e *Engine) CreateFunction(fn *Function) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if fn.ID == "" {
		fn.ID = generateID("fn")
	}

	if fn.Name == "" {
		return fmt.Errorf("函数名称不能为空")
	}

	// 检查名称唯一性
	for _, existing := range e.functions {
		if existing.Name == fn.Name {
			return fmt.Errorf("函数名称已存在: %s", fn.Name)
		}
	}

	if fn.Runtime == "" {
		return fmt.Errorf("运行时不能为空")
	}

	if !isValidRuntime(fn.Runtime) {
		return fmt.Errorf("不支持的运行时: %s", fn.Runtime)
	}

	// 检查函数数量限制
	if len(e.functions) >= e.config.MaxFunctions {
		return fmt.Errorf("已达到最大函数数量限制: %d", e.config.MaxFunctions)
	}

	// 应用默认配置
	if fn.Config.CPUMilli == 0 && fn.Config.MemoryMB == 0 {
		fn.Config = DefaultFunctionConfig()
	}

	fn.DeployStatus = DeployStatusDraft
	fn.Enabled = true
	fn.Version = "1.0.0"
	fn.CreatedAt = time.Now()
	fn.UpdatedAt = time.Now()

	e.functions[fn.ID] = fn

	// 创建初始版本
	e.addVersion(fn, "initial deployment")

	return nil
}

// UpdateFunction 更新函数.
func (e *Engine) UpdateFunction(fn *Function) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.functions[fn.ID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", fn.ID)
	}

	// 检查名称唯一性（排除自身）
	for _, f := range e.functions {
		if f.ID != fn.ID && f.Name == fn.Name {
			return fmt.Errorf("函数名称已存在: %s", fn.Name)
		}
	}

	// 保留不可变字段
	fn.CreatedAt = existing.CreatedAt
	fn.InvokeCount = existing.InvokeCount
	fn.ErrorCount = existing.ErrorCount
	fn.UpdatedAt = time.Now()

	e.functions[fn.ID] = fn

	// 版本递增
	parts := strings.Split(fn.Version, ".")
	if len(parts) == 3 {
		minor := parseInt(parts[2]) + 1
		fn.Version = fmt.Sprintf("%s.%s.%d", parts[0], parts[1], minor)
	}

	e.addVersion(fn, "function updated")

	return nil
}

// DeleteFunction 删除函数.
func (e *Engine) DeleteFunction(functionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.functions[functionID]; !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	delete(e.functions, functionID)
	delete(e.versions, functionID)

	// 清理相关调用记录
	for id, inv := range e.invocations {
		if inv.FunctionID == functionID {
			delete(e.invocations, id)
		}
	}

	return nil
}

// GetFunction 获取函数.
func (e *Engine) GetFunction(functionID string) (*Function, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return nil, fmt.Errorf("函数不存在: %s", functionID)
	}

	return fn, nil
}

// ListFunctions 列出函数.
func (e *Engine) ListFunctions(filter *FunctionFilter) []*Function {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Function, 0)
	for _, fn := range e.functions {
		if !matchFunctionFilter(fn, filter) {
			continue
		}
		result = append(result, fn)
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 分页
	if filter != nil && filter.PageSize > 0 {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		pageSize := filter.PageSize
		start := (page - 1) * pageSize
		end := start + pageSize

		if start < len(result) {
			if end > len(result) {
				end = len(result)
			}
			result = result[start:end]
		} else {
			result = []*Function{}
		}
	}

	return result
}

// EnableFunction 启用函数.
func (e *Engine) EnableFunction(functionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	fn.Enabled = true
	fn.UpdatedAt = time.Now()

	return nil
}

// DisableFunction 禁用函数.
func (e *Engine) DisableFunction(functionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	fn.Enabled = false
	fn.DeployStatus = DeployStatusStopped
	fn.UpdatedAt = time.Now()

	return nil
}

// ========== 部署管理 ==========

// DeployFunction 部署函数.
func (e *Engine) DeployFunction(functionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	if fn.Code == "" {
		return fmt.Errorf("函数代码为空")
	}

	fn.DeployStatus = DeployStatusDeploying
	fn.UpdatedAt = time.Now()

	// 模拟部署过程
	fn.DeployStatus = DeployStatusDeployed
	fn.Enabled = true

	return nil
}

// UndeployFunction 取消部署.
func (e *Engine) UndeployFunction(functionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	fn.DeployStatus = DeployStatusStopped
	fn.Enabled = false
	fn.UpdatedAt = time.Now()

	return nil
}

// ========== 触发器管理 ==========

// AddTrigger 添加触发器.
func (e *Engine) AddTrigger(trigger *Trigger) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[trigger.FunctionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", trigger.FunctionID)
	}

	if trigger.ID == "" {
		trigger.ID = generateID("tr")
	}

	if !isValidTriggerType(trigger.Type) {
		return fmt.Errorf("不支持的触发器类型: %s", trigger.Type)
	}

	trigger.Enabled = true
	trigger.CreatedAt = time.Now()

	fn.Triggers = append(fn.Triggers, trigger)
	fn.UpdatedAt = time.Now()

	return nil
}

// RemoveTrigger 移除触发器.
func (e *Engine) RemoveTrigger(functionID, triggerID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	for i, t := range fn.Triggers {
		if t.ID == triggerID {
			fn.Triggers = append(fn.Triggers[:i], fn.Triggers[i+1:]...)
			fn.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("触发器不存在: %s", triggerID)
}

// GetTriggers 获取函数的触发器.
func (e *Engine) GetTriggers(functionID string) ([]*Trigger, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return nil, fmt.Errorf("函数不存在: %s", functionID)
	}

	return fn.Triggers, nil
}

// ========== 函数调用 ==========

// InvokeSync 同步调用函数.
func (e *Engine) InvokeSync(ctx context.Context, functionID string, input map[string]interface{}) (*InvokeResponse, error) {
	return e.invoke(ctx, functionID, InvocationSync, input, "api")
}

// InvokeAsync 异步调用函数.
func (e *Engine) InvokeAsync(ctx context.Context, functionID string, input map[string]interface{}) (*InvokeResponse, error) {
	return e.invoke(ctx, functionID, InvocationAsync, input, "api")
}

// invoke 内部调用实现.
func (e *Engine) invoke(ctx context.Context, functionID string, mode InvocationMode, input map[string]interface{}, triggeredBy string) (*InvokeResponse, error) {
	// 获取信号量
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	default:
		return nil, fmt.Errorf("并发调用数已达上限")
	}

	e.mu.RLock()
	fn, exists := e.functions[functionID]
	if !exists {
		e.mu.RUnlock()
		return nil, fmt.Errorf("函数不存在: %s", functionID)
	}

	if !fn.Enabled {
		e.mu.RUnlock()
		return nil, fmt.Errorf("函数已禁用: %s", functionID)
	}

	if fn.DeployStatus != DeployStatusDeployed {
		e.mu.RUnlock()
		return nil, fmt.Errorf("函数未部署: %s", functionID)
	}
	e.mu.RUnlock()

	// 创建调用记录
	invocationID := generateID("inv")
	now := time.Now()

	invocation := &Invocation{
		ID:           invocationID,
		FunctionID:   functionID,
		FunctionName: fn.Name,
		Version:      fn.Version,
		Mode:         mode,
		Status:       InvocationStatusRunning,
		Request:      input,
		TriggeredBy:  triggeredBy,
		StartedAt:    now,
	}

	e.mu.Lock()
	e.invocations[invocationID] = invocation
	fn.LastInvokeAt = &now
	fn.InvokeCount++
	e.mu.Unlock()

	// 执行函数
	response := e.executeFunction(ctx, fn, invocation, input)

	// 更新调用记录
	completedAt := time.Now()
	invocation.CompletedAt = &completedAt
	invocation.Duration = completedAt.Sub(now)
	invocation.Status = response.Status

	if response.Error != "" {
		invocation.Error = response.Error
		invocation.Status = InvocationStatusFailed
		e.mu.Lock()
		fn.ErrorCount++
		e.mu.Unlock()
	}

	if response.Output != nil {
		invocation.Response = response.Output
	}

	response.InvocationID = invocationID
	response.Duration = invocation.Duration

	// 记录日志
	e.addLog(LogLevelInfo, invocationID, functionID, fmt.Sprintf("函数调用完成: %s", response.Status))

	return response, nil
}

// executeFunction 执行函数（模拟）.
func (e *Engine) executeFunction(ctx context.Context, fn *Function, inv *Invocation, input map[string]interface{}) *InvokeResponse {
	// 检查超时
	timeout := time.Duration(fn.Config.TimeoutS) * time.Second
	if timeout == 0 {
		timeout = time.Duration(e.config.DefaultTimeoutS) * time.Second
	}

	done := make(chan *InvokeResponse, 1)
	go func() {
		// 模拟函数执行
		time.Sleep(10 * time.Millisecond)
		done <- &InvokeResponse{
			Status: InvocationStatusSuccess,
			Output: map[string]interface{}{
				"result":    "ok",
				"function":  fn.Name,
				"runtime":   string(fn.Runtime),
				"timestamp": time.Now().Unix(),
			},
		}
	}()

	select {
	case resp := <-done:
		return resp
	case <-time.After(timeout):
		return &InvokeResponse{
			Status: InvocationStatusTimeout,
			Error:  fmt.Sprintf("函数执行超时: %v", timeout),
		}
	case <-ctx.Done():
		return &InvokeResponse{
			Status: InvocationStatusFailed,
			Error:  "上下文已取消",
		}
	}
}

// GetInvocation 获取调用记录.
func (e *Engine) GetInvocation(invocationID string) (*Invocation, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	inv, exists := e.invocations[invocationID]
	if !exists {
		return nil, fmt.Errorf("调用记录不存在: %s", invocationID)
	}

	return inv, nil
}

// ListInvocations 列出调用记录.
func (e *Engine) ListInvocations(filter *InvocationFilter) []*Invocation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Invocation, 0)
	for _, inv := range e.invocations {
		if !matchInvocationFilter(inv, filter) {
			continue
		}
		result = append(result, inv)
	}

	// 按开始时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	// 分页
	if filter != nil && filter.PageSize > 0 {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		pageSize := filter.PageSize
		start := (page - 1) * pageSize
		end := start + pageSize

		if start < len(result) {
			if end > len(result) {
				end = len(result)
			}
			result = result[start:end]
		} else {
			result = []*Invocation{}
		}
	}

	return result
}

// ========== 版本管理 ==========

// GetVersions 获取函数版本列表.
func (e *Engine) GetVersions(functionID string) ([]*FunctionVersion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.functions[functionID]; !exists {
		return nil, fmt.Errorf("函数不存在: %s", functionID)
	}

	versions := e.versions[functionID]
	if versions == nil {
		return []*FunctionVersion{}, nil
	}

	return versions, nil
}

// RollbackVersion 回滚到指定版本.
func (e *Engine) RollbackVersion(functionID, version string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return fmt.Errorf("函数不存在: %s", functionID)
	}

	versions := e.versions[functionID]
	for _, v := range versions {
		if v.Version == version {
			fn.Code = v.Code
			fn.Config = v.Config
			fn.Runtime = v.Runtime
			fn.Handler = v.Handler
			fn.Version = version
			fn.DeployStatus = DeployStatusDraft
			fn.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("版本不存在: %s", version)
}

// addVersion 添加版本记录.
func (e *Engine) addVersion(fn *Function, changelog string) {
	version := &FunctionVersion{
		Version:      fn.Version,
		FunctionID:   fn.ID,
		Code:         fn.Code,
		Config:       fn.Config,
		Runtime:      fn.Runtime,
		Handler:      fn.Handler,
		DeployStatus: fn.DeployStatus,
		CreatedAt:    time.Now(),
		Changelog:    changelog,
	}

	e.versions[fn.ID] = append(e.versions[fn.ID], version)
}

// ========== 日志管理 ==========

// GetLogs 获取函数日志.
func (e *Engine) GetLogs(functionID string, limit int) []*FunctionLog {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*FunctionLog, 0)
	for _, log := range e.logs {
		if log.FunctionID == functionID {
			result = append(result, log)
		}
	}

	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// addLog 添加日志.
func (e *Engine) addLog(level LogLevel, invocationID, functionID, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log := &FunctionLog{
		ID:           generateID("log"),
		InvocationID: invocationID,
		FunctionID:   functionID,
		Level:        level,
		Message:      message,
		Timestamp:    time.Now(),
	}

	e.logs = append(e.logs, log)
}

// ========== 指标统计 ==========

// GetMetrics 获取函数指标.
func (e *Engine) GetMetrics(functionID string) (*FunctionMetrics, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn, exists := e.functions[functionID]
	if !exists {
		return nil, fmt.Errorf("函数不存在: %s", functionID)
	}

	metrics := &FunctionMetrics{
		FunctionID:      functionID,
		TotalInvocations: fn.InvokeCount,
		ErrorCount:      fn.ErrorCount,
		SuccessCount:    fn.InvokeCount - fn.ErrorCount,
		Period:          "all",
	}

	// 计算调用统计
	durations := make([]time.Duration, 0)
	for _, inv := range e.invocations {
		if inv.FunctionID == functionID && inv.Status == InvocationStatusSuccess {
			durations = append(durations, inv.Duration)
			if inv.MemoryUsedMB > metrics.MaxMemoryUsedMB {
				metrics.MaxMemoryUsedMB = inv.MemoryUsedMB
			}
		}
		if inv.FunctionID == functionID && inv.Status == InvocationStatusTimeout {
			metrics.TimeoutCount++
		}
	}

	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})

		var total time.Duration
		for _, d := range durations {
			total += d
		}
		metrics.AvgDuration = total / time.Duration(len(durations))

		p50Idx := int(math.Ceil(float64(len(durations))*0.5)) - 1
		p95Idx := int(math.Ceil(float64(len(durations))*0.95)) - 1
		p99Idx := int(math.Ceil(float64(len(durations))*0.99)) - 1

		if p50Idx >= 0 && p50Idx < len(durations) {
			metrics.P50Duration = durations[p50Idx]
		}
		if p95Idx >= 0 && p95Idx < len(durations) {
			metrics.P95Duration = durations[p95Idx]
		}
		if p99Idx >= 0 && p99Idx < len(durations) {
			metrics.P99Duration = durations[p99Idx]
		}
	}

	return metrics, nil
}

// GetStats 获取引擎统计.
func (e *Engine) GetStats() *Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &Stats{
		TotalFunctions: len(e.functions),
		RuntimeStats:   make(map[string]int),
	}

	for _, fn := range e.functions {
		if fn.DeployStatus == DeployStatusDeployed {
			stats.DeployedFunctions++
		}
		if fn.Enabled {
			stats.EnabledFunctions++
		}
		stats.TotalInvocations += fn.InvokeCount
		stats.ErrorCount += fn.ErrorCount
		stats.SuccessCount += (fn.InvokeCount - fn.ErrorCount)
		stats.RuntimeStats[string(fn.Runtime)]++
	}

	today := time.Now().Truncate(24 * time.Hour)
	for _, inv := range e.invocations {
		if inv.StartedAt.After(today) {
			stats.TodayInvocations++
		}
	}

	if stats.TotalInvocations > 0 {
		stats.SuccessRate = float64(stats.TotalInvocations-stats.ErrorCount) / float64(stats.TotalInvocations) * 100
	}

	return stats
}

// ========== 辅助函数 ==========

// generateID 生成唯一 ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// isValidRuntime 验证运行时.
func isValidRuntime(runtime Runtime) bool {
	switch runtime {
	case RuntimeGo, RuntimePython, RuntimeNode, RuntimeShell:
		return true
	default:
		return false
	}
}

// isValidTriggerType 验证触发器类型.
func isValidTriggerType(t TriggerType) bool {
	switch t {
	case TriggerHTTP, TriggerCron, TriggerFileWatcher, TriggerEvent:
		return true
	default:
		return false
	}
}

// matchFunctionFilter 匹配函数过滤条件.
func matchFunctionFilter(fn *Function, filter *FunctionFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Runtime != "" && fn.Runtime != filter.Runtime {
		return false
	}

	if filter.DeployStatus != "" && fn.DeployStatus != filter.DeployStatus {
		return false
	}

	if filter.Enabled != nil && fn.Enabled != *filter.Enabled {
		return false
	}

	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(fn.Name), search) &&
			!strings.Contains(strings.ToLower(fn.Description), search) {
			return false
		}
	}

	if len(filter.Tags) > 0 {
		tagMatch := false
		for _, tag := range filter.Tags {
			for _, t := range fn.Tags {
				if t == tag {
					tagMatch = true
					break
				}
			}
		}
		if !tagMatch {
			return false
		}
	}

	return true
}

// matchInvocationFilter 匹配调用过滤条件.
func matchInvocationFilter(inv *Invocation, filter *InvocationFilter) bool {
	if filter == nil {
		return true
	}

	if filter.FunctionID != "" && inv.FunctionID != filter.FunctionID {
		return false
	}

	if filter.Status != "" && inv.Status != filter.Status {
		return false
	}

	if filter.StartTime != nil && inv.StartedAt.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && inv.StartedAt.After(*filter.EndTime) {
		return false
	}

	return true
}

// parseInt 解析字符串为整数.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
