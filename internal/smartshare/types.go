// Package smartshare 提供智能文件分享系统，支持多种分享模式、访问控制、统计分析、水印、安全预览等功能。
package smartshare

import "time"

// ShareMode 分享模式
type ShareMode string

const (
	ShareModePublic   ShareMode = "public"    // 公开分享
	ShareModePassword ShareMode = "password"  // 密码保护
	ShareModePrivate  ShareMode = "private"   // 指定用户
	ShareModeOnce     ShareMode = "once"      // 一次性链接
)

// ShareStatus 分享链接状态
type ShareStatus string

const (
	ShareStatusActive   ShareStatus = "active"    // 活跃
	ShareStatusExpired  ShareStatus = "expired"   // 已过期
	ShareStatusRevoked  ShareStatus = "revoked"   // 已撤销
	ShareStatusExhausted ShareStatus = "exhausted" // 下载次数已用完
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceDesktop  DeviceType = "desktop"
	DeviceMobile   DeviceType = "mobile"
	DeviceTablet   DeviceType = "tablet"
	DeviceBot      DeviceType = "bot"
	DeviceUnknown  DeviceType = "unknown"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// WatermarkPosition 水印位置
type WatermarkPosition string

const (
	WatermarkTopLeft      WatermarkPosition = "top_left"
	WatermarkTopRight     WatermarkPosition = "top_right"
	WatermarkBottomLeft   WatermarkPosition = "bottom_left"
	WatermarkBottomRight  WatermarkPosition = "bottom_right"
	WatermarkCenter       WatermarkPosition = "center"
	WatermarkTiled        WatermarkPosition = "tiled"        // 平铺
)

// ShareLink 分享链接
type ShareLink struct {
	ID          string        `json:"id"`
	Token       string        `json:"token"`
	ShortURL    string        `json:"short_url"`
	FullURL     string        `json:"full_url"`
	FilePath    string        `json:"file_path"`
	FileName    string        `json:"file_name"`
	FileSize    int64         `json:"file_size"`
	FileType    string        `json:"file_type"`
	Mode        ShareMode     `json:"mode"`
	Status      ShareStatus   `json:"status"`
	CreatorID   string        `json:"creator_id"`
	CreatorName string        `json:"creator_name"`
	Description string        `json:"description,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Password    string        `json:"password,omitempty"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`
	MaxDownloads int          `json:"max_downloads,omitempty"`
	DownloadCount int         `json:"download_count"`
	ViewCount   int           `json:"view_count"`
	AllowedUsers []string     `json:"allowed_users,omitempty"`
	IPWhitelist []string      `json:"ip_whitelist,omitempty"`
	EnableWatermark bool      `json:"enable_watermark"`
	WatermarkConfig *WatermarkConfig `json:"watermark_config,omitempty"`
	EnablePreview   bool      `json:"enable_preview"`
	BrandingConfig  *BrandingConfig  `json:"branding_config,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	LastAccessAt *time.Time   `json:"last_access_at,omitempty"`
}

// SharePolicy 分享策略
type SharePolicy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	DefaultMode     ShareMode     `json:"default_mode"`
	MaxExpiration   time.Duration `json:"max_expiration,omitempty"`
	DefaultExpiration time.Duration `json:"default_expiration,omitempty"`
	MaxDownloads    int           `json:"max_downloads,omitempty"`
	AllowPassword   bool          `json:"allow_password"`
	RequirePassword bool          `json:"require_password"`
	AllowWatermark  bool          `json:"allow_watermark"`
	AllowPreview    bool          `json:"allow_preview"`
	AllowBranding   bool          `json:"allow_branding"`
	MaxFileSize     int64         `json:"max_file_size,omitempty"`
	AllowedFileTypes []string     `json:"allowed_file_types,omitempty"`
	IPWhitelistEnabled bool      `json:"ip_whitelist_enabled"`
	AnalyticsEnabled   bool      `json:"analytics_enabled"`
	NotificationEnabled bool     `json:"notification_enabled"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// AccessLog 访问日志
type AccessLog struct {
	ID          string     `json:"id"`
	ShareID     string     `json:"share_id"`
	Token       string     `json:"token"`
	IPAddress   string     `json:"ip_address"`
	UserAgent   string     `json:"user_agent"`
	DeviceType  DeviceType `json:"device_type"`
	OS          string     `json:"os,omitempty"`
	Browser     string     `json:"browser,omitempty"`
	Country     string     `json:"country,omitempty"`
	Region      string     `json:"region,omitempty"`
	City        string     `json:"city,omitempty"`
	Referer     string     `json:"referer,omitempty"`
	Action      string     `json:"action"` // view, download, preview
	Success     bool       `json:"success"`
	FailReason  string     `json:"fail_reason,omitempty"`
	Duration    int64      `json:"duration_ms,omitempty"` // 访问时长（毫秒）
	Timestamp   time.Time  `json:"timestamp"`
}

// ShareAnalytics 分享统计分析
type ShareAnalytics struct {
	ShareID         string            `json:"share_id"`
	TotalViews      int               `json:"total_views"`
	UniqueVisitors  int               `json:"unique_visitors"`
	TotalDownloads  int               `json:"total_downloads"`
	UniqueDownloaders int             `json:"unique_downloaders"`
	DeviceBreakdown map[DeviceType]int `json:"device_breakdown"`
	OSBreakdown     map[string]int     `json:"os_breakdown"`
	BrowserBreakdown map[string]int    `json:"browser_breakdown"`
	CountryBreakdown map[string]int    `json:"country_breakdown"`
	RegionBreakdown  map[string]int    `json:"region_breakdown"`
	HourlyTraffic   map[int]int        `json:"hourly_traffic"`   // 0-23小时流量分布
	DailyTraffic    map[string]int     `json:"daily_traffic"`    // 日期流量分布
	TopReferers     []RefererStat      `json:"top_referers"`
	RecentLogs      []*AccessLog       `json:"recent_logs,omitempty"`
	AvgDuration     float64            `json:"avg_duration_ms"`  // 平均访问时长
	BounceRate      float64            `json:"bounce_rate"`      // 跳出率
	GeneratedAt     time.Time          `json:"generated_at"`
}

// RefererStat 来源统计
type RefererStat struct {
	Referer string `json:"referer"`
	Count   int    `json:"count"`
}

// WatermarkConfig 水印配置
type WatermarkConfig struct {
	Text       string            `json:"text"`
	FontSize   int               `json:"font_size"`
	FontColor  string            `json:"font_color"`
	Opacity    float64           `json:"opacity"` // 0.0-1.0
	Position   WatermarkPosition `json:"position"`
	Rotation   float64           `json:"rotation"` // 旋转角度
	Spacing    int               `json:"spacing"`  // 平铺间距
	ImageURL   string            `json:"image_url,omitempty"` // 图片水印
}

// BrandingConfig 品牌化配置
type BrandingConfig struct {
	CompanyName    string `json:"company_name,omitempty"`
	LogoURL        string `json:"logo_url,omitempty"`
	FaviconURL     string `json:"favicon_url,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	SecondaryColor string `json:"secondary_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	TextColor      string `json:"text_color,omitempty"`
	FooterText     string `json:"footer_text,omitempty"`
	CustomCSS      string `json:"custom_css,omitempty"`
	CustomHTML     string `json:"custom_html,omitempty"`
	BannerImageURL string `json:"banner_image_url,omitempty"`
}

