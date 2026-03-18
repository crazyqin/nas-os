// Package project provides milestone tracking functionality
package project

import (
	"math"
	"sync"
	"time"
)

// MilestoneHealthStatus 里程碑健康状态
type MilestoneHealthStatus string

// 里程碑健康状态常量
const (
	HealthStatusHealthy    MilestoneHealthStatus = "healthy"     // 按计划进行
	HealthStatusAtRisk     MilestoneHealthStatus = "at_risk"     // 有风险
	HealthStatusCritical   MilestoneHealthStatus = "critical"    // 严重风险
	HealthStatusOnTrack    MilestoneHealthStatus = "on_track"    // 超前进度
	HealthStatusCompleted  MilestoneHealthStatus = "completed"   // 已完成
	HealthStatusOverdue    MilestoneHealthStatus = "overdue"     // 已逾期
	HealthStatusNotStarted MilestoneHealthStatus = "not_started" // 未开始
)

// MilestoneRiskLevel 风险等级
type MilestoneRiskLevel string

// 风险等级常量
const (
	RiskLevelLow      MilestoneRiskLevel = "low"
	RiskLevelMedium   MilestoneRiskLevel = "medium"
	RiskLevelHigh     MilestoneRiskLevel = "high"
	RiskLevelCritical MilestoneRiskLevel = "critical"
)

// MilestoneHealthReport 里程碑健康报告
type MilestoneHealthReport struct {
	MilestoneID      string                `json:"milestone_id"`
	HealthStatus     MilestoneHealthStatus `json:"health_status"`
	RiskLevel        MilestoneRiskLevel    `json:"risk_level"`
	Progress         int                   `json:"progress"`
	ExpectedProgress int                   `json:"expected_progress"`
	Deviation        int                   `json:"deviation"` // 进度偏差百分比
	ForecastDate     *time.Time            `json:"forecast_date,omitempty"`
	DaysRemaining    int                   `json:"days_remaining"`
	DaysOverdue      int                   `json:"days_overdue"`
	RiskFactors      []RiskFactor          `json:"risk_factors"`
	Recommendations  []string              `json:"recommendations"`
	Bottlenecks      []BottleneckInfo      `json:"bottlenecks"`
	LastUpdated      time.Time             `json:"last_updated"`
}

// RiskFactor 风险因素
type RiskFactor struct {
	Type        string `json:"type"`     // schedule, resource, dependency, scope
	Severity    string `json:"severity"` // low, medium, high, critical
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation,omitempty"`
}

// BottleneckInfo 瓶颈信息
type BottleneckInfo struct {
	Type        string   `json:"type"`         // assignee, task, dependency
	ID          string   `json:"id"`           // 相关 ID
	Name        string   `json:"name"`         // 名称
	BlockedTask []string `json:"blocked_task"` // 被阻塞的任务
	Impact      int      `json:"impact"`       // 影响任务数
}

// MilestoneTracker 里程碑追踪器
type MilestoneTracker struct {
	mu       sync.RWMutex
	manager  *Manager
	report   map[string]*MilestoneHealthReport // milestoneID -> report
	notifier MilestoneNotifier
}

// MilestoneNotifier 里程碑通知接口
type MilestoneNotifier interface {
	NotifyHealthChange(milestoneID string, oldStatus, newStatus MilestoneHealthStatus)
	NotifyRiskAlert(milestoneID string, risk RiskFactor)
	NotifyBottleneck(milestoneID string, bottleneck BottleneckInfo)
}

// NewMilestoneTracker 创建里程碑追踪器
func NewMilestoneTracker(mgr *Manager, notifier MilestoneNotifier) *MilestoneTracker {
	return &MilestoneTracker{
		manager:  mgr,
		report:   make(map[string]*MilestoneHealthReport),
		notifier: notifier,
	}
}

