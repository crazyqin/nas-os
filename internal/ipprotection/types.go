// Package ipprotection 提供 IP 防护与入侵检测功能。
// 包含登录失败检测与自动封禁、IP 黑白名单管理、请求频率限制（令牌桶算法）、
// 异常行为检测（端口扫描、暴力破解模式识别）、IP 信誉评分系统、封禁日志与统计，
// 支持 IPv4/IPv6 双栈。
package ipprotection

import (
	"net"
	"sync"
	"time"
)

// ==================== 常量定义 ====================

// ListType 黑白名单类型
type ListType string

const (
	ListTypeAllow ListType = "allow" // 白名单
	ListTypeDeny  ListType = "deny"  // 黑名单
)

// BanReason 封禁原因
type BanReason string

const (
	BanReasonLoginFailure    BanReason = "login_failure"    // 登录失败次数过多
	BanReasonBruteForce      BanReason = "brute_force"      // 暴力破解
	BanReasonPortScan        BanReason = "port_scan"        // 端口扫描
	BanReasonRateLimit       BanReason = "rate_limit"       // 频率限制
	BanReasonSuspicious      BanReason = "suspicious"       // 可疑行为
	BanReasonManual          BanReason = "manual"           // 手动封禁
	BanReasonReputationLow   BanReason = "reputation_low"   // 信誉分过低
	BanReasonAbnormalPattern BanReason = "abnormal_pattern" // 异常模式
)

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// DetectionType 检测类型
type DetectionType string

const (
	DetectionPortScan     DetectionType = "port_scan"
	DetectionBruteForce   DetectionType = "brute_force"
	DetectionRapidRequest DetectionType = "rapid_request"
	DetectionSuspiciousUA DetectionType = "suspicious_ua"
	DetectionBotPattern   DetectionType = "bot_pattern"
)

// ==================== 配置 ====================

// IPProtectionConfig IP 防护配置
type IPProtectionConfig struct {
	// 登录失败检测
	LoginFailureThreshold int           `json:"login_failure_threshold" yaml:"login_failure_threshold"` // 失败次数阈值，默认 5
	LoginFailureWindow    time.Duration `json:"login_failure_window" yaml:"login_failure_window"`       // 检测窗口，默认 10 分钟
	AutoBanDuration       time.Duration `json:"auto_ban_duration" yaml:"auto_ban_duration"`             // 自动封禁时长，默认 1 小时
	EnableAutoBan         bool          `json:"enable_auto_ban" yaml:"enable_auto_ban"`                 // 是否启用自动封禁

	// 频率限制
	RateLimitRequestsPerSecond float64       `json:"rate_limit_rps" yaml:"rate_limit_rps"`         // 每秒请求数，默认 10
	RateLimitBurst             int           `json:"rate_limit_burst" yaml:"rate_limit_burst"`     // 突发容量，默认 20
	RateLimitCleanupInterval   time.Duration `json:"rate_limit_cleanup" yaml:"rate_limit_cleanup"` // 清理间隔

	// 异常检测
	PortScanThreshold   int           `json:"port_scan_threshold" yaml:"port_scan_threshold"`     // 端口扫描阈值，默认 20 个端口
	PortScanWindow      time.Duration `json:"port_scan_window" yaml:"port_scan_window"`           // 端口扫描检测窗口
	BruteForceThreshold int           `json:"brute_force_threshold" yaml:"brute_force_threshold"` // 暴力破解阈值
	BruteForceWindow    time.Duration `json:"brute_force_window" yaml:"brute_force_window"`       // 暴力破解检测窗口

	// 信誉评分
	InitialReputationScore int `json:"initial_reputation_score" yaml:"initial_reputation_score"` // 初始信誉分，默认 100
	MinReputationScore     int `json:"min_reputation_score" yaml:"min_reputation_score"`         // 最低信誉分，低于此值自动封禁
	LoginFailurePenalty    int `json:"login_failure_penalty" yaml:"login_failure_penalty"`       // 登录失败扣分，默认 10
	ScanPenalty            int `json:"scan_penalty" yaml:"scan_penalty"`                         // 端口扫描扣分，默认 30
	RateLimitPenalty       int `json:"rate_limit_penalty" yaml:"rate_limit_penalty"`             // 限流触发扣分，默认 5
	ReputationRecoverRate  int `json:"reputation_recover_rate" yaml:"reputation_recover_rate"`   // 每小时恢复分值，默认 1

	// 白名单 IP（始终放行）
	WhitelistedIPs []string `json:"whitelisted_ips" yaml:"whitelisted_ips"`
	// 黑名单 IP（始终拒绝）
	BlacklistedIPs []string `json:"blacklisted_ips" yaml:"blacklisted_ips"`
}

