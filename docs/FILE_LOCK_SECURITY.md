# 文件锁定安全设计文档

> **版本**: v2.384.0  
> **刑部安全评估**  
> **日期**: 2026-04-03

## 概述

本文档详细分析文件锁定机制的安全设计，对标群晖Drive文件锁定功能，确保防止死锁和权限兼容。设计基于现有 `internal/files/lock` 模块，扩展安全防护能力。

---

## 一、群晖Drive文件锁定机制分析

### 1.1 群晖锁定架构

群晖 DSM Drive 采用三层锁定架构：

```
┌─────────────────────────────────────────────────────────────┐
│                    群晖 Drive 锁定架构                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   应用层    ┌──────────────────────────────────────────┐    │
│            │  Synology Drive Client / Web Interface     │    │
│            └──────────────────────────────────────────┘    │
│                        │                                    │
│   协议层    ┌──────────┼──────────────────────────────┐    │
│            │ SMB Lock │ NFS Lock │ WebDAV Lock        │    │
│            └──────────┼──────────────────────────────┘    │
│                        │                                    │
│   存储层    ┌──────────▼──────────────────────────────┐    │
│            │    Btrfs/ZFS 文件系统属性 + 数据库记录     │    │
│            └──────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 群晖锁定特性

| 特性 | 群晖实现 | NAS-OS实现 | 安全影响 |
|------|----------|------------|----------|
| **锁类型** | 独占锁(文件级) | 独占锁 + 共享锁 | 共享锁可能扩大攻击面 |
| **锁范围** | 文件级别 | 文件级别 | 相同 |
| **超时机制** | 30分钟默认 | 30分钟默认 | 相同 |
| **自动续期** | ✓ 编辑期间自动续期 | ✓ 可配置自动续期 | 需防止无限续期攻击 |
| **多用户提示** | ✓ 弹窗提示锁定者 | ✓ API返回冲突信息 | 需防止信息泄露 |
| **会话绑定** | ✓ 与Drive会话绑定 | ✓ 支持SessionID | 防止会话劫持 |
| **权限检查** | ✓ ACL权限前置检查 | ✓ 集成RBAC | 关键安全控制点 |

### 1.3 群晖锁定流程（安全视角）

```
用户打开文件请求
        │
        ▼
┌─────────────────────────────────────────────┐
│  1. 权限验证                                 │
│  - 检查用户ACL权限                           │
│  - 检查共享访问权限                          │
│  - 记录审计日志                              │
└─────────────────────────────────────────────┘
        │ 权限OK
        ▼
┌─────────────────────────────────────────────┐
│  2. 锁定状态检查                             │
│  - 查询现有锁状态                            │
│  - 检查是否过期                              │
│  - 检查是否同一用户                          │
└─────────────────────────────────────────────┘
        │
        ├────── 已被他人锁定 ──────► 返回冲突信息
        │                              (显示锁定者姓名/邮箱)
        │
        ▼ 无锁或过期
┌─────────────────────────────────────────────┐
│  3. 创建锁记录                               │
│  - 生成锁ID (UUID)                           │
│  - 设置过期时间                              │
│  - 绑定会话信息                              │
│  - 存储到数据库                              │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  4. 启动心跳检测                             │
│  - 客户端定期发送心跳                        │
│  - 服务端更新LastAccessed                    │
│  - 失联自动释放锁                            │
└─────────────────────────────────────────────┘
```

---

## 二、安全威胁分析

### 2.1 死锁风险

**威胁场景**：

```
用户A锁定文件X ──► 等待锁定文件Y
用户B锁定文件Y ──► 等待锁定文件X
        │
        ▼
    死循环等待（死锁）
