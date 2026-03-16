package snapshot

import (
	"testing"
)

func TestNewStorageAdapter(t *testing.T) {
	// Test with nil manager (should still create adapter)
	adapter := NewStorageAdapter(nil)
	if adapter == nil {
		t.Fatal("NewStorageAdapter should not return nil")
	}
}

func TestSnapshotExecutor_NewSnapshotExecutor(t *testing.T) {
	executor := NewSnapshotExecutor(nil)
	if executor == nil {
		t.Fatal("NewSnapshotExecutor should not return nil")
	}
}

func TestSnapshotExecutor_GenerateSnapshotName(t *testing.T) {
	executor := NewSnapshotExecutor(nil)

	tests := []struct {
		name     string
		policy   *Policy
		checkLen bool
	}{
		{
			name: "manual snapshot with prefix",
			policy: &Policy{
				SnapshotPrefix: "daily",
				Type:           PolicyTypeManual,
			},
			checkLen: true,
		},
		{
			name: "scheduled snapshot",
			policy: &Policy{
				SnapshotPrefix: "auto",
				Type:           PolicyTypeScheduled,
				ID:             "policy-12345678",
			},
			checkLen: true,
		},
		{
			name: "no prefix",
			policy: &Policy{
				Type: PolicyTypeManual,
			},
			checkLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := executor.generateSnapshotName(tt.policy)
			if name == "" {
				t.Error("generateSnapshotName should not return empty string")
			}
			if tt.policy.SnapshotPrefix != "" && len(name) > 0 {
				// Should start with prefix
				prefix := tt.policy.SnapshotPrefix + "-"
				if len(name) < len(prefix) || name[:len(prefix)] != prefix {
					t.Errorf("snapshot name should start with prefix: %s", name)
				}
			}
		})
	}
}

func TestScheduleConfig_Defaults(t *testing.T) {
	// Test that schedule config constants exist
	_ = ScheduleTypeHourly
	_ = ScheduleTypeDaily
	_ = ScheduleTypeWeekly
	_ = ScheduleTypeMonthly
	_ = ScheduleTypeCustom
}

func TestPolicyType_Constants(t *testing.T) {
	// Test that policy type constants exist
	_ = PolicyTypeManual
	_ = PolicyTypeScheduled
}

func TestService_NewService(t *testing.T) {
	// Test with empty config path
	svc := NewService("", nil)
	if svc == nil {
		t.Fatal("NewService should not return nil")
	}
	if svc.PolicyManager == nil {
		t.Error("PolicyManager should not be nil")
	}
	if svc.Handlers == nil {
		t.Error("Handlers should not be nil")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	if DefaultConfigPath == "" {
		t.Error("DefaultConfigPath should not be empty")
	}
}
