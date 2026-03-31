# RAIDZ 扩展技术设计文档

## 文档信息
- **版本**: v1.1  
- **日期**: 2026-03-31
- **状态**: 设计评审  
- **作者**: nas-os 兵部
- **竞品参考**: TrueNAS Electric Eel (24.10), OpenZFS 2.2+

---

## 1. 概述

### 1.1 背景

传统RAIDZ阵列（RAIDZ1/2/3）扩容是ZFS用户的长期痛点：
- 添加新盘需重建整个阵列
- 重建期间数据风险高
- 扩容耗时数小时至数天
- 影响系统正常服务

OpenZFS 2.2+ 引入 RAIDZ Expansion 特性，解决上述问题，实现渐进式单盘扩展。

### 1.2 设计目标

| 目标 | 描述 |
|------|------|
| **渐进扩容** | 无需重建，逐块迁移 |
| **在线操作** | 扩容期间保持服务可用 |
| **可中断恢复** | 支持暂停/恢复，断点续传 |
| **进度可视** | 实时进度监控和预估 |
| **风险可控** | 操作确认、回滚机制 |

---

## 2. 技术原理

### 2.1 传统扩容 vs RAIDZ Expansion

```
传统方式:
  [D1][D2][D3] → 重建 → [D1'][D2'][D3'][D4] (全量重建)
  
RAIDZ Expansion:
  [D1][D2][D3] → 渐进迁移 → [D1][D2][D3][D4] (逐块迁移)
  旧数据保持，新数据优先写入新盘
```

### 2.2 核心机制

**1. 单盘扩展**
- 每次仅添加一块磁盘
- RAIDZ级别不变（RAIDZ1仍为单校验）
- 新盘成为vdev新成员

**2. 渐进迁移**
```
扩展前: vdev_width = 3 (nparity = 1)
扩展中: 新数据写入使用 width=4 条带
        旧数据保持 width=3 条带格式
扩展后: 完全 width=4 (可选remap重分布)
```

**3. 事务保护**
- 扩展状态持久化
- 中断后自动恢复
- 进度定期保存

### 2.3 数据结构变化

```go
// 扩展前 vdev 元数据
VdevMetadata {
    Type:       "raidz"
    Width:      3
    Nparity:    1
    Children:   [disk0, disk1, disk2]
}

// 扩展后 vdev 元数据  
VdevMetadata {
    Type:           "raidz"
    Width:          4
    Nparity:        1
    Children:       [disk0, disk1, disk2, disk3_expanded]
    OriginalWidth:  3
    ExpansionTime:  timestamp
}
```

---

## 3. 竞品分析

### 3.1 TrueNAS Electric Eel 实现

**TrueNAS SCALE 24.10** 功能：

| 特性 | TrueNAS实现 |
|------|------------|
| UI扩展入口 | 一键扩展按钮 |
| 进度监控 | Web界面实时进度 |
| 容量预估 | 自动计算扩容后容量 |
| 操作确认 | 扩展前风险提示 |
| 回滚机制 | 扩展失败自动回滚 |

**命令行接口**:
```bash
# TrueNAS/OpenZFS 命令
zpool expand tank /dev/sdc
zpool status tank  # 查看进度
```

### 3.2 ZFS RAIDZ Expansion 限制

| 限制 | 说明 |
|------|------|
| 单盘限制 | 每次只能添加一块盘 |
| 版本要求 | OpenZFS 2.2+ |
| 容量匹配 | 新盘容量 ≥ 现有最小盘 |
| 单vdev限制 | 仅支持单RAIDZ vdev的pool |
| RAIDZ级别不变 | 不能RAIDZ1→RAIDZ2 |

### 3.3 btrfs 扩容对比

| 特性 | ZFS RAIDZ Expansion | btrfs device add |
|------|---------------------|------------------|
| 添加盘数 | 每次1个 | 多个同时 |
| 数据迁移 | 后台渐进 | balance全量重分配 |
| RAID级别转换 | 不支持 | 支持 |
| 暂停恢复 | 支持 | balance不可暂停 |
| 性能影响 | 中等 | 高（balance期间） |
| 成熟度 | 新功能（2023） | 成熟 |