```

**风险等级**: 🟡 中等

**现有防护**:
- ✅ 超时机制（30分钟默认，24小时最大）
- ✅ 等待队列FIFO排序
- ⚠️ 缺少锁顺序检测（需增强）

### 2.2 锁定滥用（资源耗尽）

**威胁场景**：
- 恶意用户锁定大量文件
- 锁定超大文件占用系统资源
- 创建大量等待队列请求

**风险等级**: 🔴 高

**现有防护**:
- ✅ MaxTotalLocks: 10000 系统最大锁数量
- ✅ MaxLocksPerFile: 100 每文件最大共享锁数
- ✅ MaxWaitQueueSize: 50 等待队列最大长度
- ⚠️ 缺少用户级配额限制

### 2.3 权限提升攻击

**威胁场景**：
- 低权限用户锁定高权限文件
- 利用锁机制绕过ACL检查
- 锁定后执行越权操作

**风险等级**: 🔴 高

**现有防护**:
- ✅ 锁定前强制权限检查（LockWithAuth）
- ✅ RBAC集成（filelock:write权限）
- ✅ 共享ACL检查

### 2.4 锁劫持/会话劫持

**威胁场景**：
- 攻击者伪造SessionID获取他人锁
- 攻击者冒充ClientID释放他人锁
- 中间人攻击窃取锁Token

**风险等级**: 🔴 高

**现有防护**:
- ✅ Owner验证（Unlock需验证Owner）
- ✅ ClientID绑定
- ⚠️ 缺少Token签名验证

### 2.5 信息泄露

**威胁场景**：
- 冲突响应暴露锁定者邮箱
- 审计日志包含敏感信息
- API响应泄露用户信息

**风险等级**: 🟡 中等

**现有防护**:
- ✅ 审计日志签名防篡改
- ⚠️ 冲突信息可能暴露用户邮箱

### 2.6 锁抢占滥用

**威胁场景**：
- 管理员滥用ForceUnlock
- 高优先级用户恶意抢占
- 抢占导致数据丢失

**风险等级**: 🟡 中等

**现有防护**:
- ✅ 抢占需要管理员权限（filelock:admin）
- ✅ 抢占审计日志记录
- ⚠️ 缺少抢占通知机制

---

## 三、防死锁方案设计

### 3.1 死锁检测算法

采用**资源有序分配法**预防死锁：

```go
// DeadlockDetector 死锁检测器
type DeadlockDetector struct {
    // lockGraph 锁依赖图
    lockGraph map[string][]string // file -> waiting files
    // lockOrder 锁定顺序（路径字典序）
    lockOrder func(a, b string) bool
}

// CheckDeadlock 检查是否会导致死锁
func (d *DeadlockDetector) CheckDeadlock(req *LockRequest) error {
    // 1. 按路径字典序锁定
    if !d.isOrderedLock(req) {
        return ErrLockOrderViolation
    }
    
    // 2. 检查依赖图是否形成环
    if d.hasCycle(req.FilePath, req.Owner) {
        return ErrDeadlockDetected
    }
    
    return nil
}

// isOrderedLock 检查锁定顺序
func (d *DeadlockDetector) isOrderedLock(req *LockRequest) bool {
    // 获取用户当前持有的锁
    heldLocks := d.getHeldLocks(req.Owner)
    
    // 新锁路径必须大于所有已持有锁路径
    for _, heldPath := range heldLocks {
        if req.FilePath < heldPath {
            return false // 违反顺序
        }
    }
    
    return true
}
```

### 3.2 锁定顺序规范

强制执行**路径字典序**锁定规则：

```
正确顺序（升序）:
/doc/a.pdf ──► /doc/b.pdf ──► /doc/c.pdf

错误顺序（会导致死锁）:
/doc/c.pdf ──► /doc/a.pdf （违反升序规则）
```

**例外情况**：
- 同一用户重新锁定：允许
- 锁升级/降级：允许
- 系统操作（PriorityCritical）：允许

### 3.3 死锁检测配置

```yaml
filelock:
  deadlock:
    enabled: true
    detection_interval: 30s
    # 死锁自动解除
    auto_resolve: true
    resolve_policy: "youngest_first"  # 释放最新锁
    # 警告阈值
    warning_threshold: 3  # 同一用户跨3个文件锁定时警告
```

### 3.4 死锁解除流程

```
死锁检测触发
        │
        ▼
