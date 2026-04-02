# 多系统管理部署指南 (CMS)

**版本**: v2.372.0 | **更新**: 2026-04-01 | **对标**: 群晖CMS/飞牛FN Connect

---

## 一、功能概述

### 1.1 什么是CMS

CMS（Central Management System）多系统集中管理平台，是 NAS-OS 的多节点管理解决方案。通过CMS，管理员可统一管理多台NAS设备，实现集中监控、批量操作、资源共享。

**对标竞品**:
- 群晖 CMS (Central Management System)
- 飞牛 FN Connect
- TrueNAS Connect

### 1.2 适用场景

| 场景 | 说明 |
|------|------|
| **企业多站点管理** | 管理分布在不同地点的NAS设备 |
| **数据中心运维** | 统一监控大量NAS节点状态 |
| **家庭多设备管理** | 统一管理家中多台NAS |
| **IT服务商运维** | 为客户提供集中运维服务 |

### 1.3 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      主控节点 (Master)                       │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐             │
│  │  CMS API   │  │  Dashboard │  │  Task Queue │             │
│  │  认证服务   │  │  监控面板   │  │  任务调度   │             │
│  └────────────┘  └────────────┘  └────────────┘             │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐             │
│  │  告警中心   │  │  数据聚合   │  │  配置管理   │             │
│  └────────────┘  └────────────┘  └────────────┘             │
└─────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  子节点 A   │  │  子节点 B   │  │  子节点 C   │  │  子节点 D   │
│  NAS-OS     │  │  NAS-OS     │  │  NAS-OS     │  │  NAS-OS     │
│  Agent服务  │  │  Agent服务  │  │  Agent服务  │  │  Agent服务  │
└─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘
```

### 1.4 核心功能

| 功能类别 | 功能详情 |
|----------|----------|
| **节点管理** | 注册、发现、监控、离线告警 |
| **统一仪表板** | 状态概览、容量汇总、性能聚合 |
| **批量操作** | 共享创建、用户同步、配置推送 |
| **任务调度** | 跨节点复制、备份任务、定时作业 |
| **告警管理** | 集中告警、多渠道通知、告警历史 |
| **权限管理** | SSO认证、角色授权、审计日志 |

---

## 二、部署指南

### 2.1 部署规划

#### 部署模式选择

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **单主控模式** | 一台主控节点管理所有子节点 | 小规模部署（<20节点） |
| **层级模式** | 主控管理区域控制器，区域管理子节点 | 大规模部署（>50节点） |
| **云端模式** | 云端主控管理边缘节点 | 分布式部署（规划中） |

#### 硬件要求

| 节点类型 | CPU | 内存 | 存储 |
|----------|-----|------|------|
| 主控节点 | 4核+ | 8GB+ | 100GB+ |
| 子节点 | 2核+ | 4GB+ | 按需配置 |

#### 网络要求

- 主控节点与子节点网络互通
- 建议同一局域网或VPN连接
- 端口开放：8080（API）、8443（HTTPS）

### 2.2 主控节点配置

#### 安装CMS组件

```bash
# 安装CMS主控服务
nasctl cms install --role master

# 验证安装
nasctl cms status
```

#### 配置文件

**配置路径**: `/etc/nas-os/cms.yaml`

```yaml
cms:
  # 服务配置
  enabled: true
  role: master
  listen: 0.0.0.0:8080
  https_port: 8443
  
  # 认证配置
  auth:
    type: jwt              # JWT认证
    secret: your-secret-key-change-this
    token_expire: 24h
    refresh_token: 7d
    
  # SSL证书
  ssl:
    enabled: true
    cert: /etc/nas-os/ssl/cms.crt
    key: /etc/nas-os/ssl/cms.key
    
  # 节点管理
  nodes:
    max: 100                # 最大节点数
    discovery: auto         # 自动发现
    heartbeat: 30s          # 心跳间隔
    timeout: 5m             # 超时判定
    
  # 任务配置
  tasks:
    queue_size: 1000
    retry_times: 3
    timeout: 30m
    
  # 告警配置
  alerts:
    channels:
      - email
      - webhook
    email:
      smtp: smtp.example.com
      from: cms@example.com
      to: admin@example.com
    webhook:
      url: https://hooks.example.com/cms
```

#### 启动服务

```bash
# 启动CMS主控服务
nasctl cms start

# 设置开机自启
nasctl cms enable

