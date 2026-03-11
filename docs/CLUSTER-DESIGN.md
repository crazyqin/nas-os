# 集群支持设计文档

## 概述

为 NAS-OS 添加集群支持，实现多节点协同工作，提供高可用、负载均衡和分布式存储能力。

## 核心功能

### 1. 多节点发现和管理 (Cluster Discovery & Management)

**功能**:
- 节点自动发现 (mDNS/Bonjour 协议)
- 节点注册与注销
- 节点状态监控 (心跳机制)
- 节点角色分配 (Master/Worker)
- 集群配置管理

**实现**:
```go
type ClusterNode struct {
    ID        string    // 节点唯一标识
    Hostname  string    // 主机名
    IP        string    // IP 地址
    Port      int       // API 端口
    Role      string    // master/worker
    Status    string    // online/offline/degraded
    Heartbeat time.Time // 最后心跳时间
    Metrics   NodeMetrics // 节点指标
}

type ClusterManager struct {
    nodes     map[string]*ClusterNode
    masterID  string
    config    ClusterConfig
}
```

### 2. 分布式存储同步 (Distributed Storage Sync)

**功能**:
- 跨节点数据同步 (rsync/ssh)
- 实时文件复制 (inotify + WebSocket)
- 存储池聚合 (GlusterFS/Ceph 集成)
- 数据一致性校验
- 冲突解决策略

**实现**:
```go
type StorageSync struct {
    syncRules   []SyncRule
    syncQueue   chan SyncJob
    status      SyncStatus
}

type SyncRule struct {
    SourceNode   string
    TargetNodes  []string
    SourcePath   string
    TargetPath   string
    SyncMode     string // async/sync/realtime
    Schedule     string // cron 表达式
}
```

### 3. 负载均衡配置 (Load Balancing)

**功能**:
- 请求分发 (轮询/最少连接/加权)
- 服务健康检查
- 会话保持
- 动态权重调整
- SSL 终止

**实现**:
```go
type LoadBalancer struct {
    algorithm string // round-robin/least-conn/weighted
    backends  []Backend
    config    LBConfig
}

type Backend struct {
    NodeID   string
    Address  string
    Weight   int
    Active   bool
    Connections int
}
```

### 4. 高可用故障转移 (High Availability Failover)

**功能**:
- 主节点选举 (Raft 算法)
- 故障检测 (心跳超时)
- 自动故障转移
- 服务恢复
- 脑裂防护

**实现**:
```go
type HighAvailability struct {
    raftNode  *raft.Node
    state     string // follower/candidate/leader
    term      uint64
    votedFor  string
}
```

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Cluster Layer                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   Master    │  │   Worker 1  │  │   Worker 2  │         │
│  │   Node      │  │   Node      │  │   Node      │         │
│  │             │  │             │  │             │         │
│  │ - Raft      │  │ - Storage   │  │ - Storage   │         │
│  │ - LB        │  │ - Sync      │  │ - Sync      │         │
│  │ - Discovery │  │ - Health    │  │ - Health    │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          │                                  │
│              ┌───────────┴───────────┐                      │
│              │   Distributed Store   │                      │
│              │   (GlusterFS/Ceph)    │                      │
│              └───────────────────────┘                      │
└─────────────────────────────────────────────────────────────┘
```

## API 设计

### 节点管理 API
```
GET    /api/v1/cluster/nodes          - 获取节点列表
POST   /api/v1/cluster/nodes/join     - 加入集群
DELETE /api/v1/cluster/nodes/:id      - 移除节点
GET    /api/v1/cluster/nodes/:id/status - 节点状态
POST   /api/v1/cluster/nodes/:id/drain - 节点下线
```

### 存储同步 API
```
GET    /api/v1/cluster/sync/rules     - 获取同步规则
POST   /api/v1/cluster/sync/rules     - 创建同步规则
DELETE /api/v1/cluster/sync/rules/:id - 删除同步规则
POST   /api/v1/cluster/sync/trigger   - 手动触发同步
GET    /api/v1/cluster/sync/status    - 同步状态
```

### 负载均衡 API
```
GET    /api/v1/cluster/lb/config      - 获取 LB 配置
PUT    /api/v1/cluster/lb/config      - 更新 LB 配置
GET    /api/v1/cluster/lb/stats       - LB 统计
POST   /api/v1/cluster/lb/reset       - 重置统计
```

### 高可用 API
```
GET    /api/v1/cluster/ha/status      - HA 状态
POST   /api/v1/cluster/ha/failover    - 手动故障转移
GET    /api/v1/cluster/ha/history     - 故障转移历史
```

## 依赖

- **Raft 共识**: github.com/hashicorp/raft
- **mDNS 发现**: github.com/grandcat/zeroconf
- **分布式存储**: GlusterFS/Ceph (可选)
- **数据同步**: rsync + inotify

## 里程碑

### M7: 集群基础 (2026-03-20)
- [ ] 节点发现与注册
- [ ] 心跳机制
- [ ] 基础集群管理 API

### M8: 存储同步 (2026-04-10)
- [ ] 同步规则管理
- [ ] 定时同步任务
- [ ] 同步状态监控

### M9: 负载均衡 (2026-04-25)
- [ ] 负载均衡器实现
- [ ] 健康检查
- [ ] 动态权重

### M10: 高可用 (2026-05-15)
- [ ] Raft 共识实现
- [ ] 故障检测与转移
- [ ] 脑裂防护

## 测试策略

- 单元测试：各模块独立测试
- 集成测试：多节点集群测试
- 压力测试：模拟节点故障
- 混沌工程：随机故障注入

---

*创建日期：2026-03-11*
