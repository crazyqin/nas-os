# RAIDZ Expansion 研究报告

**研究日期**: 2026-03-29
**对标**: TrueNAS 25.10 RAIDZ Expansion

---

## 一、TrueNAS RAIDZ Expansion 概述

### 1.1 功能介绍
TrueNAS 在 2024 年引入了 RAIDZ Expansion 功能，允许用户向现有 RAIDZ 阵列添加单个磁盘来扩展存储容量，而无需重新创建阵列。

### 1.2 技术原理
- 基于 OpenZFS 的 `zpool add` 功能增强
- 支持逐步扩展，数据会自动重新平衡
- 扩展过程中系统仍可正常使用

### 1.3 支持的 RAIDZ 级别
| RAIDZ 级别 | 扩展支持 | 备注 |
|-----------|---------|------|
| RAIDZ1 | ✅ 支持 | 单盘扩展 |
| RAIDZ2 | ✅ 支持 | 单盘扩展 |
| RAIDZ3 | ✅ 支持 | 单盘扩展 |

---

## 二、btrfs 扩展能力分析

### 2.1 当前 btrfs 扩展方式
btrfs 文件系统支持以下扩展方式：

1. **添加设备**: `btrfs device add /dev/sdX /mountpoint`
2. **调整 RAID 级别**: `btrfs balance start -dconvert=raid1 /mountpoint`
3. **重新平衡**: `btrfs balance start /mountpoint`

### 2.2 btrfs vs ZFS 对比

| 功能 | btrfs | ZFS (TrueNAS) |
|------|-------|---------------|
| 添加单盘到现有阵列 | ✅ 支持 | ✅ 支持 (RAIDZ Expansion) |
| 数据自动重新平衡 | ✅ balance | ✅ 自动 |
| 扩展时保持 RAID 级别 | ⚠️ 需手动 | ✅ 自动 |
| 混合容量硬盘 | ✅ 原生支持 | ⚠️ 受限 |
| 在线扩展 | ✅ 支持 | ✅ 支持 |

### 2.3 nas-os 实现路径

nas-os 基于 btrfs，已具备扩展能力：

```bash
# 添加新磁盘到现有存储池
btrfs device add /dev/sdX /mnt/pool

# 重新平衡数据（可选，用于均匀分布）
btrfs balance start /mnt/pool

# 调整 RAID 级别（如需要）
btrfs balance start -dconvert=raid1 -mconvert=raid1 /mnt/pool
```

### 2.4 与 TrueNAS RAIDZ Expansion 的差异

| 方面 | nas-os (btrfs) | TrueNAS RAIDZ Expansion |
|------|---------------|-------------------------|
| 易用性 | 需命令行操作 | Web UI 一键扩展 |
| 自动化 | 需手动 balance | 自动重新平衡 |
| RAID 级别保持 | 需手动指定 | 自动保持 |
| 空间利用率 | 较高（支持混合容量） | 较低（要求同容量） |

---

## 三、nas-os 改进建议

### 3.1 短期优化（已实现）
- ✅ Hot Spare 热备盘功能
- ✅ SSD 三级预警
- ✅ 存储池监控

### 3.2 中期规划（P1）
1. **存储池扩展 UI**
   - 一键添加磁盘到现有存储池
   - 自动数据重新平衡
   - 扩展进度显示

2. **智能 RAID 建议**
   - 根据磁盘数量和容量推荐 RAID 级别
   - 空间利用率预估
   - 冗余级别说明

3. **混合容量支持优化**
   - 利用 btrfs 原生混合容量支持
   - 空间利用率计算优化

### 3.3 长期规划（P2）
1. **存储池迁移**
   - 从其他 NAS 系统导入数据
   - RAID 级别转换

2. **存储池快照策略**
   - 自动快照
   - 快照空间管理

---

## 四、结论

nas-os 基于 btrfs，在存储扩展方面具有天然优势：

1. **原生支持混合容量硬盘** - 比 ZFS 更灵活
2. **在线扩展** - 无需停机
3. **数据重新平衡** - balance 功能完善

**建议**: 重点优化 Web UI 的存储管理体验，提供一键扩展功能，实现与 TrueNAS RAIDZ Expansion 相当的用户体验。

---

*研究完成: 2026-03-29*
*研究人员: 工部*