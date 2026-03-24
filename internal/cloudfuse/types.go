// Package cloudfuse provides cloud storage mounting via FUSE
// 网盘原生挂载功能 - 类似飞牛fnOS的实现
package cloudfuse

import (
	"time"
)

// MountType 挂载类型
type MountType string

const (
	// MountType115 115网盘
	MountType115 MountType = "115"
	// MountTypeQuark 夸克网盘
	MountTypeQuark MountType = "quark"
	// MountTypeAliyunPan 阿里云盘
	MountTypeAliyunPan MountType = "aliyun_pan"
	// MountTypeOneDrive OneDrive
	MountTypeOneDrive MountType = "onedrive"
	// MountTypeGoogleDrive Google Drive
	MountTypeGoogleDrive MountType = "google_drive"
	// MountTypeWebDAV WebDAV
	MountTypeWebDAV MountType = "webdav"
	// MountTypeS3 S3兼容存储
	MountTypeS3 MountType = "s3"
)

// MountStatus 挂载状态
type MountStatus string

const (
	// MountStatusIdle 空闲
	MountStatusIdle MountStatus = "idle"
	// MountStatusMounting 挂载中
	MountStatusMounting MountStatus = "mounting"
	// MountStatusMounted 已挂载
	MountStatusMounted MountStatus = "mounted"
	// MountStatusUnmounting 卸载中
	MountStatusUnmounting MountStatus = "unmounting"
	// MountStatusError 错误
	MountStatusError MountStatus = "error"
)

// MountConfig 挂载配置
type MountConfig struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         MountType `json:"type"`
	MountPoint   string    `json:"mountPoint"`   // 本地挂载点路径
	RemotePath   string    `json:"remotePath"`   // 远程路径（网盘中的路径）
	Enabled      bool      `json:"enabled"`      // 是否启用
	AutoMount    bool      `json:"autoMount"`    // 开机自动挂载
	ReadOnly     bool      `json:"readOnly"`     // 只读模式
	AllowOther   bool      `json:"allowOther"`   // 允许其他用户访问
	CacheEnabled bool      `json:"cacheEnabled"` // 启用本地缓存
	CacheDir     string    `json:"cacheDir"`     // 缓存目录
	CacheSize    int64     `json:"cacheSize"`    // 缓存大小上限（MB）
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	// 认证配置
	AccessToken  string `json:"-"` // 访问令牌（安全字段）
	RefreshToken string `json:"-"` // 刷新令牌（安全字段）
	UserID       string `json:"userId,omitempty"`
	DriveID      string `json:"driveId,omitempty"` // 阿里云盘需要

	// S3/WebDAV 配置
	Endpoint   string `json:"endpoint,omitempty"`
	Bucket     string `json:"bucket,omitempty"`
	AccessKey  string `json:"accessKey,omitempty"`
	SecretKey  string `json:"-"` // 安全字段
	Region     string `json:"region,omitempty"`
	PathStyle  bool   `json:"pathStyle,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"` // 跳过TLS验证
	ClientID   string `json:"clientId,omitempty"`
	TenantID   string `json:"tenantId,omitempty"`
	RootFolder string `json:"rootFolder,omitempty"`
}

// MountInfo 挂载信息
type MountInfo struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Type           MountType   `json:"type"`
	MountPoint     string      `json:"mountPoint"`
	Status         MountStatus `json:"status"`
	CreatedAt      time.Time   `json:"createdAt"`
	MountedAt      *time.Time  `json:"mountedAt,omitempty"`
	Error          string      `json:"error,omitempty"`
	TotalSize      int64       `json:"totalSize"`      // 总容量（字节）
	UsedSize       int64       `json:"usedSize"`       // 已用容量
	FreeSize       int64       `json:"freeSize"`       // 剩余容量
	FileCount      int64       `json:"fileCount"`      // 文件数量
	FolderCount    int64       `json:"folderCount"`    // 文件夹数量
	ReadBytes      int64       `json:"readBytes"`      // 读取字节数
	WriteBytes     int64       `json:"writeBytes"`     // 写入字节数
	ReadOps        int64       `json:"readOps"`        // 读取操作次数
	WriteOps       int64       `json:"writeOps"`       // 写入操作次数
	CacheHitRate   float64     `json:"cacheHitRate"`   // 缓存命中率
	CacheUsedBytes int64       `json:"cacheUsedBytes"` // 缓存使用量
}

