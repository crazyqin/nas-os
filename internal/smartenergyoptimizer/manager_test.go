package smartenergyoptimizer

import (
	"bytes"
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
	if m.readings == nil {
		t.Error("readings not initialized")
	}
	if m.deviceStates == nil {
		t.Error("deviceStates not initialized")
	}
	if m.sleepPolicies == nil {
		t.Error("sleepPolicies not initialized")
	}
	if m.tariffPlans == nil {
		t.Error("tariffPlans not initialized")
	}
	if m.powerBudgets == nil {
		t.Error("powerBudgets not initialized")
	}
}

func TestRecordEnergyReading(t *testing.T) {
	m := NewManager()

	reading := EnergyReading{
		Timestamp:  time.Now(),
		PowerWatts: 100.5,
		DeviceID:   "hdd-1",
		DeviceType: "hdd",
		EnergyWh:   100.5,
	}

	m.RecordEnergyReading(reading)

	if len(m.readings) != 1 {
		t.Errorf("expected 1 reading, got %d", len(m.readings))
	}

	// Check device state was created
	if state, exists := m.deviceStates["hdd-1"]; !exists {
		t.Error("device state not created")
	} else if state.State != "active" {
		t.Errorf("expected state active, got %s", state.State)
	}
}

func TestGetReadings(t *testing.T) {
	m := NewManager()

	now := time.Now()
	m.RecordEnergyReading(EnergyReading{Timestamp: now.Add(-2 * time.Hour), PowerWatts: 50, DeviceID: "dev-1"})
	m.RecordEnergyReading(EnergyReading{Timestamp: now.Add(-1 * time.Hour), PowerWatts: 75, DeviceID: "dev-2"})
	m.RecordEnergyReading(EnergyReading{Timestamp: now, PowerWatts: 100, DeviceID: "dev-1"})

	// Get all readings
	readings := m.GetReadings(time.Time{}, time.Time{}, "")
	if len(readings) != 3 {
		t.Errorf("expected 3 readings, got %d", len(readings))
	}

	// Filter by device
	readings = m.GetReadings(time.Time{}, time.Time{}, "dev-1")
	if len(readings) != 2 {
		t.Errorf("expected 2 readings for dev-1, got %d", len(readings))
	}

	// Filter by time range
	readings = m.GetReadings(now.Add(-90*time.Minute), time.Time{}, "")
	if len(readings) != 2 {
		t.Errorf("expected 2 readings in time range, got %d", len(readings))
	}
}

func TestForecastPower(t *testing.T) {
	m := NewManager()

	// Add enough readings for forecasting
	for i := 0; i < 20; i++ {
		m.RecordEnergyReading(EnergyReading{
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Hour),
			PowerWatts: float64(50 + i*2),
			DeviceID:   "test-dev",
		})
	}

	forecasts := m.ForecastPower(60, 5, nil)
	if len(forecasts) == 0 {
		t.Error("expected forecasts, got none")
	}

	for _, f := range forecasts {
		if f.ForecastWatts < 0 {
			t.Error("forecast watts should not be negative")
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			t.Errorf("confidence out of range: %f", f.Confidence)
		}
	}
}

func TestForecastPowerInsufficientData(t *testing.T) {
	m := NewManager()

	// Only add a few readings
	for i := 0; i < 5; i++ {
		m.RecordEnergyReading(EnergyReading{
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Hour),
			PowerWatts: 50,
		})
	}

	forecasts := m.ForecastPower(30, 5, nil)
	if len(forecasts) == 0 {
		t.Error("expected default forecasts, got none")
	}
	if forecasts[0].ForecastMethod != "average_based" {
		t.Errorf("expected average_based method, got %s", forecasts[0].ForecastMethod)
	}
}

func TestCarbonAwareSchedule(t *testing.T) {
	m := NewManager()

	// Add some carbon data
	m.UpdateCarbonData([]CarbonIntensity{
		{Timestamp: time.Now().Add(2 * time.Hour), IntensityGPerKWh: 50, Region: "cn-east", IsLowCarbon: true},
		{Timestamp: time.Now().Add(3 * time.Hour), IntensityGPerKWh: 200, Region: "cn-east", IsLowCarbon: false},
	})

	tasks := []TaskToSchedule{
		{TaskID: "task-1", TaskName: "Backup", EstimatedPower: 100, EstimatedDuration: 30, Priority: 1},
		{TaskID: "task-2", TaskName: "Compression", EstimatedPower: 200, EstimatedDuration: 60, Priority: 2},
	}

	schedules := m.ScheduleCarbonAware(tasks, "cn-east", 24)
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}

	// Higher priority task should be scheduled first
	if schedules[0].Priority < schedules[1].Priority {
		t.Error("expected higher priority task first")
	}
}

func TestDeviceStates(t *testing.T) {
	m := NewManager()

	m.RecordEnergyReading(EnergyReading{DeviceID: "hdd-1", DeviceType: "hdd", PowerWatts: 10, Timestamp: time.Now()})
	m.RecordEnergyReading(EnergyReading{DeviceID: "ssd-1", DeviceType: "ssd", PowerWatts: 5, Timestamp: time.Now()})

	states := m.GetDeviceStates()
	if len(states) != 2 {
		t.Errorf("expected 2 device states, got %d", len(states))
	}
}

