package mediascraper

import (
	"time"
)

// MediaType 媒体类型.
type MediaType string

const (
	MediaTypeMovie    MediaType = "movie"    // 电影
	MediaTypeTVSeries MediaType = "tvseries" // 电视剧
)

// MediaItem 媒体信息项，刮削后的完整媒体元数据.
type MediaItem struct {
	ID            string    `json:"id"`                       // 唯一标识
	Title         string    `json:"title"`                    // 标题
	OriginalTitle string    `json:"original_title,omitempty"` // 原始标题
	Year          int       `json:"year"`                     // 上映年份
	Type          MediaType `json:"type"`                     // 媒体类型（电影/电视剧）
	PosterPath    string    `json:"poster_path"`              // 海报图片路径
	Overview      string    `json:"overview"`                 // 简介/剧情概要
	Cast          []string  `json:"cast"`                     // 演员列表
	Director      string    `json:"director,omitempty"`       // 导演
	Rating        float64   `json:"rating"`                   // 评分（0-10）
	Genres        []string  `json:"genres"`                   // 类型标签（动作/喜剧等）
	FilePath      string    `json:"file_path"`                // 原始文件路径
	ScrapedAt     time.Time `json:"scraped_at"`               // 刮削时间
}

// ScraperResult 刮削结果，包含刮削状态和可能的错误.
type ScraperResult struct {
	Item       *MediaItem // 刮削成功的媒体项
	Found      bool       // 是否找到匹配的元数据
	Confidence float64    // 匹配置信度（0-1）
	Error      error      // 刮削过程中的错误
}

// SubtitleResult 字幕下载结果.
type SubtitleResult struct {
	FilePath     string    // 字幕文件保存路径
	Language     string    // 字幕语言（如 zh-CN, en-US）
	Source       string    // 字幕来源
	DownloadedAt time.Time // 下载时间
	Error        error     // 下载错误（如有）
}

// PosterWallGroup 海报墙分组，按类型分组后的海报数据.
type PosterWallGroup struct {
	Type  MediaType    `json:"type"`  // 分组类型
	Title string       `json:"title"` // 分组标题（如"电影"、"电视剧"）
	Items []*MediaItem `json:"items"` // 该组下的媒体项列表
	Count int          `json:"count"` // 媒体项数量
}

// PosterWall 海报墙，包含所有分组.
type PosterWall struct {
	Groups    []*PosterWallGroup `json:"groups"`     // 所有分组
	Total     int                `json:"total"`      // 总媒体数
	UpdatedAt time.Time          `json:"updated_at"` // 更新时间
}

// metadataRecord 模拟的元数据API返回记录（内部使用）.
type metadataRecord struct {
	Title         string   `json:"title"`
	OriginalTitle string   `json:"original_title,omitempty"`
	Year          int      `json:"year"`
	Overview      string   `json:"overview"`
	Cast          []string `json:"cast"`
	Director      string   `json:"director,omitempty"`
	Rating        float64  `json:"rating"`
	Genres        []string `json:"genres"`
	PosterURL     string   `json:"poster_url"`
}

// subtitleRecord 模拟的字幕库记录（内部使用）.
type subtitleRecord struct {
	Language string `json:"language"`
	Source   string `json:"source"`
	Content  string `json:"content"` // 字幕文件内容（模拟）
}
