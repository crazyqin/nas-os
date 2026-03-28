# 私有云 AI 服务架构设计

**版本**: v2.294.0 | **日期**: 2026-03-28 | **负责**: 工部

---

## 一、背景分析

### 竞品现状

#### 群晖 Synology DSM 7.3 私有云 AI 服务
- **核心特性**: 本地 LLM 支持，OpenAI 兼容 API
- **价值主张**: 数据隐私保护，无需上传云端
- **技术栈**: 
  - 内置 AI推理引擎
  - 支持主流开源模型（LLaMA、Mistral、Qwen等）
  - 提供与 OpenAI API 兼容的接口层
- **硬件要求**: Intel/AMD x86 平台，建议 16GB+ 内存

#### 飞牛fnOS 1.1 AI 人脸识别
- **核心特性**: 本地 AI 人脸检测/聚类/人物相册
- **技术栈**: Intel 核显加速（QuickSync）
- **部署方式**: 系统内置服务，无需额外配置

### nas-os 已有 AI 能力

| 功能 | 状态 | 技术栈 |
|------|------|--------|
| AI 相册 - 以文搜图 | ✅ 已实现 | CLIP 模型本地推理 |
| AI 人脸识别 | ✅ 已实现 | 本地人脸检测 + 聚类 |
| AI 数据脱敏 | ✅ 已实现 | PII 智能识别，多提供商支持 |
| AI 智能分类 | ✅ 已实现 | 照片/文件智能分类 |

### 差距分析

| 能力 | nas-os | 群晖 DSM 7.3 | 优先级 |
|------|:------:|:-----------:|:------:|
| 本地 LLM 推理 | ❌ 缺失 | ✅ | 🔴 P0 |
| OpenAI 兼容 API | ❌ 缺失 | ✅ | 🔴 P0 |
| AI Office 智能内容 | ❌ 缺失 | ✅ | 🟡 P1 |
| 本地人脸识别 | ✅ | ✅ | - |
| 语义搜索（以文搜图） | ✅ 独家 | ❌ | - |

---

## 二、架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    NAS-OS WebUI/API                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │ 相册管理 │  │ 文档管理 │  │ 智能搜索 │  │ AI对话助手      │ │
│  └─┬───────┘  └─┬───────┘  └─┬───────┘  └─┬───────────────┘ │
│    │           │           │           │                    │
└───┼───────────┼───────────┼───────────┼────────────────────┘
    │           │           │           │
    ▼           ▼           ▼           ▼
┌─────────────────────────────────────────────────────────────┐
│                   AI Service Gateway                         │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  OpenAI-Compatible API Layer                          │  │
│  │  • /v1/chat/completions                               │  │
│  │  • /v1/embeddings                                     │  │
│  │  • /v1/models                                         │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Model Router (智能调度)                              │  │
│  │  • 本地优先 → 云端备用                                │  │
│  │  • 模型大小适配硬件                                   │  │
│  │  • 负载均衡 + 故障转移                                │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                    │
        ┌───────────┼───────────┬───────────┐
        ▼           ▼           ▼           ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────┐
│   Ollama     │ │  llama.cpp   │ │  云端 API   │ │ 本地嵌入 │
│  (推荐方案)  │ │  (轻量方案)  │ │  (备用)     │ │  模型    │
│              │ │              │ │             │ │          │
│ • 易管理     │ │ • 低内存     │ │ • OpenAI   │ │ • CLIP   │
│ • 多模型     │ │ • 快启动     │ │ • Google   │ │ • BGE    │
│ • GPU/CPU   │ │ • 纯Go集成   │ │ • Azure    │ │          │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────┘
        │               │               │               │
        └───────────────┴───────────────┴───────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Hardware Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ CPU 推理    │  │ GPU 加速    │  │ NPU 加速 (ARM)      │ │
│  │ • x86/ARM  │  │ • NVIDIA   │  │ • Rockchip RK3588   │ │
│  │ • AVX2优化 │  │ • CUDA     │  │ • NPUs              │ │
│  │ • 多线程   │  │ • cuBLAS   │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块拆分

#### 核心模块

