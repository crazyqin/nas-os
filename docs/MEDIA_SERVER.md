# 媒体服务器功能集成

NAS-OS 现已集成完整的媒体服务器功能，支持电影、电视剧、音乐、照片的管理和播放。

## 核心功能

### 1. 媒体库管理
- ✅ 自动扫描媒体文件
- ✅ 元数据抓取（TMDB/豆瓣）
- ✅ 海报墙展示
- ✅ 多类型支持（电影/电视剧/音乐/照片）

### 2. 视频播放
- ✅ 在线播放（通过 Jellyfin/Emby）
- ✅ 字幕支持
- ✅ 播放进度记录

### 3. 音乐播放
- ✅ 专辑封面显示
- ✅ 播放列表管理

### 4. 图片浏览
- ✅ 相册管理
- ✅ 幻灯片播放

### 5. 转码功能
- ✅ 实时转码（通过 Jellyfin）
- ✅ 多清晰度支持

### 6. DLNA/投屏
- ✅ 支持电视投屏
- ✅ 手机投屏

## 快速开始

### 方式一：使用内置媒体中心（推荐）

#### 1. 访问媒体中心
在 NAS-OS Web 界面左侧导航栏点击 **媒体中心**。

#### 2. 创建媒体库
1. 点击右下角 **+** 按钮
2. 填写媒体库信息：
   - **媒体库名称**: 例如"我的电影"
   - **媒体类型**: 选择 电影/电视剧/音乐/照片
   - **媒体库路径**: 例如 `/data/media/movies`
   - **元数据源**: 选择 自动/TMDB/豆瓣
