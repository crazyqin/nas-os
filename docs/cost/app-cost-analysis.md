# 应用成本分析框架

> 版本: v2.90.0 | 户部 - 财务运营

## 1. 概述

本文档定义 NAS-OS 系统中应用资源成本分析的标准框架，为运营决策提供数据支撑。

## 2. 成本分析框架

### 2.1 核心模型

```
总成本 = CPU成本 + 内存成本 + 存储成本 + 网络成本 + 固定成本
```

### 2.2 成本分类

| 成本类型 | 计量单位 | 计费周期 | 说明 |
|---------|---------|---------|------|
| CPU | 核心 | 小时 | 按使用量计费 |
| 内存 | GB | 小时 | 按分配量计费 |
| 存储 | GB | 月 | 按占用量计费 |
| 网络出流量 | GB | 累计 | 按实际流量计费 |
| 网络入流量 | GB | 累计 | 按实际流量计费 |

## 3. 资源定价模型

### 3.1 基础定价

#### 3.1.1 CPU定价

```yaml
pricing:
  cpu:
    unit: core
    period: hour
    base_rate: 0.05    # CNY/core/hour
    tiers:
      - range: [0, 2]
        rate: 0.05
      - range: [2, 8]
        rate: 0.04     # 批量折扣
      - range: [8, null]
        rate: 0.03     # 大量折扣
```

**计算公式**:
```
CPU成本 = Σ(核心数 × 使用小时 × 单价)
```

#### 3.1.2 内存定价

```yaml
pricing:
  memory:
    unit: GB
    period: hour
    base_rate: 0.02    # CNY/GB/hour
    tiers:
      - range: [0, 4]
        rate: 0.02
      - range: [4, 16]
        rate: 0.018
      - range: [16, null]
        rate: 0.015
```

**计算公式**:
```
内存成本 = Σ(内存GB × 使用小时 × 单价)
```

#### 3.1.3 存储定价

```yaml
pricing:
  storage:
    unit: GB
    period: month
    tiers:
      # SSD高性能存储
      ssd:
        rate: 0.50     # CNY/GB/month
      # HDD标准存储
      hdd:
        rate: 0.15     # CNY/GB/month
      # 归档存储
      archive:
        rate: 0.05     # CNY/GB/month
```

**计算公式**:
```
存储成本 = Σ(存储GB × 月单价 × 使用月数)
```

#### 3.1.4 网络定价

```yaml
pricing:
  network:
    outbound:
      rate: 0.80      # CNY/GB
    inbound:
      rate: 0.20      # CNY/GB
    free_quota:
      outbound: 100   # GB/月 免费额度
      inbound: 500    # GB/月 免费额度
```

**计算公式**:
```
网络成本 = Σ((出流量 - 免费额度) × 出流量单价 + 入流量 × 入流量单价)
```

### 3.2 阶梯定价模型

阶梯定价鼓励资源优化，大量使用时单价降低。

```
应用资源量级 → 适用阶梯 → 单价折扣
```

| 资源量级 | CPU折扣 | 内存折扣 | 存储折扣 |
|---------|--------|--------|--------|
| 小型 (<2核心, <4GB) | 100% | 100% | 100% |
| 中型 (2-8核心, 4-16GB) | 80% | 90% | 90% |
| 大型 (8-32核心, 16-64GB) | 60% | 75% | 80% |
| 超大型 (>32核心, >64GB) | 50% | 60% | 70% |

### 3.3 预留资源定价

对于长期稳定使用的应用，可采用预留定价获得额外折扣。

```yaml
reserved:
  discount_schedule:
    1_year: 30%       # 预留1年折扣
    3_year: 50%       # 预留3年折扣
  commitment_ratio: 0.8  # 最低使用率承诺
```

## 4. 应用成本分析方法

### 4.1 成本归集

**原则**: 每个应用独立核算，清晰追溯成本来源。

```go
// 成本归集结构
type AppCostAllocation struct {
    AppID          string
    DirectCosts    DirectCosts     // 直接成本
    SharedCosts    SharedCosts     // 共享成本分摊
    OverheadCosts  OverheadCosts   // 管理成本分摊
    TotalCost      float64         // 总成本
}

// 直接成本
type DirectCosts struct {
    CPU        float64  // 应用独占CPU成本
    Memory     float64  // 应用独占内存成本
    Storage    float64  // 应用独占存储成本
    Network    float64  // 应用独占网络成本
}

// 共享成本分摊
type SharedCosts struct {
    SharedStorage    float64  // 共享存储按比例分摊
    SharedNetwork    float64  // 共享网络带宽分摊
    SharedCompute    float64  // 共享计算资源分摊
    AllocationRatio  float64  // 分摊比例
}
```

