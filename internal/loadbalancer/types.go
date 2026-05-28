// Package loadbalancer - 服务负载均衡器
// 参考群晖 High Availability 和 TrueNAS 集群管理，实现 NAS 服务的负载均衡
package loadbalancer

import (
	"net/http"
	"sync"
	"time"
)

// ============================================================
// 负载均衡算法类型
// ============================================================

// Algorithm 负载均衡算法
type Algorithm string

const (
	// AlgorithmRoundRobin 轮询
	AlgorithmRoundRobin Algorithm = "round_robin"
	// AlgorithmWeightedRoundRobin 加权轮询
	AlgorithmWeightedRoundRobin Algorithm = "weighted_round_robin"
	// AlgorithmLeastConn 最少连接
	AlgorithmLeastConn Algorithm = "least_conn"
	// AlgorithmIPHash IP哈希
	AlgorithmIPHash Algorithm = "ip_hash"
)

// ============================================================
// 后端服务类型
// ============================================================

// Backend 后端服务节点
type Backend struct {
	ID          string            `json:"id"`           // 唯一标识
	Name        string            `json:"name"`         // 服务名称
	URL         string            `json:"url"`          // 后端地址 e.g. "http://192.168.1.10:8080"
	Weight      int               `json:"weight"`       // 权重 (1-100), 默认 1
	MaxConns    int               `json:"max_conns"`    // 最大连接数, 0=无限制
	Tags        map[string]string `json:"tags"`         // 自定义标签
	Metadata    map[string]string `json:"metadata"`     // 扩展元数据

	// 运行时状态 (由负载均衡器维护)
	ActiveConns int64     `json:"active_conns"` // 当前活跃连接数
	TotalConns  int64     `json:"total_conns"`  // 累计连接数
	TotalReqs   int64     `json:"total_reqs"`   // 累计请求数
	TotalErrors int64     `json:"total_errors"` // 累计错误数
	IsHealthy   bool      `json:"is_healthy"`   // 健康状态
	LastCheck   time.Time `json:"last_check"`   // 最后检查时间
	LastActive  time.Time `json:"last_active"`  // 最后活跃时间
	AddedAt     time.Time `json:"added_at"`     // 添加时间

	mu sync.RWMutex
}

// IncrConns 增加活跃连接数
func (b *Backend) IncrConns() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ActiveConns++
	b.TotalConns++
	b.LastActive = time.Now()
}

// DecrConns 减少活跃连接数
func (b *Backend) DecrConns() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ActiveConns > 0 {
		b.ActiveConns--
	}
}

// IncrReqs 增加请求数
func (b *Backend) IncrReqs() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.TotalReqs++
}

// IncrErrors 增加错误数
func (b *Backend) IncrErrors() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.TotalErrors++
}

// SetHealthy 设置健康状态
func (b *Backend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.IsHealthy = healthy
	b.LastCheck = time.Now()
}

// GetStats 获取后端统计信息 (线程安全)
func (b *Backend) GetStats() BackendStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BackendStats{
		ID:          b.ID,
		Name:        b.Name,
		URL:         b.URL,
		Weight:      b.Weight,
		ActiveConns: b.ActiveConns,
		TotalConns:  b.TotalConns,
		TotalReqs:   b.TotalReqs,
		TotalErrors: b.TotalErrors,
		IsHealthy:   b.IsHealthy,
		LastCheck:   b.LastCheck,
		LastActive:  b.LastActive,
	}
}

// BackendStats 后端统计信息快照
type BackendStats struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Weight      int       `json:"weight"`
	ActiveConns int64     `json:"active_conns"`
	TotalConns  int64     `json:"total_conns"`
	TotalReqs   int64     `json:"total_reqs"`
	TotalErrors int64     `json:"total_errors"`
	IsHealthy   bool      `json:"is_healthy"`
	LastCheck   time.Time `json:"last_check"`
	LastActive  time.Time `json:"last_active"`
}

// ============================================================
// 负载均衡配置
// ============================================================

