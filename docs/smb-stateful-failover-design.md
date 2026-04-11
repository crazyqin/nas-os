# SMB Stateful Failover 架构设计文档

## 概述

本文档描述 nas-os SMB 有状态故障转移的架构设计，对标 TrueNAS 26 SMB Stateful Failover 功能。目标是实现 SMB 会话在 HA 集群节点故障时无缝迁移，客户端无需重新认证或重新建立连接。

## TrueNAS SMB Stateful Failover 技术分析

### 核心机制

TrueNAS 使用 **CTDB (Cluster TDB)** 实现 SMB 有状态故障转移：

1. **CTDB 集群守护进程**
   - 管理跨节点的 TDB 数据库复制
   - 提供集群成员管理和心跳检测
   - 协调故障转移时的节点角色切换

2. **SMB 状态持久化**
   - `sessionid.tdb` - SMB 会话 ID 映射
   - `connections.tdb` - SMB 连接状态
   - `locking.tdb` - 文件锁状态
   - `brlock.tdb` - byte-range locks
   - `openfiles.tdb` - 打开的文件句柄
   - `gencache.tdb` - 通用缓存

3. **共享存储要求**
   - 所有节点访问同一 ZFS 存储池
   - 使用共享的 `smb.conf` 配置
   - VIP（虚拟 IP）漂移机制

4. **故障转移流程**
   - CTDB 检测节点故障（心跳丢失）
   - 被动节点接管 VIP
   - 从 CTDB 复制的 TDB 恢复 SMB 状态
   - SMB 客户端自动重连到新节点（TCP 重连 + SMB 会话恢复）

### 关键技术要点

- **客户端无感知**: SMB3 支持 SMB2_SESSION_FLAG_IS_SESSION_EXPIRED 标记，客户端收到后自动重新认证
- **秒级恢复**: 正常情况下 < 5 秒完成故障转移
- **文件锁保持**: 跨节点文件锁状态同步，避免锁丢失

## nas-os 实现架构

### 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        HA Cluster                                │
│  ┌─────────────────────┐    ┌─────────────────────┐            │
│  │     Node A (Active) │    │    Node B (Passive) │            │
│  │                     │    │                     │            │
│  │  ┌───────────────┐  │    │  ┌───────────────┐  │            │
│  │  │  SMB Service  │  │    │  │  SMB Service  │  │            │
│  │  │  (smbd/nmbd)  │  │    │  │  (standby)    │  │            │
│  │  └───────────────┘  │    │  └───────────────┘  │            │
│  │         │           │    │         │           │            │
│  │  ┌───────────────┐  │    │  ┌───────────────┐  │            │
│  │  │SessionManager │  │◄──►│  │SessionManager │  │            │
│  │  │   (State)     │  │    │  │   (Sync)      │  │            │
│  │  └───────────────┘  │    │  └───────────────┘  │            │
│  │         │           │    │         │           │            │
│  │  ┌───────────────┐  │    │  ┌───────────────┐  │            │
│  │  │  HA Manager   │  │◄──►│  │  HA Manager   │  │            │
│  │  │ (Failover)    │  │    │  │ (Monitoring)  │  │            │
│  │  └───────────────┘  │    │  └───────────────┘  │            │
│  └─────────────────────┘    └─────────────────────┘            │
│           │                          │                          │
│           ▼                          ▼                          │
│  ┌─────────────────────────────────────────────────────────────┤
│  │              Shared State Store (Redis/SQLite)              │
│  │  - SMB Session States                                       │
│  │  - File Lock States                                         │
│  │  - Connection Metadata                                      │
│  └─────────────────────────────────────────────────────────────┤
│           │                          │                          │
│           ▼                          ▼                          │
│  ┌─────────────────────────────────────────────────────────────┤
│  │                   Shared ZFS Pool                           │
│  │  - SMB Shares                                               │
│  │  - User Data                                                │
│  └─────────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────────┘
                    │
                    ▼
              ┌─────────┐
              │  VIP    │
              │漂移机制 │
              └─────────┘
