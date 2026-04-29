// Package selfheal 提供系统健康自检与自愈功能
package selfheal

import (
	"time"
)

// Status 健康状态.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// HealAction 自愈动作类型.
type HealAction string

const (
	HealActionNone   HealAction = "none"   // 仅告警
	HealActionAuto   HealAction = "auto"   // 自动修复
	HealActionManual HealAction = "manual" // 需人工确认
)

// CheckCategory 检查类别.
type CheckCategory string

const (
	CategoryDisk       CheckCategory = "disk"
	CategoryFilesystem CheckCategory = "filesystem"
	CategoryService    CheckCategory = "service"
	CategoryConfig     CheckCategory = "config"
	CategoryCert       CheckCategory = "certificate"
	CategoryCustom     CheckCategory = "custom"
)

// Checker 健康检查器接口.
// 模块可通过实现此接口注册自定义检查项.
type Checker interface {
	// Name 返回检查器名称（唯一标识）.
	Name() string
	// Category 返回检查类别.
	Category() CheckCategory
	// Description 返回检查项描述.
	Description() string
	// Check 执行健康检查并返回结果.
	Check(ctx *CheckContext) *CheckResult
	// Heal 尝试修复问题. 返回修复结果.
	// 如果 HealAction 为 Manual，此方法不应执行修复.
	Heal(ctx *CheckContext, result *CheckResult) *HealResult
	// HealAction 返回默认自愈策略.
	HealAction() HealAction
}

// CheckContext 检查上下文.
type CheckContext struct {
	Timeout time.Duration `json:"-"`
	Forced  bool          `json:"forced"` // 是否为手动强制执行
}

// CheckResult 检查结果.
type CheckResult struct {
	Name      string                 `json:"name"`
	Category  CheckCategory          `json:"category"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
}

// HealResult 修复结果.
type HealResult struct {
	Success       bool   `json:"success"`
	Action        string `json:"action"`
	Message       string `json:"message"`
	NeedsApproval bool   `json:"needs_approval"` // 需要人工确认
}

// HealRecord 检查与修复记录（持久化）.
type HealRecord struct {
	ID            int64     `json:"id"`
	CheckName     string    `json:"check_name"`
	Category      string    `json:"category"`
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	Details       string    `json:"details,omitempty"` // JSON 字符串
	HealAction    string    `json:"heal_action"`
	HealSuccess   *bool     `json:"heal_success,omitempty"`
	HealMessage   string    `json:"heal_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// OverallStatus 整体健康状态.
type OverallStatus struct {
	Status    Status        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Summary   *StatusSummary `json:"summary"`
	Checks    []*CheckResult `json:"checks"`
	Healed    int           `json:"healed"` // 本轮自愈次数
}

// StatusSummary 状态摘要.
type StatusSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Degraded  int `json:"degraded"`
	Unhealthy int `json:"unhealthy"`
	Healed    int `json:"healed"`
}

// StrategyConfig 自愈策略配置.
type StrategyConfig struct {
	DefaultAction HealAction          `json:"default_action"` // 全局默认策略
	Overrides     map[string]HealAction `json:"overrides"`    // 按检查项覆盖
	CheckInterval time.Duration       `json:"check_interval"` // 定期检查间隔
	Enabled       bool                `json:"enabled"`        // 是否启用调度
}
