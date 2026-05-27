// Package containerrecovery 提供容器自动恢复引擎功能
package containerrecovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Engine 容器自动恢复引擎.
type Engine struct {
	config          EngineConfig
	containers      map[string]*ContainerConfig
	depGraph        *DependencyGraph
	store           Store
	alertSender     AlertSender
	operator        ContainerOperator
	logger          *zap.Logger
	hooks           map[string][]RecoveryHook // container -> hooks
	mu              sync.RWMutex
	stopChan        chan struct{}
	running         bool
	recoverySem     chan struct{} // 并发控制
	stats           *RecoveryStats
	statsMu         sync.RWMutex
}

// NewEngine 创建恢复引擎.
func NewEngine(config EngineConfig, store Store, operator ContainerOperator, logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 3
	}
	return &Engine{
		config:      config,
		containers:  make(map[string]*ContainerConfig),
		depGraph:    NewDependencyGraph(),
		store:       store,
		operator:    operator,
		logger:      logger,
		hooks:       make(map[string][]RecoveryHook),
		recoverySem: make(chan struct{}, config.Concurrency),
		stats: &RecoveryStats{
			FailureFrequency: make(map[string]int64),
			ContainerStats:   make(map[string]*ContainerStats),
			LastUpdated:      time.Now(),
		},
	}
}

// SetAlertSender 设置告警发送器.
func (e *Engine) SetAlertSender(sender AlertSender) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alertSender = sender
}

// RegisterContainer 注册容器配置.
func (e *Engine) RegisterContainer(cfg *ContainerConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.containers[cfg.ContainerName] = cfg
	e.depGraph.Add(cfg.ContainerName, cfg.Dependencies, cfg.Priority)
	e.hooks[cfg.ContainerName] = cfg.Hooks

	e.logger.Info("registered container for recovery",
		zap.String("container", cfg.ContainerName),
		zap.Bool("enabled", cfg.Enabled),
		zap.String("action", string(cfg.Strategy.Action)),
	)
}

// UnregisterContainer 注销容器配置.
func (e *Engine) UnregisterContainer(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.containers, name)
	delete(e.hooks, name)
	e.depGraph.Remove(name)

	e.logger.Info("unregistered container", zap.String("container", name))
}

// GetContainer 获取容器配置.
func (e *Engine) GetContainer(name string) (*ContainerConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cfg, ok := e.containers[name]
	return cfg, ok
}

// ListContainers 列出所有已注册容器.
func (e *Engine) ListContainers() []*ContainerConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]*ContainerConfig, 0, len(e.containers))
	for _, cfg := range e.containers {
		list = append(list, cfg)
	}
	return list
}

// Start 启动恢复引擎.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopChan = make(chan struct{})
	e.mu.Unlock()

	go e.runMonitor()
	e.logger.Info("container recovery engine started")
}

// Stop 停止恢复引擎.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}
	e.running = false
	close(e.stopChan)
	e.logger.Info("container recovery engine stopped")
}

// IsRunning 检查引擎是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// runMonitor 监控主循环.
func (e *Engine) runMonitor() {
	interval := e.config.HealthCheckInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.checkAll()
		case <-e.stopChan:
			return
		}
	}
}

// checkAll 检查所有已注册容器.
func (e *Engine) checkAll() {
	e.mu.RLock()
	containers := make([]*ContainerConfig, 0, len(e.containers))
	for _, cfg := range e.containers {
		if cfg.Enabled {
			containers = append(containers, cfg)
		}
	}
	e.mu.RUnlock()

	var wg sync.WaitGroup
	for _, cfg := range containers {
		wg.Add(1)
		go func(c *ContainerConfig) {
			defer wg.Done()
			e.checkContainer(c)
		}(cfg)
	}
	wg.Wait()
}

