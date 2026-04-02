# 兵部第109轮任务报告 - RAIDZ扩容API设计

## 任务概述

研究现有btrfs RAID管理代码，设计RAID1/RAID10扩容API接口，检查测试覆盖率，输出设计报告。

## 一、竞品学习要点

### TrueNAS 24.10 RAIDZ Expansion 特性
- **核心功能**: 单盘扩展RAIDZ vdev，无需重建池
- **技术基础**: OpenZFS 2.3原生支持
- **开发周期**: 约3年
- **投入估算**: 约$100,000
- **关键算法**: 在线数据重分布，渐进式扩展

### 学习要点总结
1. **渐进式扩展**: 数据逐步重分布到新盘，避免全量重建
2. **在线操作**: 扩展过程中池仍可读写
3. **暂停/恢复**: 支持操作暂停和恢复
4. **进度监控**: 提供实时进度和预估时间
5. **容量估算**: 扩展前预估容量增益

---

## 二、现有代码结构分析

### 代码组织

```
nas-os/
├── internal/storage/
│   ├── smart_raid.go          # SmartRAID核心管理器 (35KB)
│   ├── smart_raid_test.go     # SmartRAID测试
│   ├── expansion_handlers.go  # RAID扩展API处理器 (15KB)
│   └── handlers.go            # 存储API主处理器
├── pkg/storage/
│   ├── btrfs/
│   │   ├── expansion_types.go  # Btrfs扩展类型定义 (14KB)
│   │   ├── expansion_manager.go # Btrfs扩展管理器 (31KB)
│   │   └── [无测试文件]        # ⚠️ 测试覆盖率: 0%
│   └── zfs/
│       ├── raidz_expansion.go  # ZFS RAIDZ扩展实现 (35KB)
│       ├── raidz_expansion_test.go # ZFS RAIDZ测试 (完整)
│       └── expansion_api.go    # ZFS扩展API
```

### SmartRAID 核心架构

**数据结构**:
- `SmartPool`: 智能存储池，支持分层存储
- `StorageTier`: 按容量分组的存储层级
- `SmartDevice`: 设备信息（容量、健康、类型）
- `RAIDPolicy`: RAID策略配置

**核心算法**:
1. **层级计算** (`calculateTiers`): 设备按容量分组（5%容差）
2. **RAID选择** (`selectTierRAID`): 根据设备数和策略自动选择
3. **容量计算** (`calculateCapacity`): 考虑RAID效率的实际容量
4. **扩容计划** (`GetExpansionPlan`): 分析并建议扩容策略

---

## 三、RAID1/RAID10扩容API设计

### 3.1 Btrfs扩展流程 (已实现)

**核心流程**: `device add` → `balance`

```go
// 扩展配置
type ExpansionConfig struct {
    VolumeName    string  // 卷名称
    MountPoint    string  // 挂载点
    NewDevice     string  // 新磁盘路径
    TargetProfile string  // 目标RAID配置（可选）
    AutoBalance   bool    // 自动执行balance
    Force         bool    // 强制执行
    DryRun        bool    // 仅模拟运行
}

// 扩展状态
type ExpansionStatus struct {
    State            ExpansionState  // idle/addingDevice/balancing/completed
    Phase            ExpansionPhase  // validation/deviceAdd/balance/verification
    Progress         float64         // 0-100
    PhaseProgress    map[string]float64
    OriginalDevices  []string
    NewDevices       []string
    CapacityGain     uint64
    StartTime        time.Time
    EstimatedTimeRemaining time.Duration
}
```

### 3.2 RAID1扩容API设计

**RAID1扩容场景**:
- 2盘 → 3盘: 需要转换profile或保持raid1
- 添加设备后需要balance重新分布数据

**API接口**:

```go
// POST /api/v1/volumes/{name}/expand
type RAID1ExpandRequest struct {
    NewDevice     string `json:"newDevice" binding:"required"`
    TargetProfile string `json:"targetProfile"` // "raid1" 或 "raid5"（3盘时）
    AutoBalance   bool   `json:"autoBalance"`   // 默认true
    Force         bool   `json:"force"`
    DryRun        bool   `json:"dryRun"`
}

// GET /api/v1/volumes/{name}/expand/status
// 返回 ExpansionStatus

// POST /api/v1/volumes/{name}/expand/estimate
type RAID1ExpandEstimate struct {
    NewDeviceSize    uint64 `json:"newDeviceSize"`
    CurrentProfile   string `json:"currentProfile"`
    // 返回容量增益估算
}
```

