// Package edgeai 提供边缘 AI 推理引擎，支持在 NAS 本地运行 AI 模型推理
package edgeai

import (
	"fmt"
	"sync"
	"time"
)

// ModelFormat 模型格式
type ModelFormat string

const (
	ModelFormatONNX    ModelFormat = "onnx"    // ONNX 格式
	ModelFormatTFLite  ModelFormat = "tflite"  // TensorFlow Lite 格式
	ModelFormatPyTorch ModelFormat = "pytorch" // PyTorch 格式
	ModelFormatTensorRT ModelFormat = "tensorrt" // TensorRT 格式
)

// ComputeDevice 计算设备
type ComputeDevice string

const (
	ComputeDeviceCPU ComputeDevice = "cpu" // CPU 计算
	ComputeDeviceGPU ComputeDevice = "gpu" // GPU 计算
	ComputeDeviceNPU ComputeDevice = "npu" // NPU 计算
)

// ModelStatus 模型状态
type ModelStatus string

const (
	ModelStatusUnloaded ModelStatus = "unloaded" // 未加载
	ModelStatusLoading  ModelStatus = "loading"  // 加载中
	ModelStatusReady    ModelStatus = "ready"    // 就绪
	ModelStatusRunning  ModelStatus = "running"  // 运行中
	ModelStatusError    ModelStatus = "error"    // 错误
)

// TaskType 推理任务类型
type TaskType string

const (
	TaskTypeClassification TaskType = "classification" // 图像分类
	TaskTypeDetection      TaskType = "detection"      // 目标检测
	TaskTypeSegmentation   TaskType = "segmentation"   // 语义分割
	TaskTypeOCR            TaskType = "ocr"            // 文字识别
	TaskTypeNLP            TaskType = "nlp"            // 自然语言处理
	TaskTypeEmbedding      TaskType = "embedding"      // 向量嵌入
	TaskTypeCustom         TaskType = "custom"         // 自定义任务
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	TaskPriorityLow    TaskPriority = 0  // 低优先级
	TaskPriorityNormal TaskPriority = 1  // 普通优先级
	TaskPriorityHigh   TaskPriority = 2  // 高优先级
	TaskPriorityUrgent TaskPriority = 3  // 紧急优先级
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"    // 等待中
	TaskStatusQueued     TaskStatus = "queued"     // 已排队
	TaskStatusProcessing TaskStatus = "processing" // 处理中
	TaskStatusCompleted  TaskStatus = "completed"  // 已完成
	TaskStatusFailed     TaskStatus = "failed"     // 失败
	TaskStatusCancelled  TaskStatus = "cancelled"  // 已取消
)

// Model 模型定义
type Model struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Format      ModelFormat   `json:"format"`
	TaskType    TaskType      `json:"taskType"`
	Device      ComputeDevice `json:"device"`
	Status      ModelStatus   `json:"status"`
	FilePath    string        `json:"filePath"`
	InputShape  []int         `json:"inputShape"`
	OutputShape []int         `json:"outputShape"`
	Labels      []string      `json:"labels,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Config      *ModelConfig  `json:"config,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	LoadedAt    *time.Time    `json:"loadedAt,omitempty"`
	MemoryUsage int64         `json:"memoryUsage"` // bytes
	InferCount  int64         `json:"inferCount"`
	AvgLatency  float64       `json:"avgLatency"` // ms
}

// ModelConfig 模型配置
type ModelConfig struct {
	BatchSize    int    `json:"batchSize"`    // 批处理大小
	NumThreads   int    `json:"numThreads"`   // 线程数
	Precision    string `json:"precision"`    // 精度：fp32/fp16/int8
	Quantized    bool   `json:"quantized"`    // 是否量化
	OptimizeLevel int   `json:"optimizeLevel"` // 优化级别 0-3
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ID         string                 `json:"id"`
	ModelID    string                 `json:"modelId"`
	TaskType   TaskType               `json:"taskType"`
	Priority   TaskPriority           `json:"priority"`
	Input      *InferenceInput        `json:"input"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Callback   string                 `json:"callback,omitempty"` // 回调 URL
	Timeout    time.Duration          `json:"timeout"`
	CreatedAt  time.Time              `json:"createdAt"`
}

// InferenceInput 推理输入
type InferenceInput struct {
	Text     string            `json:"text,omitempty"`     // 文本输入
	Image    []byte            `json:"image,omitempty"`    // 图片数据
	ImageURL string            `json:"imageUrl,omitempty"` // 图片 URL
	Tensor   []float32         `json:"tensor,omitempty"`   // 张量数据
	Shape    []int             `json:"shape,omitempty"`    // 张量形状
	Metadata map[string]string `json:"metadata,omitempty"` // 元数据
}

// InferenceResult 推理结果
type InferenceResult struct {
	ID           string                 `json:"id"`
	RequestID    string                 `json:"requestId"`
	ModelID      string                 `json:"modelId"`
	TaskType     TaskType               `json:"taskType"`
	Status       TaskStatus             `json:"status"`
	Output       *InferenceOutput       `json:"output"`
	Latency      time.Duration          `json:"latency"`
	Device       ComputeDevice          `json:"device"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt  time.Time              `json:"completedAt"`
}

