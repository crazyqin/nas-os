# 户部工作报告 - 第136轮

**提交时间**: 2026-04-01 13:05
**任务**: 内网穿透计费方案 + 存储成本预测

---

## 一、内网穿透免费额度设计

### 1.1 对标竞品定价

| 服务商 | 免费额度 | 付费方案 | 技术架构 |
|--------|----------|----------|----------|
| ngrok | 1隧道/在线端点 | $20/月起(Pay-as-you-go) | 自建服务器 |
| Cloudflare Tunnel | 无限隧道 | 免费(Zero Trust) | 全球边缘节点 |
| 花生壳 | 2个域名 | ¥198/年起 | 国内服务器 |
| 飞牛FN Connect | 基础穿透免费 | 增值服务待公布 | 闭源方案 |

### 1.2 nas-os免费额度设计

**技术基础**: nas-os 已实现完整的内网穿透技术栈（见 `internal/network/tunnel/README.md`）：
- STUN客户端：NAT类型检测
- TURN客户端：中继转发
- ICE代理：连接建立
- 端到端加密：ChaCha20-Poly1305/AES-GCM

**免费额度方案**:

| 额度项 | 免费值 | 设计理由 |
|--------|--------|----------|
| 同时隧道数 | 3个 | 覆盖基础需求：Web、SSH、远程桌面 |
| 每隧道带宽 | 5Mbps | 足够日常使用（网页、文件传输） |
| 月流量上限 | 50GB | 防滥用，正常用户不触及 |
| 自定义域名 | ❌ | 降低运营成本（无需DNS托管） |
| 持续在线时长 | 无限制 | 用户体验优先 |
| 中继时长 | 100小时/月 | P2P失败时TURN中继成本控制 |

### 1.3 技术成本分析

| 方案 | 月成本 | 可靠性 | 维护成本 |
|------|--------|--------|----------|
| Cloudflare Tunnel | ¥0 | 高（全球节点） | 低 |
| 自建frp服务器 | ¥50-100 | 中（单节点） | 中 |
| 公共STUN/TURN | ¥0（Google免费） | 高 | 无 |
| 混合方案 | ¥20-30 | 最高 | 中 |

**推荐方案**: 采用 Cloudflare Tunnel + 公共STUN 混合架构，实现零成本基础服务。

---

## 二、增值服务订阅方案

### 2.1 阶位设计

| 阶位 | 免费 | 进阶版 | 高级版 | 企业版 |
|------|------|--------|--------|--------|
| **价格** | ¥0 | ¥29/月 | ¥99/月 | ¥299/月 |
| **同时隧道** | 3 | 10 | 30 | 无限 |
| **带宽上限** | 5Mbps | 50Mbps | 200Mbps | 无限 |
| **月流量** | 50GB | 200GB | 1TB | 无限 |
| **自定义域名** | ❌ | 1个 | 5个 | 20个 |
| **TLS证书** | 自动(公共) | 自动+自定义 | 自动+通配符 | 企业证书 |
| **TURN中继** | 100h/月 | 无限 | 无限 | 无限+专属节点 |
| **技术支持** | 社区 | 工单响应24h | 工单响应4h | 专属客服+SLA |
| **API访问** | ❌ | ✅ | ✅ | ✅+SDK |

### 2.2 增值服务单项定价

| 服务项 | 单价 | 说明 |
|--------|------|------|
| 额外隧道 | ¥5/月/个 | 按需扩展 |
| 自定义域名 | ¥10/月/个 | 含DNS托管 |
| 带宽升级包 | ¥20/月/50Mbps | 临时升级 |
| 流量补充包 | ¥10/10GB | 按用量计费 |
| 专属中继节点 | ¥50/月 | 降低延迟 |

### 2.3 计费模型实现建议

```go
// 计费配置结构
type TunnelBillingConfig struct {
    Tier          string    // free, pro, premium, enterprise
    MaxTunnels    int       // 最大隧道数
    BandwidthMbps int       // 带宽上限
    MonthlyGB     int       // 月流量上限
    CustomDomains int       // 自定义域名数
    RelayHours    int       // 中继时长(小时)
    Price         float64   // 月费(元)
}

// 用量统计
type TunnelUsage struct {
    TunnelID      string
    BytesIn       uint64    // 入流量
    BytesOut      uint64    // 出流量
    BandwidthPeak uint64    // 峰值带宽
    RelaySeconds  uint64    // 中继时长
    PeriodStart   time.Time
    PeriodEnd     time.Time
}
```

---

## 三、存储成本趋势预测算法

### 3.1 现有实现分析

nas-os 已在 `internal/reports/capacity_planning.go` 实现三种预测模型：

| 模型 | 适用场景 | 算法 |
|------|----------|------|
| 线性(linear) | 稳定增长 | `y = a*x + b` |
| 指数(exponential) | 快速增长 | `y = a * e^(bx)` |
| 对数(logarithmic) | 增长趋缓 | `y = a * ln(x) + b` |

