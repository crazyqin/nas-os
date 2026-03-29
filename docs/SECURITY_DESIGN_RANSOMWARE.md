# 勒索软件检测安全设计

**版本**: v2.308.0  
**创建日期**: 2026-03-29  
**作者**: 刑部安全审计

---

## 1. 安全架构

### 1.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                      勒索软件检测安全架构                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐             │
│   │ 文件事件监控 │───→│ 行为分析器  │───→│ 威胁评分引擎│             │
│   │ (inotify)   │    │             │    │             │             │
│   └─────────────┘    └─────────────┘    └─────────────┘             │
│          │                │                   │                      │
│          │                │                   │                      │
│          ▼                ▼                   ▼                      │
│   ┌─────────────────────────────────────────────────────┐           │
│   │                    响应执行器                        │           │
│   ├─────────────────────────────────────────────────────┤           │
│   │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐ │           │
│   │  │ 记录日志 │  │ 实时告警 │  │ 文件隔离 │  │ 快照锁定 │ │           │
│   │  └─────────┘  └─────────┘  └─────────┘  └─────────┘ │           │
│   └─────────────────────────────────────────────────────┘           │
│                               │                                      │
│                               ▼                                      │
│                      ┌─────────────┐                                │
│                      │ 审计日志存储 │                                │
│                      └─────────────┘                                │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 检测模块

| 模块 | 技术实现 | 功能描述 |
|------|---------|---------|
| 文件事件监控 | Linux inotify | 实时监控文件系统事件 |
| 行为分析器 | 规则引擎 + ML | 分析文件操作行为模式 |
| 威胁评分引擎 | 多因素加权评分 | 计算威胁风险分数 |
| 响应执行器 | 自动化响应模块 | 执行防御动作 |

---

## 2. 防御机制设计

### 2.1 检测规则体系

#### P0 - 立即检测规则

```go
// 勒索软件特征检测规则
var ransomwareRules = []DetectionRule{
    {
        ID:      "RANSOM_EXT_001",
        Name:    "可疑扩展名检测",
        Pattern: regexp.MustCompile(`(?i)\.(encrypted|locked|crypto|ransom|locked_[a-f0-9]+)$`),
        Action:  ActionIsolateAndAlert,
        Severity: SeverityCritical,
    },
    {
        ID:      "RANSOM_BULK_001",
        Name:    "快速批量修改",
        Condition: "files_modified > 50 in 10s",
        Action:  ActionMonitorAndAlert,
        Severity: SeverityHigh,
    },
    {
        ID:      "RANSOM_DEL_001",
        Name:    "批量删除检测",
        Condition: "files_deleted > 100 in 30s",
        Action:  ActionIsolateAndAlert,
        Severity: SeverityCritical,
    },
    {
        ID:      "RANSOM_SNAPSHOT_001",
        Name:    "快照空间突增",
        Condition: "snapshot_growth > 500%/hour",
        Action:  ActionAlert,
        Severity: SeverityHigh,
    },
}
```

#### P1 - 近期检测规则

```go
var extendedRules = []DetectionRule{
    {
        ID:      "RANSOM_ENTROPY_001",
        Name:    "熵值异常检测",
        Condition: "entropy > 7.5 AND delta > 1.5",
        Action:  ActionDeepScan,
        Severity: SeverityMedium,
    },
    {
        ID:      "RANSOM_SNAPSHOT_DEL_001",
        Name:    "快照删除检测",
        Condition: "snapshot_delete_attempt",
        Action:  ActionBlockAndAlert,
        Severity: SeverityCritical,
    },
    {
        ID:      "RANSOM_DIR_TRAV_001",
        Name:    "目录深度遍历",
        Condition: "dirs_visited > 20 in 5s",
        Action:  ActionMonitor,
        Severity: SeverityMedium,
    },
}
```

### 2.2 威胁评分算法

