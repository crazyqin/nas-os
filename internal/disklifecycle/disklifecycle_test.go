package disklifecycle

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct{}

func (l *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = tmpDir
	config.EnablePrediction = true
	mgr, err := NewManager(config, &mockLogger{})
	require.NoError(t, err)
	return mgr
}

func TestRegisterDisk(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{
		Device:      "/dev/sda",
		Serial:      "ABC123",
		Model:       "Samsung 990 Pro",
		Vendor:      "Samsung",
		Interface:   "NVMe",
		InstallDate: time.Now().AddDate(-1, 0, 0),
	}
	err := mgr.RegisterDisk(disk)
	require.NoError(t, err)
	assert.NotEmpty(t, disk.ID)
	assert.Equal(t, StatusUnknown, disk.Status)
}

func TestListDisks(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	mgr.RegisterDisk(&Disk{Device: "/dev/sda", Model: "Disk A", HealthScore: 90})
	mgr.RegisterDisk(&Disk{Device: "/dev/sdb", Model: "Disk B", HealthScore: 50})

	disks := mgr.ListDisks()
	assert.Len(t, disks, 2)
	// Should be sorted by health (worst first)
	assert.Equal(t, "Disk B", disks[0].Model)
	assert.Equal(t, "Disk A", disks[1].Model)
}

