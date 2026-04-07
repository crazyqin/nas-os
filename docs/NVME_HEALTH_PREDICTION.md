# NVMe健康预测模型设计文档

**版本**: v1.0.0
**日期**: 2026-04-07
**编制**: 户部（财务预算与资源管理）

---

## 一、背景与目标

### 1.1 设计背景

NVMe SSD作为高性能存储介质，在企业级NAS系统中承担关键数据存储职责。与传统HDD不同，NVMe SSD的健康状态具有以下特点：

- **写入寿命有限**: TBW（Total Bytes Written）指标决定SSD寿命
- **温度敏感**: 高温加速NAND Flash磨损
- **告警滞后**: 传统SMART告警往往在严重故障前才触发

群晖Active Insight作为业界标杆，提供了以下启示：
- 提前预测而非被动告警
- 多维度数据融合分析
- 基于历史趋势的寿命预测

### 1.2 设计目标

| 目标 | 说明 | 优先级 |
|------|------|--------|
| 寿命预测 | 提前30天预测SSD寿命耗尽 | P0 |
| 异常检测 | 检测异常写入模式 | P1 |
| 告警分级 | 按紧急程度分级告警 | P0 |
| 数据融合 | 融合SMART、温度、I/O数据 | P1 |

---

## 二、群晖Active Insight监控原理分析

### 2.1 核心监控指标

群晖Active Insight主要监控以下指标：

```
┌─────────────────────────────────────────────────────────┐
│                  Synology Active Insight                  │
├─────────────────────────────────────────────────────────┤
│  1. SMART健康状态                                        │
│     - Reallocated Sectors (重映射扇区)                   │
│     - Pending Sectors (待定扇区)                         │
│     - Media Errors (媒体错误)                            │
│                                                          │
│  2. NVMe特定指标                                         │
│     - Percentage Used (寿命已用百分比)                   │
│     - Available Spare (可用备用空间)                     │
│     - Temperature (温度)                                 │
│     - Power On Hours (通电时间)                          │
│                                                          │
│  3. 性能指标                                             │
│     - I/O延迟异常                                        │
│     - 写入吞吐量变化                                     │
│                                                          │
│  4. 历史趋势                                             │
│     - 寿命消耗速率                                       │
│     - 写入模式分析                                       │
└─────────────────────────────────────────────────────────┘
```

### 2.2 告警机制设计

群晖采用多级告警策略：

| 级别 | 触发条件 | 响应建议 |
|------|----------|----------|
| Normal | 寿命<80%，温度<60°C | 正常使用 |
| Warning | 寿命80-90%，温度60-70°C | 关注，准备替换 |
| Critical | 寿命90-95%，温度70-80°C | 尽快替换 |
| Emergency | 寿命>95%，媒体错误>10 | 立即替换 |

### 2.3 预测算法核心

群晖的寿命预测基于：

1. **写入速率趋势**: 基于历史写入数据计算日均写入量
2. **TBW容量估算**: 根据SSD规格估算总写入容量
3. **剩余寿命计算**: `剩余天数 = (TBW - 已写入) / 日均写入`

---

## 三、NAS-OS NVMe健康预测框架设计

### 3.1 架构总览

