// Package edgeai 提供模型优化功能
package edgeai

import (
	"fmt"
	"log"
	"sync"
)

// Optimizer 模型优化器
type Optimizer struct {
	mu        sync.RWMutex
	strategies map[string]OptimizationStrategy
}

// OptimizationStrategy 优化策略接口
type OptimizationStrategy interface {
	// Optimize 执行优化
	Optimize(model *Model, options *OptimizationOptions) (*Model, error)
	// CanOptimize 是否可以优化
	CanOptimize(model *Model) bool
	// GetDescription 获取策略描述
	GetDescription() string
}

// OptimizationOptions 优化选项
type OptimizationOptions struct {
	TargetPrecision string `json:"targetPrecision"` // fp32/fp16/int8
	Quantize        bool   `json:"quantize"`        // 是否量化
	Prune           bool   `json:"prune"`           // 是否剪枝
	PruneRatio      float64 `json:"pruneRatio"`     // 剪枝比例
	Distill         bool   `json:"distill"`         // 是否蒸馏
	TeacherModel    string `json:"teacherModel"`    // 教师模型 ID
	TargetSize      int64  `json:"targetSize"`      // 目标大小 (bytes)
	MaxLatency      float64 `json:"maxLatency"`     // 最大延迟 (ms)
	MinAccuracy     float64 `json:"minAccuracy"`    // 最小精度
}

// NewOptimizer 创建模型优化器
func NewOptimizer() *Optimizer {
	o := &Optimizer{
		strategies: make(map[string]OptimizationStrategy),
	}

	// 注册默认优化策略
	o.RegisterStrategy("quantization", &QuantizationStrategy{})
	o.RegisterStrategy("pruning", &PruningStrategy{})
	o.RegisterStrategy("distillation", &DistillationStrategy{})
	o.RegisterStrategy("optimization", &GeneralOptimizationStrategy{})

	return o
}

// RegisterStrategy 注册优化策略
func (o *Optimizer) RegisterStrategy(name string, strategy OptimizationStrategy) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.strategies[name] = strategy
}

// Optimize 优化模型
func (o *Optimizer) Optimize(model *Model, options *OptimizationOptions) (*Model, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if options == nil {
		options = &OptimizationOptions{
			TargetPrecision: "fp16",
			Quantize:        false,
			Prune:           false,
		}
	}

	// 执行量化
	if options.Quantize {
		strategy, ok := o.strategies["quantization"]
		if ok && strategy.CanOptimize(model) {
			optimized, err := strategy.Optimize(model, options)
			if err != nil {
				return nil, fmt.Errorf("量化失败: %w", err)
			}
			model = optimized
		}
	}

	// 执行剪枝
	if options.Prune {
		strategy, ok := o.strategies["pruning"]
		if ok && strategy.CanOptimize(model) {
			optimized, err := strategy.Optimize(model, options)
			if err != nil {
				return nil, fmt.Errorf("剪枝失败: %w", err)
			}
			model = optimized
		}
	}

	// 执行通用优化
	strategy, ok := o.strategies["optimization"]
	if ok && strategy.CanOptimize(model) {
		optimized, err := strategy.Optimize(model, options)
		if err != nil {
			return nil, fmt.Errorf("优化失败: %w", err)
		}
		model = optimized
	}

	return model, nil
}

// GetStrategies 获取所有策略
func (o *Optimizer) GetStrategies() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	strategies := make(map[string]string)
	for name, strategy := range o.strategies {
		strategies[name] = strategy.GetDescription()
	}

	return strategies
}

// QuantizationStrategy 量化策略
type QuantizationStrategy struct{}

// Optimize 执行量化
func (s *QuantizationStrategy) Optimize(model *Model, options *OptimizationOptions) (*Model, error) {
	precision := options.TargetPrecision
	if precision == "" {
		precision = "int8"
	}

	log.Printf("量化模型 %s 到 %s 精度", model.ID, precision)

	// 创建优化后的模型副本
	optimized := *model
	optimized.Config = &ModelConfig{
		BatchSize:     model.Config.BatchSize,
		NumThreads:    model.Config.NumThreads,
		Precision:     precision,
		Quantized:     true,
		OptimizeLevel: model.Config.OptimizeLevel,
	}

	// 模拟内存减少
	switch precision {
	case "int8":
		optimized.MemoryUsage = model.MemoryUsage / 4
	case "fp16":
		optimized.MemoryUsage = model.MemoryUsage / 2
	default:
		optimized.MemoryUsage = model.MemoryUsage
	}

	return &optimized, nil
}

