// Package onboarding 提供新手引导功能，支持首次配置向导、功能引导、最佳实践推荐、进度追踪。
package onboarding

import "time"

// ============================================================
// 新手引导相关类型
// ============================================================

// Wizard 配置向导.
type Wizard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Steps       []*Step   `json:"steps"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Step 向导步骤.
type Step struct {
	ID          string    `json:"id"`
	WizardID    string    `json:"wizard_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
	Type        string    `json:"type"`
	IsCompleted bool      `json:"is_completed"`
	IsOptional  bool      `json:"is_optional"`
	CreatedAt   time.Time `json:"created_at"`
}

// Guide 功能引导.
type Guide struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Content     string    `json:"content"`
	Icon        string    `json:"icon"`
	Tags        []string  `json:"tags"`
	Duration    int       `json:"duration"`
	CreatedAt   time.Time `json:"created_at"`
}

// BestPractice 最佳实践.
type BestPractice struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// Progress 进度.
type Progress struct {
	UserID         string     `json:"user_id"`
	WizardID       string     `json:"wizard_id"`
	CompletedSteps []string   `json:"completed_steps"`
	TotalSteps     int        `json:"total_steps"`
	CompletedCount int        `json:"completed_count"`
	Percentage     float64    `json:"percentage"`
	IsCompleted    bool       `json:"is_completed"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ============================================================
// 请求类型
// ============================================================

// CompleteStepRequest 完成步骤请求.
type CompleteStepRequest struct {
	UserID string `json:"user_id" binding:"required"`
	StepID string `json:"step_id" binding:"required"`
}

// GetGuidesRequest 获取引导请求.
type GetGuidesRequest struct {
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}
