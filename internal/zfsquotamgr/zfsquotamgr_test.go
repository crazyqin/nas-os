package zfsquotamgr

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Fatal("expected default config")
	}
	if m.config.WarningThreshold != 80.0 {
		t.Fatalf("expected 80.0 warning threshold, got %f", m.config.WarningThreshold)
	}
}

func TestRegisterDataset(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	ds := &Dataset{
		Name:       "tank/data",
		MountPoint: "/mnt/tank/data",
		Quota:      1024 * 1024 * 1024 * 1024,
	}
	if err := m.RegisterDataset(ds); err != nil {
		t.Fatalf("RegisterDataset failed: %v", err)
	}
	if len(m.ListDatasets()) != 1 {
		t.Fatal("expected 1 dataset")
	}
}

func TestRegisterDatasetEmptyName(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	ds := &Dataset{Name: ""}
	if err := m.RegisterDataset(ds); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestUnregisterDataset(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data"})
	if err := m.UnregisterDataset("tank/data"); err != nil {
		t.Fatalf("UnregisterDataset failed: %v", err)
	}
	if len(m.ListDatasets()) != 0 {
		t.Fatal("expected 0 datasets")
	}
}

func TestUnregisterDatasetNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if err := m.UnregisterDataset("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent dataset")
	}
}

func TestGetDataset(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", MountPoint: "/mnt/tank/data"})
	ds, err := m.GetDataset("tank/data")
	if err != nil {
		t.Fatalf("GetDataset failed: %v", err)
	}
	if ds.MountPoint != "/mnt/tank/data" {
		t.Fatal("mount point mismatch")
	}
}

func TestGetDatasetNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	_, err := m.GetDataset("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent dataset")
	}
}

func TestSetDatasetQuota(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data"})
	if err := m.SetDatasetQuota("tank/data", 500*1024*1024*1024); err != nil {
		t.Fatalf("SetDatasetQuota failed: %v", err)
	}
	ds, _ := m.GetDataset("tank/data")
	if ds.Quota != 500*1024*1024*1024 {
		t.Fatal("quota mismatch")
	}
}

func TestSetDatasetQuotaNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if err := m.SetDatasetQuota("nonexistent", 100); err == nil {
		t.Fatal("expected error for nonexistent dataset")
	}
}

func TestUserQuota(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if err := m.SetUserQuota("user1", "alice", "tank/data", 100*1024*1024*1024); err != nil {
		t.Fatalf("SetUserQuota failed: %v", err)
	}
	q, err := m.GetUserQuota("user1", "tank/data")
	if err != nil {
		t.Fatalf("GetUserQuota failed: %v", err)
	}
	if q.Username != "alice" {
		t.Fatalf("expected alice, got %s", q.Username)
	}
	if q.Quota != 100*1024*1024*1024 {
		t.Fatal("quota mismatch")
	}
}

func TestGetUserQuotaNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	_, err := m.GetUserQuota("nonexistent", "tank/data")
	if err == nil {
		t.Fatal("expected error for nonexistent user quota")
	}
}

func TestListUserQuotas(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.SetUserQuota("user1", "alice", "tank/data", 100)
	m.SetUserQuota("user2", "bob", "tank/data", 200)
	m.SetUserQuota("user3", "charlie", "tank/backup", 300)

	all := m.ListUserQuotas("")
	if len(all) != 3 {
		t.Fatalf("expected 3 user quotas, got %d", len(all))
	}
	filtered := m.ListUserQuotas("tank/data")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 user quotas for tank/data, got %d", len(filtered))
	}
}

func TestGroupQuota(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	members := []string{"user1", "user2"}
	if err := m.SetGroupQuota("grp1", "developers", "tank/data", 500*1024*1024*1024, members); err != nil {
		t.Fatalf("SetGroupQuota failed: %v", err)
	}
	q, err := m.GetGroupQuota("grp1", "tank/data")
	if err != nil {
		t.Fatalf("GetGroupQuota failed: %v", err)
	}
	if q.GroupName != "developers" {
		t.Fatalf("expected developers, got %s", q.GroupName)
	}
	if len(q.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(q.Members))
	}
}

func TestGetGroupQuotaNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	_, err := m.GetGroupQuota("nonexistent", "tank/data")
	if err == nil {
		t.Fatal("expected error for nonexistent group quota")
	}
}

func TestListGroupQuotas(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.SetGroupQuota("grp1", "devs", "tank/data", 100, nil)
	m.SetGroupQuota("grp2", "ops", "tank/data", 200, nil)
	m.SetGroupQuota("grp3", "qa", "tank/backup", 300, nil)

	all := m.ListGroupQuotas("")
	if len(all) != 3 {
		t.Fatalf("expected 3 group quotas, got %d", len(all))
	}
	filtered := m.ListGroupQuotas("tank/data")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 group quotas for tank/data, got %d", len(filtered))
	}
}

func TestUpdateUsage(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000})
	if err := m.UpdateUsage("tank/data", 800); err != nil {
		t.Fatalf("UpdateUsage failed: %v", err)
	}
	ds, _ := m.GetDataset("tank/data")
	if ds.Used != 800 {
		t.Fatalf("expected 800, got %d", ds.Used)
	}
	if ds.Available != 200 {
		t.Fatalf("expected 200, got %d", ds.Available)
	}
}

func TestUpdateUsageNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if err := m.UpdateUsage("nonexistent", 100); err == nil {
		t.Fatal("expected error for nonexistent dataset")
	}
}

func TestQuotaAlertGeneration(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000, Used: 960})
	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alerts for high usage")
	}
}

func TestQuotaAlertThresholds(t *testing.T) {
	m := NewManager("/tmp/test-quota")

	// Below warning - no alert
	m.RegisterDataset(&Dataset{Name: "tank/safe", Quota: 1000, Used: 500})
	alerts := m.GetAlerts(false)
	for _, a := range alerts {
		if a.Dataset == "tank/safe" {
			t.Fatal("should not have alert for 50% usage")
		}
	}

	// Above critical
	m.RegisterDataset(&Dataset{Name: "tank/critical", Quota: 1000, Used: 960})
	alerts = m.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Dataset == "tank/critical" && a.Level == "critical" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical alert for 96% usage")
	}
}

func TestResolveAlert(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000, Used: 960})
	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alerts")
	}
	if err := m.ResolveAlert(alerts[0].ID); err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}
	resolved := m.GetAlerts(true)
	if len(resolved) == 0 {
		t.Fatal("expected resolved alerts")
	}
}

func TestResolveAlertNotFound(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	if err := m.ResolveAlert("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent alert")
	}
}

func TestGenerateRecommendations(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000, Used: 850})
	recs := m.GenerateRecommendations()
	if len(recs) == 0 {
		t.Fatal("expected recommendations for high usage")
	}
	if recs[0].Priority != "medium" {
		t.Fatalf("expected medium priority, got %s", recs[0].Priority)
	}
}

func TestGenerateRecommendationsCritical(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000, Used: 960})
	recs := m.GenerateRecommendations()
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	if recs[0].Priority != "high" {
		t.Fatalf("expected high priority, got %s", recs[0].Priority)
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	m.RegisterDataset(&Dataset{Name: "tank/data", Quota: 1000, Used: 500})
	m.RegisterDataset(&Dataset{Name: "tank/backup", Quota: 2000, Used: 1800})

	stats := m.GetStats()
	if stats.TotalDatasets != 2 {
		t.Fatalf("expected 2 datasets, got %d", stats.TotalDatasets)
	}
	if stats.TotalQuota != 3000 {
		t.Fatalf("expected 3000 total quota, got %d", stats.TotalQuota)
	}
	if stats.TotalUsed != 2300 {
		t.Fatalf("expected 2300 total used, got %d", stats.TotalUsed)
	}
}

func TestConfig(t *testing.T) {
	m := NewManager("/tmp/test-quota")
	cfg := m.GetConfig()
	if !cfg.AlertEnabled {
		t.Fatal("expected alerts enabled by default")
	}

	cfg.WarningThreshold = 75.0
	m.UpdateConfig(cfg)
	got := m.GetConfig()
	if got.WarningThreshold != 75.0 {
		t.Fatalf("expected 75.0, got %f", got.WarningThreshold)
	}
}
