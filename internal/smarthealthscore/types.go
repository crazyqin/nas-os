// Package smarthealthscore 提供综合智能健康评分功能，涵盖磁盘、网络、安全、性能、可用性五大维度。
package smarthealthscore

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoScoreData 表示尚未执行过评分。
	ErrNoScoreData = errors.New("尚未执行健康评分，请先调用 POST /api/healthscore")
	// ErrInvalidCategory 表示无效的评分类别。
	ErrInvalidCategory = errors.New("无效的评分类别")
)

// ========== 评分维度 ==========

// ScoreCategory 评分维度类别。
type ScoreCategory string

const (
	// CategoryDisk 磁盘维度。
	CategoryDisk ScoreCategory = "disk"
	// CategoryNetwork 网络维度。
	CategoryNetwork ScoreCategory = "network"
	// CategorySecurity 安全维度。
	CategorySecurity ScoreCategory = "security"
	// CategoryPerformance 性能维度。
	CategoryPerformance ScoreCategory = "performance"
	// CategoryAvailability 可用性维度。
	CategoryAvailability ScoreCategory = "availability"
)

// AllCategories 返回所有支持的评分维度。
func AllCategories() []ScoreCategory {
	return []ScoreCategory{
		CategoryDisk,
		CategoryNetwork,
		CategorySecurity,
		CategoryPerformance,
		CategoryAvailability,
	}
}

// ========== 健康等级 ==========

// HealthLevel 健康等级。
type HealthLevel string

const (
	// LevelExcellent 优秀 (90-100)。
	LevelExcellent HealthLevel = "excellent"
	// LevelGood 良好 (70-89)。
	LevelGood HealthLevel = "good"
	// LevelWarning 警告 (50-69)。
	LevelWarning HealthLevel = "warning"
	// LevelCritical 严重 (30-49)。
	LevelCritical HealthLevel = "critical"
	// LevelDanger 危险 (0-29)。
	LevelDanger HealthLevel = "danger"
)

// ClassifyLevel 根据分数返回健康等级。
func ClassifyLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return LevelExcellent
	case score >= 70:
		return LevelGood
	case score >= 50:
		return LevelWarning
	case score >= 30:
		return LevelCritical
	default:
		return LevelDanger
	}
}

// ========== 维度评分 ==========

// ComponentScore 单维度评分结果。
type ComponentScore struct {
	Category    ScoreCategory `json:"category"`    // 评分维度
	Score       float64       `json:"score"`       // 评分 0-100
	Weight      float64       `json:"weight"`      // 权重 0-1
	Level       HealthLevel   `json:"level"`       // 健康等级
	Description string        `json:"description"` // 评分描述
	Metrics     []Metric      `json:"metrics"`     // 具体指标
}

// Metric 单项指标。
type Metric struct {
	Name   string  `json:"name"`   // 指标名称
	Value  float64 `json:"value"`  // 指标值
	Unit   string  `json:"unit"`   // 单位
	Status string  `json:"status"` // 状态: healthy, warning, critical
	Detail string  `json:"detail"` // 详细说明
}

// ========== 健康评分 ==========

// HealthScore 综合健康评分结果。
type HealthScore struct {
	Overall     float64          `json:"overall"`          // 综合评分 0-100
	Level       HealthLevel      `json:"level"`            // 健康等级
	Components  []ComponentScore `json:"components"`       // 各维度评分
	Alerts      []Alert          `json:"alerts,omitempty"` // 触发的告警
	Suggestions []Suggestion     `json:"suggestions"`      // 改进建议
	EvaluatedAt time.Time        `json:"evaluated_at"`     // 评估时间
}

// ========== 告警 ==========

// Alert 告警记录。
type Alert struct {
	Timestamp time.Time     `json:"timestamp"` // 告警时间
	Category  ScoreCategory `json:"category"`  // 所属维度
	Score     float64       `json:"score"`     // 触发时的分数
	Threshold float64       `json:"threshold"` // 阈值
	Level     HealthLevel   `json:"level"`     // 严重程度
	Message   string        `json:"message"`   // 告警消息
}

// ========== 改进建议 ==========

// Suggestion 改进建议。
type Suggestion struct {
	Category    ScoreCategory `json:"category"`    // 所属维度
	Priority    string        `json:"priority"`    // 优先级: high, medium, low
	Title       string        `json:"title"`       // 建议标题
	Description string        `json:"description"` // 详细描述
	Action      string        `json:"action"`      // 建议操作
}

// ========== 趋势 ==========

// HealthTrend 健康趋势记录。
type HealthTrend struct {
	Timestamp  time.Time                 `json:"timestamp"`  // 记录时间
	Overall    float64                   `json:"overall"`    // 综合评分
	Level      HealthLevel               `json:"level"`      // 健康等级
	Components map[ScoreCategory]float64 `json:"components"` // 各维度分数
}

// TrendQuery 趋势查询参数。
type TrendQuery struct {
	Days     int           `form:"days"`     // 查询最近N天，默认30
	Limit    int           `form:"limit"`    // 最大条数，默认100
	Category ScoreCategory `form:"category"` // 过滤特定维度（可选）
}

// TrendResponse 趋势查询响应。
type TrendResponse struct {
	Trends     []HealthTrend `json:"trends"`      // 趋势记录
	TotalCount int           `json:"total_count"` // 总条数
	AvgScore   float64       `json:"avg_score"`   // 平均分
	MinScore   float64       `json:"min_score"`   // 最低分
	MaxScore   float64       `json:"max_score"`   // 最高分
	Trend      string        `json:"trend"`       // 趋势方向: rising, falling, stable
}

// ========== 告警查询 ==========

// AlertQuery 告警查询参数。
type AlertQuery struct {
	Days     int           `form:"days"`     // 查询最近N天，默认30
	Limit    int           `form:"limit"`    // 最大条数，默认50
	Category ScoreCategory `form:"category"` // 过滤特定维度（可选）
}

// ========== 配置 ==========

// Config 评分配置。
type Config struct {
	Weights   map[ScoreCategory]float64 `json:"weights"`   // 各维度权重
	Threshold float64                   `json:"threshold"` // 告警阈值，默认60
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Weights: map[ScoreCategory]float64{
			CategoryDisk:         0.25,
			CategoryNetwork:      0.15,
			CategorySecurity:     0.25,
			CategoryPerformance:  0.20,
			CategoryAvailability: 0.15,
		},
		Threshold: 60,
	}
}
