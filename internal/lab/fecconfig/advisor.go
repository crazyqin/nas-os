// Package fecconfig implements Forward Error Correction network configuration
// inspired by TrueNAS 26 FEC support and enterprise network reliability standards.
package fecconfig

import (
	"fmt"
	"sort"
	"time"
)

// FECMode indicates the FEC encoding mode.
type FECMode string

const (
	FECNone      FECMode = "none"        // No FEC
	FECRS        FECMode = "reed_solomon" // Reed-Solomon
	FECHamming   FECMode = "hamming"      // Hamming code
	FECBCH       FECMode = "bch"           // Bose-Chaudhuri-Hocquenghem
	FECLDPC      FECMode = "ldpc"          // Low-Density Parity-Check
	FECConv      FECMode = "convolutional" // Convolutional
)

// LinkType indicates the network link type.
type LinkType string

const (
	LinkEthernet LinkType = "ethernet"
	LinkFiber    LinkType = "fiber"
	LinkBond     LinkType = "bond"
	LinkWiFi     LinkType = "wifi"
)

// Interface describes a network interface with FEC signals.
type Interface struct {
	Name            string    `json:"name"`
	LinkType        LinkType  `json:"link_type"`
	SpeedGbps       float64   `json:"speed_gbps"`
	MTU             int       `json:"mtu"`
	FECModeCurrent  FECMode   `json:"fec_mode_current"`
	FECModeRecommended FECMode `json:"fec_mode_recommended,omitempty"`
	BitErrorRate    float64   `json:"bit_error_rate"`   // errors per billion bits
	PacketLossPct   float64   `json:"packet_loss_pct"`
	BandwidthUtilPct float64  `json:"bandwidth_util_pct"`
	IsStorage       bool      `json:"is_storage"`       // iSCSI/NFS/SMB interface
	IsReplication   bool      `json:"is_replication"`    // replication target interface
	HasFECSupport   bool      `json:"has_fec_support"`
	CableLengthM     int      `json:"cable_length_m"`
}

// Signal aggregates FEC configuration signals for analysis.
type Signal struct {
	Interfaces        []Interface `json:"interfaces"`
	TotalPacketLoss   float64    `json:"total_packet_loss_pct"`
	ProtocolErrors    int        `json:"protocol_errors"`
	IsHighSpeed25G     bool       `json:"is_high_speed_25g"`
	StorageOnSameNIC   bool       `json:"storage_on_same_nic"`
	ReplicationActive  bool      `json:"replication_active"`
	FECUniversallyOff  bool       `json:"fec_universally_off"`
	LastFECReview      time.Time  `json:"last_fec_review"`
}

