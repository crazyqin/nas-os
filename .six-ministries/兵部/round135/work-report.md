# 兵部工作报告 - 第135轮

**提交时间**: 2026-04-01 12:00
**任务**: RAIDZ扩展API设计

## 研究进展

### TrueNAS RAIDZ Expansion原理
- 基于OpenZFS 2.3新特性
- 支持单盘扩容，无需重建RAIDZ
- 空间利用率从66%提升至更高
- 扩展过程异步执行，支持进度监控

### API设计方案

```go
// RAIDZ扩展API接口
type RAIDZExpansionAPI struct {
    // 扩展前健康检查
    HealthCheck(poolName string) (*HealthStatus, error)
    
    // 启动扩展任务
    StartExpansion(poolName, newDisk string) (*ExpansionTask, error)
    
    // 查询扩展进度
    GetProgress(taskID string) (*ExpansionProgress, error)
    
    // 扩展任务管理
    CancelExpansion(taskID string) error
    PauseExpansion(taskID string) error
    ResumeExpansion(taskID string) error
}

// 扩展进度状态
type ExpansionProgress struct {
    TaskID     string
    Status     TaskStatus  // Running/Paused/Completed/Failed
    Progress   float64     // 0-100%
    EstimatedTime int      // 预估剩余秒数
    CurrentPhase string    // 当前阶段
}
```

## 技术挑战
- nas-os使用btrfs而非ZFS
- btrfs不支持类似RAIDZ扩展
- 需要封装外部ZFS工具或考虑btrfs替代方案

## 方案建议
| 方案 | 可行性 | 工作量 | 风险 |
|------|:------:|:------:|:----:|
| 封装OpenZFS命令 | 中 | 高 | 中 |
| btrfs扩容优化 | 高 | 中 | 低 |
| 混合存储池 | 低 | 高 | 高 |

**建议**: 优先实现btrfs扩容优化，后续研究OpenZFS迁移

## 下一步
- [ ] API接口详细设计文档
- [ ] btrfs扩容可行性验证
- [ ] OpenZFS集成预研

**状态**: 🟡 设计阶段