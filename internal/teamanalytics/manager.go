// Package teamanalytics 提供团队效能分析核心管理逻辑
package teamanalytics

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 团队效能分析管理器
type Manager struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	metrics map[string][]*DORAMetrics // teamID -> metrics
	goals   map[string]*Goal
	teams   map[string]*TeamPerformance
}

// NewManager 创建团队效能分析管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Manager{
		logger:  logger,
		metrics: make(map[string][]*DORAMetrics),
		goals:   make(map[string]*Goal),
		teams:   make(map[string]*TeamPerformance),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CalculateMetrics 计算 DORA 指标
func (m *Manager) CalculateMetrics(req *GetMetricsRequest) (*DORAMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 确定统计周期
	if req.Period == "" {
		req.Period = PeriodMonthly
	}
	if req.StartDate.IsZero() {
		req.StartDate = time.Now().AddDate(0, -1, 0)
	}
	if req.EndDate.IsZero() {
		req.EndDate = time.Now()
	}

	// 模拟计算（实际应从数据库/CI/CD系统获取数据）
	deploymentFreq := m.calculateDeploymentFrequency(req)
	leadTime := m.calculateLeadTime(req)
	mttr := m.calculateMTTR(req)
	changeFailure := m.calculateChangeFailureRate(req)

	overallLevel := m.calculateOverallLevel(deploymentFreq.Level, leadTime.Level, mttr.Level, changeFailure.Level)
	score := m.calculateScore(deploymentFreq, leadTime, mttr, changeFailure)

	metrics := &DORAMetrics{
		TeamID:              req.TeamID,
		Period:              req.Period,
		StartDate:           req.StartDate,
		EndDate:             req.EndDate,
		DeploymentFrequency: deploymentFreq,
		LeadTime:            leadTime,
		MTTR:                mttr,
		ChangeFailureRate:   changeFailure,
		OverallLevel:        overallLevel,
		Score:               score,
		GeneratedAt:         time.Now(),
	}

	// 存储指标
	m.metrics[req.TeamID] = append(m.metrics[req.TeamID], metrics)

	m.logger.Info("dora metrics calculated",
		zap.String("team_id", req.TeamID),
		zap.String("level", string(overallLevel)),
		zap.Float64("score", score))

	return metrics, nil
}

// calculateDeploymentFrequency 计算部署频率
func (m *Manager) calculateDeploymentFrequency(req *GetMetricsRequest) *DeploymentFrequency {
	// 模拟数据
	days := int(req.EndDate.Sub(req.StartDate).Hours() / 24)
	if days <= 0 {
		days = 1
	}
	count := days * 3 // 模拟每天3次部署
	dailyAvg := float64(count) / float64(days)

	level := DORALevelLow
	switch {
	case dailyAvg >= 1:
		level = DORALevelElite
	case dailyAvg >= 0.2:
		level = DORALevelHigh
	case dailyAvg >= 0.03:
		level = DORALevelMedium
	}

	return &DeploymentFrequency{
		Count:         count,
		DailyAverage:  dailyAvg,
		Level:         level,
		Trend:         TrendUp,
		PreviousCount: int(float64(count) * 0.85),
	}
}

// calculateLeadTime 计算变更前置时间
func (m *Manager) calculateLeadTime(req *GetMetricsRequest) *LeadTime {
	// 模拟数据
	avg := 4 * time.Hour
	median := 3 * time.Hour
	p90 := 12 * time.Hour

	level := DORALevelHigh
	if avg <= time.Hour {
		level = DORALevelElite
	} else if avg > 24*time.Hour {
		level = DORALevelMedium
	} else if avg > 168*time.Hour {
		level = DORALevelLow
	}

	return &LeadTime{
		Average:     avg,
		Median:      median,
		P90:         p90,
		Level:       level,
		Trend:       TrendDown,
		PreviousAvg: 5 * time.Hour,
	}
}

// calculateMTTR 计算平均恢复时间
func (m *Manager) calculateMTTR(req *GetMetricsRequest) *MTTR {
	// 模拟数据
	avg := 30 * time.Minute
	median := 20 * time.Minute
	p90 := time.Hour

	level := DORALevelElite
	if avg > time.Hour {
		level = DORALevelHigh
	}
	if avg > 24*time.Hour {
		level = DORALevelMedium
	}
	if avg > 168*time.Hour {
		level = DORALevelLow
	}

	return &MTTR{
		Average:     avg,
		Median:      median,
		P90:         p90,
		Level:       level,
		Trend:       TrendDown,
		PreviousAvg: 45 * time.Minute,
	}
}

