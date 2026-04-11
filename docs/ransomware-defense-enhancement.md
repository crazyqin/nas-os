# Ransomware Defense Enhancement Design

## 概述

本文档基于TrueNAS SCALE 26的Ransomware Defense特性，设计nas-os勒索软件防护模块的增强方案。

## TrueNAS 26 Ransomware Defense核心特性分析

### 1. SMB/NFS实时监控
- **TrueNAS实现**: 通过内核级文件系统监控，实时捕获SMB/NFS协议层面的文件操作
- **nas-os现状**: 已有FileMonitor实现轮询式监控，但缺乏协议级深度监控

### 2. 多检测方法

| 检测方法 | TrueNAS实现 | nas-os现状 | 差距 |
|---------|------------|-----------|------|
| 蜜罐文件 | 随机部署诱饵文件，监控访问/修改/删除 | honeyfile.go已实现完整功能 | ✅ 已满足 |
| 行为分析 | 分析文件操作模式（批量重命名、快速修改） | behavior_monitor.go有8种模式 | ✅ 已满足 |
| 加密签名 | 特征库匹配已知勒索软件扩展名/勒索信 | signature_db.go已实现 | ✅ 已满足 |
| 快照对比 | 比对快照大小变化，检测突增 | snapshot_anomaly.go已实现 | ✅ 已满足 |
| 熵值检测 | 检测高熵值文件（加密特征） | enhanced_detectors.go已实现 | ✅ 已满足 |

### 3. 自动响应机制

| 响应动作 | TrueNAS实现 | nas-os现状 | 差距 |
|---------|------------|-----------|------|
| 禁用共享 | 立即禁用受影响的SMB/NFS共享 | 无实现 | ❌ 需增强 |
| 只读模式 | 将共享设置为只读 | 无实现 | ❌ 需增强 |
| 限制访问 | 基于IP/用户限制访问 | 无IP级控制 | ❌ 需增强 |
| 暂停快照删除 | 锁定快照防止删除 | 部分实现 | ⚠️ 需增强 |
| IP阻断 | 自动阻断恶意IP | 无实现 | ❌ 需增强 |

### 4. 其他特性

| 特性 | TrueNAS实现 | nas-os现状 | 差距 |
|-----|------------|-----------|------|
| 威胁评分 | 综合评分系统（0-100） | 部分实现，缺乏综合评分 | ⚠️ 需增强 |
| 快照恢复工具 | 一键恢复界面 | RestoreSnapshot已实现 | ✅ 已满足 |
| 通知系统 | 多渠道告警 | alert_manager.go已实现 | ✅ 已满足 |

---

## 增强设计方案

### Phase 1: 协议级监控增强（优先级：高）

#### 1.1 SMB/NFS深度监控模块

**新增文件**: `internal/security/ransomware/protocol_monitor.go`

```go
// ProtocolMonitor 协议级监控器
type ProtocolMonitor struct {
    smbMonitor  *SMBActivityMonitor
    nfsMonitor  *NFSActivityMonitor
    ftpMonitor  *FTPActivityMonitor  // 可扩展
}

// SMBActivityMonitor SMB协议活动监控
type SMBActivityMonitor struct {
    config      SMBMonitorConfig
    connections map[string]*SMBConnection  // clientIP -> connection
    activities  chan SMBActivityEvent
}

// SMBActivityEvent SMB活动事件
type SMBActivityEvent struct {
    Timestamp    time.Time
    ClientIP     string
    ShareName    string
    Operation    string    // create, read, write, delete, rename
    FilePath     string
    FileSize     int64
    ProcessName  string    // SMB进程名
    UserID       string
    SessionID    string
}
```

**关键功能**:
1. 实时捕获SMB连接信息（客户端IP、用户、共享）
2. 监控每个SMB会话的文件操作频率
3. 识别异常SMB行为模式（如短时间内大量写操作）
4. 支持基于客户端IP的访问控制

#### 1.2 网络连接追踪

