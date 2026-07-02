// Package team 提供团队协作功能
// 包含团队文件夹管理、外链分享、协同编辑、评论系统和审计日志
package team

import (
	"time"
)

// ========== 团队管理 ==========

// Team 团队信息.
type Team struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	OwnerID     string       `json:"owner_id"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Settings    TeamSettings `json:"settings,omitempty"`
}

// TeamSettings 团队设置.
type TeamSettings struct {
	MaxMembers       int   `json:"max_members,omitempty"`        // 最大成员数
	AllowPublicShare bool  `json:"allow_public_share,omitempty"` // 允许公开分享
	RequireApproval  bool  `json:"require_approval,omitempty"`   // 加入需要审批
	DefaultQuota     int64 `json:"default_quota,omitempty"`      // 默认配额(字节)
}

// TeamMember 团队成员.
type TeamMember struct {
	TeamID    string     `json:"team_id"`
	UserID    string     `json:"user_id"`
	Username  string     `json:"username"`
	Role      MemberRole `json:"role"`
	JoinedAt  time.Time  `json:"joined_at"`
	InvitedBy string     `json:"invited_by,omitempty"`
}

// MemberRole 成员角色.
type MemberRole string

const (
	RoleOwner  MemberRole = "owner"  // 所有者
	RoleAdmin  MemberRole = "admin"  // 管理员
	RoleEditor MemberRole = "editor" // 编辑者
	RoleViewer MemberRole = "viewer" // 查看者
	RoleGuest  MemberRole = "guest"  // 访客
)

// CanManage 检查角色是否有管理权限.
func (r MemberRole) CanManage() bool {
	return r == RoleOwner || r == RoleAdmin
}

// CanEdit 检查角色是否有编辑权限.
func (r MemberRole) CanEdit() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleEditor
}

// TeamInput 创建/更新团队输入.
type TeamInput struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	OwnerID     string       `json:"owner_id,omitempty"`
	Settings    TeamSettings `json:"settings,omitempty"`
}

// MemberInput 添加成员输入.
type MemberInput struct {
	UserID string     `json:"user_id" binding:"required"`
	Role   MemberRole `json:"role"`
}

// ========== 团队文件夹 ==========

// TeamFolder 团队文件夹.
type TeamFolder struct {
	ID          string            `json:"id"`
	TeamID      string            `json:"team_id"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	ParentID    string            `json:"parent_id,omitempty"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Permissions FolderPermissions `json:"permissions,omitempty"`
	Quota       int64             `json:"quota,omitempty"` // 配额(字节)
	UsedSize    int64             `json:"used_size"`       // 已使用(字节)
}

// FolderPermissions 文件夹权限.
type FolderPermissions struct {
	DefaultRole MemberRole            `json:"default_role"`        // 默认角色
	Overrides   map[string]MemberRole `json:"overrides,omitempty"` // 用户特定权限
}

// FolderInput 创建/更新文件夹输入.
type FolderInput struct {
	Name        string            `json:"name" binding:"required"`
	Path        string            `json:"path,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Permissions FolderPermissions `json:"permissions,omitempty"`
	Quota       int64             `json:"quota,omitempty"`
}

// ========== 外链分享 ==========

// ShareLink 分享链接.
type ShareLink struct {
	ID           string          `json:"id"`
	Token        string          `json:"token"`         // 分享Token
	ResourceType ResourceType    `json:"resource_type"` // 资源类型
	ResourceID   string          `json:"resource_id"`   // 资源ID
	ResourcePath string          `json:"resource_path"` // 资源路径
	CreatedBy    string          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	Password     string          `json:"-"` // 密码(不序列化)
	HasPassword  bool            `json:"has_password"`
	MaxAccess    int             `json:"max_access,omitempty"` // 最大访问次数
	AccessCount  int             `json:"access_count"`         // 已访问次数
	Permission   SharePermission `json:"permission"`
	IsActive     bool            `json:"is_active"`
}

// ResourceType 资源类型.
type ResourceType string

const (
	ResourceFile   ResourceType = "file"
	ResourceFolder ResourceType = "folder"
	ResourceTeam   ResourceType = "team"
)

// SharePermission 分享权限.
type SharePermission string

const (
	ShareView     SharePermission = "view"     // 仅查看
	ShareDownload SharePermission = "download" // 可下载
	ShareEdit     SharePermission = "edit"     // 可编辑
	ShareAdmin    SharePermission = "admin"    // 完全控制
)

