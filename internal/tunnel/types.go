// Package tunnel 提供内网穿透服务
// 支持 NAT 穿透（STUN/TURN）、反向代理、P2P 直连和转发模式
package tunnel

import (
	"context"
	"net"
	"time"
)

// ========== 隧道模式 ==========

// TunnelMode 隧道连接模式
type TunnelMode string

const (
	// ModeP2P P2P 直连模式（通过 STUN 打洞）
	ModeP2P TunnelMode = "p2p"
	// ModeRelay 中继转发模式（通过 TURN 服务器）
	ModeRelay TunnelMode = "relay"
	// ModeReverse 反向代理模式（客户端主动连接服务端）
	ModeReverse TunnelMode = "reverse"
	// ModeAuto 自动选择模式（优先 P2P，失败则中继）
	ModeAuto TunnelMode = "auto"
)

// ========== 隧道状态 ==========

// TunnelState 隧道状态
type TunnelState string

const (
	// StateDisconnected 未连接
	StateDisconnected TunnelState = "disconnected"
	// StateConnecting 连接中
	StateConnecting TunnelState = "connecting"
	// StateConnected 已连接
	StateConnected TunnelState = "connected"
	// StateReconnecting 重连中
	StateReconnecting TunnelState = "reconnecting"
	// StateError 错误状态
	StateError TunnelState = "error"
)

// ========== NAT 类型 ==========

// NATType NAT 类型（RFC 3489）
type NATType string

const (
	// NATTypeUnknown 未知
	NATTypeUnknown NATType = "unknown"
	// NATTypeNone 无 NAT（公网 IP）
	NATTypeNone NATType = "none"
	// NATTypeFullCone 全锥形 NAT（最宽松）
	NATTypeFullCone NATType = "full_cone"
	// NATTypeRestrictedCone 受限锥形 NAT
	NATTypeRestrictedCone NATType = "restricted_cone"
	// NATTypePortRestrictedCone 端口受限锥形 NAT
	NATTypePortRestrictedCone NATType = "port_restricted_cone"
	// NATTypeSymmetric 对称型 NAT（最严格）
	NATTypeSymmetric NATType = "symmetric"
)

// ========== 配置结构 ==========

// Config 隧道管理器配置
type Config struct {
	// 服务端配置
	ServerAddr string `json:"server_addr"` // 隧道服务器地址
	ServerPort int    `json:"server_port"` // 隧道服务器端口
	AuthToken  string `json:"auth_token"`  // 认证令牌
	DeviceID   string `json:"device_id"`   // 设备唯一标识
	DeviceName string `json:"device_name"` // 设备名称

	// STUN/TURN 配置
	STUNServers []string `json:"stun_servers"` // STUN 服务器列表
	TURNServers []string `json:"turn_servers"` // TURN 服务器列表
	TURNUser    string   `json:"turn_user"`    // TURN 用户名
	TURNPass    string   `json:"turn_pass"`    // TURN 密码

	// 连接配置
	Mode         TunnelMode `json:"mode"`          // 连接模式
	HeartbeatInt int        `json:"heartbeat_int"` // 心跳间隔（秒）
	ReconnectInt int        `json:"reconnect_int"` // 重连间隔（秒）
	MaxReconnect int        `json:"max_reconnect"` // 最大重连次数
	Timeout      int        `json:"timeout"`       // 连接超时（秒）

	// 端口映射配置
	EnablePortMapping bool     `json:"enable_port_mapping"` // 是否启用端口映射
	LocalPorts        []int    `json:"local_ports"`         // 本地需要映射的端口
	RemotePorts       []int    `json:"remote_ports"`        // 远程端口（可选，自动分配则为空）
	AllowedIPs        []string `json:"allowed_ips"`         // 允许访问的 IP 白名单

	// 安全配置
	EnableTLS     bool   `json:"enable_tls"`     // 是否启用 TLS
	TLSCertFile   string `json:"tls_cert_file"`  // TLS 证书文件
	TLSKeyFile    string `json:"tls_key_file"`   // TLS 密钥文件
	AllowInsecure bool   `json:"allow_insecure"` // 允许不安全连接（测试用）
}

