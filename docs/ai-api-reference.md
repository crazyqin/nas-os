# AI 服务 API 参考文档

**版本**: v2.274.0 | **更新日期**: 2026-03-26

---

## 概述

NAS-OS AI 服务提供完整的 RESTful API，支持 OpenAI 兼容接口和原生管理 API。

### 基础 URL

```
http://localhost:8080/api/ai
```

### 认证

所有 API 请求需要 JWT Token 认证：

```bash
# 获取 Token
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 使用 Token
curl http://localhost:8080/api/ai/v1/models \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## OpenAI 兼容 API

### 聊天补全

创建聊天对话补全。

```http
POST /api/ai/v1/chat/completions
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称，如 `llama2`, `gpt-4` |
| messages | array | 是 | 消息列表 |
| temperature | number | 否 | 采样温度，0-2，默认 0.7 |
| max_tokens | integer | 否 | 最大生成 Token 数 |
| top_p | number | 否 | 核采样参数，0-1 |
| n | integer | 否 | 生成数量，默认 1 |
| stream | boolean | 否 | 是否流式输出 |
| stop | array | 否 | 停止词列表 |
| presence_penalty | number | 否 | 存在惩罚，-2.0 到 2.0 |
| frequency_penalty | number | 否 | 频率惩罚，-2.0 到 2.0 |
| user | string | 否 | 用户标识 |

#### 消息格式

```json
{
  "role": "system|user|assistant",
  "content": "消息内容",
  "name": "名称（可选）"
}
```

#### 请求示例

**基础对话**：
```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "user", "content": "介绍一下 NAS 存储"}
    ]
  }'
```

**多轮对话**：
```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "system", "content": "你是一个 NAS 专家"},
      {"role": "user", "content": "什么是 RAID？"},
      {"role": "assistant", "content": "RAID 是磁盘阵列技术..."},
      {"role": "user", "content": "RAID5 和 RAID6 有什么区别？"}
    ]
  }'
```

**流式输出**：
```bash
curl -X POST http://localhost:8080/api/ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "messages": [{"role": "user", "content": "讲一个故事"}],
    "stream": true
  }'
```

#### 响应格式

**普通响应**：
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
        "content": "NAS（网络附加存储）是一种..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 150,
    "total_tokens": 165
  }
}
```

**流式响应** (Server-Sent Events)：
```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"content":"NAS"},"index":0}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"content":"是"},"index":0}]}

data: [DONE]
```

#### 错误响应

```json
{
  "error": {
    "message": "Model not found",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

---

### 文本补全（Legacy）

创建文本补全。

```http
POST /api/ai/v1/completions
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| prompt | string | 是 | 提示文本 |
| max_tokens | integer | 否 | 最大生成 Token 数 |
| temperature | number | 否 | 采样温度 |
| stream | boolean | 否 | 是否流式输出 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "prompt": "NAS 的优点包括：",
    "max_tokens": 100
  }'
```

#### 响应格式

```json
{
  "id": "cmpl-abc123",
  "object": "text_completion",
  "created": 1703272800,
  "model": "llama2",
  "choices": [
    {
      "text": "1. 集中存储管理\n2. 数据共享便捷\n3. 高可靠性...",
      "index": 0,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 50,
    "total_tokens": 60
  }
}
```

---

### 向量嵌入

生成文本的向量表示。

```http
POST /api/ai/v1/embeddings
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 向量模型名称 |
| input | string/array | 是 | 输入文本 |
| encoding_format | string | 否 | 编码格式：float, base64 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text",
    "input": "NAS 是网络附加存储设备"
  }'
```

#### 响应格式

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.012, -0.034, 0.056, ...]
    }
  ],
  "model": "nomic-embed-text",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}
