// Package remoteaccess 提供 P2P 远程访问功能
// NAT穿透 (UDP打洞 + STUN/TURN)、中继服务器、隧道加密、连接状态管理、带宽自适应、访问控制
package remoteaccess

import (
	"time"
)

// ========== NAT 穿透类型 ==========

// NATType NAT 类型.
type NATType string

const (
	NATTypeUnknown        NATType = "unknown"         // 未知
	NATTypeFullCone       NATType = "full_cone"       // 完全锥形
	NATTypeRestricted     NATType = "restricted"      // 受限锥形
	NATTypePortRestricted NATType = "port_restricted" // 端口受限锥形
	NATTypeSymmetric      NATType = "symmetric"       // 对称型
)

// ========== STUN/TURN 配置 ==========

// STUNServer STUN 服务器配置.
type STUNServer struct {
	ID       string `json:"id"`       // 服务器 ID
	Address  string `json:"address"`  // 地址 (host:port)
	Protocol string `json:"protocol"` // 协议 (udp/tcp/tls)
	Enabled  bool   `json:"enabled"`  // 是否启用
	Priority int    `json:"priority"` // 优先级
	Region   string `json:"region"`   // 区域
}

// TURNServer TURN 服务器配置.
type TURNServer struct {
	ID           string     `json:"id"`           // 服务器 ID
	Address      string     `json:"address"`      // 地址 (host:port)
	Protocol     string     `json:"protocol"`     // 协议 (udp/tcp/tls)
	Username     string     `json:"username"`     // 用户名
	Password     string     `json:"password"`     // 密码 (加密存储)
	Realm        string     `json:"realm"`        // 域
	Enabled      bool       `json:"enabled"`      // 是否启用
	Priority     int        `json:"priority"`     // 优先级
	ExpiresAt    *time.Time `json:"expiresAt"`    // 凭证过期时间
	Region       string     `json:"region"`       // 区域
	MaxBandwidth int64      `json:"maxBandwidth"` // 最大带宽 (bytes/s)
}

// NATDetectionResult NAT 检测结果.
type NATDetectionResult struct {
	NATType       NATType   `json:"natType"`       // NAT 类型
	ExternalIP    string    `json:"externalIp"`    // 外部 IP
	ExternalPort  int       `json:"externalPort"`  // 外部端口
	LocalIP       string    `json:"localIp"`       // 本地 IP
	LocalPort     int       `json:"localPort"`     // 本地端口
	MappingType   string    `json:"mappingType"`   // 映射类型
	FilteringType string    `json:"filteringType"` // 过滤类型
	SymmetricNAT  bool      `json:"symmetricNat"`  // 是否对称 NAT
	DetectedAt    time.Time `json:"detectedAt"`    // 检测时间
	STUNServer    string    `json:"stunServer"`    // 使用的 STUN 服务器
}

// ========== P2P 连接 ==========

// P2PConnectionStatus P2P 连接状态.
type P2PConnectionStatus string

const (
	P2PStatusIdle       P2PConnectionStatus = "idle"       // 空闲
	P2PStatusConnecting P2PConnectionStatus = "connecting" // 连接中
	P2PStatusNATHole    P2PConnectionStatus = "nat_hole"   // NAT 打洞中
	P2PStatusRelay      P2PConnectionStatus = "relay"      // 中继连接
	P2PStatusDirect     P2PConnectionStatus = "direct"     // 直连
	P2PStatusFailed     P2PConnectionStatus = "failed"     // 失败
	P2PStatusClosed     P2PConnectionStatus = "closed"     // 已关闭
)

