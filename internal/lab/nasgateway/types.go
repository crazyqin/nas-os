// Package nasgateway 提供 API 网关增强功能，包括动态路由、限流熔断、WAF防护、OAuth2.0服务端等。
// 对标企业级 API 网关，为 NAS 系统提供统一的 API 入口和安全防护。
package nasgateway

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// ========== 错误定义 ==========

// 网关相关错误.
var (
	// ErrRouteNotFound 路由不存在.
	ErrRouteNotFound = errors.New("路由不存在")
	// ErrRouteExists 路由已存在.
	ErrRouteExists = errors.New("路由已存在")
	// ErrPolicyNotFound 策略不存在.
	ErrPolicyNotFound = errors.New("策略不存在")
	// ErrRateLimitExceeded 超出限流.
	ErrRateLimitExceeded = errors.New("超出限流")
	// ErrCircuitOpen 熔断器打开.
	ErrCircuitOpen = errors.New("熔断器打开")
	// ErrUnauthorized 未授权.
	ErrUnauthorized = errors.New("未授权")
	// ErrForbidden 禁止访问.
	ErrForbidden = errors.New("禁止访问")
	// ErrWAFBlocked WAF拦截.
	ErrWAFBlocked = errors.New("WAF拦截")
	// ErrPluginNotFound 插件不存在.
	ErrPluginNotFound = errors.New("插件不存在")
	// ErrOAuthInvalidClient 无效的客户端.
	ErrOAuthInvalidClient = errors.New("无效的客户端")
	// ErrOAuthInvalidGrant 无效的授权.
	ErrOAuthInvalidGrant = errors.New("无效的授权")
	// ErrOAuthInvalidToken 无效的令牌.
	ErrOAuthInvalidToken = errors.New("无效的令牌")
)

// ========== 路由相关 ==========

// Route 路由规则.
type Route struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Path        string            `json:"path"`
	Methods     []string          `json:"methods"`
	UpstreamID  string            `json:"upstream_id"`
	StripPrefix string            `json:"strip_prefix,omitempty"`
	AddPrefix   string            `json:"add_prefix,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Plugins     []string          `json:"plugins,omitempty"`
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
	Version     string            `json:"version,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Timeout     time.Duration     `json:"timeout"`
	RetryCount  int               `json:"retry_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Upstream 上游服务.
type Upstream struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Targets        []*Target     `json:"targets"`
	Algorithm      string        `json:"algorithm"` // round-robin, weighted, least-connections, ip-hash
	HealthCheck    *HealthCheck  `json:"health_check,omitempty"`
	ConnectTimeout time.Duration `json:"connect_timeout"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	MaxConnections int           `json:"max_connections"`
	Retries        int           `json:"retries"`
	Enabled        bool          `json:"enabled"`
}

