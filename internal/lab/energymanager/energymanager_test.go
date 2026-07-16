package energymanager

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:            true,
		MonitoringInterval: 60,
		ElectricityRate:    0.6,
		Currency:           "CNY",
		CarbonFactor:       0.5,
		AlertThresholdW:    200,
		AutoPowerSave:      true,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if !manager.config.Enabled {
		t.Error("Expected config.Enabled to be true")
	}
}

func TestRecordReading(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	reading := &PowerReading{
		Timestamp: time.Now(),
		Watts:     45.5,
		Voltage:   12.0,
		Current:   3.79,
		Source:    SourceGrid,
	}

	manager.RecordReading(reading)

	stats := manager.GetStats()
	if stats.CurrentWatts != 45.5 {
		t.Errorf("Expected current watts 45.5, got %f", stats.CurrentWatts)
	}
}

func TestPowerState(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	if err := manager.SetPowerState(PowerStandby); err != nil {
		t.Fatalf("SetPowerState failed: %v", err)
	}

	stats := manager.GetStats()
	if stats.PowerState != PowerStandby {
		t.Errorf("Expected standby, got %s", stats.PowerState)
	}
}

func TestBudgetEstimation(t *testing.T) {
	config := &Config{
		Enabled:         true,
		ElectricityRate: 0.6,
		Currency:        "CNY",
	}
	manager := NewManager(config)

	// Record some readings
	for i := 0; i < 10; i++ {
		manager.RecordReading(&PowerReading{
			Timestamp: time.Now(),
			Watts:     50.0,
			Source:    SourceGrid,
		})
	}

	budget := manager.EstimateBill(30)
	if budget.CostPerKWh != 0.6 {
		t.Errorf("Expected cost per kWh 0.6, got %f", budget.CostPerKWh)
	}
	if budget.Currency != "CNY" {
		t.Errorf("Expected currency CNY, got %s", budget.Currency)
	}
}

func TestCarbonMetrics(t *testing.T) {
	config := &Config{
		Enabled:      true,
		CarbonFactor: 0.5,
	}
	manager := NewManager(config)

	metrics := manager.GetCarbonMetrics()
	if metrics.GridCarbonFactor != 0.5 {
		t.Errorf("Expected carbon factor 0.5, got %f", metrics.GridCarbonFactor)
	}
}

func TestProfiles(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	settings := PowerSettings{
		HDDSpindown: 30,
		LEDControl:  true,
		FanMode:     "quiet",
		WakeOnLAN:   true,
	}

	profile := manager.CreateProfile("Quiet Mode", "Minimal noise and power", "quiet", settings)
	if profile == nil {
		t.Fatal("CreateProfile returned nil")
	}

	got, ok := manager.GetProfile(profile.ID)
	if !ok {
		t.Fatal("GetProfile failed")
	}
	if got.Name != "Quiet Mode" {
		t.Errorf("Expected Quiet Mode, got %s", got.Name)
	}
	if !got.Settings.LEDControl {
		t.Error("Expected LEDControl to be true")
	}
}
