package optimizer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewOptimizationEngine(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()

	engine := NewOptimizationEngine(logger, &config)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.metrics)
	assert.NotNil(t, engine.history)
	assert.NotNil(t, engine.autoTuner)
	assert.NotNil(t, engine.predictor)
	assert.NotNil(t, engine.detector)
	assert.NotNil(t, engine.advisor)
	assert.NotNil(t, engine.scheduler)
	assert.False(t, engine.IsRunning())
}

func TestOptimizationEngine_StartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()

	engine := NewOptimizationEngine(logger, &config)
	ctx := context.Background()

	// 测试启动
	err := engine.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// 测试重复启动
	err = engine.Start(ctx)
	assert.NoError(t, err)

	// 测试停止
	engine.Stop()
	assert.False(t, engine.IsRunning())
}

func TestMetricsCollector_Collect(t *testing.T) {
	collector := NewMetricsCollector()

	metrics := collector.Collect()

	assert.NotNil(t, metrics)
	assert.False(t, metrics.Timestamp.IsZero())
	assert.GreaterOrEqual(t, metrics.MemPercent, 0.0)
	assert.LessOrEqual(t, metrics.MemPercent, 100.0)
}

func TestResourcePredictor_Predict(t *testing.T) {
	logger := zap.NewNop()
	predictor := NewResourcePredictor(logger, 100)

	// 添加足够的数据点
	for i := 0; i < 20; i++ {
		metrics := &ResourceMetrics{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			CPUPercent: float64(50 + i),
			MemPercent: float64(60 + i),
		}
		predictor.AddMetrics(metrics)
	}

	predictions := predictor.Predict()

	assert.NotNil(t, predictions)
	assert.Len(t, predictions, 4) // cpu, memory, disk, network

	for _, pred := range predictions {
		assert.NotEmpty(t, pred.Resource)
		assert.GreaterOrEqual(t, pred.Confidence, 0.0)
		assert.LessOrEqual(t, pred.Confidence, 100.0)
	}
}

func TestBottleneckDetector_Detect(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()
	detector := NewBottleneckDetector(logger, &config)

	// 测试正常指标
	normalMetrics := &ResourceMetrics{
		CPUPercent: 50.0,
		MemPercent: 60.0,
		LoadAvg1:   1.0,
	}
	bottlenecks := detector.Detect(normalMetrics)
	assert.Len(t, bottlenecks, 0)

	// 测试高 CPU
	highCPUMetrics := &ResourceMetrics{
		CPUPercent: 90.0,
		MemPercent: 60.0,
		LoadAvg1:   1.0,
	}
	bottlenecks = detector.Detect(highCPUMetrics)
	assert.Len(t, bottlenecks, 1)
	assert.Equal(t, "cpu", bottlenecks[0].Type)
	assert.Equal(t, "warning", bottlenecks[0].Severity)

	// 测试临界 CPU
	criticalCPUMetrics := &ResourceMetrics{
		CPUPercent: 96.0,
		MemPercent: 60.0,
		LoadAvg1:   1.0,
	}
	bottlenecks = detector.Detect(criticalCPUMetrics)
	assert.Len(t, bottlenecks, 1)
	assert.Equal(t, "critical", bottlenecks[0].Severity)

	// 测试高内存
	highMemMetrics := &ResourceMetrics{
		CPUPercent: 50.0,
		MemPercent: 90.0,
		LoadAvg1:   1.0,
	}
	bottlenecks = detector.Detect(highMemMetrics)
	assert.Len(t, bottlenecks, 1)
	assert.Equal(t, "memory", bottlenecks[0].Type)
}

func TestOptimizationAdvisor_GenerateSuggestions(t *testing.T) {
	logger := zap.NewNop()
	advisor := NewOptimizationAdvisor(logger)

	metrics := &ResourceMetrics{
		CPUPercent: 50.0,
		MemPercent: 60.0,
	}

	// 测试无瓶颈时的建议
	bottlenecks := []*Bottleneck{}
	suggestions := advisor.GenerateSuggestions(metrics, bottlenecks)
	assert.NotNil(t, suggestions)

	// 测试有瓶颈时的建议
	bottlenecks = []*Bottleneck{
		{
			Type:     "cpu",
			Severity: "warning",
		},
	}
	suggestions = advisor.GenerateSuggestions(metrics, bottlenecks)
	assert.NotNil(t, suggestions)
	assert.Greater(t, len(suggestions), 0)
}

