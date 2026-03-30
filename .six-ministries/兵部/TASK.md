# 兵部任务：存储引擎与容器

## 目标
对标 TrueNAS 26 OpenZFS 2.4 特性，实现核心存储增强

## 任务清单
1. **LXC 容器支持** (参考 TrueNAS 26)
   - 轻量级容器运行时
   - 容器与存储卷集成
   - HA 故障转移支持

2. **OpenZFS 特性对齐**
   - Hybrid Pool 支持 (SSD+HDD 混合)
   - 块重写优化
   - 动态 gang header

3. **存储引擎优化**
   - 快照性能优化
   - 压缩算法升级
   - 数据去重增强

## 交付物
- `internal/storage/lxc/` - LXC 容器管理模块
- `internal/storage/zfs/` - OpenZFS 增强模块
- 相关 API 和测试用例

## 竞品参考
- TrueNAS 26 LXC containers
- TrueNAS OpenZFS 2.4 hybrid pool

## 负责人
兵部尚书

## 截止
本轮开发周期结束前