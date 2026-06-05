package smartwearleveling

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.policies) == 0 {
		t.Fatal("expected default policies")
	}
	if m.alertCfg == nil {
		t.Fatal("expected default alert config")
	}
}

func TestRegisterSSD(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{
		Device:      "/dev/nvme0n1",
		Model:       "Samsung 970 EVO",
		Capacity:    500 * 1024 * 1024 * 1024,
		TBWWritten:  100 * 1024 * 1024 * 1024,
		TBWMax:      600 * 1024 * 1024 * 1024,
		LifePercent: 85.0,
		Temperature: 45,
	}
	if err := m.RegisterSSD(ssd); err != nil {
		t.Fatalf("RegisterSSD failed: %v", err)
	}
	if ssd.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if ssd.Status != WearHealthy {
		t.Fatalf("expected healthy status, got %s", ssd.Status)
	}
}

func TestRegisterSSDEmptyDevice(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: ""}
	if err := m.RegisterSSD(ssd); err == nil {
		t.Fatal("expected error for empty device")
	}
}

func TestListSSDs(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme0n1", LifePercent: 90})
	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme1n1", LifePercent: 70})
	ssds := m.ListSSDs()
	if len(ssds) != 2 {
		t.Fatalf("expected 2 SSDs, got %d", len(ssds))
	}
}

func TestGetSSD(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: "/dev/nvme0n1", LifePercent: 90}
	m.RegisterSSD(ssd)
	got, err := m.GetSSD(ssd.ID)
	if err != nil {
		t.Fatalf("GetSSD failed: %v", err)
	}
	if got.Device != "/dev/nvme0n1" {
		t.Fatal("device mismatch")
	}
}

func TestGetSSDNotFound(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	_, err := m.GetSSD("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent SSD")
	}
}

func TestUnregisterSSD(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: "/dev/nvme0n1", LifePercent: 90}
	m.RegisterSSD(ssd)
	if err := m.UnregisterSSD(ssd.ID); err != nil {
		t.Fatalf("UnregisterSSD failed: %v", err)
	}
	if len(m.ListSSDs()) != 0 {
		t.Fatal("expected 0 SSDs after unregister")
	}
}

func TestUnregisterSSDNotFound(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	if err := m.UnregisterSSD("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent SSD")
	}
}

func TestWearStatusThresholds(t *testing.T) {
	m := NewManager("/tmp/test-wear")

	tests := []struct {
		life     float64
		expected WearStatus
	}{
		{90.0, WearHealthy},
		{55.0, WearHealthy},
		{50.0, WearModerate},
		{30.0, WearHigh},
		{10.0, WearCritical},
		{0.0, WearCritical},
	}

	for _, tt := range tests {
		ssd := &SSDInfo{Device: "/dev/test", LifePercent: tt.life}
		m.RegisterSSD(ssd)
		if ssd.Status != tt.expected {
			t.Errorf("life=%.1f: expected %s, got %s", tt.life, tt.expected, ssd.Status)
		}
	}
}

func TestPredictWear(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{
		Device:       "/dev/nvme0n1",
		LifePercent:  80.0,
		TBWWritten:   100 * 1024 * 1024 * 1024,
		TBWMax:       600 * 1024 * 1024 * 1024,
		PowerOnHours: 8760,
	}
	m.RegisterSSD(ssd)

	pred, err := m.PredictWear(ssd.ID)
	if err != nil {
		t.Fatalf("PredictWear failed: %v", err)
	}
	if pred.CurrentLife != 80.0 {
		t.Fatalf("expected current life 80, got %f", pred.CurrentLife)
	}
	if pred.PredictedLifeDays <= 0 {
		t.Fatal("expected positive predicted life days")
	}
	if pred.RiskLevel == "" {
		t.Fatal("expected non-empty risk level")
	}
}

func TestPredictWearNotFound(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	_, err := m.PredictWear("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent SSD")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme0n1", LifePercent: 90, TBWWritten: 1000})
	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme1n1", LifePercent: 25, TBWWritten: 2000})

	stats := m.GetStats()
	if stats.TotalSSDs != 2 {
		t.Fatalf("expected 2 SSDs, got %d", stats.TotalSSDs)
	}
	if stats.HealthySSDs != 1 {
		t.Fatalf("expected 1 healthy SSD, got %d", stats.HealthySSDs)
	}
	if stats.HighSSDs != 1 {
		t.Fatalf("expected 1 high SSD, got %d", stats.HighSSDs)
	}
	if stats.TotalTBW != 3000 {
		t.Fatalf("expected total TBW 3000, got %d", stats.TotalTBW)
	}
}

func TestAlertGeneration(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: "/dev/nvme0n1", LifePercent: 5.0}
	m.RegisterSSD(ssd)

	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alerts for critical SSD")
	}
}

func TestResolveAlert(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: "/dev/nvme0n1", LifePercent: 5.0}
	m.RegisterSSD(ssd)

	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alerts")
	}
	if err := m.ResolveAlert(alerts[0].ID); err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}
	if err := m.ResolveAlert("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent alert")
	}
}

func TestPolicies(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	policies := m.GetPolicies()
	if len(policies) == 0 {
		t.Fatal("expected default policies")
	}

	m.AddPolicy(&MigrationPolicy{
		ID:              "custom",
		Name:            "Custom Policy",
		SourceThreshold: 15.0,
		TargetThreshold: 70.0,
		Enabled:         true,
		AutoMigrate:     true,
	})
	policies = m.GetPolicies()
	found := false
	for _, p := range policies {
		if p.ID == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("custom policy not found")
	}
}

func TestEvaluateMigrations(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	m.AddPolicy(&MigrationPolicy{
		ID:              "test",
		Name:            "Test Policy",
		SourceThreshold: 20.0,
		TargetThreshold: 60.0,
		Enabled:         true,
		AutoMigrate:     true,
	})

	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme0n1", LifePercent: 15.0})
	m.RegisterSSD(&SSDInfo{Device: "/dev/nvme1n1", LifePercent: 80.0})

	jobs := m.EvaluateMigrations()
	if len(jobs) == 0 {
		t.Fatal("expected migration jobs")
	}
	if jobs[0].Status != "pending" {
		t.Fatalf("expected pending status, got %s", jobs[0].Status)
	}
}

func TestUpdateSSDStats(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	ssd := &SSDInfo{Device: "/dev/nvme0n1", LifePercent: 90, Temperature: 40}
	m.RegisterSSD(ssd)

	if err := m.UpdateSSDStats(ssd.ID, &SSDInfo{Temperature: 50, LifePercent: 85}); err != nil {
		t.Fatalf("UpdateSSDStats failed: %v", err)
	}
	if ssd.Temperature != 50 {
		t.Fatalf("expected temperature 50, got %d", ssd.Temperature)
	}
}

func TestUpdateSSDStatsNotFound(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	if err := m.UpdateSSDStats("nonexistent", &SSDInfo{}); err == nil {
		t.Fatal("expected error for nonexistent SSD")
	}
}

func TestAlertConfig(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	cfg := m.GetAlertConfig()
	if !cfg.Enabled {
		t.Fatal("expected alerts enabled by default")
	}

	cfg.LifeWarningThreshold = 25.0
	m.UpdateAlertConfig(cfg)
	got := m.GetAlertConfig()
	if got.LifeWarningThreshold != 25.0 {
		t.Fatalf("expected 25.0, got %f", got.LifeWarningThreshold)
	}
}

func TestGetJobs(t *testing.T) {
	m := NewManager("/tmp/test-wear")
	if len(m.GetJobs()) != 0 {
		t.Fatal("expected 0 jobs initially")
	}
}
