# 勒索软件检测模块架构设计

## 概述

本文档描述NAS-OS勒索软件检测模块的架构设计，该模块旨在实时监控文件系统活动，检测并阻止勒索软件攻击。

## 设计目标

1. **实时检测**：毫秒级响应，在文件被加密前拦截
2. **低误报率**：智能算法区分正常操作与恶意行为
3. **零数据丢失**：在检测到攻击时保护已加密文件
4. **可恢复性**：自动备份被修改的文件，支持一键恢复
5. **性能影响最小**：系统开销控制在5%以内

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Ransomware Detection Module                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐        │
│  │   File Event  │  │    Behavior   │  │   Entropy     │        │
│  │    Monitor    │  │    Analyzer   │  │   Analyzer    │        │
│  └───────┬───────┘  └───────┬───────┘  └───────┬───────┘        │
│          │                  │                  │                 │
│          └──────────────────┼──────────────────┘                 │
│                             ▼                                    │
│                   ┌─────────────────┐                            │
│                   │  Threat Engine  │                            │
│                   │   (多因子评分)   │                            │
│                   └────────┬────────┘                            │
│                            │                                     │
│          ┌─────────────────┼─────────────────┐                   │
│          ▼                 ▼                 ▼                   │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐        │
│  │   Alerting    │  │   Isolation   │  │   Recovery    │        │
│  │    Service    │  │    Service    │  │    Service    │        │
│  └───────────────┘  └───────────────┘  └───────────────┘        │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. 文件事件监控器 (File Event Monitor)

**职责**：实时捕获文件系统事件

```go
type FileEvent struct {
    ID          string
    Timestamp   time.Time
    Path        string
    Operation   FileOperation // Create, Modify, Delete, Rename
    ProcessInfo ProcessInfo   // 发起操作的进程信息
    FileSize    int64
    OldPath     string        // 用于重命名操作
    Checksum    string        // 文件哈希
}

type FileEventMonitor struct {
    watchers    map[string]*fsnotify.Watcher
    eventChan   chan FileEvent
    filters     []EventFilter
    rateLimiter *RateLimiter
}

// 配置
type MonitorConfig struct {
    WatchPaths       []string      // 监控路径
    ExcludePaths     []string      // 排除路径
    EventBufferSize  int           // 事件缓冲区大小
    SamplingInterval time.Duration // 采样间隔
}
```

**实现方案**：
- 使用 `fsnotify` 进行文件系统监控
- 结合 `inotify` (Linux) 获取更详细的事件信息
- 使用 eBPF 进行内核级进程跟踪（可选，高性能方案）

### 2. 行为分析器 (Behavior Analyzer)

**职责**：分析文件操作模式，识别勒索软件特征行为

**检测规则**：

```go
type BehaviorRule struct {
    ID          string
    Name        string
    Description string
    Severity    Severity
    Conditions  []Condition
    Weight      float64
}

// 内置规则
var DefaultRules = []BehaviorRule{
    {
        ID:   "rapid_file_modification",
        Name: "快速批量文件修改",
        Conditions: []Condition{
            {Type: "file_count", Threshold: 50, Window: 10 * time.Second},
            {Type: "extension_change", Threshold: 0.8}, // 80%文件扩展名变更
        },
        Weight: 0.8,
    },
    {
        ID:   "suspicious_extension",
        Name: "可疑文件扩展名",
        Conditions: []Condition{
            {Type: "extension_pattern", Patterns: []string{
                ".encrypted", ".locked", ".crypto", ".ransom",
                ".enc", ".cerber", ".locky", ".wannacry",
            }},
        },
        Weight: 0.7,
    },
    {
        ID:   "mass_deletion",
        Name: "批量文件删除",
        Conditions: []Condition{
            {Type: "delete_count", Threshold: 100, Window: 30 * time.Second},
        },
        Weight: 0.9,
    },
    {
        ID:   "shadow_copy_deletion",
        Name: "删除卷影副本",
        Conditions: []Condition{
            {Type: "process_cmdline", Pattern: "vssadmin.*delete"},
        },
        Weight: 1.0, // 确定性攻击指标
    },
    {
        ID:   "directory_traversal",
        Name: "深度目录遍历",
        Conditions: []Condition{
            {Type: "unique_dirs", Threshold: 20, Window: 5 * time.Second},
        },
        Weight: 0.5,
    },
}
```

**行为评分算法**：