| 模块 | 功能 | 技术选型 |
|------|------|----------|
| **AI Gateway** | OpenAI 兼容 API + 路由 | Go + Gin |
| **Model Manager** | 模型下载/管理/版本控制 | Go + SQLite |
| **Local Inference** | 本地推理引擎集成 | Ollama/llama.cpp |
| **Embedding Service** | 文本/图像嵌入向量 | CLIP/BGE-M3 |
| **Cache Layer** | 推理结果缓存 | Redis/内存缓存 |

#### 可选模块

| 模块 | 功能 | 触发条件 |
|------|------|----------|
| GPU Accelerator | GPU 加速推理 | 检测 NVIDIA GPU |
| NPU Accelerator | ARM NPU 加速 | 检测 RK3588 等 |
| Cloud Fallback | 云端 API 备用 | 本地推理失败 |
| Model Optimizer | 模型量化压缩 | 内存不足场景 |

---

## 三、技术方案对比

### 3.1 本地 LLM 方案

#### 方案 A: Ollama（推荐）

**优势**:
- 成熟稳定，社区活跃
- 支持多模型并行管理
- REST API 设计，易于集成
- GPU/CPU 自动切换
- 支持模型热加载

**劣势**:
- 需要额外进程管理
- 内存占用较高（建议 8GB+）
- 部署需额外安装

**适用场景**:
- x86 平台（Intel/AMD）
- NVIDIA GPU 环境
- 内存 ≥ 16GB

**集成方式**:
```yaml
# docker-compose.yml
services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

#### 方案 B: llama.cpp（轻量）

**优势**:
- 单进程，易于管理
- 内存占用低（量化后 2-4GB）
- Go 可直接集成（binding）
- 启动速度快

**劣势**:
- 模型管理需自行实现
- 并发处理能力有限
- GPU 加速配置复杂

**适用场景**:
- ARM 平台（Rockchip RK3588）
- 内存受限环境（4-8GB）
- 简单推理场景

**集成方式**:
```go
// internal/ai/llama.go
package ai

import (
    "github.com/go-skynet/llama.go"
)

type LLamaEngine struct {
    model *llama.LLama
}

func NewLLamaEngine(modelPath string) (*LLamaEngine, error) {
    // 加载量化模型（GGUF格式）
    l, err := llama.New(modelPath, llama.SetContext(512))
    if err != nil {
        return nil, err
    }
    return &LLamaEngine{model: l}, nil
}
```

#### 方案 C: 云端 API 备用

**适用场景**:
- 本地硬件不足
- 复杂推理任务
- 用户选择云端

**提供商**:
- OpenAI (GPT-4o, GPT-3.5)
- Google (Gemini Pro)
- Azure (Azure OpenAI)
- 百度 (文心一言)
- 本地 LLM (自建服务)

### 3.2 方案推荐矩阵

| 硬件配置 | 推荐方案 | 模型大小 | 备注 |
|----------|----------|----------|------|
| NVIDIA GPU + 16GB+ | Ollama | 7B-13B | 最佳性能 |
| x86 CPU + 16GB+ | Ollama | 4B-7B | CPU推理 |
| x86 CPU + 8-16GB | llama.cpp | 2B-4B量化 | 轻量部署 |
| ARM RK3588 + 8GB | llama.cpp | 2B-4B量化 | NPU可选 |
| ARM + 4GB | 云端API | - | 本地仅嵌入 |

---

## 四、OpenAI 兼容 API 设计

### 4.1 API 端点

```yaml
# OpenAI Compatible API
endpoints:
  chat:
    - POST /v1/chat/completions
    - GET  /v1/models
  
  embeddings:
    - POST /v1/embeddings
  
  images:
    - POST /v1/images/generations  # 可选
```

### 4.2 Chat Completions 实现

```go
// internal/ai/gateway/chat.go
package gateway

type ChatCompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    Temperature float64   `json:"temperature,omitempty"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatCompletionResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

// Model Router 策略
func (g *Gateway) RouteRequest(req *ChatCompletionRequest) (Engine, error) {
    // 1. 检查本地模型是否可用
    if g.localEngine.HasModel(req.Model) && g.localEngine.Healthy() {
        return g.localEngine, nil
    }
    
    // 2. 云端备用
    if g.cloudEngine != nil && g.config.CloudFallback {
        return g.cloudEngine, nil
    }
    
    // 3. 返回错误
    return nil, fmt.Errorf("no available engine for model %s", req.Model)
}
```

