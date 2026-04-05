# RAIDZ Expansion 成本分析报告

> **版本**: v1.0.0  
> **日期**: 2026-04-06  
> **作者**: 户部（财务运营）

---

## 1. 项目统计概览

### 1.1 代码规模

| 指标 | 数值 |
|------|------|
| **当前版本** | v2.408.0 |
| **Go源文件** | 1,205 个 |
| **Go代码行数** | 669,924 行 |
| **总代码行数** | 1,643,459 行 |
| **文档文件** | 752 个 |

### 1.2 代码分布

| 类型 | 文件数 | 占比 |
|------|--------|------|
| Go源代码 | 1,205 | 主要 |
| 文档(md/txt/rst/pdf) | 752 | 次要 |
| 其他(js/ts/py/sh/html/css/json/yaml) | 若干 | 辅助 |

---

## 2. RAIDZ Expansion 容量计算器 API

### 2.1 API 设计

```
POST /api/v1/storage/raidz/calculate
```

**请求体**：
```json
{
  "current_disks": 4,           // 当前磁盘数
  "disk_size_tb": 8,            // 单盘容量(TB)
  "raid_level": "raidz2",       // RAID级别: raidz1/raidz2/raidz3
  "expansion_disks": 1,         // 扩展磁盘数
  "data_written_tb": 12          // 已写入数据量(TB)
}
```

**响应体**：
```json
{
  "current": {
    "total_capacity_tb": 32,
    "usable_capacity_tb": 16,
    "used_capacity_tb": 12,
    "free_capacity_tb": 4,
    "efficiency_percent": 50.0
  },
  "expanded": {
    "total_capacity_tb": 40,
    "usable_capacity_tb": 24,
    "usable_gain_tb": 8,
    "efficiency_percent": 60.0
  },
  "headroom_loss": {
    "reported_capacity_tb": 16,
    "actual_capacity_tb": 24,
    "loss_percent": 33.3,
    "recoverable": true
  },
  "time_estimate": {
    "data_to_rewrite_tb": 12,
    "speed_mbps": 200,
    "estimated_hours": 16.7,
    "recommended_window": "夜间低负载时段"
  }
}
```

### 2.2 核心算法

```go
// 容量计算
func CalculateCapacity(disks, parity int, diskSizeTB float64) float64 {
    return float64(disks-parity) * diskSizeTB
}

// 扩展后容量（含折损）
func CalculateExpandedCapacity(oldDisks, newDisks, parity int, diskSizeTB float64) CapacityResult {
    oldCapacity := CalculateCapacity(oldDisks, parity, diskSizeTB)
    newCapacity := CalculateCapacity(newDisks, parity, diskSizeTB)
    
    // 旧数据块保持旧奇偶比，新数据块使用新奇偶比
    headroomLoss := (newCapacity - oldCapacity) * 0.25 // 估算折损
    
    return CapacityResult{
        ReportedCapacity: oldCapacity,        // 报告容量（保守）
        ActualCapacity:   newCapacity,        // 实际可用容量
        HeadroomLoss:     headroomLoss,        // 折损量
    }
}

// 扩展时间预估
func EstimateExpansionTime(dataTB, speedMbps float64) time.Duration {
    dataSizeBytes := dataTB * 1024 * 1024 * 1024 * 1024
    speedBytesPerSec := speedMbps * 1024 * 1024 / 8
    seconds := dataSizeBytes / speedBytesPerSec
    return time.Duration(seconds) * time.Second
}
```

---

## 3. 扩容容量对比分析

### 3.1 典型场景计算

#### 场景A：4盘RAIDZ2扩容至5盘

| 指标 | 扩容前 | 扩容后 | 变化 |
|------|--------|--------|------|
| 磁盘数 | 4 | 5 | +1 |
| 单盘容量 | 8TB | 8TB | - |
| 总容量 | 32TB | 40TB | +8TB |
| 可用容量 | 16TB (2×8) | 24TB (3×8) | +8TB |
| 报告容量 | 16TB | 16TB | 0 |
| 实际可用 | 16TB | 20-24TB | +4~8TB |
| 效率 | 50% | 60% | +10% |
| **折损率** | - | 33% | - |

#### 场景B：5盘RAIDZ2扩容至6盘

| 指标 | 扩容前 | 扩容后 | 变化 |
|------|--------|--------|------|
| 磁盘数 | 5 | 6 | +1 |
| 单盘容量 | 8TB | 8TB | - |
| 总容量 | 40TB | 48TB | +8TB |
| 可用容量 | 24TB (3×8) | 32TB (4×8) | +8TB |
| 报告容量 | 24TB | 24TB | 0 |
| 实际可用 | 24TB | 28-32TB | +4~8TB |
| 效率 | 60% | 66.7% | +6.7% |
| **折损率** | - | 25% | - |