```

### 核心组件设计

#### 1. SMB Session State Manager

负责 SMB 会话状态的管理和持久化：

```go
// SMBSessionState SMB 会话状态
type SMBSessionState struct {
    SessionID      string            `json:"session_id"`
    UserID         string            `json:"user_id"`
    Username       string            `json:"username"`
    Domain         string            `json:"domain"`
    ClientIP       string            `json:"client_ip"`
    ConnectedAt    time.Time         `json:"connected_at"`
    LastActive     time.Time         `json:"last_active"`
    SessionKey     []byte            `json:"session_key"` // 加密存储
    Shares         []string          `json:"shares"`      // 已连接的共享
    OpenFiles      []OpenFileInfo    `json:"open_files"`
    Locks          []FileLockInfo    `json:"locks"`
    Credentials    *CredentialCache  `json:"credentials"` // 加密存储
    Metadata       map[string]string `json:"metadata"`
}

// SessionStateManager 会话状态管理器
type SessionStateManager struct {
    store       StateStore          // 持久化存储接口
    syncer      StateSyncer         // 状态同步器
    encryptor   Encryptor           // 加密模块
    localCache  map[string]*SMBSessionState
    eventHooks  []SessionEventHook
    mu          sync.RWMutex
    logger      *zap.Logger
}
```

**关键功能**：
- `CaptureSession()`: 从 smbstatus 获取当前会话状态
- `PersistSession()`: 将会话状态写入共享存储
- `RestoreSession()`: 从共享存储恢复会话状态
- `SyncSession()`: 跨节点同步会话状态
- `ValidateSession()`: 验证会话有效性

#### 2. State Sync Engine

跨节点状态同步引擎：

```go
// StateSyncEngine 状态同步引擎
type StateSyncEngine struct {
    grpcClient   *grpc.ClientConn
    grpcServer   *grpc.Server
    syncQueue    chan SyncRequest
    pendingSyncs map[string]*SyncRequest
    versionMap   map[string]int64   // 状态版本号
    config       SyncConfig
    mu           sync.RWMutex
    logger       *zap.Logger
}

// SyncRequest 同步请求
type SyncRequest struct {
    RequestID    string      `json:"request_id"`
    Type         SyncType    `json:"type"`
    StateID      string      `json:"state_id"`
    StateData    []byte      `json:"state_data"`
    Version      int64       `json:"version"`
    SourceNode   string      `json:"source_node"`
    TargetNode   string      `json:"target_node"`
    Timestamp    time.Time   `json:"timestamp"`
    Priority     int         `json:"priority"`
}

// SyncType 同步类型
type SyncType string
const (
    SyncTypeSession    SyncType = "session"     // 会话状态
    SyncTypeLock       SyncType = "lock"        // 文件锁
    SyncTypeConnection SyncType = "connection"  // 连接状态
    SyncTypeConfig     SyncType = "config"      // SMB 配置
)
```

**同步策略**：
- **实时同步**: 会话创建/销毁立即同步
- **增量同步**: 仅同步变化的状态数据
- **版本控制**: 使用乐观锁避免冲突
- **压缩传输**: 大状态数据压缩后传输

#### 3. Failover Controller Integration

集成现有的 `internal/ha/failover.go`:

```go
// SMBFailoverHook SMB 故障转移钩子
type SMBFailoverHook struct {
    sessionMgr  *SessionStateManager
    vipMgr      *VIPManager
    smbCtl      *SMBController
    logger      *zap.Logger
}

// 实现 FailoverHook 接口
func (h *SMBFailoverHook) OnPhaseStart(phase FailoverPhase) {
    switch phase {
    case PhaseSMBTransfer:
        // 1. 暂停新 SMB 连接
        h.smbCtl.PauseNewConnections()
        // 2. 刷新当前会话状态到共享存储
        h.sessionMgr.FlushAllSessions()
        // 3. 等待同步完成
        h.sessionMgr.WaitForSyncComplete()
        
    case PhaseTakeover:
        // 1. VIP 漂移到本节点
        h.vipMgr.AcquireVIP()
        // 2. 从共享存储恢复会话状态
        h.sessionMgr.RestoreAllSessions()
        // 3. 启动 SMB 服务（会话恢复模式）
        h.smbCtl.StartInRecoveryMode()
        // 4. 允许新连接
        h.smbCtl.ResumeNewConnections()
    }
}
```

### SMB 会话状态保持机制

#### 状态数据结构

```go
// OpenFileInfo 打开的文件信息
type OpenFileInfo struct {
    FileID       uint32    `json:"file_id"`
    ShareName    string    `json:"share_name"`
    Path         string    `json:"path"`
    Handle       uint32    `json:"handle"`
    AccessMode   uint32    `json:"access_mode"`
    ShareMode    uint32    `json:"share_mode"`
    Position     uint64    `json:"position"`
    OpenTime     time.Time `json:"open_time"`
}

