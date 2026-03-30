# NAS-OS 模块划分方案

## 版本：v2.260.0
## 日期：2026-03-24
## 部门：兵部

---

## 一、内网穿透模块（internal/tunnel）

### 1.1 目录结构

```
internal/tunnel/
├── doc.go              # 包文档
├── types.go            # 类型定义（已完成）
├── manager.go          # 隧道管理器（需增强）
├── client.go           # 客户端接口
│
├── client/             # 客户端实现
│   ├── p2p.go          # P2P客户端
│   ├── relay.go        # 中继客户端
│   ├── reverse.go      # 反向代理客户端
│   └── auto.go         # 自动选择客户端
│
├── nat/                # NAT检测
│   ├── detector.go     # NAT检测器接口
│   ├── stun.go         # STUN协议实现
│   └── types.go        # NAT类型定义
│
├── protocol/           # 协议层
│   ├── message.go      # 消息协议
│   ├── codec.go        # 编解码器
│   └── handshake.go    # 握手协议
│
├── security/           # 安全层
│   ├── tls.go          # TLS加密
│   ├── auth.go         # 认证机制
│   └── acl.go          # 访问控制
│
├── handler.go          # HTTP处理器
├── handler_test.go     # 单元测试
└── integration_test.go # 集成测试
```

### 1.2 核心模块职责

| 模块 | 文件 | 职责 |
|------|------|------|
| Manager | manager.go | 隧道生命周期管理、状态维护 |
| P2P Client | client/p2p.go | STUN打洞、UDP直连 |
| Relay Client | client/relay.go | TURN中继、TCP转发 |
| Reverse Client | client/reverse.go | 反向代理、WebSocket |
| Auto Client | client/auto.go | 智能选择最优连接方式 |
| NAT Detector | nat/detector.go | NAT类型检测 |
| Protocol | protocol/ | 自定义协议实现 |
| Security | security/ | 加密、认证、访问控制 |
| Handler | handler.go | REST API处理器 |

### 1.3 接口定义

```go
// client.go - 客户端接口

package tunnel

// TunnelClient 隧道客户端接口
type TunnelClient interface {
    // 连接管理
    Connect(ctx context.Context) error
    Disconnect() error
    IsConnected() bool
    
    // 数据传输
    Send(data []byte) (int, error)
    Receive() ([]byte, error)
    
    // 状态
    GetStatus() TunnelStatus
}

// TunnelClientFactory 客户端工厂
type TunnelClientFactory interface {
    Create(config TunnelConfig, globalConfig Config) (TunnelClient, error)
}
```

```go
// nat/detector.go - NAT检测器接口

package nat

// Detector NAT检测器接口
type Detector interface {
    // 检测NAT类型
    Detect(ctx context.Context) (NATType, string, int, error)
    
    // 获取公网地址
    GetPublicAddr() (string, int, error)
    
    // 保持活跃
    KeepAlive(ctx context.Context) error
}
```

```go
// security/auth.go - 认证接口

package security

// Authenticator 认证器接口
type Authenticator interface {
    // 生成令牌
    GenerateToken(deviceID string) (string, error)
    
    // 验证令牌
    VerifyToken(token string) (string, error)
    
    // 刷新令牌
    RefreshToken(token string) (string, error)
}

// AccessControl 访问控制接口
type AccessControl interface {
    // 检查权限
    CheckPermission(token string, resource string, action string) (bool, error)
    
    // 添加规则
    AddRule(rule ACLRule) error
    
    // 删除规则
    RemoveRule(ruleID string) error
}
```

### 1.4 依赖关系

```
┌─────────────────────────────────────────────────────────────┐
│                       API Layer                              │
│                      handler.go                              │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          v
┌─────────────────────────────────────────────────────────────┐
│                    Manager Layer                             │
│                     manager.go                               │
└─────────────────────────┬───────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          v               v               v
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│ P2P Client  │   │Relay Client │   │Reverse Clnt │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └─────────────────┼─────────────────┘
                         v
┌─────────────────────────────────────────────────────────────┐
│                    Protocol Layer                            │
│              protocol/message.go, codec.go                   │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          v
┌─────────────────────────────────────────────────────────────┐
│                    Security Layer                            │
│               security/tls.go, auth.go                       │
└─────────────────────────────────────────────────────────────┘
```

---

## 二、分层存储模块（internal/tiering）

### 2.1 目录结构

