# NAS-OS 新功能架构设计文档

## 版本：v2.260.0
## 日期：2026-03-24
## 作者：兵部

---

## 一、概述

基于竞品分析，本文档设计两大核心功能架构：
1. **内置内网穿透服务**（类似飞牛FN Connect）
2. **智能分层存储增强**（类似群晖Synology Tiering）

---

## 二、内置内网穿透架构设计

### 2.1 设计目标

对标飞牛fnOS的FN Connect功能，实现：
- 零配置远程访问
- 多种连接模式（P2P/中继/反向代理）
- 高可用性和稳定性
- 安全的数据传输

### 2.2 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        NAS-OS 内网穿透服务                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   Web UI     │  │   REST API   │  │  WebSocket   │           │
│  │  管理界面     │  │   接口层     │  │   实时推送   │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
│         │                 │                 │                    │
│         └─────────────────┼─────────────────┘                    │
│                           │                                      │
│  ┌────────────────────────┴────────────────────────┐             │
│  │              Tunnel Manager (核心)               │             │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────┐ │             │
│  │  │ P2P     │ │ Relay   │ │ Reverse │ │ Auto   │ │             │
│  │  │ Client  │ │ Client  │ │ Client  │ │ Client │ │             │
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └───┬────┘ │             │
│  └───────┼───────────┼───────────┼──────────┼──────┘             │
│          │           │           │          │                     │
│  ┌───────┴───────────┴───────────┴──────────┴──────┐             │
│  │              Connection Layer                    │             │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │             │
│  │  │ STUN     │ │ TURN     │ │ Tunnel Protocol  │ │             │
│  │  │ (NAT检测) │ │ (中继)   │ │ (自研/frp/nps)   │ │             │
│  │  └──────────┘ └──────────┘ └──────────────────┘ │             │
│  └─────────────────────────────────────────────────┘             │
│                           │                                      │
│  ┌────────────────────────┴────────────────────────┐             │
│  │              Security Layer                      │             │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │             │
│  │  │ TLS/DTLS │ │ Token    │ │ Rate Limiting    │ │             │
│  │  │ 加密传输  │ │ 认证     │ │ 访问控制        │ │             │
│  │  └──────────┘ └──────────┘ └──────────────────┘ │             │
│  └─────────────────────────────────────────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.3 模块划分

#### 2.3.1 Tunnel Manager（隧道管理器）

**职责**：
- 隧道生命周期管理（创建、连接、断开、销毁）
- 多隧道并发管理
- 状态监控与事件通知

**核心接口**：
```go
// internal/tunnel/manager.go

type Manager interface {
    // 生命周期
    Start(ctx context.Context) error
    Stop() error
    
    // 隧道管理
    Connect(ctx context.Context, req ConnectRequest) (*ConnectResponse, error)
    Disconnect(tunnelID string) error
    ListTunnels() []TunnelStatus
    GetTunnelStatus(tunnelID string) (TunnelStatus, error)
    
    // 事件订阅
    OnEvent(callback EventCallback)
}
```

#### 2.3.2 Client Layer（客户端层）

**三种客户端模式**：

| 模式 | 适用场景 | 技术方案 |
|------|---------|---------|
| P2P | NAT穿透成功时 | STUN打洞 + UDP直连 |
| Relay | NAT穿透失败时 | TURN中继 + TCP转发 |
| Reverse | 服务端主动连接 | 反向代理 + WebSocket |

**P2P Client**：
```go
// internal/tunnel/p2p_client.go

type P2PClient struct {
    config      TunnelConfig
    stunServers []string
    conn        *net.UDPConn
    peerAddr    *net.UDPAddr
}

func (c *P2PClient) Connect(ctx context.Context) error {
    // 1. STUN探测公网地址
    // 2. 交换peer信息
    // 3. 尝试UDP打洞
    // 4. 建立P2P连接
}
```

**Relay Client**：
```go
// internal/tunnel/relay_client.go

type RelayClient struct {
    config    TunnelConfig
    turnAddr  string
    relayConn net.Conn
}

func (c *RelayClient) Connect(ctx context.Context) error {
    // 1. 连接TURN服务器
    // 2. 分配中继地址
    // 3. 创建权限
    // 4. 绑定通道
}
```