// CanOptimize 是否可以量化
func (s *QuantizationStrategy) CanOptimize(model *Model) bool {
	// 检查模型格式是否支持量化
	return model.Format == ModelFormatONNX || model.Format == ModelFormatTFLite
}

// GetDescription 获取策略描述
func (s *QuantizationStrategy) GetDescription() string {
	return "模型量化：将浮点数精度降低（FP32→FP16/INT8），减少内存占用和计算量"
}

// PruningStrategy 剪枝策略
type PruningStrategy struct{}

// Optimize 执行剪枝
func (s *PruningStrategy) Optimize(model *Model, options *OptimizationOptions) (*Model, error) {
	ratio := options.PruneRatio
	if ratio <= 0 || ratio > 0.9 {
		ratio = 0.3
	}

	log.Printf("剪枝模型 %s，比例: %.1f%%", model.ID, ratio*100)

	// 创建优化后的模型副本
	optimized := *model
	optimized.Metadata = make(map[string]string)
	for k, v := range model.Metadata {
		optimized.Metadata[k] = v
	}
	optimized.Metadata["pruned"] = "true"
	optimized.Metadata["prune_ratio"] = fmt.Sprintf("%.2f", ratio)

	// 模拟大小减少
	optimized.MemoryUsage = int64(float64(model.MemoryUsage) * (1 - ratio))

	return &optimized, nil
}

// CanOptimize 是否可以剪枝
func (s *PruningStrategy) CanOptimize(model *Model) bool {
	// 检查模型是否支持剪枝
	return model.Format == ModelFormatONNX || model.Format == ModelFormatPyTorch
}

// GetDescription 获取策略描述
func (s *PruningStrategy) GetDescription() string {
	return "模型剪枝：移除不重要的权重，减少模型大小和计算量"
}

// DistillationStrategy 蒸馏策略
type DistillationStrategy struct{}

// Optimize 执行蒸馏
func (s *DistillationStrategy) Optimize(model *Model, options *OptimizationOptions) (*Model, error) {
	if options.TeacherModel == "" {
		return nil, fmt.Errorf("蒸馏需要指定教师模型")
	}

	log.Printf("蒸馏模型 %s，教师模型: %s", model.ID, options.TeacherModel)

	// 创建优化后的模型副本
	optimized := *model
	optimized.Metadata = make(map[string]string)
	for k, v := range model.Metadata {
		optimized.Metadata[k] = v
	}
	optimized.Metadata["distilled"] = "true"
	optimized.Metadata["teacher_model"] = options.TeacherModel

	// 模拟大小减少
	optimized.MemoryUsage = model.MemoryUsage / 3

	return &optimized, nil
}

// CanOptimize 是否可以蒸馏
func (s *DistillationStrategy) CanOptimize(model *Model) bool {
	return model.Format == ModelFormatPyTorch
}

// GetDescription 获取策略描述
func (s *DistillationStrategy) GetDescription() string {
	return "知识蒸馏：使用大模型（教师）训练小模型（学生），保持精度的同时减小模型"
}

// GeneralOptimizationStrategy 通用优化策略
type GeneralOptimizationStrategy struct{}

// Optimize 执行通用优化
func (s *GeneralOptimizationStrategy) Optimize(model *Model, options *OptimizationOptions) (*Model, error) {
	log.Printf("通用优化模型 %s", model.ID)

	// 创建优化后的模型副本
	optimized := *model
	optimized.Config = &ModelConfig{
		BatchSize:     model.Config.BatchSize,
		NumThreads:    model.Config.NumThreads,
		Precision:     model.Config.Precision,
		Quantized:     model.Config.Quantized,
		OptimizeLevel: 3, // 最高优化级别
	}

	return &optimized, nil
}

// CanOptimize 是否可以优化
func (s *GeneralOptimizationStrategy) CanOptimize(model *Model) bool {
	return true
}

// GetDescription 获取策略描述
func (s *GeneralOptimizationStrategy) GetDescription() string {
	return "通用优化：应用图优化、算子融合等技术提升推理性能"
}

// ModelAnalyzer 模型分析器
type ModelAnalyzer struct{}

// NewModelAnalyzer 创建模型分析器
func NewModelAnalyzer() *ModelAnalyzer {
	return &ModelAnalyzer{}
}