// FileLockInfo 文件锁信息
type FileLockInfo struct {
    LockID       string    `json:"lock_id"`
    FileID       uint32    `json:"file_id"`
    SessionID    string    `json:"session_id"`
    StartOffset  uint64    `json:"start_offset"`
    Length       uint64    `json:"length"`
    LockType     uint32    `json:"lock_type"` // 读锁/写锁
    GrantedTime  time.Time `json:"granted_time"`
}

// CredentialCache 凭证缓存
type CredentialCache struct {
    NTHash       []byte    `json:"nt_hash"`     // 加密存储
    LMHash       []byte    `json:"lm_hash"`     // 加密存储
    SessionKey   []byte    `json:"session_key"` // 加密存储
    ValidUntil   time.Time `json:"valid_until"`
}
```

#### 状态捕获方法

```go
// 从 smbstatus/netstat 捕获当前状态
func (m *SessionStateManager) CaptureCurrentState() (*ClusterState, error) {
    // 1. 执行 smbstatus 获取会话列表
    sessions, err := m.parseSMBStatus()
    if err != nil {
        return nil, err
    }
    
    // 2. 解析打开文件列表
    openFiles, err := m.parseOpenFiles()
    if err != nil {
        return nil, err
    }
    
    // 3. 解析文件锁列表
    locks, err := m.parseFileLocks()
    if err != nil {
        return nil, err
    }
    
    // 4. 构建完整状态
    return &ClusterState{
        Sessions:   sessions,
        OpenFiles:  openFiles,
        Locks:      locks,
        CapturedAt: time.Now(),
        NodeID:     m.config.NodeID,
    }, nil
}
```

### HA 集群故障转移流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    SMB Failover Workflow                         │
└─────────────────────────────────────────────────────────────────┘

Phase 1: 故障检测 (Detection)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
┌────────────────┐
│ Primary Node A │
│  - Heartbeat ◄─│───────► Timeout detected
│  - SMB Active  │
└────────────────┘
        │
        ▼ Phase 2: 确认 (Confirmation)

┌────────────────┐
│   HA Manager   │
│  - Phi detect │ ──► Phi > threshold (8.0)
│  - Retry check│ ──► 3 retries failed
└────────────────┘
        │
        ▼ Phase 3: SMB 状态转移 (Transfer)

┌────────────────────────────────────────────────────┐
│              Passive Node B Preparation             │
│                                                     │
│  1. [SMB_PAUSE]  暂停新连接入                       │
│  2. [STATE_FLUSH] 主节点 flush 最后状态             │
│     - smbstatus --flush                            │
│     - sync session.tdb/locking.tdb                 │
│  3. [SYNC_WAIT] 等待状态同步完成                    │
│  4. [VIP_ACQUIRE] 获取虚拟 IP                       │
│     - arping -U -c 5 VIP                           │
│     - ip addr add VIP/24 dev eth0                  │
└────────────────────────────────────────────────────┘
        │
        ▼ Phase 4: 服务接管 (Takeover)

┌────────────────────────────────────────────────────┐
│              Passive Node B Takeover                │
│                                                     │
│  1. [STATE_RESTORE] 恢复 SMB 状态                   │
│     - Load sessions from shared store              │
│     - Recreate open file handles                   │
│     - Re-establish file locks                      │
│  2. [SMB_RESTART] 启动 SMB 服务                     │
│     - smbd --resume-session                        │
│     - nmbd                                         │
│  3. [DNS_UPDATE] 更新 DNS（可选）                   │
│     - nsupdate VIP A record                        │
│  4. [UNPAUSE] 允许新连接                           │
└────────────────────────────────────────────────────┘
        │
        ▼ Phase 5: 验证 (Verification)

┌────────────────────────────────────────────────────┐
│              Service Verification                   │
│                                                     │
│  1. [SMB_CHECK] SMB 服务健康检查                    │
│     - smbclient -L localhost                       │
│     - test share accessibility                     │
│  2. [CLIENT_TEST] 客户端连接测试                    │
│     - Verify existing clients can reconnect        │
│  3. [HEALTH_SCORE] 更新节点健康评分                 │
└────────────────────────────────────────────────────┘
        │
        ▼ Phase 6: 清理 (Cleanup)

┌────────────────────────────────────────────────────┐
│              Cleanup Tasks                          │
│                                                     │
│  1. [OLD_NODE_CLEAN] 标记旧主节点状态               │
│  2. [METRICS_UPDATE] 更新监控指标                   │
│  3. [EVENT_LOG] 记录故障转移事件                    │
└────────────────────────────────────────────────────┘
        │
        ▼ Phase 7: 完成 (Complete)

┌────────────────────────────────────────────────────┐
│          Failover Complete (< 5 seconds)            │
│                                                     │
│  - Node B is new Primary                           │
│  - VIP active on Node B                            │
│  - SMB clients reconnected                         │
│  - File locks preserved                            │
└────────────────────────────────────────────────────┘
```

