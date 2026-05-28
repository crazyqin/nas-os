// Package hometheater 提供媒体扫描和刮削功能
package hometheater

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scanner 媒体扫描器.
type Scanner struct {
	mu           sync.RWMutex
	engine       *Engine
	scanPaths    []string
	scanning     bool
	lastScan     *ScanResult
	tmdbClient   *TMDBClient
	imdbClient   *IMDBClient
	scanPatterns []string
}

// TMDBClient TMDB API客户端.
type TMDBClient struct {
	APIKey  string
	BaseURL string
	Enabled bool
}

// IMDBClient IMDB API客户端.
type IMDBClient struct {
	APIKey  string
	BaseURL string
	Enabled bool
}

// TMDBMovieResult TMDB电影搜索结果.
type TMDBMovieResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Year          int     `json:"year"`
	Rating        float64 `json:"rating"`
	VoteCount     int     `json:"vote_count"`
	Overview      string  `json:"overview"`
	Genres        []string `json:"genres"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	Runtime       int     `json:"runtime"`
	Directors     []string `json:"directors"`
	Cast          []string `json:"cast"`
	IMDBID        string  `json:"imdb_id"`
}

// TMDBTVShowResult TMDB剧集搜索结果.
type TMDBTVShowResult struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginalName  string  `json:"original_name"`
	Year          int     `json:"year"`
	Rating        float64 `json:"rating"`
	VoteCount     int     `json:"vote_count"`
	Overview      string  `json:"overview"`
	Genres        []string `json:"genres"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FirstAirDate  string  `json:"first_air_date"`
	Network       string  `json:"network"`
	Status        string  `json:"status"`
	SeasonCount   int     `json:"season_count"`
	EpisodeCount  int     `json:"episode_count"`
	IMDBID        string  `json:"imdb_id"`
}

// SubtitleInfo 字幕信息.
type SubtitleInfo struct {
	Language string         `json:"language"`
	Format   SubtitleFormat `json:"format"`
	FilePath string         `json:"file_path"`
	Source   string         `json:"source"` // embedded/external
}

// NewScanner 创建媒体扫描器.
func NewScanner(engine *Engine) *Scanner {
	return &Scanner{
		engine: engine,
		tmdbClient: &TMDBClient{
			BaseURL: "https://api.themoviedb.org/3",
			Enabled: false,
		},
		imdbClient: &IMDBClient{
			BaseURL: "https://www.omdbapi.com",
			Enabled: false,
		},
		scanPatterns: []string{
			"*.mkv", "*.mp4", "*.avi", "*.mov", "*.wmv",
			"*.flv", "*.m4v", "*.ts", "*.webm",
		},
	}
}

// SetTMDBKey 设置TMDB API Key.
func (s *Scanner) SetTMDBKey(apiKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tmdbClient.APIKey = apiKey
	s.tmdbClient.Enabled = apiKey != ""
}

// SetIMDBKey 设置IMDB API Key.
func (s *Scanner) SetIMDBKey(apiKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imdbClient.APIKey = apiKey
	s.imdbClient.Enabled = apiKey != ""
}

// AddScanPath 添加扫描路径.
func (s *Scanner) AddScanPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanPaths = append(s.scanPaths, path)
}

// IsScanning 返回是否正在扫描.
func (s *Scanner) IsScanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanning
}

// GetLastScan 获取上次扫描结果.
func (s *Scanner) GetLastScan() *ScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastScan
}

// ScanLibrary 扫描媒体库.
func (s *Scanner) ScanLibrary(libraryID string) (*ScanResult, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrScanInProgress
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	lib, err := s.engine.GetLibrary(libraryID)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &ScanResult{
		LibraryID: libraryID,
		StartTime: startTime,
	}

	// 扫描目录
	err = s.scanDirectory(lib, result)
	if err != nil {
		return nil, err
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime).Seconds()

	s.mu.Lock()
	s.lastScan = result
	s.mu.Unlock()

	return result, nil
}

// scanDirectory 扫描目录.
func (s *Scanner) scanDirectory(lib *MediaLibrary, result *ScanResult) error {
	// 根据媒体库类型扫描
	switch lib.Type {
	case MediaTypeMovie:
		return s.scanMovies(lib, result)
	case MediaTypeTVShow:
		return s.scanTVShows(lib, result)
	default:
		return fmt.Errorf("不支持的媒体库类型: %s", lib.Type)
	}
}

