# 工部：Fleet复制架构设计

## 概述
对标TrueNAS Replication，设计nas-os多节点数据复制架构。

## 架构设计

### 1. 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                  Fleet Replication Manager               │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │ Task Queue  │  │ Scheduler   │  │ Progress    │      │
│  │             │→ │             │→ │ Tracker     │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │ Bandwidth   │  │ Compression │  │ Encryption  │      │
│  │ Controller  │  │ Engine      │  │ Layer       │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
└─────────────────────────────────────────────────────────┘
           │                    │                    │
           ▼                    ▼                    ▼
    ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
    │   Node A    │      │   Node B    │      │   Node C    │
    │  (Source)   │─────→│  (Target)   │─────→│  (Target)   │
    └─────────────┘      └─────────────┘      └─────────────┘
```

### 2. 核心组件

#### FleetReplicationManager

```go
// FleetReplicationManager 多节点复制管理器
type FleetReplicationManager struct {
    taskQueue     *TaskQueue
    scheduler     *ReplicationScheduler
    progressTracker *ProgressTracker
    bandwidthCtrl *BandwidthController
    nodes         NodeRegistry
}

// TaskQueue 复制任务队列
type TaskQueue struct {
    tasks    map[string]*ReplicationTask
    priorityQueues [3]*PriorityQueue // P0, P1, P2
    mutex    sync.RWMutex
}

// ReplicationScheduler 复制调度器
type ReplicationScheduler struct {
    maxConcurrent int        // 最大并行任务数
    allocations   map[string]*Allocation
    bandwidthPool *BandwidthPool
}

// ProgressTracker 进度追踪器
type ProgressTracker struct {
    progress map[string]*ReplicationProgress
    history  map[string][]ProgressSnapshot
}

// BandwidthController 带宽控制器
type BandwidthController struct {
    totalLimit   int        // 总带宽限制 MB/s
    nodeLimits   map[string]int
    adaptiveMode bool       // 自适应带宽调整
}
```

### 3. 复制流程

```
1. 任务创建
   POST /api/v1/fleet/tasks
   → TaskQueue.Enqueue()

2. 任务调度
   Scheduler.SelectTask()
   → CheckBandwidth()
   → AllocateResources()
   → StartReplication()

3. 数据传输
   SourceNode.PrepareData()
   → CompressionEngine.Compress()
   → EncryptionLayer.Encrypt()
   → TargetNode.Receive()

4. 进度追踪
   ProgressTracker.Update()
   → StoreSnapshot()
   → NotifyObservers()

5. 断点续传
   Resume → LoadLastSnapshot()
   → SeekToPosition()
   → ContinueTransfer()
```

### 4. 调度策略

| 策略 | 描述 |
|------|------|
| **优先级调度** | P0任务优先执行，P1/P2按队列顺序 |
| **带宽自适应** | 根据网络状况动态调整传输速度 |
| **节点负载均衡** | 避免单节点过载，分散任务 |
| **增量复制** | 仅传输变更数据，减少带宽消耗 |
| **并行复制** | 多目标节点并行传输 |

### 5. 增量复制优化

```go
// IncrementalReplicator 增量复制器
type IncrementalReplicator struct {
    snapshotCache *SnapshotCache
    changeDetector *ChangeDetector
    deltaEncoder  *DeltaEncoder
}

// ChangeDetector 变化检测
func (d *ChangeDetector) DetectChanges(sourcePath, lastSnapshot string) []Change {
    // 1. 比较当前状态与上次快照
    // 2. 检测新增/修改/删除文件
    // 3. 计算差异块
    // 4. 返回变更列表
}

// DeltaEncoder 差异编码
func (e *DeltaEncoder) EncodeDelta(changes []Change) []byte {
    // 1. 压缩差异数据
    // 2. 添加校验信息
    // 3. 返回编码结果
}
```

### 6. 带宽自适应

```go
// BandwidthAdaptor 带宽自适应器
type BandwidthAdaptor struct {
    sampleWindow  time.Duration
    minSpeed      float64
    maxSpeed      float64
    targetLatency time.Duration
}

// AdjustBandwidth 自适应调整
func (a *BandwidthAdaptor) AdjustBandwidth(currentSpeed, latency float64) int {
    // 1. 监测网络延迟
    // 2. 检测丢包率
    // 3. 计算最优带宽
    // 4. 动态调整限速
}
```

## 实现计划

| 阶段 | 任务 | 时间 |
|------|------|------|
| M1 | TaskQueue + Scheduler | 04-03 |
| M2 | BandwidthController | 04-05 |
| M3 | IncrementalReplicator | 04-08 |
| M4 | ProgressTracker + Resume | 04-10 |
| M5 | 集成测试 | 04-15 |

## 与现有系统集成

- 扩展 `internal/replication/` 现有实现
- 利用 `internal/cluster/` 节点管理能力
- 集成 `internal/storage/snapshot.go` 快照功能