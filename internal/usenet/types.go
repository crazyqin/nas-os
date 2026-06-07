// Package usenet 提供 Usenet 下载管理功能，支持 NZB 文件管理、下载队列、服务器配置和索引器搜索。
// 提供完整的 Usenet 下载生命周期管理，包括添加 NZB、启动/暂停/取消下载、队列管理等功能。
package usenet

import "time"

// NZBStatus NZB 状态
type NZBStatus string

const (
	NZBStatusPending     NZBStatus = "pending"
	NZBStatusDownloading NZBStatus = "downloading"
	NZBStatusCompleted   NZBStatus = "completed"
	NZBStatusFailed      NZBStatus = "failed"
	NZBStatusPaused      NZBStatus = "paused"
)

// DownloadStatus 下载状态
type DownloadStatus string

const (
	DownloadStatusPending   DownloadStatus = "pending"
	DownloadStatusActive    DownloadStatus = "active"
	DownloadStatusPaused    DownloadStatus = "paused"
	DownloadStatusCompleted DownloadStatus = "completed"
	DownloadStatusFailed    DownloadStatus = "failed"
	DownloadStatusCancelled DownloadStatus = "cancelled"
)

// NZB NZB 文件信息
type NZB struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Category string    `json:"category,omitempty"`
	Size     int64     `json:"size"`
	Files    int       `json:"files"`
	Poster   string    `json:"poster,omitempty"`
	Groups   []string  `json:"groups,omitempty"`
	PostedAt time.Time `json:"posted_at,omitempty"`
	AddedAt  time.Time `json:"added_at"`
	Status   NZBStatus `json:"status"`
	FilePath string    `json:"file_path,omitempty"`
}

// Download 下载任务信息
type Download struct {
	ID          string         `json:"id"`
	NZBID       string         `json:"nzb_id"`
	Progress    float64        `json:"progress"`
	Speed       int64          `json:"speed"`
	ETA         time.Duration  `json:"eta"`
	Size        int64          `json:"size"`
	Downloaded  int64          `json:"downloaded"`
	Status      DownloadStatus `json:"status"`
	Connections int            `json:"connections"`
	Server      string         `json:"server"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// Server Usenet 服务器配置
type Server struct {
	ID            string `json:"id"`
	Host          string `json:"host" binding:"required"`
	Port          int    `json:"port" binding:"required,min=1,max=65535"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Connections   int    `json:"connections" binding:"required,min=1,max=100"`
	SSL           bool   `json:"ssl"`
	Enabled       bool   `json:"enabled"`
	Priority      int    `json:"priority"`
	RetentionDays int    `json:"retention_days"`
}

// Category 下载分类
type Category struct {
	ID         string   `json:"id"`
	Name       string   `json:"name" binding:"required"`
	DestPath   string   `json:"dest_path" binding:"required"`
	Priority   int      `json:"priority"`
	Extensions []string `json:"extensions,omitempty"`
}

// Indexer Usenet 索引器
type Indexer struct {
	ID         string   `json:"id"`
	Name       string   `json:"name" binding:"required"`
	URL        string   `json:"url" binding:"required,url"`
	APIKey     string   `json:"api_key,omitempty"`
	Enabled    bool     `json:"enabled"`
	Categories []string `json:"categories,omitempty"`
}

// QueueItem 下载队列项
type QueueItem struct {
	ID       string    `json:"id"`
	NZBID    string    `json:"nzb_id"`
	Priority int       `json:"priority"`
	AddedAt  time.Time `json:"added_at"`
	Position int       `json:"position"`
}

// Stats 下载统计信息
type Stats struct {
	TotalDownloaded int64         `json:"total_downloaded"`
	TotalSize       int64         `json:"total_size"`
	CurrentSpeed    int64         `json:"current_speed"`
	ActiveDownloads int           `json:"active_downloads"`
	QueuedItems     int           `json:"queued_items"`
	ServerStats     []ServerStats `json:"server_stats"`
}

// ServerStats 单个服务器的统计信息
type ServerStats struct {
	ServerID        string `json:"server_id"`
	ServerHost      string `json:"server_host"`
	ConnectionsUsed int    `json:"connections_used"`
	CurrentSpeed    int64  `json:"current_speed"`
	TotalDownloaded int64  `json:"total_downloaded"`
}

// DefaultServers 预置的免费 Usenet 服务器
var DefaultServers = []Server{
	{
		ID:            "server-free-001",
		Host:          "news.eternal-september.org",
		Port:          119,
		Connections:   2,
		SSL:           false,
		Enabled:       true,
		Priority:      1,
		RetentionDays: 30,
	},
	{
		ID:            "server-free-002",
		Host:          "news.aioe.org",
		Port:          119,
		Connections:   2,
		SSL:           false,
		Enabled:       true,
		Priority:      2,
		RetentionDays: 30,
	},
	{
		ID:            "server-free-003",
		Host:          "news.mixmin.net",
		Port:          119,
		Connections:   2,
		SSL:           false,
		Enabled:       true,
		Priority:      3,
		RetentionDays: 30,
	},
}

// IsValidNZBStatus 检查 NZB 状态是否有效
func IsValidNZBStatus(status NZBStatus) bool {
	switch status {
	case NZBStatusPending, NZBStatusDownloading, NZBStatusCompleted, NZBStatusFailed, NZBStatusPaused:
		return true
	default:
		return false
	}
}

// IsValidDownloadStatus 检查下载状态是否有效
func IsValidDownloadStatus(status DownloadStatus) bool {
	switch status {
	case DownloadStatusPending, DownloadStatusActive, DownloadStatusPaused, DownloadStatusCompleted, DownloadStatusFailed, DownloadStatusCancelled:
		return true
	default:
		return false
	}
}
