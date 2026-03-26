# NAS-OS AI 服务使用指南

**版本**: v2.274.0 | **更新日期**: 2026-03-26

---

## 📋 目录

- [概述](#概述)
- [快速开始](#快速开始)
- [核心功能](#核心功能)
- [多提供商配置](#多提供商配置)
- [API 参考](#api-参考)
- [最佳实践](#最佳实践)
- [常见问题](#常见问题)

---

## 概述

NAS-OS AI 服务为私有云 NAS 提供完整的 AI 能力，支持本地 LLM 推理和云端 AI 服务集成，让您的 NAS 更智能。

### 核心特性

| 特性 | 说明 |
|------|------|
| 🤖 本地 LLM | 支持 Ollama、LocalAI、vLLM 等本地推理引擎 |
| 🔌 OpenAI 兼容 API | 标准 API 接口，兼容现有应用 |
| 🔄 多提供商支持 | OpenAI、Google Gemini、Azure、百度文心等 |
| 🛡️ 数据脱敏 | 自动识别并保护敏感信息 (PII) |
| 🖼️ 智能相册 | 以文搜图、智能分类、人脸识别 |
| 📊 资源监控 | GPU/CPU 使用监控，智能资源调度 |

### 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                    用户应用 / Web UI                          │
└─────────────────────────────┬───────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    AI Gateway (统一入口)                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ 速率限制    │  │ 模型路由    │  │ 资源监控    │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────┬───────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│ Ollama        │    │ OpenAI        │    │ Google Gemini │
│ (本地推理)    │    │ (云端服务)    │    │ (云端服务)    │
└───────────────┘    └───────────────┘    └───────────────┘
```

---

## 快速开始

### 1. 启用 AI 服务

**Web UI 方式**：
1. 登录 NAS-OS 管理界面
2. 进入「设置」→「AI 服务」
3. 开启「启用 AI 服务」开关
4. 选择默认提供商（推荐 Ollama 本地推理）

**命令行方式**：
```bash
# 启用 AI 服务
sudo nasctl ai enable

# 查看服务状态
sudo nasctl ai status
```

### 2. 安装本地 LLM 引擎

推荐使用 Ollama 作为本地推理引擎：

```bash
# 安装 Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 下载模型
ollama pull llama2:7b        # 对话模型
ollama pull nomic-embed-text # 向量模型

# 验证安装
ollama list
```

### 3. 第一个对话

**Web UI**：
进入「AI 助手」页面，开始对话

**API 调用**：
```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "user", "content": "你好，介绍一下你自己"}
    ]
  }'
```

---

## 核心功能

### 1. 智能对话 (Chat)

与 AI 进行自然语言对话，支持多轮上下文。

```bash
# 多轮对话示例
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "system", "content": "你是一个友好的助手"},
      {"role": "user", "content": "我想了解 NAS 存储"},
      {"role": "assistant", "content": "NAS 是网络附加存储..."},
      {"role": "user", "content": "它有哪些优点？"}
    ]
  }'
```

**支持的功能**：
- 多轮上下文对话
- 流式输出 (Server-Sent Events)
- 自定义系统提示词
- 温度、最大长度等参数调节

### 2. 文本摘要 (Summarize)

智能提取文本要点，生成长文摘要。

```bash
curl -X POST http://localhost:8080/api/ai/summarize \
  -H "Content-Type: application/json" \
  -d '{
    "content": "长文本内容...",
    "maxLength": 200
  }'
```

**应用场景**：
- 文档摘要
- 会议纪要生成
- 新闻要点提取

### 3. 多语言翻译 (Translate)

支持多种语言之间的翻译。

```bash
curl -X POST http://localhost:8080/api/ai/translate \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello, world!",
    "targetLang": "zh-CN"
  }'
```

**支持的语言**：
- 中文（简体/繁体）
- 英语、日语、韩语
- 法语、德语、西班牙语等

### 4. 情感分析 (Sentiment)

分析文本的情感倾向。

```bash
curl -X POST http://localhost:8080/api/ai/sentiment \
  -H "Content-Type: application/json" \
  -d '{
    "text": "这个产品太棒了，非常满意！"
  }'
```

**返回结果**：
```json
{
  "sentiment": "positive",
  "confidence": 0.95,
  "scores": {
    "positive": 0.95,
    "negative": 0.03,
    "neutral": 0.02
  }
}
```

### 5. 向量嵌入 (Embeddings)

将文本转换为向量表示，用于语义搜索、相似度计算。

```bash
curl -X POST http://localhost:8080/api/ai/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "input": "这是一段需要向量化的文本"
  }'
