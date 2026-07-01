// Package webshare 实现 Web 文件分享模块
// 对标 TrueNAS 26 的 WebShare 功能
// 支持浏览器文件访问、上传下载、文件夹创建、可分享链接及 FIPS 加密传输
package webshare

import (
	"time"

	"github.com/google/uuid"
)

// ========== 核心类型定义 ==========

// ShareStatus 分享状态
type ShareStatus string

const (
	ShareStatusActive    ShareStatus = "active"     // 活跃
	ShareStatusExpired   ShareStatus = "expired"    // 已过期
	ShareStatusRevoked   ShareStatus = "revoked"    // 已撤销
	ShareStatusDisabled  ShareStatus = "disabled"   // 已禁用
)

// AccessMode 访问模式
type AccessMode string

const (
	AccessModeReadOnly  AccessMode = "read_only"  // 只读
	AccessModeReadWrite AccessMode = "read_write" // 读写
	AccessModeWriteOnly AccessMode = "write_only" // 只写（上传）
	AccessModeFull      AccessMode = "full"       // 完全控制
)

// SharePermission 分享权限
type SharePermission struct {
	// 是否允许浏览
	CanBrowse bool `json:"can_browse"`
	// 是否允许下载
	CanDownload bool `json:"can_download"`
	// 是否允许上传
	CanUpload bool `json:"can_upload"`
	// 是否允许创建文件夹
	CanMkdir bool `json:"can_mkdir"`
	// 是否允许删除
	CanDelete bool `json:"can_delete"`
	// 是否允许重命名
	CanRename bool `json:"can_rename"`
	// 是否允许分享子链接
	CanShare bool `json:"can_share"`
	// 单文件最大上传大小（字节），0 表示无限制
	MaxUploadSize int64 `json:"max_upload_size,omitempty"`
}

// DefaultReadOnlyPermission 默认只读权限
func DefaultReadOnlyPermission() *SharePermission {
	return &SharePermission{
		CanBrowse:   true,
		CanDownload: true,
		CanUpload:    false,
		CanMkdir:     false,
		CanDelete:    false,
		CanRename:    false,
		CanShare:     false,
	}
}

// DefaultReadWritePermission 默认读写权限
func DefaultReadWritePermission() *SharePermission {
	return &SharePermission{
		CanBrowse:   true,
		CanDownload: true,
		CanUpload:   true,
		CanMkdir:    true,
		CanDelete:   false,
		CanRename:    false,
		CanShare:     false,
	}
}

// DefaultFullPermission 默认完全控制权限
func DefaultFullPermission() *SharePermission {
	return &SharePermission{
		CanBrowse:   true,
		CanDownload: true,
		CanUpload:   true,
		CanMkdir:    true,
		CanDelete:   true,
		CanRename:   true,
		CanShare:    true,
	}
}

