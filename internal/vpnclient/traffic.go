package vpnclient

import (
	"fmt"
	"sync"
	"time"
)

// TrafficMonitor monitors and tracks VPN traffic statistics.
type TrafficMonitor struct {
	mu        sync.RWMutex
	stats     map[string]*TrafficStats     // connectionID -> stats
	history   map[string]*TrafficHistory   // profileID -> history
	snapshots map[string][]TrafficSnapshot // profileID -> snapshots
	alerts    map[string]*TrafficAlert     // alertID -> alert
	limits    map[string]int64             // profileID -> limit bytes
	startTime time.Time
}

// TrafficLimit represents a traffic limit configuration.
type TrafficLimit struct {
	ProfileID string `json:"profile_id"`
	Limit     int64  `json:"limit"`  // bytes, 0 = unlimited
	Period    string `json:"period"` // "hour", "day", "month"
	Action    string `json:"action"` // "alert", "disconnect", "throttle"
}

// NewTrafficMonitor creates a new traffic monitor.
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{
		stats:     make(map[string]*TrafficStats),
		history:   make(map[string]*TrafficHistory),
		snapshots: make(map[string][]TrafficSnapshot),
		alerts:    make(map[string]*TrafficAlert),
		limits:    make(map[string]int64),
		startTime: time.Now(),
	}
}

// RecordTraffic records traffic for a connection.
func (m *TrafficMonitor) RecordTraffic(connID string, rxBytes, txBytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, exists := m.stats[connID]
	if !exists {
		stats = &TrafficStats{
			UpdatedAt: time.Now(),
		}
		m.stats[connID] = stats
	}

	stats.RxBytes += rxBytes
	stats.TxBytes += txBytes
	stats.RxPackets++
	stats.TxPackets++
	stats.UpdatedAt = time.Now()
}

// GetTrafficStats returns traffic stats for a connection.
func (m *TrafficMonitor) GetTrafficStats(connID string) (*TrafficStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[connID]
	if !exists {
		return nil, fmt.Errorf("traffic stats not found for connection: %s", connID)
	}

	result := *stats
	return &result, nil
}

// GetTotalTraffic returns total traffic across all connections.
func (m *TrafficMonitor) GetTotalTraffic() TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := TrafficStats{
		UpdatedAt: time.Now(),
	}

	for _, stats := range m.stats {
		total.RxBytes += stats.RxBytes
		total.TxBytes += stats.TxBytes
		total.RxPackets += stats.RxPackets
		total.TxPackets += stats.TxPackets
	}

	return total
}

// TakeSnapshot takes a traffic snapshot for a connection.
func (m *TrafficMonitor) TakeSnapshot(connID, profileID string) TrafficSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, exists := m.stats[connID]
	if !exists {
		stats = &TrafficStats{
			UpdatedAt: time.Now(),
		}
		m.stats[connID] = stats
	}

	snapshot := TrafficSnapshot{
		Timestamp:  time.Now(),
		RxBytes:    stats.RxBytes,
		TxBytes:    stats.TxBytes,
		Connection: connID,
	}

	// Calculate rates from previous snapshot
	snapshots := m.snapshots[profileID]
	if len(snapshots) > 0 {
		prev := snapshots[len(snapshots)-1]
		duration := snapshot.Timestamp.Sub(prev.Timestamp)
		if duration.Seconds() > 0 {
			snapshot.RxRate = calculateRate(prev.RxBytes, snapshot.RxBytes, duration)
			snapshot.TxRate = calculateRate(prev.TxBytes, snapshot.TxBytes, duration)
		}
	}

	m.snapshots[profileID] = append(m.snapshots[profileID], snapshot)

	// Keep last 1000 snapshots per profile
	if len(m.snapshots[profileID]) > 1000 {
		m.snapshots[profileID] = m.snapshots[profileID][len(m.snapshots[profileID])-1000:]
	}

	return snapshot
}

// GetTrafficHistory returns traffic history for a profile.
func (m *TrafficMonitor) GetTrafficHistory(profileID, period string) *TrafficHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := m.snapshots[profileID]
	if len(snapshots) == 0 {
		return &TrafficHistory{
			ProfileID: profileID,
			Period:    period,
			Snapshots: []TrafficSnapshot{},
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
	}

	now := time.Now()
	var startTime time.Time

	switch period {
	case "hour":
		startTime = now.Add(-1 * time.Hour)
	case "day":
		startTime = now.Add(-24 * time.Hour)
	case "week":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "month":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour)
	}

	// Filter snapshots by time period
	var filtered []TrafficSnapshot
	var totalRx, totalTx int64

	for _, snap := range snapshots {
		if snap.Timestamp.After(startTime) {
			filtered = append(filtered, snap)
			totalRx += snap.RxBytes
			totalTx += snap.TxBytes
		}
	}

	return &TrafficHistory{
		ProfileID: profileID,
		Period:    period,
		Snapshots: filtered,
		TotalRx:   totalRx,
		TotalTx:   totalTx,
		StartTime: startTime,
		EndTime:   now,
	}
}

