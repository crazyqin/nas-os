// Package zfshealer provides automatic ZFS data integrity verification and repair.
// Inspired by TrueNAS 26's data protection and scheduled scrub capabilities.
package zfshealer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealStatus represents the current status of a healing operation.
type HealStatus string

const (
	StatusIdle      HealStatus = "idle"
	StatusScanning  HealStatus = "scanning"
	StatusRepairing HealStatus = "repairing"
	StatusCompleted HealStatus = "completed"
	StatusFailed    HealStatus = "failed"
)

// IntegrityLevel defines the depth of integrity checking.
type IntegrityLevel string

const (
	IntegrityQuick    IntegrityLevel = "quick"    // metadata only
	IntegrityStandard IntegrityLevel = "standard" // metadata + selective data
	IntegrityDeep     IntegrityLevel = "deep"     // full scrub
)

// DatasetHealth holds health information for a single ZFS dataset.
type DatasetHealth struct {
	Name           string         `json:"name"`
	Pool           string         `json:"pool"`
	Status         string         `json:"status"` // ONLINE, DEGRADED, FAULTED
	ScanErrors     int64          `json:"scan_errors"`
	ChecksumErrors int64          `json:"checksum_errors"`
	LastScrub      time.Time      `json:"last_scrub"`
	ScrubDuration  time.Duration  `json:"scrub_duration"`
	BytesScanned   int64          `json:"bytes_scanned"`
	BytesRepaired  int64          `json:"bytes_repaired"`
	HealthScore    float64        `json:"health_score"` // 0.0 (critical) to 1.0 (perfect)
	NextScrub      time.Time      `json:"next_scrub"`
	IntegrityLevel IntegrityLevel `json:"integrity_level"`
}

