// Package edgeaiinference 提供边缘AI推理网关功能
// v2.53.0 - 边缘设备AI推理管理
package edgeaiinference

import (
	"time"
)

// ModelStatus 定义模型状态
type ModelStatus string

const (
	// ModelStatusLoading 模型加载中
	ModelStatusLoading ModelStatus = "loading"
	// ModelStatusReady 模型就绪
	ModelStatusReady ModelStatus = "ready"
	// ModelStatusRunning 模型运行中
	ModelStatusRunning ModelStatus = "running"
	// ModelStatusError 模型错误
	ModelStatusError ModelStatus = "error"
	// ModelStatusUnloaded 模型已卸载
	ModelStatusUnloaded ModelStatus = "unloaded"
)

// ModelType 定义模型类型
type ModelType string

const (
	// ModelTypeLLM 大语言模型
	ModelTypeLLM ModelType = "llm"
	// ModelTypeVision 视觉模型
	ModelTypeVision ModelType = "vision"
	// ModelTypeAudio 音频模型
	ModelTypeAudio ModelType = "audio"
	// ModelTypeEmbedding 嵌入模型
	ModelTypeEmbedding ModelType = "embedding"
	// ModelTypeClassification 分类模型
	ModelTypeClassification ModelType = "classification"
	// ModelTypeDetection 检测模型
	ModelTypeDetection ModelType = "detection"
)

// InferenceStatus 推理状态
type InferenceStatus string

const (
	// InferenceStatusQueued 排队中
	InferenceStatusQueued InferenceStatus = "queued"
	// InferenceStatusProcessing 处理中
	InferenceStatusProcessing InferenceStatus = "processing"
	// InferenceStatusCompleted 完成
	InferenceStatusCompleted InferenceStatus = "completed"
	// InferenceStatusFailed 失败
	InferenceStatusFailed InferenceStatus = "failed"
	// InferenceStatusCancelled 已取消
	InferenceStatusCancelled InferenceStatus = "cancelled"
)

// DeviceType 定义设备类型
type DeviceType string

const (
	// DeviceTypeCPU CPU设备
	DeviceTypeCPU DeviceType = "cpu"
	// DeviceTypeGPU GPU设备
	DeviceTypeGPU DeviceType = "gpu"
	// DeviceTypeNPU NPU设备
	DeviceTypeNPU DeviceType = "npu"
	// DeviceTypeTPU TPU设备
	DeviceTypeTPU DeviceType = "tpu"
	// DeviceTypeEdgeTPU 边缘TPU
	DeviceTypeEdgeTPU DeviceType = "edge_tpu"
)

// ComputeDevice 计算设备
type ComputeDevice struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         DeviceType `json:"type"`
	MemoryMB     int        `json:"memoryMB"`
	UsedMemoryMB int        `json:"usedMemoryMB"`
	Utilization  float64    `json:"utilization"`
	Temperature  float64    `json:"temperature"`
	PowerWatts   float64    `json:"powerWatts"`
	Available    bool       `json:"available"`
	Models       []string   `json:"models,omitempty"`
}

// AIModel AI模型定义
type AIModel struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        ModelType              `json:"type"`
	Version     string                 `json:"version"`
	Status      ModelStatus            `json:"status"`
	DeviceID    string                 `json:"deviceId"`
	DeviceType  DeviceType             `json:"deviceType"`
	MemoryMB    int                    `json:"memoryMB"`
	InputShape  []int                  `json:"inputShape,omitempty"`
	OutputShape []int                  `json:"outputShape,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	LoadedAt    *time.Time             `json:"loadedAt,omitempty"`
	LastUsedAt  *time.Time             `json:"lastUsedAt,omitempty"`
	UseCount    int64                  `json:"useCount"`
	FilePath    string                 `json:"filePath"`
	Checksum    string                 `json:"checksum,omitempty"`
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ID          string                 `json:"id"`
	ModelID     string                 `json:"modelName"`
	Input       map[string]interface{} `json:"input"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Priority    int                    `json:"priority"`
	Timeout     time.Duration          `json:"timeout"`
	MaxRetries  int                    `json:"maxRetries"`
	SubmittedAt time.Time              `json:"submittedAt"`
}

// InferenceResult 推理结果
type InferenceResult struct {
	ID          string                 `json:"id"`
	RequestID   string                 `json:"requestId"`
	ModelID     string                 `json:"modelName"`
	Status      InferenceStatus        `json:"status"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	LatencyMs   int64                  `json:"latencyMs"`
	TokensUsed  int                    `json:"tokensUsed,omitempty"`
	DeviceID    string                 `json:"deviceId"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
}

// InferenceQueue 推理队列
type InferenceQueue struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MaxSize     int       `json:"maxSize"`
	CurrentSize int       `json:"currentSize"`
	Strategy    string    `json:"strategy"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ResourceQuota 资源配额
type ResourceQuota struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	MaxModels      int    `json:"maxModels"`
	MaxMemoryMB    int    `json:"maxMemoryMB"`
	MaxConcurrent  int    `json:"maxConcurrent"`
	MaxQueueSize   int    `json:"maxQueueSize"`
	RequestsPerMin int    `json:"requestsPerMin"`
	TokensPerDay   int    `json:"tokensPerDay"`
}

// InferenceMetrics 推理指标
type InferenceMetrics struct {
	TotalRequests    int64     `json:"totalRequests"`
	SuccessRequests  int64     `json:"successRequests"`
	FailedRequests   int64     `json:"failedRequests"`
	AvgLatencyMs     float64   `json:"avgLatencyMs"`
	P95LatencyMs     float64   `json:"p95LatencyMs"`
	P99LatencyMs     float64   `json:"p99LatencyMs"`
	QueueDepth       int       `json:"queueDepth"`
	GPUMemoryUsedMB  int       `json:"gpuMemoryUsedMB"`
	GPUMemoryTotalMB int       `json:"gpuMemoryTotalMB"`
	GPUUtilization   float64   `json:"gpuUtilization"`
	Timestamp        time.Time `json:"timestamp"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Strategy         string `json:"strategy"`
	MaxBatchSize     int    `json:"maxBatchSize"`
	BatchTimeoutMs   int    `json:"batchTimeoutMs"`
	MaxQueueSize     int    `json:"maxQueueSize"`
	EnablePreemption bool   `json:"enablePreemption"`
}

// InferenceEvent 推理事件
type InferenceEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	ModelID   string            `json:"modelName"`
	Message   string            `json:"message"`
	Severity  string            `json:"severity"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
