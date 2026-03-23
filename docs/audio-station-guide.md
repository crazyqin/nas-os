# NAS-OS Audio Station 功能规划

## 功能概述

NAS-OS Audio Station 是专业的音乐管理与播放解决方案，对标 Synology Audio Station，提供完整的音乐库管理、多端播放、智能推荐等功能。

---

## 功能列表

### 核心功能

| 功能模块 | 优先级 | 状态 | 说明 |
|----------|--------|------|------|
| 音乐库管理 | P0 | 规划中 | 专辑/艺术家/歌曲分类管理 |
| 元数据管理 | P0 | 规划中 | 自动抓取专辑封面、歌词 |
| 多格式支持 | P0 | 已有 | MP3/FLAC/AAC/WAV/APE/OGG |
| 播放列表 | P0 | 规划中 | 创建/编辑/分享播放列表 |
| 多端播放 | P0 | 规划中 | Web/移动端/DLNA/AirPlay |
| 歌词显示 | P1 | 规划中 | 自动匹配歌词，同步显示 |
| 在线电台 | P1 | 规划中 | 网络电台流媒体播放 |
| 播客支持 | P1 | 规划中 | 播客订阅与管理 |
| 智能推荐 | P2 | 规划中 | 基于播放历史的推荐 |

### 详细功能说明

#### 1. 音乐库管理

**艺术家视图**
- 按艺术家分组显示专辑
- 显示艺术家图片和简介
- 支持多艺术家歌曲

**专辑视图**
- 专辑封面网格/列表视图
- 专辑信息：年份、类型、曲目数
- 支持合集专辑

**歌曲视图**
- 全部歌曲列表
- 支持排序：标题、时长、添加时间、播放次数
- 快速搜索过滤

**文件夹视图**
- 按文件系统目录浏览
- 支持快速定位

#### 2. 元数据管理

**自动抓取**
- 专辑封面：Last.fm / MusicBrainz
- 歌词：网易云音乐 / QQ音乐 API
- 艺术家信息：Last.fm

**手动编辑**
- 编辑歌曲元数据
- 批量修改专辑信息
- 自定义封面图片

**元数据字段**
```
- 标题 (title)
- 艺术家 (artist)
- 专辑 (album)
- 专辑艺术家 (album_artist)
- 年份 (year)
- 曲目号 (track_number)
- 光盘号 (disc_number)
- 流派 (genre)
- 作曲 (composer)
- 歌词 (lyrics)
- 评分 (rating)
```

#### 3. 播放功能

**Web 播放器**
- HTML5 Audio API
- 支持无缝播放
- 均衡器设置
- 播放速度调节

**移动端播放**
- 后台播放
- 锁屏控制
- 蓝牙设备支持
- 离线下载

**DLNA 投射**
- 发现局域网 DLNA 设备
- 一键投射播放
- 支持播放控制

**AirPlay 支持**
- 发送到 AirPlay 设备
- 支持音频同步

#### 4. 播放列表

**系统播放列表**
- 我喜欢的音乐
- 最近播放
- 添加最多
- 播放最多

**自定义播放列表**
- 创建/编辑/删除
- 拖拽排序
- 导入/导出 (M3U/M3U8)
- 分享播放列表

**智能播放列表**
- 基于规则自动生成
- 支持条件：流派、评分、添加时间
- 定期自动更新

#### 5. 歌词显示

**歌词来源**
- 嵌入式歌词 (ID3 标签)
- 外部 .lrc 文件
- 在线歌词匹配

**显示模式**
- 同步滚动歌词
- 翻译歌词对照
- 卡拉OK模式

#### 6. 在线电台

**预设电台**
- 全球热门电台
- 按流派分类
- 自定义添加

**电台功能**
- 收藏电台
- 最近收听
- 录制功能 (P2)

#### 7. 播客支持

**订阅管理**
- RSS 订阅
- 自动更新检查
- 下载管理

**播放功能**
- 记住播放位置
- 变速播放
- 章节跳转

---

## 技术方案

### 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                     Client Layer                         │
├─────────────┬─────────────┬─────────────┬───────────────┤
│   Web App   │  iOS App    │ Android App │  DLNA/AirPlay │
└──────┬──────┴──────┬──────┴──────┬──────┴───────┬───────┘
       │             │             │              │
       └─────────────┴──────┬──────┴──────────────┘
                            │
┌───────────────────────────┼───────────────────────────┐
│                      API Gateway                        │
│  ┌─────────────────────────────────────────────────┐   │
│  │  REST API (Gin)     │  WebSocket (Real-time)    │   │
│  └─────────────────────────────────────────────────┘   │
└───────────────────────────┬───────────────────────────┘
                            │
┌───────────────────────────┼───────────────────────────┐
│                    Service Layer                        │
├─────────────┬─────────────┼─────────────┬─────────────┤
│   Library   │  Metadata   │  Streaming  │  Playlist   │
│   Manager   │   Service   │   Service   │   Service   │
├─────────────┼─────────────┼─────────────┼─────────────┤
│   Search    │   Lyrics    │    Radio    │   Podcast   │
│   Service   │   Service   │   Service   │   Service   │
└─────────────┴─────────────┴─────────────┴─────────────┘
                            │
