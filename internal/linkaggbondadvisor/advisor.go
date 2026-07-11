package linkaggbondadvisor

import (
	"math"
	"time"
)

// BondMode constants for Linux bonding driver modes.
const (
	BondModeBalanceRR    = "balance-rr"
	BondModeActiveBackup  = "active-backup"
	BondModeBalanceXOR   = "balance-xor"
	BondModeBroadcast     = "broadcast"
	BondMode8023ad        = "802.3ad"
	BondModeBalanceTLB    = "balance-tlb"
	BondModeBalanceALB    = "balance-alb"
)

// BondSignal holds the current state of a link aggregation bond for analysis.
type BondSignal struct {
	InterfaceName      string
	BondMode           string
	SlaveCount         int
	ActiveSlaves       int
	ThroughputMbps     float64
	PacketLoss         float64
	LatencyMs          float64
	FailoverCount      int64
	LastFailoverTime   time.Time
	RecommendedMode    string
	EfficiencyScore     float64
}

// BondRecommendation represents a single bond improvement suggestion.
type BondRecommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// MisconfigResult represents a detected configuration error.
type MisconfigResult struct {
	ID          string
	Severity    string // "critical", "warning", "info"
	Description string
	Suggestion  string
}

// Priority ranking values (lower index = higher priority).
var priorityRank = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
	"info":     4,
}

// validBondModes is the set of recognised Linux bonding modes.
var validBondModes = map[string]bool{
	BondModeBalanceRR:   true,
	BondModeActiveBackup: true,
	BondModeBalanceXOR:  true,
	BondModeBroadcast:   true,
	BondMode8023ad:      true,
	BondModeBalanceTLB:  true,
	BondModeBalanceALB:  true,
}

// AnalyzeBond examines the bond signal and returns the recommended mode plus
// ordered recommendations.
//
// Recommendation logic:
//   - High throughput (>800 Mbps) → 802.3ad (LACP)
//   - High availability (failover-heavy) → active-backup
//   - Balanced load → balance-tlb
//   - Low cost / simple round-robin → balance-rr
func AnalyzeBond(s BondSignal) (string, []BondRecommendation) {
	var recs []BondRecommendation
	recommended := ""

	// Validate bond mode
	if !validBondModes[s.BondMode] {
		recs = append(recs, BondRecommendation{
			ID:       "invalid-bond-mode",
			Title:    "Invalid Bond Mode",
			Priority: "critical",
			Action:   "Set the bond mode to one of: balance-rr, active-backup, balance-xor, broadcast, 802.3ad, balance-tlb, balance-alb.",
			Reason:   "The current bond mode is not recognised by the Linux bonding driver.",
		})
	}

	// Slave count checks
	if s.SlaveCount < 2 {
		recs = append(recs, BondRecommendation{
			ID:       "add-slaves",
			Title:    "Add at Least Two Slave Interfaces",
			Priority: "critical",
			Action:   "Enslave at least two network interfaces to the bond for link aggregation to function.",
			Reason:   "A bond with fewer than two slaves provides no aggregation or redundancy benefit.",
		})
	} else if s.ActiveSlaves < s.SlaveCount {
		recs = append(recs, BondRecommendation{
			ID:       "inactive-slaves",
			Title:    "Investigate Inactive Slave Interfaces",
			Priority: "high",
			Action:   "Check link status and driver state for inactive slave interfaces.",
			Reason:   "Some slave interfaces are not active, reducing the effective bandwidth and redundancy of the bond.",
		})
	}

	// Determine recommended mode based on usage profile
	switch {
	case s.ThroughputMbps >= 800:
		recommended = BondMode8023ad
		if s.BondMode != BondMode8023ad {
			recs = append(recs, BondRecommendation{
				ID:       "switch-to-8023ad",
				Title:    "Switch to 802.3ad (LACP) for High Throughput",
				Priority: "high",
				Action:   "Change the bond mode to 802.3ad and ensure the switch supports LACP.",
				Reason:   "High throughput scenarios benefit from 802.3ad dynamic link aggregation, which utilises all active slaves for traffic.",
			})
		}
	case s.FailoverCount > 5 || (!s.LastFailoverTime.IsZero() && time.Since(s.LastFailoverTime) < 24*time.Hour):
		recommended = BondModeActiveBackup
		if s.BondMode != BondModeActiveBackup {
			recs = append(recs, BondRecommendation{
				ID:       "switch-to-active-backup",
				Title:    "Switch to active-backup for High Availability",
				Priority: "high",
				Action:   "Change the bond mode to active-backup to prioritise failover reliability over throughput.",
				Reason:   "Frequent failovers indicate an unreliable link environment; active-backup provides the fastest and most reliable failover.",
			})
		}
	case s.ThroughputMbps >= 300 && s.ThroughputMbps < 800:
		recommended = BondModeBalanceTLB
		if s.BondMode != BondModeBalanceTLB {
			recs = append(recs, BondRecommendation{
				ID:       "switch-to-balance-tlb",
				Title:    "Switch to balance-tlb for Balanced Load",
				Priority: "medium",
				Action:   "Change the bond mode to balance-tlb for adaptive transmit load balancing without switch support.",
				Reason:   "balance-tlb provides a good balance of throughput and failover without requiring switch-side LACP support.",
			})
		}
	default:
		recommended = BondModeBalanceRR
		if s.BondMode != BondModeBalanceRR {
			recs = append(recs, BondRecommendation{
				ID:       "switch-to-balance-rr",
				Title:    "Switch to balance-rr for Low-Cost Round-Robin",
				Priority: "low",
				Action:   "Change the bond mode to balance-rr for simple round-robin load distribution.",
				Reason:   "For low-throughput environments, balance-rr provides basic aggregation with minimal configuration requirements.",
			})
		}
	}

	// Packet loss check
	if s.PacketLoss > 5.0 {
		recs = append(recs, BondRecommendation{
			ID:       "high-packet-loss",
			Title:    "Investigate High Packet Loss",
			Priority: "critical",
			Action:   "Check cable integrity, switch ports, and NIC drivers for sources of packet loss.",
			Reason:   "Packet loss above 5% severely impacts network performance and may indicate hardware failure.",
		})
	} else if s.PacketLoss > 1.0 {
		recs = append(recs, BondRecommendation{
			ID:       "moderate-packet-loss",
			Title:    "Monitor Packet Loss",
			Priority: "medium",
			Action:   "Monitor packet loss trends and inspect physical link quality.",
			Reason:   "Packet loss between 1-5% degrades throughput and may worsen over time.",
		})
	}

	// Latency check
	if s.LatencyMs > 10.0 {
		recs = append(recs, BondRecommendation{
			ID:       "high-latency",
			Title:    "Investigate High Latency",
			Priority: "high",
			Action:   "Check for network congestion, switch buffer bloat, or misconfigured offload settings.",
			Reason:   "Latency above 10 ms on a local bond suggests congestion or driver issues.",
		})
	}

	// Efficiency score
	s.EfficiencyScore = CalculateEfficiencyScore(s)
	if s.EfficiencyScore < 50 {
		recs = append(recs, BondRecommendation{
			ID:       "low-efficiency",
			Title:    "Improve Bond Efficiency",
			Priority: "high",
			Action:   "Review bond configuration, slave health, and switch settings to improve the efficiency score.",
			Reason:   "The bond efficiency score is below 50, indicating significant room for improvement.",
		})
	}

	// Sort by priority
	sortRecs(recs)

	return recommended, recs
}