3. （可选）配置 API 密钥：
   - **TMDB API Key**: [点击申请](https://www.themoviedb.org/settings/api)
   - **豆瓣 API Key**: 如有
4. 点击 **创建媒体库**

#### 3. 查看海报墙
创建完成后，系统会自动扫描媒体文件并生成海报墙。

### 方式二：使用 Jellyfin

#### 1. 安装 Jellyfin
1. 进入 **应用商店**
2. 找到 **Jellyfin** 🎬
3. 点击 **安装**
4. 配置端口和路径：
   - Web 端口：`8096`
   - 配置目录：`/opt/nas/apps/jellyfin/config`
   - 缓存目录：`/opt/nas/apps/jellyfin/cache`
   - 媒体目录：`/opt/nas/media`（或你的媒体路径）

#### 2. 配置 Jellyfin
1. 访问 `http://<你的 NAS IP>:8096`
2. 完成初始化向导
3. 创建用户
4. 添加媒体库

#### 3. 启用中文元数据
1. 在 Jellyfin 控制台 -> **插件**
2. 安装 **TheMovieDb** 插件
3. 配置 TMDB API Key（可选，有 Key 配额更高）
4. 在媒体库设置中，将元数据语言设置为 `zh-CN`

#### 4. 推荐插件
- **TheMovieDb**: TMDB 元数据，支持中文 ✅
- **TheOpenMovieDatabase**: OMDb 元数据
- **Douban**: 豆瓣元数据（需手动安装第三方插件）

## API 接口

### 媒体库管理

#### 创建媒体库
```bash
POST /api/v1/media/libraries
Content-Type: application/json

{
  "name": "我的电影",
  "type": "movie",
  "path": "/data/media/movies",
  "description": "收藏的电影",
  "metadataSource": "tmdb",
  "tmdbApiKey": "your-api-key"
}
```

#### 列出媒体库
```bash
GET /api/v1/media/libraries
```

#### 扫描媒体库
```bash
POST /api/v1/media/libraries/{id}/scan
```

#### 更新媒体库
```bash
PUT /api/v1/media/libraries/{id}
Content-Type: application/json

{
  "name": "新名称",
  "enabled": true,
  "autoScan": true,
  "scanInterval": 60
}
```

#### 删除媒体库
```bash
DELETE /api/v1/media/libraries/{id}
```

### 海报墙

#### 获取全部海报墙
```bash
GET /api/v1/media/wall
```

#### 获取电影海报墙
```bash
GET /api/v1/media/wall/movies
```

#### 获取电视剧海报墙
```bash
GET /api/v1/media/wall/tv
```

#### 获取音乐专辑墙
```bash
GET /api/v1/media/wall/music
```

### 元数据搜索

#### 搜索电影元数据
```bash
GET /api/v1/media/metadata/search/movie?q=流浪地球&source=tmdb
```

#### 搜索电视剧元数据
```bash
GET /api/v1/media/metadata/search/tv?q=三体&source=douban
```

#### 获取电影元数据详情
```bash
GET /api/v1/media/metadata/movie/tmdb_12345
```

### 搜索媒体

#### 搜索媒体
```bash
GET /api/v1/media/items?q=流浪地球&type=movie
```

### 播放历史

#### 获取播放历史
```bash
GET /api/v1/media/history
```

#### 添加播放记录
```bash
POST /api/v1/media/history
Content-Type: application/json

{
  "mediaId": "item_movie_123",
  "position": 3600,
  "duration": 7200,
  "completed": false
}
```

### 收藏

#### 获取收藏列表
```bash
GET /api/v1/media/favorites
```

#### 切换收藏状态
```bash
POST /api/v1/media/items/{id}/favorite
```

## 元数据提供商

### TMDB（The Movie Database）
- **支持**: 电影、电视剧
- **语言**: 支持中文
- **API Key**: [免费申请](https://www.themoviedb.org/settings/api)
- **优点**: 数据全、更新快、支持中文

### 豆瓣
- **支持**: 电影、电视剧
- **语言**: 中文
- **API Key**: 需要申请（已限制）
- **优点**: 中文元数据质量高、评分参考价值大

### 推荐配置
```json
{
  "metadataSource": "auto",
  "tmdbApiKey": "your-tmdb-api-key",
  "doubanApiKey": "",
  "chinesePriority": 3
}
```

## 文件命名规范

### 电影
```
/data/media/movies/
├── 流浪地球 (2019).mp4
├── 流浪地球 2 (2023).mkv
├── The Wandering Earth (2019).mp4
└── 三体 (2023).mp4
```

### 电视剧
```
/data/media/tv/
├── 三体/
│   ├── S01E01.mp4
│   ├── S01E02.mp4
│   └── S01E03.mp4
└── 狂飙/
    ├── S01E01.mp4
    └── S01E02.mp4
```

### 音乐
```
/data/media/music/
├── 周杰伦/
│   └── 范特西/
│       ├── 01.爱在西元前.mp3
│       └── 02.爸我回来了.mp3
└── 陈奕迅/
    └── 十年/
        └── 十年.mp3
```

### 照片
```
/data/photos/
├── 2024/
│   ├── 2024-01-01 元旦/
│   └── 2024-02-10 春节/
└── 2025/
    └── 2025-01-01 元旦/
```

## Jellyfin 集成

### Docker Compose 配置
```yaml
version: '3'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    restart: unless-stopped
    ports:
      - "8096:8096"
    volumes:
      - /opt/nas/apps/jellyfin/config:/config
      - /opt/nas/apps/jellyfin/cache:/cache
      - /data/media:/media:ro
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
    devices:
      - /dev/dri:/dev/dri  # 硬件转码（可选）
```

### 硬件转码
如果 CPU 支持硬件转码（Intel QuickSync、NVIDIA NVENC），可以在 Docker 配置中添加设备映射：

**Intel:**
```yaml
devices:
  - /dev/dri:/dev/dri
```

**NVIDIA:**
```yaml
devices:
  - /dev/nvidia0:/dev/nvidia0
  - /dev/nvidiactl:/dev/nvidiactl
  - /dev/nvidia-uvm:/dev/nvidia-uvm
```

### 优化建议
1. **启用硬件转码**: 降低 CPU 占用
2. **配置转码阈值**: 根据网络带宽调整
3. **启用字幕**: 支持外挂字幕
4. **配置 DLNA**: 支持电视直接播放

## 故障排查

### 媒体库扫描失败
1. 检查路径是否正确
2. 检查文件权限：`chmod -R 755 /data/media`
3. 检查文件格式是否支持

### 元数据获取失败
1. 检查网络连接
2. 验证 API Key 是否有效
3. 尝试更换元数据源

### Jellyfin 无法连接
1. 检查容器状态：`docker ps | grep jellyfin`
2. 检查端口占用：`netstat -tlnp | grep 8096`
3. 查看日志：`docker logs jellyfin`

### 海报墙不显示
1. 确认媒体库已扫描完成
2. 检查元数据是否获取成功
3. 刷新页面缓存

## 高级功能

### 定时扫描
媒体库支持自动扫描，默认间隔 60 分钟。可在媒体库设置中调整。

### 元数据手动修正
如果自动抓取的元数据不准确，可以：
1. 在媒体详情中点击 **编辑**
2. 搜索正确的元数据
3. 手动选择并应用

### 批量操作
支持批量：
- 扫描媒体库
- 刷新元数据
- 删除媒体项

## 性能优化

### 数据库优化
定期清理播放历史和元数据缓存。

### 缩略图缓存
缩略图缓存在 `/var/cache/nas-os/thumbnails`，可定期清理。

### 并发扫描
大量媒体文件时，建议分库管理，避免单次扫描过多文件。

## 安全建议

1. **访问控制**: 配置用户权限
2. **网络隔离**: 媒体服务限制在局域网
3. **API 密钥**: 妥善保管 TMDB/豆瓣 API Key
4. **定期备份**: 备份媒体库配置和元数据

## 更新日志

### v1.0.0 (2026-03-11)
- ✅ 初始版本发布
- ✅ 媒体库管理
- ✅ 元数据抓取（TMDB/豆瓣）
- ✅ 海报墙展示
- ✅ Jellyfin 集成
- ✅ Web UI 媒体中心

## 反馈与支持

如有问题或建议，请提交到 [GitHub Issues](https://github.com/crazyqin/nas-os/issues)。