┌─────────────────────────────────────────────┐
│  1. 识别死锁参与者                           │
│  - 分析依赖图                               │
│  - 找出环中所有用户                          │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  2. 选择释放策略                             │
│  - youngest_first: 释放最新锁                │
│  - lowest_priority: 释放低优先级锁           │
│  - notify_wait: 通知参与者协商               │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  3. 执行锁释放                               │
│  - 记录审计日志                             │
│  - 通知被释放用户                           │
│  - 恢复等待队列                             │
└─────────────────────────────────────────────┘
```

---

## 四、超时释放机制设计

### 4.1 多层超时体系

```
┌─────────────────────────────────────────────────────────────┐
│                      超时机制层级                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   L1: 锁持有超时                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ DefaultTimeout: 30分钟                               │   │
│   │ MaxTimeout: 24小时                                   │   │
│   │ 强制过期释放                                          │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
│   L2: 等待队列超时                                           │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ WaitTimeout: 用户可配置（最大5分钟）                   │   │
│   │ 超时自动取消等待                                      │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
│   L3: 心跳超时                                               │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ HeartbeatInterval: 1分钟                             │   │
│   │ HeartbeatTimeout: 5分钟（3次失联）                    │   │
│   │ 客户端断连自动释放                                    │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
│   L4: 会话超时                                               │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ SessionTimeout: 与协议绑定                            │   │
│   │ SMB: 会话断开即释放                                   │   │
│   │ WebDAV: HTTP会话超时                                  │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
│   L5: 系统清理超时                                           │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ CleanupInterval: 5分钟                               │   │
│   │ 扫描并释放所有过期锁                                  │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 超时配置最佳实践

```yaml
filelock:
  timeout:
    # 锁持有超时
    default_timeout: 30m        # 默认30分钟
    max_timeout: 24h            # 最大24小时（防无限锁定）
    
    # 自动续期（需限制）
    auto_renewal: true
    renewal_interval: 10m       # 每10分钟检查续期
    renewal_before: 5m          # 过期前5分钟续期
    max_renewal_count: 48       # 最大续期次数（48次=8小时）
    
    # 心跳检测
    heartbeat_enabled: true
    heartbeat_interval: 1m      # 心跳间隔
    heartbeat_timeout: 3m       # 3次失联即释放
    
    # 系统清理
    cleanup_interval: 5m        # 清理间隔
    
    # 等待队列
    wait_timeout_max: 5m        # 最大等待时间
```

### 4.3 超时释放审计日志

所有超时释放必须记录审计：

```json
{
  "id": "audit-xxx",
  "timestamp": "2026-04-03T14:00:00Z",
  "event": "lock_expired",
  "lockId": "lock-12345",
  "filePath": "/share/docs/report.pdf",
  "owner": "user@example.com",
  "ownerName": "张三",
  "duration": 1800000,  // 持有30分钟
  "reason": "timeout",
  "details": {
    "expired_at": "2026-04-03T14:00:00Z",
    "last_heartbeat": "2026-04-03T13:55:00Z",
    "heartbeat_missed": 3
  }
}
```

### 4.4 强制释放权限控制

```go
// ForceUnlockPolicy 强制释放策略
type ForceUnlockPolicy struct {
    // 允许强制释放的角色
    AllowedRoles []string `json:"allowedRoles"` // ["admin", "super-admin"]
    
    // 强制释放前必须通知
    NotifyBefore time.Duration `json:"notifyBefore"` // 1分钟
    
    // 强制释放需要理由
    RequireReason bool `json:"requireReason"` // true
    
    // 强制释放后锁定冷却期
    CoolDownPeriod time.Duration `json:"coolDownPeriod"` // 5分钟内不能重新锁定
}

// 强制释放检查
func (m *Manager) ForceUnlock(lockID string, reason string, operator string) error {
    // 1. 权限检查
    if !m.rbac.Check(operator, "filelock", "admin", "") {
        return ErrPermissionDenied
    }
    
    // 2. 原因检查
    if m.policy.RequireReason && reason == "" {
        return ErrReasonRequired
    }
    
    // 3. 执行释放
    lock := m.getLock(lockID)
    
    // 4. 记录审计
    m.logAudit(&LockAuditEntry{
        Event:     AuditEventLockForceReleased,
        LockID:    lockID,
        FilePath:  lock.FilePath,
        Owner:     lock.Owner,
        Reason:    reason,
        AdminID:   operator,
    })
    
    // 5. 通知原持有者
    m.notifyOwner(lock, "您的文件锁已被管理员强制释放: "+reason)
    
    return nil
}
```

