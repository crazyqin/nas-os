# 内网穿透优化方案

> 参考：飞牛 FN Connect
> 版本：v1.0
> 日期：2026-03-25

---

## 一、现状分析

### 1.1 代码结构

当前存在两个穿透相关模块：

| 模块 | 路径 | 职责 | 状态 |
|------|------|------|------|
| `tunnel` | `internal/tunnel/` | 完整的 STUN/TURN/ICE 实现 | 主要模块 |
| `natpierce` | `internal/natpierce/` | 简化的打洞实现 | 辅助模块 |

**文件统计：**
- `tunnel/`: 12 个文件，约 3500 行代码
- `natpierce/`: 4 个文件，约 800 行代码

### 1.2 已实现功能

#### ✅ 完整实现
- STUN 协议（RFC 5389）：Binding Request/Response、XOR-MAPPED-ADDRESS
- TURN 协议（RFC 5766）：Allocate、CreatePermission、ChannelBind
- NAT 类型检测：Full Cone、Restricted、Port Restricted、Symmetric
- 信令服务：WebSocket 连接、Offer/Answer/Candidate 交换
- ICE 候选收集：host、srflx、relay 候选

#### ⚠️ 部分实现
- P2P 打洞：框架完整，但 `Connect` 方法未实现
- 中继转发：TURN 客户端可用，但数据传输未完整实现
- 自动降级：AutoClient 存在，但降级逻辑简单

#### ❌ 未实现
- 带宽监控：无实时带宽统计
- 连接质量监控：无延迟、丢包率追踪
- 端口预测：打洞失败时的备选策略
- 多路径传输：无并发多通道

### 1.3 架构问题

```
问题1: 模块重复
├── tunnel/p2p.go     ──┐
├── tunnel/stun.go       ├── 功能重叠
├── natpierce/holepunch.go ──┘
└── 需要统一

问题2: 连接流程不完整
├── P2PClient.Connect() 返回 TODO
├── RelayClient.Connect() 返回 TODO
└── AutoClient 依赖上述实现

问题3: 缺少可观测性
├── 无 Prometheus 指标导出
├── 无实时带宽统计
└── 无连接质量监控
```

---

## 二、优化方案

### 2.1 模块整合

**目标：** 合并 `natpierce` 到 `tunnel`，消除重复代码

```
优化后结构：
internal/tunnel/
├── types.go         # 类型定义（保留）
├── config.go        # 配置管理（新增）
├── manager.go       # 隧道管理器（优化）
├── stun.go          # STUN 协议（保留）
├── turn.go          # TURN 协议（保留）
├── ice.go           # ICE 代理（从 p2p.go 重构）
├── holepunch.go     # 打洞逻辑（从 natpierce 合并）
├── signaling.go     # 信令服务（保留）
├── client.go        # 客户端实现（完善）
├── service.go       # 服务层（保留）
├── monitor.go       # 监控模块（新增）
└── metrics.go       # 指标导出（新增）
```

**迁移计划：**
1. 将 `natpierce/holepunch.go` 的打洞逻辑迁移到 `tunnel/holepunch.go`
2. 将 `natpierce/stun.go` 的 NAT 检测合并到 `tunnel/stun.go`
3. 删除 `natpierce` 模块
4. 更新所有导入路径

### 2.2 P2P 连接优化

#### 2.2.1 完善 ICE 连接流程

```go
// ice.go - 新增 ICE 代理

type ICEAgent struct {
    config       ICEConfig
    localUfrag   string
    localPwd     string
    candidates   []ICECandidate
    state        ICEState
    // ...
}

func (a *ICEAgent) Start(ctx context.Context) error {
    // 1. 收集候选
    candidates, err := a.GatherCandidates(ctx)
    
    // 2. 按优先级排序
    a.sortCandidates(candidates)
    
    // 3. 启动连接检查
    go a.connectivityCheckLoop()
    
    return nil
}

func (a *ICEAgent) connectivityCheckLoop() {
    for _, pair := range a.getValidPairs() {
        // 尝试连接每一对候选
        if err := a.tryConnect(pair); err == nil {
            a.selectPair(pair)
            return
        }
    }
}
```

