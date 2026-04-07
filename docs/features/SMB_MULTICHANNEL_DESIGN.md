# SMB Multichannel 设计文档

> 版本: v2.421.0 | 编制: 兵部 | 日期: 2026-04-07

## 一、概述

SMB Multichannel（多通道）是SMB 3.x协议的关键特性，允许客户端通过多个网络连接同时访问服务器，显著提升吞吐量和可靠性。

## 二、竞品对标

### TrueNAS SMB Multichannel
- 自动发现多个网络接口
- 支持网卡绑定（LAGG）
- 负载均衡：轮询/源地址哈希
- 故障自动切换

### 群晖 DSM
- 多网卡自动识别
- SMB Multichannel自动启用
- 支持IEEE 802.3ad动态聚合

## 三、NAS-OS设计

### 3.1 架构组件

```
┌─────────────────────────────────────────────┐
│           SMB Multichannel Manager          │
├─────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐           │
│  │ Interface   │  │ Load Balance│           │
│  │ Discovery   │  │ Algorithm   │           │
│  └─────────────┘  └─────────────┘           │
│  ┌─────────────┐  ┌─────────────┐           │
│  │ Channel     │  │ Failover    │           │
│  │ Monitor     │  │ Handler     │           │
│  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────┘
          │                    │
          ▼                    ▼
    ┌──────────┐         ┌──────────┐
    │ NIC 1    │         │ NIC 2    │
    │ (eth0)   │         │ (eth1)   │
    └──────────┘         └──────────┘
```

### 3.2 核心功能

#### 网卡发现
```go
type InterfaceInfo struct {
    Name      string   // 网卡名称
    IP        string   // IP地址
    Speed     int      // 链路速度(Mbps)
    MTU       int      // MTU大小
    Status    string   // up/down
    IsSMBCap  bool     // SMB能力标记
}
```

#### 负载均衡策略
| 算法 | 描述 | 适用场景 |
|------|------|----------|
| RoundRobin | 轮询分配 | 相同速度网卡 |
| SourceHash | 源地址哈希 | 固定客户端路由 |
| LeastLoad | 最小负载 | 异速网卡 |
| Adaptive | 自适应 | 动态调整 |

#### 故障切换机制
- 通道健康检查（心跳检测）
- 故障通道自动剔除
- 客户端透明重连
- 恢复后自动加回

### 3.3 API设计

```
GET  /api/v1/smb/channels         # 获取通道列表
POST /api/v1/smb/channels/enable  # 启用多通道
GET  /api/v1/smb/channels/status  # 通道状态
POST /api/v1/smb/channels/config  # 配置负载均衡策略
```

### 3.4 配置示例

```yaml
smb:
  multichannel:
    enabled: true
    interfaces:
      - eth0  # 1GbE
      - eth1  # 1GbE
    load_balance: round_robin
    failover:
      enabled: true
      check_interval: 5s
      timeout: 30s
```

## 四、实现路线图

| 阶段 | 任务 | 周期 |
|------|------|------|
| Phase 1 | 网卡自动发现 | M113 |
| Phase 2 | 基础多通道支持 | M114 |
| Phase 3 | 负载均衡实现 | M115 |
| Phase 4 | 故障切换完善 | M116 |

## 五、性能预期

- **吞吐提升**: 2x-4x（多网卡）
- **延迟降低**: 20%-30%
- **可靠性**: 单网卡故障不影响服务

---

*文档编制: 兵部技术组*
*更新频率: 每版本同步*