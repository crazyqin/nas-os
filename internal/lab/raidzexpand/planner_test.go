package raidzexpand

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBuildExpansionPlanHealthy(t *testing.T) {
	provider := &MockPoolStatusProvider{
		Health:     HealthOnline,
		TotalBytes: 10 * tebibyte,
		UsedBytes:  6 * tebibyte,
		FreeBytes:  4 * tebibyte,
		VDevDisks:  []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"},
		AvailableDisks: map[string]bool{
			"/dev/sde": true,
		},
	}
	cfg := &ExpansionConfig{
		PoolName:   "tank",
		VDevPath:   "raidz1-0",
		NewDisks:   []string{"/dev/sde"},
		RAIDZLevel: RAIDZ1,
	}

	plan, err := BuildExpansionPlan(context.Background(), cfg, NewHealthChecker(provider), tebibyte, gibibyte)
	if err != nil {
		t.Fatalf("BuildExpansionPlan returned error: %v", err)
	}

	if plan.CurrentDiskCount != 4 || plan.NewDiskCount != 1 || plan.FinalDiskCount != 5 {
		t.Fatalf("unexpected disk counts: current=%d new=%d final=%d", plan.CurrentDiskCount, plan.NewDiskCount, plan.FinalDiskCount)
	}
	if plan.ParityDiskCount != 1 {
		t.Fatalf("expected one parity disk, got %d", plan.ParityDiskCount)
	}
	if plan.EstimatedCapacityGain != tebibyte {
		t.Fatalf("expected 1 TiB capacity gain, got %d", plan.EstimatedCapacityGain)
	}
	if plan.CurrentUsableCapacity != 3*tebibyte || plan.ProjectedUsableCapacity != 4*tebibyte {
		t.Fatalf("unexpected usable capacity projection: current=%d projected=%d", plan.CurrentUsableCapacity, plan.ProjectedUsableCapacity)
	}
	wantResilver := uint64(6 * tebibyte / 5)
	if plan.EstimatedResilverBytes != wantResilver {
		t.Fatalf("expected resilver bytes %d, got %d", wantResilver, plan.EstimatedResilverBytes)
	}
	if plan.EstimatedResilverTime != estimateDuration(wantResilver, gibibyte) {
		t.Fatalf("unexpected resilver ETA: %s", plan.EstimatedResilverTime)
	}
	if plan.RiskLevel != RiskLow {
		t.Fatalf("expected low risk, got %s warnings=%v", plan.RiskLevel, plan.Warnings)
	}
	if plan.Health == nil || !plan.Health.PoolHealthy || !plan.Health.DisksAvailable {
		t.Fatalf("expected embedded healthy preflight result: %#v", plan.Health)
	}
}

func TestBuildExpansionPlanWarningsAndRisk(t *testing.T) {
	provider := &MockPoolStatusProvider{
		Health:     HealthDegraded,
		TotalBytes: 12 * tebibyte,
		UsedBytes:  10 * tebibyte,
		FreeBytes:  50 * gibibyte,
		VDevDisks:  []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd", "/dev/sde"},
		AvailableDisks: map[string]bool{
			"/dev/sdf": true,
			"/dev/sdg": true,
		},
	}
	cfg := &ExpansionConfig{
		PoolName:   "tank",
		VDevPath:   "raidz1-0",
		NewDisks:   []string{"/dev/sdf", "/dev/sdg"},
		RAIDZLevel: RAIDZ1,
	}

	plan, err := BuildExpansionPlan(context.Background(), cfg, NewHealthChecker(provider), tebibyte, 10*mebibyte)
	if err != nil {
		t.Fatalf("BuildExpansionPlan returned error: %v", err)
	}
	if plan.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk, got %s warnings=%v", plan.RiskLevel, plan.Warnings)
	}
	joined := strings.Join(plan.Warnings, "\n")
	for _, want := range []string{"状态为 DEGRADED", "空闲空间低于预计数据重排量", "RAIDZ1 扩展到 6 盘", "一次添加多块盘", "超过 24 小时"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings %q do not contain %q", joined, want)
		}
	}
}

