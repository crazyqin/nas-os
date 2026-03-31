// Package media provides intelligent metadata scraping with high recognition rate
// Inspired by fnOS 99% recognition rate approach
package media

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SmartScraper 智能刮削器 - 高识别率设计
// 核心策略：
// 1. 多源聚合（TMDB + 豆瓣 + IMDb + 本地数据库）
// 2. 智能文件名解析（中文/英文/混合/特殊格式）
// 3. 模糊匹配（拼写容错、别名匹配）
// 4. 目录结构推断（根据文件夹层级推断类型）
// 5. 缓存优化（减少重复请求）
type SmartScraper struct {
	tmdb       *TMDBScraper
	douban     *DoubanScraper
	cache      *SmartCache
	matcher    *FuzzyMatcher
	config     SmartScraperConfig
	httpClient HTTPClient
	mu         sync.RWMutex
}

// HTTPClient HTTP客户端接口
type HTTPClient interface {
	Do(req interface{}) (interface{}, error)
}

// SmartScraperConfig 智能刮削器配置
type SmartScraperConfig struct {
	// 数据源优先级
	SourcePriority []string `json:"sourcePriority"` // ["tmdb", "douban", "imdb"]

	// TMDB配置
	TMDBAPIKey string `json:"tmdbApiKey"`
	TMDBLang   string `json:"tmdbLang"` // zh-CN, en-US

	// 豆瓣配置（可选，豆瓣API已关闭公开接口，使用爬虫或代理）
	DoubanEnabled bool   `json:"doubanEnabled"`
	DoubanProxy   string `json:"doubanProxy"` // 代理地址

	// IMDb配置
	IMDbEnabled bool `json:"imdbEnabled"`

	// 智能匹配配置
	FuzzyMatchEnabled  bool    `json:"fuzzyMatchEnabled"`  // 启用模糊匹配
	FuzzyMatchThreshold float64 `json:"fuzzyMatchThreshold"` // 模糊匹配阈值 0.7-0.95

	// 缓存配置
	CacheEnabled bool          `json:"cacheEnabled"`
	CacheTTL     time.Duration `json:"cacheTtl"`

	// 重试配置
	MaxRetries    int           `json:"maxRetries"`
	RetryDelay    time.Duration `json:"retryDelay"`
	RequestTimeout time.Duration `json:"requestTimeout"`

	// 下载海报
	DownloadPosters  bool   `json:"downloadPosters"`
	PosterDir        string `json:"posterDir"`
	PosterSize       string `json:"posterSize"` // w200, w500, original

	// 中文偏好
	PreferChineseTitle bool `json:"preferChineseTitle"`
	PreferChineseOverview bool `json:"preferChineseOverview"`
}

// DefaultSmartScraperConfig 默认配置
func DefaultSmartScraperConfig() SmartScraperConfig {
	return SmartScraperConfig{
		SourcePriority:      []string{"tmdb", "douban"},
		TMDBLang:            "zh-CN",
		DoubanEnabled:       true,
		FuzzyMatchEnabled:   true,
		FuzzyMatchThreshold: 0.75,
		CacheEnabled:        true,
		CacheTTL:            7 * 24 * time.Hour, // 7天缓存
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
		RequestTimeout:      30 * time.Second,
		DownloadPosters:     true,
		PosterSize:          "w500",
		PreferChineseTitle:  true,
	}
}

// NewSmartScraper 创建智能刮削器
func NewSmartScraper(config SmartScraperConfig) *SmartScraper {
	cache := NewSmartCache(config.CacheTTL)
	matcher := NewFuzzyMatcher(config.FuzzyMatchThreshold)

	return &SmartScraper{
		config:  config,
		cache:   cache,
		matcher: matcher,
	}
}

// SetTMDBScraper 设置TMDB刮削器
func (s *SmartScraper) SetTMDBScraper(tmdb *TMDBScraper) {
	s.tmdb = tmdb
}

// SetDoubanScraper 设置豆瓣刮削器
func (s *SmartScraper) SetDoubanScraper(douban *DoubanScraper) {
	s.douban = douban
}

