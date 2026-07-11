package tierheatmap

import (
	"testing"
	"time"
)

func TestAnalyzeEmpty(t *testing.T) {
	result := Analyze(nil)
	if len(result.Signals) != 0 {
		t.Fatalf("expected 0 signals for empty input, got %d", len(result.Signals))
	}
	if result.Summary != "no tier data available for analysis" {
		t.Errorf("expected empty-summary message, got %q", result.Summary)
	}
}

func TestAnalyzeEmptySlice(t *testing.T) {
	result := Analyze([]TierHeatmapSignal{})
	if len(result.Signals) != 0 {
		t.Fatalf("expected 0 signals for empty slice, got %d", len(result.Signals))
	}
}

func TestAnalyzeNormalMixed(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "fast-cache",
			TierType:        TierNVMe,
			Temperature:     85,
			FileCount:       10000,
			AccessFrequency: 500,
			LastAccessTime:  now.Add(-1 * time.Hour),
		},
		{
			LayerName:       "bulk-storage",
			TierType:        TierHDD,
			Temperature:     10,
			FileCount:       50000,
			AccessFrequency: 5,
			LastAccessTime:  now.Add(-120 * 24 * time.Hour), // 120 days ago
		},
		{
			LayerName:       "ssd-pool",
			TierType:        TierSSD,
			Temperature:     20,
			FileCount:       5000,
			AccessFrequency: 10,
			LastAccessTime:  now.Add(-10 * 24 * time.Hour),
		},
		{
			LayerName:       "hot-hdd",
			TierType:        TierHDD,
			Temperature:     60,
			FileCount:       20000,
			AccessFrequency: 450,
			LastAccessTime:  now.Add(-30 * time.Minute),
		},
	}

	result := Analyze(signals)

	if len(result.Signals) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(result.Signals))
	}

	// The NVMe with high frequency should be kept.
	nvmeSig := findSignal(result.Signals, "fast-cache")
	if nvmeSig == nil {
		t.Fatal("fast-cache signal not found")
	}
	if nvmeSig.RecommendedAction != ActionKeep {
		t.Errorf("expected Keep for hot NVMe, got %s", nvmeSig.RecommendedAction)
	}
	if nvmeSig.MigrationScore < 70 {
		t.Errorf("expected high migration score for NVMe, got %.2f", nvmeSig.MigrationScore)
	}

	// The bulk-storage HDD with very low frequency and stale access should be archived.
	bulkSig := findSignal(result.Signals, "bulk-storage")
	if bulkSig == nil {
		t.Fatal("bulk-storage signal not found")
	}
	if bulkSig.RecommendedAction != ActionArchive {
		t.Errorf("expected Archive for stale HDD, got %s", bulkSig.RecommendedAction)
	}

	// The ssd-pool with low frequency should be demoted to cold.
	ssdSig := findSignal(result.Signals, "ssd-pool")
	if ssdSig == nil {
		t.Fatal("ssd-pool signal not found")
	}
	if ssdSig.RecommendedAction != ActionMoveToCold {
		t.Errorf("expected MoveToCold for cold SSD, got %s", ssdSig.RecommendedAction)
	}

	// The hot-hdd with high frequency should be promoted to hot.
	hotHddSig := findSignal(result.Signals, "hot-hdd")
	if hotHddSig == nil {
		t.Fatal("hot-hdd signal not found")
	}
	if hotHddSig.RecommendedAction != ActionMoveToHot {
		t.Errorf("expected MoveToHot for hot HDD, got %s", hotHddSig.RecommendedAction)
	}

	// Verify signals are sorted by MigrationScore descending.
	for i := 1; i < len(result.Signals); i++ {
		if result.Signals[i].MigrationScore > result.Signals[i-1].MigrationScore {
			t.Errorf("signals not sorted descending: index %d (%.2f) > index %d (%.2f)",
				i, result.Signals[i].MigrationScore, i-1, result.Signals[i-1].MigrationScore)
		}
	}

	// Verify categorization.
	if len(result.HotSignals) != 1 {
		t.Errorf("expected 1 hot signal, got %d", len(result.HotSignals))
	}
	if len(result.ColdSignals) != 1 {
		t.Errorf("expected 1 cold signal, got %d", len(result.ColdSignals))
	}
	if len(result.ArchiveSignals) != 1 {
		t.Errorf("expected 1 archive signal, got %d", len(result.ArchiveSignals))
	}
}