#### 2.3.3 NAT Detection（NAT检测）

**NAT类型分级**：

```
NAT类型            穿透难度    P2P成功率
─────────────────────────────────────
None (公网IP)      低          100%
Full Cone          低          95%
Restricted Cone    中          80%
Port Restricted    中          60%
Symmetric          高          20%
```

**检测流程**：
```go
// internal/tunnel/nat_detector.go

type NATDetector interface {
    Detect(ctx context.Context) (NATType, string, int, error)
}

// RFC 3489 测试流程
func (d *STUNDetector) Detect(ctx context.Context) (NATType, string, int, error) {
    // Test I: 获取映射地址
    // Test II: 检测是否开放
    // Test III: 检测是否对称NAT
    // 返回NAT类型和公网地址
}
```

#### 2.3.4 Security Layer（安全层）

**安全机制**：

```yaml
认证方式:
  - Token认证（设备唯一标识 + 密钥）
  - TLS双向认证（mTLS）
  - 签名验证（HMAC-SHA256）

传输加密:
  - TLS 1.3（TCP）
  - DTLS 1.2（UDP）
  - 端到端加密（可选）

访问控制:
  - IP白名单
  - 端口级别权限
  - 速率限制
```

### 2.4 API设计

#### 2.4.1 REST API

```yaml
# 隧道管理
POST   /api/v1/tunnel/connect      # 建立隧道
DELETE /api/v1/tunnel/{id}         # 断开隧道
GET    /api/v1/tunnel              # 列出隧道
GET    /api/v1/tunnel/{id}         # 隧道状态

# 配置管理
GET    /api/v1/tunnel/config       # 获取配置
PUT    /api/v1/tunnel/config       # 更新配置

# 状态监控
GET    /api/v1/tunnel/status       # 全局状态
GET    /api/v1/tunnel/metrics      # 统计指标
```

#### 2.4.2 API请求/响应示例

**建立隧道**：
```json
// POST /api/v1/tunnel/connect
// Request
{
  "name": "web-ui",
  "mode": "auto",
  "local_port": 5000,
  "remote_port": 0,
  "protocol": "tcp",
  "description": "NAS Web管理界面"
}

// Response
{
  "code": 0,
  "message": "success",
  "data": {
    "tunnel_id": "a1b2c3d4e5f6",
    "name": "web-ui",
    "mode": "p2p",
    "state": "connecting",
    "local_addr": "127.0.0.1:5000",
    "public_addr": "203.0.113.50:12345",
    "message": "正在建立P2P连接..."
  }
}
```

### 2.5 关键技术选型

| 组件 | 技术选型 | 理由 |
|------|---------|------|
| STUN/TURN | pion/stun, pion/turn | Go原生实现，WebRTC兼容 |
| P2P协议 | WebRTC DataChannel | 成熟的P2P方案 |
| 中继协议 | frp / 自研 | frp成熟稳定，可自建服务器 |
| 服务发现 | 内置注册中心 | 简化部署，降低依赖 |
| 数据序列化 | Protobuf | 高效二进制协议 |

### 2.6 部署架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        云端服务（可选）                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   STUN/TURN  │  │   Relay      │  │   Registry   │           │
│  │   服务器集群   │  │   服务器集群  │  │   注册中心    │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ 互联网
                              │
┌─────────────────────────────────────────────────────────────────┐
│                        NAS设备（用户侧）                          │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    NAS-OS Tunnel Service                     │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐   │ │
│  │  │ Web:5000│ │SMB:445  │ │NFS:2049 │ │ 其他服务端口...   │   │ │
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────────┬────────┘   │ │
│  │       │           │           │                 │            │ │
│  │       └───────────┴───────────┴─────────────────┘            │ │
│  │                           │                                  │ │
│  │                    Tunnel Manager                            │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## 三、智能分层存储架构设计

### 3.1 设计目标

