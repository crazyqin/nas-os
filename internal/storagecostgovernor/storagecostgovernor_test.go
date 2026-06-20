package storagecostgovernor

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewCostGovernor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("valid config", func(t *testing.T) {
		config := &GovernorConfig{
			ForecastWindow: 90 * 24 * time.Hour,
			AlertThreshold: 80,
			Currency:       "CNY",
			ReviewCycle:    24 * time.Hour,
		}

		g, err := NewCostGovernor(config, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g == nil {
			t.Fatal("expected non-nil governor")
		}
		if g.config.Currency != "CNY" {
			t.Errorf("expected currency CNY, got %s", g.config.Currency)
		}
	})

	t.Run("nil config", func(t *testing.T) {
		_, err := NewCostGovernor(nil, logger)
		if err != ErrInvalidConfig {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		config := &GovernorConfig{
			ForecastWindow: 90 * 24 * time.Hour,
			Currency:       "USD",
		}

		g, err := NewCostGovernor(config, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g.logger == nil {
			t.Error("expected non-nil logger")
		}
	})

	t.Run("default values", func(t *testing.T) {
		config := &GovernorConfig{}
		g, err := NewCostGovernor(config, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g.config.ForecastWindow != 90*24*time.Hour {
			t.Errorf("expected default forecast window 90 days, got %v", g.config.ForecastWindow)
		}
		if g.config.AlertThreshold != 80.0 {
			t.Errorf("expected default alert threshold 80%%, got %v", g.config.AlertThreshold)
		}
		if g.config.Currency != "CNY" {
			t.Errorf("expected default currency CNY, got %s", g.config.Currency)
		}
	})
}

func TestRegisterPool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	t.Run("valid pool", func(t *testing.T) {
		pool := &StoragePool{
			ID:         "pool-1",
			Name:       "Main Storage",
			Type:       PoolTypeLocal,
			TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
			CostPerGB:  0.1,
			Tier:       TierHot,
			Health:     100,
		}

		err := g.RegisterPool(pool)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pool.FreeBytes != 512*1024*1024*1024 {
			t.Errorf("expected free bytes 512GB, got %d", pool.FreeBytes)
		}
		if pool.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("duplicate pool", func(t *testing.T) {
		pool := &StoragePool{
			ID:         "pool-1",
			Name:       "Duplicate Pool",
			Type:       PoolTypeNAS,
			TotalBytes: 1024 * 1024 * 1024 * 1024,
			CostPerGB:  0.2,
		}

		err := g.RegisterPool(pool)
		if err != ErrPoolAlreadyExists {
			t.Errorf("expected ErrPoolAlreadyExists, got %v", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		pool := &StoragePool{
			Name:       "No ID Pool",
			Type:       PoolTypeCloud,
			TotalBytes: 1024 * 1024 * 1024 * 1024,
			CostPerGB:  0.3,
		}

		err := g.RegisterPool(pool)
		if err != ErrInvalidPoolID {
			t.Errorf("expected ErrInvalidPoolID, got %v", err)
		}
	})

	t.Run("nil pool", func(t *testing.T) {
		err := g.RegisterPool(nil)
		if err != ErrInvalidPoolID {
			t.Errorf("expected ErrInvalidPoolID, got %v", err)
		}
	})
}

func TestUpdateUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	// 注册测试池
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		CostPerGB:  0.1,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	t.Run("valid update", func(t *testing.T) {
		newUsage := int64(600 * 1024 * 1024 * 1024) // 600GB
		err := g.UpdateUsage("pool-1", newUsage)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pool.UsedBytes != newUsage {
			t.Errorf("expected used bytes %d, got %d", newUsage, pool.UsedBytes)
		}
		expectedFree := int64(1024*1024*1024*1024) - newUsage
		if pool.FreeBytes != expectedFree {
			t.Errorf("expected free bytes %d, got %d", expectedFree, pool.FreeBytes)
		}
	})

	t.Run("non-existent pool", func(t *testing.T) {
		err := g.UpdateUsage("non-existent", 100*1024*1024*1024)
		if err != ErrPoolNotFound {
			t.Errorf("expected ErrPoolNotFound, got %v", err)
		}
	})

	t.Run("history tracking", func(t *testing.T) {
		// 更新多次
		for i := 0; i < 5; i++ {
			g.UpdateUsage("pool-1", int64(i+1)*100*1024*1024*1024)
		}

		g.mu.RLock()
		history := g.usageHistory["pool-1"]
		g.mu.RUnlock()

		if len(history) != 6 { // 初始 + 5次更新
			t.Errorf("expected 6 history records, got %d", len(history))
		}
	})
}

func TestForecastCapacity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	// 注册测试池
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		CostPerGB:  0.1,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	t.Run("insufficient data", func(t *testing.T) {
		_, err := g.ForecastCapacity("pool-1")
		if err == nil {
			t.Error("expected error for insufficient data")
		}
	})

	t.Run("valid forecast", func(t *testing.T) {
		// 添加足够的历史数据
		startTime := time.Now().Add(-30 * 24 * time.Hour) // 30天前
		for i := 0; i < 30; i++ {
			usage := int64(512*1024*1024*1024) + int64(i)*10*1024*1024*1024 // 每天增长10GB
			record := UsageRecord{
				Timestamp: startTime.Add(time.Duration(i) * 24 * time.Hour),
				UsedBytes: usage,
			}
			g.usageHistory["pool-1"] = append(g.usageHistory["pool-1"], record)
		}

		forecast, err := g.ForecastCapacity("pool-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if forecast.PoolID != "pool-1" {
			t.Errorf("expected pool ID pool-1, got %s", forecast.PoolID)
		}
		if forecast.GrowthRate <= 0 {
			t.Error("expected positive growth rate")
		}
		if forecast.Confidence < 0 || forecast.Confidence > 1 {
			t.Errorf("confidence should be between 0 and 1, got %f", forecast.Confidence)
		}
		if forecast.Predicted30 <= forecast.Current {
			t.Error("predicted 30-day usage should be greater than current")
		}
		if forecast.Predicted90 <= forecast.Predicted30 {
			t.Error("predicted 90-day usage should be greater than 30-day")
		}
	})

	t.Run("non-existent pool", func(t *testing.T) {
		_, err := g.ForecastCapacity("non-existent")
		if err != ErrPoolNotFound {
			t.Errorf("expected ErrPoolNotFound, got %v", err)
		}
	})
}

func TestCheckBudgets(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	t.Run("no alerts", func(t *testing.T) {
		g.budgets["budget-1"] = &Budget{
			ID:           "budget-1",
			Name:         "Test Budget",
			MonthlyCap:   1000,
			CurrentSpend: 500,
			AlertAt:      80,
			Period:       "monthly",
		}

		alerts, err := g.CheckBudgets()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(alerts))
		}
	})

	t.Run("warning alert", func(t *testing.T) {
		g.budgets["budget-2"] = &Budget{
			ID:           "budget-2",
			Name:         "Warning Budget",
			MonthlyCap:   1000,
			CurrentSpend: 850,
			AlertAt:      80,
			Period:       "monthly",
		}

		alerts, err := g.CheckBudgets()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) == 0 {
			t.Error("expected at least 1 alert")
		}

		found := false
		for _, alert := range alerts {
			if alert.BudgetID == "budget-2" && alert.Level == AlertWarning {
				found = true
			}
		}
		if !found {
			t.Error("expected warning alert for budget-2")
		}
	})

	t.Run("critical alert", func(t *testing.T) {
		g.budgets["budget-3"] = &Budget{
			ID:           "budget-3",
			Name:         "Over Budget",
			MonthlyCap:   1000,
			CurrentSpend: 1200,
			AlertAt:      80,
			Period:       "monthly",
		}

		alerts, err := g.CheckBudgets()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, alert := range alerts {
			if alert.BudgetID == "budget-3" && alert.Level == AlertCritical {
				found = true
			}
		}
		if !found {
			t.Error("expected critical alert for budget-3")
		}
	})
}

func TestGenerateRecommendations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	t.Run("low utilization recommendation", func(t *testing.T) {
		// 创建一个低利用率的池
		pool := &StoragePool{
			ID:         "pool-low",
			Name:       "Low Utilization Pool",
			Type:       PoolTypeLocal,
			TotalBytes: 500 * 1024 * 1024 * 1024, // 500GB
			UsedBytes:  50 * 1024 * 1024 * 1024,  // 50GB = 10%
			CostPerGB:  0.2,
			Tier:       TierHot,
		}
		g.RegisterPool(pool)

		recommendations := g.GenerateRecommendations()
		found := false
		for _, rec := range recommendations {
			if rec.Type == RecTypeResize && rec.Title == "Consider downsizing pool Low Utilization Pool" {
				found = true
			}
		}
		if !found {
			t.Error("expected resize recommendation for low utilization pool")
		}
	})

	t.Run("cold data tiering recommendation", func(t *testing.T) {
		// 创建一个高利用率的热存储池
		pool := &StoragePool{
			ID:         "pool-hot",
			Name:       "Hot Storage Pool",
			Type:       PoolTypeLocal,
			TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:  800 * 1024 * 1024 * 1024,  // 800GB = 78%
			CostPerGB:  0.5,
			Tier:       TierHot,
		}
		g.RegisterPool(pool)

		recommendations := g.GenerateRecommendations()
		found := false
		for _, rec := range recommendations {
			if rec.Type == RecTypeTierDown && rec.Title == "Archive cold data from pool Hot Storage Pool" {
				found = true
			}
		}
		if !found {
			t.Error("expected tier down recommendation for hot storage")
		}
	})

	t.Run("compression recommendation", func(t *testing.T) {
		// 创建一个高成本的池
		pool := &StoragePool{
			ID:         "pool-expensive",
			Name:       "Expensive Pool",
			Type:       PoolTypeCloud,
			TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:  500 * 1024 * 1024 * 1024,  // 500GB
			CostPerGB:  1.0,                        // 1元/GB
			Tier:       TierHot,
		}
		g.RegisterPool(pool)

		recommendations := g.GenerateRecommendations()
		found := false
		for _, rec := range recommendations {
			if rec.Type == RecTypeCompress && rec.Title == "Enable compression for pool Expensive Pool" {
				found = true
			}
		}
		if !found {
			t.Error("expected compression recommendation for expensive pool")
		}
	})

	t.Run("critical capacity recommendation", func(t *testing.T) {
		// 创建一个接近满容量的池
		pool := &StoragePool{
			ID:         "pool-full",
			Name:       "Almost Full Pool",
			Type:       PoolTypeNAS,
			TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:  950 * 1024 * 1024 * 1024,  // 950GB = 92%
			CostPerGB:  0.3,
			Tier:       TierWarm,
		}
		g.RegisterPool(pool)

		recommendations := g.GenerateRecommendations()
		found := false
		for _, rec := range recommendations {
			if rec.Type == RecTypeDelete && rec.Title == "Pool Almost Full Pool is almost full" && rec.Priority == PriorityCritical {
				found = true
			}
		}
		if !found {
			t.Error("expected critical delete recommendation for almost full pool")
		}
	})
}

