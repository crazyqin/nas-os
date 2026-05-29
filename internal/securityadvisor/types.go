// Package securityadvisor provides comprehensive security auditing and advisory functionality.
// 系统安全审计顾问 - 参考群晖 Security Advisor 设计
// 支持安全评分、漏洞扫描、恶意软件检测、网络安全、账户安全、
// 文件完整性监控、加固建议、合规检查、审计日志、威胁情报集成
package securityadvisor

import (
	"time"
)

// ============================================================
// 安全评分与等级
// ============================================================

// SecurityScore 综合安全评分
type SecurityScore struct {
	Overall      int       `json:"overall"`       // 0-100 综合安全分数
	Vulnerability int      `json:"vulnerability"` // 漏洞评分
	Malware       int      `json:"malware"`       // 恶意软件评分
	Network       int      `json:"network"`       // 网络安全评分
	Account       int      `json:"account"`       // 账户安全评分
	FileIntegrity int      `json:"file_integrity"` // 文件完整性评分
	Compliance    int      `json:"compliance"`    // 合规评分
	Level         string   `json:"level"`         // "good", "warning", "critical", "danger"
	Grade         string   `json:"grade"`         // "A", "B", "C", "D", "F"
	UpdatedAt     time.Time `json:"updated_at"`
	Summary       string    `json:"summary,omitempty"`
	Password      int       `json:"password,omitempty"`
	Port          int       `json:"port,omitempty"`
	Permission    int       `json:"permission,omitempty"`
	SSL           int       `json:"ssl,omitempty"`
	Update        int       `json:"update,omitempty"`
	Firewall      int       `json:"firewall,omitempty"`
}

// ScoreWeight 评分权重
type ScoreWeight struct {
	Vulnerability float64 `json:"vulnerability"`
	Malware       float64 `json:"malware"`
	Network       float64 `json:"network"`
	Account       float64 `json:"account"`
	FileIntegrity float64 `json:"file_integrity"`
	Compliance    float64 `json:"compliance"`
	Password      float64 `json:"password"`
	Port          float64 `json:"port"`
	Permission    float64 `json:"permission"`
	SSL           float64 `json:"ssl"`
	Update        float64 `json:"update"`
	Firewall      float64 `json:"firewall"`
}

// DefaultScoreWeight 默认评分权重
func DefaultScoreWeight() ScoreWeight {
	return ScoreWeight{
		Vulnerability: 0.15,
		Malware:       0.10,
		Network:       0.10,
		Account:       0.10,
		FileIntegrity: 0.05,
		Compliance:    0.05,
		Password:      0.15,
		Port:          0.10,
		Permission:    0.05,
		SSL:           0.05,
		Update:        0.05,
		Firewall:      0.05,
	}
}

// ============================================================
// 安全检查
// ============================================================

