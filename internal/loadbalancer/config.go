// Package loadbalancer - 动态配置热加载
package loadbalancer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ConfigManager 配置管理器.
type ConfigManager struct {
	config     LBConfig
	configPath string
	watchers   []ConfigWatcher
	lastUpdate time.Time

	mu sync.RWMutex
}

// ConfigWatcher 配置变更监听器.
type ConfigWatcher interface {
	OnConfigChange(config LBConfig)
}

// ConfigWatcherFunc 配置变更监听函数.
type ConfigWatcherFunc struct {
	fn func(config LBConfig)
}

// NewConfigWatcherFunc 创建配置变更监听函数.
func NewConfigWatcherFunc(fn func(config LBConfig)) *ConfigWatcherFunc {
	return &ConfigWatcherFunc{fn: fn}
}

// OnConfigChange 配置变更回调.
func (f *ConfigWatcherFunc) OnConfigChange(config LBConfig) {
	f.fn(config)
}

// NewConfigManager 创建配置管理器.
func NewConfigManager(configPath string, initialConfig LBConfig) *ConfigManager {
	return &ConfigManager{
		config:     initialConfig,
		configPath: configPath,
		lastUpdate: time.Now(),
	}
}

// GetConfig 获取当前配置.
func (cm *ConfigManager) GetConfig() LBConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateConfig 更新配置.
func (cm *ConfigManager) UpdateConfig(config LBConfig) {
	cm.mu.Lock()
	cm.config = config
	cm.lastUpdate = time.Now()
	cm.mu.Unlock()

	// 通知所有监听器
	cm.notifyWatchers(config)
}

// AddWatcher 添加配置监听器.
func (cm *ConfigManager) AddWatcher(watcher ConfigWatcher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.watchers = append(cm.watchers, watcher)
}

// RemoveWatcher 移除配置监听器.
func (cm *ConfigManager) RemoveWatcher(watcher ConfigWatcher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, w := range cm.watchers {
		if w == watcher {
			cm.watchers = append(cm.watchers[:i], cm.watchers[i+1:]...)
			return
		}
	}
}

// notifyWatchers 通知所有监听器.
func (cm *ConfigManager) notifyWatchers(config LBConfig) {
	cm.mu.RLock()
	watchers := make([]ConfigWatcher, len(cm.watchers))
	copy(watchers, cm.watchers)
	cm.mu.RUnlock()

	for _, watcher := range watchers {
		go watcher.OnConfigChange(config)
	}
}

// LoadFromFile 从文件加载配置.
func (cm *ConfigManager) LoadFromFile() error {
	if cm.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var config LBConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	cm.UpdateConfig(config)
	return nil
}

// SaveToFile 保存配置到文件.
func (cm *ConfigManager) SaveToFile() error {
	if cm.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	cm.mu.RLock()
	config := cm.config
	cm.mu.RUnlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// StartWatching 启动配置文件监听.
func (cm *ConfigManager) StartWatching(interval time.Duration) {
	go cm.watchLoop(interval)
}

// watchLoop 配置文件监听循环.
func (cm *ConfigManager) watchLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastModTime time.Time

	// 获取初始修改时间
	if info, err := os.Stat(cm.configPath); err == nil {
		lastModTime = info.ModTime()
	}

	for range ticker.C {
		info, err := os.Stat(cm.configPath)
		if err != nil {
			continue
		}

		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
			if err := cm.LoadFromFile(); err != nil {
				fmt.Printf("Failed to reload config: %v\n", err)
			} else {
				fmt.Println("Config reloaded successfully")
			}
		}
	}
}

// ============================================================
// 配置热加载器
// ============================================================

// HotReloader 配置热加载器.
type HotReloader struct {
	configManager *ConfigManager
	balancer      *Balancer
	healthChecker *HealthChecker
	rateLimiter   RateLimiter
}

// NewHotReloader 创建配置热加载器.
func NewHotReloader(configManager *ConfigManager, balancer *Balancer, healthChecker *HealthChecker, rateLimiter RateLimiter) *HotReloader {
	return &HotReloader{
		configManager: configManager,
		balancer:      balancer,
		healthChecker: healthChecker,
		rateLimiter:   rateLimiter,
	}
}

// Start 启动热加载.
func (hr *HotReloader) Start() {
	// 注册配置变更监听
	hr.configManager.AddWatcher(NewConfigWatcherFunc(hr.onConfigChange))
}

// Stop 停止热加载.
func (hr *HotReloader) Stop() {
	hr.configManager.RemoveWatcher(NewConfigWatcherFunc(hr.onConfigChange))
}

// onConfigChange 配置变更处理.
func (hr *HotReloader) onConfigChange(config LBConfig) {
	// 更新负载均衡算法
	hr.balancer.UpdateAlgorithm(config.Algorithm)

	// 更新后端列表
	hr.updateBackends(config.Backends)

	// 更新健康检查配置
	if hr.healthChecker != nil {
		hr.healthChecker.SetConfig(config.HealthCheck)
	}

	// 更新限流配置
	if hr.rateLimiter != nil {
		// 限流器配置更新需要重新创建
		// 这里简化处理，实际应该支持动态更新
	}

	fmt.Println("Configuration hot-reloaded successfully")
}

