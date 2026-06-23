package dockercompose

import "time"

// LogLevel 日志级别.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry 日志条目.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Service   string    `json:"service"`
	Project   string    `json:"project"`
	Message   string    `json:"message"`
}

// LogOptions 日志查询选项.
type LogOptions struct {
	Since time.Time `json:"since,omitempty"`
	Until time.Time `json:"until,omitempty"`
	Tail  int       `json:"tail,omitempty"`
	Level LogLevel  `json:"level,omitempty"`
}

// HealthStatus 健康状态.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStarting  HealthStatus = "starting"
	HealthUnknown   HealthStatus = "unknown"
)

// HealthServiceStatus 服务健康状态.
type HealthServiceStatus struct {
	Project   string       `json:"project"`
	Service   string       `json:"service"`
	Status    HealthStatus `json:"status"`
	LastCheck time.Time    `json:"lastCheck"`
	FailCount int          `json:"failCount"`
	Message   string       `json:"message,omitempty"`
}

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	Timestamp time.Time `json:"timestamp"`
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message,omitempty"`
	Duration  int64     `json:"durationMs,omitempty"`
}
