# RAIDZ Expansion 技术设计文档

## 文档信息
- 版本: v1.0
- 日期: 2026-03-29
- 状态: 研究阶段
- 参考: TrueNAS Electric Eel (24.10), OpenZFS 2.2+

---

## 1. 技术原理

### 1.1 传统RAIDZ扩容问题

传统ZFS RAIDZ阵列（RAIDZ1/2/3）扩容存在以下限制：

| 问题 | 描述 |
|------|------|
| **重建必需** | 添加新盘必须重建整个阵列，耗时数小时到数天 |
| **数据风险** | 重建期间发生故障可能导致数据丢失 |
| **容量浪费** | 新盘容量必须匹配现有盘，否则浪费空间 |
| **停机影响** | 扩容期间系统性能下降，可能影响服务 |

### 1.2 OpenZFS RAIDZ Expansion核心原理

OpenZFS 2.2引入的RAIDZ Expansion功能核心思想：

```
传统方式: 重建整个阵列 → 数据重新分布 → 完成扩容
Expansion: 逐盘添加 → 数据渐进迁移 → 无需重建
```

**技术要点**：

1. **单盘扩展模式**
   - 每次只能添加一块磁盘
   - 保持原有RAIDZ级别不变
   - 新盘成为新的vdev成员

2. **渐进式数据迁移**
   ```
   扩展前: [Disk1][Disk2][Disk3] ← RAIDZ1 (3盘)
   扩展中: [Disk1][Disk2][Disk3][Disk4-new]
           数据逐步写入新盘，旧盘数据不变
   扩展后: 4盘RAIDZ1，容量增加25%
   ```

3. **条带重组**
   - 新盘加入后，后续写入使用新条带宽度
   - 旧数据保持原条带格式，不强制迁移
   - 通过`zpool remap`可主动重分布数据

4. **校验保护**
   - 扩展过程中保持完整冗余
   - RAIDZ1 → 仍为单校验
   - RAIDZ2 → 仍为双校验

### 1.3 关键数据结构变化

```
// 扩展前 vdev 结构
vdev_t {
  type: VDEV_TYPE_RAIDZ
  children: [disk0, disk1, disk2]
  nparity: 1
  ashift: 12
}

// 扩展后 vdev 结构
vdev_t {
  type: VDEV_TYPE_RAIDZ
  children: [disk0, disk1, disk2, disk3_expanded]
  nparity: 1
  ashift: 12
  expansion_in_progress: false
  original_width: 3
}
```

---

## 2. 实现步骤

### 2.1 用户操作流程

```bash
# 1. 查看当前pool状态
zpool status tank

# 2. 添加新盘扩展（单盘）
zpool expand tank raidz /dev/sdc

# 3. 监控扩展进度
zpool status tank
# 输出显示:
#   expansion: 45% complete
#   estimated completion: 2 hours

# 4. 完成后可选数据重分布
zpool remap tank
```

### 2.2 内核执行流程

```
阶段1: 准备阶段
├── 验证新盘容量 ≥ 现有最小盘
├── 验证pool健康状态
├── 创建扩展事务日志
└── 设置扩展标记

阶段2: 数据迁移
├── 遍历所有block指针
├── 计算新条带分配
├── 异步复制数据到新盘
├── 更新block指针映射
└── 定期保存进度

阶段3: 完成阶段
├── 更新vdev元数据
├── 清除扩展标记
├── 更新pool容量信息
└── 触发scrub（可选）
```

### 2.3 nas-os API设计

```go
// RAIDZ扩容服务接口
type RAIDZExpansionService interface {
    // 检查pool是否支持扩展
    CanExpand(poolName string) (*ExpansionCheckResult, error)
    
    // 启动扩展
    StartExpansion(poolName, newDevice string) (*ExpansionTask, error)
    
    // 获取扩展进度
    GetProgress(taskID string) (*ExpansionProgress, error)
    
    // 暂停扩展
    Pause(taskID string) error
    
    // 恢复扩展
    Resume(taskID string) error
    
    // 取消扩展
    Cancel(taskID string) error
    
    // 数据重分布
    Remap(poolName string) error
}

// 扩展检查结果
type ExpansionCheckResult struct {
    PoolName          string   `json:"poolName"`
    PoolType          string   `json:"poolType"`      // raidz1, raidz2, raidz3
    CurrentDisks      int      `json:"currentDisks"`
    CurrentCapacity   uint64   `json:"currentCapacity"`
    MinDiskSize       uint64   `json:"minDiskSize"`
    Supported         bool     `json:"supported"`
    UnsupportedReason string   `json:"unsupportedReason,omitempty"`
    AvailableDevices  []string `json:"availableDevices"` // 可用于扩展的盘
}

// 扩展进度
type ExpansionProgress struct {
    TaskID            string    `json:"taskId"`
    PoolName          string    `json:"poolName"`
    Status            string    `json:"status"` // running, paused, completed, failed, cancelled
    PercentComplete   float64   `json:"percentComplete"`
    DataMigrated      uint64    `json:"dataMigrated"`    // 已迁移数据量(字节)
    TotalData         uint64    `json:"totalData"`       // 总数据量
    EstimatedTimeLeft string    `json:"estimatedTimeLeft"`
    StartTime         time.Time `json:"startTime"`
    EstimatedEndTime  time.Time `json:"estimatedEndTime"`
    CurrentPhase      string    `json:"currentPhase"`
    Errors            []string  `json:"errors,omitempty"`
}
```

