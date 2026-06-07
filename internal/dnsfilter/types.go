// Package dnsfilter 提供 DNS 广告过滤服务
package dnsfilter

import (
	"net"
	"time"
)

// DNSRecord DNS 记录类型
type DNSRecordType string

const (
	RecordTypeA     DNSRecordType = "A"
	RecordTypeAAAA  DNSRecordType = "AAAA"
	RecordTypeCNAME DNSRecordType = "CNAME"
)

// FilterAction 过滤动作
type FilterAction string

const (
	ActionBlock FilterAction = "block" // 阻止
	ActionAllow FilterAction = "allow" // 放行
	ActionDrop  FilterAction = "drop"  // 丢弃（不响应）
)

// FilterListType 规则列表类型
type FilterListType string

const (
	FilterListBlock FilterListType = "block" // 黑名单
	FilterListAllow FilterListType = "allow" // 白名单
)

// DNSRecord 自定义 DNS 记录.
type DNSRecord struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`  // 域名
	Type      DNSRecordType `json:"type"`  // A/AAAA/CNAME
	Value     string        `json:"value"` // IP 或目标域名
	TTL       int           `json:"ttl"`   // TTL（秒）
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// FilterRule 过滤规则.
type FilterRule struct {
	ID        string       `json:"id"`
	Pattern   string       `json:"pattern"` // 域名或正则模式
	Action    FilterAction `json:"action"`  // block/allow/drop
	ListID    string       `json:"list_id"` // 所属规则列表
	Enabled   bool         `json:"enabled"`
	HitCount  int64        `json:"hit_count"` // 命中次数
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// FilterList 过滤规则列表.
type FilterList struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        FilterListType `json:"type"`          // block/allow
	URL         string         `json:"url,omitempty"` // 订阅 URL
	Enabled     bool           `json:"enabled"`
	RuleCount   int            `json:"rule_count"`
	LastUpdated *time.Time     `json:"last_updated,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// UpstreamDNS 上游 DNS 服务器.
type UpstreamDNS struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`  // 如 "8.8.8.8:53", "https://dns.google/dns-query", "tls://dns.google"
	Protocol  string `json:"protocol"` // udp, tcp, doh, dot
	Enabled   bool   `json:"enabled"`
	Weight    int    `json:"weight"`     // 权重（用于负载均衡）
	IsDefault bool   `json:"is_default"` // 是否为默认上游
}

// DNSCacheEntry DNS 缓存条目.
type DNSCacheEntry struct {
	Domain    string    `json:"domain"`
	Type      string    `json:"type"`
	Answers   []string  `json:"answers"`
	TTL       int       `json:"ttl"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// QueryLog DNS 查询日志.
type QueryLog struct {
	ID         string       `json:"id"`
	Timestamp  time.Time    `json:"timestamp"`
	ClientIP   string       `json:"client_ip"`
	ClientMAC  string       `json:"client_mac,omitempty"` // 客户端 MAC 地址
	Domain     string       `json:"domain"`
	Type       string       `json:"type"`                  // A, AAAA, CNAME 等
	Answer     string       `json:"answer,omitempty"`      // 解析结果
	IsFiltered bool         `json:"is_filtered"`           // 是否被过滤
	FilterRule string       `json:"filter_rule,omitempty"` // 命中的规则
	Action     FilterAction `json:"action"`                // allow/block/drop
	Upstream   string       `json:"upstream,omitempty"`    // 使用的上游 DNS
	Duration   int64        `json:"duration"`              // 查询耗时（毫秒）
}

