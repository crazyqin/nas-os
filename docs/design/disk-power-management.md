# 硬盘按需唤醒功能设计文档

> Version: v1.0
> Author: 兵部（软件工程部门）
> Date: 2026-03-29
> Status: 设计阶段

---

## 1. 功能概述

### 1.1 背景

NAS系统通常配置多块硬盘用于数据存储，硬盘是系统中最主要的能耗来源之一。在空闲时段，硬盘持续空转消耗电力，产生噪音和热量。参考飞牛fnOS的"按需唤醒硬盘"功能，本设计旨在实现nas-os的硬盘节能方案：

- **节能降耗**：空闲硬盘自动休眠，减少电力消耗
- **延长寿命**：减少硬盘空转时间，降低机械磨损
- **降低噪音**：休眠硬盘停止旋转，降低运行噪音
- **智能唤醒**：访问休眠硬盘时自动唤醒，用户无感知

### 1.2 功能目标

1. **空闲超时休眠**：硬盘空闲超过设定时间后自动进入休眠状态
2. **按需唤醒**：当有I/O请求访问休眠硬盘时自动唤醒
3. **灵活配置**：支持全局配置和单盘个性化设置
4. **系统盘保护**：自动识别并排除系统盘，避免系统不稳定
5. **状态监控**：实时显示各硬盘电源状态，提供唤醒日志

### 1.3 适用场景

- 家庭NAS：夜间或工作时段空闲，硬盘可休眠节能
- 小型办公NAS：非工作时间硬盘休眠
- 多盘位NAS：部分盘位数据访问频率低，适合休眠策略

---

## 2. 技术原理

### 2.1 硬盘电源状态

ATA硬盘定义了四种电源状态：

| 状态 | 描述 | 功耗 | 响应时间 |
|------|------|------|----------|
| **Active** | 完全运行，读写操作 | 最高 | 即时 |
| **Idle** | 空转，磁盘旋转但无I/O | 较高 | 即时 |
| **Standby** | 待机，磁盘停止旋转，缓存保留 | 很低 | 5-15秒唤醒 |
| **Sleep** | 深度休眠，几乎完全关闭 | 极低 | 10-30秒唤醒 |

**设计选择**：使用 **Standby** 状态作为休眠目标，兼顾节能效果和唤醒响应时间。

### 2.2 Linux硬盘电源管理工具

#### 2.2.1 hdparm

`hdparm` 是Linux标准的硬盘电源管理工具：

```bash
# 查看当前电源状态
hdparm -C /dev/sda

# 设置空闲超时（单位：5秒倍数）
# 例如：-S 12 表示60秒后进入standby
hdparm -S <value> /dev/sda

# 立即进入standby
hdparm -y /dev/sda

# 立即进入sleep（需要更长时间唤醒）
hdparm -Y /dev/sda

# 设置APM级别（1-255）
# 1-127: 允许standby
# 128-254: 不允许standby，但允许其他节能
# 255: 完全禁用APM
hdparm -B <level> /dev/sda
```

**超时值编码表**（-S参数）：

| 值 | 超时时间 |
|----|----------|
| 0 | 关闭休眠功能 |
| 1-240 | 值 × 5秒（5秒到20分钟） |
| 241-251 | (值-240) × 30分钟（30分钟到5.5小时） |
| 252 | 21分钟 |
| 253 | 供应商特定（通常8-12小时） |
| 254 | 供应商特定 |
| 255 | 21分钟15秒 |

#### 2.2.2 SATA链路电源管理（ALPM）

对于SATA硬盘，还可以启用链路层电源管理：

```bash
# 查看ALPM状态
cat /sys/class/scsi_host/host*/link_power_management_policy

# 设置ALPM策略
# min_power: 最大节能（可能影响性能）
# medium_power: 中等节能
# max_performance: 无节能
echo min_power > /sys/class/scsi_host/host0/link_power_management_policy
```

#### 2.2.3 udev规则持久化

通过udev规则确保配置在系统重启后生效：

```bash
# /etc/udev/rules.d/50-hdparm.rules
ACTION=="add", KERNEL=="sd[a-z]", ATTR{queue/rotational}=="1", RUN+="/usr/bin/hdparm -B 127 -S 241 $root/%k"
```

### 2.3 唤醒机制

当应用程序尝试访问休眠的硬盘时：

1. **内核检测**：内核检测到I/O请求目标磁盘处于standby
2. **自动唤醒**：内核发送ATA命令唤醒磁盘
3. **等待就绪**：磁盘旋转到正常速度（5-15秒）
4. **执行I/O**：唤醒完成后执行请求的I/O操作