**新增文件**: `internal/security/ransomware/network_tracker.go`

```go
// NetworkConnectionTracker 网络连接追踪器
type NetworkConnectionTracker struct {
    connections    map[string]*NetworkConnection  // IP -> connection
    suspiciousIPs  map[string]*SuspiciousIPRecord
    ipBlocklist    map[string]bool
    whitelist      map[string]bool
}

// NetworkConnection 网络连接记录
type NetworkConnection struct {
    IPAddress      string
    ConnectionType string    // smb, nfs, ftp, webdav, ssh
    FirstSeen      time.Time
    LastActive     time.Time
    FileOps        int64     // 文件操作计数
    WriteOps       int64
    DeleteOps      int64
    RenameOps      int64
    SuspicionScore int
    Blocked        bool
    BlockReason    string
}

// SuspiciousIPRecord 可疑IP记录
type SuspiciousIPRecord struct {
    IP            string
    FirstDetected time.Time
    ThreatLevel   ThreatLevel
    IncidentCount int
    Incidents     []IPIncident
    AutoBlocked   bool
}
```

---

### Phase 2: 自动响应增强（优先级：高）

#### 2.1 共享锁定系统

**新增文件**: `internal/security/ransomware/share_lockdown.go`

```go
// ShareLockdownManager 共享锁定管理器
type ShareLockdownManager struct {
    shares       map[string]*ShareStatus  // shareName -> status
    lockPolicy   LockdownPolicy
    lockHistory  []LockdownRecord
}

// ShareStatus 共享状态
type ShareStatus struct {
    Name          string
    Type          string    // smb, nfs, ftp
    Path          string
    Status        ShareLockStatus  // active, readonly, locked, disabled
    LockReason    string
    LockedAt      *time.Time
    LockedBy      string
    AllowedIPs    []string  // 锁定后允许访问的IP
    BlockedIPs    []string
    AutoLocked    bool
}

// ShareLockStatus 共享锁定状态
type ShareLockStatus string
const (
    ShareActive    ShareLockStatus = "active"
    ShareReadOnly  ShareLockStatus = "readonly"
    ShareLocked    ShareLockStatus = "locked"    // 只有管理员可访问
    ShareDisabled  ShareLockStatus = "disabled" // 完全禁用
)

// LockdownPolicy 锁定策略
type LockdownPolicy struct {
    AutoLockOnCritical     bool    // 检测到critical威胁自动锁定
    AutoLockOnHigh         bool    // 检测到high威胁自动锁定
    LockDuration           time.Duration  // 自动解锁时间
    AllowAdminOverride     bool
    NotifyOnLock           bool
    PreserveConnections    bool    // 锁定时保持现有连接（但限制操作）
}

// LockdownAction 锁定动作
type LockdownAction struct {
    Type          LockdownActionType
    TargetShare   string
    TargetIP      string
    Reason        string
    Duration      time.Duration
    TriggeredBy   string    // detection_id or manual
}

// LockdownActionType 锁定动作类型
type LockdownActionType string
const (
    ActionSetReadOnly      LockdownActionType = "set_readonly"
    ActionLockShare        LockdownActionType = "lock_share"
    ActionDisableShare     LockdownActionType = "disable_share"
    ActionBlockIP          LockdownActionType = "block_ip"
    ActionBlockUser        LockdownActionType = "block_user"
    ActionRestrictAccess   LockdownActionType = "restrict_access"
)
```

**关键功能**:
1. 检测到威胁时自动锁定受影响的共享
2. 支持多种锁定级别（只读、锁定、禁用）
3. IP级别的访问控制
4. 定时自动解锁机制
5. 锁定历史记录和审计

#### 2.2 IP阻断系统

**新增文件**: `internal/security/ransomware/ip_blocker.go`