// SecurityCheck 安全检查项
type SecurityCheck struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Category    string        `json:"category"` // "vulnerability", "malware", "network", "account", "file_integrity", "compliance"
	Severity    string        `json:"severity"` // "critical", "high", "medium", "low", "info"
	Status      string        `json:"status"`   // "pass", "fail", "warning", "error"
	Score       int           `json:"score"`    // 0-100
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Message     string        `json:"message,omitempty"`
	Details     string        `json:"details,omitempty"`
	Evidence    string        `json:"evidence,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
	Reference   string        `json:"reference,omitempty"` // CVE ID 或标准参考
	CheckedAt   time.Time     `json:"checked_at"`
	Duration    time.Duration `json:"duration"`
}

// ============================================================
// 漏洞扫描
// ============================================================

// VulnerabilityScan 漏洞扫描配置
type VulnerabilityScan struct {
	Enabled          bool     `json:"enabled"`
	ScanSystem       bool     `json:"scan_system"`       // 系统漏洞
	ScanServices     bool     `json:"scan_services"`     // 服务漏洞
	ScanConfig       bool     `json:"scan_config"`       // 配置漏洞
	ScanPackages     bool     `json:"scan_packages"`     // 软件包漏洞
	ScanPorts        bool     `json:"scan_ports"`        // 端口漏洞
	ExcludeCVEs      []string `json:"exclude_cves"`      // 排除的CVE
	CriticalPorts    []int    `json:"critical_ports"`    // 关键端口
	MaxConcurrent    int      `json:"max_concurrent"`    // 最大并发
}

// DefaultVulnerabilityScan 默认漏洞扫描配置
func DefaultVulnerabilityScan() VulnerabilityScan {
	return VulnerabilityScan{
		Enabled:       true,
		ScanSystem:    true,
		ScanServices:  true,
		ScanConfig:    true,
		ScanPackages:  true,
		ScanPorts:     true,
		ExcludeCVEs:   []string{},
		CriticalPorts: []int{22, 80, 443, 3306, 5432, 6379, 8080, 8443},
		MaxConcurrent: 10,
	}
}

// Vulnerability 漏洞详情
type Vulnerability struct {
	ID          string    `json:"id"`          // CVE ID 或自定义ID
	Severity    string    `json:"severity"`    // "critical", "high", "medium", "low"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Affected    string    `json:"affected"`    // 受影响组件
	FixedIn     string    `json:"fixed_in"`    // 修复版本
	Reference   string    `json:"reference"`   // 参考链接
	DetectedAt  time.Time `json:"detected_at"`
	CVSS        float64   `json:"cvss"`        // CVSS评分
}

// ============================================================
// 恶意软件检测
// ============================================================

// MalwareDetection 恶意软件检测配置
type MalwareDetection struct {
	Enabled          bool     `json:"enabled"`
	ScanSignatures   bool     `json:"scan_signatures"`   // 特征扫描
	ScanBehavior     bool     `json:"scan_behavior"`     // 行为检测
	ScanPaths        []string `json:"scan_paths"`        // 扫描路径
	ExcludePaths     []string `json:"exclude_paths"`     // 排除路径
	SignatureDBPath  string   `json:"signature_db_path"` // 特征库路径
	MaxFileSize      int64    `json:"max_file_size"`     // 最大文件大小
	QuarantinePath   string   `json:"quarantine_path"`   // 隔离路径
}

// DefaultMalwareDetection 默认恶意软件检测配置
func DefaultMalwareDetection() MalwareDetection {
	return MalwareDetection{
		Enabled:        true,
		ScanSignatures: true,
		ScanBehavior:   true,
		ScanPaths:      []string{"/home", "/tmp", "/var/tmp", "/opt"},
		ExcludePaths:   []string{"/proc", "/sys", "/dev"},
		MaxFileSize:    100 * 1024 * 1024, // 100MB
		QuarantinePath: "/var/quarantine",
	}
}

// MalwareThreat 恶意软件威胁
type MalwareThreat struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`     // "virus", "trojan", "ransomware", "spyware", "adware", "rootkit"
	Path        string    `json:"path"`
	Hash        string    `json:"hash"`     // 文件哈希
	Size        int64     `json:"size"`
	DetectedAt  time.Time `json:"detected_at"`
	Severity    string    `json:"severity"` // "critical", "high", "medium", "low"
	Action      string    `json:"action"`   // "quarantined", "deleted", "none"
	Description string    `json:"description"`
	Signature   string    `json:"signature"`
	Behavior    string    `json:"behavior,omitempty"` // 行为分析结果
}

// ============================================================
// 网络安全检查
// ============================================================

// NetworkSecurity 网络安全配置
type NetworkSecurity struct {
	Enabled            bool     `json:"enabled"`
	CheckOpenPorts     bool     `json:"check_open_ports"`
	CheckFirewall      bool     `json:"check_firewall"`
	CheckConnections   bool     `json:"check_connections"`
	CheckDNS           bool     `json:"check_dns"`
	AllowedPorts       []int    `json:"allowed_ports"`
	BlockedPorts       []int    `json:"blocked_ports"`
	TrustedNetworks    []string `json:"trusted_networks"`
	MonitorConnections bool     `json:"monitor_connections"`
}

// DefaultNetworkSecurity 默认网络安全配置
func DefaultNetworkSecurity() NetworkSecurity {
	return NetworkSecurity{
		Enabled:            true,
		CheckOpenPorts:     true,
		CheckFirewall:      true,
		CheckConnections:   true,
		CheckDNS:           true,
		AllowedPorts:       []int{22, 80, 443, 8080, 8443},
		BlockedPorts:       []int{23, 21, 25, 110, 143, 3389},
		TrustedNetworks:    []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		MonitorConnections: true,
	}
}

// OpenPort 开放端口信息
type OpenPort struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Service     string `json:"service"`
	State       string `json:"state"` // "open", "filtered", "closed"
	Risk        string `json:"risk"`  // "high", "medium", "low", "safe"
	Process     string `json:"process,omitempty"`
	PID         int    `json:"pid,omitempty"`
	BindAddress string `json:"bind_address"`
}

