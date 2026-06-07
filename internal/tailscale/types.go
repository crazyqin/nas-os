// Package tailscale 提供 Tailscale VPN 零配置组网功能
// 连接状态管理、节点发现、ACL 策略、子网路由、Exit Node、DNS 配置、认证密钥
package tailscale

import (
	"time"
)

// ========== Tailscale 状态 ==========

// TailscaleStatus Tailscale 连接状态
type TailscaleStatus struct {
	Connected   bool      `json:"connected"`   // 是否已连接
	NodeID      string    `json:"nodeId"`      // 本地节点 ID
	HostName    string    `json:"hostName"`    // 主机名
	TailnetName string    `json:"tailnetName"` // Tailnet 名称
	Version     string    `json:"version"`     // Tailscale 版本
	IPv4        string    `json:"ipv4"`        // IPv4 地址
	IPv6        string    `json:"ipv6"`        // IPv6 地址
	PublicKey   string    `json:"publicKey"`   // 公钥
	OS          string    `json:"os"`          // 操作系统
	Online      bool      `json:"online"`      // 是否在线
	LastSeen    time.Time `json:"lastSeen"`    // 最后在线时间
	StartedAt   time.Time `json:"startedAt"`   // 启动时间
}

// ========== 节点管理 ==========

// TailscaleNode Tailscale 节点
type TailscaleNode struct {
	ID          string    `json:"id"`          // 节点 ID
	HostName    string    `json:"hostName"`    // 主机名
	IPv4        string    `json:"ipv4"`        // IPv4 地址
	IPv6        string    `json:"ipv6"`        // IPv6 地址
	OS          string    `json:"os"`          // 操作系统
	Online      bool      `json:"online"`      // 是否在线
	LastSeen    time.Time `json:"lastSeen"`    // 最后在线时间
	Tags        []string  `json:"tags"`        // 标签
	Approved    bool      `json:"approved"`    // 是否已批准
	ExitNode    bool      `json:"exitNode"`    // 是否为出口节点
	SubnetRoute []string  `json:"subnetRoute"` // 子网路由
	PublicKey   string    `json:"publicKey"`   // 公钥
}

// ========== ACL 策略 ==========

// ACLPolicy ACL 策略
type ACLPolicy struct {
	Version   int       `json:"version"`   // 策略版本
	ACLs      []ACLRule `json:"acls"`      // ACL 规则列表
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// ACLRule ACL 规则
type ACLRule struct {
	Sources      []string `json:"sources"`      // 源地址/标签
	Destinations []string `json:"destinations"` // 目标地址/端口
	Action       string   `json:"action"`       // 动作 (accept/deny)
	Description  string   `json:"description"`  // 描述
}

// ========== 子网路由 ==========

// SubnetRoute 子网路由
type SubnetRoute struct {
	ID         string `json:"id"`         // 路由 ID
	CIDR       string `json:"cidr"`       // 路由 CIDR
	NodeID     string `json:"nodeId"`     // 节点 ID
	Enabled    bool   `json:"enabled"`    // 是否启用
	Advertised bool   `json:"advertised"` // 是否已通告
}

// ========== Exit Node ==========

// ExitNode 出口节点
type ExitNode struct {
	ID        string `json:"id"`        // 节点 ID
	IP        string `json:"ip"`        // IP 地址
	HostName  string `json:"hostName"`  // 主机名
	IsCurrent bool   `json:"isCurrent"` // 是否当前使用
	Latency   int    `json:"latency"`   // 延迟 (ms)
	Online    bool   `json:"online"`    // 是否在线
	Country   string `json:"country"`   // 国家/地区
}

// ========== DNS 配置 ==========

// DNSConfig DNS 配置
type DNSConfig struct {
	MagicDNSEnabled bool     `json:"magicDnsEnabled"` // 是否启用 MagicDNS
	Domains         []string `json:"domains"`         // 自定义域名
	Nameservers     []string `json:"nameservers"`     // DNS 服务器列表
	SearchDomains   []string `json:"searchDomains"`   // 搜索域
}

// ========== 认证密钥 ==========

// AuthKey 认证密钥
type AuthKey struct {
	ID          string     `json:"id"`          // 密钥 ID
	Key         string     `json:"key"`         // 密钥值
	Description string     `json:"description"` // 描述
	CreatedAt   time.Time  `json:"createdAt"`   // 创建时间
	ExpiresAt   *time.Time `json:"expiresAt"`   // 过期时间 (nil 表示不过期)
	Reusable    bool       `json:"reusable"`    // 是否可重复使用
	Ephemeral   bool       `json:"ephemeral"`   // 是否临时节点
	Revoked     bool       `json:"revoked"`     // 是否已撤销
	UsedCount   int        `json:"usedCount"`   // 使用次数
}

// ========== 流量统计 ==========

// TrafficStats 流量统计
type TrafficStats struct {
	InboundBytes  int64     `json:"inboundBytes"`  // 入站字节数
	OutboundBytes int64     `json:"outboundBytes"` // 出站字节数
	Connections   int       `json:"connections"`   // 连接数
	ActivePeers   int       `json:"activePeers"`   // 活跃 peer 数
	Latency       int       `json:"latency"`       // 平均延迟 (ms)
	PacketLoss    float64   `json:"packetLoss"`    // 丢包率 (%)
	Timestamp     time.Time `json:"timestamp"`     // 统计时间
}

// ========== 请求/响应结构 ==========

// ApproveNodeRequest 批准节点请求
type ApproveNodeRequest struct {
	Approved bool `json:"approved"` // 是否批准
}

// UpdateACLRequest 更新 ACL 请求
type UpdateACLRequest struct {
	ACLs []ACLRule `json:"acls" binding:"required"` // ACL 规则列表
}

// AddSubnetRouteRequest 添加子网路由请求
type AddSubnetRouteRequest struct {
	CIDR   string `json:"cidr" binding:"required"`   // 路由 CIDR
	NodeID string `json:"nodeId" binding:"required"` // 节点 ID
}

// ToggleSubnetRouteRequest 切换子网路由请求
type ToggleSubnetRouteRequest struct {
	Enabled bool `json:"enabled"` // 是否启用
}

// SelectExitNodeRequest 选择出口节点请求
type SelectExitNodeRequest struct {
	NodeID string `json:"nodeId" binding:"required"` // 节点 ID
}

// UpdateDNSRequest 更新 DNS 请求
type UpdateDNSRequest struct {
	MagicDNSEnabled *bool    `json:"magicDnsEnabled"` // 是否启用 MagicDNS
	Domains         []string `json:"domains"`         // 自定义域名
	Nameservers     []string `json:"nameservers"`     // DNS 服务器列表
}

// CreateAuthKeyRequest 创建认证密钥请求
type CreateAuthKeyRequest struct {
	Description string     `json:"description"`         // 描述
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"` // 过期时间
	Reusable    bool       `json:"reusable"`            // 是否可重复使用
	Ephemeral   bool       `json:"ephemeral"`           // 是否临时节点
}
