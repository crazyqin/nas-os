package storageroi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestCalculator() *ROICalculator {
	calc := NewROICalculator()
	calc.SetElectricityRate(0.8)
	return calc
}

func setupTestHandler(t *testing.T) *Handlers {
	t.Helper()
	return NewHandlers(setupTestCalculator(), zap.NewNop())
}

func setupTestRouter(t *testing.T, h *Handlers) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h.RegisterRoutes(rg)
	return r
}

func setupTestDisk(id string) *DiskCostRecord {
	return &DiskCostRecord{
		ID:            id,
		SerialNumber:  "SN-" + id,
		Model:         "ST8000NM0055",
		Vendor:        "Seagate",
		DiskType:      DiskTypeHDD,
		CapacityBytes: 8e12, // 8TB
		PurchasePrice: 1200,
		Currency:      "CNY",
		PurchaseDate:  time.Now().AddDate(-2, 0, 0), // 2年前购买
		WarrantyYears: 5,
	}
}

func setupTestLifetime(id string, health float64, status DiskStatus) *LifetimeTracker {
	return &LifetimeTracker{
		DiskID:             id,
		SerialNumber:       "SN-" + id,
		PurchaseDate:       time.Now().AddDate(-2, 0, 0),
		PowerOnHours:       17520, // 2年
		EstimatedTBW:       1000,  // 1000 TB 估计寿命
		ActualTBW:          400,   // 400 TB 已写入
		WarrantyEnd:        time.Now().AddDate(3, 0, 0),
		Status:             status,
		HealthScore:        health,
		ReallocatedSectors: 0,
		TemperatureMax:     45,
		LastChecked:        time.Now(),
	}
}

func setupTestUtilization(diskID string, used, total int64) *CapacityUtilization {
	return &CapacityUtilization{
		DiskID:     diskID,
		Timestamp:  time.Now(),
		TotalBytes: total,
		UsedBytes:  used,
	}
}

// ==================== CapacityUtilization 测试 ====================

func TestCapacityUtilization_Percent(t *testing.T) {
	u := &CapacityUtilization{
		TotalBytes: 8e12,
		UsedBytes:  5e12,
	}
	pct := u.UtilizationPercent()
	expected := (5e12 / 8e12) * 100
	if pct != expected {
		t.Errorf("expected %.2f, got %.2f", expected, pct)
	}
}

func TestCapacityUtilization_AvailableBytes(t *testing.T) {
	u := &CapacityUtilization{
		TotalBytes:    8e12,
		UsedBytes:     5e12,
		ReservedBytes: 1e12,
	}
	avail := u.AvailableBytes()
	expected := int64(8e12 - 5e12 - 1e12)
	if avail != expected {
		t.Errorf("expected %d, got %d", expected, avail)
	}
}

func TestCapacityUtilization_ZeroTotal(t *testing.T) {
	u := &CapacityUtilization{TotalBytes: 0, UsedBytes: 100}
	if u.UtilizationPercent() != 0 {
		t.Error("expected 0 percent for zero total")
	}
}

func TestCapacityUtilization_NegativeAvailable(t *testing.T) {
	u := &CapacityUtilization{
		TotalBytes:    100,
		UsedBytes:     80,
		ReservedBytes: 30,
	}
	if u.AvailableBytes() != 0 {
		t.Error("expected 0 available when over-allocated")
	}
}

// ==================== LifetimeTracker 测试 ====================

func TestLifetimeTracker_EstimatedRemainingHours(t *testing.T) {
	lt := &LifetimeTracker{
		PowerOnHours: 17520, // 2年 * 24 * 365
		EstimatedTBW: 1000,
		ActualTBW:    400,
	}
	remaining := lt.EstimatedRemainingHours()
	// avgWriteRate = 400/17520 ≈ 0.02286 TB/h
	// remaining = (1000-400)/0.02286 ≈ 26236 hours
	expected := (1000 - 400) / (400.0 / 17520)
	if remaining < expected*0.99 || remaining > expected*1.01 {
		t.Errorf("expected ~%.0f, got %.0f", expected, remaining)
	}
}