┌───────────────────────────┼───────────────────────────┐
│                    Data Layer                           │
├─────────────┬─────────────┼─────────────┬─────────────┤
│   SQLite    │   Redis     │    Files    │    Cache    │
│  (Metadata) │  (Session)  │  (Audio)    │   (Cover)   │
└─────────────┴─────────────┴─────────────┴─────────────┘
```

### 核心模块设计

#### 1. Library Manager (库管理器)

```go
// internal/audio/library.go

// Library 音乐库
type Library struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Path        string    `json:"path"`
    Enabled     bool      `json:"enabled"`
    AutoScan    bool      `json:"autoScan"`
    ScanInterval int      `json:"scanInterval"` // 分钟
    LastScan    time.Time `json:"lastScan"`
}

// LibraryManager 音乐库管理器
type LibraryManager struct {
    libraries  map[string]*Library
    db         *sql.DB
    scanner    *Scanner
    metadata   *MetadataService
    mu         sync.RWMutex
}

// Scan 扫描音乐库
func (lm *LibraryManager) Scan(libraryID string) error {
    // 1. 遍历目录
    // 2. 识别音频文件
    // 3. 提取元数据
    // 4. 匹配封面/歌词
    // 5. 更新数据库
}
```

#### 2. Metadata Service (元数据服务)

```go
// internal/audio/metadata.go

// MetadataProvider 元数据提供商接口
type MetadataProvider interface {
    GetAlbumInfo(artist, album string) (*AlbumInfo, error)
    GetArtistInfo(artist string) (*ArtistInfo, error)
    SearchLyrics(artist, title string) (string, error)
    GetCoverArt(artist, album string) ([]byte, error)
}

// MetadataService 元数据服务
type MetadataService struct {
    providers []MetadataProvider
    cache     *Cache
}

// 内置提供商
type LastFMProvider struct { apiKey string }
type MusicBrainzProvider struct {}
class NetEaseLyricsProvider struct { apiKey string }
```

#### 3. Streaming Service (流媒体服务)

```go
// internal/audio/streaming.go

// StreamingService 流媒体服务
type StreamingService struct {
    transcoder *Transcoder
    cache      *StreamCache
}

// Stream 音频流
func (s *StreamingService) Stream(songID string, format string, quality int) (io.ReadCloser, error) {
    // 1. 获取源文件路径
    // 2. 检查缓存
    // 3. 按需转码
    // 4. 返回流
}

// Transcoder 转码器
type Transcoder struct {
    ffmpegPath string
}

// 支持的输出格式
var OutputFormats = []string{
    "mp3",    // 兼容性最好
    "aac",    // 高效压缩
    "flac",   // 无损
    "opus",   // 低延迟
}
```

#### 4. Playlist Service (播放列表服务)

```go
// internal/audio/playlist.go

// Playlist 播放列表
type Playlist struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Owner       string    `json:"owner"`
    IsPublic    bool      `json:"isPublic"`
    Songs       []string  `json:"songs"` // song IDs
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

// SmartPlaylist 智能播放列表
type SmartPlaylist struct {
    ID       string        `json:"id"`
    Name     string        `json:"name"`
    Rules    []FilterRule  `json:"rules"`
    Limit    int           `json:"limit"`
    OrderBy  string        `json:"orderBy"`
}