```go
// IPBlocker IP阻断器
type IPBlocker struct {
    blocklist      map[string]*BlockedIP
    whitelist      map[string]bool
    tempBlocks     map[string]*TemporaryBlock
    firewallAdapter FirewallAdapterInterface
}

// BlockedIP 已阻断IP
type BlockedIP struct {
    IP            string
    BlockedAt     time.Time
    BlockReason   string
    ThreatLevel   ThreatLevel
    DetectionID   string
    ExpiresAt     *time.Time    // 临时阻断的过期时间
    AutoBlocked   bool
    ManualBlock   bool
    UnblockCount  int           // 曾被解封次数
}

// TemporaryBlock 临时阻断
type TemporaryBlock struct {
    IP          string
    BlockedAt   time.Time
    ExpiresAt   time.Time
    Reason      string
    TriggeredBy string
}

// FirewallAdapterInterface 防火墙适配器接口
type FirewallAdapterInterface interface {
    BlockIP(ip string, reason string) error
    UnblockIP(ip string) error
    ListBlockedIPs() ([]string, error)
    AddRule(rule FirewallRule) error
    RemoveRule(ruleID string) error
}

// FirewallRule 防火墙规则
type FirewallRule struct {
    ID          string
    Type        string    // block, allow, rate-limit
    SourceIP    string
    Port        int
    Protocol    string
    Reason      string
    ExpiresAt   *time.Time
}
```

---

### Phase 3: 综合威胁评分系统（优先级：中）

#### 3.1 多维度威胁评分

**新增文件**: `internal/security/ransomware/threat_scorer.go`

```go
// ThreatScorer 威胁评分器
type ThreatScorer struct {
    scoringEngine *ScoringEngine
    factors       []ScoringFactor
}

// ThreatScore 综合威胁评分
type ThreatScore struct {
    ID            string
    Timestamp     time.Time
    TotalScore    int       // 0-100
    Level         ThreatLevel
    Confidence    float64   // 0-1
    
    // 分项评分
    FactorScores  map[string]FactorScore
    
    // 综合评估
    RiskFactors   []string
    AttackStage   AttackStage
    AttackType    AttackType
    TimeToImpact  time.Duration  // 预估影响时间
    
    // 推荐响应
    Urgency       ResponseUrgency
    RecommendedActions []RecommendedAction
}

// FactorScore 分项评分
type FactorScore struct {
    Name        string
    Score       int       // 0-100
    Weight      float64   // 权重
    Contribution int      // 对总分的贡献
    Details     map[string]interface{}
}

// ScoringFactor 评分因素
type ScoringFactor struct {
    Name        string
    Weight      float64
    Calculator  FactorCalculator
    Enabled     bool
}

// 评分因素列表（按TrueNAS设计）
var DefaultScoringFactors = []ScoringFactor{
    {Name: "file_entropy", Weight: 0.15},       // 文件熵值变化
    {Name: "extension_change", Weight: 0.20},   // 扩展名变更频率
    {Name: "file_operations", Weight: 0.15},    // 文件操作频率
    {Name: "honeyfile_trigger", Weight: 0.25},  // 蜜罐触发（最高权重）
    {Name: "snapshot_anomaly", Weight: 0.15},   // 快照异常
    {Name: "ransom_note", Weight: 0.10},        // 勒索信检测
    {Name: "known_signature", Weight: 0.20},    // 已知特征库匹配
    {Name: "process_behavior", Weight: 0.10},   // 进程行为异常
    {Name: "network_pattern", Weight: 0.05},    // 网络模式异常
}

// AttackStage 攻击阶段
type AttackStage string
const (
    StageReconnaissance   AttackStage = "reconnaissance"   // 侦察阶段
    StageInitialAccess    AttackStage = "initial_access"   // 初始访问
    StageExecution        AttackStage = "execution"        // 执行加密
    StagePersistence      AttackStage = "persistence"      // 持久化
    StageDataExfiltration AttackStage = "exfiltration"     // 数据外泄
    StageImpact           AttackStage = "impact"           // 影响阶段
)

// AttackType 攻击类型
type AttackType string
const (
    TypeEncryption        AttackType = "encryption"        // 加密型勒索
    TypeLocker            AttackType = "locker"            // 锁屏型勒索
    TypeDestructive       AttackType = "destructive"       // 破坏型攻击
    TypeDoubleExtortion   AttackType = "double_extortion"  // 双重勒索
    TypeUnknown           AttackType = "unknown"
)

// ResponseUrgency 响应紧急度
type ResponseUrgency string
const (
    UrgencyImmediate  ResponseUrgency = "immediate"   // 立即响应（<1分钟）
    UrgencyUrgent     ResponseUrgency = "urgent"      // 紧急响应（<5分钟）
    UrgencyHigh       ResponseUrgency = "high"        // 高优先级（<15分钟）
    UrgencyNormal     ResponseUrgency = "normal"      // 正常优先级
)

// RecommendedAction 推荐动作
type RecommendedAction struct {
    Priority    int
    Action      string
    Reason      string
    Automated   bool       // 是否可自动执行
    Risk        string     // 执行风险
}
```