// LBConfig 负载均衡器配置
type LBConfig struct {
	// 基础配置
	ListenAddr     string    `json:"listen_addr"`      // 监听地址, e.g. ":8080"
	Algorithm      Algorithm `json:"algorithm"`         // 负载均衡算法
	Backends       []BackendConfig `json:"backends"`    // 后端列表

	// 健康检查配置
	HealthCheck     HealthCheckConfig `json:"health_check"`

	// 熔断器配置
	CircuitBreaker  CircuitBreakerConfig `json:"circuit_breaker"`

	// 限流配置
	RateLimit       RateLimitConfig `json:"rate_limit"`

	// 代理配置
	Proxy           ProxyConfig `json:"proxy"`

	// 中间件配置
	Middleware      MiddlewareConfig `json:"middleware"`
}

// BackendConfig 后端配置
type BackendConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Weight   int               `json:"weight"`
	MaxConns int               `json:"max_conns"`
	Tags     map[string]string `json:"tags"`
}

// DefaultLBConfig 默认负载均衡配置
func DefaultLBConfig() LBConfig {
	return LBConfig{
		ListenAddr:    ":8080",
		Algorithm:     AlgorithmRoundRobin,
		HealthCheck:   DefaultHealthCheckConfig(),
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		RateLimit:     DefaultRateLimitConfig(),
		Proxy:         DefaultProxyConfig(),
		Middleware:    DefaultMiddlewareConfig(),
	}
}

// ============================================================
// 健康检查配置
// ============================================================

// HealthCheckType 健康检查类型
type HealthCheckType string

const (
	// HealthCheckHTTP HTTP健康检查
	HealthCheckHTTP HealthCheckType = "http"
	// HealthCheckTCP TCP健康检查
	HealthCheckTCP HealthCheckType = "tcp"
	// HealthCheckCustom 自定义探针
	HealthCheckCustom HealthCheckType = "custom"
)

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled         bool              `json:"enabled"`
	Type            HealthCheckType   `json:"type"`              // 检查类型
	Interval        time.Duration     `json:"interval"`          // 检查间隔, 默认 10s
	Timeout         time.Duration     `json:"timeout"`           // 超时时间, 默认 5s
	HealthyThreshold int              `json:"healthy_threshold"` // 健康阈值, 连续成功次数
	UnhealthyThreshold int           `json:"unhealthy_threshold"` // 不健康阈值, 连续失败次数
	Path            string            `json:"path"`              // HTTP检查路径
	ExpectedStatus  int               `json:"expected_status"`   // 期望HTTP状态码
	Headers         map[string]string `json:"headers"`           // 自定义请求头
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Enabled:            true,
		Type:               HealthCheckHTTP,
		Interval:           10 * time.Second,
		Timeout:            5 * time.Second,
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
		Path:               "/health",
		ExpectedStatus:     http.StatusOK,
	}
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	BackendID string        `json:"backend_id"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ============================================================
// 熔断器配置
// ============================================================

// CircuitState 熔断器状态
type CircuitState string

const (
	// CircuitClosed 关闭状态 (正常)
	CircuitClosed CircuitState = "closed"
	// CircuitOpen 打开状态 (熔断)
	CircuitOpen CircuitState = "open"
	// CircuitHalfOpen 半开状态 (探测)
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`   // 失败次数阈值, 默认 5
	FailureRatio     float64       `json:"failure_ratio"`       // 失败率阈值, 默认 0.5
	SuccessThreshold int           `json:"success_threshold"`   // 成功次数阈值 (半开->关闭), 默认 3
	Timeout          time.Duration `json:"timeout"`             // 熔断恢复时间, 默认 30s
	MaxRequests      int           `json:"max_requests"`        // 半开状态最大请求数, 默认 1
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 5,
		FailureRatio:     0.5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		MaxRequests:      1,
	}
}

// CircuitBreakerStats 熔断器统计
type CircuitBreakerStats struct {
	BackendID     string       `json:"backend_id"`
	State         CircuitState `json:"state"`
	Failures      int          `json:"failures"`
	Successes     int          `json:"successes"`
	TotalRequests int64        `json:"total_requests"`
	LastFailure   time.Time    `json:"last_failure"`
	LastSuccess   time.Time    `json:"last_success"`
	OpenedAt      *time.Time   `json:"opened_at,omitempty"`
}

// ============================================================
// 限流配置
// ============================================================

// RateLimitAlgorithm 限流算法
type RateLimitAlgorithm string

