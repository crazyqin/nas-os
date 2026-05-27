// Package securityadvisor provides security advisory and scanning functionality.
// 系统安全配置扫描、漏洞检测、安全评分 - 参考群晖 Security Advisor 设计
package securityadvisor

import (
	"time"
)

// ============================================================
// 安全扫描配置
// ============================================================

// ScanConfig 安全扫描配置
type ScanConfig struct {
	Enabled            bool `json:"enabled"`
	ScanWeakPasswords  bool `json:"scan_weak_passwords"`
	ScanOpenPorts      bool `json:"scan_open_ports"`
	ScanFilePermissions bool `json:"scan_file_permissions"`
	ScanSSLCertificates bool `json:"scan_ssl_certificates"`
	ScanSystemUpdates  bool `json:"scan_system_updates"`
	ScanMalware        bool `json:"scan_malware"`
	ScanFirewall       bool `json:"scan_firewall"`
}

// DefaultScanConfig 默认扫描配置
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		Enabled:             true,
		ScanWeakPasswords:   true,
		ScanOpenPorts:       true,
		ScanFilePermissions: true,
		ScanSSLCertificates: true,
		ScanSystemUpdates:   true,
		ScanMalware:         true,
		ScanFirewall:        true,
	}
}

// ============================================================
// 扫描结果类型
// ============================================================

// SecurityReport 安全扫描报告
type SecurityReport struct {
	ID              string           `json:"id"`
	ScanTime        time.Time        `json:"scan_time"`
	Duration        time.Duration    `json:"duration"`
	OverallScore    int              `json:"overall_score"`    // 0-100
	SecurityLevel   string           `json:"security_level"`   // "good", "warning", "critical"
	TotalIssues     int              `json:"total_issues"`
	CriticalIssues  int              `json:"critical_issues"`
	WarningIssues   int              `json:"warning_issues"`
	InfoIssues      int              `json:"info_issues"`
	Checks          []SecurityCheck  `json:"checks"`
	Recommendations []Recommendation `json:"recommendations"`
}

// SecurityCheck 安全检查项
type SecurityCheck struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Category    string        `json:"category"`     // "password", "port", "permission", "ssl", "update", "malware", "firewall"
	Status      string        `json:"status"`       // "pass", "warning", "critical", "info"
	Score       int           `json:"score"`        // 0-100
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
	CheckedAt   time.Time     `json:"checked_at"`
	Duration    time.Duration `json:"duration"`
}