### 会话恢复与重连策略

#### SMB3 客户端重连机制

SMB3 协议支持会话恢复：

1. **TCP 层重连**
   - 客户端检测到 TCP 连接断开
   - 尝试重新连接到 VIP（已漂移到新节点）

2. **SMB 会话恢复**
   - 客户端发送 SMB2_NEGOTIATE
   - 服务器返回 SMB2_SESSION_FLAG_IS_SESSION_EXPIRED
   - 客户端使用缓存的 SessionKey 重新认证
   - 服务器验证 SessionKey，恢复会话

3. **文件句柄恢复**
   - 客户端使用 Persistent File ID
   - 服务器从持久化状态恢复文件句柄
   - 继续之前的文件操作

#### 会话恢复代码实现

```go
// SMBRecoveryHandler SMB 会话恢复处理器
type SMBRecoveryHandler struct {
    sessionStore SessionStore
    smbService   SMBService
}

// HandleReconnect 处理客户端重连
func (h *SMBRecoveryHandler) HandleReconnect(clientIP string, sessionID string) (*RecoveryResult, error) {
    // 1. 查找会话状态
    session, err := h.sessionStore.GetSession(sessionID)
    if err != nil {
        return nil, ErrSessionNotFound
    }
    
    // 2. 验证会话有效性
    if time.Since(session.LastActive) > h.config.SessionTimeout {
        h.sessionStore.DeleteSession(sessionID)
        return nil, ErrSessionExpired
    }
    
    // 3. 恢复会话
    if err := h.smbService.RestoreSession(session); err != nil {
        return nil, err
    }
    
    // 4. 恢复打开的文件
    for _, file := range session.OpenFiles {
        if err := h.smbService.RestoreFileHandle(file); err != nil {
            log.Warnf("failed to restore file %s: %v", file.Path, err)
        }
    }
    
    // 5. 恢复文件锁
    for _, lock := range session.Locks {
        if err := h.smbService.RestoreLock(lock); err != nil {
            log.Warnf("failed to restore lock %s: %v", lock.LockID, err)
        }
    }
    
    return &RecoveryResult{
        SessionID:    sessionID,
        FilesRestored: len(session.OpenFiles),
        LocksRestored: len(session.Locks),
        RecoveredAt:  time.Now(),
    }, nil
}
```

### 存储层设计

#### 共享状态存储接口

```go
// StateStore 状态存储接口
type StateStore interface {
    // 会话操作
    SaveSession(session *SMBSessionState) error
    GetSession(sessionID string) (*SMBSessionState, error)
    DeleteSession(sessionID string) error
    ListSessions() ([]*SMBSessionState, error)
    
    // 文件锁操作
    SaveLock(lock *FileLockInfo) error
    GetLock(lockID string) (*FileLockInfo, error)
    DeleteLock(lockID string) error
    ListLocks() ([]*FileLockInfo, error)
    
    // 打开文件操作
    SaveOpenFile(file *OpenFileInfo) error
    GetOpenFile(fileID uint32) (*OpenFileInfo, error)
    DeleteOpenFile(fileID uint32) error
    ListOpenFiles() ([]*OpenFileInfo, error)
    
    // 批量操作
    BatchSave(states map[string]interface{}) error
    BatchDelete(ids []string) error
    
    // 事务支持
    BeginTx() (StateTx, error)
}

// StateTx 状态事务接口
type StateTx interface {
    Commit() error
    Rollback() error
    SaveSession(session *SMBSessionState) error
    DeleteSession(sessionID string) error
}
```

