// Package chat 提供即时通讯功能，对标群晖 Synology Chat
package chat

import (
	"time"
)

// ChannelType 频道类型.
type ChannelType string

const (
	ChannelTypeDirect  ChannelType = "direct"  // 私聊
	ChannelTypeGroup   ChannelType = "group"   // 群组
	ChannelTypeChannel ChannelType = "channel" // 频道
)

// MemberRole 成员角色.
type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"  // 所有者
	MemberRoleAdmin  MemberRole = "admin"  // 管理员
	MemberRoleMember MemberRole = "member" // 普通成员
)

// MessageType 消息类型.
type MessageType string

const (
	MessageTypeText   MessageType = "text"   // 文本消息
	MessageTypeFile   MessageType = "file"   // 文件消息
	MessageTypeImage  MessageType = "image"  // 图片消息
	MessageTypeSystem MessageType = "system" // 系统消息
)

// Channel 频道/群组.
type Channel struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Type        ChannelType `json:"type"`
	CreatorID   string      `json:"creator_id"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Message 消息.
type Message struct {
	ID        string      `json:"id"`
	ChannelID string      `json:"channel_id"`
	SenderID  string      `json:"sender_id"`
	Content   string      `json:"content"`
	Type      MessageType `json:"type"`
	ReplyTo   string      `json:"reply_to,omitempty"`
	Reactions []Reaction  `json:"reactions,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	EditedAt  *time.Time  `json:"edited_at,omitempty"`
	DeletedAt *time.Time  `json:"deleted_at,omitempty"`
}

// ChannelMember 频道成员.
type ChannelMember struct {
	ChannelID string     `json:"channel_id"`
	UserID    string     `json:"user_id"`
	Role      MemberRole `json:"role"`
	JoinedAt  time.Time  `json:"joined_at"`
	LastRead  time.Time  `json:"last_read"`
}

// Reaction 消息表情反应.
type Reaction struct {
	Emoji     string    `json:"emoji"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ========== 请求结构 ==========

// CreateChannelRequest 创建频道请求.
type CreateChannelRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description,omitempty"`
	Type        ChannelType `json:"type" binding:"required,oneof=direct group channel"`
	CreatorID   string      `json:"creator_id" binding:"required"`
}

// UpdateChannelRequest 更新频道请求.
type UpdateChannelRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SendMessageRequest 发送消息请求.
type SendMessageRequest struct {
	SenderID string      `json:"sender_id" binding:"required"`
	Content  string      `json:"content" binding:"required"`
	Type     MessageType `json:"type"`
	ReplyTo  string      `json:"reply_to,omitempty"`
}

// UpdateMessageRequest 编辑消息请求.
type UpdateMessageRequest struct {
	Content *string `json:"content" binding:"required"`
}

// AddMemberRequest 添加成员请求.
type AddMemberRequest struct {
	UserID string     `json:"user_id" binding:"required"`
	Role   MemberRole `json:"role"`
}

// UpdateMemberRoleRequest 更新成员角色请求.
type UpdateMemberRoleRequest struct {
	Role MemberRole `json:"role" binding:"required,oneof=owner admin member"`
}

// AddReactionRequest 添加反应请求.
type AddReactionRequest struct {
	Emoji  string `json:"emoji" binding:"required"`
	UserID string `json:"user_id" binding:"required"`
}

// RemoveReactionRequest 移除反应请求.
type RemoveReactionRequest struct {
	Emoji  string `json:"emoji" binding:"required"`
	UserID string `json:"user_id" binding:"required"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query   string `form:"q" binding:"required"`
	Channel string `form:"channel,omitempty"`
}

// UnreadCount 未读计数.
type UnreadCount struct {
	ChannelID string `json:"channel_id"`
	Count     int    `json:"count"`
}