// TunnelConfig 单个隧道配置
type TunnelConfig struct {
	ID          string     `json:"id"`          // 隧道唯一标识
	Name        string     `json:"name"`        // 隧道名称
	Mode        TunnelMode `json:"mode"`        // 连接模式
	LocalAddr   string     `json:"local_addr"`  // 本地监听地址（host:port）
	RemotePort  int        `json:"remote_port"` // 远程端口
	Protocol    string     `json:"protocol"`    // 协议：tcp, udp
	Description string     `json:"description"` // 描述
	Enabled     bool       `json:"enabled"`     // 是否启用
	CreatedAt   time.Time  `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`  // 更新时间
}

// ========== 隧道状态结构 ==========

// TunnelStatus 隧道状态信息
type TunnelStatus struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Mode          TunnelMode  `json:"mode"`
	State         TunnelState `json:"state"`
	LocalAddr     string      `json:"local_addr"`     // 本地地址
	PublicAddr    string      `json:"public_addr"`    // 公网地址（通过 STUN 获取）
	RemoteAddr    string      `json:"remote_addr"`    // 远程地址
	BytesSent     int64       `json:"bytes_sent"`     // 发送字节数
	BytesReceived int64       `json:"bytes_received"` // 接收字节数
	Connections   int         `json:"connections"`    // 当前连接数
	Uptime        int64       `json:"uptime"`         // 运行时间（秒）
	LastError     string      `json:"last_error"`     // 最后错误信息
	LastConnected time.Time   `json:"last_connected"` // 最后连接时间
	NATType       NATType     `json:"nat_type"`       // NAT 类型
	PeerAddr      string      `json:"peer_addr"`      // 对端地址（P2P 模式）
	RelayAddr     string      `json:"relay_addr"`     // 中继地址（Relay 模式）
}

// ManagerStatus 管理器状态
type ManagerStatus struct {
	State         TunnelState    `json:"state"`
	NATType       NATType        `json:"nat_type"`
	PublicIP      string         `json:"public_ip"`
	PublicPort    int            `json:"public_port"`
	Tunnels       []TunnelStatus `json:"tunnels"`
	ActiveTunnels int            `json:"active_tunnels"`
	TotalBytesTx  int64          `json:"total_bytes_tx"`
	TotalBytesRx  int64          `json:"total_bytes_rx"`
	ServerLatency int64          `json:"server_latency"` // 服务器延迟（毫秒）
	StartTime     time.Time      `json:"start_time"`
}

// ========== 连接信息 ==========

// Connection 表示一个隧道连接
type Connection struct {
	ID          string
	TunnelID    string
	LocalAddr   net.Addr
	RemoteAddr  net.Addr
	Established time.Time
	LastActive  time.Time
	BytesSent   int64
	BytesRecv   int64
	Closed      bool
}

// PeerInfo 对端节点信息
type PeerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PublicKey string    `json:"public_key"`
	Endpoints []string  `json:"endpoints"` // 可用的端点列表
	NATType   NATType   `json:"nat_type"`
	LastSeen  time.Time `json:"last_seen"`
}

// ========== API 请求/响应结构 ==========

// ConnectRequest 建立隧道请求
type ConnectRequest struct {
	Name        string     `json:"name" binding:"required"`
	Mode        TunnelMode `json:"mode"`
	LocalPort   int        `json:"local_port" binding:"required,min=1,max=65535"`
	RemotePort  int        `json:"remote_port"` // 0 表示自动分配
	Protocol    string     `json:"protocol" binding:"omitempty,oneof=tcp udp"`
	Description string     `json:"description"`
}

// ConnectResponse 建立隧道响应
type ConnectResponse struct {
	TunnelID   string      `json:"tunnel_id"`
	Name       string      `json:"name"`
	Mode       TunnelMode  `json:"mode"`
	State      TunnelState `json:"state"`
	LocalAddr  string      `json:"local_addr"`
	PublicAddr string      `json:"public_addr,omitempty"`
	Message    string      `json:"message"`
}

