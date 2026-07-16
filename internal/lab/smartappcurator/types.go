// Package smartappcurator 提供 AI 驱动的应用推荐引擎功能。
package smartappcurator

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoProfile 表示用户画像不存在。
	ErrNoProfile = errors.New("用户画像不存在")
	// ErrInsufficientData 表示数据不足以生成推荐。
	ErrInsufficientData = errors.New("使用数据不足，无法生成推荐")
)

// ========== 应用信息 ==========

// AppCategory 应用类别。
type AppCategory string

const (
	CategoryMedia        AppCategory = "media"
	CategoryProductivity AppCategory = "productivity"
	CategoryBackup       AppCategory = "backup"
	CategorySecurity     AppCategory = "security"
	CategoryNetwork      AppCategory = "network"
	CategoryDev          AppCategory = "development"
	CategoryHome         AppCategory = "home"
	CategoryOffice       AppCategory = "office"
	CategoryAI           AppCategory = "ai"
	CategoryStorage      AppCategory = "storage"
)

// AppInfo 应用信息。
type AppInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Category    AppCategory `json:"category"`
	Description string      `json:"description"`
	Icon        string      `json:"icon,omitempty"`
	Version     string      `json:"version"`
	Downloads   int64       `json:"downloads"`
	Rating      float64     `json:"rating"`
	Tags        []string    `json:"tags,omitempty"`
	Size        int64       `json:"size_bytes"`
}

// ========== 用户画像 ==========

// UserProfile 用户使用画像。
type UserProfile struct {
	UserID        string              `json:"user_id"`
	InstalledApps []string            `json:"installed_apps"`
	UsageStats    map[string]AppUsage `json:"usage_stats"`
	Preferences   UserPreferences     `json:"preferences"`
	LastUpdated   time.Time           `json:"last_updated"`
}

// AppUsage 应用使用统计。
type AppUsage struct {
	AppID         string    `json:"app_id"`
	LaunchCount   int       `json:"launch_count"`
	TotalDuration int64     `json:"total_duration_seconds"`
	LastUsed      time.Time `json:"last_used"`
	AvgSession    int64     `json:"avg_session_seconds"`
}

// UserPreferences 用户偏好。
type UserPreferences struct {
	Categories []AppCategory `json:"preferred_categories,omitempty"`
	Tags       []string      `json:"preferred_tags,omitempty"`
	MaxAppSize int64         `json:"max_app_size,omitempty"`
	FreeOnly   bool          `json:"free_only,omitempty"`
	Language   string        `json:"language,omitempty"`
}

// ========== 推荐结果 ==========

// Recommendation 推荐结果。
type Recommendation struct {
	App       AppInfo  `json:"app"`
	Score     float64  `json:"score"`
	Reason    string   `json:"reason"`
	MatchTags []string `json:"match_tags,omitempty"`
	SimilarTo string   `json:"similar_to,omitempty"`
}

// RecommendationSet 推荐集合。
type RecommendationSet struct {
	UserID          string           `json:"user_id"`
	GeneratedAt     time.Time        `json:"generated_at"`
	Recommendations []Recommendation `json:"recommendations"`
	TrendingApps    []AppInfo        `json:"trending_apps"`
	NewApps         []AppInfo        `json:"new_apps"`
}

// ========== 推荐请求 ==========

// RecommendRequest 推荐请求。
type RecommendRequest struct {
	UserID  string   `json:"user_id"`
	Limit   int      `json:"limit,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}
