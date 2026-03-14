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

// MovieInfo 电影信息
type MovieInfo struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"originalTitle"`
	Overview      string    `json:"overview"`
	ReleaseDate   string    `json:"releaseDate"`
	Runtime       int       `json:"runtime"`
	Rating        float64   `json:"rating"`
	VoteCount     int       `json:"voteCount"`
	Genres        []string  `json:"genres"`
	Directors     []string  `json:"directors"`
	Cast          []string  `json:"cast"`
	PosterPath    string    `json:"posterPath"`
	BackdropPath  string    `json:"backdropPath"`
	Tagline       string    `json:"tagline"`
	Language      string    `json:"language"`
	Country       string    `json:"country"`
	Source        string    `json:"source"` // tmdb/douban
	LastUpdated   time.Time `json:"lastUpdated"`
}

// TVShowInfo 电视剧信息
type TVShowInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OriginalName string    `json:"originalName"`
	Overview     string    `json:"overview"`
	FirstAirDate string    `json:"firstAirDate"`
	LastAirDate  string    `json:"lastAirDate"`
	Status       string    `json:"status"`
	Seasons      int       `json:"seasons"`
	Episodes     int       `json:"episodes"`
	Rating       float64   `json:"rating"`
	VoteCount    int       `json:"voteCount"`
	Genres       []string  `json:"genres"`
	Creators     []string  `json:"creators"`
	Cast         []string  `json:"cast"`
	PosterPath   string    `json:"posterPath"`
	BackdropPath string    `json:"backdropPath"`
	Networks     []string  `json:"networks"`
	Language     string    `json:"language"`
	Country      string    `json:"country"`
	Source       string    `json:"source"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

// MusicAlbumInfo 音乐专辑信息
type MusicAlbumInfo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	ReleaseDate string    `json:"releaseDate"`
	Genres      []string  `json:"genres"`
	Label       string    `json:"label"`
	TotalTracks int       `json:"totalTracks"`
	CoverArt    string    `json:"coverArt"`
	Source      string    `json:"source"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// MetadataProvider 元数据提供商接口
type MetadataProvider interface {
	SearchMovie(query string) ([]*MovieInfo, error)
	GetMovie(id string) (*MovieInfo, error)
	SearchTV(query string) ([]*TVShowInfo, error)
	GetTV(id string) (*TVShowInfo, error)
	SearchMusic(query string) ([]*MusicAlbumInfo, error)
	GetMusic(id string) (*MusicAlbumInfo, error)
}

// TMDBProvider TMDB 元数据提供商
type TMDBProvider struct {
	apiKey     string
	baseURL    string
	imageURL   string
	language   string
	httpClient *http.Client
}

// NewTMDBProvider 创建 TMDB 提供商
func NewTMDBProvider(apiKey string, language string) *TMDBProvider {
	if language == "" {
		language = "zh-CN"
	}
	return &TMDBProvider{
		apiKey:   apiKey,
		baseURL:  "https://api.themoviedb.org/3",
		imageURL: "https://image.tmdb.org/t/p/original",
		language: language,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchMovie 搜索电影
func (p *TMDBProvider) SearchMovie(query string) ([]*MovieInfo, error) {
	endpoint := fmt.Sprintf("%s/search/movie", p.baseURL)
	params := url.Values{}
	params.Set("api_key", p.apiKey)
	params.Set("query", query)
	params.Set("language", p.language)
	params.Set("page", "1")

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
			PosterPath:    p.getImageURL(r.PosterPath),
			BackdropPath:  p.getImageURL(r.BackdropPath),
			Source:        "tmdb",
			LastUpdated:   time.Now(),
		}
		movies = append(movies, movie)
	}

	return movies, nil
}

// GetMovie 获取电影详情
func (p *TMDBProvider) GetMovie(id string) (*MovieInfo, error) {
	// 提取 TMDB ID
	tmdbID := strings.TrimPrefix(id, "tmdb_")
	endpoint := fmt.Sprintf("%s/movie/%s", p.baseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", p.apiKey)
	params.Set("language", p.language)
	params.Set("append_to_response", "credits,videos")

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		ID            int     `json:"id"`
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
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	genres := make([]string, 0, len(result.Genres))
	for _, g := range result.Genres {
		genres = append(genres, g.Name)
	}

	directors := make([]string, 0)
	cast := make([]string, 0, len(result.Credits.Cast))
	for _, c := range result.Credits.Cast {
		cast = append(cast, c.Name)
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

	movie := &MovieInfo{
		ID:            fmt.Sprintf("tmdb_%d", result.ID),
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
		PosterPath:    p.getImageURL(result.PosterPath),
		BackdropPath:  p.getImageURL(result.BackdropPath),
		Tagline:       result.Tagline,
		Language:      strings.Join(languages, ", "),
		Country:       strings.Join(countries, ", "),
		Source:        "tmdb",
		LastUpdated:   time.Now(),
	}

	return movie, nil
}

// SearchTV 搜索电视剧
func (p *TMDBProvider) SearchTV(query string) ([]*TVShowInfo, error) {
	endpoint := fmt.Sprintf("%s/search/tv", p.baseURL)
	params := url.Values{}
	params.Set("api_key", p.apiKey)
	params.Set("query", query)
	params.Set("language", p.language)
	params.Set("page", "1")

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
			Popularity   float64 `json:"popularity"`
			VoteAverage  float64 `json:"vote_average"`
			VoteCount    int     `json:"vote_count"`
			GenreIDs     []int   `json:"genre_ids"`
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
			PosterPath:   p.getImageURL(r.PosterPath),
			BackdropPath: p.getImageURL(r.BackdropPath),
			Source:       "tmdb",
			LastUpdated:  time.Now(),
		}
		shows = append(shows, show)
	}

	return shows, nil
}

// GetTV 获取电视剧详情
func (p *TMDBProvider) GetTV(id string) (*TVShowInfo, error) {
	tmdbID := strings.TrimPrefix(id, "tmdb_")
	endpoint := fmt.Sprintf("%s/tv/%s", p.baseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", p.apiKey)
	params.Set("language", p.language)
	params.Set("append_to_response", "credits,videos")

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

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

	cast := make([]string, 0, len(result.Credits.Cast))
	for _, c := range result.Credits.Cast {
		cast = append(cast, c.Name)
	}

	networks := make([]string, 0, len(result.Networks))
	for _, n := range result.Networks {
		networks = append(networks, n.Name)
	}

	totalSeasons := 0
	totalEpisodes := 0
	for _, s := range result.Seasons {
		totalSeasons++
		totalEpisodes += s.EpisodeCount
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
		PosterPath:   p.getImageURL(result.PosterPath),
		BackdropPath: p.getImageURL(result.BackdropPath),
		Networks:     networks,
		Language:     strings.Join(languages, ", "),
		Country:      strings.Join(result.OriginCountry, ", "),
		Source:       "tmdb",
		LastUpdated:  time.Now(),
	}

	return show, nil
}

// SearchMusic 搜索音乐（TMDB 不支持音乐，返回空）
func (p *TMDBProvider) SearchMusic(query string) ([]*MusicAlbumInfo, error) {
	return []*MusicAlbumInfo{}, nil
}

// GetMusic 获取音乐详情（TMDB 不支持音乐，返回空）
func (p *TMDBProvider) GetMusic(id string) (*MusicAlbumInfo, error) {
	return nil, fmt.Errorf("TMDB 不支持音乐元数据")
}

// getImageURL 获取完整图片 URL
func (p *TMDBProvider) getImageURL(path string) string {
	if path == "" {
		return ""
	}
	return p.imageURL + path
}