```

**应用场景**：
- 语义搜索
- 文档相似度比较
- 推荐系统

### 6. AI 数据脱敏

自动识别并替换敏感信息，保护隐私。

```bash
curl -X POST http://localhost:8080/api/ai/desensitize \
  -H "Content-Type: application/json" \
  -d '{
    "text": "张三的手机号是 13812345678，邮箱 test@example.com"
  }'
```

**返回结果**：
```json
{
  "text": "张三的手机号是 [PHONE]，邮箱 [EMAIL]",
  "redactions": [
    {"type": "phone", "original": "13812345678", "replacement": "[PHONE]"},
    {"type": "email", "original": "test@example.com", "replacement": "[EMAIL]"}
  ]
}
```

**支持的脱敏类型**：
| 类型 | 说明 | 示例 |
|------|------|------|
| 手机号 | 11位手机号码 | 13812345678 → [PHONE] |
| 邮箱 | 电子邮件地址 | user@example.com → [EMAIL] |
| 身份证 | 18位身份证号 | 110101199001011234 → [ID] |
| 银行卡 | 16位银行卡号 | 6222021234567890 → [CARD] |
| IP地址 | IPv4地址 | 192.168.1.1 → [IP] |

### 7. 智能相册（以文搜图）

通过自然语言描述搜索照片。

```bash
curl -X POST http://localhost:8080/api/v1/photos/search \
  -H "Content-Type: application/json" \
  -d '{
    "text": "海边日落的照片",
    "limit": 20
  }'
```

**功能特点**：
- 支持中文语义搜索
- 支持时间、地点、场景过滤
- 支持人物识别搜索

---

## 多提供商配置

NAS-OS AI 服务支持多种 AI 提供商，可灵活配置和切换。

### 支持的提供商

| 提供商 | 类型 | 适用场景 | 特点 |
|--------|------|----------|------|
| **Ollama** | 本地 | 隐私优先、离线场景 | 免费、开源、隐私保护 |
| **LocalAI** | 本地 | 兼容 OpenAI API | 高性能、支持多种模型 |
| **vLLM** | 本地 | 高并发场景 | 高吞吐量、GPU优化 |
| **OpenAI** | 云端 | 高质量对话 | 模型能力强 |
| **Google Gemini** | 云端 | 多模态任务 | 支持图片理解 |
| **Azure OpenAI** | 云端 | 企业合规场景 | 企业级安全 |
| **百度文心** | 云端 | 中文优化 | 中文理解能力强 |

### 配置文件

编辑 `/etc/nas-os/config.yaml`：

```yaml
ai:
  # 网关配置
  gateway:
    listen: ":11435"
    enableOpenAICompat: true
    defaultBackend: "ollama"
    requestTimeout: 300

  # 速率限制
    rateLimit:
      enabled: true
      requestsPerMin: 60
      tokensPerMin: 100000
      burstSize: 10
      concurrencyLimit: 5

  # 后端配置
  backends:
    # Ollama 本地推理
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
      defaultModel: "llama2"
      gpuLayers: 0
      contextSize: 4096
      modelPath: "/var/lib/nas-os/ai/models/ollama"

    # LocalAI
    localai:
      enabled: false
      endpoint: "http://localhost:8080"
      defaultModel: "llama2"
      threads: 4

    # vLLM 高性能推理
    vllm:
      enabled: false
      endpoint: "http://localhost:8000"
      defaultModel: "facebook/opt-125m"
      tensorParallelSize: 1
      gpuMemoryUtil: 0.9

    # OpenAI 云端
    openai:
      enabled: false
      apiKey: "sk-..."
      defaultModel: "gpt-4"
      organization: ""

    # Google Gemini
    gemini:
      enabled: false
      apiKey: "..."
      defaultModel: "gemini-pro"

    # Azure OpenAI
    azure:
      enabled: false
      endpoint: "https://your-resource.openai.azure.com"
      apiKey: "..."
      defaultModel: "gpt-4"
      apiVersion: "2024-02-15-preview"

  # 模型管理
  modelManager:
    storagePath: "/var/lib/nas-os/ai/models"
    autoDownload: false
    cache:
      enabled: true
      maxSizeGB: 50

  # 资源限制
  resources:
    maxGpuMemory: 80    # GPU内存使用上限 (%)
    maxSystemMemory: 50  # 系统内存使用上限 (%)
    maxConcurrent: 10    # 最大并发请求数

  # 安全配置
  security:
    enableDeId: true      # 启用数据脱敏
    auditLogging: true    # 启用审计日志