// PreviewConfig 预览配置
type PreviewConfig struct {
	AllowDownload    bool `json:"allow_download"`
	AllowPrint       bool `json:"allow_print"`
	AllowCopy        bool `json:"allow_copy"`
	WatermarkPreview bool `json:"watermark_preview"`
	MaxPreviewPages  int  `json:"max_preview_pages,omitempty"` // PDF最大预览页数
	PreviewQuality   int  `json:"preview_quality,omitempty"`  // 预览质量 1-100
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	OnView      bool   `json:"on_view"`
	OnDownload  bool   `json:"on_download"`
	OnFirstAccess bool `json:"on_first_access"`
	OnExpired   bool   `json:"on_expired"`
	OnAnomaly   bool   `json:"on_anomaly"`    // 异常访问告警
	WebhookURL  string `json:"webhook_url,omitempty"`
	EmailTo     string `json:"email_to,omitempty"`
	MaxPerHour  int    `json:"max_per_hour"`  // 每小时最大通知数
}

// NotifyEvent 通知事件
type NotifyEvent struct {
	ID        string     `json:"id"`
	ShareID   string     `json:"share_id"`
	EventType string     `json:"event_type"`
	Level     AlertLevel `json:"level"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	IPAddress string     `json:"ip_address,omitempty"`
	UserAgent string     `json:"user_agent,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// BatchShareRequest 批量分享请求
type BatchShareRequest struct {
	FilePaths  []string      `json:"file_paths" binding:"required,min=1"`
	Mode       ShareMode     `json:"mode"`
	Password   string        `json:"password,omitempty"`
	ExpiresIn  time.Duration `json:"expires_in,omitempty"`
	MaxDownloads int         `json:"max_downloads,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
}

// BatchShareResult 批量分享结果
type BatchShareResult struct {
	Success []*ShareLink `json:"success"`
	Failed  []BatchError `json:"failed,omitempty"`
	Total   int          `json:"total"`
}

// BatchError 批量操作错误
type BatchError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

// DefaultSharePolicy 默认分享策略
func DefaultSharePolicy() *SharePolicy {
	return &SharePolicy{
		ID:                  "policy-default",
		Name:                "默认策略",
		DefaultMode:         ShareModePassword,
		DefaultExpiration:   7 * 24 * time.Hour,  // 7天
		MaxExpiration:       365 * 24 * time.Hour, // 1年
		MaxDownloads:        100,
		AllowPassword:       true,
		RequirePassword:     false,
		AllowWatermark:      true,
		AllowPreview:        true,
		AllowBranding:       true,
		MaxFileSize:         10 * 1024 * 1024 * 1024, // 10GB
		IPWhitelistEnabled:  true,
		AnalyticsEnabled:    true,
		NotificationEnabled: true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// DefaultWatermarkConfig 默认水印配置
func DefaultWatermarkConfig() *WatermarkConfig {
	return &WatermarkConfig{
		Text:      "Confidential",
		FontSize:  14,
		FontColor: "#999999",
		Opacity:   0.3,
		Position:  WatermarkTiled,
		Rotation:  -30,
		Spacing:   100,
	}
}

// DefaultBrandingConfig 默认品牌配置
func DefaultBrandingConfig() *BrandingConfig {
	return &BrandingConfig{
		CompanyName:     "NAS-OS",
		PrimaryColor:    "#1890ff",
		SecondaryColor:  "#52c41a",
		BackgroundColor: "#ffffff",
		TextColor:       "#333333",
		FooterText:      "Powered by NAS-OS SmartShare",
	}
}

// DefaultNotifyConfig 默认通知配置
func DefaultNotifyConfig() *NotifyConfig {
	return &NotifyConfig{
		OnView:       false,
		OnDownload:   true,
		OnFirstAccess: true,
		OnExpired:    true,
		OnAnomaly:    true,
		MaxPerHour:   10,
	}
}

// DefaultPreviewConfig 默认预览配置
func DefaultPreviewConfig() *PreviewConfig {
	return &PreviewConfig{
		AllowDownload:    false,
		AllowPrint:       false,
		AllowCopy:        false,
		WatermarkPreview: true,
		MaxPreviewPages:  50,
		PreviewQuality:   80,
	}
}
