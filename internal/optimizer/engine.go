package optimizer

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
)

// OptimizationEngine 优化引擎核心
type OptimizationEngine struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	config         *AutoTuneConfig
	history        *OptimizationHistory
	metrics        *MetricsCollector
	autoTuner      *AutoTuner
	predictor      *ResourcePredictor
	detector       *BottleneckDetector
	advisor        *OptimizationAdvisor
	scheduler      *ScheduledOptimizer
	stats          *EngineStats
	running        bool
	cancel         context.CancelFunc
	startTime      time.Time
	metricsHistory []*ResourceMetrics
	maxHistorySize int
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics *ResourceMetrics
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// Collect 收集当前系统指标
func (mc *MetricsCollector) Collect() *ResourceMetrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := &ResourceMetrics{
		Timestamp:  time.Now(),
		MemUsedMB:  float64(memStats.Alloc) / 1024 / 1024,
		MemTotalMB: float64(memStats.Sys) / 1024 / 1024,
		MemPercent: float64(memStats.Alloc) / float64(memStats.Sys) * 100,
	}

	// 注意：实际的 CPU 和 IO 指标需要通过系统调用获取
	// 这里使用模拟数据，实际实现需要调用 /proc/stat 等
	metrics.CPUPercent = mc.collectCPU()
	metrics.LoadAvg1, metrics.LoadAvg5, metrics.LoadAvg15 = mc.collectLoadAvg()

	mc.metrics = metrics
	return metrics
}

// collectCPU 收集 CPU 使用率
func (mc *MetricsCollector) collectCPU() float64 {
	// 实际实现需要读取 /proc/stat 或使用 gopsutil
	// 这里返回模拟值
	return float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 10
}

// collectLoadAvg 收集系统负载
func (mc *MetricsCollector) collectLoadAvg() (float64, float64, float64) {
	// 实际实现需要读取 /proc/loadavg
	// 这里返回模拟值
	goroutines := float64(runtime.NumGoroutine())
	return goroutines * 0.1, goroutines * 0.08, goroutines * 0.05
}

// AutoTuner 自动性能调优器
type AutoTuner struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *AutoTuneConfig
	engine *OptimizationEngine
}

// NewAutoTuner 创建自动调优器
func NewAutoTuner(logger *zap.Logger, config *AutoTuneConfig, engine *OptimizationEngine) *AutoTuner {
	return &AutoTuner{
		logger: logger,
		config: config,
		engine: engine,
	}
}

// Tune 执行自动调优
func (at *AutoTuner) Tune(ctx context.Context, metrics *ResourceMetrics) []*OptimizationRecord {
	at.mu.RLock()
	defer at.mu.RUnlock()

	if !at.config.Enabled {
		return nil
	}

	var records []*OptimizationRecord

	// CPU 调优
	if metrics.CPUPercent > at.config.CPUThreshold {
		record := at.tuneCPU(ctx, metrics)
		if record != nil {
			records = append(records, record)
		}
	}

	// 内存调优
	if metrics.MemPercent > at.config.MemThreshold {
		record := at.tuneMemory(ctx, metrics)
		if record != nil {
			records = append(records, record)
		}
	}

	// IO 调优
	if metrics.DiskReadKB+metrics.DiskWriteKB > at.config.IOThreshold*1024 {
		record := at.tuneIO(ctx, metrics)
		if record != nil {
			records = append(records, record)
		}
	}

	return records
}