// updateBackends 更新后端列表.
func (hr *HotReloader) updateBackends(configs []BackendConfig) {
	// 获取当前后端列表
	currentBackends := hr.balancer.GetBackends()
	currentMap := make(map[string]*Backend)
	for _, b := range currentBackends {
		currentMap[b.ID] = b
	}

	// 添加或更新后端
	for _, config := range configs {
		if existing, exists := currentMap[config.ID]; exists {
			// 更新现有后端
			existing.Name = config.Name
			existing.URL = config.URL
			hr.balancer.UpdateBackendWeight(config.ID, config.Weight)
			existing.MaxConns = config.MaxConns
			existing.Tags = config.Tags
		} else {
			// 添加新后端
			backend := &Backend{
				ID:        config.ID,
				Name:      config.Name,
				URL:       config.URL,
				Weight:    config.Weight,
				MaxConns:  config.MaxConns,
				Tags:      config.Tags,
				IsHealthy: true,
				AddedAt:   time.Now(),
			}
			hr.balancer.AddBackend(backend)
		}
	}

	// 删除不再存在的后端
	configMap := make(map[string]bool)
	for _, config := range configs {
		configMap[config.ID] = true
	}

	for _, backend := range currentBackends {
		if !configMap[backend.ID] {
			hr.balancer.RemoveBackend(backend.ID)
		}
	}
}

// SetConfig 设置健康检查配置.
func (hc *HealthChecker) SetConfig(config HealthCheckConfig) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.config = config
}

// ============================================================
// 配置验证
// ============================================================

// ValidateConfig 验证配置.
func ValidateConfig(config LBConfig) error {
	// 验证监听地址
	if config.ListenAddr == "" {
		return fmt.Errorf("listen_addr is required")
	}

	// 验证负载均衡算法
	switch config.Algorithm {
	case AlgorithmRoundRobin, AlgorithmWeightedRoundRobin, AlgorithmLeastConn, AlgorithmIPHash:
		// 有效
	default:
		return fmt.Errorf("invalid algorithm: %s", config.Algorithm)
	}

	// 验证后端列表
	if len(config.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	// 验证每个后端
	for i, backend := range config.Backends {
		if backend.URL == "" {
			return fmt.Errorf("backend[%d].url is required", i)
		}
		if backend.Weight < 0 || backend.Weight > 100 {
			return fmt.Errorf("backend[%d].weight must be between 0 and 100", i)
		}
	}

	// 验证健康检查配置
	if config.HealthCheck.Enabled {
		if config.HealthCheck.Interval <= 0 {
			return fmt.Errorf("health_check.interval must be positive")
		}
		if config.HealthCheck.Timeout <= 0 {
			return fmt.Errorf("health_check.timeout must be positive")
		}
		if config.HealthCheck.HealthyThreshold <= 0 {
			return fmt.Errorf("health_check.healthy_threshold must be positive")
		}
		if config.HealthCheck.UnhealthyThreshold <= 0 {
			return fmt.Errorf("health_check.unhealthy_threshold must be positive")
		}
	}

	// 验证熔断器配置
	if config.CircuitBreaker.Enabled {
		if config.CircuitBreaker.FailureThreshold <= 0 {
			return fmt.Errorf("circuit_breaker.failure_threshold must be positive")
		}
		if config.CircuitBreaker.FailureRatio < 0 || config.CircuitBreaker.FailureRatio > 1 {
			return fmt.Errorf("circuit_breaker.failure_ratio must be between 0 and 1")
		}
		if config.CircuitBreaker.Timeout <= 0 {
			return fmt.Errorf("circuit_breaker.timeout must be positive")
		}
	}

	// 验证限流配置
	if config.RateLimit.Enabled {
		if config.RateLimit.Rate <= 0 {
			return fmt.Errorf("rate_limit.rate must be positive")
		}
		if config.RateLimit.Burst <= 0 {
			return fmt.Errorf("rate_limit.burst must be positive")
		}
	}

	return nil
}

// MergeConfig 合并配置.
func MergeConfig(base, override LBConfig) LBConfig {
	result := base

	// 覆盖基础配置
	if override.ListenAddr != "" {
		result.ListenAddr = override.ListenAddr
	}
	if override.Algorithm != "" {
		result.Algorithm = override.Algorithm
	}

	// 覆盖后端列表
	if len(override.Backends) > 0 {
		result.Backends = override.Backends
	}

	// 覆盖健康检查配置
	if override.HealthCheck.Enabled {
		result.HealthCheck = override.HealthCheck
	}

	// 覆盖熔断器配置
	if override.CircuitBreaker.Enabled {
		result.CircuitBreaker = override.CircuitBreaker
	}

	// 覆盖限流配置
	if override.RateLimit.Enabled {
		result.RateLimit = override.RateLimit
	}

	// 覆盖代理配置
	if override.Proxy.DialTimeout > 0 {
		result.Proxy.DialTimeout = override.Proxy.DialTimeout
	}
	if override.Proxy.ResponseTimeout > 0 {
		result.Proxy.ResponseTimeout = override.Proxy.ResponseTimeout
	}

	// 覆盖中间件配置
	if override.Middleware.Logging.Enabled {
		result.Middleware.Logging = override.Middleware.Logging
	}
	if override.Middleware.CORS.Enabled {
		result.Middleware.CORS = override.Middleware.CORS
	}
	if override.Middleware.Compression.Enabled {
		result.Middleware.Compression = override.Middleware.Compression
	}
	if override.Middleware.Cache.Enabled {
		result.Middleware.Cache = override.Middleware.Cache
	}

	return result
}
