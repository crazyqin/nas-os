// Package dnsmanager 提供 DNS 管理服务器功能
package dnsmanager

import (
	"time"
)

// DNSRecordType DNS 记录类型
type DNSRecordType string

const (
	RecordTypeA     DNSRecordType = "A"
	RecordTypeAAAA  DNSRecordType = "AAAA"
	RecordTypeCNAME DNSRecordType = "CNAME"
	RecordTypeMX    DNSRecordType = "MX"
	RecordTypeTXT   DNSRecordType = "TXT"
	RecordTypeNS    DNSRecordType = "NS"
	RecordTypeSRV   DNSRecordType = "SRV"
)

// RuleAction 规则动作
type RuleAction string

const (
	ActionBlock    RuleAction = "block"    // 阻止
	ActionAllow    RuleAction = "allow"    // 放行
	ActionRedirect RuleAction = "redirect" // 重定向
)

// UpstreamProtocol 上游协议
type UpstreamProtocol string

const (
	ProtocolUDP UpstreamProtocol = "udp"
	ProtocolTCP UpstreamProtocol = "tcp"
	ProtocolDoH UpstreamProtocol = "doh"
	ProtocolDoT UpstreamProtocol = "dot"
)

// DNSRecord DNS 记录
type DNSRecord struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`       // 域名
	Type      DNSRecordType `json:"type"`       // A/AAAA/CNAME/MX/TXT/NS/SRV
	Value     string        `json:"value"`      // 记录值
	TTL       int           `json:"ttl"`        // TTL（秒）
	Priority  int           `json:"priority"`   // 优先级（MX/SRV 使用）
	CreatedAt time.Time     `json:"created_at"`
	Enabled   bool          `json:"enabled"`
}

// DNSZone DNS 区域
type DNSZone struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`     // 区域名称（如 example.com）
	Records  map[string][]DNSRecord `json:"records"`  // 域名 -> 记录列表
	Serial   uint32               `json:"serial"`   // SOA 序列号
	Refresh  int                  `json:"refresh"`  // SOA 刷新间隔（秒）
	Retry    int                  `json:"retry"`    // SOA 重试间隔（秒）
	Expire   int                  `json:"expire"`   // SOA 过期时间（秒）
	Minimum  int                  `json:"minimum"`  // SOA 最小 TTL（秒）
}

// DNSRule DNS 过滤规则
type DNSRule struct {
	ID        string     `json:"id"`
	Pattern   string     `json:"pattern"`    // 域名模式（支持通配符）
	Action    RuleAction `json:"action"`     // block/allow/redirect
	Target    string     `json:"target"`     // 重定向目标（redirect 时使用）
	Enabled   bool       `json:"enabled"`
	Category  string     `json:"category"`   // 规则分类（如 ads, malware, tracking）
	HitCount  int64      `json:"hit_count"`  // 命中次数
	CreatedAt time.Time  `json:"created_at"`
}

// DNSQuery DNS 查询日志
type DNSQuery struct {
	ID           string    `json:"id"`
	Client       string    `json:"client"`        // 客户端 IP
	Domain       string    `json:"domain"`        // 查询域名
	Type         string    `json:"type"`          // 查询类型
	Timestamp    time.Time `json:"timestamp"`     // 查询时间
	Blocked      bool      `json:"blocked"`       // 是否被拦截
	ResponseTime int64     `json:"response_time"` // 响应时间（毫秒）
}

// DNSStats DNS 统计信息
type DNSStats struct {
	TotalQueries   int64         `json:"total_queries"`
	BlockedQueries int64         `json:"blocked_queries"`
	AllowedQueries int64         `json:"allowed_queries"`
	TopDomains     []DomainStat  `json:"top_domains"`
	TopClients     []ClientStat  `json:"top_clients"`
	TopBlocked     []DomainStat  `json:"top_blocked"`
}

// DomainStat 域名统计
type DomainStat struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

// ClientStat 客户端统计
type ClientStat struct {
	Client string `json:"client"`
	Count  int64  `json:"count"`
}

// UpstreamServer 上游 DNS 服务器
type UpstreamServer struct {
	ID       string           `json:"id"`
	Address  string           `json:"address"`  // 如 8.8.8.8
	Port     int              `json:"port"`     // 端口号
	Protocol UpstreamProtocol `json:"protocol"` // udp/tcp/doh/dot
	Enabled  bool             `json:"enabled"`
	Latency  int64            `json:"latency"`  // 延迟（毫秒）
}

// ========== 请求/响应结构 ==========

// CreateRecordRequest 创建记录请求
type CreateRecordRequest struct {
	Zone    string        `json:"zone" binding:"required"`
	Name    string        `json:"name" binding:"required"`
	Type    DNSRecordType `json:"type" binding:"required"`
	Value   string        `json:"value" binding:"required"`
	TTL     int           `json:"ttl"`
	Priority int          `json:"priority"`
}

// UpdateRecordRequest 更新记录请求
type UpdateRecordRequest struct {
	Name     *string        `json:"name,omitempty"`
	Type     *DNSRecordType `json:"type,omitempty"`
	Value    *string        `json:"value,omitempty"`
	TTL      *int           `json:"ttl,omitempty"`
	Priority *int           `json:"priority,omitempty"`
	Enabled  *bool          `json:"enabled,omitempty"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Pattern  string     `json:"pattern" binding:"required"`
	Action   RuleAction `json:"action" binding:"required"`
	Target   string     `json:"target"`
	Category string     `json:"category"`
}

// UpdateRuleRequest 更新规则请求
type UpdateRuleRequest struct {
	Pattern  *string     `json:"pattern,omitempty"`
	Action   *RuleAction `json:"action,omitempty"`
	Target   *string     `json:"target,omitempty"`
	Category *string     `json:"category,omitempty"`
	Enabled  *bool       `json:"enabled,omitempty"`
}

// CreateUpstreamRequest 创建上游服务器请求
type CreateUpstreamRequest struct {
	Address  string           `json:"address" binding:"required"`
	Port     int              `json:"port" binding:"required"`
	Protocol UpstreamProtocol `json:"protocol" binding:"required"`
}

// ImportBlockListRequest 导入拦截列表请求
type ImportBlockListRequest struct {
	URL string `json:"url" binding:"required"`
}

// ResolveRequest DNS 解析请求
type ResolveRequest struct {
	Domain string `json:"domain" binding:"required"`
	Type   string `json:"type"`
}

// QueryLogRequest 查询日志请求
type QueryLogRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// StatsRequest 统计请求
type StatsRequest struct {
	Period string `json:"period"` // hour, day, week, month
}

// ========== 内部结构 ==========

// domainKey 域名缓存键
type domainKey struct {
	Domain string
	Type   string
}
