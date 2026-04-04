# NVMe over Fabric 设计文档

> **兵部第162轮任务** - 对标TrueNAS 25.10 Goldeye
> **参考**: TrueNAS Scale 25.10 NVMe over Fabric Implementation

---

## 1.概述

### 1.1设计目标

对标TrueNAS 25.10 NVMe over Fabric功能，实现：
- **NVMe/TCP**: 标准TCP网络传输（Community Edition）
- **NVMe/RDMA**: 高性能RDMA传输（Enterprise硬件）
- **400GbE支持**: 高速网络接口兼容
- **多路径支持**: 高可用连接

### 1.2 TrueNAS 25.10架构分析

TrueNAS使用以下组件实现NVMe-oF：
- **Linux NVMe Target**:内核级NVMe target服务
- **SPDK**:可选的高性能用户态NVMe栈
- **NVMe initiator**:客户端连接支持

---

## 2.系统架构

### 2.1 核心组件

```
┌─────────────────────────────────────────────────────┐
│                  NVMe-oF API                         │
│  (Target管理/连接配置/性能监控)                       │
├─────────────────────────────────────────────────────┤
│               NVMe Target Service                    │
│  (Subsystem/Namespace/Port/ACL管理)                  │
├─────────────────────────────────────────────────────┤
│              Transport Layer                         │
│  (TCP Transport / RDMA Transport)                    │
├─────────────────────────────────────────────────────┤
│               Storage Backend                        │
│  (NVMe SSD / ZFS NVMe Special VDEV)                  │
└─────────────────────────────────────────────────────┘
```

### 2.2 连接流程

1. 创建NVMe Subsystem（命名空间容器）
2. 配置Transport（TCP/RDMA端口）
3. 创建Namespace（存储卷映射）
4. 配置ACL（访问控制）
5. 客户端发起连接
6. 多路径负载均衡（可选）

---

## 3. 功能设计

### 3.1 Target配置

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| Subsystem NQN | NVMe Qualified Name | 自动生成 |
| Transport Type | TCP / RDMA | TCP |
| Port ID | 端口标识 | 1 |
| IP Address | 监听地址 | 0.0.0.0 |
| Port Number | TCP端口 | 4420 |
| Max Namespaces | 最大命名空间数 | 256 |

### 3.2 Namespace配置

| 配置项 | 说明 |
|--------|------|
| NSID | Namespace ID |
| Device Path | 后端设备路径（NVMe SSD或ZFS volume） |
| Size | 命名空间大小 |
| Block Size | 块大小（512/4096） |
| ANAGRPID | ANA组ID（多路径） |

### 3.3 Transport对比

| 特性 | NVMe/TCP | NVMe/RDMA |
|------|----------|-----------|
| **性能** | ~80%本地 | ~95%本地 |
| **延迟** | ~50μs | ~10μs |
| **网络要求** | 标准TCP/IP | RDMA网卡（IB/RoCE） |
| **适用场景** | Community通用 | Enterprise高性能 |
| **成本** | 低 | 高（专用网卡） |

---

## 4. API设计

### 4.1 REST API

```yaml
# Subsystem管理
POST /api/v1/nvme/subsystems
  Body:
    - nqn: Subsystem NQN
    - max_namespaces: 最大命名空间数
    
GET /api/v1/nvme/subsystems
  Response:
    - subsystems: [{nqn, namespaces, ports}]

DELETE /api/v1/nvme/subsystems/:nqn

# Namespace管理
POST /api/v1/nvme/subsystems/:nqn/namespaces
  Body:
    - device: 设备路径
    - size: 大小（可选，默认使用设备大小）

GET /api/v1/nvme/subsystems/:nqn/namespaces

DELETE /api/v1/nvme/subsystems/:nqn/namespaces/:nsid

# Transport配置
POST /api/v1/nvme/transports
  Body:
    - type: tcp/rdma
    - port: 端口号
    - ip: 监听地址

# 连接监控
GET /api/v1/nvme/connections
  Response:
    - connections: [{hostnqn, state, paths}]
```

### 4.2 命令行接口

```bash
# 创建Subsystem
nasctl nvme subsystem create mypool --nqn "nqn.2026-04.nas-os:mypool"

# 添加Namespace
nasctl nvme namespace add mypool --device /dev/nvme0n1

# 配置TCP Transport
nasctl nvme transport create tcp --port 4420 --ip 192.168.1.100

# 配置RDMA Transport
nasctl nvme transport create rdma --port 4420 --ip 192.168.1.100

# 查看连接状态
nasctl nvme connections list
```

---

## 5. 性能考虑

### 5.1 TCP优化

- **零拷贝**: 使用sendfile/splice
- **TCP调优**: 调整buffer大小、拥塞控制
- **多队列**: 多线程处理连接

### 5.2 RDMA优化

- **内存注册**: 预注册内存区域
- **Completion Queue**: 多CQ并行
- **Flow Control**: 智能流控策略

### 5.3 后端存储优化

- **NVMe SSD直通**: 最大性能
- **ZFS NVMe Special VDEV**: 混合存储
- **Direct I/O**: 绕过缓存层

---

## 6. 安全考虑

### 6.1 访问控制

- **ACL**: 基于HostNQN的访问控制
- **认证**: DH-HMAC-CHAP认证支持
- **加密**: TLS over TCP可选

### 6.2 网络安全

- **IP白名单**: 限制允许连接的IP
- **端口隔离**: 专用网络接口
- **防火墙**: 自动配置防火墙规则

---

## 7. 实现计划

### Phase 1: 基础实现
- NVMe/TCP target服务
- Subsystem/Namespace管理API
- 基础连接支持

### Phase 2: 高级功能
- RDMA transport支持
- 多路径配置
- 性能监控仪表板

### Phase 3: 企业特性
- DH-HMAC-CHAP认证
- TLS加密传输
- 高可用集群支持

---

## 8. 对标TrueNAS差异

| 功能 | TrueNAS 25.10 | nas-os v2.393.0 | 对标建议 |
|------|---------------|-----------------|----------|
| NVMe/TCP | ✅ Community | 📋Phase 1 | M108实现 |
| NVMe/RDMA | ✅ Enterprise | 📋Phase 2 | M109实现 |
| 400GbE支持 | ✅ | ⚠️ 需验证驱动 | 驱动兼容检查 |
| SPDK高性能栈 | ✅ 可选 | 📋Phase 3 | 评估需求 |
| WebUI集成 | ✅ 完整 | 📋Phase 1 | 同步开发 |

---

**预计完成**: M108 (Phase 1) - 2026-04-15