# ZFS Fast Deduplication API 设计文档

## 概述

对标 TrueNAS 25.04.2 的 ZFS Fast Deduplication 功能，设计 nas-os 的快速去重 API。

## TrueNAS Fast Dedup 特性分析

TrueNAS 25.04.2 引入的 Fast Deduplication 主要特性：
1. **在线去重（Inline Dedup）**：写入时实时检测重复块
2. **快速哈希索引**：使用 Bloom Filter 加速块查找
3. **内存优化**：减少 DDT（Dedup Table）内存占用
4. **异步去重**：后台批量处理已存在数据

## API 设计

### 1. Fast Dedup 配置 API

```go
// FastDedupConfig 快速去重配置
type FastDedupConfig struct {
    // 基础配置
    Enabled         bool   `json:"enabled"`
    Mode            string `json:"mode"` // inline, async, hybrid
    
    // ZFS 池配置
    PoolName        string `json:"poolName"`
    Dataset         string `json:"dataset"`
    
    // 性能配置
    HashAlgorithm   string `json:"hashAlgorithm"` // sha256, sha512, skein
    ChunkSize       int64  `json:"chunkSize"`     // 4KB-64KB
    MaxMemoryMB     int    `json:"maxMemoryMB"`   // DDT 内存限制
    
    // Bloom Filter 配置
    BloomFilterSize int    `json:"bloomFilterSize"` // 位图大小
    BloomFilterK    int    `json:"bloomFilterK"`    // 哈希函数数量
    
    // 异步去重配置
    AsyncSchedule   string `json:"asyncSchedule"`   // cron 表达式
    AsyncBatchSize  int    `json:"asyncBatchSize"`  // 批量处理大小
    
    // 统计配置
    StatsInterval   int    `json:"statsInterval"`   // 统计上报间隔（秒）
}
```

### 2. Fast Dedup 状态 API

```go
// FastDedupStatus 快速去重状态
type FastDedupStatus struct {
    // 运行状态
    State           string    `json:"state"` // idle, scanning, deduping, error
    LastScanTime    time.Time `json:"lastScanTime"`
    LastDedupTime   time.Time `json:"lastDedupTime"`
    
    // 统计信息
    TotalBlocks     int64     `json:"totalBlocks"`
    UniqueBlocks    int64     `json:"uniqueBlocks"`
    DuplicateBlocks int64     `json:"duplicateBlocks"`
    
    // 空间统计
    TotalDataSize   int64     `json:"totalDataSize"`
    DedupedSize     int64     `json:"dedupedSize"`
    SavingsRatio    float64   `json:"savingsRatio"`
    
    // 性能指标
    DDTMemoryUsage  int64     `json:"ddtMemoryUsage"`  // DDT 内存占用
    BloomHitRate    float64   `json:"bloomHitRate"`    // Bloom Filter 命中率
    AvgLatencyMs    float64   `json:"avgLatencyMs"`    // 平均延迟
    
    // 错误信息
    Errors          []string  `json:"errors,omitempty"`
}
```

### 3. Fast Dedup 操作 API

```go
// FastDedupRequest 快速去重请求
type FastDedupRequest struct {
    // 目标配置
    PoolName    string `json:"poolName"`
    Dataset     string `json:"dataset"`
    
    // 操作类型
    Action      string `json:"action"` // enable, disable, scan, dedup, stats
    
    // 可选参数
    Force       bool   `json:"force"`       // 强制执行
    DryRun      bool   `json:"dryRun"`      // 试运行
    ProgressCB  bool   `json:"progressCb"`  // 是否需要进度回调
}

// FastDedupProgress 去重进度
type FastDedupProgress struct {
    Phase       string    `json:"phase"` // init, scan, hash, dedup, cleanup
    Current     int64     `json:"current"`
    Total       int64     `json:"total"`
    Percent     float64   `json:"percent"`
    SpeedMBps   float64   `json:"speedMBps"`
    ETASeconds  int       `json:"etaSeconds"`
    Message     string    `json:"message"`
    Timestamp   time.Time `json:"timestamp"`
}

// FastDedupResult 去重结果
type FastDedupResult struct {
    Success         bool      `json:"success"`
    BlocksProcessed int64     `json:"blocksProcessed"`
    BlocksDeduped   int64     `json:"blocksDeduped"`
    BytesSaved      int64     `json:"bytesSaved"`
    Duration        time.Duration `json:"duration"`
    Errors          []DedupError `json:"errors,omitempty"`
}

type DedupError struct {
    BlockHash string `json:"blockHash"`
    Path      string `json:"path"`
    Error     string `json:"error"`
}
```

