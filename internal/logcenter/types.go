// Package logcenter 提供系统日志中心功能
// 对标群晖 Log Center，实现日志收集、查询、过滤、实时流
package logcenter

import (
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// LogSource 日志来源
type LogSource string

const (
	SourceSystem  LogSource = "system"
	SourceAuth    LogSource = "auth"
	SourceDocker  LogSource = "docker"
	SourceSMB     LogSource = "smb"
	SourceNFS     LogSource = "nfs"
	SourceNetwork LogSource = "network"
	SourceStorage LogSource = "storage"
	SourceApp     LogSource = "app"
)

// LogEntry 日志条目
type LogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Source    LogSource `json:"source"`
	Category  string    `json:"category,omitempty"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Hostname  string    `json:"hostname,omitempty"`
	Service   string    `json:"service,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
}

// LogQuery 日志查询条件
type LogQuery struct {
	Keywords  string    `form:"keywords"`
	Level     LogLevel  `form:"level"`
	Source    LogSource `form:"source"`
	Category  string    `form:"category"`
	Hostname  string    `form:"hostname"`
	Service   string    `form:"service"`
	StartTime time.Time `form:"start_time"`
	EndTime   time.Time `form:"end_time"`
	Page      int       `form:"page,default=1"`
	PageSize  int       `form:"page_size,default=50"`
	SortBy    string    `form:"sort_by,default=timestamp"`
	SortDesc  bool      `form:"sort_desc,default=true"`
}

// LogQueryResult 日志查询结果
type LogQueryResult struct {
	Logs       []LogEntry `json:"logs"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// LogStats 日志统计
type LogStats struct {
	TotalCount    int            `json:"total_count"`
	LevelCounts   map[string]int `json:"level_counts"`
	SourceCounts  map[string]int `json:"source_counts"`
	RecentErrors  []LogEntry     `json:"recent_errors"`
	OldestLog     time.Time      `json:"oldest_log"`
	NewestLog     time.Time      `json:"newest_log"`
	TodayCount    int            `json:"today_count"`
	ErrorRate24h  float64        `json:"error_rate_24h"`
}

// LogConfig 日志配置
type LogConfig struct {
	MaxEntries     int  `json:"max_entries"`      // 最大日志条数
	RetentionDays  int  `json:"retention_days"`   // 保留天数
	EnableSyslog   bool `json:"enable_syslog"`    // 收集 syslog
	EnableAuth     bool `json:"enable_auth"`      // 收集认证日志
	EnableDocker   bool `json:"enable_docker"`    // 收集 Docker 日志
	StreamEnabled  bool `json:"stream_enabled"`   // 启用实时流
}

// DefaultConfig 默认配置
func DefaultConfig() *LogConfig {
	return &LogConfig{
		MaxEntries:    100000,
		RetentionDays: 30,
		EnableSyslog:  true,
		EnableAuth:    true,
		EnableDocker:  true,
		StreamEnabled: true,
	}
}

// LogStreamMessage 实时流消息
type LogStreamMessage struct {
	Type  string    `json:"type"` // "log", "stats", "error"
	Log   *LogEntry `json:"log,omitempty"`
	Stats *LogStats `json:"stats,omitempty"`
	Error string    `json:"error,omitempty"`
}
