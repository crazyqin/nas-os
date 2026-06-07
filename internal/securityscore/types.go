// Package securityscore 提供安全评分功能
package securityscore

import (
	"time"
)

// CheckStatus 检查状态.
type CheckStatus string

const (
	StatusPass    CheckStatus = "pass"
	StatusFail    CheckStatus = "fail"
	StatusWarning CheckStatus = "warning"
)

// Grade 等级.
type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeF Grade = "F"
)

// SecurityScore 安全评分.
type SecurityScore struct {
	Overall     float64                  `json:"overall"`
	Categories  map[string]CategoryScore `json:"categories"`
	Grade       Grade                    `json:"grade"`
	LastUpdated time.Time                `json:"last_updated"`
}

// CategoryScore 分类评分.
type CategoryScore struct {
	Name   string          `json:"name"`
	Score  float64         `json:"score"`
	Weight float64         `json:"weight"`
	Checks []SecurityCheck `json:"checks"`
	Issues []string        `json:"issues"`
}

// SecurityCheck 安全检查.
type SecurityCheck struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Status      CheckStatus `json:"status"`
	Details     string      `json:"details"`
}

// ScoreHistory 评分历史记录.
type ScoreHistory struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Score     SecurityScore `json:"score"`
}

// Recommendation 改进建议.
type Recommendation struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high/medium/low
	Impact      string `json:"impact"`
}

// ========== 漏洞扫描类型 ==========

// CVSSSeverity CVSS 严重程度.
type CVSSSeverity string

const (
	CVSSCritical CVSSSeverity = "critical"
	CVSSHigh     CVSSSeverity = "high"
	CVSSMedium   CVSSSeverity = "medium"
	CVSSLow      CVSSSeverity = "low"
	CVSSNone     CVSSSeverity = "none"
)

// VulnSource 漏洞来源.
type VulnSource string

const (
	VulnSourceNVD   VulnSource = "nvd"
	VulnSourceOSV   VulnSource = "osv"
	VulnSourceCNVD  VulnSource = "cnvd"
	VulnSourceCNNVD VulnSource = "cnnvd"
)

// CVEDetail CVE 详情.
type CVEDetail struct {
	ID                 string            `json:"id"`
	CVEID              string            `json:"cve_id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Severity           CVSSSeverity      `json:"severity"`
	CVSSScore          float64           `json:"cvss_score"`
	CVSSVector         string            `json:"cvss_vector"`
	AttackVector       string            `json:"attack_vector"`
	AttackComplexity   string            `json:"attack_complexity"`
	PrivilegesRequired string            `json:"privileges_required"`
	UserInteraction    string            `json:"user_interaction"`
	Scope              string            `json:"scope"`
	Confidentiality    string            `json:"confidentiality_impact"`
	Integrity          string            `json:"integrity_impact"`
	Availability       string            `json:"availability_impact"`
	PublishedDate      time.Time         `json:"published_date"`
	ModifiedDate       time.Time         `json:"modified_date"`
	References         []string          `json:"references"`
	CWEs               []string          `json:"cwes"`
	CPEs               []string          `json:"cpes"`
	AffectedProducts   []AffectedProduct `json:"affected_products"`
	ExploitStatus      ExploitStatus     `json:"exploit_status"`
	PatchInfo          *PatchInfo        `json:"patch_info,omitempty"`
}

// AffectedProduct 受影响产品.
type AffectedProduct struct {
	Vendor   string   `json:"vendor"`
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

// ExploitStatus 漏洞利用状态.
type ExploitStatus struct {
	KnownExploit  bool       `json:"known_exploit"`
	ExploitDBID   string     `json:"exploit_db_id,omitempty"`
	ExploitURL    string     `json:"exploit_url,omitempty"`
	ActiveInWild  bool       `json:"active_in_wild"`
	FirstSeenDate *time.Time `json:"first_seen_date,omitempty"`
}

// PatchInfo 补丁信息.
type PatchInfo struct {
	Available   bool       `json:"available"`
	VendorPatch string     `json:"vendor_patch,omitempty"`
	PatchDate   *time.Time `json:"patch_date,omitempty"`
	Workaround  string     `json:"workaround,omitempty"`
}

// VulnerabilityScanner 漏洞扫描器.
type VulnerabilityScanner struct {
	databases map[string]*VulnerabilityDatabase
	cache     map[string]*VulnCacheEntry
}

// VulnerabilityDatabase 漏洞数据库.
type VulnerabilityDatabase struct {
	Name        string     `json:"name"`
	Source      VulnSource `json:"source"`
	URL         string     `json:"url"`
	APIKey      string     `json:"api_key,omitempty"`
	Enabled     bool       `json:"enabled"`
	LastSync    time.Time  `json:"last_sync"`
	RecordCount int        `json:"record_count"`
	Status      string     `json:"status"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
}