```

---

### 模型列表

列出所有可用模型。

```http
GET /api/ai/v1/models
```

#### 请求示例

```bash
curl http://localhost:8080/api/ai/v1/models
```

#### 响应格式

```json
{
  "object": "list",
  "data": [
    {
      "id": "llama2",
      "object": "model",
      "created": 1703272800,
      "owned_by": "ollama"
    },
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1703272800,
      "owned_by": "openai"
    }
  ]
}
```

---

### 模型详情

获取特定模型信息。

```http
GET /api/ai/v1/models/{model}
```

#### 请求示例

```bash
curl http://localhost:8080/api/ai/v1/models/llama2
```

#### 响应格式

```json
{
  "id": "llama2",
  "object": "model",
  "created": 1703272800,
  "owned_by": "ollama",
  "permission": [],
  "root": "llama2",
  "parent": null
}
```

---

## 网关管理 API

### 网关状态

获取 AI 网关运行状态。

```http
GET /api/ai/gateway/status
```

#### 响应示例

```json
{
  "status": "running",
  "uptime": "10h30m15s",
  "totalRequests": 1523,
  "activeConnections": 5,
  "defaultBackend": "ollama",
  "startTime": "2026-03-26T08:00:00Z"
}
```

---

### 性能指标

获取性能和统计指标。

```http
GET /api/ai/gateway/metrics
```

#### 响应示例

```json
{
  "requests": {
    "total": 15234,
    "success": 15000,
    "error": 234,
    "successRate": 0.985
  },
  "latency": {
    "p50": 45,
    "p90": 120,
    "p99": 350,
    "avg": 55
  },
  "tokens": {
    "prompt": 150000,
    "completion": 300000,
    "total": 450000
  },
  "byBackend": {
    "ollama": {
      "requests": 10000,
      "avgLatency": 35
    },
    "openai": {
      "requests": 5234,
      "avgLatency": 320
    }
  }
}
```

---

### 后端状态

列出所有后端及其健康状态。

```http
GET /api/ai/gateway/backends
```

#### 响应示例

```json
{
  "backends": [
    {
      "name": "ollama",
      "status": "healthy",
      "endpoint": "http://localhost:11434",
      "latency": "35ms",
      "models": ["llama2", "qwen", "nomic-embed-text"],
      "lastCheck": "2026-03-26T10:30:00Z"
    },
    {
      "name": "openai",
      "status": "healthy",
      "endpoint": "https://api.openai.com",
      "latency": "320ms",
      "models": ["gpt-4", "gpt-3.5-turbo"],
      "lastCheck": "2026-03-26T10:30:00Z"
    }
  ]
}
```

---

### 设置模型路由

配置模型到后端的路由规则。

```http
POST /api/ai/gateway/route
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| modelPattern | string | 是 | 模型名称匹配模式 |
| backend | string | 是 | 目标后端名称 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/gateway/route \
  -H "Content-Type: application/json" \
  -d '{
    "modelPattern": "gpt-*",
    "backend": "openai"
  }'
```

#### 响应示例

```json
{
  "success": true,
  "message": "Route added: gpt-* -> openai"
}
```

---

## 模型管理 API

### 列出本地模型

获取已下载的本地模型列表。

```http
GET /api/ai/models
```

#### 响应示例

```json
{
  "models": [
    {
      "name": "llama2:7b",
      "size": 3825816320,
      "modified": "2026-03-25T10:00:00Z",
      "digest": "abc123...",
      "details": {
        "format": "gguf",
        "parameter_size": "7B",
        "quantization": "q4_0"
      }
    }
  ]
}
```

---

### 下载模型

下载新模型。

```http
POST /api/ai/models/download
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| modelName | string | 是 | 模型名称 |
| source | string | 是 | 来源：ollama, huggingface |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/models/download \
  -H "Content-Type: application/json" \
  -d '{
    "modelName": "llama2:13b",
    "source": "ollama"
  }'
```

#### 响应示例

```json
{
  "jobId": "dl-abc123",
  "status": "downloading",
  "progress": 0,
  "message": "Download started"
}
```

---

### 获取下载进度

查询模型下载进度。

```http
GET /api/ai/models/{name}/progress
```

#### 响应示例

```json
{
  "modelName": "llama2:13b",
  "status": "downloading",
  "progress": 45.5,
  "downloadedBytes": 1800000000,
  "totalBytes": 4000000000,
  "speed": "25MB/s",
  "eta": "2m30s"
}
```

---

### 删除模型

删除本地模型。

```http
DELETE /api/ai/models/{name}
```

#### 请求示例

```bash
curl -X DELETE http://localhost:8080/api/ai/models/llama2:7b
```

#### 响应示例

```json
{
  "success": true,
  "message": "Model llama2:7b deleted"
}
```

---

### 搜索模型

在模型库中搜索模型。

```http
POST /api/ai/models/search
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 搜索关键词 |
| source | string | 否 | 来源过滤 |
| limit | integer | 否 | 返回数量，默认 10 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/models/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "chinese",
    "source": "ollama",
    "limit": 5
  }'