func TestUpdateSMARTData(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{
		Device:      "/dev/sda",
		InstallDate: time.Now().AddDate(-2, 0, 0),
	}
	mgr.RegisterDisk(disk)

	err := mgr.UpdateSMARTData(disk.ID, SMARTData{
		HealthOK:     true,
		Temperature:  45.0,
		PowerOnHours: 10000,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetDisk(disk.ID)
	assert.Greater(t, updated.HealthScore, 0.0)
	assert.Equal(t, SmartPassed, updated.SmartStatus)
	assert.Equal(t, 45.0, updated.Temperature)
}

func TestUpdateSMARTData_WithReallocatedSectors(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	err := mgr.UpdateSMARTData(disk.ID, SMARTData{
		HealthOK:           true,
		ReallocatedSectors: 10,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetDisk(disk.ID)
	assert.Less(t, updated.HealthScore, 100.0)
	assert.Equal(t, StatusWarning, updated.Status)
}

func TestUpdateSMARTData_FailedSMART(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	err := mgr.UpdateSMARTData(disk.ID, SMARTData{
		HealthOK: false,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetDisk(disk.ID)
	assert.Equal(t, 0.0, updated.HealthScore)
	assert.Equal(t, SmartFailed, updated.SmartStatus)
	assert.Equal(t, StatusFailed, updated.Status)
}

func TestRetireDisk(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	err := mgr.RetireDisk(disk.ID, "End of life")
	require.NoError(t, err)

	updated, _ := mgr.GetDisk(disk.ID)
	assert.Equal(t, StatusRetired, updated.Status)
}

func TestUnregisterDisk(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	err := mgr.UnregisterDisk(disk.ID)
	require.NoError(t, err)

	_, err = mgr.GetDisk(disk.ID)
	assert.Error(t, err)
}

func TestGetPrediction(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{
		Device:      "/dev/sda",
		InstallDate: time.Now().AddDate(-4, 0, 0),
	}
	mgr.RegisterDisk(disk)

	mgr.UpdateSMARTData(disk.ID, SMARTData{
		HealthOK:           true,
		ReallocatedSectors: 5,
		Temperature:        58,
	})

	prediction, err := mgr.GetPrediction(disk.ID)
	require.NoError(t, err)
	assert.NotNil(t, prediction)
	assert.Greater(t, prediction.FailureProb, 0.0)
	assert.NotEmpty(t, prediction.Recommendation)
}

func TestFleetSummary(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	mgr.RegisterDisk(&Disk{Device: "/dev/sda", Vendor: "Samsung", Interface: "NVMe", CapacityBytes: 1000000000})
	mgr.RegisterDisk(&Disk{Device: "/dev/sdb", Vendor: "WD", Interface: "SATA", CapacityBytes: 2000000000})

	summary := mgr.GetFleetSummary()
	assert.Equal(t, 2, summary.TotalDisks)
	assert.Equal(t, int64(3000000000), summary.TotalCapacity)
	assert.Equal(t, 1, summary.ByInterface["NVMe"])
	assert.Equal(t, 1, summary.ByInterface["SATA"])
}

func TestAlerts(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	// Trigger health alert
	mgr.UpdateSMARTData(disk.ID, SMARTData{
		HealthOK:    true,
		Temperature: 75, // High temp
	})

	alerts := mgr.GetAlerts(false)
	assert.NotEmpty(t, alerts)
}

func TestDismissAlert(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)
	mgr.UpdateSMARTData(disk.ID, SMARTData{HealthOK: true, Temperature: 75})

	alerts := mgr.GetAlerts(false)
	if len(alerts) > 0 {
		err := mgr.DismissAlert(alerts[0].ID)
		require.NoError(t, err)

		activeAlerts := mgr.GetAlerts(false)
		dismissedAlerts := mgr.GetAlerts(true)
		assert.Less(t, len(activeAlerts), len(alerts))
		assert.NotEmpty(t, dismissedAlerts)
	}
}

func TestEvents(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)

	events := mgr.GetEvents(disk.ID, 10)
	assert.NotEmpty(t, events)
	assert.Equal(t, EventDiskAdded, events[0].Type)
}

func TestHealthTrend(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	disk := &Disk{Device: "/dev/sda", InstallDate: time.Now().AddDate(-2, 0, 0)}
	mgr.RegisterDisk(disk)

	// Simulate declining health over time
	for i := 0; i < 5; i++ {
		mgr.UpdateSMARTData(disk.ID, SMARTData{
			HealthOK:    true,
			Temperature: 40 + float64(i)*5,
		})
	}

	updated, _ := mgr.GetDisk(disk.ID)
	assert.Len(t, updated.HealthHistory, 5)
}

func TestHandler_Disks(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Register disk (POST not supported on new handler, use RegisterDisk directly)
	disk := &Disk{Device: "/dev/sda", Model: "Test Disk"}
	mgr.RegisterDisk(disk)

	// List disks
	req := httptest.NewRequest(http.MethodGet, "/api/disklifecycle/disks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandler_GetDisk(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Register disk
	disk := &Disk{Device: "/dev/sda", Model: "Test Disk"}
	mgr.RegisterDisk(disk)

	// Get disk by ID
	req := httptest.NewRequest(http.MethodGet, "/api/disklifecycle/disks/"+disk.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandler_Predict(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Register disk
	disk := &Disk{Device: "/dev/sda", Model: "Test Disk"}
	mgr.RegisterDisk(disk)

	// Trigger prediction
	req := httptest.NewRequest(http.MethodPost, "/api/disklifecycle/disks/"+disk.ID+"/predict", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandler_Retire(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Register disk
	disk := &Disk{Device: "/dev/sda", Model: "Test Disk"}
	mgr.RegisterDisk(disk)

	// Retire disk
	body, _ := json.Marshal(map[string]string{"reason": "End of life"})
	req := httptest.NewRequest(http.MethodPost, "/api/disklifecycle/disks/"+disk.ID+"/retire", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])

	// Verify disk is retired
	updated, _ := mgr.GetDisk(disk.ID)
	assert.Equal(t, StatusRetired, updated.Status)
}

func TestHandler_Alerts(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Register disk and trigger alert
	disk := &Disk{Device: "/dev/sda"}
	mgr.RegisterDisk(disk)
	mgr.UpdateSMARTData(disk.ID, SMARTData{HealthOK: true, Temperature: 75})

	req := httptest.NewRequest(http.MethodGet, "/api/disklifecycle/alerts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

func TestHandler_Report(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/disklifecycle/report", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["totalDisks"])
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, 6, config.ScanIntervalHours)
	assert.Equal(t, 70.0, config.HealthThreshold)
	assert.Equal(t, 30.0, config.RetireThreshold)
	assert.True(t, config.EnablePrediction)
}