// P2PConnection P2P 连接.
type P2PConnection struct {
	ID             string              `json:"id"`             // 连接 ID
	LocalPeerID    string              `json:"localPeerId"`    // 本地节点 ID
	RemotePeerID   string              `json:"remotePeerId"`   // 远程节点 ID
	RemoteAddr     string              `json:"remoteAddr"`     // 远程地址
	LocalAddr      string              `json:"localAddr"`      // 本地地址
	Status         P2PConnectionStatus `json:"status"`         // 连接状态
	ConnectionType string              `json:"connectionType"` // 连接类型 (direct/relay)
	NATType        NATType             `json:"natType"`        // NAT 类型
	RelayServerID  string              `json:"relayServerId"`  // 中继服务器 ID
	EstablishedAt  time.Time           `json:"establishedAt"`  // 建立时间
	LastActivity   time.Time           `json:"lastActivity"`   // 最后活动时间
	BytesSent      int64               `json:"bytesSent"`      // 发送字节数
	BytesReceived  int64               `json:"bytesReceived"`  // 接收字节数
	Latency        int                 `json:"latency"`        // 延迟 (ms)
	RTT            int                 `json:"rtt"`            // 往返时间 (ms)
	Encrypted      bool                `json:"encrypted"`      // 是否加密
	ReconnectCount int                 `json:"reconnectCount"` // 重连次数
}

// P2PSession P2P 会话.
type P2PSession struct {
	ID           string              `json:"id"`           // 会话 ID
	LocalPeerID  string              `json:"localPeerId"`  // 本地节点 ID
	RemotePeerID string              `json:"remotePeerId"` // 远程节点 ID
	Status       P2PConnectionStatus `json:"status"`       // 会话状态
	Connections  []P2PConnection     `json:"connections"`  // 连接列表
	CreatedAt    time.Time           `json:"createdAt"`    // 创建时间
	ExpiresAt    time.Time           `json:"expiresAt"`    // 过期时间
	Metadata     map[string]string   `json:"metadata"`     // 元数据
}

// ========== 中继服务器 ==========

// RelayServerStatus 中继服务器状态.
type RelayServerStatus string

const (
	RelayStatusOnline      RelayServerStatus = "online"      // 在线
	RelayStatusOffline     RelayServerStatus = "offline"     // 离线
	RelayStatusMaintenance RelayServerStatus = "maintenance" // 维护中
	RelayStatusOverloaded  RelayServerStatus = "overloaded"  // 过载
)

// RelayServer 中继服务器.
type RelayServer struct {
	ID            string            `json:"id"`            // 服务器 ID
	Name          string            `json:"name"`          // 名称
	Address       string            `json:"address"`       // 地址
	Port          int               `json:"port"`          // 端口
	Protocol      string            `json:"protocol"`      // 协议
	Status        RelayServerStatus `json:"status"`        // 状态
	Region        string            `json:"region"`        // 区域
	MaxCapacity   int               `json:"maxCapacity"`   // 最大容量
	CurrentLoad   int               `json:"currentLoad"`   // 当前负载
	Bandwidth     int64             `json:"bandwidth"`     // 总带宽 (bytes/s)
	UsedBandwidth int64             `json:"usedBandwidth"` // 已用带宽
	Latency       int               `json:"latency"`       // 延迟 (ms)
	Uptime        time.Duration     `json:"uptime"`        // 运行时间
	LastCheck     time.Time         `json:"lastCheck"`     // 最后检查时间
	TLSEnabled    bool              `json:"tlsEnabled"`    // 是否启用 TLS
	CertExpiry    *time.Time        `json:"certExpiry"`    // 证书过期时间
}

// RelayConnection 中继连接.
type RelayConnection struct {
	ID            string    `json:"id"`            // 连接 ID
	RelayServerID string    `json:"relayServerId"` // 中继服务器 ID
	LocalPeerID   string    `json:"localPeerId"`   // 本地节点 ID
	RemotePeerID  string    `json:"remotePeerId"`  // 远程节点 ID
	Status        string    `json:"status"`        // 状态
	BytesRelayed  int64     `json:"bytesRelayed"`  // 中继字节数
	EstablishedAt time.Time `json:"establishedAt"` // 建立时间
	LastActivity  time.Time `json:"lastActivity"`  // 最后活动时间
}

