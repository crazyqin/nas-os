# 文件锁定机制设计

## 1. 概述

本文档描述 NAS-OS 系统中文件锁定机制的设计，用于解决多用户、多进程并发访问文件时的冲突问题，确保数据一致性和完整性。

## 2. 需求分析

### 2.1 使用场景

| 场景 | 描述 | 锁类型 |
|------|------|--------|
| 文件编辑 | 用户在编辑器中打开文件 | 独占锁 |
| 批量处理 | 后台任务批量处理文件 | 共享锁/独占锁 |
| 协作编辑 | 多人同时查看文件 | 共享锁 |
| 文件传输 | 上传/下载大文件 | 独占锁 |
| 系统维护 | 备份、扫描等操作 | 共享锁 |
| 网盘同步 | 云端同步文件 | 独占锁 |

### 2.2 功能需求

1. 支持文件级和记录级锁定
2. 支持锁超时自动释放
3. 支持锁升级/降级
4. 支持死锁检测和解决
5. 支持分布式环境（多节点）
6. 支持锁信息持久化

## 3. 锁定粒度设计

### 3.1 锁粒度层次

```
┌─────────────────────────────────────────────────────────┐
│                     锁粒度层次                           │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │  文件系统锁 (Filesystem Lock)                    │   │
│  │  - 锁定整个文件系统                              │   │
│  │  - 用于系统维护                                  │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
│                          ▼                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │  目录锁 (Directory Lock)                         │   │
│  │  - 锁定整个目录                                  │   │
│  │  - 用于批量操作                                  │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
│                          ▼                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │  文件锁 (File Lock)                              │   │
│  │  - 锁定整个文件                                  │   │
│  │  - 最常用的锁级别                                │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
│                          ▼                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │  记录锁 (Record Lock / Range Lock)               │   │
│  │  - 锁定文件的特定范围                            │   │
│  │  - 用于并发写入不同区域                          │   │
│  └─────────────────────────────────────────────────┘   │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 3.2 锁类型定义

```go
type LockType int

const (
    LockTypeNone    LockType = iota // 无锁
    LockTypeShared                   // 共享锁（读锁）
    LockTypeExclusive                // 独占锁（写锁）
    LockTypeIntentShared             // 意向共享锁
    LockTypeIntentExclusive          // 意向独占锁
)

type LockScope int

const (
    LockScopeFilesystem LockScope = iota // 文件系统级
    LockScopeDirectory                   // 目录级
    LockScopeFile                        // 文件级
    LockScopeRecord                      // 记录级（字节范围）
)
```

### 3.3 锁兼容性矩阵

| 当前锁 | 请求锁 | 兼容? |
|--------|--------|-------|
| 无锁 | 共享锁 | ✅ |
| 无锁 | 独占锁 | ✅ |
| 共享锁 | 共享锁 | ✅ |
| 共享锁 | 独占锁 | ❌ |
| 独占锁 | 共享锁 | ❌ |
| 独占锁 | 独占锁 | ❌ |
| IS | IS | ✅ |
| IS | IX | ✅ |
| IS | S | ✅ |
| IS | X | ❌ |
| IX | IS | ✅ |
| IX | IX | ✅ |
| IX | S | ❌ |
| IX | X | ❌ |

## 4. 锁数据结构

### 4.1 锁对象

```go
type FileLock struct {
    ID          string      `json:"id"`          // 锁ID
    Resource    string      `json:"resource"`    // 锁定资源路径
    Scope       LockScope   `json:"scope"`       // 锁范围
    Type        LockType    `json:"type"`        // 锁类型
    Owner       string      `json:"owner"`       // 锁持有者
    OwnerType   OwnerType   `json:"ownerType"`   // 用户/进程/会话
    PID         int         `json:"pid"`         // 进程ID（可选）
    SessionID   string      `json:"sessionId"`   // 会话ID（可选）

    // 范围锁参数
    Offset      int64       `json:"offset"`      // 起始偏移
    Length      int64       `json:"length"`      // 锁定长度

    // 时间参数
    CreatedAt   time.Time   `json:"createdAt"`   // 创建时间
    ExpiresAt   time.Time   `json:"expiresAt"`   // 过期时间
    LastActive  time.Time   `json:"lastActive"`  // 最后活动时间

    // 状态
    Status      LockStatus  `json:"status"`      // 锁状态
    WaitQueue   []string    `json:"waitQueue"`   // 等待队列
}

