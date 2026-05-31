// Package edgeinference 实现边缘AI推理平台。
// 支持本地模型加载、推理调度、多模型管理、GPU加速、批量推理。
package edgeinference

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ModelType 模型类型
type ModelType string

const (
	ModelTypeLLM        ModelType = "llm"         // 大语言模型
	ModelTypeVision     ModelType = "vision"      // 视觉模型
	ModelTypeAudio      ModelType = "audio"       // 音频模型
	ModelTypeEmbedding  ModelType = "embedding"   // 嵌入模型
	ModelTypeDetection  ModelType = "detection"   // 目标检测
	ModelTypeSegmentation ModelType = "segmentation" // 图像分割
	ModelTypeOCR        ModelType = "ocr"         // 文字识别
	ModelTypeTTS        ModelType = "tts"         // 语音合成
)

// ModelStatus 模型状态
type ModelStatus string

const (
	ModelStatusLoading  ModelStatus = "loading"
	ModelStatusReady    ModelStatus = "ready"
	ModelStatusRunning  ModelStatus = "running"
	ModelStatusError    ModelStatus = "error"
	ModelStatusUnloaded ModelStatus = "unloaded"
)

// ComputeDevice 计算设备
type ComputeDevice string

const (
	DeviceCPU  ComputeDevice = "cpu"
	DeviceGPU  ComputeDevice = "gpu"
	DeviceNPU  ComputeDevice = "npu"  // 神经网络处理器
	DeviceAuto ComputeDevice = "auto"
)

// InferenceModel 推理模型
type InferenceModel struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Type          ModelType     `json:"type"`
	Version       string        `json:"version"`
	Path          string        `json:"path"`
	Status        ModelStatus   `json:"status"`
	Device        ComputeDevice `json:"device"`
	InputFormat   string        `json:"inputFormat"`
	OutputFormat  string        `json:"outputFormat"`
	MaxBatchSize  int           `json:"maxBatchSize"`
	LoadedAt      time.Time     `json:"loadedAt,omitempty"`
	LastUsedAt    time.Time     `json:"lastUsedAt,omitempty"`
	InferCount    int64         `json:"inferCount"`
	TotalLatency  int64         `json:"totalLatencyMs"`
	AvgLatency    float64       `json:"avgLatencyMs"`
	MemoryMB      int           `json:"memoryMB"`
	GPUMemoryMB   int           `json:"gpuMemoryMB"`
}

// InferRequest 推理请求
type InferRequest struct {
	ModelID   string                 `json:"modelId"`
	Input     interface{}            `json:"input"`
	Params    map[string]interface{} `json:"params,omitempty"`
	BatchSize int                    `json:"batchSize,omitempty"`
	Device    ComputeDevice          `json:"device,omitempty"`
	Timeout   int                    `json:"timeoutSec,omitempty"`
}

// InferResponse 推理响应
type InferResponse struct {
	ID        string      `json:"id"`
	ModelID   string      `json:"modelId"`
	Output    interface{} `json:"output"`
	LatencyMs int64       `json:"latencyMs"`
	Device    string      `json:"device"`
	Timestamp time.Time   `json:"timestamp"`
	BatchSize int         `json:"batchSize"`
}

// InferenceEngine 推理引擎
type InferenceEngine struct {
	mu        sync.RWMutex
	models    map[string]*InferenceModel
	jobQueue  chan InferJob
	results   map[string]chan InferResponse
	workers   int
	quit      chan struct{}
	running   bool
	gpuAvail  bool
}

// InferJob 推理任务
type InferJob struct {
	Request  InferRequest
	ResultCh chan InferResponse
	ErrorCh  chan error
}

// NewInferenceEngine 创建推理引擎
func NewInferenceEngine(workers int, gpuAvail bool) *InferenceEngine {
	if workers <= 0 {
		workers = 4
	}
	return &InferenceEngine{
		models:   make(map[string]*InferenceModel),
		jobQueue: make(chan InferJob, 1000),
		results:  make(map[string]chan InferResponse),
		workers:  workers,
		quit:     make(chan struct{}),
		gpuAvail: gpuAvail,
	}
}

// LoadModel 加载模型
func (e *InferenceEngine) LoadModel(model InferenceModel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if model.ID == "" {
		return fmt.Errorf("模型ID不能为空")
	}

	model.Status = ModelStatusLoading
	model.LoadedAt = time.Now()

	// 模拟模型加载
	model.Status = ModelStatusReady
	e.models[model.ID] = &model
	return nil
}

