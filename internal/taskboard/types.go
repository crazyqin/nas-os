// Package taskboard provides kanban task board management functionality.
package taskboard

import "time"

// TaskStatus 任务状态类型.
type TaskStatus string

// 任务状态常量.
const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
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

// Board 看板.
type Board struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	OwnerID     string     `json:"owner_id"`
	TaskCount   int        `json:"task_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
}

// TaskCard 任务卡片.
type TaskCard struct {
	ID          string       `json:"id"`
	BoardID     string       `json:"board_id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	Progress    int          `json:"progress"` // 0-100
	DueDate     *time.Time   `json:"due_date,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CreatedBy   string       `json:"created_by"`
}

// Label 标签.
type Label struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// BoardStats 看板统计.
type BoardStats struct {
	TotalTasks   int            `json:"total_tasks"`
	ByStatus     map[string]int `json:"by_status"`
	ByPriority   map[string]int `json:"by_priority"`
	AvgProgress  float64        `json:"avg_progress"`
	OverdueTasks int            `json:"overdue_tasks"`
}

// TaskFilter 任务筛选条件.
type TaskFilter struct {
	Status     []TaskStatus   `json:"status,omitempty"`
	Priority   []TaskPriority `json:"priority,omitempty"`
	AssigneeID string         `json:"assignee_id,omitempty"`
	Labels     []string       `json:"labels,omitempty"`
	Search     string         `json:"search,omitempty"`
	OrderBy    string         `json:"order_by,omitempty"`
	OrderDesc  bool           `json:"order_desc,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
}