---

### Phase 4: 快照保护增强（优先级：中）

#### 4.1 快照锁定机制

**新增文件**: `internal/security/ransomware/snapshot_lock.go`

```go
// SnapshotLockManager 快照锁定管理器
type SnapshotLockManager struct {
    zfsAdapter    ZFSAdapterInterface
    lockedSnaps   map[string]*LockedSnapshot
    lockPolicy    SnapshotLockPolicy
}

// LockedSnapshot 已锁定快照
type LockedSnapshot struct {
    SnapshotName  string
    Dataset       string
    LockedAt      time.Time
    LockReason    string
    LockTag       string
    ThreatLevel   ThreatLevel
    DetectionID   string
    ExpiresAt     *time.Time
    Permanent     bool
}

// SnapshotLockPolicy 快照锁定策略
type SnapshotLockPolicy struct {
    AutoLockOnThreat     bool          // 检测到威胁自动锁定
    LockRecentCount      int           // 锁定最近的N个快照
    MinLockDuration      time.Duration // 最小锁定时间
    MaxLockDuration      time.Duration // 最大锁定时间（自动解锁）
    ProtectCritical      bool          // 保护关键数据集的所有快照
    AlertOnUnlockAttempt bool          // 尝试解锁时告警
}
```

---

### Phase 5: 响应编排系统（优先级：高）

#### 5.1 自动响应编排

**增强文件**: `internal/security/ransomware/response_orchestrator.go`

```go
// ResponseOrchestrator 响应编排器
type ResponseOrchestrator struct {
    config        ResponseOrchestratorConfig
    actionQueue   chan ResponseAction
    executedActions []ResponseActionRecord
    dependencies  ActionDependencyGraph
}

// ResponseOrchestratorConfig 响应编排配置
type ResponseOrchestratorConfig struct {
    AutoExecuteThreshold   ThreatLevel   // 自动执行的威胁阈值
    MaxAutoActions         int           // 一次事件最多自动动作数
    ActionTimeout          time.Duration // 动作执行超时
    RequireConfirmation    []string      // 需确认的动作类型
    ParallelExecution      bool          // 是否并行执行
    RollbackEnabled        bool          // 是否支持回滚
}

// ResponseAction 响应动作
type ResponseAction struct {
    ID          string
    Type        ResponseActionType
    Priority    int
    Target      string
    Parameters  map[string]interface{}
    Dependencies []string  // 依赖的动作ID
    Timeout     time.Duration
    RetryCount  int
    AutoExecute bool
    Status      ActionStatus
}

// ResponseActionType 响应动作类型
type ResponseActionType string
const (
    // 立即响应（critical级别）
    ActionLockShares       ResponseActionType = "lock_shares"
    ActionBlockSourceIP    ResponseActionType = "block_source_ip"
    ActionKillSuspiciousProcess ResponseActionType = "kill_process"
    ActionEmergencySnapshot ResponseActionType = "emergency_snapshot"
    
    // 高级响应（high级别）
    ActionSetReadOnly      ResponseActionType = "set_readonly"
    ActionQuarantineFiles  ResponseActionType = "quarantine_files"
    ActionLockSnapshots    ResponseActionType = "lock_snapshots"
    ActionRestrictUser     ResponseActionType = "restrict_user"
    
    // 中级响应（medium级别）
    ActionAlertAdmins      ResponseActionType = "alert_admins"
    ActionMonitorIntensify ResponseActionType = "intensify_monitoring"
    ActionCreateSnapshot   ResponseActionType = "create_snapshot"
    
    // 低级响应（low级别）
    ActionLogEvent         ResponseActionType = "log_event"
    ActionUpdateStats      ResponseActionType = "update_stats"
)

// ResponseActionRecord 动作执行记录
type ResponseActionRecord struct {
    ActionID    string
    ActionType  ResponseActionType
    StartedAt   time.Time
    CompletedAt *time.Time
    Status      ActionStatus
    Result      interface{}
    Error       string
    RollbackData interface{}  // 用于回滚的数据
}
```