// CalculateEfficiencyScore computes a 0-100 efficiency score based on
// throughput, packet loss, latency, and failover frequency.
//
//   - Throughput contribution (0-40 pts): scaled relative to 1000 Mbps
//   - Packet loss penalty (0-25 pts): 0% loss = full 25, 10%+ = 0
//   - Latency contribution (0-20 pts): 0 ms = full 20, 20 ms+ = 0
//   - Stability contribution (0-15 pts): 0 failovers = full 15, 10+ = 0
func CalculateEfficiencyScore(s BondSignal) float64 {
	// Throughput score (0-40)
	throughputScore := math.Min(s.ThroughputMbps/1000.0, 1.0) * 40.0

	// Packet loss score (0-25): linear penalty, 0%→25, 10%+→0
	packetLossScore := 25.0 * (1.0 - math.Min(s.PacketLoss/10.0, 1.0))
	if packetLossScore < 0 {
		packetLossScore = 0
	}

	// Latency score (0-20): linear penalty, 0ms→20, 20ms+→0
	latencyScore := 20.0 * (1.0 - math.Min(s.LatencyMs/20.0, 1.0))
	if latencyScore < 0 {
		latencyScore = 0
	}

	// Stability score (0-15): 0 failovers→15, 10+→0
	failoverScore := 15.0 * (1.0 - math.Min(float64(s.FailoverCount)/10.0, 1.0))
	if failoverScore < 0 {
		failoverScore = 0
	}

	score := throughputScore + packetLossScore + latencyScore + failoverScore
	return math.Round(score*100) / 100
}

