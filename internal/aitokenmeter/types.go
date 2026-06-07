// Package aitokenmeter 提供 AI Token 智能计量功能
// 对标群晖 AI Console 的 Token 管理，支持多提供商统一计量、用户配额、预算控制、审计日志
package aitokenmeter

import (
	"errors"
	"sync"
	"time"
)

// ========== AI 提供商 ==========

// Provider AI 提供商标识.
type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderClaude   Provider = "claude"
	ProviderGemini   Provider = "gemini"
	ProviderDeepSeek Provider = "deepseek"
	ProviderDoubao   Provider = "doubao"
	ProviderLocal    Provider = "local"
	ProviderCustom   Provider = "custom"
)

// ========== 错误定义 ==========

var (
	// ErrQuotaExceeded 配额超限.
	ErrQuotaExceeded = errors.New("token quota exceeded")
	// ErrRateLimited 触发限流.
	ErrRateLimited = errors.New("rate limited, too many requests")
	// ErrBudgetExceeded 预算超限.
	ErrBudgetExceeded = errors.New("budget exceeded")
	// ErrProviderNotFound 提供商未找到.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrUserNotFound 用户未找到.
	ErrUserNotFound = errors.New("user not found")
	// ErrPlanNotFound 套餐未找到.
	ErrPlanNotFound = errors.New("plan not found")
	// ErrInvalidParams 参数无效.
	ErrInvalidParams = errors.New("invalid parameters")
)

// ========== Token 用量记录 ==========

// TokenUsage 单次 Token 使用记录.
type TokenUsage struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	Provider         Provider  `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	Cost             float64   `json:"cost"` // 美元
	RequestID        string    `json:"requestId,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// ========== 用户配额 ==========

// QuotaPeriod 配额周期.
type QuotaPeriod string

const (
	PeriodPerMinute QuotaPeriod = "per_minute"
	PeriodPerHour   QuotaPeriod = "per_hour"
	PeriodPerDay    QuotaPeriod = "per_day"
	PeriodPerMonth  QuotaPeriod = "per_month"
)

// UserQuota 用户 Token 配额.
type UserQuota struct {
	UserID        string                  `json:"userId"`
	PlanID        string                  `json:"planId,omitempty"`
	Limits        map[QuotaPeriod]int     `json:"limits"`                  // 每周期 Token 上限
	CostLimits    map[QuotaPeriod]float64 `json:"costLimits"`              // 每周期费用上限（美元）
	ProviderQuota map[Provider]int        `json:"providerQuota,omitempty"` // 按提供商限额
	Enabled       bool                    `json:"enabled"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// ========== 套餐 ==========

// Plan AI 使用套餐.
type Plan struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	TokenLimits   map[QuotaPeriod]int     `json:"tokenLimits"`
	CostLimits    map[QuotaPeriod]float64 `json:"costLimits"`
	ProviderQuota map[Provider]int        `json:"providerQuota,omitempty"`
	Priority      int                     `json:"priority"` // 越高优先级越高
	Enabled       bool                    `json:"enabled"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// ========== 预算 ==========

// BudgetType 预算类型.
type BudgetType string

const (
	BudgetTypeGlobal  BudgetType = "global"  // 全局预算
	BudgetTypeUser    BudgetType = "user"    // 用户预算
	BudgetTypeProject BudgetType = "project" // 项目预算
)

// Budget 预算配置.
type Budget struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Type           BudgetType  `json:"type"`
	TargetID       string      `json:"targetId"`       // 用户ID/项目ID，全局时为空
	Amount         float64     `json:"amount"`         // 预算金额（美元）
	Spent          float64     `json:"spent"`          // 已花费
	Period         QuotaPeriod `json:"period"`         // 预算周期
	AlertThreshold float64     `json:"alertThreshold"` // 告警阈值 (0.0-1.0)
	Enabled        bool        `json:"enabled"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// ========== 限流 ==========

// RateLimit 限流配置.
type RateLimit struct {
	UserID      string        `json:"userId"`
	Provider    Provider      `json:"provider,omitempty"`
	MaxTokens   int           `json:"maxTokens"`   // 窗口内最大 Token 数
	Window      time.Duration `json:"window"`      // 滑动窗口大小
	MaxRequests int           `json:"maxRequests"` // 窗口内最大请求数
}

// ========== 告警 ==========

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert 告警事件.
type Alert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	UserID    string     `json:"userId,omitempty"`
	BudgetID  string     `json:"budgetId,omitempty"`
	Threshold float64    `json:"threshold,omitempty"`
	Actual    float64    `json:"actual,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// ========== 审计日志 ==========

// AuditAction 审计动作.
type AuditAction string

const (
	AuditActionRecord    AuditAction = "token_recorded"
	AuditActionQuotaSet  AuditAction = "quota_set"
	AuditActionQuotaHit  AuditAction = "quota_exceeded"
	AuditActionRateLimit AuditAction = "rate_limited"
	AuditActionBudgetHit AuditAction = "budget_exceeded"
	AuditActionAlert     AuditAction = "alert_triggered"
	AuditActionPlanSet   AuditAction = "plan_assigned"
)

// AuditLog 审计日志.
type AuditLog struct {
	ID        string      `json:"id"`
	Action    AuditAction `json:"action"`
	UserID    string      `json:"userId,omitempty"`
	Details   string      `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ========== 告警回调 ==========

// AlertHandler 告警回调函数.
type AlertHandler func(alert Alert)

// ========== 滑动窗口内部结构 ==========

// slidingWindow 滑动窗口限流器 (并发安全).
type slidingWindow struct {
	mu          sync.Mutex
	window      time.Duration
	maxTokens   int
	maxRequests int
	events      []windowEvent
}

// windowEvent 窗口事件.
type windowEvent struct {
	timestamp time.Time
	tokens    int
}

// ========== 环形缓冲区 ==========

// ringBuffer 环形缓冲区，用于高效审计日志.
type ringBuffer struct {
	mu   sync.Mutex
	buf  []AuditLog
	size int
	head int
	cnt  int
}
