# Presto 高速文件传输模块

Presto 是 NAS-OS 的高速文件传输模块，对标群晖 Presto File Server。基于 QUIC 协议实现高速、加密、可断点续传的文件传输。

## 功能特性

### 核心功能
- **QUIC 协议传输**: 基于 QUIC 协议，支持多路复用、0-RTT 连接、拥塞控制
- **断点续传**: 支持传输中断后从断点继续，无需重新开始
- **AES-256-GCM 加密**: 端到端加密传输，保障数据安全
- **数据压缩**: 支持 zstd 压缩，减少传输数据量
- **分块传输**: 大文件自动分块，支持并行传输
- **速度限制**: 可配置传输速度限制
- **并发控制**: 可配置最大并发传输数

### 管理功能
- **传输任务管理**: 创建、暂停、恢复、取消传输任务
- **实时监控**: 实时查看传输进度、速度、剩余时间
- **统计汇总**: 查看历史传输统计数据
- **自动清理**: 自动清理过期的已完成任务

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Presto 架构                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    QUIC/TLS 1.3    ┌─────────────┐        │
│  │   客户端     │◄──────────────────►│   服务端     │        │
│  │  (Client)    │                    │  (Server)    │        │
│  └──────┬──────┘                    └──────┬──────┘        │
│         │                                  │                │
│         ▼                                  ▼                │
│  ┌─────────────┐                    ┌─────────────┐        │
│  │  文件读取    │                    │  文件写入    │        │
│  │  分块处理    │                    │  块校验      │        │
│  │  压缩加密    │                    │  解压解密    │        │
│  └─────────────┘                    └─────────────┘        │
│         │                                  │                │
│         ▼                                  ▼                │
│  ┌─────────────────────────────────────────────────┐       │
│  │              传输管理器 (Manager)                 │       │
│  │  - 任务队列管理                                   │       │
│  │  - 并发控制                                       │       │
│  │  - 状态追踪                                       │       │
│  │  - 统计汇总                                       │       │
│  └─────────────────────────────────────────────────┘       │
│                          │                                  │
│                          ▼                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │              REST API (Handlers)                 │       │
│  │  - 传输任务 CRUD                                 │       │
│  │  - 服务端管理                                     │       │
│  │  - 配置管理                                       │       │
│  │  - 统计查询                                       │       │
│  └─────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 文件结构

```
internal/presto/
├── presto.go           # 核心类型定义和管理器
├── server.go           # QUIC 服务端实现
├── client.go           # QUIC 客户端实现
├── handlers.go         # REST API 处理器
├── presto_test.go      # 单元测试
├── integration_test.go # 集成测试
└── README.md           # 本文档
```

## API 接口

### 传输管理

#### 创建传输任务
```http
POST /api/v1/presto/transfers
Content-Type: application/json

{
  "name": "传输任务名称",
  "source_path": "/path/to/source/file",
  "dest_path": "/path/to/destination",
  "mode": "send"  // send 或 recv
}
```

**响应 (201 Created):**
```json
{
  "id": "uuid",
  "name": "传输任务名称",
  "source_path": "/path/to/source/file",
  "dest_path": "/path/to/destination",
  "mode": "send",
  "status": "pending",
  "total_bytes": 1048576,
  "transferred": 0,
  "progress": 0,
  "chunk_count": 4,
  "chunks_done": 0,
  "speed_bps": 0,
  "speed_human": "0 B/s",
  "compressed": true,
  "encrypted": true,
  "started_at": "2024-01-01T00:00:00Z",
  "elapsed": 0,
  "elapsed_human": "0s"
}
```

#### 列出所有传输任务
```http
GET /api/v1/presto/transfers
GET /api/v1/presto/transfers?status=running
```

#### 获取传输任务详情
```http
GET /api/v1/presto/transfers/{id}
```

#### 取消传输任务
```http
POST /api/v1/presto/transfers/{id}/cancel
```

#### 暂停传输任务
```http
POST /api/v1/presto/transfers/{id}/pause
```