// Recommendation is an actionable FEC configuration suggestion.
type Recommendation struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Priority    string  `json:"priority"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
	Interface   string  `json:"interface,omitempty"`
	FECFrom     FECMode `json:"fec_from,omitempty"`
	FECTo       FECMode `json:"fec_to,omitempty"`
}

// Analyze evaluates FEC configuration signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// FEC universally off
	if s.FECUniversallyOff {
		recs = append(recs, Recommendation{
			ID:       "fec-enable-global",
			Title:    "FEC is disabled on all interfaces",
			Priority: "high",
			Action:   "Enable Forward Error Correction on high-speed and storage interfaces to reduce packet loss",
			Reason:   "No FEC on any interface; 25G+ and storage links benefit significantly from FEC",
		})
	}

	// High packet loss with FEC disabled
	if s.TotalPacketLoss > 0.1 && s.FECUniversallyOff {
		recs = append(recs, Recommendation{
			ID:       "fec-loss-detected",
			Title:    "Packet loss detected with FEC disabled",
			Priority: "critical",
			Action:   "Immediately enable FEC on interfaces with packet loss to prevent data corruption",
			Reason:   fmt.Sprintf("%.3f%% packet loss with no FEC; storage protocols like iSCSI are especially vulnerable", s.TotalPacketLoss),
		})
	}

	// Per-interface analysis
	for _, iface := range s.Interfaces {
		// High-speed interface without FEC
		if iface.SpeedGbps >= 25 && iface.FECModeCurrent == FECNone && iface.HasFECSupport {
			recs = append(recs, Recommendation{
				ID:        "fec-enable-" + iface.Name,
				Title:     fmt.Sprintf("Enable FEC on %s (%.0fG)", iface.Name, iface.SpeedGbps),
				Priority:  "high",
				Action:    fmt.Sprintf("Enable %s FEC mode on interface %s", recommendedFEC(iface), iface.Name),
				Reason:    fmt.Sprintf("%.0fG link without FEC is prone to bit errors over cable runs", iface.SpeedGbps),
				Interface: iface.Name,
				FECFrom:   FECNone,
				FECTo:     recommendedFECMode(iface),
			})
		}

		// Storage interface without FEC
		if iface.IsStorage && iface.FECModeCurrent == FECNone && iface.HasFECSupport {
			recs = append(recs, Recommendation{
				ID:        "fec-storage-" + iface.Name,
				Title:     fmt.Sprintf("Enable FEC on storage interface %s", iface.Name),
				Priority:  "critical",
				Action:    fmt.Sprintf("Enable FEC on %s to protect iSCSI/NFS/SMB traffic from bit errors", iface.Name),
				Reason:    "Storage protocols require data integrity; bit errors can cause silent data corruption",
				Interface: iface.Name,
				FECFrom:   FECNone,
				FECTo:     recommendedFECMode(iface),
			})
		}

		// Replication interface with high BER
		if iface.IsReplication && iface.BitErrorRate > 1e-9 {
			recs = append(recs, Recommendation{
				ID:        "fec-replication-" + iface.Name,
				Title:     fmt.Sprintf("High bit error rate on replication interface %s", iface.Name),
				Priority:  "high",
				Action:    fmt.Sprintf("Enable %s FEC or replace cable to reduce BER from %.2e", recommendedFECMode(iface), iface.BitErrorRate),
				Reason:    "Replication traffic needs reliable links; high BER risks data consistency",
				Interface: iface.Name,
				FECTo:     recommendedFECMode(iface),
			})
		}

		// Long cable with no FEC
		if iface.CableLengthM > 10 && iface.FECModeCurrent == FECNone && iface.HasFECSupport {
			recs = append(recs, Recommendation{
				ID:        "fec-long-cable-" + iface.Name,
				Title:     fmt.Sprintf("Long cable on %s without FEC", iface.Name),
				Priority:  "medium",
				Action:    fmt.Sprintf("Enable FEC on %s (%dm cable) to prevent signal degradation", iface.Name, iface.CableLengthM),
				Reason:    fmt.Sprintf("Cable length %dm exceeds 10m recommendation without FEC; signal integrity at risk", iface.CableLengthM),
				Interface: iface.Name,
				FECFrom:   FECNone,
				FECTo:     recommendedFECMode(iface),
			})
		}

		// High bandwidth utilization with FEC disabled
		if iface.BandwidthUtilPct > 70 && iface.FECModeCurrent == FECNone && iface.HasFECSupport {
			recs = append(recs, Recommendation{
				ID:        "fec-high-util-" + iface.Name,
				Title:     fmt.Sprintf("High bandwidth utilization on %s without FEC", iface.Name),
				Priority:  "medium",
				Action:    fmt.Sprintf("Enable FEC on %s to maintain reliability at %.0f%% utilization", iface.Name, iface.BandwidthUtilPct),
				Reason:    "High utilization increases collision/retry probability; FEC mitigates retransmissions",
				Interface: iface.Name,
				FECTo:     recommendedFECMode(iface),
			})
		}

		// WiFi interface - recommend not using for storage
		if iface.LinkType == LinkWiFi && iface.IsStorage {
			recs = append(recs, Recommendation{
				ID:        "fec-wifi-storage-" + iface.Name,
				Title:     fmt.Sprintf("Storage traffic on WiFi interface %s", iface.Name),
				Priority:  "high",
				Action:    "Move storage traffic to a wired interface; WiFi is unreliable for iSCSI/NFS",
				Reason:    "WiFi has high packet loss and no FEC support; storage on WiFi risks data corruption",
				Interface: iface.Name,
			})
		}
	}

	// Protocol errors detected
	if s.ProtocolErrors > 100 {
		recs = append(recs, Recommendation{
			ID:       "fec-protocol-errors",
			Title:    "High protocol errors detected",
			Priority: "high",
			Action:   "Check cable integrity and enable FEC on affected interfaces",
			Reason:   fmt.Sprintf("%d protocol errors suggest physical layer issues; FEC can mitigate", s.ProtocolErrors),
		})
	}

	// Storage and replication on same NIC
	if s.StorageOnSameNIC && s.ReplicationActive {
		recs = append(recs, Recommendation{
			ID:       "fec-separate-nics",
			Title:    "Storage and replication on same interface",
			Priority:  "medium",
			Action:    "Dedicate separate interfaces for storage and replication traffic",
			Reason:    "Shared interface risks contention during replication; FEC cannot resolve bandwidth competition",
		})
	}

	// Stale FEC review
	if !s.LastFECReview.IsZero() && time.Since(s.LastFECReview) > 90*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "fec-stale-review",
			Title:    "FEC configuration not reviewed in 90+ days",
			Priority:  "low",
			Action:    "Review FEC configuration after any network hardware changes",
			Reason:    "Network changes may require FEC reconfiguration; stale config risks undetected errors",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityValue(recs[i].Priority) > priorityValue(recs[j].Priority)
	})

	return recs
}

func recommendedFEC(iface Interface) string {
	return string(recommendedFECMode(iface))
}

func recommendedFECMode(iface Interface) FECMode {
	switch {
	case iface.SpeedGbps >= 100:
		return FECLDPC
	case iface.SpeedGbps >= 40:
		return FECRS
	case iface.SpeedGbps >= 25:
		return FECRS
	case iface.SpeedGbps >= 10:
		return FECBCH
	default:
		return FECHamming
	}
}

func priorityValue(p string) int {
	switch p {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}