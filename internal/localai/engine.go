// Package localai 提供本地AI推理引擎，支持完全离线的AI模型推理
package localai

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 错误定义
var (
	ErrModelNotFound      = errors.New("模型不存在")
	ErrModelNotLoaded     = errors.New("模型未加载")
	ErrInvalidInput       = errors.New("无效输入参数")
	ErrInferenceFailed    = errors.New("推理失败")
	ErrModelAlreadyExists = errors.New("模型已存在")
	ErrResourceExhausted  = errors.New("GPU/内存资源不足")
)

// ModelType 模型类型
type ModelType string

const (
	ModelTypeLLM       ModelType = "llm"       // 大语言模型
	ModelTypeVision    ModelType = "vision"    // 视觉模型
	ModelTypeEmbedding ModelType = "embedding" // 嵌入模型
	ModelTypeAudio     ModelType = "audio"     // 音频模型
	ModelTypeCustom    ModelType = "custom"    // 自定义模型
)

// ModelStatus 模型状态
type ModelStatus string

const (
	StatusAvailable ModelStatus = "available" // 可用
	StatusLoading   ModelStatus = "loading"   // 加载中
	StatusReady     ModelStatus = "ready"     // 就绪
	StatusError     ModelStatus = "error"     // 错误
	StatusUnloading ModelStatus = "unloading" // 卸载中
)

// InferenceBackend 推理后端
type InferenceBackend string

const (
	BackendCPU      InferenceBackend = "cpu"      // CPU推理
	BackendCUDA     InferenceBackend = "cuda"     // NVIDIA CUDA
	BackendROCm     InferenceBackend = "rocm"     // AMD ROCm
	BackendMetal    InferenceBackend = "metal"    // Apple Metal
	BackendVulkan   InferenceBackend = "vulkan"   // Vulkan
	BackendOpenVINO InferenceBackend = "openvino" // Intel OpenVINO
	BackendAuto     InferenceBackend = "auto"     // 自动选择
)

// Quantization 量化类型
type Quantization string

const (
	QuantNone Quantization = "none" // 无量化
	QuantINT8 Quantization = "int8" // INT8量化
	QuantINT4 Quantization = "int4" // INT4量化
	QuantFP16 Quantization = "fp16" // FP16半精度
	QuantBF16 Quantization = "bf16" // BF16
)

// Model 模型定义
type Model struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Type          ModelType         `json:"type"`
	Status        ModelStatus       `json:"status"`
	Backend       InferenceBackend  `json:"backend"`
	Quantization  Quantization      `json:"quantization"`
	ModelPath     string            `json:"model_path"`
	ConfigPath    string            `json:"config_path,omitempty"`
	Parameters    int64             `json:"parameters"`     // 参数量 (B)
	ContextLength int               `json:"context_length"` // 上下文长度
	MaxBatchSize  int               `json:"max_batch_size"`
	GPUMemoryMB   int64             `json:"gpu_memory_mb"` // GPU显存需求
	SystemMemoryMB int64            `json:"system_memory_mb"` // 系统内存需求
	VRAMUsageMB   int64             `json:"vram_usage_mb"` // 当前显存占用
	Metadata      map[string]string `json:"metadata,omitempty"`
	LoadedAt      *time.Time        `json:"loaded_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ModelID       string            `json:"model_id" binding:"required"`
	Prompt        string            `json:"prompt" binding:"required"`
	SystemPrompt  string            `json:"system_prompt,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   float64           `json:"temperature,omitempty"`
	TopP          float64           `json:"top_p,omitempty"`
	TopK          int               `json:"top_k,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Images        []string          `json:"images,omitempty"` // Base64编码的图片
	AudioData     []byte            `json:"audio_data,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// InferenceResponse 推理响应
type InferenceResponse struct {
	ID           string        `json:"id"`
	ModelID      string        `json:"model_id"`
	Text         string        `json:"text,omitempty"`
	Tokens       int           `json:"tokens"`
	PromptTokens int           `json:"prompt_tokens"`
	TotalTokens  int           `json:"total_tokens"`
	Duration     time.Duration `json:"duration"`
	TokensPerSec float64       `json:"tokens_per_sec"`
	FinishReason string        `json:"finish_reason"` // stop, length, error
	Error        string        `json:"error,omitempty"`
}

