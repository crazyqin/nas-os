# 多系统管理部署指南

## 功能概述

多系统集中管理（CMS）是 NAS-OS 的多节点管理平台，对标群晖 CMS 和飞牛 FN Connect。

通过 CMS，管理员可以统一管理多台 NAS 设备，实现集中监控、批量操作、资源共享。

## 系统架构

```
┌─────────────────────────────────────────────────┐
│                  主控节点                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐         │
│  │ CMS API │  │Dashboard│  │ 任务调度 │         │
│  └─────────┘  └─────────┘  └─────────┘         │
└─────────────────────────────────────────────────┘
         │           │           │
         ▼           ▼           ▼
┌─────────┐    ┌─────────┐    ┌─────────┐
│ 子节点A │    │ 子节点B │    │ 子节点C │
│ NAS-OS  │    │ NAS-OS  │    │ NAS-OS  │
└─────────┘    └─────────┘    └─────────┘
```

## 部署步骤

### 1. 主控节点配置

```yaml
# config/cms.yaml
cms:
  enabled: true
  role: master
  listen: 0.0.0.0:8080
  auth:
    type: jwt
    secret: your-secret-key
  nodes:
    max: 100
    discovery: auto  # 自动发现或手动注册
```

### 2. 子节点注册

子节点向主控节点注册：

```yaml
# config/cms.yaml (子节点)
cms:
  enabled: true
  role: slave
  master:
    address: 192.168.1.100:8080
    token: registration-token
```

### 3. 节点发现机制

**自动发现** (推荐):
- 子节点启动时自动扫描局域网
- 主控节点广播发现信号
- 子节点响应并注册

**手动注册**:
- 在主控节点 WebUI 添加子节点
- 输入子节点 IP 和认证 Token
- 子节点确认注册请求

### 4. 节点认证

使用 JWT Token 进行节点间认证：
- 注册时主控节点生成 Token
- Token 有效期可配置
- 支持 Token 刷新机制

## 管理功能

### 统一仪表板

- 所有节点状态概览
- 存储容量汇总
- 性能指标聚合
- 告警集中显示

### 批量操作

| 操作 | 说明 |
|------|------|
| 共享创建 | 批量在多节点创建共享 |
| 用户同步 | 同步用户到指定节点 |
| 配置推送 | 推送配置模板到节点 |
| 任务下发 | 创建跨节点任务 |

### 节点管理

- 添加/移除节点
- 节点状态监控
- 节点离线告警
- 节点版本查看

## API 接口

### 节点注册
```
POST /api/cms/register
{
  "name": "nas-node-1",
  "address": "192.168.1.101",
  "token": "registration-token"
}
```

### 节点列表
```
GET /api/cms/nodes
```

### 批量操作
```
POST /api/cms/batch
{
  "operation": "create_share",
  "nodes": ["node-1", "node-2"],
  "params": {
    "name": "shared-docs",
    "path": "/data/docs"
  }
}
```

### 跨节点任务
```
POST /api/cms/task
{
  "type": "replication",
  "source": "node-1:/data/backup",
  "target": "node-2:/backup/incoming",
  "schedule": "daily"
}
```

## 与竞品对比

| 功能 | nas-os | 群晖 CMS | 飞牛 FN Connect | TrueNAS Connect |
|------|:------:|:--------:|:---------------:|:---------------:|
| 多节点管理 | ✅ | ✅ | ✅ | ✅ |
| 统一仪表板 | ✅ | ✅ | ✅ | ✅ |
| 批量操作 | ✅ | ✅ | ❌ | ✅ |
| 云端管理 | 规划中 | ❌ | ✅ | ✅ |
| 节点发现 | ✅ 自动 | ✅ | ✅ | ✅ |
| SSO认证 | ✅ | ✅ | ✅ | ✅ |

---

*文档版本: v2.372.0*
*更新日期: 2026-04-01*