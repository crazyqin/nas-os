// Package media provides enhanced metadata scraping functionality
// with support for TMDB, Douban, auto-detection, and poster downloading
package media

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ====== 统一刮削器配置 ======

// ScraperConfig 统一刮削器配置
type ScraperConfig struct {
	// TMDB配置
	TMDBAPIKey string
	TMDBLang   string // 默认 zh-CN

	// 豆瓣配置
	DoubanAPIKey string

	// 下载配置
	PosterDir       string        // 海报保存目录
	PosterSize      string        // w200, w300, w500, original
	DownloadTimeout time.Duration // 海报下载超时
	MaxRetries      int           // 最大重试次数

	// 缓存配置
	CacheTTL time.Duration // 缓存过期时间

	// 优先级
	ProviderPriority []string // 优先使用的刮削源，如 ["tmdb", "douban"]
}

// DefaultScraperConfig 返回默认配置
func DefaultScraperConfig() *ScraperConfig {
	return &ScraperConfig{
		TMDBLang:         "zh-CN",
		PosterSize:       "w500",
		DownloadTimeout:  30 * time.Second,
		MaxRetries:       3,
		CacheTTL:         24 * time.Hour,
		ProviderPriority: []string{"tmdb", "douban"},
	}
}

// ====== 统一刮削器 ======

