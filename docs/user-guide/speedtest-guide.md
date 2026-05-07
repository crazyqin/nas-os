# 网络测速使用指南

> **模块**: 网络诊断 | **版本**: v2.483.0 | **API**: `/api/v1/network/diagnostics/*`

---

## 1. 简介

NAS-OS 内建网络测速与诊断工具，提供 Ping、Traceroute、DNS 查询、端口扫描等全套网络诊断能力。帮助用户快速定位网络问题、评估带宽质量。此功能为 NAS-OS 独占，群晖 / TrueNAS / 飞牛均无原生网络测速工具。

### 适用场景
- 检测 NAS 到目标服务器的网络延迟
- 排查网络连接故障
- 评估外网访问速度
- 检测端口是否可达
- DNS 解析问题诊断

---

## 2. 功能特性

| 功能 | 说明 |
|------|------|
| **Ping 测试** | 发送 ICMP 包，测量延迟、丢包率、RTT 统计 |
| **Traceroute** | 逐跳路由追踪，定位网络瓶颈 |
| **DNS 查询** | A / AAAA / MX / NS / TXT 多种记录查询 |
| **端口扫描** | TCP 端口开放状态检测 |
| **带宽测试** | 上行 / 下行速度测量 |

### Ping 输出指标
| 指标 | 说明 |
|------|------|
| PacketsSent | 发送包数 |
| PacketsRecv | 接收包数 |
| PacketLoss | 丢包率（%） |
| MinRTT | 最小延迟（ms） |
| MaxRTT | 最大延迟（ms） |
| AvgRTT | 平均延迟（ms） |
| StdDevRTT | 延迟标准差（ms） |

---

## 3. 配置方法

网络测速功能开箱即用，无需额外配置。

### 前置条件
- NAS-OS v2.483.0 或更高版本
- `ping`、`traceroute` 命令已安装（通常系统自带）
- 如需 DNS 查询，需确保 DNS 服务可达

---

## 4. 使用示例

### 4.1 Ping 测试

**基本 Ping：**

```bash
curl -X POST http://NAS_IP:8080/api/v1/network/diagnostics/ping \
  -H "Content-Type: application/json" \
  -d '{
    "host": "8.8.8.8",
    "count": 4,
    "timeout": 1000
  }'
```

**返回示例：**

```json
{
  "host": "8.8.8.8",
  "packetsSent": 4,
  "packetsRecv": 4,
  "packetLoss": 0,
  "minRtt": 1.23,
  "maxRtt": 3.45,
  "avgRtt": 2.10,
  "stdDevRtt": 0.89
}
```

**自定义参数 Ping：**

```bash
curl -X POST http://NAS_IP:8080/api/v1/network/diagnostics/ping \
  -H "Content-Type: application/json" \
  -d '{
    "host": "baidu.com",
    "count": 10,
    "interval": 500,
    "timeout": 2000,
    "size": 64
  }'
```

### 4.2 Traceroute 路由追踪

```bash
curl -X POST http://NAS_IP:8080/api/v1/network/diagnostics/traceroute \
  -H "Content-Type: application/json" \
  -d '{
    "host": "8.8.8.8",
    "maxHops": 30
  }'
```

**返回示例：**

```json
{
  "host": "8.8.8.8",
  "hops": [
    {"hop": 1, "host": "192.168.1.1", "ip": "192.168.1.1", "rtt1": 0.5},
    {"hop": 2, "host": "10.0.0.1", "ip": "10.0.0.1", "rtt1": 2.3},
    {"hop": 3, "host": "8.8.8.8", "ip": "8.8.8.8", "rtt1": 15.2}
  ],
  "complete": true
}
```

### 4.3 DNS 查询

**查询 A 记录：**

```bash
curl "http://NAS_IP:8080/api/v1/network/diagnostics/dns?host=example.com"
```

**返回示例：**

```json
{
  "host": "example.com",
  "addresses": ["93.184.216.34"],
  "cname": "",
  "mxRecords": [],
  "nsRecords": [{"host": "ns1.example.com"}],
  "txtRecords": [],
  "queryTime": 1234567
}
```

### 4.4 端口扫描

```bash
curl -X POST http://NAS_IP:8080/api/v1/network/diagnostics/portscan \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.100",
    "ports": [22, 80, 443, 8080, 3306, 5432]
  }'
```

**返回示例：**

```json
{
  "host": "192.168.1.100",
  "ports": [
    {"port": 22, "protocol": "tcp", "open": true, "service": "ssh"},
    {"port": 80, "protocol": "tcp", "open": true, "service": "http"},
    {"port": 443, "protocol": "tcp", "open": true, "service": "https"},
    {"port": 8080, "protocol": "tcp", "open": true, "service": "http-alt"},
    {"port": 3306, "protocol": "tcp", "open": false},
    {"port": 5432, "protocol": "tcp", "open": false}
  ],
  "scanTime": 1234
}
```

---

## 5. 常见问题

### Q: Ping 超时怎么办？
A: 可能原因：
1. 目标主机禁止 ICMP — 尝试 `traceroute` 或端口扫描
2. 防火墙拦截 — 检查 NAS 防火墙规则
3. 网络不通 — 检查网关和路由配置

### Q: 为什么 Traceroute 显示 `* * *`？
A: 中间路由器可能不响应 ICMP TTL 超时消息。这不代表网络不通，仅表示该跳不返回追踪信息。最终目标可达即可。

### Q: DNS 查询返回空结果？
A: 可能原因：
1. 域名不存在或拼写错误
2. DNS 服务器配置问题 — 检查 `/etc/resolv.conf`
3. 网络不通 — 先用 Ping 测试 DNS 服务器连通性

### Q: 端口扫描结果不准确？
A: 端口扫描基于 TCP 连接尝试。如果目标有防火墙或 IDS，可能屏蔽扫描请求。建议在授权范围内使用此功能。

### Q: 如何定期执行网络诊断？
A: 可配合 NAS-OS 的 Smart Cron 定时任务功能，定期执行 Ping 测试并记录结果，用于网络质量趋势分析。

### Q: 能测到 NAS 的实际带宽吗？
A: 当前版本的带宽测试为应用层测量，结果可能受协议开销影响。如需精确带宽，建议配合 `iperf3` 工具在 NAS 和客户端之间进行测试。