---

## 五、权限兼容设计

### 5.1 与现有权限体系集成

NAS-OS 权限体系：
- ✅ SMB/NFS ACL权限
- ✅ LDAP/AD用户认证
- ✅ RBAC角色管理
- ✅ 共享权限控制

锁定机制权限检查流程：

```
┌─────────────────────────────────────────────────────────────┐
│                     权限检查流程                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Step 1: 用户身份验证                                 │    │
│  │ - LDAP/AD认证                                        │    │
│  │ - Session有效性检查                                  │    │
│  └─────────────────────────────────────────────────────┘    │
│                        │                                     │
│                        ▼                                     │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Step 2: 文件访问权限检查                             │    │
│  │ - SMB/NFS ACL权限                                    │    │
│  │ - 检查文件写权限（锁定需要写权限）                     │    │
│  │ - 检查共享访问权限                                   │    │
│  └─────────────────────────────────────────────────────┘    │
│                        │                                     │
│                        ▼                                     │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Step 3: 锁操作权限检查                               │    │
│  │ - RBAC: filelock:write                              │    │
│  │ - 用户配额检查                                       │    │
│  └─────────────────────────────────────────────────────┘    │
│                        │                                     │
│                        ▼                                     │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Step 4: 管理操作权限检查（可选）                      │    │
│  │ - RBAC: filelock:admin                              │    │
│  │ - 强制释放/抢占权限                                  │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 RBAC权限定义

```go
// 文件锁相关权限
var FileLockPermissions = []Permission{
    {
        Resource:    "filelock",
        Action:      "read",
        Description: "查看文件锁状态",
    },
    {
        Resource:    "filelock",
        Action:      "write",
        Description: "获取/释放文件锁",
        DependsOn:   []string{"filelock:read", "file:write"},
    },
    {
        Resource:    "filelock",
        Action:      "admin",
        Description: "强制释放锁/管理锁策略",
        DependsOn:   []string{"filelock:write"},
    },
}

// 角色权限映射
var RolePermissions = map[string][]Permission{
    "user": []Permission{
        {Resource: "filelock", Action: "read"},
        {Resource: "filelock", Action: "write"},
    },
    "admin": []Permission{
        {Resource: "filelock", Action: "read"},
        {Resource: "filelock", Action: "write"},
        {Resource: "filelock", Action: "admin"},
    },
}
```

### 5.3 用户配额限制

防止锁定滥用的配额设计：

```go
// LockQuota 用户锁配额
type LockQuota struct {
    // MaxLocksPerUser 用户最大锁数量
    MaxLocksPerUser int `json:"maxLocksPerUser"` // 默认50
    
    // MaxLockDurationPerUser 用户最大锁持有时长（小时）
    MaxLockDurationPerUser int `json:"maxLockDurationPerUser"` // 默认48小时
    
    // MaxWaitRequestsPerUser 用户最大等待请求数
    MaxWaitRequestsPerUser int `json:"maxWaitRequestsPerUser"` // 默认10
    
    // CooldownAfterForceUnlock 强制释放后冷却期（分钟）
    CooldownAfterForceUnlock int `json:"cooldownAfterForceUnlock"` // 默认5
}

// 配额检查
func (m *Manager) checkQuota(owner string) error {
    // 检查用户当前锁数量
    userLocks := m.ListLocksByOwner(owner)
    if len(userLocks) >= m.quota.MaxLocksPerUser {
        return ErrQuotaExceeded
    }
    
    // 检查用户锁持有时长
    totalDuration := 0
    for _, lock := range userLocks {
        duration := time.Since(lock.CreatedAt).Hours()
        totalDuration += int(duration)
    }
    if totalDuration >= m.quota.MaxLockDurationPerUser {
        return ErrDurationQuotaExceeded
    }
    
    return nil
}
```

---

## 六、审计日志安全设计

### 6.1 审计日志安全特性

现有审计模块（`internal/files/lock/audit.go`）已实现：

| 安全特性 | 实现状态 | 说明 |
|----------|----------|------|
| **HMAC签名** | ✅ 已实现 | SHA256签名防篡改 |
| **签名验证** | ✅ 已实现 | VerifyEntry方法 |
| **日志轮转** | ✅ 已实现 | 按大小/时间轮转 |
| **自动清理** | ✅ 已实现 | 90天保留期 |
| **访问控制** | ⚠️ 需加强 | 日志文件权限0640 |

### 6.2 审计事件安全分级

```go
// AuditLevel 审计级别
type AuditLevel int

