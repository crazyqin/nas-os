// Package project provides project statistics functionality
package project

import (
	"time"
)

// StatsExtended 扩展项目统计
type StatsExtended struct {
	// 基础统计
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`

	// 任务统计
	TaskStats TaskStatsDetail `json:"task_stats"`

	// 时间统计
	TimeStats TimeStats `json:"time_stats"`

	// 里程碑统计
	MilestoneStats MilestoneStatsDetail `json:"milestone_stats"`

	// 团队统计
	TeamStats TeamStats `json:"team_stats"`

	// 进度统计
	ProgressStats ProgressStats `json:"progress_stats"`

	// 质量指标
	QualityMetrics QualityMetrics `json:"quality_metrics"`

	// 风险指标
	RiskIndicators RiskIndicators `json:"risk_indicators"`
}

// ProjectStatsExtended 是 StatsExtended 的别名，保持向后兼容
type ProjectStatsExtended = StatsExtended

// TaskStatsDetail 详细任务统计
type TaskStatsDetail struct {
	Total       int            `json:"total"`
	ByStatus    map[string]int `json:"by_status"`
	ByPriority  map[string]int `json:"by_priority"`
	ByAssignee  map[string]int `json:"by_assignee"`
	ByMilestone map[string]int `json:"by_milestone"`

	// 完成统计
	Completed      int     `json:"completed"`
	CompletionRate float64 `json:"completion_rate"`

	// 过期统计
	Overdue      int `json:"overdue"`
	DueToday     int `json:"due_today"`
	DueThisWeek  int `json:"due_this_week"`
	DueThisMonth int `json:"due_this_month"`

	// 本周统计
	CreatedThisWeek   int `json:"created_this_week"`
	CompletedThisWeek int `json:"completed_this_week"`

	// 本月统计
	CreatedThisMonth   int `json:"created_this_month"`
	CompletedThisMonth int `json:"completed_this_month"`

	// 平均数据
	AverageCompletionTime float64 `json:"average_completion_time"` // 小时
	AverageTasksPerMember float64 `json:"average_tasks_per_member"`
}

// TimeStats 时间统计
type TimeStats struct {
	// 工时统计
	TotalEstimatedHours float64 `json:"total_estimated_hours"`
	TotalActualHours    float64 `json:"total_actual_hours"`
	HoursVariance       float64 `json:"hours_variance"`
	HoursVarianceRate   float64 `json:"hours_variance_rate"` // 百分比

	// 按成员统计
	MemberHours map[string]MemberHours `json:"member_hours"`

	// 按里程碑统计
	MilestoneHours map[string]float64 `json:"milestone_hours"`

	// 时间趋势
	HoursTrend []HoursTrendPoint `json:"hours_trend,omitempty"`
}

// MemberHours 成员工时
type MemberHours struct {
	UserID          string  `json:"user_id"`
	EstimatedHours  float64 `json:"estimated_hours"`
	ActualHours     float64 `json:"actual_hours"`
	TasksCompleted  int     `json:"tasks_completed"`
	AvgHoursPerTask float64 `json:"avg_hours_per_task"`
}

// HoursTrendPoint 工时趋势点
type HoursTrendPoint struct {
	Date    string  `json:"date"`
	Logged  float64 `json:"logged"`
	Planned float64 `json:"planned"`
}

// MilestoneStatsDetail 详细里程碑统计
type MilestoneStatsDetail struct {
	Total          int     `json:"total"`
	Completed      int     `json:"completed"`
	Active         int     `json:"active"`
	Overdue        int     `json:"overdue"`
	CompletionRate float64 `json:"completion_rate"`

	// 平均完成时间
	AvgCompletionDays float64 `json:"avg_completion_days"`

	// 按状态统计
	ByStatus map[string]int `json:"by_status"`

	// 进度分布
	ProgressDistribution ProgressDistribution `json:"progress_distribution"`
}

// ProgressDistribution 进度分布
type ProgressDistribution struct {
	ZeroTo25         int `json:"0_to_25"`   // 0-25%
	TwentyFiveTo50   int `json:"25_to_50"`  // 25-50%
	FiftyTo75        int `json:"50_to_75"`  // 50-75%
	SeventyFiveTo100 int `json:"75_to_100"` // 75-100%
	Completed        int `json:"completed"` // 100%
}

// TeamStats 团队统计
type TeamStats struct {
	TeamSize        int                    `json:"team_size"`
	MemberWorkloads []MemberWorkloadDetail `json:"member_workloads"`
	TopPerformers   []TopPerformer         `json:"top_performers"`
	WorkloadBalance float64                `json:"workload_balance"` // 0-1，1表示完美平衡
}

// MemberWorkloadDetail 详细成员工作量
type MemberWorkloadDetail struct {
	UserID          string  `json:"user_id"`
	TotalTasks      int     `json:"total_tasks"`
	TodoTasks       int     `json:"todo_tasks"`
	InProgressTasks int     `json:"in_progress_tasks"`
	DoneTasks       int     `json:"done_tasks"`
	OverdueTasks    int     `json:"overdue_tasks"`
	WorkloadScore   float64 `json:"workload_score"` // 0-100
	EstimatedHours  float64 `json:"estimated_hours"`
	ActualHours     float64 `json:"actual_hours"`
	CompletionRate  float64 `json:"completion_rate"`
}

// TopPerformer 表现最好的成员
type TopPerformer struct {
	UserID         string  `json:"user_id"`
	TasksCompleted int     `json:"tasks_completed"`
	CompletionRate float64 `json:"completion_rate"`
	OnTimeRate     float64 `json:"on_time_rate"`
}

// ProgressStats 进度统计
type ProgressStats struct {
	OverallProgress     int        `json:"overall_progress"`  // 0-100
	ExpectedProgress    int        `json:"expected_progress"` // 基于时间计算的预期进度
	ProgressDelta       int        `json:"progress_delta"`    // 实际-预期
	OnTrack             bool       `json:"on_track"`
	DaysRemaining       int        `json:"days_remaining"`
	EstimatedCompletion *time.Time `json:"estimated_completion,omitempty"`

	// 燃尽图数据
	BurndownData []BurndownPoint `json:"burndown_data,omitempty"`
}

// BurndownPoint 燃尽图数据点
type BurndownPoint struct {
	Date      string `json:"date"`
	Remaining int    `json:"remaining"`
	Ideal     int    `json:"ideal"`
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	// 任务完成质量
	OnTimeCompletionRate float64 `json:"on_time_completion_rate"` // 按时完成率
	ReopenRate           float64 `json:"reopen_rate"`             // 重开率
	BugRate              float64 `json:"bug_rate"`                // Bug任务占比

	// 工时准确度
	EstimationAccuracy float64 `json:"estimation_accuracy"` // 估算准确度（0-100）

	// 效率指标
	TasksPerDay  float64 `json:"tasks_per_day"`  // 每日完成任务数
	AvgCycleTime float64 `json:"avg_cycle_time"` // 平均周期时间（小时）
	AvgLeadTime  float64 `json:"avg_lead_time"`  // 平均交付时间（小时）
}

// RiskIndicators 风险指标
type RiskIndicators struct {
	OverallRisk            string   `json:"overall_risk"` // low, medium, high, critical
	RiskScore              int      `json:"risk_score"`   // 0-100
	RiskFactors            []string `json:"risk_factors"`
	OverdueTasksRatio      float64  `json:"overdue_tasks_ratio"`
	BlockedTasksRatio      float64  `json:"blocked_tasks_ratio"`
	OverloadedMembers      int      `json:"overloaded_members"`
	NearDeadlineMilestones int      `json:"near_deadline_milestones"`
}

// StatsManager 统计管理器
type StatsManager struct {
	manager *Manager
}

// NewStatsManager 创建统计管理器
func NewStatsManager(mgr *Manager) *StatsManager {
	return &StatsManager{
		manager: mgr,
	}
}

// GetExtendedStats 获取扩展统计
func (sm *StatsManager) GetExtendedStats(projectID string) (*StatsExtended, error) {
	project, err := sm.manager.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	stats := &StatsExtended{
		ProjectID:   projectID,
		ProjectName: project.Name,
		TaskStats: TaskStatsDetail{
			ByStatus:    make(map[string]int),
			ByPriority:  make(map[string]int),
			ByAssignee:  make(map[string]int),
			ByMilestone: make(map[string]int),
		},
		MilestoneStats: MilestoneStatsDetail{
			ByStatus: make(map[string]int),
		},
		TimeStats: TimeStats{
			MemberHours:    make(map[string]MemberHours),
			MilestoneHours: make(map[string]float64),
		},
		TeamStats: TeamStats{
			MemberWorkloads: make([]MemberWorkloadDetail, 0),
			TopPerformers:   make([]TopPerformer, 0),
		},
	}

	// 计算任务统计
	sm.calculateTaskStats(stats, projectID)

	// 计算时间统计
	sm.calculateTimeStats(stats, projectID)

	// 计算里程碑统计
	sm.calculateMilestoneStats(stats, projectID)

	// 计算团队统计
	sm.calculateTeamStats(stats, projectID)

	// 计算进度统计
	sm.calculateProgressStats(stats, projectID, project)

	// 计算质量指标
	sm.calculateQualityMetrics(stats, projectID)

	// 计算风险指标
	sm.calculateRiskIndicators(stats, projectID)

	return stats, nil
}

// calculateTaskStats 计算任务统计
func (sm *StatsManager) calculateTaskStats(stats *StatsExtended, projectID string) {
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := today.Add(24 * time.Hour)
	weekEnd := today.AddDate(0, 0, 7)
	monthEnd := today.AddDate(0, 0, 30)

	var totalCompletionTime float64
	var completionCount int

	filter := TaskFilter{ProjectID: projectID, Limit: 10000}
	tasks := sm.manager.ListTasks(filter)

	for _, task := range tasks {
		stats.TaskStats.Total++

		// 按状态统计
		stats.TaskStats.ByStatus[string(task.Status)]++

		// 按优先级统计
		stats.TaskStats.ByPriority[string(task.Priority)]++

		// 按负责人统计
		if task.AssigneeID != "" {
			stats.TaskStats.ByAssignee[task.AssigneeID]++
		}

		// 按里程碑统计
		if task.MilestoneID != "" {
			stats.TaskStats.ByMilestone[task.MilestoneID]++
		}

		// 完成统计
		if task.Status == TaskStatusDone {
			stats.TaskStats.Completed++

			// 计算完成时间
			if task.CompletedAt != nil && task.CreatedAt.Before(*task.CompletedAt) {
				duration := task.CompletedAt.Sub(task.CreatedAt).Hours()
				totalCompletionTime += duration
				completionCount++
			}
		}

		// 过期统计
		if task.DueDate != nil && task.Status != TaskStatusDone {
			if task.DueDate.Before(now) {
				stats.TaskStats.Overdue++
			}
			if task.DueDate.After(today) && task.DueDate.Before(todayEnd) {
				stats.TaskStats.DueToday++
			}
			if task.DueDate.After(today) && task.DueDate.Before(weekEnd) {
				stats.TaskStats.DueThisWeek++
			}
			if task.DueDate.After(today) && task.DueDate.Before(monthEnd) {
				stats.TaskStats.DueThisMonth++
			}
		}

		// 时间统计
		if task.CreatedAt.After(weekAgo) {
			stats.TaskStats.CreatedThisWeek++
		}
		if task.CompletedAt != nil && task.CompletedAt.After(weekAgo) {
			stats.TaskStats.CompletedThisWeek++
		}
		if task.CreatedAt.After(monthAgo) {
			stats.TaskStats.CreatedThisMonth++
		}
		if task.CompletedAt != nil && task.CompletedAt.After(monthAgo) {
			stats.TaskStats.CompletedThisMonth++
		}
	}

	// 计算完成率
	if stats.TaskStats.Total > 0 {
		stats.TaskStats.CompletionRate = float64(stats.TaskStats.Completed) / float64(stats.TaskStats.Total) * 100
	}

	// 平均完成时间
	if completionCount > 0 {
		stats.TaskStats.AverageCompletionTime = totalCompletionTime / float64(completionCount)
	}

	// 平均每人任务数
	if len(stats.TaskStats.ByAssignee) > 0 {
		stats.TaskStats.AverageTasksPerMember = float64(stats.TaskStats.Total) / float64(len(stats.TaskStats.ByAssignee))
	}
}

// calculateTimeStats 计算时间统计
func (sm *StatsManager) calculateTimeStats(stats *StatsExtended, projectID string) {
	filter := TaskFilter{ProjectID: projectID, Limit: 10000}
	tasks := sm.manager.ListTasks(filter)

	for _, task := range tasks {
		stats.TimeStats.TotalEstimatedHours += task.EstimatedHours
		stats.TimeStats.TotalActualHours += task.ActualHours

		// 按成员统计
		if task.AssigneeID != "" {
			mh := stats.TimeStats.MemberHours[task.AssigneeID]
			mh.UserID = task.AssigneeID
			mh.EstimatedHours += task.EstimatedHours
			mh.ActualHours += task.ActualHours
			if task.Status == TaskStatusDone {
				mh.TasksCompleted++
			}
			stats.TimeStats.MemberHours[task.AssigneeID] = mh
		}

		// 按里程碑统计
		if task.MilestoneID != "" {
			stats.TimeStats.MilestoneHours[task.MilestoneID] += task.ActualHours
		}
	}

	// 计算工时差异
	stats.TimeStats.HoursVariance = stats.TimeStats.TotalActualHours - stats.TimeStats.TotalEstimatedHours
	if stats.TimeStats.TotalEstimatedHours > 0 {
		stats.TimeStats.HoursVarianceRate = stats.TimeStats.HoursVariance / stats.TimeStats.TotalEstimatedHours * 100
	}

	// 计算平均每任务工时
	for userID, mh := range stats.TimeStats.MemberHours {
		if mh.TasksCompleted > 0 {
			mh.AvgHoursPerTask = mh.ActualHours / float64(mh.TasksCompleted)
			stats.TimeStats.MemberHours[userID] = mh
		}
	}
}

// calculateMilestoneStats 计算里程碑统计
func (sm *StatsManager) calculateMilestoneStats(stats *StatsExtended, projectID string) {
	now := time.Now()
	milestones := sm.manager.ListMilestones(projectID)

	var totalCompletionDays float64
	var completionCount int

	for _, ms := range milestones {
		stats.MilestoneStats.Total++
		stats.MilestoneStats.ByStatus[ms.Status]++

		switch ms.Status {
		case "completed":
			stats.MilestoneStats.Completed++
			if ms.CompletedAt != nil {
				days := ms.CompletedAt.Sub(ms.CreatedAt).Hours() / 24
				totalCompletionDays += days
				completionCount++
			}
		case "active":
			stats.MilestoneStats.Active++
		}

		// 过期里程碑
		if ms.DueDate != nil && ms.DueDate.Before(now) && ms.Status != "completed" {
			stats.MilestoneStats.Overdue++
		}

		// 进度分布
		progress := ms.Progress
		switch {
		case progress == 100:
			stats.MilestoneStats.ProgressDistribution.Completed++
		case progress >= 75:
			stats.MilestoneStats.ProgressDistribution.SeventyFiveTo100++
		case progress >= 50:
			stats.MilestoneStats.ProgressDistribution.FiftyTo75++
		case progress >= 25:
			stats.MilestoneStats.ProgressDistribution.TwentyFiveTo50++
		default:
			stats.MilestoneStats.ProgressDistribution.ZeroTo25++
		}
	}

	// 完成率
	if stats.MilestoneStats.Total > 0 {
		stats.MilestoneStats.CompletionRate = float64(stats.MilestoneStats.Completed) / float64(stats.MilestoneStats.Total) * 100
	}

	// 平均完成天数
	if completionCount > 0 {
		stats.MilestoneStats.AvgCompletionDays = totalCompletionDays / float64(completionCount)
	}
}

// calculateTeamStats 计算团队统计
func (sm *StatsManager) calculateTeamStats(stats *StatsExtended, projectID string) {
	memberMap := make(map[string]*MemberWorkloadDetail)
	now := time.Now()

	filter := TaskFilter{ProjectID: projectID, Limit: 10000}
	tasks := sm.manager.ListTasks(filter)

	for _, task := range tasks {
		if task.AssigneeID == "" {
			continue
		}

		if memberMap[task.AssigneeID] == nil {
			memberMap[task.AssigneeID] = &MemberWorkloadDetail{
				UserID: task.AssigneeID,
			}
		}

		m := memberMap[task.AssigneeID]
		m.TotalTasks++

		switch task.Status {
		case TaskStatusTodo:
			m.TodoTasks++
		case TaskStatusInProgress:
			m.InProgressTasks++
		case TaskStatusDone:
			m.DoneTasks++
		}

		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			m.OverdueTasks++
		}

		m.EstimatedHours += task.EstimatedHours
		m.ActualHours += task.ActualHours
	}

	// 计算工作量和完成率
	for _, m := range memberMap {
		if m.TotalTasks > 0 {
			m.CompletionRate = float64(m.DoneTasks) / float64(m.TotalTasks) * 100
		}
		// 工作量评分（基于任务数和过期任务）
		m.WorkloadScore = float64(m.TotalTasks) - float64(m.OverdueTasks)*1.5
		if m.WorkloadScore < 0 {
			m.WorkloadScore = 0
		}
		stats.TeamStats.MemberWorkloads = append(stats.TeamStats.MemberWorkloads, *m)
	}

	stats.TeamStats.TeamSize = len(memberMap)

	// 计算工作量平衡度
	if len(memberMap) > 1 {
		var sum, sumSq float64
		for _, m := range memberMap {
			sum += float64(m.TotalTasks)
			sumSq += float64(m.TotalTasks) * float64(m.TotalTasks)
		}
		mean := sum / float64(len(memberMap))
		variance := sumSq/float64(len(memberMap)) - mean*mean
		// 标准差越小，平衡度越高
		if mean > 0 {
			stats.TeamStats.WorkloadBalance = 1.0 - min(variance/(mean*mean), 1.0)
		}
	} else {
		stats.TeamStats.WorkloadBalance = 1.0
	}

	// 找出表现最好的成员
	stats.TeamStats.TopPerformers = sm.findTopPerformers(memberMap)
}

// findTopPerformers 找出表现最好的成员
func (sm *StatsManager) findTopPerformers(memberMap map[string]*MemberWorkloadDetail) []TopPerformer {
	performers := make([]TopPerformer, 0, len(memberMap))

	for _, m := range memberMap {
		if m.DoneTasks > 0 {
			performers = append(performers, TopPerformer{
				UserID:         m.UserID,
				TasksCompleted: m.DoneTasks,
				CompletionRate: m.CompletionRate,
				OnTimeRate:     float64(m.DoneTasks-m.OverdueTasks) / float64(m.DoneTasks) * 100,
			})
		}
	}

	// 按完成数排序
	for i := 0; i < len(performers); i++ {
		for j := i + 1; j < len(performers); j++ {
			if performers[j].TasksCompleted > performers[i].TasksCompleted {
				performers[i], performers[j] = performers[j], performers[i]
			}
		}
	}

	// 返回前5名
	if len(performers) > 5 {
		performers = performers[:5]
	}

	return performers
}

// calculateProgressStats 计算进度统计
func (sm *StatsManager) calculateProgressStats(stats *StatsExtended, projectID string, project *Project) {
	// 整体进度
	if stats.TaskStats.Total > 0 {
		stats.ProgressStats.OverallProgress = int(float64(stats.TaskStats.Completed) / float64(stats.TaskStats.Total) * 100)
	}

	// 计算预期进度（基于项目开始和结束时间）
	if project.StartDtae != nil && project.EndDate != nil {
		now := time.Now()
		total := project.EndDate.Sub(*project.StartDtae).Hours()
		elapsed := now.Sub(*project.StartDtae).Hours()

		if total > 0 {
			stats.ProgressStats.ExpectedProgress = int(elapsed / total * 100)
			if stats.ProgressStats.ExpectedProgress > 100 {
				stats.ProgressStats.ExpectedProgress = 100
			}
		}

		stats.ProgressStats.DaysRemaining = int(project.EndDate.Sub(now).Hours() / 24)
		if stats.ProgressStats.DaysRemaining < 0 {
			stats.ProgressStats.DaysRemaining = 0
		}
	}

	// 进度差异
	stats.ProgressStats.ProgressDelta = stats.ProgressStats.OverallProgress - stats.ProgressStats.ExpectedProgress

	// 是否在轨道上
	stats.ProgressStats.OnTrack = stats.ProgressStats.ProgressDelta >= -10
}

// calculateQualityMetrics 计算质量指标
func (sm *StatsManager) calculateQualityMetrics(stats *StatsExtended, projectID string) {
	filter := TaskFilter{ProjectID: projectID, Limit: 10000}
	tasks := sm.manager.ListTasks(filter)

	var onTimeCount, totalCompleted, totalEstimated, totalActual float64
	var cycleTimes, leadTimes []float64

	for _, task := range tasks {
		totalEstimated += task.EstimatedHours
		totalActual += task.ActualHours

		if task.Status == TaskStatusDone {
			totalCompleted++

			// 按时完成
			if task.CompletedAt != nil && task.DueDate != nil {
				if task.CompletedAt.Before(*task.DueDate) || task.CompletedAt.Equal(*task.DueDate) {
					onTimeCount++
				}
			}

			// 周期时间（创建到完成）
			if task.CompletedAt != nil {
				cycleTime := task.CompletedAt.Sub(task.CreatedAt).Hours()
				cycleTimes = append(cycleTimes, cycleTime)
			}

			// 交付时间（开始到完成）
			if task.StartDate != nil && task.CompletedAt != nil {
				leadTime := task.CompletedAt.Sub(*task.StartDate).Hours()
				leadTimes = append(leadTimes, leadTime)
			}
		}
	}

	// 按时完成率
	if totalCompleted > 0 {
		stats.QualityMetrics.OnTimeCompletionRate = onTimeCount / totalCompleted * 100
	}

	// 估算准确度
	if totalEstimated > 0 {
		accuracy := 100.0 - abs(totalActual-totalEstimated)/totalEstimated*100
		if accuracy < 0 {
			accuracy = 0
		}
		stats.QualityMetrics.EstimationAccuracy = accuracy
	}

	// 平均周期时间
	if len(cycleTimes) > 0 {
		var sum float64
		for _, t := range cycleTimes {
			sum += t
		}
		stats.QualityMetrics.AvgCycleTime = sum / float64(len(cycleTimes))
	}

	// 平均交付时间
	if len(leadTimes) > 0 {
		var sum float64
		for _, t := range leadTimes {
			sum += t
		}
		stats.QualityMetrics.AvgLeadTime = sum / float64(len(leadTimes))
	}

	// 每日完成任务数
	if stats.TaskStats.CompletedThisWeek > 0 {
		stats.QualityMetrics.TasksPerDay = float64(stats.TaskStats.CompletedThisWeek) / 7.0
	}
}

// calculateRiskIndicators 计算风险指标
func (sm *StatsManager) calculateRiskIndicators(stats *StatsExtended, projectID string) {
	// 过期任务比例
	if stats.TaskStats.Total > 0 {
		stats.RiskIndicators.OverdueTasksRatio = float64(stats.TaskStats.Overdue) / float64(stats.TaskStats.Total)
	}

	// 阻塞任务比例（假设 review 状态为阻塞）
	reviewCount := stats.TaskStats.ByStatus[string(TaskStatusReview)]
	if stats.TaskStats.Total > 0 {
		stats.RiskIndicators.BlockedTasksRatio = float64(reviewCount) / float64(stats.TaskStats.Total)
	}

	// 工作过载成员
	overloadedThreshold := stats.TaskStats.AverageTasksPerMember * 1.5
	for _, m := range stats.TeamStats.MemberWorkloads {
		if float64(m.TotalTasks) > overloadedThreshold {
			stats.RiskIndicators.OverloadedMembers++
		}
	}

	// 计算风险评分
	riskScore := 0
	riskFactors := make([]string, 0)

	if stats.RiskIndicators.OverdueTasksRatio > 0.3 {
		riskScore += 30
		riskFactors = append(riskFactors, "过期任务比例过高")
	} else if stats.RiskIndicators.OverdueTasksRatio > 0.15 {
		riskScore += 15
		riskFactors = append(riskFactors, "过期任务比例偏高")
	}

	if stats.RiskIndicators.BlockedTasksRatio > 0.2 {
		riskScore += 20
		riskFactors = append(riskFactors, "阻塞任务比例过高")
	}

	if stats.RiskIndicators.OverloadedMembers > 0 {
		riskScore += 15
		riskFactors = append(riskFactors, "部分成员工作过载")
	}

	if stats.MilestoneStats.Overdue > 0 {
		riskScore += 20
		riskFactors = append(riskFactors, "存在过期里程碑")
	}

	if stats.ProgressStats.ProgressDelta < -20 {
		riskScore += 25
		riskFactors = append(riskFactors, "进度严重滞后")
	} else if stats.ProgressStats.ProgressDelta < -10 {
		riskScore += 10
		riskFactors = append(riskFactors, "进度略有滞后")
	}

	stats.RiskIndicators.RiskScore = riskScore
	stats.RiskIndicators.RiskFactors = riskFactors

	// 确定整体风险级别
	switch {
	case riskScore >= 70:
		stats.RiskIndicators.OverallRisk = "critical"
	case riskScore >= 50:
		stats.RiskIndicators.OverallRisk = "high"
	case riskScore >= 30:
		stats.RiskIndicators.OverallRisk = "medium"
	default:
		stats.RiskIndicators.OverallRisk = "low"
	}
}

// GetGlobalStats 获取全局统计
func (sm *StatsManager) GetGlobalStats() GlobalStats {
	projects := sm.manager.ListProjects("", 1000, 0)

	stats := GlobalStats{
		ByStatus:        make(map[string]int),
		TasksByStatus:   make(map[string]int),
		TasksByPriority: make(map[string]int),
	}

	now := time.Now()

	for _, project := range projects {
		stats.TotalProjects++
		stats.ByStatus[project.Status]++

		projectStats := sm.manager.GetTaskStats(project.ID)
		stats.TotalTasks += projectStats.Total
		stats.TotalCompleted += projectStats.ByStatus[string(TaskStatusDone)]
		stats.TotalOverdue += projectStats.Overdue

		for status, count := range projectStats.ByStatus {
			stats.TasksByStatus[status] += count
		}
		for priority, count := range projectStats.ByPriority {
			stats.TasksByPriority[priority] += count
		}
	}

	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.TotalCompleted) / float64(stats.TotalTasks) * 100
		stats.OverdueRate = float64(stats.TotalOverdue) / float64(stats.TotalTasks) * 100
	}

	stats.GeneratedAt = now

	return stats
}

// GlobalStats 全局统计
type GlobalStats struct {
	TotalProjects   int            `json:"total_projects"`
	TotalTasks      int            `json:"total_tasks"`
	TotalCompleted  int            `json:"total_completed"`
	TotalOverdue    int            `json:"total_overdue"`
	CompletionRate  float64        `json:"completion_rate"`
	OverdueRate     float64        `json:"overdue_rate"`
	ByStatus        map[string]int `json:"by_status"`
	TasksByStatus   map[string]int `json:"tasks_by_status"`
	TasksByPriority map[string]int `json:"tasks_by_priority"`
	GeneratedAt     time.Time      `json:"generated_at"`
}

// 辅助函数
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
