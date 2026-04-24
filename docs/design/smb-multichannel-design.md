# SMB Multichannel 设计文档

> 版本: v2.462.0
> 更新日期: 2026-04-24
> 对标: TrueNAS 26 SMB Stateful Failover

---

## 设计目标

SMB Multichannel允许客户端通过多个网络通道同时连接到SMB服务器，实现：
- **带宽聚合**: 多网卡并发传输，提升吞吐量
- **故障切换**: 单通道故障时自动切换，保证服务连续性
- **负载均衡**: 智能分配连接，优化资源利用

---

## 已实现功能

### 核心架构 (internal/smb/multichannel.go)

```
┌─────────────────────────────────────────────────────────────┐
│                   MultichannelManager                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Channel 1    │  │ Channel 2    │  │ Channel N    │       │
│  │ eth0: 1Gbps  │  │ eth1: 1Gbps  │  │ ethN: xGbps  │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              HealthCheck Loop (30s)                  │    │
│  │  - 接口状态检测                                       │    │
│  │  - 丢包率监控                                         │    │
│  │  - SMB连接测试                                        │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              RoundRobin LoadBalancer                 │    │
│  │  - 轮询分配连接                                       │    │
│  │  - 健康通道优先                                       │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 核心组件

| 组件 | 文件 | 功能 |
|------|------|------|
| MultichannelManager | multichannel.go | 多通道管理核心 |
| StatefulFailoverManager | stateful/manager.go | 会话状态故障转移 |
| LoadBalancer | stateful/loadbalancer.go | 负载均衡策略 |
| FailoverIntegration | stateful/failover.go | HA模块集成 |

### 配置参数

```go
type MultichannelConfig struct {
    Enabled           bool     // 启用多通道
    MaxChannels       int      // 最大通道数 (默认4)
    Interfaces        []string // 绑定网卡
    AutoDiscover      bool     // 自动发现网卡
    RoundRobin        bool     // 轮询负载均衡
    FailoverEnabled   bool     // 故障切换
    HealthCheckSec    int      // 健康检查间隔(秒)
    MinBandwidthMbps  int      // 最低带宽要求
    RequireSameSubnet bool     // 要求同一子网
}
```

---

## 对标TrueNAS 26

| 功能 | TrueNAS 26 | nas-os v2.462.0 | 状态 |
|------|------------|-----------------|------|
| SMB多通道 | ✅ | ✅ 已实现 | **对标完成** |
| 会话状态故障转移 | ✅ Stateful Failover | ✅ stateful/ | **对标完成** |
| 健康检查 | ✅ | ✅ 30s周期 | **对标完成** |
| 负载均衡 | ✅ | ✅ RoundRobin | **对标完成** |
| RDMA支持 | ✅ SMB Direct | 📋 Phase2 | 待实现 |
| ANA多路径 | ✅ | 📋 NVMe-oF | 待实现 |

---

## API端点

### 已实现

```
GET  /api/v1/smb/multichannel/status      - 获取多通道状态
POST /api/v1/smb/multichannel/config      - 更新配置
POST /api/v1/smb/multichannel/channel/{id}/enable   - 启用通道
POST /api/v1/smb/multichannel/channel/{id}/disable  - 禁用通道
GET  /api/v1/smb/multichannel/metrics     - 获取性能指标
```

### 返回示例

```json
{
  "enabled": true,
  "total_channels": 4,
  "active_channels": 3,
  "total_bandwidth_mbps": 3000,
  "channels": [
    {
      "id": 1,
      "interface_name": "eth0",
      "ip_address": "192.168.1.10",
      "bandwidth_mbps": 1000,
      "connected": true,
      "health_score": 100
    }
  ]
}
```

---

## 下一步计划 (Phase2)

### RDMA支持 (SMB Direct)

```go
// internal/smb/rdma.go (规划)
type SMBDirectConfig struct {
    Enabled       bool
    RDMAInterfaces []string  // RDMA网卡列表
    MaxThroughput int        // 最大吞吐量目标
}
```

### ANA多路径集成

与NVMe-oF ANA多路径共享负载均衡逻辑。

---

## 性能指标

- **单通道**: ~1Gbps吞吐
- **4通道聚合**: ~4Gbps理论吞吐
- **故障切换延迟**: <5秒
- **健康检查周期**: 30秒

---

*设计文档由兵部维护*