func TestSleepPolicy(t *testing.T) {
	m := NewManager()

	// Default policy should exist
	policies := m.GetSleepPolicies()
	if len(policies) == 0 {
		t.Error("expected default sleep policies")
	}

	// Update policy
	newPolicy := SleepPolicy{
		PolicyID:       "ssd-custom",
		DeviceType:     "ssd",
		IdleTimeoutSec: 600,
		Enabled:        true,
	}
	m.UpdateSleepPolicy(newPolicy)

	policies = m.GetSleepPolicies()
	found := false
	for _, p := range policies {
		if p.PolicyID == "ssd-custom" {
			found = true
			if p.IdleTimeoutSec != 600 {
				t.Errorf("expected 600 idle timeout, got %d", p.IdleTimeoutSec)
			}
		}
	}
	if !found {
		t.Error("custom policy not found")
	}
}

func TestEnergyCost(t *testing.T) {
	m := NewManager()

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	// Add readings
	for i := 0; i < 24; i++ {
		m.RecordEnergyReading(EnergyReading{
			Timestamp:  start.Add(time.Duration(i) * time.Hour),
			PowerWatts: 100,
			EnergyWh:   100,
		})
	}

	cost := m.CalculateEnergyCost(start, now, "")
	if cost.TotalKWh <= 0 {
		t.Error("expected positive total KWh")
	}
	if cost.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if cost.Currency != "CNY" {
		t.Errorf("expected CNY currency, got %s", cost.Currency)
	}
}

func TestTariffPlan(t *testing.T) {
	m := NewManager()

	// Default plan should exist
	plan := m.GetTariffPlan("default")
	if plan == nil {
		t.Fatal("default tariff plan not found")
	}
	if plan.Currency != "CNY" {
		t.Errorf("expected CNY currency, got %s", plan.Currency)
	}

	// Update plan
	newPlan := TariffPlan{
		PlanID:     "custom",
		Name:       "Custom Plan",
		Currency:   "USD",
		FlatRate:   0.12,
		IsFlatRate: true,
	}
	m.UpdateTariffPlan(newPlan)

	plan = m.GetTariffPlan("custom")
	if plan == nil {
		t.Fatal("custom tariff plan not found")
	}
	if plan.FlatRate != 0.12 {
		t.Errorf("expected 0.12 flat rate, got %f", plan.FlatRate)
	}
}

func TestPowerBudget(t *testing.T) {
	m := NewManager()

	now := time.Now()
	budget := PowerBudget{
		BudgetID:       "monthly-budget",
		Name:           "Monthly Target",
		PeriodType:     "monthly",
		TargetKWh:      500,
		StartDate:      now.AddDate(0, -1, 0),
		EndDate:        now,
		AlertThreshold: 80,
	}

	m.SetPowerBudget(budget)

	retrieved := m.GetPowerBudget("monthly-budget")
	if retrieved == nil {
		t.Fatal("budget not found")
	}
	if retrieved.TargetKWh != 500 {
		t.Errorf("expected 500 target KWh, got %f", retrieved.TargetKWh)
	}
}

func TestBudgetStatus(t *testing.T) {
	m := NewManager()

	// Set a budget that's exceeded
	m.SetPowerBudget(PowerBudget{
		BudgetID:       "test-budget",
		Name:           "Test Budget",
		TargetKWh:      10,
		StartDate:      time.Now().Add(-24 * time.Hour),
		EndDate:        time.Now(),
		AlertThreshold: 80,
	})

	// Add readings that exceed budget
	for i := 0; i < 100; i++ {
		m.RecordEnergyReading(EnergyReading{
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Minute),
			PowerWatts: 100,
			EnergyWh:   100,
		})
	}

	budgets, alerts := m.GetBudgetStatus()
	if len(budgets) != 1 {
		t.Errorf("expected 1 budget, got %d", len(budgets))
	}
	// Should have alerts since we exceeded threshold
	if len(alerts) == 0 {
		t.Error("expected budget alerts")
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager()

	// Add readings
	for i := 0; i < 48; i++ {
		m.RecordEnergyReading(EnergyReading{
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Hour),
			PowerWatts: float64(50 + i),
			EnergyWh:   float64(50 + i),
			DeviceType: "hdd",
		})
	}

	report := m.GenerateReport("daily", time.Now().AddDate(0, 0, -1).Format("2006-01-02"), time.Now().Format("2006-01-02"))
	if report.TotalEnergyKWh <= 0 {
		t.Error("expected positive total energy")
	}
	if report.ReportType != "daily" {
		t.Errorf("expected daily report type, got %s", report.ReportType)
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations")
	}
}

