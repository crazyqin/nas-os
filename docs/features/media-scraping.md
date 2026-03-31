# 影视刮削增强设计

## 功能概述
对标飞牛fnOS智能影视，增强影视元数据刮削功能。

## API设计

### 刮削API
```
POST /api/v1/media/scrape/start             - 开始刮削任务
GET  /api/v1/media/scrape/status/:id        - 获取刮削状态
POST /api/v1/media/scrape/batch             - 批量刮削
GET  /api/v1/media/metadata/:id             - 获取元数据
PUT  /api/v1/media/metadata/:id             - 更新元数据
```

### 海报墙API
```
GET  /api/v1/media/poster-wall              - 获取海报墙列表
GET  /api/v1/media/poster-wall/:category    - 分类海报墙
GET  /api/v1/media/posters/:id              - 获取海报图片
POST /api/v1/media/posters/download         - 批量下载海报
```

### 元数据源配置
```
GET  /api/v1/media/scrapers                 - 获取刮削源列表
PUT  /api/v1/media/scrapers/:provider       - 配置刮削源
```

## 数据模型

### 媒体元数据
```go
type MediaMetadata struct {
    ID           string    `json:"id"`
    Type         string    `json:"type"` // movie, tv, music
    Title        string    `json:"title"`
    OriginalTitle string   `json:"original_title"`
    Year         int       `json:"year"`
    Rating       float64   `json:"rating"`
    Runtime      int       `json:"runtime"` // 分钟
    Genres       []string  `json:"genres"`
    Directors    []string  `json:"directors"`
    Actors       []string  `json:"actors"`
    Overview     string    `json:"overview"`
    PosterURL    string    `json:"poster_url"`
    BackdropURL  string    `json:"backdrop_url"`
    Language     string    `json:"language"`
    TMDBID       string    `json:"tmdb_id"`
    IMDBID       string    `json:"imdb_id"`
    LocalPath    string    `json:"local_path"`
    ScrapedAt    time.Time `json:"scraped_at"`
}
```

## 支持的刮削源

| 源 | 说明 | 优先级 |
|-----|------|--------|
| TMDB | The Movie Database | 1 |
| IMDB | Internet Movie Database | 2 |
| Douban | 豆瓣电影 | 3 |
| OMDB | Open Movie Database | 4 |

## 实现要点

1. **自动识别**: 根据文件名智能识别电影/电视剧
2. **批量刮削**: 并行处理提升效率
3. **海报下载**: 自动下载海报/剧照
4. **多语言**: 支持中/英/日等多语言元数据
5. **本地缓存**: 缓存刮削结果减少API调用

## WebUI展示
- 海报墙画廊视图
- 媒体详情页面
- 刮削进度显示
- 元数据编辑界面

## 版本计划
- v2.362.0: API设计完成
- v2.365.0: TMDB刮削实现
- v2.370.0: 海报墙WebUI