func TestAnalyzeAllHot(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "nvme-1",
			TierType:        TierNVMe,
			Temperature:     90,
			FileCount:       1000,
			AccessFrequency: 800,
			LastAccessTime:  now.Add(-5 * time.Minute),
		},
		{
			LayerName:       "nvme-2",
			TierType:        TierNVMe,
			Temperature:     85,
			FileCount:       2000,
			AccessFrequency: 700,
			LastAccessTime:  now.Add(-10 * time.Minute),
		},
		{
			LayerName:       "hot-hdd",
			TierType:        TierHDD,
			Temperature:     80,
			FileCount:       3000,
			AccessFrequency: 750,
			LastAccessTime:  now.Add(-3 * time.Minute),
		},
	}

	result := Analyze(signals)

	// All NVMe should be kept (already hot).
	for _, s := range result.Signals {
		if s.TierType == TierNVMe && s.RecommendedAction != ActionKeep {
			t.Errorf("expected Keep for hot NVMe %s, got %s", s.LayerName, s.RecommendedAction)
		}
	}

	// The hot HDD should be promoted to hot.
	hddSig := findSignal(result.Signals, "hot-hdd")
	if hddSig == nil {
		t.Fatal("hot-hdd signal not found")
	}
	if hddSig.RecommendedAction != ActionMoveToHot {
		t.Errorf("expected MoveToHot for hot HDD, got %s", hddSig.RecommendedAction)
	}

	if len(result.HotSignals) != 1 {
		t.Errorf("expected 1 hot signal (HDD promote), got %d", len(result.HotSignals))
	}
}

func TestAnalyzeAllCold(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "ssd-cold",
			TierType:        TierSSD,
			Temperature:     5,
			FileCount:       500,
			AccessFrequency: 2,
			LastAccessTime:  now.Add(-100 * 24 * time.Hour), // 100 days ago
		},
		{
			LayerName:       "hdd-cold",
			TierType:        TierHDD,
			Temperature:     3,
			FileCount:       1000,
			AccessFrequency: 1,
			LastAccessTime:  now.Add(-100 * 24 * time.Hour),
		},
	}

	result := Analyze(signals)

	// The SSD with low score should be demoted to cold.
	ssdSig := findSignal(result.Signals, "ssd-cold")
	if ssdSig == nil {
		t.Fatal("ssd-cold signal not found")
	}
	if ssdSig.RecommendedAction != ActionMoveToCold {
		t.Errorf("expected MoveToCold for cold SSD, got %s", ssdSig.RecommendedAction)
	}

	// The HDD with very low score and stale access should be archived.
	hddSig := findSignal(result.Signals, "hdd-cold")
	if hddSig == nil {
		t.Fatal("hdd-cold signal not found")
	}
	if hddSig.RecommendedAction != ActionArchive {
		t.Errorf("expected Archive for stale cold HDD, got %s", hddSig.RecommendedAction)
	}

	if len(result.ColdSignals) != 1 {
		t.Errorf("expected 1 cold signal, got %d", len(result.ColdSignals))
	}
	if len(result.ArchiveSignals) != 1 {
		t.Errorf("expected 1 archive signal, got %d", len(result.ArchiveSignals))
	}
}

func TestAnalyzeSingleNVMeHot(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "nvme-only",
			TierType:        TierNVMe,
			Temperature:     95,
			FileCount:       100,
			AccessFrequency: 1000,
			LastAccessTime:  now,
		},
	}

	result := Analyze(signals)

	if len(result.Signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(result.Signals))
	}

	s := result.Signals[0]
	if s.RecommendedAction != ActionKeep {
		t.Errorf("expected Keep for single hot NVMe, got %s", s.RecommendedAction)
	}
	if s.MigrationScore < 98 || s.MigrationScore > 100 {
		t.Errorf("expected migration score ~99 for peak NVMe (temp=95), got %.2f", s.MigrationScore)
	}
}

func TestAnalyzeMigrationScoreRange(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "very-hot",
			TierType:        TierNVMe,
			Temperature:     100,
			FileCount:       100,
			AccessFrequency: 1000,
			LastAccessTime:  now,
		},
		{
			LayerName:       "very-cold",
			TierType:        TierHDD,
			Temperature:     0,
			FileCount:       100,
			AccessFrequency: 0,
			LastAccessTime:  now.Add(-365 * 24 * time.Hour),
		},
	}

	result := Analyze(signals)

	for _, s := range result.Signals {
		if s.MigrationScore < 0 || s.MigrationScore > 100 {
			t.Errorf("migration score %.2f out of range [0,100] for %s", s.MigrationScore, s.LayerName)
		}
	}
}

func TestAnalyzeSummary(t *testing.T) {
	now := time.Now()
	signals := []TierHeatmapSignal{
		{
			LayerName:       "keep-me",
			TierType:        TierNVMe,
			Temperature:     80,
			FileCount:       100,
			AccessFrequency: 500,
			LastAccessTime:  now,
		},
	}

	result := Analyze(signals)

	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

// findSignal searches for a signal by layer name in a slice.
func findSignal(signals []TierHeatmapSignal, name string) *TierHeatmapSignal {
	for i := range signals {
		if signals[i].LayerName == name {
			return &signals[i]
		}
	}
	return nil
}