**用户感知**：首次访问休眠硬盘会有5-15秒延迟，之后正常响应。

### 2.4 btrfs与硬盘休眠

#### 2.4.1 潜在冲突

btrfs的一些特性可能导致硬盘频繁唤醒：

| 特性 | 影响 | 解决方案 |
|------|------|----------|
| **周期性scrub** | 定期扫描所有设备唤醒硬盘 | 仅对active设备执行，或安排在唤醒时段 |
| **后台清理** | 空间回收可能触发I/O | 配置清理任务运行时段 |
| **设备统计** | 定期读取设备状态 | 可配置检查间隔 |
| **元数据更新** | 某些操作会写入所有设备 | 正常，唤醒必要设备 |

#### 2.4.2 最佳实践

```bash
# 禁用btrfs设备定期状态检查（减少唤醒）
btrfs device stats /mnt/data --clear

# scrub仅在需要时手动触发，或安排在特定时段
btrfs scrub start /mnt/data

# 查看当前scrub状态
btrfs scrub status /mnt/data
```

**设计策略**：nas-os应协调btrfs维护任务与硬盘休眠策略：
- 系统维护任务（scrub、balance）安排在用户可配置时段
- 维护任务执行期间临时禁用休眠或仅唤醒必要硬盘
- 提供任务完成后恢复休眠的机制

### 2.5 系统盘识别

系统盘不应进入休眠状态，原因：

1. **操作系统稳定性**：系统盘I/O中断会导致系统响应延迟甚至卡死
2. **日志写入**：系统日志、应用日志持续写入系统盘
3. **缓存依赖**：系统缓存可能频繁访问系统盘

**识别方法**：

```bash
# 方法1：通过挂载点识别
mount | grep -E "on / |on /boot |on /home "

# 方法2：通过fstab识别
grep -E "^/dev|^UUID" /etc/fstab | grep -E "/ |/boot |/home "

# 方法3：通过root设备识别
findmnt -n -o SOURCE /

# 方法4：通过内核命令行识别
cat /proc/cmdline | grep -oP 'root=\K[^ ]+'
```

---

## 3. 配置选项

### 3.1 全局配置

```yaml
# /etc/nas-os/disk-power.yaml
power_management:
  enabled: true
  
  # 全局默认空闲超时（分钟）
  # 0 = 禁用休眠
  # 5-360 = 允许范围
  default_idle_timeout: 30
  
  # APM级别（1-255）
  # 127 = 允许standby
  # 128 = 不允许standby
  default_apm_level: 127
  
  # SATA ALPM策略
  alpm_policy: "medium_power"  # min_power | medium_power | max_performance
  
  # 自动排除系统盘
  exclude_system_disk: true
  
  # 唤醒日志记录
  wakeup_logging: true
  
  # 唤醒日志保留天数
  wakeup_log_retention: 30
```

### 3.2 单盘配置

```yaml
# 单盘个性化配置（可选）
disks:
  /dev/sdb:
    enabled: true
    idle_timeout: 60  # 1小时
    apm_level: 128    # 不自动休眠，依赖手动控制
    
  /dev/sdc:
    enabled: true
    idle_timeout: 15  # 15分钟
    apm_level: 127
    
  /dev/sdd:
    enabled: false    # 禁用休眠（如热备盘）
    
  /dev/sde:
    idle_timeout: 120 # 2小时（低访问频率数据盘）
```

### 3.3 维护任务协调

```yaml
# btrfs维护任务配置
maintenance:
  # scrub配置
  scrub:
    enabled: true
    schedule: "Sunday 02:00"  # 每周日凌晨2点
    scope: "all"  # all | active_only
    
  # balance配置
  balance:
    enabled: false
    schedule: null
    
  # 维护期间休眠策略
  during_maintenance:
    suspend_sleep: true     # 维护期间暂停休眠
    auto_resume: true       # 维护完成后自动恢复
```

### 3.4 前端展示配置

| 配置项 | 类型 | 说明 |
|--------|------|------|
| 空闲超时 | Slider | 5-360分钟，或"禁用" |
| 单盘开关 | Toggle | 每块硬盘独立启用/禁用 |
| APM级别 | Select | 127/128/255 |
| 系统盘排除 | Info | 自动识别，不可配置 |
| 维护时段 | TimePicker | 选择scrub等任务运行时间 |

---

## 4. API设计

