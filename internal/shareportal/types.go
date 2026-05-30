// Package shareportal 提供文件分享门户功能
package shareportal

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrShareNotFound 分享链接不存在
	ErrShareNotFound = errors.New("分享链接不存在")
	// ErrShareExpired 分享链接已过期
	ErrShareExpired = errors.New("分享链接已过期")
	// ErrShareInactive 分享链接已停用
	ErrShareInactive = errors.New("分享链接已停用")
	// ErrPasswordRequired 需要密码
	ErrPasswordRequired = errors.New("需要密码验证")
	// ErrPasswordWrong 密码错误
	ErrPasswordWrong = errors.New("密码错误")
	// ErrMaxDownloadsExceeded 已达最大下载次数
	ErrMaxDownloadsExceeded = errors.New("已达最大下载次数")
	// ErrUploadNotAllowed 不允许上传
	ErrUploadNotAllowed = errors.New("不允许上传")
	// ErrPreviewNotAllowed 不允许预览
	ErrPreviewNotAllowed = errors.New("不允许预览")
	// ErrBrandingNotFound 品牌配置不存在
	ErrBrandingNotFound = errors.New("品牌配置不存在")
	// ErrPortalNotFound 门户不存在
	ErrPortalNotFound = errors.New("门户不存在")
)

// ========== 访问动作类型 ==========

// AccessAction 访问动作类型
type AccessAction string

const (
	// ActionView 查看
	ActionView AccessAction = "view"
	// ActionDownload 下载
	ActionDownload AccessAction = "download"
	// ActionUpload 上传
	ActionUpload AccessAction = "upload"
)

// ========== 核心数据结构 ==========

// ShareLink 分享链接
type ShareLink struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	FilePath       string    `json:"file_path"`
	CreatorID      string    `json:"creator_id"`
	CreatorName    string    `json:"creator_name,omitempty"`
	Password       string    `json:"password,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxDownloads   int       `json:"max_downloads,omitempty"`   // 0 表示无限制
	DownloadCount  int       `json:"download_count"`
	ViewCount      int       `json:"view_count"`
	AllowPreview   bool      `json:"allow_preview"`
	AllowDownload  bool      `json:"allow_download"`
	AllowUpload    bool      `json:"allow_upload"`
	IsActive       bool      `json:"is_active"`
	BrandingID     string    `json:"branding_id,omitempty"`
	ShortURL       string    `json:"short_url"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ShareBranding 分享品牌配置
type ShareBranding struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LogoURL        string `json:"logo_url,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	SecondaryColor string `json:"secondary_color,omitempty"`
	FooterText     string `json:"footer_text,omitempty"`
	CustomCSS      string `json:"custom_css,omitempty"`
	IsDefault      bool   `json:"is_default"`
}

// ShareAccess 分享访问记录
type ShareAccess struct {
	ID               string       `json:"id"`
	ShareLinkID      string       `json:"share_link_id"`
	VisitorIP        string       `json:"visitor_ip"`
	VisitorUA        string       `json:"visitor_user_agent,omitempty"`
	Action           AccessAction `json:"action"`
	FileName         string       `json:"file_name,omitempty"`
	BytesTransferred int64        `json:"bytes_transferred,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// FileStat 文件统计
type FileStat struct {
	FileName       string    `json:"file_name"`
	DownloadCount  int       `json:"download_count"`
	LastDownloaded time.Time `json:"last_downloaded,omitempty"`
}

// DailyStat 日统计
type DailyStat struct {
	Date           string `json:"date"`
	Views          int    `json:"views"`
	Downloads      int    `json:"downloads"`
	UniqueVisitors int    `json:"unique_visitors"`
}

// ShareStats 分享统计
type ShareStats struct {
	ShareLinkID    string        `json:"share_link_id"`
	TotalViews     int           `json:"total_views"`
	TotalDownloads int           `json:"total_downloads"`
	UniqueVisitors int           `json:"unique_visitors"`
	TopFiles       []FileStat    `json:"top_files,omitempty"`
	DailyStats     []DailyStat   `json:"daily_stats,omitempty"`
	RecentAccess   []ShareAccess `json:"recent_access,omitempty"`
}

// ShareUpload 分享上传记录
type ShareUpload struct {
	ShareLinkID string `json:"share_link_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	MimeType    string `json:"mime_type,omitempty"`
	UploadedBy  string `json:"uploaded_by,omitempty"`
}

// SharePortal 分享门户
type SharePortal struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	ShareIDs     []string  `json:"share_ids"`
	BrandingID   string    `json:"branding_id,omitempty"`
	CustomDomain string    `json:"custom_domain,omitempty"`
	IsPublic     bool      `json:"is_public"`
	CreatedAt    time.Time `json:"created_at"`
}