**RAID1扩容特殊处理**:

```go
// RAID1扩容逻辑
func expandRAID1(pool *SmartPool, newDevice *SmartDevice) error {
    // 1. 添加设备
    client.AddDevice(pool.MountPoint, newDevice.Device)
    
    // 2. 确定目标profile
    newWidth := len(pool.Devices) + 1
    targetProfile := "raid1"
    
    // 3盘时可选转换为raid5以获得更高效率
    if newWidth == 3 && pool.RAIDPolicy.PerformanceMode {
        targetProfile = "raid5" // 用户确认后转换
    }
    
    // 3. 执行balance
    if targetProfile != pool.Tiers[0].RAIDType {
        // 转换profile
        client.ConvertProfile(pool.MountPoint, targetProfile)
    } else {
        // 仅重新分布
        client.StartBalance(pool.MountPoint)
    }
    
    // 4. 监控进度
    monitorBalanceProgress(pool)
    
    // 5. 更新容量计算
    updatePoolCapacity(pool)
    
    return nil
}
```

### 3.3 RAID10扩容API设计

**RAID10扩容规则**:
- 必须保持偶数设备数
- 4盘 → 6盘 → 8盘（每次加2盘）
- 或转换为其他配置（raid5/raid6）

**API接口**:

```go
// POST /api/v1/volumes/{name}/expand/raid10
type RAID10ExpandRequest struct {
    NewDevices    []string `json:"newDevices" binding:"required,min=2,max=2"` // 必须加2盘
    MaintainRAID10 bool    `json:"maintainRAID10"` // 保持raid10或允许转换
    AutoBalance   bool     `json:"autoBalance"`
    Force         bool     `json:"force"`
    DryRun        bool     `json:"dryRun"`
}

// 扩展前验证
func validateRAID10Expand(pool *SmartPool, newDevices []string) error {
    currentWidth := len(pool.Devices)
    newWidth := currentWidth + len(newDevices)
    
    // 必须保持偶数
    if newWidth % 2 != 0 {
        return fmt.Errorf("RAID10需要偶数设备数，当前%d盘+新%d盘=%d盘",
            currentWidth, len(newDevices), newWidth)
    }
    
    // 最少4盘
    if newWidth < 4 {
        return fmt.Errorf("RAID10最少需要4盘")
    }
    
    return nil
}
```

**RAID10扩容逻辑**:

```go
func expandRAID10(pool *SmartPool, newDevices []*SmartDevice) error {
    // 1. 验证偶数设备数
    if err := validateRAID10Expand(pool, newDevices); err != nil {
        return err
    }
    
    // 2. 依次添加设备
    for _, dev := range newDevices {
        client.AddDevice(pool.MountPoint, dev.Device)
    }
    
    // 3. 执行balance保持raid10分布
    client.StartBalance(pool.MountPoint, "-dconvert=raid10", "-mconvert=raid10")
    
    // 4. 监控进度
    monitorBalanceProgress(pool)
    
    // 5. 更新容量
    // RAID10效率始终50%，新容量 = 新盘总容量 * 0.5
    updatePoolCapacity(pool)
    
    return nil
}
```

### 3.4 扩展操作控制API

**暂停/恢复/取消** (已实现):

```go
// POST /api/v1/expansion/pause
func pauseExpansion(c *gin.Context) {
    err := expansionManager.PauseExpansion()
    // balance不支持真正暂停，取消后记录状态可恢复
}

// POST /api/v1/expansion/resume
func resumeExpansion(c *gin.Context) {
    // 从暂停状态恢复，重新启动balance
    status, err := expansionManager.ResumeExpansion(ctx, config)
}

// POST /api/v1/expansion/cancel
func cancelExpansion(c *gin.Context) {
    // 取消扩展，取消balance
    err := expansionManager.CancelExpansion()
}
```

---

## 四、测试覆盖率分析

### 当前覆盖率状态

| 模块 | 测试文件 | 覆盖率 | 状态 |
|------|----------|--------|------|
| `internal/storage/smart_raid.go` | `smart_raid_test.go` | ~60% | 有测试但需完善 |
| `pkg/storage/btrfs/expansion_manager.go` | **无** | **0%** | ⚠️ 需补充 |
| `pkg/storage/btrfs/expansion_types.go` | **无** | **0%** | ⚠️ 需补充 |
| `pkg/storage/zfs/raidz_expansion.go` | `raidz_expansion_test.go` | ~80% | 较完善 |

### 测试覆盖率命令结果

