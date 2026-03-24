# NAS-OS 关键技术选型

## 版本：v2.260.0
## 日期：2026-03-24
## 部门：兵部

---

## 一、内网穿透技术选型

### 1.1 NAT穿透技术

#### 1.1.1 STUN (Session Traversal Utilities for NAT)

**技术方案**：`pion/stun` 库

**选型理由**：
- Go原生实现，无CGO依赖
- WebRTC生态成熟组件
- 支持RFC 5389标准
- 活跃维护，文档完善

**使用场景**：
- NAT类型检测
- 公网地址发现
- P2P打洞协商

**代码示例**：
```go
import "github.com/pion/stun"

// NAT类型检测
func DetectNATType(stunAddr string) (NATType, error) {
    conn, err := net.Dial("udp", stunAddr)
    if err != nil {
        return NATTypeUnknown, err
    }
    defer conn.Close()
    
    // 创建STUN客户端
    client, err := stun.NewClient(conn)
    if err != nil {
        return NATTypeUnknown, err
    }
    
    // 发送Binding请求
    var mappedAddr stun.XORMappedAddress
    err = client.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
        if res.Error != nil {
            return
        }
        stun.XORMappedAddress.GetFrom(res.Message, &mappedAddr)
    })
    
    // 分析响应判断NAT类型
    return analyzeNATType(mappedAddr)
}
```

#### 1.1.2 TURN (Traversal Using Relays around NAT)

**技术方案**：`pion/turn` 库

**选型理由**：
- 与pion/stun配套使用
- 支持RFC 5766标准
- 支持UDP/TCP传输
- 内置权限和通道管理

**使用场景**：
- P2P打洞失败时的中继
- 对称型NAT穿透
- 稳定的数据转发

**代码示例**：
```go
import "github.com/pion/turn/v2"

// 创建TURN客户端
func CreateTURNClient(cfg TURNConfig) (*turn.Client, error) {
    conn, err := net.Dial("udp", cfg.ServerAddr)
    if err != nil {
        return nil, err
    }
    
    client, err := turn.NewClient(&turn.ClientConfig{
        STUNServerAddr: cfg.STUNAddr,
        TURNServerAddr: cfg.ServerAddr,
        Conn:           conn,
        Username:       cfg.Username,
        Password:       cfg.Password,
    })
    if err != nil {
        return nil, err
    }
    
    return client, nil
}

// 分配中继地址
func AllocateRelay(client *turn.Client) (net.Addr, error) {
    return client.Allocate()
}
```

### 1.2 隧道协议选型

#### 1.2.1 方案对比

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **frp** | 成熟稳定、配置简单、社区活跃 | 性能一般、协议开销大 | 通用内网穿透 |
| **nps** | 轻量级、功能丰富、Web管理 | 文档较少、维护频率低 | 小规模部署 |
| **WireGuard** | 高性能、安全、内核级 | 配置复杂、需要权限 | 站点到站点VPN |
| **自研协议** | 可定制、轻量、低延迟 | 开发成本高、需自建生态 | 特定场景优化 |

**最终选型**：**frp + 自研轻量协议混合方案**

- 默认使用frp协议，兼容性好
- 高性能场景使用自研协议
- 支持协议协商和切换

#### 1.2.2 frp集成方案

**集成方式**：内嵌frp客户端

```go
import "github.com/fatedier/frp/client"

// frp客户端封装
type FRPClient struct {
    config *client.Config
    svr    *client.Service
}

func NewFRPClient(cfg TunnelConfig) *FRPClient {
    return &FRPClient{
        config: &client.Config{
            ClientConfig: client.ClientConfig{
                ServerAddr: cfg.ServerAddr,
                ServerPort: cfg.ServerPort,
                Token:      cfg.AuthToken,
            },
            Proxies: []client.ProxyConfig{
                {
                    BaseProxyConfig: client.BaseProxyConfig{
                        Name:   cfg.Name,
                        Type:   "tcp",
                        LocalIP:   "127.0.0.1",
                        LocalPort: cfg.LocalPort,
                    },
                },
            },
        },
    }
}

func (c *FRPClient) Start() error {
    svr, err := client.NewService(c.config)
    if err != nil {
        return err
    }
    c.svr = svr
    return c.svr.Run(context.Background())
}
```

