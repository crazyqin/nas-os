# 文件锁定机制设计

## 概述

参考群晖 DSM Drive 的文件锁定功能，设计多用户协作文件锁系统。基于现有 `internal/lock` 模块扩展，支持分布式场景、RBAC 集成和审计日志。

## 一、锁状态模型

### 1.1 锁类型

```
┌─────────────────────────────────────────────────────────────┐
│                      FileLock 状态机                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌──────────┐   Lock()    ┌──────────┐                    │
│   │  None    │ ──────────► │  Active  │                    │
│   └──────────┘             └────┬─────┘                    │
│                                 │                           │
│              ┌──────────────────┼──────────────────┐       │
│              │                  │                  │       │
│              ▼                  ▼                  ▼       │
│        ┌──────────┐      ┌──────────┐      ┌──────────┐    │
│        │ Expired  │      │ Released │      │ Breaking │    │
│        └──────────┘      └──────────┘      └──────────┘    │
│              │                                    │        │
│              └────────────────────────────────────┘        │
│                          自动清理                           │
└─────────────────────────────────────────────────────────────┘
```

| 类型 | 说明 | 兼容性 |
|------|------|--------|
| **Shared（共享锁）** | 读锁，多个用户可同时持有 | 与其他共享锁兼容 |
| **Exclusive（独占锁）** | 写锁，仅一个用户可持有 | 与任何锁都不兼容 |

### 1.2 锁状态

| 状态 | 说明 | 触发条件 |
|------|------|----------|
| `Active` | 锁活跃 | 成功获取锁 |
| `Expired` | 锁过期 | 超过过期时间 |
| `Released` | 锁释放 | 用户主动释放 |
| `Breaking` | 锁中断中 | 管理员强制释放（等待其他客户端确认） |

### 1.3 数据结构

```go
// FileLock 文件锁（扩展现有设计）
type FileLock struct {
    ID           string            `json:"id"`
    FilePath     string            `json:"filePath"`
    LockType     LockType          `json:"lockType"`
    Status       LockStatus        `json:"status"`
    
    // 持有者信息
    Owner        string            `json:"owner"`        // 用户ID
    OwnerName    string            `json:"ownerName"`    // 显示名称
    ClientID     string            `json:"clientId"`     // 客户端标识
    SessionID    string            `json:"sessionId"`    // 会话标识
    
    // 来源信息
    Protocol     string            `json:"protocol"`     // SMB/NFS/WebDAV/API
    ShareName    string            `json:"shareName"`    // 所属共享
    
    // 时间信息
    CreatedAt    time.Time         `json:"createdAt"`
    ExpiresAt    time.Time         `json:"expiresAt"`
    LastAccessed time.Time         `json:"lastAccessed"`
    
    // 扩展字段
    Priority     int               `json:"priority"`     // 锁优先级
    IsAdvisory   bool              `json:"isAdvisory"`   // 是否为建议锁
    Metadata     map[string]string `json:"metadata"`
    
    // 分布式支持
    NodeID       string            `json:"nodeId"`       // 持有锁的节点
    Version      int64             `json:"version"`      // 乐观锁版本号
}
```

## 二、锁协议设计

### 2.1 获取锁流程

```
┌────────┐                    ┌────────┐                    ┌────────┐
│ Client │                    │ Manager│                    │Storage │
└───┬────┘                    └───┬────┘                    └───┬────┘
    │                             │                             │
    │  1. LockRequest             │                             │
    │  {filePath, type, owner}    │                             │
    │────────────────────────────►│                             │
    │                             │                             │
    │                             │  2. 检查现有锁               │
    │                             │────────────────────────────►│
    │                             │                             │
    │                             │  3. 返回锁状态               │
    │                             │◄────────────────────────────│
    │                             │                             │
    │                             │  4. 冲突检测                 │
    │                             │  ┌───────────────┐          │
    │                             │  │ 无冲突/可升级  │          │
    │                             │  └───────┬───────┘          │
    │                             │          │                  │
    │                             │  5. 写入新锁                │
    │                             │────────────────────────────►│
    │                             │                             │
    │  6. LockResult              │                             │
    │  {id, expiresAt}            │                             │
    │◄────────────────────────────│                             │
    │                             │                             │
    │  7. 定期心跳（可选）          │                             │
    │────────────────────────────►│                             │
    │                             │                             │
```

### 2.2 锁请求参数