// DefaultIPProtectionConfig 返回默认配置
func DefaultIPProtectionConfig() *IPProtectionConfig {
	return &IPProtectionConfig{
		LoginFailureThreshold:      5,
		LoginFailureWindow:         10 * time.Minute,
		AutoBanDuration:            1 * time.Hour,
		EnableAutoBan:              true,
		RateLimitRequestsPerSecond: 10,
		RateLimitBurst:             20,
		RateLimitCleanupInterval:   5 * time.Minute,
		PortScanThreshold:          20,
		PortScanWindow:             5 * time.Minute,
		BruteForceThreshold:        10,
		BruteForceWindow:           5 * time.Minute,
		InitialReputationScore:     100,
		MinReputationScore:         20,
		LoginFailurePenalty:        10,
		ScanPenalty:                30,
		RateLimitPenalty:           5,
		ReputationRecoverRate:      1,
		WhitelistedIPs:             []string{"127.0.0.1", "::1"},
		BlacklistedIPs:             []string{},
	}
}

// ==================== 核心数据结构 ====================

// IPRecord IP 记录
type IPRecord struct {
	IP                 net.IP       `json:"ip"`
	IPString           string       `json:"ip_string"`
	IsIPv6             bool         `json:"is_ipv6"`
	ReputationScore    int          `json:"reputation_score"`
	IsBanned           bool         `json:"is_banned"`
	BanReason          BanReason    `json:"ban_reason,omitempty"`
	BanExpiry          time.Time    `json:"ban_expiry,omitempty"`
	BanCount           int          `json:"ban_count"`
	TotalLoginFailures int          `json:"total_login_failures"`
	TotalRequests      int64        `json:"total_requests"`
	FirstSeen          time.Time    `json:"first_seen"`
	LastSeen           time.Time    `json:"last_seen"`
	mu                 sync.RWMutex `json:"-"`
}

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	IP        string    `json:"ip"`
	Username  string    `json:"username,omitempty"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// AccessRecord 访问记录（用于端口扫描/暴力破解检测）
type AccessRecord struct {
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Path      string    `json:"path,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// BanRecord 封禁记录
type BanRecord struct {
	ID        string        `json:"id"`
	IP        string        `json:"ip"`
	IsIPv6    bool          `json:"is_ipv6"`
	Reason    BanReason     `json:"reason"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	IsActive  bool          `json:"is_active"`
	Details   string        `json:"details,omitempty"`
}

// IPStats IP 统计信息
type IPStats struct {
	IP              string      `json:"ip"`
	ReputationScore int         `json:"reputation_score"`
	ThreatLevel     ThreatLevel `json:"threat_level"`
	IsBanned        bool        `json:"is_banned"`
	BanReason       BanReason   `json:"ban_reason,omitempty"`
	BanExpiry       time.Time   `json:"ban_expiry,omitempty"`
	BanCount        int         `json:"ban_count"`
	TotalRequests   int64       `json:"total_requests"`
	FirstSeen       time.Time   `json:"first_seen"`
	LastSeen        time.Time   `json:"last_seen"`
	RecentFailures  int         `json:"recent_failures"`
	RecentPorts     int         `json:"recent_ports_scanned"`
}

// GlobalStats 全局统计
type GlobalStats struct {
	TotalIPsTracked  int       `json:"total_ips_tracked"`
	ActiveBans       int       `json:"active_bans"`
	TotalBans        int       `json:"total_bans"`
	WhitelistedIPs   int       `json:"whitelisted_ips"`
	BlacklistedIPs   int       `json:"blacklisted_ips"`
	AvgReputation    float64   `json:"avg_reputation"`
	LowReputationIPs int       `json:"low_reputation_ips"`
	LastUpdated      time.Time `json:"last_updated"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	Detected    bool          `json:"detected"`
	Type        DetectionType `json:"type"`
	ThreatLevel ThreatLevel   `json:"threat_level"`
	IP          string        `json:"ip"`
	Details     string        `json:"details"`
	Confidence  float64       `json:"confidence"` // 0.0 - 1.0
	Timestamp   time.Time     `json:"timestamp"`
}

// AllowListEntry 白名单条目
type AllowListEntry struct {
	IP        string    `json:"ip"`
	IsIPv6    bool      `json:"is_ipv6"`
	Subnet    string    `json:"subnet,omitempty"` // CIDR 表示，如 192.168.1.0/24
	Comment   string    `json:"comment,omitempty"`
	AddedAt   time.Time `json:"added_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"` // 空表示永不过期
	IsActive  bool      `json:"is_active"`
}

// DenyListEntry 黑名单条目
type DenyListEntry struct {
	IP        string    `json:"ip"`
	IsIPv6    bool      `json:"is_ipv6"`
	Subnet    string    `json:"subnet,omitempty"`
	Reason    BanReason `json:"reason"`
	Comment   string    `json:"comment,omitempty"`
	AddedAt   time.Time `json:"added_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	IsActive  bool      `json:"is_active"`
	AutoBan   bool      `json:"auto_ban"` // 是否自动封禁产生
}