```

#### 响应示例

```json
{
  "results": [
    {
      "name": "qwen:7b",
      "description": "通义千问 7B 中文模型",
      "size": 4000000000,
      "downloads": 100000,
      "tags": ["chinese", "chat"]
    }
  ]
}
```

---

### 存储使用量

获取模型存储使用情况。

```http
GET /api/ai/models/storage
```

#### 响应示例

```json
{
  "totalSize": 15000000000,
  "usedSize": 8000000000,
  "freeSize": 7000000000,
  "usagePercent": 53.3,
  "modelCount": 5,
  "models": [
    {
      "name": "llama2:7b",
      "size": 3825816320
    },
    {
      "name": "qwen:7b",
      "size": 4175000000
    }
  ]
}
```

---

## 资源监控 API

### GPU 信息

获取 GPU 设备信息。

```http
GET /api/ai/resources/gpu
```

#### 响应示例

```json
{
  "gpus": [
    {
      "index": 0,
      "name": "NVIDIA GeForce RTX 3060",
      "driverVersion": "535.104.05",
      "cudaVersion": "12.2",
      "memoryTotal": 12288,
      "memoryUsed": 4096,
      "memoryFree": 8192,
      "utilization": 45.5,
      "temperature": 65
    }
  ]
}
```

---

### 内存信息

获取系统内存信息。

```http
GET /api/ai/resources/memory
```

#### 响应示例

```json
{
  "total": 16777216,
  "used": 8388608,
  "free": 4194304,
  "available": 12582912,
  "usagePercent": 50.0,
  "swapTotal": 8388608,
  "swapUsed": 0
}
```

---

### 所有资源信息

获取完整的资源信息。

```http
GET /api/ai/resources
```

#### 响应示例

```json
{
  "gpu": {
    "available": true,
    "devices": [...]
  },
  "memory": {
    "total": 16777216,
    "used": 8388608
  },
  "cpu": {
    "cores": 8,
    "usage": 25.5
  },
  "disk": {
    "total": 1000000000000,
    "used": 500000000000
  }
}
```

---

## AI 控制台 API

### 摘要生成

生成文本摘要。

```http
POST /api/ai/summarize
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content | string | 是 | 待摘要文本 |
| maxLength | integer | 否 | 最大长度，默认 200 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/summarize \
  -H "Content-Type: application/json" \
  -d '{
    "content": "长文本内容...",
    "maxLength": 100
  }'
```

#### 响应示例

```json
{
  "summary": "这是文本的摘要内容..."
}
```

---

### 翻译

文本翻译。

```http
POST /api/ai/translate
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| text | string | 是 | 待翻译文本 |
| targetLang | string | 是 | 目标语言 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/translate \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello, world!",
    "targetLang": "zh-CN"
  }'
```

#### 响应示例

```json
{
  "translation": "你好，世界！"
}
```

---

### 情感分析

分析文本情感。

```http
POST /api/ai/sentiment
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| text | string | 是 | 待分析文本 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/sentiment \
  -H "Content-Type: application/json" \
  -d '{
    "text": "这个产品太棒了！"
  }'