// ShareInput 创建分享输入.
type ShareInput struct {
	ResourceType ResourceType    `json:"resource_type" binding:"required"`
	ResourceID   string          `json:"resource_id" binding:"required"`
	ResourcePath string          `json:"resource_path"`
	Password     string          `json:"password,omitempty"`
	ExpiresIn    time.Duration   `json:"expires_in,omitempty"` // 过期时长(小时)
	MaxAccess    int             `json:"max_access,omitempty"`
	Permission   SharePermission `json:"permission"`
}

// ShareAccess 分享访问记录.
type ShareAccess struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	UserID    string    `json:"user_id,omitempty"` // 可能为空(匿名访问)
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent,omitempty"`
	AccessAt  time.Time `json:"access_at"`
	Action    string    `json:"action"` // view, download, edit
}

// ========== 协同编辑 ==========

// EditSession 编辑会话.
type EditSession struct {
	ID           string       `json:"id"`
	ResourceType ResourceType `json:"resource_type"`
	ResourceID   string       `json:"resource_id"`
	ResourcePath string       `json:"resource_path"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	IsActive     bool         `json:"is_active"`
}

// EditOperation 编辑操作.
type EditOperation struct {
	ID         string        `json:"id"`
	SessionID  string        `json:"session_id"`
	UserID     string        `json:"user_id"`
	Username   string        `json:"username"`
	Operation  OperationType `json:"operation"`
	Position   int64         `json:"position,omitempty"`    // 文件位置
	Length     int64         `json:"length,omitempty"`      // 长度
	Content    string        `json:"content,omitempty"`     // 内容
	OldContent string        `json:"old_content,omitempty"` // 旧内容(用于撤销)
	Timestamp  time.Time     `json:"timestamp"`
	Version    int64         `json:"version"` // 文档版本号
}

// OperationType 操作类型.
type OperationType string

const (
	OpInsert  OperationType = "insert"  // 插入
	OpDelete  OperationType = "delete"  // 删除
	OpReplace OperationType = "replace" // 替换
	OpMove    OperationType = "move"    // 移动
	OpRename  OperationType = "rename"  // 重命名
	OpCreate  OperationType = "create"  // 创建
)

// CursorPosition 光标位置.
type CursorPosition struct {
	SessionID  string     `json:"session_id"`
	UserID     string     `json:"user_id"`
	Username   string     `json:"username"`
	ResourceID string     `json:"resource_id"`
	Position   int64      `json:"position"`
	Selection  *Selection `json:"selection,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Selection 选区.
type Selection struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// ========== 评论系统 ==========

// Comment 评论.
type Comment struct {
	ID           string         `json:"id"`
	ResourceType ResourceType   `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	ResourcePath string         `json:"resource_path,omitempty"`
	ParentID     string         `json:"parent_id,omitempty"` // 回复的评论ID
	UserID       string         `json:"user_id"`
	Username     string         `json:"username"`
	Content      string         `json:"content"`
	Mentions     []Mention      `json:"mentions,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	IsEdited     bool           `json:"is_edited"`
	IsDeleted    bool           `json:"is_deleted"`
	Reactions    map[string]int `json:"reactions,omitempty"` // emoji -> count
}

// Mention @提及.
type Mention struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Position int    `json:"position"` // 在评论中的位置
}

// CommentInput 评论输入.
type CommentInput struct {
	ResourceType ResourceType `json:"resource_type" binding:"required"`
	ResourceID   string       `json:"resource_id" binding:"required"`
	ResourcePath string       `json:"resource_path,omitempty"`
	ParentID     string       `json:"parent_id,omitempty"`
	Content      string       `json:"content" binding:"required,min=1,max=5000"`
}

// ========== 审计日志 ==========

