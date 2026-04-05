# 磁盘电源管理设计

## 概述

对标飞牛fnOS按需唤醒硬盘功能，实现智能休眠/唤醒策略。

## 核心功能

### 1. 智能休眠策略
- **检测访问模式**: 监控磁盘IO模式（连续访问 vs 随机访问）
- **空闲计时**: 设置空闲阈值（可配置，默认30分钟）
- **自动休眠**: 空闲超时后自动进入低功耗模式

### 2. 唤醒触发机制
- **IO唤醒**: 检测到磁盘访问请求立即唤醒
- **预热唤醒**: 定时任务前预先唤醒（如备份、快照）
- **手动唤醒**: 用户主动触发唤醒

## API设计

### REST API

```go
// 磁盘电源状态
type DiskPowerState struct {
    DiskID      string    `json:"disk_id"`
    State       string    `json:"state"`       // active/standby/sleep
    LastActive  time.Time `json:"last_active"`
    IdleMinutes int       `json:"idle_minutes"`
}

// 电源策略配置
type PowerPolicy struct {
    IdleThresholdMinutes int  `json:"idle_threshold_minutes"` // 空闲阈值(分钟)
    StandbyEnabled       bool `json:"standby_enabled"`        // 是否启用休眠
    ExcludeDisks         []string `json:"exclude_disks"`      // 排除的磁盘(如系统盘)
    PreWarmForBackup     bool `json:"prewarm_for_backup"`     // 备份前预热
}

// API endpoints
// GET  /api/v1/storage/disks/power-state        - 获取所有磁盘电源状态
// GET  /api/v1/storage/disks/{id}/power-state   - 获取指定磁盘状态
// POST /api/v1/storage/disks/{id}/power-control - 控制磁盘电源(唤醒/休眠)
// PUT  /api/v1/storage/power-policy             - 更新电源策略
```

## 实现要点

### 1. 状态监控服务
```go
// DiskPowerMonitor - 磁盘电源监控服务
type DiskPowerMonitor struct {
    policy      PowerPolicy
    states      map[string]*DiskPowerState
    monitorChan chan DiskIOEvent
}

func (m *DiskPowerMonitor) Run(ctx context.Context) {
    for {
        select {
        case event := <-m.monitorChan:
            m.handleIOEvent(event)
        case <-time.Tick(1 * time.Minute):
            m.checkIdleTimeout()
        }
    }
}
```

### 2. 与ZFS集成
- 使用 `zpool status` 获取磁盘列表
- 使用 `hdparm -C` 检查电源状态
- 使用 `hdparm -y` 进入休眠
- 使用 `hdparm -S` 设置自动休眠时间

### 3. 健康检查
- 休眠前检查磁盘健康状况（SMART）
- 排除故障盘，避免唤醒导致进一步损坏
- 监控唤醒时间，异常时告警

## 配置示例

```yaml
disk_power_management:
  idle_threshold_minutes: 30
  standby_enabled: true
  exclude_disks:
    - "sda"  # 系统盘
  prewarm_for_backup: true
  smart_check_before_sleep: true
```

## 竞品对标

| 功能 | 飞牛fnOS | nas-os设计 |
|------|----------|-----------|
| 智能休眠 | ✅ | ✅ 本设计 |
| 访问模式检测 | ✅ | ✅ 监控IO模式 |
| 自动唤醒 | ✅ | ✅ IO触发唤醒 |
| 备份前预热 | ✅ | ✅ prewarm_for_backup |
| 排除系统盘 | ✅ | ✅ exclude_disks |

## 实现计划

| 阶段 | 任务 | 预期日期 |
|------|------|----------|
| M1 | 监控服务实现 | 04-10 |
| M2 | REST API | 04-15 |
| M3 | WebUI控制 | 04-20 |
| M4 | 测试与发布 | 04-25 |

## 注意事项

- SATA磁盘休眠可能影响RAID阵列稳定性
- 建议仅用于独立磁盘或冷存储池
- 企业级环境建议禁用休眠功能