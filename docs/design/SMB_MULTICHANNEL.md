# SMB Multichannel 设计文档

## 概述

本文档设计 NAS-OS 的 SMB 多通道（Multichannel）架构，参考 TrueNAS Scale 25.10 的实现，支持网络绑定、负载均衡和故障切换。

## 参考来源

- TrueNAS Scale 25.10 SMB Multichannel 实现
- Samba 4.x `server multi channel support` 配置
- SMB 3.0+ 协议规范

---

## 1. 架构设计

### 1.1 核心组件

```
┌─────────────────────────────────────────────────────────────────┐
│                     SMB Multichannel Manager                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │ Interface    │  │ Channel      │  │ Health Check         │   │
│  │ Discovery    │  │ Pool         │  │ Monitor              │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │ Load         │  │ Failover     │  │ Config               │   │
│  │ Balancer     │  │ Controller   │  │ Generator            │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Samba Configuration                           │
│                                                                   │
│  [global]                                                         │
│    server multi channel support = yes                             │
│    interfaces = 192.168.1.10 192.168.2.10 10.0.0.10              │
│    bind interfaces only = yes                                     │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 数据流

```
客户端请求
    │
    ▼
┌─────────────────┐
│ Load Balancer   │ ─── 选择最优通道
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ Channel Pool    │ ─── 通道状态管理
└─────────────────┘
    │
    ├──── Channel 1 (eth0) ──── 192.168.1.10:445
    ├──── Channel 2 (eth1) ──── 192.168.2.10:445
    └──── Channel 3 (bond0) ──── 10.0.0.10:445
    │
    ▼
┌─────────────────┐
│ Health Monitor  │ ─── 持续监控通道健康
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ Failover Ctrl   │ ─── 故障时切换通道
└─────────────────┘
```

---

## 2. 网络绑定

### 2.1 接口发现策略

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `auto_discover` | 自动发现所有物理网卡 | 默认模式，简单部署 |
| `manual` | 手动指定接口列表 | 生产环境，精确控制 |
| `bond_aware` | 感知bond接口优先 | 高可用部署 |

### 2.2 接口选择规则

```yaml
interface_selection:
  priorities:
    - type: bond        # 最高优先级
      min_speed: 1000   # Mbps
      weight: 100
    
    - type: ethernet
      min_speed: 1000
      weight: 80
    
    - type: ethernet
      min_speed: 100
      weight: 50
    
  filters:
    exclude:
      - loopback
      - virtual         # docker0, veth*
      - wireless        # wlan* (可选排除)
    
    require:
      - ipv4_address
      - link_up
```

### 2.3 接口类型判断

| 接口名称前缀 | 类型 |
|-------------|------|
| `lo` | loopback |
| `eth`, `en` | ethernet |
| `wlan`, `wl` | wireless |
| `br` | bridge |
| `bond` | bond |
| `docker`, `veth` | virtual |

---

## 3. 负载均衡

### 3.1 负载均衡策略

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `round_robin` | 轮询分配 | 连接数均匀，适合多客户端 |
| `least_connections` | 最少连接优先 | 动态负载，单个客户端多连接 |
| `weighted` | 按带宽权重分配 | 异构网络，如1G+10G混合 |
| `source_ip_hash` | 源IP哈希固定 | 保持客户端到同一通道 |

### 3.2 Round Robin 实现

```go
// 轮询选择通道
func (m *MultichannelManager) GetRoundRobinInterface() *NetworkInterface {
    // 找到下一个健康的通道
    for i := 0; i < len(m.channels); i++ {
        idx := (m.currentIndex + i) % len(m.channels)
        ch := m.channels[idx]
        
        if ch.Connected && ch.HealthScore >= 70 {
            m.currentIndex = (idx + 1) % len(m.channels)
            return m.getInterfaceByChannel(ch)
        }
    }
    return nil // 无可用通道
}
```

### 3.3 Least Connections 实现

```go
// 最少连接优先
func (m *MultichannelManager) GetLeastConnectionInterface() *NetworkInterface {
    var best *SMBChannel
    minConn := int64(math.MaxInt64)
    
    for _, ch := range m.channels {
        if ch.Connected && ch.HealthScore >= 70 {
            if ch.Connections < minConn {
                minConn = ch.Connections
                best = ch
            }
        }
    }
    
    if best != nil {
        return m.getInterfaceByChannel(best)
    }
    return nil
}
```

### 3.4 权重计算

```
权重 = 基础权重 × 健康因子 × 负载因子

