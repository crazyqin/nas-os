// Package wgvdeploy 提供 WireGuard 一键部署引擎
package wgvdeploy

import (
	"time"
)

// ============================================================
// 密钥相关类型
// ============================================================

// KeyPair 密钥对.
type KeyPair struct {
	PrivateKey   string `json:"private_key"`   // WireGuard Base64 私钥
	PublicKey    string `json:"public_key"`    // WireGuard Base64 公钥
	PresharedKey string `json:"preshared_key"` // 预共享密钥（可选）
}

// ============================================================
// 服务端配置相关类型
// ============================================================

// ServerConfig 服务端 WireGuard 配置.
type ServerConfig struct {
	InterfaceName string `json:"interface_name"` // 接口名称，如 wg0
	ListenPort    int    `json:"listen_port"`    // 监听端口
	Address       string `json:"address"`        // 服务端地址（CIDR）
	PrivateKey    string `json:"private_key"`    // 服务端私钥
	PublicKey     string `json:"public_key"`     // 服务端公钥
	DNS           string `json:"dns"`            // DNS 服务器
	MTU           int    `json:"mtu"`            // MTU 大小
	PostUp        string `json:"post_up"`        // 启动后命令
	PostDown      string `json:"post_down"`      // 停止后命令
	Peers         []Peer `json:"peers"`          // 已配置的对端
}

// ServerConfigTemplate 服务端配置模板（用于生成 wg0.conf 文件）.
type ServerConfigTemplate struct {
	Interface string // [Interface] 部分
	Peers     string // [Peer] 部分
}

// ============================================================
// 对端（Peer）相关类型
// ============================================================

// Peer 对端配置.
type Peer struct {
	ID                  string    `json:"id"`                    // 唯一标识
	Name                string    `json:"name"`                  // 对端名称（如 phone、laptop）
	PublicKey           string    `json:"public_key"`            // 对端公钥
	PresharedKey        string    `json:"preshared_key"`         // 预共享密钥
	PrivateKey          string    `json:"private_key,omitempty"` // 对端私钥（仅在创建时返回）
	AllowedIPs          string    `json:"allowed_ips"`           // 允许的 IP 地址
	Endpoint            string    `json:"endpoint,omitempty"`    // 端点地址
	PersistentKeepalive int       `json:"persistent_keepalive"`  // 持久保持连接间隔（秒）
	Enabled             bool      `json:"enabled"`               // 是否启用
	AssignedIPv4        string    `json:"assigned_ipv4"`         // 分配的 IPv4 地址
	AssignedIPv6        string    `json:"assigned_ipv6"`         // 分配的 IPv6 地址
	DNS                 string    `json:"dns"`                   // 客户端 DNS
	BytesRx             int64     `json:"bytes_rx"`              // 接收字节数
	BytesTx             int64     `json:"bytes_tx"`              // 发送字节数
	LastHandshake       time.Time `json:"last_handshake"`        // 最后握手时间
	CreatedAt           time.Time `json:"created_at"`            // 创建时间
	UpdatedAt           time.Time `json:"updated_at"`            // 更新时间
}

// CreatePeerRequest 添加对端请求.
type CreatePeerRequest struct {
	Name       string `json:"name" binding:"required"` // 对端名称
	AllowedIPs string `json:"allowed_ips"`             // 允许的 IP（可选，自动生成）
	IPv4       string `json:"ipv4,omitempty"`          // 指定 IPv4 地址
	IPv6       string `json:"ipv6,omitempty"`          // 指定 IPv6 地址
	Template   string `json:"template,omitempty"`      // 使用的模板
}

// UpdatePeerRequest 更新对端请求.
type UpdatePeerRequest struct {
	Name                *string `json:"name,omitempty"`                 // 对端名称
	AllowedIPs          *string `json:"allowed_ips,omitempty"`          // 允许的 IP
	PersistentKeepalive *int    `json:"persistent_keepalive,omitempty"` // 持久保持连接间隔
	Enabled             *bool   `json:"enabled,omitempty"`              // 是否启用
}