const (
	// RateLimitTokenBucket 令牌桶
	RateLimitTokenBucket RateLimitAlgorithm = "token_bucket"
	// RateLimitSlidingWindow 滑动窗口
	RateLimitSlidingWindow RateLimitAlgorithm = "sliding_window"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled   bool                `json:"enabled"`
	Algorithm RateLimitAlgorithm  `json:"algorithm"`    // 限流算法
	Rate      int                 `json:"rate"`         // 每秒请求数
	Burst     int                 `json:"burst"`        // 突发请求量
	ByIP      bool                `json:"by_ip"`        // 按IP限流
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:   true,
		Algorithm: RateLimitTokenBucket,
		Rate:      1000,
		Burst:     2000,
		ByIP:      true,
	}
}

// RateLimitResult 限流结果
type RateLimitResult struct {
	Allowed    bool  `json:"allowed"`
	Remaining  int   `json:"remaining"`
	RetryAfter int64 `json:"retry_after_ms"` // 毫秒
}

// ============================================================
// 代理配置
// ============================================================

// ProxyConfig 反向代理配置
type ProxyConfig struct {
	// 超时配置
	DialTimeout     time.Duration `json:"dial_timeout"`      // 连接超时, 默认 5s
	ResponseTimeout time.Duration `json:"response_timeout"`  // 响应超时, 默认 30s
	IdleTimeout     time.Duration `json:"idle_timeout"`      // 空闲超时, 默认 90s

	// 连接池配置
	MaxIdleConns        int `json:"max_idle_conns"`        // 最大空闲连接, 默认 100
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host"` // 每主机最大空闲连接, 默认 10
	MaxConnsPerHost     int `json:"max_conns_per_host"`    // 每主机最大连接, 默认 100

	// 请求配置
	FlushInterval   time.Duration `json:"flush_interval"`   // 流式响应刷新间隔
	BufferSize      int           `json:"buffer_size"`      // 缓冲区大小, 默认 32KB

	// 头部配置
	PassHostHeader  bool     `json:"pass_host_header"`   // 传递Host头
	TrustedProxies  []string `json:"trusted_proxies"`    // 可信代理列表
	XForwardedFor   bool     `json:"x_forwarded_for"`    // 添加X-Forwarded-For
	XRealIP         bool     `json:"x_real_ip"`          // 添加X-Real-IP
}

// DefaultProxyConfig 默认代理配置
func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		DialTimeout:         5 * time.Second,
		ResponseTimeout:     30 * time.Second,
		IdleTimeout:         90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		FlushInterval:       100 * time.Millisecond,
		BufferSize:          32 * 1024,
		PassHostHeader:      true,
		XForwardedFor:       true,
		XRealIP:             true,
	}
}