### 4.1 API端点

```
/api/v1/disk/power
```

### 4.2 获取电源配置

**GET `/api/v1/disk/power/config`**

Response:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "enabled": true,
    "default_idle_timeout": 30,
    "default_apm_level": 127,
    "alpm_policy": "medium_power",
    "exclude_system_disk": true,
    "wakeup_logging": true,
    "disks": {
      "/dev/sda": {
        "is_system_disk": true,
        "sleep_enabled": false,
        "reason": "系统盘自动排除"
      },
      "/dev/sdb": {
        "is_system_disk": false,
        "sleep_enabled": true,
        "idle_timeout": 60,
        "apm_level": 127
      }
    }
  }
}
```

### 4.3 更新全局配置

**PUT `/api/v1/disk/power/config`**

Request:
```json
{
  "enabled": true,
  "default_idle_timeout": 30,
  "default_apm_level": 127,
  "alpm_policy": "medium_power"
}
```

### 4.4 更新单盘配置

**PUT `/api/v1/disk/power/config/:device`**

Request:
```json
{
  "sleep_enabled": true,
  "idle_timeout": 60,
  "apm_level": 127
}
```

### 4.5 获取电源状态

**GET `/api/v1/disk/power/status`**

Response:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "disks": [
      {
        "device": "/dev/sda",
        "power_state": "active",
        "is_system_disk": true,
        "last_activity": "2026-03-29T10:00:00Z",
        "spindle_up_time": null
      },
      {
        "device": "/dev/sdb",
        "power_state": "standby",
        "is_system_disk": false,
        "last_activity": "2026-03-29T06:30:00Z",
        "spindle_up_time": null
      },
      {
        "device": "/dev/sdc",
        "power_state": "active",
        "is_system_disk": false,
        "last_activity": "2026-03-29T09:45:00Z",
        "spindle_up_time": "2026-03-29T09:30:00Z"
      }
    ],
    "summary": {
      "active": 2,
      "standby": 1,
      "sleep": 0,
      "unknown": 0
    }
  }
}
```

### 4.6 手动控制

**POST `/api/v1/disk/power/:device/standby`**

手动让指定硬盘进入休眠状态：

```json
{
  "code": 0,
  "message": "硬盘 /dev/sdb 已进入休眠状态"
}
```

**POST `/api/v1/disk/power/:device/wakeup`**

手动唤醒指定硬盘：

```json
{
  "code": 0,
  "message": "硬盘 /dev/sdb 已唤醒",
  "data": {
    "wakeup_time_ms": 8500
  }
}
```

### 4.7 唤醒日志

**GET `/api/v1/disk/power/wakeup-log`**

Query params:
- `device`: 过滤设备（可选）
- `start`: 开始时间
- `end`: 结束时间
- `limit`: 返回数量（默认100）

Response:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "logs": [
      {
        "id": "log_001",
        "device": "/dev/sdb",
        "timestamp": "2026-03-29T07:00:00Z",
        "trigger": "auto",  // auto | manual
        "wakeup_time_ms": 8500,
        "request_source": "btrfs-scrub"
      }
    ],
    "total": 45,
    "summary": {
      "total_wakeups": 45,
      "avg_wakeup_time_ms": 7800,
      "by_device": {
        "/dev/sdb": 30,
        "/dev/sdc": 15
      }
    }
  }
}
```

### 4.8 维护任务配置

**GET `/api/v1/disk/power/maintenance`**

**PUT `/api/v1/disk/power/maintenance`**

Request:
```json
{
  "scrub": {
    "enabled": true,
    "schedule": "Sunday 02:00",
    "scope": "active_only"
  },
  "during_maintenance": {
    "suspend_sleep": true
  }
}
```

---

## 5. 数据结构

### 5.1 Go结构定义

```go
// PowerState 磁盘电源状态
type PowerState string

const (
    PowerActive    PowerState = "active"    // 完全运行
    PowerIdle      PowerState = "idle"      // 空转
    PowerStandby   PowerState = "standby"   // 待机（休眠）
    PowerSleep     PowerState = "sleep"     // 深度休眠
    PowerUnknown   PowerState = "unknown"   // 未知
)

// DiskPowerConfig 磁盘电源配置
type DiskPowerConfig struct {
    Enabled          bool              `json:"enabled"`
    DefaultTimeout   int               `json:"default_idle_timeout"`  // 分钟
    DefaultAPMLevel  int               `json:"default_apm_level"`     // 1-255
    ALPMPolicy       string            `json:"alpm_policy"`
    ExcludeSystem    bool              `json:"exclude_system_disk"`
    WakeupLogging    bool              `json:"wakeup_logging"`
    LogRetention     int               `json:"wakeup_log_retention"`  // 天数
    DiskOverrides    map[string]DiskConfig `json:"disks"`
}

