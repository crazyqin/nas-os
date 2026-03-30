# RAIDZ Expansion 技术研究报告

> **版本**: v1.0.0  
> **日期**: 2026-03-30  
> **作者**: 兵部（软件工程与系统架构）

---

## 1. 概述

RAIDZ Expansion 是 OpenZFS 2.3 引入的革命性特性，允许用户向现有 RAIDZ VDEV 逐个添加磁盘，实现存储池的渐进式扩容。此特性由 iXsystems 赞助开发，并在 TrueNAS 24.10 (Electric Eel) 中首次集成 UI 支持。

### 1.1 解决的核心问题

传统 ZFS 扩容方式存在两大局限：
1. **整组扩容**：添加新 VDEV 需要成倍增加磁盘（如 RAIDZ2 需加一组 6 盘）
2. **空间浪费**：小规模部署难以有效利用增量扩容

RAIDZ Expansion 允许单盘扩容，大幅降低扩容门槛：
- 原：5 盘 RAIDZ2 → 扩容需再加一组 5-6 盘（10-11 盘总量）
- 新：5 盘 RAIDZ2 → 单盘扩容至 6 盘（增量成本降低 80%+）

---

## 2. 技术原理

### 2.1 核心机制

```
┌─────────────────────────────────────────────────────────────┐
│                   RAIDZ Expansion 流程                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  原始状态              扩展中              完成状态           │
│  ┌─┐ ┌─┐ ┌─┐ ┌─┐     ┌─┐ ┌─┐ ┌─┐ ┌─┐     ┌─┐ ┌─┐ ┌─┐ ┌─┐ ┌─┐│
│  │D│ │D│ │P│ │P│     │D│ │D│ │P│ │P│     │D│ │D│ │D│ │P│ │P││
│  └─┘ └─┘ └─┘ └─┘     └─┘ └─┘ └─┘ └─┘     └─┘ └─┘ └─┘ └─┘ └─┘│
│       4盘 RAIDZ2    +   新盘(迁移中)      5盘 RAIDZ2         │
│                                                             │
│  数据块需重新分布 → 新盘逐步接收数据 → 容量增加             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 关键技术点

| 要素 | 说明 |
|------|------|
| **触发命令** | `zpool attach POOL raidzP-N NEW_DEVICE` |
| **特性标志** | `feature@raidz_expansion` 必须启用 |
| **数据迁移** | 读取已分配空间 → 重写到新配置（含新盘） |
| **进度监控** | `zpool status` 显示扩展进度百分比 |
| **中断恢复** | 支持重启/export/import 后继续 |

### 2.3 数据重分布算法

```
原始布局 (5盘 RAIDZ2):
┌─────────────────────────────────────┐
│ Block A: D1 D2 P1 P2 (宽=4)         │
│ Block B: D3 D4 P1 P2 (宽=4)         │
│ 奇偶比: 3数据 : 2校验               │
└─────────────────────────────────────┘

扩展后布局 (6盘 RAIDZ2):
┌─────────────────────────────────────┐
│ Block A: D1 D2 D3 P1 P2 (宽=5)      │ ← 新块
│ Block B (旧): 分布到 6 盘           │ ← 旧块保持原奇偶比
│ 新块奇偶比: 4数据 : 2校验           │
└─────────────────────────────────────┘
```

**重要发现**：
- 旧数据块保持原有奇偶比，分布到更多磁盘
- 新数据块采用新的奇偶比
- 存在"容量折损"（headroom loss）现象

---

## 3. 容量分析

### 3.1 容量计算公式

```
理论容量 = (N - P) × DiskSize

其中:
- N = 磁盘总数
- P = 奇偶校验盘数 (RAIDZ1=1, RAIDZ2=2, RAIDZ3=3)

扩展后实际可用容量:
- 新写入数据: 使用新奇偶比 (N_new - P)
- 旧数据区块: 按旧奇偶比计算 (N_old - P)
- 报告容量: 取保守值（旧奇偶比）
```

### 3.2 容量折损示例

| 配置 | 理论容量 | 报告容量 | 折损率 |
|------|----------|----------|--------|
| 4盘 RAIDZ2 → 5盘 | 3×D | 2×D (旧) | ~33% |
| 5盘 RAIDZ2 → 6盘 | 4×D | 3×D (旧) | ~25% |
| 6盘 RAIDZ2 → 7盘 | 5×D | 4×D (旧) | ~20% |

**恢复方法**：
1. 自然恢复：随数据修改/删除逐步释放
2. 主动恢复：复制重写数据到扩展池

### 3.3 RAIDZ Expansion Calculator

TrueNAS 提供官方计算器：
- https://www.truenas.com/docs/references/extensioncalculator/
- 可预估容量折损和恢复潜力

---

## 4. 可用性与容错

### 4.1 扩展期间特性

| 特性 | 状态 |
|------|------|
| **池可访问** | ✅ 全程可读写 |
| **数据冗余** | ✅ 保持原有容错能力 |
| **磁盘故障处理** | 扩展暂停 → 修复 → 继续 |
| **重启恢复** | ✅ 自动从断点继续 |
| **取消支持** | ❌ 不可逆操作 |

### 4.2 容错能力不变性

```
扩展前后容错能力对比:

4盘 RAIDZ2 (容错 2 盘) → 5盘 RAIDZ2 (容错 2 盘)
✅ 容错级别不变
⚠️ 但磁盘总数增加，故障概率略有上升
```

### 4.3 多次扩展支持

- RAIDZ VDEV 可多次扩展
- 每次扩展都会进一步改变奇偶比
- 建议：扩展后尽量让数据自然迁移

---

## 5. 系统要求

### 5.1 OpenZFS 版本

| 版本 | RAIDZ Expansion 支持 |
|------|----------------------|
| OpenZFS 2.2.x | ✅ 实验性支持 |
| OpenZFS 2.3+ | ✅ 正式支持 |
| TrueNAS 24.10 | ✅ UI集成 |

### 5.2 特性标志

```bash
# 检查特性是否启用
zpool get feature@raidz_expansion POOL

# 启用特性（池升级）
zpool upgrade POOL

# 特性状态:
# - disabled: 未启用
# - enabled: 可扩展
# - active: 已扩展（不可回退）
```

### 5.3 前置条件

1. 池健康状态良好
2. 目标磁盘未被使用
3. 特性标志已启用
4. 单 RAIDZ VDEV（不支持多 VDEV 池扩展）

---

## 6. 命令参考

### 6.1 基本操作

```bash
# 查看池状态（含扩展进度）
zpool status -v POOL

# 执行扩展
zpool attach POOL raidz2-0 /dev/sdX

# 估算扩展时间（基于 200MB/s 假设）
# 实际取决于数据量和硬件性能
```

### 6.2 监控扩展进度

```bash
# zpool status 输出示例
pool: tank
state: ONLINE
expand: raidz expansion in progress, 45.2% complete

config:
    NAME        STATE     READ WRITE CKSUM
    tank        ONLINE       0     0     0
      raidz2-0  ONLINE       0     0     0
        sda     ONLINE       0     0     0
        sdb     ONLINE       0     0     0
        sdc     ONLINE       0     0     0
        sdd     ONLINE       0     0     0  (expanding)
```

### 6.3 故障处理

```bash
# 扩展中磁盘故障
# 1. 扩展自动暂停
# 2. 替换故障盘
zpool replace tank sdd sdf

# 3. 等待 resilver 完成
# 4. 扩展自动继续
```

---

## 7. 性能影响

### 7.1 扩展期间性能

| 指标 | 影响 |
|------|------|
| 读取性能 | 轻微下降 (~5-10%) |
| 写入性能 | 中等下降 (~10-20%) |
| 扩展速度 | 100-500 MB/s（取决于硬件） |
| CPU 使用 | 增加计算开销 |

### 7.2 扩展后性能

- **新数据块**：更高数据/奇偶比 → 更高空间效率
- **旧数据块**：原有奇偶比 → 分布更广
- **混合读取**：需要识别块类型

---

## 8. 最佳实践

### 8.1 执行时机

- ✅ 池负载较低时
- ✅ 有充足时间窗口
- ✅ 备份已完成
- ❌ 避免高负载时段

### 8.2 硬件准备

1. 确保新盘与现有盘容量匹配（或更大）
2. 建议使用相同型号磁盘
3. 预留备用盘以防故障

### 8.3 数据管理

```bash
# 扩展后主动恢复容量（可选）
# 方法：复制数据到新位置，删除旧数据