// ScrapeFile 智能刮削单个文件 - 核心识别流程
func (s *SmartScraper) ScrapeFile(ctx context.Context, filePath string) (*ScrapeResult, error) {
	result := &ScrapeResult{
		FilePath: filePath,
		Attempts: make([]ScrapeAttempt, 0),
	}

	// Step 1: 深度解析文件名
	parsed := s.deepParseFilename(filePath)
	result.ParsedInfo = parsed

	// Step 2: 根据目录结构推断类型
	dirInferred := s.inferFromDirectory(filePath)
	if dirInferred.Type != MediaTypeUnknown {
		// 目录推断优先级更高
		if parsed.Type == MediaTypeUnknown || dirInferred.Confidence > parsed.Confidence {
			parsed.Type = dirInferred.Type
			parsed.Title = dirInferred.Title
			parsed.Confidence = dirInferred.Confidence
		}
	}

	// Step 3: 尝试从嵌入元数据获取信息（NFO文件等）
	nfoInfo := s.tryParseNFO(filepath.Dir(filePath), filepath.Base(filePath))
	if nfoInfo != nil && nfoInfo.TMDBID > 0 {
		// NFO文件提供的ID最可靠
		return s.scrapeByID(ctx, nfoInfo.TMDBID, parsed.Type, result)
	}

	// Step 4: 按优先级尝试各数据源
	for _, source := range s.config.SourcePriority {
		attempt := ScrapeAttempt{
			Source:    source,
			Timestamp: time.Now(),
		}

		var meta *MediaMetadata
		var err error

		switch source {
		case "tmdb":
			meta, err = s.scrapeFromTMDB(ctx, parsed)
		case "douban":
			meta, err = s.scrapeFromDouban(ctx, parsed)
		case "imdb":
			meta, err = s.scrapeFromIMDb(ctx, parsed)
		}

		if err != nil {
			attempt.Error = err.Error()
			result.Attempts = append(result.Attempts, attempt)
			continue
		}

		attempt.Success = true
		attempt.MatchScore = s.matcher.Score(parsed.Title, meta.Title)
		result.Attempts = append(result.Attempts, attempt)

		// Step 5: 验证匹配质量
		if attempt.MatchScore >= s.config.FuzzyMatchThreshold {
			result.Metadata = meta
			result.Source = source
			result.Confidence = attempt.MatchScore

			// 下载海报
			if s.config.DownloadPosters && meta.PosterPath != "" {
				s.downloadPosterAsync(ctx, meta)
			}

			// 缓存结果
			s.cacheResult(filePath, result)

			return result, nil
		}
	}

	// Step 6: 尝试模糊搜索（使用标题变体）
	if s.config.FuzzyMatchEnabled {
		variants := s.generateTitleVariants(parsed.Title)
		for _, variant := range variants {
			for _, source := range s.config.SourcePriority {
				attempt := ScrapeAttempt{
					Source:    source,
					Query:     variant,
					Timestamp: time.Now(),
				}

				var meta *MediaMetadata
				switch source {
				case "tmdb":
					meta, _ = s.scrapeFromTMDB(ctx, &ParsedInfo{
						Title:   variant,
						Year:    parsed.Year,
						Type:    parsed.Type,
					})
				case "douban":
					meta, _ = s.scrapeFromDouban(ctx, &ParsedInfo{
						Title:   variant,
						Year:    parsed.Year,
						Type:    parsed.Type,
					})
				}

				if meta != nil {
					score := s.matcher.Score(parsed.Title, meta.Title)
					attempt.Success = true
					attempt.MatchScore = score
					result.Attempts = append(result.Attempts, attempt)

					if score >= s.config.FuzzyMatchThreshold {
						result.Metadata = meta
						result.Source = source
						result.Confidence = score
						return result, nil
					}
				}
			}
		}
	}

	// 所有尝试都失败
	result.Error = "未能识别媒体"
	return result, fmt.Errorf("无法识别媒体: %s (尝试 %d 个数据源)", filePath, len(result.Attempts))
}