// DiskConfig 单盘配置
type DiskConfig struct {
    Enabled      bool  `json:"sleep_enabled"`
    Timeout      int   `json:"idle_timeout"`  // 分钟，0=禁用
    APMLevel     int   `json:"apm_level"`
    IsSystemDisk bool  `json:"is_system_disk"`
    Reason       string `json:"reason,omitempty"`  // 排除原因
}

// DiskPowerStatus 磁盘电源状态
type DiskPowerStatus struct {
    Device        string     `json:"device"`
    PowerState    PowerState `json:"power_state"`
    IsSystemDisk  bool       `json:"is_system_disk"`
    LastActivity  time.Time  `json:"last_activity"`
    SpindleUpTime *time.Time `json:"spindle_up_time,omitempty"`  // 最近唤醒时间
    Config        DiskConfig `json:"config"`
}

// WakeupLog 唤醒日志
type WakeupLog struct {
    ID            string    `json:"id"`
    Device        string    `json:"device"`
    Timestamp     time.Time `json:"timestamp"`
    Trigger       string    `json:"trigger"`  // auto | manual
    WakeupTimeMs  int       `json:"wakeup_time_ms"`
    RequestSource string    `json:"request_source,omitempty"`  // 触发唤醒的请求来源
}

// MaintenanceConfig 维护任务配置
type MaintenanceConfig struct {
    Scrub          ScrubConfig `json:"scrub"`
    Balance        BalanceConfig `json:"balance"`
    DuringMaintenance MaintenancePolicy `json:"during_maintenance"`
}

type ScrubConfig struct {
    Enabled  bool   `json:"enabled"`
    Schedule string `json:"schedule"`  // cron格式或"Sunday 02:00"
    Scope    string `json:"scope"`     // all | active_only
}

type MaintenancePolicy struct {
    SuspendSleep bool `json:"suspend_sleep"`
    AutoResume   bool `json:"auto_resume"`
}
```

---

## 6. 实现计划

### 6.1 开发阶段

| 阶段 | 任务 | 预估时间 | 优先级 |
|------|------|----------|--------|
| **Phase 1** | 核心功能 | 2周 | P0 |
| | - 电源状态检测与监控 | | |
| | - hdparm集成与配置应用 | | |
| | - 系统盘自动识别与排除 | | |
| | - 基础API实现 | | |
| **Phase 2** | 配置管理 | 1周 | P1 |
| | - 全局配置存储与加载 | | |
| | - 单盘个性化配置 | | |
| | - 配置持久化（udev规则） | | |
| | - 前端配置界面 | | |
| **Phase 3** | 监控与日志 | 1周 | P1 |
| | - 实时电源状态监控 | | |
| | - 唤醒事件检测与记录 | | |
| | - 唤醒统计与分析 | | |
| | - 前端状态展示 | | |
| **Phase 4** | 维护任务协调 | 1周 | P2 |
| | - btrfs scrub调度集成 | | |
| | - 维护期间休眠策略 | | |
| | - 任务完成后恢复机制 | | |
| **Phase 5** | 测试与优化 | 1周 | P0 |
| | - 功能测试 | | |
| | - 性能测试（唤醒延迟） | | |
| | - 稳定性测试 | | |
| | - 文档完善 | | |

**总预估时间：6周**

### 6.2 技术实现要点

#### 6.2.1 电源状态检测

```go
// 使用hdparm检测电源状态
func (m *PowerManager) detectPowerState(device string) (PowerState, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, "hdparm", "-C", device)
    output, err := cmd.Output()
    if err != nil {
        return PowerUnknown, err
    }
    
    // 解析hdparm输出
    // "drive state is: active/idle"
    // "drive state is: standby"
    // "drive state is: sleeping"
    result := strings.ToLower(string(output))
    if strings.Contains(result, "standby") {
        return PowerStandby, nil
    }
    if strings.Contains(result, "sleeping") {
        return PowerSleep, nil
    }
    if strings.Contains(result, "active") || strings.Contains(result, "idle") {
        return PowerActive, nil
    }
    
    return PowerUnknown, nil
}
```

#### 6.2.2 应用休眠配置

```go
// 应用休眠超时配置
func (m *PowerManager) applyIdleTimeout(device string, timeoutMinutes int) error {
    // hdparm -S 值编码
    var sValue int
    if timeoutMinutes == 0 {
        sValue = 0  // 禁用休眠
    } else if timeoutMinutes <= 20 {
        sValue = timeoutMinutes * 60 / 5  // 5秒倍数
    } else if timeoutMinutes <= 330 {
        sValue = 240 + timeoutMinutes/30  // 30分钟倍数
    } else {
        sValue = 251  // 最大5.5小时
    }
    
    cmd := exec.Command("hdparm", "-S", strconv.Itoa(sValue), device)
    return cmd.Run()
}