func TestMLModel(t *testing.T) {
	m := NewManager()

	// No model initially
	if model := m.GetMLModel(); model != nil {
		t.Error("expected no model initially")
	}

	// Train a model
	model := m.TrainModel("linear_regression")
	if !model.IsReady {
		t.Error("model should be ready after training")
	}
	if model.ModelType != "linear_regression" {
		t.Errorf("expected linear_regression, got %s", model.ModelType)
	}

	// Retrieve the model
	retrieved := m.GetMLModel()
	if retrieved == nil {
		t.Fatal("model not found")
	}
	if retrieved.ModelID != model.ModelID {
		t.Error("model ID mismatch")
	}
}

func TestDashboardData(t *testing.T) {
	m := NewManager()

	// Add some data
	m.RecordEnergyReading(EnergyReading{DeviceID: "dev-1", PowerWatts: 50, Timestamp: time.Now()})
	m.RecordEnergyReading(EnergyReading{DeviceID: "dev-2", PowerWatts: 75, Timestamp: time.Now()})

	data := m.GetDashboardData()
	if data["total_devices"] != 2 {
		t.Errorf("expected 2 total devices, got %v", data["total_devices"])
	}
	if data["active_devices"] != 2 {
		t.Errorf("expected 2 active devices, got %v", data["active_devices"])
	}
}

// HTTP Handler tests

func TestHandleEnergyReading(t *testing.T) {
	reading := EnergyReading{
		PowerWatts: 100,
		DeviceID:   "test-1",
		DeviceType: "ssd",
		EnergyWh:   100,
	}
	body, _ := json.Marshal(reading)

	req := httptest.NewRequest(http.MethodPost, "/api/energy/readings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleEnergyReading(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleEnergyReadingMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/readings", nil)
	w := httptest.NewRecorder()

	HandleEnergyReading(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandlePowerForecast(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/forecast", nil)
	w := httptest.NewRecorder()

	HandlePowerForecast(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp PowerPredictionResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Predictions == nil {
		t.Error("expected predictions in response")
	}
}

func TestHandleDeviceStates(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/devices", nil)
	w := httptest.NewRecorder()

	HandleDeviceStates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleSleepPolicy(t *testing.T) {
	// GET
	req := httptest.NewRequest(http.MethodGet, "/api/energy/sleep-policy", nil)
	w := httptest.NewRecorder()

	HandleSleepPolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// POST
	policy := SleepPolicy{DeviceType: "ssd", IdleTimeoutSec: 300, Enabled: true}
	body, _ := json.Marshal(policy)
	req = httptest.NewRequest(http.MethodPost, "/api/energy/sleep-policy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	HandleSleepPolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleEnergyCost(t *testing.T) {
	reqBody := EnergyCostRequest{
		StartDate: time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
		EndDate:   time.Now().Format("2006-01-02"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/energy/cost", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleEnergyCost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleTariffPlan(t *testing.T) {
	// GET
	req := httptest.NewRequest(http.MethodGet, "/api/energy/tariff", nil)
	w := httptest.NewRecorder()

	HandleTariffPlan(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlePowerBudget(t *testing.T) {
	// POST
	budget := PowerBudget{
		BudgetID:       "test-api-budget",
		Name:           "Test Budget",
		PeriodType:     "monthly",
		TargetKWh:      100,
		StartDate:      time.Now().AddDate(0, -1, 0),
		EndDate:        time.Now(),
		AlertThreshold: 80,
	}
	body, _ := json.Marshal(budget)

	req := httptest.NewRequest(http.MethodPost, "/api/energy/budget", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandlePowerBudget(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// GET
	req = httptest.NewRequest(http.MethodGet, "/api/energy/budget", nil)
	w = httptest.NewRecorder()

	HandlePowerBudget(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleBudgetStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/budget/status", nil)
	w := httptest.NewRecorder()

	HandleBudgetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleEnergyReport(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/report", nil)
	w := httptest.NewRecorder()

	HandleEnergyReport(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleDashboard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/energy/dashboard", nil)
	w := httptest.NewRecorder()

	HandleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleMLModel(t *testing.T) {
	// GET - no model initially
	req := httptest.NewRequest(http.MethodGet, "/api/energy/ml-model", nil)
	w := httptest.NewRecorder()

	HandleMLModel(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// POST - train a model
	reqBody := map[string]string{"model_type": "linear_regression"}
	body, _ := json.Marshal(reqBody)
	req = httptest.NewRequest(http.MethodPost, "/api/energy/ml-model", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	HandleMLModel(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var model MLModel
	json.NewDecoder(w.Body).Decode(&model)
	if !model.IsReady {
		t.Error("model should be ready after training")
	}
}

func TestCheckSleepEligibility(t *testing.T) {
	m := NewManager()

	// Add a device that's been idle
	m.deviceStates["old-hdd"] = &DeviceState{
		DeviceID:       "old-hdd",
		DeviceType:     "hdd",
		State:          "active",
		LastAccessTime: time.Now().Add(-1 * time.Hour), // idle for 1 hour
		PowerUsage:     8,
	}

	eligible := m.CheckSleepEligibility()
	found := false
	for _, d := range eligible {
		if d.DeviceID == "old-hdd" {
			found = true
		}
	}
	if !found {
		t.Error("expected old-hdd to be eligible for sleep")
	}
}