// checkContainer 检查单个容器.
func (e *Engine) checkContainer(cfg *ContainerConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.HealthCheck.Timeout)
	defer cancel()

	healthStatus, err := e.performHealthCheck(ctx, cfg)
	if err != nil {
		e.logger.Warn("health check failed",
			zap.String("container", cfg.ContainerName),
			zap.Error(err),
		)
		healthStatus = HealthStatusUnknown
	}

	if healthStatus != HealthStatusHealthy {
		e.logger.Warn("container unhealthy",
			zap.String("container", cfg.ContainerName),
			zap.String("status", string(healthStatus)),
		)
		e.triggerRecovery(cfg, healthStatus)
	}
}

// performHealthCheck 执行健康检查.
func (e *Engine) performHealthCheck(ctx context.Context, cfg *ContainerConfig) (HealthStatus, error) {
	if e.operator == nil {
		return HealthStatusUnknown, fmt.Errorf("container operator not configured")
	}

	switch cfg.HealthCheck.Type {
	case HealthCheckContainer:
		return e.operator.GetHealthCheck(cfg.ContainerName)
	case HealthCheckHTTP:
		// HTTP 检查通过 operator 实现
		return e.operator.GetHealthCheck(cfg.ContainerName)
	case HealthCheckTCP:
		// TCP 检查通过 operator 实现
		return e.operator.GetHealthCheck(cfg.ContainerName)
	case HealthCheckCommand:
		// 命令检查通过 operator 实现
		return e.operator.GetHealthCheck(cfg.ContainerName)
	default:
		return HealthStatusUnknown, fmt.Errorf("unsupported health check type: %s", cfg.HealthCheck.Type)
	}
}

// triggerRecovery 触发恢复流程.
func (e *Engine) triggerRecovery(cfg *ContainerConfig, healthStatus HealthStatus) {
	// 检查冷却期
	if e.isInCooldown(cfg.ContainerName) {
		e.logger.Info("container in cooldown period, skipping recovery",
			zap.String("container", cfg.ContainerName),
		)
		return
	}

	// 并发控制
	select {
	case e.recoverySem <- struct{}{}:
		defer func() { <-e.recoverySem }()
	default:
		e.logger.Warn("recovery concurrency limit reached, queuing",
			zap.String("container", cfg.ContainerName),
		)
		e.recoverySem <- struct{}{}
		defer func() { <-e.recoverySem }()
	}

	// 检测故障模式
	failureMode := e.detectFailureMode(cfg.ContainerName)

	// 创建恢复记录
	record := &RecoveryRecord{
		ID:          uuid.New().String(),
		Container:   cfg.ContainerName,
		Action:      cfg.Strategy.Action,
		Status:      RecoveryStatusPending,
		FailureMode: failureMode,
		Reason:      fmt.Sprintf("health status: %s, failure mode: %s", healthStatus, failureMode),
		MaxRetries:  cfg.Strategy.MaxRetries,
		StartTime:   time.Now(),
	}

	e.logger.Info("starting recovery",
		zap.String("container", cfg.ContainerName),
		zap.String("action", string(cfg.Strategy.Action)),
		zap.String("failure_mode", string(failureMode)),
	)

	// 执行恢复
	e.executeRecovery(cfg, record)
}

// detectFailureMode 检测故障模式.
func (e *Engine) detectFailureMode(container string) FailureMode {
	if e.operator == nil {
		return FailureModeUnknown
	}

	status, err := e.operator.GetStatus(container)
	if err != nil {
		return FailureModeUnknown
	}

	// 根据容器状态推断故障模式
	switch {
	case status == "oom_killed":
		return FailureModeOOMKilled
	case status == "crash_loop" || status == "crash_loop_backoff":
		return FailureModeCrashLoopBackOff
	case status == "image_pull_error":
		return FailureModeImagePullBackOff
	case status == "network_error":
		return FailureModeNetworkError
	default:
		return FailureModeUnknown
	}
}

// isInCooldown 检查是否在冷却期内.
func (e *Engine) isInCooldown(container string) bool {
	if e.store == nil {
		return false
	}

	records, err := e.store.GetRecords(container, 1)
	if err != nil || len(records) == 0 {
		return false
	}

	lastRecord := records[0]
	e.mu.RLock()
	cfg, ok := e.containers[container]
	e.mu.RUnlock()

	if !ok {
		return false
	}

	cooldownEnd := lastRecord.StartTime.Add(cfg.Strategy.CooldownPeriod)
	return time.Now().Before(cooldownEnd)
}