// VulnCacheEntry 缓存条目.
type VulnCacheEntry struct {
	Key        string
	Data       interface{}
	Expiry     time.Time
	HitCount   int
	LastAccess time.Time
}

// ScanTarget 扫描目标.
type ScanTarget struct {
	Type     string            `json:"type"`
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Vendor   string            `json:"vendor,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VulnerabilityScanResult 漏洞扫描结果.
type VulnerabilityScanResult struct {
	ScanID          string              `json:"scan_id"`
	Timestamp       time.Time           `json:"timestamp"`
	Target          ScanTarget          `json:"target"`
	Vulnerabilities []VulnerabilityItem `json:"vulnerabilities"`
	Summary         VulnScanSummary     `json:"summary"`
	Score           int                 `json:"score"`
	RiskLevel       string              `json:"risk_level"`
	Duration        time.Duration       `json:"duration"`
	DataSource      []string            `json:"data_source"`
}

// VulnerabilityItem 漏洞项.
type VulnerabilityItem struct {
	ID               string       `json:"id"`
	CVEID            string       `json:"cve_id"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	Severity         CVSSSeverity `json:"severity"`
	CVSSScore        float64      `json:"cvss_score"`
	CVSSVector       string       `json:"cvss_vector"`
	Status           string       `json:"status"`
	Priority         int          `json:"priority"`
	FirstDetected    time.Time    `json:"first_detected"`
	LastSeen         time.Time    `json:"last_seen"`
	FixedVersion     string       `json:"fixed_version,omitempty"`
	Remediation      string       `json:"remediation,omitempty"`
	References       []string     `json:"references"`
	ExploitAvailable bool         `json:"exploit_available"`
	PatchAvailable   bool         `json:"patch_available"`
	Acknowledged     bool         `json:"acknowledged"`
	AcknowledgedBy   string       `json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time   `json:"acknowledged_at,omitempty"`
}

// VulnScanSummary 扫描摘要.
type VulnScanSummary struct {
	TotalVulns       int `json:"total_vulns"`
	CriticalVulns    int `json:"critical_vulns"`
	HighVulns        int `json:"high_vulns"`
	MediumVulns      int `json:"medium_vulns"`
	LowVulns         int `json:"low_vulns"`
	ExploitableVulns int `json:"exploitable_vulns"`
	PatchableVulns   int `json:"patchable_vulns"`
}

// ========== 合规检查类型 ==========

// ComplianceStandard 合规标准类型.
type ComplianceStandard string

const (
	StandardGDPR     ComplianceStandard = "gdpr"
	StandardSOC2     ComplianceStandard = "soc2"
	StandardISO27001 ComplianceStandard = "iso27001"
	StandardHIPAA    ComplianceStandard = "hipaa"
	StandardPCI      ComplianceStandard = "pci"
	StandardCCPA     ComplianceStandard = "ccpa"
	StandardNIST     ComplianceStandard = "nist"
	StandardCSL      ComplianceStandard = "csl"
	StandardDSL      ComplianceStandard = "dsl"
	StandardPIPL     ComplianceStandard = "pipl"
	StandardMLPS     ComplianceStandard = "mlps" // 等保 2.0
)

// ComplianceLevel 合规等级.
type ComplianceLevel string

const (
	LevelFull         ComplianceLevel = "full"
	LevelPartial      ComplianceLevel = "partial"
	LevelNonCompliant ComplianceLevel = "non_compliant"
	LevelUnknown      ComplianceLevel = "unknown"
)

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	StatusPassed        ComplianceStatus = "passed"
	StatusFailed        ComplianceStatus = "failed"
	StatusCompWarning   ComplianceStatus = "warning"
	StatusSkipped       ComplianceStatus = "skipped"
	StatusNotApplicable ComplianceStatus = "not_applicable"
)

// ComplianceCategory 合规检查类别.
type ComplianceCategory string

const (
	CategoryAccessControl      ComplianceCategory = "access_control"
	CategoryDataProtection     ComplianceCategory = "data_protection"
	CategoryEncryption         ComplianceCategory = "encryption"
	CategoryAudit              ComplianceCategory = "audit"
	CategoryIncidentResponse   ComplianceCategory = "incident_response"
	CategoryBusinessContinuity ComplianceCategory = "business_continuity"
	CategoryAssetManagement    ComplianceCategory = "asset_management"
	CategoryNetworkSecurity    ComplianceCategory = "network_security"
	CategoryVulnerability      ComplianceCategory = "vulnerability"
	CategoryPrivacy            ComplianceCategory = "privacy"
	CategoryConsent            ComplianceCategory = "consent"
	CategoryBreachNotification ComplianceCategory = "breach_notification"
	CategoryDataRetention      ComplianceCategory = "data_retention"
	CategoryThirdParty         ComplianceCategory = "third_party"
)

// ComplianceCheckItem 合规检查项.
type ComplianceCheckItem struct {
	ID              string             `json:"id"`
	Standard        ComplianceStandard `json:"standard"`
	ControlID       string             `json:"control_id"`
	Category        ComplianceCategory `json:"category"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Requirement     string             `json:"requirement"`
	Weight          int                `json:"weight"`
	Severity        string             `json:"severity"`
	Remediation     string             `json:"remediation"`
	References      []string           `json:"references"`
	ApplicableRoles []string           `json:"applicable_roles"`
	Tags            []string           `json:"tags"`
}

