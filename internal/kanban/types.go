// Package kanban provides Kanban board management functionality.
// 支持看板、列、卡片、标签、成员管理
package kanban

import (
	"time"
)

// ============================================================
// 看板相关类型
// ============================================================

// Board 看板
type Board struct {
	ID          string    `json:"id"`           // 看板ID
	Name        string    `json:"name"`         // 看板名称
	Description string    `json:"description"`  // 看板描述
	Columns     []*Column `json:"columns"`      // 看板列
	Tags        []*Tag    `json:"tags"`         // 标签列表
	Members     []*Member `json:"members"`      // 成员列表
	OwnerID     string    `json:"owner_id"`     // 所有者ID
	IsTemplate  bool      `json:"is_template"`  // 是否为模板
	IsPublic    bool      `json:"is_public"`    // 是否公开
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// Column 看板列
type Column struct {
	ID        string  `json:"id"`         // 列ID
	BoardID   string  `json:"board_id"`   // 所属看板ID
	Name      string  `json:"name"`       // 列名称 (待办、进行中、已完成等)
	Position  int     `json:"position"`   // 位置排序
	Cards     []*Card `json:"cards"`      // 卡片列表
	WIPLimit  int     `json:"wip_limit"`  // 在制品限制 (0表示无限制)
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// Card 卡片
type Card struct {
	ID          string    `json:"id"`           // 卡片ID
	ColumnID    string    `json:"column_id"`    // 所属列ID
	BoardID     string    `json:"board_id"`     // 所属看板ID
	Title       string    `json:"title"`        // 卡片标题
	Description string    `json:"description"`  // 卡片描述
	Position    int       `json:"position"`     // 位置排序
	Priority    string    `json:"priority"`     // 优先级: low, medium, high, urgent
	AssigneeID  string    `json:"assignee_id"`  // 负责人ID
	Tags        []string  `json:"tags"`         // 标签ID列表
	Comments    []*Comment `json:"comments"`    // 评论列表
	Attachments []*Attachment `json:"attachments"` // 附件列表
	DueDate     *time.Time `json:"due_date,omitempty"` // 截止日期
	StartDate   *time.Time `json:"start_date,omitempty"` // 开始日期
	CreatedBy   string    `json:"created_by"`   // 创建者ID
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// Tag 标签
type Tag struct {
	ID      string `json:"id"`      // 标签ID
	BoardID string `json:"board_id"` // 所属看板ID
	Name    string `json:"name"`    // 标签名称
	Color   string `json:"color"`   // 标签颜色 (hex)
}

// Member 成员
type Member struct {
	UserID   string `json:"user_id"`   // 用户ID
	Username string `json:"username"`  // 用户名
	Role     string `json:"role"`      // 角色: owner, admin, member, viewer
	JoinedAt time.Time `json:"joined_at"` // 加入时间
}

// Comment 评论
type Comment struct {
	ID        string    `json:"id"`         // 评论ID
	CardID    string    `json:"card_id"`    // 所属卡片ID
	UserID    string    `json:"user_id"`    // 评论者ID
	Username  string    `json:"username"`   // 评论者用户名
	Content   string    `json:"content"`    // 评论内容
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// Attachment 附件
type Attachment struct {
	ID        string    `json:"id"`         // 附件ID
	CardID    string    `json:"card_id"`    // 所属卡片ID
	Filename  string    `json:"filename"`   // 文件名
	FileSize  int64     `json:"file_size"`  // 文件大小 (bytes)
	MimeType  string    `json:"mime_type"`  // MIME类型
	URL       string    `json:"url"`        // 文件URL
	UploadedBy string   `json:"uploaded_by"` // 上传者ID
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// BoardPermission 看板权限
type BoardPermission struct {
	BoardID  string `json:"board_id"`  // 看板ID
	UserID   string `json:"user_id"`   // 用户ID
	CanView  bool   `json:"can_view"`  // 可查看
	CanEdit  bool   `json:"can_edit"`  // 可编辑
	CanAdmin bool   `json:"can_admin"` // 可管理
}

// BoardTemplate 看板模板
type BoardTemplate struct {
	ID          string    `json:"id"`           // 模板ID
	Name        string    `json:"name"`         // 模板名称
	Description string    `json:"description"`  // 模板描述
	Columns     []string  `json:"columns"`      // 默认列名列表
	Tags        []*Tag    `json:"tags"`         // 默认标签
	IsDefault   bool      `json:"is_default"`   // 是否为默认模板
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}

// ============================================================
// 请求/响应类型
// ============================================================

// CreateBoardRequest 创建看板请求
type CreateBoardRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	TemplateID  string `json:"template_id"`  // 使用模板创建
	IsPublic    bool   `json:"is_public"`
}

// UpdateBoardRequest 更新看板请求
type UpdateBoardRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"is_public"`
}

// CreateCardRequest 创建卡片请求
type CreateCardRequest struct {
	ColumnID    string  `json:"column_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	AssigneeID  string  `json:"assignee_id"`
	Tags        []string `json:"tags"`
	DueDate     *time.Time `json:"due_date"`
}

// MoveCardRequest 移动卡片请求
type MoveCardRequest struct {
	TargetColumnID string `json:"target_column_id" binding:"required"`
	Position       int    `json:"position"`
}

// AddCommentRequest 添加评论请求
type AddCommentRequest struct {
	Content string `json:"content" binding:"required"`
}
