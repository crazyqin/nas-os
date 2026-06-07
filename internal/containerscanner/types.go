// Package containerscanner provides container image security scanning with vulnerability
// detection, compliance checking, scheduled scans, and security report generation.
package containerscanner

import (
	"time"
)

// Severity levels for vulnerabilities
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityInfo     = "INFO"
)

// ScanStatus represents the status of a scan
type ScanStatus string

const (
	ScanStatusQueued    ScanStatus = "queued"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ComplianceStatus represents compliance check result
type ComplianceStatus string

const (
	ComplianceStatusPass ComplianceStatus = "pass"
	ComplianceStatusFail ComplianceStatus = "fail"
	ComplianceStatusWarn ComplianceStatus = "warn"
	ComplianceStatusSkip ComplianceStatus = "skip"
)

// ReportFormat defines output format for reports
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatHTML ReportFormat = "html"
	ReportFormatPDF  ReportFormat = "pdf"
)

// ScanResult represents the complete result of an image scan
type ScanResult struct {
	ID              string           `json:"id"`
	Image           string           `json:"image"`
	Registry        string           `json:"registry"`
	Tag             string           `json:"tag"`
	Digest          string           `json:"digest"`
	Status          ScanStatus       `json:"status"`
	Summary         VulnSummary      `json:"summary"`
	Vulnerabilities []Vulnerability  `json:"vulnerabilities"`
	Compliance      []ComplianceRule `json:"compliance"`
	ScannedAt       time.Time        `json:"scanned_at"`
	Duration        time.Duration    `json:"duration"`
	Error           string           `json:"error,omitempty"`
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
	Layer       string    `json:"layer,omitempty"`
	URL         string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// ComplianceRule represents a compliance check rule
type ComplianceRule struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Status      ComplianceStatus `json:"status"`
	Message     string           `json:"message,omitempty"`
	Severity    string           `json:"severity"`
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

// ScanPolicy defines scanning behavior
type ScanPolicy struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	MaxSeverity      string        `json:"max_severity"` // fail if vuln above this
	IgnoreCVEs       []string      `json:"ignore_cves"`
	RequireFix       bool          `json:"require_fix"` // only flag vulns with fixes
	ScanLayers       bool          `json:"scan_layers"`
	ComplianceChecks bool          `json:"compliance_checks"`
	Timeout          time.Duration `json:"timeout"`
	CreatedAt        time.Time     `json:"created_at"`
}

// ScanSchedule defines a scheduled scan
type ScanSchedule struct {
	ID        string        `json:"id"`
	Image     string        `json:"image"`
	Interval  time.Duration `json:"interval"`
	PolicyID  string        `json:"policy_id,omitempty"`
	Enabled   bool          `json:"enabled"`
	LastRun   *time.Time    `json:"last_run,omitempty"`
	NextRun   time.Time     `json:"next_run"`
	CreatedAt time.Time     `json:"created_at"`
}

// ScanRequest is the request body for starting a scan
type ScanRequest struct {
	Image       string `json:"image" binding:"required"`
	Registry    string `json:"registry,omitempty"`
	PolicyID    string `json:"policy_id,omitempty"`
	ForceRescan bool   `json:"force_rescan"`
}

// ScanResponse is the response for a scan request
type ScanResponse struct {
	ScanID string     `json:"scan_id"`
	Image  string     `json:"image"`
	Status ScanStatus `json:"status"`
}

// GenerateReportRequest is the request body for generating a report
type GenerateReportRequest struct {
	Image  string       `json:"image" binding:"required"`
	Format ReportFormat `json:"format"`
}