// ComplianceCheckResult 合规检查结果.
type ComplianceCheckResult struct {
	ItemID         string                 `json:"item_id"`
	Standard       ComplianceStandard     `json:"standard"`
	ControlID      string                 `json:"control_id"`
	Category       ComplianceCategory     `json:"category"`
	Name           string                 `json:"name"`
	Status         ComplianceStatus       `json:"status"`
	Level          ComplianceLevel        `json:"level"`
	Score          int                    `json:"score"`
	Message        string                 `json:"message"`
	Details        map[string]interface{} `json:"details,omitempty"`
	Evidence       []string               `json:"evidence,omitempty"`
	Remediation    string                 `json:"remediation,omitempty"`
	CheckTime      time.Time              `json:"check_time"`
	Duration       time.Duration          `json:"duration"`
	CheckedBy      string                 `json:"checked_by"`
	Acknowledged   bool                   `json:"acknowledged"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ReportID        string                   `json:"report_id"`
	Title           string                   `json:"title"`
	Standard        ComplianceStandard       `json:"standard"`
	GeneratedAt     time.Time                `json:"generated_at"`
	ValidUntil      time.Time                `json:"valid_until"`
	OverallScore    int                      `json:"overall_score"`
	OverallLevel    ComplianceLevel          `json:"overall_level"`
	Summary         ComplianceSummary        `json:"summary"`
	CategoryScores  map[string]int           `json:"category_scores"`
	Results         []*ComplianceCheckResult `json:"results"`
	Remediations    []RemediationItem        `json:"remediations"`
	Recommendations []string                 `json:"recommendations"`
	NextReviewDate  time.Time                `json:"next_review_date"`
	Version         string                   `json:"version"`
}

// ComplianceSummary 合规摘要.
type ComplianceSummary struct {
	TotalChecks    int `json:"total_checks"`
	PassedChecks   int `json:"passed_checks"`
	FailedChecks   int `json:"failed_checks"`
	WarningChecks  int `json:"warning_checks"`
	SkippedChecks  int `json:"skipped_checks"`
	NotApplicable  int `json:"not_applicable"`
	CriticalIssues int `json:"critical_issues"`
	HighIssues     int `json:"high_issues"`
	MediumIssues   int `json:"medium_issues"`
	LowIssues      int `json:"low_issues"`
}

// RemediationItem 整改项.
type RemediationItem struct {
	ID          string     `json:"id"`
	ItemID      string     `json:"item_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"`
	Status      string     `json:"status"`
	AssignedTo  string     `json:"assigned_to,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Resolution  string     `json:"resolution,omitempty"`
}