// Target 目标服务.
type Target struct {
	ID       string            `json:"id"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Weight   int               `json:"weight"`
	Health   string            `json:"health"` // healthy, unhealthy, unknown
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HealthCheck 健康检查配置.
type HealthCheck struct {
	Active  *ActiveHealthCheck  `json:"active,omitempty"`
	Passive *PassiveHealthCheck `json:"passive,omitempty"`
}

// ActiveHealthCheck 主动健康检查.
type ActiveHealthCheck struct {
	HTTPPath   string        `json:"http_path"`
	HTTPStatus []int         `json:"http_status"`
	Interval   time.Duration `json:"interval"`
	Timeout    time.Duration `json:"timeout"`
}

// PassiveHealthCheck 被动健康检查.
type PassiveHealthCheck struct {
	HTTPStatus []int `json:"http_status"`
	Unhealthy  int   `json:"unhealthy"` // 连续失败次数
	Healthy    int   `json:"healthy"`   // 连续成功次数
}

// ========== 策略相关 ==========

// PolicyType 策略类型.
type PolicyType string

const (
	// PolicyTypeRateLimit 限流策略.
	PolicyTypeRateLimit PolicyType = "rate_limit"
	// PolicyTypeAuth 认证策略.
	PolicyTypeAuth PolicyType = "auth"
	// PolicyTypeCircuitBreaker 熔断策略.
	PolicyTypeCircuitBreaker PolicyType = "circuit_breaker"
	// PolicyTypeWAF WAF策略.
	PolicyTypeWAF PolicyType = "waf"
	// PolicyTypeTransform 转换策略.
	PolicyTypeTransform PolicyType = "transform"
	// PolicyTypeCache 缓存策略.
	PolicyTypeCache PolicyType = "cache"
)

// Policy 策略.
type Policy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        PolicyType             `json:"type"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
	Routes      []string               `json:"routes,omitempty"` // 绑定的路由ID
	Enabled     bool                   `json:"enabled"`
	Priority    int                    `json:"priority"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// ========== 限流相关 ==========

// RateLimitAlgorithm 限流算法.
type RateLimitAlgorithm string

const (
	// AlgorithmTokenBucket 令牌桶.
	AlgorithmTokenBucket RateLimitAlgorithm = "token_bucket"
	// AlgorithmSlidingWindow 滑动窗口.
	AlgorithmSlidingWindow RateLimitAlgorithm = "sliding_window"
	// AlgorithmFixedWindow 固定窗口.
	AlgorithmFixedWindow RateLimitAlgorithm = "fixed_window"
	// AlgorithmLeakyBucket 漏桶.
	AlgorithmLeakyBucket RateLimitAlgorithm = "leaky_bucket"
)

// RateLimitKey 限流键类型.
type RateLimitKey string

const (
	// KeyIP 按IP限流.
	KeyIP RateLimitKey = "ip"
	// KeyUser 按用户限流.
	KeyUser RateLimitKey = "user"
	// KeyAPI 按API路径限流.
	KeyAPI RateLimitKey = "api"
	// KeyHeader 按请求头限流.
	KeyHeader RateLimitKey = "header"
	// KeyGlobal 全局限流.
	KeyGlobal RateLimitKey = "global"
)

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	Enabled           bool               `json:"enabled"`
	Algorithm         RateLimitAlgorithm `json:"algorithm"`
	RequestsPerSecond int                `json:"requests_per_second"`
	Burst             int                `json:"burst"`
	KeyType           RateLimitKey       `json:"key_type"`
	KeyName           string             `json:"key_name,omitempty"`
	WindowSize        int                `json:"window_size"` // 秒
	MaxRequests       int                `json:"max_requests"`
	ResponseCode      int                `json:"response_code"`
	ResponseMessage   string             `json:"response_message"`
}

// RateLimitResult 限流结果.
type RateLimitResult struct {
	Allowed    bool  `json:"allowed"`
	Limit      int   `json:"limit"`
	Remaining  int   `json:"remaining"`
	ResetAt    int64 `json:"reset_at"`              // Unix timestamp
	RetryAfter int   `json:"retry_after,omitempty"` // 秒
}

// ========== 熔断器相关 ==========

// CircuitState 熔断器状态.
type CircuitState string

const (
	// CircuitClosed 正常状态.
	CircuitClosed CircuitState = "closed"
	// CircuitOpen 熔断打开.
	CircuitOpen CircuitState = "open"
	// CircuitHalfOpen 半开状态.
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitBreakerConfig 熔断器配置.
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	MaxRequests      int           `json:"max_requests"`
}

// CircuitBreaker 熔断器.
type CircuitBreaker struct {
	mu             sync.RWMutex
	config         CircuitBreakerConfig
	state          CircuitState
	failureCount   int
	successCount   int
	lastFailure    time.Time
	nextRetry      time.Time
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64
}

// ========== WAF 相关 ==========

// WAFRuleType WAF规则类型.
type WAFRuleType string

const (
	// WAFRuleSQLInjection SQL注入防护.
	WAFRuleSQLInjection WAFRuleType = "sql_injection"
	// WAFRuleXSS XSS防护.
	WAFRuleXSS WAFRuleType = "xss"
	// WAFRuleCSRF CSRF防护.
	WAFRuleCSRF WAFRuleType = "csrf"
	// WAFRulePathTraversal 路径遍历防护.
	WAFRulePathTraversal WAFRuleType = "path_traversal"
	// WAFRuleCommandInjection 命令注入防护.
	WAFRuleCommandInjection WAFRuleType = "command_injection"
	// WAFRuleFileUpload 文件上传防护.
	WAFRuleFileUpload WAFRuleType = "file_upload"
	// WAFRuleRateLimit 速率限制.
	WAFRuleRateLimit WAFRuleType = "rate_limit"
	// WAFRuleIPBlacklist IP黑名单.
	WAFRuleIPBlacklist WAFRuleType = "ip_blacklist"
	// WAFRuleIPWhitelist IP白名单.
	WAFRuleIPWhitelist WAFRuleType = "ip_whitelist"
	// WAFRuleGeoBlock 地理位置封锁.
	WAFRuleGeoBlock WAFRuleType = "geo_block"
)

// WAFAction WAF动作.
type WAFAction string

const (
	// WAFActionBlock 阻止请求.
	WAFActionBlock WAFAction = "block"
	// WAFActionAllow 允许请求.
	WAFActionAllow WAFAction = "allow"
	// WAFActionLog 仅记录日志.
	WAFActionLog WAFAction = "log"
	// WAFActionRedirect 重定向.
	WAFActionRedirect WAFAction = "redirect"
)

// WAFRule WAF规则.
type WAFRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        WAFRuleType `json:"type"`
	Action      WAFAction   `json:"action"`
	Pattern     string      `json:"pattern"`           // 正则表达式或规则模式
	Paths       []string    `json:"paths,omitempty"`   // 适用路径
	Methods     []string    `json:"methods,omitempty"` // 适用方法
	Enabled     bool        `json:"enabled"`
	Priority    int         `json:"priority"`
	Description string      `json:"description"`
	Threshold   int         `json:"threshold,omitempty"`  // 触发阈值
	WindowSec   int         `json:"window_sec,omitempty"` // 时间窗口（秒）
}

// WAFConfig WAF配置.
type WAFConfig struct {
	Enabled          bool       `json:"enabled"`
	Mode             string     `json:"mode"` // block, detect
	DefaultAction    WAFAction  `json:"default_action"`
	Rules            []*WAFRule `json:"rules"`
	IPBlacklist      []string   `json:"ip_blacklist,omitempty"`
	IPWhitelist      []string   `json:"ip_whitelist,omitempty"`
	BlockedCountries []string   `json:"blocked_countries,omitempty"`
	MaxBodySize      int64      `json:"max_body_size"`
	AllowedOrigins   []string   `json:"allowed_origins,omitempty"`
	EnableLogging    bool       `json:"enable_logging"`
}

// WAFResult WAF检查结果.
type WAFResult struct {
	Blocked  bool     `json:"blocked"`
	Rule     *WAFRule `json:"rule,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	ClientIP string   `json:"client_ip"`
	Path     string   `json:"path"`
	Severity string   `json:"severity"` // low, medium, high, critical
}

