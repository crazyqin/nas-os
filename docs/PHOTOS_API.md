# Photos API 文档

**版本**: v2.25.0  
**基础路径**: `/api/v1/photos`

---

## 概述

NAS-OS 相册模块提供完整的照片管理功能，包括：
- 📸 照片上传（单张/批量/分片）
- 🖼️ 缩略图自动生成
- 📁 相册管理
- ⏰ 时间线视图
- 👤 人物识别
- 🤖 AI 智能分类与分析
- 🔍 智能搜索

---

## 照片上传

### 单张上传

```bash
curl -X POST http://localhost:8080/api/v1/photos/upload \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@/path/to/photo.jpg"
```

**响应**:
```json
{
  "code": 0,
  "message": "上传成功",
  "data": {
    "photoId": "550e8400-e29b-41d4-a716-446655440000",
    "filename": "photo.jpg",
    "size": 1048576
  }
}
```

### 批量上传

```bash
curl -X POST http://localhost:8080/api/v1/photos/upload/batch \
  -H "Authorization: Bearer TOKEN" \
  -F "files=@/path/to/photo1.jpg" \
  -F "files=@/path/to/photo2.jpg"
```

**响应**:
```json
{
  "code": 0,
  "message": "批量上传完成",
  "data": {
    "uploaded": [
      {"photoId": "...", "filename": "photo1.jpg", "size": 1048576}
    ],
    "failed": [],
    "total": 2,
    "success": 2,
    "errors": 0
  }
}
```

### 分片上传（大文件）

适用于超过 500MB 的大文件或需要断点续传的场景。

#### 1. 创建上传会话

```bash
curl -X POST http://localhost:8080/api/v1/photos/upload/session \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "large_video.mp4",
    "totalSize": 524288000,
    "chunkSize": 10485760
  }'
```

**响应**:
```json
{
  "code": 0,
  "message": "上传会话创建成功",
  "data": {
    "sessionId": "session-uuid",
    "filename": "large_video.mp4",
    "totalSize": 524288000,
    "chunkSize": 10485760,
    "totalChunks": 50,
    "uploadedChunks": [],
    "expiresAt": "2026-03-15T10:00:00Z"
  }
}
```

#### 2. 上传分片

```bash
curl -X PUT "http://localhost:8080/api/v1/photos/upload/session/session-uuid?chunk=0" \
  -H "Authorization: Bearer TOKEN" \
  -F "chunk=@chunk_0.dat"
```

#### 3. 完成上传

```bash
curl -X POST http://localhost:8080/api/v1/photos/upload/session/session-uuid/complete \
  -H "Authorization: Bearer TOKEN"
```

---

## 照片管理

### 列出照片

```bash
curl "http://localhost:8080/api/v1/photos?page=1&limit=50&album=album-id" \
  -H "Authorization: Bearer TOKEN"
```

**查询参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 页码（默认 1） |
| limit | int | 每页数量（默认 50） |
| album | string | 相册 ID 过滤 |
| favorite | bool | 仅收藏 |
| person | string | 人物 ID 过滤 |

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "photos": [
      {
        "id": "photo-uuid",
        "filename": "IMG_001.jpg",
        "path": "/data/photos/IMG_001.jpg",
        "size": 1048576,
        "width": 1920,
        "height": 1080,
        "takenAt": "2026-03-14T10:00:00Z",
        "createdAt": "2026-03-14T12:00:00Z",
        "favorite": false,
        "thumbnailId": "thumb-uuid"
      }
    ],
    "total": 1250,
    "page": 1,
    "limit": 50
  }
}
```

### 获取照片详情

```bash
curl http://localhost:8080/api/v1/photos/photo-uuid \
  -H "Authorization: Bearer TOKEN"
```

### 更新照片信息

```bash
curl -X PUT http://localhost:8080/api/v1/photos/photo-uuid \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "美丽的日落",
    "description": "在海边拍的日落照片",
    "tags": ["日落", "海边", "旅行"]
  }'
```

### 删除照片

```bash
curl -X DELETE http://localhost:8080/api/v1/photos/photo-uuid \
  -H "Authorization: Bearer TOKEN"
```

### 收藏/取消收藏

```bash
curl -X POST http://localhost:8080/api/v1/photos/photo-uuid/favorite \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"favorite": true}'
```

### 下载照片

```bash
curl http://localhost:8080/api/v1/photos/photo-uuid/download \
  -H "Authorization: Bearer TOKEN" \
  -o photo.jpg
```

---

## 缩略图

### 获取缩略图

```bash
# 默认尺寸（512px）
curl http://localhost:8080/api/v1/photos/photo-uuid/thumbnail \
  -H "Authorization: Bearer TOKEN"