对标群晖Synology Tiering功能，实现：
- 基于访问模式的热/温/冷数据自动识别
- 多存储层智能迁移（SSD/HDD/Cloud）
- 策略引擎支持自定义规则
- 数据完整性保证

### 3.2 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     NAS-OS 智能分层存储系统                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   Web UI     │  │   REST API   │  │   Schedule   │           │
│  │  管理界面     │  │   接口层     │  │   调度器     │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
│         │                 │                 │                    │
│         └─────────────────┼─────────────────┘                    │
│                           │                                      │
│  ┌────────────────────────┴────────────────────────┐             │
│  │            Tiering Manager (核心)                │             │
│  │                                                  │             │
│  │  ┌──────────────┐  ┌──────────────┐             │             │
│  │  │ Policy Engine│  │ Access Tracker│             │             │
│  │  │  策略引擎    │  │  访问追踪器   │             │             │
│  │  └──────┬───────┘  └──────┬───────┘             │             │
│  │         │                 │                      │             │
│  │  ┌──────┴─────────────────┴───────┐             │             │
│  │  │        Migration Engine        │             │             │
│  │  │          迁移引擎               │             │             │
│  │  └────────────────┬───────────────┘             │             │
│  └───────────────────┼─────────────────────────────┘             │
│                      │                                          │
│  ┌───────────────────┴─────────────────────────────┐             │
│  │              Storage Layer                       │             │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐            │             │
│  │  │  SSD    │ │  HDD    │ │  Cloud  │            │             │
│  │  │  热数据  │ │  温/冷  │ │  归档   │            │             │
│  │  │ Tier-0  │ │ Tier-1  │ │ Tier-2  │            │             │
│  │  └─────────┘ └─────────┘ └─────────┘            │             │
│  └─────────────────────────────────────────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3.3 模块划分

#### 3.3.1 Tiering Manager（分层管理器）

**职责**：
- 存储层配置管理
- 策略生命周期管理
- 协调各子模块工作

**核心接口**：
```go
// internal/tiering/manager.go

type Manager interface {
    // 生命周期
    Initialize() error
    Shutdown() error
    
    // 存储层管理
    CreateTier(tierType TierType, config TierConfig) error
    GetTier(tierType TierType) (*TierConfig, error)
    UpdateTier(tierType TierType, config TierConfig) error
    DeleteTier(tierType TierType) error
    ListTiers() []*TierConfig
    
    // 策略管理
    CreatePolicy(policy Policy) (*Policy, error)
    GetPolicy(id string) (*Policy, error)
    UpdatePolicy(id string, policy Policy) error
    DeletePolicy(id string) error
    ListPolicies() []*Policy
    
    // 手动迁移
    Migrate(req MigrateRequest) (*MigrateTask, error)
    
    // 状态查询
    GetStatus() Status
    GetStats() *StatsReport
}
```

#### 3.3.2 Access Tracker（访问追踪器）

**追踪指标**：

| 指标 | 说明 | 用途 |
|------|------|------|
| 访问次数 | 文件被读取的次数 | 热度判断 |
| 访问间隔 | 距上次访问的时间 | 冷度判断 |
| 读取字节数 | 累计读取数据量 | IO热点分析 |
| 写入字节数 | 累计写入数据量 | 写入模式分析 |
| 文件大小 | 文件大小 | 迁移优先级 |

**追踪实现**：
```go
// internal/tiering/tracker.go

type AccessTracker struct {
    config   StatisticsConfig
    records  map[string]*FileAccessRecord
    hotMap   *lru.Cache  // 热数据缓存
    db       *sql.DB     // 持久化存储
}

func (t *AccessTracker) TrackAccess(path string, op Operation) {
    record := t.getOrCreate(path)
    record.AccessCount++
    record.AccessTime = time.Now()
    
    // 更新热度
    frequency := t.calculateFrequency(record)
    record.Frequency = frequency
    
    // 持久化
    t.persist(record)
}

func (t *AccessTracker) calculateFrequency(record *FileAccessRecord) AccessFrequency {
    age := time.Since(record.AccessTime)
    
    // 热数据：最近7天内访问超过100次
    if age < 7*24*time.Hour && record.AccessCount > 100 {
        return AccessFrequencyHot
    }
    
    // 冷数据：超过30天未访问
    if age > 30*24*time.Hour {
        return AccessFrequencyCold
    }
    
    return AccessFrequencyWarm
}
```