---

## 4. nas-os 实现方案

### 4.1 现有代码分析

**已有实现**:

| 模块 | 文件 | 功能 |
|------|------|------|
| ZFS扩展管理器 | `pkg/storage/zfs/raidz_expansion.go` | RAIDZ扩展核心逻辑 |
| btrfs扩展管理器 | `pkg/storage/btrfs/expansion_manager.go` | btrfs设备添加+balance |
| 存储池扩展服务 | `internal/storagepool/expansion.go` | 统一扩容服务接口 |
| API处理器 | `internal/storage/expansion_handlers.go` | REST API端点 |

**代码覆盖度**: 核心框架已实现，需完善以下：

1. 实际命令执行（目前为TODO模拟）
2. 进度监控解析逻辑
3. 错误恢复机制
4. UI集成

### 4.2 技术架构

```
┌─────────────────────────────────────────────┐
│                 Web UI                       │
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐  │
│  │ 扩展入口 │  │ 进度监控 │  │ 容量预估    │  │
│  └─────────┘  └─────────┘  └─────────────┘  │
└────────────────────┬────────────────────────┘
                     │ REST API
┌────────────────────┴────────────────────────┐
│              API Layer                       │
│  /api/v1/storage/pools/{name}/expansion     │
│  - POST   start                              │
│  - GET    progress                           │
│  - PUT    pause/resume                       │
│  - DELETE cancel                             │
└────────────────────┬────────────────────────┘
                     │
┌────────────────────┴────────────────────────┐
│           Expansion Service                  │
│  ┌─────────────────┐  ┌─────────────────┐   │
│  │ ZFS Manager     │  │ btrfs Manager   │   │
│  │ raidz_expansion │  │ expansion_      │   │
│  │                 │  │ manager         │   │
│  └─────────────────┘  └─────────────────┘   │
└────────────────────┬────────────────────────┘
                     │ CLI
┌────────────────────┴────────────────────────┐
│           System Layer                       │
│  zpool expand │ btrfs device add/balance    │
└─────────────────────────────────────────────┘
```

### 4.3 接口设计

```go
// 统一扩容服务接口（已存在于 internal/storagepool/expansion.go）
type ExpansionService interface {
    Expand(poolID string, devices []string, opts ExpansionOptions) (*ExpansionTask, error)
    ConvertRAIDLevel(poolID string, targetLevel RAIDLevel) (*ExpansionTask, error)
    GetTaskProgress(taskID string) (*ExpansionTask, error)
    CancelTask(taskID string) error
    RollbackExpansion(taskID string) error
    ListTasks(poolID string) []*ExpansionTask
}

// ZFS RAIDZ 扩展接口（已存在于 pkg/storage/zfs/raidz_expansion.go）
type RAIDZExpansionManager struct {
    // 核心方法
    StartExpansion(ctx context.Context, config ExpansionConfig) (*ExpansionStatus, error)
    GetExpansionStatus() *ExpansionStatus
    PauseExpansion() error
    ResumeExpansion() error
    CancelExpansion() error
    GetPoolExpansionInfo(ctx context.Context, poolName string) (*PoolExpansionInfo, error)
    EstimateExpansionTime(ctx context.Context, poolName string) (time.Duration, error)
    CheckExpansionSupport() (bool, string)
}
```

