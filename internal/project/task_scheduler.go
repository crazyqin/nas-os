// Package project provides task scheduling functionality
package project

import (
	"math"
	"sort"
	"sync"
	"time"
)

// TaskScheduler 任务调度器
type TaskScheduler struct {
	mu       sync.RWMutex
	manager  *Manager
	config   SchedulerConfig
	strategy SchedulingStrategy
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxTasksPerUser     int            `json:"max_tasks_per_user"`     // 每人最大任务数
	PriorityWeight      float64        `json:"priority_weight"`        // 优先级权重
	DueDateWeight       float64        `json:"due_date_weight"`        // 截止日期权重
	DependencyWeight    float64        `json:"dependency_weight"`      // 依赖权重
	WorkloadBalance     bool           `json:"workload_balance"`       // 是否平衡工作负载
	ConsiderSkills      bool           `json:"consider_skills"`        // 是否考虑技能
	BusinessHoursPerDay int            `json:"business_hours_per_day"` // 每天工作小时数
	WorkingDays         []time.Weekday `json:"working_days"`           // 工作日
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxTasksPerUser:     10,
		PriorityWeight:      0.4,
		DueDateWeight:       0.3,
		DependencyWeight:    0.3,
		WorkloadBalance:     true,
		ConsiderSkills:      false,
		BusinessHoursPerDay: 8,
		WorkingDays:         []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
	}
}

// SchedulingStrategy 调度策略接口
type SchedulingStrategy interface {
	Score(task *Task, user *UserInfo, context SchedulingContext) float64
	Name() string
}

// UserInfo 用户信息
type UserInfo struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Skills         map[string]int   `json:"skills,omitempty"` // 技能 -> 熟练度 (1-10)
	CurrentLoad    int              `json:"current_load"`     // 当前任务数
	CompletedTasks int              `json:"completed_tasks"`  // 已完成任务数
	History        *UserPerformance `json:"history,omitempty"`
}

// UserPerformance 用户绩效
type UserPerformance struct {
	AverageCompletionTime float64 `json:"average_completion_time"` // 平均完成时间（小时）
	OnTimeRate            float64 `json:"on_time_rate"`            // 按时完成率
	QualityScore          float64 `json:"quality_score"`           // 质量分数 (0-100)
}

// SchedulingContext 调度上下文
type SchedulingContext struct {
	ProjectTasks    []*Task             `json:"project_tasks"`
	UserWorkloads   map[string]int      `json:"user_workloads"`
	Dependencies    map[string][]string `json:"dependencies"` // taskID -> dependent task IDs
	CurrentDate     time.Time           `json:"current_date"`
	ProjectDeadline *time.Time          `json:"project_deadline,omitempty"`
}

// SchedulingResult 调度结果
type SchedulingResult struct {
	TaskID          string       `json:"task_id"`
	TaskTitle       string       `json:"task_title"`
	RecommendedUser string       `json:"recommended_user,omitempty"`
	RecommendedDate *time.Time   `json:"recommended_date,omitempty"`
	Priority        TaskPriority `json:"priority"`
	Score           float64      `json:"score"`
	Reason          string       `json:"reason"`
	Alternatives    []UserScore  `json:"alternatives,omitempty"`
	Conflicts       []Conflict   `json:"conflicts,omitempty"`
}

