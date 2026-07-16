// Package nasgateway 提供网关引擎核心功能
package nasgateway

import (
	"fmt"
	"sync"
	"time"
)

// Engine 网关引擎.
type Engine struct {
	mu              sync.RWMutex
	routes          map[string]*Route
	upstreams       map[string]*Upstream
	policies        map[string]*Policy
	plugins         map[string]Plugin
	pluginConfigs   map[string]*PluginConfig
	versions        map[string]*APIVersion
	rateLimiters    map[string]*RateLimiter
	circuitBreakers map[string]*CircuitBreaker
	waf             *WAF
	oauthServer     *OAuthServer
	running         bool
	startTime       time.Time
	config          *GatewayConfig
	stats           *GatewayStats
	requestLogs     []*RequestLog
	maxLogs         int
}

// NewEngine 创建网关引擎.
func NewEngine() *Engine {
	return &Engine{
		routes:          make(map[string]*Route),
		upstreams:       make(map[string]*Upstream),
		policies:        make(map[string]*Policy),
		plugins:         make(map[string]Plugin),
		pluginConfigs:   make(map[string]*PluginConfig),
		versions:        make(map[string]*APIVersion),
		rateLimiters:    make(map[string]*RateLimiter),
		circuitBreakers: make(map[string]*CircuitBreaker),
		waf:             NewWAF(),
		oauthServer:     NewOAuthServer(),
		config:          &GatewayConfig{},
		stats:           &GatewayStats{},
		requestLogs:     make([]*RequestLog, 0),
		maxLogs:         10000,
	}
}

// Start 启动引擎.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.running = true
	e.startTime = time.Now()
	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	return nil
}

// IsRunning 返回是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// ========== 路由管理 ==========

// AddRoute 添加路由.
func (e *Engine) AddRoute(route *Route) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if route.ID == "" {
		return fmt.Errorf("路由ID不能为空")
	}

	if _, exists := e.routes[route.ID]; exists {
		return ErrRouteExists
	}

	now := time.Now()
	route.CreatedAt = now
	route.UpdatedAt = now
	route.Enabled = true
	e.routes[route.ID] = route
	e.stats.TotalRoutes++

	return nil
}

// GetRoute 获取路由.
func (e *Engine) GetRoute(id string) (*Route, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	route, exists := e.routes[id]
	if !exists {
		return nil, ErrRouteNotFound
	}
	return route, nil
}

// ListRoutes 列出路由.
func (e *Engine) ListRoutes() []*Route {
	e.mu.RLock()
	defer e.mu.RUnlock()

	routes := make([]*Route, 0, len(e.routes))
	for _, r := range e.routes {
		routes = append(routes, r)
	}
	return routes
}

// UpdateRoute 更新路由.
func (e *Engine) UpdateRoute(route *Route) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.routes[route.ID]; !exists {
		return ErrRouteNotFound
	}

	route.UpdatedAt = time.Now()
	e.routes[route.ID] = route
	return nil
}

// DeleteRoute 删除路由.
func (e *Engine) DeleteRoute(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.routes[id]; !exists {
		return ErrRouteNotFound
	}

	delete(e.routes, id)
	e.stats.TotalRoutes--
	return nil
}

// FindRoute 查找匹配的路由.
func (e *Engine) FindRoute(method, path, host string) (*Route, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 按优先级排序查找
	var bestMatch *Route
	bestPriority := -1

	for _, route := range e.routes {
		if !route.Enabled {
			continue
		}

		// 匹配方法
		if len(route.Methods) > 0 {
			methodMatch := false
			for _, m := range route.Methods {
				if m == method || m == "*" {
					methodMatch = true
					break
				}
			}
			if !methodMatch {
				continue
			}
		}

		// 匹配路径
		if !matchPath(route.Path, path) {
			continue
		}

		// 匹配主机
		if len(route.Hosts) > 0 {
			hostMatch := false
			for _, h := range route.Hosts {
				if h == host || h == "*" {
					hostMatch = true
					break
				}
			}
			if !hostMatch {
				continue
			}
		}

		// 选择优先级最高的
		if route.Priority > bestPriority {
			bestPriority = route.Priority
			bestMatch = route
		}
	}

	if bestMatch == nil {
		return nil, ErrRouteNotFound
	}

	return bestMatch, nil
}

