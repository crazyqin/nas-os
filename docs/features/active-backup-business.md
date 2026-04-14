# Active Backup for Business 设计

## 功能概述
对标群晖DSM Active Backup for Business，实现物理/虚拟机集中备份。

## API设计

### 备份任务API
```
GET  /api/v1/backup/abb/tasks               - 获取备份任务列表
POST /api/v1/backup/abb/tasks               - 创建备份任务
GET  /api/v1/backup/abb/tasks/:id           - 获取任务详情
PUT  /api/v1/backup/abb/tasks/:id           - 更新任务
DELETE /api/v1/backup/abb/tasks/:id         - 删除任务
POST /api/v1/backup/abb/tasks/:id/run       - 执行备份
POST /api/v1/backup/abb/tasks/:id/restore   - 执行恢复
```

### 备份代理API
```
GET  /api/v1/backup/abb/agents              - 获取代理列表
POST /api/v1/backup/abb/agents/register     - 注册代理
DELETE /api/v1/backup/abb/agents/:id        - 移除代理
GET  /api/v1/backup/abb/agents/:id/status   - 获取代理状态
```

### 备份目标API
```
GET  /api/v1/backup/abb/targets             - 获取备份目标
POST /api/v1/backup/abb/targets             - 添加备份目标
```

## 数据模型

### 备份任务
```go
type ABBackupTask struct {
    ID           string        `json:"id"`
    Name         string        `json:"name"`
    Type         string        `json:"type"` // full, incremental
    Target       BackupTarget  `json:"target"`
    Schedule     ScheduleSpec  `json:"schedule"`
    RPO          time.Duration `json:"rpo"` // 恢复点目标
    RTO          time.Duration `json:"rto"` // 恢复时间目标
    Retention    RetentionPolicy `json:"retention"`
    Compression  string        `json:"compression"`
    Encryption   bool          `json:"encryption"`
    Status       string        `json:"status"`
    LastRun      time.Time     `json:"last_run"`
    NextRun      time.Time     `json:"next_run"`
}
```

### 备份代理
```go
type BackupAgent struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    Type         string    `json:"type"` // windows, linux, mac, vm
    IP           string    `json:"ip"`
    Port         int       `json:"port"`
    Version      string    `json:"version"`
    Status       string    `json:"status"` // online, offline
    LastContact  time.Time `json:"last_contact"`
}
```

## 支持的备份类型

| 类型 | 说明 |
|------|------|
| Windows Agent | Windows物理机备份 |
| Linux Agent | Linux物理机备份 |
| macOS Agent | macOS物理机备份 |
| VMware VM | VMware虚拟机备份 |
| Hyper-V VM | Hyper-V虚拟机备份 |
| KVM/QEMU VM | Linux KVM虚拟机备份 |

## 代理安装脚本

### Linux Agent
```bash
curl -fsSL https://nas-server:8080/api/v1/backup/abb/agents/install.sh | bash
```

### Windows Agent
```powershell
Invoke-WebRequest -Uri "https://nas-server:8080/api/v1/backup/abb/agents/install.ps1" | PowerShell
```

## 实现要点

1. **增量备份**: 基于块级变化的增量备份
2. **压缩传输**: 支持lz4/zstd压缩
3. **加密存储**: AES-256加密
4. **多目标**: 支持本地/NFS/S3多目标
5. **即时恢复**: 支持快照即时恢复

## WebUI展示
- 任务管理界面
- 代理状态监控
- 备份进度可视化
- 恢复操作界面

## 版本计划
- v2.362.0: API设计完成
- v2.370.0: Linux Agent实现
- v2.380.0: Windows Agent实现
- v2.390.0: VM备份实现
## v2.454.0 实现状态更新

### 群晖 Active Backup for Business 功能对标

| 子功能 | DSM ABB | nas-os | 优先级 | 状态 |
|--------|---------|--------|--------|------|
| 整机备份（PC/Server） | ✅ | 📋 规划中 | P2 | 待开发 |
| 裸机恢复 | ✅ | 📋 规划中 | P2 | 待开发 |
| 备份到本地存储 | ✅ | ✅ 已有 | 🟢 | 完成 |
| 备份到云存储 | ✅ | ✅ Cloud Sync | 🟢 | 完成 |
| 增量备份 | ✅ | ✅ 快照 | 🟢 | 完成 |
| 备份去重 | ✅ | 📋 规划中 | P2 | 待开发 |
| 备份加密 | ✅ | ✅ | 🟢 | 完成 |
| 备份计划 | ✅ | ✅ Cron | 🟢 | 完成 |
| 恢复向导 | ✅ | 📋 规划中 | P2 | 待开发 |

### 实现建议
1. 利用现有快照系统作为备份基础
2. 与 Cloud Sync 模块集成支持多云备份
3. 设计备份恢复向导 UI
4. 参考 Hyper Backup 设计去重方案

---
*更新于: v2.454.0*
