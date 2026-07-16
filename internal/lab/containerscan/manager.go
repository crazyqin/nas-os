// Package containerscan provides Docker image vulnerability scanning with CVE detection,
// layer analysis, severity rating, auto-fix suggestions, scheduled scanning, and report generation.
package containerscan

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages container scan operations including scheduling, lists, and reports.
type Manager struct {
	logger    *zap.Logger
	scanner   *Scanner
	schedules map[string]*ScanSchedule
	schedMu   sync.RWMutex
	whitelist map[string]*ImageListEntry
	blacklist map[string]*ImageListEntry
	listMu    sync.RWMutex
	reports   map[string]*ScanReport
	reportMu  sync.RWMutex
	stopCh    chan struct{}
}

// NewManager creates a new scan manager.
func NewManager(logger *zap.Logger, scanner *Scanner) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:    logger,
		scanner:   scanner,
		schedules: make(map[string]*ScanSchedule),
		whitelist: make(map[string]*ImageListEntry),
		blacklist: make(map[string]*ImageListEntry),
		reports:   make(map[string]*ScanReport),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the scheduled scan runner.
func (m *Manager) Start(ctx context.Context) {
	go m.runScheduler(ctx)
	m.logger.Info("scan manager started")
}

// Stop stops the scheduled scan runner.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("scan manager stopped")
}

// runScheduler periodically checks and runs scheduled scans.
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

// runDueScans runs all scheduled scans that are due.
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

		result, err := m.scanner.ScanImage(ctx, sched.Image, "", false)
		if err != nil {
			m.logger.Error("scheduled scan failed",
				zap.String("schedule_id", sched.ID),
				zap.Error(err))
			continue
		}

		// Generate report
		m.GenerateReport(sched.Image, ReportFormatJSON)

		// Update schedule
		m.schedMu.Lock()
		sched.LastRun = now
		sched.NextRun = now.Add(sched.Interval)
		m.schedMu.Unlock()

		m.logger.Info("scheduled scan completed",
			zap.String("schedule_id", sched.ID),
			zap.Int("vulns", result.Summary.Total))
	}
}

// AddSchedule adds a new scan schedule.
func (m *Manager) AddSchedule(schedule *ScanSchedule) error {
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

	schedule.CreatedAt = time.Now()
	schedule.NextRun = time.Now().Add(schedule.Interval)
	m.schedules[schedule.ID] = schedule

	m.logger.Info("schedule added",
		zap.String("schedule_id", schedule.ID),
		zap.String("image", schedule.Image),
		zap.Duration("interval", schedule.Interval))

	return nil
}

// GetSchedule returns a schedule by ID.
func (m *Manager) GetSchedule(id string) *ScanSchedule {
	m.schedMu.RLock()
	defer m.schedMu.RUnlock()
	return m.schedules[id]
}

