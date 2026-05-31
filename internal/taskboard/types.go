// Package taskboard 任务看板模块 - 管理任务、标签和统计
package taskboard

import "time"

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"        // 待办
	StatusInProgress TaskStatus = "in_progress" // 进行中
	StatusDone       TaskStatus = "done"        // 已完成
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	PriorityUrgent TaskPriority = "urgent" // 紧急
	PriorityHigh   TaskPriority = "high"   // 高
	PriorityMedium TaskPriority = "medium" // 中
	PriorityLow    TaskPriority = "low"    // 低
)

// Board 看板
type Board struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	TaskCount   int       `json:"task_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// TaskCard 任务卡片
type TaskCard struct {
	ID          string       `json:"id"`
	BoardID     string       `json:"board_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	Progress    int          `json:"progress"` // 0-100
	AssigneeID  string       `json:"assignee_id,omitempty"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CreatedBy   string       `json:"created_by"`
}

// Label 标签
type Label struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskFilter 任务过滤条件
type TaskFilter struct {
	Status     []TaskStatus   `form:"status"`
	Priority   []TaskPriority `form:"priority"`
	AssigneeID string         `form:"assignee_id"`
	Labels     []string       `form:"labels"`
	Search     string         `form:"search"`
	OrderBy    string         `form:"order_by"`    // priority, due_date, status, created_at
	OrderDesc  bool           `form:"order_desc"`
	Limit      int            `form:"limit"`
	Offset     int            `form:"offset"`
}

// BoardStats 看板统计
type BoardStats struct {
	TotalTasks  int            `json:"total_tasks"`
	AvgProgress float64        `json:"avg_progress"`
	OverdueTasks int           `json:"overdue_tasks"`
	ByStatus    map[string]int `json:"by_status"`
	ByPriority  map[string]int `json:"by_priority"`
}

// CreateBoardRequest 创建看板请求
type CreateBoardRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Title       string       `json:"title" binding:"required"`
	Description string       `json:"description"`
	Priority    TaskPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id"`
	DueDate     *time.Time   `json:"due_date"`
	Labels      []string     `json:"labels"`
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Title       *string       `json:"title,omitempty"`
	Description *string       `json:"description,omitempty"`
	Priority    *TaskPriority `json:"priority,omitempty"`
	AssigneeID  *string       `json:"assignee_id,omitempty"`
	DueDate     *time.Time    `json:"due_date,omitempty"`
	Labels      []string      `json:"labels,omitempty"`
}

// MoveTaskRequest 移动任务请求
type MoveTaskRequest struct {
	Status TaskStatus `json:"status" binding:"required"`
}

// CreateLabelRequest 创建标签请求
type CreateLabelRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color" binding:"required"`
}

// UpdateProgressRequest 更新进度请求
type UpdateProgressRequest struct {
	Progress int `json:"progress" binding:"required,min=0,max=100"`
}
