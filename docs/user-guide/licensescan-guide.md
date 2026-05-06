# 许可证合规扫描

> **功能模块**: `licensescan` | **API 前缀**: `/api/v1/licensescan`

## 概述

自动扫描 Docker 应用和 Go 依赖的许可证合规性，通过白名单/黑名单/灰名单策略管理合规风险，生成合规报告，满足企业级开源合规要求。

## 核心能力

- **Docker 镜像扫描** — 检查容器镜像中所有依赖的许可证
- **Go 模块扫描** — 解析 `go.mod` 扫描所有间接依赖
- **三级策略管理** — 白名单（允许）/ 黑名单（禁止）/ 灰名单（需审批）
- **合规仪表盘** — 合规率、违规分布、趋势分析
- **定时调度** — 自动定期扫描
- **告警通知** — 发现违规立即告警

## 许可证分类

| 类别 | 说明 | 示例 |
|------|------|------|
| `permissive` | 宽松许可证 | MIT、BSD、Apache 2.0 |
| `weak_copyleft` | 弱传染 | LGPL |
| `strong_copyleft` | 强传染 | GPL、AGPL |
| `custom` | 自定义许可证 | 自有协议 |
| `unknown` | 未识别 | 未知来源 |

## API 接口

### 扫描 Docker 镜像

```
POST /api/v1/licensescan/scan/docker
```

**请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `target` | string | ✅ | Docker 镜像名（如 `nginx:latest`） |
| `policy_id` | string | ❌ | 使用的合规策略 ID |

### 扫描 Go 模块

```
POST /api/v1/licensescan/scan/gomod
```

**请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `target` | string | ✅ | go.mod 文件路径 |
| `policy_id` | string | ❌ | 使用的合规策略 ID |

### 查询扫描结果

```
GET /api/v1/licensescan/scan/results       # 列表
GET /api/v1/licensescan/scan/result/:id    # 详情
```

**扫描结果示例**:

```json
{
  "id": "docker-1714950000000",
  "scan_type": "docker",
  "target": "nginx:latest",
  "status": "complete",
  "licenses": [
    {
      "name": "MIT",
      "spdx_id": "MIT",
      "category": "permissive",
      "compliance": "allowed",
      "source": "nginx:latest"
    }
  ],
  "summary": {
    "total_licenses": 15,
    "allowed": 12,
    "denied": 1,
    "review_required": 2,
    "unknown": 0
  },
  "violations": [
    {
      "license_name": "GPL-3.0",
      "source": "libfoo",
      "list_type": "blacklist",
      "severity": "high",
      "message": "GPL-3.0 在黑名单中，禁止使用"
    }
  ]
}
```

### 策略管理

```
GET    /api/v1/licensescan/policies        # 策略列表
POST   /api/v1/licensescan/policies        # 创建策略
GET    /api/v1/licensescan/policy/:id      # 策略详情
PUT    /api/v1/licensescan/policy/:id      # 更新策略
DELETE /api/v1/licensescan/policy/:id      # 删除策略
```

**策略示例**:

```json
{
  "name": "企业默认策略",
  "description": "允许宽松许可证，禁止强传染许可证",
  "whitelist": ["MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0"],
  "blacklist": ["AGPL-3.0", "SSPL-1.0"],
  "graylist": ["GPL-2.0", "GPL-3.0", "LGPL-2.1"],
  "default_list": "graylist"
}
```

### 合规仪表盘

```
GET /api/v1/licensescan/dashboard
```

**响应**:

```json
{
  "compliance_rate": 94.5,
  "total_scans": 128,
  "total_violations": 7,
  "license_breakdown": {
    "permissive": 85,
    "weak_copyleft": 12,
    "strong_copyleft": 5,
    "unknown": 2
  },
  "top_violations": [
    {
      "license_name": "AGPL-3.0",
      "count": 3,
      "severity": "critical"
    }
  ]
}
```

### 报告管理

```
GET  /api/v1/licensescan/reports           # 报告列表
POST /api/v1/licensescan/report/generate   # 生成报告
GET  /api/v1/licensescan/report/:id        # 报告详情
```

### 告警查询

```
GET /api/v1/licensescan/alerts
```

### 调度器管理

```
GET  /api/v1/licensescan/scheduler/tasks       # 调度任务列表
POST /api/v1/licensescan/scheduler/tasks       # 创建调度任务
GET  /api/v1/licensescan/scheduler/task/:id    # 任务详情
PUT  /api/v1/licensescan/scheduler/task/:id    # 更新任务
```

## 使用场景

| 场景 | 说明 |
|------|------|
| CI/CD 合规门禁 | 在构建流水线中自动扫描，阻止不合规依赖 |
| 容器安全审计 | 上线前扫描 Docker 镜像许可证合规性 |
| 开源合规报告 | 定期生成许可证合规报告，满足审计要求 |
| 供应商评估 | 扫描第三方组件的许可证风险 |

## 注意事项

- 扫描基于已知许可证数据库，未知许可证需人工确认
- Docker 扫描需要系统已安装 Docker
- 灰名单中的许可证需要管理员手动审批
- 建议在 CI/CD 流水线中集成自动扫描
