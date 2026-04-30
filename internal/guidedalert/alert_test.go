package guidedalert

import (
	"testing"
	"time"
)

func TestFireAndResolve(t *testing.T) {
	m := NewManager()

	// Fire a SMART warning
	alert := m.Fire("SMART_WARNING", "磁盘sda检测到坏扇区", "sda")
	if alert == nil {
		t.Fatal("Fire returned nil")
	}
	if alert.Code != "SMART_WARNING" {
		t.Errorf("expected code SMART_WARNING, got %s", alert.Code)
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("expected severity warning, got %s", alert.Severity)
	}
	if alert.Status != StatusOpen {
		t.Errorf("expected status open, got %s", alert.Status)
	}
	if len(alert.GuidedSteps) == 0 {
		t.Error("expected guided steps")
	}
	if len(alert.RootCauses) == 0 {
		t.Error("expected root causes")
	}
	if len(alert.MenuPath) == 0 {
		t.Error("expected menu path")
	}

	// Acknowledge
	err := m.Acknowledge(alert.ID, "admin")
	if err != nil {
		t.Fatalf("Acknowledge failed: %v", err)
	}
	updated, _ := m.Get(alert.ID)
	if updated.Status != StatusAcknowledged {
		t.Errorf("expected status acknowledged, got %s", updated.Status)
	}

	// Resolve
	err = m.Resolve(alert.ID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	resolved, _ := m.Get(alert.ID)
	if resolved.Status != StatusResolved {
		t.Errorf("expected status resolved, got %s", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestDeduplicate(t *testing.T) {
	m := NewManager()
	alert1 := m.Fire("SMART_WARNING", "msg1", "sda")
	alert2 := m.Fire("SMART_WARNING", "msg2", "sda")
	if alert1.ID != alert2.ID {
		t.Error("expected same resource alerts to be deduplicated")
	}
	if alert2.Message != "msg2" {
		t.Error("expected deduplicated alert to have updated message")
	}
}

func TestMenuIndicators(t *testing.T) {
	m := NewManager()
	m.Fire("SMART_WARNING", "msg1", "sda")
	m.Fire("POOL_DEGRADED", "msg2", "tank")
	indicators := m.GetMenuIndicators()
	if len(indicators) != 2 {
		t.Errorf("expected 2 indicators, got %d", len(indicators))
	}
}

func TestListFilter(t *testing.T) {
	m := NewManager()
	a1 := m.Fire("SMART_WARNING", "msg1", "sda")
	m.Fire("POOL_DEGRADED", "msg2", "tank")
	m.Resolve(a1.ID)
	time.Sleep(10 * time.Millisecond)
	openAlerts := m.List(StatusOpen, "")
	if len(openAlerts) != 1 {
		t.Errorf("expected 1 open alert, got %d", len(openAlerts))
	}
}