// FirewallRule 防火墙规则
type FirewallRule struct {
	ID          string `json:"id"`
	Action      string `json:"action"` // "allow", "deny", "reject", "log"
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	PortRange   string `json:"port_range,omitempty"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Direction   string `json:"direction"` // "inbound", "outbound", "both"
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// NetworkConnection 网络连接信息
type NetworkConnection struct {
	LocalAddr   string    `json:"local_addr"`
	LocalPort   int       `json:"local_port"`
	RemoteAddr  string    `json:"remote_addr"`
	RemotePort  int       `json:"remote_port"`
	Protocol    string    `json:"protocol"`
	State       string    `json:"state"`
	PID         int       `json:"pid"`
	Process     string    `json:"process"`
	EstablishedAt time.Time `json:"established_at"`
	Risk        string    `json:"risk"` // "suspicious", "normal", "blocked"
}

// ============================================================
// 账户安全检查
// ============================================================

// AccountSecurity 账户安全配置
type AccountSecurity struct {
	Enabled              bool `json:"enabled"`
	CheckWeakPasswords   bool `json:"check_weak_passwords"`
	CheckMFA             bool `json:"check_mfa"`
	CheckLoginAnomalies  bool `json:"check_login_anomalies"`
	CheckPrivileges      bool `json:"check_privileges"`
	PasswordPolicy       PasswordPolicy `json:"password_policy"`
	MaxFailedLogins      int  `json:"max_failed_logins"`
	LockoutDuration      int  `json:"lockout_duration"` // 分钟
	SessionTimeout       int  `json:"session_timeout"`  // 分钟
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireNumbers   bool `json:"require_numbers"`
	RequireSpecial   bool `json:"require_special"`
	MaxAge           int  `json:"max_age_days"`
	HistoryCount     int  `json:"history_count"` // 禁止重复最近N个密码
}

// DefaultAccountSecurity 默认账户安全配置
func DefaultAccountSecurity() AccountSecurity {
	return AccountSecurity{
		Enabled:             true,
		CheckWeakPasswords:  true,
		CheckMFA:            true,
		CheckLoginAnomalies: true,
		CheckPrivileges:     true,
		PasswordPolicy: PasswordPolicy{
			MinLength:        12,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxAge:           90,
			HistoryCount:     5,
		},
		MaxFailedLogins: 5,
		LockoutDuration: 30,
		SessionTimeout:  60,
	}
}

// UserAccount 用户账户信息
type UserAccount struct {
	Username       string    `json:"username"`
	UID            int       `json:"uid"`
	Group          string    `json:"group"`
	HomeDir        string    `json:"home_dir"`
	Shell          string    `json:"shell"`
	PasswordAge    int       `json:"password_age"` // 天数
	LastLogin      time.Time `json:"last_login"`
	FailedLogins   int       `json:"failed_logins"`
	MFAEnabled     bool      `json:"mfa_enabled"`
	Privileged     bool      `json:"privileged"`   // sudo 权限
	Locked         bool      `json:"locked"`
	PasswordStrength string  `json:"password_strength"` // "weak", "medium", "strong"
}

// LoginEvent 登录事件
type LoginEvent struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	SourceIP  string    `json:"source_ip"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Method    string    `json:"method"` // "password", "key", "mfa", "sso"
	Reason    string    `json:"reason,omitempty"` // 失败原因
	GeoLocation string  `json:"geo_location,omitempty"`
	Device    string    `json:"device,omitempty"`
}

// ============================================================
// 文件完整性监控
// ============================================================

// FileIntegrity 文件完整性监控配置
type FileIntegrity struct {
	Enabled         bool     `json:"enabled"`
	MonitoredPaths  []string `json:"monitored_paths"`
	CriticalFiles   []string `json:"critical_files"`
	HashAlgorithm   string   `json:"hash_algorithm"` // "sha256", "sha512", "md5"
	ScanInterval    int      `json:"scan_interval"`  // 秒
	AlertOnChange    bool     `json:"alert_on_change"`
	AutoBaseline     bool     `json:"auto_baseline"`
}

// DefaultFileIntegrity 默认文件完整性配置
func DefaultFileIntegrity() FileIntegrity {
	return FileIntegrity{
		Enabled:       true,
		MonitoredPaths: []string{"/etc", "/usr/bin", "/usr/sbin"},
		CriticalFiles: []string{
			"/etc/passwd",
			"/etc/shadow",
			"/etc/sudoers",
			"/etc/ssh/sshd_config",
			"/etc/hosts",
			"/etc/resolv.conf",
		},
		HashAlgorithm: "sha256",
		ScanInterval:  3600,
		AlertOnChange:  true,
		AutoBaseline:   true,
	}
}

