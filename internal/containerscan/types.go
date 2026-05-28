// Package containerscan provides Docker image vulnerability scanning with CVE detection,
// layer analysis, severity rating, auto-fix suggestions, scheduled scanning, and report generation.
package containerscan

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

// ReportFormat defines output format for scan reports
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatPDF  ReportFormat = "pdf"
)

// ScanStatus represents the status of a scan job
type ScanStatus string

const (
	StatusQueued    ScanStatus = "queued"
	StatusScanning  ScanStatus = "scanning"
	StatusCompleted ScanStatus = "completed"
	StatusFailed    ScanStatus = "failed"
)

// CVE represents a Common Vulnerabilities and Exposures entry
type CVE struct {
	ID          string    `json:"id"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixedIn     string    `json:"fixed_in"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// Vulnerability represents a vulnerability found in an image
type Vulnerability struct {
	CVE
	Layer        string `json:"layer"`
	InstalledPkg string `json:"installed_pkg"`
	Path         string `json:"path"`
}

// FixSuggestion provides auto-fix recommendation for a vulnerability
type FixSuggestion struct {
	VulnID      string `json:"vuln_id"`
	Package     string `json:"package"`
	CurrentVer  string `json:"current_version"`
	FixedVer    string `json:"fixed_version"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ImageLayer represents a single layer in a container image
type ImageLayer struct {
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Command   string    `json:"command"`
	Vulns     int       `json:"vulns"`
}

// VulnSummary provides vulnerability count by severity
type VulnSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

// ScanResult represents the result of scanning an image
type ScanResult struct {
	Image        string          `json:"image"`
	Registry     string          `json:"registry"`
	Digest       string          `json:"digest"`
	ScanTime     time.Time       `json:"scan_time"`
	Duration     time.Duration   `json:"duration"`
	Vulns        []Vulnerability `json:"vulnerabilities"`
	Layers       []ImageLayer    `json:"layers"`
	Summary      VulnSummary     `json:"summary"`
	FixSuggestions []FixSuggestion `json:"fix_suggestions,omitempty"`
	Compliant    bool            `json:"compliant"`
}

// ScanRequest is the API request for scanning an image
type ScanRequest struct {
	Image       string `json:"image"`
	Registry    string `json:"registry"`
	ForceRescan bool   `json:"force_rescan"`
}

// ScanResponse is the API response for a scan
type ScanResponse struct {
	ScanID string     `json:"scan_id"`
	Image  string     `json:"image"`
	Status ScanStatus `json:"status"`
	Result *ScanResult `json:"result,omitempty"`
}

// ScanSchedule defines a scheduled scan configuration
type ScanSchedule struct {
	ID        string        `json:"id"`
	Image     string        `json:"image"`
	Interval  time.Duration `json:"interval"`
	Enabled   bool          `json:"enabled"`
	LastRun   time.Time     `json:"last_run"`
	NextRun   time.Time     `json:"next_run"`
	CreatedAt time.Time     `json:"created_at"`
}

// ImageListEntry represents an image in the whitelist/blacklist
type ImageListEntry struct {
	Image     string    `json:"image"`
	Reason    string    `json:"reason"`
	AddedAt   time.Time `json:"added_at"`
	AddedBy   string    `json:"added_by"`
}

// ScanReport represents a generated scan report
type ScanReport struct {
	ID        string      `json:"id"`
	Image     string      `json:"image"`
	Format    ReportFormat `json:"format"`
	Result    *ScanResult `json:"result"`
	GeneratedAt time.Time `json:"generated_at"`
	Content   []byte      `json:"content,omitempty"`
}

// ListType defines whitelist or blacklist
type ListType string

const (
	ListTypeWhite ListType = "whitelist"
	ListTypeBlack ListType = "blacklist"
)