// tuneCPU CPU 调优
func (at *AutoTuner) tuneCPU(ctx context.Context, metrics *ResourceMetrics) *OptimizationRecord {
	start := time.Now()

	record := &OptimizationRecord{
		ID:            fmt.Sprintf("cpu-%d", time.Now().UnixNano()),
		Type:          "auto",
		Category:      "cpu",
		Action:        "cpu_optimization",
		BeforeMetrics: metrics,
		ExecutedAt:    start,
		ExecutedBy:    "system",
		Status:        "success",
	}

	// 实际的 CPU 优化逻辑
	if !at.config.DryRun {
		// 1. 调整 GOMAXPROCS
		// 2. 优化 goroutine 池
		// 3. 调整调度策略
		at.logger.Info("执行 CPU 调优",
			zap.Float64("cpu_percent", metrics.CPUPercent),
			zap.Float64("threshold", at.config.CPUThreshold))
	}

	record.Duration = time.Since(start)
	record.AfterMetrics = at.engine.metrics.Collect()

	// 计算性能提升
	if record.BeforeMetrics != nil && record.AfterMetrics != nil {
		record.Improvement = record.BeforeMetrics.CPUPercent - record.AfterMetrics.CPUPercent
	}

	return record
}

// tuneMemory 内存调优
func (at *AutoTuner) tuneMemory(ctx context.Context, metrics *ResourceMetrics) *OptimizationRecord {
	start := time.Now()

	record := &OptimizationRecord{
		ID:            fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		Type:          "auto",
		Category:      "memory",
		Action:        "memory_optimization",
		BeforeMetrics: metrics,
		ExecutedAt:    start,
		ExecutedBy:    "system",
		Status:        "success",
	}

	// 实际的内存优化逻辑
	if !at.config.DryRun {
		// 1. 触发 GC
		// 2. 清理缓存
		// 3. 调整内存分配策略
		runtime.GC()
		at.logger.Info("执行内存调优",
			zap.Float64("mem_percent", metrics.MemPercent),
			zap.Float64("threshold", at.config.MemThreshold))
	}

	record.Duration = time.Since(start)
	record.AfterMetrics = at.engine.metrics.Collect()

	if record.BeforeMetrics != nil && record.AfterMetrics != nil {
		record.Improvement = record.BeforeMetrics.MemPercent - record.AfterMetrics.MemPercent
	}

	return record
}

// tuneIO IO 调优
func (at *AutoTuner) tuneIO(ctx context.Context, metrics *ResourceMetrics) *OptimizationRecord {
	start := time.Now()

	record := &OptimizationRecord{
		ID:            fmt.Sprintf("io-%d", time.Now().UnixNano()),
		Type:          "auto",
		Category:      "io",
		Action:        "io_optimization",
		BeforeMetrics: metrics,
		ExecutedAt:    start,
		ExecutedBy:    "system",
		Status:        "success",
	}

	// 实际的 IO 优化逻辑
	if !at.config.DryRun {
		// 1. 调整 IO 调度器
		// 2. 优化读写缓冲
		// 3. 调整预读策略
		at.logger.Info("执行 IO 调优",
			zap.Float64("disk_read_kb", metrics.DiskReadKB),
			zap.Float64("disk_write_kb", metrics.DiskWriteKB))
	}

	record.Duration = time.Since(start)
	record.AfterMetrics = at.engine.metrics.Collect()

	return record
}

// ResourcePredictor 资源预测器
type ResourcePredictor struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	history []*ResourceMetrics
	maxSize int
}

// NewResourcePredictor 创建资源预测器
func NewResourcePredictor(logger *zap.Logger, maxSize int) *ResourcePredictor {
	return &ResourcePredictor{
		logger:  logger,
		history: make([]*ResourceMetrics, 0),
		maxSize: maxSize,
	}
}

// AddMetrics 添加指标数据
func (rp *ResourcePredictor) AddMetrics(metrics *ResourceMetrics) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.history = append(rp.history, metrics)
	if len(rp.history) > rp.maxSize {
		rp.history = rp.history[len(rp.history)-rp.maxSize:]
	}
}