// deepParseFilename 深度解析文件名 - 支持各种命名格式
func (s *SmartScraper) deepParseFilename(filePath string) *ParsedInfo {
	filename := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	info := &ParsedInfo{
		OriginalFilename: filename,
		Type:             MediaTypeUnknown,
		Confidence:       0.5,
	}

	// 移除扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 预处理：移除常见杂质
	cleaned := s.cleanFilename(name)

	// 尝试多种解析模式
	patterns := []ParsePattern{
		// 标准格式: Title (Year) 或 Title.Year
		{Name: "standard", Pattern: regexp.MustCompile(`^(.+?)[\.\s\-\_]+(?:\((\d{4})\)|\.(\d{4})\.|[\.\s\-\_](\d{4})$)`)},
		// TV标准: Title S01E01
		{Name: "tv_standard", Pattern: regexp.MustCompile(`(?i)^(.+?)[\.\s\-\_]+[sS](\d{1,2})[eE](\d{1,3})`)},
		// TV变体: Title 1x01
		{Name: "tv_variant", Pattern: regexp.MustCompile(`(?i)^(.+?)[\.\s\-\_]+(\d{1,2})[xX](\d{1,3})`)},
		// 中文TV: 剧名 第X季 第Y集
		{Name: "tv_chinese", Pattern: regexp.MustCompile(`^([^\.\s]+)[\.\s\-\_]*第\s*([一二三四五六七八九十百零\d]+)\s*季\s*第\s*([一二三四五六七八九十百零\d]+)\s*集`)},
		// 剧场版/特别篇: Title 剧场版 / SP
		{Name: "special", Pattern: regexp.MustCompile(`^([^\.\s]+)[\.\s\-\_]*(剧场版|特别篇|SP|OAD|OVA)`)},
		// 年份标签: Title 2024 或 Title.2024
		{Name: "year_suffix", Pattern: regexp.MustCompile(`^(.+?)[\.\s\-\_](\d{4})$`)},
		// 简单格式: 纯标题
		{Name: "simple", Pattern: regexp.MustCompile(`^(.+)$`)},
	}

	for _, pp := range patterns {
		matches := pp.Pattern.FindStringSubmatch(cleaned)
		if len(matches) > 1 {
			switch pp.Name {
			case "tv_standard", "tv_variant":
				info.Title = cleanTitle(matches[1])
				info.Season = parseSeasonOrEpisode(matches[2])
				info.Episode = parseSeasonOrEpisode(matches[3])
				info.Type = MediaTypeTVShow
				info.Confidence = 0.95
				return info

			case "tv_chinese":
				info.Title = matches[1]
				info.Season = chineseToInt(matches[2])
				info.Episode = chineseToInt(matches[3])
				info.Type = MediaTypeTVShow
				info.Confidence = 0.95
				return info

			case "special":
				info.Title = matches[1]
				info.SpecialType = matches[2]
				info.Type = MediaTypeMovie // 剧场版归为电影
				info.Confidence = 0.85
				return info

			case "standard", "year_suffix":
				info.Title = cleanTitle(matches[1])
				// 年份在matches[2]或matches[3]
				for i := 2; i < len(matches); i++ {
					if matches[i] != "" && len(matches[i]) == 4 {
						info.Year = parseInt(matches[i])
						break
					}
				}
				info.Type = MediaTypeMovie
				info.Confidence = 0.9
				return info

			case "simple":
				info.Title = cleanTitle(matches[1])
				info.Confidence = 0.6
			}
			break
		}
	}

	// 检测剩余内容中的季集信息
	tvPatterns := []struct {
		Pattern *regexp.Regexp
		GetFunc func(matches []string) (season, episode int)
	}{
		{regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,3})`), func(m []string) (int, int) { return parseInt(m[1]), parseInt(m[2]) }},
		{regexp.MustCompile(`(?i)[eE][pP]?(\d{1,3})`), func(m []string) (int, int) { return 1, parseInt(m[1]) }},
		{regexp.MustCompile(`第\s*([一二三四五六七八九十百零\d]+)\s*季\s*第\s*([一二三四五六七八九十百零\d]+)\s*集`), func(m []string) (int, int) { return chineseToInt(m[1]), chineseToInt(m[2]) }},
	}

	for _, tp := range tvPatterns {
		if matches := tp.Pattern.FindStringSubmatch(cleaned); len(matches) >= 2 {
			info.Season, info.Episode = tp.GetFunc(matches)
			if info.Episode > 0 {
				info.Type = MediaTypeTVShow
				info.Confidence = math.Max(info.Confidence, 0.85)
				// 移除季集信息
				cleaned = tp.Pattern.ReplaceAllString(cleaned, "")
				info.Title = cleanTitle(cleaned)
			}
			break
		}
	}

	// 根据目录名推断（辅助信息）
	if info.Confidence < 0.8 {
		dirInfo := s.inferFromDirectory(dir)
		if dirInfo.Type != MediaTypeUnknown && dirInfo.Confidence > info.Confidence {
			info.Type = dirInfo.Type
			if dirInfo.Title != "" && len(info.Title) < 3 {
				info.Title = dirInfo.Title
			}
			info.Confidence = math.Max(info.Confidence, dirInfo.Confidence * 0.8)
		}
	}

	return info
}

// cleanFilename 清理文件名中的杂质
func (s *SmartScraper) cleanFilename(name string) string {
	// 移除编码/质量标签
	cleanupPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(1080p|720p|2160p|4k|uhd|hd|hdr|sdr|dvd|bluray|bdrip|brrip|webrip|web-dl|hdtv|cam|ts|hdrip)\b`),
		regexp.MustCompile(`(?i)\b(x264|x265|h264|h265|hevc|avc|av1|vp9)\b`),
		regexp.MustCompile(`(?i)\b(aac|ac3|dts|dd|ddp|truehd|atmos|flac|mp3)\b`),
		regexp.MustCompile(`(?i)\b(remux|proper|repack|extended|uncut|final|limited|unrated)\b`),
		regexp.MustCompile(`(?i)\b(dual|multi|sub|subbed|hardsub|softsub)\b`),
		regexp.MustCompile(`(?i)\b(netflix|amazon|hbo|disney|apple|hulu|bbc)\b`),
		regexp.MustCompile(`(?i)\b(ntb|rarbg|yify|yts|fgt|sparks|mars|playnow)\b`),
		regexp.MustCompile(`(?i)\b[a-z]{2,5}-[a-z]{2,5}\b`), // 发布组标签如 WEB-DL-Group
		regexp.MustCompile(`(?i)\[\w+\]`),                  // 方括号标签
		regexp.MustCompile(`(?i)【.+?】`),                  // 中文方括号
		regexp.MustCompile(`(?i)完结|全集|待更|更新至|连载中`),
		regexp.MustCompile(`(?i)\d+(?:GB|MB|M)\b`), // 大小标记
		regexp.MustCompile(`\d+\.\d+\s*(?:fps|FPS)`), // 帧率标记
	}

	result := name
	for _, p := range cleanupPatterns {
		result = p.ReplaceAllString(result, "")
	}

	// 清理分隔符
	result = strings.NewReplacer(".", " ", "_", " ", "-", " ", "+", " ").Replace(result)
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// inferFromDirectory 从目录结构推断媒体信息
func (s *SmartScraper) inferFromDirectory(filePath string) *ParsedInfo {
	info := &ParsedInfo{Type: MediaTypeUnknown, Confidence: 0.3}

	// 分析目录层级
	// 常见结构:
	// /Movies/电影名 (年份)/电影名.mkv
	// /TV/剧名/Season 01/S01E01.mkv
	// /TV/剧名/S01E01.mkv

	dir := filepath.Dir(filePath)
	parts := strings.Split(filepath.Clean(dir), "/")

	// 倒序分析
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part == "" || part == "." || part == ".." {
			continue
		}

		// Season目录
		if strings.HasPrefix(strings.ToLower(part), "season ") ||
		   regexp.MustCompile(`(?i)^s\d{1,2}$`).MatchString(part) {
			info.Type = MediaTypeTVShow
			info.Confidence = 0.9
			// 季号
			matches := regexp.MustCompile(`(?i)(?:season\s*|s)(\d{1,2})`).FindStringSubmatch(part)
			if len(matches) > 1 {
				info.Season = parseInt(matches[1])
			}
			continue
		}

		// 电影目录（带年份）
		yearMatch := regexp.MustCompile(`\((\d{4})\)|\.(\d{4})$`).FindStringSubmatch(part)
		if len(yearMatch) > 1 {
			year := 0
			for j := 1; j < len(yearMatch); j++ {
				if yearMatch[j] != "" {
					year = parseInt(yearMatch[j])
					break
				}
			}
			if year >= 1900 && year <= time.Now().Year()+1 {
				// 移除年份部分得到标题
				title := regexp.MustCompile(`[\.\s\-\_]+\(?(\d{4})\)?`).ReplaceAllString(part, "")
				title = strings.TrimSpace(title)
				info.Title = title
				info.Year = year
				info.Type = MediaTypeMovie
				info.Confidence = 0.85
				return info
			}
		}

		// 简单剧名目录
		if info.Type == MediaTypeTVShow && info.Title == "" {
			info.Title = part
			info.Confidence = math.Max(info.Confidence, 0.7)
		}
	}

	return info
}