// FileBaseline 文件基线信息
type FileBaseline struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	Permission   string    `json:"permission"`
	Owner        string    `json:"owner"`
	Group        string    `json:"group"`
	ModifiedAt   time.Time `json:"modified_at"`
	BaselineAt   time.Time `json:"baseline_at"`
	Algorithm    string    `json:"algorithm"`
}

// FileChange 文件变更信息
type FileChange struct {
	Path        string    `json:"path"`
	ChangeType  string    `json:"change_type"` // "modified", "deleted", "created", "permission_changed"
	OldHash     string    `json:"old_hash,omitempty"`
	NewHash     string    `json:"new_hash,omitempty"`
	OldPerm     string    `json:"old_perm,omitempty"`
	NewPerm     string    `json:"new_perm,omitempty"`
	OldOwner    string    `json:"old_owner,omitempty"`
	NewOwner    string    `json:"new_owner,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Severity    string    `json:"severity"` // "critical", "high", "medium", "low"
	Verified    bool      `json:"verified"`
}

// ============================================================
// 加固建议
// ============================================================

// HardeningRule 加固规则
type HardeningRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"` // "system", "network", "account", "service", "file", "crypto"
	Severity    string `json:"severity"` // "critical", "high", "medium", "low"
	Title       string `json:"title"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
	Remediation string `json:"remediation"`
	Commands    []string `json:"commands,omitempty"` // 修复命令
	Reference   string   `json:"reference,omitempty"`
	Automated   bool     `json:"automated"` // 是否可自动修复
	Enabled     bool     `json:"enabled"`
}

// HardeningSuggestion 加固建议
type HardeningSuggestion struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"` // "critical", "high", "medium", "low"
	Category    string    `json:"category"`
	Action      string    `json:"action"`
	Commands    []string  `json:"commands,omitempty"`
	Applied     bool      `json:"applied"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	Verified    bool      `json:"verified"`
	Impact      string    `json:"impact,omitempty"` // 预期影响
	Risk        string    `json:"risk,omitempty"`   // 实施风险
}

// ============================================================
// 合规检查
// ============================================================

// ComplianceStandard 合规标准
type ComplianceStandard struct {
	ID          string `json:"id"`
	Name        string `json:"name"` // "CIS", "GDPR", "等保2.0", "ISO27001", "PCI-DSS"
	Version     string `json:"version"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	ID          string `json:"id"`
	Standard    string `json:"standard"` // "CIS", "GDPR", "等保2.0"
	ControlID   string `json:"control_id"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Requirement string `json:"requirement"`
	Status      string `json:"status"` // "compliant", "non_compliant", "partial", "not_applicable"
	Evidence    string `json:"evidence,omitempty"`
	Gap         string `json:"gap,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID           string           `json:"id"`
	Standard     string           `json:"standard"`
	Version      string           `json:"version"`
	GeneratedAt  time.Time        `json:"generated_at"`
	Score        int              `json:"score"` // 0-100
	Status       string           `json:"status"` // "compliant", "non_compliant", "partial"
	TotalChecks  int              `json:"total_checks"`
	Passed       int              `json:"passed"`
	Failed       int              `json:"failed"`
	Partial      int              `json:"partial"`
	NotApplicable int             `json:"not_applicable"`
	Checks       []ComplianceCheck `json:"checks"`
	Summary      string           `json:"summary"`
}

// CISBenchmark CIS基准检查
type CISBenchmark struct {
	ID          string `json:"id"`
	Section     string `json:"section"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Level       int    `json:"level"` // 1=基本, 2=增强
	Status      string `json:"status"`
	Evidence    string `json:"evidence"`
}

// ============================================================
// 审计日志
// ============================================================

// AuditLog 审计日志
type AuditLog struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	EventType  string    `json:"event_type"` // "login", "logout", "config_change", "file_access", "admin_action", "security_event"
	Category   string    `json:"category"`   // "authentication", "authorization", "system", "data", "security"
	Username   string    `json:"username"`
	SourceIP   string    `json:"source_ip"`
	Resource   string    `json:"resource"`
	Action     string    `json:"action"`
	Result     string    `json:"result"` // "success", "failure", "denied"
	Details    string    `json:"details,omitempty"`
	Severity   string    `json:"severity"` // "critical", "high", "medium", "low", "info"
	UserAgent  string    `json:"user_agent,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
}

