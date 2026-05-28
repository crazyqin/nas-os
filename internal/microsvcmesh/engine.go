// Package microsvcmesh 服务网格引擎
package microsvcmesh

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Engine 服务网格引擎
type Engine struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *Config
	services map[string]*Service
	proxy    *Proxy
	breakers map[string]*CircuitBreaker // service -> circuit breaker
	tracer   *Tracer
	metrics  *MetricsCollector
	stopCh   chan struct{}
	running  bool
}

// NewEngine 创建服务网格引擎
func NewEngine(logger *zap.Logger, config *Config) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	e := &Engine{
		logger:   logger,
		config:   config,
		services: make(map[string]*Service),
		breakers: make(map[string]*CircuitBreaker),
		stopCh:   make(chan struct{}),
	}

	e.proxy = NewProxy(logger, e)
	e.tracer = NewTracer(logger, config.TracingSampleRate)
	e.metrics = NewMetricsCollector(logger)

	return e
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.mu.Unlock()

	e.logger.Info("starting service mesh engine",
		zap.String("addr", e.config.ListenAddr),
		zap.Bool("tracing", e.config.TracingEnabled),
		zap.Bool("metrics", e.config.MetricsEnabled),
	)

	// 启动健康检查
	go e.healthCheckLoop(ctx)

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.running = false
	close(e.stopCh)
	e.logger.Info("service mesh engine stopped")
}

// IsRunning 是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RegisterService 注册服务
func (e *Engine) RegisterService(svc *Service) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if svc.Name == "" {
		return fmt.Errorf("service name is required")
	}

	key := e.serviceKey(svc.Name, svc.Namespace)
	if _, exists := e.services[key]; exists {
		return fmt.Errorf("service %s already registered", key)
	}

	now := time.Now()
	svc.CreatedAt = now
	svc.UpdatedAt = now
	if svc.Status == "" {
		svc.Status = ServiceStatusHealthy
	}
	if svc.Endpoints == nil {
		svc.Endpoints = make([]*Endpoint, 0)
	}
	if svc.Policies == nil {
		svc.Policies = make(map[string]string)
	}
	if svc.Metadata == nil {
		svc.Metadata = make(map[string]string)
	}

	e.services[key] = svc

	// 确保端点状态初始化
	for _, ep := range svc.Endpoints {
		if ep.Status == "" {
			ep.Status = ServiceStatusHealthy
		}
		if ep.Weight <= 0 {
			ep.Weight = 1
		}
	}

	// 创建熔断器
	e.breakers[key] = NewCircuitBreaker(e.logger, svc.Name, DefaultCircuitBreakerConfig())

	e.logger.Info("service registered",
		zap.String("name", svc.Name),
		zap.String("namespace", svc.Namespace),
		zap.Int("endpoints", len(svc.Endpoints)),
	)
	return nil
}

// DeregisterService 注销服务
func (e *Engine) DeregisterService(name, namespace string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.serviceKey(name, namespace)
	if _, exists := e.services[key]; !exists {
		return fmt.Errorf("service %s not found", key)
	}

	delete(e.services, key)
	delete(e.breakers, key)

	e.logger.Info("service deregistered", zap.String("name", name))
	return nil
}

// GetService 获取服务
func (e *Engine) GetService(name, namespace string) (*Service, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := e.serviceKey(name, namespace)
	svc, exists := e.services[key]
	if !exists {
		return nil, fmt.Errorf("service %s not found", key)
	}
	return svc, nil
}

// ListServices 列出所有服务
func (e *Engine) ListServices(namespace string) []*Service {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Service, 0)
	for _, svc := range e.services {
		if namespace == "" || svc.Namespace == namespace {
			result = append(result, svc)
		}
	}
	return result
}

// AddEndpoint 添加端点
func (e *Engine) AddEndpoint(serviceName, namespace string, ep *Endpoint) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.serviceKey(serviceName, namespace)
	svc, exists := e.services[key]
	if !exists {
		return fmt.Errorf("service %s not found", key)
	}

	if ep.ID == "" {
		ep.ID = generateID()
	}
	if ep.Status == "" {
		ep.Status = ServiceStatusHealthy
	}
	if ep.Weight <= 0 {
		ep.Weight = 1
	}
	if ep.Tags == nil {
		ep.Tags = make(map[string]string)
	}

	svc.Endpoints = append(svc.Endpoints, ep)
	svc.UpdatedAt = time.Now()

	e.logger.Info("endpoint added",
		zap.String("service", serviceName),
		zap.String("endpoint_id", ep.ID),
		zap.String("host", ep.Host),
		zap.Int("port", ep.Port),
	)
	return nil
}

// RemoveEndpoint 移除端点
func (e *Engine) RemoveEndpoint(serviceName, namespace, endpointID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.serviceKey(serviceName, namespace)
	svc, exists := e.services[key]
	if !exists {
		return fmt.Errorf("service %s not found", key)
	}

	for i, ep := range svc.Endpoints {
		if ep.ID == endpointID {
			svc.Endpoints = append(svc.Endpoints[:i], svc.Endpoints[i+1:]...)
			svc.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("endpoint %s not found", endpointID)
}

// AddRoute 添加路由规则
func (e *Engine) AddRoute(serviceName, namespace string, route *Route) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.serviceKey(serviceName, namespace)
	svc, exists := e.services[key]
	if !exists {
		return fmt.Errorf("service %s not found", key)
	}

	if route.ID == "" {
		route.ID = generateID()
	}
	svc.Routes = append(svc.Routes, route)
	svc.UpdatedAt = time.Now()

	e.logger.Info("route added",
		zap.String("service", serviceName),
		zap.String("route_id", route.ID),
		zap.String("prefix", route.Prefix),
	)
	return nil
}

// ProxyRequest 代理请求
func (e *Engine) ProxyRequest(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	return e.proxy.HandleRequest(ctx, req)
}

// GetCircuitBreakerState 获取熔断器状态
func (e *Engine) GetCircuitBreakerState(serviceName, namespace string) (CircuitState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := e.serviceKey(serviceName, namespace)
	cb, exists := e.breakers[key]
	if !exists {
		return "", fmt.Errorf("service %s not found", key)
	}

	return cb.State(), nil
}

// healthCheckLoop 健康检查循环
func (e *Engine) healthCheckLoop(ctx context.Context) {
	interval := time.Duration(e.config.HealthCheckInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.runHealthChecks()
		}
	}
}

// runHealthChecks 运行所有服务的健康检查
func (e *Engine) runHealthChecks() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, svc := range e.services {
		for _, ep := range svc.Endpoints {
			// 简化实现：模拟健康检查
			if ep.Status == ServiceStatusUnhealthy {
				e.logger.Debug("endpoint still unhealthy",
					zap.String("service", svc.Name),
					zap.String("endpoint", ep.ID),
				)
			}
		}
	}
}

// GetStats 获取引擎统计
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalEndpoints := 0
	for _, svc := range e.services {
		totalEndpoints += len(svc.Endpoints)
	}

	return map[string]interface{}{
		"services":        len(e.services),
		"total_endpoints": totalEndpoints,
		"circuit_breakers": len(e.breakers),
		"tracing_enabled":  e.config.TracingEnabled,
		"metrics_enabled":  e.config.MetricsEnabled,
	}
}

// serviceKey 生成服务键
func (e *Engine) serviceKey(name, namespace string) string {
	if namespace == "" {
		namespace = "default"
	}
	return namespace + "/" + name
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