// InferenceOutput 推理输出
type InferenceOutput struct {
	Classes    []ClassificationResult `json:"classes,omitempty"`    // 分类结果
	Objects    []DetectionResult      `json:"objects,omitempty"`    // 检测结果
	Text       string                 `json:"text,omitempty"`       // 文本输出
	Embedding  []float32              `json:"embedding,omitempty"`  // 向量输出
	Tensor     []float32              `json:"tensor,omitempty"`     // 张量输出
	Shape      []int                  `json:"shape,omitempty"`      // 输出形状
	RawOutput  []byte                 `json:"rawOutput,omitempty"`  // 原始输出
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Index      int     `json:"index"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	BBox       BBox      `json:"bbox"`
}

// BBox 边界框
type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// EngineConfig 引擎配置
type EngineConfig struct {
	ModelDir        string          `json:"modelDir"`        // 模型存储目录
	MaxConcurrent   int             `json:"maxConcurrent"`   // 最大并发推理数
	MaxQueueSize    int             `json:"maxQueueSize"`    // 最大排队数
	DefaultDevice   ComputeDevice   `json:"defaultDevice"`   // 默认计算设备
	EnableGPU       bool            `json:"enableGPU"`       // 启用 GPU
	EnableNPU       bool            `json:"enableNPU"`       // 启用 NPU
	GPUDeviceID     int             `json:"gpuDeviceId"`     // GPU 设备 ID
	MemoryLimit     int64           `json:"memoryLimit"`     // 内存限制 (bytes)
	CacheSize       int             `json:"cacheSize"`       // 结果缓存大小
	CacheTTL        time.Duration   `json:"cacheTTL"`        // 缓存过期时间
	Workers         int             `json:"workers"`         // 工作线程数
	MonitorInterval time.Duration   `json:"monitorInterval"` // 监控间隔
	EnableProfiling bool            `json:"enableProfiling"` // 启用性能分析
}

// InferStats 推理统计
type InferStats struct {
	TotalRequests    int64         `json:"totalRequests"`
	SuccessRequests  int64         `json:"successRequests"`
	FailedRequests   int64         `json:"failedRequests"`
	QueuedRequests   int64         `json:"queuedRequests"`
	AvgLatency       float64       `json:"avgLatency"`       // ms
	P95Latency       float64       `json:"p95Latency"`       // ms
	P99Latency       float64       `json:"p99Latency"`       // ms
	TotalMemory      int64         `json:"totalMemory"`      // bytes
	GPUUtilization   float64       `json:"gpuUtilization"`   // 0-100%
	CPUUtilization   float64       `json:"cpuUtilization"`   // 0-100%
	ModelsLoaded     int           `json:"modelsLoaded"`
	Uptime           time.Duration `json:"uptime"`
	LastInferTime    time.Time     `json:"lastInferTime"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	CPU     CPUUsage     `json:"cpu"`
	GPU     GPUUsage     `json:"gpu"`
	Memory  MemoryUsage  `json:"memory"`
	Disk    DiskUsage    `json:"disk"`
}

// CPUUsage CPU 使用情况
type CPUUsage struct {
	Usage     float64 `json:"usage"`     // 0-100%
	Cores     int     `json:"cores"`
	Threads   int     `json:"threads"`
	Frequency float64 `json:"frequency"` // GHz
}

// GPUUsage GPU 使用情况
type GPUUsage struct {
	Available    bool    `json:"available"`
	Usage        float64 `json:"usage"`        // 0-100%
	MemoryUsed   int64   `json:"memoryUsed"`   // bytes
	MemoryTotal  int64   `json:"memoryTotal"`  // bytes
	Temperature  float64 `json:"temperature"`  // 摄氏度
	PowerUsage   float64 `json:"powerUsage"`   // 瓦特
}

// MemoryUsage 内存使用情况
type MemoryUsage struct {
	Total     int64   `json:"total"`     // bytes
	Used      int64   `json:"used"`      // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // 0-100%
}

// DiskUsage 磁盘使用情况
type DiskUsage struct {
	Total     int64   `json:"total"`     // bytes
	Used      int64   `json:"used"`      // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // 0-100%
}

