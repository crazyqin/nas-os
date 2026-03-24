# 存储分层效率报告设计方案

## 1. 概述

### 1.1 目标
基于 `pkg/storage/tiering` 和 `internal/tiering` 模块，设计存储分层效率报告，帮助管理员：
- 了解各存储层使用效率
- 评估分层策略效果
- 优化存储成本

### 1.2 参考设计
参考群晖 DSM 7.3 Storage Tiering 报告风格，采用简洁实用的设计。

---

## 2. 统计指标体系

### 2.1 存储层核心指标

| 指标名称 | 说明 | 单位 | 采集频率 |
|---------|------|------|---------|
| `tier_capacity` | 存储层总容量 | GB | 实时 |
| `tier_used` | 已使用容量 | GB | 实时 |
| `tier_available` | 可用容量 | GB | 实时 |
| `tier_usage_percent` | 使用率 | % | 实时 |
| `tier_file_count` | 文件数量 | 个 | 实时 |
| `tier_io_read_bytes` | 读取字节数 | GB | 累计 |
| `tier_io_write_bytes` | 写入字节数 | GB | 累计 |

### 2.2 分层效率指标

| 指标名称 | 说明 | 计算公式 |
|---------|------|---------|
| `hot_data_ratio` | 热数据占比 | 热数据大小 / 总数据大小 × 100% |
| `warm_data_ratio` | 温数据占比 | 温数据大小 / 总数据大小 × 100% |
| `cold_data_ratio` | 冷数据占比 | 冷数据大小 / 总数据大小 × 100% |
| `tiering_efficiency` | 分层效率评分 | 综合加权计算 (0-100) |
| `cache_hit_rate` | SSD缓存命中率 | SSD命中次数 / 总访问次数 × 100% |
| `migration_success_rate` | 迁移成功率 | 成功迁移数 / 总迁移数 × 100% |

### 2.3 迁移统计指标

| 指标名称 | 说明 | 单位 |
|---------|------|------|
| `migrations_total` | 总迁移任务数 | 个 |
| `migrations_running` | 运行中任务数 | 个 |
| `migrations_completed` | 已完成任务数 | 个 |
| `migrations_failed` | 失败任务数 | 个 |
| `migration_throughput` | 迁移吞吐量 | MB/s |
| `avg_migration_time` | 平均迁移时间 | 秒 |

### 2.4 成本相关指标

| 指标名称 | 说明 | 单位 |
|---------|------|------|
| `storage_cost_per_gb` | 每GB存储成本 | 元/GB/月 |
| `hot_storage_cost` | 热存储成本 | 元/月 |
| `cold_storage_cost` | 冷存储成本 | 元/月 |
| `potential_savings` | 潜在节省 | 元/月 |
| `cost_efficiency_score` | 成本效率评分 | 0-100 |

---

## 3. 报告模板设计

### 3.1 概览报告 (Dashboard)

```
┌─────────────────────────────────────────────────────────────────┐
│                    存储分层效率报告                               │
│                    2026-03-25 01:36                              │
├─────────────────────────────────────────────────────────────────┤
│  总览                                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   SSD 层    │  │   HDD 层    │  │   云存储    │              │
│  │  ████████░░ │  │  █████░░░░░ │  │  ███░░░░░░░ │              │
│  │   800GB     │  │   2.5TB     │  │   1.2TB     │              │
│  │   使用率 80% │  │   使用率 50% │  │   使用率 30% │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│                                                                  │
│  分层效率评分: 85/100  ████████████████░░░░ 优秀                  │
│  SSD 缓存命中率: 92%   ████████████████████░ 极佳                 │
├─────────────────────────────────────────────────────────────────┤
│  数据分布                                                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ 热数据 ████████████████████░░░░░░░░ 60% (1.8TB)           │   │
│  │ 温数据 ████████░░░░░░░░░░░░░░░░░░ 20% (600GB)             │   │
│  │ 冷数据 ██████░░░░░░░░░░░░░░░░░░░░ 20% (600GB)             │   │
│  └──────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│  近24小时迁移                                                    │
│  │ 完成: 156  运行: 2  失败: 0  吞吐: 45 MB/s                   │
│  │ 热数据提升: 12GB  冷数据下沉: 45GB                           │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 存储层详情报告

```markdown
# 存储层详情报告