```

### 配置示例

#### 1. 纯本地部署（隐私优先）

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
  backends:
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
    openai:
      enabled: false
    gemini:
      enabled: false
  security:
    enableDeId: true
```

**优点**：数据不出域，完全隐私保护
**缺点**：模型能力受本地硬件限制

#### 2. 混合部署（灵活切换）

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
  backends:
    ollama:
      enabled: true
    openai:
      enabled: true
      apiKey: "sk-..."
  security:
    enableDeId: true  # 云端请求自动脱敏
```

**优点**：日常使用本地模型，复杂任务切换云端
**注意**：云端请求会自动脱敏敏感信息

#### 3. 高性能推理服务器

```yaml
ai:
  gateway:
    defaultBackend: "vllm"
  backends:
    vllm:
      enabled: true
      endpoint: "http://localhost:8000"
      tensorParallelSize: 2  # 多GPU并行
      gpuMemoryUtil: 0.9
  resources:
    maxConcurrent: 50
```

**适用场景**：高并发、多用户访问

---

## API 参考

### OpenAI 兼容 API

#### 聊天补全

```http
POST /api/ai/v1/chat/completions
Content-Type: application/json
Authorization: Bearer {token}
```

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| messages | array | 是 | 消息列表 |
| temperature | number | 否 | 温度参数 (0-2)，默认 0.7 |
| max_tokens | integer | 否 | 最大生成令牌数 |
| stream | boolean | 否 | 是否流式输出，默认 false |

**请求示例**：
```json
{
  "model": "llama2",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user", "content": "你好"}
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "stream": false
}
```

**响应示例**：
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1703272800,
  "model": "llama2",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！有什么我可以帮助你的吗？"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 10,
    "total_tokens": 25
  }
}
```

#### 文本补全（Legacy）

```http
POST /api/ai/v1/completions
```

#### 向量嵌入

```http
POST /api/ai/v1/embeddings
```

**请求示例**：
```json
{
  "model": "nomic-embed-text",
  "input": "需要向量化的文本"
}
```

#### 模型列表

```http
GET /api/ai/v1/models
```

### 网关管理 API

#### 网关状态

```http
GET /api/ai/gateway/status
```

**响应示例**：
```json
{
  "status": "running",
  "uptime": "10h30m",
  "totalRequests": 1523,
  "activeConnections": 5,
  "defaultBackend": "ollama"
}
```

#### 性能指标

```http
GET /api/ai/gateway/metrics
```

#### 后端状态

```http
GET /api/ai/gateway/backends
```

**响应示例**：
```json
{
  "backends": [
    {
      "name": "ollama",
      "status": "healthy",
      "latency": "45ms",
      "models": ["llama2", "nomic-embed-text"]
    },
    {
      "name": "openai",
      "status": "healthy",
      "latency": "320ms",
      "models": ["gpt-4", "gpt-3.5-turbo"]
    }
  ]
}
```

#### 模型路由

```http
POST /api/ai/gateway/route
```

**请求示例**：
```json
{
  "modelPattern": "llama*",
  "backend": "ollama"
}
```

### 模型管理 API

#### 列出本地模型

```http
GET /api/ai/models
```

#### 下载模型

```http
POST /api/ai/models/download
```

**请求示例**：
```json
{
  "modelName": "llama2:7b",
  "source": "ollama"
}
```

#### 删除模型

```http
DELETE /api/ai/models/{name}
```

#### 搜索模型

```http
POST /api/ai/models/search
```

#### 存储使用量

```http
GET /api/ai/models/storage
```

### 资源监控 API

#### GPU 信息

```http
GET /api/ai/resources/gpu
```

**响应示例**：
```json
{
  "gpus": [
    {
      "index": 0,
      "name": "NVIDIA RTX 3060",
      "memoryTotal": 12288,
      "memoryUsed": 4096,
      "memoryFree": 8192,
      "utilization": 45.5,
      "temperature": 65
    }
  ]
}
```

#### 内存信息

```http
GET /api/ai/resources/memory
```

#### 所有资源信息

```http
GET /api/ai/resources
```

### AI 控制台 API

#### 摘要生成

```http
POST /api/ai/summarize
```

#### 翻译

```http
POST /api/ai/translate
```

#### 情感分析

```http
POST /api/ai/sentiment
```

#### 提供商列表

```http
GET /api/ai/providers
```

#### 使用统计

```http
GET /api/ai/usage
```

#### 审计日志

```http
GET /api/ai/audit-logs
```

### 数据脱敏 API

#### 脱敏处理

```http
POST /api/ai/desensitize
```

#### 恢复原文

```http
POST /api/ai/restore
```

#### 规则管理