### 4.3 模型管理

```go
// internal/ai/models/manager.go
package models

type ModelManager struct {
    db       *sql.DB
    cache    *cache.Cache
    downloader *Downloader
}

type Model struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Size        int64     `json:"size"`       // bytes
    Quantization string   `json:"quantization"` // Q4_K_M, Q5_K_M, Q8_0
    ContextSize int       `json:"context_size"`
    Modality    string    `json:"modality"`    // text, text+image
    Status      string    `json:"status"`      // downloading, ready, error
    Path        string    `json:"path"`
    CreatedAt   time.Time `json:"created_at"`
}

// 预置模型列表
var DefaultModels = []Model{
    {ID: "qwen2.5-3b-instruct", Name: "Qwen 2.5 3B", Size: 2_000_000_000, Quantization: "Q4_K_M"},
    {ID: "llama3.2-3b-instruct", Name: "Llama 3.2 3B", Size: 2_000_000_000, Quantization: "Q4_K_M"},
    {ID: "mistral-7b-instruct", Name: "Mistral 7B", Size: 4_000_000_000, Quantization: "Q4_K_M"},
    {ID: "gemma2-9b-instruct", Name: "Gemma 2 9B", Size: 6_000_000_000, Quantization: "Q4_K_M"},
}
```

---

## 五、部署架构

### 5.1 独立镜像部署（已实现）

nas-os 已提供 AI 服务独立镜像（v2.255.0）:
- `ghcr.io/crazyqin/nas-os-ai:latest-gpu` - GPU 加速版
- `ghcr.io/crazyqin/nas-os-ai:latest-cpu` - CPU 推理版

### 5.2 模块化部署

```yaml
# docker-compose.ai.yml
services:
  nas-os:
    image: ghcr.io/crazyqin/nas-os:latest
    ports:
      - "8080:8080"
    environment:
      - AI_SERVICE_ENABLED=true
      - AI_SERVICE_URL=http://ai-service:8081
    volumes:
      - nas_data:/data
      - nas_config:/config

  ai-service:
    image: ghcr.io/crazyqin/nas-os-ai:latest-cpu
    ports:
      - "8081:8081"
    environment:
      - OLLAMA_HOST=http://ollama:11434
      - EMBEDDING_MODEL=clip-vit-b-32
    volumes:
      - ai_models:/models
      - ai_cache:/cache

  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    profiles:
      - gpu
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]

volumes:
  nas_data:
  nas_config:
  ai_models:
  ai_cache:
  ollama_data:
```

### 5.3 单进程部署（ARM 轻量）

```yaml
# docker-compose.arm.yml - ARM 轻量部署
services:
  nas-os:
    image: ghcr.io/crazyqin/nas-os:latest
    ports:
      - "8080:8080"
    environment:
      - AI_SERVICE_ENABLED=true
      - AI_ENGINE=llama.cpp
      - AI_MODEL_PATH=/models/llama-3.2-3b-q4.gguf
    volumes:
      - nas_data:/data
      - ai_models:/models
```

---

## 六、硬件加速方案

### 6.1 NVIDIA GPU

```go
// internal/ai/gpu/nvidia.go
package gpu

func DetectNVIDIA() (*GPUInfo, error) {
    // 检测 NVIDIA GPU
    cmd := exec.Command("nvidia-smi", "--query-gpu", "name,memory.total", "--format=csv")
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    // 解析 GPU 信息
    // ...
}

// CUDA 加速配置
func ConfigureCUDA(engine *LLamaEngine) error {
    engine.SetGPULayers(35) // GPU 层数
    engine.SetMainGPU(0)    // 主 GPU
    return nil
}
```

### 6.2 ARM NPU (Rockchip RK3588)

```go
// internal/ai/npu/rockchip.go
package npu

// RK3588 NPU 加速
// 使用 rknn-toolkit2 或 llama.cpp NPU 支持

func ConfigureNPU(engine *LLamaEngine) error {
    // 1. 检测 NPU
    if !hasRK3588NPU() {
        return errors.New("NPU not detected")
    }
    
    // 2. 配置 NPU 加速
    // llama.cpp RK3588 NPU 支持正在开发中
    // 当前方案: CPU + AVX2 优化
    
    return nil
}
```

---

## 七、性能基准