```
┌────────────────────────────────────────────────────────────────┐
│                    NAS-OS NVMe健康预测框架                       │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ SMART采集   │  │ 温度监控    │  │ I/O统计     │             │
│  │ (smartctl)  │  │ (nvme-cli)  │  │ (iostat)    │             │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘             │
│         │                │                │                     │
│         └────────────────┼────────────────┘                     │
│                          ▼                                      │
│                  ┌───────────────┐                              │
│                  │  数据融合层   │                              │
│                  │ HealthFusion │                              │
│                  └───────┬───────┘                              │
│                          ▼                                      │
│         ┌────────────────┴────────────────┐                    │
│         │                                  │                    │
│  ┌──────▼──────┐  ┌──────────▼──────────┐  │                    │
│  │ 寿命预测器  │  │ 异常检测器          │  │                    │
│  │ LifePredict │  │ AnomalyDetector     │  │                    │
│  └──────┬──────┘  └───────┬──────────────┘  │                    │
│         │                 │                 │                    │
│         └─────────────────┼─────────────────┘                    │
│                           ▼                                      │
│                  ┌───────────────┐                              │
│                  │  告警引擎    │                              │
│                  │ AlertEngine  │                              │
│                  └───────┬───────┘                              │
│                          ▼                                      │
│                  ┌───────────────┐                              │
│                  │  API/通知    │                              │
│                  └───────────────┘                              │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### 3.2 SMART数据分析模块

#### 3.2.1 NVMe SMART关键属性

| 属性ID | 名称 | 说明 | 权重 |
|--------|------|------|------|
| - | Percentage Used | 寿命已用百分比 | 40% |
| - | Available Spare | 可用备用空间 | 30% |
| - | Temperature | 当前温度 | 15% |
| - | Media Errors | 媒体错误数 | 10% |
| - | Critical Warning | 严重警告标志 | 5% |

#### 3.2.2 SATA SSD SMART属性映射

| 属性ID | 名称 | 用途 |
|--------|------|------|
| 5 | Reallocated Sector Ct | 坏块计数 |
| 177 | Wear Leveling Count | 磨损平衡计数(Samsung) |
| 202 | Percent Lifetime Remain | 剩余寿命(Crucial/Micron) |
| 231 | SSD Life Left | SSD寿命(Intel) |
| 233 | Media Wearout Indicator | 媒体磨损(Intel) |
| 241/245 | Total LBAs Written | 总写入量 |

### 3.3 寿命预测算法

#### 3.3.1 基础算法: 写入速率投影法

```go
// PredictedLife 寿命预测结果
type PredictedLife struct {
    RemainingDays    int       // 预计剩余天数
    EstimatedEndDate time.Time // 预计失效日期
    WriteRatePerDay  uint64    // 日均写入量(字节)
    Confidence       float64   // 置信度(0-1)
    Method           string    // 预测方法
}

// 计算公式:
// 日均写入量 = Σ(历史写入增量) / 观测天数
// 剩余写入容量 = TBW - 已写入总量
// 剩余天数 = 剩余写入容量 / 日均写入量
// 置信度 = min(观测样本数/1440, 1.0) * 0.7
```

#### 3.3.2 增强算法: 多因子加权预测

```go
// HealthScore 健康评分计算
func CalculateHealthScore(health *SSDHealth) float64 {
    score := 100.0
    
    // 1. 寿命因子 (权重40%)
    lifeScore := (100 - health.LifeUsedPercent) * 0.4
    
    // 2. 备用空间因子 (权重30%)
    spareScore := health.AvailableSpare * 0.3
    
    // 3. 温度因子 (权重15%)
    tempScore := calculateTempScore(health.Temperature) * 0.15
    
    // 4. 媒体错误因子 (权重10%)
    errorScore := calculateErrorScore(health.MediaErrors) * 0.10
    
    // 5. 警告标志因子 (权重5%)
    warningScore := (1 - float64(health.CriticalWarning)/255) * 0.05
    
    return lifeScore + spareScore + tempScore + errorScore + warningScore
}

// 温度评分: 60°C以下满分，超过则递减
func calculateTempScore(temp int) float64 {
    if temp <= 60 {
        return 100
    }
    // 60-80°C线性递减，80°C以上为0
    return max(0, 100 - (temp - 60) * 5)
}

// 错误评分: 无错误满分，错误越多递减
func calculateErrorScore(errors uint64) float64 {
    if errors == 0 {
        return 100
    }
    // 10个错误为0分
    return max(0, 100 - float64(errors) * 10)
}
```

#### 3.3.3 预测置信度模型

```go
// ConfidenceModel 置信度计算
type ConfidenceModel struct {
    MinSamples    int     // 最小样本数 (默认10)
    OptimalSamples int    // 最优样本数 (默认1440 = 30天)
    MaxConfidence float64 // 最大置信度 (默认0.85)
}

