# 系统健康评分

> **功能模块**: `healthscore` | **API 前缀**: `/api/v1/healthscore`

## 概述

系统健康评分引擎，通过多维度检查器对 NAS 整体状态进行量化评分（0-100），自动识别潜在风险，生成可操作的健康报告。

## 核心能力

- **多维度检查** — CPU、内存、磁盘、网络、服务状态等
- **量化评分** — 0-100 分制，直观评估系统状态
- **三级状态** — healthy（健康）/ warning（警告）/ critical（严重）
- **定期巡检** — 默认每 5 分钟自动检查

## API 接口

### 获取最新报告

```
GET /api/v1/healthscore/report
```

**响应示例**:

```json
{
  "overall_score": 87,
  "overall_status": "healthy",
  "checks": [
    {
      "name": "CPU 使用率",
      "score": 95,
      "status": "healthy",
      "message": "CPU 使用率 12%，状态良好"
    },
    {
      "name": "磁盘空间",
      "score": 72,
      "status": "warning",
      "message": "/mnt/hdd 使用率 78%，建议清理",
      "details": {
        "mount_point": "/mnt/hdd",
        "used_percent": 78,
        "free_gb": 450
      }
    },
    {
      "name": "内存使用",
      "score": 90,
      "status": "healthy",
      "message": "内存使用 62%，正常范围"
    }
  ],
  "generated_at": "2026-05-06T09:00:00Z"
}
```

### 手动触发检查

```
POST /api/v1/healthscore/check
```

立即执行一轮全量健康检查，返回最新报告。

## 评分等级

| 分数范围 | 状态 | 含义 |
|----------|------|------|
| 80-100 | ✅ healthy | 系统健康，无需干预 |
| 60-79 | ⚠️ warning | 存在风险，建议关注 |
| 0-59 | 🔴 critical | 严重问题，需要立即处理 |

## 使用场景

| 场景 | 说明 |
|------|------|
| 日常监控 | 通过仪表盘查看系统健康趋势 |
| 故障预防 | 告警触发前发现问题 |
| 运维报告 | 定期生成系统健康报告 |
| 容量规划 | 结合历史评分趋势做存储规划 |
