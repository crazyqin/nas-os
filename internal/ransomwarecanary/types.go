// Package ransomwarecanary 提供勒索软件金丝雀检测系统
package ransomwarecanary

import (
	"time"
)

// CanaryFile 金丝雀文件.
type CanaryFile struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`         // 文件名，如 financial_report_2026.xlsx
	FilePath    string     `json:"file_path"`    // 完整路径
	ContentHash string     `json:"content_hash"` // 内容 SHA256 哈希
	FileSize    int64      `json:"file_size"`    // 文件大小（字节）
	Status      string     `json:"status"`       // active, triggered, disabled
	ShareName   string     `json:"share_name"`   // 所属共享名称
	CreatedAt   time.Time  `json:"created_at"`
	LastChecked time.Time  `json:"last_checked"`
	TriggeredAt *time.Time `json:"triggered_at,omitempty"`
}

// CanaryConfig 金丝雀配置.
type CanaryConfig struct {
	Enabled          bool     `json:"enabled"`
	CheckIntervalSec int      `json:"check_interval_sec"`  // 检查间隔（秒）
	MonitoredPaths   []string `json:"monitored_paths"`     // 监控路径列表
	AutoLockEnabled  bool     `json:"auto_lock_enabled"`   // 是否自动锁定共享
	AlertWebhookURL  string   `json:"alert_webhook_url"`   // 告警 webhook
	MaxAlertsPerHour int      `json:"max_alerts_per_hour"` // 每小时最大告警数
}

// CanaryAlert 金丝雀告警.
type CanaryAlert struct {
	ID          string    `json:"id"`
	CanaryID    string    `json:"canary_id"`
	CanaryName  string    `json:"canary_name"`
	AlertType   string    `json:"alert_type"` // modified, deleted, encrypted, access_denied
	Severity    string    `json:"severity"`   // low, medium, high, critical
	Description string    `json:"description"`
	ShareName   string    `json:"share_name"`
	ShareLocked bool      `json:"share_locked"`
	Timestamp   time.Time `json:"timestamp"`
}

// DetectionResult 检测结果.
type DetectionResult struct {
	TotalChecked int            `json:"total_checked"`
	AlertCount   int            `json:"alert_count"`
	Alerts       []*CanaryAlert `json:"alerts"`
	Timestamp    time.Time      `json:"timestamp"`
	Duration     time.Duration  `json:"duration"`
}

// DeployCanaryRequest 部署金丝雀请求.
type DeployCanaryRequest struct {
	Name      string `json:"name" binding:"required"`
	ShareName string `json:"share_name" binding:"required"`
	FilePath  string `json:"file_path"` // 可选，为空则自动生成
}

// CanaryStatusResponse 金丝雀状态响应.
type CanaryStatusResponse struct {
	Enabled        bool         `json:"enabled"`
	TotalCanaries  int          `json:"total_canaries"`
	ActiveCount    int          `json:"active_count"`
	TriggeredCount int          `json:"triggered_count"`
	TotalAlerts    int          `json:"total_alerts"`
	LastCheckTime  *time.Time   `json:"last_check_time,omitempty"`
	Config         CanaryConfig `json:"config"`
}

// ShareLockRequest 锁定/解锁共享请求.
type ShareLockRequest struct {
	ShareName string `json:"share_name" binding:"required"`
	Reason    string `json:"reason"`
}
