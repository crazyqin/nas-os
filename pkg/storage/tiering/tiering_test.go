package tiering

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.policies) != 0 {
		t.Errorf("expected empty policies, got %d", len(m.policies))
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.Name != "default" {
		t.Errorf("expected name 'default', got %s", p.Name)
	}
	if !p.Enabled {
		t.Error("expected enabled policy")
	}
	if p.HotThreshold != 100 {
		t.Errorf("expected hot threshold 100, got %d", p.HotThreshold)
	}
	if p.ColdThreshold != 30 {
		t.Errorf("expected cold threshold 30, got %d", p.ColdThreshold)
	}
}

func TestAddPolicy(t *testing.T) {
	m := NewManager()
	p := &Policy{
		Name:          "test-policy",
		Enabled:       true,
		HotThreshold:  50,
		ColdThreshold: 60,
	}

	err := m.AddPolicy(p)
	if err != nil {
		t.Fatalf("failed to add policy: %v", err)
	}

	if len(m.policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(m.policies))
	}
}

func TestAddInvalidPolicy(t *testing.T) {
	m := NewManager()
	p := &Policy{
		Name: "", // empty name should fail
	}

	err := m.AddPolicy(p)
	if err != ErrInvalidPolicy {
		t.Errorf("expected ErrInvalidPolicy, got %v", err)
	}
}

func TestGetPolicy(t *testing.T) {
	m := NewManager()
	p := &Policy{Name: "test", Enabled: true}
	m.AddPolicy(p)

	retrieved, ok := m.GetPolicy("test")
	if !ok {
		t.Fatal("expected to find policy")
	}
	if retrieved.Name != "test" {
		t.Errorf("expected name 'test', got %s", retrieved.Name)
	}
}

func TestRemovePolicy(t *testing.T) {
	m := NewManager()
	p := &Policy{Name: "test", Enabled: true}
	m.AddPolicy(p)

	err := m.RemovePolicy("test")
	if err != nil {
		t.Fatalf("failed to remove policy: %v", err)
	}

	if len(m.policies) != 0 {
		t.Errorf("expected 0 policies, got %d", len(m.policies))
	}
}

func TestRemoveNonExistentPolicy(t *testing.T) {
	m := NewManager()
	err := m.RemovePolicy("nonexistent")
	if err != ErrPolicyNotFound {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected string
	}{
		{TierHot, "hot"},
		{TierWarm, "warm"},
		{TierCold, "cold"},
		{Tier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.expected {
			t.Errorf("Tier(%d).String() = %s, want %s", tt.tier, got, tt.expected)
		}
	}
}

func TestDetermineTier(t *testing.T) {
	m := NewManager()
	policy := DefaultPolicy()

	tests := []struct {
		name     string
		info     FileInfo
		expected Tier
	}{
		{
			name: "hot file with high access",
			info: FileInfo{
				AccessCount: 150,
				LastAccess:  time.Now(),
			},
			expected: TierHot,
		},
		{
			name: "cold file with old access",
			info: FileInfo{
				AccessCount: 5,
				LastAccess:  time.Now().Add(-40 * 24 * time.Hour),
			},
			expected: TierCold,
		},
		{
			name: "warm file",
			info: FileInfo{
				AccessCount: 50,
				LastAccess:  time.Now().Add(-10 * 24 * time.Hour),
			},
			expected: TierWarm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.DetermineTier(tt.info, policy)
			if got != tt.expected {
				t.Errorf("DetermineTier() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMigrateFile(t *testing.T) {
	m := NewManager()
	info := FileInfo{
		Path:        "/test/file.txt",
		CurrentTier: TierWarm,
	}

	task, err := m.MigrateFile(context.Background(), info, TierCold)
	if err != nil {
		t.Fatalf("failed to create migration task: %v", err)
	}

	if task.SourcePath != info.Path {
		t.Errorf("expected source path %s, got %s", info.Path, task.SourcePath)
	}
	if task.TargetTier != TierCold {
		t.Errorf("expected target tier cold, got %s", task.TargetTier)
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager()
	info := FileInfo{Path: "/test/file.txt", CurrentTier: TierWarm}

	m.MigrateFile(context.Background(), info, TierCold)
	m.MigrateFile(context.Background(), info, TierHot)

	tasks := m.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	stats := m.GetStats()

	if stats.TotalFiles != 0 {
		t.Errorf("expected 0 total files, got %d", stats.TotalFiles)
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	if !m.running {
		t.Error("expected manager to be running")
	}

	err = m.Stop()
	if err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}

	if m.running {
		t.Error("expected manager to be stopped")
	}
}

func TestDoubleStart(t *testing.T) {
	m := NewManager()
	m.Start(context.Background())
	defer m.Stop()

	err := m.Start(context.Background())
	if err != ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}
}

func TestStopNotRunning(t *testing.T) {
	m := NewManager()
	err := m.Stop()
	if err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}