```
internal/tiering/
├── doc.go              # 包文档
├── types.go            # 类型定义（已完成）
├── manager.go          # 分层管理器（需增强）
├── handler.go          # HTTP处理器
│
├── tracker/            # 访问追踪
│   ├── tracker.go      # 追踪器主逻辑
│   ├── inotify.go      # inotify监控
│   ├── audit.go        # 审计日志集成
│   └── store.go        # 数据持久化
│
├── policy/             # 策略引擎
│   ├── engine.go       # 策略引擎
│   ├── evaluator.go    # 规则评估器
│   ├── scheduler.go    # 策略调度器
│   └── templates.go    # 预设策略模板
│
├── migration/          # 迁移引擎
│   ├── engine.go       # 迁移引擎
│   ├── worker.go       # 工作线程池
│   ├── copier.go       # 文件复制器
│   ├── verifier.go     # 完整性验证
│   └── rollback.go     # 回滚机制
│
├── tier/               # 存储层管理
│   ├── ssd.go          # SSD层
│   ├── hdd.go          # HDD层
│   ├── cloud.go        # 云存储层
│   └── memory.go       # 内存缓存层（可选）
│
├── stats/              # 统计分析
│   ├── collector.go    # 数据收集器
│   ├── analyzer.go     # 分析器
│   ├── reporter.go     # 报告生成器
│   └── cache.go        # 统计缓存
│
├── handler.go          # HTTP处理器
├── handler_test.go     # 单元测试
└── integration.go      # 集成测试
```

### 2.2 核心模块职责

| 模块 | 文件 | 职责 |
|------|------|------|
| Manager | manager.go | 存储层/策略/任务协调管理 |
| Access Tracker | tracker/tracker.go | 文件访问监控、热度计算 |
| Policy Engine | policy/engine.go | 策略评估、任务生成 |
| Migration Engine | migration/engine.go | 文件迁移执行、进度跟踪 |
| Tier Manager | tier/ | 存储层配置、容量管理 |
| Stats | stats/ | 统计分析、报告生成 |
| Handler | handler.go | REST API处理器 |

### 2.3 接口定义

```go
// tracker/tracker.go - 访问追踪器接口

package tracker

// AccessTracker 访问追踪器接口
type AccessTracker interface {
    // 启动/停止
    Start() error
    Stop() error
    
    // 追踪访问
    TrackAccess(path string, op Operation) error
    
    // 查询
    GetRecord(path string) (*FileAccessRecord, error)
    GetHotFiles(limit int) ([]*FileAccessRecord, error)
    GetColdFiles(limit int) ([]*FileAccessRecord, error)
    
    // 统计
    GetStats() (*AccessStats, error)
}

// Operation 文件操作类型
type Operation string

const (
    OpRead   Operation = "read"
    OpWrite  Operation = "write"
    OpDelete Operation = "delete"
)
```

```go
// policy/engine.go - 策略引擎接口

package policy

// Engine 策略引擎接口
type Engine interface {
    // 启动/停止
    Start() error
    Stop() error
    
    // 策略管理
    AddPolicy(policy *Policy) error
    RemovePolicy(policyID string) error
    GetPolicy(policyID string) (*Policy, error)
    ListPolicies() ([]*Policy, error)
    
    // 执行
    RunPolicy(ctx context.Context, policyID string) (*MigrateTask, error)
    RunAll(ctx context.Context) ([]*MigrateTask, error)
}

// Evaluator 规则评估器接口
type Evaluator interface {
    // 评估文件是否匹配规则
    Evaluate(record *FileAccessRecord, rule *PolicyRule) (bool, error)
    
    // 批量评估
    EvaluateBatch(records []*FileAccessRecord, rule *PolicyRule) ([]*FileAccessRecord, error)
}
```

```go
// migration/engine.go - 迁移引擎接口

package migration

// Engine 迁移引擎接口
type Engine interface {
    // 启动/停止
    Start() error
    Stop() error
    
    // 提交任务
    Submit(task *MigrateTask) error
    
    // 任务管理
    GetTask(taskID string) (*MigrateTask, error)
    ListTasks() ([]*MigrateTask, error)
    CancelTask(taskID string) error
    
    // 回调
    OnComplete(callback func(task *MigrateTask))
}

// Copier 文件复制器接口
type Copier interface {
    // 复制文件
    Copy(src, dst string, opts CopyOptions) error
    
    // 增量复制
    CopyIncremental(src, dst string, opts CopyOptions) error
    
    // 进度回调
    OnProgress(callback func(copied, total int64))
}

// Verifier 完整性验证器接口
type Verifier interface {
    // 验证文件
    Verify(src, dst string) error
    
    // 计算哈希
    Hash(path string) (string, error)
}
```