const (
    AuditLevelInfo AuditLevel = iota     // 普通
    AuditLevelWarning                    // 警告
    AuditLevelCritical                   // 关键（必须审计）
)

// 事件分级映射
var EventAuditLevel = map[LockAuditEvent]AuditLevel{
    AuditEventLockAcquired:        AuditLevelInfo,
    AuditEventLockReleased:        AuditLevelInfo,
    AuditEventLockExpired:         AuditLevelInfo,
    AuditEventLockExtended:        AuditLevelInfo,
    AuditEventLockConflict:        AuditLevelWarning,
    AuditEventLockPreempted:       AuditLevelWarning,
    AuditEventLockForceReleased:   AuditLevelCritical,  // 必须审计
    AuditEventLockUpgraded:        AuditLevelInfo,
    AuditEventLockDowngraded:      AuditLevelInfo,
}
```

### 6.3 敏感信息保护

```go
// SanitizeAuditEntry 清理审计条目敏感信息
func SanitizeAuditEntry(entry *LockAuditEntry) *LockAuditEntry {
    // 邮箱脱敏
    if entry.OwnerEmail != "" {
        entry.OwnerEmail = maskEmail(entry.OwnerEmail)
    }
    
    // 用户名脱敏（可选）
    // entry.OwnerName = maskName(entry.OwnerName)
    
    return entry
}

func maskEmail(email string) string {
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return email
    }
    name := parts[0]
    if len(name) > 2 {
        return name[:2] + "***@" + parts[1]
    }
    return "***@" + parts[1]
}
```

---

## 七、协议适配安全

### 7.1 SMB锁定适配

```go
// SMBLockAdapter SMB锁适配器（已实现）
type SMBLockAdapter struct {
    manager *Manager
}

// SMB锁定安全注意事项：
// 1. SMB2/SMB3 锁定协议映射
// 2. 会话断开自动释放
// 3. Opportunistic Locking (OpLock) 支持
// 4. 与Windows文件共享兼容
```

### 7.2 NFS锁定适配

```go
// NFSLockAdapter NFS锁适配器（已实现）
type NFSLockAdapter struct {
    manager *Manager
}

// NFS锁定安全注意事项：
// 1. NFSv4 锁状态映射
// 2. lockd守护进程集成
// 3. 网络分区处理
// 4. 客户端重连恢复
```

### 7.3 WebDAV锁定适配

现有 WebDAV 锁模块（`internal/webdav/lock.go`）：

```go
// WebDAV锁安全注意事项：
// 1. RFC 2518 锁令牌格式
// 2. urn:uuid: 格式令牌
// 3. 锁深度（Depth: 0/1）
// 4. 锁范围（exclusive/shared）
// 5. 超时格式（Second-3600）
```

---

## 八、安全配置清单

### 8.1 推荐安全配置

```yaml
# 文件锁定安全配置
filelock:
  # 基本超时（防止无限锁定）
  default_timeout: 30m
  max_timeout: 24h
  
  # 系统资源限制（防止资源耗尽）
  max_total_locks: 10000
  max_locks_per_file: 100
  max_locks_per_user: 50         # 用户配额
  
  # 等待队列限制
  enable_wait_queue: true
  max_wait_queue_size: 50
  max_wait_requests_per_user: 10
  wait_timeout_max: 5m
  
  # 死锁预防
  deadlock_detection: true
  deadlock_resolve_policy: "youngest_first"
  
  # 心跳检测（防止僵尸锁）
  heartbeat_enabled: true
  heartbeat_interval: 1m
  heartbeat_timeout: 3m
  
  # 自动续期限制
  auto_renewal: true
  max_renewal_count: 48          # 限制续期次数
  
  # 抢占控制
  enable_preemption: true
  preempt_timeout: 5m
  preempt_min_hold_time: 5m      # 抢占保护期
  
  # 审计日志
  audit_enabled: true
  audit_storage: "database"
  audit_retention: 90d
  audit_sign_enabled: true
  
  # 权限控制
  require_write_permission: true  # 锁定需写权限
  cooldown_after_force: 5m       # 强制释放冷却期
