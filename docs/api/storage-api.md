# 存储 API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

---

## 概述

存储 API 提供卷管理、子卷管理、快照管理等功能。

## 基础路径

```
/api/v1/volumes
```

---

## 卷管理

### 获取卷列表

```http
GET /api/v1/volumes
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "volumes": [
      {
        "name": "data",
        "path": "/data",
        "fs_type": "btrfs",
        "size_gb": 1000,
        "used_gb": 350,
        "available_gb": 650,
        "usage_percent": 35
      }
    ]
  }
}
```

### 创建卷

```http
POST /api/v1/volumes
```

**请求体**

```json
{
  "name": "data",
  "path": "/dev/sda1",
  "raid_level": "single",
  "options": {
    "compression": "zstd"
  }
}
```

### 获取卷详情

```http
GET /api/v1/volumes/:name
```

### 删除卷

```http
DELETE /api/v1/volumes/:name
```

---

## 子卷管理

### 创建子卷

```http
POST /api/v1/volumes/:name/subvolumes
```

**请求体**

```json
{
  "subvolume_name": "photos",
  "path": "/data/photos",
  "options": {
    "compression": "zstd"
  }
}
```

### 获取子卷列表

```http
GET /api/v1/volumes/:name/subvolumes
```

### 删除子卷

```http
DELETE /api/v1/volumes/:name/subvolumes/:subvol_name
```

---

## 快照管理

### 创建快照

```http
POST /api/v1/volumes/:name/snapshots
```

**请求体**

```json
{
  "snapshot_name": "daily-2026-03-15",
  "source_path": "/data/photos",
  "readonly": true
}
```

### 获取快照列表

```http
GET /api/v1/volumes/:name/snapshots
```

### 删除快照

```http
DELETE /api/v1/volumes/:name/snapshots/:snapshot_name
```

### 恢复快照

```http
POST /api/v1/volumes/:name/snapshots/:snapshot_name/restore
```

---

## 数据平衡

### 启动数据平衡

```http
POST /api/v1/volumes/:name/balance
```

**请求体**

```json
{
  "force": false
}
```

### 获取平衡状态

```http
GET /api/v1/volumes/:name/balance/status
```

---

## 数据校验

### 启动数据校验

```http
POST /api/v1/volumes/:name/scrub
```

### 获取校验状态

```http
GET /api/v1/volumes/:name/scrub/status
```

---

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 404 | 卷不存在 |
| 409 | 卷已存在 |
| 500 | 服务器内部错误 |

---

## 相关文档

- [管理员指南](../ADMIN_GUIDE_v2.5.0.md) - 存储配置
- [快照策略指南](../SNAPSHOT_POLICY_GUIDE.md) - 自动快照配置

---

*最后更新：2026-03-15*