#### 恢复传输任务
```http
POST /api/v1/presto/transfers/{id}/resume
```

#### 删除传输任务
```http
DELETE /api/v1/presto/transfers/{id}
```

### 统计信息

#### 获取传输统计
```http
GET /api/v1/presto/stats
```

**响应:**
```json
{
  "total_transfers": 100,
  "active_transfers": 3,
  "completed_count": 90,
  "failed_count": 7,
  "total_bytes": 1073741824,
  "total_transferred": 1073741824,
  "avg_speed_bps": 10485760,
  "avg_speed_human": "10.00 MB/s"
}
```

### 服务端管理

#### 获取服务端状态
```http
GET /api/v1/presto/server/status
```

#### 启动服务端
```http
POST /api/v1/presto/server/start
```

#### 停止服务端
```http
POST /api/v1/presto/server/stop
```

### 配置管理

#### 获取配置
```http
GET /api/v1/presto/config
```

#### 更新配置
```http
PUT /api/v1/presto/config
Content-Type: application/json

{
  "max_concurrent": 8,
  "chunk_size": 4194304,
  "compression_level": 6,
  "speed_limit": 10485760
}
```

### 清理任务

#### 清理已完成的任务
```http
POST /api/v1/presto/cleanup?hours=24
```

## 配置说明

```go
type Config struct {
    // 服务端监听地址
    ListenAddr string `json:"listen_addr" yaml:"listen_addr"` // 默认: ":9443"
    
    // 最大并发传输数
    MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"` // 默认: 8
    
    // 数据块大小（字节）
    ChunkSize int `json:"chunk_size" yaml:"chunk_size"` // 默认: 4MB
    
    // 是否启用压缩
    EnableCompression bool `json:"enable_compression" yaml:"enable_compression"` // 默认: true
    
    // 压缩级别 1-9
    CompressionLevel int `json:"compression_level" yaml:"compression_level"` // 默认: 6
    
    // 是否启用加密
    EnableEncryption bool `json:"enable_encryption" yaml:"enable_encryption"` // 默认: true
    
    // 加密密钥（32字节 AES-256）
    EncryptionKey []byte `json:"-" yaml:"-"`
    
    // 传输超时时间
    TransferTimeout time.Duration `json:"transfer_timeout" yaml:"transfer_timeout"` // 默认: 30分钟
    
    // 速度限制（字节/秒），0 表示不限速
    SpeedLimit int64 `json:"speed_limit" yaml:"speed_limit"` // 默认: 0
    
    // 存储根目录
    StorageRoot string `json:"storage_root" yaml:"storage_root"` // 默认: "/mnt/presto"
    
    // 临时文件目录
    TempDir string `json:"temp_dir" yaml:"temp_dir"` // 默认: "/tmp/presto"
    
    // TLS 证书文件
    TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
    
    // TLS 密钥文件
    TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file"`
    
    // 是否启用 mTLS（双向认证）
    EnableMTLS bool `json:"enable_mtls" yaml:"enable_mtls"`
    
    // 客户端 CA 证书
    ClientCAFile string `json:"client_ca_file" yaml:"client_ca_file"`
}
```

## 传输协议

### 消息格式

所有消息使用长度前缀 + JSON 格式：

```
[4字节长度][JSON消息体]
```

### 消息类型

| 类型 | 说明 | 方向 |
|------|------|------|
| `handshake` | 握手消息 | 双向 |
| `file_meta` | 文件元数据 | 客户端→服务端 |
| `chunk_req` | 数据块请求 | 客户端→服务端 |
| `chunk_data` | 数据块数据 | 双向 |
| `chunk_ack` | 数据块确认 | 双向 |
| `complete` | 传输完成 | 双向 |
| `error` | 错误消息 | 双向 |
| `resume_req` | 续传请求 | 客户端→服务端 |
| `resume_resp` | 续传响应 | 服务端→客户端 |
| `heartbeat` | 心跳消息 | 双向 |

### 传输流程

```
客户端                                服务端
   │                                    │
   │──── handshake ─────────────────────►│
   │◄─── handshake ─────────────────────│
   │                                    │
   │──── file_meta ─────────────────────►│
   │◄─── chunk_ack (accepted) ──────────│
   │                                    │
   │──── chunk_data (0) ────────────────►│
   │◄─── chunk_ack (ok) ────────────────│
   │                                    │
   │──── chunk_data (1) ────────────────►│
   │◄─── chunk_ack (ok) ────────────────│
   │                                    │
   │           ... (重复直到完成) ...     │
   │                                    │
   │──── complete ──────────────────────►│
   │                                    │