// UserScore 用户评分
type UserScore struct {
	UserID string  `json:"user_id"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// Conflict 冲突信息
type Conflict struct {
	Type        string `json:"type"` // workload, skill, dependency, schedule
	Description string `json:"description"`
	TaskID      string `json:"task_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Severity    string `json:"severity"` // low, medium, high
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(mgr *Manager, config SchedulerConfig) *TaskScheduler {
	return &TaskScheduler{
		manager:  mgr,
		config:   config,
		strategy: &DefaultSchedulingStrategy{config: config},
	}
}

// DefaultSchedulingStrategy 默认调度策略
type DefaultSchedulingStrategy struct {
	config SchedulerConfig
}

// Name 策略名称
func (s *DefaultSchedulingStrategy) Name() string {
	return "default"
}

// Score 计算用户对任务的适配分数
func (s *DefaultSchedulingStrategy) Score(task *Task, user *UserInfo, context SchedulingContext) float64 {
	score := 0.0

	// 1. 工作负载因素
	if s.config.WorkloadBalance {
		loadScore := 1.0 - float64(user.CurrentLoad)/float64(s.config.MaxTasksPerUser)
		if loadScore < 0 {
			loadScore = 0
		}
		score += loadScore * 0.3
	}

	// 2. 优先级匹配
	if task.AssigneeID == "" || task.AssigneeID == user.ID {
		score += s.config.PriorityWeight
	}

	// 3. 历史绩效
	if user.History != nil {
		performanceScore := user.History.OnTimeRate * 0.2
		performanceScore += (user.History.QualityScore / 100) * 0.1
		score += performanceScore
	}

	// 4. 技能匹配
	if s.config.ConsiderSkills && len(user.Skills) > 0 {
		// 简单匹配：检查标签和技能
		for _, tag := range task.Tags {
			if level, ok := user.Skills[tag]; ok {
				score += float64(level) * 0.05
			}
		}
	}

	return score
}

// ScheduleTask 调度单个任务
func (ts *TaskScheduler) ScheduleTask(taskID string, users []*UserInfo) (*SchedulingResult, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, err := ts.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	if task.AssigneeID != "" {
		// 任务已分配
		return &SchedulingResult{
			TaskID:          taskID,
			TaskTitle:       task.Title,
			RecommendedUser: task.AssigneeID,
			Priority:        task.Priority,
			Reason:          "任务已分配",
		}, nil
	}

	// 构建上下文
	context := ts.buildContext(task.ProjectID)

	// 计算每个用户的得分
	userScores := make([]UserScore, 0, len(users))
	for _, user := range users {
		score := ts.strategy.Score(task, user, context)
		reason := ts.explainScore(task, user, score, context)
		userScores = append(userScores, UserScore{
			UserID: user.ID,
			Score:  score,
			Reason: reason,
		})
	}

	// 排序
	sort.Slice(userScores, func(i, j int) bool {
		return userScores[i].Score > userScores[j].Score
	})

	if len(userScores) == 0 {
		return &SchedulingResult{
			TaskID:    taskID,
			TaskTitle: task.Title,
			Priority:  task.Priority,
			Reason:    "无可用用户",
		}, nil
	}

	// 检测冲突
	conflicts := ts.detectConflicts(task, userScores[0].UserID, context)

	result := &SchedulingResult{
		TaskID:          taskID,
		TaskTitle:       task.Title,
		RecommendedUser: userScores[0].UserID,
		Priority:        task.Priority,
		Score:           userScores[0].Score,
		Reason:          userScores[0].Reason,
		Conflicts:       conflicts,
	}

	// 添加备选用户
	if len(userScores) > 1 {
		endIdx := 3
		if len(userScores) < endIdx {
			endIdx = len(userScores)
		}
		result.Alternatives = userScores[1:endIdx]
	}

	// 计算建议开始日期
	result.RecommendedDate = ts.calculateRecommendedDate(task, context)

	return result, nil
}

// ScheduleProject 调度项目所有未分配任务
func (ts *TaskScheduler) ScheduleProject(projectID string, users []*UserInfo) ([]*SchedulingResult, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 获取项目所有未分配任务
	filter := TaskFilter{
		ProjectID: projectID,
		Status:    []TaskStatus{TaskStatusTodo, TaskStatusInProgress},
		Limit:     1000,
	}
	tasks := ts.manager.ListTasks(filter)

	results := make([]*SchedulingResult, 0)
	context := ts.buildContext(projectID)

	// 按优先级和截止日期排序任务
	sort.Slice(tasks, func(i, j int) bool {
		// 高优先级优先
		priorityOrder := map[TaskPriority]int{
			PriorityUrgent: 4, PriorityHigh: 3, PriorityMedium: 2, PriorityLow: 1,
		}
		if priorityOrder[tasks[i].Priority] != priorityOrder[tasks[j].Priority] {
			return priorityOrder[tasks[i].Priority] > priorityOrder[tasks[j].Priority]
		}
		// 有截止日期的优先
		if tasks[i].DueDate != nil && tasks[j].DueDate == nil {
			return true
		}
		if tasks[i].DueDate != nil && tasks[j].DueDate != nil {
			return tasks[i].DueDate.Before(*tasks[j].DueDate)
		}
		return false
	})

	// 更新用户负载追踪
	userLoads := make(map[string]int)
	for _, user := range users {
		userLoads[user.ID] = user.CurrentLoad
	}

	for _, task := range tasks {
		if task.AssigneeID != "" {
			continue // 已分配
		}

		// 计算每个用户的得分（考虑当前负载）
		userScores := make([]UserScore, 0, len(users))
		for _, user := range users {
			// 创建带有更新负载的用户副本
			userCopy := *user
			userCopy.CurrentLoad = userLoads[user.ID]

			score := ts.strategy.Score(task, &userCopy, context)
			reason := ts.explainScore(task, &userCopy, score, context)
			userScores = append(userScores, UserScore{
				UserID: user.ID,
				Score:  score,
				Reason: reason,
			})
		}

		// 排序
		sort.Slice(userScores, func(i, j int) bool {
			return userScores[i].Score > userScores[j].Score
		})

		if len(userScores) == 0 {
			continue
		}

		bestUser := userScores[0]
		conflicts := ts.detectConflicts(task, bestUser.UserID, context)

		result := &SchedulingResult{
			TaskID:          task.ID,
			TaskTitle:       task.Title,
			RecommendedUser: bestUser.UserID,
			Priority:        task.Priority,
			Score:           bestUser.Score,
			Reason:          bestUser.Reason,
			Conflicts:       conflicts,
		}

		result.RecommendedDate = ts.calculateRecommendedDate(task, context)
		results = append(results, result)

		// 更新负载追踪
		userLoads[bestUser.UserID]++
	}

	return results, nil
}

// AutoAssign 自动分配任务
func (ts *TaskScheduler) AutoAssign(taskID string, users []*UserInfo) (string, error) {
	result, err := ts.ScheduleTask(taskID, users)
	if err != nil {
		return "", err
	}

	if result.RecommendedUser == "" {
		return "", nil
	}

	// 执行分配
	tt := NewTaskTracker(ts.manager)
	_, err = tt.AssignTask(taskID, result.RecommendedUser, "scheduler")
	if err != nil {
		return "", err
	}

	return result.RecommendedUser, nil
}

// buildContext 构建调度上下文
func (ts *TaskScheduler) buildContext(projectID string) SchedulingContext {
	context := SchedulingContext{
		ProjectTasks:  ts.manager.ListTasks(TaskFilter{ProjectID: projectID, Limit: 10000}),
		UserWorkloads: make(map[string]int),
		Dependencies:  make(map[string][]string),
		CurrentDate:   time.Now(),
	}

	// 统计用户负载
	for _, task := range context.ProjectTasks {
		if task.AssigneeID != "" && task.Status != TaskStatusDone {
			context.UserWorkloads[task.AssigneeID]++
		}
	}

	// 统计依赖关系
	for _, task := range context.ProjectTasks {
		if task.ParentID != "" {
			context.Dependencies[task.ParentID] = append(context.Dependencies[task.ParentID], task.ID)
		}
	}

	return context
}

// explainScore 解释评分原因
func (ts *TaskScheduler) explainScore(task *Task, user *UserInfo, score float64, context SchedulingContext) string {
	reasons := make([]string, 0)

	// 工作负载
	if user.CurrentLoad == 0 {
		reasons = append(reasons, "当前无任务负载")
	} else if user.CurrentLoad < ts.config.MaxTasksPerUser/2 {
		reasons = append(reasons, "负载适中")
	} else if user.CurrentLoad >= ts.config.MaxTasksPerUser {
		reasons = append(reasons, "⚠️ 已达到最大任务数")
	} else {
		reasons = append(reasons, "负载较高")
	}

	// 绩效
	if user.History != nil {
		if user.History.OnTimeRate > 0.9 {
			reasons = append(reasons, "按时完成率高")
		}
		if user.History.QualityScore > 90 {
			reasons = append(reasons, "质量优秀")
		}
	}

	// 技能匹配
	if ts.config.ConsiderSkills && len(user.Skills) > 0 {
		for _, tag := range task.Tags {
			if level, ok := user.Skills[tag]; ok && level >= 7 {
				reasons = append(reasons, "技能匹配: "+tag)
			}
		}
	}

	if len(reasons) == 0 {
		return "综合评分"
	}

	return reasons[0]
}

// detectConflicts 检测冲突
func (ts *TaskScheduler) detectConflicts(task *Task, userID string, context SchedulingContext) []Conflict {
	conflicts := make([]Conflict, 0)

	// 1. 负载冲突
	load := context.UserWorkloads[userID]
	if load >= ts.config.MaxTasksPerUser {
		conflicts = append(conflicts, Conflict{
			Type:        "workload",
			Description: "用户已达到最大任务数限制",
			UserID:      userID,
			Severity:    "high",
		})
	} else if load >= ts.config.MaxTasksPerUser*2/3 {
		conflicts = append(conflicts, Conflict{
			Type:        "workload",
			Description: "用户负载较高",
			UserID:      userID,
			Severity:    "medium",
		})
	}

	// 2. 依赖冲突
	for _, depTaskIDs := range context.Dependencies {
		for _, depID := range depTaskIDs {
			if depID == task.ID {
				// 有任务依赖此任务
				conflicts = append(conflicts, Conflict{
					Type:        "dependency",
					Description: "存在依赖此任务的其他任务",
					TaskID:      task.ID,
					Severity:    "low",
				})
			}
		}
	}

	// 3. 截止日期冲突
	if task.DueDate != nil {
		now := time.Now()
		daysUntilDue := task.DueDate.Sub(now).Hours() / 24

		if daysUntilDue < 0 {
			conflicts = append(conflicts, Conflict{
				Type:        "schedule",
				Description: "任务已过期",
				TaskID:      task.ID,
				Severity:    "critical",
			})
		} else if daysUntilDue < 2 {
			conflicts = append(conflicts, Conflict{
				Type:        "schedule",
				Description: "任务即将到期",
				TaskID:      task.ID,
				Severity:    "high",
			})
		}
	}

	return conflicts
}

// calculateRecommendedDate 计算建议开始日期
func (ts *TaskScheduler) calculateRecommendedDate(task *Task, context SchedulingContext) *time.Time {
	if task.StartDate != nil {
		return task.StartDate
	}

	now := time.Now()

	// 找到下一个工作日
	date := now
	for {
		isWorkingDay := false
		for _, wd := range ts.config.WorkingDays {
			if date.Weekday() == wd {
				isWorkingDay = true
				break
			}
		}
		if isWorkingDay {
			break
		}
		date = date.Add(24 * time.Hour)
	}

	return &date
}

// GetSchedulingRecommendations 获取调度建议
func (ts *TaskScheduler) GetSchedulingRecommendations(projectID string, users []*UserInfo) (*SchedulingRecommendations, error) {
	results, err := ts.ScheduleProject(projectID, users)
	if err != nil {
		return nil, err
	}

	recs := &SchedulingRecommendations{
		ProjectID:            projectID,
		GeneratedAt:          time.Now(),
		TaskAssignments:      results,
		LoadDistribution:     make(map[string]int),
		UnassignedTasks:      make([]*Task, 0),
		PotentialBottlenecks: make([]BottleneckInfo, 0),
		Suggestions:          make([]string, 0),
	}

	// 统计负载分布
	for _, result := range results {
		if result.RecommendedUser != "" {
			recs.LoadDistribution[result.RecommendedUser]++
		}
	}

	// 检测潜在瓶颈
	maxLoad := 0
	var maxUser string
	for user, load := range recs.LoadDistribution {
		if load > maxLoad {
			maxLoad = load
			maxUser = user
		}
	}
	if maxLoad > ts.config.MaxTasksPerUser*2/3 {
		recs.PotentialBottlenecks = append(recs.PotentialBottlenecks, BottleneckInfo{
			Type:   "assignee",
			ID:     maxUser,
			Name:   "高负载用户: " + maxUser,
			Impact: maxLoad,
		})
	}

	// 生成建议
	avgLoad := 0.0
	if len(recs.LoadDistribution) > 0 {
		total := 0
		for _, load := range recs.LoadDistribution {
			total += load
		}
		avgLoad = float64(total) / float64(len(recs.LoadDistribution))
	}

	// 负载不均衡检测
	loadVariance := 0.0
	for _, load := range recs.LoadDistribution {
		loadVariance += math.Pow(float64(load)-avgLoad, 2)
	}
	loadVariance = math.Sqrt(loadVariance / float64(len(recs.LoadDistribution)))

	if loadVariance > 2 {
		recs.Suggestions = append(recs.Suggestions, "负载分布不均衡，建议重新分配部分任务")
	}

	// 高优先级任务未分配
	for _, result := range results {
		if result.RecommendedUser == "" && (result.Priority == PriorityUrgent || result.Priority == PriorityHigh) {
			recs.Suggestions = append(recs.Suggestions, "高优先级任务 '"+result.TaskTitle+"' 无合适分配对象")
		}
	}

	// 冲突检测
	conflictCount := 0
	for _, result := range results {
		conflictCount += len(result.Conflicts)
	}
	if conflictCount > 0 {
		recs.Suggestions = append(recs.Suggestions, "存在 "+string(rune(conflictCount))+" 个潜在冲突需要关注")
	}

	return recs, nil
}

// SchedulingRecommendations 调度建议
type SchedulingRecommendations struct {
	ProjectID            string              `json:"project_id"`
	GeneratedAt          time.Time           `json:"generated_at"`
	TaskAssignments      []*SchedulingResult `json:"task_assignments"`
	LoadDistribution     map[string]int      `json:"load_distribution"`
	UnassignedTasks      []*Task             `json:"unassigned_tasks"`
	PotentialBottlenecks []BottleneckInfo    `json:"potential_bottlenecks"`
	Suggestions          []string            `json:"suggestions"`
}

// OptimizeAssignment 优化任务分配
func (ts *TaskScheduler) OptimizeAssignment(projectID string, users []*UserInfo) ([]*SchedulingResult, error) {
	// 多轮优化
	results, err := ts.ScheduleProject(projectID, users)
	if err != nil {
		return nil, err
	}

	// 第一轮：初始分配
	userTasks := make(map[string][]*SchedulingResult)
	for _, r := range results {
		if r.RecommendedUser != "" {
			userTasks[r.RecommendedUser] = append(userTasks[r.RecommendedUser], r)
		}
	}

	// 第二轮：负载均衡
	optimized := make([]*SchedulingResult, 0)
	for _, r := range results {
		if r.RecommendedUser == "" {
			optimized = append(optimized, r)
			continue
		}

		tasks := userTasks[r.RecommendedUser]
		if len(tasks) > ts.config.MaxTasksPerUser {
			// 尝试重新分配
			for _, alt := range r.Alternatives {
				altTasks := userTasks[alt.UserID]
				if len(altTasks) < ts.config.MaxTasksPerUser {
					// 重新分配
					r.RecommendedUser = alt.UserID
					r.Score = alt.Score
					r.Reason = alt.Reason + " (负载均衡优化)"
					break
				}
			}
		}
		optimized = append(optimized, r)
	}

	return optimized, nil
}

// GetWorkloadReport 获取负载报告
func (ts *TaskScheduler) GetWorkloadReport(projectID string, users []*UserInfo) (*WorkloadReport, error) {
	context := ts.buildContext(projectID)

	report := &WorkloadReport{
		ProjectID:   projectID,
		GeneratedAt: time.Now(),
		UserStats:   make([]UserWorkloadStat, 0),
		TotalTasks:  len(context.ProjectTasks),
	}

	// 计算每个用户的负载
	for _, user := range users {
		stat := UserWorkloadStat{
			UserID:         user.ID,
			UserName:       user.Name,
			CurrentTasks:   context.UserWorkloads[user.ID],
			MaxCapacity:    ts.config.MaxTasksPerUser,
			Utilization:    float64(context.UserWorkloads[user.ID]) / float64(ts.config.MaxTasksPerUser) * 100,
			AvailableSlots: ts.config.MaxTasksPerUser - context.UserWorkloads[user.ID],
		}

		if stat.Utilization >= 100 {
			stat.Status = "overloaded"
		} else if stat.Utilization >= 80 {
			stat.Status = "high"
		} else if stat.Utilization >= 50 {
			stat.Status = "moderate"
		} else {
			stat.Status = "available"
		}

		report.UserStats = append(report.UserStats, stat)
	}

	// 计算总体负载
	totalLoad := 0
	for _, load := range context.UserWorkloads {
		totalLoad += load
	}
	report.AverageLoad = float64(totalLoad) / float64(len(users))

	// 检测瓶颈
	for _, stat := range report.UserStats {
		if stat.Status == "overloaded" || stat.Status == "high" {
			report.Bottlenecks = append(report.Bottlenecks, BottleneckInfo{
				Type:   "assignee",
				ID:     stat.UserID,
				Name:   stat.UserName,
				Impact: stat.CurrentTasks,
			})
		}
	}

	return report, nil
}

// WorkloadReport 负载报告
type WorkloadReport struct {
	ProjectID   string             `json:"project_id"`
	GeneratedAt time.Time          `json:"generated_at"`
	UserStats   []UserWorkloadStat `json:"user_stats"`
	TotalTasks  int                `json:"total_tasks"`
	AverageLoad float64            `json:"average_load"`
	Bottlenecks []BottleneckInfo   `json:"bottlenecks"`
}

// UserWorkloadStat 用户负载统计
type UserWorkloadStat struct {
	UserID         string  `json:"user_id"`
	UserName       string  `json:"user_name"`
	CurrentTasks   int     `json:"current_tasks"`
	MaxCapacity    int     `json:"max_capacity"`
	Utilization    float64 `json:"utilization"` // 百分比
	AvailableSlots int     `json:"available_slots"`
	Status         string  `json:"status"` // available, moderate, high, overloaded
}

// SetStrategy 设置调度策略
func (ts *TaskScheduler) SetStrategy(strategy SchedulingStrategy) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.strategy = strategy
}