// AuditLogQuery 审计日志查询
type AuditLogQuery struct {
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	EventType  string    `json:"event_type,omitempty"`
	Category   string    `json:"category,omitempty"`
	Username   string    `json:"username,omitempty"`
	SourceIP   string    `json:"source_ip,omitempty"`
	Severity   string    `json:"severity,omitempty"`
	Result     string    `json:"result,omitempty"`
	Keyword    string    `json:"keyword,omitempty"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	SortBy     string    `json:"sort_by"`
	SortOrder  string    `json:"sort_order"`
}

// AuditLogResult 审计日志查询结果
type AuditLogResult struct {
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Logs     []AuditLog  `json:"logs"`
}

// ============================================================
// 威胁情报
// ============================================================

// ThreatIntel 威胁情报
type ThreatIntel struct {
	ID          string    `json:"id"`
	Indicator   string    `json:"indicator"` // IP, domain, hash, URL
	Type        string    `json:"type"`      // "ip", "domain", "hash", "url", "email"
	Severity    string    `json:"severity"`
	Confidence  float64   `json:"confidence"` // 0-100
	Source      string    `json:"source"`
	Description string    `json:"description"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Tags        []string  `json:"tags"`
	Reference   string    `json:"reference"`
}

// ThreatMatch 威胁匹配结果
type ThreatMatch struct {
	Indicator   string    `json:"indicator"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Confidence  float64   `json:"confidence"`
	Source      string    `json:"source"`
	Description string    `json:"description"`
	MatchedAt   time.Time `json:"matched_at"`
	Context     string    `json:"context"`
}

// ============================================================
// 安全报告
// ============================================================

// ThreatReport 威胁报告
type ThreatReport struct {
	ID              string               `json:"id"`
	GeneratedAt     time.Time            `json:"generated_at"`
	Score           SecurityScore        `json:"score"`
	Vulnerabilities []Vulnerability      `json:"vulnerabilities"`
	Malware         []MalwareThreat      `json:"malware"`
	NetworkIssues   []OpenPort           `json:"network_issues"`
	AccountIssues   []UserAccount        `json:"account_issues"`
	FileChanges     []FileChange         `json:"file_changes"`
	Hardening       []HardeningSuggestion `json:"hardening"`
	Compliance      []ComplianceReport   `json:"compliance"`
	AuditSummary    AuditSummary         `json:"audit_summary"`
	ThreatMatches   []ThreatMatch        `json:"threat_matches"`
	Recommendations []Recommendation     `json:"recommendations"`
	Summary         string               `json:"summary"`
	Duration        time.Duration        `json:"duration"`
}

// Recommendation 安全建议
type Recommendation struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // "critical", "high", "medium", "low"
	Category    string `json:"category"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // "low", "medium", "high"
}

// AuditSummary 审计摘要
type AuditSummary struct {
	TotalEvents   int            `json:"total_events"`
	ByEventType   map[string]int `json:"by_event_type"`
	BySeverity    map[string]int `json:"by_severity"`
	FailedLogins  int            `json:"failed_logins"`
	SuccessLogins int            `json:"success_logins"`
	ConfigChanges int            `json:"config_changes"`
	SecurityEvents int           `json:"security_events"`
	TopUsers      []UserActivity `json:"top_users"`
	TopIPs        []IPActivity   `json:"top_ips"`
	Period        string         `json:"period"`
}

// UserActivity 用户活动统计
type UserActivity struct {
	Username    string `json:"username"`
	EventCount  int    `json:"event_count"`
	FailedLogins int   `json:"failed_logins"`
	LastActive  time.Time `json:"last_active"`
}

// IPActivity IP活动统计
type IPActivity struct {
	IP          string `json:"ip"`
	EventCount  int    `json:"event_count"`
	FailedLogins int   `json:"failed_logins"`
	Country     string `json:"country,omitempty"`
	LastActive  time.Time `json:"last_active"`
}

// ============================================================
// 管理器配置
// ============================================================

