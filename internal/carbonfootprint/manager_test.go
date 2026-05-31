package carbonfootprint

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Error("config not initialized")
	}
	if m.intensities == nil {
		t.Error("intensities not initialized")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		DefaultRegion:    "CN",
		GridIntensity:    500.0,
		SolarIntensity:   40.0,
		WindIntensity:    15.0,
		CarbonOffsetCost: 0.03,
		TreeAbsorptionKg: 25.0,
	}

	m := NewManager(config)
	if m.config.DefaultRegion != "CN" {
		t.Errorf("expected region 'CN', got '%s'", m.config.DefaultRegion)
	}
}

func TestRecordEnergy(t *testing.T) {
	m := NewManager(nil)

	record := &EnergyRecord{
		Source:    SourceGrid,
		Wh:        1000,
		Service:   "nas",
		Device:    "server",
	}

	err := m.RecordEnergy(record)
	if err != nil {
		t.Fatalf("RecordEnergy failed: %v", err)
	}

	if record.ID == "" {
		t.Error("record ID not generated")
	}
	if record.Timestamp.IsZero() {
		t.Error("timestamp not set")
	}
}

func TestCalculateFootprint(t *testing.T) {
	m := NewManager(nil)

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// Record some energy
	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceGrid,
		Wh:        1000,
	})
	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceSolar,
		Wh:        500,
	})

	ctx := context.Background()
	footprint, err := m.CalculateFootprint(ctx, start, end)
	if err != nil {
		t.Fatalf("CalculateFootprint failed: %v", err)
	}

	if footprint.EnergyWh != 1500 {
		t.Errorf("expected 1500 Wh, got %f", footprint.EnergyWh)
	}

	if footprint.CarbonG <= 0 {
		t.Error("carbon should be positive")
	}

	if footprint.CarbonKg <= 0 {
		t.Error("carbon kg should be positive")
	}
}

func TestGetDailyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordEnergy(&EnergyRecord{
		Source: SourceGrid,
		Wh:     1000,
	})

	ctx := context.Background()
	footprint, err := m.GetDailyFootprint(ctx)
	if err != nil {
		t.Fatalf("GetDailyFootprint failed: %v", err)
	}

	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestGetWeeklyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordEnergy(&EnergyRecord{
		Source: SourceGrid,
		Wh:     1000,
	})

	ctx := context.Background()
	footprint, err := m.GetWeeklyFootprint(ctx)
	if err != nil {
		t.Fatalf("GetWeeklyFootprint failed: %v", err)
	}

	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestGetMonthlyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordEnergy(&EnergyRecord{
		Source: SourceGrid,
		Wh:     1000,
	})

	ctx := context.Background()
	footprint, err := m.GetMonthlyFootprint(ctx)
	if err != nil {
		t.Fatalf("GetMonthlyFootprint failed: %v", err)
	}

	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestGetRecommendations(t *testing.T) {
	m := NewManager(nil)

	// Add some records with high grid usage
	for i := 0; i < 10; i++ {
		m.RecordEnergy(&EnergyRecord{
			Source: SourceGrid,
			Wh:     1000,
		})
	}

	ctx := context.Background()
	recs, err := m.GetRecommendations(ctx)
	if err != nil {
		t.Fatalf("GetRecommendations failed: %v", err)
	}

	if len(recs) == 0 {
		t.Error("expected recommendations")
	}
}

func TestGetRecords(t *testing.T) {
	m := NewManager(nil)

	now := time.Now()
	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceGrid,
		Wh:        1000,
	})

	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	records := m.GetRecords(start, end)
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestUpdateIntensity(t *testing.T) {
	m := NewManager(nil)

	m.UpdateIntensity(SourceGrid, 500.0)

	m.mu.RLock()
	intensity := m.intensities[SourceGrid]
	m.mu.RUnlock()

	if intensity.Intensity != 500.0 {
		t.Errorf("expected intensity 500.0, got %f", intensity.Intensity)
	}
}

func TestSourceBreakdown(t *testing.T) {
	m := NewManager(nil)

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceGrid,
		Wh:        1000,
	})
	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceSolar,
		Wh:        500,
	})

	ctx := context.Background()
	footprint, _ := m.CalculateFootprint(ctx, start, end)

	if footprint.SourceBreakdown[SourceGrid] != 1000 {
		t.Errorf("expected grid 1000, got %f", footprint.SourceBreakdown[SourceGrid])
	}
	if footprint.SourceBreakdown[SourceSolar] != 500 {
		t.Errorf("expected solar 500, got %f", footprint.SourceBreakdown[SourceSolar])
	}
}

func TestTreesNeeded(t *testing.T) {
	config := DefaultConfig()
	config.TreeAbsorptionKg = 22.0

	m := NewManager(config)

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// Add enough energy to require trees
	m.RecordEnergy(&EnergyRecord{
		Timestamp: now,
		Source:    SourceGrid,
		Wh:        100000, // 100 kWh
	})

	ctx := context.Background()
	footprint, _ := m.CalculateFootprint(ctx, start, end)

	if footprint.TreesNeeded <= 0 {
		t.Error("trees needed should be positive")
	}
}