#### Redis 实现（推荐）

```go
// RedisStateStore Redis 状态存储实现
type RedisStateStore struct {
    client     *redis.Client
    keyPrefix  string
    ttl        time.Duration
    encryptor  Encryptor
}

func (r *RedisStateStore) SaveSession(session *SMBSessionState) error {
    key := r.keyPrefix + "session:" + session.SessionID
    
    // 加密敏感数据
    encrypted, err := r.encryptor.Encrypt(session)
    if err != nil {
        return err
    }
    
    return r.client.Set(r.ctx, key, encrypted, r.ttl).Err()
}

func (r *RedisStateStore) GetSession(sessionID string) (*SMBSessionState, error) {
    key := r.keyPrefix + "session:" + sessionID
    
    data, err := r.client.Get(r.ctx, key).Bytes()
    if err != nil {
        return nil, err
    }
    
    return r.encryptor.Decrypt(data)
}
```

#### SQLite 实现（备选）

```go
// SQLiteStateStore SQLite 状态存储实现
type SQLiteStateStore struct {
    db        *sql.DB
    encryptor Encryptor
}

// 数据库表结构
CREATE TABLE smb_sessions (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    session_key_encrypted BLOB NOT NULL,
    connected_at DATETIME NOT NULL,
    last_active DATETIME NOT NULL,
    metadata JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE smb_open_files (
    file_id INTEGER PRIMARY KEY,
    session_id TEXT NOT NULL,
    share_name TEXT NOT NULL,
    path TEXT NOT NULL,
    handle INTEGER NOT NULL,
    access_mode INTEGER,
    share_mode INTEGER,
    position INTEGER,
    open_time DATETIME,
    FOREIGN KEY (session_id) REFERENCES smb_sessions(session_id)
);

CREATE TABLE smb_file_locks (
    lock_id TEXT PRIMARY KEY,
    file_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    start_offset INTEGER NOT NULL,
    length INTEGER NOT NULL,
    lock_type INTEGER NOT NULL,
    granted_time DATETIME,
    FOREIGN KEY (file_id) REFERENCES smb_open_files(file_id),
    FOREIGN KEY (session_id) REFERENCES smb_sessions(session_id)
);
```

### VIP 漂移机制

```go
// VIPManager VIP 管理器
type VIPManager struct {
    vip        string
    interface  string
    acquired   bool
    arpUtil    ARPUtil
    netLink    NetLink
    logger     *zap.Logger
    mu         sync.Mutex
}

// AcquireVIP 获取 VIP
func (v *VIPManager) AcquireVIP() error {
    v.mu.Lock()
    defer v.mu.Unlock()
    
    if v.acquired {
        return nil
    }
    
    // 1. 添加 IP 地址
    if err := v.netLink.AddIP(v.vip, v.interface); err != nil {
        return err
    }
    
    // 2. 发送 ARP 通告
    for i := 0; i < 5; i++ {
        v.arpUtil.SendARPAnnounce(v.vip, v.interface)
        time.Sleep(100 * time.Millisecond)
    }
    
    v.acquired = true
    v.logger.Info("VIP acquired", zap.String("vip", v.vip))
    
    return nil
}

// ReleaseVIP 释放 VIP
func (v *VIPManager) ReleaseVIP() error {
    v.mu.Lock()
    defer v.mu.Unlock()
    
    if !v.acquired {
        return nil
    }
    
    // 1. 删除 IP 地址
    if err := v.netLink.DeleteIP(v.vip, v.interface); err != nil {
        return err
    }
    
    v.acquired = false
    v.logger.Info("VIP released", zap.String("vip", v.vip))
    
    return nil
}
```

### 安全考量

#### 1. 状态数据加密

敏感数据必须加密存储：
- SessionKey
- NT/LM Hash
- 用户凭证

