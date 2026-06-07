package disklifecycle

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// NewManager creates a new disk lifecycle manager.
func NewManager(config Config, logger Logger) (*Manager, error) {
	if config.MaxAlerts <= 0 {
		config.MaxAlerts = 1000
	}
	if config.TrendWindowDays <= 0 {
		config.TrendWindowDays = 30
	}

	if config.StoragePath != "" {
		if err := os.MkdirAll(config.StoragePath, 0750); err != nil {
			return nil, fmt.Errorf("create storage path: %w", err)
		}
	}

	m := &Manager{
		config:    config,
		disks:     make(map[string]*Disk),
		alerts:    make([]*Alert, 0),
		events:    make([]*Event, 0),
		predictor: &Predictor{model: config.PredictionModel, window: config.TrendWindowDays},
		logger:    logger,
		stopCh:    make(chan struct{}),
	}

	if err := m.loadState(); err != nil {
		logger.Warn("failed to load disk lifecycle state", "error", err)
	}

	if config.Enabled && config.ScanIntervalHours > 0 {
		go m.scanLoop()
	}

	return m, nil
}

// RegisterDisk registers a new disk for monitoring.
func (m *Manager) RegisterDisk(disk *Disk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if disk.ID == "" {
		disk.ID = generateID("disk")
	}
	now := time.Now()
	disk.CreatedAt = now
	disk.UpdatedAt = now
	if disk.Status == "" {
		disk.Status = StatusUnknown
	}
	if disk.SmartStatus == "" {
		disk.SmartStatus = SmartUnknown
	}
	if len(disk.HealthHistory) == 0 {
		disk.HealthHistory = make([]HealthSample, 0)
	}

	m.disks[disk.ID] = disk

	m.addEvent(disk.ID, EventDiskAdded, fmt.Sprintf("Disk %s registered", disk.Device), "", "")

	if err := m.saveState(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	m.logger.Info("disk registered", "diskId", disk.ID, "device", disk.Device, "model", disk.Model)
	return nil
}

// UnregisterDisk removes a disk from monitoring.
func (m *Manager) UnregisterDisk(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[id]
	if !exists {
		return fmt.Errorf("disk %s not found", id)
	}

	m.addEvent(id, EventDiskRemoved, fmt.Sprintf("Disk %s removed", disk.Device), "", "")
	delete(m.disks, id)

	return m.saveState()
}

// GetDisk returns a disk by ID.
func (m *Manager) GetDisk(id string) (*Disk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[id]
	if !exists {
		return nil, fmt.Errorf("disk %s not found", id)
	}
	return disk, nil
}

// ListDisks returns all registered disks.
func (m *Manager) ListDisks() []*Disk {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*Disk, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].HealthScore < disks[j].HealthScore // Worst first
	})
	return disks
}

// UpdateSMARTData updates S.M.A.R.T. data for a disk.
func (m *Manager) UpdateSMARTData(diskID string, data SMARTData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return fmt.Errorf("disk %s not found", diskID)
	}

	oldStatus := disk.Status

	disk.Temperature = data.Temperature
	disk.ReallocatedSectors = data.ReallocatedSectors
	disk.PendingSectors = data.PendingSectors
	disk.PowerOnHours = data.PowerOnHours
	disk.TotalBytesRead = data.TotalBytesRead
	disk.TotalBytesWrite = data.TotalBytesWrite
	disk.SmartStatus = SmartPassed
	if data.HealthOK {
		disk.SmartStatus = SmartPassed
	} else {
		disk.SmartStatus = SmartFailed
	}
	disk.LastScan = time.Now()
	disk.UpdatedAt = time.Now()

	// Calculate health score
	disk.HealthScore = m.calculateHealthScore(disk)

	// Add health sample
	sample := HealthSample{
		Timestamp:          time.Now(),
		HealthScore:        disk.HealthScore,
		Temperature:        disk.Temperature,
		ReallocatedSectors: disk.ReallocatedSectors,
		PendingSectors:     disk.PendingSectors,
	}
	disk.HealthHistory = append(disk.HealthHistory, sample)

	// Trim history
	maxHistory := m.config.TrendWindowDays * 4 // 4 samples per day
	if len(disk.HealthHistory) > maxHistory {
		disk.HealthHistory = disk.HealthHistory[len(disk.HealthHistory)-maxHistory:]
	}

	// Calculate trend
	disk.TrendScore = m.calculateTrend(disk.HealthHistory)

	// Update status based on health
	newStatus := m.evaluateStatus(disk)
	if newStatus != oldStatus {
		m.addEvent(diskID, EventStatusChange, fmt.Sprintf("Status changed: %s → %s", oldStatus, newStatus), string(oldStatus), string(newStatus))
		disk.Status = newStatus
	}

	// Check for alerts
	m.checkAlerts(disk)

	// Prediction
	if m.config.EnablePrediction {
		disk.PredictedLife = m.predictor.PredictDays(disk)
	}

	return m.saveState()
}