#### 1.2.3 自研轻量协议

**协议设计**：

```
┌─────────────────────────────────────────────────────────────┐
│                    NAS-OS Tunnel Protocol                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    Frame Format                       │   │
│  │  ┌─────────┬─────────┬─────────┬─────────────────┐   │   │
│  │  │ Version │  Type   │ Length  │     Payload     │   │   │
│  │  │  1 byte │ 1 byte  │ 2 bytes │   Variable      │   │   │
│  │  └─────────┴─────────┴─────────┴─────────────────┘   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  Frame Types:                                               │
│  0x01 - Handshake Request                                   │
│  0x02 - Handshake Response                                  │
│  0x03 - Data                                                │
│  0x04 - Ack                                                 │
│  0x05 - Ping                                                │
│  0x06 - Pong                                                │
│  0x07 - Close                                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Go实现**：
```go
// protocol/frame.go

const (
    FrameVersion = 0x01
    
    FrameHandshakeRequest = 0x01
    FrameHandshakeResponse = 0x02
    FrameData = 0x03
    FrameAck = 0x04
    FramePing = 0x05
    FramePong = 0x06
    FrameClose = 0x07
)

type Frame struct {
    Version byte
    Type    byte
    Length  uint16
    Payload []byte
}

func (f *Frame) Encode() []byte {
    buf := make([]byte, 4+len(f.Payload))
    buf[0] = f.Version
    buf[1] = f.Type
    binary.BigEndian.PutUint16(buf[2:4], f.Length)
    copy(buf[4:], f.Payload)
    return buf
}

func DecodeFrame(data []byte) (*Frame, error) {
    if len(data) < 4 {
        return nil, errors.New("frame too short")
    }
    
    f := &Frame{
        Version: data[0],
        Type:    data[1],
        Length:  binary.BigEndian.Uint16(data[2:4]),
    }
    
    if len(data) < int(4+f.Length) {
        return nil, errors.New("incomplete frame")
    }
    
    f.Payload = data[4 : 4+f.Length]
    return f, nil
}
```

### 1.3 安全技术选型

#### 1.3.1 TLS/DTLS加密

**技术方案**：Go标准库 `crypto/tls`

**配置示例**：
```go
func CreateTLSConfig(certFile, keyFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    
    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_CHACHA20_POLY1305_SHA256,
            tls.TLS_AES_128_GCM_SHA256,
        },
    }, nil
}
```

**DTLS支持**：`pion/dtls` 库

```go
import "github.com/pion/dtls/v2"

func CreateDTLSConfig(certFile, keyFile string) (*dtls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    
    return &dtls.Config{
        Certificates: []tls.Certificate{cert},
        CipherSuites: []dtls.CipherSuiteID{
            dtls.TLS_AES_256_GCM_SHA384,
            dtls.TLS_CHACHA20_POLY1305_SHA256,
        },
    }, nil
}
```

#### 1.3.2 Token认证

**技术方案**：JWT (JSON Web Token)

**选型库**：`golang-jwt/jwt/v5`

**实现示例**：
```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    DeviceID string `json:"device_id"`
    jwt.RegisteredClaims
}

