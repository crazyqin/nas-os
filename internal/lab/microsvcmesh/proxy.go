// Package microsvcmesh 代理层，支持请求路由和协议转换
package microsvcmesh

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Proxy 代理层.
type Proxy struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	engine     *Engine
	rrCounters map[string]*int64 // service -> round-robin counter
}

// NewProxy 创建代理层.
func NewProxy(logger *zap.Logger, engine *Engine) *Proxy {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Proxy{
		logger:     logger,
		engine:     engine,
		rrCounters: make(map[string]*int64),
	}
}

// HandleRequest 处理代理请求.
func (p *Proxy) HandleRequest(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	start := time.Now()

	// 查找服务
	p.engine.mu.RLock()
	var svc *Service
	for _, s := range p.engine.services {
		if s.Name == req.Service {
			svc = s
			break
		}
	}
	p.engine.mu.RUnlock()

	if svc == nil {
		return nil, fmt.Errorf("service %s not found", req.Service)
	}

	// 检查熔断器
	p.engine.mu.RLock()
	key := p.engine.serviceKey(svc.Name, svc.Namespace)
	cb, hasBreaker := p.engine.breakers[key]
	p.engine.mu.RUnlock()

	if hasBreaker && !cb.Allow() {
		return nil, fmt.Errorf("circuit breaker open for service %s", req.Service)
	}

	// 选择端点
	endpoint, strategy := p.selectEndpoint(svc, req)
	if endpoint == nil {
		if hasBreaker {
			cb.RecordFailure()
		}
		return nil, fmt.Errorf("no available endpoint for service %s", req.Service)
	}

	// 模拟请求转发
	response := &ProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"X-Service":  svc.Name,
			"X-Endpoint": endpoint.ID,
			"X-Strategy": string(strategy),
		},
		Body:     []byte("OK"),
		Duration: time.Since(start),
		Upstream: fmt.Sprintf("%s://%s:%d", endpoint.Protocol, endpoint.Host, endpoint.Port),
	}

	// 记录成功
	if hasBreaker {
		cb.RecordSuccess()
	}

	// 记录追踪
	if p.engine.config.TracingEnabled {
		span := p.engine.tracer.StartSpan(req.Path, svc.Name)
		span.Tags["method"] = req.Method
		span.Tags["endpoint"] = endpoint.ID
		span.Status = "ok"
		span.EndTime = time.Now()
		span.Duration = span.EndTime.Sub(span.StartTime)
		p.engine.tracer.RecordSpan(span)
	}

	// 记录指标
	if p.engine.config.MetricsEnabled {
		p.engine.metrics.Record(MetricPoint{
			Name:   "request_duration",
			Type:   "histogram",
			Value:  float64(response.Duration.Milliseconds()),
			Labels: map[string]string{"service": svc.Name, "status": "200"},
		})
	}

	return response, nil
}

// selectEndpoint 选择端点.
func (p *Proxy) selectEndpoint(svc *Service, req *ProxyRequest) (*Endpoint, RouteStrategy) {
	// 查找匹配的路由
	route := p.findRoute(svc, req)
	strategy := RouteRoundRobin
	if route != nil {
		strategy = route.Strategy
	}

	// 过滤可用端点
	available := make([]*Endpoint, 0)
	for _, ep := range svc.Endpoints {
		if ep.Status == ServiceStatusHealthy {
			available = append(available, ep)
		}
	}

	if len(available) == 0 {
		return nil, strategy
	}

	switch strategy {
	case RouteRoundRobin:
		return p.roundRobin(svc.Name, available), strategy
	case RouteWeighted:
		return p.weightedSelect(available), strategy
	case RouteRandom:
		return available[rand.Intn(len(available))], strategy
	case RouteLeastConn:
		// 简化：使用 round-robin 代替
		return p.roundRobin(svc.Name, available), strategy
	case RouteSticky:
		// 简化：基于请求路径的 hash
		idx := hashString(req.Path) % len(available)
		return available[idx], strategy
	default:
		return p.roundRobin(svc.Name, available), strategy
	}
}

// roundRobin 轮询选择.
func (p *Proxy) roundRobin(serviceName string, endpoints []*Endpoint) *Endpoint {
	p.mu.Lock()
	counter, exists := p.rrCounters[serviceName]
	if !exists {
		c := int64(0)
		counter = &c
		p.rrCounters[serviceName] = counter
	}
	p.mu.Unlock()

	idx := atomic.AddInt64(counter, 1)
	return endpoints[int(idx)%len(endpoints)]
}

// weightedSelect 权重选择.
func (p *Proxy) weightedSelect(endpoints []*Endpoint) *Endpoint {
	totalWeight := 0
	for _, ep := range endpoints {
		totalWeight += ep.Weight
	}
	if totalWeight == 0 {
		return endpoints[rand.Intn(len(endpoints))]
	}

	r := rand.Intn(totalWeight)
	for _, ep := range endpoints {
		r -= ep.Weight
		if r < 0 {
			return ep
		}
	}
	return endpoints[0]
}

// findRoute 查找匹配的路由.
func (p *Proxy) findRoute(svc *Service, req *ProxyRequest) *Route {
	for _, route := range svc.Routes {
		if route.Prefix != "" && !matchPrefix(req.Path, route.Prefix) {
			continue
		}
		if len(route.Methods) > 0 && !contains(route.Methods, req.Method) {
			continue
		}
		return route
	}
	return nil
}

// matchPrefix 前缀匹配.
func matchPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}

// contains 检查切片是否包含元素.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// hashString 字符串哈希.
func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