#### 2.2.2 增强打洞策略

```go
// holepunch.go - 打洞策略

type HolePunchStrategy int

const (
    StrategyDirect    HolePunchStrategy = iota  // 直连
    StrategyPortPrediction                      // 端口预测
    StrategyRandomProbe                         // 随机探测
    StrategyRelay                               // 中继回退
)

func (h *HolePuncher) Punch(ctx context.Context, peer *PeerInfo) (*Connection, error) {
    strategies := []HolePunchStrategy{
        StrategyDirect,
        StrategyPortPrediction,
        StrategyRandomProbe,
        StrategyRelay,
    }
    
    for _, strategy := range strategies {
        conn, err := h.tryStrategy(strategy, peer)
        if err == nil {
            return conn, nil
        }
        h.logger.Debug("strategy failed", 
            zap.String("strategy", strategy.String()),
            zap.Error(err))
    }
    
    return nil, ErrHolePunchFailed
}

// 端口预测：针对对称型 NAT
func (h *HolePuncher) portPrediction(basePort int) []int {
    ports := make([]int, 0, 10)
    for delta := -5; delta <= 5; delta++ {
        port := basePort + delta
        if port > 0 && port <= 65535 {
            ports = append(ports, port)
        }
    }
    return ports
}
```

#### 2.2.3 打洞成功率优化

| 场景 | 原方案 | 优化方案 |
|------|--------|----------|
| Full Cone NAT | 直接打洞 | 直接打洞 ✅ |
| Restricted NAT | 打洞 | 打洞 + 保活 |
| Symmetric NAT | 放弃 | 端口预测 + 中继 |
| 双 Symmetric | 中继 | 端口预测 + 中继 |

### 2.3 带宽监控机制

#### 2.3.1 监控数据结构

```go
// monitor.go

type TunnelMetrics struct {
    // 连接统计
    TotalConnections   int64         `json:"total_connections"`
    ActiveConnections  int64         `json:"active_connections"`
    FailedConnections  int64         `json:"failed_connections"`
    
    // 流量统计
    BytesSent          int64         `json:"bytes_sent"`
    BytesReceived      int64         `json:"bytes_received"`
    
    // 实时带宽
    UploadRate         int64         `json:"upload_rate"`   // bytes/sec
    DownloadRate       int64         `json:"download_rate"` // bytes/sec
    
    // 连接质量
    Latency            time.Duration `json:"latency"`
    PacketLoss         float64       `json:"packet_loss"`   // 0.0 - 1.0
    Jitter             time.Duration `json:"jitter"`
    
    // P2P 统计
    P2PSuccessRate     float64       `json:"p2p_success_rate"`
    RelayUsage         float64       `json:"relay_usage"`   // 中继流量占比
}

type PeerMetrics struct {
    PeerID             string        `json:"peer_id"`
    ConnectionType     string        `json:"connection_type"` // p2p/relay
    BytesSent          int64         `json:"bytes_sent"`
    BytesReceived      int64         `json:"bytes_received"`
    Latency            time.Duration `json:"latency"`
    LastActivity       time.Time     `json:"last_activity"`
    Uptime             time.Duration `json:"uptime"`
}
```

#### 2.3.2 带宽统计实现