### 4. HTTP API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/zfs/fast-dedup/config` | GET/PUT | 获取/更新配置 |
| `/api/v1/zfs/fast-dedup/status` | GET | 获取状态 |
| `/api/v1/zfs/fast-dedup/enable` | POST | 启用快速去重 |
| `/api/v1/zfs/fast-dedup/disable` | POST | 禁用快速去重 |
| `/api/v1/zfs/fast-dedup/scan` | POST | 执行扫描 |
| `/api/v1/zfs/fast-dedup/dedup` | POST | 执行去重 |
| `/api/v1/zfs/fast-dedup/progress` | GET | 获取进度（WebSocket推送） |
| `/api/v1/zfs/fast-dedup/stats` | GET | 获取统计 |

### 5. WebSocket 实时进度推送

```json
{
  "type": "fast_dedup_progress",
  "data": {
    "phase": "hash",
    "current": 150000,
    "total": 500000,
    "percent": 30.0,
    "speedMBps": 125.5,
    "etaSeconds": 2800,
    "message": "计算块哈希中..."
  }
}
```

## Bloom Filter 实现

```go
// BloomFilter 快速哈希查找过滤器
type BloomFilter struct {
    bits     []uint64
    size     int
    k        int      // 哈希函数数量
    hashFunc func([]byte) uint64
}

// NewBloomFilter 创建 Bloom Filter
func NewBloomFilter(size int, k int) *BloomFilter {
    return &BloomFilter{
        bits: make([]uint64, (size/64)+1),
        size: size,
        k:    k,
    }
}

// Add 添加元素
func (bf *BloomFilter) Add(hash []byte) {
    for i := 0; i < bf.k; i++ {
        idx := bf.hashWithSeed(hash, i) % uint64(bf.size)
        bf.bits[idx/64] |= (1 << (idx % 64))
    }
}

// MightContain 检查可能存在（有假阳性）
func (bf *BloomFilter) MightContain(hash []byte) bool {
    for i := 0; i < bf.k; i++ {
        idx := bf.hashWithSeed(hash, i) % uint64(bf.size)
        if bf.bits[idx/64] & (1 << (idx % 64)) == 0 {
            return false
        }
    }
    return true
}
```

## DDT（Dedup Table）优化

```go
// DedupTable 去重表
type DedupTable struct {
    entries  map[string]*DedupEntry
    bloom    *BloomFilter
    memoryMB int
    mu       sync.RWMutex
}

// DedupEntry 去重条目
type DedupEntry struct {
    Hash       string    `json:"hash"`
    BlockCount int64     `json:"blockCount"`
    RefCount   int32     `json:"refCount"`
    Size       int64     `json:"size"`
    FirstSeen  time.Time `json:"firstSeen"`
    LastSeen   time.Time `json:"lastSeen"`
}

// Lookup 查找块（先 Bloom Filter，再精确查找）
func (dt *DedupTable) Lookup(hash string) (*DedupEntry, bool) {
    // 快速 Bloom Filter 检查
    if !dt.bloom.MightContain([]byte(hash)) {
        return nil, false
    }
    
    // 精确查找
    dt.mu.RLock()
    defer dt.mu.RUnlock()
    entry, exists := dt.entries[hash]
    return entry, exists
}
```

## 与现有 dedup 模块集成

现有的 `internal/dedup` 模块提供文件级去重，ZFS Fast Dedup 提供块级去重：

1. **文件级去重**：适用于用户文件管理，跨用户共享检测
2. **块级去重**：适用于 ZFS 存储池，在线写入去重

集成策略：
- 保持现有 `dedup` 模块不变
- 新增 `internal/zfs/fastdedup` 模块
- 通过统一 API 层协调两种去重策略

## 实现优先级

| 阶段 | 功能 | 优先级 | 预计工作量 |
|------|------|--------|-----------|
| P1 | API 结构定义 | P0 | 2h |
| P2 | Bloom Filter 实现 | P1 | 4h |
| P3 | DDT 内存优化 | P1 | 6h |
| P4 | WebSocket 进度推送 | P1 | 3h |
| P5 | ZFS 命令集成 | P2 | 8h |
| P6 | WebUI 组件 | P2 | 6h |