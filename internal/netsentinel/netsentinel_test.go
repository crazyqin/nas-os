package netsentinel

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:            true,
		MonitorInterval:    5,
		AlertThresholdBps:  1000000000,
		DDoSThreshold:      10000,
		PortScanThreshold:  100,
		TrafficRetentionH:  24,
		AlertRetentionDays: 30,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateAlert(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	alert := manager.CreateAlert(AlertBandwidthSpike, SeverityWarning, "192.168.1.10", "10.0.0.1", "High bandwidth detected")
	if alert == nil {
		t.Fatal("CreateAlert returned nil")
	}
	if alert.Type != AlertBandwidthSpike {
		t.Errorf("expected bandwidth_spike, got %s", alert.Type)
	}
	if alert.Acknowledged {
		t.Error("new alert should not be acknowledged")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	alert := manager.CreateAlert(AlertPortScan, SeverityCritical, "1.2.3.4", "192.168.1.1", "Port scan detected")
	if err := manager.AcknowledgeAlert(alert.ID); err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	alerts := manager.ListAlerts()
	if !alerts[0].Acknowledged {
		t.Error("expected alert to be acknowledged")
	}
}

func TestRecordTraffic(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	manager.RecordTraffic(&TrafficRecord{
		SrcIP:    "192.168.1.10",
		DstIP:    "10.0.0.1",
		SrcPort:  12345,
		DstPort:  443,
		Protocol: "TCP",
		BytesIn:  1024,
		BytesOut: 512,
	})

	stats := manager.GetStats()
	if stats.TotalTrafficRecs != 1 {
		t.Errorf("expected 1 traffic record, got %d", stats.TotalTrafficRecs)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalAlerts != 0 {
		t.Errorf("expected 0 alerts, got %d", stats.TotalAlerts)
	}
}