// StandardDefinition 标准定义.
type StandardDefinition struct {
	Standard   ComplianceStandard     `json:"standard"`
	Name       string                 `json:"name"`
	Version    string                 `json:"version"`
	Effective  time.Time              `json:"effective"`
	CheckItems []*ComplianceCheckItem `json:"check_items"`
	Weights    map[string]int         `json:"weights"`
}

// ========== 安全趋势类型 ==========

// SecurityTrend 安全趋势.
type SecurityTrend struct {
	Period      TrendPeriod       `json:"period"`
	StartDate   time.Time         `json:"start_date"`
	EndDate     time.Time         `json:"end_date"`
	DataPoints  []TrendDataPoint  `json:"data_points"`
	Analysis    TrendAnalysis     `json:"analysis"`
	Predictions []TrendPrediction `json:"predictions,omitempty"`
}

// TrendPeriod 趋势周期.
type TrendPeriod string

const (
	TrendDaily   TrendPeriod = "daily"
	TrendWeekly  TrendPeriod = "weekly"
	TrendMonthly TrendPeriod = "monthly"
	TrendYearly  TrendPeriod = "yearly"
)

// TrendDataPoint 趋势数据点.
type TrendDataPoint struct {
	Timestamp     time.Time          `json:"timestamp"`
	Score         float64            `json:"score"`
	Grade         Grade              `json:"grade"`
	Categories    map[string]float64 `json:"categories"`
	VulnCount     int                `json:"vuln_count"`
	Compliance    int                `json:"compliance_score"`
	IncidentCount int                `json:"incident_count"`
	Changes       []TrendChange      `json:"changes,omitempty"`
}

// TrendChange 趋势变化.
type TrendChange struct {
	Category string  `json:"category"`
	OldScore float64 `json:"old_score"`
	NewScore float64 `json:"new_score"`
	Change   float64 `json:"change"`
	Reason   string  `json:"reason"`
}

// TrendAnalysis 趋势分析.
type TrendAnalysis struct {
	OverallTrend    string   `json:"overall_trend"` // improving, declining, stable
	ImprovementRate float64  `json:"improvement_rate"`
	DeclineRate     float64  `json:"decline_rate"`
	Volatility      float64  `json:"volatility"`
	BestCategory    string   `json:"best_category"`
	WorstCategory   string   `json:"worst_category"`
	Insights        []string `json:"insights"`
}

// TrendPrediction 趋势预测.
type TrendPrediction struct {
	Date           time.Time `json:"date"`
	PredictedScore float64   `json:"predicted_score"`
	Confidence     float64   `json:"confidence"`
	Factors        []string  `json:"factors"`
}

// ========== 安全报告类型 ==========