# 检查服务状态
nasctl cms status
```

### 2.3 子节点配置

#### 配置Agent服务

**配置文件**: `/etc/nas-os/cms.yaml`

```yaml
cms:
  # 服务配置
  enabled: true
  role: agent              # 子节点角色
  
  # 主控节点配置
  master:
    address: 192.168.1.100:8080
    token: registration-token-from-master
    
  # Agent配置
  agent:
    heartbeat: 30s         # 心跳上报间隔
    metrics_interval: 1m   # 指标上报间隔
    report_interval: 5m    # 状态上报间隔
    
  # 本地服务
  local:
    listen: 127.0.0.1:8090  # 本地Agent端口
```

#### 注册到主控节点

**方式一：自动发现**

子节点启动后自动扫描局域网，发现主控节点并注册。

```bash
# 启动Agent服务
nasctl cms start

# Agent自动发现主控并注册
```

**方式二：手动注册**

1. 在主控节点生成注册Token：
```bash
nasctl cms token create --name node-1
# 输出: registration-token-xxxx
```

2. 在子节点使用Token注册：
```bash
nasctl cms register --master 192.168.1.100:8080 --token xxx
```

3. 在主控节点确认注册：
```bash
nasctl cms nodes list
```

### 2.4 节点发现机制

#### 自动发现流程

```
┌──────────────────────────────────────────────────────────┐
│                    自动发现流程                           │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  1. 主控节点启动 → 广播发现信号                          │
│     [UDP Broadcast] → 239.255.255.250:37020             │
│                                                          │
│  2. 子节点启动 → 监听发现信号                            │
│     [UDP Listen] → 239.255.255.250:37020                │
│                                                          │
│  3. 子节点收到信号 → 响应注册请求                        │
│     [HTTP POST] → /api/cms/register                     │
│                                                          │
│  4. 主控节点验证 → 生成认证Token                         │
│     [JWT Token] → 返回给子节点                           │
│                                                          │
│  5. 子节点保存Token → 开始心跳上报                       │
│     [HTTP POST] → /api/cms/heartbeat                    │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

#### 手动注册流程

```
主控节点                          子节点
    │                               │
    │ 1. 生成注册Token              │
    │                               │
    │ 2. 提供Token给管理员          │
    │                               │
    │                 3. 配置Token  │
    │                               │
    │ 4. 接收注册请求 ←─────────────│
    │                               │
    │ 5. 验证并生成JWT              │
    │                               │
    │ 6. 返回JWT ─────────────────→│
    │                               │
    │ 7. 开始心跳 ←─────────────────│
    │                               │
```

---

## 三、集群配置指南

### 3.1 节点分组管理

#### 创建节点组

**场景**: 按业务/区域/功能分组管理节点

```bash
# 创建节点组
nasctl cms group create --name "北京办公区" --desc "北京办公室NAS集群"

# 添加节点到组
nasctl cms group add --group "北京办公区" --nodes node-1,node-2,node-3

# 查看组信息
nasctl cms group list
```

**配置文件方式**:

```yaml
cms:
  groups:
    - name: "北京办公区"
      description: "北京办公室NAS集群"
      nodes: ["node-1", "node-2", "node-3"]
      
    - name: "上海办公区"
      description: "上海办公室NAS集群"
      nodes: ["node-4", "node-5"]
      
    - name: "备份集群"
      description: "备份专用节点"
      nodes: ["backup-1", "backup-2"]
```

### 3.2 节点角色配置

| 角色 | 权限 | 说明 |
|------|------|------|
| **存储节点** | 提供存储服务 | 主存储节点 |
| **备份节点** | 接收备份数据 | 备份专用节点 |
| **边缘节点** | 边缘缓存服务 | CDN边缘节点 |
| **管理节点** | 辅助管理 | 层级管理模式下的区域管理节点 |

**配置角色**:

```bash
# 设置节点角色
nasctl cms node role --node node-1 --role storage
nasctl cms node role --node backup-1 --role backup
```

### 3.3 跨节点资源共享

#### 资源池配置

```yaml
cms:
  pools:
    - name: "主存储池"
      nodes: ["node-1", "node-2"]
      shares:
        - name: "公司文档"
          path: /data/documents
          nodes: ["node-1", "node-2"]
          
    - name: "备份池"
      nodes: ["backup-1", "backup-2"]
      shares:
        - name: "备份存储"
          path: /backup
          nodes: ["backup-1", "backup-2"]
```

