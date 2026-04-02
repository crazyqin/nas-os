# RAIDZ Expansion API 设计文档

**版本**: v1.0.0  
**日期**: 2026-04-02  
**作者**: 兵部  
**参考**: TrueNAS Electric Eel 24.10, OpenZFS 2.3

---

## 1. 概述

### 1.1 功能目标

实现 NAS-OS 的 RAIDZ 单盘扩容功能，对标 TrueNAS 24.10 Electric Eel 的 RAIDZ Expansion 特性：

- **核心能力**: 向现有 RAIDZ vdev 单盘扩容
- **在线扩容**: 扩容过程中池可读写
- **进度监控**: 实时显示扩容进度、速度、ETA
- **中断恢复**: 支持暂停/恢复、重启后自动继续

### 1.2 技术背景

OpenZFS 2.3 引入的 RAIDZ Expansion 特性：
- 命令: `zpool attach POOL raidzP-N NEW_DEVICE`
- 特性标志: `feature@raidz_expansion`
- 数据迁移: 自动重分布数据到新配置

---

## 2. 系统架构

### 2.1 分层架构

```
┌────────────────────────────────────────────────────────────────┐
│                      Web UI (前端)                              │
│  - 存储池管理页面                                               │
│  - 扩容进度组件                                                 │
│  - 磁盘选择器                                                   │
└────────────────────────────────────────────────────────────────┘
                              ↓ REST API
┌────────────────────────────────────────────────────────────────┐
│                    API Handler Layer                           │
│  internal/storage/raidz_handlers.go                            │
│  - HTTP 请求处理                                               │
│  - 参数验证                                                     │
│  - 响应格式化                                                   │
└────────────────────────────────────────────────────────────────┘
                              ↓ Service Interface
┌────────────────────────────────────────────────────────────────┐
│                     Service Layer                              │
│  internal/storage/raidz_service.go                             │
│  - 业务逻辑                                                     │
│  - 任务管理                                                     │
│  - 状态追踪                                                     │
│  - 回调通知                                                     │
└────────────────────────────────────────────────────────────────┘
                              ↓ ZFS Interface
┌────────────────────────────────────────────────────────────────┐
│                     ZFS Manager Layer                          │
│  pkg/storage/zfs/raidz_expansion.go                            │
│  - ZFS 命令封装                                                 │
│  - 进度解析                                                     │
│  - 错误处理                                                     │
└────────────────────────────────────────────────────────────────┘
                              ↓ CLI
┌────────────────────────────────────────────────────────────────┐
│                    OpenZFS 2.3 CLI                             │
│  zpool attach, zpool status, zpool list                        │
└────────────────────────────────────────────────────────────────┘
```

### 2.2 核心模块

| 模块 | 文件路径 | 职责 |
|------|----------|------|
| ZFS Manager | `pkg/storage/zfs/raidz_expansion.go` | ZFS 命令封装、状态解析 |
| 接口定义 | `pkg/storage/zfs/expansion_api.go` | 类型定义、接口契约 |
| 业务服务 | `internal/storage/raidz_service.go` | 任务管理、进度追踪 |
| API 处理器 | `internal/storage/raidz_handlers.go` | HTTP 接入 |

---

## 3. API 设计

### 3.1 REST API 端点

**基础路径**: `/api/v1/storage/raidz-expansion`

| 方法 | 路径 | 功能 | 认证 |
|------|------|------|------|
| GET | `/eligibility/:pool` | 检查扩容资格 | ✅ |
| GET | `/status/:pool` | 获取扩容状态 | ✅ |
| GET | `/tasks` | 获取所有活跃任务 | ✅ |
| POST | `/start` | 启动扩容 | ✅ |
| POST | `/pause/:pool` | 暂停扩容 | ✅ |
| POST | `/resume/:pool` | 恢复扩容 | ✅ |
| POST | `/cancel/:pool` | 取消扩容 | ✅ |
| GET | `/available-disks` | 获取可用磁盘 | ✅ |
| GET | `/history` | 获取扩容历史 | ✅ |
| POST | `/estimate` | 预估扩容效果 | ✅ |
| GET | `/service-status` | 服务状态检查 | ✅ |

### 3.2 API 详情

#### 3.2.1 检查扩容资格

