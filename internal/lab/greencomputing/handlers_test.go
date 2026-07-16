package greencomputing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandler() (*Manager, *Handler) {
	manager := NewManager(nil)
	handler := NewHandler(manager)
	return manager, handler
}

func TestHandlerRecordReading(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	body := `{"total_watts":75.5,"cpu_watts":30,"disk_watts":20,"source":"grid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/green/energy/reading", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.handleRecordReading(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var reading EnergyReading
	json.NewDecoder(w.Body).Decode(&reading)
	if reading.TotalWatts != 75.5 {
		t.Errorf("expected total watts 75.5, got %f", reading.TotalWatts)
	}
}

func TestHandlerRecordReadingMethodNotAllowed(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/energy/reading", nil)
	w := httptest.NewRecorder()

	handler.handleRecordReading(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandlerLatestReading(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	// Record a reading first
	manager.RecordReading(&EnergyReading{
		TotalWatts: 80,
		Source:     SourceGrid,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/energy/latest", nil)
	w := httptest.NewRecorder()

	handler.handleLatestReading(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var reading EnergyReading
	json.NewDecoder(w.Body).Decode(&reading)
	if reading.TotalWatts != 80 {
		t.Errorf("expected total watts 80, got %f", reading.TotalWatts)
	}
}

func TestHandlerLatestReadingNoData(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/energy/latest", nil)
	w := httptest.NewRecorder()

	handler.handleLatestReading(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerFootprint(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	manager.RecordReading(&EnergyReading{
		TotalWatts: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/carbon/footprint?period=daily", nil)
	w := httptest.NewRecorder()

	handler.handleFootprint(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var footprint CarbonFootprint
	json.NewDecoder(w.Body).Decode(&footprint)
	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestHandlerDailyFootprint(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/carbon/daily", nil)
	w := httptest.NewRecorder()

	handler.handleDailyFootprint(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerWeeklyFootprint(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/carbon/weekly", nil)
	w := httptest.NewRecorder()

	handler.handleWeeklyFootprint(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerMonthlyFootprint(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/carbon/monthly", nil)
	w := httptest.NewRecorder()

	handler.handleMonthlyFootprint(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerStrategiesList(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	manager.CreateStrategy("Test", "Desc", 30*60)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/sleep/strategies", nil)
	w := httptest.NewRecorder()

	handler.handleStrategies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var strategies []SleepStrategy
	json.NewDecoder(w.Body).Decode(&strategies)
	if len(strategies) != 1 {
		t.Errorf("expected 1 strategy, got %d", len(strategies))
	}
}

func TestHandlerStrategiesCreate(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	body := `{"name":"夜间模式","description":"夜间自动休眠","idle_threshold":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/green/sleep/strategies", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.handleStrategies(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var strategy SleepStrategy
	json.NewDecoder(w.Body).Decode(&strategy)
	if strategy.Name != "夜间模式" {
		t.Errorf("expected name '夜间模式', got %s", strategy.Name)
	}
}

func TestHandlerStrategyGet(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	created := manager.CreateStrategy("Test", "Desc", 30*60)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/sleep/strategies/"+created.ID, nil)
	w := httptest.NewRecorder()

	handler.handleStrategyByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerStrategyNotFound(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/sleep/strategies/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.handleStrategyByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandlerStrategyUpdate(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	created := manager.CreateStrategy("Old", "Desc", 30*60)

	body := `{"name":"New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/green/sleep/strategies/"+created.ID, bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.handleStrategyByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerStrategyDelete(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	created := manager.CreateStrategy("Test", "Desc", 30*60)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/green/sleep/strategies/"+created.ID, nil)
	w := httptest.NewRecorder()

	handler.handleStrategyByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerReport(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	manager.RecordReading(&EnergyReading{
		TotalWatts: 75,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/report?period=daily", nil)
	w := httptest.NewRecorder()

	handler.handleReport(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var report EfficiencyReport
	json.NewDecoder(w.Body).Decode(&report)
	if report.Period == "" {
		t.Error("period not set")
	}
}

func TestHandlerGreenScore(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/score", nil)
	w := httptest.NewRecorder()

	handler.handleGreenScore(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var score GreenScore
	json.NewDecoder(w.Body).Decode(&score)
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("expected score between 0-100, got %f", score.Score)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	endpoints := []string{
		"/api/v1/green/energy/latest",
		"/api/v1/green/carbon/daily",
		"/api/v1/green/report",
		"/api/v1/green/score",
	}

	for _, endpoint := range endpoints {
		req := httptest.NewRequest(http.MethodPost, endpoint, nil)
		w := httptest.NewRecorder()

		switch endpoint {
		case "/api/v1/green/energy/latest":
			handler.handleLatestReading(w, req)
		case "/api/v1/green/carbon/daily":
			handler.handleDailyFootprint(w, req)
		case "/api/v1/green/report":
			handler.handleReport(w, req)
		case "/api/v1/green/score":
			handler.handleGreenScore(w, req)
		}

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("endpoint %s: expected 405, got %d", endpoint, w.Code)
		}
	}
}

func TestHandlerGetReadings(t *testing.T) {
	manager := NewManager(nil)
	handler := NewHandler(manager)

	manager.RecordReading(&EnergyReading{
		TotalWatts: 50,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/green/energy/readings?period=hour", nil)
	w := httptest.NewRecorder()

	handler.handleGetReadings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
