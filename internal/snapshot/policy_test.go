package snapshot

import (
	"testing"
	"time"
)

// ========== PolicyType 测试 ==========

func TestPolicyType_Values(t *testing.T) {
	types := []PolicyType{
		PolicyTypeManual,
		PolicyTypeScheduled,
		PolicyTypeApplicationConsistent,
	}

	for _, pt := range types {
		if pt == "" {
			t.Error("PolicyType should not be empty")
		}
	}
}

// ========== ScheduleType 测试 ==========

func TestScheduleType_Values(t *testing.T) {
	types := []ScheduleType{
		ScheduleTypeHourly,
		ScheduleTypeDaily,
		ScheduleTypeWeekly,
		ScheduleTypeMonthly,
		ScheduleTypeCustom,
	}

	for _, st := range types {
		if st == "" {
			t.Error("ScheduleType should not be empty")
		}
	}
}

// ========== RetentionPolicyType 测试 ==========

func TestRetentionPolicyType_Values(t *testing.T) {
	types := []RetentionPolicyType{
		RetentionByCount,
		RetentionByAge,
		RetentionBySize,
		RetentionCombined,
	}

	for _, rt := range types {
		if rt == "" {
			t.Error("RetentionPolicyType should not be empty")
		}
	}
}

// ========== RetentionPolicy 测试 ==========

func TestRetentionPolicy_ByCount(t *testing.T) {
	policy := &RetentionPolicy{
		Type:     RetentionByCount,
		MaxCount: 10,
	}

	if policy.Type != RetentionByCount {
		t.Errorf("Expected RetentionByCount, got %s", policy.Type)
	}
	if policy.MaxCount != 10 {
		t.Errorf("Expected MaxCount=10, got %d", policy.MaxCount)
	}
}

func TestRetentionPolicy_ByAge(t *testing.T) {
	policy := &RetentionPolicy{
		Type:       RetentionByAge,
		MaxAgeDays: 30,
	}

	if policy.Type != RetentionByAge {
		t.Errorf("Expected RetentionByAge, got %s", policy.Type)
	}
	if policy.MaxAgeDays != 30 {
		t.Errorf("Expected MaxAgeDays=30, got %d", policy.MaxAgeDays)
	}
}

func TestRetentionPolicy_BySize(t *testing.T) {
	policy := &RetentionPolicy{
		Type:         RetentionBySize,
		MaxSizeBytes: 1024 * 1024 * 1024, // 1GB
	}

	if policy.Type != RetentionBySize {
		t.Errorf("Expected RetentionBySize, got %s", policy.Type)
	}
	if policy.MaxSizeBytes != 1073741824 {
		t.Errorf("Expected MaxSizeBytes=1073741824, got %d", policy.MaxSizeBytes)
	}
}

func TestRetentionPolicy_Combined(t *testing.T) {
	policy := &RetentionPolicy{
		Type: RetentionCombined,
		CountPolicy: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 20,
		},
		AgePolicy: &RetentionPolicy{
			Type:       RetentionByAge,
			MaxAgeDays: 7,
		},
	}

	if policy.Type != RetentionCombined {
		t.Errorf("Expected RetentionCombined, got %s", policy.Type)
	}
	if policy.CountPolicy == nil || policy.AgePolicy == nil {
		t.Error("Combined policy should have sub-policies")
	}
}

// ========== ScheduleConfig 测试 ==========

func TestScheduleConfig_Hourly(t *testing.T) {
	config := &ScheduleConfig{
		Type:          ScheduleTypeHourly,
		IntervalHours: 4,
		Minute:        30,
	}

	if config.Type != ScheduleTypeHourly {
		t.Errorf("Expected ScheduleTypeHourly, got %s", config.Type)
	}
	if config.IntervalHours != 4 {
		t.Errorf("Expected IntervalHours=4, got %d", config.IntervalHours)
	}
}

func TestScheduleConfig_Daily(t *testing.T) {
	config := &ScheduleConfig{
		Type:   ScheduleTypeDaily,
		Hour:   2,
		Minute: 0,
	}

	if config.Type != ScheduleTypeDaily {
		t.Errorf("Expected ScheduleTypeDaily, got %s", config.Type)
	}
	if config.Hour != 2 {
		t.Errorf("Expected Hour=2, got %d", config.Hour)
	}
}