func TestLifetimeTracker_EstimatedRemainingHours_Exhausted(t *testing.T) {
	lt := &LifetimeTracker{
		PowerOnHours: 20000,
		EstimatedTBW: 500,
		ActualTBW:    500,
	}
	if lt.EstimatedRemainingHours() != 0 {
		t.Error("expected 0 remaining hours when TBW exhausted")
	}
}

func TestLifetimeTracker_EstimatedReplacementDate(t *testing.T) {
	lt := &LifetimeTracker{
		PowerOnHours: 10000,
		EstimatedTBW: 1000,
		ActualTBW:    100,
	}
	date := lt.EstimatedReplacementDate()
	if date.Before(time.Now()) {
		t.Error("replacement date should be in the future")
	}
}

func TestLifetimeTracker_NeedsReplacement_Failed(t *testing.T) {
	lt := &LifetimeTracker{Status: DiskStatusFailed}
	if !lt.NeedsReplacement() {
		t.Error("failed disk should need replacement")
	}
}

func TestLifetimeTracker_NeedsReplacement_LowHealth(t *testing.T) {
	lt := &LifetimeTracker{
		Status:      DiskStatusActive,
		HealthScore: 20,
	}
	if !lt.NeedsReplacement() {
		t.Error("low health disk should need replacement")
	}
}

func TestLifetimeTracker_NeedsReplacement_ReallocatedSectors(t *testing.T) {
	lt := &LifetimeTracker{
		Status:             DiskStatusActive,
		HealthScore:        80,
		ReallocatedSectors: 150,
	}
	if !lt.NeedsReplacement() {
		t.Error("disk with many reallocated sectors should need replacement")
	}
}

func TestLifetimeTracker_NeedsReplacement_Healthy(t *testing.T) {
	lt := &LifetimeTracker{
		Status:       DiskStatusActive,
		HealthScore:  95,
		EstimatedTBW: 1000,
		ActualTBW:    100,
		PowerOnHours: 5000,
	}
	if lt.NeedsReplacement() {
		t.Error("healthy disk should not need replacement")
	}
}

// ==================== ROICalculator 测试 ====================

func TestROICalculator_CalculateTCO(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-001")
	lt := setupTestLifetime("disk-001", 85, DiskStatusActive)

	report := calc.CalculateTCO(cost, lt)

	if report.DiskID != "disk-001" {
		t.Errorf("expected disk-001, got %s", report.DiskID)
	}
	if report.PurchaseCost != 1200 {
		t.Errorf("expected purchase cost 1200, got %.2f", report.PurchaseCost)
	}
	if report.ElectricityCost <= 0 {
		t.Error("expected positive electricity cost")
	}
	if report.MaintenanceCost <= 0 {
		t.Error("expected positive maintenance cost")
	}
	if report.ReplacementCost != 0 {
		t.Error("active disk should have zero replacement cost")
	}
	if report.TotalCost <= report.PurchaseCost {
		t.Error("total cost should exceed purchase cost")
	}
	if report.CostPerMonth <= 0 {
		t.Error("expected positive cost per month")
	}
	if report.CostPerTB <= 0 {
		t.Error("expected positive cost per TB")
	}
}

func TestROICalculator_CalculateTCO_FailedDisk(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-002")
	lt := setupTestLifetime("disk-002", 20, DiskStatusFailed)

	report := calc.CalculateTCO(cost, lt)

	if report.ReplacementCost != 1200 {
		t.Errorf("expected replacement cost 1200, got %.2f", report.ReplacementCost)
	}
}

func TestROICalculator_CalculateTCO_NilCost(t *testing.T) {
	calc := setupTestCalculator()
	report := calc.CalculateTCO(nil, &LifetimeTracker{})
	if report.TotalCost != 0 {
		t.Error("expected zero total cost for nil input")
	}
}

func TestROICalculator_CalculateTCOBatch(t *testing.T) {
	calc := setupTestCalculator()
	costs := []*DiskCostRecord{
		setupTestDisk("disk-001"),
		setupTestDisk("disk-002"),
	}
	lifetimes := map[string]*LifetimeTracker{
		"disk-001": setupTestLifetime("disk-001", 85, DiskStatusActive),
		"disk-002": setupTestLifetime("disk-002", 60, DiskStatusDegraded),
	}

	reports := calc.CalculateTCOBatch(costs, lifetimes)
	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}
	for _, r := range reports {
		if r.TotalCost <= 0 {
			t.Error("expected positive total cost")
		}
	}
}

