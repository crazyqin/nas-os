// Package scanner 提供安全扫描功能
// 包括文件系统安全扫描、权限检查、漏洞检测集成和安全评分系统
package scanner

import "time"

// ========== 文件系统扫描类型 ==========

// ScanType 扫描类型
type ScanType string

const (
	ScanTypeFull       ScanType = "full"       // 完整扫描
	ScanTypeQuick      ScanType = "quick"      // 快速扫描
	ScanTypeCustom     ScanType = "custom"     // 自定义扫描
	ScanTypeScheduled  ScanType = "scheduled"  // 定时扫描
	ScanTypePermission ScanType = "permission" // 权限扫描
	ScanTypeMalware    ScanType = "malware"    // 恶意软件扫描
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"   // 待执行
	ScanStatusRunning   ScanStatus = "running"   // 执行中
	ScanStatusCompleted ScanStatus = "completed" // 已完成
	ScanStatusFailed    ScanStatus = "failed"    // 失败
	ScanStatusCancelled ScanStatus = "cancelled" // 已取消
	ScanStatusPaused    ScanStatus = "paused"    // 已暂停
)

// Severity 严重级别
type Severity string

const (
	SeverityCritical Severity = "critical" // 严重
	SeverityHigh     Severity = "high"     // 高
	SeverityMedium   Severity = "medium"   // 中
	SeverityLow      Severity = "low"      // 低
	SeverityInfo     Severity = "info"     // 信息
)

// FindingType 发现类型
type FindingType string

const (
	FindingTypePermission    FindingType = "permission"     // 权限问题
	FindingTypeMalware       FindingType = "malware"        // 恶意软件
	FindingTypeSensitiveData FindingType = "sensitive_data" // 敏感数据
	FindingTypeConfiguration FindingType = "configuration"  // 配置问题
	FindingTypeVulnerability FindingType = "vulnerability"  // 漏洞
	FindingTypeSuspicious    FindingType = "suspicious"     // 可疑文件
	FindingTypeIntegrity     FindingType = "integrity"      // 完整性问题
)

// FileFinding 文件扫描发现
type FileFinding struct {
	ID             string                 `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	Type           FindingType            `json:"type"`
	Severity       Severity               `json:"severity"`
	FilePath       string                 `json:"file_path"`
	FileName       string                 `json:"file_name"`
	FileSize       int64                  `json:"file_size"`
	FileHash       string                 `json:"file_hash,omitempty"`
	FileModTime    time.Time              `json:"file_mod_time"`
	Description    string                 `json:"description"`
	Details        map[string]interface{} `json:"details,omitempty"`
	Remediation    string                 `json:"remediation,omitempty"`
	RiskScore      int                    `json:"risk_score"` // 0-100
	FalsePositive  bool                   `json:"false_positive"`
	Acknowledged   bool                   `json:"acknowledged"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
}

// ScanTask 扫描任务
type ScanTask struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Type               ScanType       `json:"type"`
	Status             ScanStatus     `json:"status"`
	TargetPaths        []string       `json:"target_paths"`
	ExcludePaths       []string       `json:"exclude_paths"`
	Options            ScanOptions    `json:"options"`
	CreatedAt          time.Time      `json:"created_at"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	Progress           int            `json:"progress"` // 0-100
	FilesScanned       int            `json:"files_scanned"`
	FilesTotal         int            `json:"files_total"`
	FindingsCount      int            `json:"findings_count"`
	FindingsBySeverity map[string]int `json:"findings_by_severity"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	CreatedBy          string         `json:"created_by"`
}

// ScanOptions 扫描选项
type ScanOptions struct {
	CheckPermissions   bool     `json:"check_permissions"`
	CheckMalware       bool     `json:"check_malware"`
	CheckSensitiveData bool     `json:"check_sensitive_data"`
	CheckIntegrity     bool     `json:"check_integrity"`
	CheckConfiguration bool     `json:"check_configuration"`
	MaxFileSize        int64    `json:"max_file_size"` // 最大扫描文件大小(字节)
	FollowSymlinks     bool     `json:"follow_symlinks"`
	ScanHiddenFiles    bool     `json:"scan_hidden_files"`
	ScanArchives       bool     `json:"scan_archives"`
	ConcurrentScanners int      `json:"concurrent_scanners"`
	FileExtensions     []string `json:"file_extensions,omitempty"`
	ExcludeExtensions  []string `json:"exclude_extensions,omitempty"`
	HashAlgorithms     []string `json:"hash_algorithms"`
	GenerateReport     bool     `json:"generate_report"`
}

