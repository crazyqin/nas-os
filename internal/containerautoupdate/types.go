// Package containerautoupdate 提供 Docker 容器自动更新功能
package containerautoupdate

import (
	"time"
)

// UpdateStatus 更新状态.
type UpdateStatus string

const (
	StatusPending    UpdateStatus = "pending"
	StatusChecking   UpdateStatus = "checking"
	StatusPulling    UpdateStatus = "pulling"
	StatusStopping   UpdateStatus = "stopping"
	StatusStarting   UpdateStatus = "starting"
	StatusHealthCheck UpdateStatus = "health_check"
	StatusCompleted  UpdateStatus = "completed"
	StatusFailed     UpdateStatus = "failed"
	StatusRolledBack UpdateStatus = "rolled_back"
)

// ContainerConfig 容器配置.
type ContainerConfig struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Image         string          `json:"image"`
	Tag           string          `json:"tag"`
	Enabled       bool            `json:"enabled"`
	Policy        UpdatePolicy    `json:"policy"`
	Rollback      RollbackConfig  `json:"rollback"`
	HealthCheck   HealthCheckConfig `json:"health_check"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// UpdatePolicy 更新策略.
type UpdatePolicy struct {
	Schedule    string `json:"schedule"`     // cron 表达式，如 "0 3 * * *" 表示每天凌晨3点
	AutoUpdate  bool   `json:"auto_update"`  // 是否自动更新
	NotifyOnly  bool   `json:"notify_only"`  // 仅通知不更新
	MaxRetries  int    `json:"max_retries"`  // 最大重试次数
	PreScript   string `json:"pre_script"`   // 更新前脚本
	PostScript  string `json:"post_script"`  // 更新后脚本
}

// RollbackConfig 回滚配置.
type RollbackConfig struct {
	Enabled         bool          `json:"enabled"`
	AutoRollback    bool          `json:"auto_rollback"`    // 健康检查失败时自动回滚
	MaxHistory      int           `json:"max_history"`      // 保留的历史版本数量
	RollbackTimeout time.Duration `json:"rollback_timeout"` // 回滚超时时间
}

// HealthCheckConfig 健康检查配置.
type HealthCheckConfig struct {
	Enabled         bool          `json:"enabled"`
	URL             string        `json:"url"`              // 健康检查 URL
	Interval        time.Duration `json:"interval"`         // 检查间隔
	Timeout         time.Duration `json:"timeout"`          // 超时时间
	Retries         int           `json:"retries"`          // 重试次数
	ExpectedStatus  int           `json:"expected_status"`  // 期望的 HTTP 状态码
}

// ContainerUpdate 更新记录.
type ContainerUpdate struct {
	ID            string       `json:"id"`
	ContainerID   string       `json:"container_id"`
	ContainerName string       `json:"container_name"`
	OldImage      string       `json:"old_image"`
	OldTag        string       `json:"old_tag"`
	NewImage      string       `json:"new_image"`
	NewTag        string       `json:"new_tag"`
	Status        UpdateStatus `json:"status"`
	Error         string       `json:"error,omitempty"`
	StartedAt     time.Time    `json:"started_at"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	Duration      int64        `json:"duration"` // 毫秒
}

// UpdateStats 更新统计.
type UpdateStats struct {
	TotalUpdates   int       `json:"total_updates"`
	SuccessCount   int       `json:"success_count"`
	FailedCount    int       `json:"failed_count"`
	RollbackCount  int       `json:"rollback_count"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

// CheckUpdateRequest 检查更新请求.
type CheckUpdateRequest struct {
	ContainerID string `json:"container_id,omitempty"`
}

// ApplyUpdateRequest 应用更新请求.
type ApplyUpdateRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
	NewTag      string `json:"new_tag,omitempty"`
}

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
	UpdateID    string `json:"update_id,omitempty"`
}

// AddContainerRequest 添加容器请求.
type AddContainerRequest struct {
	Name        string           `json:"name" binding:"required"`
	Image       string           `json:"image" binding:"required"`
	Tag         string           `json:"tag"`
	Policy      UpdatePolicy     `json:"policy"`
	Rollback    RollbackConfig   `json:"rollback"`
	HealthCheck HealthCheckConfig `json:"health_check"`
}
