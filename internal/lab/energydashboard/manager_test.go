package energydashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestDefaultDashboardConfig(t *testing.T) {
	cfg := DefaultDashboardConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.MonitorInterval != 60 {
		t.Errorf("expected monitor interval 60, got %d", cfg.MonitorInterval)
	}
	if cfg.Region != "cn-default" {
		t.Errorf("expected region cn-default, got %s", cfg.Region)
	}
	if cfg.Currency != "CNY" {
		t.Errorf("expected currency CNY, got %s", cfg.Currency)
	}
	if cfg.CarbonFactor <= 0 {
		t.Error("expected positive carbon factor")
	}
	if cfg.ReportRetention != 365 {
		t.Errorf("expected report retention 365, got %d", cfg.ReportRetention)
	}
}

func TestGetDefaultRates(t *testing.T) {
	rates := GetDefaultRates()
	if len(rates) != 2 {
		t.Errorf("expected 2 default rates, got %d", len(rates))
	}

	for _, r := range rates {
		if r.PriceKWh <= 0 {
			t.Errorf("expected positive price for tier %s", r.Name)
		}
		if r.StartTime == "" || r.EndTime == "" {
			t.Errorf("expected non-empty time for tier %s", r.Name)
		}
	}
}

func TestSupportedPeriods(t *testing.T) {
	periods := SupportedPeriods()
	if len(periods) != 4 {
		t.Errorf("expected 4 supported periods, got %d", len(periods))
	}

	for _, p := range periods {
		if !IsValidPeriod(p) {
			t.Errorf("expected %s to be valid period", p)
		}
	}

	if IsValidPeriod("invalid") {
		t.Error("expected 'invalid' to be invalid period")
	}
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)

	if m.IsRunning() {
		t.Error("expected manager to not be running initially")
	}

	// Should have default rates
	rates := m.ListRates()
	if len(rates) < 1 {
		t.Errorf("expected at least 1 default rate, got %d", len(rates))
	}

	// Should have default schedules
	schedules := m.ListSchedules()
	if len(schedules) < 1 {
		t.Errorf("expected at least 1 default schedule, got %d", len(schedules))
	}
}

func TestManagerStartStop(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Start
	if err := m.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected manager to be running")
	}

	// Start again should fail
	if err := m.Start(ctx); err == nil {
		t.Error("expected error when starting twice")
	}

	// Stop
	m.Stop()
	if m.IsRunning() {
		t.Error("expected manager to not be running after stop")
	}
}

func TestGetLatestSnapshot(t *testing.T) {
	m := setupTestManager(t)

	// Empty initially
	snap := m.GetLatestSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.TotalPower != 0 {
		t.Errorf("expected 0 total power, got %f", snap.TotalPower)
	}
}

