# 高性能存储模块

本模块提供企业级高性能存储功能，包括：

## 1. Fast Deduplication (pkg/storage/dedup/)

快速去重模块，参考 TrueNAS 和 OpenZFS DDT 架构设计：

### 核心特性
- **DDT (Deduplication Table)**: 内存哈希表 + LRU 缓存混合架构
- **高性能哈希**: 支持 SHA256、BLAKE3、XXHash
- **块级去重**: 支持 4KB - 128KB 可配置块大小
- **快速路径缓存**: 高频访问条目的 LRU 缓存
- **后台清理**: 自动清理低引用计数条目

### 使用示例
```go
import "nas-os/pkg/storage/dedup"

// 创建去重器
config := dedup.DefaultDedupConfig()
deduplicator, err := dedup.NewFastDeduplicator(config)

// 执行去重写入
entry, deduped, err := deduplicator.DedupWrite(ctx, data)

// 获取统计
stats := deduplicator.GetStats()
```

## 2. RDMA 支持 (pkg/storage/rdma/)

RDMA (Remote Direct Memory Access) 高性能传输模块：

### 支持协议
- **iSER (iSCSI Extensions for RDMA)**: 高性能块存储
- **NFS over RDMA**: 高性能文件存储

### 核心特性
- **零拷贝传输**: 绕过内核，直接内存访问
- **队列对管理**: 支持多队列并发
- **内存区域管理**: 自动 MR 注册和注销
- **性能监控**: 实时吞吐量和延迟统计

### 使用示例
```go
import "nas-os/pkg/storage/rdma"

// 创建 RDMA 管理器
mgr, _ := rdma.NewRDMAManager(rdma.DefaultRDMAConfig())

// 创建 iSER 会话
session, _ := rdma.NewISERSession(&rdma.ISERConfig{
    TargetIQN: "iqn.2024.nas-os:target1",
    TargetAddr: "192.168.1.100",
})
session.Connect(ctx)

// 执行 RDMA 读写
session.Read(ctx, lba, buffer)
session.Write(ctx, lba, data)
```

## 3. ZFS 不可变快照 (pkg/storage/zfs/)

企业级 ZFS 不可变快照管理：

### 核心特性
- **多种锁类型**: Soft、Hard、Timed、Permanent
- **保留策略**: 按数量、时间、大小组合保留
- **完整性验证**: SHA256 校验和验证
- **书签支持**: 快照书签用于增量复制
- **审批流程**: 敏感操作需要审批

### 使用示例
```go
import "nas-os/pkg/storage/zfs"

// 创建 ZFS 管理器
mgr, _ := zfs.NewZFSManager("/etc/nas-os/zfs.json", zfs.DefaultImmutablePolicy())

// 创建不可变快照
snap, _ := mgr.CreateSnapshot(ctx, "pool/dataset", zfs.SnapshotCreateOptions{
    Name:       "backup-20240301",
    Immutable:  true,
    LockType:   zfs.LockTypeHard,
    ExpiryTime: &expiry,
})

// 验证快照
mgr.VerifySnapshot(ctx, snap.FullName)
```

## 配置参考

### dedup.DedupConfig
| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| ChunkSize | uint32 | 32768 | 块大小 (4KB-128KB) |
| HashAlgorithm | string | "sha256" | 哈希算法 |
| MaxMemoryMB | uint32 | 1024 | 最大内存使用 |
| EnableFastPath | bool | true | 启用快速路径缓存 |

### rdma.RDMAConfig
| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| Transport | string | "iSER" | 传输协议 |
| Port | int | 3260 | 服务端口 |
| MaxQP | int | 1024 | 最大队列对数 |
| InlineSize | int | 128 | 内联数据大小 |

### zfs.ImmutablePolicy
| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| DefaultLockType | LockType | "soft" | 默认锁类型 |
| DefaultRetentionDays | int | 365 | 默认保留天数 |
| RequireApproval | bool | true | 是否需要审批 |
| AutoVerifyInterval | int | 24 | 自动验证间隔(小时) |