type LockStatus string

const (
    LockStatusActive   LockStatus = "active"   // 活跃
    LockStatusWaiting  LockStatus = "waiting"  // 等待中
    LockStatusExpired  LockStatus = "expired"  // 已过期
    LockStatusReleased LockStatus = "released" // 已释放
)

type OwnerType string

const (
    OwnerTypeUser    OwnerType = "user"     // 用户
    OwnerTypeProcess OwnerType = "process"  // 进程
    OwnerTypeSession OwnerType = "session"  // 会话
    OwnerTypeSystem  OwnerType = "system"   // 系统
)
```

### 4.2 锁管理器

```go
type LockManager struct {
    mu            sync.RWMutex
    locks         map[string]*FileLock      // lockID -> Lock
    resourceLocks map[string][]string       // resource -> []lockID
    ownerLocks    map[string][]string       // owner -> []lockID
    config        *LockConfig
    storage       LockStorage
    notifier      LockNotifier
}

type LockConfig struct {
    DefaultTimeout    time.Duration `json:"defaultTimeout"`    // 默认超时
    MaxTimeout        time.Duration `json:"maxTimeout"`        // 最大超时
    CleanupInterval   time.Duration `json:"cleanupInterval"`   // 清理间隔
    MaxLocksPerOwner  int           `json:"maxLocksPerOwner"`  // 每个持有者最大锁数
    EnablePersistence bool          `json:"enablePersistence"` // 是否持久化
    DeadlockDetection bool          `json:"deadlockDetection"` // 死锁检测
}
```

## 5. 锁超时机制

### 5.1 超时策略

```go
type TimeoutPolicy struct {
    DefaultTimeout     time.Duration `json:"defaultTimeout"`     // 默认30分钟
    MinTimeout         time.Duration `json:"minTimeout"`         // 最小1秒
    MaxTimeout         time.Duration `json:"maxTimeout"`         // 最大24小时
    AutoExtend         bool          `json:"autoExtend"`         // 自动延长
    ExtendInterval     time.Duration `json:"extendInterval"`     // 延长间隔
    GracePeriod        time.Duration `json:"gracePeriod"`        // 宽限期
}

// 默认超时策略
var DefaultTimeoutPolicy = TimeoutPolicy{
    DefaultTimeout: 30 * time.Minute,
    MinTimeout:     1 * time.Second,
    MaxTimeout:     24 * time.Hour,
    AutoExtend:     true,
    ExtendInterval: 5 * time.Minute,
    GracePeriod:    30 * time.Second,
}
```

### 5.2 超时处理流程

```
┌─────────────────────────────────────────────────────────┐
│                   锁超时处理流程                         │
│                                                          │
│  ┌─────────────┐                                        │
│  │  创建锁     │                                        │
│  │  设置超时   │                                        │
│  └──────┬──────┘                                        │
│         │                                               │
│         ▼                                               │
│  ┌─────────────┐     是      ┌─────────────┐           │
│  │ 检查活动?   │────────────▶│ 延长超时    │           │
│  └──────┬──────┘             └─────────────┘           │
│         │ 否                                           │
│         ▼                                               │
│  ┌─────────────┐     未到期   ┌─────────────┐           │
│  │ 检查过期?   │◀─────────────│ 继续等待    │           │
│  └──────┬──────┘             └─────────────┘           │
│         │ 已过期                                       │
│         ▼                                               │
│  ┌─────────────┐     有       ┌─────────────┐           │
│  │ 宽限期?     │────────────▶│ 等待恢复    │           │
│  └──────┬──────┘             └─────────────┘           │
│         │ 无宽限期                                     │
│         ▼                                               │
│  ┌─────────────┐                                        │
│  │ 释放锁      │                                        │
│  │ 通知等待者  │                                        │
│  └─────────────┘                                        │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 5.3 自动续期机制

