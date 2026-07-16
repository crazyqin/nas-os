package cost

import (
	"context"
	"testing"
	"time"
)

func TestNewTieringAnalyzer(t *testing.T) {
	analysis := TieringCostAnalysis{
		HotStorageCost:  0.10,
		WarmStorageCost: 0.05,
		ColdStorageCost: 0.01,
		HotAccessCost:   0.001,
		WarmAccessCost:  0.005,
		ColdAccessCost:  0.01,
	}
	analyzer := NewTieringAnalyzer(analysis)
	if analyzer == nil {
		t.Fatal("NewTieringAnalyzer returned nil")
	}
	if len(analyzer.tiers) != 3 {
		t.Errorf("expected 3 tiers, got %d", len(analyzer.tiers))
	}
}

func TestDetermineOptimalTier(t *testing.T) {
	analysis := TieringCostAnalysis{
		HotStorageCost:  0.10,
		WarmStorageCost: 0.05,
		ColdStorageCost: 0.01,
	}
	analyzer := NewTieringAnalyzer(analysis)

	tests := []struct {
		accessFreq float64
		expected   string
	}{
		{15.0, "hot"},
		{5.0, "warm"},
		{0.5, "cold"},
	}

	for _, tt := range tests {
		pattern := AccessPattern{AccessFreq: tt.accessFreq}
		result := analyzer.determineOptimalTier(pattern)
		if result != tt.expected {
			t.Errorf("accessFreq %f: expected %s, got %s", tt.accessFreq, tt.expected, result)
		}
	}
}

func TestAnalyzeDataTiering(t *testing.T) {
	analysis := TieringCostAnalysis{
		HotStorageCost:  0.10,
		WarmStorageCost: 0.05,
		ColdStorageCost: 0.01,
		HotAccessCost:   0.001,
		WarmAccessCost:  0.005,
		ColdAccessCost:  0.01,
	}
	analyzer := NewTieringAnalyzer(analysis)

	patterns := []AccessPattern{
		{
			Path:         "/data/active",
			DataVolumeGB: 100,
			AccessCount:  1000,
			AccessFreq:   20.0,
			LastAccess:   time.Now(),
		},
		{
			Path:         "/data/archive",
			DataVolumeGB: 500,
			AccessCount:  10,
			AccessFreq:   0.1,
			LastAccess:   time.Now().Add(-30 * 24 * time.Hour),
		},
	}

	report := analyzer.AnalyzeDataTiering(context.Background(), patterns)

	if report.CurrentCost <= 0 {
		t.Error("CurrentCost should be positive")
	}
	if report.OptimizedCost <= 0 {
		t.Error("OptimizedCost should be positive")
	}
	if len(report.Recommendations) < 1 {
		t.Error("Should have at least one recommendation")
	}
}

func TestCalculateTierCost(t *testing.T) {
	analysis := TieringCostAnalysis{
		HotStorageCost:  0.10,
		WarmStorageCost: 0.05,
		ColdStorageCost: 0.01,
		HotAccessCost:   0.001,
		WarmAccessCost:  0.005,
		ColdAccessCost:  0.01,
	}
	analyzer := NewTieringAnalyzer(analysis)

	pattern := AccessPattern{
		DataVolumeGB: 100,
		AccessCount:  100,
	}

	hotCost := analyzer.calculateTierCost(pattern, "hot")
	warmCost := analyzer.calculateTierCost(pattern, "warm")
	coldCost := analyzer.calculateTierCost(pattern, "cold")

	if hotCost <= warmCost {
		t.Error("Hot tier should be more expensive than warm")
	}
	if warmCost <= coldCost {
		t.Error("Warm tier should be more expensive than cold")
	}
}

func TestCompareCloudStorage(t *testing.T) {
	analysis := TieringCostAnalysis{}
	analyzer := NewTieringAnalyzer(analysis)

	report := analyzer.CompareCloudStorage(context.Background(), 1000, 0.05)

	if report.LocalCost <= 0 {
		t.Error("LocalCost should be positive")
	}
	if report.CloudHotCost <= 0 {
		t.Error("CloudHotCost should be positive")
	}
}

func TestGetReason(t *testing.T) {
	analysis := TieringCostAnalysis{}
	analyzer := NewTieringAnalyzer(analysis)

	tests := []struct {
		tier     string
		expected string
	}{
		{"cold", "Low access frequency"},
		{"warm", "Moderate access frequency"},
		{"hot", "High access frequency"},
	}

	for _, tt := range tests {
		pattern := AccessPattern{}
		reason := analyzer.getReason(pattern, tt.tier)
		if !tieringContains(reason, tt.expected) {
			t.Errorf("tier %s: reason should contain '%s', got '%s'", tt.tier, tt.expected, reason)
		}
	}
}

func tieringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
