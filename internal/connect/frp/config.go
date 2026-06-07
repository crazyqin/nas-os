// Package frp provides FRP (Fast Reverse Proxy) client implementation
// 内网穿透核心模块 - 对标飞牛fnOS FN Connect
package frp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TunnelType 隧道类型
type TunnelType string

const (
	TunnelTypeTCP   TunnelType = "tcp"
	TunnelTypeUDP   TunnelType = "udp"
	TunnelTypeHTTP  TunnelType = "http"
	TunnelTypeHTTPS TunnelType = "https"
	TunnelTypeSTCP  TunnelType = "stcp" // Secret TCP, 需要访问密钥
	TunnelTypeXTCP  TunnelType = "xtcp" // P2P TCP
)

// ClientConfig FRP客户端配置
type ClientConfig struct {
	// 基础配置
	Common CommonConfig `json:"common" yaml:"common"`

	// 隧道配置
	Tunnels []TunnelConfig `json:"tunnels" yaml:"tunnels"`

	// 配置文件路径
	ConfigPath string `json:"-" yaml:"-"`
}

// CommonConfig FRP通用配置
type CommonConfig struct {
	// 服务器地址
	ServerAddr string `json:"server_addr" yaml:"server_addr"`

	// 服务器端口
	ServerPort int `json:"server_port" yaml:"server_port"`

	// 认证令牌
	Token string `json:"token" yaml:"token"`

	// 心跳间隔
	HeartbeatInterval int `json:"heartbeat_interval" yaml:"heartbeat_interval"`

	// 心跳超时
	HeartbeatTimeout int `json:"heartbeat_timeout" yaml:"heartbeat_timeout"`

	// TLS配置
	TLSEnable bool `json:"tls_enable" yaml:"tls_enable"`

	// TLS证书文件
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`

	// TLS密钥文件
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file"`

	// 日志级别
	LogLevel string `json:"log_level" yaml:"log_level"`

	// 日志文件
	LogFile string `json:"log_file" yaml:"log_file"`

	// 管理端口
	AdminAddr string `json:"admin_addr" yaml:"admin_addr"`
	AdminPort int    `json:"admin_port" yaml:"admin_port"`
	AdminUser string `json:"admin_user" yaml:"admin_user"`
	AdminPwd  string `json:"admin_pwd" yaml:"admin_pwd"`

	// 连接池大小
	PoolCount int `json:"pool_count" yaml:"pool_count"`

	// TCP多路复用
	TCPMux bool `json:"tcp_mux" yaml:"tcp_mux"`

	// TCP多路复用KeepAlive
	TCPMuxKeepalive int `json:"tcp_mux_keepalive" yaml:"tcp_mux_keepalive"`

	// 协议类型 (tcp, kcp, websocket)
	Protocol string `json:"protocol" yaml:"protocol"`

	// QUIC配置
	QUICKeepalivePeriod int `json:"quic_keepalive_period" yaml:"quic_keepalive_period"`
	QUICMaxIdleTimeout  int `json:"quic_max_idle_timeout" yaml:"quic_max_idle_timeout"`

	// HTTP代理
	HTTPProxy string `json:"http_proxy" yaml:"http_proxy"`

	// 带宽限制 (MB/s)
	BandwidthLimit string `json:"bandwidth_limit" yaml:"bandwidth_limit"`

	// DNS服务器
	DNSServer string `json:"dns_server" yaml:"dns_server"`

	// 登录失败重试
	LoginFailExit bool `json:"login_fail_exit" yaml:"login_fail_exit"`

	// 启动参数
	Start []string `json:"start" yaml:"start"`
}

