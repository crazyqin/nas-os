// Package kanban 提供看板管理功能，支持看板创建、卡片拖拽、标签管理、成员分配、进度追踪。
package kanban

import "time"

// ============================================================
// 看板相关类型
// ============================================================

// Board 看板
type Board struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Columns     []*Column `json:"columns"`
	Labels      []*Label  `json:"labels"`
	Members     []*Member `json:"members"`
	OwnerID     string    `json:"owner_id"`
	IsArchived  bool      `json:"is_archived"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Column 看板列
type Column struct {
	ID        string  `json:"id"`
	BoardID   string  `json:"board_id"`
	Name      string  `json:"name"`
	Position  int     `json:"position"`
	Cards     []*Card `json:"cards"`
	WIPLimit  int     `json:"wip_limit"`
	CreatedAt time.Time `json:"created_at"`
}

// Card 卡片
type Card struct {
	ID          string     `json:"id"`
	ColumnID    string     `json:"column_id"`
	BoardID     string     `json:"board_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Position    int        `json:"position"`
	Priority    string     `json:"priority"`
	AssigneeID  string     `json:"assignee_id"`
	LabelIDs    []string   `json:"label_ids"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
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
	Role     string    `json:"role"`
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

// AddCardRequest 创建卡片请求
type AddCardRequest struct {
	ColumnID    string  `json:"column_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	AssigneeID  string  `json:"assignee_id"`
	LabelIDs    []string `json:"label_ids"`
	DueDate     *time.Time `json:"due_date"`
	CreatedBy   string  `json:"created_by"`
}

// MoveCardRequest 移动卡片请求
type MoveCardRequest struct {
	TargetColumnID string `json:"target_column_id" binding:"required"`
	Position       int    `json:"position"`
}

// AddLabelRequest 添加标签请求
type AddLabelRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color" binding:"required"`
}

// AssignMemberRequest 分配成员请求
type AssignMemberRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required"`
}