// FilterPolicy 过滤策略.
type FilterPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// 客户端匹配（MAC 或 IP）
	ClientMAC string `json:"client_mac,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
	// 时间段过滤
	StartTime string `json:"start_time,omitempty"` // HH:MM 格式
	EndTime   string `json:"end_time,omitempty"`   // HH:MM 格式
	Weekdays  []int  `json:"weekdays,omitempty"`   // 0=周日, 1=周一, ... 6=周六
	// 关联的规则列表
	BlockListIDs []string `json:"block_list_ids,omitempty"`
	AllowListIDs []string `json:"allow_list_ids,omitempty"`
	// 自定义上游 DNS
	UpstreamIDs []string  `json:"upstream_ids,omitempty"`
	Enabled     bool      `json:"enabled"`
	Priority    int       `json:"priority"` // 优先级，数值越大优先级越高
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 统计结构 ==========

// QueryStats 查询统计.
type QueryStats struct {
	TotalQueries   int64        `json:"total_queries"`
	BlockedQueries int64        `json:"blocked_queries"`
	AllowedQueries int64        `json:"allowed_queries"`
	BlockRate      float64      `json:"block_rate"` // 拦截率百分比
	UniqueDomains  int          `json:"unique_domains"`
	UniqueClients  int          `json:"unique_clients"`
	TopBlocked     []DomainStat `json:"top_blocked"`
	TopAllowed     []DomainStat `json:"top_allowed"`
	TopClients     []ClientStat `json:"top_clients"`
	HourlyStats    []HourlyStat `json:"hourly_stats"`
}

// DomainStat 域名统计.
type DomainStat struct {
	Domain   string `json:"domain"`
	Count    int64  `json:"count"`
	LastSeen string `json:"last_seen"`
}

// ClientStat 客户端统计.
type ClientStat struct {
	ClientIP  string `json:"client_ip"`
	ClientMAC string `json:"client_mac,omitempty"`
	Total     int64  `json:"total"`
	Blocked   int64  `json:"blocked"`
	Allowed   int64  `json:"allowed"`
}

// HourlyStat 每小时统计.
type HourlyStat struct {
	Hour    string `json:"hour"` // YYYY-MM-DD HH:00
	Total   int64  `json:"total"`
	Blocked int64  `json:"blocked"`
	Allowed int64  `json:"allowed"`
}

// DNSStatus DNS 服务状态.
type DNSStatus struct {
	Running        bool      `json:"running"`
	ListenAddr     string    `json:"listen_addr"`
	UDPPort        int       `json:"udp_port"`
	TCPPort        int       `json:"tcp_port"`
	TotalRules     int       `json:"total_rules"`
	CacheSize      int       `json:"cache_size"`
	Uptime         string    `json:"uptime"`
	StartTime      time.Time `json:"start_time"`
	QueriesServed  int64     `json:"queries_served"`
	QueriesBlocked int64     `json:"queries_blocked"`
}

// ========== 请求/响应结构 ==========

// CreateDNSRecordRequest 创建 DNS 记录请求.
type CreateDNSRecordRequest struct {
	Name  string        `json:"name" binding:"required"`
	Type  DNSRecordType `json:"type" binding:"required"`
	Value string        `json:"value" binding:"required"`
	TTL   int           `json:"ttl"`
}

// UpdateDNSRecordRequest 更新 DNS 记录请求.
type UpdateDNSRecordRequest struct {
	Name    *string        `json:"name,omitempty"`
	Type    *DNSRecordType `json:"type,omitempty"`
	Value   *string        `json:"value,omitempty"`
	TTL     *int           `json:"ttl,omitempty"`
	Enabled *bool          `json:"enabled,omitempty"`
}

// CreateFilterListRequest 创建过滤规则列表请求.
type CreateFilterListRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Type        FilterListType `json:"type" binding:"required"`
	URL         string         `json:"url,omitempty"` // 订阅 URL
}

// UpdateFilterListRequest 更新过滤规则列表请求.
type UpdateFilterListRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// CreateFilterRuleRequest 创建过滤规则请求.
type CreateFilterRuleRequest struct {
	Pattern string       `json:"pattern" binding:"required"`
	Action  FilterAction `json:"action" binding:"required"`
	ListID  string       `json:"list_id"`
}

// CreateUpstreamDNSRequest 创建上游 DNS 请求.
type CreateUpstreamDNSRequest struct {
	Name      string `json:"name" binding:"required"`
	Address   string `json:"address" binding:"required"`
	Protocol  string `json:"protocol" binding:"required"` // udp, tcp, doh, dot
	Weight    int    `json:"weight"`
	IsDefault bool   `json:"is_default"`
}

// UpdateUpstreamDNSRequest 更新上游 DNS 请求.
type UpdateUpstreamDNSRequest struct {
	Name      *string `json:"name,omitempty"`
	Address   *string `json:"address,omitempty"`
	Protocol  *string `json:"protocol,omitempty"`
	Weight    *int    `json:"weight,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
}

