package compliancechecker

import (
	"crypto/aes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Severity levels for compliance issues
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// IssueType categorizes compliance issues
type IssueType string

const (
	IssueGDPR           IssueType = "gdpr"
	IssueGB20           IssueType = "gb20"
	IssueFilePermission IssueType = "file_permission"
	IssueSensitiveData  IssueType = "sensitive_data"
	IssueEncryption     IssueType = "encryption"
)

// Issue represents a compliance violation
type Issue struct {
	ID          string    `json:"id"`
	Type        IssueType `json:"type"`
	Severity    Severity  `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Path        string    `json:"path,omitempty"`
	Details     string    `json:"details,omitempty"`
	Fix         string    `json:"fix,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ComplianceScore represents overall compliance status
type ComplianceScore struct {
	Overall   float64            `json:"overall"`
	Level     string             `json:"level"`
	GDPR      float64            `json:"gdpr"`
	GB20      float64            `json:"gb20"`
	Security  float64            `json:"security"`
	Breakdown map[string]float64 `json:"breakdown"`
	Issues    int                `json:"issues"`
	Timestamp time.Time          `json:"timestamp"`
}

// ComplianceReport is the full scan result
type ComplianceReport struct {
	Summary    ComplianceScore `json:"summary"`
	Issues     []Issue         `json:"issues"`
	ScanTime   time.Time       `json:"scan_time"`
	Duration   time.Duration   `json:"duration"`
	PathsCount int             `json:"paths_count"`
}

// ScanConfig configures what to scan
type ScanConfig struct {
	Paths              []string `json:"paths"`
	ScanGDPR           bool     `json:"scan_gdpr"`
	ScanGB20           bool     `json:"scan_gb20"`
	ScanFilePermission bool     `json:"scan_file_permission"`
	ScanSensitiveData  bool     `json:"scan_sensitive_data"`
	ScanEncryption     bool     `json:"scan_encryption"`
	MaxFileSize        int64    `json:"max_file_size"`
	IgnorePatterns     []string `json:"ignore_patterns"`
}

// DefaultScanConfig returns config with all scans enabled
func DefaultScanConfig(paths []string) ScanConfig {
	return ScanConfig{
		Paths:              paths,
		ScanGDPR:           true,
		ScanGB20:           true,
		ScanFilePermission: true,
		ScanSensitiveData:  true,
		ScanEncryption:     true,
		MaxFileSize:        10 * 1024 * 1024, // 10MB
		IgnorePatterns:     []string{".git", "node_modules", ".DS_Store"},
	}
}

// Checker performs compliance checks
type Checker struct {
	config   ScanConfig
	issues   []Issue
	mu       sync.Mutex
	nextID   int
	issueMap map[string]bool // dedup
}

// NewChecker creates a new compliance checker
func NewChecker(config ScanConfig) *Checker {
	return &Checker{
		config:   config,
		issues:   make([]Issue, 0),
		issueMap: make(map[string]bool),
	}
}

func (c *Checker) nextIssueID() string {
	c.nextID++
	return fmt.Sprintf("issue-%04d", c.nextID)
}

func (c *Checker) addIssue(issue Issue) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Dedup by title+path
	key := issue.Title + "|" + issue.Path
	if c.issueMap[key] {
		return
	}
	c.issueMap[key] = true

	issue.ID = c.nextIssueID()
	issue.Timestamp = time.Now()
	c.issues = append(c.issues, issue)
}

// RunScan performs all configured compliance checks
func (c *Checker) RunScan() (*ComplianceReport, error) {
	start := time.Now()

	for _, path := range c.config.Paths {
		if err := c.scanPath(path); err != nil {
			return nil, fmt.Errorf("scan path %s: %w", path, err)
		}
	}

	duration := time.Since(start)
	score := c.calculateScore()

	return &ComplianceReport{
		Summary:    score,
		Issues:     c.issues,
		ScanTime:   start,
		Duration:   duration,
		PathsCount: len(c.config.Paths),
	}, nil
}

func (c *Checker) scanPath(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}

		// Check ignore patterns
		for _, pattern := range c.config.IgnorePatterns {
			if strings.Contains(path, pattern) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// File permission check
		if c.config.ScanFilePermission {
			c.checkFilePermission(path, info)
		}

		// Skip large files or dirs for content scans
		if info.IsDir() {
			return nil
		}
		if c.config.MaxFileSize > 0 && info.Size() > c.config.MaxFileSize {
			return nil
		}

		// Sensitive data detection
		if c.config.ScanSensitiveData {
			c.scanFileForSensitiveData(path)
		}

		// Encryption check
		if c.config.ScanEncryption {
			c.checkEncryption(path)
		}

		return nil
	})
}

