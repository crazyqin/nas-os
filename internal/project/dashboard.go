// Package project provides project management functionality
package project

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Dashboard 仪表板数据结构
type Dashboard struct {
	// 项目概览
	TotalProjects     int `json:"total_projects"`
	ActiveProjects    int `json:"active_projects"`
	CompletedProjects int `json:"completed_projects"`

	// 任务概览
	TotalTasks      int `json:"total_tasks"`
	TodoTasks       int `json:"todo_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	ReviewTasks     int `json:"review_tasks"`
	DoneTasks       int `json:"done_tasks"`
	CancelledTasks  int `json:"cancelled_tasks"`
	OverdueTasks    int `json:"overdue_tasks"`

	// 完成率
	TaskCompletionRate   float64 `json:"task_completion_rate"`
	OnTimeCompletionRate float64 `json:"on_time_completion_rate"`

	// 里程碑
	TotalMilestones         int     `json:"total_milestones"`
	CompletedMilestones     int     `json:"completed_milestones"`
	ActiveMilestones        int     `json:"active_milestones"`
	OverdueMilestones       int     `json:"overdue_milestones"`
	MilestoneCompletionRate float64 `json:"milestone_completion_rate"`

	// 时间统计
	TasksCreatedThisWeek    int `json:"tasks_created_this_week"`
	TasksCompletedThisWeek  int `json:"tasks_completed_this_week"`
	TasksCreatedThisMonth   int `json:"tasks_created_this_month"`
	TasksCompletedThisMonth int `json:"tasks_completed_this_month"`

	// 趋势数据
	DailyTaskTrend       []DailyTaskStats  `json:"daily_task_trend,omitempty"`
	WeeklyTaskTrend      []WeeklyTaskStats `json:"weekly_task_trend,omitempty"`
	PriorityDistribution map[string]int    `json:"priority_distribution"`
	AssigneeWorkload     map[string]int    `json:"assignee_workload"`
}

// DailyTaskStats 每日任务统计
type DailyTaskStats struct {
	Date      string `json:"date"`
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

// WeeklyTaskStats 每周任务统计
type WeeklyTaskStats struct {
	WeekStart string `json:"week_start"`
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

// ProjectDashboard 项目仪表板
type ProjectDashboard struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`

	// 进度统计
	TaskStats      TaskStats `json:"task_stats"`
	Progress       int       `json:"progress"` // 0-100
	EstimatedHours float64   `json:"estimated_hours"`
	ActualHours    float64   `json:"actual_hours"`
	HoursVariance  float64   `json:"hours_variance"` // 实际-预估

	// 里程碑统计
	TotalMilestones     int `json:"total_milestones"`
	CompletedMilestones int `json:"completed_milestones"`
	ActiveMilestones    int `json:"active_milestones"`
	OverdueMilestones   int `json:"overdue_milestones"`

	// 时间线
	StartDate     *time.Time `json:"start_date,omitempty"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	DaysRemaining int        `json:"days_remaining"`

	// 团队统计
	TeamSize        int                       `json:"team_size"`
	MemberWorkloads map[string]MemberWorkload `json:"member_workloads"`
}

// MemberWorkload 成员工作量
type MemberWorkload struct {
	UserID          string  `json:"user_id"`
	TotalTasks      int     `json:"total_tasks"`
	TodoTasks       int     `json:"todo_tasks"`
	InProgressTasks int     `json:"in_progress_tasks"`
	DoneTasks       int     `json:"done_tasks"`
	OverdueTasks    int     `json:"overdue_tasks"`
	EstimatedHours  float64 `json:"estimated_hours"`
	ActualHours     float64 `json:"actual_hours"`
}

// MilestoneProgress 里程碑进度
type MilestoneProgress struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	TaskCount     int            `json:"task_count"`
	DoneCount     int            `json:"done_count"`
	Progress      int            `json:"progress"` // 0-100
	DueDate       *time.Time     `json:"due_date,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	IsOverdue     bool           `json:"is_overdue"`
	DaysRemaining int            `json:"days_remaining"`
	TasksByStatus map[string]int `json:"tasks_by_status"`
}

// DashboardHandlers 仪表板处理器
type DashboardHandlers struct {
	manager *Manager
}

// NewDashboardHandlers 创建仪表板处理器
func NewDashboardHandlers(mgr *Manager) *DashboardHandlers {
	return &DashboardHandlers{
		manager: mgr,
	}
}