// UpdateConfig 更新配置
func (ts *TaskScheduler) UpdateConfig(config SchedulerConfig) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.config = config
}

// PriorityBasedStrategy 优先级调度策略
type PriorityBasedStrategy struct {
	config SchedulerConfig
}

// Name 策略名称
func (s *PriorityBasedStrategy) Name() string {
	return "priority_based"
}

// Score 基于优先级评分
func (s *PriorityBasedStrategy) Score(task *Task, user *UserInfo, context SchedulingContext) float64 {
	baseScore := 0.0

	// 纯粹基于优先级
	priorityScore := map[TaskPriority]float64{
		PriorityUrgent: 1.0,
		PriorityHigh:   0.8,
		PriorityMedium: 0.5,
		PriorityLow:    0.2,
	}

	baseScore = priorityScore[task.Priority]

	// 考虑截止日期
	if task.DueDate != nil {
		daysUntilDue := time.Until(*task.DueDate).Hours() / 24
		if daysUntilDue < 1 {
			baseScore *= 1.5 // 紧急任务加权
		} else if daysUntilDue < 3 {
			baseScore *= 1.2
		}
	}

	// 考虑用户负载
	loadFactor := 1.0 - float64(user.CurrentLoad)/float64(s.config.MaxTasksPerUser)*0.5
	baseScore *= loadFactor

	return baseScore
}

// BalancedStrategy 均衡调度策略
type BalancedStrategy struct {
	config SchedulerConfig
}

// Name 策略名称
func (s *BalancedStrategy) Name() string {
	return "balanced"
}

// Score 均衡评分
func (s *BalancedStrategy) Score(task *Task, user *UserInfo, context SchedulingContext) float64 {
	score := 0.0

	// 负载均衡 (50%)
	loadScore := 1.0 - float64(user.CurrentLoad)/float64(s.config.MaxTasksPerUser)
	score += loadScore * 0.5

	// 优先级 (30%)
	priorityScore := map[TaskPriority]float64{
		PriorityUrgent: 1.0,
		PriorityHigh:   0.8,
		PriorityMedium: 0.5,
		PriorityLow:    0.2,
	}
	score += priorityScore[task.Priority] * 0.3

	// 绩效 (20%)
	if user.History != nil {
		score += user.History.OnTimeRate * 0.1
		score += (user.History.QualityScore / 100) * 0.1
	}

	return score
}
