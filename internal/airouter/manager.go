package airouter

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Manager 路由管理器
type Manager struct {
	routes  sync.Map // routeID -> *RouteConfig
	metrics sync.Map // modelID -> *modelMetricsData
	stats   sync.Map // routeID -> *routeStatsData
	mu      sync.RWMutex
}

type modelMetricsData struct {
	metrics   ModelMetrics
	latencies []int64
	mu        sync.Mutex
}

type routeStatsData struct {
	stats             RouteStats
	modelDistribution map[string]int64
	mu                sync.Mutex
}

// NewManager 创建路由管理器
func NewManager() *Manager {
	return &Manager{}
}

// CreateRoute 创建路由配置
func (m *Manager) CreateRoute(req *CreateRouteRequest) (*RouteConfig, error) {
	route := &RouteConfig{
		ID:              fmt.Sprintf("route_%d", time.Now().UnixNano()),
		Name:            req.Name,
		Strategy:        req.Strategy,
		FallbackEnabled: req.FallbackEnabled,
		MaxRetries:      req.MaxRetries,
		TimeoutMs:       req.TimeoutMs,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	for _, modelID := range req.ModelIDs {
		route.Models = append(route.Models, ModelEntry{
			ModelID:  modelID,
			Weight:   1,
			Priority: 1,
			Health:   HealthHealthy,
		})
	}

	m.routes.Store(route.ID, route)
	m.stats.Store(route.ID, &routeStatsData{
		stats: RouteStats{
			RouteID:           route.ID,
			ModelDistribution: make(map[string]int64),
		},
		modelDistribution: make(map[string]int64),
	})

	return route, nil
}

// Select 选择模型
func (m *Manager) Select(ctx context.Context, req *RouteRequest) (*RouteDecision, error) {
	start := time.Now()

	routeObj, ok := m.routes.Load(req.RouteID)
	if !ok {
		return nil, fmt.Errorf("route %s not found", req.RouteID)
	}
	route := routeObj.(*RouteConfig)

	if !route.Enabled {
		return nil, fmt.Errorf("route %s is disabled", req.RouteID)
	}

	var decision *RouteDecision

	switch route.Strategy {
	case StrategyRoundRobin:
		decision = m.selectRoundRobin(route)
	case StrategyLeastLatency:
		decision = m.selectLeastLatency(route)
	case StrategyCostOptimized:
		decision = m.selectCostOptimized(route)
	case StrategyQualityFirst:
		decision = m.selectQualityFirst(route)
	case StrategyFailover:
		decision = m.selectFailover(route)
	case StrategyWeighted:
		decision = m.selectWeighted(route)
	default:
		decision = m.selectRoundRobin(route)
	}

	decision.DecisionMs = time.Since(start).Milliseconds()
	return decision, nil
}

func (m *Manager) selectRoundRobin(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	idx := rand.Intn(len(healthyModels))
	selected := healthyModels[idx]

	alternatives := make([]string, 0, len(healthyModels)-1)
	for i, model := range healthyModels {
		if i != idx {
			alternatives = append(alternatives, model.ModelID)
		}
	}

	return &RouteDecision{
		RouteID:       route.ID,
		SelectedModel: selected.ModelID,
		Strategy:      route.Strategy,
		Reason:        fmt.Sprintf("round-robin selected from %d healthy models", len(healthyModels)),
		Alternatives:  alternatives,
	}
}

func (m *Manager) selectLeastLatency(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	var selected *ModelEntry
	var minLatency int64 = math.MaxInt64

	for i := range healthyModels {
		latency := m.getModelLatency(healthyModels[i].ModelID)
		if latency < minLatency {
			minLatency = latency
			selected = &healthyModels[i]
		}
	}

	return &RouteDecision{
		RouteID:       route.ID,
		SelectedModel: selected.ModelID,
		Strategy:      route.Strategy,
		Reason:        fmt.Sprintf("lowest latency: %dms", minLatency),
	}
}

func (m *Manager) selectCostOptimized(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	var selected *ModelEntry
	minCost := math.MaxFloat64

	for i := range healthyModels {
		cost := m.getModelCost(healthyModels[i].ModelID)
		if cost < minCost {
			minCost = cost
			selected = &healthyModels[i]
		}
	}

	return &RouteDecision{
		RouteID:       route.ID,
		SelectedModel: selected.ModelID,
		Strategy:      route.Strategy,
		Reason:        fmt.Sprintf("lowest cost: %.4f", minCost),
	}
}

func (m *Manager) selectQualityFirst(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	var selected *ModelEntry
	maxSuccessRate := 0.0

	for i := range healthyModels {
		rate := m.getModelSuccessRate(healthyModels[i].ModelID)
		if rate > maxSuccessRate {
			maxSuccessRate = rate
			selected = &healthyModels[i]
		}
	}

	return &RouteDecision{
		RouteID:       route.ID,
		SelectedModel: selected.ModelID,
		Strategy:      route.Strategy,
		Reason:        fmt.Sprintf("highest success rate: %.2f%%", maxSuccessRate*100),
	}
}

func (m *Manager) selectFailover(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	// 按优先级排序
	var selected *ModelEntry
	maxPriority := -1

	for i := range healthyModels {
		if healthyModels[i].Priority > maxPriority {
			maxPriority = healthyModels[i].Priority
			selected = &healthyModels[i]
		}
	}

	return &RouteDecision{
		RouteID:       route.ID,
		SelectedModel: selected.ModelID,
		Strategy:      route.Strategy,
		Reason:        fmt.Sprintf("failover: highest priority %d", maxPriority),
	}
}

func (m *Manager) selectWeighted(route *RouteConfig) *RouteDecision {
	healthyModels := m.getHealthyModels(route)
	if len(healthyModels) == 0 {
		return &RouteDecision{
			RouteID:  route.ID,
			Strategy: route.Strategy,
			Reason:   "no healthy models available",
		}
	}

	totalWeight := 0
	for _, model := range healthyModels {
		totalWeight += model.Weight
	}

	if totalWeight == 0 {
		return m.selectRoundRobin(route)
	}

	random := rand.Intn(totalWeight)
	cumulative := 0

	for _, model := range healthyModels {
		cumulative += model.Weight
		if random < cumulative {
			return &RouteDecision{
				RouteID:       route.ID,
				SelectedModel: model.ModelID,
				Strategy:      route.Strategy,
				Reason:        fmt.Sprintf("weighted selection (weight=%d)", model.Weight),
			}
		}
	}

	return m.selectRoundRobin(route)
}

func (m *Manager) getHealthyModels(route *RouteConfig) []ModelEntry {
	var healthy []ModelEntry
	for _, model := range route.Models {
		if model.Health == HealthHealthy || model.Health == HealthDegraded {
			healthy = append(healthy, model)
		}
	}
	return healthy
}

func (m *Manager) getModelLatency(modelID string) int64 {
	if data, ok := m.metrics.Load(modelID); ok {
		md := data.(*modelMetricsData)
		md.mu.Lock()
		defer md.mu.Unlock()
		return md.metrics.AvgLatencyMs
	}
	return 1000 // 默认延迟
}

func (m *Manager) getModelCost(modelID string) float64 {
	if data, ok := m.metrics.Load(modelID); ok {
		md := data.(*modelMetricsData)
		md.mu.Lock()
		defer md.mu.Unlock()
		return md.metrics.TotalCost / float64(md.metrics.TotalTokens+1)
	}
	return 0.001 // 默认成本
}

func (m *Manager) getModelSuccessRate(modelID string) float64 {
	if data, ok := m.metrics.Load(modelID); ok {
		md := data.(*modelMetricsData)
		md.mu.Lock()
		defer md.mu.Unlock()
		return md.metrics.SuccessRate
	}
	return 0.99 // 默认成功率
}

// ReportResult 上报结果
func (m *Manager) ReportResult(ctx context.Context, decision *RouteDecision, success bool, latencyMs int64, tokens int) error {
	// 更新模型指标
	m.updateModelMetrics(decision.SelectedModel, success, latencyMs, int64(tokens))

	// 更新路由统计
	m.updateRouteStats(decision.RouteID, decision.SelectedModel, success, latencyMs, int64(tokens))

	return nil
}

func (m *Manager) updateModelMetrics(modelID string, success bool, latencyMs, tokens int64) {
	data, _ := m.metrics.LoadOrStore(modelID, &modelMetricsData{
		metrics: ModelMetrics{
			ModelID: modelID,
			Health:  HealthHealthy,
		},
		latencies: make([]int64, 0, 1000),
	})
	md := data.(*modelMetricsData)

	md.mu.Lock()
	defer md.mu.Unlock()

	md.metrics.TotalRequests++
	md.metrics.TotalTokens += tokens

	if success {
		md.metrics.SuccessRate = (md.metrics.SuccessRate*float64(md.metrics.TotalRequests-1) + 1) / float64(md.metrics.TotalRequests)
	} else {
		md.metrics.SuccessRate = (md.metrics.SuccessRate * float64(md.metrics.TotalRequests-1)) / float64(md.metrics.TotalRequests)
	}

	md.latencies = append(md.latencies, latencyMs)
	if len(md.latencies) > 1000 {
		md.latencies = md.latencies[len(md.latencies)-1000:]
	}

	// 计算平均延迟
	total := int64(0)
	for _, l := range md.latencies {
		total += l
	}
	md.metrics.AvgLatencyMs = total / int64(len(md.latencies))

	md.metrics.LastCheckTime = time.Now()

	// 更新健康状态
	if md.metrics.SuccessRate < 0.5 {
		md.metrics.Health = HealthUnhealthy
	} else if md.metrics.SuccessRate < 0.9 {
		md.metrics.Health = HealthDegraded
	} else {
		md.metrics.Health = HealthHealthy
	}
}

func (m *Manager) updateRouteStats(routeID, modelID string, success bool, latencyMs, tokens int64) {
	data, ok := m.stats.Load(routeID)
	if !ok {
		return
	}
	rs := data.(*routeStatsData)

	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.stats.TotalRequests++
	rs.stats.TotalTokens += tokens
	rs.stats.LastRequestTime = time.Now()

	if success {
		rs.stats.SuccessRequests++
	} else {
		rs.stats.FailedRequests++
	}

	rs.stats.ModelDistribution[modelID]++
}

// GetStats 获取路由统计
func (m *Manager) GetStats(ctx context.Context, routeID string) (*RouteStats, error) {
	data, ok := m.stats.Load(routeID)
	if !ok {
		return nil, fmt.Errorf("route %s not found", routeID)
	}

	rs := data.(*routeStatsData)
	rs.mu.Lock()
	defer rs.mu.Unlock()

	stats := rs.stats
	return &stats, nil
}

// GetModelMetrics 获取模型指标
func (m *Manager) GetModelMetrics(ctx context.Context, modelID string) (*ModelMetrics, error) {
	data, ok := m.metrics.Load(modelID)
	if !ok {
		return nil, fmt.Errorf("model %s not found", modelID)
	}

	md := data.(*modelMetricsData)
	md.mu.Lock()
	defer md.mu.Unlock()

	metrics := md.metrics
	return &metrics, nil
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context) error {
	return nil
}

// ListRoutes 列出路由配置
func (m *Manager) ListRoutes() []*RouteConfig {
	var routes []*RouteConfig
	m.routes.Range(func(key, value interface{}) bool {
		routes = append(routes, value.(*RouteConfig))
		return true
	})
	return routes
}

// DeleteRoute 删除路由配置
func (m *Manager) DeleteRoute(routeID string) error {
	if _, loaded := m.routes.LoadAndDelete(routeID); !loaded {
		return fmt.Errorf("route %s not found", routeID)
	}
	m.stats.Delete(routeID)
	return nil
}
