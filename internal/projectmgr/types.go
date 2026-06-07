// Package projectmgr provides project management functionality.
// 支持项目、里程碑、任务、工时、甘特图、报表等功能
package projectmgr

import (
	"time"
)

// ============================================================
// 项目相关类型
// ============================================================

// Project 项目
type Project struct {
	ID          string       `json:"id"`          // 项目ID
	Name        string       `json:"name"`        // 项目名称
	Description string       `json:"description"` // 项目描述
	Status      string       `json:"status"`      // 状态: planning, active, on_hold, completed, archived
	Priority    string       `json:"priority"`    // 优先级: low, medium, high, critical
	OwnerID     string       `json:"owner_id"`    // 所有者ID
	StartDate   *time.Time   `json:"start_date"`  // 开始日期
	EndDate     *time.Time   `json:"end_date"`    // 结束日期
	Budget      float64      `json:"budget"`      // 预算
	Spent       float64      `json:"spent"`       // 已花费
	Tags        []string     `json:"tags"`        // 标签
	Members     []*Member    `json:"members"`     // 成员列表
	Milestones  []*Milestone `json:"milestones"`  // 里程碑列表
	Tasks       []*Task      `json:"tasks"`       // 任务列表
	TasksTotal  int          `json:"tasks_total"` // 任务总数
	TasksDone   int          `json:"tasks_done"`  // 完成任务数
	Progress    float64      `json:"progress"`    // 进度百分比
	CreatedAt   time.Time    `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time    `json:"updated_at"`  // 更新时间
}

// Milestone 里程碑
type Milestone struct {
	ID          string     `json:"id"`           // 里程碑ID
	ProjectID   string     `json:"project_id"`   // 所属项目ID
	Name        string     `json:"name"`         // 里程碑名称
	Description string     `json:"description"`  // 描述
	DueDate     *time.Time `json:"due_date"`     // 截止日期
	Status      string     `json:"status"`       // 状态: pending, in_progress, completed
	Progress    float64    `json:"progress"`     // 进度百分比
	Tasks       []string   `json:"tasks"`        // 关联任务ID列表
	CreatedAt   time.Time  `json:"created_at"`   // 创建时间
	CompletedAt *time.Time `json:"completed_at"` // 完成时间
}

// Task 任务
type Task struct {
	ID             string       `json:"id"`              // 任务ID
	ProjectID      string       `json:"project_id"`      // 所属项目ID
	MilestoneID    string       `json:"milestone_id"`    // 所属里程碑ID
	ParentID       string       `json:"parent_id"`       // 父任务ID (支持子任务)
	Title          string       `json:"title"`           // 任务标题
	Description    string       `json:"description"`     // 任务描述
	Status         string       `json:"status"`          // 状态: todo, in_progress, review, done, cancelled
	Priority       string       `json:"priority"`        // 优先级: low, medium, high, critical
	AssigneeID     string       `json:"assignee_id"`     // 负责人ID
	AssigneeName   string       `json:"assignee_name"`   // 负责人名称
	Tags           []string     `json:"tags"`            // 标签
	Dependencies   []string     `json:"dependencies"`    // 依赖任务ID列表
	StartDate      *time.Time   `json:"start_date"`      // 开始日期
	DueDate        *time.Time   `json:"due_date"`        // 截止日期
	EstimatedHours float64      `json:"estimated_hours"` // 预估工时
	ActualHours    float64      `json:"actual_hours"`    // 实际工时
	SubTasks       []*Task      `json:"sub_tasks"`       // 子任务
	Timesheets     []*Timesheet `json:"timesheets"`      // 工时记录
	Comments       []*Comment   `json:"comments"`        // 评论
	CreatedBy      string       `json:"created_by"`      // 创建者ID
	CreatedAt      time.Time    `json:"created_at"`      // 创建时间
	UpdatedAt      time.Time    `json:"updated_at"`      // 更新时间
	CompletedAt    *time.Time   `json:"completed_at"`    // 完成时间
}

// Member 项目成员
type Member struct {
	UserID     string    `json:"user_id"`     // 用户ID
	Username   string    `json:"username"`    // 用户名
	Role       string    `json:"role"`        // 角色: owner, manager, member, viewer
	HourlyRate float64   `json:"hourly_rate"` // 时薪
	JoinedAt   time.Time `json:"joined_at"`   // 加入时间
}

// Timesheet 工时记录
type Timesheet struct {
	ID          string    `json:"id"`          // 工时ID
	TaskID      string    `json:"task_id"`     // 所属任务ID
	UserID      string    `json:"user_id"`     // 用户ID
	Username    string    `json:"username"`    // 用户名
	Date        time.Time `json:"date"`        // 日期
	Hours       float64   `json:"hours"`       // 工时
	Description string    `json:"description"` // 工作描述
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
}

// Comment 任务评论
type Comment struct {
	ID        string    `json:"id"`         // 评论ID
	TaskID    string    `json:"task_id"`    // 所属任务ID
	UserID    string    `json:"user_id"`    // 评论者ID
	Username  string    `json:"username"`   // 评论者用户名
	Content   string    `json:"content"`    // 评论内容
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// GanttTask 甘特图任务数据
type GanttTask struct {
	ID           string     `json:"id"`           // 任务ID
	Title        string     `json:"title"`        // 任务标题
	StartDate    *time.Time `json:"start_date"`   // 开始日期
	EndDate      *time.Time `json:"end_date"`     // 结束日期 (DueDate)
	Progress     float64    `json:"progress"`     // 进度百分比
	Dependencies []string   `json:"dependencies"` // 依赖任务ID
	Assignee     string     `json:"assignee"`     // 负责人
	Status       string     `json:"status"`       // 状态
	ParentID     string     `json:"parent_id"`    // 父任务ID
	Level        int        `json:"level"`        // 层级 (用于缩进)
}

// ProjectReport 项目报表
type ProjectReport struct {
	ProjectID        string          `json:"project_id"`
	ProjectName      string          `json:"project_name"`
	TotalTasks       int             `json:"total_tasks"`
	CompletedTasks   int             `json:"completed_tasks"`
	OverdueTasks     int             `json:"overdue_tasks"`
	Progress         float64         `json:"progress"`
	BudgetUsed       float64         `json:"budget_used"`
	BudgetRemaining  float64         `json:"budget_remaining"`
	TotalHoursLogged float64         `json:"total_hours_logged"`
	MemberStats      []*MemberStat   `json:"member_stats"`
	TimelineStats    []*TimelineStat `json:"timeline_stats"`
	GeneratedAt      time.Time       `json:"generated_at"`
}

// MemberStat 成员统计
type MemberStat struct {
	UserID         string  `json:"user_id"`
	Username       string  `json:"username"`
	TasksAssigned  int     `json:"tasks_assigned"`
	TasksCompleted int     `json:"tasks_completed"`
	HoursLogged    float64 `json:"hours_logged"`
}

// TimelineStat 时间线统计
type TimelineStat struct {
	Date         string  `json:"date"`          // 日期 (YYYY-MM-DD)
	TasksCreated int     `json:"tasks_created"` // 创建任务数
	TasksDone    int     `json:"tasks_done"`    // 完成任务数
	HoursLogged  float64 `json:"hours_logged"`  // 记录工时
}

// ============================================================
// 请求/响应类型
// ============================================================

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	Budget      float64    `json:"budget"`
	Tags        []string   `json:"tags"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	Priority    *string    `json:"priority"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	Budget      *float64   `json:"budget"`
	Tags        []string   `json:"tags"`
}

// CreateMilestoneRequest 创建里程碑请求
type CreateMilestoneRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	MilestoneID    string     `json:"milestone_id"`
	ParentID       string     `json:"parent_id"`
	Title          string     `json:"title" binding:"required"`
	Description    string     `json:"description"`
	Priority       string     `json:"priority"`
	AssigneeID     string     `json:"assignee_id"`
	Tags           []string   `json:"tags"`
	Dependencies   []string   `json:"dependencies"`
	StartDate      *time.Time `json:"start_date"`
	DueDate        *time.Time `json:"due_date"`
	EstimatedHours float64    `json:"estimated_hours"`
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Title          *string    `json:"title"`
	Description    *string    `json:"description"`
	Status         *string    `json:"status"`
	Priority       *string    `json:"priority"`
	AssigneeID     *string    `json:"assignee_id"`
	Tags           []string   `json:"tags"`
	Dependencies   []string   `json:"dependencies"`
	StartDate      *time.Time `json:"start_date"`
	DueDate        *time.Time `json:"due_date"`
	EstimatedHours *float64   `json:"estimated_hours"`
}

// LogTimeRequest 记录工时请求
type LogTimeRequest struct {
	TaskID      string    `json:"task_id" binding:"required"`
	Date        time.Time `json:"date" binding:"required"`
	Hours       float64   `json:"hours" binding:"required"`
	Description string    `json:"description"`
}

// AddCommentRequest 添加评论请求
type AddCommentRequest struct {
	Content string `json:"content" binding:"required"`
}