# 指定尺寸: small(128), medium(512), large(1024)
curl http://localhost:8080/api/v1/photos/photo-uuid/thumbnail/large \
  -H "Authorization: Bearer TOKEN"
```

---

## 相册管理

### 列出相册

```bash
curl http://localhost:8080/api/v1/photos/albums \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "album-uuid",
      "name": "2026年旅行",
      "description": "年度旅行照片",
      "coverPhotoId": "photo-uuid",
      "photoCount": 156,
      "createdAt": "2026-03-01T00:00:00Z"
    }
  ]
}
```

### 创建相册

```bash
curl -X POST http://localhost:8080/api/v1/photos/albums \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "家庭聚会",
    "description": "周末家庭聚会照片",
    "coverPhotoId": "photo-uuid"
  }'
```

### 获取相册详情

```bash
curl http://localhost:8080/api/v1/photos/albums/album-uuid \
  -H "Authorization: Bearer TOKEN"
```

### 更新相册

```bash
curl -X PUT http://localhost:8080/api/v1/photos/albums/album-uuid \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "2026年家庭聚会",
    "description": "更新后的描述"
  }'
```

### 删除相册

```bash
curl -X DELETE http://localhost:8080/api/v1/photos/albums/album-uuid \
  -H "Authorization: Bearer TOKEN"
```

### 添加照片到相册

```bash
curl -X POST http://localhost:8080/api/v1/photos/albums/album-uuid/photos \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photoIds": ["photo-uuid-1", "photo-uuid-2"]
  }'
```

### 从相册移除照片

```bash
curl -X DELETE http://localhost:8080/api/v1/photos/albums/album-uuid/photos/photo-uuid \
  -H "Authorization: Bearer TOKEN"
```

---

## 时间线

### 获取时间线

按日期分组显示照片。

```bash
curl "http://localhost:8080/api/v1/photos/timeline?year=2026&month=3" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "2026-03-14": [
      {"id": "photo-1", "filename": "IMG_001.jpg", "...": "..."}
    ],
    "2026-03-13": [
      {"id": "photo-2", "filename": "IMG_002.jpg", "...": "..."}
    ]
  }
}
```

---

## 人物管理

### 列出人物

```bash
curl http://localhost:8080/api/v1/photos/persons \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "person-uuid",
      "name": "张三",
      "faceCount": 125,
      "coverFaceId": "face-uuid",
      "createdAt": "2026-03-01T00:00:00Z"
    }
  ]
}
```

### 创建人物

```bash
curl -X POST http://localhost:8080/api/v1/photos/persons \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "李四",
    "faceIds": ["face-uuid-1", "face-uuid-2"]
  }'
```

### 更新人物

```bash
curl -X PUT http://localhost:8080/api/v1/photos/persons/person-uuid \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "李四（同事）"}'
```

### 删除人物

```bash
curl -X DELETE http://localhost:8080/api/v1/photos/persons/person-uuid \
  -H "Authorization: Bearer TOKEN"
```

---

## AI 功能 🤖

### AI 分析统计

```bash
curl http://localhost:8080/api/v1/photos/ai/stats \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "totalPhotos": 5000,
    "analyzedPhotos": 4850,
    "pendingPhotos": 150,
    "facesDetected": 12500,
    "personsIdentified": 45,
    "objectsDetected": 25000,
    "scenesClassified": 4850
  }
}
```

### AI 任务列表

```bash
curl http://localhost:8080/api/v1/photos/ai/tasks \
  -H "Authorization: Bearer TOKEN"
```

### 分析单张照片

```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/analyze/photo-uuid \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "photoId": "photo-uuid",
    "faces": [
      {"id": "face-1", "personId": "person-uuid", "confidence": 0.95}
    ],
    "objects": ["猫", "沙发", "窗户"],
    "scene": "室内",
    "tags": ["宠物", "居家"],
    "quality": {
      "score": 85,
      "blur": false,
      "brightness": "normal"
    }
  }
}
```

### 批量分析

```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/analyze/batch \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photoIds": ["photo-1", "photo-2", "photo-3"]
  }'
```

### 智能相册

系统自动创建的智能相册：

```bash
curl http://localhost:8080/api/v1/photos/ai/smart-albums \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "smart-album-uuid",
      "name": "宠物",
      "type": "auto",
      "rule": {"object": "猫"},
      "photoCount": 125,
      "coverPhotoId": "photo-uuid"
    },
    {
      "id": "smart-album-uuid-2",
      "name": "风景",
      "type": "auto",
      "rule": {"scene": "自然风光"},
      "photoCount": 89,
      "coverPhotoId": "photo-uuid"
    }
  ]
}
```

### 创建智能相册

```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/smart-albums \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "海边度假",
    "rule": {
      "tags": ["海边", "沙滩"],
      "dateRange": {
        "start": "2026-03-01",
        "end": "2026-03-15"
      }
    }
  }'