// StreamChunk 流式推理块
type StreamChunk struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	ModelID  string   `json:"model_id" binding:"required"`
	Texts    []string `json:"texts" binding:"required,min=1"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	ModelID    string      `json:"model_id"`
	Embeddings [][]float64 `json:"embeddings"`
	Dimensions int         `json:"dimensions"`
	Tokens     int         `json:"tokens"`
	Duration   time.Duration `json:"duration"`
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	GPUTotalMB     int64   `json:"gpu_total_mb"`
	GPUUsedMB      int64   `json:"gpu_used_mb"`
	GPUFreeMB      int64   `json:"gpu_free_mb"`
	RAMTotalMB     int64   `json:"ram_total_mb"`
	RAMUsedMB      int64   `json:"ram_used_mb"`
	RAMFreeMB      int64   `json:"ram_free_mb"`
	GPUUtilization float64 `json:"gpu_utilization"`
	CPUUtilization float64 `json:"cpu_utilization"`
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalModels      int           `json:"total_models"`
	LoadedModels     int           `json:"loaded_models"`
	TotalInferences  int64         `json:"total_inferences"`
	TotalTokens      int64         `json:"total_tokens"`
	AvgLatencyMs     float64       `json:"avg_latency_ms"`
	AvgTokensPerSec  float64       `json:"avg_tokens_per_sec"`
	UptimeSeconds    int64         `json:"uptime_seconds"`
	LastInferenceAt  *time.Time    `json:"last_inference_at,omitempty"`
}

// Engine 本地AI推理引擎
type Engine struct {
	mu           sync.RWMutex
	models       map[string]*Model
	inferences   []*InferenceRecord
	stats        EngineStats
	startTime    time.Time
	maxModels    int
	gpuDevices   []GPUDevice
}

// GPUDevice GPU设备信息
type GPUDevice struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	MemoryMB   int64  `json:"memory_mb"`
	UsedMB     int64  `json:"used_mb"`
	FreeMB     int64  `json:"free_mb"`
	Temperature int   `json:"temperature"`
	Utilization float64 `json:"utilization"`
}

// InferenceRecord 推理记录
type InferenceRecord struct {
	RequestID string        `json:"request_id"`
	ModelID   string        `json:"model_id"`
	Tokens    int           `json:"tokens"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// NewEngine 创建推理引擎
func NewEngine(maxModels int) *Engine {
	if maxModels <= 0 {
		maxModels = 10
	}
	return &Engine{
		models:     make(map[string]*Model),
		inferences: make([]*InferenceRecord, 0),
		startTime:  time.Now(),
		maxModels:  maxModels,
		gpuDevices: make([]GPUDevice, 0),
	}
}

// RegisterModel 注册模型
func (e *Engine) RegisterModel(model *Model) error {
	if model == nil || model.ID == "" || model.Name == "" {
		return ErrInvalidInput
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.models[model.ID]; exists {
		return ErrModelAlreadyExists
	}

	if len(e.models) >= e.maxModels {
		return ErrResourceExhausted
	}

	model.Status = StatusAvailable
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()
	e.models[model.ID] = model
	e.stats.TotalModels = len(e.models)

	return nil
}

// UnregisterModel 注销模型
func (e *Engine) UnregisterModel(modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelID]
	if !exists {
		return ErrModelNotFound
	}

	if model.Status == StatusReady {
		model.Status = StatusUnloading
	}

	delete(e.models, modelID)
	e.stats.TotalModels = len(e.models)
	return nil
}

// LoadModel 加载模型到内存/GPU
func (e *Engine) LoadModel(modelID string) error {
	e.mu.Lock()
	model, exists := e.models[modelID]
	if !exists {
		e.mu.Unlock()
		return ErrModelNotFound
	}

	if model.Status == StatusReady {
		e.mu.Unlock()
		return nil
	}

	model.Status = StatusLoading
	e.mu.Unlock()

	// 模拟加载过程
	time.Sleep(100 * time.Millisecond)

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	model.Status = StatusReady
	model.LoadedAt = &now
	model.UpdatedAt = now
	e.stats.LoadedModels++

	return nil
}

// UnloadModel 卸载模型
func (e *Engine) UnloadModel(modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelID]
	if !exists {
		return ErrModelNotFound
	}

	if model.Status != StatusReady {
		return ErrModelNotLoaded
	}

	model.Status = StatusUnloading
	model.UpdatedAt = time.Now()

	// 模拟卸载
	time.Sleep(50 * time.Millisecond)

	model.Status = StatusAvailable
	model.LoadedAt = nil
	model.VRAMUsageMB = 0
	e.stats.LoadedModels--

	return nil
}

// GetModel 获取模型信息
func (e *Engine) GetModel(modelID string) (*Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	model, exists := e.models[modelID]
	if !exists {
		return nil, ErrModelNotFound
	}
	return model, nil
}

// ListModels 列出所有模型
func (e *Engine) ListModels(modelType *ModelType) []*Model {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Model, 0, len(e.models))
	for _, model := range e.models {
		if modelType == nil || model.Type == *modelType {
			result = append(result, model)
		}
	}
	return result
}

