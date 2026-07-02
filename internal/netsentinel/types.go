// Package netsentinel provides real-time network traffic analysis and anomaly detection for NAS-OS
// Features: Traffic analysis, anomaly detection, bandwidth monitoring, alert system
// Competitor benchmark: 对标群晖Traffic Monitor, 超越TrueNAS网络监控能力
package netsentinel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AlertSeverity represents alert severity.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertType represents the type of alert.
type AlertType string

const (
	AlertBandwidthSpike  AlertType = "bandwidth_spike"
	AlertSuspiciousConn  AlertType = "suspicious_connection"
	AlertPortScan        AlertType = "port_scan"
	AlertDDoS            AlertType = "ddos_detected"
	AlertDataExfil       AlertType = "data_exfiltration"
	AlertUnusualProtocol AlertType = "unusual_protocol"
)

// TrafficRecord represents a traffic record.
type TrafficRecord struct {
	ID         string    `json:"id"`
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	SrcPort    int       `json:"src_port"`
	DstPort    int       `json:"dst_port"`
	Protocol   string    `json:"protocol"`
	BytesIn    int64     `json:"bytes_in"`
	BytesOut   int64     `json:"bytes_out"`
	PacketsIn  int64     `json:"packets_in"`
	PacketsOut int64     `json:"packets_out"`
	Duration   int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// Alert represents a network alert.
type Alert struct {
	ID           string        `json:"id"`
	Type         AlertType     `json:"type"`
	Severity     AlertSeverity `json:"severity"`
	Source       string        `json:"source"`
	Destination  string        `json:"destination"`
	Description  string        `json:"description"`
	Details      string        `json:"details"`
	Acknowledged bool          `json:"acknowledged"`
	Timestamp    time.Time     `json:"timestamp"`
}

// BandwidthRecord represents bandwidth usage at a point in time.
type BandwidthRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Interface  string    `json:"interface"`
	BytesIn    int64     `json:"bytes_in"`
	BytesOut   int64     `json:"bytes_out"`
	PacketsIn  int64     `json:"packets_in"`
	PacketsOut int64     `json:"packets_out"`
	BpsIn      int64     `json:"bps_in"`
	BpsOut     int64     `json:"bps_out"`
}

// ConnectionInfo represents an active connection.
type ConnectionInfo struct {
	SrcIP     string    `json:"src_ip"`
	DstIP     string    `json:"dst_ip"`
	SrcPort   int       `json:"src_port"`
	DstPort   int       `json:"dst_port"`
	Protocol  string    `json:"protocol"`
	State     string    `json:"state"`
	PID       int       `json:"pid"`
	Process   string    `json:"process"`
	BytesIn   int64     `json:"bytes_in"`
	BytesOut  int64     `json:"bytes_out"`
	StartTime time.Time `json:"start_time"`
}

// SentinelStats represents network sentinel statistics.
type SentinelStats struct {
	TotalAlerts       int   `json:"total_alerts"`
	UnackAlerts       int   `json:"unacknowledged_alerts"`
	TotalTrafficRecs  int   `json:"total_traffic_records"`
	ActiveConnections int   `json:"active_connections"`
	BandwidthIn       int64 `json:"current_bps_in"`
	BandwidthOut      int64 `json:"current_bps_out"`
	TopTalkers        int   `json:"top_talkers"`
}

// Config holds network sentinel configuration.
type Config struct {
	Enabled              bool  `json:"enabled"`
	MonitorInterval      int   `json:"monitor_interval_seconds"`
	AlertThresholdBps    int64 `json:"alert_threshold_bps"`
	DDoSThreshold        int   `json:"ddos_threshold_pps"`
	PortScanThreshold    int   `json:"port_scan_threshold"`
	TrafficRetentionH    int   `json:"traffic_retention_hours"`
	AlertRetentionDays   int   `json:"alert_retention_days"`
	EnableDeepInspection bool  `json:"enable_deep_inspection"`
}

// Manager manages network sentinel.
type Manager struct {
	config      *Config
	traffic     []*TrafficRecord
	alerts      []*Alert
	bandwidth   []*BandwidthRecord
	connections []*ConnectionInfo
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager creates a new network sentinel manager.
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:      config,
		traffic:     make([]*TrafficRecord, 0),
		alerts:      make([]*Alert, 0),
		bandwidth:   make([]*BandwidthRecord, 0),
		connections: make([]*ConnectionInfo, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the network sentinel.
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return fmt.Errorf("network sentinel is disabled")
	}
	return nil
}

// Stop stops the network sentinel.
func (m *Manager) Stop() {
	m.cancel()
}

// RecordTraffic records a traffic entry.
func (m *Manager) RecordTraffic(record *TrafficRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record.ID = fmt.Sprintf("traf-%d", time.Now().UnixNano())
	record.Timestamp = time.Now()
	m.traffic = append(m.traffic, record)
}

// CreateAlert creates a new alert.
func (m *Manager) CreateAlert(alertType AlertType, severity AlertSeverity, src, dst, desc string) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert := &Alert{
		ID:          fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:        alertType,
		Severity:    severity,
		Source:      src,
		Destination: dst,
		Description: desc,
		Timestamp:   time.Now(),
	}
	m.alerts = append(m.alerts, alert)
	return alert
}

// ListAlerts returns all alerts.
func (m *Manager) ListAlerts() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// AcknowledgeAlert acknowledges an alert.
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id {
			a.Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// RecordBandwidth records bandwidth usage.
func (m *Manager) RecordBandwidth(record *BandwidthRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record.Timestamp = time.Now()
	m.bandwidth = append(m.bandwidth, record)
}

// GetConnections returns active connections.
func (m *Manager) GetConnections() []*ConnectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections
}

// GetStats returns sentinel statistics.
func (m *Manager) GetStats() *SentinelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SentinelStats{
		TotalAlerts:       len(m.alerts),
		TotalTrafficRecs:  len(m.traffic),
		ActiveConnections: len(m.connections),
	}

	for _, a := range m.alerts {
		if !a.Acknowledged {
			stats.UnackAlerts++
		}
	}

	if len(m.bandwidth) > 0 {
		last := m.bandwidth[len(m.bandwidth)-1]
		stats.BandwidthIn = last.BpsIn
		stats.BandwidthOut = last.BpsOut
	}

	return stats
}