// ========== OAuth2.0 相关 ==========

// GrantType 授权类型.
type GrantType string

const (
	// GrantTypeAuthorizationCode 授权码模式.
	GrantTypeAuthorizationCode GrantType = "authorization_code"
	// GrantTypeClientCredentials 客户端凭证模式.
	GrantTypeClientCredentials GrantType = "client_credentials"
	// GrantTypePassword 密码模式.
	GrantTypePassword GrantType = "password"
	// GrantTypeRefreshToken 刷新令牌.
	GrantTypeRefreshToken GrantType = "refresh_token"
)

// OAuthClient OAuth2.0客户端.
type OAuthClient struct {
	ID              string        `json:"id"`
	Secret          string        `json:"secret"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	RedirectURIs    []string      `json:"redirect_uris"`
	GrantTypes      []GrantType   `json:"grant_types"`
	Scopes          []string      `json:"scopes"`
	Enabled         bool          `json:"enabled"`
	AccessTokenTTL  time.Duration `json:"access_token_ttl"`
	RefreshTokenTTL time.Duration `json:"refresh_token_ttl"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// OAuthUser OAuth2.0用户.
type OAuthUser struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Password string   `json:"password"` // hashed
	Email    string   `json:"email"`
	Scopes   []string `json:"scopes"`
	Enabled  bool     `json:"enabled"`
}

// AuthorizationCode 授权码.
type AuthorizationCode struct {
	Code        string    `json:"code"`
	ClientID    string    `json:"client_id"`
	UserID      string    `json:"user_id"`
	RedirectURI string    `json:"redirect_uri"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   time.Time `json:"expires_at"`
	Used        bool      `json:"used"`
}

// AccessToken 访问令牌.
type AccessToken struct {
	Token     string    `json:"token"`
	Type      string    `json:"type"` // Bearer
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id,omitempty"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

// RefreshToken 刷新令牌.
type RefreshToken struct {
	Token     string    `json:"token"`
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id,omitempty"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

// TokenResponse 令牌响应.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// AuthorizationRequest 授权请求.
type AuthorizationRequest struct {
	ResponseType string `json:"response_type"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	Scope        string `json:"scope"`
	State        string `json:"state"`
}