```

#### 响应示例

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

---

### 提供商列表

获取可用 AI 提供商。

```http
GET /api/ai/providers
```

#### 响应示例

```json
{
  "providers": [
    {
      "name": "ollama",
      "type": "local",
      "enabled": true,
      "models": ["llama2", "qwen"]
    },
    {
      "name": "openai",
      "type": "cloud",
      "enabled": true,
      "models": ["gpt-4", "gpt-3.5-turbo"]
    }
  ]
}
```

---

### 使用统计

获取 AI 使用统计。

```http
GET /api/ai/usage
```

#### 请求示例

```bash
curl "http://localhost:8080/api/ai/usage?period=7d"
```

#### 响应示例

```json
{
  "period": "7d",
  "requests": {
    "total": 15234,
    "byDay": [
      {"date": "2026-03-20", "count": 2000},
      {"date": "2026-03-21", "count": 2500}
    ]
  },
  "tokens": {
    "prompt": 150000,
    "completion": 300000,
    "total": 450000
  },
  "byProvider": {
    "ollama": 10000,
    "openai": 5234
  }
}
```

---

### 审计日志

获取 AI 调用审计日志。

```http
GET /api/ai/audit-logs
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| userId | string | 否 | 用户 ID 过滤 |
| since | string | 否 | 起始时间 (RFC3339) |
| limit | integer | 否 | 返回数量，默认 100 |

#### 响应示例

```json
{
  "logs": [
    {
      "id": "log-001",
      "timestamp": "2026-03-26T10:00:00Z",
      "userId": "user-123",
      "action": "chat",
      "model": "llama2",
      "provider": "ollama",
      "tokensUsed": 150,
      "latency": 45,
      "status": "success"
    }
  ]
}
```

---

## 数据脱敏 API

### 脱敏处理

识别并替换敏感信息。

```http
POST /api/ai/desensitize
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| text | string | 是 | 待处理文本 |
| types | array | 否 | 脱敏类型过滤 |

#### 请求示例

```bash
curl -X POST http://localhost:8080/api/ai/desensitize \
  -H "Content-Type: application/json" \
  -d '{
    "text": "张三的手机号是 13812345678，身份证 110101199001011234"
  }'
```

#### 响应示例

```json
{
  "text": "张三的手机号是 [PHONE]，身份证 [ID]",
  "redactions": [
    {
      "type": "phone",
      "original": "13812345678",
      "replacement": "[PHONE]",
      "start": 7,
      "end": 18
    },
    {
      "type": "id_card",
      "original": "110101199001011234",
      "replacement": "[ID]",
      "start": 23,
      "end": 41
    }
  ]
}
```

---

### 恢复原文

恢复脱敏后的文本。

```http
POST /api/ai/restore
```

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| text | string | 是 | 脱敏文本 |
| redactions | array | 是 | 脱敏记录 |

#### 请求示例

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

#### 响应示例

```json
{
  "text": "手机号是 13812345678"
}
```

---

### 规则管理

#### 获取规则列表

```http
GET /api/ai/rules
```

#### 添加规则

```http
POST /api/ai/rules
```

#### 更新规则

```http
PUT /api/ai/rules/{id}
```

#### 删除规则

```http
DELETE /api/ai/rules/{id}
```

---

## 错误码

| 代码 | 说明 |
|------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 429 | 请求频率超限 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用 |

---

## SDK 示例

### Python

```python
import requests

BASE_URL = "http://localhost:8080/api/ai"
TOKEN = "your-jwt-token"

headers = {"Authorization": f"Bearer {TOKEN}"}

# 聊天补全
response = requests.post(
    f"{BASE_URL}/v1/chat/completions",
    headers=headers,
    json={
        "model": "llama2",
        "messages": [{"role": "user", "content": "Hello!"}]
    }
)
print(response.json())

# 流式输出
import sseclient

response = requests.post(
    f"{BASE_URL}/v1/chat/completions",
    headers=headers,
    json={
        "model": "llama2",
        "messages": [{"role": "user", "content": "Tell a story"}],
        "stream": True
    },
    stream=True
)

client = sseclient.SSEClient(response)
for event in client.events():
    if event.data != "[DONE]":
        print(event.data)
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

func main() {
    baseURL := "http://localhost:8080/api/ai"
    token := "your-jwt-token"

    req := map[string]interface{}{
        "model": "llama2",
        "messages": []map[string]string{
            {"role": "user", "content": "Hello!"},
        },
    }

    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+token)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, _ := http.DefaultClient.Do(httpReq)
    defer resp.Body.Close()

    data, _ := io.ReadAll(resp.Body)
    fmt.Println(string(data))
}
```

---

*文档版本：v2.274.0 | 最后更新：2026-03-26*