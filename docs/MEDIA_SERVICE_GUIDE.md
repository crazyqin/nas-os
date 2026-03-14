# 媒体服务模块使用指南

> NAS-OS v2.27.0 新增功能

媒体服务模块提供完整的媒体处理能力，包括流媒体服务、字幕处理、视频转码和缩略图生成等功能。

## 目录

- [流媒体服务](#流媒体服务)
- [字幕处理](#字幕处理)
- [视频转码](#视频转码)
- [缩略图生成](#缩略图生成)
- [API 参考](#api-参考)

---

## 流媒体服务

流媒体服务支持 HLS 和 DASH 两种主流流媒体协议，适用于各种播放场景。

### 支持的协议

| 协议 | 说明 | 适用场景 |
|------|------|----------|
| HLS | HTTP Live Streaming | iOS/macOS 设备、Web 播放器 |
| DASH | Dynamic Adaptive Streaming | 多平台自适应码率播放 |
| 自适应 HLS | 多质量 HLS 流 | 根据网络带宽自动切换画质 |

### 基本配置

```yaml
streaming:
  video_bitrate: "2M"          # 视频比特率
  audio_bitrate: "128k"        # 音频比特率
  resolution: "1280x720"       # 默认分辨率
  framerate: 30                # 帧率
  hls_segment_duration: 6      # HLS 分片时长（秒）
  hls_list_size: 10            # HLS 播放列表分片数量
  hw_accel: "none"             # 硬件加速：none, cuda, nvenc, qsv, vaapi
```

### 硬件加速

支持多种硬件加速方案，显著提升转码性能：

| 类型 | 说明 | 要求 |
|------|------|------|
| `cuda` / `nvenc` | NVIDIA GPU 加速 | NVIDIA 显卡，安装驱动 |
| `qsv` | Intel Quick Sync Video | Intel 核显 |
| `vaapi` | VA-API (Linux) | 支持 VA-API 的显卡 |

### 创建 HLS 流

```bash
# 创建 HLS 流会话
curl -X POST http://localhost:8080/api/v1/media/stream/hls \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "/data/videos/movie.mp4",
    "output_dir": "/data/stream/movie"
  }'

# 响应
{
  "id": "hls_1710123456789",
  "source_path": "/data/videos/movie.mp4",
  "type": "hls",
  "status": "starting",
  "manifest_url": "/stream/hls_1710123456789/stream.m3u8"
}
```

### 创建自适应 HLS 流

自适应流提供多个画质选项，播放器会根据网络状况自动选择：

```bash
# 创建自适应 HLS 流（多质量）
curl -X POST http://localhost:8080/api/v1/media/stream/adaptive-hls \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "/data/videos/movie.mp4",
    "output_dir": "/data/stream/movie-adaptive",
    "qualities": [
      {"quality": "1080p", "resolution": "1920x1080", "bitrate": "5M"},
      {"quality": "720p", "resolution": "1280x720", "bitrate": "2M"},
      {"quality": "480p", "resolution": "854x480", "bitrate": "1M"}
    ]
  }'
```

### 管理流会话

```bash
# 列出所有流会话
curl http://localhost:8080/api/v1/media/stream/sessions

# 获取会话详情
curl http://localhost:8080/api/v1/media/stream/sessions/hls_1710123456789

# 停止流会话
curl -X POST http://localhost:8080/api/v1/media/stream/sessions/hls_1710123456789/stop

# 删除流会话
curl -X DELETE http://localhost:8080/api/v1/media/stream/sessions/hls_1710123456789
```

### 直接流式传输

对于无需转码的场景，支持直接流式传输（支持 Range 请求）：

```bash
# 直接流式传输文件
GET /api/v1/media/stream/file?path=/data/videos/movie.mp4

# 带 Range 请求（断点续传）
GET /api/v1/media/stream/file?path=/data/videos/movie.mp4
Range: bytes=0-1048576
```

### 播放器集成

**HLS 播放示例 (HTML5)**

```html
<video controls>
  <source src="http://nas-ip:8080/stream/hls_xxx/stream.m3u8" type="application/x-mpegURL">
</video>

<!-- 使用 hls.js -->
<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
<video id="video" controls></video>
<script>
  const video = document.getElementById('video');
  const hls = new Hls();
  hls.loadSource('http://nas-ip:8080/stream/hls_xxx/stream.m3u8');
  hls.attachMedia(video);
</script>
```

---

## 字幕处理

字幕模块支持多种常见字幕格式的解析、转换和编辑。

### 支持的字幕格式

| 格式 | 扩展名 | 说明 |
|------|--------|------|
| SRT | `.srt` | SubRip 格式，最通用 |
| VTT | `.vtt` | WebVTT 格式，Web 标准 |
| ASS/SSA | `.ass`, `.ssa` | Advanced SubStation Alpha，支持样式 |

### 解析字幕文件

```bash
# 解析字幕文件
curl http://localhost:8080/api/v1/media/subtitle/parse?path=/data/videos/movie.srt

# 响应
{
  "format": "srt",
  "items": [
    {
      "index": 1,
      "start_time": 1000000000,    // 纳秒
      "end_time": 4000000000,
      "text": "第一句字幕内容"
    }
  ]
}
```

### 字幕格式转换

```bash
# SRT 转 VTT
curl -X POST http://localhost:8080/api/v1/media/subtitle/convert \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/data/videos/movie.srt",
    "output_path": "/data/videos/movie.vtt"
  }'
```

### 合并字幕文件

```bash
# 合并多个字幕文件
curl -X POST http://localhost:8080/api/v1/media/subtitle/merge \
  -H "Content-Type: application/json" \
  -d '{
    "paths": [
      "/data/videos/part1.srt",
      "/data/videos/part2.srt"
    ],
    "output_path": "/data/videos/merged.srt"
  }'
```

### 时间偏移调整

```bash
# 调整字幕时间（偏移 2 秒）
curl -X POST http://localhost:8080/api/v1/media/subtitle/shift \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/videos/movie.srt",
    "offset_seconds": 2.0
  }'
```

### 从视频中提取字幕

```bash
# 从 MKV 文件提取字幕轨道
curl -X POST http://localhost:8080/api/v1/media/subtitle/extract \
  -H "Content-Type: application/json" \
  -d '{
    "video_path": "/data/videos/movie.mkv",
    "output_path": "/data/videos/movie.srt",
    "stream_index": 0
  }'
```

### VTT 字幕样式

WebVTT 支持区域定义和样式：

```vtt
WEBVTT

Region: id=bottom, width=80%, height=10%, viewportanchor=50%,90%

00:00:01.000 --> 00:00:04.000 region:bottom
字幕显示在底部区域

00:00:05.000 --> 00:00:08.000
<b>粗体</b> <i>斜体</i> <u>下划线</u>
```

---

## 视频转码

转码服务基于 FFmpeg，支持多种视频格式转换和压缩。

### 转码配置

```yaml
transcode:
  video_codec: "libx264"      # 视频编码器
  audio_codec: "aac"          # 音频编码器
  video_bitrate: "2M"         # 视频比特率
  audio_bitrate: "128k"       # 音频比特率
  resolution: "1920x1080"     # 目标分辨率
  framerate: 30               # 帧率
  output_format: "mp4"        # 输出格式
  preset: "medium"            # 编码预设
  crf: 23                     # 恒定质量因子 (0-51)
  hw_accel: "none"            # 硬件加速
```

### 编码预设说明

| 预设 | 速度 | 压缩率 | 适用场景 |
|------|------|--------|----------|
| `ultrafast` | 最快 | 较低 | 实时转码、预览 |
| `fast` | 快 | 中等 | 快速处理 |
| `medium` | 中等 | 较好 | 平衡选择（默认） |
| `slow` | 慢 | 优秀 | 存储归档 |
| `veryslow` | 最慢 | 最佳 | 最高质量需求 |

### CRF 质量控制

CRF (Constant Rate Factor) 控制视频质量：

| CRF 值 | 质量 | 文件大小 |
|--------|------|----------|
| 18-22 | 高质量 | 大 |
| 23 | 默认 | 中等 |
| 24-28 | 可接受 | 小 |
| 28+ | 较低 | 最小 |

### 创建转码任务

```bash
# 创建转码任务
curl -X POST http://localhost:8080/api/v1/media/transcode \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/data/videos/source.mkv",
    "output_path": "/data/videos/output.mp4",
    "config": {
      "video_codec": "libx264",
      "audio_codec": "aac",
      "video_bitrate": "2M",
      "audio_bitrate": "128k",
      "resolution": "1280x720",
      "preset": "fast",
      "crf": 23
    }
  }'

# 响应
{
  "id": "transcode_1710123456789",
  "status": "pending",
  "input_path": "/data/videos/source.mkv",
  "output_path": "/data/videos/output.mp4"
}
```

### 启动和监控转码

```bash
# 启动转码任务
curl -X POST http://localhost:8080/api/v1/media/transcode/transcode_1710123456789/start

# 查询转码进度
curl http://localhost:8080/api/v1/media/transcode/transcode_1710123456789

# 响应
{
  "id": "transcode_1710123456789",
  "status": "running",
  "progress": 45.5,
  "current_frame": 1365,
  "total_frames": 3000,
  "speed": "1.5x",
  "eta": "00:02:30"
}

# 取消转码任务
curl -X POST http://localhost:8080/api/v1/media/transcode/transcode_1710123456789/cancel
```

### 快速转换

```bash
# 快速转换为 Web 格式
curl -X POST http://localhost:8080/api/v1/media/transcode/quick \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/data/videos/source.mkv",
    "output_path": "/data/videos/web.mp4",
    "format": "mp4"
  }'

# 优化为 Web 播放格式
curl -X POST http://localhost:8080/api/v1/media/transcode/optimize-web \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/data/videos/source.mkv",
    "output_path": "/data/videos/optimized.mp4"
  }'
```

### 获取视频信息

```bash
# 获取视频详细信息
curl http://localhost:8080/api/v1/media/info?path=/data/videos/movie.mp4

# 响应
{
  "duration": 7200.5,          // 秒
  "width": 1920,
  "height": 1080,
  "framerate": 30.0,
  "video_codec": "h264",
  "audio_codec": "aac",
  "bitrate": 5000000,          // bps
  "size": 4500000000,          // 字节
  "streams": [...]
}
```

---

## 缩略图生成

### 生成视频缩略图

```bash
# 生成单个缩略图
curl -X POST http://localhost:8080/api/v1/media/thumbnail \
  -H "Content-Type: application/json" \
  -d '{
    "video_path": "/data/videos/movie.mp4",
    "output_path": "/data/thumbs/movie.jpg",
    "time_offset": 60          // 时间偏移（秒）
  }'

# 生成多个缩略图（预览图）
curl -X POST http://localhost:8080/api/v1/media/thumbnails \
  -H "Content-Type: application/json" \
  -d '{
    "video_path": "/data/videos/movie.mp4",
    "output_dir": "/data/thumbs/movie",
    "count": 10,                // 缩略图数量
    "width": 320,               // 宽度
    "height": 180               // 高度
  }'
```

### 缩略图拼图（预览条）

```bash
# 生成缩略图拼图
curl -X POST http://localhost:8080/api/v1/media/thumbnail/sprite \
  -H "Content-Type: application/json" \
  -d '{
    "video_path": "/data/videos/movie.mp4",
    "output_path": "/data/thumbs/movie-sprite.jpg",
    "interval": 60,             // 每隔多少秒截取一帧
    "columns": 5,               // 每行列数
    "thumb_width": 160,         // 缩略图宽度
    "thumb_height": 90          // 缩略图高度
  }'
```

---

## API 参考

### 流媒体 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/media/stream/hls` | 创建 HLS 流会话 |
| POST | `/api/v1/media/stream/dash` | 创建 DASH 流会话 |
| POST | `/api/v1/media/stream/adaptive-hls` | 创建自适应 HLS 流 |
| GET | `/api/v1/media/stream/sessions` | 列出所有流会话 |
| GET | `/api/v1/media/stream/sessions/:id` | 获取会话详情 |
| POST | `/api/v1/media/stream/sessions/:id/stop` | 停止流会话 |
| DELETE | `/api/v1/media/stream/sessions/:id` | 删除流会话 |
| GET | `/api/v1/media/stream/file` | 直接流式传输文件 |

### 字幕 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/media/subtitle/parse` | 解析字幕文件 |
| POST | `/api/v1/media/subtitle/convert` | 转换字幕格式 |
| POST | `/api/v1/media/subtitle/merge` | 合并字幕文件 |
| POST | `/api/v1/media/subtitle/shift` | 调整时间偏移 |
| POST | `/api/v1/media/subtitle/extract` | 从视频提取字幕 |

### 转码 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/media/transcode` | 创建转码任务 |
| POST | `/api/v1/media/transcode/:id/start` | 启动转码任务 |
| GET | `/api/v1/media/transcode/:id` | 查询转码进度 |
| POST | `/api/v1/media/transcode/:id/cancel` | 取消转码任务 |
| DELETE | `/api/v1/media/transcode/:id` | 删除转码任务 |
| POST | `/api/v1/media/transcode/quick` | 快速转换 |
| POST | `/api/v1/media/transcode/optimize-web` | Web 优化 |
| GET | `/api/v1/media/info` | 获取视频信息 |

### 缩略图 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/media/thumbnail` | 生成单个缩略图 |
| POST | `/api/v1/media/thumbnails` | 生成多个缩略图 |
| POST | `/api/v1/media/thumbnail/sprite` | 生成缩略图拼图 |

---

## 常见问题

### Q: 为什么 HLS 流启动慢？

A: HLS 需要等待第一个分片生成完成才能播放。可以通过减小 `hls_segment_duration` 来加快首屏时间，但会增加分片数量。

### Q: 如何选择编码预设？

A: 
- 实时转码：选择 `ultrafast` 或 `fast`
- 离线转码：选择 `medium` 或 `slow` 获得更好的压缩率
- 存储归档：选择 `slow` 或 `veryslow`

### Q: 硬件加速如何配置？

A: 需要确保系统安装了相应的驱动和 FFmpeg 支持：
- NVIDIA: 安装 NVIDIA 驱动和 CUDA
- Intel QSV: 安装 Intel Media Driver
- VAAPI: 安装 mesa-va-drivers 或对应驱动

### Q: 字幕提取支持哪些格式？

A: 目前支持从 MKV 容器中提取内嵌字幕，支持 SRT、ASS、SSA、VTT 等格式的提取和转换。