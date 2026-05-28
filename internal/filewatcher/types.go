// Package filewatcher 提供实时文件变更监控
package filewatcher

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrWatcherNotFound 监控器不存在.
	ErrWatcherNotFound = errors.New("监控器不存在")
	// ErrWatcherAlreadyExists 监控器已存在.
	ErrWatcherAlreadyExists = errors.New("监控器已存在")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
)

// ========== 核心类型 ==========

// EventType 事件类型.
type EventType string

const (
	// EventCreate 创建事件.
	EventCreate EventType = "create"
	// EventModify 修改事件.
	EventModify EventType = "modify"
	// EventDelete 删除事件.
	EventDelete EventType = "delete"
	// EventRename 重命名事件.
	EventRename EventType = "rename"
	// EventChmod 权限变更事件.
	EventChmod EventType = "chmod"
)

// WatcherStatus 监控器状态.
type WatcherStatus string

const (
	// WatcherStatusActive 活跃.
	WatcherStatusActive WatcherStatus = "active"
	// WatcherStatusInactive 未活跃.
	WatcherStatusInactive WatcherStatus = "inactive"
)

// ========== 数据结构 ==========

// Watcher 文件监控器.
type Watcher struct {
	ID        string        `json:"id"`         // 监控器ID
	Name      string        `json:"name"`       // 名称
	Paths     []string      `json:"paths"`      // 监控路径
	Events    []EventType   `json:"events"`     // 监控事件类型
	Patterns  []string      `json:"patterns"`   // 文件匹配模式（glob）
	Recursive bool          `json:"recursive"`  // 是否递归
	Status    WatcherStatus `json:"status"`     // 状态
	Webhook   string        `json:"webhook"`    // 回调URL
	CreatedAt time.Time     `json:"created_at"` // 创建时间
	UpdatedAt time.Time     `json:"updated_at"` // 更新时间
}

// FileEvent 文件事件.
type FileEvent struct {
	ID        string    `json:"id"`         // 事件ID
	WatcherID string    `json:"watcher_id"` // 监控器ID
	Type      EventType `json:"type"`       // 事件类型
	Path      string    `json:"path"`       // 文件路径
	OldPath   string    `json:"old_path"`   // 旧路径（重命名）
	Size      int64     `json:"size"`       // 文件大小
	Timestamp time.Time `json:"timestamp"`  // 时间戳
}

// WatcherStats 监控统计.
type WatcherStats struct {
	TotalWatchers int64 `json:"total_watchers"` // 总监控器数
	ActiveWatchers int64 `json:"active_watchers"` // 活跃监控器数
	TotalEvents   int64 `json:"total_events"`   // 总事件数
	EventsToday   int64 `json:"events_today"`   // 今日事件数
}

// CreateWatcherRequest 创建监控器请求.
type CreateWatcherRequest struct {
	Name      string      `json:"name" binding:"required"`
	Paths     []string    `json:"paths" binding:"required,min=1"`
	Events    []EventType `json:"events"`
	Patterns  []string    `json:"patterns"`
	Recursive bool        `json:"recursive"`
	Webhook   string      `json:"webhook"`
}