## SSD 缓存层
| 指标 | 数值 |
|------|------|
| 总容量 | 1 TB |
| 已使用 | 800 GB (80%) |
| 可用 | 200 GB |
| 文件数 | 125,000 |
| 热文件 | 100,000 (80%) |
| 温文件 | 20,000 (16%) |
| 冷文件 | 5,000 (4%) |
| 日读取 | 450 GB |
| 日写入 | 120 GB |

### I/O 性能
- 平均读延迟: 0.5 ms
- 平均写延迟: 0.8 ms
- IOPS: 50,000

### 建议
- ⚠️ 使用率超过80%，建议扩容或下沉冷数据
- ✓ 缓存命中率优秀 (92%)

---

## HDD 存储层
| 指标 | 数值 |
|------|------|
| 总容量 | 10 TB |
| 已使用 | 5 TB (50%) |
| 可用 | 5 TB |
| 文件数 | 500,000 |
| 热文件 | 50,000 (10%) |
| 温文件 | 350,000 (70%) |
| 冷文件 | 100,000 (20%) |

### 建议
- ✓ 使用率适中
- 💡 可考虑将冷数据迁移至云存储
```

### 3.3 迁移历史报告

```markdown
# 迁移历史报告

## 概览
- 时间范围: 2026-03-01 ~ 2026-03-25
- 总迁移任务: 3,456
- 成功: 3,420 (99%)
- 失败: 36 (1%)

## 迁移趋势
```
任务数
  │
150│        ██
120│     ██ ██
 90│  ██ ██ ██
 60│██ ██ ██ ██
 30│██ ██ ██ ██
   └──────────────
    03-01 03-08 03-15 03-22
```

## 迁移详情
| 时间 | 源层 | 目标层 | 文件数 | 大小 | 状态 |
|------|------|--------|-------|------|------|
| 03-25 01:00 | SSD | HDD | 1,200 | 45 GB | 完成 |
| 03-24 02:00 | HDD | Cloud | 500 | 120 GB | 完成 |
| 03-23 02:00 | SSD | HDD | 800 | 32 GB | 完成 |

## 失败任务
| 任务ID | 时间 | 原因 | 处理建议 |
|--------|------|------|---------|
| T-20260322-001 | 03-22 02:15 | 文件被占用 | 稍后重试 |
```

### 3.4 成本分析报告

```markdown
# 成本分析报告

## 月度成本概览
| 存储层 | 容量 | 单价 | 月成本 | 占比 |
|--------|------|------|--------|------|
| SSD | 1 TB | ¥50/GB | ¥500 | 60% |
| HDD | 10 TB | ¥5/GB | ¥500 | 30% |
| 云存储 | 3 TB | ¥1/GB | ¥30 | 10% |
| **合计** | - | - | **¥1,030** | 100% |

## 成本优化建议
| 建议 | 潜在节省 | 实施难度 |
|------|---------|---------|
| 下沉SSD冷数据到HDD | ¥150/月 | 低 |
| 开启HDD压缩 | ¥80/月 | 中 |
| 归档冷数据到云 | ¥100/月 | 低 |

## 成本趋势
```
成本(元)
  │
1200│                    ●
1100│              ● ●
1000│        ● ●
 900│  ● ●
   └────────────────────
     01月 02月 03月 04月
```
```

---

## 4. 可视化方案

### 4.1 图表类型

| 图表 | 用途 | 数据源 |
|------|------|--------|
| 环形图 | 存储层容量分布 | TierStats |
| 堆叠柱状图 | 数据冷热分布 | AccessStats |
| 折线图 | 迁移趋势 | MigrationMetrics |
| 仪表盘 | 效率评分 | MetricsSummary |
| 热力图 | 访问频率分布 | FileAccessRecord |

### 4.2 实时监控面板

```
┌─────────────────────────────────────────────────────────────┐
│ 实时监控                                    刷新: 5s        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  迁移队列                                                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ [====45%====]  正在迁移: /data/videos/2025/...       │   │
│  │ 剩余: 3 个任务  预计完成: 15 分钟                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  IOPS                        吞吐量                         │
│  ┌──────────────┐           ┌──────────────┐              │
│  │  R: ████████ │           │  R: 150 MB/s │              │
│  │  W: ████     │           │  W: 45 MB/s  │              │
│  │  12K/8K      │           │              │              │
│  └──────────────┘           └──────────────┘              │
│                                                             │
│  事件日志                                                    │
│  [INFO]  01:35:22  迁移完成: 120 文件 → HDD                │
│  [INFO]  01:34:15  检测到热数据: /data/projects/           │
│  [WARN]  01:30:00  SSD 使用率 > 80%                        │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 周期报告模板

