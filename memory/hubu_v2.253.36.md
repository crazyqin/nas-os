# 户部资源统计报告 - v2.253.36

**生成时间**: 2026-03-19 18:46
**生成部门**: 户部（财务运营）

---

## 代码资源统计

### 代码量概览

| 指标 | 数值 | 上版(v2.253.33) | 变化 |
|------|------|----------------|------|
| Go 源文件数 | 731 个 | 732 个 | -1 |
| 总代码行数 | 411,688 行 | 411,732 行 | -44 |
| go.mod 行数 | 173 行 | 172 行 | +1 |
| go.sum 行数 | 478 行 | 478 行 | 0 |
| 依赖模块数 | 215 个 | 263 个 | -48 |
| 测试文件数 | 264 个 | 264 个 | 0 |

### 测试覆盖率

**总体覆盖率: 35.9%** (上版: 37.0%, ↓1.1%)

| 模块 | 覆盖率 | 状态 |
|------|-------|------|
| internal/version | 100.0% | ⭐ 完美 |
| internal/notify | 92.9% | ⭐ 优秀 |
| internal/trash | 87.6% | ⭐ 优秀 |
| internal/database | 82.5% | ⭐ 优秀 |
| internal/transfer | 80.7% | ⭐ 优秀 |
| internal/smb | 77.7% | ✅ 良好 |
| internal/downloader | 77.7% | ✅ 良好 |
| internal/versioning | 76.6% | ✅ 良好 |
| internal/dashboard | 75.7% | ✅ 良好 |
| internal/nfs | 75.5% | ✅ 良好 |
| internal/iscsi | 73.4% | ✅ 良好 |
| internal/rbac | 69.4% | ✅ 良好 |
| internal/billing | 68.1% | ✅ 良好 |
| internal/replication | 67.5% | ✅ 良好 |
| pkg/btrfs | 61.1% | ✅ 良好 |
| internal/concurrency | 60.6% | ✅ 良好 |
| internal/prediction | 60.4% | ✅ 良好 |
| internal/monitor | 24.9% | ⚠️ 需改进 |
| internal/container | 24.0% | ⚠️ 需改进 |
| internal/budget | 15.2% | ⚠️ 需改进 |
| plugins/* | 0.0% | ❌ 无测试 |
| cmd/* | 0.0% | ❌ 无测试 |

---

## 变化分析

### 代码变化
- **文件数减少 1 个**: 代码库轻微收缩
- **代码行数减少 44 行**: 正常波动范围
- **依赖模块减少 48 个**: 清理了未使用依赖

### 覆盖率变化
- **总体下降 1.1%**: 需关注新增代码的测试覆盖
- **internal/notify 提升至 92.9%**: 持续优秀
- **internal/trash 提升至 87.6%**: 改进明显

### 需关注模块
1. **internal/monitor (24.9%)**: 监控模块覆盖率偏低
2. **internal/container (24.0%)**: 容器模块覆盖率偏低
3. **internal/budget (15.2%)**: 预算模块覆盖率严重不足

---

## 建议

### 短期（1周内）
1. 为 `internal/budget` 添加基础测试
2. 提升 `internal/monitor` 测试覆盖

### 中期（1月内）
1. 将核心模块覆盖率提升至 80% 以上
2. 为 `cmd/` 目录添加集成测试

---

**户部**