```go
// 锁心跳 - 用于自动续期
func (lm *LockManager) Heartbeat(lockID string) error {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    lock, exists := lm.locks[lockID]
    if !exists {
        return ErrLockNotFound
    }

    if lock.Status != LockStatusActive {
        return ErrLockNotActive
    }

    // 更新活动时间和过期时间
    lock.LastActive = time.Now()
    if lm.config.AutoExtend {
        lock.ExpiresAt = time.Now().Add(lm.config.ExtendInterval)
    }

    return nil
}

// 后台清理任务
func (lm *LockManager) cleanupExpiredLocks() {
    ticker := time.NewTicker(lm.config.CleanupInterval)
    defer ticker.Stop()

    for range ticker.C {
        lm.mu.Lock()
        now := time.Now()
        for id, lock := range lm.locks {
            if lock.ExpiresAt.Before(now) && lock.Status == LockStatusActive {
                lm.releaseLockLocked(id, "expired")
            }
        }
        lm.mu.Unlock()
    }
}
```

## 6. 冲突解决策略

### 6.1 冲突检测

```go
// 检查锁冲突
func (lm *LockManager) checkConflict(resource string, lockType LockType, offset, length int64) (*FileLock, error) {
    lockIDs := lm.resourceLocks[resource]
    for _, id := range lockIDs {
        existing := lm.locks[id]
        if existing.Status != LockStatusActive {
            continue
        }

        // 检查范围重叠
        if lockType == LockTypeExclusive || existing.Type == LockTypeExclusive {
            // 范围检查
            if lm.rangesOverlap(existing.Offset, existing.Length, offset, length) {
                return existing, ErrLockConflict
            }
        }
    }
    return nil, nil
}

// 范围重叠检测
func (lm *LockManager) rangesOverlap(off1, len1, off2, len2 int64) bool {
    if len1 == 0 || len2 == 0 {
        // 全文件锁
        return true
    }
    end1 := off1 + len1
    end2 := off2 + len2
    return off1 < end2 && off2 < end1
}
```

### 6.2 冲突解决策略

```go
type ConflictStrategy string

const (
    ConflictStrategyFail       ConflictStrategy = "fail"       // 直接失败
    ConflictStrategyWait       ConflictStrategy = "wait"       // 等待锁释放
    ConflictStrategyTimeout    ConflictStrategy = "timeout"    // 超时等待
    ConflictStrategyPreempt    ConflictStrategy = "preempt"    // 抢占锁
    ConflictStrategyDowngrade  ConflictStrategy = "downgrade"  // 降级锁
)
```

### 6.3 策略实现

#### 6.3.1 等待策略

```go
func (lm *LockManager) LockWithWait(ctx context.Context, req *LockRequest) (*FileLock, error) {
    // 计算最大等待时间
    deadline := time.Now().Add(req.MaxWait)

    for {
        lock, err := lm.tryLock(req)
        if err == nil {
            return lock, nil
        }

        if !errors.Is(err, ErrLockConflict) {
            return nil, err
        }

        // 检查是否超时
        if time.Now().After(deadline) {
            return nil, ErrLockTimeout
        }

        // 等待锁释放通知或超时
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-lm.waitForUnlock(req.Resource):
            // 继续尝试
        case <-time.After(100 * time.Millisecond):
            // 轮询检查
        }
    }
}
```

#### 6.3.2 抢占策略