```go
// tier/manager.go - 存储层管理器接口

package tier

// Manager 存储层管理器接口
type Manager interface {
    // 存储层管理
    Create(config *TierConfig) error
    Get(tierType TierType) (*TierConfig, error)
    Update(tierType TierType, config *TierConfig) error
    Delete(tierType TierType) error
    List() ([]*TierConfig, error)
    
    // 容量管理
    GetCapacity(tierType TierType) (*CapacityInfo, error)
    RefreshCapacity(tierType TierType) error
    
    // 健康检查
    HealthCheck(tierType TierType) (*HealthStatus, error)
}

// CapacityInfo 容量信息
type CapacityInfo struct {
    Total     int64   `json:"total"`
    Used      int64   `json:"used"`
    Available int64   `json:"available"`
    UsagePercent float64 `json:"usagePercent"`
}
```

### 2.4 依赖关系

```
┌─────────────────────────────────────────────────────────────┐
│                       API Layer                              │
│                      handler.go                              │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          v
┌─────────────────────────────────────────────────────────────┐
│                    Manager Layer                             │
│                     manager.go                               │
└─────────────────────────┬───────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          v               v               v
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   Tracker   │   │   Policy    │   │  Migration  │
│   访问追踪   │   │   策略引擎  │   │   迁移引擎   │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └─────────────────┼─────────────────┘
                         v
┌─────────────────────────────────────────────────────────────┐
│                     Tier Layer                               │
│           tier/ssd.go, tier/hdd.go, tier/cloud.go           │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          v
┌─────────────────────────────────────────────────────────────┐
│                    Storage Backend                           │
│                 文件系统 / 云存储 API                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 三、模块间交互

### 3.1 内网穿透与分层存储联动

```
┌─────────────────────────────────────────────────────────────┐
│                      NAS-OS Core                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌───────────────┐                    ┌───────────────┐     │
│  │ Tunnel Module │                    │ Tiering Module│     │
│  │               │                    │               │     │
│  │ ┌───────────┐ │                    │ ┌───────────┐ │     │
│  │ │ Handler   │ │                    │ │ Handler   │ │     │
│  │ └─────┬─────┘ │                    │ └─────┬─────┘ │     │
│  │       │       │                    │       │       │     │
│  │ ┌─────┴─────┐ │                    │ ┌─────┴─────┐ │     │
│  │ │  Manager  │ │                    │ │  Manager  │ │     │
│  │ └─────┬─────┘ │                    │ └─────┬─────┘ │     │
│  │       │       │                    │       │       │     │
│  │       │       │                    │       │       │     │
│  └───────┼───────┘                    └───────┼───────┘     │
│          │                                    │             │
│          │         共享组件                    │             │
│          │    ┌───────────────┐              │             │
│          └───>│ Event Bus     │<─────────────┘             │
│          │    │ 事件总线       │              │             │
│          │    └───────────────┘              │             │
│          │                                    │             │
│          │    ┌───────────────┐              │             │
│          └───>│ Metrics       │<─────────────┘             │
│          │    │ 监控指标       │              │             │
│          │    └───────────────┘              │             │
│          │                                    │             │
│          │    ┌───────────────┐              │             │
│          └───>│ Logger        │<─────────────┘             │
│               │ 日志系统       │                            │
│               └───────────────┘                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 事件流

```go
// 事件定义

// Tunnel事件
type TunnelEvent struct {
    Type      string    // created, connected, disconnected, error
    TunnelID  string
    Timestamp time.Time
    Data      interface{}
}

// Tiering事件
type TieringEvent struct {
    Type      string    // policy_run, migration_start, migration_complete
    PolicyID  string
    TaskID    string
    Timestamp time.Time
    Data      interface{}
}

// 事件总线接口
type EventBus interface {
    Publish(topic string, event interface{}) error
    Subscribe(topic string, handler EventHandler) error
    Unsubscribe(topic string, handler EventHandler) error
}
```

---

## 四、配置管理

### 4.1 配置文件结构

```yaml
# /etc/nas-os/tunnel.yaml

tunnel:
  # 服务端配置
  server:
    addr: "tunnel.nas-os.io"
    port: 443
    
  # STUN/TURN
  stun_servers:
    - "stun:stun.l.google.com:19302"
    - "stun:stun1.l.google.com:19302"
  turn_servers:
    - "turn:turn.nas-os.io:3478"
  turn_credentials:
    username: ""
    password: ""
    
  # 连接配置
  connection:
    mode: "auto"  # p2p, relay, reverse, auto
    heartbeat_interval: 30
    reconnect_interval: 5
    max_reconnect: 10
    timeout: 30
    
  # 安全配置
  security:
    tls_enabled: true
    cert_file: "/etc/nas-os/certs/tunnel.crt"
    key_file: "/etc/nas-os/certs/tunnel.key"
    
  # 默认隧道
  tunnels:
    - name: "web-ui"
      local_port: 5000
      protocol: "tcp"
      enabled: true
```

