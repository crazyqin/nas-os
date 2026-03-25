// Package directplay provides direct link playback for cloud drives
// 网盘直链播放功能 - 支持百度网盘、123云盘、阿里云盘
package directplay

import (
	"time"
)

// ProviderType 云盘提供商类型
type ProviderType string

const (
	// ProviderBaiduPan 百度网盘
	ProviderBaiduPan ProviderType = "baidu_pan"
	// Provider123Pan 123云盘
	Provider123Pan ProviderType = "123_pan"
	// ProviderAliyunPan 阿里云盘
	ProviderAliyunPan ProviderType = "aliyun_pan"
)

// DirectLinkInfo 直链信息
type DirectLinkInfo struct {
	// 文件标识
	FileID   string `json:"fileId"`
	FileName string `json:"fileName"`
	FilePath string `json:"filePath"`
	FileSize int64  `json:"fileSize"`

	// 直链信息
	URL           string            `json:"url"`           // 直链URL
	DownloadURL   string            `json:"downloadUrl"`   // 下载链接
	StreamURL     string            `json:"streamUrl"`     // 流媒体链接（如果有）
	ExpiresAt     time.Time         `json:"expiresAt"`     // 链接过期时间
	ExpiresIn     int64             `json:"expiresIn"`     // 有效期秒数
	Headers       map[string]string `json:"headers"`       // 请求头（某些网盘需要）
	ExtraParams   map[string]string `json:"extraParams"`   // 额外参数

	// 媒体信息
	Duration     int64  `json:"duration"`     // 时长（秒）
	VideoCodec   string `json:"videoCodec"`   // 视频编码
	AudioCodec   string `json:"audioCodec"`   // 音频编码
	Width        int    `json:"width"`        // 视频宽度
	Height       int    `json:"height"`       // 视频高度
	ThumbnailURL string `json:"thumbnailUrl"` // 缩略图URL

	// 状态
	Provider ProviderType `json:"provider"`
	Cached   bool         `json:"cached"`   // 是否缓存
	Error    string       `json:"error,omitempty"`
}

// DirectPlayRequest 直链播放请求
type DirectPlayRequest struct {
	Provider ProviderType `json:"provider"`
	FileID   string       `json:"fileId"`
	FilePath string       `json:"filePath"` // 可选，某些网盘通过路径获取

	// 认证信息（运行时提供）
	AccessToken  string `json:"-"` // 访问令牌
	RefreshToken string `json:"-"` // 刷新令牌
	DriveID      string `json:"-"` // 云盘ID（阿里云盘需要）

	// 选项
	ForceRefresh bool `json:"forceRefresh"` // 强制刷新直链
	NeedStream   bool `json:"needStream"`   // 是否需要流媒体链接
}

// DirectPlayConfig 直链播放配置
type DirectPlayConfig struct {
	// 总开关
	Enabled bool `json:"enabled"`

	// 各网盘开关
	BaiduPanEnabled  bool `json:"baiduPanEnabled"`
	Pan123Enabled    bool `json:"pan123Enabled"`
	AliyunPanEnabled bool `json:"aliyunPanEnabled"`

	// 缓存配置
	CacheEnabled  bool          `json:"cacheEnabled"`
	CacheTTL      time.Duration `json:"cacheTtl"`
	CacheMaxItems int           `json:"cacheMaxItems"`

	// 超时配置
	RequestTimeout time.Duration `json:"requestTimeout"`
	LinkExpireMin  time.Duration `json:"linkExpireMin"` // 最小有效期检查

	// 并发限制
	MaxConcurrent int `json:"maxConcurrent"`

	// 重试配置
	MaxRetries    int           `json:"maxRetries"`
	RetryInterval time.Duration `json:"retryInterval"`
}

// DefaultDirectPlayConfig 默认配置
func DefaultDirectPlayConfig() *DirectPlayConfig {
	return &DirectPlayConfig{
		Enabled:         true,
		BaiduPanEnabled: true,
		Pan123Enabled:   true,
		AliyunPanEnabled: true,
		CacheEnabled:    true,
		CacheTTL:        30 * time.Minute,
		CacheMaxItems:   1000,
		RequestTimeout:  30 * time.Second,
		LinkExpireMin:   5 * time.Minute,
		MaxConcurrent:   10,
		MaxRetries:      3,
		RetryInterval:   time.Second,
	}
}

// FileInfo 文件信息
type FileInfo struct {
	FileID   string    `json:"fileId"`
	FileName string    `json:"fileName"`
	FilePath string    `json:"filePath"`
	FileSize int64     `json:"fileSize"`
	IsDir    bool      `json:"isDir"`
	ModTime  time.Time `json:"modTime"`
	MimeType string    `json:"mimeType"`

	// 媒体信息
	Duration   int64  `json:"duration"`
	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`

	Provider ProviderType `json:"provider"`
}

// ListFilesRequest 列出文件请求
type ListFilesRequest struct {
	Provider  ProviderType `json:"provider"`
	DirPath   string       `json:"dirPath"`
	Recursive bool         `json:"recursive"`
	Page      int          `json:"page"`
	PageSize  int          `json:"pageSize"`

	// 认证信息
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
	DriveID      string `json:"-"`
}

// ListFilesResponse 列出文件响应
type ListFilesResponse struct {
	Files      []FileInfo `json:"files"`
	TotalCount int64      `json:"totalCount"`
	HasMore    bool       `json:"hasMore"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// ProviderInfo 网盘信息
type ProviderInfo struct {
	Type         ProviderType `json:"type"`
	Name         string       `json:"name"`
	Enabled      bool         `json:"enabled"`
	Connected    bool         `json:"connected"`
	UserName     string       `json:"userName"`
	TotalSpace   int64        `json:"totalSpace"`
	UsedSpace    int64        `json:"usedSpace"`
	Expired      bool         `json:"expired"`     // token是否过期
	ExpiresAt    time.Time    `json:"expiresAt"`   // token过期时间
}

// DirectPlayStatus 直链播放状态
type DirectPlayStatus struct {
	Enabled       bool          `json:"enabled"`
	Providers     []ProviderInfo `json:"providers"`
	CacheSize     int           `json:"cacheSize"`
	ActiveStreams int           `json:"activeStreams"`
}

// StreamSession 流媒体会话
type StreamSession struct {
	ID           string          `json:"id"`
	Provider     ProviderType    `json:"provider"`
	FileID       string          `json:"fileId"`
	FileName     string          `json:"fileName"`
	DirectLink   *DirectLinkInfo `json:"directLink"`
	StartTime    time.Time       `json:"startTime"`
	LastAccess   time.Time       `json:"lastAccess"`
	Viewers      int             `json:"viewers"`
	BytesServed  int64           `json:"bytesServed"`
}