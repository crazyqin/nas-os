# RAIDZ扩展研究报告

## 背景
TrueNAS 25引入RAIDZ单盘扩展功能，允许在不重建阵列的情况下向现有RAIDZ阵列添加新磁盘。本文研究该技术原理及btrfs实现可能性。

## TrueNAS RAIDZ扩展原理

### ZFS RAIDZ结构
- RAIDZ类似RAID5/6，但采用变长条带
- 传统扩展需要重建整个阵列
- OpenZFS 2.2引入raidz expansion功能

### 技术核心
1. **扩展过程**：新盘加入后，逐步将数据迁移到新盘
2. **无需重建**：保持原数据完整性，增量扩容
3. **性能影响**：扩展期间性能下降约20-30%

## btrfs实现分析

### btrfs RAID结构
- btrfs使用chunk条带单元
- 支持RAID0/1/10/5/6
- 当前无原生RAID扩展功能

### btrfs扩容现状
```bash
# 当前扩容方式（需重建）
btrfs device add /dev/sdb /mnt/data
btrfs balance start /mnt/data  # 重新分配数据
```

### 可行性评估
| 方面 | ZFS RAIDZ扩展 | btrfs现状 |
|------|--------------|----------|
| 无损扩容 | ✅ 支持 | ❌ 需balance |
| 性能影响 | 适中 | balance重负载 |
| 数据安全 | 高 | 中等 |
| 实现难度 | 中 | 高 |

## nas-os实现建议

### 方案一：优化balance流程
- 实现渐进式balance
- 支持暂停/恢复
- 性能监控告警

### 方案二：新增扩容模式
- 研究btrfs内核模块扩展
- 与OpenZFS方案对比

### P2优先级
当前btrfs官方无此功能，建议作为P2长期规划，关注上游开发进展。

## 参考
- OpenZFS 2.2 RAIDZ Expansion
- btrfs wiki: Device management