package storageefficiency

import (
	"testing"
)

func TestManager_CreateConfig(t *testing.T) {
	manager := NewManager()

	req := &CreateConfigRequest{
		Name:        "test-efficiency",
		Strategy:    StrategyBoth,
		Compression: AlgoLZ4,
		DedupMode:   DedupInline,
		ChunkSizeKB: 64,
	}

	config, err := manager.CreateConfig(req)
	if err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	if config.Name != "test-efficiency" {
		t.Errorf("expected name 'test-efficiency', got '%s'", config.Name)
	}

	if config.Strategy != StrategyBoth {
		t.Errorf("expected strategy 'both', got '%s'", config.Strategy)
	}
}

func TestManager_RunEfficiency(t *testing.T) {
	manager := NewManager()

	config, _ := manager.CreateConfig(&CreateConfigRequest{
		Name:     "test",
		Strategy: StrategyBoth,
	})

	task, err := manager.RunEfficiency(config.ID)
	if err != nil {
		t.Fatalf("RunEfficiency failed: %v", err)
	}

	if task.Status != TaskPending && task.Status != TaskRunning {
		t.Errorf("expected pending or running status, got '%s'", task.Status)
	}
}

func TestManager_AnalyzeStorage(t *testing.T) {
	manager := NewManager()

	analysis := manager.AnalyzeStorage()
	if analysis == nil {
		t.Fatal("expected non-nil analysis")
	}

	if analysis.TotalCapacity == 0 {
		t.Error("expected non-zero total capacity")
	}

	if len(analysis.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}
}

func TestManager_ListConfigs(t *testing.T) {
	manager := NewManager()

	manager.CreateConfig(&CreateConfigRequest{
		Name:     "config1",
		Strategy: StrategyDedup,
	})

	manager.CreateConfig(&CreateConfigRequest{
		Name:     "config2",
		Strategy: StrategyCompress,
	})

	configs := manager.ListConfigs()
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
}

func TestManager_InvalidStrategy(t *testing.T) {
	manager := NewManager()

	_, err := manager.CreateConfig(&CreateConfigRequest{
		Name:     "test",
		Strategy: "invalid",
	})

	if err != ErrInvalidStrategy {
		t.Errorf("expected ErrInvalidStrategy, got: %v", err)
	}
}