// ========== 上游服务管理 ==========

// AddUpstream 添加上游服务.
func (e *Engine) AddUpstream(upstream *Upstream) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if upstream.ID == "" {
		return fmt.Errorf("上游服务ID不能为空")
	}

	e.upstreams[upstream.ID] = upstream
	e.stats.TotalUpstreams++
	return nil
}

// GetUpstream 获取上游服务.
func (e *Engine) GetUpstream(id string) (*Upstream, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	upstream, exists := e.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("上游服务不存在: %s", id)
	}
	return upstream, nil
}

// ListUpstreams 列出上游服务.
func (e *Engine) ListUpstreams() []*Upstream {
	e.mu.RLock()
	defer e.mu.RUnlock()

	upstreams := make([]*Upstream, 0, len(e.upstreams))
	for _, u := range e.upstreams {
		upstreams = append(upstreams, u)
	}
	return upstreams
}

// SelectTarget 选择目标（负载均衡）.
func (e *Engine) SelectTarget(upstreamID string) (*Target, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	upstream, exists := e.upstreams[upstreamID]
	if !exists {
		return nil, fmt.Errorf("上游服务不存在: %s", upstreamID)
	}

	// 筛选健康的目标
	healthy := make([]*Target, 0)
	for _, t := range upstream.Targets {
		if t.Health == "healthy" || t.Health == "unknown" {
			healthy = append(healthy, t)
		}
	}

	if len(healthy) == 0 {
		return nil, fmt.Errorf("没有可用的目标")
	}

	// 简单轮询
	idx := int(time.Now().UnixNano()) % len(healthy)
	return healthy[idx], nil
}

// ========== 策略管理 ==========

// AddPolicy 添加策略.
func (e *Engine) AddPolicy(policy *Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略ID不能为空")
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	e.policies[policy.ID] = policy

	// 根据策略类型初始化
	switch policy.Type {
	case PolicyTypeRateLimit:
		e.initRateLimiter(policy)
	case PolicyTypeCircuitBreaker:
		e.initCircuitBreaker(policy)
	}

	return nil
}

// GetPolicy 获取策略.
func (e *Engine) GetPolicy(id string) (*Policy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, exists := e.policies[id]
	if !exists {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

// ListPolicies 列出策略.
func (e *Engine) ListPolicies(policyType PolicyType) []*Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*Policy, 0)
	for _, p := range e.policies {
		if policyType == "" || p.Type == policyType {
			policies = append(policies, p)
		}
	}
	return policies
}

// DeletePolicy 删除策略.
func (e *Engine) DeletePolicy(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[id]; !exists {
		return ErrPolicyNotFound
	}

	delete(e.policies, id)
	delete(e.rateLimiters, id)
	delete(e.circuitBreakers, id)
	return nil
}

// ========== 插件管理 ==========

// RegisterPlugin 注册插件.
func (e *Engine) RegisterPlugin(plugin Plugin, config *PluginConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.plugins[plugin.Name()] = plugin
	if config != nil {
		e.pluginConfigs[plugin.Name()] = config
	}
}

// UnregisterPlugin 注销插件.
func (e *Engine) UnregisterPlugin(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.plugins, name)
	delete(e.pluginConfigs, name)
}

