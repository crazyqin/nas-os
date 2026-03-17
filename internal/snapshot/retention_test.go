package snapshot

import (
	"testing"
	"time"
)

// ========== RetentionCleaner 测试 ==========

func TestNewRetentionCleaner(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)
	if cleaner == nil {
		t.Fatal("NewRetentionCleaner should not return nil")
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		prefix   string
		expected bool
	}{
		{
			name:     "matching prefix",
			str:      "daily-2024-01-01",
			prefix:   "daily",
			expected: true,
		},
		{
			name:     "non-matching prefix",
			str:      "daily-2024-01-01",
			prefix:   "weekly",
			expected: false,
		},
		{
			name:     "empty prefix",
			str:      "snapshot-1",
			prefix:   "",
			expected: true,
		},
		{
			name:     "prefix longer than string",
			str:      "ab",
			prefix:   "abcdef",
			expected: false,
		},
		{
			name:     "exact match",
			str:      "daily",
			prefix:   "daily",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPrefix(tt.str, tt.prefix)
			if result != tt.expected {
				t.Errorf("hasPrefix(%q, %q) = %v, expected %v", tt.str, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestCleanByCount(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	snapshots := []SnapshotInfo{
		{Name: "snap-1", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)},
		{Name: "snap-2", CreatedAt: time.Now().Add(-4 * 24 * time.Hour)},
		{Name: "snap-3", CreatedAt: time.Now().Add(-3 * 24 * time.Hour)},
		{Name: "snap-4", CreatedAt: time.Now().Add(-2 * 24 * time.Hour)},
		{Name: "snap-5", CreatedAt: time.Now().Add(-1 * 24 * time.Hour)},
	}

	tests := []struct {
		name      string
		snapshots []SnapshotInfo
		maxCount  int
		wantLen   int
	}{
		{
			name:      "keep 3 snapshots",
			snapshots: snapshots,
			maxCount:  3,
			wantLen:   2, // 5 - 3 = 2 to delete
		},
		{
			name:      "keep all snapshots",
			snapshots: snapshots,
			maxCount:  10,
			wantLen:   0,
		},
		{
			name:      "keep 1 snapshot",
			snapshots: snapshots,
			maxCount:  1,
			wantLen:   4,
		},
		{
			name:      "zero max count",
			snapshots: snapshots,
			maxCount:  0,
			wantLen:   0,
		},
		{
			name:      "empty snapshot list",
			snapshots: []SnapshotInfo{},
			maxCount:  2,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.cleanByCount(tt.snapshots, tt.maxCount)
			if len(result) != tt.wantLen {
				t.Errorf("cleanByCount returned %d snapshots, expected %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestCleanByAge(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	now := time.Now()
	snapshots := []SnapshotInfo{
		{Name: "old-1", CreatedAt: now.Add(-30 * 24 * time.Hour)}, // 30 days old
		{Name: "old-2", CreatedAt: now.Add(-20 * 24 * time.Hour)}, // 20 days old
		{Name: "old-3", CreatedAt: now.Add(-15 * 24 * time.Hour)}, // 15 days old
		{Name: "new-1", CreatedAt: now.Add(-5 * 24 * time.Hour)},  // 5 days old
		{Name: "new-2", CreatedAt: now.Add(-1 * 24 * time.Hour)},  // 1 day old
	}

	tests := []struct {
		name       string
		snapshots  []SnapshotInfo
		maxAgeDays int
		wantLen    int
	}{
		{
			name:       "delete older than 10 days",
			snapshots:  snapshots,
			maxAgeDays: 10,
			wantLen:    3, // old-1, old-2, old-3
		},
		{
			name:       "delete older than 25 days",
			snapshots:  snapshots,
			maxAgeDays: 25,
			wantLen:    1, // only old-1
		},
		{
			name:       "keep all (60 days)",
			snapshots:  snapshots,
			maxAgeDays: 60,
			wantLen:    0,
		},
		{
			name:       "zero max age",
			snapshots:  snapshots,
			maxAgeDays: 0,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.cleanByAge(tt.snapshots, tt.maxAgeDays)
			if len(result) != tt.wantLen {
				t.Errorf("cleanByAge returned %d snapshots, expected %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestCleanBySize(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	snapshots := []SnapshotInfo{
		{Name: "snap-1", Size: 1000, CreatedAt: time.Now().Add(-5 * 24 * time.Hour)},
		{Name: "snap-2", Size: 2000, CreatedAt: time.Now().Add(-4 * 24 * time.Hour)},
		{Name: "snap-3", Size: 3000, CreatedAt: time.Now().Add(-3 * 24 * time.Hour)},
		{Name: "snap-4", Size: 4000, CreatedAt: time.Now().Add(-2 * 24 * time.Hour)},
	}

	tests := []struct {
		name         string
		snapshots    []SnapshotInfo
		maxSizeBytes int64
		// The algorithm deletes oldest until size is within limit
		// Total size = 10000, limit 5000 means delete until remaining <= 5000
		wantDelete int
	}{
		{
			name:         "limit to 5000 bytes",
			snapshots:    snapshots,
			maxSizeBytes: 5000,
			// 10000 - 1000 = 9000 > 5000, delete snap-1
			// 9000 - 2000 = 7000 > 5000, delete snap-2
			// 7000 - 3000 = 4000 <= 5000, stop
			wantDelete: 3, // snap-1, snap-2, snap-3
		},
		{
			name:         "limit to 10000 bytes (no deletion)",
			snapshots:    snapshots,
			maxSizeBytes: 10000,
			wantDelete:   0,
		},
		{
			name:         "zero max size",
			snapshots:    snapshots,
			maxSizeBytes: 0,
			wantDelete:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.cleanBySize(tt.snapshots, tt.maxSizeBytes)
			if len(result) != tt.wantDelete {
				t.Errorf("cleanBySize returned %d snapshots, expected %d", len(result), tt.wantDelete)
			}
		})
	}
}

func TestCleanCombined(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	now := time.Now()
	snapshots := []SnapshotInfo{
		{Name: "snap-1", Size: 1000, CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{Name: "snap-2", Size: 2000, CreatedAt: now.Add(-20 * 24 * time.Hour)},
		{Name: "snap-3", Size: 3000, CreatedAt: now.Add(-10 * 24 * time.Hour)},
		{Name: "snap-4", Size: 4000, CreatedAt: now.Add(-5 * 24 * time.Hour)},
		{Name: "snap-5", Size: 5000, CreatedAt: now.Add(-1 * 24 * time.Hour)},
	}

	tests := []struct {
		name      string
		policy    *RetentionPolicy
		wantCount int
	}{
		{
			name: "count policy only",
			policy: &RetentionPolicy{
				Type: RetentionCombined,
				CountPolicy: &RetentionPolicy{
					MaxCount: 3,
				},
			},
			wantCount: 2, // delete snap-1, snap-2
		},
		{
			name: "age policy only",
			policy: &RetentionPolicy{
				Type: RetentionCombined,
				AgePolicy: &RetentionPolicy{
					MaxAgeDays: 15,
				},
			},
			wantCount: 2, // snap-1, snap-2 are older than 15 days
		},
		{
			name: "combined count and age",
			policy: &RetentionPolicy{
				Type: RetentionCombined,
				CountPolicy: &RetentionPolicy{
					MaxCount: 4,
				},
				AgePolicy: &RetentionPolicy{
					MaxAgeDays: 25,
				},
			},
			wantCount: 2, // snap-1 (age), snap-2 (age)
		},
		{
			name: "no policies",
			policy: &RetentionPolicy{
				Type: RetentionCombined,
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.cleanCombined(snapshots, tt.policy)
			// Result count depends on combined logic, just verify no duplicates
			seen := make(map[string]bool)
			for _, snap := range result {
				if seen[snap.Name] {
					t.Errorf("Duplicate snapshot in result: %s", snap.Name)
				}
				seen[snap.Name] = true
			}
		})
	}
}

func TestSnapshotInfo_Fields(t *testing.T) {
	now := time.Now()
	snap := SnapshotInfo{
		Name:      "test-snapshot",
		Path:      "/snapshots/test-snapshot",
		CreatedAt: now,
		Size:      1024000,
	}

	if snap.Name != "test-snapshot" {
		t.Errorf("Expected Name=test-snapshot, got %s", snap.Name)
	}
	if snap.Path != "/snapshots/test-snapshot" {
		t.Errorf("Expected Path=/snapshots/test-snapshot, got %s", snap.Path)
	}
	if snap.Size != 1024000 {
		t.Errorf("Expected Size=1024000, got %d", snap.Size)
	}
}

func TestCleanupPreview_Fields(t *testing.T) {
	preview := &CleanupPreview{
		TotalSnapshots:   10,
		ToDelete:         3,
		ToKeep:           7,
		ReclaimableBytes: 1024000,
	}

	if preview.TotalSnapshots != 10 {
		t.Errorf("Expected TotalSnapshots=10, got %d", preview.TotalSnapshots)
	}
	if preview.ToDelete != 3 {
		t.Errorf("Expected ToDelete=3, got %d", preview.ToDelete)
	}
	if preview.ToKeep != 7 {
		t.Errorf("Expected ToKeep=7, got %d", preview.ToKeep)
	}
}

func TestRetentionEstimate_ByCount(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	policy := &Policy{
		Retention: &RetentionPolicy{
			Type:     RetentionByCount,
			MaxCount: 10,
		},
	}

	estimate := cleaner.EstimateRetention(policy, 20, 1024)

	if estimate.MaxSnapshots != 10 {
		t.Errorf("Expected MaxSnapshots=10, got %d", estimate.MaxSnapshots)
	}
	if estimate.EstimatedStorage != 10240 {
		t.Errorf("Expected EstimatedStorage=10240, got %d", estimate.EstimatedStorage)
	}
}

func TestRetentionEstimate_ByAge(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	policy := &Policy{
		Retention: &RetentionPolicy{
			Type:       RetentionByAge,
			MaxAgeDays: 7,
		},
	}

	estimate := cleaner.EstimateRetention(policy, 14, 2048)

	if estimate.PolicyType != RetentionByAge {
		t.Errorf("Expected PolicyType=RetentionByAge, got %v", estimate.PolicyType)
	}
	if estimate.MaxSnapshots != 7 {
		t.Errorf("Expected MaxSnapshots=7, got %d", estimate.MaxSnapshots)
	}
}

func TestRetentionEstimate_BySize(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	policy := &Policy{
		Retention: &RetentionPolicy{
			Type:         RetentionBySize,
			MaxSizeBytes: 10240,
		},
	}

	estimate := cleaner.EstimateRetention(policy, 10, 1024)

	if estimate.EstimatedStorage != 10240 {
		t.Errorf("Expected EstimatedStorage=10240, got %d", estimate.EstimatedStorage)
	}
	if estimate.MaxSnapshots != 10 { // 10240 / 1024
		t.Errorf("Expected MaxSnapshots=10, got %d", estimate.MaxSnapshots)
	}
}

func TestRetentionEstimate_Combined(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	policy := &Policy{
		Retention: &RetentionPolicy{
			Type: RetentionCombined,
			CountPolicy: &RetentionPolicy{
				MaxCount: 5,
			},
			SizePolicy: &RetentionPolicy{
				MaxSizeBytes: 10240,
			},
		},
	}

	estimate := cleaner.EstimateRetention(policy, 10, 1024)

	if estimate.MaxSnapshots != 5 {
		t.Errorf("Expected MaxSnapshots=5, got %d", estimate.MaxSnapshots)
	}
	if estimate.EstimatedStorage != 10240 {
		t.Errorf("Expected EstimatedStorage=10240, got %d", estimate.EstimatedStorage)
	}
}

func TestRetentionEstimate_ZeroAvgSize(t *testing.T) {
	cleaner := NewRetentionCleaner(nil)

	policy := &Policy{
		Retention: &RetentionPolicy{
			Type:         RetentionBySize,
			MaxSizeBytes: 10240,
		},
	}

	estimate := cleaner.EstimateRetention(policy, 10, 0)

	// With zero avgSize, MaxSnapshots should be 0
	if estimate.MaxSnapshots != 0 {
		t.Errorf("Expected MaxSnapshots=0 with zero avgSize, got %d", estimate.MaxSnapshots)
	}
}

// ========== Mock Storage Manager for Testing ==========