// tryParseNFO 尝试解析NFO文件（Kodi/Jellyfin标准）
func (s *SmartScraper) tryParseNFO(dir, filename string) *NFOInfo {
	// 常见NFO文件名
	nfoNames := []string{
		filename + ".nfo",
		strings.TrimSuffix(filename, filepath.Ext(filename)) + ".nfo",
		"movie.nfo",
		"tvshow.nfo",
	}

	for _, nfoName := range nfoNames {
		nfoPath := filepath.Join(dir, nfoName)
		data, err := readFile(nfoPath)
		if err != nil {
			continue
		}
		return parseNFOContent(data)
	}

	return nil
}

// generateTitleVariants 生成标题变体用于模糊匹配
func (s *SmartScraper) generateTitleVariants(title string) []string {
	variants := []string{title}

	// 移除中文标点
	cleaned := regexp.MustCompile(`[：；，。！？·\-（）【】「」『』〈〉《》]`).ReplaceAllString(title, " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned != title {
		variants = append(variants, cleaned)
	}

	// 英文转小写
	lower := strings.ToLower(title)
	if lower != title {
		variants = append(variants, lower)
	}

	// 移除数字（针对"XX 2"之类的续集）
	noNum := regexp.MustCompile(`\d+$`).ReplaceAllString(title, "")
	noNum = strings.TrimSpace(noNum)
	if noNum != title && len(noNum) > 2 {
		variants = append(variants, noNum)
	}

	// 尝试中英互换（如果有英文部分）
	englishPart := regexp.MustCompile(`[a-zA-Z\s]+`).FindString(title)
	if englishPart != "" && len(englishPart) > 2 {
		variants = append(variants, strings.TrimSpace(englishPart))
	}

	// 中文部分
	chinesePart := regexp.MustCompile(`[\p{Han}]+`).FindString(title)
	if chinesePart != "" && len(chinesePart) > 2 {
		variants = append(variants, chinesePart)
	}

	return variants
}