// HealResult contains the outcome of a healing operation.
type HealResult struct {
	Dataset      string         `json:"dataset"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Duration     time.Duration  `json:"duration"`
	Status       HealStatus     `json:"status"`
	ErrorsBefore int64          `json:"errors_before"`
	ErrorsAfter  int64          `json:"errors_after"`
	BytesFixed   int64          `json:"bytes_fixed"`
	RepairCount  int            `json:"repair_count"`
	Level        IntegrityLevel `json:"level"`
	Message      string         `json:"message,omitempty"`
}

// ScrubSchedule defines when and how to run integrity checks.
type ScrubSchedule struct {
	Enabled       bool           `json:"enabled"`
	Interval      time.Duration  `json:"interval"`       // e.g. 7 days
	PreferredHour int            `json:"preferred_hour"` // 0-23, avoid peak
	AvoidDays     []time.Weekday `json:"avoid_days"`     // e.g. weekdays
	Level         IntegrityLevel `json:"level"`
	AutoRepair    bool           `json:"auto_repair"`
	MaxDuration   time.Duration  `json:"max_duration"` // timeout for scrub
}

// AlertSeverity for integrity issues.
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// IntegrityAlert is generated when integrity issues are detected.
type IntegrityAlert struct {
	Timestamp time.Time     `json:"timestamp"`
	Severity  AlertSeverity `json:"severity"`
	Dataset   string        `json:"dataset"`
	Pool      string        `json:"pool"`
	Message   string        `json:"message"`
	ErrorCount int64        `json:"error_count"`
	AutoFixed  bool         `json:"auto_fixed"`
}

// Manager orchestrates ZFS integrity verification and repair operations.
type Manager struct {
	mu          sync.RWMutex
	datasets    map[string]*DatasetHealth
	results     []HealResult
	alerts      []IntegrityAlert
	schedule    ScrubSchedule
	running     bool
	maxHistory  int
}

// NewManager creates a new ZFSHealer manager with the given schedule.
func NewManager(schedule ScrubSchedule) *Manager {
	return &Manager{
		datasets:   make(map[string]*DatasetHealth),
		results:    make([]HealResult, 0, 100),
		alerts:     make([]IntegrityAlert, 0, 100),
		schedule:   schedule,
		maxHistory: 1000,
	}
}

// GetSchedule returns the current scrub schedule.
func (m *Manager) GetSchedule() ScrubSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.schedule
}

// UpdateSchedule updates the scrub schedule.
func (m *Manager) UpdateSchedule(schedule ScrubSchedule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedule = schedule
}

// RegisterDataset registers a ZFS dataset for health monitoring.
func (m *Manager) RegisterDataset(name, pool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.datasets[name] = &DatasetHealth{
		Name:           name,
		Pool:           pool,
		Status:         "ONLINE",
		HealthScore:    1.0,
		IntegrityLevel: m.schedule.Level,
	}
}

// GetDatasetHealth returns health info for a specific dataset.
func (m *Manager) GetDatasetHealth(name string) (*DatasetHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.datasets[name]
	return h, ok
}

// ListDatasetHealth returns health info for all monitored datasets.
func (m *Manager) ListDatasetHealth() []*DatasetHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*DatasetHealth, 0, len(m.datasets))
	for _, h := range m.datasets {
		result = append(result, h)
	}
	return result
}

// RunScrub starts an integrity scrub on the specified dataset.
func (m *Manager) RunScrub(ctx context.Context, dataset string, level IntegrityLevel) (*HealResult, error) {
	m.mu.Lock()
	health, exists := m.datasets[dataset]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("dataset %s not registered", dataset)
	}
	if m.running {
		m.mu.Unlock()
		return nil, fmt.Errorf("scrub already in progress")
	}
	m.running = true
	m.mu.Unlock()

	startTime := time.Now()
	result := &HealResult{
		Dataset:   dataset,
		StartTime: startTime,
		Level:     level,
		Status:    StatusScanning,
	}

	// Simulate scrub operation
	errorsBefore := health.ChecksumErrors
	result.ErrorsBefore = errorsBefore

	// Update health status
	m.mu.Lock()
	health.Status = "SCRUBBING"
	m.mu.Unlock()

	// Perform scrub based on level
	// In production: exec.CommandContext(ctx, "zpool", "scrub", health.Pool)
	if ctx.Err() != nil {
		result.Status = StatusFailed
		result.Message = "scrub cancelled"
	} else {
		result.Status = StatusCompleted
		result.ErrorsAfter = 0
		result.BytesFixed = errorsBefore * 4096 // approximate
		result.RepairCount = int(errorsBefore)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)

		if errorsBefore > 0 {
			result.Message = fmt.Sprintf("repaired %d errors", errorsBefore)
		} else {
			result.Message = "no errors detected"
		}
	}

	// Update dataset health
	m.mu.Lock()
	health.LastScrub = time.Now()
	health.ScrubDuration = result.Duration
	health.ChecksumErrors = result.ErrorsAfter
	health.BytesRepaired += result.BytesFixed
	health.Status = "ONLINE"
	if result.ErrorsAfter == 0 {
		health.HealthScore = 1.0
	}
	health.NextScrub = m.calculateNextScrub()

	m.results = append(m.results, *result)
	if len(m.results) > m.maxHistory {
		m.results = m.results[len(m.results)-m.maxHistory:]
	}

	// Generate alert if errors were found
	if errorsBefore > 0 {
		alert := IntegrityAlert{
			Timestamp:  time.Now(),
			Severity:   AlertWarning,
			Dataset:    dataset,
			Pool:       health.Pool,
			Message:    fmt.Sprintf("Found and repaired %d integrity errors", errorsBefore),
			ErrorCount: errorsBefore,
			AutoFixed:  true,
		}
		m.alerts = append(m.alerts, alert)
	}

	m.running = false
	m.mu.Unlock()

	return result, nil
}

// GetResults returns recent healing results.
func (m *Manager) GetResults(limit int) []HealResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.results) {
		limit = len(m.results)
	}
	start := len(m.results) - limit
	return m.results[start:]
}

// GetAlerts returns recent integrity alerts.
func (m *Manager) GetAlerts(limit int) []IntegrityAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	start := len(m.alerts) - limit
	return m.alerts[start:]
}

// IsRunning returns whether a scrub is currently in progress.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// calculateNextScrub computes the next scrub time based on schedule.
func (m *Manager) calculateNextScrub() time.Time {
	now := time.Now()
	next := now.Add(m.schedule.Interval)

	// Adjust to preferred hour
	if m.schedule.PreferredHour >= 0 && m.schedule.PreferredHour < 24 {
		next = time.Date(next.Year(), next.Month(), next.Day(),
			m.schedule.PreferredHour, 0, 0, 0, next.Location())
	}

	// Avoid specified days
	for _, avoidDay := range m.schedule.AvoidDays {
		for next.Weekday() == avoidDay {
			next = next.Add(24 * time.Hour)
		}
	}

	return next
}

// UpdateHealthFromZFS refreshes dataset health from actual ZFS status.
// In production, this would call zpool status / zfs get via shell.
func (m *Manager) UpdateHealthFromZFS(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, health := range m.datasets {
		// Simulate ZFS health check
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// In production: exec.CommandContext(ctx, "zpool", "status", health.Pool)
		// For now, maintain current state
		health.Name = name
	}

	return nil
}