```go
type BandwidthMonitor struct {
    mu            sync.RWMutex
    samples       []BandwidthSample
    windowSize    time.Duration  // 统计窗口，默认 60s
    
    // 计数器
    bytesSent     int64
    bytesRecv     int64
    lastSent      int64
    lastRecv      int64
    lastUpdate    time.Time
}

type BandwidthSample struct {
    Timestamp time.Time `json:"timestamp"`
    TxBytes   int64     `json:"tx_bytes"`
    RxBytes   int64     `json:"rx_bytes"`
    TxRate    int64     `json:"tx_rate"`  // bytes/sec
    RxRate    int64     `json:"rx_rate"`  // bytes/sec
}

func (m *BandwidthMonitor) Record(sent, recv int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    now := time.Now()
    m.bytesSent += int64(sent)
    m.bytesRecv += int64(recv)
    
    // 计算瞬时速率
    elapsed := now.Sub(m.lastUpdate).Seconds()
    if elapsed > 0 {
        txRate := int64(float64(m.bytesSent-m.lastSent) / elapsed)
        rxRate := int64(float64(m.bytesRecv-m.lastRecv) / elapsed)
        
        m.samples = append(m.samples, BandwidthSample{
            Timestamp: now,
            TxRate:    txRate,
            RxRate:    rxRate,
        })
        
        // 清理过期样本
        m.cleanup()
    }
    
    m.lastSent = m.bytesSent
    m.lastRecv = m.bytesRecv
    m.lastUpdate = now
}

func (m *BandwidthMonitor) GetBandwidth() (upload, download int64) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    if len(m.samples) == 0 {
        return 0, 0
    }
    
    // 计算平均速率
    var txSum, rxSum int64
    for _, s := range m.samples {
        txSum += s.TxRate
        rxSum += s.RxRate
    }
    n := int64(len(m.samples))
    return txSum / n, rxSum / n
}
```

#### 2.3.3 Prometheus 指标导出

```go
// metrics.go

var (
    metricConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "tunnel_connections_total",
        Help: "Total tunnel connections",
    }, []string{"state", "type"})
    
    metricBytes = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "tunnel_bytes_total",
        Help: "Total bytes transferred",
    }, []string{"direction", "peer_id"})
    
    metricBandwidth = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "tunnel_bandwidth_bytes_sec",
        Help: "Current bandwidth in bytes/sec",
    }, []string{"direction"})
    
    metricLatency = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "tunnel_latency_seconds",
        Help: "Connection latency in seconds",
    }, []string{"peer_id"})
    
    metricP2PSuccess = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "tunnel_p2p_success_rate",
        Help: "P2P connection success rate",
    })
    
    metricRelayUsage = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "tunnel_relay_usage_ratio",
        Help: "Relay traffic ratio",
    })
)
```

### 2.4 稳定性改进

#### 2.4.1 自动重连机制

```go
type ReconnectPolicy struct {
    MaxRetries      int           `json:"max_retries"`       // 最大重试次数
    InitialDelay    time.Duration `json:"initial_delay"`     // 初始延迟
    MaxDelay        time.Duration `json:"max_delay"`         // 最大延迟
    BackoffFactor   float64       `json:"backoff_factor"`    // 退避因子
    
    // 状态
    retryCount      int
    lastAttempt     time.Time
}

func (p *ReconnectPolicy) ShouldRetry() bool {
    if p.retryCount >= p.MaxRetries {
        return false
    }
    return true
}

func (p *ReconnectPolicy) NextDelay() time.Duration {
    delay := p.InitialDelay * time.Duration(math.Pow(p.BackoffFactor, float64(p.retryCount)))
    if delay > p.MaxDelay {
        delay = p.MaxDelay
    }
    p.retryCount++
    return delay
}

func (p *ReconnectPolicy) Reset() {
    p.retryCount = 0
}
```

#### 2.4.2 连接健康检查

```go
type HealthChecker struct {
    interval       time.Duration
    timeout        time.Duration
    failureThreshold int
    successThreshold  int
    
    // 状态
    failureCount   int
    successCount   int
    healthy        bool
}

func (h *HealthChecker) Check(conn net.Conn) error {
    // 发送心跳
    deadline := time.Now().Add(h.timeout)
    conn.SetDeadline(deadline)
    
    _, err := conn.Write([]byte("PING"))
    if err != nil {
        h.recordFailure()
        return err
    }
    
    // 等待响应
    buf := make([]byte, 4)
    _, err = conn.Read(buf)
    if err != nil {
        h.recordFailure()
        return err
    }
    
    if string(buf) != "PONG" {
        h.recordFailure()
        return errors.New("invalid pong")
    }
    
    h.recordSuccess()
    return nil
}

func (h *HealthChecker) recordFailure() {
    h.failureCount++
    h.successCount = 0
    if h.failureCount >= h.failureThreshold {
        h.healthy = false
    }
}

func (h *HealthChecker) recordSuccess() {
    h.successCount++
    h.failureCount = 0
    if h.successCount >= h.successThreshold {
        h.healthy = true
    }
}
```