// executeRecovery 执行恢复流程.
func (e *Engine) executeRecovery(cfg *ContainerConfig, record *RecoveryRecord) {
	record.Status = RecoveryStatusRunning
	e.saveRecord(record)

	// 执行 pre-recovery 钩子
	preResults := e.executeHooks(cfg.ContainerName, HookPhasePreRecovery, record)
	record.HooksExecuted = append(record.HooksExecuted, preResults...)

	// 检查 pre-hooks 是否失败
	for _, r := range preResults {
		if !r.Success {
			e.logger.Warn("pre-recovery hook failed",
				zap.String("container", cfg.ContainerName),
				zap.String("hook", r.Name),
			)
			// 根据钩子配置决定是否继续
			for _, hook := range e.hooks[cfg.ContainerName] {
				if hook.Name == r.Name && !hook.ContinueOnError {
					record.Status = RecoveryStatusFailed
					record.ErrorMessage = fmt.Sprintf("pre-recovery hook %q failed: %s", r.Name, r.Error)
					endTime := time.Now()
					record.EndTime = &endTime
					record.Duration = endTime.Sub(record.StartTime)
					e.saveRecord(record)
					e.updateStats(record)
					e.sendAlert(cfg, record)
					return
				}
			}
		}
	}

	// 执行恢复动作
	var err error
	switch cfg.Strategy.Action {
	case RecoveryActionRestart:
		err = e.executeRestart(cfg, record)
	case RecoveryActionNotify:
		e.sendAlert(cfg, record)
		record.Status = RecoveryStatusSuccess
	case RecoveryActionRollback:
		err = e.executeRollback(cfg, record)
	case RecoveryActionScaleUp:
		err = e.executeScaleUp(cfg, record)
	default:
		err = fmt.Errorf("unsupported recovery action: %s", cfg.Strategy.Action)
	}

	if err != nil {
		record.Status = RecoveryStatusFailed
		record.ErrorMessage = err.Error()
	} else {
		record.Status = RecoveryStatusSuccess
	}

	endTime := time.Now()
	record.EndTime = &endTime
	record.Duration = endTime.Sub(record.StartTime)

	// 执行 post-recovery 钩子
	postResults := e.executeHooks(cfg.ContainerName, HookPhasePostRecovery, record)
	record.HooksExecuted = append(record.HooksExecuted, postResults...)

	e.saveRecord(record)
	e.updateStats(record)

	if record.Status == RecoveryStatusFailed {
		e.sendAlert(cfg, record)
	}
}

// executeRestart 执行重启恢复.
func (e *Engine) executeRestart(cfg *ContainerConfig, record *RecoveryRecord) error {
	if e.operator == nil {
		return fmt.Errorf("container operator not configured")
	}

	var lastErr error
	backoff := cfg.Strategy.InitialBackoff

	for i := 0; i < cfg.Strategy.MaxRetries; i++ {
		record.RetryCount = i + 1

		e.logger.Info("attempting restart",
			zap.String("container", cfg.ContainerName),
			zap.Int("attempt", i+1),
			zap.Int("max_retries", cfg.Strategy.MaxRetries),
		)

		err := e.operator.Rename(cfg.ContainerName)
		if err == nil {
			// 等待容器启动
			time.Sleep(2 * time.Second)

			// 验证恢复是否成功
			status, err := e.operator.GetHealthCheck(cfg.ContainerName)
			if err == nil && status == HealthStatusHealthy {
				e.logger.Info("recovery successful",
					zap.String("container", cfg.ContainerName),
					zap.Int("attempts", i+1),
				)
				return nil
			}
		}

		lastErr = err
		e.logger.Warn("restart attempt failed",
			zap.String("container", cfg.ContainerName),
			zap.Int("attempt", i+1),
			zap.Error(err),
		)

		// 指数退避
		if i < cfg.Strategy.MaxRetries-1 {
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * cfg.Strategy.BackoffMultiplier)
			if backoff > cfg.Strategy.MaxBackoff {
				backoff = cfg.Strategy.MaxBackoff
			}
		}
	}

	return fmt.Errorf("recovery failed after %d attempts: %w", cfg.Strategy.MaxRetries, lastErr)
}