// Predict 预测资源使用
func (rp *ResourcePredictor) Predict() []*PredictionResult {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	if len(rp.history) < 10 {
		return nil
	}

	var results []*PredictionResult

	// 预测 CPU
	cpuResult := rp.predictResource("cpu", rp.history, func(m *ResourceMetrics) float64 {
		return m.CPUPercent
	})
	results = append(results, cpuResult)

	// 预测内存
	memResult := rp.predictResource("memory", rp.history, func(m *ResourceMetrics) float64 {
		return m.MemPercent
	})
	results = append(results, memResult)

	// 预测磁盘 IO
	diskResult := rp.predictResource("disk", rp.history, func(m *ResourceMetrics) float64 {
		return m.DiskReadKB + m.DiskWriteKB
	})
	results = append(results, diskResult)

	// 预测网络
	netResult := rp.predictResource("network", rp.history, func(m *ResourceMetrics) float64 {
		return m.NetworkInKB + m.NetworkOutKB
	})
	results = append(results, netResult)

	return results
}

// predictResource 预测单个资源
func (rp *ResourcePredictor) predictResource(resource string, history []*ResourceMetrics, extractor func(*ResourceMetrics) float64) *PredictionResult {
	if len(history) < 2 {
		return &PredictionResult{
			Resource:     resource,
			CurrentValue: extractor(history[len(history)-1]),
			Trend:        "stable",
			Confidence:   0,
		}
	}

	// 提取最近的数据点
	recentSize := int(math.Min(float64(len(history)), 60))
	recent := history[len(history)-recentSize:]

	// 计算线性回归
	var sumX, sumY, sumXY, sumX2 float64
	for i, m := range recent {
		x := float64(i)
		y := extractor(m)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	n := float64(len(recent))
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	currentValue := extractor(recent[len(recent)-1])

	// 预测未来值（假设每个数据点间隔 1 分钟）
	predicted1H := intercept + slope*(n+60)
	predicted6H := intercept + slope*(n+360)
	predicted24H := intercept + slope*(n+1440)

	// 限制在 0-100 范围内
	predicted1H = math.Max(0, math.Min(100, predicted1H))
	predicted6H = math.Max(0, math.Min(100, predicted6H))
	predicted24H = math.Max(0, math.Min(100, predicted24H))

	// 确定趋势
	trend := "stable"
	if slope > 0.1 {
		trend = "increasing"
	} else if slope < -0.1 {
		trend = "decreasing"
	}

	// 计算置信度（基于数据点数量和相关性）
	confidence := math.Min(95, float64(len(recent))*1.5)

	// 检查是否需要警告
	warning := false
	warningMsg := ""
	if predicted1H > 90 {
		warning = true
		warningMsg = fmt.Sprintf("%s 使用率预计在 1 小时内达到 %.1f%%", resource, predicted1H)
	} else if predicted6H > 95 {
		warning = true
		warningMsg = fmt.Sprintf("%s 使用率预计在 6 小时内达到 %.1f%%", resource, predicted6H)
	}

	return &PredictionResult{
		Resource:     resource,
		CurrentValue: currentValue,
		Predicted1H:  predicted1H,
		Predicted6H:  predicted6H,
		Predicted24H: predicted24H,
		Trend:        trend,
		Confidence:   confidence,
		Warning:      warning,
		WarningMsg:   warningMsg,
	}
}

// BottleneckDetector 瓶颈检测器
type BottleneckDetector struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *AutoTuneConfig
}

// NewBottleneckDetector 创建瓶颈检测器
func NewBottleneckDetector(logger *zap.Logger, config *AutoTuneConfig) *BottleneckDetector {
	return &BottleneckDetector{
		logger: logger,
		config: config,
	}
}

