// Package apigateway 提供 API 网关核心逻辑
package apigateway

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Manager API 网关管理器
type Manager struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	config         *GatewayConfig
	routes         map[string]*Route
	upstreams      map[string]*Upstream
	consumers      map[string]*Consumer
	apiKeys        map[string]*APIKeyInfo
	versions       map[string]*APIVersion
	circuitBreakers map[string]*CircuitBreaker
	rateLimiter    RateLimiter
	plugins        []Plugin
	requestLogs    []*RequestLog
	stats          *GatewayStats
	startTime      time.Time
	stopChan       chan struct{}
	running        bool
}

// NewManager 创建 API 网关管理器
func NewManager(logger *zap.Logger, config *GatewayConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultGatewayConfig()
	}

	m := &Manager{
		logger:          logger,
		config:          config,
		routes:          make(map[string]*Route),
		upstreams:       make(map[string]*Upstream),
		consumers:       make(map[string]*Consumer),
		apiKeys:         make(map[string]*APIKeyInfo),
		versions:        make(map[string]*APIVersion),
		circuitBreakers: make(map[string]*CircuitBreaker),
		plugins:         make([]Plugin, 0),
		requestLogs:     make([]*RequestLog, 0, config.LogMaxSize),
		stats:           &GatewayStats{},
		startTime:       time.Now(),
		stopChan:        make(chan struct{}),
	}

	// 初始化限流器
	if config.RateLimit.Enabled {
		switch config.RateLimit.Algorithm {
		case "sliding-window":
			m.rateLimiter = NewSlidingWindowLimiter(
				config.RateLimit.RequestsPerSecond,
				time.Duration(config.RateLimit.WindowSize)*time.Second,
			)
		default:
			m.rateLimiter = NewTokenBucketLimiter(
				float64(config.RateLimit.RequestsPerSecond),
				config.RateLimit.Burst,
			)
		}
	}

	return m
}

// Start 启动网关
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("gateway already running")
	}

	m.running = true
	m.startTime = time.Now()
	m.logger.Info("API gateway started",
		zap.String("addr", m.config.ListenAddr),
		zap.Int("port", m.config.ListenPort),
	)
	return nil
}

// Stop 停止网关
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)
	m.logger.Info("API gateway stopped")
	return nil
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ==================== 路由管理 ====================

// AddRoute 添加路由
func (m *Manager) AddRoute(route *Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if route.ID == "" {
		route.ID = generateID()
	}
	if _, exists := m.routes[route.ID]; exists {
		return fmt.Errorf("route %s already exists", route.ID)
	}

	route.CreatedAt = time.Now()
	route.UpdatedAt = time.Now()
	m.routes[route.ID] = route
	m.stats.TotalRoutes = len(m.routes)

	m.logger.Info("route added", zap.String("id", route.ID), zap.String("path", route.Path))
	return nil
}

// UpdateRoute 更新路由
func (m *Manager) UpdateRoute(route *Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.routes[route.ID]; !exists {
		return fmt.Errorf("route %s not found", route.ID)
	}

	route.UpdatedAt = time.Now()
	m.routes[route.ID] = route
	return nil
}

// DeleteRoute 删除路由
func (m *Manager) DeleteRoute(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.routes[id]; !exists {
		return fmt.Errorf("route %s not found", id)
	}

	delete(m.routes, id)
	m.stats.TotalRoutes = len(m.routes)
	m.logger.Info("route deleted", zap.String("id", id))
	return nil
}

// GetRoute 获取路由
func (m *Manager) GetRoute(id string) (*Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	route, exists := m.routes[id]
	if !exists {
		return nil, fmt.Errorf("route %s not found", id)
	}
	return route, nil
}

// ListRoutes 列出路由
func (m *Manager) ListRoutes() []*Route {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make([]*Route, 0, len(m.routes))
	for _, r := range m.routes {
		routes = append(routes, r)
	}
	return routes
}

