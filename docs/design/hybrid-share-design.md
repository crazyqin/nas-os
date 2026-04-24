# Hybrid Share 设计文档

**版本**: v2.463.0
**部门**: 工部
**日期**: 2026-04-24

---

## 1. 概述

Hybrid Share对标群晖DSM Hybrid Share功能，实现本地存储与云存储的混合管理，提供：
- 本地优先、云端备份
- 自动同步策略
- 智能缓存机制

---

## 2. 功能设计

### 2.1 核心特性

| 特性 | 描述 | 优先级 |
|------|------|--------|
| 本地缓存 | 高频文件本地优先 | P0 |
| 云端备份 | 自动上传到云存储 | P1 |
| 智能同步 | 根据访问频率决定同步策略 | P1 |
| 空间管理 | 本地缓存空间自动清理 | P2 |

---

## 2. 技术架构

```
┌──────────────────────────────────────────────────────┐
│                    Hybrid Share Manager               │
├──────────────────────────────────────────────────────┤
│                                                      │
│   ┌──────────────┐    ┌──────────────┐              │
│   │  本地存储层   │    │  云端存储层   │              │
│   │  (btrfs/ZFS) │    │ (S3/OSS/GCS) │              │
│   └──────┬───────┘    └──────┬───────┘              │
│          │                    │                      │
│          └────────────────────┘                      │
│                    │                                  │
│         ┌──────────┴──────────┐                      │
│         │   混合存储策略引擎    │                      │
│         └─────────────────────┘                      │
│                    │                                  │
│         ┌──────────┴──────────┐                      │
│         │    智能缓存管理器     │                      │
│         └─────────────────────┘                      │
└──────────────────────────────────────────────────────┘
```

---

## 3. 存储策略

### 3.1 文件分类

| 类别 | 存放位置 | 同步策略 |
|------|----------|----------|
| 高频访问 | 本地+云端 | 实时同步 |
| 中频访问 | 本地缓存 | 定期同步 |
| 低频访问 | 仅云端 | 按需下载 |
| 冷数据 | 仅云端 | 无缓存 |

### 3.2 缓存策略

```go
type CachePolicy struct {
    MaxLocalSize    int64   // 最大本地缓存大小
    EvictionPolicy  string  // 清理策略 (LRU/LFU)
    HotThreshold    int     // 高频阈值(访问次数)
    SyncInterval    int     // 同步间隔(分钟)
}
```

---

## 4. API设计

```go
// POST /api/v1/hybrid/create
type HybridShareRequest struct {
    Name          string   `json:"name"`
    LocalPath     string   `json:"local_path"`
    CloudProvider string   `json:"cloud_provider"` // s3/oss/gcs
    CloudBucket   string   `json:"cloud_bucket"`
    Policy        CachePolicy `json:"policy"`
}

// GET /api/v1/hybrid/{name}/status
type HybridShareStatus struct {
    LocalUsage    int64   `json:"local_usage"`
    CloudUsage    int64   `json:"cloud_usage"`
    CachedFiles   int     `json:"cached_files"`
    CloudFiles    int     `json:"cloud_files"`
    SyncPending   int     `json:"sync_pending"`
}
```

---

## 5. 对标群晖Hybrid Share

| 功能 | 群晖 | nas-os设计 | 对标状态 |
|------|------|------------|----------|
| 本地缓存 | ✅ | ✅设计完成 | 🟢对标 |
| 多云支持 | 🟡有限 | ✅6+平台 | 🟢领先 |
| 智能策略 | ✅ | ✅设计完成 | 🟢对标 |
| 空间管理 | ✅ | ✅LRU/LFU | 🟢对标 |
| 国内云 | ❌ | ✅阿里/腾讯 | 🟢领先 |

---

## 6. 实现计划

| 阶段 | 时间 | 任务 |
|------|------|------|
| M1 | 第235轮 | 架构设计文档 |
| M2 | 第240轮 | 存储策略引擎 |
| M3 | 第245轮 | 缓存管理器 |
| M4 | 第250轮 | API实现 |
| M5 | 第255轮 | 测试发布 |

---

**工部签名**: 工部
**提交时间**: 2026-04-24