package gpuaccel2

import (
	"sync"
	"time"
)

// GPUDevice GPU设备信息。
type GPUDevice struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	MemoryMB   int64   `json:"memory_mb"`
	UsedMB     int64   `json:"used_mb"`
	TempC      int     `json:"temp_c"`
	PowerW     int     `json:"power_w"`
	UtilPct    float64 `json:"util_pct"`
	DriverVer  string  `json:"driver_ver"`
	Status     GPUStatus `json:"status"`
}

// GPUStatus GPU状态。
type GPUStatus string

const (
	GPUStatusOnline  GPUStatus = "online"
	GPUStatusOffline GPUStatus = "offline"
	GPUStatusError   GPUStatus = "error"
)

// Model 模型信息。
type Model struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Family     string    `json:"family"`      // llama, qwen, etc.
	SizeMB     int64     `json:"size_mb"`
	QuantType  string    `json:"quant_type"`  // Q4_K_M, Q8_0, etc.
	GPUID      string    `json:"gpu_id"`
	Status     ModelStatus `json:"status"`
	LoadedAt   time.Time `json:"loaded_at"`
}

// ModelStatus 模型状态。
type ModelStatus string

const (
	ModelStatusLoading ModelStatus = "loading"
	ModelStatusReady   ModelStatus = "ready"
	ModelStatusError   ModelStatus = "error"
)

// InferenceRequest 推理请求。
type InferenceRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	MaxTokens int  `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// InferenceResponse 推理响应。
type InferenceResponse struct {
	Text       string  `json:"text"`
	Tokens     int     `json:"tokens"`
	DurationMs int64   `json:"duration_ms"`
	ModelID    string  `json:"model_id"`
}

// Stats 推理统计。
type Stats struct {
	TotalInferences int64   `json:"total_inferences"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	TotalTokens     int64   `json:"total_tokens"`
	GPUMemoryUsedMB int64   `json:"gpu_memory_used_mb"`
}

// Engine GPU加速推理引擎。
type Engine struct {
	mu      sync.RWMutex
	gpus    map[string]*GPUDevice
	models  map[string]*Model
	stats   Stats
}

// NewEngine 创建新的推理引擎。
func NewEngine() *Engine {
	return &Engine{
		gpus:   make(map[string]*GPUDevice),
		models: make(map[string]*Model),
	}
}

// AddGPU 添加GPU设备。
func (e *Engine) AddGPU(gpu *GPUDevice) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gpus[gpu.ID] = gpu
}

// GetGPU 获取GPU设备。
func (e *Engine) GetGPU(id string) (*GPUDevice, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	gpu, exists := e.gpus[id]
	return gpu, exists
}

// ListGPUs 列出所有GPU。
func (e *Engine) ListGPUs() []*GPUDevice {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*GPUDevice, 0, len(e.gpus))
	for _, gpu := range e.gpus {
		result = append(result, gpu)
	}
	return result
}

// LoadModel 加载模型到GPU。
func (e *Engine) LoadModel(model *Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	gpu, exists := e.gpus[model.GPUID]
	if !exists {
		return ErrGPUNotFound
	}
	if gpu.UsedMB+model.SizeMB > gpu.MemoryMB {
		return ErrInsufficientMemory
	}

	model.Status = ModelStatusReady
	model.LoadedAt = time.Now()
	e.models[model.ID] = model
	gpu.UsedMB += model.SizeMB
	return nil
}

// UnloadModel 卸载模型。
func (e *Engine) UnloadModel(modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelID]
	if !exists {
		return ErrModelNotFound
	}

	if gpu, ok := e.gpus[model.GPUID]; ok {
		gpu.UsedMB -= model.SizeMB
	}
	delete(e.models, modelID)
	return nil
}

// GetModel 获取模型信息。
func (e *Engine) GetModel(id string) (*Model, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	model, exists := e.models[id]
	return model, exists
}

// ListModels 列出所有模型。
func (e *Engine) ListModels() []*Model {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Model, 0, len(e.models))
	for _, m := range e.models {
		result = append(result, m)
	}
	return result
}

// Infer 执行推理（模拟）。
func (e *Engine) Infer(req InferenceRequest) (*InferenceResponse, error) {
	e.mu.RLock()
	model, exists := e.models[req.ModelID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrModelNotFound
	}
	if model.Status != ModelStatusReady {
		return nil, ErrModelNotReady
	}

	// 模拟推理
	start := time.Now()
	time.Sleep(10 * time.Millisecond) // 模拟延迟

	e.mu.Lock()
	e.stats.TotalInferences++
	e.stats.TotalTokens += int64(req.MaxTokens)
	e.mu.Unlock()

	return &InferenceResponse{
		Text:       "Simulated response for: " + req.Prompt,
		Tokens:     req.MaxTokens,
		DurationMs: time.Since(start).Milliseconds(),
		ModelID:    req.ModelID,
	}, nil
}

// GetStats 获取统计信息。
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// 错误定义。
var (
	ErrGPUNotFound        = &AccelError{"GPU not found"}
	ErrInsufficientMemory = &AccelError{"insufficient GPU memory"}
	ErrModelNotFound      = &AccelError{"model not found"}
	ErrModelNotReady      = &AccelError{"model not ready"}
)

type AccelError struct {
	msg string
}

func (e *AccelError) Error() string {
	return e.msg
}
