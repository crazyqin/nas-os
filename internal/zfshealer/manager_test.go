package zfshealer

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
	schedule := ScrubSchedule{
		Enabled:       true,
		Interval:      7 * 24 * time.Hour,
		PreferredHour: 2,
		Level:         IntegrityStandard,
		AutoRepair:    true,
		MaxDuration:   4 * time.Hour,
	}
	m := NewManager(schedule)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.running {
		t.Error("new manager should not be running")
	}
	if len(m.datasets) != 0 {
		t.Error("new manager should have no datasets")
	}
}

func TestRegisterDataset(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")
	m.RegisterDataset("tank/backup", "tank")

	health, ok := m.GetDatasetHealth("tank/data")
	if !ok {
		t.Fatal("dataset not found after registration")
	}
	if health.Pool != "tank" {
		t.Errorf("expected pool 'tank', got '%s'", health.Pool)
	}
	if health.Status != "ONLINE" {
		t.Errorf("expected status 'ONLINE', got '%s'", health.Status)
	}
	if health.HealthScore != 1.0 {
		t.Errorf("expected health score 1.0, got %f", health.HealthScore)
	}
}

func TestListDatasetHealth(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/a", "tank")
	m.RegisterDataset("tank/b", "tank")

	list := m.ListDatasetHealth()
	if len(list) != 2 {
		t.Errorf("expected 2 datasets, got %d", len(list))
	}
}

func TestRunScrub(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")

	result, err := m.RunScrub(context.Background(), "tank/data", IntegrityQuick)
	if err != nil {
		t.Fatalf("RunScrub failed: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", result.Status)
	}
	if result.Dataset != "tank/data" {
		t.Errorf("expected dataset 'tank/data', got '%s'", result.Dataset)
	}
}

func TestRunScrubNotFound(t *testing.T) {
	m := NewManager(DefaultSchedule())
	_, err := m.RunScrub(context.Background(), "nonexistent", IntegrityQuick)
	if err == nil {
		t.Error("expected error for unregistered dataset")
	}
}

func TestRunScrubAlreadyRunning(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")
	m.running = true
	_, err := m.RunScrub(context.Background(), "tank/data", IntegrityQuick)
	if err == nil {
		t.Error("expected error when scrub already running")
	}
}

func TestGetResults(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")

	m.RunScrub(context.Background(), "tank/data", IntegrityQuick)
	results := m.GetResults(10)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")

	// No errors expected for clean dataset
	m.RunScrub(context.Background(), "tank/data", IntegrityQuick)
	alerts := m.GetAlerts(10)
	// Should have no alerts for clean scrub
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestScheduleUpdate(t *testing.T) {
	m := NewManager(DefaultSchedule())
	newSchedule := ScrubSchedule{
		Enabled:       true,
		Interval:      14 * 24 * time.Hour,
		PreferredHour: 3,
		Level:         IntegrityDeep,
		AutoRepair:    true,
	}
	m.UpdateSchedule(newSchedule)
	got := m.GetSchedule()
	if got.Interval != 14*24*time.Hour {
		t.Errorf("expected interval 14 days, got %v", got.Interval)
	}
}

func TestIsRunning(t *testing.T) {
	m := NewManager(DefaultSchedule())
	if m.IsRunning() {
		t.Error("new manager should not be running")
	}
}

func TestHandlerHealth(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zfshealer/health", nil)
	w := httptest.NewRecorder()
	h.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("expected count 1, got %v", resp["count"])
	}
}

func TestHandlerScrub(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")
	h := NewHandler(m)

	body, _ := json.Marshal(map[string]string{
		"dataset": "tank/data",
		"level":   "quick",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/zfshealer/scrub", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleScrub(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	m := NewManager(DefaultSchedule())
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/zfshealer/health", nil)
	w := httptest.NewRecorder()
	h.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDefaultSchedule(t *testing.T) {
	s := DefaultSchedule()
	if !s.Enabled {
		t.Error("default schedule should be enabled")
	}
	if s.Interval != 7*24*time.Hour {
		t.Errorf("default interval should be 7 days, got %v", s.Interval)
	}
	if !s.AutoRepair {
		t.Error("default schedule should have auto repair enabled")
	}
}

func TestCalculateNextScrub(t *testing.T) {
	m := NewManager(ScrubSchedule{
		Enabled:       true,
		Interval:      7 * 24 * time.Hour,
		PreferredHour: 2,
		AvoidDays:     []time.Weekday{time.Saturday, time.Sunday},
	})
	next := m.calculateNextScrub()
	if next.Before(time.Now()) {
		t.Error("next scrub should be in the future")
	}
	if next.Hour() != 2 {
		t.Errorf("expected hour 2, got %d", next.Hour())
	}
}

func TestHandlerStatus(t *testing.T) {
	m := NewManager(DefaultSchedule())
	m.RegisterDataset("tank/data", "tank")
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zfshealer/status", nil)
	w := httptest.NewRecorder()
	h.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerScheduleGet(t *testing.T) {
	m := NewManager(DefaultSchedule())
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zfshealer/schedule", nil)
	w := httptest.NewRecorder()
	h.handleSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerSchedulePut(t *testing.T) {
	m := NewManager(DefaultSchedule())
	h := NewHandler(m)

	newSchedule := ScrubSchedule{
		Enabled:       true,
		Interval:      14 * 24 * time.Hour,
		PreferredHour: 3,
		Level:         IntegrityDeep,
		AutoRepair:    true,
	}
	body, _ := json.Marshal(newSchedule)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/zfshealer/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func DefaultSchedule() ScrubSchedule {
	return ScrubSchedule{
		Enabled:       true,
		Interval:      7 * 24 * time.Hour,
		PreferredHour: 2,
		Level:         IntegrityStandard,
		AutoRepair:    true,
		MaxDuration:   4 * time.Hour,
	}
}
