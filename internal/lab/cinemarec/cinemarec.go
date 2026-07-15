// Package cinemarec 提供家庭影院推荐引擎
// 对标飞牛 fnOS 媒体中心和 Synology Video Station
package cinemarec

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaMovie MediaType = "movie"
	MediaTV    MediaType = "tv"
	MediaMusic MediaType = "music"
	MediaVideo MediaType = "video"
	MediaPhoto MediaType = "photo"
)

// WatchHistory 观看历史
type WatchHistory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	MediaID   string    `json:"media_id"`
	MediaType MediaType `json:"media_type"`
	Title     string    `json:"title"`
	Genres    []string  `json:"genres"`
	WatchedAt time.Time `json:"watched_at"`
	Duration  int       `json:"duration_seconds"`
	Position  int       `json:"position_seconds"`
	Completed bool      `json:"completed"`
	Rating    int       `json:"rating,omitempty"`
}

// MediaItem 媒体条目
type MediaItem struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Type         MediaType `json:"type"`
	Genres       []string  `json:"genres"`
	Year         int       `json:"year"`
	Duration     int       `json:"duration_minutes"`
	Rating       float64   `json:"rating"`
	PosterPath   string    `json:"poster_path,omitempty"`
	BackdropPath string    `json:"backdrop_path,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
}

// Recommendation 推荐
type Recommendation struct {
	Media    *MediaItem  `json:"media"`
	Score    float64     `json:"score"`
	Reason   string      `json:"reason"`
	Category RecCategory `json:"category"`
}

// RecCategory 推荐类别
type RecCategory string

const (
	RecBecauseYouWatched RecCategory = "because_you_watched"
	RecTrending          RecCategory = "trending"
	RecNewReleases       RecCategory = "new_releases"
	RecByGenre           RecCategory = "by_genre"
	RecContinueWatching  RecCategory = "continue_watching"
	RecFamilyPick        RecCategory = "family_pick"
	RecTopRated          RecCategory = "top_rated"
	RecSimilar           RecCategory = "similar"
)

// Engine 推荐引擎
type Engine struct {
	mu          sync.RWMutex
	history     []*WatchHistory
	library     []*MediaItem
	userProfile *UserProfile
}

// UserProfile 用户画像
type UserProfile struct {
	UserID           string            `json:"user_id"`
	PreferredGenres  map[string]int    `json:"preferred_genres"`
	PreferredTypes   map[MediaType]int `json:"preferred_types"`
	AvgRating        float64           `json:"avg_rating"`
	TotalWatched     int               `json:"total_watched"`
	TotalCompleted   int               `json:"total_completed"`
	WatchTimeMinutes int               `json:"watch_time_minutes"`
	LastActive       time.Time         `json:"last_active"`
}

// NewEngine 创建推荐引擎
func NewEngine() *Engine {
	return &Engine{
		history: make([]*WatchHistory, 0),
		library: make([]*MediaItem, 0),
		userProfile: &UserProfile{
			PreferredGenres: make(map[string]int),
			PreferredTypes:  make(map[MediaType]int),
		},
	}
}

// AddToLibrary 添加到媒体库
func (e *Engine) AddToLibrary(item *MediaItem) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.library = append(e.library, item)
}

// RecordWatch 记录观看
func (e *Engine) RecordWatch(entry *WatchHistory) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.history = append(e.history, entry)
	e.userProfile.TotalWatched++
	if entry.Completed {
		e.userProfile.TotalCompleted++
	}
	e.userProfile.WatchTimeMinutes += entry.Duration / 60
	e.userProfile.LastActive = entry.WatchedAt

	// 更新偏好
	for _, genre := range entry.Genres {
		e.userProfile.PreferredGenres[genre]++
	}
	e.userProfile.PreferredTypes[entry.MediaType]++

	// 更新平均评分
	if entry.Rating > 0 {
		total := e.userProfile.AvgRating * float64(e.userProfile.TotalWatched-1)
		e.userProfile.AvgRating = (total + float64(entry.Rating)) / float64(e.userProfile.TotalWatched)
	}
}

// GetRecommendations 获取推荐
func (e *Engine) GetRecommendations(limit int) []*Recommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var recs []*Recommendation

	// 继续观看
	recs = append(recs, e.continueWatching(limit/3)...)

	// 基于偏好类型推荐
	recs = append(recs, e.byGenre(limit/3)...)

	// 热门推荐
	recs = append(recs, e.trending(limit/3)...)

	// 去重并截断
	recs = e.dedup(recs)
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}

	return recs
}

// continueWatching 继续观看
func (e *Engine) continueWatching(limit int) []*Recommendation {
	var recs []*Recommendation
	for _, entry := range e.history {
		if !entry.Completed && entry.Position > 0 {
			item := e.findInLibrary(entry.MediaID)
			if item != nil {
				recs = append(recs, &Recommendation{
					Media:    item,
					Score:    0.9,
					Reason:   fmt.Sprintf("继续观看: %s (已观看 %d%%)", item.Title, entry.Position*100/entry.Duration),
					Category: RecContinueWatching,
				})
			}
		}
		if limit > 0 && len(recs) >= limit {
			break
		}
	}
	return recs
}

// byGenre 按类型推荐
func (e *Engine) byGenre(limit int) []*Recommendation {
	// 获取最喜欢的类型
	var topGenres []string
	for genre, count := range e.userProfile.PreferredGenres {
		if count > 0 {
			topGenres = append(topGenres, genre)
		}
	}
	sort.Slice(topGenres, func(i, j int) bool {
		return e.userProfile.PreferredGenres[topGenres[i]] > e.userProfile.PreferredGenres[topGenres[j]]
	})

	watched := make(map[string]bool)
	for _, h := range e.history {
		watched[h.MediaID] = true
	}

	var recs []*Recommendation
	for _, item := range e.library {
		if watched[item.ID] {
			continue
		}
		for _, genre := range item.Genres {
			if contains(topGenres, genre) {
				score := 0.5 + float64(e.userProfile.PreferredGenres[genre])*0.1
				if score > 1.0 {
					score = 1.0
				}
				recs = append(recs, &Recommendation{
					Media:    item,
					Score:    score,
					Reason:   fmt.Sprintf("因为你喜欢 %s 类型", genre),
					Category: RecByGenre,
				})
				break
			}
		}
		if limit > 0 && len(recs) >= limit {
			break
		}
	}
	return recs
}

// trending 热门推荐
func (e *Engine) trending(limit int) []*Recommendation {
	var recs []*Recommendation
	items := make([]*MediaItem, len(e.library))
	copy(items, e.library)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Rating > items[j].Rating
	})
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		recs = append(recs, &Recommendation{
			Media:    item,
			Score:    item.Rating / 10 * 0.7,
			Reason:   "高分推荐",
			Category: RecTopRated,
		})
	}
	return recs
}

// findInLibrary 在库中查找媒体
func (e *Engine) findInLibrary(mediaID string) *MediaItem {
	for _, item := range e.library {
		if item.ID == mediaID {
			return item
		}
	}
	return nil
}

// dedup 去重
func (e *Engine) dedup(recs []*Recommendation) []*Recommendation {
	seen := make(map[string]bool)
	var result []*Recommendation
	for _, rec := range recs {
		if rec.Media != nil && !seen[rec.Media.ID] {
			seen[rec.Media.ID] = true
			result = append(result, rec)
		}
	}
	// 按分数排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result
}

// GetUserProfile 获取用户画像
func (e *Engine) GetUserProfile() *UserProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	profile := &UserProfile{
		UserID:           e.userProfile.UserID,
		PreferredGenres:  make(map[string]int),
		PreferredTypes:   make(map[MediaType]int),
		AvgRating:        e.userProfile.AvgRating,
		TotalWatched:     e.userProfile.TotalWatched,
		TotalCompleted:   e.userProfile.TotalCompleted,
		WatchTimeMinutes: e.userProfile.WatchTimeMinutes,
		LastActive:       e.userProfile.LastActive,
	}
	for k, v := range e.userProfile.PreferredGenres {
		profile.PreferredGenres[k] = v
	}
	for k, v := range e.userProfile.PreferredTypes {
		profile.PreferredTypes[k] = v
	}
	return profile
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// FormatRecommendations 格式化推荐列表
func (e *Engine) FormatRecommendations(recs []*Recommendation) string {
	if len(recs) == 0 {
		return "暂无推荐，请先添加媒体和观看历史"
	}
	var sb strings.Builder
	sb.WriteString("🏠 影院推荐:\n")
	sb.WriteString(strings.Repeat("═", 50) + "\n")
	for i, rec := range recs {
		if rec.Media == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("%d. %s (%d) ⭐%.1f\n", i+1, rec.Media.Title, rec.Media.Year, rec.Media.Rating))
		sb.WriteString(fmt.Sprintf("   理由: %s\n", rec.Reason))
	}
	return sb.String()
}