基础权重:
  - bond0:    100
  - eth0:     80
  - eth1:     80

健康因子:
  - health_score >= 90: 1.0
  - health_score >= 70: 0.8
  - health_score >= 50: 0.5
  - health_score < 50:  0 (禁用)

负载因子:
  - connections < 10:  1.0
  - connections < 50:  0.8
  - connections < 100: 0.6
  - connections >= 100: 0.3
```

---

## 4. 故障切换

### 4.1 故障检测

| 检测项 | 阈值 | 操作 |
|-------|------|------|
| 接口 Down | 立即 | 禁用通道 |
| 丢包率 > 5% | 警告 | 降低健康分数 |
| 丢包率 > 20% | 禁用 | 禁用通道 |
| 连接超时 | 禁用 | 禁用通道 |
| 健康分数 < 50 | 禁用 | 禁用通道 |

### 4.2 健康检查机制

```go
// 健康检查间隔
HealthCheckSec: 30  // 默认30秒

// 健康检查内容
func (m *MultichannelManager) performHealthCheck() {
    for _, channel := range m.channels {
        health := 100
        
        // 1. 接口状态检查
        if !interfaceUp(channel.InterfaceName) {
            health = 0
        }
        
        // 2. 丢包率检查
        droppedRate := getDroppedRate(channel.InterfaceName)
        if droppedRate > 0.20 {
            health -= 50
        } else if droppedRate > 0.05 {
            health -= 20
        }
        
        // 3. TCP连接测试
        if !testTCPConnection(channel.IPAddress, 445) {
            health -= 40
        }
        
        // 4. SMB协议测试
        if !testSMBProtocol(channel.IPAddress) {
            health -= 30
        }
        
        channel.HealthScore = health
        
        // 触发故障切换
        if health < 50 && m.config.FailoverEnabled {
            m.triggerFailover(channel)
        }
    }
}
```

### 4.3 故障切换流程

```
通道故障检测
    │
    ▼
┌─────────────────┐
│ 标记通道禁用     │
│ Connected=false │
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ 选择备用通道     │
│ 选择健康分数最高 │
│ 且连接数最低     │
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ 重新分配连接     │
│ RoundRobin跳过  │
│ 禁用通道         │
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ 记录故障事件     │
│ 发送告警         │
└─────────────────┘
```

### 4.4 通道恢复

```go
// 通道恢复流程
func (m *MultichannelManager) recoverChannel(channel *SMBChannel) {
    // 1. 检查是否满足恢复条件
    if channel.HealthScore >= 70 {
        // 2. 测试连接
        if m.testSMBConnection(channel.IPAddress, channel.Port) {
            channel.Connected = true
            channel.LastError = ""
            channel.ActiveSince = time.Now()
            
            // 3. 通知恢复
            m.notifyChannelRecovered(channel)
        }
    }
}
```

---

## 5. 配置示例

### 5.1 基础配置

```yaml
smb:
  multichannel:
    enabled: true
    max_channels: 4
    auto_discover: true
    interfaces: []
    
    # 负载均衡
    load_balance:
      strategy: round_robin  # round_robin | least_connections | weighted
      
    # 故障切换
    failover:
      enabled: true
      health_check_sec: 30
      min_bandwidth_mbps: 100
      
    # 健康检查
    health_check:
      interval_sec: 30
      timeout_sec: 5
      retry_count: 3
      
    # 子网策略
    require_same_subnet: false
```

### 5.2 手动指定接口

```yaml
smb:
  multichannel:
    enabled: true
    auto_discover: false
    interfaces:
      - bond0
      - eth0
      - eth1
```

### 5.3 Samba 配置生成

```ini
[global]
    server multi channel support = yes
    interfaces = 192.168.1.10/24 192.168.2.10/24 10.0.0.10/24
    bind interfaces only = yes
    
    # 性能优化
    socket options = TCP_NODELAY IPTOS_LOWDELAY SO_KEEPALIVE
    keepalive = 60
    deadtime = 30
