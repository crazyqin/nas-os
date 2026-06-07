// Package clustermgr 提供分布式集群管理功能
package clustermgr

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// ServiceDiscoveryConfig 服务发现配置.
type ServiceDiscoveryConfig struct {
	// 服务注册配置
	RegisterEnabled  bool          `json:"registerEnabled"`  // 启用服务注册
	RegisterTTL      time.Duration `json:"registerTTL"`      // 注册TTL
	RegisterInterval time.Duration `json:"registerInterval"` // 注册间隔

	// 健康检查配置
	HealthCheckEnabled  bool          `json:"healthCheckEnabled"`  // 启用健康检查
	HealthCheckInterval time.Duration `json:"healthCheckInterval"` // 健康检查间隔
	HealthCheckTimeout  time.Duration `json:"healthCheckTimeout"`  // 健康检查超时
	HealthCheckPath     string        `json:"healthCheckPath"`     // 健康检查路径

	// 发现配置
	DiscoveryEnabled  bool          `json:"discoveryEnabled"`  // 启用服务发现
	DiscoveryInterval time.Duration `json:"discoveryInterval"` // 发现间隔
	CacheEnabled      bool          `json:"cacheEnabled"`      // 启用缓存
	CacheTTL          time.Duration `json:"cacheTTL"`          // 缓存TTL

	// gRPC配置
	GRPCEnabled bool `json:"grpcEnabled"` // 启用gRPC
	GRPCPort    int  `json:"grpcPort"`    // gRPC端口

	// HTTP配置
	HTTPEnabled bool `json:"httpEnabled"` // 启用HTTP
	HTTPPort    int  `json:"httpPort"`    // HTTP端口
}

// DefaultServiceDiscoveryConfig 返回默认服务发现配置.
func DefaultServiceDiscoveryConfig() ServiceDiscoveryConfig {
	return ServiceDiscoveryConfig{
		RegisterEnabled:     true,
		RegisterTTL:         time.Hour,
		RegisterInterval:    30 * time.Second,
		HealthCheckEnabled:  true,
		HealthCheckInterval: 15 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		HealthCheckPath:     "/health",
		DiscoveryEnabled:    true,
		DiscoveryInterval:   30 * time.Second,
		CacheEnabled:        true,
		CacheTTL:            5 * time.Minute,
		GRPCEnabled:         false,
		GRPCPort:            9090,
		HTTPEnabled:         true,
		HTTPPort:            8080,
	}
}

// ServiceRegistry 服务注册表.
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*ServiceInfo // 服务ID -> 服务信息
	index    uint64                  // 变更索引
}

// NewServiceRegistry 创建服务注册表.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*ServiceInfo),
	}
}

// Register 注册服务.
func (r *ServiceRegistry) Register(service *ServiceInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 设置注册时间
	service.RegisteredAt = time.Now()
	if service.ExpiresAt.IsZero() {
		service.ExpiresAt = time.Now().Add(time.Hour)
	}

	r.services[service.ID] = service
	r.index++

	log.Printf("[服务注册表] 注册服务: %s (%s:%d)", service.Name, service.Address, service.Port)
}

// Deregister 注销服务.
func (r *ServiceRegistry) Deregister(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.services[id]; ok {
		delete(r.services, id)
		r.index++
		log.Printf("[服务注册表] 注销服务: %s", id)
		return true
	}
	return false
}

// Get 获取服务.
func (r *ServiceRegistry) Get(id string) (*ServiceInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	service, ok := r.services[id]
	return service, ok
}

// List 列出所有服务.
func (r *ServiceRegistry) List() []*ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services := make([]*ServiceInfo, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	return services
}

// GetByName 按名称获取服务.
func (r *ServiceRegistry) GetByName(name string) []*ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var services []*ServiceInfo
	for _, service := range r.services {
		if service.Name == name {
			services = append(services, service)
		}
	}
	return services
}

// GetHealthy 获取健康的服务.
func (r *ServiceRegistry) GetHealthy() []*ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var services []*ServiceInfo
	for _, service := range r.services {
		if service.Healthy && !service.IsExpired() {
			services = append(services, service)
		}
	}
	return services
}

// Cleanup 清理过期服务.
func (r *ServiceRegistry) Cleanup() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for id, service := range r.services {
		if service.IsExpired() {
			delete(r.services, id)
			count++
		}
	}

	if count > 0 {
		r.index++
		log.Printf("[服务注册表] 清理过期服务: %d", count)
	}
	return count
}

// GetIndex 获取变更索引.
func (r *ServiceRegistry) GetIndex() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.index
}

// ServiceDiscoverer 服务发现器.
type ServiceDiscoverer struct {
	mu       sync.RWMutex
	config   ServiceDiscoveryConfig
	registry *ServiceRegistry
	cache    map[string]*serviceCacheEntry
	stopCh   chan struct{}
}

// serviceCacheEntry 服务缓存条目.
type serviceCacheEntry struct {
	services  []*ServiceInfo
	expiresAt time.Time
}