// UnifiedScraper 统一元数据刮削器
// 支持TMDB、豆瓣多数据源，自动识别电影/电视剧，自动下载海报
type UnifiedScraper struct {
	config      *ScraperConfig
	tmdb        *TMDBScraper
	douban      *DoubanScraper
	posterCache *PosterCache
	scanner     *Scanner
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewUnifiedScraper 创建统一刮削器
func NewUnifiedScraper(config *ScraperConfig, cache *Cache) *UnifiedScraper {
	if config == nil {
		config = DefaultScraperConfig()
	}

	// 创建HTTP客户端（跳过证书验证，某些网站需要）
	httpClient := &http.Client{
		Timeout: config.DownloadTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 创建TMDB刮削器
	tmdbScraper := NewTMDBScraper(TMDBConfig{
		APIKey:  config.TMDBAPIKey,
		BaseURL: "https://api.themoviedb.org/3",
		Timeout: config.DownloadTimeout,
	}, NewEnhancedCache())

	// 创建豆瓣刮削器
	doubanScraper := NewDoubanScraper(DoubanConfig{
		APIKey:  config.DoubanAPIKey,
		Timeout: config.DownloadTimeout,
	}, cache)

	// 创建海报缓存
	posterCache := NewPosterCache(config.PosterDir)

	return &UnifiedScraper{
		config:      config,
		tmdb:        tmdbScraper,
		douban:      doubanScraper,
		posterCache: posterCache,
		httpClient:  httpClient,
	}
}

// SetScanner 设置扫描器
func (s *UnifiedScraper) SetScanner(scanner *Scanner) {
	s.scanner = scanner
}

// ScrapeMedia 刮削媒体元数据（自动识别电影/电视剧）
func (s *UnifiedScraper) ScrapeMedia(ctx context.Context, filename string, hints ...ScrapeHint) (*UnifiedMetadata, error) {
	// 解析文件名
	title, year, season, episode := s.parseFilename(filename)

	// 应用提示
	mediaType := s.detectMediaType(filename, hints...)
	if len(hints) > 0 {
		if hints[0].MediaType != "" {
			mediaType = hints[0].MediaType
		}
		if hints[0].Title != "" {
			title = hints[0].Title
		}
		if hints[0].Year > 0 {
			year = hints[0].Year
		}
	}

	// 根据类型刮削
	switch mediaType {
	case MediaTypeTVShow:
		return s.scrapeTVShow(ctx, title, season, episode)
	case MediaTypeMovie:
		return s.scrapeMovie(ctx, title, year)
	default:
		// 尝试先作为电影刮削
		meta, err := s.scrapeMovie(ctx, title, year)
		if err == nil {
			return meta, nil
		}
		// 失败则尝试电视剧
		return s.scrapeTVShow(ctx, title, season, episode)
	}
}

// ScrapeHint 刮削提示
type ScrapeHint struct {
	MediaType MediaType
	Title     string
	Year      int
	Season    int
	Episode   int
}

// scrapeMovie 刮削电影
func (s *UnifiedScraper) scrapeMovie(ctx context.Context, title string, year int) (*UnifiedMetadata, error) {
	var result *UnifiedMetadata
	var lastErr error

	// 按优先级尝试
	for _, provider := range s.config.ProviderPriority {
		switch provider {
		case "tmdb":
			meta, err := s.tmdb.SearchMovie(ctx, title, year)
			if err == nil && meta != nil {
				result = &UnifiedMetadata{
					MediaMetadata: *meta,
					Source:        "tmdb",
				}
				// 下载海报
				if meta.PosterPath != "" {
					posterPath, _ := s.DownloadPoster(ctx, meta.PosterPath, fmt.Sprintf("movie_%d", meta.TMDBID))
					result.LocalPosterPath = posterPath
				}
				return result, nil
			}
			lastErr = err

		case "douban":
			meta, err := s.douban.SearchMovie(ctx, title, year)
			if err == nil && meta != nil {
				result = &UnifiedMetadata{
					MediaMetadata: MediaMetadata{
						ID:            meta.ID,
						Title:         meta.Title,
						OriginalTitle: meta.OriginalTitle,
						Overview:      meta.Overview,
						Rating:        meta.Rating,
						VoteCount:     meta.VoteCount,
						ReleaseDate:   meta.ReleaseDate,
						Genres:        meta.Genres,
						Directors:     meta.Directors,
						PosterPath:    meta.PosterPath,
					},
					Source: "douban",
				}
				// 下载海报
				if meta.PosterPath != "" {
					posterPath, _ := s.DownloadPoster(ctx, meta.PosterPath, fmt.Sprintf("douban_%s", meta.ID))
					result.LocalPosterPath = posterPath
				}
				return result, nil
			}
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("所有数据源都失败: %w", lastErr)
	}
	return nil, fmt.Errorf("未找到电影: %s (%d)", title, year)
}

// scrapeTVShow 刮削电视剧
func (s *UnifiedScraper) scrapeTVShow(ctx context.Context, title string, season, episode int) (*UnifiedMetadata, error) {
	var result *UnifiedMetadata
	var lastErr error

	for _, provider := range s.config.ProviderPriority {
		switch provider {
		case "tmdb":
			meta, err := s.tmdb.SearchTVShow(ctx, title)
			if err == nil && meta != nil {
				result = &UnifiedMetadata{
					MediaMetadata: meta.MediaMetadata,
					TVShowData:    meta,
					Season:        season,
					Episode:       episode,
					Source:        "tmdb",
				}
				// 下载海报
				if meta.PosterPath != "" {
					posterPath, _ := s.DownloadPoster(ctx, meta.PosterPath, fmt.Sprintf("tv_%d", meta.TMDBID))
					result.LocalPosterPath = posterPath
				}
				return result, nil
			}
			lastErr = err

		case "douban":
			meta, err := s.douban.SearchTVShow(ctx, title)
			if err == nil && meta != nil {
				result = &UnifiedMetadata{
					MediaMetadata: MediaMetadata{
						ID:            meta.ID,
						Title:         meta.Name,
						OriginalTitle: meta.OriginalName,
						Overview:      meta.Overview,
						Rating:        meta.Rating,
						VoteCount:     meta.VoteCount,
						ReleaseDate:   meta.FirstAirDate,
						Genres:        meta.Genres,
						PosterPath:    meta.PosterPath,
					},
					Season:  season,
					Episode: episode,
					Source:  "douban",
				}
				// 下载海报
				if meta.PosterPath != "" {
					posterPath, _ := s.DownloadPoster(ctx, meta.PosterPath, fmt.Sprintf("douban_tv_%s", meta.ID))
					result.LocalPosterPath = posterPath
				}
				return result, nil
			}
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("所有数据源都失败: %w", lastErr)
	}
	return nil, fmt.Errorf("未找到电视剧: %s", title)
}

// DownloadPoster 下载海报到本地
func (s *UnifiedScraper) DownloadPoster(ctx context.Context, posterURL, filename string) (string, error) {
	if posterURL == "" {
		return "", fmt.Errorf("海报URL为空")
	}

	// 检查缓存
	if s.posterCache != nil {
		if localPath, ok := s.posterCache.Get(filename); ok {
			return localPath, nil
		}
	}

	// 构建完整URL
	fullURL := posterURL
	if strings.HasPrefix(posterURL, "/") {
		// TMDB 相对路径
		fullURL = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", s.config.PosterSize, posterURL)
	}

	// 下载图片
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 读取图片数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片失败: %w", err)
	}

	// 确定文件扩展名
	ext := ".jpg"
	contentType := resp.Header.Get("Content-Type")
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}

	// 保存到本地
	localPath := filepath.Join(s.config.PosterDir, filename+ext)
	if err := os.MkdirAll(s.config.PosterDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	// 更新缓存
	if s.posterCache != nil {
		s.posterCache.Set(filename, localPath)
	}

	return localPath, nil
}

// parseFilename 解析文件名
func (s *UnifiedScraper) parseFilename(filename string) (title string, year, season, episode int) {
	if s.scanner != nil {
		return s.scanner.ParseFilename(filename)
	}

	// 内置解析逻辑
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 预处理：移除常见的视频质量标识
	qualityPatterns := []string{
		`(?i)\b(720p|1080p|2160p|4K|HDR|SDR|HDTV|WEB-?DL|BluRay|BDRip|DVDRip|CAM|TS|HDTC|WEBRip)\b`,
		`(?i)\b(x264|x265|h\.?264|h\.?265|HEVC|AVC|AV1|AAC|DD5\.?1|DDP5\.?1|DTS-HD|DTS|TrueHD|Atmos)\b`,
		`(?i)\b(NF|AMZN|HMAX|APTV|DSNP|HULU|iTunes|MA)\b`, // 发行商标识
		`(?i)\b(RARBG|YIFY|SPARKS|MARS|FGT|TBS|BOOM)\b`,   // 发布组
		`(?i)\b(REPACK|PROPER|EXTENDED|UNCUT|REMUX)\b`,    // 版本标识
	}

	for _, pattern := range qualityPatterns {
		re := regexp.MustCompile(pattern)
		name = re.ReplaceAllString(name, " ")
	}

	// 清理多余空格
	spaceRe := regexp.MustCompile(`\s+`)
	name = spaceRe.ReplaceAllString(strings.TrimSpace(name), " ")

	// 提取季集信息
	seasonEpisodePattern := regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,3})`)
	if matches := seasonEpisodePattern.FindStringSubmatch(name); len(matches) == 3 {
		season = parseIntSafe(matches[1])
		episode = parseIntSafe(matches[2])
		name = seasonEpisodePattern.ReplaceAllString(name, "")
	}

	// 提取年份
	yearPattern := regexp.MustCompile(`[\(\[\.]?(19\d{2}|20[0-2]\d)[\)\]\.]?`)
	if matches := yearPattern.FindStringSubmatch(name); len(matches) == 2 {
		year = parseIntSafe(matches[1])
		name = yearPattern.ReplaceAllString(name, "")
	}

	// 清理标题
	title = cleanTitleString(name)
	return title, year, season, episode
}

// detectMediaType 检测媒体类型
func (s *UnifiedScraper) detectMediaType(filename string, hints ...ScrapeHint) MediaType {
	if len(hints) > 0 && hints[0].MediaType != "" {
		return hints[0].MediaType
	}

	if s.scanner != nil {
		return s.scanner.DetectMediaType(filename)
	}

	// 内置检测逻辑
	name := strings.ToLower(filename)

	// 电视剧特征
	tvPatterns := []string{
		"s01e", "s02e", "s03e", "s1e", "s2e",
		"season", "episode", "ep.", "hdtv", "web-dl",
		".hdtv.", ".webdl.", ".web-dl.",
	}

	for _, p := range tvPatterns {
		if strings.Contains(name, p) {
			return MediaTypeTVShow
		}
	}

	// 检测季集模式
	seasonEpisodePattern := regexp.MustCompile(`(?i)[sS]\d{1,2}[eE]\d{1,3}`)
	if seasonEpisodePattern.MatchString(name) {
		return MediaTypeTVShow
	}

	// 年份模式（电影常见）
	yearPattern := regexp.MustCompile(`[\(\.](19\d{2}|20\d{2})[\)\.]`)
	if yearPattern.MatchString(name) {
		return MediaTypeMovie
	}

	return MediaTypeUnknown
}

// ====== 豆瓣刮削器 ======

// DoubanScraper 豆瓣刮削器
type DoubanScraper struct {
	config     DoubanConfig
	httpClient *http.Client
	cache      *Cache
}

// DoubanConfig 豆瓣配置
type DoubanConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// NewDoubanScraper 创建豆瓣刮削器
func NewDoubanScraper(config DoubanConfig, cache *Cache) *DoubanScraper {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.douban.com/v2"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &DoubanScraper{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache: cache,
	}
}

// SearchMovie 搜索电影
func (s *DoubanScraper) SearchMovie(ctx context.Context, title string, year int) (*DoubanMovieResult, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("douban:movie:%s:%d", title, year)
	if s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if m, ok := meta.(*DoubanMovieResult); ok {
				return m, nil
			}
		}
	}

	// 构建搜索URL
	params := url.Values{}
	params.Set("q", title)
	if s.config.APIKey != "" {
		params.Set("apikey", s.config.APIKey)
	}

	searchURL := fmt.Sprintf("%s/movie/search?%s", s.config.BaseURL, params.Encode())

	resp, err := s.makeRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result doubanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Subjects) == 0 {
		return nil, fmt.Errorf("未找到电影: %s", title)
	}

	// 查找最匹配的结果
	var bestMatch *doubanSubject
	for i := range result.Subjects {
		subject := &result.Subjects[i]
		if year > 0 && subject.Year == fmt.Sprintf("%d", year) {
			bestMatch = subject
			break
		}
		if bestMatch == nil {
			bestMatch = subject
		}
	}

	meta := s.convertSubjectToMovie(bestMatch)

	// 缓存结果
	if s.cache != nil {
		s.cache.SetMetadata(cacheKey, meta)
	}

	return meta, nil
}

// SearchTVShow 搜索电视剧
func (s *DoubanScraper) SearchTVShow(ctx context.Context, title string) (*DoubanTVResult, error) {
	cacheKey := fmt.Sprintf("douban:tv:%s", title)
	if s.cache != nil {
		if meta, ok := s.cache.GetMetadata(cacheKey); ok {
			if m, ok := meta.(*DoubanTVResult); ok {
				return m, nil
			}
		}
	}

	params := url.Values{}
	params.Set("q", title)
	if s.config.APIKey != "" {
		params.Set("apikey", s.config.APIKey)
	}

	searchURL := fmt.Sprintf("%s/tv/search?%s", s.config.BaseURL, params.Encode())

	resp, err := s.makeRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result doubanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Subjects) == 0 {
		return nil, fmt.Errorf("未找到电视剧: %s", title)
	}

	subject := &result.Subjects[0]
	meta := s.convertSubjectToTV(subject)

	if s.cache != nil {
		s.cache.SetMetadata(cacheKey, meta)
	}

	return meta, nil
}

// makeRequest 发送HTTP请求
func (s *DoubanScraper) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	return s.httpClient.Do(req)
}

// convertSubjectToMovie 转换豆瓣数据到电影结果
func (s *DoubanScraper) convertSubjectToMovie(subject *doubanSubject) *DoubanMovieResult {
	directors := make([]string, 0, len(subject.Directors))
	for _, d := range subject.Directors {
		directors = append(directors, d.Name)
	}

	cast := make([]Cast, 0, len(subject.Casts))
	for _, c := range subject.Casts {
		cast = append(cast, Cast{
			Name:        c.Name,
			ProfilePath: c.Avatar,
		})
	}

	return &DoubanMovieResult{
		ID:            subject.ID,
		Title:         subject.Title,
		OriginalTitle: subject.OriginalTitle,
		Overview:      subject.Summary,
		Rating:        subject.Rating.Average,
		VoteCount:     subject.Rating.NumRaters,
		ReleaseDate:   subject.Year,
		Genres:        subject.Genres,
		Directors:     directors,
		Cast:          cast,
		PosterPath:    subject.Images.Large,
		Source:        "douban",
	}
}

// convertSubjectToTV 转换豆瓣数据到电视剧结果
func (s *DoubanScraper) convertSubjectToTV(subject *doubanSubject) *DoubanTVResult {
	cast := make([]Cast, 0, len(subject.Casts))
	for _, c := range subject.Casts {
		cast = append(cast, Cast{
			Name:        c.Name,
			ProfilePath: c.Avatar,
		})
	}

	return &DoubanTVResult{
		ID:           subject.ID,
		Name:         subject.Title,
		OriginalName: subject.OriginalTitle,
		Overview:     subject.Summary,
		Rating:       subject.Rating.Average,
		VoteCount:    subject.Rating.NumRaters,
		FirstAirDate: subject.Year,
		Genres:       subject.Genres,
		Cast:         cast,
		PosterPath:   subject.Images.Large,
		Seasons:      subject.SeasonsCount,
		Episodes:     subject.EpisodesCount,
		Source:       "douban",
	}
}

// ====== 数据结构 ======

// UnifiedMetadata 统一元数据结果
type UnifiedMetadata struct {
	MediaMetadata
	TVShowData      *TVShowMetadata `json:"tv_show_data,omitempty"`
	Season          int             `json:"season,omitempty"`
	Episode         int             `json:"episode,omitempty"`
	Source          string          `json:"source"`
	LocalPosterPath string          `json:"local_poster_path,omitempty"`
}

// DoubanMovieResult 豆瓣电影结果
type DoubanMovieResult struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title"`
	Overview      string    `json:"overview"`
	Rating        float64   `json:"rating"`
	VoteCount     int       `json:"vote_count"`
	ReleaseDate   string    `json:"release_date"`
	Runtime       int       `json:"runtime"`
	Genres        []string  `json:"genres"`
	Directors     []string  `json:"directors"`
	Cast          []Cast    `json:"cast"`
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	Source        string    `json:"source"`
	ScrapedAt     time.Time `json:"scraped_at"`
}

// DoubanTVResult 豆瓣电视剧结果
type DoubanTVResult struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name"`
	Overview     string    `json:"overview"`
	Rating       float64   `json:"rating"`
	VoteCount    int       `json:"vote_count"`
	FirstAirDate string    `json:"first_air_date"`
	Genres       []string  `json:"genres"`
	Cast         []Cast    `json:"cast"`
	PosterPath   string    `json:"poster_path"`
	Seasons      int       `json:"seasons"`
	Episodes     int       `json:"episodes"`
	Source       string    `json:"source"`
	ScrapedAt    time.Time `json:"scraped_at"`
}