```

---

## 6. API 设计

### 6.1 REST API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/smb/multichannel/status` | GET | 获取多通道状态 |
| `/api/v1/smb/multichannel/config` | GET/PUT | 获取/更新配置 |
| `/api/v1/smb/multichannel/channels` | GET | 获取通道列表 |
| `/api/v1/smb/multichannel/channels/:id` | GET | 获取单个通道详情 |
| `/api/v1/smb/multichannel/channels/:id/enable` | POST | 启用通道 |
| `/api/v1/smb/multichannel/channels/:id/disable` | POST | 禁用通道 |
| `/api/v1/smb/multichannel/interfaces` | GET | 获取可用接口列表 |
| `/api/v1/smb/multichannel/metrics` | GET | 获取性能指标 |
| `/api/v1/smb/multichannel/health-check` | POST | 手动触发健康检查 |

### 6.2 状态响应示例

```json
{
  "enabled": true,
  "total_channels": 3,
  "active_channels": 2,
  "total_bandwidth_mbps": 2000,
  "total_connections": 45,
  "channels": [
    {
      "id": 1,
      "interface_name": "bond0",
      "ip_address": "192.168.1.10",
      "port": 445,
      "connected": true,
      "connections": 25,
      "bandwidth_mbps": 1000,
      "health_score": 95,
      "active_since": "2026-04-24T07:00:00Z"
    },
    {
      "id": 2,
      "interface_name": "eth1",
      "ip_address": "192.168.2.10",
      "port": 445,
      "connected": true,
      "connections": 20,
      "bandwidth_mbps": 1000,
      "health_score": 88,
      "active_since": "2026-04-24T07:00:00Z"
    },
    {
      "id": 3,
      "interface_name": "eth2",
      "ip_address": "10.0.0.10",
      "port": 445,
      "connected": false,
      "connections": 0,
      "bandwidth_mbps": 100,
      "health_score": 0,
      "last_error": "接口已断开"
    }
  ],
  "failover_active": false,
  "last_health_check": "2026-04-24T07:30:00Z"
}
```

---

## 7. 性能优化

### 7.1 TrueNAS Scale 25.10 参考

| 配置项 | TrueNAS 值 | 说明 |
|-------|-----------|------|
| `aio read size` | 1 | 异步读取大小 |
| `aio write size` | 1 | 异步写入大小 |
| `socket options` | TCP_NODELAY | 减少延迟 |
| `max xmit` | 65535 | 最大传输单元 |
| `strict sync` | no | 性能优化 |

### 7.2 通道性能监控

```go
// 性能指标
type MultichannelMetrics struct {
    TotalChannels    int
    ActiveChannels   int
    TotalBandwidth   int        // Mbps
    AvgHealthScore   int
    ThroughputRead   int64      // bytes/sec
    ThroughputWrite  int64      // bytes/sec
    ConnectionCount  int
    FailoverCount    int
}
```

---

## 8. 安全考虑

### 8.1 接口隔离

- 只绑定物理接口，排除虚拟接口
- 支持按子网限制通道（`require_same_subnet`）
- 禁用后自动跳过，不暴露故障信息

### 8.2 健康检查安全

- TCP连接测试不传输数据
- SMB协议测试使用匿名探测
- 不暴露内部网络拓扑

---

## 9. 测试用例

### 9.1 功能测试

| 测试 | 验证 |
|------|------|
| 自动发现 | 检测到所有物理网卡 |
| 手动配置 | 只使用指定接口 |
| 负载均衡 | 连接均匀分布 |
| 故障切换 | 通道断开后自动切换 |
| 通道恢复 | 恢复后自动启用 |

### 9.2 性能测试

| 测试 | 目标 |
|------|------|
| 单通道吞吐 | 基准测试 |
| 多通道吞吐 | >= N × 单通道 |
| 故障切换延迟 | < 5秒 |
| 健康检查开销 | < 0.5% CPU |

---

## 10. 实现状态

| 模块 | 状态 | 文件 |
|------|------|------|
| MultichannelManager | 已实现 | `internal/smb/multichannel.go` |
| 接口发现 | 已实现 | `internal/smb/multichannel.go` |
| 健康检查 | 已实现 | `internal/smb/multichannel.go` |
| Round Robin | 已实现 | `internal/smb/multichannel.go` |
| 故障切换 | 已实现 | `internal/smb/failover.go` |
| REST API | 待实现 | `internal/smb/multichannel_api.go` |
| 前端UI | 待实现 | `webui/src/pages/SMBMultichannel.vue` |

---

## 参考文献

1. TrueNAS Scale 25.10 Documentation - SMB Multichannel
2. Samba 4.x Configuration Guide - `server multi channel support`
3. SMB 3.0 Protocol Specification - Multichannel Feature
4. Microsoft TechNet - SMB Multichannel Performance