// NewServiceDiscoverer 创建服务发现器.
func NewServiceDiscoverer(config ServiceDiscoveryConfig, registry *ServiceRegistry) *ServiceDiscoverer {
	return &ServiceDiscoverer{
		config:   config,
		registry: registry,
		cache:    make(map[string]*serviceCacheEntry),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动服务发现器.
func (d *ServiceDiscoverer) Start(ctx context.Context) error {
	if !d.config.DiscoveryEnabled {
		return nil
	}

	// 启动健康检查
	if d.config.HealthCheckEnabled {
		go d.healthChecker(ctx)
	}

	// 启动缓存清理
	if d.config.CacheEnabled {
		go d.cacheCleaner(ctx)
	}

	log.Printf("[服务发现器] 启动成功")
	return nil
}

// Stop 停止服务发现器.
func (d *ServiceDiscoverer) Stop() {
	close(d.stopCh)
	log.Printf("[服务发现器] 已停止")
}

// Discover 发现服务.
func (d *ServiceDiscoverer) Discover(name string) ([]*ServiceInfo, error) {
	// 检查缓存
	if d.config.CacheEnabled {
		if cached, ok := d.getFromCache(name); ok {
			return cached, nil
		}
	}

	// 从注册表获取
	services := d.registry.GetByName(name)
	if len(services) == 0 {
		return nil, fmt.Errorf("未找到服务: %s", name)
	}

	// 更新缓存
	if d.config.CacheEnabled {
		d.updateCache(name, services)
	}

	return services, nil
}

// DiscoverHealthy 发现健康的服务.
func (d *ServiceDiscoverer) DiscoverHealthy(name string) ([]*ServiceInfo, error) {
	services, err := d.Discover(name)
	if err != nil {
		return nil, err
	}

	// 过滤健康的服务
	var healthy []*ServiceInfo
	for _, service := range services {
		if service.Healthy {
			healthy = append(healthy, service)
		}
	}

	if len(healthy) == 0 {
		return nil, fmt.Errorf("未找到健康的服务: %s", name)
	}

	return healthy, nil
}

// getFromCache 从缓存获取.
func (d *ServiceDiscoverer) getFromCache(name string) ([]*ServiceInfo, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entry, ok := d.cache[name]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.services, true
}

// updateCache 更新缓存.
func (d *ServiceDiscoverer) updateCache(name string, services []*ServiceInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.cache[name] = &serviceCacheEntry{
		services:  services,
		expiresAt: time.Now().Add(d.config.CacheTTL),
	}
}

// healthChecker 健康检查器.
func (d *ServiceDiscoverer) healthChecker(ctx context.Context) {
	ticker := time.NewTicker(d.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.checkAllServices()
		}
	}
}

// checkAllServices 检查所有服务健康状态.
func (d *ServiceDiscoverer) checkAllServices() {
	services := d.registry.List()

	for _, service := range services {
		go d.checkService(service)
	}
}

// checkService 检查单个服务健康状态.
func (d *ServiceDiscoverer) checkService(service *ServiceInfo) {
	var healthy bool

	switch service.Protocol {
	case ProtocolHTTP:
		healthy = d.checkHTTP(service)
	case ProtocolGRPC:
		healthy = d.checkGRPC(service)
	case ProtocolTCP:
		healthy = d.checkTCP(service)
	default:
		healthy = d.checkTCP(service)
	}

	service.UpdateHealth(healthy)
}

// checkHTTP HTTP健康检查.
func (d *ServiceDiscoverer) checkHTTP(service *ServiceInfo) bool {
	host := net.JoinHostPort(service.Address, fmt.Sprintf("%d", service.Port))
	url := fmt.Sprintf("http://%s%s", host, d.config.HealthCheckPath)

	client := &http.Client{
		Timeout: d.config.HealthCheckTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// checkGRPC gRPC健康检查.
func (d *ServiceDiscoverer) checkGRPC(service *ServiceInfo) bool {
	// 简化实现：尝试TCP连接
	return d.checkTCP(service)
}

// checkTCP TCP健康检查.
func (d *ServiceDiscoverer) checkTCP(service *ServiceInfo) bool {
	addr := net.JoinHostPort(service.Address, fmt.Sprintf("%d", service.Port))
	conn, err := net.DialTimeout("tcp", addr, d.config.HealthCheckTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// cacheCleaner 缓存清理器.
func (d *ServiceDiscoverer) cacheCleaner(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.cleanCache()
		}
	}
}

// cleanCache 清理过期缓存.
func (d *ServiceDiscoverer) cleanCache() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for name, entry := range d.cache {
		if now.After(entry.expiresAt) {
			delete(d.cache, name)
		}
	}
}

// ServiceRegistrar 服务注册器.
type ServiceRegistrar struct {
	mu       sync.RWMutex
	config   ServiceDiscoveryConfig
	registry *ServiceRegistry
	services map[string]*ServiceInfo // 本地注册的服务
	stopCh   chan struct{}
}

// NewServiceRegistrar 创建服务注册器.
func NewServiceRegistrar(config ServiceDiscoveryConfig, registry *ServiceRegistry) *ServiceRegistrar {
	return &ServiceRegistrar{
		config:   config,
		registry: registry,
		services: make(map[string]*ServiceInfo),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动服务注册器.
func (r *ServiceRegistrar) Start(ctx context.Context) error {
	if !r.config.RegisterEnabled {
		return nil
	}

	// 启动注册续期
	go r.registrationRenewer(ctx)

	log.Printf("[服务注册器] 启动成功")
	return nil
}

// Stop 停止服务注册器.
func (r *ServiceRegistrar) Stop() {
	close(r.stopCh)

	// 注销所有本地服务
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id := range r.services {
		r.registry.Deregister(id)
	}

	log.Printf("[服务注册器] 已停止")
}

// Register 注册服务.
func (r *ServiceRegistrar) Register(service *ServiceInfo) error {
	if service.ID == "" {
		service.ID = generateServiceID(service.Name, service.Address, service.Port)
	}

	// 设置过期时间
	service.ExpiresAt = time.Now().Add(r.config.RegisterTTL)

	// 注册到注册表
	r.registry.Register(service)

	// 保存到本地服务列表
	r.mu.Lock()
	r.services[service.ID] = service
	r.mu.Unlock()

	return nil
}

// Deregister 注销服务.
func (r *ServiceRegistrar) Deregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.services[id]; ok {
		delete(r.services, id)
		r.registry.Deregister(id)
		return nil
	}

	return fmt.Errorf("服务不存在: %s", id)
}

// registrationRenewer 注册续期器.
func (r *ServiceRegistrar) registrationRenewer(ctx context.Context) {
	ticker := time.NewTicker(r.config.RegisterInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.renewRegistrations()
		}
	}
}

// renewRegistrations 续期注册.
func (r *ServiceRegistrar) renewRegistrations() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, service := range r.services {
		// 更新过期时间
		service.ExpiresAt = time.Now().Add(r.config.RegisterTTL)
	}
}

// generateServiceID 生成服务ID.
func generateServiceID(name, address string, port int) string {
	return fmt.Sprintf("%s-%s-%d-%d", name, address, port, time.Now().UnixNano())
}

// ServiceDiscoveryManager 服务发现管理器.
type ServiceDiscoveryManager struct {
	mu         sync.RWMutex
	config     ServiceDiscoveryConfig
	registry   *ServiceRegistry
	discoverer *ServiceDiscoverer
	registrar  *ServiceRegistrar
}

// NewServiceDiscoveryManager 创建服务发现管理器.
func NewServiceDiscoveryManager(config ServiceDiscoveryConfig) *ServiceDiscoveryManager {
	registry := NewServiceRegistry()
	discoverer := NewServiceDiscoverer(config, registry)
	registrar := NewServiceRegistrar(config, registry)

	return &ServiceDiscoveryManager{
		config:     config,
		registry:   registry,
		discoverer: discoverer,
		registrar:  registrar,
	}
}

// Start 启动服务发现管理器.
func (m *ServiceDiscoveryManager) Start(ctx context.Context) error {
	if err := m.discoverer.Start(ctx); err != nil {
		return fmt.Errorf("启动服务发现器失败: %w", err)
	}

	if err := m.registrar.Start(ctx); err != nil {
		return fmt.Errorf("启动服务注册器失败: %w", err)
	}

	log.Printf("[服务发现管理器] 启动成功")
	return nil
}

// Stop 停止服务发现管理器.
func (m *ServiceDiscoveryManager) Stop() {
	m.discoverer.Stop()
	m.registrar.Stop()
	log.Printf("[服务发现管理器] 已停止")
}

// Register 注册服务.
func (m *ServiceDiscoveryManager) Register(service *ServiceInfo) error {
	return m.registrar.Register(service)
}

// Deregister 注销服务.
func (m *ServiceDiscoveryManager) Deregister(id string) error {
	return m.registrar.Deregister(id)
}

// Discover 发现服务.
func (m *ServiceDiscoveryManager) Discover(name string) ([]*ServiceInfo, error) {
	return m.discoverer.Discover(name)
}

// DiscoverHealthy 发现健康的服务.
func (m *ServiceDiscoveryManager) DiscoverHealthy(name string) ([]*ServiceInfo, error) {
	return m.discoverer.DiscoverHealthy(name)
}

// GetRegistry 获取服务注册表.
func (m *ServiceDiscoveryManager) GetRegistry() *ServiceRegistry {
	return m.registry
}

// GetConfig 获取配置.
func (m *ServiceDiscoveryManager) GetConfig() ServiceDiscoveryConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *ServiceDiscoveryManager) UpdateConfig(config ServiceDiscoveryConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}