// calculateChangeFailureRate 计算变更失败率
func (m *Manager) calculateChangeFailureRate(req *GetMetricsRequest) *ChangeFailureRate {
	// 模拟数据
	totalChanges := 150
	failedChanges := 12
	rate := float64(failedChanges) / float64(totalChanges) * 100

	level := DORALevelHigh
	if rate <= 5 {
		level = DORALevelElite
	} else if rate > 30 {
		level = DORALevelLow
	} else if rate > 15 {
		level = DORALevelMedium
	}

	return &ChangeFailureRate{
		TotalChanges:  totalChanges,
		FailedChanges: failedChanges,
		Rate:          rate,
		Level:         level,
		Trend:         TrendDown,
		PreviousRate:  10.5,
	}
}

// calculateOverallLevel 计算总体等级
func (m *Manager) calculateOverallLevel(levels ...DORALevel) DORALevel {
	levelScore := map[DORALevel]int{
		DORALevelElite:  4,
		DORALevelHigh:   3,
		DORALevelMedium: 2,
		DORALevelLow:    1,
	}

	total := 0
	for _, l := range levels {
		total += levelScore[l]
	}
	avg := float64(total) / float64(len(levels))

	switch {
	case avg >= 3.5:
		return DORALevelElite
	case avg >= 2.5:
		return DORALevelHigh
	case avg >= 1.5:
		return DORALevelMedium
	default:
		return DORALevelLow
	}
}

// calculateScore 计算综合得分
func (m *Manager) calculateScore(df *DeploymentFrequency, lt *LeadTime, mttr *MTTR, cfr *ChangeFailureRate) float64 {
	levelScore := map[DORALevel]float64{
		DORALevelElite:  100,
		DORALevelHigh:   75,
		DORALevelMedium: 50,
		DORALevelLow:    25,
	}

	score := levelScore[df.Level] + levelScore[lt.Level] + levelScore[mttr.Level] + levelScore[cfr.Level]
	return score / 4
}

// GetTrends 获取趋势数据
func (m *Manager) GetTrends(teamID, metric string, period MetricPeriod, startDate, endDate time.Time) (*TrendReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 生成模拟趋势数据点
	var dataPoints []TrendData
	days := int(endDate.Sub(startDate).Hours() / 24)
	if days <= 0 {
		days = 30
	}

	baseValue := 50.0
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i)
		// 模拟波动上升趋势
		noise := math.Sin(float64(i)*0.3)*10 + float64(i)*0.5
		value := baseValue + noise

		dataPoints = append(dataPoints, TrendData{
			Date:   date,
			Value:  math.Round(value*100) / 100,
			Target: 80,
		})
	}

	// 计算统计值
	var sum, min, max float64
	min = math.MaxFloat64
	for _, dp := range dataPoints {
		sum += dp.Value
		if dp.Value < min {
			min = dp.Value
		}
		if dp.Value > max {
			max = dp.Value
		}
	}
	avg := sum / float64(len(dataPoints))

	trend := TrendStable
	if len(dataPoints) >= 2 {
		last := dataPoints[len(dataPoints)-1].Value
		first := dataPoints[0].Value
		if last > first*1.1 {
			trend = TrendUp
		} else if last < first*0.9 {
			trend = TrendDown
		}
	}

	return &TrendReport{
		TeamID:     teamID,
		Metric:     metric,
		Period:     period,
		DataPoints: dataPoints,
		Trend:      trend,
		Average:    math.Round(avg*100) / 100,
		Min:        min,
		Max:        max,
	}, nil
}

