package backupdashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewDashboard(t *testing.T) {
	d := NewDashboard(slog.Default())
	if d == nil {
		t.Fatal("Expected non-nil dashboard")
	}
	data := d.GetData()
	if data.GeneratedAt.IsZero() {
		t.Error("Expected non-zero GeneratedAt")
	}
}

func TestDashboard_GetOverview(t *testing.T) {
	d := NewDashboard(slog.Default())
	overview := d.GetOverview()

	if overview.TotalJobs == 0 {
		t.Error("Expected non-zero TotalJobs")
	}
	if overview.HealthScore < 0 || overview.HealthScore > 100 {
		t.Errorf("HealthScore should be 0-100, got %f", overview.HealthScore)
	}
	if overview.UpdatedAt.IsZero() {
		t.Error("Expected non-zero UpdatedAt")
	}
}

func TestDashboard_GetRecentJobs(t *testing.T) {
	d := NewDashboard(slog.Default())

	// 获取所有任务
	jobs := d.GetRecentJobs(0)
	if len(jobs) == 0 {
		t.Error("Expected non-empty jobs list")
	}

	// 获取限制数量
	jobs = d.GetRecentJobs(2)
	if len(jobs) > 2 {
		t.Errorf("Expected at most 2 jobs, got %d", len(jobs))
	}
}

func TestDashboard_GetAlerts(t *testing.T) {
	d := NewDashboard(slog.Default())

	// 不包含已解决的
	alerts := d.GetAlerts(false)
	for _, alert := range alerts {
		if alert.Resolved {
			t.Error("Should not include resolved alerts when includeResolved=false")
		}
	}

	// 包含已解决的
	allAlerts := d.GetAlerts(true)
	if len(allAlerts) < len(alerts) {
		t.Error("Should have at least as many alerts when including resolved")
	}
}

func TestDashboard_GetData(t *testing.T) {
	d := NewDashboard(slog.Default())
	data := d.GetData()

	if len(data.RecentJobs) == 0 {
		t.Error("Expected non-empty RecentJobs")
	}
	if len(data.Sources) == 0 {
		t.Error("Expected non-empty Sources")
	}
	if len(data.Trends) == 0 {
		t.Error("Expected non-empty Trends")
	}
}

func TestGenerateTrends(t *testing.T) {
	trends := generateTrends(7)
	if len(trends) != 7 {
		t.Errorf("Expected 7 trends, got %d", len(trends))
	}

	for _, trend := range trends {
		if trend.UsedBytes <= 0 {
			t.Error("Expected positive UsedBytes")
		}
		if trend.TotalBytes <= 0 {
			t.Error("Expected positive TotalBytes")
		}
		if trend.UsedBytes > trend.TotalBytes {
			t.Error("UsedBytes should not exceed TotalBytes")
		}
		if trend.Date == "" {
			t.Error("Expected non-empty Date")
		}
	}
}

func TestDashboard_RegisterRoutes(t *testing.T) {
	d := NewDashboard(slog.Default())
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	routes := []string{
		"/api/v1/backup/dashboard",
		"/api/v1/backup/dashboard/overview",
		"/api/v1/backup/dashboard/jobs",
		"/api/v1/backup/dashboard/trends",
		"/api/v1/backup/dashboard/alerts",
		"/api/v1/backup/dashboard/sources",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Route %s: expected status 200, got %d", route, w.Code)
		}
	}
}

func TestDashboard_HandleDashboard(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard", nil)
	w := httptest.NewRecorder()

	d.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var data DashboardData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if data.Overview.TotalJobs == 0 {
		t.Error("Expected non-zero TotalJobs in response")
	}
}

func TestDashboard_HandleOverview(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard/overview", nil)
	w := httptest.NewRecorder()

	d.handleOverview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var overview BackupOverview
	if err := json.NewDecoder(w.Body).Decode(&overview); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if overview.HealthScore == 0 {
		t.Error("Expected non-zero HealthScore")
	}
}

