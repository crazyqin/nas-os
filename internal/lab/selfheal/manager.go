// Package selfheal 提供系统健康自检与自愈功能
package selfheal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 自检自愈管理器.
type Manager struct {
	checkers  map[string]Checker
	config    *StrategyConfig
	store     *Store
	logger    *zap.Logger
	mu        sync.RWMutex
	stopChan  chan struct{}
	running   bool
	lastRun   *OverallStatus
	lastRunMu sync.RWMutex
}

// NewManager 创建自检自愈管理器.
func NewManager(store *Store, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		checkers: make(map[string]Checker),
		config: &StrategyConfig{
			DefaultAction: HealActionNone,
			Overrides:     make(map[string]HealAction),
			CheckInterval: 30 * time.Minute,
			Enabled:       true,
		},
		store:    store,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// RegisterChecker 注册检查器.
func (m *Manager) RegisterChecker(checker Checker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers[checker.Name()] = checker
	m.logger.Info("registered self-heal checker", zap.String("name", checker.Name()))
}

// UnregisterChecker 注销检查器.
func (m *Manager) UnregisterChecker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checkers, name)
}

// GetChecker 获取检查器.
func (m *Manager) GetChecker(name string) (Checker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.checkers[name]
	return c, ok
}

// ListCheckers 列出所有已注册检查器.
func (m *Manager) ListCheckers() []CheckerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]CheckerInfo, 0, len(m.checkers))
	for _, c := range m.checkers {
		action := m.getAction(c.Name())
		infos = append(infos, CheckerInfo{
			Name:        c.Name(),
			Category:    c.Category(),
			Description: c.Description(),
			HealAction:  action,
		})
	}
	return infos
}

// CheckerInfo 检查项信息.
type CheckerInfo struct {
	Name        string        `json:"name"`
	Category    CheckCategory `json:"category"`
	Description string        `json:"description"`
	HealAction  HealAction    `json:"heal_action"`
}

// UpdateConfig 更新自愈策略配置.
func (m *Manager) UpdateConfig(cfg *StrategyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg == nil {
		return
	}
	if cfg.DefaultAction != "" {
		m.config.DefaultAction = cfg.DefaultAction
	}
	if cfg.Overrides != nil {
		m.config.Overrides = cfg.Overrides
	}
	if cfg.CheckInterval > 0 {
		m.config.CheckInterval = cfg.CheckInterval
	}
	if !cfg.Enabled {
		m.config.Enabled = false
	} else {
		m.config.Enabled = true
	}
}

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() *StrategyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 返回副本
	cp := &StrategyConfig{
		DefaultAction: m.config.DefaultAction,
		CheckInterval: m.config.CheckInterval,
		Enabled:       m.config.Enabled,
		Overrides:     make(map[string]HealAction),
	}
	for k, v := range m.config.Overrides {
		cp.Overrides[k] = v
	}
	return cp
}

// Start 启动定期健康检查调度器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.runScheduler()
	m.logger.Info("self-heal scheduler started")
}

// Stop 停止调度器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.logger.Info("self-heal scheduler stopped")
}

// IsRunning 检查调度器是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// runScheduler 调度器主循环.
func (m *Manager) runScheduler() {
	interval := m.getConfig().CheckInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动后立即执行一次
	m.RunAll(context.Background())

	for {
		select {
		case <-ticker.C:
			m.RunAll(context.Background())
		case <-m.stopChan:
			return
		}
	}
}

// RunAll 执行所有检查并尝试自愈.
func (m *Manager) RunAll(ctx context.Context) *OverallStatus {
	m.mu.RLock()
	checkers := make([]Checker, 0, len(m.checkers))
	for _, c := range m.checkers {
		checkers = append(checkers, c)
	}
	m.mu.RUnlock()

	overall := &OverallStatus{
		Timestamp: time.Now(),
		Summary:   &StatusSummary{},
		Checks:    make([]*CheckResult, 0, len(checkers)),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, checker := range checkers {
		wg.Add(1)
		go func(c Checker) {
			defer wg.Done()

			result := m.runSingle(ctx, c)

			mu.Lock()
			overall.Checks = append(overall.Checks, result)
			overall.Summary.Total++
			switch result.Status {
			case StatusHealthy:
				overall.Summary.Healthy++
			case StatusDegraded:
				overall.Summary.Degraded++
			case StatusUnhealthy:
				overall.Summary.Unhealthy++
			}

			// 根据策略决定是否自愈
			action := m.getAction(c.Name())
			if result.Status != StatusHealthy && action != HealActionNone {
				healResult := m.tryHeal(ctx, c, result, action)
				if healResult != nil && healResult.Success {
					overall.Healed++
					overall.Summary.Healed++
				}
			}

			// 持久化记录
			if m.store != nil {
				_ = m.store.SaveRecord(result, action)
			}
			mu.Unlock()
		}(checker)
	}

	wg.Wait()

	// 确定整体状态
	if overall.Summary.Unhealthy > 0 {
		overall.Status = StatusUnhealthy
	} else if overall.Summary.Degraded > 0 {
		overall.Status = StatusDegraded
	} else {
		overall.Status = StatusHealthy
	}

	// 缓存最近结果
	m.lastRunMu.Lock()
	m.lastRun = overall
	m.lastRunMu.Unlock()

	return overall
}

// RunSingle 执行单个检查.
func (m *Manager) RunSingle(ctx context.Context, name string) (*CheckResult, error) {
	m.mu.RLock()
	checker, exists := m.checkers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("checker %q not found", name)
	}

	result := m.runSingle(ctx, checker)

	// 持久化
	action := m.getAction(name)
	if m.store != nil {
		_ = m.store.SaveRecord(result, action)
	}

	return result, nil
}

// runSingle 内部执行单个检查.
func (m *Manager) runSingle(ctx context.Context, checker Checker) *CheckResult {
	checkCtx := &CheckContext{
		Timeout: 30 * time.Second,
		Forced:  true,
	}

	start := time.Now()
	result := checker.Check(checkCtx)
	result.Duration = time.Since(start)
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now()
	}

	return result
}

// tryHeal 尝试修复.
func (m *Manager) tryHeal(ctx context.Context, checker Checker, result *CheckResult, action HealAction) *HealResult {
	checkCtx := &CheckContext{
		Timeout: 60 * time.Second,
		Forced:  true,
	}

	healResult := checker.Heal(checkCtx, result)
	if healResult == nil {
		return nil
	}

	// 持久化修复结果
	if m.store != nil {
		_ = m.store.UpdateHealResult(result.Name, healResult)
	}

	m.logger.Info("heal attempt",
		zap.String("checker", result.Name),
		zap.Bool("success", healResult.Success),
		zap.String("message", healResult.Message),
	)

	return healResult
}

// GetLastStatus 获取最近一次检查的整体状态.
func (m *Manager) GetLastStatus() *OverallStatus {
	m.lastRunMu.RLock()
	defer m.lastRunMu.RUnlock()
	return m.lastRun
}

// getAction 获取检查项的自愈策略.
func (m *Manager) getAction(name string) HealAction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a, ok := m.config.Overrides[name]; ok {
		return a
	}
	return m.config.DefaultAction
}

// getConfig 获取配置副本.
func (m *Manager) getConfig() *StrategyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &StrategyConfig{
		DefaultAction: m.config.DefaultAction,
		CheckInterval: m.config.CheckInterval,
		Enabled:       m.config.Enabled,
	}
}

// GetHistory 获取检查历史.
func (m *Manager) GetHistory(limit int) ([]*HealRecord, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	return m.store.GetHistory(limit)
}
