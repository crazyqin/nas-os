// Package sprintboard 提供敏捷看板管理功能，支持 Scrum 和 Kanban 工作流。
// 提供 Sprint 管理、任务分配、泳道管理、燃尽图生成等能力。
package sprintboard

import "time"

// BoardType 看板类型.
type BoardType string

const (
	BoardTypeScrum  BoardType = "scrum"
	BoardTypeKanban BoardType = "kanban"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusBacklog    TaskStatus = "backlog"
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusBlocked    TaskStatus = "blocked"
)

// TaskPriority 任务优先级.
type TaskPriority string

const (
	PriorityCritical TaskPriority = "critical"
	PriorityHigh     TaskPriority = "high"
	PriorityMedium   TaskPriority = "medium"
	PriorityLow      TaskPriority = "low"
)

// TaskType 任务类型.
type TaskType string

const (
	TaskTypeStory   TaskType = "story"
	TaskTypeBug     TaskType = "bug"
	TaskTypeTask    TaskType = "task"
	TaskTypeEpic    TaskType = "epic"
	TaskTypeSubtask TaskType = "subtask"
)

// SprintStatus Sprint 状态.
type SprintStatus string

const (
	SprintStatusPlanning SprintStatus = "planning"
	SprintStatusActive   SprintStatus = "active"
	SprintStatusComplete SprintStatus = "complete"
	SprintStatusCanceled SprintStatus = "canceled"
)

// Sprint Sprint 迭代.
type Sprint struct {
	ID        string       `json:"id"`
	BoardID   string       `json:"board_id"`
	Name      string       `json:"name" binding:"required"`
	Goal      string       `json:"goal,omitempty"`
	Status    SprintStatus `json:"status"`
	StartDate time.Time    `json:"start_date"`
	EndDate   time.Time    `json:"end_date"`
	Capacity  int          `json:"capacity"` // 故事点容量
	Tasks     []*Task      `json:"tasks,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Task 任务卡片.
type Task struct {
	ID          string       `json:"id"`
	BoardID     string       `json:"board_id"`
	SprintID    string       `json:"sprint_id,omitempty"`
	Title       string       `json:"title" binding:"required"`
	Description string       `json:"description,omitempty"`
	Type        TaskType     `json:"type"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id,omitempty"`
	ReporterID  string       `json:"reporter_id,omitempty"`
	StoryPoints int          `json:"story_points"`
	Tags        []string     `json:"tags,omitempty"`
	SwimLaneID  string       `json:"swim_lane_id,omitempty"`
	ParentID    string       `json:"parent_id,omitempty"`
	BlockedBy   []string     `json:"blocked_by,omitempty"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// Board 看板.
type Board struct {
	ID          string      `json:"id"`
	Name        string      `json:"name" binding:"required"`
	Type        BoardType   `json:"type"`
	Description string      `json:"description,omitempty"`
	Columns     []Column    `json:"columns"`
	SwimLanes   []*SwimLane `json:"swim_lanes,omitempty"`
	OwnerID     string      `json:"owner_id"`
	MemberIDs   []string    `json:"member_ids,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Column 看板列.
type Column struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Status    TaskStatus `json:"status"`
	Position  int        `json:"position"`
	WIPLimit  int        `json:"wip_limit"` // 0 表示无限制
	TaskCount int        `json:"task_count"`
}

// SwimLane 泳道.
type SwimLane struct {
	ID          string    `json:"id"`
	BoardID     string    `json:"board_id"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description,omitempty"`
	Position    int       `json:"position"`
	IsCollapsed bool      `json:"is_collapsed"`
	CreatedAt   time.Time `json:"created_at"`
}

// SprintMetrics Sprint 指标.
type SprintMetrics struct {
	SprintID        string         `json:"sprint_id"`
	SprintName      string         `json:"sprint_name"`
	TotalTasks      int            `json:"total_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	TotalPoints     int            `json:"total_points"`
	CompletedPoints int            `json:"completed_points"`
	Velocity        float64        `json:"velocity"` // 每天完成的故事点
	Progress        float64        `json:"progress"` // 完成百分比 0-100
	DaysRemaining   int            `json:"days_remaining"`
	DaysElapsed     int            `json:"days_elapsed"`
	BurndownData    []BurndownDay  `json:"burndown_data,omitempty"`
	TasksByStatus   map[string]int `json:"tasks_by_status"`
	TasksByPriority map[string]int `json:"tasks_by_priority"`
	TasksByAssignee map[string]int `json:"tasks_by_assignee"`
	OverdueTasks    int            `json:"overdue_tasks"`
	BlockedTasks    int            `json:"blocked_tasks"`
}

// BurndownDay 燃尽图每日数据点.
type BurndownDay struct {
	Date            time.Time `json:"date"`
	RemainingPoints int       `json:"remaining_points"`
	IdealPoints     int       `json:"ideal_points"`
	TasksCompleted  int       `json:"tasks_completed"`
	PointsCompleted int       `json:"points_completed"`
}

// CreateSprintRequest 创建 Sprint 请求.
type CreateSprintRequest struct {
	BoardID   string    `json:"board_id" binding:"required"`
	Name      string    `json:"name" binding:"required"`
	Goal      string    `json:"goal,omitempty"`
	StartDate time.Time `json:"start_date" binding:"required"`
	EndDate   time.Time `json:"end_date" binding:"required"`
	Capacity  int       `json:"capacity,omitempty"`
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	BoardID     string       `json:"board_id" binding:"required"`
	SprintID    string       `json:"sprint_id,omitempty"`
	Title       string       `json:"title" binding:"required"`
	Description string       `json:"description,omitempty"`
	Type        TaskType     `json:"type"`
	Priority    TaskPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id,omitempty"`
	StoryPoints int          `json:"story_points,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	SwimLaneID  string       `json:"swim_lane_id,omitempty"`
	ParentID    string       `json:"parent_id,omitempty"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
}

// MoveTaskRequest 移动任务请求.
type MoveTaskRequest struct {
	TargetStatus TaskStatus `json:"target_status" binding:"required"`
	Position     int        `json:"position,omitempty"`
	SwimLaneID   string     `json:"swim_lane_id,omitempty"`
}

// CreateBoardRequest 创建看板请求.
type CreateBoardRequest struct {
	Name        string    `json:"name" binding:"required"`
	Type        BoardType `json:"type" binding:"required"`
	Description string    `json:"description,omitempty"`
	OwnerID     string    `json:"owner_id" binding:"required"`
}

// CreateSwimLaneRequest 创建泳道请求.
type CreateSwimLaneRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
}