### 4.4 REST API 端点

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/storage/pools/{name}/expansion/check` | 检查扩展可行性 |
| POST | `/api/v1/storage/pools/{name}/expansion` | 启动扩展 |
| GET | `/api/v1/storage/pools/{name}/expansion/progress` | 查询进度 |
| PUT | `/api/v1/storage/pools/{name}/expansion/pause` | 暂停扩展 |
| PUT | `/api/v1/storage/pools/{name}/expansion/resume` | 恢复扩展 |
| DELETE | `/api/v1/storage/pools/{name}/expansion` | 取消扩展 |
| POST | `/api/v1/storage/pools/{name}/remap` | 数据重分布（可选） |

---

## 5. 风险评估

### 5.1 技术风险

| 风险 | 等级 | 描述 | 缓解措施 |
|------|------|------|----------|
| **扩展中断** | 中 | 断电/崩溃中断扩展 | 事务日志持久化，支持断点续传 |
| **数据不一致** | 低 | 迁移过程中数据损坏 | 校验保护持续有效 |
| **性能下降** | 中 | 扩展期间IO性能降低 | 限速模式，支持暂停 |
| **版本不兼容** | 低 | OpenZFS版本过低 | 前置版本检查 |
| **容量不匹配** | 低 | 新盘小于现有盘 | API前置校验拒绝 |

### 5.2 操作风险

| 风险 | 等级 | 描述 | 缓解措施 |
|------|------|------|----------|
| **误操作** | 高 | 扩展错误pool | 二次确认对话框 |
| **并发扩展** | 中 | 多pool同时扩展 | 扩展队列管理 |
| **Scrub冲突** | 中 | 扩展时运行scrub | 自动暂停scrub |
| **空间不足** | 低 | 扩展中途空间耗尽 | 前置空间检查 |

### 5.3 btrfs特定风险

| 风险 | 等级 | 描述 | 缺解措施 |
|------|------|------|----------|
| **Balance性能** | 高 | balance期间性能严重下降 | 渐进式balance，分批处理 |
| **不可中断** | 中 | balance只能cancel不能pause | 实现分阶段balance |
| **数据布局** | 中 | chunk分布不均 | 后台优化任务 |

---

## 6. 工作量估算

### 6.1 Phase 1: 核心完善（预计 5 人日）

| 任务 | 工时 | 说明 |
|------|------|------|
| ZFS命令执行实现 | 2d | 实现实际zpool expand调用 |
| 进度解析完善 | 1d | 解析zpool status输出 |
| 错误恢复机制 | 1d | 断点续传，状态恢复 |
| 单元测试补充 | 1d | 覆盖核心场景 |

### 6.2 Phase 2: btrfs优化（预计 4 人日）

| 任务 | 工时 | 说明 |
|------|------|------|
| 渐进式balance | 2d | 分批处理chunk |
| 暂停恢复支持 | 1d | balance状态管理 |
| 性能监控 | 1d | 实时性能指标 |

### 6.3 Phase 3: UI集成（预计 3 人日）

| 任务 | 工时 | 说明 |
|------|------|------|
| 扩展入口页面 | 1d | 扩展操作UI |
| 进度监控页面 | 1d | 实时进度展示 |
| 容量预估展示 | 1d | 扩容容量计算UI |

### 6.4 总计：12 人日

---

## 7. 实现计划

### 7.1 v2.336.0（Phase 1）

- [ ] 完善ZFS RAIDZ扩展命令执行
- [ ] 实现进度监控解析
- [ ] 添加错误恢复机制
- [ ] 补充单元测试

### 7.2 v2.340.0（Phase 2）

- [ ] 渐进式btrfs balance
- [ ] 暂停/恢复支持
- [ ] 性能监控指标

### 7.3 v2.344.0（Phase 3）

- [ ] Web UI扩展入口
- [ ] 进度监控页面
- [ ] 容量预估展示

---

## 8. 验收标准

### 8.1 功能验收

| 场景 | 验收标准 |
|------|----------|
| 启动扩展 | 正确调用zpool expand，返回任务ID |
| 进度查询 | 实时返回百分比、预估时间 |
| 暂停恢复 | 成功暂停后可恢复继续 |
| 取消扩展 | 成功取消，设备可移除 |
| 版本检查 | 低版本ZFS拒绝扩展并提示 |
| 容量检查 | 新盘过小拒绝扩展并提示 |

### 8.2 编译验收

```bash
# 确保编译通过
cd nas-os && go build ./...
go test ./pkg/storage/zfs/... ./internal/storagepool/...
```

---

## 9. 参考资料

- [OpenZFS RAIDZ Expansion PR #15803](https://github.com/openzfs/zfs/pull/15803)
- [TrueNAS Electric Eel Release Notes](https://www.truenas.com/docs/releasenotes/)
- [btrfs Device Management](https://btrfs.readthedocs.io/en/latest/Device-management.html)
- [nas-os 现有代码](../pkg/storage/zfs/raidz_expansion.go)

---

**兵部出品 | v2.335.0 第110轮六部协同**