#### 3.3.3 Policy Engine（策略引擎）

**策略类型**：

```yaml
# 时间策略
time_based:
  description: "基于时间的分层"
  rules:
    - condition: "access_age > 30 days"
      action: "move"
      target: "hdd"
    - condition: "access_age > 180 days"
      action: "archive"
      target: "cloud"

# 频率策略
frequency_based:
  description: "基于访问频率的分层"
  rules:
    - condition: "access_count > 100 AND access_age < 7 days"
      action: "move"
      target: "ssd"
    - condition: "access_count < 10 AND access_age > 14 days"
      action: "move"
      target: "hdd"

# 大小策略
size_based:
  description: "基于文件大小的分层"
  rules:
    - condition: "file_size < 1MB"
      action: "move"
      target: "ssd"
    - condition: "file_size > 1GB"
      action: "move"
      target: "hdd"

# 组合策略
hybrid:
  description: "组合策略"
  rules:
    - condition: "access_count > 50 AND file_size < 100MB"
      action: "copy"  # 保留副本在SSD
      target: "ssd"
```

**策略执行**：
```go
// internal/tiering/policy_engine.go

type PolicyEngine struct {
    config   PolicyEngineConfig
    tracker  *AccessTracker
    migrator *Migrator
    rules    []PolicyRule
}

func (e *PolicyEngine) Run(ctx context.Context) error {
    // 1. 扫描所有文件
    files := e.scanFiles()
    
    // 2. 评估每个文件
    for _, file := range files {
        record := e.tracker.GetRecord(file.Path)
        
        // 匹配策略
        for _, policy := range e.policies {
            if e.matchPolicy(record, policy) {
                // 创建迁移任务
                task := e.createMigrationTask(file, policy)
                e.migrator.Submit(task)
            }
        }
    }
    
    return nil
}

func (e *PolicyEngine) matchPolicy(record *FileAccessRecord, policy *Policy) bool {
    // 检查访问次数
    if policy.MinAccessCount > 0 && record.AccessCount < policy.MinAccessCount {
        return false
    }
    
    // 检查访问间隔
    if policy.MaxAccessAge > 0 {
        age := time.Since(record.AccessTime)
        if age > policy.MaxAccessAge {
            return false
        }
    }
    
    // 检查文件大小
    if policy.MinFileSize > 0 && record.Size < policy.MinFileSize {
        return false
    }
    if policy.MaxFileSize > 0 && record.Size > policy.MaxFileSize {
        return false
    }
    
    // 检查文件模式
    if len(policy.FilePatterns) > 0 {
        matched := false
        for _, pattern := range policy.FilePatterns {
            if match, _ := filepath.Match(pattern, record.Path); match {
                matched = true
                break
            }
        }
        if !matched {
            return false
        }
    }
    
    return true
}
```

#### 3.3.4 Migration Engine（迁移引擎）

**迁移流程**：
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   扫描文件   │ -> │  检查权限   │ -> │  数据复制   │ -> │  验证完整性  │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                                                │
                                                                v
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   更新索引   │ <- │  清理源文件  │ <- │  更新元数据  │ <- │  迁移完成   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

