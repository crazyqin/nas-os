// Package projectcenter 提供项目中心管理功能
package projectcenter

import "time"

// ========== 项目类型 ==========

// Project 项目.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	TeamIDs     []string  `json:"team_ids"`
	Status      string    `json:"status"` // active, archived, paused, completed
	Tags        []string  `json:"tags"`
	Visibility  string    `json:"visibility"` // public, private
	TemplateID  string    `json:"template_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 任务类型 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusBlocked    TaskStatus = "blocked"
)

// TaskPriority 任务优先级.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

// Task 任务.
type Task struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Title         string       `json:"title" binding:"required"`
	Description   string       `json:"description"`
	Status        TaskStatus   `json:"status"`
	Priority      TaskPriority `json:"priority"`
	AssigneeID    string       `json:"assignee_id,omitempty"`
	ReporterID    string       `json:"reporter_id"`
	Tags          []string     `json:"tags"`
	ParentTaskID  string       `json:"parent_task_id,omitempty"`
	SubtaskIDs    []string     `json:"subtask_ids"`
	Dependencies  []string     `json:"dependencies"` // 依赖的任务ID
	EstimateHours float64      `json:"estimate_hours"`
	ActualHours   float64      `json:"actual_hours"`
	StartDate     *time.Time   `json:"start_date,omitempty"`
	DueDate       *time.Time   `json:"due_date,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// Comment 任务评论.
type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	Mentions  []string  `json:"mentions"`            // @提及的用户ID列表
	ParentID  string    `json:"parent_id,omitempty"` // 回复的评论ID
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 看板类型 ==========

// KanbanBoard 看板.
type KanbanBoard struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Columns   []KanbanColumn `json:"columns"`
	Filters   KanbanFilters  `json:"filters"`
	CreatedAt time.Time      `json:"created_at"`
}

// KanbanColumn 看板列.
type KanbanColumn struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Status   TaskStatus `json:"status"`
	Order    int        `json:"order"`
	WIPLimit int        `json:"wip_limit"` // 在制品限制
}

// KanbanFilters 看板过滤条件.
type KanbanFilters struct {
	AssigneeIDs []string `json:"assignee_ids,omitempty"`
	Priorities  []string `json:"priorities,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ========== 里程碑类型 ==========

// Milestone 里程碑.
type Milestone struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	TaskIDs     []string   `json:"task_ids"`
	Progress    float64    `json:"progress"` // 0-100
	Status      string     `json:"status"`   // pending, in_progress, completed, overdue
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ========== 甘特图类型 ==========

// GanttTask 甘特图任务数据.
type GanttTask struct {
	TaskID       string    `json:"task_id"`
	Title        string    `json:"title"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Progress     float64   `json:"progress"`
	Dependencies []string  `json:"dependencies"`
	AssigneeID   string    `json:"assignee_id"`
	Level        int       `json:"level"` // 层级（用于子任务缩进）
}

// GanttData 甘特图数据.
type GanttData struct {
	ProjectID string      `json:"project_id"`
	StartDate time.Time   `json:"start_date"`
	EndDate   time.Time   `json:"end_date"`
	Tasks     []GanttTask `json:"tasks"`
}

// ========== 项目模板类型 ==========

// ProjectTemplate 项目模板.
type ProjectTemplate struct {
	ID          string           `json:"id"`
	Name        string           `json:"name" binding:"required"`
	Description string           `json:"description"`
	Category    string           `json:"category"` // software, marketing, research, general
	Columns     []TemplateColumn `json:"columns"`
	Tasks       []TemplateTask   `json:"tasks"`
	Tags        []string         `json:"tags"`
	IsDefault   bool             `json:"is_default"`
	UsageCount  int              `json:"usage_count"`
	CreatedAt   time.Time        `json:"created_at"`
}

// TemplateColumn 模板列定义.
type TemplateColumn struct {
	Name   string     `json:"name"`
	Status TaskStatus `json:"status"`
	Order  int        `json:"order"`
}

