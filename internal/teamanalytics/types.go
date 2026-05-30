// Package teamanalytics 提供团队效能分析功能，基于 DORA 指标评估开发团队表现。
// 提供部署频率、变更前置时间、服务恢复时间、变更失败率等核心指标的计算和趋势分析。
package teamanalytics

import "time"

// DORALevel DORA 绩效等级
type DORALevel string

const (
	DORALevelElite    DORALevel = "elite"
	DORALevelHigh     DORALevel = "high"
	DORALevelMedium   DORALevel = "medium"
	DORALevelLow      DORALevel = "low"
)

// MetricPeriod 指标统计周期
type MetricPeriod string

const (
	PeriodDaily   MetricPeriod = "daily"
	PeriodWeekly  MetricPeriod = "weekly"
	PeriodMonthly MetricPeriod = "monthly"
	PeriodQuarterly MetricPeriod = "quarterly"
)

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendUp      TrendDirection = "up"
	TrendDown    TrendDirection = "down"
	TrendStable  TrendDirection = "stable"
)

// DORAMetrics DORA 四大核心指标
type DORAMetrics struct {
	TeamID                string              `json:"team_id"`
	Period                MetricPeriod        `json:"period"`
	StartDate             time.Time           `json:"start_date"`
	EndDate               time.Time           `json:"end_date"`
	DeploymentFrequency   *DeploymentFrequency `json:"deployment_frequency"`
	LeadTime              *LeadTime           `json:"lead_time"`
	MTTR                  *MTTR               `json:"mttr"`
	ChangeFailureRate     *ChangeFailureRate  `json:"change_failure_rate"`
	OverallLevel          DORALevel           `json:"overall_level"`
	Score                 float64             `json:"score"` // 0-100
	GeneratedAt           time.Time           `json:"generated_at"`
}

// DeploymentFrequency 部署频率
type DeploymentFrequency struct {
	Count        int           `json:"count"`
	DailyAverage float64       `json:"daily_average"`
	Level        DORALevel     `json:"level"`
	Trend        TrendDirection `json:"trend"`
	PreviousCount int          `json:"previous_count"`
}

// LeadTime 变更前置时间（从提交到部署）
type LeadTime struct {
	Average      time.Duration `json:"average"`
	Median       time.Duration `json:"median"`
	P90          time.Duration `json:"p90"`
	Level        DORALevel     `json:"level"`
	Trend        TrendDirection `json:"trend"`
	PreviousAvg  time.Duration `json:"previous_avg"`
}

// MTTR 平均恢复时间
type MTTR struct {
	Average      time.Duration `json:"average"`
	Median       time.Duration `json:"median"`
	P90          time.Duration `json:"p90"`
	Level        DORALevel     `json:"level"`
	Trend        TrendDirection `json:"trend"`
	PreviousAvg  time.Duration `json:"previous_avg"`
}

// ChangeFailureRate 变更失败率
type ChangeFailureRate struct {
	TotalChanges   int           `json:"total_changes"`
	FailedChanges  int           `json:"failed_changes"`
	Rate           float64       `json:"rate"` // 百分比 0-100
	Level          DORALevel     `json:"level"`
	Trend          TrendDirection `json:"trend"`
	PreviousRate   float64       `json:"previous_rate"`
}

// TeamPerformance 团队综合表现
type TeamPerformance struct {
	TeamID          string        `json:"team_id"`
	TeamName        string        `json:"team_name"`
	MemberCount     int           `json:"member_count"`
	DORA            *DORAMetrics  `json:"dora"`
	Throughput      *Throughput   `json:"throughput"`
	Quality         *Quality      `json:"quality"`
	Collaboration   *Collaboration `json:"collaboration"`
	HealthScore     float64       `json:"health_score"` // 0-100
	GeneratedAt     time.Time     `json:"generated_at"`
}

// Throughput 吞吐量指标
type Throughput struct {
	TasksCompleted    int           `json:"tasks_completed"`
	StoryPointsDone   int           `json:"story_points_done"`
	AverageCycleTime  time.Duration `json:"average_cycle_time"`
	AverageWaitTime   time.Duration `json:"average_wait_time"`
	Trend             TrendDirection `json:"trend"`
}

// Quality 质量指标
type Quality struct {
	BugCount           int           `json:"bug_count"`
	BugRate            float64       `json:"bug_rate"` // bugs per feature
	CodeReviewCoverage float64       `json:"code_review_coverage"`
	TestCoverage       float64       `json:"test_coverage"`
	Trend              TrendDirection `json:"trend"`
}

// Collaboration 协作指标
type Collaboration struct {
	CodeReviewTurnaround time.Duration `json:"code_review_turnaround"`
	PRMergeRate          float64       `json:"pr_merge_rate"`
	CrossTeamPRs         int           `json:"cross_team_prs"`
	Trend                TrendDirection `json:"trend"`
}

// TrendData 趋势数据点
type TrendData struct {
	Date   time.Time `json:"date"`
	Value  float64   `json:"value"`
	Target float64   `json:"target,omitempty"`
}

// TrendReport 趋势报告
type TrendReport struct {
	TeamID    string       `json:"team_id"`
	Metric    string       `json:"metric"`
	Period    MetricPeriod `json:"period"`
	DataPoints []TrendData `json:"data_points"`
	Trend     TrendDirection `json:"trend"`
	Average   float64      `json:"average"`
	Min       float64      `json:"min"`
	Max       float64      `json:"max"`
}

// PerformanceReport 团队效能报告
type PerformanceReport struct {
	TeamID        string         `json:"team_id"`
	TeamName      string         `json:"team_name"`
	Period        MetricPeriod   `json:"period"`
	StartDate     time.Time      `json:"start_date"`
	EndDate       time.Time      `json:"end_date"`
	Summary       string         `json:"summary"`
	DORA          *DORAMetrics   `json:"dora"`
	Trends        []*TrendReport `json:"trends"`
	Highlights    []string       `json:"highlights"`
	Recommendations []string     `json:"recommendations"`
	GeneratedAt   time.Time      `json:"generated_at"`
}

// Goal 绩效目标
type Goal struct {
	ID          string       `json:"id"`
	TeamID      string       `json:"team_id"`
	Metric      string       `json:"metric" binding:"required"`
	TargetValue float64      `json:"target_value" binding:"required"`
	CurrentValue float64     `json:"current_value"`
	Unit        string       `json:"unit"`
	Deadline    time.Time    `json:"deadline" binding:"required"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// SetGoalRequest 设置目标请求
type SetGoalRequest struct {
	TeamID      string    `json:"team_id" binding:"required"`
	Metric      string    `json:"metric" binding:"required"`
	TargetValue float64  `json:"target_value" binding:"required"`
	Unit        string    `json:"unit"`
	Deadline    time.Time `json:"deadline" binding:"required"`
}

// GetMetricsRequest 获取指标请求
type GetMetricsRequest struct {
	TeamID    string       `json:"team_id" binding:"required"`
	Period    MetricPeriod `json:"period"`
	StartDate time.Time    `json:"start_date"`
	EndDate   time.Time    `json:"end_date"`
}

// GenerateReportRequest 生成报告请求
type GenerateReportRequest struct {
	TeamID    string       `json:"team_id" binding:"required"`
	Period    MetricPeriod `json:"period"`
	StartDate time.Time    `json:"start_date"`
	EndDate   time.Time    `json:"end_date"`
}
