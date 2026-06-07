package alertdigest

import (
	"testing"
	"time"
)

func TestAddAlert(t *testing.T) {
	c := NewCollector()

	alert := c.AddAlert("disk", "Disk Warning", "Disk /dev/sda is 90% full", SeverityWarning)
	if alert.ID == "" {
		t.Error("expected non-empty alert ID")
	}
	if alert.Source != "disk" {
		t.Errorf("expected source 'disk', got %q", alert.Source)
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("expected severity warning, got %q", alert.Severity)
	}

	if c.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", c.PendingCount())
	}
}

func TestAckAlert(t *testing.T) {
	c := NewCollector()

	alert := c.AddAlert("network", "Network Down", "eth0 unreachable", SeverityCritical)

	if !c.AckAlert(alert.ID) {
		t.Error("expected AckAlert to return true")
	}
	if c.AckAlert("nonexistent") {
		t.Error("expected AckAlert to return false for nonexistent ID")
	}
}

func TestFlushPending(t *testing.T) {
	c := NewCollector()

	c.AddAlert("disk", "Alert 1", "msg1", SeverityCritical)
	c.AddAlert("disk", "Alert 2", "msg2", SeverityWarning)
	c.AddAlert("backup", "Alert 3", "msg3", SeverityInfo)
	c.AddAlert("disk", "Alert 4", "msg4", SeverityCritical)

	digest := c.FlushPending("hourly")
	if digest == nil {
		t.Fatal("expected non-nil digest")
	}

	if len(digest.Alerts) != 4 {
		t.Errorf("expected 4 alerts in digest, got %d", len(digest.Alerts))
	}
	if digest.Critical != 2 {
		t.Errorf("expected 2 critical, got %d", digest.Critical)
	}
	if digest.Warning != 1 {
		t.Errorf("expected 1 warning, got %d", digest.Warning)
	}
	if digest.Info != 1 {
		t.Errorf("expected 1 info, got %d", digest.Info)
	}
	if digest.Period != "hourly" {
		t.Errorf("expected period 'hourly', got %q", digest.Period)
	}

	// Pending should be empty after flush
	if c.PendingCount() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", c.PendingCount())
	}
}

func TestFlushEmpty(t *testing.T) {
	c := NewCollector()

	digest := c.FlushPending("daily")
	if digest != nil {
		t.Error("expected nil digest when no pending alerts")
	}
}

func TestMarkDelivered(t *testing.T) {
	c := NewCollector()

	c.AddAlert("test", "T", "M", SeverityInfo)
	digest := c.FlushPending("hourly")

	if digest.Delivered {
		t.Error("expected digest to not be delivered initially")
	}

	if !c.MarkDelivered(digest.ID) {
		t.Error("expected MarkDelivered to return true")
	}

	digests := c.GetDigests()
	if !digests[0].Delivered {
		t.Error("expected digest to be marked as delivered")
	}
}

func TestConfigureDigest(t *testing.T) {
	c := NewCollector()

	c.ConfigureDigest("email", DigestConfig{
		Channel:    "email",
		Interval:   30 * time.Minute,
		MinCount:   5,
		Severities: []Severity{SeverityCritical, SeverityWarning},
	})

	// Just verify no panic and config is stored
	c.mu.RLock()
	cfg, ok := c.configs["email"]
	c.mu.RUnlock()

	if !ok {
		t.Error("expected email config to be stored")
	}
	if cfg.Interval != 30*time.Minute {
		t.Errorf("expected 30m interval, got %v", cfg.Interval)
	}
	if cfg.MinCount != 5 {
		t.Errorf("expected min count 5, got %d", cfg.MinCount)
	}
}

func TestSummary(t *testing.T) {
	c := NewCollector()

	if c.Summary() != "No pending alerts" {
		t.Errorf("expected 'No pending alerts', got %q", c.Summary())
	}

	c.AddAlert("a", "A", "M", SeverityCritical)
	c.AddAlert("b", "B", "M", SeverityWarning)

	s := c.Summary()
	if s != "2 alerts: 1 critical, 1 warning, 0 info" {
		t.Errorf("unexpected summary: %q", s)
	}
}

func TestGetPending(t *testing.T) {
	c := NewCollector()

	c.AddAlert("a", "A", "M", SeverityInfo)
	c.AddAlert("b", "B", "M", SeverityWarning)

	pending := c.GetPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestGetDigests(t *testing.T) {
	c := NewCollector()

	c.AddAlert("a", "A", "M", SeverityInfo)
	c.FlushPending("hourly")

	c.AddAlert("b", "B", "M", SeverityWarning)
	c.FlushPending("daily")

	digests := c.GetDigests()
	if len(digests) != 2 {
		t.Errorf("expected 2 digests, got %d", len(digests))
	}
}