// FilterRule 过滤规则
type FilterRule struct {
    Field    string `json:"field"`    // genre, rating, year, etc.
    Operator string `json:"operator"` // eq, ne, gt, lt, contains
    Value    string `json:"value"`
}
```

### 数据存储设计

#### SQLite 表结构

```sql
-- 歌曲表
CREATE TABLE songs (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    artist TEXT,
    album TEXT,
    album_artist TEXT,
    year INTEGER,
    track_number INTEGER,
    disc_number INTEGER,
    genre TEXT,
    composer TEXT,
    duration INTEGER,
    bitrate INTEGER,
    sample_rate INTEGER,
    channels INTEGER,
    file_size INTEGER,
    file_format TEXT,
    rating INTEGER DEFAULT 0,
    play_count INTEGER DEFAULT 0,
    date_added TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_played TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 专辑表
CREATE TABLE albums (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    artist TEXT,
    year INTEGER,
    genre TEXT,
    cover_path TEXT,
    cover_url TEXT,
    song_count INTEGER DEFAULT 0,
    duration INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 艺术家表
CREATE TABLE artists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    image_path TEXT,
    image_url TEXT,
    bio TEXT,
    album_count INTEGER DEFAULT 0,
    song_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 播放列表表
CREATE TABLE playlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    owner TEXT NOT NULL,
    is_public BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 播放列表歌曲关联表
CREATE TABLE playlist_songs (
    playlist_id TEXT NOT NULL,
    song_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (playlist_id, song_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
    FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE CASCADE
);

-- 歌词表
CREATE TABLE lyrics (
    song_id TEXT PRIMARY KEY,
    lyrics TEXT,
    synced_lyrics TEXT,  -- LRC 格式
    source TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE CASCADE
);
```

---

## API 设计

### REST API 端点

#### 音乐库管理

```yaml
# 获取音乐库列表
GET /api/audio/libraries
Response: { "libraries": [{ "id", "name", "path", ... }] }

# 创建音乐库
POST /api/audio/libraries
Body: { "name": "My Music", "path": "/music", "autoScan": true }

# 扫描音乐库
POST /api/audio/libraries/{id}/scan

# 获取扫描状态
GET /api/audio/libraries/{id}/scan/status
```

#### 歌曲管理

```yaml
# 获取歌曲列表
GET /api/audio/songs
Query: ?page=1&limit=50&sort=title&order=asc&filter=artist:周杰伦
Response: { "songs": [...], "total": 1000, "page": 1 }

# 获取歌曲详情
GET /api/audio/songs/{id}

# 更新歌曲元数据
PUT /api/audio/songs/{id}
Body: { "title": "...", "artist": "...", "rating": 5 }

# 批量更新
PUT /api/audio/songs/batch
Body: { "ids": ["id1", "id2"], "updates": { "genre": "Pop" } }

# 获取歌曲歌词
GET /api/audio/songs/{id}/lyrics
```

#### 专辑与艺术家

```yaml
# 获取专辑列表
GET /api/audio/albums
Query: ?artist=周杰伦&sort=year&order=desc

# 获取专辑详情
GET /api/audio/albums/{id}
Response: { "album": {...}, "songs": [...] }

# 获取艺术家列表
GET /api/audio/artists

# 获取艺术家详情
GET /api/audio/artists/{id}
Response: { "artist": {...}, "albums": [...] }
```

#### 播放列表

```yaml
# 获取播放列表
GET /api/audio/playlists

# 创建播放列表
POST /api/audio/playlists
Body: { "name": "我的最爱", "description": "...", "isPublic": true }

# 更新播放列表
PUT /api/audio/playlists/{id}

# 添加歌曲到播放列表
POST /api/audio/playlists/{id}/songs
Body: { "songIds": ["song1", "song2"] }

# 移除歌曲
DELETE /api/audio/playlists/{id}/songs/{songId}

# 导入播放列表
POST /api/audio/playlists/import
Body: multipart/form-data (M3U/M3U8 file)

# 导出播放列表
GET /api/audio/playlists/{id}/export?format=m3u
```

#### 流媒体播放

```yaml
# 获取音频流
GET /api/audio/stream/{songId}
Query: ?format=mp3&quality=320&transcode=true

# 获取转码进度
GET /api/audio/stream/{songId}/status

# 获取封面图片
GET /api/audio/cover/{albumId}
Query: ?size=300
```

#### 搜索

```yaml
# 搜索
GET /api/audio/search
Query: ?q=周杰伦&type=song,album,artist

# 高级搜索
POST /api/audio/search/advanced
Body: {
  "query": "周杰伦",
  "filters": {
    "genre": "Pop",
    "year": [2000, 2010]
  },
  "sort": "rating",
  "order": "desc"
}
```

### WebSocket 事件

```javascript
// 播放状态同步
ws.send({
  "type": "play_state",
  "data": {
    "songId": "song123",
    "position": 120,
    "state": "playing"
  }
});

// 多设备同步
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case "sync_play":
      // 同步播放
      break;
    case "sync_pause":
      // 同步暂停
      break;
  }
};
```

---

## 与现有模块集成

### 集成点

| 模块 | 集成方式 | 说明 |
|------|----------|------|
| internal/media | 扩展现有 media 模块 | 复用元数据服务、流媒体功能 |
| internal/files | 依赖 | 音频文件读取、缩略图生成 |
| internal/search | 依赖 | 歌曲搜索功能 |
| internal/auth | 依赖 | 用户认证、权限控制 |
| internal/webdav | 协作 | 支持 WebDAV 暴露音乐库 |

### 代码位置

```
internal/audio/
├── library.go         # 音乐库管理
├── metadata.go        # 元数据服务
├── playlist.go        # 播放列表
├── streaming.go       # 流媒体
├── lyrics.go          # 歌词服务
├── radio.go           # 在线电台
├── podcast.go         # 播客支持
├── transcoder.go      # 音频转码
├── dlna.go            # DLNA 服务
├── airplay.go         # AirPlay 支持
├── handlers.go        # API 处理器
├── models.go          # 数据模型
└── audio_test.go      # 单元测试
```

### 开发排期

| 周期 | 任务 | 交付物 |
|------|------|--------|
| W1 | 音乐库核心 + 元数据服务 | library.go, metadata.go |
| W2 | 流媒体播放 + 播放列表 | streaming.go, playlist.go |
| W3 | 歌词 + 搜索 + API | lyrics.go, handlers.go |
| W4 | DLNA + 移动端适配 + 测试 | dlna.go, 测试用例 |

---

## 相关文档

- [Drive 用户指南](./drive-user-guide.md)
- [Calendar & Contacts 规格说明](./calendar-contacts-spec.md)
- [API 参考文档](./API_GUIDE.md)

---

**文档版本**: v1.0.0  
**最后更新**: 2026-03-23