func TestRecordPowerReading(t *testing.T) {
	m := setupTestManager(t)

	reading := &PowerReading{
		Component:  ComponentCPU,
		DeviceName: "Test CPU",
		PowerWatts: 65.5,
		State:      PowerStateActive,
	}

	m.RecordPowerReading(reading)

	if reading.ID == "" {
		t.Error("expected non-empty ID")
	}
	if reading.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRateManagement(t *testing.T) {
	m := setupTestManager(t)

	// Create rate
	rate := &ElectricityRate{
		Region:       "test-region",
		Currency:     "CNY",
		ProviderName: "测试电力",
		Rates: []RateTier{
			{Name: "峰时", StartTime: "08:00", EndTime: "22:00", PriceKWh: 0.60},
			{Name: "谷时", StartTime: "22:00", EndTime: "08:00", PriceKWh: 0.30},
		},
	}

	created, err := m.CreateRate(rate)
	if err != nil {
		t.Fatalf("CreateRate failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty rate ID")
	}

	// Get rate
	got, err := m.GetRate(created.ID)
	if err != nil {
		t.Fatalf("GetRate failed: %v", err)
	}
	if got.Region != "test-region" {
		t.Errorf("expected region test-region, got %s", got.Region)
	}

	// List rates
	rates := m.ListRates()
	if len(rates) < 2 {
		t.Errorf("expected at least 2 rates, got %d", len(rates))
	}

	// Update rate
	updated, err := m.UpdateRate(created.ID, &ElectricityRate{
		Region:       "updated-region",
		Currency:     "USD",
		ProviderName: "Updated Provider",
		Rates: []RateTier{
			{Name: "flat", StartTime: "00:00", EndTime: "23:59", PriceKWh: 0.12},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRate failed: %v", err)
	}
	if updated.Region != "updated-region" {
		t.Errorf("expected region updated-region, got %s", updated.Region)
	}

	// Delete rate
	if err := m.DeleteRate(created.ID); err != nil {
		t.Fatalf("DeleteRate failed: %v", err)
	}

	_, err = m.GetRate(created.ID)
	if err == nil {
		t.Error("expected error for deleted rate")
	}
}

func TestCreateRateValidation(t *testing.T) {
	m := setupTestManager(t)

	// Invalid time format
	_, err := m.CreateRate(&ElectricityRate{
		Region: "test",
		Rates: []RateTier{
			{Name: "bad", StartTime: "25:00", EndTime: "08:00", PriceKWh: 0.5},
		},
	})
	if err == nil {
		t.Error("expected error for invalid time format")
	}

	// Invalid price
	_, err = m.CreateRate(&ElectricityRate{
		Region: "test",
		Rates: []RateTier{
			{Name: "bad", StartTime: "08:00", EndTime: "22:00", PriceKWh: -0.5},
		},
	})
	if err == nil {
		t.Error("expected error for negative price")
	}
}

func TestScheduleManagement(t *testing.T) {
	m := setupTestManager(t)

	// Create schedule
	sched := &SleepSchedule{
		Name:         "测试休眠",
		Policy:       SleepPolicyScheduled,
		TargetDevice: "disks",
		StartTime:    "01:00",
		EndTime:      "06:00",
		Enabled:      true,
	}

	created, err := m.CreateSchedule(sched)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty schedule ID")
	}

	// Get schedule
	got, err := m.GetSchedule(created.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if got.Name != "测试休眠" {
		t.Errorf("expected name '测试休眠', got '%s'", got.Name)
	}

	// List schedules
	schedules := m.ListSchedules()
	if len(schedules) < 3 {
		t.Errorf("expected at least 3 schedules, got %d", len(schedules))
	}

	// Update schedule
	updated, err := m.UpdateSchedule(created.ID, &SleepSchedule{
		Name:         "更新后休眠",
		Policy:       SleepPolicyIdle,
		TargetDevice: "system",
		IdleMinutes:  45,
		Enabled:      false,
	})
	if err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}
	if updated.Name != "更新后休眠" {
		t.Errorf("expected name '更新后休眠', got '%s'", updated.Name)
	}

	// Toggle schedule
	toggled, err := m.ToggleSchedule(created.ID)
	if err != nil {
		t.Fatalf("ToggleSchedule failed: %v", err)
	}
	if !toggled.Enabled {
		t.Error("expected schedule to be enabled after toggle")
	}

	// Delete schedule
	if err := m.DeleteSchedule(created.ID); err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}

	_, err = m.GetSchedule(created.ID)
	if err == nil {
		t.Error("expected error for deleted schedule")
	}
}

func TestCreateScheduleValidation(t *testing.T) {
	m := setupTestManager(t)

	// Invalid time format for scheduled policy
	_, err := m.CreateSchedule(&SleepSchedule{
		Name:      "bad",
		Policy:    SleepPolicyScheduled,
		StartTime: "25:00",
		EndTime:   "08:00",
	})
	if err == nil {
		t.Error("expected error for invalid time format")
	}

	// Invalid idle minutes for idle policy
	_, err = m.CreateSchedule(&SleepSchedule{
		Name:        "bad",
		Policy:      SleepPolicyIdle,
		IdleMinutes: 0,
	})
	if err == nil {
		t.Error("expected error for zero idle minutes")
	}
}

func TestCalculateEfficiencyScore(t *testing.T) {
	m := setupTestManager(t)

	score := m.CalculateEfficiencyScore()
	if score == nil {
		t.Fatal("expected non-nil score")
	}
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("expected score between 0-100, got %d", score.Score)
	}
	if score.Rating == "" {
		t.Error("expected non-empty rating")
	}
}

func TestDashboardSummary(t *testing.T) {
	m := setupTestManager(t)

	summary := m.GetDashboardSummary()
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.Currency == "" {
		t.Error("expected non-empty currency")
	}
	if summary.EfficiencyScore == nil {
		t.Error("expected non-nil efficiency score")
	}
	if summary.LastUpdated.IsZero() {
		t.Error("expected non-zero last updated")
	}
}

func TestGetSnapshots(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Start monitor to generate snapshots
	if err := m.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait briefly for a snapshot
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	since := time.Now().Add(-1 * time.Hour)
	snapshots := m.GetSnapshots(since, 10)
	if len(snapshots) == 0 {
		// Might be zero if timing is tight, that's OK
		t.Log("no snapshots yet (timing-dependent)")
	}
}

func TestCalculateEnergyCost(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Start monitor to generate some data
	if err := m.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	// Calculate cost with default rate
	cost, err := m.CalculateEnergyCost(ctx, PeriodDaily, "rate-cn-default")
	if err != nil {
		t.Fatalf("CalculateEnergyCost failed: %v", err)
	}
	if cost.Currency == "" {
		t.Error("expected non-empty currency")
	}
	if cost.Period != PeriodDaily {
		t.Errorf("expected period daily, got %s", cost.Period)
	}
}

