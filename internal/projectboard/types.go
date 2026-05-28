// Package projectboard 提供项目看板管理功能，支持看板/列表/甘特图多视图。
package projectboard

import "time"

// ProjectStatus 项目状态类型。
type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusArchived  ProjectStatus = "archived"
	ProjectStatusSuspended ProjectStatus = "suspended"
)

// CardStatus 卡片状态类型。
type CardStatus string

const (
	CardStatusBacklog    CardStatus = "backlog"
	CardStatusTodo       CardStatus = "todo"
	CardStatusInProgress CardStatus = "in_progress"
	CardStatusReview     CardStatus = "review"
	CardStatusDone       CardStatus = "done"
)

// CardPriority 卡片优先级类型。
type CardPriority string

const (
	PriorityLow    CardPriority = "low"
	PriorityMedium CardPriority = "medium"
	PriorityHigh   CardPriority = "high"
	PriorityUrgent CardPriority = "urgent"
)

// ViewMode 视图模式。
type ViewMode string

const (
	ViewBoard   ViewMode = "board"
	ViewList    ViewMode = "list"
	ViewGantt   ViewMode = "gantt"
	ViewCalendar ViewMode = "calendar"
)

// Project 项目。
type Project struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Status      ProjectStatus `json:"status"`
	OwnerID     string        `json:"owner_id"`
	MemberIDs   []string      `json:"member_ids,omitempty"`
	BoardCount  int           `json:"board_count"`
	CardCount   int           `json:"card_count"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	CreatedBy   string        `json:"created_by"`
}

// Board 看板。
type Board struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Columns     []Column  `json:"columns"`
	CardCount   int       `json:"card_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Column 看板列。
type Column struct {
	ID        string     `json:"id"`
	BoardID   string     `json:"board_id"`
	Name      string     `json:"name"`
	Status    CardStatus `json:"status"`
	Position  int        `json:"position"`
	WIPLimit  int        `json:"wip_limit"` // 0 表示无限制
	CardCount int        `json:"card_count"`
}

// Card 卡片。
type Card struct {
	ID           string       `json:"id"`
	BoardID      string       `json:"board_id"`
	ColumnID     string       `json:"column_id"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	Status       CardStatus   `json:"status"`
	Priority     CardPriority `json:"priority"`
	AssigneeID   string       `json:"assignee_id,omitempty"`
	ReporterID   string       `json:"reporter_id"`
	Labels       []string     `json:"labels,omitempty"`
	StoryPoints  int          `json:"story_points"`
	Progress     int          `json:"progress"` // 0-100
	EstimateHrs  float64      `json:"estimate_hrs"`
	SpentHrs     float64      `json:"spent_hrs"`
	SprintID     string       `json:"sprint_id,omitempty"`
	MilestoneID  string       `json:"milestone_id,omitempty"`
	Dependencies []string     `json:"dependencies,omitempty"` // 依赖的卡片 ID
	SubtaskIDs   []string     `json:"subtask_ids,omitempty"`
	ParentID     string       `json:"parent_id,omitempty"`
	DueDate      *time.Time   `json:"due_date,omitempty"`
	StartDate    *time.Time   `json:"start_date,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CreatedBy    string       `json:"created_by"`
}

// Label 标签。
type Label struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// Sprint 迭代。
type Sprint struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Goal        string    `json:"goal,omitempty"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Status      string    `json:"status"` // planned, active, completed
	CardIDs     []string  `json:"card_ids,omitempty"`
	Velocity    int       `json:"velocity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Milestone 里程碑。
type Milestone struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Status      string     `json:"status"` // open, closed
	Progress    int        `json:"progress"`
	CardIDs     []string   `json:"card_ids,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Workflow 自定义工作流。
type Workflow struct {
	ID          string           `json:"id"`
	ProjectID   string           `json:"project_id"`
	Name        string           `json:"name"`
	Transitions []Transition     `json:"transitions"`
	Rules       []AutomationRule `json:"rules,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Transition 状态转换。
type Transition struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	FromStatus CardStatus `json:"from_status"`
	ToStatus   CardStatus `json:"to_status"`
	GuardExpr  string     `json:"guard_expr,omitempty"`
}

// AutomationRule 自动化规则。
type AutomationRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Trigger   string    `json:"trigger"`   // card_moved, card_created, card_completed, etc.
	Condition string    `json:"condition"` // 条件表达式
	Actions   []string  `json:"actions"`   // 动作列表
	Enabled   bool      `json:"enabled"`
}

// TaskDependency 任务依赖。
type TaskDependency struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Type     string `json:"type"` // finish_to_start, start_to_start, finish_to_finish, start_to_finish
}

// TimeEntry 时间记录。
type TimeEntry struct {
	ID        string    `json:"id"`
	CardID    string    `json:"card_id"`
	UserID    string    `json:"user_id"`
	Hours     float64   `json:"hours"`
	Note      string    `json:"note,omitempty"`
	Date      time.Time `json:"date"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectStats 项目统计。
type ProjectStats struct {
	TotalCards    int            `json:"total_cards"`
	ByStatus      map[string]int `json:"by_status"`
	ByPriority    map[string]int `json:"by_priority"`
	TotalPoints   int            `json:"total_points"`
	CompletedPoints int          `json:"completed_points"`
	TotalHours    float64        `json:"total_hours"`
	SpentHours    float64        `json:"spent_hours"`
	OverdueCards  int            `json:"overdue_cards"`
}

// BurndownPoint 燃尽图数据点。
type BurndownPoint struct {
	Date       time.Time `json:"date"`
	Remaining  int       `json:"remaining"`
	Ideal      int       `json:"ideal"`
	Completed  int       `json:"completed"`
}

// VelocityData 速度数据。
type VelocityData struct {
	SprintID      string `json:"sprint_id"`
	SprintName    string `json:"sprint_name"`
	Committed     int    `json:"committed"`
	Completed     int    `json:"completed"`
	CompletionRate float64 `json:"completion_rate"`
}

// CardFilter 卡片筛选条件。
type CardFilter struct {
	Status      []CardStatus   `json:"status,omitempty"`
	Priority    []CardPriority `json:"priority,omitempty"`
	AssigneeID  string         `json:"assignee_id,omitempty"`
	SprintID    string         `json:"sprint_id,omitempty"`
	MilestoneID string         `json:"milestone_id,omitempty"`
	Labels      []string       `json:"labels,omitempty"`
	Search      string         `json:"search,omitempty"`
	OrderBy     string         `json:"order_by,omitempty"`
	OrderDesc   bool           `json:"order_desc,omitempty"`
	Limit       int            `json:"limit,omitempty"`
	Offset      int            `json:"offset,omitempty"`
}