func TestOptimizationHistory(t *testing.T) {
	history := NewOptimizationHistory(100)

	// 测试添加记录
	record := &OptimizationRecord{
		ID:        "test-1",
		Type:      "auto",
		Category:  "cpu",
		Status:    "success",
		ExecutedAt: time.Now(),
	}
	history.Add(record)
	assert.Equal(t, 1, history.Count())

	// 测试获取所有记录
	records := history.GetAll()
	assert.Len(t, records, 1)
	assert.Equal(t, "test-1", records[0].ID)

	// 测试按类型获取
	autoRecords := history.GetByType("auto")
	assert.Len(t, autoRecords, 1)

	manualRecords := history.GetByType("manual")
	assert.Len(t, manualRecords, 0)

	// 测试按类别获取
	cpuRecords := history.GetByCategory("cpu")
	assert.Len(t, cpuRecords, 1)

	// 测试获取最近记录
	recent := history.GetRecent(10)
	assert.Len(t, recent, 1)

	// 测试清空历史
	history.Clear()
	assert.Equal(t, 0, history.Count())
}

func TestScheduledOptimizer(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()
	engine := NewOptimizationEngine(logger, &config)

	scheduler := engine.GetScheduler()
	assert.NotNil(t, scheduler)

	// 测试添加任务
	task := &ScheduledTask{
		ID:       "task-1",
		Name:     "Test Task",
		CronExpr: "0 * * * *",
		Category: "cpu",
		Actions:  []string{"optimize"},
		Enabled:  true,
	}
	scheduler.AddTask(task)

	tasks := scheduler.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "task-1", tasks[0].ID)

	// 测试移除任务
	scheduler.RemoveTask("task-1")
	tasks = scheduler.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestAutoTuner_Tune(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()
	engine := NewOptimizationEngine(logger, &config)

	autoTuner := engine.autoTuner
	ctx := context.Background()

	// 测试正常指标（不应触发调优）
	normalMetrics := &ResourceMetrics{
		CPUPercent: 50.0,
		MemPercent: 60.0,
	}
	records := autoTuner.Tune(ctx, normalMetrics)
	assert.Len(t, records, 0)

	// 测试高 CPU（应触发调优）
	highCPUMetrics := &ResourceMetrics{
		CPUPercent: 90.0,
		MemPercent: 60.0,
	}
	records = autoTuner.Tune(ctx, highCPUMetrics)
	assert.Len(t, records, 1)
	assert.Equal(t, "cpu", records[0].Category)

	// 测试高内存（应触发调优）
	highMemMetrics := &ResourceMetrics{
		CPUPercent: 50.0,
		MemPercent: 90.0,
	}
	records = autoTuner.Tune(ctx, highMemMetrics)
	assert.Len(t, records, 1)
	assert.Equal(t, "memory", records[0].Category)
}

func TestDefaultAutoTuneConfig(t *testing.T) {
	config := DefaultAutoTuneConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 80.0, config.CPUThreshold)
	assert.Equal(t, 85.0, config.MemThreshold)
	assert.Equal(t, 70.0, config.IOThreshold)
	assert.Equal(t, 300, config.TuneInterval)
	assert.Equal(t, 3, config.MaxConcurrent)
	assert.False(t, config.DryRun)
	assert.True(t, config.AutoApply)
	assert.True(t, config.NotifyOnTune)
}

func TestEngineStats(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultAutoTuneConfig()
	engine := NewOptimizationEngine(logger, &config)

	stats := engine.GetStats()

	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalOptimizations)
	assert.Equal(t, int64(0), stats.SuccessfulTunes)
	assert.Equal(t, int64(0), stats.FailedTunes)
	assert.Equal(t, 0.0, stats.TotalImprovement)
	assert.Equal(t, 0.0, stats.AvgImprovement)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	// 注意：由于时间精度，ID 可能相同，这里只测试非空
}