// Detect 检测瓶颈
func (bd *BottleneckDetector) Detect(metrics *ResourceMetrics) []*Bottleneck {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	var bottlenecks []*Bottleneck

	// CPU 瓶颈检测
	if metrics.CPUPercent > bd.config.CPUThreshold {
		severity := "warning"
		if metrics.CPUPercent > 95 {
			severity = "critical"
		}
		bottlenecks = append(bottlenecks, &Bottleneck{
			ID:          fmt.Sprintf("cpu-%d", time.Now().UnixNano()),
			Type:        "cpu",
			Severity:    severity,
			Description: fmt.Sprintf("CPU 使用率过高: %.1f%%", metrics.CPUPercent),
			Metric:      "cpu_percent",
			Value:       metrics.CPUPercent,
			Threshold:   bd.config.CPUThreshold,
			DetectedAt:  time.Now(),
			Suggestions: []string{
				"检查是否有异常进程占用 CPU",
				"考虑启用 CPU 亲和性优化",
				"检查是否有死循环或低效算法",
				"考虑增加 CPU 资源或优化代码",
			},
		})
	}

	// 内存瓶颈检测
	if metrics.MemPercent > bd.config.MemThreshold {
		severity := "warning"
		if metrics.MemPercent > 95 {
			severity = "critical"
		}
		bottlenecks = append(bottlenecks, &Bottleneck{
			ID:          fmt.Sprintf("mem-%d", time.Now().UnixNano()),
			Type:        "memory",
			Severity:    severity,
			Description: fmt.Sprintf("内存使用率过高: %.1f%%", metrics.MemPercent),
			Metric:      "mem_percent",
			Value:       metrics.MemPercent,
			Threshold:   bd.config.MemThreshold,
			DetectedAt:  time.Now(),
			Suggestions: []string{
				"检查是否有内存泄漏",
				"清理不必要的缓存",
				"调整 GC 参数",
				"考虑增加内存或优化数据结构",
			},
		})
	}

	// 负载瓶颈检测
	if metrics.LoadAvg1 > float64(runtime.NumCPU())*2 {
		severity := "warning"
		if metrics.LoadAvg1 > float64(runtime.NumCPU())*4 {
			severity = "critical"
		}
		bottlenecks = append(bottlenecks, &Bottleneck{
			ID:          fmt.Sprintf("load-%d", time.Now().UnixNano()),
			Type:        "cpu",
			Severity:    severity,
			Description: fmt.Sprintf("系统负载过高: %.2f (CPU 核心数: %d)", metrics.LoadAvg1, runtime.NumCPU()),
			Metric:      "load_avg_1",
			Value:       metrics.LoadAvg1,
			Threshold:   float64(runtime.NumCPU()) * 2,
			DetectedAt:  time.Now(),
			Suggestions: []string{
				"减少并发任务数量",
				"优化任务调度策略",
				"检查是否有阻塞操作",
			},
		})
	}

	return bottlenecks
}

// OptimizationAdvisor 优化建议器
type OptimizationAdvisor struct {
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewOptimizationAdvisor 创建优化建议器
func NewOptimizationAdvisor(logger *zap.Logger) *OptimizationAdvisor {
	return &OptimizationAdvisor{
		logger: logger,
	}
}

// GenerateSuggestions 生成优化建议
func (oa *OptimizationAdvisor) GenerateSuggestions(metrics *ResourceMetrics, bottlenecks []*Bottleneck) []*OptimizationSuggestion {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	suggestions := make([]*OptimizationSuggestion, 0)

	// 基于瓶颈生成建议
	for _, bottleneck := range bottlenecks {
		switch bottleneck.Type {
		case "cpu":
			suggestions = append(suggestions, oa.generateCPUSuggestions(bottleneck)...)
		case "memory":
			suggestions = append(suggestions, oa.generateMemorySuggestions(bottleneck)...)
		case "io":
			suggestions = append(suggestions, oa.generateIOSuggestions(bottleneck)...)
		}
	}

	// 通用优化建议
	suggestions = append(suggestions, oa.generateGeneralSuggestions(metrics)...)

	return suggestions
}

// generateCPUSuggestions 生成 CPU 优化建议
func (oa *OptimizationAdvisor) generateCPUSuggestions(bottleneck *Bottleneck) []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion

	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:             fmt.Sprintf("cpu-opt-1-%d", time.Now().UnixNano()),
		Category:       "cpu",
		Title:          "优化 GOMAXPROCS 设置",
		Description:    "根据实际 CPU 核心数调整 GOMAXPROCS，避免过度并发",
		Impact:         "medium",
		EstimatedGain:  10.0,
		Risk:           "low",
		Implementation: "设置 runtime.GOMAXPROCS(runtime.NumCPU())",
		AutoApplicable: true,
		CreatedAt:      time.Now(),
	})

	if bottleneck.Severity == "critical" {
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:             fmt.Sprintf("cpu-opt-2-%d", time.Now().UnixNano()),
			Category:       "cpu",
			Title:          "启用 CPU 限流",
			Description:    "在 CPU 使用率过高时启用限流，保护系统稳定性",
			Impact:         "high",
			EstimatedGain:  20.0,
			Risk:           "medium",
			Implementation: "配置 CPU 限流器，限制并发任务数量",
			AutoApplicable: false,
			CreatedAt:      time.Now(),
		})
	}

	return suggestions
}

