# SMB 多通道配置指南

> **模块**: SMB Multichannel | **版本**: v2.483.0 | **API**: `/api/v1/smb/multichannel/*`

---

## 1. 简介

SMB Multichannel（SMB 多通道）允许 SMB 文件传输同时使用多个网络连接，聚合带宽、提升传输性能。NAS-OS 的 SMB Multichannel 对标 TrueNAS 的同名功能，支持多网卡聚合、四种负载均衡模式、自动健康监控和故障切换。

### 适用场景
- 多网卡 NAS 的带宽聚合（2.5GbE / 10GbE）
- 大文件传输加速（视频编辑、备份、虚拟化存储）
- 高可用网络连接（故障自动切换）
- 多客户端并发访问优化

---

## 2. 功能特性

### 2.1 核心能力
- **多通道聚合**: 自动检测并使用多块网卡建立 SMB 连接
- **四种负载均衡**: Round Robin / Least Load / Hash / Adaptive
- **自动故障切换**: 通道异常时自动切换到健康通道
- **RSS 卸载**: Receive Side Scaling 硬件卸载，降低 CPU 开销
- **Jumbo Frame**: 支持 MTU 9000 大帧，减少协议开销
- **实时监控**: 带宽、延迟、丢包率实时统计

### 2.2 负载均衡模式

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| **round_robin** | 轮询分配 | 网卡性能一致时使用 |
| **least_load** | 最少负载优先 | 网卡性能差异较大时使用 |
| **hash** | 源/目标 IP 哈希 | 需要会话保持时使用 |
| **adaptive** | 自适应（默认） | 综合考虑带宽、延迟、丢包率 |

### 2.3 通道状态

| 状态 | 说明 |
|------|------|
| **active** | 所有通道正常 |
| **degraded** | 部分通道故障，仍可工作 |
| **failed** | 所有通道不可用 |

---

## 3. 配置方法

### 3.1 检查网卡状态

首先确认 NAS 上有多块可用网卡：

```bash
# Linux 下查看网卡
ip link show
# 或
ls /sys/class/net/
```

多通道至少需要 2 块已启用且分配了 IP 地址的网卡。

### 3.2 配置参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | true | 是否启用多通道 |
| `max_channels_per_client` | 4 | 每客户端最大通道数 |
| `min_channels` | 1 | 最少通道数要求 |
| `health_check_interval` | 10s | 健康检查间隔 |
| `failover_timeout` | 5s | 故障切换超时 |
| `balance_mode` | adaptive | 负载均衡模式 |
| `mtu` | 9000 | 最大传输单元（Jumbo Frame） |
| `rss_enabled` | true | 启用 Receive Side Scaling |

### 3.3 通过 API 配置

```bash
# 更新多通道配置
curl -X PUT http://NAS_IP:8080/api/v1/smb/multichannel/config \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "max_channels_per_client": 4,
    "min_channels": 2,
    "health_check_interval": 10000000000,
    "failover_timeout": 5000000000,
    "balance_mode": "adaptive",
    "interfaces": ["eth0", "eth1", "eth2"],
    "mtu": 9000,
    "rss_enabled": true
  }'
```

---

## 4. 使用示例

### 4.1 获取多通道状态

```bash
curl http://NAS_IP:8080/api/v1/smb/multichannel/status
```

**返回示例：**

```json
{
  "enabled": true,
  "active_groups": 3,
  "total_bandwidth_mbps": 30000,
  "utilized_bandwidth_mbps": 12500,
  "failover_count": 0,
  "avg_latency_ms": 0.85
}
```

### 4.2 查看通道组详情

```bash
curl http://NAS_IP:8080/api/v1/smb/multichannel/groups
```

**返回示例：**

```json
{
  "groups": [
    {
      "client_ip": "192.168.1.100",
      "channels": [
        {
          "id": "192.168.1.100-eth0-0",
          "local_addr": "192.168.1.10",
          "interface": "eth0",
          "bandwidth_mbps": 10000,
          "latency": 1200000,
          "packet_loss": 0,
          "state": "up"
        },
        {
          "id": "192.168.1.100-eth1-1",
          "local_addr": "192.168.1.11",
          "interface": "eth1",
          "bandwidth_mbps": 10000,
          "latency": 980000,
          "packet_loss": 0,
          "state": "up"
        }
      ],
      "total_bandwidth_mbps": 20000,
      "state": "active"
    }
  ]
}
```

### 4.3 获取性能指标

```bash
curl http://NAS_IP:8080/api/v1/smb/multichannel/metrics
```

### 4.4 更新负载均衡模式

```bash
curl -X PUT http://NAS_IP:8080/api/v1/smb/multichannel/config \
  -H "Content-Type: application/json" \
  -d '{"balance_mode": "least_load"}'
```

---

## 5. 常见问题

### Q: 如何确认 SMB Multichannel 生效？
A: 在 Windows 客户端上，打开 PowerShell 执行：
```powershell
Get-SmbMultichannelConnection
```
如果显示多条连接到 NAS IP，说明多通道已生效。

### Q: Windows 10/11 支持 SMB Multichannel 吗？
A: 支持。Windows 10 Pro 及以上版本默认启用 SMB Multichannel。确保客户端有多块网卡或单网卡多队列（RSS）。

### Q: 2.5GbE 网卡能用多通道吗？
A: 可以。SMB Multichannel 不限于万兆网卡，2.5GbE、5GbE、10GbE 均可使用。两块 2.5GbE 网卡聚合可获得约 5Gbps 的理论带宽。

### Q: 需要配置 LACP/Link Aggregation 吗？
A: **不需要**，而且建议不要同时使用。SMB Multichannel 在应用层实现多连接聚合，与交换机层面的 LACP 是独立的。同时使用可能产生冲突。

### Q: Jumbo Frame 需要交换机支持吗？
A: 是的。MTU 9000 需要从 NAS 到客户端的整条链路（交换机、网卡）都支持 Jumbo Frame。如果中间有不支持的设备，会导致分片甚至丢包。如不确定，请保持 MTU 1500。

### Q: 通道故障后多久恢复？
A: 健康检查间隔默认 10 秒。通道故障后，下一次健康检查（≤10秒）会检测到并触发故障切换。通道恢复后，下次检查自动将其标记为可用。

### Q: macOS 支持 SMB Multichannel 吗？
A: macOS 从 Ventura (13) 开始支持 SMB Multichannel。在 `/etc/nsmb.conf` 中添加：
```ini
[default]
multichannel=yes
```
然后重启 SMB 连接即可。

### Q: 最多支持多少条通道？
A: 默认每客户端最多 4 条通道，可通过 `max_channels_per_client` 参数调整。受网卡数量和系统资源限制。
