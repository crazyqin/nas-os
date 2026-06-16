// Package appmarket 应用市场模块 - 完整的应用生命周期管理
package appmarket

import (
	"time"
)

// AppStatus 应用状态
type AppStatus string

const (
	StatusDraft        AppStatus = "draft"
	StatusPendingReview AppStatus = "pending_review"
	StatusApproved     AppStatus = "approved"
	StatusRejected     AppStatus = "rejected"
	StatusRevision     AppStatus = "revision"
	StatusPublished    AppStatus = "published"
	StatusSuspended    AppStatus = "suspended"
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
	CategorySmartHome    AppCategory = "smart_home"
	CategoryDownload     AppCategory = "download"
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
	SortSize      SortOption = "size"
)

// InstallStatus 安装状态
type InstallStatus string

const (
	InstallPending     InstallStatus = "pending"
	InstallPulling     InstallStatus = "pulling"
	InstallConfiguring InstallStatus = "configuring"
	InstallStarting    InstallStatus = "starting"
	InstallComplete    InstallStatus = "complete"
	InstallFailed      InstallStatus = "failed"
)

// SecurityScanStatus 安全扫描状态
type SecurityScanStatus string

const (
	SecurityScanPending  SecurityScanStatus = "pending"
	SecurityScanRunning  SecurityScanStatus = "running"
	SecurityScanPassed   SecurityScanStatus = "passed"
	SecurityScanFailed   SecurityScanStatus = "failed"
	SecurityScanWarning  SecurityScanStatus = "warning"
)

// VulnerabilitySeverity 漏洞严重级别
type VulnerabilitySeverity string

const (
	VulnSeverityCritical VulnerabilitySeverity = "critical"
	VulnSeverityHigh     VulnerabilitySeverity = "high"
	VulnSeverityMedium   VulnerabilitySeverity = "medium"
	VulnSeverityLow      VulnerabilitySeverity = "low"
	VulnSeverityInfo     VulnerabilitySeverity = "info"
)

// ========== 应用信息 ==========

// AppInfo 应用信息
type AppInfo struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	DisplayName  string      `json:"display_name"`
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
	Conflicts    []string    `json:"conflicts,omitempty"`
	Status       AppStatus   `json:"status"`
	Downloads    int64       `json:"downloads"`
	Rating       float64     `json:"rating"`
	RatingCount  int         `json:"rating_count"`
	Featured     bool        `json:"featured"`
	Verified     bool        `json:"verified"`
	DeveloperID  string      `json:"developer_id"`
	ReviewNote   string      `json:"review_note,omitempty"`
	ReviewedBy   string      `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time  `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// InstalledApp 已安装应用
type InstalledApp struct {
	AppID       string            `json:"app_id"`
	Version     string            `json:"version"`
	Status      string            `json:"status"` // running/stopped/error/installing
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ConfigPath  string            `json:"config_path,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Volumes     []VolumeMapping   `json:"volumes,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	Name       string `json:"name,omitempty"`
	Container  int    `json:"container"`
	Host       int    `json:"host"`
	Protocol   string `json:"protocol"`
}

// VolumeMapping 卷映射
type VolumeMapping struct {
	Name      string `json:"name,omitempty"`
	Container string `json:"container"`
	Host      string `json:"host"`
	ReadOnly  bool   `json:"read_only"`
}

// ========== 审核系统 ==========

// ReviewRecord 审核记录
type ReviewRecord struct {
	ID        string       `json:"id"`
	AppID     string       `json:"app_id"`
	Reviewer  string       `json:"reviewer"`
	Action    ReviewAction `json:"action"`
	Note      string       `json:"note"`
	CreatedAt time.Time    `json:"created_at"`
}

// ========== 评分评论系统 ==========

// Rating 评分记录
type Rating struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Score     int       `json:"score"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 搜索与筛选 ==========

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string      `json:"query,omitempty"`
	Category   AppCategory `json:"category,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Sort       SortOption  `json:"sort,omitempty"`
	MinRating  float64     `json:"min_rating,omitempty"`
	Verified   *bool       `json:"verified,omitempty"`
	Page       int         `json:"page,omitempty"`
	PageSize   int         `json:"page_size,omitempty"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Apps       []*AppInfo `json:"apps"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// ========== 发布与更新请求 ==========

// PublishRequest 发布请求
type PublishRequest struct {
	Name         string      `json:"name"`
	DisplayName  string      `json:"display_name"`
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
	Conflicts    []string    `json:"conflicts,omitempty"`
	ComposeYAML  string      `json:"compose_yaml,omitempty"`
}

// ReviewRequest 审核请求
type ReviewRequest struct {
	Action ReviewAction `json:"action"`
	Note   string       `json:"note,omitempty"`
}

// InstallRequest 安装请求
type InstallRequest struct {
	AppID   string            `json:"app_id"`
	Version string            `json:"version,omitempty"`
	EnvVars map[string]string `json:"env_vars,omitempty"`
}