// RegisterDashboardRoutes 注册仪表板路由
func (h *DashboardHandlers) RegisterDashboardRoutes(router *gin.RouterGroup) {
	dashboard := router.Group("/dashboard")
	{
		dashboard.GET("/overview", h.getOverview)
		dashboard.GET("/project/:id", h.getProjectDashboard)
		dashboard.GET("/milestones", h.getMilestonesProgress)
		dashboard.GET("/trends", h.getTrends)
		dashboard.GET("/workload", h.getWorkload)
	}
}

// getOverview 获取全局仪表板概览
// @Summary 获取仪表板概览
// @Description 获取全局项目管理仪表板数据
// @Tags dashboard
// @Accept json
// @Produce json
// @Param days query int false "趋势天数" default(7)
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/overview [get]
func (h *DashboardHandlers) getOverview(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	dashboard := h.manager.GetDashboardOverview(days)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    dashboard,
	})
}

// getProjectDashboard 获取项目仪表板
// @Summary 获取项目仪表板
// @Description 获取指定项目的仪表板数据
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /dashboard/project/{id} [get]
func (h *DashboardHandlers) getProjectDashboard(c *gin.Context) {
	projectID := c.Param("id")

	dashboard, err := h.manager.GetProjectDashboard(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    dashboard,
	})
}

// getMilestonesProgress 获取里程碑进度
// @Summary 获取里程碑进度
// @Description 获取所有里程碑的进度追踪数据
// @Tags dashboard
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID筛选"
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/milestones [get]
func (h *DashboardHandlers) getMilestonesProgress(c *gin.Context) {
	projectID := c.Query("project_id")

	milestones := h.manager.GetMilestonesProgress(projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    milestones,
	})
}

// getTrends 获取任务趋势
// @Summary 获取任务趋势
// @Description 获取任务创建和完成的趋势数据
// @Tags dashboard
// @Accept json
// @Produce json
// @Param days query int false "天数" default(7)
// @Param project_id query string false "项目ID筛选"
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/trends [get]
func (h *DashboardHandlers) getTrends(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	projectID := c.Query("project_id")

	trends := h.manager.GetTaskTrends(days, projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    trends,
	})
}

// getWorkload 获取工作量统计
// @Summary 获取工作量统计
// @Description 获取各成员的工作量分布
// @Tags dashboard
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID筛选"
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/workload [get]
func (h *DashboardHandlers) getWorkload(c *gin.Context) {
	projectID := c.Query("project_id")

	workload := h.manager.GetWorkload(projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    workload,
	})
}

// ========== Manager 仪表板方法 ==========

// GetDashboardOverview 获取仪表板概览
func (m *Manager) GetDashboardOverview(trendDays int) Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	dashboard := Dashboard{
		PriorityDistribution: make(map[string]int),
		AssigneeWorkload:     make(map[string]int),
	}

	// 项目统计
	for _, project := range m.projects {
		dashboard.TotalProjects++
		switch project.Status {
		case "active":
			dashboard.ActiveProjects++
		case "completed", "archived":
			dashboard.CompletedProjects++
		}
	}

	// 任务统计
	for _, task := range m.tasks {
		dashboard.TotalTasks++

		switch task.Status {
		case TaskStatusTodo:
			dashboard.TodoTasks++
		case TaskStatusInProgress:
			dashboard.InProgressTasks++
		case TaskStatusReview:
			dashboard.ReviewTasks++
		case TaskStatusDone:
			dashboard.DoneTasks++
		case TaskStatusCancelled:
			dashboard.CancelledTasks++
		}

		// 优先级分布
		dashboard.PriorityDistribution[string(task.Priority)]++

		// 成员工作量
		if task.AssigneeID != "" {
			dashboard.AssigneeWorkload[task.AssigneeID]++
		}

		// 过期任务
		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			dashboard.OverdueTasks++
		}

		// 时间统计
		if task.CreatedAt.After(weekAgo) {
			dashboard.TasksCreatedThisWeek++
		}
		if task.CompletedAt != nil && task.CompletedAt.After(weekAgo) {
			dashboard.TasksCompletedThisWeek++
		}
		if task.CreatedAt.After(monthAgo) {
			dashboard.TasksCreatedThisMonth++
		}
		if task.CompletedAt != nil && task.CompletedAt.After(monthAgo) {
			dashboard.TasksCompletedThisMonth++
		}
	}

	// 完成率计算
	if dashboard.TotalTasks > 0 {
		dashboard.TaskCompletionRate = float64(dashboard.DoneTasks) / float64(dashboard.TotalTasks) * 100
	}

	// 准时完成率（已完成任务中，按时完成的比例）
	onTimeCount := 0
	for _, task := range m.tasks {
		if task.Status == TaskStatusDone && task.CompletedAt != nil && task.DueDate != nil {
			if task.CompletedAt.Before(*task.DueDate) || task.CompletedAt.Equal(*task.DueDate) {
				onTimeCount++
			}
		}
	}
	if dashboard.DoneTasks > 0 {
		dashboard.OnTimeCompletionRate = float64(onTimeCount) / float64(dashboard.DoneTasks) * 100
	}

	// 里程碑统计
	for _, milestone := range m.milestones {
		dashboard.TotalMilestones++

		switch milestone.Status {
		case "completed":
			dashboard.CompletedMilestones++
		case "active":
			dashboard.ActiveMilestones++
		}

		// 过期里程碑
		if milestone.DueDate != nil && milestone.DueDate.Before(now) && milestone.Status != "completed" {
			dashboard.OverdueMilestones++
		}
	}

	// 里程碑完成率
	if dashboard.TotalMilestones > 0 {
		dashboard.MilestoneCompletionRate = float64(dashboard.CompletedMilestones) / float64(dashboard.TotalMilestones) * 100
	}

	// 生成趋势数据
	if trendDays > 0 {
		dashboard.DailyTaskTrend = m.generateDailyTrend(trendDays, now)
	}

	return dashboard
}