// checkFilePermission detects overly permissive file permissions
func (c *Checker) checkFilePermission(path string, info os.FileInfo) {
	mode := info.Mode()

	// World-writable
	if mode&0002 != 0 {
		c.addIssue(Issue{
			Type:        IssueFilePermission,
			Severity:    SeverityHigh,
			Title:       "World-writable file permission",
			Description: fmt.Sprintf("File %s has world-writable permission (%s)", path, mode),
			Path:        path,
			Fix:         fmt.Sprintf("chmod o-w %s", path),
		})
	}

	// World-readable sensitive files
	if mode&0004 != 0 && isSensitiveFile(path) {
		c.addIssue(Issue{
			Type:        IssueFilePermission,
			Severity:    SeverityMedium,
			Title:       "World-readable sensitive file",
			Description: fmt.Sprintf("Sensitive file %s is world-readable", path),
			Path:        path,
			Fix:         fmt.Sprintf("chmod o-r %s", path),
		})
	}

	// Group-writable sensitive files
	if mode&0020 != 0 && isSensitiveFile(path) {
		c.addIssue(Issue{
			Type:        IssueFilePermission,
			Severity:    SeverityMedium,
			Title:       "Group-writable sensitive file",
			Description: fmt.Sprintf("Sensitive file %s is group-writable", path),
			Path:        path,
			Fix:         fmt.Sprintf("chmod g-w %s", path),
		})
	}

	// Executable data files
	ext := strings.ToLower(filepath.Ext(path))
	dataExts := map[string]bool{
		".csv": true, ".json": true, ".xml": true, ".txt": true,
		".log": true, ".md": true, ".yaml": true, ".yml": true,
	}
	if dataExts[ext] && mode&0111 != 0 {
		c.addIssue(Issue{
			Type:        IssueFilePermission,
			Severity:    SeverityLow,
			Title:       "Executable bit set on data file",
			Description: fmt.Sprintf("Data file %s has executable permission", path),
			Path:        path,
			Fix:         fmt.Sprintf("chmod -x %s", path),
		})
	}
}

func isSensitiveFile(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	sensitive := []string{
		"password", "secret", "key", "credential", "token",
		".env", ".pem", ".key", ".p12", ".pfx", ".keystore",
		"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	}
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// Sensitive data patterns
var (
	// Chinese ID card (18 digits)
	idCardPattern = regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	// Chinese mobile phone
	phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	// Email
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`)
	// Bank card (16-19 digits)
	bankCardPattern = regexp.MustCompile(`\b[1-9]\d{15,18}\b`)
	// SSN-like pattern (US)
	ssnPattern = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	// IP address (v4)
	ipv4Pattern = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`)
)

// scanFileForSensitiveData checks file content for sensitive patterns
func (c *Checker) scanFileForSensitiveData(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	content := string(data)

	// Check ID card
	if matches := idCardPattern.FindAllString(content, -1); len(matches) > 0 {
		c.addIssue(Issue{
			Type:        IssueSensitiveData,
			Severity:    SeverityCritical,
			Title:       "Chinese ID card number detected",
			Description: fmt.Sprintf("Found %d ID card number(s) in %s", len(matches), path),
			Path:        path,
			Details:     fmt.Sprintf("Count: %d", len(matches)),
			Fix:         "Remove or encrypt sensitive personal data",
		})
	}

	// Check phone
	if matches := phonePattern.FindAllString(content, -1); len(matches) > 0 {
		c.addIssue(Issue{
			Type:        IssueSensitiveData,
			Severity:    SeverityHigh,
			Title:       "Phone number detected",
			Description: fmt.Sprintf("Found %d phone number(s) in %s", len(matches), path),
			Path:        path,
			Details:     fmt.Sprintf("Count: %d", len(matches)),
			Fix:         "Remove or mask phone numbers",
		})
	}

	// Check email
	if matches := emailPattern.FindAllString(content, -1); len(matches) > 0 {
		c.addIssue(Issue{
			Type:        IssueSensitiveData,
			Severity:    SeverityMedium,
			Title:       "Email address detected",
			Description: fmt.Sprintf("Found %d email address(es) in %s", len(matches), path),
			Path:        path,
			Details:     fmt.Sprintf("Count: %d", len(matches)),
			Fix:         "Remove or mask email addresses",
		})
	}

	// Check SSN
	if matches := ssnPattern.FindAllString(content, -1); len(matches) > 0 {
		c.addIssue(Issue{
			Type:        IssueSensitiveData,
			Severity:    SeverityCritical,
			Title:       "SSN-like pattern detected",
			Description: fmt.Sprintf("Found %d SSN-like pattern(s) in %s", len(matches), path),
			Path:        path,
			Details:     fmt.Sprintf("Count: %d", len(matches)),
			Fix:         "Remove or encrypt sensitive data",
		})
	}
}

// checkEncryption checks if file is encrypted
func (c *Checker) checkEncryption(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	encryptedExts := map[string]bool{
		".gpg": true, ".pgp": true, ".enc": true, ".aes": true,
		".crypt": true, ".encrypted": true, ".p12": true, ".pfx": true,
	}

	// Sensitive files should be encrypted
	if isSensitiveFile(path) && !encryptedExts[ext] {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}

		// Check if it's PEM-encoded (not encrypted)
		if strings.Contains(string(data), "-----BEGIN") {
			if !strings.Contains(string(data), "ENCRYPTED") {
				c.addIssue(Issue{
					Type:        IssueEncryption,
					Severity:    SeverityHigh,
					Title:       "Unencrypted sensitive key file",
					Description: fmt.Sprintf("Sensitive file %s is not encrypted", path),
					Path:        path,
					Fix:         "Encrypt this file using GPG or similar encryption",
				})
			}
		}
	}
}

