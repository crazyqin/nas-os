# ZFS Scrubber - 智能数据清洗模块

对标 TrueNAS 数据保护能力，提供 ZFS 存储池的数据完整性校验、自动修复和健康监控功能。

## 功能特性

### ScrubScheduler - 定时清洗调度器
- 支持每日/每周/每月清洗周期
- 可配置执行时间（小时、星期、日期）
- 自动计算下次运行时间
- 支持手动触发清洗任务

### IntegrityChecker - 数据完整性校验器
- 校验 ZFS 数据块的 checksum
- 支持多种校验和算法（SHA256、Fletcher4、SHA512）
- 详细的校验结果记录

### AutoRepair - 自动修复
- 从冗余副本恢复损坏数据
- 支持镜像和 RAID-Z 修复
- 修复动作追踪和记录
- 可配置的重试策略

### ScrubReport - 清洗报告
- 记录扫描进度和耗时
- 统计发现的错误和修复结果
- 生成维护建议
- 支持按池查询历史报告

### HealthMonitor - 存储健康监控
- 实时监测磁盘 SMART 状态
- ZFS 池健康状态评估
- 多级健康告警（good/warning/critical）
- 告警确认管理

## 目录结构

```
internal/zfsscrubber/
├── zfsscrubber.go      # 核心实现
├── errors.go           # 错误定义
├── handler.go          # HTTP API 处理器
└── zfsscrubber_test.go # 单元测试
```

## API 端点

### 调度管理
- `POST /zfs-scrubber/schedules` - 创建调度
- `GET /zfs-scrubber/schedules` - 列出调度
- `GET /zfs-scrubber/schedules/:id` - 获取调度
- `PUT /zfs-scrubber/schedules/:id` - 更新调度
- `DELETE /zfs-scrubber/schedules/:id` - 删除调度

### 清洗任务
- `POST /zfs-scrubber/scrub/:poolId` - 执行清洗
- `GET /zfs-scrubber/jobs` - 列出任务
- `GET /zfs-scrubber/jobs/:id` - 获取任务
- `POST /zfs-scrubber/jobs/:id/cancel` - 取消任务

### 清洗报告
- `GET /zfs-scrubber/reports` - 列出报告
- `GET /zfs-scrubber/reports/:id` - 获取报告

### 健康监控
- `GET /zfs-scrubber/health/pools` - 列出池健康状态
- `GET /zfs-scrubber/health/pools/:id` - 检查池健康
- `PUT /zfs-scrubber/health/pools/:id` - 更新池健康
- `GET /zfs-scrubber/health/disks/:path` - 检查磁盘 SMART

### 告警管理
- `GET /zfs-scrubber/alerts` - 列出告警
- `GET /zfs-scrubber/alerts/:id` - 获取告警
- `POST /zfs-scrubber/alerts/:id/ack` - 确认告警

### 修复动作
- `GET /zfs-scrubber/repairs` - 列出修复动作
- `GET /zfs-scrubber/repairs/:id` - 获取修复动作

### 配置
- `GET /zfs-scrubber/config` - 获取配置
- `PUT /zfs-scrubber/config` - 更新配置

### 状态
- `GET /zfs-scrubber/status` - 获取系统状态

## 使用示例

```go
// 创建清洗器
config := &ScrubberConfig{
    DefaultFrequency:   FrequencyWeekly,
    AutoRepairEnabled:  true,
    MaxConcurrentScans: 1,
}
scrubber := NewZFSScrubber(config)

// 启动清洗器
scrubber.Start()
defer scrubber.Stop()

// 创建每周一凌晨2点的清洗调度
schedule := &ScrubSchedule{
    ID:        "weekly-scrub",
    PoolID:    "tank",
    Frequency: FrequencyWeekly,
    DayOfWeek: 1,
    Hour:      2,
    Enabled:   true,
}
scrubber.CreateSchedule(schedule)

// 手动执行清洗
job, err := scrubber.ExecuteScrub("tank")
if err != nil {
    log.Fatal(err)
}

// 等待完成并获取报告
time.Sleep(5 * time.Minute)
reports := scrubber.ListReports("tank")
```

## 测试

```bash
cd nas-os
go test ./internal/zfsscrubber/... -v
```
