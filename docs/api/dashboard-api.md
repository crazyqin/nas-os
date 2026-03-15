# 仪表板 API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

## 概述

仪表板 API 提供系统监控仪表板的创建、配置和数据获取功能。

## 基础路径

```
/api/v1/dashboard
```

---

## 仪表板管理

### 获取仪表板列表

```http
GET /api/v1/dashboard
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "dashboards": [
      {
        "id": "default",
        "name": "系统概览",
        "description": "默认系统监控仪表板",
        "widgetCount": 6,
        "isDefault": true,
        "createdAt": "2026-03-15T10:00:00Z",
        "updatedAt": "2026-03-15T12:00:00Z"
      }
    ]
  }
}
```

### 创建仪表板

```http
POST /api/v1/dashboard
Content-Type: application/json

{
  "name": "我的仪表板",
  "description": "自定义监控仪表板",
  "widgets": [
    {
      "type": "cpu",
      "title": "CPU 使用率",
      "size": "medium",
      "position": {"x": 0, "y": 0},
      "config": {
        "showPerCore": true,
        "warningThreshold": 80,
        "criticalThreshold": 95
      }
    }
  ],
  "layout": {
    "columns": 4,
    "rows": 3,
    "gap": 16
  }
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "db-12345",
    "name": "我的仪表板",
    "createdAt": "2026-03-15T12:30:00Z"
  }
}
```

### 获取仪表板详情

```http
GET /api/v1/dashboard/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "default",
    "name": "系统概览",
    "description": "默认系统监控仪表板",
    "widgets": [
      {
        "id": "cpu-1",
        "type": "cpu",
        "title": "CPU 使用率",
        "size": "medium",
        "position": {"x": 0, "y": 0},
        "enabled": true,
        "refreshRate": 5000000000
      }
    ],
    "layout": {
      "columns": 4,
      "rows": 3,
      "gap": 16
    },
    "isDefault": true,
    "createdAt": "2026-03-15T10:00:00Z",
    "updatedAt": "2026-03-15T12:00:00Z"
  }
}
```

### 更新仪表板

```http
PUT /api/v1/dashboard/:id
Content-Type: application/json

{
  "name": "更新后的名称",
  "widgets": [...],
  "layout": {...}
}
```

### 删除仪表板

```http
DELETE /api/v1/dashboard/:id
```

---

## 小组件数据

### 获取小组件实时数据

```http
GET /api/v1/dashboard/:id/widgets
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "dashboardId": "default",
    "lastUpdate": "2026-03-15T12:35:00Z",
    "widgetData": {
      "cpu-1": {
        "widgetId": "cpu-1",
        "type": "cpu",
        "timestamp": "2026-03-15T12:35:00Z",
        "data": {
          "usage": 35.2,
          "perCore": [32.1, 38.5, 31.2, 39.0],
          "loadAvg1": 1.25,
          "loadAvg5": 1.10,
          "loadAvg15": 0.95,
          "processCount": 156
        }
      },
      "memory-1": {
        "widgetId": "memory-1",
        "type": "memory",
        "timestamp": "2026-03-15T12:35:00Z",
        "data": {
          "total": 8589934592,
          "used": 5368709120,
          "free": 3221225472,
          "usage": 62.5,
          "swapTotal": 4294967296,
          "swapUsed": 438937600,
          "swapUsage": 10.2
        }
      }
    }
  }
}
```

### 获取单个小组件数据

```http
GET /api/v1/dashboard/:id/widgets/:widgetId
```

---

## 小组件类型

### CPU 小组件

```json
{
  "type": "cpu",
  "config": {
    "showPerCore": true,
    "showAverage": true,
    "warningThreshold": 80,
    "criticalThreshold": 95
  }
}
```

**数据字段**
| 字段 | 类型 | 说明 |
|------|------|------|
| usage | float64 | 总 CPU 使用率 |
| perCore | []float64 | 每核心使用率 |
| loadAvg1 | float64 | 1分钟平均负载 |
| loadAvg5 | float64 | 5分钟平均负载 |
| loadAvg15 | float64 | 15分钟平均负载 |
| processCount | int | 进程数 |

### 内存小组件

```json
{
  "type": "memory",
  "config": {
    "showSwap": true,
    "showBuffers": true
  }
}
```

**数据字段**
| 字段 | 类型 | 说明 |
|------|------|------|
| total | uint64 | 总内存 |
| used | uint64 | 已用内存 |
| free | uint64 | 空闲内存 |
| usage | float64 | 使用率 |
| swapTotal | uint64 | 总交换空间 |
| swapUsed | uint64 | 已用交换空间 |
| swapUsage | float64 | 交换空间使用率 |

### 磁盘小组件

```json
{
  "type": "disk",
  "config": {
    "mountPoints": ["/", "/data"],
    "showIOStats": true
  }
}
```

**数据字段**
| 字段 | 类型 | 说明 |
|------|------|------|
| devices | []object | 磁盘设备列表 |
| total | object | 总计数据 |

### 网络小组件

```json
{
  "type": "network",
  "config": {
    "interfaces": ["eth0", "wlan0"],
    "showPackets": true,
    "showErrors": true
  }
}
```

**数据字段**
| 字段 | 类型 | 说明 |
|------|------|------|
| interfaces | []object | 网络接口列表 |
| total | object | 总计数据 |

---

## WebSocket 实时推送

### 连接端点

```
ws://localhost:8080/api/v1/dashboard/:id/stream
```

### 消息格式

```json
{
  "type": "widget_update",
  "dashboardId": "default",
  "widgetId": "cpu-1",
  "timestamp": "2026-03-15T12:35:00Z",
  "data": {...}
}
```

---

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 仪表板不存在 |
| 500 | 服务器内部错误 |