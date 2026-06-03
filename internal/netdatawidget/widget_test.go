package netdatawidget

import (
	"testing"
)

func TestMetricRecording(t *testing.T) {
	tmpDir := t.TempDir()
	widget := NewNetdataWidget(tmpDir, 100)

	widget.RecordMetric("system.cpu", MetricCPU, "%", 45.5, nil)
	widget.RecordMetric("system.cpu", MetricCPU, "%", 50.2, nil)

	series, ok := widget.GetMetric("system.cpu")
	if !ok {
		t.Fatal("metric not found")
	}
	if series.Current != 50.2 {
		t.Fatalf("expected 50.2, got %f", series.Current)
	}
	if series.Min != 45.5 {
		t.Fatalf("expected min 45.5, got %f", series.Min)
	}
}

func TestWidgets(t *testing.T) {
	tmpDir := t.TempDir()
	widget := NewNetdataWidget(tmpDir, 100)

	widgets := widget.GetWidgets()
	if len(widgets) == 0 {
		t.Fatal("expected default widgets")
	}

	widget.AddWidget(&DashboardWidget{
		ID:    "custom",
		Title: "Custom Widget",
		Type:  "chart",
	})

	widgets = widget.GetWidgets()
	found := false
	for _, w := range widgets {
		if w.ID == "custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom widget not found")
	}
}

func TestAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	widget := NewNetdataWidget(tmpDir, 100)

	widget.AddWidget(&DashboardWidget{
		ID:    "cpu-alert",
		Title: "CPU Alert",
		Type:  "gauge",
		Metrics: []string{"system.cpu"},
		Thresholds: []Threshold{
			{Value: 90, Level: AlertCritical, Color: "#f44336"},
		},
	})

	widget.RecordMetric("system.cpu", MetricCPU, "%", 95, nil)

	allAlerts := widget.GetAlerts(false)
	if len(allAlerts) == 0 {
		t.Fatal("expected alerts")
	}

	widget.AcknowledgeAlert(allAlerts[0].ID)
	unacked := widget.GetAlerts(true)
	if len(unacked) != 0 {
		t.Fatalf("all alerts should be acked, got %d unacked", len(unacked))
	}
}

func TestSystemOverview(t *testing.T) {
	tmpDir := t.TempDir()
	widget := NewNetdataWidget(tmpDir, 100)

	widget.RecordMetric("system.cpu", MetricCPU, "%", 25, nil)
	widget.RecordMetric("system.memory", MetricMemory, "%", 60, nil)

	overview := widget.GetSystemOverview()
	if overview["cpu_usage"] != 25.0 {
		t.Fatal("cpu mismatch")
	}
}
