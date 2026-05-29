// Package projectkanban 提供项目看板管理功能
package projectkanban

import "time"

// Board 看板
type Board struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Columns     []Column  `json:"columns"`
	Members     []string  `json:"members"`
	CreatedAt   time.Time `json:"created_at"`
	Archived    bool      `json:"archived"`
}

// Column 看板列
type Column struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
	Cards []Card `json:"cards"`
}

// Card 任务卡片
type Card struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"` // low, medium, high, urgent
	Labels      []string   `json:"labels"`
	Assignee    string     `json:"assignee"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Attachments []string   `json:"attachments"` // file paths
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BoardStats 看板统计
type BoardStats struct {
	TotalCards     int     `json:"total_cards"`
	CompletedCards int     `json:"completed_cards"`
	OverdueCards   int     `json:"overdue_cards"`
	CompletionRate float64 `json:"completion_rate"`
}

// CreateBoardRequest 创建看板请求
type CreateBoardRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// CreateCardRequest 创建卡片请求
type CreateCardRequest struct {
	Title       string     `json:"title" binding:"required"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	Labels      []string   `json:"labels"`
	Assignee    string     `json:"assignee"`
	DueDate     *time.Time `json:"due_date"`
	Attachments []string   `json:"attachments"`
	ColumnID    string     `json:"column_id" binding:"required"`
}

// UpdateCardRequest 更新卡片请求
type UpdateCardRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	Labels      []string   `json:"labels"`
	Assignee    string     `json:"assignee"`
	DueDate     *time.Time `json:"due_date"`
	Attachments []string   `json:"attachments"`
	ColumnID    string     `json:"column_id"`
}

// MoveCardRequest 移动卡片请求
type MoveCardRequest struct {
	TargetColumnID string `json:"target_column_id" binding:"required"`
	Position       int    `json:"position"`
}
