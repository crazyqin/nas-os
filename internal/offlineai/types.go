// Package offlineai 提供离线AI助手功能，支持本地LLM推理、对话管理、模型切换和任务调度。
// 兼容 llama.cpp 格式模型，支持 GPU 加速检测和量化模型加载。
package offlineai

import "time"

// EngineType 推理引擎类型
type EngineType string

const (
	EngineLlamaCpp EngineType = "llamacpp" // llama.cpp 引擎
	EngineGGML     EngineType = "ggml"     // GGML 引擎
	EngineONNX     EngineType = "onnx"     // ONNX Runtime
)

// ModelFormat 模型格式
type ModelFormat string

const (
	ModelFormatGGUF ModelFormat = "gguf" // GGUF 格式
	ModelFormatGGML ModelFormat = "ggml" // GGML 格式
	ModelFormatONNX ModelFormat = "onnx" // ONNX 格式
)

// QuantType 量化类型
type QuantType string

const (
	QuantNone QuantType = "none" // 无量化
	QuantQ4_0 QuantType = "q4_0" // 4-bit 量化
	QuantQ4_1 QuantType = "q4_1"
	QuantQ5_0 QuantType = "q5_0" // 5-bit 量化
	QuantQ5_1 QuantType = "q5_1"
	QuantQ8_0 QuantType = "q8_0" // 8-bit 量化
	QuantF16  QuantType = "f16"  // 半精度浮点
	QuantF32  QuantType = "f32"  // 全精度浮点
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityUrgent TaskPriority = 3
)

// ModelStatus 模型状态
type ModelStatus string

const (
	ModelStatusUnloaded ModelStatus = "unloaded"
	ModelStatusLoading  ModelStatus = "loading"
	ModelStatusReady    ModelStatus = "ready"
	ModelStatusError    ModelStatus = "error"
)

// Role 消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Config 离线AI引擎配置
type Config struct {
	Enabled          bool       `json:"enabled"`
	EngineType       EngineType `json:"engine_type"`
	ModelDir         string     `json:"model_dir"`         // 模型存放目录
	DefaultModel     string     `json:"default_model"`     // 默认模型名称
	ContextSize      int        `json:"context_size"`      // 上下文窗口大小
	MaxTokens        int        `json:"max_tokens"`        // 单次生成最大 token 数
	Temperature      float64    `json:"temperature"`       // 采样温度
	TopP             float64    `json:"top_p"`             // Top-P 采样
	TopK             int        `json:"top_k"`             // Top-K 采样
	RepeatPenalty    float64    `json:"repeat_penalty"`    // 重复惩罚
	GPUEnabled       bool       `json:"gpu_enabled"`       // 是否启用 GPU
	GPULayers        int        `json:"gpu_layers"`        // GPU 卸载层数
	Threads          int        `json:"threads"`           // CPU 线程数
	MaxConcurrent    int        `json:"max_concurrent"`    // 最大并发推理数
	MaxHistory       int        `json:"max_history"`       // 最大对话历史条数
	SchedulerWorkers int        `json:"scheduler_workers"` // 调度器工作线程数
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		EngineType:       EngineLlamaCpp,
		ModelDir:         "/var/lib/nas-os/models",
		DefaultModel:     "default",
		ContextSize:      4096,
		MaxTokens:        2048,
		Temperature:      0.7,
		TopP:             0.9,
		TopK:             40,
		RepeatPenalty:    1.1,
		GPUEnabled:       true,
		GPULayers:        32,
		Threads:          4,
		MaxConcurrent:    2,
		MaxHistory:       100,
		SchedulerWorkers: 4,
	}
}

// Model 模型信息
type Model struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Format      ModelFormat `json:"format"`
	QuantType   QuantType   `json:"quant_type"`
	Size        int64       `json:"size"`       // 模型文件大小（字节）
	Parameters  int64       `json:"parameters"` // 参数量
	VRAMUsage   int64       `json:"vram_usage"` // 显存占用（字节）
	Status      ModelStatus `json:"status"`
	GPUSupport  bool        `json:"gpu_support"` // 是否支持 GPU
	MaxContext  int         `json:"max_context"` // 模型最大上下文长度
	LoadedAt    time.Time   `json:"loaded_at"`
	Description string      `json:"description,omitempty"`
}

// GPUInfo GPU 信息
type GPUInfo struct {
	Available   bool   `json:"available"`
	Name        string `json:"name"`
	VRAMTotal   int64  `json:"vram_total"` // 总显存（字节）
	VRAMUsed    int64  `json:"vram_used"`  // 已用显存
	VRAMFree    int64  `json:"vram_free"`  // 可用显存
	Driver      string `json:"driver"`
	CUDAVersion string `json:"cuda_version,omitempty"`
}

// Message 对话消息
type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens"` // 消耗的 token 数
	Timestamp time.Time `json:"timestamp"`
}

// Conversation 对话会话
type Conversation struct {
	ID          string    `json:"id"`
	Messages    []Message `json:"messages"`
	ModelName   string    `json:"model_name"`
	TotalTokens int       `json:"total_tokens"` // 总消耗 token
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InferRequest 推理请求
type InferRequest struct {
	Prompt      string   `json:"prompt" binding:"required"`
	ModelName   string   `json:"model_name,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	TopK        int      `json:"top_k,omitempty"`
	Stream      bool     `json:"stream,omitempty"`     // 是否流式输出
	StopWords   []string `json:"stop_words,omitempty"` // 停止词
}

// InferResponse 推理响应
type InferResponse struct {
	Text       string        `json:"text"`
	TokensUsed int           `json:"tokens_used"`
	Duration   time.Duration `json:"duration"`
	ModelName  string        `json:"model_name"`
	Finished   bool          `json:"finished"`
}

// Task 任务
type Task struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"` // infer, batch, scheduled
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	Payload     interface{}  `json:"payload"` // 任务负载
	Result      interface{}  `json:"result"`  // 任务结果
	Error       string       `json:"error,omitempty"`
	Attempts    int          `json:"attempts"`
	MaxAttempts int          `json:"max_attempts"`
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
	ScheduledAt *time.Time   `json:"scheduled_at,omitempty"` // 定时执行时间
}

// ChatRequest 对话请求
type ChatRequest struct {
	ConversationID string `json:"conversation_id,omitempty"` // 继续已有对话
	Message        string `json:"message" binding:"required"`
	ModelName      string `json:"model_name,omitempty"`
	MaxTokens      int    `json:"max_tokens,omitempty"`
	Stream         bool   `json:"stream,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	ConversationID string        `json:"conversation_id"`
	Reply          string        `json:"reply"`
	TokensUsed     int           `json:"tokens_used"`
	Duration       time.Duration `json:"duration"`
}

// StreamChunk 流式输出块
type StreamChunk struct {
	Text    string `json:"text"`
	Done    bool   `json:"done"`
	TokenID int    `json:"token_id,omitempty"`
}
