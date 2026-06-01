package healthscore

import (
	"testing"
	"time"
)

func TestNewHealthScoreManager(t *testing.T) {
	mgr := NewHealthScoreManager()
	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}
	if mgr.calculator == nil {
		t.Fatal("Expected non-nil calculator")
	}
	if mgr.analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}
}

func TestGenerateReport(t *testing.T) {
	mgr := NewHealthScoreManager()

	report, err := mgr.GenerateReport()
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report == nil {
		t.Fatal("Expected non-nil report")
	}

	if report.OverallScore < 0 || report.OverallScore > 100 {
		t.Fatalf("Expected score between 0-100, got %f", report.OverallScore)
	}

	if report.OverallStatus == "" {
		t.Fatal("Expected non-empty status")
	}

	if len(report.Components) == 0 {
		t.Fatal("Expected at least one component")
	}
}

func TestScoreCalculation(t *testing.T) {
	mgr := NewHealthScoreManager()

	components := []ComponentScore{
		{Type: ComponentCPU, Score: 80, Weight: 0.3},
		{Type: ComponentMemory, Score: 70, Weight: 0.3},
		{Type: ComponentDisk, Score: 90, Weight: 0.4},
	}

	score := mgr.GetCalculator().CalculateOverallScore(components)
	if score <= 0 || score > 100 {
		t.Fatalf("Expected valid score, got %f", score)
	}
}

func TestDetermineStatus(t *testing.T) {
	mgr := NewHealthScoreManager()

	tests := []struct {
		score    float64
		expected HealthStatus
	}{
		{95, StatusExcellent},
		{80, StatusGood},
		{65, StatusFair},
		{45, StatusPoor},
		{20, StatusCritical},
	}

	for _, test := range tests {
		status := mgr.GetCalculator().DetermineStatus(test.score)
		if status != test.expected {
			t.Errorf("Score %f: expected %s, got %s", test.score, test.expected, status)
		}
	}
}

func TestRecommendations(t *testing.T) {
	mgr := NewHealthScoreManager()

	components := []ComponentScore{
		{Type: ComponentCPU, Score: 30},    // Should trigger recommendation
		{Type: ComponentMemory, Score: 80}, // Should not trigger
		{Type: ComponentDisk, Score: 45},   // Should trigger recommendation
	}

	recs := mgr.GetCalculator().GenerateRecommendations(components)
	if len(recs) < 2 {
		t.Fatalf("Expected at least 2 recommendations, got %d", len(recs))
	}
}

func TestHistory(t *testing.T) {
	mgr := NewHealthScoreManager()

	// Generate a few reports to build history
	for i := 0; i < 5; i++ {
		mgr.GenerateReport()
		time.Sleep(10 * time.Millisecond) // Small delay for different timestamps
	}

	history := mgr.GetHistory(10)
	if len(history) != 5 {
		t.Fatalf("Expected 5 history entries, got %d", len(history))
	}
}

func TestTrendAnalysis(t *testing.T) {
	mgr := NewHealthScoreManager()

	// Add some history
	for i := 0; i < 10; i++ {
		mgr.mu.Lock()
		mgr.history = append(mgr.history, ScoreHistory{
			Timestamp: time.Now().Add(-time.Duration(10-i) * time.Hour),
			Score:     float64(70 + i*2), // Increasing trend
			Status:    StatusGood,
		})
		mgr.mu.Unlock()
	}

	trend := mgr.GetAnalyzer().AnalyzeTrend(24 * time.Hour)
	if trend == nil {
		t.Fatal("Expected non-nil trend")
	}
}

func TestWorstComponents(t *testing.T) {
	mgr := NewHealthScoreManager()

	// Generate a report first
	mgr.GenerateReport()

	worst := mgr.GetAnalyzer().GetWorstComponents(3)
	if len(worst) == 0 {
		t.Fatal("Expected worst components")
	}
}

func TestScoreDistribution(t *testing.T) {
	mgr := NewHealthScoreManager()

	// Generate a report first
	mgr.GenerateReport()

	distribution := mgr.GetAnalyzer().GetScoreDistribution()
	total := 0
	for _, count := range distribution {
		total += count
	}
	if total == 0 {
		t.Fatal("Expected non-zero distribution")
	}
}