```go
// LockRequest 锁请求
type LockRequest struct {
    FilePath     string            `json:"filePath"`     // 必填：文件路径
    LockType     LockType          `json:"lockType"`     // 锁类型
    Owner        string            `json:"owner"`        // 必填：用户ID
    OwnerName    string            `json:"ownerName"`    // 显示名称
    ClientID     string            `json:"clientId"`     // 客户端标识
    SessionID    string            `json:"sessionId"`    // 会话标识
    Protocol     string            `json:"protocol"`     // 协议来源
    ShareName    string            `json:"shareName"`    // 共享名称
    Timeout      int               `json:"timeout"`      // 超时秒数
    Wait         bool              `json:"wait"`         // 是否等待
    WaitTimeout  int               `json:"waitTimeout"`  // 等待超时
    Priority     int               `json:"priority"`     // 优先级
    IsAdvisory   bool              `json:"isAdvisory"`   // 建议锁
    Metadata     map[string]string `json:"metadata"`     // 元数据
}
```

### 2.3 锁协议操作

| 操作 | 说明 | 权限要求 |
|------|------|----------|
| `Lock` | 获取锁 | 文件写权限 |
| `Unlock` | 释放锁 | 锁持有者 |
| `ForceUnlock` | 强制释放 | 管理员权限 |
| `Extend` | 延长锁 | 锁持有者 |
| `Upgrade` | 升级锁（共享→独占） | 锁持有者，无其他共享锁 |
| `Downgrade` | 降级锁（独占→共享） | 锁持有者 |
| `Break` | 中断锁（通知后释放） | 管理员权限 |

### 2.4 冲突检测规则

```go
// 检查锁兼容性
func CheckConflict(existing *FileLock, req *LockRequest) *LockConflict {
    // 1. 同一用户不冲突（可升级/续期）
    if existing.Owner == req.Owner {
        return nil
    }
    
    // 2. 过期锁不冲突
    if existing.IsExpired() {
        return nil
    }
    
    // 3. 独占锁与任何锁冲突
    if existing.LockType == LockTypeExclusive || 
       req.LockType == LockTypeExclusive {
        return &LockConflict{
            ExistingLock: existing,
            Message:      "file is exclusively locked",
        }
    }
    
    // 4. 共享锁之间兼容
    return nil
}
```

## 三、冲突解决策略

### 3.1 策略矩阵

| 场景 | 策略 | 说明 |
|------|------|------|
| 共享锁 + 共享锁请求 | 允许 | 多用户可同时读取 |
| 共享锁 + 独占锁请求 | 拒绝/等待 | 等待所有共享锁释放 |
| 独占锁 + 任何锁请求 | 拒绝/等待 | 等待独占锁释放 |
| 同用户锁升级 | 允许 | 共享→独占（无其他共享锁时） |
| 同用户锁降级 | 允许 | 独占→共享 |

### 3.2 等待队列

```
文件: /shared/project/report.docx

锁状态: Shared (用户A, 用户B, 用户C)

等待队列:
┌────────────────────────────────────────────────┐
│  1. 用户D - Exclusive (等待中, 排队时间: 30s)  │
│  2. 用户E - Shared (等待中, 排队时间: 15s)     │
└────────────────────────────────────────────────┘
```

### 3.3 公平性保证

1. **FIFO 队列**：请求按到达顺序排队
2. **优先级**：管理员请求可插队
3. **超时机制**：等待超时自动取消
4. **饥饿防止**：共享锁等待升级时，新共享锁请求需等待

### 3.4 锁抢占

```go
// 抢占策略
type PreemptPolicy struct {
    EnablePreempt     bool          // 允许抢占
    MinHoldTime       time.Duration // 最小持有时间（抢占保护期）
    PreemptNotice     time.Duration // 抢占通知时间
    PriorityThreshold int           // 抢占优先级阈值
}

// 抢占流程：
// 1. 高优先级请求到达
// 2. 检查是否超过最小持有时间
// 3. 发送通知给当前持有者
// 4. 等待 PreemptNotice 时间
// 5. 持有者释放或超时后强制释放
```

## 四、审计日志设计

### 4.1 审计事件类型