**请求**:
```
GET /api/v1/storage/raidz-expansion/eligibility/:pool
```

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "pool_name": "tank",
    "eligible": true,
    "raidz_level": "raidz2",
    "current_width": 4,
    "new_width": 5,
    "capacity_gain": 107374182400,
    "current_capacity": 429496729600,
    "new_capacity": 536870912000,
    "warnings": [],
    "pre_checks": [
      {
        "name": "pool_health",
        "passed": true,
        "message": "ONLINE",
        "required": true
      },
      {
        "name": "scan_status",
        "passed": true,
        "message": "no active scan",
        "required": true
      },
      {
        "name": "vdev_type",
        "passed": true,
        "message": "raidz2 (4 disks)",
        "required": true
      }
    ],
    "estimated_time": "3h30m",
    "disk_requirements": {
      "min_size_gb": 100,
      "recommended_gb": 100,
      "interfaces": ["SATA", "NVMe", "SAS"],
      "must_match_size": false
    }
  }
}
```

#### 3.2.2 启动扩容

**请求**:
```
POST /api/v1/storage/raidz-expansion/start
Content-Type: application/json

{
  "pool_name": "tank",
  "new_disk": "/dev/sde",
  "force": false,
  "confirm": true
}
```

**响应**:
```json
{
  "code": 200,
  "message": "RAIDZ扩容已启动",
  "data": {
    "id": "raidz-exp-tank-1712345678901",
    "pool_name": "tank",
    "new_disk": "/dev/sde",
    "raidz_level": "raidz2",
    "status": "preparing",
    "progress": 0,
    "bytes_processed": 0,
    "total_bytes": 429496729600,
    "speed_mbps": 0,
    "start_time": "2026-04-02T12:00:00Z",
    "eta": "3h30m",
    "can_pause": true,
    "can_cancel": true,
    "can_resume": false,
    "warnings": []
  }
}
```

#### 3.2.3 获取扩容状态

**请求**:
```
GET /api/v1/storage/raidz-expansion/status/:pool
```

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "raidz-exp-tank-1712345678901",
    "pool_name": "tank",
    "status": "running",
    "progress": 45.2,
    "bytes_processed": 193273528320,
    "total_bytes": 429496729600,
    "speed_mbps": 245.5,
    "eta": "2h15m",
    "start_time": "2026-04-02T12:00:00Z",
    "last_update": "2026-04-02T13:30:00Z",
    "can_pause": true,
    "can_cancel": true,
    "can_resume": false
  }
}
```

#### 3.2.4 预估扩容效果

**请求**:
```
POST /api/v1/storage/raidz-expansion/estimate
Content-Type: application/json

{
  "pool_name": "tank"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "pool_name": "tank",
    "current_capacity_gb": 400,
    "capacity_gain_gb": 100,
    "new_capacity_gb": 500,
    "estimated_time": "3h30m",
    "estimated_minutes": 210,
    "raidz_level": "raidz2",
    "current_width": 4,
    "new_width": 5,
    "pre_checks_passed": 3,
    "pre_checks_failed": 0,
    "warnings": [],
    "eligible": true,
    "disk_requirements": {
      "min_size_gb": 100,
      "recommended_gb": 100,
      "interfaces": ["SATA", "NVMe", "SAS"],
      "must_match_size": false
    }
  }
}
```

---

## 4. 数据模型

### 4.1 核心类型

#### ExpansionTask (扩容任务)

```go
type ExpansionTask struct {
    ID             string            `json:"id"`              // 任务ID
    PoolName       string            `json:"pool_name"`       // 池名称
    NewDisk        string            `json:"new_disk"`        // 新磁盘路径
    RAIDZLevel     string            `json:"raidz_level"`     // RAIDZ级别
    Status         ExpansionStatus   `json:"status"`          // 任务状态
    Progress       float64           `json:"progress"`        // 进度(0-100)
    BytesProcessed uint64            `json:"bytes_processed"` // 已处理字节
    TotalBytes     uint64            `json:"total_bytes"`     // 总字节
    SpeedMBps      float64           `json:"speed_mbps"`      // 速度(MB/s)
    StartTime      time.Time         `json:"start_time"`      // 开始时间
    EndTime        *time.Time        `json:"end_time"`        // 结束时间
    ETA            time.Duration     `json:"eta"`             // 预估剩余时间
    Errors         []string          `json:"errors"`          // 错误信息
    Warnings       []string          `json:"warnings"`        // 警告信息
    CanPause       bool              `json:"can_pause"`       // 是否可暂停
    CanCancel      bool              `json:"can_cancel"`      // 是否可取消
    CanResume      bool              `json:"can_resume"`      // 是否可恢复
    PauseCount     int               `json:"pause_count"`     // 暂停次数
    Metadata       map[string]string `json:"metadata"`        // 元数据
    LastUpdate     time.Time         `json:"last_update"`     // 最后更新
}
```

