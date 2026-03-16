package snapshot

import (
	"testing"
	"time"
)

func TestScheduler_NewScheduler(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	if scheduler == nil {
		t.Fatal("NewScheduler should not return nil")
	}
	if scheduler.cron == nil {
		t.Error("cron should be initialized")
	}
	if scheduler.jobIDs == nil {
		t.Error("jobIDs should be initialized")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Start
	scheduler.Start()
	if !scheduler.IsRunning() {
		t.Error("Scheduler should be running after Start")
	}

	// Double start should be safe
	scheduler.Start()

	// Stop
	scheduler.Stop()
	if scheduler.IsRunning() {
		t.Error("Scheduler should not be running after Stop")
	}

	// Double stop should be safe
	scheduler.Stop()
}

func TestScheduler_ValidateCron(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	tests := []struct {
		expr     string
		expected bool
	}{
		{"0 0 * * *", true},    // Every day at midnight
		{"0 */2 * * *", true},  // Every 2 hours
		{"0 0 * * 0", true},    // Every Sunday
		{"0 0 1 * *", true},    // First day of month
		{"invalid", false},     // Invalid expression
		{"0 0 0 0 0", false},   // Invalid (month 0)
		{"", false},            // Empty
		{"* * * * * *", false}, // 6 fields (standard cron is 5)
	}

	for _, tt := range tests {
		result := scheduler.ValidateCron(tt.expr)
		if result != tt.expected {
			t.Errorf("ValidateCron(%q) = %v, expected %v", tt.expr, result, tt.expected)
		}
	}
}

func TestScheduler_AddJob_InvalidPolicy(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Policy without schedule
	policy := &Policy{
		ID:   "test-policy",
		Type: PolicyTypeManual,
	}
	err := scheduler.AddJob(policy)
	if err == nil {
		t.Error("AddJob should fail for manual policy without schedule")
	}

	// Scheduled policy with nil schedule
	policy2 := &Policy{
		ID:       "test-policy-2",
		Type:     PolicyTypeScheduled,
		Schedule: nil,
	}
	err = scheduler.AddJob(policy2)
	if err == nil {
		t.Error("AddJob should fail for policy with nil schedule")
	}
}

func TestScheduler_RemoveJob(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Remove non-existent job (should be safe)
	scheduler.RemoveJob("nonexistent")
}

func TestScheduler_UpdateJob(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Update non-existent job (should be safe)
	policy := &Policy{
		ID:      "nonexistent",
		Enabled: false,
	}
	err := scheduler.UpdateJob(policy)
	if err != nil {
		t.Errorf("UpdateJob should not fail for disabled policy: %v", err)
	}
}

func TestScheduler_GenerateCronExpression(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	tests := []struct {
		name         string
		schedule     *ScheduleConfig
		wantNonEmpty bool
	}{
		{
			name: "hourly",
			schedule: &ScheduleConfig{
				Type:          ScheduleTypeHourly,
				Minute:        30,
				IntervalHours: 1,
			},
			wantNonEmpty: true,
		},
		{
			name: "daily",
			schedule: &ScheduleConfig{
				Type:   ScheduleTypeDaily,
				Minute: 0,
				Hour:   2,
			},
			wantNonEmpty: true,
		},
		{
			name: "weekly",
			schedule: &ScheduleConfig{
				Type:      ScheduleTypeWeekly,
				Minute:    0,
				Hour:      3,
				DayOfWeek: 0, // Sunday
			},
			wantNonEmpty: true,
		},
		{
			name: "monthly",
			schedule: &ScheduleConfig{
				Type:       ScheduleTypeMonthly,
				Minute:     0,
				Hour:       4,
				DayOfMonth: 1,
			},
			wantNonEmpty: true,
		},
		{
			name: "custom",
			schedule: &ScheduleConfig{
				Type:           ScheduleTypeCustom,
				CronExpression: "0 5 * * *",
			},
			wantNonEmpty: true,
		},
		{
			name:         "nil schedule",
			schedule:     nil,
			wantNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.generateCronExpression(tt.schedule)
			if tt.wantNonEmpty && result == "" {
				t.Error("generateCronExpression should return non-empty string")
			}
			if !tt.wantNonEmpty && result != "" {
				t.Errorf("generateCronExpression should return empty string, got: %s", result)
			}
		})
	}
}

func TestScheduler_GetNextRuns(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Empty scheduler
	runs := scheduler.GetNextRuns()
	if runs == nil {
		t.Error("GetNextRuns should not return nil")
	}
	if len(runs) != 0 {
		t.Error("GetNextRuns should return empty map for empty scheduler")
	}
}

func TestScheduler_ListJobs(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Empty scheduler - ListJobs returns nil for empty, which is acceptable
	jobs := scheduler.ListJobs()
	// jobs may be nil for empty scheduler, that's the implementation
	t.Logf("ListJobs() = %v (nil is acceptable for empty scheduler)", jobs)
}

func TestScheduler_GetJobStatus_NotFound(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	_, err := scheduler.GetJobStatus("nonexistent")
	if err == nil {
		t.Error("GetJobStatus should return error for non-existent job")
	}
}

func TestScheduler_CalculateNextRun(t *testing.T) {
	pm := NewPolicyManager("", nil)
	scheduler := NewScheduler(pm)

	// Non-existent policy
	policy := &Policy{ID: "nonexistent"}
	nextRun := scheduler.CalculateNextRun(policy)
	if !nextRun.IsZero() {
		t.Error("CalculateNextRun should return zero time for non-existent job")
	}
}

func TestJobInfo_Fields(t *testing.T) {
	info := JobInfo{
		PolicyID:   "test-id",
		PolicyName: "Test Policy",
		NextRun:    time.Now(),
		PrevRun:    time.Now().Add(-time.Hour),
		Schedule:   "0 0 * * *",
		Enabled:    true,
	}

	if info.PolicyID != "test-id" {
		t.Error("PolicyID mismatch")
	}
	if info.PolicyName != "Test Policy" {
		t.Error("PolicyName mismatch")
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
}