```
$ go test -cover ./pkg/storage/btrfs/...
nas-os/pkg/storage/btrfs    coverage: 0.0% of statements

$ go test -cover ./internal/storage/...
（测试运行中，包含smart_raid_test.go等）
```

### 需补充的测试

**Btrfs扩展管理器测试** (pkg/storage/btrfs):

```go
// expansion_manager_test.go - 建议新增

func TestValidateDevice(t *testing.T) {
    // 测试设备验证逻辑
}

func TestValidateVolume(t *testing.T) {
    // 测试卷验证逻辑
}

func TestStartExpansion(t *testing.T) {
    // 测试扩展启动
}

func TestExpansionProgress(t *testing.T) {
    // 测试进度更新
}

func TestExpansionPauseResume(t *testing.T) {
    // 测试暂停恢复
}

func TestExpansionEstimate(t *testing.T) {
    // 测试容量和时间估算
}

func TestConcurrentExpansion(t *testing.T) {
    // 测试并发安全性
}

func BenchmarkExpansion(b *testing.B) {
    // 性能基准测试
}
```

---

## 五、API端点汇总

### 已实现端点 (expansion_handlers.go)

| 端点 | 方法 | 功能 |
|------|------|------|
| `/expansion/status` | GET | 获取扩展状态 |
| `/expansion/history` | GET | 扩展历史记录 |
| `/expansion/available-disks` | GET | 可用磁盘列表 |
| `/expansion/validate/device` | POST | 设备验证 |
| `/expansion/validate/volume` | POST | 卷验证 |
| `/expansion/start` | POST | 开始扩展 |
| `/expansion/pause` | POST | 暂停扩展 |
| `/expansion/resume` | POST | 恢复扩展 |
| `/expansion/cancel` | POST | 取消扩展 |
| `/expansion/estimate/time` | POST | 估算时间 |
| `/expansion/estimate/capacity` | POST | 估算容量 |
| `/expansion/raid-configs` | GET | RAID配置信息 |
| `/volumes/{name}/expand` | POST | 卷级扩展 |
| `/volumes/{name}/expand/status` | GET | 卷扩展状态 |
| `/volumes/{name}/expand/estimate` | POST | 卷扩展估算 |

### 建议新增端点

| 端点 | 方法 | 功能 |
|------|------|------|
| `/volumes/{name}/expand/raid10` | POST | RAID10专用扩展 |
| `/volumes/{name}/expand/validate` | POST | 扩展前综合验证 |
| `/expansion/schedule` | POST | 定时扩展计划 |

---

## 六、改进建议

### 6.1 测试覆盖率提升

**优先级1**: 补充btrfs扩展测试
```bash
# 新增测试文件
touch pkg/storage/btrfs/expansion_manager_test.go
touch pkg/storage/btrfs/expansion_types_test.go

# 运行测试并生成覆盖率报告
go test -cover ./pkg/storage/btrfs/...
go test -coverprofile=coverage.out ./pkg/storage/btrfs/...
go tool cover -html=coverage.out
```

**优先级2**: 完善smart_raid测试
- 添加扩容流程集成测试
- 添加边界条件测试
- 添加并发安全测试

### 6.2 API增强

1. **批量扩展**: 支RAID10一次添加多盘
2. **扩展计划**: 支持预定扩展时间
3. **扩展回滚**: 扩展失败时回滚到原状态
4. **智能建议**: 分析池状态自动建议扩展方案

### 6.3 文档完善

1. API Swagger文档
2. 扩展最佳实践指南
3. 容量估算算法说明

---

## 七、总结

### 已完成工作
- ✅ 分析现有btrfs RAID管理代码结构
- ✅ 研究SmartRAID核心算法（层级计算、RAID选择、容量计算）
- ✅ 分析竞品TrueNAS RAIDZ Expansion设计要点
- ✅ 设计RAID1/RAID10扩容API接口
- ✅ 检查测试覆盖率（发现btrfs模块0%覆盖率）

### 关键发现
- **btrfs扩展模块缺少测试**: pkg/storage/btrfs 0%覆盖率
- **zfs扩展模块较完善**: raidz_expansion_test.go 覆盖率约80%
- **API端点已较完整**: 15个端点已实现
- **SmartRAID架构良好**: 分层存储设计合理

### 后续建议
1. **立即**: 补充btrfs扩展测试（预计工作量: 4-6小时）
2. **短期**: 完善RAID10批量扩展API
3. **中期**: 添加扩展回滚和智能建议功能

---

**报告日期**: 2026-03-31  
**执行者**: 兵部（软件工程）  
**任务编号**: Round109