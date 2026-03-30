# 存储分层管理器 (Storage Tiering Manager)

## 概述

存储分层管理器实现智能数据迁移，自动将热数据提升到SSD缓存层，冷数据降级到HDD存储层。参考竞品实现：

- **TrueNAS Electric Eel**: Docker应用架构
- **群晖DSM 7.3**: 分层存储自动化
- **飞牛fnOS**: 内网穿透服务

## 架构

```
internal/storage/tiering/
├── manager.go       # 主管理器
├── manager_test.go  # 单元测试
└── README.md        # 文档
```

## 核心功能

### 1. 存储层管理

```go
// 创建存储层
tier := TierConfig{
    Type:        TierTypeSSD,
    Name:        "SSD缓存层",
    Path:        "/mnt/ssd",
    Capacity:    1 << 30, // 1GB
    Priority:    100,
    Enabled:     true,
    AutoPromote: true,
    AutoDemote:  true,
    Threshold:   80,
}
err := manager.CreateTier(tier)

// 列出存储层
tiers := manager.ListTiers()

// 获取存储层统计
stats, _ := manager.GetTierStats(TierTypeSSD)
```

### 2. 策略配置

```go
// 创建分层策略
policy := PolicyConfig{
    ID:             "hot-to-ssd",
    Name:           "热数据自动提升到SSD",
    Enabled:        true,
    SourceTier:     TierTypeHDD,
    TargetTier:     TierTypeSSD,
    MinAccessCount: 100,          // 访问次数>=100为热数据
    MaxAccessAge:   24 * time.Hour, // 最近24小时
    Schedule:       "0 * * * *",  // 每小时执行
}

// 冷数据策略
coldPolicy := PolicyConfig{
    ID:            "cold-to-hdd",
    Name:          "冷数据自动降级到HDD",
    Enabled:       true,
    SourceTier:    TierTypeSSD,
    TargetTier:    TierTypeHDD,
    MaxAccessAge:  720 * time.Hour, // 30天未访问
    Schedule:      "0 3 * * *",     // 每天凌晨3点
}
```

### 3. 访问追踪

```go
// 记录文件访问
manager.RecordAccess("/path/to/file", TierTypeHDD, readBytes, writeBytes)

// 获取热数据文件
hotFiles := manager.GetHotFiles(TierTypeHDD, 100)

// 获取冷数据文件
coldFiles := manager.GetColdFiles(TierTypeSSD, 100)
```

### 4. 手动迁移

```go
// 迁移热数据到SSD
task, err := manager.MigrateHotToSSD(ctx)

// 迁移冷数据到HDD
task, err := manager.MigrateColdToHDD(ctx)

// 查询任务状态
task, _ := manager.GetTask(taskID)
```

## 配置

### 默认配置

```go
config := ManagerConfig{
    CheckInterval:   1 * time.Hour,  // 检查间隔
    HotThreshold:    100,            // 热数据访问次数阈值
    WarmThreshold:   10,             // 温数据访问次数阈值
    ColdAgeHours:    720,            // 冷数据判断时长（30天）
    MaxConcurrent:   5,              // 最大并发迁移数
    EnableAutoTier:  true,           // 启用自动分层
    ConfigPath:      "/etc/nas-os/tiering.json",
}
```

### 存储层类型

| 类型 | 优先级 | 用途 |
|------|--------|------|
| `ssd` | 100 | 热数据缓存 |
| `hdd` | 50 | 冷数据存储 |
| `cloud` | 10 | 归档存储 |

### 访问频率分类

| 频率 | 条件 | 行为 |
|------|------|------|
| `hot` | 访问次数 >= 100 且 24小时内访问 | 迁移到SSD |
| `warm` | 介于热和冷之间 | 保持当前位置 |
| `cold` | 超过30天未访问 | 迁移到HDD |

## API

### 管理器生命周期

```go
// 创建管理器
manager := NewManager(config)

// 初始化
manager.Initialize()

// 启动（开始自动分层）
manager.Start()

// 停止
manager.Stop()
```

### 状态查询

```go
// 获取整体状态
status := manager.GetStatus()
// {
//   "enabled": true,
//   "runningTasks": 0,
//   "pendingTasks": 1,
//   "totalPolicies": 2,
//   "activePolicies": 2,
//   "hotFiles": 150,
//   "coldFiles": 50
// }

// 获取存储层统计
stats, _ := manager.GetTierStats(TierTypeSSD)
// {
//   "type": "ssd",
//   "name": "SSD缓存层",
//   "totalFiles": 1000,
//   "hotFiles": 150,
//   "warmFiles": 300,
//   "coldFiles": 50,
//   "hotBytes": 1073741824,
//   "warmBytes": 2147483648,
//   "coldBytes": 536870912
// }
```

## 迁移流程

### 热数据提升 (HDD → SSD)

```
1. 扫描HDD上的文件访问记录
2. 识别访问频率为"hot"的文件
3. 检查SSD可用空间
4. 复制文件到SSD
5. 更新访问记录的存储层信息
```

### 冷数据降级 (SSD → HDD)

```
1. 扫描SSD上的文件访问记录
2. 识别访问频率为"cold"的文件
3. 检查HDD可用空间
4. 移动文件到HDD（删除SSD上的副本）
5. 更新访问记录的存储层信息
```

## 性能优化

1. **异步迁移**: 所有迁移操作在后台执行
2. **并发控制**: 限制最大并发迁移数
3. **空间保护**: 保留20% SSD空间用于缓存
4. **增量扫描**: 只扫描有变化的文件

## 监控指标

- `tiering_tasks_total`: 总迁移任务数
- `tiering_files_hot`: 热数据文件数
- `tiering_files_cold`: 冷数据文件数
- `tiering_migration_bytes`: 迁移数据量
- `tiering_policy_runs`: 策略执行次数

## 最佳实践

1. **SSD容量**: 建议占总存储的10-20%
2. **检查间隔**: 建议设置为1小时
3. **热数据阈值**: 根据实际访问模式调整
4. **冷数据时长**: 建议30天以上

## 与竞品对比

| 功能 | TrueNAS | 群晖 | NAS-OS |
|------|---------|------|--------|
| 自动分层 | ✅ | ✅ | ✅ |
| SSD缓存 | ✅ | ✅ | ✅ |
| 策略配置 | 图形化 | 图形化 | API+配置文件 |
| 调度方式 | 自动 | 自动 | 自动+手动 |
| 云归档 | ✅ | ✅ | 规划中 |

## 扩展性

### 添加新的存储层

```go
manager.CreateTier(TierConfig{
    Type:     TierType("nvme"),
    Name:     "NVMe缓存层",
    Path:     "/mnt/nvme",
    Priority: 150,
    Enabled:  true,
})
```

### 自定义策略

```go
manager.CreatePolicy(PolicyConfig{
    ID:             "large-files-to-hdd",
    Name:           "大文件迁移到HDD",
    SourceTier:     TierTypeSSD,
    TargetTier:     TierTypeHDD,
    MinFileSize:    1 << 30, // 1GB以上
    MaxAccessAge:   168 * time.Hour, // 7天未访问
    Schedule:       "0 4 * * *", // 每天凌晨4点
})
```

## 故障恢复

- 迁移任务失败自动重试
- 保留源文件直到迁移成功
- 支持手动触发迁移
- 配置自动持久化

## 未来计划

- [ ] 云存储归档支持
- [ ] 机器学习预测访问模式
- [ ] 多节点协同分层
- [ ] 图形化策略配置界面