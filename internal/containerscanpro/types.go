package containerscanpro

import (
	"sync"
	"time"
)

// Severity 漏洞严重程度
type Severity = string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// ScanStatus 扫描状态
type ScanStatus = string

const (
	StatusPending   ScanStatus = "pending"
	StatusScanning  ScanStatus = "scanning"
	StatusRunning   ScanStatus = "running"
	StatusCompleted ScanStatus = "completed"
	StatusFailed    ScanStatus = "failed"
	StatusCancelled ScanStatus = "cancelled"
)

// RuntimeAnomalyType 运行时异常类型
type RuntimeAnomalyType = string

const (
	AnomalySuspiciousProcess   RuntimeAnomalyType = "suspicious_process"
	AnomalyFileModification    RuntimeAnomalyType = "file_modification"
	AnomalyNetworkConnection   RuntimeAnomalyType = "network_connection"
	AnomalyPrivilegeEscalation RuntimeAnomalyType = "privilege_escalation"
	AnomalyResourceAbuse       RuntimeAnomalyType = "resource_abuse"
)

// AnomalyType 运行时异常检测类型
type AnomalyType = string

const (
	AnomalyAbnormalProcess  AnomalyType = "abnormal_process"
	AnomalyAbnormalNetwork  AnomalyType = "abnormal_network"
	AnomalyPrivilegedContainer AnomalyType = "privileged_container"
)

// AlertLevel 告警级别
type AlertLevel = string

const (
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelInfo     AlertLevel = "info"
)

// ComplianceStandard 合规标准
type ComplianceStandard = string

const (
	StandardCIS    ComplianceStandard = "CIS"
	StandardNIST   ComplianceStandard = "NIST"
	StandardOWASP  ComplianceStandard = "OWASP"
	StandardCustom ComplianceStandard = "CUSTOM"
)

// FixAction 修复动作类型
type FixAction = string

const (
	FixActionUpgrade FixAction = "upgrade"
	FixActionPatch   FixAction = "patch"
	FixActionIgnore  FixAction = "ignore"
	FixActionRemove  FixAction = "remove"
)

// CVEInfo CVE 漏洞信息
type CVEInfo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
	Score       float64   `json:"score"`
	PublishedAt time.Time `json:"published_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	References  []string  `json:"references,omitempty"`
	FixVersions []string  `json:"fix_versions,omitempty"`
}

// VulnerabilityCVE CVE 漏洞详细信息
type VulnerabilityCVE struct {
	CVEID       string    `json:"cve_id"`
	Severity    Severity  `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixVersion  string    `json:"fix_version,omitempty"`
	CVSS        float64   `json:"cvss"`
	PublishedAt time.Time `json:"published_at"`
	ContainerID string    `json:"container_id,omitempty"`
	ImageName   string    `json:"image_name,omitempty"`
}

// PackageInfo 受影响的软件包信息
type PackageInfo struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installed_version"`
	FixedVersion     string `json:"fixed_version,omitempty"`
	Source           string `json:"source,omitempty"`
}

// ContainerVulnerability 容器漏洞
type ContainerVulnerability struct {
	CVE         CVEInfo     `json:"cve"`
	Package     PackageInfo `json:"package"`
	ContainerID string      `json:"container_id"`
	ImageName   string      `json:"image_name"`
	Layer       string      `json:"layer,omitempty"`
}

// RuntimeAnomaly 运行时异常
type RuntimeAnomaly struct {
	Type        RuntimeAnomalyType `json:"type"`
	ContainerID string             `json:"container_id"`
	Timestamp   time.Time          `json:"timestamp"`
	Description string             `json:"description"`
	Details     map[string]string  `json:"details,omitempty"`
	Severity    Severity           `json:"severity"`
}

// Anomaly 运行时异常检测结果
type Anomaly struct {
	Type       AnomalyType `json:"type"`
	Message    string      `json:"message"`
	Severity   Severity    `json:"severity"`
	DetectedAt time.Time   `json:"detected_at"`
	Details    string      `json:"details,omitempty"`
}

