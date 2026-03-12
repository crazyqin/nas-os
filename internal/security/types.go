package security

import "time"

// ========== 防火墙类型 ==========

// FirewallRule 防火墙规则
type FirewallRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Action      string    `json:"action"`       // allow, deny, drop
	Protocol    string    `json:"protocol"`     // tcp, udp, icmp, all
	SourceIP    string    `json:"source_ip"`    // 源 IP/CIDR，空表示任意
	DestIP      string    `json:"dest_ip"`      // 目标 IP/CIDR，空表示任意
	SourcePort  string    `json:"source_port"`  // 源端口，空表示任意
	DestPort    string    `json:"dest_port"`    // 目标端口，空表示任意
	Direction   string    `json:"direction"`    // inbound, outbound
	Interface   string    `json:"interface"`    // 网络接口，空表示任意
	GeoLocation string    `json:"geo_location"` // 地理位置限制 (国家代码)
	Priority    int       `json:"priority"`     // 规则优先级，数字越小优先级越高
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IPBlacklistEntry IP 黑名单条目
type IPBlacklistEntry struct {
	IP        string     `json:"ip"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // 空表示永久
	CreatedAt time.Time  `json:"created_at"`
}

// IPWhitelistEntry IP 白名单条目
type IPWhitelistEntry struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// GeoRestriction 地理位置限制
type GeoRestriction struct {
	Enabled      bool     `json:"enabled"`
	Mode         string   `json:"mode"`          // allowlist, blocklist
	CountryCodes []string `json:"country_codes"` // 国家代码列表
}

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	Enabled        bool           `json:"enabled"`
	DefaultPolicy  string         `json:"default_policy"` // allow, deny
	IPv6Enabled    bool           `json:"ipv6_enabled"`
	GeoRestriction GeoRestriction `json:"geo_restriction"`
	LogDropped     bool           `json:"log_dropped"` // 记录被丢弃的数据包
}

// ========== 失败登录保护类型 ==========

// Fail2BanConfig 失败登录保护配置
type Fail2BanConfig struct {
	Enabled            bool     `json:"enabled"`
	MaxAttempts        int      `json:"max_attempts"`         // 最大失败尝试次数
	WindowMinutes      int      `json:"window_minutes"`       // 时间窗口（分钟）
	BanDurationMinutes int      `json:"ban_duration_minutes"` // 封禁时长（分钟）
	AutoUnban          bool     `json:"auto_unban"`           // 自动解封
	NotifyOnBan        bool     `json:"notify_on_ban"`        // 封禁时通知
	ProtectedServices  []string `json:"protected_services"`   // 保护的服务列表
}

// FailedLoginAttempt 失败登录尝试记录
type FailedLoginAttempt struct {
	IP        string    `json:"ip"`
	Username  string    `json:"username"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
	Reason    string    `json:"reason"`
}

// BannedIP 被封禁的 IP
type BannedIP struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	BannedAt  time.Time `json:"banned_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

// AccountLockout 账户锁定状态
type AccountLockout struct {
	Username    string     `json:"username"`
	LockedAt    time.Time  `json:"locked_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	FailedCount int        `json:"failed_count"`
}

// ========== 安全审计类型 ==========

// AuditLogEntry 审计日志条目
type AuditLogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`    // info, warning, error, critical
	Category  string                 `json:"category"` // auth, firewall, system, file, config
	Event     string                 `json:"event"`    // 事件类型
	UserID    string                 `json:"user_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Resource  string                 `json:"resource,omitempty"` // 操作的资源
	Action    string                 `json:"action,omitempty"`   // 执行的操作
	Details   map[string]interface{} `json:"details,omitempty"`  // 详细信息
	Status    string                 `json:"status"`             // success, failure
}

// LoginLogEntry 登录日志条目
type LoginLogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Status    string    `json:"status"` // success, failure
	Reason    string    `json:"reason,omitempty"`
	MFAMethod string    `json:"mfa_method,omitempty"`
}

// SecurityAlert 安全告警
type SecurityAlert struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Severity     string                 `json:"severity"` // low, medium, high, critical
	Type         string                 `json:"type"`     // 告警类型
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	SourceIP     string                 `json:"source_ip,omitempty"`
	Username     string                 `json:"username,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Acknowledged bool                   `json:"acknowledged"`
	AckedBy      string                 `json:"acked_by,omitempty"`
	AckedAt      *time.Time             `json:"acked_at,omitempty"`
}

// ========== 安全基线类型 ==========

// BaselineCheckResult 基线检查结果
type BaselineCheckResult struct {
	CheckID     string                 `json:"check_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"` // auth, network, system, file
	Severity    string                 `json:"severity"` // low, medium, high, critical
	Status      string                 `json:"status"`   // pass, fail, warning, skipped
	Message     string                 `json:"message"`
	Remediation string                 `json:"remediation,omitempty"` // 修复建议
	Details     map[string]interface{} `json:"details,omitempty"`
}

// BaselineReport 基线检查报告
type BaselineReport struct {
	ReportID     string                `json:"report_id"`
	Timestamp    time.Time             `json:"timestamp"`
	OverallScore int                   `json:"overall_score"` // 0-100
	TotalChecks  int                   `json:"total_checks"`
	Passed       int                   `json:"passed"`
	Failed       int                   `json:"failed"`
	Warning      int                   `json:"warning"`
	Skipped      int                   `json:"skipped"`
	Results      []BaselineCheckResult `json:"results"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Firewall       FirewallConfig `json:"firewall"`
	Fail2Ban       Fail2BanConfig `json:"fail2ban"`
	GeoRestriction GeoRestriction `json:"geo_restriction"`
	AuditEnabled   bool           `json:"audit_enabled"`
	AlertEnabled   bool           `json:"alert_enabled"`
}

// ========== 通用响应类型 ==========

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}