// PeerConfig 客户端配置.
type PeerConfig struct {
	Config string `json:"config"` // .conf 配置文件内容
}

// PeerQRCode 对端 QR 码.
type PeerQRCode struct {
	PeerID string `json:"peer_id"` // 对端 ID
	Format string `json:"format"`  // 格式：png 或 svg
	Base64 string `json:"base64"`  // Base64 编码的图片
}

// ============================================================
// 流量监控相关类型
// ============================================================

// TrafficStats 流量统计.
type TrafficStats struct {
	TotalBytesRx int64         `json:"total_bytes_rx"` // 总接收字节数
	TotalBytesTx int64         `json:"total_bytes_tx"` // 总发送字节数
	ActivePeers  int           `json:"active_peers"`   // 活跃对端数
	TotalPeers   int           `json:"total_peers"`    // 总对端数
	PeerStats    []PeerTraffic `json:"peer_stats"`     // 每个对端的流量
	Timestamp    time.Time     `json:"timestamp"`      // 统计时间
}

// PeerTraffic 单个对端流量.
type PeerTraffic struct {
	PeerID        string    `json:"peer_id"`        // 对端 ID
	Name          string    `json:"name"`           // 对端名称
	BytesRx       int64     `json:"bytes_rx"`       // 接收字节数
	BytesTx       int64     `json:"bytes_tx"`       // 发送字节数
	LastHandshake time.Time `json:"last_handshake"` // 最后握手时间
	Connected     bool      `json:"connected"`      // 是否在线
}

// TrafficHistoryRequest 历史流量查询请求.
type TrafficHistoryRequest struct {
	Interval string `form:"interval" binding:"required,oneof=hour day week"` // 聚合间隔：hour/day/week
	PeerID   string `form:"peer_id,omitempty"`                               // 对端 ID（可选）
	Start    string `form:"start,omitempty"`                                 // 开始时间
	End      string `form:"end,omitempty"`                                   // 结束时间
}

// TrafficHistory 历史流量数据.
type TrafficHistory struct {
	Interval   string             `json:"interval"`    // 聚合间隔
	DataPoints []TrafficDataPoint `json:"data_points"` // 数据点
}

// TrafficDataPoint 流量数据点.
type TrafficDataPoint struct {
	Timestamp time.Time `json:"timestamp"` // 时间戳
	BytesRx   int64     `json:"bytes_rx"`  // 接收字节数
	BytesTx   int64     `json:"bytes_tx"`  // 发送字节数
	PeerID    string    `json:"peer_id"`   // 对端 ID（可选）
}

// TrafficAlert 流量异常告警.
type TrafficAlert struct {
	ID        string    `json:"id"`         // 告警 ID
	PeerID    string    `json:"peer_id"`    // 对端 ID
	PeerName  string    `json:"peer_name"`  // 对端名称
	AlertType string    `json:"alert_type"` // 告警类型：high_usage、unusual_traffic、connection_lost
	Message   string    `json:"message"`    // 告警消息
	Threshold int64     `json:"threshold"`  // 阈值
	Actual    int64     `json:"actual"`     // 实际值
	Timestamp time.Time `json:"timestamp"`  // 告警时间
}

// ============================================================
// 服务管理相关类型
// ============================================================

// ServiceStatus 服务状态.
type ServiceStatus struct {
	Running       bool      `json:"running"`        // 是否运行中
	InterfaceName string    `json:"interface_name"` // 接口名称
	ListenPort    int       `json:"listen_port"`    // 监听端口
	PublicKey     string    `json:"public_key"`     // 服务端公钥
	PeerCount     int       `json:"peer_count"`     // 对端数量
	StartedAt     time.Time `json:"started_at"`     // 启动时间
	Uptime        string    `json:"uptime"`         // 运行时长
}