// 应用APM级别
func (m *PowerManager) applyAPMLevel(device string, level int) error {
    cmd := exec.Command("hdparm", "-B", strconv.Itoa(level), device)
    return cmd.Run()
}
```

#### 6.2.3 系统盘识别

```go
// 识别系统盘
func (m *PowerManager) identifySystemDisks() ([]string, error) {
    systemDisks := []string{}
    
    // 方法1：通过root挂载点识别
    cmd := exec.Command("findmnt", "-n", "-o", "SOURCE", "/")
    output, err := cmd.Output()
    if err == nil {
        rootDevice := strings.TrimSpace(string(output))
        // 解析设备名（可能是UUID或设备路径）
        if strings.HasPrefix(rootDevice, "/dev/") {
            systemDisks = append(systemDisks, rootDevice)
        } else if strings.HasPrefix(rootDevice, "UUID=") {
            // 通过UUID查找设备
            uuid := strings.TrimPrefix(rootDevice, "UUID=")
            device, _ := m.resolveUUID(uuid)
            if device != "" {
                systemDisks = append(systemDisks, device)
            }
        }
    }
    
    // 方法2：检查/boot和/home挂载点
    mountPoints := []string{"/boot", "/home", "/var"}
    for _, mp := range mountPoints {
        cmd := exec.Command("findmnt", "-n", "-o", "SOURCE", mp)
        output, err := cmd.Output()
        if err == nil {
            device := strings.TrimSpace(string(output))
            if strings.HasPrefix(device, "/dev/") && !contains(systemDisks, device) {
                // 检查是否是同一块盘的不同分区
                baseDevice := getBaseDevice(device)
                if !contains(systemDisks, baseDevice) {
                    systemDisks = append(systemDisks, baseDevice)
                }
            }
        }
    }
    
    return systemDisks, nil
}

// 获取基础设备名（去除分区号）
func getBaseDevice(device string) string {
    // /dev/sda1 -> /dev/sda
    // /dev/nvme0n1p1 -> /dev/nvme0n1
    re := regexp.MustCompile(`(sd[a-z]|nvme[0-9]+n[0-9]+|hd[a-z])`)
    match := re.FindString(device)
    if match != "" {
        return "/dev/" + match
    }
    return device
}
```

#### 6.2.4 udev规则持久化

```go
// 生成udev规则文件
func (m *PowerManager) generateUdevRules() error {
    rules := m.buildUdevRulesContent()
    
    rulesPath := "/etc/udev/rules.d/99-nas-os-disk-power.rules"
    return os.WriteFile(rulesPath, []byte(rules), 0644)
}

func (m *PowerManager) buildUdevRulesContent() string {
    var lines []string
    lines = append(lines, "# NAS-OS Disk Power Management Rules")
    lines = append(lines, "# Generated by nas-os disk power manager")
    
    for device, config := range m.config.DiskOverrides {
        if !config.Enabled || config.IsSystemDisk {
            continue
        }
        
        // 提取内核设备名
        kernelName := strings.TrimPrefix(device, "/dev/")
        
        // APM级别
        lines = append(lines, fmt.Sprintf(
            "ACTION==\"add\", KERNEL==\"%s\", RUN+=\"/usr/bin/hdparm -B %d $root/%k\"",
            kernelName, config.APMLevel,
        ))
        
        // 休眠超时
        sValue := m.calculateSValue(config.Timeout)
        lines = append(lines, fmt.Sprintf(
            "ACTION==\"add\", KERNEL==\"%s\", RUN+=\"/usr/bin/hdparm -S %d $root/%k\"",
            kernelName, sValue,
        ))
    }
    
    return strings.Join(lines, "\n") + "\n"
}
```

#### 6.2.5 唤醒事件检测

```go
// 监控唤醒事件（通过定期状态变化检测）
func (m *PowerManager) monitorWakeupEvents() {
    ticker := time.NewTicker(10 * time.Second)
    prevStates := make(map[string]PowerState)
    
    for {
        select {
        case <-ticker.C:
            m.checkWakeupEvents(prevStates)
        case <-m.stopChan:
            return
        }
    }
}

