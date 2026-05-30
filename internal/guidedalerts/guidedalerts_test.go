package guidedalerts

import (
	"testing"
)

func TestNewAlertManager(t *testing.T) {
	am := NewAlertManager()
	if am == nil {
		t.Fatal("NewAlertManager returned nil")
	}
	if len(am.alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(am.alerts))
	}
}

func TestCreateAlert(t *testing.T) {
	am := NewAlertManager()

	alert := &Alert{
		ID:       "alert-1",
		Title:    "磁盘空间不足",
		Severity: SeverityWarning,
		Category: CategoryStorage,
	}

	am.CreateAlert(alert)

	got, ok := am.GetAlert("alert-1")
	if !ok {
		t.Fatal("GetAlert returned false")
	}
	if got.Title != "磁盘空间不足" {
		t.Errorf("expected title '磁盘空间不足', got '%s'", got.Title)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	am := NewAlertManager()
	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo, Category: CategorySystem})

	err := am.AcknowledgeAlert("a1")
	if err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	alert, _ := am.GetAlert("a1")
	if !alert.Acked {
		t.Error("expected alert to be acknowledged")
	}
}

func TestResolveAlert(t *testing.T) {
	am := NewAlertManager()
	am.CreateAlert(&Alert{ID: "a1", Title: "test", Severity: SeverityInfo, Category: CategorySystem})

	err := am.ResolveAlert("a1")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	alert, _ := am.GetAlert("a1")
	if !alert.Resolved {
		t.Error("expected alert to be resolved")
	}
	if alert.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestListAlerts(t *testing.T) {
	am := NewAlertManager()
	am.CreateAlert(&Alert{ID: "a1", Title: "t1", Severity: SeverityCritical, Category: CategoryStorage})
	am.CreateAlert(&Alert{ID: "a2", Title: "t2", Severity: SeverityWarning, Category: CategoryNetwork})
	am.CreateAlert(&Alert{ID: "a3", Title: "t3", Severity: SeverityCritical, Category: CategoryStorage, Resolved: true})

	// 全部
	all := am.ListAlerts("", "", false)
	if len(all) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(all))
	}

	// 只看 critical
	critical := am.ListAlerts(SeverityCritical, "", false)
	if len(critical) != 2 {
		t.Errorf("expected 2 critical alerts, got %d", len(critical))
	}

	// 只看未解决
	unresolved := am.ListAlerts("", "", true)
	if len(unresolved) != 2 {
		t.Errorf("expected 2 unresolved alerts, got %d", len(unresolved))
	}
}

func TestGetMenuBadges(t *testing.T) {
	am := NewAlertManager()
	am.CreateAlert(&Alert{
		ID:       "a1",
		Title:    "test",
		Severity: SeverityWarning,
		Category: CategoryStorage,
		MenuHint: &MenuHint{MenuPath: "/storage", Badge: true},
	})

	badges := am.GetMenuBadges()
	if badges["/storage"] != 1 {
		t.Errorf("expected badge count 1 for /storage, got %d", badges["/storage"])
	}
}

func TestGetStats(t *testing.T) {
	am := NewAlertManager()
	am.CreateAlert(&Alert{ID: "a1", Severity: SeverityCritical, Category: CategoryStorage})
	am.CreateAlert(&Alert{ID: "a2", Severity: SeverityWarning, Category: CategoryNetwork})
	am.CreateAlert(&Alert{ID: "a3", Severity: SeverityCritical, Category: CategoryStorage, Resolved: true})

	stats := am.GetStats()
	if stats["total"] != 3 {
		t.Errorf("expected total 3, got %d", stats["total"])
	}
	if stats["critical"] != 2 {
		t.Errorf("expected critical 2, got %d", stats["critical"])
	}
	if stats["resolved"] != 1 {
		t.Errorf("expected resolved 1, got %d", stats["resolved"])
	}
}