func TestDashboard_HandleRecentJobs(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard/jobs", nil)
	w := httptest.NewRecorder()

	d.handleRecentJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var jobs []RecentJob
	if err := json.NewDecoder(w.Body).Decode(&jobs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(jobs) == 0 {
		t.Error("Expected non-empty jobs")
	}
}

func TestDashboard_HandleTrends(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard/trends", nil)
	w := httptest.NewRecorder()

	d.handleTrends(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDashboard_HandleAlerts(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard/alerts", nil)
	w := httptest.NewRecorder()

	d.handleAlerts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDashboard_HandleSources(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/dashboard/sources", nil)
	w := httptest.NewRecorder()

	d.handleSources(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDashboard_MethodNotAllowed(t *testing.T) {
	d := NewDashboard(slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backup/dashboard", nil)
	w := httptest.NewRecorder()

	d.handleDashboard(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestBackupOverview_Fields(t *testing.T) {
	now := time.Now()
	overview := BackupOverview{
		TotalJobs:       10,
		ActiveJobs:      2,
		SuccessfulToday: 7,
		FailedToday:     1,
		TotalSources:    4,
		ProtectedData:   "1.5 TB",
		LastBackupTime:  now,
		NextBackupTime:  now.Add(time.Hour),
		HealthScore:     95.0,
		UpdatedAt:       now,
	}

	if overview.TotalJobs != 10 {
		t.Errorf("Expected 10 TotalJobs, got %d", overview.TotalJobs)
	}
	if overview.HealthScore != 95.0 {
		t.Errorf("Expected HealthScore=95.0, got %f", overview.HealthScore)
	}
}

func TestRecentJob_Fields(t *testing.T) {
	now := time.Now()
	job := RecentJob{
		ID:         "job-1",
		Name:       "Test Backup",
		Source:     "server-01",
		Type:       "full",
		Status:     "done",
		Size:       "50 GB",
		Duration:   "30m",
		StartedAt:  now.Add(-time.Hour),
		FinishedAt: now.Add(-30 * time.Minute),
	}

	if job.ID != "job-1" {
		t.Errorf("Expected ID=job-1, got %s", job.ID)
	}
	if job.Status != "done" {
		t.Errorf("Expected Status=done, got %s", job.Status)
	}
}

func TestBackupAlert_Fields(t *testing.T) {
	alert := BackupAlert{
		ID:        "alert-1",
		Level:     "warning",
		Message:   "Test alert",
		JobID:     "job-1",
		Source:    "server-01",
		Timestamp: time.Now(),
		Resolved:  false,
	}

	if alert.Level != "warning" {
		t.Errorf("Expected Level=warning, got %s", alert.Level)
	}
	if alert.Resolved {
		t.Error("Expected Resolved=false")
	}
}

func TestSourceStatus_Fields(t *testing.T) {
	source := SourceStatus{
		ID:           "src-1",
		Name:         "server-01",
		Platform:     "Linux",
		Status:       "online",
		LastBackup:   time.Now(),
		DataSize:     "500 GB",
		AgentVersion: "2.1.0",
		Uptime:       "30d",
	}

	if source.Status != "online" {
		t.Errorf("Expected Status=online, got %s", source.Status)
	}
	if source.Platform != "Linux" {
		t.Errorf("Expected Platform=Linux, got %s", source.Platform)
	}
}

func TestStorageTrend_Fields(t *testing.T) {
	trend := StorageTrend{
		Date:        "2026-06-24",
		UsedBytes:   2 * 1024 * 1024 * 1024 * 1024,
		TotalBytes:  4 * 1024 * 1024 * 1024 * 1024,
		GrowthBytes: 50 * 1024 * 1024 * 1024,
		GrowthRate:  1.22,
	}

	if trend.Date != "2026-06-24" {
		t.Errorf("Expected Date=2026-06-24, got %s", trend.Date)
	}
	if trend.UsedBytes >= trend.TotalBytes {
		t.Error("UsedBytes should be less than TotalBytes")
	}
}

func TestDashboard_ConcurrentAccess(t *testing.T) {
	d := NewDashboard(slog.Default())

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = d.GetData()
			_ = d.GetOverview()
			_ = d.GetRecentJobs(5)
			_ = d.GetAlerts(false)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent access")
		}
	}
}
