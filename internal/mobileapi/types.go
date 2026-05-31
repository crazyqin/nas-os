// Package mobileapi 提供移动端远程管理API服务
// 支持设备注册、认证、远程控制、文件访问、推送通知和数据同步
package mobileapi

import (
	"sync"
	"time"
)

// Platform 移动平台类型.
type Platform string

const (
	PlatformIOS     Platform = "ios"     // iOS
	PlatformAndroid Platform = "android" // Android
)

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	StatusOnline  DeviceStatus = "online"  // 在线
	StatusOffline DeviceStatus = "offline" // 离线
	StatusBlocked DeviceStatus = "blocked" // 已封禁
)

// PushProvider 推送服务提供商.
type PushProvider string

const (
	ProviderFCM  PushProvider = "fcm"  // Firebase Cloud Messaging (Android)
	ProviderAPNs PushProvider = "apns" // Apple Push Notification Service (iOS)
)

// SyncItemType 同步数据类型.
type SyncItemType string

const (
	SyncPhoto    SyncItemType = "photo"    // 照片
	SyncVideo    SyncItemType = "video"    // 视频
	SyncFile     SyncItemType = "file"     // 文件
	SyncContact  SyncItemType = "contact"  // 联系人
	SyncDocument SyncItemType = "document" // 文档
)

// SyncStatus 同步状态.
type SyncStatus string

const (
	SyncPending    SyncStatus = "pending"    // 待同步
	SyncInProgress SyncStatus = "in_progress" // 同步中
	SyncCompleted  SyncStatus = "completed"  // 已完成
	SyncFailed     SyncStatus = "failed"     // 失败
	SyncConflict   SyncStatus = "conflict"   // 冲突
)

// MobileDevice 移动设备信息.
type MobileDevice struct {
	ID           string       `json:"id"`           // 设备唯一标识
	UserID       string       `json:"userId"`       // 关联用户ID
	DeviceName   string       `json:"deviceName"`   // 设备名称
	Platform     Platform     `json:"platform"`     // 平台类型
	OSVersion    string       `json:"osVersion"`    // 操作系统版本
	AppVersion   string       `json:"appVersion"`   // 应用版本
	PushToken    string       `json:"pushToken"`    // 推送Token
	PushProvider PushProvider `json:"pushProvider"` // 推送服务提供商
	Status       DeviceStatus `json:"status"`       // 设备状态
	LastSeenAt   time.Time    `json:"lastSeenAt"`   // 最后在线时间
	RegisteredAt time.Time    `json:"registeredAt"` // 注册时间
	UpdatedAt    time.Time    `json:"updatedAt"`    // 更新时间

	// 设备能力
	Capabilities []string `json:"capabilities,omitempty"` // 支持的功能列表
}

// AuthToken 认证令牌.
type AuthToken struct {
	AccessToken  string    `json:"accessToken"`  // 访问令牌 (JWT)
	RefreshToken string    `json:"refreshToken"` // 刷新令牌
	TokenType    string    `json:"tokenType"`    // 令牌类型 (Bearer)
	ExpiresAt    time.Time `json:"expiresAt"`    // 过期时间
	DeviceID     string    `json:"deviceId"`     // 关联设备ID
	CreatedAt    time.Time `json:"createdAt"`    // 创建时间
}

// RefreshToken 刷新令牌信息.
type RefreshToken struct {
	Token     string    `json:"token"`     // 令牌值
	DeviceID  string    `json:"deviceId"`  // 关联设备ID
	UserID    string    `json:"userId"`    // 关联用户ID
	ExpiresAt time.Time `json:"expiresAt"` // 过期时间
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	Revoked   bool      `json:"revoked"`   // 是否已撤销
}

// Session 会话信息.
type Session struct {
	ID           string    `json:"id"`           // 会话ID
	UserID       string    `json:"userId"`       // 用户ID
	DeviceID     string    `json:"deviceId"`     // 设备ID
	AccessToken  string    `json:"accessToken"`  // 当前访问令牌
	RefreshToken string    `json:"refreshToken"` // 当前刷新令牌
	IPAddress    string    `json:"ipAddress"`    // 客户端IP
	UserAgent    string    `json:"userAgent"`    // 客户端UA
	ExpiresAt    time.Time `json:"expiresAt"`    // 过期时间
	LastActiveAt time.Time `json:"lastActiveAt"` // 最后活跃时间
	CreatedAt    time.Time `json:"createdAt"`    // 创建时间
	Revoked      bool      `json:"revoked"`      // 是否已撤销
}