---

## 实施计划

### 阶段一：核心响应能力（预计2周）

| 任务 | 文件 | 优先级 | 预计时间 |
|-----|------|-------|---------|
| 共享锁定管理器 | share_lockdown.go | P0 | 3天 |
| IP阻断系统 | ip_blocker.go | P0 | 2天 |
| 响应编排器 | response_orchestrator.go | P0 | 3天 |
| 集成到实时防护 | realtime_protection.go修改 | P0 | 2天 |

### 阶段二：协议级监控（预计1周）

| 任务 | 文件 | 优先级 | 预计时间 |
|-----|------|-------|---------|
| SMB/NFS监控 | protocol_monitor.go | P1 | 3天 |
| 网络连接追踪 | network_tracker.go | P1 | 2天 |
| 防火墙适配器 | firewall_adapter.go | P1 | 2天 |

### 阶段三：评分与快照增强（预计1周）

| 任务 | 文件 | 优先级 | 预计时间 |
|-----|------|-------|---------|
| 综合威胁评分 | threat_scorer.go | P2 | 3天 |
| 快照锁定机制 | snapshot_lock.go | P2 | 2天 |
| API与集成测试 | 多文件 | P2 | 2天 |

---

## API设计

### 新增REST API端点

```
# 共享锁定
POST   /api/v1/ransomware/shares/{name}/lock
POST   /api/v1/ransomware/shares/{name}/unlock
GET    /api/v1/ransomware/shares/status

# IP阻断
POST   /api/v1/ransomware/ip/block
POST   /api/v1/ransomware/ip/unblock
GET    /api/v1/ransomware/ip/blocklist
GET    /api/v1/ransomware/ip/suspicious

# 响应编排
POST   /api/v1/ransomware/response/execute
POST   /api/v1/ransomware/response/rollback/{actionId}
GET    /api/v1/ransomware/response/history

# 威胁评分
GET    /api/v1/ransomware/threat/score/{detectionId}
GET    /api/v1/ransomware/threat/analysis

# 快照锁定
POST   /api/v1/ransomware/snapshots/{name}/lock
POST   /api/v1/ransomware/snapshots/{name}/unlock
GET    /api/v1/ransomware/snapshots/locked
```

---

## 与现有模块的集成

### realtime_protection.go修改

```go
// 新增组件
type RealtimeProtection struct {
    // ... 现有组件 ...
    
    // 新增组件
    shareLockdown   *ShareLockdownManager    // 共享锁定
    ipBlocker       *IPBlocker                // IP阻断
    responseOrchestrator *ResponseOrchestrator // 响应编排
    threatScorer    *ThreatScorer             // 威胁评分
    protocolMonitor *ProtocolMonitor          // 协议监控
}

// 修改executeResponse方法
func (rp *RealtimeProtection) executeResponse(result *DetectionResult) {
    // 1. 综合评分
    score := rp.threatScorer.CalculateScore(result)
    
    // 2. 编排响应动作
    actions := rp.responseOrchestrator.PlanActions(score)
    
    // 3. 执行响应（支持自动和手动确认）
    rp.responseOrchestrator.ExecuteActions(actions)
}
```