// FileNode 文件节点（用于FUSE）
type FileNode struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	IsDir      bool        `json:"isDir"`
	Size       int64       `json:"size"`
	ModTime    time.Time   `json:"modTime"`
	Mode       uint32      `json:"mode"`       // 文件权限
	UID        uint32      `json:"uid"`        // 用户ID
	GID        uint32      `json:"gid"`        // 组ID
	Children   []*FileNode `json:"children"`   // 子节点（目录）
	RemoteID   string      `json:"remoteId"`   // 远程文件ID
	ParentID   string      `json:"parentId"`   // 父节点ID
	CachedPath string      `json:"cachedPath"` // 本地缓存路径
	Dirty      bool        `json:"dirty"`      // 是否有未上传的修改
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Path       string    `json:"path"`
	LocalPath  string    `json:"localPath"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	AccessTime time.Time `json:"accessTime"`
	HitCount   int64     `json:"hitCount"`
	RemoteID   string    `json:"remoteId"`
}

// MountStats 挂载统计
type MountStats struct {
	MountID           string    `json:"mountId"`
	StartTime         time.Time `json:"startTime"`
	Uptime            int64     `json:"uptime"` // 秒
	TotalReadBytes    int64     `json:"totalReadBytes"`
	TotalWriteBytes   int64     `json:"totalWriteBytes"`
	TotalReadOps      int64     `json:"totalReadOps"`
	TotalWriteOps     int64     `json:"totalWriteOps"`
	CacheHits         int64     `json:"cacheHits"`
	CacheMisses       int64     `json:"cacheMisses"`
	CacheEvictions    int64     `json:"cacheEvictions"`
	PendingUploads    int64     `json:"pendingUploads"`
	FailedUploads     int64     `json:"failedUploads"`
	AvgReadLatencyMs  int64     `json:"avgReadLatencyMs"`
	AvgWriteLatencyMs int64     `json:"avgWriteLatencyMs"`
}

// ProviderInfo 提供商信息
type ProviderInfo struct {
	Type        MountType `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Features    []string  `json:"features"`
	MaxSize     int64     `json:"maxSize,omitempty"` // 最大单文件大小（字节）
}

// SupportedProviders 返回支持的提供商列表
func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Type:        MountTypeAliyunPan,
			Name:        "阿里云盘",
			Description: "阿里云盘 - 支持秒传和分享",
			Features:    []string{"read", "write", "mkdir", "delete", "rename", "instant_upload", "share"},
			MaxSize:     100 * 1024 * 1024 * 1024, // 100GB
		},
		{
			Type:        MountType115,
			Name:        "115网盘",
			Description: "115网盘 - 支持秒传和离线下载",
			Features:    []string{"read", "write", "mkdir", "delete", "rename", "instant_upload", "offline_download"},
			MaxSize:     50 * 1024 * 1024 * 1024, // 50GB
		},
		{
			Type:        MountTypeQuark,
			Name:        "夸克网盘",
			Description: "夸克网盘 - 大容量存储",
			Features:    []string{"read", "write", "mkdir", "delete", "rename"},
			MaxSize:     50 * 1024 * 1024 * 1024, // 50GB
		},
		{
			Type:        MountTypeOneDrive,
			Name:        "OneDrive",
			Description: "Microsoft OneDrive",
			Features:    []string{"read", "write", "mkdir", "delete", "rename", "share"},
			MaxSize:     100 * 1024 * 1024 * 1024, // 100GB
		},
		{
			Type:        MountTypeGoogleDrive,
			Name:        "Google Drive",
			Description: "Google 云端硬盘",
			Features:    []string{"read", "write", "mkdir", "delete", "rename", "share"},
			MaxSize:     5 * 1024 * 1024 * 1024, // 5TB
		},
		{
			Type:        MountTypeWebDAV,
			Name:        "WebDAV",
			Description: "通用 WebDAV 协议",
			Features:    []string{"read", "write", "mkdir", "delete", "rename"},
		},
		{
			Type:        MountTypeS3,
			Name:        "S3兼容存储",
			Description: "AWS S3 / 阿里云OSS / 腾讯云COS 等",
			Features:    []string{"read", "write", "mkdir", "delete", "rename", "multipart"},
		},
	}
}

// MountRequest 挂载请求
type MountRequest struct {
	Name         string    `json:"name" binding:"required"`
	Type         MountType `json:"type" binding:"required"`
	MountPoint   string    `json:"mountPoint" binding:"required"`
	RemotePath   string    `json:"remotePath"`
	ReadOnly     bool      `json:"readOnly"`
	AllowOther   bool      `json:"allowOther"`
	CacheEnabled bool      `json:"cacheEnabled"`
	CacheDir     string    `json:"cacheDir"`
	CacheSize    int64     `json:"cacheSize"` // MB
	AutoMount    bool      `json:"autoMount"`

	// 认证信息
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	UserID       string `json:"userId"`
	DriveID      string `json:"driveId"`

	// S3/WebDAV 认证
	Endpoint     string `json:"endpoint"`
	Bucket       string `json:"bucket"`
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	Region       string `json:"region"`
	PathStyle    bool   `json:"pathStyle"`
	Insecure     bool   `json:"insecure"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	TenantID     string `json:"tenantId"`
	RootFolder   string `json:"rootFolder"`
}

// MountListResponse 挂载列表响应
type MountListResponse struct {
	Total int64       `json:"total"`
	Items []MountInfo `json:"items"`
}

// OperationResult 操作结果
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
