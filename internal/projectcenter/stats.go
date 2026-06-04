package projectcenter

import (
	"sync"
	"time"
)

// StatsManager 统计管理器
type StatsManager struct {
	mu             sync.RWMutex
	taskMgr        *TaskManager
	milestoneMgr   *MilestoneManager
}

// NewStatsManager 创建统计管理器
func NewStatsManager(taskMgr *TaskManager, milestoneMgr *MilestoneManager) *StatsManager {
	return &StatsManager{
		taskMgr:      taskMgr,
		milestoneMgr: milestoneMgr,
	}
}

// GetProjectStats 获取项目统计
func (m *StatsManager) GetProjectStats(projectID string) (*ProjectStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ProjectStats{
		ProjectID:       projectID,
		TasksByStatus:   make(map[string]int),
		TasksByPriority: make(map[string]int),
		TasksByAssignee: make(map[string]int),
	}

	// 获取所有项目任务
	tasks, total, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})
	stats.TotalTasks = total

	var totalDuration float64
	var completedWithDuration int

	for _, task := range tasks {
		// 按状态统计
		stats.TasksByStatus[string(task.Status)]++

		// 按优先级统计
		stats.TasksByPriority[string(task.Priority)]++

		// 按分配人统计
		if task.AssigneeID != "" {
			stats.TasksByAssignee[task.AssigneeID]++
		}

		// 完成任务统计
		if task.Status == TaskStatusDone {
			stats.CompletedTasks++
			if task.ActualHours > 0 {
				totalDuration += task.ActualHours
				completedWithDuration++
			} else if task.EstimateHours > 0 {
				totalDuration += task.EstimateHours
				completedWithDuration++
			}
		}

		// 过期任务统计
		if task.Status != TaskStatusDone && task.DueDate != nil && task.DueDate.Before(time.Now()) {
			stats.OverdueTasks++
		}
	}

	// 完成率
	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	// 平均任务时长
	if completedWithDuration > 0 {
		stats.AvgTaskDuration = totalDuration / float64(completedWithDuration)
	}

	// 里程碑统计
	msList := m.milestoneMgr.ListProjectMilestones(projectID)
	stats.MilestoneStats.Total = len(msList)
	for _, ms := range msList {
		if ms.Status == "completed" {
			stats.MilestoneStats.Completed++
		}
		if ms.Status == "overdue" {
			stats.MilestoneStats.Overdue++
		}
	}
	if stats.MilestoneStats.Total > 0 {
		stats.MilestoneStats.CompletionRate = float64(stats.MilestoneStats.Completed) / float64(stats.MilestoneStats.Total) * 100
	}

	// 时间线统计
	stats.TimelineStats = m.calculateTimelineStats(projectID, tasks)

	return stats, nil
}

// calculateTimelineStats 计算时间线统计
func (m *StatsManager) calculateTimelineStats(projectID string, tasks []*Task) TimelineStats {
	var earliestStart, latestEnd time.Time
	var set bool

	for _, task := range tasks {
		if task.StartDate != nil {
			if !set || task.StartDate.Before(earliestStart) {
				earliestStart = *task.StartDate
				set = true
			}
		}
		if task.DueDate != nil {
			if latestEnd.IsZero() || task.DueDate.After(latestEnd) {
				latestEnd = *task.DueDate
			}
		}
	}

	if !set {
		return TimelineStats{}
	}

	now := time.Now()
	totalDays := int(latestEnd.Sub(earliestStart).Hours() / 24)
	elapsedDays := int(now.Sub(earliestStart).Hours() / 24)
	remainingDays := int(latestEnd.Sub(now).Hours() / 24)

	if elapsedDays < 0 {
		elapsedDays = 0
	}
	if remainingDays < 0 {
		remainingDays = 0
	}

	var progress float64
	if totalDays > 0 {
		progress = float64(elapsedDays) / float64(totalDays) * 100
		if progress > 100 {
			progress = 100
		}
	}

	return TimelineStats{
		StartDate:     earliestStart,
		EndDate:       latestEnd,
		ElapsedDays:   elapsedDays,
		RemainingDays: remainingDays,
		TotalDuration: totalDays,
		Progress:      progress,
	}
}

// GetMemberWorkloads 获取成员工作量统计
func (m *StatsManager) GetMemberWorkloads(projectID string) []*MemberWorkload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workloadMap := make(map[string]*MemberWorkload)

	tasks, _, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})

	for _, task := range tasks {
		if task.AssigneeID == "" {
			continue
		}

		wl, exists := workloadMap[task.AssigneeID]
		if !exists {
			wl = &MemberWorkload{
				UserID: task.AssigneeID,
			}
			workloadMap[task.AssigneeID] = wl
		}

		wl.TotalTasks++

		if task.Status == TaskStatusDone {
			wl.CompletedTasks++
			if task.ActualHours > 0 {
				wl.TotalHours += task.ActualHours
			} else {
				wl.TotalHours += task.EstimateHours
			}
		} else {
			wl.ActiveTasks++
			wl.TotalHours += task.EstimateHours
		}
	}

	// 计算利用率（基于 40 小时/周标准）
	var workloads []*MemberWorkload
	for _, wl := range workloadMap {
		if wl.TotalHours > 0 {
			wl.Utilization = (wl.TotalHours / 40) * 100
			if wl.Utilization > 100 {
				wl.Utilization = 100
			}
		}
		workloads = append(workloads, wl)
	}

	return workloads
}

