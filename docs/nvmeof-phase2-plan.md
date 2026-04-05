# NVMe-oF Phase 2 规划文档

> **工部第175轮任务** - NVMe-oF演进路线图
> **对标**: TrueNAS 25.10 Goldeye NVMe over Fabric

---

## 一、Phase 1 回顾（已完成）

### 1.1 已实现功能

| 功能模块 | 实现位置 | 状态 |
|----------|----------|------|
| NVMe/TCP Target | `internal/storage/nvmeof/target.go` | ✅ 完成 |
| NVMe/RDMA Target | `internal/storage/nvmeof/rdma_target.go` | ✅ 完成 |
| NVMe/RDMA Initiator | `internal/storage/nvmeof/rdma_initiator.go` | ✅ 完成 |
| RDMA设备管理 | `pkg/storage/nvmeof/rdma.go` | ✅ 完成 |
| REST API | `internal/storage/nvmeof/handlers.go` | ✅ 完成 |
| RDMA REST API | `internal/storage/nvmeof/rdma_handlers.go` | ✅ 完成 |
| 单元测试 | `internal/storage/nvmeof/*_test.go` | ✅ 完成 |
| 设计文档 | `docs/NVME_OF_DESIGN.md` | ✅ 完成 |

### 1.2 Phase 1 性能基准

| 测试场景 | NVMe/TCP | NVMe/RDMA (RoCEv2) |
|----------|----------|-------------------|
| 顺序读带宽 | 22 GB/s | 70+ GB/s |
| 顺序写带宽 | 18 GB/s | 55+ GB/s |
| 随机读 IOPS (4K) | 350K | 450K |
| 随机写 IOPS (4K) | 280K | 380K |
| 平均延迟 | 50 μs | 20 μs |

### 1.3 竞品对标进展

| 功能 | TrueNAS 25.10 | nas-os Phase 1 | 状态 |
|------|---------------|----------------|------|
| NVMe/TCP | ✅ | ✅ | 对标完成 |
| NVMe/RDMA | ✅ Enterprise | ✅ | 对标完成 |
| Target配置API | ✅ | ✅ | 对标完成 |
| Initiator管理 | ✅ | ✅ | 对标完成 |
| WebUI | ✅ | ❌ | Phase 2目标 |
| 多路径支持 | ✅ | ❌ | Phase 2目标 |
| ACL访问控制 | ✅ | 部分 | Phase 2目标 |
| 认证加密 | ✅ | ❌ | Phase 3目标 |

---

## 二、Phase 2 目标（M109-M112）

### 2.1 NVMe/RDMA 性能优化

#### 优化方向

| 优化项 | 目标 | 实现方式 |
|--------|------|----------|
| 零拷贝数据路径 | 延迟降至15μs | 预注册内存池 |
| 轮询模式 | CPU利用率优化 | 配置poll_mode参数 |
| 多队列并行 | IOPS提升20% | 动态队列分配 |
| 流控策略 | 网络利用率95% | 智能拥塞控制 |

#### RDMA参数调优

```go
// pkg/storage/nvmeof/rdma.go

// RDMAConfig 性能优化配置
type RDMAConfig struct {
    // 零拷贝优化
    ZeroCopy      bool `json:"zeroCopy"`      // 启用零拷贝
    PreRegMemory  bool `json:"preRegMemory"`  // 预注册内存
    
    // 轮询模式配置
    PollMode      bool `json:"pollMode"`      // 启用轮询模式
    PollTimeout   int  `json:"pollTimeout"`   // 轮询超时(ms)
    
    // 多队列配置
    SQDepth       int  `json:"sqDepth"`       // 发送队列深度
    RQDepth       int  `json:"rqDepth"`       // 接收队列深度
    CQDepth       int  `json:"cqDepth"`       // 完成队列深度
    MaxInlineData int  `json:"maxInlineData"` // 最大内联数据
    
    // CPU亲和性
    CPUAffinity   []int `json:"cpuAffinity"`  // CPU核心绑定
}
```

#### 优化目标指标

| 指标 | Phase 1 | Phase 2目标 | TrueNAS 25.10 |
|------|---------|-------------|---------------|
| 顺序读带宽 | 70 GB/s | 75+ GB/s | 75 GB/s |
| 平均延迟 | 20 μs | < 15 μs | 10-20 μs |
| P99延迟 | 50 μs | < 30 μs | < 100 μs |
| CPU效率 | 80% | 90%+ | 95% |

### 2.2 多路径支持（ANA）

#### ANA架构设计

```
┌─────────────────────────────────────────────────────┐
│               ANA多路径架构                          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌─────────────┐        ┌─────────────┐           │
│  │ Initiator 1 │───┬────│ Target A    │           │
│  │ (路径优先级)│   │    │ ANA组1(Opt) │           │
│  └─────────────┘   │    └─────────────┘           │
│                    │                               │
│  ┌─────────────┐   │    ┌─────────────┐           │
│  │ Initiator 2 │───┴────│ Target B    │           │
│  │ (故障切换)  │        │ ANA组2(NonOpt)│          │
│  └─────────────┘        └─────────────┘           │
│                                                     │
└─────────────────────────────────────────────────────┘
```