---

## 配置示例

```json
{
  "ransomware_defense": {
    "enabled": true,
    "monitoring": {
      "protocols": ["smb", "nfs", "ftp"],
      "paths": ["/data", "/shares", "/home"],
      "sensitivity": "high"
    },
    "detection": {
      "honeyfile": {
        "enabled": true,
        "files_per_path": 10
      },
      "entropy_threshold": 7.5,
      "behavior_window": "5m"
    },
    "response": {
      "auto_lock_threshold": "high",
      "auto_block_ip": true,
      "lock_duration": "24h",
      "preserve_snapshots": true,
      "max_auto_actions": 5
    },
    "alerting": {
      "channels": ["email", "webhook", "push"],
      "severity_threshold": "medium"
    }
  }
}
```

---

## 测试计划

### 单元测试覆盖

- 每个新模块需达到80%+覆盖率
- 重点关注边界条件和错误处理

### 集成测试场景

1. **蜜罐文件触发流程**: 创建蜜罐文件 → 模拟修改 → 验证检测和响应
2. **SMB异常行为检测**: 模拟高频文件操作 → 验证协议监控和锁定
3. **IP阻断流程**: 模拟恶意IP → 验证自动阻断和防火墙规则
4. **快照保护流程**: 模拟威胁 → 验证快照锁定和恢复能力
5. **综合响应编排**: 模拟critical威胁 → 验证完整响应流程

---

## 风险与注意事项

### 潜在风险

1. **误报影响业务**: 自动锁定可能导致正常业务中断
   - 缓解：设置白名单、确认机制、快速解锁流程

2. **防火墙操作风险**: 错误阻断可能影响系统访问
   - 缓解：白名单IP（管理网络）、阻断前验证、回滚机制

3. **性能影响**: 协议级监控可能增加延迟
   - 缓解：异步处理、事件缓冲、可配置监控深度

### 回滚机制

- 所有自动响应动作需记录回滚数据
- 提供一键撤销功能
- 支持批量回滚（如撤销一次事件的所有动作）

---

## 参考文档

- TrueNAS SCALE 26 Ransomware Defense官方文档
- NIST勒索软件防护指南
- CISA勒索软件最佳实践

---

## 附录：现有模块能力对照表

| 能力 | 现有实现 | TrueNAS对标 | 增强需求 |
|-----|---------|------------|---------|
| 文件监控 | FileMonitor | SMB/NFS实时监控 | 需协议级增强 |
| 蜜罐检测 | honeyfile.go | Honeypot Files | ✅ 已满足 |
| 行为分析 | behavior_monitor.go | Behavior Analysis | ✅ 已满足 |
| 熵值检测 | enhanced_detectors.go | Entropy Detection | ✅ 已满足 |
| 特征匹配 | signature_db.go | Signature DB | ✅ 已满足 |
| 快照异常 | snapshot_anomaly.go | Snapshot Comparison | ✅ 已满足 |
| 自动快照 | auto_snapshot.go | Auto Snapshot | ✅ 已满足 |
| 隔离管理 | quarantine.go | Quarantine | ✅ 已满足 |
| 告警管理 | alert_manager.go | Alerting | ✅ 已满足 |
| 共享锁定 | 无 | Share Lockdown | ❌ 需新增 |
| IP阻断 | 无 | IP Blocking | ❌ 需新增 |
| 响应编排 | 部分 | Response Actions | ⚠️ 需增强 |
| 威胁评分 | 部分 | Threat Scoring | ⚠️ 需增强 |
| 快照锁定 | 无 | Snapshot Hold | ❌ 需新增 |

---

**文档版本**: v1.0
**创建日期**: 2026-04-11
**作者**: 刑部（安全审计与合规）
**审批状态**: 待审批