**迁移实现**：
```go
// internal/tiering/migrator.go

type Migrator struct {
    config   PolicyEngineConfig
    queue    chan *MigrateTask
    workers  int
    active   sync.Map
}

func (m *Migrator) Start() {
    for i := 0; i < m.workers; i++ {
        go m.worker(i)
    }
}

func (m *Migrator) worker(id int) {
    for task := range m.queue {
        m.executeTask(task)
    }
}

func (m *Migrator) executeTask(task *MigrateTask) error {
    task.Status = MigrateStatusRunning
    task.StartedAt = time.Now()
    
    for _, file := range task.Files {
        // 1. 检查源文件
        srcPath := filepath.Join(task.SourcePath, file.Path)
        if _, err := os.Stat(srcPath); err != nil {
            file.Error = err.Error()
            task.FailedFiles++
            continue
        }
        
        // 2. 复制到目标
        dstPath := filepath.Join(task.TargetPath, file.Path)
        if err := m.copyFile(srcPath, dstPath); err != nil {
            file.Error = err.Error()
            task.FailedFiles++
            continue
        }
        
        // 3. 验证完整性
        if err := m.verifyFile(srcPath, dstPath); err != nil {
            file.Error = err.Error()
            task.FailedFiles++
            continue
        }
        
        // 4. 更新元数据（如果策略要求）
        if task.Action != PolicyActionCopy {
            // 更新符号链接或元数据指向新位置
            m.updateMetadata(srcPath, dstPath)
        }
        
        // 5. 清理源文件（如果策略要求）
        if task.Action == PolicyActionMove {
            os.Remove(srcPath)
        }
        
        task.ProcessedFiles++
        task.ProcessedBytes += file.Size
    }
    
    task.Status = MigrateStatusCompleted
    task.CompletedAt = time.Now()
    
    return nil
}

func (m *Migrator) copyFile(src, dst string) error {
    // 使用rsync风格的增量复制
    // 支持断点续传
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()
    
    dstFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer dstFile.Close()
    
    // 复制数据
    _, err = io.Copy(dstFile, srcFile)
    return err
}

func (m *Migrator) verifyFile(src, dst string) error {
    // 校验文件完整性
    srcHash, _ := m.fileHash(src)
    dstHash, _ := m.fileHash(dst)
    
    if srcHash != dstHash {
        return errors.New("file integrity check failed")
    }
    return nil
}
```

### 3.4 数据模型

#### 3.4.1 存储层配置

```go
type TierConfig struct {
    Type       TierType `json:"type"`       // ssd/hdd/cloud
    Name       string   `json:"name"`       // 显示名称
    Path       string   `json:"path"`       // 存储路径
    Capacity   int64    `json:"capacity"`   // 总容量（字节）
    Used       int64    `json:"used"`       // 已使用（字节）
    Threshold  int      `json:"threshold"`  // 使用阈值（百分比）
    Priority   int      `json:"priority"`   // 优先级（数字越大越高）
    Enabled    bool     `json:"enabled"`    // 是否启用
    ProviderID string   `json:"providerId"` // 云提供商ID（仅云层）
}
```

#### 3.4.2 策略配置

```go
type Policy struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Enabled     bool         `json:"enabled"`
    Status      PolicyStatus `json:"status"`
    
    // 分层规则
    SourceTier TierType     `json:"sourceTier"`
    TargetTier TierType     `json:"targetTier"`
    Action     PolicyAction `json:"action"` // move/copy/archive/delete
    
    // 条件
    MinAccessCount  int64         `json:"minAccessCount"`
    MaxAccessAge    time.Duration `json:"maxAccessAge"`
    MinFileSize     int64         `json:"minFileSize"`
    MaxFileSize     int64         `json:"maxFileSize"`
    FilePatterns    []string      `json:"filePatterns"`
    ExcludePatterns []string      `json:"excludePatterns"`
    
    // 调度
    ScheduleType ScheduleType `json:"scheduleType"`
    ScheduleExpr string       `json:"scheduleExpr"`
    LastRun      time.Time    `json:"lastRun"`
    NextRun      time.Time    `json:"nextRun"`
    
    // 高级选项
    DryRun         bool `json:"dryRun"`         // 试运行模式
    PreserveOrigin bool `json:"preserveOrigin"` // 保留原文件
    VerifyAfter    bool `json:"verifyAfter"`    // 迁移后验证
}
```

### 3.5 API设计

