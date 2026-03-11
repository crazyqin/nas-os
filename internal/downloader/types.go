package downloader

import (
	"time"
)

// DownloadType 下载类型
type DownloadType string

const (
	TypeBT       DownloadType = "bt"       // BT 种子
	TypeMagnet   DownloadType = "magnet"   // 磁力链接
	TypeHTTP     DownloadType = "http"     // HTTP 下载
	TypeFTP      DownloadType = "ftp"      // FTP 下载
	TypeCloud    DownloadType = "cloud"    // 网盘
)

// DownloadStatus 下载状态
type DownloadStatus string

const (
	StatusWaiting   DownloadStatus = "waiting"   // 等待中
	StatusDownloading DownloadStatus = "downloading" // 下载中
	StatusPaused    DownloadStatus = "paused"    // 已暂停
	StatusCompleted DownloadStatus = "completed" // 已完成
	StatusError     DownloadStatus = "error"     // 错误
	StatusSeeding   DownloadStatus = "seeding"   // 做种中
)

// DownloadTask 下载任务
type DownloadTask struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         DownloadType   `json:"type"`
	URL          string         `json:"url"`           // 磁力链接/HTTP URL/种子文件路径
	Status       DownloadStatus `json:"status"`
	Progress     float64        `json:"progress"`      // 进度 0-100
	TotalSize    int64          `json:"total_size"`    // 总大小 (字节)
	Downloaded   int64          `json:"downloaded"`    // 已下载 (字节)
	Uploaded     int64          `json:"uploaded"`      // 已上传 (字节，PT 用)
	Speed        int64          `json:"speed"`         // 下载速度 (字节/秒)
	UploadSpeed  int64          `json:"upload_speed"`  // 上传速度 (字节/秒)
	Peers        int            `json:"peers"`         // 连接数
	Seeds        int            `json:"seeds"`         // 种子数
	Ratio        float64        `json:"ratio"`         // 分享率
	DestPath     string         `json:"dest_path"`     // 保存路径
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	
	// 计划任务
	Schedule     *ScheduleConfig `json:"schedule,omitempty"`
	// 限速配置
	SpeedLimit   *SpeedLimitConfig `json:"speed_limit,omitempty"`
}

// ScheduleConfig 计划任务配置
type ScheduleConfig struct {
	StartTime string `json:"start_time"` // HH:MM 格式
	EndTime   string `json:"end_time"`   // HH:MM 格式
	Days      []int  `json:"days"`       // 0=周日，1-6=周一到周六
	Enabled   bool   `json:"enabled"`
}

// SpeedLimitConfig 限速配置
type SpeedLimitConfig struct {
	DownloadLimit int64 `json:"download_limit"` // 下载限速 (KB/s), 0=不限
	UploadLimit   int64 `json:"upload_limit"`   // 上传限速 (KB/s), 0=不限
	Enabled       bool  `json:"enabled"`
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	URL        string              `json:"url"`
	Name       string              `json:"name,omitempty"`
	Type       DownloadType        `json:"type,omitempty"`
	DestPath   string              `json:"dest_path,omitempty"`
	Schedule   *ScheduleConfig     `json:"schedule,omitempty"`
	SpeedLimit *SpeedLimitConfig   `json:"speed_limit,omitempty"`
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Status     DownloadStatus      `json:"status,omitempty"`
	SpeedLimit *SpeedLimitConfig   `json:"speed_limit,omitempty"`
	Schedule   *ScheduleConfig     `json:"schedule,omitempty"`
}

// TaskStats 任务统计
type TaskStats struct {
	TotalTasks     int   `json:"total_tasks"`
	Downloading    int   `json:"downloading"`
	Waiting        int   `json:"waiting"`
	Paused         int   `json:"paused"`
	Completed      int   `json:"completed"`
	Seeding        int   `json:"seeding"`
	TotalSpeed     int64 `json:"total_speed"`
	TotalUploaded  int64 `json:"total_uploaded"`
}

// PeerInfo 节点信息
type PeerInfo struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Client   string `json:"client"`
	Progress float64 `json:"progress"`
	Speed    int64  `json:"speed"`
}

// TrackerInfo Tracker 信息
type TrackerInfo struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	Peers      int    `json:"peers"`
	Seeds      int    `json:"seeds"`
	Leechers   int    `json:"leechers"`
	LastUpdate time.Time `json:"last_update"`
}
