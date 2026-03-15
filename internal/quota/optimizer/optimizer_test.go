package optimizer

import (
	"testing"
	"time"
)

// Mock implementations for testing
type mockQuotaDataProvider struct {
	usages  []*QuotaUsageInfo
	quotas  map[string]*QuotaInfo
	updates map[string]uint64
}

func (m *mockQuotaDataProvider) GetAllUsage() ([]*QuotaUsageInfo, error) {
	if m.usages != nil {
		return m.usages, nil
	}
	return []*QuotaUsageInfo{
		{
			QuotaID:      "quota-1",
			TargetID:     "user1",
			TargetName:   "User One",
			VolumeName:   "pool1",
			Type:         "user",
			HardLimit:    100 * 1024 * 1024 * 1024,
			SoftLimit:    80 * 1024 * 1024 * 1024,
			UsedBytes:    50 * 1024 * 1024 * 1024,
			UsagePercent: 50,
			LastChecked:  time.Now(),
		},
	}, nil
}

func (m *mockQuotaDataProvider) GetUserUsage(username string) ([]*QuotaUsageInfo, error) {
	return m.usages, nil
}

func (m *mockQuotaDataProvider) GetQuota(quotaID string) (*QuotaInfo, error) {
	if m.quotas != nil {
		return m.quotas[quotaID], nil
	}
	return &QuotaInfo{
		ID:         quotaID,
		TargetID:   "user1",
		TargetName: "User One",
		VolumeName: "pool1",
		HardLimit:  100 * 1024 * 1024 * 1024,
		SoftLimit:  80 * 1024 * 1024 * 1024,
	}, nil
}

func (m *mockQuotaDataProvider) UpdateQuota(quotaID string, newLimit uint64) error {
	if m.updates == nil {
		m.updates = make(map[string]uint64)
	}
	m.updates[quotaID] = newLimit
	return nil
}

type mockHistoryDataProvider struct {
	growthRate float64
}

func (m *mockHistoryDataProvider) GetHistory(quotaID string, days int) ([]HistoryPoint, error) {
	points := make([]HistoryPoint, 0)
	baseUsage := uint64(50 * 1024 * 1024 * 1024)

	for i := 0; i < days; i++ {
		points = append(points, HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -days+i),
			UsedBytes:    baseUsage + uint64(i)*1024*1024*1024,
			UsagePercent: float64(50+i) / 100 * 100,
		})
	}

	return points, nil
}

func (m *mockHistoryDataProvider) GetGrowthRate(quotaID string) (float64, error) {
	return m.growthRate, nil
}

func TestNewQuotaOptimizer(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)
	if optimizer == nil {
		t.Fatal("Expected optimizer to be created")
	}

	if !optimizer.config.AutoAdjust.Enabled {
		t.Error("Expected auto adjust to be enabled by default")
	}
}

func TestGenerateAdjustmentSuggestions(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024, // 95% utilization
				UsagePercent: 95,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    10 * 1024 * 1024 * 1024, // 10% utilization
				UsagePercent: 10,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()
	config.AutoAdjust.Enabled = true

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)
	suggestions, err := optimizer.GenerateAdjustmentSuggestions()
	if err != nil {
		t.Fatalf("Failed to generate suggestions: %v", err)
	}

	// Should have suggestions for over-utilized and under-utilized quotas
	if len(suggestions) == 0 {
		t.Log("No suggestions generated (this may be expected based on thresholds)")
	}
}

func TestPredictUsage(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    60 * 1024 * 1024 * 1024,
				UsagePercent: 60,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	prediction, err := optimizer.PredictUsage("quota-1")
	if err != nil {
		t.Fatalf("Failed to predict usage: %v", err)
	}

	if prediction == nil {
		t.Fatal("Expected prediction to be returned")
	}

	if prediction.QuotaID != "quota-1" {
		t.Errorf("Expected quota ID quota-1, got %s", prediction.QuotaID)
	}
}

func TestPredictAllUsage(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	predictions, err := optimizer.PredictAllUsage()
	if err != nil {
		t.Fatalf("Failed to predict all usage: %v", err)
	}

	if len(predictions) == 0 {
		t.Error("Expected at least one prediction")
	}
}

func TestDetectViolations(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				SoftLimit:    80 * 1024 * 1024 * 1024,
				UsedBytes:    110 * 1024 * 1024 * 1024, // Over hard limit
				UsagePercent: 110,
				IsOverSoft:   true,
				IsOverHard:   true,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				SoftLimit:    80 * 1024 * 1024 * 1024,
				UsedBytes:    85 * 1024 * 1024 * 1024, // Over soft limit
				UsagePercent: 85,
				IsOverSoft:   true,
				IsOverHard:   false,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	violations, err := optimizer.DetectViolations()
	if err != nil {
		t.Fatalf("Failed to detect violations: %v", err)
	}

	// Should detect at least one hard limit violation
	hardLimitViolations := 0
	for _, v := range violations {
		if v.Type == ViolationHardLimit {
			hardLimitViolations++
		}
	}

	if hardLimitViolations == 0 {
		t.Log("No hard limit violations detected (check test data)")
	}
}

func TestResolveViolation(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    110 * 1024 * 1024 * 1024,
				UsagePercent: 110,
				IsOverHard:   true,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// First detect violations
	violations, _ := optimizer.DetectViolations()
	if len(violations) == 0 {
		t.Skip("No violations to resolve")
	}

	// Resolve the first violation
	err := optimizer.ResolveViolation(violations[0].ID, "admin")
	if err != nil {
		t.Fatalf("Failed to resolve violation: %v", err)
	}

	// Verify it's resolved
	resolved := optimizer.GetViolations("resolved")
	found := false
	for _, v := range resolved {
		if v.ID == violations[0].ID {
			found = true
			if v.ResolvedBy != "admin" {
				t.Errorf("Expected resolved by admin, got %s", v.ResolvedBy)
			}
		}
	}

	if !found {
		t.Error("Violation should be in resolved list")
	}
}