#### 日报
- 24小时迁移统计
- 存储层使用率变化
- 异常事件汇总

#### 周报
- 7天趋势分析
- 策略执行效果
- 优化建议

#### 月报
- 成本分析
- 容量规划
- 长期趋势

---

## 5. 数据模型

### 5.1 报告数据结构

```go
// TieringReport 分层效率报告
type TieringReport struct {
    ID          string            `json:"id"`
    GeneratedAt time.Time         `json:"generatedAt"`
    Period      ReportPeriod      `json:"period"`

    // 存储层统计
    Tiers       map[TierType]*TierReport `json:"tiers"`

    // 效率指标
    Efficiency  EfficiencyMetrics `json:"efficiency"`

    // 迁移统计
    Migrations  MigrationReport   `json:"migrations"`

    // 成本分析
    Cost        CostReport        `json:"cost"`

    // 建议
    Recommendations []Recommendation `json:"recommendations"`
}

// TierReport 存储层报告
type TierReport struct {
    Type            TierType    `json:"type"`
    Name            string      `json:"name"`
    Capacity        int64       `json:"capacity"`        // bytes
    Used            int64       `json:"used"`
    Available       int64       `json:"available"`
    UsagePercent    float64     `json:"usagePercent"`

    // 文件分布
    TotalFiles      int64       `json:"totalFiles"`
    HotFiles        int64       `json:"hotFiles"`
    WarmFiles       int64       `json:"warmFiles"`
    ColdFiles       int64       `json:"coldFiles"`

    // I/O 统计
    ReadBytes       int64       `json:"readBytes"`
    WriteBytes      int64       `json:"writeBytes"`
    ReadIOPS        float64     `json:"readIOPS"`
    WriteIOPS       float64     `json:"writeIOPS"`

    // 迁移统计
    FilesIn         int64       `json:"filesIn"`
    FilesOut        int64       `json:"filesOut"`
    BytesIn         int64       `json:"bytesIn"`
    BytesOut        int64       `json:"bytesOut"`
}

// EfficiencyMetrics 效率指标
type EfficiencyMetrics struct {
    // 数据分布
    HotDataRatio    float64 `json:"hotDataRatio"`    // %
    WarmDataRatio   float64 `json:"warmDataRatio"`
    ColdDataRatio   float64 `json:"coldDataRatio"`

    // 效率评分
    TieringScore    float64 `json:"tieringScore"`    // 0-100
    CacheHitRate    float64 `json:"cacheHitRate"`    // %

    // 迁移效率
    MigrationSuccessRate float64 `json:"migrationSuccessRate"`
    AvgMigrationTime     float64 `json:"avgMigrationTime"` // seconds
}

// CostReport 成本报告
type CostReport struct {
    MonthlyCost     float64             `json:"monthlyCost"`
    CostPerGB       float64             `json:"costPerGB"`
    CostByTier      map[TierType]float64 `json:"costByTier"`
    PotentialSavings float64            `json:"potentialSavings"`
    CostTrend       []CostDataPoint     `json:"costTrend"`
}

// Recommendation 建议
type Recommendation struct {
    Type        string  `json:"type"`     // capacity, migration, cost
    Priority    string  `json:"priority"` // high, medium, low
    Title       string  `json:"title"`
    Description string  `json:"description"`
    Impact      string  `json:"impact"`   // 预期效果
    Action      string  `json:"action"`   // 操作建议
}
```

### 5.2 报告生成器