```go
func (lm *LockManager) PreemptLock(req *LockRequest) (*FileLock, error) {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    // 检查现有锁
    existingLocks := lm.resourceLocks[req.Resource]

    // 检查抢占权限
    if !lm.canPreempt(req.Owner, existingLocks) {
        return nil, ErrCannotPreempt
    }

    // 强制释放现有锁
    for _, id := range existingLocks {
        lm.releaseLockLocked(id, "preempted")
    }

    // 创建新锁
    return lm.createLockLocked(req)
}

func (lm *LockManager) canPreempt(requester string, existingLocks []string) bool {
    // 抢占规则：
    // 1. 系统管理员可以抢占任何锁
    // 2. 锁已过期的可以被抢占
    // 3. 同一用户的锁可以被抢占
    for _, id := range existingLocks {
        lock := lm.locks[id]
        if lock.OwnerType == OwnerTypeSystem {
            return false
        }
        if lock.ExpiresAt.After(time.Now()) && lock.Owner != requester {
            // 非同用户且未过期
            if !lm.isAdmin(requester) {
                return false
            }
        }
    }
    return true
}
```

### 6.4 死锁检测与解决

```go
// 死锁检测 - 使用等待图算法
func (lm *LockManager) detectDeadlock() [][]string {
    lm.mu.RLock()
    defer lm.mu.RUnlock()

    // 构建等待图
    waitGraph := make(map[string][]string)

    for _, lock := range lm.locks {
        if lock.Status != LockStatusWaiting {
            continue
        }

        // 找到它等待的锁
        for _, blockerID := range lm.resourceLocks[lock.Resource] {
            blocker := lm.locks[blockerID]
            if blocker.Status == LockStatusActive {
                waitGraph[lock.Owner] = append(waitGraph[lock.Owner], blocker.Owner)
            }
        }
    }

    // 检测环
    return lm.findCycles(waitGraph)
}

// 解决死锁 - 选择一个锁进行回滚
func (lm *LockManager) resolveDeadlock(cycle []string) error {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    // 选择牺牲者（最年轻的锁）
    var victim *FileLock
    for _, owner := range cycle {
        for _, lockID := range lm.ownerLocks[owner] {
            lock := lm.locks[lockID]
            if victim == nil || lock.CreatedAt.After(victim.CreatedAt) {
                victim = lock
            }
        }
    }

    if victim != nil {
        lm.releaseLockLocked(victim.ID, "deadlock_victim")
        return nil
    }

    return ErrDeadlockResolution
}
```

## 7. 锁升级与降级

### 7.1 锁升级

```go
// 共享锁 → 独占锁
func (lm *LockManager) UpgradeLock(lockID string) (*FileLock, error) {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    lock, exists := lm.locks[lockID]
    if !exists {
        return nil, ErrLockNotFound
    }

    if lock.Type != LockTypeShared {
        return nil, ErrInvalidLockType
    }

    // 检查是否有其他共享锁
    otherShared := 0
    for _, id := range lm.resourceLocks[lock.Resource] {
        if id == lockID {
            continue
        }
        other := lm.locks[id]
        if other.Type == LockTypeShared && other.Status == LockStatusActive {
            otherShared++
        }
    }

    if otherShared > 0 {
        return nil, ErrLockUpgradeConflict
    }

    // 执行升级
    lock.Type = LockTypeExclusive
    lock.UpdatedAt = time.Now()

    return lock, nil
}
```

### 7.2 锁降级

```go
// 独占锁 → 共享锁
func (lm *LockManager) DowngradeLock(lockID string) (*FileLock, error) {
    lm.mu.Lock()
    defer lm.mu.Unlock()

    lock, exists := lm.locks[lockID]
    if !exists {
        return nil, ErrLockNotFound
    }

    if lock.Type != LockTypeExclusive {
        return nil, ErrInvalidLockType
    }

    // 降级总是成功
    lock.Type = LockTypeShared
    lock.UpdatedAt = time.Now()

    // 通知等待的共享锁请求
    lm.notifyWaiters(lock.Resource)

    return lock, nil
}
```

