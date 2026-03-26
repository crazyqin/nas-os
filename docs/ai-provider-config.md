# AI 服务多提供商配置教程

**版本**: v2.274.0 | **更新日期**: 2026-03-26

---

## 概述

NAS-OS AI 服务支持多种 AI 提供商，包括本地推理引擎和云端 AI 服务。本教程将详细介绍如何配置和管理多个提供商。

---

## 目录

- [提供商概览](#提供商概览)
- [本地推理引擎配置](#本地推理引擎配置)
- [云端 AI 服务配置](#云端-ai-服务配置)
- [混合部署策略](#混合部署策略)
- [模型路由配置](#模型路由配置)
- [故障转移与高可用](#故障转移与高可用)

---

## 提供商概览

### 本地推理引擎

| 引擎 | 特点 | 适用场景 |
|------|------|----------|
| **Ollama** | 简单易用、社区活跃 | 个人用户、快速部署 |
| **LocalAI** | OpenAI 兼容、高性能 | 开发者、生产环境 |
| **vLLM** | 高吞吐量、GPU 优化 | 高并发、企业级 |

### 云端 AI 服务

| 服务 | 特点 | 适用场景 |
|------|------|----------|
| **OpenAI** | 模型能力强、生态完善 | 通用场景、高质量需求 |
| **Google Gemini** | 多模态、长上下文 | 图像理解、长文本处理 |
| **Azure OpenAI** | 企业合规、数据安全 | 企业级应用 |
| **百度文心** | 中文优化、国内服务 | 中文场景、合规要求 |

---

## 本地推理引擎配置

### Ollama 配置

#### 1. 安装 Ollama

```bash
# Linux 一键安装
curl -fsSL https://ollama.com/install.sh | sh

# macOS
brew install ollama

# Windows
# 访问 https://ollama.com/download 下载安装包
```

#### 2. 启动服务

```bash
# 启动 Ollama 服务
ollama serve

# 或作为系统服务
sudo systemctl enable ollama
sudo systemctl start ollama
```

#### 3. 下载模型

```bash
# 对话模型
ollama pull llama2:7b
ollama pull llama2:13b
ollama pull qwen:7b        # 中文模型
ollama pull codellama:7b   # 代码模型

# 向量模型
ollama pull nomic-embed-text
ollama pull mxbai-embed-large
```

#### 4. NAS-OS 配置

编辑 `/etc/nas-os/config.yaml`：

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
  
  backends:
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
      defaultModel: "llama2"
      gpuLayers: 35        # GPU 加速层数，0 为纯 CPU
      contextSize: 4096    # 上下文窗口大小
      modelPath: "/var/lib/nas-os/ai/models/ollama"
```

#### 5. 验证配置

```bash
# 检查 Ollama 状态
curl http://localhost:11434/api/tags

# 测试推理
curl http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama2", "messages": [{"role": "user", "content": "hello"}]}'
```

### LocalAI 配置

#### 1. 安装 LocalAI

```bash
# Docker 方式（推荐）
docker run -d \
  --name localai \
  -p 8080:8080 \
  -v $PWD/models:/models \
  localai/localai:latest

# 二进制安装
wget https://github.com/go-skynet/LocalAI/releases/download/v2.0.0/local-ai-linux-amd64
chmod +x local-ai-linux-amd64
sudo mv local-ai-linux-amd64 /usr/local/bin/local-ai
```

#### 2. 配置模型

创建模型配置文件 `models/gpt-3.5-turbo.yaml`：

```yaml
name: gpt-3.5-turbo
parameters:
  model: llama-2-7b-chat.gguf
  temperature: 0.7
  top_p: 0.9
  max_tokens: 2048
context_size: 4096
threads: 8
gpu_layers: 35
```

#### 3. NAS-OS 配置

```yaml
ai:
  backends:
    localai:
      enabled: true
      endpoint: "http://localhost:8080"
      defaultModel: "gpt-3.5-turbo"
      threads: 8
      contextSize: 4096
```

### vLLM 配置

#### 1. 安装 vLLM

```bash
# 需要 NVIDIA GPU 和 CUDA
pip install vllm

# 或使用 Docker
docker run -d \
  --gpus all \
  --name vllm \
  -p 8000:8000 \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  vllm/vllm-openai:latest \
  --model meta-llama/Llama-2-7b-chat-hf
```

#### 2. NAS-OS 配置

```yaml
ai:
  backends:
    vllm:
      enabled: true
      endpoint: "http://localhost:8000"
      defaultModel: "meta-llama/Llama-2-7b-chat-hf"
      tensorParallelSize: 1    # GPU 数量
      gpuMemoryUtil: 0.9       # GPU 内存利用率
```

---

## 云端 AI 服务配置

### OpenAI 配置

#### 1. 获取 API Key

1. 访问 [OpenAI Platform](https://platform.openai.com/)
2. 登录并进入 API Keys 页面
3. 点击 "Create new secret key" 创建密钥
4. 复制并保存 API Key

#### 2. NAS-OS 配置

```yaml
ai:
  backends:
    openai:
      enabled: true
      apiKey: "sk-proj-xxxxx"
      defaultModel: "gpt-4-turbo"
      organization: ""         # 可选，组织 ID
      baseUrl: ""              # 可选，自定义端点
```

#### 3. 安全建议

```yaml
ai:
  security:
    enableDeId: true    # 启用数据脱敏，保护隐私
    auditLogging: true  # 记录所有云端请求
```

#### 4. 测试连接

```bash
curl http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Google Gemini 配置

#### 1. 获取 API Key

1. 访问 [Google AI Studio](https://aistudio.google.com/)
2. 点击 "Get API Key"
3. 创建并复制 API Key

#### 2. NAS-OS 配置

```yaml
ai:
  backends:
    gemini:
      enabled: true
      apiKey: "AIzaxxxxx"
      defaultModel: "gemini-pro"
      # 可用模型: gemini-pro, gemini-1.5-pro, gemini-1.5-flash
```

### Azure OpenAI 配置

#### 1. 创建 Azure OpenAI 资源

1. 登录 [Azure Portal](https://portal.azure.com/)
2. 创建 "Azure OpenAI" 资源
3. 获取端点 URL 和 API Key
4. 部署模型（如 gpt-4, gpt-35-turbo）

#### 2. NAS-OS 配置

```yaml
ai:
  backends:
    azure:
      enabled: true
      endpoint: "https://your-resource.openai.azure.com"
      apiKey: "xxxxx"
      defaultModel: "gpt-4"
      apiVersion: "2024-02-15-preview"
      deploymentName: "gpt-4-deployment"
```

### 百度文心配置

#### 1. 获取访问凭证

1. 访问 [百度智能云](https://cloud.baidu.com/)
2. 开通文心一言服务
3. 创建应用获取 API Key 和 Secret Key

#### 2. NAS-OS 配置

```yaml
ai:
  backends:
    baidu:
      enabled: true
      apiKey: "xxxxx"
      secretKey: "xxxxx"
      defaultModel: "ernie-bot-4"
      # 可用模型: ernie-bot-4, ernie-bot, ernie-bot-turbo
```

---

## 混合部署策略

### 场景一：隐私优先 + 云端备份

**需求**：日常使用本地模型，敏感数据不出域；云端服务作为备用。

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
    fallbackBackend: "openai"
  
  backends:
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
      defaultModel: "llama2"
    
    openai:
      enabled: true
      apiKey: "sk-..."
      defaultModel: "gpt-4"
  
  security:
    enableDeId: true    # 云端请求自动脱敏
```

**工作流程**：
1. 默认使用 Ollama 本地推理
2. Ollama 不可用时自动切换 OpenAI
3. 云端请求前自动脱敏敏感信息

### 场景二：模型能力分层

**需求**：简单任务用本地模型，复杂任务用云端大模型。

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
  
  backends:
    ollama:
      enabled: true
      defaultModel: "llama2"
    
    openai:
      enabled: true
      apiKey: "sk-..."
  
  # 模型路由规则
  routes:
    - pattern: "gpt-*"
      backend: "openai"
    - pattern: "llama*"
      backend: "ollama"
    - pattern: "qwen*"
      backend: "ollama"
```

**使用方式**：
```bash
# 使用本地模型（默认）
curl -d '{"model": "llama2", "messages": [...]}'

# 使用云端大模型
curl -d '{"model": "gpt-4", "messages": [...]}'
```

### 场景三：中文优化

**需求**：中文场景用百度文心或 Qwen，英文场景用 OpenAI。

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
  
  backends:
    ollama:
      enabled: true
      defaultModel: "qwen:7b"  # 中文模型
    
    openai:
      enabled: true
      apiKey: "sk-..."
    
    baidu:
      enabled: true
      apiKey: "..."
      secretKey: "..."
```

---

## 模型路由配置

### 自动路由规则

```yaml
ai:
  gateway:
    routes:
      # 按模型名称路由
      - pattern: "gpt-*"
        backend: "openai"
      
      - pattern: "gemini-*"
        backend: "gemini"
      
      - pattern: "ernie-*"
        backend: "baidu"
      
      # 本地模型
      - pattern: "llama*"
        backend: "ollama"
      
      - pattern: "qwen*"
        backend: "ollama"
      
      - pattern: "*-embed-*"
        backend: "ollama"
```

### 动态路由

通过 API 设置路由：

```bash
# 设置模型路由
curl -X POST http://localhost:8080/api/ai/gateway/route \
  -H "Content-Type: application/json" \
  -d '{
    "modelPattern": "gpt-*",
    "backend": "openai"
  }'

# 查看当前路由
curl http://localhost:8080/api/ai/gateway/backends
```

---

## 故障转移与高可用

### 自动故障转移

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
    fallbackBackend: "openai"
    
    healthCheck:
      enabled: true
      interval: 30s
      timeout: 5s
      unhealthyThreshold: 3
  
  backends:
    ollama:
      enabled: true
      endpoint: "http://localhost:11434"
    
    openai:
      enabled: true
      apiKey: "sk-..."
```

**工作原理**：
1. 定期检查后端健康状态
2. 主后端连续失败 3 次后标记为不健康
3. 自动切换到备用后端
4. 主后端恢复后自动切换回来

### 多后端负载均衡

```yaml
ai:
  gateway:
    defaultBackend: "ollama"
    loadBalance:
      enabled: true
      strategy: "round-robin"  # round-robin, least-connections, random
  
  backends:
    ollama-1:
      enabled: true
      endpoint: "http://192.168.1.10:11434"
    
    ollama-2:
      enabled: true
      endpoint: "http://192.168.1.11:11434"
    
    ollama-3:
      enabled: true
      endpoint: "http://192.168.1.12:11434"
```

---

## 监控与调试

### 查看后端状态

```bash
# 所有后端状态
curl http://localhost:8080/api/ai/gateway/backends

# 单个后端健康检查
curl http://localhost:8080/api/ai/gateway/backends/ollama/health
```

### 查看请求日志

```bash
# 审计日志
curl http://localhost:8080/api/ai/audit-logs

# 使用统计
curl http://localhost:8080/api/ai/usage
```

### Prometheus 指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:8080/metrics | grep ai_

# 主要指标
# ai_requests_total - 总请求数
# ai_request_duration_seconds - 请求延迟
# ai_backend_requests_total - 按后端统计
# ai_tokens_total - Token 使用量
```

---

## 常见问题

### Q1: 如何测试提供商连接？

```bash
# 测试 OpenAI 连接
curl http://localhost:8080/api/ai/gateway/backends/openai/health

# 测试本地 Ollama
curl http://localhost:11434/api/tags
```

### Q2: 云端 API Key 泄露怎么办？

1. 立即在提供商控制台撤销旧 Key
2. 生成新 Key
3. 更新 NAS-OS 配置
4. 检查审计日志确认未授权使用

### Q3: 如何限制云端 API 费用？

```yaml
ai:
  gateway:
    rateLimit:
      enabled: true
      tokensPerMin: 10000    # 每分钟 Token 限制
  
  backends:
    openai:
      enabled: true
      maxTokensPerMonth: 1000000  # 月度 Token 限制
```

### Q4: 本地模型推理太慢怎么办？

1. 增加 `gpuLayers` 参数启用 GPU 加速
2. 使用量化模型（如 llama2:7b-q4）
3. 减小 `contextSize` 降低内存占用
4. 考虑使用 vLLM 提高吞吐量

---

## 参考链接

- [Ollama 官方文档](https://ollama.com/docs)
- [LocalAI GitHub](https://github.com/go-skynet/LocalAI)
- [vLLM 文档](https://vllm.readthedocs.io/)
- [OpenAI API 文档](https://platform.openai.com/docs)
- [Google Gemini API](https://ai.google.dev/docs)

---

*文档版本：v2.274.0 | 最后更新：2026-03-26*