// DetectMisconfig scans the bond signal for common configuration errors.
func DetectMisconfig(s BondSignal) []MisconfigResult {
	var results []MisconfigResult

	// 1. Invalid bond mode
	if !validBondModes[s.BondMode] {
		results = append(results, MisconfigResult{
			ID:          "invalid-mode",
			Severity:    "critical",
			Description: "Bond mode '" + s.BondMode + "' is not a valid Linux bonding mode.",
			Suggestion:  "Use one of: balance-rr, active-backup, balance-xor, broadcast, 802.3ad, balance-tlb, balance-alb.",
		})
	}

	// 2. 802.3ad requires at least 2 slaves
	if s.BondMode == BondMode8023ad && s.SlaveCount < 2 {
		results = append(results, MisconfigResult{
			ID:          "8023ad-insufficient-slaves",
			Severity:    "critical",
			Description: "802.3ad (LACP) mode requires at least 2 slave interfaces, but only " + itoa(s.SlaveCount) + " are configured.",
			Suggestion:  "Add at least one more slave interface to the bond, or switch to active-backup if only one link is available.",
		})
	}

	// 3. broadcast mode is rarely appropriate
	if s.BondMode == BondModeBroadcast {
		results = append(results, MisconfigResult{
			ID:          "broadcast-mode-inefficient",
			Severity:    "warning",
			Description: "broadcast mode duplicates all traffic across all slaves, wasting bandwidth.",
			Suggestion:  "Consider using 802.3ad or balance-tlb unless fault tolerance of every packet is strictly required.",
		})
	}

	// 4. balance-xor requires switch-side awareness
	if s.BondMode == BondModeBalanceXOR && s.SlaveCount > 1 {
		results = append(results, MisconfigResult{
			ID:          "balance-xor-switch-config",
			Severity:    "warning",
			Description: "balance-xor mode distributes traffic by MAC address hash; single-client traffic will only use one slave.",
			Suggestion:  "Ensure the traffic pattern matches the hash distribution, or consider 802.3ad for dynamic aggregation.",
		})
	}

	// 5. Active slaves less than slave count (dead links)
	if s.SlaveCount > 0 && s.ActiveSlaves < s.SlaveCount {
		results = append(results, MisconfigResult{
			ID:          "inactive-slave-detected",
			Severity:    "warning",
			Description: itoa(s.SlaveCount-s.ActiveSlaves) + " slave interface(s) are inactive.",
			Suggestion:  "Check cable connections, switch port status, and NIC link LEDs for the inactive slave(s).",
		})
	}

	// 6. 802.3ad with active slaves < slave count (LACP partner mismatch)
	if s.BondMode == BondMode8023ad && s.ActiveSlaves < s.SlaveCount {
		results = append(results, MisconfigResult{
			ID:          "8023ad-lACP-partner-mismatch",
			Severity:    "warning",
			Description: "802.3ad mode has inactive slaves; the switch may not have LACP enabled on the corresponding ports.",
			Suggestion:  "Verify that LACP (802.3ad) is enabled on all switch ports connected to the bond's slave interfaces.",
		})
	}

	// 7. balance-alb / balance-tlb with switch-side LACP configured (mismatch)
	if (s.BondMode == BondModeBalanceALB || s.BondMode == BondModeBalanceTLB) && s.SlaveCount >= 2 && s.ActiveSlaves == s.SlaveCount {
		// This is actually fine — balance-tlb/alb do not require switch support.
		// But if the user has LACP on the switch and tlb on the host, it's a mismatch.
		// We can't detect switch-side config directly, so we skip this to avoid false positives.
	}

	// 8. Zero slave count
	if s.SlaveCount == 0 {
		results = append(results, MisconfigResult{
			ID:          "no-slaves",
			Severity:    "critical",
			Description: "The bond has no slave interfaces attached.",
			Suggestion:  "Enslave at least two network interfaces to provide aggregation and redundancy.",
		})
	}

	// 9. active-backup with only 1 slave (pointless)
	if s.BondMode == BondModeActiveBackup && s.SlaveCount == 1 {
		results = append(results, MisconfigResult{
			ID:          "active-backup-single-slave",
			Severity:    "warning",
			Description: "active-backup mode with only one slave provides no failover capability.",
			Suggestion:  "Add at least one more slave interface, or remove the bond and use the interface directly.",
		})
	}

	// 10. balance-rr with high failover count (mode mismatch)
	if s.BondMode == BondModeBalanceRR && s.FailoverCount > 10 {
		results = append(results, MisconfigResult{
			ID:          "balance-rr-high-failover",
			Severity:    "warning",
			Description: "balance-rr mode has a high failover count (" + itoa64(s.FailoverCount) + "), suggesting unstable links.",
			Suggestion:  "Consider switching to active-backup for better failover resilience, or investigate the physical link stability.",
		})
	}

	return results
}

// sortRecs sorts recommendations by priority rank (stable).
func sortRecs(recs []BondRecommendation) {
	// Simple stable sort by priority rank
	// Using bubble sort for stability without import
	n := len(recs)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-1-i; j++ {
			if priorityRank[recs[j].Priority] > priorityRank[recs[j+1].Priority] {
				recs[j], recs[j+1] = recs[j+1], recs[j]
			}
		}
	}
}

// itoa converts an int to its decimal string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// itoa64 converts an int64 to its decimal string representation.
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}