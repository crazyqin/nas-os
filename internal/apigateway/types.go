// Package apigateway 提供 API 网关功能，包括路由管理、负载均衡、限流、认证、熔断等。
// 对标企业级 API 网关，为 NAS 系统提供统一的 API 入口。
package apigateway

import (
	"net/http"
	"sync"
	"time"
)

// ==================== 路由相关 ====================

// Route 路由规则
type Route struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Path          string            `json:"path"`
	Method        string            `json:"method"`
	UpstreamID    string            `json:"upstream_id"`
	StripPrefix   string            `json:"strip_prefix,omitempty"`
	AddPrefix     string            `json:"add_prefix,omitempty"`
	Plugins       []string          `json:"plugins,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	RegexPriority int               `json:"regex_priority,omitempty"`
	Protocols     []string          `json:"protocols,omitempty"`
	Hosts         []string          `json:"hosts,omitempty"`
	Paths         []string          `json:"paths,omitempty"`
	Methods       []string          `json:"methods,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ==================== 上游服务相关 ====================

// Upstream 上游服务
type Upstream struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Algorithm         string           `json:"algorithm"` // round-robin, weighted, least-connections, ip-hash
	Targets           []Target         `json:"targets"`
	HealthCheck       *HealthCheck     `json:"health_check,omitempty"`
	ConnectTimeout    time.Duration    `json:"connect_timeout"`
	ReadTimeout       time.Duration    `json:"read_timeout"`
	WriteTimeout      time.Duration    `json:"write_timeout"`
	MaxConnections    int              `json:"max_connections"`
	Retries           int              `json:"retries"`
	Enabled           bool             `json:"enabled"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// Target 目标服务
type Target struct {
	ID       string  `json:"id"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Weight   int     `json:"weight"`
	Health   string  `json:"health"` // healthy, unhealthy, unknown
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Active  *ActiveHealthCheck  `json:"active,omitempty"`
	Passive *PassiveHealthCheck `json:"passive,omitempty"`
}

// ActiveHealthCheck 主动健康检查
type ActiveHealthCheck struct {
	HTTPPath     string        `json:"http_path"`
	HTTPStatus   []int         `json:"http_status"`
	Interval     time.Duration `json:"interval"`
	Timeout      time.Duration `json:"timeout"`
	Concurrency  int           `json:"concurrency"`
}

// PassiveHealthCheck 被动健康检查
type PassiveHealthCheck struct {
	HTTPStatus []int         `json:"http_status"`
	Unhealthy  *UnhealthyConfig `json:"unhealthy,omitempty"`
	Healthy    *HealthyConfig   `json:"healthy,omitempty"`
}

// UnhealthyConfig 不健康配置
type UnhealthyConfig struct {
	HTTPStatuses []int `json:"http_statuses"`
	TCPFailures  int   `json:"tcp_failures"`
	Timeouts     int   `json:"timeouts"`
	Interval     int   `json:"interval"`
}

// HealthyConfig 健康配置
type HealthyConfig struct {
	HTTPStatuses []int `json:"http_statuses"`
	Interval     int   `json:"interval"`
}

// ==================== 限流相关 ====================

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(key string) bool
	Reset(key string)
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool   `json:"enabled"`
	Algorithm         string `json:"algorithm"` // token-bucket, sliding-window, fixed-window
	RequestsPerSecond int    `json:"requests_per_second"`
	Burst             int    `json:"burst"`
	KeyType           string `json:"key_type"` // ip, header, consumer
	KeyName           string `json:"key_name,omitempty"`
	WindowSize        int    `json:"window_size,omitempty"` // seconds
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	mu       sync.Mutex
	rate     float64
	burst    int
	buckets  map[string]*tokenBucket
}

// tokenBucket 令牌桶
type tokenBucket struct {
	tokens    float64
	lastTime  time.Time
	rate      float64
	burst     int
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	counters map[string]*slidingWindow
}

// slidingWindow 滑动窗口
type slidingWindow struct {
	windowStart time.Time
	count       int
	prevCount   int
}

// ==================== 认证相关 ====================

// AuthType 认证类型
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeJWT    AuthType = "jwt"
	AuthTypeOAuth2 AuthType = "oauth2"
)

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled bool     `json:"enabled"`
	Type    AuthType `json:"type"`
	// API Key 配置
	HeaderName string `json:"header_name,omitempty"`
	QueryParam string `json:"query_param,omitempty"`
	// JWT 配置
	JWTSecret     string `json:"jwt_secret,omitempty"`
	JWTAlgorithm  string `json:"jwt_algorithm,omitempty"`
	JWTIssuer     string `json:"jwt_issuer,omitempty"`
	JWTExpiration int    `json:"jwt_expiration,omitempty"` // seconds
	// OAuth2 配置
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	AuthURL      string   `json:"auth_url,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	RedirectURL  string   `json:"redirect_url,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

// APIKeyInfo API Key 信息
type APIKeyInfo struct {
	Key       string    `json:"key"`
	ConsumerID string   `json:"consumer_id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Tags      []string  `json:"tags,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ==================== 熔断器相关 ====================

// CircuitBreakerState 熔断器状态
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	MaxRequests      int           `json:"max_requests"`
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu             sync.Mutex
	config         CircuitBreakerConfig
	state          CircuitBreakerState
	failureCount   int
	successCount   int
	lastFailure    time.Time
	nextRetry      time.Time
}

// ==================== 请求/响应转换相关 ====================