// calculateScore computes compliance score from issues
func (c *Checker) calculateScore() ComplianceScore {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.issues) == 0 {
		return ComplianceScore{
			Overall:   100.0,
			Level:     "A",
			GDPR:      100.0,
			GB20:      100.0,
			Security:  100.0,
			Breakdown: map[string]float64{},
			Issues:    0,
			Timestamp: time.Now(),
		}
	}

	deductions := map[string]float64{
		"gdpr":            0,
		"gb20":            0,
		"file_permission": 0,
		"sensitive_data":  0,
		"encryption":      0,
	}

	severityWeight := map[Severity]float64{
		SeverityCritical: 25,
		SeverityHigh:     15,
		SeverityMedium:   8,
		SeverityLow:      3,
		SeverityInfo:     1,
	}

	for _, issue := range c.issues {
		weight := severityWeight[issue.Severity]
		switch issue.Type {
		case IssueGDPR:
			deductions["gdpr"] += weight
		case IssueGB20:
			deductions["gb20"] += weight
		case IssueFilePermission:
			deductions["file_permission"] += weight
		case IssueSensitiveData:
			deductions["sensitive_data"] += weight
		case IssueEncryption:
			deductions["encryption"] += weight
		}
	}

	// Calculate component scores
	gdprScore := 100.0 - deductions["gdpr"]
	if gdprScore < 0 {
		gdprScore = 0
	}

	gb20Score := 100.0 - deductions["gb20"]
	if gb20Score < 0 {
		gb20Score = 0
	}

	securityScore := 100.0 - deductions["file_permission"] - deductions["sensitive_data"] - deductions["encryption"]
	if securityScore < 0 {
		securityScore = 0
	}

	overall := (gdprScore + gb20Score + securityScore) / 3

	level := "A"
	switch {
	case overall >= 90:
		level = "A"
	case overall >= 80:
		level = "B"
	case overall >= 70:
		level = "C"
	case overall >= 60:
		level = "D"
	default:
		level = "F"
	}

	return ComplianceScore{
		Overall:   overall,
		Level:     level,
		GDPR:      gdprScore,
		GB20:      gb20Score,
		Security:  securityScore,
		Breakdown: deductions,
		Issues:    len(c.issues),
		Timestamp: time.Now(),
	}
}

// GetIssues returns all found issues
func (c *Checker) GetIssues() []Issue {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.issues
}

// GetScore returns current compliance score
func (c *Checker) GetScore() ComplianceScore {
	return c.calculateScore()
}

// AddIssue manually adds an issue (for testing or external checks)
func (c *Checker) AddIssue(issue Issue) {
	c.addIssue(issue)
}

// FixSuggestion returns fix suggestions for an issue
func FixSuggestion(issue Issue) string {
	if issue.Fix != "" {
		return issue.Fix
	}

	switch issue.Type {
	case IssueFilePermission:
		return "Review and restrict file permissions: chmod 600 <file>"
	case IssueSensitiveData:
		return "Remove or encrypt sensitive data before storage"
	case IssueEncryption:
		return "Encrypt sensitive files using GPG: gpg -c <file>"
	case IssueGDPR:
		return "Implement data retention policies and user data deletion mechanisms"
	case IssueGB20:
		return "Implement access controls, audit logging, and regular backups per GB/T 22239-2019"
	}
	return "Review and fix the compliance issue"
}

// MarshalReport serializes report to JSON
func MarshalReport(report *ComplianceReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// UnmarshalReport deserializes report from JSON
func UnmarshalReport(data []byte) (*ComplianceReport, error) {
	var report ComplianceReport
	err := json.Unmarshal(data, &report)
	return &report, err
}

// AES key length validation
func ValidateAESKeyLength(key []byte) bool {
	return len(key) == 16 || len(key) == 24 || len(key) == 32
}

// CheckAESBlockCipher checks if data could be valid AES ciphertext
func CheckAESBlockCipher(data []byte) bool {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return false
	}
	return true
}
