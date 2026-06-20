// Package apigateway 提供 API 网关功能
// 统一API路由管理、请求限流、认证、日志审计
package apigateway

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRouteNotFound 路由未找到
	ErrRouteNotFound = errors.New("路由未找到")
	// ErrRateLimited 请求被限流
	ErrRateLimited = errors.New("请求被限流")
	// ErrUnauthorized 未授权
	ErrUnauthorized = errors.New("未授权")
	// ErrForbidden 禁止访问
	ErrForbidden = errors.New("禁止访问")
	// ErrAPIKeyInvalid 无效API Key
	ErrAPIKeyInvalid = errors.New("无效API Key")
	// ErrJWTInvalid 无效JWT
	ErrJWTInvalid = errors.New("无效JWT")
)

// ========== 认证方式 ==========

// AuthType 认证类型
type AuthType string

const (
	AuthNone   AuthType = "none"
	AuthAPIKey AuthType = "api_key"
	AuthJWT    AuthType = "jwt"
	AuthBasic  AuthType = "basic"
	AuthOAuth  AuthType = "oauth"
)

// ========== 路由配置 ==========

// Route 路由配置
type Route struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Method      string    `json:"method"` // GET, POST, PUT, DELETE, *
	Backend     string    `json:"backend"` // 后端服务地址
	StripPrefix bool      `json:"strip_prefix"`
	Auth        AuthType  `json:"auth"`
	RateLimit   *RateLimit `json:"rate_limit,omitempty"`
	Timeout     int       `json:"timeout"` // seconds
	RetryCount  int       `json:"retry_count"`
	Plugins     []string  `json:"plugins,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RouteCreateRequest 创建路由请求
type RouteCreateRequest struct {
	Path        string     `json:"path"`
	Method      string     `json:"method"`
	Backend     string     `json:"backend"`
	StripPrefix bool       `json:"strip_prefix"`
	Auth        AuthType   `json:"auth"`
	RateLimit   *RateLimit `json:"rate_limit,omitempty"`
	Timeout     int        `json:"timeout"`
	RetryCount  int        `json:"retry_count"`
	Plugins     []string   `json:"plugins,omitempty"`
}

// ========== 限流配置 ==========

// RateLimit 限流配置
type RateLimit struct {
	Enabled     bool   `json:"enabled"`
	Requests    int    `json:"requests"`    // 请求数
	Window      int    `json:"window"`      // 时间窗口（秒）
	Burst       int    `json:"burst"`       // 突发请求数
	Strategy    string `json:"strategy"`    // fixed_window, sliding_window, token_bucket
	ByIP        bool   `json:"by_ip"`       // 按IP限流
	ByAPIKey    bool   `json:"by_api_key"`  // 按API Key限流
}

// RateLimitResult 限流结果
type RateLimitResult struct {
	Allowed     bool  `json:"allowed"`
	Remaining   int   `json:"remaining"`   // 剩余请求数
	ResetAt     int64 `json:"reset_at"`    // 重置时间戳
	RetryAfter  int   `json:"retry_after"` // 建议重试等待秒数
}

// ========== 认证凭证 ==========

// APIKey API Key
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	Secret      string    `json:"secret,omitempty"`
	UserID      string    `json:"user_id"`
	Permissions []string  `json:"permissions"`
	RateLimit   *RateLimit `json:"rate_limit,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// APIKeyCreateRequest 创建API Key请求
type APIKeyCreateRequest struct {
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	RateLimit   *RateLimit `json:"rate_limit,omitempty"`
	ExpiresIn   int        `json:"expires_in"` // days, 0=永不过期
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	Expiration  int    `json:"expiration"` // seconds
	RefreshExp  int    `json:"refresh_exp"` // refresh token expiration
	Algorithm   string `json:"algorithm"`   // HS256, RS256
}

// JWTClaims JWT声明
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
	Issuer      string   `json:"iss"`
}

// ========== 请求日志 ==========

// RequestLog 请求日志
type RequestLog struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	Query         string    `json:"query,omitempty"`
	StatusCode    int       `json:"status_code"`
	LatencyMs     int64     `json:"latency_ms"`
	IP            string    `json:"ip"`
	UserAgent     string    `json:"user_agent"`
	UserID        string    `json:"user_id,omitempty"`
	APIKeyID      string    `json:"api_key_id,omitempty"`
	RequestBody   string    `json:"request_body,omitempty"`
	ResponseBody  string    `json:"response_body,omitempty"`
	ContentLength int64     `json:"content_length"`
	Error         string    `json:"error,omitempty"`
}

// RequestLogSearchRequest 日志搜索请求
type RequestLogSearchRequest struct {
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`
	StatusCode *int      `json:"status_code,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	IP         string    `json:"ip,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	MinLatency *int64    `json:"min_latency,omitempty"`
	HasError   *bool     `json:"has_error,omitempty"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

// RequestLogSearchResult 日志搜索结果
type RequestLogSearchResult struct {
	Logs       []*RequestLog `json:"logs"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// ========== 统计 ==========

// GatewayStats 网关统计
type GatewayStats struct {
	TotalRequests   int64            `json:"total_requests"`
	TotalErrors     int64            `json:"total_errors"`
	AvgLatencyMs    float64          `json:"avg_latency_ms"`
	RequestsPerSec  float64          `json:"requests_per_sec"`
	ActiveRoutes    int              `json:"active_routes"`
	TotalAPIKeys    int              `json:"total_api_keys"`
	RateLimitedReqs int64            `json:"rate_limited_requests"`
	StatusCounts    map[int]int64    `json:"status_counts"`
	TopPaths        []PathStats      `json:"top_paths"`
	TopIPs          []IPStats        `json:"top_ips"`
}

// PathStats 路径统计
type PathStats struct {
	Path      string  `json:"path"`
	Count     int64   `json:"count"`
	AvgLatency float64 `json:"avg_latency"`
	ErrorRate float64 `json:"error_rate"`
}

// IPStats IP统计
type IPStats struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// ========== 配置 ==========

// GatewayConfig 网关配置
type GatewayConfig struct {
	ListenAddr     string      `json:"listen_addr"`
	ListenPort     int         `json:"listen_port"`
	ReadTimeout    int         `json:"read_timeout"`    // seconds
	WriteTimeout   int         `json:"write_timeout"`   // seconds
	MaxBodySize    int64       `json:"max_body_size"`   // bytes
	LogEnabled     bool        `json:"log_enabled"`
	LogRequestBody bool        `json:"log_request_body"`
	LogMaxDays     int         `json:"log_max_days"`
	JWTConfig      *JWTConfig  `json:"jwt_config,omitempty"`
}

// DefaultGatewayConfig 默认配置
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		ListenAddr:     "0.0.0.0",
		ListenPort:     8080,
		ReadTimeout:    30,
		WriteTimeout:   30,
		MaxBodySize:    10 * 1024 * 1024, // 10MB
		LogEnabled:     true,
		LogRequestBody: false,
		LogMaxDays:     30,
	}
}