// UnloadModel 卸载模型
func (e *InferenceEngine) UnloadModel(modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, ok := e.models[modelID]
	if !ok {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}
	model.Status = ModelStatusUnloaded
	return nil
}

// GetModel 获取模型信息
func (e *InferenceEngine) GetModel(modelID string) (*InferenceModel, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	model, ok := e.models[modelID]
	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}
	return model, nil
}

// ListModels 列出所有模型
func (e *InferenceEngine) ListModels(modelType ModelType) []InferenceModel {
	e.mu.RLock()
	defer e.mu.RUnlock()

	models := make([]InferenceModel, 0)
	for _, m := range e.models {
		if modelType == "" || m.Type == modelType {
			models = append(models, *m)
		}
	}
	return models
}

// Infer 执行推理
func (e *InferenceEngine) Infer(req InferRequest) (*InferResponse, error) {
	e.mu.RLock()
	model, ok := e.models[req.ModelID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", req.ModelID)
	}
	if model.Status != ModelStatusReady && model.Status != ModelStatusRunning {
		return nil, fmt.Errorf("模型 %s 状态异常: %s", req.ModelID, model.Status)
	}

	start := time.Now()

	// 选择计算设备
	device := req.Device
	if device == "" || device == DeviceAuto {
		if e.gpuAvail && model.GPUMemoryMB > 0 {
			device = DeviceGPU
		} else {
			device = DeviceCPU
		}
	}

	// 模拟推理
	output := map[string]interface{}{
		"model":   model.Name,
		"type":    model.Type,
		"input":   req.Input,
		"device":  device,
		"message": "推理完成（模拟）",
	}

	latency := time.Since(start).Milliseconds()

	// 更新模型统计
	e.mu.Lock()
	model.InferCount++
	model.TotalLatency += latency
	model.AvgLatency = float64(model.TotalLatency) / float64(model.InferCount)
	model.LastUsedAt = time.Now()
	model.Status = ModelStatusReady
	e.mu.Unlock()

	return &InferResponse{
		ID:        fmt.Sprintf("infer_%d", time.Now().UnixNano()),
		ModelID:   req.ModelID,
		Output:    output,
		LatencyMs: latency,
		Device:    string(device),
		Timestamp: time.Now(),
		BatchSize: 1,
	}, nil
}

// Start 启动推理引擎
func (e *InferenceEngine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	for i := 0; i < e.workers; i++ {
		go e.worker(i)
	}
}

// Stop 停止推理引擎
func (e *InferenceEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}
	e.running = false
	close(e.quit)
}

// worker 工作线程
func (e *InferenceEngine) worker(id int) {
	for {
		select {
		case job := <-e.jobQueue:
			resp, err := e.Infer(job.Request)
			if err != nil {
				select {
				case job.ErrorCh <- err:
				default:
				}
			} else {
				select {
				case job.ResultCh <- *resp:
				default:
				}
			}
		case <-e.quit:
			return
		}
	}
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalModels    int   `json:"totalModels"`
	ReadyModels    int   `json:"readyModels"`
	TotalInferences int64 `json:"totalInferences"`
	GPUAvailable   bool  `json:"gpuAvailable"`
	Workers        int   `json:"workers"`
	QueueLength    int   `json:"queueLength"`
}

// GetStats 获取引擎统计
func (e *InferenceEngine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := EngineStats{
		TotalModels:  len(e.models),
		GPUAvailable: e.gpuAvail,
		Workers:      e.workers,
		QueueLength:  len(e.jobQueue),
	}
	for _, m := range e.models {
		if m.Status == ModelStatusReady {
			stats.ReadyModels++
		}
		stats.TotalInferences += m.InferCount
	}
	return stats
}

// RegisterRoutes 注册 HTTP 路由
func (e *InferenceEngine) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/inference/models", e.handleModels)
	mux.HandleFunc("/api/v1/inference/infer", e.handleInfer)
	mux.HandleFunc("/api/v1/inference/stats", e.handleStats)
}

func (e *InferenceEngine) handleModels(w http.ResponseWriter, r *http.Request) {
	models := e.ListModels("")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (e *InferenceEngine) handleInfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req InferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	resp, err := e.Infer(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (e *InferenceEngine) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := e.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