// SecurityScore 安全评分
type SecurityScore struct {
	Overall      float64            `json:"overall"`
	VulnScore    float64            `json:"vuln_score"`
	RuntimeScore float64            `json:"runtime_score"`
	ConfigScore  float64            `json:"config_score"`
	Breakdown    map[string]float64 `json:"breakdown"`
	Grade        string             `json:"grade"`
}

// VulnSummary 漏洞统计摘要
type VulnSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

// ComplianceResult 合规检查结果
type ComplianceResult struct {
	RuleID   string   `json:"rule_id"`
	RuleName string   `json:"rule_name"`
	Passed   bool     `json:"passed"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

// AutoFixAction 自动修复动作
type AutoFixAction struct {
	VulnID      string    `json:"vuln_id"`
	Package     string    `json:"package"`
	CurrentVer  string    `json:"current_ver"`
	FixVer      string    `json:"fix_ver"`
	Action      FixAction `json:"action"`
	Command     string    `json:"command,omitempty"`
	Description string    `json:"description"`
	Applied     bool      `json:"applied"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID                string              `json:"id"`
	ScanID            string              `json:"scan_id,omitempty"`
	ContainerID       string              `json:"container_id,omitempty"`
	ContainerName     string              `json:"container_name,omitempty"`
	ImageName         string              `json:"image_name"`
	ImageID           string              `json:"image_id"`
	Status            ScanStatus          `json:"status"`
	ScanTime          time.Time           `json:"scan_time"`
	StartTime         time.Time           `json:"start_time,omitempty"`
	EndTime           *time.Time          `json:"end_time,omitempty"`
	Duration          time.Duration       `json:"duration,omitempty"`
	PolicyID          string              `json:"policy_id,omitempty"`
	Vulnerabilities   []VulnerabilityCVE  `json:"vulnerabilities,omitempty"`
	VulnSummary       VulnSummary         `json:"vuln_summary"`
	Anomalies         []RuntimeAnomaly    `json:"anomalies,omitempty"`
	Score             *SecurityScore      `json:"score,omitempty"`
	Recommendations   []string            `json:"recommendations,omitempty"`
	ComplianceStatus  []ComplianceResult  `json:"compliance_status,omitempty"`
	Compliant         bool                `json:"compliant"`
	FixSuggestions    []AutoFixAction     `json:"fix_suggestions,omitempty"`
	Error             string              `json:"error,omitempty"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	EnableCVEScan        bool          `json:"enable_cve_scan"`
	EnableRuntimeMonitor bool          `json:"enable_runtime_monitor"`
	ScanInterval         time.Duration `json:"scan_interval"`
	AlertThreshold       Severity      `json:"alert_threshold"`
	MaxConcurrent        int           `json:"max_concurrent"`
	Timeout              time.Duration `json:"timeout"`
	ExcludedContainers   []string      `json:"excluded_containers,omitempty"`
	ExcludedImages       []string      `json:"excluded_images,omitempty"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled     bool       `json:"enabled"`
	WebhookURL  string     `json:"webhook_url,omitempty"`
	EmailTo     []string   `json:"email_to,omitempty"`
	MinLevel    AlertLevel `json:"min_level"`
	CooldownSec int        `json:"cooldown_sec"`
}

