package energymanager

import (
	"testing"
	"time"
)

func TestNewManagerViaHandler(t *testing.T) {
	manager := NewManager(nil)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config.ElectricityRate != 0.12 {
		t.Errorf("Expected electricity rate 0.12, got %f", manager.config.ElectricityRate)
	}
}

func TestCreateProfile(t *testing.T) {
	manager := NewManager(nil)

	settings := PowerSettings{
		HDDSpindown: 30,
		LEDControl:  true,
		FanMode:     "auto",
		WakeOnLAN:   true,
	}

	profile := manager.CreateProfile("Eco Mode", "Energy saving profile", "eco", settings)

	if profile == nil {
		t.Fatal("Expected profile to be created")
	}

	if profile.Name != "Eco Mode" {
		t.Errorf("Expected name 'Eco Mode', got %s", profile.Name)
	}

	if profile.Type != "eco" {
		t.Errorf("Expected type 'eco', got %s", profile.Type)
	}

	if profile.Settings.HDDSpindown != 30 {
		t.Errorf("Expected HDD spindown 30, got %d", profile.Settings.HDDSpindown)
	}
}

func TestGetProfile(t *testing.T) {
	manager := NewManager(nil)

	created := manager.CreateProfile("Test", "Desc", "custom", PowerSettings{})

	found, ok := manager.GetProfile(created.ID)
	if !ok {
		t.Fatal("Expected to find profile")
	}

	if found.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestListProfiles(t *testing.T) {
	manager := NewManager(nil)

	manager.CreateProfile("Profile 1", "", "eco", PowerSettings{})
	manager.CreateProfile("Profile 2", "", "performance", PowerSettings{})

	profiles := manager.ListProfiles()

	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}
}

func TestSetActiveProfile(t *testing.T) {
	manager := NewManager(nil)

	profile1 := manager.CreateProfile("Profile 1", "", "eco", PowerSettings{})
	profile2 := manager.CreateProfile("Profile 2", "", "performance", PowerSettings{})

	err := manager.SetActiveProfile(profile1.ID)
	if err != nil {
		t.Fatalf("Failed to set active profile: %v", err)
	}

	found1, _ := manager.GetProfile(profile1.ID)
	found2, _ := manager.GetProfile(profile2.ID)

	if !found1.IsActive {
		t.Error("Expected profile 1 to be active")
	}

	if found2.IsActive {
		t.Error("Expected profile 2 to not be active")
	}
}

func TestGetPowerHistory(t *testing.T) {
	manager := NewManager(nil)

	// Add some test data
	manager.history = []PowerUsage{
		{Timestamp: time.Now(), TotalWatts: 50},
		{Timestamp: time.Now().Add(-time.Hour), TotalWatts: 60},
		{Timestamp: time.Now().Add(-2 * time.Hour), TotalWatts: 45},
	}

	history := manager.GetPowerHistory("day")

	if history.Period != "day" {
		t.Errorf("Expected period 'day', got %s", history.Period)
	}

	if len(history.DataPoints) != 3 {
		t.Errorf("Expected 3 data points, got %d", len(history.DataPoints))
	}

	if history.AvgWatts < 51.66 || history.AvgWatts > 51.67 {
		t.Errorf("Expected avg watts ~51.67, got %f", history.AvgWatts)
	}
}

func TestGetEnergyStats(t *testing.T) {
	manager := NewManager(nil)

	manager.currentUsage = &PowerUsage{
		Timestamp:  time.Now(),
		TotalWatts: 75.5,
	}

	stats := manager.GetEnergyStats()

	if stats.CurrentWatts != 75.5 {
		t.Errorf("Expected current watts 75.5, got %f", stats.CurrentWatts)
	}

	if stats.CostPerKWh != 0.12 {
		t.Errorf("Expected cost per kWh 0.12, got %f", stats.CostPerKWh)
	}
}

func TestCreateSchedule(t *testing.T) {
	manager := NewManager(nil)

	schedule := manager.CreateSchedule("Night Power Off", "power_off", "23:00", []string{"Mon", "Tue", "Wed", "Thu", "Fri"})

	if schedule == nil {
		t.Fatal("Expected schedule to be created")
	}

	if schedule.Name != "Night Power Off" {
		t.Errorf("Expected name 'Night Power Off', got %s", schedule.Name)
	}

	if schedule.Type != "power_off" {
		t.Errorf("Expected type 'power_off', got %s", schedule.Type)
	}

	if len(schedule.Days) != 5 {
		t.Errorf("Expected 5 days, got %d", len(schedule.Days))
	}
}

func TestListSchedules(t *testing.T) {
	manager := NewManager(nil)

	manager.CreateSchedule("Schedule 1", "power_on", "08:00", []string{"Mon"})
	manager.CreateSchedule("Schedule 2", "power_off", "22:00", []string{"Tue"})

	schedules := manager.ListSchedules()

	if len(schedules) != 2 {
		t.Errorf("Expected 2 schedules, got %d", len(schedules))
	}
}

func TestDeleteSchedule(t *testing.T) {
	manager := NewManager(nil)

	created := manager.CreateSchedule("Test", "power_on", "10:00", []string{"Wed"})

	err := manager.DeleteSchedule(created.ID)
	if err != nil {
		t.Fatalf("Failed to delete schedule: %v", err)
	}

	schedules := manager.ListSchedules()
	if len(schedules) != 0 {
		t.Errorf("Expected 0 schedules, got %d", len(schedules))
	}
}

func TestGetAlerts(t *testing.T) {
	manager := NewManager(nil)

	manager.alerts = []*TemperatureAlert{
		{ID: "alert1", Component: "CPU", Temperature: 85, Threshold: 80, Severity: "warning"},
		{ID: "alert2", Component: "Disk", Temperature: 65, Threshold: 70, Severity: "normal"},
	}

	alerts := manager.GetAlerts()

	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	manager := NewManager(nil)

	alert := &TemperatureAlert{
		ID:          "alert1",
		Component:   "CPU",
		Temperature: 85,
		Threshold:   80,
		Severity:    "warning",
	}
	manager.alerts = append(manager.alerts, alert)

	err := manager.AcknowledgeAlert("alert1")
	if err != nil {
		t.Fatalf("Failed to acknowledge alert: %v", err)
	}

	if !manager.alerts[0].Acknowledged {
		t.Error("Expected alert to be acknowledged")
	}
}