#### 场景C：6盘RAIDZ2扩容至7盘

| 指标 | 扩容前 | 扩容后 | 变化 |
|------|--------|--------|------|
| 磁盘数 | 6 | 7 | +1 |
| 单盘容量 | 8TB | 8TB | - |
| 总容量 | 48TB | 56TB | +8TB |
| 可用容量 | 32TB (4×8) | 40TB (5×8) | +8TB |
| 报告容量 | 32TB | 32TB | 0 |
| 实际可用 | 32TB | 36-40TB | +4~8TB |
| 效率 | 66.7% | 71.4% | +4.7% |
| **折损率** | - | 20% | - |

### 3.2 容量折损曲线

```
折损率随磁盘数变化:

磁盘数:    4→5    5→6    6→7    7→8    8→9
折损率:   33%    25%    20%    17%    14%

规律：折损率 = 1/(新磁盘数 - 奇偶盘数)
      RAIDZ2: 折损率 ≈ 1/(N-2)
```

**结论**：磁盘基数越大，扩展后折损率越低。

---

## 4. 扩展时间预估

### 4.1 时间计算模型

```
扩展时间 = 已写入数据量 / 重写速度

重写速度影响因素：
- HDD：100-250 MB/s（典型 150 MB/s）
- SSD：300-600 MB/s（典型 400 MB/s）
- NVMe：500-3000 MB/s（典型 1500 MB/s）
```

### 4.2 典型场景时间预估

#### HDD场景（平均150MB/s）

| 已写入数据 | 预估时间 | 建议窗口 |
|------------|-----------|----------|
| 1TB | 1.8小时 | 任意时段 |
| 5TB | 9.3小时 | 夜间 |
| 10TB | 18.5小时 | 周末 |
| 20TB | 37小时 | 周末+夜间 |
| 50TB | 93小时 | 规划停机 |

#### SSD场景（平均400MB/s）

| 已写入数据 | 预估时间 | 建议窗口 |
|------------|-----------|----------|
| 1TB | 42分钟 | 任意时段 |
| 5TB | 3.5小时 | 夜间 |
| 10TB | 7小时 | 夜间 |
| 20TB | 14小时 | 周末 |
| 50TB | 35小时 | 规划停机 |

#### NVMe场景（平均1500MB/s）

| 已写入数据 | 预估时间 | 建议窗口 |
|------------|-----------|----------|
| 1TB | 11分钟 | 任意时段 |
| 5TB | 56分钟 | 任意时段 |
| 10TB | 1.9小时 | 任意时段 |
| 20TB | 3.7小时 | 夜间 |
| 50TB | 9.3小时 | 夜间 |

### 4.3 时间估算API

```go
// 智能时间预估
func EstimateExpansionTimeWithFactors(dataTB, diskCount float64, isSSD bool) TimeEstimate {
    var baseSpeedMBps float64
    
    if isSSD {
        baseSpeedMBps = 400 // SSD默认
    } else {
        baseSpeedMBps = 150 // HDD默认
    }
    
    // 多盘并行加速（最多+50%）
    parallelFactor := 1.0 + math.Min(diskCount/10, 0.5)
    effectiveSpeed := baseSpeedMBps * parallelFactor
    
    hours := dataTB * 1024 * 1024 / (effectiveSpeed * 3600 / 8)
    
    return TimeEstimate{
        EstimatedHours:   hours,
        SpeedMBps:        effectiveSpeed,
        RecommendedStart: getOptimalStartWindow(hours),
        CanRunDuringDay: hours < 4,
    }
}
```

---

## 5. 成本效益分析

### 5.1 扩容方案成本对比

#### 场景：从24TB可用容量扩容到32TB

| 方案 | 硬件成本 | 时间成本 | 数据风险 | 推荐度 |
|------|----------|----------|----------|--------|
| **RAIDZ Expansion** | 1×8TB盘 | 5-20小时 | 低 | ⭐⭐⭐⭐⭐ |
| 添加新VDEV | 5×8TB盘 | 即时 | 低 | ⭐⭐⭐ |
| 更换大容量盘 | 5×16TB盘 | 50+小时 | 中 | ⭐⭐ |

#### 成本计算

```
方案对比（目标：+8TB可用容量）

1. RAIDZ Expansion:
   成本 = 1 × 8TB盘价格 ≈ ¥800-1200
   时间 = 5-20小时
   复杂度 = 低

2. 添加新VDEV (RAIDZ2):
   成本 = 5 × 8TB盘价格 ≈ ¥4000-6000
   时间 = 即时生效
   复杂度 = 低

3. 更换大容量盘:
   成本 = 5 × 16TB盘价格 - 回收旧盘 ≈ ¥8000-12000
   时间 = 50-100小时（逐盘替换）
   复杂度 = 高
```