// Recommendation 安全建议
type Recommendation struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`    // "high", "medium", "low"
	Category    string `json:"category"`
	Action      string `json:"action"`      // 具体操作步骤
}

// ============================================================
// 弱密码检测类型
// ============================================================

// WeakPasswordResult 弱密码检测结果
type WeakPasswordResult struct {
	Username    string `json:"username"`
	HasWeakPass bool   `json:"has_weak_pass"`
	Strength    string `json:"strength"` // "weak", "medium", "strong"
	Reason      string `json:"reason"`
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireNumbers   bool `json:"require_numbers"`
	RequireSpecial   bool `json:"require_special"`
	MaxAge           int  `json:"max_age_days"`
}

// DefaultPasswordPolicy 默认密码策略
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSpecial:   true,
		MaxAge:           90,
	}
}

// ============================================================
// 端口扫描类型
// ============================================================

// PortScanResult 端口扫描结果
type PortScanResult struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // "tcp", "udp"
	Service     string `json:"service"`
	State       string `json:"state"` // "open", "closed", "filtered"
	Risk        string `json:"risk"`  // "high", "medium", "low", "safe"
	Description string `json:"description"`
}

// PortRiskConfig 端口风险配置
type PortRiskConfig struct {
	HighRiskPorts   []int `json:"high_risk_ports"`
	MediumRiskPorts []int `json:"medium_risk_ports"`
	BlockedPorts    []int `json:"blocked_ports"`
}

// DefaultPortRiskConfig 默认端口风险配置
func DefaultPortRiskConfig() PortRiskConfig {
	return PortRiskConfig{
		HighRiskPorts:   []int{21, 23, 25, 110, 143, 3389},
		MediumRiskPorts: []int{80, 443, 8080, 8443},
		BlockedPorts:    []int{22, 3306, 5432, 6379},
	}
}

// ============================================================
// 文件权限审计类型
// ============================================================

// FilePermissionResult 文件权限审计结果
type FilePermissionResult struct {
	Path        string `json:"path"`
	Permission  string `json:"permission"` // e.g. "0755"
	Owner       string `json:"owner"`
	Group       string `json:"group"`
	Expected    string `json:"expected,omitempty"`
	Risk        string `json:"risk"` // "high", "medium", "low", "safe"
	Issue       string `json:"issue,omitempty"`
}

// CriticalFileConfig 关键文件配置
type CriticalFileConfig struct {
	Paths              []string `json:"paths"`
	MaxPermission      string   `json:"max_permission"` // e.g. "0644"
	RequiredOwner      string   `json:"required_owner"`
}

// DefaultCriticalFileConfig 默认关键文件配置
func DefaultCriticalFileConfig() CriticalFileConfig {
	return CriticalFileConfig{
		Paths: []string{
			"/etc/passwd",
			"/etc/shadow",
			"/etc/ssh/sshd_config",
			"/etc/sudoers",
		},
		MaxPermission: "0644",
		RequiredOwner: "root",
	}
}

// ============================================================
// SSL/TLS 证书检查类型
// ============================================================

// SSLCertificateResult SSL证书检查结果
type SSLCertificateResult struct {
	Domain          string    `json:"domain"`
	Issuer          string    `json:"issuer"`
	Subject         string    `json:"subject"`
	NotBefore       time.Time `json:"not_before"`
	NotAfter        time.Time `json:"not_after"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
	IsValid         bool      `json:"is_valid"`
	Protocol        string    `json:"protocol"`
	CipherSuite     string    `json:"cipher_suite"`
	KeySize         int       `json:"key_size"`
	Risk            string    `json:"risk"` // "expired", "expiring_soon", "valid", "weak"
	Issue           string    `json:"issue,omitempty"`
}

// SSLCheckConfig SSL检查配置
type SSLCheckConfig struct {
	WarningDays  int      `json:"warning_days"`  // 证书过期前多少天警告
	CriticalDays int      `json:"critical_days"` // 证书过期前多少天严重警告
	Domains      []string `json:"domains"`       // 要检查的域名
}

// DefaultSSLCheckConfig 默认SSL检查配置
func DefaultSSLCheckConfig() SSLCheckConfig {
	return SSLCheckConfig{
		WarningDays:  30,
		CriticalDays: 7,
		Domains:      []string{},
	}
}

// ============================================================
// 系统更新检查类型
// ============================================================

// SystemUpdateResult 系统更新检查结果
type SystemUpdateResult struct {
	PackageName    string    `json:"package_name"`
	CurrentVersion string    `json:"current_version"`
	AvailableVersion string  `json:"available_version"`
	UpdateType     string    `json:"update_type"` // "security", "bugfix", "feature"
	Severity       string    `json:"severity"`    // "critical", "important", "moderate", "low"
	ReleaseDate    time.Time `json:"release_date"`
	Description    string    `json:"description"`
}

// UpdateCheckConfig 更新检查配置
type UpdateCheckConfig struct {
	Enabled         bool     `json:"enabled"`
	CheckSecurity   bool     `json:"check_security"`
	CheckBugfix     bool     `json:"check_bugfix"`
	AutoUpdate      bool     `json:"auto_update"`
	ExcludePackages []string `json:"exclude_packages"`
}

