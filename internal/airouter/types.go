// Package airouter 提供 AI 智能路由引擎
// 支持多模型智能选择、负载均衡、故障转移、成本优化
package airouter

import (
	"context"
	"time"
)

// ========== 路由策略 ==========

// RouteStrategy 路由策略类型.
type RouteStrategy string

const (
	// StrategyRoundRobin 轮询.
	StrategyRoundRobin RouteStrategy = "round_robin"
	// StrategyLeastLatency 最低延迟.
	StrategyLeastLatency RouteStrategy = "least_latency"
	// StrategyCostOptimized 成本优化.
	StrategyCostOptimized RouteStrategy = "cost_optimized"
	// StrategyQualityFirst 质量优先.
	StrategyQualityFirst RouteStrategy = "quality_first"
	// StrategyFailover 故障转移.
	StrategyFailover RouteStrategy = "failover"
	// StrategyWeighted 权重分配.
	StrategyWeighted RouteStrategy = "weighted"
)

// ModelHealth 模型健康状态.
type ModelHealth string

const (
	HealthHealthy   ModelHealth = "healthy"
	HealthDegraded  ModelHealth = "degraded"
	HealthUnhealthy ModelHealth = "unhealthy"
	HealthUnknown   ModelHealth = "unknown"
)

// ========== 路由配置 ==========

// RouteConfig 路由配置.
type RouteConfig struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Strategy        RouteStrategy `json:"strategy"`
	Models          []ModelEntry  `json:"models"`
	FallbackEnabled bool          `json:"fallbackEnabled"`
	MaxRetries      int           `json:"maxRetries"`
	TimeoutMs       int           `json:"timeoutMs"`
	Enabled         bool          `json:"enabled"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// ModelEntry 路由模型条目.
type ModelEntry struct {
	ModelID       string      `json:"modelName"`
	Weight        int         `json:"weight"`
	MaxQPS        int         `json:"maxQPS"`
	CurrentQPS    int         `json:"currentQPS"`
	Priority      int         `json:"priority"`
	Health        ModelHealth `json:"health"`
	AvgLatencyMs  int64       `json:"avgLatencyMs"`
	SuccessRate   float64     `json:"successRate"`
	TotalRequests int64       `json:"totalRequests"`
	TotalTokens   int64       `json:"totalTokens"`
	TotalCost     float64     `json:"totalCost"`
}

// CreateRouteRequest 创建路由请求.
type CreateRouteRequest struct {
	Name            string        `json:"name" binding:"required"`
	Strategy        RouteStrategy `json:"strategy" binding:"required"`
	ModelIDs        []string      `json:"modelIds" binding:"required"`
	FallbackEnabled bool          `json:"fallbackEnabled"`
	MaxRetries      int           `json:"maxRetries"`
	TimeoutMs       int           `json:"timeoutMs"`
}

// ========== 路由决策 ==========

// RouteDecision 路由决策结果.
type RouteDecision struct {
	RouteID       string        `json:"routeId"`
	SelectedModel string        `json:"selectedModel"`
	Strategy      RouteStrategy `json:"strategy"`
	Reason        string        `json:"reason"`
	Alternatives  []string      `json:"alternatives,omitempty"`
	DecisionMs    int64         `json:"decisionMs"`
}

// RouteRequest 路由请求.
type RouteRequest struct {
	RouteID   string `json:"routeId"`
	PromptLen int    `json:"promptLen"`
	Priority  int    `json:"priority"`
	UserID    string `json:"userId"`
}

// ========== 路由统计 ==========

// RouteStats 路由统计.
type RouteStats struct {
	RouteID           string           `json:"routeId"`
	TotalRequests     int64            `json:"totalRequests"`
	SuccessRequests   int64            `json:"successRequests"`
	FailedRequests    int64            `json:"failedRequests"`
	AvgLatencyMs      int64            `json:"avgLatencyMs"`
	P99LatencyMs      int64            `json:"p99LatencyMs"`
	TotalTokens       int64            `json:"totalTokens"`
	TotalCost         float64          `json:"totalCost"`
	ModelDistribution map[string]int64 `json:"modelDistribution"`
	LastRequestTime   time.Time        `json:"lastRequestTime"`
}

// ModelMetrics 模型性能指标.
type ModelMetrics struct {
	ModelID       string      `json:"modelName"`
	Health        ModelHealth `json:"health"`
	AvgLatencyMs  int64       `json:"avgLatencyMs"`
	P50LatencyMs  int64       `json:"p50LatencyMs"`
	P95LatencyMs  int64       `json:"p95LatencyMs"`
	P99LatencyMs  int64       `json:"p99LatencyMs"`
	SuccessRate   float64     `json:"successRate"`
	QPS           float64     `json:"qps"`
	TotalRequests int64       `json:"totalRequests"`
	TotalTokens   int64       `json:"totalTokens"`
	TotalCost     float64     `json:"totalCost"`
	Uptime        float64     `json:"uptime"`
	LastCheckTime time.Time   `json:"lastCheckTime"`
	ErrorMessage  string      `json:"errorMessage,omitempty"`
}

// ========== 路由接口 ==========

// Router 路由器接口.
type Router interface {
	// Select 选择模型
	Select(ctx context.Context, req *RouteRequest) (*RouteDecision, error)
	// ReportResult 上报结果
	ReportResult(ctx context.Context, decision *RouteDecision, success bool, latencyMs int64, tokens int) error
	// GetStats 获取统计
	GetStats(ctx context.Context, routeID string) (*RouteStats, error)
	// GetModelMetrics 获取模型指标
	GetModelMetrics(ctx context.Context, modelID string) (*ModelMetrics, error)
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}