```go
type BehaviorAnalyzer struct {
    rules         []BehaviorRule
    eventBuffer   *RingBuffer[FileEvent]
    processTree   *ProcessTree
    scoreCache    *LRUCache[string, float64]
}

func (a *BehaviorAnalyzer) CalculateScore(events []FileEvent) ThreatScore {
    var totalWeight float64
    var weightedScore float64
    
    for _, rule := range a.rules {
        if rule.Match(events) {
            weightedScore += rule.Weight * rule.Severity.Score()
            totalWeight += rule.Weight
        }
    }
    
    if totalWeight == 0 {
        return ThreatScore{Level: ThreatLevelNone, Score: 0}
    }
    
    normalizedScore := weightedScore / totalWeight
    return ThreatScore{
        Level:    scoreToLevel(normalizedScore),
        Score:    normalizedScore,
        MatchedRules: a.getMatchedRules(events),
    }
}
```

### 3. 熵值分析器 (Entropy Analyzer)

**职责**：检测文件内容熵值变化，识别加密行为

```go
type EntropyAnalyzer struct {
    windowSize    int
    baselineCache *LRUCache[string, float64]
}

// 计算香农熵
func (e *EntropyAnalyzer) CalculateEntropy(data []byte) float64 {
    if len(data) == 0 {
        return 0
    }
    
    freq := make(map[byte]float64)
    for _, b := range data {
        freq[b]++
    }
    
    var entropy float64
    size := float64(len(data))
    for _, count := range freq {
        p := count / size
        entropy -= p * math.Log2(p)
    }
    return entropy
}

// 检测文件是否被加密
func (e *EntropyAnalyzer) DetectEncryption(oldEntropy, newEntropy float64) bool {
    // 加密文件的熵值通常接近8.0（最大值）
    // 普通文本文件熵值约为4.0-5.5
    // 压缩文件熵值约为7.0-7.8
    
    entropyIncrease := newEntropy - oldEntropy
    return entropyIncrease > 1.5 && newEntropy > 7.5
}
```

### 4. 威胁引擎 (Threat Engine)

**职责**：综合多因子评分，决策是否触发防护

```go
type ThreatEngine struct {
    analyzers      []Analyzer
    decisionTree   *DecisionTree
    actionPolicy   ActionPolicy
    alertManager   *AlertManager
}

type ThreatScore struct {
    Level        ThreatLevel
    Score        float64
    Confidence   float64
    MatchedRules []string
    ProcessInfo  ProcessInfo
    Timestamp    time.Time
}

type ThreatLevel int

const (
    ThreatLevelNone ThreatLevel = iota
    ThreatLevelLow      // 可疑，持续监控
    ThreatLevelMedium   // 高风险，准备隔离
    ThreatLevelHigh     // 确认攻击，立即隔离
    ThreatLevelCritical // 已造成损害，紧急响应
)

// 决策流程
func (e *ThreatEngine) Evaluate(events []FileEvent) ThreatDecision {
    var scores []ThreatScore
    
    for _, analyzer := range e.analyzers {
        score := analyzer.Analyze(events)
        scores = append(scores, score)
    }
    
    finalScore := e.aggregateScores(scores)
    
    return e.decisionTree.Decide(finalScore)
}
```

### 5. 隔离服务 (Isolation Service)

**职责**：阻止恶意进程，隔离受影响区域

```go
type IsolationService struct {
    processKiller  *ProcessKiller
    networkBlocker *NetworkBlocker
    fileLocker     *FileLocker
}

type IsolationAction struct {
    KillProcesses  []int      // 需要终止的进程ID
    BlockNetwork   bool       // 是否阻止网络访问
    LockPaths      []string   // 需要锁定的路径
    SnapshotBefore bool       // 隔离前创建快照
}

func (s *IsolationService) Isolate(ctx context.Context, decision ThreatDecision) error {
    // 1. 记录隔离前状态
    if decision.Action.SnapshotBefore {
        s.createSnapshot()
    }
    
    // 2. 终止恶意进程
    for _, pid := range decision.Action.KillProcesses {
        if err := s.processKiller.Kill(pid); err != nil {
            log.Warnf("终止进程 %d 失败: %v", pid, err)
        }
    }
    
    // 3. 锁定文件系统
    for _, path := range decision.Action.LockPaths {
        s.fileLocker.Lock(path)
    }
    
    // 4. 可选：阻断网络
    if decision.Action.BlockNetwork {
        s.networkBlocker.BlockAll()
    }
    
    return nil
}
```

### 6. 恢复服务 (Recovery Service)

**职责**：备份被修改的文件，支持数据恢复

