package smartnas

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.subsystems) != 0 {
		t.Error("new manager should have no subsystems")
	}
}

func TestUpdateSubsystem(t *testing.T) {
	m := NewManager()
	health := &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 85.0,
		Level: HealthGood,
		Metrics: []Metric{
			{Name: "disk_usage", Value: 72.0, Unit: "%", Threshold: 80.0, Status: "ok"},
		},
	}
	m.UpdateSubsystem(SubsystemStorage, health)

	got, ok := m.GetSubsystem(SubsystemStorage)
	if !ok {
		t.Fatal("subsystem not found")
	}
	if got.Score != 85.0 {
		t.Errorf("expected score 85.0, got %f", got.Score)
	}
}

func TestGetScore(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 90.0,
		Level: HealthExcellent,
	})
	m.UpdateSubsystem(SubsystemSecurity, &SubsystemHealth{
		Type:  SubsystemSecurity,
		Score: 80.0,
		Level: HealthGood,
	})

	score := m.GetScore()
	if score.Overall <= 0 {
		t.Error("overall score should be positive")
	}
	if score.Level == HealthCritical {
		t.Error("level should not be critical with these scores")
	}
}

func TestGetScoreEmpty(t *testing.T) {
	m := NewManager()
	score := m.GetScore()
	if score.Overall != 0 {
		t.Errorf("expected 0 for empty manager, got %f", score.Overall)
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  HealthLevel
	}{
		{95, HealthExcellent},
		{80, HealthGood},
		{65, HealthFair},
		{50, HealthPoor},
		{20, HealthCritical},
	}
	for _, tt := range tests {
		got := scoreToLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToLevel(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestRecommendations(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 50.0,
		Level: HealthPoor,
		Metrics: []Metric{
			{Name: "disk_usage", Value: 95.0, Unit: "%", Threshold: 80.0, Status: "critical"},
			{Name: "io_wait", Value: 70.0, Unit: "ms", Threshold: 50.0, Status: "warning"},
		},
	})

	recs := m.GetRecommendations(false)
	if len(recs) < 1 {
		t.Error("expected at least 1 recommendation")
	}
}

func TestDismissRecommendation(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 50.0,
		Level: HealthPoor,
		Metrics: []Metric{
			{Name: "disk_usage", Value: 95.0, Unit: "%", Threshold: 80.0, Status: "critical"},
		},
	})

	recs := m.GetRecommendations(false)
	if len(recs) == 0 {
		t.Fatal("expected at least 1 recommendation")
	}

	ok := m.DismissRecommendation(recs[0].ID)
	if !ok {
		t.Error("dismiss should return true")
	}

	active := m.GetRecommendations(false)
	if len(active) >= len(recs) {
		t.Error("active recommendations should decrease after dismiss")
	}
}

func TestDismissNotFound(t *testing.T) {
	m := NewManager()
	ok := m.DismissRecommendation("nonexistent")
	if ok {
		t.Error("dismiss should return false for nonexistent ID")
	}
}

func TestRecordSnapshot(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 80.0,
		Level: HealthGood,
	})
	m.RecordSnapshot()
	m.RecordSnapshot()

	history := m.GetHistory(10)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestTrend(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 60.0,
		Level: HealthFair,
	})
	m.RecordSnapshot()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 70.0,
		Level: HealthGood,
	})
	m.RecordSnapshot()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 80.0,
		Level: HealthGood,
	})
	m.RecordSnapshot()

	score := m.GetScore()
	if score.Trend != "improving" {
		t.Errorf("expected improving trend, got %s", score.Trend)
	}
}

func TestRefreshAll(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 80.0,
		Level: HealthGood,
	})
	err := m.RefreshAll(context.Background())
	if err != nil {
		t.Fatalf("RefreshAll failed: %v", err)
	}
}

func TestHandlerScore(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 85.0,
		Level: HealthGood,
	})
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smartnas/score", nil)
	w := httptest.NewRecorder()
	h.handleScore(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp NASHealthScore
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Overall <= 0 {
		t.Error("expected positive overall score")
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	m := NewManager()
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smartnas/score", nil)
	w := httptest.NewRecorder()
	h.handleScore(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandlerHistory(t *testing.T) {
	m := NewManager()
	m.RecordSnapshot()
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smartnas/history?limit=5", nil)
	w := httptest.NewRecorder()
	h.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerDismiss(t *testing.T) {
	m := NewManager()
	m.UpdateSubsystem(SubsystemStorage, &SubsystemHealth{
		Type:  SubsystemStorage,
		Score: 50.0,
		Level: HealthPoor,
		Metrics: []Metric{
			{Name: "disk_usage", Value: 95.0, Unit: "%", Threshold: 80.0, Status: "critical"},
		},
	})
	h := NewHandler(m)

	recs := m.GetRecommendations(false)
	if len(recs) == 0 {
		t.Skip("no recommendations to dismiss")
	}

	reqBody := bytes.NewBufferString(`{"id": "` + recs[0].ID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smartnas/dismiss", reqBody)
	w := httptest.NewRecorder()
	h.handleDismiss(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUptime(t *testing.T) {
	m := NewManager()
	time.Sleep(10 * time.Millisecond)
	score := m.GetScore()
	if score.Uptime < 10*time.Millisecond {
		t.Error("uptime should be positive")
	}
}
