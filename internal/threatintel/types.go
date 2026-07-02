// Package threatintel 实现威胁情报中心功能，提供多源威胁情报聚合、
// 自动漏洞扫描、IOC 匹配与阻断、威胁评分和情报共享能力。
// 参考群晖 Security Advisor 和 NIDS 设计，整合漏洞扫描与威胁检测。
package threatintel

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ============================================================
// 威胁级别常量
// ============================================================

// Severity 威胁严重级别.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// ============================================================
// IOC 类型常量
// ============================================================

// IOCType IOC 指标类型.
type IOCType string

const (
	IOCTypeIP       IOCType = "ip"
	IOCTypeDomain   IOCType = "domain"
	IOCTypeURL      IOCType = "url"
	IOCTypeFileHash IOCType = "file_hash"
	IOCTypeEmail    IOCType = "email"
	IOCTypeCIDR     IOCType = "cidr"
)

// ============================================================
// 情报源类型
// ============================================================

// FeedType 情报源类型.
type FeedType string

const (
	FeedTypeOpenSource FeedType = "open_source"
	FeedTypeCommercial FeedType = "commercial"
	FeedTypeGovernment FeedType = "government"
	FeedTypeCommunity  FeedType = "community"
	FeedTypeInternal   FeedType = "internal"
)

// FeedStatus 情报源状态.
type FeedStatus string

const (
	FeedStatusActive   FeedStatus = "active"
	FeedStatusInactive FeedStatus = "inactive"
	FeedStatusError    FeedStatus = "error"
)

// ============================================================
// 扫描状态
// ============================================================

// ScanStatus 扫描状态.
type ScanStatus string

const (
	ScanStatusPending  ScanStatus = "pending"
	ScanStatusRunning  ScanStatus = "running"
	ScanStatusComplete ScanStatus = "complete"
	ScanStatusFailed   ScanStatus = "failed"
)

// ============================================================
// 告警类型
// ============================================================

// AlertStatus 告警状态.
type AlertStatus string

const (
	AlertStatusOpen          AlertStatus = "open"
	AlertStatusAcknowledged  AlertStatus = "acknowledged"
	AlertStatusResolved      AlertStatus = "resolved"
	AlertStatusFalsePositive AlertStatus = "false_positive"
)

// ============================================================
// 数据结构定义
// ============================================================