// TeamAuditLog 团队审计日志.
type TeamAuditLog struct {
	ID           string                 `json:"id"`
	TeamID       string                 `json:"team_id,omitempty"`
	UserID       string                 `json:"user_id"`
	Username     string                 `json:"username"`
	Action       TeamAuditAction        `json:"action"`
	ResourceType string                 `json:"resource_type,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	ResourcePath string                 `json:"resource_path,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	IP           string                 `json:"ip,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// TeamAuditAction 团队审计操作.
type TeamAuditAction string

const (
	// 团队操作.
	AuditTeamCreate       TeamAuditAction = "team_create"
	AuditTeamUpdate       TeamAuditAction = "team_update"
	AuditTeamDelete       TeamAuditAction = "team_delete"
	AuditTeamMemberAdd    TeamAuditAction = "team_member_add"
	AuditTeamMemberRemove TeamAuditAction = "team_member_remove"
	AuditTeamMemberRole   TeamAuditAction = "team_member_role"

	// 文件夹操作.
	AuditFolderCreate TeamAuditAction = "folder_create"
	AuditFolderUpdate TeamAuditAction = "folder_update"
	AuditFolderDelete TeamAuditAction = "folder_delete"
	AuditFolderMove   TeamAuditAction = "folder_move"

	// 文件操作.
	AuditFileUpload   TeamAuditAction = "file_upload"
	AuditFileDownload TeamAuditAction = "file_download"
	AuditFileDelete   TeamAuditAction = "file_delete"
	AuditFileMove     TeamAuditAction = "file_move"
	AuditFileCopy     TeamAuditAction = "file_copy"
	AuditFileRename   TeamAuditAction = "file_rename"

	// 分享操作.
	AuditShareCreate TeamAuditAction = "share_create"
	AuditShareAccess TeamAuditAction = "share_access"
	AuditShareRevoke TeamAuditAction = "share_revoke"

	// 协同编辑.
	AuditEditStart    TeamAuditAction = "edit_start"
	AuditEditEnd      TeamAuditAction = "edit_end"
	AuditEditSave     TeamAuditAction = "edit_save"
	AuditEditConflict TeamAuditAction = "edit_conflict"

	// 评论操作.
	AuditCommentCreate TeamAuditAction = "comment_create"
	AuditCommentUpdate TeamAuditAction = "comment_update"
	AuditCommentDelete TeamAuditAction = "comment_delete"
)

// AuditQueryOptions 审计查询选项.
type AuditQueryOptions struct {
	TeamID       string          `json:"team_id,omitempty"`
	UserID       string          `json:"user_id,omitempty"`
	Action       TeamAuditAction `json:"action,omitempty"`
	ResourceType string          `json:"resource_type,omitempty"`
	ResourceID   string          `json:"resource_id,omitempty"`
	StartTime    *time.Time      `json:"start_time,omitempty"`
	EndTime      *time.Time      `json:"end_time,omitempty"`
	Limit        int             `json:"limit,omitempty"`
	Offset       int             `json:"offset,omitempty"`
}

// ========== 通知消息 ==========

// NotificationType 通知类型.
type NotificationType string

const (
	NotifyTeamInvite    NotificationType = "team_invite"
	NotifyTeamMemberAdd NotificationType = "team_member_add"
	NotifyShareCreated  NotificationType = "share_created"
	NotifyShareAccessed NotificationType = "share_accessed"
	NotifyCommentAdded  NotificationType = "comment_added"
	NotifyMention       NotificationType = "mention"
	NotifyEditConflict  NotificationType = "edit_conflict"
	NotifyFileChanged   NotificationType = "file_changed"
)

// Notification 通知.
type Notification struct {
	ID        string                 `json:"id"`
	Type      NotificationType       `json:"type"`
	UserID    string                 `json:"user_id"` // 接收者
	FromUser  string                 `json:"from_user,omitempty"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
}

// ========== WebSocket消息 ==========

// WSMessage WebSocket消息.
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// WSEventType WebSocket事件类型.
type WSEventType string

const (
	WSEventJoin         WSEventType = "join"
	WSEventLeave        WSEventType = "leave"
	WSEventEdit         WSEventType = "edit"
	WSEventCursor       WSEventType = "cursor"
	WSEventComment      WSEventType = "comment"
	WSEventNotification WSEventType = "notification"
	WSEventSync         WSEventType = "sync"
	WSEventConflict     WSEventType = "conflict"
)

// ========== 错误定义 ==========

// TeamError 团队错误.
type TeamError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *TeamError) Error() string {
	return e.Message
}

// 预定义错误.
var (
	ErrTeamNotFound    = &TeamError{Code: 404, Message: "团队不存在"}
	ErrTeamExists      = &TeamError{Code: 409, Message: "团队已存在"}
	ErrMemberNotFound  = &TeamError{Code: 404, Message: "成员不存在"}
	ErrMemberExists    = &TeamError{Code: 409, Message: "成员已存在"}
	ErrFolderNotFound  = &TeamError{Code: 404, Message: "文件夹不存在"}
	ErrFolderExists    = &TeamError{Code: 409, Message: "文件夹已存在"}
	ErrShareNotFound   = &TeamError{Code: 404, Message: "分享不存在"}
	ErrShareExpired    = &TeamError{Code: 410, Message: "分享已过期"}
	ErrSharePassword   = &TeamError{Code: 401, Message: "分享密码错误"}
	ErrShareLimit      = &TeamError{Code: 403, Message: "分享访问次数已达上限"}
	ErrNoPermission    = &TeamError{Code: 403, Message: "无权限执行此操作"}
	ErrCommentNotFound = &TeamError{Code: 404, Message: "评论不存在"}
	ErrEditConflict    = &TeamError{Code: 409, Message: "编辑冲突，请刷新后重试"}
	ErrSessionNotFound = &TeamError{Code: 404, Message: "编辑会话不存在"}
)
