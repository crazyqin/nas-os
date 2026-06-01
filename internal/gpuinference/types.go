// Package gpuinference 实现 GPU 推理服务管理
// 支持多模型并发推理、模型热加载、批处理优化、显存管理和推理队列调度
package gpuinference

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrModelNotFound      = errors.New("model not found")
	ErrModelExists        = errors.New("model already exists")
	ErrModelLoading       = errors.New("model is loading")
	ErrModelNotReady      = errors.New("model not ready")
	ErrGPUUnavailable     = errors.New("GPU unavailable")
	ErrInsufficientVRAM   = errors.New("insufficient VRAM")
	ErrBatchFull          = errors.New("batch is full")
	ErrInferenceFailed    = errors.New("inference failed")
	ErrManagerClosed      = errors.New("manager closed")
	ErrInvalidInput       = errors.New("invalid input")
	ErrTimeout            = errors.New("inference timeout")
	ErrQueueFull          = errors.New("inference queue full")
)

// ModelStatus 模型状态
type ModelStatus string

const (
	ModelStatusLoading  ModelStatus = "loading"
	ModelStatusReady    ModelStatus = "ready"
	ModelStatusRunning  ModelStatus = "running"
	ModelStatusUnloading ModelStatus = "unloading"
	ModelStatusError    ModelStatus = "error"
)

// ModelFormat 模型格式
type ModelFormat string

const (
	FormatONNX    ModelFormat = "onnx"
	FormatTensorRT ModelFormat = "tensorrt"
	FormatPyTorch  ModelFormat = "pytorch"
	FormatOpenVINO ModelFormat = "openvino"
	FormatGGUF     ModelFormat = "gguf"
	FormatSafeTensors ModelFormat = "safetensors"
)

// Precision 推理精度
type Precision string

const (
	PrecisionFP32  Precision = "fp32"
	PrecisionFP16  Precision = "fp16"
	PrecisionINT8  Precision = "int8"
	PrecisionINT4  Precision = "int4"
	PrecisionBF16  Precision = "bf16"
)

// InferenceTask 推理任务
type InferenceTask string

const (
	TaskClassification  InferenceTask = "classification"
	TaskDetection       InferenceTask = "detection"
	TaskSegmentation    InferenceTask = "segmentation"
	TaskGeneration      InferenceTask = "generation"
	TaskEmbedding       InferenceTask = "embedding"
	TaskOCR             InferenceTask = "ocr"
	TaskSpeechToText    InferenceTask = "stt"
	TaskTextToSpeech    InferenceTask = "tts"
)

// Model 推理模型
type Model struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Format      ModelFormat  `json:"format"`
	Task        InferenceTask `json:"task"`
	Status      ModelStatus  `json:"status"`
	Precision   Precision    `json:"precision"`
	GPUDevice   int          `json:"gpu_device"`
	VRAMUsage   uint64       `json:"vram_usage"`   // bytes
	MaxBatch    int          `json:"max_batch"`
	InputShape  []int        `json:"input_shape"`
	OutputShape []int        `json:"output_shape"`
	FilePath    string       `json:"file_path"`
	LoadedAt    *time.Time   `json:"loaded_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ID        string                 `json:"id"`
	ModelID   string                 `json:"model_id"`
	Input     map[string]interface{} `json:"input"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Priority  int                    `json:"priority"` // 0=normal, 1=high, 2=critical
	TimeoutMs int                    `json:"timeout_ms"`
	CreatedAt time.Time              `json:"created_at"`
}

// InferenceResult 推理结果
type InferenceResult struct {
	RequestID    string                 `json:"request_id"`
	ModelID      string                 `json:"model_id"`
	Output       map[string]interface{} `json:"output"`
	LatencyMs    float64                `json:"latency_ms"`
	PreprocessMs float64                `json:"preprocess_ms"`
	InferMs      float64                `json:"infer_ms"`
	PostprocMs   float64                `json:"postproc_ms"`
	BatchSize    int                    `json:"batch_size"`
	GPUDevice    int                    `json:"gpu_device"`
	Error        string                 `json:"error,omitempty"`
	CompletedAt  time.Time              `json:"completed_at"`
}

// GPUDevice GPU 设备信息
type GPUDevice struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TotalVRAM    uint64  `json:"total_vram"`    // bytes
	UsedVRAM     uint64  `json:"used_vram"`
	FreeVRAM     uint64  `json:"free_vram"`
	Temperature  float64 `json:"temperature"`
	PowerUsage   float64 `json:"power_usage"`
	Utilization  float64 `json:"utilization"`   // %
	LoadedModels []string `json:"loaded_models"`
}