// SMARTData holds raw S.M.A.R.T. data.
type SMARTData struct {
	HealthOK           bool    `json:"healthOk"`
	Temperature        float64 `json:"temperature"`
	ReallocatedSectors int64   `json:"reallocatedSectors"`
	PendingSectors     int64   `json:"pendingSectors"`
	PowerOnHours       int64   `json:"powerOnHours"`
	TotalBytesRead     int64   `json:"totalBytesRead"`
	TotalBytesWrite    int64   `json:"totalBytesWrite"`
}

// RetireDisk marks a disk as retired.
func (m *Manager) RetireDisk(id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disk, exists := m.disks[id]
	if !exists {
		return fmt.Errorf("disk %s not found", id)
	}

	oldStatus := disk.Status
	disk.Status = StatusRetired
	disk.UpdatedAt = time.Now()

	m.addEvent(id, EventRetired, fmt.Sprintf("Disk retired: %s", reason), string(oldStatus), string(StatusRetired))

	return m.saveState()
}

// GetPrediction returns a failure prediction for a disk.
func (m *Manager) GetPrediction(id string) (*PredictionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[id]
	if !exists {
		return nil, fmt.Errorf("disk %s not found", id)
	}

	return m.predictor.Predict(disk), nil
}

// GetFleetSummary returns a summary of all disks.
func (m *Manager) GetFleetSummary() *FleetSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &FleetSummary{
		ByInterface: make(map[string]int),
		ByVendor:    make(map[string]int),
	}

	totalHealth := 0.0
	totalAge := 0.0
	now := time.Now()

	for _, d := range m.disks {
		summary.TotalDisks++
		summary.TotalCapacity += d.CapacityBytes
		totalHealth += d.HealthScore

		age := now.Sub(d.InstallDate).Hours() / 24
		totalAge += age

		switch d.Status {
		case StatusHealthy:
			summary.HealthyDisks++
		case StatusWarning:
			summary.WarningDisks++
		case StatusCritical:
			summary.CriticalDisks++
		case StatusFailed:
			summary.FailedDisks++
		case StatusRetired:
			summary.RetiredDisks++
		}

		summary.ByInterface[d.Interface]++
		summary.ByVendor[d.Vendor]++

		if d.PredictedLife > 0 && d.PredictedLife < 90 {
			summary.PredictedFails++
		}
	}

	if summary.TotalDisks > 0 {
		summary.AvgHealthScore = totalHealth / float64(summary.TotalDisks)
		summary.AvgAge = totalAge / float64(summary.TotalDisks)
	}

	summary.AlertsCount = len(m.alerts)
	return summary
}