### 3.2 增强算法 v2

**新增季节性调整**:

```go
// 季节性预测增强
type SeasonalForecast struct {
    BaseGrowth     float64   // 基础增长率
    SeasonFactor   []float64 // 季节因子(12个月)
    TrendDecay     float64   // 趋势衰减系数
}

// 计算季节因子
func CalculateSeasonalFactors(history []CapacityHistory) []float64 {
    factors := make([]float64, 12)
    // 按月份分组计算平均增长率
    monthlyAvg := groupByMonth(history)
    overallAvg := mean(monthlyAvg)
    
    for i := 0; i < 12; i++ {
        factors[i] = monthlyAvg[i] / overallAvg
    }
    return factors
}
```

**成本换算公式**:

```
预测成本 = 预测容量(GB) × 存储单价(¥/GB/月)
         + 带宽成本(流量 × ¥/GB传输)
         + 备份成本(数据量 × ¥/GB备份存储)
```

### 3.3 成本预警阈值

| 阈值类型 | 触发条件 | 建议行动 |
|----------|----------|----------|
| 预算预警 | 月成本 > 预算80% | 审查增长来源 |
| 预算超标 | 月成本 > 预算100% | 启动优化措施 |
| 增长异常 | 增长率 > 历史2倍 | 紧急评估 |
| 预测超标 | 90天后超预算 | 提前扩容/优化 |

---

## 四、成本优化建议报告v2

### 4.1 存储成本优化矩阵

| 策略 | 节省潜力 | 实施难度 | 适用场景 |
|------|----------|----------|----------|
| 数据压缩 | 20-40% | 中 | 文档、日志、代码 |
| 数据去重 | 10-30% | 高 | 备份、相似文件 |
| 冷数据归档 | 30-50% | 低 | >90天未访问 |
| SSD缓存优化 | 提升性能 | 中 | 热数据识别 |
| 快照策略调整 | 10-20% | 低 | 频率优化 |
| 云存储分层 | 50-70% | 中 | 归档数据 |

### 4.2 内网穿透成本优化

| 方案 | 成本节省 | 技术要点 |
|------|----------|----------|
| P2P优先路由 | 100%（无中继） | ICE候选者优先P2P |
| Cloudflare Tunnel替代 | 100% | 零成本替代自建服务器 |
| 公共STUN服务 | ¥0 | Google/Cloudflare免费STUN |
| 本地信令服务器 | 降低延迟 | 自建WebSocket信令 |
| 带宽智能调度 | 提升利用率 | 按负载动态分配 |

### 4.3 成本优化报告模板

```go
// 成本优化报告结构
type CostOptimizationReport struct {
    ID              string
    GeneratedAt     time.Time
    CurrentCost     CostBreakdown      // 当前成本明细
    ProjectedCost   CostProjection     // 预测成本
    Optimizations   []OptimizationItem // 优化建议列表
    PotentialSavings float64           // 预期节省
    ROIAnalysis     ROIResult          // 投入产出分析
}

// 成本明细
type CostBreakdown struct {
    StorageCost      float64 // 存储成本
    BandwidthCost    float64 // 带宽成本
    TunnelRelayCost  float64 // 穿透中继成本
    ComputeCost      float64 // 计算资源
    BackupCost       float64 // 备份成本
    Total            float64 // 总成本
}
```

### 4.4 自动化成本控制建议

| 功能 | 实现 | 效果 |
|------|------|------|
| 自动冷数据迁移 | tiering模块已有 | 释放SSD空间 |
| 带宽动态限速 | 根据用量自动调整 | 避免超额费用 |
| 快照自动清理 | 按保留策略执行 | 减少冗余存储 |
| 成本异常告警 | 监控模块集成 | 及时干预 |
| 月度成本报告 | 定时生成推送 | 持续关注 |

---

## 五、与飞牛FN Connect差异化对比

| 特性 | nas-os | FN Connect | 优势方 |
|------|--------|------------|--------|
| 技术架构 | 开源(STUN/TURN/ICE) | 闭源 | nas-os |
| 免费额度 | 3隧道/5Mbps/50GB | 基础穿透 | 相当 |
| 自建选项 | ✅ 可完全自建 | ❌ 依赖官方 | nas-os |
| Cloudflare集成 | ✅ 零成本方案 | ❌ | nas-os |
| 成本透明 | ✅ 详细预测 | ❌ | nas-os |
| 存储优化建议 | ✅ 自动生成 | ❌ | nas-os |

---

## 六、下一步工作

- [ ] TunnelBilling模块实现（计费统计逻辑）
- [ ] SeasonalForecast季节性预测算法集成
- [ ] CostOptimizationReport自动化报告生成
- [ ] 成本异常告警与Dashboard集成
- [ ] 用户用量Dashboard UI设计

---

**状态**: 🟢 方案完成，待实现
**负责人**: 户部尚书