#### 资源调度策略

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| **轮询** | 依次分配到各节点 | 均衡分布 |
| **容量优先** | 优先分配到容量充裕节点 | 避免单节点过载 |
| **就近分配** | 分配到网络最近节点 | 优化访问速度 |
| **固定分配** | 指定节点分配 | 特定需求 |

---

## 四、多节点管理手册

### 4.1 统一仪表板

#### 状态概览

**仪表板布局**:

```
┌─────────────────────────────────────────────────────────────┐
│  CMS 管理仪表板                          [节点: 12] [告警: 2]│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────节点状态───────────────┐                    │
│  │ 在线 ████████████████░ 10/12      │                    │
│  │ 离线 ████░░░░░░░░░░░░░░░░ 2/12    │                    │
│  │ 总容量 12.5TB │ 已用 8.2TB │ 65% │                    │
│  └────────────────────────────────────┘                    │
│                                                             │
│  ┌─────────────节点列表───────────────┐                    │
│  │ node-1  ████████████  在线  北京  │                    │
│  │ node-2  ████████████  在线  北京  │                    │
│  │ node-3  ████████░░░░  告警  上海  │                    │
│  │ node-4  ░░░░░░░░░░░░  离线  上海  │                    │
│  │ ...                               │                    │
│  └────────────────────────────────────┘                    │
│                                                             │
│  ┌─────────────告警列表───────────────┐                    │
│  │ 🔴 node-3 存储空间不足 (85%)      │                    │
│  │ 🟡 node-4 心跳超时离线            │                    │
│  └────────────────────────────────────┘                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 性能监控

| 监控指标 | 说明 |
|----------|------|
| **CPU使用率** | 各节点CPU使用情况 |
| **内存使用率** | 各节点内存使用情况 |
| **网络带宽** | 各节点网络吞吐量 |
| **存储IOPS** | 各节点存储读写速度 |
| **容量趋势** | 存储容量变化趋势图 |

### 4.2 批量操作

#### 共享批量创建

```bash
# 在多个节点创建相同共享
nasctl cms batch share-create \
  --nodes node-1,node-2,node-3 \
  --name "项目文档" \
  --path /data/project \
  --permissions "rw"

# 通过配置文件批量创建
nasctl cms batch share-create --config shares.yaml
```

**配置文件示例**:

```yaml
batch:
  operation: share_create
  nodes: ["node-1", "node-2", "node-3"]
  shares:
    - name: "项目文档"
      path: /data/project
      permissions: rw
      
    - name: "财务数据"
      path: /data/financial
      permissions: r
      users: ["finance-team"]
```

#### 用户批量同步

```bash
# 同步用户到多个节点
nasctl cms batch user-sync \
  --nodes node-1,node-2 \
  --users user1,user2,user3

# 同步用户组
nasctl cms batch group-sync \
  --nodes node-1,node-2,node-3 \
  --groups finance-team,dev-team
```

#### 配置批量推送

```bash
# 推送配置模板
nasctl cms batch config-push \
  --nodes node-1,node-2 \
  --template /etc/nas-os/templates/smb.yaml

# 推送安全策略
nasctl cms batch security-push \
  --nodes all \
  --policy firewall-rules.yaml
```

### 4.3 节点管理操作

#### 添加节点

```bash
# WebUI方式
# 「CMS管理」→「节点管理」→「添加节点」

# CLI方式
nasctl cms node add --name node-new --address 192.168.1.150
```

#### 移除节点

```bash
# 移除节点（需确认）
nasctl cms node remove --node node-4 --confirm

# 强制移除离线节点
nasctl cms node remove --node node-4 --force
```

#### 节点状态检查

```bash
# 查看单个节点详情
nasctl cms node status --node node-1

# 查看所有节点状态
nasctl cms nodes status

# 检查节点健康状态
nasctl cms nodes health-check
```

### 4.4 跨节点任务

#### 数据复制任务

```bash
# 创建跨节点复制任务
nasctl cms task create \
  --type replication \
  --source node-1:/data/documents \
  --target node-2:/backup/documents \
  --schedule daily \
  --time 02:00

# 立即执行复制
nasctl cms task run --task replication-1

# 查看任务状态
nasctl cms task status --task replication-1
```

#### 备份任务

```bash
# 创建跨节点备份任务
nasctl cms task create \
  --type backup \
  --source node-1:/data \
  --target backup-1:/incoming \
  --schedule weekly \
  --retention 30d

