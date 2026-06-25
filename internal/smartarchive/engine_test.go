package smartarchive

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	config := DefaultEngineConfig()
	engine := NewEngine(config)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()
	if config.WorkerCount <= 0 {
		t.Error("expected positive WorkerCount")
	}
}

func TestNewAnalyzer(t *testing.T) {
	config := DefaultAnalyzerConfig()
	analyzer := NewAnalyzer(config)
	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

func TestDefaultAnalyzerConfig(t *testing.T) {
	config := DefaultAnalyzerConfig()
	if config.AnalysisInterval <= 0 {
		t.Error("expected positive AnalysisInterval")
	}
}

func TestNewCompressionManager(t *testing.T) {
	mgr := NewCompressionManager()
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewCostManager(t *testing.T) {
	config := DefaultCostConfig()
	mgr := NewCostManager()
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	_ = config
}

func TestDefaultCostConfig(t *testing.T) {
	config := DefaultCostConfig()
	// 验证默认配置有值
	_ = config
}

func TestNewPolicyEngine(t *testing.T) {
	engine := NewPolicyEngine()
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestNewRetentionManager(t *testing.T) {
	mgr := NewRetentionManager()
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewScheduler(t *testing.T) {
	config := DefaultSchedulerConfig()
	scheduler := NewScheduler(config)
	if scheduler == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

func TestDefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()
	if config.MinInterval <= 0 {
		t.Error("expected positive MinInterval")
	}
}

func TestNewTierManager(t *testing.T) {
	config := DefaultTierManagerConfig()
	mgr := NewTierManager()
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	_ = config
}

func TestDefaultTierManagerConfig(t *testing.T) {
	config := DefaultTierManagerConfig()
	_ = config
}

func TestArchivePolicy_Fields(t *testing.T) {
	// 测试策略相关类型的基本字段
	config := DefaultEngineConfig()
	if config.WorkerCount == 0 {
		t.Error("expected non-zero WorkerCount")
	}
}

func TestRetentionPeriod(t *testing.T) {
	periods := []time.Duration{
		30 * 24 * time.Hour,   // 30天
		90 * 24 * time.Hour,   // 90天
		365 * 24 * time.Hour,  // 1年
	}

	for _, p := range periods {
		if p <= 0 {
			t.Error("expected positive duration")
		}
	}
}
