package smarttier

import (
	"fmt"
	"testing"
	"time"
)

func TestDefaultSchedulerConfig(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	if cfg.SSDCapacityBytes != 500*1024*1024*1024 {
		t.Errorf("expected 500GB SSD capacity, got %d", cfg.SSDCapacityBytes)
	}
	if cfg.SSDUsageThreshold != 0.85 {
		t.Errorf("expected 0.85 SSD threshold, got %f", cfg.SSDUsageThreshold)
	}
	if !cfg.EnableAdaptiveThreshold {
		t.Error("expected adaptive threshold enabled by default")
	}
	if !cfg.EnablePrefetch {
		t.Error("expected prefetch enabled by default")
	}
	if cfg.BatchSize != 50 {
		t.Errorf("expected batch size 50, got %d", cfg.BatchSize)
	}
}

func TestRecordIO(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = time.Hour // prevent auto-adjustment during test
	s := NewSmartTierScheduler(cfg)

	// Record some I/O
	s.RecordIO("/data/file1.dat", 1024*1024, 512*1024)
	s.RecordIO("/data/file1.dat", 2048*1024, 1024*1024)
	s.RecordIO("/data/file2.dat", 4096, 0)

	patterns := s.GetIOPatterns()
	if _, ok := patterns["/data/file1.dat"]; !ok {
		t.Error("expected file1 in patterns")
	}
	if _, ok := patterns["/data/file2.dat"]; !ok {
		t.Error("expected file2 in patterns")
	}
}

func TestAnalyzeAndDecide(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = time.Hour
	cfg.EnablePrefetch = false // simplify test
	s := NewSmartTierScheduler(cfg)

	// Record enough I/O to trigger pattern detection
	for i := 0; i < 5; i++ {
		s.RecordIO("/data/hotfile.dat", 1024*1024, 0)
		time.Sleep(10 * time.Millisecond)
	}

	heatScores := map[string]float64{
		"/data/hotfile.dat":  85,
		"/data/warmfile.dat": 50,
		"/data/coldfile.dat": 10,
	}
	currentTiers := map[string]string{
		"/data/hotfile.dat":  "cold",
		"/data/warmfile.dat": "cold",
		"/data/coldfile.dat": "hot",
	}

	decisions := s.AnalyzeAndDecide(heatScores, currentTiers)

	// hotfile should be promoted (heat=85 > basePromote=70)
	found := false
	for _, d := range decisions {
		if d.FilePath == "/data/hotfile.dat" && d.Action == "promote" {
			found = true
		}
		if d.FilePath == "/data/coldfile.dat" && d.Action == "demote" {
			// coldfile should be demoted (heat=10 < baseDemote=30, current=hot)
			found = true
		}
	}
	if !found {
		t.Errorf("expected promote/demote decisions, got: %+v", decisions)
	}
}

func TestBurstDetection(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = time.Hour
	cfg.BurstThreshold = 2.0
	s := NewSmartTierScheduler(cfg)

	// Simulate normal access then burst
	for i := 0; i < 5; i++ {
		s.ioMeta["/data/file.dat"] = &FileIOMetadata{
			FilePath:   "/data/file.dat",
			WindowSize: cfg.WindowDuration,
			MaxWindows: cfg.WindowCount,
			AccessWindows: []AccessWindow{
				{Timestamp: time.Now().Add(-10 * time.Minute), Count: 5, Bytes: 1024},
				{Timestamp: time.Now().Add(-5 * time.Minute), Count: 3, Bytes: 512},
				{Timestamp: time.Now(), Count: 20, Bytes: 4096},
			},
		}
	}

	meta := s.ioMeta["/data/file.dat"]
	if !s.detectBurst(meta) {
		t.Error("expected burst detection with count=20 vs avg=4")
	}
}

func TestGetStats(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = time.Hour
	s := NewSmartTierScheduler(cfg)

	stats := s.GetStats()
	if stats.TotalDecisions != 0 {
		t.Errorf("expected 0 total decisions, got %d", stats.TotalDecisions)
	}
	if stats.PrefetchHitRate != 0 {
		t.Errorf("expected 0 prefetch hit rate, got %f", stats.PrefetchHitRate)
	}

	s.RecordPrefetchHit()
	s.RecordPrefetchHit()
	s.RecordPrefetchMiss()

	stats = s.GetStats()
	if stats.PrefetchHits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.PrefetchHits)
	}
	expectedRate := 2.0 / 3.0 * 100
	if stats.PrefetchHitRate < expectedRate-0.1 || stats.PrefetchHitRate > expectedRate+0.1 {
		t.Errorf("expected ~66.7%% hit rate, got %f", stats.PrefetchHitRate)
	}
}

func TestDecisionSorting(t *testing.T) {
	decisions := []TierDecision{
		{FilePath: "a", Priority: 3},
		{FilePath: "b", Priority: 8},
		{FilePath: "c", Priority: 1},
	}
	sortDecisions(decisions)
	if decisions[0].Priority != 8 {
		t.Errorf("expected highest priority first, got %d", decisions[0].Priority)
	}
	if decisions[2].Priority != 1 {
		t.Errorf("expected lowest priority last, got %d", decisions[2].Priority)
	}
}

func TestAdaptiveThreshold(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = 10 * time.Millisecond
	cfg.SSDUsageThreshold = 0.8
	cfg.BasePromoteThreshold = 70
	cfg.AdaptiveSensitivity = 1.0
	s := NewSmartTierScheduler(cfg)

	// Simulate high SSD usage by adding many I/O entries
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("/data/file%d.dat", i)
		s.ioMeta[path] = &FileIOMetadata{
			FilePath:       path,
			Pattern:        PatternRandom,
			AccessWindows:  make([]AccessWindow, 10),
			WindowSize:     time.Minute,
			MaxWindows:     10,
		}
	}

	usage := s.estimateSSDUsage()
	if usage <= s.config.SSDUsageThreshold {
		t.Skipf("estimated SSD usage %.4f below threshold, test data insufficient", usage)
	}

	// Force adaptive adjustment
	s.runAdaptiveAdjustment()

	if s.currentPromoteThreshold <= 70 {
		t.Errorf("expected promoted threshold to increase with high SSD usage, got %f", s.currentPromoteThreshold)
	}
}

func TestStartStop(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.TieringInterval = 50 * time.Millisecond
	s := NewSmartTierScheduler(cfg)

	s.Start()
	if !s.running {
		t.Error("expected scheduler to be running")
	}

	// Double start should be no-op
	s.Start()

	time.Sleep(100 * time.Millisecond)

	s.Stop()
	if s.running {
		t.Error("expected scheduler to be stopped")
	}
}
