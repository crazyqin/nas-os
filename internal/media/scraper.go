package media

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TMDBScraper TMDB 刮削器
type TMDBScraper struct {
	apiKey     string
	baseURL    string
	imageURL   string
	language   string
	httpClient *http.Client
	cache      MetadataCache
}

// TMDBConfig TMDB 配置
type TMDBConfig struct {
	APIKey   string
	Language string
	Timeout  time.Duration
	Cache    MetadataCache
}

// NewTMDBScraper 创建 TMDB 刮削器
func NewTMDBScraper(config *TMDBConfig) *TMDBScraper {
	if config.Language == "" {
		config.Language = "zh-CN"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &TMDBScraper{
		apiKey:   config.APIKey,
		baseURL:  "https://api.themoviedb.org/3",
		imageURL: "https://image.tmdb.org/t/p/original",
		language: config.Language,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache: config.Cache,
	}
}

// Name 返回提供商名称
func (s *TMDBScraper) Name() string {
	return "tmdb"
}

// SearchMovie 搜索电影
func (s *TMDBScraper) SearchMovie(query string) ([]*MovieInfo, error) {
	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("search_movie", query); ok {
			if results, ok := cached.([]*MovieInfo); ok {
				return results, nil
			}
		}
	}

	endpoint := fmt.Sprintf("%s/search/movie", s.baseURL)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("query", query)
	params.Set("language", s.language)
	params.Set("page", "1")

	resp, err := s.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("搜索电影失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID            int     `json:"id"`
			Title         string  `json:"title"`
			OriginalTitle string  `json:"original_title"`
			Overview      string  `json:"overview"`
			ReleaseDate   string  `json:"release_date"`
			Popularity    float64 `json:"popularity"`
			VoteAverage   float64 `json:"vote_average"`
			VoteCount     int     `json:"vote_count"`
			GenreIDs      []int   `json:"genre_ids"`
			PosterPath    string  `json:"poster_path"`
			BackdropPath  string  `json:"backdrop_path"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	movies := make([]*MovieInfo, 0, len(result.Results))
	for _, r := range result.Results {
		movie := &MovieInfo{
			ID:            fmt.Sprintf("tmdb_%d", r.ID),
			Title:         r.Title,
			OriginalTitle: r.OriginalTitle,
			Overview:      r.Overview,
			ReleaseDate:   r.ReleaseDate,
			Rating:        r.VoteAverage,
			VoteCount:     r.VoteCount,
			PosterPath:    s.getImageURL(r.PosterPath),
			BackdropPath:  s.getImageURL(r.BackdropPath),
			Source:        "tmdb",
			LastUpdated:   time.Now(),
		}
		movies = append(movies, movie)
	}

	// 缓存结果
	if s.cache != nil && len(movies) > 0 {
		s.cache.Set("search_movie", query, movies, 24*time.Hour)
	}

	return movies, nil
}

// GetMovie 获取电影详情
func (s *TMDBScraper) GetMovie(id string) (*MovieInfo, error) {
	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("movie", id); ok {
			if movie, ok := cached.(*MovieInfo); ok {
				return movie, nil
			}
		}
	}

	tmdbID := strings.TrimPrefix(id, "tmdb_")
	endpoint := fmt.Sprintf("%s/movie/%s", s.baseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", s.language)
	params.Set("append_to_response", "credits,videos,external_ids")

	resp, err := s.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("获取电影详情失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	movie, err := s.parseMovieDetail(body)
	if err != nil {
		return nil, err
	}

	// 缓存结果
	if s.cache != nil {
		s.cache.Set("movie", id, movie, 7*24*time.Hour)
	}

	return movie, nil
}

// GetMovieByIMDB 通过 IMDB ID 获取电影
func (s *TMDBScraper) GetMovieByIMDB(imdbID string) (*MovieInfo, error) {
	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("movie_imdb", imdbID); ok {
			if movie, ok := cached.(*MovieInfo); ok {
				return movie, nil
			}
		}
	}

	// 先通过 IMDB ID 查找 TMDB ID
	findEndpoint := fmt.Sprintf("%s/find/%s", s.baseURL, imdbID)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("external_source", "imdb_id")

	resp, err := s.httpClient.Get(findEndpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("通过 IMDB ID 查找电影失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var findResult struct {
		MovieResults []struct {
			ID int `json:"id"`
		} `json:"movie_results"`
	}

	if err := json.Unmarshal(body, &findResult); err != nil {
		return nil, err
	}

	if len(findResult.MovieResults) == 0 {
		return nil, fmt.Errorf("未找到 IMDB ID 对应的电影: %s", imdbID)
	}

	// 获取电影详情
	tmdbID := findResult.MovieResults[0].ID
	movie, err := s.GetMovie(fmt.Sprintf("tmdb_%d", tmdbID))
	if err != nil {
		return nil, err
	}

	movie.IMDBID = imdbID

	// 缓存结果
	if s.cache != nil {
		s.cache.Set("movie_imdb", imdbID, movie, 7*24*time.Hour)
	}

	return movie, nil
}

// parseMovieDetail 解析电影详情
func (s *TMDBScraper) parseMovieDetail(body []byte) (*MovieInfo, error) {
	var result struct {
		ID            int     `json:"id"`
		ImdbID        string  `json:"imdb_id"`
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		Overview      string  `json:"overview"`
		Runtime       int     `json:"runtime"`
		ReleaseDate   string  `json:"release_date"`
		VoteAverage   float64 `json:"vote_average"`
		VoteCount     int     `json:"vote_count"`
		Genres        []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
		PosterPath      string `json:"poster_path"`
		BackdropPath    string `json:"backdrop_path"`
		Tagline         string `json:"tagline"`
		SpokenLanguages []struct {
			Iso639_1 string `json:"iso_639_1"`
			Name     string `json:"name"`
		} `json:"spoken_languages"`
		ProductionCountries []struct {
			Iso3166_1 string `json:"iso_3166_1"`
			Name      string `json:"name"`
		} `json:"production_countries"`
		Credits struct {
			Cast []struct {
				Name string `json:"name"`
			} `json:"cast"`
			Crew []struct {
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
		ExternalIDs struct {
			ImdbID string `json:"imdb_id"`
		} `json:"external_ids"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	genres := make([]string, 0, len(result.Genres))
	for _, g := range result.Genres {
		genres = append(genres, g.Name)
	}

	directors := make([]string, 0)
	cast := make([]string, 0, min(10, len(result.Credits.Cast)))
	for _, c := range result.Credits.Cast {
		if len(cast) < 10 {
			cast = append(cast, c.Name)
		}
	}
	for _, c := range result.Credits.Crew {
		if c.Job == "Director" {
			directors = append(directors, c.Name)
		}
	}

	languages := make([]string, 0, len(result.SpokenLanguages))
	for _, l := range result.SpokenLanguages {
		languages = append(languages, l.Name)
	}

	countries := make([]string, 0, len(result.ProductionCountries))
	for _, c := range result.ProductionCountries {
		countries = append(countries, c.Name)
	}

	imdbID := result.ImdbID
	if result.ExternalIDs.ImdbID != "" {
		imdbID = result.ExternalIDs.ImdbID
	}

	movie := &MovieInfo{
		ID:            fmt.Sprintf("tmdb_%d", result.ID),
		IMDBID:        imdbID,
		Title:         result.Title,
		OriginalTitle: result.OriginalTitle,
		Overview:      result.Overview,
		Runtime:       result.Runtime,
		ReleaseDate:   result.ReleaseDate,
		Rating:        result.VoteAverage,
		VoteCount:     result.VoteCount,
		Genres:        genres,
		Directors:     directors,
		Cast:          cast,
		PosterPath:    s.getImageURL(result.PosterPath),
		BackdropPath:  s.getImageURL(result.BackdropPath),
		Tagline:       result.Tagline,
		Language:      strings.Join(languages, ", "),
		Country:       strings.Join(countries, ", "),
		Source:        "tmdb",
		LastUpdated:   time.Now(),
	}

	return movie, nil
}

// SearchTV 搜索电视剧
func (s *TMDBScraper) SearchTV(query string) ([]*TVShowInfo, error) {
	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("search_tv", query); ok {
			if results, ok := cached.([]*TVShowInfo); ok {
				return results, nil
			}
		}
	}

	endpoint := fmt.Sprintf("%s/search/tv", s.baseURL)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("query", query)
	params.Set("language", s.language)
	params.Set("page", "1")

	resp, err := s.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("搜索电视剧失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID           int     `json:"id"`
			Name         string  `json:"name"`
			OriginalName string  `json:"original_name"`
			Overview     string  `json:"overview"`
			FirstAirDate string  `json:"first_air_date"`
			VoteAverage  float64 `json:"vote_average"`
			VoteCount    int     `json:"vote_count"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	shows := make([]*TVShowInfo, 0, len(result.Results))
	for _, r := range result.Results {
		show := &TVShowInfo{
			ID:           fmt.Sprintf("tmdb_%d", r.ID),
			Name:         r.Name,
			OriginalName: r.OriginalName,
			Overview:     r.Overview,
			FirstAirDate: r.FirstAirDate,
			Rating:       r.VoteAverage,
			VoteCount:    r.VoteCount,
			PosterPath:   s.getImageURL(r.PosterPath),
			BackdropPath: s.getImageURL(r.BackdropPath),
			Source:       "tmdb",
			LastUpdated:  time.Now(),
		}
		shows = append(shows, show)
	}

	// 缓存结果
	if s.cache != nil && len(shows) > 0 {
		s.cache.Set("search_tv", query, shows, 24*time.Hour)
	}

	return shows, nil
}

// GetTV 获取电视剧详情
func (s *TMDBScraper) GetTV(id string) (*TVShowInfo, error) {
	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("tv", id); ok {
			if show, ok := cached.(*TVShowInfo); ok {
				return show, nil
			}
		}
	}

	tmdbID := strings.TrimPrefix(id, "tmdb_")
	endpoint := fmt.Sprintf("%s/tv/%s", s.baseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", s.language)
	params.Set("append_to_response", "credits,external_ids")

	resp, err := s.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("获取电视剧详情失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	show, err := s.parseTVDetail(body)
	if err != nil {
		return nil, err
	}

	// 缓存结果
	if s.cache != nil {
		s.cache.Set("tv", id, show, 7*24*time.Hour)
	}

	return show, nil
}

// GetTVSeason 获取电视剧季信息
func (s *TMDBScraper) GetTVSeason(tvID string, seasonNumber int) (*SeasonInfo, error) {
	tmdbID := strings.TrimPrefix(tvID, "tmdb_")
	cacheKey := fmt.Sprintf("%s_s%d", tmdbID, seasonNumber)

	// 检查缓存
	if s.cache != nil {
		if cached, ok := s.cache.Get("tv_season", cacheKey); ok {
			if season, ok := cached.(*SeasonInfo); ok {
				return season, nil
			}
		}
	}

	endpoint := fmt.Sprintf("%s/tv/%s/season/%d", s.baseURL, tmdbID, seasonNumber)
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", s.language)

	resp, err := s.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, s.wrapError("获取季信息失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		SeasonNumber int    `json:"season_number"`
		Name         string `json:"name"`
		Overview     string `json:"overview"`
		AirDate      string `json:"air_date"`
		PosterPath   string `json:"poster_path"`
		Episodes     []struct {
			EpisodeNumber int     `json:"episode_number"`
			Name          string  `json:"name"`
			Overview      string  `json:"overview"`
			AirDate       string  `json:"air_date"`
			StillPath     string  `json:"still_path"`
			Runtime       int     `json:"runtime"`
			VoteAverage   float64 `json:"vote_average"`
			VoteCount     int     `json:"vote_count"`
		} `json:"episodes"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	episodes := make([]EpisodeInfo, 0, len(result.Episodes))
	for _, e := range result.Episodes {
		episodes = append(episodes, EpisodeInfo{
			EpisodeNumber: e.EpisodeNumber,
			Name:          e.Name,
			Overview:      e.Overview,
			AirDate:       e.AirDate,
			StillPath:     s.getImageURL(e.StillPath),
			Runtime:       e.Runtime,
			Rating:        e.VoteAverage,
			VoteCount:     e.VoteCount,
		})
	}

	season := &SeasonInfo{
		SeasonNumber: result.SeasonNumber,
		Name:         result.Name,
		Overview:     result.Overview,
		AirDate:      result.AirDate,
		EpisodeCount: len(result.Episodes),
		PosterPath:   s.getImageURL(result.PosterPath),
		Episodes:     episodes,
	}

	// 缓存结果
	if s.cache != nil {
		s.cache.Set("tv_season", cacheKey, season, 7*24*time.Hour)
	}

	return season, nil
}

// parseTVDetail 解析电视剧详情
func (s *TMDBScraper) parseTVDetail(body []byte) (*TVShowInfo, error) {
	var result struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Overview     string `json:"overview"`
		FirstAirDate string `json:"first_air_date"`
		LastAirDate  string `json:"last_air_date"`
		Status       string `json:"status"`
		Seasons      []struct {
			SeasonNumber int `json:"season_number"`
			EpisodeCount int `json:"episode_count"`
		} `json:"seasons"`
		VoteAverage float64 `json:"vote_average"`
		VoteCount   int     `json:"vote_count"`
		Genres      []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
		PosterPath   string `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
		Networks     []struct {
			Name string `json:"name"`
		} `json:"networks"`
		CreatedBy []struct {
			Name string `json:"name"`
		} `json:"created_by"`
		Credits struct {
			Cast []struct {
				Name string `json:"name"`
			} `json:"cast"`
		} `json:"credits"`
		SpokenLanguages []struct {
			Name string `json:"name"`
		} `json:"spoken_languages"`
		OriginCountry []string `json:"origin_country"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	genres := make([]string, 0, len(result.Genres))
	for _, g := range result.Genres {
		genres = append(genres, g.Name)
	}

	creators := make([]string, 0, len(result.CreatedBy))
	for _, c := range result.CreatedBy {
		creators = append(creators, c.Name)
	}

	cast := make([]string, 0, min(10, len(result.Credits.Cast)))
	for _, c := range result.Credits.Cast {
		if len(cast) < 10 {
			cast = append(cast, c.Name)
		}
	}

	networks := make([]string, 0, len(result.Networks))
	for _, n := range result.Networks {
		networks = append(networks, n.Name)
	}

	totalSeasons := 0
	totalEpisodes := 0
	for _, season := range result.Seasons {
		totalSeasons++
		totalEpisodes += season.EpisodeCount
	}

	languages := make([]string, 0, len(result.SpokenLanguages))
	for _, l := range result.SpokenLanguages {
		languages = append(languages, l.Name)
	}

	show := &TVShowInfo{
		ID:           fmt.Sprintf("tmdb_%d", result.ID),
		Name:         result.Name,
		OriginalName: result.OriginalName,
		Overview:     result.Overview,
		FirstAirDate: result.FirstAirDate,
		LastAirDate:  result.LastAirDate,
		Status:       result.Status,
		Seasons:      totalSeasons,
		Episodes:     totalEpisodes,
		Rating:       result.VoteAverage,
		VoteCount:    result.VoteCount,
		Genres:       genres,
		Creators:     creators,
		Cast:         cast,
		PosterPath:   s.getImageURL(result.PosterPath),
		BackdropPath: s.getImageURL(result.BackdropPath),
		Networks:     networks,
		Language:     strings.Join(languages, ", "),
		Country:      strings.Join(result.OriginCountry, ", "),
		Source:       "tmdb",
		LastUpdated:  time.Now(),
	}

	return show, nil
}

// SearchMusic 搜索音乐（TMDB 不支持音乐）
func (s *TMDBScraper) SearchMusic(query string) ([]*MusicAlbumInfo, error) {
	return []*MusicAlbumInfo{}, nil
}

// GetMusic 获取音乐详情（TMDB 不支持音乐）
func (s *TMDBScraper) GetMusic(id string) (*MusicAlbumInfo, error) {
	return nil, fmt.Errorf("TMDB 不支持音乐元数据")
}

// getImageURL 获取完整图片 URL
func (s *TMDBScraper) getImageURL(path string) string {
	if path == "" {
		return ""
	}
	return s.imageURL + path
}

// wrapError 包装错误信息
func (s *TMDBScraper) wrapError(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}

// min 返回最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