// generateMemorySuggestions 生成内存优化建议
func (oa *OptimizationAdvisor) generateMemorySuggestions(bottleneck *Bottleneck) []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion

	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:             fmt.Sprintf("mem-opt-1-%d", time.Now().UnixNano()),
		Category:       "memory",
		Title:          "触发垃圾回收",
		Description:    "手动触发 GC 释放未使用的内存",
		Impact:         "medium",
		EstimatedGain:  15.0,
		Risk:           "low",
		Implementation: "调用 runtime.GC()",
		AutoApplicable: true,
		CreatedAt:      time.Now(),
	})

	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:             fmt.Sprintf("mem-opt-2-%d", time.Now().UnixNano()),
		Category:       "memory",
		Title:          "清理内存缓存",
		Description:    "清理不必要的内存缓存，释放内存空间",
		Impact:         "high",
		EstimatedGain:  25.0,
		Risk:           "low",
		Implementation: "调用缓存管理器的 Clear() 方法",
		AutoApplicable: true,
		CreatedAt:      time.Now(),
	})

	return suggestions
}

// generateIOSuggestions 生成 IO 优化建议
func (oa *OptimizationAdvisor) generateIOSuggestions(bottleneck *Bottleneck) []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion

	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:             fmt.Sprintf("io-opt-1-%d", time.Now().UnixNano()),
		Category:       "io",
		Title:          "启用 IO 缓冲",
		Description:    "增加 IO 缓冲区大小，减少磁盘访问次数",
		Impact:         "medium",
		EstimatedGain:  15.0,
		Risk:           "low",
		Implementation: "调整 IO 缓冲区配置",
		AutoApplicable: false,
		CreatedAt:      time.Now(),
	})

	return suggestions
}

// generateGeneralSuggestions 生成通用优化建议
func (oa *OptimizationAdvisor) generateGeneralSuggestions(metrics *ResourceMetrics) []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion

	// 如果 goroutine 数量过多
	goroutines := runtime.NumGoroutine()
	if goroutines > 1000 {
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:             fmt.Sprintf("general-opt-1-%d", time.Now().UnixNano()),
			Category:       "general",
			Title:          "优化 Goroutine 使用",
			Description:    fmt.Sprintf("当前 goroutine 数量: %d，建议优化并发策略", goroutines),
			Impact:         "medium",
			EstimatedGain:  10.0,
			Risk:           "low",
			Implementation: "使用工作池模式限制并发数",
			AutoApplicable: false,
			CreatedAt:      time.Now(),
		})
	}

	return suggestions
}

// ScheduledOptimizer 定时优化器
type ScheduledOptimizer struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	engine  *OptimizationEngine
	tasks   map[string]*ScheduledTask
	running bool
	cancel  context.CancelFunc
}

// NewScheduledOptimizer 创建定时优化器
func NewScheduledOptimizer(logger *zap.Logger, engine *OptimizationEngine) *ScheduledOptimizer {
	return &ScheduledOptimizer{
		logger: logger,
		engine: engine,
		tasks:  make(map[string]*ScheduledTask),
	}
}

