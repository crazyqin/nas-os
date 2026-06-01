package smartbandwidthpredict

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewEngine(t *testing.T) {
	config := &Config{
		TotalBandwidthMbps: 1000,
		CollectInterval:    30 * time.Second,
		PredictionWindow:   100,
		PredictionHorizon:  30,
		AnomalyThreshold:   2.0,
		SmoothingAlpha:     0.3,
		MaxSamples:         10000,
		Interfaces:         []string{"eth0"},
		Enabled:            true,
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)

	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.config.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected TotalBandwidthMbps 1000, got %f", engine.config.TotalBandwidthMbps)
	}

	if !engine.config.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

func TestNewEngineWithNilConfig(t *testing.T) {
	engine, err := NewEngine(nil, nil)

	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.config.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected default TotalBandwidthMbps 1000, got %f", engine.config.TotalBandwidthMbps)
	}
}

func TestEngineStartStop(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	// 启动引擎
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !engine.IsRunning() {
		t.Error("Expected engine to be running")
	}

	// 重复启动应失败
	err = engine.Start()
	if err == nil {
		t.Error("Expected error on duplicate start")
	}

	// 停止引擎
	engine.Stop()

	if engine.IsRunning() {
		t.Error("Expected engine to be stopped")
	}
}

func TestRecordTraffic(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	sample := &TrafficSample{
		Timestamp:    time.Now(),
		InboundMbps:  100.5,
		OutboundMbps: 50.3,
		LatencyMs:    10.5,
		PacketLoss:   0.1,
		Interface:    "eth0",
	}

	err := engine.RecordTraffic(sample)
	if err != nil {
		t.Fatalf("RecordTraffic failed: %v", err)
	}

	// 测试空采样
	err = engine.RecordTraffic(nil)
	if err == nil {
		t.Error("Expected error for nil sample")
	}
}

func TestPredictBandwidth(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	// 添加足够的采样数据
	for i := 0; i < 100; i++ {
		sample := &TrafficSample{
			Timestamp:    time.Now().Add(-time.Duration(100-i) * time.Minute),
			InboundMbps:  float64(100 + i),
			OutboundMbps: float64(50 + i/2),
			Interface:    "eth0",
		}
		engine.RecordTraffic(sample)
	}

	// 预测带宽
	prediction, err := engine.PredictBandwidth(30)
	if err != nil {
		t.Fatalf("PredictBandwidth failed: %v", err)
	}

	if prediction == nil {
		t.Fatal("PredictBandwidth returned nil")
	}

	if prediction.PredictedMbps <= 0 {
		t.Error("Expected positive predicted Mbps")
	}

	if prediction.Confidence < 0 || prediction.Confidence > 1 {
		t.Errorf("Expected confidence between 0 and 1, got %f", prediction.Confidence)
	}
}

func TestPredictBandwidthInsufficientData(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	// 只添加少量采样
	for i := 0; i < 5; i++ {
		sample := &TrafficSample{
			Timestamp:    time.Now(),
			InboundMbps:  100,
			OutboundMbps: 50,
			Interface:    "eth0",
		}
		engine.RecordTraffic(sample)
	}

	// 应该失败
	_, err := engine.PredictBandwidth(30)
	if err == nil {
		t.Error("Expected error for insufficient data")
	}
}

func TestCreateSchedule(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	// 添加采样数据
	for i := 0; i < 100; i++ {
		sample := &TrafficSample{
			Timestamp:    time.Now().Add(-time.Duration(100-i) * time.Minute),
			InboundMbps:  float64(100 + i),
			OutboundMbps: float64(50 + i/2),
			Interface:    "eth0",
		}
		engine.RecordTraffic(sample)
	}

	tasks := []*ScheduleTask{
		{
			ID:           "task1",
			Name:         "备份任务",
			Priority:     5,
			RequiredMbps: 50,
			Duration:     10 * time.Minute,
		},
		{
			ID:           "task2",
			Name:         "下载任务",
			Priority:     8,
			RequiredMbps: 100,
			Duration:     5 * time.Minute,
		},
	}

	plan, err := engine.CreateSchedule(tasks)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	if plan == nil {
		t.Fatal("CreateSchedule returned nil")
	}

	if len(plan.Tasks) == 0 {
		t.Error("Expected at least one scheduled task")
	}
}