// GetProjectDashboard 获取项目仪表板
func (m *Manager) GetProjectDashboard(projectID string) (*ProjectDashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	dashboard := &ProjectDashboard{
		ProjectID:       projectID,
		ProjectName:     project.Name,
		StartDate:       project.StartDtae,
		EndDate:         project.EndDate,
		MemberWorkloads: make(map[string]MemberWorkload),
		TaskStats: TaskStats{
			ByStatus:   make(map[string]int),
			ByPriority: make(map[string]int),
			ByAssignee: make(map[string]int),
		},
	}

	// 收集项目成员
	memberSet := make(map[string]bool)
	if project.OwnerID != "" {
		memberSet[project.OwnerID] = true
	}
	for _, memberID := range project.MemberIDs {
		memberSet[memberID] = true
	}
	dashboard.TeamSize = len(memberSet)

	// 任务统计
	for _, task := range m.tasks {
		if task.ProjectID != projectID {
			continue
		}

		dashboard.TaskStats.Total++
		switch task.Status {
		case TaskStatusTodo:
			dashboard.TaskStats.ByStatus["todo"]++
		case TaskStatusInProgress:
			dashboard.TaskStats.ByStatus["in_progress"]++
		case TaskStatusReview:
			dashboard.TaskStats.ByStatus["review"]++
		case TaskStatusDone:
			dashboard.TaskStats.ByStatus["done"]++
		case TaskStatusCancelled:
			dashboard.TaskStats.ByStatus["cancelled"]++
		}

		// 优先级
		dashboard.TaskStats.ByPriority[string(task.Priority)]++

		// 工时
		dashboard.EstimatedHours += task.EstimatedHours
		dashboard.ActualHours += task.ActualHours

		// 过期任务
		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			dashboard.TaskStats.Overdue++
		}

		// 成员工作量
		if task.AssigneeID != "" {
			wl := dashboard.MemberWorkloads[task.AssigneeID]
			wl.UserID = task.AssigneeID
			wl.TotalTasks++
			switch task.Status {
			case TaskStatusTodo:
				wl.TodoTasks++
			case TaskStatusInProgress:
				wl.InProgressTasks++
			case TaskStatusDone:
				wl.DoneTasks++
			}
			if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
				wl.OverdueTasks++
			}
			wl.EstimatedHours += task.EstimatedHours
			wl.ActualHours += task.ActualHours
			dashboard.MemberWorkloads[task.AssigneeID] = wl
		}
	}

	// 计算进度
	if dashboard.TaskStats.Total > 0 {
		doneCount := dashboard.TaskStats.ByStatus["done"]
		dashboard.Progress = int(float64(doneCount) / float64(dashboard.TaskStats.Total) * 100)
	}

	// 工时差异
	dashboard.HoursVariance = dashboard.ActualHours - dashboard.EstimatedHours

	// 里程碑统计
	for _, milestone := range m.milestones {
		if milestone.ProjectID != projectID {
			continue
		}

		dashboard.TotalMilestones++
		switch milestone.Status {
		case "completed":
			dashboard.CompletedMilestones++
		case "active":
			dashboard.ActiveMilestones++
		}

		// 过期里程碑
		if milestone.DueDate != nil && milestone.DueDate.Before(now) && milestone.Status != "completed" {
			dashboard.OverdueMilestones++
		}
	}

	// 计算剩余天数
	if dashboard.EndDate != nil {
		dashboard.DaysRemaining = int(time.Until(*dashboard.EndDate).Hours() / 24)
		if dashboard.DaysRemaining < 0 {
			dashboard.DaysRemaining = 0
		}
	}

	return dashboard, nil
}

