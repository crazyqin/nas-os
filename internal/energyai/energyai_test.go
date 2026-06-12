package energyai

import (
	"context"
	"testing"
	"time"
)

func TestRecordEnergy(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	record := EnergyRecord{
		Timestamp:   time.Now(),
		Watts:       150.5,
		Voltage:     12.0,
		Current:     12.5,
		Component:   "cpu",
		Temperature: 45.0,
	}

	err := ea.RecordEnergy(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ea.records) != 1 {
		t.Errorf("expected 1 record, got %d", len(ea.records))
	}
}

func TestPredictEnergy(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	// Add enough records
	for i := 0; i < 24; i++ {
		ea.RecordEnergy(ctx, EnergyRecord{
			Timestamp: time.Now().Add(-time.Duration(24-i) * time.Hour),
			Watts:     100.0 + float64(i)*10,
			Component: "total",
		})
	}

	prediction, err := ea.PredictEnergy(ctx, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prediction.PredictedWatts <= 0 {
		t.Errorf("expected positive prediction, got %f", prediction.PredictedWatts)
	}
	if prediction.Confidence < 0 || prediction.Confidence > 1 {
		t.Errorf("confidence should be 0-1, got %f", prediction.Confidence)
	}
}

func TestPredictEnergyInsufficientData(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	// Add only 5 records
	for i := 0; i < 5; i++ {
		ea.RecordEnergy(ctx, EnergyRecord{
			Timestamp: time.Now(),
			Watts:     100.0,
			Component: "total",
		})
	}

	_, err := ea.PredictEnergy(ctx, time.Hour)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestCalculateCarbonFootprint(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()
	ea.SetGridIntensity(500) // gCO2/kWh

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	// Add records
	for i := 0; i < 24; i++ {
		ea.RecordEnergy(ctx, EnergyRecord{
			Timestamp: start.Add(time.Duration(i) * time.Hour),
			Watts:     200.0,
			Component: "total",
		})
	}

	cfp, err := ea.CalculateCarbonFootprint(ctx, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfp.EnergyKWh <= 0 {
		t.Errorf("expected positive energy, got %f", cfp.EnergyKWh)
	}
	if cfp.CarbonKg <= 0 {
		t.Errorf("expected positive carbon, got %f", cfp.CarbonKg)
	}
}

func TestCalculateCarbonFootprintNoData(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	_, err := ea.CalculateCarbonFootprint(ctx, start, end)
	if err == nil {
		t.Fatal("expected error for no data")
	}
}

func TestCreateProfile(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	profile := &PowerProfile{
		ID:          "eco-mode",
		Name:        "Eco Mode",
		Description: "Power saving mode",
		Rules: []PowerRule{
			{
				Component: "disk",
				Condition: "threshold",
				Threshold: 30,
				Action:    "standby",
				DelaySec:  300,
			},
		},
		Enabled: true,
	}

	err := ea.CreateProfile(ctx, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profiles := ea.GetProfiles(ctx)
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(profiles))
	}
}

func TestCreateProfileNoID(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	profile := &PowerProfile{
		Name: "No ID",
	}

	err := ea.CreateProfile(ctx, profile)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestGetStats(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	// Add records
	for i := 0; i < 10; i++ {
		ea.RecordEnergy(ctx, EnergyRecord{
			Timestamp: time.Now(),
			Watts:     100.0 + float64(i)*10,
			Component: "cpu",
		})
	}

	stats, err := ea.GetStats(ctx, "today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.AvgWatts <= 0 {
		t.Errorf("expected positive avg watts, got %f", stats.AvgWatts)
	}
	if stats.PeakWatts < stats.AvgWatts {
		t.Errorf("peak should be >= avg")
	}
}

func TestGetStatsEmpty(t *testing.T) {
	ea := NewEnergyAI()
	ctx := context.Background()

	stats, err := ea.GetStats(ctx, "today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.AvgWatts != 0 {
		t.Errorf("expected 0 avg watts, got %f", stats.AvgWatts)
	}
}

func TestSetGridIntensity(t *testing.T) {
	ea := NewEnergyAI()
	ea.SetGridIntensity(300)

	if ea.gridIntensity != 300 {
		t.Errorf("expected 300, got %f", ea.gridIntensity)
	}
}

func TestSet电价(t *testing.T) {
	ea := NewEnergyAI()
	ea.Set电价(0.15)

	if ea.电价PerKWh != 0.15 {
		t.Errorf("expected 0.15, got %f", ea.电价PerKWh)
	}
}