// MatchRoute 匹配路由
func (m *Manager) MatchRoute(method, path string) *Route {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, route := range m.routes {
		if !route.Enabled {
			continue
		}
		if m.matchMethod(route, method) && m.matchPath(route, path) {
			return route
		}
	}
	return nil
}

// matchMethod 匹配方法
func (m *Manager) matchMethod(route *Route, method string) bool {
	if len(route.Methods) == 0 {
		return true
	}
	for _, m := range route.Methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// matchPath 匹配路径
func (m *Manager) matchPath(route *Route, path string) bool {
	// 检查单个路径字段
	if route.Path != "" {
		if path == route.Path || (strings.HasSuffix(route.Path, "/") && strings.HasPrefix(path, route.Path)) {
			return true
		}
	}
	// 检查多个路径字段
	if len(route.Paths) > 0 {
		for _, p := range route.Paths {
			if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
				return true
			}
		}
	}
	return false
}

// ==================== 上游服务管理 ====================

// AddUpstream 添加上游服务
func (m *Manager) AddUpstream(upstream *Upstream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if upstream.ID == "" {
		upstream.ID = generateID()
	}
	if _, exists := m.upstreams[upstream.ID]; exists {
		return fmt.Errorf("upstream %s already exists", upstream.ID)
	}

	upstream.CreatedAt = time.Now()
	upstream.UpdatedAt = time.Now()
	m.upstreams[upstream.ID] = upstream
	m.stats.TotalUpstreams = len(m.upstreams)

	// 初始化熔断器
	if upstream.HealthCheck != nil {
		m.circuitBreakers[upstream.ID] = &CircuitBreaker{
			config: CircuitBreakerConfig{
				FailureThreshold: 5,
				SuccessThreshold: 3,
				Timeout:          30 * time.Second,
			},
			state: StateClosed,
		}
	}

	m.logger.Info("upstream added", zap.String("id", upstream.ID), zap.String("name", upstream.Name))
	return nil
}

// UpdateUpstream 更新上游服务
func (m *Manager) UpdateUpstream(upstream *Upstream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.upstreams[upstream.ID]; !exists {
		return fmt.Errorf("upstream %s not found", upstream.ID)
	}

	upstream.UpdatedAt = time.Now()
	m.upstreams[upstream.ID] = upstream
	return nil
}

// DeleteUpstream 删除上游服务
func (m *Manager) DeleteUpstream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.upstreams[id]; !exists {
		return fmt.Errorf("upstream %s not found", id)
	}

	delete(m.upstreams, id)
	delete(m.circuitBreakers, id)
	m.stats.TotalUpstreams = len(m.upstreams)
	m.logger.Info("upstream deleted", zap.String("id", id))
	return nil
}

// GetUpstream 获取上游服务
func (m *Manager) GetUpstream(id string) (*Upstream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upstream, exists := m.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", id)
	}
	return upstream, nil
}

// ListUpstreams 列出上游服务
func (m *Manager) ListUpstreams() []*Upstream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upstreams := make([]*Upstream, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		upstreams = append(upstreams, u)
	}
	return upstreams
}

// SelectTarget 选择目标（负载均衡）
func (m *Manager) SelectTarget(upstreamID string) (*Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upstream, exists := m.upstreams[upstreamID]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", upstreamID)
	}

	// 过滤健康的目标
	healthyTargets := make([]Target, 0)
	for _, t := range upstream.Targets {
		if t.Health != "unhealthy" {
			healthyTargets = append(healthyTargets, t)
		}
	}

	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstreamID)
	}

	// 根据算法选择目标
	switch upstream.Algorithm {
	case "weighted":
		return m.selectWeighted(healthyTargets), nil
	case "least-connections":
		// 简化实现，使用随机
		return &healthyTargets[rand.Intn(len(healthyTargets))], nil
	case "ip-hash":
		// 简化实现，使用随机
		return &healthyTargets[rand.Intn(len(healthyTargets))], nil
	default: // round-robin
		return &healthyTargets[rand.Intn(len(healthyTargets))], nil
	}
}

