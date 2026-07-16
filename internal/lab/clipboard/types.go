// Package clipboard 提供跨设备剪贴板同步功能，对标飞牛fnOS剪贴板同步
package clipboard

import (
	"time"
)

// ClipItem 剪贴板条目.
type ClipItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Type      ClipType  `json:"type"`
	Source    string    `json:"source"` // 设备标识
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ClipType 剪贴板内容类型.
type ClipType string

const (
	ClipTypeText  ClipType = "text"
	ClipTypeImage ClipType = "image"
	ClipTypeFile  ClipType = "file"
	ClipTypeLink  ClipType = "link"
)

// ClipboardStats 剪贴板统计.
type ClipboardStats struct {
	TotalItems  int64     `json:"total_items"`
	TotalSize   int64     `json:"total_bytes"`
	ActiveUsers int       `json:"active_users"`
	OldestItem  time.Time `json:"oldest_item"`
	NewestItem  time.Time `json:"newest_item"`
	DeviceCount int       `json:"device_count"`
}

// SyncMessage WebSocket同步消息.
type SyncMessage struct {
	Action   string     `json:"action"` // push, pull, delete
	Items    []ClipItem `json:"items,omitempty"`
	DeviceID string     `json:"device_id"`
}

// CreateClipRequest 创建剪贴板条目请求.
type CreateClipRequest struct {
	Content string   `json:"content" binding:"required"`
	Type    ClipType `json:"type"`
	Source  string   `json:"source"`
	TTL     int      `json:"ttl"` // 过期时间（秒），0表示永不过期
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query    string   `json:"query"`
	Type     ClipType `json:"type"`
	UserID   string   `json:"user_id"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}
