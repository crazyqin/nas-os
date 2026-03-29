// Package cloudmount provides cloud storage mounting via rclone
// 网盘挂载框架 - 支持: 阿里云盘、百度网盘、115、夸克
//
// 设计理念:
// - 使用 rclone 作为挂载后端（成熟稳定，支持 40+ 云存储）
// - 统一的挂载管理接口
// - 自动配置生成和热加载
// - 缓存策略优化（针对网盘特性）
//
// 当前版本: v2.317.0（工部维护）
package cloudmount

import (
	"time"
)

// ==================== 类型定义 ====================

// ProviderType 云盘提供商类型.
type ProviderType string

// 支持的云盘提供商.
const (
	ProviderAliyunPan ProviderType = "aliyun"     // 阿里云盘
	ProviderBaiduPan  ProviderType = "baidu"      // 百度网盘
	Provider115       ProviderType = "115"        // 115网盘
	ProviderQuark     ProviderType = "quark"      // 夸克网盘
	ProviderGoogle    ProviderType = "google"     // Google Drive
	ProviderOneDrive  ProviderType = "onedrive"   // Microsoft OneDrive
	ProviderWebDAV    ProviderType = "webdav"     // WebDAV
	ProviderS3        ProviderType = "s3"         // S3 兼容存储
)

// MountStatus 挂载状态.
type MountStatus string

// 挂载状态常量.
const (
	MountStatusIdle      MountStatus = "idle"      // 未挂载
	MountStatusMounting  MountStatus = "mounting"  // 正在挂载
	MountStatusMounted   MountStatus = "mounted"   // 已挂载
	MountStatusUnmounting MountStatus = "unmounting" // 正在卸载
	MountStatusError     MountStatus = "error"     // 挂载错误
	MountStatusPaused    MountStatus = "paused"    // 已暂停
)

// CacheMode 缓存模式.
type CacheMode string

// 缓存模式常量（rclone cache backend）.
const (
	CacheModeOff    CacheMode = "off"     // 不缓存
	CacheModeFull   CacheMode = "full"    // 完整缓存（下载到本地）
	CacheModeWrite  CacheMode = "write"   // 写缓存
	CacheModeRead   CacheMode = "read"    // 读缓存
	CacheModeWarm   CacheMode = "warm"    // 预热缓存
)

// ==================== 配置结构 ====================

// CloudMountConfig 挂载配置.
type CloudMountConfig struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`          // 挂载点名称
	ProviderType ProviderType `json:"providerType"`  // 云盘类型
	Enabled      bool         `json:"enabled"`       // 是否启用
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`

	// 挂载配置
	MountPoint   string `json:"mountPoint"`     // 本地挂载路径（如 /mnt/aliyun）
	RemotePath   string `json:"remotePath"`     // 远程路径（如 /或 /Documents）
	ReadOnly     bool   `json:"readOnly"`       // 只读挂载
	AllowOther   bool   `json:"allowOther"`     // 允许其他用户访问
	AllowRoot    bool   `json:"allowRoot"`      // 允许 root 访问

	// 缓存配置（rclone vfs-cache-mode）
	CacheMode     CacheMode `json:"cacheMode"`      // 缓存模式
	CacheDir      string    `json:"cacheDir"`       // 缓存目录
	CacheMaxAge   int       `json:"cacheMaxAge"`    // 缓存最大保留时间（秒）
	CacheMaxSize  int64     `json:"cacheMaxSize"`   // 缓存最大大小（字节）
	CacheWarm     bool      `json:"cacheWarm"`      // 预热缓存

	// 性能配置
	ChunkSize     int64 `json:"chunkSize"`      // 分块大小（字节）
	BufferSize    int64 `json:"bufferSize"`     // 缓冲区大小（字节）
	MaxTransfers  int   `json:"maxTransfers"`   // 最大并发传输数

	// 网络配置
	BandwidthLimit int64 `json:"bandwidthLimit"` // 带宽限制（KB/s, 0=不限制）
	Timeout        int   `json:"timeout"`        // 超时时间（秒）
	RetryCount     int   `json:"retryCount"`     // 重试次数

	// 认证配置（安全字段不序列化）
	AccessToken  string `json:"-"` // 访问令牌
	RefreshToken string `json:"-"` // 刷新令牌
	UserID       string `json:"userId,omitempty"`
	DriveID      string `json:"driveId,omitempty"` // 阿里云盘需要

	// S3/WebDAV 配置
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	AccessKey string `json:"-"`
	SecretKey string `json:"-"`
	Insecure  bool   `json:"insecure,omitempty"` // 跳过 TLS 验证

	// 状态信息
	Status     MountStatus `json:"status"`
	Error      string      `json:"error,omitempty"`
	MountedAt  time.Time   `json:"mountedAt,omitempty"`
	LastCheck  time.Time   `json:"lastCheck,omitempty"`
}