func CalculateConfidence(historyCount int, model *ConfidenceModel) float64 {
    if historyCount < model.MinSamples {
        return 0 // 样本不足，不提供预测
    }
    
    ratio := float64(historyCount) / float64(model.OptimalSamples)
    if ratio > 1 {
        ratio = 1
    }
    
    return ratio * model.MaxConfidence
}
```

### 3.4 告警阈值设计

#### 3.4.1 分级告警阈值

| 指标 | 正常 | 警告 | 严重 | 紧急 |
|------|------|------|------|------|
| 寿命已用 | <80% | 80-90% | 90-95% | >95% |
| 备用空间 | >20% | 10-20% | 5-10% | <5% |
| 温度 | <60°C | 60-70°C | 70-80°C | >80°C |
| 媒体错误 | 0 | 1-5 | 5-10 | >10 |
| 剩余天数 | >180 | 90-180 | 30-90 | <30 |

#### 3.4.2 动态阈值调整

```go
// DynamicThreshold 动态阈值配置
type DynamicThreshold struct {
    BaseThreshold    float64 // 基础阈值
    AdjustmentFactor float64 // 调整因子
    HistoryWindow    int     // 历史窗口(天)
}

// 根据历史趋势动态调整阈值
// 如果写入速率突然增加，提前触发警告
func AdjustThreshold(base float64, writeRateChange float64) float64 {
    // 写入速率增加50%，阈值降低10%
    if writeRateChange > 1.5 {
        return base * 0.9
    }
    return base
}
```

### 3.5 异常检测模块

#### 3.5.1 写入模式异常检测

```go
// WritePatternAnalyzer 写入模式分析器
type WritePatternAnalyzer struct {
    BaselineWriteRate  uint64  // 基线写入速率
    AnomalyThreshold   float64 // 异常阈值(默认3倍)
    SpikeWindow        int     // 突发窗口(分钟)
}

// 检测写入异常
func (a *WritePatternAnalyzer) DetectAnomaly(currentRate uint64) AnomalyType {
    ratio := float64(currentRate) / float64(a.BaselineWriteRate)
    
    switch {
    case ratio > 10:
        return AnomalySevere // 严重异常: 10倍以上
    case ratio > 5:
        return AnomalyHigh   // 高异常: 5-10倍
    case ratio > 3:
        return AnomalyMedium // 中异常: 3-5倍
    default:
        return AnomalyNone
    }
}
```

#### 3.5.2 温度异常检测

```go
// TemperatureMonitor 温度监控
type TemperatureMonitor struct {
    NormalRange    Range   // 正常范围(50-60°C)
    WarningRange   Range   // 警告范围(60-70°C)
    CriticalRange  Range   // 严重范围(70-80°C)
    EmergencyRange Range   // 紧急范围(>80°C)
}

// 检测温度异常并触发冷却建议
func (m *TemperatureMonitor) CheckTemperature(temp int) TemperatureStatus {
    if temp > 80 {
        return TemperatureEmergency
    }
    if temp > 70 {
        return TemperatureCritical
    }
    if temp > 60 {
        return TemperatureWarning
    }
    return TemperatureNormal
}
```

---

## 四、数据采集与存储设计

### 4.1 数据采集频率

| 数据类型 | 采集频率 | 存储策略 |
|----------|----------|----------|
| SMART数据 | 30分钟 | 保留90天 |
| 温度数据 | 5分钟 | 保留30天 |
| I/O统计 | 1分钟 | 保留7天 |
| 告警记录 | 实时 | 持久化 |

### 4.2 数据存储结构

```go
// NVMeHealthRecord 健康数据记录
type NVMeHealthRecord struct {
    ID              string    `json:"id"`
    Device          string    `json:"device"`
    Timestamp       time.Time `json:"timestamp"`
    
    // SMART数据
    PercentUsed     float64   `json:"percentUsed"`
    AvailableSpare  float64   `json:"availableSpare"`
    Temperature     int       `json:"temperature"`
    MediaErrors     uint64    `json:"mediaErrors"`
    PowerOnHours    uint64    `json:"powerOnHours"`
    
    // I/O数据
    ReadBytes       uint64    `json:"readBytes"`
    WriteBytes      uint64    `json:"writeBytes"`
    IOLatency       float64   `json:"ioLatency"` // ms
    
    // 预测数据
    HealthScore     float64   `json:"healthScore"`
    PredictedDays   int       `json:"predictedDays,omitempty"`
    Confidence      float64   `json:"confidence,omitempty"`
}
```

### 4.3 历史数据压缩

```go
// 数据压缩策略:
// - 7天内: 原始数据保留
// - 7-30天: 每小时聚合
// - 30-90天: 每天聚合
// - 90天后: 只保留关键事件