```

### 8.2 禁止的不安全配置

```yaml
# ❌ 禁止以下配置
filelock:
  max_timeout: 0                  # 禁止无限超时
  auto_renewal: false             # 关闭续期可能导致数据丢失
  heartbeat_enabled: false        # 关闭心跳会产生僵尸锁
  audit_enabled: false            # 关闭审计无法追溯
  deadlock_detection: false       # 关闭死锁检测
```

---

## 九、安全测试验证

### 9.1 死锁测试

```bash
# 测试死锁检测
go test -run TestDeadlockDetection ./internal/files/lock/

# 测试场景：
# 1. 用户A锁定文件1，等待文件2
# 2. 用户B锁定文件2，等待文件1
# 3. 系统应检测并自动解除
```

### 9.2 超时测试

```bash
# 测试超时释放
go test -run TestTimeoutRelease ./internal/files/lock/

# 测试场景：
# 1. 创建锁，设置超时
# 2. 等待超时
# 3. 验证锁自动释放
```

### 9.3 权限测试

```bash
# 测试权限检查
go test -run TestPermissionCheck ./internal/files/lock/

# 测试场景：
# 1. 低权限用户锁定高权限文件
# 2. 验证返回权限错误
# 3. 验证审计日志记录
```

---

## 十、安全加固建议

### 10.1 立即实施

| 项目 | 优先级 | 工作量 |
|------|--------|--------|
| 用户配额限制 | 🔴 高 | 小 |
| 死锁检测增强 | 🔴 高 | 中 |
| 审计日志脱敏 | 🟡 中 | 小 |
| 强制释放通知 | 🟡 中 | 小 |
| 心跳超时优化 | 🟡 中 | 小 |

### 10.2 后续规划

| 项目 | 优先级 | 工作量 |
|------|--------|--------|
| Token签名验证 | 🟡 中 | 中 |
| 分布式锁一致性 | 🟡 中 | 大 |
| 锁升级/降级权限细化 | 🟢 低 | 小 |
| WebSocket实时通知 | 🟢 低 | 中 |

---

## 附录

### A. 安全事件响应流程

```
发现安全事件（异常锁定/死锁）
        │
        ▼
┌─────────────────────────────────────────────┐
│  1. 分析事件类型                             │
│  - 死锁：自动解除                            │
│  - 滥用：限制配额                            │
│  - 越权：记录审计                            │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  2. 执行响应动作                             │
│  - 强制释放锁                                │
│  - 限制用户权限                              │
│  - 发送告警                                  │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  3. 记录审计日志                             │
│  - 事件详情                                  │
│  - 响应动作                                  │
│  - 操作者                                    │
└─────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────┐
│  4. 通知相关人员                             │
│  - 锁持有者                                  │
│  - 管理员                                    │
│  - 安全团队                                  │
└─────────────────────────────────────────────┘
```

### B. 审计日志查询示例

```bash
# 查询强制释放事件
nasctl audit query --event lock_force_released --days 7

# 查询用户锁定统计
nasctl audit stats --owner user@example.com

# 查询冲突事件
nasctl audit query --event lock_conflict --severity warning
```

---

**刑部安全评估结论**：

- ✅ 文件锁定机制基础安全设计完善
- ⚠️ 需增强用户配额限制和死锁检测
- ⚠️ 需加强审计日志敏感信息脱敏
- ⚠️ 强制释放需增加通知机制

**评估日期**: 2026-04-03  
**评估版本**: v2.384.0  
**评估部门**: 刑部