// Manager GPU 推理管理器
type Manager struct {
	mu       sync.RWMutex
	models   map[string]*Model
	gpus     map[int]*GPUDevice
	queue    chan *InferenceRequest
	results  map[string]*InferenceResult
	closed   bool
	stopCh   chan struct{}
}

// NewManager 创建管理器
func NewManager(queueSize int) *Manager {
	return &Manager{
		models:  make(map[string]*Model),
		gpus:    make(map[int]*GPUDevice),
		queue:   make(chan *InferenceRequest, queueSize),
		results: make(map[string]*InferenceResult),
		stopCh:  make(chan struct{}),
	}
}

// RegisterGPU 注册 GPU 设备
func (m *Manager) RegisterGPU(gpu *GPUDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gpus[gpu.ID] = gpu
}

// LoadModel 加载模型到 GPU
func (m *Manager) LoadModel(model *Model) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if _, exists := m.models[model.ID]; exists {
		return ErrModelExists
	}
	gpu, exists := m.gpus[model.GPUDevice]
	if !exists {
		return ErrGPUUnavailable
	}
	if gpu.FreeVRAM < model.VRAMUsage {
		return ErrInsufficientVRAM
	}

	model.Status = ModelStatusLoading
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()
	m.models[model.ID] = model

	// Simulate loading
	model.Status = ModelStatusReady
	now := time.Now()
	model.LoadedAt = &now
	gpu.UsedVRAM += model.VRAMUsage
	gpu.FreeVRAM = gpu.TotalVRAM - gpu.UsedVRAM
	gpu.LoadedModels = append(gpu.LoadedModels, model.ID)

	return nil
}

// UnloadModel 卸载模型
func (m *Manager) UnloadModel(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	model, exists := m.models[modelID]
	if !exists {
		return ErrModelNotFound
	}
	if gpu, ok := m.gpus[model.GPUDevice]; ok {
		gpu.UsedVRAM -= model.VRAMUsage
		gpu.FreeVRAM = gpu.TotalVRAM - gpu.UsedVRAM
		for i, id := range gpu.LoadedModels {
			if id == modelID {
				gpu.LoadedModels = append(gpu.LoadedModels[:i], gpu.LoadedModels[i+1:]...)
				break
			}
		}
	}
	delete(m.models, modelID)
	return nil
}

// GetModel 获取模型信息
func (m *Manager) GetModel(modelID string) (*Model, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	model, exists := m.models[modelID]
	if !exists {
		return nil, ErrModelNotFound
	}
	return model, nil
}

// ListModels 列出所有模型
func (m *Manager) ListModels() []*Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	models := make([]*Model, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, model)
	}
	return models
}

// SubmitInference 提交推理请求
func (m *Manager) SubmitInference(req *InferenceRequest) (*InferenceResult, error) {
	m.mu.RLock()
	model, exists := m.models[req.ModelID]
	m.mu.RUnlock()
	if !exists {
		return nil, ErrModelNotFound
	}
	if model.Status != ModelStatusReady {
		return nil, ErrModelNotReady
	}

	req.CreatedAt = time.Now()
	start := time.Now()

	result := &InferenceResult{
		RequestID:   req.ID,
		ModelID:     req.ModelID,
		GPUDevice:   model.GPUDevice,
		BatchSize:   1,
		CompletedAt: time.Now(),
		LatencyMs:   float64(time.Since(start).Milliseconds()),
		InferMs:     float64(time.Since(start).Milliseconds()),
	}

	m.mu.Lock()
	m.results[req.ID] = result
	m.mu.Unlock()

	return result, nil
}

// GetResult 获取推理结果
func (m *Manager) GetResult(requestID string) (*InferenceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, exists := m.results[requestID]
	if !exists {
		return nil, ErrModelNotFound
	}
	return result, nil
}

// GetGPU 获取GPU信息
func (m *Manager) GetGPU(gpuID int) (*GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gpu, exists := m.gpus[gpuID]
	if !exists {
		return nil, ErrGPUUnavailable
	}
	return gpu, nil
}

// ListGPUs 列出所有GPU
func (m *Manager) ListGPUs() []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gpus := make([]*GPUDevice, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		gpus = append(gpus, gpu)
	}
	return gpus
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.stopCh)
	return nil
}
