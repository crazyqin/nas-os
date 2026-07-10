package hybridpoolmgr

import (
	"testing"
	"time"
)

// TestAnalyze_EmptySignal verifies that a zero-value Signal with hybrid pool
// disabled and no devices produces no recommendations.
func TestAnalyze_EmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations for empty signal, got %d: %+v", len(recs), recs)
	}
}

// TestAnalyze_EnableHybridPool checks that when hybrid pool is disabled but
// both flash and HDD devices exist, the enable-hybrid-pool recommendation fires.
func TestAnalyze_EnableHybridPool(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  false,
		FlashDeviceCount:   2,
		HDDDeviceCount:     4,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
	})
	found := false
	for _, r := range recs {
		if r.ID == "enable-hybrid-pool" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected enable-hybrid-pool priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected enable-hybrid-pool recommendation when hybrid pool disabled with flash+HDD devices")
	}
}

// TestAnalyze_DisabledHybridPoolNoDevices verifies no recommendations when
// hybrid pool is disabled and there are no devices.
func TestAnalyze_DisabledHybridPoolNoDevices(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled: false,
	})
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations when hybrid pool disabled and no devices, got %d", len(recs))
	}
}

// TestAnalyze_DisabledHybridPoolFlashOnly verifies no recommendations when
// hybrid pool is disabled and only flash devices exist (no HDD).
func TestAnalyze_DisabledHybridPoolFlashOnly(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled: false,
		FlashDeviceCount:  2,
		HDDDeviceCount:    0,
	})
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations when only flash devices present, got %d", len(recs))
	}
}

// TestAnalyze_DisabledHybridPoolHDDOnly verifies no recommendations when
// hybrid pool is disabled and only HDD devices exist (no flash).
func TestAnalyze_DisabledHybridPoolHDDOnly(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled: false,
		FlashDeviceCount:  0,
		HDDDeviceCount:    4,
	})
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations when only HDD devices present, got %d", len(recs))
	}
}

// TestAnalyze_IncreaseFlashCapacity checks the recommendation when flash
// tier ratio is below 5%.
func TestAnalyze_IncreaseFlashCapacity(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.03,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "increase-flash-capacity" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected increase-flash-capacity priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected increase-flash-capacity recommendation when FlashTierRatio < 0.05")
	}
}

// TestAnalyze_EnableHotDataTiering checks the recommendation when hot data
// is not on flash.
func TestAnalyze_EnableHotDataTiering(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     false,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "enable-hot-data-tiering" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected enable-hot-data-tiering priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected enable-hot-data-tiering recommendation when HotDataOnFlash is false")
	}
}

// TestAnalyze_RebalancePool checks the recommendation when fragmentation
// score exceeds 0.7.
func TestAnalyze_RebalancePool(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:   true,
		FlashTierRatio:      0.1,
		HotDataOnFlash:      true,
		ColdDataMigrated:    true,
		PoolUtilizationPct:  50,
		FragmentationScore:  0.8,
		AutoRebalance:       true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "rebalance-pool" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected rebalance-pool priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected rebalance-pool recommendation when FragmentationScore > 0.7")
	}
}

// TestAnalyze_ExpandPool checks the recommendation when pool utilization
// exceeds 85%.
func TestAnalyze_ExpandPool(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 90,
		AutoRebalance:      true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "expand-pool" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected expand-pool priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected expand-pool recommendation when PoolUtilizationPct > 85")
	}
}

// TestAnalyze_EnableAutoRebalance checks the recommendation when auto-rebalance
// is disabled and last rebalance was more than 7 days ago.
func TestAnalyze_EnableAutoRebalance(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      false,
		LastRebalanceAge:   8 * 24 * time.Hour,
	})
	found := false
	for _, r := range recs {
		if r.ID == "enable-auto-rebalance" {
			found = true
			if r.Priority != "low" {
				t.Errorf("expected enable-auto-rebalance priority low, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected enable-auto-rebalance recommendation when AutoRebalance=false and LastRebalanceAge > 7d")
	}
}

// TestAnalyze_EnableAutoRebalance_RecentRebalance verifies no auto-rebalance
// recommendation when last rebalance is recent.
func TestAnalyze_EnableAutoRebalance_RecentRebalance(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      false,
		LastRebalanceAge:   3 * 24 * time.Hour,
	})
	for _, r := range recs {
		if r.ID == "enable-auto-rebalance" {
			t.Error("did not expect enable-auto-rebalance recommendation when LastRebalanceAge < 7d")
		}
	}
}

// TestAnalyze_ReplaceFlashDevices checks the recommendation when flash wear
// level exceeds 80%.
func TestAnalyze_ReplaceFlashDevices(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      true,
		FlashWearLevelPct:  85,
	})
	found := false
	for _, r := range recs {
		if r.ID == "replace-flash-devices" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected replace-flash-devices priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected replace-flash-devices recommendation when FlashWearLevelPct > 80")
	}
}