// WebShare Web 文件分享
type WebShare struct {
	// 分享唯一标识
	ID string `json:"id"`
	// 分享名称
	Name string `json:"name"`
	// 分享根路径
	RootPath string `json:"root_path"`
	// 访问令牌
	Token string `json:"token"`
	// 访问模式
	AccessMode AccessMode `json:"access_mode"`
	// 权限配置
	Permission *SharePermission `json:"permission"`
	// 是否启用 FIPS 加密传输
	FIPSEnabled bool `json:"fips_enabled"`
	// 创建者
	CreatedBy string `json:"created_by"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// 过期时间（nil 表示永不过期）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// 状态
	Status ShareStatus `json:"status"`
	// 描述
	Description string `json:"description,omitempty"`
	// 最大并发访问数，0 表示无限制
	MaxConcurrentAccess int `json:"max_concurrent_access,omitempty"`
	// 当前活跃会话数
	ActiveSessionCount int `json:"active_session_count"`
	// 是否需要密码
	PasswordEnabled bool `json:"password_enabled"`
	// 密码哈希（内部存储）
	PasswordHash string `json:"-"`
}

// BrowserSession 浏览器访问会话
type BrowserSession struct {
	// 会话唯一标识
	ID string `json:"id"`
	// 关联的 WebShare ID
	ShareID string `json:"share_id"`
	// 客户端 IP 地址
	ClientIP string `json:"client_ip"`
	// User-Agent
	UserAgent string `json:"user_agent,omitempty"`
	// 会话创建时间
	CreatedAt time.Time `json:"created_at"`
	// 最后活动时间
	LastActiveAt time.Time `json:"last_active_at"`
	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// 当前浏览的路径
	CurrentPath string `json:"current_path"`
	// 会话是否活跃
	IsActive bool `json:"is_active"`
}

// FileEntry 目录条目
type FileEntry struct {
	// 文件/文件夹名称
	Name string `json:"name"`
	// 相对路径（相对于分享根路径）
	RelativePath string `json:"relative_path"`
	// 是否为文件夹
	IsDir bool `json:"is_dir"`
	// 文件大小（字节）
	Size int64 `json:"size,omitempty"`
	// 修改时间
	ModTime time.Time `json:"mod_time"`
	// MIME 类型
	MimeType string `json:"mime_type,omitempty"`
}

// ========== 配置 ==========

// WebShareConfig WebShare 配置
type WebShareConfig struct {
	// 是否启用 WebShare 功能
	Enabled bool `json:"enabled"`
	// 默认访问模式
	DefaultAccessMode AccessMode `json:"default_access_mode"`
	// 默认最大并发访问数
	DefaultMaxConcurrentAccess int `json:"default_max_concurrent_access"`
	// 默认会话超时（分钟）
	DefaultSessionTimeoutMinutes int `json:"default_session_timeout_minutes"`
	// 默认分享过期时间（小时），0 表示永不过期
	DefaultExpiryHours int `json:"default_expiry_hours"`
	// 是否默认启用 FIPS 加密
	DefaultFIPSEnabled bool `json:"default_fips_enabled"`
	// 单文件最大上传大小（字节），0 表示无限制
	MaxUploadSize int64 `json:"max_upload_size"`
	// 令牌长度
	TokenLength int `json:"token_length"`
}

// DefaultConfig 默认配置
func DefaultConfig() *WebShareConfig {
	return &WebShareConfig{
		Enabled:                      true,
		DefaultAccessMode:            AccessModeReadOnly,
		DefaultMaxConcurrentAccess:   10,
		DefaultSessionTimeoutMinutes: 30,
		DefaultExpiryHours:           0,
		DefaultFIPSEnabled:           true,
		MaxUploadSize:                0,
		TokenLength:                  32,
	}
}

// ========== 请求/响应结构 ==========

// CreateShareRequest 创建分享请求
type CreateShareRequest struct {
	Name        string `json:"name" binding:"required"`
	RootPath    string `json:"root_path" binding:"required"`
	AccessMode  AccessMode `json:"access_mode"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by" binding:"required"`
	// 过期小时数，0 表示永不过期
	ExpiryHours int `json:"expiry_hours,omitempty"`
	// 是否启用 FIPS 加密（nil 表示使用默认值）
	FIPSEnabled *bool `json:"fips_enabled,omitempty"`
	// 是否启用密码保护
	Password string `json:"password,omitempty"`
	// 最大并发访问数
	MaxConcurrentAccess int `json:"max_concurrent_access,omitempty"`
	// 自定义权限
	Permission *SharePermission `json:"permission,omitempty"`
}

// CreateFolderRequest 创建文件夹请求
type CreateFolderRequest struct {
	// 会话令牌
	SessionToken string `json:"session_token" binding:"required"`
	// 目标路径（相对于分享根路径）
	Path string `json:"path" binding:"required"`
}

// UploadFileRequest 上传文件请求
type UploadFileRequest struct {
	// 会话令牌
	SessionToken string `json:"session_token" binding:"required"`
	// 目标路径（相对于分享根路径）
	Path string `json:"path" binding:"required"`
	// 是否覆盖已存在文件
	Overwrite bool `json:"overwrite,omitempty"`
}

// ListFilesRequest 浏览文件请求
type ListFilesRequest struct {
	// 会话令牌
	SessionToken string `json:"session_token" binding:"required"`
	// 浏览路径（相对于分享根路径，空表示根目录）
	Path string `json:"path,omitempty"`
}