```go
// ThreatScore 威胁评分计算
type ThreatScore struct {
    BaseScore    float64  // 基础分数 (0-1)
    WeightFactors []WeightFactor
}

func CalculateThreatScore(events []FileEvent) float64 {
    score := 0.0
    
    // 扩展名特征 (权重 0.3)
    if hasRansomwareExtension(events) {
        score += 0.3
    }
    
    // 操作频率 (权重 0.25)
    freq := calculateOperationFrequency(events)
    score += freq * 0.25
    
    // 熵值变化 (权重 0.2)
    entropy := calculateEntropyChange(events)
    score += entropy * 0.2
    
    // 文件类型分布 (权重 0.15)
    typeDist := calculateFileTypeDistribution(events)
    score += typeDist * 0.15
    
    // 时间模式 (权重 0.1)
    timePattern := analyzeTimePattern(events)
    score += timePattern * 0.1
    
    return min(score, 1.0)  // 上限 1.0
}
```

### 2.3 防护响应级别

| 风险级别 | 分数范围 | 响应动作 |
|---------|---------|---------|
| 低风险 | < 0.3 | 记录日志，持续监控 |
| 中风险 | 0.3 - 0.5 | 加强监控，准备隔离 |
| 高风险 | 0.5 - 0.7 | 立即隔离，锁定文件 |
| 紧急 | > 0.7 | 终止进程，锁定快照，全系统告警 |

### 2.4 快照保护机制

```go
// SnapshotProtector 快照保护器
type SnapshotProtector struct {
    protectedSnapshots map[string]bool
    lock               sync.RWMutex
}

// ProtectCriticalSnapshots 保护关键快照
func (sp *SnapshotProtector) ProtectCriticalSnapshots(poolID string) error {
    // 1. 获取最近 7 天的所有快照
    snapshots, err := sp.listRecentSnapshots(poolID, 7*24*time.Hour)
    if err != nil {
        return err
    }
    
    // 2. 标记为保护状态
    sp.lock.Lock()
    for _, snap := range snapshots {
        sp.protectedSnapshots[snap.ID] = true
        // 设置 immutable 属性
        sp.setImmutable(snap.ID)
    }
    sp.lock.Unlock()
    
    // 3. 禁止删除保护快照
    return sp.enableDeleteProtection(poolID)
}

// CheckDeleteAttempt 检查删除尝试
func (sp *SnapshotProtector) CheckDeleteAttempt(snapID string) bool {
    sp.lock.RLock()
    protected := sp.protectedSnapshots[snapID]
    sp.lock.RUnlock()
    
    if protected {
        // 记录告警
        sp.alertSuspiciousDelete(snapID)
        return false  // 拒绝删除
    }
    return true
}
```

---

## 3. 告警策略

### 3.1 告警渠道配置

```yaml
alert_channels:
  - type: email
    recipients: ["admin@example.com", "security@example.com"]
    severity_threshold: medium
    
  - type: webhook
    url: "https://api.example.com/security/alert"
    severity_threshold: high
    
  - type: push
    platform: discord
    channel: "#security-alerts"
    severity_threshold: critical
    
  - type: sms
    recipients: ["+86-xxx-xxxx"]
    severity_threshold: critical
```

### 3.2 告警消息格式

```json
{
  "timestamp": "2026-03-29T12:00:00Z",
  "severity": "critical",
  "type": "ransomware_detected",
  "source": {
    "host": "nas-server",
    "share": "/mnt/data/share1",
    "user": "unknown",
    "ip": "192.168.1.xxx"
  },
  "details": {
    "threat_score": 0.85,
    "detected_rules": ["RANSOM_EXT_001", "RANSOM_BULK_001"],
    "affected_files": 127,
    "suspicious_extensions": [".encrypted", ".locked"]
  },
  "actions_taken": [
    "file_isolation",
    "snapshot_locked",
    "share_blocked"
  ],
  "recommendation": "立即检查并隔离受影响系统"
}
```

### 3.3 告警抑制策略

```go
// AlertSuppressor 告警抑制器
type AlertSuppressor struct {
    recentAlerts map[string]time.Time
    window       time.Duration  // 抑制窗口 (默认 5分钟)
}

func (as *AlertSuppressor) ShouldAlert(alertID string) bool {
    as.lock.Lock()
    defer as.lock.Unlock()
    
    lastAlert, exists := as.recentAlerts[alertID]
    if exists && time.Since(lastAlert) < as.window {
        return false  // 抑制重复告警
    }
    
    as.recentAlerts[alertID] = time.Now()
    return true
}

// 相似告警合并
func (as *AlertSuppressor) MergeSimilarAlerts(alerts []Alert) *Alert {
    if len(alerts) == 0 {
        return nil
    }
    
    merged := alerts[0]
    for i := 1; i < len(alerts); i++ {
        merged.AffectedFiles += alerts[i].AffectedFiles
        merged.Details = mergeDetails(merged.Details, alerts[i].Details)
    }
    merged.IsMerged = true
    merged.MergeCount = len(alerts)
    return merged
}
```