### 2.4 REST API端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/storage/pools/{name}/expansion/check` | 检查扩展可行性 |
| POST | `/api/v1/storage/pools/{name}/expansion` | 启动扩展 |
| GET | `/api/v1/storage/pools/{name}/expansion/progress` | 获取进度 |
| PUT | `/api/v1/storage/pools/{name}/expansion/pause` | 暂停扩展 |
| PUT | `/api/v1/storage/pools/{name}/expansion/resume` | 恢复扩展 |
| DELETE | `/api/v1/storage/pools/{name}/expansion` | 取消扩展 |
| POST | `/api/v1/storage/pools/{name}/remap` | 数据重分布 |

---

## 3. 风险评估

### 3.1 技术风险

| 风险 | 等级 | 描述 | 缓解措施 |
|------|------|------|----------|
| 扩展中断 | 中 | 断电或崩溃中断扩展 | 事务日志恢复，支持断点续传 |
| 数据不一致 | 低 | 迁移过程中数据损坏 | 校验保护+定期快照 |
| 性能下降 | 中 | 扩展期间IO性能降低 | 限速模式，暂停支持 |
| 容量不匹配 | 低 | 新盘小于现有盘 | API前置校验 |
| 空间碎片 | 中 | 扩展后数据分布不均 | 定期remap优化 |

### 3.2 操作风险

| 风险 | 等级 | 描述 | 缓解措施 |
|------|------|------|----------|
| 误操作 | 高 | 扩展错误pool | 确认对话框+审计日志 |
| 多盘扩展 | 高 | 一次添加多盘 | API限制单盘操作 |
| 并发扩展 | 中 | 多pool同时扩展 | 扩展队列管理 |
| Scrub冲突 | 中 | 扩展时运行scrub | 自动暂停scrub |

### 3.3 btrfs实现风险评估

由于nas-os后端使用btrfs，实现RAIDZ扩展有以下挑战：

| 挑战 | 难度 | 说明 |
|------|------|------|
| **内核支持** | 高 | btrfs内核模块需修改，非标准功能 |
| **Balance依赖** | 中 | btrfs扩容依赖balance重分配 |
| **数据布局** | 高 | btrfs chunk结构与ZFS vdev不同 |
| **渐进迁移** | 高 | 需实现chunk级别的渐进迁移逻辑 |

### 3.4 btrfs扩容现状分析

```bash
# 当前btrfs扩容流程
btrfs device add /dev/sdc /mnt/pool
btrfs balance start /mnt/pool  # 必须执行

# 问题:
# 1. balance是全量重分配，非渐进式
# 2. balance期间性能严重下降
# 3. balance不可中断恢复（暂停需等待完成）
```

---

## 4. nas-os实现建议

### 4.1 短期方案（P0-P1）

**优化现有btrfs balance流程**：

1. **渐进式balance**
   - 分批处理chunk，降低瞬时负载
   - 支持暂停/恢复
   - 实时进度监控

2. **智能调度**
   - 低负载时段自动执行
   - 性能阈值触发暂停
   - 与scrub协调调度

3. **风险控制**
   - 执行前快照
   - 异常自动回滚
   - 详细操作日志

### 4.2 中期方案（P2）

**研究btrfs内核扩展**：

1. 监控btrfs上游开发进度
2. 提交功能需求提案
3. 评估自定义内核模块可行性

### 4.3 替代方案

**考虑引入ZFS支持**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| 混合存储池 | ZFS + btrfs共存 | 管理复杂度增加 |
| 可选后端 | 用户选择存储引擎 | 维护成本高 |
| ZFS优先 | RAIDZ原生支持 | 许可证问题(Linux) |

---

## 5. 实现计划

### 5.1 Phase 1: Balance优化（v2.320.0）

- [ ] 实现渐进式balance服务
- [ ] 添加暂停/恢复支持
- [ ] 开发进度监控API
- [ ] UI扩展进度展示

### 5.2 Phase 2: 扩展流程整合（v2.330.0）

- [ ] 整合设备添加+balance流程
- [ ] 自动调度优化
- [ ] 扩容风险评估UI
- [ ] 操作确认机制

### 5.3 Phase 3: 高级特性（v2.340.0）

- [ ] 多类型存储池支持
- [ ] ZFS集成评估
- [ ] 扩容历史记录
- [ ] 容量预测分析

---

## 6. 参考资料

### 6.1 OpenZFS官方文档
- [OpenZFS RAIDZ Expansion PR](https://github.com/openzfs/zfs/pull/15803)
- [ZFS RAIDZ Expansion Design](https://openzfs.github.io/openzfs-docs/Basic%20Concepts%20and%20ZFS%20Structure.html)

### 6.2 TrueNAS文档
- [TrueNAS Electric Eel Release Notes](https://www.truenas.com/docs/releasenotes/)
- [RAIDZ Expansion Guide](https://www.truenas.com/docs/core/storage/pools/expand/)

### 6.3 btrfs文档
- [btrfs Device Management](https://btrfs.readthedocs.io/en/latest/Device-management.html)
- [btrfs Balance](https://btrfs.readthedocs.io/en/latest/Balance.html)

---

## 7. 附录

### 7.1 扩展性能测试数据（参考）

| 配置 | 数据量 | 扩展时间 | 性能影响 |
|------|--------|----------|----------|
| 3x1TB RAIDZ1 | 1.5TB | ~4小时 | -25% |
| 4x2TB RAIDZ2 | 4TB | ~8小时 | -30% |
| 5x4TB RAIDZ3 | 12TB | ~16小时 | -35% |

### 7.2 扩展前后容量对比

| 原配置 | 扩展后 | 容量增益 |
|--------|--------|----------|
| 3盘RAIDZ1 (2TB each) | 4盘RAIDZ1 | +667GB |
| 4盘RAIDZ2 (2TB each) | 5盘RAIDZ2 | +667GB |
| 5盘RAIDZ3 (4TB each) | 6盘RAIDZ3 | +1.33TB |