```yaml
# /etc/nas-os/tiering.yaml

tiering:
  # 存储层配置
  tiers:
    ssd:
      name: "SSD缓存层"
      path: "/mnt/ssd"
      priority: 100
      threshold: 80
      enabled: true
      
    hdd:
      name: "HDD存储层"
      path: "/mnt/hdd"
      priority: 50
      threshold: 90
      enabled: true
      
    cloud:
      name: "云存储归档层"
      path: "/mnt/cloud"
      priority: 10
      enabled: false
      provider: "s3"
      bucket: "nas-archive"
      
  # 策略引擎配置
  policy_engine:
    check_interval: "1h"
    hot_threshold: 100      # 访问次数
    warm_threshold: 10      # 访问次数
    cold_age_hours: 720     # 30天
    max_concurrent: 5       # 最大并发迁移数
    enable_auto_tier: true
    
  # 访问追踪配置
  tracker:
    track_interval: "5m"
    retention_days: 90
    max_records: 1000000
    enable_hot_cold: true
    storage_backend: "sqlite"
    storage_path: "/var/lib/nas-os/tiering/tracker.db"
    
  # 默认策略
  policies:
    - id: "hot-to-ssd"
      name: "热数据提升到SSD"
      source_tier: "hdd"
      target_tier: "ssd"
      action: "move"
      min_access_count: 100
      max_access_age: "168h"  # 7天
      enabled: true
      
    - id: "cold-to-hdd"
      name: "冷数据下沉到HDD"
      source_tier: "ssd"
      target_tier: "hdd"
      action: "move"
      max_access_age: "720h"  # 30天
      enabled: true
```

### 4.2 配置加载

```go
// internal/tunnel/config.go

package tunnel

type Config struct {
    Server    ServerConfig    `yaml:"server"`
    STUN      STUNConfig      `yaml:"stun_servers"`
    TURN      TURNConfig      `yaml:"turn_servers"`
    Connection ConnConfig     `yaml:"connection"`
    Security  SecurityConfig  `yaml:"security"`
    Tunnels   []TunnelConfig  `yaml:"tunnels"`
}

func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    // 设置默认值
    setDefaults(&config)
    
    // 验证配置
    if err := validate(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

---

## 五、测试策略

### 5.1 单元测试

```go
// internal/tunnel/manager_test.go

func TestManagerConnect(t *testing.T) {
    tests := []struct {
        name    string
        req     ConnectRequest
        wantErr bool
    }{
        {
            name: "valid request",
            req: ConnectRequest{
                Name:      "test-tunnel",
                Mode:      ModeP2P,
                LocalPort: 5000,
            },
            wantErr: false,
        },
        {
            name: "invalid port",
            req: ConnectRequest{
                Name:      "test-tunnel",
                LocalPort: 70000,
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewTestManager(t)
            _, err := m.Connect(context.Background(), tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 5.2 集成测试

```go
// internal/tunnel/integration_test.go

func TestP2PConnection(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // 启动模拟STUN服务器
    stunServer := NewMockSTUNServer(t)
    defer stunServer.Close()
    
    // 创建客户端
    config := Config{
        STUNServers: []string{stunServer.Addr()},
    }
    
    m, err := NewManager(config, zap.NewNop())
    require.NoError(t, err)
    
    // 测试NAT检测
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    natType, ip, port, err := m.detector.Detect(ctx)
    require.NoError(t, err)
    assert.NotEmpty(t, ip)
    assert.Greater(t, port, 0)
}
```

### 5.3 性能测试

```go
// internal/tiering/migration/engine_test.go

func BenchmarkMigrateFile(b *testing.B) {
    // 创建测试文件
    tmpDir := b.TempDir()
    srcPath := filepath.Join(tmpDir, "source", "test.dat")
    dstPath := filepath.Join(tmpDir, "dest", "test.dat")
    
    // 生成100MB测试文件
    createTestFile(srcPath, 100*1024*1024)
    
    engine := NewEngine(DefaultEngineConfig())
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        task := &MigrateTask{
            Files: []MigrateFile{
                {Path: "test.dat", Size: 100 * 1024 * 1024},
            },
            SourcePath: filepath.Join(tmpDir, "source"),
            TargetPath: filepath.Join(tmpDir, "dest"),
        }
        engine.executeTask(task)
    }
}
```

---

*文档版本：v1.0*
*创建日期：2026-03-24*
*维护部门：兵部*