// Package containerguardian provides container security scanning, runtime monitoring,
// image signature verification, CIS Docker Benchmark compliance, sensitive data leak
// detection, security grading (A/B/C/D/F), auto-remediation, and security report generation.
package containerguardian

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Guardian is the main container security guardian that orchestrates all security operations.
type Guardian struct {
	logger    *zap.Logger
	results   map[string]*ScanResult
	resultMu  sync.RWMutex
	reports   map[string][]byte
	reportMu  sync.RWMutex
	auditLog  []AuditEntry
	auditMu   sync.RWMutex
	vulnDB    map[string][]Vulnerability
	vulnMu    sync.RWMutex
	stopCh    chan struct{}
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	ContainerID string    `json:"container_id"`
	Image       string    `json:"image"`
	Action      string    `json:"action"`
	Details     string    `json:"details"`
	Severity    string    `json:"severity"`
	Success     bool      `json:"success"`
}

// NewGuardian creates a new Guardian instance with zap logger
func NewGuardian(logger *zap.Logger) *Guardian {
	if logger == nil {
		logger = zap.NewNop()
	}
	g := &Guardian{
		logger:   logger,
		results:  make(map[string]*ScanResult),
		reports:  make(map[string][]byte),
		auditLog: make([]AuditEntry, 0),
		vulnDB:   make(map[string][]Vulnerability),
		stopCh:   make(chan struct{}),
	}
	g.initVulnDB()
	return g
}

// Start begins the guardian's background workers
func (g *Guardian) Start(ctx context.Context) {
	g.logger.Info("container guardian started")
}

// Stop stops the guardian
func (g *Guardian) Stop() {
	close(g.stopCh)
	g.logger.Info("container guardian stopped")
}

// ScanImage performs a full security scan on a container image
func (g *Guardian) ScanImage(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	if req.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	scanID := fmt.Sprintf("guardian-%s-%d", req.Image, time.Now().UnixNano())
	startTime := time.Now()

	result := &ScanResult{
		ID:        scanID,
		Image:     req.Image,
		Status:    ScanStatusRunning,
		ScannedAt: startTime,
	}

	// Store intermediate result
	g.resultMu.Lock()
	g.results[scanID] = result
	g.resultMu.Unlock()

	// 1. CVE vulnerability scan
	vulns, err := g.scanVulnerabilities(ctx, req.Image)
	if err != nil {
		g.logger.Error("vulnerability scan failed", zap.String("image", req.Image), zap.Error(err))
	}
	result.Vulnerabilities = vulns

	// 2. CIS Docker Benchmark compliance checks
	if !req.SkipCompliance {
		compliance := g.runComplianceChecks(ctx, req.Image)
		result.Compliance = compliance
	}

	// 3. Image signature verification
	if !req.SkipSignature {
		sigResult := g.verifySignature(ctx, req.Image)
		result.Signature = sigResult
	}

	// 4. Sensitive data leak detection
	if !req.SkipSensitive {
		sensitive := g.detectSensitiveData(ctx, req.Image)
		result.Sensitive = sensitive
	}

	// 5. Generate remediations
	if req.IncludeRemediation {
		result.Remediations = g.generateRemediations(result)
	} else {
		result.Remediations = g.generateRemediations(result)
	}

	// 6. Calculate score and grade
	score := g.CalculateScore(result)
	result.Score = score.Overall
	result.Grade = score.Grade

	// 7. Build summary
	result.Summary = g.calculateSummary(result)
	result.Status = ScanStatusCompleted
	result.Duration = time.Since(startTime)

	// Update stored result
	g.resultMu.Lock()
	g.results[scanID] = result
	g.resultMu.Unlock()

	// Audit log
	g.addAuditEntry("", req.Image, "ScanImage",
		fmt.Sprintf("scanned image %s: vulns=%d grade=%s score=%.1f duration=%v",
			req.Image, result.Summary.Total, result.Grade, result.Score, result.Duration),
		"INFO", true)

	g.logger.Info("scan completed",
		zap.String("scan_id", scanID),
		zap.String("image", req.Image),
		zap.Int("vulns", result.Summary.Total),
		zap.String("grade", string(result.Grade)),
		zap.Float64("score", result.Score),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// GetScanResult returns a scan result by ID
func (g *Guardian) GetScanResult(scanID string) *ScanResult {
	g.resultMu.RLock()
	defer g.resultMu.RUnlock()
	return g.results[scanID]
}

// ListScanResults returns all scan results
func (g *Guardian) ListScanResults() []*ScanResult {
	g.resultMu.RLock()
	defer g.resultMu.RUnlock()

	results := make([]*ScanResult, 0, len(g.results))
	for _, r := range g.results {
		results = append(results, r)
	}
	return results
}

// DeleteScanResult deletes a scan result by ID
func (g *Guardian) DeleteScanResult(scanID string) bool {
	g.resultMu.Lock()
	defer g.resultMu.Unlock()

	if _, exists := g.results[scanID]; !exists {
		return false
	}
	delete(g.results, scanID)
	return true
}

// GetSecurityScore returns the detailed security score for a scan
func (g *Guardian) GetSecurityScore(scanID string) (*SecurityScore, error) {
	g.resultMu.RLock()
	result, exists := g.results[scanID]
	g.resultMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}

	return g.CalculateScore(result), nil
}

// GetAuditLog returns audit log entries, optionally filtered by image
func (g *Guardian) GetAuditLog(image string) []AuditEntry {
	g.auditMu.RLock()
	defer g.auditMu.RUnlock()

	if image == "" {
		result := make([]AuditEntry, len(g.auditLog))
		copy(result, g.auditLog)
		return result
	}

	filtered := make([]AuditEntry, 0)
	for _, entry := range g.auditLog {
		if entry.Image == image {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// AddVulnerability adds a custom vulnerability to the database
func (g *Guardian) AddVulnerability(image string, vuln Vulnerability) {
	g.vulnMu.Lock()
	defer g.vulnMu.Unlock()
	g.vulnDB[image] = append(g.vulnDB[image], vuln)
}

// GetVulnerabilities returns vulnerabilities for a scan result, optionally filtered by severity
func (g *Guardian) GetVulnerabilities(scanID string, minSeverity string) ([]Vulnerability, error) {
	g.resultMu.RLock()
	result, exists := g.results[scanID]
	g.resultMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}

	if minSeverity == "" {
		return result.Vulnerabilities, nil
	}

	filtered := make([]Vulnerability, 0)
	for _, v := range result.Vulnerabilities {
		if severityRank(v.Severity) >= severityRank(minSeverity) {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
}

// addAuditEntry adds an audit log entry (caller must hold appropriate lock or call in safe context)
func (g *Guardian) addAuditEntry(containerID, image, action, details, severity string, success bool) {
	g.auditMu.Lock()
	defer g.auditMu.Unlock()

	entry := AuditEntry{
		Timestamp:   time.Now(),
		ContainerID: containerID,
		Image:       image,
		Action:      action,
		Details:     details,
		Severity:    severity,
		Success:     success,
	}
	g.auditLog = append(g.auditLog, entry)
}

// severityRank returns numeric rank for severity comparison
func severityRank(severity string) int {
	switch severity {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
