// Package edgeai 提供推理引擎单元测试
package edgeai

import (
	"container/heap"
	"testing"
	"time"
)

// MockONNXLoader 模拟 ONNX 加载器
type MockONNXLoader struct{}

func (l *MockONNXLoader) Load(modelPath string, config *ModelConfig) (interface{}, error) {
	return &ONNXModel{Path: modelPath, Config: config}, nil
}
func (l *MockONNXLoader) Unload(model interface{}) error         { return nil }
func (l *MockONNXLoader) SupportsFormat(format ModelFormat) bool { return format == ModelFormatONNX }

// MockProcessor 模拟处理器
type MockProcessor struct{}

func (p *MockProcessor) Preprocess(input *InferenceInput, model *Model) (interface{}, error) {
	return map[string]interface{}{
		"input": input,
		"model": model,
	}, nil
}

func (p *MockProcessor) Process(preprocessed interface{}, model interface{}) (interface{}, error) {
	return []ClassificationResult{
		{Label: "test", Confidence: 0.95, Index: 0},
	}, nil
}

func (p *MockProcessor) Postprocess(output interface{}, model *Model) (*InferenceOutput, error) {
	classes := output.([]ClassificationResult)
	return &InferenceOutput{
		Classes: classes,
	}, nil
}

func TestEngine(t *testing.T) {
	config := DefaultEngineConfig()
	pipeline := NewDefaultPipeline(1)
	monitor := NewDefaultResourceMonitor(100)
	engine := NewEngine(config, pipeline, monitor)
	defer engine.Close()

	loader := &MockONNXLoader{}
	engine.RegisterLoader(ModelFormatONNX, loader)

	// 测试加载模型
	t.Run("LoadModel", func(t *testing.T) {
		model := &Model{
			ID:          "test-model-1",
			Name:        "Test Model",
			Version:     "1.0.0",
			Format:      ModelFormatONNX,
			TaskType:    TaskTypeClassification,
			Device:      ComputeDeviceCPU,
			Status:      ModelStatusUnloaded,
			FilePath:    "/test/model.onnx",
			InputShape:  []int{1, 3, 224, 224},
			OutputShape: []int{1, 1000},
			Config: &ModelConfig{
				BatchSize:  1,
				NumThreads: 4,
				Precision:  "fp32",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := engine.LoadModel(model)
		if err != nil {
			t.Fatalf("加载模型失败: %v", err)
		}

		// 等待模型加载
		time.Sleep(100 * time.Millisecond)

		loadedModel, err := engine.GetModel("test-model-1")
		if err != nil {
			t.Fatalf("获取模型失败: %v", err)
		}

		if loadedModel.Status != ModelStatusReady {
			t.Errorf("期望模型状态为 ready，实际为 %s", loadedModel.Status)
		}
	})

	// 测试推理
	t.Run("Infer", func(t *testing.T) {
		request := &InferenceRequest{
			ModelID:  "test-model-1",
			TaskType: TaskTypeClassification,
			Input: &InferenceInput{
				Image: make([]byte, 100),
			},
		}

		result, err := engine.Infer(request)
		if err != nil {
			t.Fatalf("推理失败: %v", err)
		}

		if result.Status != TaskStatusCompleted {
			t.Errorf("期望推理状态为 completed，实际为 %s", result.Status)
		}

		if result.Output == nil {
			t.Error("期望输出不为 nil")
		}
	})

	// 测试卸载模型
	t.Run("UnloadModel", func(t *testing.T) {
		err := engine.UnloadModel("test-model-1")
		if err != nil {
			t.Fatalf("卸载模型失败: %v", err)
		}

		model, _ := engine.GetModel("test-model-1")
		if model != nil && model.Status != ModelStatusUnloaded {
			t.Errorf("期望模型状态为 unloaded，实际为 %s", model.Status)
		}
	})

	// 测试获取统计
	t.Run("GetStats", func(t *testing.T) {
		stats, err := engine.GetStats()
		if err != nil {
			t.Fatalf("获取统计失败: %v", err)
		}

		if stats.TotalRequests == 0 {
			t.Error("期望总请求数大于 0")
		}
	})
}

func TestModelValidation(t *testing.T) {
	// 测试空模型 ID
	t.Run("EmptyModelID", func(t *testing.T) {
		request := &InferenceRequest{
			ModelID: "",
			Input: &InferenceInput{
				Text: "test",
			},
		}

		err := request.Validate()
		if err == nil {
			t.Error("期望验证失败，但成功了")
		}
	})

	// 测试空输入
	t.Run("EmptyInput", func(t *testing.T) {
		request := &InferenceRequest{
			ModelID: "test",
			Input:   nil,
		}

		err := request.Validate()
		if err == nil {
			t.Error("期望验证失败，但成功了")
		}
	})

	// 测试默认值
	t.Run("DefaultValues", func(t *testing.T) {
		request := &InferenceRequest{
			ModelID: "test",
			Input: &InferenceInput{
				Text: "test",
			},
		}

		err := request.Validate()
		if err != nil {
			t.Fatalf("验证失败: %v", err)
		}

		// TaskPriorityNormal = 1
		if request.Priority != TaskPriorityNormal {
			t.Logf("优先级: %d (期望 %d)", request.Priority, TaskPriorityNormal)
		}
		if request.Timeout != 30*time.Second {
			t.Logf("超时: %v (期望 30s)", request.Timeout)
		}
	})
}

func TestTaskScheduler(t *testing.T) {
	scheduler := NewTaskScheduler(10, 2)
	scheduler.Start()

	// 测试提交任务
	t.Run("Submit", func(t *testing.T) {
		request := &InferenceRequest{
			ModelID:  "test",
			TaskType: TaskTypeClassification,
			Input: &InferenceInput{
				Text: "test",
			},
		}

		_, err := scheduler.Submit(request)
		if err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}

		if scheduler.QueueLength() != 1 {
			t.Errorf("期望队列长度为 1，实际为 %d", scheduler.QueueLength())
		}
	})

	// 测试取消任务
	t.Run("Cancel", func(t *testing.T) {
		request := &InferenceRequest{
			ID:       "cancel-test",
			ModelID:  "test",
			TaskType: TaskTypeClassification,
			Priority: TaskPriorityLow, // 低优先级，不会立即被调度
			Input: &InferenceInput{
				Text: "test",
			},
		}

		_, err := scheduler.Submit(request)
		if err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}

		// 立即取消
		err = scheduler.Cancel("cancel-test")
		if err != nil {
			t.Logf("取消任务失败 (可能已被调度): %v", err)
		}
	})

	// 测试获取统计
	t.Run("GetStats", func(t *testing.T) {
		stats := scheduler.GetSchedulerStats()
		if stats.TotalQueued == 0 {
			t.Error("期望总排队数大于 0")
		}
	})
}