```go
// LockAuditEvent 锁审计事件
type LockAuditEvent struct {
    Timestamp  time.Time `json:"timestamp"`
    Event      string    `json:"event"`      // 事件类型
    LockID     string    `json:"lockId"`
    FilePath   string    `json:"filePath"`
    LockType   string    `json:"lockType"`
    
    // 用户信息
    UserID     string    `json:"userId"`
    Username   string    `json:"username"`
    ClientID   string    `json:"clientId"`
    
    // 结果
    Result     string    `json:"result"`     // success/failure
    Reason     string    `json:"reason"`     // 原因
    
    // 冲突信息
    ConflictWith string  `json:"conflictWith,omitempty"`
    
    // 管理员操作
    AdminID      string  `json:"adminId,omitempty"`
    AdminName    string  `json:"adminName,omitempty"`
    
    // 元数据
    IP          string   `json:"ip"`
    Protocol    string   `json:"protocol"`
    Duration    int64    `json:"duration"`   // 持续时间（秒）
}
```

### 4.2 审计事件清单

| 事件 | 触发条件 | 审计级别 |
|------|----------|----------|
| `lock_acquired` | 成功获取锁 | Info |
| `lock_released` | 主动释放锁 | Info |
| `lock_expired` | 锁自动过期 | Info |
| `lock_conflict` | 锁冲突被拒绝 | Warning |
| `lock_forced` | 管理员强制释放 | Warning |
| `lock_upgraded` | 锁升级成功 | Info |
| `lock_downgraded` | 锁降级成功 | Info |
| `lock_broken` | 锁被中断 | Warning |
| `lock_wait_timeout` | 等待超时 | Info |

### 4.3 审计存储

```go
// AuditStorage 审计存储接口
type AuditStorage interface {
    // Write 写入审计事件
    Write(ctx context.Context, event *LockAuditEvent) error
    
    // Query 查询审计日志
    Query(ctx context.Context, filter *AuditFilter) ([]*LockAuditEvent, error)
    
    // Rotate 日志轮转
    Rotate(ctx context.Context) error
}

// 实现选项：
// 1. 本地文件存储（JSON Lines）
// 2. 数据库存储（PostgreSQL/MySQL）
// 3. ELK 集成
// 4. SIEM 集成
```

## 五、分布式场景设计

### 5.1 架构

```
┌──────────────────────────────────────────────────────────────────┐
│                        分布式锁集群                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐                   │
│   │ Node A  │     │ Node B  │     │ Node C  │                   │
│   │ (主节点)│     │ (从节点)│     │ (从节点)│                   │
│   └────┬────┘     └────┬────┘     └────┬────┘                   │
│        │               │               │                         │
│        └───────────────┼───────────────┘                         │
│                        │                                         │
│              ┌─────────▼─────────┐                              │
│              │   分布式存储后端    │                              │
│              │ (Redis/etcd/DB)   │                              │
│              └───────────────────┘                              │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 5.2 分布式锁协议

#### 方案一：基于 Redis（推荐）

```go
// RedisLockStorage Redis 锁存储
type RedisLockStorage struct {
    client *redis.Client
    prefix string
}

// Lock 获取分布式锁（Redlock 算法简化版）
func (s *RedisLockStorage) Lock(ctx context.Context, lock *FileLock) error {
    key := s.prefix + ":lock:" + lock.FilePath
    
    // 使用 SET NX EX 原子操作
    ok, err := s.client.SetNX(ctx, key, lock.JSON(), 
        time.Until(lock.ExpiresAt)).Result()
    if err != nil {
        return err
    }
    if !ok {
        return ErrLockConflict
    }
    return nil
}

// Unlock 释放锁（Lua 脚本保证原子性）
func (s *RedisLockStorage) Unlock(ctx context.Context, lockID, owner string) error {
    script := `
        local val = redis.call('GET', KEYS[1])
        if val then
            local data = cjson.decode(val)
            if data.id == ARGV[1] and data.owner == ARGV[2] then
                redis.call('DEL', KEYS[1])
                return 1
            end
        end
        return 0
    `
    // ...
}
```

#### 方案二：基于数据库

```sql
-- 锁表设计
CREATE TABLE file_locks (
    id VARCHAR(36) PRIMARY KEY,
    file_path VARCHAR(1024) NOT NULL,
    lock_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    owner VARCHAR(100) NOT NULL,
    owner_name VARCHAR(100),
    client_id VARCHAR(100),
    protocol VARCHAR(20),
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    last_accessed TIMESTAMP NOT NULL,
    node_id VARCHAR(50),
    version BIGINT DEFAULT 1,
    metadata JSONB,
    
    UNIQUE INDEX idx_file_path (file_path),
    INDEX idx_owner (owner),
    INDEX idx_expires_at (expires_at)
);