func TestGetCostReport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	// 添加测试数据
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		CostPerGB:  0.2,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	report := g.GetCostReport()

	if report.Currency != "CNY" {
		t.Errorf("expected currency CNY, got %s", report.Currency)
	}
	if report.Period != "monthly" {
		t.Errorf("expected period monthly, got %s", report.Period)
	}
	if len(report.PoolReports) != 1 {
		t.Errorf("expected 1 pool report, got %d", len(report.PoolReports))
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected GeneratedAt to be set")
	}

	poolReport := report.PoolReports[0]
	if poolReport.PoolID != "pool-1" {
		t.Errorf("expected pool ID pool-1, got %s", poolReport.PoolID)
	}
	if poolReport.Utilization != 50.0 {
		t.Errorf("expected utilization 50%%, got %f", poolReport.Utilization)
	}
}

func TestGetMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	// 添加测试数据
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		CostPerGB:  0.2,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	metrics := g.GetMetrics()

	if metrics.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if len(metrics.PoolMetrics) != 1 {
		t.Errorf("expected 1 pool metric, got %d", len(metrics.PoolMetrics))
	}
	if metrics.ForecastAccuracy != 0.85 {
		t.Errorf("expected forecast accuracy 0.85, got %f", metrics.ForecastAccuracy)
	}

	poolMetric := metrics.PoolMetrics["pool-1"]
	if poolMetric.Utilization != 0.5 {
		t.Errorf("expected utilization 0.5, got %f", poolMetric.Utilization)
	}
	if poolMetric.CostPerGB != 0.2 {
		t.Errorf("expected cost per GB 0.2, got %f", poolMetric.CostPerGB)
	}
}