// executeRollback 执行回滚恢复.
func (e *Engine) executeRollback(cfg *ContainerConfig, record *RecoveryRecord) error {
	if e.operator == nil {
		return fmt.Errorf("container operator not configured")
	}

	e.logger.Info("attempting rollback",
		zap.String("container", cfg.ContainerName),
	)

	return e.operator.Rollback(cfg.ContainerName)
}

// executeScaleUp 执行扩容恢复.
func (e *Engine) executeScaleUp(cfg *ContainerConfig, record *RecoveryRecord) error {
	if e.operator == nil {
		return fmt.Errorf("container operator not configured")
	}

	e.logger.Info("attempting scale up",
		zap.String("container", cfg.ContainerName),
	)

	return e.operator.Scale(cfg.ContainerName, 2)
}

// executeHooks 执行钩子.
func (e *Engine) executeHooks(container string, phase HookPhase, record *RecoveryRecord) []HookResult {
	e.mu.RLock()
	hooks := e.hooks[container]
	e.mu.RUnlock()

	var results []HookResult
	for _, hook := range hooks {
		if hook.Phase != phase {
			continue
		}

		result := e.executeHook(hook, record)
		results = append(results, result)
	}

	return results
}

// executeHook 执行单个钩子.
func (e *Engine) executeHook(hook RecoveryHook, record *RecoveryRecord) HookResult {
	start := time.Now()

	// 这里应该实际执行命令，简化实现
	// 实际实现中应使用 os/exec 执行命令
	result := HookResult{
		Name:     hook.Name,
		Phase:    hook.Phase,
		Success:  true,
		Output:   "hook executed (mock)",
		Duration: time.Since(start),
	}

	e.logger.Debug("executed hook",
		zap.String("name", hook.Name),
		zap.String("phase", string(hook.Phase)),
		zap.Bool("success", result.Success),
	)

	return result
}

// saveRecord 保存恢复记录.
func (e *Engine) saveRecord(record *RecoveryRecord) {
	if e.store == nil {
		return
	}
	if err := e.store.SaveRecord(record); err != nil {
		e.logger.Error("failed to save recovery record",
			zap.String("id", record.ID),
			zap.Error(err),
		)
	}
}

// updateStats 更新统计信息.
func (e *Engine) updateStats(record *RecoveryRecord) {
	if e.store == nil {
		return
	}
	if err := e.store.UpdateStats(record); err != nil {
		e.logger.Error("failed to update stats",
			zap.String("id", record.ID),
			zap.Error(err),
		)
	}
}

// sendAlert 发送告警.
func (e *Engine) sendAlert(cfg *ContainerConfig, record *RecoveryRecord) {
	e.mu.RLock()
	sender := e.alertSender
	e.mu.RUnlock()

	if sender == nil {
		return
	}

	// 判断是否需要发送告警
	if cfg.Strategy.Action == RecoveryActionNotify || record.Status == RecoveryStatusFailed {
		if !cfg.Strategy.NotifyOnFailure && record.Status == RecoveryStatusFailed {
			return
		}

		alertLevel := AlertLevelWarning
		if record.Status == RecoveryStatusFailed {
			alertLevel = AlertLevelError
		}

		alert := &Alert{
			Level:     alertLevel,
			Container: cfg.ContainerName,
			Title:     fmt.Sprintf("Container Recovery: %s", cfg.ContainerName),
			Message:   fmt.Sprintf("Recovery %s for container %s", record.Status, cfg.ContainerName),
			Timestamp: time.Now(),
			Details:   record.ErrorMessage,
		}

		if err := sender.Send(alert); err != nil {
			e.logger.Error("failed to send alert",
				zap.String("container", cfg.ContainerName),
				zap.Error(err),
			)
		}
	}
}