// ShareLinkResponse 分享链接响应
type ShareLinkResponse struct {
	// 分享 URL
	URL string `json:"url"`
	// 分享令牌
	Token string `json:"token"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	// 当前路径
	Path string `json:"path"`
	// 文件条目列表
	Entries []FileEntry `json:"entries"`
	// 条目总数
	Total int `json:"total"`
}

// SessionResponse 会话响应
type SessionResponse struct {
	SessionToken string `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ShareStats 分享统计
type ShareStats struct {
	TotalShares        int `json:"total_shares"`
	ActiveShares       int `json:"active_shares"`
	ExpiredShares      int `json:"expired_shares"`
	RevokedShares      int `json:"revoked_shares"`
	TotalSessions      int `json:"total_sessions"`
	ActiveSessions     int `json:"active_sessions"`
	FIPSEnabledShares  int `json:"fips_enabled_shares"`
	PasswordProtected  int `json:"password_protected"`
}

// ========== 内部辅助函数 ==========

// newWebShare 创建 WebShare 实例（内部辅助函数）
func newWebShare(req *CreateShareRequest, cfg *WebShareConfig) *WebShare {
	now := time.Now()

	share := &WebShare{
		ID:                  "ws_" + uuid.New().String()[:12],
		Name:                req.Name,
		RootPath:            req.RootPath,
		Token:               generateToken(cfg.TokenLength),
		AccessMode:          req.AccessMode,
		CreatedBy:           req.CreatedBy,
		CreatedAt:           now,
		UpdatedAt:           now,
		Status:              ShareStatusActive,
		Description:         req.Description,
		MaxConcurrentAccess: req.MaxConcurrentAccess,
		FIPSEnabled:         req.FIPSEnabled != nil && *req.FIPSEnabled,
		PasswordEnabled:     req.Password != "",
	}

	// 设置权限
	if req.Permission != nil {
		share.Permission = req.Permission
	} else {
		share.Permission = permissionForMode(req.AccessMode)
	}

	// 设置过期时间
	if req.ExpiryHours > 0 {
		expires := now.Add(time.Duration(req.ExpiryHours) * time.Hour)
		share.ExpiresAt = &expires
	} else if cfg.DefaultExpiryHours > 0 {
		expires := now.Add(time.Duration(cfg.DefaultExpiryHours) * time.Hour)
		share.ExpiresAt = &expires
	}

	// 设置密码哈希
	if req.Password != "" {
		share.PasswordHash = hashPassword(req.Password)
	}

	// 默认 FIPS：仅在请求未显式指定时使用默认值
	if req.FIPSEnabled == nil && cfg.DefaultFIPSEnabled {
		share.FIPSEnabled = true
	}

	// 默认最大并发
	if share.MaxConcurrentAccess == 0 {
		share.MaxConcurrentAccess = cfg.DefaultMaxConcurrentAccess
	}

	return share
}

// permissionForMode 根据访问模式返回默认权限
func permissionForMode(mode AccessMode) *SharePermission {
	switch mode {
	case AccessModeReadOnly:
		return DefaultReadOnlyPermission()
	case AccessModeReadWrite:
		return DefaultReadWritePermission()
	case AccessModeFull:
		return DefaultFullPermission()
	case AccessModeWriteOnly:
		return &SharePermission{
			CanBrowse:   false,
			CanDownload: false,
			CanUpload:   true,
			CanMkdir:    true,
			CanDelete:   false,
			CanRename:   false,
			CanShare:    false,
		}
	default:
		return DefaultReadOnlyPermission()
	}
}

// generateToken 生成访问令牌
func generateToken(length int) string {
	if length <= 0 {
		length = 32
	}
	return uuid.New().String() + uuid.New().String()
}

// hashPassword 简单密码哈希（实际生产应使用 bcrypt/argon2）
func hashPassword(password string) string {
	// 简单实现：实际应使用安全哈希算法
	return "sha256:" + password
}

// verifyPassword 验证密码
func verifyPassword(hash, password string) bool {
	return hash == "sha256:"+password
}

// newSession 创建浏览器会话
func newSession(shareID, clientIP, userAgent, currentPath string, timeoutMinutes int) *BrowserSession {
	now := time.Now()
	if timeoutMinutes <= 0 {
		timeoutMinutes = 30
	}
	return &BrowserSession{
		ID:           "sess_" + uuid.New().String()[:12],
		ShareID:      shareID,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:     now.Add(time.Duration(timeoutMinutes) * time.Minute),
		CurrentPath:  currentPath,
		IsActive:     true,
	}
}