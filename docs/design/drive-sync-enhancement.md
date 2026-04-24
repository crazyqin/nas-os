# Drive Sync Enhancement 设计文档

**版本**: v2.463.0
**部门**: 礼部
**日期**: 2026-04-24

---

## 1. 概述

Drive Sync对标群晖Synology Drive，提供多设备文件同步服务。Phase1已完成基础同步，本轮增强：
- 协作编辑支持
- 版本控制优化
- 实时同步改进

---

## 2. Phase1回顾

| 功能 | Phase1状态 | 备注 |
|------|------------|------|
| 文件同步 | ✅完成 | 基础同步 |
| 增量同步 | ✅完成 | rsync算法 |
| 冲突检测 | 🟡基础 | 本轮增强 |
| 版本历史 | 🟡基础 | 本轮增强 |

---

## 3. Phase2增强目标

### 3.1 协作编辑

| 特性 | 描述 | 优先级 |
|------|------|--------|
| 实时协作 | 多用户同时编辑通知 | P0 |
| 锁定机制 | 编辑时文件锁定 | P1 |
| 变更通知 | WebSocket推送变更 | P0 |

### 3.2 版本控制

| 特性 | 描述 | 优先级 |
|------|------|--------|
| 版本历史 | 保留30天版本历史 | P0 |
| 版本对比 | diff显示变更 | P1 |
| 版本恢复 | 恢复到任意版本 | P0 |

---

## 4. 技术架构

```
┌──────────────────────────────────────────────────────┐
│                 Drive Sync Manager                    │
├──────────────────────────────────────────────────────┤
│                                                      │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │ 同步引擎   │  │ 版本管理   │  │ 协作服务   │    │
│  │ (rsync)    │  │ (git-like) │  │ (WebSocket)│    │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘    │
│        │               │               │           │
│        └───────────────┴───────────────┘           │
│                        │                            │
│             ┌──────────┴──────────┐                │
│             │   文件变更监控       │                │
│             │   (fsnotify)        │                │
│             └─────────────────────┘                │
└──────────────────────────────────────────────────────┘
```

---

## 5. API增强

### 5.1 协作接口

```go
// WebSocket /api/v1/drive/sync/ws
type SyncEvent struct {
    Type      string `json:"type"` // created/modified/deleted/locked
    Path      string `json:"path"`
    User      string `json:"user"`
    Timestamp string `json:"timestamp"`
    Version   int    `json:"version"`
}

// POST /api/v1/drive/file/{path}/lock
type LockRequest struct {
    User      string `json:"user"`
    Duration  int    `json:"duration"` // 锁定时长(秒)
}

// GET /api/v1/drive/file/{path}/versions
type VersionList struct {
    Versions []FileVersion `json:"versions"`
}

type FileVersion struct {
    Version    int    `json:"version"`
    Timestamp  string `json:"timestamp"`
    User       string `json:"user"`
    Size       int64  `json:"size"`
    Checksum   string `json:"checksum"`
}
```

---

## 6. 对标群晖Synology Drive

| 功能 | 群晖Drive | nas-os Phase2 | 对标状态 |
|------|-----------|---------------|----------|
| 文件同步 | ✅ | ✅已有 | 🟢持平 |
| 实时协作 | ✅ | ✅本轮增强 | 🟢对标 |
| 版本历史 | ✅ | ✅本轮增强 | 🟢对标 |
| 冲突解决 | ✅ | ✅本轮增强 | 🟢对标 |
| 文件锁定 | ✅ | ✅本轮增强 | 🟢对标 |
| 团队文件夹 | ✅ | 📋下一轮 | 🟡跟进 |

---

## 7. 实现计划

| 阶段 | 时间 | 任务 |
|------|------|------|
| M1 | 第235轮 | 设计文档 |
| M2 | 第236轮 | WebSocket服务 |
| M3 | 第237轮 | 版本管理模块 |
| M4 | 第238轮 | 协作API |
| M5 | 第239轮 | UI集成 |

---

**礼部签名**: 礼部
**提交时间**: 2026-04-24