// ========== 隧道加密 ==========

// TLSConfig TLS 配置.
type TLSConfig struct {
	Enabled            bool     `json:"enabled"`            // 是否启用
	MinVersion         string   `json:"minVersion"`         // 最低版本
	MaxVersion         string   `json:"maxVersion"`         // 最高版本
	CipherSuites       []string `json:"cipherSuites"`       // 密码套件
	CertFile           string   `json:"certFile"`           // 证书文件
	KeyFile            string   `json:"keyFile"`            // 私钥文件
	CAFile             string   `json:"caFile"`             // CA 文件
	InsecureSkipVerify bool     `json:"insecureSkipVerify"` // 跳过验证
	AutoCert           bool     `json:"autoCert"`           // 自动证书
	CertDomain         string   `json:"certDomain"`         // 证书域名
}

// TunnelStatus 隧道状态.
type TunnelStatus struct {
	ID               string    `json:"id"`               // 隧道 ID
	LocalPeerID      string    `json:"localPeerId"`      // 本地节点 ID
	RemotePeerID     string    `json:"remotePeerId"`     // 远程节点 ID
	Protocol         string    `json:"protocol"`         // 协议
	LocalPort        int       `json:"localPort"`        // 本地端口
	RemotePort       int       `json:"remotePort"`       // 远程端口
	Encrypted        bool      `json:"encrypted"`        // 是否加密
	TLSVersion       string    `json:"tlsVersion"`       // TLS 版本
	CipherSuite      string    `json:"cipherSuite"`      // 密码套件
	BytesTransferred int64     `json:"bytesTransferred"` // 传输字节数
	Active           bool      `json:"active"`           // 是否活跃
	EstablishedAt    time.Time `json:"establishedAt"`    // 建立时间
	LastActivity     time.Time `json:"lastActivity"`     // 最后活动时间
}

// ========== 带宽自适应 ==========

// BandwidthPolicy 带宽策略.
type BandwidthPolicy string

const (
	BandwidthPolicyFixed    BandwidthPolicy = "fixed"    // 固定带宽
	BandwidthPolicyAdaptive BandwidthPolicy = "adaptive" // 自适应
	BandwidthPolicyPriority BandwidthPolicy = "priority" // 基于优先级
	BandwidthPolicyFair     BandwidthPolicy = "fair"     // 公平共享
)

// BandwidthConfig 带宽配置.
type BandwidthConfig struct {
	Policy          BandwidthPolicy `json:"policy"`          // 策略
	MaxBandwidth    int64           `json:"maxBandwidth"`    // 最大带宽 (bytes/s)
	MinBandwidth    int64           `json:"minBandwidth"`    // 最小带宽 (bytes/s)
	ReservedPercent float64         `json:"reservedPercent"` // 保留百分比
	BurstAllowed    bool            `json:"burstAllowed"`    // 允许突发
	BurstMaxBytes   int64           `json:"burstMaxBytes"`   // 突发最大字节数
	BurstDuration   time.Duration   `json:"burstDuration"`   // 突发持续时间
	QoSEnabled      bool            `json:"qosEnabled"`      // QoS 启用
}

// BandwidthStats 带宽统计.
type BandwidthStats struct {
	CurrentBandwidth int64     `json:"currentBandwidth"` // 当前带宽 (bytes/s)
	PeakBandwidth    int64     `json:"peakBandwidth"`    // 峰值带宽
	AvgBandwidth     int64     `json:"avgBandwidth"`     // 平均带宽
	TotalBytes       int64     `json:"totalBytes"`       // 总字节数
	Samples          int       `json:"samples"`          // 采样数
	Timestamp        time.Time `json:"timestamp"`        // 时间戳
}

// BandwidthSample 带宽采样.
type BandwidthSample struct {
	Timestamp    time.Time `json:"timestamp"`    // 时间戳
	BytesIn      int64     `json:"bytesIn"`      // 输入字节
	BytesOut     int64     `json:"bytesOut"`     // 输出字节
	Bandwidth    int64     `json:"bandwidth"`    // 带宽 (bytes/s)
	ConnectionID string    `json:"connectionId"` // 连接 ID
}

