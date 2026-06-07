// Package appmarket 应用市场模块
package appmarket

import (
	"time"
)

// AppStatus 应用状态
type AppStatus string

const (
	// StatusDraft 草稿
	StatusDraft AppStatus = "draft"
	// StatusPendingReview 待审核
	StatusPendingReview AppStatus = "pending_review"
	// StatusApproved 已通过
	StatusApproved AppStatus = "approved"
	// StatusRejected 已拒绝
	StatusRejected AppStatus = "rejected"
	// StatusRevision 需修改
	StatusRevision AppStatus = "revision"
	// StatusPublished 已发布
	StatusPublished AppStatus = "published"
	// StatusSuspended 已下架
	StatusSuspended AppStatus = "suspended"
)

// AppCategory 应用分类
type AppCategory string

const (
	CategoryProductivity AppCategory = "productivity"
	CategoryMedia        AppCategory = "media"
	CategoryNetwork      AppCategory = "network"
	CategoryStorage      AppCategory = "storage"
	CategorySecurity     AppCategory = "security"
	CategoryDevOps       AppCategory = "devops"
	CategoryDatabase     AppCategory = "database"
	CategoryAI           AppCategory = "ai"
	CategoryGaming       AppCategory = "gaming"
	CategoryUtility      AppCategory = "utility"
	CategoryOther        AppCategory = "other"
)

// ReviewAction 审核动作
type ReviewAction string

const (
	ReviewApprove  ReviewAction = "approve"
	ReviewReject   ReviewAction = "reject"
	ReviewRevision ReviewAction = "revision"
)

// SortOption 排序选项
type SortOption string

const (
	SortLatest    SortOption = "latest"
	SortRating    SortOption = "rating"
	SortDownloads SortOption = "downloads"
	SortName      SortOption = "name"
)

// AppInfo 应用信息
type AppInfo struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Icon         string      `json:"icon"`
	Version      string      `json:"version"`
	Author       string      `json:"author"`
	Category     AppCategory `json:"category"`
	Tags         []string    `json:"tags,omitempty"`
	License      string      `json:"license,omitempty"`
	Homepage     string      `json:"homepage,omitempty"`
	Repository   string      `json:"repository,omitempty"`
	Size         int64       `json:"size"`
	MinCPU       int         `json:"min_cpu,omitempty"`
	MinMemory    int64       `json:"min_memory,omitempty"`
	MinDisk      int64       `json:"min_disk,omitempty"`
	Dependencies []string    `json:"dependencies,omitempty"`
	Status       AppStatus   `json:"status"`
	Downloads    int64       `json:"downloads"`
	Rating       float64     `json:"rating"`
	RatingCount  int         `json:"rating_count"`
	DeveloperID  string      `json:"developer_id"`
	ReviewNote   string      `json:"review_note,omitempty"`
	ReviewedBy   string      `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time  `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// InstalledApp 已安装应用
type InstalledApp struct {
	AppID       string    `json:"app_id"`
	Version     string    `json:"version"`
	Status      string    `json:"status"` // running/stopped/error
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ConfigPath  string    `json:"config_path,omitempty"`
}

// ReviewRecord 审核记录
type ReviewRecord struct {
	ID        string       `json:"id"`
	AppID     string       `json:"app_id"`
	Reviewer  string       `json:"reviewer"`
	Action    ReviewAction `json:"action"`
	Note      string       `json:"note"`
	CreatedAt time.Time    `json:"created_at"`
}

// Rating 评分记录
type Rating struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Score     int       `json:"score"` // 1-5
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query    string      `json:"query,omitempty"`
	Category AppCategory `json:"category,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Sort     SortOption  `json:"sort,omitempty"`
	Page     int         `json:"page,omitempty"`
	PageSize int         `json:"page_size,omitempty"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Apps       []*AppInfo `json:"apps"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// PublishRequest 发布请求
type PublishRequest struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Icon         string      `json:"icon"`
	Version      string      `json:"version"`
	Category     AppCategory `json:"category"`
	Tags         []string    `json:"tags,omitempty"`
	License      string      `json:"license,omitempty"`
	Homepage     string      `json:"homepage,omitempty"`
	Repository   string      `json:"repository,omitempty"`
	Size         int64       `json:"size"`
	MinCPU       int         `json:"min_cpu,omitempty"`
	MinMemory    int64       `json:"min_memory,omitempty"`
	MinDisk      int64       `json:"min_disk,omitempty"`
	Dependencies []string    `json:"dependencies,omitempty"`
}

// ReviewRequest 审核请求
type ReviewRequest struct {
	Action ReviewAction `json:"action"`
	Note   string       `json:"note,omitempty"`
}

// InstallRequest 安装请求
type InstallRequest struct {
	AppID   string `json:"app_id"`
	Version string `json:"version,omitempty"`
}

// UpdateRequest 更新请求
type UpdateRequest struct {
	AppID         string `json:"app_id"`
	TargetVersion string `json:"target_version,omitempty"`
}

// RatingRequest 评分请求
type RatingRequest struct {
	Score   int    `json:"score"`
	Comment string `json:"comment,omitempty"`
}