// DefaultScanOptions 默认扫描选项
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		CheckPermissions:   true,
		CheckMalware:       false, // 需要额外配置
		CheckSensitiveData: true,
		CheckIntegrity:     true,
		CheckConfiguration: false,
		MaxFileSize:        100 * 1024 * 1024, // 100MB
		FollowSymlinks:     false,
		ScanHiddenFiles:    true,
		ScanArchives:       false,
		ConcurrentScanners: 4,
		HashAlgorithms:     []string{"sha256", "md5"},
		GenerateReport:     true,
	}
}

// FileScanReport 文件扫描报告
type FileScanReport struct {
	ReportID        string            `json:"report_id"`
	TaskID          string            `json:"task_id"`
	GeneratedAt     time.Time         `json:"generated_at"`
	ScanDuration    int64             `json:"scan_duration"` // 秒
	Summary         FileScanSummary   `json:"summary"`
	Findings        []*FileFinding    `json:"findings"`
	Recommendations []string          `json:"recommendations"`
	RiskScore       int               `json:"risk_score"`
	RiskLevel       string            `json:"risk_level"`
}

// FileScanSummary 文件扫描摘要
type FileScanSummary struct {
	TotalFiles     int            `json:"total_files"`
	ScannedFiles   int            `json:"scanned_files"`
	SkippedFiles   int            `json:"skipped_files"`
	ErrorFiles     int            `json:"error_files"`
	TotalFindings  int            `json:"total_findings"`
	CriticalCount  int            `json:"critical_count"`
	HighCount      int            `json:"high_count"`
	MediumCount    int            `json:"medium_count"`
	LowCount       int            `json:"low_count"`
	InfoCount      int            `json:"info_count"`
	FindingsByType map[string]int `json:"findings_by_type"`
}

// ========== 权限检查类型 ==========

// PermissionIssue 权限问题
type PermissionIssue struct {
	Path            string   `json:"path"`
	Type            string   `json:"type"` // file, directory
	CurrentMode     string   `json:"current_mode"`
	RecommendedMode string   `json:"recommended_mode"`
	Issue           string   `json:"issue"`
	Severity        Severity `json:"severity"`
	Owner           string   `json:"owner"`
	Group           string   `json:"group"`
	Risk            string   `json:"risk"`
}

// PermissionCheckResult 权限检查结果
type PermissionCheckResult struct {
	ScanTime       time.Time          `json:"scan_time"`
	TotalChecked   int                `json:"total_checked"`
	IssuesFound    int                `json:"issues_found"`
	CriticalIssues int                `json:"critical_issues"`
	WarningIssues  int                `json:"warning_issues"`
	Issues         []*PermissionIssue `json:"issues"`
	Suggestions    []string           `json:"suggestions"`
}

// PermissionRule 权限规则
type PermissionRule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	PathPattern   string   `json:"path_pattern"`  // glob模式
	RequiredMode  string   `json:"required_mode"` // 如 "0755"
	RequiredOwner string   `json:"required_owner,omitempty"`
	RequiredGroup string   `json:"required_group,omitempty"`
	MaxMode       string   `json:"max_mode,omitempty"` // 最大允许权限
	Severity      Severity `json:"severity"`
	Enabled       bool     `json:"enabled"`
}

// ========== 漏洞检测类型 ==========

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID                string     `json:"id"`
	CVE               string     `json:"cve,omitempty"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Severity          Severity   `json:"severity"`
	CVSSScore         float64    `json:"cvss_score"`
	CVSSVector        string     `json:"cvss_vector,omitempty"`
	AffectedComponent string     `json:"affected_component"`
	AffectedVersion   string     `json:"affected_version"`
	FixedVersion      string     `json:"fixed_version,omitempty"`
	References        []string   `json:"references,omitempty"`
	PublishedDate     *time.Time `json:"published_date,omitempty"`
	LastModified      *time.Time `json:"last_modified,omitempty"`
	ExploitAvailable  bool       `json:"exploit_available"`
	PatchAvailable    bool       `json:"patch_available"`
}

// VulnerabilityScanResult 漏洞扫描结果
type VulnerabilityScanResult struct {
	ScanID          string           `json:"scan_id"`
	ScanTime        time.Time        `json:"scan_time"`
	Component       string           `json:"component"`
	Version         string           `json:"version"`
	Vulnerabilities []*Vulnerability `json:"vulnerabilities"`
	TotalVulns      int              `json:"total_vulns"`
	CriticalVulns   int              `json:"critical_vulns"`
	HighVulns       int              `json:"high_vulns"`
	MediumVulns     int              `json:"medium_vulns"`
	LowVulns        int              `json:"low_vulns"`
	Summary         string           `json:"summary"`
	LastUpdated     time.Time        `json:"last_updated"`
}

// VulnerabilityDatabase 漏洞数据库配置
type VulnerabilityDatabase struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	LastSync    time.Time `json:"last_sync"`
	RecordCount int       `json:"record_count"`
	Enabled     bool      `json:"enabled"`
}

// ========== 安全评分类型 ==========

// SecurityScore 安全评分
type SecurityScore struct {
	OverallScore       int                `json:"overall_score"` // 0-100
	Grade              string             `json:"grade"`         // A-F
	Level              string             `json:"level"`         // excellent, good, fair, poor, critical
	CalculatedAt       time.Time          `json:"calculated_at"`
	CategoryScores     map[string]int     `json:"category_scores"`
	CategoryWeights    map[string]float64 `json:"category_weights"`
	Findings           []*ScoreFinding    `json:"findings"`
	Trend              string             `json:"trend"` // improving, stable, declining
	PreviousScore      *int               `json:"previous_score,omitempty"`
	ChangeFromPrevious int                `json:"change_from_previous"`
}

// ScoreCategory 评分类别
type ScoreCategory struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"` // 权重 0-1
	MaxScore    int     `json:"max_score"`
	Enabled     bool    `json:"enabled"`
}