// TokenRequest 令牌请求.
type TokenRequest struct {
	GrantType    GrantType `json:"grant_type"`
	Code         string    `json:"code,omitempty"`
	RedirectURI  string    `json:"redirect_uri,omitempty"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	Username     string    `json:"username,omitempty"`
	Password     string    `json:"password,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// ========== 插件相关 ==========

// Plugin 插件接口.
type Plugin interface {
	Name() string
	Description() string
	Execute(ctx *PluginContext) error
	OnRequest(req *http.Request) error
	OnResponse(resp *http.Response) error
}

// PluginContext 插件上下文.
type PluginContext struct {
	RequestID string
	Route     *Route
	Request   *http.Request
	Response  http.ResponseWriter
	Metadata  map[string]interface{}
}

// PluginConfig 插件配置.
type PluginConfig struct {
	Name     string                 `json:"name"`
	Enabled  bool                   `json:"enabled"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Routes   []string               `json:"routes,omitempty"`
	Priority int                    `json:"priority"`
}

// PluginInfo 插件信息.
type PluginInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
	Routes      int    `json:"routes"` // 绑定的路由数
}

// ========== API版本管理 ==========

// APIVersion API版本.
type APIVersion struct {
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Deprecated  bool       `json:"deprecated"`
	SunsetDate  *time.Time `json:"sunset_date,omitempty"`
	Routes      []string   `json:"routes"`
	BasePath    string     `json:"base_path"`
}

// ========== 请求日志 ==========

// RequestLog 请求日志.
type RequestLog struct {
	ID              string           `json:"id"`
	RequestID       string           `json:"request_id"`
	Method          string           `json:"method"`
	Path            string           `json:"path"`
	Host            string           `json:"host"`
	ClientIP        string           `json:"client_ip"`
	UserAgent       string           `json:"user_agent"`
	StatusCode      int              `json:"status_code"`
	Latency         time.Duration    `json:"latency"`
	RequestSize     int64            `json:"request_size"`
	ResponseSize    int64            `json:"response_size"`
	UpstreamHost    string           `json:"upstream_host"`
	RouteID         string           `json:"route_id"`
	WAFResult       *WAFResult       `json:"waf_result,omitempty"`
	RateLimitResult *RateLimitResult `json:"rate_limit_result,omitempty"`
	Timestamp       time.Time        `json:"timestamp"`
}

// ========== 网关统计 ==========

// GatewayStats 网关统计.
type GatewayStats struct {
	TotalRequests  int64         `json:"total_requests"`
	TotalErrors    int64         `json:"total_errors"`
	AvgLatency     time.Duration `json:"avg_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	ActiveSessions int           `json:"active_sessions"`
	TotalRoutes    int           `json:"total_routes"`
	TotalUpstreams int           `json:"total_upstreams"`
	WAFBlocked     int64         `json:"waf_blocked"`
	RateLimited    int64         `json:"rate_limited"`
	CircuitBroken  int64         `json:"circuit_broken"`
	Uptime         time.Duration `json:"uptime"`
	BytesIn        int64         `json:"bytes_in"`
	BytesOut       int64         `json:"bytes_out"`
}

// ========== 网关配置 ==========

// GatewayConfig 网关配置.
type GatewayConfig struct {
	ListenAddr     string        `json:"listen_addr"`
	ListenPort     int           `json:"listen_port"`
	TLSEnabled     bool          `json:"tls_enabled"`
	CertFile       string        `json:"cert_file"`
	KeyFile        string        `json:"key_file"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	MaxHeaderBytes int           `json:"max_header_bytes"`
	TrustedProxies []string      `json:"trusted_proxies"`
	EnableCORS     bool          `json:"enable_cors"`
	EnableMetrics  bool          `json:"enable_metrics"`
	EnableLogging  bool          `json:"enable_logging"`
	LogLevel       string        `json:"log_level"`
	WAFConfig      *WAFConfig    `json:"waf_config,omitempty"`
}