func GenerateToken(deviceID, secret string) (string, error) {
    claims := &Claims{
        DeviceID: deviceID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "nas-os-tunnel",
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func VerifyToken(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, errors.New("invalid token")
}
```

---

## 二、分层存储技术选型

### 2.1 文件监控技术

#### 2.1.1 方案对比

| 方案 | 实时性 | 性能开销 | 跨平台 | 适用场景 |
|------|--------|----------|--------|---------|
| **inotify** | 高 | 低 | Linux only | Linux系统首选 |
| **fanotify** | 高 | 低 | Linux only | 需要文件内容时 |
| **轮询扫描** | 低 | 高 | 全平台 | 兼容性方案 |
| **eBPF** | 最高 | 最低 | Linux only | 内核级监控 |

**最终选型**：**inotify + 定时扫描混合方案**

- Linux环境使用inotify实时监控
- 定时扫描作为补充和校验
- 其他平台使用轮询方案

#### 2.1.2 inotify实现

**技术方案**：`fsnotify/fsnotify` 库

```go
import "github.com/fsnotify/fsnotify"

type FileWatcher struct {
    watcher *fsnotify.Watcher
    tracker *AccessTracker
}

func NewFileWatcher(paths []string) (*FileWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    
    fw := &FileWatcher{
        watcher: watcher,
    }
    
    for _, path := range paths {
        if err := watcher.Add(path); err != nil {
            return nil, err
        }
    }
    
    return fw, nil
}

func (fw *FileWatcher) Start(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-fw.watcher.Events:
            if !ok {
                return
            }
            fw.handleEvent(event)
        case err, ok := <-fw.watcher.Errors:
            if !ok {
                return
            }
            log.Printf("watcher error: %v", err)
        }
    }
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
    switch {
    case event.Op&fsnotify.Create == fsnotify.Create:
        fw.tracker.TrackAccess(event.Name, OpCreate)
    case event.Op&fsnotify.Write == fsnotify.Write:
        fw.tracker.TrackAccess(event.Name, OpWrite)
    case event.Op&fsnotify.Remove == fsnotify.Remove:
        fw.tracker.TrackAccess(event.Name, OpDelete)
    }
}
```

### 2.2 数据存储技术

#### 2.2.1 访问记录存储

**方案对比**：

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **SQLite** | 零配置、嵌入式、支持SQL | 单写入限制 | 本地存储首选 |
| **BadgerDB** | 高性能、纯Go、LSM树 | 无SQL、内存占用 | 高写入场景 |
| **LevelDB** | 成熟稳定、轻量 | CGO依赖 | 大数据量 |
| **Redis** | 高性能、丰富数据结构 | 需要独立服务 | 分布式场景 |

**最终选型**：**SQLite + BadgerDB 混合方案**

- SQLite存储访问记录（关系查询）
- BadgerDB存储热点缓存（快速访问）

#### 2.2.2 SQLite实现

```go
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type RecordStore struct {
    db *sql.DB
}

func NewRecordStore(path string) (*RecordStore, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }
    
    // 创建表
    schema := `
    CREATE TABLE IF NOT EXISTS access_records (
        path TEXT PRIMARY KEY,
        size INTEGER,
        mod_time DATETIME,
        access_time DATETIME,
        access_count INTEGER,
        read_bytes INTEGER,
        write_bytes INTEGER,
        current_tier TEXT,
        frequency TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE INDEX IF NOT EXISTS idx_access_time ON access_records(access_time);
    CREATE INDEX IF NOT EXISTS idx_access_count ON access_records(access_count);
    CREATE INDEX IF NOT EXISTS idx_frequency ON access_records(frequency);
    `
    
    if _, err := db.Exec(schema); err != nil {
        return nil, err
    }
    
    return &RecordStore{db: db}, nil
}

func (s *RecordStore) Upsert(record *FileAccessRecord) error {
    query := `
    INSERT INTO access_records 
        (path, size, mod_time, access_time, access_count, read_bytes, write_bytes, current_tier, frequency)
    VALUES 
        (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(path) DO UPDATE SET
        size = excluded.size,
        mod_time = excluded.mod_time,
        access_time = excluded.access_time,
        access_count = excluded.access_count,
        read_bytes = excluded.read_bytes,
        write_bytes = excluded.write_bytes,
        current_tier = excluded.current_tier,
        frequency = excluded.frequency,
        updated_at = CURRENT_TIMESTAMP
    `
    
    _, err := s.db.Exec(query,
        record.Path,
        record.Size,
        record.ModTime,
        record.AccessTime,
        record.AccessCount,
        record.ReadBytes,
        record.WriteBytes,
        record.CurrentTier,
        record.Frequency,
    )
    
    return err
}