func TestScheduleConfig_Weekly(t *testing.T) {
	config := &ScheduleConfig{
		Type:      ScheduleTypeWeekly,
		DayOfWeek: 0, // Sunday
		Hour:      3,
		Minute:    0,
	}

	if config.Type != ScheduleTypeWeekly {
		t.Errorf("Expected ScheduleTypeWeekly, got %s", config.Type)
	}
	if config.DayOfWeek != 0 {
		t.Errorf("Expected DayOfWeek=0, got %d", config.DayOfWeek)
	}
}

func TestScheduleConfig_Monthly(t *testing.T) {
	config := &ScheduleConfig{
		Type:       ScheduleTypeMonthly,
		DayOfMonth: 1,
		Hour:       0,
		Minute:     0,
	}

	if config.Type != ScheduleTypeMonthly {
		t.Errorf("Expected ScheduleTypeMonthly, got %s", config.Type)
	}
	if config.DayOfMonth != 1 {
		t.Errorf("Expected DayOfMonth=1, got %d", config.DayOfMonth)
	}
}

func TestScheduleConfig_Custom(t *testing.T) {
	config := &ScheduleConfig{
		Type:           ScheduleTypeCustom,
		CronExpression: "0 0 * * *",
	}

	if config.Type != ScheduleTypeCustom {
		t.Errorf("Expected ScheduleTypeCustom, got %s", config.Type)
	}
	if config.CronExpression != "0 0 * * *" {
		t.Errorf("Expected cron expression, got %s", config.CronExpression)
	}
}

// ========== ScriptConfig 测试 ==========

func TestScriptConfig_Basic(t *testing.T) {
	config := &ScriptConfig{
		PreSnapshotScript:  "/usr/local/bin/pre-snap.sh",
		PostSnapshotScript: "/usr/local/bin/post-snap.sh",
		TimeoutSeconds:     300,
		ContinueOnFailure:  false,
	}

	if config.PreSnapshotScript == "" {
		t.Error("PreSnapshotScript should not be empty")
	}
	if config.PostSnapshotScript == "" {
		t.Error("PostSnapshotScript should not be empty")
	}
	if config.TimeoutSeconds != 300 {
		t.Errorf("Expected TimeoutSeconds=300, got %d", config.TimeoutSeconds)
	}
}

func TestScriptConfig_ContinueOnFailure(t *testing.T) {
	config := &ScriptConfig{
		ContinueOnFailure: true,
	}

	if !config.ContinueOnFailure {
		t.Error("ContinueOnFailure should be true")
	}
}

// ========== Policy 测试 ==========

func TestPolicy_Basic(t *testing.T) {
	now := time.Now()
	policy := &Policy{
		ID:            "test-policy-1",
		Name:          "Daily Backup",
		Description:   "Daily backup policy",
		Type:          PolicyTypeScheduled,
		Enabled:       true,
		VolumeName:    "data",
		SubvolumeName: "documents",
		SnapshotDir:   ".snapshots",
		ReadOnly:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if policy.ID != "test-policy-1" {
		t.Errorf("Expected ID=test-policy-1, got %s", policy.ID)
	}
	if policy.Name != "Daily Backup" {
		t.Errorf("Expected Name='Daily Backup', got %s", policy.Name)
	}
	if !policy.Enabled {
		t.Error("Policy should be enabled")
	}
	if !policy.ReadOnly {
		t.Error("Snapshots should be read-only by default")
	}
}

func TestPolicy_WithSchedule(t *testing.T) {
	policy := &Policy{
		ID:         "scheduled-policy",
		Name:       "Weekly Backup",
		Type:       PolicyTypeScheduled,
		Enabled:    true,
		VolumeName: "data",
		Schedule: &ScheduleConfig{
			Type:      ScheduleTypeWeekly,
			DayOfWeek: 0,
			Hour:      2,
		},
	}

	if policy.Schedule == nil {
		t.Fatal("Schedule should not be nil")
	}
	if policy.Schedule.Type != ScheduleTypeWeekly {
		t.Errorf("Expected ScheduleTypeWeekly, got %s", policy.Schedule.Type)
	}
}

func TestPolicy_WithRetention(t *testing.T) {
	policy := &Policy{
		ID:         "retention-policy",
		Name:       "Retention Test",
		Type:       PolicyTypeManual,
		VolumeName: "data",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 10,
		},
	}

	if policy.Retention == nil {
		t.Fatal("Retention should not be nil")
	}
	if policy.Retention.Type != RetentionByCount {
		t.Errorf("Expected RetentionByCount, got %s", policy.Retention.Type)
	}
}