```go
type RecoveryService struct {
    backupQueue   *BackupQueue
    storage       BackupStorage
    index         *RecoveryIndex
}

type BackupRecord struct {
    ID           string
    OriginalPath string
    BackupPath   string
    Timestamp    time.Time
    Checksum     string
    ProcessID    int
    Recoverable  bool
}

// 实时备份策略
func (s *RecoveryService) StartRealtimeBackup(eventChan <-chan FileEvent) {
    for event := range eventChan {
        if event.Operation == FileModify || event.Operation == FileCreate {
            // 异步备份原始文件
            go s.backupFile(event)
        }
    }
}

// 恢复文件
func (s *RecoveryService) Recover(backupID string) error {
    record, err := s.index.Get(backupID)
    if err != nil {
        return err
    }
    
    // 从备份恢复
    return s.storage.Restore(record.BackupPath, record.OriginalPath)
}

// 批量恢复
func (s *RecoveryService) RecoverAll(startTime, endTime time.Time) ([]string, error) {
    records := s.index.GetByTimeRange(startTime, endTime)
    
    var recovered []string
    var errors []error
    
    for _, record := range records {
        if err := s.Recover(record.ID); err != nil {
            errors = append(errors, err)
        } else {
            recovered = append(recovered, record.OriginalPath)
        }
    }
    
    return recovered, errors[0]
}
```

### 7. 告警服务 (Alerting Service)

**职责**：通知管理员，记录安全事件

```go
type AlertManager struct {
    notifiers []Notifier
    eventLog  *SecurityEventLog
}

type Alert struct {
    ID          string
    Level       ThreatLevel
    Title       string
    Message     string
    ProcessInfo ProcessInfo
    FileCount   int
    Timestamp   time.Time
    Actions     []string
}

func (m *AlertManager) SendAlert(alert Alert) error {
    // 记录到安全日志
    m.eventLog.Record(alert)
    
    // 发送通知
    for _, notifier := range m.notifiers {
        if err := notifier.Notify(alert); err != nil {
            log.Warnf("通知发送失败: %v", err)
        }
    }
    
    return nil
}

// 通知渠道
type Notifier interface {
    Notify(alert Alert) error
}

type EmailNotifier struct{ /* ... */ }
type WebhookNotifier struct{ /* ... */ }
type PushNotifier struct{ /* ... */ }
type SMSNotifier struct{ /* ... */ }
```

## 数据流

```
文件操作
    │
    ▼
┌─────────────────┐
│  Event Monitor  │ ──► 原始事件
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Event Filter   │ ──► 过滤噪音
└────────┬────────┘
         │
         ├──────────────────┬──────────────────┐
         ▼                  ▼                  ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ Behavior Check  │ │ Entropy Check   │ │ Signature Check │
└────────┬────────┘ └────────┬────────┘ └────────┬────────┘
         │                  │                  │
         └──────────────────┼──────────────────┘
                           ▼
                  ┌─────────────────┐
                  │  Threat Engine  │ ──► 威胁评分
                  └────────┬────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │  Low Risk   │ │ Medium Risk │ │  High Risk  │
    │   记录日志   │ │  加强监控   │ │  立即隔离   │
    └─────────────┘ └─────────────┘ └─────────────┘
```

## 配置示例

```yaml
# configs/ransomware.yaml
ransomware_detection:
  enabled: true
  
  monitor:
    paths:
      - "/data"
      - "/home"
    exclude_paths:
      - "/data/cache"
      - "/data/tmp"
    event_buffer_size: 10000
    sampling_interval: 100ms
    
  behavior:
    rules:
      - id: rapid_file_modification
        enabled: true
        threshold: 50
        window: 10s
        weight: 0.8
        
      - id: suspicious_extension
        enabled: true
        patterns:
          - ".encrypted"
          - ".locked"
          - ".crypto"
        weight: 0.7
        
      - id: mass_deletion
        enabled: true
        threshold: 100
        window: 30s
        weight: 0.9
        
    score_thresholds:
      low: 0.3
      medium: 0.5
      high: 0.7
      critical: 0.9
      
  entropy:
    enabled: true
    sample_size: 4096
    high_entropy_threshold: 7.5
    entropy_increase_threshold: 1.5
    
  recovery:
    enabled: true
    backup_path: "/var/lib/nas-os/ransomware-backup"
    max_backup_size: 10GB
    retention_days: 30
    realtime_backup: true
    
  isolation:
    auto_isolate: true
    kill_process: true
    lock_files: true
    block_network: false  # 谨慎启用
    
  alerting:
    channels:
      - type: email
        recipients:
          - "admin@example.com"
      - type: webhook
        url: "https://hooks.example.com/alert"
      - type: push
        # 系统通知
        
  whitelist:
    processes:
      - "/usr/bin/rsync"
      - "/usr/bin/tar"
      - "/usr/bin/zip"
    paths:
      - "/data/backup"
      - "/data/archives"
```

## API 设计

### REST API