#### 2.4.3 降级策略

```go
type FailoverPolicy struct {
    strategies []ConnectionStrategy
    current    int
}

type ConnectionStrategy struct {
    Name        string
    Priority    int
    MinDuration time.Duration  // 最少使用时长才能降级
    StartTime   time.Time
}

func (f *FailoverPolicy) ShouldFailover(err error) bool {
    // 判断是否需要降级
    if err == nil {
        return false
    }
    
    current := f.strategies[f.current]
    elapsed := time.Since(current.StartTime)
    
    // 使用时长不足，不降级
    if elapsed < current.MinDuration {
        return false
    }
    
    // 检查是否还有下一策略
    return f.current < len(f.strategies)-1
}

func (f *FailoverPolicy) NextStrategy() *ConnectionStrategy {
    if f.current < len(f.strategies)-1 {
        f.current++
        f.strategies[f.current].StartTime = time.Now()
        return &f.strategies[f.current]
    }
    return nil
}
```

---

## 三、实施计划

### 3.1 阶段一：模块整合（2天）

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 合并 natpierce 到 tunnel | P0 | 4h |
| 完善 P2PClient.Connect() | P0 | 4h |
| 完善 RelayClient.Connect() | P0 | 2h |
| 单元测试补充 | P1 | 4h |

### 3.2 阶段二：打洞优化（2天）

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 实现端口预测策略 | P0 | 3h |
| 增强 ICE 连接检查 | P0 | 4h |
| 对称型 NAT 支持 | P1 | 3h |
| 集成测试 | P1 | 2h |

### 3.3 阶段三：监控机制（2天）

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 带宽统计模块 | P0 | 3h |
| Prometheus 指标 | P0 | 3h |
| 连接质量监控 | P1 | 4h |
| 监控 API | P1 | 2h |

### 3.4 阶段四：稳定性增强（1天）

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 自动重连机制 | P0 | 2h |
| 健康检查 | P0 | 2h |
| 降级策略 | P1 | 2h |
| 压力测试 | P1 | 2h |

---

## 四、预期效果

### 4.1 性能指标

| 指标 | 当前 | 目标 |
|------|------|------|
| P2P 成功率 | ~60% | >85% |
| 连接延迟 | 未知 | <100ms |
| 带宽开销 | 未知 | <5% |
| 重连时间 | 未知 | <3s |

### 4.2 功能对比

| 功能 | FN Connect | 优化后 |
|------|------------|--------|
| NAT 检测 | ✅ | ✅ |
| P2P 打洞 | ✅ | ✅ |
| 端口预测 | ✅ | ✅ |
| 中继回退 | ✅ | ✅ |
| 带宽监控 | ✅ | ✅ |
| 质量监控 | ❌ | ✅ |
| 自建信令 | ❌ | ✅ |
| 开源 | ❌ | ✅ |

---

## 五、风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 对称型 NAT 穿透困难 | 中 | 端口预测 + 中继回退 |
| STUN 服务器不可用 | 高 | 多服务器备份 |
| TURN 中继成本 | 中 | 按需启用，流量限制 |
| 代码重构风险 | 低 | 分阶段迁移，保留测试 |

---

## 六、参考资源

- [RFC 5389: STUN](https://tools.ietf.org/html/rfc5389)
- [RFC 5766: TURN](https://tools.ietf.org/html/rfc5766)
- [RFC 5245: ICE](https://tools.ietf.org/html/rfc5245)
- [WebRTC ICE 实现](https://webrtc.org/)
- [WireGuard 协议](https://www.wireguard.com/protocol/)