// UpdateRequest 更新请求
type UpdateRequest struct {
	AppID         string `json:"app_id"`
	TargetVersion string `json:"target_version,omitempty"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	AppID         string `json:"app_id"`
	TargetVersion string `json:"target_version"`
}

// RatingRequest 评分请求
type RatingRequest struct {
	Score   int    `json:"score"`
	Comment string `json:"comment,omitempty"`
}

// ========== 依赖管理 ==========

// DependencyResult 依赖解析结果
type DependencyResult struct {
	Resolved     []string   `json:"resolved"`
	Conflicts    []Conflict `json:"conflicts"`
	Warnings     []string   `json:"warnings"`
	TotalApps    int        `json:"total_apps"`
	InstallOrder []string   `json:"install_order"`
}

// Conflict 冲突定义
type Conflict struct {
	AppA     string `json:"app_a"`
	AppB     string `json:"app_b"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"` // error, warning
}

// DependencyGraph 依赖关系图
type DependencyGraph struct {
	Root string              `json:"root"`
	Deps map[string][]string `json:"deps"`
}

// ========== 版本管理 ==========

// VersionInfo 版本信息
type VersionInfo struct {
	Version     string    `json:"version"`
	Changelog   string    `json:"changelog,omitempty"`
	Size        int64     `json:"size"`
	ReleasedAt  time.Time `json:"released_at"`
	IsCurrent   bool      `json:"is_current"`
	IsRollback  bool      `json:"is_rollback"`
}

// VersionHistory 版本历史
type VersionHistory struct {
	AppID      string        `json:"app_id"`
	Versions   []VersionInfo `json:"versions"`
	CurrentIdx int           `json:"current_idx"`
}

// ========== 安全扫描 ==========

// SecurityScanResult 安全扫描结果
type SecurityScanResult struct {
	ID               string               `json:"id"`
	AppID            string               `json:"app_id"`
	Version          string               `json:"version"`
	Status           SecurityScanStatus   `json:"status"`
	Vulnerabilities  []Vulnerability      `json:"vulnerabilities,omitempty"`
	Score            float64              `json:"score"` // 0-100
	ScannedAt        time.Time            `json:"scanned_at"`
	Scanner          string               `json:"scanner"`
	Message          string               `json:"message,omitempty"`
}

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID          string                `json:"id"`
	CVE         string                `json:"cve,omitempty"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Severity    VulnerabilitySeverity `json:"severity"`
	Package     string                `json:"package,omitempty"`
	Version     string                `json:"version,omitempty"`
	FixedIn     string                `json:"fixed_in,omitempty"`
	References  []string              `json:"references,omitempty"`
}

// ========== 推荐引擎 ==========

// Recommendation 推荐结果
type Recommendation struct {
	AppID       string   `json:"app_id"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons"`
	IsNew       bool     `json:"is_new"`
	IsPopular   bool     `json:"is_popular"`
}

// UsageRecord 使用记录
type UsageRecord struct {
	AppID        string    `json:"app_id"`
	LaunchCount  int       `json:"launch_count"`
	TotalRuntime int64     `json:"total_runtime"`
	LastUsed     time.Time `json:"last_used"`
	FirstUsed    time.Time `json:"first_used"`
	Rating       int       `json:"rating"`
}

// UserPreferences 用户偏好
type UserPreferences struct {
	FavoriteCategories  []AppCategory    `json:"favorite_categories,omitempty"`
	FavoriteTags        []string         `json:"favorite_tags,omitempty"`
	BlockedApps         map[string]bool  `json:"blocked_apps,omitempty"`
	InstalledCategories map[string]int   `json:"installed_categories,omitempty"`
}

// ========== Docker Compose 模板 ==========

// ComposeTemplate Docker Compose 模板
type ComposeTemplate struct {
	AppID    string          `json:"app_id"`
	Version  string          `json:"version"`
	YAML     string          `json:"yaml"`
	Services []ComposeService `json:"services"`
}

// ComposeService Compose 服务定义
type ComposeService struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Ports         []PortMapping     `json:"ports,omitempty"`
	Volumes       []VolumeMapping   `json:"volumes,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
}

// ========== 元数据同步 ==========

// SyncSource 同步源
type SyncSource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Type        string    `json:"type"` // official, community, custom
	Enabled     bool      `json:"enabled"`
	Priority    int       `json:"priority"`
	LastSyncAt  time.Time `json:"last_sync_at"`
	AppCount    int       `json:"app_count"`
	Verified    bool      `json:"verified"`
}

// SyncResult 同步结果
type SyncResult struct {
	SourceID    string    `json:"source_id"`
	Added       int       `json:"added"`
	Updated     int       `json:"updated"`
	Removed     int       `json:"removed"`
	Failed      int       `json:"failed"`
	SyncedAt    time.Time `json:"synced_at"`
	Error       string    `json:"error,omitempty"`
}

// ========== 持久化配置 ==========
