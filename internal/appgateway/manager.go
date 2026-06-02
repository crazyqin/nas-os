package appgateway

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// NewManager 创建应用网关管理器
func NewManager(config *ProxyConfig) *Manager {
	if config == nil {
		config = DefaultProxyConfig()
	}

	return &Manager{
		config:        config,
		apps:          make(map[string]*Application),
		routes:        make(map[string]*RouteRule),
		lbConfig:      &LoadBalancerConfig{Algorithm: AlgorithmRoundRobin},
		requestLogs:   make([]*AccessLog, 0, config.LogMaxSize),
		stats:         &GatewayStats{},
		startTime:     time.Now(),
		stopChan:      make(chan struct{}),
		roundRobinIdx: make(map[string]int),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("app gateway already running")
	}

	m.running = true
	m.startTime = time.Now()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)
	return nil
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ==================== 应用管理 ====================

// RegisterApp 注册应用
func (m *Manager) RegisterApp(app *Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if app.ID == "" {
		app.ID = generateID()
	}

	if app.Name == "" {
		return fmt.Errorf("application name is required")
	}

	if app.Port <= 0 || app.Port > 65535 {
		return fmt.Errorf("invalid port: %d", app.Port)
	}

	if _, exists := m.apps[app.ID]; exists {
		return fmt.Errorf("application %s already exists", app.ID)
	}

	// 设置默认值
	if app.Protocol == "" {
		app.Protocol = "http"
	}
	if app.HealthCheck == nil {
		app.HealthCheck = &HealthCheckConfig{
			Enabled:           true,
			Path:              "/health",
			Interval:          30 * time.Second,
			Timeout:           5 * time.Second,
			HealthyCodes:      []int{200},
			UnhealthyThreshold: 3,
			HealthyThreshold:   2,
		}
	}
	if app.Instances == nil {
		app.Instances = []AppInstance{
			{
				ID:     generateID(),
				Host:   "localhost",
				Port:   app.Port,
				Weight: 1,
				Health: "unknown",
			},
		}
	}

	app.Enabled = true
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()

	m.apps[app.ID] = app
	m.stats.TotalApps = len(m.apps)
	m.updateActiveCount()

	return nil
}

// UnregisterApp 注销应用
func (m *Manager) UnregisterApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.apps[id]; !exists {
		return fmt.Errorf("application %s not found", id)
	}

	// 删除关联的路由
	for routeID, route := range m.routes {
		if route.AppID == id {
			delete(m.routes, routeID)
		}
	}

	delete(m.apps, id)
	delete(m.roundRobinIdx, id)
	m.stats.TotalApps = len(m.apps)
	m.stats.TotalRoutes = len(m.routes)
	m.updateActiveCount()

	return nil
}

// GetApp 获取应用
func (m *Manager) GetApp(id string) (*Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.apps[id]
	if !exists {
		return nil, fmt.Errorf("application %s not found", id)
	}
	return app, nil
}

// UpdateApp 更新应用
func (m *Manager) UpdateApp(app *Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.apps[app.ID]
	if !exists {
		return fmt.Errorf("application %s not found", app.ID)
	}

	if app.Name != "" {
		existing.Name = app.Name
	}
	if app.Description != "" {
		existing.Description = app.Description
	}
	if app.Domain != "" {
		existing.Domain = app.Domain
	}
	if app.Path != "" {
		existing.Path = app.Path
	}
	if app.Port > 0 {
		existing.Port = app.Port
	}
	if app.Protocol != "" {
		existing.Protocol = app.Protocol
	}
	if app.HealthCheck != nil {
		existing.HealthCheck = app.HealthCheck
	}
	if app.Access != nil {
		existing.Access = app.Access
	}
	if app.SSL != nil {
		existing.SSL = app.SSL
	}

	existing.UpdatedAt = time.Now()

	return nil
}

// ListApps 列出所有应用
func (m *Manager) ListApps() []*Application {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*Application, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, app)
	}
	return apps
}

// EnableApp 启用应用
func (m *Manager) EnableApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return fmt.Errorf("application %s not found", id)
	}

	app.Enabled = true
	app.UpdatedAt = time.Now()
	m.updateActiveCount()

	return nil
}

// DisableApp 禁用应用
func (m *Manager) DisableApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return fmt.Errorf("application %s not found", id)
	}

	app.Enabled = false
	app.UpdatedAt = time.Now()
	m.updateActiveCount()

	return nil
}

// ==================== 实例管理 ====================

// AddInstance 添加应用实例
func (m *Manager) AddInstance(appID string, instance *AppInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[appID]
	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	if instance.ID == "" {
		instance.ID = generateID()
	}
	if instance.Weight <= 0 {
		instance.Weight = 1
	}
	instance.Health = "unknown"

	app.Instances = append(app.Instances, *instance)
	app.UpdatedAt = time.Now()

	return nil
}

