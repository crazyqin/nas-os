// Package smartscrub 提供智能 ZFS 擦洗调度
package smartscrub

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPolicyNotFound 策略不存在.
	ErrPolicyNotFound = errors.New("策略不存在")
	// ErrPolicyAlreadyExists 策略已存在.
	ErrPolicyAlreadyExists = errors.New("策略已存在")
	// ErrPoolNotFound 存储池不存在.
	ErrPoolNotFound = errors.New("存储池不存在")
	// ErrScrubRunning 擦洗正在运行.
	ErrScrubRunning = errors.New("擦洗正在运行")
)

// ========== 核心类型 ==========

// ScrubPriority 擦洗优先级.
type ScrubPriority string

const (
	// PriorityLow 低优先级（后台运行）.
	PriorityLow ScrubPriority = "low"
	// PriorityNormal 正常优先级.
	PriorityNormal ScrubPriority = "normal"
	// PriorityHigh 高优先级.
	PriorityHigh ScrubPriority = "high"
)

// ScrubStatus 擦洗状态.
type ScrubStatus string

const (
	// ScrubStatusIdle 空闲.
	ScrubStatusIdle ScrubStatus = "idle"
	// ScrubStatusRunning 运行中.
	ScrubStatusRunning ScrubStatus = "running"
	// ScrubStatusPaused 暂停.
	ScrubStatusPaused ScrubStatus = "paused"
	// ScrubStatusCompleted 已完成.
	ScrubStatusCompleted ScrubStatus = "completed"
	// ScrubStatusFailed 失败.
	ScrubStatusFailed ScrubStatus = "failed"
)

// TriggerMode 触发模式.
type TriggerMode string

const (
	// TriggerManual 手动触发.
	TriggerManual TriggerMode = "manual"
	// TriggerSchedule 定时触发.
	TriggerSchedule TriggerMode = "schedule"
	// TriggerSmart 智能触发（基于数据变化量）.
	TriggerSmart TriggerMode = "smart"
)

// ========== 数据结构 ==========

// ScrubPolicy 擦洗策略.
type ScrubPolicy struct {
	ID              string        `json:"id"`               // 策略ID
	Name            string        `json:"name"`             // 策略名称
	Pools           []string      `json:"pools"`            // 目标池列表
	Trigger         TriggerMode   `json:"trigger"`          // 触发模式
	Schedule        string        `json:"schedule"`         // Cron表达式
	Priority        ScrubPriority `json:"priority"`         // 优先级
	ThresholdDays   int           `json:"threshold_days"`   // 智能触发阈值天数
	ThresholdChange float64       `json:"threshold_change"` // 智能触发数据变化比例
	MaxDuration     time.Duration `json:"max_duration"`     // 最大持续时间
	Enabled         bool          `json:"enabled"`          // 是否启用
	CreatedAt       time.Time     `json:"created_at"`       // 创建时间
	UpdatedAt       time.Time     `json:"updated_at"`       // 更新时间
}

// ScrubRecord 擦洗记录.
type ScrubRecord struct {
	ID        string      `json:"id"`         // 记录ID
	PolicyID  string      `json:"policy_id"`  // 策略ID
	Pool      string      `json:"pool"`       // 存储池
	Status    ScrubStatus `json:"status"`     // 状态
	StartTime time.Time   `json:"start_time"` // 开始时间
	EndTime   time.Time   `json:"end_time"`   // 结束时间
	Duration  time.Duration `json:"duration"` // 耗时
	Errors    int         `json:"errors"`     // 错误数
	Repaired  int         `json:"repaired"`   // 修复数
	Summary   string      `json:"summary"`    // 摘要
}

// ScrubStats 擦洗统计.
type ScrubStats struct {
	TotalPolicies  int64     `json:"total_policies"`  // 总策略数
	ActivePolicies int64     `json:"active_policies"` // 活跃策略数
	TotalScrubs    int64     `json:"total_scrubs"`    // 总擦洗次数
	TotalErrors    int64     `json:"total_errors"`    // 总错误数
	LastScrubTime  *time.Time `json:"last_scrub_time"` // 最后擦洗时间
}

// CreatePolicyRequest 创建策略请求.
type CreatePolicyRequest struct {
	Name            string        `json:"name" binding:"required"`
	Pools           []string      `json:"pools" binding:"required,min=1"`
	Trigger         TriggerMode   `json:"trigger"`
	Schedule        string        `json:"schedule"`
	Priority        ScrubPriority `json:"priority"`
	ThresholdDays   int           `json:"threshold_days"`
	ThresholdChange float64       `json:"threshold_change"`
	MaxDuration     time.Duration `json:"max_duration"`
}
