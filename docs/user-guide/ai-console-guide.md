# AI Console 隐私数据脱敏 (AI Console & PII Redaction)

> **适用版本**: v2.482.0+ | **模块**: `internal/aiconsole`

AI Console 提供统一的 AI 模型管理、隐私数据自动脱敏和完整审计日志，确保 AI 对话中的敏感信息不会泄露给外部模型服务。

## 核心特性

| 特性 | 说明 |
|------|------|
| 🤖 多模型管理 | 支持 OpenAI 兼容 API、本地 LLM（Ollama）、自定义提供者 |
| 🔐 PII 自动脱敏 | 发送前自动识别并替换敏感信息，支持 8 种 PII 类型 |
| 📋 完整审计日志 | 记录每次 AI 请求的用户、模型、Token 用量、脱敏详情 |
| 🔄 多种脱敏策略 | 掩码、部分显示、哈希替换、完全移除 |
| 🎯 优先级控制 | 规则按优先级排序，高优先级规则先处理 |

## 支持的 PII 类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `email` | 邮箱地址 | `user@example.com` → `u***@e***.com` |
| `phone` | 电话号码 | `13812345678` → `138****5678` |
| `id_card` | 身份证号 | `110105199001011234` → `110105****1234` |
| `bank_card` | 银行卡号 | `6222021234567890123` → `6222****0123` |
| `name` | 中文姓名 | `张三` → `*三` |
| `passport` | 护照号 | `E12345678` → `E***5678` |
| `ip_address` | IP 地址 | `192.168.1.100` → `192.168.*.*` |
| `custom` | 自定义规则 | 用户定义正则和替换策略 |

## 脱敏策略

| 策略 | 说明 | 示例（原值 `13812345678`） |
|------|------|---------------------------|
| `mask` | 掩码替换 | `138****5678` |
| `partial` | 部分显示（可配置前 N 位 + 后 N 位） | `138****5678` |
| `hash` | SHA-256 哈希 | `a3f2b1c8...` |
| `remove` | 完全移除 | *(空)* |

## 快速开始

### 1. 添加 AI 模型

```bash
curl -X POST http://localhost:8080/api/v1/aiconsole/models \
  -H "Content-Type: application/json" \
  -d '{
    "name": "本地 Llama3",
    "provider": "local",
    "endpoint": "http://localhost:11434",
    "modelName": "llama3",
    "maxTokens": 4096,
    "temperature": 0.7,
    "isDefault": true
  }'
```

### 2. 创建脱敏规则

```bash
curl -X POST http://localhost:8080/api/v1/aiconsole/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "手机号脱敏",
    "piiType": "phone",
    "pattern": "1[3-9]\\d{9}",
    "strategy": "mask",
    "showFirst": 3,
    "showLast": 4,
    "enabled": true,
    "priority": 100
  }'
```

### 3. 发送 AI 对话（自动脱敏）

```bash
curl -X POST http://localhost:8080/api/v1/aiconsole/chat \
  -H "Content-Type: application/json" \
  -d '{
    "modelId": "model-xxx",
    "messages": [
      {"role": "user", "content": "我的手机号是13812345678，请帮我记录一下"}
    ]
  }'
```

响应中会包含：
```json
{
  "redacted": true,
  "redactCount": 1,
  "content": "好的，已记录。"
}
```

## API 接口

所有接口挂载在 `/api/v1/aiconsole/` 下。

### 模型管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/models` | 列出所有模型 |
| `POST` | `/models` | 添加模型 |
| `PUT` | `/models/{id}` | 更新模型 |
| `DELETE` | `/models/{id}` | 删除模型 |
| `POST` | `/models/{id}/test` | 测试模型连接 |

### 脱敏规则管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/rules` | 列出所有规则 |
| `POST` | `/rules` | 创建规则 |
| `PUT` | `/rules/{id}` | 更新规则 |
| `DELETE` | `/rules/{id}` | 删除规则 |
| `POST` | `/rules/test` | 测试规则匹配效果 |

### AI 对话

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/chat` | 发送对话（自动脱敏） |
| `POST` | `/chat/stream` | 流式对话（SSE） |

### 审计日志

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/audit?page=&pageSize=` | 查询审计日志 |
| `GET` | `/audit/stats` | 审计统计摘要 |

## 脱敏流程

```
用户输入 → PII 正则匹配 → 按优先级排序 → 执行脱敏策略 → 替换后发送给 AI → 记录审计日志
```

1. 用户发送消息前，引擎按优先级从高到低逐条匹配
2. 匹配到的 PII 按对应策略替换
3. 替换后的文本发送给 AI 模型
4. AI 原始请求和脱敏结果写入审计日志
5. 响应返回时标记 `redacted: true` 和 `redactCount`

## 内置默认规则

系统预置以下脱敏规则（均可自定义修改）：

| 规则 | PII 类型 | 策略 | 匹配 |
|------|----------|------|------|
| 手机号 | `phone` | `mask` | `1[3-9]\d{9}` |
| 邮箱 | `email` | `partial` | 标准邮箱格式 |
| 身份证 | `id_card` | `mask` | 18 位身份证 |
| 银行卡 | `bank_card` | `mask` | 16-19 位纯数字 |
| 中文姓名 | `name` | `mask` | 2-4 位中文 |
| 护照 | `passport` | `partial` | 护照号格式 |
| IP 地址 | `ip_address` | `mask` | IPv4 地址 |

## 安全说明

- 模型 API Key 存储时加密，不明文保存
- 审计日志中不记录 PII 原文（`original` 字段标记为 `json:"-"`）
- 脱敏处理在本地完成，敏感信息不会发送到外部模型服务

---

*最后更新：2026-05-06 | 礼部文档完善*