#### ExpansionStatus (任务状态)

```go
type ExpansionStatus string

const (
    StatusIdle       ExpansionStatus = "idle"       // 空闲
    StatusPreparing  ExpansionStatus = "preparing"  // 准备中
    StatusRunning    ExpansionStatus = "running"    // 运行中
    StatusPaused     ExpansionStatus = "paused"     // 已暂停
    StatusCompleted  ExpansionStatus = "completed"  // 已完成
    StatusFailed     ExpansionStatus = "failed"     // 失败
    StatusCancelled  ExpansionStatus = "cancelled"  // 已取消
)
```

#### ExpansionEligibilityResult (资格检查结果)

```go
type ExpansionEligibilityResult struct {
    PoolName        string           `json:"pool_name"`
    Eligible        bool             `json:"eligible"`
    RAIDZLevel      string           `json:"raidz_level"`
    CurrentWidth    int              `json:"current_width"`    // 当前磁盘数
    NewWidth        int              `json:"new_width"`        // 扩容后磁盘数
    CapacityGain    uint64           `json:"capacity_gain"`    // 容量增益(bytes)
    CurrentCapacity uint64           `json:"current_capacity"` // 当前容量
    NewCapacity     uint64           `json:"new_capacity"`     // 新容量
    Warnings        []string         `json:"warnings"`
    PreChecks       []PreCheckResult `json:"pre_checks"`
    EstimatedTime   time.Duration    `json:"estimated_time"`
    DiskRequirements DiskRequirements `json:"disk_requirements"`
}
```

---

## 5. 状态监控接口

### 5.1 WebSocket 进度推送

**端点**: `/ws/storage/raidz-expansion/:pool`

**消息格式**:
```json
{
  "type": "progress",
  "task_id": "raidz-exp-tank-1712345678901",
  "pool_name": "tank",
  "timestamp": "2026-04-02T13:30:00Z",
  "data": {
    "percentage": 45.2,
    "bytes_processed": 193273528320,
    "bytes_total": 429496729600,
    "speed_mbps": 245.5,
    "eta_seconds": 8100,
    "phase": "data_migration"
  }
}
```

### 5.2 Prometheus Metrics

**指标端点**: `/metrics`

```prometheus
# RAIDZ Expansion Metrics
raidz_expansion_tasks_active 1
raidz_expansion_progress_percentage{pool="tank"} 45.2
raidz_expansion_bytes_processed{pool="tank"} 193273528320
raidz_expansion_bytes_total{pool="tank"} 429496729600
raidz_expansion_speed_mbps{pool="tank"} 245.5
raidz_expansion_eta_seconds{pool="tank"} 8100
raidz_expansion_pause_count{pool="tank"} 0
raidz_expansion_errors_total{pool="tank"} 0
raidz_expansion_tasks_completed_total 5
raidz_expansion_tasks_failed_total 0
```

### 5.3 状态回调接口

```go
// 进度回调
type ProgressCallback func(progress *ExpansionProgress)

// 状态变更回调
type StateChangeCallback func(task *ExpansionTask)

// 注册回调
service.RegisterProgressCallback(poolName, func(p *ExpansionProgress) {
    // 处理进度更新
})

service.RegisterStateCallback(func(task *ExpansionTask) {
    // 处理状态变更
})
```

---

## 6. 错误处理

### 6.1 错误类型

```go
var (
    ErrExpansionNotReady    = fmt.Errorf("pool not ready for expansion")
    ErrDiskTooSmall         = fmt.Errorf("new disk too small")
    ErrResilverInProgress   = fmt.Errorf("resilver operation in progress")
    ErrScrubInProgress      = fmt.Errorf("scrub operation in progress")
    ErrPoolNotHealthy       = fmt.Errorf("pool health check failed")
    ErrExpansionAlreadyRuns = fmt.Errorf("expansion already running on this pool")
    ErrZFSNotAvailable      = fmt.Errorf("ZFS not available")
    ErrExpansionNotSupported = fmt.Errorf("RAIDZ expansion not supported")
)
```

### 6.2 HTTP 状态码映射

