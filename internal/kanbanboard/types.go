// Package kanbanboard 提供高级项目看板管理功能，对标群晖 DSM 7.3 项目管理工具。
// 支持看板管理、卡片管理、自定义列、WIP 限制、标签系统、搜索过滤、统计报表。
package kanbanboard

import "time"

// ============================================================
// 核心类型
// ============================================================

// BoardStatus 看板状态
type BoardStatus string

const (
	BoardStatusActive   BoardStatus = "active"
	BoardStatusArchived BoardStatus = "archived"
)

// CardPriority 卡片优先级
type CardPriority string

const (
	PriorityLow    CardPriority = "low"
	PriorityMedium CardPriority = "medium"
	PriorityHigh   CardPriority = "high"
	PriorityUrgent CardPriority = "urgent"
)

// CardStatus 卡片状态
type CardStatus string

const (
	CardStatusTodo       CardStatus = "todo"
	CardStatusInProgress CardStatus = "in_progress"
	CardStatusDone       CardStatus = "done"
	CardStatusBlocked    CardStatus = "blocked"
)

// ============================================================
// 看板实体
// ============================================================

// Board 看板
type Board struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Status      BoardStatus `json:"status"`
	Columns     []*Column   `json:"columns"`
	Labels      []*Label    `json:"labels"`
	Members     []*Member   `json:"members"`
	OwnerID     string      `json:"owner_id"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Column 看板列
type Column struct {
	ID        string  `json:"id"`
	BoardID   string  `json:"board_id"`
	Name      string  `json:"name"`
	Position  int     `json:"position"`
	WIPLimit  int     `json:"wip_limit"` // 0 表示无限制
	Cards     []*Card `json:"cards"`
	CreatedAt time.Time `json:"created_at"`
}

// Card 卡片
type Card struct {
	ID          string       `json:"id"`
	ColumnID    string       `json:"column_id"`
	BoardID     string       `json:"board_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      CardStatus   `json:"status"`
	Priority    CardPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id"`
	LabelIDs    []string     `json:"label_ids"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	Position    int          `json:"position"`
	CreatedBy   string       `json:"created_by"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// Label 标签
type Label struct {
	ID      string `json:"id"`
	BoardID string `json:"board_id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
}

// Member 成员
type Member struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
}

// Activity 活动记录
type Activity struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	CardID    string    `json:"card_id,omitempty"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// 请求类型
// ============================================================

// CreateBoardRequest 创建看板请求
type CreateBoardRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id" binding:"required"`
}

// UpdateBoardRequest 更新看板请求
type UpdateBoardRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// CreateColumnRequest 创建列请求
type CreateColumnRequest struct {
	Name     string `json:"name" binding:"required"`
	Position int    `json:"position"`
	WIPLimit int    `json:"wip_limit"`
}

// UpdateColumnRequest 更新列请求
type UpdateColumnRequest struct {
	Name     *string `json:"name,omitempty"`
	WIPLimit *int    `json:"wip_limit,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// CreateCardRequest 创建卡片请求
type CreateCardRequest struct {
	ColumnID    string       `json:"column_id" binding:"required"`
	Title       string       `json:"title" binding:"required"`
	Description string       `json:"description"`
	Priority    CardPriority `json:"priority"`
	AssigneeID  string       `json:"assignee_id"`
	LabelIDs    []string     `json:"label_ids"`
	DueDate     *time.Time   `json:"due_date"`
	CreatedBy   string       `json:"created_by"`
}

// UpdateCardRequest 更新卡片请求
type UpdateCardRequest struct {
	Title       *string       `json:"title,omitempty"`
	Description *string       `json:"description,omitempty"`
	Priority    *CardPriority `json:"priority,omitempty"`
	AssigneeID  *string       `json:"assignee_id,omitempty"`
	DueDate     *time.Time    `json:"due_date,omitempty"`
	Status      *CardStatus   `json:"status,omitempty"`
}

// MoveCardRequest 移动卡片请求
type MoveCardRequest struct {
	TargetColumnID string `json:"target_column_id" binding:"required"`
	Position       int    `json:"position"`
}

// CreateLabelRequest 创建标签请求
type CreateLabelRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color" binding:"required"`
}

// UpdateLabelRequest 更新标签请求
type UpdateLabelRequest struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
}

// AssignMemberRequest 分配成员请求
type AssignMemberRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// ============================================================
// 搜索与过滤
// ============================================================

// CardFilter 卡片过滤条件
type CardFilter struct {
	BoardID    string       `json:"board_id"`
	AssigneeID string       `json:"assignee_id"`
	LabelIDs   []string     `json:"label_ids"`
	Status     CardStatus   `json:"status"`
	Priority   CardPriority `json:"priority"`
	Keyword    string       `json:"keyword"`
}

// ============================================================
// 统计报表
// ============================================================

// BurndownPoint 燃尽图数据点
type BurndownPoint struct {
	Date      time.Time `json:"date"`
	Remaining int       `json:"remaining"`
	Completed int       `json:"completed"`
}

// VelocityPoint 速度图数据点
type VelocityPoint struct {
	SprintName string `json:"sprint_name"`
	Planned    int    `json:"planned"`
	Completed  int    `json:"completed"`
}

// CumulativeFlowPoint 累积流图数据点
type CumulativeFlowPoint struct {
	Date   time.Time      `json:"date"`
	Counts map[string]int `json:"counts"` // column_name -> count
}

// BoardStats 看板统计
type BoardStats struct {
	TotalCards      int                `json:"total_cards"`
	CompletedCards  int                `json:"completed_cards"`
	TodoCards       int                `json:"todo_cards"`
	InProgressCards int                `json:"in_progress_cards"`
	BlockedCards    int                `json:"blocked_cards"`
	Burndown        []*BurndownPoint   `json:"burndown,omitempty"`
	Velocity        []*VelocityPoint   `json:"velocity,omitempty"`
	CumulativeFlow  []*CumulativeFlowPoint `json:"cumulative_flow,omitempty"`
}
