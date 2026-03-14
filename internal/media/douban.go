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

// DoubanProvider 豆瓣元数据提供商
type DoubanProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// DoubanMovie 豆瓣电影响应
type DoubanMovie struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	OrigTitle string `json:"orig_title"`
	Summary   string `json:"summary"`
	Year      string `json:"year"`
	Rating    struct {
		Average   float64 `json:"average"`
		NumRaters int     `json:"numRaters"`
	} `json:"rating"`
	Genres    []string `json:"genres"`
	Directors []struct {
		Name string `json:"name"`
	} `json:"directors"`
	Cast []struct {
		Name string `json:"name"`
	} `json:"casts"`
	Images struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
	} `json:"images"`
	Alt       string   `json:"alt"`
	MobileURL string   `json:"mobile_url"`
	Doi       []string `json:"doi"`
	Seasons   int      `json:"seasons_count"`
	Episodes  int      `json:"episodes_count"`
}

// NewDoubanProvider 创建豆瓣提供商
func NewDoubanProvider(apiKey string) *DoubanProvider {
	return &DoubanProvider{
		apiKey:  apiKey,
		baseURL: "https://api.douban.com/v2",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchMovie 搜索电影（豆瓣 API）
func (p *DoubanProvider) SearchMovie(query string) ([]*MovieInfo, error) {
	endpoint := fmt.Sprintf("%s/movie/search", p.baseURL)
	params := url.Values{}
	params.Set("q", query)
	params.Set("start", "0")
	params.Set("count", "20")
	if p.apiKey != "" {
		params.Set("apikey", p.apiKey)
	}

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
		Count    int           `json:"count"`
		Start    int           `json:"start"`
		Total    int           `json:"total"`
		Subjects []DoubanMovie `json:"subjects"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	movies := make([]*MovieInfo, 0, len(result.Subjects))
	for _, subject := range result.Subjects {
		directors := make([]string, 0, len(subject.Directors))
		for _, d := range subject.Directors {
			directors = append(directors, d.Name)
		}

		cast := make([]string, 0, len(subject.Cast))
		for _, c := range subject.Cast {
			cast = append(cast, c.Name)
		}

		movie := &MovieInfo{
			ID:            fmt.Sprintf("douban_%s", subject.ID),
			Title:         subject.Title,
			OriginalTitle: subject.OrigTitle,
			Overview:      subject.Summary,
			ReleaseDate:   subject.Year,
			Rating:        subject.Rating.Average,
			VoteCount:     subject.Rating.NumRaters,
			Genres:        subject.Genres,
			Directors:     directors,
			Cast:          cast,
			PosterPath:    subject.Images.Large,
			Source:        "douban",
			LastUpdated:   time.Now(),
		}
		movies = append(movies, movie)
	}

	return movies, nil
}

// GetMovie 获取电影详情（豆瓣 API）
func (p *DoubanProvider) GetMovie(id string) (*MovieInfo, error) {
	doubanID := strings.TrimPrefix(id, "douban_")
	endpoint := fmt.Sprintf("%s/movie/%s", p.baseURL, doubanID)
	params := url.Values{}
	if p.apiKey != "" {
		params.Set("apikey", p.apiKey)
	}

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var subject DoubanMovie
	if err := json.Unmarshal(body, &subject); err != nil {
		return nil, err
	}

	directors := make([]string, 0, len(subject.Directors))
	for _, d := range subject.Directors {
		directors = append(directors, d.Name)
	}

	cast := make([]string, 0, len(subject.Cast))
	for _, c := range subject.Cast {
		cast = append(cast, c.Name)
	}

	movie := &MovieInfo{
		ID:            fmt.Sprintf("douban_%s", subject.ID),
		Title:         subject.Title,
		OriginalTitle: subject.OrigTitle,
		Overview:      subject.Summary,
		ReleaseDate:   subject.Year,
		Rating:        subject.Rating.Average,
		VoteCount:     subject.Rating.NumRaters,
		Genres:        subject.Genres,
		Directors:     directors,
		Cast:          cast,
		PosterPath:    subject.Images.Large,
		Source:        "douban",
		LastUpdated:   time.Now(),
	}

	return movie, nil
}

// SearchTV 搜索电视剧（豆瓣 API）
func (p *DoubanProvider) SearchTV(query string) ([]*TVShowInfo, error) {
	endpoint := fmt.Sprintf("%s/tv/search", p.baseURL)
	params := url.Values{}
	params.Set("q", query)
	params.Set("start", "0")
	params.Set("count", "20")
	if p.apiKey != "" {
		params.Set("apikey", p.apiKey)
	}

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
		Count    int           `json:"count"`
		Start    int           `json:"start"`
		Total    int           `json:"total"`
		Subjects []DoubanMovie `json:"subjects"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	shows := make([]*TVShowInfo, 0, len(result.Subjects))
	for _, subject := range result.Subjects {
		cast := make([]string, 0, len(subject.Cast))
		for _, c := range subject.Cast {
			cast = append(cast, c.Name)
		}

		show := &TVShowInfo{
			ID:           fmt.Sprintf("douban_%s", subject.ID),
			Name:         subject.Title,
			OriginalName: subject.OrigTitle,
			Overview:     subject.Summary,
			FirstAirDate: subject.Year,
			Seasons:      subject.Seasons,
			Episodes:     subject.Episodes,
			Rating:       subject.Rating.Average,
			VoteCount:    subject.Rating.NumRaters,
			Genres:       subject.Genres,
			Cast:         cast,
			PosterPath:   subject.Images.Large,
			Source:       "douban",
			LastUpdated:  time.Now(),
		}
		shows = append(shows, show)
	}

	return shows, nil
}

// GetTV 获取电视剧详情（豆瓣 API）
func (p *DoubanProvider) GetTV(id string) (*TVShowInfo, error) {
	doubanID := strings.TrimPrefix(id, "douban_")
	endpoint := fmt.Sprintf("%s/tv/%s", p.baseURL, doubanID)
	params := url.Values{}
	if p.apiKey != "" {
		params.Set("apikey", p.apiKey)
	}

	resp, err := p.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var subject DoubanMovie
	if err := json.Unmarshal(body, &subject); err != nil {
		return nil, err
	}

	cast := make([]string, 0, len(subject.Cast))
	for _, c := range subject.Cast {
		cast = append(cast, c.Name)
	}

	show := &TVShowInfo{
		ID:           fmt.Sprintf("douban_%s", subject.ID),
		Name:         subject.Title,
		OriginalName: subject.OrigTitle,
		Overview:     subject.Summary,
		FirstAirDate: subject.Year,
		Seasons:      subject.Seasons,
		Episodes:     subject.Episodes,
		Rating:       subject.Rating.Average,
		VoteCount:    subject.Rating.NumRaters,
		Genres:       subject.Genres,
		Cast:         cast,
		PosterPath:   subject.Images.Large,
		Source:       "douban",
		LastUpdated:  time.Now(),
	}

	return show, nil
}

// SearchMusic 搜索音乐（豆瓣 API 暂不支持）
func (p *DoubanProvider) SearchMusic(query string) ([]*MusicAlbumInfo, error) {
	return []*MusicAlbumInfo{}, nil
}

// GetMusic 获取音乐详情（豆瓣 API 暂不支持）
func (p *DoubanProvider) GetMusic(id string) (*MusicAlbumInfo, error) {
	return nil, fmt.Errorf("豆瓣 API 暂不支持音乐元数据")
}