func TestROICalculator_CalculateROI_GoodDisk(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-001")
	lt := setupTestLifetime("disk-001", 90, DiskStatusActive)

	utils := []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(5e12), int64(8e12)), // 62.5%
	}

	tco := calc.CalculateTCO(cost, lt)
	score := calc.CalculateROI(utils, tco, lt, cost)

	if score.Score < 60 {
		t.Errorf("good disk should have score >= 60, got %.2f", score.Score)
	}
	if score.Grade == "F" {
		t.Error("good disk should not get F grade")
	}
	if score.CapacityScore < 95 || score.CapacityScore > 100 {
		t.Logf("capacity score: %.2f", score.CapacityScore)
	}
}

func TestROICalculator_CalculateROI_FullDisk(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-001")
	lt := setupTestLifetime("disk-001", 80, DiskStatusActive)

	utils := []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(7.8e12), int64(8e12)), // 97.5%
	}

	tco := calc.CalculateTCO(cost, lt)
	score := calc.CalculateROI(utils, tco, lt, cost)

	if score.CapacityScore > 50 {
		t.Errorf("nearly full disk should have low capacity score, got %.2f", score.CapacityScore)
	}

	// Should have high priority recommendation
	hasHighPriority := false
	for _, rec := range score.Recommendations {
		if rec.Priority == "high" && rec.Category == "capacity" {
			hasHighPriority = true
			break
		}
	}
	if !hasHighPriority {
		t.Error("expected high priority capacity recommendation for nearly full disk")
	}
}

func TestROICalculator_CalculateROI_EmptyDisk(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-001")
	lt := setupTestLifetime("disk-001", 85, DiskStatusActive)

	utils := []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(0.5e12), int64(8e12)), // 6.25%
	}

	tco := calc.CalculateTCO(cost, lt)
	score := calc.CalculateROI(utils, tco, lt, cost)

	if score.CapacityScore > 20 {
		t.Errorf("nearly empty disk should have low capacity score, got %.2f", score.CapacityScore)
	}

	// Should have underutilization recommendation
	found := false
	for _, rec := range score.Recommendations {
		if rec.Category == "capacity" && rec.Title == "磁盘容量利用率过低" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected underutilization recommendation")
	}
}

func TestROICalculator_CalculateROI_FailedDisk(t *testing.T) {
	calc := setupTestCalculator()
	cost := setupTestDisk("disk-001")
	lt := setupTestLifetime("disk-001", 10, DiskStatusFailed)

	utils := []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(6e12), int64(8e12)),
	}

	tco := calc.CalculateTCO(cost, lt)
	score := calc.CalculateROI(utils, tco, lt, cost)

	if score.HealthScore != 10 {
		t.Errorf("expected health score 10, got %.2f", score.HealthScore)
	}
	if score.LifetimeScore != 0 {
		t.Errorf("expected lifetime score 0 for failed disk, got %.2f", score.LifetimeScore)
	}
	// Failed disk should need replacement recommendation
	found := false
	for _, rec := range score.Recommendations {
		if rec.Category == "lifetime" && rec.Title == "磁盘需要替换" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected replacement recommendation for failed disk")
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{90, "A"},
		{85, "B"},
		{80, "B"},
		{75, "C"},
		{70, "C"},
		{65, "D"},
		{60, "D"},
		{55, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("score %.1f: expected %s, got %s", tt.score, tt.grade, got)
		}
	}
}

func TestCalculateAverageUtilization(t *testing.T) {
	utils := []*CapacityUtilization{
		{TotalBytes: 100, UsedBytes: 50},
		{TotalBytes: 100, UsedBytes: 70},
		{TotalBytes: 100, UsedBytes: 30},
	}
	avg := calculateAverageUtilization(utils)
	expected := (50 + 70 + 30) / 3.0
	if avg != expected {
		t.Errorf("expected %.2f, got %.2f", expected, avg)
	}
}

