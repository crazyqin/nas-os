package linkaggbondadvisor

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AnalyzeBond — recommendation tests
// ---------------------------------------------------------------------------

func TestAnalyzeBondHighThroughputRecommends8023ad(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 950,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondMode8023ad {
		t.Errorf("expected recommendation 802.3ad, got %s", rec)
	}
	// No recommendations for a well-configured bond
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations, got %d", len(recs))
	}
}

func TestAnalyzeBondHighThroughputWrongMode(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondModeBalanceRR,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 900,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondMode8023ad {
		t.Errorf("expected recommendation 802.3ad, got %s", rec)
	}
	found := false
	for _, r := range recs {
		if r.ID == "switch-to-8023ad" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected switch-to-8023ad recommendation")
	}
}

func TestAnalyzeBondHighAvailabilityRecommendsActiveBackup(t *testing.T) {
	s := BondSignal{
		InterfaceName:    "bond0",
		BondMode:         BondModeActiveBackup,
		SlaveCount:       2,
		ActiveSlaves:     2,
		ThroughputMbps:   100,
		PacketLoss:       0,
		LatencyMs:        0.5,
		FailoverCount:    8,
		LastFailoverTime: time.Now().Add(-1 * time.Hour),
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondModeActiveBackup {
		t.Errorf("expected recommendation active-backup, got %s", rec)
	}
	// failover count > 5 but mode is correct → no switch recommendation
	for _, r := range recs {
		if r.ID == "switch-to-active-backup" {
			t.Error("did not expect switch-to-active-backup when already in active-backup")
		}
	}
}

func TestAnalyzeBondHighAvailabilityWrongMode(t *testing.T) {
	s := BondSignal{
		InterfaceName:    "bond0",
		BondMode:         BondModeBalanceRR,
		SlaveCount:       2,
		ActiveSlaves:     2,
		ThroughputMbps:   100,
		PacketLoss:       0,
		LatencyMs:        0.5,
		FailoverCount:    8,
		LastFailoverTime: time.Now().Add(-1 * time.Hour),
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondModeActiveBackup {
		t.Errorf("expected recommendation active-backup, got %s", rec)
	}
	found := false
	for _, r := range recs {
		if r.ID == "switch-to-active-backup" {
			found = true
		}
	}
	if !found {
		t.Error("expected switch-to-active-backup recommendation")
	}
}

func TestAnalyzeBondBalancedRecommendsBalanceTLB(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondModeBalanceTLB,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 500,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, _ := AnalyzeBond(s)
	if rec != BondModeBalanceTLB {
		t.Errorf("expected recommendation balance-tlb, got %s", rec)
	}
}

func TestAnalyzeBondBalancedWrongMode(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondModeActiveBackup,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 500,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondModeBalanceTLB {
		t.Errorf("expected recommendation balance-tlb, got %s", rec)
	}
	found := false
	for _, r := range recs {
		if r.ID == "switch-to-balance-tlb" {
			found = true
		}
	}
	if !found {
		t.Error("expected switch-to-balance-tlb recommendation")
	}
}

func TestAnalyzeBondLowCostRecommendsBalanceRR(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondModeBalanceRR,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 100,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, _ := AnalyzeBond(s)
	if rec != BondModeBalanceRR {
		t.Errorf("expected recommendation balance-rr, got %s", rec)
	}
}

func TestAnalyzeBondLowCostWrongMode(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 100,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	rec, recs := AnalyzeBond(s)
	if rec != BondModeBalanceRR {
		t.Errorf("expected recommendation balance-rr, got %s", rec)
	}
	found := false
	for _, r := range recs {
		if r.ID == "switch-to-balance-rr" {
			found = true
		}
	}
	if !found {
		t.Error("expected switch-to-balance-rr recommendation")
	}
}

func TestAnalyzeBondInvalidMode(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       "invalid-mode",
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 100,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "invalid-bond-mode" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected priority critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected invalid-bond-mode recommendation")
	}
}

func TestAnalyzeBondInsufficientSlaves(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondModeActiveBackup,
		SlaveCount:     1,
		ActiveSlaves:   1,
		ThroughputMbps: 100,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "add-slaves" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected priority critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected add-slaves recommendation")
	}
}

func TestAnalyzeBondInactiveSlaves(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     4,
		ActiveSlaves:   2,
		ThroughputMbps: 900,
		PacketLoss:     0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "inactive-slaves" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected inactive-slaves recommendation")
	}
}

