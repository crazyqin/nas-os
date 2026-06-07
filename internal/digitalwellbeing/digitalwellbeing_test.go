package digitalwellbeing

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:         true,
		TrackingEnabled: true,
		AlertsEnabled:   true,
		BreakReminder:   60,
		BreakDuration:   5,
		DataRetention:   90,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	manager.Stop()
}

func TestRecordUsage(t *testing.T) {
	config := &Config{
		Enabled:         true,
		TrackingEnabled: true,
	}

	manager := NewManager(config)

	record := &UsageRecord{
		UserID:    "user1",
		AppName:   "Chrome",
		Category:  CategoryProductivity,
		Duration:  30 * time.Minute,
		StartTime: time.Now().Add(-30 * time.Minute),
		EndTime:   time.Now(),
		Device:    "desktop",
	}

	if err := manager.RecordUsage(record); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	if record.ID == "" {
		t.Error("Record ID not generated")
	}
}

func TestSetGoal(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	goal := &UsageGoal{
		UserID:   "user1",
		Category: CategorySocial,
		DailyMax: 2 * time.Hour,
		Enabled:  true,
	}

	if err := manager.SetGoal(goal); err != nil {
		t.Fatalf("SetGoal failed: %v", err)
	}

	if goal.ID == "" {
		t.Error("Goal ID not generated")
	}
}

func TestGetDailyUsage(t *testing.T) {
	config := &Config{
		Enabled:         true,
		TrackingEnabled: true,
	}

	manager := NewManager(config)

	// Record some usage
	now := time.Now()
	manager.RecordUsage(&UsageRecord{
		UserID:    "user1",
		AppName:   "Chrome",
		Category:  CategoryProductivity,
		Duration:  30 * time.Minute,
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(-30 * time.Minute),
	})

	manager.RecordUsage(&UsageRecord{
		UserID:    "user1",
		AppName:   "Instagram",
		Category:  CategorySocial,
		Duration:  15 * time.Minute,
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   now,
	})

	usage := manager.GetDailyUsage("user1", now)

	if usage.UserID != "user1" {
		t.Errorf("Expected user1, got %s", usage.UserID)
	}

	if usage.TotalTime != 45*time.Minute {
		t.Errorf("Expected 45 minutes, got %v", usage.TotalTime)
	}

	if usage.ByCategory[CategoryProductivity] != 30*time.Minute {
		t.Errorf("Expected 30 minutes productivity, got %v", usage.ByCategory[CategoryProductivity])
	}
}

func TestEnableFocusMode(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	mode := &FocusMode{
		UserID:      "user1",
		Name:        "Work Focus",
		BlockedApps: []string{"Instagram", "TikTok"},
		AllowedApps: []string{"Slack", "VSCode"},
		DNDMode:     true,
	}

	if err := manager.EnableFocusMode(mode); err != nil {
		t.Fatalf("EnableFocusMode failed: %v", err)
	}

	if mode.ID == "" {
		t.Error("Focus mode ID not generated")
	}
}

func TestSetParentalControl(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	control := &ParentalControl{
		UserID:          "child1",
		DailyLimit:      2 * time.Hour,
		BedtimeStart:    "21:00",
		BedtimeEnd:      "07:00",
		BlockedContent:  []string{"violence", "gambling"},
		RequireApproval: true,
	}

	if err := manager.SetParentalControl(control); err != nil {
		t.Fatalf("SetParentalControl failed: %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	config := &Config{
		Enabled:         true,
		TrackingEnabled: true,
	}

	manager := NewManager(config)

	// Record some usage
	now := time.Now()
	for i := 0; i < 7; i++ {
		manager.RecordUsage(&UsageRecord{
			UserID:    "user1",
			AppName:   "Chrome",
			Category:  CategoryProductivity,
			Duration:  2 * time.Hour,
			StartTime: now.AddDate(0, 0, -i),
			EndTime:   now.AddDate(0, 0, -i).Add(2 * time.Hour),
		})
	}

	report := manager.GenerateReport("user1", "weekly")

	if report.UserID != "user1" {
		t.Errorf("Expected user1, got %s", report.UserID)
	}

	if report.Period != "weekly" {
		t.Errorf("Expected weekly, got %s", report.Period)
	}

	if report.TotalTime == 0 {
		t.Error("Expected non-zero total time")
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		Enabled:         true,
		TrackingEnabled: true,
	}

	manager := NewManager(config)

	// Add some data
	manager.RecordUsage(&UsageRecord{
		UserID:   "user1",
		AppName:  "Chrome",
		Category: CategoryProductivity,
		Duration: 30 * time.Minute,
	})

	manager.SetGoal(&UsageGoal{
		UserID:   "user1",
		DailyMax: 2 * time.Hour,
		Enabled:  true,
	})

	stats := manager.GetStats()

	if stats["total_records"] != 1 {
		t.Errorf("Expected 1 record, got %v", stats["total_records"])
	}

	if stats["total_goals"] != 1 {
		t.Errorf("Expected 1 goal, got %v", stats["total_goals"])
	}
}

func TestAppCategories(t *testing.T) {
	categories := []AppCategory{
		CategoryProductivity, CategoryEntertainment, CategorySocial,
		CategoryGaming, CategoryEducation, CategoryHealth, CategoryOther,
	}

	for _, cat := range categories {
		if string(cat) == "" {
			t.Errorf("Empty category: %v", cat)
		}
	}
}

func TestAlertTypes(t *testing.T) {
	alertTypes := []AlertType{
		AlertGoalExceeded, AlertBedtime, AlertBreakReminder, AlertWeeklyReport,
	}

	for _, at := range alertTypes {
		if string(at) == "" {
			t.Errorf("Empty alert type: %v", at)
		}
	}
}