// Alert 告警消息
type Alert struct {
	ID        string            `json:"id"`
	Level     AlertLevel        `json:"level"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Source    string            `json:"source"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

// ScanPolicy 扫描策略
type ScanPolicy struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	SeverityThreshold   Severity `json:"severity_threshold"`
	AutoFixEnabled      bool     `json:"auto_fix_enabled"`
	ComplianceStandards []string `json:"compliance_standards,omitempty"`
	ExcludePackages     []string `json:"exclude_packages,omitempty"`
	ScheduleCron        string   `json:"schedule_cron,omitempty"`
	Enabled             bool     `json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ComplianceRule 合规规则
type ComplianceRule struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Standard    ComplianceStandard  `json:"standard"`
	CheckLogic  string              `json:"check_logic"`
	Severity    Severity            `json:"severity"`
	Enabled     bool                `json:"enabled"`
}

// ListEntry 黑白名单条目
type ListEntry struct {
	ImageName string    `json:"image_name"`
	Reason    string    `json:"reason,omitempty"`
	AddedAt   time.Time `json:"added_at"`
	AddedBy   string    `json:"added_by,omitempty"`
}

// RuntimeMonitor 运行时监控状态
type RuntimeMonitor struct {
	ContainerID   string     `json:"container_id"`
	ContainerName string     `json:"container_name"`
	ImageName     string     `json:"image_name"`
	IsPrivileged  bool       `json:"is_privileged"`
	LastCheckTime time.Time  `json:"last_check_time"`
	Status        string     `json:"status"`
	Anomalies     []Anomaly  `json:"anomalies,omitempty"`
}

// ScanRequest API 扫描请求
type ScanRequest struct {
	ImageName string `json:"image_name" binding:"required"`
	PolicyID  string `json:"policy_id,omitempty"`
}

// PolicyRequest API 策略请求
type PolicyRequest struct {
	Name                string   `json:"name" binding:"required"`
	Description         string   `json:"description,omitempty"`
	SeverityThreshold   Severity `json:"severity_threshold,omitempty"`
	AutoFixEnabled      bool     `json:"auto_fix_enabled"`
	ComplianceStandards []string `json:"compliance_standards,omitempty"`
	ExcludePackages     []string `json:"exclude_packages,omitempty"`
	ScheduleCron        string   `json:"schedule_cron,omitempty"`
	Enabled             *bool    `json:"enabled,omitempty"`
}

// AutoFixRequest API 自动修复请求
type AutoFixRequest struct {
	ScanID     string     `json:"scan_id" binding:"required"`
	VulnIDs    []string   `json:"vuln_ids,omitempty"`
	FixAction  FixAction  `json:"fix_action,omitempty"`
}

// APIResponse API 响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ScanStats 扫描统计
type ScanStats struct {
	mu                  sync.RWMutex
	TotalScans          int64         `json:"total_scans"`
	CompletedScans      int64         `json:"completed_scans"`
	FailedScans         int64         `json:"failed_scans"`
	TotalVulns          int64         `json:"total_vulns"`
	CriticalVulns       int64         `json:"critical_vulns"`
	HighVulns           int64         `json:"high_vulns"`
	MediumVulns         int64         `json:"medium_vulns"`
	LowVulns            int64         `json:"low_vulns"`
	TotalAnomalies      int64         `json:"total_anomalies"`
	LastScanTime        *time.Time    `json:"last_scan_time,omitempty"`
	AverageScanDuration time.Duration `json:"avg_scan_duration"`
}

// IncrementScans 增加扫描计数
func (s *ScanStats) IncrementScans() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalScans++
}

// IncrementCompleted 增加完成计数
func (s *ScanStats) IncrementCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompletedScans++
}

// IncrementFailed 增加失败计数
func (s *ScanStats) IncrementFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailedScans++
}

// AddVulns 增加漏洞计数
func (s *ScanStats) AddVulns(critical, high, medium, low int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CriticalVulns += int64(critical)
	s.HighVulns += int64(high)
	s.MediumVulns += int64(medium)
	s.LowVulns += int64(low)
	s.TotalVulns += int64(critical + high + medium + low)
}

// AddAnomalies 增加异常计数
func (s *ScanStats) AddAnomalies(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalAnomalies += int64(count)
}

// UpdateLastScan 更新最后扫描时间
func (s *ScanStats) UpdateLastScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.LastScanTime = &now
}

// GetStats 获取统计信息副本
func (s *ScanStats) GetStats() ScanStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ScanStats{
		TotalScans:          s.TotalScans,
		CompletedScans:      s.CompletedScans,
		FailedScans:         s.FailedScans,
		TotalVulns:          s.TotalVulns,
		CriticalVulns:       s.CriticalVulns,
		HighVulns:           s.HighVulns,
		MediumVulns:         s.MediumVulns,
		LowVulns:            s.LowVulns,
		TotalAnomalies:      s.TotalAnomalies,
		LastScanTime:        s.LastScanTime,
		AverageScanDuration: s.AverageScanDuration,
	}
}
