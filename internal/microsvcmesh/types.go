// Package microsvcmesh 提供微服务网格功能，支持服务发现与注册、智能路由、
// 熔断降级和分布式追踪。
package microsvcmesh

import "time"

// ServiceStatus 服务状态
type ServiceStatus string

const (
	ServiceStatusHealthy   ServiceStatus = "healthy"
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	ServiceStatusDraining  ServiceStatus = "draining"
	ServiceStatusUnknown   ServiceStatus = "unknown"
)

// CircuitState 熔断器状态
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // 正常（放行）
	CircuitOpen     CircuitState = "open"      // 熔断（阻断）
	CircuitHalfOpen CircuitState = "half_open" // 半开（探测）
)

// RouteStrategy 路由策略
type RouteStrategy string

const (
	RouteRoundRobin  RouteStrategy = "round_robin"  // 轮询
	RouteWeighted    RouteStrategy = "weighted"      // 权重
	RouteRandom      RouteStrategy = "random"        // 随机
	RouteLeastConn   RouteStrategy = "least_conn"    // 最少连接
	RouteSticky      RouteStrategy = "sticky"        // 粘性会话
)

// Protocol 协议类型
type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
	ProtocolgRPC  Protocol = "grpc"
	ProtocolTCP   Protocol = "tcp"
)

// Config 微服务网格配置
type Config struct {
	Enabled            bool    `json:"enabled"`
	ListenAddr         string  `json:"listen_addr"`          // 代理监听地址
	HealthCheckInterval int    `json:"health_check_interval"` // 健康检查间隔（秒）
	RequestTimeout     int     `json:"request_timeout"`       // 请求超时（秒）
	MaxConnections     int     `json:"max_connections"`       // 最大连接数
	RetryAttempts      int     `json:"retry_attempts"`        // 重试次数
	RetryDelay         int     `json:"retry_delay"`           // 重试延迟（ms）
	TracingEnabled     bool    `json:"tracing_enabled"`       // 是否启用分布式追踪
	TracingSampleRate  float64 `json:"tracing_sample_rate"`   // 采样率
	MetricsEnabled     bool    `json:"metrics_enabled"`       // 是否启用指标收集
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		ListenAddr:          ":8080",
		HealthCheckInterval: 30,
		RequestTimeout:      30,
		MaxConnections:      1000,
		RetryAttempts:       3,
		RetryDelay:          100,
		TracingEnabled:      true,
		TracingSampleRate:   0.1,
		MetricsEnabled:      true,
	}
}

// Service 服务定义
type Service struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Namespace string            `json:"namespace"`
	Status    ServiceStatus     `json:"status"`
	Endpoints []*Endpoint       `json:"endpoints"`
	Routes    []*Route          `json:"routes,omitempty"`
	Policies  map[string]string `json:"policies,omitempty"` // 策略键值对
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Endpoint 服务端点
type Endpoint struct {
	ID       string   `json:"id"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Protocol Protocol `json:"protocol"`
	Weight   int      `json:"weight"`   // 权重
	Status   ServiceStatus `json:"status"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// Route 路由规则
type Route struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Prefix   string        `json:"prefix"`           // 路径前缀
	Methods  []string      `json:"methods,omitempty"` // HTTP 方法
	Strategy RouteStrategy `json:"strategy"`
	Target   string        `json:"target"`           // 目标服务名
	Weight   int           `json:"weight"`           // 流量权重（用于金丝雀发布）
	Headers  map[string]string `json:"headers,omitempty"` // 匹配头
	Retry    *RetryPolicy  `json:"retry,omitempty"`
	Timeout  int           `json:"timeout"`          // 超时（秒）
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries int      `json:"max_retries"`
	RetryOn    []string `json:"retry_on"`    // 重试条件：5xx, timeout, connection_error
	Backoff    string   `json:"backoff"`     // 退避策略：linear, exponential
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold    int     `json:"failure_threshold"`    // 失败阈值
	SuccessThreshold    int     `json:"success_threshold"`    // 成功阈值（半开→关闭）
	Timeout             int     `json:"timeout"`              // 熔断超时（秒）
	FailureRate         float64 `json:"failure_rate"`         // 失败率阈值
	MinRequests         int     `json:"min_requests"`         // 最小请求数（触发评估）
	WindowSize          int     `json:"window_size"`          // 统计窗口大小（秒）
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30,
		FailureRate:      0.5,
		MinRequests:      10,
		WindowSize:       60,
	}
}

// TraceSpan 追踪跨度
type TraceSpan struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Service    string            `json:"service"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Duration   time.Duration     `json:"duration"`
	Status     string            `json:"status"`        // ok, error
	Tags       map[string]string `json:"tags,omitempty"`
	Events     []TraceEvent      `json:"events,omitempty"`
}

// TraceEvent 追踪事件
type TraceEvent struct {
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"` // counter, gauge, histogram
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// ProxyRequest 代理请求
type ProxyRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Service string            `json:"service"`
}

// ProxyResponse 代理响应
type ProxyResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
	Duration   time.Duration     `json:"duration"`
	Upstream   string            `json:"upstream"` // 实际处理的端点
}