// ModelVersion 模型版本
type ModelVersion struct {
	Version   string    `json:"version"`
	FilePath  string    `json:"filePath"`
	Checksum  string    `json:"checksum"`
	Size      int64     `json:"size"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	Notes     string    `json:"notes,omitempty"`
}

// ValidationError 参数校验错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("参数校验失败 [%s]: %s", e.Field, e.Message)
}

// Validate 校验 InferenceRequest
func (r *InferenceRequest) Validate() error {
	if r.ModelID == "" {
		return &ValidationError{Field: "modelId", Message: "不能为空"}
	}
	if r.Input == nil {
		return &ValidationError{Field: "input", Message: "不能为空"}
	}
	if r.Input.Text == "" && len(r.Input.Image) == 0 && len(r.Input.Tensor) == 0 && r.Input.ImageURL == "" {
		return &ValidationError{Field: "input", Message: "必须提供至少一种输入"}
	}
	if r.Priority < TaskPriorityLow || r.Priority > TaskPriorityUrgent {
		r.Priority = TaskPriorityNormal
	}
	if r.Timeout <= 0 {
		r.Timeout = 30 * time.Second
	}
	return nil
}

// Validate 校验 ModelConfig
func (c *ModelConfig) Validate() error {
	if c.BatchSize < 1 || c.BatchSize > 64 {
		c.BatchSize = 1
	}
	if c.NumThreads < 1 || c.NumThreads > 32 {
		c.NumThreads = 4
	}
	if c.Precision != "" && c.Precision != "fp32" && c.Precision != "fp16" && c.Precision != "int8" {
		return &ValidationError{Field: "precision", Message: "必须是 fp32/fp16/int8"}
	}
	if c.OptimizeLevel < 0 || c.OptimizeLevel > 3 {
		c.OptimizeLevel = 1
	}
	return nil
}

// DefaultEngineConfig 默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		ModelDir:        "/var/lib/nas-os/edgeai/models",
		MaxConcurrent:   4,
		MaxQueueSize:    100,
		DefaultDevice:   ComputeDeviceCPU,
		EnableGPU:       true,
		EnableNPU:       false,
		GPUDeviceID:     0,
		MemoryLimit:     4 * 1024 * 1024 * 1024, // 4GB
		CacheSize:       1000,
		CacheTTL:        5 * time.Minute,
		Workers:         2,
		MonitorInterval: 10 * time.Second,
		EnableProfiling: false,
	}
}

// InferEngine 推理引擎接口
type InferEngine interface {
	// LoadModel 加载模型
	LoadModel(model *Model) error
	// UnloadModel 卸载模型
	UnloadModel(modelID string) error
	// Infer 执行推理
	Infer(request *InferenceRequest) (*InferenceResult, error)
	// InferAsync 异步推理
	InferAsync(request *InferenceRequest) (string, error)
	// GetResult 获取推理结果
	GetResult(requestID string) (*InferenceResult, error)
	// GetModel 获取模型信息
	GetModel(modelID string) (*Model, error)
	// ListModels 列出所有模型
	ListModels() ([]*Model, error)
	// GetStats 获取推理统计
	GetStats() (*InferStats, error)
	// GetResourceUsage 获取资源使用情况
	GetResourceUsage() (*ResourceUsage, error)
	// Close 关闭引擎
	Close() error
}

// ModelLoader 模型加载器接口
type ModelLoader interface {
	// Load 加载模型
	Load(modelPath string, config *ModelConfig) (interface{}, error)
	// Unload 卸载模型
	Unload(model interface{}) error
	// SupportsFormat 是否支持该格式
	SupportsFormat(format ModelFormat) bool
}

// InferPipeline 推理管道接口
type InferPipeline interface {
	// Process 处理推理请求
	Process(request *InferenceRequest, model interface{}) (*InferenceResult, error)
	// Preprocess 输入预处理
	Preprocess(input *InferenceInput, model *Model) (interface{}, error)
	// Postprocess 输出后处理
	Postprocess(output interface{}, model *Model) (*InferenceOutput, error)
}

// ResourceMonitor 资源监控器接口
type ResourceMonitor interface {
	// GetUsage 获取资源使用情况
	GetUsage() (*ResourceUsage, error)
	// Start 开始监控
	Start(interval time.Duration)
	// Stop 停止监控
	Stop()
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	mu          sync.RWMutex
	queue       []*InferenceRequest
	processing  map[string]*InferenceRequest
	maxQueue    int
	maxConcurrent int
	priorities  map[TaskPriority]int
	stats       *SchedulerStats
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	TotalQueued    int64     `json:"totalQueued"`
	TotalProcessed int64     `json:"totalProcessed"`
	AvgWaitTime    float64   `json:"avgWaitTime"` // ms
	MaxWaitTime    float64   `json:"maxWaitTime"` // ms
	QueueLength    int       `json:"queueLength"`
	Processing     int       `json:"processing"`
}
