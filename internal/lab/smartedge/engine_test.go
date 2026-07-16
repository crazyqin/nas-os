package smartedge

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(zap.NewNop())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestRegisterDevice(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	device := &IoTDevice{
		ID:      "device-1",
		Name:    "temp-sensor-01",
		Type:    DeviceTypeSensor,
		Address: "192.168.1.50",
	}

	if err := engine.RegisterDevice(device); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := engine.GetDevice("device-1")
	if !ok {
		t.Fatal("expected device to be registered")
	}
	if got.State != DeviceStateOnline {
		t.Errorf("expected state online, got %s", got.State)
	}
}

func TestIngestData(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.RegisterDevice(&IoTDevice{ID: "d1", Type: DeviceTypeSensor})

	point := &DataPoint{
		DeviceID: "d1",
		Metric:   "temperature",
		Value:    25.5,
		Unit:     "celsius",
	}

	if err := engine.IngestData(point); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	device, _ := engine.GetDevice("d1")
	if device.DataPoints != 1 {
		t.Errorf("expected 1 data point, got %d", device.DataPoints)
	}
}

func TestGetEdgeStats(t *testing.T) {
	engine := NewEngine(zap.NewNop())

	engine.RegisterDevice(&IoTDevice{ID: "d1", Type: DeviceTypeSensor})
	engine.RegisterDevice(&IoTDevice{ID: "d2", Type: DeviceTypeCamera})

	stats := engine.GetEdgeStats()

	if stats["total_devices"] != 2 {
		t.Errorf("expected 2 devices, got %v", stats["total_devices"])
	}
	if stats["online_devices"] != 2 {
		t.Errorf("expected 2 online devices, got %v", stats["online_devices"])
	}
}
