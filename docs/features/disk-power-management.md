# 硬盘电源管理设计

> 对标飞牛fnOS按需唤醒硬盘功能

## 概述

智能硬盘电源管理系统，支持自动休眠、按需唤醒、节能策略配置。

## API设计

### 电源状态管理

```
GET    /api/v1/storage/disks/:id/power       # 获取硬盘电源状态
POST   /api/v1/storage/disks/:id/power       # 设置电源状态
PUT    /api/v1/storage/disks/:id/power/sleep # 强制休眠
PUT    /api/v1/storage/disks/:id/power/wake  # 强制唤醒
```

### 电源策略

```
GET    /api/v1/storage/power/policies        # 获取电源策略列表
POST   /api/v1/storage/power/policies        # 创建电源策略
PUT    /api/v1/storage/power/policies/:id    # 更新策略
DELETE /api/v1/storage/power/policies/:id    # 删除策略
```

## 数据模型

```go
type DiskPowerState struct {
    DiskID      string    `json:"disk_id"`
    State       string    `json:"state"` // active, standby, sleeping
    LastActive  time.Time `json:"last_active"`
    SleepCount  int       `json:"sleep_count"`
    WakeCount   int       `json:"wake_count"`
}

type PowerPolicy struct {
    ID              string        `json:"id"`
    Name            string        `json:"name"`
    IdleTimeout     time.Duration `json:"idle_timeout"`  // 闲置超时(分钟)
    WakeOnAccess    bool          `json:"wake_on_access"` // 访问时唤醒
    Schedule        *Schedule     `json:"schedule"`       // 定时休眠
    Disks           []string      `json:"disks"`          // 应用磁盘
}

type Schedule struct {
    Weekdays []int `json:"weekdays"` // 0-6 (周日-周六)
    Hour     int   `json:"hour"`     // 休眠时间
    Minute   int   `json:"minute"`
}
```

## 功能特性

1. **智能休眠**: IO闲置超过阈值自动休眠
2. **按需唤醒**: 访请求时自动唤醒硬盘
3. **定时策略**: 支持定时休眠/唤醒调度
4. **批量操作**: 支持批量休眠/唤醒
5. **状态监控**: 实时电源状态监控
6. **节能统计**: 计算节省电量和费用

## 实现要点

- 使用 hdparm -Y 命令休眠硬盘
- 使用 hdparm -C 检查电源状态
- 通过IO监控判断闲置状态
- 优先级队列管理唤醒请求