// PushNotification 推送通知.
type PushNotification struct {
	ID        string       `json:"id"`        // 通知ID
	DeviceID  string       `json:"deviceId"`  // 目标设备ID
	Provider  PushProvider `json:"provider"`   // 推送服务提供商
	Title     string       `json:"title"`      // 通知标题
	Body      string       `json:"body"`       // 通知内容
	Data      map[string]string `json:"data,omitempty"` // 自定义数据
	Image     string       `json:"image,omitempty"`    // 图片URL
	Priority  string       `json:"priority,omitempty"` // 优先级 (high/normal)
	Badge     int          `json:"badge,omitempty"`     // 角标数
	Sound     string       `json:"sound,omitempty"`     // 提示音
	CreatedAt time.Time    `json:"createdAt"`           // 创建时间
	SentAt    *time.Time   `json:"sentAt,omitempty"`    // 发送时间
	Sent      bool         `json:"sent"`                // 是否已发送
	Error     string       `json:"error,omitempty"`     // 发送错误
}

// SyncItem 同步数据项.
type SyncItem struct {
	ID          string       `json:"id"`          // 同步项ID
	UserID      string       `json:"userId"`      // 用户ID
	DeviceID    string       `json:"deviceId"`    // 设备ID
	Type        SyncItemType `json:"type"`         // 数据类型
	LocalPath   string       `json:"localPath"`   // 设备本地路径
	RemotePath  string       `json:"remotePath"`  // 远程NAS路径
	FileName    string       `json:"fileName"`    // 文件名
	FileSize    int64        `json:"fileSize"`    // 文件大小
	MimeType    string       `json:"mimeType"`    // MIME类型
	Checksum    string       `json:"checksum"`    // 文件校验和
	Status      SyncStatus   `json:"status"`      // 同步状态
	Progress    float64      `json:"progress"`    // 同步进度 (0-100)
	Error       string       `json:"error,omitempty"` // 错误信息
	RetryCount  int          `json:"retryCount"`  // 重试次数
	CreatedAt   time.Time    `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time    `json:"updatedAt"`   // 更新时间
	CompletedAt *time.Time   `json:"completedAt,omitempty"` // 完成时间
}

// SyncConfig 同步配置.
type SyncConfig struct {
	Enabled          bool     `json:"enabled"`          // 启用同步
	AutoSyncPhotos   bool     `json:"autoSyncPhotos"`   // 自动同步照片
	AutoSyncVideos   bool     `json:"autoSyncVideos"`   // 自动同步视频
	AutoSyncContacts bool     `json:"autoSyncContacts"` // 自动同步联系人
	SyncOnWifiOnly   bool     `json:"syncOnWifiOnly"`   // 仅WiFi下同步
	MaxFileSize      int64    `json:"maxFileSize"`      // 最大同步文件大小
	ExcludePatterns  []string `json:"excludePatterns"`  // 排除模式
	RemoteBasePath   string   `json:"remoteBasePath"`   // 远程基础路径
}

// SyncStats 同步统计.
type SyncStats struct {
	mu sync.RWMutex `json:"-"`

	// 总体统计
	TotalItems    int64 `json:"totalItems"`    // 总同步项数
	CompletedItems int64 `json:"completedItems"` // 已完成数
	FailedItems   int64 `json:"failedItems"`    // 失败数
	PendingItems  int64 `json:"pendingItems"`   // 待处理数

	// 按类型统计
	Photos    int64 `json:"photos"`    // 照片数
	Videos    int64 `json:"videos"`    // 视频数
	Files     int64 `json:"files"`     // 文件数
	Contacts  int64 `json:"contacts"`  // 联系人数
	Documents int64 `json:"documents"` // 文档数

	// 空间统计
	TotalBytes    int64 `json:"totalBytes"`    // 总字节数
	SyncedBytes   int64 `json:"syncedBytes"`   // 已同步字节数

	// 时间统计
	LastSyncTime  time.Time     `json:"lastSyncTime"`  // 最后同步时间
	TotalSyncTime time.Duration `json:"totalSyncTime"` // 总同步耗时

	// 状态
	IsSyncing bool `json:"isSyncing"` // 是否正在同步
}

// GetSnapshot 获取统计快照（线程安全）.
func (s *SyncStats) GetSnapshot() *SyncStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &SyncStats{
		TotalItems:     s.TotalItems,
		CompletedItems: s.CompletedItems,
		FailedItems:    s.FailedItems,
		PendingItems:   s.PendingItems,
		Photos:         s.Photos,
		Videos:         s.Videos,
		Files:          s.Files,
		Contacts:       s.Contacts,
		Documents:      s.Documents,
		TotalBytes:     s.TotalBytes,
		SyncedBytes:    s.SyncedBytes,
		LastSyncTime:   s.LastSyncTime,
		TotalSyncTime:  s.TotalSyncTime,
		IsSyncing:      s.IsSyncing,
	}
}

// ========== 推送通知偏好 ==========

// NotificationCategory 通知分类.
type NotificationCategory string

const (
	CategorySystem    NotificationCategory = "system"    // 系统通知
	CategoryBackup    NotificationCategory = "backup"    // 备份通知
	CategorySecurity  NotificationCategory = "security"  // 安全通知
	CategoryShare     NotificationCategory = "share"     // 分享通知
	CategoryUpdate    NotificationCategory = "update"    // 更新通知
	CategoryAlert     NotificationCategory = "alert"     // 告警通知
)

// NotificationPreference 通知偏好设置.
type NotificationPreference struct {
	UserID       string               `json:"userId"`       // 用户ID
	DeviceID     string               `json:"deviceId"`     // 设备ID
	Category     NotificationCategory `json:"category"`     // 通知分类
	Enabled      bool                 `json:"enabled"`      // 是否启用
	Sound        bool                 `json:"sound"`        // 是否播放声音
	Vibrate      bool                 `json:"vibrate"`      // 是否震动
	Badge        bool                 `json:"badge"`        // 是否显示角标
	QuietStart   string               `json:"quietStart"`   // 免打扰开始时间 (HH:MM)
	QuietEnd     string               `json:"quietEnd"`     // 免打扰结束时间 (HH:MM)
	UpdatedAt    time.Time            `json:"updatedAt"`    // 更新时间
}

// NotificationHistoryItem 通知历史记录.
type NotificationHistoryItem struct {
	ID         string               `json:"id"`         // 记录ID
	UserID     string               `json:"userId"`     // 用户ID
	DeviceID   string               `json:"deviceId"`   // 设备ID
	Category   NotificationCategory `json:"category"`   // 通知分类
	Title      string               `json:"title"`      // 标题
	Body       string               `json:"body"`       // 内容
	Data       map[string]string    `json:"data,omitempty"` // 自定义数据
	Read       bool                 `json:"read"`       // 是否已读
	ReadAt     *time.Time           `json:"readAt,omitempty"` // 阅读时间
	CreatedAt  time.Time            `json:"createdAt"`  // 创建时间
}

// NotificationReadRequest 批量标记已读请求.
type NotificationReadRequest struct {
	IDs     []string `json:"ids"`               // 通知ID列表
	ReadAll bool     `json:"readAll,omitempty"` // 标记所有为已读
}

// ========== 离线同步协议 ==========

// SyncConflictType 冲突类型.
type SyncConflictType string

const (
	ConflictModify  SyncConflictType = "modify"  // 双方修改
	ConflictDelete  SyncConflictType = "delete"  // 一方删除
	ConflictRename  SyncConflictType = "rename"  // 重命名冲突
)

// ConflictResolution 冲突解决策略.
type ConflictResolution string

const (
	ResolutionServer   ConflictResolution = "server"   // 以服务端为准
	ResolutionClient   ConflictResolution = "client"   // 以客户端为准
	ResolutionBoth     ConflictResolution = "both"     // 保留两者
	ResolutionManual   ConflictResolution = "manual"   // 手动解决
)

// ConflictRecord 同步冲突记录.
type ConflictRecord struct {
	ID           string             `json:"id"`           // 冲突ID
	ItemID       string             `json:"itemId"`       // 同步项ID
	ConflictType SyncConflictType   `json:"conflictType"` // 冲突类型
	ServerVersion string            `json:"serverVersion"` // 服务端版本
	ClientVersion string            `json:"clientVersion"` // 客户端版本
	ServerMtime  time.Time          `json:"serverMtime"`  // 服务端修改时间
	ClientMtime  time.Time          `json:"clientMtime"`  // 客户端修改时间
	Resolution   *ConflictResolution `json:"resolution,omitempty"` // 解决策略
	Resolved     bool               `json:"resolved"`     // 是否已解决
	CreatedAt    time.Time          `json:"createdAt"`    // 创建时间
	ResolvedAt   *time.Time         `json:"resolvedAt,omitempty"` // 解决时间
}

// SyncDelta 增量同步数据.
type SyncDelta struct {
	LastSyncTime time.Time   `json:"lastSyncTime"` // 上次同步时间
	Changes      []SyncChange `json:"changes"`     // 变更列表
	HasMore      bool        `json:"hasMore"`      // 是否还有更多
	NextToken    string      `json:"nextToken"`    // 下次分页令牌
}

// SyncChange 同步变更记录.
type SyncChange struct {
	Path      string    `json:"path"`      // 文件路径
	Action    string    `json:"action"`    // 操作类型 (create/update/delete)
	Checksum  string    `json:"checksum"`  // 文件校验和
	Size      int64     `json:"size"`      // 文件大小
	Mtime     time.Time `json:"mtime"`     // 修改时间
}

// ========== 移动端适配 ==========

// ImageFormat 图片格式.
type ImageFormat string

const (
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatWebP ImageFormat = "webp"
)

// ThumbnailSize 缩略图尺寸.
type ThumbnailSize string

const (
	ThumbSmall  ThumbnailSize = "small"  // 150x150
	ThumbMedium ThumbnailSize = "medium" // 300x300
	ThumbLarge  ThumbnailSize = "large"  // 600x600
)

// ImageProcessRequest 图片处理请求.
type ImageProcessRequest struct {
	Path       string      `json:"path"`                 // 图片路径
	Format     ImageFormat `json:"format,omitempty"`     // 输出格式
	MaxWidth   int         `json:"maxWidth,omitempty"`   // 最大宽度
	MaxHeight  int         `json:"maxHeight,omitempty"`  // 最大高度
	Quality    int         `json:"quality,omitempty"`    // 压缩质量 (1-100)
	Thumbnail  bool        `json:"thumbnail,omitempty"`  // 生成缩略图
	ThumbSize  ThumbnailSize `json:"thumbSize,omitempty"` // 缩略图尺寸
}

// ImageProcessResult 图片处理结果.
type ImageProcessResult struct {
	OriginalPath  string `json:"originalPath"`  // 原始路径
	ProcessedPath string `json:"processedPath"` // 处理后路径
	ThumbPath     string `json:"thumbPath,omitempty"` // 缩略图路径
	OriginalSize  int64  `json:"originalSize"`  // 原始大小
	ProcessedSize int64  `json:"processedSize"` // 处理后大小
	Width         int    `json:"width"`         // 宽度
	Height        int    `json:"height"`        // 高度
	Format        string `json:"format"`        // 格式
}

// MobileResponse 移动端响应格式（精简版）.
type MobileResponse struct {
	Code    int         `json:"c"`           // 状态码
	Message string      `json:"msg,omitempty"` // 消息
	Data    interface{} `json:"d,omitempty"`   // 数据
}

// ========== 会话管理 ==========

// DeviceBinding 设备绑定记录.
type DeviceBinding struct {
	ID         string    `json:"id"`         // 绑定ID
	UserID     string    `json:"userId"`     // 用户ID
	DeviceID   string    `json:"deviceId"`   // 设备ID
	BoundAt    time.Time `json:"boundAt"`    // 绑定时间
	UnboundAt  *time.Time `json:"unboundAt,omitempty"` // 解绑时间
	Active     bool      `json:"active"`     // 是否活跃
}

// TokenRefreshRequest Token刷新请求.
type TokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
	DeviceID     string `json:"deviceId" binding:"required"`
}