func TestStartMonitoring(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    100 * time.Millisecond, // 短周期用于测试
	}

	g, _ := NewCostGovernor(config, logger)

	// 添加测试数据
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024,
		UsedBytes:  512 * 1024 * 1024 * 1024,
		CostPerGB:  0.2,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	g.StartMonitoring()

	// 等待一个监控周期
	time.Sleep(200 * time.Millisecond)

	// 停止监控
	g.Stop()

	// 验证指标被更新
	if g.metrics == nil {
		t.Error("expected metrics to be updated")
	}
}

func TestConcurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    24 * time.Hour,
	}

	g, _ := NewCostGovernor(config, logger)

	// 注册池
	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		Type:       PoolTypeLocal,
		TotalBytes: 1024 * 1024 * 1024 * 1024,
		UsedBytes:  512 * 1024 * 1024 * 1024,
		CostPerGB:  0.2,
		Tier:       TierHot,
	}
	g.RegisterPool(pool)

	// 并发测试
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			g.UpdateUsage("pool-1", 600*1024*1024*1024)
			g.GetMetrics()
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// 辅助函数测试
func TestEnumStringMethods(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{ String() string }
		expected string
	}{
		{"PoolTypeLocal", PoolTypeLocal, "local"},
		{"PoolTypeNAS", PoolTypeNAS, "nas"},
		{"PoolTypeCloud", PoolTypeCloud, "cloud"},
		{"PoolTypeHybrid", PoolTypeHybrid, "hybrid"},
		{"TierHot", TierHot, "hot"},
		{"TierWarm", TierWarm, "warm"},
		{"TierCold", TierCold, "cold"},
		{"TierArchive", TierArchive, "archive"},
		{"AlertInfo", AlertInfo, "info"},
		{"AlertWarning", AlertWarning, "warning"},
		{"AlertCritical", AlertCritical, "critical"},
		{"PriorityLow", PriorityLow, "low"},
		{"PriorityMedium", PriorityMedium, "medium"},
		{"PriorityHigh", PriorityHigh, "high"},
		{"PriorityCritical", PriorityCritical, "critical"},
		{"EffortLow", EffortLow, "low"},
		{"EffortMedium", EffortMedium, "medium"},
		{"EffortHigh", EffortHigh, "high"},
		{"ImpactLow", ImpactLow, "low"},
		{"ImpactMedium", ImpactMedium, "medium"},
		{"ImpactHigh", ImpactHigh, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestCalculateGrowthRate(t *testing.T) {
	// 线性增长数据
	history := []UsageRecord{
		{Timestamp: time.Now().Add(-6 * 24 * time.Hour), UsedBytes: 100 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-5 * 24 * time.Hour), UsedBytes: 110 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-4 * 24 * time.Hour), UsedBytes: 120 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-3 * 24 * time.Hour), UsedBytes: 130 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-2 * 24 * time.Hour), UsedBytes: 140 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-1 * 24 * time.Hour), UsedBytes: 150 * 1024 * 1024 * 1024},
		{Timestamp: time.Now(), UsedBytes: 160 * 1024 * 1024 * 1024},
	}

	rate := calculateGrowthRate(history)
	if rate <= 0 {
		t.Errorf("expected positive growth rate, got %f", rate)
	}
}