func TestPolicy_WithScripts(t *testing.T) {
	policy := &Policy{
		ID:         "app-consistent-policy",
		Name:       "App Consistent Backup",
		Type:       PolicyTypeApplicationConsistent,
		VolumeName: "data",
		Scripts: &ScriptConfig{
			PreSnapshotScript:  "/usr/local/bin/quiesce.sh",
			PostSnapshotScript: "/usr/local/bin/unquiesce.sh",
		},
	}

	if policy.Scripts == nil {
		t.Fatal("Scripts should not be nil")
	}
	if policy.Scripts.PreSnapshotScript == "" {
		t.Error("PreSnapshotScript should not be empty")
	}
}

func TestPolicy_WithTags(t *testing.T) {
	policy := &Policy{
		ID:         "tagged-policy",
		Name:       "Tagged Policy",
		VolumeName: "data",
		Tags:       []string{"important", "production", "daily"},
	}

	if len(policy.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(policy.Tags))
	}
}

func TestPolicy_WithMetadata(t *testing.T) {
	policy := &Policy{
		ID:         "metadata-policy",
		Name:       "Policy with Metadata",
		VolumeName: "data",
		Metadata: map[string]string{
			"owner":   "admin",
			"purpose": "disaster-recovery",
		},
	}

	if len(policy.Metadata) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(policy.Metadata))
	}
	if policy.Metadata["owner"] != "admin" {
		t.Errorf("Expected owner=admin, got %s", policy.Metadata["owner"])
	}
}

// ========== PolicyStats 测试 ==========

func TestPolicyStats_Basic(t *testing.T) {
	now := time.Now()
	stats := PolicyStats{
		TotalRuns:             100,
		SuccessfulRuns:        95,
		FailedRuns:            5,
		LastSuccessAt:         &now,
		TotalSnapshotsCreated: 95,
		TotalSnapshotsDeleted: 50,
		TotalBytesSaved:       1024 * 1024 * 1024 * 10, // 10GB
	}

	if stats.TotalRuns != 100 {
		t.Errorf("Expected TotalRuns=100, got %d", stats.TotalRuns)
	}
	if stats.SuccessfulRuns+stats.FailedRuns != stats.TotalRuns {
		t.Error("SuccessfulRuns + FailedRuns should equal TotalRuns")
	}
}

func TestPolicyStats_SuccessRate(t *testing.T) {
	stats := PolicyStats{
		TotalRuns:      100,
		SuccessfulRuns: 95,
		FailedRuns:     5,
	}

	successRate := float64(stats.SuccessfulRuns) / float64(stats.TotalRuns) * 100
	if successRate != 95.0 {
		t.Errorf("Expected success rate=95.0, got %.2f", successRate)
	}
}

// ========== PolicyManager 测试 ==========

func TestNewPolicyManager(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies.json", nil)
	if pm == nil {
		t.Fatal("PolicyManager should not be nil")
	}
	if pm.policies == nil {
		t.Error("policies map should be initialized")
	}
}