// GetStatusDistribution 获取任务状态分布
func (m *StatsManager) GetStatusDistribution(projectID string) map[string]StatusDistItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dist := make(map[string]StatusDistItem)
	tasks, total, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})

	counts := make(map[string]int)
	for _, task := range tasks {
		counts[string(task.Status)]++
	}

	statusLabels := map[string]string{
		"todo":        "待办",
		"in_progress": "进行中",
		"review":      "审核中",
		"done":        "已完成",
		"blocked":     "已阻塞",
	}

	for status, count := range counts {
		label := status
		if l, ok := statusLabels[status]; ok {
			label = l
		}

		var pct float64
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}

		dist[status] = StatusDistItem{
			Status:     status,
			Label:      label,
			Count:      count,
			Percentage: pct,
		}
	}

	return dist
}

// StatusDistItem 状态分布项
type StatusDistItem struct {
	Status     string  `json:"status"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GetPriorityDistribution 获取任务优先级分布
func (m *StatsManager) GetPriorityDistribution(projectID string) map[string]PriorityDistItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dist := make(map[string]PriorityDistItem)
	tasks, total, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})

	counts := make(map[string]int)
	for _, task := range tasks {
		counts[string(task.Priority)]++
	}

	priorityLabels := map[string]string{
		"low":    "低",
		"medium": "中",
		"high":   "高",
		"urgent": "紧急",
	}

	for priority, count := range counts {
		label := priority
		if l, ok := priorityLabels[priority]; ok {
			label = l
		}

		var pct float64
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}

		dist[priority] = PriorityDistItem{
			Priority:   priority,
			Label:      label,
			Count:      count,
			Percentage: pct,
		}
	}

	return dist
}

// PriorityDistItem 优先级分布项
type PriorityDistItem struct {
	Priority   string  `json:"priority"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GetWeeklyProgress 获取每周进度
func (m *StatsManager) GetWeeklyProgress(projectID string, weeks int) []WeeklyProgressItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if weeks <= 0 {
		weeks = 8
	}

	tasks, _, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})

	now := time.Now()
	var items []WeeklyProgressItem

	for i := weeks - 1; i >= 0; i-- {
		weekEnd := now.AddDate(0, 0, -i*7)
		weekStart := weekEnd.AddDate(0, 0, -7)

		completed := 0
		created := 0
		for _, task := range tasks {
			if task.CompletedAt != nil && task.CompletedAt.After(weekStart) && task.CompletedAt.Before(weekEnd) {
				completed++
			}
			if task.CreatedAt.After(weekStart) && task.CreatedAt.Before(weekEnd) {
				created++
			}
		}

		items = append(items, WeeklyProgressItem{
			WeekStart: weekStart,
			WeekEnd:   weekEnd,
			Created:   created,
			Completed: completed,
		})
	}

	return items
}

// WeeklyProgressItem 每周进度项
type WeeklyProgressItem struct {
	WeekStart time.Time `json:"week_start"`
	WeekEnd   time.Time `json:"week_end"`
	Created   int       `json:"created"`
	Completed int       `json:"completed"`
}

// GetProjectSummary 获取项目概览摘要
func (m *StatsManager) GetProjectSummary(projectID string) (*ProjectSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &ProjectSummary{}

	// 任务统计
	tasks, total, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})
	summary.TotalTasks = total

	for _, task := range tasks {
		if task.Status == TaskStatusDone {
			summary.CompletedTasks++
		} else if task.Status == TaskStatusInProgress {
			summary.InProgressTasks++
		} else if task.Status == TaskStatusBlocked {
			summary.BlockedTasks++
		}
	}

	// 过期任务
	summary.OverdueTasks = len(m.taskMgr.GetOverdueTasks(projectID))

	// 里程碑
	milestones := m.milestoneMgr.ListProjectMilestones(projectID)
	summary.TotalMilestones = len(milestones)
	for _, ms := range milestones {
		if ms.Status == "completed" {
			summary.CompletedMilestones++
		}
	}

	// 计算进度
	if summary.TotalTasks > 0 {
		summary.OverallProgress = float64(summary.CompletedTasks) / float64(summary.TotalTasks) * 100
	}

	return summary, nil
}

// ProjectSummary 项目摘要
type ProjectSummary struct {
	TotalTasks          int     `json:"total_tasks"`
	CompletedTasks      int     `json:"completed_tasks"`
	InProgressTasks     int     `json:"in_progress_tasks"`
	BlockedTasks        int     `json:"blocked_tasks"`
	OverdueTasks        int     `json:"overdue_tasks"`
	TotalMilestones     int     `json:"total_milestones"`
	CompletedMilestones int     `json:"completed_milestones"`
	OverallProgress     float64 `json:"overall_progress"`
}

// GetBurndownData 获取燃尽图数据
func (m *StatsManager) GetBurndownData(projectID string, startDate, endDate time.Time) []BurndownItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks, _, _ := m.taskMgr.ListProjectTasks(projectID, ListOptions{PageSize: 10000})
	totalTasks := len(tasks)

	var items []BurndownItem
	day := startDate
	idealPerDay := float64(totalTasks) / float64(int(endDate.Sub(startDate).Hours()/24)+1)

	dayIndex := 0
	for !day.After(endDate) {
		completed := 0
		for _, task := range tasks {
			if task.CompletedAt != nil && !task.CompletedAt.After(day) {
				completed++
			}
		}

		remaining := totalTasks - completed
		ideal := float64(totalTasks) - idealPerDay*float64(dayIndex)
		if ideal < 0 {
			ideal = 0
		}

		items = append(items, BurndownItem{
			Date:      day,
			Actual:    remaining,
			Ideal:     int(ideal),
			Completed: completed,
		})

		day = day.AddDate(0, 0, 1)
		dayIndex++
	}

	return items
}

// BurndownItem 燃尽图数据项
type BurndownItem struct {
	Date      time.Time `json:"date"`
	Actual    int       `json:"actual"`
	Ideal     int       `json:"ideal"`
	Completed int       `json:"completed"`
}