```go
// Encryptor 加密接口
type Encryptor interface {
    Encrypt(data []byte) ([]byte, error)
    Decrypt(data []byte) ([]byte, error)
    EncryptString(s string) ([]byte, error)
    DecryptString(data []byte) (string, error)
}

// AES256GCMEncryptor AES-256-GCM 加密实现
type AES256GCMEncryptor struct {
    key []byte  // 从安全存储获取
}
```

#### 2. 跨节点认证

```go
// NodeAuthenticator 节点认证器
type NodeAuthenticator interface {
    AuthenticateNode(nodeID string, token string) error
    GenerateToken(nodeID string) (string, error)
    ValidateToken(token string) (string, error)
}

// 使用 TLS + JWT 实现
type TLSNodeAuthenticator struct {
    caCert     *x509.Certificate
    jwtSecret  []byte
}
```

#### 3. 权限一致性

故障转移后权限必须保持一致：
- ZFS ACL 同步
- SMB 权限映射一致
- 用户/组数据库同步

### 性能目标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| 故障检测延迟 | < 2秒 | Phi 检测器阈值 8.0 |
| 状态同步延迟 | < 500ms | 实时同步，增量传输 |
| VIP 漂移延迟 | < 1秒 | ARP 通告完成 |
| SMB 恢复延迟 | < 3秒 | 会话恢复完成 |
| 总故障转移时间 | < 5秒 | 完整流程 |
| 客户端重连成功率 | > 95% | SMB3 客户端 |

### 实现里程碑

| 阶段 | 内容 | 完成日期 | 状态 |
|------|------|----------|------|
| M1 | SessionStateManager 原型 | 2026-04-15 | 🟡 设计中 |
| M2 | StateSyncEngine (gRPC) | 2026-04-20 | 🔴 待开始 |
| M3 | VIPManager 集成 | 2026-04-22 | 🔴 待开始 |
| M4 | FailoverHook 实现 | 2026-04-25 | 🔴 待开始 |
| M5 | 存储层实现 (Redis) | 2026-04-28 | 🔴 待开始 |
| M6 | 故障转移测试 | 2026-04-30 | 🔴 待开始 |
| M7 | 生产验证 | 2026-05-05 | 🔴 待开始 |

### 与现有模块集成点

| 模块 | 集成方式 | 说明 |
|------|----------|------|
| `internal/ha/ha.go` | 已集成 | HA Manager 提供集群管理 |
| `internal/ha/failover.go` | 已集成 | FailoverController 提供故障转移框架 |
| `internal/ha/sync.go` | 待扩展 | StateSyncer 扩展 SMB 状态同步 |
| `internal/audit/enhanced/smb_audit.go` | 待集成 | SMB 审计事件触发状态更新 |
| `internal/webshare/webshare.go` | 待集成 | 共享配置同步 |
| `internal/smb` | 待创建 | SMB 服务控制模块 |

### API 设计

```go
// SMBHAService SMB HA 服务接口
type SMBHAService interface {
    // 会话管理
    GetSessions() ([]*SMBSessionState, error)
    GetSession(sessionID string) (*SMBSessionState, error)
    FlushSessions() error
    
    // 故障转移
    TriggerFailover() error
    GetFailoverStatus() (*FailoverStatus, error)
    
    // 状态同步
    SyncState() error
    GetSyncStatus() (*SyncStatus, error)
    
    // VIP 管理
    GetVIPStatus() (*VIPStatus, error)
}
```

### 测试策略

#### 单元测试

```go
func TestSessionStateCapture(t *testing.T) {
    // 测试会话状态捕获
}

func TestStateSync(t *testing.T) {
    // 测试状态同步
}

func TestSessionRecovery(t *testing.T) {
    // 测试会话恢复
}
```

#### 集成测试

```go
func TestFailoverWorkflow(t *testing.T) {
    // 1. 创建双节点集群
    // 2. 建立 SMB 连接
    // 3. 触发故障转移
    // 4. 验证会话恢复
}
```

#### 性能测试

```go
func TestFailoverLatency(t *testing.T) {
    // 测试故障转移延迟 < 5s
}

func TestSyncThroughput(t *testing.T) {
    // 测试状态同步吞吐量
}
```

---

**文档版本**: v1.0
**创建日期**: 2026-04-11
**负责部门**: 兵部
**文档状态**: 设计完成，待实现