// Package sharelinks 提供文件共享链接增强功能
// 支持：公开/私有/加密链接、密码保护、过期时间、访问统计、
//       批量分享、短链接、预览、二维码、防盗链等
package sharelinks

import (
	"errors"
	"time"
)

// LinkType 链接类型
type LinkType string

const (
	LinkTypePublic  LinkType = "public"  // 公开链接
	LinkTypePrivate LinkType = "private" // 私有链接（需登录）
	LinkTypeEncrypted LinkType = "encrypted" // 加密链接（需密码）
)

// PreviewType 预览类型
type PreviewType string

const (
	PreviewTypeImage    PreviewType = "image"
	PreviewTypeDocument PreviewType = "document"
	PreviewTypeVideo    PreviewType = "video"
	PreviewTypeAudio    PreviewType = "audio"
	PreviewTypeNone     PreviewType = "none"
)

// ShareLink 共享链接
type ShareLink struct {
	ID              string      `json:"id"`
	ShortCode       string      `json:"shortCode"`       // Base62短码
	Path            string      `json:"path"`            // 文件/目录路径
	Name            string      `json:"name"`            // 显示名称
	Type            LinkType    `json:"type"`            // 链接类型
	Token           string      `json:"token"`           // 访问令牌
	Password        string      `json:"password,omitempty"` // 访问密码
	MaxDownloads    int         `json:"maxDownloads"`    // 最大下载次数（0=不限）
	DownloadCount   int         `json:"downloadCount"`   // 已下载次数
	ExpiresAt       *time.Time  `json:"expiresAt"`       // 过期时间
	CreatedBy       string      `json:"createdBy"`       // 创建者
	CreatedAt       time.Time   `json:"createdAt"`       // 创建时间
	UpdatedAt       time.Time   `json:"updatedAt"`       // 更新时间
	IsActive        bool        `json:"isActive"`        // 是否启用
	Description     string      `json:"description"`     // 描述
	Tags            []string    `json:"tags"`            // 标签
	PreviewType     PreviewType `json:"previewType"`     // 预览类型
	RefererWhitelist []string   `json:"refererWhitelist,omitempty"` // Referer白名单

	// 批量分享相关
	IsBatch         bool     `json:"isBatch"`         // 是否批量分享
	BatchPaths      []string `json:"batchPaths,omitempty"` // 批量路径列表

	// 访问统计
	AccessLog       []AccessEntry `json:"accessLog"`
	UniqueVisitors  int           `json:"uniqueVisitors"` // 独立访客数
	LastAccessedAt  *time.Time    `json:"lastAccessedAt"` // 最后访问时间
}

// AccessEntry 访问记录
type AccessEntry struct {
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
	Referer   string    `json:"referer,omitempty"`
	Action    string    `json:"action"` // view, download, preview
	Timestamp time.Time `json:"timestamp"`
}

// ShareStats 共享统计
type ShareStats struct {
	TotalLinks      int   `json:"totalLinks"`
	ActiveLinks     int   `json:"activeLinks"`
	ExpiredLinks    int   `json:"expiredLinks"`
	DisabledLinks   int   `json:"disabledLinks"`
	TotalDownloads  int64 `json:"totalDownloads"`
	TotalViews      int64 `json:"totalViews"`
	TotalPreviews   int64 `json:"totalPreviews"`
}

// LinkConfig 链接配置
type LinkConfig struct {
	DefaultExpiryHours int    `json:"defaultExpiryHours"` // 默认过期时间（小时）
	MaxFileSize        int64  `json:"maxFileSize"`        // 最大文件大小
	EnablePassword     bool   `json:"enablePassword"`     // 启用密码保护
	EnableAccessLog    bool   `json:"enableAccessLog"`    // 启用访问日志
	EnableQRCode       bool   `json:"enableQRCode"`       // 启用二维码
	BaseURL            string `json:"baseUrl"`            // 基础URL
	ShortCodeLength    int    `json:"shortCodeLength"`    // 短码长度
	MaxBatchSize       int    `json:"maxBatchSize"`       // 最大批量数
}

// DefaultConfig 默认配置
func DefaultConfig() *LinkConfig {
	return &LinkConfig{
		DefaultExpiryHours: 72,
		MaxFileSize:        10 * 1024 * 1024 * 1024, // 10GB
		EnablePassword:     true,
		EnableAccessLog:    true,
		EnableQRCode:       true,
		ShortCodeLength:    6,
		MaxBatchSize:       100,
	}
}

// 错误定义
var (
	ErrLinkNotFound      = errors.New("share link not found")
	ErrLinkExpired       = errors.New("share link has expired")
	ErrLinkDisabled      = errors.New("share link is disabled")
	ErrDownloadLimit     = errors.New("download limit reached")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrRefererDenied     = errors.New("referer not in whitelist")
	ErrInvalidShortCode  = errors.New("invalid short code")
	ErrBatchSizeExceeded = errors.New("batch size exceeded")
	ErrInvalidPath       = errors.New("invalid path")
)
