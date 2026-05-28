// Package storagehealthgrade 提供存储健康评分系统
// 支持多维度健康评估、趋势分析、智能预警
package storagehealthgrade

import (
	"time"
)

// Grade 健康等级
type Grade string

const (
	GradeA Grade = "A" // 90-100 优秀
	GradeB Grade = "B" // 75-89 良好
	GradeC Grade = "C" // 60-74 一般
	GradeD Grade = "D" // 40-59 较差
	GradeF Grade = "F" // 0-39 严重
)

// DimensionType 评估维度
type DimensionType string

const (
	DimStorage   DimensionType = "storage"   // 存储使用率
	DimRAID      DimensionType = "raid"      // RAID 状态
	DimSMART     DimensionType = "smart"     // 磁盘健康
	DimPerf      DimensionType = "performance" // 性能指标
	DimSecurity  DimensionType = "security"  // 安全状态
	DimBackup    DimensionType = "backup"    // 备份状态
	DimNetwork   DimensionType = "network"   // 网络状态
)

// DimensionScore 维度评分
type DimensionScore struct {
	Type        DimensionType `json:"type"`
	Name        string        `json:"name"`
	Score       float64       `json:"score"`       // 0-100
	Weight      float64       `json:"weight"`      // 权重 0-1
	Grade       Grade         `json:"grade"`
	Issues      []string      `json:"issues,omitempty"`
	Suggestions []string      `json:"suggestions,omitempty"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// HealthReport 健康报告
type HealthReport struct {
	ID           string            `json:"id"`
	OverallScore float64           `json:"overallScore"`
	OverallGrade Grade             `json:"overallGrade"`
	Dimensions   []*DimensionScore `json:"dimensions"`
	Trend        string            `json:"trend"` // improving, stable, declining
	LastChecked  time.Time         `json:"lastChecked"`
	NextCheck    time.Time         `json:"nextCheck"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Grade     Grade     `json:"grade"`
}

// HealthAlert 健康告警
type HealthAlert struct {
	ID        string    `json:"id"`
	Dimension DimensionType `json:"dimension"`
	Level     string    `json:"level"` // info, warning, critical
	Message   string    `json:"message"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
	Resolved  bool      `json:"resolved"`
}

// HealthStats 健康统计
type HealthStats struct {
	CurrentGrade    Grade  `json:"currentGrade"`
	CurrentScore    float64 `json:"currentScore"`
	Trend           string `json:"trend"`
	TotalChecks     int    `json:"totalChecks"`
	AlertCount      int    `json:"alertCount"`
	UnresolvedAlerts int   `json:"unresolvedAlerts"`
	BestScore       float64 `json:"bestScore"`
	WorstScore      float64 `json:"worstScore"`
	AvgScore        float64 `json:"avgScore"`
}