# 配置增量备份
nasctl cms task create \
  --type incremental-backup \
  --source node-1:/data \
  --target backup-1:/incremental \
  --schedule daily
```

#### 定时任务管理

```bash
# 查看所有定时任务
nasctl cms tasks list

# 暂停任务
nasctl cms task pause --task replication-1

# 恢复任务
nasctl cms task resume --task replication-1

# 删除任务
nasctl cms task delete --task replication-1
```

### 4.5 告警管理

#### 告警配置

```yaml
cms:
  alerts:
    # 告警规则
    rules:
      - name: "节点离线"
        condition: "heartbeat_timeout"
        severity: high
        channels: [email, webhook]
        
      - name: "存储容量告警"
        condition: "storage_usage > 80%"
        severity: medium
        channels: [email]
        
      - name: "性能异常"
        condition: "cpu_usage > 90%"
        severity: low
        channels: [webhook]
        
    # 告警渠道
    channels:
      email:
        smtp: smtp.example.com:587
        from: cms@example.com
        to: admin@example.com
        
      webhook:
        url: https://hooks.example.com/cms/alert
        method: POST
        
      push:
        enabled: true
```

#### 告警历史查看

```bash
# 查看最近告警
nasctl cms alerts list --limit 20

# 查看特定节点告警
nasctl cms alerts list --node node-3

# 清除已处理告警
nasctl cms alerts clear --alert alert-123
```

---

## 五、API 接口文档

### 5.1 节点管理 API

#### 节点注册

```bash
POST /api/cms/register
{
  "name": "nas-node-1",
  "address": "192.168.1.101",
  "token": "registration-token",
  "metadata": {
    "location": "北京",
    "role": "storage"
  }
}

Response:
{
  "success": true,
  "node_id": "node-1",
  "jwt_token": "eyJhbGc...",
  "heartbeat_url": "/api/cms/heartbeat"
}
```

#### 节点列表

```bash
GET /api/cms/nodes

Response:
{
  "nodes": [
    {
      "id": "node-1",
      "name": "nas-node-1",
      "address": "192.168.1.101",
      "status": "online",
      "last_heartbeat": "2026-04-01T10:30:00Z",
      "storage": {
        "total": 2TB,
        "used": 1.2TB,
        "available": 800GB
      },
      "metrics": {
        "cpu": 25,
        "memory": 60,
        "network_in": 50MB/s,
        "network_out": 30MB/s
      }
    }
  ],
  "total": 10,
  "online": 8,
  "offline": 2
}
```

#### 节点详情

```bash
GET /api/cms/nodes/{node_id}

Response:
{
  "id": "node-1",
  "name": "nas-node-1",
  "address": "192.168.1.101",
  "status": "online",
  "uptime": "30d",
  "version": "v2.372.0",
  "shares": [
    {"name": "documents", "path": "/data/documents"},
    {"name": "photos", "path": "/data/photos"}
  ],
  "storage": {...},
  "metrics": {...}
}
```

### 5.2 批量操作 API

```bash
POST /api/cms/batch
{
  "operation": "create_share",
  "nodes": ["node-1", "node-2", "node-3"],
  "params": {
    "name": "shared-docs",
    "path": "/data/docs",
    "permissions": "rw",
    "users": ["team-dev"]
  }
}

Response:
{
  "success": true,
  "task_id": "batch-123",
  "status": {
    "node-1": "completed",
    "node-2": "completed",
    "node-3": "failed"
  },
  "errors": [
    {"node": "node-3", "error": "path already exists"}
  ]
}
```

### 5.3 任务管理 API

```bash
# 创建任务
POST /api/cms/task
{
  "type": "replication",
  "source": "node-1:/data/backup",
  "target": "node-2:/backup/incoming",
  "schedule": "daily",
  "time": "02:00"
}

# 查看任务状态
GET /api/cms/task/{task_id}

# 执行任务
POST /api/cms/task/{task_id}/run

# 暂停任务
POST /api/cms/task/{task_id}/pause
```

### 5.4 心跳上报 API

```bash
POST /api/cms/heartbeat
Authorization: Bearer <jwt_token>
{
  "node_id": "node-1",
  "timestamp": "2026-04-01T10:30:00Z",
  "status": {
    "cpu": 25,
    "memory": 60,
    "disk_io": 100,
    "network": {
      "in": 50MB/s,
      "out": 30MB/s
    }
  },
  "storage": {
    "total": 2TB,
    "used": 1.2TB
  }
}