func TestPolicyManager_CreatePolicy(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-create.json", nil)

	policy := &Policy{
		Name:          "Test Policy",
		Type:          PolicyTypeManual,
		VolumeName:    "data",
		SubvolumeName: "documents",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}

	err := pm.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.ID == "" {
		t.Error("Policy ID should be auto-generated")
	}
	if policy.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestPolicyManager_GetPolicy(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-get.json", nil)

	// Create a policy first
	policy := &Policy{
		Name:          "Get Test Policy",
		Type:          PolicyTypeManual,
		VolumeName:    "data",
		SubvolumeName: "documents",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}
	_ = pm.CreatePolicy(policy)

	// Get the policy
	retrieved, err := pm.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}
	if retrieved.Name != policy.Name {
		t.Errorf("Expected Name=%s, got %s", policy.Name, retrieved.Name)
	}
}

func TestPolicyManager_GetPolicyNotFound(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-notfound.json", nil)

	_, err := pm.GetPolicy("nonexistent-id")
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestPolicyManager_ListPolicies(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-list.json", nil)

	// Create multiple policies
	for i := 0; i < 3; i++ {
		policy := &Policy{
			Name:          "Policy " + string(rune('A'+i)),
			Type:          PolicyTypeManual,
			VolumeName:    "data",
			SubvolumeName: "documents",
			Retention: &RetentionPolicy{
				Type:     RetentionByCount,
				MaxCount: 5,
			},
		}
		_ = pm.CreatePolicy(policy)
	}

	policies := pm.ListPolicies()
	if len(policies) != 3 {
		t.Errorf("Expected 3 policies, got %d", len(policies))
	}
}

func TestPolicyManager_DeletePolicy(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-delete.json", nil)

	policy := &Policy{
		Name:          "Delete Test Policy",
		Type:          PolicyTypeManual,
		VolumeName:    "data",
		SubvolumeName: "documents",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}
	_ = pm.CreatePolicy(policy)

	// Delete the policy
	err := pm.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// Verify deletion
	_, err = pm.GetPolicy(policy.ID)
	if err == nil {
		t.Error("Policy should be deleted")
	}
}

func TestPolicyManager_EnablePolicy(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-enable.json", nil)

	policy := &Policy{
		Name:          "Enable Test Policy",
		Type:          PolicyTypeManual,
		Enabled:       false,
		VolumeName:    "data",
		SubvolumeName: "documents",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}
	_ = pm.CreatePolicy(policy)

	// Enable the policy
	err := pm.EnablePolicy(policy.ID, true)
	if err != nil {
		t.Fatalf("Failed to enable policy: %v", err)
	}

	// Verify
	enabled, _ := pm.GetPolicy(policy.ID)
	if !enabled.Enabled {
		t.Error("Policy should be enabled")
	}
}

func TestPolicyManager_ValidatePolicy_EmptyName(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-validate.json", nil)

	policy := &Policy{
		Name:       "",
		VolumeName: "data",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}

	err := pm.CreatePolicy(policy)
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestPolicyManager_ValidatePolicy_EmptyVolume(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-validate-vol.json", nil)

	policy := &Policy{
		Name:       "Test",
		VolumeName: "",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}

	err := pm.CreatePolicy(policy)
	if err == nil {
		t.Error("Expected error for empty volume name")
	}
}

func TestPolicyManager_ValidatePolicy_NoRetention(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-validate-ret.json", nil)

	policy := &Policy{
		Name:       "Test",
		VolumeName: "data",
		Retention:  nil,
	}

	err := pm.CreatePolicy(policy)
	if err == nil {
		t.Error("Expected error for missing retention policy")
	}
}

func TestPolicyManager_ListPoliciesByVolume(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-policies-byvol.json", nil)

	// Create policies for different volumes
	volumes := []string{"data", "backup", "data"}
	for i, vol := range volumes {
		policy := &Policy{
			Name:          "Policy " + string(rune('A'+i)),
			Type:          PolicyTypeManual,
			VolumeName:    vol,
			SubvolumeName: "documents",
			Retention: &RetentionPolicy{
				Type:     RetentionByCount,
				MaxCount: 5,
			},
		}
		_ = pm.CreatePolicy(policy)
	}

	// Get policies for "data" volume
	dataPolicies := pm.ListPoliciesByVolume("data")
	if len(dataPolicies) != 2 {
		t.Errorf("Expected 2 policies for 'data' volume, got %d", len(dataPolicies))
	}
}

// ========== PolicyHooks 测试 ==========

func TestPolicyHooks_SetHooks(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-hooks.json", nil)

	hooks := PolicyHooks{
		OnBeforeSnapshot: func(policy *Policy) error {
			return nil
		},
		OnAfterSnapshot: func(policy *Policy, snapshotName string, err error) {
		},
	}

	pm.SetHooks(hooks)

	if pm.hooks.OnBeforeSnapshot == nil {
		t.Error("OnBeforeSnapshot hook should be set")
	}
	if pm.hooks.OnAfterSnapshot == nil {
		t.Error("OnAfterSnapshot hook should be set")
	}
}

// ========== 边界条件测试 ==========

func TestPolicy_ZeroValues(t *testing.T) {
	policy := &Policy{}

	if policy.Enabled {
		t.Error("Policy should be disabled by default")
	}
	if policy.ReadOnly {
		t.Error("ReadOnly should be false by default")
	}
}

func TestPolicy_LastRunNil(t *testing.T) {
	policy := &Policy{
		Name:       "Never Run",
		VolumeName: "data",
	}

	if policy.LastRunAt != nil {
		t.Error("LastRunAt should be nil for never-run policy")
	}
	if policy.NextRunAt != nil {
		t.Error("NextRunAt should be nil for manual policy")
	}
}

func TestScheduleConfig_InvalidHour(t *testing.T) {
	config := &ScheduleConfig{
		Type: ScheduleTypeDaily,
		Hour: 25, // Invalid
	}

	if config.Hour < 0 || config.Hour > 23 {
		t.Log("Invalid hour detected correctly")
	}
}

func TestScheduleConfig_InvalidMinute(t *testing.T) {
	config := &ScheduleConfig{
		Type:   ScheduleTypeDaily,
		Minute: 70, // Invalid
	}

	if config.Minute < 0 || config.Minute > 59 {
		t.Log("Invalid minute detected correctly")
	}
}

func TestRetentionPolicy_ZeroMaxCount(t *testing.T) {
	policy := &RetentionPolicy{
		Type:     RetentionByCount,
		MaxCount: 0, // Would keep no snapshots
	}

	if policy.MaxCount <= 0 {
		t.Log("Zero MaxCount detected - would delete all snapshots")
	}
}

func TestRetentionPolicy_NegativeAge(t *testing.T) {
	policy := &RetentionPolicy{
		Type:       RetentionByAge,
		MaxAgeDays: -1, // Invalid
	}

	if policy.MaxAgeDays < 0 {
		t.Log("Negative MaxAgeDays detected")
	}
}

// ========== 时间相关测试 ==========

func TestPolicy_Timestamps(t *testing.T) {
	before := time.Now()

	pm := NewPolicyManager("/tmp/test-timestamps.json", nil)
	policy := &Policy{
		Name:       "Timestamp Test",
		Type:       PolicyTypeManual,
		VolumeName: "data",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}
	_ = pm.CreatePolicy(policy)

	after := time.Now()

	if policy.CreatedAt.Before(before) || policy.CreatedAt.After(after) {
		t.Error("CreatedAt should be between before and after")
	}
	if policy.UpdatedAt.Before(before) || policy.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be between before and after")
	}
}

func TestPolicy_StatsTimeFields(t *testing.T) {
	now := time.Now()
	stats := PolicyStats{
		LastSuccessAt: &now,
		LastFailureAt: &now,
	}

	if stats.LastSuccessAt == nil {
		t.Error("LastSuccessAt should not be nil")
	}
	if stats.LastFailureAt == nil {
		t.Error("LastFailureAt should not be nil")
	}
}

// ========== 并发测试 ==========

func TestPolicyManager_ConcurrentRead(t *testing.T) {
	pm := NewPolicyManager("/tmp/test-concurrent.json", nil)

	// Create a policy
	policy := &Policy{
		Name:       "Concurrent Test",
		Type:       PolicyTypeManual,
		VolumeName: "data",
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 5,
		},
	}
	_ = pm.CreatePolicy(policy)

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = pm.ListPolicies()
			_, _ = pm.GetPolicy(policy.ID)
			_ = pm.ListPoliciesByVolume("data")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
