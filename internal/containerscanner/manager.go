// Package containerscanner provides container image security scanning with vulnerability
// detection, compliance checking, scheduled scans, and security report generation.
package containerscanner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages container security scanning operations
type Manager struct {
	logger    *zap.Logger
	results   map[string]*ScanResult
	resultMu  sync.RWMutex
	schedules map[string]*ScanSchedule
	schedMu   sync.RWMutex
	policies  map[string]*ScanPolicy
	policyMu  sync.RWMutex
	reports   map[string][]byte
	reportMu  sync.RWMutex
	stopCh    chan struct{}
}

// NewManager creates a new container security scanner manager
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:    logger,
		results:   make(map[string]*ScanResult),
		schedules: make(map[string]*ScanSchedule),
		policies:  make(map[string]*ScanPolicy),
		reports:   make(map[string][]byte),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the scheduled scan runner
func (m *Manager) Start(ctx context.Context) {
	go m.runScheduler(ctx)
	m.logger.Info("container security scanner started")
}

// Stop stops the scheduled scan runner
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("container security scanner stopped")
}

// ScanImage scans a container image for vulnerabilities
func (m *Manager) ScanImage(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	if req.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	scanID := fmt.Sprintf("scan-%s-%d", req.Image, time.Now().UnixNano())
	startTime := time.Now()

	result := &ScanResult{
		ID:        scanID,
		Image:     req.Image,
		Registry:  req.Registry,
		Status:    ScanStatusRunning,
		ScannedAt: startTime,
	}

	// Store intermediate result
	m.resultMu.Lock()
	m.results[scanID] = result
	m.resultMu.Unlock()

	// Simulate scanning
	go func() {
		time.Sleep(200 * time.Millisecond)

		// Simulate vulnerability detection
		result.Vulnerabilities = []Vulnerability{
			{
				ID:          "vuln-001",
				CVE:         "CVE-2024-0001",
				Severity:    SeverityHigh,
				Title:       "Buffer overflow in libssl",
				Description: "A buffer overflow vulnerability in OpenSSL",
				Package:     "libssl1.1",
				Version:     "1.1.1k-1",
				FixedIn:     "1.1.1l-1",
			},
			{
				ID:          "vuln-002",
				CVE:         "CVE-2024-0002",
				Severity:    SeverityMedium,
				Title:       "Cross-site scripting in curl",
				Description: "XSS vulnerability in curl output",
				Package:     "curl",
				Version:     "7.74.0-1",
				FixedIn:     "7.74.0-2",
			},
		}

		// Simulate compliance checks
		result.Compliance = []ComplianceRule{
			{
				ID:       "comp-001",
				Name:     "Run as non-root",
				Category: "security",
				Status:   ComplianceStatusPass,
				Severity: SeverityHigh,
			},
			{
				ID:       "comp-002",
				Name:     "No privileged mode",
				Category: "security",
				Status:   ComplianceStatusPass,
				Severity: SeverityCritical,
			},
			{
				ID:       "comp-003",
				Name:     "Read-only root filesystem",
				Category: "security",
				Status:   ComplianceStatusFail,
				Message:  "Root filesystem is writable",
				Severity: SeverityMedium,
			},
		}

		// Calculate summary
		result.Summary = m.calculateSummary(result)
		result.Status = ScanStatusCompleted
		result.Duration = time.Since(startTime)

		// Update stored result
		m.resultMu.Lock()
		m.results[scanID] = result
		m.resultMu.Unlock()

		m.logger.Info("scan completed",
			zap.String("scan_id", scanID),
			zap.String("image", req.Image),
			zap.Int("vulns", result.Summary.Total),
			zap.Duration("duration", result.Duration))
	}()

	return result, nil
}

// ScheduleScan creates a new scheduled scan
func (m *Manager) ScheduleScan(schedule *ScanSchedule) error {
	if schedule.ID == "" {
		return fmt.Errorf("schedule ID is required")
	}
	if schedule.Image == "" {
		return fmt.Errorf("image is required")
	}
	if schedule.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	m.schedMu.Lock()
	defer m.schedMu.Unlock()

	if _, exists := m.schedules[schedule.ID]; exists {
		return fmt.Errorf("schedule %s already exists", schedule.ID)
	}

	schedule.Enabled = true
	schedule.CreatedAt = time.Now()
	schedule.NextRun = time.Now().Add(schedule.Interval)
	m.schedules[schedule.ID] = schedule

	m.logger.Info("scan scheduled",
		zap.String("schedule_id", schedule.ID),
		zap.String("image", schedule.Image),
		zap.Duration("interval", schedule.Interval))

	return nil
}

// GetVulnerabilities returns vulnerabilities for a scan result
func (m *Manager) GetVulnerabilities(scanID string, minSeverity string) ([]Vulnerability, error) {
	m.resultMu.RLock()
	result, exists := m.results[scanID]
	m.resultMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}

	if minSeverity == "" {
		return result.Vulnerabilities, nil
	}

	filtered := make([]Vulnerability, 0)
	for _, v := range result.Vulnerabilities {
		if m.severityRank(v.Severity) >= m.severityRank(minSeverity) {
			filtered = append(filtered, v)
		}
	}

	return filtered, nil
}

