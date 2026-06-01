package containerreslimiter

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	m := NewManager(&Config{
		Enabled:             true,
		DefaultStrategy:     StrategyBalanced,
		SampleInterval:      time.Minute,
		MinSamplesRequired:  5,
		AnalysisWindow:      time.Hour * 24,
		MaxAdjustmentPerDay: 3,
		CPUBufferPercent:    20,
		MemoryBufferPercent: 15,
		AutoApply:           false,
	})
	return m
}

func TestRegisterContainer(t *testing.T) {
	m := newTestManager()

	err := m.RegisterContainer("c1", "nginx", "nginx:latest", ResourceLimits{
		CPUMilliCores: 1000,
		MemoryBytes:   512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("RegisterContainer failed: %v", err)
	}

	// 重复注册应覆盖
	err = m.RegisterContainer("c1", "nginx-v2", "nginx:1.25", ResourceLimits{
		CPUMilliCores: 2000,
		MemoryBytes:   1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("re-register failed: %v", err)
	}

	p, err := m.GetContainer("c1")
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if p.ContainerName != "nginx-v2" {
		t.Errorf("expected name nginx-v2, got %s", p.ContainerName)
	}

	// 空ID应失败
	err = m.RegisterContainer("", "test", "test:latest", ResourceLimits{})
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestRecordUsage(t *testing.T) {
	m := newTestManager()

	err := m.RecordUsage("nonexistent", UsageSample{})
	if err != ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}

	m.RegisterContainer("c1", "app", "app:latest", ResourceLimits{
		CPUMilliCores: 1000,
		MemoryBytes:   512 * 1024 * 1024,
	})

	for i := 0; i < 10; i++ {
		err = m.RecordUsage("c1", UsageSample{
			Timestamp:     time.Now().Add(time.Duration(i) * time.Minute),
			CPUMilliCores: float64(200 + i*50),
			MemoryBytes:   int64(100*1024*1024 + i*10*1024*1024),
		})
		if err != nil {
			t.Fatalf("RecordUsage failed: %v", err)
		}
	}

	p, _ := m.GetContainer("c1")
	// GetContainer不返回Samples
	if p.Samples != nil {
		t.Error("GetContainer should not expose samples")
	}
}

func TestAnalyzeContainer(t *testing.T) {
	m := newTestManager()
	m.config.MinSamplesRequired = 5

	m.RegisterContainer("c1", "web", "nginx:latest", ResourceLimits{
		CPUMilliCores: 2000,
		MemoryBytes:   1024 * 1024 * 1024,
	})

	// 不够数据
	_, err := m.AnalyzeContainer("c1")
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}

	// 添加足够数据
	for i := 0; i < 20; i++ {
		m.RecordUsage("c1", UsageSample{
			Timestamp:     time.Now().Add(time.Duration(i) * time.Minute),
			CPUMilliCores: float64(300 + i*20),
			MemoryBytes:   int64(200*1024*1024 + i*5*1024*1024),
		})
	}

	analysis, err := m.AnalyzeContainer("c1")
	if err != nil {
		t.Fatalf("AnalyzeContainer failed: %v", err)
	}
	if analysis.SampleCount != 20 {
		t.Errorf("expected 20 samples, got %d", analysis.SampleCount)
	}
	if analysis.CPUPercentile.P95 <= 0 {
		t.Error("expected positive P95 CPU")
	}
	if analysis.CPUPercentile.Max <= analysis.CPUPercentile.P95 {
		t.Error("expected Max >= P95")
	}
	if analysis.Recommendation == "" {
		t.Error("expected non-empty recommendation")
	}
}

func TestCalculateRecommendedLimits(t *testing.T) {
	m := newTestManager()
	m.config.MinSamplesRequired = 5

	m.RegisterContainer("c1", "api", "api:v1", ResourceLimits{
		CPUMilliCores: 4000,
		MemoryBytes:   2 * 1024 * 1024 * 1024,
	})

	// 不够数据
	_, err := m.CalculateRecommendedLimits("c1")
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}

	// 添加数据（使用率约50%）
	for i := 0; i < 20; i++ {
		m.RecordUsage("c1", UsageSample{
			Timestamp:     time.Now().Add(time.Duration(i) * time.Minute),
			CPUMilliCores: 1800 + float64(i%3)*100,
			MemoryBytes:   900 * 1024 * 1024,
		})
	}

	limits, err := m.CalculateRecommendedLimits("c1")
	if err != nil {
		t.Fatalf("CalculateRecommendedLimits failed: %v", err)
	}
	if limits.CPUMilliCores <= 0 {
		t.Error("expected positive CPU limit")
	}
	if limits.MemoryBytes <= 0 {
		t.Error("expected positive memory limit")
	}
	// 推荐值应小于当前限制（因为使用率只有~50%）
	if limits.CPUMilliCores >= 4000 {
		t.Errorf("expected recommended CPU < 4000, got %d", limits.CPUMilliCores)
	}
}

