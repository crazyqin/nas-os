package fastdedup

import (
	"testing"
)

func TestNewFastDedupEngine(t *testing.T) {
	cfg := DefaultEngineConfig()
	engine := NewFastDedupEngine(cfg)
	if engine == nil {
		t.Fatal("engine should not be nil")
	}
	if engine.IsRunning() {
		t.Error("engine should not be running initially")
	}
}

func TestStartStop(t *testing.T) {
	engine := NewFastDedupEngine(DefaultEngineConfig())

	if err := engine.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !engine.IsRunning() {
		t.Error("engine should be running after start")
	}
	if err := engine.Start(); err != ErrEngineRunning {
		t.Errorf("expected ErrEngineRunning, got %v", err)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if engine.IsRunning() {
		t.Error("engine should not be running after stop")
	}
	if err := engine.Stop(); err != ErrEngineNotRunning {
		t.Errorf("expected ErrEngineNotRunning, got %v", err)
	}
}

func TestPolicyCRUD(t *testing.T) {
	engine := NewFastDedupEngine(DefaultEngineConfig())

	policy := &DedupPolicy{
		Name:         "test-policy",
		Mode:         ModeRealtime,
		Algorithm:    AlgoHybrid,
		MinBlockSize: 4096,
		MaxBlockSize: 131072,
		Tiers:        []StorageTier{TierNVMe, TierSSD},
		Enabled:      true,
	}

	if err := engine.AddPolicy(policy); err != nil {
		t.Fatalf("add policy failed: %v", err)
	}
	if err := engine.AddPolicy(policy); err != ErrPolicyExists {
		t.Errorf("expected ErrPolicyExists, got %v", err)
	}

	got, err := engine.GetPolicy("test-policy")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if got.Name != "test-policy" {
		t.Errorf("expected name test-policy, got %s", got.Name)
	}

	policies := engine.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	if err := engine.RemovePolicy("test-policy"); err != nil {
		t.Fatalf("remove policy failed: %v", err)
	}
	if _, err := engine.GetPolicy("test-policy"); err != ErrPolicyNotFound {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}
}

func TestRunDedup(t *testing.T) {
	engine := NewFastDedupEngine(DefaultEngineConfig())
	engine.Start()

	policy := &DedupPolicy{
		Name:         "default",
		Mode:         ModeRealtime,
		Algorithm:    AlgoHybrid,
		MinBlockSize: 4096,
		MaxBlockSize: 131072,
		Tiers:        []StorageTier{TierNVMe},
		Enabled:      true,
	}
	engine.AddPolicy(policy)

	result, err := engine.RunDedup("default")
	if err != nil {
		t.Fatalf("run dedup failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Duration < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestRegisterBlock(t *testing.T) {
	engine := NewFastDedupEngine(DefaultEngineConfig())

	if err := engine.RegisterBlock("abc123", 4096, TierNVMe); err != nil {
		t.Fatalf("register block failed: %v", err)
	}
	if engine.GetBlockCount() != 1 {
		t.Errorf("expected 1 block, got %d", engine.GetBlockCount())
	}

	// 重复块应该增加引用计数
	if err := engine.RegisterBlock("abc123", 4096, TierNVMe); err != nil {
		t.Fatalf("register duplicate block failed: %v", err)
	}
	if engine.GetBlockCount() != 1 {
		t.Errorf("expected 1 block after dedup, got %d", engine.GetBlockCount())
	}
}

func TestGetStats(t *testing.T) {
	engine := NewFastDedupEngine(DefaultEngineConfig())
	stats := engine.GetStats()
	if stats.TotalBlocks != 0 {
		t.Errorf("expected 0 total blocks, got %d", stats.TotalBlocks)
	}
	engine.ResetStats()
	stats = engine.GetStats()
	if stats.TotalBlocks != 0 {
		t.Error("stats should be zero after reset")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultEngineConfig()
	if cfg.DefaultMode != ModeRealtime {
		t.Errorf("expected realtime mode, got %s", cfg.DefaultMode)
	}
	if !cfg.NVMeOptimized {
		t.Error("NVMe should be optimized by default")
	}
	if cfg.WorkerCount != 8 {
		t.Errorf("expected 8 workers, got %d", cfg.WorkerCount)
	}
}