// CreateFilterPolicyRequest 创建过滤策略请求.
type CreateFilterPolicyRequest struct {
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description,omitempty"`
	ClientMAC    string   `json:"client_mac,omitempty"`
	ClientIP     string   `json:"client_ip,omitempty"`
	StartTime    string   `json:"start_time,omitempty"`
	EndTime      string   `json:"end_time,omitempty"`
	Weekdays     []int    `json:"weekdays,omitempty"`
	BlockListIDs []string `json:"block_list_ids,omitempty"`
	AllowListIDs []string `json:"allow_list_ids,omitempty"`
	UpstreamIDs  []string `json:"upstream_ids,omitempty"`
	Priority     int      `json:"priority"`
}

// UpdateFilterPolicyRequest 更新过滤策略请求.
type UpdateFilterPolicyRequest struct {
	Name         *string  `json:"name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	ClientMAC    *string  `json:"client_mac,omitempty"`
	ClientIP     *string  `json:"client_ip,omitempty"`
	StartTime    *string  `json:"start_time,omitempty"`
	EndTime      *string  `json:"end_time,omitempty"`
	Weekdays     []int    `json:"weekdays,omitempty"`
	BlockListIDs []string `json:"block_list_ids,omitempty"`
	AllowListIDs []string `json:"allow_list_ids,omitempty"`
	UpstreamIDs  []string `json:"upstream_ids,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	Priority     *int     `json:"priority,omitempty"`
}

// QueryLogRequest 查询日志请求.
type QueryLogRequest struct {
	ClientIP string `form:"client_ip"`
	Domain   string `form:"domain"`
	Action   string `form:"action"` // allow/block
	Since    string `form:"since"`  // ISO 8601 时间
	Limit    int    `form:"limit"`
}

// TestDNSRequest 测试 DNS 解析请求.
type TestDNSRequest struct {
	Domain string `json:"domain" binding:"required"`
	Type   string `json:"type"` // A, AAAA, CNAME
}

// TestDNSResponse 测试 DNS 解析响应.
type TestDNSResponse struct {
	Domain     string   `json:"domain"`
	Type       string   `json:"type"`
	IsFiltered bool     `json:"is_filtered"`
	Action     string   `json:"action,omitempty"`
	Answers    []string `json:"answers,omitempty"`
	Source     string   `json:"source"` // cache/upstream/custom
	RuleMatch  string   `json:"rule_match,omitempty"`
	Duration   int64    `json:"duration"` // 毫秒
}

// ========== 用于过滤匹配的内部结构 ==========

// filterMatch 过滤匹配结果.
type filterMatch struct {
	Matched bool
	Action  FilterAction
	Rule    string // 命中的规则内容
	ListID  string
}

// clientInfo 客户端信息.
type clientInfo struct {
	IP      net.IP
	MAC     string
	Profile *FilterPolicy // 匹配到的策略
}

// cacheKey 缓存键.
type cacheKey struct {
	Domain string
	Type   string
}

// ========== SSE 实时日志流 ==========

// LogStreamEvent 日志流事件.
type LogStreamEvent struct {
	ID         string       `json:"id"`
	Timestamp  time.Time    `json:"timestamp"`
	ClientIP   string       `json:"client_ip"`
	Domain     string       `json:"domain"`
	Type       string       `json:"type"`
	Answer     string       `json:"answer,omitempty"`
	IsFiltered bool         `json:"is_filtered"`
	Action     FilterAction `json:"action"`
	Duration   int64        `json:"duration"`
}