// scanMovies 扫描电影目录.
func (s *Scanner) scanMovies(lib *MediaLibrary, result *ScanResult) error {
	// 模拟扫描过程
	// 实际实现会遍历目录、解析文件名、匹配TMDB等
	result.TotalFiles = 0
	return nil
}

// scanTVShows 扫描电视剧目录.
func (s *Scanner) scanTVShows(lib *MediaLibrary, result *ScanResult) error {
	// 模拟扫描过程
	result.TotalFiles = 0
	return nil
}

// ParseMovieName 解析电影文件名.
func (s *Scanner) ParseMovieName(filename string) (title string, year int) {
	// 移除扩展名
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// 尝试匹配年份模式 (YYYY)
	for i := 0; i < len(name)-4; i++ {
		if name[i] == '(' || name[i] == '.' || name[i] == ' ' {
			candidate := name[i+1 : i+5]
			if y := parseYear(candidate); y > 1900 && y <= time.Now().Year() {
				title = strings.TrimSpace(name[:i])
				year = y
				return
			}
		}
	}

	// 没有找到年份
	title = name
	return
}

// ParseEpisodeName 解析剧集文件名.
func (s *Scanner) ParseEpisodeName(filename string) (showName string, season, episode int, err error) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// 尝试匹配 S01E01 模式
	upper := strings.ToUpper(name)
	if idx := strings.Index(upper, "S"); idx >= 0 {
		for i := idx; i < len(upper)-5; i++ {
			if upper[i] == 'S' && i+2 < len(upper) {
				if s1, s2 := upper[i+1], upper[i+2]; s1 >= '0' && s1 <= '9' && s2 >= '0' && s2 <= '9' {
					if i+3 < len(upper) && upper[i+3] == 'E' {
						if i+5 < len(upper) {
							e1, e2 := upper[i+4], upper[i+5]
							if e1 >= '0' && e1 <= '9' && e2 >= '0' && e2 <= '9' {
								showName = strings.TrimSpace(name[:i])
								season = int(s1-'0')*10 + int(s2-'0')
								episode = int(e1-'0')*10 + int(e2-'0')
								return
							}
						}
					}
				}
			}
		}
	}

	// 尝试匹配 Exx 模式
	if idx := strings.Index(upper, "E"); idx > 0 && idx < len(upper)-2 {
		e1, e2 := upper[idx+1], upper[idx+2]
		if e1 >= '0' && e1 <= '9' && e2 >= '0' && e2 <= '9' {
			showName = strings.TrimSpace(name[:idx])
			season = 1
			episode = int(e1-'0')*10 + int(e2-'0')
			return
		}
	}

	return "", 0, 0, fmt.Errorf("无法解析剧集文件名: %s", filename)
}

// SearchTMDB 搜索TMDB.
func (s *Scanner) SearchTMDB(title string, mediaType MediaType) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.tmdbClient.Enabled {
		return nil, fmt.Errorf("TMDB未配置")
	}

	// 模拟TMDB搜索
	switch mediaType {
	case MediaTypeMovie:
		return &TMDBMovieResult{
			ID:    12345,
			Title: title,
			Year:  2024,
		}, nil
	case MediaTypeTVShow:
		return &TMDBTVShowResult{
			ID:   12345,
			Name: title,
			Year: 2024,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的媒体类型: %s", mediaType)
	}
}

// ScrapeMovieMetadata 刮削电影元数据.
func (s *Scanner) ScrapeMovieMetadata(movie *Movie) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.tmdbClient.Enabled && !s.imdbClient.Enabled {
		return nil
	}

	// 如果已有TMDB ID，直接获取详情
	if movie.TMDBID > 0 {
		return s.fetchMovieDetails(movie)
	}

	// 否则搜索
	result, err := s.SearchTMDB(movie.Title, MediaTypeMovie)
	if err != nil {
		return err
	}

	if tmdbResult, ok := result.(*TMDBMovieResult); ok {
		movie.TMDBID = tmdbResult.ID
		movie.IMDBID = tmdbResult.IMDBID
		movie.Overview = tmdbResult.Overview
		movie.Rating = tmdbResult.Rating
		movie.VoteCount = tmdbResult.VoteCount
		movie.Genres = tmdbResult.Genres
		movie.PosterPath = tmdbResult.PosterPath
		movie.BackdropPath = tmdbResult.BackdropPath
		movie.Directors = tmdbResult.Directors
		movie.Cast = tmdbResult.Cast
	}

	return nil
}