// selectWeighted 加权选择
func (m *Manager) selectWeighted(targets []Target) *Target {
	totalWeight := 0
	for _, t := range targets {
		totalWeight += t.Weight
	}
	if totalWeight == 0 {
		return &targets[rand.Intn(len(targets))]
	}

	r := rand.Intn(totalWeight)
	for i := range targets {
		r -= targets[i].Weight
		if r < 0 {
			return &targets[i]
		}
	}
	return &targets[0]
}

// AddTarget 添加目标
func (m *Manager) AddTarget(upstreamID string, target *Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	upstream, exists := m.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	if target.ID == "" {
		target.ID = generateID()
	}
	target.Health = "unknown"

	upstream.Targets = append(upstream.Targets, *target)
	upstream.UpdatedAt = time.Now()

	m.updateHealthyCounts()
	m.logger.Info("target added", zap.String("upstream", upstreamID), zap.String("host", target.Host))
	return nil
}

// RemoveTarget 移除目标
func (m *Manager) RemoveTarget(upstreamID, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	upstream, exists := m.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	for i, t := range upstream.Targets {
		if t.ID == targetID {
			upstream.Targets = append(upstream.Targets[:i], upstream.Targets[i+1:]...)
			upstream.UpdatedAt = time.Now()
			m.updateHealthyCounts()
			m.logger.Info("target removed", zap.String("upstream", upstreamID), zap.String("target", targetID))
			return nil
		}
	}

	return fmt.Errorf("target %s not found in upstream %s", targetID, upstreamID)
}

// updateHealthyCounts 更新健康目标计数
func (m *Manager) updateHealthyCounts() {
	healthy := 0
	unhealthy := 0
	for _, u := range m.upstreams {
		for _, t := range u.Targets {
			if t.Health == "healthy" {
				healthy++
			} else if t.Health == "unhealthy" {
				unhealthy++
			}
		}
	}
	m.stats.HealthyTargets = healthy
	m.stats.UnhealthyTargets = unhealthy
}

// ==================== 限流器实现 ====================

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(rate float64, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:    rate,
		burst:   burst,
		buckets: make(map[string]*tokenBucket),
	}
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.buckets[key]
	if !exists {
		b = &tokenBucket{
			tokens:   float64(l.burst),
			lastTime: time.Now(),
			rate:     l.rate,
			burst:    l.burst,
		}
		l.buckets[key] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > float64(b.burst) {
		b.tokens = float64(b.burst)
	}
	b.lastTime = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Reset 重置限流器
func (l *TokenBucketLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		limit:    limit,
		window:   window,
		counters: make(map[string]*slidingWindow),
	}
}

// Allow 检查是否允许请求
func (l *SlidingWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Truncate(l.window)

	c, exists := l.counters[key]
	if !exists {
		c = &slidingWindow{
			windowStart: windowStart,
			count:       0,
			prevCount:   0,
		}
		l.counters[key] = c
	}

	// 如果进入新窗口
	if windowStart != c.windowStart {
		c.prevCount = c.count
		c.count = 0
		c.windowStart = windowStart
	}

	// 计算滑动窗口内的请求数
	elapsedInWindow := now.Sub(windowStart).Seconds()
	windowSeconds := l.window.Seconds()
	weight := 1 - (elapsedInWindow / windowSeconds)
	weightedCount := float64(c.prevCount)*weight + float64(c.count)

	if int(weightedCount) < l.limit {
		c.count++
		return true
	}
	return false
}

// Reset 重置限流器
func (l *SlidingWindowLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counters, key)
}

// ==================== 认证相关 ====================