func TestCalculateAverageUtilization_Empty(t *testing.T) {
	avg := calculateAverageUtilization(nil)
	if avg != 0 {
		t.Errorf("expected 0 for empty input, got %.2f", avg)
	}
}

func TestROICalculator_SetElectricityRate(t *testing.T) {
	calc := setupTestCalculator()
	calc.SetElectricityRate(1.2)
	if calc.electricityRate != 1.2 {
		t.Errorf("expected 1.2, got %.2f", calc.electricityRate)
	}
	// Zero/negative should be ignored
	calc.SetElectricityRate(0)
	if calc.electricityRate != 1.2 {
		t.Error("zero rate should be ignored")
	}
}

func TestROICalculator_SetDiskPowerWatts(t *testing.T) {
	calc := setupTestCalculator()
	calc.SetDiskPowerWatts(DiskTypeNVMe, 7.5)
	if calc.diskPowerWatts[DiskTypeNVMe] != 7.5 {
		t.Error("expected NVMe power to be 7.5W")
	}
}

func TestROICalculator_SetAnnualMaintenanceRate(t *testing.T) {
	calc := setupTestCalculator()
	calc.SetAnnualMaintenanceRate(0.05)
	if calc.annualMaintenanceRate != 0.05 {
		t.Error("expected 5% maintenance rate")
	}
	calc.SetAnnualMaintenanceRate(-1)
	if calc.annualMaintenanceRate != 0.05 {
		t.Error("negative rate should be ignored")
	}
}

// ==================== HTTP Handler 测试 ====================

