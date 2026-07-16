# 存储 API 文档

**版本**: v3.24.3  
**更新日期**: 2026-07-16

---

## 概述

存储 API 提供卷、子卷、快照管理。自 **v3.24** 起仅保留 **单一契约**，无 `/api/v1/volumes` 兼容层。

## 基础路径

```
/api/v1/storage
```

---

## 卷管理

### 获取卷列表

```http
GET /api/v1/storage/volumes
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "name": "data",
      "uuid": "...",
      "devices": ["/dev/sda"],
      "size": 1000000000000,
      "used": 350000000000,
      "free": 650000000000,
      "dataProfile": "raid1",
      "mountPoint": "/mnt/data",
      "status": { "healthy": true }
    }
  ]
}
```

### 创建卷

```http
POST /api/v1/storage/volumes
```

**请求体**（`storage.CreateVolumeRequest`）

```json
{
  "name": "data",
  "devices": ["/dev/sda", "/dev/sdb"],
  "profile": "raid1"
}
```

### 删除卷

```http
DELETE /api/v1/storage/volumes/:name?force=true
```

### 数据校验 / 平衡

```http
POST /api/v1/storage/volumes/:name/scrub
POST /api/v1/storage/volumes/:name/balance
```

---

## 子卷

```http
GET  /api/v1/storage/volumes/:name/subvolumes
POST /api/v1/storage/volumes/:name/subvolumes
DELETE /api/v1/storage/volumes/:name/subvolumes/:subvol
POST /api/v1/storage/volumes/:name/subvolumes/:subvol/mount
```

**创建子卷请求体**（`storage.CreateSubvolumeRequest`）

```json
{ "name": "@home", "path": "" }
```

**挂载请求体**（`storage.MountSubvolumeRequest`）

```json
{ "mountPath": "/mnt/home" }
```

---

## 快照

```http
GET  /api/v1/storage/snapshots
GET  /api/v1/storage/volumes/:name/snapshots
POST /api/v1/storage/volumes/:name/snapshots
DELETE /api/v1/storage/volumes/:name/snapshots/:snap
POST /api/v1/storage/volumes/:name/snapshots/:snap/restore
```

**创建快照**（`storage.CreateSnapshotRequest`）

```json
{ "subvolume": "@home", "name": "manual-1", "readOnly": true }
```

**恢复快照**（`storage.RestoreSnapshotRequest`）

```json
{ "targetName": "manual-1-restored" }
```

---

## 存储池

```http
GET /api/v1/storage/pools
```

---

## 已删除（破坏性）

| 路径 | 状态 |
|------|------|
| `/api/v1/volumes/*` | **已删除**，请改用 `/api/v1/storage/*` |