// RcloneConfig rclone 远程存储配置（生成 rclone.conf）.
type RcloneConfig struct {
	Name     string            `json:"name"`     // 远程名称（在 rclone.conf 中）
	Type     ProviderType      `json:"type"`     // 存储类型
	Options  map[string]string `json:"options"`  // 类型特定选项
	Advanced map[string]string `json:"advanced"` // 高级选项
}

// ==================== 状态结构 ====================

// MountInstance 挂载实例.
type MountInstance struct {
	ID         string            `json:"id"`
	Config     *CloudMountConfig `json:"config"`
	MountPoint string            `json:"mountPoint"`
	Status     MountStatus       `json:"status"`
	PID        int               `json:"pid"`         // rclone 进程 PID
	CmdArgs    []string          `json:"cmdArgs"`     // rclone 命令参数
	Error      string            `json:"error"`
	MountedAt  time.Time         `json:"mountedAt"`
	Stats      *MountStats       `json:"stats"`
}

// MountStats 挂载统计.
type MountStats struct {
	// 存储使用
	TotalSize    int64  `json:"totalSize"`    // 总存储大小
	UsedSize     int64  `json:"usedSize"`     // 已用大小
	FreeSize     int64  `json:"freeSize"`     // 剩余大小
	UsedPercent  float64 `json:"usedPercent"` // 使用百分比

	// 文件统计
	TotalFiles   int64 `json:"totalFiles"`   // 总文件数
	TotalDirs    int64 `json:"totalDirs"`    // 总目录数

	// 缓存统计
	CacheSize    int64 `json:"cacheSize"`    // 缓存大小
	CacheFiles   int64 `json:"cacheFiles"`   // 缓存文件数
	CacheUsed    int64 `json:"cacheUsed"`    // 已用缓存

	// I/O 统计
	BytesRead    int64 `json:"bytesRead"`    // 读取字节
	BytesWritten int64 `json:"bytesWritten"` // 写入字节
	ReadOps      int64 `json:"readOps"`      // 读操作次数
	WriteOps     int64 `json:"writeOps"`     // 写操作次数

	// 时间戳
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MountEvent 挂载事件.
type MountEvent struct {
	ID        string      `json:"id"`
	MountID   string      `json:"mountId"`
	Type      EventType   `json:"type"`
	Status    MountStatus `json:"status"`
	Message   string      `json:"message"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// EventType 事件类型.
type EventType string

const (
	EventMountStart    EventType = "mount_start"
	EventMountSuccess  EventType = "mount_success"
	EventMountFailed   EventType = "mount_failed"
	EventUnmountStart  EventType = "unmount_start"
	EventUnmountSuccess EventType = "unmount_success"
	EventUnmountFailed EventType = "unmount_failed"
	EventError         EventType = "error"
	EventCacheWarm     EventType = "cache_warm"
	EventReconnect     EventType = "reconnect"
)

// ==================== 提供商信息 ====================

// ProviderInfo 提供商信息.
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	RcloneName  string       `json:"rcloneName"`  // rclone 配置中的类型名
	Features    []string     `json:"features"`
	AuthType    AuthType     `json:"authType"`
	AuthURL     string       `json:"authUrl,omitempty"`
}

// AuthType 认证类型.
type AuthType string

const (
	AuthTypeOAuth2    AuthType = "oauth2"    // OAuth2 认证
	AuthTypeToken     AuthType = "token"     // Token 认证
	AuthTypeAPIKey    AuthType = "api_key"   // API Key 认证
	AuthTypeWebDAV    AuthType = "webdav"    // 用户名密码
	AuthTypeS3        AuthType = "s3"        // Access Key + Secret Key
)

// SupportedProviders 返回支持的提供商列表.
func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Type:        ProviderAliyunPan,
			Name:        "阿里云盘",
			Description: "阿里云盘 - 支持 CDN 加速、秒传",
			RcloneName:  "aliauth",  // 使用 aliauth 进行 OAuth2
			Features:    []string{"upload", "download", "stream", "share", "instant_upload"},
			AuthType:    AuthTypeOAuth2,
			AuthURL:     "https://www.aliyundrive.com/",
		},
		{
			Type:        ProviderBaiduPan,
			Name:        "百度网盘",
			Description: "百度网盘 - 中国最大的云存储平台",
			RcloneName:  "baidu",  // 需要第三方 rclone fork
			Features:    []string{"upload", "download", "stream", "share"},
			AuthType:    AuthTypeOAuth2,
			AuthURL:     "https://pan.baidu.com/",
		},
		{
			Type:        Provider115,
			Name:        "115网盘",
			Description: "115网盘 - 支持秒传和离线下载",
			RcloneName:  "115",  // 需要 rclone 115 backend
			Features:    []string{"upload", "download", "stream", "offline_download"},
			AuthType:    AuthTypeOAuth2,
			AuthURL:     "https://115.com/",
		},
		{
			Type:        ProviderQuark,
			Name:        "夸克网盘",
			Description: "夸克网盘 - 大容量、高速度",
			RcloneName:  "quark",  // 需要第三方实现
			Features:    []string{"upload", "download", "stream"},
			AuthType:    AuthTypeToken,
			AuthURL:     "https://pan.quark.cn/",
		},
		{
			Type:        ProviderGoogle,
			Name:        "Google Drive",
			Description: "Google 云端硬盘 - 全球用户最多",
			RcloneName:  "drive",
			Features:    []string{"upload", "download", "stream", "share", "team_drive"},
			AuthType:    AuthTypeOAuth2,
			AuthURL:     "https://drive.google.com/",
		},
		{
			Type:        ProviderOneDrive,
			Name:        "OneDrive",
			Description: "Microsoft OneDrive - Office 365 集成",
			RcloneName:  "onedrive",
			Features:    []string{"upload", "download", "stream", "share"},
			AuthType:    AuthTypeOAuth2,
			AuthURL:     "https://onedrive.live.com/",
		},
		{
			Type:        ProviderWebDAV,
			Name:        "WebDAV",
			Description: "通用 WebDAV 协议",
			RcloneName:  "webdav",
			Features:    []string{"upload", "download", "stream"},
			AuthType:    AuthTypeWebDAV,
		},
		{
			Type:        ProviderS3,
			Name:        "S3 兼容存储",
			Description: "兼容 S3 协议的存储服务",
			RcloneName:  "s3",
			Features:    []string{"upload", "download", "multipart", "bucket"},
			AuthType:    AuthTypeS3,
		},
	}
}

// ==================== 全局配置 ====================

// GlobalConfig 全局挂载配置.
type GlobalConfig struct {
	Version       string           `json:"version"`
	DefaultConfig DefaultSettings  `json:"defaultConfig"`
	Mounts        []CloudMountConfig `json:"mounts"`
	RclonePath    string           `json:"rclonePath"`    // rclone 二进制路径
	RcloneConfPath string          `json:"rcloneConfPath"` // rclone.conf 路径
	LogLevel      string           `json:"logLevel"`
}

// DefaultSettings 默认设置.
type DefaultSettings struct {
	// 挂载默认值
	AllowOther   bool     `json:"allowOther"`
	AllowRoot    bool     `json:"allowRoot"`
	ReadOnly     bool     `json:"readOnly"`

	// 缓存默认值
	CacheMode    CacheMode `json:"cacheMode"`
	CacheDir     string    `json:"cacheDir"`
	CacheMaxAge  int       `json:"cacheMaxAge"`    // 默认 24 小时
	CacheMaxSize int64     `json:"cacheMaxSize"`   // 默认 10GB

	// 性能默认值
	ChunkSize    int64    `json:"chunkSize"`      // 默认 64MB
	BufferSize   int64    `json:"bufferSize"`     // 默认 16MB
	MaxTransfers int      `json:"maxTransfers"`   // 默认 4

	// 网络默认值
	Timeout     int      `json:"timeout"`        // 默认 30 秒
	RetryCount  int      `json:"retryCount"`     // 默认 3 次
}

// GetDefaultSettings 返回默认设置.
func GetDefaultSettings() DefaultSettings {
	return DefaultSettings{
		AllowOther:   false,
		AllowRoot:    true,
		ReadOnly:     false,
		CacheMode:    CacheModeFull,
		CacheDir:     "/var/lib/nas-os/cloudmount/cache",
		CacheMaxAge:  86400,  // 24 hours
		CacheMaxSize: 10 * 1024 * 1024 * 1024, // 10GB
		ChunkSize:    64 * 1024 * 1024, // 64MB
		BufferSize:   16 * 1024 * 1024, // 16MB
		MaxTransfers: 4,
		Timeout:      30,
		RetryCount:   3,
	}
}