// TunnelConfig 隧道配置
type TunnelConfig struct {
	// 隧道ID
	ID string `json:"id" yaml:"id"`

	// 隧道名称
	Name string `json:"name" yaml:"name"`

	// 隧道类型
	Type TunnelType `json:"type" yaml:"type"`

	// 本地IP
	LocalIP string `json:"local_ip" yaml:"local_ip"`

	// 本地端口
	LocalPort int `json:"local_port" yaml:"local_port"`

	// 远程端口 (TCP/UDP)
	RemotePort int `json:"remote_port" yaml:"remote_port"`

	// 子域名 (HTTP/HTTPS)
	SubDomain string `json:"sub_domain" yaml:"sub_domain"`

	// 自定义域名
	CustomDomains []string `json:"custom_domains" yaml:"custom_domains"`

	// 路径
	Locations []string `json:"locations" yaml:"locations"`

	// HTTP用户
	HTTPUser string `json:"http_user" yaml:"http_user"`

	// HTTP密码
	HTTPPwd string `json:"http_pwd" yaml:"http_pwd"`

	// Host头重写
	HostHeaderRewrite string `json:"host_header_rewrite" yaml:"host_header_rewrite"`

	// 健康检查类型
	HealthCheckType string `json:"health_check_type" yaml:"health_check_type"`

	// 健康检查超时
	HealthCheckTimeoutS int `json:"health_check_timeout_s" yaml:"health_check_timeout_s"`

	// 健康检查最大失败次数
	HealthCheckMaxFailed int `json:"health_check_max_failed" yaml:"health_check_max_failed"`

	// 健康检查间隔
	HealthCheckIntervalS int `json:"health_check_interval_s" yaml:"health_check_interval_s"`

	// STCP访问密钥
	Sk string `json:"sk" yaml:"sk"`

	// 带宽限制
	BandwidthLimit string `json:"bandwidth_limit" yaml:"bandwidth_limit"`

	// 负载均衡组
	LoadBalancerGroup string `json:"load_balancer_group" yaml:"load_balancer_group"`

	// 负载均衡策略
	LoadBalancerKey string `json:"load_balancer_key" yaml:"load_balancer_key"`

	// 元数据
	Metas map[string]string `json:"metas" yaml:"metas"`

	// 是否启用
	Enabled bool `json:"enabled" yaml:"enabled"`

	// 创建时间
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// 更新时间
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// TunnelStatus 隧道状态
type TunnelStatus struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        TunnelType `json:"type"`
	Status      string     `json:"status"` // running, stopped, error
	LocalAddr   string     `json:"local_addr"`
	RemoteAddr  string     `json:"remote_addr,omitempty"`
	PublicURL   string     `json:"public_url,omitempty"`
	BytesSent   uint64     `json:"bytes_sent"`
	BytesRecv   uint64     `json:"bytes_recv"`
	Connections int        `json:"connections"`
	LastActive  time.Time  `json:"last_active"`
	Error       string     `json:"error,omitempty"`
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Version      string `json:"version"`
	ServerAddr   string `json:"server_addr"`
	ServerPort   int    `json:"server_port"`
	TotalTraffic uint64 `json:"total_traffic"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		Common: CommonConfig{
			ServerAddr:        "connect.nas-os.io",
			ServerPort:        7000,
			HeartbeatInterval: 30,
			HeartbeatTimeout:  90,
			TLSEnable:         true,
			LogLevel:          "info",
			PoolCount:         5,
			TCPMux:            true,
			TCPMuxKeepalive:   60,
			Protocol:          "tcp",
			LoginFailExit:     false,
			AdminAddr:         "127.0.0.1",
			AdminPort:         7500,
		},
		Tunnels: []TunnelConfig{},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClientConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	config.ConfigPath = path
	return &config, nil
}

// SaveConfig 保存配置到文件
func (c *ClientConfig) SaveConfig() error {
	if c.ConfigPath == "" {
		return fmt.Errorf("config path not set")
	}

	if err := os.MkdirAll(filepath.Dir(c.ConfigPath), 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(c.ConfigPath, data, 0600)
}

// AddTunnel 添加隧道
func (c *ClientConfig) AddTunnel(tunnel TunnelConfig) {
	tunnel.CreatedAt = time.Now()
	tunnel.UpdatedAt = time.Now()
	if tunnel.ID == "" {
		tunnel.ID = generateTunnelID()
	}
	c.Tunnels = append(c.Tunnels, tunnel)
}

// RemoveTunnel 移除隧道
func (c *ClientConfig) RemoveTunnel(id string) bool {
	for i, t := range c.Tunnels {
		if t.ID == id {
			c.Tunnels = append(c.Tunnels[:i], c.Tunnels[i+1:]...)
			return true
		}
	}
	return false
}

// GetTunnel 获取隧道
func (c *ClientConfig) GetTunnel(id string) *TunnelConfig {
	for i := range c.Tunnels {
		if c.Tunnels[i].ID == id {
			return &c.Tunnels[i]
		}
	}
	return nil
}

// UpdateTunnel 更新隧道
func (c *ClientConfig) UpdateTunnel(tunnel TunnelConfig) bool {
	for i := range c.Tunnels {
		if c.Tunnels[i].ID == tunnel.ID {
			tunnel.UpdatedAt = time.Now()
			tunnel.CreatedAt = c.Tunnels[i].CreatedAt
			c.Tunnels[i] = tunnel
			return true
		}
	}
	return false
}

// generateTunnelID 生成隧道ID
func generateTunnelID() string {
	return fmt.Sprintf("tunnel_%d", time.Now().UnixNano())
}
