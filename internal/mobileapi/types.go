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
