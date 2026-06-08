// Package teamfile 团队文件夹管理
// 支持团队级别的文件共享和协作，不归属个人用户。
// 对标飞牛fnOS团队文件夹，支持成员权限精细化管理。
package teamfile

import (
	"errors"
	"sync"
	"time"
)

// FolderPermission 文件夹权限
type FolderPermission string

const (
	PermOwner FolderPermission = "owner"
	PermAdmin FolderPermission = "admin"
	PermWrite FolderPermission = "write"
	PermRead  FolderPermission = "read"
	PermDeny  FolderPermission = "deny"
)

// MemberRole 成员角色
type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
	RoleGuest  MemberRole = "guest"
)

// TeamFolder 团队文件夹
type TeamFolder struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	OwnerTeam   string    `json:"owner_team"`
	SizeBytes   int64     `json:"size_bytes"`
	FileCount   int64     `json:"file_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsActive    bool      `json:"is_active"`
}

// FolderMember 文件夹成员
type FolderMember struct {
	UserID     string           `json:"user_id"`
	Role       MemberRole       `json:"role"`
	Permission FolderPermission `json:"permission"`
	JoinedAt   time.Time        `json:"joined_at"`
	AddedBy    string           `json:"added_by"`
}

// ShareLink 分享链接
type ShareLink struct {
	ID            string           `json:"id"`
	FolderID      string           `json:"folder_id"`
	Token         string           `json:"token"`
	Permission    FolderPermission `json:"permission"`
	ExpiresAt     *time.Time       `json:"expires_at,omitempty"`
	MaxDownloads  int              `json:"maxDownloads"`
	DownloadCount int              `json:"download_count"`
	CreatedBy     string           `json:"created_by"`
	CreatedAt     time.Time        `json:"created_at"`
	IsActive      bool             `json:"is_active"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	FolderID  string    `json:"folder_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// TeamFileManager 团队文件管理器
type TeamFileManager struct {
	mu       sync.RWMutex
	folders  map[string]*TeamFolder
	members  map[string][]*FolderMember // folderID -> members
	links    map[string]*ShareLink
	auditLog []*AuditLog
	config   ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxFolders          int  `json:"max_folders"`
	MaxMembersPerFolder int  `json:"max_members_per_folder"`
	AllowGuestAccess    bool `json:"allow_guest_access"`
	LinkExpiryDays      int  `json:"link_expiry_days"`
	AuditEnabled        bool `json:"audit_enabled"`
	MaxFileSizeMB       int  `json:"max_file_size_mb"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxFolders:          100,
		MaxMembersPerFolder: 200,
		AllowGuestAccess:    true,
		LinkExpiryDays:      30,
		AuditEnabled:        true,
		MaxFileSizeMB:       10240,
	}
}

// 预定义错误
var (
	ErrFolderNotFound    = errors.New("team folder not found")
	ErrFolderExists      = errors.New("team folder already exists")
	ErrMemberNotFound    = errors.New("member not found")
	ErrMemberExists      = errors.New("member already exists")
	ErrMaxFoldersReached = errors.New("max folders reached")
	ErrMaxMembersReached = errors.New("max members per folder reached")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrLinkNotFound      = errors.New("share link not found")
	ErrLinkExpired       = errors.New("share link expired")
	ErrGuestNotAllowed   = errors.New("guest access not allowed")
)