func (s *RecordStore) GetHotFiles(limit int) ([]*FileAccessRecord, error) {
    query := `
    SELECT path, size, mod_time, access_time, access_count, read_bytes, write_bytes, current_tier, frequency
    FROM access_records
    WHERE frequency = 'hot'
    ORDER BY access_count DESC
    LIMIT ?
    `
    
    rows, err := s.db.Query(query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var records []*FileAccessRecord
    for rows.Next() {
        r := &FileAccessRecord{}
        err := rows.Scan(
            &r.Path, &r.Size, &r.ModTime, &r.AccessTime,
            &r.AccessCount, &r.ReadBytes, &r.WriteBytes,
            &r.CurrentTier, &r.Frequency,
        )
        if err != nil {
            return nil, err
        }
        records = append(records, r)
    }
    
    return records, nil
}
```

### 2.3 文件复制技术

#### 2.3.1 rsync算法

**选型理由**：
- 增量复制，减少数据传输
- 断点续传支持
- 广泛验证的算法

**Go实现**：`github.com/kahing/go-rsync`

```go
import "github.com/kahing/go-rsync/rsync"

type RsyncCopier struct {
    chunkSize int
}

func NewRsyncCopier() *RsyncCopier {
    return &RsyncCopier{
        chunkSize: 4096,
    }
}

func (c *RsyncCopier) Copy(src, dst string) error {
    // 创建目标目录
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return err
    }
    
    // 计算源文件签名
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
    
    // 使用rsync算法进行增量复制
    rs := rsync.New(c.chunkSize)
    
    // 如果目标文件存在，计算delta
    if _, err := os.Stat(dst); err == nil {
        return c.deltaCopy(rs, srcFile, dstFile)
    }
    
    // 全量复制
    _, err = io.Copy(dstFile, srcFile)
    return err
}
```

#### 2.3.2 数据完整性验证

**技术方案**：xxHash算法

**选型库**：`github.com/cespare/xxhash/v2`

**选型理由**：
- 极快的哈希速度（~40GB/s）
- 良好的碰撞率
- 稳定的API

```go
import "github.com/cespare/xxhash/v2"

type Verifier struct{}

func (v *Verifier) HashFile(path string) (uint64, error) {
    f, err := os.Open(path)
    if err != nil {
        return 0, err
    }
    defer f.Close()
    
    h := xxhash.New()
    if _, err := io.Copy(h, f); err != nil {
        return 0, err
    }
    
    return h.Sum64(), nil
}

func (v *Verifier) Verify(src, dst string) error {
    srcHash, err := v.HashFile(src)
    if err != nil {
        return fmt.Errorf("hash source: %w", err)
    }
    
    dstHash, err := v.HashFile(dst)
    if err != nil {
        return fmt.Errorf("hash destination: %w", err)
    }
    
    if srcHash != dstHash {
        return errors.New("integrity check failed: hash mismatch")
    }
    
    return nil
}
```

### 2.4 调度技术

#### 2.4.1 Cron调度

**技术方案**：`robfig/cron` 库

```go
import "github.com/robfig/cron/v3"

type PolicyScheduler struct {
    cron   *cron.Cron
    engine *PolicyEngine
}

func NewPolicyScheduler(engine *PolicyEngine) *PolicyScheduler {
    return &PolicyScheduler{
        cron:   cron.New(cron.WithSeconds()),
        engine: engine,
    }
}

func (s *PolicyScheduler) AddPolicy(policy *Policy) error {
    if policy.ScheduleType == ScheduleTypeCron {
        _, err := s.cron.AddFunc(policy.ScheduleExpr, func() {
            s.engine.RunPolicy(context.Background(), policy.ID)
        })
        return err
    }
    
    if policy.ScheduleType == ScheduleTypeInterval {
        // 解析间隔表达式
        duration, err := time.ParseDuration(policy.ScheduleExpr)
        if err != nil {
            return err
        }
        
        spec := fmt.Sprintf("@every %s", duration)
        _, err = s.cron.AddFunc(spec, func() {
            s.engine.RunPolicy(context.Background(), policy.ID)
        })
        return err
    }
    
    return nil
}

