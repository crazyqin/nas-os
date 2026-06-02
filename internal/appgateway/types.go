// Package appgateway 提供统一应用网关功能，为 NAS 系统提供应用注册、域名路由、负载均衡、访问控制等能力。
package appgateway

import (
	"sync"
	"time"
)

// ==================== 应用相关 ====================

// Application 应用信息
type Application struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Domain      string            `json:"domain"`              // 自定义域名，如 app.nas.local
	Path        string            `json:"path"`                // 路径前缀，如 /appname
	Port        int               `json:"port"`                // 应用监听端口
	Protocol    string            `json:"protocol"`            // http, https, ws, wss
	Enabled     bool              `json:"enabled"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	HealthCheck *HealthCheckConfig `json:"health_check,omitempty"`
	Access      *AccessConfig     `json:"access,omitempty"`
	SSL         *SSLConfig        `json:"ssl,omitempty"`
	Instances   []AppInstance     `json:"instances,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AppInstance 应用实例
type AppInstance struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Weight    int       `json:"weight"`   // 负载均衡权重
	Health    string    `json:"health"`   // healthy, unhealthy, unknown
	LastCheck time.Time `json:"last_check,omitempty"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled       bool          `json:"enabled"`
	Path          string        `json:"path"`           // 健康检查路径，如 /health
	Interval      time.Duration `json:"interval"`       // 检查间隔
	Timeout       time.Duration `json:"timeout"`        // 超时时间
	HealthyCodes  []int         `json:"healthy_codes"`  // 期望的状态码
	UnhealthyThreshold int     `json:"unhealthy_threshold"` // 连续失败次数判定不健康
	HealthyThreshold   int     `json:"healthy_threshold"`   // 连续成功次数判定健康
}

// AccessConfig 访问控制配置
type AccessConfig struct {
	RequireAuth    bool     `json:"require_auth"`     // 是否需要认证
	AllowedIPs     []string `json:"allowed_ips"`      // 允许的IP列表
	BlockedIPs     []string `json:"blocked_ips"`      // 拒绝的IP列表
	AllowedDomains []string `json:"allowed_domains"`  // 允许的来源域名
	APIKey         string   `json:"api_key,omitempty"` // API Key 认证
	BasicAuth      *BasicAuth `json:"basic_auth,omitempty"` // Basic认证
}

// BasicAuth Basic认证配置
type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SSLConfig SSL配置
type SSLConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	AutoCert bool   `json:"auto_cert"` // 自动申请证书
}

// ==================== 路由相关 ====================

// RouteRule 路由规则
type RouteRule struct {
	ID            string            `json:"id"`
	AppID         string            `json:"app_id"`
	Domain        string            `json:"domain"`        // 匹配的域名
	Path          string            `json:"path"`          // 匹配的路径前缀
	StripPrefix   bool              `json:"strip_prefix"`  // 是否剥离前缀
	Headers       map[string]string `json:"headers,omitempty"` // 自定义响应头
	Priority      int               `json:"priority"`      // 优先级，越大越优先
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ==================== 代理相关 ====================

// ProxyConfig 代理配置
type ProxyConfig struct {
	ListenAddr       string        `json:"listen_addr"`
	ListenPort       int           `json:"listen_port"`
	TLSPort          int           `json:"tls_port"`
	ReadTimeout      time.Duration `json:"read_timeout"`
	WriteTimeout     time.Duration `json:"write_timeout"`
	IdleTimeout      time.Duration `json:"idle_timeout"`
	MaxHeaderBytes   int           `json:"max_header_bytes"`
	WebSocketEnabled bool          `json:"websocket_enabled"`
	LogEnabled       bool          `json:"log_enabled"`
	LogMaxSize       int           `json:"log_max_size"`
}

// DefaultProxyConfig 默认代理配置
func DefaultProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ListenAddr:       "0.0.0.0",
		ListenPort:       80,
		TLSPort:          443,
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		IdleTimeout:      120 * time.Second,
		MaxHeaderBytes:   1 << 20, // 1MB
		WebSocketEnabled: true,
		LogEnabled:       true,
		LogMaxSize:       10000,
	}
}

// ==================== 负载均衡相关 ====================

// LoadBalancerAlgorithm 负载均衡算法
type LoadBalancerAlgorithm string

const (
	AlgorithmRoundRobin    LoadBalancerAlgorithm = "round-robin"
	AlgorithmWeighted      LoadBalancerAlgorithm = "weighted"
	AlgorithmLeastConn     LoadBalancerAlgorithm = "least-connections"
	AlgorithmIPHash        LoadBalancerAlgorithm = "ip-hash"
	AlgorithmRandom        LoadBalancerAlgorithm = "random"
)

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Algorithm LoadBalancerAlgorithm `json:"algorithm"`
}

// ==================== 请求日志相关 ====================

// AccessLog 访问日志
type AccessLog struct {
	ID            string        `json:"id"`
	RequestID     string        `json:"request_id"`
	AppID         string        `json:"app_id"`
	AppName       string        `json:"app_name"`
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	StatusCode    int           `json:"status_code"`
	ClientIP      string        `json:"client_ip"`
	UserAgent     string        `json:"user_agent"`
	RequestSize   int64         `json:"request_size"`
	ResponseSize  int64         `json:"response_size"`
	Duration      time.Duration `json:"duration"`
	UpstreamHost  string        `json:"upstream_host"`
	IsWebSocket   bool          `json:"is_websocket"`
	Error         string        `json:"error,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// ==================== 统计相关 ====================

// GatewayStats 网关统计
type GatewayStats struct {
	TotalApps         int           `json:"total_apps"`
	ActiveApps        int           `json:"active_apps"`
	TotalRoutes       int           `json:"total_routes"`
	TotalRequests     int64         `json:"total_requests"`
	ActiveConnections int64         `json:"active_connections"`
	AverageLatency    time.Duration `json:"average_latency"`
	ErrorRate         float64       `json:"error_rate"`
	BytesReceived     int64         `json:"bytes_received"`
	BytesSent         int64         `json:"bytes_sent"`
	Uptime            time.Duration `json:"uptime"`
}

// ==================== 管理器 ====================

// Manager 应用网关管理器
type Manager struct {
	mu           sync.RWMutex
	config       *ProxyConfig
	apps         map[string]*Application
	routes       map[string]*RouteRule
	lbConfig     *LoadBalancerConfig
	requestLogs  []*AccessLog
	stats        *GatewayStats
	startTime    time.Time
	stopChan     chan struct{}
	running      bool
	roundRobinIdx map[string]int // 应用ID -> 当前轮询索引
}

// ==================== WebSocket 相关 ====================

// WebSocketSession WebSocket会话
type WebSocketSession struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	ClientIP    string    `json:"client_ip"`
	ConnectedAt time.Time `json:"connected_at"`
	LastActive  time.Time `json:"last_active"`
	MessagesIn  int64     `json:"messages_in"`
	MessagesOut int64     `json:"messages_out"`
}