```

### 回忆功能

系统自动生成的"回忆"（类似 Google Photos）：

```bash
curl http://localhost:8080/api/v1/photos/ai/memories \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "memory-uuid",
      "title": "一年前的今天",
      "type": "on_this_day",
      "date": "2025-03-14",
      "photos": ["photo-1", "photo-2"],
      "coverPhotoId": "photo-1"
    },
    {
      "id": "memory-uuid-2",
      "title": "最佳瞬间",
      "type": "highlights",
      "photos": ["photo-3", "photo-4"]
    }
  ]
}
```

### 重新分析全部

```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/reanalyze \
  -H "Authorization: Bearer TOKEN"
```

### 清除 AI 数据

```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/clear \
  -H "Authorization: Bearer TOKEN"
```

---

## 智能搜索

### 搜索照片

```bash
curl "http://localhost:8080/api/v1/photos/search?q=日落&tags=海边&date=2026-03" \
  -H "Authorization: Bearer TOKEN"
```

**查询参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| q | string | 关键词搜索（支持语义） |
| tags | string | 标签过滤（逗号分隔） |
| date | string | 日期过滤（YYYY-MM 或 YYYY-MM-DD） |
| person | string | 人物 ID |
| album | string | 相册 ID |
| min_width | int | 最小宽度 |
| min_height | int | 最小高度 |

**响应**:
```json
{
  "code": 0,
  "data": {
    "photos": [
      {"id": "photo-uuid", "...": "..."}
    ],
    "total": 45,
    "suggestions": ["日落", "海边日落", "日落风景"]
  }
}
```

---

## 统计

### 获取相册统计

```bash
curl http://localhost:8080/api/v1/photos/stats \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "totalPhotos": 5000,
    "totalSize": 53687091200,
    "totalSizeHuman": "50 GB",
    "albums": 25,
    "persons": 45,
    "favorites": 125,
    "byMonth": {
      "2026-03": 450,
      "2026-02": 380
    },
    "byFormat": {
      "jpg": 4500,
      "png": 300,
      "heic": 200
    }
  }
}
```

---

## 支持的格式

| 格式 | 扩展名 | 说明 |
|------|--------|------|
| JPEG | .jpg, .jpeg | 标准照片格式 |
| PNG | .png | 无损压缩 |
| HEIC | .heic | Apple 设备格式（需 ffmpeg） |
| WebP | .webp | Google 格式 |
| RAW | .raw, .cr2, .nef | 专业相机格式（需 ffmpeg） |
| GIF | .gif | 动图 |
| BMP | .bmp | 位图 |

---

## 配置说明

### 环境变量

```bash
# 照片存储目录
NAS_PHOTOS_DIR=/var/lib/nas-os/photos

# 缓存目录
NAS_PHOTOS_CACHE=/var/lib/nas-os/photos/cache

# 最大上传大小（字节）
NAS_PHOTOS_MAX_UPLOAD=524288000

# AI 功能开关
NAS_PHOTOS_AI_ENABLED=true
```

### 配置文件

`/var/lib/nas-os/photos/photos-config.json`:

```json
{
  "maxUploadSize": 524288000,
  "supportedFormats": [".jpg", ".jpeg", ".png", ".heic", ".webp"],
  "thumbnailConfig": {
    "smallSize": 128,
    "mediumSize": 512,
    "largeSize": 1024,
    "quality": 85
  },
  "aiConfig": {
    "enabled": true,
    "faceDetection": true,
    "objectDetection": true,
    "sceneClassification": true
  }
}
```

---

## 移动端备份

### iOS 快捷指令

创建自动备份快捷指令：

1. 打开「快捷指令」App
2. 创建新快捷指令
3. 添加操作：「获取最近的相片」
4. 添加操作：`URL` → `http://nas-ip:8080/api/v1/photos/upload/batch`
5. 添加操作：「上传文件到 URL」
6. 设置自动化：每天运行

### Android (Tasker)

```
触发器：充电时 + 连接到 WiFi
动作：HTTP POST
URL: http://nas-ip:8080/api/v1/photos/upload/batch
文件：/sdcard/DCIM/Camera/
```

---

## 错误处理

### 错误响应格式

```json
{
  "code": 400,
  "message": "文件大小超过限制（最大 500MB）",
  "data": null
}
```

### 常见错误码

| 代码 | 说明 |
|------|------|
| 400 | 请求参数错误（文件过大、格式不支持等） |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 照片/相册不存在 |
| 413 | 文件大小超过限制 |
| 500 | 服务器内部错误 |

---

**最后更新**: 2026-03-14