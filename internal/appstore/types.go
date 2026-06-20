// Package appstore 提供应用商店增强功能
// 对标飞牛fnOS的应用商店
// 支持应用分类、推荐、依赖管理、版本更新、评分评论
package appstore

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrAppNotFound 应用未找到
	ErrAppNotFound = errors.New("应用未找到")
	// ErrAlreadyInstalled 已安装
	ErrAlreadyInstalled = errors.New("应用已安装")
	// ErrDependencyMissing 缺少依赖
	ErrDependencyMissing = errors.New("缺少依赖")
	// ErrVersionConflict 版本冲突
	ErrVersionConflict = errors.New("版本冲突")
)

// ========== 应用分类 ==========

// AppCategory 应用分类
type AppCategory string

const (
	CategoryMedia      AppCategory = "media"       // 媒体
	CategoryDownload   AppCategory = "download"    // 下载
	CategoryNetwork    AppCategory = "network"     // 网络
	CategoryStorage    AppCategory = "storage"     // 存储
	CategorySecurity   AppCategory = "security"    // 安全
	CategoryDev        AppCategory = "development" // 开发
	CategoryUtility    AppCategory = "utility"     // 工具
	CategoryGame       AppCategory = "game"        // 游戏
	CategoryOther      AppCategory = "other"       // 其他
)

// ========== 应用信息 ==========

// App 应用信息
type App struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	DisplayName   string        `json:"display_name"`
	Description   string        `json:"description"`
	Version       string        `json:"version"`
	Author        string        `json:"author"`
	Website       string        `json:"website,omitempty"`
	Icon          string        `json:"icon,omitempty"`
	Category      AppCategory   `json:"category"`
	Tags          []string      `json:"tags,omitempty"`

	// 安装信息
	Installed     bool          `json:"installed"`
	InstalledVersion string     `json:"installed_version,omitempty"`
	InstalledAt   *time.Time    `json:"installed_at,omitempty"`

	// 依赖
	Dependencies  []Dependency  `json:"dependencies,omitempty"`

	// 评分
	Rating        float64       `json:"rating"`       // 0-5
	RatingCount   int           `json:"rating_count"`
	Downloads     int           `json:"downloads"`

	// 版本信息
	MinNASVersion string        `json:"min_nas_version"`
	MaxNASVersion string        `json:"max_nas_version,omitempty"`
	Size          int64         `json:"size"` // bytes

	// 更新
	Changelog     string        `json:"changelog,omitempty"`
	UpdatedAt     time.Time     `json:"updated_at"`
	CreatedAt     time.Time     `json:"created_at"`

	// 状态
	Status        AppStatus     `json:"status"`
	Featured      bool          `json:"featured"`
	Verified      bool          `json:"verified"`
}

// AppStatus 应用状态
type AppStatus string

const (
	StatusActive    AppStatus = "active"
	StatusBeta      AppStatus = "beta"
	StatusDeprecated AppStatus = "deprecated"
	StatusDisabled  AppStatus = "disabled"
)

// Dependency 依赖信息
type Dependency struct {
	AppID       string `json:"app_id"`
	AppName     string `json:"app_name"`
	MinVersion  string `json:"min_version"`
	Required    bool   `json:"required"`
}

// ========== 安装请求 ==========

// InstallRequest 安装请求
type InstallRequest struct {
	AppID       string `json:"app_id"`
	Version     string `json:"version,omitempty"`
	Force       bool   `json:"force"`
	SkipDeps    bool   `json:"skip_deps"`
}

// InstallResult 安装结果
type InstallResult struct {
	Success     bool     `json:"success"`
	AppID       string   `json:"app_id"`
	Version     string   `json:"version"`
	Message     string   `json:"message,omitempty"`
	InstalledDeps []string `json:"installed_deps,omitempty"`
}

// ========== 更新信息 ==========

// UpdateInfo 更新信息
type UpdateInfo struct {
	AppID         string    `json:"app_id"`
	CurrentVersion string   `json:"current_version"`
	LatestVersion string    `json:"latest_version"`
	Changelog     string    `json:"changelog"`
	Size          int64     `json:"size"`
	ReleasedAt    time.Time `json:"released_at"`
	Critical      bool      `json:"critical"`
}

// ========== 评分评论 ==========

// Review 评论
type Review struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Rating    int       `json:"rating"` // 1-5
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content,omitempty"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Helpful   int       `json:"helpful"` // 有帮助的数量
}

// ReviewRequest 评论请求
type ReviewRequest struct {
	AppID   string `json:"app_id"`
	Rating  int    `json:"rating"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

// ========== 搜索过滤 ==========

// AppSearchRequest 搜索请求
type AppSearchRequest struct {
	Query     string      `json:"query,omitempty"`
	Category  AppCategory `json:"category,omitempty"`
	Tags      []string    `json:"tags,omitempty"`
	Installed *bool       `json:"installed,omitempty"`
	Featured  *bool       `json:"featured,omitempty"`
	SortBy    string      `json:"sort_by,omitempty"` // rating, downloads, updated
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}

// AppSearchResult 搜索结果
type AppSearchResult struct {
	Apps       []*App `json:"apps"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

// ========== 统计 ==========

// AppStoreStats 商店统计
type AppStoreStats struct {
	TotalApps     int            `json:"total_apps"`
	InstalledApps int            `json:"installed_apps"`
	UpdatesAvail  int            `json:"updates_available"`
	CategoryCounts map[AppCategory]int `json:"category_counts"`
	TotalDownloads int           `json:"total_downloads"`
}