```http
GET    /api/ai/rules
POST   /api/ai/rules
PUT    /api/ai/rules/{id}
DELETE /api/ai/rules/{id}
```

---

## 最佳实践

### 1. 模型选择建议

| 使用场景 | 推荐模型 | 提供商 |
|----------|----------|--------|
| 日常对话 | llama2:7b | Ollama |
| 代码生成 | codellama | Ollama |
| 中文对话 | qwen:7b | Ollama |
| 高质量翻译 | gpt-4 | OpenAI |
| 向量搜索 | nomic-embed-text | Ollama |

### 2. 性能优化

```yaml
# 启用批处理
ai:
  gateway:
    rateLimit:
      concurrencyLimit: 10
  backends:
    ollama:
      contextSize: 4096  # 根据内存调整
```

### 3. 隐私保护

```yaml
ai:
  security:
    enableDeId: true      # 自动脱敏
    auditLogging: true    # 记录所有 AI 调用
```

### 4. 资源监控

```bash
# 监控 GPU 使用
watch -n 1 'curl -s http://localhost:8080/api/ai/resources/gpu | jq'

# 监控请求量
curl http://localhost:8080/api/ai/gateway/metrics
```

### 5. 故障转移配置

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
    fallbackBackend: "openai"  # 本地模型失败时切换云端
  backends:
    ollama:
      enabled: true
    openai:
      enabled: true
      apiKey: "sk-..."
```

---

## 常见问题

### Q1: 如何选择本地模型还是云端服务？

**A**: 根据需求选择：

| 考虑因素 | 本地模型 | 云端服务 |
|----------|----------|----------|
| 隐私要求 | ✅ 数据不出域 | ⚠️ 数据传输到云端 |
| 成本 | ✅ 无使用费用 | ⚠️ 按 Token 计费 |
| 模型能力 | ⚠️ 受硬件限制 | ✅ 最新最强模型 |
| 离线使用 | ✅ 可离线运行 | ❌ 需要网络 |
| 响应速度 | ✅ 低延迟 | ⚠️ 网络延迟 |

### Q2: 本地推理需要什么硬件？

**A**: 硬件需求参考：

| 模型大小 | 最低内存 | 推荐 GPU | 推理速度 |
|----------|----------|----------|----------|
| 7B 参数 | 8GB | RTX 3060 (12GB) | ~20 tokens/s |
| 13B 参数 | 16GB | RTX 4080 (16GB) | ~15 tokens/s |
| 70B 参数 | 64GB | 2× RTX 4090 | ~5 tokens/s |

### Q3: 如何切换默认模型？

**A**: 通过配置文件或 API：

```bash
# 方式一：修改配置文件
# /etc/nas-os/config.yaml
ai:
  gateway:
    defaultBackend: "ollama"
  backends:
    ollama:
      defaultModel: "llama2"

# 方式二：API 调用时指定
curl -X POST .../chat/completions \
  -d '{"model": "gpt-4", "messages": [...]}'
```

### Q4: 如何查看 AI 使用统计？

**A**: 使用 API 或 Web UI：

```bash
# API 方式
curl http://localhost:8080/api/ai/usage

# 审计日志
curl http://localhost:8080/api/ai/audit-logs
```

### Q5: 数据脱敏后如何恢复？

**A**: 使用恢复 API：

```bash
curl -X POST http://localhost:8080/api/ai/restore \
  -H "Content-Type: application/json" \
  -d '{
    "text": "手机号是 [PHONE]",
    "redactions": [
      {"type": "phone", "original": "13812345678", "replacement": "[PHONE]"}
    ]
  }'
```

### Q6: 如何限制 AI 调用频率？

**A**: 配置速率限制：

```yaml
ai:
  gateway:
    rateLimit:
      enabled: true
      requestsPerMin: 60
      tokensPerMin: 100000
```

### Q7: Ollama 无法启动怎么办？

**A**: 排查步骤：

```bash
# 检查 Ollama 服务
systemctl status ollama

# 检查端口
ss -tulpn | grep 11434

# 查看 Ollama 日志
journalctl -u ollama -n 50

# 重启服务
sudo systemctl restart ollama
```

### Q8: 如何添加自定义模型？

**A**: 通过 Ollama 导入：

```bash
# 从 GGUF 文件创建模型
ollama create mymodel -f Modelfile

# 或从 Ollama 库拉取
ollama pull modelname
```

---

## 📞 获取帮助

- **完整 API 文档**: [API_GUIDE.md](API_GUIDE.md)
- **OpenAPI 规范**: http://localhost:8080/swagger/
- **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)

---

*文档版本：v2.274.0 | 最后更新：2026-03-26*