// ========== 访问控制 ==========

// AccessPolicy 访问策略.
type AccessPolicy string

const (
	AccessPolicyAllow AccessPolicy = "allow" // 允许
	AccessPolicyDeny  AccessPolicy = "deny"  // 拒绝
	AccessPolicyAuth  AccessPolicy = "auth"  // 需要认证
	AccessPolicyTFA   AccessPolicy = "tfa"   // 需要双因素认证
)

// Permission 权限.
type Permission string

const (
	PermissionConnect   Permission = "connect"    // 连接
	PermissionRelay     Permission = "relay"      // 中继
	PermissionAdmin     Permission = "admin"      // 管理
	PermissionViewStats Permission = "view_stats" // 查看统计
	PermissionManageACL Permission = "manage_acl" // 管理 ACL
	PermissionTunnel    Permission = "tunnel"     // 隧道
)

// AccessControlEntry 访问控制条目.
type AccessControlEntry struct {
	ID          string       `json:"id"`          // 条目 ID
	Subject     string       `json:"subject"`     // 主体 (用户/节点/组)
	Resource    string       `json:"resource"`    // 资源
	Permission  Permission   `json:"permission"`  // 权限
	Policy      AccessPolicy `json:"policy"`      // 策略
	Priority    int          `json:"priority"`    // 优先级
	Description string       `json:"description"` // 描述
	ExpiresAt   *time.Time   `json:"expiresAt"`   // 过期时间
	Enabled     bool         `json:"enabled"`     // 是否启用
	CreatedAt   time.Time    `json:"createdAt"`   // 创建时间
}

// ACLRule 远程访问 ACL 规则.
type ACLRule struct {
	ID          string       `json:"id"`          // 规则 ID
	Name        string       `json:"name"`        // 规则名称
	SourceNode  string       `json:"sourceNode"`  // 源节点 (或 *)
	DestNode    string       `json:"destNode"`    // 目标节点 (或 *)
	Ports       []int        `json:"ports"`       // 端口列表
	Protocol    string       `json:"protocol"`    // 协议
	Policy      AccessPolicy `json:"policy"`      // 策略
	Enabled     bool         `json:"enabled"`     // 是否启用
	Description string       `json:"description"` // 描述
}

// PeerAuth 节点认证.
type PeerAuth struct {
	PeerID     string    `json:"peerId"`     // 节点 ID
	PublicKey  string    `json:"publicKey"`  // 公钥
	AuthToken  string    `json:"authToken"`  // 认证令牌
	AuthMethod string    `json:"authMethod"` // 认证方法
	ExpiresAt  time.Time `json:"expiresAt"`  // 过期时间
	Trusted    bool      `json:"trusted"`    // 是否受信任
	LastAuth   time.Time `json:"lastAuth"`   // 最后认证时间
}

// ========== 连接统计 ==========

// ConnectionStats 连接统计.
type ConnectionStats struct {
	TotalConnections   int           `json:"totalConnections"`   // 总连接数
	ActiveConnections  int           `json:"activeConnections"`  // 活跃连接数
	DirectConnections  int           `json:"directConnections"`  // 直连数
	RelayConnections   int           `json:"relayConnections"`   // 中继连接数
	FailedConnections  int           `json:"failedConnections"`  // 失败连接数
	TotalBytesSent     int64         `json:"totalBytesSent"`     // 总发送字节
	TotalBytesReceived int64         `json:"totalBytesReceived"` // 总接收字节
	AvgLatency         int           `json:"avgLatency"`         // 平均延迟 (ms)
	PacketLossRate     float64       `json:"packetLossRate"`     // 丢包率 (%)
	Uptime             time.Duration `json:"uptime"`             // 运行时间
	Timestamp          time.Time     `json:"timestamp"`          // 时间戳
}

