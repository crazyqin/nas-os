// Package containerguardian provides container security scanning, runtime monitoring,
// image signature verification, CIS Docker Benchmark compliance, sensitive data leak
// detection, security grading (A/B/C/D/F), auto-remediation, and security report generation.
package containerguardian

import (
	"time"
)

// SeverityLevel 表示漏洞严重程度
// 兼容 containerguardian.go 的 int 类型用法
type SeverityLevel = string

// Severity levels for vulnerabilities and findings
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityInfo     = "INFO"
)

// SecurityGrade represents a letter security grade
type SecurityGrade string

const (
	GradeA SecurityGrade = "A"
	GradeB SecurityGrade = "B"
	GradeC SecurityGrade = "C"
	GradeD SecurityGrade = "D"
	GradeF SecurityGrade = "F"
)

// ScanStatus represents the status of a scan
type ScanStatus string

const (
	ScanStatusQueued    ScanStatus = "queued"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// SignatureStatus represents image signature verification result
type SignatureStatus string

const (
	SignatureValid    SignatureStatus = "valid"
	SignatureInvalid  SignatureStatus = "invalid"
	SignatureMissing  SignatureStatus = "missing"
	SignatureUnknown  SignatureStatus = "unknown"
)

// ComplianceStatus represents compliance check result
type ComplianceStatus string

const (
	CompliancePass ComplianceStatus = "pass"
	ComplianceFail ComplianceStatus = "fail"
	ComplianceWarn ComplianceStatus = "warn"
	ComplianceSkip ComplianceStatus = "skip"
)

// ReportFormat defines output format for reports
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatHTML ReportFormat = "html"
)

// SensitivityLevel defines how sensitive leaked data is
type SensitivityLevel string

const (
	SensitivityCritical SensitivityLevel = "critical"
	SensitivityHigh     SensitivityLevel = "high"
	SensitivityMedium   SensitivityLevel = "medium"
	SensitivityLow      SensitivityLevel = "low"
)

// ScanResult represents the complete result of a security scan
type ScanResult struct {
	ID              string              `json:"id"`
	Image           string              `json:"image"`
	Tag             string              `json:"tag"`
	Status          ScanStatus          `json:"status"`
	Score           float64             `json:"score"`
	Grade           SecurityGrade       `json:"grade"`
	Summary         VulnSummary         `json:"summary"`
	Vulnerabilities []Vulnerability     `json:"vulnerabilities"`
	Compliance      []ComplianceRule    `json:"compliance"`
	Signature       *SignatureResult    `json:"signature,omitempty"`
	Sensitive       []SensitiveFinding  `json:"sensitive,omitempty"`
	Runtime         *RuntimeStatus      `json:"runtime,omitempty"`
	Remediations    []Remediation       `json:"remediations"`
	ScannedAt       time.Time           `json:"scanned_at"`
	Duration        time.Duration       `json:"duration"`
	Error           string              `json:"error,omitempty"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string    `json:"id"`
	CVE         string    `json:"cve"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixedIn     string    `json:"fixed_in,omitempty"`
	URL         string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// VulnSummary provides vulnerability count by severity
type VulnSummary struct {
	Total          int `json:"total"`
	Critical       int `json:"critical"`
	High           int `json:"high"`
	Medium         int `json:"medium"`
	Low            int `json:"low"`
	Info           int `json:"info"`
	Fixed          int `json:"fixed"`
	Unfixed        int `json:"unfixed"`
	CompliancePass int `json:"compliance_pass"`
	ComplianceFail int `json:"compliance_fail"`
}

// ComplianceRule represents a CIS Docker Benchmark check
type ComplianceRule struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Status      ComplianceStatus `json:"status"`
	Message     string           `json:"message,omitempty"`
	Severity    string           `json:"severity"`
}

