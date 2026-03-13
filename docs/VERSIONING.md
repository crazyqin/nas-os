# 文件版本控制使用指南

**版本**: v1.8.0  
**更新日期**: 2026-03-20

---

## 📋 目录

1. [概述](#概述)
2. [快速开始](#快速开始)
3. [配置](#配置)
4. [API 参考](#api-参考)
5. [最佳实践](#最佳实践)
6. [常见问题](#常见问题)

---

## 概述

文件版本控制模块为 NAS-OS 提供了企业级的文件历史管理能力。它可以自动保存文件的历史版本，支持版本对比、版本恢复等功能，有效防止数据丢失和误操作。

### 核心功能

| 功能 | 说明 |
|------|------|
| 自动版本快照 | 基于时间间隔或文件变更自动创建版本 |
| 手动版本创建 | 支持用户手动创建重要节点的版本 |
| 版本对比 | 可视化显示两个版本之间的差异 |
| 版本恢复 | 一键将文件恢复到任意历史版本 |
| 版本保留策略 | 按数量、时间或空间限制版本存储 |
| 自动清理 | 自动删除过期的旧版本 |

---

## 快速开始

### 1. 查看文件版本历史

```bash
# 列出文件的所有版本
curl "http://localhost:8080/api/v1/files/data/important.docx?versions=true" \
  -H "Authorization: Bearer TOKEN"
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "ver-20260320-001",
      "path": "/data/important.docx",
      "size": 102400,
      "checksum": "sha256:abc123...",
      "createdAt": "2026-03-20T10:00:00Z",
      "createdBy": "admin",
      "triggerType": "manual",
      "description": "重要修改前备份"
    },
    {
      "id": "ver-20260319-003",
      "path": "/data/important.docx",
      "size": 98304,
      "checksum": "sha256:def456...",
      "createdAt": "2026-03-19T15:30:00Z",
      "triggerType": "auto",
      "description": "自动快照"
    }
  ]
}
```

### 2. 创建文件版本

```bash
# 手动创建版本
curl -X POST "http://localhost:8080/api/v1/files/data/important.docx/versions" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "重要修改前备份",
    "triggerType": "manual"
  }'
```

### 3. 恢复文件版本

```bash
# 恢复到指定版本
curl -X POST "http://localhost:8080/api/v1/versions/ver-20260319-003/restore" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### 4. 版本对比

```bash
# 查看版本与当前文件的差异
curl "http://localhost:8080/api/v1/versions/ver-20260319-003/diff" \
  -H "Authorization: Bearer TOKEN"
```

---

## 配置

### 获取配置

```bash
curl "http://localhost:8080/api/v1/versions/config" \
  -H "Authorization: Bearer TOKEN"
```

### 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | true | 是否启用版本控制 |
| `autoVersion` | bool | true | 是否自动创建版本 |
| `versionInterval` | string | "1h" | 自动版本间隔 |
| `maxVersions` | int | 100 | 最大版本数量 |
| `maxAge` | string | "30d" | 版本最大保留时间 |
| `maxSpace` | int64 | 10737418240 | 版本存储空间上限 (10GB) |
| `excludePatterns` | []string | [] | 排除的文件模式 |
| `includePatterns` | []string | ["*"] | 包含的文件模式 |

### 更新配置

```bash
curl -X PUT "http://localhost:8080/api/v1/versions/config" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "autoVersion": true,
    "versionInterval": "30m",
    "maxVersions": 200,
    "maxAge": "60d",
    "maxSpace": 21474836480,
    "excludePatterns": ["*.tmp", "*.log", "/tmp/*"]
  }'
```

---

## API 参考

### 文件版本操作

#### 列出文件版本

```
GET /api/v1/files/:path?versions=true
```

**参数**:
- `path` - 文件路径（URL 编码）

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "string",
      "path": "string",
      "size": 0,
      "checksum": "string",
      "createdAt": "2026-03-20T00:00:00Z",
      "createdBy": "string",
      "triggerType": "manual|auto|scheduled",
      "description": "string"
    }
  ]
}
```

#### 创建文件版本

```
POST /api/v1/files/:path/versions
```

**请求体**:
```json
{
  "description": "版本描述",
  "triggerType": "manual",
  "userId": "用户ID"
}
```

### 版本操作

#### 获取版本详情

```
GET /api/v1/versions/:id
```

#### 恢复版本

```
POST /api/v1/versions/:id/restore
```

**请求体**:
```json
{
  "targetPath": "/data/restored/file.docx"  // 可选，为空则恢复到原路径
}
```

#### 删除版本

```
DELETE /api/v1/versions/:id
```

#### 版本对比

```
GET /api/v1/versions/:id/diff
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "versionId": "ver-001",
    "currentPath": "/data/file.txt",
    "versionPath": "/data/.versions/ver-001/file.txt",
    "changes": [
      {
        "type": "modified",
        "line": 10,
        "oldContent": "原始内容",
        "newContent": "修改后内容"
      }
    ],
    "addedLines": 5,
    "removedLines": 2,
    "modifiedLines": 3
  }
}
```

### 统计信息

```
GET /api/v1/versions/stats
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "totalVersions": 1234,
    "totalSize": 1073741824,
    "totalSizeHuman": "1.0 GB",
    "versionedFiles": 567,
    "oldestVersion": "2026-01-01T00:00:00Z",
    "newestVersion": "2026-03-20T00:00:00Z"
  }
}
```

---

## 最佳实践

### 1. 版本保留策略

根据数据重要性和存储容量设置合理的保留策略：

```json
{
  "maxVersions": 100,      // 保留最近 100 个版本
  "maxAge": "30d",         // 保留 30 天内的版本
  "maxSpace": 10737418240  // 最多占用 10GB 空间
}
```

### 2. 排除不必要的文件

避免为临时文件、缓存文件创建版本：

```json
{
  "excludePatterns": [
    "*.tmp",
    "*.log",
    "*.cache",
    "/tmp/*",
    "/cache/*",
    "node_modules/*"
  ]
}
```

### 3. 重要文件手动版本

对于重要文档，建议在关键修改前手动创建版本：

```bash
# 修改前创建版本
curl -X POST "/api/v1/files/data/contract.pdf/versions" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"description": "签署前版本"}'
```

### 4. 定期检查版本统计

```bash
# 查看版本存储使用情况
curl "/api/v1/versions/stats" -H "Authorization: Bearer TOKEN"
```

---

## 常见问题

### Q: 版本控制会占用多少存储空间？

A: 版本存储采用增量存储技术，仅保存文件变更部分。实际占用空间取决于：
- 文件变更频率
- 文件大小变化
- 版本保留策略

通常建议预留总存储空间的 10-20% 用于版本存储。

### Q: 如何批量删除旧版本？

A: 系统会根据配置的策略自动清理过期版本。也可以手动删除：

```bash
# 删除单个版本
curl -X DELETE "/api/v1/versions/ver-001" -H "Authorization: Bearer TOKEN"
```

### Q: 版本恢复会覆盖当前文件吗？

A: 默认情况下，恢复操作会覆盖当前文件。建议先为当前文件创建版本再恢复。

### Q: 支持哪些文件类型？

A: 版本控制支持所有文件类型。文本文件可以进行差异对比，二进制文件仅保存完整副本。

### Q: 如何关闭某个目录的版本控制？

A: 在配置中添加排除模式：

```json
{
  "excludePatterns": ["/data/temp/*"]
}
```

---

## 相关文档

- [云同步配置指南](CLOUDSYNC.md)
- [API 使用指南](API_GUIDE.md)
- [用户手册](USER_MANUAL_v1.0.md)

---

**最后更新**: 2026-03-20  
**维护团队**: NAS-OS 礼部