```yaml
# 存储层管理
GET    /api/v1/tiering/tiers              # 列出存储层
POST   /api/v1/tiering/tiers              # 创建存储层
GET    /api/v1/tiering/tiers/{type}       # 获取存储层详情
PUT    /api/v1/tiering/tiers/{type}       # 更新存储层
DELETE /api/v1/tiering/tiers/{type}       # 删除存储层

# 策略管理
GET    /api/v1/tiering/policies           # 列出策略
POST   /api/v1/tiering/policies           # 创建策略
GET    /api/v1/tiering/policies/{id}      # 获取策略详情
PUT    /api/v1/tiering/policies/{id}      # 更新策略
DELETE /api/v1/tiering/policies/{id}      # 删除策略
POST   /api/v1/tiering/policies/{id}/run  # 手动执行策略

# 迁移任务
POST   /api/v1/tiering/migrate            # 手动迁移
GET    /api/v1/tiering/tasks              # 列出迁移任务
GET    /api/v1/tiering/tasks/{id}         # 任务详情
DELETE /api/v1/tiering/tasks/{id}         # 取消任务

# 统计分析
GET    /api/v1/tiering/stats              # 分层统计
GET    /api/v1/tiering/stats/hot          # 热数据统计
GET    /api/v1/tiering/stats/cold         # 冷数据统计
GET    /api/v1/tiering/access-records     # 访问记录
```

### 3.6 关键技术选型

| 组件 | 技术选型 | 理由 |
|------|---------|------|
| 文件追踪 | inotify + 内核审计 | 实时监控文件访问 |
| 数据存储 | SQLite + BadgerDB | 轻量级，嵌入式 |
| 调度器 | cron + 延迟队列 | 灵活的调度策略 |
| 文件复制 | rsync算法 | 增量复制，断点续传 |
| 数据验证 | xxHash | 快速校验算法 |
| 缓存 | groupcache | 热数据缓存 |

---

## 四、实现路线图

### Phase 1：内网穿透基础功能（v2.260.0）

**周期**：2周

**任务清单**：
- [ ] 完善Tunnel Manager核心逻辑
- [ ] 实现P2P Client（STUN打洞）
- [ ] 实现Relay Client（TURN中继）
- [ ] NAT类型检测
- [ ] REST API开发
- [ ] Web UI界面

### Phase 2：内网穿透高级功能（v2.261.0）

**周期**：1周

**任务清单**：
- [ ] Auto模式智能切换
- [ ] TLS/DTLS加密
- [ ] 多隧道并发管理
- [ ] WebSocket实时状态
- [ ] 云端服务部署方案

### Phase 3：分层存储基础功能（v2.262.0）

**周期**：2周

**任务清单**：
- [ ] 完善Tiering Manager
- [ ] Access Tracker实现
- [ ] Policy Engine实现
- [ ] Migration Engine基础框架
- [ ] REST API开发

### Phase 4：分层存储高级功能（v2.263.0）

**周期**：2周

**任务清单**：
- [ ] 智能策略引擎
- [ ] 增量迁移算法
- [ ] 数据完整性验证
- [ ] Web UI界面
- [ ] 性能优化

---

## 五、风险与缓解

### 5.1 内网穿透风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| NAT穿透失败 | 用户无法远程访问 | 自动切换到中继模式 |
| 中继服务器负载 | 性能下降 | 负载均衡 + 按需扩展 |
| 安全漏洞 | 数据泄露 | TLS加密 + 访问控制 |

### 5.2 分层存储风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 迁移过程数据丢失 | 数据损坏 | 验证机制 + 保留原文件选项 |
| 热数据误判 | 性能下降 | 可调阈值 + 机器学习优化 |
| 策略冲突 | 数据混乱 | 策略优先级 + 冲突检测 |

---

## 六、附录

### A. 参考资料

1. RFC 3489 - STUN协议
2. RFC 5766 - TURN协议
3. Synology Tiering白皮书
4. 飞牛fnOS技术文档

### B. 术语表

| 术语 | 说明 |
|------|------|
| NAT | Network Address Translation，网络地址转换 |
| STUN | Session Traversal Utilities for NAT |
| TURN | Traversal Using Relays around NAT |
| P2P | Peer-to-Peer，点对点连接 |
| Tier | 存储层，如SSD/HDD/Cloud |

---

*文档版本：v1.0*
*创建日期：2026-03-24*
*维护部门：兵部*