// Analyze 分析模型
func (a *ModelAnalyzer) Analyze(model *Model) (*ModelAnalysis, error) {
	analysis := &ModelAnalysis{
		ModelID:       model.ID,
		Format:        model.Format,
		TaskType:      model.TaskType,
		MemoryUsage:   model.MemoryUsage,
		InputShape:    model.InputShape,
		OutputShape:   model.OutputShape,
		Optimizations: make([]string, 0),
	}

	// 分析优化建议
	if model.Config != nil && model.Config.Precision == "fp32" {
		analysis.Optimizations = append(analysis.Optimizations, "可以量化到 FP16 或 INT8")
	}

	if model.MemoryUsage > 1024*1024*1024 { // > 1GB
		analysis.Optimizations = append(analysis.Optimizations, "模型较大，建议剪枝或蒸馏")
	}

	if model.Config != nil && model.Config.OptimizeLevel < 3 {
		analysis.Optimizations = append(analysis.Optimizations, "可以提升优化级别到 3")
	}

	// 计算复杂度评分
	analysis.ComplexityScore = a.calculateComplexity(model)

	return analysis, nil
}

// calculateComplexity 计算复杂度评分
func (a *ModelAnalyzer) calculateComplexity(model *Model) float64 {
	score := 0.0

	// 基于内存使用
	if model.MemoryUsage > 0 {
		score += float64(model.MemoryUsage) / (1024 * 1024 * 1024) * 30 // 30% 权重
	}

	// 基于输入形状
	if len(model.InputShape) > 0 {
		dim := 1
		for _, d := range model.InputShape {
			dim *= d
		}
		score += float64(dim) / 1000000 * 30 // 30% 权重
	}

	// 基于格式
	switch model.Format {
	case ModelFormatONNX:
		score += 20
	case ModelFormatPyTorch:
		score += 25
	case ModelFormatTFLite:
		score += 15
	}

	return score
}

// ModelAnalysis 模型分析结果
type ModelAnalysis struct {
	ModelID          string      `json:"modelId"`
	Format           ModelFormat `json:"format"`
	TaskType         TaskType    `json:"taskType"`
	MemoryUsage      int64       `json:"memoryUsage"`
	InputShape       []int       `json:"inputShape"`
	OutputShape      []int       `json:"outputShape"`
	ComplexityScore  float64     `json:"complexityScore"`
	Optimizations    []string    `json:"optimizations"`
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	ModelID       string        `json:"modelId"`
	Device        ComputeDevice `json:"device"`
	BatchSize     int           `json:"batchSize"`
	AvgLatency    float64       `json:"avgLatency"`    // ms
	P50Latency    float64       `json:"p50Latency"`    // ms
	P95Latency    float64       `json:"p95Latency"`    // ms
	P99Latency    float64       `json:"p99Latency"`    // ms
	Throughput    float64       `json:"throughput"`    // 推理/秒
	MemoryPeak    int64         `json:"memoryPeak"`    // bytes
	Iterations    int           `json:"iterations"`
}

// Benchmarker 基准测试器
type Benchmarker struct {
	engine *Engine
}

// NewBenchmarker 创建基准测试器
func NewBenchmarker(engine *Engine) *Benchmarker {
	return &Benchmarker{engine: engine}
}

// Run 运行基准测试
func (b *Benchmarker) Run(modelID string, iterations int, batchSize int) (*BenchmarkResult, error) {
	if iterations <= 0 {
		iterations = 100
	}
	if batchSize <= 0 {
		batchSize = 1
	}

	// 获取模型
	model, err := b.engine.GetModel(modelID)
	if err != nil {
		return nil, err
	}

	latencies := make([]float64, 0, iterations)

	// 执行基准测试
	for i := 0; i < iterations; i++ {
		request := &InferenceRequest{
			ModelID:  modelID,
			TaskType: model.TaskType,
			Input: &InferenceInput{
				Tensor: make([]float32, 100),
				Shape:  []int{1, 100},
			},
		}

		result, err := b.engine.Infer(request)
		if err != nil {
			continue
		}

		latencies = append(latencies, float64(result.Latency.Milliseconds()))
	}

	if len(latencies) == 0 {
		return nil, fmt.Errorf("所有推理都失败了")
	}

	// 计算统计数据
	avgLatency := calculateAverage(latencies)
	p50 := calculatePercentile(latencies, 50)
	p95 := calculatePercentile(latencies, 95)
	p99 := calculatePercentile(latencies, 99)
	throughput := 1000.0 / avgLatency * float64(batchSize)

	return &BenchmarkResult{
		ModelID:    modelID,
		Device:     model.Device,
		BatchSize:  batchSize,
		AvgLatency: avgLatency,
		P50Latency: p50,
		P95Latency: p95,
		P99Latency: p99,
		Throughput: throughput,
		Iterations: len(latencies),
	}, nil
}

// calculateAverage 计算平均值
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculatePercentile 计算百分位数
func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 简化实现：排序后取百分位
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// 简单冒泡排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[idx]
}
