package greencomputing

import (
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
	if m.readings == nil {
		t.Error("readings not initialized")
	}
	if m.strategies == nil {
		t.Error("strategies not initialized")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		Enabled:            true,
		MonitoringInterval: 30,
		GridIntensity:      500.0,
		SolarIntensity:     40.0,
		WindIntensity:      15.0,
		CarbonOffsetCost:   0.03,
		TreeAbsorptionKg:   25.0,
		ElectricityRate:    0.15,
		Currency:           "CNY",
		IdleThreshold:      20,
	}

	m := NewManager(config)
	if m.config.GridIntensity != 500.0 {
		t.Errorf("expected grid intensity 500.0, got %f", m.config.GridIntensity)
	}
	if m.config.Currency != "CNY" {
		t.Errorf("expected currency CNY, got %s", m.config.Currency)
	}
}

func TestRecordReading(t *testing.T) {
	m := NewManager(nil)

	reading := &EnergyReading{
		TotalWatts:   75.5,
		CPUWatts:     30.0,
		DiskWatts:    20.0,
		NetworkWatts: 10.0,
		FanWatts:     5.0,
		OtherWatts:   10.5,
		Source:       SourceGrid,
	}

	m.RecordReading(reading)

	if reading.Timestamp.IsZero() {
		t.Error("timestamp not set")
	}

	latest := m.GetLatestReading()
	if latest == nil {
		t.Fatal("latest reading is nil")
	}
	if latest.TotalWatts != 75.5 {
		t.Errorf("expected total watts 75.5, got %f", latest.TotalWatts)
	}
}

func TestGetReadings(t *testing.T) {
	m := NewManager(nil)

	now := time.Now()
	m.RecordReading(&EnergyReading{
		Timestamp:  now.Add(-30 * time.Minute),
		TotalWatts: 50,
	})
	m.RecordReading(&EnergyReading{
		Timestamp:  now.Add(-15 * time.Minute),
		TotalWatts: 60,
	})
	m.RecordReading(&EnergyReading{
		Timestamp:  now,
		TotalWatts: 70,
	})

	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Minute)

	readings := m.GetReadings(start, end)
	if len(readings) != 3 {
		t.Errorf("expected 3 readings, got %d", len(readings))
	}
}

func TestCalculateFootprint(t *testing.T) {
	m := NewManager(nil)

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	m.RecordReading(&EnergyReading{
		Timestamp:  now,
		TotalWatts: 100,
		CPUWatts:   40,
		DiskWatts:  30,
	})
	m.RecordReading(&EnergyReading{
		Timestamp:  now,
		TotalWatts: 50,
		CPUWatts:   20,
		DiskWatts:  15,
	})

	footprint := m.CalculateFootprint(start, end)
	if footprint.Period == "" {
		t.Error("period not set")
	}
	if footprint.CarbonKg <= 0 {
		t.Error("carbon should be positive")
	}
}

func TestGetDailyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordReading(&EnergyReading{
		TotalWatts: 100,
	})

	footprint := m.GetDailyFootprint()
	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestGetWeeklyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordReading(&EnergyReading{
		TotalWatts: 100,
	})

	footprint := m.GetWeeklyFootprint()
	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestGetMonthlyFootprint(t *testing.T) {
	m := NewManager(nil)

	m.RecordReading(&EnergyReading{
		TotalWatts: 100,
	})

	footprint := m.GetMonthlyFootprint()
	if footprint.Period == "" {
		t.Error("period not set")
	}
}

func TestCreateStrategy(t *testing.T) {
	m := NewManager(nil)

	strategy := m.CreateStrategy("夜间休眠", "夜间自动降低功耗", 30*time.Minute)
	if strategy == nil {
		t.Fatal("CreateStrategy returned nil")
	}
	if strategy.Name != "夜间休眠" {
		t.Errorf("expected name '夜间休眠', got %s", strategy.Name)
	}
	if strategy.DiskSpindown != true {
		t.Error("expected DiskSpindown to be true")
	}
	if strategy.CPUGovernor != "powersave" {
		t.Errorf("expected CPUGovernor 'powersave', got %s", strategy.CPUGovernor)
	}
}