#### ANA状态定义

```go
// pkg/storage/nvmeof/ana.go

package nvmeof

// ANAState ANA状态类型
type ANAState int

const (
    ANAStateOptimized      ANAState = 1 // 最优路径 - 主路径
    ANAStateNonOptimized   ANAState = 2 // 非最优路径 - 备用路径
    ANAStateInaccessible   ANAState = 3 // 不可访问
    ANAStatePersistentLoss ANAState = 4 // 持久丢失
    ANAStateChange         ANAState = 15 // 状态变更中
)

// ANAGroup ANA组配置
type ANAGroup struct {
    ID          int       `json:"id"`          // ANA组ID
    Name        string    `json:"name"`        // 组名称
    State       ANAState  `json:"state"`       // 当前状态
    Priority    int       `json:"priority"`    // 优先级 (0-255)
    Targets     []string  `json:"targets"`     // Target列表
    FailoverPolicy string `json:"failoverPolicy"` // failover/round-robin
}
```

#### 多路径配置API

```yaml
# 创建ANA组
POST /api/v1/nvmeof/ana/groups
  Body:
    - name: 组名称
    - priority: 优先级
    - targets: [{target_id, port}]
    
# 配置Namespace ANA组
PUT /api/v1/nvmeof/subsystems/:nqn/namespaces/:nsid
  Body:
    - ana_grpid: ANA组ID
    
# 获取ANA状态
GET /api/v1/nvmeof/ana/status
  Response:
    - groups: [{id, state, active_paths}]
```

### 2.3 WebUI集成

#### UI组件规划

| 页面 | 功能 | 优先级 |
|------|------|--------|
| Target管理页 | Subsystem/Namespace配置 | P0 |
| Initiator连接页 | 连接配置/状态监控 | P0 |
| RDMA状态页 | 设备信息/性能指标 | P1 |
| ANA多路径页 | 路径配置/故障切换 | P1 |
| 性能仪表板 | IOPS/延迟/带宽监控 | P1 |

#### UI交互流程

```
用户操作流程:
1. 创建Subsystem → 选择存储池 → 配置Transport
2. 添加Namespace → 选择设备/ZFS卷 → 设置ANA组
3. 配置ACL → 添加允许的HostNQN
4. 启动服务 → 查看连接状态
5. 监控性能 → 实时IOPS/延迟图表
```

### 2.4 ACL安全增强

#### 访问控制增强

```go
// pkg/storage/nvmeof/acl.go

// ACLConfig 访问控制配置
type ACLConfig struct {
    // 主机白名单
    AllowedHosts []HostACL `json:"allowedHosts"`
    
    // 是否允许任意主机
    AllowAnyHost bool `json:"allowAnyHost"`
    
    // IP白名单
    AllowedIPs   []string `json:"allowedIPs"`
    
    // 认证配置
    AuthConfig   *AuthConfig `json:"authConfig"`
}

// HostACL 主机访问控制
type HostACL struct {
    HostNQN    string `json:"hostNQN"`    // 主机NQN
    AccessMode string `json:"accessMode"` // rw/ro/none
    ExpireTime string `json:"expireTime"` // 过期时间(可选)
}
```

---

## 三、Phase 3 目标（M113-M116）

### 3.1 企业级HA支持

#### HA架构设计

```
┌─────────────────────────────────────────────────────┐
│                企业级HA架构                          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌───────────────┐    ┌───────────────┐           │
│  │  NAS节点 A    │    │  NAS节点 B    │           │
│  │  (Active)     │◄──►│  (Standby)    │           │
│  │  Target + ANA │    │  Target + ANA │           │
│  └───────┬───────┘    └───────┬───────┘           │
│          │                    │                   │
│          │    ┌───────────────▼───┐               │
│          │    │  共享存储池        │               │
│          └───►│  (ZFS Mirror/RAIDZ)│               │
│               └───────────────────┘               │
│                                                     │
│  故障切换流程:                                      │
│  1. 心跳检测失败                                    │
│  2. ANA状态切换 (Opt → NonOpt)                     │
│  3. Standby接管                                    │
│  4. Initiator自动重连                              │
│                                                     │
└─────────────────────────────────────────────────────┘
```

#### HA功能清单

| 功能 | 说明 | 实现方式 |
|------|------|----------|
| 节点心跳检测 | 1s间隔健康检查 | gRPC心跳 |
| 状态同步 | 配置实时同步 | etcd/raft |
| 自动故障切换 | <30s切换时间 | ANA状态更新 |
| 数据一致性 | 共享存储保证 | ZFS replication |
| 回切恢复 | 手动/自动回切 | 管理员决策 |

### 3.2 认证与加密

