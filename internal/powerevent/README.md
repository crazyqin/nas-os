# powerevent 电源事件管理模块

## 概述

powerevent 模块负责 NAS 系统的电源事件管理，包括定时开关机、UPS 事件处理、电源故障恢复策略、WOL 唤醒等功能。

## 功能特性

### 核心功能

1. **定时电源操作**
   - `SchedulePowerOn`: 定时开机（通过 WOL）
   - `SchedulePowerOff`: 定时关机
   - `ScheduleRestart`: 定时重启

2. **UPS 事件处理**
   - `HandleUPSEvent`: 处理 UPS 事件（电池供电、市电恢复、低电量、关机请求）
   - 自动低电量保护
   - 临界电量自动关机

3. **电源故障恢复**
   - 可配置的关机策略（优雅/立即/延迟）
   - 延迟关机支持
   - 电源事件日志记录

4. **Wake-on-LAN**
   - `TriggerWakeOnLan`: 发送 WOL 魔术包
   - 自动重试机制
   - 广播和定向唤醒支持

5. **电源调度**
   - `AddSchedule`: 添加定时任务
   - `RemoveSchedule`: 删除定时任务
   - `GetSchedules`: 获取所有调度
   - `UpdateSchedule`: 更新调度配置

6. **状态监控**
   - `CheckBatteryStatus`: 检查 UPS 电池状态
   - `UpdateUPSStatus`: 更新 UPS 状态
   - `GetPowerHistory`: 获取电源事件历史

## 核心类型

### Manager
电源事件管理器，负责协调所有电源相关操作。

```go
type Manager struct {
    config    Config
    logger    *zap.Logger
    events    []PowerEvent
    schedules map[string]*PowerSchedule
    upsStatus UPSStatus
    policy    ShutdownPolicy
    // ...
}
```

### PowerEvent
电源事件，记录每次电源操作。

```go
type PowerEvent struct {
    ID          string
    Type        PowerEventType
    State       PowerEventState
    ScheduledAt *time.Time
    ExecutedAt  *time.Time
    CompletedAt *time.Time
    TargetMAC   string
    TargetIP    string
    Message     string
    Error       string
    // ...
}
```

### PowerSchedule
电源调度配置。

```go
type PowerSchedule struct {
    ID             string
    Name           string
    Enabled        bool
    EventType      PowerEventType
    CronExpr       string
    TargetMAC      string
    TargetIP       string
    ShutdownPolicy ShutdownPolicy
    DelaySeconds   int
    // ...
}
```

### UPSStatus
UPS 状态信息。

```go
type UPSStatus struct {
    Online        bool
    BatteryLevel  int      // 0-100
    BatteryHealth string   // good, replace, unknown
    InputVoltage  float64
    OutputVoltage float64
    LoadPercent   int
    Temperature   float64
    EstimatedMin  int      // 预计剩余分钟数
    // ...
}
```

### ShutdownPolicy
关机策略类型。

```go
type ShutdownPolicy string

const (
    ShutdownPolicyGraceful  ShutdownPolicy = "graceful"   // 优雅关机
    ShutdownPolicyImmediate ShutdownPolicy = "immediate"  // 立即关机
    ShutdownPolicyDelayed   ShutdownPolicy = "delayed"    // 延迟关机
)
```

### Config
配置参数。

```go
type Config struct {
    LowBatteryThreshold      int           // 低电量阈值(默认20%)
    CriticalBatteryThreshold int           // 临界电量阈值(默认10%)
    UPSCheckInterval         time.Duration // UPS检查间隔
    ShutdownDelay            time.Duration // 关机延迟
    WOLRetryCount            int           // WOL重试次数
    WOLRetryInterval         time.Duration // WOL重试间隔
    MaxHistorySize           int           // 最大历史记录数
}
```

## 使用示例

### 创建管理器