// GenerateReport 生成团队效能报告
func (m *Manager) GenerateReport(req *GenerateReportRequest) (*PerformanceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算指标
	metricsReq := &GetMetricsRequest{
		TeamID:    req.TeamID,
		Period:    req.Period,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	// 解锁后重新加锁计算
	m.mu.RUnlock()
	dora, err := m.CalculateMetrics(metricsReq)
	if err != nil {
		m.mu.RLock()
		return nil, err
	}
	m.mu.RLock()

	// 生成趋势报告
	trends := make([]*TrendReport, 0)
	metricNames := []string{"deployment_frequency", "lead_time", "mttr", "change_failure_rate"}
	for _, name := range metricNames {
		trend, _ := m.GetTrends(req.TeamID, name, req.Period, req.StartDate, req.EndDate)
		if trend != nil {
			trends = append(trends, trend)
		}
	}

	// 生成摘要和建议
	highlights := m.generateHighlights(dora)
	recommendations := m.generateRecommendations(dora)
	summary := m.generateSummary(dora)

	report := &PerformanceReport{
		TeamID:          req.TeamID,
		TeamName:        fmt.Sprintf("Team %s", req.TeamID),
		Period:          req.Period,
		StartDate:       req.StartDate,
		EndDate:         req.EndDate,
		Summary:         summary,
		DORA:            dora,
		Trends:          trends,
		Highlights:      highlights,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}

	m.logger.Info("performance report generated",
		zap.String("team_id", req.TeamID),
		zap.String("overall_level", string(dora.OverallLevel)))

	return report, nil
}

// generateSummary 生成报告摘要
func (m *Manager) generateSummary(dora *DORAMetrics) string {
	switch dora.OverallLevel {
	case DORALevelElite:
		return fmt.Sprintf("团队 %s 表现出色，DORA 综合评级为 Elite（%.1f分），所有指标均达到行业顶尖水平。", dora.TeamID, dora.Score)
	case DORALevelHigh:
		return fmt.Sprintf("团队 %s 表现良好，DORA 综合评级为 High（%.1f分），大部分指标处于行业领先水平。", dora.TeamID, dora.Score)
	case DORALevelMedium:
		return fmt.Sprintf("团队 %s 表现中等，DORA 综合评级为 Medium（%.1f分），有改进空间。", dora.TeamID, dora.Score)
	default:
		return fmt.Sprintf("团队 %s 有较大改进空间，DORA 综合评级为 Low（%.1f分），建议重点关注以下改进项。", dora.TeamID, dora.Score)
	}
}

// generateHighlights 生成亮点
func (m *Manager) generateHighlights(dora *DORAMetrics) []string {
	highlights := make([]string, 0)

	if dora.DeploymentFrequency.Level == DORALevelElite || dora.DeploymentFrequency.Level == DORALevelHigh {
		highlights = append(highlights, fmt.Sprintf("部署频率表现优秀，日均 %.1f 次部署", dora.DeploymentFrequency.DailyAverage))
	}
	if dora.MTTR.Level == DORALevelElite || dora.MTTR.Level == DORALevelHigh {
		highlights = append(highlights, fmt.Sprintf("故障恢复迅速，平均恢复时间 %v", dora.MTTR.Average))
	}
	if dora.ChangeFailureRate.Rate < 10 {
		highlights = append(highlights, fmt.Sprintf("变更失败率较低，仅为 %.1f%%", dora.ChangeFailureRate.Rate))
	}

	if len(highlights) == 0 {
		highlights = append(highlights, "团队正在持续改进中")
	}

	return highlights
}

// generateRecommendations 生成改进建议
func (m *Manager) generateRecommendations(dora *DORAMetrics) []string {
	recommendations := make([]string, 0)

	if dora.DeploymentFrequency.Level == DORALevelLow || dora.DeploymentFrequency.Level == DORALevelMedium {
		recommendations = append(recommendations, "建议优化 CI/CD 流程，提高部署自动化程度，增加部署频率")
	}
	if dora.LeadTime.Level == DORALevelLow || dora.LeadTime.Level == DORALevelMedium {
		recommendations = append(recommendations, "建议缩短变更前置时间，优化代码审查流程，减少等待时间")
	}
	if dora.MTTR.Level == DORALevelLow || dora.MTTR.Level == DORALevelMedium {
		recommendations = append(recommendations, "建议完善监控告警机制，建立故障响应 SOP，缩短平均恢复时间")
	}
	if dora.ChangeFailureRate.Level == DORALevelLow || dora.ChangeFailureRate.Level == DORALevelMedium {
		recommendations = append(recommendations, "建议加强测试覆盖率，实施渐进式发布策略，降低变更失败率")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "继续保持当前优秀表现，探索更高层次的工程卓越实践")
	}

	return recommendations
}

