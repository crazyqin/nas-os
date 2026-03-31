# 影视刮削增强设计

> 对标飞牛fnOS智能影视海报墙功能

## 概述

增强影视刮削功能，支持TMDB/IMDB元数据获取、海报墙展示、智能识别。

## API设计

### 元数据刮削

```
GET    /api/v1/media/movies                   # 电影列表(海报墙)
GET    /api/v1/media/tvshows                  # 电视剧列表
POST   /api/v1/media/scrape                   # 手动刮削
POST   /api/v1/media/scrape/batch             # 批量刮削
GET    /api/v1/media/movies/:id               # 电影详情
GET    /api/v1/media/tvshows/:id              # 电视剧详情
GET    /api/v1/media/movies/:id/poster        # 获取海报
GET    /api/v1/media/search                   # 搜索影视
```

### 海报墙展示

```
GET    /api/v1/media/posters                  # 海报墙列表
GET    /api/v1/media/posters/recent           # 最近添加
GET    /api/v1/media/posters/favorites        # 收藏
GET    /api/v1/media/posters/unwatched        # 未观看
```

## 数据模型

```go
type Movie struct {
    ID           string   `json:"id"`
    Title        string   `json:"title"`
    OriginalTitle string  `json:"original_title"`
    Year         int      `json:"year"`
    Rating       float64  `json:"rating"`
    Runtime      int      `json:"runtime"` // 分钟
    Genres       []string `json:"genres"`
    Overview     string   `json:"overview"`
    PosterURL    string   `json:"poster_url"`
    BackdropURL  string   `json:"backdrop_url"`
    IMDbID       string   `json:"imdb_id"`
    TMDBID       int      `json:"tmdb_id"`
    LocalPath    string   `json:"local_path"`
    DateAdded    time.Time `json:"date_added"`
    Watched      bool     `json:"watched"`
}

type TVShow struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    Year         int       `json:"year"`
    Seasons      []Season  `json:"seasons"`
    Episodes     int       `json:"total_episodes"`
    Rating       float64   `json:"rating"`
    PosterURL    string    `json:"poster_url"`
    Status       string    `json:"status"` // ongoing, ended
    LocalPath    string    `json:"local_path"`
}
```

## 功能特性

1. **自动识别**: 根据文件名智能识别电影/电视剧
2. **多源刮削**: TMDB、IMDB、豆瓣多源支持
3. **海报下载**: 自动下载海报、剧照
4. **批量处理**: 批量刮削整个目录
5. **多语言**: 支持中英文元数据
6. **海报墙**: 美观的海报墙展示界面

## 实现要点

- 文件名解析正则表达式
- TMDB API集成 (https://api.themoviedb.org)
- 海报缓存本地存储
- 并发刮削限制