### 4.2 成本效率指标

| 指标 | 定义 | 目标值 |
|-----|-----|-------|
| CPU利用率 | 实际使用/分配量 | >60% |
| 内存利用率 | 实际使用/分配量 | >70% |
| 存储利用率 | 实际占用/分配量 | >80% |
| 成本效率 | 产出价值/总成本 | 应用相关 |

### 4.3 成本趋势分析

```
短期趋势 (7天) → 监控异常波动
中期趋势 (30天) → 评估优化效果
长期趋势 (90天) → 规划资源配置
```

## 5. 成本优化策略

### 5.1 自动缩放

基于资源使用自动调整配置：

```
CPU低 (<30%持续1小时) → 触发缩容评估
CPU高 (>80%持续10分钟) → 触发扩容
内存接近上限 (>90%) → 立即扩容
```

### 5.2 存储分层

根据访问频率自动迁移存储：

| 访问频率 | 存储类型 | 成本比例 |
|---------|---------|--------|
| 高频 (>100次/天) | SSD | 100% |
| 中频 (10-100次/天) | HDD | 30% |
| 低频 (<10次/天) | Archive | 10% |

### 5.3 网络优化

- **流量压缩**: 减少传输量
- **本地缓存**: 减少重复传输
- **时段调度**: 利用免费时段

## 6. 成本报告

### 6.1 报告类型

| 报告类型 | 频率 | 内容 |
|---------|-----|-----|
| 实时监控 | 1分钟 | 当前资源使用 |
| 日报 | 每天 | 当日成本汇总 |
| 周报 | 每周 | 成本趋势分析 |
| 月报 | 每月 | 成本结算与建议 |

### 6.2 关键报告字段

```json
{
  "report_type": "daily",
  "period": "2024-03-01",
  "applications": [
    {
      "app_id": "app-001",
      "app_name": "Nextcloud",
      "cost_breakdown": {
        "cpu": 12.5,
        "memory": 8.0,
        "storage": 15.0,
        "network": 3.2
      },
      "total_cost": 38.7,
      "efficiency": {
        "cpu_util": 45,
        "memory_util": 72,
        "storage_util": 85
      },
      "trend": {
        "cost_change": -5.2,  // 与昨日对比
        "usage_change": -3.1
      },
      "alerts": ["cpu_utilization_low"],
      "recommendations": [
        {
          "type": "scale_down",
          "savings_estimate": 6.0,
          "confidence": "high"
        }
      ]
    }
  ],
  "summary": {
    "total_cost": 387.0,
    "top_cost_apps": ["app-001", "app-002", "app-003"],
    "optimization_potential": 45.0
  }
}
```

## 7. 集成与实施

### 7.1 数据采集集成

应用成本分析依赖以下数据源：

1. **Docker Stats**: 容器资源使用
2. **cAdvisor**: 容器监控
3. **Prometheus**: 指标聚合
4. **系统监控**: 主机资源

### 7.2 API接口

```go
// 成本查询API
GET /api/v1/cost/apps/{app_id}?period=daily

// 成本报告API  
GET /api/v1/cost/reports?type=monthly

// 成本优化建议API
GET /api/v1/cost/recommendations/{app_id}

// 成本趋势API
GET /api/v1/cost/trends/{app_id}?days=30
```

### 7.3 告警规则

```yaml
alerts:
  - name: cost_spike
    condition: cost_increase > 20%
    duration: 1h
    severity: warning
    
  - name: resource_waste
    condition: cpu_util < 10% OR memory_util < 20%
    duration: 24h
    severity: info
    
  - name: budget_exceed
    condition: monthly_cost > budget_limit
    severity: critical
```

## 8. 参考定价

### 8.1 云服务商对比 (CNY)

| 资源类型 | AWS | Azure | 阿里云 | 自建 |
|---------|-----|-------|-------|-----|
| 1核心/小时 | 0.07 | 0.06 | 0.05 | 0.03 |
| 1GB内存/小时 | 0.02 | 0.02 | 0.02 | 0.01 |
| 1GB SSD/月 | 0.80 | 0.75 | 0.50 | 0.30 |
| 1GB出流量 | 0.90 | 0.85 | 0.80 | 0.00 |

### 8.2 自建优势

自建NAS系统的成本优势主要体现在：

1. **硬件摊销**: 长期使用成本递减
2. **无流量费**: 本地网络无出流量成本
3. **存储自由**: 无存储容量限制
4. **定制优化**: 可按需优化配置

## 9. 版本历史

| 版本 | 日期 | 变更 |
|-----|-----|-----|
| v2.90.0 | 2024-03-28 | 初始版本，定义成本分析框架 |

---

**户部出品** - 为NAS-OS提供精细化成本管理