// GetAlerts returns all alerts.
func (m *Manager) GetAlerts(dismissed bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, a := range m.alerts {
		if a.Dismissed == dismissed {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// DismissAlert dismisses an alert.
func (m *Manager) DismissAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == id {
			now := time.Now()
			a.Dismissed = true
			a.DismissedAt = &now
			return m.saveState()
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// GetEvents returns lifecycle events for a disk.
func (m *Manager) GetEvents(diskID string, limit int) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*Event, 0)
	for _, e := range m.events {
		if diskID == "" || e.DiskID == diskID {
			events = append(events, e)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})

	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events
}

// Stop stops the manager.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("disk lifecycle manager stopped")
}

// ========== Internal Methods ==========

func (m *Manager) calculateHealthScore(disk *Disk) float64 {
	score := 100.0

	// Reallocated sectors penalty
	if disk.ReallocatedSectors > 0 {
		penalty := float64(disk.ReallocatedSectors) * 5.0
		if penalty > 50 {
			penalty = 50
		}
		score -= penalty
	}

	// Pending sectors penalty
	if disk.PendingSectors > 0 {
		penalty := float64(disk.PendingSectors) * 3.0
		if penalty > 30 {
			penalty = 30
		}
		score -= penalty
	}

	// Temperature penalty
	if disk.Temperature > 60 {
		score -= (disk.Temperature - 60) * 2
	}

	// S.M.A.R.T. failure = immediate critical
	if disk.SmartStatus == SmartFailed {
		score = 0
	}

	// Age penalty
	if !disk.InstallDate.IsZero() {
		years := time.Since(disk.InstallDate).Hours() / (24 * 365)
		if years > 5 {
			score -= (years - 5) * 5
		}
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func (m *Manager) calculateTrend(history []HealthSample) float64 {
	if len(history) < 2 {
		return 0
	}

	// Simple linear regression on health scores
	n := float64(len(history))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, sample := range history {
		x := float64(i)
		y := sample.HealthScore
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return math.Round(slope*100) / 100
}

func (m *Manager) evaluateStatus(disk *Disk) DiskStatus {
	if disk.SmartStatus == SmartFailed {
		return StatusFailed
	}
	if disk.HealthScore < m.config.RetireThreshold {
		return StatusCritical
	}
	if disk.HealthScore < m.config.HealthThreshold {
		return StatusWarning
	}
	return StatusHealthy
}

func (m *Manager) checkAlerts(disk *Disk) {
	// Health decline alert
	if disk.HealthScore < m.config.HealthThreshold {
		m.addAlert(disk.ID, disk.Device, AlertHealthDecline, SeverityWarning,
			"Disk Health Declining",
			fmt.Sprintf("Disk %s health score is %.1f (threshold: %.1f)", disk.Device, disk.HealthScore, m.config.HealthThreshold),
			disk.HealthScore, m.config.HealthThreshold)
	}

	// Temperature alert
	if disk.Temperature > 60 {
		sev := SeverityWarning
		if disk.Temperature > 70 {
			sev = SeverityCritical
		}
		m.addAlert(disk.ID, disk.Device, AlertTemperatureHigh, sev,
			"High Temperature",
			fmt.Sprintf("Disk %s temperature is %.1f°C", disk.Device, disk.Temperature),
			disk.Temperature, 60)
	}

	// Reallocated sectors
	if disk.ReallocatedSectors > 0 {
		m.addAlert(disk.ID, disk.Device, AlertReallocatedSectors, SeverityWarning,
			"Reallocated Sectors Detected",
			fmt.Sprintf("Disk %s has %d reallocated sectors", disk.Device, disk.ReallocatedSectors),
			float64(disk.ReallocatedSectors), 0)
	}

	// Predicted failure
	if disk.PredictedLife > 0 && disk.PredictedLife < 90 {
		m.addAlert(disk.ID, disk.Device, AlertPredictedFailure, SeverityCritical,
			"Predicted Failure",
			fmt.Sprintf("Disk %s predicted to fail in %d days", disk.Device, disk.PredictedLife),
			float64(disk.PredictedLife), 90)
	}

	// Warranty expiring
	if !disk.WarrantyExpiry.IsZero() {
		daysUntilExpiry := time.Until(disk.WarrantyExpiry).Hours() / 24
		if daysUntilExpiry > 0 && daysUntilExpiry < 90 {
			m.addAlert(disk.ID, disk.Device, AlertWarrantyExpiring, SeverityInfo,
				"Warranty Expiring Soon",
				fmt.Sprintf("Disk %s warranty expires in %.0f days", disk.Device, daysUntilExpiry),
				daysUntilExpiry, 90)
		}
	}
}

func (m *Manager) addAlert(diskID, device string, alertType AlertType, severity Severity, title, message string, value, threshold float64) {
	alert := &Alert{
		ID:        generateID("alert"),
		DiskID:    diskID,
		Device:    device,
		Type:      alertType,
		Severity:  severity,
		Title:     title,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		CreatedAt: time.Now(),
	}

	m.alerts = append(m.alerts, alert)

	// Trim alerts
	if len(m.alerts) > m.config.MaxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.config.MaxAlerts:]
	}
}

func (m *Manager) addEvent(diskID string, eventType EventType, message, oldVal, newVal string) {
	event := &Event{
		ID:        generateID("evt"),
		DiskID:    diskID,
		Type:      eventType,
		Message:   message,
		OldValue:  oldVal,
		NewValue:  newVal,
		CreatedAt: time.Now(),
	}
	m.events = append(m.events, event)
}

// ========== Predictor ==========

// Predict generates a failure prediction for a disk.
func (p *Predictor) Predict(disk *Disk) *PredictionResult {
	result := &PredictionResult{
		DiskID:      disk.ID,
		GeneratedAt: time.Now(),
		Factors:     make([]string, 0),
	}

	failureScore := 0.0

	// S.M.A.R.T. failure
	if disk.SmartStatus == SmartFailed {
		failureScore += 0.5
		result.Factors = append(result.Factors, "S.M.A.R.T. failure detected")
	}

	// Reallocated sectors
	if disk.ReallocatedSectors > 0 {
		failureScore += float64(disk.ReallocatedSectors) * 0.05
		result.Factors = append(result.Factors, fmt.Sprintf("%d reallocated sectors", disk.ReallocatedSectors))
	}

	// Pending sectors
	if disk.PendingSectors > 0 {
		failureScore += float64(disk.PendingSectors) * 0.03
		result.Factors = append(result.Factors, fmt.Sprintf("%d pending sectors", disk.PendingSectors))
	}

	// Temperature
	if disk.Temperature > 55 {
		failureScore += (disk.Temperature - 55) * 0.01
		result.Factors = append(result.Factors, fmt.Sprintf("high temperature: %.1f°C", disk.Temperature))
	}

	// Health trend
	if disk.TrendScore < -0.5 {
		failureScore += math.Abs(disk.TrendScore) * 0.1
		result.Factors = append(result.Factors, fmt.Sprintf("declining health trend: %.2f/day", disk.TrendScore))
	}

	// Age
	if !disk.InstallDate.IsZero() {
		years := time.Since(disk.InstallDate).Hours() / (24 * 365)
		if years > 3 {
			failureScore += (years - 3) * 0.05
			result.Factors = append(result.Factors, fmt.Sprintf("age: %.1f years", years))
		}
	}

	if failureScore > 1.0 {
		failureScore = 1.0
	}

	result.FailureProb = math.Round(failureScore*100) / 100
	result.Confidence = 0.7 // Base confidence

	if len(disk.HealthHistory) > 10 {
		result.Confidence = 0.85
	}
	if len(disk.HealthHistory) > 30 {
		result.Confidence = 0.95
	}

	// Predict days left
	if failureScore > 0.5 {
		result.PredictedDays = int((1.0 - failureScore) * 365)
	} else {
		result.PredictedDays = 365 * 3 // 3+ years
	}

	// Recommendation
	switch {
	case failureScore > 0.7:
		result.Recommendation = "Replace disk immediately"
	case failureScore > 0.4:
		result.Recommendation = "Plan disk replacement within 30 days"
	case failureScore > 0.2:
		result.Recommendation = "Monitor closely, consider replacement in 6 months"
	default:
		result.Recommendation = "Disk is healthy, continue normal monitoring"
	}

	return result
}

// PredictDays returns predicted days of life remaining.
func (p *Predictor) PredictDays(disk *Disk) int {
	return p.Predict(disk).PredictedDays
}

// ========== Persistence ==========

func (m *Manager) saveState() error {
	if m.config.StoragePath == "" {
		return nil
	}

	state := struct {
		Disks  map[string]*Disk `json:"disks"`
		Alerts []*Alert         `json:"alerts"`
		Events []*Event         `json:"events"`
	}{
		Disks:  m.disks,
		Alerts: m.alerts,
		Events: m.events,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.config.StoragePath, "state.json")
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) loadState() error {
	if m.config.StoragePath == "" {
		return nil
	}

	path := filepath.Join(m.config.StoragePath, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Disks  map[string]*Disk `json:"disks"`
		Alerts []*Alert         `json:"alerts"`
		Events []*Event         `json:"events"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if state.Disks != nil {
		m.disks = state.Disks
	}
	if state.Alerts != nil {
		m.alerts = state.Alerts
	}
	if state.Events != nil {
		m.events = state.Events
	}

	return nil
}

func (m *Manager) scanLoop() {
	interval := time.Duration(m.config.ScanIntervalHours) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.runScan()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) runScan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, disk := range m.disks {
		if disk.Status == StatusRetired || disk.Status == StatusReplaced {
			continue
		}
		// In a real implementation, this would query S.M.A.R.T. data
		m.logger.Debug("scanning disk", "device", disk.Device)
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