func TestGetStrategy(t *testing.T) {
	m := NewManager(nil)

	created := m.CreateStrategy("Test", "Desc", 30*time.Minute)
	found, ok := m.GetStrategy(created.ID)
	if !ok {
		t.Fatal("GetStrategy failed")
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestListStrategies(t *testing.T) {
	m := NewManager(nil)

	m.CreateStrategy("Strategy 1", "", 10*time.Minute)
	m.CreateStrategy("Strategy 2", "", 20*time.Minute)

	strategies := m.ListStrategies()
	if len(strategies) != 2 {
		t.Errorf("expected 2 strategies, got %d", len(strategies))
	}
}

func TestUpdateStrategy(t *testing.T) {
	m := NewManager(nil)

	created := m.CreateStrategy("Old Name", "Old Desc", 30*time.Minute)

	updates := &SleepStrategy{
		Name:          "New Name",
		IdleThreshold: 60 * time.Minute,
		DiskSpindown:  false,
	}

	err := m.UpdateStrategy(created.ID, updates)
	if err != nil {
		t.Fatalf("UpdateStrategy failed: %v", err)
	}

	found, _ := m.GetStrategy(created.ID)
	if found.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %s", found.Name)
	}
	if found.IdleThreshold != 60*time.Minute {
		t.Errorf("expected idle threshold 60m, got %v", found.IdleThreshold)
	}
	if found.DiskSpindown != false {
		t.Error("expected DiskSpindown to be false")
	}
}

func TestActivateStrategy(t *testing.T) {
	m := NewManager(nil)

	s1 := m.CreateStrategy("Strategy 1", "", 10*time.Minute)
	s2 := m.CreateStrategy("Strategy 2", "", 20*time.Minute)

	err := m.ActivateStrategy(s1.ID)
	if err != nil {
		t.Fatalf("ActivateStrategy failed: %v", err)
	}

	found1, _ := m.GetStrategy(s1.ID)
	found2, _ := m.GetStrategy(s2.ID)

	if !found1.IsActive {
		t.Error("expected strategy 1 to be active")
	}
	if found2.IsActive {
		t.Error("expected strategy 2 to not be active")
	}
}

func TestDeleteStrategy(t *testing.T) {
	m := NewManager(nil)

	created := m.CreateStrategy("Test", "", 10*time.Minute)

	err := m.DeleteStrategy(created.ID)
	if err != nil {
		t.Fatalf("DeleteStrategy failed: %v", err)
	}

	strategies := m.ListStrategies()
	if len(strategies) != 0 {
		t.Errorf("expected 0 strategies, got %d", len(strategies))
	}
}

func TestGenerateEfficiencyReport(t *testing.T) {
	m := NewManager(nil)

	// Add some readings
	for i := 0; i < 10; i++ {
		m.RecordReading(&EnergyReading{
			TotalWatts: float64(50 + i*5),
		})
	}

	report := m.GenerateEfficiencyReport("daily")
	if report == nil {
		t.Fatal("GenerateEfficiencyReport returned nil")
	}
	if report.Period != "daily" {
		t.Errorf("expected period 'daily', got %s", report.Period)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated at not set")
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations")
	}
}

func TestGetGreenScore(t *testing.T) {
	m := NewManager(nil)

	score := m.GetGreenScore()
	if score == nil {
		t.Fatal("GetGreenScore returned nil")
	}
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("expected score between 0-100, got %f", score.Score)
	}
	if score.Grade == "" {
		t.Error("grade not set")
	}
	if score.Breakdown == nil {
		t.Error("breakdown not set")
	}
}

func TestGreenScoreWithActiveStrategy(t *testing.T) {
	m := NewManager(nil)

	s := m.CreateStrategy("Test", "", 10*time.Minute)
	m.ActivateStrategy(s.ID)

	score := m.GetGreenScore()
	if score.Score < 50 {
		t.Errorf("expected higher score with active strategy, got %f", score.Score)
	}
}

func TestRecommendations(t *testing.T) {
	m := NewManager(nil)

	// Add high power readings to trigger recommendations
	for i := 0; i < 5; i++ {
		m.RecordReading(&EnergyReading{
			TotalWatts: 100,
		})
	}

	report := m.GenerateEfficiencyReport("daily")
	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations for high power usage")
	}
}

func TestTrends(t *testing.T) {
	m := NewManager(nil)

	// Add readings to generate trends
	for i := 0; i < 20; i++ {
		m.RecordReading(&EnergyReading{
			TotalWatts: float64(50 + i),
		})
	}

	report := m.GenerateEfficiencyReport("daily")
	if report.Trends == nil {
		t.Error("trends not generated")
	}
}