// ============================================================
// 中间件配置
// ============================================================

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// 日志配置
	Logging     LoggingConfig     `json:"logging"`

	// CORS配置
	CORS        CORSConfig        `json:"cors"`

	// 压缩配置
	Compression CompressionConfig `json:"compression"`

	// 缓存配置
	Cache       CacheConfig       `json:"cache"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Enabled    bool   `json:"enabled"`
	Format     string `json:"format"`      // "json", "text"
	Level      string `json:"level"`       // "debug", "info", "warn", "error"
	AccessLog  bool   `json:"access_log"`  // 访问日志
	ErrorLog   bool   `json:"error_log"`   // 错误日志
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled        bool     `json:"enabled"`
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
	ExposedHeaders []string `json:"exposed_headers"`
	MaxAge         int      `json:"max_age"` // 秒
	AllowCredentials bool   `json:"allow_credentials"`
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled    bool     `json:"enabled"`
	MinSize    int      `json:"min_size"`    // 最小压缩大小 (bytes), 默认 1024
	Types      []string `json:"types"`       // 压缩的MIME类型
	Level      int      `json:"level"`       // 压缩级别 1-9, 默认 6
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled     bool          `json:"enabled"`
	TTL         time.Duration `json:"ttl"`          // 缓存过期时间, 默认 60s
	MaxSize     int           `json:"max_size"`     // 最大缓存条目数, 默认 1000
	Methods     []string      `json:"methods"`      // 缓存的HTTP方法
	StatusCodes []int         `json:"status_codes"` // 缓存的状态码
}

// DefaultMiddlewareConfig 默认中间件配置
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Logging: LoggingConfig{
			Enabled:   true,
			Format:    "json",
			Level:     "info",
			AccessLog: true,
			ErrorLog:  true,
		},
		CORS: CORSConfig{
			Enabled:        false,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         3600,
		},
		Compression: CompressionConfig{
			Enabled: true,
			MinSize: 1024,
			Types:   []string{"text/plain", "text/html", "application/json", "application/xml"},
			Level:   6,
		},
		Cache: CacheConfig{
			Enabled:     false,
			TTL:         60 * time.Second,
			MaxSize:     1000,
			Methods:     []string{"GET"},
			StatusCodes: []int{200, 203, 204, 206, 300, 301, 308},
		},
	}
}

// ============================================================
// 负载均衡器统计
// ============================================================

// LBStats 负载均衡器统计信息
type LBStats struct {
	// 全局统计
	TotalRequests   int64         `json:"total_requests"`
	TotalErrors     int64         `json:"total_errors"`
	ActiveConns     int64         `json:"active_conns"`
	TotalBytesSent  int64         `json:"total_bytes_sent"`
	TotalBytesRecv  int64         `json:"total_bytes_recv"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	Uptime          time.Duration `json:"uptime"`

	// 后端统计
	BackendStats    []BackendStats      `json:"backend_stats"`
	CircuitStats    []CircuitBreakerStats `json:"circuit_stats"`

	// 时间信息
	StartedAt       time.Time `json:"started_at"`
	LastRequestAt   time.Time `json:"last_request_at"`

	mu sync.RWMutex
}

// IncrRequests 增加请求计数
func (s *LBStats) IncrRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	s.LastRequestAt = time.Now()
}

// IncrErrors 增加错误计数
func (s *LBStats) IncrErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalErrors++
}

// AddBytes 增加字节计数
func (s *LBStats) AddBytes(sent, recv int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalBytesSent += sent
	s.TotalBytesRecv += recv
}

// SetActiveConns 设置活跃连接数
func (s *LBStats) SetActiveConns(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveConns = n
}

// GetSnapshot 获取统计快照
func (s *LBStats) GetSnapshot() LBStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return LBStatsSnapshot{
		TotalRequests:   s.TotalRequests,
		TotalErrors:     s.TotalErrors,
		ActiveConns:     s.ActiveConns,
		TotalBytesSent:  s.TotalBytesSent,
		TotalBytesRecv:  s.TotalBytesRecv,
		AvgResponseTime: s.AvgResponseTime,
		Uptime:          time.Since(s.StartedAt),
		StartedAt:       s.StartedAt,
		LastRequestAt:   s.LastRequestAt,
	}
}

// LBStatsSnapshot 统计信息快照
type LBStatsSnapshot struct {
	TotalRequests   int64         `json:"total_requests"`
	TotalErrors     int64         `json:"total_errors"`
	ActiveConns     int64         `json:"active_conns"`
	TotalBytesSent  int64         `json:"total_bytes_sent"`
	TotalBytesRecv  int64         `json:"total_bytes_recv"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	Uptime          time.Duration `json:"uptime"`
	StartedAt       time.Time     `json:"started_at"`
	LastRequestAt   time.Time     `json:"last_request_at"`
}

// ============================================================
// 流量监控
// ============================================================

// TrafficMetrics 流量指标
type TrafficMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	BytesPerSec    float64 `json:"bytes_per_sec"`
	ActiveConns    int64   `json:"active_conns"`
	ErrorRate      float64 `json:"error_rate"`
	P50LatencyMs   float64 `json:"p50_latency_ms"`
	P95LatencyMs   float64 `json:"p95_latency_ms"`
	P99LatencyMs   float64 `json:"p99_latency_ms"`
}

// TrafficWindow 流量监控窗口
type TrafficWindow struct {
	Duration   time.Duration     `json:"duration"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Requests   int64             `json:"requests"`
	Errors     int64             `json:"errors"`
	BytesSent  int64             `json:"bytes_sent"`
	BytesRecv  int64             `json:"bytes_recv"`
	Latencies  []time.Duration   `json:"-"`
}