// TemplateTask 模板任务定义.
type TemplateTask struct {
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Priority      TaskPriority `json:"priority"`
	Tags          []string     `json:"tags"`
	EstimateHours float64      `json:"estimate_hours"`
	Phase         string       `json:"phase"` // 所属阶段
	Order         int          `json:"order"`
}

// ========== 统计类型 ==========

// ProjectStats 项目统计.
type ProjectStats struct {
	ProjectID       string         `json:"project_id"`
	TotalTasks      int            `json:"total_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	OverdueTasks    int            `json:"overdue_tasks"`
	CompletionRate  float64        `json:"completion_rate"`
	AvgTaskDuration float64        `json:"avg_task_duration_hours"`
	TasksByStatus   map[string]int `json:"tasks_by_status"`
	TasksByPriority map[string]int `json:"tasks_by_priority"`
	TasksByAssignee map[string]int `json:"tasks_by_assignee"`
	MilestoneStats  MilestoneStats `json:"milestone_stats"`
	TimelineStats   TimelineStats  `json:"timeline_stats"`
}

// MilestoneStats 里程碑统计.
type MilestoneStats struct {
	Total          int     `json:"total"`
	Completed      int     `json:"completed"`
	Overdue        int     `json:"overdue"`
	CompletionRate float64 `json:"completion_rate"`
}

// TimelineStats 时间线统计.
type TimelineStats struct {
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	ElapsedDays   int       `json:"elapsed_days"`
	RemainingDays int       `json:"remaining_days"`
	TotalDuration int       `json:"total_duration_days"`
	Progress      float64   `json:"progress"` // 0-100
}

// MemberWorkload 成员工作量.
type MemberWorkload struct {
	UserID         string  `json:"user_id"`
	TotalTasks     int     `json:"total_tasks"`
	ActiveTasks    int     `json:"active_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	TotalHours     float64 `json:"total_hours"`
	Utilization    float64 `json:"utilization"` // 0-100 利用率
}

// ========== 请求/响应类型 ==========

// CreateProjectRequest 创建项目请求.
type CreateProjectRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	TeamIDs     []string `json:"team_ids"`
	Tags        []string `json:"tags"`
	Visibility  string   `json:"visibility"`
	TemplateID  string   `json:"template_id"`
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	Title         string       `json:"title" binding:"required"`
	Description   string       `json:"description"`
	Priority      TaskPriority `json:"priority"`
	AssigneeID    string       `json:"assignee_id"`
	Tags          []string     `json:"tags"`
	ParentTaskID  string       `json:"parent_task_id"`
	Dependencies  []string     `json:"dependencies"`
	EstimateHours float64      `json:"estimate_hours"`
	StartDate     *time.Time   `json:"start_date"`
	DueDate       *time.Time   `json:"due_date"`
}

// UpdateTaskRequest 更新任务请求.
type UpdateTaskRequest struct {
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Status        TaskStatus   `json:"status"`
	Priority      TaskPriority `json:"priority"`
	AssigneeID    string       `json:"assignee_id"`
	Tags          []string     `json:"tags"`
	Dependencies  []string     `json:"dependencies"`
	EstimateHours *float64     `json:"estimate_hours"`
	ActualHours   *float64     `json:"actual_hours"`
	StartDate     *time.Time   `json:"start_date"`
	DueDate       *time.Time   `json:"due_date"`
}

// MoveTaskRequest 移动任务请求.
type MoveTaskRequest struct {
	Status   TaskStatus `json:"status" binding:"required"`
	Position int        `json:"position"`
}

// CreateMilestoneRequest 创建里程碑请求.
type CreateMilestoneRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
}

// CreateCommentRequest 创建评论请求.
type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required"`
	ParentID string `json:"parent_id"`
}

// ListOptions 列表查询选项.
type ListOptions struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Status   string `json:"status"`
	SortBy   string `json:"sort_by"`
	Order    string `json:"order"` // asc, desc
}

// PaginatedResponse 分页响应.
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