// FirewallRule 防火墙规则.
type FirewallRule struct {
	Port     int    `json:"port"`     // 端口
	Protocol string `json:"protocol"` // 协议：tcp/udp
	Action   string `json:"action"`   // 动作：allow/deny
	Source   string `json:"source"`   // 来源 IP
	Comment  string `json:"comment"`  // 备注
}

// PortForwardRule 端口转发规则.
type PortForwardRule struct {
	Name     string `json:"name"`      // 规则名称
	Protocol string `json:"protocol"`  // 协议：tcp/udp
	SrcPort  int    `json:"src_port"`  // 源端口
	DestIP   string `json:"dest_ip"`   // 目标 IP
	DestPort int    `json:"dest_port"` // 目标端口
	Enabled  bool   `json:"enabled"`   // 是否启用
}

// DNSConfig DNS 配置.
type DNSConfig struct {
	Enabled    bool        `json:"enabled"`     // 是否启用内置 DNS
	ListenAddr string      `json:"listen_addr"` // 监听地址
	Upstream   []string    `json:"upstream"`    // 上游 DNS 服务器
	Records    []DNSRecord `json:"records"`     // 自定义 DNS 记录
}

// DNSRecord DNS 记录.
type DNSRecord struct {
	Name  string `json:"name"`  // 域名
	Type  string `json:"type"`  // 记录类型：A/AAAA/CNAME
	Value string `json:"value"` // 记录值
}

// ============================================================
// 配置模板相关类型
// ============================================================

// ConfigTemplate 配置模板.
type ConfigTemplate struct {
	ID          string            `json:"id"`          // 模板 ID
	Name        string            `json:"name"`        // 模板名称
	Description string            `json:"description"` // 模板描述
	Category    string            `json:"category"`    // 分类：home/office/mobile/site-to-site
	Interface   TemplateInterface `json:"interface"`   // 接口配置
	Peer        TemplatePeer      `json:"peer"`        // 对端配置
}

// TemplateInterface 模板接口配置.
type TemplateInterface struct {
	Address    string `json:"address"`     // 地址
	ListenPort int    `json:"listen_port"` // 监听端口
	DNS        string `json:"dns"`         // DNS
	MTU        int    `json:"mtu"`         // MTU
}

// TemplatePeer 模板对端配置.
type TemplatePeer struct {
	AllowedIPs          string `json:"allowed_ips"`          // 允许的 IP
	PersistentKeepalive int    `json:"persistent_keepalive"` // 持久保持连接间隔
}

// ============================================================
// 一键部署相关类型
// ============================================================

// DeployRequest 一键部署请求.
type DeployRequest struct {
	Template      string `json:"template"`       // 使用的模板
	ServerAddress string `json:"server_address"` // 服务端公网地址
	ListenPort    int    `json:"listen_port"`    // 监听端口
	Network       string `json:"network"`        // VPN 网络（如 10.0.0.0/24）
	DNS           string `json:"dns"`            // DNS 服务器
	ClientCount   int    `json:"client_count"`   // 初始客户端数量
	EnableNAT     bool   `json:"enable_nat"`     // 启用 NAT
	EnableDNS     bool   `json:"enable_dns"`     // 启用内置 DNS
}

// DeployResult 一键部署结果.
type DeployResult struct {
	Success       bool           `json:"success"`        // 是否成功
	ServerConfig  ServerConfig   `json:"server_config"`  // 服务端配置
	Clients       []Peer         `json:"clients"`        // 创建的客户端列表
	FirewallRules []FirewallRule `json:"firewall_rules"` // 创建的防火墙规则
	Message       string         `json:"message"`        // 部署消息
}

// ============================================================
// 通用响应类型
// ============================================================

// APIResponse 标准 API 响应.
type APIResponse struct {
	Code    int         `json:"code"`           // 状态码
	Message string      `json:"message"`        // 消息
	Data    interface{} `json:"data,omitempty"` // 数据
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
}