## 8. 分布式锁支持

### 8.1 分布式架构

```
┌─────────────────────────────────────────────────────────┐
│                   分布式锁架构                           │
│                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │   Node 1    │  │   Node 2    │  │   Node 3    │      │
│  │ LockManager │  │ LockManager │  │ LockManager │      │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘      │
│         │                │                │              │
│         └────────────────┼────────────────┘              │
│                          │                               │
│                          ▼                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │              分布式锁协调器                       │    │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐   │    │
│  │  │  Redis    │  │  etcd     │  │  Consul   │   │    │
│  │  │ (主选)    │  │  (备选)   │  │  (备选)   │   │    │
│  │  └───────────┘  └───────────┘  └───────────┘   │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 8.2 Redis 实现

```go
type RedisLockManager struct {
    client    *redis.Client
    localLock *LockManager // 本地缓存
    prefix    string
}

func (rlm *RedisLockManager) Acquire(ctx context.Context, req *LockRequest) (*FileLock, error) {
    key := rlm.prefix + req.Resource
    value := uuid.New().String()
    ttl := req.Timeout

    // 使用 SET NX EX 原子操作
    ok, err := rlm.client.SetNX(ctx, key, value, ttl).Result()
    if err != nil {
        return nil, err
    }
    if !ok {
        return nil, ErrLockConflict
    }

    lock := &FileLock{
        ID:        value,
        Resource:  req.Resource,
        Type:      req.Type,
        Owner:     req.Owner,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(ttl),
        Status:    LockStatusActive,
    }

    // 本地缓存
    rlm.localLock.mu.Lock()
    rlm.localLock.locks[value] = lock
    rlm.localLock.mu.Unlock()

    return lock, nil
}

