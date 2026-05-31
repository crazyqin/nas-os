package gpumonitor

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(GPUConfig{
		MonitorInterval: time.Hour,
		TempWarning:     80,
		TempCritical:    90,
		PowerWarning:    90,
		VRAMWarning:     80,
		RetentionDays:   7,
	})
}

func TestRegisterAndGetGPU(t *testing.T) {
	m := newTestManager()
	gpu := &GPU{
		ID: "gpu-0", Name: "RTX 4090", Vendor: VendorNVIDIA,
		VRAMTotal: 24576, Driver: "535.86",
	}
	if err := m.RegisterGPU(gpu); err != nil {
		t.Fatalf("RegisterGPU failed: %v", err)
	}
	got, err := m.GetGPU("gpu-0")
	if err != nil {
		t.Fatalf("GetGPU failed: %v", err)
	}
	if got.Name != "RTX 4090" {
		t.Errorf("name = %q, want %q", got.Name, "RTX 4090")
	}
}

func TestListGPUs(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "GPU 1", Vendor: VendorNVIDIA})
	_ = m.RegisterGPU(&GPU{ID: "g2", Name: "GPU 2", Vendor: VendorAMD})
	list := m.ListGPUs()
	if len(list) != 2 {
		t.Errorf("ListGPUs() = %d, want 2", len(list))
	}
}

func TestUnregisterGPU(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "test", Vendor: VendorIntel})
	if err := m.UnregisterGPU("g1"); err != nil {
		t.Fatalf("UnregisterGPU failed: %v", err)
	}
	if _, err := m.GetGPU("g1"); err != ErrGPUNotFound {
		t.Errorf("expected ErrGPUNotFound, got %v", err)
	}
}

func TestUpdateGPU(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "test", Vendor: VendorNVIDIA, VRAMTotal: 8192})
	err := m.UpdateGPU("g1", &GPU{Temperature: 75, VRAMUsed: 4096, UtilizationGPU: 60, PowerDraw: 200})
	if err != nil {
		t.Fatalf("UpdateGPU failed: %v", err)
	}
	gpu, _ := m.GetGPU("g1")
	if gpu.Temperature != 75 {
		t.Errorf("temperature = %f, want 75", gpu.Temperature)
	}
	if gpu.VRAMUsed != 4096 {
		t.Errorf("vram_used = %d, want 4096", gpu.VRAMUsed)
	}
}

func TestGPUStats(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "NVIDIA", Vendor: VendorNVIDIA, VRAMTotal: 8192, Temperature: 60})
	_ = m.RegisterGPU(&GPU{ID: "g2", Name: "AMD", Vendor: VendorAMD, VRAMTotal: 16384, Temperature: 50})
	stats := m.GetStats()
	if stats.TotalGPUs != 2 {
		t.Errorf("TotalGPUs = %d, want 2", stats.TotalGPUs)
	}
	if stats.TotalVRAM != 24576 {
		t.Errorf("TotalVRAM = %d, want 24576", stats.TotalVRAM)
	}
}

func TestGPUAlerts(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "test", Vendor: VendorNVIDIA, Temperature: 95})
	m.checkAlerts()
	alerts := m.GetAlerts("g1", 10)
	if len(alerts) == 0 {
		t.Error("expected alerts for high temperature")
	}
	if len(alerts) > 0 && alerts[0].Level != AlertCritical {
		t.Errorf("alert level = %q, want %q", alerts[0].Level, AlertCritical)
	}
}

func TestMetricsCollection(t *testing.T) {
	m := newTestManager()
	_ = m.RegisterGPU(&GPU{ID: "g1", Name: "test", Vendor: VendorNVIDIA, Temperature: 60, PowerDraw: 150})
	m.collectMetrics()
	metrics := m.GetMetrics("g1", time.Now().Add(-time.Minute))
	if len(metrics) != 1 {
		t.Errorf("metrics count = %d, want 1", len(metrics))
	}
}

func TestStartStop(t *testing.T) {
	m := newTestManager()
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	m.Stop()
}
