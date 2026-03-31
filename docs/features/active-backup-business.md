# Active Backup for Business 设计

> 对标群晖DSM Active Backup for Business

## 概述

企业级备份解决方案，支持物理机、虚拟机、云服务备份。

## API设计

### 备份任务管理

```
GET    /api/v1/backup/tasks                   # 备份任务列表
POST   /api/v1/backup/tasks                   # 创建备份任务
PUT    /api/v1/backup/tasks/:id               # 更新任务
DELETE /api/v1/backup/tasks/:id               # 删除任务
POST   /api/v1/backup/tasks/:id/run           # 执行备份
GET    /api/v1/backup/tasks/:id/status        # 任务状态
```

### 备份目标类型

```
GET    /api/v1/backup/targets                 # 备份目标列表
POST   /api/v1/backup/targets/physical        # 添加物理机
POST   /api/v1/backup/targets/vm              # 添加虚拟机
POST   /api/v1/backup/targets/cloud           # 添加云服务
```

### 恢复管理

```
GET    /api/v1/backup/recovery-points         # 恢复点列表
POST   /api/v1/backup/recover                 # 执行恢复
GET    /api/v1/backup/recover/:id/status      # 恢复状态
```

## 数据模型

```go
type BackupTask struct {
    ID           string       `json:"id"`
    Name         string       `json:"name"`
    TargetType   string       `json:"target_type"` // physical, vm, cloud
    Target       BackupTarget `json:"target"`
    Schedule     Schedule     `json:"schedule"`
    Retention    Retention    `json:"retention"`
    Compression  bool         `json:"compression"`
    Encryption   bool         `json:"encryption"`
    Status       string       `json:"status"`
    LastRun      time.Time    `json:"last_run"`
    NextRun      time.Time    `json:"next_run"`
}

type BackupTarget struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Type     string `json:"type"`
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    OS       string `json:"os"` // windows, linux, macos
}

type Retention struct {
    Daily   int `json:"daily"`   // 保留天数
    Weekly  int `json:"weekly"`  // 保留周数
    Monthly int `json:"monthly"` // 保留月数
}

type RecoveryPoint struct {
    ID        string    `json:"id"`
    TaskID    string    `json:"task_id"`
    Timestamp time.Time `json:"timestamp"`
    Size      int64     `json:"size"`
    Type      string    `json:"type"` // full, incremental
    Status    string    `json:"status"`
}
```

## 功能特性

1. **多平台支持**: Windows、Linux、MacOS物理机
2. **VM备份**: VMware、Hyper-V虚拟机快照
3. **增量备份**: 智能增量备份节省空间
4. **加密传输**: AES-256加密传输存储
5. **RPO/RTO**: 自定义恢复点目标
6. **压缩存储**: 多算法压缩节省空间

## 实现要点

- 备份代理客户端设计
- rsync/borg备份引擎
- 增量备份块级去重
- 任务调度cron系统