// DefaultUpdateCheckConfig 默认更新检查配置
func DefaultUpdateCheckConfig() UpdateCheckConfig {
	return UpdateCheckConfig{
		Enabled:         true,
		CheckSecurity:   true,
		CheckBugfix:     true,
		AutoUpdate:      false,
		ExcludePackages: []string{},
	}
}

// ============================================================
// 恶意软件扫描类型
// ============================================================

// MalwareScanResult 恶意软件扫描结果
type MalwareScanResult struct {
	Path        string    `json:"path"`
	ThreatName  string    `json:"threat_name"`
	ThreatType  string    `json:"threat_type"` // "virus", "trojan", "malware", "suspicious"
	Risk        string    `json:"risk"`        // "critical", "high", "medium", "low"
	Action      string    `json:"action"`      // "quarantined", "deleted", "none"
	ScannedAt   time.Time `json:"scanned_at"`
	Details     string    `json:"details,omitempty"`
}

// MalwareScanConfig 恶意软件扫描配置
type MalwareScanConfig struct {
	Enabled         bool     `json:"enabled"`
	ScanPaths       []string `json:"scan_paths"`
	ExcludePaths    []string `json:"exclude_paths"`
	MaxFileSize     int64    `json:"max_file_size"` // bytes
	QuickScan       bool     `json:"quick_scan"`
}

// DefaultMalwareScanConfig 默认恶意软件扫描配置
func DefaultMalwareScanConfig() MalwareScanConfig {
	return MalwareScanConfig{
		Enabled: true,
		ScanPaths: []string{
			"/home",
			"/var/www",
			"/tmp",
		},
		ExcludePaths: []string{
			"/proc",
			"/sys",
			"/dev",
		},
		MaxFileSize: 100 * 1024 * 1024, // 100MB
		QuickScan:   true,
	}
}

// ============================================================
// 防火墙检查类型
// ============================================================

// FirewallStatus 防火墙状态
type FirewallStatus struct {
	Enabled        bool              `json:"enabled"`
	DefaultPolicy  string            `json:"default_policy"` // "accept", "drop", "reject"
	RuleCount      int               `json:"rule_count"`
	OpenPorts      []int             `json:"open_ports"`
	BlockedPorts   []int             `json:"blocked_ports"`
	Rules          []FirewallRule    `json:"rules"`
	Risk           string            `json:"risk"` // "good", "warning", "critical"
	Issue          string            `json:"issue,omitempty"`
}

// FirewallRule 防火墙规则
type FirewallRule struct {
	ID          string `json:"id"`
	Action      string `json:"action"` // "allow", "deny", "reject"
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Description string `json:"description"`
}

// ============================================================
// 综合评分类型
// ============================================================

// SecurityScore 安全评分
type SecurityScore struct {
	Overall     int              `json:"overall"`      // 0-100
	Password    int              `json:"password"`     // 0-100
	Port        int              `json:"port"`         // 0-100
	Permission  int              `json:"permission"`   // 0-100
	SSL         int              `json:"ssl"`          // 0-100
	Update      int              `json:"update"`       // 0-100
	Malware     int              `json:"malware"`      // 0-100
	Firewall    int              `json:"firewall"`     // 0-100
	Level       string           `json:"level"`        // "good", "warning", "critical"
	Summary     string           `json:"summary"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// ScoreWeight 评分权重
type ScoreWeight struct {
	Password   float64 `json:"password"`
	Port       float64 `json:"port"`
	Permission float64 `json:"permission"`
	SSL        float64 `json:"ssl"`
	Update     float64 `json:"update"`
	Malware    float64 `json:"malware"`
	Firewall   float64 `json:"firewall"`
}

// DefaultScoreWeight 默认评分权重
func DefaultScoreWeight() ScoreWeight {
	return ScoreWeight{
		Password:   0.20,
		Port:       0.15,
		Permission: 0.15,
		SSL:        0.15,
		Update:     0.15,
		Malware:    0.10,
		Firewall:   0.10,
	}
}
