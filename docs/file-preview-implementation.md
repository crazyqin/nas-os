# 文件预览功能实现报告

## 完成内容

### 1. 后端 API 实现 (`internal/files/manager.go`)

**文件类型支持：**
- 图片: jpg, jpeg, png, gif, webp, bmp, svg, ico, tiff, heic, heif
- 视频: mp4, mkv, avi, mov, wmv, flv, webm, m4v, mpeg, mpg, 3gp
- 音频: mp3, wav, flac, aac, ogg, wma, m4a, ape
- 文档: pdf, doc, docx, xls, xlsx, ppt, pptx, txt, rtf, odt, ods, odp
- 代码: js, ts, py, go, java, c, cpp, html, css, json, xml, yaml, md, sh, sql, php, rb, rs
- 压缩包: zip, rar, 7z, tar, gz

**API 端点：**
- `GET /api/v1/files/list` - 列出目录文件（支持缩略图生成）
- `GET /api/v1/files/preview` - 文件预览（流式返回）
- `GET /api/v1/files/thumbnail` - 获取缩略图
- `GET /api/v1/files/download` - 下载文件
- `POST /api/v1/files/upload` - 上传文件
- `POST /api/v1/files/mkdir` - 创建目录
- `DELETE /api/v1/files/delete` - 删除文件
- `GET /api/v1/files/info` - 获取文件详情

### 2. 缩略图生成

- **图片缩略图**：使用 nfnt/resize 库，支持 Lanczos3 高质量缩放
- **视频缩略图**：使用 ffmpeg 提取第1秒帧（需系统安装 ffmpeg）
- **缓存机制**：内存缓存，支持配置过期时间
- **尺寸配置**：默认 256px，可自定义

### 3. 前端预览组件 (`webui/pages/files.html`)

**预览功能：**
- 图片预览（支持缩放）
- 视频播放（HTML5 原生播放器，支持缩略图封面）
- 音频播放（自定义播放器 UI）
- PDF 预览（浏览器原生 iframe 支持）
- 文档/代码预览（语法高亮容器）

**交互功能：**
- 左右箭头键导航（多文件预览）
- ESC 键关闭
- 全屏模式
- 文件下载
- 文件信息显示（名称、大小、日期、尺寸/时长）

### 4. 媒体播放器集成

- **视频播放器**：HTML5 `<video>` 标签，支持控制条、预加载、封面图
- **音频播放器**：自定义 UI，显示封面和文件信息
- **键盘控制**：空格暂停/播放，方向键快进/快退

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (files.html)                   │
├─────────────────────────────────────────────────────────────┤
│  FilePreviewer Class                                         │
│  ├── renderImagePreview()    - 图片预览                      │
│  ├── renderVideoPreview()    - 视频播放                      │
│  ├── renderAudioPreview()    - 音频播放                      │
│  ├── renderPDFPreview()      - PDF 查看                      │
│  ├── renderCodePreview()     - 代码预览                      │
│  └── renderDocumentPreview() - 文档预览                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Backend API (manager.go)                  │
├─────────────────────────────────────────────────────────────┤
│  Manager                                                      │
│  ├── ListFiles()           - 列出文件                        │
│  ├── PreviewFile()         - 文件流式返回                    │
│  ├── GenerateImageThumbnail() - 图片缩略图                   │
│  ├── GenerateVideoThumbnail() - 视频缩略图                   │
│  ├── GetVideoInfo()        - 视频元数据                      │
│  └── GetFileContent()      - 文本内容读取                    │
└─────────────────────────────────────────────────────────────┘
```

## 配置选项

```go
PreviewConfig{
    ThumbnailSize:    256,              // 缩略图尺寸
    MaxPreviewSize:   50 * 1024 * 1024, // 最大预览文件大小
    CacheDir:         "/var/cache/nas-os/thumbnails", // 缓存目录
    CacheExpiry:      24 * time.Hour,   // 缓存过期时间
    EnableVideoThumb: true,             // 启用视频缩略图
    EnableDocPreview: true,             // 启用文档预览
}
```

## 依赖项

- **后端**:
  - `github.com/nfnt/resize` - 图片缩放
  - `ffmpeg` (系统级) - 视频缩略图生成

- **前端**: 
  - 原生 HTML5 Media API
  - 无需额外 JavaScript 库

## 安全考虑

1. **路径遍历防护**：检查 `..` 防止目录遍历攻击
2. **文件大小限制**：防止大文件导致内存溢出
3. **MIME 类型验证**：正确设置 Content-Type

## 后续优化建议

1. **文档预览增强**：集成 LibreOffice 或 OnlyOffice 实现在线编辑
2. **PDF.js 集成**：更好的 PDF 查看体验
3. **语法高亮**：集成 highlight.js 或 prism.js
4. **缩略图持久化**：使用 Redis 或文件系统缓存
5. **大文件分片预览**：支持大文件流式预览
6. **3D 文件预览**：支持 STL、OBJ 等格式