// ========== 手动触发 ==========

// TriggerRecovery 手动触发恢复.
func (e *Engine) TriggerRecovery(container string) (*RecoveryRecord, error) {
	e.mu.RLock()
	cfg, ok := e.containers[container]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("container %q not registered", container)
	}

	record := &RecoveryRecord{
		ID:          uuid.New().String(),
		Container:   container,
		Action:      cfg.Strategy.Action,
		Status:      RecoveryStatusPending,
		FailureMode: FailureModeUnknown,
		Reason:      "manual trigger",
		MaxRetries:  cfg.Strategy.MaxRetries,
		StartTime:   time.Now(),
	}

	e.executeRecovery(cfg, record)
	return record, nil
}

// GetRecoveryRecords 获取恢复记录.
func (e *Engine) GetRecoveryRecords(container string, limit int) ([]*RecoveryRecord, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	return e.store.GetRecords(container, limit)
}

// GetStats 获取恢复统计.
func (e *Engine) GetStats() (*RecoveryStats, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	return e.store.GetStats()
}

// UpdateConfig 更新引擎配置.
func (e *Engine) UpdateConfig(config EngineConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = config

	// 更新并发控制
	if config.Concurrency > 0 {
		e.recoverySem = make(chan struct{}, config.Concurrency)
	}
}

// GetConfig 获取引擎配置.
func (e *Engine) GetConfig() EngineConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// ========== 依赖图操作 ==========

// NewDependencyGraph 创建依赖图.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		containers: make(map[string]*ContainerDependency),
		dependents: make(map[string][]string),
	}
}

// Add 添加容器依赖.
func (g *DependencyGraph) Add(name string, deps []string, priority int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.containers[name] = &ContainerDependency{
		Name:         name,
		Dependencies: deps,
		Priority:     priority,
	}

	// 更新反向索引
	for _, dep := range deps {
		g.dependents[dep] = append(g.dependents[dep], name)
	}
}

// Remove 移除容器.
func (g *DependencyGraph) Remove(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cd, ok := g.containers[name]; ok {
		// 清理反向索引
		for _, dep := range cd.Dependencies {
			dependents := g.dependents[dep]
			for i, d := range dependents {
				if d == name {
					g.dependents[dep] = append(dependents[:i], dependents[i+1:]...)
					break
				}
			}
		}
		delete(g.dependents, name)
		delete(g.containers, name)
	}
}

// GetRecoveryOrder 获取恢复顺序（拓扑排序）.
func (g *DependencyGraph) GetRecoveryOrder(containers []string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 构建子图
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)
	priorities := make(map[string]int)

	for _, name := range containers {
		if cd, ok := g.containers[name]; ok {
			inDegree[name] = 0
			priorities[name] = cd.Priority
			for _, dep := range cd.Dependencies {
				// 只考虑在请求列表中的依赖
				for _, c := range containers {
					if c == dep {
						adjList[dep] = append(adjList[dep], name)
						inDegree[name]++
						break
					}
				}
			}
		}
	}

	// 拓扑排序（Kahn 算法）
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// 按优先级排序队列
	sortByPriority(queue, priorities)

	var result []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, neighbor := range adjList[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				sortByPriority(queue, priorities)
			}
		}
	}

	return result
}

// sortByPriority 按优先级排序（数值越小优先级越高）.
func sortByPriority(items []string, priorities map[string]int) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			if priorities[items[j]] < priorities[items[j-1]] {
				items[j], items[j-1] = items[j-1], items[j]
			}
		}
	}
}

// GetDependents 获取依赖指定容器的容器列表.
func (g *DependencyGraph) GetDependents(name string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.dependents[name]
}

// GetDependencies 获取指定容器的依赖列表.
func (g *DependencyGraph) GetDependencies(name string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if cd, ok := g.containers[name]; ok {
		return cd.Dependencies
	}
	return nil
}