#### DH-HMAC-CHAP认证

```go
// pkg/storage/nvmeof/auth.go

// DHCHAPConfig DH-HMAC-CHAP认证配置
type DHCHAPConfig struct {
    // 密钥配置
    KeyID       string `json:"keyId"`       // 密钥标识
    Secret      string `json:"secret"`      // 认证密钥
    
    // 哈希算法
    HashAlgo    string `json:"hashAlgo"`    // sha256/sha384/sha512
    
    // DH组配置
    DHGroup     string `json:"dhGroup"`     // ffdhe2048/ffdhe3072...
}
```

#### TLS加密传输

```go
// TLSConfig TLS配置
type TLSConfig struct {
    Enabled     bool   `json:"enabled"`     // 启用TLS
    CertFile    string `json:"certFile"`    // 证书文件
    KeyFile     string `json:"keyFile"`     // 私钥文件
    MinVersion  string `json:"minVersion"`  // TLS 1.2/1.3
    ClientAuth  bool   `json:"clientAuth"`  // 客户端认证
}
```

---

## 四、对标TrueNAS 25.10 Goldeye

### 4.1 功能对标矩阵

| 功能特性 | TrueNAS 25.10 | nas-os Phase 1 | Phase 2 | Phase 3 |
|----------|---------------|----------------|---------|---------|
| NVMe/TCP Target | ✅ Community | ✅ | - | - |
| NVMe/RDMA Target | ✅ Enterprise | ✅ | 优化 | 优化 |
| 400GbE支持 | ✅ | ⚠️需验证 | ✅ | ✅ |
| WebUI完整集成 | ✅ | ❌ | ✅ | ✅ |
| ANA多路径 | ✅ | ❌ | ✅ | ✅ |
| DH-HMAC-CHAP | ✅ | ❌ | ❌ | ✅ |
| TLS加密 | ✅ | ❌ | ❌ | ✅ |
| 企业级HA | ✅ Scale | ❌ | ❌ | ✅ |
| SPDK高性能栈 | ✅可选 | ❌ | 评估 | 可选 |
| 性能监控仪表板 | ✅ | ❌ | ✅ | ✅ |

### 4.2 性能对标目标

| 性能指标 | TrueNAS 25.10 RDMA | nas-os Phase 2目标 | Phase 3目标 |
|----------|--------------------|--------------------|-------------|
| 顺序读带宽 | 75 GB/s | 75+ GB/s | 78+ GB/s |
| 顺序写带宽 | 60 GB/s | 60+ GB/s | 65+ GB/s |
| 随机读 IOPS | 5M+ | 4.5M+ | 5M+ |
| 随机写 IOPS | 4M+ | 3.5M+ | 4M+ |
| 平均延迟 | 10-20 μs | < 15 μs | < 12 μs |
| P99延迟 | < 100 μs | < 30 μs | < 20 μs |
| HA切换时间 | < 30s | - | < 30s |

### 4.3 差距分析与追赶策略

| 差距项 | 原因 | 追赶策略 | 预计完成 |
|--------|------|----------|----------|
| WebUI缺失 | 优先API开发 | Phase 2同步开发 | M110 |
| ANA未实现 | 单路径先实现 | Phase 2核心功能 | M111 |
| 认证缺失 | 安全优先级后置 | Phase 3企业特性 | M114 |
| HA集群 | 架构复杂度高 | Phase 3渐进实现 | M116 |
| SPDK栈 | 可选高级特性 | Phase 3评估引入 | M116+ |

---

## 五、里程碑规划

### Phase 2 里程碑

| 里程碑 | 目标 | 预计完成 |
|--------|------|----------|
| M109 | RDMA性能优化 + 基础WebUI | 2026-04-15 |
| M110 | WebUI完整集成 + 性能仪表板 | 2026-04-30 |
| M111 | ANA多路径支持 | 2026-05-15 |
| M112 | ACL增强 + Phase 2完成 | 2026-05-30 |

### Phase 3 里程碑

| 里程碑 | 目标 | 预计完成 |
|--------|------|----------|
| M113 | DH-HMAC-CHAP认证 | 2026-06-15 |
| M114 | TLS加密传输 | 2026-06-30 |
| M115 | 企业级HA框架 | 2026-07-15 |
| M116 | Phase 3完成 + SPDK评估 | 2026-07-30 |

---

## 六、风险评估

| 风险项 | 影响 | 缓解措施 |
|--------|------|----------|
| RDMA硬件依赖 | Phase 2进度 | 软件模拟测试先行 |
| ANA内核版本要求 | 功能限制 | 检测内核版本适配 |
| HA架构复杂度 | Phase 3延期 | 分模块渐进实现 |
| SPDK集成难度 | 可选功能延期 | 保持可选不阻塞主线 |
| WebUI开发资源 | Phase 2延期 | 优先P0功能 |

---

**文档版本**: v1.0
**创建日期**: 2026-04-05
**负责部门**: 工部