// AddTask 添加定时任务
func (so *ScheduledOptimizer) AddTask(task *ScheduledTask) {
	so.mu.Lock()
	defer so.mu.Unlock()

	so.tasks[task.ID] = task
	so.logger.Info("添加定时优化任务",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.String("cron", task.CronExpr))
}

// RemoveTask 移除定时任务
func (so *ScheduledOptimizer) RemoveTask(taskID string) {
	so.mu.Lock()
	defer so.mu.Unlock()

	delete(so.tasks, taskID)
	so.logger.Info("移除定时优化任务", zap.String("id", taskID))
}

// GetTasks 获取所有定时任务
func (so *ScheduledOptimizer) GetTasks() []*ScheduledTask {
	so.mu.RLock()
	defer so.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(so.tasks))
	for _, task := range so.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// Start 启动定时优化器
func (so *ScheduledOptimizer) Start(ctx context.Context) {
	so.mu.Lock()
	defer so.mu.Unlock()

	if so.running {
		return
	}

	ctx, so.cancel = context.WithCancel(ctx)
	so.running = true

	go so.run(ctx)
	so.logger.Info("定时优化器已启动")
}

// Stop 停止定时优化器
func (so *ScheduledOptimizer) Stop() {
	so.mu.Lock()
	defer so.mu.Unlock()

	if !so.running {
		return
	}

	if so.cancel != nil {
		so.cancel()
	}

	so.running = false
	so.logger.Info("定时优化器已停止")
}

// run 运行定时任务
func (so *ScheduledOptimizer) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			so.executeDueTasks()
		}
	}
}

// executeDueTasks 执行到期的任务
func (so *ScheduledOptimizer) executeDueTasks() {
	so.mu.RLock()
	tasks := make([]*ScheduledTask, 0)
	for _, task := range so.tasks {
		if task.Enabled && so.isTaskDue(task) {
			tasks = append(tasks, task)
		}
	}
	so.mu.RUnlock()

	for _, task := range tasks {
		so.executeTask(task)
	}
}

// isTaskDue 检查任务是否到期
func (so *ScheduledOptimizer) isTaskDue(task *ScheduledTask) bool {
	if task.LastRun == nil {
		return true
	}

	// 简化的 cron 解析，实际实现需要使用 cron 库
	// 这里假设每小时执行一次
	return time.Since(*task.LastRun) >= 1*time.Hour
}

// executeTask 执行任务
func (so *ScheduledOptimizer) executeTask(task *ScheduledTask) {
	so.mu.Lock()
	now := time.Now()
	task.LastRun = &now
	task.RunCount++
	so.mu.Unlock()

	so.logger.Info("执行定时优化任务",
		zap.String("id", task.ID),
		zap.String("name", task.Name))

	// 收集指标
	metrics := so.engine.metrics.Collect()

	// 检测瓶颈
	bottlenecks := so.engine.detector.Detect(metrics)

	// 生成建议
	suggestions := so.engine.advisor.GenerateSuggestions(metrics, bottlenecks)

	// 应用优化
	if so.engine.config.AutoApply {
		for _, suggestion := range suggestions {
			if suggestion.AutoApplicable {
				so.engine.applySuggestion(suggestion)
			}
		}
	}
}

// NewOptimizationEngine 创建优化引擎
func NewOptimizationEngine(logger *zap.Logger, config *AutoTuneConfig) *OptimizationEngine {
	engine := &OptimizationEngine{
		logger:         logger,
		config:         config,
		history:        NewOptimizationHistory(1000),
		metrics:        NewMetricsCollector(),
		stats:          &EngineStats{},
		maxHistorySize: 1000,
		startTime:      time.Now(),
		metricsHistory: make([]*ResourceMetrics, 0),
	}

	engine.autoTuner = NewAutoTuner(logger, config, engine)
	engine.predictor = NewResourcePredictor(logger, 1440) // 保留 24 小时数据（每分钟一个点）
	engine.detector = NewBottleneckDetector(logger, config)
	engine.advisor = NewOptimizationAdvisor(logger)
	engine.scheduler = NewScheduledOptimizer(logger, engine)

	return engine
}

