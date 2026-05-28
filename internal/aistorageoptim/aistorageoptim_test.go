package aistorageoptim

import (
	"testing"
	"time"
)

func TestDefaultTieringPolicy(t *testing.T) {
	policy := DefaultTieringPolicy()

	if policy.AccessFrequencyWeight != 0.4 {
		t.Errorf("expected 0.4 access frequency weight, got %f", policy.AccessFrequencyWeight)
	}
	if policy.FileSizeWeight != 0.3 {
		t.Errorf("expected 0.3 file size weight, got %f", policy.FileSizeWeight)
	}
	if policy.IOPatternWeight != 0.2 {
		t.Errorf("expected 0.2 IO pattern weight, got %f", policy.IOPatternWeight)
	}
	if policy.TimeDecayWeight != 0.1 {
		t.Errorf("expected 0.1 time decay weight, got %f", policy.TimeDecayWeight)
	}
	if policy.NVMePromoteThreshold != 80.0 {
		t.Errorf("expected 80.0 NVMe threshold, got %f", policy.NVMePromoteThreshold)
	}
	if policy.SSDPromoteThreshold != 50.0 {
		t.Errorf("expected 50.0 SSD threshold, got %f", policy.SSDPromoteThreshold)
	}
	if policy.HDDDemoteThreshold != 20.0 {
		t.Errorf("expected 20.0 HDD threshold, got %f", policy.HDDDemoteThreshold)
	}
	if policy.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", policy.BatchSize)
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if manager.IsRunning() {
		t.Error("expected manager to not be running initially")
	}
}

func TestRecordAccess(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	now := time.Now()
	manager.RecordAccess("/data/file1.dat", 1024*1024, "dat", 4096, 0)
	manager.RecordAccess("/data/file1.dat", 1024*1024, "dat", 8192, 0)

	stats := manager.GetFileStats("/data/file1.dat")
	if stats == nil {
		t.Fatal("expected file stats to exist")
	}
	if stats.AccessCount != 2 {
		t.Errorf("expected 2 access count, got %d", stats.AccessCount)
	}
	if stats.TotalBytesRead != 12288 {
		t.Errorf("expected 12288 bytes read, got %d", stats.TotalBytesRead)
	}
	if stats.LastAccessTime.Before(now) {
		t.Error("expected last access time to be recent")
	}
}

func TestAnalyzeAndOptimize(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Add hot file
	for i := 0; i < 10; i++ {
		manager.RecordAccess("/data/hotfile.dat", 1024*1024, "dat", 4096, 0)
	}

	// Add cold file
	manager.RecordAccess("/data/coldfile.dat", 1024*1024*100, "dat", 4096, 0)

	// Set tiers
	manager.SetFileTier("/data/hotfile.dat", TierHDD)
	manager.SetFileTier("/data/coldfile.dat", TierNVMe)

	decisions, stats := manager.AnalyzeAndOptimize("", true)

	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}

	// Hot file should be promoted
	foundPromote := false
	for _, d := range decisions {
		if d.FilePath == "/data/hotfile.dat" && d.Action == "promote" {
			foundPromote = true
		}
	}
	if !foundPromote {
		t.Error("expected hotfile to be promoted")
	}
}

func TestOptimizerCalculateScore(t *testing.T) {
	policy := DefaultTieringPolicy()
	optimizer := NewOptimizer(policy)

	now := time.Now()
	stats := &FileAccessStats{
		FilePath:        "/data/file.dat",
		FileSize:        1024 * 1024, // 1MB
		AccessCount:     100,
		AccessFrequency: 10, // 10 per hour
		LastAccessTime:  now.Add(-1 * time.Hour),
		FirstAccessTime: now.Add(-24 * time.Hour),
		IOPattern:       IOPatternRandom,
	}

	score := optimizer.CalculateScore(stats, now)

	if score.Score <= 0 || score.Score > 100 {
		t.Errorf("expected score between 0-100, got %f", score.Score)
	}
	if score.RecommendedTier == "" {
		t.Error("expected recommended tier to be set")
	}
	if score.Priority < 1 || score.Priority > 10 {
		t.Errorf("expected priority between 1-10, got %d", score.Priority)
	}
}

func TestPredictAccessPattern(t *testing.T) {
	predictor := NewPredictor(100)
	now := time.Now()

	// Hot data - high frequency, recent access
	hotStats := &FileAccessStats{
		AccessCount:     1000,
		AccessFrequency: 50,
		LastAccessTime:  now.Add(-1 * time.Hour),
	}
	pattern := predictor.PredictAccessPattern(hotStats, now)
	if pattern != PatternHot {
		t.Errorf("expected hot pattern, got %s", pattern)
	}

	// Cold data
	coldStats := &FileAccessStats{
		AccessCount:     10,
		AccessFrequency: 0.1,
		LastAccessTime:  now.Add(-7 * 24 * time.Hour),
	}
	pattern = predictor.PredictAccessPattern(coldStats, now)
	if pattern != PatternCold && pattern != PatternArchive {
		t.Errorf("expected cold or archive pattern, got %s", pattern)
	}
}