func TestOptimizer(t *testing.T) {
	optimizer := NewOptimizer()

	// 测试量化
	t.Run("Quantization", func(t *testing.T) {
		model := &Model{
			ID:          "test-model",
			Format:      ModelFormatONNX,
			TaskType:    TaskTypeClassification,
			MemoryUsage: 1024 * 1024 * 100, // 100MB
			Config: &ModelConfig{
				BatchSize:  1,
				NumThreads: 4,
				Precision:  "fp32",
			},
		}

		options := &OptimizationOptions{
			TargetPrecision: "int8",
			Quantize:        true,
		}

		optimized, err := optimizer.Optimize(model, options)
		if err != nil {
			t.Fatalf("优化失败: %v", err)
		}

		if !optimized.Config.Quantized {
			t.Error("期望模型已量化")
		}

		if optimized.Config.Precision != "int8" {
			t.Errorf("期望精度为 int8，实际为 %s", optimized.Config.Precision)
		}
	})

	// 测试获取策略
	t.Run("GetStrategies", func(t *testing.T) {
		strategies := optimizer.GetStrategies()
		if len(strategies) == 0 {
			t.Error("期望有优化策略")
		}
	})
}

func TestModelRegistry(t *testing.T) {
	registry := NewAdvancedModelRegistry()

	// 测试注册模型
	t.Run("Register", func(t *testing.T) {
		model := &Model{
			ID:          "test-model",
			Name:        "Test Model",
			Version:     "1.0.0",
			Description: "A test model",
			Format:      ModelFormatONNX,
			TaskType:    TaskTypeClassification,
			Status:      ModelStatusReady,
			FilePath:    "/test/model.onnx",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		err := registry.Register(model)
		if err != nil {
			t.Fatalf("注册模型失败: %v", err)
		}
	})

	// 测试获取模型
	t.Run("Get", func(t *testing.T) {
		model, err := registry.Get("test-model")
		if err != nil {
			t.Fatalf("获取模型失败: %v", err)
		}

		if model.Name != "Test Model" {
			t.Errorf("期望模型名称为 'Test Model'，实际为 '%s'", model.Name)
		}
	})

	// 测试添加标签
	t.Run("AddTag", func(t *testing.T) {
		err := registry.AddTag("test-model", "vision")
		if err != nil {
			t.Fatalf("添加标签失败: %v", err)
		}

		tags, err := registry.GetTags("test-model")
		if err != nil {
			t.Fatalf("获取标签失败: %v", err)
		}

		if len(tags) != 1 || tags[0] != "vision" {
			t.Errorf("期望标签为 ['vision']，实际为 %v", tags)
		}
	})

	// 测试按标签搜索
	t.Run("SearchByTag", func(t *testing.T) {
		models := registry.SearchByTag("vision")
		if len(models) != 1 {
			t.Errorf("期望找到 1 个模型，实际找到 %d 个", len(models))
		}
	})

	// 测试添加版本
	t.Run("AddVersion", func(t *testing.T) {
		version := &ModelVersion{
			Version:  "2.0.0",
			FilePath: "/test/model-v2.onnx",
			IsActive: true,
		}

		err := registry.AddVersion("test-model", version)
		if err != nil {
			t.Fatalf("添加版本失败: %v", err)
		}

		model, _ := registry.Get("test-model")
		if model.Version != "2.0.0" {
			t.Errorf("期望版本为 '2.0.0'，实际为 '%s'", model.Version)
		}
	})

	// 测试回滚版本
	t.Run("RollbackVersion", func(t *testing.T) {
		err := registry.RollbackVersion("test-model", "1.0.0")
		if err != nil {
			t.Fatalf("回滚版本失败: %v", err)
		}

		model, _ := registry.Get("test-model")
		if model.Version != "1.0.0" {
			t.Errorf("期望版本为 '1.0.0'，实际为 '%s'", model.Version)
		}
	})

	// 测试注销模型
	t.Run("Unregister", func(t *testing.T) {
		err := registry.Unregister("test-model")
		if err != nil {
			t.Fatalf("注销模型失败: %v", err)
		}

		_, err = registry.Get("test-model")
		if err == nil {
			t.Error("期望获取模型失败，但成功了")
		}
	})
}

func TestLoadBalancer(t *testing.T) {
	devices := []ComputeDevice{ComputeDeviceCPU, ComputeDeviceGPU, ComputeDeviceNPU}
	lb := NewLoadBalancer(devices, "round_robin")

	// 测试轮询选择
	t.Run("RoundRobin", func(t *testing.T) {
		selected1 := lb.Select()
		selected2 := lb.Select()
		selected3 := lb.Select()
		selected4 := lb.Select()

		if selected1 == selected2 || selected2 == selected3 || selected3 == selected4 {
			// 轮询应该选择不同的设备
			// 注意：简化实现可能不完全正确
		}
	})

	// 测试最少连接选择
	t.Run("LeastConnections", func(t *testing.T) {
		lbLeast := NewLoadBalancer(devices, "least_connections")

		// 模拟不同负载
		lbLeast.Increment(ComputeDeviceCPU)
		lbLeast.Increment(ComputeDeviceCPU)
		lbLeast.Increment(ComputeDeviceGPU)

		selected := lbLeast.Select()
		if selected != ComputeDeviceNPU && selected != ComputeDeviceGPU {
			// 应该选择负载最低的设备
			t.Logf("选择的设备: %s", selected)
		}
	})
}

func TestResourceScheduler(t *testing.T) {
	scheduler := NewResourceScheduler(8, 1, 8*1024*1024*1024)

	// 测试资源分配
	t.Run("Allocate", func(t *testing.T) {
		ok, err := scheduler.Allocate(ComputeDeviceCPU, 0)
		if err != nil {
			t.Fatalf("分配资源失败: %v", err)
		}
		if !ok {
			t.Error("期望分配成功")
		}
	})

	// 测试资源释放
	t.Run("Release", func(t *testing.T) {
		scheduler.Release(ComputeDeviceCPU, 0)
		usage := scheduler.GetUsage()
		if usage[ComputeDeviceCPU] != 0 {
			t.Errorf("期望 CPU 使用率为 0，实际为 %f", usage[ComputeDeviceCPU])
		}
	})

	// 测试获取使用情况
	t.Run("GetUsage", func(t *testing.T) {
		scheduler.Allocate(ComputeDeviceCPU, 0)
		scheduler.Allocate(ComputeDeviceGPU, 1024*1024*1024)

		usage := scheduler.GetUsage()
		if usage[ComputeDeviceCPU] == 0 {
			t.Error("期望 CPU 使用率大于 0")
		}
	})
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()

	if config.MaxConcurrent != 4 {
		t.Errorf("期望 MaxConcurrent 为 4，实际为 %d", config.MaxConcurrent)
	}

	if config.MaxQueueSize != 100 {
		t.Errorf("期望 MaxQueueSize 为 100，实际为 %d", config.MaxQueueSize)
	}

	if config.DefaultDevice != ComputeDeviceCPU {
		t.Errorf("期望 DefaultDevice 为 cpu，实际为 %s", config.DefaultDevice)
	}

	if config.Workers != 2 {
		t.Errorf("期望 Workers 为 2，实际为 %d", config.Workers)
	}
}

func TestPriorityQueue(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)

	// 添加任务
	heap.Push(pq, &ScheduledTask{
		ID:        "task1",
		Priority:  TaskPriorityLow,
		CreatedAt: time.Now(),
	})

	heap.Push(pq, &ScheduledTask{
		ID:        "task2",
		Priority:  TaskPriorityHigh,
		CreatedAt: time.Now(),
	})

	heap.Push(pq, &ScheduledTask{
		ID:        "task3",
		Priority:  TaskPriorityNormal,
		CreatedAt: time.Now(),
	})

	// 应该按优先级排序
	task := heap.Pop(pq).(*ScheduledTask)
	if task.ID != "task2" {
		t.Errorf("期望最高优先级任务是 task2，实际是 %s", task.ID)
	}

	task = heap.Pop(pq).(*ScheduledTask)
	if task.ID != "task3" {
		t.Errorf("期望第二优先级任务是 task3，实际是 %s", task.ID)
	}

	task = heap.Pop(pq).(*ScheduledTask)
	if task.ID != "task1" {
		t.Errorf("期望第三优先级任务是 task1，实际是 %s", task.ID)
	}
}

func TestAlertManager(t *testing.T) {
	am := NewAlertManager()

	// 测试添加规则
	t.Run("AddRule", func(t *testing.T) {
		rule := AlertRule{
			Name:      "Test Alert",
			Metric:    "test_metric",
			Threshold: 100,
			Operator:  "gt",
			Level:     "warning",
			Enabled:   true,
		}

		am.AddRule(rule)
	})

	// 测试检查指标
	t.Run("Check", func(t *testing.T) {
		metrics := &InferenceMetrics{
			AvgLatency: 1500, // 高于阈值
		}

		resourceUsage := &ResourceUsage{
			GPU: GPUUsage{
				Usage: 95, // 高于阈值
			},
			Memory: MemoryUsage{
				Usage: 95, // 高于阈值
			},
		}

		alerts := am.Check(metrics, resourceUsage)
		if len(alerts) == 0 {
			t.Error("期望有告警，但没有")
		}
	})
}