// RequestTransform 请求转换配置
type RequestTransform struct {
	RemoveHeaders []string          `json:"remove_headers,omitempty"`
	AddHeaders    map[string]string `json:"add_headers,omitempty"`
	ReplaceURI    string            `json:"replace_uri,omitempty"`
	AppendURI     string            `json:"append_uri,omitempty"`
	HTTPMethod    string            `json:"http_method,omitempty"`
	Body          *BodyTransform    `json:"body,omitempty"`
}

// ResponseTransform 响应转换配置
type ResponseTransform struct {
	RemoveHeaders []string          `json:"remove_headers,omitempty"`
	AddHeaders    map[string]string `json:"add_headers,omitempty"`
	StatusCode    int               `json:"status_code,omitempty"`
	Body          *BodyTransform    `json:"body,omitempty"`
}

// BodyTransform 请求/响应体转换
type BodyTransform struct {
	Template string `json:"template,omitempty"`
	Replace  map[string]string `json:"replace,omitempty"`
	Append   string `json:"append,omitempty"`
	Prepend  string `json:"prepend,omitempty"`
}

// ==================== 中间件/插件相关 ====================

// Plugin 插件接口
type Plugin interface {
	Name() string
	Execute(next http.Handler) http.Handler
}

// PluginConfig 插件配置
type PluginConfig struct {
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// ==================== CORS 相关 ====================

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
	ExposeHeaders    []string `json:"expose_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"` // seconds
}

// ==================== API 版本管理 ====================

// APIVersion API 版本
type APIVersion struct {
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	Deprecated  bool      `json:"deprecated"`
	SunsetDate  *time.Time `json:"sunset_date,omitempty"`
	Routes      []string  `json:"routes,omitempty"`
}

// ==================== 请求日志相关 ====================

// RequestLog 请求日志
type RequestLog struct {
	ID            string        `json:"id"`
	RequestID     string        `json:"request_id"`
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	StatusCode    int           `json:"status_code"`
	UpstreamHost  string        `json:"upstream_host,omitempty"`
	UpstreamPath  string        `json:"upstream_path,omitempty"`
	ClientIP      string        `json:"client_ip"`
	UserAgent     string        `json:"user_agent,omitempty"`
	RequestBody   string        `json:"request_body,omitempty"`
	ResponseBody  string        `json:"response_body,omitempty"`
	RequestSize   int64         `json:"request_size"`
	ResponseSize  int64         `json:"response_size"`
	Duration      time.Duration `json:"duration"`
	ConsumerID    string        `json:"consumer_id,omitempty"`
	ConsumerName  string        `json:"consumer_name,omitempty"`
	Authenticated bool          `json:"authenticated"`
	Error         string        `json:"error,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// ==================== 消费者相关 ====================

// Consumer 消费者（API 调用者）
type Consumer struct {
	ID        string            `json:"id"`
	Username  string            `json:"username"`
	CustomID  string            `json:"custom_id,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
}

// ==================== 配置相关 ====================

// GatewayConfig 网关配置
type GatewayConfig struct {
	ListenAddr       string           `json:"listen_addr"`
	ListenPort       int              `json:"listen_port"`
	TLSEnabled       bool             `json:"tls_enabled"`
	TLSCertFile      string           `json:"tls_cert_file,omitempty"`
	TLSKeyFile       string           `json:"tls_key_file,omitempty"`
	MaxBodySize      int64            `json:"max_body_size"`      // bytes
	ReadTimeout      time.Duration    `json:"read_timeout"`
	WriteTimeout     time.Duration    `json:"write_timeout"`
	IdleTimeout      time.Duration    `json:"idle_timeout"`
	CORS             CORSConfig       `json:"cors"`
	RateLimit        RateLimitConfig  `json:"rate_limit"`
	Auth             AuthConfig       `json:"auth"`
	LogEnabled       bool             `json:"log_enabled"`
	LogMaxSize       int              `json:"log_max_size"`
	PluginConfigs    []PluginConfig   `json:"plugins,omitempty"`
}

// DefaultGatewayConfig 默认网关配置
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		ListenAddr:   "0.0.0.0",
		ListenPort:   8080,
		MaxBodySize:  10 << 20, // 10MB
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		CORS: CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"},
			MaxAge:       86400,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			Algorithm:         "token-bucket",
			RequestsPerSecond: 100,
			Burst:             200,
			KeyType:           "ip",
		},
		LogEnabled: true,
		LogMaxSize: 10000,
	}
}

// ==================== WebSocket 相关 ====================

// WebSocketConfig WebSocket 代理配置
type WebSocketConfig struct {
	Enabled        bool          `json:"enabled"`
	Path           string        `json:"path"`
	UpstreamURL    string        `json:"upstream_url"`
	ReadBufferSize  int          `json:"read_buffer_size"`
	WriteBufferSize int          `json:"write_buffer_size"`
	HandshakeTimeout time.Duration `json:"handshake_timeout"`
}

// ==================== 统计相关 ====================

// GatewayStats 网关统计
type GatewayStats struct {
	TotalRequests      int64         `json:"total_requests"`
	ActiveConnections  int64         `json:"active_connections"`
	TotalUpstreams     int           `json:"total_upstreams"`
	TotalRoutes        int           `json:"total_routes"`
	HealthyTargets     int           `json:"healthy_targets"`
	UnhealthyTargets   int           `json:"unhealthy_targets"`
	AverageLatency     time.Duration `json:"average_latency"`
	RequestsPerSecond  float64       `json:"requests_per_second"`
	ErrorRate          float64       `json:"error_rate"`
	BytesReceived      int64         `json:"bytes_received"`
	BytesSent          int64         `json:"bytes_sent"`
	Uptime             time.Duration `json:"uptime"`
	LastRequestTime    time.Time     `json:"last_request_time"`
}