// AddAPIKey 添加 API Key
func (m *Manager) AddAPIKey(keyInfo *APIKeyInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.apiKeys[keyInfo.Key]; exists {
		return fmt.Errorf("api key already exists")
	}

	keyInfo.CreatedAt = time.Now()
	m.apiKeys[keyInfo.Key] = keyInfo
	m.logger.Info("api key added", zap.String("consumer", keyInfo.ConsumerID))
	return nil
}

// ValidateAPIKey 验证 API Key
func (m *Manager) ValidateAPIKey(key string) (*APIKeyInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyInfo, exists := m.apiKeys[key]
	if !exists {
		return nil, fmt.Errorf("invalid api key")
	}

	if !keyInfo.Enabled {
		return nil, fmt.Errorf("api key is disabled")
	}

	if !keyInfo.ExpiresAt.IsZero() && time.Now().After(keyInfo.ExpiresAt) {
		return nil, fmt.Errorf("api key has expired")
	}

	return keyInfo, nil
}

// DeleteAPIKey 删除 API Key
func (m *Manager) DeleteAPIKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.apiKeys[key]; !exists {
		return fmt.Errorf("api key not found")
	}

	delete(m.apiKeys, key)
	return nil
}

// ListAPIKeys 列出 API Key
func (m *Manager) ListAPIKeys(consumerID string) []*APIKeyInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKeyInfo, 0)
	for _, k := range m.apiKeys {
		if consumerID == "" || k.ConsumerID == consumerID {
			keys = append(keys, k)
		}
	}
	return keys
}

// ==================== 熔断器相关 ====================

// ExecuteWithCircuitBreaker 使用熔断器执行请求
func (m *Manager) ExecuteWithCircuitBreaker(upstreamID string, fn func() (*http.Response, error)) (*http.Response, error) {
	m.mu.RLock()
	cb, exists := m.circuitBreakers[upstreamID]
	m.mu.RUnlock()

	if !exists {
		return fn()
	}

	cb.mu.Lock()
	state := cb.state
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		if time.Now().After(cb.nextRetry) {
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			state = StateHalfOpen
		} else {
			return nil, fmt.Errorf("circuit breaker is open for upstream %s", upstreamID)
		}
	case StateHalfOpen:
		// 允许有限请求通过
	default: // StateClosed
	}

	resp, err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = StateOpen
			cb.nextRetry = time.Now().Add(cb.config.Timeout)
			cb.failureCount = 0
			m.logger.Warn("circuit breaker opened", zap.String("upstream", upstreamID))
		}
		return nil, err
	}

	if state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.successCount = 0
			cb.failureCount = 0
			m.logger.Info("circuit breaker closed", zap.String("upstream", upstreamID))
		}
	} else {
		cb.failureCount = 0
	}

	return resp, nil
}

// GetCircuitBreakerState 获取熔断器状态
func (m *Manager) GetCircuitBreakerState(upstreamID string) (CircuitBreakerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cb, exists := m.circuitBreakers[upstreamID]
	if !exists {
		return "", fmt.Errorf("circuit breaker not found for upstream %s", upstreamID)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state, nil
}

// ==================== 消费者管理 ====================

// AddConsumer 添加消费者
func (m *Manager) AddConsumer(consumer *Consumer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if consumer.ID == "" {
		consumer.ID = generateID()
	}
	if _, exists := m.consumers[consumer.ID]; exists {
		return fmt.Errorf("consumer %s already exists", consumer.ID)
	}

	consumer.CreatedAt = time.Now()
	m.consumers[consumer.ID] = consumer
	m.logger.Info("consumer added", zap.String("id", consumer.ID), zap.String("username", consumer.Username))
	return nil
}

// GetConsumer 获取消费者
func (m *Manager) GetConsumer(id string) (*Consumer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	consumer, exists := m.consumers[id]
	if !exists {
		return nil, fmt.Errorf("consumer %s not found", id)
	}
	return consumer, nil
}

// ListConsumers 列出消费者
func (m *Manager) ListConsumers() []*Consumer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	consumers := make([]*Consumer, 0, len(m.consumers))
	for _, c := range m.consumers {
		consumers = append(consumers, c)
	}
	return consumers
}

