// Package netshield provides network traffic protection capabilities including
// port scan detection, DDoS mitigation, traffic anomaly monitoring, and network
// isolation policy enforcement. It acts as a defensive shield layer for the NAS
// operating system, inspecting traffic patterns and enforcing security policies.
package netshield

import (
	"context"
	"net"
	"sync"
	"time"
)

// -------------------- Domain Types --------------------

// TrafficShield is the central service for network traffic protection.
type TrafficShield struct {
	mu          sync.RWMutex
	alerts      []PortScanAlert
	mitigations map[string]*DDoSMitigation
	policies    []IsolationPolicy
	flowStats   *FlowStats
	thresholds  DetectionThresholds
}

// FlowStats tracks aggregate traffic statistics for anomaly detection.
type FlowStats struct {
	TotalPackets   uint64        `json:"total_packets"`
	TotalBytes     uint64        `json:"total_bytes"`
	TotalSYN      uint64        `json:"total_syn"`
	TotalConnections uint64      `json:"total_connections"`
	WindowStart   time.Time      `json:"window_start"`
}

// DetectionThresholds configures when alerts are triggered.
type DetectionThresholds struct {
	PortScanMaxUniquePorts int           `json:"port_scan_max_unique_ports"` // default 10
	PortScanTimeWindow     time.Duration `json:"port_scan_time_window"`      // default 60s
	DDoSPacketsPerSecond    uint64        `json:"ddos_pps_threshold"`         // default 50000
	DDoSBytesPerSecond      uint64        `json:"ddos_bps_threshold"`        // default 100_000_000
	DDoSConnPerSecond       uint64        `json:"ddos_conn_threshold"`       // default 1000
	SYNRatioThreshold       float64       `json:"syn_ratio_threshold"`       // default 0.8
}

// PortScanAlert represents a detected port scanning attempt.
type PortScanAlert struct {
	ID            string    `json:"id"`
	SourceIP      net.IP    `json:"source_ip"`
	TargetIP      net.IP    `json:"target_ip"`
	ScannedPorts  []int     `json:"scanned_ports"`
	ScanType      ScanType  `json:"scan_type"`
	Severity      string    `json:"severity"`     // "low" | "medium" | "high"
	PacketCount   int       `json:"packet_count"`
	DetectedAt    time.Time `json:"detected_at"`
	AutoBlocked   bool      `json:"auto_blocked"`
}

// ScanType classifies the kind of port scan detected.
type ScanType string

const (
	ScanTypeTCPConnect ScanType = "tcp_connect"
	ScanTypeSYN        ScanType = "syn_scan"
	ScanTypeUDP        ScanType = "udp_scan"
	ScanTypeFIN        ScanType = "fin_scan"
	ScanTypeNull       ScanType = "null_scan"
)