### 7.1 预期性能指标

| 硬件配置 | 模型 | 推理速度 | 内存占用 |
|----------|------|----------|----------|
| NVIDIA RTX 3060 | Llama 3.1 8B Q4 | 40-60 tok/s | 6GB VRAM |
| Intel i5-12400 | Llama 3.2 3B Q4 | 8-15 tok/s | 4GB RAM |
| RK3588 (CPU) | Llama 3.2 3B Q4 | 3-5 tok/s | 3GB RAM |
| RK3588 (NPU) | Llama 3.2 3B Q4 | 10-20 tok/s* | 2GB RAM |

*NPU 加速待 llama.cpp 官方支持

### 7.2 模型推荐

| 用户场景 | 推荐模型 | 理由 |
|----------|----------|------|
| 日常对话 | Qwen 2.5 3B | 中文支持好，轻量 |
| 文档总结 | Mistral 7B | 长文本能力强 |
| 代码辅助 | Llama 3.1 8B | 代码能力强 |
| 翻译 | Qwen 2.5 7B | 多语言支持 |

---

## 八、安全与隐私

### 8.1 数据处理原则

| 场景 | 处理方式 | 备注 |
|------|----------|------|
| 本地推理 | 数据不离开设备 | 默认行为 |
| 云端备用 | 明确告知用户 | 用户选择 |
| 模型下载 | 仅下载开源模型 | HuggingFace/ModelScope |
| 缓存数据 | 本地加密存储 | AES-256 |

### 8.2 用户配置

```yaml
# /config/ai.yaml
ai:
  # 推理引擎
  engine: "ollama"  # ollama, llama.cpp, cloud
  
  # 本地优先
  local_first: true
  
  # 云端备用
  cloud_fallback: false
  
  # 模型配置
  models:
    chat: "qwen2.5-3b-instruct"
    embedding: "clip-vit-b-32"
  
  # GPU 配置
  gpu:
    enabled: true
    layers: 35
    memory_limit: 6GB
  
  # 隐私配置
  privacy:
    allow_cloud: false  # 禁止云端推理
    encrypt_cache: true
    log_requests: false
```

---

## 九、开发计划

### 9.1 里程碑

| 版本 | 功能 | 预计发布 |
|------|------|----------|
| v2.295.0 | OpenAI 兼容 API Gateway | 2026-03-30 |
| v2.300.0 | Ollama 集成 + 模型管理 | 2026-04-01 |
| v2.305.0 | llama.cpp 集成（ARM） | 2026-04-05 |
| v2.310.0 | GPU/NPU 加速优化 | 2026-04-10 |
| v2.315.0 | AI Office 功能 | 2026-04-15 |

### 9.2 任务分解

**Phase 1: API Gateway（P0）**
- [ ] 实现 OpenAI 兼容 API 端点
- [ ] Model Router 调度逻辑
- [ ] Stream 流式响应支持
- [ ] API 认证 + 限流

**Phase 2: 本地推理（P0）**
- [ ] Ollama 集成（主方案）
- [ ] llama.cpp 集成（轻量方案）
- [ ] 模型下载/管理
- [ ] 嵌入服务集成

**Phase 3: 硬件加速（P1）**
- [ ] NVIDIA CUDA 加速
- [ ] ARM NPU 加速（待上游支持）
- [ ] 性能监控 + 优化

**Phase 4: AI 功能（P1）**
- [ ] 文档智能总结
- [ ] 智能搜索增强
- [ ] AI 对话助手
- [ ] AI Office 集成

---

## 十、附录

### A. 模型资源

| 来源 | 说明 |
|------|------|
| [HuggingFace](https://huggingface.co/models) | 主流开源模型库 |
| [ModelScope](https://modelscope.cn/models) | 阿里云模型库（国内加速） |
| [Ollama Library](https://ollama.com/library) | Ollama 官方模型 |

### B. 技术文档

- [Ollama GitHub](https://github.com/ollama/ollama)
- [llama.cpp GitHub](https://github.com/ggerganov/llama.cpp)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)

### C. 相关 issue

- [#XXX] 私有云 AI 服务设计讨论
- [#XXX] Ollama 集成实现
- [#XXX] ARM 平台推理优化

---

**文档维护**: 工部 | **更新日期**: 2026-03-28