```go
// ReportGenerator 报告生成器
type ReportGenerator struct {
    metrics    *Metrics
    analyzer   *CostAnalyzer
    config     ReportConfig
}

// Generate 生成报告
func (g *ReportGenerator) Generate(ctx context.Context, period ReportPeriod) (*TieringReport, error) {
    report := &TieringReport{
        ID:          generateReportID(),
        GeneratedAt: time.Now(),
        Period:      period,
        Tiers:       make(map[TierType]*TierReport),
    }

    // 1. 收集存储层数据
    for tierType, metrics := range g.metrics.GetAllTierMetrics() {
        report.Tiers[tierType] = g.buildTierReport(tierType, metrics)
    }

    // 2. 计算效率指标
    report.Efficiency = g.calculateEfficiency(report.Tiers)

    // 3. 迁移统计
    report.Migrations = g.buildMigrationReport()

    // 4. 成本分析
    report.Cost = g.buildCostReport(report.Tiers)

    // 5. 生成建议
    report.Recommendations = g.generateRecommendations(report)

    return report, nil
}
```

---

## 6. API 设计

### 6.1 报告 API

```
GET /api/v1/tiering/reports
  ?period=daily|weekly|monthly
  &start=2026-03-01
  &end=2026-03-25

Response:
{
  "id": "rpt_202603250136",
  "generatedAt": "2026-03-25T01:36:00Z",
  "period": "daily",
  "tiers": { ... },
  "efficiency": { ... },
  "migrations": { ... },
  "cost": { ... },
  "recommendations": [ ... ]
}
```

### 6.2 实时指标 API

```
GET /api/v1/tiering/metrics

Response:
{
  "tiers": {
    "ssd": { "used": 800, "capacity": 1000, "usagePercent": 80 },
    "hdd": { "used": 5000, "capacity": 10000, "usagePercent": 50 }
  },
  "migrations": {
    "running": 2,
    "queued": 5
  },
  "efficiency": {
    "score": 85,
    "cacheHitRate": 92
  }
}
```

### 6.3 Prometheus 指标

复用现有的 `ExportPrometheus()` 方法，暴露：
- `nas_tier_capacity_bytes`
- `nas_tier_used_bytes`
- `nas_tiering_efficiency_score`
- `nas_tiering_cache_hit_rate`
- `nas_tiering_migrations_total`

---

## 7. 实现计划

### Phase 1: 基础报告 (1周)
- [ ] 实现 TieringReport 数据结构
- [ ] 实现 ReportGenerator
- [ ] 实现 REST API 端点
- [ ] 单元测试

### Phase 2: 可视化 (1周)
- [ ] WebUI 报告页面
- [ ] 实时监控面板
- [ ] 图表组件

### Phase 3: 高级功能 (1周)
- [ ] 成本分析集成
- [ ] 自动化建议引擎
- [ ] 定时报告生成
- [ ] 报告导出 (PDF/CSV)

---

## 8. 附录

### 8.1 效率评分算法

```go
func calculateEfficiencyScore(tiers map[TierType]*TierReport) float64 {
    score := 100.0

    // 1. SSD 使用率评分 (权重 30%)
    if ssd, ok := tiers[TierTypeSSD]; ok {
        if ssd.UsagePercent > 90 {
            score -= 15 // 过满扣分
        } else if ssd.UsagePercent < 50 {
            score -= 10 // 浪费扣分
        }
    }

    // 2. 缓存命中率 (权重 30%)
    hitRate := calculateCacheHitRate(tiers)
    if hitRate < 80 {
        score -= (80 - hitRate) * 0.5
    }

    // 3. 数据分布合理性 (权重 20%)
    // 理想分布: 热60% 温20% 冷20%
    distributionScore := calculateDistributionScore(tiers)
    score -= (100 - distributionScore) * 0.2

    // 4. 迁移成功率 (权重 20%)
    migrationScore := calculateMigrationSuccessScore()
    score -= (100 - migrationScore) * 0.2

    return math.Max(0, math.Min(100, score))
}
```

### 8.2 建议生成规则

```yaml
recommendations:
  - type: capacity
    condition: ssd.usagePercent > 80
    priority: high
    title: "SSD 容量告急"
    description: "SSD 使用率超过80%，建议扩容或下沉冷数据"

  - type: capacity
    condition: ssd.usagePercent < 30
    priority: low
    title: "SSD 利用率低"
    description: "SSD 使用率低于30%，可考虑缩减容量降低成本"

  - type: migration
    condition: coldDataOnSSD > 10GB
    priority: medium
    title: "冷数据下沉"
    description: "检测到SSD上存在冷数据，建议迁移至HDD"

  - type: cost
    condition: potentialSavings > monthlyCost * 0.2
    priority: high
    title: "成本优化机会"
    description: "存在显著成本优化空间，可节省{{savings}}/月"
```