-- 获取锁（乐观锁）
INSERT INTO file_locks (id, file_path, lock_type, ...)
SELECT $1, $2, $3, ...
WHERE NOT EXISTS (
    SELECT 1 FROM file_locks 
    WHERE file_path = $2 
    AND status = 'active' 
    AND expires_at > NOW()
);
```

### 5.3 一致性保证

```go
// 分布式锁管理器
type DistributedLockManager struct {
    local   *Manager           // 本地缓存
    storage LockStorage        // 持久化存储
    nodeID  string             // 当前节点ID
}

// 双层锁策略
func (m *DistributedLockManager) Lock(req *LockRequest) (*FileLock, error) {
    // 1. 先获取本地锁（快速失败）
    if conflict := m.localCheckConflict(req); conflict != nil {
        return nil, conflict
    }
    
    // 2. 获取分布式锁
    lock, err := m.storage.Lock(m.ctx, buildLock(req))
    if err != nil {
        return nil, err
    }
    
    // 3. 更新本地缓存
    m.local.Store(lock)
    
    return lock, nil
}
```

### 5.4 故障恢复

```
┌─────────────────────────────────────────────────────────┐
│                    故障恢复机制                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  节点故障:                                                │
│  ┌─────────────────────────────────────────────────┐   │
│  │ 1. 心跳检测超时                                   │   │
│  │ 2. 标记该节点持有的锁为可疑状态                    │   │
│  │ 3. 等待安全期（锁剩余时间/2）                      │   │
│  │ 4. 自动释放或迁移锁                               │   │
│  └─────────────────────────────────────────────────┘   │
│                                                          │
│  网络分区:                                                │
│  ┌─────────────────────────────────────────────────┐   │
│  │ 1. 多数派节点继续服务                             │   │
│  │ 2. 少数派节点进入只读模式                         │   │
│  │ 3. 分区恢复后同步状态                             │   │
│  └─────────────────────────────────────────────────┘   │
│                                                          │
│  存储故障:                                                │
│  ┌─────────────────────────────────────────────────┐   │
│  │ 1. 本地缓存继续服务读取请求                       │   │
│  │ 2. 写入请求降级为本地模式（标记）                  │   │
│  │ 3. 存储恢复后合并状态                             │   │
│  └─────────────────────────────────────────────────┘   │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## 六、RBAC 集成

### 6.1 权限模型扩展

```go
// 新增权限定义
var (
    PermFileLockRead   = Permission{
        Resource: "filelock", 
        Action: "read", 
        Desc: "查看文件锁状态",
    }
    PermFileLockWrite  = Permission{
        Resource: "filelock", 
        Action: "write", 
        Desc: "获取/释放文件锁",
        DependsOn: "filelock:read",
    }
    PermFileLockAdmin  = Permission{
        Resource: "filelock", 
        Action: "admin", 
        Desc: "强制释放锁/管理锁策略",
        DependsOn: "filelock:write",
    }
)
```

### 6.2 权限检查流程

```go
// LockWithAuth 带权限检查的锁操作
func (m *Manager) LockWithAuth(ctx context.Context, req *LockRequest) (*FileLock, error) {
    // 1. 获取用户信息
    userID := auth.GetUserID(ctx)
    
    // 2. 检查文件访问权限
    if !m.rbac.Check(userID, "file", "write", req.FilePath) {
        return nil, ErrPermissionDenied
    }
    
    // 3. 检查锁权限
    if !m.rbac.Check(userID, "filelock", "write", req.FilePath) {
        return nil, ErrPermissionDenied
    }
    
    // 4. 执行锁操作
    return m.Lock(req)
}

// ForceUnlockWithAuth 带权限检查的强制释放
func (m *Manager) ForceUnlockWithAuth(ctx context.Context, lockID string) error {
    userID := auth.GetUserID(ctx)
    
    // 需要管理员权限
    if !m.rbac.Check(userID, "filelock", "admin", "") {
        return ErrPermissionDenied
    }
    
    return m.ForceUnlock(lockID)
}
```

### 6.3 共享权限关联

```go
// 锁与共享 ACL 集成
func (m *Manager) checkShareAccess(req *LockRequest) error {
    // 获取文件所属共享
    share, err := m.getShareForPath(req.FilePath)
    if err != nil {
        return err
    }
    
    // 检查共享权限
    perm, err := m.shareACL.GetPermission(share.Name, req.Owner)
    if err != nil {
        return err
    }
    
    // 独占锁需要写权限
    if req.LockType == LockTypeExclusive {
        if perm.AccessLevel != AccessWrite && 
           perm.AccessLevel != AccessFull {
            return ErrPermissionDenied
        }
    }
    
    return nil
}
```