// ScrapeTVShowMetadata 刮削剧集元数据.
func (s *Scanner) ScrapeTVShowMetadata(show *TVShow) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.tmdbClient.Enabled {
		return nil
	}

	if show.TMDBID > 0 {
		return s.fetchTVShowDetails(show)
	}

	result, err := s.SearchTMDB(show.Title, MediaTypeTVShow)
	if err != nil {
		return err
	}

	if tmdbResult, ok := result.(*TMDBTVShowResult); ok {
		show.TMDBID = tmdbResult.ID
		show.IMDBID = tmdbResult.IMDBID
		show.Overview = tmdbResult.Overview
		show.Rating = tmdbResult.Rating
		show.VoteCount = tmdbResult.VoteCount
		show.Genres = tmdbResult.Genres
		show.PosterPath = tmdbResult.PosterPath
		show.BackdropPath = tmdbResult.BackdropPath
		show.Network = tmdbResult.Network
		show.Status = tmdbResult.Status
	}

	return nil
}

// fetchMovieDetails 获取电影详情.
func (s *Scanner) fetchMovieDetails(movie *Movie) error {
	// 模拟获取电影详情
	return nil
}

// fetchTVShowDetails 获取剧集详情.
func (s *Scanner) fetchTVShowDetails(show *TVShow) error {
	// 模拟获取剧集详情
	return nil
}

// FindSubtitles 查找字幕文件.
func (s *Scanner) FindSubtitles(mediaPath string) ([]*SubtitleInfo, error) {
	dir := filepath.Dir(mediaPath)
	baseName := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))

	subtitles := make([]*SubtitleInfo, 0)
	subtitleExts := map[string]SubtitleFormat{
		".srt": SubtitleSRT,
		".ass": SubtitleASS,
		".ssa": SubtitleASS,
		".vtt": SubtitleVTT,
	}

	// 搜索同名字幕文件
	for ext, format := range subtitleExts {
		pattern := filepath.Join(dir, baseName+"*"+ext)
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			lang := extractLanguage(match, baseName)
			subtitles = append(subtitles, &SubtitleInfo{
				Language: lang,
				Format:   format,
				FilePath: match,
				Source:   "external",
			})
		}
	}

	return subtitles, nil
}

// DownloadPoster 下载海报.
func (s *Scanner) DownloadPoster(posterPath, destPath string) error {
	if posterPath == "" {
		return fmt.Errorf("海报路径为空")
	}

	// 模拟下载海报
	return nil
}

// DownloadBackdrop 下载背景图.
func (s *Scanner) DownloadBackdrop(backdropPath, destPath string) error {
	if backdropPath == "" {
		return fmt.Errorf("背景图路径为空")
	}

	// 模拟下载背景图
	return nil
}

// 辅助函数

func parseYear(s string) int {
	if len(s) != 4 {
		return 0
	}
	year := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		year = year*10 + int(c-'0')
	}
	return year
}

func extractLanguage(filePath, baseName string) string {
	// 从文件名提取语言信息
	// 例如: movie.zh.srt -> zh
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	// 移除基础文件名
	remaining := strings.TrimPrefix(name, baseName)
	remaining = strings.TrimPrefix(remaining, ".")

	if remaining == "" {
		return "und" // undefined
	}

	// 语言代码映射
	langMap := map[string]string{
		"zh":    "zh",
		"chs":   "zh",
		"cht":   "zh-TW",
		"en":    "en",
		"eng":   "en",
		"ja":    "ja",
		"jpn":   "ja",
		"ko":    "ko",
		"kor":   "ko",
	}

	if lang, ok := langMap[strings.ToLower(remaining)]; ok {
		return lang
	}

	return remaining
}