// ====== 海报缓存 ======

// PosterCache 海报缓存
type PosterCache struct {
	dir   string
	cache map[string]string
	mu    sync.RWMutex
}

// NewPosterCache 创建海报缓存
func NewPosterCache(dir string) *PosterCache {
	return &PosterCache{
		dir:   dir,
		cache: make(map[string]string),
	}
}

// Get 获取缓存的海报路径
func (c *PosterCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	path, ok := c.cache[key]
	return path, ok
}

// Set 设置缓存
func (c *PosterCache) Set(key, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = path
}

// ====== 豆瓣API响应结构 ======

type doubanSearchResponse struct {
	Count    int             `json:"count"`
	Start    int             `json:"start"`
	Total    int             `json:"total"`
	Subjects []doubanSubject `json:"subjects"`
}

type doubanSubject struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Summary       string `json:"summary"`
	Year          string `json:"year"`
	Rating        struct {
		Average   float64 `json:"average"`
		NumRaters int     `json:"numRaters"`
	} `json:"rating"`
	Genres []string `json:"genres"`
	Images struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
	} `json:"images"`
	Directors []struct {
		Name string `json:"name"`
	} `json:"directors"`
	Casts []struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"casts"`
	SeasonsCount  int `json:"seasons_count"`
	EpisodesCount int `json:"episodes_count"`
}

// ====== 辅助函数 ======

// parseIntSafeSafe 安全解析整数（增强版）
func parseIntSafeSafe(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// cleanTitleString 清理标题字符串
func cleanTitleString(title string) string {
	// 替换常见分隔符
	result := strings.NewReplacer(
		".", " ",
		"_", " ",
		"-", " ",
		"  ", " ",
	).Replace(title)

	// 移除特殊字符（保留中文、字母、数字、空格）
	// Go 正则不支持 \uXXXX，使用 \p{Han} 匹配中文字符
	reg := regexp.MustCompile(`[^\p{Han}\p{L}\p{N}\s]`)
	result = reg.ReplaceAllString(result, "")

	return strings.TrimSpace(result)
}

// ====== 批量刮削 ======

// BatchScrapeResult 批量刮削结果
type BatchScrapeResult struct {
	Total    int                `json:"total"`
	Success  int                `json:"success"`
	Failed   int                `json:"failed"`
	Results  []*UnifiedMetadata `json:"results"`
	Errors   []BatchScrapeError `json:"errors,omitempty"`
	Duration time.Duration      `json:"duration"`
}

// BatchScrapeError 批量刮削错误
type BatchScrapeError struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

// BatchScrape 批量刮削
func (s *UnifiedScraper) BatchScrape(ctx context.Context, files []string) *BatchScrapeResult {
	start := time.Now()
	result := &BatchScrapeResult{
		Total:   len(files),
		Results: make([]*UnifiedMetadata, 0, len(files)),
		Errors:  make([]BatchScrapeError, 0),
	}

	for _, file := range files {
		filename := filepath.Base(file)
		meta, err := s.ScrapeMedia(ctx, filename)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchScrapeError{
				Filename: filename,
				Error:    err.Error(),
			})
			continue
		}

		result.Success++
		meta.ID = file // 使用文件路径作为ID
		result.Results = append(result.Results, meta)
	}

	result.Duration = time.Since(start)
	return result
}

// ====== 智能匹配 ======

// SmartMatch 智能匹配媒体信息
// 通过文件名自动识别并刮削元数据
func (s *UnifiedScraper) SmartMatch(ctx context.Context, filePath string) (*UnifiedMetadata, error) {
	filename := filepath.Base(filePath)

	// 解析文件名
	title, year, season, episode := s.parseFilename(filename)

	// 检测媒体类型
	mediaType := s.detectMediaType(filename)

	// 根据类型刮削
	switch mediaType {
	case MediaTypeTVShow:
		meta, err := s.scrapeTVShow(ctx, title, season, episode)
		if err != nil {
			return nil, fmt.Errorf("刮削电视剧失败: %w", err)
		}
		return meta, nil

	case MediaTypeMovie:
		meta, err := s.scrapeMovie(ctx, title, year)
		if err != nil {
			return nil, fmt.Errorf("刮削电影失败: %w", err)
		}
		return meta, nil

	default:
		// 先尝试电影
		meta, err := s.scrapeMovie(ctx, title, year)
		if err == nil {
			return meta, nil
		}

		// 再尝试电视剧
		tvMeta, err := s.scrapeTVShow(ctx, title, season, episode)
		if err != nil {
			return nil, fmt.Errorf("无法识别媒体类型: %s", filename)
		}
		return tvMeta, nil
	}
}

// ====== 海报下载队列 ======

// PosterDownloadQueue 海报下载队列
type PosterDownloadQueue struct {
	items    []posterDownloadItem
	parallel int
	mu       sync.Mutex
}

type posterDownloadItem struct {
	URL      string
	Filename string
	Callback func(string, error)
}

// NewPosterDownloadQueue 创建下载队列
func NewPosterDownloadQueue(parallel int) *PosterDownloadQueue {
	return &PosterDownloadQueue{
		items:    make([]posterDownloadItem, 0),
		parallel: parallel,
	}
}

// Add 添加下载任务
func (q *PosterDownloadQueue) Add(url, filename string, callback func(string, error)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, posterDownloadItem{
		URL:      url,
		Filename: filename,
		Callback: callback,
	})
}

// Process 处理队列
func (q *PosterDownloadQueue) Process(ctx context.Context, scraper *UnifiedScraper) {
	q.mu.Lock()
	items := make([]posterDownloadItem, len(q.items))
	copy(items, q.items)
	q.items = q.items[:0]
	q.mu.Unlock()

	var wg sync.WaitGroup
	sem := make(chan struct{}, q.parallel)

	for _, item := range items {
		wg.Add(1)
		go func(i posterDownloadItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			path, err := scraper.DownloadPoster(ctx, i.URL, i.Filename)
			if i.Callback != nil {
				i.Callback(path, err)
			}
		}(item)
	}

	wg.Wait()
}