zfs send tank/data@snap | zfs recv tank/data_new
zfs destroy tank/data
zfs rename tank/data_new tank/data
```

---

## 9. 与其他扩容方式对比

| 方式 | 磁盘需求 | 容量增长 | 复杂度 |
|------|----------|----------|--------|
| RAIDZ Expansion | 单盘 | 线性增长 | 低 |
| 添加新 VDEV | 整组 | 成倍增长 | 低 |
| 更换大容量盘 | 全换 | 增量替换 | 高 |

### 选择建议

```
┌────────────────────────────────────────────────────┐
│             扩容方案决策树                          │
├────────────────────────────────────────────────────┤
│                                                    │
│  需要扩容?                                         │
│      │                                             │
│      ├─ 有空闲盘位?                                │
│      │     ├─ YES → RAIDZ Expansion (推荐)        │
│      │     │       单盘增量，成本最优              │
│      │     │                                       │
│      │     └─ NO → 评估方案                        │
│      │           ├─ 有额外盘位 → 加新 VDEV         │
│      │           ├─ 无盘位 → 更换大容量盘          │
│      │           └─ 预算有限 → RAIDZ Expansion +   │
│      │                        扩展机箱              │
│      │                                             │
│      └─ 磁盘接近寿命极限?                          │
│            ├─ YES → 更换大容量盘                   │
│            └─ NO → 继续评估                        │
│                                                    │
└────────────────────────────────────────────────────┘
```

---

## 10. nas-os 实现建议

### 10.1 API 架构设计要点

1. **异步执行模型**：扩展为长时间操作，需后台任务机制
2. **进度追踪**：实时百分比、速度、ETA
3. **状态管理**：idle/preparing/running/paused/completed/failed
4. **错误处理**：磁盘故障自动暂停、恢复机制

### 10.2 UI 设计要点

参考 TrueNAS 24.10 实现：

```
┌─────────────────────────────────────────────────────┐
│  Storage Dashboard > Pool > Manage Devices          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  VDEV: raidz2-0                                     │
│  ┌───┐ ┌───┐ ┌───┐ ┌───┐                           │
│  │sda│ │sdb│ │sdc│ │sdd│                           │
│  └───┘ └───┘ └───┘ └───┘                           │
│                                                     │
│  [Extend VDEV]  ← 按钮                              │
│                                                     │
│  ┌─ Extend VDEV Dialog ────────────────────────┐   │
│  │                                             │   │
│  │  Select new disk:                           │   │
│  │  ┌─────────────────────────────────────┐   │   │
│  │  │ /dev/sde (1TB, available)          ▼│   │   │
│  │  └─────────────────────────────────────┘   │   │
│  │                                             │   │
│  │  Estimated capacity gain: +200 GB          │   │
│  │  Estimated time: 3-5 hours                 │   │
│  │                                             │   │
│  │  ⚠️ Warning: Cannot undo expansion          │   │
│  │                                             │   │
│  │        [Cancel]  [Extend]                  │   │
│  │                                             │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  Expansion Progress (during operation):            │
│  ┌─────────────────────────────────────────────┐   │
│  │ ████████████████████░░░░░░░░░░░░░░░░░░░░░░░ │   │
│  │                                              │   │
│  │  45.2% complete                              │   │
│  │  Speed: 245 MB/s                             │   │
│  │  ETA: 2h 15m                                 │   │
│  │                                              │   │
│  │  [Pause]  [Cancel]                           │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 10.3 安全考虑

1. **不可逆操作警告**：UI 必须明确提示
2. **确认机制**：输入池名称确认
3. **健康检查前置**：扩展前检查池状态
4. **容量折损提示**：告知用户可能的容量折损

---

## 11. 参考资源

### 11.1 官方文档

- [TrueNAS 24.10 Pool Management](https://www.truenas.com/docs/scale/24.10/scaletutorials/storage/managepoolsscale/)
- [OpenZFS RAIDZ Expansion PR #15022](https://github.com/openzfs/zfs/pull/15022)
- [RAIDZ Extension Calculator](https://www.truenas.com/docs/references/extensioncalculator/)

### 11.2 技术文章

- [Jim Salter - RAIDZ Expansion lands in OpenZFS](https://arstechnica.com/gadgets/2021/06/raidz-expansion-code-lands-in-openzfs-master/)
- [Louwrentius - ZFS RAIDZ Expansion Caveat](https://louwrentius.com/zfs-raidz-expansion-is-awesome-but-has-a-small-caveat.html)

### 11.3 nas-os 现有实现

- `pkg/storage/zfs/raidz_expansion.go` - 已有基础实现
- `pkg/storage/zfs/interfaces.go` - ZFS 接口定义
- `internal/storage/handlers.go` - 存储 API 处理器

---

## 12. 总结

RAIDZ Expansion 是 OpenZFS 的里程碑特性，解决了 ZFS 长期以来的扩容痛点：

**优势**：
- ✅ 单盘增量扩容，成本最优
- ✅ 全程在线，无服务中断
- ✅ 支持中断恢复
- ✅ 保持原有容错级别

**注意事项**：
- ⚠️ 不可逆操作
- ⚠️ 存在容量折损（可恢复）
- ⚠️ 仅支持单 RAIDZ VDEV 池
- ⚠️ 扩展后无法降级 ZFS 版本

**nas-os 实现建议**：
- 优先复用现有 `raidz_expansion.go` 代码
- API 设计遵循异步任务模式
- UI 参考 TrueNAS 24.10 实现
- 提供容量计算器和预估工具

---

**报告完成 | 兵部 | 2026-03-30**