// ScoreFinding 评分发现
type ScoreFinding struct {
	Category    string   `json:"category"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Impact      int      `json:"impact"` // 对分数的影响
	Remediation string   `json:"remediation,omitempty"`
}

// ScoreHistory 评分历史
type ScoreHistory struct {
	Date     time.Time `json:"date"`
	Score    int       `json:"score"`
	Grade    string    `json:"grade"`
	Findings int       `json:"findings"`
}

// ========== 敏感数据检测类型 ==========

// SensitiveDataType 敏感数据类型
type SensitiveDataType string

const (
	SensitiveDataPassword   SensitiveDataType = "password"
	SensitiveDataAPIKey     SensitiveDataType = "api_key"
	SensitiveDataCreditCard SensitiveDataType = "credit_card"
	SensitiveDataSSN        SensitiveDataType = "ssn"
	SensitiveDataPrivateKey SensitiveDataType = "private_key"
	SensitiveDataCredential SensitiveDataType = "credential"
	SensitiveDataToken      SensitiveDataType = "token"
	SensitiveDataDatabase   SensitiveDataType = "database"
)

// SensitiveDataFinding 敏感数据发现
type SensitiveDataFinding struct {
	Type        SensitiveDataType `json:"type"`
	FilePath    string            `json:"file_path"`
	LineNumber  int               `json:"line_number,omitempty"`
	Match       string            `json:"match,omitempty"` // 脱敏后的匹配内容
	Context     string            `json:"context,omitempty"`
	Confidence  float64           `json:"confidence"` // 0-1
	Severity    Severity          `json:"severity"`
	Description string            `json:"description"`
}

// SensitiveDataRule 敏感数据检测规则
type SensitiveDataRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        SensitiveDataType `json:"type"`
	Pattern     string            `json:"pattern"` // 正则表达式
	Description string            `json:"description"`
	Severity    Severity          `json:"severity"`
	Enabled     bool              `json:"enabled"`
	FileTypes   []string          `json:"file_types,omitempty"` // 适用的文件类型
}

// ========== 恶意软件检测类型 ==========

// MalwareType 恶意软件类型
type MalwareType string

const (
	MalwareTypeVirus      MalwareType = "virus"
	MalwareTypeTrojan     MalwareType = "trojan"
	MalwareTypeWorm       MalwareType = "worm"
	MalwareTypeRansomware MalwareType = "ransomware"
	MalwareTypeSpyware    MalwareType = "spyware"
	MalwareTypeAdware     MalwareType = "adware"
	MalwareTypeRootkit    MalwareType = "rootkit"
	MalwareTypeBackdoor   MalwareType = "backdoor"
	MalwareTypePUA        MalwareType = "pua" // Potentially Unwanted Application
)

// MalwareFinding 恶意软件发现
type MalwareFinding struct {
	FilePath       string      `json:"file_path"`
	FileName       string      `json:"file_name"`
	FileHash       string      `json:"file_hash"`
	FileSize       int64       `json:"file_size"`
	MalwareType    MalwareType `json:"malware_type"`
	MalwareName    string      `json:"malware_name"`
	Confidence     float64     `json:"confidence"`
	Severity       Severity    `json:"severity"`
	Description    string      `json:"description"`
	FirstSeen      *time.Time  `json:"first_seen,omitempty"`
	Quarantined    bool        `json:"quarantined"`
	QuarantinePath string      `json:"quarantine_path,omitempty"`
}

// ========== 配置类型 ==========

// ScannerConfig 扫描器配置
type ScannerConfig struct {
	Enabled            bool                    `json:"enabled"`
	MaxConcurrentScans int                     `json:"max_concurrent_scans"`
	DefaultOptions     ScanOptions             `json:"default_options"`
	ScheduledScans     []ScheduledScan         `json:"scheduled_scans"`
	SensitiveDataRules []SensitiveDataRule     `json:"sensitive_data_rules"`
	PermissionRules    []PermissionRule        `json:"permission_rules"`
	VulnerabilityDBs   []VulnerabilityDatabase `json:"vulnerability_dbs"`
	QuarantineDir      string                  `json:"quarantine_dir"`
	ReportRetention    int                     `json:"report_retention"` // 天数
	MaxReportCount     int                     `json:"max_report_count"`
}

// ScheduledScan 定时扫描配置
type ScheduledScan struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        ScanType   `json:"type"`
	TargetPaths []string   `json:"target_paths"`
	Schedule    string     `json:"schedule"` // cron表达式
	Enabled     bool       `json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
}