// scrapeFromTMDB 从TMDB刮削
func (s *SmartScraper) scrapeFromTMDB(ctx context.Context, info *ParsedInfo) (*MediaMetadata, error) {
	if s.tmdb == nil {
		return nil, fmt.Errorf("TMDB未配置")
	}

	switch info.Type {
	case MediaTypeTVShow:
		tv, err := s.tmdb.SearchTVShow(ctx, info.Title)
		if err != nil {
			return nil, err
		}
		return &tv.MediaMetadata, nil
	default:
		return s.tmdb.SearchMovie(ctx, info.Title, info.Year)
	}
}

// scrapeFromDouban 从豆瓣刮削
func (s *SmartScraper) scrapeFromDouban(ctx context.Context, info *ParsedInfo) (*MediaMetadata, error) {
	if s.douban == nil {
		return nil, fmt.Errorf("豆瓣未配置")
	}

	switch info.Type {
	case MediaTypeTVShow:
		tv, err := s.douban.SearchTVShow(ctx, info.Title)
		if err != nil {
			return nil, err
		}
		return &MediaMetadata{
			ID:            tv.ID,
			Title:         tv.Name,
			OriginalTitle: tv.OriginalName,
			Overview:      tv.Overview,
			Rating:        tv.Rating,
			VoteCount:     tv.VoteCount,
			ReleaseDate:   tv.FirstAirDate,
			Genres:        tv.Genres,
			PosterPath:    tv.PosterPath,
			ScrapedAt:     time.Now(),
		}, nil
	default:
		movie, err := s.douban.SearchMovie(ctx, info.Title, info.Year)
		if err != nil {
			return nil, err
		}
		return &MediaMetadata{
			ID:            movie.ID,
			Title:         movie.Title,
			OriginalTitle: movie.OriginalTitle,
			Overview:      movie.Overview,
			Rating:        movie.Rating,
			VoteCount:     movie.VoteCount,
			ReleaseDate:   movie.ReleaseDate,
			Genres:        movie.Genres,
			Directors:     movie.Directors,
			PosterPath:    movie.PosterPath,
			ScrapedAt:     time.Now(),
		}, nil
	}
}

