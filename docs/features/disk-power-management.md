# 磁盘智能电源管理设计

## 功能概述
对标飞牛fnOS节能特性，实现磁盘智能休眠/唤醒功能。

## API设计

### 磁盘电源状态API
```
GET  /api/v1/storage/disks/:id/power        - 获取磁盘电源状态
POST /api/v1/storage/disks/:id/power/sleep  - 手动休眠磁盘
POST /api/v1/storage/disks/:id/power/wake   - 手动唤醒磁盘
GET  /api/v1/storage/disks/power/stats      - 获取电源统计
```

### 休眠策略API
```
GET  /api/v1/storage/power/policies         - 获取休眠策略列表
POST /api/v1/storage/power/policies         - 创建休眠策略
PUT  /api/v1/storage/power/policies/:id     - 更新策略
DELETE /api/v1/storage/power/policies/:id   - 删除策略
```

## 数据模型

### 磁盘电源状态
```go
type DiskPowerState struct {
    DiskID       string    `json:"disk_id"`
    Status       string    `json:"status"` // active, standby, sleeping
    LastActive   time.Time `json:"last_active"`
    SleepCount   int       `json:"sleep_count"`
    WakeCount    int       `json:"wake_count"`
    PowerSaving  float64   `json:"power_saving"` // kWh saved
}
```

### 休眠策略
```go
type SleepPolicy struct {
    ID           string        `json:"id"`
    Name         string        `json:"name"`
    Disks        []string      `json:"disks"` // 磁盘列表
    IdleTimeout  time.Duration `json:"idle_timeout"` // 闲置超时
    Schedule     ScheduleSpec  `json:"schedule"` // 定时休眠
    WakeOnIO     bool          `json:"wake_on_io"` // IO自动唤醒
    Enabled      bool          `json:"enabled"`
}
```

## 实现要点

1. **状态检测**: 使用 hdparm -C 或 SCSI ATA Translation 检测磁盘状态
2. **休眠控制**: 使用 hdparm -Y 或 sg_io 命令
3. **IO监控**: 监控磁盘IO活动，触发唤醒
4. **策略调度**: 定时任务调度休眠策略

## WebUI展示
- 磁盘电源状态可视化
- 节能统计仪表板
- 策略配置界面
- 自动唤醒日志

## 版本计划
- v2.362.0: API设计完成
- v2.365.0: 核心功能实现
- v2.370.0: WebUI集成