// ListPlugins 列出插件.
func (e *Engine) ListPlugins() []*PluginInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]*PluginInfo, 0)
	for name, plugin := range e.plugins {
		config := e.pluginConfigs[name]
		info := &PluginInfo{
			Name:        name,
			Description: plugin.Description(),
			Enabled:     config != nil && config.Enabled,
		}
		if config != nil {
			info.Routes = len(config.Routes)
		}
		infos = append(infos, info)
	}
	return infos
}

// ========== API版本管理 ==========

// AddAPIVersion 添加API版本.
func (e *Engine) AddAPIVersion(version *APIVersion) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.versions[version.Version] = version
}

// GetAPIVersion 获取API版本.
func (e *Engine) GetAPIVersion(version string) (*APIVersion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	v, exists := e.versions[version]
	if !exists {
		return nil, fmt.Errorf("API版本不存在: %s", version)
	}
	return v, nil
}

// ListAPIVersions 列出API版本.
func (e *Engine) ListAPIVersions() []*APIVersion {
	e.mu.RLock()
	defer e.mu.RUnlock()

	versions := make([]*APIVersion, 0, len(e.versions))
	for _, v := range e.versions {
		versions = append(versions, v)
	}
	return versions
}

// ========== 请求处理 ==========

// ProcessRequest 处理请求.
func (e *Engine) ProcessRequest(method, path, host, clientIP string) (*Route, *WAFResult, *RateLimitResult, error) {
	// WAF检查
	if e.waf.IsEnabled() {
		wafResult := e.waf.Check(clientIP, path, method, nil)
		if wafResult.Blocked {
			e.mu.Lock()
			e.stats.WAFBlocked++
			e.mu.Unlock()
			return nil, wafResult, nil, ErrWAFBlocked
		}
	}

	// 查找路由
	route, err := e.FindRoute(method, path, host)
	if err != nil {
		return nil, nil, nil, err
	}

	// 限流检查
	rateLimitResult, err := e.checkRateLimit(route, clientIP)
	if err != nil {
		e.mu.Lock()
		e.stats.RateLimited++
		e.mu.Unlock()
		return nil, nil, rateLimitResult, err
	}

	// 熔断检查
	if err := e.checkCircuitBreaker(route); err != nil {
		e.mu.Lock()
		e.stats.CircuitBroken++
		e.mu.Unlock()
		return nil, nil, rateLimitResult, err
	}

	return route, nil, rateLimitResult, nil
}

// LogRequest 记录请求日志.
func (e *Engine) LogRequest(log *RequestLog) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.requestLogs = append(e.requestLogs, log)
	if len(e.requestLogs) > e.maxLogs {
		e.requestLogs = e.requestLogs[len(e.requestLogs)-e.maxLogs:]
	}

	e.stats.TotalRequests++
	if log.StatusCode >= 400 {
		e.stats.TotalErrors++
	}
}

// GetRequestLogs 获取请求日志.
func (e *Engine) GetRequestLogs(limit int) []*RequestLog {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.requestLogs) {
		limit = len(e.requestLogs)
	}

	logs := make([]*RequestLog, limit)
	copy(logs, e.requestLogs[len(e.requestLogs)-limit:])
	return logs
}

// ClearRequestLogs 清空请求日志.
func (e *Engine) ClearRequestLogs() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requestLogs = make([]*RequestLog, 0)
}

// ========== 统计 ==========

// GetStats 获取统计.
func (e *Engine) GetStats() *GatewayStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	if e.running {
		stats.Uptime = time.Since(e.startTime)
	}
	stats.TotalRoutes = len(e.routes)
	stats.TotalUpstreams = len(e.upstreams)
	return &stats
}

// GetWAF 获取WAF.
func (e *Engine) GetWAF() *WAF {
	return e.waf
}

// GetOAuthServer 获取OAuth服务器.
func (e *Engine) GetOAuthServer() *OAuthServer {
	return e.oauthServer
}

// ========== 内部方法 ==========

