# 全局搜索使用指南

## 概述

nas-os 提供全局搜索功能，参考 TrueNAS Electric Eel 的设计理念，支持跨多种数据源的统一搜索体验。

## 功能特性

### 搜索范围

全局搜索支持以下数据源：

| 类型 | 说明 | 示例 |
|------|------|------|
| 文件 | 文件系统中的文件和目录 | `readme.md`, `/docs/guide.pdf` |
| 应用 | 已安装的应用和容器 | `docker`, `backup` |
| 设置 | 系统配置项 | `wifi`, `storage`, `network` |
| API | 系统API端点 | `/api/v1/users`, `/api/v1/files` |
| 文档 | 系统文档和帮助 | `如何备份`, `RAID配置` |
| 日志 | 系统日志和服务日志 | `error`, `warning` |

### 搜索语法

基础搜索：
```
GET /api/v1/search?q=docker
```

类型过滤：
```
GET /api/v1/search?q=docker&types=app,file
```

分页：
```
GET /api/v1/search?q=file&limit=10&offset=20
```

排序：
```
GET /api/v1/search?q=backup&sort=modified&desc=true
```

## API 参考

### 全局搜索

```
GET /api/v1/search
```

参数：
- `q` (必填): 搜索查询
- `types` (可选): 类型过滤，逗号分隔
- `limit` (可选): 结果数量限制，默认50
- `offset` (可选): 分页偏移，默认0
- `sort` (可选): 排序字段 (name, score, modified, size)
- `desc` (可选): 降序排序，默认false

响应：
```json
{
  "query": "docker",
  "results": [
    {
      "type": "app",
      "name": "Docker Manager",
      "path": "docker",
      "description": "Container management application",
      "score": 0.8
    },
    {
      "type": "file",
      "name": "docker-compose.yml",
      "path": "/opt/docker-compose.yml",
      "size": 1024,
      "modifiedAt": "2026-03-31T12:00:00Z",
      "score": 0.5
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0
}
```

### 文件索引

```
POST /api/v1/search/index
```

请求体：
```json
{
  "path": "/data/photos/vacation.jpg",
  "name": "vacation.jpg",
  "contentType": "image/jpeg",
  "tags": ["travel", "vacation", "2024"]
}
```

### 批量索引

```
POST /api/v1/search/index/batch
```

请求体：
```json
{
  "files": [
    {"path": "/file1.txt", "name": "file1.txt"},
    {"path": "/file2.pdf", "name": "file2.pdf"}
  ]
}
```

## 使用场景

### 1. 快速定位文件

搜索文档、照片、视频等：
```
搜索: vacation
结果: /photos/2024/vacation.jpg, /docs/travel/vacation_plan.pdf
```

### 2. 查找应用和容器

快速启动或管理应用：
```
搜索: docker
结果: Docker Manager 应用，docker-compose.yml 配置文件
```

### 3. 查找设置项

快速导航到系统设置：
```
搜索: wifi
结果: WiFi Settings -> /settings/network/wifi
```

### 4. 搜索API端点

开发时快速查找API：
```
搜索: backup
结果: /api/v1/backup/start, /api/v1/backup/status
```

## 最佳实践

1. **使用具体关键词**：更精确的查询获得更好的结果
2. **利用类型过滤**：缩小搜索范围提高效率
3. **定期索引**：确保搜索结果准确和最新
4. **合理分页**：大数据量时使用分页避免超载

## 竞品对比

| 功能 | nas-os | TrueNAS | 群晖 DSM |
|------|:------:|:-------:|:--------:|
| 全局搜索 | ✅ | ✅ | ✅ |
| 文件搜索 | ✅ | ✅ | ✅ |
| 设置搜索 | ✅ | ✅ | ❌ |
| 应用搜索 | ✅ | ✅ | ✅ |
| API搜索 | ✅ | ❌ | ❌ |
| 日志搜索 | ✅ | ✅ | ✅ |

## 相关文档

- [API密钥使用指南](./API_KEY_GUIDE.md)
- [文件管理指南](./FILE_MANAGEMENT_GUIDE.md)
- [应用管理指南](./APP_MANAGEMENT_GUIDE.md)