// scrapeFromIMDb 从IMDb刮削（使用OMDb API或IMDb API）
func (s *SmartScraper) scrapeFromIMDb(ctx context.Context, info *ParsedInfo) (*MediaMetadata, error) {
	// IMDb API需要单独实现
	return nil, fmt.Errorf("IMDb刮削未实现")
}

// scrapeByID 通过ID直接刮削（最可靠）
func (s *SmartScraper) scrapeByID(ctx context.Context, tmdbID int, mediaType MediaType, result *ScrapeResult) (*ScrapeResult, error) {
	if s.tmdb == nil {
		return result, fmt.Errorf("TMDB未配置")
	}

	var meta *MediaMetadata
	var err error

	switch mediaType {
	case MediaTypeTVShow:
		tv, err := s.tmdb.GetTVShowDetails(ctx, tmdbID)
		if err != nil {
			return result, err
		}
		meta = &tv.MediaMetadata
	default:
		meta, err = s.tmdb.GetMovieDetails(ctx, tmdbID)
		if err != nil {
			return result, err
		}
	}

	result.Metadata = meta
	result.Source = "tmdb"
	result.Confidence = 1.0
	result.Attempts = append(result.Attempts, ScrapeAttempt{
		Source:    "tmdb",
		Query:     fmt.Sprintf("ID:%d", tmdbID),
		Success:   true,
		MatchScore: 1.0,
		Timestamp: time.Now(),
	})

	return result, nil
}

// downloadPosterAsync 异步下载海报
func (s *SmartScraper) downloadPosterAsync(ctx context.Context, meta *MediaMetadata) {
	go func() {
		if s.tmdb != nil && meta.PosterPath != "" {
			path, err := s.tmdb.DownloadPoster(ctx, meta.PosterPath, meta.Type, meta.TMDBID, s.config.PosterSize)
			if err == nil {
				meta.LocalPosterPath = path
			}
		}
	}()
}

// cacheResult 缓存刮削结果
func (s *SmartScraper) cacheResult(filePath string, result *ScrapeResult) {
	if s.config.CacheEnabled && s.cache != nil {
		s.cache.Set(filePath, result, s.config.CacheTTL)
	}
}

// BatchScrape 批量刮削 - 并行处理
func (s *SmartScraper) BatchScrape(ctx context.Context, files []string, workers int) *BatchScrapeResult {
	if workers <= 0 {
		workers = 5
	}

	result := &BatchScrapeResult{
		Total:   len(files),
		Results: make(map[string]*ScrapeResult),
		Errors:  make(map[string]string),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, workers)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sr, err := s.ScrapeFile(ctx, f)
			mu.Lock()
			if err != nil {
				result.Failed++
				result.Errors[f] = err.Error()
			} else {
				result.Success++
				result.Results[f] = sr
			}
			mu.Unlock()
		}(file)
	}

	wg.Wait()
	result.Duration = time.Since(result.StartTime)
	return result
}

// ====== 辅助类型和函数 ======