func (rlm *RedisLockManager) Release(ctx context.Context, lockID string) error {
    // 使用 Lua 脚本保证原子性
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `

    rlm.localLock.mu.RLock()
    lock, exists := rlm.localLock.locks[lockID]
    rlm.localLock.mu.RUnlock()

    if !exists {
        return ErrLockNotFound
    }

    key := rlm.prefix + lock.Resource
    _, err := rlm.client.Eval(ctx, script, []string{key}, lockID).Result()
    return err
}
```

## 9. API 接口

### 9.1 RESTful API

```
# 锁管理
POST   /api/v1/locks                        # 创建锁
GET    /api/v1/locks/:id                    # 获取锁信息
DELETE /api/v1/locks/:id                    # 释放锁
PUT    /api/v1/locks/:id/upgrade            # 升级锁
PUT    /api/v1/locks/:id/downgrade          # 降级锁
POST   /api/v1/locks/:id/heartbeat          # 锁心跳

# 查询
GET    /api/v1/locks                        # 列出所有锁
GET    /api/v1/locks/resource/:path         # 查询资源锁
GET    /api/v1/locks/owner/:owner           # 查询持有者锁

# 管理
POST   /api/v1/locks/:id/preempt            # 抢占锁
GET    /api/v1/locks/deadlock/check         # 检测死锁
POST   /api/v1/locks/deadlock/resolve       # 解决死锁
```

### 9.2 请求/响应示例

**创建锁请求：**
```json
{
  "resource": "/data/important.doc",
  "scope": "file",
  "type": "exclusive",
  "owner": "user:alice",
  "ownerType": "user",
  "timeout": "30m",
  "maxWait": "5m"
}
```

**锁信息响应：**
```json
{
  "id": "lock-abc123",
  "resource": "/data/important.doc",
  "scope": "file",
  "type": "exclusive",
  "owner": "user:alice",
  "ownerType": "user",
  "status": "active",
  "createdAt": "2026-03-24T12:00:00Z",
  "expiresAt": "2026-03-24T12:30:00Z",
  "lastActive": "2026-03-24T12:15:00Z"
}
```

## 10. 监控与告警

### 10.1 监控指标

```go
type LockMetrics struct {
    // 锁统计
    TotalLocks        int64 `json:"totalLocks"`
    ActiveLocks       int64 `json:"activeLocks"`
    WaitingLocks      int64 `json:"waitingLocks"`
    ExpiredLocks      int64 `json:"expiredLocks"`

    // 按类型统计
    SharedLocks       int64 `json:"sharedLocks"`
    ExclusiveLocks    int64 `json:"exclusiveLocks"`

    // 性能指标
    AvgAcquireTime    int64 `json:"avgAcquireTimeMs"`  // 平均获取时间
    AvgHoldTime       int64 `json:"avgHoldTimeMs"`     // 平均持有时间
    TimeoutRate       float64 `json:"timeoutRate"`     // 超时率
    ConflictRate      float64 `json:"conflictRate"`    // 冲突率

    // 死锁统计
    DeadlockCount     int64 `json:"deadlockCount"`
    DeadlockResolved  int64 `json:"deadlockResolved"`
}
```

### 10.2 告警规则

| 告警 | 条件 | 级别 |
|------|------|------|
| 锁等待过长 | 等待时间 > 5分钟 | WARNING |
| 锁持有过长 | 持有时间 > 1小时 | WARNING |
| 锁超时过多 | 超时率 > 10% | WARNING |
| 死锁检测 | 检测到死锁 | ERROR |
| 锁冲突过多 | 冲突率 > 30% | WARNING |

## 11. 最佳实践

### 11.1 应用层建议

1. **最小化锁持有时间**
   - 只在必要时获取锁
   - 操作完成后立即释放

2. **合理设置超时**
   - 根据操作类型设置适当的超时时间
   - 避免过长的锁持有时间

3. **使用正确的锁类型**
   - 读取操作使用共享锁
   - 写入操作使用独占锁
   - 批量操作考虑范围锁

4. **处理锁失败**
   - 实现重试机制
   - 提供友好的用户提示

### 11.2 集成示例

```go
// 文件操作集成锁
func (fs *FileSystem) WriteFile(ctx context.Context, path string, data []byte) error {
    // 1. 获取独占锁
    lock, err := fs.lockManager.Lock(ctx, &LockRequest{
        Resource: path,
        Scope:    LockScopeFile,
        Type:     LockTypeExclusive,
        Owner:    getCurrentUser(),
        Timeout:  30 * time.Minute,
    })
    if err != nil {
        return fmt.Errorf("获取文件锁失败: %w", err)
    }
    defer fs.lockManager.Unlock(lock.ID)

    // 2. 执行写入
    return fs.doWriteFile(ctx, path, data)
}

// 带心跳的长时间操作
func (fs *FileSystem) ProcessLargeFile(ctx context.Context, path string) error {
    lock, err := fs.lockManager.Lock(ctx, &LockRequest{
        Resource: path,
        Type:     LockTypeExclusive,
        Timeout:  2 * time.Hour,
    })
    if err != nil {
        return err
    }

    // 启动心跳协程
    done := make(chan struct{})
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                fs.lockManager.Heartbeat(lock.ID)
            case <-done:
                return
            }
        }
    }()
    defer close(done)
    defer fs.lockManager.Unlock(lock.ID)

    // 执行处理...
    return fs.doProcess(ctx, path)
}
```

## 12. 配置参考

### 12.1 默认配置

```yaml
lock:
  # 基础配置
  default_timeout: 30m
  max_timeout: 24h
  cleanup_interval: 1m
  max_locks_per_owner: 100

  # 功能开关
  enable_persistence: true
  deadlock_detection: true
  auto_extend: true
  extend_interval: 5m

  # 超时策略
  grace_period: 30s
  wait_timeout: 5m

  # 分布式配置
  distributed:
    enabled: false
    backend: redis
    redis:
      addr: localhost:6379
      prefix: "nas-os:lock:"
```

---

**版本**: v1.0
**更新日期**: 2026-03-24
**作者**: NAS-OS 兵部