// ManagerConfig 管理器配置
type ManagerConfig struct {
	VulnerabilityScan VulnerabilityScan `json:"vulnerability_scan"`
	MalwareDetection  MalwareDetection  `json:"malware_detection"`
	NetworkSecurity   NetworkSecurity   `json:"network_security"`
	AccountSecurity   AccountSecurity   `json:"account_security"`
	FileIntegrity     FileIntegrity     `json:"file_integrity"`
	ScoreWeight       ScoreWeight       `json:"score_weight"`
	AutoFix           bool              `json:"auto_fix"`           // 自动修复
	ReportInterval    int               `json:"report_interval"`    // 报告间隔（小时）
	AlertThreshold    int               `json:"alert_threshold"`    // 告警阈值分数
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		VulnerabilityScan: DefaultVulnerabilityScan(),
		MalwareDetection:  DefaultMalwareDetection(),
		NetworkSecurity:   DefaultNetworkSecurity(),
		AccountSecurity:   DefaultAccountSecurity(),
		FileIntegrity:     DefaultFileIntegrity(),
		ScoreWeight:       DefaultScoreWeight(),
		AutoFix:           false,
		ReportInterval:    24,
		AlertThreshold:    60,
	}
}

// ============================================================
// 扫描器配置与报告（scanner.go 依赖）
// ============================================================

// ScanConfig 扫描器配置
type ScanConfig struct {
	WeakPasswords   bool     `json:"weak_passwords"`
	OpenPorts       bool     `json:"open_ports"`
	FilePermissions bool     `json:"file_permissions"`
	SSLCertificates bool     `json:"ssl_certificates"`
	SystemUpdates   bool     `json:"system_updates"`
	MalwareScan     bool     `json:"malware_scan"`
	FirewallCheck   bool     `json:"firewall_check"`
	CriticalPaths   []string `json:"critical_paths,omitempty"`
	ExcludePaths    []string `json:"exclude_paths,omitempty"`
}

// DefaultScanConfig 默认扫描配置
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		WeakPasswords:   true,
		OpenPorts:       true,
		FilePermissions: true,
		SSLCertificates: true,
		SystemUpdates:   true,
		MalwareScan:     true,
		FirewallCheck:   true,
	}
}

// SecurityReport 安全扫描报告
type SecurityReport struct {
	ID              string               `json:"id"`
	ScanTime        time.Time            `json:"scan_time"`
	Duration        time.Duration        `json:"duration"`
	Checks          []SecurityCheck      `json:"checks"`
	OverallScore    int                  `json:"overall_score"`
	SecurityLevel   string               `json:"security_level"`
	TotalIssues     int                  `json:"total_issues"`
	CriticalIssues  int                  `json:"critical_issues"`
	WarningIssues   int                  `json:"warning_issues"`
	InfoIssues      int                  `json:"info_issues"`
	Recommendations []Recommendation     `json:"recommendations"`
	Summary         string               `json:"summary,omitempty"`
}

// PortScanResult 端口扫描结果
type PortScanResult struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	Risk     string `json:"risk,omitempty"`
	Service  string `json:"service,omitempty"`
}

// PortRiskConfig 端口风险配置
type PortRiskConfig struct {
	HighRiskPorts   []int `json:"high_risk_ports"`
	MediumRiskPorts []int `json:"medium_risk_ports"`
}

// DefaultPortRiskConfig 默认端口风险配置
func DefaultPortRiskConfig() PortRiskConfig {
	return PortRiskConfig{
		HighRiskPorts:   []int{21, 23, 25, 135, 139, 445, 1433, 3306, 3389, 5432, 5900, 6379},
		MediumRiskPorts: []int{110, 143, 993, 995, 1521, 5432, 8080, 8443, 9090},
	}
}

// CriticalFileConfig 关键文件配置
type CriticalFileConfig struct {
	Paths          []string `json:"paths"`
	MaxPermission  string   `json:"max_permission"`
}

// DefaultCriticalFileConfig 默认关键文件配置
func DefaultCriticalFileConfig() CriticalFileConfig {
	return CriticalFileConfig{
		Paths: []string{
			"/etc/passwd", "/etc/shadow", "/etc/sudoers",
			"/etc/ssh/sshd_config", "/etc/crontab",
		},
		MaxPermission: "0644",
	}
}

// SSLCheckConfig SSL 证书检查配置
type SSLCheckConfig struct {
	Domains      []string `json:"domains"`
	WarningDays  int      `json:"warning_days"`
	CriticalDays int      `json:"critical_days"`
}

// DefaultSSLCheckConfig 默认 SSL 检查配置
func DefaultSSLCheckConfig() SSLCheckConfig {
	return SSLCheckConfig{
		Domains:      []string{},
		WarningDays:  30,
		CriticalDays: 7,
	}
}

// PasswordPolicy 密码策略（scanner 用）

// DefaultPasswordPolicy 默认密码策略（兼容 scanner.go 调用）
func DefaultPasswordPolicy() PasswordPolicy {
	return DefaultAccountSecurity().PasswordPolicy
}