// initRateLimiter 初始化限流器.
func (e *Engine) initRateLimiter(policy *Policy) {
	config, ok := policy.Config["rate_limit"]
	if !ok {
		return
	}

	// 从配置创建限流器
	if cfgMap, ok := config.(map[string]interface{}); ok {
		rps := 100
		if v, ok := cfgMap["requests_per_second"].(int); ok {
			rps = v
		}
		burst := 200
		if v, ok := cfgMap["burst"].(int); ok {
			burst = v
		}
		limiter := NewRateLimiter(AlgorithmTokenBucket, rps, burst)
		e.rateLimiters[policy.ID] = limiter
	}
}

// initCircuitBreaker 初始化熔断器.
func (e *Engine) initCircuitBreaker(policy *Policy) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		MaxRequests:      10,
	}

	if cfgMap, ok := policy.Config["circuit_breaker"].(map[string]interface{}); ok {
		if v, ok := cfgMap["failure_threshold"].(int); ok {
			config.FailureThreshold = v
		}
		if v, ok := cfgMap["timeout"].(time.Duration); ok {
			config.Timeout = v
		}
	}

	cb := NewCircuitBreaker(config)
	e.circuitBreakers[policy.ID] = cb
}

// checkRateLimit 检查限流.
func (e *Engine) checkRateLimit(route *Route, clientIP string) (*RateLimitResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, policy := range e.policies {
		if !policy.Enabled || policy.Type != PolicyTypeRateLimit {
			continue
		}

		// 检查策略是否适用于此路由
		if len(policy.Routes) > 0 {
			found := false
			for _, rid := range policy.Routes {
				if rid == route.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		limiter, exists := e.rateLimiters[policy.ID]
		if !exists {
			continue
		}

		key := clientIP + ":" + route.ID
		allowed := limiter.Allow(key)
		if !allowed {
			return &RateLimitResult{
				Allowed:   false,
				Limit:     limiter.limit,
				Remaining: 0,
				ResetAt:   time.Now().Add(time.Second).Unix(),
			}, ErrRateLimitExceeded
		}
	}

	return &RateLimitResult{Allowed: true}, nil
}

// checkCircuitBreaker 检查熔断器.
func (e *Engine) checkCircuitBreaker(route *Route) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, policy := range e.policies {
		if !policy.Enabled || policy.Type != PolicyTypeCircuitBreaker {
			continue
		}

		if len(policy.Routes) > 0 {
			found := false
			for _, rid := range policy.Routes {
				if rid == route.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		cb, exists := e.circuitBreakers[policy.ID]
		if !exists {
			continue
		}

		if !cb.Allow() {
			return ErrCircuitOpen
		}
	}

	return nil
}

// ========== 熔断器操作 ==========

// NewCircuitBreaker 创建熔断器.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// Allow 检查是否允许请求.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 检查是否可以进入半开状态
		if time.Now().After(cb.nextRetry) {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.successCount < cb.config.MaxRequests
	default:
		return false
	}
}

// RecordSuccess 记录成功.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalSuccesses++
	cb.failureCount = 0

	if cb.state == CircuitHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.successCount = 0
		}
	}
}

// RecordFailure 记录失败.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalFailures++
	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
		cb.nextRetry = time.Now().Add(cb.config.Timeout)
		return
	}

	if cb.failureCount >= cb.config.FailureThreshold {
		cb.state = CircuitOpen
		cb.nextRetry = time.Now().Add(cb.config.Timeout)
	}
}

// GetState 获取熔断器状态.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取熔断器统计.
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":           cb.state,
		"failure_count":   cb.failureCount,
		"success_count":   cb.successCount,
		"total_requests":  cb.totalRequests,
		"total_failures":  cb.totalFailures,
		"total_successes": cb.totalSuccesses,
	}
}

// matchPath 匹配路径.
func matchPath(pattern, path string) bool {
	if pattern == "*" || pattern == path {
		return true
	}

	// 简单前缀匹配
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}

	// 精确匹配
	return pattern == path
}