func TestCreateScheduleEmptyTasks(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	_, err := engine.CreateSchedule(nil)
	if err == nil {
		t.Error("Expected error for nil tasks")
	}

	_, err = engine.CreateSchedule([]*ScheduleTask{})
	if err == nil {
		t.Error("Expected error for empty tasks")
	}
}

func TestApplyQoS(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	policy := &QoSPolicy{
		Name:     "视频流策略",
		MinMbps:  50,
		MaxMbps:  200,
		Priority: 8,
	}

	err := engine.ApplyQoS(policy)
	if err != nil {
		t.Fatalf("ApplyQoS failed: %v", err)
	}

	if policy.ID == "" {
		t.Error("Expected policy ID to be set")
	}

	if !policy.Enabled {
		t.Error("Expected policy to be enabled")
	}

	// 测试无效策略
	invalidPolicy := &QoSPolicy{
		Name:     "无效策略",
		MinMbps:  200,
		MaxMbps:  100,
		Priority: 5,
	}

	err = engine.ApplyQoS(invalidPolicy)
	if err == nil {
		t.Error("Expected error for invalid policy")
	}
}

func TestApplyQoSValidation(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	tests := []struct {
		name    string
		policy  *QoSPolicy
		wantErr bool
	}{
		{
			name:    "nil policy",
			policy:  nil,
			wantErr: true,
		},
		{
			name: "empty name",
			policy: &QoSPolicy{
				MinMbps:  10,
				MaxMbps:  100,
				Priority: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			policy: &QoSPolicy{
				Name:     "Test",
				MinMbps:  10,
				MaxMbps:  100,
				Priority: 11,
			},
			wantErr: true,
		},
		{
			name: "min > max",
			policy: &QoSPolicy{
				Name:     "Test",
				MinMbps:  200,
				MaxMbps:  100,
				Priority: 5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ApplyQoS(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyQoS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetPredictions(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	predictions := engine.GetPredictions()
	if predictions == nil {
		t.Error("Expected non-nil predictions slice")
	}

	if len(predictions) != 0 {
		t.Error("Expected empty predictions")
	}
}

func TestGetSchedules(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	schedules := engine.GetSchedules()
	if schedules == nil {
		t.Error("Expected non-nil schedules slice")
	}

	if len(schedules) != 0 {
		t.Error("Expected empty schedules")
	}
}

func TestGetQoSPolicies(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	policies := engine.GetQoSPolicies()
	if policies == nil {
		t.Error("Expected non-nil policies map")
	}

	if len(policies) != 0 {
		t.Error("Expected empty policies")
	}
}

func TestUpdateConfig(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	newConfig := &Config{
		TotalBandwidthMbps: 2000,
		CollectInterval:    60 * time.Second,
		PredictionWindow:   200,
		PredictionHorizon:  60,
		AnomalyThreshold:   3.0,
		SmoothingAlpha:     0.5,
		MaxSamples:         20000,
		Interfaces:         []string{"eth0", "eth1"},
		Enabled:            true,
	}

	err := engine.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	config := engine.GetConfig()
	if config.TotalBandwidthMbps != 2000 {
		t.Errorf("Expected TotalBandwidthMbps 2000, got %f", config.TotalBandwidthMbps)
	}
}

func TestUpdateConfigInvalid(t *testing.T) {
	engine, _ := NewEngine(nil, nil)

	// 测试无效配置
	invalidConfig := &Config{
		TotalBandwidthMbps: -100,
		CollectInterval:    30 * time.Second,
		PredictionWindow:   100,
		PredictionHorizon:  30,
		AnomalyThreshold:   2.0,
		SmoothingAlpha:     0.3,
		MaxSamples:         10000,
		Interfaces:         []string{"eth0"},
		Enabled:            true,
	}

	err := engine.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestCollectorRecord(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	collector := NewCollector(config, logger)

	sample := &TrafficSample{
		Timestamp:    time.Now(),
		InboundMbps:  100,
		OutboundMbps: 50,
		Interface:    "eth0",
	}

	err := collector.Record(sample)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if collector.GetSampleCount() != 1 {
		t.Errorf("Expected 1 sample, got %d", collector.GetSampleCount())
	}
}

func TestCollectorGetRecentSamples(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	collector := NewCollector(config, logger)

	// 添加多个采样
	for i := 0; i < 10; i++ {
		sample := &TrafficSample{
			Timestamp:    time.Now().Add(-time.Duration(10-i) * time.Minute),
			InboundMbps:  float64(i * 10),
			OutboundMbps: float64(i * 5),
			Interface:    "eth0",
		}
		collector.Record(sample)
	}

	// 获取最近5个
	recent := collector.GetRecentSamples(5)
	if len(recent) != 5 {
		t.Errorf("Expected 5 samples, got %d", len(recent))
	}
}

func TestCollectorGetBandwidthStats(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	collector := NewCollector(config, logger)

	// 添加采样
	for i := 0; i < 5; i++ {
		sample := &TrafficSample{
			Timestamp:    time.Now(),
			InboundMbps:  100 + float64(i*10),
			OutboundMbps: 50 + float64(i*5),
			LatencyMs:    10,
			PacketLoss:   0.1,
			Interface:    "eth0",
		}
		collector.Record(sample)
	}

	stats := collector.GetBandwidthStats()
	if stats.SampleCount != 5 {
		t.Errorf("Expected 5 samples, got %d", stats.SampleCount)
	}

	if stats.AvgInboundMbps <= 0 {
		t.Error("Expected positive avg inbound Mbps")
	}
}

func TestPredictorPredict(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	predictor := NewPredictor(config, logger)

	// 创建采样数据
	samples := make([]*TrafficSample, 100)
	for i := 0; i < 100; i++ {
		samples[i] = &TrafficSample{
			Timestamp:    time.Now().Add(-time.Duration(100-i) * time.Minute),
			InboundMbps:  float64(100 + i),
			OutboundMbps: float64(50 + i/2),
		}
	}

	prediction, err := predictor.Predict(samples, 30)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if prediction.PredictedMbps <= 0 {
		t.Error("Expected positive predicted Mbps")
	}
}

func TestPredictorIsAnomaly(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	predictor := NewPredictor(config, logger)

	// 正常数据
	normalSamples := make([]*TrafficSample, 20)
	for i := 0; i < 20; i++ {
		normalSamples[i] = &TrafficSample{
			Timestamp:    time.Now(),
			InboundMbps:  100 + float64(i%5),
			OutboundMbps: 50 + float64(i%3),
		}
	}

	if predictor.IsAnomaly(normalSamples) {
		t.Error("Expected no anomaly for normal data")
	}

	// 异常数据
	anomalySamples := make([]*TrafficSample, 20)
	for i := 0; i < 19; i++ {
		anomalySamples[i] = &TrafficSample{
			Timestamp:    time.Now(),
			InboundMbps:  100,
			OutboundMbps: 50,
		}
	}
	anomalySamples[19] = &TrafficSample{
		Timestamp:    time.Now(),
		InboundMbps:  1000,
		OutboundMbps: 500,
	}

	if !predictor.IsAnomaly(anomalySamples) {
		t.Error("Expected anomaly for outlier data")
	}
}

func TestSchedulerCreatePlan(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	scheduler := NewScheduler(config, logger)

	tasks := []*ScheduleTask{
		{
			ID:           "task1",
			Name:         "备份",
			Priority:     5,
			RequiredMbps: 50,
			Duration:     10 * time.Minute,
		},
		{
			ID:           "task2",
			Name:         "下载",
			Priority:     8,
			RequiredMbps: 100,
			Duration:     5 * time.Minute,
		},
	}

	prediction := &BandwidthPrediction{
		PredictedMbps: 500,
		Confidence:    0.8,
		Trend:         TrendStable,
	}

	policies := make(map[string]*QoSPolicy)

	plan, err := scheduler.CreatePlan(tasks, prediction, policies)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("CreatePlan returned nil")
	}

	if len(plan.Tasks) == 0 {
		t.Error("Expected at least one scheduled task")
	}
}

func TestSchedulerOptimizeForPeakHours(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	scheduler := NewScheduler(config, logger)

	tasks := []*ScheduleTask{
		{
			ID:           "task1",
			Name:         "备份",
			Priority:     5,
			RequiredMbps: 100,
			Duration:     10 * time.Minute,
		},
	}

	peakHours := []int{time.Now().Hour()}

	optimized := scheduler.OptimizeForPeakHours(tasks, peakHours, nil)
	if len(optimized) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(optimized))
	}

	// 高峰时段应该降低带宽
	if optimized[0].RequiredMbps >= 100 {
		t.Error("Expected reduced bandwidth during peak hours")
	}
}

func TestSchedulerEstimateCompletionTime(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	scheduler := NewScheduler(config, logger)

	tasks := []*ScheduleTask{
		{
			ID:           "task1",
			Name:         "任务1",
			Priority:     5,
			RequiredMbps: 50,
			Duration:     10 * time.Minute,
		},
		{
			ID:           "task2",
			Name:         "任务2",
			Priority:     8,
			RequiredMbps: 100,
			Duration:     5 * time.Minute,
		},
	}

	duration := scheduler.EstimateCompletionTime(tasks, 200)
	if duration <= 0 {
		t.Error("Expected positive duration")
	}
}

func TestSchedulerGetPlanStats(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()
	scheduler := NewScheduler(config, logger)

	stats := scheduler.GetPlanStats()
	if stats.TotalPlans != 0 {
		t.Errorf("Expected 0 plans, got %d", stats.TotalPlans)
	}

	// 添加计划
	tasks := []*ScheduleTask{
		{
			ID:           "task1",
			Name:         "备份",
			Priority:     5,
			RequiredMbps: 50,
			Duration:     10 * time.Minute,
		},
	}

	prediction := &BandwidthPrediction{
		PredictedMbps: 500,
		Confidence:    0.8,
		Trend:         TrendStable,
	}

	scheduler.CreatePlan(tasks, prediction, make(map[string]*QoSPolicy))

	stats = scheduler.GetPlanStats()
	if stats.TotalPlans != 1 {
		t.Errorf("Expected 1 plan, got %d", stats.TotalPlans)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected TotalBandwidthMbps 1000, got %f", config.TotalBandwidthMbps)
	}

	if config.CollectInterval != 30*time.Second {
		t.Errorf("Expected CollectInterval 30s, got %v", config.CollectInterval)
	}

	if config.PredictionWindow != 100 {
		t.Errorf("Expected PredictionWindow 100, got %d", config.PredictionWindow)
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "negative bandwidth",
			config: &Config{
				TotalBandwidthMbps: -100,
				CollectInterval:    30 * time.Second,
				PredictionWindow:   100,
				PredictionHorizon:  30,
				AnomalyThreshold:   2.0,
				SmoothingAlpha:     0.3,
				MaxSamples:         10000,
				Interfaces:         []string{"eth0"},
			},
			wantErr: true,
		},
		{
			name: "small prediction window",
			config: &Config{
				TotalBandwidthMbps: 1000,
				CollectInterval:    30 * time.Second,
				PredictionWindow:   5,
				PredictionHorizon:  30,
				AnomalyThreshold:   2.0,
				SmoothingAlpha:     0.3,
				MaxSamples:         10000,
				Interfaces:         []string{"eth0"},
			},
			wantErr: true,
		},
		{
			name: "invalid alpha",
			config: &Config{
				TotalBandwidthMbps: 1000,
				CollectInterval:    30 * time.Second,
				PredictionWindow:   100,
				PredictionHorizon:  30,
				AnomalyThreshold:   2.0,
				SmoothingAlpha:     1.5,
				MaxSamples:         10000,
				Interfaces:         []string{"eth0"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrendTypes(t *testing.T) {
	if TrendRising != "rising" {
		t.Errorf("Expected TrendRising to be 'rising', got '%s'", TrendRising)
	}

	if TrendFalling != "falling" {
		t.Errorf("Expected TrendFalling to be 'falling', got '%s'", TrendFalling)
	}

	if TrendStable != "stable" {
		t.Errorf("Expected TrendStable to be 'stable', got '%s'", TrendStable)
	}
}