```go
config := powerevent.DefaultConfig()
config.LowBatteryThreshold = 25
config.UPSCheckInterval = 60 * time.Second

logger, _ := zap.NewProduction()
manager := powerevent.NewManager(config, logger)

ctx := context.Background()
manager.Start(ctx)
defer manager.Stop()
```

### 定时开机

```go
scheduledAt := time.Now().Add(2 * time.Hour)
event, err := manager.SchedulePowerOn(ctx, scheduledAt, "00:11:22:33:44:55", "192.168.1.100")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("定时开机任务已创建: %s\n", event.ID)
```

### 处理 UPS 事件

```go
upsStatus := powerevent.UPSStatus{
    Online:       false,
    BatteryLevel: 15,
    EstimatedMin: 30,
}

err := manager.HandleUPSEvent(ctx, powerevent.PowerEventUPSOnBattery, upsStatus)
if err != nil {
    log.Fatal(err)
}
```

### 添加定时调度

```go
schedule := &powerevent.PowerSchedule{
    Name:           "每日凌晨关机",
    Enabled:        true,
    EventType:      powerevent.PowerEventPowerOff,
    CronExpr:       "0 2 * * *",
    ShutdownPolicy: powerevent.ShutdownPolicyGraceful,
    DelaySeconds:   300,
}

err := manager.AddSchedule(schedule)
if err != nil {
    log.Fatal(err)
}
```

### 触发 WOL 唤醒

```go
err := manager.TriggerWakeOnLan(ctx, "00:11:22:33:44:55", "192.168.1.255")
if err != nil {
    log.Fatal(err)
}
```

### 获取事件历史

```go
history := manager.GetPowerHistory(10)
for _, event := range history {
    fmt.Printf("[%s] %s - %s\n", event.CreatedAt.Format(time.RFC3339), event.Type, event.Message)
}
```

## 事件类型

| 事件类型 | 说明 |
|---------|------|
| `power_on` | 开机 |
| `power_off` | 关机 |
| `restart` | 重启 |
| `ups_on_battery` | UPS 切换到电池供电 |
| `ups_on_line` | UPS 恢复市电供电 |
| `ups_low_battery` | UPS 低电量 |
| `ups_shutdown` | UPS 请求关机 |
| `wol` | Wake-on-LAN 唤醒 |
| `scheduled` | 定时任务执行 |

## 事件状态

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行 |
| `running` | 执行中 |
| `completed` | 已完成 |
| `failed` | 失败 |
| `cancelled` | 已取消 |

## 低电量保护机制

1. **低电量警告** (默认 20%)
   - 发送警告通知
   - 记录事件日志
   - 不执行关机

2. **临界电量保护** (默认 10%)
   - 自动执行优雅关机
   - 延迟 30 秒（可配置）
   - 记录保护事件

## WOL 魔术包格式

Wake-on-LAN 魔术包格式：
- 6 字节同步流：`0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`
- 16 次重复的目标 MAC 地址
- 总长度：102 字节

## 测试

运行单元测试：

```bash
cd nas-os
go test ./internal/powerevent/ -v
```

测试覆盖率：

```bash
go test ./internal/powerevent/ -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 依赖

- `github.com/google/uuid`: UUID 生成
- `go.uber.org/zap`: 结构化日志
- `github.com/stretchr/testify`: 测试断言

## 注意事项

1. **WOL 限制**
   - WOL 只能在同一局域网内工作
   - 目标设备必须支持 WOL 功能
   - 需要正确配置 BIOS 和网卡设置

2. **UPS 集成**
   - 需要实现实际的 UPS 通信接口
   - 当前为模拟实现，生产环境需要集成 nut/upsc 等工具

3. **关机操作**
   - 实际关机需要调用系统命令
   - 当前为模拟实现，生产环境需要调用 shutdown/reboot 命令

4. **并发安全**
   - 所有公共方法都是线程安全的
   - 使用读写锁保护共享状态

## 许可证

内部模块，仅供 nas-os 项目使用。
