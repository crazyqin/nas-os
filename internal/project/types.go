// Package project provides project management functionality
package project

import "time"

// TaskStatus 任务状态类型.
type TaskStatus string

// 任务状态常量.
const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskPriority 任务优先级类型.
type TaskPriority string

// 任务优先级常量.
const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

// Task 任务定义.
type Task struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	Status         TaskStatus   `json:"status"`
	Priority       TaskPriority `json:"priority"`
	AssigneeID     string       `json:"assignee_id,omitempty"`
	ReporterID     string       `json:"reporter_id"`
	ProjectID      string       `json:"project_id"`
	MilestoneID    string       `json:"milestone_id,omitempty"`
	ParentID       string       `json:"parent_id,omitempty"` // 子任务
	Tags           []string     `json:"tags,omitempty"`
	Labels         []string     `json:"labels,omitempty"`
	DueDate        *time.Time   `json:"due_date,omitempty"`
	StartDate      *time.Time   `json:"start_date,omitempty"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
	EstimatedHours float64      `json:"estimated_hours,omitempty"`
	ActualHours    float64      `json:"actual_hours,omitempty"`
	Progress       int          `json:"progress"` // 0-100
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	CreatedBy      string       `json:"created_by"`
}

// Milestone 里程碑.
type Milestone struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ProjectID   string     `json:"project_id"`
	Status      string     `json:"status"` // planned, active, completed, cancelled
	DueDate     *time.Time `json:"due_date,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	TaskCount   int        `json:"task_count"`
	DoneCount   int        `json:"done_count"`
	Progress    int        `json:"progress"` // 0-100
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
}

// Project 项目.
type Project struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Key         string     `json:"key"`    // 项目简称，用于任务编号
	Status      string     `json:"status"` // active, archived, cancelled
	OwnerID     string     `json:"owner_id"`
	MemberIDs   []string   `json:"member_ids,omitempty"`
	StartDtae   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	TaskCount   int        `json:"task_count"`
	DoneCount   int        `json:"done_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
}

// TaskComment 任务评论.
type TaskComment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskHistory 任务历史记录.
type TaskHistory struct {
	ID        string                 `json:"id"`
	TaskID    string                 `json:"task_id"`
	Field     string                 `json:"field"`
	OldValue  string                 `json:"old_value"`
	NewValue  string                 `json:"new_value"`
	UserID    string                 `json:"user_id"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TaskFilter 任务筛选条件.
type TaskFilter struct {
	Status      []TaskStatus   `json:"status,omitempty"`
	Priority    []TaskPriority `json:"priority,omitempty"`
	AssigneeID  string         `json:"assignee_id,omitempty"`
	ReporterID  string         `json:"reporter_id,omitempty"`
	ProjectID   string         `json:"project_id,omitempty"`
	MilestoneID string         `json:"milestone_id,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Labels      []string       `json:"labels,omitempty"`
	Search      string         `json:"search,omitempty"`
	DueBefore   *time.Time     `json:"due_before,omitempty"`
	DueAfter    *time.Time     `json:"due_after,omitempty"`
	OrderBy     string         `json:"order_by,omitempty"`
	OrderDesc   bool           `json:"order_desc,omitempty"`
	Limit       int            `json:"limit,omitempty"`
	Offset      int            `json:"offset,omitempty"`
}

// TaskStats 任务统计.
type TaskStats struct {
	Total             int            `json:"total"`
	ByStatus          map[string]int `json:"by_status"`
	ByPriority        map[string]int `json:"by_priority"`
	ByAssignee        map[string]int `json:"by_assignee"`
	Overdue           int            `json:"overdue"`
	CompletedThisWeek int            `json:"completed_this_week"`
	CreatedThisWeek   int            `json:"created_this_week"`
}
