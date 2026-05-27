// Package containersec provides container image vulnerability scanning and security policy enforcement.
package containersec

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

// CVE represents a Common Vulnerabilities and Exposures entry
type CVE struct {
	ID          string    `json:"id"`           // e.g. CVE-2024-1234
	Severity    string    `json:"severity"`     // CRITICAL, HIGH, MEDIUM, LOW, INFO
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Package     string    `json:"package"`      // affected package name
	Version     string    `json:"version"`      // affected version
	FixedIn     string    `json:"fixed_in"`     // fix version, empty if not available
	URL         string    `json:"url"`          // reference URL
	PublishedAt time.Time `json:"published_at"`
}

// Vulnerability represents a vulnerability found in an image
type Vulnerability struct {
	CVE
	Layer        string `json:"layer"`         // image layer digest
	InstalledPkg string `json:"installed_pkg"` // installed package
	Path         string `json:"path"`          // file path in image
}

// ImageLayer represents a single layer in a container image
type ImageLayer struct {
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Command   string    `json:"command"` // Dockerfile command
	Vulns     int       `json:"vulns"`   // vulnerability count in this layer
}

// ScanResult represents the result of scanning an image
type ScanResult struct {
	Image          string           `json:"image"`           // image name:tag
	Registry       string           `json:"registry"`        // registry URL
	Digest         string           `json:"digest"`          // image digest
	ScanTime      time.Time        `json:"scan_time"`
	Duration       time.Duration    `json:"duration"`
	Vulns          []Vulnerability  `json:"vulnerabilities"`
	Layers         []ImageLayer     `json:"layers"`
	Summary        VulnSummary      `json:"summary"`
	Compliant      bool             `json:"compliant"`
	PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`
	BenchmarkScore *BenchmarkScore  `json:"benchmark_score,omitempty"`
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

// SecurityPolicy defines rules for image security
type SecurityPolicy struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Enabled       bool           `json:"enabled"`
	MaxCritical   int            `json:"max_critical"`    // max allowed critical vulns, 0 = block all
	MaxHigh       int            `json:"max_high"`        // max allowed high vulns
	MaxTotal      int            `json:"max_total"`       // max total vulns
	BlockedPackages []string     `json:"blocked_packages"` // packages to block
	RequiredLabels []string      `json:"required_labels"` // required image labels
	AllowedRegistries []string   `json:"allowed_registries"` // allowed registries, empty = all
	MaxImageAge   time.Duration  `json:"max_image_age"`   // max age of base image
	RequireNonRoot bool          `json:"require_non_root"` // require non-root user
	BenchmarkMinScore float64    `json:"benchmark_min_score"` // minimum CIS benchmark score (0-100)
}

// DefaultSecurityPolicy returns a sensible default policy
func DefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{
		ID:              "default",
		Name:            "Default Security Policy",
		Enabled:         true,
		MaxCritical:     0,
		MaxHigh:         5,
		MaxTotal:        50,
		BlockedPackages: []string{},
		RequiredLabels:  []string{},
		AllowedRegistries: []string{},
		MaxImageAge:     90 * 24 * time.Hour, // 90 days
		RequireNonRoot:  true,
		BenchmarkMinScore: 60.0,
	}
}

// PolicyViolation represents a policy violation
type PolicyViolation struct {
	PolicyID    string `json:"policy_id"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Severity    string `json:"severity"` // blocking, warning
}

// ImageReport is a summary report for an image
type ImageReport struct {
	Image       string        `json:"image"`
	ScanResult  *ScanResult   `json:"scan_result"`
	Policies    []string      `json:"policies_applied"`
	Approved    bool          `json:"approved"`
	ScannedAt   time.Time     `json:"scanned_at"`
	ExpiresAt   time.Time     `json:"expires_at"` // when to re-scan
}

// BenchmarkCheck represents a single CIS Docker Benchmark check
type BenchmarkCheck struct {
	ID       string `json:"id"`       // e.g. "CIS-1.1"
	Title    string `json:"title"`
	Status   string `json:"status"`   // PASS, FAIL, WARN, INFO
	Score    float64 `json:"score"`   // 0-100
	Detail   string `json:"detail"`
}

// BenchmarkScore represents overall CIS Docker Benchmark score
type BenchmarkScore struct {
	TotalScore float64          `json:"total_score"` // 0-100
	Checks     []BenchmarkCheck `json:"checks"`
	Level      string           `json:"level"` // Level 1, Level 2
}

// ScanRequest is the API request for scanning an image
type ScanRequest struct {
	Image    string `json:"image" binding:"required"`
	Registry string `json:"registry"`
	Tag      string `json:"tag"`
	ForceRescan bool `json:"force_rescan"` // ignore cache
}

// ScanResponse is the API response for a scan
type ScanResponse struct {
	ScanID   string     `json:"scan_id"`
	Image    string     `json:"image"`
	Status   string     `json:"status"` // queued, scanning, completed, failed
	Result   *ScanResult `json:"result,omitempty"`
}