// GetRealtimeBandwidth returns real-time bandwidth for a connection.
func (m *TrafficMonitor) GetRealtimeBandwidth(profileID string) (rxRate, txRate float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := m.snapshots[profileID]
	if len(snapshots) < 2 {
		return 0, 0
	}

	// Use last two snapshots to calculate rate
	last := snapshots[len(snapshots)-1]
	prev := snapshots[len(snapshots)-2]
	duration := last.Timestamp.Sub(prev.Timestamp)

	if duration.Seconds() <= 0 {
		return 0, 0
	}

	rxRate = calculateRate(prev.RxBytes, last.RxBytes, duration)
	txRate = calculateRate(prev.TxBytes, last.TxBytes, duration)
	return rxRate, txRate
}

// SetTrafficLimit sets a traffic limit for a profile.
func (m *TrafficMonitor) SetTrafficLimit(profileID string, limit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.limits[profileID] = limit
}

// GetTrafficLimit returns the traffic limit for a profile.
func (m *TrafficMonitor) GetTrafficLimit(profileID string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.limits[profileID]
}

// CheckTrafficLimit checks if a profile has exceeded its traffic limit.
func (m *TrafficMonitor) CheckTrafficLimit(profileID string) (exceeded bool, usage int64, limit int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit = m.limits[profileID]
	if limit <= 0 {
		return false, 0, 0 // unlimited
	}

	// Calculate total usage from snapshots
	var totalRx, totalTx int64
	for _, snap := range m.snapshots[profileID] {
		totalRx += snap.RxBytes
		totalTx += snap.TxBytes
	}

	usage = totalRx + totalTx
	exceeded = usage >= limit
	return exceeded, usage, limit
}

// CreateAlert creates a traffic alert.
func (m *TrafficMonitor) CreateAlert(alertID, profileID string, threshold int64, direction, period string) *TrafficAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert := &TrafficAlert{
		ID:        alertID,
		ProfileID: profileID,
		Threshold: threshold,
		Direction: direction,
		Period:    period,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	m.alerts[alertID] = alert
	return alert
}

// GetAlert returns a traffic alert.
func (m *TrafficMonitor) GetAlert(alertID string) (*TrafficAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}

	result := *alert
	return &result, nil
}

// ListAlerts returns all traffic alerts.
func (m *TrafficMonitor) ListAlerts() []TrafficAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]TrafficAlert, 0, len(m.alerts))
	for _, alert := range m.alerts {
		alerts = append(alerts, *alert)
	}
	return alerts
}

// DeleteAlert deletes a traffic alert.
func (m *TrafficMonitor) DeleteAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alerts[alertID]; !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	delete(m.alerts, alertID)
	return nil
}

// CheckAlerts checks all alerts and returns triggered ones.
func (m *TrafficMonitor) CheckAlerts() []TrafficAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	var triggered []TrafficAlert

	for _, alert := range m.alerts {
		if !alert.Enabled {
			continue
		}

		// Get traffic data for the alert's profile
		var totalRx, totalTx int64
		for _, snap := range m.snapshots[alert.ProfileID] {
			totalRx += snap.RxBytes
			totalTx += snap.TxBytes
		}

		var usage int64
		switch alert.Direction {
		case "rx":
			usage = totalRx
		case "tx":
			usage = totalTx
		case "both":
			usage = totalRx + totalTx
		}

		if usage >= alert.Threshold {
			alert.Triggered = true
			alert.TriggeredAt = time.Now()
			alert.LastValue = usage
			m.alerts[alert.ID] = alert
			triggered = append(triggered, *alert)
		}
	}

	return triggered
}

// ResetStats resets traffic stats for a connection.
func (m *TrafficMonitor) ResetStats(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats[connID] = &TrafficStats{
		UpdatedAt: time.Now(),
	}
}

// ResetAllStats resets all traffic stats.
func (m *TrafficMonitor) ResetAllStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = make(map[string]*TrafficStats)
	m.snapshots = make(map[string][]TrafficSnapshot)
}

// GetUptime returns the uptime of the traffic monitor.
func (m *TrafficMonitor) GetUptime() time.Duration {
	return time.Since(m.startTime)
}