## 七、API 设计

### 7.1 REST API

```
POST   /api/v1/locks              # 获取锁
DELETE /api/v1/locks/:id          # 释放锁
PUT    /api/v1/locks/:id/extend   # 延长锁
PUT    /api/v1/locks/:id/upgrade  # 升级锁
PUT    /api/v1/locks/:id/downgrade # 降级锁
DELETE /api/v1/locks/:id/force    # 强制释放
GET    /api/v1/locks/:id          # 获取锁信息
GET    /api/v1/locks/path/*path   # 按路径查询
GET    /api/v1/locks              # 列出锁
GET    /api/v1/locks/check/*path  # 检查锁定状态
GET    /api/v1/locks/stats        # 统计信息
GET    /api/v1/locks/owner/:owner # 用户锁列表
```

### 7.2 WebSocket 通知

```go
// 锁事件通知
type LockNotification struct {
    Event     string    `json:"event"`
    LockID    string    `json:"lockId"`
    FilePath  string    `json:"filePath"`
    Owner     string    `json:"owner"`
    Timestamp time.Time `json:"timestamp"`
}

// 订阅文件锁事件
ws.send({
    "action": "subscribe",
    "files": ["/shared/project/report.docx"]
})

// 接收事件
ws.onmessage = (event) => {
    // lock_acquired, lock_released, lock_conflict, etc.
}
```

## 八、配置项

```yaml
# 锁配置
filelock:
  # 基本配置
  default_timeout: 30m        # 默认锁超时
  max_timeout: 24h            # 最大锁超时
  cleanup_interval: 5m        # 清理间隔
  
  # 分布式配置
  distributed: true           # 启用分布式模式
  backend: redis              # 存储后端: redis/etcd/database
  redis:
    addr: localhost:6379
    prefix: nas:lock
    pool_size: 10
  
  # 自动续期
  auto_renewal: true          # 启用自动续期
  renewal_interval: 10m       # 续期间隔
  renewal_before: 5m          # 提前续期时间
  
  # 等待队列
  enable_wait_queue: true     # 启用等待队列
  max_wait_time: 5m           # 最大等待时间
  
  # 抢占配置
  preempt:
    enabled: false            # 启用抢占
    min_hold_time: 5m         # 最小持有时间
    notice_time: 1m           # 抢占通知时间
  
  # 审计配置
  audit:
    enabled: true             # 启用审计
    storage: database         # 存储: database/file/elk
    retention: 30d            # 保留时间
```

## 九、监控指标

```go
// Prometheus 指标
var (
    // 锁总数
    LocksTotal = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nas_locks_total",
            Help: "Total number of locks",
        },
        []string{"type", "status"},
    )
    
    // 锁等待时间
    LockWaitDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "nas_lock_wait_duration_seconds",
            Help: "Lock wait duration",
        },
        []string{"type"},
    )
    
    // 锁冲突次数
    LockConflicts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nas_lock_conflicts_total",
            Help: "Total lock conflicts",
        },
        []string{"file_type"},
    )
    
    // 锁操作延迟
    LockOperationLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "nas_lock_operation_latency_seconds",
            Help: "Lock operation latency",
        },
        []string{"operation"},
    )
)
```

## 十、实现优先级

### Phase 1（已完成）
- [x] 基本锁类型（共享锁/独占锁）
- [x] 锁状态管理
- [x] REST API
- [x] SMB/NFS 适配器

### Phase 2（待实现）
- [ ] RBAC 集成
- [ ] 审计日志
- [ ] WebSocket 通知
- [ ] 等待队列

### Phase 3（待实现）
- [ ] 分布式支持（Redis）
- [ ] 锁升级/降级
- [ ] 抢占机制
- [ ] 监控指标

## 附录

### A. 与群晖 Drive 对比

| 特性 | 群晖 Drive | 本设计 |
|------|------------|--------|
| 锁类型 | 独占锁 | 独占锁 + 共享锁 |
| 锁范围 | 文件级 | 文件级 |
| 超时机制 | ✓ | ✓ |
| 自动续期 | ✓ | ✓ |
| 多用户提示 | ✓ | ✓ |
| 分布式 | ✓ | ✓（可选） |
| 审计日志 | 部分 | 完整 |

### B. 参考文档

- [RFC 3530 - NFSv4 锁机制](https://tools.ietf.org/html/rfc3530)
- [SMB2 锁协议](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/)
- [Redis Redlock 算法](https://redis.io/topics/distlock)
- 现有实现：`internal/lock/*.go`