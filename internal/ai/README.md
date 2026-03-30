# 私有云AI服务模块

## 概述

本模块为NAS-OS提供完整的私有云AI推理能力，支持多种本地LLM后端，提供OpenAI兼容API，以及模型管理功能。

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                    AI Gateway (统一入口)                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ Rate Limiter│  │Model Router │  │   Monitor   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│ Ollama Backend│    │LocalAI Backend│    │ vLLM Backend  │
│  (Ollama API) │    │(OpenAI Compat)│    │(OpenAI Compat)│
└───────────────┘    └───────────────┘    └───────────────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              ▼
                    ┌───────────────────┐
                    │   Model Manager   │
                    │ - 下载/删除模型    │
                    │ - 存储管理        │
                    │ - 模型切换        │
                    └───────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Resource Monitor  │
                    │ - GPU监控         │
                    │ - 内存监控        │
                    └───────────────────┘
```

## 核心组件

### 1. Gateway（API网关）

提供统一的AI推理入口，支持：
- OpenAI兼容API (`/v1/chat/completions`, `/v1/embeddings`, `/v1/models`)
- 请求路由和负载均衡
- 速率限制和并发控制
- 健康检查和故障转移
- 监控指标收集

```go
// 初始化Gateway
gateway := NewGateway(&config.GatewayConfig{
    Listen:             ":11435",
    EnableOpenAICompat: true,
    DefaultBackend:     "ollama",
})

// 注册后端
gateway.RegisterBackend(BackendOllama, ollamaBackend)

// 发送请求
resp, err := gateway.Chat(ctx, &ChatRequest{
    Model:    "llama2",
    Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
})
```

### 2. Backend（后端适配器）

支持多种LLM后端：

#### Ollama
```yaml
backends:
  ollama:
    enabled: true
    endpoint: "http://localhost:11434"
    defaultModel: "llama2"
    gpuLayers: 0
    contextSize: 4096
    modelPath: "/var/lib/nas-os/ai/models/ollama"
```

#### LocalAI
```yaml
backends:
  localai:
    enabled: true
    endpoint: "http://localhost:8080"
    defaultModel: "llama2"
    threads: 4
    contextSize: 4096
```

#### vLLM
```yaml
backends:
  vllm:
    enabled: true
    endpoint: "http://localhost:8000"
    defaultModel: "facebook/opt-125m"
    tensorParallelSize: 1
    gpuMemoryUtil: 0.9
```

### 3. Model Manager（模型管理）

提供模型生命周期管理：

```go
// 搜索模型
results, err := modelMgr.SearchModels(ctx, "llama", "ollama")

// 下载模型
progress, err := modelMgr.DownloadModel(ctx, &ModelDownloadRequest{
    ModelName: "llama2:7b",
    Source:    "ollama",
})

// 列出已安装模型
models := modelMgr.ListModels()

// 删除模型
err := modelMgr.DeleteModel(ctx, "llama2")
```

### 4. Resource Monitor（资源监控）

监控系统资源使用：

```go
// GPU信息
gpus, _ := resourceMon.GetGPUInfo()

// 内存信息
mem, _ := resourceMon.GetMemoryInfo()
```

## API 端点

### OpenAI 兼容 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/ai/v1/chat/completions` | POST | 聊天补全 |
| `/api/ai/v1/completions` | POST | 文本补全 |
| `/api/ai/v1/embeddings` | POST | 生成嵌入向量 |
| `/api/ai/v1/models` | GET | 列出可用模型 |
| `/api/ai/v1/models/:model` | GET | 获取模型信息 |

### 网关管理 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/ai/gateway/status` | GET | 网关状态 |
| `/api/ai/gateway/metrics` | GET | 性能指标 |
| `/api/ai/gateway/backends` | GET | 后端状态 |
| `/api/ai/gateway/route` | POST | 设置模型路由 |

### 模型管理 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/ai/models` | GET | 列出本地模型 |
| `/api/ai/models/:name` | GET | 获取模型详情 |
| `/api/ai/models/download` | POST | 下载模型 |
| `/api/ai/models/:name` | DELETE | 删除模型 |
| `/api/ai/models/search` | POST | 搜索模型 |
| `/api/ai/models/storage` | GET | 存储使用量 |

### 资源监控 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/ai/resources/gpu` | GET | GPU信息 |
| `/api/ai/resources/memory` | GET | 内存信息 |
| `/api/ai/resources` | GET | 所有资源信息 |

## 使用示例

### 1. 聊天补全

```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### 2. 流式输出

```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

### 3. 生成嵌入向量

```bash
curl -X POST http://localhost:8080/api/ai/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "input": "Hello, world!"
  }'
```

### 4. 下载模型

```bash
curl -X POST http://localhost:8080/api/ai/models/download \
  -H "Content-Type: application/json" \
  -d '{
    "modelName": "llama2:7b",
    "source": "ollama"
  }'
```

## 配置说明

完整配置示例：

```yaml
ai:
  gateway:
    listen: ":11435"
    enableOpenAICompat: true
    defaultBackend: "ollama"
    requestTimeout: 300
    rateLimit:
      enabled: true
      requestsPerMin: 60
      tokensPerMin: 100000
      burstSize: 10
      concurrencyLimit: 5

  backends:
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
      defaultModel: "llama2"
      gpuLayers: 0
      contextSize: 4096
      modelPath: "/var/lib/nas-os/ai/models/ollama"

    localai:
      enabled: false
      endpoint: "http://localhost:8080"

    vllm:
      enabled: false
      endpoint: "http://localhost:8000"

  modelManager:
    storagePath: "/var/lib/nas-os/ai/models"
    autoDownload: false
    cache:
      enabled: true
      maxSizeGB: 50

  resources:
    maxGpuMemory: 80
    maxSystemMemory: 50
    maxConcurrent: 10

  security:
    enableDeId: true
    auditLogging: true
```

## 与群晖 DSM 7.3 对比

| 功能 | 群晖 DSM 7.3 | NAS-OS |
|------|--------------|--------|
| 本地LLM支持 | ✅ | ✅ |
| 多后端支持 | 有限 | Ollama/LocalAI/vLLM/自定义 |
| OpenAI兼容API | ✅ | ✅ |
| 模型下载管理 | ✅ | ✅ |
| GPU加速 | ✅ | ✅ (NVIDIA) |
| 资源监控 | ✅ | ✅ |
| 速率限制 | ✅ | ✅ |
| 隐私保护(PII) | ✅ | ✅ |
| 模型路由 | ❌ | ✅ |
| 故障转移 | ❌ | ✅ |

## 测试

运行测试：

```bash
cd internal/ai
go test -v ./...
```

运行基准测试：

```bash
go test -bench=. -benchmem
```

## 依赖

- Go 1.21+
- Ollama (可选)
- LocalAI (可选)
- vLLM (可选)
- NVIDIA GPU (可选，用于加速)

## 扩展

### 添加自定义后端

实现 `Backend` 接口：

```go
type CustomBackend struct {
    *BaseBackend
}

func (b *CustomBackend) Name() BackendType {
    return BackendCustom
}

func (b *CustomBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 实现聊天逻辑
}

func (b *CustomBackend) IsHealthy(ctx context.Context) bool {
    // 实现健康检查
}

// ... 其他方法
```

### 添加模型源

在 `ModelManager` 中添加新的下载方法：

```go
func (m *ModelManager) downloadFromCustom(ctx context.Context, req *ModelDownloadRequest, progress *DownloadProgress) {
    // 实现自定义下载逻辑
}
```