```

### 断点续传流程

```
客户端                                服务端
   │                                    │
   │──── resume_req ────────────────────►│
   │◄─── resume_resp (can_resume=true) ─│
   │        done_chunks: [0, 1, 2]      │
   │                                    │
   │──── chunk_data (3) ────────────────►│  // 从断点继续
   │◄─── chunk_ack (ok) ────────────────│
   │                                    │
   │           ... (继续传输) ...        │
   │                                    │
   │──── complete ──────────────────────►│
   │                                    │
```

## 使用示例

### Go 代码示例

```go
package main

import (
    "context"
    "log"
    "nas-os/internal/presto"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewDevelopment()
    
    // 创建配置
    cfg := presto.DefaultConfig()
    cfg.ListenAddr = ":9443"
    cfg.StorageRoot = "/mnt/presto"
    
    // 创建管理器
    mgr := presto.NewManager(cfg, logger)
    
    // 创建并启动服务端
    server, _ := presto.NewServer(cfg, mgr, logger)
    server.Start(context.Background())
    defer server.Stop()
    
    // 创建客户端
    client := presto.NewClient(cfg, logger)
    client.Connect(context.Background(), "localhost:9443")
    defer client.Disconnect()
    
    // 发送文件
    transfer, err := client.SendFile(
        context.Background(),
        "/path/to/large/file.zip",
        "/destination/file.zip",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("传输已开始: %s", transfer.ID)
}
```

### curl 示例

```bash
# 创建传输任务
curl -X POST http://localhost:8080/api/v1/presto/transfers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "备份文件",
    "source_path": "/data/backup.tar.gz",
    "dest_path": "/remote/backup.tar.gz",
    "mode": "send"
  }'

# 查看传输状态
curl http://localhost:8080/api/v1/presto/transfers/{id}

# 查看统计信息
curl http://localhost:8080/api/v1/presto/stats

# 暂停传输
curl -X POST http://localhost:8080/api/v1/presto/transfers/{id}/pause

# 恢复传输
curl -X POST http://localhost:8080/api/v1/presto/transfers/{id}/resume

# 取消传输
curl -X POST http://localhost:8080/api/v1/presto/transfers/{id}/cancel
```

## 性能优化建议

1. **调整块大小**: 根据网络环境调整 `ChunkSize`，局域网可使用更大的块（8-16MB）
2. **并发数**: 根据 CPU 核心数和网络带宽调整 `MaxConcurrent`
3. **压缩级别**: 高带宽网络可降低压缩级别以减少 CPU 开销
4. **速度限制**: 在共享网络环境中设置合理的速度限制

## 安全建议

1. **生产环境必须配置 TLS 证书**，不要使用自签名证书
2. **启用 mTLS** 进行双向认证
3. **定期轮换加密密钥**
4. **限制访问 IP** 通过防火墙规则

## 依赖

- `github.com/quic-go/quic-go`: QUIC 协议实现
- `github.com/gin-gonic/gin`: HTTP 框架
- `go.uber.org/zap`: 日志库
- `github.com/google/uuid`: UUID 生成
- `github.com/stretchr/testify`: 测试框架

## 后续计划

- [ ] 实现 zstd 压缩算法
- [ ] 支持目录传输（自动打包）
- [ ] 支持多文件批量传输
- [ ] 实现 Web UI 界面
- [ ] 支持 P2P 传输模式
- [ ] 集成 Prometheus 监控指标
- [ ] 支持传输带宽调度
