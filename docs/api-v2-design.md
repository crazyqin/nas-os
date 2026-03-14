# API v2 设计文档

## 概述

NAS-OS API v2 引入了版本化 API 和 WebSocket 实时通信支持，提供更好的向后兼容性和实时通知能力。

## API 版本化

### 版本策略

- **URL 路径版本化**: `/api/v1/...`, `/api/v2/...`
- **版本发现端点**: `GET /api/versions`
- **废弃通知**: 通过 HTTP 头传递废弃信息

### 支持的版本

| 版本 | 状态 | 废弃日期 | 说明 |
|------|------|----------|------|
| v1 | 当前稳定 | - | 推荐 |
| v0 | 已废弃 | 2026-06-01 | 请迁移到 v1 |

### 版本发现

```http
GET /api/versions

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "current": "v1",
    "versions": [
      {
        "version": "v1",
        "deprecated": false
      },
      {
        "version": "v0",
        "deprecated": true,
        "sunsetDate": "2026-06-01"
      }
    ]
  }
}
```

### 废弃响应头

```http
X-API-Deprecated: true
X-API-Deprecation-Reason: 升级到 v1 API
X-API-Removal-Date: 2026-06-01
X-API-Alternatives: /api/v1/...
```

## WebSocket 实时通信

### 连接

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');
```

### 消息格式

```json
{
  "type": "notification|metric|alert|event|container|storage|backup|sync",
  "timestamp": 1710374400,
  "data": { ... }
}
```

### 消息类型

| 类型 | 说明 | 数据示例 |
|------|------|----------|
| system | 系统消息 | `{"pong": true}` |
| notification | 通知消息 | `{"title": "...", "message": "...", "level": "info"}` |
| metric | 性能指标 | `{"cpu": 45.5, "memory": 60.2, "disk": 70.0}` |
| alert | 告警消息 | `{"id": "...", "severity": "warning", "title": "..."}` |
| container | 容器事件 | `{"containerId": "...", "action": "start"}` |
| storage | 存储事件 | `{"volumeName": "...", "eventType": "error"}` |
| backup | 备份事件 | `{"jobId": "...", "status": "completed"}` |

### 订阅机制

客户端可以订阅特定类型的消息：

```json
{
  "action": "subscribe",
  "subscriptions": ["metric", "alert"]
}
```

取消订阅：

```json
{
  "action": "unsubscribe",
  "subscriptions": ["metric"]
}
```

### 心跳

客户端发送 ping，服务器返回 pong：

```json
// 发送
{"action": "ping"}

// 接收
{
  "type": "system",
  "timestamp": 1710374400,
  "data": {"pong": true}
}
```

## 迁移指南

### v0 → v1 迁移

1. 更新 API 基础路径：`/api/v0` → `/api/v1`
2. 检查废弃的端点（参考废弃通知）
3. 更新响应格式处理

### 使用 WebSocket

```javascript
// 初始化 WebSocket
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onopen = () => {
  // 订阅需要的消息类型
  ws.send(JSON.stringify({
    action: 'subscribe',
    subscriptions: ['metric', 'alert', 'notification']
  }));
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  switch (message.type) {
    case 'metric':
      updateDashboard(message.data);
      break;
    case 'alert':
      showAlert(message.data);
      break;
    case 'notification':
      showNotification(message.data);
      break;
  }
};

// 心跳
setInterval(() => {
  ws.send(JSON.stringify({ action: 'ping' }));
}, 30000);
```

## 错误处理

### 版本错误

```json
{
  "code": 400,
  "message": "Unsupported API version: v3",
  "data": {
    "currentVersion": "v1",
    "supportedVersions": ["v0", "v1"]
  }
}
```

### WebSocket 错误

WebSocket 连接错误通过标准 WebSocket 事件处理：

```javascript
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = (event) => {
  console.log('WebSocket closed:', event.code, event.reason);
  // 重连逻辑
};
```

## 安全考虑

- WebSocket 连接需要认证
- 敏感操作需要重新验证
- 消息频率限制
- 输入验证和清理