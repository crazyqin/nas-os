package chatplus

import (
	"sync"
	"time"
)

// MessageType 消息类型.
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeFile     MessageType = "file"
	MessageTypeCode     MessageType = "code"
	MessageTypeMarkdown MessageType = "markdown"
	MessageTypeSystem   MessageType = "system"
)

// ChannelType 频道类型.
type ChannelType string

const (
	ChannelTypeDirect  ChannelType = "direct"  // 私聊
	ChannelTypeGroup   ChannelType = "group"   // 群组
	ChannelTypeChannel ChannelType = "channel" // 频道
)

// UserStatus 用户状态.
type UserStatus string

const (
	UserStatusOnline  UserStatus = "online"
	UserStatusOffline UserStatus = "offline"
	UserStatusBusy    UserStatus = "busy"
	UserStatusAway    UserStatus = "away"
)

// ChatUser 聊天用户.
type ChatUser struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	FullName  string     `json:"fullName"`
	Avatar    string     `json:"avatar"`
	Email     string     `json:"email"`
	Status    UserStatus `json:"status"`
	LastSeen  time.Time  `json:"lastSeen"`
	CreatedAt time.Time  `json:"createdAt"`
}

// ChatMessage 聊天消息.
type ChatMessage struct {
	ID          string       `json:"id"`
	ChannelID   string       `json:"channelId"`
	SenderID    string       `json:"senderId"`
	SenderName  string       `json:"senderName"`
	Type        MessageType  `json:"type"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ReplyTo     string       `json:"replyTo,omitempty"` // 回复的消息ID
	Edited      bool         `json:"edited"`
	EditedAt    *time.Time   `json:"editedAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Attachment 附件.
type Attachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
}

// ChatChannel 聊天频道.
type ChatChannel struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        ChannelType  `json:"type"`
	IsPrivate   bool         `json:"isPrivate"`
	Members     []string     `json:"members"` // 成员ID列表
	Admins      []string     `json:"admins"`  // 管理员ID列表
	CreatedBy   string       `json:"createdBy"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	LastMessage *ChatMessage `json:"lastMessage,omitempty"`
	UnreadCount int          `json:"unreadCount"`
}

// TranslationConfig 翻译配置.
type TranslationConfig struct {
	Enabled            bool     `json:"enabled"`
	DefaultLanguage    string   `json:"defaultLanguage"`
	SupportedLanguages []string `json:"supportedLanguages"`
	AutoTranslate      bool     `json:"autoTranslate"`
	Provider           string   `json:"provider"` // openai, deepl, google
}

// DefaultTranslationConfig 默认翻译配置.
func DefaultTranslationConfig() *TranslationConfig {
	return &TranslationConfig{
		Enabled:            true,
		DefaultLanguage:    "zh-CN",
		SupportedLanguages: []string{"zh-CN", "en", "ja", "ko"},
		AutoTranslate:      false,
		Provider:           "openai",
	}
}

// ChatPlusConfig ChatPlus 配置.
type ChatPlusConfig struct {
	Enabled              bool               `json:"enabled"`
	MaxMessageLength     int                `json:"maxMessageLength"`
	MaxFileSize          int64              `json:"maxFileSize"`
	MaxChannels          int                `json:"maxChannels"`
	MaxMembersPerChannel int                `json:"maxMembersPerChannel"`
	RetentionDays        int                `json:"retentionDays"`
	Translation          *TranslationConfig `json:"translation"`
}

// DefaultChatPlusConfig 默认配置.
func DefaultChatPlusConfig() *ChatPlusConfig {
	return &ChatPlusConfig{
		Enabled:              true,
		MaxMessageLength:     10000,
		MaxFileSize:          100 * 1024 * 1024, // 100MB
		MaxChannels:          1000,
		MaxMembersPerChannel: 500,
		RetentionDays:        365,
		Translation:          DefaultTranslationConfig(),
	}
}

// ChatPlusManager ChatPlus 管理器.
type ChatPlusManager struct {
	mu          sync.RWMutex
	config      *ChatPlusConfig
	users       map[string]*ChatUser
	channels    map[string]*ChatChannel
	messages    map[string][]*ChatMessage // channelID -> messages
	onlineUsers map[string]bool
	running     bool
	stopCh      chan struct{}
	onMessage   func(msg *ChatMessage)
	onTyping    func(channelID, userID string)
}

// TypingIndicator 输入指示器.
type TypingIndicator struct {
	ChannelID string    `json:"channelId"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadReceipt 已读回执.
type ReadReceipt struct {
	MessageID string    `json:"messageId"`
	UserID    string    `json:"userId"`
	ReadAt    time.Time `json:"readAt"`
}

// SearchResult 搜索结果.
type SearchResult struct {
	Messages []*ChatMessage `json:"messages"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Query    string         `json:"query"`
}

// ChatStats 聊天统计.
type ChatStats struct {
	TotalUsers      int       `json:"totalUsers"`
	OnlineUsers     int       `json:"onlineUsers"`
	TotalChannels   int       `json:"totalChannels"`
	TotalMessages   int       `json:"totalMessages"`
	MessagesToday   int       `json:"messagesToday"`
	AvgResponseTime float64   `json:"avgResponseTime"`
	LastUpdated     time.Time `json:"lastUpdated"`
}