func (s *PolicyScheduler) Start() {
    s.cron.Start()
}

func (s *PolicyScheduler) Stop() {
    s.cron.Stop()
}
```

---

## 三、共享技术选型

### 3.1 日志系统

**技术方案**：`uber-go/zap`

```go
import "go.uber.org/zap"

func NewLogger(level string) (*zap.Logger, error) {
    config := zap.NewProductionConfig()
    
    switch level {
    case "debug":
        config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
    case "info":
        config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    case "warn":
        config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
    case "error":
        config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
    }
    
    return config.Build()
}
```

### 3.2 配置管理

**技术方案**：`spf13/viper`

```go
import "github.com/spf13/viper"

func LoadConfig(path string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)
    v.SetConfigType("yaml")
    
    // 设置默认值
    v.SetDefault("tunnel.connection.heartbeat_interval", 30)
    v.SetDefault("tunnel.connection.reconnect_interval", 5)
    v.SetDefault("tiering.policy_engine.check_interval", "1h")
    
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    if err := v.Unmarshal(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

### 3.3 监控指标

**技术方案**：`prometheus/client_golang`

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    tunnelConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nasos_tunnel_connections",
            Help: "Number of active tunnel connections",
        },
        []string{"mode", "state"},
    )
    
    tunnelBytesTx = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nasos_tunnel_bytes_transmitted_total",
            Help: "Total bytes transmitted through tunnels",
        },
        []string{"tunnel_id"},
    )
    
    tieringMigrations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nasos_tiering_migrations_total",
            Help: "Total number of file migrations",
        },
        []string{"source_tier", "target_tier", "status"},
    )
    
    tieringBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nasos_tiering_tier_bytes",
            Help: "Bytes stored in each tier",
        },
        []string{"tier", "frequency"},
    )
)
```

---

## 四、依赖清单

### 4.1 内网穿透模块

```go
// go.mod dependencies

require (
    // NAT穿透
    github.com/pion/stun v2.1.0+incompatible
    github.com/pion/turn/v2 v2.1.6
    github.com/pion/dtls/v2 v2.2.7
    
    // 隧道协议
    github.com/fatedier/frp v0.61.1
    
    // 安全
    github.com/golang-jwt/jwt/v5 v5.2.0
    
    // 网络
    golang.org/x/net v0.24.0
)
```

### 4.2 分层存储模块

```go
// go.mod dependencies

require (
    // 文件监控
    github.com/fsnotify/fsnotify v1.7.0
    
    // 数据存储
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/dgraph-io/badger/v4 v4.2.0
    
    // 文件操作
    github.com/cespare/xxhash/v2 v2.3.0
    
    // 调度
    github.com/robfig/cron/v3 v3.0.1
)
```

### 4.3 共享依赖

```go
// go.mod dependencies

require (
    // 日志
    go.uber.org/zap v1.27.0
    
    // 配置
    github.com/spf13/viper v1.18.2
    
    // 监控
    github.com/prometheus/client_golang v1.19.0
    
    // 工具库
    github.com/google/uuid v1.6.0
    golang.org/x/sync v0.7.0
)
```

---

## 五、版本兼容性

### 5.1 Go版本要求

- **最低版本**：Go 1.21
- **推荐版本**：Go 1.22+

### 5.2 系统兼容性

| 功能 | Linux | macOS | Windows |
|------|-------|-------|---------|
| 内网穿透 | ✅ | ✅ | ✅ |
| inotify监控 | ✅ | ❌ (使用FSEvents) | ❌ (使用ReadDirectoryChangesW) |
| 分层存储 | ✅ | ✅ | ✅ |

---

*文档版本：v1.0*
*创建日期：2026-03-24*
*维护部门：兵部*