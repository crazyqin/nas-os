// Package rssreader 提供 RSS 订阅阅读器功能
package rssreader

import (
	"time"
)

// ========== 错误定义 ==========

// ErrFeedNotFound 订阅源未找到.
var ErrFeedNotFound = &FeedError{Code: "FEED_NOT_FOUND", Message: "订阅源未找到"}

// ErrArticleNotFound 文章未找到.
var ErrArticleNotFound = &ArticleError{Code: "ARTICLE_NOT_FOUND", Message: "文章未找到"}

// ErrCategoryNotFound 分类未找到.
var ErrCategoryNotFound = &CategoryError{Code: "CATEGORY_NOT_FOUND", Message: "分类未找到"}

// ErrDuplicateArticle 文章重复.
var ErrDuplicateArticle = &ArticleError{Code: "DUPLICATE_ARTICLE", Message: "文章已存在"}

// ErrInvalidFeedURL 无效的订阅源URL.
var ErrInvalidFeedURL = &FeedError{Code: "INVALID_FEED_URL", Message: "无效的订阅源URL"}

// ErrFeedFetchFailed 订阅源抓取失败.
var ErrFeedFetchFailed = &FeedError{Code: "FEED_FETCH_FAILED", Message: "订阅源抓取失败"}

// FeedError 订阅源错误.
type FeedError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *FeedError) Error() string {
	return e.Code + ": " + e.Message
}

// ArticleError 文章错误.
type ArticleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ArticleError) Error() string {
	return e.Code + ": " + e.Message
}

// CategoryError 分类错误.
type CategoryError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CategoryError) Error() string {
	return e.Code + ": " + e.Message
}

// ========== 数据模型 ==========

// Feed 订阅源.
type Feed struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	FeedURL     string    `json:"feed_url"`
	SiteURL     string    `json:"site_url,omitempty"`
	Description string    `json:"description,omitempty"`
	CategoryID  string    `json:"category_id,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	IsEnabled   bool      `json:"is_enabled"`
	LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	FetchInterval int      `json:"fetch_interval"` // 抓取间隔（分钟）
	ArticleCount  int      `json:"article_count"`
	UnreadCount   int      `json:"unread_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Article 文章.
type Article struct {
	ID          string    `json:"id"`
	FeedID      string    `json:"feed_id"`
	GUID        string    `json:"guid,omitempty"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Content     string    `json:"content"`
	Summary     string    `json:"summary,omitempty"`
	Author      string    `json:"author,omitempty"`
	IsRead      bool      `json:"is_read"`
	IsFavorite  bool      `json:"is_favorite"`
	IsMarked    bool      `json:"is_marked"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Category 分类.
type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ParentID    string    `json:"parent_id,omitempty"`
	FeedCount   int       `json:"feed_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// FeedHealth 订阅源健康状态.
type FeedHealth struct {
	FeedID        string     `json:"feed_id"`
	IsReachable   bool       `json:"is_reachable"`
	LastCheckedAt time.Time  `json:"last_checked_at"`
	ResponseTime  int64      `json:"response_time"` // 毫秒
	ErrorMessage  string     `json:"error_message,omitempty"`
	UpdateFrequency string   `json:"update_frequency,omitempty"` // 更新频率描述
}

// Stats 统计信息.
type Stats struct {
	TotalFeeds     int `json:"total_feeds"`
	TotalArticles  int `json:"total_articles"`
	UnreadCount    int `json:"unread_count"`
	ReadCount      int `json:"read_count"`
	FavoriteCount  int `json:"favorite_count"`
	MarkedCount    int `json:"marked_count"`
	FeedStats      []FeedStat `json:"feed_stats,omitempty"`
}

// FeedStat 单个订阅源统计.
type FeedStat struct {
	FeedID      string `json:"feed_id"`
	FeedTitle   string `json:"feed_title"`
	TotalCount  int    `json:"total_count"`
	UnreadCount int    `json:"unread_count"`
}

// ========== 请求/响应模型 ==========

// CreateFeedRequest 创建订阅源请求.
type CreateFeedRequest struct {
	Title         string   `json:"title"`
	FeedURL       string   `json:"feed_url" binding:"required"`
	CategoryID    string   `json:"category_id,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	FetchInterval int      `json:"fetch_interval,omitempty"` // 分钟，默认30
}

// UpdateFeedRequest 更新订阅源请求.
type UpdateFeedRequest struct {
	Title         *string  `json:"title,omitempty"`
	CategoryID    *string  `json:"category_id,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	IsEnabled     *bool    `json:"is_enabled,omitempty"`
	FetchInterval *int     `json:"fetch_interval,omitempty"`
}

// CreateCategoryRequest 创建分类请求.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

// UpdateCategoryRequest 更新分类请求.
type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parent_id,omitempty"`
}

// UpdateArticleRequest 更新文章状态请求.
type UpdateArticleRequest struct {
	IsRead     *bool `json:"is_read,omitempty"`
	IsFavorite *bool `json:"is_favorite,omitempty"`
	IsMarked   *bool `json:"is_marked,omitempty"`
}

// ImportOPMLRequest 导入OPML请求.
type ImportOPMLRequest struct {
	Content string `json:"content" binding:"required"` // OPML XML 内容
}

// ExportOPMLResponse 导出OPML响应.
type ExportOPMLResponse struct {
	Content string `json:"content"` // OPML XML 内容
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query      string `form:"q" binding:"required"`
	FeedID     string `form:"feed_id,omitempty"`
	IsRead     *bool  `form:"is_read,omitempty"`
	IsFavorite *bool  `form:"is_favorite,omitempty"`
}

// ListArticlesRequest 列出文章请求.
type ListArticlesRequest struct {
	FeedID     string `form:"feed_id,omitempty"`
	CategoryID string `form:"category_id,omitempty"`
	IsRead     *bool  `form:"is_read,omitempty"`
	IsFavorite *bool  `form:"is_favorite,omitempty"`
	IsMarked   *bool  `form:"is_marked,omitempty"`
	Page       int    `form:"page,omitempty"`
	PageSize   int    `form:"page_size,omitempty"`
}

// OPMLDocument OPML文档结构.
type OPMLDocument struct {
	Head OPMLHead `xml:"head"`
	Body OPMLBody `xml:"body"`
}

// OPMLHead OPML头部.
type OPMLHead struct {
	Title   string `xml:"title"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

// OPMLBody OPML内容.
type OPMLBody struct {
	Outlines []OPMLOutline `xml:"outline"`
}

// OPMLOutline OPML条目.
type OPMLOutline struct {
	Text        string          `xml:"text,attr"`
	Title       string          `xml:"title,attr,omitempty"`
	Type        string          `xml:"type,attr,omitempty"`
	XMLURL      string          `xml:"xmlUrl,attr,omitempty"`
	HTMLURL     string          `xml:"htmlUrl,attr,omitempty"`
	Description string          `xml:"description,attr,omitempty"`
	Outlines    []OPMLOutline   `xml:"outline,omitempty"`
}
