package experiencescore

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestRecordAccess(t *testing.T) {
	m := NewManager()
	pattern := &AccessPattern{
		UserID:     "user1",
		FilePath:   "/docs/report.pdf",
		AccessType: "read",
		LatencyMs:  15.5,
		Success:    true,
		DeviceType: "desktop",
	}
	m.RecordAccess(pattern)
	stats := m.GetStats()
	if stats["total_patterns"] != 1 {
		t.Errorf("expected 1 pattern, got %v", stats["total_patterns"])
	}
}

func TestCalculateScore(t *testing.T) {
	m := NewManager()
	score := m.CalculateScore("user1")
	if score == nil {
		t.Fatal("CalculateScore returned nil")
	}
	if score.OverallScore == 0 {
		t.Error("overall score should not be 0")
	}
	if score.CategoryScores[CategoryPerformance] != 85.0 {
		t.Errorf("expected perf score 85.0, got %f", score.CategoryScores[CategoryPerformance])
	}
}

func TestUpdateQuality(t *testing.T) {
	m := NewManager()
	quality := &StorageQuality{
		DeviceID:         "ssd1",
		DeviceName:       "NVMe SSD",
		IOPSScore:        95.0,
		LatencyScore:     98.0,
		ThroughputScore:  92.0,
		ReliabilityScore: 99.0,
		OverallScore:     96.0,
	}
	m.UpdateQuality(quality)
}

func TestAddSurvey(t *testing.T) {
	m := NewManager()
	survey := &SatisfactionSurvey{
		ID:       "s1",
		UserID:   "user1",
		Score:    8,
		Category: CategoryPerformance,
		Feedback: "Fast response",
	}
	m.AddSurvey(survey)
	stats := m.GetStats()
	if stats["total_surveys"] != 1 {
		t.Errorf("expected 1 survey, got %v", stats["total_surveys"])
	}
}

func TestRunBenchmark(t *testing.T) {
	m := NewManager()
	result := m.RunBenchmark("io")
	if result == nil {
		t.Fatal("RunBenchmark returned nil")
	}
	if result.Score != 85.0 {
		t.Errorf("expected score 85.0, got %f", result.Score)
	}
}

func TestGetScore(t *testing.T) {
	m := NewManager()
	m.CalculateScore("user1")
	score, ok := m.GetScore("user1")
	if !ok {
		t.Fatal("GetScore failed")
	}
	if score.UserID != "user1" {
		t.Errorf("expected user1, got %s", score.UserID)
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	m.CalculateScore("u1")
	m.CalculateScore("u2")
	stats := m.GetStats()
	if stats["total_users"] != 2 {
		t.Errorf("expected 2 users, got %v", stats["total_users"])
	}
}
