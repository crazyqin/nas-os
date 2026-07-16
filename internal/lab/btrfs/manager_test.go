package btrfs

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewBtrfsManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewBtrfsManager(logger)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewBtrfsManager(logger)
	handler := NewHandler(mgr, logger)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRAIDProfile_Constants(t *testing.T) {
	tests := []struct {
		profile RAIDProfile
		want    string
	}{
		{RAIDSingle, "single"},
		{RAIDRAID0, "raid0"},
		{RAIDRAID1, "raid1"},
		{RAIDRAID5, "raid5"},
		{RAIDRAID6, "raid6"},
		{RAIDRAID10, "raid10"},
		{RAIDDUP, "dup"},
	}

	for _, tt := range tests {
		if string(tt.profile) != tt.want {
			t.Errorf("RAIDProfile %v = %s, want %s", tt.profile, string(tt.profile), tt.want)
		}
	}
}

// 集成测试 - 需要root权限和btrfs文件系统.
func TestBtrfsManager_CreatePool_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	logger, _ := zap.NewDevelopment()
	mgr := NewBtrfsManager(logger)
	ctx := context.Background()

	// 这个测试需要真实设备，跳过
	t.Skip("requires real btrfs devices")

	err := mgr.CreatePool(ctx, "test-pool", []string{"/dev/loop0"}, RAIDSingle)
	if err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}
}