### 5.2 ROI分析

**RAIDZ Expansion投资回报率**：

```
ROI = (节省成本 / 投入成本) × 100%

假设场景：
- 目标：+8TB可用容量
- RAIDZ Expansion成本：¥1000（单盘）
- 传统扩容成本：¥5000（5盘RAIDZ2）

ROI = ((5000 - 1000) / 1000) × 100% = 400%

结论：RAIDZ Expansion可节省80%硬件成本
```

---

## 6. nas-os实现建议

### 6.1 功能优先级

| 功能 | 优先级 | 复杂度 | 预计工时 |
|------|--------|--------|----------|
| 容量计算器API | P0 | 低 | 4h |
| 扩容预览UI | P0 | 中 | 8h |
| 时间预估模块 | P1 | 低 | 2h |
| 折损分析报告 | P1 | 中 | 4h |
| 扩容进度监控 | P0 | 高 | 12h |
| 智能调度建议 | P2 | 中 | 6h |

### 6.2 API端点规划

```
# 已有
GET  /api/v1/storage/pools              # 列出存储池
GET  /api/v1/storage/pools/:id          # 存储池详情

# 新增
POST /api/v1/storage/raidz/calculate    # 容量计算
POST /api/v1/storage/raidz/estimate     # 时间预估
POST /api/v1/storage/raidz/expand        # 执行扩展
GET  /api/v1/storage/raidz/expand/status # 扩展进度
POST /api/v1/storage/raidz/expand/cancel # 取消扩展(不可用)
```

### 6.3 UI设计要点

```
┌─────────────────────────────────────────────────────────┐
│  存储池扩容向导                                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  当前配置:                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │ RAIDZ2 | 4×8TB | 可用: 16TB | 已用: 12TB (75%) │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  扩容方案:                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │ ○ 添加单盘 (推荐)                                │   │
│  │   └ 8TB盘 → 可用增至 20-24TB                     │   │
│  │   耗时约: 5-10小时                               │   │
│  │   成本: ¥800-1200                                │   │
│  │                                                   │   │
│  │ ○ 添加新VDEV                                     │   │
│  │   └ 4×8TB盘组 → 可用增至 32TB                    │   │
│  │   耗时约: 即时                                   │   │
│  │   成本: ¥4000-6000                               │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ⚠️ 注意: 扩容为不可逆操作，请确认后继续               │
│                                                         │
│         [上一步]  [取消]  [开始扩容]                   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 7. 风险与缓解措施

### 7.1 风险矩阵

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 扩展中磁盘故障 | 低 | 高 | 自动暂停→替换→继续 |
| 断电中断 | 中 | 低 | 自动从断点恢复 |
| 性能下降 | 高 | 低 | 选择业务低峰期 |
| 容量折损 | 高 | 中 | 自然恢复/主动重写 |
| 操作失误 | 低 | 高 | 多重确认机制 |

### 7.2 故障处理流程

```
扩展中断处理:

1. 磁盘故障:
   zpool status → 检测故障
   → 扩展自动暂停
   → zpool replace 替换磁盘
   → resilver 完成
   → 扩展自动继续

2. 系统重启:
   扩展状态持久化
   → 重启后自动继续
   → 无需手动干预

3. 用户取消:
   ⚠️ 不支持取消
   → 必须等待完成
```

---

## 8. 总结

### 8.1 核心数据

| 指标 | 数值 |
|------|------|
| 项目版本 | v2.408.0 |
| Go源文件 | 1,205个 |
| Go代码行数 | 669,924行 |
| 总代码行数 | 1,643,459行 |
| 文档文件 | 752个 |

### 8.2 RAIDZ Expansion成本优势

| 指标 | 传统扩容 | RAIDZ Expansion | 节省 |
|------|----------|-----------------|------|
| 硬件成本 | ¥4,000-6,000 | ¥800-1,200 | 80% |
| 时间成本 | 即时 | 5-20小时 | - |
| 操作复杂度 | 低 | 低 | - |
| 数据风险 | 低 | 低 | - |

### 8.3 建议

1. **优先实现容量计算器API**（P0，4h工时）
2. **设计直观的扩容预览UI**（P0，8h工时）
3. **实现扩容进度实时监控**（P0，12h工时）
4. **提供智能时间预估**（P1，2h工时）

---

**报告完成 | 户部（财务运营） | 2026-04-06**