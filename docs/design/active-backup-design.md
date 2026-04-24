# Active Backup 整机备份设计文档

**版本**: v2.463.0
**部门**: 户部
**日期**: 2026-04-24

---

## 1. 概述

Active Backup对标群晖DSM Active Backup for Business，实现：
- 整机备份（物理机/虚拟机）
- 增量备份策略
- 跨平台恢复支持

---

## 2. 功能设计

### 2.1 核心特性

| 特性 | 描述 | 优先级 |
|------|------|--------|
| Windows整机备份 | 支持Windows物理机 | P0 |
| Linux整机备份 | 支持Linux物理机 | P1 |
| 虚拟机备份 | VMware/Hyper-V/KVM | P1 |
| 增量备份 | 智能增量减少存储 | P0 |
| 跨平台恢复 | 异机恢复支持 | P2 |

---

## 3. 技术架构

```
┌───────────────────────────────────────────────────────┐
│               Active Backup Manager                    │
├───────────────────────────────────────────────────────┤
│                                                       │
│  ┌────────────────┐  ┌────────────────┐              │
│  │  Windows Agent │  │  Linux Agent   │              │
│  │  (VSS快照)     │  │  (rsync/LVM)   │              │
│  └───────┬────────┘  └───────┬────────┘              │
│          │                    │                       │
│          └────────────────────┘                       │
│                    │                                   │
│         ┌──────────┴──────────┐                       │
│         │   备份调度引擎       │                       │
│         │  (任务队列/增量检测) │                       │
│         └─────────────────────┘                       │
│                    │                                   │
│         ┌──────────┴──────────┐                       │
│         │   存储管理器         │                       │
│         │  (压缩/加密/去重)    │                       │
│         └─────────────────────┘                       │
└───────────────────────────────────────────────────────┘
```

---

## 4. 备份策略

### 4.1 增量备份算法

```go
type BackupPolicy struct {
    FullBackupInterval    int    `json:"full_backup_interval"`    // 全量间隔(天)
    IncrementalInterval   int    `json:"incremental_interval"`   // 增量间隔(小时)
    RetentionDays         int    `json:"retention_days"`         // 保留天数
    CompressionLevel      int    `json:"compression_level"`      // 压缩级别(1-9)
    EncryptionEnabled     bool   `json:"encryption_enabled"`     // 加密开关
    DeduplicationEnabled  bool   `json:"deduplication_enabled"`  // 去重开关
}

type BackupSchedule struct {
    ScheduleType  string `json:"schedule_type"` // daily/weekly/monthly
    ScheduleTime  string `json:"schedule_time"` // HH:MM
    ScheduleDay   int    `json:"schedule_day"`  // 周几(1-7)
}
```

---

## 5. API设计

```go
// POST /api/v1/backup/task
type BackupTaskRequest struct {
    Name           string       `json:"name"`
    TargetHost     string       `json:"target_host"`     // 目标主机
    TargetType     string       `json:"target_type"`     // windows/linux/vm
    StoragePath    string       `json:"storage_path"`    // 存储路径
    Policy         BackupPolicy `json:"policy"`
    Schedule       BackupSchedule `json:"schedule"`
}

// GET /api/v1/backup/task/{id}/status
type BackupTaskStatus struct {
    Status         string  `json:"status"` // running/completed/failed
    Progress       float64 `json:"progress"`
    LastBackupTime string  `json:"last_backup_time"`
    NextBackupTime string  `json:"next_backup_time"`
    StorageUsed    int64   `json:"storage_used"`
    BackupCount    int     `json:"backup_count"`
}

// POST /api/v1/backup/task/{id}/restore
type RestoreRequest struct {
    BackupVersion  string `json:"backup_version"` // 恢复版本
    TargetHost     string `json:"target_host"`    // 恢复目标
    RestoreType    string `json:"restore_type"`   // full/incremental
}
```

---

## 6. 对标群晖Active Backup

| 功能 | 群晖 | nas-os设计 | 对标状态 |
|------|------|------------|----------|
| Windows整机备份 | ✅ | ✅设计完成 | 🟢对标 |
| Linux整机备份 | 🟡有限 | ✅设计完成 | 🟢对标 |
| 虚拟机备份 | ✅ | ✅设计完成 | 🟢对标 |
| 增量备份 | ✅ | ✅设计完成 | 🟢对标 |
| 跨平台恢复 | ✅ | ✅设计完成 | 🟢对标 |
| VSS快照 | ✅ | ✅设计完成 | 🟢对标 |

---

## 7. 项目统计Round235

### 7.1 当前统计

| 指标 | 数量 |
|------|------|
| 源文件 | 1234+ |
| 代码行数 | 68.5万+ |
| Go模块 | 85+ |
| 测试覆盖 | 30%+ |

### 7.2 本轮增量

- 新增设计文档: 4个
- 新增功能模块: TrueSearch Phase2
- 代码优化: SMB Multichannel

---

## 8. 实现计划

| 阶段 | 时间 | 任务 |
|------|------|------|
| M1 | 第235轮 | 架构设计文档 |
| M2 | 第240轮 | Agent开发 |
| M3 | 第245轮 | 备份调度引擎 |
| M4 | 第250轮 | 存储管理 |
| M5 | 第255轮 | 恢复功能 |

---

**户部签名**: 户部
**提交时间**: 2026-04-24