// ========== 请求/响应结构 ==========

// ConnectRequest 连接请求.
type ConnectRequest struct {
	RemotePeerID string `json:"remotePeerId" binding:"required"` // 远程节点 ID
	Protocol     string `json:"protocol"`                        // 协议
	ForceRelay   bool   `json:"forceRelay"`                      // 强制中继
}

// DisconnectRequest 断开请求.
type DisconnectRequest struct {
	ConnectionID string `json:"connectionId" binding:"required"` // 连接 ID
	Reason       string `json:"reason"`                          // 原因
}

// AddSTUNServerRequest 添加 STUN 服务器请求.
type AddSTUNServerRequest struct {
	Address  string `json:"address" binding:"required"` // 地址
	Protocol string `json:"protocol"`                   // 协议
	Region   string `json:"region"`                     // 区域
	Priority int    `json:"priority"`                   // 优先级
}

// AddTURNServerRequest 添加 TURN 服务器请求.
type AddTURNServerRequest struct {
	Address      string `json:"address" binding:"required"`  // 地址
	Protocol     string `json:"protocol"`                    // 协议
	Username     string `json:"username" binding:"required"` // 用户名
	Password     string `json:"password" binding:"required"` // 密码
	Realm        string `json:"realm"`                       // 域
	Region       string `json:"region"`                      // 区域
	Priority     int    `json:"priority"`                    // 优先级
	MaxBandwidth int64  `json:"maxBandwidth"`                // 最大带宽
}

// UpdateBandwidthConfigRequest 更新带宽配置请求.
type UpdateBandwidthConfigRequest struct {
	Policy          *string  `json:"policy"`          // 策略
	MaxBandwidth    *int64   `json:"maxBandwidth"`    // 最大带宽
	MinBandwidth    *int64   `json:"minBandwidth"`    // 最小带宽
	ReservedPercent *float64 `json:"reservedPercent"` // 保留百分比
	BurstAllowed    *bool    `json:"burstAllowed"`    // 允许突发
}

// AddACLRuleRequest 添加 ACL 规则请求.
type AddACLRuleRequest struct {
	Name        string       `json:"name" binding:"required"`   // 规则名称
	SourceNode  string       `json:"sourceNode"`                // 源节点
	DestNode    string       `json:"destNode"`                  // 目标节点
	Ports       []int        `json:"ports"`                     // 端口列表
	Protocol    string       `json:"protocol"`                  // 协议
	Policy      AccessPolicy `json:"policy" binding:"required"` // 策略
	Description string       `json:"description"`               // 描述
}

// UpdateRelayServerRequest 更新中继服务器请求.
type UpdateRelayServerRequest struct {
	Name        *string `json:"name"`        // 名称
	Address     *string `json:"address"`     // 地址
	Port        *int    `json:"port"`        // 端口
	MaxCapacity *int    `json:"maxCapacity"` // 最大容量
	Bandwidth   *int64  `json:"bandwidth"`   // 带宽
	TLSEnabled  *bool   `json:"tlsEnabled"`  // TLS 启用
}

// ConnectResponse 连接响应.
type ConnectResponse struct {
	ConnectionID string              `json:"connectionId"` // 连接 ID
	Status       P2PConnectionStatus `json:"status"`       // 状态
	Type         string              `json:"type"`         // 类型
	RelayUsed    bool                `json:"relayUsed"`    // 是否使用中继
}

// PeerInfo 节点信息.
type PeerInfo struct {
	ID           string    `json:"id"`           // 节点 ID
	HostName     string    `json:"hostName"`     // 主机名
	PublicKey    string    `json:"publicKey"`    // 公钥
	NATType      NATType   `json:"natType"`      // NAT 类型
	Online       bool      `json:"online"`       // 是否在线
	LastSeen     time.Time `json:"lastSeen"`     // 最后在线时间
	RelayCapable bool      `json:"relayCapable"` // 中继能力
}
