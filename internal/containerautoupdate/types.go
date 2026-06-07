// Package containerautoupdate 提供容器自动更新功能
package containerautoupdate

import (
	"time"
)

// UpdateStatus 更新状态.
type UpdateStatus string

const (
	StatusPending     UpdateStatus = "pending"
	StatusDownloading UpdateStatus = "downloading"
	StatusStopping    UpdateStatus = "stopping"
	StatusStarting    UpdateStatus = "starting"
	StatusHealthCheck UpdateStatus = "health_check"
	StatusRolledBack  UpdateStatus = "rolled_back"
	StatusSuccess     UpdateStatus = "success"
	StatusFailed      UpdateStatus = "failed"
)

// HealthStatus 健康状态.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStarting  HealthStatus = "starting"
	HealthUnknown   HealthStatus = "unknown"
)

// UpdatePolicy 更新策略.
type UpdatePolicy struct {
	ID                 string    `json:"id"`
	ContainerID        string    `json:"container_id"`
	ContainerName      string    `json:"container_name"`
	Enabled            bool      `json:"enabled"`
	Schedule           string    `json:"schedule"` // cron 表达式
	MaxRetries         int       `json:"max_retries"`
	RollbackOnFailure  bool      `json:"rollback_on_failure"`
	HealthCheckURL     string    `json:"health_check_url"`
	HealthCheckTimeout int       `json:"health_check_timeout"` // 秒
	PreUpdateHook      string    `json:"pre_update_hook"`
	PostUpdateHook     string    `json:"post_update_hook"`
	NotifyOnUpdate     bool      `json:"notify_on_update"`
	NotifyOnFailure    bool      `json:"notify_on_failure"`
	CreatedAt          time.Time `json:"created_at"`
}

// UpdateRecord 更新记录.
type UpdateRecord struct {
	ID            string       `json:"id"`
	ContainerID   string       `json:"container_id"`
	OldImage      string       `json:"old_image"`
	NewImage      string       `json:"new_image"`
	OldDigest     string       `json:"old_digest"`
	NewDigest     string       `json:"new_digest"`
	Status        UpdateStatus `json:"status"`
	StartedAt     time.Time    `json:"started_at"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	Duration      int64        `json:"duration"` // 毫秒
	Error         string       `json:"error,omitempty"`
	RollbackImage string       `json:"rollback_image,omitempty"`
}

// UpdateCheck 更新检查结果.
type UpdateCheck struct {
	ContainerID   string    `json:"container_id"`
	CurrentImage  string    `json:"current_image"`
	CurrentDigest string    `json:"current_digest"`
	LatestDigest  string    `json:"latest_digest"`
	LatestTag     string    `json:"latest_tag"`
	HasUpdate     bool      `json:"has_update"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ContainerHealth 容器健康状态.
type ContainerHealth struct {
	ContainerID         string       `json:"container_id"`
	Status              HealthStatus `json:"status"`
	LastCheck           time.Time    `json:"last_check"`
	ConsecutiveFailures int          `json:"consecutive_failures"`
	Uptime              int64        `json:"uptime"` // 秒
	RestartCount        int          `json:"restart_count"`
	CPU                 float64      `json:"cpu"`    // 百分比
	Memory              int64        `json:"memory"` // 字节
	NetworkIO           NetworkIO    `json:"network_io"`
}

// NetworkIO 网络 IO 统计.
type NetworkIO struct {
	RxBytes int64 `json:"rx_bytes"`
	TxBytes int64 `json:"tx_bytes"`
}

// UpdateStats 更新统计.
type UpdateStats struct {
	TotalUpdates      int       `json:"total_updates"`
	SuccessfulUpdates int       `json:"successful_updates"`
	FailedUpdates     int       `json:"failed_updates"`
	RolledBackUpdates int       `json:"rolled_back_updates"`
	AvgUpdateDuration float64   `json:"avg_update_duration"` // 毫秒
	LastUpdateTime    time.Time `json:"last_update_time"`
}

// RollbackConfig 回滚配置.
type RollbackConfig struct {
	MaxHistory    int           `json:"max_history"`
	AutoRollback  bool          `json:"auto_rollback"`
	RollbackDelay time.Duration `json:"rollback_delay"`
}

// SetPolicyRequest 设置策略请求.
type SetPolicyRequest struct {
	ContainerID        string `json:"container_id" binding:"required"`
	ContainerName      string `json:"container_name"`
	Enabled            bool   `json:"enabled"`
	Schedule           string `json:"schedule"`
	MaxRetries         int    `json:"max_retries"`
	RollbackOnFailure  bool   `json:"rollback_on_failure"`
	HealthCheckURL     string `json:"health_check_url"`
	HealthCheckTimeout int    `json:"health_check_timeout"`
	PreUpdateHook      string `json:"pre_update_hook"`
	PostUpdateHook     string `json:"post_update_hook"`
	NotifyOnUpdate     bool   `json:"notify_on_update"`
	NotifyOnFailure    bool   `json:"notify_on_failure"`
}

// ApplyUpdateRequest 应用更新请求.
type ApplyUpdateRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
}