func TestAnalyzeBondHighPacketLoss(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 900,
		PacketLoss:     7.0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "high-packet-loss" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected priority critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected high-packet-loss recommendation")
	}
}

func TestAnalyzeBondModeratePacketLoss(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 900,
		PacketLoss:     3.0,
		LatencyMs:      0.5,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "moderate-packet-loss" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected moderate-packet-loss recommendation")
	}
}

func TestAnalyzeBondHighLatency(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 900,
		PacketLoss:     0,
		LatencyMs:      15.0,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "high-latency" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected high-latency recommendation")
	}
}

func TestAnalyzeBondPriorityOrdering(t *testing.T) {
	s := BondSignal{
		BondMode:       "invalid",
		SlaveCount:     1,
		ActiveSlaves:   1,
		ThroughputMbps: 50,
		PacketLoss:     8.0,
		LatencyMs:      15.0,
		FailoverCount:  0,
	}
	_, recs := AnalyzeBond(s)
	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}
	for i := 0; i < len(recs)-1; i++ {
		rankI := priorityRank[recs[i].Priority]
		rankJ := priorityRank[recs[i+1].Priority]
		if rankI > rankJ {
			t.Errorf("recommendations not sorted: %s (rank %d) before %s (rank %d)",
				recs[i].Priority, rankI, recs[i+1].Priority, rankJ)
		}
	}
}

// ---------------------------------------------------------------------------
// CalculateEfficiencyScore
// ---------------------------------------------------------------------------

func TestCalculateEfficiencyScorePerfect(t *testing.T) {
	s := BondSignal{
		ThroughputMbps: 1000,
		PacketLoss:     0,
		LatencyMs:      0,
		FailoverCount:  0,
	}
	score := CalculateEfficiencyScore(s)
	if score != 100.0 {
		t.Errorf("expected 100.0, got %.2f", score)
	}
}

func TestCalculateEfficiencyScoreZeroThroughput(t *testing.T) {
	s := BondSignal{
		ThroughputMbps: 0,
		PacketLoss:     0,
		LatencyMs:      0,
		FailoverCount:  0,
	}
	score := CalculateEfficiencyScore(s)
	// 0 throughput + 25 loss + 20 latency + 15 stability = 60
	if score != 60.0 {
		t.Errorf("expected 60.0, got %.2f", score)
	}
}

func TestCalculateEfficiencyScoreHighPacketLoss(t *testing.T) {
	s := BondSignal{
		ThroughputMbps: 1000,
		PacketLoss:     10.0,
		LatencyMs:      20.0,
		FailoverCount:  10,
	}
	score := CalculateEfficiencyScore(s)
	// 40 throughput + 0 loss + 0 latency + 0 stability = 40
	if score != 40.0 {
		t.Errorf("expected 40.0, got %.2f", score)
	}
}

func TestCalculateEfficiencyScorePartial(t *testing.T) {
	s := BondSignal{
		ThroughputMbps: 500,   // 500/1000*40 = 20
		PacketLoss:     2.5,   // 25*(1-2.5/10) = 18.75
		LatencyMs:      5.0,  // 20*(1-5/20) = 15
		FailoverCount:  3,    // 15*(1-3/10) = 10.5
	}
	score := CalculateEfficiencyScore(s)
	expected := 20.0 + 18.75 + 15.0 + 10.5
	if score != expected {
		t.Errorf("expected %.2f, got %.2f", expected, score)
	}
}

func TestCalculateEfficiencyScoreClampedLoss(t *testing.T) {
	s := BondSignal{
		ThroughputMbps: 1000,
		PacketLoss:     50.0, // clamped to 10 → 0
		LatencyMs:      0,
		FailoverCount:  0,
	}
	score := CalculateEfficiencyScore(s)
	// 40 + 0 + 20 + 15 = 75
	if score != 75.0 {
		t.Errorf("expected 75.0, got %.2f", score)
	}
}

// ---------------------------------------------------------------------------
// DetectMisconfig
// ---------------------------------------------------------------------------

