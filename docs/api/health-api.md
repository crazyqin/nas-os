# 健康检查 API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

## 概述

健康检查 API 提供系统健康状态评估、检查项配置和健康评分功能。

## 基础路径

```
/api/v1/health
```

---

## 健康检查

### 获取系统健康状态

```http
GET /api/v1/health
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "healthy",
    "score": 95,
    "timestamp": "2026-03-15T12:35:00Z",
    "checks": [
      {
        "name": "cpu",
        "status": "ok",
        "value": 35.2,
        "threshold": 90,
        "message": "CPU 使用率正常"
      },
      {
        "name": "memory",
        "status": "warning",
        "value": 85.5,
        "threshold": 80,
        "message": "内存使用率较高"
      },
      {
        "name": "disk",
        "status": "ok",
        "value": 45.0,
        "threshold": 90,
        "message": "磁盘空间充足"
      },
      {
        "name": "network",
        "status": "ok",
        "value": 0,
        "threshold": 0,
        "message": "网络连接正常"
      }
    ]
  }
}
```

### 状态值说明

| 状态 | 说明 |
|------|------|
| healthy | 系统健康 |
| warning | 存在警告 |
| critical | 系统异常 |
| unknown | 状态未知 |

### 执行健康检查

```http
POST /api/v1/health/check
Content-Type: application/json

{
  "checks": ["cpu", "memory", "disk", "network", "services"]
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "healthy",
    "score": 92,
    "checks": [...]
  }
}
```

---

## 检查项管理

### 获取检查项列表

```http
GET /api/v1/health/checks
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "checks": [
      {
        "name": "cpu",
        "description": "CPU 使用率检查",
        "enabled": true,
        "threshold": 90,
        "interval": 60
      },
      {
        "name": "memory",
        "description": "内存使用率检查",
        "enabled": true,
        "threshold": 85,
        "interval": 60
      },
      {
        "name": "disk",
        "description": "磁盘空间检查",
        "enabled": true,
        "threshold": 90,
        "interval": 300
      }
    ]
  }
}
```

### 更新检查项配置

```http
PUT /api/v1/health/checks/:name
Content-Type: application/json

{
  "enabled": true,
  "threshold": 85,
  "interval": 120
}
```

---

## 健康评分

### 获取健康评分历史

```http
GET /api/v1/health/history?period=24h
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period": "24h",
    "scores": [
      {"timestamp": "2026-03-15T00:00:00Z", "score": 95},
      {"timestamp": "2026-03-15T06:00:00Z", "score": 92},
      {"timestamp": "2026-03-15T12:00:00Z", "score": 88}
    ],
    "average": 91.7,
    "min": 85,
    "max": 98
  }
}
```

---

## 检查项详情

### CPU 检查

| 参数 | 说明 | 默认值 |
|------|------|--------|
| threshold | 告警阈值 (%) | 90 |
| interval | 检查间隔 (秒) | 60 |

**状态判定**
- ok: usage < threshold * 0.8
- warning: usage >= threshold * 0.8
- critical: usage >= threshold

### 内存检查

| 参数 | 说明 | 默认值 |
|------|------|--------|
| threshold | 告警阈值 (%) | 85 |
| interval | 检查间隔 (秒) | 60 |

### 磁盘检查

| 参数 | 说明 | 默认值 |
|------|------|--------|
| threshold | 告警阈值 (%) | 90 |
| interval | 检查间隔 (秒) | 300 |
| mountPoints | 检查的挂载点 | ["/"] |

### 网络检查

| 参数 | 说明 | 默认值 |
|------|------|--------|
| interval | 检查间隔 (秒) | 60 |
| endpoints | 检测端点 | [] |

### 服务检查

| 参数 | 说明 | 默认值 |
|------|------|--------|
| services | 检查的服务列表 | ["nasd"] |
| interval | 检查间隔 (秒) | 30 |

---

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 404 | 检查项不存在 |
| 500 | 服务器内部错误 |