// StatusResponse 状态响应
type StatusResponse struct {
	State         TunnelState    `json:"state"`
	NATType       NATType        `json:"nat_type"`
	PublicIP      string         `json:"public_ip,omitempty"`
	PublicPort    int            `json:"public_port,omitempty"`
	Tunnels       []TunnelStatus `json:"tunnels"`
	ActiveTunnels int            `json:"active_tunnels"`
	ServerLatency int64          `json:"server_latency_ms"`
	Uptime        int64          `json:"uptime_seconds"`
}

// DisconnectRequest 断开隧道请求
type DisconnectRequest struct {
	TunnelID string `json:"tunnel_id" binding:"required"`
}

// DisconnectResponse 断开隧道响应
type DisconnectResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ========== 事件类型 ==========

// EventType 事件类型
type EventType string

const (
	// EventTunnelCreated 隧道创建
	EventTunnelCreated EventType = "tunnel_created"
	// EventTunnelConnected 隧道连接成功
	EventTunnelConnected EventType = "tunnel_connected"
	// EventTunnelDisconnected 隧道断开
	EventTunnelDisconnected EventType = "tunnel_disconnected"
	// EventTunnelError 隧道错误
	EventTunnelError EventType = "tunnel_error"
	// EventNATDetected NAT 类型检测完成
	EventNATDetected EventType = "nat_detected"
	// EventPeerDiscovered 发现对端节点
	EventPeerDiscovered EventType = "peer_discovered"
)

// Event 隧道事件
type Event struct {
	Type      EventType   `json:"type"`
	TunnelID  string      `json:"tunnel_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// EventCallback 事件回调函数
type EventCallback func(Event)

// ========== 接口定义 ==========

// TunnelClient 隧道客户端接口
type TunnelClient interface {
	// Connect 连接到隧道服务器
	Connect(ctx context.Context) error
	// Disconnect 断开连接
	Disconnect() error
	// GetStatus 获取连接状态
	GetStatus() TunnelStatus
	// Send 发送数据
	Send(data []byte) (int, error)
	// Receive 接收数据
	Receive() ([]byte, error)
	// IsConnected 检查是否已连接
	IsConnected() bool
}

// TunnelServer 隧道服务端接口
type TunnelServer interface {
	// Start 启动服务器
	Start(ctx context.Context) error
	// Stop 停止服务器
	Stop() error
	// AddTunnel 添加隧道配置
	AddTunnel(config TunnelConfig) error
	// RemoveTunnel 移除隧道
	RemoveTunnel(id string) error
	// GetTunnelStatus 获取隧道状态
	GetTunnelStatus(id string) (TunnelStatus, error)
	// ListTunnels 列出所有隧道
	ListTunnels() []TunnelStatus
}

// NATDetector NAT 类型检测接口
type NATDetector interface {
	// Detect 检测 NAT 类型
	Detect(ctx context.Context) (NATType, string, int, error)
	// GetPublicAddr 获取公网地址
	GetPublicAddr() (string, int, error)
}

// STUNClient STUN 客户端接口
type STUNClient interface {
	// Discover 发现公网地址
	Discover(ctx context.Context, server string) (net.Addr, error)
	// GetNATType 获取 NAT 类型
	GetNATType(ctx context.Context) (NATType, error)
}

// TURNClient TURN 客户端接口
type TURNClient interface {
	// Allocate 分配中继地址
	Allocate(ctx context.Context) (net.Addr, error)
	// CreatePermission 创建权限
	CreatePermission(peer net.Addr) error
	// Bind 绑定通道
	Bind(peer net.Addr) (int, error)
	// SendTo 发送数据到对端
	SendTo(data []byte, peer net.Addr) (int, error)
	// ReceiveFrom 接收数据
	ReceiveFrom() ([]byte, net.Addr, error)
	// Close 关闭连接
	Close() error
}
