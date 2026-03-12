# 相册模块 (Photos)

NAS-OS 相册功能模块，提供照片备份、管理、浏览和分享功能。

## 功能特性

### 核心功能
- ✅ **照片上传**: 支持单张/批量上传，断点续传
- ✅ **相册管理**: 创建、编辑、删除、共享相册
- ✅ **EXIF 元数据**: 自动提取相机信息、拍摄时间、GPS 位置
- ✅ **缩略图生成**: 多尺寸缩略图（128/512/1024）
- ✅ **时间线浏览**: 按年/月/日分组查看照片
- ✅ **人物管理**: 人脸识别和管理（待实现 AI）
- ✅ **收藏功能**: 标记喜欢的照片
- ✅ **格式支持**: JPG/PNG/HEIC/RAW/视频

### 待实现功能
- 🔄 **AI 分类**: 场景识别、物体识别
- 🔄 **人脸识别**: 自动聚合同一人物
- 🔄 **照片编辑**: 裁剪、旋转、滤镜
- 🔄 **分享链接**: 生成带密码的分享链接
- 🔄 **手机备份**: iOS/Android/鸿蒙自动备份

## API 接口

### 照片管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/photos/upload | 上传单张照片 |
| POST | /api/v1/photos/upload/batch | 批量上传照片 |
| GET | /api/v1/photos | 列出照片 |
| GET | /api/v1/photos/:id | 获取照片详情 |
| DELETE | /api/v1/photos/:id | 删除照片 |
| POST | /api/v1/photos/:id/favorite | 切换收藏 |
| PUT | /api/v1/photos/:id | 更新照片信息 |
| GET | /api/v1/photos/:id/download | 下载照片 |
| GET | /api/v1/photos/:id/thumbnail/:size | 获取缩略图 |

### 相册管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/photos/albums | 列出相册 |
| POST | /api/v1/photos/albums | 创建相册 |
| GET | /api/v1/photos/albums/:id | 获取相册详情 |
| PUT | /api/v1/photos/albums/:id | 更新相册 |
| DELETE | /api/v1/photos/albums/:id | 删除相册 |
| POST | /api/v1/photos/albums/:id/photos | 添加照片到相册 |
| DELETE | /api/v1/photos/albums/:id/photos/:photoId | 从相册移除照片 |

### 时间线

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/photos/timeline | 获取时间线 |

### 人物管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/photos/persons | 列出人物 |
| POST | /api/v1/photos/persons | 创建人物 |
| PUT | /api/v1/photos/persons/:id | 更新人物 |
| DELETE | /api/v1/photos/persons/:id | 删除人物 |

### 其他

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/photos/search | 搜索照片 |
| GET | /api/v1/photos/stats | 获取统计信息 |

## 使用示例

### 上传照片

```bash
# 单张上传
curl -X POST http://localhost:8080/api/v1/photos/upload \
  -F "file=@photo.jpg"

# 批量上传
curl -X POST http://localhost:8080/api/v1/photos/upload/batch \
  -F "files=@photo1.jpg" \
  -F "files=@photo2.jpg" \
  -F "files=@photo3.jpg"
```

### 列出照片

```bash
# 获取全部照片
curl http://localhost:8080/api/v1/photos

# 获取相册中的照片
curl http://localhost:8080/api/v1/photos?albumId=xxx

# 获取收藏的照片
curl http://localhost:8080/api/v1/photos?favorite=true

# 分页获取
curl http://localhost:8080/api/v1/photos?limit=50&offset=0
```

### 创建相册

```bash
curl -X POST http://localhost:8080/api/v1/photos/albums \
  -H "Content-Type: application/json" \
  -d '{
    "name": "旅行照片",
    "description": "2026 年旅行照片"
  }'
```

### 获取时间线

```bash
# 按月分组
curl http://localhost:8080/api/v1/photos/timeline?groupBy=month

# 按年分组
curl http://localhost:8080/api/v1/photos/timeline?groupBy=year

# 按日分组
curl http://localhost:8080/api/v1/photos/timeline?groupBy=day
```

## 数据存储

### 目录结构

```
/var/lib/nas-os/photos/
├── photos/              # 原始照片
│   ├── abc123.jpg
│   ├── def456.png
│   └── ...
├── thumbnails/          # 缩略图
│   ├── abc123_128.jpg
│   ├── abc123_512.jpg
│   ├── abc123_1024.jpg
│   └── ...
├── cache/              # 缓存文件
│   └── uploads/        # 上传临时文件
├── albums.json         # 相册数据
├── persons.json        # 人物数据
└── photos-config.json  # 配置文件
```

### 配置文件

```json
{
  "enableAI": true,
  "enableFaceRec": true,
  "enableObjectRec": true,
  "autoBackup": false,
  "backupPaths": [],
  "thumbnailConfig": {
    "smallSize": 128,
    "mediumSize": 512,
    "largeSize": 1024,
    "originalMax": 2048,
    "quality": 85
  },
  "supportedFormats": [
    ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp",
    ".heic", ".heif", ".raw", ".dng", ".cr2", ".nef", ".arw",
    ".mp4", ".mov", ".avi", ".mkv", ".webm"
  ],
  "maxUploadSize": 524288000
}
```

## Web UI

访问 `http://localhost:8080/photos/` 使用相册 Web 界面。

### 功能
- 📷 照片网格浏览
- 📁 相册管理
- ❤️ 收藏管理
- ⬆️ 拖拽上传
- 📱 响应式设计（支持移动端）
- 🖼️ 照片查看器

## 依赖

- Go 1.26+
- github.com/google/uuid
- github.com/rwcarlsen/goexif/exif
- golang.org/x/image/draw
- ffmpeg（可选，用于 HEIC/RAW/视频缩略图）

## 性能优化

### 缩略图缓存
- 生成后缓存到磁盘
- 支持多尺寸（128/512/1024）
- 懒加载优化

### 批量操作
- 支持批量上传
- 后台索引照片
- 异步 EXIF 提取

### 内存管理
- 流式上传处理
- 大文件分块处理
- 自动垃圾回收

## TODO

### 短期
- [ ] 完善错误处理
- [ ] 添加单元测试
- [ ] 实现搜索功能
- [ ] 添加照片编辑功能

### 中期
- [ ] 集成 AI 分类（场景/物体）
- [ ] 人脸识别和聚类
- [ ] 分享链接功能
- [ ] 手机自动备份

### 长期
- [ ] 智能相册（自动创建）
- [ ] 照片故事（自动生成）
- [ ] 协作相册
- [ ] 第三方集成（Google Photos 导入等）

## 故障排除

### 缩略图生成失败
确保安装了 `ffmpeg` 用于处理 HEIC/RAW 格式：
```bash
sudo apt install ffmpeg
```

### EXIF 提取失败
某些照片可能没有 EXIF 信息，这是正常的。

### 上传失败
检查文件大小是否超过限制（默认 500MB）。

## 贡献

欢迎提交 Issue 和 Pull Request！

## License

MIT
