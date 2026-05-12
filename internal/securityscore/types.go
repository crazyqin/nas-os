// Package securityscore 提供安全评分功能
package securityscore

import (
	"time"
)

// CheckStatus 检查状态.
type CheckStatus string

const (
	StatusPass    CheckStatus = "pass"
	StatusFail    CheckStatus = "fail"
	StatusWarning CheckStatus = "warning"
)

// Grade 等级.
type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeF Grade = "F"
)

// SecurityScore 安全评分.
type SecurityScore struct {
	Overall     float64                  `json:"overall"`
	Categories  map[string]CategoryScore `json:"categories"`
	Grade       Grade                    `json:"grade"`
	LastUpdated time.Time                `json:"last_updated"`
}

// CategoryScore 分类评分.
type CategoryScore struct {
	Name   string         `json:"name"`
	Score  float64        `json:"score"`
	Weight float64        `json:"weight"`
	Checks []SecurityCheck `json:"checks"`
	Issues []string       `json:"issues"`
}

// SecurityCheck 安全检查.
type SecurityCheck struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Status      CheckStatus `json:"status"`
	Details     string      `json:"details"`
}

// ScoreHistory 评分历史记录.
type ScoreHistory struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Score     SecurityScore  `json:"score"`
}

// Recommendation 改进建议.
type Recommendation struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high/medium/low
	Impact      string `json:"impact"`
}