// GetMilestonesProgress 获取里程碑进度
func (m *Manager) GetMilestonesProgress(projectID string) []MilestoneProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	result := make([]MilestoneProgress, 0)

	for _, milestone := range m.milestones {
		if projectID != "" && milestone.ProjectID != projectID {
			continue
		}

		progress := MilestoneProgress{
			ID:            milestone.ID,
			Name:          milestone.Name,
			Status:        milestone.Status,
			TaskCount:     milestone.TaskCount,
			DoneCount:     milestone.DoneCount,
			DueDate:       milestone.DueDate,
			CompletedAt:   milestone.CompletedAt,
			TasksByStatus: make(map[string]int),
		}

		// 计算进度
		if milestone.TaskCount > 0 {
			progress.Progress = int(float64(milestone.DoneCount) / float64(milestone.TaskCount) * 100)
		}

		// 检查是否过期
		if milestone.DueDate != nil && milestone.DueDate.Before(now) && milestone.Status != "completed" {
			progress.IsOverdue = true
		}

		// 计算剩余天数
		if milestone.DueDate != nil && milestone.Status != "completed" {
			progress.DaysRemaining = int(time.Until(*milestone.DueDate).Hours() / 24)
			if progress.DaysRemaining < 0 {
				progress.DaysRemaining = 0
			}
		}

		// 统计里程碑下任务状态分布
		for _, task := range m.tasks {
			if task.MilestoneID == milestone.ID {
				progress.TasksByStatus[string(task.Status)]++
			}
		}

		result = append(result, progress)
	}

	return result
}

// GetTaskTrends 获取任务趋势
func (m *Manager) GetTaskTrends(days int, projectID string) []DailyTaskStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	trends := make([]DailyTaskStats, 0, days)

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		dayEnd := dayStart.Add(24 * time.Hour)

		stats := DailyTaskStats{
			Date: dateStr,
		}

		for _, task := range m.tasks {
			if projectID != "" && task.ProjectID != projectID {
				continue
			}

			// 当天创建
			if task.CreatedAt.After(dayStart) && task.CreatedAt.Before(dayEnd) {
				stats.Created++
			}

			// 当天完成
			if task.CompletedAt != nil && task.CompletedAt.After(dayStart) && task.CompletedAt.Before(dayEnd) {
				stats.Completed++
			}

			// 当天累计总数（创建时间早于当天结束）
			if task.CreatedAt.Before(dayEnd) {
				stats.Total++
			}
		}

		trends = append(trends, stats)
	}

	return trends
}

// GetWorkload 获取工作量统计
func (m *Manager) GetWorkload(projectID string) map[string]MemberWorkload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	workload := make(map[string]MemberWorkload)

	for _, task := range m.tasks {
		if projectID != "" && task.ProjectID != projectID {
			continue
		}

		if task.AssigneeID == "" {
			continue
		}

		wl := workload[task.AssigneeID]
		wl.UserID = task.AssigneeID
		wl.TotalTasks++

		switch task.Status {
		case TaskStatusTodo:
			wl.TodoTasks++
		case TaskStatusInProgress:
			wl.InProgressTasks++
		case TaskStatusDone:
			wl.DoneTasks++
		}

		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			wl.OverdueTasks++
		}

		wl.EstimatedHours += task.EstimatedHours
		wl.ActualHours += task.ActualHours

		workload[task.AssigneeID] = wl
	}

	return workload
}

// generateDailyTrend 生成每日趋势
func (m *Manager) generateDailyTrend(days int, now time.Time) []DailyTaskStats {
	trends := make([]DailyTaskStats, 0, days)

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		dayEnd := dayStart.Add(24 * time.Hour)

		stats := DailyTaskStats{
			Date: dateStr,
		}

		for _, task := range m.tasks {
			// 当天创建
			if task.CreatedAt.After(dayStart) && task.CreatedAt.Before(dayEnd) {
				stats.Created++
			}

			// 当天完成
			if task.CompletedAt != nil && task.CompletedAt.After(dayStart) && task.CompletedAt.Before(dayEnd) {
				stats.Completed++
			}

			// 累计总数
			if task.CreatedAt.Before(dayEnd) && task.Status != TaskStatusCancelled {
				stats.Total++
			}
		}

		trends = append(trends, stats)
	}

	return trends
}