// DeleteConsumer 删除消费者
func (m *Manager) DeleteConsumer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.consumers[id]; !exists {
		return fmt.Errorf("consumer %s not found", id)
	}

	delete(m.consumers, id)
	return nil
}

// ==================== 插件管理 ====================

// RegisterPlugin 注册插件
func (m *Manager) RegisterPlugin(plugin Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, plugin)
	m.logger.Info("plugin registered", zap.String("name", plugin.Name()))
}

// GetPlugins 获取所有插件
func (m *Manager) GetPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins
}

// ==================== 请求日志 ====================

// LogRequest 记录请求日志
func (m *Manager) LogRequest(log *RequestLog) {
	if !m.config.LogEnabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	log.ID = generateID()
	log.Timestamp = time.Now()

	// 限制日志数量
	if len(m.requestLogs) >= m.config.LogMaxSize {
		m.requestLogs = m.requestLogs[1:]
	}
	m.requestLogs = append(m.requestLogs, log)

	// 更新统计
	atomic.AddInt64(&m.stats.TotalRequests, 1)
	m.stats.LastRequestTime = log.Timestamp
	m.stats.BytesReceived += log.RequestSize
	m.stats.BytesSent += log.ResponseSize
}

// GetRequestLogs 获取请求日志
func (m *Manager) GetRequestLogs(limit int) []*RequestLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.requestLogs) {
		limit = len(m.requestLogs)
	}

	start := len(m.requestLogs) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]*RequestLog, limit)
	copy(logs, m.requestLogs[start:])
	return logs
}

// ClearRequestLogs 清除请求日志
func (m *Manager) ClearRequestLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestLogs = make([]*RequestLog, 0, m.config.LogMaxSize)
}

// ==================== 版本管理 ====================

// AddAPIVersion 添加 API 版本
func (m *Manager) AddAPIVersion(version *APIVersion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[version.Version] = version
	m.logger.Info("api version added", zap.String("version", version.Version))
}

// GetAPIVersion 获取 API 版本
func (m *Manager) GetAPIVersion(version string) (*APIVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, exists := m.versions[version]
	if !exists {
		return nil, fmt.Errorf("api version %s not found", version)
	}
	return v, nil
}

// ListAPIVersions 列出 API 版本
func (m *Manager) ListAPIVersions() []*APIVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := make([]*APIVersion, 0, len(m.versions))
	for _, v := range m.versions {
		versions = append(versions, v)
	}
	return versions
}

// ==================== 代理转发 ====================

// CreateReverseProxy 创建反向代理
func (m *Manager) CreateReverseProxy(target *Target, route *Route) *httputil.ReverseProxy {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", target.Host, target.Port),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 自定义修改请求
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// 路径前缀处理
		if route.StripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, route.StripPrefix)
		}
		if route.AddPrefix != "" {
			req.URL.Path = route.AddPrefix + req.URL.Path
		}

		// 添加自定义头部
		for k, v := range route.Headers {
			req.Header.Set(k, v)
		}

		// 添加代理头部
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	// 自定义错误处理
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		m.logger.Error("proxy error",
			zap.String("target", targetURL.String()),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"code":1,"message":"bad gateway"}`))
	}

	return proxy
}

// ==================== 统计 ====================

// GetStats 获取网关统计
func (m *Manager) GetStats() *GatewayStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.Uptime = time.Since(m.startTime)

	// 计算每秒请求数
	if stats.Uptime.Seconds() > 0 {
		stats.RequestsPerSecond = float64(stats.TotalRequests) / stats.Uptime.Seconds()
	}

	return &stats
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *GatewayConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *GatewayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// ==================== 辅助函数 ====================

// generateID 生成唯一 ID
func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	id := make([]byte, 16)
	for i := range id {
		id[i] = chars[rand.Intn(len(chars))]
	}
	return string(id)
}