// TestAnalyze_ReduceFlashTemperature checks the recommendation when flash
// temperature exceeds 70°C.
func TestAnalyze_ReduceFlashTemperature(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   true,
		PoolUtilizationPct: 50,
		AutoRebalance:      true,
		FlashTemperatureC:  75,
	})
	found := false
	for _, r := range recs {
		if r.ID == "reduce-flash-temperature" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected reduce-flash-temperature priority high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected reduce-flash-temperature recommendation when FlashTemperatureC > 70")
	}
}

// TestAnalyze_ConfigureSpecialDeviceClass checks the recommendation when no
// special device class is configured.
func TestAnalyze_ConfigureSpecialDeviceClass(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:    true,
		FlashTierRatio:       0.1,
		HotDataOnFlash:       true,
		ColdDataMigrated:     true,
		PoolUtilizationPct:   50,
		AutoRebalance:        true,
		HasSpecialDeviceClass: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "configure-special-device-class" {
			found = true
			if r.Priority != "low" {
				t.Errorf("expected configure-special-device-class priority low, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected configure-special-device-class recommendation when HasSpecialDeviceClass is false")
	}
}

// TestAnalyze_MigrateColdData checks the recommendation when cold data has
// not been migrated.
func TestAnalyze_MigrateColdData(t *testing.T) {
	recs := Analyze(Signal{
		HybridPoolEnabled:  true,
		FlashTierRatio:     0.1,
		HotDataOnFlash:     true,
		ColdDataMigrated:   false,
		PoolUtilizationPct: 50,
		AutoRebalance:      true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "migrate-cold-data" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected migrate-cold-data priority medium, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected migrate-cold-data recommendation when ColdDataMigrated is false")
	}
}

// TestAnalyze_PriorityOrdering verifies that recommendations are sorted by
// priority: high < medium < low.
func TestAnalyze_PriorityOrdering(t *testing.T) {
	// Construct a Signal that triggers multiple recommendations across all priorities.
	signal := Signal{
		HybridPoolEnabled:    true,
		FlashTierRatio:       0.03,       // medium: increase-flash-capacity
		HotDataOnFlash:       false,      // high: enable-hot-data-tiering
		ColdDataMigrated:     false,      // medium: migrate-cold-data
		PoolUtilizationPct:   90,         // high: expand-pool
		FragmentationScore:   0.8,       // medium: rebalance-pool
		HasSpecialDeviceClass: false,     // low: configure-special-device-class
		AutoRebalance:        false,      // low: enable-auto-rebalance (with stale age)
		LastRebalanceAge:     10 * 24 * time.Hour,
		FlashWearLevelPct:    85,        // high: replace-flash-devices
		FlashTemperatureC:    75,        // high: reduce-flash-temperature
	}

	recs := Analyze(signal)

	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}

	for i := 0; i < len(recs)-1; i++ {
		rankI := priorityRank(recs[i].Priority)
		rankJ := priorityRank(recs[i+1].Priority)
		if rankI > rankJ {
			t.Errorf("recommendations not sorted: %s (rank %d) before %s (rank %d) at index %d-%d",
				recs[i].Priority, rankI, recs[i+1].Priority, rankJ, i, i+1)
		}
	}
}

// TestAnalyze_AllRecommendations verifies that a Signal triggering all
// possible recommendations produces the expected count.
func TestAnalyze_AllRecommendations(t *testing.T) {
	signal := Signal{
		HybridPoolEnabled:    true,
		FlashTierRatio:       0.03,
		HotDataOnFlash:       false,
		ColdDataMigrated:     false,
		PoolUtilizationPct:   90,
		FragmentationScore:   0.8,
		HasSpecialDeviceClass: false,
		AutoRebalance:        false,
		LastRebalanceAge:     10 * 24 * time.Hour,
		FlashWearLevelPct:    85,
		FlashTemperatureC:    75,
	}

	recs := Analyze(signal)
	expectedIDs := []string{
		"increase-flash-capacity",
		"enable-hot-data-tiering",
		"migrate-cold-data",
		"expand-pool",
		"rebalance-pool",
		"configure-special-device-class",
		"enable-auto-rebalance",
		"replace-flash-devices",
		"reduce-flash-temperature",
	}

	if len(recs) != len(expectedIDs) {
		t.Fatalf("expected %d recommendations, got %d: %+v", len(expectedIDs), len(recs), recs)
	}

	recMap := make(map[string]bool, len(recs))
	for _, r := range recs {
		recMap[r.ID] = true
	}
	for _, id := range expectedIDs {
		if !recMap[id] {
			t.Errorf("missing expected recommendation: %s", id)
		}
	}
}