// SetGoals 设置团队目标
func (m *Manager) SetGoals(req *SetGoalRequest) (*Goal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证指标类型
	validMetrics := map[string]bool{
		"deployment_frequency": true,
		"lead_time":            true,
		"mttr":                 true,
		"change_failure_rate":  true,
		"test_coverage":        true,
		"code_review_coverage": true,
	}
	if !validMetrics[req.Metric] {
		return nil, fmt.Errorf("invalid metric: %s", req.Metric)
	}

	goal := &Goal{
		ID:           generateID(),
		TeamID:       req.TeamID,
		Metric:       req.Metric,
		TargetValue:  req.TargetValue,
		CurrentValue: 0,
		Unit:         req.Unit,
		Deadline:     req.Deadline,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.goals[goal.ID] = goal

	m.logger.Info("goal set",
		zap.String("goal_id", goal.ID),
		zap.String("team_id", req.TeamID),
		zap.String("metric", req.Metric))

	return goal, nil
}

// GetGoals 获取团队目标
func (m *Manager) GetGoals(teamID string) []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	goals := make([]*Goal, 0)
	for _, g := range m.goals {
		if teamID == "" || g.TeamID == teamID {
			goals = append(goals, g)
		}
	}

	sort.Slice(goals, func(i, j int) bool {
		return goals[i].Deadline.Before(goals[j].Deadline)
	})

	return goals
}

// UpdateGoalProgress 更新目标进度
func (m *Manager) UpdateGoalProgress(goalID string, currentValue float64) (*Goal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal not found: %s", goalID)
	}

	goal.CurrentValue = currentValue
	goal.UpdatedAt = time.Now()

	// 检查是否达标
	if currentValue >= goal.TargetValue {
		goal.Status = "achieved"
	} else if time.Now().After(goal.Deadline) {
		goal.Status = "missed"
	}

	return goal, nil
}

// DeleteGoal 删除目标
func (m *Manager) DeleteGoal(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.goals[id]; !ok {
		return fmt.Errorf("goal not found: %s", id)
	}

	delete(m.goals, id)
	return nil
}

// GetTeamPerformance 获取团队综合表现
func (m *Manager) GetTeamPerformance(teamID string) (*TeamPerformance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算 DORA 指标
	metricsReq := &GetMetricsRequest{
		TeamID:    teamID,
		Period:    PeriodMonthly,
		StartDate: time.Now().AddDate(0, -1, 0),
		EndDate:   time.Now(),
	}

	metrics, err := m.calculateDORAMetrics(metricsReq)
	if err != nil {
		return nil, err
	}

	performance := &TeamPerformance{
		TeamID:      teamID,
		TeamName:    fmt.Sprintf("Team %s", teamID),
		MemberCount: 8,
		DORA:        metrics,
		Throughput: &Throughput{
			TasksCompleted:   156,
			StoryPointsDone:  312,
			AverageCycleTime: 2 * 24 * time.Hour,
			AverageWaitTime:  4 * time.Hour,
			Trend:            TrendUp,
		},
		Quality: &Quality{
			BugCount:           12,
			BugRate:            0.08,
			CodeReviewCoverage: 95.5,
			TestCoverage:       82.3,
			Trend:              TrendUp,
		},
		Collaboration: &Collaboration{
			CodeReviewTurnaround: 2 * time.Hour,
			PRMergeRate:          88.5,
			CrossTeamPRs:         15,
			Trend:                TrendStable,
		},
		HealthScore: metrics.Score,
		GeneratedAt: time.Now(),
	}

	m.teams[teamID] = performance

	return performance, nil
}

// calculateDORAMetrics 内部计算方法
func (m *Manager) calculateDORAMetrics(req *GetMetricsRequest) (*DORAMetrics, error) {
	if req.Period == "" {
		req.Period = PeriodMonthly
	}

	deploymentFreq := m.calculateDeploymentFrequency(req)
	leadTime := m.calculateLeadTime(req)
	mttr := m.calculateMTTR(req)
	changeFailure := m.calculateChangeFailureRate(req)

	overallLevel := m.calculateOverallLevel(deploymentFreq.Level, leadTime.Level, mttr.Level, changeFailure.Level)
	score := m.calculateScore(deploymentFreq, leadTime, mttr, changeFailure)

	return &DORAMetrics{
		TeamID:              req.TeamID,
		Period:              req.Period,
		StartDate:           req.StartDate,
		EndDate:             req.EndDate,
		DeploymentFrequency: deploymentFreq,
		LeadTime:            leadTime,
		MTTR:                mttr,
		ChangeFailureRate:   changeFailure,
		OverallLevel:        overallLevel,
		Score:               score,
		GeneratedAt:         time.Now(),
	}, nil
}