func TestCalculateConfidence(t *testing.T) {
	// 高质量数据（线性增长，30个点）
	goodHistory := make([]UsageRecord, 30)
	for i := 0; i < 30; i++ {
		goodHistory[i] = UsageRecord{
			Timestamp: time.Now().Add(-time.Duration(30-i) * 24 * time.Hour),
			UsedBytes: int64(100+i*10) * 1024 * 1024 * 1024,
		}
	}

	rate := calculateGrowthRate(goodHistory)
	confidence := calculateConfidence(goodHistory, rate)
	if confidence < 0.5 {
		t.Errorf("expected high confidence (>0.5), got %f", confidence)
	}

	// 低质量数据（少量数据点）
	badHistory := []UsageRecord{
		{Timestamp: time.Now().Add(-6 * 24 * time.Hour), UsedBytes: 100 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-5 * 24 * time.Hour), UsedBytes: 150 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-4 * 24 * time.Hour), UsedBytes: 80 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-3 * 24 * time.Hour), UsedBytes: 200 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-2 * 24 * time.Hour), UsedBytes: 90 * 1024 * 1024 * 1024},
		{Timestamp: time.Now().Add(-1 * 24 * time.Hour), UsedBytes: 180 * 1024 * 1024 * 1024},
		{Timestamp: time.Now(), UsedBytes: 100 * 1024 * 1024 * 1024},
	}

	badRate := calculateGrowthRate(badHistory)
	badConfidence := calculateConfidence(badHistory, badRate)
	if badConfidence >= confidence {
		t.Errorf("expected lower confidence for irregular data")
	}
}

func TestCalculateSavingsForResize(t *testing.T) {
	pool := &StoragePool{
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  300 * 1024 * 1024 * 1024,  // 300GB
		CostPerGB:  0.2,
	}

	savings := calculateSavingsForResize(pool, 80) // 目标80%利用率
	if savings <= 0 {
		t.Error("expected positive savings")
	}
}

func TestStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &GovernorConfig{
		ForecastWindow: 90 * 24 * time.Hour,
		Currency:       "CNY",
		ReviewCycle:    100 * time.Millisecond,
	}

	g, _ := NewCostGovernor(config, logger)
	g.StartMonitoring()
	time.Sleep(50 * time.Millisecond)
	g.Stop()

	// 验证上下文被取消
	select {
	case <-g.ctx.Done():
		// 期望的行为
	case <-time.After(100 * time.Millisecond):
		t.Error("expected context to be cancelled")
	}
}