// DDoSMitigation tracks an ongoing or completed DDoS mitigation response.
type DDoSMitigation struct {
	ID              string          `json:"id"`
	AttackType      string          `json:"attack_type"` // "syn_flood" | "udp_flood" | "http_flood" | "amplification"
	SourceIPs       []net.IP        `json:"source_ips"`
	TargetIP        net.IP          `json:"target_ip"`
	TargetPort      int             `json:"target_port,omitempty"`
	PeakPPS         uint64          `json:"peak_pps"`
	PeakBPS         uint64          `json:"peak_bps"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         *time.Time      `json:"end_time,omitempty"`
	MitigationStrategy string       `json:"mitigation_strategy"`
	Status          MitigationStatus `json:"status"`
	BlockedIPs      []net.IP        `json:"blocked_ips"`
}

// MitigationStatus tracks the lifecycle of a DDoS response.
type MitigationStatus string

const (
	MitigationStatusActive     MitigationStatus = "active"
	MitigationStatusMonitoring MitigationStatus = "monitoring"
	MitigationStatusResolved   MitigationStatus = "resolved"
)

// IsolationPolicy defines rules for isolating compromised or untrusted hosts.
type IsolationPolicy struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	TargetHosts   []string       `json:"target_hosts"`     // IP or CIDR
	IsolationMode IsolationMode  `json:"isolation_mode"`
	Allowlist     []string       `json:"allowlist,omitempty"`  // IPs allowed through isolation
	Denylist      []string       `json:"denylist,omitempty"`
	Duration      time.Duration  `json:"duration,omitempty"`   // 0 = indefinite
	Enabled       bool           `json:"enabled"`
	CreatedAt     time.Time      `json:"created_at"`
	Reason        string         `json:"reason,omitempty"`
}

// IsolationMode specifies the degree of network isolation.
type IsolationMode string

const (
	IsolationModeFull       IsolationMode = "full"        // no traffic in or out
	IsolationModeInbound    IsolationMode = "inbound_only" // only allow inbound from allowlist
	IsolationModeOutbound   IsolationMode = "outbound_blocked"
	IsolationModeQuarantine IsolationMode = "quarantine"   // only management VLAN
)

// -------------------- Constructor --------------------

// NewTrafficShield creates a new TrafficShield with default detection thresholds.
func NewTrafficShield() *TrafficShield {
	return &TrafficShield{
		alerts:      make([]PortScanAlert, 0, 128),
		mitigations: make(map[string]*DDoSMitigation, 8),
		policies:    make([]IsolationPolicy, 0, 16),
		flowStats:   &FlowStats{WindowStart: time.Now()},
		thresholds: DetectionThresholds{
			PortScanMaxUniquePorts: 10,
			PortScanTimeWindow:     60 * time.Second,
			DDoSPacketsPerSecond:    50000,
			DDoSBytesPerSecond:      100_000_000,
			DDoSConnPerSecond:       1000,
			SYNRatioThreshold:       0.8,
		},
	}
}

// -------------------- Core Methods --------------------

// DetectPortScan examines connection attempts and flags port scanning behavior.
// It tracks unique ports touched by a source IP within a time window.
func (s *TrafficShield) DetectPortScan(ctx context.Context, sourceIP, targetIP net.IP, port int) (*PortScanAlert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find existing recent alerts from this source.
	now := time.Now()
	windowStart := now.Add(-s.thresholds.PortScanTimeWindow)

	// Collect ports this source has hit recently (from existing alerts).
	var alert *PortScanAlert
	for i := range s.alerts {
		a := &s.alerts[i]
		if a.SourceIP.Equal(sourceIP) && a.DetectedAt.After(windowStart) {
			alert = a
			break
		}
	}

	if alert == nil {
		alert = &PortScanAlert{
			ID:         "ps-" + sourceIP.String() + "-" + now.Format("20060102150405"),
			SourceIP:   sourceIP,
			TargetIP:   targetIP,
			ScannedPorts: []int{port},
			ScanType:   ScanTypeTCPConnect,
			Severity:   "low",
			DetectedAt: now,
		}
		s.alerts = append(s.alerts, *alert)
	} else {
		// Add port if not already recorded.
		alreadySeen := false
		for _, p := range alert.ScannedPorts {
			if p == port {
				alreadySeen = true
				break
			}
		}
		if !alreadySeen {
			alert.ScannedPorts = append(alert.ScannedPorts, port)
		}
	}

	// Escalate severity based on number of unique ports touched.
	uniquePorts := len(alert.ScannedPorts)
	switch {
	case uniquePorts >= s.thresholds.PortScanMaxUniquePorts:
		alert.Severity = "high"
		alert.AutoBlocked = true
	case uniquePorts >= s.thresholds.PortScanMaxUniquePorts/2:
		alert.Severity = "medium"
	}

	if uniquePorts >= s.thresholds.PortScanMaxUniquePorts {
		return alert, true
	}
	return alert, false
}

// MitigateDDoS activates DDoS mitigation for an ongoing attack.
func (s *TrafficShield) MitigateDDoS(ctx context.Context, attack DDoSMitigation) (*DDoSMitigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if attack.ID == "" {
		attack.ID = "ddos-" + attack.TargetIP.String() + "-" + time.Now().Format("20060102150405")
	}
	if attack.StartTime.IsZero() {
		attack.StartTime = time.Now()
	}
	if attack.Status == "" {
		attack.Status = MitigationStatusActive
	}

	// Determine mitigation strategy based on attack type.
	switch attack.AttackType {
	case "syn_flood":
		attack.MitigationStrategy = "syn_cookies+rate_limit"
	case "udp_flood":
		attack.MitigationStrategy = "rate_limit+drop_udp"
	case "http_flood":
		attack.MitigationStrategy = "rate_limit+captcha"
	case "amplification":
		attack.MitigationStrategy = "drop_amplification+block_sources"
	default:
		attack.MitigationStrategy = "rate_limit"
	}

	s.mitigations[attack.ID] = &attack
	return &attack, nil
}

// ApplyIsolation enforces an isolation policy on target hosts.
func (s *TrafficShield) ApplyIsolation(ctx context.Context, policy IsolationPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = "iso-" + time.Now().Format("20060102150405")
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	policy.Enabled = true
	s.policies = append(s.policies, policy)

	// In a real implementation, this would program iptables/nftables rules.
	// Here we record the policy; enforcement integration would be wired up
	// by the caller through the provided IsolationMode.
	return nil
}

// GetAlerts returns recent port scan alerts, optionally filtered by source IP.
func (s *TrafficShield) GetAlerts(sourceIP net.IP) []PortScanAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sourceIP == nil {
		result := make([]PortScanAlert, len(s.alerts))
		copy(result, s.alerts)
		return result
	}

	var filtered []PortScanAlert
	for _, a := range s.alerts {
		if a.SourceIP.Equal(sourceIP) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// GetActiveMitigations returns all ongoing (non-resolved) DDoS mitigations.
func (s *TrafficShield) GetActiveMitigations() []DDoSMitigation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []DDoSMitigation
	for _, m := range s.mitigations {
		if m.Status == MitigationStatusActive || m.Status == MitigationStatusMonitoring {
			active = append(active, *m)
		}
	}
	return active
}

// ResolveMitigation marks a DDoS mitigation as resolved.
func (s *TrafficShield) ResolveMitigation(mitigationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.mitigations[mitigationID]
	if !ok {
		return &MitigationNotFoundError{ID: mitigationID}
	}
	m.Status = MitigationStatusResolved
	now := time.Now()
	m.EndTime = &now
	return nil
}

// GetIsolationPolicies returns all isolation policies.
func (s *TrafficShield) GetIsolationPolicies() []IsolationPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]IsolationPolicy, len(s.policies))
	copy(result, s.policies)
	return result
}

// RemoveIsolation disables and removes an isolation policy by ID.
func (s *TrafficShield) RemoveIsolation(policyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.policies {
		if p.ID == policyID {
			s.policies = append(s.policies[:i], s.policies[i+1:]...)
			return nil
		}
	}
	return &PolicyNotFoundError{ID: policyID}
}

// SetThresholds updates the detection thresholds for anomaly detection.
func (s *TrafficShield) SetThresholds(t DetectionThresholds) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.thresholds = t
}

// UpdateFlowStats records traffic flow statistics and checks for anomalies.
func (s *TrafficShield) UpdateFlowStats(packets, bytes, synConns, totalConns uint64) (*DDoSMitigation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.flowStats.TotalPackets += packets
	s.flowStats.TotalBytes += bytes
	s.flowStats.TotalSYN += synConns
	s.flowStats.TotalConnections += totalConns

	elapsed := time.Since(s.flowStats.WindowStart).Seconds()
	if elapsed <= 0 {
		return nil, false
	}

	pps := uint64(float64(packets) / elapsed)
	bps := uint64(float64(bytes) / elapsed)
	cps := uint64(float64(totalConns) / elapsed)

	var attacked bool
	var mitigation DDoSMitigation

	if pps > s.thresholds.DDoSPacketsPerSecond {
		mitigation.AttackType = "udp_flood"
		attacked = true
	} else if bps > s.thresholds.DDoSBytesPerSecond {
		mitigation.AttackType = "bandwidth_flood"
		attacked = true
	} else if cps > s.thresholds.DDoSConnPerSecond {
		mitigation.AttackType = "connection_flood"
		attacked = true
	} else if totalConns > 0 {
		synRatio := float64(synConns) / float64(totalConns)
		if synRatio > s.thresholds.SYNRatioThreshold {
			mitigation.AttackType = "syn_flood"
			attacked = true
		}
	}

	if attacked {
		mitigation.PeakPPS = pps
		mitigation.PeakBPS = bps
		mitigation.StartTime = time.Now()
		mitigation.Status = MitigationStatusActive
	}

	if attacked {
		return &mitigation, true
	}
	return nil, false
}

// -------------------- Errors --------------------

// MitigationNotFoundError is returned when a mitigation ID is not found.
type MitigationNotFoundError struct {
	ID string
}

func (e *MitigationNotFoundError) Error() string {
	return "mitigation not found: " + e.ID
}

// PolicyNotFoundError is returned when an isolation policy ID is not found.
type PolicyNotFoundError struct {
	ID string
}

func (e *PolicyNotFoundError) Error() string {
	return "isolation policy not found: " + e.ID
}