// ListSchedules returns all schedules.
func (m *Manager) ListSchedules() []*ScanSchedule {
	m.schedMu.RLock()
	defer m.schedMu.RUnlock()

	schedules := make([]*ScanSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// DeleteSchedule removes a schedule by ID.
func (m *Manager) DeleteSchedule(id string) bool {
	m.schedMu.Lock()
	defer m.schedMu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return false
	}
	delete(m.schedules, id)
	return true
}

// UpdateSchedule updates an existing schedule.
func (m *Manager) UpdateSchedule(id string, schedule *ScanSchedule) error {
	m.schedMu.Lock()
	defer m.schedMu.Unlock()

	existing, exists := m.schedules[id]
	if !exists {
		return fmt.Errorf("schedule %s not found", id)
	}

	if schedule.Interval > 0 {
		existing.Interval = schedule.Interval
	}
	if schedule.Image != "" {
		existing.Image = schedule.Image
	}
	existing.Enabled = schedule.Enabled
	existing.NextRun = time.Now().Add(existing.Interval)

	return nil
}

// AddToWhitelist adds an image to the whitelist.
func (m *Manager) AddToWhitelist(image, reason, addedBy string) {
	m.listMu.Lock()
	defer m.listMu.Unlock()

	m.whitelist[image] = &ImageListEntry{
		Image:   image,
		Reason:  reason,
		AddedAt: time.Now(),
		AddedBy: addedBy,
	}
	m.logger.Info("image added to whitelist", zap.String("image", image))
}

// RemoveFromWhitelist removes an image from the whitelist.
func (m *Manager) RemoveFromWhitelist(image string) bool {
	m.listMu.Lock()
	defer m.listMu.Unlock()

	if _, exists := m.whitelist[image]; !exists {
		return false
	}
	delete(m.whitelist, image)
	return true
}

// IsWhitelisted checks if an image is whitelisted.
func (m *Manager) IsWhitelisted(image string) bool {
	m.listMu.RLock()
	defer m.listMu.RUnlock()
	_, exists := m.whitelist[image]
	return exists
}

// ListWhitelist returns all whitelisted images.
func (m *Manager) ListWhitelist() []*ImageListEntry {
	m.listMu.RLock()
	defer m.listMu.RUnlock()

	entries := make([]*ImageListEntry, 0, len(m.whitelist))
	for _, e := range m.whitelist {
		entries = append(entries, e)
	}
	return entries
}

// AddToBlacklist adds an image to the blacklist.
func (m *Manager) AddToBlacklist(image, reason, addedBy string) {
	m.listMu.Lock()
	defer m.listMu.Unlock()

	m.blacklist[image] = &ImageListEntry{
		Image:   image,
		Reason:  reason,
		AddedAt: time.Now(),
		AddedBy: addedBy,
	}
	m.logger.Info("image added to blacklist", zap.String("image", image))
}

// RemoveFromBlacklist removes an image from the blacklist.
func (m *Manager) RemoveFromBlacklist(image string) bool {
	m.listMu.Lock()
	defer m.listMu.Unlock()

	if _, exists := m.blacklist[image]; !exists {
		return false
	}
	delete(m.blacklist, image)
	return true
}

// IsBlacklisted checks if an image is blacklisted.
func (m *Manager) IsBlacklisted(image string) bool {
	m.listMu.RLock()
	defer m.listMu.RUnlock()
	_, exists := m.blacklist[image]
	return exists
}

// ListBlacklist returns all blacklisted images.
func (m *Manager) ListBlacklist() []*ImageListEntry {
	m.listMu.RLock()
	defer m.listMu.RUnlock()

	entries := make([]*ImageListEntry, 0, len(m.blacklist))
	for _, e := range m.blacklist {
		entries = append(entries, e)
	}
	return entries
}

// GenerateReport generates a scan report in the specified format.
func (m *Manager) GenerateReport(image string, format ReportFormat) (*ScanReport, error) {
	result := m.scanner.GetCachedResult(image)
	if result == nil {
		return nil, fmt.Errorf("no scan result found for image %s", image)
	}

	report := &ScanReport{
		ID:          fmt.Sprintf("report-%s-%d", image, time.Now().Unix()),
		Image:       image,
		Format:      format,
		Result:      result,
		GeneratedAt: time.Now(),
	}

	switch format {
	case ReportFormatJSON:
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to generate JSON report: %w", err)
		}
		report.Content = data
	case ReportFormatPDF:
		// PDF generation would be implemented here
		report.Content = []byte("PDF report placeholder")
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}

	m.reportMu.Lock()
	m.reports[report.ID] = report
	m.reportMu.Unlock()

	m.logger.Info("report generated",
		zap.String("report_id", report.ID),
		zap.String("image", image),
		zap.String("format", string(format)))

	return report, nil
}

// GetReport returns a report by ID.
func (m *Manager) GetReport(id string) *ScanReport {
	m.reportMu.RLock()
	defer m.reportMu.RUnlock()
	return m.reports[id]
}

// ListReports returns all reports.
func (m *Manager) ListReports() []*ScanReport {
	m.reportMu.RLock()
	defer m.reportMu.RUnlock()

	reports := make([]*ScanReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// DeleteReport removes a report by ID.
func (m *Manager) DeleteReport(id string) bool {
	m.reportMu.Lock()
	defer m.reportMu.Unlock()

	if _, exists := m.reports[id]; !exists {
		return false
	}
	delete(m.reports, id)
	return true
}

// CheckImageAccess checks if an image is allowed to be scanned
// Returns true if allowed, false if blocked.
func (m *Manager) CheckImageAccess(image string) (bool, string) {
	// Check blacklist first
	if m.IsBlacklisted(image) {
		return false, "image is blacklisted"
	}

	// If whitelist is not empty, image must be whitelisted
	whitelist := m.ListWhitelist()
	if len(whitelist) > 0 && !m.IsWhitelisted(image) {
		return false, "image is not whitelisted"
	}

	return true, ""
}