```yaml
# 获取检测状态
GET /api/security/ransomware/status

# 获取威胁事件列表
GET /api/security/ransomware/events?level=high&start=2024-01-01&end=2024-01-31

# 获取单个事件详情
GET /api/security/ransomware/events/:id

# 恢复被隔离的文件
POST /api/security/ransomware/recover
{
  "event_id": "evt_xxx",
  "file_paths": ["/data/document.pdf"]
}

# 添加白名单
POST /api/security/ransomware/whitelist
{
  "type": "process",
  "path": "/usr/bin/myapp"
}

# 更新检测规则
PUT /api/security/ransomware/rules/:rule_id
{
  "enabled": true,
  "threshold": 100
}
```

### WebSocket 实时事件

```javascript
// 连接
ws://nas-ip/api/security/ransomware/stream

// 事件格式
{
  "type": "threat_detected",
  "data": {
    "level": "high",
    "process": {
      "pid": 12345,
      "name": "suspicious_app",
      "path": "/tmp/suspicious_app"
    },
    "affected_files": 127,
    "timestamp": "2024-03-26T10:00:00Z"
  }
}
```

## 性能考虑

### 资源占用

| 组件 | CPU | 内存 | 磁盘 I/O |
|------|-----|------|----------|
| 事件监控 | 1-2% | 50MB | 低 |
| 行为分析 | 2-3% | 100MB | 无 |
| 熵值分析 | 1-2% | 30MB | 中（采样） |
| 实时备份 | 1-2% | 50MB | 高 |
| **总计** | **5-9%** | **~250MB** | **中** |

### 优化策略

1. **事件过滤**：早期过滤系统文件、缓存文件
2. **采样分析**：对大文件采用采样熵值分析
3. **异步处理**：备份和分析异步进行
4. **缓存结果**：缓存已知进程的评分结果
5. **批量操作**：批量处理事件，减少系统调用

## 测试策略

### 单元测试

```go
func TestBehaviorAnalyzer_RapidModification(t *testing.T) {
    analyzer := NewBehaviorAnalyzer(DefaultRules)
    
    // 模拟快速文件修改
    events := generateRapidModifyEvents(100, 5*time.Second)
    
    score := analyzer.CalculateScore(events)
    assert.GreaterOrEqual(t, score.Level, ThreatLevelMedium)
}

func TestEntropyAnalyzer_EncryptedFile(t *testing.T) {
    analyzer := NewEntropyAnalyzer()
    
    original := []byte("This is a normal text file content")
    encrypted := encryptData(original) // 模拟加密
    
    oldEntropy := analyzer.CalculateEntropy(original)
    newEntropy := analyzer.CalculateEntropy(encrypted)
    
    assert.True(t, analyzer.DetectEncryption(oldEntropy, newEntropy))
}
```

### 集成测试

```go
func TestRansomwareDetection_FullWorkflow(t *testing.T) {
    // 启动检测模块
    detector := NewRansomwareDetector(testConfig)
    detector.Start()
    defer detector.Stop()
    
    // 模拟勒索软件行为
    simulateRansomwareActivity(t, "/test/data")
    
    // 验证检测结果
    events := detector.GetRecentEvents()
    assert.NotEmpty(t, events)
    assert.Equal(t, ThreatLevelHigh, events[0].Level)
    
    // 验证隔离效果
    assert.False(t, isProcessRunning("ransomware_simulator"))
}
```

### 压力测试

```go
func BenchmarkEventProcessing(b *testing.B) {
    detector := setupBenchmarkDetector()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        event := generateRandomEvent()
        detector.ProcessEvent(event)
    }
}
```

## 部署建议

### 生产环境

1. **监控范围**：仅监控用户数据目录，排除系统和缓存目录
2. **白名单**：预先配置备份工具、压缩工具白名单
3. **告警渠道**：配置多个告警渠道，确保及时通知
4. **恢复测试**：定期测试恢复流程有效性
5. **容量规划**：备份存储至少为监控数据量的10%

### 测试环境

1. 先在测试环境验证规则有效性
2. 模拟各类勒索软件行为测试检测率
3. 测试误报率，优化规则阈值

## 未来扩展

1. **机器学习模型**：训练行为分类模型，提高检测准确率
2. **威胁情报集成**：接入外部威胁情报源
3. **蜜罐文件**：部署诱饵文件，提前发现攻击
4. **网络流量分析**：检测C&C通信
5. **跨节点协同**：多节点联合检测，提高整体安全性

## 参考资料

- [NIST Ransomware Guide](https://www.nist.gov/publications/nist-sp-800-171-ransomware-risk-management)
- [No More Ransom Project](https://www.nomoreransom.org/)
- [MITRE ATT&CK - Ransomware](https://attack.mitre.org/software/)