func TestHandler_AddDiskCost(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	body := `{"id":"disk-001","serial_number":"SN001","model":"ST8000","vendor":"Seagate","disk_type":"hdd","capacity_bytes":8000000000000,"purchase_price":1200,"currency":"CNY","purchase_date":"2024-01-01T00:00:00Z","warranty_years":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageroi/disks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_AddDiskCost_NoID(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	body := `{"serial_number":"SN001","model":"ST8000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageroi/disks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_ListDiskCosts(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	h.diskCosts["disk-002"] = setupTestDisk("disk-002")
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/disks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetDiskCost_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/disks/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetDiskCost(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/disks/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_AddUtilization(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	body := `{"disk_id":"disk-001","total_bytes":8000000000000,"used_bytes":5000000000000,"reserved_bytes":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageroi/utilization", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_AddUtilization_NoDiskID(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	body := `{"total_bytes":8000000000000,"used_bytes":5000000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageroi/utilization", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetUtilization(t *testing.T) {
	h := setupTestHandler(t)
	h.utilizations["disk-001"] = []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(5e12), int64(8e12)),
	}
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/utilization/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetUtilization_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/utilization/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_AddLifetime(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	body := `{"disk_id":"disk-001","serial_number":"SN001","power_on_hours":17520,"estimated_tbw":1000,"actual_tbw":400,"status":"active","health_score":85,"reallocated_sectors":0,"temperature_max":45}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storageroi/lifetime", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetLifetime(t *testing.T) {
	h := setupTestHandler(t)
	h.lifetimes["disk-001"] = setupTestLifetime("disk-001", 85, DiskStatusActive)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/lifetime/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetROI(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	h.lifetimes["disk-001"] = setupTestLifetime("disk-001", 85, DiskStatusActive)
	h.utilizations["disk-001"] = []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(5e12), int64(8e12)),
	}
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/roi/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetROI_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/roi/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetTCO(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	h.lifetimes["disk-001"] = setupTestLifetime("disk-001", 85, DiskStatusActive)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/tco/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetTCO_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/tco/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetRecommendations(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	h.lifetimes["disk-001"] = setupTestLifetime("disk-001", 85, DiskStatusActive)
	h.utilizations["disk-001"] = []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(7.5e12), int64(8e12)), // 93.75%
	}
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/recommendations/disk-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	// Should have recommendations
	recs, ok := resp.Data.([]interface{})
	if !ok || len(recs) == 0 {
		t.Error("expected non-empty recommendations array")
	}
}

func TestHandler_GetRecommendations_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/recommendations/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetSummary(t *testing.T) {
	h := setupTestHandler(t)
	h.diskCosts["disk-001"] = setupTestDisk("disk-001")
	h.diskCosts["disk-002"] = setupTestDisk("disk-002")
	h.lifetimes["disk-001"] = setupTestLifetime("disk-001", 85, DiskStatusActive)
	h.lifetimes["disk-002"] = setupTestLifetime("disk-002", 60, DiskStatusDegraded)
	h.utilizations["disk-001"] = []*CapacityUtilization{
		setupTestUtilization("disk-001", int64(5e12), int64(8e12)),
	}
	h.utilizations["disk-002"] = []*CapacityUtilization{
		setupTestUtilization("disk-002", int64(7e12), int64(8e12)),
	}
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetSummary_Empty(t *testing.T) {
	h := setupTestHandler(t)
	r := setupTestRouter(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storageroi/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestNewHandlers_NilCalculator(t *testing.T) {
	h := NewHandlers(nil, nil)
	if h.calculator == nil {
		t.Error("expected non-nil calculator when nil provided")
	}
	if h.logger == nil {
		t.Error("expected non-nil logger when nil provided")
	}
}

// ==================== 综合场景测试 ====================

func TestScenario_MixedDiskPool(t *testing.T) {
	calc := setupTestCalculator()

	// 磁盘1: 健康、利用率适中、成本合理
	cost1 := &DiskCostRecord{
		ID:            "disk-1",
		DiskType:      DiskTypeSSD,
		CapacityBytes: 2e12, // 2TB
		PurchasePrice: 400,
		PurchaseDate:  time.Now().AddDate(-1, 0, 0),
		WarrantyYears: 5,
	}
	lt1 := &LifetimeTracker{
		DiskID:       "disk-1",
		PurchaseDate: time.Now().AddDate(-1, 0, 0),
		PowerOnHours: 8760,
		EstimatedTBW: 600,
		ActualTBW:    100,
		Status:       DiskStatusActive,
		HealthScore:  92,
	}
	utils1 := []*CapacityUtilization{
		setupTestUtilization("disk-1", int64(1.4e12), int64(2e12)), // 70%
	}
	tco1 := calc.CalculateTCO(cost1, lt1)
	score1 := calc.CalculateROI(utils1, tco1, lt1, cost1)

	// 磁盘2: 即将满、成本高
	cost2 := &DiskCostRecord{
		ID:            "disk-2",
		DiskType:      DiskTypeHDD,
		CapacityBytes: 8e12,
		PurchasePrice: 2500, // 偏贵
		PurchaseDate:  time.Now().AddDate(-3, 0, 0),
		WarrantyYears: 5,
	}
	lt2 := &LifetimeTracker{
		DiskID:             "disk-2",
		PurchaseDate:       time.Now().AddDate(-3, 0, 0),
		PowerOnHours:       26280,
		EstimatedTBW:       1000,
		ActualTBW:          950,
		Status:             DiskStatusDegraded,
		HealthScore:        35,
		ReallocatedSectors: 60,
	}
	utils2 := []*CapacityUtilization{
		setupTestUtilization("disk-2", int64(7.6e12), int64(8e12)), // 95%
	}
	tco2 := calc.CalculateTCO(cost2, lt2)
	score2 := calc.CalculateROI(utils2, tco2, lt2, cost2)

	// 磁盘1 应该明显优于磁盘2
	if score1.Score <= score2.Score {
		t.Errorf("disk-1 (%.1f) should outscore disk-2 (%.1f)", score1.Score, score2.Score)
	}
	if score1.Grade == "F" {
		t.Errorf("disk-1 should not get F grade, got %s", score1.Grade)
	}
	if score2.Grade != "F" && score2.Grade != "D" {
		t.Errorf("disk-2 should get D or F grade, got %s", score2.Grade)
	}

	// 磁盘2 应该有多个高优先级建议
	highPriorityCount := 0
	for _, rec := range score2.Recommendations {
		if rec.Priority == "high" {
			highPriorityCount++
		}
	}
	if highPriorityCount < 2 {
		t.Errorf("disk-2 should have at least 2 high priority recommendations, got %d", highPriorityCount)
	}
}