// ThreatFeed 威胁情报源.
type ThreatFeed struct {
	// ID 情报源唯一标识符
	ID string `json:"id"`
	// Name 情报源名称
	Name string `json:"name"`
	// Description 情报源描述
	Description string `json:"description"`
	// Type 情报源类型
	Type FeedType `json:"type"`
	// URL 情报源 URL
	URL string `json:"url"`
	// Status 情报源状态
	Status FeedStatus `json:"status"`
	// TrustLevel 信任级别（0-100）
	TrustLevel int `json:"trust_level"`
	// UpdateInterval 自动更新间隔
	UpdateInterval time.Duration `json:"update_interval"`
	// LastUpdate 最后更新时间
	LastUpdate time.Time `json:"last_update"`
	// IOCCount 该源的 IOC 数量
	IOCCount int `json:"ioc_count"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// IOC 威胁指标 (Indicator of Compromise).
type IOC struct {
	// ID IOC 唯一标识符
	ID string `json:"id"`
	// Type IOC 类型
	Type IOCType `json:"type"`
	// Value IOC 值（IP、域名、哈希等）
	Value string `json:"value"`
	// Severity 威胁级别
	Severity Severity `json:"severity"`
	// Confidence 置信度（0-100）
	Confidence int `json:"confidence"`
	// ThreatScore 威胁评分（0-100）
	ThreatScore int `json:"threat_score"`
	// Description 描述
	Description string `json:"description"`
	// SourceID 来源情报源 ID
	SourceID string `json:"source_id"`
	// Tags 标签
	Tags []string `json:"tags,omitempty"`
	// FirstSeen 首次发现时间
	FirstSeen time.Time `json:"first_seen"`
	// LastSeen 最后发现时间
	LastSeen time.Time `json:"last_seen"`
	// ExpiresAt 过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Blocked 是否已阻断
	Blocked bool `json:"blocked"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// Alert 威胁告警.
type Alert struct {
	// ID 告警唯一标识符
	ID string `json:"id"`
	// Title 告警标题
	Title string `json:"title"`
	// Description 告警描述
	Description string `json:"description"`
	// Severity 威胁级别
	Severity Severity `json:"severity"`
	// Status 告警状态
	Status AlertStatus `json:"status"`
	// SourceIP 源 IP
	SourceIP net.IP `json:"source_ip,omitempty"`
	// DestIP 目标 IP
	DestIP net.IP `json:"dest_ip,omitempty"`
	// RelatedIOCs 关联的 IOC 列表
	RelatedIOCs []string `json:"related_iocs,omitempty"`
	// Action 建议动作
	Action string `json:"action"`
	// Score 威胁评分
	Score int `json:"score"`
	// FirstSeen 首次发现时间
	FirstSeen time.Time `json:"first_seen"`
	// LastSeen 最后发现时间
	LastSeen time.Time `json:"last_seen"`
	// AckedAt 确认时间
	AckedAt *time.Time `json:"acked_at,omitempty"`
	// ResolvedAt 解决时间
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	// ID 扫描唯一标识符
	ID string `json:"id"`
	// ScanType 扫描类型
	ScanType string `json:"scan_type"` // "port", "service", "vuln", "full"
	// Status 扫描状态
	Status ScanStatus `json:"status"`
	// Target 扫描目标
	Target string `json:"target"`
	// StartTime 开始时间
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间
	EndTime time.Time `json:"end_time"`
	// Duration 扫描耗时
	Duration time.Duration `json:"duration"`
	// TotalPorts 扫描端口总数
	TotalPorts int `json:"total_ports"`
	// OpenPorts 发现的开放端口数
	OpenPorts int `json:"open_ports"`
	// Vulnerabilities 发现的漏洞列表
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	// Services 发现的服务列表
	Services []ServiceInfo `json:"services"`
	// RiskScore 风险评分（0-100）
	RiskScore int `json:"risk_score"`
	// Summary 扫描摘要
	Summary string `json:"summary"`
}

// Vulnerability 漏洞信息.
type Vulnerability struct {
	// ID 漏洞唯一标识符
	ID string `json:"id"`
	// CVE CVE 编号
	CVE string `json:"cve"`
	// Title 漏洞标题
	Title string `json:"title"`
	// Description 漏洞描述
	Description string `json:"description"`
	// Severity 严重级别
	Severity Severity `json:"severity"`
	// CVSS CVSS 评分
	CVSS float64 `json:"cvss"`
	// AffectedService 受影响服务
	AffectedService string `json:"affected_service"`
	// AffectedPort 受影响端口
	AffectedPort int `json:"affected_port"`
	// Solution 修复建议
	Solution string `json:"solution"`
	// References 参考链接
	References []string `json:"references,omitempty"`
	// PublishedAt 发布时间
	PublishedAt time.Time `json:"published_at"`
}

// ServiceInfo 服务信息.
type ServiceInfo struct {
	// Port 端口号
	Port int `json:"port"`
	// Protocol 协议
	Protocol string `json:"protocol"`
	// ServiceName 服务名称
	ServiceName string `json:"service_name"`
	// Version 服务版本
	Version string `json:"version"`
	// Banner 服务 Banner
	Banner string `json:"banner"`
	// State 端口状态
	State string `json:"state"`
}

// ThreatScore 威胁评分详情.
type ThreatScore struct {
	// Overall 总体评分（0-100）
	Overall int `json:"overall"`
	// NetworkScore 网络安全评分
	NetworkScore int `json:"network_score"`
	// VulnScore 漏洞评分
	VulnScore int `json:"vuln_score"`
	// IOCScore IOC 威胁评分
	IOCScore int `json:"ioc_score"`
	// ReputationScore 信誉评分
	ReputationScore int `json:"reputation_score"`
	// Level 威胁级别
	Level string `json:"level"` // "safe", "low", "medium", "high", "critical"
	// Summary 评分摘要
	Summary string `json:"summary"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================
// 配置与统计
// ============================================================

// ThreatIntelConfig 威胁情报中心配置.
type ThreatIntelConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// AutoScan 是否启用自动扫描
	AutoScan bool `json:"auto_scan"`
	// ScanInterval 自动扫描间隔
	ScanInterval time.Duration `json:"scan_interval"`
	// AutoBlock 是否自动阻断高威胁 IOC
	AutoBlock bool `json:"auto_block"`
	// BlockThreshold 自动阻断阈值（威胁评分）
	BlockThreshold int `json:"block_threshold"`
	// MaxIOCs 最大 IOC 数量
	MaxIOCs int `json:"max_max_iocs"`
	// IOCExpiryDays IOC 默认过期天数
	IOCExpiryDays int `json:"ioc_expiry_days"`
	// MaxAlerts 最大告警数量
	MaxAlerts int `json:"max_alerts"`
	// FeedUpdateInterval 情报源默认更新间隔
	FeedUpdateInterval time.Duration `json:"feed_update_interval"`
}

// ThreatIntelStats 威胁情报统计.
type ThreatIntelStats struct {
	// TotalFeeds 情报源总数
	TotalFeeds int `json:"total_feeds"`
	// ActiveFeeds 活跃情报源数
	ActiveFeeds int `json:"active_feeds"`
	// TotalIOCs IOC 总数
	TotalIOCs int `json:"total_iocs"`
	// BlockedIOCs 已阻断 IOC 数
	BlockedIOCs int `json:"blocked_iocs"`
	// TotalAlerts 告警总数
	TotalAlerts int `json:"total_alerts"`
	// OpenAlerts 未处理告警数
	OpenAlerts int `json:"open_alerts"`
	// ScansPerformed 执行扫描次数
	ScansPerformed int `json:"scans_performed"`
	// LastScan 最后扫描时间
	LastScan time.Time `json:"last_scan"`
	// ThreatScore 当前威胁评分
	ThreatScore *ThreatScore `json:"threat_score,omitempty"`
}

// ============================================================
// 错误类型
// ============================================================

// ThreatIntelError 威胁情报错误.
type ThreatIntelError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error 实现 error 接口.
func (e *ThreatIntelError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回内部错误.
func (e *ThreatIntelError) Unwrap() error {
	return e.Err
}

// 预定义错误.
var (
	ErrFeedNotFound     = &ThreatIntelError{Code: "FEED_NOT_FOUND", Message: "情报源不存在"}
	ErrFeedExists       = &ThreatIntelError{Code: "FEED_EXISTS", Message: "情报源已存在"}
	ErrIOCNotFound      = &ThreatIntelError{Code: "IOC_NOT_FOUND", Message: "IOC 不存在"}
	ErrAlertNotFound    = &ThreatIntelError{Code: "ALERT_NOT_FOUND", Message: "告警不存在"}
	ErrScanInProgress   = &ThreatIntelError{Code: "SCAN_IN_PROGRESS", Message: "扫描正在进行中"}
	ErrInvalidTarget    = &ThreatIntelError{Code: "INVALID_TARGET", Message: "无效的扫描目标"}
	ErrFeedUpdateFailed = &ThreatIntelError{Code: "FEED_UPDATE_FAILED", Message: "情报源更新失败"}
)

// NewThreatIntelError 创建包含内部错误的 ThreatIntelError.
func NewThreatIntelError(code, message string, err error) *ThreatIntelError {
	return &ThreatIntelError{Code: code, Message: message, Err: err}
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *ThreatIntelConfig {
	return &ThreatIntelConfig{
		Enabled:            true,
		AutoScan:           true,
		ScanInterval:       24 * time.Hour,
		AutoBlock:          false,
		BlockThreshold:     80,
		MaxIOCs:            100000,
		IOCExpiryDays:      90,
		MaxAlerts:          10000,
		FeedUpdateInterval: 1 * time.Hour,
	}
}

// ============================================================
// 常见漏洞数据库（示例）
// ============================================================

// KnownVulnerability 已知漏洞模板.
type KnownVulnerability struct {
	CVE         string
	Title       string
	Severity    Severity
	CVSS        float64
	Service     string
	Version     string
	Description string
	Solution    string
}

// CommonVulns 常见漏洞示例数据.
var CommonVulns = []KnownVulnerability{
	{
		CVE: "CVE-2021-44228", Title: "Log4Shell", Severity: SeverityCritical, CVSS: 10.0,
		Service: "log4j", Description: "Apache Log4j2 远程代码执行漏洞",
		Solution: "升级 Log4j2 到 2.17.0 或更高版本",
	},
	{
		CVE: "CVE-2014-0160", Title: "Heartbleed", Severity: SeverityCritical, CVSS: 7.5,
		Service: "openssl", Description: "OpenSSL Heartbleed 内存泄露漏洞",
		Solution: "升级 OpenSSL 到 1.0.1g 或更高版本",
	},
	{
		CVE: "CVE-2017-0144", Title: "EternalBlue", Severity: SeverityCritical, CVSS: 8.1,
		Service: "smb", Description: "Windows SMB 远程代码执行漏洞",
		Solution: "安装 MS17-010 安全更新",
	},
}

// ScanManager 返回互斥锁管理器.
type ScanManager struct {
	mu       sync.Mutex
	isScan   bool
	scanChan chan struct{}
}

// NewScanManager 创建扫描管理器.
func NewScanManager() *ScanManager {
	return &ScanManager{
		scanChan: make(chan struct{}, 1),
	}
}

// TryStartScan 尝试开始扫描，返回是否成功.
func (sm *ScanManager) TryStartScan() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.isScan {
		return false
	}
	sm.isScan = true
	return true
}

// FinishScan 完成扫描.
func (sm *ScanManager) FinishScan() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isScan = false
}

// IsScanning 是否正在扫描.
func (sm *ScanManager) IsScanning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.isScan
}