---

## 4. 实现路线图

### 4.1 Phase 1 - 核心检测 (P0)

**时间**: 2026-03-29 ~ 2026-04-05

| 任务 | 状态 | 优先级 |
|------|------|--------|
| 文件事件监控器 (inotify) | 待实现 | P0 |
| 可疑扩展名检测规则 | 待实现 | P0 |
| 批量操作检测规则 | 待实现 | P0 |
| 基础告警通知 | 已有基础 | P0 |
|威胁评分引擎框架 | 待实现 | P0 |

### 4.2 Phase 2 - 增强检测 (P1)

**时间**: 2026-04-06 ~ 2026-04-20

| 任务 | 状态 | 优先级 |
|------|------|--------|
| 熵值分析器 | 待实现 | P1 |
| 快照异常检测 | 待实现 | P1 |
| 快照保护机制 | 待实现 | P1 |
| 文件隔离模块 | 待实现 | P1 |

### 4.3 Phase 3 - 高级功能 (P2)

**时间**: 2026-04-21 ~ 2026-05-15

| 任务 | 状态 | 优先级 |
|------|------|--------|
| ML行为分类 | 待实现 | P2 |
| C&C通信检测 | 待实现 | P2 |
| 蜜罐文件系统 | 待实现 | P2 |
| 自动恢复机制 | 待实现 | P2 |

---

## 5. 测试验证

### 5.1 安全测试用例

```go
// TestRansomwareDetection 测试勒索软件检测
func TestRansomwareDetection(t *testing.T) {
    tests := []struct {
        name     string
        events   []FileEvent
        expected ThreatLevel
    }{
        {
            name: "正常文件操作",
            events: []FileEvent{
                {Type: "modify", Path: "/data/file1.txt", Timestamp: now()},
                {Type: "modify", Path: "/data/file2.txt", Timestamp: now()},
            },
            expected: ThreatLevelLow,
        },
        {
            name: "可疑扩展名批量修改",
            events: []FileEvent{
                {Type: "modify", Path: "/data/file1.encrypted", Timestamp: now()},
                {Type: "modify", Path: "/data/file2.locked", Timestamp: now()},
                // ... 50+ 文件
            },
            expected: ThreatLevelCritical,
        },
        {
            name: "快速批量删除",
            events: generateBulkDeleteEvents(150, 30*time.Second),
            expected: ThreatLevelCritical,
        },
    }
    
    for _, tt := range tests {
        score := CalculateThreatScore(tt.events)
        level := GetThreatLevel(score)
        assert.Equal(t, tt.expected, level)
    }
}
```

---

## 6. 运维指南

### 6.1 监控指标

| 指标 | 说明 | 告警阈值 |
|------|------|---------|
| `ransomware_events_total` | 检测事件总数 | > 100/min |
| `ransomware_threat_score_avg` | 平均威胁分数 | > 0.5 |
| `ransomware_files_isolated` | 已隔离文件数 | > 1000 |
| `ransomware_snapshots_protected` | 保护快照数 | 减少 > 10% |

### 6.2 响应流程

```
检测告警 → 确认威胁级别 → 执行响应动作 → 记录审计 → 事后分析
         │                  │                │              │
         │                  ▼                │              │
         │          ┌───────────────┐        │              │
         │          │ 低风险：监控   │        │              │
         │          │ 中风险：准备   │        │              │
         │          │ 高风险：隔离   │        │              │
         │          │ 紧急：全阻断   │        │              │
         │          └───────────────┘        │              │
         └─────────────────────────────────→│              │
                                        ┌───────────────┐ │
                                        │ 审计日志记录   │ │
                                        └───────────────┘ │
                                                          ▼
                                                   ┌───────────────┐
                                                   │ 安全事件报告   │
                                                   └───────────────┘
```

---

**文档状态**: 已创建  
**下一步**: 实现 Phase 1 核心检测模块