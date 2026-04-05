# RAIDZ Expansion API 设计预研

## 状态
设计文档完成: `docs/storage/RAIDZ_EXPANSION_DESIGN.md`

## 技术参考
- OpenZFS 2.2+ RAIDZ Expansion特性
- TrueNAS Electric Eel (24.10) 实现

## 核心机制
1. 单盘扩展 - 每次添加一块磁盘
2. 渐进迁移 - 无需重建，逐块迁移
3. 事务保护 - 扩展状态持久化

## API设计要点
```go
// 扩展请求
type ExpansionRequest struct {
    PoolName    string    // ZFS池名
    NewDiskPath string    // 新磁盘路径
    Force       bool      // 强制执行（跳过健康检查）
}

// 扩展进度
type ExpansionProgress struct {
    Status      string    // running/paused/completed
    Percentage  float64   // 完成百分比
    ETA         time.Duration // 预估剩余时间
    StartTime   time.Time
}
```

## 实现计划
| 阶段 | 任务 | 预期日期 |
|------|------|----------|
| M1 | API接口定义 | 04-05 |
| M2 | 核心扩容逻辑 | 04-10 |
| M3 | WebUI进度监控 | 04-15 |
| M4 | 测试与发布 | 04-20 |

## 安全考虑
- 扩展前健康检查（SMART状态）
- 扩展期间监控告警
- 回滚机制设计

## 依赖
- OpenZFS 2.2+内核模块
- `zpool add`扩容命令封装