// AnalyzeMilestone 分析里程碑健康状态
func (mt *MilestoneTracker) AnalyzeMilestone(milestoneID string) (*MilestoneHealthReport, error) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	milestone, err := mt.manager.GetMilestone(milestoneID)
	if err != nil {
		return nil, err
	}

	report := &MilestoneHealthReport{
		MilestoneID:     milestoneID,
		RiskFactors:     make([]RiskFactor, 0),
		Recommendations: make([]string, 0),
		Bottlenecks:     make([]BottleneckInfo, 0),
		LastUpdated:     time.Now(),
	}

	// 获取任务统计
	mm := NewMilestoneManager(mt.manager)
	stats := mm.GetMilestoneStats(milestoneID)
	report.Progress = stats.Progress

	// 计算预期进度
	now := time.Now()
	if milestone.DueDate != nil {
		totalDuration := milestone.DueDate.Sub(milestone.CreatedAt)
		elapsed := now.Sub(milestone.CreatedAt)

		if elapsed.Seconds() > 0 && totalDuration.Seconds() > 0 {
			report.ExpectedProgress = int(math.Min(100, (elapsed.Seconds()/totalDuration.Seconds())*100))
		}

		// 计算剩余天数
		if now.Before(*milestone.DueDate) {
			report.DaysRemaining = int(milestone.DueDate.Sub(now).Hours() / 24)
		} else {
			report.DaysOverdue = int(now.Sub(*milestone.DueDate).Hours() / 24)
		}
	}

	// 计算进度偏差
	report.Deviation = report.Progress - report.ExpectedProgress

	// 确定健康状态
	report.HealthStatus = mt.determineHealthStatus(milestone, report, stats)
	report.RiskLevel = mt.determineRiskLevel(report)

	// 分析风险因素
	mt.analyzeRiskFactors(milestone, report, stats)

	// 分析瓶颈
	mt.analyzeBottlenecks(milestoneID, report)

	// 生成建议
	mt.generateRecommendations(report)

	// 预测完成日期
	report.ForecastDate = mt.forecastCompletionDate(milestone, stats)

	// 存储报告
	oldReport, exists := mt.report[milestoneID]
	mt.report[milestoneID] = report

	// 触发通知
	if mt.notifier != nil && exists && oldReport.HealthStatus != report.HealthStatus {
		mt.notifier.NotifyHealthChange(milestoneID, oldReport.HealthStatus, report.HealthStatus)
	}

	// 风险预警通知
	if mt.notifier != nil {
		for _, risk := range report.RiskFactors {
			if risk.Severity == "high" || risk.Severity == "critical" {
				mt.notifier.NotifyRiskAlert(milestoneID, risk)
			}
		}
	}

	return report, nil
}

// determineHealthStatus 确定健康状态
func (mt *MilestoneTracker) determineHealthStatus(milestone *Milestone, report *MilestoneHealthReport, stats MilestoneStats) MilestoneHealthStatus {
	// 已完成
	if milestone.Status == string(MilestoneStatusCompleted) {
		return HealthStatusCompleted
	}

	// 已逾期
	if report.DaysOverdue > 0 {
		return HealthStatusOverdue
	}

	// 未开始
	if milestone.Status == string(MilestoneStatusPlanned) {
		return HealthStatusNotStarted
	}

	// 根据进度偏差判断
	switch {
	case report.Deviation >= 10:
		return HealthStatusOnTrack
	case report.Deviation >= -10:
		return HealthStatusHealthy
	case report.Deviation >= -25:
		return HealthStatusAtRisk
	default:
		return HealthStatusCritical
	}
}

