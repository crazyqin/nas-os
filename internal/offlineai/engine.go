// Package offlineai 离线AI引擎核心
package offlineai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Engine 离线AI引擎.
type Engine struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *Config
	models      map[string]*Model
	gpuInfo     *GPUInfo
	concurrency chan struct{} // 并发控制
	running     int32         // atomic
	stopCh      chan struct{}
}

// NewEngine 创建离线AI引擎.
func NewEngine(logger *zap.Logger, config *Config) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	e := &Engine{
		logger:      logger,
		config:      config,
		models:      make(map[string]*Model),
		concurrency: make(chan struct{}, config.MaxConcurrent),
		stopCh:      make(chan struct{}),
	}

	// 检测 GPU 信息
	e.gpuInfo = e.detectGPU()

	return e
}

// detectGPU 检测 GPU 信息.
func (e *Engine) detectGPU() *GPUInfo {
	info := &GPUInfo{
		Available: false,
	}

	// 简化实现：实际应调用 nvidia-smi 或检测 CUDA
	if e.config.GPUEnabled {
		// 模拟 GPU 检测
		info.Available = true
		info.Name = "Detected GPU"
		info.VRAMTotal = 8 * 1024 * 1024 * 1024 // 8GB
		info.VRAMFree = 6 * 1024 * 1024 * 1024  // 6GB
		info.Driver = "535.104.05"
		e.logger.Info("GPU detected",
			zap.String("name", info.Name),
			zap.Int64("vram_mb", info.VRAMTotal/1024/1024),
		)
	} else {
		e.logger.Info("GPU disabled, using CPU inference")
	}

	return info
}

// Start 启动引擎.
func (e *Engine) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&e.running, 0, 1) {
		return fmt.Errorf("engine already running")
	}

	e.logger.Info("starting offline AI engine",
		zap.String("engine", string(e.config.EngineType)),
		zap.Int("context_size", e.config.ContextSize),
		zap.Bool("gpu", e.config.GPUEnabled),
	)

	// 加载默认模型
	if e.config.DefaultModel != "" {
		if err := e.loadDefaultModel(ctx); err != nil {
			e.logger.Warn("failed to load default model", zap.Error(err))
		}
	}

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	if !atomic.CompareAndSwapInt32(&e.running, 1, 0) {
		return
	}

	close(e.stopCh)

	// 卸载所有模型
	e.mu.Lock()
	for name := range e.models {
		e.logger.Info("unloading model", zap.String("name", name))
		delete(e.models, name)
	}
	e.mu.Unlock()

	e.logger.Info("offline AI engine stopped")
}

// IsRunning 引擎是否运行中.
func (e *Engine) IsRunning() bool {
	return atomic.LoadInt32(&e.running) == 1
}

// loadDefaultModel 加载默认模型.
func (e *Engine) loadDefaultModel(ctx context.Context) error {
	return e.LoadModel(ctx, &Model{
		Name:        e.config.DefaultModel,
		Path:        fmt.Sprintf("%s/%s.gguf", e.config.ModelDir, e.config.DefaultModel),
		Format:      ModelFormatGGUF,
		QuantType:   QuantQ4_0,
		Status:      ModelStatusReady,
		GPUSupport:  e.gpuInfo.Available,
		MaxContext:  e.config.ContextSize,
		LoadedAt:    time.Now(),
		Description: "Default offline AI model",
	})
}

// LoadModel 加载模型.
func (e *Engine) LoadModel(ctx context.Context, model *Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.models[model.Name]; exists {
		return fmt.Errorf("model %s already loaded", model.Name)
	}

	model.Status = ModelStatusLoading
	e.logger.Info("loading model", zap.String("name", model.Name), zap.String("path", model.Path))

	// 模拟模型加载
	model.Status = ModelStatusReady
	model.LoadedAt = time.Now()
	e.models[model.Name] = model

	e.logger.Info("model loaded", zap.String("name", model.Name))
	return nil
}

// UnloadModel 卸载模型.
func (e *Engine) UnloadModel(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[name]
	if !exists {
		return fmt.Errorf("model %s not found", name)
	}

	model.Status = ModelStatusUnloaded
	delete(e.models, name)
	e.logger.Info("model unloaded", zap.String("name", name))
	return nil
}

// GetModel 获取模型信息.
func (e *Engine) GetModel(name string) (*Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	model, exists := e.models[name]
	if !exists {
		return nil, fmt.Errorf("model %s not found", name)
	}
	return model, nil
}

// ListModels 列出所有已加载模型.
func (e *Engine) ListModels() []*Model {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Model, 0, len(e.models))
	for _, m := range e.models {
		result = append(result, m)
	}
	return result
}

// SwitchModel 切换默认模型.
func (e *Engine) SwitchModel(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.models[name]; !exists {
		return fmt.Errorf("model %s not loaded", name)
	}

	e.config.DefaultModel = name
	e.logger.Info("default model switched", zap.String("name", name))
	return nil
}

// GetGPUInfo 获取 GPU 信息.
func (e *Engine) GetGPUInfo() *GPUInfo {
	return e.gpuInfo
}

// Infer 执行推理.
func (e *Engine) Infer(ctx context.Context, req *InferRequest) (*InferResponse, error) {
	if !e.IsRunning() {
		return nil, fmt.Errorf("engine not running")
	}

	// 并发控制
	select {
	case e.concurrency <- struct{}{}:
		defer func() { <-e.concurrency }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	start := time.Now()

	// 确定使用的模型
	modelName := req.ModelName
	if modelName == "" {
		modelName = e.config.DefaultModel
	}

	e.mu.RLock()
	model, exists := e.models[modelName]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model %s not loaded", modelName)
	}

	if model.Status != ModelStatusReady {
		return nil, fmt.Errorf("model %s not ready (status: %s)", modelName, model.Status)
	}

	// 设置默认参数
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = e.config.MaxTokens
	}
	temperature := req.Temperature
	if temperature <= 0 {
		temperature = e.config.Temperature
	}

	// 模拟推理
	result := fmt.Sprintf("Response from %s: processed '%s'", modelName, truncateString(req.Prompt, 50))

	_ = temperature // 使用参数

	return &InferResponse{
		Text:       result,
		TokensUsed: estimateTokens(result),
		Duration:   time.Since(start),
		ModelName:  modelName,
		Finished:   true,
	}, nil
}

// estimateTokens 估算 token 数.
func estimateTokens(text string) int {
	// 粗略估算：平均 1 token ≈ 4 字符（英文）或 1.5 字符（中文）
	runes := []rune(text)
	return len(runes)*2/3 + 1
}

// truncateString 截断字符串.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