// SecurityReport 安全报告.
type SecurityReport struct {
	ReportID         string                   `json:"report_id"`
	Title            string                   `json:"title"`
	GeneratedAt      time.Time                `json:"generated_at"`
	ValidUntil       time.Time                `json:"valid_until"`
	Score            SecurityScore            `json:"score"`
	VulnScan         *VulnerabilityScanResult `json:"vuln_scan,omitempty"`
	Compliance       []*ComplianceReport      `json:"compliance,omitempty"`
	Trends           *SecurityTrend           `json:"trends,omitempty"`
	Recommendations  []Recommendation         `json:"recommendations"`
	Summary          ReportSummary            `json:"summary"`
	RiskMatrix       RiskMatrix               `json:"risk_matrix"`
	ExecutiveSummary string                   `json:"executive_summary"`
	Version          string                   `json:"version"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	OverallScore    float64 `json:"overall_score"`
	Grade           Grade   `json:"grade"`
	TotalChecks     int     `json:"total_checks"`
	PassedChecks    int     `json:"passed_checks"`
	FailedChecks    int     `json:"failed_checks"`
	WarningChecks   int     `json:"warning_checks"`
	CriticalVulns   int     `json:"critical_vulns"`
	HighVulns       int     `json:"high_vulns"`
	ComplianceScore int     `json:"compliance_score"`
	TrendDirection  string  `json:"trend_direction"`
	RiskLevel       string  `json:"risk_level"`
}

// RiskMatrix 风险矩阵.
type RiskMatrix struct {
	HighImpactHighProb []RiskItem `json:"high_impact_high_prob"`
	HighImpactLowProb  []RiskItem `json:"high_impact_low_prob"`
	LowImpactHighProb  []RiskItem `json:"low_impact_high_prob"`
	LowImpactLowProb   []RiskItem `json:"low_impact_low_prob"`
}

// RiskItem 风险项.
type RiskItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Impact      int     `json:"impact"`      // 1-5
	Probability int     `json:"probability"` // 1-5
	RiskScore   float64 `json:"risk_score"`
	Category    string  `json:"category"`
	Source      string  `json:"source"`
}

// ========== 安全建议类型 ==========

// SecurityAdvisor 安全建议器.
type SecurityAdvisor struct {
	rules []AdvisorRule
}

// AdvisorRule 建议规则.
type AdvisorRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Condition  string   `json:"condition"`
	Severity   string   `json:"severity"`
	Suggestion string   `json:"suggestion"`
	References []string `json:"references"`
	AutoFix    bool     `json:"auto_fix"`
	FixScript  string   `json:"fix_script,omitempty"`
}

// AdvisorAdvice 建议.
type AdvisorAdvice struct {
	RuleID        string   `json:"rule_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Priority      string   `json:"priority"`
	Impact        string   `json:"impact"`
	Steps         []string `json:"steps"`
	References    []string `json:"references"`
	AutoFixable   bool     `json:"auto_fixable"`
	EstimatedTime string   `json:"estimated_time"`
}

// ========== 配置类型 ==========

// SecurityScoreConfig 安全评分配置.
type SecurityScoreConfig struct {
	Enabled           bool                 `json:"enabled"`
	AutoScan          bool                 `json:"auto_scan"`
	ScanInterval      time.Duration        `json:"scan_interval"`
	EnabledStandards  []ComplianceStandard `json:"enabled_standards"`
	ScoreThreshold    float64              `json:"score_threshold"`
	NotifyOnLowScore  bool                 `json:"notify_on_low_score"`
	ReportRetention   int                  `json:"report_retention"`
	MaxHistoryEntries int                  `json:"max_history_entries"`
}

// DefaultSecurityScoreConfig 默认配置.
func DefaultSecurityScoreConfig() SecurityScoreConfig {
	return SecurityScoreConfig{
		Enabled:           true,
		AutoScan:          true,
		ScanInterval:      24 * time.Hour,
		EnabledStandards:  []ComplianceStandard{StandardGDPR, StandardISO27001, StandardMLPS},
		ScoreThreshold:    70,
		NotifyOnLowScore:  true,
		ReportRetention:   365,
		MaxHistoryEntries: 100,
	}
}