func (m *PowerManager) checkWakeupEvents(prevStates map[string]PowerState) {
    for device := range m.config.DiskOverrides {
        currentState, err := m.detectPowerState(device)
        if err != nil {
            continue
        }
        
        prevState := prevStates[device]
        prevStates[device] = currentState
        
        // 检测唤醒：从standby/sleep变为active
        if (prevState == PowerStandby || prevState == PowerSleep) && 
           currentState == PowerActive {
            m.recordWakeupEvent(device)
        }
    }
}

func (m *PowerManager) recordWakeupEvent(device string) {
    if !m.config.WakeupLogging {
        return
    }
    
    log := WakeupLog{
        ID:        fmt.Sprintf("wakeup_%d", time.Now().UnixNano()),
        Device:    device,
        Timestamp: time.Now(),
        Trigger:   "auto",
    }
    
    m.wakeupLogs = append(m.wakeupLogs, log)
    m.pruneOldLogs()
}
```

### 6.3 依赖与前置条件

| 依赖 | 说明 |
|------|------|
| **hdparm** | 需要系统安装hdparm工具 |
| **root权限** | hdparm命令需要root权限执行 |
| **ATA硬盘** | 仅适用于SATA/PATA硬盘，NVMe需不同方案 |
| **udev** | 需要udev支持配置持久化 |

**NVMe硬盘说明**：
NVMe SSD不支持传统standby休眠。对于NVMe，可考虑：
- APST (Autonomous Power State Transition) 配置
- 通过nvme-cli设置电源状态

```bash
# NVMe电源状态
nvme get-feature -f 0x02 /dev/nvme0  # 获取电源配置
nvme set-feature -f 0x02 -v 1 /dev/nvme0  # 设置电源状态
```

### 6.4 文件结构

```
internal/
└── diskpower/
    ├── manager.go          # 核心管理器
    ├── config.go           # 配置加载与保存
    ├── detector.go         # 电源状态检测
    ├── hdparm.go           # hdparm命令封装
    ├── systemdisk.go       # 系统盘识别
    ├── udev.go             # udev规则生成
    ├── wakeup.go           # 唤醒事件监控与记录
    ├── handlers.go         # API处理器
    ├── handlers_test.go    # API测试
    └── monitor.go          # 后台监控服务

config/
└── disk-power.yaml         # 默认配置文件

docs/design/
└── disk-power-management.md  # 本设计文档
```

### 6.5 风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| **频繁唤醒** | 降低节能效果，增加延迟 | 监控唤醒频率，优化超时设置 |
| **唤醒失败** | 数据访问失败 | 提供手动唤醒接口，错误提示 |
| **系统不稳定** | 系统盘误休眠 | 自动排除系统盘，配置验证 |
| **配置丢失** | 重启后失效 | udev规则持久化 |
| **btrfs冲突** | 维护任务唤醒硬盘 | 维护任务协调机制 |

---

## 7. 附录

### 7.1 hdparm命令参考

```bash
# 查看电源状态
hdparm -C /dev/sda

# 查看APM设置
hdparm -I /dev/sda | grep -i "Advanced Power Management"

# 查看当前设置
hdparm -I /dev/sda

# 安全测试（不会立即休眠）
hdparm -B 127 /dev/sda

# 禁用休眠（测试用途）
hdparm -B 255 /dev/sda
```

### 7.2 状态监控脚本示例

```bash
#!/bin/bash
# disk-power-monitor.sh - 定期检查硬盘电源状态

DISKS="/dev/sdb /dev/sdc /dev/sdd"

for disk in $DISKS; do
    state=$(hdparm -C $disk 2>/dev/null | grep -oP 'drive state is: \K.*')
    timestamp=$(date -Iseconds)
    echo "$timestamp $disk $state"
done
```

### 7.3 参考资源

- [Linux hdparm Documentation](https://man7.org/linux/man-pages/man8/hdparm.8.html)
- [ATA Power Management Specification](https://www.t13.org/)
- [btrfs Maintenance Tasks](https://btrfs.readthedocs.io/en/latest/Maintenance.html)
- [SATA Link Power Management](https://www.kernel.org/doc/Documentation/ata.txt)

---

**文档完成，待审核后进入开发阶段。**