// SignatureResult represents image signature verification result
type SignatureResult struct {
	Status      SignatureStatus `json:"status"`
	Signer      string          `json:"signer,omitempty"`
	SignedAt    *time.Time      `json:"signed_at,omitempty"`
	KeyID       string          `json:"key_id,omitempty"`
	Algorithm   string          `json:"algorithm,omitempty"`
	Fingerprint string          `json:"fingerprint,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// SensitiveFinding represents a sensitive data leak finding
type SensitiveFinding struct {
	Type        string          `json:"type"`
	Location    string          `json:"location"`
	Value       string          `json:"value,omitempty"`
	Sensitivity SensitivityLevel `json:"sensitivity"`
	Description string          `json:"description"`
	Remediation string          `json:"remediation,omitempty"`
}

// RuntimeStatus represents container runtime security status
type RuntimeStatus struct {
	ContainerID      string        `json:"container_id"`
	Running          bool          `json:"running"`
	CPUUsage         float64       `json:"cpu_usage"`
	MemoryUsage      int64         `json:"memory_usage"`
	NetworkIO        int64         `json:"network_io"`
	PidsCount        int           `json:"pids_count"`
	Anomalies        []Anomaly     `json:"anomalies"`
	Uptime           time.Duration `json:"uptime"`
	Privileged       bool          `json:"privileged"`
	ReadOnlyRoot     bool          `json:"read_only_root"`
	SeccompProfile   string        `json:"seccomp_profile"`
	AppArmorProfile  string        `json:"apparmor_profile"`
	RootNamespace    bool          `json:"root_namespace"`
}

// Anomaly represents a detected runtime anomaly
type Anomaly struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	DetectedAt  time.Time `json:"detected_at"`
	Process     string    `json:"process,omitempty"`
	Details     string    `json:"details,omitempty"`
}

// Remediation represents an auto-fix suggestion
type Remediation struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	AutoFixable bool   `json:"auto_fixable"`
}

// ResourceLimits represents container resource constraints
type ResourceLimits struct {
	CPUQuota    int64 `json:"cpu_quota"`
	MemoryLimit int64 `json:"memory_limit"`
	PidsLimit   int64 `json:"pids_limit"`
	IOReadBPS   int64 `json:"io_read_bps"`
	IOWriteBPS  int64 `json:"io_write_bps"`
}

// ScanRequest is the request body for starting a scan
type ScanRequest struct {
	Image            string `json:"image" binding:"required"`
	Registry         string `json:"registry,omitempty"`
	ForceRescan      bool   `json:"force_rescan"`
	SkipSignature    bool   `json:"skip_signature"`
	SkipCompliance   bool   `json:"skip_compliance"`
	SkipSensitive    bool   `json:"skip_sensitive"`
	IncludeRemediation bool `json:"include_remediation"`
}

// ScanResponse is the response for a scan request
type ScanResponse struct {
	ScanID string     `json:"scan_id"`
	Image  string     `json:"image"`
	Status ScanStatus `json:"status"`
}

// ReportRequest is the request body for generating a report
type ReportRequest struct {
	Format ReportFormat `json:"format"`
}

// ReportResponse contains generated report data
type ReportResponse struct {
	ReportID string       `json:"report_id"`
	Format   ReportFormat `json:"format"`
	Size     int          `json:"size"`
}

// SecurityScore represents the computed security score with details
type SecurityScore struct {
	Overall    float64       `json:"overall"`
	Grade      SecurityGrade `json:"grade"`
	VulnScore  float64       `json:"vuln_score"`
	CompScore  float64       `json:"comp_score"`
	RuntimeScore float64     `json:"runtime_score"`
	SensitiveScore float64   `json:"sensitive_score"`
	Breakdown  ScoreBreakdown `json:"breakdown"`
}

// ScoreBreakdown provides detailed score components
type ScoreBreakdown struct {
	VulnDeductions      float64 `json:"vuln_deductions"`
	ComplianceDeductions float64 `json:"compliance_deductions"`
	RuntimeDeductions   float64 `json:"runtime_deductions"`
	SensitiveDeductions float64 `json:"sensitive_deductions"`
	SignatureBonus      float64 `json:"signature_bonus"`
	RemediationBonus    float64 `json:"remediation_bonus"`
}

// SecurityPolicy 安全策略（用于 ContainerGuardian 独立运行模式）
type SecurityPolicy struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	MaxSeverity    string          `json:"max_severity"`
	EnforceLimits  bool            `json:"enforce_limits"`
	RequireScan    bool            `json:"require_scan"`
	AutoRemediate  bool            `json:"auto_remediate"`
	ResourceLimits *ResourceLimits `json:"resource_limits"`
	IsActive       bool            `json:"is_active"`
}

// NetworkPolicy 网络隔离策略（用于 ContainerGuardian 独立运行模式）
type NetworkPolicy struct {
	Name         string   `json:"name"`
	AllowIngress bool     `json:"allow_ingress"`
	AllowEgress  bool     `json:"allow_egress"`
	AllowedPorts []int    `json:"allowed_ports"`
	BlockedPorts []int    `json:"blocked_ports"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
	IsActive     bool     `json:"is_active"`
}