// ParsedInfo 解析后的文件信息
type ParsedInfo struct {
	OriginalFilename string
	Title            string
	Year             int
	Season           int
	Episode          int
	SpecialType      string // 剧场版、特别篇等
	Type             MediaType
	Confidence       float64
}

// ScrapeResult 刮削结果
type ScrapeResult struct {
	FilePath    string
	ParsedInfo  *ParsedInfo
	Metadata    *MediaMetadata
	Source      string
	Confidence  float64
	Attempts    []ScrapeAttempt
	Error       string
}

// ScrapeAttempt 刮削尝试记录
type ScrapeAttempt struct {
	Source    string
	Query     string
	Success   bool
	MatchScore float64
	Error     string
	Timestamp time.Time
}

// NFOInfo NFO文件信息
type NFOInfo struct {
	Title   string
	Year    int
	TMDBID  int
	IMDbID  string
	Type    MediaType
}

// BatchScrapeResultV2 批量刮削结果
type BatchScrapeResultV2 struct {
	Total     int
	Success   int
	Failed    int
	StartTime time.Time
	Duration  time.Duration
	Results   map[string]*ScrapeResult
	Errors    map[string]string
}

// parseNFOContent 解析NFO内容（简化版）
func parseNFOContent(data string) *NFOInfo {
	info := &NFOInfo{}

	// 提取title
	titleMatch := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(data)
	if len(titleMatch) > 1 {
		info.Title = titleMatch[1]
	}

	// 提取year
	yearMatch := regexp.MustCompile(`<year>(\d{4})</year>`).FindStringSubmatch(data)
	if len(yearMatch) > 1 {
		info.Year = parseInt(yearMatch[1])
	}

	// 提取tmdbid
	tmdbMatch := regexp.MustCompile(`<tmdbid>(\d+)</tmdbid>`).FindStringSubmatch(data)
	if len(tmdbMatch) > 1 {
		info.TMDBID = parseInt(tmdbMatch[1])
	}

	// 提取imdbid
	imdbMatch := regexp.MustCompile(`<imdb>(tt\d+)</imdb>|<imdbid>(tt\d+)</imdbid>`).FindStringSubmatch(data)
	if len(imdbMatch) > 1 {
		for i := 1; i < len(imdbMatch); i++ {
			if imdbMatch[i] != "" {
				info.IMDbID = imdbMatch[i]
				break
			}
		}
	}

	return info
}

// readFile 简化的文件读取
func readFile(path string) (string, error) {
	data, err := readFileBytes(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readFileBytes 占位函数
func readFileBytes(path string) ([]byte, error) {
	// 实际实现需要os.ReadFile
	return nil, fmt.Errorf("not implemented")
}

// ParsePattern 解析模式
type ParsePattern struct {
	Name    string
	Pattern *regexp.Regexp
}

// parseIntSafeV2 安全解析整数
func parseIntSafeV2(s string) int {
	s = strings.TrimSpace(s)
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result * 10 + int(c - '0')
		}
	}
	return result
}

// parseSeasonOrEpisode 解析季/集号（支持中文）
func parseSeasonOrEpisode(s string) int {
	s = strings.TrimSpace(s)
	// 先尝试中文
	if regexp.MustCompile(`[\p{Han}]`).MatchString(s) {
		return chineseToInt(s)
	}
	return parseInt(s)
}

// cleanTitleV2 清理标题
func cleanTitleV2(title string) string {
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	// 移除前后括号
	if strings.HasPrefix(title, "(") && strings.HasSuffix(title, ")") {
		title = strings.TrimPrefix(title, "(")
		title = strings.TrimSuffix(title, ")")
	}
	return title
}

// chineseToIntV2 中文数字转整数
func chineseToIntV2(s string) int {
	mapping := map[rune]int{
		'零': 0, '〇': 0, '一': 1, '二': 2, '三': 3, '四': 4,
		'五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
		'百': 100, '千': 1000,
	}

	result := 0
	temp := 0

	for _, c := range s {
		if val, ok := mapping[c]; ok {
			if val >= 10 {
				if temp == 0 {
					temp = 1
				}
				result += temp * val
				temp = 0
			} else {
				temp = val
			}
		}
	}

	return result + temp
}