func AggregateOldData(records []NVMeHealthRecord, window time.Duration) AggregatedRecord {
    return AggregatedRecord{
        AvgHealthScore: calculateAverage(records),
        MaxTemperature: findMax(records),
        WriteRateTrend: calculateTrend(records),
        AnomalyCount:   countAnomalies(records),
    }
}
```

---

## 五、API设计

### 5.1 RESTful API

```yaml
# NVMe健康监控API

# 获取所有NVMe设备健康状态
GET /api/v1/nvme/health
Response: {
  "devices": [
    {
      "device": "/dev/nvme0n1",
      "healthPercent": 95.0,
      "lifeUsedPercent": 5.0,
      "temperature": 45,
      "status": "healthy",
      "predictedLife": {
        "remainingDays": 365,
        "confidence": 0.85
      }
    }
  ],
  "summary": {
    "total": 1,
    "healthy": 1,
    "warning": 0,
    "critical": 0
  }
}

# 获取单个设备详细健康信息
GET /api/v1/nvme/health/{device}
Response: NVMeHealthDetail

# 获取设备历史数据
GET /api/v1/nvme/health/{device}/history?days=30
Response: []NVMeHealthRecord

# 获取设备寿命预测
GET /api/v1/nvme/health/{device}/prediction
Response: PredictedLife

# 获取告警列表
GET /api/v1/nvme/alerts?status=active
Response: []NVMeAlert
```

### 5.2 Prometheus指标导出

```go
// ExportMetrics 导出Prometheus格式指标
func ExportMetrics(devices []*SSDHealth) string {
    metrics := ""
    for _, d := range devices {
        deviceLabel := sanitizeDeviceName(d.Device)
        
        metrics += fmt.Sprintf(
            "nvme_health_percent{device=\"%s\"} %.1f\n",
            deviceLabel, d.HealthPercent)
        metrics += fmt.Sprintf(
            "nvme_life_used_percent{device=\"%s\"} %.1f\n",
            deviceLabel, d.LifeUsedPercent)
        metrics += fmt.Sprintf(
            "nvme_temperature_celsius{device=\"%s\"} %d\n",
            deviceLabel, d.Temperature)
        metrics += fmt.Sprintf(
            "nvme_available_spare_percent{device=\"%s\"} %.1f\n",
            deviceLabel, d.AvailableSpare)
        metrics += fmt.Sprintf(
            "nvme_media_errors{device=\"%s\"} %d\n",
            deviceLabel, d.MediaErrors)
        metrics += fmt.Sprintf(
            "nvme_power_on_hours{device=\"%s\"} %d\n",
            deviceLabel, d.PowerOnHours)
        metrics += fmt.Sprintf(
            "nvme_predicted_remaining_days{device=\"%s\"} %d\n",
            deviceLabel, d.PredictedLife.RemainingDays)
    }
    return metrics
}
```

---

## 六、告警通知设计

### 6.1 通知渠道

| 渠道 | 用途 | 配置 |
|------|------|------|
| Webhook | 外部系统集成 | URL + Secret |
| Email | 管理员通知 | SMTP配置 |
| 系统通知 | 桌面/Web UI | 内置通知 |
| Discord | 即时通讯 | Webhook URL |

### 6.2 告警消息模板

```go
// AlertMessage 告警消息
type AlertMessage struct {
    Device      string    `json:"device"`
    Severity    string    `json:"severity"` // warning/critical/emergency
    Type        string    `json:"type"`     // lifespan/temperature/media_error
    Message     string    `json:"message"`
    Value       string    `json:"value"`
    PredictedDays int     `json:"predictedDays,omitempty"`
    Timestamp   time.Time `json:"timestamp"`
    Action      string    `json:"action"` // 建议操作
}