func TestDetectMisconfigInvalidMode(t *testing.T) {
	s := BondSignal{
		BondMode:   "not-a-real-mode",
		SlaveCount: 2,
		ActiveSlaves: 2,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "invalid-mode" {
			found = true
			if r.Severity != "critical" {
				t.Errorf("expected severity critical, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected invalid-mode misconfig")
	}
}

func TestDetectMisconfig8023adInsufficientSlaves(t *testing.T) {
	s := BondSignal{
		BondMode:     BondMode8023ad,
		SlaveCount:   1,
		ActiveSlaves: 1,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "8023ad-insufficient-slaves" {
			found = true
			if r.Severity != "critical" {
				t.Errorf("expected severity critical, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected 8023ad-insufficient-slaves misconfig")
	}
}

func TestDetectMisconfigBroadcastMode(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeBroadcast,
		SlaveCount:   2,
		ActiveSlaves: 2,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "broadcast-mode-inefficient" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity warning, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected broadcast-mode-inefficient misconfig")
	}
}

func TestDetectMisconfigNoSlaves(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeActiveBackup,
		SlaveCount:   0,
		ActiveSlaves: 0,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "no-slaves" {
			found = true
			if r.Severity != "critical" {
				t.Errorf("expected severity critical, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected no-slaves misconfig")
	}
}

func TestDetectMisconfigActiveBackupSingleSlave(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeActiveBackup,
		SlaveCount:   1,
		ActiveSlaves: 1,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "active-backup-single-slave" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity warning, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected active-backup-single-slave misconfig")
	}
}

func TestDetectMisconfig8023adLACPMismatch(t *testing.T) {
	s := BondSignal{
		BondMode:     BondMode8023ad,
		SlaveCount:   4,
		ActiveSlaves: 2,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "8023ad-lACP-partner-mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected 8023ad-lACP-partner-mismatch misconfig")
	}
}

func TestDetectMisconfigBalanceXOR(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeBalanceXOR,
		SlaveCount:   2,
		ActiveSlaves: 2,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "balance-xor-switch-config" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity warning, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected balance-xor-switch-config misconfig")
	}
}

func TestDetectMisconfigBalanceRRHighFailover(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeBalanceRR,
		SlaveCount:   2,
		ActiveSlaves: 2,
		FailoverCount: 15,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "balance-rr-high-failover" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity warning, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected balance-rr-high-failover misconfig")
	}
}

func TestDetectMisconfigCleanConfig(t *testing.T) {
	s := BondSignal{
		BondMode:     BondMode8023ad,
		SlaveCount:   2,
		ActiveSlaves: 2,
		FailoverCount: 0,
	}
	results := DetectMisconfig(s)
	if len(results) != 0 {
		t.Fatalf("expected 0 misconfigs for clean config, got %d", len(results))
	}
}

func TestDetectMisconfigInactiveSlaves(t *testing.T) {
	s := BondSignal{
		BondMode:     BondModeActiveBackup,
		SlaveCount:   3,
		ActiveSlaves: 1,
	}
	results := DetectMisconfig(s)
	found := false
	for _, r := range results {
		if r.ID == "inactive-slave-detected" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity warning, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected inactive-slave-detected misconfig")
	}
}

// ---------------------------------------------------------------------------
// Integration: AnalyzeBond + DetectMisconfig together
// ---------------------------------------------------------------------------

func TestAnalyzeBondLowEfficiency(t *testing.T) {
	s := BondSignal{
		InterfaceName:  "bond0",
		BondMode:       BondMode8023ad,
		SlaveCount:     2,
		ActiveSlaves:   2,
		ThroughputMbps: 100,
		PacketLoss:     8.0,
		LatencyMs:      18.0,
		FailoverCount:  9,
	}
	_, recs := AnalyzeBond(s)
	found := false
	for _, r := range recs {
		if r.ID == "low-efficiency" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected low-efficiency recommendation")
	}
}

func TestAnalyzeBondRecentFailoverTriggersActiveBackup(t *testing.T) {
	s := BondSignal{
		InterfaceName:    "bond0",
		BondMode:         BondModeBalanceRR,
		SlaveCount:       2,
		ActiveSlaves:     2,
		ThroughputMbps:   100,
		PacketLoss:       0,
		LatencyMs:        0.5,
		FailoverCount:    3,
		LastFailoverTime: time.Now().Add(-30 * time.Minute),
	}
	rec, _ := AnalyzeBond(s)
	// Failover within 24h triggers active-backup path
	if rec != BondModeActiveBackup {
		t.Errorf("expected recommendation active-backup due to recent failover, got %s", rec)
	}
}