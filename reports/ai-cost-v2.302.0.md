# AI 服务成本追踪报告

**报告日期**: 2026-03-29
**版本**: v2.302.0

---

## 一、GPU 资源消耗统计

### 1.1 资源监控方法

nas-os 通过以下方式监控 GPU 资源：

```go
// GPU 资源统计
type GPUStats struct {
    DeviceID    string
    MemoryUsed  uint64  // 已用显存 (bytes)
    MemoryTotal uint64  // 总显存 (bytes)
    Utilization float64 // GPU 利用率 (%)
    Temperature uint    // 温度 (°C)
    PowerDraw   uint    // 功耗 (W)
}
```

### 1.2 支持的 GPU 类型

| GPU 类型 | 监控支持 | 加速支持 |
|---------|---------|---------|
| NVIDIA | ✅ nvidia-smi | ✅ CUDA |
| Intel | ✅ intel_gpu_top | ✅ QuickSync |
| AMD | ✅ rocm-smi | ✅ ROCm |

### 1.3 GPU 调度策略

```go
// GPU 调度优先级
const (
    PriorityLow    = 0  // 后台任务
    PriorityNormal = 1  // 普通请求
    PriorityHigh   = 2  // 实时交互
)
```

---

## 二、Token 成本计算

### 2.1 计费公式

```
成本 = 输入Token数 × 输入单价 + 输出Token数 × 输出单价
```

### 2.2 各提供商价格参考

| 提供商 | 模型 | 输入价格 (¥/1M tokens) | 输出价格 (¥/1M tokens) |
|-------|------|------------------------|------------------------|
| OpenAI | GPT-4o | ¥70 | ¥210 |
| OpenAI | GPT-4o-mini | ¥1.05 | ¥4.2 |
| Claude | Sonnet | ¥21 | ¥105 |
| 本地 Ollama | 各模型 | ¥0 | ¥0 (硬件成本) |

### 2.3 Token 计数实现

```go
// TokenCounter 实现
type TokenCounter struct {
    InputTokens  int64
    OutputTokens int64
    Model        string
    Provider     string
}

func (tc *TokenCounter) CalculateCost() float64 {
    pricing := GetPricing(tc.Provider, tc.Model)
    inputCost := float64(tc.InputTokens) * pricing.InputPrice / 1_000_000
    outputCost := float64(tc.OutputTokens) * pricing.OutputPrice / 1_000_000
    return inputCost + outputCost
}
```

---

## 三、成本优化建议

### 3.1 使用本地模型

通过 Ollama 部署本地模型，可大幅降低 Token 成本：

| 场景 | 云端模型 | 本地模型 | 节省 |
|------|---------|---------|------|
| 日常对话 | GPT-4o | Qwen2.5 | 100% |
| 代码辅助 | Claude | CodeQwen | 100% |
| 翻译 | GPT-4o | Qwen2.5 | 100% |

### 3.2 缓存策略

- **响应缓存**: 相同请求返回缓存结果
- **上下文复用**: 多轮对话保持上下文
- **批量处理**: 合并小请求为批量请求

### 3.3 模型选择策略

```go
// 根据任务复杂度选择模型
func SelectModel(complexity int) string {
    switch {
    case complexity < 3:
        return "qwen2.5:7b"  // 本地模型
    case complexity < 7:
        return "gpt-4o-mini" // 云端低成本
    default:
        return "gpt-4o"      // 云端高端
    }
}
```

---

## 四、成本分摊机制

### 4.1 多用户场景

| 用户类型 | 配额 | 超额处理 |
|---------|------|---------|
| 管理员 | 无限 | - |
| 普通用户 | 10万 tokens/月 | 降级到本地模型 |
| 访客 | 1万 tokens/月 | 拒绝服务 |

### 4.2 计费统计

```go
// 用户计费记录
type UsageRecord struct {
    UserID      string
    Date        time.Time
    InputTokens int64
    OutputTokens int64
    Cost        float64
    Model       string
}
```

---

## 五、监控与告警

### 5.1 成本监控指标

- 每日 Token 消耗量
- 每日成本金额
- 各用户消耗排行
- 模型使用分布

### 5.2 告警规则

| 指标 | 阈值 | 告警级别 |
|------|------|---------|
| 日成本 | > ¥100 | 警告 |
| 日成本 | > ¥500 | 严重 |
| 单用户日消耗 | > 10万 tokens | 警告 |
| GPU 内存使用 | > 90% | 警告 |

---

## 六、总结

nas-os 的 AI 成本追踪系统提供：

1. **精确计费**: Token 级别的精确统计
2. **成本优化**: 本地模型 + 智能路由
3. **多用户管理**: 配额控制 + 分摊机制
4. **实时监控**: 成本指标 + 告警

通过合理配置，可将 AI 服务成本降低 50% 以上。

---

*报告完成: 2026-03-29*
*报告人: 户部*