func TestStrategies(t *testing.T) {
	m := newTestManager()
	m.config.MinSamplesRequired = 5

	strategies := []LimitStrategy{StrategyConservative, StrategyBalanced, StrategyAggressive}

	for _, strategy := range strategies {
		m.RegisterContainer(string(strategy), "test", "test:latest", ResourceLimits{
			CPUMilliCores: 2000,
			MemoryBytes:   1024 * 1024 * 1024,
		})
		m.SetStrategy(string(strategy), strategy)

		for i := 0; i < 10; i++ {
			m.RecordUsage(string(strategy), UsageSample{
				Timestamp:     time.Now().Add(time.Duration(i) * time.Minute),
				CPUMilliCores: 800,
				MemoryBytes:   500 * 1024 * 1024,
			})
		}

		limits, err := m.CalculateRecommendedLimits(string(strategy))
		if err != nil {
			t.Fatalf("CalculateRecommendedLimits for %s failed: %v", strategy, err)
		}

		if limits.CPUMilliCores <= 0 || limits.MemoryBytes <= 0 {
			t.Errorf("invalid limits for strategy %s", strategy)
		}
	}
}

func TestAutoAdjust(t *testing.T) {
	m := newTestManager()
	m.config.AutoApply = true
	m.config.MinSamplesRequired = 5

	m.RegisterContainer("c1", "app", "app:latest", ResourceLimits{
		CPUMilliCores: 4000,
		MemoryBytes:   2 * 1024 * 1024 * 1024,
	})

	// 添加低使用率数据
	for i := 0; i < 20; i++ {
		m.RecordUsage("c1", UsageSample{
			Timestamp:     time.Now().Add(time.Duration(i) * time.Minute),
			CPUMilliCores: 500,
			MemoryBytes:   300 * 1024 * 1024,
		})
	}

	adjustments, err := m.AutoAdjust()
	if err != nil {
		t.Fatalf("AutoAdjust failed: %v", err)
	}
	if len(adjustments) == 0 {
		t.Error("expected at least one adjustment")
	}

	// 检查调整历史
	history := m.GetAdjustmentHistory("", 10)
	if len(history) == 0 {
		t.Error("expected adjustment history")
	}
}

func TestAutoAdjustDisabled(t *testing.T) {
	m := newTestManager()
	m.config.AutoApply = false

	_, err := m.AutoAdjust()
	if err == nil {
		t.Error("expected error when auto-apply disabled")
	}
}

func TestDashboard(t *testing.T) {
	m := newTestManager()

	m.RegisterContainer("c1", "web", "nginx:latest", ResourceLimits{
		CPUMilliCores: 2000,
		MemoryBytes:   1024 * 1024 * 1024,
	})
	m.SetStrategy("c1", StrategyBalanced)

	m.RegisterContainer("c2", "db", "postgres:15", ResourceLimits{
		CPUMilliCores: 4000,
		MemoryBytes:   4 * 1024 * 1024 * 1024,
	})
	m.SetStrategy("c2", StrategyManual)

	dash := m.GetDashboard()
	if dash["totalContainers"] != 2 {
		t.Errorf("expected 2 containers, got %v", dash["totalContainers"])
	}
	if dash["autoManaged"] != 1 {
		t.Errorf("expected 1 auto-managed, got %v", dash["autoManaged"])
	}
}

func TestGetAllContainers(t *testing.T) {
	m := newTestManager()

	m.RegisterContainer("c1", "web", "nginx:latest", ResourceLimits{})
	m.RegisterContainer("c2", "db", "postgres:15", ResourceLimits{})

	all := m.GetAllContainers()
	if len(all) != 2 {
		t.Errorf("expected 2 containers, got %d", len(all))
	}
}

func TestContainerNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.GetContainer("nonexistent")
	if err != ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}

	_, err = m.AnalyzeContainer("nonexistent")
	if err != ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}

	err = m.SetStrategy("nonexistent", StrategyAggressive)
	if err != ErrContainerNotFound {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}
}