func TestBuildExpansionPlanValidation(t *testing.T) {
	_, err := BuildExpansionPlan(context.Background(), &ExpansionConfig{
		PoolName:   "tank",
		VDevPath:   "raidz2-0",
		NewDisks:   []string{"/dev/sde"},
		RAIDZLevel: RAIDZ2,
	}, nil, tebibyte, 0)
	if err == nil || !strings.Contains(err.Error(), "无法确定") {
		t.Fatalf("expected unknown topology error, got %v", err)
	}

	provider := &MockPoolStatusProvider{
		Health:         HealthOnline,
		TotalBytes:     tebibyte,
		UsedBytes:      100 * gibibyte,
		FreeBytes:      900 * gibibyte,
		VDevDisks:      []string{"/dev/sda", "/dev/sdb"},
		AvailableDisks: map[string]bool{"/dev/sdc": true},
	}
	_, err = BuildExpansionPlan(context.Background(), &ExpansionConfig{
		PoolName:   "tank",
		VDevPath:   "raidz2-0",
		NewDisks:   []string{"/dev/sdc"},
		RAIDZLevel: RAIDZ2,
	}, NewHealthChecker(provider), tebibyte, 0)
	if err == nil || !strings.Contains(err.Error(), "最小拓扑") {
		t.Fatalf("expected minimum topology error, got %v", err)
	}

	if got := estimateDuration(1, 10); got != time.Second {
		t.Fatalf("duration should round up to one second, got %s", got)
	}
}

func TestBuildExpansionPlanHighRiskForSingleHealthFailure(t *testing.T) {
	provider := &MockPoolStatusProvider{
		Health:     HealthDegraded,
		TotalBytes: 4 * tebibyte,
		UsedBytes:  2 * tebibyte,
		FreeBytes:  2 * tebibyte,
		VDevDisks:  []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"},
		AvailableDisks: map[string]bool{
			"/dev/sde": true,
		},
	}
	plan, err := BuildExpansionPlan(context.Background(), &ExpansionConfig{PoolName: "tank", VDevPath: "raidz1-0", NewDisks: []string{"/dev/sde"}, RAIDZLevel: RAIDZ1}, NewHealthChecker(provider), tebibyte, gibibyte)
	if err != nil {
		t.Fatalf("BuildExpansionPlan returned error: %v", err)
	}
	if plan.RiskLevel != RiskHigh {
		t.Fatalf("single degraded health failure should be high risk, got %s warnings=%v", plan.RiskLevel, plan.Warnings)
	}
}

func TestBuildExpansionPlanWarnsWhenFreeBytesZero(t *testing.T) {
	provider := &MockPoolStatusProvider{
		Health:     HealthOnline,
		TotalBytes: 4 * tebibyte,
		UsedBytes:  3 * tebibyte,
		FreeBytes:  0,
		VDevDisks:  []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"},
		AvailableDisks: map[string]bool{
			"/dev/sde": true,
		},
	}
	plan, err := BuildExpansionPlan(context.Background(), &ExpansionConfig{PoolName: "tank", VDevPath: "raidz1-0", NewDisks: []string{"/dev/sde"}, RAIDZLevel: RAIDZ1}, NewHealthChecker(provider), tebibyte, gibibyte)
	if err != nil {
		t.Fatalf("BuildExpansionPlan returned error: %v", err)
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "空闲空间低于预计数据重排量") {
		t.Fatalf("expected low free space warning, got %v", plan.Warnings)
	}
	if plan.EstimatedResilverSeconds <= 0 {
		t.Fatalf("expected explicit resilver seconds, got %d", plan.EstimatedResilverSeconds)
	}
}

const (
	mebibyte uint64 = 1024 * 1024
	gibibyte uint64 = 1024 * mebibyte
	tebibyte uint64 = 1024 * gibibyte
)