Response:
{
  "success": true,
  "next_heartbeat": 30s
}
```

---

## 六、权限与认证

### 6.1 SSO认证配置

```yaml
cms:
  auth:
    sso:
      enabled: true
      provider: ldap          # LDAP/AD/OAuth
      
      ldap:
        server: ldap.example.com
        port: 389
        base_dn: dc=example,dc=com
        bind_dn: cn=admin,dc=example,dc=com
        bind_password: secret
        
      oauth:
        provider: custom
        auth_url: https://auth.example.com/oauth/authorize
        token_url: https://auth.example.com/oauth/token
        client_id: cms-client
        client_secret: secret
```

### 6.2 角色权限配置

| 角色 | 权限范围 |
|------|----------|
| **管理员** | 所有节点、所有操作 |
| **运维员** | 监控查看、告警处理 |
| **操作员** | 执行批量操作 |
| **审计员** | 仅查看日志和报告 |

```yaml
cms:
  roles:
    admin:
      permissions: [all]
      
    operator:
      permissions:
        - nodes:view
        - tasks:execute
        - batch:execute
        
    auditor:
      permissions:
        - logs:view
        - alerts:view
        - reports:view
```

### 6.3 审计日志

```bash
# 查看操作日志
nasctl cms logs list --limit 100

# 查看特定节点日志
nasctl cms logs list --node node-1

# 查看特定操作日志
nasctl cms logs list --operation batch-create

# 导出审计日志
nasctl cms logs export --format csv --output audit.csv
```

---

## 七、最佳实践

### 7.1 部署建议

| 规模 | 部署模式 | 主控节点配置 |
|------|----------|--------------|
| <10节点 | 单主控 | 4核+8GB |
| 10-50节点 | 单主控 | 8核+16GB |
| >50节点 | 层级模式 | 多区域管理节点 |

### 7.2 安全建议

1. 启用HTTPS加密通信
2. 定期更新认证Token
3. 配置IP白名单限制访问
4. 启用操作审计日志
5. 配置告警及时响应

### 7.3 性能优化

| 配置项 | 建议值 | 说明 |
|----------|--------|------|
| heartbeat | 30s | 心跳间隔不宜过短 |
| metrics_interval | 1m | 指标上报频率 |
| queue_size | 1000 | 任务队列大小 |
| workers | 4-8 | API服务进程数 |

---

## 八、常见问题

### Q1: 节点注册失败？

**A**: 可能原因：
1. Token无效 → 重新生成注册Token
2. 网络不通 → 检查防火墙和端口
3. 主控节点未启动 → 启动CMS服务
4. 地址错误 → 确认主控节点地址

### Q2: 节点频繁离线？

**A**: 检查：
1. 网络稳定性
2. 心跳超时配置是否过短
3. 子节点负载过高
4. 主控节点负载过高

### Q3: 批量操作部分失败？

**A**: 排查步骤：
1. 查看任务错误详情
2. 检查失败节点状态
3. 确认节点权限配置
4. 手动重试失败操作

---

## 九、与竞品对比

| 功能 | NAS-OS | 群晖CMS | 飞牛FN Connect | TrueNAS Connect |
|------|:------:|:-------:|:---------------:|:---------------:|
| 多节点管理 | ✅ | ✅ | ✅ | ✅ |
| 统一仪表板 | ✅ | ✅ | ✅ | ✅ |
| 批量操作 | ✅ | ✅ | ❌ | ✅ |
| 云端管理 | 📋 规划 | ❌ | ✅ | ✅ |
| 自动发现 | ✅ | ✅ | ✅ | ✅ |
| SSO认证 | ✅ | ✅ | ✅ | ✅ |
| 跨节点复制 | ✅ | ✅ | ❌ | ✅ |
| 告警管理 | ✅ | ✅ | ⚠️ | ✅ |
| 层级管理 | 📋 规划 | ✅ | ❌ | ✅ |
| 中文原生 | ✅ | ⚠️ 翻译 | ✅ | ❌ |

---

## 十、相关文档

| 文档 | 路径 | 说明 |
|------|------|------|
| 多系统架构设计 | `docs/MULTI_SYSTEM_GUIDE.md` | 架构详细说明 |
| API完整参考 | `docs/API_GUIDE.md` | API接口文档 |
| 任务调度设计 | `docs/MULTI_SYSTEM_API.md` | 任务调度机制 |

---

**文档维护**: 礼部 | **版本**: v2.372.0 | **更新**: 2026-04-01