// DefaultScannerConfig 默认扫描器配置
func DefaultScannerConfig() ScannerConfig {
	return ScannerConfig{
		Enabled:            true,
		MaxConcurrentScans: 3,
		DefaultOptions:     DefaultScanOptions(),
		ScheduledScans:     make([]ScheduledScan, 0),
		SensitiveDataRules: DefaultSensitiveDataRules(),
		PermissionRules:    DefaultPermissionRules(),
		QuarantineDir:      "/var/lib/nas-os/quarantine",
		ReportRetention:    90,
		MaxReportCount:     100,
	}
}

// DefaultSensitiveDataRules 默认敏感数据检测规则
func DefaultSensitiveDataRules() []SensitiveDataRule {
	return []SensitiveDataRule{
		{
			ID:          "password",
			Name:        "密码检测",
			Type:        SensitiveDataPassword,
			Pattern:     `(?i)(password|passwd|pwd)\s*[=:]\s*['"]?[^\s'"]+['"]?`,
			Description: "检测可能的密码字符串",
			Severity:    SeverityHigh,
			Enabled:     true,
		},
		{
			ID:          "api_key",
			Name:        "API密钥检测",
			Type:        SensitiveDataAPIKey,
			Pattern:     `(?i)(api[_-]?key|apikey)\s*[=:]\s*['"]?[a-zA-Z0-9]{20,}['"]?`,
			Description: "检测可能的API密钥",
			Severity:    SeverityHigh,
			Enabled:     true,
		},
		{
			ID:          "private_key",
			Name:        "私钥检测",
			Type:        SensitiveDataPrivateKey,
			Pattern:     `-----BEGIN (RSA |DSA |EC |OPENSSH )?PRIVATE KEY-----`,
			Description: "检测私钥文件",
			Severity:    SeverityCritical,
			Enabled:     true,
		},
		{
			ID:          "credit_card",
			Name:        "信用卡号检测",
			Type:        SensitiveDataCreditCard,
			Pattern:     `\b(?:\d{4}[-\s]?){3}\d{4}\b`,
			Description: "检测信用卡号码",
			Severity:    SeverityHigh,
			Enabled:     true,
		},
		{
			ID:          "aws_key",
			Name:        "AWS密钥检测",
			Type:        SensitiveDataAPIKey,
			Pattern:     `(?:A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`,
			Description: "检测AWS访问密钥",
			Severity:    SeverityCritical,
			Enabled:     true,
		},
	}
}

// DefaultPermissionRules 默认权限规则
func DefaultPermissionRules() []PermissionRule {
	return []PermissionRule{
		{
			ID:           "ssh_keys",
			Name:         "SSH私钥权限",
			Description:  "SSH私钥文件权限应为600",
			PathPattern:  "*id_rsa*",
			RequiredMode: "0600",
			Severity:     SeverityCritical,
			Enabled:      true,
		},
		{
			ID:           "ssh_dir",
			Name:         "SSH目录权限",
			Description:  "SSH目录权限应为700",
			PathPattern:  "*/.ssh",
			RequiredMode: "0700",
			Severity:     SeverityHigh,
			Enabled:      true,
		},
		{
			ID:          "world_writable",
			Name:        "全局可写文件",
			Description: "文件不应全局可写",
			MaxMode:     "0755",
			Severity:    SeverityMedium,
			Enabled:     true,
		},
		{
			ID:          "config_files",
			Name:        "配置文件权限",
			Description: "配置文件权限应不大于640",
			PathPattern: "*.conf",
			MaxMode:     "0640",
			Severity:    SeverityMedium,
			Enabled:     true,
		},
	}
}

// ========== API响应类型 ==========

// APIResponse 通用API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 成功响应
func SuccessResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// ErrorResponse 错误响应
func ErrorResponse(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// 错误码定义
const (
	ErrCodeInvalidParam   = 400
	ErrCodeNotFound       = 404
	ErrCodeInternalError  = 500
	ErrCodeScanInProgress = 409
	ErrCodeScanNotFound   = 404
)