// determineRiskLevel 确定风险等级
func (mt *MilestoneTracker) determineRiskLevel(report *MilestoneHealthReport) MilestoneRiskLevel {
	switch {
	case report.DaysOverdue > 7 || report.Deviation < -40:
		return RiskLevelCritical
	case report.DaysOverdue > 0 || report.Deviation < -25:
		return RiskLevelHigh
	case report.Deviation < -10 || len(report.Bottlenecks) > 2:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// analyzeRiskFactors 分析风险因素
func (mt *MilestoneTracker) analyzeRiskFactors(milestone *Milestone, report *MilestoneHealthReport, stats MilestoneStats) {
	mm := NewMilestoneManager(mt.manager)
	// 1. 进度风险
	if report.Deviation < -20 {
		severity := "high"
		if report.Deviation < -40 {
			severity = "critical"
		}
		report.RiskFactors = append(report.RiskFactors, RiskFactor{
			Type:        "schedule",
			Severity:    severity,
			Description: "里程碑进度严重滞后",
			Impact:      "可能无法按时完成",
			Mitigation:  "考虑增加资源或调整范围",
		})
	}

	// 2. 逾期风险
	if report.DaysRemaining < 7 && stats.IncompleteTasks > 0 {
		report.RiskFactors = append(report.RiskFactors, RiskFactor{
			Type:        "schedule",
			Severity:    "high",
			Description: "距离截止日期不足一周，仍有未完成任务",
			Impact:      "极有可能逾期",
			Mitigation:  "立即处理关键任务或申请延期",
		})
	}

	// 3. 资源风险
	// 统计各负责人的任务数
	assigneeCounts := make(map[string]int)
	for _, task := range mm.GetMilestoneTasks(milestone.ID, 10000, 0) {
		if task.AssigneeID != "" && task.Status != TaskStatusDone {
			assigneeCounts[task.AssigneeID]++
		}
	}
	overloadedUsers := 0
	for _, count := range assigneeCounts {
		if count > 5 {
			overloadedUsers++
		}
	}
	if overloadedUsers > 0 {
		report.RiskFactors = append(report.RiskFactors, RiskFactor{
			Type:        "resource",
			Severity:    "medium",
			Description: "部分成员任务过载",
			Impact:      "可能导致任务延迟",
			Mitigation:  "重新分配任务或增加人手",
		})
	}

	// 4. 过期任务风险
	if stats.OverdueTasks > 0 {
		severity := "medium"
		if stats.OverdueTasks > stats.TotalTasks/3 {
			severity = "high"
		}
		report.RiskFactors = append(report.RiskFactors, RiskFactor{
			Type:        "scope",
			Severity:    severity,
			Description: "存在过期未完成的任务",
			Impact:      "影响整体进度",
			Mitigation:  "优先处理过期任务",
		})
	}

	// 5. 依赖风险 (检查里程碑依赖)
	dm := NewDependencyManager()
	deps := dm.GetDependencies(milestone.ID)
	for _, dep := range deps {
		depMilestone, err := mt.manager.GetMilestone(dep.DependsOnID)
		if err == nil && depMilestone.Status != string(MilestoneStatusCompleted) {
			report.RiskFactors = append(report.RiskFactors, RiskFactor{
				Type:        "dependency",
				Severity:    "high",
				Description: "依赖的里程碑尚未完成: " + dep.DependsOnID,
				Impact:      "阻塞当前里程碑",
				Mitigation:  "协调依赖里程碑的完成",
			})
		}
	}
}

// analyzeBottlenecks 分析瓶颈
func (mt *MilestoneTracker) analyzeBottlenecks(milestoneID string, report *MilestoneHealthReport) {
	mm := NewMilestoneManager(mt.manager)
	tasks := mm.GetMilestoneTasks(milestoneID, 1000, 0)

	// 按负责人分析瓶颈
	assigneeTasks := make(map[string][]*Task)
	for _, task := range tasks {
		if task.AssigneeID != "" && task.Status != TaskStatusDone {
			assigneeTasks[task.AssigneeID] = append(assigneeTasks[task.AssigneeID], task)
		}
	}

	for assigneeID, tasks := range assigneeTasks {
		if len(tasks) > 5 {
			blockedTasks := make([]string, 0)
			for _, t := range tasks {
				blockedTasks = append(blockedTasks, t.ID)
			}
			report.Bottlenecks = append(report.Bottlenecks, BottleneckInfo{
				Type:        "assignee",
				ID:          assigneeID,
				Name:        "负责人: " + assigneeID,
				BlockedTask: blockedTasks,
				Impact:      len(tasks),
			})
		}
	}

	// 按任务状态分析瓶颈 (Review 状态积压)
	reviewTasks := make([]*Task, 0)
	for _, task := range tasks {
		if task.Status == TaskStatusReview {
			reviewTasks = append(reviewTasks, task)
		}
	}
	if len(reviewTasks) > 3 {
		blockedTasks := make([]string, 0)
		for _, t := range reviewTasks {
			blockedTasks = append(blockedTasks, t.ID)
		}
		report.Bottlenecks = append(report.Bottlenecks, BottleneckInfo{
			Type:        "task",
			ID:          "review_backlog",
			Name:        "审核任务积压",
			BlockedTask: blockedTasks,
			Impact:      len(reviewTasks),
		})
	}

	// 触发瓶颈通知
	if mt.notifier != nil {
		for _, bn := range report.Bottlenecks {
			mt.notifier.NotifyBottleneck(milestoneID, bn)
		}
	}
}

// generateRecommendations 生成建议
func (mt *MilestoneTracker) generateRecommendations(report *MilestoneHealthReport) {
	switch report.HealthStatus {
	case HealthStatusCritical:
		report.Recommendations = append(report.Recommendations,
			"⚠️ 里程碑处于严重风险状态，建议立即采取行动",
			"考虑召开紧急会议评估当前状况",
			"评估是否需要调整里程碑范围或截止日期",
		)
	case HealthStatusAtRisk:
		report.Recommendations = append(report.Recommendations,
			"里程碑有延期风险，建议加强监控",
			"优先处理高优先级任务",
			"检查资源分配是否合理",
		)
	case HealthStatusOverdue:
		report.Recommendations = append(report.Recommendations,
			"里程碑已逾期，请评估影响",
			"与相关方沟通调整预期",
			"制定追赶计划或重新规划",
		)
	}

	// 基于瓶颈的建议
	for _, bn := range report.Bottlenecks {
		switch bn.Type {
		case "assignee":
			report.Recommendations = append(report.Recommendations,
				"考虑重新分配 "+bn.ID+" 的部分任务")
		case "task":
			if bn.ID == "review_backlog" {
				report.Recommendations = append(report.Recommendations,
					"加快审核流程，减少任务积压")
			}
		}
	}

	// 基于风险的建议
	for _, risk := range report.RiskFactors {
		if risk.Mitigation != "" {
			report.Recommendations = append(report.Recommendations, risk.Mitigation)
		}
	}
}

// forecastCompletionDate 预测完成日期
func (mt *MilestoneTracker) forecastCompletionDate(milestone *Milestone, stats MilestoneStats) *time.Time {
	if stats.Progress >= 100 || stats.TotalTasks == 0 {
		return nil
	}

	// 基于历史速度预测
	now := time.Now()
	elapsed := now.Sub(milestone.CreatedAt).Hours() / 24 // 天数

	if elapsed <= 0 || stats.CompletedTasks == 0 {
		return milestone.DueDate // 无法预测，返回原定日期
	}

	// 计算完成速度 (任务/天)
	velocity := float64(stats.CompletedTasks) / elapsed
	if velocity <= 0 {
		return milestone.DueDate
	}

	// 预测剩余任务完成所需天数
	remainingTasks := float64(stats.TotalTasks - stats.CompletedTasks)
	daysNeeded := remainingTasks / velocity

	// 预测完成日期
	forecast := now.Add(time.Duration(daysNeeded*24) * time.Hour)
	return &forecast
}

// GetMilestoneHealthReport 获取里程碑健康报告
func (mt *MilestoneTracker) GetMilestoneHealthReport(milestoneID string) (*MilestoneHealthReport, error) {
	mt.mu.RLock()
	report, exists := mt.report[milestoneID]
	mt.mu.RUnlock()

	if exists {
		return report, nil
	}

	// 没有缓存，重新分析
	return mt.AnalyzeMilestone(milestoneID)
}

// GetAllMilestoneReports 获取所有里程碑报告
func (mt *MilestoneTracker) GetAllMilestoneReports(projectID string) ([]*MilestoneHealthReport, error) {
	milestones := mt.manager.ListMilestones(projectID)
	reports := make([]*MilestoneHealthReport, 0, len(milestones))

	for _, m := range milestones {
		report, err := mt.GetMilestoneHealthReport(m.ID)
		if err != nil {
			continue
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// GetAtRiskMilestones 获取有风险的里程碑
func (mt *MilestoneTracker) GetAtRiskMilestones(projectID string) ([]*MilestoneHealthReport, error) {
	reports, err := mt.GetAllMilestoneReports(projectID)
	if err != nil {
		return nil, err
	}

	atRisk := make([]*MilestoneHealthReport, 0)
	for _, r := range reports {
		if r.HealthStatus == HealthStatusAtRisk ||
			r.HealthStatus == HealthStatusCritical ||
			r.HealthStatus == HealthStatusOverdue {
			atRisk = append(atRisk, r)
		}
	}

	return atRisk, nil
}

// MilestoneTrend 里程碑趋势
type MilestoneTrend struct {
	MilestoneID  string       `json:"milestone_id"`
	TrendPoints  []TrendPoint `json:"trend_points"`
	Direction    string       `json:"direction"`    // improving, declining, stable
	Velocity     float64      `json:"velocity"`     // 进度变化速度
	Acceleration float64      `json:"acceleration"` // 加速度
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date     time.Time `json:"date"`
	Progress int       `json:"progress"`
}

// CalculateTrend 计算里程碑趋势
func (mt *MilestoneTracker) CalculateTrend(milestoneID string) (*MilestoneTrend, error) {
	mpt := NewMilestoneProgressTracker(mt.manager)
	records := mpt.GetProgressHistory(milestoneID)

	if len(records) < 2 {
		return nil, nil // 数据点不足
	}

	trend := &MilestoneTrend{
		MilestoneID: milestoneID,
		TrendPoints: make([]TrendPoint, 0, len(records)),
	}

	for _, r := range records {
		trend.TrendPoints = append(trend.TrendPoints, TrendPoint{
			Date:     r.Date,
			Progress: r.Progress,
		})
	}

	// 计算速度和加速度
	if len(records) >= 2 {
		last := records[len(records)-1]
		prev := records[len(records)-2]
		timeDiff := last.Date.Sub(prev.Date).Hours() / 24
		if timeDiff > 0 {
			trend.Velocity = float64(last.Progress-prev.Progress) / timeDiff
		}
	}

	if len(records) >= 3 {
		last := records[len(records)-1]
		prev := records[len(records)-2]
		prevPrev := records[len(records)-3]

		timeDiff1 := last.Date.Sub(prev.Date).Hours() / 24
		timeDiff2 := prev.Date.Sub(prevPrev.Date).Hours() / 24

		if timeDiff1 > 0 && timeDiff2 > 0 {
			vel1 := float64(last.Progress-prev.Progress) / timeDiff1
			vel2 := float64(prev.Progress-prevPrev.Progress) / timeDiff2
			trend.Acceleration = vel1 - vel2
		}
	}

	// 判断趋势方向
	switch {
	case trend.Acceleration > 1:
		trend.Direction = "improving"
	case trend.Acceleration < -1:
		trend.Direction = "declining"
	default:
		trend.Direction = "stable"
	}

	return trend, nil
}

// BatchAnalyzeMilestones 批量分析里程碑
func (mt *MilestoneTracker) BatchAnalyzeMilestones(projectID string) (map[string]*MilestoneHealthReport, error) {
	milestones := mt.manager.ListMilestones(projectID)
	results := make(map[string]*MilestoneHealthReport)

	for _, m := range milestones {
		report, err := mt.AnalyzeMilestone(m.ID)
		if err != nil {
			continue
		}
		results[m.ID] = report
	}

	return results, nil
}