// Start 启动优化引擎
func (e *OptimizationEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	ctx, e.cancel = context.WithCancel(ctx)
	e.running = true

	// 启动定时优化器
	e.scheduler.Start(ctx)

	// 启动监控循环
	go e.monitorLoop(ctx)

	e.logger.Info("优化引擎已启动")
	return nil
}

// Stop 停止优化引擎
func (e *OptimizationEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	if e.cancel != nil {
		e.cancel()
	}

	e.scheduler.Stop()
	e.running = false
	e.logger.Info("优化引擎已停止")
}

// monitorLoop 监控循环
func (e *OptimizationEngine) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(e.config.TuneInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.monitorCycle()
		}
	}
}

// monitorCycle 监控周期
func (e *OptimizationEngine) monitorCycle() {
	// 收集指标
	metrics := e.metrics.Collect()

	// 保存到历史
	e.mu.Lock()
	e.metricsHistory = append(e.metricsHistory, metrics)
	if len(e.metricsHistory) > e.maxHistorySize {
		e.metricsHistory = e.metricsHistory[len(e.metricsHistory)-e.maxHistorySize:]
	}
	e.mu.Unlock()

	// 添加到预测器
	e.predictor.AddMetrics(metrics)

	// 检测瓶颈
	bottlenecks := e.detector.Detect(metrics)

	// 更新统计
	e.mu.Lock()
	e.stats.BottlenecksDetected += len(bottlenecks)
	e.mu.Unlock()

	// 执行自动调优
	if e.config.Enabled {
		records := e.autoTuner.Tune(context.Background(), metrics)
		for _, record := range records {
			e.history.Add(record)
			e.mu.Lock()
			e.stats.TotalOptimizations++
			e.stats.SuccessfulTunes++
			e.stats.TotalImprovement += record.Improvement
			e.mu.Unlock()
		}
	}
}

// applySuggestion 应用优化建议
func (e *OptimizationEngine) applySuggestion(suggestion *OptimizationSuggestion) {
	e.logger.Info("应用优化建议",
		zap.String("id", suggestion.ID),
		zap.String("title", suggestion.Title))

	now := time.Now()
	suggestion.AppliedAt = &now

	// 实际的应用逻辑需要根据具体建议实现
}

// GetStats 获取引擎统计
func (e *OptimizationEngine) GetStats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.Uptime = time.Since(e.startTime)
	if stats.TotalOptimizations > 0 {
		stats.AvgImprovement = stats.TotalImprovement / float64(stats.TotalOptimizations)
	}
	return &stats
}

// GetPredictions 获取资源预测
func (e *OptimizationEngine) GetPredictions() []*PredictionResult {
	return e.predictor.Predict()
}

// GetBottlenecks 获取当前瓶颈
func (e *OptimizationEngine) GetBottlenecks() []*Bottleneck {
	metrics := e.metrics.Collect()
	return e.detector.Detect(metrics)
}

// GetSuggestions 获取优化建议
func (e *OptimizationEngine) GetSuggestions() []*OptimizationSuggestion {
	metrics := e.metrics.Collect()
	bottlenecks := e.detector.Detect(metrics)
	return e.advisor.GenerateSuggestions(metrics, bottlenecks)
}

// GetHistory 获取优化历史
func (e *OptimizationEngine) GetHistory() []*OptimizationRecord {
	return e.history.GetAll()
}

// GetScheduler 获取定时优化器
func (e *OptimizationEngine) GetScheduler() *ScheduledOptimizer {
	return e.scheduler
}

// GetConfig 获取配置
func (e *OptimizationEngine) GetConfig() *AutoTuneConfig {
	return e.config
}

// UpdateConfig 更新配置
func (e *OptimizationEngine) UpdateConfig(config *AutoTuneConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// IsRunning 是否运行中
func (e *OptimizationEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
