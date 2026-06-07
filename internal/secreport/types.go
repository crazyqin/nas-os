// Package secreport 提供安全审计报告功能
package secreport

import (
	"time"
)

// Severity 漏洞严重程度.
type Severity string

// 严重程度常量.
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// SeverityWeight 严重程度权重.
var SeverityWeight = map[Severity]float64{
	SeverityCritical: 10.0,
	SeverityHigh:     5.0,
	SeverityMedium:   2.0,
	SeverityLow:      1.0,
	SeverityInfo:     0.0,
}

// Status 检查状态.
type Status string

// 状态常量.
const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
)

// ReportFormat 报告格式.
type ReportFormat string

// 报告格式常量.
const (
	FormatJSON ReportFormat = "json"
	FormatHTML ReportFormat = "html"
	FormatCSV  ReportFormat = "csv"
)

// Category 安全检查分类.
type Category string

// 分类常量.
const (
	CategoryAuth       Category = "authentication"
	CategoryNetwork    Category = "network"
	CategoryStorage    Category = "storage"
	CategoryAccess     Category = "access_control"
	CategoryLogging    Category = "logging"
	CategoryEncryption Category = "encryption"
	CategorySystem     Category = "system"
	CategoryCIS        Category = "cis_benchmark"
)

// Finding 安全发现/漏洞.
type Finding struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"rule_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    Severity          `json:"severity"`
	Category    Category          `json:"category"`
	Status      Status            `json:"status"`
	Remediation string            `json:"remediation"`
	Reference   string            `json:"reference,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// SecurityReport 安全审计报告.
type SecurityReport struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	Score           int               `json:"score"`
	Grade           string            `json:"grade"`
	TotalChecks     int               `json:"total_checks"`
	PassedChecks    int               `json:"passed_checks"`
	FailedChecks    int               `json:"failed_checks"`
	WarningChecks   int               `json:"warning_checks"`
	Findings        []Finding         `json:"findings"`
	Categories      []CategorySummary `json:"categories"`
	Summary         string            `json:"summary"`
	Recommendations []string          `json:"recommendations"`
	Duration        time.Duration     `json:"duration"`
}

// CategorySummary 分类摘要.
type CategorySummary struct {
	Category    Category `json:"category"`
	Score       int      `json:"score"`
	Total       int      `json:"total"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Warnings    int      `json:"warnings"`
	Description string   `json:"description"`
}

// Auditor 审计器配置.
type Auditor struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Version     string     `json:"version"`
	Enabled     bool       `json:"enabled"`
	Categories  []Category `json:"categories"`
}

// AuditSchedule 审计调度配置.
type AuditSchedule struct {
	Enabled    bool          `json:"enabled"`
	Interval   time.Duration `json:"interval"`
	LastRun    time.Time     `json:"last_run"`
	NextRun    time.Time     `json:"next_run"`
	Categories []Category    `json:"categories"`
}

// CISControl CIS 控制项.
type CISControl struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Level       int      `json:"level"` // 1 或 2
	Category    Category `json:"category"`
	Status      Status   `json:"status"`
	Remediation string   `json:"remediation"`
}

// SystemConfig 系统安全配置.
type SystemConfig struct {
	SSHConfig      SSHConfig      `json:"ssh_config"`
	FirewallConfig FirewallConfig `json:"firewall_config"`
	AuthConfig     AuthConfig     `json:"auth_config"`
	NetworkConfig  NetworkConfig  `json:"network_config"`
	StorageConfig  StorageConfig  `json:"storage_config"`
	LoggingConfig  LoggingConfig  `json:"logging_config"`
}

// SSHConfig SSH 配置.
type SSHConfig struct {
	RootLogin            bool `json:"root_login"`
	PasswordAuth         bool `json:"password_auth"`
	Port                 int  `json:"port"`
	MaxAuthTries         int  `json:"max_auth_tries"`
	Protocol             int  `json:"protocol"`
	PermitEmptyPasswords bool `json:"permit_empty_passwords"`
	X11Forwarding        bool `json:"x11_forwarding"`
}

// FirewallConfig 防火墙配置.
type FirewallConfig struct {
	Enabled       bool     `json:"enabled"`
	DefaultPolicy string   `json:"default_policy"`
	OpenPorts     []int    `json:"open_ports"`
	AllowedIPs    []string `json:"allowed_ips"`
}

// AuthConfig 认证配置.
type AuthConfig struct {
	MFAEnabled        bool `json:"mfa_enabled"`
	PasswordMinLength int  `json:"password_min_length"`
	PasswordExpiry    int  `json:"password_expiry_days"`
	AccountLockout    bool `json:"account_lockout"`
	LockoutThreshold  int  `json:"lockout_threshold"`
	LockoutDuration   int  `json:"lockout_duration_minutes"`
	SessionTimeout    int  `json:"session_timeout_minutes"`
}

// NetworkConfig 网络配置.
type NetworkConfig struct {
	IPv6Enabled   bool     `json:"ipv6_enabled"`
	IPForwarding  bool     `json:"ip_forwarding"`
	TCPSynCookies bool     `json:"tcp_syn_cookies"`
	ListenPorts   []int    `json:"listen_ports"`
	DNSServers    []string `json:"dns_servers"`
}

// StorageConfig 存储配置.
type StorageConfig struct {
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionAlgo    string `json:"encryption_algo"`
	BackupEnabled     bool   `json:"backup_enabled"`
	BackupEncrypted   bool   `json:"backup_encrypted"`
	RAIDLevel         string `json:"raid_level"`
}

// LoggingConfig 日志配置.
type LoggingConfig struct {
	AuditEnabled  bool `json:"audit_enabled"`
	LogRetention  int  `json:"log_retention_days"`
	RemoteLogging bool `json:"remote_logging"`
	LogRotation   bool `json:"log_rotation"`
	AccessLogging bool `json:"access_logging"`
}

// AuditHistory 审计历史记录.
type AuditHistory struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	ReportID  string        `json:"report_id"`
	Score     int           `json:"score"`
	Grade     string        `json:"grade"`
	Findings  int           `json:"findings_count"`
	Duration  time.Duration `json:"duration"`
}

// ReportListResponse 报告列表响应.
type ReportListResponse struct {
	Total   int              `json:"total"`
	Reports []SecurityReport `json:"reports"`
}

// HistoryResponse 历史记录响应.
type HistoryResponse struct {
	Total   int            `json:"total"`
	History []AuditHistory `json:"history"`
}