| 错误类型 | HTTP 状态码 | 错误码 |
|----------|-------------|--------|
| ErrExpansionNotReady | 400 | 40001 |
| ErrDiskTooSmall | 400 | 40002 |
| ErrResilverInProgress | 409 | 40901 |
| ErrScrubInProgress | 409 | 40902 |
| ErrPoolNotHealthy | 503 | 50301 |
| ErrExpansionAlreadyRuns | 409 | 40903 |
| ErrZFSNotAvailable | 503 | 50302 |

---

## 7. 安全设计

### 7.1 操作确认机制

```go
// 启动扩容必须明确确认
type StartExpansionRequest struct {
    PoolName string `json:"pool_name" binding:"required"`
    NewDisk  string `json:"new_disk" binding:"required"`
    Force    bool   `json:"force"`
    Confirm  bool   `json:"confirm"` // 必须为 true
}

// 未确认时返回错误
if !req.Confirm {
    return api.BadRequest(c, "扩容操作需要明确确认（confirm=true）")
}
```

### 7.2 权限控制

```yaml
# RBAC 权限定义
permissions:
  storage.raidz-expansion.read:
    description: 查看RAIDZ扩容状态
    roles: [admin, operator, viewer]
  
  storage.raidz-expansion.write:
    description: 执行RAIDZ扩容操作
    roles: [admin, operator]
  
  storage.raidz-expansion.control:
    description: 暂停/恢复/取消扩容
    roles: [admin]
```

### 7.3 操作审计

```go
// 扩容操作审计日志
type ExpansionAuditLog struct {
    Timestamp   time.Time `json:"timestamp"`
    User        string    `json:"user"`
    Action      string    `json:"action"`     // start, pause, resume, cancel
    PoolName    string    `json:"pool_name"`
    NewDisk     string    `json:"new_disk"`
    Result      string    `json:"result"`     // success, failure
    IPAddress   string    `json:"ip_address"`
    Duration    int       `json:"duration"`   // 操作耗时(ms)
}
```

---

## 8. 容量计算

### 8.1 容量增益公式

```
原始容量 = DiskSize × (N_old - P)
新容量 = DiskSize × (N_new - P)
容量增益 = DiskSize × (N_new - P) - DiskSize × (N_old - P)

其中:
- N_old: 原始磁盘数
- N_new: 扩容后磁盘数
- P: 奇偶校验盘数 (RAIDZ1=1, RAIDZ2=2, RAIDZ3=3)
```

### 8.2 容量折损说明

**重要提示**: RAIDZ 扩容存在"容量折损"现象

```
旧数据块保持原有奇偶比，分布到更多磁盘
新数据块采用新的奇偶比

示例 (4盘 RAIDZ2 → 5盘):
- 旧块奇偶比: 2数据 : 2校验
- 新块奇偶比: 3数据 : 2校验
- 报告容量取保守值（旧奇偶比）
- 容量折损率约 25%
```

**恢复方法**:
1. 自然恢复：随数据修改/删除逐步释放
2. 主动恢复：复制重写数据到扩展池

---

## 9. 最佳实践

### 9.1 执行时机建议

| 场景 | 建议 |
|------|------|
| 低负载时段 | ✅ 推荐 |
| 高负载时段 | ⚠️ 避免 |
| 有充足备份 | ✅ 必须 |
| 磁盘接近寿命极限 | ❌ 先更换 |

### 9.2 硬件建议

1. 新盘容量 ≥ 现有盘最小容量
2. 建议相同型号磁盘
3. 预留备用盘以防故障

### 9.3 扩容后操作

```bash
# 验证扩容完成
zpool status tank

# 检查容量
zpool list tank

# 可选：主动恢复容量折损
zfs send tank/data@snap | zfs recv tank/data_new
zfs destroy tank/data
zfs rename tank/data_new tank/data
```

---

## 10. 参考资源

- [TrueNAS 24.10 Pool Management](https://www.truenas.com/docs/scale/24.10/scaletutorials/storage/managepoolsscale/)
- [OpenZFS RAIDZ Expansion PR #15022](https://github.com/openzfs/zfs/pull/15022)
- [RAIDZ Extension Calculator](https://www.truenas.com/docs/references/extensioncalculator/)
- nas-os 现有实现:
  - `pkg/storage/zfs/raidz_expansion.go`
  - `internal/storage/raidz_service.go`
  - `internal/storage/raidz_handlers.go`

---

**文档完成 | 兵部 | 2026-04-02**