// 示例告警消息
{
    "device": "/dev/nvme0n1",
    "severity": "warning",
    "type": "lifespan",
    "message": "NVMe寿命已使用85%，预计剩余120天",
    "value": "85%",
    "predictedDays": 120,
    "timestamp": "2026-04-07T21:00:00Z",
    "action": "建议准备替换硬盘，当前可继续使用"
}
```

---

## 七、与现有系统集成

### 7.1 与NAS-OS现有模块集成

```go
// 现有模块:
// - internal/hardware/nvme/monitor.go (基础NVMe监控)
// - internal/monitoring/ssd_health.go (SSD健康监控)
// - internal/storage/smart/smart-flexible.go (SMART监控)
// - internal/hardware/smart/cron.go (SMART定时任务)

// 集成方案:
// 1. 扩展 internal/hardware/nvme/monitor.go
//    - 添加寿命预测功能
//    - 添加历史数据存储
//
// 2. 增强 internal/monitoring/ssd_health.go
//    - 添加多因子健康评分
//    - 添加动态阈值调整
//
// 3. 新增 internal/hardware/smart/nvme/predictor.go
//    - 寿命预测核心算法
//    - 异常检测模块
//
// 4. 新增 internal/hardware/smart/nvme/alert.go
//    - 分级告警引擎
//    - 多渠道通知
```

### 7.2 配置文件设计

```yaml
# nvme-health-prediction.yaml

monitoring:
  enabled: true
  checkInterval: 30m
  historyRetention: 90d
  
prediction:
  enabled: true
  minSamples: 10
  optimalSamples: 1440
  maxConfidence: 0.85
  
alerts:
  thresholds:
    lifespan:
      warning: 80
      critical: 90
      emergency: 95
    temperature:
      warning: 60
      critical: 70
      emergency: 80
    spare:
      warning: 20
      critical: 10
      emergency: 5
    remainingDays:
      warning: 180
      critical: 90
      emergency: 30
      
  channels:
    - type: webhook
      url: https://example.com/alert
    - type: email
      to: admin@example.com
```

---

## 八、实施路线图

### 8.1 Phase 1: 基础增强 (Week 1-2)

- [ ] 扩展NVMe监控数据采集
- [ ] 实现基础寿命预测算法
- [ ] 设计数据存储结构

### 8.2 Phase 2: 预测引擎 (Week 3-4)

- [ ] 实现多因子健康评分
- [ ] 实现异常检测模块
- [ ] 验证预测准确度

### 8.3 Phase 3: 告警系统 (Week 5-6)

- [ ] 实现分级告警引擎
- [ ] 集成多渠道通知
- [ ] 设计告警消息模板

### 8.4 Phase 4: API与集成 (Week 7-8)

- [ ] 实现RESTful API
- [ ] 导出Prometheus指标
- [ ] 与现有系统集成

---

## 九、附录

### A. 参考文献

1. NVMe Specification 1.4 - SMART Health Information
2. Synology Active Insight Documentation
3. Intel SSD SMART Attributes Reference
4. Samsung SSD SMART Attributes Guide

### B. 测试用例

```go
// TestLifePrediction 寿命预测测试
func TestLifePrediction(t *testing.T) {
    history := createMockHistory(30) // 30天历史数据
    
    predictor := NewLifePredictor(DefaultPredictionConfig())
    prediction := predictor.Predict(history)
    
    assert.Greater(t, prediction.RemainingDays, 0)
    assert.LessOrEqual(t, prediction.Confidence, 1.0)
    assert.NotEmpty(t, prediction.Method)
}

// TestAlertThreshold 告警阈值测试
func TestAlertThreshold(t *testing.T) {
    health := &SSDHealth{
        LifeUsedPercent: 85,
        Temperature:     65,
        AvailableSpare:  15,
    }
    
    alert := EvaluateAlertLevel(health, DefaultThresholds())
    assert.Equal(t, AlertLevelWarning, alert.Level)
    assert.Contains(t, alert.Message, "85%")
}
```

---

**文档编制**: 户部
**审核日期**: 2026-04-07
**版本**: v1.0.0