func TestDetectIOPattern(t *testing.T) {
	predictor := NewPredictor(100)

	// Burst IO - high variance with spike
	burstStats := &FileAccessStats{
		Windows: []AccessWindow{
			{Timestamp: time.Now().Add(-3 * time.Hour), Count: 1, Bytes: 256},
			{Timestamp: time.Now().Add(-2 * time.Hour), Count: 1, Bytes: 256},
			{Timestamp: time.Now().Add(-1 * time.Hour), Count: 50, Bytes: 10240},
			{Timestamp: time.Now(), Count: 1, Bytes: 256},
		},
	}
	pattern := predictor.DetectIOPattern(burstStats)
	if pattern != IOPatternBurst {
		t.Errorf("expected burst pattern for high variance, got %s", pattern)
	}

	// Sequential IO - low variance
	sequentialStats := &FileAccessStats{
		Windows: []AccessWindow{
			{Timestamp: time.Now().Add(-3 * time.Hour), Count: 10, Bytes: 1024},
			{Timestamp: time.Now().Add(-2 * time.Hour), Count: 11, Bytes: 1024},
			{Timestamp: time.Now().Add(-1 * time.Hour), Count: 9, Bytes: 1024},
			{Timestamp: time.Now(), Count: 10, Bytes: 1024},
		},
	}
	pattern = predictor.DetectIOPattern(sequentialStats)
	if pattern != IOPatternSequential {
		t.Errorf("expected sequential pattern for low variance, got %s", pattern)
	}
}

func TestMakeDecision(t *testing.T) {
	policy := DefaultTieringPolicy()
	optimizer := NewOptimizer(policy)

	// High score file - should promote to NVMe
	highScore := OptimizationScore{
		FilePath:        "/data/hotfile.dat",
		CurrentTier:     TierHDD,
		RecommendedTier: TierNVMe,
		Score:           85,
		Priority:        8,
		Reason:          "high frequency access",
	}

	decision := optimizer.MakeDecision(highScore)
	if decision.Action != "promote" {
		t.Errorf("expected promote action, got %s", decision.Action)
	}
	if decision.ToTier != TierNVMe {
		t.Errorf("expected to tier nvme, got %s", decision.ToTier)
	}

	// Same tier file - should keep
	sameScore := OptimizationScore{
		FilePath:        "/data/file.dat",
		CurrentTier:     TierSSD,
		RecommendedTier: TierSSD,
		Score:           60,
		Priority:        1,
		Reason:          "appropriate tier",
	}

	decision = optimizer.MakeDecision(sameScore)
	if decision.Action != "keep" {
		t.Errorf("expected keep action, got %s", decision.Action)
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalFiles != 0 {
		t.Errorf("expected 0 files initially, got %d", stats.TotalFiles)
	}
	if stats.TotalDecisions != 0 {
		t.Errorf("expected 0 decisions initially, got %d", stats.TotalDecisions)
	}
}

func TestUpdatePolicy(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	newPolicy := DefaultTieringPolicy()
	newPolicy.AccessFrequencyWeight = 0.5
	newPolicy.NVMePromoteThreshold = 85.0

	manager.UpdatePolicy(newPolicy)

	policy := manager.GetPolicy()
	if policy.AccessFrequencyWeight != 0.5 {
		t.Errorf("expected 0.5 access frequency weight, got %f", policy.AccessFrequencyWeight)
	}
	if policy.NVMePromoteThreshold != 85.0 {
		t.Errorf("expected 85.0 NVMe threshold, got %f", policy.NVMePromoteThreshold)
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultManagerConfig()
	config.Policy.AnalysisInterval = 100 * time.Millisecond
	manager := NewManager(config)

	manager.Start()
	if !manager.IsRunning() {
		t.Error("expected manager to be running")
	}

	// Double start should be no-op
	manager.Start()

	manager.Stop()
	if manager.IsRunning() {
		t.Error("expected manager to be stopped")
	}
}

func TestPredictNextAccessTime(t *testing.T) {
	predictor := NewPredictor(100)
	now := time.Now()

	stats := &FileAccessStats{
		LastAccessTime: now.Add(-1 * time.Hour),
		Windows: []AccessWindow{
			{Timestamp: now.Add(-3 * time.Hour), Count: 5, Bytes: 1024},
			{Timestamp: now.Add(-2 * time.Hour), Count: 5, Bytes: 1024},
			{Timestamp: now.Add(-1 * time.Hour), Count: 5, Bytes: 1024},
		},
	}

	predicted, confidence := predictor.PredictNextAccessTime(stats, now)

	if predicted.IsZero() {
		t.Error("expected non-zero predicted time")
	}
	if confidence < 0 || confidence > 1 {
		t.Errorf("expected confidence between 0-1, got %f", confidence)
	}
}

func TestGetMigrationHistory(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	history := manager.GetMigrationHistory()
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d records", len(history))
	}
}

func TestSetFileTier(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	manager.RecordAccess("/data/file.dat", 1024, "dat", 100, 0)
	manager.SetFileTier("/data/file.dat", TierSSD)

	stats := manager.GetFileStats("/data/file.dat")
	if stats == nil {
		t.Fatal("expected file stats")
	}
	if stats.CurrentTier != TierSSD {
		t.Errorf("expected SSD tier, got %s", stats.CurrentTier)
	}
}

func TestBatchOptimize(t *testing.T) {
	policy := DefaultTieringPolicy()
	optimizer := NewOptimizer(policy)

	now := time.Now()
	statsList := []*FileAccessStats{
		{
			FilePath:        "/data/hot.dat",
			FileSize:        1024 * 1024,
			AccessCount:     100,
			AccessFrequency: 20,
			LastAccessTime:  now.Add(-1 * time.Hour),
			CurrentTier:     TierHDD,
			IOPattern:       IOPatternRandom,
		},
		{
			FilePath:        "/data/cold.dat",
			FileSize:        1024 * 1024 * 100,
			AccessCount:     5,
			AccessFrequency: 0.01,
			LastAccessTime:  now.Add(-30 * 24 * time.Hour),
			CurrentTier:     TierNVMe,
			IOPattern:       IOPatternSequential,
		},
	}

	decisions, stats := optimizer.BatchOptimize(statsList, now)

	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}

	// Verify decisions
	if len(decisions) == 0 {
		t.Error("expected at least one decision")
	}
}