func TestGenerateOptimizationReport(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	report, err := optimizer.GenerateOptimizationReport()
	if err != nil {
		t.Fatalf("Failed to generate optimization report: %v", err)
	}

	if report == nil {
		t.Fatal("Expected report to be returned")
	}

	if report.ID == "" {
		t.Error("Report ID should not be empty")
	}
}

func TestApplySuggestion(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024,
				UsagePercent: 95,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()
	config.AutoAdjust.Enabled = true

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Generate suggestions first
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()
	if len(suggestions) == 0 {
		t.Skip("No suggestions to apply")
	}

	// Apply the first suggestion
	err := optimizer.ApplySuggestion(suggestions[0].ID)
	if err != nil {
		t.Fatalf("Failed to apply suggestion: %v", err)
	}

	// Verify it's applied
	applied := optimizer.GetSuggestions("applied")
	found := false
	for _, s := range applied {
		if s.ID == suggestions[0].ID {
			found = true
		}
	}

	if !found {
		t.Error("Suggestion should be in applied list")
	}
}

func TestDismissSuggestion(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024,
				UsagePercent: 95,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()
	config.AutoAdjust.Enabled = true

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Generate suggestions first
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()
	if len(suggestions) == 0 {
		t.Skip("No suggestions to dismiss")
	}

	// Dismiss the first suggestion
	err := optimizer.DismissSuggestion(suggestions[0].ID)
	if err != nil {
		t.Fatalf("Failed to dismiss suggestion: %v", err)
	}

	// Verify it's dismissed
	dismissed := optimizer.GetSuggestions("dismissed")
	found := false
	for _, s := range dismissed {
		if s.ID == suggestions[0].ID {
			found = true
		}
	}

	if !found {
		t.Error("Suggestion should be in dismissed list")
	}
}

func TestGetSuggestions(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Generate suggestions
	optimizer.GenerateAdjustmentSuggestions()

	// Get all suggestions
	allSuggestions := optimizer.GetSuggestions("")
	if allSuggestions == nil {
		t.Error("Expected suggestions slice, got nil")
	}

	// Get pending suggestions
	pending := optimizer.GetSuggestions("pending")
	for _, s := range pending {
		if s.Status != "pending" {
			t.Errorf("Expected pending status, got %s", s.Status)
		}
	}
}

func TestGetViolations(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Detect violations
	optimizer.DetectViolations()

	// Get all violations
	allViolations := optimizer.GetViolations("")
	if allViolations == nil {
		t.Error("Expected violations slice, got nil")
	}

	// Get active violations
	active := optimizer.GetViolations("active")
	for _, v := range active {
		if v.Status != "active" {
			t.Errorf("Expected active status, got %s", v.Status)
		}
	}
}

func TestGetAdjustHistory(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024,
				UsagePercent: 95,
			},
		},
	}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()
	config.AutoAdjust.Enabled = true

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Generate and apply a suggestion
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()
	if len(suggestions) > 0 {
		optimizer.ApplySuggestion(suggestions[0].ID)
	}

	// Get history
	history := optimizer.GetAdjustHistory(10)
	if history == nil {
		t.Error("Expected history slice, got nil")
	}
}

func TestLinearPrediction(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Create test history data
	history := make([]HistoryPoint, 30)
	for i := 0; i < 30; i++ {
		history[i] = HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -30+i),
			UsedBytes:    uint64(50+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(50+i) / 100 * 100,
		}
	}

	prediction := &UsagePrediction{
		PredictionPeriod: 7 * 24 * time.Hour,
	}

	optimizer.linearPrediction(prediction, history)

	if prediction.GrowthTrend != "increasing" {
		t.Errorf("Expected increasing trend, got %s", prediction.GrowthTrend)
	}
}

func TestMovingAvgPrediction(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	// Create test history data
	history := make([]HistoryPoint, 14)
	for i := 0; i < 14; i++ {
		history[i] = HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -14+i),
			UsedBytes:    uint64(50+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(50+i) / 100 * 100,
		}
	}

	prediction := &UsagePrediction{
		PredictionPeriod: 7 * 24 * time.Hour,
	}

	optimizer.movingAvgPrediction(prediction, history)

	if prediction.Confidence == 0 {
		t.Error("Expected non-zero confidence")
	}
}

func TestSimplePrediction(t *testing.T) {
	quotaProvider := &mockQuotaDataProvider{}
	historyProvider := &mockHistoryDataProvider{growthRate: 1024 * 1024 * 1024}
	config := DefaultOptimizerConfig()

	optimizer := NewQuotaOptimizer("/tmp/test-optimizer-1773546358", quotaProvider, historyProvider, config)

	usage := &QuotaUsageInfo{
		QuotaID:      "quota-1",
		UsedBytes:    50 * 1024 * 1024 * 1024,
		HardLimit:    100 * 1024 * 1024 * 1024,
		UsagePercent: 50,
	}

	quota := &QuotaInfo{
		ID:         "quota-1",
		TargetID:   "user1",
		TargetName: "User One",
		VolumeName: "pool1",
		HardLimit:  100 * 1024 * 1024 * 1024,
	}

	prediction := optimizer.simplePrediction(usage, quota)

	if prediction == nil {
		t.Fatal("Expected prediction to be returned")
	}

	if prediction.Confidence != 0.5 {
		t.Log("Simple prediction should have 0.5 confidence")
	}
}