// RemoveInstance 移除应用实例
func (m *Manager) RemoveInstance(appID, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[appID]
	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	for i, inst := range app.Instances {
		if inst.ID == instanceID {
			app.Instances = append(app.Instances[:i], app.Instances[i+1:]...)
			app.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("instance %s not found in application %s", instanceID, appID)
}

// UpdateInstanceHealth 更新实例健康状态
func (m *Manager) UpdateInstanceHealth(appID, instanceID, health string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[appID]
	if !exists {
		return fmt.Errorf("application %s not found", appID)
	}

	for i, inst := range app.Instances {
		if inst.ID == instanceID {
			app.Instances[i].Health = health
			app.Instances[i].LastCheck = time.Now()
			app.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("instance %s not found in application %s", instanceID, appID)
}

// ==================== 路由管理 ====================

// AddRoute 添加路由规则
func (m *Manager) AddRoute(route *RouteRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if route.ID == "" {
		route.ID = generateID()
	}

	if route.AppID == "" {
		return fmt.Errorf("app_id is required")
	}

	if _, exists := m.apps[route.AppID]; !exists {
		return fmt.Errorf("application %s not found", route.AppID)
	}

	if route.Domain == "" && route.Path == "" {
		return fmt.Errorf("domain or path is required")
	}

	if _, exists := m.routes[route.ID]; exists {
		return fmt.Errorf("route %s already exists", route.ID)
	}

	route.Enabled = true
	route.CreatedAt = time.Now()
	route.UpdatedAt = time.Now()

	m.routes[route.ID] = route
	m.stats.TotalRoutes = len(m.routes)

	return nil
}

// UpdateRoute 更新路由规则
func (m *Manager) UpdateRoute(route *RouteRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.routes[route.ID]; !exists {
		return fmt.Errorf("route %s not found", route.ID)
	}

	route.UpdatedAt = time.Now()
	m.routes[route.ID] = route

	return nil
}

// DeleteRoute 删除路由规则
func (m *Manager) DeleteRoute(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.routes[id]; !exists {
		return fmt.Errorf("route %s not found", id)
	}

	delete(m.routes, id)
	m.stats.TotalRoutes = len(m.routes)

	return nil
}

// GetRoute 获取路由规则
func (m *Manager) GetRoute(id string) (*RouteRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	route, exists := m.routes[id]
	if !exists {
		return nil, fmt.Errorf("route %s not found", id)
	}
	return route, nil
}

// ListRoutes 列出路由规则
func (m *Manager) ListRoutes() []*RouteRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make([]*RouteRule, 0, len(m.routes))
	for _, r := range m.routes {
		routes = append(routes, r)
	}
	return routes
}

// MatchRoute 匹配路由
func (m *Manager) MatchRoute(host, path string) (*RouteRule, *Application) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bestMatch *RouteRule
	bestPriority := -1

	for _, route := range m.routes {
		if !route.Enabled {
			continue
		}

		// 检查域名匹配
		if route.Domain != "" && !matchDomain(route.Domain, host) {
			continue
		}

		// 检查路径匹配
		if route.Path != "" && !strings.HasPrefix(path, route.Path) {
			continue
		}

		// 选择优先级最高的匹配
		if route.Priority > bestPriority {
			bestPriority = route.Priority
			bestMatch = route
		}
	}

	if bestMatch == nil {
		return nil, nil
	}

	app, exists := m.apps[bestMatch.AppID]
	if !exists || !app.Enabled {
		return nil, nil
	}

	return bestMatch, app
}

// matchDomain 匹配域名
func matchDomain(pattern, host string) bool {
	// 精确匹配
	if pattern == host {
		return true
	}

	// 通配符匹配 *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // .example.com
		return strings.HasSuffix(host, suffix)
	}

	return false
}

// ==================== 负载均衡 ====================

// SelectInstance 选择实例（负载均衡）
func (m *Manager) SelectInstance(app *Application, clientIP string) (*AppInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 过滤健康的实例
	healthy := make([]AppInstance, 0)
	for _, inst := range app.Instances {
		if inst.Health != "unhealthy" {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy instances for app %s", app.ID)
	}

	var selected *AppInstance

	switch m.lbConfig.Algorithm {
	case AlgorithmWeighted:
		selected = selectWeighted(healthy)
	case AlgorithmIPHash:
		selected = selectIPHash(healthy, clientIP)
	case AlgorithmRandom:
		selected = &healthy[rand.Intn(len(healthy))]
	default: // round-robin
		idx := m.roundRobinIdx[app.ID]
		selected = &healthy[idx%len(healthy)]
		m.roundRobinIdx[app.ID] = idx + 1
	}

	return selected, nil
}

// selectWeighted 加权选择
func selectWeighted(instances []AppInstance) *AppInstance {
	totalWeight := 0
	for _, inst := range instances {
		totalWeight += inst.Weight
	}
	if totalWeight == 0 {
		return &instances[rand.Intn(len(instances))]
	}

	r := rand.Intn(totalWeight)
	for i := range instances {
		r -= instances[i].Weight
		if r < 0 {
			return &instances[i]
		}
	}
	return &instances[0]
}

// selectIPHash IP哈希选择
func selectIPHash(instances []AppInstance, clientIP string) *AppInstance {
	hash := 0
	for _, c := range clientIP {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return &instances[hash%len(instances)]
}

// SetLoadBalancerAlgorithm 设置负载均衡算法
func (m *Manager) SetLoadBalancerAlgorithm(algo LoadBalancerAlgorithm) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lbConfig.Algorithm = algo
}

// GetLoadBalancerAlgorithm 获取负载均衡算法
func (m *Manager) GetLoadBalancerAlgorithm() LoadBalancerAlgorithm {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lbConfig.Algorithm
}

// ==================== 访问控制 ====================

// CheckAccess 检查访问权限
func (m *Manager) CheckAccess(app *Application, clientIP string) error {
	if app.Access == nil {
		return nil
	}

	access := app.Access

	// 检查IP黑名单
	for _, blocked := range access.BlockedIPs {
		if matchIP(blocked, clientIP) {
			return fmt.Errorf("access denied: IP %s is blocked", clientIP)
		}
	}

	// 检查IP白名单
	if len(access.AllowedIPs) > 0 {
		allowed := false
		for _, allowedIP := range access.AllowedIPs {
			if matchIP(allowedIP, clientIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("access denied: IP %s is not allowed", clientIP)
		}
	}

	return nil
}

// matchIP 匹配IP地址
func matchIP(pattern, ip string) bool {
	// 精确匹配
	if pattern == ip {
		return true
	}

	// CIDR匹配
	_, cidr, err := net.ParseCIDR(pattern)
	if err != nil {
		return false
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return cidr.Contains(parsedIP)
}

// CheckAPIKey 检查API Key
func (m *Manager) CheckAPIKey(app *Application, apiKey string) error {
	if app.Access == nil || app.Access.APIKey == "" {
		return nil
	}

	if apiKey != app.Access.APIKey {
		return fmt.Errorf("invalid API key")
	}

	return nil
}

// CheckBasicAuth 检查Basic认证
func (m *Manager) CheckBasicAuth(app *Application, username, password string) error {
	if app.Access == nil || app.Access.BasicAuth == nil {
		return nil
	}

	ba := app.Access.BasicAuth
	if username != ba.Username || password != ba.Password {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

// ==================== 健康检查 ====================

// CheckHealth 检查应用健康状态
func (m *Manager) CheckHealth(appID string) (map[string]string, error) {
	m.mu.RLock()
	app, exists := m.apps[appID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	if app.HealthCheck == nil || !app.HealthCheck.Enabled {
		return nil, fmt.Errorf("health check not configured for app %s", appID)
	}

	result := make(map[string]string)
	for _, inst := range app.Instances {
		result[inst.ID] = inst.Health
	}

	return result, nil
}

// GetHealthyInstances 获取健康实例
func (m *Manager) GetHealthyInstances(appID string) ([]AppInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.apps[appID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", appID)
	}

	healthy := make([]AppInstance, 0)
	for _, inst := range app.Instances {
		if inst.Health != "unhealthy" {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// ==================== 请求日志 ====================

// LogRequest 记录请求日志
func (m *Manager) LogRequest(log *AccessLog) {
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
	atomic.AddInt64(&m.stats.BytesReceived, log.RequestSize)
	atomic.AddInt64(&m.stats.BytesSent, log.ResponseSize)
}

// GetRequestLogs 获取请求日志
func (m *Manager) GetRequestLogs(limit int) []*AccessLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.requestLogs) {
		limit = len(m.requestLogs)
	}

	start := len(m.requestLogs) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]*AccessLog, limit)
	copy(logs, m.requestLogs[start:])
	return logs
}

// ClearRequestLogs 清除请求日志
func (m *Manager) ClearRequestLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestLogs = make([]*AccessLog, 0, m.config.LogMaxSize)
}

// ==================== 统计 ====================

// GetStats 获取网关统计
func (m *Manager) GetStats() *GatewayStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.Uptime = time.Since(m.startTime)
	stats.TotalApps = len(m.apps)
	stats.TotalRoutes = len(m.routes)

	// 计算活跃应用数
	activeCount := 0
	for _, app := range m.apps {
		if app.Enabled {
			activeCount++
		}
	}
	stats.ActiveApps = activeCount

	return &stats
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *ProxyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// ==================== 辅助函数 ====================

// updateActiveCount 更新活跃应用计数
func (m *Manager) updateActiveCount() {
	count := 0
	for _, app := range m.apps {
		if app.Enabled {
			count++
		}
	}
	m.stats.ActiveApps = count
}

// generateID 生成唯一 ID
func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	id := make([]byte, 16)
	for i := range id {
		id[i] = chars[rand.Intn(len(chars))]
	}
	return string(id)
}