// GenerateReport generates a security report for a scan
func (m *Manager) GenerateReport(scanID string, format ReportFormat) ([]byte, error) {
	m.resultMu.RLock()
	result, exists := m.results[scanID]
	m.resultMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scan %s not found", scanID)
	}

	var data []byte
	var err error

	switch format {
	case ReportFormatJSON:
		data, err = json.MarshalIndent(result, "", "  ")
	default:
		data, err = json.MarshalIndent(result, "", "  ")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	reportID := fmt.Sprintf("report-%s-%s", scanID, format)
	m.reportMu.Lock()
	m.reports[reportID] = data
	m.reportMu.Unlock()

	m.logger.Info("report generated",
		zap.String("report_id", reportID),
		zap.String("format", string(format)))

	return data, nil
}

// GetScanResult returns a scan result by ID
func (m *Manager) GetScanResult(scanID string) *ScanResult {
	m.resultMu.RLock()
	defer m.resultMu.RUnlock()
	return m.results[scanID]
}

// ListScanResults returns all scan results
func (m *Manager) ListScanResults() []*ScanResult {
	m.resultMu.RLock()
	defer m.resultMu.RUnlock()

	results := make([]*ScanResult, 0, len(m.results))
	for _, r := range m.results {
		results = append(results, r)
	}
	return results
}

// DeleteScanResult deletes a scan result
func (m *Manager) DeleteScanResult(scanID string) bool {
	m.resultMu.Lock()
	defer m.resultMu.Unlock()

	if _, exists := m.results[scanID]; !exists {
		return false
	}
	delete(m.results, scanID)
	return true
}

// GetSchedule returns a schedule by ID
func (m *Manager) GetSchedule(id string) *ScanSchedule {
	m.schedMu.RLock()
	defer m.schedMu.RUnlock()
	return m.schedules[id]
}

// ListSchedules returns all schedules
func (m *Manager) ListSchedules() []*ScanSchedule {
	m.schedMu.RLock()
	defer m.schedMu.RUnlock()

	schedules := make([]*ScanSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// DeleteSchedule deletes a schedule
func (m *Manager) DeleteSchedule(id string) bool {
	m.schedMu.Lock()
	defer m.schedMu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return false
	}
	delete(m.schedules, id)
	return true
}

// AddPolicy adds a scan policy
func (m *Manager) AddPolicy(policy *ScanPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	m.policyMu.Lock()
	defer m.policyMu.Unlock()

	if _, exists := m.policies[policy.ID]; exists {
		return fmt.Errorf("policy %s already exists", policy.ID)
	}

	policy.CreatedAt = time.Now()
	m.policies[policy.ID] = policy

	return nil
}

// GetPolicy returns a policy by ID
func (m *Manager) GetPolicy(id string) *ScanPolicy {
	m.policyMu.RLock()
	defer m.policyMu.RUnlock()
	return m.policies[id]
}

// ListPolicies returns all policies
func (m *Manager) ListPolicies() []*ScanPolicy {
	m.policyMu.RLock()
	defer m.policyMu.RUnlock()

	policies := make([]*ScanPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy deletes a policy
func (m *Manager) DeletePolicy(id string) bool {
	m.policyMu.Lock()
	defer m.policyMu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return false
	}
	delete(m.policies, id)
	return true
}

// calculateSummary calculates vulnerability summary
func (m *Manager) calculateSummary(result *ScanResult) VulnSummary {
	summary := VulnSummary{
		Total: len(result.Vulnerabilities),
	}

	for _, v := range result.Vulnerabilities {
		switch v.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityHigh:
			summary.High++
		case SeverityMedium:
			summary.Medium++
		case SeverityLow:
			summary.Low++
		default:
			summary.Info++
		}

		if v.FixedIn != "" {
			summary.Fixed++
		} else {
			summary.Unfixed++
		}
	}

	for _, c := range result.Compliance {
		if c.Status == ComplianceStatusPass {
			summary.CompliancePass++
		} else if c.Status == ComplianceStatusFail {
			summary.ComplianceFail++
		}
	}

	return summary
}

// severityRank returns numeric rank for severity comparison
func (m *Manager) severityRank(severity string) int {
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

// runScheduler periodically runs scheduled scans
func (m *Manager) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runDueScans(ctx)
		}
	}
}

// runDueScans runs all scheduled scans that are due
func (m *Manager) runDueScans(ctx context.Context) {
	m.schedMu.RLock()
	var dueScans []*ScanSchedule
	now := time.Now()
	for _, sched := range m.schedules {
		if sched.Enabled && now.After(sched.NextRun) {
			dueScans = append(dueScans, sched)
		}
	}
	m.schedMu.RUnlock()

	for _, sched := range dueScans {
		m.logger.Info("running scheduled scan",
			zap.String("schedule_id", sched.ID),
			zap.String("image", sched.Image))

		_, err := m.ScanImage(ctx, ScanRequest{Image: sched.Image})
		if err != nil {
			m.logger.Error("scheduled scan failed",
				zap.String("schedule_id", sched.ID),
				zap.Error(err))
			continue
		}

		// Update schedule
		m.schedMu.Lock()
		sched.LastRun = &now
		sched.NextRun = now.Add(sched.Interval)
		m.schedMu.Unlock()
	}
}