// Inference 执行推理
func (e *Engine) Inference(req *InferenceRequest) (*InferenceResponse, error) {
	if req == nil || req.ModelID == "" || req.Prompt == "" {
		return nil, ErrInvalidInput
	}

	e.mu.RLock()
	model, exists := e.models[req.ModelID]
	if !exists {
		e.mu.RUnlock()
		return nil, ErrModelNotFound
	}
	if model.Status != StatusReady {
		e.mu.RUnlock()
		return nil, ErrModelNotLoaded
	}
	e.mu.RUnlock()

	start := time.Now()

	// 模拟推理
	time.Sleep(50 * time.Millisecond)

	duration := time.Since(start)
	promptTokens := len(req.Prompt) / 4 // 粗略估算
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}
	generatedTokens := maxTokens / 2

	resp := &InferenceResponse{
		ID:           fmt.Sprintf("inf-%d", time.Now().UnixNano()),
		ModelID:      req.ModelID,
		Text:         fmt.Sprintf("[本地AI推理] 基于模型 %s 的响应: 处理了 %d 个token", model.Name, promptTokens),
		Tokens:       generatedTokens,
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens + generatedTokens,
		Duration:     duration,
		TokensPerSec: float64(generatedTokens) / duration.Seconds(),
		FinishReason: "stop",
	}

	// 记录推理
	e.mu.Lock()
	e.inferences = append(e.inferences, &InferenceRecord{
		RequestID: resp.ID,
		ModelID:   req.ModelID,
		Tokens:    resp.TotalTokens,
		Duration:  duration,
		Success:   true,
		Timestamp: time.Now(),
	})
	e.stats.TotalInferences++
	e.stats.TotalTokens += int64(resp.TotalTokens)
	now := time.Now()
	e.stats.LastInferenceAt = &now
	e.mu.Unlock()

	return resp, nil
}

// Embedding 计算嵌入向量
func (e *Engine) Embedding(req *EmbeddingRequest) (*EmbeddingResponse, error) {
	if req == nil || req.ModelID == "" || len(req.Texts) == 0 {
		return nil, ErrInvalidInput
	}

	e.mu.RLock()
	model, exists := e.models[req.ModelID]
	if !exists {
		e.mu.RUnlock()
		return nil, ErrModelNotFound
	}
	if model.Status != StatusReady {
		e.mu.RUnlock()
		return nil, ErrModelNotLoaded
	}
	if model.Type != ModelTypeEmbedding {
		e.mu.RUnlock()
		return nil, fmt.Errorf("模型类型 %s 不支持嵌入计算", model.Type)
	}
	e.mu.RUnlock()

	start := time.Now()

	// 模拟嵌入计算
	time.Sleep(30 * time.Millisecond)

	dimensions := 768 // 标准维度
	embeddings := make([][]float64, len(req.Texts))
	for i := range embeddings {
		embedding := make([]float64, dimensions)
		for j := range embedding {
			embedding[j] = float64(i*dimensions+j) * 0.001
		}
		embeddings[i] = embedding
	}

	duration := time.Since(start)
	totalTokens := 0
	for _, text := range req.Texts {
		totalTokens += len(text) / 4
	}

	return &EmbeddingResponse{
		ModelID:    req.ModelID,
		Embeddings: embeddings,
		Dimensions: dimensions,
		Tokens:     totalTokens,
		Duration:   duration,
	}, nil
}

// GetResourceInfo 获取资源信息
func (e *Engine) GetResourceInfo() *ResourceInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info := &ResourceInfo{
		GPUTotalMB:     8192,
		RAMTotalMB:     16384,
		GPUUtilization: 0,
		CPUUtilization: 0,
	}

	for _, model := range e.models {
		if model.Status == StatusReady {
			info.GPUUsedMB += model.VRAMUsageMB
			info.RAMUsedMB += model.SystemMemoryMB
		}
	}

	info.GPUFreeMB = info.GPUTotalMB - info.GPUUsedMB
	info.RAMFreeMB = info.RAMTotalMB - info.RAMUsedMB

	return info
}

// GetStats 获取引擎统计
func (e *Engine) GetStats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := e.stats
	stats.UptimeSeconds = int64(time.Since(e.startTime).Seconds())

	if stats.TotalInferences > 0 {
		var totalDuration time.Duration
		var totalTokensPerSec float64
		for _, rec := range e.inferences {
			totalDuration += rec.Duration
			if rec.Duration.Seconds() > 0 {
				totalTokensPerSec += float64(rec.Tokens) / rec.Duration.Seconds()
			}
		}
		stats.AvgLatencyMs = float64(totalDuration.Milliseconds()) / float64(stats.TotalInferences)
		stats.AvgTokensPerSec = totalTokensPerSec / float64(stats.TotalInferences)
	}

	return &stats
}

// GetInferenceHistory 获取推理历史
func (e *Engine) GetInferenceHistory(modelID string, limit int) []*InferenceRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*InferenceRecord, 0)
	for i := len(e.inferences) - 1; i >= 0; i-- {
		if modelID == "" || e.inferences[i].ModelID == modelID {
			result = append(result, e.inferences[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// SetGPUDevice 设置GPU设备信息
func (e *Engine) SetGPUDevice(device GPUDevice) {
	e.mu.Lock()
	defer e.mu.Unlock()

	found := false
	for i, d := range e.gpuDevices {
		if d.Index == device.Index {
			e.gpuDevices[i] = device
			found = true
			break
		}
	}
	if !found {
		e.gpuDevices = append(e.gpuDevices, device)
	}
}

// GetGPUDevices 获取GPU设备列表
func (e *Engine) GetGPUDevices() []GPUDevice {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gpuDevices
}