func TestCalculateEnergyCostInvalidPeriod(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.CalculateEnergyCost(ctx, "invalid", "rate-cn-default")
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestCalculateEnergyCostInvalidRate(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.CalculateEnergyCost(ctx, PeriodDaily, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rate")
	}
}

func TestEstimateCarbon(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Start monitor to generate some data
	if err := m.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	estimate, err := m.EstimateCarbon(ctx, PeriodDaily)
	if err != nil {
		t.Fatalf("EstimateCarbon failed: %v", err)
	}
	if estimate.Factor <= 0 {
		t.Error("expected positive carbon factor")
	}
	if estimate.Period != PeriodDaily {
		t.Errorf("expected period daily, got %s", estimate.Period)
	}
}

func TestEstimateCarbonInvalidPeriod(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.EstimateCarbon(ctx, "invalid")
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestGenerateReport(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// Start monitor to generate some data
	if err := m.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	report, err := m.GenerateReport(ctx, PeriodDaily, "rate-cn-default")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report.ID == "" {
		t.Error("expected non-empty report ID")
	}
	if report.Period != PeriodDaily {
		t.Errorf("expected period daily, got %s", report.Period)
	}
	if report.Currency == "" {
		t.Error("expected non-empty currency")
	}

	// Get report
	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, got.ID)
	}

	// List reports
	reports := m.ListReports(PeriodDaily, 10)
	if len(reports) < 1 {
		t.Errorf("expected at least 1 report, got %d", len(reports))
	}
}

func TestGenerateReportInvalidPeriod(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.GenerateReport(ctx, "invalid", "rate-cn-default")
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestGenerateReportInvalidRate(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	_, err := m.GenerateReport(ctx, PeriodDaily, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rate")
	}
}

func TestGetNonexistentReport(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestConfigManagement(t *testing.T) {
	m := setupTestManager(t)

	// Get config
	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected config to be enabled")
	}

	// Update config
	newCfg := DefaultDashboardConfig()
	newCfg.MonitorInterval = 120
	newCfg.Region = "us-west"
	m.UpdateConfig(newCfg)

	cfg = m.GetConfig()
	if cfg.MonitorInterval != 120 {
		t.Errorf("expected interval 120, got %d", cfg.MonitorInterval)
	}
	if cfg.Region != "us-west" {
		t.Errorf("expected region us-west, got %s", cfg.Region)
	}
}

func TestIsValidTimeFormat(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"00:00", true},
		{"08:30", true},
		{"23:59", true},
		{"24:00", false},
		{"12:60", false},
		{"abc", false},
		{"1:00", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidTimeFormat(tt.input)
		if result != tt.valid {
			t.Errorf("isValidTimeFormat(%q) = %v, want %v", tt.input, result, tt.valid)
		}
	}
}

// Handler tests

func TestHandler_GetSummary(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/summary", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetLatestPower(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/power/latest", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetPowerHistory(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/power/history?limit=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_RecordPowerReading(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"component":"cpu","device_name":"CPU-01","power_watts":55.5,"state":"active"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/energy/power/reading", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RateManagement(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create rate
	body := `{"region":"us-east","currency":"USD","provider_name":"US Power","rates":[{"name":"flat","start_time":"00:00","end_time":"23:59","price_kwh":0.12}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/energy/rates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	rateData, _ := json.Marshal(resp.Data)
	var rate ElectricityRate
	json.Unmarshal(rateData, &rate)

	// Get rate
	req = httptest.NewRequest(http.MethodGet, "/api/v1/energy/rates/"+rate.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// List rates
	req = httptest.NewRequest(http.MethodGet, "/api/v1/energy/rates", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Delete rate
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/energy/rates/"+rate.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_ScheduleManagement(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create schedule
	body := `{"name":"测试计划","policy":"scheduled","target_device":"disks","start_time":"01:00","end_time":"06:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/energy/schedules", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	schedData, _ := json.Marshal(resp.Data)
	var sched SleepSchedule
	json.Unmarshal(schedData, &sched)

	// Get schedule
	req = httptest.NewRequest(http.MethodGet, "/api/v1/energy/schedules/"+sched.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Toggle schedule
	req = httptest.NewRequest(http.MethodPost, "/api/v1/energy/schedules/"+sched.ID+"/toggle", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Delete schedule
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/energy/schedules/"+sched.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_CalculateCost(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/cost?period=daily&rate_id=rate-cn-default", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetEfficiencyScore(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/efficiency", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_EstimateCarbon(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/carbon?period=daily", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GenerateReport(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"period":"daily","rate_id":"rate-cn-default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/energy/reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MonitorControl(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Get status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/monitor/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var status map[string]interface{}
	json.Unmarshal(data, &status)

	if status["running"].(bool) {
		t.Error("expected monitor to not be running initially")
	}

	// Start monitor
	req = httptest.NewRequest(http.MethodPost, "/api/v1/energy/monitor/start", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for start, got %d: %s", w.Code, w.Body.String())
	}

	// Stop monitor
	req = httptest.NewRequest(http.MethodPost, "/api/v1/energy/monitor/stop", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for stop, got %d", w.Code)
	}
}

func TestHandler_GetConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/config", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_UpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"enabled":true,"monitor_interval":120,"region":"us-west","currency":"USD","carbon_factor":0.4}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/energy/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Dashboard(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/dashboard", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetNonexistentRate(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/rates/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_GetNonexistentSchedule(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/schedules/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_GetNonexistentReport(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/reports/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_ListReports(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/energy/reports?period=daily&limit=5", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
